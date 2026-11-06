# Configuration

Corvauth is configured entirely through environment variables.

Invalid values stop the server on startup.

Variables marked `?` are optional.

`duration` values follow [Durations](../reference.md#durations).

## Server

###### Server Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| HTTP_ADDRESS? | string | `:80` | Listen address<br>`unix:///path/to.sock` listens on a Unix socket |
| HTTP_MODE? | string | `production` | `development` drops `Secure` from the `refresh_token` cookie, for local HTTP testing |
| REDIS_URL? | string | — | Stores `state`, caches device secrets, and dedupes [Complete Login](../resources/session.md#get-callback) and refresh token rotation across instances<br>Unset: in-memory, single instance only |
| TRUSTED_CLIENT_IP_HEADER? | string | — | Client IP header set by the trusted proxy, e.g. `X-Real-IP`<br>Unset: remote address<br>See [Client IP](./logging.md#client-ip) |

## Logging

###### Logging Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| LOG_LEVEL? | string | `info` | `debug`, `info`, `warn`, or `error` |
| LOG_FORMAT? | string | `console` | `console` or `json` |

## OAuth2

OAuth2 is enabled only when `OAUTH2_CLIENT_ID` and `OAUTH2_CLIENT_SECRET` are set along with `OAUTH2_REDIRECT_URL` or `OAUTH2_ALLOWED_REDIRECT_URLS`.

Provider selection is described in [Providers](./oauth2.md#providers).

###### OAuth2 Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| OAUTH2_CLIENT_ID | string | — | Client ID |
| OAUTH2_CLIENT_SECRET | string | — | Client secret |
| OAUTH2_REDIRECT_URL? | string | — | Default redirect URL when [Start Login](../resources/session.md#post-sessions) omits `redirect_uri` |
| OAUTH2_ALLOWED_REDIRECT_URLS? | comma-separated strings | — | Redirect URLs [Start Login](../resources/session.md#post-sessions) may request via `redirect_uri`, matched exactly<br>Each must also be registered with the provider |
| OAUTH2_PROVIDER? | string | — | `discord`, `github`, `google`, or any other name for the generic provider |
| OAUTH2_SCOPES? | comma-separated strings | — | Requested scopes |
| OAUTH2_AUTH_URL? | string | — | Authorization endpoint<br>Required for the generic provider |
| OAUTH2_TOKEN_URL? | string | — | Token endpoint<br>Required for the generic provider |
| OAUTH2_USERINFO_URL? | string | — | User info endpoint<br>Required for the generic provider |
| OAUTH2_REVOKE_URL? | string | — | Revocation endpoint, for revoking provider tokens |
| OIDC_ISSUER_URL? | string | — | Reads endpoints from `{issuer}/.well-known/openid-configuration`<br>Overrides `OAUTH2_PROVIDER` |
| OAUTH2_CLIENT_PKCE? | boolean | `false` | Lets the client hold the PKCE verifier<br>See [Client PKCE](./oauth2.md#client-pkce) |

## Database

###### Database Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| DATABASE_URL? | string | — | PostgreSQL connection string<br>Unset: [stateless proxy](./deployment-modes.md#stateless-proxy) |
| DB_USER_EMAIL_REFERENCE? | string | — | Your users table, `[<schema>.]<table>(<column>)[:<type>]`<br>See [User Email Reference](../../../database/README.md#user-email-reference)<br>Unset: [managed users](./deployment-modes.md#managed-users) |
| DB_USER_USERNAME_COLUMN? | string | — | Column used as the `username` claim<br>Requires `DB_USER_EMAIL_REFERENCE` |
| DB_AUTO_CREATE_SCHEMA? | boolean | `false` | Create tables on startup<br>See [Deployment Modes](./deployment-modes.md)<br>Requires DDL privileges |
| DB_CLEANUP_INTERVAL? | duration | `24h` | How often sessions not refreshed for 7 days are deleted<br>A bare integer is hours |
| DB_MAX_OPEN_CONNS? | integer | `20` | Max open connections |
| DB_MAX_IDLE_CONNS? | integer | `15` | Max idle connections |
| DB_CONN_MAX_LIFETIME? | duration | `5m` | Max connection lifetime<br>A bare integer is minutes |
| DB_CONN_MAX_IDLE_TIME? | duration | `2m` | Max connection idle time<br>A bare integer is minutes |

## Tokens

###### Token Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| JWT_ACCESS_TOKEN_DURATION? | duration | `15m` | [Access token](./tokens.md#access-token) lifetime<br>Unit required |
| JWT_REFRESH_TOKEN_DURATION? | duration | `7d` | [Refresh token](./tokens.md#refresh-token) and cookie lifetime<br>Unit required |
| JWT_ISSUER? | string | — | `iss` claim, set and verified |
| JWT_AUDIENCE? | string | — | `aud` claim, set and verified |
| IDENTITY_JWT_SECRET? | string | — | At least 32 bytes<br>[Verify](../resources/verify.md) returns a signed [identity token](../resources/verify.md#identity-token-object) instead of plain headers |
| JWT_DEVICE_TAG_SECRET? | string | — | At least 32 bytes<br>Secret for generating the token's `device_tag` |

## Identity Headers

Names of the [user headers](../resources/verify.md#user-headers) Verify returns.

Only this server may produce them, since requests carrying them are checked against the access token.

###### Identity Header Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| IDENTITY_HEADER? | string | `X-Corvauth-Identity` | Identity token, when `IDENTITY_JWT_SECRET` is set |
| USER_EMAIL_HEADER? | string | `X-Corvauth-User-Email` | User email |
| DEVICE_ID_HEADER? | string | `X-Corvauth-Device-ID` | Device ID |
| USERNAME_HEADER? | string | `X-Corvauth-Username` | Username |

::: info
Each must be a valid header name, and all four must differ (case-insensitive)
:::

## Sign-in Policy

###### Sign-in Policy Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| ALLOW_REGISTRATION? | boolean | `false` | Insert first-time users into `users`<br>Managed users only |
| ALLOW_UNVERIFIED_EMAIL? | boolean | `false` | Accept unverified emails<br>By default `email_verified` must be true |
| PASS_OAUTH_TOKEN? | boolean | `false` | Return provider tokens to the client<br>See [Provider Tokens](./oauth2.md#provider-tokens) |
| PROVIDER_ACCESS_TOKEN_HEADER? | string | `X-Corvauth-Provider-Access-Token` | Header carrying the provider access token when `PASS_OAUTH_TOKEN=true` |
| PROVIDER_REFRESH_TOKEN_HEADER? | string | `X-Corvauth-Provider-Refresh-Token` | Header carrying the provider refresh token when `PASS_OAUTH_TOKEN=true`<br>Must be a valid header name different from the access token header (case-insensitive) |
