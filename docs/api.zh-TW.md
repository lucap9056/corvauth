# API

- JSON 回應格式：`{ "success": bool, "message": ... }`
- 錯誤時 `success` 為 `false`，`message` 為字串
- Token 缺少或無效：`401` + `WWW-Authenticate: Bearer`
- Refresh token 輸入：`refresh_token` cookie，沒有 cookie 時改讀 JSON body `{ "refresh_token": "..." }`

| Endpoint | 提供於 | 輸入 | 成功時 |
|---|---|---|---|
| [`GET /health`](#get-health) | 所有模式 | — | `200` |
| [`GET /login`](#get-login) | 所有模式 | — | `200` |
| [`GET /callback`](#get-callback) | 所有模式 | `code`、`state` query | `200` |
| [`POST /refresh`](#post-refresh) | session 模式 | `refresh_token` cookie 或 body | `200`，設定 cookie |
| [`POST /refresh-access`](#post-refresh-access) | session 模式 | `refresh_token` cookie 或 body | `200` |
| [`POST /refresh-status`](#post-refresh-status) | session 模式 | `refresh_token` cookie 或 body | `200`，不 rotate |
| [`GET /verify`](#get-verify) | session 模式 | `Authorization: Bearer` access token | `204` + 使用者 header |
| [`POST /logout`](#post-logout) | session 模式 | `refresh_token` cookie 或 body（選填） | `200`，刪除 device session 並清除 cookie |
| [`DELETE /users/me`](#delete-usersme) | managed users | `Authorization: Bearer` access token | `200`，刪除所有 session 與該使用者，並清除 cookie |

## `GET /health`

純文字：

```text
OK
```

## `GET /login`

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

## `POST /refresh`

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

## `POST /refresh-access`

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

## `POST /refresh-status`

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

## `POST /logout`

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

`401` 由 device session 造成時，`/refresh`、`/refresh-access`、`/refresh-status`、`/verify`、`DELETE /users/me` 會帶 `X-Auth-Error`，讓 gateway 與過期或格式錯誤的 token 區分：

| 值 | 原因 |
|---|---|
| `device_not_found` | Device session 已不存在 |
| `invalid_signature` | Signature 與 device secret 不符 |
