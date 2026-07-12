# Updating Jabali Sounder

Sounder ships as three deliverables, all published to the GitHub Releases page
(`shukiv/jabali-sounder`). Every release includes a `checksums.txt` covering all
binaries; the desktop self-update and the server script both verify against it.

The running build reports its version at **Settings → About & updates** (desktop
and browser) and via `GET /api/v1/version`, which also reports whether a newer
release exists (checked against GitHub at most once an hour).

## 1. Desktop app — in-app self-update

When a newer release is available an **Update** affordance appears in the header
and on the About card. Click **Install update**: the app downloads the binary
for your OS, verifies its checksum, swaps itself in place, and relaunches.

- Linux/macOS replace the running binary directly (safe — the process keeps its
  open inode); Windows moves the current `.exe` aside to `.exe.old` first.
- On failure (offline, checksum mismatch) nothing is changed and an error toast
  is shown.
- macOS/Windows builds are unsigned, so the OS may still warn on first launch of
  a freshly downloaded binary. Signing is tracked separately.

## 2. Headless server — update script

Use `scripts/update-server.sh`. It checks GitHub, compares against the running
server binary's `version`, verifies the checksum, keeps a `.bak` rollback
copy, and restarts the systemd service.

```bash
sudo ./scripts/update-server.sh \
  --binary /usr/local/bin/jabali-sounder \
  --service jabali-sounder
```

Flags: `--force` reinstalls even if already current; `--asset` overrides the
asset name; `--repo` via the `JABALI_UPDATE_REPO` env var.

### Automatic weekly updates (systemd timer)

`/etc/systemd/system/jabali-sounder-update.service`:

```ini
[Unit]
Description=Update Jabali Sounder server
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/jabali-sounder/scripts/update-server.sh --service jabali-sounder
```

`/etc/systemd/system/jabali-sounder-update.timer`:

```ini
[Unit]
Description=Weekly Jabali Sounder update check

[Timer]
OnCalendar=Sun 04:00
Persistent=true
RandomizedDelaySec=1h

[Install]
WantedBy=timers.target
```

Enable:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now jabali-sounder-update.timer
```

## 3. curl | bash installer

The one-line installer always fetches the latest release, so re-running it
updates an existing install:

```bash
curl -fsSL https://raw.githubusercontent.com/shukiv/jabali-sounder/main/install.sh | bash
```

## Rollback

- Server: `sudo mv /usr/local/bin/jabali-sounder.bak /usr/local/bin/jabali-sounder && sudo systemctl restart jabali-sounder`.
- Desktop (Windows): rename `jabali-sounder.exe.old` back over the `.exe`.
- Any platform: download a specific older release from the Releases page.
