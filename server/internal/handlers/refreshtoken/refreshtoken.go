package refreshtoken

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lucap9056/corvauth/server/internal/handlers/bearer"
)

const CookieName = "refresh_token"

var ErrMissing = errors.New("missing refresh token")

type Request struct {
	RefreshToken string `json:"refresh_token"`
}

func FromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err == nil {
		return cookie.Value, nil
	}

	var req Request
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		return "", err
	}
	return req.RefreshToken, nil
}

func FromCookieOrHeader(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err == nil {
		return cookie.Value, nil
	}

	if token, ok := bearer.Token(r); ok {
		return token, nil
	}
	return "", ErrMissing
}

func SetCookie(w http.ResponseWriter, token string, secure bool) {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if expiresAt, ok := expiryOf(token); ok {
		cookie.Expires = expiresAt
	}
	http.SetCookie(w, cookie)
}

func expiryOf(token string) (time.Time, bool) {
	claims := &jwt.RegisteredClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil || claims.ExpiresAt == nil {
		return time.Time{}, false
	}
	return claims.ExpiresAt.Time, true
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}
