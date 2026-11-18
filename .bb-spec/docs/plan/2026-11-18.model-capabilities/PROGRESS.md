# 执行进度
| 序号 | Plan | 状态 | 完成时间 |
|---|---|---|---|
| 01 | capability-on-model-record | done | 2026-11-18 |
| 02 | v1-models-capabilities | done | 2026-11-18 |
| 03 | smartroute-single-source | done | 2026-11-18 |
| 04 | capability-migration | done | 2026-11-18 |
| 05 | capabilities-json-api | done | 2026-11-18 |
| 06 | drawer-capabilities-section | pending | — |
| 07 | capabilities-json-dialog | pending | — |
| 08 | smart-routing-cleanup | pending | — |
## 当前
准备执行 `06-drawer-capabilities-section.md` 与 `08-smart-routing-cleanup.md`（并行）。剩余：真实程序启动的全新库/升级库各启动两次验证（三库）。待办：三库（SQLite/MySQL/PostgreSQL）验证合并到 05 完成后统一跑；06 补前端 locale 时要包含后端能力错误 key 共 7 条：01 的 5 条（unknown value / must be an integer / is not a capability field / must be an object / must be an array of strings）+ 05 的 2 条（Model name must be 1 to 128 characters、{{name}} is used by a prefix, suffix, or contains rule record. Edit that record in the drawer.），以及 409 文案 Model metadata changed. Preview again before saving.
## 阻塞
（无）
## 重试与自修（每次记一条）
- 01 自修第 1 轮：同规则多条命中按 id 排序（含模型广场同类位置）、错误路径与 key、读库失败保留旧快照（含供应商同类位置）、删多余 BumpSyncTopic 与复述注释、补同步主题与错误参数测试 — 结果：复查 5 条修好，剩 2 条（读库失败测试未触及新分支、类型错误 key 参数为英文）
- 01 自修第 2 轮：读库失败测试改为 abilities 成功而 models 查询失败；类型错误 key 拆成两个固定 key — 结果：复查全部通过
- 03 实现重试第 1 次：推算规则误删了"relaykit 判定不支持思考的老 Gemini 模型"分支，补测试并恢复 — 结果：通过
- 05 自修第 1 轮：模型名 1~128 字符校验（含 CreateModelMeta 长度同类位置）、与非完全匹配记录同名报错（搬迁同类位置跳过）、更新与清空写 updated_time（搬迁同类位置）、内联单调用辅助函数、删复述注释、补排序/前缀能力不变/复制来源优先级/来源被删 409 测试 — 结果：复查全部通过；新增能力测试已三库化，SQLite / MySQL 5.7.44 / PostgreSQL 9.6 全部通过
- 04 自修第 1 轮：覆盖里不认识的档位值与空/超长模型名记日志跳过、删两条不合规注释、补对应测试；PRD 04 C6 按用户决定改为只按原名一致（日期后缀不兜底）— 结果：复查全部通过
- 03 自修第 1 轮：能力缓存在 abilities 查询前写入、缓存建精确索引并预解析能力、删多余 nil 判断、补整条记录生效与推算 known 测试 — 结果：复查全部通过
## 无关发现（发现一条记一条）
- controller/option.go:233 附近：PUT /api/option 仍可写入键 smart_routing_setting.thinking_capabilities，值不生效，只在 options 表留一行废数据；不处理会留脏数据；约 15 分钟（由 04 搬迁删除旧行后，可在写入处拒绝该键）
- model/model_capability.go:200-213：MarshalCanonical 的局部结构体与 relaykit dto.OpenAIModelTopProvider/Architecture/Reasoning 重复；不处理两处形状可能走样；约 20 分钟
- controller/model.go:181、:196：buildOpenAIModel 每个模型查两次能力索引（EffectiveThinkingCapability 内部再查一次）；模型多时多一倍开销；约 15 分钟
