---
name: wallet-ui
description: 钱包页统计卡新增「下次重置」信息（时间 + 重置值），仅生效规则用户可见；收口六语言文案。
---

# 用户端下次重置展示

## 目标

有生效重置规则的用户在钱包页看到"下次重置时间"与"重置为多少"；无生效规则的用户页面无任何重置信息。

## 业务规则（来源：spec user-visibility）

- 展示两项：下次重置时间（与实际定时触发时点一致）、重置值（按站点额度展示方式换算：货币或 token）。
- 无生效规则时不渲染该信息区块，而非渲染空值。
- 文案走前端 i18n，随站点语言切换。

## 涉及文件

- `web/default/src/features/wallet/index.tsx` — 修改（透传 self 响应新字段）
- `web/default/src/features/wallet/components/wallet-stats-card.tsx` — 修改（新增 tile）
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` — 修改

## 函数清单

### wallet/index.tsx（修改）

| 位置 | 职责 |
|---|---|
| `Wallet` 的 self 数据装载 | `getSelf()` 响应中的可选 `quota_reset` 字段并入 `UserWalletData`（类型加可选字段），传给统计卡 |

### wallet-stats-card.tsx（修改）

| 位置 | 职责 |
|---|---|
| `WalletStatsCard` 的 stats 数组 | `quota_reset` 存在时追加"下次重置"tile：时间用 `formatTimestampToDate`（lib/format.ts 既有），值用 `formatQuota` 按站点货币展示；字段缺席时不追加 tile |

## 协作关系

- 数据来源：05 产出的 `GET /api/user/self` 可选 `quota_reset {period, reset_value, next_reset_time}`；字段缺席即"无生效规则"，前端不做二次规则判断。
- 换算复用 `lib/format.ts` 的 `formatQuota` 与 `lib/currency.ts` 的货币展示工具。
- 本文件同时是前端文案收口点：06/07/08 三份新增的 locale key 全部就位后统一跑 `bun run i18n:sync`，确认六语言 key 集合对称。

## 验证方式

- [ ] `bun run build` 通过；`bun run i18n:sync` 报告六语言无 missing key、无遗漏英文占位。
- [ ] 浏览器：全局规则启用后普通用户钱包页出现"下次重置"tile，时间与重置值正确（USD 展示模式下按汇率换算）。
- [ ] 给该用户打退出标记后刷新钱包页，tile 消失。
