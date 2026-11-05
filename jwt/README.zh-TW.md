# JWT Module

簽發與驗證 access token 與 refresh token，每個 device 有各自的 signing secret

## 安裝

```bash
go get github.com/lucap9056/corvauth/jwt
```

## Database

`JWTManager` 透過 `Database` interface 讀寫 device 的 secret 與 generation：

```go
type Database interface {
	UpdateDeviceSecret(deviceID string) (int, error)
	GetDeviceSecret(deviceID string) (string, int, error)
}
```

| Method | 行為 |
|---|---|
| `UpdateDeviceSecret(deviceID)` | generation 加一並回傳新值，device 不存在時回傳 error |
| `GetDeviceSecret(deviceID)` | 回傳 secret 與目前 generation |

## 使用方式

```go
import "github.com/lucap9056/corvauth/jwt"

manager := jwt.NewJWTManager(db,
	jwt.WithIssuer("auth-service"),
	jwt.WithAudience("web"),
)
```

### 登入

呼叫端產生 secret 存入 database，再傳給 `GenerateRefresh` 簽章

新 device 的 generation 為 `1`：

```go
secret := randomHex(32)
deviceID, err := db.SaveDeviceSecret(userEmail, deviceName, secret)

refreshToken, err := manager.GenerateRefresh(userEmail, deviceID, secret, 1)
accessToken, err := manager.GenerateAccess(refreshToken, username)
```

### Rotate refresh token

`RotateRefresh` 回傳的 claims：

- 成功：新 refresh token 的 claims
- 驗證失敗：傳入 token 的 claims

```go
newRefreshToken, claims, err := manager.RotateRefresh(refreshToken)

username := lookupUsername(claims.Subject)
accessToken, err := manager.GenerateAccess(newRefreshToken, username)
```

### 驗證 access token

```go
claims, err := manager.VerifyAccess(accessToken)
```

## Token

皆以 device secret 做 HS256 簽章，以 JWS header `typ` 區分用途（RFC 8725 §3.11）

Access token，`typ`：`access+jwt`

```json
{
  "username": "alice",
  "user_email": "alice@example.com",
  "device_id": "6f1c2a9e-4b7d-4c1e-9a3f-2d8b5e7c0a14",
  "device_tag": "Qk3x9vLm2Tn8YpR4sWc1hA",
  "generation": 1,
  "iss": "auth-service",
  "sub": "alice@example.com",
  "aud": ["web"],
  "exp": 1762400700,
  "iat": 1762399800
}
```

Refresh token，`typ`：`refresh+jwt`

```json
{
  "device_id": "6f1c2a9e-4b7d-4c1e-9a3f-2d8b5e7c0a14",
  "device_tag": "hkwVBjACbjZU0JwM0yuWVQ",
  "generation": 1,
  "iss": "auth-service",
  "sub": "alice@example.com",
  "aud": ["web"],
  "exp": 1763004600,
  "iat": 1762399800
}
```

- `sub` 為使用者 email
- `device_tag`：只在設定 `WithDeviceTagSecret` 時出現 - `BASE64URL( HMAC-SHA256(secret, device_id || uint64_be(exp))[16] )`

### 驗證規則

`VerifyAccess`、`VerifyRefresh`、`GenerateAccess`（驗證傳入的 refresh token）依序檢查：

1. `typ` 符合 token 種類（不分大小寫，可帶 `application/` 前綴）
2. `exp` 未過期（在驗證簽章前讀取）
3. 有設 `WithDeviceTagSecret` 時，`device_tag` 與 `device_id`、`exp` 相符；沒有 `device_tag` 或 `exp` 的 token 一律拒絕
4. 透過 `GetDeviceSecret` 向 database 讀取 device，確認是否存在
5. 簽章正確且未過期
6. 有設 `WithIssuer` / `WithAudience` 時，`iss` / `aud` 相符
7. token 的 `generation` 等於 device 目前的 generation

### Errors

| Error | 情況 |
|---|---|
| `ErrTokenRevoked` | 只有第 7 項不符：由此服務簽發但已被 rotate |
| `ErrInvalidDeviceTag` | 第 3 項不符 |
| `ErrInvalidToken` | 其餘驗證失敗 |

`ErrTokenRevoked`、`ErrInvalidDeviceTag` 皆 wrap 了 `ErrInvalidToken`，`errors.Is(err, jwt.ErrInvalidToken)` 對三者皆成立

需要區分時先判斷 `ErrTokenRevoked`

## Options

| Option | 預設值 | 說明 |
|---|---|---|
| `WithAccessTokenDuration(d)` | `15m` | access token 有效時長 |
| `WithRefreshTokenDuration(d)` | `7d` | refresh token 有效時長 |
| `WithIssuer(iss)` | 空字串 | 寫入 `iss`，非空時驗證須相符 |
| `WithAudience(aud)` | 空字串 | 寫入 `aud`，非空時驗證須包含 |
| `WithDeviceTagSecret(secret)` | 空字串 | `device_tag` 使用的 secret；為空時不寫入也不驗證 `device_tag`。所有 instance 必須共用同一把，更換後已簽發的 token 全部失效 |

## 測試

```bash
go test ./...
```
