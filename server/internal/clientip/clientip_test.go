package clientip

import (
	"net/http/httptest"
	"testing"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		values     []string
		remoteAddr string
		wantIP     string
		wantSource Source
	}{
		{"remote addr", "", nil, "[::ffff:203.0.113.7]:80", "203.0.113.7", SourceRemoteAddr},
		{"unix socket peer", "", nil, "@", "", SourceInvalid},
		{"right-most entry of last line", "X-Forwarded-For", []string{"10.0.0.1", "10.0.0.2, 198.51.100.4:8080"}, "", "198.51.100.4", SourceHeader},
		{"header missing", "X-Real-IP", nil, "192.0.2.1:1234", "", SourceMissing},
		{"header invalid", "X-Real-IP", []string{"not-an-ip"}, "192.0.2.1:1234", "", SourceInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remoteAddr
			for _, v := range tc.values {
				req.Header.Add(tc.header, v)
			}
			ip, source := NewResolver(tc.header).Resolve(req)
			if ip != tc.wantIP || source != tc.wantSource {
				t.Errorf("got (%q, %q), want (%q, %q)", ip, source, tc.wantIP, tc.wantSource)
			}
		})
	}
}
