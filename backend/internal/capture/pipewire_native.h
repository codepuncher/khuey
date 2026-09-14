#ifndef KHUEY_PIPEWIRE_NATIVE_H
#define KHUEY_PIPEWIRE_NATIVE_H

#include <stdint.h>

struct user_data;

struct frame_status {
	// A caller that sees the same seq twice has been handed the same pixels
	// twice.
	uint64_t seq;
	int streaming;
	uint64_t unusable_run;
	int64_t unusable_span_ns; // first to last buffer of the run
	const char *unusable_reason; // the latest buffer's; NULL with no run
};

struct user_data *pw_stream_connect_to_node(uint32_t node_id);
int pw_start_loop(struct user_data *ud);
void pw_stop_loop(struct user_data *ud);
void pw_cleanup(struct user_data *ud);
int pw_get_frame(struct user_data *ud, uint8_t **data, int *width, int *height,
                 int *stride, uint32_t *format, struct frame_status *status);

#endif
