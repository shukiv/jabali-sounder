# Security

Jabali Sounder is an administrative control plane. Treat its database, config,
secret key, and admin credentials as privileged infrastructure.

## Core Rules

- Do not SSH into managed Jabali Panel servers from Sounder workflows.
- Use scoped automation tokens for every managed-server action.
- Prefer `read:*` only for development or trusted internal test servers.
- Never store automation token secrets unencrypted in production.
- Never include tokens, passwords, or customer private data in issue trackers,
  logs, screenshots, or documentation.

## Admin Authentication

Admins log in with username/password. Passwords are stored as bcrypt hashes in
the `admins` table.

Successful login returns a JWT signed with `JABALI_SOUNDER_JWT_SECRET` or
`[jwt].secret`.

Set a strong JWT secret in production. If it is missing, Sounder uses a
development fallback and logs a warning.

## Automation Token Storage

Each enrolled managed server stores:

- `token_id`
- encrypted `token_secret`
- selected scopes

The token secret is encrypted with the local 32-byte key file configured by:

```toml
[secrets]
key_file = "/etc/jabali-sounder/secrets.key"
```

Generate it with:

```bash
openssl rand -out /etc/jabali-sounder/secrets.key 32
chmod 600 /etc/jabali-sounder/secrets.key
```

If the key cannot be loaded, Sounder has a development fallback that stores
hex-encoded plaintext. This fallback is not acceptable for production.

## Remote Authentication

Sounder signs managed-server automation requests with HMAC:

```text
Authorization: Jabali-HMAC kid=<token-id>, ts=<unix>, sig=<hex>
```

The managed Panel server is responsible for:

- Validating the HMAC signature.
- Checking token expiry/revocation.
- Enforcing scopes.
- Returning 401/403 for invalid credentials or insufficient scope.

## TLS

The remote client currently allows self-signed certificates for managed servers.
HMAC is the authentication layer, but TLS still protects request metadata and
responses in transit. Prefer valid TLS certificates where possible.

## Data Exposure

Current Sounder UI/API exposes:

- Server metadata.
- User and domain inventory from managed Panels.
- Live server metrics.
- Mail metadata when the managed Panel exposes mail automation endpoints.

Do not expose autoresponder body fields broadly unless security review accepts
that operational need. Metadata-only autoresponder responses may be safer.

## Secret Rotation

Recommended rotation cases:

- Automation token was exposed.
- Managed server ownership changed.
- Admin left the team.
- Secret key file may have been copied.

Current implementation does not provide a first-class token rotation endpoint.
Rotate by creating a new automation token in the managed Panel, updating or
reenrolling the server in Sounder, then revoking the old token in Panel.

## Backups

Back up the database and secret key together. If the database is restored
without the original secret key, encrypted automation token secrets cannot be
opened.

Store backups with access controls equivalent to production secrets.
