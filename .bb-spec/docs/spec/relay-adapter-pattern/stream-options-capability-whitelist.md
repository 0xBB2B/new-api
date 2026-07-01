---
name: stream-options-capability-whitelist
description: 是否支持 OpenAI stream_options 由 relay-info 集中白名单决定；adapter 只读能力标志不自行决策。
---

# stream_options 集中白名单

## 目的

把「该 provider 能不能吃 `stream_options`」的能力判定收敛到一处集中维护，避免每个 adapter 各自散落 `if channelType == X` 判断导致易错易漂移。

## 逻辑

relay-info 层维护一份 `channelType → bool` 的白名单映射，列出支持 `stream_options` 的 provider 类型。relay info 初始化时读白名单，把结果写入统一能力标志字段（例如 `SupportStreamOptions`）。

adapter 在 `ConvertRequest` 阶段只读该能力标志：`info.SupportStreamOptions && info.IsStream` 决定是否向上游请求注入 `stream_options.include_usage=true`。禁止在 adapter 内散落 channel-type 比较。

白名单是显式加入制：默认不支持，新 provider 需显式登记。

## 约束

- adapter 禁根据 channel-type 常量自行判断是否支持 `stream_options`；只能读能力标志字段。
- 白名单默认拒绝：未登记视为不支持。
- 能力标志字段只由 relay info 初始化路径写入，adapter 只读不写。
- 新增支持 `stream_options` 的 provider 只需在白名单加一行，adapter 零改动。

## 例子

- 输入：新增 OpenAI 兼容 provider Z，其上游对 `stream_options` 报错。
- 过程：不在白名单登记 Z 的 channel type；Z 的 adapter 保留同款 `if info.SupportStreamOptions && info.IsStream { ... }` 代码。
- 预期结果：Z 走流式请求时不会向上游注入 `stream_options`，避免上游 400；未来 Z 支持后仅在白名单加一行，adapter 不改。

## 验收

- [ ] 白名单集中在 relay-info 一个位置，全仓仅一份定义。
- [ ] adapter 内 grep channel-type 常量与 `stream_options` 的组合判断命中数为 0。
- [ ] 能力标志字段全项目只读；写路径只有 relay info 初始化。
- [ ] 新增 provider 时 diff 仅涉及白名单一行；adapter 无 stream_options 相关改动。
