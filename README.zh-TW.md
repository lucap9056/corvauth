[English](README.md) | 繁體中文

# Corvauth

Device-bound session 認證 server

已有 reverse proxy 時，交給本專案處理驗證：

- **後端不碰認證**：proxy 透過 `/verify` 驗證，後端只需讀 header
- **直接對接 proxy**：支援 nginx `auth_request`、Traefik `forwardAuth`
- **Header 防偽造**：設定 `IDENTITY_JWT_SECRET` 後改傳簽章過的 identity token
- **整合部分 OAuth2.0**：Discord、GitHub、Google 與任意 OIDC provider
- **可沿用既有 users table**：以 email 關聯，不需搬資料

```
               ┌──────────────────────── server ────────────────────────┐
 browser ───▶   /login, /callback ──▶ OAuth2.0 / OIDC provider
 proxy   ───▶   /verify, /refresh ──▶ jwt ──▶ database ──▶ PostgreSQL
               └────────────────────────────────────────────────────────┘
```

- 每次登入建立一個 **device session**，有自己的 signing secret
  - 登出或刪除 device 只影響該 device 的 token
- 使用者以 **email** 識別
  - `auth_user_devices` 以 foreign key 參照 users table，刪除 user 時 session 一併刪除

## 快速開始

複製 [`docker/docker-compose.yml`](docker/docker-compose.yml)，替換 `change-me` 密碼、OAuth2.0 client credentials 與 redirect URL 後：

```bash
docker compose up -d
```

使用 image `ghcr.io/lucap9056/corvauth`，以 **managed users** 模式並開啟註冊，透過 GitHub 登入（其他 provider 見[設定](docs/configuration.zh-TW.md)）

### 開發

從原始碼 build，啟動 PostgreSQL、Redis、假 OIDC provider 與 server（`http://localhost:8080`）：

```bash
docker compose -f docker-dev/docker-compose.yml up --build -d
```

假 provider 的任何使用者都能登入

## 部署模式

由 `DATABASE_URL` 與 `DB_USER_EMAIL_REFERENCE` 決定：

| | Stateless proxy | Managed users | External users |
|---|---|---|---|
| **條件** | 未設 `DATABASE_URL` | 只設 `DATABASE_URL` | 兩者都設 |
| **Users table** | 無 | `users`，由本 server 管理 | 你的 table，以 email 參照 |
| **Session token** | 無，直接回傳 provider token | 有 | 有 |
| **`/refresh`、`/verify`、`/logout`** | ✗ | ✓ | ✓ |
| **`ALLOW_REGISTRATION`** | 無效 | 新使用者 insert 到 `users` | 無效（啟動時警告） |
| **`DELETE /users/me`** | ✗ | 刪除使用者與所有 session | ✗ |
| **`username` claim** | — | `users.username` | `DB_USER_USERNAME_COLUMN`，未設則為空 |
| **`DB_AUTO_CREATE_SCHEMA=true` 建立** | — | `users`、`auth_user_devices` | `auth_user_devices` |

## 文件

- [部署模式](docs/deployment-modes.zh-TW.md)：各模式的 users table 設定
- [Session](docs/sessions.zh-TW.md)：登入流程、token、rotation、provider token
- [API](docs/api.zh-TW.md)：endpoint 與 error header
- [設定](docs/configuration.zh-TW.md)：環境變數

## 需求

- PostgreSQL 14+（session 模式）
- Redis（選用，多實例時必須）
- 從原始碼建置：Go 1.27+

## 測試

在 `server/` 執行：

```bash
go test ./internal/... ./tests/
```

Unit 與 integration test 不需外部服務，其餘需要對應環境：

| 測試 | 需求 | 指令 |
|---|---|---|
| `internal/cache/device`、`internal/flight` 的 Redis test | `TEST_REDIS_URL` | `go test ./internal/...`（未設則 skip） |
| End-to-end | Compose 已啟動 | `go test ./tests/e2e/` |

## Library

| Module | 用途 |
|---|---|
| [jwt](./jwt/README.zh-TW.md) | 簽發與驗證 access / refresh token，每個 device 各自的 secret |
| [database](./database/README.zh-TW.md) | 把 device session 存進 PostgreSQL，以 email 關聯 users table |
| [oauth2](./oauth2/README.zh-TW.md) | OAuth2.0 / OIDC provider 與登入 client |
