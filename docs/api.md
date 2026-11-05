# API

- JSON responses use `{ "success": bool, "message": ... }`
- Errors return `success: false` with a string `message`
- Missing or invalid token: `401` + `WWW-Authenticate: Bearer`
- Refresh token input: the `refresh_token` cookie, or a JSON body `{ "refresh_token": "..." }` when the cookie is absent (`GET /sessions/current` reads `Authorization: Bearer` instead of the body)

| Endpoint | Available in | Input | Success |
|---|---|---|---|
| [`GET /health`](#get-health) | all modes | — | `200` |
| [`POST /sessions`](#post-sessions) | all modes | `redirect_uri` body (optional) | `200` |
| [`GET /callback`](#get-callback) | all modes | `code`, `state` query | `200` |
| [`POST /sessions/current/tokens`](#post-sessionscurrenttokens) | session modes | `refresh_token` cookie or body | `200`, sets cookie |
| [`POST /sessions/current/access-token`](#post-sessionscurrentaccess-token) | session modes | `refresh_token` cookie or body | `200` |
| [`GET /sessions/current`](#get-sessionscurrent) | session modes | `refresh_token` cookie or `Authorization: Bearer` refresh token | `200`, no rotation |
| [`GET /verify`](#get-verify) | session modes | `Authorization: Bearer` access token | `204` + user headers |
| [`GET /verify-optional`](#get-verify-optional) | session modes | `Authorization: Bearer` access token, checked only when present | `204`, user headers only when signed in |
| [`DELETE /sessions/current`](#delete-sessionscurrent) | session modes | `refresh_token` cookie or body (optional) | `200`, deletes the device session, clears the cookie |
| [`GET /sessions`](#get-sessions) | session modes | `Authorization: Bearer` access token | `200`, lists the user's device sessions |
| [`DELETE /sessions`](#delete-sessions) | session modes | `Authorization: Bearer` access token | `200`, deletes all of the user's device sessions, clears the cookie |
| [`DELETE /users/me`](#delete-usersme) | managed users | `Authorization: Bearer` access token | `200`, deletes all sessions and the user, clears the cookie |

## `GET /health`

Plain text:

```text
OK
```

## `POST /sessions`

Starts a login; the device session is created by `/callback`

Request (optional):

```json
{
  "redirect_uri": "https://app.example.com/callback"
}
```

- `redirect_uri`: must exactly match `OAUTH2_REDIRECT_URL` or an entry in `OAUTH2_ALLOWED_REDIRECT_URLS`, otherwise `400`; omitted: `OAUTH2_REDIRECT_URL`, or `400` when unset

```json
{
  "success": true,
  "message": {
    "url": "https://provider.example.com/authorize?...",
    "verifier": "..."
  }
}
```

- `verifier`: present only when `OAUTH2_CLIENT_PKCE=true`

## `GET /callback`

Optional request headers are in [Login Flow](sessions.md#login-flow)

Session modes return session tokens and set the `refresh_token` cookie; stateless proxy returns the provider's tokens:

```json
{
  "success": true,
  "message": {
    "access_token": "...",
    "refresh_token": "..."
  }
}
```

Stateless proxy with `PASS_OAUTH_TOKEN=true` moves the tokens to headers instead (see [Provider Tokens](sessions.md#provider-tokens)):

```json
{
  "success": true,
  "message": "Logged in"
}
```

## `POST /sessions/current/tokens`

Rotates the refresh token

Request: `refresh_token` cookie, or body:

```json
{
  "refresh_token": "..."
}
```

Response:

```json
{
  "success": true,
  "message": {
    "access_token": "...",
    "refresh_token": "..."
  }
}
```

## `POST /sessions/current/access-token`

Issues a new access token without rotating the refresh token

Request: `refresh_token` cookie, or body:

```json
{
  "refresh_token": "..."
}
```

Response, `message` is the new access token:

```json
{
  "success": true,
  "message": "..."
}
```

## `GET /sessions/current`

Request: `refresh_token` cookie, or `Authorization: Bearer <refresh_token>` when the cookie is absent; the body is ignored

Response:

```json
{
  "success": true,
  "message": {
    "device_id": "3f1c2a9e-...",
    "issued_at": 1767225600,
    "expires_at": 1769817600
  }
}
```

- `issued_at` / `expires_at`: Unix seconds of the refresh token

## `GET /verify`

No body; returns user headers (see [Identity Token](#identity-token))

For a reverse proxy's auth request (e.g. nginx `auth_request`, Traefik `forwardAuth`); forward the returned headers upstream

## `GET /verify-optional`

For routes that also allow anonymous access; only requests with an `Authorization: Bearer` header are checked for validity

- Without `Authorization: Bearer`, including other schemes such as `Basic`: `204`, no user headers (anonymous)
- With `Authorization: Bearer`: checked the same way as `/verify`, including the `401` for an invalid or expired token, so clients still refresh
- Upstreams treat missing user headers as anonymous, so the proxy must overwrite or strip client-supplied copies of them

## `DELETE /sessions/current`

Request (optional): `refresh_token` cookie, or body:

```json
{
  "refresh_token": "..."
}
```

Response:

```json
{
  "success": true,
  "message": "Logged out and device session revoked"
}
```

## `GET /sessions`

Lists every device session of the user, most recently refreshed first

```json
{
  "success": true,
  "message": [
    {
      "device_id": "3f1c2a9e-...",
      "device_name": "Unknown Device",
      "updated_at": 1767225600,
      "current": true
    }
  ]
}
```

- `device_name`: the `X-Device-Name` sent to `/callback`
- `updated_at`: Unix seconds of the last login or refresh
- `current`: the session the access token belongs to

## `DELETE /sessions`

Revokes every device session of the user, including the current one; their access tokens fail `/verify` immediately

```json
{
  "success": true,
  "message": "Logged out from all sessions"
}
```

## `DELETE /users/me`

```json
{
  "success": true,
  "message": "Account deleted"
}
```

## Identity Token

Setting `IDENTITY_JWT_SECRET` is recommended

- Set: `/verify` returns a signed header, verified upstream with the same secret
- Unset: headers are plain text
  - upstreams must only be reachable through the proxy
  - the proxy must strip client-supplied copies of these headers

| Header | Unset | Set |
|---|---|---|
| `X-Forwarded-User-Email` | User email | — |
| `X-Forwarded-Device-ID` | Device ID | — |
| `X-Forwarded-Username` | Username, omitted when empty | — |
| `X-Forwarded-Identity` | — | Identity token |

The identity token is signed with `HS256`, header `typ`: `identity+jwt`:

```json
{
  "sub": "user@example.com",
  "username": "User",
  "device_id": "3f1c2a9e-...",
  "iss": "https://auth.example.com",
  "aud": ["api"],
  "iat": 1767225600,
  "exp": 1767225660
}
```

- `username`: omitted when empty
- `iss` / `aud`: present only when `JWT_ISSUER` / `JWT_AUDIENCE` are set
- `exp`: 1 minute after `iat`, or the access token's expiry if sooner

## Error Headers

When a `401` is caused by the device session, `/sessions/current/tokens`, `/sessions/current/access-token`, `GET /sessions/current`, `GET /sessions`, `DELETE /sessions`, `/verify`, `/verify-optional`, and `DELETE /users/me` add `X-Auth-Error`, so a gateway can tell it apart from expired or malformed tokens:

| Value | Cause |
|---|---|
| `device_not_found` | The device session no longer exists |
| `invalid_signature` | The signature does not match the device secret |
| `invalid_device_tag` | The token's `device_tag` does not match its `device_id` and `exp` (only when `JWT_DEVICE_TAG_SECRET` is set) |
| `unexpected_algorithm` | The token is not signed with `HS256` |

`GET /callback` adds `X-Auth-Error` to the `400` caused by the authorization response `iss`:

| Value | Cause |
|---|---|
| `invalid_issuer` | `iss` does not match the provider, or is missing when the provider declares it |
