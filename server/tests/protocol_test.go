package integration

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func assertNoStore(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: want %q, got %q", "no-store", got)
	}
}

func callback(env *testEnv, params url.Values) *httptest.ResponseRecorder {
	return env.do(httptest.NewRequest(http.MethodGet, "/callback?"+params.Encode(), nil))
}

func TestCallback_ProviderErrorConsumesState(t *testing.T) {
	tests := []struct {
		providerError string
		wantStatus    int
	}{
		{"access_denied", http.StatusForbidden},
		{"server_error", http.StatusBadGateway},
		{"temporarily_unavailable", http.StatusServiceUnavailable},
		{"invalid_scope", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.providerError, func(t *testing.T) {
			stub := newOAuthStub("u1", "a@b.com", "A")
			defer stub.Close()
			env := newTestEnv(stub, nil)

			stateVal := loginState(t, env)
			w := callback(env, url.Values{"error": {tt.providerError}, "state": {stateVal}})
			if w.Code != tt.wantStatus {
				t.Fatalf("want %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			w = callback(env, url.Values{"code": {"testcode"}, "state": {stateVal}})
			if w.Code != http.StatusBadRequest {
				t.Fatalf("state should be consumed after provider error: want 400, got %d", w.Code)
			}
		})
	}
}

func TestCallback_MissingCodeKeepsState(t *testing.T) {
	stub := newOAuthStub("u1", "a@b.com", "A")
	defer stub.Close()
	env := newTestEnv(stub, nil)

	stateVal := loginState(t, env)
	w := callback(env, url.Values{"state": {stateVal}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}

	w = callback(env, url.Values{"code": {"testcode"}, "state": {stateVal}})
	if w.Code != http.StatusOK {
		t.Fatalf("state should survive a request without code: want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCallback_AuthorizationResponseIssuer(t *testing.T) {
	tests := []struct {
		name       string
		iss        func(stub *oauthStub) string
		wantStatus int
	}{
		{"matching", func(stub *oauthStub) string { return stub.URL }, http.StatusOK},
		{"missing", func(*oauthStub) string { return "" }, http.StatusBadRequest},
		{"mismatched", func(*oauthStub) string { return "https://evil.example.com" }, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newOAuthStub("u1", "a@b.com", "A")
			defer stub.Close()
			env := newOIDCTestEnv(t, stub, nil)

			params := url.Values{"code": {"testcode"}, "state": {loginState(t, env)}}
			if iss := tt.iss(stub); iss != "" {
				params.Set("iss", iss)
			}
			if w := callback(env, params); w.Code != tt.wantStatus {
				t.Fatalf("want %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestUnauthorized_WWWAuthenticate(t *testing.T) {
	const (
		missingChallenge = `Bearer`
		invalidChallenge = `Bearer error="invalid_token"`
		invalidRefresh   = `{"refresh_token":"not.a.valid.jwt"}`
	)
	tests := []struct {
		name          string
		method        string
		path          string
		authorization string
		body          string
		want          string
	}{
		{"verify missing token", http.MethodGet, "/verify", "", "", missingChallenge},
		{"verify invalid token", http.MethodGet, "/verify", "Bearer invalid.token.value", "", invalidChallenge},
		{"delete me missing token", http.MethodDelete, "/users/me", "", "", missingChallenge},
		{"delete me invalid token", http.MethodDelete, "/users/me", "Bearer invalid.token.value", "", invalidChallenge},
		{"refresh missing token", http.MethodPost, "/refresh", "", "", missingChallenge},
		{"refresh invalid token", http.MethodPost, "/refresh", "", invalidRefresh, invalidChallenge},
		{"refresh access missing token", http.MethodPost, "/refresh-access", "", "", missingChallenge},
		{"refresh access invalid token", http.MethodPost, "/refresh-access", "", invalidRefresh, invalidChallenge},
		{"refresh status missing token", http.MethodPost, "/refresh-status", "", "", missingChallenge},
		{"refresh status invalid token", http.MethodPost, "/refresh-status", "", invalidRefresh, invalidChallenge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newOAuthStub("", "", "")
			defer stub.Close()
			env := newTestEnv(stub, newMockDB())

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			w := env.do(req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("want 401, got %d: %s", w.Code, w.Body.String())
			}
			if got := w.Header().Get("WWW-Authenticate"); got != tt.want {
				t.Errorf("WWW-Authenticate: want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestVerify_BearerSchemeCaseInsensitive(t *testing.T) {
	const email = "case@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	_, access := env.issueTokens(email, "device")

	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	req.Header.Set("Authorization", "bEaReR "+access)
	w := env.do(req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}
