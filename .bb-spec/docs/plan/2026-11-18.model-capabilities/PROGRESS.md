# 执行进度
| 序号 | Plan | 状态 | 完成时间 |
|---|---|---|---|
| 01 | capability-on-model-record | done | 2026-11-18 |
| 02 | v1-models-capabilities | pending | — |
| 03 | smartroute-single-source | pending | — |
| 04 | capability-migration | pending | — |
| 05 | capabilities-json-api | pending | — |
| 06 | drawer-capabilities-section | pending | — |
| 07 | capabilities-json-dialog | pending | — |
| 08 | smart-routing-cleanup | pending | — |
## 当前
准备执行 `03-smartroute-single-source.md`。待办：三库（SQLite/MySQL/PostgreSQL）验证合并到 05 完成后统一跑；06 补前端 locale 时要包含后端 5 个能力错误 key。
## 阻塞
（无）
## 重试与自修（每次记一条）
- 01 自修第 1 轮：同规则多条命中按 id 排序（含模型广场同类位置）、错误路径与 key、读库失败保留旧快照（含供应商同类位置）、删多余 BumpSyncTopic 与复述注释、补同步主题与错误参数测试 — 结果：复查 5 条修好，剩 2 条（读库失败测试未触及新分支、类型错误 key 参数为英文）
- 01 自修第 2 轮：读库失败测试改为 abilities 成功而 models 查询失败；类型错误 key 拆成两个固定 key — 结果：复查全部通过
## 无关发现（发现一条记一条）
（无）
