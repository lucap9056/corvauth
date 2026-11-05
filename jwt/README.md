# JWT Module

Issues and verifies access and refresh tokens, with one signing secret per device

## Installation

```bash
go get github.com/lucap9056/corvauth/jwt
```

## Database

`JWTManager` reads and updates a device's secret and generation through the `Database` interface:

```go
type Database interface {
	UpdateDeviceSecret(deviceID string) (int, error)
	GetDeviceSecret(deviceID string) (string, int, error)
}
```

| Method | Behavior |
|---|---|
| `UpdateDeviceSecret(deviceID)` | Increments the generation and returns it; errors if the device does not exist |
| `GetDeviceSecret(deviceID)` | Returns the secret and current generation |

## Usage

```go
import "github.com/lucap9056/corvauth/jwt"

manager := jwt.NewJWTManager(db,
	jwt.WithIssuer("auth-service"),
	jwt.WithAudience("web"),
)
```

### Login

The caller generates the secret, stores it, and passes it to `GenerateRefresh`

New devices start at generation `1`:

```go
secret := randomHex(32)
deviceID, err := db.SaveDeviceSecret(userEmail, deviceName, secret)

refreshToken, err := manager.GenerateRefresh(userEmail, deviceID, secret, 1)
accessToken, err := manager.GenerateAccess(refreshToken, username)
```

### Rotating the refresh token

Claims returned by `RotateRefresh`:

- success: the new refresh token's claims
- verification failure: the given token's claims

```go
newRefreshToken, claims, err := manager.RotateRefresh(refreshToken)

username := lookupUsername(claims.Subject)
accessToken, err := manager.GenerateAccess(newRefreshToken, username)
```

### Verifying an access token

```go
claims, err := manager.VerifyAccess(accessToken)
```

## Tokens

Both are signed with HS256 using the device secret; the JWS `typ` header marks their purpose (RFC 8725 §3.11)

Access token, `typ`: `access+jwt`

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

Refresh token, `typ`: `refresh+jwt`

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

- `sub` is the user's email
- `device_tag`: present only when `WithDeviceTagSecret` is set - `BASE64URL( HMAC-SHA256(secret, device_id || uint64_be(exp))[16] )`

### Verification

`VerifyAccess`, `VerifyRefresh`, and `GenerateAccess` (on the given refresh token) check, in order:

1. `typ` matches the token kind (case-insensitive, optional `application/` prefix)
2. `exp`, read before the signature is checked, has not passed
3. `device_tag` matches `device_id` and `exp` when `WithDeviceTagSecret` is set; tokens without it or without `exp` are rejected
4. The device is read from the database through `GetDeviceSecret` and exists
5. The signature is valid and not expired
6. `iss` / `aud` match when `WithIssuer` / `WithAudience` are set
7. The token's `generation` equals the device's current generation

### Errors

| Error | When |
|---|---|
| `ErrTokenRevoked` | Only check 7 fails: issued here but since rotated |
| `ErrInvalidDeviceTag` | Check 3 fails |
| `ErrInvalidToken` | Any other failure |

`ErrTokenRevoked` and `ErrInvalidDeviceTag` wrap `ErrInvalidToken`, so `errors.Is(err, jwt.ErrInvalidToken)` holds for all of them

Check `ErrTokenRevoked` first to tell them apart

## Options

| Option | Default | Description |
|---|---|---|
| `WithAccessTokenDuration(d)` | `15m` | Access token lifetime |
| `WithRefreshTokenDuration(d)` | `7d` | Refresh token lifetime |
| `WithIssuer(iss)` | empty | Written to `iss`; must match when non-empty |
| `WithAudience(aud)` | empty | Written to `aud`; must be contained when non-empty |
| `WithDeviceTagSecret(secret)` | empty | Secret for `device_tag`; when empty, `device_tag` is neither written nor checked. Must be shared by all instances; changing it invalidates every issued token |

## Testing

```bash
go test ./...
```
