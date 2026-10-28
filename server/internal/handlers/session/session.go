package session

import (
	"log"
	"net/http"
	"strings"

	"github.com/lucap9056/corvauth/jwt"
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

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, true
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	tokenStr, ok := bearerToken(r)
	if !ok {
		response.Unauthorized(w, response.BearerChallenge, "Missing Bearer token", nil)
		return
	}

	claims, err := h.jwtManager.VerifyAccess(tokenStr)
	if err != nil {
		response.SetAuthError(w, err)
		response.Unauthorized(w, response.InvalidTokenChallenge, "Invalid access token", err)
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
	if _, ok := bearerToken(r); !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.Verify(w, r)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
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
	tokenStr, ok := bearerToken(r)
	if !ok {
		response.Unauthorized(w, response.BearerChallenge, "Missing Bearer token", nil)
		return
	}

	claims, err := h.jwtManager.VerifyAccess(tokenStr)
	if err != nil {
		response.SetAuthError(w, err)
		response.Unauthorized(w, response.InvalidTokenChallenge, "Invalid access token", err)
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
