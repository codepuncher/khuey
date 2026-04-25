# Contributing to KDE Hue Control

Thank you for your interest in contributing to KHuey! This guide will help you set up your development environment and understand our development workflow.

## Development Setup

### Prerequisites

- **Go 1.23.0+**: For backend development
- **Qt6 & KDE Frameworks 6**: For tray application
- **CMake 3.16+**: For building the tray app
- **Philips Hue Bridge**: For testing (optional for some changes)

### Initial Setup

1. **Clone the repository:**
   ```bash
   git clone <repo-url>
   cd khuey
   ```

2. **Install dependencies:**
   ```bash
   # Install Go dependencies
   cd backend
   go mod download

   # Qt6 and KDE Frameworks should already be installed on KDE Plasma 6
   ```

3. **Configure Hue bridge (optional):**
   ```bash
   # If you have a Hue bridge
   openhue setup
   ```

4. **Set up Git hooks:**
   ```bash
   # Install lefthook
   go install github.com/evilmartians/lefthook/v2@latest

   # Install hooks into your local repository
   lefthook install
   ```

## Git Hooks (Lefthook)

This project uses [Lefthook](https://github.com/evilmartians/lefthook) for Git hooks to ensure code quality and consistency.

### What the hooks do

#### Pre-commit (runs on `git commit`)
- **go-fmt**: Auto-formats Go code with `gofmt`
- **go-test**: Runs tests for modified Go packages (short mode)
- **go-vet**: Checks for common Go mistakes
- **cpp-format**: Auto-formats C++ code with `clang-format`
- **markdown-lint**: Basic markdown validation
- **shellcheck**: Validates shell scripts (if shellcheck installed)

All pre-commit checks run in parallel for speed (~5 seconds typical).

#### Pre-push (runs on `git push`)
- **go-test-all**: Full Go test suite with verbose output
- **go-build**: Ensures backend compiles
- **cpp-build**: Ensures tray app compiles

Pre-push hooks are skipped on the `main` branch to avoid blocking PR merges.

#### Commit-msg (runs on `git commit`)
- Validates commit message format (minimum 10 characters, warns if >72)
- Reminds about `Co-authored-by: Copilot` trailer (required by project)

### Skip hooks temporarily

Sometimes you need to bypass hooks (use sparingly):

```bash
# Skip pre-commit hooks for this commit
LEFTHOOK=0 git commit -m "message"

# Skip all hooks
git commit --no-verify -m "message"
```

### Customize for your workflow

Create a `lefthook-local.yml` file (not committed to git) to override settings:

```yaml
# Example: Skip slow tests locally
pre-commit:
  commands:
    go-test:
      skip: true  # Run tests manually instead

# Example: Skip pre-push builds (run manually)
pre-push:
  skip: true
```

### Optional: Install additional tools

For enhanced linting:

```bash
# Go linting (comprehensive static analysis)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Shell script validation
sudo pacman -S shellcheck  # Arch/Manjaro
# or
snap install shellcheck
```

After installing golangci-lint, uncomment the `go-lint` section in `lefthook.yml`.

## Development Workflow

### Continuous Integration

All pull requests automatically run CI checks via GitHub Actions:
- **Backend tests** - Full Go test suite with race detection
- **Backend build** - Ensure code compiles
- **Tray app build** - Ensure Qt/C++ code compiles
- **Linting** - golangci-lint code quality checks
- **Code formatting** - go fmt validation
- **Test coverage** - Ensure coverage doesn't drop below 10%

**PRs must pass all CI checks before merging.** You can view CI status on the PR page.

### Standard Branch-Based Workflow

All changes must use feature branches and pull requests. Never commit directly to `main`.

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make your changes:**
   - Write code following existing conventions
   - Add tests for new functionality
   - Update documentation as needed

3. **Test locally:**
   ```bash
   # Run backend tests
   cd backend
   go test ./...

   # Run specific package tests
   go test ./internal/config -v

   # Build backend
   go build -o hue-sync ./cmd/hue-sync

   # Build tray app
   cd ../trayapp
   cmake . && make
   ```

4. **Commit with descriptive messages:**
   ```bash
   git add .
   git commit -m "Add feature X to improve Y

   - Implemented Z
   - Updated documentation
   - Added tests

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
   ```

5. **Push and create PR:**
   ```bash
   git push origin feature/my-new-feature
   # Then create a PR on GitHub
   ```

### Commit Message Format

- **First line**: Brief summary (10-72 characters ideal)
- **Body**: Detailed explanation if needed (wrap at 72 characters)
- **Trailers**: Always include `Co-authored-by: Copilot` if using GitHub Copilot

Example:
```
Add Entertainment API screen sync support

- Implemented Wayland/Pipewire screen capture
- Added UV coordinate zone mapping for multi-light setups
- Integrated DTLS streaming for Entertainment API v2
- Added configuration options for FPS and subsampling

Closes #42

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

## Code Style

### Go Code

- Follow standard `gofmt` formatting (enforced by pre-commit hook)
- Use `go vet` recommendations (enforced by pre-commit hook)
- Write tests for new functionality
- Keep test coverage above 15% overall (current baseline)

### C++ Code

- Follow LLVM style (configured in `.clang-format`)
- 4-space indentation, no tabs
- 100 character line limit
- Use modern C++17 features

### Configuration Files

- YAML files: 2-space indentation
- Markdown: 80-100 character line limit, no trailing whitespace

## Testing

### Backend Tests

```bash
cd backend

# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/config -v

# Run specific test
go test ./internal/config -v -run TestDefaultConfig
```

### Test Utilities

The backend includes test utilities in `backend/cmd/`:

```bash
# Test screen capture
go run ./cmd/test-capture

# Test color extraction
go run ./cmd/test-color

# Test Entertainment API streaming
go run ./cmd/test-entertainment

# Visualize zone mapping
go run ./cmd/test-zones-visual
```

### Manual Integration Testing

```bash
# Start backend manually
./backend/hue-sync &

# Test DBus methods
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus

# Monitor DBus traffic
dbus-monitor --session "interface='org.kde.plasma.hue'"

# Restart tray app
./trayapp/hue-tray &
```

## Project Structure

```
khuey/
├── backend/              # Go backend
│   ├── cmd/             # Executables
│   │   ├── hue-sync/    # Main backend service
│   │   └── test-*/      # Test utilities
│   ├── internal/        # Internal packages
│   │   ├── config/      # YAML config management
│   │   ├── hue/         # Hue API client wrapper
│   │   ├── dbus/        # DBus service
│   │   ├── capture/     # Wayland/Pipewire screen capture
│   │   ├── sync/        # Entertainment API streaming
│   │   ├── entertainment/ # DTLS Entertainment API
│   │   └── color/       # Color extraction
│   └── go.mod
├── trayapp/             # Qt6/C++ tray application
│   ├── main.cpp
│   └── CMakeLists.txt
├── systemd/             # Service files
│   ├── hue-backend.service
│   └── hue-tray.desktop
├── scripts/             # Utility scripts
├── lefthook.yml         # Git hooks configuration
└── .clang-format        # C++ formatting rules
```

## Architecture

### Three-Component Design

```
┌──────────────┐      DBus Session Bus      ┌─────────────┐
│  Qt Tray App │◄────────────────────────────►│ Go Backend  │
│ (hue-tray)   │  org.kde.plasma.hue         │ (hue-sync)  │
└──────────────┘                              │             │
                                              │  • Config   │
                                              │  • Hue API  │
                                              │  • Sync     │
                                              └──────┬──────┘
                                                     │ HTTPS/UDP
                                                     ▼
                                              ┌─────────────┐
                                              │ Hue Bridge  │
                                              └─────────────┘
```

### Key Components

1. **Backend (hue-sync)**: Go service managing Hue API, DBus communication, and Entertainment streaming
2. **Tray App (hue-tray)**: Qt6/C++ system tray UI using KStatusNotifierItem
3. **DBus Interface**: Communication between tray app and backend (`org.kde.plasma.hue`)

## Common Development Tasks

### Adding a New DBus Method

1. **Add method to backend:**
   ```go
   // internal/dbus/service.go
   func (s *Service) MyMethod(param string) (bool, *dbus.Error) {
       // Implementation
       return true, nil
   }
   ```

2. **Update introspection XML** in same file

3. **Call from tray app:**
   ```cpp
   // trayapp/main.cpp
   iface.call("MyMethod", "param_value");
   ```

### Adding a Config Option

1. **Add field to config struct:**
   ```go
   // internal/config/config.go
   type Config struct {
       MyOption string `mapstructure:"myOption"`
   }
   ```

2. **Update DefaultConfig()** with default value

3. **Use in code:**
   ```go
   cfg, _ := config.Load()
   value := cfg.MyOption
   ```

### Testing DBus Changes

```bash
# Start backend
./backend/hue-sync &

# Introspect interface
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.freedesktop.DBus.Introspectable.Introspect

# Test method call
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.MyMethod string:"test"
```

## Troubleshooting Development Issues

### Backend won't start
```bash
# Check if already running
ps aux | grep hue-sync

# View error logs
journalctl --user -u plasma-hue-backend -n 50

# Verify config exists
cat ~/.openhue/config.yaml
```

### DBus errors
```bash
# Verify service is registered
dbus-send --session --dest=org.freedesktop.DBus \
  --print-reply /org/freedesktop/DBus \
  org.freedesktop.DBus.ListNames | grep hue
```

### Build failures
```bash
# Clean build artifacts
cd backend && go clean
cd ../trayapp && rm -rf CMakeCache.txt CMakeFiles/ Makefile

# Rebuild from scratch
cd backend && go build -o hue-sync ./cmd/hue-sync
cd ../trayapp && cmake . && make
```

## Getting Help

- **Documentation**: See README.md, TESTING.md, DEVELOPMENT.md
- **Issues**: Check existing GitHub issues or create a new one
- **Architecture questions**: See the KHuey Expert agent documentation in `.github/agents/`

## License

This project is licensed under the terms specified in the LICENSE file.
