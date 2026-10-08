---
name: claude-limit-reset-card
description: Claude 用量弹窗新增「限额重置」卡片，分全部重置与 5 小时重置两行，按上游返回的状态决定说明文字与「免费重置」按钮能否点击。
---

# Claude 用量弹窗的限额重置卡片

## 目的
让管理员在 Claude 订阅渠道的用量弹窗里看到这个账号还有没有限额重置可用、什么时候过期、用了会清空哪些窗口，并从这里发起重置。

## 逻辑
- 位置：Claude 用量弹窗里，用量窗口卡片之下、原始 JSON 折叠区之上。数据来自本次用量接口响应 `data` 里的 `cedar_ember`（全部重置）与 `juniper_tide`（5 小时重置），不单独请求。Codex 弹窗不变。
- 卡片状态优先级：
  1. 用量请求进行中 → 两行骨架占位；
  2. 响应带 `limit_reset_unavailable: "client_version"` → 显示「取不到 Claude Code 最新版本，暂时无法查询重置」；
  3. `cedar_ember` 和 `juniper_tide` 都缺失或为 null → 显示「上游未返回重置信息」；
  4. 否则显示两行。
- 用量接口失败：打开弹窗时失败 → 只弹出错误提示，不打开弹窗；弹窗内刷新时失败 → 只弹出错误提示，弹窗与卡片保留刷新前的数据，卡片不单独显示错误。
- 全部重置行：取 `grants` 中 `id == next_grant_id` 的授予记录。
  - 找到时分三层显示：标题「全部重置」；下一行单独显示 `label`（上游原文，不翻译）；再下面是左右对齐的字段表，每个字段一行：
    - 剩余次数：`resets_left/resets_total`；
    - 到期时间：`ends_at` 的本地时间；
    - 冷却：仅冷却中时出现，值为「<时间> 后可用」；
    - 会清空：`clears` 映射成窗口名后，按界面语言的列表格式连接（简体中文为「A、B、C」，其他语言按各自习惯，如繁体中文「A、B和C」、英文「A, B, C」）。映射：`five_hour` → 5 小时窗口，`seven_day` → 每周窗口，`seven_day_overage_included` → Fable 每周窗口，其他原样显示。
  - 按钮可点的条件全部满足：`eligible == true`；`usable_now == true`；`paused != true`；`resets_left > 0`；`cooldown_until` 为空或已过；`use_requires_limit != true` 或 `at_limit == true`。
  - `cooldown_until` 非空且未过、或非空但无法解析时，视为冷却中：按钮禁用，字段表出现「冷却」一行。
  - 找不到授予记录时显示「目前没有。拿到后会显示在这里」，不显示按钮。
- 5 小时重置行：
  - `eligible == true` 且 `available == true` 时，显示「本周剩余可用 · 只清空 5 小时窗口，不影响每周上限」和可点的按钮；
  - 否则不显示按钮，说明按 `ineligible_reason` 映射：`not_at_wall` → 「5 小时额度用完时才会出现」；`surface` 或 `cli_version` → 「客户端身份校验未通过」；其他非空值原样显示；空值且 `next_available_at` 有值 → 「<时间> 后可用」。
- 按钮文字为「免费重置」，加载或执行中禁用。

## 约束
- 按钮可点的判定只依据上面列出的字段，任一条件不满足就禁用或不显示。
- 时间字段（`ends_at`、`cooldown_until`、`next_available_at`）是 ISO 8601 字符串，按浏览器本地时区显示到分钟；解析失败显示 `-`。
- 卡片所有文字走 i18n。

## 例子
- 实测响应：`cedar_ember.eligible=true`，`next_grant_id="opus55-launch-promax-20260921"`，该授予记录 `resets_left=1`、`resets_total=1`、`usable_now=true`、`paused=false`、`use_requires_limit=false`、`ends_at="2026-10-22T16:00:00+00:00"`、`clears=["five_hour","seven_day","seven_day_overage_included"]`；`juniper_tide.eligible=false`、`ineligible_reason="not_at_wall"` → 全部重置行：标题下一行「Claude Opus 5.5 launch: one usage-limit reset for Pro and Max」；字段表「剩余次数 1/1」「到期时间 2026-10-23 00:00」（UTC+8）「会清空 5 小时窗口、每周窗口、Fable 每周窗口」，没有「冷却」行，按钮可点；5 小时重置行显示「5 小时额度用完时才会出现」，没有按钮。
- 同上但 `cooldown_until` 为当前时间后 10 分钟 → 全部重置按钮禁用，字段表多出「冷却 <结束时间> 后可用」一行。
- `cedar_ember.ineligible_reason="surface"`、`grants=[]` → 全部重置行显示「目前没有。拿到后会显示在这里」。
- 响应 `cedar_ember=null`、`juniper_tide=null` → 卡片显示「上游未返回重置信息」。
- 弹窗内点「刷新」时上游返回 401 → 弹出错误提示；卡片仍显示刷新前的两行与按钮状态。

## 验收
- [ ] 卡片位置在用量窗口与原始 JSON 之间，Codex 弹窗没有这张卡片。
- [ ] 四种卡片状态按优先级显示；用量刷新失败时卡片保留上一次成功的数据。
- [ ] 全部重置按钮仅在六个条件同时满足时可点。
- [ ] 5 小时重置按 `ineligible_reason` 显示对应说明，可用时才有按钮。
- [ ] 时间按本地时区显示，解析失败显示 `-`。
