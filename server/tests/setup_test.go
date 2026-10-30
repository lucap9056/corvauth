package integration

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/oauth2/oauthclient"
	"github.com/lucap9056/corvauth/server/internal/cache/state"
	"github.com/lucap9056/corvauth/server/internal/flight"
	"github.com/lucap9056/corvauth/server/internal/handlers"
	"github.com/lucap9056/corvauth/server/internal/handlers/login"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/identity"
	"github.com/lucap9056/corvauth/server/internal/usersdb"
)

// oauthStub is a minimal in-process OAuth2 provider for testing.
type oauthStub struct {
	*httptest.Server
	UserID string
	Email  string
	Name   string

	mu                  sync.Mutex
	exchangeRedirectURI string
}

func newOAuthStub(userID, email, name string) *oauthStub {
	s := &oauthStub{UserID: userID, Email: email, Name: name}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", s.handleToken)
	mux.HandleFunc("GET /userinfo", s.handleUserInfo)
	mux.HandleFunc("POST /revoke", s.handleRevoke)
	mux.HandleFunc("GET /.well-known/openid-configuration", s.handleDiscovery)
	s.Server = httptest.NewServer(mux)
	return s
}

func (s *oauthStub) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.MarshalWrite(w, map[string]any{
		"issuer":                 s.URL,
		"authorization_endpoint": s.URL + "/authorize",
		"token_endpoint":         s.URL + "/token",
		"userinfo_endpoint":      s.URL + "/userinfo",
		"revocation_endpoint":    s.URL + "/revoke",
		"authorization_response_iss_parameter_supported": true,
	})
}

func (s *oauthStub) handleToken(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	s.mu.Lock()
	s.exchangeRedirectURI = r.PostForm.Get("redirect_uri")
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.MarshalWrite(w, map[string]any{
		"access_token":  "stub-access-token",
		"refresh_token": "stub-refresh-token",
		"token_type":    "Bearer",
		"expires_in":    3600,
	})
}

func (s *oauthStub) lastExchangeRedirectURI() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exchangeRedirectURI
}

func (s *oauthStub) handleUserInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.MarshalWrite(w, map[string]any{
		"id":             s.UserID,
		"email":          s.Email,
		"email_verified": true,
		"name":           s.Name,
	})
}

func (s *oauthStub) handleRevoke(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

const testRedirectURL = "http://localhost/callback"

// mockDB implements options.DB and jwt.Database using in-memory maps.
type mockDB struct {
	mu            sync.Mutex
	users         map[string]bool // email -> exists
	usernames     map[string]string
	devices       map[string]*mockDevice
	idCounter     int
	externalUsers bool
	usernameErr   error
}

type mockDevice struct {
	owner      string
	secret     string
	generation int
}

func newMockDB() *mockDB {
	return &mockDB{
		users:     make(map[string]bool),
		usernames: make(map[string]string),
		devices:   make(map[string]*mockDevice),
	}
}

func (m *mockDB) seedUser(email string) {
	m.users[email] = true
}

func (m *mockDB) seedUsername(email, username string) {
	m.users[email] = true
	m.usernames[email] = username
}

func (m *mockDB) GetUsername(email string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.usernameErr != nil {
		return "", m.usernameErr
	}
	if !m.users[email] {
		return "", database.ErrUserNotFound
	}
	return m.usernames[email], nil
}

func (m *mockDB) deviceCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.devices)
}

func (m *mockDB) External() bool {
	return m.externalUsers
}

func (m *mockDB) hasUser(email string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.users[email]
}

func (m *mockDB) CreateUser(username, email string) (*usersdb.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[email] = true
	m.usernames[email] = username
	return &usersdb.User{Username: username, Email: email}, nil
}

func (m *mockDB) DeleteUser(email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.users, email)
	for id, d := range m.devices {
		if d.owner == email {
			delete(m.devices, id)
		}
	}
	return nil
}

func (m *mockDB) SaveDeviceSecret(email, _, secret string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.users[email] {
		return "", database.ErrUserNotFound
	}
	m.idCounter++
	deviceID := fmt.Sprintf("device-%d", m.idCounter)
	m.devices[deviceID] = &mockDevice{owner: email, secret: secret, generation: 1}
	return deviceID, nil
}

func (m *mockDB) UpdateDeviceSecret(deviceID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[deviceID]
	if !ok {
		return 0, errors.New("device not found")
	}
	d.generation++
	return d.generation, nil
}

func (m *mockDB) GetDeviceSecret(deviceID string) (string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[deviceID]
	if !ok {
		return "", 0, jwt.ErrDeviceNotFound
	}
	return d.secret, d.generation, nil
}

func (m *mockDB) DeleteDevice(email, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.devices[deviceID]; ok && d.owner == email {
		delete(m.devices, deviceID)
	}
	return nil
}

func (m *mockDB) DeleteAllDevices(email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, d := range m.devices {
		if d.owner == email {
			delete(m.devices, id)
		}
	}
	return nil
}

// testEnv holds all wired-up dependencies for a single test scenario.
type testEnv struct {
	stub       *oauthStub
	db         *mockDB
	jwtManager *jwt.JWTManager
	mux        *http.ServeMux
}

// newTestEnv builds a handler stack backed by the given stub and mock DB.
// Pass db=nil to simulate a no-database deployment.
func newTestEnv(stub *oauthStub, db *mockDB, opts ...options.Option) *testEnv {
	oauth2Client := oauthclient.New(oauthclient.Config{
		Provider:     "generic",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  testRedirectURL,
		AuthURL:      stub.URL + "/authorize",
		TokenURL:     stub.URL + "/token",
		UserinfoURL:  stub.URL + "/userinfo",
		RevokeURL:    stub.URL + "/revoke",
		Scopes:       []string{"email"},
	})
	return newTestEnvWithClient(stub, db, oauth2Client, opts...)
}

func newOIDCTestEnv(t *testing.T, stub *oauthStub, db *mockDB, opts ...options.Option) *testEnv {
	t.Helper()
	oauth2Client, err := oauthclient.NewOIDC(context.Background(), oauthclient.OIDCConfig{
		IssuerURL:    stub.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  testRedirectURL,
		Scopes:       []string{"email"},
	})
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	return newTestEnvWithClient(stub, db, oauth2Client, opts...)
}

func newTestEnvWithClient(stub *oauthStub, db *mockDB, oauth2Client login.OAuth2Client, opts ...options.Option) *testEnv {
	return newTestEnvWithIdentity(stub, db, oauth2Client, nil, opts...)
}

func newTestEnvWithIdentity(stub *oauthStub, db *mockDB, oauth2Client login.OAuth2Client, identitySigner *identity.Signer, opts ...options.Option) *testEnv {
	var jwtDB jwt.Database
	var handlerDB options.DB
	var usersDB options.UsersDB
	if db != nil {
		jwtDB = db
		handlerDB = db
		usersDB = db
	}

	jwtManager := jwt.NewJWTManager(jwtDB)
	flightGroup, err := flight.New()
	if err != nil {
		panic(err)
	}
	stateCache, err := state.NewCache(nil)
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, handlers.Dependencies{
		DB:             handlerDB,
		UsersDB:        usersDB,
		JWTManager:     jwtManager,
		IdentitySigner: identitySigner,
		Flight:         flightGroup,
		StateCache:     stateCache,
		OAuth2Client:   oauth2Client,
		Options:        append([]options.Option{options.WithRedirectURLs(testRedirectURL, nil)}, opts...),
	})

	return &testEnv{stub: stub, db: db, jwtManager: jwtManager, mux: mux}
}

func (e *testEnv) do(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

// issueTokens seeds a device in the mock DB and returns a valid (refresh, access) pair.
func (e *testEnv) issueTokens(email, deviceName string) (refresh, access string) {
	const secret = "test-device-secret"
	deviceID, _ := e.db.SaveDeviceSecret(email, deviceName, secret)
	_, generation, _ := e.db.GetDeviceSecret(deviceID)
	refresh, _ = e.jwtManager.GenerateRefresh(email, deviceID, secret, generation)
	access, _ = e.jwtManager.GenerateAccess(refresh, "")
	return
}
