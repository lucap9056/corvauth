package login

import (
	"net/http"
	"strings"

	"github.com/ua-parser/uap-go/uaparser"
)

const (
	maxDeviceNameLength = 64
	unknownFamily       = "Other"
)

type deviceNamer struct {
	parser *uaparser.Parser
}

func newDeviceNamer() *deviceNamer {
	parser, err := uaparser.New(uaparser.WithMode(uaparser.EUserAgentLookUpMode | uaparser.EOsLookUpMode))
	if err != nil {
		return &deviceNamer{}
	}
	return &deviceNamer{parser: parser}
}

func knownFamily(family string) string {
	if family == unknownFamily {
		return ""
	}
	return family
}

func truncateDeviceName(name string) string {
	if len(name) <= maxDeviceNameLength {
		return name
	}
	return strings.ToValidUTF8(name[:maxDeviceNameLength], "")
}

func (n *deviceNamer) fromUserAgent(userAgent string) string {
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" || n.parser == nil {
		return DefaultDeviceName
	}

	browser := knownFamily(n.parser.ParseUserAgent(userAgent).Family)
	os := knownFamily(n.parser.ParseOs(userAgent).Family)
	switch {
	case browser != "" && os != "":
		return truncateDeviceName(browser + " on " + os)
	case browser != "":
		return truncateDeviceName(browser)
	case os != "":
		return truncateDeviceName(os)
	}
	return DefaultDeviceName
}

func (n *deviceNamer) resolve(r *http.Request) string {
	if name := strings.TrimSpace(r.Header.Get("X-Device-Name")); name != "" {
		return name
	}
	return n.fromUserAgent(r.UserAgent())
}
