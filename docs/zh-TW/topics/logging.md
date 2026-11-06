# Logging

Log 輸出到 stdout，格式由 `LOG_FORMAT` 決定、以 `LOG_LEVEL` 過濾（見 [Logging 變數](./configuration.md#logging)）。

每個失敗的 request 記錄一筆。

## Log Entry {#log-entry}

###### 失敗 Request 的 Log 結構 {#failed-request-log-structure}

| 欄位 | 型別 | 說明 |
|---|---|---|
| level | string | `DEBUG`、`INFO`、`WARN` 或 `ERROR`，由 [event 類型](#event-types) 決定 |
| msg | string | 人類可讀的摘要 |
| clientIp | string | 判定出的 [client IP](#client-ip)，無法判定時為空 |
| clientIpSource | string | `clientIp` 的來源<br>見 [Client IP 來源](#client-ip-sources) |
| method | string | Request method |
| path | string | Request path |
| status | integer | Response status code |
| event | string | [Event 類型](#event-types) |
| reason | string | 詳細原因，依 event 列於下方 |
| error? | string | 底層錯誤，有的話才出現 |

###### Log Entry 範例 {#example-log-entry}

```json
{"level":"WARN","msg":"request rejected","clientIp":"203.0.113.7","clientIpSource":"header","method":"GET","path":"/verify","status":401,"event":"rejected","reason":"invalid_signature","error":"..."}
```

## Event {#events}

###### Event 類型 {#event-types}

| Event | Level | 說明 |
|---|---|---|
| rejected | WARN | Response 帶有 [`X-Auth-Error`](./status-codes.md#auth-error-header) |
| unauthenticated | INFO | `401` 且沒有 `X-Auth-Error` |
| invalid_request | WARN | `400` |
| denied | INFO | 使用者不允許或 provider 拒絕 |
| upstream_error | ERROR | OAuth2 provider 失敗 |
| internal_error | ERROR | 內部失敗 |
| degraded | WARN | 非 request 結果 |

###### Rejected Reason {#rejected-reasons}

| Reason | 說明 |
|---|---|
| device_not_found | Token 對應的 device 已不存在 |
| invalid_signature | 簽章與 device secret 不符 |
| invalid_device_tag | Device tag 不是本 server 簽發 |
| unexpected_algorithm | Token 使用非預期的簽章演算法 |
| invalid_issuer | [完成登入](../resources/session.md#get-sessions-callback) 的 `iss` 與 provider 不符 |
| spoofed_identity_header | Request 帶有與 access token 不符的 identity header |

###### Unauthenticated Reason {#unauthenticated-reasons}

| Reason | 說明 |
|---|---|
| token_missing | 沒有 Bearer token |
| token_invalid | Access token 過期、格式錯誤或已撤銷 |
| refresh_token_missing | Cookie 或 header 中沒有 refresh token |
| refresh_token_invalid | Refresh token 過期或格式錯誤 |
| refresh_token_reused | 送來已 rotate 過的 refresh token，對應 device 已撤銷<br>見 [重複使用偵測](./tokens.md#reuse-detection) |

###### Invalid Request Reason {#invalid-request-reasons}

| Reason | 說明 |
|---|---|
| body_invalid | [開始登入](../resources/session.md#post-sessions) 的 body 不是合法 JSON |
| redirect_uri_missing | 沒有 `redirect_uri` 且未設定預設值 |
| redirect_uri_not_allowed | `redirect_uri` 不在允許清單 |
| state_missing | [完成登入](../resources/session.md#get-sessions-callback) 缺少 `state` |
| state_invalid | `state` 不存在、已過期或已使用 |
| code_missing | [完成登入](../resources/session.md#get-sessions-callback) 缺少 `code` |
| pkce_invalid | `X-PKCE-Verifier` 不符 |

###### Denied Reason {#denied-reasons}

| Reason | 說明 |
|---|---|
| user_not_found | 使用者不存在且未開放註冊，或已被刪除 |
| provider_access_denied | 使用者在 provider 拒絕授權 |
| provider_invalid_request | Provider 拒絕授權請求 |
| provider_unauthorized_client | Client 不允許使用此流程 |
| provider_unsupported_response_type | Provider 不支援該 response type |
| provider_invalid_scope | Provider 拒絕請求的 scope |
| provider_error | 無法辨識的 provider `error` |

###### Upstream Error Reason {#upstream-error-reasons}

| Reason | 說明 |
|---|---|
| provider_timeout | Code exchange 逾時 |
| code_exchange_failed | Code exchange 失敗 |
| userinfo_failed | 取得 user info 失敗 |
| provider_server_error | Provider 回傳 `server_error` |
| provider_temporarily_unavailable | Provider 回傳 `temporarily_unavailable` |

###### Internal Error Reason {#internal-error-reasons}

| Reason | 說明 |
|---|---|
| session_init_failed | 儲存登入 `state` 失敗 |
| state_lookup_failed | 讀取登入 `state` 失敗 |
| user_create_failed | 註冊使用者失敗 |
| user_lookup_failed | 查詢 username 失敗 |
| device_register_failed | 儲存 device session 失敗 |
| device_delete_failed | 登出或 refresh token reuse 時刪除 device 失敗 |
| token_issue_failed | 產生 refresh token 或 access token 失敗 |
| refresh_rotate_failed | Rotate refresh token 失敗 |
| session_list_failed | 列出 session 失敗 |
| session_revoke_failed | 刪除 device session 失敗 |
| account_delete_failed | 刪除使用者失敗 |
| identity_sign_failed | 簽署 identity token 失敗 |
| database_cleanup_failed | 定期 database cleanup 失敗 |
| unexpected_result | 登入 exchange 沒有產生可用結果 |

###### Degraded Reason {#degraded-reasons}

| Reason | 說明 |
|---|---|
| flight_redis_unavailable | Redis 讀寫共用結果失敗，改在本機執行 |
| flight_lock_unavailable | 無法取得 distributed lock，改在本機執行 |
| flight_result_corrupt | 共用結果無法 encode 或 decode |
| cache_redis_failed | Redis 中 device secret cache 讀寫失敗 |
| oidc_discovery_retry | 啟動時取得 OIDC discovery document 失敗，重試中 |

## Client IP {#client-ip}

有設定 `TRUSTED_CLIENT_IP_HEADER` 時取其最右側的值，否則取 remote address

###### Client IP 來源 {#client-ip-sources}

| 值 | 說明 |
|---|---|
| remote_addr | 未設定 `TRUSTED_CLIENT_IP_HEADER`，取自 remote address |
| header | 取自 `TRUSTED_CLIENT_IP_HEADER` |
| missing | 沒有該 header，`clientIp` 為空 |
| invalid | 值不是合法 IP，`clientIp` 為空 |
