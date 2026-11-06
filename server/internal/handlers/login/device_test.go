package login

import (
	"net/http/httptest"
	"testing"
)

func TestResolveDeviceName(t *testing.T) {
	const firefoxOnLinux = "Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0"
	namer := newDeviceNamer()

	cases := []struct {
		name, header, userAgent, want string
	}{
		{"header wins", "My Laptop", firefoxOnLinux, "My Laptop"},
		{"from user agent", "", firefoxOnLinux, "Firefox on Linux"},
		{"blank header falls back", "  ", firefoxOnLinux, "Firefox on Linux"},
		{"unrecognized user agent", "", "???", DefaultDeviceName},
		{"no user agent", "", "", DefaultDeviceName},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/sessions/callback", nil)
		r.Header.Set("X-Device-Name", c.header)
		r.Header.Set("User-Agent", c.userAgent)
		if got := namer.resolve(r); got != c.want {
			t.Errorf("%s: want %q, got %q", c.name, c.want, got)
		}
	}
}
