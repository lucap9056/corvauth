# Database Module

Stores `corvauth` device sessions in PostgreSQL

Your application owns user accounts; this module only references them by email

## Requirements

- **PostgreSQL 14+**
- **An existing users table**: email column must be `PRIMARY KEY` or `UNIQUE` for the foreign key

## Installation

```bash
go get github.com/lucap9056/corvauth/database
```

## Usage

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

### Using an Existing Connection Pool

If you already have a `*sql.DB` to the same PostgreSQL, use `New`:

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

- The `*sql.DB` must use the pgx stdlib driver
- The caller owns the `*sql.DB`; `Close()` only stops the cleanup worker
- Pool options (`WithMaxOpenConns`, `WithMaxIdleConns`, `WithConnMaxLifetime`, `WithConnMaxIdleTime`) are ignored

## Options

| Option | Default | Description |
|---|---|---|
| `WithMaxOpenConns(n)` | `20` | Max open connections (`NewDatabase` only) |
| `WithMaxIdleConns(n)` | `15` | Max idle connections (`NewDatabase` only) |
| `WithConnMaxLifetime(d)` | `5m` | Max connection lifetime (`NewDatabase` only) |
| `WithConnMaxIdleTime(d)` | `2m` | Max connection idle time (`NewDatabase` only) |
| `WithCleanupInterval(d)` | `24h` | Interval for deleting devices not updated in 7 days; `0` disables |
| `WithCleanupErrorHandler(fn)` | `log.Println` | Receives cleanup worker errors; `nil` discards them |
| `WithAutoCreateSchema(bool)` | `false` | Create `auth_user_devices` and its indexes on startup |
| `WithUserEmailReference(ref)` | `users(email)` | Users table and email column for the foreign key |

### User Email Reference

Format: `[<schema>.]<table>(<column>)[:<type>]`

| Example | Meaning |
|---|---|
| `users(email)` | `users.email`, type `text` |
| `users(email):citext` | `users.email`, type `citext` |
| `auth.members(mail):varchar(255)` | `auth.members.mail`, type `varchar(255)` |

- Case-insensitive, normalized to lowercase
- `<type>` must match the referenced column's type

### Auto Create Schema

- Enabled: `NewDatabase` and `New` create the schema; requires DDL privileges
- Disabled: create it yourself with `schema.Generate` or the SQL below

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

| Method | Description |
|---|---|
| `SaveDeviceSecret(userEmail, deviceName, secret)` | Creates a device and returns its `device_id`; `ErrUserNotFound` if the email is not in the users table |
| `GetDeviceSecret(deviceID)` | Returns `secret` and current `generation` |
| `UpdateDeviceSecret(deviceID)` | Increments `generation`, refreshes `updated_at`, returns the new `generation` |
| `ListDevices(userEmail)` | Returns the user's devices as `UserDevice` (`DeviceID`, `DeviceName`, `UpdatedAt`), most recently updated first |
| `DeleteDevice(userEmail, deviceID)` | Deletes one of the user's devices |
| `DeleteAllDevices(userEmail)` | Deletes all of the user's devices and returns their IDs |
| `Close()` | Stops the cleanup worker; closes the pool only if `NewDatabase` opened it |
