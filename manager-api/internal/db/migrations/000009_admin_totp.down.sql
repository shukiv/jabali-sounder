-- 000009_admin_totp.down.sql
ALTER TABLE admins
    DROP COLUMN totp_secret_enc,
    DROP COLUMN totp_enabled;
