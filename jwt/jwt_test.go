package jwt

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testEmail    = "user@example.com"
	testDeviceID = "device456"
	testUsername = "testuser"
	testSecret   = "device-secret"
)

func newTestManager(t *testing.T, opts ...Option) (*JWTManager, *MockDatabase) {
	t.Helper()
	db := NewMockDatabase()
	db.AddDevice(testDeviceID, testSecret)
	return NewJWTManager(db, opts...), db
}

func mustGenerateRefresh(t *testing.T, manager *JWTManager) string {
	t.Helper()
	token, err := manager.GenerateRefresh(testEmail, testDeviceID, testSecret, 1)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	return token
}

func mustRotateRefresh(t *testing.T, manager *JWTManager, refreshToken string) string {
	t.Helper()
	token, _, err := manager.RotateRefresh(refreshToken)
	if err != nil {
		t.Fatalf("failed to rotate refresh token: %v", err)
	}
	return token
}

func mustGenerateAccess(t *testing.T, manager *JWTManager, refreshToken string) string {
	t.Helper()
	token, err := manager.GenerateAccess(refreshToken, testUsername)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	return token
}

func TestJWTManager_VerifyRefresh(t *testing.T) {
	manager, db := newTestManager(t, WithIssuer("auth-service"), WithAudience("web"))

	claims, err := manager.VerifyRefresh(mustGenerateRefresh(t, manager))
	if err != nil {
		t.Fatalf("failed to verify refresh token: %v", err)
	}

	if claims.DeviceID != testDeviceID {
		t.Errorf("expected device ID %s, got %s", testDeviceID, claims.DeviceID)
	}
	if claims.Subject != testEmail {
		t.Errorf("expected subject %s, got %s", testEmail, claims.Subject)
	}
	if claims.Issuer != "auth-service" {
		t.Errorf("expected issuer auth-service, got %s", claims.Issuer)
	}
	if claims.Generation != 1 {
		t.Errorf("expected generation 1, got %d", claims.Generation)
	}
	if db.devices[testDeviceID].generation != 1 {
		t.Errorf("expected stored generation to stay 1, got %d", db.devices[testDeviceID].generation)
	}
}

func TestJWTManager_VerifyAccess(t *testing.T) {
	manager, _ := newTestManager(t, WithIssuer("auth-service"), WithAudience("web"))

	refreshToken := mustGenerateRefresh(t, manager)
	claims, err := manager.VerifyAccess(mustGenerateAccess(t, manager, refreshToken))
	if err != nil {
		t.Fatalf("failed to verify access token: %v", err)
	}

	if claims.UserEmail != testEmail {
		t.Errorf("expected user email %s, got %s", testEmail, claims.UserEmail)
	}
	if claims.Subject != testEmail {
		t.Errorf("expected subject %s, got %s", testEmail, claims.Subject)
	}
	if claims.Username != testUsername {
		t.Errorf("expected username %s, got %s", testUsername, claims.Username)
	}
	if claims.DeviceID != testDeviceID {
		t.Errorf("expected device ID %s, got %s", testDeviceID, claims.DeviceID)
	}
	if claims.Generation != 1 {
		t.Errorf("expected generation 1, got %d", claims.Generation)
	}
}

func TestJWTManager_RotateRefresh(t *testing.T) {
	manager, db := newTestManager(t, WithIssuer("auth-service"), WithAudience("web"))

	newRefresh, returned, err := manager.RotateRefresh(mustGenerateRefresh(t, manager))
	if err != nil {
		t.Fatalf("failed to rotate refresh token: %v", err)
	}

	verified, err := manager.VerifyRefresh(newRefresh)
	if err != nil {
		t.Fatalf("failed to verify rotated refresh token: %v", err)
	}

	for name, claims := range map[string]*RefreshClaims{"returned": returned, "verified": verified} {
		if claims.Subject != testEmail {
			t.Errorf("%s: expected subject %s, got %s", name, testEmail, claims.Subject)
		}
		if claims.DeviceID != testDeviceID {
			t.Errorf("%s: expected device ID %s, got %s", name, testDeviceID, claims.DeviceID)
		}
		if claims.Issuer != "auth-service" {
			t.Errorf("%s: expected issuer auth-service, got %s", name, claims.Issuer)
		}
		if claims.Generation != 2 {
			t.Errorf("%s: expected generation 2, got %d", name, claims.Generation)
		}
	}
	if db.devices[testDeviceID].generation != 2 {
		t.Errorf("expected stored generation 2, got %d", db.devices[testDeviceID].generation)
	}
}

func TestJWTManager_RotateRefresh_RevokedTokenDoesNotBumpGeneration(t *testing.T) {
	manager, db := newTestManager(t)

	oldRefresh := mustGenerateRefresh(t, manager)
	mustRotateRefresh(t, manager, oldRefresh)

	_, claims, err := manager.RotateRefresh(oldRefresh)
	if !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("expected ErrTokenRevoked, got %v", err)
	}
	if claims.DeviceID != testDeviceID {
		t.Errorf("expected revoked token's device ID %s, got %s", testDeviceID, claims.DeviceID)
	}
	if db.devices[testDeviceID].generation != 2 {
		t.Errorf("expected stored generation to stay 2, got %d", db.devices[testDeviceID].generation)
	}
}

func TestJWTManager_RotateRefresh_UpdateError(t *testing.T) {
	manager, db := newTestManager(t)
	refreshToken := mustGenerateRefresh(t, manager)
	db.updateErr = errors.New("db down")

	if _, _, err := manager.RotateRefresh(refreshToken); err == nil {
		t.Fatal("expected error when UpdateDeviceSecret fails")
	}
}

func TestJWTManager_RotatedRefreshTokenIsRevoked(t *testing.T) {
	manager, _ := newTestManager(t)

	oldRefresh := mustGenerateRefresh(t, manager)
	oldAccess := mustGenerateAccess(t, manager, oldRefresh)
	newRefresh := mustRotateRefresh(t, manager, oldRefresh)

	if _, err := manager.VerifyRefresh(oldRefresh); !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("expected rotated refresh token to be revoked, got %v", err)
	}
	if _, err := manager.GenerateAccess(oldRefresh, testUsername); !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("expected rotated refresh token to be unable to mint access token, got %v", err)
	}
	if _, err := manager.VerifyAccess(oldAccess); !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("expected access token of previous generation to be revoked, got %v", err)
	}
	if _, err := manager.VerifyRefresh(newRefresh); err != nil {
		t.Errorf("expected new refresh token to be valid, got %v", err)
	}
	if _, err := manager.VerifyAccess(mustGenerateAccess(t, manager, newRefresh)); err != nil {
		t.Errorf("expected new access token to be valid, got %v", err)
	}
}

func TestErrTokenRevoked_WrapsErrInvalidToken(t *testing.T) {
	if !errors.Is(ErrTokenRevoked, ErrInvalidToken) {
		t.Fatal("expected ErrTokenRevoked to match ErrInvalidToken")
	}
}

func TestJWTManager_RejectsInvalidTokens(t *testing.T) {
	verifyRefreshWith := func(t *testing.T, opts ...Option) error {
		issuer, db := newTestManager(t, WithIssuer("auth-service"), WithAudience("web"))
		refreshToken := mustGenerateRefresh(t, issuer)
		_, err := NewJWTManager(db, opts...).VerifyRefresh(refreshToken)
		return err
	}

	tests := []struct {
		name   string
		verify func(t *testing.T) error
	}{
		{"malformed", func(t *testing.T) error {
			manager, _ := newTestManager(t)
			_, err := manager.VerifyAccess("not-a-jwt")
			return err
		}},
		{"secret changed", func(t *testing.T) error {
			manager, db := newTestManager(t)
			refreshToken := mustGenerateRefresh(t, manager)
			db.devices[testDeviceID].secret = "different-secret"
			_, err := manager.VerifyRefresh(refreshToken)
			return err
		}},
		{"missing device", func(t *testing.T) error {
			manager, db := newTestManager(t)
			refreshToken := mustGenerateRefresh(t, manager)
			delete(db.devices, testDeviceID)
			_, err := manager.VerifyRefresh(refreshToken)
			return err
		}},
		{"refresh expired", func(t *testing.T) error {
			manager, _ := newTestManager(t, WithRefreshTokenDuration(-time.Second))
			_, err := manager.VerifyRefresh(mustGenerateRefresh(t, manager))
			return err
		}},
		{"access expired", func(t *testing.T) error {
			manager, _ := newTestManager(t, WithAccessTokenDuration(-time.Second))
			_, err := manager.VerifyAccess(mustGenerateAccess(t, manager, mustGenerateRefresh(t, manager)))
			return err
		}},
		{"issuer mismatch", func(t *testing.T) error {
			return verifyRefreshWith(t, WithIssuer("other-service"), WithAudience("web"))
		}},
		{"audience mismatch", func(t *testing.T) error {
			return verifyRefreshWith(t, WithIssuer("auth-service"), WithAudience("mobile"))
		}},
		{"access used as refresh", func(t *testing.T) error {
			manager, _ := newTestManager(t)
			_, err := manager.VerifyRefresh(mustGenerateAccess(t, manager, mustGenerateRefresh(t, manager)))
			return err
		}},
		{"refresh used as access", func(t *testing.T) error {
			manager, _ := newTestManager(t)
			_, err := manager.VerifyAccess(mustGenerateRefresh(t, manager))
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.verify(t)
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("expected ErrInvalidToken, got %v", err)
			}
			if errors.Is(err, ErrTokenRevoked) {
				t.Fatal("expected non-generation failure not to be ErrTokenRevoked")
			}
		})
	}
}

func TestJWTManager_VerifyRefresh_TokenTypeVariants(t *testing.T) {
	tests := []struct {
		typ     string
		wantErr bool
	}{
		{RefreshTokenType, false},
		{"REFRESH+JWT", false},
		{"application/refresh+jwt", false},
		{"JWT", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			manager, db := newTestManager(t)
			gen, _ := db.UpdateDeviceSecret(testDeviceID)

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, RefreshClaims{
				DeviceID:   testDeviceID,
				Generation: gen,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   testEmail,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			})
			if tt.typ == "" {
				delete(token.Header, "typ")
			} else {
				token.Header["typ"] = tt.typ
			}
			signed, err := token.SignedString([]byte(testSecret))
			if err != nil {
				t.Fatalf("failed to sign token: %v", err)
			}

			_, err = manager.VerifyRefresh(signed)
			if (err != nil) != tt.wantErr {
				t.Fatalf("typ %q: error = %v, wantErr %v", tt.typ, err, tt.wantErr)
			}
		})
	}
}

func TestJWTManager_ExpiredTokenSkipsDeviceLookup(t *testing.T) {
	manager, db := newTestManager(t, WithRefreshTokenDuration(-time.Second))
	refreshToken := mustGenerateRefresh(t, manager)
	delete(db.devices, testDeviceID)

	_, err := manager.VerifyRefresh(refreshToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
	if errors.Is(err, ErrDeviceNotFound) {
		t.Fatal("expected expired token to be rejected before the device lookup")
	}
}

func TestJWTManager_RejectsUnexpectedAlgorithmBeforeDeviceLookup(t *testing.T) {
	manager := NewJWTManager(NewMockDatabase())
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	payload := encode(`{"device_id":"` + testDeviceID + `"}`)

	for _, alg := range []string{"none", "HS384", "XYZ", ""} {
		token := encode(`{"alg":"`+alg+`","typ":"`+RefreshTokenType+`"}`) + "." + payload + "."
		_, err := manager.VerifyRefresh(token)
		if !errors.Is(err, ErrUnexpectedSigningMethod) || !errors.Is(err, ErrInvalidToken) {
			t.Errorf("alg %q: expected ErrUnexpectedSigningMethod wrapping ErrInvalidToken, got %v", alg, err)
		}
	}
}

func TestJWTManager_DeviceTag(t *testing.T) {
	manager, _ := newTestManager(t, WithDeviceTagSecret("device-tag-secret"))
	refreshToken := mustGenerateRefresh(t, manager)

	refreshClaims, err := manager.VerifyRefresh(refreshToken)
	if err != nil {
		t.Fatalf("failed to verify refresh token: %v", err)
	}
	accessClaims, err := manager.VerifyAccess(mustGenerateAccess(t, manager, refreshToken))
	if err != nil {
		t.Fatalf("failed to verify access token: %v", err)
	}
	if refreshClaims.DeviceTag == "" || accessClaims.DeviceTag == "" {
		t.Errorf("expected both tokens to carry a device tag, got %q and %q", refreshClaims.DeviceTag, accessClaims.DeviceTag)
	}

	untagged, _ := newTestManager(t)
	claims, err := untagged.VerifyRefresh(mustGenerateRefresh(t, untagged))
	if err != nil || claims.DeviceTag != "" {
		t.Errorf("expected untagged token without secret, got tag %q, err %v", claims.DeviceTag, err)
	}
}

func TestJWTManager_RejectsInvalidDeviceTagBeforeDeviceLookup(t *testing.T) {
	const unknownDeviceID = "forged-device"
	manager, _ := newTestManager(t, WithDeviceTagSecret("device-tag-secret"))
	otherServer, _ := newTestManager(t, WithDeviceTagSecret("other-secret"))
	expiresAt := jwt.NewNumericDate(time.Now().Add(time.Hour))
	earlierExpiry := jwt.NewNumericDate(time.Now().Add(time.Minute))

	tests := map[string]struct {
		tag       string
		expiresAt *jwt.NumericDate
	}{
		"missing":             {"", expiresAt},
		"from another secret": {otherServer.deviceTag(unknownDeviceID, expiresAt), expiresAt},
		"for another device":  {manager.deviceTag(testDeviceID, expiresAt), expiresAt},
		"for another expiry":  {manager.deviceTag(unknownDeviceID, earlierExpiry), expiresAt},
		"without expiry":      {manager.deviceTag(unknownDeviceID, expiresAt), nil},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			token, err := signToken(RefreshTokenType, RefreshClaims{
				DeviceID:         unknownDeviceID,
				DeviceTag:        tt.tag,
				RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: tt.expiresAt},
			}, testSecret)
			if err != nil {
				t.Fatalf("failed to sign token: %v", err)
			}
			if _, err := manager.VerifyRefresh(token); !errors.Is(err, ErrInvalidDeviceTag) {
				t.Fatalf("expected ErrInvalidDeviceTag, got %v", err)
			}
		})
	}
}
