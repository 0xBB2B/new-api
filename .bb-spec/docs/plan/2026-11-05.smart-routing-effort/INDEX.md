# smart-routing-effort 实施计划
## 阶段 1：后端配置与能力
- [01-route-config-schema](01-route-config-schema.md) — 配置新增接口列表与三种力度方式、能力覆盖选项、启动迁移、保存校验
- [02-thinking-capability-core](02-thinking-capability-core.md) — 内置思考能力表、覆盖合并、就近降档、内置能力查询接口 [依赖: 01-route-config-schema]
- [06-user-models-virtual](06-user-models-virtual.md) — /api/user/models 末尾补上虚拟模型，可按格式过滤 [依赖: 01-route-config-schema]
## 阶段 2：后端请求路径
- [03-effort-runtime](03-effort-runtime.md) — 请求时计算、收敛、改写思考力度，记进会话和日志 [依赖: 02-thinking-capability-core]
- [04-anthropic-routing](04-anthropic-routing.md) — Anthropic 格式请求接入路由，换渠道清理思考块 [依赖: 03-effort-runtime]
- [05-format-save-check](05-format-save-check.md) — 保存时拦下渠道拒收所勾格式的配置 [依赖: 04-anthropic-routing]
## 阶段 3：前端
- [08-group-order-compact](08-group-order-compact.md) — 分组顺序组件的一行紧凑样式与拖动排序
- [07-route-form-model](07-route-form-model.md) — 前端类型、表单校验、摘要与能力合并 [依赖: 02-thinking-capability-core]
- [09-route-editor-dialog](09-route-editor-dialog.md) — 虚拟模型编辑弹窗改版 [依赖: 07-route-form-model, 08-group-order-compact, 05-format-save-check]
- [10-routes-and-capability-cards](10-routes-and-capability-cards.md) — 表格摘要与标黄、模型思考能力卡片和覆盖弹窗 [依赖: 09-route-editor-dialog]
- [11-model-pickers-frontend](11-model-pickers-frontend.md) — CC Switch、密钥模型限制、演练场的虚拟模型 [依赖: 06-user-models-virtual]
- [12-log-effort-row](12-log-effort-row.md) — 日志详情显示思考力度 [依赖: 03-effort-runtime]
