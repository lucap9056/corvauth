# Reference

## Base URL {#base-url}

所有 endpoint 都由 Corvauth 監聽的位址（`HTTP_ADDRESS`）提供，本文件的路徑皆相對於此：

```
https://auth.example.com/verify
```

Corvauth 的路徑沒有版本前綴。

機器可讀的 [OpenAPI 規格](../public/openapi.yaml)描述相同的 endpoint。

## 各模式可用的 Endpoint {#mode-availability}

不是每個 [部署模式](./topics/deployment-modes.md) 都提供所有 endpoint：

###### 各模式的 Endpoint {#endpoints-by-mode}

| 模式 | 可用的 Endpoint |
|---|---|
| Stateless proxy | [取得健康狀態](./resources/health.md#get-health)、[開始登入](./resources/session.md#post-sessions)、[完成登入](./resources/session.md#get-callback) |
| Managed users | 全部 |
| External users | [刪除目前使用者](./resources/user.md#delete-users-me) 以外的全部 |

Managed users 與 external users 合稱 **session 模式**。

## 認證 {#authentication}

各 endpoint 接受下列其中一種 credential，詳見各 endpoint：

###### Credential 類型 {#credential-types}

| Credential | 傳送方式 | 接受的 Endpoint |
|---|---|---|
| Access token | `Authorization: Bearer <access_token>` | [取得 Device Session](./resources/session.md#get-sessions)、[刪除 Device Session](./resources/session.md#delete-sessions)、[刪除目前使用者](./resources/user.md#delete-users-me)、[驗證 Access Token](./resources/verify.md#get-verify)、[選擇性驗證 Access Token](./resources/verify.md#get-verify-optional) |
| Refresh token | `refresh_token` cookie，沒有 cookie 時改讀 JSON body `{ "refresh_token": "..." }` | [替換 Refresh Token](./resources/session.md#post-sessions-current-tokens)、[建立 Access Token](./resources/session.md#post-sessions-current-access-token)、[刪除目前 Session](./resources/session.md#delete-sessions-current) |
| Refresh token（Bearer） | `refresh_token` cookie，沒有 cookie 時改讀 `Authorization: Bearer <refresh_token>` | [取得目前 Session](./resources/session.md#get-sessions-current) |

Token 缺少或無效時回 `401` 並帶 `WWW-Authenticate: Bearer`。

Token 有效期與 rotation 見 [Token](./topics/tokens.md)。

## 回應格式 {#response-format}

JSON 回應都包在同一個 envelope 中：

###### Response Envelope 結構 {#response-envelope-structure}

| 欄位 | 型別 | 說明 |
|---|---|---|
| success | boolean | Request 是否成功 |
| message | string、object 或 array | 成功時為結果<br>失敗時為錯誤說明 |

Endpoint 說明中「回傳 [Token Pair](./resources/session.md#token-pair-object) 物件」指的就是 `message` 欄位的內容。

###### 成功回應範例 {#example-success-response}

```json
{
  "success": true,
  "message": {
    "access_token": "...",
    "refresh_token": "..."
  }
}
```

###### 錯誤回應範例 {#example-error-response}

```json
{
  "success": false,
  "message": "Invalid or expired state"
}
```

機器可判讀的原因由 HTTP status code 與 `X-Auth-Error` header 表示。

見 [Status Code](./topics/status-codes.md)。

## 選填欄位 {#optional-fields}

可能省略的欄位名稱以 `?` 結尾：

| 欄位 | 型別 |
|---|---|
| optional_field? | string |

## 時間戳記 {#timestamps}

時間戳記皆為 Unix 秒數的整數，例如 `1767225600`。

## Duration {#durations}

Duration 類型的變數由一或多段 `<integer><unit>` 組成，單位為 `d`、`h`、`m`、`s`：

| 範例 | 意義 |
|---|---|
| `30m` | 30 分鐘 |
| `7d` | 7 天 |
| `1d12h` | 36 小時 |

部分變數也接受不帶單位的整數，以預設單位計算。

見 [設定](./topics/configuration.md)。

## 快取 {#caching}

帶有 token 或 session 資料的回應會設定 `Cache-Control: no-store`。
