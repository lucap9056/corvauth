package identity

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenType       = "identity+jwt"
	MinSecretLength = 32
	TokenDuration   = time.Minute
)

type Headers struct {
	Identity  string
	UserEmail string
	DeviceID  string
	Username  string
}

var DefaultHeaders = Headers{
	Identity:  "X-Corvauth-Identity",
	UserEmail: "X-Corvauth-User-Email",
	DeviceID:  "X-Corvauth-Device-ID",
	Username:  "X-Corvauth-Username",
}

func (h Headers) Names() []string {
	return []string{h.Identity, h.UserEmail, h.DeviceID, h.Username}
}

type Claims struct {
	Username string `json:"username,omitempty"`
	DeviceID string `json:"device_id"`
	jwt.RegisteredClaims
}

type Signer struct {
	secret   []byte
	issuer   string
	audience jwt.ClaimStrings
}

func NewSigner(secret, issuer, audience string) *Signer {
	signer := &Signer{secret: []byte(secret), issuer: issuer}
	if audience != "" {
		signer.audience = jwt.ClaimStrings{audience}
	}
	return signer
}

func (s *Signer) Sign(userEmail, username, deviceID string, accessExpiresAt *jwt.NumericDate) (string, error) {
	now := time.Now()
	expiresAt := now.Add(TokenDuration)
	if accessExpiresAt != nil && accessExpiresAt.Before(expiresAt) {
		expiresAt = accessExpiresAt.Time
	}

	claims := Claims{
		Username: username,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  s.audience,
			Subject:   userEmail,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["typ"] = TokenType
	return token.SignedString(s.secret)
}
