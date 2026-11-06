package e2e

import (
	"context"
	"encoding/json/v2"
	"net"
	"net/http"
	"testing"
)

const middlewareBase = "http://localhost:8080"

func newClient() *http.Client {
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if host == "test-provider" {
				addr = net.JoinHostPort("127.0.0.1", port)
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestFlow(t *testing.T) {
	client := newClient()

	var accessToken string

	// 1. Health
	t.Log("[1/4] health check")
	func() {
		resp, err := client.Get(middlewareBase + "/health")
		if err != nil {
			t.Fatalf("GET /health: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /health: expected 200, got %d", resp.StatusCode)
		}
		t.Logf("GET /health: %d", resp.StatusCode)
	}()

	// 2. Verify unauthenticated
	t.Log("[2/4] verify without token")
	func() {
		resp, err := client.Get(middlewareBase + "/verify")
		if err != nil {
			t.Fatalf("GET /verify: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("GET /verify: expected 401, got %d", resp.StatusCode)
		}
		t.Logf("GET /verify: %d", resp.StatusCode)
	}()

	// 3. Login
	t.Log("[3/4] login")
	func() {
		t.Log("  POST /sessions")
		resp, err := client.Post(middlewareBase+"/sessions", "application/json", nil)
		if err != nil {
			t.Fatalf("POST /sessions: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /sessions: expected 200, got %d", resp.StatusCode)
		}

		var loginBody struct {
			Success bool `json:"success"`
			Message struct {
				URL string `json:"url"`
			} `json:"message"`
		}
		if err := json.UnmarshalRead(resp.Body, &loginBody); err != nil {
			t.Fatalf("POST /sessions: decode: %v", err)
		}
		if !loginBody.Success || loginBody.Message.URL == "" {
			t.Fatalf("POST /sessions: missing url in response")
		}
		t.Logf("  auth URL: %s", loginBody.Message.URL)

		t.Log("  GET auth URL (test-provider /auth)")
		authResp, err := client.Get(loginBody.Message.URL)
		if err != nil {
			t.Fatalf("GET auth URL: %v", err)
		}
		authResp.Body.Close()
		if authResp.StatusCode != http.StatusFound {
			t.Fatalf("GET auth URL: expected 302, got %d", authResp.StatusCode)
		}

		callbackURL := authResp.Header.Get("Location")
		if callbackURL == "" {
			t.Fatalf("GET auth URL: missing Location header")
		}
		t.Logf("  callback URL: %s", callbackURL)

		t.Log("  GET /sessions/callback")
		cbResp, err := client.Get(callbackURL)
		if err != nil {
			t.Fatalf("GET /sessions/callback: %v", err)
		}
		defer cbResp.Body.Close()
		if cbResp.StatusCode != http.StatusOK {
			t.Fatalf("GET /sessions/callback: expected 200, got %d", cbResp.StatusCode)
		}

		var tokenBody struct {
			Success bool `json:"success"`
			Message struct {
				AccessToken string `json:"access_token"`
			} `json:"message"`
		}
		if err := json.UnmarshalRead(cbResp.Body, &tokenBody); err != nil {
			t.Fatalf("GET /sessions/callback: decode: %v", err)
		}
		if !tokenBody.Success || tokenBody.Message.AccessToken == "" {
			t.Fatalf("GET /sessions/callback: missing access_token")
		}

		accessToken = tokenBody.Message.AccessToken
		t.Logf("  access token: %.20s...", accessToken)
	}()

	// 4. Verify authenticated
	t.Log("[4/4] verify with token")
	func() {
		req, _ := http.NewRequest(http.MethodGet, middlewareBase+"/verify", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /verify: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("GET /verify: expected 204, got %d", resp.StatusCode)
		}
		userEmail := resp.Header.Get("X-Corvauth-User-Email")
		if userEmail == "" {
			t.Fatalf("GET /verify: missing X-Corvauth-User-Email header")
		}
		t.Logf("GET /verify: %d  user-email=%s", resp.StatusCode, userEmail)
	}()
}
