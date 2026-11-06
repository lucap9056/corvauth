# Tokens

Session modes issue two JWTs per [device session](../resources/session.md#device-session-object), both signed with `HS256` using that device's own secret.

## Access Token

Short-lived (`JWT_ACCESS_TOKEN_DURATION`, default `15m`), sent as `Authorization: Bearer <access_token>`

###### Access Token Claims

| Field | Type | Description |
|---|---|---|
| user_email | string | The user's email |
| device_id | string | ID of the device session |
| username | string | The user's display name<br>See [Username Updates](#username-updates) |
| device_tag? | string | Binds the token to its `device_id` and `exp`<br>Present when `JWT_DEVICE_TAG_SECRET` is set |
| generation | integer | Rotation generation of the device session |
| iss? | string | `JWT_ISSUER`, set and verified when configured |
| aud? | array of strings | `JWT_AUDIENCE`, set and verified when configured |
| exp | integer | Expiry in Unix seconds |

## Refresh Token

Long-lived (`JWT_REFRESH_TOKEN_DURATION`, default `7d`), read from the `refresh_token` cookie, or a JSON body `{ "refresh_token": "..." }` when the cookie is absent

###### Refresh Token Cookie Attributes

| Attribute | Value |
|---|---|
| Name | `refresh_token` |
| Path | `/` |
| HttpOnly | always |
| Secure | unless `HTTP_MODE=development` |
| SameSite | `Lax` |
| Expires | together with the token |

::: tip
Pages that cannot read the `HttpOnly` cookie get its expiry from [Get Current Session](../resources/session.md#get-sessions-current)
:::

## Rotation

Each [Rotate Refresh Token](../resources/session.md#post-sessions-current-tokens) call issues a new refresh token and invalidates the old one, so active sessions keep extending

- Concurrent rotation calls with the same token share one rotation (across instances with Redis)
- [Create Access Token](../resources/session.md#post-sessions-current-access-token) issues an access token without rotating

## Reuse Detection

Presenting an already-rotated refresh token is treated as theft: the device session is deleted, and the request fails with `401` (`refresh_token_reused` in [Logging](./logging.md#unauthenticated-reasons))

## Username Updates

`username` is re-read on login and every refresh, so renames apply within one access token lifetime

## Device Revocation

Deleting a device session ([Delete Current Session](../resources/session.md#delete-sessions-current), [Delete Device Sessions](../resources/session.md#delete-sessions), or reuse detection) removes its secret, so its access tokens fail [Verify Access Token](../resources/verify.md#get-verify) immediately with `X-Auth-Error: device_not_found`
