package jwt

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken            = errors.New("invalid or expired token")
	ErrUnexpectedSigningMethod = errors.New("unexpected token signing method")
	ErrTypeAssertionFailed     = errors.New("failed to assert token claims")
	ErrTokenRevoked            = fmt.Errorf("%w: token generation has been revoked", ErrInvalidToken)
	ErrDeviceNotFound          = fmt.Errorf("%w: device not found", ErrInvalidToken)
	ErrInvalidSignature        = fmt.Errorf("%w: signature does not match the device secret", ErrInvalidToken)
)

const (
	AccessTokenType  = "access+jwt"
	RefreshTokenType = "refresh+jwt"
)

type Database interface {
	UpdateDeviceSecret(deviceID string) (int, error)
	GetDeviceSecret(deviceID string) (string, int, error)
}

type AccessClaims struct {
	Username   string `json:"username"`
	UserEmail  string `json:"user_email"`
	DeviceID   string `json:"device_id"`
	Generation int    `json:"generation"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	DeviceID   string `json:"device_id"`
	Generation int    `json:"generation"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	db     Database
	config *options
}

func NewJWTManager(db Database, opts ...Option) *JWTManager {
	cfg := defaultOptions()

	for _, opt := range opts {
		opt(cfg)
	}

	return &JWTManager{
		db:     db,
		config: cfg,
	}
}

func (m *JWTManager) GenerateRefresh(userEmail, deviceID, secret string, gen int) (string, error) {
	return signToken(RefreshTokenType, m.newRefreshClaims(userEmail, deviceID, gen), secret)
}

func (m *JWTManager) newRefreshClaims(userEmail, deviceID string, gen int) *RefreshClaims {
	return &RefreshClaims{
		DeviceID:   deviceID,
		Generation: gen,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Audience:  jwt.ClaimStrings{m.config.Audience},
			Subject:   userEmail,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.config.RefreshTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

func (m *JWTManager) GenerateAccess(refreshToken, username string) (string, error) {
	claims, secret, err := verifyToken(m, refreshToken, &RefreshClaims{})
	if err != nil {
		return "", err
	}

	accessClaims := AccessClaims{
		Username:   username,
		UserEmail:  claims.Subject,
		DeviceID:   claims.DeviceID,
		Generation: claims.Generation,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Audience:  jwt.ClaimStrings{m.config.Audience},
			Subject:   claims.Subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.config.AccessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	return signToken(AccessTokenType, accessClaims, secret)
}

func isExpired(expiresAt *jwt.NumericDate) bool {
	return expiresAt != nil && !time.Now().Before(expiresAt.Time)
}

func signToken(tokenType string, claims jwt.Claims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["typ"] = tokenType
	return token.SignedString([]byte(secret))
}

func hasTokenType(token *jwt.Token, expected string) bool {
	typ, _ := token.Header["typ"].(string)
	return strings.TrimPrefix(strings.ToLower(typ), "application/") == expected
}

func (m *JWTManager) VerifyAccess(accessToken string) (*AccessClaims, error) {
	claims, _, err := verifyToken(m, accessToken, &AccessClaims{})
	return claims, err
}

func (m *JWTManager) VerifyRefresh(refreshToken string) (*RefreshClaims, error) {
	claims, _, err := verifyToken(m, refreshToken, &RefreshClaims{})
	return claims, err
}

func (m *JWTManager) RotateRefresh(refreshToken string) (string, *RefreshClaims, error) {
	claims, secret, err := verifyToken(m, refreshToken, &RefreshClaims{})
	if err != nil {
		return "", claims, err
	}

	gen, err := m.db.UpdateDeviceSecret(claims.DeviceID)
	if err != nil {
		return "", claims, err
	}

	newClaims := m.newRefreshClaims(claims.Subject, claims.DeviceID, gen)
	token, err := signToken(RefreshTokenType, newClaims, secret)
	return token, newClaims, err
}

func (m *JWTManager) parserOptions() []jwt.ParserOption {
	var opts []jwt.ParserOption
	if m.config.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(m.config.Issuer))
	}
	if m.config.Audience != "" {
		opts = append(opts, jwt.WithAudience(m.config.Audience))
	}
	return opts
}

func verifyToken[T jwt.Claims](m *JWTManager, tokenStr string, claims T) (T, string, error) {
	parser := jwt.NewParser()

	unverifiedToken, _, err := parser.ParseUnverified(tokenStr, claims)
	if err != nil {
		return claims, "", ErrInvalidToken
	}

	var tokenType, deviceID string
	var generation int
	switch c := any(claims).(type) {
	case *AccessClaims:
		tokenType = AccessTokenType
		deviceID = c.DeviceID
		generation = c.Generation
	case *RefreshClaims:
		tokenType = RefreshTokenType
		deviceID = c.DeviceID
		generation = c.Generation
	default:
		return claims, "", ErrInvalidToken
	}

	if !hasTokenType(unverifiedToken, tokenType) {
		return claims, "", ErrInvalidToken
	}
	expiresAt, _ := claims.GetExpirationTime()
	if isExpired(expiresAt) {
		return claims, "", ErrInvalidToken
	}

	secret, gen, err := m.db.GetDeviceSecret(deviceID)
	if errors.Is(err, ErrDeviceNotFound) {
		return claims, "", ErrDeviceNotFound
	}
	if err != nil {
		return claims, "", ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnexpectedSigningMethod
		}
		return []byte(secret), nil
	}, m.parserOptions()...)

	if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
		return claims, "", ErrInvalidSignature
	}
	if err != nil || !token.Valid {
		return claims, "", ErrInvalidToken
	}

	if generation != gen {
		return claims, "", ErrTokenRevoked
	}

	return claims, secret, nil
}
