package login

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/oauth2/providers"
	"github.com/lucap9056/corvauth/server/internal/cache/state"
	"github.com/lucap9056/corvauth/server/internal/flight"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/handlers/refreshtoken"
	"github.com/lucap9056/corvauth/server/internal/handlers/response"
	"golang.org/x/oauth2"
)

const (
	flightKeyPrefix         = "exchange:"
	exchangeTimeout         = 30 * time.Second
	initialDeviceGeneration = 1
)

type OAuth2Client interface {
	AuthURL(state, verifier, redirectURL string) string
	ValidateIssuer(iss string) error
	Exchange(ctx context.Context, code, verifier, redirectURL string) (*oauth2.Token, error)
	GetUser(ctx context.Context, token *oauth2.Token) (*providers.Userinfo, error)
	Revoke(ctx context.Context, token *oauth2.Token) error
}

type Handler struct {
	db           options.DB
	users        options.UsersDB
	jwtManager   *jwt.JWTManager
	stateCache   state.Cache
	oauth2Client OAuth2Client
	flight       *flight.Group
	options      *options.Options
}

func New(db options.DB, users options.UsersDB, jwtManager *jwt.JWTManager, stateCache state.Cache, oauth2Client OAuth2Client, flightGroup *flight.Group, opts *options.Options) *Handler {
	return &Handler{
		db:           db,
		users:        users,
		jwtManager:   jwtManager,
		stateCache:   stateCache,
		oauth2Client: oauth2Client,
		flight:       flightGroup,
		options:      opts,
	}
}

const DefaultDeviceName = "Unknown Device"

type LoginResponse struct {
	Verifier string `json:"verifier,omitempty"`
	URL      string `json:"url"`
}

type ExchangeResponse struct {
	Success     bool
	Message     string
	State       int
	OAuth2Token *response.TokenPair
	AuthToken   *response.TokenPair
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateDeviceSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {

	verifier := oauth2.GenerateVerifier()
	stateVal := generateState()

	session := state.Session{Verifier: verifier}
	if err := h.stateCache.Set(r.Context(), stateVal, session); err != nil {
		response.JSON(w, false, "Failed to initialize login session", http.StatusInternalServerError, err)
		return
	}

	url := h.oauth2Client.AuthURL(stateVal, verifier, session.RedirectURL)
	resp := &LoginResponse{URL: url}
	if h.options.ClientPKCE {
		resp.Verifier = verifier
	}
	response.NoStoreJSON(w, true, resp, http.StatusOK)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {

	code := r.FormValue("code")
	state := r.FormValue("state")
	if state == "" {
		response.JSON(w, false, "Missing state parameter", http.StatusBadRequest, nil)
		return
	}

	if err := h.oauth2Client.ValidateIssuer(r.FormValue("iss")); err != nil {
		response.JSON(w, false, "Invalid issuer", http.StatusBadRequest, err)
		return
	}

	if providerError := r.FormValue("error"); providerError != "" {
		deleteCtx, deleteCancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer deleteCancel()
		_, deleteErr := h.stateCache.Take(deleteCtx, state)
		response.JSON(w, false, "Authorization failed at provider", providerErrorStatus(providerError),
			errors.Join(fmt.Errorf("provider returned error: %q", providerError), deleteErr))
		return
	}

	if code == "" {
		response.JSON(w, false, "Missing code parameter", http.StatusBadRequest, nil)
		return
	}

	deviceName := r.Header.Get("X-Device-Name")
	if deviceName == "" {
		deviceName = DefaultDeviceName
	}

	headerVerifier := r.Header.Get("X-PKCE-Verifier")

	res, err := h.handleExchange(code, state, deviceName, headerVerifier)
	if err != nil || !res.Success {
		response.JSON(w, res.Success, res.Message, res.State, err)
		return
	}

	if res.AuthToken != nil {

		refreshtoken.SetCookie(w, res.AuthToken.RefreshToken, !h.options.DevMode)

		if h.options.PassOAuthToken && res.OAuth2Token != nil {
			w.Header().Set("X-Forwarded-Refresh-Token", res.OAuth2Token.RefreshToken)
			w.Header().Set("X-Forwarded-Access-Token", res.OAuth2Token.AccessToken)
		}

		response.NoStoreJSON(w, res.Success, res.AuthToken, res.State)
		return
	}

	if res.OAuth2Token != nil {

		if h.options.PassOAuthToken {
			w.Header().Set("X-Forwarded-Refresh-Token", res.OAuth2Token.RefreshToken)
			w.Header().Set("X-Forwarded-Access-Token", res.OAuth2Token.AccessToken)
			response.NoStoreJSON(w, res.Success, "Logged in", res.State)
			return
		}

		response.NoStoreJSON(w, res.Success, res.OAuth2Token, res.State)
		return
	}

	response.JSON(w, false, "", http.StatusInternalServerError, nil)
}

func providerErrorStatus(providerError string) int {
	switch providerError {
	case "access_denied":
		return http.StatusForbidden
	case "server_error":
		return http.StatusBadGateway
	case "temporarily_unavailable":
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

func (h *Handler) handleExchange(code, state, device, headerVerifier string) (*ExchangeResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), exchangeTimeout)
	defer cancel()

	key := flightKeyPrefix + code + "|" + state + "|" + headerVerifier
	return flight.Do(ctx, h.flight, key, func(ctx context.Context) (*ExchangeResponse, error) {

		stateCtx, stateCancel := context.WithTimeout(ctx, 5*time.Second)
		defer stateCancel()

		session, err := h.stateCache.Take(stateCtx, state)
		if err != nil {
			return &ExchangeResponse{
				Success: false,
				Message: "Failed to retrieve login session",
				State:   http.StatusInternalServerError,
			}, err
		}
		if session.Verifier == "" {
			return &ExchangeResponse{
				Success: false,
				Message: "Invalid or expired state",
				State:   http.StatusBadRequest,
			}, nil
		}

		if h.options.ClientPKCE {
			if headerVerifier != session.Verifier {
				return &ExchangeResponse{
					Success: false,
					Message: "Invalid PKCE verifier",
					State:   http.StatusBadRequest,
				}, nil
			}
		}

		exchangeCtx, exchangeCancel := context.WithTimeout(ctx, 10*time.Second)
		defer exchangeCancel()
		oauth2Token, err := h.oauth2Client.Exchange(exchangeCtx, code, session.Verifier, session.RedirectURL)
		if err != nil {
			if err == context.DeadlineExceeded {
				return &ExchangeResponse{
					Success: false,
					Message: "OAuth2 provider timeout",
					State:   http.StatusGatewayTimeout,
				}, err
			}
			return &ExchangeResponse{
				Success: false,
				Message: "Code exchange failed",
				State:   http.StatusInternalServerError,
			}, err
		}

		if h.db != nil {
			getUserCtx, getUserCancel := context.WithTimeout(ctx, 5*time.Second)
			defer getUserCancel()
			user, err := h.oauth2Client.GetUser(getUserCtx, oauth2Token)
			if err != nil {
				return &ExchangeResponse{
					Success: false,
					Message: "Failed to fetch user info",
					State:   http.StatusInternalServerError,
				}, err
			}

			username, err := h.users.GetUsername(user.Email)
			if errors.Is(err, database.ErrUserNotFound) && !h.users.External() && h.options.AllowRegistration {
				createdUser, createErr := h.users.CreateUser(user.Name, user.Email)
				if createErr != nil {
					return &ExchangeResponse{
						Success: false,
						Message: "Failed to create user",
						State:   http.StatusInternalServerError,
					}, createErr
				}
				username, err = createdUser.Username, nil
			}
			if errors.Is(err, database.ErrUserNotFound) {
				return &ExchangeResponse{
					Success: false,
					Message: "User not found",
					State:   http.StatusUnauthorized,
				}, nil
			}
			if err != nil {
				return &ExchangeResponse{
					Success: false,
					Message: "Failed to fetch username",
					State:   http.StatusInternalServerError,
				}, err
			}

			secret := generateDeviceSecret()

			deviceID, err := h.db.SaveDeviceSecret(user.Email, device, secret)
			if errors.Is(err, database.ErrUserNotFound) {
				return &ExchangeResponse{
					Success: false,
					Message: "User not found",
					State:   http.StatusUnauthorized,
				}, nil
			}
			if err != nil {
				return &ExchangeResponse{
					Success: false,
					Message: "Failed to register device session",
					State:   http.StatusInternalServerError,
				}, err
			}

			refreshToken, err := h.jwtManager.GenerateRefresh(user.Email, deviceID, secret, initialDeviceGeneration)
			if err != nil {
				return &ExchangeResponse{
					Success: false,
					Message: "Refresh token generation failed",
					State:   http.StatusInternalServerError,
				}, errors.Join(err, h.db.DeleteDevice(user.Email, deviceID))
			}

			accessToken, err := h.jwtManager.GenerateAccess(refreshToken, username)
			if err != nil {
				return &ExchangeResponse{
					Success: false,
					Message: "Access token generation failed",
					State:   http.StatusInternalServerError,
				}, errors.Join(err, h.db.DeleteDevice(user.Email, deviceID))
			}

			if !h.options.PassOAuthToken {
				revokeCtx, revokeCancel := context.WithTimeout(ctx, 5*time.Second)
				defer revokeCancel()
				h.oauth2Client.Revoke(revokeCtx, oauth2Token)
			}

			return &ExchangeResponse{
				Success: true,
				Message: "Login successful",
				State:   http.StatusOK,
				OAuth2Token: &response.TokenPair{
					AccessToken:  oauth2Token.AccessToken,
					RefreshToken: oauth2Token.RefreshToken,
				},
				AuthToken: &response.TokenPair{
					AccessToken:  accessToken,
					RefreshToken: refreshToken,
				},
			}, nil
		}

		return &ExchangeResponse{
			Success: true,
			Message: "Login successful",
			State:   http.StatusOK,
			OAuth2Token: &response.TokenPair{
				AccessToken:  oauth2Token.AccessToken,
				RefreshToken: oauth2Token.RefreshToken,
			},
		}, nil
	}, flight.InFlightOnly())
}
