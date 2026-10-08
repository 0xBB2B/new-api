---
name: claude-limit-reset-redeem
description: 新接口 POST /api/channel/:id/claude/usage/reset 执行全部重置或 5 小时重置；先取组织 ID 再调上游，全部重置的 request_id 固定为 <grant_id>-u<resets_left>；仅 result=reset 算成功。
---

# 执行 Claude 限额重置

## 目的
让有渠道操作权限的管理员在 Claude 用量弹窗里用掉一次全部重置或 5 小时重置。每次重置都会花掉一次有限的上游权益，所以要二次确认，并且同一次重试不能扣两次。

## 逻辑
- 接口：`POST /api/channel/:id/claude/usage/reset`，权限 `ChannelOperate`，在 `router/channel-router.go` 的权限路由表中声明（访问令牌权限范围随之登记）。
- 请求体：`{"program": "cedar_ember" | "juniper_tide", "grant_id": string, "resets_left": int}`；`grant_id`、`resets_left` 仅 `cedar_ember` 必填。
- 后端处理顺序：
  1. 校验渠道：存在、类型为 Claude 订阅、不是多 key 渠道、凭据可解析；
  2. 校验请求：`program` 只能是两个值之一；`cedar_ember` 时 `grant_id` 匹配 `^[a-z0-9_-]{1,40}$`，`resets_left` 为 1～100 的整数；不合法返回 `success=false` 与翻译后的原因，不调上游；
  3. 取客户端版本号（与用量查询共用同一份缓存），取不到则返回「取不到 Claude Code 最新版本，无法执行重置」，不调上游；
  4. `GET <base_url>/api/oauth/profile` 取 `organization.uuid`，必须是标准 UUID 格式，否则返回失败；
  5. `POST <base_url>/api/organizations/<uuid>/reset_rate_limits`。请求体：`cedar_ember` 为 `{"program":"cedar_ember","grant_id":<grant_id>,"request_id":"<grant_id>-u<resets_left>"}`；`juniper_tide` 为 `{"program":"juniper_tide"}`。
- 两次上游请求都带 `Authorization: Bearer <accessToken>`、`anthropic-beta: oauth-2025-04-20`、`User-Agent: claude-cli/<版本号> (external, cli)`；access token 失效（401/403）时刷新凭据并重试一次，规则与用量查询相同。
- 结果判定：上游 HTTP 2xx 且 `result == "reset"` → `success=true`，`message` 为「重置成功」；其他情况 `success=false`，`message` 按下表给出，并带回 `upstream_status` 与上游原始 JSON。
  - `already_used` → 这次重置已经用过了
  - `not_limited` → 当前没有触顶，不需要重置
  - `cooldown` → 冷却中，请稍后再试
  - `ineligible` → 账号不符合使用条件
  - `unavailable` → 上游暂时不可用
  - HTTP 2xx 但 `result` 不是以上任何值（含 body 不是 JSON）→ 上游返回未知结果
  - HTTP 429 → 请求太频繁，请稍后再试
  - HTTP 401 / 403（重试后仍是）→ 凭据无效或权限不足
  - 其他 HTTP 状态 → 上游返回 HTTP <状态码>
  - `reset_rate_limits` 请求发出后网络出错或超时（拿不到响应）→ 重置结果未知，请先刷新用量确认后再决定是否重试
  - 按渠道代理设置构造 HTTP 客户端失败 → 重置失败，请稍后重试
- 前端：点「免费重置」先弹确认框，说明会清空哪些窗口、次数用完就没有了；确认后调用接口；成功后显示成功提示并重新调用用量接口刷新弹窗；失败显示失败原因。执行中按钮禁用。
- `message` 由 controller 按用户语言翻译（en / zh-CN / zh-TW），service 层只回传翻译 key 与参数；上表与下文所列为简体中文译文，带状态码的一条用模板参数 `Status` 注入。
- 不写审计日志。

## 约束
- 相同的 `grant_id` 与 `resets_left` 重试时，发往上游的 `request_id` 完全相同。
- 请求校验不通过、取不到版本号、取不到组织 ID 时，不向 `reset_rate_limits` 发请求。
- 只有 `result == "reset"` 返回 `success=true`。
- 上游错误文本不进入 `message`，只出现在原始 JSON 字段里。

## 例子
- 请求 `{"program":"cedar_ember","grant_id":"opus55-launch-promax-20260921","resets_left":1}`，profile 返回 `organization.uuid="2f1c…"`（标准 UUID）→ 上游收到 `POST /api/organizations/2f1c…/reset_rate_limits`，请求体 `request_id` 为 `opus55-launch-promax-20260921-u1`；上游返回 `{"result":"reset","resets_left":0}` → 接口 `success=true`，前端提示成功并刷新用量。
- 同一请求第二次提交，上游返回 `{"result":"already_used"}` → `success=false`，`message`「这次重置已经用过了」。
- 上游返回 HTTP 200 `{"result":"something_new"}` → `success=false`，`message`「上游返回未知结果」。
- 上游收到 reset 请求后连接断开 → `success=false`，`message`「重置结果未知，请先刷新用量确认后再决定是否重试」；用户刷新后看到 `resets_left` 已变 0，说明其实已重置成功；若直接重试，相同 `request_id` 会得到 `already_used`，不会重复扣次数。
- 请求 `{"program":"cedar_ember","grant_id":"../x","resets_left":1}` → `success=false`，不调上游。
- 请求 `{"program":"juniper_tide"}`，上游返回 `{"result":"not_limited"}` → `success=false`，`message`「当前没有触顶，不需要重置」。

## 验收
- [ ] 路由权限为 `ChannelOperate`，访问令牌权限范围对照测试通过。
- [ ] 非法 `program`、`grant_id`、`resets_left` 被拒绝且不调上游。
- [ ] 全部重置的 `request_id` 为 `<grant_id>-u<resets_left>`，5 小时重置请求体只有 `program`。
- [ ] 结果映射与表格一致（含「重置成功」「上游返回未知结果」「重置结果未知…」），仅 `reset` 算成功。
- [ ] 前端确认后才调用接口，成功后自动刷新用量。
