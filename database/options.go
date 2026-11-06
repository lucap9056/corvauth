package database

import (
	"log"
	"time"

	"github.com/lucap9056/corvauth/database/schema"
)

type options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	CleanupInterval time.Duration
	OnCleanupError  func(error)

	Schema           *schema.Params
	AutoCreateSchema bool
}

type Option func(*options)

func defaultOptions() *options {
	return &options{
		MaxOpenConns:    20,
		MaxIdleConns:    15,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
		CleanupInterval: 24 * time.Hour,
		OnCleanupError:  logCleanupError,
		Schema:          schema.DefaultParams(),
	}
}

func newOptions(opts []Option) *options {
	cfg := defaultOptions()
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func WithMaxOpenConns(n int) Option {
	return func(o *options) {
		o.MaxOpenConns = n
	}
}

func WithMaxIdleConns(n int) Option {
	return func(o *options) {
		o.MaxIdleConns = n
	}
}

func WithCleanupInterval(d time.Duration) Option {
	return func(o *options) {
		o.CleanupInterval = d
	}
}

func WithCleanupErrorHandler(handler func(error)) Option {
	return func(o *options) {
		o.OnCleanupError = handler
	}
}

func logCleanupError(err error) {
	log.Println("Cleanup error:", err.Error())
}

func WithConnMaxLifetime(d time.Duration) Option {
	return func(o *options) {
		o.ConnMaxLifetime = d
	}
}

func WithConnMaxIdleTime(d time.Duration) Option {
	return func(o *options) {
		o.ConnMaxIdleTime = d
	}
}

func WithAutoCreateSchema(enabled bool) Option {
	return func(o *options) {
		o.AutoCreateSchema = enabled
	}
}

func WithUserEmailReference(reference string) (Option, error) {
	params, err := schema.ParseUserEmailReference(reference)
	if err != nil {
		return nil, err
	}

	return func(o *options) {
		o.Schema = params
	}, nil
}
