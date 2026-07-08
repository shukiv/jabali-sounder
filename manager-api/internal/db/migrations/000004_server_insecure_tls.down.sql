-- 000004_server_insecure_tls.down.sql
ALTER TABLE servers DROP COLUMN insecure_skip_verify;
