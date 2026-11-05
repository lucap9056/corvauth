package session

import (
	"log"
	"net/http"

	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/handlers/bearer"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/handlers/refreshtoken"
	"github.com/lucap9056/corvauth/server/internal/handlers/response"
	"github.com/lucap9056/corvauth/server/internal/identity"
)

type Handler struct {
	db             options.DB
	usersDB        options.UsersDB
	jwtManager     *jwt.JWTManager
	identitySigner *identity.Signer
}

const IdentityHeader = "X-Forwarded-Identity"

func New(db options.DB, usersDB options.UsersDB, jwtManager *jwt.JWTManager, identitySigner *identity.Signer) *Handler {
	return &Handler{
		db:             db,
		usersDB:        usersDB,
		jwtManager:     jwtManager,
		identitySigner: identitySigner,
	}
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (*jwt.AccessClaims, bool) {
	tokenStr, ok := bearer.Token(r)
	if !ok {
		response.Unauthorized(w, response.BearerChallenge, "Missing Bearer token", nil)
		return nil, false
	}

	claims, err := h.jwtManager.VerifyAccess(tokenStr)
	if err != nil {
		response.SetAuthError(w, err)
		response.Unauthorized(w, response.InvalidTokenChallenge, "Invalid access token", err)
		return nil, false
	}
	return claims, true
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	if h.identitySigner != nil {
		identityToken, err := h.identitySigner.Sign(claims.UserEmail, claims.Username, claims.DeviceID, claims.ExpiresAt)
		if err != nil {
			response.JSON(w, false, "Failed to sign identity token", http.StatusInternalServerError, err)
			return
		}
		w.Header().Set(IdentityHeader, identityToken)
	} else {
		w.Header().Set("X-Forwarded-User-Email", claims.UserEmail)
		w.Header().Set("X-Forwarded-Device-ID", claims.DeviceID)
		if claims.Username != "" {
			w.Header().Set("X-Forwarded-Username", claims.Username)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) VerifyOptional(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearer.Token(r); !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.Verify(w, r)
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
		response.JSON(w, false, "Failed to list sessions", http.StatusInternalServerError, err)
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
		response.JSON(w, false, "Failed to remove device sessions", http.StatusInternalServerError, err)
		return
	}

	refreshtoken.ClearCookie(w)

	response.JSON(w, true, "Logged out from all sessions", http.StatusOK, nil)
}

func (h *Handler) DeleteCurrent(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := refreshtoken.FromRequest(r)
	if err == nil {
		claims, pErr := h.jwtManager.VerifyRefresh(refreshToken)
		if pErr == nil {
			err = h.db.DeleteDevice(claims.Subject, claims.DeviceID)
			if err != nil {
				log.Printf("[WARN] Failed to delete device from DB on logout: %v", err)
			}
		}
	}

	refreshtoken.ClearCookie(w)

	response.JSON(w, true, "Logged out and device session revoked", http.StatusOK, nil)
}

func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	userEmail := claims.UserEmail

	if err := h.db.DeleteAllDevices(userEmail); err != nil {
		response.JSON(w, false, "Failed to remove device sessions", http.StatusInternalServerError, err)
		return
	}

	if err := h.usersDB.DeleteUser(userEmail); err != nil {
		response.JSON(w, false, "Failed to delete account", http.StatusInternalServerError, err)
		return
	}

	refreshtoken.ClearCookie(w)

	response.JSON(w, true, "Account deleted", http.StatusOK, nil)
}
