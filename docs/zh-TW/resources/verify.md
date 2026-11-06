# Verify API

供 reverse proxy 的 auth request 使用，例如 nginx `auth_request`、Traefik `forwardAuth`。

Proxy 以 client 的 access token 呼叫 verify endpoint，再把回傳的 [使用者 header](#user-headers) 轉發給 upstream。

只在 session 模式提供。

### 使用者 Header {#user-headers}

Header 名稱可自訂。

見 [Identity Header](../topics/configuration.md#identity-headers)。

###### 使用者 Header 結構 {#user-header-structure}

| Header | 未設定 `IDENTITY_JWT_SECRET` | 有設定 `IDENTITY_JWT_SECRET` |
|---|---|---|
| X-Corvauth-User-Email | 使用者 email | — |
| X-Corvauth-Device-ID | Device ID | — |
| X-Corvauth-Username | Username，為空時不帶 | — |
| X-Corvauth-Identity | — | [Identity token](#identity-token-object) |

::: warning
建議設定 `IDENTITY_JWT_SECRET`。

未設定時 header 為明文：
- upstream 必須只能經由 proxy 存取
- proxy 必須清除 client 自帶的同名 header
:::

### Identity Token 物件 {#identity-token-object}

以 `IDENTITY_JWT_SECRET` 用 `HS256` 簽章的 JWT，header 的 `typ` 為 `identity+jwt`。

Upstream 以同一個 secret 驗證。

###### Identity Token Claim {#identity-token-claims}

| 欄位 | 型別 | 說明 |
|---|---|---|
| sub | string | 使用者 email |
| username? | string | Username，為空時省略 |
| device_id | string | Device session 的 ID |
| iss? | string | `JWT_ISSUER`，有設定時才出現 |
| aud? | array of strings | `JWT_AUDIENCE`，有設定時才出現 |
| iat | integer | 簽發時間，Unix 秒數 |
| exp | integer | `iat` 後 1 分鐘，access token 更早過期則以其為準 |

###### Identity Token Claim 範例 {#example-identity-token-claims}

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

## 驗證 Access Token % GET /verify

Access token 有效時以 `204` 回傳 [使用者 header](#user-headers)，沒有 body。

Request 帶有的使用者 header 須與 access token 一致才會接受，否則回 `403`。

例如外層已用同一個 token 呼叫過此 endpoint 並轉發其 header。

::: danger
沒有 `Authorization: Bearer` 時，request 帶有任一使用者 header（即使值為空）都會最先以 `403` 拒絕，並帶 `X-Auth-Error: spoofed_identity_header`
:::

## 選擇性驗證 Access Token % GET /verify-optional

供同時允許匿名存取的路由使用，只檢查帶有 `Authorization: Bearer` header 的 request。

- 沒有 `Authorization: Bearer`（包括 `Basic` 等其他 scheme）：`204`，不帶使用者 header（匿名）
- 有 `Authorization: Bearer`：與 [驗證 Access Token](#get-verify) 相同檢查
  - Token 無效或過期同樣回 `401`，client 仍會去 refresh
- Request 帶有的使用者 header 與 [驗證 Access Token](#get-verify) 相同方式檢查

::: warning
Upstream 以「沒有使用者 header」判定匿名，proxy 必須覆寫或清除 client 自帶的同名 header
:::
