# 設定

Corvauth 完全以環境變數設定，值無效時 server 會在啟動時停止。

標示 `?` 的變數為選填。

`duration` 的格式見 [Duration](../reference.md#durations)。

## Server {#server}

###### Server 變數 {#server-variables}

| 變數 | 型別 | 預設值 | 說明 |
|---|---|---|---|
| HTTP_ADDRESS? | string | `:80` | 監聽位址<br>`unix:///path/to.sock` 改監聽 Unix socket |
| HTTP_MODE? | string | `production` | `development` 會移除 `refresh_token` cookie 的 `Secure`，供本機 HTTP 測試 |
| REDIS_URL? | string | — | 存放 `state`、快取 device secret，並跨實例去重 [完成登入](../resources/session.md#get-callback) 與 refresh token rotation<br>未設定時存在記憶體，僅限單一實例 |
| TRUSTED_CLIENT_IP_HEADER? | string | — | 受信任 proxy 設定的 client IP header，例如 `X-Real-IP`<br>未設定時使用 remote address<br>見 [Client IP](./logging.md#client-ip) |

## Logging {#logging}

###### Logging 變數 {#logging-variables}

| 變數 | 型別 | 預設值 | 說明 |
|---|---|---|---|
| LOG_LEVEL? | string | `info` | `debug`、`info`、`warn` 或 `error` |
| LOG_FORMAT? | string | `console` | `console` 或 `json` |

## OAuth2 {#oauth2}

需同時設定 `OAUTH2_CLIENT_ID`、`OAUTH2_CLIENT_SECRET`，以及 `OAUTH2_REDIRECT_URL` 或 `OAUTH2_ALLOWED_REDIRECT_URLS` 才會啟用 OAuth2。

Provider 的選擇方式見 [Provider](./oauth2.md#providers)。

###### OAuth2 變數 {#oauth2-variables}

| 變數 | 型別 | 預設值 | 說明 |
|---|---|---|---|
| OAUTH2_CLIENT_ID | string | — | Client ID |
| OAUTH2_CLIENT_SECRET | string | — | Client secret |
| OAUTH2_REDIRECT_URL? | string | — | [開始登入](../resources/session.md#post-sessions) 未帶 `redirect_uri` 時的預設 redirect URL |
| OAUTH2_ALLOWED_REDIRECT_URLS? | 逗號分隔的 string | — | [開始登入](../resources/session.md#post-sessions) 可透過 `redirect_uri` 指定的 redirect URL，需完全相符<br>每一項也必須在 provider 註冊 |
| OAUTH2_PROVIDER? | string | — | `discord`、`github`、`google`，或其他任意名稱（generic provider） |
| OAUTH2_SCOPES? | 逗號分隔的 string | — | 請求的 scope |
| OAUTH2_AUTH_URL? | string | — | Authorization endpoint<br>generic provider 必填 |
| OAUTH2_TOKEN_URL? | string | — | Token endpoint<br>generic provider 必填 |
| OAUTH2_USERINFO_URL? | string | — | User info endpoint<br>generic provider 必填 |
| OAUTH2_REVOKE_URL? | string | — | Revocation endpoint，用於撤銷 provider token |
| OIDC_ISSUER_URL? | string | — | 從 `{issuer}/.well-known/openid-configuration` 讀取 endpoint<br>會覆蓋 `OAUTH2_PROVIDER` |
| OAUTH2_CLIENT_PKCE? | boolean | `false` | 讓 client 持有 PKCE verifier<br>見 [Client PKCE](./oauth2.md#client-pkce) |

## Database {#database}

###### Database 變數 {#database-variables}

| 變數 | 型別 | 預設值 | 說明 |
|---|---|---|---|
| DATABASE_URL? | string | — | PostgreSQL 連線字串<br>未設定時為 [stateless proxy](./deployment-modes.md#stateless-proxy) |
| DB_USER_EMAIL_REFERENCE? | string | — | 你的 users table，`[<schema>.]<table>(<column>)[:<type>]`<br>見 [User Email Reference](../../../database/README.md#user-email-reference)<br>未設定時為 [managed users](./deployment-modes.md#managed-users) |
| DB_USER_USERNAME_COLUMN? | string | — | 作為 `username` claim 的欄位<br>需搭配 `DB_USER_EMAIL_REFERENCE` |
| DB_AUTO_CREATE_SCHEMA? | boolean | `false` | 啟動時建立 table<br>見 [部署模式](./deployment-modes.md)<br>需要 DDL 權限 |
| DB_CLEANUP_INTERVAL? | duration | `24h` | 刪除 7 天未 refresh 的 session 的間隔<br>純整數以小時計 |
| DB_MAX_OPEN_CONNS? | integer | `20` | 最大開啟連線數 |
| DB_MAX_IDLE_CONNS? | integer | `15` | 最大閒置連線數 |
| DB_CONN_MAX_LIFETIME? | duration | `5m` | 連線最長存活時間<br>純整數以分鐘計 |
| DB_CONN_MAX_IDLE_TIME? | duration | `2m` | 連線最長閒置時間<br>純整數以分鐘計 |

## Token {#tokens}

###### Token 變數 {#token-variables}

| 變數 | 型別 | 預設值 | 說明 |
|---|---|---|---|
| JWT_ACCESS_TOKEN_DURATION? | duration | `15m` | [Access token](./tokens.md#access-token) 有效期<br>必須帶單位 |
| JWT_REFRESH_TOKEN_DURATION? | duration | `7d` | [Refresh token](./tokens.md#refresh-token) 與 cookie 有效期<br>必須帶單位 |
| JWT_ISSUER? | string | — | `iss` claim，寫入並驗證 |
| JWT_AUDIENCE? | string | — | `aud` claim，寫入並驗證 |
| IDENTITY_JWT_SECRET? | string | — | 至少 32 bytes<br>[Verify](../resources/verify.md) 改回傳簽章過的 [identity token](../resources/verify.md#identity-token-object)，不再回傳明文 header |
| JWT_DEVICE_TAG_SECRET? | string | — | 至少 32 bytes<br>產生 token `device_tag` 用的 secret |

## Identity Header {#identity-headers}

Verify 回傳的 [使用者 header](../resources/verify.md#user-headers) 名稱。

只有本 server 能產生這些 header，帶有它們的 request 會與 access token 比對。

###### Identity Header 變數 {#identity-header-variables}

| 變數 | 型別 | 預設值 | 說明 |
|---|---|---|---|
| IDENTITY_HEADER? | string | `X-Corvauth-Identity` | Identity token，設定 `IDENTITY_JWT_SECRET` 時使用 |
| USER_EMAIL_HEADER? | string | `X-Corvauth-User-Email` | 使用者 email |
| DEVICE_ID_HEADER? | string | `X-Corvauth-Device-ID` | Device ID |
| USERNAME_HEADER? | string | `X-Corvauth-Username` | Username |

::: info
每個都必須是合法的 header 名稱，且四者互不相同（不分大小寫）
:::

## 登入政策 {#sign-in-policy}

###### 登入政策變數 {#sign-in-policy-variables}

| 變數 | 型別 | 預設值 | 說明 |
|---|---|---|---|
| ALLOW_REGISTRATION? | boolean | `false` | 將首次登入的使用者寫入 `users`<br>僅限 managed users |
| ALLOW_UNVERIFIED_EMAIL? | boolean | `false` | 接受未驗證的 email<br>預設 `email_verified` 必須為 true |
| PASS_OAUTH_TOKEN? | boolean | `false` | 將 provider token 回傳給 client<br>見 [Provider Token](./oauth2.md#provider-tokens) |
| PROVIDER_ACCESS_TOKEN_HEADER? | string | `X-Corvauth-Provider-Access-Token` | `PASS_OAUTH_TOKEN=true` 時攜帶 provider access token 的 header |
| PROVIDER_REFRESH_TOKEN_HEADER? | string | `X-Corvauth-Provider-Refresh-Token` | `PASS_OAUTH_TOKEN=true` 時攜帶 provider refresh token 的 header<br>必須是合法 header 名稱，且與 access token header 不同（不分大小寫） |
