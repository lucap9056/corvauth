package handlers

import (
	"log"
	"net/http"

	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/server/internal/cache/state"
	"github.com/lucap9056/corvauth/server/internal/flight"
	"github.com/lucap9056/corvauth/server/internal/handlers/health"
	"github.com/lucap9056/corvauth/server/internal/handlers/login"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/handlers/refresh"
	"github.com/lucap9056/corvauth/server/internal/handlers/session"
	"github.com/lucap9056/corvauth/server/internal/identity"
)

type Dependencies struct {
	DB             options.DB
	UsersDB        options.UsersDB
	JWTManager     *jwt.JWTManager
	IdentitySigner *identity.Signer
	Flight         *flight.Group
	StateCache     state.Cache
	OAuth2Client   login.OAuth2Client
	Options        []options.Option
}

func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
	opts := options.New(deps.Options...)

	mux.HandleFunc("GET /health", health.Handle)

	if deps.DB != nil {
		refreshHandler := refresh.New(deps.DB, deps.UsersDB, deps.JWTManager, deps.Flight, !opts.DevMode)
		mux.HandleFunc("POST /refresh", refreshHandler.Refresh)
		mux.HandleFunc("POST /refresh-access", refreshHandler.RefreshAccess)
		mux.HandleFunc("POST /refresh-status", refreshHandler.RefreshStatus)

		sessionHandler := session.New(deps.DB, deps.UsersDB, deps.JWTManager, deps.IdentitySigner)
		mux.HandleFunc("GET /verify", sessionHandler.Verify)
		mux.HandleFunc("POST /logout", sessionHandler.Logout)
		if !deps.UsersDB.External() {
			mux.HandleFunc("DELETE /users/me", sessionHandler.DeleteMe)
		}
	}

	if deps.OAuth2Client != nil {
		loginHandler := login.New(deps.DB, deps.UsersDB, deps.JWTManager, deps.StateCache, deps.OAuth2Client, deps.Flight, opts)
		mux.HandleFunc("GET /login", loginHandler.Login)
		mux.HandleFunc("GET /callback", loginHandler.Callback)
		log.Println("OAuth2 is enabled")
	}
}
