# Database Module

將 `corvauth` 的 device session 存放於 PostgreSQL

使用者帳號由你的應用程式管理，此 module 只以 email 參照 users table

## 需求

- **PostgreSQL 14+**
- **既有的 users table**：email column 須為 `PRIMARY KEY` 或 `UNIQUE`，供 foreign key 參照

## 安裝

```bash
go get github.com/lucap9056/corvauth/database
```

## 使用方式

```go
import "github.com/lucap9056/corvauth/database"

userRef, err := database.WithUserEmailReference("users(email)")
if err != nil {
	return err
}

db, err := database.NewDatabase(dsn,
	database.WithAutoCreateSchema(true),
	userRef,
)
if err != nil {
	return err
}
defer db.Close()
```

### 使用既有的 Connection Pool

已有連到同一個 PostgreSQL 的 `*sql.DB` 時改用 `New`：

```go
sqlDB, err := sql.Open("pgx", dsn)
if err != nil {
	return err
}
defer sqlDB.Close()

db, err := database.New(sqlDB, database.WithAutoCreateSchema(true))
if err != nil {
	return err
}
defer db.Close()
```

- `*sql.DB` 須使用 pgx stdlib driver
- `*sql.DB` 由呼叫端擁有，`Close()` 只停止 cleanup worker
- Connection pool option（`WithMaxOpenConns`、`WithMaxIdleConns`、`WithConnMaxLifetime`、`WithConnMaxIdleTime`）會被忽略

## Options

| Option | 預設值 | 說明 |
|---|---|---|
| `WithMaxOpenConns(n)` | `20` | 最大連線數（僅 `NewDatabase`） |
| `WithMaxIdleConns(n)` | `15` | 最大閒置連線數（僅 `NewDatabase`） |
| `WithConnMaxLifetime(d)` | `5m` | 連線最長存活時間（僅 `NewDatabase`） |
| `WithConnMaxIdleTime(d)` | `2m` | 連線最長閒置時間（僅 `NewDatabase`） |
| `WithCleanupInterval(d)` | `24h` | 清除 7 天未更新 device 的間隔，`0` 停用 |
| `WithAutoCreateSchema(bool)` | `false` | 啟動時建立 `auth_user_devices` table 與 index |
| `WithUserEmailReference(ref)` | `users(email)` | foreign key 參照的 users table 與 email column |

### User Email Reference

格式：`[<schema>.]<table>(<column>)[:<type>]`

| 範例 | 意義 |
|---|---|
| `users(email)` | `users.email`，type `text` |
| `users(email):citext` | `users.email`，type `citext` |
| `auth.members(mail):varchar(255)` | `auth.members.mail`，type `varchar(255)` |

- 不分大小寫，一律轉小寫
- `<type>` 須與被參照的 column type 一致

### Auto Create Schema

- 啟用：`NewDatabase` 與 `New` 建立 schema，需 DDL 權限
- 未啟用：以 `schema.Generate` 或下方 SQL 自行建立

## Schema

```sql
CREATE TABLE IF NOT EXISTS auth_user_devices (
    device_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_name TEXT NOT NULL,
    user_email TEXT NOT NULL,
    secret TEXT NOT NULL,
    generation INT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_auth_user_devices FOREIGN KEY (user_email)
        REFERENCES users (email)
        ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_auth_user_devices_user_email ON auth_user_devices(user_email);
CREATE INDEX IF NOT EXISTS idx_auth_user_devices_updated_at ON auth_user_devices(updated_at);
```

## API

| Method | 說明 |
|---|---|
| `SaveDeviceSecret(userEmail, deviceName, secret)` | 建立 device 並回傳 `device_id`，email 不在 users table 時回傳 `ErrUserNotFound` |
| `GetDeviceSecret(deviceID)` | 回傳 `secret` 與目前 `generation` |
| `UpdateDeviceSecret(deviceID)` | `generation` 加 1、更新 `updated_at`，回傳新 `generation` |
| `ListDevices(userEmail)` | 以 `UserDevice`（`DeviceID`、`DeviceName`、`UpdatedAt`）回傳該使用者所有 device，最近更新的在前 |
| `DeleteDevice(userEmail, deviceID)` | 刪除該使用者的單一 device |
| `DeleteAllDevices(userEmail)` | 刪除該使用者所有 device 並回傳 ID |
| `Close()` | 停止 cleanup worker，pool 由 `NewDatabase` 開啟時才一併關閉 |
