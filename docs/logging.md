# Logging

Logs go to stdout; each failed request logs one entry with `event` (category), `reason` (detail) and `clientIp`

```json
{"level":"WARN","msg":"request rejected","clientIp":"203.0.113.7","clientIpSource":"header","method":"GET","path":"/verify","status":401,"event":"rejected","reason":"invalid_signature","error":"..."}
```

## Events

- `rejected` (WARN): response carries `X-Auth-Error` (see [API](api.md))
  ```
  device_not_found         the token's device no longer exists
  invalid_signature        signature does not match the device secret
  invalid_device_tag       device tag was not issued by this server
  unexpected_algorithm     token signed with an unexpected algorithm
  invalid_issuer           "iss" on "/callback" does not match the provider
  spoofed_identity_header  request carries identity headers that do not match the access token
  ```
- `unauthenticated` (INFO): 401 without `X-Auth-Error`
  ```
  token_missing          no Bearer token
  token_invalid          access token expired, malformed or revoked
  refresh_token_missing  no refresh token in the cookie or header
  refresh_token_invalid  refresh token expired or malformed
  refresh_token_reused   an already rotated refresh token was presented and its device revoked
  ```
- `invalid_request` (WARN): 400
  ```
  body_invalid              "POST /sessions" body is not valid JSON
  redirect_uri_missing      no "redirect_uri" and no default configured
  redirect_uri_not_allowed  "redirect_uri" is not in the allowed list
  state_missing             "/callback" without "state"
  state_invalid             "state" unknown, expired or already used
  code_missing              "/callback" without "code"
  pkce_invalid              "X-PKCE-Verifier" does not match
  ```
- `denied` (INFO): user not allowed or provider denied
  ```
  user_not_found                      user does not exist and registration is off, or was deleted
  provider_access_denied              user declined consent at the provider
  provider_invalid_request            provider rejected the authorization request
  provider_unauthorized_client        client not allowed to use this flow
  provider_unsupported_response_type  provider does not support the response type
  provider_invalid_scope              requested scope rejected by the provider
  provider_error                      unrecognised provider "error"
  ```
- `upstream_error` (ERROR): OAuth2 provider failure
  ```
  provider_timeout                  code exchange timed out
  code_exchange_failed              code exchange failed
  userinfo_failed                   fetching user info failed
  provider_server_error             provider returned "server_error"
  provider_temporarily_unavailable  provider returned "temporarily_unavailable"
  ```
- `internal_error` (ERROR): internal failure
  ```
  session_init_failed      storing the login "state" failed
  state_lookup_failed      reading the login "state" failed
  user_create_failed       registering the user failed
  user_lookup_failed       fetching the username failed
  device_register_failed   saving the device session failed
  device_delete_failed     deleting a device on logout or refresh token reuse failed
  token_issue_failed       generating a refresh or access token failed
  refresh_rotate_failed    rotating the refresh token failed
  session_list_failed      listing sessions failed
  session_revoke_failed    deleting device sessions failed
  account_delete_failed    deleting the user failed
  identity_sign_failed     signing the identity token failed
  database_cleanup_failed  periodic database cleanup failed
  unexpected_result        login exchange produced no usable result
  ```
- `degraded` (WARN): not a request outcome
  ```
  flight_redis_unavailable  Redis read or write of a shared result failed; runs locally
  flight_lock_unavailable   distributed lock unavailable; runs locally
  flight_result_corrupt     shared result could not be encoded or decoded
  cache_redis_failed        device secret cache read or write in Redis failed
  oidc_discovery_retry      fetching the OIDC discovery document failed on startup; retrying
  ```

## Client IP

From the right-most entry of `TRUSTED_CLIENT_IP_HEADER`, otherwise the remote address; empty when the header is absent or invalid
