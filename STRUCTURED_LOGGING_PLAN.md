# Structured Logging Migration Plan (Post-v1.0)

## Issue: QUAL-009 - Structured Logging Migration

**Status**: Deferred to post-v1.0.0 release (low-priority, low-risk enhancement)

## Current State

The backend currently uses `log.Printf()` for all logging, which works well but lacks:
- Structured fields for machine parsing
- Log levels (info/warn/error distinction in output format)
- Integration with modern log aggregation tools

## Post-v1.0 Migration Plan

### Goal
Migrate from `log.Printf` to Go's standard `log/slog` package (available since Go 1.21).

### Benefits
- Structured logging with key-value pairs
- Proper log levels (Info/Warn/Error/Debug)
- Better systemd journal integration
- Machine-parseable output for log aggregation
- Zero external dependencies (standard library)

### Example Migration

**Before**:
```go
log.Printf("Starting sync at %d FPS", fps)
log.Printf("⚠️  Frame skip: dropped %d frames", count)
```

**After**:
```go
slog.Info("Starting sync", "fps", fps)
slog.Warn("Frame skip", "dropped_frames", count)
```

### Implementation Steps

1. **Add slog handler configuration** in `cmd/hue-sync/main.go`:
   ```go
   logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
       Level: slog.LevelInfo,
   }))
   slog.SetDefault(logger)
   ```

2. **Migrate log calls package by package**:
   - `internal/dbus/service.go` - DBus service logs
   - `internal/sync/engine.go` - Sync loop logs
   - `internal/capture/capture.go` - Screen capture logs
   - `internal/entertainment/client.go` - Entertainment API logs
   - `cmd/hue-sync/main.go` - Main application logs

3. **Preserve systemd journal compatibility**:
   - Use `slog.NewTextHandler` for human-readable systemd logs
   - Consider `slog.NewJSONHandler` for machine parsing if needed

4. **Update log level usage**:
   - `slog.Info()` - Normal operations (start/stop/status)
   - `slog.Warn()` - Recoverable errors (frame drops, temporary failures)
   - `slog.Error()` - Serious errors (connection failures, invalid config)
   - `slog.Debug()` - Verbose debugging (disabled by default)

### Testing Requirements

- Verify logs appear correctly in `journalctl --user -u plasma-hue-backend`
- Ensure no performance regression (slog is optimized)
- Confirm systemd service logs are still readable

### Why Deferred?

- **Low risk of bugs**: Current logging works perfectly
- **Not user-facing**: Users interact via tray app, not logs
- **Post-v1.0 stability**: Avoid pre-release churn
- **Low priority**: Other features provide more value

### Estimated Effort

- 1-2 hours to migrate all log calls
- Low risk, high confidence (standard library)
- Good candidate for first post-v1.0 enhancement

## Related Issues

- QUAL-006: Error message consistency (✅ DONE - emoji standardization)
- PERF-* issues: Performance optimizations (✅ DONE)

## Notes

The current `log.Printf` approach is perfectly acceptable for v1.0. This migration is purely
an enhancement for better observability and doesn't fix any bugs or add user-visible features.
