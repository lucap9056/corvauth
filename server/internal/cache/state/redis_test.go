package state

import "testing"

func TestDecodeSession(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want Session
	}{
		{"legacy verifier", "abc-_123", Session{Verifier: "abc-_123"}},
		{"default redirect", `{"verifier":"v1"}`, Session{Verifier: "v1"}},
		{"custom redirect", `{"verifier":"v1","redirect_url":"https://app.example/callback"}`, Session{Verifier: "v1", RedirectURL: "https://app.example/callback"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeSession(tt.val)
			if err != nil {
				t.Fatalf("decodeSession: %v", err)
			}
			if got != tt.want {
				t.Fatalf("decodeSession = %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestDecodeSession_Malformed(t *testing.T) {
	if _, err := decodeSession("{bad"); err == nil {
		t.Fatal("expected error for malformed session")
	}
}
