-- 000006_server_cert_expiry.down.sql
ALTER TABLE servers
    DROP COLUMN cert_expires_at;
