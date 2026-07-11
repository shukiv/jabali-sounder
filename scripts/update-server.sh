#!/usr/bin/env bash
# update-server.sh — update the headless Jabali Sounder server binary in place.
#
# Downloads the latest GitHub release asset, verifies its sha256 against the
# published checksums.txt, swaps the binary, and restarts the systemd service.
# Idempotent: exits early when already on the latest version.
#
# Usage:
#   sudo ./update-server.sh [--binary /usr/local/bin/jabali-sounder-server] \
#                           [--service jabali-sounder] [--force]
#
# Requires: curl, sha256sum, python3, systemctl (optional; skipped if absent).
set -euo pipefail

REPO="${JABALI_UPDATE_REPO:-shukiv/jabali-sounder}"
BINARY="/usr/local/bin/jabali-sounder-server"
SERVICE="jabali-sounder"
ASSET="jabali-sounder-server-linux-amd64"
FORCE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary) BINARY="$2"; shift 2 ;;
    --service) SERVICE="$2"; shift 2 ;;
    --asset) ASSET="$2"; shift 2 ;;
    --force) FORCE=1; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

for tool in curl sha256sum python3; do
  command -v "$tool" >/dev/null 2>&1 || { echo "missing required tool: $tool" >&2; exit 1; }
done

api="https://api.github.com/repos/${REPO}/releases/latest"
echo "Checking ${REPO} for the latest release…"
release_json="$(curl -fsSL -H 'Accept: application/vnd.github+json' -H 'User-Agent: jabali-sounder' "$api")"

latest_tag="$(printf '%s' "$release_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])')"
echo "Latest release: ${latest_tag}"

# Compare against the running binary (skip if identical, unless --force).
if [[ "$FORCE" -eq 0 && -x "$BINARY" ]]; then
  current="$("$BINARY" version 2>/dev/null | awk '{print $2}' || true)"
  if [[ -n "$current" && "$current" == "$latest_tag" ]]; then
    echo "Already on ${current}. Nothing to do (use --force to reinstall)."
    exit 0
  fi
  echo "Current: ${current:-unknown} -> updating to ${latest_tag}"
fi

# Resolve the asset + checksums download URLs.
read -r asset_url sums_url <<<"$(printf '%s' "$release_json" | ASSET="$ASSET" python3 -c '
import json,os,sys
d=json.load(sys.stdin); want=os.environ["ASSET"]
a=s=""
for x in d.get("assets",[]):
    if x["name"]==want: a=x["browser_download_url"]
    if x["name"]=="checksums.txt": s=x["browser_download_url"]
print(a,s)')"

[[ -n "$asset_url" ]] || { echo "release has no asset named ${ASSET}" >&2; exit 1; }
[[ -n "$sums_url"  ]] || { echo "release has no checksums.txt" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
echo "Downloading ${ASSET}…"
curl -fsSL -H 'User-Agent: jabali-sounder' -o "$tmp/$ASSET" "$asset_url"
curl -fsSL -H 'User-Agent: jabali-sounder' -o "$tmp/checksums.txt" "$sums_url"

want_sum="$(awk -v n="$ASSET" '$2==n || $2=="*"n {print $1}' "$tmp/checksums.txt" | head -n1)"
[[ -n "$want_sum" ]] || { echo "no checksum published for ${ASSET}" >&2; exit 1; }
got_sum="$(sha256sum "$tmp/$ASSET" | awk '{print $1}')"
if [[ "$want_sum" != "$got_sum" ]]; then
  echo "CHECKSUM MISMATCH (want $want_sum, got $got_sum) — aborting." >&2
  exit 1
fi
echo "Checksum verified."

chmod +x "$tmp/$ASSET"

# Stop the service (if managed by systemd), swap the binary, restart.
have_systemd=0
if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files "${SERVICE}.service" >/dev/null 2>&1; then
  have_systemd=1
fi

if [[ "$have_systemd" -eq 1 ]]; then
  echo "Stopping ${SERVICE}…"
  systemctl stop "$SERVICE" || true
fi

# Keep a rollback copy.
if [[ -f "$BINARY" ]]; then
  cp -f "$BINARY" "${BINARY}.bak"
fi
install -m 0755 "$tmp/$ASSET" "$BINARY"
echo "Installed ${latest_tag} to ${BINARY} (previous kept at ${BINARY}.bak)."

if [[ "$have_systemd" -eq 1 ]]; then
  echo "Starting ${SERVICE}…"
  systemctl start "$SERVICE"
  systemctl --no-pager --lines=0 status "$SERVICE" || true
else
  echo "systemd unit ${SERVICE} not found — restart the server manually."
fi
echo "Done."
