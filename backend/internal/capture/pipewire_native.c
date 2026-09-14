#include "pipewire_native.h"

#include <pipewire/pipewire.h>
#include <spa/param/video/format-utils.h>
#include <spa/param/video/type-info.h>
#include <spa/debug/types.h>
#include <spa/debug/format.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <limits.h>
#include <time.h>

// Diagnostics below go to stderr, not stdout. systemd hands both fds to
// journald, and since neither is a terminal libc fully buffers stdout while
// stderr stays unbuffered: the one-shot warnings here would otherwise sit in
// that buffer and never reach the journal. Go's log package also writes to
// stderr, so the two stay in order.

// One of the two frame slots in the double buffer below. Dimensions are
// captured per-slot, at write time, rather than read from the mutable
// ud->frame_* fields at claim time: those globals can be updated by a
// param_changed event between a slot's write and pw_get_frame claiming it,
// and pairing a stale slot with a fresh global height/stride would let
// convertToRGBA compute a buffer size larger than what was actually
// allocated for that slot.
struct frame_buf {
	uint8_t *data;
	int capacity; // bytes allocated; may exceed the current frame's size
	int stride;
	int width;
	int height;
	uint32_t format;
};

// Why a frame was dropped before any copy. Each reason warns once on its own
// flag: a not-yet-negotiated stream drops frames routinely, and a shared flag
// would let that benign case swallow the single warning a real fault emits,
// or blame it on the wrong part of the pipeline.
enum drop_reason {
	DROP_BOTTOM_UP = 0,
	DROP_NOT_NEGOTIATED,
	DROP_ZERO_STRIDE,
	DROP_OVERFLOW,
	DROP_SHORT_CHUNK,
	DROP_CORRUPTED,
	DROP_NO_DATA,
	DROP_UNMAPPED,
	DROP_ALLOC_FAILED,
	DROP_REASON_COUNT,
};

static const char *drop_reason_text[DROP_REASON_COUNT] = {
	"bottom-up buffers are not supported",
	"stream geometry not negotiated yet",
	"producer reported zero stride on a negotiated stream",
	"frame size overflows int",
	"chunk carries fewer bytes than the negotiated frame",
	"producer flagged the chunk corrupted",
	"buffer carries no data planes",
	"buffer memory is not mapped",
	"could not allocate a frame slot",
};

// User data passed to callbacks
struct user_data {
	struct pw_thread_loop *loop;
	struct pw_stream *stream;

	// Double-buffered frame storage. on_stream_process (PipeWire thread,
	// implicitly under the loop lock while dispatching) always writes into
	// frame[1 - front_idx]: the slot Go does not currently hold a pointer
	// into. pw_get_frame (explicitly under the loop lock) hands out
	// frame[ready_idx] and then sets front_idx = ready_idx, so the very
	// next write recomputes its target away from the slot just handed out.
	// Because front_idx only ever changes inside pw_get_frame, and both
	// sides run under the same lock, the producer can never write into the
	// buffer Go is mid-read on, regardless of how long that read takes.
	struct frame_buf frame[2];
	int front_idx; // slot most recently handed to Go
	int ready_idx; // slot most recently completed by on_stream_process, or
	               // -1 if no frame has been produced yet

	// Bumped every time on_stream_process publishes a slot. Go compares it
	// against the value from its previous poll to tell "the compositor has
	// sent nothing since" from "here is a new frame". Without it a stalled
	// stream looks identical to a healthy one, because pw_get_frame keeps
	// handing back the last slot written.
	uint64_t frame_seq;

	// Set from on_stream_state_changed once the stream reaches a state it
	// cannot come back from, so pw_get_frame reports the failure rather
	// than serving the last good slot for the rest of the session.
	int stream_failed;

	// Whether the stream is currently in PW_STREAM_STATE_STREAMING. A
	// healthy screencast stays streaming even while the screen is static
	// and no buffers are produced, so leaving that state is what separates
	// a dead producer from an idle one.
	int streaming;

	// The run of consecutive buffers the producer has sent that could not
	// be used, with the monotonic times of the first and last of them. A
	// still screen sends no buffers at all, so only the producer's own
	// output can end the run: a usable buffer, a new format, or a drop
	// arriving more than UNUSABLE_RUN_GAP_NS after the one before it.
	uint64_t unusable_run;
	int64_t unusable_first_ns;
	int64_t unusable_last_ns;
	enum drop_reason unusable_reason;

	int frame_width;
	int frame_height;
	uint32_t frame_format;
	int loop_started; // pw_start_loop succeeded; gates pw_stop_loop
	int warned_drop[DROP_REASON_COUNT];
	// For Go callbacks
	void *go_context;
};

// Stream event handlers
static void on_stream_state_changed(void *data, enum pw_stream_state old,
				    enum pw_stream_state state, const char *error)
{
	struct user_data *ud = data;
	fprintf(stderr, "[PipeWire] Stream state changed: %s -> %s\n",
		pw_stream_state_as_string(old),
		pw_stream_state_as_string(state));

	ud->streaming = (state == PW_STREAM_STATE_STREAMING);

	if (state == PW_STREAM_STATE_ERROR) {
		fprintf(stderr, "[PipeWire] Stream error: %s\n", error);
		ud->stream_failed = 1;
		pw_thread_loop_signal(ud->loop, false);
	}
}

static void on_stream_param_changed(void *data, uint32_t id, const struct spa_pod *param)
{
	struct user_data *ud = data;

	if (id != SPA_PARAM_Format)
		return;

	// Drops while a stream renegotiates are about the geometry being
	// replaced. Carrying them over would let unrelated renegotiations on a
	// still screen, minutes apart, add up to one run.
	ud->unusable_run = 0;

	if (param == NULL)
		return;

	// Parse video format
	struct spa_video_info_raw format;
	if (spa_format_video_raw_parse(param, &format) < 0) {
		fprintf(stderr, "[PipeWire] Failed to parse video format\n");
		return;
	}

	// Store format info
	ud->frame_width = format.size.width;
	ud->frame_height = format.size.height;
	ud->frame_format = format.format;

	fprintf(stderr, "[PipeWire] Video format: %dx%d, format=%s\n",
		format.size.width, format.size.height,
		spa_debug_type_find_name(spa_type_video_format, format.format));
}

static void warn_drop_once(struct user_data *ud, enum drop_reason reason,
			   int stride, int height)
{
	if (ud->warned_drop[reason]) {
		return;
	}
	ud->warned_drop[reason] = 1;
	fprintf(stderr, "[PipeWire] Dropping frames: %s (stride %d, height %d)\n",
		drop_reason_text[reason], stride, height);
}

static int64_t monotonic_ns(void)
{
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (int64_t)ts.tv_sec * 1000000000 + ts.tv_nsec;
}

// A drop this long after the previous one starts a new run. Without it, a
// burst that no usable frame followed and a stray drop minutes later on a
// still screen would add up to one run. The cost is that a producer sending
// less often than this is never judged, which only a near-still screen does.
#define UNUSABLE_RUN_GAP_NS (5LL * 1000000000)

static void reject_buffer(struct user_data *ud, struct pw_buffer *b,
			  enum drop_reason reason, int stride, int height)
{
	warn_drop_once(ud, reason, stride, height);

	int64_t now = monotonic_ns();
	if (ud->unusable_run > 0 && now - ud->unusable_last_ns > UNUSABLE_RUN_GAP_NS) {
		ud->unusable_run = 0;
	}
	if (ud->unusable_run == 0) {
		ud->unusable_first_ns = now;
	}
	ud->unusable_run++;
	ud->unusable_last_ns = now;
	ud->unusable_reason = reason;

	pw_stream_queue_buffer(ud->stream, b);
}

static void on_stream_process(void *data)
{
	struct user_data *ud = data;
	struct pw_buffer *b;
	struct spa_buffer *buf;

	// Dequeue buffer
	b = pw_stream_dequeue_buffer(ud->stream);
	if (b == NULL) {
		return;
	}

	buf = b->buffer;

	// datas is a pointer, so an empty buffer makes datas[0] a read of
	// unallocated memory rather than of a zeroed struct.
	if (buf->n_datas == 0) {
		reject_buffer(ud, b, DROP_NO_DATA, 0, ud->frame_height);
		return;
	}

	// MAP_BUFFERS leaves DmaBuf unmapped unless the producer marks it
	// mappable, so a producer that moves to DmaBuf lands here every time.
	if (buf->datas[0].data == NULL) {
		reject_buffer(ud, b, DROP_UNMAPPED, buf->datas[0].chunk->stride,
			      ud->frame_height);
		return;
	}

	uint8_t *src = buf->datas[0].data;
	int stride = buf->datas[0].chunk->stride;

	// A corrupted chunk can still be full size and pass every bounds
	// check below, so nothing else would catch it: the garbage would be
	// extracted and streamed to the lights as if it were a frame. Only
	// CORRUPTED is rejected; EMPTY means a legitimately black frame.
	//
	// It leaves any unusable run alone. KWin sets the flag on every buffer
	// it sends without video, a cursor-only update for one, so it is the
	// producer working as intended, but it carries no frame to show that
	// the producer's frames are usable either.
	if (buf->datas[0].chunk->flags & SPA_CHUNK_FLAG_CORRUPTED) {
		warn_drop_once(ud, DROP_CORRUPTED, stride, ud->frame_height);
		pw_stream_queue_buffer(ud->stream, b);
		return;
	}

	uint32_t maxsize = buf->datas[0].maxsize;
	uint32_t claimed = buf->datas[0].chunk->size;

	// A data-less frame carries no new pixels. Leave the slot holding
	// what it already has rather than publishing anything. It comes ahead
	// of the geometry checks because producers leave the stride of such a
	// frame at zero, and like a corrupted chunk it leaves an unusable run
	// alone.
	if (maxsize == 0 || claimed == 0) {
		pw_stream_queue_buffer(ud->stream, b);
		return;
	}

	// chunk->offset and chunk->size are the producer's claims about a
	// mapping that is only maxsize bytes long, and buffer.h says they
	// "should be" clamped to it rather than that they are. Bound the
	// read by the mapping itself, or a producer overstating either one
	// walks the memcpy below off the end of the mmap.
	//
	// buffer.h specifies offset modulo maxsize, which is what a
	// ring-buffer producer relies on; rejecting an out-of-range offset
	// instead would drop every frame such a producer sends.
	uint32_t offset = buf->datas[0].chunk->offset % maxsize;
	uint32_t available = maxsize - offset;
	uint32_t usable = claimed < available ? claimed : available;
	if (usable > INT_MAX) {
		usable = INT_MAX;
	}
	src += offset;
	int size = (int)usable;

	// Calculate expected size
	int height = ud->frame_height;

	// chunk->stride is int32_t, and frame_height is still 0 until
	// param_changed fires. Either makes stride * height non-positive,
	// which the grow check below reads as "capacity is already big
	// enough" and memcpy then converts to a huge size_t. Reject the
	// frame before any of that arithmetic is used. The last term
	// rejects a product that would overflow int rather than wrap.
	//
	// A negative stride means a bottom-up buffer, which this converter
	// does not handle: reading it correctly needs a row-reversed copy,
	// not just a sign change. Such frames are dropped rather than
	// rendered upside down, so say so once instead of failing silently
	// until the circuit breaker stops capture.
	if (stride <= 0 || height <= 0 || height > INT_MAX / stride) {
		// stride > 0 && height > 0 by the time the last branch is
		// reached, so it can only be the overflow term above.
		enum drop_reason reason = DROP_OVERFLOW;
		if (stride < 0) {
			reason = DROP_BOTTOM_UP;
		} else if (height <= 0) {
			reason = DROP_NOT_NEGOTIATED;
		} else if (stride == 0) {
			reason = DROP_ZERO_STRIDE;
		}
		reject_buffer(ud, b, reason, stride, height);
		return;
	}
	int expected_size = stride * height;

	// A chunk shorter than the negotiated frame cannot be published
	// either way. Advertising the full height would stream the tail
	// realloc left uninitialized, and advertising only the whole rows
	// that arrived would shrink the frame's reported bounds, which
	// extractZoneColor reads as fractions of those bounds: a half-height
	// frame makes a light mapped to the bottom of the screen sample the
	// middle instead, silently and with nothing logged. Drop it.
	if (size < expected_size) {
		reject_buffer(ud, b, DROP_SHORT_CHUNK, stride, height);
		return;
	}

	// Always target the slot Go does not currently hold (see the
	// struct user_data comment for why this is race-free).
	int back_idx = 1 - ud->front_idx;
	struct frame_buf *fb = &ud->frame[back_idx];

	// Grow (never shrink) the back buffer if needed.
	if (fb->data == NULL || fb->capacity < expected_size) {
		uint8_t *grown = realloc(fb->data, expected_size);
		if (grown == NULL) {
			// Keep the old buffer and drop this frame rather than
			// leak or write past its end.
			reject_buffer(ud, b, DROP_ALLOC_FAILED, stride, height);
			return;
		}
		fb->data = grown;
		fb->capacity = expected_size;
	}
	// Every short or absent chunk was rejected above, so the slot only
	// ever describes a full frame it actually holds. Metadata is written
	// with that data and published by index last, so Go can never read
	// geometry that outruns the bytes behind it.
	memcpy(fb->data, src, (size_t)expected_size);
	fb->stride = stride;
	fb->width = ud->frame_width;
	fb->height = height;
	fb->format = ud->frame_format;
	ud->ready_idx = back_idx;
	ud->frame_seq++;
	ud->unusable_run = 0;

	// Return buffer to PipeWire
	pw_stream_queue_buffer(ud->stream, b);
}

static const struct pw_stream_events stream_events = {
	PW_VERSION_STREAM_EVENTS,
	.state_changed = on_stream_state_changed,
	.param_changed = on_stream_param_changed,
	.process = on_stream_process,
};

// Helper function to create and connect stream
struct user_data* pw_stream_connect_to_node(uint32_t node_id) {
	struct pw_thread_loop *loop;
	struct pw_stream *stream;
	struct user_data *ud;
	const struct spa_pod *params[1];
	uint8_t buffer[1024];
	struct spa_pod_builder b = SPA_POD_BUILDER_INIT(buffer, sizeof(buffer));

	// Initialize PipeWire
	pw_init(NULL, NULL);

	// Create thread loop (for multi-threaded applications)
	loop = pw_thread_loop_new("khuey-pipewire", NULL);
	if (!loop) {
		fprintf(stderr, "[PipeWire] Failed to create thread loop\n");
		return NULL;
	}

	// Allocate user data
	ud = calloc(1, sizeof(struct user_data));
	if (!ud) {
		pw_thread_loop_destroy(loop);
		return NULL;
	}
	ud->loop = loop;
	ud->ready_idx = -1; // no frame captured yet

	// Lock the loop before creating stream
	pw_thread_loop_lock(loop);

	// Create stream
	stream = pw_stream_new_simple(
		pw_thread_loop_get_loop(loop),
		"khuey-screen-capture",
		pw_properties_new(
			PW_KEY_MEDIA_TYPE, "Video",
			PW_KEY_MEDIA_CATEGORY, "Capture",
			PW_KEY_MEDIA_ROLE, "Screen",
			NULL),
		&stream_events,
		ud);

	if (!stream) {
		fprintf(stderr, "[PipeWire] Failed to create stream\n");
		pw_thread_loop_unlock(loop);
		free(ud);
		pw_thread_loop_destroy(loop);
		return NULL;
	}
	ud->stream = stream;

	// Build format parameters - accept any video format
	params[0] = spa_pod_builder_add_object(&b,
		SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat,
		SPA_FORMAT_mediaType, SPA_POD_Id(SPA_MEDIA_TYPE_video),
		SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
		SPA_FORMAT_VIDEO_format, SPA_POD_CHOICE_ENUM_Id(5,
			SPA_VIDEO_FORMAT_BGRx,
			SPA_VIDEO_FORMAT_RGBx,
			SPA_VIDEO_FORMAT_BGRA,
			SPA_VIDEO_FORMAT_RGBA,
			SPA_VIDEO_FORMAT_BGR),
		0);

	// Connect stream to specific node
	if (pw_stream_connect(stream,
			      PW_DIRECTION_INPUT,
			      node_id,
			      PW_STREAM_FLAG_AUTOCONNECT |
			      PW_STREAM_FLAG_MAP_BUFFERS,
			      params, 1) < 0) {
		fprintf(stderr, "[PipeWire] Failed to connect stream to node %u\n", node_id);
		pw_stream_destroy(stream);
		pw_thread_loop_unlock(loop);
		free(ud);
		pw_thread_loop_destroy(loop);
		return NULL;
	}

	// Unlock before starting the loop
	pw_thread_loop_unlock(loop);

	fprintf(stderr, "[PipeWire] Stream connected to node %u\n", node_id);
	return ud;
}

// Start the thread loop (non-blocking)
int pw_start_loop(struct user_data *ud) {
	if (ud && ud->loop) {
		int res = pw_thread_loop_start(ud->loop);
		if (res == 0) {
			ud->loop_started = 1;
		}
		return res;
	}
	return -1;
}

// Stop the thread loop. Teardown now runs even when the loop was never
// started (see NativePipeWireCapture.Stop), so this must be a no-op in that
// case rather than stopping a loop that was only ever created.
void pw_stop_loop(struct user_data *ud) {
	if (ud && ud->loop && ud->loop_started) {
		pw_thread_loop_stop(ud->loop);
		ud->loop_started = 0;
	}
}

// Cleanup
void pw_cleanup(struct user_data *ud) {
	if (ud) {
		if (ud->loop) {
			pw_thread_loop_lock(ud->loop);
		}
		if (ud->stream) {
			pw_stream_destroy(ud->stream);
		}
		if (ud->loop) {
			pw_thread_loop_unlock(ud->loop);
			pw_thread_loop_destroy(ud->loop);
		}
		free(ud->frame[0].data);
		free(ud->frame[1].data);
		free(ud);
	}
	pw_deinit();
}

// Get frame data. Returns 1 with the whole of *status set when a slot is
// available, 0 when the stream has not produced one yet, and -1 once the
// stream has failed. Everything but seq is set on both of the first two, so a
// caller waiting for the first frame can tell a stream that is up from one
// that never came up.
int pw_get_frame(struct user_data *ud, uint8_t **data, int *width, int *height,
                 int *stride, uint32_t *format, struct frame_status *status) {
	if (ud == NULL) {
		return 0;
	}

	// Lock the thread loop for thread-safe access
	pw_thread_loop_lock(ud->loop);

	if (ud->stream_failed) {
		pw_thread_loop_unlock(ud->loop);
		return -1;
	}

	// Set before the early return as well: the caller needs to tell a stream
	// that is up and has simply sent nothing yet from one that never came up.
	status->streaming = ud->streaming;
	status->unusable_run = ud->unusable_run;
	status->unusable_span_ns = 0;
	status->unusable_reason = NULL;
	if (ud->unusable_run > 0) {
		status->unusable_span_ns = ud->unusable_last_ns - ud->unusable_first_ns;
		status->unusable_reason = drop_reason_text[ud->unusable_reason];
	}

	if (ud->ready_idx < 0) {
		pw_thread_loop_unlock(ud->loop);
		return 0;
	}

	// Claim the most recently completed slot as the new front. From this
	// point on, on_stream_process (which recomputes its write target as
	// 1 - front_idx on every call) will never touch it again until this
	// function is called a second time and moves front_idx elsewhere.
	int claimed = ud->ready_idx;
	ud->front_idx = claimed;

	// Report the slot's own dimensions, captured when it was written, not
	// the current ud->frame_width/height/format: those can have moved on
	// to a new value (via on_stream_param_changed) since this slot's last
	// write, and pairing this slot's data with a newer/larger size would
	// let the caller read past what was actually allocated for it.
	struct frame_buf *fb = &ud->frame[claimed];
	*data = fb->data;
	*width = fb->width;
	*height = fb->height;
	*stride = fb->stride;
	*format = fb->format;
	status->seq = ud->frame_seq;

	pw_thread_loop_unlock(ud->loop);

	return 1;
}
