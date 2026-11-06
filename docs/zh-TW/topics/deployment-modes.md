# 部署模式

Corvauth 依 `DATABASE_URL` 與 `DB_USER_EMAIL_REFERENCE` 以三種模式之一運作。

Managed users 與 external users 合稱 **session 模式**。

###### 部署模式比較 {#deployment-mode-comparison}

| | Stateless Proxy | Managed Users | External Users |
|---|---|---|---|
| 啟用條件 | 未設 `DATABASE_URL` | 只設 `DATABASE_URL` | 兩者都設 |
| Users table | 無 | `users`，由本 server 管理 | 你的 table，以 email 參照 |
| Session token | 無，直接回傳 provider 的 token | 有 | 有 |
| Session 與 verify endpoint | 無 | 有 | 有 |
| [刪除目前使用者](../resources/user.md#delete-users-me) | 無 | 刪除該使用者與所有 session | 無 |
| `ALLOW_REGISTRATION` | 忽略 | 將新使用者寫入 `users` | 忽略，啟動時警告 |
| `username` claim | — | `users.username` | `DB_USER_USERNAME_COLUMN`，或空字串 |
| `DB_AUTO_CREATE_SCHEMA=true` 建立 | — | `users`、`auth_user_devices` | `auth_user_devices` |

## Stateless Proxy {#stateless-proxy}

輕量的 OAuth2 client：client secret、`state` 與 PKCE verifier 留在 server，provider 的 token 交給 client

- 不需要 database
- `DB_*`、`JWT_*`、`ALLOW_REGISTRATION`、`ALLOW_UNVERIFIED_EMAIL` 無作用
- 只提供 [取得健康狀態](../resources/health.md#get-health)、[開始登入](../resources/session.md#post-sessions)、[完成登入](../resources/session.md#get-callback)

::: warning
下游服務需自行驗證 provider 的 token
:::

## Managed Users {#managed-users}

本 server 擁有一張 `users` table：

```sql
CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);
```

- `ALLOW_REGISTRATION=true`：首次登入的使用者會被寫入，`username` 為 provider 的顯示名稱
- 否則：只有已在 `users` 的 email 能登入
- `DB_AUTO_CREATE_SCHEMA` 不為 `true` 時，table 必須已存在（啟動時檢查）
  - 見 [Schema 初始化](#schema-initialization)

## External Users {#external-users}

由你的應用程式擁有 users table。

以 `DB_USER_EMAIL_REFERENCE` 指定，例如 `public.accounts(email):citext`

- Email 欄位必須是 `PRIMARY KEY` 或 `UNIQUE`
- 本 server 不寫入該 table
- 只有已存在的 email 能登入
- 刪除使用者時，透過 foreign key 一併刪除其 session

參照格式見 [User Email Reference](../../../database/README.md#user-email-reference)。

要在 `username` claim 放顯示名稱時，將 `DB_USER_USERNAME_COLUMN` 設為同一張 table 的欄位

- 需要 `SELECT` 權限，啟動時檢查
- `NULL` 視為空字串

## Schema 初始化 {#schema-initialization}

在 server 啟動前建立 table，讓依賴 `users` 的服務能控制啟動順序

###### Schema 指令 {#schema-commands}

| 指令 | 說明 |
|---|---|
| `corvauth schema apply` | 建立 table 後結束<br>讀取 `DATABASE_URL` 與 `DB_*` |
| `corvauth schema print` | 將 SQL 輸出到 stdout |

::: info
External users 模式需在你的 users table 建立後才執行 `schema apply`
:::

###### Docker Compose 範例 {#example-docker-compose}

```yaml
corvauth-schema:
  image: ghcr.io/lucap9056/corvauth
  command: ["schema", "apply"]
  environment:
    DATABASE_URL: postgres://...

app:
  depends_on:
    corvauth-schema:
      condition: service_completed_successfully
```
