-- 000005_server_tags.down.sql
ALTER TABLE servers
    DROP COLUMN tags;
