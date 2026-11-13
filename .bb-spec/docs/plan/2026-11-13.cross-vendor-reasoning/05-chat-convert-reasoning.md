---
name: 05-chat-convert-reasoning
description: relaykit 的 Responses→Chat 请求转换把保留的思考记录放进 assistant 的 reasoning_content，不生成用户消息
---
# 转聊天格式时处理思考记录

## 目标
Responses 请求转成 Chat Completions 时，`type` 为 `reasoning` 的条目不变成任何角色的消息；带 `id` 的（宿主已确认是本渠道生成并还原了原值）把可读文字放进紧跟着的 assistant 消息的 `reasoning_content`。

## 业务规则（来源：PRD 03-chat-reasoning）
- `type` 为 `reasoning` 的条目一律不转成 user 或其他角色的消息。
- 宿主约定：发往上游前，宿主已把别的渠道、没有标记、标记读不出的思考记录去掉 `id`（或整条去掉）；只有本渠道生成的会带着非空 `id` 到达转换器。所以：`id` 为空的 reasoning 条目整条丢掉，摘要和正文都不用。
- 带非空 `id` 的条目取可读文字：`content[]` 里 `type` 为 `reasoning_text` 的 `text` 按顺序拼接，非空就用它；否则用 `summary[]` 里各项的 `text` 按顺序拼接。都为空（只有加密内容）→ 丢掉。
- 可读文字挂到它后面紧挨着的 assistant 消息上：后续条目是 assistant 角色的 message，或是 function_call / custom_tool_call（会并进 assistant 的 `tool_calls`）。连续多条思考记录对应同一条 assistant 消息时，文字按原顺序用一个空行（`\n\n`）连接。
- 在出现 assistant 消息之前先遇到 user 消息或工具结果（function_call_output / custom_tool_call_output），或者 input 到头了 → 积攒的思考文字丢掉，不产生任何消息。
- 转换器只依赖 relaykit 自身，不能 import 根模块（`GOWORK=off go build ./...` 在 relaykit 内必须通过）。

## 涉及文件
- relaykit/relayconvert/internal/oai_responses/to_oai_chat_req.go（修改）
- relaykit/relayconvert/internal/oai_responses/to_oai_chat_req_test.go（修改：补用例）

## 函数清单
### relaykit/relayconvert/internal/oai_responses/to_oai_chat_req.go
| 函数名 | 职责 |
|---|---|
| input 数组遍历处（`case "array"` 分支所在函数） | 维护"待挂的思考文字"：遇到 reasoning 条目累加；生成或并入 assistant 消息时写入 `reasoning_content` 并清空；遇到 user 消息或工具结果时清空 |
| `responsesInputItemToChatMessages` | reasoning 条目走自己的分支，不落到"默认按 user 处理"的分支 |
| `responsesReasoningItemText` | 从一条 reasoning 条目取可读文字（先正文后摘要，`id` 为空返回空） |
| `appendToolCallToLastAssistant` | 保持现有职责；新建或并入 assistant 消息时不覆盖已写入的 `reasoning_content` |

## 协作关系
- `dto.Message.ReasoningContent` 是 `*string`（relaykit/dto/openai_request.go:499），没有文字时保持 nil，不发空串。
- 依赖 04 的宿主约定（清理后的条目无 `id`）；本 plan 自身可独立实现和测试。

## 验证方式
- 测试入口：`relaykit/relayconvert/internal/oai_responses` 包里现有 to_oai_chat_req_test.go 调用的 Responses→Chat 请求转换入口，输入 `dto.OpenAIResponsesRequest`，断言输出的 `messages`。
- [ ] input = [user "问", reasoning(id `r1`, content reasoning_text "先看看目录结构"), function_call(call_1), function_call_output(call_1)] → messages 依次为 user、assistant(tool_calls=[call_1], reasoning_content="先看看目录结构")、tool，共 3 条。
- [ ] reasoning 同时有正文"A"和摘要"B" → reasoning_content="A"；只有摘要"B" → "B"。
- [ ] 两条 reasoning（正文"A"、"B"）后接 assistant message "答" → 该 assistant 的 reasoning_content="A\n\nB"，content="答"。
- [ ] reasoning(id `r1`, 正文"A") 后直接是 user 消息 → 输出里没有含"A"的消息，也没有空 user 消息。
- [ ] 只有 encrypted_content 的 reasoning(id `r1`) → 不产生消息、不写 reasoning_content。
- [ ] 没有 id、带 summary "S" 的 reasoning，没有 id、带正文 "T" 的 reasoning，后接 assistant → reasoning_content 为 nil。
- [ ] 在 relaykit 目录执行 `GOWORK=off go build ./...` 成功。
