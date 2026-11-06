# Intro

Corvauth is a device-bound session authentication server that sits next to your reverse proxy.

- The proxy asks Corvauth whether a request is signed in
- Backends only read the identity headers it returns

## Features

- **Backends skip auth**
  - The proxy calls [Verify Access Token](./resources/verify.md#get-verify)
  - Backends only read headers
- **Plugs into the proxy**
  - Works with nginx `auth_request` and Traefik `forwardAuth`
- **Tamper-proof headers**
  - With `IDENTITY_JWT_SECRET`, a signed [identity token](./resources/verify.md#identity-token-object) is sent instead
- **Built-in OAuth2 providers**
  - Discord, GitHub, Google, and any OIDC provider
  - See [OAuth2](./topics/oauth2.md)
- **Reuse your users table**
  - Linked by email, no data migration
  - See [External Users](./topics/deployment-modes.md#external-users)

## How It Works

```
               ┌──────────────────────────── server ─────────────────────────────┐
 browser ───▶   POST /sessions, /sessions/callback ──▶ OAuth2.0 / OIDC provider
 proxy   ───▶   /verify, /sessions ──▶ jwt ──▶ database ──▶ PostgreSQL
               └─────────────────────────────────────────────────────────────────┘
```

- Every login creates a [device session](./resources/session.md#device-session-object) with its own signing secret
  - Logging out or deleting a device only affects that device's tokens
- Users are identified by **email**
  - `auth_user_devices` references the users table by foreign key
  - Deleting a user removes their sessions

## Where To Go Next

| Page | Covers |
|---|---|
| [Quick Start](./quick-start.md) | Running the server with Docker Compose |
| [Reference](./reference.md) | Authentication, response format, and conventions shared by every endpoint |
| [Deployment Modes](./topics/deployment-modes.md) | Stateless proxy, managed users, and external users |
| [Configuration](./topics/configuration.md) | Every environment variable |
| [Session API](./resources/session.md) | Login, token rotation, and device session endpoints |
