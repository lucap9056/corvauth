package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/identity"
)

func TestVerifyOptional(t *testing.T) {
	h := New(nil, nil, jwt.NewJWTManager(nil), nil, identity.DefaultHeaders)

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
		for _, name := range identity.DefaultHeaders.Names() {
			if v := w.Header().Get(name); v != "" {
				t.Errorf("VerifyOptional(%q) set %s = %q; want none", tt.header, name, v)
			}
		}
	}
}
