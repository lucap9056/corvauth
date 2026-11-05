package response

import (
	"encoding/json/v2"
	"errors"
	"log"
	"net/http"

	"github.com/lucap9056/corvauth/jwt"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type Body[T any] struct {
	Success bool `json:"success"`
	Message T    `json:"message"`
}

func JSON[T any](w http.ResponseWriter, success bool, message T, code int, internalErr error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if internalErr != nil {
		log.Printf("[ERROR] Status: %d, Msg: %v, Err: %v", code, message, internalErr)
	}

	json.MarshalWrite(w, Body[T]{
		Success: success,
		Message: message,
	})
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

func SetAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, jwt.ErrDeviceNotFound):
		w.Header().Set(AuthErrorHeader, AuthErrorDeviceNotFound)
	case errors.Is(err, jwt.ErrInvalidSignature):
		w.Header().Set(AuthErrorHeader, AuthErrorInvalidSignature)
	case errors.Is(err, jwt.ErrInvalidDeviceTag):
		w.Header().Set(AuthErrorHeader, AuthErrorInvalidDeviceTag)
	case errors.Is(err, jwt.ErrUnexpectedSigningMethod):
		w.Header().Set(AuthErrorHeader, AuthErrorUnexpectedAlg)
	}
}

func Unauthorized(w http.ResponseWriter, challenge, message string, internalErr error) {
	w.Header().Set("WWW-Authenticate", challenge)
	JSON(w, false, message, http.StatusUnauthorized, internalErr)
}

func NoStoreJSON[T any](w http.ResponseWriter, success bool, message T, code int) {
	w.Header().Set("Cache-Control", "no-store")
	JSON(w, success, message, code, nil)
}
