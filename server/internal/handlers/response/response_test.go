package response

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAuthFailure(t *testing.T) {
	cases := []struct {
		err        error
		wantHeader string
		wantEvent  events.Event
		wantReason string
	}{
		{jwt.ErrInvalidSignature, AuthErrorInvalidSignature, events.Rejected, AuthErrorInvalidSignature},
		{jwt.ErrTokenRevoked, "", events.Unauthenticated, events.ReasonTokenInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			core, recorded := observer.New(zapcore.DebugLevel)
			r := httptest.NewRequest(http.MethodGet, "/verify", nil)
			r = r.WithContext(logs.WithLogger(r.Context(), zap.New(core)))
			w := httptest.NewRecorder()

			AuthFailure(w, r, events.ReasonTokenInvalid, "", tc.err)

			fields := recorded.All()[0].ContextMap()
			if w.Header().Get(AuthErrorHeader) != tc.wantHeader || fields["event"] != string(tc.wantEvent) || fields["reason"] != tc.wantReason {
				t.Errorf("got header=%q event=%v reason=%v", w.Header().Get(AuthErrorHeader), fields["event"], fields["reason"])
			}
		})
	}
}

func TestFail_RejectsRejectedEvent(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want panic when Fail is used with the rejected event")
		}
	}()
	Fail(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), events.Rejected, "", http.StatusUnauthorized, "", nil)
}
