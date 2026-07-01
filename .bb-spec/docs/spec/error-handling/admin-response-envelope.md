---
name: admin-response-envelope
description: 管理端 HTTP 响应统一 {success, message, data?} 三字段信封；错误默认 HTTP 200 由 success:false 承载。
---

# 管理端响应信封

## 目的

让前端只解析一套响应结构、不必按端点区分成功/失败分支；把「用户成功语义」与「HTTP 传输层状态」解耦。

## 逻辑

`common/gin.go` 提供 `ApiSuccess` / `ApiSuccessI18n`（`success:true`）与 `ApiError` / `ApiErrorMsg` / `ApiErrorI18n`（`success:false`），HTTP 状态一律 200 OK。

controller 直接使用 `c.JSON` 时也必须保持相同信封形状：`"success"` 布尔、`"message"` 字符串、可选 `"data"`。

少量参数校验失败/服务器异常可用非 200 状态码（400/404/409/500），但 body 仍须包含 `success:false` + `message`，不引入独立错误字段。

无附加数据的成功响应写 `message:""`；错误响应 `message` 取 `err.Error()` 或 i18n 翻译串。

**规则口径**：本规则仅适用于**管理面 / dashboard / web API**。`relay/*` 的 OpenAI 兼容响应（错误 body 为 OpenAI 错误 schema）不受此规则约束。

## 约束

- 响应 body 顶层键固定为 `success`、`message`，可选 `data`；禁用 `code` / `error` / `status` 等替代命名。
- `success` 必须是 `bool`；`message` 必须是 `string`，可为空串。
- 错误默认返回 HTTP 200 + `success:false`；非 200 状态码仅用于参数校验或服务器异常且 body 结构不变。
- controller 层新增端点应优先使用 `common.ApiSuccess` / `ApiError`；直写 `c.JSON` 时不得偏离信封形状。

## 例子

- 输入：`GET /api/user/self`，一路有效 session。
- 过程：controller 校验通过后组装 user 对象；一路 session 无效则返回错误。
- 预期结果：成功 → HTTP 200，body = `{"success":true,"message":"","data":{...user}}`；失败 → HTTP 200，body = `{"success":false,"message":"无权限"}`。

## 验收

- [ ] 管理面 controller 响应 body 的顶层 key 集均为 `success` / `message` / `data`。
- [ ] `success` 类型均为 `bool`；`message` 类型均为 `string`。
- [ ] 抓包任一管理面接口，body 形状与规则一致。
- [ ] 直写 `c.JSON` 的 controller 端点也遵循该信封形状。
