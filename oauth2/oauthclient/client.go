package oauthclient

import (
	"context"
	"fmt"
	"slices"

	"github.com/lucap9056/corvauth/oauth2/providers"
	"golang.org/x/oauth2"
)

type Config struct {
	Provider     string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserinfoURL  string
	RevokeURL    string
	Scopes       []string
}

type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

type Client struct {
	config        *oauth2.Config
	provider      providers.Provider
	authOptions   []oauth2.AuthCodeOption
	issuer        string
	requireIssuer bool
}

func New(cfg Config, opts ...providers.Option) *Client {
	config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		},
	}

	provider := providers.New(cfg.Provider, config, cfg.UserinfoURL, cfg.RevokeURL, opts...)

	var authOptions []oauth2.AuthCodeOption
	if cfg.Provider == providers.GoogleName {
		authOptions = append(authOptions, oauth2.AccessTypeOffline)
	}

	return &Client{
		config:      config,
		provider:    provider,
		authOptions: authOptions,
	}
}

// NewOIDC builds a Client using OIDC discovery. It fetches the provider's
// well-known configuration document and auto-populates all endpoints.
// The "openid" scope is automatically added if not already present.
func NewOIDC(ctx context.Context, cfg OIDCConfig, opts ...providers.Option) (*Client, error) {
	discovery, err := providers.FetchDiscovery(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, err
	}

	scopes := cfg.Scopes
	if !slices.Contains(scopes, "openid") {
		scopes = append([]string{"openid"}, scopes...)
	}

	config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  discovery.AuthorizationEndpoint,
			TokenURL: discovery.TokenEndpoint,
		},
	}

	var authOptions []oauth2.AuthCodeOption
	if slices.Contains(scopes, "offline_access") {
		authOptions = append(authOptions, oauth2.SetAuthURLParam("prompt", "consent"))
	}

	return &Client{
		config:        config,
		provider:      providers.NewOIDCProvider(config, discovery, opts...),
		authOptions:   authOptions,
		issuer:        discovery.Issuer,
		requireIssuer: discovery.AuthorizationResponseIssParameterSupported,
	}, nil
}

func (c *Client) AuthURL(state, verifier, redirectURL string) string {
	options := append(slices.Clone(c.authOptions), oauth2.S256ChallengeOption(verifier))
	options = appendRedirectURL(options, redirectURL)
	return c.config.AuthCodeURL(state, options...)
}

func (c *Client) ValidateIssuer(iss string) error {
	if iss == "" {
		if c.requireIssuer {
			return fmt.Errorf("authorization response missing iss parameter")
		}
		return nil
	}
	if c.issuer != "" && iss != c.issuer {
		return fmt.Errorf("authorization response iss mismatch: got %q, want %q", iss, c.issuer)
	}
	return nil
}

func (c *Client) Exchange(ctx context.Context, code, verifier, redirectURL string) (*oauth2.Token, error) {
	options := appendRedirectURL([]oauth2.AuthCodeOption{oauth2.VerifierOption(verifier)}, redirectURL)
	return c.config.Exchange(ctx, code, options...)
}

func appendRedirectURL(options []oauth2.AuthCodeOption, redirectURL string) []oauth2.AuthCodeOption {
	if redirectURL == "" {
		return options
	}
	return append(options, oauth2.SetAuthURLParam("redirect_uri", redirectURL))
}

func (c *Client) GetUser(ctx context.Context, token *oauth2.Token) (*providers.Userinfo, error) {
	return c.provider.GetUser(ctx, token)
}

func (c *Client) Revoke(ctx context.Context, token *oauth2.Token) error {
	return c.provider.Revoke(ctx, token)
}
