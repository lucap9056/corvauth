package integration

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/lucap9056/corvauth/server/internal/handlers/options"
)

const otherRedirectURL = "https://app.example/callback"

func loginBody(redirectURI string) io.Reader {
	if redirectURI == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"redirect_uri": redirectURI})
	return strings.NewReader(string(body))
}

func loginQuery(t *testing.T, env *testEnv, body io.Reader) url.Values {
	t.Helper()
	w := env.do(httptest.NewRequest(http.MethodPost, "/sessions", body))
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
	return u.Query()
}

func TestLogin_RedirectURL(t *testing.T) {
	cases := []struct {
		name        string
		redirectURI string
		want        string
	}{
		{"default when omitted", "", testRedirectURL},
		{"default when requested explicitly", testRedirectURL, testRedirectURL},
		{"allowed", otherRedirectURL, otherRedirectURL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newOAuthStub("uid", "user@example.com", "user")
			defer stub.Close()
			env := newTestEnv(stub, nil, options.WithRedirectURLs(testRedirectURL, []string{otherRedirectURL}))

			query := loginQuery(t, env, loginBody(tc.redirectURI))
			if got := query.Get("redirect_uri"); got != tc.want {
				t.Fatalf("auth URL redirect_uri = %q; want %q", got, tc.want)
			}

			w := env.do(httptest.NewRequest(http.MethodGet, "/callback?code=testcode&state="+query.Get("state"), nil))
			if w.Code != http.StatusOK {
				t.Fatalf("callback: want 200, got %d: %s", w.Code, w.Body.String())
			}
			if got := stub.lastExchangeRedirectURI(); got != tc.want {
				t.Errorf("exchange redirect_uri = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestLogin_RedirectURLRejected(t *testing.T) {
	cases := []struct {
		name string
		opt  options.Option
		body io.Reader
	}{
		{"not allowed", options.WithRedirectURLs(testRedirectURL, []string{otherRedirectURL}), loginBody("https://evil.example/callback")},
		{"prefix of allowed", options.WithRedirectURLs(testRedirectURL, []string{otherRedirectURL}), loginBody(otherRedirectURL + "/extra")},
		{"omitted without default", options.WithRedirectURLs("", []string{otherRedirectURL}), nil},
		{"malformed body", options.WithRedirectURLs(testRedirectURL, nil), strings.NewReader("{")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newOAuthStub("", "", "")
			defer stub.Close()
			env := newTestEnv(stub, nil, tc.opt)

			w := env.do(httptest.NewRequest(http.MethodPost, "/sessions", tc.body))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
