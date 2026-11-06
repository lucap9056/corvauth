# 簡介

Corvauth 是 device-bound session 認證 server，搭配既有的 reverse proxy 使用。

- Proxy 向 Corvauth 確認 request 是否已登入
- 後端只需讀取回傳的 identity header

## 特色 {#features}

- **後端不碰認證**
  - Proxy 呼叫 [驗證 Access Token](./resources/verify.md#get-verify)
  - 後端只需讀 header
- **直接對接 proxy**
  - 支援 nginx `auth_request`、Traefik `forwardAuth`
- **Header 防偽造**
  - 設定 `IDENTITY_JWT_SECRET` 後改傳簽章過的 [identity token](./resources/verify.md#identity-token-object)
- **內建 OAuth2 provider**
  - Discord、GitHub、Google 與任意 OIDC provider
  - 見 [OAuth2](./topics/oauth2.md)
- **可沿用既有 users table**
  - 以 email 關聯，不需搬資料
  - 見 [External Users](./topics/deployment-modes.md#external-users)

## 運作方式 {#how-it-works}

```
               ┌──────────────────────── server ────────────────────────┐
 browser ───▶   POST /sessions, /callback ──▶ OAuth2.0 / OIDC provider
 proxy   ───▶   /verify, /sessions ──▶ jwt ──▶ database ──▶ PostgreSQL
               └────────────────────────────────────────────────────────┘
```

- 每次登入建立一個 [device session](./resources/session.md#device-session-object)，有自己的 signing secret
  - 登出或刪除 device 只影響該 device 的 token
- 使用者以 **email** 識別
  - `auth_user_devices` 以 foreign key 參照 users table
  - 刪除 user 時 session 一併刪除

## 下一步 {#where-to-go-next}

| 頁面 | 內容 |
|---|---|
| [快速開始](./quick-start.md) | 以 Docker Compose 啟動 server |
| [Reference](./reference.md) | 所有 endpoint 共用的認證方式、回應格式與慣例 |
| [部署模式](./topics/deployment-modes.md) | Stateless proxy、managed users、external users |
| [設定](./topics/configuration.md) | 所有環境變數 |
| [Session API](./resources/session.md) | 登入、token rotation 與 device session endpoint |
