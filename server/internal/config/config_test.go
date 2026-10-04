package config

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/lucap9056/corvauth/database/schema"
)

var allEnvKeys = []string{
	EnvHTTPAddress, EnvHTTPMode, EnvDatabaseURL, EnvRedisURL,
	EnvDBMaxOpenConns, EnvDBMaxIdleConns, EnvDBConnMaxLifetime, EnvDBConnMaxIdleTime, EnvDBCleanupInterval,
	EnvDBAutoCreateSchema, EnvDBUserEmailReference, EnvDBUserUsernameColumn,
	EnvJWTAccessDuration, EnvJWTRefreshDuration, EnvJWTIssuer, EnvJWTAudience, EnvIdentityJWTSecret,
	EnvOAuth2Provider, EnvOAuth2ClientID, EnvOAuth2ClientSecret, EnvOAuth2RedirectURL,
	EnvOAuth2AuthURL, EnvOAuth2TokenURL, EnvOAuth2UserinfoURL, EnvOAuth2RevokeURL,
	EnvOAuth2Scopes, EnvOAuth2ClientPKCE, EnvOIDCIssuerURL,
	EnvAllowRegistration, EnvPassOAuthToken, EnvAllowUnverifiedEmail,
}

func setEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, key := range allEnvKeys {
		t.Setenv(key, values[key])
	}
}

func setClientEnv(values map[string]string) map[string]string {
	values[EnvOAuth2ClientID] = "client-id"
	values[EnvOAuth2ClientSecret] = "client-secret"
	values[EnvOAuth2RedirectURL] = "http://localhost/callback"
	return values
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, map[string]string{})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTP.Address != DefaultHTTPAddress {
		t.Errorf("HTTP.Address: got %q, want %q", cfg.HTTP.Address, DefaultHTTPAddress)
	}
	if cfg.HTTP.Mode != ModeProduction || cfg.HTTP.DevMode() {
		t.Errorf("HTTP.Mode: got %q, want %q", cfg.HTTP.Mode, ModeProduction)
	}
	if cfg.Database != nil {
		t.Errorf("Database: got %+v, want nil", cfg.Database)
	}
	if cfg.Redis != nil {
		t.Errorf("Redis: got %+v, want nil", cfg.Redis)
	}
	if cfg.OAuth2 != nil {
		t.Errorf("OAuth2: got %+v, want nil", cfg.OAuth2)
	}
	if cfg.Auth == nil || *cfg.Auth != (Auth{}) {
		t.Errorf("Auth: got %+v, want zero value", cfg.Auth)
	}
	expectedJWT := JWT{AccessTokenDuration: DefaultJWTAccessDuration, RefreshTokenDuration: DefaultJWTRefreshDuration}
	if *cfg.JWT != expectedJWT {
		t.Errorf("JWT: got %+v, want %+v", cfg.JWT, expectedJWT)
	}
}

func TestLoad_DatabaseDefaults(t *testing.T) {
	setEnv(t, map[string]string{EnvDatabaseURL: "postgres://localhost/auth"})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedDatabase := Database{
		URL:             "postgres://localhost/auth",
		MaxOpenConns:    DefaultDBMaxOpenConns,
		MaxIdleConns:    DefaultDBMaxIdleConns,
		ConnMaxLifetime: DefaultDBConnMaxLifetime,
		ConnMaxIdleTime: DefaultDBConnMaxIdleTime,
		CleanupInterval: DefaultDBCleanupInterval,
	}
	if cfg.Database == nil || *cfg.Database != expectedDatabase {
		t.Errorf("Database: got %+v, want %+v", cfg.Database, expectedDatabase)
	}
}

func TestLoad_DatabaseSettingsIgnoredWithoutURL(t *testing.T) {
	setEnv(t, map[string]string{EnvDBMaxOpenConns: "many"})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database != nil {
		t.Errorf("Database: got %+v, want nil", cfg.Database)
	}
}

func TestLoadDatabase_IgnoresUnrelatedSettings(t *testing.T) {
	setEnv(t, map[string]string{
		EnvDatabaseURL:       "postgres://localhost/auth",
		EnvJWTAccessDuration: "invalid",
		EnvOIDCIssuerURL:     "https://issuer.example.com",
	})

	db, err := LoadDatabase()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db == nil || db.URL != "postgres://localhost/auth" {
		t.Errorf("Database: got %+v", db)
	}
}

func TestLoad_DatabaseAndJWT(t *testing.T) {
	setEnv(t, map[string]string{
		EnvDatabaseURL:          "postgres://localhost/auth",
		EnvDBMaxOpenConns:       "50",
		EnvDBMaxIdleConns:       "0",
		EnvDBConnMaxLifetime:    "10",
		EnvDBConnMaxIdleTime:    "90s",
		EnvDBCleanupInterval:    "12",
		EnvDBAutoCreateSchema:   "true",
		EnvDBUserEmailReference: "auth.members(mail):citext",
		EnvDBUserUsernameColumn: " Display_Name ",
		EnvJWTAccessDuration:    "30m",
		EnvJWTRefreshDuration:   "1d12h",
		EnvJWTIssuer:            "https://auth.example.com",
		EnvJWTAudience:          "api",
		EnvIdentityJWTSecret:    "0123456789abcdef0123456789abcdef",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedDatabase := Database{
		URL:                "postgres://localhost/auth",
		MaxOpenConns:       50,
		MaxIdleConns:       0,
		ConnMaxLifetime:    10 * time.Minute,
		ConnMaxIdleTime:    90 * time.Second,
		CleanupInterval:    12 * time.Hour,
		AutoCreateSchema:   true,
		UserEmailReference: "auth.members(mail):citext",
		UserUsernameColumn: "display_name",
	}
	if cfg.Database == nil || *cfg.Database != expectedDatabase {
		t.Errorf("Database: got %+v, want %+v", cfg.Database, expectedDatabase)
	}
	expectedJWT := JWT{
		AccessTokenDuration:  30 * time.Minute,
		RefreshTokenDuration: 36 * time.Hour,
		Issuer:               "https://auth.example.com",
		Audience:             "api",
		IdentitySecret:       "0123456789abcdef0123456789abcdef",
	}
	if *cfg.JWT != expectedJWT {
		t.Errorf("JWT: got %+v, want %+v", cfg.JWT, expectedJWT)
	}
}

func TestLoad_NestedValues(t *testing.T) {
	setEnv(t, setClientEnv(map[string]string{
		EnvHTTPAddress:          "unix:///tmp/auth.sock",
		EnvHTTPMode:             "production",
		EnvDatabaseURL:          "postgres://localhost/auth",
		EnvRedisURL:             "redis://localhost:6379",
		EnvOAuth2Provider:       "github",
		EnvOAuth2Scopes:         " openid , ,email,",
		EnvOAuth2ClientPKCE:     "true",
		EnvAllowRegistration:    "true",
		EnvPassOAuthToken:       "false",
		EnvAllowUnverifiedEmail: "true",
	}))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTP.DevMode() {
		t.Error("expected production mode")
	}
	if cfg.Database.URL != "postgres://localhost/auth" {
		t.Errorf("Database.URL: got %q", cfg.Database.URL)
	}
	if cfg.Redis.URL != "redis://localhost:6379" {
		t.Errorf("Redis.URL: got %q", cfg.Redis.URL)
	}
	if cfg.OAuth2.Client.ID != "client-id" || !cfg.OAuth2.Client.PKCE {
		t.Errorf("OAuth2.Client: got %+v", cfg.OAuth2.Client)
	}
	if cfg.OAuth2.Provider == nil || cfg.OAuth2.Provider.Name != "github" {
		t.Errorf("OAuth2.Provider: got %+v", cfg.OAuth2.Provider)
	}
	if cfg.OAuth2.OIDC != nil {
		t.Errorf("OAuth2.OIDC: got %+v, want nil", cfg.OAuth2.OIDC)
	}
	if !slices.Equal(cfg.OAuth2.Scopes, []string{"openid", "email"}) {
		t.Errorf("Scopes: got %q, want [openid email]", cfg.OAuth2.Scopes)
	}
	expectedAuth := Auth{AllowRegistration: true, PassOAuthToken: false, AllowUnverifiedEmail: true}
	if *cfg.Auth != expectedAuth {
		t.Errorf("Auth: got %+v, want %+v", cfg.Auth, expectedAuth)
	}
}

func TestLoad_OIDCTakesPrecedence(t *testing.T) {
	setEnv(t, setClientEnv(map[string]string{
		EnvOIDCIssuerURL:  "https://issuer.example.com",
		EnvOAuth2Provider: "github",
	}))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OAuth2 == nil || cfg.OAuth2.OIDC == nil || cfg.OAuth2.OIDC.IssuerURL != "https://issuer.example.com" {
		t.Fatalf("OAuth2.OIDC: got %+v", cfg.OAuth2)
	}
	if cfg.OAuth2.Provider != nil {
		t.Errorf("OAuth2.Provider: got %+v, want nil", cfg.OAuth2.Provider)
	}
}

func TestLoad_Validation(t *testing.T) {
	cases := []struct {
		name        string
		env         map[string]string
		expectedErr error
	}{
		{
			name:        "oidc without client",
			env:         map[string]string{EnvOIDCIssuerURL: "https://issuer.example.com"},
			expectedErr: ErrOIDCMissingClient,
		},
		{
			name:        "oidc with client",
			env:         setClientEnv(map[string]string{EnvOIDCIssuerURL: "https://issuer.example.com"}),
			expectedErr: nil,
		},
		{
			name:        "generic without urls",
			env:         setClientEnv(map[string]string{EnvOAuth2Provider: "custom"}),
			expectedErr: ErrGenericProviderMissingURLs,
		},
		{
			name: "generic with urls",
			env: setClientEnv(map[string]string{
				EnvOAuth2AuthURL:     "https://provider.example.com/auth",
				EnvOAuth2TokenURL:    "https://provider.example.com/token",
				EnvOAuth2UserinfoURL: "https://provider.example.com/userinfo",
			}),
			expectedErr: nil,
		},
		{
			name:        "invalid integer",
			env:         map[string]string{EnvDatabaseURL: "postgres://localhost/auth", EnvDBMaxOpenConns: "many"},
			expectedErr: ErrInvalidInteger,
		},
		{
			name:        "negative integer",
			env:         map[string]string{EnvDatabaseURL: "postgres://localhost/auth", EnvDBMaxIdleConns: "-1"},
			expectedErr: ErrInvalidInteger,
		},
		{
			name:        "invalid duration",
			env:         map[string]string{EnvDatabaseURL: "postgres://localhost/auth", EnvDBConnMaxLifetime: "5 minutes"},
			expectedErr: ErrInvalidDuration,
		},
		{
			name:        "invalid user email reference",
			env:         map[string]string{EnvDatabaseURL: "postgres://localhost/auth", EnvDBUserEmailReference: "users"},
			expectedErr: schema.ErrInvalidUserEmailReference,
		},
		{
			name:        "username column without reference",
			env:         map[string]string{EnvDatabaseURL: "postgres://localhost/auth", EnvDBUserUsernameColumn: "name"},
			expectedErr: ErrUsernameColumnWithoutRef,
		},
		{
			name: "invalid username column",
			env: map[string]string{
				EnvDatabaseURL:          "postgres://localhost/auth",
				EnvDBUserEmailReference: "users(email)",
				EnvDBUserUsernameColumn: "name; drop",
			},
			expectedErr: ErrInvalidUsernameColumn,
		},
		{
			name:        "jwt duration without unit",
			env:         map[string]string{EnvJWTAccessDuration: "15"},
			expectedErr: ErrInvalidDuration,
		},
		{
			name:        "zero jwt duration",
			env:         map[string]string{EnvJWTRefreshDuration: "0d"},
			expectedErr: ErrNonPositiveDuration,
		},
		{
			name:        "short identity secret",
			env:         map[string]string{EnvIdentityJWTSecret: "too-short"},
			expectedErr: ErrIdentitySecretTooShort,
		},
		{
			name:        "builtin without urls",
			env:         setClientEnv(map[string]string{EnvOAuth2Provider: "google"}),
			expectedErr: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t, tc.env)

			_, err := Load()
			if !errors.Is(err, tc.expectedErr) {
				t.Fatalf("got error %v, want %v", err, tc.expectedErr)
			}
		})
	}
}
