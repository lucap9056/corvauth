# OAuth2

Corvauth 透過 OAuth2 或 OIDC provider 讓使用者登入，並將結果轉成 [device session](../resources/session.md#device-session-object)。

## 登入流程 {#login-flow}

1. Client 呼叫 [開始登入](../resources/session.md#post-sessions)，將使用者導向回傳的 `url`
   - JSON body 的 `redirect_uri` 從 `OAUTH2_ALLOWED_REDIRECT_URLS` 選擇 redirect URL
   - 未帶時使用 `OAUTH2_REDIRECT_URL`
   - Server 產生 PKCE verifier，與 redirect URL 一起存在一次性的 `state` 底下
   - 有設 `REDIS_URL` 時 `state` 存在 Redis，否則存在記憶體
2. Provider 導回 redirect URL，該頁面將 `code` 與 `state` 交給 [完成登入](../resources/session.md#get-callback)
   - Server 以 `state` 取出 verifier 與 redirect URL 後立即刪除，防止 replay
   - Server 進行 code exchange
3. Session 模式下，server 讀取 email、建立 device session、設定 `refresh_token` cookie，並回傳 [Token Pair](../resources/session.md#token-pair-object)

::: info
預設 email 必須經 provider 驗證（`email_verified`）

`ALLOW_UNVERIFIED_EMAIL=true` 可解除此限制
:::

## Client PKCE {#client-pkce}

`OAUTH2_CLIENT_PKCE=true` 供自行處理 PKCE 的 client 使用：

1. [開始登入](../resources/session.md#post-sessions) 額外回傳 `verifier`
2. Client 以 `X-PKCE-Verifier` 將它送回 [完成登入](../resources/session.md#get-callback)
3. Verifier 缺少或不符時回 `400`

## Provider {#providers}

`OAUTH2_PROVIDER` 選擇內建 provider，其他值使用 generic provider。

###### Provider 類型 {#provider-types}

| Provider | `OAUTH2_PROVIDER` | 必要變數 |
|---|---|---|
| Discord | `discord` | client credential |
| GitHub | `github` | client credential |
| Google | `google` | client credential |
| Generic | 其他任意名稱 | client credential、`OAUTH2_AUTH_URL`、`OAUTH2_TOKEN_URL`、`OAUTH2_USERINFO_URL` |
| OIDC | 忽略 | client credential、`OIDC_ISSUER_URL` |

`OIDC_ISSUER_URL` 會切換為 OIDC discovery，從 `{issuer}/.well-known/openid-configuration` 讀取 endpoint，並覆蓋 `OAUTH2_PROVIDER`。

所有變數見 [OAuth2 變數](./configuration.md#oauth2)。

## Provider Token {#provider-tokens}

`PASS_OAUTH_TOKEN` 決定 provider 本身的 access token 與 refresh token 如何處理：

###### Provider Token 處理方式 {#provider-token-handling}

| 模式 | `PASS_OAUTH_TOKEN=false`（預設） | `PASS_OAUTH_TOKEN=true` |
|---|---|---|
| Session 模式 | 登入後立即撤銷並丟棄 | 放在 [完成登入](../resources/session.md#get-callback) 的 header 回傳，由 client 保存 |
| Stateless proxy | 放在 response body 回傳 | 只放在 header 回傳，`message` 為 `Logged in` |

###### Provider Token Header {#provider-token-headers}

| Header | 名稱設定 |
|---|---|
| `X-Corvauth-Provider-Access-Token` | `PROVIDER_ACCESS_TOKEN_HEADER` |
| `X-Corvauth-Provider-Refresh-Token` | `PROVIDER_REFRESH_TOKEN_HEADER` |
