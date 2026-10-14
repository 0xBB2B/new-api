# user-utilization 实施计划
## 阶段 1：后端
- [01-utilization-api](01-utilization-api.md) — 管理员接口 GET /api/data/utilization，返回开通人数与四类人数
## 阶段 2：前端
- [02-user-analytics-default-tokens](02-user-analytics-default-tokens.md) — 用户分析默认选中 Tokens
- [03-utilization-section-ui](03-utilization-section-ui.md) — 利用率分区界面 [依赖: 01-utilization-api, 02-user-analytics-default-tokens（同改 index.tsx，须串行）]
