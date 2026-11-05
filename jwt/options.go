package jwt

import (
	"time"
)

type options struct {
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	Issuer               string
	Audience             string
	DeviceTagSecret      []byte
}

type Option func(*options)

func defaultOptions() *options {
	return &options{
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
	}
}

func WithAccessTokenDuration(d time.Duration) Option {
	return func(o *options) {
		o.AccessTokenDuration = d
	}
}

func WithRefreshTokenDuration(d time.Duration) Option {
	return func(o *options) {
		o.RefreshTokenDuration = d
	}
}

func WithIssuer(issuer string) Option {
	return func(o *options) {
		o.Issuer = issuer
	}
}

func WithAudience(audience string) Option {
	return func(o *options) {
		o.Audience = audience
	}
}

func WithDeviceTagSecret(secret string) Option {
	return func(o *options) {
		o.DeviceTagSecret = []byte(secret)
	}
}
