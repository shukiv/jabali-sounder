# Desktop Standalone App

Jabali Sounder supports a local standalone desktop architecture using Wails.
The desktop app embeds the existing React UI, runs the existing Gin API in the
same process, and stores local state in SQLite.

This keeps the server deployment intact while adding a Windows/macOS/Linux
runtime.

## Installers vs. the headless server

There are **two different downloads** — do not confuse them:

| You want | Download | Docs |
|----------|----------|------|
| The **desktop app** (GUI, own local database) | a desktop **installer** or portable binary below | this file |
| A **headless server** on a box (the `curl \| bash` one-liner) | `jabali-sounder-server-linux-amd64` | [DOCKER.md](DOCKER.md) / install.sh |

The `install.sh` one-liner installs the **server**, not this desktop app.

## Desktop installers (SND-95)

Each tagged release publishes OS-integrated installers alongside the raw
portable binaries. All are covered by `checksums.txt`.

| OS | Artifact | Install |
|----|----------|---------|
| Windows 10/11 (x64) | `jabali-sounder-setup-<ver>-amd64.exe` | Run it. Per-user install (no admin) to `%LOCALAPPDATA%\Programs\JabaliSounder`; adds a Start-menu entry (desktop shortcut optional) and an Apps & Features entry. |
| Debian/Ubuntu (amd64) | `jabali-sounder_<ver>_amd64.deb` | `sudo apt install ./jabali-sounder_<ver>_amd64.deb` — pulls WebKitGTK/GTK deps. |
| Fedora (x86_64) | `jabali-sounder-<ver>-1.x86_64.rpm` | `sudo dnf install ./jabali-sounder-<ver>-1.x86_64.rpm` |

Portable binaries (`jabali-sounder-<os>-amd64-<ver>[.exe]`) remain published for
users who prefer to place and launch them manually, plus the macOS `.dmg`.

### Prerequisites

- **Windows:** WebView2 runtime (present on current Windows 10/11).
- **Linux:** **WebKitGTK 4.1** + GTK 3. The `.deb` (Debian/Ubuntu) declares
  `libwebkit2gtk-4.1-0` + `libgtk-3-0`; the `.rpm` (Fedora) declares
  `webkit2gtk4.1` + `gtk3`. The package manager installs them for you.

  > **Supported RPM distro:** Fedora. RHEL/AlmaLinux/Rocky 9 ship only WebKitGTK
  > **4.0** (`webkit2gtk3`), and openSUSE names the package differently
  > (`libwebkit2gtk-4_1-0`), so the current `.rpm` does not resolve there. Use the
  > portable binary on those distros (with a WebKitGTK 4.1 runtime), or track the
  > follow-up for a 4.0 build variant.

### Upgrade

Install the newer package/installer over the old one. Program files are replaced
in place; your **data directory is untouched**, so the database, `secrets.key`,
`jwt.secret`, enrolled servers, preferences, and login state are preserved. The
desktop self-updater continues to update the installed binary in place.

### Uninstall

- **Windows:** Apps & Features → Jabali Sounder → Uninstall (or run
  `uninstall.exe` in the install dir).
- **Linux:** `sudo apt remove jabali-sounder` / `sudo dnf remove jabali-sounder`.

Uninstall removes program files, shortcuts, and OS registrations but **keeps your
user data by default** (see [Local Data](#local-data)). To also remove data,
delete the `Jabali Sounder/` directory under your OS user-config directory
yourself — that step is deliberately manual.

### Reinstall / repair

Reinstalling restores missing program files and shortcuts. It reuses the same
install location and the same per-user data directory, so it never creates a
second database or a duplicate application registration.

### Reproducibility & security

Installers are built in the release workflow from a clean checkout, tied to the
release version, and published with checksums. They ship **no** default
credentials, pre-created database, API keys, or machine-specific secrets — all of
that is generated on first launch (see First-Run Setup). Code signing and macOS
notarization are out of scope.

> Package builds are exercised locally (deb/rpm layout + NSIS compile); on-device
> clean-install/upgrade/repair/uninstall smoke tests on Windows 10/11, Debian/
> Ubuntu, and an RPM distro still require dedicated runners/VMs.

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
