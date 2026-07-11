-- 000009_admin_totp.up.sql
-- Admin two-factor (TOTP) secret + enabled flag (M3).
ALTER TABLE admins
    ADD COLUMN totp_secret_enc BLOB NULL,
    ADD COLUMN totp_enabled TINYINT(1) NOT NULL DEFAULT 0;
