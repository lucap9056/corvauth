package login

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/oauth2/providers"
	"github.com/lucap9056/corvauth/server/internal/cache/state"
	"github.com/lucap9056/corvauth/server/internal/events"
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
	maxLoginRequestSize     = 8 << 10
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
	deviceNamer  *deviceNamer
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
		deviceNamer:  newDeviceNamer(),
	}
}

const DefaultDeviceName = "Unknown Device"

type LoginRequest struct {
	RedirectURI string `json:"redirect_uri"`
}

type LoginResponse struct {
	Verifier string `json:"verifier,omitempty"`
	URL      string `json:"url"`
}

type ExchangeResponse struct {
	Success     bool
	Message     string
	Event       events.Event
	Reason      string
	State       int
	OAuth2Token *response.TokenPair
	AuthToken   *response.TokenPair
}

func exchangeFailure(event events.Event, reason, message string, status int) *ExchangeResponse {
	return &ExchangeResponse{
		Success: false,
		Message: message,
		Event:   event,
		Reason:  reason,
		State:   status,
	}
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

	req, err := readLoginRequest(r)
	if err != nil {
		response.Fail(w, r, events.InvalidRequest, events.ReasonBodyInvalid, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	requestedRedirectURL := req.RedirectURI
	if requestedRedirectURL == "" && h.options.DefaultRedirectURL == "" {
		response.Fail(w, r, events.InvalidRequest, events.ReasonRedirectURIMissing, http.StatusBadRequest, "Missing redirect_uri parameter", nil)
		return
	}
	redirectURL, allowed := h.resolveRedirectURL(requestedRedirectURL)
	if !allowed {
		response.Fail(w, r, events.InvalidRequest, events.ReasonRedirectURINotAllowed, http.StatusBadRequest, "redirect_uri is not allowed", nil)
		return
	}

	verifier := oauth2.GenerateVerifier()
	stateVal := generateState()

	session := state.Session{Verifier: verifier, RedirectURL: redirectURL}
	if err := h.stateCache.Set(r.Context(), stateVal, session); err != nil {
		response.Fail(w, r, events.InternalError, events.ReasonSessionInitFailed, http.StatusInternalServerError, "Failed to initialize login session", err)
		return
	}

	url := h.oauth2Client.AuthURL(stateVal, verifier, redirectURL)
	resp := &LoginResponse{URL: url}
	if h.options.ClientPKCE {
		resp.Verifier = verifier
	}
	response.NoStoreJSON(w, true, resp, http.StatusOK)
}

func readLoginRequest(r *http.Request) (LoginRequest, error) {
	var req LoginRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, maxLoginRequestSize))
	if err != nil || len(body) == 0 {
		return req, err
	}
	err = json.Unmarshal(body, &req)
	return req, err
}

func (h *Handler) resolveRedirectURL(requested string) (string, bool) {
	if requested == "" || requested == h.options.DefaultRedirectURL {
		return "", true
	}
	return requested, slices.Contains(h.options.AllowedRedirectURLs, requested)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {

	code := r.FormValue("code")
	state := r.FormValue("state")
	if state == "" {
		response.Fail(w, r, events.InvalidRequest, events.ReasonStateMissing, http.StatusBadRequest, "Missing state parameter", nil)
		return
	}

	if err := h.oauth2Client.ValidateIssuer(r.FormValue("iss")); err != nil {
		response.Reject(w, r, response.AuthErrorInvalidIssuer, http.StatusBadRequest, "Invalid issuer", err)
		return
	}

	if providerError := r.FormValue("error"); providerError != "" {
		deleteCtx, deleteCancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer deleteCancel()
		_, deleteErr := h.stateCache.Take(deleteCtx, state)
		status := providerErrorStatus(providerError)
		response.Fail(w, r, providerErrorEvent(status), events.ProviderReason(providerError), status, "Authorization failed at provider",
			errors.Join(fmt.Errorf("provider returned error: %q", providerError), deleteErr))
		return
	}

	if code == "" {
		response.Fail(w, r, events.InvalidRequest, events.ReasonCodeMissing, http.StatusBadRequest, "Missing code parameter", nil)
		return
	}

	deviceName := h.deviceNamer.resolve(r)

	headerVerifier := r.Header.Get("X-PKCE-Verifier")

	res, err := h.handleExchange(code, state, deviceName, headerVerifier)
	if res == nil {
		response.Fail(w, r, events.InternalError, events.ReasonUnexpectedResult, http.StatusInternalServerError, "", err)
		return
	}
	if err != nil || !res.Success {
		response.Fail(w, r, res.Event, res.Reason, res.State, res.Message, err)
		return
	}

	if res.AuthToken != nil {

		refreshtoken.SetCookie(w, res.AuthToken.RefreshToken, !h.options.DevMode)

		if h.options.PassOAuthToken && res.OAuth2Token != nil {
			w.Header().Set(h.options.ProviderTokenHeaders.RefreshToken, res.OAuth2Token.RefreshToken)
			w.Header().Set(h.options.ProviderTokenHeaders.AccessToken, res.OAuth2Token.AccessToken)
		}

		response.NoStoreJSON(w, res.Success, res.AuthToken, res.State)
		return
	}

	if res.OAuth2Token != nil {

		if h.options.PassOAuthToken {
			w.Header().Set(h.options.ProviderTokenHeaders.RefreshToken, res.OAuth2Token.RefreshToken)
			w.Header().Set(h.options.ProviderTokenHeaders.AccessToken, res.OAuth2Token.AccessToken)
			response.NoStoreJSON(w, res.Success, "Logged in", res.State)
			return
		}

		response.NoStoreJSON(w, res.Success, res.OAuth2Token, res.State)
		return
	}

	response.Fail(w, r, events.InternalError, events.ReasonUnexpectedResult, http.StatusInternalServerError, "", nil)
}

func providerErrorEvent(status int) events.Event {
	if status >= http.StatusInternalServerError {
		return events.UpstreamError
	}
	return events.Denied
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
			return exchangeFailure(events.InternalError, events.ReasonStateLookupFailed, "Failed to retrieve login session", http.StatusInternalServerError), err
		}
		if session.Verifier == "" {
			return exchangeFailure(events.InvalidRequest, events.ReasonStateInvalid, "Invalid or expired state", http.StatusBadRequest), nil
		}

		if h.options.ClientPKCE {
			if headerVerifier != session.Verifier {
				return exchangeFailure(events.InvalidRequest, events.ReasonPKCEInvalid, "Invalid PKCE verifier", http.StatusBadRequest), nil
			}
		}

		exchangeCtx, exchangeCancel := context.WithTimeout(ctx, 10*time.Second)
		defer exchangeCancel()
		oauth2Token, err := h.oauth2Client.Exchange(exchangeCtx, code, session.Verifier, session.RedirectURL)
		if err != nil {
			if err == context.DeadlineExceeded {
				return exchangeFailure(events.UpstreamError, events.ReasonProviderTimeout, "OAuth2 provider timeout", http.StatusGatewayTimeout), err
			}
			return exchangeFailure(events.UpstreamError, events.ReasonCodeExchangeFailed, "Code exchange failed", http.StatusInternalServerError), err
		}

		if h.db != nil {
			getUserCtx, getUserCancel := context.WithTimeout(ctx, 5*time.Second)
			defer getUserCancel()
			user, err := h.oauth2Client.GetUser(getUserCtx, oauth2Token)
			if err != nil {
				return exchangeFailure(events.UpstreamError, events.ReasonUserinfoFailed, "Failed to fetch user info", http.StatusInternalServerError), err
			}

			username, err := h.users.GetUsername(user.Email)
			if errors.Is(err, database.ErrUserNotFound) && !h.users.External() && h.options.AllowRegistration {
				createdUser, createErr := h.users.CreateUser(user.Name, user.Email)
				if createErr != nil {
					return exchangeFailure(events.InternalError, events.ReasonUserCreateFailed, "Failed to create user", http.StatusInternalServerError), createErr
				}
				username, err = createdUser.Username, nil
			}
			if errors.Is(err, database.ErrUserNotFound) {
				return exchangeFailure(events.Denied, events.ReasonUserNotFound, "User not found", http.StatusUnauthorized), nil
			}
			if err != nil {
				return exchangeFailure(events.InternalError, events.ReasonUserLookupFailed, "Failed to fetch username", http.StatusInternalServerError), err
			}

			secret := generateDeviceSecret()

			deviceID, err := h.db.SaveDeviceSecret(user.Email, device, secret)
			if errors.Is(err, database.ErrUserNotFound) {
				return exchangeFailure(events.Denied, events.ReasonUserNotFound, "User not found", http.StatusUnauthorized), nil
			}
			if err != nil {
				return exchangeFailure(events.InternalError, events.ReasonDeviceRegisterFailed, "Failed to register device session", http.StatusInternalServerError), err
			}

			refreshToken, err := h.jwtManager.GenerateRefresh(user.Email, deviceID, secret, initialDeviceGeneration)
			if err != nil {
				return exchangeFailure(events.InternalError, events.ReasonTokenIssueFailed, "Refresh token generation failed", http.StatusInternalServerError),
					errors.Join(err, h.db.DeleteDevice(user.Email, deviceID))
			}

			accessToken, err := h.jwtManager.GenerateAccess(refreshToken, username)
			if err != nil {
				return exchangeFailure(events.InternalError, events.ReasonTokenIssueFailed, "Access token generation failed", http.StatusInternalServerError),
					errors.Join(err, h.db.DeleteDevice(user.Email, deviceID))
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
