# Status Codes

## HTTP Response Codes

| Code | Meaning |
|---|---|
| 200 (OK) | The request succeeded |
| 204 (No Content) | The access token is valid<br>User headers only<br>See [Verify](../resources/verify.md) |
| 400 (Bad Request) | The request is malformed, or the provider rejected the authorization request |
| 401 (Unauthorized) | The token is missing, invalid, or expired<br>On [Complete Login](../resources/session.md#get-sessions-callback), the user is not allowed to sign in |
| 403 (Forbidden) | Identity headers in the request do not match the access token, or the user declined consent at the provider |
| 500 (Internal Server Error) | An internal failure, or the code exchange or user info request to the provider failed |
| 502 (Bad Gateway) | The provider returned `server_error` |
| 503 (Service Unavailable) | The provider returned `temporarily_unavailable` |
| 504 (Gateway Timeout) | The code exchange with the provider timed out |

A `401` for a missing or invalid token carries `WWW-Authenticate: Bearer`, e.g. `Bearer error="invalid_token"`, so clients know to refresh.

The `reason` of each failure is written to the [log](./logging.md#events), not to the response.

## Auth Error Header

`X-Auth-Error` tells a gateway why a request was rejected when the cause is not an ordinary expired or malformed token.

It marks requests that most likely come from unnatural operations, such as forged or tampered tokens and spoofed identity headers, rather than normal client behavior.

###### Device Session Errors

Added to `401` by [Rotate Refresh Token](../resources/session.md#post-sessions-current-tokens), [Create Access Token](../resources/session.md#post-sessions-current-access-token), [Get Current Session](../resources/session.md#get-sessions-current), [Get Device Sessions](../resources/session.md#get-sessions), [Delete Device Sessions](../resources/session.md#delete-sessions), [Delete Current User](../resources/user.md#delete-users-me), and both [Verify](../resources/verify.md) endpoints

| Value | Cause |
|---|---|
| device_not_found | The device session no longer exists |
| invalid_signature | The signature does not match the device secret |
| invalid_device_tag | The token's `device_tag` does not match its `device_id` and `exp`<br>Only when `JWT_DEVICE_TAG_SECRET` is set |
| unexpected_algorithm | The token is not signed with `HS256` |

###### Issuer Errors

Added to `400` by [Complete Login](../resources/session.md#get-sessions-callback)

| Value | Cause |
|---|---|
| invalid_issuer | The authorization response `iss` does not match the provider, or is missing when the provider declares it |

###### Identity Header Errors

Added to `403` by both [Verify](../resources/verify.md) endpoints

| Value | Cause |
|---|---|
| spoofed_identity_header | User headers are sent without an access token, or do not match it |
