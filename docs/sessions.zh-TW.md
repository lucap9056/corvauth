# Session

## 登入流程

1. `GET /login` 回傳 `{ "url": "..." }`，將使用者導向 `url`
   - Server 產生 PKCE verifier，以一次性 `state` 為 key 儲存（有 `REDIS_URL` 存 Redis，否則存記憶體）
2. Provider 導回 `GET /callback?code=...&state=...`
   - Server 用 `state` 取出 verifier 後立即刪除，防止重放，再交換 code
3. Session 模式下，server 取得 email、建立 device session、設定 `refresh_token` cookie，回傳兩個 token

`/callback` 可選的 request header：

| Header | 用途 |
|---|---|
| `X-Device-Name` | Device session 名稱（預設 `Unknown Device`） |
| `X-PKCE-Verifier` | `OAUTH2_CLIENT_PKCE=true` 時必填 |

`OAUTH2_CLIENT_PKCE=true` 適用於 client 自行處理 PKCE：`/login` 多回傳 `verifier`，client 透過 `X-PKCE-Verifier` 帶回

## Token

- **Access token**：短效（`JWT_ACCESS_TOKEN_DURATION`），以 `Authorization: Bearer <token>` 傳送
  - Claim 含 `user_email`、`device_id`、`username`
- **Refresh token**：讀自 `refresh_token` cookie 或 JSON body `{ "refresh_token": "..." }`
  - Cookie 為 `HttpOnly`、`SameSite=Lax`，與 token 同時到期
  - 讀不到 cookie 的頁面可由 `/refresh-status` 取得到期時間
- **Rotation**：每次 `/refresh` 簽發新 token、舊的失效，使用中的 session 持續延長
  - 同一 token 並行 `/refresh` 共用同一次 rotation（有 Redis 時跨實例也成立）
- **重複使用偵測**：送出已 rotate 的舊 token 視為外洩，刪除該 device session
- **Username 更新**：登入與每次 refresh 都重讀 `username`，最晚一個 access token 有效期後生效

## Provider Token

`PASS_OAUTH_TOKEN` 決定 session 模式下 provider token 的處理：

- `false`（預設）：登入後立即 revoke 並捨棄
- `true`：以 `/callback` 的 `X-Forwarded-Access-Token`、`X-Forwarded-Refresh-Token` header 回傳，由 client 保管

Stateless proxy 一定回傳 provider token：

- 預設放在 response body
- `PASS_OAUTH_TOKEN=true` 時只放在上述 header
