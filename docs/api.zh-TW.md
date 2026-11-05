# API

- JSON 回應格式：`{ "success": bool, "message": ... }`
- 錯誤時 `success` 為 `false`，`message` 為字串
- Token 缺少或無效：`401` + `WWW-Authenticate: Bearer`
- Refresh token 輸入：`refresh_token` cookie，沒有 cookie 時改讀 JSON body `{ "refresh_token": "..." }`（`GET /sessions/current` 改讀 `Authorization: Bearer`，不讀 body）

| Endpoint | 提供於 | 輸入 | 成功時 |
|---|---|---|---|
| [`GET /health`](#get-health) | 所有模式 | — | `200` |
| [`POST /sessions`](#post-sessions) | 所有模式 | `redirect_uri` body（選填） | `200` |
| [`GET /callback`](#get-callback) | 所有模式 | `code`、`state` query | `200` |
| [`POST /sessions/current/tokens`](#post-sessionscurrenttokens) | session 模式 | `refresh_token` cookie 或 body | `200`，設定 cookie |
| [`POST /sessions/current/access-token`](#post-sessionscurrentaccess-token) | session 模式 | `refresh_token` cookie 或 body | `200` |
| [`GET /sessions/current`](#get-sessionscurrent) | session 模式 | `refresh_token` cookie 或 `Authorization: Bearer` refresh token | `200`，不 rotate |
| [`GET /verify`](#get-verify) | session 模式 | `Authorization: Bearer` access token | `204` + 使用者 header |
| [`GET /verify-optional`](#get-verify-optional) | session 模式 | `Authorization: Bearer` access token，有帶才檢查 | `204`，登入時才帶使用者 header |
| [`DELETE /sessions/current`](#delete-sessionscurrent) | session 模式 | `refresh_token` cookie 或 body（選填） | `200`，刪除 device session 並清除 cookie |
| [`GET /sessions`](#get-sessions) | session 模式 | `Authorization: Bearer` access token | `200`，列出該使用者的 device session |
| [`DELETE /sessions`](#delete-sessions) | session 模式 | `Authorization: Bearer` access token | `200`，刪除該使用者所有 device session 並清除 cookie |
| [`DELETE /users/me`](#delete-usersme) | managed users | `Authorization: Bearer` access token | `200`，刪除所有 session 與該使用者，並清除 cookie |

## `GET /health`

純文字：

```text
OK
```

## `POST /sessions`

發起登入，device session 由 `/callback` 建立

Request（選填）：

```json
{
  "redirect_uri": "https://app.example.com/callback"
}
```

- `redirect_uri`：必須與 `OAUTH2_REDIRECT_URL` 或 `OAUTH2_ALLOWED_REDIRECT_URLS` 其中一項完全相同，否則 `400`；未帶時使用 `OAUTH2_REDIRECT_URL`，未設定則 `400`

```json
{
  "success": true,
  "message": {
    "url": "https://provider.example.com/authorize?...",
    "verifier": "..."
  }
}
```

- `verifier`：只在 `OAUTH2_CLIENT_PKCE=true` 時出現

## `GET /callback`

選填的 request header 見[登入流程](sessions.zh-TW.md#登入流程)

Session 模式回傳 session token 並設定 `refresh_token` cookie；stateless proxy 回傳 provider 的 token：

```json
{
  "success": true,
  "message": {
    "access_token": "...",
    "refresh_token": "..."
  }
}
```

Stateless proxy 在 `PASS_OAUTH_TOKEN=true` 時改把 token 放在 header（見 [Provider Token](sessions.zh-TW.md#provider-token)）：

```json
{
  "success": true,
  "message": "Logged in"
}
```

## `POST /sessions/current/tokens`

Rotate refresh token

Request：`refresh_token` cookie，或 body：

```json
{
  "refresh_token": "..."
}
```

Response：

```json
{
  "success": true,
  "message": {
    "access_token": "...",
    "refresh_token": "..."
  }
}
```

## `POST /sessions/current/access-token`

簽發新的 access token，不 rotate refresh token

Request：`refresh_token` cookie，或 body：

```json
{
  "refresh_token": "..."
}
```

Response，`message` 為新的 access token：

```json
{
  "success": true,
  "message": "..."
}
```

## `GET /sessions/current`

Request：`refresh_token` cookie，沒有 cookie 時改讀 `Authorization: Bearer <refresh_token>`；不讀 body

Response：

```json
{
  "success": true,
  "message": {
    "device_id": "3f1c2a9e-...",
    "issued_at": 1767225600,
    "expires_at": 1769817600
  }
}
```

- `issued_at`、`expires_at`：refresh token 的 Unix 秒數

## `GET /verify`

無 body，回傳使用者 header（見 [Identity Token](#identity-token)）

供 reverse proxy 的 auth request 使用（如 nginx `auth_request`、Traefik `forwardAuth`），把回傳的 header 轉發給 upstream

## `GET /verify-optional`

供同時允許匿名存取的路由使用，只對帶有 `Authorization: Bearer` header 的請求檢查有效性

- 沒有 `Authorization: Bearer`（包括 `Basic` 等其他 scheme）：`204`，不帶使用者 header（匿名）
- 有 `Authorization: Bearer`：與 `/verify` 相同檢查，token 無效或過期同樣回 `401`，client 仍會去 refresh
- upstream 以「沒有使用者 header」判定匿名，proxy 必須覆寫或清除 client 自帶的同名 header

## `DELETE /sessions/current`

Request（選填）：`refresh_token` cookie，或 body：

```json
{
  "refresh_token": "..."
}
```

Response：

```json
{
  "success": true,
  "message": "Logged out and device session revoked"
}
```

## `GET /sessions`

列出該使用者所有 device session，最近 refresh 的在前

```json
{
  "success": true,
  "message": [
    {
      "device_id": "3f1c2a9e-...",
      "device_name": "Unknown Device",
      "updated_at": 1767225600,
      "current": true
    }
  ]
}
```

- `device_name`：送到 `/callback` 的 `X-Device-Name`
- `updated_at`：最後一次登入或 refresh 的 Unix 秒數
- `current`：此 access token 所屬的 session

## `DELETE /sessions`

撤銷該使用者所有 device session（包括目前這個），其 access token 立即無法通過 `/verify`

```json
{
  "success": true,
  "message": "Logged out from all sessions"
}
```

## `DELETE /users/me`

```json
{
  "success": true,
  "message": "Account deleted"
}
```

## Identity Token

建議設定 `IDENTITY_JWT_SECRET`

- 有設定：`/verify` 回傳簽章過的 header，upstream 用同一個 secret 驗證
- 未設定：header 為明文
  - upstream 必須只能經由 proxy 存取
  - proxy 必須清除 client 自帶的同名 header

| Header | 未設定 | 有設定 |
|---|---|---|
| `X-Forwarded-User-Email` | 使用者 email | — |
| `X-Forwarded-Device-ID` | Device ID | — |
| `X-Forwarded-Username` | Username，為空時不帶 | — |
| `X-Forwarded-Identity` | — | Identity token |

Identity token 以 `HS256` 簽章，header `typ` 為 `identity+jwt`：

```json
{
  "sub": "user@example.com",
  "username": "User",
  "device_id": "3f1c2a9e-...",
  "iss": "https://auth.example.com",
  "aud": ["api"],
  "iat": 1767225600,
  "exp": 1767225660
}
```

- `username`：為空時省略
- `iss`、`aud`：只在設定 `JWT_ISSUER`、`JWT_AUDIENCE` 時出現
- `exp`：`iat` 後 1 分鐘，access token 更早過期則以其為準

## Error Header

`401` 由 device session 造成時，`/sessions/current/tokens`、`/sessions/current/access-token`、`GET /sessions/current`、`GET /sessions`、`DELETE /sessions`、`/verify`、`/verify-optional`、`DELETE /users/me` 會帶 `X-Auth-Error`，讓 gateway 與過期或格式錯誤的 token 區分：

| 值 | 原因 |
|---|---|
| `device_not_found` | Device session 已不存在 |
| `invalid_signature` | Signature 與 device secret 不符 |
| `invalid_device_tag` | Token 的 `device_tag` 與其 `device_id`、`exp` 不符（僅在設定 `JWT_DEVICE_TAG_SECRET` 時） |
| `unexpected_algorithm` | Token 未以 `HS256` 簽章 |
