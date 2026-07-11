-- 000008_admin_role.down.sql
ALTER TABLE admins
    DROP COLUMN role;
