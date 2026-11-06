package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/jwt"
	"github.com/lucap9056/corvauth/oauth2/oauthclient"
	"github.com/lucap9056/corvauth/oauth2/providers"
	"github.com/lucap9056/corvauth/server/internal/cache"
	"github.com/lucap9056/corvauth/server/internal/cache/device"
	"github.com/lucap9056/corvauth/server/internal/cache/state"
	"github.com/lucap9056/corvauth/server/internal/clientip"
	"github.com/lucap9056/corvauth/server/internal/config"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/flight"
	"github.com/lucap9056/corvauth/server/internal/handlers"
	"github.com/lucap9056/corvauth/server/internal/handlers/login"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/identity"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"github.com/lucap9056/corvauth/server/internal/usersdb"
	"github.com/lucap9056/go-lifecycle/v2/lifecycle"
	"github.com/lucap9056/go-lifecycle/v2/runner"
	"go.uber.org/zap"
)

const unixAddressPrefix = "unix://"

func main() {
	if len(os.Args) > 1 {
		if err := runCommand(os.Args[1:], os.Stdout); err != nil {
			exitWithError(err)
		}
		return
	}

	logConfig, err := config.LoadLog()
	if err != nil {
		exitWithError(fmt.Errorf("invalid configuration: %w", err))
	}
	logs.Init(logConfig.Level, logConfig.JSON())
	restoreStdLog := zap.RedirectStdLog(logs.Out.Named("stdlog"))

	err = runner.Run(run)
	restoreStdLog()
	if err != nil {
		logs.Out.Error("server stopped", zap.Error(err))
		logs.Sync()
		os.Exit(1)
	}
	logs.Sync()
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func run(life *lifecycle.Coordinator) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	clientIPResolver := clientip.NewResolver(cfg.HTTP.TrustedClientIPHeader)
	if strings.HasPrefix(cfg.HTTP.Address, unixAddressPrefix) && !clientIPResolver.TrustsHeader() {
		logs.Out.Warn("client IP cannot be resolved from a unix socket peer; set the trusted header so events carry clientIp",
			zap.String("address", cfg.HTTP.Address),
			zap.String("env", config.EnvTrustedClientIPHeader))
	}

	var store *usersdb.Store
	if cfg.Database != nil {
		store, err = openDatabase(life, cfg.Database)
		if err != nil {
			return err
		}
	}

	allowRegistration := cfg.Auth.AllowRegistration
	if allowRegistration && store != nil && store.External() {
		logs.Out.Warn("registration disabled for external user table",
			zap.String("ignored", config.EnvAllowRegistration),
			zap.String("because", config.EnvDBUserEmailReference))
		allowRegistration = false
	}

	var redisClient *cache.RedisClient
	var flightOptions []flight.Option
	if cfg.Redis != nil {
		redisClient, err = cache.NewRedisClient(cfg.Redis.URL)
		if err != nil {
			return fmt.Errorf("failed to connect to Redis: %w", err)
		}
		life.OnExit(redisClient.Close)
		flightOptions = append(flightOptions, flight.WithRedis(cfg.Redis.URL, 0))
	}

	deviceCache, err := device.NewSecretCache(redisClient)
	if err != nil {
		return fmt.Errorf("failed to create device secret cache: %w", err)
	}
	life.OnExit(func() { deviceCache.Close() })

	var jwtDB jwt.Database
	var authDB options.DB
	var usersDB options.UsersDB
	if store != nil {
		cachedDB := device.NewCachedDB(store.Database, deviceCache)
		jwtDB = cachedDB
		authDB = cachedDB
		usersDB = store
	}

	jwtManager := jwt.NewJWTManager(jwtDB,
		jwt.WithAccessTokenDuration(cfg.JWT.AccessTokenDuration),
		jwt.WithRefreshTokenDuration(cfg.JWT.RefreshTokenDuration),
		jwt.WithIssuer(cfg.JWT.Issuer),
		jwt.WithAudience(cfg.JWT.Audience),
		jwt.WithDeviceTagSecret(cfg.JWT.DeviceTagSecret),
	)

	var identitySigner *identity.Signer
	if cfg.JWT.IdentitySecret != "" {
		identitySigner = identity.NewSigner(cfg.JWT.IdentitySecret, cfg.JWT.Issuer, cfg.JWT.Audience)
	}

	oauth2Client, err := newOAuth2Client(cfg)
	if err != nil {
		return err
	}

	flightGroup, err := flight.New(flightOptions...)
	if err != nil {
		return fmt.Errorf("failed to create flight group: %w", err)
	}
	life.OnExit(flightGroup.Close)

	stateCache, err := state.NewCache(redisClient)
	if err != nil {
		return fmt.Errorf("failed to create state cache: %w", err)
	}

	authOptions := []options.Option{
		options.WithDevMode(cfg.HTTP.DevMode()),
		options.WithAllowRegistration(allowRegistration),
		options.WithPassOAuthToken(cfg.Auth.PassOAuthToken),
		options.WithClientPKCE(cfg.OAuth2 != nil && cfg.OAuth2.Client.PKCE),
		options.WithIdentityHeaders(cfg.IdentityHeaders),
		options.WithProviderTokenHeaders(cfg.Auth.ProviderTokenHeaders),
	}
	if cfg.OAuth2 != nil {
		authOptions = append(authOptions, options.WithRedirectURLs(cfg.OAuth2.Client.RedirectURL, cfg.OAuth2.Client.AllowedRedirectURLs))
	}

	mux := http.NewServeMux()

	handlers.RegisterRoutes(mux, handlers.Dependencies{
		DB:             authDB,
		UsersDB:        usersDB,
		JWTManager:     jwtManager,
		IdentitySigner: identitySigner,
		Flight:         flightGroup,
		StateCache:     stateCache,
		OAuth2Client:   oauth2Client,
		Options:        authOptions,
	})

	httpErrorLog, err := zap.NewStdLogAt(logs.Out.Named("http"), zap.WarnLevel)
	if err != nil {
		return fmt.Errorf("failed to create http error logger: %w", err)
	}

	server := &http.Server{
		Handler:      logs.Middleware(clientIPResolver, mux),
		ErrorLog:     httpErrorLog,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	listener, err := createListener(cfg.HTTP.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", cfg.HTTP.Address, err)
	}
	life.OnExit(func() { listener.Close() })

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logs.Out.Error("http server failed", zap.Error(err))
			life.Exit()
		}
	}()

	life.OnExit(func() {
		logs.Out.Info("shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	return nil
}

func openDatabase(life *lifecycle.Coordinator, cfg *config.Database) (*usersdb.Store, error) {
	sqlDB, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	life.OnExit(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	store, err := usersdb.New(sqlDB, usersOptions(cfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	life.OnExit(func() { store.Close() })

	return store, nil
}

func usersOptions(cfg *config.Database) []usersdb.Option {
	opts := []usersdb.Option{
		usersdb.WithAutoCreateSchema(cfg.AutoCreateSchema),
		usersdb.WithDatabaseOptions(
			database.WithCleanupInterval(cfg.CleanupInterval),
			database.WithCleanupErrorHandler(func(err error) {
				logs.Event(logs.Out, events.InternalError, events.ReasonDatabaseCleanupFailed, "database cleanup failed", err)
			}),
		),
	}
	if cfg.UserEmailReference != "" {
		opts = append(opts, usersdb.WithExternal(cfg.UserEmailReference, cfg.UserUsernameColumn))
	}
	return opts
}

func newOAuth2Client(cfg *config.Config) (login.OAuth2Client, error) {
	oauth2Config := cfg.OAuth2
	if oauth2Config == nil {
		return nil, nil
	}

	providerOptions := []providers.Option{
		providers.WithAllowUnverifiedEmail(cfg.Auth.AllowUnverifiedEmail),
		providers.WithDiscoveryRetryHandler(func(err error) {
			logs.Event(logs.Out, events.Degraded, events.ReasonOIDCDiscoveryRetry, "OIDC discovery failed, retrying", err)
		}),
	}

	if oauth2Config.OIDC != nil {
		discoveryCtx, discoveryCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer discoveryCancel()

		oidcClient, err := oauthclient.NewOIDC(discoveryCtx, oauthclient.OIDCConfig{
			IssuerURL:    oauth2Config.OIDC.IssuerURL,
			ClientID:     oauth2Config.Client.ID,
			ClientSecret: oauth2Config.Client.Secret,
			RedirectURL:  oauth2Config.Client.RedirectURL,
			Scopes:       oauth2Config.Scopes,
		}, providerOptions...)
		if err != nil {
			return nil, fmt.Errorf("OIDC setup failed: %w", err)
		}
		logs.Out.Info("starting OIDC server",
			zap.String("issuer", oauth2Config.OIDC.IssuerURL),
			zap.String("address", cfg.HTTP.Address),
			zap.String("mode", cfg.HTTP.Mode))
		return oidcClient, nil
	}

	provider := oauth2Config.Provider
	oauth2Client := oauthclient.New(oauthclient.Config{
		Provider:     provider.Name,
		ClientID:     oauth2Config.Client.ID,
		ClientSecret: oauth2Config.Client.Secret,
		RedirectURL:  oauth2Config.Client.RedirectURL,
		AuthURL:      provider.AuthURL,
		TokenURL:     provider.TokenURL,
		UserinfoURL:  provider.UserinfoURL,
		RevokeURL:    provider.RevokeURL,
		Scopes:       oauth2Config.Scopes,
	}, providerOptions...)
	logs.Out.Info("starting OAuth2 server",
		zap.String("provider", provider.Name),
		zap.String("address", cfg.HTTP.Address),
		zap.String("mode", cfg.HTTP.Mode))
	return oauth2Client, nil
}

func createListener(addr string) (net.Listener, error) {
	if after, ok := strings.CutPrefix(addr, unixAddressPrefix); ok {
		if err := os.MkdirAll(filepath.Dir(after), 0777); err != nil {
			return nil, err
		}
		temp := after + ".temp"
		os.Remove(temp)
		os.Remove(after)
		l, err := net.Listen("unix", temp)
		if err != nil {
			return nil, err
		}
		if err := os.Chmod(temp, 0666); err != nil {
			l.Close()
			os.Remove(temp)
			return nil, err
		}
		if err := os.Rename(temp, after); err != nil {
			l.Close()
			os.Remove(temp)
			return nil, err
		}
		return l, nil
	}
	return net.Listen("tcp", addr)
}
