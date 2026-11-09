# smart-routing-scorer-effort 实施计划
## 阶段 1：后端
- [01-scorer-effort-setting](01-scorer-effort-setting.md) — 打分配置新增思考力度字段，保存校验只接受留空或七档之一
- [02-scorer-request](02-scorer-request.md) — 打分请求按配置和模型能力发思考力度、去掉输出上限，格式不对时写服务器警告日志 [依赖: 01-scorer-effort-setting]
## 阶段 2：前端
- [03-capability-scorer-models](03-capability-scorer-models.md) — 「模型思考能力」卡片和内置能力查询把打分模型也算进去
- [04-scorer-effort-field](04-scorer-effort-field.md) — 编辑弹窗「打分」页签新增思考力度下拉，页签小字带上思考力度 [依赖: 01-scorer-effort-setting, 03-capability-scorer-models]
