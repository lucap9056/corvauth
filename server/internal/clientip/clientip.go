package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"strings"
)

type Source string

const (
	SourceRemoteAddr Source = "remote_addr"
	SourceHeader     Source = "header"
	SourceMissing    Source = "missing"
	SourceInvalid    Source = "invalid"
)

type Resolver struct {
	header string
}

func NewResolver(trustedHeader string) Resolver {
	if trustedHeader == "" {
		return Resolver{}
	}
	return Resolver{header: textproto.CanonicalMIMEHeaderKey(trustedHeader)}
}

func (r Resolver) TrustsHeader() bool {
	return r.header != ""
}

func (r Resolver) Resolve(req *http.Request) (string, Source) {
	if r.header == "" {
		return fromRemoteAddr(req.RemoteAddr)
	}
	return fromHeader(req.Header.Values(r.header))
}

func fromRemoteAddr(remoteAddr string) (string, Source) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if ip, ok := parse(host); ok {
		return ip, SourceRemoteAddr
	}
	return "", SourceInvalid
}

func fromHeader(values []string) (string, Source) {
	if len(values) == 0 {
		return "", SourceMissing
	}
	last := values[len(values)-1]
	if i := strings.LastIndexByte(last, ','); i >= 0 {
		last = last[i+1:]
	}
	last = strings.TrimSpace(last)
	if last == "" {
		return "", SourceMissing
	}
	if ip, ok := parse(last); ok {
		return ip, SourceHeader
	}
	return "", SourceInvalid
}

func parse(raw string) (string, bool) {
	addr, err := netip.ParseAddr(strings.Trim(raw, "[]"))
	if err != nil {
		addrPort, portErr := netip.ParseAddrPort(raw)
		if portErr != nil {
			return "", false
		}
		addr = addrPort.Addr()
	}
	return addr.Unmap().WithZone("").String(), true
}
