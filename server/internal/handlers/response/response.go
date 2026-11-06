package response

import (
	"encoding/json/v2"
	"errors"
	"net/http"

	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"go.uber.org/zap"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type Body[T any] struct {
	Success bool `json:"success"`
	Message T    `json:"message"`
}

func JSON[T any](w http.ResponseWriter, success bool, message T, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	json.MarshalWrite(w, Body[T]{
		Success: success,
		Message: message,
	})
}

func NoStoreJSON[T any](w http.ResponseWriter, success bool, message T, code int) {
	w.Header().Set("Cache-Control", "no-store")
	JSON(w, success, message, code)
}

const (
	BearerChallenge       = `Bearer`
	InvalidTokenChallenge = `Bearer error="invalid_token"`
)

const (
	AuthErrorHeader           = "X-Auth-Error"
	AuthErrorDeviceNotFound   = "device_not_found"
	AuthErrorInvalidSignature = "invalid_signature"
	AuthErrorInvalidDeviceTag = "invalid_device_tag"
	AuthErrorUnexpectedAlg    = "unexpected_algorithm"
	AuthErrorInvalidIssuer    = "invalid_issuer"
	AuthErrorSpoofedIdentity  = "spoofed_identity_header"
)

func Fail(w http.ResponseWriter, r *http.Request, event events.Event, reason string, status int, message string, err error, fields ...zap.Field) {
	if event == events.Rejected {
		panic("response: rejected events must go through Reject so X-Auth-Error stays in sync")
	}
	logFailure(r, event, reason, status, err, fields)
	JSON(w, false, message, status)
}

func Reject(w http.ResponseWriter, r *http.Request, reason string, status int, message string, err error, fields ...zap.Field) {
	w.Header().Set(AuthErrorHeader, reason)
	logFailure(r, events.Rejected, reason, status, err, fields)
	JSON(w, false, message, status)
}

func Unauthorized(w http.ResponseWriter, r *http.Request, challenge string, event events.Event, reason, message string, err error, fields ...zap.Field) {
	w.Header().Set("WWW-Authenticate", challenge)
	Fail(w, r, event, reason, http.StatusUnauthorized, message, err, fields...)
}

func AuthFailure(w http.ResponseWriter, r *http.Request, fallbackReason, message string, err error, fields ...zap.Field) {
	w.Header().Set("WWW-Authenticate", InvalidTokenChallenge)
	if reason, ok := authErrorReason(err); ok {
		Reject(w, r, reason, http.StatusUnauthorized, message, err, fields...)
		return
	}
	Fail(w, r, events.Unauthenticated, fallbackReason, http.StatusUnauthorized, message, err, fields...)
}

func authErrorReason(err error) (string, bool) {
	switch {
	case errors.Is(err, jwt.ErrDeviceNotFound):
		return AuthErrorDeviceNotFound, true
	case errors.Is(err, jwt.ErrInvalidSignature):
		return AuthErrorInvalidSignature, true
	case errors.Is(err, jwt.ErrInvalidDeviceTag):
		return AuthErrorInvalidDeviceTag, true
	case errors.Is(err, jwt.ErrUnexpectedSigningMethod):
		return AuthErrorUnexpectedAlg, true
	}
	return "", false
}

func logFailure(r *http.Request, event events.Event, reason string, status int, err error, fields []zap.Field) {
	msg := "request rejected"
	if status >= http.StatusInternalServerError {
		msg = "request failed"
	}
	fields = append(fields, zap.Int("status", status))
	logs.Event(logs.FromContext(r.Context()), event, reason, msg, err, fields...)
}
