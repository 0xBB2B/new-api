# model-capabilities 实施计划
## 阶段 1：能力存储
- [01-capability-on-model-record](01-capability-on-model-record.md) — 模型记录新增能力列、规范化与校验、按名字解析
## 阶段 2：后端读写与单一来源
- [02-v1-models-capabilities](02-v1-models-capabilities.md) — /v1/models 返回能力 [依赖: 01-capability-on-model-record, 03-smartroute-single-source]
- [03-smartroute-single-source](03-smartroute-single-source.md) — 智能路由改读模型记录，删内置表与覆盖设置 [依赖: 01-capability-on-model-record]
- [04-capability-migration](04-capability-migration.md) — 一次性搬迁内置表与覆盖到模型记录 [依赖: 01-capability-on-model-record]
- [05-capabilities-json-api](05-capabilities-json-api.md) — 能力配置 JSON 读取、预览、整体替换接口 [依赖: 01-capability-on-model-record, 04-capability-migration]
## 阶段 3：前端
- [06-drawer-capabilities-section](06-drawer-capabilities-section.md) — 模型编辑抽屉「模型能力」分区 [依赖: 01-capability-on-model-record]
- [07-capabilities-json-dialog](07-capabilities-json-dialog.md) — 「能力配置 JSON」窗口 [依赖: 05-capabilities-json-api, 06-drawer-capabilities-section]
- [08-smart-routing-cleanup](08-smart-routing-cleanup.md) — 删除思考能力卡片，改提示文案 [依赖: 03-smartroute-single-source]
