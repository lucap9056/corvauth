# Logging

Logs go to stdout, formatted by `LOG_FORMAT` and filtered by `LOG_LEVEL` (see [Logging Variables](./configuration.md#logging)).

Each failed request logs one entry.

## Log Entry

###### Failed Request Log Structure

| Field | Type | Description |
|---|---|---|
| level | string | `DEBUG`, `INFO`, `WARN`, or `ERROR`, set by the [event type](#event-types) |
| msg | string | Human-readable summary |
| clientIp | string | The resolved [client IP](#client-ip), empty when unresolved |
| clientIpSource | string | Where `clientIp` came from<br>See [Client IP Sources](#client-ip-sources) |
| method | string | Request method |
| path | string | Request path |
| status | integer | Response status code |
| event | string | The [event type](#event-types) |
| reason | string | The detailed cause, listed per event below |
| error? | string | The underlying error, when there is one |

###### Example Log Entry

```json
{"level":"WARN","msg":"request rejected","clientIp":"203.0.113.7","clientIpSource":"header","method":"GET","path":"/verify","status":401,"event":"rejected","reason":"invalid_signature","error":"..."}
```

## Events

###### Event Types

| Event | Level | Description |
|---|---|---|
| rejected | WARN | The response carries [`X-Auth-Error`](./status-codes.md#auth-error-header) |
| unauthenticated | INFO | `401` without `X-Auth-Error` |
| invalid_request | WARN | `400` |
| denied | INFO | The user is not allowed, or the provider denied |
| upstream_error | ERROR | OAuth2 provider failure |
| internal_error | ERROR | Internal failure |
| degraded | WARN | Not a request outcome |

###### Rejected Reasons

| Reason | Description |
|---|---|
| device_not_found | The token's device no longer exists |
| invalid_signature | The signature does not match the device secret |
| invalid_device_tag | The device tag was not issued by this server |
| unexpected_algorithm | The token is signed with an unexpected algorithm |
| invalid_issuer | `iss` on [Complete Login](../resources/session.md#get-sessions-callback) does not match the provider |
| spoofed_identity_header | The request carries identity headers that do not match the access token |

###### Unauthenticated Reasons

| Reason | Description |
|---|---|
| token_missing | No Bearer token |
| token_invalid | The access token is expired, malformed, or revoked |
| refresh_token_missing | No refresh token in the cookie or header |
| refresh_token_invalid | The refresh token is expired or malformed |
| refresh_token_reused | An already rotated refresh token was presented and its device revoked<br>See [Reuse Detection](./tokens.md#reuse-detection) |

###### Invalid Request Reasons

| Reason | Description |
|---|---|
| body_invalid | The [Start Login](../resources/session.md#post-sessions) body is not valid JSON |
| redirect_uri_missing | No `redirect_uri` and no default configured |
| redirect_uri_not_allowed | `redirect_uri` is not in the allowed list |
| state_missing | [Complete Login](../resources/session.md#get-sessions-callback) without `state` |
| state_invalid | `state` is unknown, expired, or already used |
| code_missing | [Complete Login](../resources/session.md#get-sessions-callback) without `code` |
| pkce_invalid | `X-PKCE-Verifier` does not match |

###### Denied Reasons

| Reason | Description |
|---|---|
| user_not_found | The user does not exist and registration is off, or was deleted |
| provider_access_denied | The user declined consent at the provider |
| provider_invalid_request | The provider rejected the authorization request |
| provider_unauthorized_client | The client is not allowed to use this flow |
| provider_unsupported_response_type | The provider does not support the response type |
| provider_invalid_scope | The requested scope was rejected by the provider |
| provider_error | Unrecognised provider `error` |

###### Upstream Error Reasons

| Reason | Description |
|---|---|
| provider_timeout | The code exchange timed out |
| code_exchange_failed | The code exchange failed |
| userinfo_failed | Fetching user info failed |
| provider_server_error | The provider returned `server_error` |
| provider_temporarily_unavailable | The provider returned `temporarily_unavailable` |

###### Internal Error Reasons

| Reason | Description |
|---|---|
| session_init_failed | Storing the login `state` failed |
| state_lookup_failed | Reading the login `state` failed |
| user_create_failed | Registering the user failed |
| user_lookup_failed | Fetching the username failed |
| device_register_failed | Saving the device session failed |
| device_delete_failed | Deleting a device on logout or refresh token reuse failed |
| token_issue_failed | Generating a refresh or access token failed |
| refresh_rotate_failed | Rotating the refresh token failed |
| session_list_failed | Listing sessions failed |
| session_revoke_failed | Deleting device sessions failed |
| account_delete_failed | Deleting the user failed |
| identity_sign_failed | Signing the identity token failed |
| database_cleanup_failed | Periodic database cleanup failed |
| unexpected_result | The login exchange produced no usable result |

###### Degraded Reasons

| Reason | Description |
|---|---|
| flight_redis_unavailable | A Redis read or write of a shared result failed<br>Runs locally |
| flight_lock_unavailable | The distributed lock is unavailable<br>Runs locally |
| flight_result_corrupt | A shared result could not be encoded or decoded |
| cache_redis_failed | A device secret cache read or write in Redis failed |
| oidc_discovery_retry | Fetching the OIDC discovery document failed on startup<br>Retrying |

## Client IP

Taken from the right-most entry of `TRUSTED_CLIENT_IP_HEADER` when it is set, otherwise from the remote address

###### Client IP Sources

| Value | Description |
|---|---|
| remote_addr | `TRUSTED_CLIENT_IP_HEADER` is unset<br>Taken from the remote address |
| header | Taken from `TRUSTED_CLIENT_IP_HEADER` |
| missing | The header is absent<br>`clientIp` is empty |
| invalid | The value is not a valid IP<br>`clientIp` is empty |
