package bearer

import (
	"net/http/httptest"
	"testing"
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
		token, ok := Token(r)
		if token != tt.wantToken || ok != tt.wantOK {
			t.Errorf("Token(%q) = (%q, %v); want (%q, %v)", tt.header, token, ok, tt.wantToken, tt.wantOK)
		}
	}
}
