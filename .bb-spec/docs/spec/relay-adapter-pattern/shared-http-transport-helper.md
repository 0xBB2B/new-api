---
name: shared-http-transport-helper
description: adapter 的 HTTP 执行阶段委托到共享传输 helper；禁自行手写请求构造与发送。
---

# 共享 HTTP 传输 helper

## 目的

把 URL 拼装、请求构造、鉴权头装配、header override、超时/代理/日志等横切能力集中到一处；adapter 只需描述自己的差异化片段（URL、鉴权头、请求体）。

## 逻辑

`channel` 包提供统一 HTTP 发起 helper，接受 adapter 接口 + `gin.Context` + relay info + 请求体 reader。helper 内部依次：

1. 调用 adapter 的 `GetRequestURL` 得到目标 URL。
2. 构造 `http.Request`。
3. 调用 adapter 的 `SetupRequestHeader` 装配鉴权头。
4. 应用用户级 header override（覆盖默认 Authorization 等），保证用户设置优先级最高。
5. 发起请求。

adapter 的 `DoRequest` 实现体保持一行：委托到该 helper。只有真正与标准 HTTP 流程不同的 provider（如 SDK-based 传输、form 请求）才允许绕过，但仍走同层的其他共享 helper。

## 约束

- adapter 的 `DoRequest` 禁直接调用 `net/http` 构造与发起（SDK-based 传输除外）。
- user header override 必须在 `SetupRequestHeader` 之后应用。
- URL、鉴权头、请求体这三样差异化片段以外的横切逻辑禁下沉到 adapter。
- 绕过标准 helper 的 provider 必须走同层的共享 helper 变体（例如 form 请求变体）。

## 例子

- 输入：新增一个 OpenAI 兼容协议的上游 Y。
- 过程：Y 的 adapter 仅实现 `GetRequestURL` 返回 `base_url+path`、`SetupRequestHeader` 设 Bearer 头、`ConvertOpenAIRequest` 走透传；`DoRequest` 直接委托共享 helper。
- 预期结果：Y 自动获得 header override、代理、日志、超时等能力；日后横切策略升级 Y 无需改代码。

## 验收

- [ ] adapter 的 `DoRequest` grep 出的实现体基本都是「一行委托到共享 helper」。
- [ ] adapter 内 grep `http.NewRequest` / `http.DefaultClient.Do` 命中数为 0（SDK-based 除外）。
- [ ] user header override 生效顺序：在 `SetupRequestHeader` 之后。
- [ ] 新增 OpenAI 兼容 provider 无需重写 HTTP 层，仅描述 URL/鉴权/请求体差异。
