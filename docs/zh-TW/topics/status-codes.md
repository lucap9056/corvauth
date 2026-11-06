# Status Code

## HTTP Response Code {#http-response-codes}

| Code | 意義 |
|---|---|
| 200 (OK) | Request 成功 |
| 204 (No Content) | Access token 有效，只回傳使用者 header<br>見 [Verify](../resources/verify.md) |
| 400 (Bad Request) | Request 格式錯誤，或 provider 拒絕授權請求 |
| 401 (Unauthorized) | Token 缺少、無效或過期<br>在 [完成登入](../resources/session.md#get-sessions-callback) 表示該使用者不允許登入 |
| 403 (Forbidden) | Request 帶的 identity header 與 access token 不符，或使用者在 provider 拒絕授權 |
| 500 (Internal Server Error) | 內部錯誤，或向 provider 進行 code exchange、取得 user info 失敗 |
| 502 (Bad Gateway) | Provider 回傳 `server_error` |
| 503 (Service Unavailable) | Provider 回傳 `temporarily_unavailable` |
| 504 (Gateway Timeout) | 與 provider 的 code exchange 逾時 |

因 token 缺少或無效而回的 `401` 會帶 `WWW-Authenticate: Bearer`（例如 `Bearer error="invalid_token"`），讓 client 知道要 refresh。

每次失敗的 `reason` 寫在 [log](./logging.md#events)，不在 response 中。

## Auth Error Header {#auth-error-header}

當原因不是單純的 token 過期或格式錯誤時，`X-Auth-Error` 讓 gateway 知道 request 被拒絕的理由。

它用來標記高機率來自非自然操作所產生的 request，例如偽造或竄改的 token、偽造的 identity header，而非一般 client 的正常行為。

###### Device Session 錯誤 {#device-session-errors}

由 [替換 Refresh Token](../resources/session.md#post-sessions-current-tokens)、[建立 Access Token](../resources/session.md#post-sessions-current-access-token)、[取得目前 Session](../resources/session.md#get-sessions-current)、[取得 Device Session](../resources/session.md#get-sessions)、[刪除 Device Session](../resources/session.md#delete-sessions)、[刪除目前使用者](../resources/user.md#delete-users-me) 與兩個 [Verify](../resources/verify.md) endpoint 加在 `401` 上

| 值 | 原因 |
|---|---|
| device_not_found | Device session 已不存在 |
| invalid_signature | Signature 與 device secret 不符 |
| invalid_device_tag | Token 的 `device_tag` 與其 `device_id`、`exp` 不符<br>僅在設定 `JWT_DEVICE_TAG_SECRET` 時 |
| unexpected_algorithm | Token 未以 `HS256` 簽章 |

###### Issuer 錯誤 {#issuer-errors}

由 [完成登入](../resources/session.md#get-sessions-callback) 加在 `400` 上

| 值 | 原因 |
|---|---|
| invalid_issuer | Authorization response 的 `iss` 與 provider 不符，或 provider 宣告支援卻缺少 `iss` |

###### Identity Header 錯誤 {#identity-header-errors}

由兩個 [Verify](../resources/verify.md) endpoint 加在 `403` 上

| 值 | 原因 |
|---|---|
| spoofed_identity_header | 沒有 access token 卻帶有使用者 header，或與 access token 不一致 |
