# Configuration

Durations: one or more `<integer><unit>` segments, units `d`, `h`, `m`, `s` (e.g. `30m`, `7d`, `1d12h`)

Invalid values stop the server on startup

## Server

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDRESS` | `:80` | Listen address; `unix:///path/to.sock` listens on a Unix socket |
| `HTTP_MODE` | `production` | `development` drops `Secure` from the `refresh_token` cookie, for local HTTP testing |
| `REDIS_URL` | — | Stores `state`, caches device secrets, dedupes `/callback` and `/refresh` across instances; unset: in-memory, single instance only |

## OAuth2.0 Provider

- Built-in: `discord`, `github`, `google`; any other value uses the generic provider
- `OIDC_ISSUER_URL` switches to OIDC discovery and overrides `OAUTH2_PROVIDER`

| Variable | Description |
|---|---|
| `OAUTH2_CLIENT_ID`, `OAUTH2_CLIENT_SECRET` | Client credentials; OAuth2.0 is enabled only when both are set along with `OAUTH2_REDIRECT_URL` or `OAUTH2_ALLOWED_REDIRECT_URLS` |
| `OAUTH2_REDIRECT_URL` | Default redirect URL when `/login` omits `redirect_uri` |
| `OAUTH2_ALLOWED_REDIRECT_URLS` | Comma-separated redirect URLs `/login` may request via `redirect_uri`, matched exactly; each must also be registered with the provider |
| `OAUTH2_PROVIDER` | `discord`, `github`, `google`, or any other name (generic) |
| `OAUTH2_SCOPES` | Comma-separated scopes |
| `OAUTH2_AUTH_URL`, `OAUTH2_TOKEN_URL`, `OAUTH2_USERINFO_URL` | Required for the generic provider |
| `OAUTH2_REVOKE_URL` | Optional, for revoking provider tokens |
| `OIDC_ISSUER_URL` | Reads endpoints from `{issuer}/.well-known/openid-configuration` |
| `OAUTH2_CLIENT_PKCE` | `true` lets the client hold the PKCE verifier (see [Login Flow](sessions.md#login-flow)) |

## Database

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | — | PostgreSQL connection string; unset: stateless proxy mode |
| `DB_USER_EMAIL_REFERENCE` | — | Your users table, `[<schema>.]<table>(<column>)[:<type>]` (see [User Email Reference](../database/README.md#user-email-reference)); unset: managed users mode |
| `DB_USER_USERNAME_COLUMN` | — | Column used as the `username` claim; requires `DB_USER_EMAIL_REFERENCE` |
| `DB_AUTO_CREATE_SCHEMA` | `false` | Create tables on startup (see [Deployment Modes](../README.md#deployment-modes)); requires DDL privileges |
| `DB_CLEANUP_INTERVAL` | `24h` | How often sessions not refreshed for 7 days are deleted; bare integer = hours |
| `DB_MAX_OPEN_CONNS` | `20` | Max open connections |
| `DB_MAX_IDLE_CONNS` | `15` | Max idle connections |
| `DB_CONN_MAX_LIFETIME` | `5m` | Max connection lifetime; bare integer = minutes |
| `DB_CONN_MAX_IDLE_TIME` | `2m` | Max connection idle time; bare integer = minutes |

## Tokens

| Variable | Default | Description |
|---|---|---|
| `JWT_ACCESS_TOKEN_DURATION` | `15m` | Access token lifetime, unit required |
| `JWT_REFRESH_TOKEN_DURATION` | `7d` | Refresh token and cookie lifetime, unit required |
| `JWT_ISSUER` | — | `iss` claim, set and verified |
| `JWT_AUDIENCE` | — | `aud` claim, set and verified |
| `IDENTITY_JWT_SECRET` | — | At least 32 bytes, `/verify` returns a signed identity token instead (see [Identity Token](api.md#identity-token)) |

## Sign-in Policy

| Variable | Default | Description |
|---|---|---|
| `ALLOW_REGISTRATION` | `false` | Insert first-time users into `users` (managed users only) |
| `ALLOW_UNVERIFIED_EMAIL` | `false` | Accept unverified emails; by default `email_verified` must be true |
| `PASS_OAUTH_TOKEN` | `false` | Return provider tokens to the client (see [Provider Tokens](sessions.md#provider-tokens)) |
