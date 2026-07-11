-- 000012_server_environment.up.sql
-- Single-value environment/group for a managed server (M4), distinct from tags.
ALTER TABLE servers
    ADD COLUMN environment VARCHAR(50) NULL AFTER tags;
