package options

import "github.com/lucap9056/corvauth/server/internal/usersdb"

type DB interface {
	SaveDeviceSecret(userEmail, deviceName, secret string) (string, error)
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
	DevMode             bool
	AllowRegistration   bool
	PassOAuthToken      bool
	ClientPKCE          bool
	DefaultRedirectURL  string
	AllowedRedirectURLs []string
}

type Option func(*Options)

func New(opts ...Option) *Options {
	o := &Options{}
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

func WithRedirectURLs(defaultURL string, allowed []string) Option {
	return func(c *Options) {
		c.DefaultRedirectURL = defaultURL
		c.AllowedRedirectURLs = allowed
	}
}
