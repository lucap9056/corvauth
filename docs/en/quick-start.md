# Quick Start

## Run With Docker Compose

Copy [`docker/docker-compose.yml`](../../docker/docker-compose.yml), replace the `change-me` password, the OAuth2 client credentials, and the redirect URL, then:

```bash
docker compose up -d
```

It runs the image `ghcr.io/lucap9056/corvauth` in [managed users](./topics/deployment-modes.md#managed-users) mode with registration on, signing in through GitHub

::: tip
Other providers are configured in [OAuth2 Variables](./topics/configuration.md#oauth2)
:::

## Development

Builds from source and starts PostgreSQL, Redis, a fake OIDC provider, and the server (`http://localhost:8080`):

```bash
docker compose -f docker-dev/docker-compose.yml up --build -d
```

Any fake provider user can sign in

## Requirements

| Dependency | Required | Notes |
|---|---|---|
| PostgreSQL 14+ | session modes | Not used by the [stateless proxy](./topics/deployment-modes.md#stateless-proxy) |
| Redis | multiple instances | Optional for a single instance, see `REDIS_URL` in [Server Variables](./topics/configuration.md#server) |
| Go 1.27+ | building from source | — |
