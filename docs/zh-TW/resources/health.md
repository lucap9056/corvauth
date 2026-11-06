# Health API

供 load balancer 與 orchestrator 使用的存活檢查，所有 [部署模式](../topics/deployment-modes.md) 皆提供。

## 取得健康狀態 % GET /health

Server 運作中時以 `200` 回傳純文字 `OK`，不需認證。
