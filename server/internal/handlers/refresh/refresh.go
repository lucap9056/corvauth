package refresh

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/flight"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/handlers/refreshtoken"
	"github.com/lucap9056/corvauth/server/internal/handlers/response"
)

const (
	flightKeyPrefix = "refresh:"
	rotateTimeout   = 10 * time.Second
)

type rotateError struct {
	message string
	status  int
	err     error
}

func (e *rotateError) Error() string {
	return fmt.Sprintf("%s: %v", e.message, e.err)
}

func (e *rotateError) Unwrap() error {
	return e.err
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

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := refreshtoken.FromRequest(r)
	if err != nil {
		response.Unauthorized(w, response.BearerChallenge, "Invalid refresh token", err)
		return
	}

	// The rotation is shared with concurrent callers, so one client disconnecting must not abort it for the rest.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), rotateTimeout)
	defer cancel()

	tokens, err := flight.Do(ctx, h.flight, flightKeyPrefix+refreshToken, func(ctx context.Context) (response.TokenPair, error) {
		return h.rotate(refreshToken)
	})
	if err != nil {
		var rotateErr *rotateError
		if errors.As(err, &rotateErr) {
			if rotateErr.status == http.StatusUnauthorized {
				w.Header().Set("WWW-Authenticate", response.InvalidTokenChallenge)
				response.SetAuthError(w, rotateErr.err)
			}
			response.JSON(w, false, rotateErr.message, rotateErr.status, rotateErr.err)
			return
		}
		response.JSON(w, false, "Failed to rotate refresh token", http.StatusInternalServerError, err)
		return
	}

	refreshtoken.SetCookie(w, tokens.RefreshToken, h.secureCookie)
	response.NoStoreJSON(w, true, tokens, http.StatusOK)
}

func (h *Handler) deleteDeviceOnReuse(claims *jwt.RefreshClaims, err error) {
	if !errors.Is(err, jwt.ErrTokenRevoked) {
		return
	}
	if delErr := h.db.DeleteDevice(claims.Subject, claims.DeviceID); delErr != nil {
		log.Printf("[WARN] Failed to delete device after refresh token reuse: %v", delErr)
	}
}

func (h *Handler) rotate(refreshToken string) (response.TokenPair, error) {
	claims, err := h.jwtManager.VerifyRefresh(refreshToken)
	if err != nil {
		h.deleteDeviceOnReuse(claims, err)
		return response.TokenPair{}, &rotateError{"Invalid session or expired refresh token", http.StatusUnauthorized, err}
	}

	username, err := h.users.GetUsername(claims.Subject)
	if errors.Is(err, database.ErrUserNotFound) {
		return response.TokenPair{}, &rotateError{"User not found", http.StatusUnauthorized, err}
	}
	if err != nil {
		return response.TokenPair{}, &rotateError{"Failed to fetch username", http.StatusInternalServerError, err}
	}

	newRefreshToken, rotatedClaims, err := h.jwtManager.RotateRefresh(refreshToken)
	if errors.Is(err, jwt.ErrInvalidToken) {
		h.deleteDeviceOnReuse(rotatedClaims, err)
		return response.TokenPair{}, &rotateError{"Invalid session or expired refresh token", http.StatusUnauthorized, err}
	}
	if err != nil {
		return response.TokenPair{}, &rotateError{"Failed to rotate refresh token", http.StatusInternalServerError, err}
	}

	accessToken, err := h.jwtManager.GenerateAccess(newRefreshToken, username)
	if err != nil {
		return response.TokenPair{}, &rotateError{"Access token generation failed", http.StatusInternalServerError, err}
	}

	return response.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (h *Handler) RefreshAccess(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := refreshtoken.FromRequest(r)
	if err != nil {
		response.Unauthorized(w, response.BearerChallenge, "Invalid refresh token", err)
		return
	}

	claims, err := h.jwtManager.VerifyRefresh(refreshToken)
	if err != nil {
		h.deleteDeviceOnReuse(claims, err)
		response.SetAuthError(w, err)
		response.Unauthorized(w, response.InvalidTokenChallenge, "Invalid session or expired refresh token", err)
		return
	}

	username, err := h.users.GetUsername(claims.Subject)
	if errors.Is(err, database.ErrUserNotFound) {
		response.Unauthorized(w, response.InvalidTokenChallenge, "User not found", err)
		return
	}
	if err != nil {
		response.JSON(w, false, "Failed to fetch username", http.StatusInternalServerError, err)
		return
	}

	accessToken, err := h.jwtManager.GenerateAccess(refreshToken, username)
	if errors.Is(err, jwt.ErrInvalidToken) {
		response.SetAuthError(w, err)
		response.Unauthorized(w, response.InvalidTokenChallenge, "Invalid session or expired refresh token", err)
		return
	}
	if err != nil {
		response.JSON(w, false, "Refresh failed", http.StatusInternalServerError, err)
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
	refreshToken, err := refreshtoken.FromRequest(r)
	if err != nil {
		response.Unauthorized(w, response.BearerChallenge, "Invalid refresh token", err)
		return
	}

	claims, err := h.jwtManager.VerifyRefresh(refreshToken)
	if err != nil {
		h.deleteDeviceOnReuse(claims, err)
		response.SetAuthError(w, err)
		response.Unauthorized(w, response.InvalidTokenChallenge, "Invalid session or expired refresh token", err)
		return
	}

	response.NoStoreJSON(w, true, Status{
		DeviceID:  claims.DeviceID,
		IssuedAt:  claims.IssuedAt.Unix(),
		ExpiresAt: claims.ExpiresAt.Unix(),
	}, http.StatusOK)
}
