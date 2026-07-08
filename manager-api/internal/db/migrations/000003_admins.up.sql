-- 000003_admins.up.sql
CREATE TABLE IF NOT EXISTS admins (
    id              CHAR(26)     NOT NULL PRIMARY KEY,
    username        VARCHAR(100) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_admins_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
