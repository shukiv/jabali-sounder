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
# Overrides (env): JABALI_SOUNDER_ADDR (default 0.0.0.0:8484),
# JABALI_SOUNDER_ADMIN (default admin), JABALI_SOUNDER_ADMIN_PASSWORD,
# JABALI_SOUNDER_BIN_URL.
set -Eeuo pipefail
export DEBIAN_FRONTEND=noninteractive
export PATH="$PATH:/usr/sbin:/sbin"

REPO="shukiv/jabali-sounder"
BIN_URL="${JABALI_SOUNDER_BIN_URL:-https://github.com/${REPO}/releases/latest/download/jabali-sounder-server-linux-amd64}"
BIN="/usr/local/bin/jabali-sounder"
CONF_DIR="/etc/jabali-sounder"
DATA_DIR="/var/lib/jabali-sounder"
SVC_USER="jabali-sounder"
SVC="jabali-sounder"
CONF="${CONF_DIR}/config.toml"
KEY_FILE="${CONF_DIR}/secrets.key"
ADDR="${JABALI_SOUNDER_ADDR:-0.0.0.0:8484}"

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
sudo -u "$SVC_USER" JABALI_SOUNDER_CONFIG="$CONF" "$BIN" admin set-password -u "$ADMIN_USER" -p "$ADMIN_PW" \
  || die "admin setup failed"

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
systemctl enable "$SVC" >/dev/null 2>&1 || true
# Use restart, not `enable --now`: on a re-install the unit is already active, and
# `--now` would leave the OLD process running the now-(deleted) binary on disk.
# restart starts it if stopped and reloads the freshly installed binary if running.
systemctl restart "$SVC" || die "failed to start ${SVC} (journalctl -u ${SVC})"

port="${ADDR##*:}"
for _ in $(seq 1 30); do
  curl -fsS "http://127.0.0.1:${port}/health" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -fsS "http://127.0.0.1:${port}/health" >/dev/null 2>&1 \
  || die "service did not become healthy — check: journalctl -u ${SVC} -e"

ip="$(hostname -I 2>/dev/null | awk '{print $1}')"; [ -n "$ip" ] || ip="<server-ip>"
ok "Jabali Sounder is running."
printf '\n'
printf '  URL:      http://%s:%s/\n' "$ip" "$port"
printf '  Login:    %s\n' "$ADMIN_USER"
printf '  Password: %s\n' "$ADMIN_PW"
printf '\n'
log "Save the password now, then change it in Settings after logging in."
log "Manage: systemctl status ${SVC}   ·   journalctl -u ${SVC} -f"
log "Config: ${CONF}   (serves plain HTTP — put a TLS reverse proxy in front for public use)"
