-- 000012_server_environment.down.sql
ALTER TABLE servers
    DROP COLUMN environment;
