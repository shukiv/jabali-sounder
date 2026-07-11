ALTER TABLE notifications
    DROP COLUMN severity,
    DROP COLUMN acked_at,
    DROP COLUMN acked_by,
    DROP COLUMN snoozed_until,
    DROP COLUMN escalated_at;
