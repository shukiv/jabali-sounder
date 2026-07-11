-- 000006_server_cert_expiry.up.sql
-- TLS certificate expiry of the managed panel, sampled by the health poller.
ALTER TABLE servers
    ADD COLUMN cert_expires_at DATETIME NULL AFTER last_checked_at;
