package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newSeededEnv(t *testing.T, emails ...string) *testEnv {
	t.Helper()
	stub := newOAuthStub("", "", "")
	t.Cleanup(stub.Close)
	db := newMockDB()
	for _, email := range emails {
		db.seedUser(email)
	}
	return newTestEnv(stub, db)
}

func withBearer(env *testEnv, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return env.do(req)
}

func assertCookieCleared(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" && c.Value == "" {
			return
		}
	}
	t.Error("refresh_token cookie should be cleared")
}

func TestCurrentSession_BearerRefreshToken(t *testing.T) {
	env := newSeededEnv(t, "current-session@example.com")
	refresh, access := env.issueTokens("current-session@example.com", "device")

	if w := withBearer(env, http.MethodGet, "/sessions/current", refresh); w.Code != http.StatusOK {
		t.Errorf("refresh token: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := withBearer(env, http.MethodGet, "/sessions/current", access); w.Code != http.StatusUnauthorized {
		t.Errorf("access token: want 401, got %d", w.Code)
	}
}
