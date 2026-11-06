package providers

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const testIssuer = "https://issuer.example.com"

// makeIDToken creates an unsigned JWT for testing purposes only.
func makeIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal id_token claims: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".fakesig"
}

func newOIDCServer(t *testing.T, discovery *OIDCDiscovery, userinfoHandler, revokeHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		json.MarshalWrite(w, discovery)
	})
	if userinfoHandler != nil {
		mux.HandleFunc("/userinfo", userinfoHandler)
	}
	if revokeHandler != nil {
		mux.HandleFunc("/revoke", revokeHandler)
	}
	return httptest.NewServer(mux)
}

// --- FetchDiscovery ---

func TestFetchDiscovery_Success(t *testing.T) {
	expected := &OIDCDiscovery{
		Issuer:                "https://example.com",
		AuthorizationEndpoint: "https://example.com/authorize",
		TokenEndpoint:         "https://example.com/token",
		UserinfoEndpoint:      "https://example.com/userinfo",
		RevocationEndpoint:    "https://example.com/revoke",
	}
	srv := newOIDCServer(t, expected, nil, nil)
	defer srv.Close()

	doc, err := FetchDiscovery(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.AuthorizationEndpoint != expected.AuthorizationEndpoint {
		t.Errorf("AuthorizationEndpoint: got %q, want %q", doc.AuthorizationEndpoint, expected.AuthorizationEndpoint)
	}
	if doc.TokenEndpoint != expected.TokenEndpoint {
		t.Errorf("TokenEndpoint: got %q, want %q", doc.TokenEndpoint, expected.TokenEndpoint)
	}
}

func TestFetchDiscovery_TrailingSlashNormalized(t *testing.T) {
	expected := &OIDCDiscovery{
		Issuer:                "https://example.com",
		AuthorizationEndpoint: "https://example.com/authorize",
		TokenEndpoint:         "https://example.com/token",
	}
	srv := newOIDCServer(t, expected, nil, nil)
	defer srv.Close()

	_, err := FetchDiscovery(context.Background(), srv.URL+"/")
	if err != nil {
		t.Fatalf("unexpected error with trailing slash: %v", err)
	}
}

func TestFetchDiscovery_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := FetchDiscovery(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for non-OK status, got nil")
	}
}

func TestFetchDiscovery_RetriesOnServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		json.MarshalWrite(w, OIDCDiscovery{
			Issuer:                "http://" + r.Host,
			AuthorizationEndpoint: "http://" + r.Host + "/authorize",
			TokenEndpoint:         "http://" + r.Host + "/token",
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var retries int
	onRetry := WithDiscoveryRetryHandler(func(error) { retries++ })

	if _, err := FetchDiscovery(ctx, srv.URL, onRetry); err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("expected 2 calls, got %d", got)
	}
	if retries != 1 {
		t.Errorf("expected retry handler to be called once, got %d", retries)
	}
}

func TestFetchDiscovery_StopsRetryingWhenContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := FetchDiscovery(ctx, srv.URL); err == nil {
		t.Fatal("expected error after context timeout, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("expected prompt return after context timeout, took %v", elapsed)
	}
}

func TestFetchDiscovery_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := FetchDiscovery(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestFetchDiscovery_MissingRequiredEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.MarshalWrite(w, map[string]string{"issuer": "https://example.com"})
	}))
	defer srv.Close()

	_, err := FetchDiscovery(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for missing endpoints, got nil")
	}
}

func TestFetchDiscovery_MissingIssuer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.MarshalWrite(w, OIDCDiscovery{
			AuthorizationEndpoint: "https://example.com/authorize",
			TokenEndpoint:         "https://example.com/token",
		})
	}))
	defer srv.Close()

	_, err := FetchDiscovery(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for missing issuer, got nil")
	}
}

// --- parseIDTokenClaims ---

func TestParseIDTokenClaims_ValidAudString(t *testing.T) {
	token := makeIDToken(t, map[string]any{
		"sub":   "user-1",
		"email": "user@example.com",
		"name":  "Test User",
		"aud":   "client-id",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	claims, err := parseIDTokenClaims(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != "user-1" {
		t.Errorf("Sub: got %q, want %q", claims.Sub, "user-1")
	}
	if claims.Email != "user@example.com" {
		t.Errorf("Email: got %q, want %q", claims.Email, "user@example.com")
	}
	if len(claims.Aud) != 1 || claims.Aud[0] != "client-id" {
		t.Errorf("Aud: got %v, want [client-id]", claims.Aud)
	}
}

func TestParseIDTokenClaims_ValidAudArray(t *testing.T) {
	token := makeIDToken(t, map[string]any{
		"sub":   "user-2",
		"email": "user2@example.com",
		"aud":   []string{"client-id", "other-client"},
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	claims, err := parseIDTokenClaims(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims.Aud) != 2 {
		t.Errorf("Aud: got %v, want 2 entries", claims.Aud)
	}
}

func TestParseIDTokenClaims_Expired(t *testing.T) {
	token := makeIDToken(t, map[string]any{
		"sub": "user-1",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	_, err := parseIDTokenClaims(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestParseIDTokenClaims_MalformedParts(t *testing.T) {
	_, err := parseIDTokenClaims("only.two")
	if err == nil {
		t.Fatal("expected error for wrong number of parts, got nil")
	}
}

func TestParseIDTokenClaims_InvalidBase64(t *testing.T) {
	_, err := parseIDTokenClaims("header.!!!invalid!!!.sig")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestParseIDTokenClaims_NoExpiry(t *testing.T) {
	token := makeIDToken(t, map[string]any{
		"sub":   "user-1",
		"email": "user@example.com",
	})

	claims, err := parseIDTokenClaims(token)
	if err != nil {
		t.Fatalf("token without exp should not error: %v", err)
	}
	if claims.Sub != "user-1" {
		t.Errorf("Sub: got %q, want %q", claims.Sub, "user-1")
	}
}

// --- OIDCProvider.GetUser ---

func TestOIDCProvider_GetUser_ViaIDToken(t *testing.T) {
	idToken := makeIDToken(t, map[string]any{
		"iss":            testIssuer,
		"aud":            "test-client-id",
		"sub":            "oidc-user-1",
		"email":          "oidc@example.com",
		"email_verified": true,
		"name":           "OIDC User",
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	token := newTestToken().WithExtra(map[string]any{"id_token": idToken})

	provider := NewOIDCProvider(newTestConfig("http://localhost"), &OIDCDiscovery{Issuer: testIssuer})
	user, err := provider.GetUser(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "oidc-user-1" {
		t.Errorf("ID: got %q, want %q", user.ID, "oidc-user-1")
	}
	if user.Email != "oidc@example.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "oidc@example.com")
	}
	if user.Name != "OIDC User" {
		t.Errorf("Name: got %q, want %q", user.Name, "OIDC User")
	}
}

func TestOIDCProvider_GetUser_UserinfoFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.MarshalWrite(w, idTokenClaims{
			Sub:           "oidc-user-2",
			Email:         "fallback@example.com",
			EmailVerified: true,
			Name:          "Fallback User",
		})
	}))
	defer srv.Close()

	discovery := &OIDCDiscovery{Issuer: testIssuer, UserinfoEndpoint: srv.URL}
	provider := NewOIDCProvider(newTestConfig(srv.URL), discovery)

	user, err := provider.GetUser(testContextWithClient(srv.URL), newTestToken())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "oidc-user-2" {
		t.Errorf("ID: got %q, want %q", user.ID, "oidc-user-2")
	}
	if user.Email != "fallback@example.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "fallback@example.com")
	}
}

func TestOIDCProvider_GetUser_IDTokenMissingEmail_FallsBackToUserinfo(t *testing.T) {
	idToken := makeIDToken(t, map[string]any{
		"iss": testIssuer,
		"aud": "test-client-id",
		"sub": "oidc-user-3",
		"exp": time.Now().Add(time.Hour).Unix(),
		// no email
	})
	token := newTestToken().WithExtra(map[string]any{"id_token": idToken})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.MarshalWrite(w, idTokenClaims{
			Sub:           "oidc-user-3",
			Email:         "fromendpoint@example.com",
			EmailVerified: true,
		})
	}))
	defer srv.Close()

	discovery := &OIDCDiscovery{Issuer: testIssuer, UserinfoEndpoint: srv.URL}
	provider := NewOIDCProvider(newTestConfig(srv.URL), discovery)

	user, err := provider.GetUser(testContextWithClient(srv.URL), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "fromendpoint@example.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "fromendpoint@example.com")
	}
}

func TestOIDCProvider_GetUser_UnverifiedEmailRejected(t *testing.T) {
	idToken := makeIDToken(t, map[string]any{
		"iss":            testIssuer,
		"aud":            "test-client-id",
		"sub":            "oidc-user-4",
		"email":          "unverified@example.com",
		"email_verified": false,
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	token := newTestToken().WithExtra(map[string]any{"id_token": idToken})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.MarshalWrite(w, idTokenClaims{Sub: "oidc-user-4", Email: "unverified@example.com"})
	}))
	defer srv.Close()

	discovery := &OIDCDiscovery{Issuer: testIssuer, UserinfoEndpoint: srv.URL}
	provider := NewOIDCProvider(newTestConfig(srv.URL), discovery)

	_, err := provider.GetUser(testContextWithClient(srv.URL), token)
	if !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("got error %v, want %v", err, ErrUnverifiedEmail)
	}
}

func TestOIDCProvider_GetUser_UnverifiedEmailAllowed(t *testing.T) {
	idToken := makeIDToken(t, map[string]any{
		"iss":   testIssuer,
		"aud":   "test-client-id",
		"sub":   "oidc-user-5",
		"email": "unverified@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	token := newTestToken().WithExtra(map[string]any{"id_token": idToken})

	provider := NewOIDCProvider(newTestConfig("http://localhost"), &OIDCDiscovery{Issuer: testIssuer}, WithAllowUnverifiedEmail(true))
	user, err := provider.GetUser(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "unverified@example.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "unverified@example.com")
	}
}

func TestOIDCProvider_GetUser_AudArrayContainingClientID(t *testing.T) {
	idToken := makeIDToken(t, map[string]any{
		"iss":            testIssuer,
		"aud":            []string{"other-client", "test-client-id"},
		"sub":            "oidc-user-6",
		"email":          "oidc@example.com",
		"email_verified": true,
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	token := newTestToken().WithExtra(map[string]any{"id_token": idToken})

	provider := NewOIDCProvider(newTestConfig("http://localhost"), &OIDCDiscovery{Issuer: testIssuer})
	user, err := provider.GetUser(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "oidc-user-6" {
		t.Errorf("ID: got %q, want %q", user.ID, "oidc-user-6")
	}
}

func TestOIDCProvider_GetUser_IssuerMismatchRejected(t *testing.T) {
	idToken := makeIDToken(t, map[string]any{
		"iss":            "https://evil.example.com",
		"aud":            "test-client-id",
		"sub":            "oidc-user-7",
		"email":          "oidc@example.com",
		"email_verified": true,
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	token := newTestToken().WithExtra(map[string]any{"id_token": idToken})

	provider := NewOIDCProvider(newTestConfig("http://localhost"), &OIDCDiscovery{Issuer: testIssuer, UserinfoEndpoint: "http://localhost/userinfo"})
	if _, err := provider.GetUser(context.Background(), token); err == nil {
		t.Fatal("expected error for issuer mismatch, got nil")
	}
}

func TestOIDCProvider_GetUser_AudienceMismatchRejected(t *testing.T) {
	idToken := makeIDToken(t, map[string]any{
		"iss":            testIssuer,
		"aud":            []string{"other-client"},
		"sub":            "oidc-user-8",
		"email":          "oidc@example.com",
		"email_verified": true,
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	token := newTestToken().WithExtra(map[string]any{"id_token": idToken})

	provider := NewOIDCProvider(newTestConfig("http://localhost"), &OIDCDiscovery{Issuer: testIssuer, UserinfoEndpoint: "http://localhost/userinfo"})
	if _, err := provider.GetUser(context.Background(), token); err == nil {
		t.Fatal("expected error for audience mismatch, got nil")
	}
}

func TestOIDCProvider_GetUser_NoIDTokenNoUserinfoEndpoint(t *testing.T) {
	provider := NewOIDCProvider(newTestConfig("http://localhost"), &OIDCDiscovery{})

	_, err := provider.GetUser(context.Background(), newTestToken())
	if err == nil {
		t.Fatal("expected error when neither id_token nor userinfo endpoint available")
	}
}

// --- OIDCProvider.Revoke ---

func TestOIDCProvider_Revoke_Success(t *testing.T) {
	var receivedToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		receivedToken = r.FormValue("token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	discovery := &OIDCDiscovery{RevocationEndpoint: srv.URL}
	provider := NewOIDCProvider(newTestConfig(srv.URL), discovery)
	provider.httpClient = newTestClient(srv.URL)

	tok := newTestToken()
	if err := provider.Revoke(context.Background(), tok); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedToken != tok.RefreshToken {
		t.Errorf("expected refresh token %q to be sent, got %q", tok.RefreshToken, receivedToken)
	}
}

func TestOIDCProvider_Revoke_NoEndpoint(t *testing.T) {
	provider := NewOIDCProvider(newTestConfig("http://localhost"), &OIDCDiscovery{})
	if err := provider.Revoke(context.Background(), newTestToken()); err == nil {
		t.Fatal("expected error when revocation endpoint is missing, got nil")
	}
}
