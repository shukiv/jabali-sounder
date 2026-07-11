-- 000008_admin_role.up.sql
-- Operator permission level (M3: RBAC). Existing single admin becomes owner.
ALTER TABLE admins
    ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'owner';
