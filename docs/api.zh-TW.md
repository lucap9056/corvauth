# API

- JSON 回應格式：`{ "success": bool, "message": ... }`
- Token 缺少或無效：`401` + `WWW-Authenticate: Bearer`

| Endpoint | 提供於 | 輸入 | 成功時 |
|---|---|---|---|
| `GET /health` | 所有模式 | — | `200` |
| `GET /login` | 所有模式 | — | `200`，`message`：`{ "url", "verifier"? }` |
| `GET /callback` | 所有模式 | `code`、`state` query | `200`，`message`：`{ "access_token", "refresh_token" }`（見下方） |
| `POST /refresh` | session 模式 | refresh token | `200`，`message`：新 token pair，設定 cookie |
| `POST /refresh-access` | session 模式 | refresh token | `200`，`message`：新 access token |
| `POST /refresh-status` | session 模式 | refresh token | `200`，`message`：`{ "device_id", "issued_at", "expires_at" }`，不 rotate |
| `GET /verify` | session 模式 | Bearer access token | `204` + 使用者 header（見 [Identity Token](#identity-token)） |
| `POST /logout` | session 模式 | refresh token（選填） | `200`，刪除 device session 並清除 cookie |
| `DELETE /users/me` | managed users | Bearer access token | `200`，刪除所有 session 與該使用者 |

`/callback` 回傳的 token：

- Session 模式：session token，並設定 cookie
- Stateless proxy：provider 的 token

`/verify` 供 reverse proxy 的 auth request 使用（如 nginx `auth_request`、Traefik `forwardAuth`），把回傳的 header 轉發給 upstream

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
