# Session API

每次登入建立一個 device session，有自己的 signing secret。

登出或刪除 device 只影響該 device 的 token。

登入流程見 [OAuth2](../topics/oauth2.md)，token 有效期見 [Token](../topics/tokens.md)。

::: info
除了 [開始登入](#post-sessions) 與 [完成登入](#get-sessions-callback)，以下 endpoint 只在 session 模式提供
:::

### Device Session 物件 {#device-session-object}

###### Device Session 結構 {#device-session-structure}

| 欄位 | 型別 | 說明 |
|---|---|---|
| device_id | string | Device session 的 ID |
| device_name | string | 送到 [完成登入](#get-sessions-callback) 的 `X-Device-Name`，預設 `Unknown Device` |
| updated_at | integer | 最後一次登入或 refresh 的 Unix 秒數 |
| current | boolean | 此 request 的 access token 是否屬於這個 session |

###### Device Session 範例 {#example-device-session}

```json
{
  "device_id": "3f1c2a9e-...",
  "device_name": "My Laptop",
  "updated_at": 1767225600,
  "current": true
}
```

### Token Pair 物件 {#token-pair-object}

###### Token Pair 結構 {#token-pair-structure}

| 欄位 | 型別 | 說明 |
|---|---|---|
| access_token | string | Session 模式為 [access token](../topics/tokens.md#access-token)<br>stateless proxy 為 provider 的 access token |
| refresh_token | string | Session 模式為 [refresh token](../topics/tokens.md#refresh-token)<br>stateless proxy 為 provider 的 refresh token |

### Session Status 物件 {#session-status-object}

###### Session Status 結構 {#session-status-structure}

| 欄位 | 型別 | 說明 |
|---|---|---|
| device_id | string | Device session 的 ID |
| issued_at | integer | Refresh token 簽發時間，Unix 秒數 |
| expires_at | integer | Refresh token 到期時間，Unix 秒數 |

### Authorization 物件 {#authorization-object}

###### Authorization 結構 {#authorization-structure}

| 欄位 | 型別 | 說明 |
|---|---|---|
| url | string | Provider 的授權 URL，將使用者導向此處 |
| verifier? | string | PKCE verifier<br>只在 `OAUTH2_CLIENT_PKCE=true` 時出現<br>見 [Client PKCE](../topics/oauth2.md#client-pkce) |

## 開始登入 % POST /sessions

發起登入，回傳 [Authorization](#authorization-object) 物件。

Device session 稍後由 [完成登入](#get-sessions-callback) 建立。

###### JSON Params {#post-sessions-json-params}

| 欄位 | 型別 | 說明 |
|---|---|---|
| redirect_uri? | string | 必須與 `OAUTH2_REDIRECT_URL` 或 `OAUTH2_ALLOWED_REDIRECT_URLS` 其中一項完全相同，否則 `400`<br>未帶時使用 `OAUTH2_REDIRECT_URL`，未設定則 `400` |

## 完成登入 % GET /sessions/callback

以 authorization code 換取 token。

由 redirect URL 頁面傳入從 provider 收到的 `code` 與 `state`。

- Session 模式：回傳 [Token Pair](#token-pair-object) 物件並設定 `refresh_token` cookie
- Stateless proxy：以 Token Pair 物件回傳 provider 的 token
  - `PASS_OAUTH_TOKEN=true` 時改放在 header，`message` 為 `Logged in`
  - 見 [Provider Token](../topics/oauth2.md#provider-tokens)

###### Query String Params {#get-sessions-callback-query-string-params}

| 欄位 | 型別 | 說明 |
|---|---|---|
| code | string | Provider 給的 authorization code |
| state | string | [開始登入](#post-sessions) 產生的一次性 `state`，使用後刪除 |
| iss? | string | Authorization response 的 issuer，provider 有宣告時才檢查 |
| error? | string | Provider 回傳的錯誤 |

###### Request Header {#get-sessions-callback-request-headers}

| Header | 說明 |
|---|---|
| X-Device-Name? | 新 device session 的名稱，預設 `Unknown Device` |
| X-PKCE-Verifier? | `OAUTH2_CLIENT_PKCE=true` 時必填 |

::: warning
`iss` 不符時回 `400` 並帶 `X-Auth-Error: invalid_issuer`

見 [Issuer 錯誤](../topics/status-codes.md#issuer-errors)
:::

## 取得 Device Session % GET /sessions

回傳該使用者所有 [Device Session](#device-session-object) 的 array，最近 refresh 的在前。

需要 access token。

## 刪除 Device Session % DELETE /sessions

撤銷該使用者所有 device session（包括目前這個）並清除 cookie，其 access token 立即無法通過 [驗證 Access Token](./verify.md#get-verify)。

需要 access token。

## 替換 Refresh Token % POST /sessions/current/tokens

替換 refresh token，回傳新的 [Token Pair](#token-pair-object) 物件並設定 `refresh_token` cookie。

見 [Rotation](../topics/tokens.md#rotation)。

###### JSON Params {#post-sessions-current-tokens-json-params}

| 欄位 | 型別 | 說明 |
|---|---|---|
| refresh_token? | string | 只在沒有 `refresh_token` cookie 時讀取 |

::: danger
注意[重複使用偵測](../topics/tokens.md#reuse-detection)
:::

## 建立 Access Token % POST /sessions/current/access-token

簽發新的 access token，不 rotate refresh token。

`message` 為新的 access token 字串。

###### JSON Params {#post-sessions-current-access-token-json-params}

| 欄位 | 型別 | 說明 |
|---|---|---|
| refresh_token? | string | 只在沒有 `refresh_token` cookie 時讀取 |

## 取得目前 Session % GET /sessions/current

回傳 refresh token 的 [Session Status](#session-status-object) 物件，不 rotate。

讀取 `refresh_token` cookie，沒有 cookie 時改讀 `Authorization: Bearer <refresh_token>`。

不讀 body。

## 刪除目前 Session % DELETE /sessions/current

登出：刪除 device session 並清除 cookie。

Refresh token 可不帶，沒有時仍會清除 cookie。

###### JSON Params {#delete-sessions-current-json-params}

| 欄位 | 型別 | 說明 |
|---|---|---|
| refresh_token? | string | 只在沒有 `refresh_token` cookie 時讀取 |
