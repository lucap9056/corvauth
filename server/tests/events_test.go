package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lucap9056/corvauth/server/internal/clientip"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func observeLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, recorded := observer.New(zapcore.DebugLevel)
	previous := logs.Out
	logs.Out = zap.New(core)
	t.Cleanup(func() { logs.Out = previous })
	return recorded
}

func requireEvent(t *testing.T, recorded *observer.ObservedLogs, event events.Event, reason string) map[string]any {
	t.Helper()
	matched := recorded.FilterField(zap.String("event", string(event))).FilterField(zap.String("reason", reason)).All()
	if len(matched) == 0 {
		t.Fatalf("no %s/%s event logged; got %v", event, reason, recorded.All())
	}
	return matched[0].ContextMap()
}

func TestEvents_InvalidState(t *testing.T) {
	recorded := observeLogs(t)
	stub := newOAuthStub("u1", "a@b.com", "A")
	defer stub.Close()
	env := newTestEnv(stub, newMockDB())

	env.do(httptest.NewRequest(http.MethodGet, "/callback?code=testcode&state=unknown-state", nil))

	requireEvent(t, recorded, events.InvalidRequest, events.ReasonStateInvalid)
}

func TestEvents_TrustedHeaderClientIP(t *testing.T) {
	recorded := observeLogs(t)
	stub := newOAuthStub("", "", "")
	defer stub.Close()
	env := newTestEnv(stub, newMockDB())

	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.9, 203.0.113.7")
	logs.Middleware(clientip.NewResolver("X-Forwarded-For"), env.mux).ServeHTTP(httptest.NewRecorder(), req)

	if fields := requireEvent(t, recorded, events.Unauthenticated, events.ReasonTokenMissing); fields["clientIp"] != "203.0.113.7" {
		t.Errorf("clientIp: got %v", fields["clientIp"])
	}
}
