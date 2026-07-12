# Mobile (iOS & Android)

Jabali Sounder ships as native iOS and Android apps built with **Wails v3**.
There is **no separate mobile codebase**: the same Go backend and the same React
SPA that power the desktop app also run on phones. This document explains the
architecture, the build matrix, and the toolchain each target needs.

## Why Wails v3

The desktop app was already a Wails app (v2). Wails **v3** adds first-class iOS
and Android targets that compile the *same* `main.go`: the Go code becomes a C
shared library, an OS WebView renders the existing frontend, and
`@wailsio/runtime` bindings (service calls, events, dialogs) work unchanged. So
mobile reuses ~100% of the UI and the entire `internal/*` backend — we add
platform wiring, not a second product.

The trade-off: v3 is pre-1.0 (alpha), and the desktop app had to be migrated
from v2 → v3 first (see [Migration](#migration-v2--v3)).

## Architecture

### One binary, three platforms

```
manager-ui/ (React SPA)  ─┐
internal/*  (Go backend) ─┼─► cmd/desktop/main.go  ─► desktop (win/mac/linux)
                          ├─► same main.go +android ─► libwails.so  ─► Android APK/AAB
                          └─► same main.go +ios     ─► libwails     ─► iOS .app/.ipa
```

Platform-specific behaviour lives in per-OS Go files guarded by build tags
(`//go:build android`, `//go:build ios`, `//go:build desktop`). Shared code
compiles on all three.

### The gin router *is* the asset handler

The desktop build runs the full API (gin) in-process and points the WebView at
it. Wails v3's `application.AssetOptions.Handler` accepts any `http.Handler`, so
the existing combined handler — `/api/v1/*` + `/health` → gin, everything else →
the embedded SPA — is passed straight in. This matters on mobile: **there is no
localhost server and no open ports**; the WebView loads assets and calls the API
in-process through that same handler. The SPA's `fetch("/api/v1/...")` keeps
working verbatim on every platform.

### Local data

Each install is self-contained with a local SQLite database, a generated secret
key, and a JWT secret — exactly like the desktop build. On mobile these live in
the app sandbox:

| Platform | Data dir |
|----------|----------|
| Desktop  | `os.UserConfigDir()/Jabali Sounder` |
| Android  | app files dir (`getFilesDir()`) |
| iOS      | `application.IOS.StoragePath()` (Application Support) |

### Native features → existing systems

| Native capability | Wails v3 API | Wired to |
|-------------------|--------------|----------|
| Push notifications | `PostNotification` / `common:notification` event | the alerting/incident system (M5) |
| Share sheet | `IOS.Share` / Android intent | audit/report CSV export |
| Haptics | `IOS.Haptic` / `Android.Haptics.Vibrate` | action confirmations |
| Biometric unlock | `BiometricAuthenticate` → `common:biometric` | app lock (optional) |
| Secure storage | `IOS.SecureSet` / Keystore | session token at rest |

## Build matrix & toolchains

| Target | Build host | Toolchain | Buildable on the current Linux dev box? |
|--------|-----------|-----------|------------------------------------------|
| Desktop Linux | Linux | Go + `-tags gtk3` (webkit2gtk-4.1) or GTK4/webkitgtk-6.0 | ✅ yes |
| Desktop Windows | Windows (or cross) | Go + webview2 | ⚠️ CI |
| Desktop macOS | macOS | Go + Xcode CLT | ❌ needs Mac |
| **Android** | Linux/macOS | Android SDK (API 35) + NDK 26.3 + JDK 21 + Go | ✅ yes (`make android-apk`) |
| **iOS** | **macOS only** | full Xcode + Go | ❌ needs Mac |

Notes:

- **Linux desktop** builds today with `-tags gtk3` against the installed
  `webkit2gtk-4.1`. The GTK3 legacy path is supported through the v3.0.x line
  and removed in v3.1; moving to GTK4 later just means installing
  `libgtk-4-dev libwebkitgtk-6.0-dev`.
- **Android** needs `sdkmanager "platform-tools" "platforms;android-35"
  "build-tools;35.0.0" "ndk;26.3.11579264"` and `ANDROID_HOME` set. The dev box
  has all of these — `make android-apk` produces a working APK.
- **iOS** cannot be built or tested on Linux at all — it requires macOS with the
  full Xcode (command-line tools alone are insufficient). iOS code is written
  here and built on a Mac.

## Building the APK (validated on Linux)

The repo carries a self-contained gradle project under `build/android`. A debug
APK is one command (needs the Android SDK API 35 + NDK 26.3 + JDK 21):

```bash
make android-apk          # -> bin/jabali-sounder.apk
```

Under the hood this: builds the SPA and stages it where the Go lib embeds it
(`//go:embed dist`); cross-compiles the shared `main` package to
`libwails.so` for `arm64-v8a` and `x86_64` with the NDK clang
(`-buildmode=c-shared -tags android`); then runs `gradlew assembleDebug`, which
packages both libs + the Java WebView host into the APK. The produced APK is
`com.jabali.sounder` (label "Jabali Sounder"), min SDK 21, target SDK 35.

For a Play-ready bundle, use the wails3 tasks below (`.aab`, signed).

## Commands (Wails v3 CLI)

```bash
wails3 doctor                     # show what the toolchain finds

# Android
wails3 task android:run           # emulator debug build + launch
wails3 task android:run:device    # physical device (DEVICE_ID=<serial>)
wails3 task android:bundle:fat    # Play-ready .aab (arm64 + x86_64)
wails3 task android:logs          # stream logcat

# iOS (on macOS)
wails3 task ios:run               # simulator
wails3 task ios:package IOS_PLATFORM=device \
    CODESIGN_IDENTITY="Apple Development: You (TEAMID)" \
    PROVISIONING_PROFILE=path/to/profile.mobileprovision
wails3 task ios:package:ipa IOS_PLATFORM=device ...   # distribution .ipa
```

Store formats: Google Play requires an `.aab` targeting API 35+; the App Store
requires an `.ipa` signed with an Apple Developer account.

## Signing & release

- **Android:** set `ANDROID_KEYSTORE_FILE`, `ANDROID_KEYSTORE_PASSWORD`,
  `ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD` before `android:bundle:fat`.
  Without them the bundle is debug-signed and Play rejects it.
- **iOS:** entitlements in `build/ios/entitlements.plist`; signing identity and
  provisioning profile passed on the `ios:package` command.

Mobile self-update is **not** the desktop binary-swap flow (app stores forbid
it) — updates ship through the App Store / Play Store. The in-app "update
available" notice still works as an informational prompt.

## Migration (v2 → v3)

Moving the shipped desktop from Wails v2 to v3 is the prerequisite for mobile.
Key changes:

| v2 | v3 |
|----|----|
| `wails.Run(&options.App{...})` | `application.New(application.Options{...})` + `app.Window.NewWithOptions(...)` |
| `AssetServer.Handler` | `Assets: application.AssetOptions{Handler: ...}` |
| `Bind: []interface{}{bridge}` | `Services: []application.Service{application.NewService(bridge)}` |
| `OnStartup(ctx)` | `ServiceStartup(ctx, options)` on the service |
| `runtime.BrowserOpenURL(ctx, url)` | `app.Browser.OpenURL(url)` |
| `runtime.SaveFileDialog(ctx, opts)` | `app.Dialog.SaveFile()...PromptForSingleSelection()` |
| `window.go.main.Bridge.X()` (frontend) | generated `@wailsio/runtime` bindings |

See [ROADMAP.md](ROADMAP.md) M9 for status.
