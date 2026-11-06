package refresh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/flight"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/handlers/refreshtoken"
	"github.com/lucap9056/corvauth/server/internal/handlers/response"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"go.uber.org/zap"
)

const (
	flightKeyPrefix = "refresh:"
	rotateTimeout   = 10 * time.Second
)

const (
	msgInvalidRefreshToken = "Invalid refresh token"
	msgInvalidSession      = "Invalid session or expired refresh token"
	msgUserNotFound        = "User not found"
)

type rotateError struct {
	event   events.Event
	reason  string
	message string
	status  int
	err     error
	fields  []zap.Field
}

func (e *rotateError) Error() string {
	return fmt.Sprintf("%s: %v", e.message, e.err)
}

func (e *rotateError) Unwrap() error {
	return e.err
}

func (e *rotateError) respond(w http.ResponseWriter, r *http.Request) {
	switch {
	case e.event == events.Unauthenticated:
		response.AuthFailure(w, r, e.reason, e.message, e.err, e.fields...)
	case e.status == http.StatusUnauthorized:
		response.Unauthorized(w, r, response.InvalidTokenChallenge, e.event, e.reason, e.message, e.err, e.fields...)
	default:
		response.Fail(w, r, e.event, e.reason, e.status, e.message, e.err, e.fields...)
	}
}

type Handler struct {
	db           options.DB
	users        options.UsersDB
	jwtManager   *jwt.JWTManager
	flight       *flight.Group
	secureCookie bool
}

func New(db options.DB, users options.UsersDB, jwtManager *jwt.JWTManager, flightGroup *flight.Group, secureCookie bool) *Handler {
	return &Handler{
		db:           db,
		users:        users,
		jwtManager:   jwtManager,
		flight:       flightGroup,
		secureCookie: secureCookie,
	}
}

func missingRefreshToken(w http.ResponseWriter, r *http.Request, err error) {
	response.Unauthorized(w, r, response.BearerChallenge, events.Unauthenticated, events.ReasonRefreshTokenMissing, msgInvalidRefreshToken, err)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := refreshtoken.FromRequest(r)
	if err != nil {
		missingRefreshToken(w, r, err)
		return
	}

	// The rotation is shared with concurrent callers, so one client disconnecting must not abort it for the rest.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), rotateTimeout)
	defer cancel()

	tokens, err := flight.Do(ctx, h.flight, flightKeyPrefix+refreshToken, func(ctx context.Context) (response.TokenPair, error) {
		return h.rotate(ctx, refreshToken)
	})
	if err != nil {
		var rotateErr *rotateError
		if errors.As(err, &rotateErr) {
			rotateErr.respond(w, r)
			return
		}
		response.Fail(w, r, events.InternalError, events.ReasonRefreshRotateFailed, http.StatusInternalServerError, "Failed to rotate refresh token", err)
		return
	}

	refreshtoken.SetCookie(w, tokens.RefreshToken, h.secureCookie)
	response.NoStoreJSON(w, true, tokens, http.StatusOK)
}

func invalidRefreshReason(err error) string {
	if errors.Is(err, jwt.ErrTokenRevoked) {
		return events.ReasonRefreshTokenReused
	}
	return events.ReasonRefreshTokenInvalid
}

func deviceFields(claims *jwt.RefreshClaims) []zap.Field {
	if claims == nil || claims.DeviceID == "" {
		return nil
	}
	return []zap.Field{zap.String("deviceId", claims.DeviceID)}
}

func (h *Handler) invalidSession(ctx context.Context, claims *jwt.RefreshClaims, err error) *rotateError {
	h.deleteDeviceOnReuse(ctx, claims, err)
	return &rotateError{events.Unauthenticated, invalidRefreshReason(err), msgInvalidSession, http.StatusUnauthorized, err, deviceFields(claims)}
}

func (h *Handler) deleteDeviceOnReuse(ctx context.Context, claims *jwt.RefreshClaims, err error) {
	if !errors.Is(err, jwt.ErrTokenRevoked) {
		return
	}
	if delErr := h.db.DeleteDevice(claims.Subject, claims.DeviceID); delErr != nil {
		logs.Event(logs.FromContext(ctx), events.InternalError, events.ReasonDeviceDeleteFailed,
			"failed to delete device after refresh token reuse", delErr, deviceFields(claims)...)
	}
}

func (h *Handler) rotate(ctx context.Context, refreshToken string) (response.TokenPair, error) {
	claims, err := h.jwtManager.VerifyRefresh(refreshToken)
	if err != nil {
		return response.TokenPair{}, h.invalidSession(ctx, claims, err)
	}

	username, err := h.users.GetUsername(claims.Subject)
	if errors.Is(err, database.ErrUserNotFound) {
		return response.TokenPair{}, &rotateError{events.Denied, events.ReasonUserNotFound, msgUserNotFound, http.StatusUnauthorized, err, deviceFields(claims)}
	}
	if err != nil {
		return response.TokenPair{}, &rotateError{events.InternalError, events.ReasonUserLookupFailed, "Failed to fetch username", http.StatusInternalServerError, err, nil}
	}

	newRefreshToken, rotatedClaims, err := h.jwtManager.RotateRefresh(refreshToken)
	if errors.Is(err, jwt.ErrInvalidToken) {
		return response.TokenPair{}, h.invalidSession(ctx, rotatedClaims, err)
	}
	if err != nil {
		return response.TokenPair{}, &rotateError{events.InternalError, events.ReasonRefreshRotateFailed, "Failed to rotate refresh token", http.StatusInternalServerError, err, nil}
	}

	accessToken, err := h.jwtManager.GenerateAccess(newRefreshToken, username)
	if err != nil {
		return response.TokenPair{}, &rotateError{events.InternalError, events.ReasonTokenIssueFailed, "Access token generation failed", http.StatusInternalServerError, err, nil}
	}

	return response.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (h *Handler) RefreshAccess(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := refreshtoken.FromRequest(r)
	if err != nil {
		missingRefreshToken(w, r, err)
		return
	}

	claims, err := h.jwtManager.VerifyRefresh(refreshToken)
	if err != nil {
		h.invalidSession(r.Context(), claims, err).respond(w, r)
		return
	}

	username, err := h.users.GetUsername(claims.Subject)
	if errors.Is(err, database.ErrUserNotFound) {
		response.Unauthorized(w, r, response.InvalidTokenChallenge, events.Denied, events.ReasonUserNotFound, msgUserNotFound, err, deviceFields(claims)...)
		return
	}
	if err != nil {
		response.Fail(w, r, events.InternalError, events.ReasonUserLookupFailed, http.StatusInternalServerError, "Failed to fetch username", err)
		return
	}

	accessToken, err := h.jwtManager.GenerateAccess(refreshToken, username)
	if errors.Is(err, jwt.ErrInvalidToken) {
		response.AuthFailure(w, r, invalidRefreshReason(err), msgInvalidSession, err, deviceFields(claims)...)
		return
	}
	if err != nil {
		response.Fail(w, r, events.InternalError, events.ReasonTokenIssueFailed, http.StatusInternalServerError, "Refresh failed", err)
		return
	}

	response.NoStoreJSON(w, true, accessToken, http.StatusOK)
}

type Status struct {
	DeviceID  string `json:"device_id"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
}

func (h *Handler) RefreshStatus(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := refreshtoken.FromCookieOrHeader(r)
	if err != nil {
		missingRefreshToken(w, r, err)
		return
	}

	claims, err := h.jwtManager.VerifyRefresh(refreshToken)
	if err != nil {
		h.invalidSession(r.Context(), claims, err).respond(w, r)
		return
	}

	response.NoStoreJSON(w, true, Status{
		DeviceID:  claims.DeviceID,
		IssuedAt:  claims.IssuedAt.Unix(),
		ExpiresAt: claims.ExpiresAt.Unix(),
	}, http.StatusOK)
}
