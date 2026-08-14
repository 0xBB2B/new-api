# subscription-usage-admin-visibility 实施计划

## 阶段 1：后端端点

- [usage-endpoint](01-usage-endpoint.md) — 管理端只读端点：model 用量快照（含 saturated 判定）+ controller + 路由注册

## 阶段 2：前端展示

- [list-display](02-list-display.md) — 渠道列表打标：常量 + API 函数 + 用量 cell + 状态列触顶徽标 + i18n [依赖: usage-endpoint]
