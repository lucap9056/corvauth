package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lucap9056/corvauth/database/schema"
	"github.com/lucap9056/corvauth/oauth2/providers"
	"github.com/lucap9056/corvauth/server/internal/identity"
)

const (
	EnvHTTPAddress          = "HTTP_ADDRESS"
	EnvHTTPMode             = "HTTP_MODE"
	EnvDatabaseURL          = "DATABASE_URL"
	EnvDBMaxOpenConns       = "DB_MAX_OPEN_CONNS"
	EnvDBMaxIdleConns       = "DB_MAX_IDLE_CONNS"
	EnvDBConnMaxLifetime    = "DB_CONN_MAX_LIFETIME"
	EnvDBConnMaxIdleTime    = "DB_CONN_MAX_IDLE_TIME"
	EnvDBCleanupInterval    = "DB_CLEANUP_INTERVAL"
	EnvDBAutoCreateSchema   = "DB_AUTO_CREATE_SCHEMA"
	EnvDBUserEmailReference = "DB_USER_EMAIL_REFERENCE"
	EnvDBUserUsernameColumn = "DB_USER_USERNAME_COLUMN"
	EnvJWTAccessDuration    = "JWT_ACCESS_TOKEN_DURATION"
	EnvJWTRefreshDuration   = "JWT_REFRESH_TOKEN_DURATION"
	EnvJWTIssuer            = "JWT_ISSUER"
	EnvJWTAudience          = "JWT_AUDIENCE"
	EnvIdentityJWTSecret    = "IDENTITY_JWT_SECRET"
	EnvRedisURL             = "REDIS_URL"
	EnvOAuth2Provider       = "OAUTH2_PROVIDER"
	EnvOAuth2ClientID       = "OAUTH2_CLIENT_ID"
	EnvOAuth2ClientSecret   = "OAUTH2_CLIENT_SECRET"
	EnvOAuth2RedirectURL    = "OAUTH2_REDIRECT_URL"
	EnvOAuth2AuthURL        = "OAUTH2_AUTH_URL"
	EnvOAuth2TokenURL       = "OAUTH2_TOKEN_URL"
	EnvOAuth2UserinfoURL    = "OAUTH2_USERINFO_URL"
	EnvOAuth2RevokeURL      = "OAUTH2_REVOKE_URL"
	EnvOAuth2Scopes         = "OAUTH2_SCOPES"
	EnvOAuth2ClientPKCE     = "OAUTH2_CLIENT_PKCE"
	EnvOIDCIssuerURL        = "OIDC_ISSUER_URL"
	EnvAllowRegistration    = "ALLOW_REGISTRATION"
	EnvPassOAuthToken       = "PASS_OAUTH_TOKEN"
	EnvAllowUnverifiedEmail = "ALLOW_UNVERIFIED_EMAIL"

	DefaultHTTPAddress = ":80"
	ModeDevelopment    = "development"
	ModeProduction     = "production"

	DefaultDBMaxOpenConns     = 20
	DefaultDBMaxIdleConns     = 15
	DefaultDBConnMaxLifetime  = 5 * time.Minute
	DefaultDBConnMaxIdleTime  = 2 * time.Minute
	DefaultDBCleanupInterval  = 24 * time.Hour
	DefaultJWTAccessDuration  = 15 * time.Minute
	DefaultJWTRefreshDuration = 7 * 24 * time.Hour
)

var (
	ErrOIDCMissingClient          = errors.New("OIDC_ISSUER_URL requires OAUTH2_CLIENT_ID, OAUTH2_CLIENT_SECRET, and OAUTH2_REDIRECT_URL")
	ErrGenericProviderMissingURLs = errors.New("generic OAuth2 provider requires AUTH_URL, TOKEN_URL, and USERINFO_URL")
	ErrInvalidInteger             = errors.New("invalid integer")
	ErrNonPositiveDuration        = errors.New("duration must be positive")
	ErrInvalidUsernameColumn      = errors.New("invalid username column")
	ErrUsernameColumnWithoutRef   = errors.New("DB_USER_USERNAME_COLUMN requires DB_USER_EMAIL_REFERENCE")
	ErrIdentitySecretTooShort     = fmt.Errorf("IDENTITY_JWT_SECRET must be at least %d bytes", identity.MinSecretLength)
)

var identifierPattern = regexp.MustCompile("^[a-z_][a-z0-9_]*$")

type Config struct {
	HTTP     *HTTP
	Database *Database
	JWT      *JWT
	Redis    *Redis
	OAuth2   *OAuth2
	Auth     *Auth
}

type HTTP struct {
	Address string
	Mode    string
}

func (h *HTTP) DevMode() bool {
	return h.Mode == ModeDevelopment
}

type Database struct {
	URL                string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	CleanupInterval    time.Duration
	AutoCreateSchema   bool
	UserEmailReference string
	UserUsernameColumn string
}

type JWT struct {
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	Issuer               string
	Audience             string
	IdentitySecret       string
}

type Redis struct {
	URL string
}

type OAuth2 struct {
	Client   *Client
	Scopes   []string
	OIDC     *OIDC
	Provider *Provider
}

type Client struct {
	ID          string
	Secret      string
	RedirectURL string
	PKCE        bool
}

type OIDC struct {
	IssuerURL string
}

type Provider struct {
	Name        string
	AuthURL     string
	TokenURL    string
	UserinfoURL string
	RevokeURL   string
}

type Auth struct {
	AllowRegistration    bool
	PassOAuthToken       bool
	AllowUnverifiedEmail bool
}

func Load() (*Config, error) {
	env := &envReader{}
	cfg := &Config{
		HTTP: &HTTP{
			Address: stringOr(EnvHTTPAddress, DefaultHTTPAddress),
			Mode:    stringOr(EnvHTTPMode, ModeProduction),
		},
		Database: loadDatabase(env),
		JWT:      loadJWT(env),
		Redis:    loadRedis(),
		OAuth2:   loadOAuth2(env),
		Auth: &Auth{
			AllowRegistration:    isTrue(EnvAllowRegistration),
			PassOAuthToken:       isTrue(EnvPassOAuthToken),
			AllowUnverifiedEmail: isTrue(EnvAllowUnverifiedEmail),
		},
	}

	if env.errs != nil {
		return nil, env.errs
	}
	return cfg, nil
}

func LoadDatabase() (*Database, error) {
	env := &envReader{}
	db := loadDatabase(env)
	if env.errs != nil {
		return nil, env.errs
	}
	return db, nil
}

func loadDatabase(env *envReader) *Database {
	url := os.Getenv(EnvDatabaseURL)
	if url == "" {
		return nil
	}
	db := &Database{
		URL:                url,
		MaxOpenConns:       env.integer(EnvDBMaxOpenConns, DefaultDBMaxOpenConns),
		MaxIdleConns:       env.integer(EnvDBMaxIdleConns, DefaultDBMaxIdleConns),
		ConnMaxLifetime:    env.duration(EnvDBConnMaxLifetime, DefaultDBConnMaxLifetime, time.Minute),
		ConnMaxIdleTime:    env.duration(EnvDBConnMaxIdleTime, DefaultDBConnMaxIdleTime, time.Minute),
		CleanupInterval:    env.duration(EnvDBCleanupInterval, DefaultDBCleanupInterval, time.Hour),
		AutoCreateSchema:   isTrue(EnvDBAutoCreateSchema),
		UserEmailReference: os.Getenv(EnvDBUserEmailReference),
		UserUsernameColumn: strings.ToLower(strings.TrimSpace(os.Getenv(EnvDBUserUsernameColumn))),
	}
	if db.UserEmailReference != "" {
		if _, err := schema.ParseUserEmailReference(db.UserEmailReference); err != nil {
			env.fail(fmt.Errorf("%s: %w", EnvDBUserEmailReference, err))
		}
	}
	if db.UserUsernameColumn != "" {
		if db.UserEmailReference == "" {
			env.fail(ErrUsernameColumnWithoutRef)
		} else if !identifierPattern.MatchString(db.UserUsernameColumn) {
			env.fail(fmt.Errorf("%s: %w: %q", EnvDBUserUsernameColumn, ErrInvalidUsernameColumn, db.UserUsernameColumn))
		}
	}
	return db
}

func loadJWT(env *envReader) *JWT {
	jwt := &JWT{
		AccessTokenDuration:  env.duration(EnvJWTAccessDuration, DefaultJWTAccessDuration, 0),
		RefreshTokenDuration: env.duration(EnvJWTRefreshDuration, DefaultJWTRefreshDuration, 0),
		Issuer:               os.Getenv(EnvJWTIssuer),
		Audience:             os.Getenv(EnvJWTAudience),
		IdentitySecret:       os.Getenv(EnvIdentityJWTSecret),
	}
	if jwt.AccessTokenDuration <= 0 {
		env.fail(fmt.Errorf("%s: %w", EnvJWTAccessDuration, ErrNonPositiveDuration))
	}
	if jwt.RefreshTokenDuration <= 0 {
		env.fail(fmt.Errorf("%s: %w", EnvJWTRefreshDuration, ErrNonPositiveDuration))
	}
	if jwt.IdentitySecret != "" && len(jwt.IdentitySecret) < identity.MinSecretLength {
		env.fail(ErrIdentitySecretTooShort)
	}
	return jwt
}

func loadRedis() *Redis {
	url := os.Getenv(EnvRedisURL)
	if url == "" {
		return nil
	}
	return &Redis{URL: url}
}

func loadOAuth2(env *envReader) *OAuth2 {
	client := &Client{
		ID:          os.Getenv(EnvOAuth2ClientID),
		Secret:      os.Getenv(EnvOAuth2ClientSecret),
		RedirectURL: os.Getenv(EnvOAuth2RedirectURL),
		PKCE:        isTrue(EnvOAuth2ClientPKCE),
	}
	issuerURL := os.Getenv(EnvOIDCIssuerURL)

	if client.ID == "" || client.Secret == "" || client.RedirectURL == "" {
		if issuerURL != "" {
			env.fail(ErrOIDCMissingClient)
		}
		return nil
	}

	oauth2 := &OAuth2{
		Client: client,
		Scopes: splitList(os.Getenv(EnvOAuth2Scopes)),
	}
	if issuerURL != "" {
		oauth2.OIDC = &OIDC{IssuerURL: issuerURL}
		return oauth2
	}

	provider := &Provider{
		Name:        os.Getenv(EnvOAuth2Provider),
		AuthURL:     os.Getenv(EnvOAuth2AuthURL),
		TokenURL:    os.Getenv(EnvOAuth2TokenURL),
		UserinfoURL: os.Getenv(EnvOAuth2UserinfoURL),
		RevokeURL:   os.Getenv(EnvOAuth2RevokeURL),
	}
	if !providers.IsBuiltin(provider.Name) &&
		(provider.AuthURL == "" || provider.TokenURL == "" || provider.UserinfoURL == "") {
		env.fail(ErrGenericProviderMissingURLs)
	}
	oauth2.Provider = provider
	return oauth2
}

type envReader struct {
	errs error
}

func (r *envReader) fail(err error) {
	r.errs = errors.Join(r.errs, err)
}

func (r *envReader) integer(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || n < 0 {
		r.fail(fmt.Errorf("%s: %w: %q must be a non-negative integer", key, ErrInvalidInteger, val))
		return fallback
	}
	return n
}

func (r *envReader) duration(key string, fallback, bareUnit time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	d, err := parseDuration(val, bareUnit)
	if err != nil {
		r.fail(fmt.Errorf("%s: %w", key, err))
		return fallback
	}
	return d
}

func stringOr(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func isTrue(key string) bool {
	return os.Getenv(key) == "true"
}

func splitList(val string) []string {
	var items []string
	for item := range strings.SplitSeq(val, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
