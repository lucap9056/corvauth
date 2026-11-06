# Verify API

For a reverse proxy's auth request, e.g. nginx `auth_request` or Traefik `forwardAuth`.

The proxy calls a verify endpoint with the client's access token and forwards the returned [user headers](#user-headers) upstream.

Available in session modes only.

### User Headers

Header names are configurable.

See [Identity Headers](../topics/configuration.md#identity-headers).

###### User Header Structure

| Header | Without `IDENTITY_JWT_SECRET` | With `IDENTITY_JWT_SECRET` |
|---|---|---|
| X-Corvauth-User-Email | User email | — |
| X-Corvauth-Device-ID | Device ID | — |
| X-Corvauth-Username | Username, omitted when empty | — |
| X-Corvauth-Identity | — | [Identity token](#identity-token-object) |

::: warning
Setting `IDENTITY_JWT_SECRET` is recommended.

Without it the headers are plain text:
- upstreams must only be reachable through the proxy
- the proxy must strip client-supplied copies of these headers
:::

### Identity Token Object

A JWT signed with `HS256` using `IDENTITY_JWT_SECRET`, with header `typ` set to `identity+jwt`.

Upstreams verify it with the same secret.

###### Identity Token Claims

| Field | Type | Description |
|---|---|---|
| sub | string | User email |
| username? | string | Username, omitted when empty |
| device_id | string | ID of the device session |
| iss? | string | `JWT_ISSUER`, present only when set |
| aud? | array of strings | `JWT_AUDIENCE`, present only when set |
| iat | integer | Unix seconds when issued |
| exp | integer | 1 minute after `iat`, or the access token's expiry if sooner |

###### Example Identity Token Claims

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

## Verify Access Token % GET /verify

Returns `204` with [user headers](#user-headers) and no body when the access token is valid.

User headers in the request are accepted only when they match the access token, otherwise `403`.

For example, headers forwarded by an outer layer that already called this endpoint with the same token.

::: danger
Without `Authorization: Bearer`, any user header in the request, even an empty one, is rejected with `403` and `X-Auth-Error: spoofed_identity_header` before anything else
:::

## Verify Optional Access Token % GET /verify-optional

For routes that also allow anonymous access.

Only requests with an `Authorization: Bearer` header are checked.

- Without `Authorization: Bearer`, including other schemes such as `Basic`: `204` with no user headers (anonymous)
- With `Authorization: Bearer`: checked the same way as [Verify Access Token](#get-verify)
  - An invalid or expired token still returns `401`, so clients refresh
- User headers in the request are checked the same way as [Verify Access Token](#get-verify)

::: warning
Upstreams treat missing user headers as anonymous, so the proxy must overwrite or strip client-supplied copies of them
:::
