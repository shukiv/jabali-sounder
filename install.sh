#!/usr/bin/env bash
#
# Jabali Sounder one-liner installer.
#
#   curl -fsSL https://raw.githubusercontent.com/shukiv/jabali-sounder/main/install.sh | sudo bash
#
# On a fresh Debian/Ubuntu root shell it:
#   1. Downloads the latest prebuilt server binary (single static binary that
#      serves both the API and the SPA on one port — no Go/Node/MariaDB needed).
#   2. Creates a `jabali-sounder` system user + /etc/jabali-sounder (config +
#      encryption key) + /var/lib/jabali-sounder (SQLite database).
#   3. Runs migrations, creates an admin with a random password, installs and
#      starts a hardened systemd service, and health-checks it.
#
# Overrides (env): JABALI_SOUNDER_ADDR (default 127.0.0.1:8484 — loopback only;
# set to 0.0.0.0:PORT to expose, ideally behind a TLS reverse proxy),
# JABALI_SOUNDER_ADMIN (default admin), JABALI_SOUNDER_ADMIN_PASSWORD,
# JABALI_SOUNDER_BIN_URL, JABALI_SOUNDER_SHA256_URL, JABALI_SOUNDER_BIN_SHA256.
set -Eeuo pipefail
export DEBIAN_FRONTEND=noninteractive
export PATH="$PATH:/usr/sbin:/sbin"

REPO="shukiv/jabali-sounder"
BIN_URL="${JABALI_SOUNDER_BIN_URL:-https://github.com/${REPO}/releases/latest/download/jabali-sounder-server-linux-amd64}"
# Checksum manifest lives next to the binary in the same release.
SHA256_URL="${JABALI_SOUNDER_SHA256_URL:-${BIN_URL%/*}/SHA256SUMS}"
BIN="/usr/local/bin/jabali-sounder"
CONF_DIR="/etc/jabali-sounder"
DATA_DIR="/var/lib/jabali-sounder"
SVC_USER="jabali-sounder"
SVC="jabali-sounder"
CONF="${CONF_DIR}/config.toml"
KEY_FILE="${CONF_DIR}/secrets.key"
ADDR="${JABALI_SOUNDER_ADDR:-127.0.0.1:8484}"

log() { printf '\033[1;34m[sounder]\033[0m %s\n' "$*"; }
ok()  { printf '\033[1;32m[sounder]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[sounder] %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "must run as root (curl … | sudo bash)"
for t in curl openssl install useradd systemctl; do
  command -v "$t" >/dev/null 2>&1 || die "missing required tool: $t"
done

log "downloading server binary"
tmp="$(mktemp)"; trap 'rm -f "$tmp"' EXIT
curl -fsSL "$BIN_URL" -o "$tmp" || die "download failed: $BIN_URL"

# Verify integrity before installing/executing as root. A pinned hash
# (JABALI_SOUNDER_BIN_SHA256) wins; otherwise fetch the release SHA256SUMS and
# match the binary's basename. Abort on mismatch or if no checksum is available.
got="$(sha256sum "$tmp" | awk '{print $1}')"
want="${JABALI_SOUNDER_BIN_SHA256:-}"
if [ -z "$want" ]; then
  bin_name="${BIN_URL##*/}"
  sums="$(mktemp)"; trap 'rm -f "$tmp" "$sums"' EXIT
  curl -fsSL "$SHA256_URL" -o "$sums"     || die "could not fetch checksums ($SHA256_URL); set JABALI_SOUNDER_BIN_SHA256 to install"
  want="$(awk -v f="$bin_name" '$2==f || $2=="*"f {print $1; exit}' "$sums")"
  [ -n "$want" ] || die "no checksum for ${bin_name} in SHA256SUMS — refusing to install"
fi
[ "$got" = "$want" ] || die "checksum mismatch — refusing to install (expected ${want}, got ${got})"
ok "verified binary checksum"

install -m 0755 "$tmp" "$BIN"
ok "installed $BIN"

id "$SVC_USER" >/dev/null 2>&1 || useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$SVC_USER"
install -d -o "$SVC_USER" -g "$SVC_USER" -m 0750 "$DATA_DIR"
install -d -m 0755 "$CONF_DIR"

# Token-encryption key + JWT signing secret.
[ -f "$KEY_FILE" ] || openssl rand -out "$KEY_FILE" 32
chown "$SVC_USER:$SVC_USER" "$KEY_FILE"; chmod 0600 "$KEY_FILE"
JWT_SECRET="$(openssl rand -hex 32)"

if [ ! -f "$CONF" ]; then
  cat > "$CONF" <<EOF
[server]
addr = "${ADDR}"
env  = "production"

[log]
level  = "info"
format = "text"

[database]
driver = "sqlite"
url    = "${DATA_DIR}/sounder.db"

[secrets]
key_file = "${KEY_FILE}"

[jwt]
secret = "${JWT_SECRET}"
EOF
  ok "wrote ${CONF}"
else
  log "keeping existing ${CONF}"
fi
chown "$SVC_USER:$SVC_USER" "$CONF"; chmod 0600 "$CONF"

log "running migrations"
sudo -u "$SVC_USER" JABALI_SOUNDER_CONFIG="$CONF" "$BIN" migrate up || die "migrate failed"

ADMIN_USER="${JABALI_SOUNDER_ADMIN:-admin}"
ADMIN_PW="${JABALI_SOUNDER_ADMIN_PASSWORD:-$(openssl rand -hex 12)}"
log "setting admin password for '${ADMIN_USER}'"
# Pass the password via env (readable only by the process owner in
# /proc/<pid>/environ), never on argv where any local user can see it (#117).
sudo -u "$SVC_USER" JABALI_SOUNDER_CONFIG="$CONF" JABALI_SOUNDER_ADMIN_PASSWORD="$ADMIN_PW" \
  "$BIN" admin set-password -u "$ADMIN_USER" || die "admin setup failed"

cat > "/etc/systemd/system/${SVC}.service" <<EOF
[Unit]
Description=Jabali Sounder control plane
After=network-online.target
Wants=network-online.target

[Service]
User=${SVC_USER}
Group=${SVC_USER}
Environment=JABALI_SOUNDER_CONFIG=${CONF}
ExecStart=${BIN} serve
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=${DATA_DIR}
CapabilityBoundingSet=
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now "$SVC" >/dev/null 2>&1 || die "failed to start ${SVC} (journalctl -u ${SVC})"

port="${ADDR##*:}"
for _ in $(seq 1 30); do
  curl -fsS "http://127.0.0.1:${port}/health" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -fsS "http://127.0.0.1:${port}/health" >/dev/null 2>&1 \
  || die "service did not become healthy — check: journalctl -u ${SVC} -e"

bind_host="${ADDR%:*}"
is_loopback=0
case "$bind_host" in 127.0.0.1|localhost|::1|"[::1]") is_loopback=1 ;; esac

ok "Jabali Sounder is running."
printf '\n'
if [ "$is_loopback" -eq 1 ]; then
  printf '  URL:      http://127.0.0.1:%s/  (loopback only)\n' "$port"
else
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"; [ -n "$ip" ] || ip="<server-ip>"
  printf '  URL:      http://%s:%s/\n' "$ip" "$port"
fi
printf '  Login:    %s\n' "$ADMIN_USER"
printf '  Password: %s\n' "$ADMIN_PW"
printf '\n'
log "Save the password now, then change it in Settings after logging in."
log "Manage: systemctl status ${SVC}   ·   journalctl -u ${SVC} -f"
if [ "$is_loopback" -eq 1 ]; then
  log "Bound to loopback (safe default). To reach it remotely, front it with a TLS"
  log "reverse proxy and set JABALI_SOUNDER_ADDR (e.g. 0.0.0.0:8484). It serves plain"
  log "HTTP — never expose it directly without TLS."
else
  log "WARNING: bound to ${ADDR} over plain HTTP — login and tokens travel in cleartext."
  log "Put a TLS reverse proxy in front and restrict network access."
fi
log "Config: ${CONF}"
