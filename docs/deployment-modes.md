# Deployment Modes

Selected by `DATABASE_URL` and `DB_USER_EMAIL_REFERENCE`, see the [comparison table](../README.md#deployment-modes)

## Stateless proxy

A lightweight OAuth2.0 client: the client secret, `state`, and PKCE verifier stay on the server, and the provider's tokens go to the client

- No database needed
- `DB_*`, `JWT_*`, `ALLOW_REGISTRATION`, `ALLOW_UNVERIFIED_EMAIL` have no effect
- Downstream services must validate the provider's tokens themselves

## Managed users

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
- If `DB_AUTO_CREATE_SCHEMA` is not `true`, the table must already exist (checked on startup); see [Schema Initialization](#schema-initialization)

## External users

Your application owns the users table; point to it with `DB_USER_EMAIL_REFERENCE`, e.g. `public.accounts(email):citext`

- The email column must be `PRIMARY KEY` or `UNIQUE`
- This server never writes to it; only existing emails can sign in
- Deleting a user removes their sessions through the foreign key

For a display name in the `username` claim, set `DB_USER_USERNAME_COLUMN` to a column of the same table

- Requires `SELECT` privilege, checked on startup
- `NULL` becomes an empty string

## Schema Initialization

Create tables before the server starts, so services depending on `users` control the startup order

- `corvauth schema apply`: creates the tables, then exits; reads `DATABASE_URL` and `DB_*`
  > external users: run after the users table exists
- `corvauth schema print`: writes the SQL to stdout

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
