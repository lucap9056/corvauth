# Reference

## Base URL

Every endpoint is served from the address Corvauth listens on (`HTTP_ADDRESS`).

Paths in this documentation are relative to it:

```
https://auth.example.com/verify
```

Corvauth has no versioned path prefix.

The machine-readable [OpenAPI specification](../public/openapi.yaml) describes the same endpoints.

## Mode Availability

Not every endpoint exists in every [deployment mode](./topics/deployment-modes.md):

###### Endpoints By Mode

| Mode | Available Endpoints |
|---|---|
| Stateless proxy | [Get Health](./resources/health.md#get-health), [Start Login](./resources/session.md#post-sessions), [Complete Login](./resources/session.md#get-callback) |
| Managed users | All endpoints |
| External users | All endpoints except [Delete Current User](./resources/user.md#delete-users-me) |

Managed users and external users are together called **session modes**.

## Authentication

Endpoints accept one of the following credentials, as listed on each endpoint:

###### Credential Types

| Credential | Sent As | Accepted By |
|---|---|---|
| Access token | `Authorization: Bearer <access_token>` | [Get Device Sessions](./resources/session.md#get-sessions), [Delete Device Sessions](./resources/session.md#delete-sessions), [Delete Current User](./resources/user.md#delete-users-me), [Verify Access Token](./resources/verify.md#get-verify), [Verify Optional Access Token](./resources/verify.md#get-verify-optional) |
| Refresh token | `refresh_token` cookie, or JSON body `{ "refresh_token": "..." }` when the cookie is absent | [Rotate Refresh Token](./resources/session.md#post-sessions-current-tokens), [Create Access Token](./resources/session.md#post-sessions-current-access-token), [Delete Current Session](./resources/session.md#delete-sessions-current) |
| Refresh token (Bearer) | `refresh_token` cookie, or `Authorization: Bearer <refresh_token>` when the cookie is absent | [Get Current Session](./resources/session.md#get-sessions-current) |

A missing or invalid token returns `401` with `WWW-Authenticate: Bearer`.

See [Tokens](./topics/tokens.md) for lifetimes and rotation.

## Response Format

JSON responses are wrapped in an envelope:

###### Response Envelope Structure

| Field | Type | Description |
|---|---|---|
| success | boolean | Whether the request succeeded |
| message | string, object, or array | The result on success<br>An error description on failure |

When an endpoint "returns a [token pair](./resources/session.md#token-pair-object) object", that object is the `message` field.

###### Example Success Response

```json
{
  "success": true,
  "message": {
    "access_token": "...",
    "refresh_token": "..."
  }
}
```

###### Example Error Response

```json
{
  "success": false,
  "message": "Invalid or expired state"
}
```

The HTTP status code and the `X-Auth-Error` header carry the machine-readable cause.

See [Status Codes](./topics/status-codes.md).

## Optional Fields

Fields that may be omitted are suffixed with `?`:

| Field | Type |
|---|---|
| optional_field? | string |

## Timestamps

Timestamps are integers in Unix seconds, e.g. `1767225600`.

## Durations

Duration variables take one or more `<integer><unit>` segments, with units `d`, `h`, `m`, `s`:

| Example | Meaning |
|---|---|
| `30m` | 30 minutes |
| `7d` | 7 days |
| `1d12h` | 36 hours |

Some variables also accept a bare integer in a default unit, noted in [Configuration](./topics/configuration.md).

## Caching

Responses carrying tokens or session data set `Cache-Control: no-store`.
