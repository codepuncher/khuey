# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please send an email to the repository owner with:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if available)

We will respond within 48 hours and work with you to understand and address the issue.

## Security Considerations

### Configuration File Security

The configuration file (`~/.openhue/config.yaml` by default; see "Configuration File Location" in `docs/CONFIGURATION.md`) contains sensitive data:
- Hue Bridge API keys
- Entertainment API client keys
- Network information

**The application will warn if config file permissions are too permissive** and should be set to `0600` (owner read/write only):

```bash
chmod 600 ~/.openhue/config.yaml
```

### DBus Service

The DBus service (`org.kde.plasma.hue`) runs on the session bus and is accessible to all processes in the user session. This is by design for desktop integration, but means any application running as the same user can control the lights.

### Screen Capture

The screen sync feature requires screen capture permissions via XDG Desktop Portal. The first time sync is activated, the user must approve screen sharing through a GUI dialog. A restore token is saved to avoid showing the dialog on subsequent syncs.

### Network Security

- All communication with the Hue Bridge uses HTTPS (REST API)
- Entertainment API uses DTLS 1.2 over UDP for secure streaming
- Bridge API keys are generated during initial setup and should be kept secure
