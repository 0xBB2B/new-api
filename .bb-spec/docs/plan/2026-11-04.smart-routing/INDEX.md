# 智能路由 实施计划
## 阶段 1：配置
- [01-route-config](01-route-config.md) — 配置结构、注册、请求策略白名单
- [02-route-validation](02-route-validation.md) — 字段级检查、检查接口、保存时整体检查 [依赖: 01-route-config]
## 阶段 2：纯逻辑
- [03-request-inspection](03-request-inspection.md) — 识别用户新消息、取用户消息和特征、会话标识
- [04-tier-rules](04-tier-rules.md) — 解析打分结果、选档、顾问计数、降档保护 [依赖: 01-route-config]
- [05-session-store](05-session-store.md) — 会话记录读写
## 阶段 3：调用与接入
- [06-scorer-call](06-scorer-call.md) — 打分提示词、内部调用、计费与打分日志 [依赖: 03-request-inspection, 04-tier-rules]
- [07-distributor-routing](07-distributor-routing.md) — Distribute 接入路由 [依赖: 02-route-validation, 05-session-store, 06-scorer-call]
- [08-reasoning-cleanup](08-reasoning-cleanup.md) — 换渠道时清理加密思考内容 [依赖: 07-distributor-routing]
- [09-routing-log-fields](09-routing-log-fields.md) — 日志里的 smart_routing 字段 [依赖: 07-distributor-routing]
- [10-model-list](10-model-list.md) — /v1/models 显示虚拟模型 [依赖: 02-route-validation]
## 阶段 4：前端
- [11-frontend-config-page](11-frontend-config-page.md) — 智能路由分区、表格、保存 [依赖: 02-route-validation]
- [12-frontend-route-dialog](12-frontend-route-dialog.md) — 添加/编辑弹窗与字段报错 [依赖: 11-frontend-config-page]
- [13-frontend-log-display](13-frontend-log-display.md) — 日志列表小字与详情块 [依赖: 09-routing-log-fields]
