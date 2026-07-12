# Deployment & reconciliation runbook

This runbook reconciles a host that has drifted into an ambiguous state — more
than one Sounder service/layout, a binary replaced under a running process, or
uncertainty about which release is actually serving (SND-33). Run it on the
host; the code changes referenced here ship in the current build, but the host
steps must be performed by an operator with verified access.

## Container deployment (Docker / Podman)

For a fresh containerised deploy (as opposed to the on-host reconciliation this
runbook covers), use the image:

```bash
JABALI_SOUNDER_ADMIN_PASSWORD=change-me docker compose up -d --build
```

SQLite + secrets live in the `/data` volume; the entrypoint provisions keys,
runs migrations, and bootstraps the admin. Full guide: [Docker](DOCKER.md).

---

## 0. Verify host identity first (do NOT bypass)

If SSH host-key verification fails (changed fingerprint), STOP. Confirm the new
fingerprint through a trusted out-of-band channel (console, provider dashboard,
a colleague who provisioned the host) before connecting. Never pass
`-o StrictHostKeyChecking=no` or delete `known_hosts` to "get in" — a changed
key can mean a man-in-the-middle.

## 1. Inventory every Sounder unit

```bash
systemctl list-unit-files | grep -i sounder
systemctl list-units --all | grep -i sounder
```

For each unit found, inspect what it actually runs and reads:

```bash
systemctl cat <unit>            # ExecStart + EnvironmentFile / config path
systemctl status <unit>         # active? which PID?
```

## 2. Designate exactly one active service

Pick the intended unit (newer executable + config layout). Disable and mask the
obsolete ones so they can never be mistaken for the deployment or started by a
stray `systemctl start`:

```bash
sudo systemctl disable --now <legacy-unit>
sudo systemctl mask <legacy-unit>      # blocks accidental start
```

## 3. Confirm the active config has a database URL

The active unit's config (TOML or `EnvironmentFile`) must set a populated
database URL, e.g. `JABALI_SOUNDER_DATABASE_URL` or `[database].url`. An empty
URL runs with enrollment disabled and no persistence:

```bash
sudo systemctl cat <active-unit> | grep -iE 'EnvironmentFile|ExecStart'
# then check the referenced file actually exists and has a non-empty DB URL
```

## 4. Ensure the running binary is the installed release (not "deleted")

A process shows `(deleted)` when the on-disk binary was replaced without a
restart — the kernel keeps the old inode, so the service runs stale code:

```bash
pid=$(systemctl show -p MainPID --value <active-unit>)
sudo ls -l /proc/"$pid"/exe          # must NOT end in "(deleted)"
```

If it says `(deleted)`, restart the service so it loads the new binary:

```bash
sudo systemctl restart <active-unit>
```

Always restart after swapping a binary. `scripts/update-server.sh` does this
automatically; a manual `cp` over the binary does not.

## 5. Verify which release is serving

The build identity is now stamped into the binary and surfaced three ways —
all three must agree with the release you intend to test:

```bash
jabali-sounder-server version                 # CLI
curl -fsS http://127.0.0.1:<port>/health      # {"status":"ok","version":"vX.Y.Z","commit":"…"}
# authenticated, richer detail (current + latest + update_available):
#   GET /api/v1/version   (Settings -> About & updates in the UI)
```

If `/health` still reports `dev`, the running binary was built without version
stamping — rebuild with the release workflow or `make build` (which injects
`-ldflags -X`), redeploy, and restart.

## 6. Reset the admin password against the ACTIVE database

Recovery commands must use the same config/DB as the active service, or they
silently edit the wrong database:

```bash
# Point at the SAME config/env the active unit uses.
sudo JABALI_SOUNDER_DATABASE_URL='<same-url-as-active-unit>' \
  jabali-sounder-server admin set-password <username>
# (desktop build: jabali-sounder-desktop reset-password <username>)
```

Never record the password, DB credentials, or other secrets anywhere.

## 7. Final health check

```bash
curl -fsS http://127.0.0.1:<port>/health          # ok + expected version
curl -fsSI https://<public-host>/                 # proxied UI reachable (200)
```

Then confirm in the UI: **Settings → About & updates** shows the expected
version, and a password change returns the backend's actionable message (e.g.
"current password incorrect", "new password must be at least 8 characters")
rather than a bare HTTP 422 — a generic status there means a stale frontend is
still deployed; rebuild + redeploy the UI/binary.

## Notes

- The actionable password-change errors and the version endpoints are already in
  the current source; a generic 422 or a `dev` version indicates a stale
  deployment, not a code bug — the fix is deploying the current build.
- Keep one rollback copy of the previous binary (`update-server.sh` writes
  `<binary>.bak`).
