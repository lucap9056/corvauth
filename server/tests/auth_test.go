package integration

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/handlers/response"
	"github.com/lucap9056/corvauth/server/internal/identity"
)

func TestHealth(t *testing.T) {
	stub := newOAuthStub("", "", "")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB())
	w := env.do(httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestLogin(t *testing.T) {
	stub := newOAuthStub("", "", "")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB())
	w := env.do(httptest.NewRequest(http.MethodPost, "/sessions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	assertNoStore(t, w)

	var resp struct {
		Success bool `json:"success"`
		Message struct {
			URL string `json:"url"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatal("want success=true")
	}
	if !strings.Contains(resp.Message.URL, stub.URL) {
		t.Errorf("URL %q should point to stub server %s", resp.Message.URL, stub.URL)
	}
	u, err := url.Parse(resp.Message.URL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if u.Query().Get("state") == "" {
		t.Error("URL should contain a non-empty state parameter")
	}
	if u.Query().Get("code_challenge") == "" {
		t.Error("URL should contain a non-empty code_challenge parameter")
	}
}

// loginState calls POST /sessions and extracts the state parameter from the returned URL.
func loginState(t *testing.T, env *testEnv) string {
	t.Helper()
	w := env.do(httptest.NewRequest(http.MethodPost, "/sessions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Message struct {
			URL string `json:"url"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	u, err := url.Parse(resp.Message.URL)
	if err != nil {
		t.Fatalf("parse login URL: %v", err)
	}
	s := u.Query().Get("state")
	if s == "" {
		t.Fatal("login URL missing state parameter")
	}
	return s
}

// TestCallback_ExistingUser verifies the full OAuth2 callback for a known user:
// code exchange → userinfo fetch → DB lookup → JWT issuance → cookie set.
func TestCallback_ExistingUser(t *testing.T) {
	const (
		email    = "existing@example.com"
		username = "existinguser"
	)

	stub := newOAuthStub("oauth-uid", email, username)
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	stateVal := loginState(t, env)
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertNoStore(t, w)

	var resp struct {
		Success bool `json:"success"`
		Message struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatal("want success=true")
	}
	if resp.Message.AccessToken == "" || resp.Message.RefreshToken == "" {
		t.Error("want non-empty access and refresh tokens")
	}

	var cookieSet bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" && c.Value == resp.Message.RefreshToken {
			cookieSet = true
		}
	}
	if !cookieSet {
		t.Error("refresh_token cookie not set or value mismatch")
	}
}

func TestCallback_RegistrationDisabled(t *testing.T) {
	stub := newOAuthStub("u1", "new@example.com", "New User")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB()) // AllowRegistration defaults to false

	stateVal := loginState(t, env)
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCallback_RegistrationEnabled(t *testing.T) {
	const email = "newuser@example.com"

	stub := newOAuthStub("u1", email, "New User")
	defer stub.Close()

	db := newMockDB()
	env := newTestEnv(stub, db, options.WithAllowRegistration(true))

	stateVal := loginState(t, env)
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatal("want success=true")
	}

	if !db.hasUser(email) {
		t.Error("user should have been created in DB")
	}
}

func TestCallback_MissingState(t *testing.T) {
	stub := newOAuthStub("u1", "a@b.com", "A")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB())

	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode", nil) // no state
	w := env.do(req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCallback_InvalidState(t *testing.T) {
	stub := newOAuthStub("u1", "a@b.com", "A")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB())

	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state=unknown-state", nil)
	w := env.do(req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCallback_StateReplay(t *testing.T) {
	stub := newOAuthStub("u1", "a@b.com", "A")
	defer stub.Close()

	env := newTestEnv(stub, nil)

	stateVal := loginState(t, env)

	// First use succeeds
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("first callback: want 200, got %d: %s", w.Code, w.Body.String())
	}

	// Replay with the same state must fail
	req2 := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w2 := env.do(req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("replay callback: want 400, got %d", w2.Code)
	}
}

// TestCallback_NoDB_OAuthTokens verifies that without a DB the raw OAuth tokens
// are returned in the response body.
func TestCallback_NoDB_OAuthTokens(t *testing.T) {
	stub := newOAuthStub("u1", "a@b.com", "A")
	defer stub.Close()

	env := newTestEnv(stub, nil)

	stateVal := loginState(t, env)
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertNoStore(t, w)

	var resp struct {
		Success bool `json:"success"`
		Message struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatal("want success=true")
	}
	if resp.Message.AccessToken != "stub-access-token" || resp.Message.RefreshToken != "stub-refresh-token" {
		t.Errorf("expected stub oauth tokens, got access=%q refresh=%q",
			resp.Message.AccessToken, resp.Message.RefreshToken)
	}
}

// TestCallback_NoDB_PassOAuthToken verifies that with PassOAuthToken=true the
// OAuth tokens are forwarded via response headers instead of the body.
func TestCallback_NoDB_PassOAuthToken(t *testing.T) {
	stub := newOAuthStub("u1", "a@b.com", "A")
	defer stub.Close()

	env := newTestEnv(stub, nil, options.WithPassOAuthToken(true))

	stateVal := loginState(t, env)
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertNoStore(t, w)
	headers := options.DefaultProviderTokenHeaders
	if got := w.Header().Get(headers.AccessToken); got != "stub-access-token" {
		t.Errorf("%s: want %q, got %q", headers.AccessToken, "stub-access-token", got)
	}
	if got := w.Header().Get(headers.RefreshToken); got != "stub-refresh-token" {
		t.Errorf("%s: want %q, got %q", headers.RefreshToken, "stub-refresh-token", got)
	}
}

func TestRefresh_WithBody(t *testing.T) {
	const email = "user1@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")

	body, _ := json.Marshal(map[string]string{"refresh_token": refresh})
	req := httptest.NewRequest(http.MethodPost, "/sessions/current/tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertNoStore(t, w)

	var resp struct {
		Success bool `json:"success"`
		Message struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatal("want success=true")
	}
	if resp.Message.AccessToken == "" || resp.Message.RefreshToken == "" {
		t.Error("want non-empty rotated token pair")
	}
	if resp.Message.RefreshToken == refresh {
		t.Error("refresh token should have been rotated")
	}
}

func TestRefresh_WithCookie(t *testing.T) {
	const email = "user2@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")

	req := httptest.NewRequest(http.MethodPost, "/sessions/current/tokens", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatal("want success=true")
	}
}

func refreshWithCookie(env *testEnv, refreshToken string) (int, string) {
	req := httptest.NewRequest(http.MethodPost, "/sessions/current/tokens", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	w := env.do(req)

	var resp struct {
		Message struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"message"`
	}
	json.UnmarshalRead(w.Body, &resp)
	return w.Code, resp.Message.RefreshToken
}

func TestRefresh_ConcurrentSameTokenSharesRotation(t *testing.T) {
	const email = "concurrent@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")

	const n = 10
	codes := make([]int, n)
	rotated := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			codes[i], rotated[i] = refreshWithCookie(env, refresh)
		})
	}
	wg.Wait()

	for i := range n {
		if codes[i] != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i, codes[i])
		}
		if rotated[i] != rotated[0] {
			t.Fatalf("request %d received a different rotated token; concurrent refreshes must share one rotation", i)
		}
	}

	if code, _ := refreshWithCookie(env, refresh); code != http.StatusOK {
		t.Errorf("retry with the original token inside the grace window: want 200, got %d", code)
	}

	if code, _ := refreshWithCookie(env, rotated[0]); code != http.StatusOK {
		t.Errorf("rotated token must stay valid after concurrent refreshes: want 200, got %d", code)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	stub := newOAuthStub("", "", "")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB())

	body, _ := json.Marshal(map[string]string{"refresh_token": "not.a.valid.jwt"})
	req := httptest.NewRequest(http.MethodPost, "/sessions/current/tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := env.do(req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestRefreshAccess(t *testing.T) {
	const email = "user3@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")

	body, _ := json.Marshal(map[string]string{"refresh_token": refresh})
	req := httptest.NewRequest(http.MethodPost, "/sessions/current/access-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertNoStore(t, w)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Message == "" {
		t.Error("want success=true and a new access token in message")
	}
}

func TestRefreshStatus(t *testing.T) {
	const email = "status@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")
	claims, err := env.jwtManager.VerifyRefresh(refresh)
	if err != nil {
		t.Fatalf("VerifyRefresh: %v", err)
	}

	w := refreshAt(env, routeCurrentSession, refresh)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertNoStore(t, w)
	if cookies := w.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("want no cookies, got %v", cookies)
	}

	var resp struct {
		Success bool `json:"success"`
		Message struct {
			DeviceID  string `json:"device_id"`
			IssuedAt  int64  `json:"issued_at"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("want success=true")
	}
	if resp.Message.DeviceID != claims.DeviceID {
		t.Errorf("device_id: want %q, got %q", claims.DeviceID, resp.Message.DeviceID)
	}
	if resp.Message.IssuedAt != claims.IssuedAt.Unix() {
		t.Errorf("issued_at: want %d, got %d", claims.IssuedAt.Unix(), resp.Message.IssuedAt)
	}
	if resp.Message.ExpiresAt != claims.ExpiresAt.Unix() {
		t.Errorf("expires_at: want %d, got %d", claims.ExpiresAt.Unix(), resp.Message.ExpiresAt)
	}

	if _, err := env.jwtManager.VerifyRefresh(refresh); err != nil {
		t.Errorf("refresh token must not be rotated by GET /sessions/current: %v", err)
	}
}

func TestVerify_Valid(t *testing.T) {
	const email = "user4@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	_, access := env.issueTokens(email, "device")

	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := env.do(req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get(identity.DefaultHeaders.UserEmail); got != email {
		t.Errorf("%s: want %q, got %q", identity.DefaultHeaders.UserEmail, email, got)
	}
}

func TestVerify_MissingToken(t *testing.T) {
	stub := newOAuthStub("", "", "")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB())

	w := env.do(httptest.NewRequest(http.MethodGet, "/verify", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestVerify_InvalidToken(t *testing.T) {
	stub := newOAuthStub("", "", "")
	defer stub.Close()

	env := newTestEnv(stub, newMockDB())

	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.value")
	w := env.do(req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestLogout(t *testing.T) {
	const email = "user5@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")

	req := httptest.NewRequest(http.MethodDelete, "/sessions/current", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	assertCookieCleared(t, w)
}

func TestDeleteMe(t *testing.T) {
	const email = "user6@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	_, access := env.issueTokens(email, "device")

	req := httptest.NewRequest(http.MethodDelete, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	if db.hasUser(email) {
		t.Error("user should have been deleted from DB")
	}
}

func TestCallback_ExternalUsers_RegistrationIgnored(t *testing.T) {
	const email = "unknown@example.com"

	stub := newOAuthStub("u1", email, "Unknown User")
	defer stub.Close()

	db := newMockDB()
	db.externalUsers = true
	env := newTestEnv(stub, db, options.WithAllowRegistration(true))

	stateVal := loginState(t, env)
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", w.Code, w.Body.String())
	}
	if db.hasUser(email) {
		t.Error("user must not be created when the users table is external")
	}
}

func TestCallback_ExternalUsers_ExistingUser(t *testing.T) {
	const email = "external@example.com"

	stub := newOAuthStub("u1", email, "External User")
	defer stub.Close()

	db := newMockDB()
	db.externalUsers = true
	db.seedUser(email)
	env := newTestEnv(stub, db)

	stateVal := loginState(t, env)
	req := httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil)
	w := env.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteMe_ExternalUsers_NotRegistered(t *testing.T) {
	const email = "external-delete@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.externalUsers = true
	db.seedUser(email)
	env := newTestEnv(stub, db)

	_, access := env.issueTokens(email, "device")

	req := httptest.NewRequest(http.MethodDelete, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := env.do(req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", w.Code, w.Body.String())
	}
	if !db.hasUser(email) {
		t.Error("user must not be deleted when the users table is external")
	}
}

func accessUsername(t *testing.T, env *testEnv, accessToken string) string {
	t.Helper()
	claims, err := env.jwtManager.VerifyAccess(accessToken)
	if err != nil {
		t.Fatalf("VerifyAccess: %v", err)
	}
	return claims.Username
}

func TestCallback_UsernameFromUsersTable(t *testing.T) {
	const email = "named@example.com"

	stub := newOAuthStub("u1", email, "Provider Name")
	defer stub.Close()

	db := newMockDB()
	db.seedUsername(email, "Stored Name")
	env := newTestEnv(stub, db)

	stateVal := loginState(t, env)
	w := env.do(httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Message struct {
			AccessToken string `json:"access_token"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := accessUsername(t, env, resp.Message.AccessToken); got != "Stored Name" {
		t.Errorf("username claim: want %q, got %q", "Stored Name", got)
	}
}

func TestRefresh_UsernameFromUsersTable(t *testing.T) {
	const email = "renamed@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUsername(email, "Old Name")
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")
	db.seedUsername(email, "New Name")

	req := httptest.NewRequest(http.MethodPost, "/sessions/current/tokens", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	w := env.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Message struct {
			AccessToken string `json:"access_token"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := accessUsername(t, env, resp.Message.AccessToken); got != "New Name" {
		t.Errorf("username claim: want %q, got %q", "New Name", got)
	}
}

func TestRefresh_UnknownUserKeepsSession(t *testing.T) {
	const email = "vanished@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")
	db.mu.Lock()
	delete(db.users, email)
	db.mu.Unlock()

	if code, _ := refreshWithCookie(env, refresh); code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", code)
	}
	if _, err := env.jwtManager.VerifyRefresh(refresh); err != nil {
		t.Errorf("refresh token must not be rotated when the username lookup fails: %v", err)
	}
}

func TestRefreshAccess_UsernameFromUsersTable(t *testing.T) {
	const email = "access-name@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUsername(email, "Access Name")
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")

	body, _ := json.Marshal(map[string]string{"refresh_token": refresh})
	req := httptest.NewRequest(http.MethodPost, "/sessions/current/access-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := env.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Message string `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := accessUsername(t, env, resp.Message); got != "Access Name" {
		t.Errorf("username claim: want %q, got %q", "Access Name", got)
	}
}

func TestVerify_ForwardsUsername(t *testing.T) {
	const email = "verify-name@example.com"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	refresh, _ := env.issueTokens(email, "device")
	access, err := env.jwtManager.GenerateAccess(refresh, "Verify Name")
	if err != nil {
		t.Fatalf("GenerateAccess: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := env.do(req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get(identity.DefaultHeaders.Username); got != "Verify Name" {
		t.Errorf("%s: want %q, got %q", identity.DefaultHeaders.Username, "Verify Name", got)
	}
}

func TestVerify_IdentityToken(t *testing.T) {
	const email = "identity@example.com"
	const secret = "0123456789abcdef0123456789abcdef"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnvWithIdentity(stub, db, nil, identity.NewSigner(secret, "", ""))

	refresh, _ := env.issueTokens(email, "device")
	access, err := env.jwtManager.GenerateAccess(refresh, "Identity Name")
	if err != nil {
		t.Fatalf("GenerateAccess: %v", err)
	}
	accessClaims, err := env.jwtManager.VerifyAccess(access)
	if err != nil {
		t.Fatalf("VerifyAccess: %v", err)
	}

	w := verifyWith(env, access)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", w.Code, w.Body.String())
	}
	for _, header := range []string{identity.DefaultHeaders.UserEmail, identity.DefaultHeaders.Username, identity.DefaultHeaders.DeviceID} {
		if got := w.Header().Get(header); got != "" {
			t.Errorf("%s: want empty, got %q", header, got)
		}
	}

	claims := &identity.Claims{}
	token, err := gojwt.ParseWithClaims(w.Header().Get(identity.DefaultHeaders.Identity), claims, func(*gojwt.Token) (any, error) {
		return []byte(secret), nil
	}, gojwt.WithValidMethods([]string{gojwt.SigningMethodHS256.Alg()}))
	if err != nil {
		t.Fatalf("parse identity token: %v", err)
	}
	if typ := token.Header["typ"]; typ != identity.TokenType {
		t.Errorf("typ: got %v, want %q", typ, identity.TokenType)
	}
	if claims.Subject != email || claims.Username != "Identity Name" || claims.DeviceID != accessClaims.DeviceID {
		t.Errorf("claims: got %+v", claims)
	}
}

func TestVerify_ForwardedIdentityHeaders(t *testing.T) {
	const email = "forwarded@example.com"
	headers := identity.DefaultHeaders

	tests := []struct {
		name       string
		signer     *identity.Signer
		forward    func(first *httptest.ResponseRecorder, req *http.Request)
		wantStatus int
	}{
		{"plain matching", nil, func(first *httptest.ResponseRecorder, req *http.Request) {
			req.Header.Set(headers.UserEmail, first.Header().Get(headers.UserEmail))
			req.Header.Set(headers.DeviceID, first.Header().Get(headers.DeviceID))
		}, http.StatusNoContent},
		{"plain mismatched", nil, func(_ *httptest.ResponseRecorder, req *http.Request) {
			req.Header.Set(headers.UserEmail, "victim@example.com")
		}, http.StatusForbidden},
		{"signed matching", identity.NewSigner("0123456789abcdef0123456789abcdef", "", ""), func(first *httptest.ResponseRecorder, req *http.Request) {
			req.Header.Set(headers.Identity, first.Header().Get(headers.Identity))
		}, http.StatusNoContent},
		{"signed with plain header", identity.NewSigner("0123456789abcdef0123456789abcdef", "", ""), func(_ *httptest.ResponseRecorder, req *http.Request) {
			req.Header.Set(headers.UserEmail, email)
		}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newOAuthStub("", "", "")
			defer stub.Close()
			db := newMockDB()
			db.seedUser(email)
			env := newTestEnvWithIdentity(stub, db, nil, tt.signer)
			_, access := env.issueTokens(email, "device")

			req := httptest.NewRequest(http.MethodGet, "/verify", nil)
			req.Header.Set("Authorization", "Bearer "+access)
			tt.forward(verifyWith(env, access), req)

			w := env.do(req)
			if w.Code != tt.wantStatus {
				t.Fatalf("want %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantStatus == http.StatusForbidden && w.Header().Get(response.AuthErrorHeader) != response.AuthErrorSpoofedIdentity {
				t.Errorf("%s: want %q, got %q", response.AuthErrorHeader, response.AuthErrorSpoofedIdentity, w.Header().Get(response.AuthErrorHeader))
			}
		})
	}
}

func TestCallback_UsernameLookupFailureCreatesNoDevice(t *testing.T) {
	const email = "lookup-fail@example.com"

	stub := newOAuthStub("u1", email, "Lookup Fail")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	db.usernameErr = errors.New("connection reset")
	env := newTestEnv(stub, db)

	stateVal := loginState(t, env)
	w := env.do(httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
	}
	if n := db.deviceCount(); n != 0 {
		t.Errorf("no device session should be created, got %d", n)
	}
}

func TestCallback_RegistrationUsesProviderName(t *testing.T) {
	const email = "register-name@example.com"

	stub := newOAuthStub("u1", email, "Provider Name")
	defer stub.Close()

	db := newMockDB()
	env := newTestEnv(stub, db, options.WithAllowRegistration(true))

	stateVal := loginState(t, env)
	w := env.do(httptest.NewRequest(http.MethodGet, "/sessions/callback?code=testcode&state="+stateVal, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Message struct {
			AccessToken string `json:"access_token"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := accessUsername(t, env, resp.Message.AccessToken); got != "Provider Name" {
		t.Errorf("username claim: want %q, got %q", "Provider Name", got)
	}
}

const (
	routeTokens         = "POST /sessions/current/tokens"
	routeAccessToken    = "POST /sessions/current/access-token"
	routeCurrentSession = "GET /sessions/current"
)

func refreshAt(env *testEnv, route, token string) *httptest.ResponseRecorder {
	method, path, _ := strings.Cut(route, " ")
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: token})
	return env.do(req)
}

func verifyWith(env *testEnv, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return env.do(req)
}

func TestAuthErrorHeader(t *testing.T) {
	const email = "header@example.com"

	endpoints := map[string]func(env *testEnv, refreshToken, accessToken string) *httptest.ResponseRecorder{
		routeTokens:      func(env *testEnv, r, _ string) *httptest.ResponseRecorder { return refreshAt(env, routeTokens, r) },
		routeAccessToken: func(env *testEnv, r, _ string) *httptest.ResponseRecorder { return refreshAt(env, routeAccessToken, r) },
		routeCurrentSession: func(env *testEnv, r, _ string) *httptest.ResponseRecorder {
			return refreshAt(env, routeCurrentSession, r)
		},
		"GET /verify": func(env *testEnv, _, a string) *httptest.ResponseRecorder { return verifyWith(env, a) },
	}
	cases := []struct {
		name   string
		tamper func(db *mockDB)
		want   string
	}{
		{"device not found", func(db *mockDB) { clear(db.devices) }, response.AuthErrorDeviceNotFound},
		{"invalid signature", func(db *mockDB) {
			for _, d := range db.devices {
				d.secret = "rotated-secret"
			}
		}, response.AuthErrorInvalidSignature},
	}

	for path, call := range endpoints {
		for _, tc := range cases {
			t.Run(path+" "+tc.name, func(t *testing.T) {
				stub := newOAuthStub("", "", "")
				defer stub.Close()

				db := newMockDB()
				db.seedUser(email)
				env := newTestEnv(stub, db)

				refreshToken, accessToken := env.issueTokens(email, "device")
				tc.tamper(db)

				w := call(env, refreshToken, accessToken)
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("want 401, got %d: %s", w.Code, w.Body.String())
				}
				if got := w.Header().Get(response.AuthErrorHeader); got != tc.want {
					t.Errorf("%s: want %q, got %q", response.AuthErrorHeader, tc.want, got)
				}
			})
		}
	}
}

func TestAuthErrorHeader_UnexpectedAlgorithm(t *testing.T) {
	stub := newOAuthStub("", "", "")
	defer stub.Close()
	env := newTestEnv(stub, newMockDB())

	token := gojwt.NewWithClaims(gojwt.SigningMethodNone, gojwt.MapClaims{"device_id": "forged"})
	token.Header["typ"] = jwt.AccessTokenType
	unsigned, _ := token.SignedString(gojwt.UnsafeAllowNoneSignatureType)

	w := verifyWith(env, unsigned)
	if got := w.Header().Get(response.AuthErrorHeader); w.Code != http.StatusUnauthorized || got != response.AuthErrorUnexpectedAlg {
		t.Errorf("want 401 %q, got %d %q", response.AuthErrorUnexpectedAlg, w.Code, got)
	}
}

func TestAuthErrorHeader_InvalidDeviceTag(t *testing.T) {
	const email = "tag@example.com"
	const secret = "test-device-secret"

	stub := newOAuthStub("", "", "")
	defer stub.Close()

	db := newMockDB()
	db.seedUser(email)
	env := newTestEnv(stub, db)

	otherServer := jwt.NewJWTManager(db, jwt.WithDeviceTagSecret("other-device-tag-secret-32-bytes!"))
	deviceID, _ := db.SaveDeviceSecret(email, "device", secret)
	_, generation, _ := db.GetDeviceSecret(deviceID)
	refreshToken, _ := otherServer.GenerateRefresh(email, deviceID, secret, generation)
	accessToken, _ := otherServer.GenerateAccess(refreshToken, "")

	responses := map[string]*httptest.ResponseRecorder{
		routeTokens:         refreshAt(env, routeTokens, refreshToken),
		routeAccessToken:    refreshAt(env, routeAccessToken, refreshToken),
		routeCurrentSession: refreshAt(env, routeCurrentSession, refreshToken),
		"GET /verify":       verifyWith(env, accessToken),
	}
	for route, w := range responses {
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: want 401, got %d: %s", route, w.Code, w.Body.String())
		}
		if got := w.Header().Get(response.AuthErrorHeader); got != response.AuthErrorInvalidDeviceTag {
			t.Errorf("%s: %s want %q, got %q", route, response.AuthErrorHeader, response.AuthErrorInvalidDeviceTag, got)
		}
	}
	if _, exists := db.devices[deviceID]; !exists {
		t.Error("device must not be deleted when the device tag does not match")
	}
}

func TestRefresh_RevokedTokenDeletesDevice(t *testing.T) {
	const email = "reuse@example.com"
	const secret = "test-device-secret"

	for _, route := range []string{routeTokens, routeAccessToken, routeCurrentSession} {
		t.Run(route, func(t *testing.T) {
			recorded := observeLogs(t)
			stub := newOAuthStub("", "", "")
			defer stub.Close()

			db := newMockDB()
			db.seedUser(email)
			env := newTestEnv(stub, db)

			deviceID, _ := db.SaveDeviceSecret(email, "device", secret)
			staleToken, _ := env.jwtManager.GenerateRefresh(email, deviceID, secret, 1)
			db.UpdateDeviceSecret(deviceID)

			w := refreshAt(env, route, staleToken)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("want 401, got %d: %s", w.Code, w.Body.String())
			}
			if got := w.Header().Get(response.AuthErrorHeader); got != "" {
				t.Errorf("%s: want empty, got %q", response.AuthErrorHeader, got)
			}
			if _, exists := db.devices[deviceID]; exists {
				t.Error("device must be deleted after a revoked refresh token is reused")
			}
			if fields := requireEvent(t, recorded, events.Unauthenticated, events.ReasonRefreshTokenReused); fields["deviceId"] != deviceID {
				t.Errorf("deviceId: got %v, want %q", fields["deviceId"], deviceID)
			}
		})
	}
}
