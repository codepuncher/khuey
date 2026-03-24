# Changelog

## [Unreleased]

### Added - Power & Brightness Controls (2026-03-24)
- **Backend**: Added grouped light (room/zone) configuration support
- **Backend**: Implemented `GetGroupedLights()` to fetch available rooms/zones
- **Backend**: Updated `SetPower()` and `SetBrightness()` methods
- **Backend**: Added `GetGroupedLights` DBus method
- **Frontend**: Enabled power checkbox control
- **Frontend**: Enabled brightness slider (0-100%)
- **Frontend**: Added "Select Room/Zone" settings button
- **Frontend**: Implemented grouped light selection dialog
- **Config**: Added `grouped_light_id` field to config.yaml

### Changed
- Removed "Not yet implemented" labels from power and brightness controls
- Controls are now fully functional with configured grouped light

### Fixed
- Power and brightness controls now work when grouped light is configured

## [0.1.0] - 2026-03-23

### Added
- Initial implementation of KDE Hue control system
- Go backend with openhue-go integration
- DBus service for IPC
- Qt tray application with KStatusNotifierItem
- Scene control with room names
- Alphabetical sorting of scenes
- Auto-start via systemd and KDE
- Desktop notifications
- Clean repository with proper git attribution

