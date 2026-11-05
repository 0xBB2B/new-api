---
name: 05-format-save-check
description: 保存虚拟模型时，检查三档和顾问在每个分组里是否只有拒收所勾格式的渠道
---
# 保存时检查渠道能否接收所勾格式

## 目标
管理员勾了某种接口，而某档在某个分组里的渠道全都接收不了这种格式时，保存被拦下，红字指明是哪档、哪个分组、什么渠道、什么格式。

## 业务规则（来源：PRD）
- 03-endpoint-formats C6：对三档和顾问逐个检查（打分模型不查）：在它的每个分组里，这个模型的可用渠道如果全都属于「明确拒收某个所勾格式」的渠道类型 → 不能保存，写在这一档的分组字段上：「<模型> 在分组 <分组> 里只有 <渠道类型> 渠道，接收不了 <接口> 格式」。渠道类型名用现有的渠道类型名称（`constant.GetChannelTypeName`）；有多种类型时用顿号连接。
- 「明确拒收」以渠道适配器代码里写死返回「不支持」为准。本次核对结果（`relay/channel/*/adaptor.go`）：
  - 拒收 Anthropic（`ConvertClaudeRequest` 直接报错）：Codex、Coze、即梦、Replicate、SubModel、xAI。
  - 拒收 Chat Completions（`ConvertOpenAIRequest` 直接报错）：Codex、Replicate。
  - 拒收 Responses（`ConvertOpenAIResponsesRequest` 直接报错）：AWS、百度、百度 V2、Claude 订阅、Cohere、Coze、Dify、即梦、Jina、MiniMax、Mistral、MokaAI、PaLM、Replicate、硅基流动、SubModel、Vertex、讯飞、腾讯、智谱。
- 这张表要和适配器代码保持一致：用测试逐个调用表里渠道类型的适配器转换函数，确认确实返回错误；表外的不在本次范围。
- PRD 决策：只拦明确拒收的，不依据模型的接口元数据（元数据会误判，例如 DeepSeek）。
- 有这项错误时，前端把它显示在该档的分组那一行（字段名是 `tiers.<档>.groups` 或 `advisor.groups`）。

## 涉及文件
- 修改 `dto/channel_constraints.go`、`model/channel_constraint.go`
- 修改 `service/smartroute/validate.go`
- 测试：扩展 `service/smartroute/smartroute_test.go`；新建 `relay/channel/format_rejection_test.go`

## 函数清单
### dto/channel_constraints.go、model/channel_constraint.go
| 函数名 | 职责 |
|---|---|
| `FilterAcceptsFormat`（过滤器种类） | 新增一种渠道过滤：只留能接收指定请求格式的渠道；照 `FilterScorerCapable` 的写法接进现有过滤流程 |
| `ChannelTypeRejectsFormat` | 按上面的表回答「这种渠道类型是否明确拒收这种格式」 |
### service/smartroute/validate.go
| 函数名 | 职责 |
|---|---|
| `ValidateRoute` | 在现有渠道检查通过后，对三档和顾问、每个所勾格式、每个分组：不加格式过滤能找到渠道、加了格式过滤找不到 → 报上面的错误，渠道类型列出该分组里这个模型的可用渠道类型 |
| `formatDisplayName` | `openai-response` / `openai` / `anthropic` → Responses / Chat Completions / Anthropic |
### relay/channel/format_rejection_test.go
| 函数名 | 职责 |
|---|---|
| 表一致性测试 | 对表里每个「渠道类型 × 格式」取适配器调用对应转换函数，断言返回错误 |

## 协作关系
检查复用 `model.GetRandomSatisfiedChannel` 加过滤器的方式（和打分模型的 `FilterScorerCapable` 一样）。错误信息是 `common.NewMessage` 英文原文，前端翻译键在 07 / 09 里加。

## 验证方式
- 测试入口：`smartroute.ValidateRoute`（测试里建渠道和 ability 数据，照 `smartroute_test.go` 现有写法初始化数据库）；`model.ChannelTypeRejectsFormat`
- 测试输入：普通档 gpt-5.5，分组 subscription 只有一个 Codex 渠道、api 有 OpenAI 渠道；`endpoints` 不同组合
- 预期结果：
  - [ ] 勾 Responses → 能保存；勾 Anthropic → 字段 `tiers.normal.groups` 报「gpt-5.5 has only Codex channels in group subscription, which cannot accept the Anthropic format」（中文「gpt-5.5 在分组 subscription 里只有 Codex 渠道，接收不了 Anthropic 格式」）；勾 Chat → 同样报 Chat Completions
  - [ ] subscription 再加一个 OpenAI 类型的 gpt-5.5 渠道 → 勾 Anthropic 也能保存
  - [ ] 顾问模型只在 Claude 订阅渠道、勾 Responses → `advisor.groups` 报错
  - [ ] 打分模型只在 Codex 渠道、勾 Anthropic → 不报这项错误
  - [ ] 表一致性测试：表里每一项调用适配器转换都返回错误
