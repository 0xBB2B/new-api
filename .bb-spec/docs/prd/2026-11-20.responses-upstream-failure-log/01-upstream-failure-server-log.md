---
name: 01-upstream-failure-server-log
description: 三种流式 Responses 方式在上游发失败事件时，各写一行带错误原文的服务器日志
date: 2026-11-20
---

# 上游失败原文写进服务器日志

本文的"失败事件"指上游在流式 Responses 回复里发的下面四种事件之一：`type` 为 `error`、`response.failed`、`response.error`，或者事件里 `response.status` 为 `failed`。

## 用户故事
### P0：作为管理员，我想要在服务器日志里看到上游失败的原文，以便判断是上游哪里出了问题

**用例（主路径）**
- 场景：管理员看到渠道 #7 有一批 `/v1/responses` 请求失败，想知道原因。
- 操作：客户端用流式方式调 `/v1/responses`，选中渠道 #7。上游发了一个 `response.failed` 事件，`response.error` 里是 `{"code":"server_error","type":"server_error","message":"The server had an error processing your request."}`。管理员按这个请求的编号查服务器日志。
- 预期：查到一行 ERR 日志，里面有请求编号、渠道号 7、事件类型 `response.failed`、错误码 `server_error`、错误类型 `server_error`，以及消息原文。

**用例（边界 / 异常）**
- 场景：上游发了失败事件，但没给任何错误信息。
- 操作：上游发 `response.failed`，`response.error` 为 null。
- 预期：照样有一行 ERR 日志，带请求编号、渠道号和事件类型，并写明上游没给错误信息。

- 场景：同一个请求里上游连续发了两个失败事件。
- 操作：上游先发 `error`（带 message），再发 `response.failed`（带 code）。
- 预期：服务器日志里有两行 ERR，各自写着对应事件的原文。

- 场景：上游的错误消息很长，或者带着密钥、上游地址。
- 操作：上游的错误消息有 5000 字节，里面带一个 `api_key:abc123` 写法的密钥和上游地址。
- 预期：日志里的消息被截到 2048 字节，并注明原始长度；密钥被替换掉，不出现原文。开了 DEBUG 时不截断，但照样脱敏。

## 约束
- C1：HTTP 流式直接转发的 `/v1/responses` 请求，上游每发一个失败事件，服务器日志就多一行 ERR，内容含请求编号、渠道号、事件类型、错误码、错误类型、错误消息。
- C2：客户端用 Chat Completions 格式、上游走 Responses 流式的请求，失败事件的记录规则和 C1 一样。
- C3：WebSocket 方式的 Responses 请求，上游发的 `error` 帧和 `response.failed` 终止事件，记录规则和 C1 一样。
- C4：失败事件里错误码、错误类型、错误消息全都拿不到时，照样按 C1 记一行，并写明上游没给错误信息。
- C5：一个请求里有几个失败事件，就记几行。
- C6：错误消息的截断规则和非流的 `channel error` 日志一样：超过 2048 字节就截断，并注明原始长度；开了 DEBUG 时不截断。
- C7：错误消息写进日志前先脱敏，规则和非流的 `channel error` 日志一样：`api_key:xxx` 写法的密钥、上游地址、网址里的查询参数、IP 都不出现原文。
- C8：客户端收到的内容、消费日志里的字段、扣费金额，都和改动前一致。

## 验收
- [ ] C1：mock 上游在 HTTP 流式直接转发里发带 code、type、message 的 `response.failed` → 服务器日志恰好多一行 ERR，六项内容齐全。
- [ ] C2：同一个 mock 事件走 Responses 转 Chat 的流式请求 → 服务器日志恰好多一行 ERR，六项内容齐全。
- [ ] C3：同一个 mock 事件走 WebSocket，再单独发一个 `error` 帧 → 两种情况各有一行 ERR，六项内容齐全。
- [ ] C4：`response.failed` 的 error 为 null → 有一行 ERR，带请求编号、渠道号、事件类型，并写明上游没给错误信息。
- [ ] C5：一个请求里先发 `error`、再发 `response.failed` → 恰好两行 ERR，内容分别对应两个事件。
- [ ] C6：5000 字节的错误消息 → 日志里只保留前 2048 字节，并注明原始长度 5000；开了 DEBUG 时保留全文。
- [ ] C7：错误消息里带 `api_key:abc123` 和上游地址 → 日志里两者都被替换，原文不出现。
- [ ] C8：上面各个用例里，客户端收到的字节、消费日志的 other 字段、扣费金额，都和改动前的基线一致。

## 验证路径
1. 起一个 mock 上游，分别用 HTTP 流式、Responses 转 Chat 流式、WebSocket 三种方式发请求，让上游返回带 code、type、message 的 `response.failed` → 每种方式的服务器日志里各有一行 ERR，六项内容齐全。
2. 让 mock 上游改发 error 为 null 的 `response.failed` → 日志写明上游没给错误信息。
3. 让 mock 上游在一个请求里先发 `error`，再发 `response.failed` → 日志恰好两行。
4. 让 mock 上游返回 5000 字节、带密钥的错误消息 → 日志被截到 2048 字节，密钥被替换。
5. 对比改动前后客户端收到的内容、消费日志和扣费 → 三者都没有变化。
