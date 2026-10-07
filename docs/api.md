# API

- JSON responses use `{ "success": bool, "message": ... }`
- Errors return `success: false` with a string `message`
- Missing or invalid token: `401` + `WWW-Authenticate: Bearer`
- Refresh token input: the `refresh_token` cookie, or a JSON body `{ "refresh_token": "..." }` when the cookie is absent

| Endpoint | Available in | Input | Success |
|---|---|---|---|
| [`GET /health`](#get-health) | all modes | — | `200` |
| [`GET /login`](#get-login) | all modes | — | `200` |
| [`GET /callback`](#get-callback) | all modes | `code`, `state` query | `200` |
| [`POST /refresh`](#post-refresh) | session modes | `refresh_token` cookie or body | `200`, sets cookie |
| [`POST /refresh-access`](#post-refresh-access) | session modes | `refresh_token` cookie or body | `200` |
| [`POST /refresh-status`](#post-refresh-status) | session modes | `refresh_token` cookie or body | `200`, no rotation |
| [`GET /verify`](#get-verify) | session modes | `Authorization: Bearer` access token | `204` + user headers |
| [`POST /logout`](#post-logout) | session modes | `refresh_token` cookie or body (optional) | `200`, deletes the device session, clears the cookie |
| [`DELETE /users/me`](#delete-usersme) | managed users | `Authorization: Bearer` access token | `200`, deletes all sessions and the user, clears the cookie |

## `GET /health`

Plain text:

```text
OK
```

## `GET /login`

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

## `POST /refresh`

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

## `POST /refresh-access`

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

## `POST /refresh-status`

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

## `POST /logout`

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

When a `401` is caused by the device session, `/refresh`, `/refresh-access`, `/refresh-status`, `/verify`, and `DELETE /users/me` add `X-Auth-Error`, so a gateway can tell it apart from expired or malformed tokens:

| Value | Cause |
|---|---|
| `device_not_found` | The device session no longer exists |
| `invalid_signature` | The signature does not match the device secret |
