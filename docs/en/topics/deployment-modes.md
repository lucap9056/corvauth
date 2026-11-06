# Deployment Modes

Corvauth runs in one of three modes, selected by `DATABASE_URL` and `DB_USER_EMAIL_REFERENCE`.

Managed users and external users are together called **session modes**.

###### Deployment Mode Comparison

| | Stateless Proxy | Managed Users | External Users |
|---|---|---|---|
| Set when | `DATABASE_URL` unset | only `DATABASE_URL` set | both set |
| Users table | none | `users`, owned by this server | yours, referenced by email |
| Session tokens | no, provider tokens returned as-is | yes | yes |
| Session and verify endpoints | no | yes | yes |
| [Delete Current User](../resources/user.md#delete-users-me) | no | deletes the user and all sessions | no |
| `ALLOW_REGISTRATION` | ignored | inserts new users into `users` | ignored, warns on startup |
| `username` claim | — | `users.username` | `DB_USER_USERNAME_COLUMN`, or empty |
| `DB_AUTO_CREATE_SCHEMA=true` creates | — | `users`, `auth_user_devices` | `auth_user_devices` |

## Stateless Proxy

A lightweight OAuth2 client: the client secret, `state`, and PKCE verifier stay on the server, and the provider's tokens go to the client

- No database needed
- `DB_*`, `JWT_*`, `ALLOW_REGISTRATION`, and `ALLOW_UNVERIFIED_EMAIL` have no effect
- Only [Get Health](../resources/health.md#get-health), [Start Login](../resources/session.md#post-sessions), and [Complete Login](../resources/session.md#get-callback) are available

::: warning
Downstream services must validate the provider's tokens themselves
:::

## Managed Users

This server owns a `users` table:

```sql
CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);
```

- `ALLOW_REGISTRATION=true`: first-time users are inserted, with the provider's display name as `username`
- Otherwise: only emails already in `users` can sign in
- If `DB_AUTO_CREATE_SCHEMA` is not `true`, the table must already exist, checked on startup
  - See [Schema Initialization](#schema-initialization)

## External Users

Your application owns the users table.

Point to it with `DB_USER_EMAIL_REFERENCE`, e.g. `public.accounts(email):citext`

- The email column must be `PRIMARY KEY` or `UNIQUE`
- This server never writes to it
- Only existing emails can sign in
- Deleting a user removes their sessions through the foreign key

The reference format is described in [User Email Reference](../../../database/README.md#user-email-reference).

For a display name in the `username` claim, set `DB_USER_USERNAME_COLUMN` to a column of the same table

- Requires `SELECT` privilege, checked on startup
- `NULL` becomes an empty string

## Schema Initialization

Create the tables before the server starts, so services depending on `users` control the startup order

###### Schema Commands

| Command | Description |
|---|---|
| `corvauth schema apply` | Creates the tables, then exits<br>Reads `DATABASE_URL` and `DB_*` |
| `corvauth schema print` | Writes the SQL to stdout |

::: info
In external users mode, run `schema apply` after your users table exists
:::

###### Example Docker Compose

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
