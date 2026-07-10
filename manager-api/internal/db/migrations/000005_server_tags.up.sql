-- 000005_server_tags.up.sql
-- Operator-defined labels used to group and filter managed servers.
-- Single statement: golang-migrate's MySQL driver runs each file as one Exec and
-- rejects multiple statements unless the DSN sets multiStatements=true. Existing
-- NULL rows read back as [] via JSONStringArray.Scan, and new rows are always
-- written through Value() ("[]" for nil), so no backfill UPDATE is needed.
ALTER TABLE servers
    ADD COLUMN tags TEXT NULL AFTER scopes;
