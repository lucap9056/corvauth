# 快速開始

## 以 Docker Compose 啟動 {#run-with-docker-compose}

複製 [`docker/docker-compose.yml`](../../docker/docker-compose.yml)，替換 `change-me` 密碼、OAuth2 client credential 與 redirect URL，接著：

```bash
docker compose up -d
```

使用 image `ghcr.io/lucap9056/corvauth`，以 [managed users](./topics/deployment-modes.md#managed-users) 模式並開啟註冊，透過 GitHub 登入

::: tip
其他 provider 的設定見 [OAuth2 變數](./topics/configuration.md#oauth2)
:::

## 開發環境 {#development}

從原始碼 build，並啟動 PostgreSQL、Redis、假的 OIDC provider 與 server（`http://localhost:8080`）：

```bash
docker compose -f docker-dev/docker-compose.yml up --build -d
```

假 provider 的任何使用者都能登入

## 需求 {#requirements}

| 相依套件 | 何時需要 | 備註 |
|---|---|---|
| PostgreSQL 14+ | session 模式 | [Stateless proxy](./topics/deployment-modes.md#stateless-proxy) 不使用 |
| Redis | 多實例 | 單一實例可不用<br>見 [Server 變數](./topics/configuration.md#server) 的 `REDIS_URL` |
| Go 1.27+ | 從原始碼 build | — |
