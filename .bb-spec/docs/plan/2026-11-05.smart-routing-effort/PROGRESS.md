# 执行进度
| 序号 | Plan | 状态 | 完成时间 |
|---|---|---|---|
| 01 | 01-route-config-schema | done | 2026-11-05 |
| 02 | 02-thinking-capability-core | done | 2026-11-05 |
| 03 | 03-effort-runtime | done | 2026-11-05 |
| 04 | 04-anthropic-routing | done | 2026-11-06 |
| 05 | 05-format-save-check | done | 2026-11-06 |
| 06 | 06-user-models-virtual | done | 2026-11-05 |
| 07 | 07-route-form-model | done | 2026-11-05 |
| 08 | 08-group-order-compact | done | 2026-11-05 |
| 09 | 09-route-editor-dialog | done | 2026-11-06 |
| 10 | 10-routes-and-capability-cards | pending | — |
| 11 | 11-model-pickers-frontend | done | 2026-11-05 |
| 12 | 12-log-effort-row | done | 2026-11-05 |
## 当前
下一步：`10-routes-and-capability-cards`。
- 09 自修 3 轮：力度方式改分段按钮、「～」、未确认标签移到力度行；本地与服务端错误都计入左栏和档位 Tab（红点 + N 个错误，全空表单三个 Tab 都标错按 C11 规则，2026-11-06 主线程定）；手机宽度横排；高级字段说明收进 ⓘ；档位悬停摘要；能力表加载中或失败时禁用力度控件；旧格式档位表格和弹窗都按固定显示；复数键 7 种语言写齐 4 个后缀防 i18n:sync 回填。待浏览器确认 C12（1100px 不出滚动、档位 Tab 英文不挤出）。
- 05 自修 3 轮：补 Anthropic 收到即 panic 的 10 种渠道和 MokaAI Chat；用户 2026-11-06 决定保存检查算上「Chat 转 Responses」策略，按运行时规则逐个渠道判断（命中策略且没开透传的看 Responses，其余看原格式）；渠道类型用 ", " 连接、无类型时用通用提示；查询提到格式循环外避免重复全表读。
- 04 自修 3 轮：透传分支清理思考块且逐字保留原文、去空的 assistant 消息整条删除（核实 Anthropic text minLength 1）、Claude 模型用 RenderClaude 生成思考设置（默认 max_tokens、采样参数、display/block_binding）、thinking/output_config 内外重复键拦截。待实测：工具循环中途换渠道去掉思考块后上游是否接受。
- 03 自修 2 轮：续跑档位不同时重算、会话保存改普通参数；用户 2026-11-05 批准 DeepSeek 适配器 Chat 格式 none 改写为 thinking.type=disabled（限 v4/flash 家族）；plan 11 审查发现演练场 /pg/chat/completions 选虚拟模型 400、会话不分用户、演练场打分扣 id=0 令牌失败，一并修复（打分请求继承演练场标记，只扣钱包）。Anthropic 分支的 3 个问题转入 plan 04。
- 01/02/06/07/08/11 已完成。

## 阻塞
（无）
