# Logging

log 輸出到 stdout，每個失敗的 request 輸出一筆，帶 `event`（大分類）、`reason`（細項）與 `clientIp`

```json
{"level":"WARN","msg":"request rejected","clientIp":"203.0.113.7","clientIpSource":"header","method":"GET","path":"/verify","status":401,"event":"rejected","reason":"invalid_signature","error":"..."}
```

## Event

- `rejected`（WARN）：response 帶 `X-Auth-Error`（見 [API](api.zh-TW.md)）
  ```
  device_not_found         token 對應的 device 已不存在
  invalid_signature        簽章與 device secret 不符
  invalid_device_tag       device tag 不是本 server 簽發
  unexpected_algorithm     token 使用非預期的簽章演算法
  invalid_issuer           "/callback" 的 "iss" 與 provider 不符
  spoofed_identity_header  request 帶有與 access token 不符的 identity header
  ```
- `unauthenticated`（INFO）：401 且無 `X-Auth-Error`
  ```
  token_missing          沒有 Bearer token
  token_invalid          access token 過期、格式錯誤或已撤銷
  refresh_token_missing  cookie 或 header 中沒有 refresh token
  refresh_token_invalid  refresh token 過期或格式錯誤
  refresh_token_reused   送來已 rotate 過的 refresh token，對應 device 已撤銷
  ```
- `invalid_request`（WARN）：400
  ```
  body_invalid              "POST /sessions" 的 body 不是合法 JSON
  redirect_uri_missing      沒有 "redirect_uri" 且未設定預設值
  redirect_uri_not_allowed  "redirect_uri" 不在允許清單
  state_missing             "/callback" 缺少 "state"
  state_invalid             "state" 不存在、已過期或已使用
  code_missing              "/callback" 缺少 "code"
  pkce_invalid              "X-PKCE-Verifier" 不符
  ```
- `denied`（INFO）：使用者不允許或 provider 拒絕
  ```
  user_not_found                      使用者不存在且未開放註冊，或已被刪除
  provider_access_denied              使用者在 provider 拒絕授權
  provider_invalid_request            provider 拒絕授權請求
  provider_unauthorized_client        client 不允許使用此流程
  provider_unsupported_response_type  provider 不支援該 response type
  provider_invalid_scope              provider 拒絕請求的 scope
  provider_error                      無法辨識的 provider "error"
  ```
- `upstream_error`（ERROR）：OAuth2 provider 失敗
  ```
  provider_timeout                  code exchange 逾時
  code_exchange_failed              code exchange 失敗
  userinfo_failed                   取得 user info 失敗
  provider_server_error             provider 回傳 "server_error"
  provider_temporarily_unavailable  provider 回傳 "temporarily_unavailable"
  ```
- `internal_error`（ERROR）：內部失敗
  ```
  session_init_failed      儲存登入 "state" 失敗
  state_lookup_failed      讀取登入 "state" 失敗
  user_create_failed       註冊使用者失敗
  user_lookup_failed       查詢 username 失敗
  device_register_failed   儲存 device session 失敗
  device_delete_failed     登出或 refresh token reuse 時刪除 device 失敗
  token_issue_failed       產生 refresh token 或 access token 失敗
  refresh_rotate_failed    rotate refresh token 失敗
  session_list_failed      列出 session 失敗
  session_revoke_failed    刪除 device session 失敗
  account_delete_failed    刪除使用者失敗
  identity_sign_failed     簽署 identity token 失敗
  database_cleanup_failed  定期 database cleanup 失敗
  unexpected_result        登入 exchange 沒有產生可用結果
  ```
- `degraded`（WARN）：非 request 結果
  ```
  flight_redis_unavailable  Redis 讀寫共用結果失敗，改在本機執行
  flight_lock_unavailable   無法取得 distributed lock，改在本機執行
  flight_result_corrupt     共用結果無法 encode 或 decode
  cache_redis_failed        Redis 中 device secret cache 讀寫失敗
  oidc_discovery_retry      啟動時取得 OIDC discovery document 失敗，重試中
  ```

## Client IP

取 `TRUSTED_CLIENT_IP_HEADER` 最右邊的項目，未設則用 remote address；header 不存在或不合法時為空
