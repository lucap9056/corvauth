# Session API

Every login creates a device session with its own signing secret.

Logging out or deleting a device only affects that device's tokens.

See [OAuth2](../topics/oauth2.md) for the login flow and [Tokens](../topics/tokens.md) for token lifetimes.

::: info
Except [Start Login](#post-sessions) and [Complete Login](#get-callback), these endpoints are available in session modes only
:::

### Device Session Object

###### Device Session Structure

| Field | Type | Description |
|---|---|---|
| device_id | string | ID of the device session |
| device_name | string | `X-Device-Name` sent to [Complete Login](#get-callback), `Unknown Device` by default |
| updated_at | integer | Unix seconds of the last login or refresh |
| current | boolean | Whether the access token of the request belongs to this session |

###### Example Device Session

```json
{
  "device_id": "3f1c2a9e-...",
  "device_name": "My Laptop",
  "updated_at": 1767225600,
  "current": true
}
```

### Token Pair Object

###### Token Pair Structure

| Field | Type | Description |
|---|---|---|
| access_token | string | The [access token](../topics/tokens.md#access-token) in session modes<br>The provider's access token in stateless proxy |
| refresh_token | string | The [refresh token](../topics/tokens.md#refresh-token) in session modes<br>The provider's refresh token in stateless proxy |

### Session Status Object

###### Session Status Structure

| Field | Type | Description |
|---|---|---|
| device_id | string | ID of the device session |
| issued_at | integer | Unix seconds when the refresh token was issued |
| expires_at | integer | Unix seconds when the refresh token expires |

### Authorization Object

###### Authorization Structure

| Field | Type | Description |
|---|---|---|
| url | string | The provider's authorization URL to redirect the user to |
| verifier? | string | PKCE verifier<br>Present only when `OAUTH2_CLIENT_PKCE=true`<br>See [Client PKCE](../topics/oauth2.md#client-pkce) |

## Start Login % POST /sessions

Starts a login.

Returns an [authorization](#authorization-object) object.

The device session is created later by [Complete Login](#get-callback).

###### JSON Params

| Field | Type | Description |
|---|---|---|
| redirect_uri? | string | Must exactly match `OAUTH2_REDIRECT_URL` or an entry in `OAUTH2_ALLOWED_REDIRECT_URLS`, otherwise `400`<br>Omitted: `OAUTH2_REDIRECT_URL`, or `400` when unset |

## Complete Login % GET /callback

Exchanges the authorization code for tokens.

The redirect URL page passes the `code` and `state` it received from the provider.

- Session modes: returns a [token pair](#token-pair-object) object and sets the `refresh_token` cookie
- Stateless proxy: returns the provider's tokens as a token pair object
  - With `PASS_OAUTH_TOKEN=true` they move to headers and `message` is `Logged in`
  - See [Provider Tokens](../topics/oauth2.md#provider-tokens)

###### Query String Params

| Field | Type | Description |
|---|---|---|
| code | string | Authorization code from the provider |
| state | string | One-time `state` from [Start Login](#post-sessions)<br>Deleted on use |
| iss? | string | Authorization response issuer, checked when the provider declares it |
| error? | string | Error returned by the provider |

###### Request Headers

| Header | Description |
|---|---|
| X-Device-Name? | Name of the new device session, `Unknown Device` by default |
| X-PKCE-Verifier? | Required when `OAUTH2_CLIENT_PKCE=true` |

::: warning
A mismatched `iss` returns `400` with `X-Auth-Error: invalid_issuer`

See [Issuer Errors](../topics/status-codes.md#issuer-errors)
:::

## Get Device Sessions % GET /sessions

Returns an array of every [device session](#device-session-object) of the user, most recently refreshed first.

Requires an access token.

## Delete Device Sessions % DELETE /sessions

Revokes every device session of the user, including the current one, and clears the cookie.

Their access tokens fail [Verify Access Token](./verify.md#get-verify) immediately.

Requires an access token.

## Rotate Refresh Token % POST /sessions/current/tokens

Rotates the refresh token.

Returns a new [token pair](#token-pair-object) object and sets the `refresh_token` cookie.

See [Rotation](../topics/tokens.md#rotation).

###### JSON Params

| Field | Type | Description |
|---|---|---|
| refresh_token? | string | Read only when the `refresh_token` cookie is absent |

::: danger
Beware of [Reuse Detection](../topics/tokens.md#reuse-detection)
:::

## Create Access Token % POST /sessions/current/access-token

Issues a new access token without rotating the refresh token.

`message` is the new access token string.

###### JSON Params

| Field | Type | Description |
|---|---|---|
| refresh_token? | string | Read only when the `refresh_token` cookie is absent |

## Get Current Session % GET /sessions/current

Returns a [session status](#session-status-object) object for the refresh token, without rotating it.

Reads the `refresh_token` cookie, or `Authorization: Bearer <refresh_token>` when the cookie is absent.

The body is ignored.

## Delete Current Session % DELETE /sessions/current

Logs out: deletes the device session and clears the cookie.

The refresh token is optional, so the cookie is cleared even without one.

###### JSON Params

| Field | Type | Description |
|---|---|---|
| refresh_token? | string | Read only when the `refresh_token` cookie is absent |
