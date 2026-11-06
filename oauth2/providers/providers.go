package providers

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
)

type Userinfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

var (
	ErrMissingEmail    = errors.New("provider returned no email")
	ErrUnverifiedEmail = errors.New("provider email is not verified")
)

type Options struct {
	AllowUnverifiedEmail bool
	OnDiscoveryRetry     func(err error)
}

type Option func(*Options)

func WithAllowUnverifiedEmail(allowed bool) Option {
	return func(o *Options) {
		o.AllowUnverifiedEmail = allowed
	}
}

func WithDiscoveryRetryHandler(handler func(err error)) Option {
	return func(o *Options) {
		o.OnDiscoveryRetry = handler
	}
}

func DefaultDiscoveryRetryHandler(err error) {
	fmt.Printf("Failed to fetch discovery document: %v. Retrying...\n", err)
}

func (o Options) discoveryRetried(err error) {
	if o.OnDiscoveryRetry != nil {
		o.OnDiscoveryRetry(err)
	}
}

func newOptions(opts []Option) Options {
	o := Options{OnDiscoveryRetry: DefaultDiscoveryRetryHandler}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func (o Options) checkEmail(email string, verified bool) error {
	if email == "" {
		return ErrMissingEmail
	}
	if !verified && !o.AllowUnverifiedEmail {
		return ErrUnverifiedEmail
	}
	return nil
}

func revocableToken(token *oauth2.Token) string {
	if token.RefreshToken != "" {
		return token.RefreshToken
	}
	return token.AccessToken
}

type Provider interface {
	GetUser(ctx context.Context, token *oauth2.Token) (*Userinfo, error)
	Revoke(ctx context.Context, token *oauth2.Token) error
}

const (
	DiscordName = "discord"
	GitHubName  = "github"
	GoogleName  = "google"
	OIDCName    = "oidc"
)

func IsBuiltin(name string) bool {
	switch name {
	case DiscordName, GitHubName, GoogleName:
		return true
	}
	return false
}

func New(name string, config *oauth2.Config, userinfoURL, revokeURL string, opts ...Option) Provider {
	switch name {
	case DiscordName:
		return NewDiscordProvider(config, opts...)
	case GitHubName:
		return NewGitHubProvider(config, opts...)
	case GoogleName:
		return NewGoogleProvider(config, opts...)
	default:
		return NewGenericProvider(config, userinfoURL, revokeURL, opts...)
	}
}
