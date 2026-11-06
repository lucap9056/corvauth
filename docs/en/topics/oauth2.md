# OAuth2

Corvauth signs users in through an OAuth2 or OIDC provider and turns the result into a [device session](../resources/session.md#device-session-object).

## Login Flow

1. The client calls [Start Login](../resources/session.md#post-sessions) and redirects the user to the returned `url`
   - `redirect_uri` in the JSON body picks a redirect URL from `OAUTH2_ALLOWED_REDIRECT_URLS`
   - Omitted: `OAUTH2_REDIRECT_URL`
   - The server generates a PKCE verifier and stores it with the redirect URL under a one-time `state`
   - The `state` lives in Redis if `REDIS_URL` is set, otherwise in memory
2. The provider redirects to the redirect URL, whose page passes `code` and `state` to [Complete Login](../resources/session.md#get-callback)
   - The server takes the verifier and redirect URL by `state`, then deletes it to prevent replay
   - The server exchanges the code
3. In session modes, the server reads the email, creates a device session, sets the `refresh_token` cookie, and returns a [token pair](../resources/session.md#token-pair-object)

::: info
By default the email must be verified by the provider (`email_verified`)

`ALLOW_UNVERIFIED_EMAIL=true` lifts this
:::

## Client PKCE

`OAUTH2_CLIENT_PKCE=true` is for clients that drive PKCE themselves:

1. [Start Login](../resources/session.md#post-sessions) also returns `verifier`
2. The client sends it back to [Complete Login](../resources/session.md#get-callback) in `X-PKCE-Verifier`
3. A missing or mismatched verifier returns `400`

## Providers

`OAUTH2_PROVIDER` selects a built-in provider.

Any other value uses the generic provider.

###### Provider Types

| Provider | `OAUTH2_PROVIDER` | Required Variables |
|---|---|---|
| Discord | `discord` | client credentials |
| GitHub | `github` | client credentials |
| Google | `google` | client credentials |
| Generic | any other name | client credentials, `OAUTH2_AUTH_URL`, `OAUTH2_TOKEN_URL`, `OAUTH2_USERINFO_URL` |
| OIDC | ignored | client credentials, `OIDC_ISSUER_URL` |

`OIDC_ISSUER_URL` switches to OIDC discovery, reads endpoints from `{issuer}/.well-known/openid-configuration`, and overrides `OAUTH2_PROVIDER`.

All variables are listed in [OAuth2 Variables](./configuration.md#oauth2).

## Provider Tokens

`PASS_OAUTH_TOKEN` decides what happens to the provider's own access and refresh tokens:

###### Provider Token Handling

| Mode | `PASS_OAUTH_TOKEN=false` (default) | `PASS_OAUTH_TOKEN=true` |
|---|---|---|
| Session modes | Revoked and discarded right after login | Returned in headers of [Complete Login](../resources/session.md#get-callback)<br>The client keeps them |
| Stateless proxy | Returned in the response body | Returned only in headers<br>`message` is `Logged in` |

###### Provider Token Headers

| Header | Name Set By |
|---|---|
| `X-Corvauth-Provider-Access-Token` | `PROVIDER_ACCESS_TOKEN_HEADER` |
| `X-Corvauth-Provider-Refresh-Token` | `PROVIDER_REFRESH_TOKEN_HEADER` |
