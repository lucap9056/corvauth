package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
	"github.com/lucap9056/corvauth/server/internal/config"
	"github.com/lucap9056/corvauth/server/internal/flight"
	"github.com/lucap9056/corvauth/server/internal/handlers"
	"github.com/lucap9056/corvauth/server/internal/handlers/login"
	"github.com/lucap9056/corvauth/server/internal/handlers/options"
	"github.com/lucap9056/corvauth/server/internal/identity"
	"github.com/lucap9056/corvauth/server/internal/usersdb"
	"github.com/lucap9056/go-lifecycle/v2/lifecycle"
	"github.com/lucap9056/go-lifecycle/v2/runner"
)

func main() {
	if len(os.Args) > 1 {
		if err := runCommand(os.Args[1:], os.Stdout); err != nil {
			log.Fatalln(err)
		}
		return
	}

	if err := runner.Run(run); err != nil {
		log.Fatalln(err)
	}
}

func run(life *lifecycle.Coordinator) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
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
		log.Printf("[WARN] %s is ignored because %s is set", config.EnvAllowRegistration, config.EnvDBUserEmailReference)
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

	server := &http.Server{
		Handler:      mux,
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
			life.Exitln(err.Error())
		}
	}()

	life.OnExit(func() {
		log.Println("Shutting down server...")
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
		usersdb.WithDatabaseOptions(database.WithCleanupInterval(cfg.CleanupInterval)),
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
		log.Printf("Starting OIDC server (Issuer: %s) on %s (Mode: %s)", oauth2Config.OIDC.IssuerURL, cfg.HTTP.Address, cfg.HTTP.Mode)
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
	log.Printf("Starting OAuth2 server (Provider: %s) on %s (Mode: %s)", provider.Name, cfg.HTTP.Address, cfg.HTTP.Mode)
	return oauth2Client, nil
}

func createListener(addr string) (net.Listener, error) {
	if after, ok := strings.CutPrefix(addr, "unix://"); ok {
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
