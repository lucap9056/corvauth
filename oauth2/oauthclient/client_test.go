package oauthclient

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/lucap9056/corvauth/oauth2/providers"
	"golang.org/x/oauth2"
)

func TestValidateIssuer(t *testing.T) {
	const issuer = "https://issuer.example.com"
	tests := []struct {
		name    string
		client  Client
		iss     string
		wantErr bool
	}{
		{"required and matching", Client{issuer: issuer, requireIssuer: true}, issuer, false},
		{"required and missing", Client{issuer: issuer, requireIssuer: true}, "", true},
		{"required and mismatched", Client{issuer: issuer, requireIssuer: true}, "https://evil.example.com", true},
		{"not required and missing", Client{issuer: issuer}, "", false},
		{"not required but mismatched", Client{issuer: issuer}, "https://evil.example.com", true},
		{"no known issuer", Client{}, "https://any.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.ValidateIssuer(tt.iss)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIssuer(%q) error = %v; wantErr %v", tt.iss, err, tt.wantErr)
			}
		})
	}
}

func TestAuthURL_ProviderSpecificParams(t *testing.T) {
	tests := []struct {
		provider       string
		wantAccessType string
	}{
		{providers.GoogleName, "offline"},
		{providers.GitHubName, ""},
		{providers.DiscordName, ""},
		{"generic", ""},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			c := New(Config{Provider: tt.provider, ClientID: "id", AuthURL: "https://example.com/auth"})
			query := authURLQuery(t, c)
			if got := query.Get("access_type"); got != tt.wantAccessType {
				t.Errorf("access_type = %q; want %q", got, tt.wantAccessType)
			}
			if query.Get("prompt") != "" {
				t.Errorf("prompt = %q; want empty", query.Get("prompt"))
			}
			if query.Get("code_challenge") == "" {
				t.Error("missing code_challenge")
			}
		})
	}
}

func TestRedirectURL_DefaultAndOverride(t *testing.T) {
	const defaultURL = "https://default.example/callback"
	const overrideURL = "https://other.example/callback"
	tests := []struct {
		name        string
		redirectURL string
		want        string
	}{
		{"default", "", defaultURL},
		{"override", overrideURL, overrideURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exchangedRedirectURL string
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				exchangedRedirectURL = r.PostForm.Get("redirect_uri")
				w.Header().Set("Content-Type", "application/json")
				json.MarshalWrite(w, map[string]any{"access_token": "token", "token_type": "Bearer"})
			}))
			defer tokenServer.Close()

			c := New(Config{
				Provider:    "generic",
				ClientID:    "id",
				RedirectURL: defaultURL,
				AuthURL:     "https://example.com/auth",
				TokenURL:    tokenServer.URL,
			})

			u, err := url.Parse(c.AuthURL("state", oauth2.GenerateVerifier(), tt.redirectURL))
			if err != nil {
				t.Fatalf("parse AuthURL: %v", err)
			}
			if got := u.Query().Get("redirect_uri"); got != tt.want {
				t.Errorf("AuthURL redirect_uri = %q; want %q", got, tt.want)
			}

			if _, err := c.Exchange(context.Background(), "code", oauth2.GenerateVerifier(), tt.redirectURL); err != nil {
				t.Fatalf("Exchange: %v", err)
			}
			if exchangedRedirectURL != tt.want {
				t.Errorf("Exchange redirect_uri = %q; want %q", exchangedRedirectURL, tt.want)
			}
		})
	}
}

func TestNewOIDC_OfflineAccessAddsPromptConsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.MarshalWrite(w, providers.OIDCDiscovery{
			Issuer:                "http://" + r.Host,
			AuthorizationEndpoint: "http://" + r.Host + "/auth",
			TokenEndpoint:         "http://" + r.Host + "/token",
		})
	}))
	defer srv.Close()

	tests := []struct {
		name       string
		scopes     []string
		wantPrompt string
	}{
		{"with offline_access", []string{"openid", "offline_access"}, "consent"},
		{"without offline_access", []string{"openid", "email"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewOIDC(context.Background(), OIDCConfig{IssuerURL: srv.URL, ClientID: "id", Scopes: tt.scopes})
			if err != nil {
				t.Fatalf("NewOIDC: %v", err)
			}
			query := authURLQuery(t, c)
			if got := query.Get("prompt"); got != tt.wantPrompt {
				t.Errorf("prompt = %q; want %q", got, tt.wantPrompt)
			}
			if query.Get("access_type") != "" {
				t.Errorf("access_type = %q; want empty", query.Get("access_type"))
			}
		})
	}
}

func authURLQuery(t *testing.T, c *Client) url.Values {
	t.Helper()
	u, err := url.Parse(c.AuthURL("state", oauth2.GenerateVerifier(), ""))
	if err != nil {
		t.Fatalf("parse AuthURL: %v", err)
	}
	return u.Query()
}
