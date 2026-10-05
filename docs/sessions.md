# Sessions

## Login Flow

1. `GET /login` returns `{ "url": "..." }`; redirect the user there
   - The server generates a PKCE verifier and stores it under a one-time `state` (Redis if `REDIS_URL` is set, otherwise memory)
2. The provider redirects back to `GET /callback?code=...&state=...`
   - The server takes the verifier by `state`, deletes it to prevent replay, then exchanges the code
3. In session modes, the server reads the email, creates a device session, sets the `refresh_token` cookie, and returns both tokens

Optional `/callback` request headers:

| Header | Purpose |
|---|---|
| `X-Device-Name` | Device session name (default `Unknown Device`) |
| `X-PKCE-Verifier` | Required when `OAUTH2_CLIENT_PKCE=true` |

`OAUTH2_CLIENT_PKCE=true` is for clients that drive PKCE themselves: `/login` also returns `verifier`, which the client sends back in `X-PKCE-Verifier`

## Tokens

- **Access token**: short-lived (`JWT_ACCESS_TOKEN_DURATION`), sent as `Authorization: Bearer <token>`
  - Claims include `user_email`, `device_id`, `username`
- **Refresh token**: read from the `refresh_token` cookie or JSON body `{ "refresh_token": "..." }`
  - Cookie is `HttpOnly`, `SameSite=Lax`, and expires with the token
  - Pages that cannot read the cookie get its expiry from `/refresh-status`
- **Rotation**: each `/refresh` issues a new token and invalidates the old one, so active sessions keep extending
  - Concurrent `/refresh` calls with the same token share one rotation (across instances with Redis)
- **Reuse detection**: an already-rotated token is treated as theft and deletes the device session
- **Username updates**: `username` is re-read on login and every refresh; renames apply within one access token lifetime

## Provider Tokens

In session modes, `PASS_OAUTH_TOKEN` decides what happens to the provider's tokens:

- `false` (default): revoked and discarded right after login
- `true`: returned in the `X-Forwarded-Access-Token` and `X-Forwarded-Refresh-Token` headers of `/callback`; the client keeps them

Stateless proxy always returns them:

- in the response body by default
- only in the headers above when `PASS_OAUTH_TOKEN=true`
