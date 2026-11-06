# 設定

時間格式：一或多段 `<整數><單位>`，單位為 `d`、`h`、`m`、`s`（如 `30m`、`7d`、`1d12h`）

格式錯誤時 server 啟動失敗

## Server

| 變數 | 預設值 | 說明 |
|---|---|---|
| `HTTP_ADDRESS` | `:80` | 監聽位址，`unix:///path/to.sock` 改聽 Unix socket |
| `HTTP_MODE` | `production` | `development` 時 `refresh_token` cookie 不標 `Secure`，供本機 HTTP 測試 |
| `REDIS_URL` | — | 存 `state`、快取 device secret、跨實例去重 `/callback` 與 refresh token rotation，未設則用記憶體，僅限單一實例 |
| `TRUSTED_CLIENT_IP_HEADER` | — | 信任的 proxy 寫入 client IP 的 header（如 `X-Real-IP`），未設則用 remote address，見 [Logging](logging.zh-TW.md#client-ip) |

## Logging

| 變數 | 預設值 | 說明 |
|---|---|---|
| `LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` |
| `LOG_FORMAT` | `console` | `console` 或 `json` |

## OAuth2.0 Provider

- 內建：`discord`、`github`、`google`，其他值使用 generic provider
- 設定 `OIDC_ISSUER_URL` 改用 OIDC discovery，優先於 `OAUTH2_PROVIDER`

| 變數 | 說明 |
|---|---|
| `OAUTH2_CLIENT_ID`、`OAUTH2_CLIENT_SECRET` | Client credentials，兩者都設且設定 `OAUTH2_REDIRECT_URL` 或 `OAUTH2_ALLOWED_REDIRECT_URLS` 才啟用 OAuth2.0 |
| `OAUTH2_REDIRECT_URL` | `POST /sessions` 未帶 `redirect_uri` 時的預設 redirect URL |
| `OAUTH2_ALLOWED_REDIRECT_URLS` | 逗號分隔，`POST /sessions` 可透過 `redirect_uri` 指定的 redirect URL，須完全相同；每個也都要在 provider 註冊 |
| `OAUTH2_PROVIDER` | `discord`、`github`、`google`，或其他名稱（generic） |
| `OAUTH2_SCOPES` | 逗號分隔的 scope |
| `OAUTH2_AUTH_URL`、`OAUTH2_TOKEN_URL`、`OAUTH2_USERINFO_URL` | Generic provider 必填 |
| `OAUTH2_REVOKE_URL` | 選填，用於 revoke provider token |
| `OIDC_ISSUER_URL` | 從 `{issuer}/.well-known/openid-configuration` 讀取 endpoint |
| `OAUTH2_CLIENT_PKCE` | `true` 時由 client 持有 PKCE verifier（見[登入流程](sessions.zh-TW.md#登入流程)） |

## Database

| 變數 | 預設值 | 說明 |
|---|---|---|
| `DATABASE_URL` | — | PostgreSQL 連線字串，未設為 stateless proxy 模式 |
| `DB_USER_EMAIL_REFERENCE` | — | 你的 users table，格式 `[<schema>.]<table>(<column>)[:<type>]`（見 [User Email Reference](../database/README.zh-TW.md#user-email-reference)），未設為 managed users 模式 |
| `DB_USER_USERNAME_COLUMN` | — | 作為 `username` claim 的 column，需搭配 `DB_USER_EMAIL_REFERENCE` |
| `DB_AUTO_CREATE_SCHEMA` | `false` | 啟動時建立 table（見[部署模式](../README.zh-TW.md#部署模式)），需 DDL 權限 |
| `DB_CLEANUP_INTERVAL` | `24h` | 刪除 7 天未 refresh 之 session 的頻率，純整數視為小時 |
| `DB_MAX_OPEN_CONNS` | `20` | 最大連線數 |
| `DB_MAX_IDLE_CONNS` | `15` | 最大閒置連線數 |
| `DB_CONN_MAX_LIFETIME` | `5m` | 連線最長存活時間，純整數視為分鐘 |
| `DB_CONN_MAX_IDLE_TIME` | `2m` | 連線最長閒置時間，純整數視為分鐘 |

## Token

| 變數 | 預設值 | 說明 |
|---|---|---|
| `JWT_ACCESS_TOKEN_DURATION` | `15m` | Access token 有效期，須帶單位 |
| `JWT_REFRESH_TOKEN_DURATION` | `7d` | Refresh token 與 cookie 有效期，須帶單位 |
| `JWT_ISSUER` | — | 寫入並驗證 `iss` claim |
| `JWT_AUDIENCE` | — | 寫入並驗證 `aud` claim |
| `IDENTITY_JWT_SECRET` | — | 至少 32 bytes，`/verify` 改回傳簽章過的 identity token（見 [Identity Token](api.zh-TW.md#identity-token)） |
| `JWT_DEVICE_TAG_SECRET` | — | 至少 32 bytes，用於生成 token 的 `device_tag` |

## Identity Header

`/verify` 回傳的使用者 header 名稱；請求帶有這些 header 時會與 access token 比對（見 [`GET /verify`](api.zh-TW.md#get-verify)），因此只能由本 server 產生

| 變數 | 預設值 | 說明 |
|---|---|---|
| `IDENTITY_HEADER` | `X-Corvauth-Identity` | Identity token，設定 `IDENTITY_JWT_SECRET` 時使用 |
| `USER_EMAIL_HEADER` | `X-Corvauth-User-Email` | 使用者 email |
| `DEVICE_ID_HEADER` | `X-Corvauth-Device-ID` | Device ID |
| `USERNAME_HEADER` | `X-Corvauth-Username` | Username |

- 須為合法的 header 名稱，且四者不可重複（不分大小寫）

## 登入政策

| 變數 | 預設值 | 說明 |
|---|---|---|
| `ALLOW_REGISTRATION` | `false` | 首次登入的使用者 insert 到 `users`（僅 managed users） |
| `ALLOW_UNVERIFIED_EMAIL` | `false` | 接受未驗證的 email，預設要求 `email_verified` 為 true |
| `PASS_OAUTH_TOKEN` | `false` | 將 provider token 回傳給 client（見 [Provider Token](sessions.zh-TW.md#provider-token)） |
| `PROVIDER_ACCESS_TOKEN_HEADER` | `X-Corvauth-Provider-Access-Token` | `PASS_OAUTH_TOKEN=true` 時放 provider access token 的 header |
| `PROVIDER_REFRESH_TOKEN_HEADER` | `X-Corvauth-Provider-Refresh-Token` | `PASS_OAUTH_TOKEN=true` 時放 provider refresh token 的 header；須為合法 header 名稱，且與 access token header 不同（不分大小寫） |
