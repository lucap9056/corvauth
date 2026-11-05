package integration

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lucap9056/corvauth/server/internal/handlers/response"
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

func TestListSessions(t *testing.T) {
	env := newSeededEnv(t, "list@example.com", "other@example.com")
	env.issueTokens("list@example.com", "laptop")
	_, phoneAccess := env.issueTokens("list@example.com", "phone")
	env.issueTokens("other@example.com", "foreign")

	w := withBearer(env, http.MethodGet, "/sessions", phoneAccess)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertNoStore(t, w)

	var resp struct {
		Message []struct {
			DeviceName string `json:"device_name"`
			Current    bool   `json:"current"`
		} `json:"message"`
	}
	if err := json.UnmarshalRead(w.Body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	current := make(map[string]bool)
	for _, s := range resp.Message {
		current[s.DeviceName] = s.Current
	}
	want := map[string]bool{"laptop": false, "phone": true}
	if len(current) != len(want) || current["laptop"] != want["laptop"] || current["phone"] != want["phone"] {
		t.Errorf("sessions (name → current): want %v, got %v", want, current)
	}
}

func TestDeleteAllSessions(t *testing.T) {
	env := newSeededEnv(t, "logout-all@example.com", "bystander@example.com")
	_, currentAccess := env.issueTokens("logout-all@example.com", "current")
	otherRefresh, otherAccess := env.issueTokens("logout-all@example.com", "other")
	bystanderRefresh, _ := env.issueTokens("bystander@example.com", "bystander")

	w := withBearer(env, http.MethodDelete, "/sessions", currentAccess)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	assertCookieCleared(t, w)

	for name, w := range map[string]*httptest.ResponseRecorder{
		"verify current": verifyWith(env, currentAccess),
		"verify other":   verifyWith(env, otherAccess),
		"refresh other":  refreshAt(env, routeTokens, otherRefresh),
	} {
		if got := w.Header().Get(response.AuthErrorHeader); w.Code != http.StatusUnauthorized || got != response.AuthErrorDeviceNotFound {
			t.Errorf("%s: want 401 %s, got %d %q", name, response.AuthErrorDeviceNotFound, w.Code, got)
		}
	}

	if w := refreshAt(env, routeCurrentSession, bystanderRefresh); w.Code != http.StatusOK {
		t.Errorf("other users' sessions must survive: want 200, got %d", w.Code)
	}
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
