package session

import (
	"net/http"

	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/handlers/bearer"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/handlers/refreshtoken"
	"github.com/lucap9056/corvauth/server/internal/handlers/response"
	"github.com/lucap9056/corvauth/server/internal/identity"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"go.uber.org/zap"
)

type Handler struct {
	db             options.DB
	usersDB        options.UsersDB
	jwtManager     *jwt.JWTManager
	identitySigner *identity.Signer
	headers        identity.Headers
}

func New(db options.DB, usersDB options.UsersDB, jwtManager *jwt.JWTManager, identitySigner *identity.Signer, headers identity.Headers) *Handler {
	return &Handler{
		db:             db,
		usersDB:        usersDB,
		jwtManager:     jwtManager,
		identitySigner: identitySigner,
		headers:        headers,
	}
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (*jwt.AccessClaims, bool) {
	tokenStr, ok := bearer.Token(r)
	if !ok {
		response.Unauthorized(w, r, response.BearerChallenge, events.Unauthenticated, events.ReasonTokenMissing, "Missing Bearer token", nil)
		return nil, false
	}

	claims, err := h.jwtManager.VerifyAccess(tokenStr)
	if err != nil {
		response.AuthFailure(w, r, events.ReasonTokenInvalid, "Invalid access token", err)
		return nil, false
	}
	return claims, true
}

func rejectSpoofedIdentity(w http.ResponseWriter, r *http.Request) {
	response.Reject(w, r, response.AuthErrorSpoofedIdentity, http.StatusForbidden, "Identity headers do not match the access token", nil)
}

func (h *Handler) carriesIdentity(r *http.Request) bool {
	for _, name := range h.headers.Names() {
		if len(r.Header.Values(name)) > 0 {
			return true
		}
	}
	return false
}

func absent(r *http.Request, names ...string) bool {
	for _, name := range names {
		if len(r.Header.Values(name)) > 0 {
			return false
		}
	}
	return true
}

func absentOrEqual(r *http.Request, name, want string) bool {
	values := r.Header.Values(name)
	return len(values) == 0 || (len(values) == 1 && values[0] == want)
}

func (h *Handler) identityTokenMatches(r *http.Request, claims *jwt.AccessClaims) bool {
	values := r.Header.Values(h.headers.Identity)
	if len(values) == 0 {
		return true
	}
	if len(values) > 1 {
		return false
	}
	forwarded, err := h.identitySigner.Verify(values[0])
	return err == nil &&
		forwarded.Subject == claims.UserEmail &&
		forwarded.DeviceID == claims.DeviceID &&
		forwarded.Username == claims.Username
}

func (h *Handler) matchesClaims(r *http.Request, claims *jwt.AccessClaims) bool {
	if h.identitySigner != nil {
		return absent(r, h.headers.UserEmail, h.headers.DeviceID, h.headers.Username) && h.identityTokenMatches(r, claims)
	}
	return absent(r, h.headers.Identity) &&
		absentOrEqual(r, h.headers.UserEmail, claims.UserEmail) &&
		absentOrEqual(r, h.headers.DeviceID, claims.DeviceID) &&
		absentOrEqual(r, h.headers.Username, claims.Username)
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearer.Token(r); !ok && h.carriesIdentity(r) {
		rejectSpoofedIdentity(w, r)
		return
	}

	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if !h.matchesClaims(r, claims) {
		rejectSpoofedIdentity(w, r)
		return
	}

	if h.identitySigner != nil {
		identityToken, err := h.identitySigner.Sign(claims.UserEmail, claims.Username, claims.DeviceID, claims.ExpiresAt)
		if err != nil {
			response.Fail(w, r, events.InternalError, events.ReasonIdentitySignFailed, http.StatusInternalServerError, "Failed to sign identity token", err)
			return
		}
		w.Header().Set(h.headers.Identity, identityToken)
	} else {
		w.Header().Set(h.headers.UserEmail, claims.UserEmail)
		w.Header().Set(h.headers.DeviceID, claims.DeviceID)
		if claims.Username != "" {
			w.Header().Set(h.headers.Username, claims.Username)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) VerifyOptional(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearer.Token(r); ok {
		h.Verify(w, r)
		return
	}
	if h.carriesIdentity(r) {
		rejectSpoofedIdentity(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type Session struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	UpdatedAt  int64  `json:"updated_at"`
	Current    bool   `json:"current"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	devices, err := h.db.ListDevices(claims.UserEmail)
	if err != nil {
		response.Fail(w, r, events.InternalError, events.ReasonSessionListFailed, http.StatusInternalServerError, "Failed to list sessions", err)
		return
	}

	sessions := make([]Session, 0, len(devices))
	for _, device := range devices {
		sessions = append(sessions, Session{
			DeviceID:   device.DeviceID,
			DeviceName: device.DeviceName,
			UpdatedAt:  device.UpdatedAt.Unix(),
			Current:    device.DeviceID == claims.DeviceID,
		})
	}
	response.NoStoreJSON(w, true, sessions, http.StatusOK)
}

func (h *Handler) DeleteAll(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	if err := h.db.DeleteAllDevices(claims.UserEmail); err != nil {
		response.Fail(w, r, events.InternalError, events.ReasonSessionRevokeFailed, http.StatusInternalServerError, "Failed to remove device sessions", err)
		return
	}

	refreshtoken.ClearCookie(w)

	response.JSON(w, true, "Logged out from all sessions", http.StatusOK)
}

func (h *Handler) DeleteCurrent(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := refreshtoken.FromRequest(r)
	if err == nil {
		claims, pErr := h.jwtManager.VerifyRefresh(refreshToken)
		if pErr == nil {
			err = h.db.DeleteDevice(claims.Subject, claims.DeviceID)
			if err != nil {
				logs.Event(logs.FromContext(r.Context()), events.InternalError, events.ReasonDeviceDeleteFailed, "failed to delete device on logout", err, zap.String("deviceId", claims.DeviceID))
			}
		}
	}

	refreshtoken.ClearCookie(w)

	response.JSON(w, true, "Logged out and device session revoked", http.StatusOK)
}

func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	userEmail := claims.UserEmail

	if err := h.db.DeleteAllDevices(userEmail); err != nil {
		response.Fail(w, r, events.InternalError, events.ReasonSessionRevokeFailed, http.StatusInternalServerError, "Failed to remove device sessions", err)
		return
	}

	if err := h.usersDB.DeleteUser(userEmail); err != nil {
		response.Fail(w, r, events.InternalError, events.ReasonAccountDeleteFailed, http.StatusInternalServerError, "Failed to delete account", err)
		return
	}

	refreshtoken.ClearCookie(w)

	response.JSON(w, true, "Account deleted", http.StatusOK)
}
