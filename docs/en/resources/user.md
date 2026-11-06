# User API

Users are identified by email.

In [managed users](../topics/deployment-modes.md#managed-users) mode they live in this server's `users` table.

In [external users](../topics/deployment-modes.md#external-users) mode they live in your own table and this server never writes to it.

## Delete Current User % DELETE /users/me

Deletes all of the user's device sessions and the user, and clears the cookie.

Requires an access token.

::: warning
Managed users mode only
:::
