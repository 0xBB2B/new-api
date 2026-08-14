---
name: claude-usage-dialog
description: Claude 订阅渠道点击列表用量后的账户和用量弹窗：状态卡 + 动态窗口卡（阈值配色）+ 原始 JSON。
---

# Claude 订阅账户和用量弹窗

## 目的

管理员在渠道列表点击 Claude 订阅渠道（type 61）的用量单元格后，弹窗查看该账号的实时用量窗口详情与原始响应。

## 逻辑

点击触发一次实时用量请求（`GET /api/channel/:id/claude/usage`），成功后打开弹窗。弹窗分三个区块：

- **状态卡**：套餐 badge（端点响应的 subscription_type 值，如 max/pro；缺失时不渲染该 badge）、`HTTP <upstream_status>` badge、渠道字段（渠道名 + `(#id)`；敏感信息隐藏开启时名称与 ID 以掩码显示）、刷新按钮（重新请求端点并更新弹窗，请求中禁用）。
- **窗口卡**：按响应实际出现的用量窗口动态渲染，两种形态都支持——顶层对象形态渲染键为 `five_hour`（5小时窗口）与 `seven_day` 前缀（7天/7天 Opus/7天 Sonnet 等）的窗口，取各自 `utilization`；`limits` 数组形态按各项 `kind` 命名窗口，取 `percent`。每张卡：窗口名、大号百分比、进度条；百分比与进度条按同一阈值着色——≥95 红（danger）、≥80 橙（warning）、其余默认。窗口对象含重置时间字段时展示，缺失时不显示该行。
- **原始 JSON 折叠区**：端点完整响应的格式化 JSON，附复制按钮。

端点返回 `success:false` 时不打开弹窗、toast 提示错误；弹窗已打开后刷新失败时在弹窗顶部横幅展示 message。关闭弹窗清空本地状态。

## 约束

- 仅 type 61 渠道走本弹窗；窗口渲染不硬编码窗口数量，响应有几个窗口渲染几张卡。
- `extra_usage` 等非窗口对象不渲染为窗口卡（仍可在原始 JSON 中查看）。
- 百分比着色阈值固定：≥95 红、≥80 橙、其余默认；进度条与百分比同色。
- 所有文案走 i18n（英文源串作 key），7 语言键集对称。
- 弹窗不展示重置次数、Email、User ID、额外限额等区块。

## 例子

渠道 12 响应 `data` 为 `{"five_hour":{"utilization":96.2},"seven_day":{"utilization":88},"seven_day_opus":{"utilization":41}}`：弹窗渲染三张窗口卡——「5小时窗口 96.2%」红色、「7天窗口 88%」橙色、「7天 Opus 窗口 41%」默认色，各带同色进度条；状态卡显示套餐 badge（如 max）、`HTTP 200`、渠道名 `Claude 订阅 C (#12)`。另一渠道响应为 `{"limits":[{"kind":"session","percent":40},{"kind":"weekly_all","percent":85}]}`：渲染「session 40%」默认色与「weekly_all 85%」橙色两张卡。

## 验收

- [ ] 顶层对象形态三窗口响应渲染三张卡，96.2 红、88 橙、41 默认，进度条同色。
- [ ] limits 数组形态按 kind 渲染，percent 着色规则一致。
- [ ] 响应含 extra_usage 时不出现对应窗口卡。
- [ ] 端点响应含 subscription_type 时渲染套餐 badge，缺失不渲染。
- [ ] 敏感信息隐藏开启时渠道名与 ID 显示掩码。
- [ ] 端点 success:false 时首次点击 toast 报错不开弹窗；弹窗内刷新失败显示错误横幅。
- [ ] 原始 JSON 区内容与端点响应一致且可复制。
- [ ] 新增文案在 7 语言 locale 中键集对称。
