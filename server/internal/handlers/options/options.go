package options

import (
	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/server/internal/identity"
	"github.com/lucap9056/corvauth/server/internal/usersdb"
)

type DB interface {
	SaveDeviceSecret(userEmail, deviceName, secret string) (string, error)
	ListDevices(userEmail string) ([]database.UserDevice, error)
	DeleteDevice(userEmail, deviceID string) error
	DeleteAllDevices(userEmail string) error
}

type UsersDB interface {
	CreateUser(username, email string) (*usersdb.User, error)
	GetUsername(email string) (string, error)
	DeleteUser(email string) error
	External() bool
}

type Options struct {
	DevMode              bool
	AllowRegistration    bool
	PassOAuthToken       bool
	ClientPKCE           bool
	DefaultRedirectURL   string
	AllowedRedirectURLs  []string
	IdentityHeaders      identity.Headers
	ProviderTokenHeaders ProviderTokenHeaders
}

type ProviderTokenHeaders struct {
	AccessToken  string
	RefreshToken string
}

var DefaultProviderTokenHeaders = ProviderTokenHeaders{
	AccessToken:  "X-Corvauth-Provider-Access-Token",
	RefreshToken: "X-Corvauth-Provider-Refresh-Token",
}

type Option func(*Options)

func New(opts ...Option) *Options {
	o := &Options{IdentityHeaders: identity.DefaultHeaders, ProviderTokenHeaders: DefaultProviderTokenHeaders}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithDevMode(enabled bool) Option {
	return func(c *Options) {
		c.DevMode = enabled
	}
}

func WithAllowRegistration(enabled bool) Option {
	return func(c *Options) {
		c.AllowRegistration = enabled
	}
}

func WithPassOAuthToken(enabled bool) Option {
	return func(c *Options) {
		c.PassOAuthToken = enabled
	}
}

func WithClientPKCE(enabled bool) Option {
	return func(c *Options) { c.ClientPKCE = enabled }
}

func WithIdentityHeaders(headers identity.Headers) Option {
	return func(c *Options) { c.IdentityHeaders = headers }
}

func WithProviderTokenHeaders(headers ProviderTokenHeaders) Option {
	return func(c *Options) { c.ProviderTokenHeaders = headers }
}

func WithRedirectURLs(defaultURL string, allowed []string) Option {
	return func(c *Options) {
		c.DefaultRedirectURL = defaultURL
		c.AllowedRedirectURLs = allowed
	}
}
