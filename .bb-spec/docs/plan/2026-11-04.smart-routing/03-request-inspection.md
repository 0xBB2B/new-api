---
name: 03-request-inspection
description: 从 Responses / Chat Completions 请求体里识别用户新消息、取出用户消息和本地特征、算会话标识
---
# 请求内容识别

## 目标
给定请求体和接口类型，能判断这是"用户新消息"还是"工具结果续跑"，取出打分要用的用户消息和本地特征，并算出会话标识。全部是纯函数，不访问网络和数据库。

## 业务规则（来源：PRD 02-scoring C1、C2，03-session-tier-switch C1）
- 最后一条输入是用户写的文字 → 用户新消息；最后一条输入是工具调用结果 → 工具结果续跑。
  - Responses：`input` 是字符串时视为用户新消息。是数组时看最后一项：`type` 为 `function_call_output`、`custom_tool_call_output`、`local_shell_call_output`、`mcp_call_output` 等以 `_output` 结尾的，算工具结果；`role` 为 `user` 的 message 算用户文字。
  - Chat Completions：看 `messages` 最后一条：`role` 为 `tool`（或旧写法 `function`）算工具结果；`role` 为 `user` 算用户文字。
  - 其他情况（例如最后一条是 assistant）按工具结果续跑处理，不打分。
- 用户消息只取文字部分：content 是字符串就直接用；是数组就拼接 `input_text` / `text` 类型条目的文字。
- 发给打分模型的内容：最新一条用户消息，超过 4000 字截断（按 rune 计）；往前再取 N+1 条用户消息，每条取开头 200 字（N 由调用方传入）。系统提示词（system / developer 角色、Responses 的 `instructions`）、工具结果内容、思考内容一律不取。
- 本地特征：最新消息的字数（rune）、对话轮数（用户消息条数）、是否带图片或文件（content 里有 `input_image` / `image_url` / `input_file` / `file` 类型条目）、是否贴了代码或表格（最新消息里有 ``` 代码块，或者有至少两行以 `|` 开头的表格行）。
- 会话标识：请求带 `prompt_cache_key`（Responses 和 Chat Completions 都看这个字段，字符串）→ `sha256(令牌 ID + "|pck|" + prompt_cache_key)`；不带 → `sha256(令牌 ID + "|first|" + 第一条用户消息的完整文字)`。没有任何用户消息时返回空，调用方按新会话处理。
- 请求的接口类型按请求路径判断：`/v1/responses` 是 `openai-response`，`/v1/chat/completions` 是 `openai`，其他路径不属于智能路由支持的接口。

## 涉及文件
- service/smartroute/inspect.go（新建）

## 函数清单
### service/smartroute/inspect.go
| 函数名 | 职责 |
|---|---|
| `RequestEndpointFromPath` | 按请求路径返回接口类型；不支持的路径返回空 |
| `InspectRequest` | 解析请求体，返回：是否为用户新消息、按时间顺序排列的用户消息文字列表、本地特征、prompt_cache_key |
| `BuildScorerInput` | 按截断规则（最新消息 4000 字、前 N+1 条各 200 字）和本地特征，生成打分输入的结构 |
| `SessionKey` | 按会话标识规则算出会话键 |

JSON 读取统一用项目现有的 `github.com/MoeForge/moego/ojson`，或 `common.Unmarshal` 解析到 `relaykit/dto` 的 `OpenAIResponsesRequest` / `GeneralOpenAIRequest`。禁止直接调用 `encoding/json`。

## 协作关系
- 被 07（路由接入）调用：先判断接口类型和是否为用户新消息，再决定打不打分。
- `BuildScorerInput` 的结果交给 06（打分调用）。
- `SessionKey` 的结果交给 05（会话记录）。

## 验证方式
- 测试入口：`smartroute.InspectRequest`、`smartroute.BuildScorerInput`、`smartroute.SessionKey`、`smartroute.RequestEndpointFromPath`（用例写在 service/smartroute/smartroute_test.go，表格驱动）。
- 测试输入：手写的 Responses 和 Chat Completions 请求体 JSON。
- 预期结果：
- [ ] Responses `input` 为字符串 → 用户新消息；最后一项为 `function_call_output` → 工具结果续跑；最后一项为 user message → 用户新消息
- [ ] Chat 最后一条 `role=tool` → 工具结果续跑；`role=user` → 用户新消息
- [ ] 带 system、developer、instructions、工具结果、reasoning 条目的请求 → 取出的用户消息里不含这些内容
- [ ] 5000 字的最新消息被截到 4000 字；N=2 时取最近 3 条用户消息，每条 ≤ 200 字
- [ ] 带 `input_image` → 有图片；最新消息含 ``` → 贴了代码；两行 `|a|b|` → 贴了表格
- [ ] 同一令牌、同一 prompt_cache_key → 同一个会话键；不带 prompt_cache_key、第一条用户消息相同 → 同一个会话键；不同令牌 → 不同的会话键
- [ ] `/v1/responses` → openai-response，`/v1/chat/completions` → openai，`/v1/messages` → 空
