-- 000005_server_tags.up.sql
-- Operator-defined labels used to group and filter managed servers.
ALTER TABLE servers
    ADD COLUMN tags TEXT NULL AFTER scopes;

UPDATE servers
SET tags = '[]'
WHERE tags IS NULL;
