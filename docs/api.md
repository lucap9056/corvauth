# API

- JSON response shape: `{ "success": bool, "message": ... }`
- Missing or invalid token: `401` + `WWW-Authenticate: Bearer`

| Endpoint | Available in | Input | Success |
|---|---|---|---|
| `GET /health` | all modes | — | `200` |
| `GET /login` | all modes | — | `200`, `message`: `{ "url", "verifier"? }` |
| `GET /callback` | all modes | `code`, `state` query | `200`, `message`: `{ "access_token", "refresh_token" }` (see below) |
| `POST /refresh` | session modes | refresh token | `200`, `message`: new token pair, sets cookie |
| `POST /refresh-access` | session modes | refresh token | `200`, `message`: new access token |
| `POST /refresh-status` | session modes | refresh token | `200`, `message`: `{ "device_id", "issued_at", "expires_at" }`, no rotation |
| `GET /verify` | session modes | Bearer access token | `204` + user headers (see [Identity Token](#identity-token)) |
| `POST /logout` | session modes | refresh token (optional) | `200`, deletes the device session, clears the cookie |
| `DELETE /users/me` | managed users | Bearer access token | `200`, deletes all sessions and the user |

Tokens returned by `/callback`:

- Session modes: session tokens, and the cookie is set
- Stateless proxy: the provider's tokens

`/verify` is for a reverse proxy's auth request (e.g. nginx `auth_request`, Traefik `forwardAuth`); forward the returned headers upstream

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
