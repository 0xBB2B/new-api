---
name: admin-response-envelope
description: 管理端 HTTP 响应统一 {success, message, data?} 信封，网页提示另带 message_key / message_params；错误默认 HTTP 200 由 success:false 承载。
---

# 管理端响应信封

## 目的

让前端只解析一套响应结构、不必按端点区分成功/失败分支；把「用户成功语义」与「HTTP 传输层状态」解耦。

## 逻辑

`common/gin.go` 提供：
- 成功：`ApiSuccess`、`ApiSuccessT`（带要翻译的提示）。
- 失败：`ApiError`、`ApiErrorT`（带要翻译的提示）、`ApiErrorStatus`（换 HTTP 状态码或加字段）、`ApiErrorMsg`。
- `ApiSuccessI18n` / `ApiErrorI18n`：在后端按请求语言翻译 `i18n/keys.go` 里的 key，只给 API 令牌直接调用的接口用（例如 `controller/token.go` 的令牌用量查询），网页控制台的接口不用。

HTTP 状态默认 200 OK。

网页控制台看到的提示走 `*common.Message`：`message` 是英文渲染结果，另带 `message_key`（英文原文，也是前端语言文件的 key），有参数时再带 `message_params`。`ApiError` 收到 `*common.Message` 时会自动补上这两个字段，前端按它们翻译。

controller 直接使用 `c.JSON` 时也必须保持相同信封形状：`"success"` 布尔、`"message"` 字符串、可选 `"data"`；要翻译的提示用 `maps.Copy(resp, msg.Fields())` 并入 `message_key` / `message_params`。

少量参数校验失败/服务器异常可用非 200 状态码（400/401/403/404/409/500），body 仍须包含 `success:false` + `message`。需要前端按错误种类做不同处理时，可加一个字符串字段 `code` 作稳定错误码（例如 `AUTH_SESSION_NOT_FOUND`），写法是 `ApiErrorStatus(c, status, err, gin.H{"code": code})`。

无附加数据的成功响应写 `message:""`。

**规则口径**：本规则仅适用于**管理面 / dashboard / web API**。`relay/*` 的 OpenAI 兼容响应（错误 body 为 OpenAI 错误 schema）不受此规则约束。

## 约束

- 响应 body 顶层必有 `success`、`message`；可选 `data`、`message_key`、`message_params`、`code`，以及接口自己的业务字段。禁用 `error` / `status` 代替 `success` / `message`。
- `success` 必须是 `bool`；`message` 必须是 `string`，可为空串；`code` 必须是字符串。
- 错误默认返回 HTTP 200 + `success:false`；非 200 状态码仅用于参数校验、鉴权或服务器异常，且 body 结构不变。
- 网页控制台的提示必须带 `message_key`：用 `ApiErrorT` / `ApiSuccessT`，或返回 `*common.Message` 交给 `ApiError`。
- controller 层新增端点优先使用 `common.ApiSuccess` / `ApiError` / `ApiErrorT`；直写 `c.JSON` 时不得偏离信封形状。

## 例子

- 输入：`GET /api/channel/999999/claude/usage`，渠道不存在。
- 过程：controller 调用 `common.ApiErrorT(c, "Channel does not exist")`。
- 预期结果：HTTP 200，body = `{"success":false,"message":"Channel does not exist","message_key":"Channel does not exist"}`；中文界面显示「渠道不存在」。
- 成功时：body = `{"success":true,"message":"","data":{...}}`。

## 验收

- [ ] 管理面 controller 响应 body 都有 `success` / `message`，其余顶层字段只在约束列出的范围内。
- [ ] `success` 类型均为 `bool`；`message` 类型均为 `string`。
- [ ] 网页控制台的错误提示响应都带 `message_key`。
- [ ] 直写 `c.JSON` 的 controller 端点也遵循该信封形状。
