ALTER TABLE api_tokens
    ADD COLUMN scopes     TEXT NULL,
    ADD COLUMN allowed_ips TEXT NULL;
