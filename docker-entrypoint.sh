#!/bin/sh
# Container entrypoint: provision secrets, run migrations, optionally bootstrap
# the admin, then hand off to the server. Mirrors install.sh for containers.
# Everything is idempotent, so it is safe to run on every start.
set -eu

DATA_DIR="${JABALI_SOUNDER_DATA_DIR:-/data}"
KEY_FILE="${JABALI_SOUNDER_SECRET_KEY_FILE:-${DATA_DIR}/secrets.key}"
JWT_FILE="${DATA_DIR}/jwt.secret"
BIN=jabali-sounder-server

# 1. Token-encryption key (32 random bytes). Sealed AES-GCM secrets depend on
#    it, so it must be stable — keep it on the /data volume.
if [ ! -f "${KEY_FILE}" ]; then
  openssl rand -out "${KEY_FILE}" 32
  chmod 600 "${KEY_FILE}"
fi

# 2. JWT signing secret. Required in production. Persist it so existing sessions
#    survive restarts, unless the operator supplies one via the environment.
if [ -z "${JABALI_SOUNDER_JWT_SECRET:-}" ]; then
  if [ ! -f "${JWT_FILE}" ]; then
    openssl rand -hex 32 > "${JWT_FILE}"
    chmod 600 "${JWT_FILE}"
  fi
  JABALI_SOUNDER_JWT_SECRET="$(cat "${JWT_FILE}")"
  export JABALI_SOUNDER_JWT_SECRET
fi

# 3. Apply database migrations (idempotent).
"${BIN}" migrate up

# 4. First-run admin bootstrap from the environment (optional). Re-running
#    updates the password, which is the intended recovery path.
if [ -n "${JABALI_SOUNDER_ADMIN_PASSWORD:-}" ]; then
  ADMIN_USER="${JABALI_SOUNDER_ADMIN:-admin}"
  "${BIN}" admin set-password -u "${ADMIN_USER}" -p "${JABALI_SOUNDER_ADMIN_PASSWORD}"
fi

# 5. Hand off (CMD defaults to "serve"). exec so the server is PID 1 and gets
#    the container's stop signals directly.
exec "${BIN}" "$@"
