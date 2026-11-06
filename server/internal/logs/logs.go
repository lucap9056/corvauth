package logs

import (
	"context"
	"net/http"
	"os"

	"github.com/lucap9056/corvauth/server/internal/clientip"
	"github.com/lucap9056/corvauth/server/internal/events"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Out *zap.Logger = zap.NewNop()

func Init(level zapcore.Level, jsonFormat bool) {
	Out = New(zapcore.Lock(os.Stdout), level, jsonFormat)
}

func New(w zapcore.WriteSyncer, level zapcore.Level, jsonFormat bool) *zap.Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	var encoder zapcore.Encoder
	if jsonFormat {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	return zap.New(zapcore.NewCore(encoder, w, level))
}

func Sync() {
	_ = Out.Sync()
}

type contextKey struct{}

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*zap.Logger); ok {
		return logger
	}
	return Out
}

func Middleware(resolver clientip.Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, source := resolver.Resolve(r)
		logger := Out.With(
			zap.String("clientIp", ip),
			zap.String("clientIpSource", string(source)),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)
		next.ServeHTTP(w, r.WithContext(WithLogger(r.Context(), logger)))
	})
}

func Event(logger *zap.Logger, event events.Event, reason, msg string, err error, fields ...zap.Field) {
	fields = append(fields, zap.String("event", string(event)), zap.String("reason", reason))
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	logger.Log(event.Level(), msg, fields...)
}

func Degraded(ctx context.Context, reason, msg string, err error) {
	Event(FromContext(ctx), events.Degraded, reason, msg, err)
}
