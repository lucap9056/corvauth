# User API

使用者以 email 識別。

[Managed users](../topics/deployment-modes.md#managed-users) 模式下存在本 server 的 `users` table。

[External users](../topics/deployment-modes.md#external-users) 模式下存在你自己的 table，本 server 不會寫入。

## 刪除目前使用者 % DELETE /users/me

刪除該使用者所有 device session 與使用者本身，並清除 cookie。

需要 access token。

::: warning
僅限 managed users 模式
:::
