package events

import "go.uber.org/zap/zapcore"

type Event string

const (
	Rejected        Event = "rejected"
	Unauthenticated Event = "unauthenticated"
	InvalidRequest  Event = "invalid_request"
	Denied          Event = "denied"
	UpstreamError   Event = "upstream_error"
	InternalError   Event = "internal_error"
	Degraded        Event = "degraded"
)

func (e Event) Level() zapcore.Level {
	switch e {
	case Unauthenticated, Denied:
		return zapcore.InfoLevel
	case UpstreamError, InternalError:
		return zapcore.ErrorLevel
	default:
		return zapcore.WarnLevel
	}
}

const (
	ReasonTokenMissing        = "token_missing"
	ReasonTokenInvalid        = "token_invalid"
	ReasonRefreshTokenMissing = "refresh_token_missing"
	ReasonRefreshTokenInvalid = "refresh_token_invalid"
	ReasonRefreshTokenReused  = "refresh_token_reused"

	ReasonBodyInvalid           = "body_invalid"
	ReasonRedirectURIMissing    = "redirect_uri_missing"
	ReasonRedirectURINotAllowed = "redirect_uri_not_allowed"
	ReasonStateMissing          = "state_missing"
	ReasonStateInvalid          = "state_invalid"
	ReasonCodeMissing           = "code_missing"
	ReasonPKCEInvalid           = "pkce_invalid"

	ReasonUserNotFound  = "user_not_found"
	ReasonProviderError = "provider_error"

	ReasonProviderTimeout    = "provider_timeout"
	ReasonCodeExchangeFailed = "code_exchange_failed"
	ReasonUserinfoFailed     = "userinfo_failed"

	ReasonSessionInitFailed    = "session_init_failed"
	ReasonStateLookupFailed    = "state_lookup_failed"
	ReasonUserCreateFailed     = "user_create_failed"
	ReasonUserLookupFailed     = "user_lookup_failed"
	ReasonDeviceRegisterFailed = "device_register_failed"
	ReasonDeviceDeleteFailed   = "device_delete_failed"
	ReasonTokenIssueFailed     = "token_issue_failed"
	ReasonRefreshRotateFailed  = "refresh_rotate_failed"
	ReasonSessionListFailed    = "session_list_failed"
	ReasonSessionRevokeFailed  = "session_revoke_failed"
	ReasonAccountDeleteFailed  = "account_delete_failed"
	ReasonIdentitySignFailed   = "identity_sign_failed"
	ReasonUnexpectedResult     = "unexpected_result"

	ReasonFlightRedisUnavailable = "flight_redis_unavailable"
	ReasonFlightLockUnavailable  = "flight_lock_unavailable"
	ReasonFlightResultCorrupt    = "flight_result_corrupt"
	ReasonCacheRedisFailed       = "cache_redis_failed"
	ReasonDatabaseCleanupFailed  = "database_cleanup_failed"
	ReasonOIDCDiscoveryRetry     = "oidc_discovery_retry"
)

var knownProviderErrors = map[string]struct{}{
	"access_denied":             {},
	"invalid_request":           {},
	"unauthorized_client":       {},
	"unsupported_response_type": {},
	"invalid_scope":             {},
	"server_error":              {},
	"temporarily_unavailable":   {},
}

func ProviderReason(providerError string) string {
	if _, ok := knownProviderErrors[providerError]; ok {
		return "provider_" + providerError
	}
	return ReasonProviderError
}
