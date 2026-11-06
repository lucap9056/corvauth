English | [繁體中文](README.zh-TW.md)

# Corvauth

Device-bound session authentication server

If you already run a reverse proxy, let this project handle authentication:

- **Backends skip auth**: the proxy verifies via `/verify`, backends only read headers
- **Plugs into the proxy**: works with nginx `auth_request` and Traefik `forwardAuth`
- **Tamper-proof headers**: with `IDENTITY_JWT_SECRET`, a signed identity token is sent instead
- **Built-in OAuth2.0 providers**: Discord, GitHub, Google, and any OIDC provider
- **Reuse your users table**: linked by email, no data migration

```
               ┌──────────────────────── server ────────────────────────┐
 browser ───▶   POST /sessions, /callback ──▶ OAuth2.0 / OIDC provider
 proxy   ───▶   /verify, /sessions ──▶ jwt ──▶ database ──▶ PostgreSQL
               └────────────────────────────────────────────────────────┘
```

- Every login creates a **device session** with its own signing secret
  - Logging out or deleting a device only affects that device's tokens
- Users are identified by **email**
  - `auth_user_devices` references the users table by foreign key, so deleting a user removes their sessions

## Quick Start

Copy [`docker/docker-compose.yml`](docker/docker-compose.yml), replace the `change-me` password, OAuth2.0 client credentials, and redirect URL, then:

```bash
docker compose up -d
```

It uses the image `ghcr.io/lucap9056/corvauth` in **managed users** mode with registration on, signing in through GitHub (see [Configuration](docs/en/topics/configuration.md#oauth2) for other providers)

### Development

Builds from source and starts PostgreSQL, Redis, a fake OIDC provider, and the server (`http://localhost:8080`):

```bash
docker compose -f docker-dev/docker-compose.yml up --build -d
```

Any fake provider user can sign in

## Deployment Modes

Selected by `DATABASE_URL` and `DB_USER_EMAIL_REFERENCE`:

| | Stateless proxy | Managed users | External users |
|---|---|---|---|
| **Set when** | `DATABASE_URL` unset | only `DATABASE_URL` set | both set |
| **Users table** | none | `users`, owned by this server | yours, referenced by email |
| **Session tokens** | no, provider tokens returned as-is | yes | yes |
| **`GET`/`DELETE /sessions`, `/sessions/current`, `/verify`** | X | O | O |
| **`ALLOW_REGISTRATION`** | ignored | inserts new users into `users` | ignored (warns on startup) |
| **`DELETE /users/me`** | X | deletes the user and all sessions | X |
| **`username` claim** | — | `users.username` | `DB_USER_USERNAME_COLUMN`, or empty |
| **`DB_AUTO_CREATE_SCHEMA=true` creates** | — | `users`, `auth_user_devices` | `auth_user_devices` |

## Documentation

- [Reference](docs/en/reference.md): authentication, response format, and shared conventions
- [Deployment Modes](docs/en/topics/deployment-modes.md): users table setup per mode
- [OAuth2](docs/en/topics/oauth2.md) and [Tokens](docs/en/topics/tokens.md): login flow, rotation, provider tokens
- [API](docs/en/resources/session.md): endpoints, objects, and parameters
- [Status Codes](docs/en/topics/status-codes.md): HTTP status codes and error headers
- [Configuration](docs/en/topics/configuration.md): environment variables
- [Logging](docs/en/topics/logging.md): log fields, error events, client IP resolution

## Requirements

- PostgreSQL 14+ (session modes)
- Redis (optional; required for multiple instances)
- Building from source: Go 1.27+

## Testing

Run in `server/`:

```bash
go test ./internal/... ./tests/
```

Unit and integration tests need no external services; others need their environment:

| Suite | Requirement | Command |
|---|---|---|
| Redis tests in `internal/cache/device`, `internal/flight` | `TEST_REDIS_URL` | `go test ./internal/...` (skipped when unset) |
| End-to-end | compose running | `go test ./tests/e2e/` |

## Libraries

| Module | Purpose |
|---|---|
| [jwt](./jwt/README.md) | Issues and verifies access / refresh tokens, one secret per device |
| [database](./database/README.md) | Stores device sessions in PostgreSQL, linked to your users table by email |
| [oauth2](./oauth2/README.md) | OAuth2.0 / OIDC providers and login client |
