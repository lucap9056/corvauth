package providers

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type OIDCDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	RevocationEndpoint    string `json:"revocation_endpoint"`

	AuthorizationResponseIssParameterSupported bool `json:"authorization_response_iss_parameter_supported"`
}

// audienceClaim handles both "aud":"string" and "aud":["string"] in ID tokens.
type audienceClaim []string

func (a *audienceClaim) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*a = audienceClaim{s}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*a = audienceClaim(arr)
	return nil
}

type flexibleBool bool

func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	var boolean bool
	if err := json.Unmarshal(data, &boolean); err == nil {
		*b = flexibleBool(boolean)
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*b = str == "true"
	return nil
}

type idTokenClaims struct {
	Sub           string        `json:"sub"`
	Email         string        `json:"email"`
	EmailVerified flexibleBool  `json:"email_verified"`
	Name          string        `json:"name"`
	Iss           string        `json:"iss"`
	Aud           audienceClaim `json:"aud"`
	Exp           int64         `json:"exp"`
}

// FetchDiscovery retrieves the OIDC discovery document from the issuer's
// well-known endpoint.
func FetchDiscovery(ctx context.Context, issuerURL string, opts ...Option) (*OIDCDiscovery, error) {
	options := newOptions(opts)
	discoveryURL := strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled while fetching discovery document: %w", ctx.Err())
		default:
			body, retryable, err := tryFetchDiscovery(ctx, discoveryURL)
			if err == nil {
				var doc OIDCDiscovery
				if err := json.Unmarshal(body, &doc); err != nil {
					return nil, fmt.Errorf("failed to decode discovery document: %w", err)
				}

				if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
					return nil, fmt.Errorf("discovery document missing required endpoints")
				}
				if doc.Issuer == "" {
					return nil, fmt.Errorf("discovery document missing issuer")
				}
				return &doc, nil
			}

			if !retryable {
				return nil, err
			}

			options.discoveryRetried(err)
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func tryFetchDiscovery(ctx context.Context, discoveryURL string) (body []byte, retryable bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create discovery request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("failed to fetch discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests
		return nil, retryable, fmt.Errorf("discovery endpoint returned status: %s", resp.Status)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("failed to read discovery response body: %w", err)
	}

	return body, false, nil
}

type OIDCProvider struct {
	config     *oauth2.Config
	options    Options
	discovery  *OIDCDiscovery
	httpClient *http.Client
}

func NewOIDCProvider(config *oauth2.Config, discovery *OIDCDiscovery, opts ...Option) *OIDCProvider {
	return &OIDCProvider{config: config, options: newOptions(opts), discovery: discovery, httpClient: http.DefaultClient}
}

// GetUser extracts user info from the id_token JWT payload when available,
// falling back to the userinfo endpoint if claims are absent or incomplete.
func (p *OIDCProvider) GetUser(ctx context.Context, token *oauth2.Token) (*Userinfo, error) {
	if idToken, ok := token.Extra("id_token").(string); ok && idToken != "" {
		claims, err := parseIDTokenClaims(idToken)
		if err == nil {
			if err := p.validateIDTokenClaims(claims); err != nil {
				return nil, err
			}
		}
		if err == nil && claims.Sub != "" && p.options.checkEmail(claims.Email, bool(claims.EmailVerified)) == nil {
			return &Userinfo{ID: claims.Sub, Email: claims.Email, Name: claims.Name}, nil
		}
	}

	if p.discovery.UserinfoEndpoint == "" {
		return nil, fmt.Errorf("no userinfo endpoint available and id_token claims insufficient")
	}

	return p.fetchUserinfo(ctx, token)
}

func (p *OIDCProvider) validateIDTokenClaims(claims *idTokenClaims) error {
	if claims.Iss != p.discovery.Issuer {
		return fmt.Errorf("id_token issuer mismatch: got %q, want %q", claims.Iss, p.discovery.Issuer)
	}
	if !slices.Contains(claims.Aud, p.config.ClientID) {
		return fmt.Errorf("id_token audience %v does not contain client_id %q", []string(claims.Aud), p.config.ClientID)
	}
	return nil
}

func (p *OIDCProvider) fetchUserinfo(ctx context.Context, token *oauth2.Token) (*Userinfo, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get(p.discovery.UserinfoEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned status: %s", resp.Status)
	}

	var claims idTokenClaims
	if err := json.UnmarshalRead(resp.Body, &claims); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}

	if claims.Sub == "" {
		return nil, fmt.Errorf("userinfo response missing required claim: sub")
	}
	if err := p.options.checkEmail(claims.Email, bool(claims.EmailVerified)); err != nil {
		return nil, err
	}

	return &Userinfo{ID: claims.Sub, Email: claims.Email, Name: claims.Name}, nil
}

func (p *OIDCProvider) Revoke(ctx context.Context, token *oauth2.Token) error {
	if p.discovery.RevocationEndpoint == "" {
		return fmt.Errorf("revocation endpoint not available for this provider")
	}

	data := url.Values{}
	data.Set("token", revocableToken(token))
	data.Set("client_id", p.config.ClientID)
	data.Set("client_secret", p.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.discovery.RevocationEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send revoke request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("revoke failed with status: %d", resp.StatusCode)
	}

	return nil
}

func parseIDTokenClaims(idToken string) (*idTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid id_token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode id_token payload: %w", err)
	}

	var claims idTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal id_token claims: %w", err)
	}

	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("id_token is expired")
	}

	return &claims, nil
}
