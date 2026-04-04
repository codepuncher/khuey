# Changelog

All notable changes to KDE Hue Control are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Nothing yet.

## [1.0.0] - 2026-04-04

Production-ready release with comprehensive security, performance, and quality improvements.

### Added
- **Settings Dialog**: Comprehensive settings UI with bridge configuration, room selection, and screen sync controls
- **Screen Sync**: Real-time screen color synchronization with Hue lights using native PipeWire capture
- **Zone Mapping**: Configurable UV-based screen zones for multi-light setups
- **Power & Brightness Controls**: Room/zone-level light control with grouped light selection
- **Rate Limiting**: Configurable rate limiting (10 req/sec) to protect Hue bridge
- **Input Validation**: DBus string validation (length and UTF-8)
- **Access Control**: UID-based DBus access control for security
- **Performance Monitoring**: Frame drop detection and performance logging
- **Buffer Pooling**: Image buffer pooling to reduce GC pressure
- **Build Automation**: Lefthook pre-commit/pre-push hooks for code quality

### Changed
- **Image Conversion**: Optimized pixel conversion using `draw.Draw()` (~100x faster)
- **Startup Time**: Reduced from 500ms to 100ms
- **HTTP Client**: Reusable HTTP client instead of per-request creation
- **Context Propagation**: Proper context hierarchies throughout backend
- **Error Handling**: Comprehensive error wrapping and logging
- **Crypto**: Switched from `math/rand` to `crypto/rand` for session tokens

### Fixed
- Config file permissions set to 0600 (was 0644, exposed API keys)
- Resource leaks: DBus connections, contexts, zombie processes, temp files
- Qt memory leaks: KNotification objects now properly parented
- Screen sync restart crashes
- Room dropdown initialization in settings dialog
- Config validation now runs on load

### Security
- Crypto-secure random number generation for all tokens
- Strict file permissions on config files (API key protection)
- TLS validation documented (self-signed cert rationale)
- DBus UID-based access control
- Rate limiting to prevent bridge DoS
- Input validation on all external inputs
- Log sanitization for sensitive data

### Performance
- 100x faster image conversion (draw.Draw)
- HTTP client connection reuse
- Slice pre-allocation (~30 fewer allocations/sec @ 30 FPS)
- Image buffer pooling (reduced GC pauses)
- Native PipeWire capture (no GStreamer dependency)

### Documentation
- Complete API documentation (Godoc)
- Constants reference guide
- Screen sync quickstart guide
- Zone mapping quick reference
- Git commit guidelines
- Contributing guide
- Development guide
- Testing guide

