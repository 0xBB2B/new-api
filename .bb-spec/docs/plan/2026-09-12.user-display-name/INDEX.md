# user-display-name 实施计划

## 阶段 1：后端（列表接口附带 display_name）

- [user-names-lookup](01-user-names-lookup.md) — 按用户 ID 批量取 username/display_name 的 model 查询；看板按用户分组改用它
- [log-audit-flow-display-name](02-log-audit-flow-display-name.md) — 使用日志 / 审计日志 / 看板流向管理端响应附带 display_name [依赖: user-names-lookup]
- [task-display-name](03-task-display-name.md) — 任务日志管理端响应附带 display_name，tasksToDto 改批量回填 [依赖: user-names-lookup]

## 阶段 2：前端（名称主显、ID 进 tip）

- [user-identity-label](04-user-identity-label.md) — 名称解析纯函数 + UserIdentityLabel 共享组件
- [logs-user-columns](05-logs-user-columns.md) — 使用日志 / 任务日志用户列、移动端卡片、任务详情 [依赖: log-audit-flow-display-name, task-display-name, user-identity-label]
- [audit-user-display](06-audit-user-display.md) — 审计用户列与详情操作者行 [依赖: log-audit-flow-display-name, user-identity-label]
- [dashboard-user-labels](07-dashboard-user-labels.md) — 看板排行/趋势图标签与流向图用户节点 [依赖: log-audit-flow-display-name, user-identity-label]

并行提示：02 与 03 互不依赖、文件不重叠；05、06、07 三者互不依赖、文件不重叠。
