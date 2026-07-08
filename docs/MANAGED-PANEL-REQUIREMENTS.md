# Managed Panel Requirements

Jabali Sounder does not SSH into managed servers. Every managed server must
expose the required Jabali Panel HTTP endpoints and have a scoped automation
token enrolled in Sounder.

## Required Base Endpoints

### `GET /health`

Unauthenticated liveness endpoint used during enrollment and health checks.

Expected response:

```json
{
  "status": "ok",
  "version": "v0.1.0"
}
```

### `GET /api/v1/automation/status`

HMAC-protected automation status endpoint.

Minimum expected response:

```json
{
  "healthy": true,
  "time": "2026-07-08T18:00:00Z",
  "version": "v0.1.0"
}
```

For the Monitor tab, richer status payloads should also include:

- `host` or `system`
- `cpu`
- `io`

Sounder accepts either `host` or `system` for host metrics.

## Inventory Endpoints

### `GET /api/v1/automation/domains`

Scope:

```text
read:domains or read:*
```

Response:

```json
{
  "data": [
    {
      "id": "01...",
      "name": "example.com",
      "user_id": "01...",
      "is_enabled": true
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 1
}
```

### `GET /api/v1/automation/users`

Scope:

```text
read:users or read:*
```

Response:

```json
{
  "data": [
    {
      "id": "01...",
      "email": "user@example.com",
      "username": "user",
      "package_id": "01...",
      "is_admin": false
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 1
}
```

### `GET /api/v1/automation/applications`

Scope:

```text
read:applications or read:*
```

Response:

```json
{
  "data": [
    {
      "id": "01...",
      "app_type": "wordpress",
      "domain_id": "01...",
      "status": "active"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 1
}
```

## Monitor Metrics Fields

`/api/v1/automation/status` should include these optional fields for full
Monitor support:

```json
{
  "as_of": "2026-07-08T18:00:00Z",
  "host": {
    "load_avg": [0.2, 0.3, 0.4],
    "mem_total_kb": 4096000,
    "mem_used_kb": 1024000,
    "partitions": [
      {
        "mount_point": "/",
        "total_bytes": 10737418240,
        "used_bytes": 2147483648,
        "free_bytes": 8589934592
      }
    ]
  },
  "cpu": {
    "usage_percent": 7.4,
    "iowait_percent": 0.1,
    "warming_up": false,
    "as_of": "2026-07-08T18:00:00Z"
  },
  "io": {
    "read_bps": 1024,
    "write_bps": 2048
  }
}
```

## Mail Endpoints

The Sounder Mail tab expects these read-only endpoints. They are not currently
available on all managed Panel servers.

Recommended scope:

```text
read:mail or read:*
```

### `GET /api/v1/automation/mail/mailboxes`

Rows should include:

- `id`
- `domain_id`
- `email`
- `display_name`
- `quota_bytes`
- `is_disabled`
- `last_usage_bytes`
- `last_usage_at`
- `created_at`
- `updated_at`
- `domain_name`
- `owner_user_id`
- `user_username`

### `GET /api/v1/automation/mail/groups`

Rows should include:

- `id`
- `domain_id`
- `local_part`
- `email`
- `display_name`
- `description`
- `group_kind`
- `has_mailbox`
- `has_calendar`
- `has_addressbook`
- `has_files`
- `internal_only`
- `created_at`
- `updated_at`
- `domain_name`
- `owner_user_id`
- `user_username`
- `member_count`

### `GET /api/v1/automation/mail/forwarders`

Rows should include:

- `id`
- `mailbox_id`
- `mailbox_email`
- `domain_id`
- `domain_name`
- `type`
- `local_part`
- `target`
- `keep_copy`
- `enabled`
- `created_at`

### `GET /api/v1/automation/mail/domain-forwarders`

Rows should include:

- `id`
- `domain_id`
- `domain_name`
- `type`
- `local_part`
- `target`
- `enabled`
- `managed_by`
- `created_at`

### `GET /api/v1/automation/mail/autoresponders`

Rows should include:

- `mailbox_id`
- `mailbox_email`
- `domain_id`
- `domain_name`
- `enabled`
- `from_date`
- `to_date`
- `subject`
- `text_body`
- `html_body`
- `updated_at`

Security review may choose to omit body fields and expose metadata only. If so,
Sounder UI should be adjusted to avoid expecting body content.

## HMAC Signature

Automation requests must validate:

```text
Authorization: Jabali-HMAC kid=<token-id>, ts=<unix>, sig=<hex>
```

Signature input:

```text
METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(body))
```

`RequestURI` includes the query string.
