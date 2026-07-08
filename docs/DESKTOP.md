# Desktop Standalone App

Jabali Sounder supports a local standalone desktop architecture using Wails.
The desktop app embeds the existing React UI, runs the existing Gin API in the
same process, and stores local state in SQLite.

This keeps the server deployment intact while adding a Windows/macOS/Linux
runtime.

## Architecture

```text
Wails WebView
  |
  | /api/v1 and /health
  v
Embedded Gin API
  |
  v
SQLite database in the user's config directory
```

The Wails entrypoint is:

```text
manager-api/cmd/desktop
```

It is guarded by:

```go
//go:build desktop
```

Normal server builds and tests do not compile Wails code.

## Local Data

The desktop app creates local files under the OS user config directory:

```text
Jabali Sounder/
  sounder.db
  secrets.key
  jwt.secret
```

On first launch:

- `sounder.db` is created and migrated with GORM AutoMigrate.
- `secrets.key` is generated as a 32-byte AES key.
- `jwt.secret` is generated as a random hex secret.
- The login screen switches to first-run setup if no admin exists.

## First-Run Setup

The API exposes:

```text
GET  /api/v1/auth/setup
POST /api/v1/auth/setup
```

Setup is only available when the `admins` table is empty. Once an admin exists,
`POST /api/v1/auth/setup` returns HTTP 409.

This avoids shipping a default desktop password.

## Build Targets

Stage the current React build into the desktop embed directory:

```bash
make desktop-stage
```

Build a desktop binary for the current OS:

```bash
make desktop-build
```

The output is:

```text
bin/jabali-sounder-desktop
```

## Wails Tooling

Install the Wails CLI on the build machine:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Check local platform dependencies:

```bash
wails doctor
```

For Windows, macOS, and Linux releases, build on the target OS or use a CI
runner for that OS. Wails depends on each platform's native WebView stack:

- Windows: WebView2.
- macOS: WKWebView.
- Linux: WebKitGTK.

## Current State

Implemented:

- SQLite database driver support.
- Wails desktop entrypoint behind the `desktop` build tag.
- Embedded SPA staging target.
- Local secrets and JWT secret generation.
- First-run admin setup endpoint and UI.

Still needed before production desktop distribution:

- Wails project/release packaging metadata.
- macOS signing/notarization.
- Windows signing.
- Desktop auto-update strategy.
- End-to-end packaging tests on Windows, macOS, and Linux.
