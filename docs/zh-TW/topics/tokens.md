# Token

Session 模式為每個 [device session](../resources/session.md#device-session-object) 簽發兩種 JWT，皆以該 device 自己的 secret 用 `HS256` 簽章。

## Access Token {#access-token}

短效（`JWT_ACCESS_TOKEN_DURATION`，預設 `15m`），以 `Authorization: Bearer <access_token>` 傳送

###### Access Token Claim {#access-token-claims}

| 欄位 | 型別 | 說明 |
|---|---|---|
| user_email | string | 使用者 email |
| device_id | string | Device session 的 ID |
| username | string | 使用者顯示名稱<br>見 [Username 更新](#username-updates) |
| device_tag? | string | 將 token 綁定到其 `device_id` 與 `exp`<br>設定 `JWT_DEVICE_TAG_SECRET` 時才有 |
| generation | integer | Device session 的 rotation 世代 |
| iss? | string | `JWT_ISSUER`，有設定時寫入並驗證 |
| aud? | array of strings | `JWT_AUDIENCE`，有設定時寫入並驗證 |
| exp | integer | 到期時間，Unix 秒數 |

## Refresh Token {#refresh-token}

長效（`JWT_REFRESH_TOKEN_DURATION`，預設 `7d`），讀自 `refresh_token` cookie，沒有 cookie 時改讀 JSON body `{ "refresh_token": "..." }`

###### Refresh Token Cookie 屬性 {#refresh-token-cookie-attributes}

| 屬性 | 值 |
|---|---|
| Name | `refresh_token` |
| Path | `/` |
| HttpOnly | 一律設定 |
| Secure | `HTTP_MODE=development` 以外皆設定 |
| SameSite | `Lax` |
| Expires | 與 token 同時到期 |

::: tip
讀不到 `HttpOnly` cookie 的頁面，可由 [取得目前 Session](../resources/session.md#get-sessions-current) 取得到期時間
:::

## Rotation {#rotation}

每次呼叫 [替換 Refresh Token](../resources/session.md#post-sessions-current-tokens) 都會簽發新的 refresh token 並讓舊的失效，使用中的 session 因此持續延長

- 同一 token 並行的 rotation 共用同一次 rotation（有 Redis 時跨實例也成立）
- [建立 Access Token](../resources/session.md#post-sessions-current-access-token) 只簽發 access token，不 rotate

## 重複使用偵測 {#reuse-detection}

送出已 rotate 過的 refresh token 視為外洩：刪除該 device session，request 以 `401` 失敗（[Logging](./logging.md#unauthenticated-reasons) 中的 `refresh_token_reused`）

## Username 更新 {#username-updates}

登入與每次 refresh 都會重讀 `username`，改名最晚在一個 access token 有效期後生效

## Device 撤銷 {#device-revocation}

刪除 device session（[刪除目前 Session](../resources/session.md#delete-sessions-current)、[刪除 Device Session](../resources/session.md#delete-sessions) 或重複使用偵測）會移除其 secret，該 device 的 access token 立即無法通過 [驗證 Access Token](../resources/verify.md#get-verify)，並帶 `X-Auth-Error: device_not_found`
