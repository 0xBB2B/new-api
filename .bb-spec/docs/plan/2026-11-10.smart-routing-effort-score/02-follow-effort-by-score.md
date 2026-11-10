---
name: 02-follow-effort-by-score
description: 「跟随打分」按 effort_score 在最低～最高范围内模型支持的档位里按比例选力度
---
# 跟随打分按比例选力度

## 目标
档位力度方式是「跟随打分」（`EffortModeFollow`）时，力度由 `effort_score` 按比例决定，不再由 `reasoning` 标签决定。

## 业务规则（来源：PRD 02-follow-effort-by-score）
- 七档顺序固定为 `none < minimal < low < medium < high < xhigh < max`（`operation_setting.SmartEffortLevels`）。
- **候选档位**：七档里在档位 `EffortMin`～`EffortMax` 之间（含两端），并且在目标模型能力 `Capability.Efforts` 里的档位，按七档顺序从低到高排列，共 n 档。
- **有分数时**（用户新消息、打分成功、`EffortScore` 非 nil）：设分数为 s，取从低往高数第 `min(⌊s × n⌋, n − 1)` 档（从 0 开始数）。
- **候选为 0 档时**：改用范围内的七档（不看模型能力）按同样的分段选出一档，再用现有的 `ClampEffortToCapability` 换成模型支持的最近一档（上下一样近取更高）。换了档时 `From` 记换档前的值，日志照常显示「由 X 降档 / 升档」。
- **没有分数时**（打分失败、`EffortScore` 为 nil、不是用户新消息而且会话里没有记录）：取候选档位里最高的一档；候选为 0 档时取 `EffortMax`，再按换档规则换档。
- 打分把握不足而改用默认档时，标签照样传进来；只要 `EffortScore` 有效，就按"有分数时"计算。
- `reasoning` 标签不再影响力度。
- 不变的部分：会话续跑沿用 `SessionEffort`；「固定」「不覆盖」两种方式；模型不支持思考时 `Drop`；顾问不走这个函数。Responses、Chat Completions、Anthropic 三种格式共用这个函数，结果自然相同。
- 已保存的配置不用迁移：`EffortMode`、`EffortMin`、`EffortMax` 的存储格式不变。
- 例子：gpt-5.6-sol「跟随打分 medium～xhigh」，候选为 medium、high、xhigh。s = 0 / 0.30 / 0.50 / 0.72 / 1 时依次得到 medium / medium / high / xhigh / xhigh。打分失败时得到 xhigh。
- 例子：glm-5.3（只支持 low、high、max）「跟随打分 low～max」，候选为 low、high、max。s = 0.2 / 0.5 / 0.9 时依次得到 low / high / max。
- 例子：模型被覆盖成只支持 low、xhigh，档位是「跟随打分 medium～high」，候选为 0 档。范围是 medium、high 两档：s = 0.3 先得 medium，再换成 low，`From` 为 medium；s = 0.8 先得 high，再换成 xhigh，`From` 为 high。
- 例子：模型被覆盖成只支持 none、low、medium、high，档位是「跟随打分 medium～xhigh」，打分超时：候选为 medium、high，得到 high，`From` 为空（不再是"由 xhigh 降档"）。

## 涉及文件
- 修改 `service/smartroute/effort.go`
- 修改 `middleware/distributor_test.go`（改写 `TestSmartRouteDistributeEffortFollowMapsReasoningLabelAndClamps`、`TestSmartRouteDistributeEffortLogShape` 里依赖旧映射的用例；测试辅助 `smartLabels` 需要能带上 `EffortScore`）
- 修改 `service/smartroute/smartroute_test.go`（可选：给 `ResolveTierEffort` 加表格测试，覆盖上面的例子）

## 函数清单
### service/smartroute/effort.go
| 函数名 | 职责 |
|---|---|
| `ResolveTierEffort` | 「跟随打分」分支改成按上面的规则算：先求候选档位，有分数就按比例选，没分数就取最高；候选为 0 档时退回范围内七档，再换档。其余分支不变 |
| `scoredEffort` | 删除（按 `reasoning` 映射力度的旧逻辑不再使用）；同时删掉它带来的无用引用 |

## 协作关系
`middleware/smart_route.go` 的 `ResolveTierEffort` 调用点不变：`Labels` 来自打分结果（已带 `EffortScore`，见计划 01），`Capability` 来自 `smartroute.EffectiveThinkingCapability(realModel)`。档位排序用 `operation_setting.SmartEffortRank` 和 `operation_setting.SmartEffortLevels`；换档复用 `ClampEffortToCapability`。

## 验证方式
- 测试入口：`go test ./middleware/ ./service/smartroute/`。中间件测试走真实的分发流程：`setupSmartEffortRoute` 配置档位和能力覆盖，`installSmartScorerStub` 注入打分结果，`serveSmartRoute` 发请求，再看上游请求体里的力度和日志的 `effort` 字段。
- 测试输入：档位 gpt-5.6-sol「跟随打分 medium～xhigh」，打分标签 `EffortScore` 依次为 0、0.30、0.50、0.72、1、nil；打分失败（超时、报错、格式不对）；能力覆盖 `{"gpt-5.6-sol":{"efforts":["low","xhigh"]}}` 加「跟随打分 medium～high」，分数 0.3、0.8；能力覆盖 none～high 加打分超时；glm-5.3「跟随打分 low～max」，分数 0.2、0.5、0.9。
- 预期结果：
- [ ] gpt-5.6-sol 依次得到 medium、medium、high、xhigh、xhigh；`EffortScore` 为 nil 和三种打分失败都得到 xhigh；日志 `effort` 为 `{"value":…,"source":"follow"}`，没有 `from`。
- [ ] 把握不足走默认档（默认档为同样配置）、`EffortScore` 为 0.30 → medium。
- [ ] `EffortScore` 都是 0.30，`reasoning` 分别为 low 和 high、进同一档 → 都得到 medium。
- [ ] 只支持 low、xhigh 的例子：0.3 得 low，日志 `from` 为 medium；0.8 得 xhigh，`from` 为 high。
- [ ] 能力为 none～high 时打分超时 → high，日志没有 `from`。
- [ ] glm-5.3 依次得到 low、high、max。
- [ ] 同一个分数分别用 `/v1/responses`、`/v1/chat/completions`、`/v1/messages` 请求，上游收到的力度相同。
- [ ] 现有的续跑沿用、固定、不覆盖、顾问、不支持思考的用例全部仍然通过。
- [ ] `go build ./...` 通过；`gofmt` 无差异。
