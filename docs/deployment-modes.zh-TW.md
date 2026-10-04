# 部署模式

由 `DATABASE_URL` 與 `DB_USER_EMAIL_REFERENCE` 決定，比較表見 [README](../README.zh-TW.md#部署模式)

## Stateless proxy

輕量的 OAuth2.0 client：client secret、`state`、PKCE verifier 留在 server，最後把 provider token 交給 client

- 不需要 database
- `DB_*`、`JWT_*`、`ALLOW_REGISTRATION`、`ALLOW_UNVERIFIED_EMAIL` 無作用
- 下游 service 須自行驗證 provider token

## Managed users

本 server 管理 `users` table：

```sql
CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);
```

- `ALLOW_REGISTRATION=true`：首次登入的使用者會被 insert，`username` 取自 provider 顯示名稱
- 未開啟：只有已在 `users` 的 email 能登入
- `DB_AUTO_CREATE_SCHEMA` 非 `true` 時，table 須事先存在（啟動時檢查），見[建立 Schema](#建立-schema)

## External users

Users table 由你的應用程式管理，以 `DB_USER_EMAIL_REFERENCE` 指定位置，如 `public.accounts(email):citext`

- Email column 須為 `PRIMARY KEY` 或 `UNIQUE`
- 本 server 不寫入此 table，只有已存在的 email 能登入
- 刪除 user 時 session 透過 foreign key 一併刪除

`username` claim 要放顯示名稱，將 `DB_USER_USERNAME_COLUMN` 設為同 table 的 column

- 需要 `SELECT` 權限，啟動時檢查
- `NULL` 視為空字串

## 建立 Schema

在 server 啟動前先建立 table，讓依賴 `users` 的 service 自行控制啟動順序

- `corvauth schema apply`：建立 table 後結束，讀取 `DATABASE_URL` 與 `DB_*`
  > external users 須在 users table 建立後執行
- `corvauth schema print`：將 SQL 輸出到 stdout

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
