package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lucap9056/corvauth/jwt"
)

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header    string
		wantToken string
		wantOK    bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"BEARER abc", "abc", true},
		{"bEaReR abc", "abc", true},
		{"Basic abc", "", false},
		{"Bearerabc", "", false},
		{"Bearer", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/verify", nil)
		if tt.header != "" {
			r.Header.Set("Authorization", tt.header)
		}
		token, ok := bearerToken(r)
		if token != tt.wantToken || ok != tt.wantOK {
			t.Errorf("bearerToken(%q) = (%q, %v); want (%q, %v)", tt.header, token, ok, tt.wantToken, tt.wantOK)
		}
	}
}

func TestVerifyOptional(t *testing.T) {
	h := New(nil, nil, jwt.NewJWTManager(nil), nil)
	identityHeaders := []string{IdentityHeader, "X-Forwarded-User-Email", "X-Forwarded-Device-ID", "X-Forwarded-Username"}

	tests := []struct {
		header     string
		wantStatus int
	}{
		{"", http.StatusNoContent},
		{"Basic abc", http.StatusNoContent},
		{"Bearer invalid", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/verify-optional", nil)
		if tt.header != "" {
			r.Header.Set("Authorization", tt.header)
		}
		w := httptest.NewRecorder()
		h.VerifyOptional(w, r)

		if w.Code != tt.wantStatus {
			t.Errorf("VerifyOptional(%q) status = %d; want %d", tt.header, w.Code, tt.wantStatus)
		}
		for _, name := range identityHeaders {
			if v := w.Header().Get(name); v != "" {
				t.Errorf("VerifyOptional(%q) set %s = %q; want none", tt.header, name, v)
			}
		}
	}
}
