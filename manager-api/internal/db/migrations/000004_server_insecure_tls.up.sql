-- 000004_server_insecure_tls.up.sql
-- Per-server opt-in to skip TLS verification (self-signed panels on a LAN).
-- Default 0 (verify). HMAC authenticates requests but not responses, so this
-- widens MITM exposure per-server and is off unless explicitly enabled.
ALTER TABLE servers
    ADD COLUMN insecure_skip_verify TINYINT(1) NOT NULL DEFAULT 0;
