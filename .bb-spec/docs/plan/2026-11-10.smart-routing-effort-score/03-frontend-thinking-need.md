---
name: 03-frontend-thinking-need
description: 日志详情打分标签显示「思考需求」，档位力度 ⓘ 换成新说明，七种语言补齐翻译
---
# 前端显示思考需求

## 目标
日志详情的「打分标签」行显示「思考需求」；档位「思考力度」旁 ⓘ 的说明换成按思考需求按比例选档的新规则。

## 业务规则（来源：PRD 03-log-thinking-need、02-follow-effort-by-score C9）
- 日志里 `other.smart_routing.labels.effort_score` 有三种情况：
  - 是数字时，显示「思考需求: 0.72」，保留两位小数，按界面语言格式化，写法和「可信度」一样（0.5 显示 0.50）；
  - 是字符串 `"invalid"` 时，显示「思考需求: 无效」；
  - 没有这个键时（旧日志），不显示这一项。
- 显示位置在「要想多深」后面，其余各项顺序不变：任务类型、要想多深、思考需求、工作量、出错代价、工作类型、可信度、用户反馈。
- 工具续跑、打分失败的日志没有 `labels`，和现在一样不显示「打分标签」行。
- 英文界面：标签名 "Thinking need"，无效值 "Invalid"。
- 档位 ⓘ 说明只换「跟随打分」那一段，其他两句保持现在的写法。模型不支持思考时，那一行用的是同一段说明。
- 决策理由：放在「打分标签」行，管理员可以拿它和「要想多深」对照；无效时写「无效」，管理员能看出力度为什么是最高档（用户 2026-11-10 在原型确认时选定）。

## 涉及文件
- 修改 `web/src/features/usage-logs/types.ts`（`SmartRouteLabels`）
- 修改 `web/src/features/usage-logs/components/dialogs/smart-route-section.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/tier-effort-field.tsx`
- 修改 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`（删掉旧说明的键，加新说明、"Thinking need"、无效值的键）
- 修改 `web/src/features/usage-logs/components/__tests__/detail-preview.test.tsx`
- 修改 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`（如果里面有断言旧说明文字）

## 成品定义
`SmartRouteLabels` 新增字段：

```ts
  effort_score?: number | 'invalid'
```

ⓘ 说明的英文键（也是英文原文），替换旧键 "Not overridden keeps the value sent by the client and switches to the closest level when the model does not support it. Follow scoring takes the thinking depth from scoring (low to low, medium to medium, high to high) and limits it between the minimum and maximum; the maximum is used when scoring fails. The dropdowns only list the levels this model supports"：

```text
Not overridden keeps the value sent by the client and switches to the closest level when the model does not support it. Follow scoring uses the thinking need from scoring (0 to 1, higher when the goal is vague or the problem is hard) and picks proportionally among the levels this model supports between the minimum and maximum; with 3 levels, 0 to 0.33 picks the lowest and 0.67 to 1 the highest. The highest of them is used when scoring fails. The dropdowns only list the levels this model supports
```

简体中文翻译：

```text
不覆盖：保留客户端传来的值，模型不支持这一档时自动换成最接近的档位。跟随打分：打分会给出「思考需求」（0～1，目标越模糊、问题越难越大），按比例在最低和最高之间、这个模型支持的档位里选；例如范围里有 3 档，0～0.33 取最低一档，0.67～1 取最高一档。打分失败时用其中最高的一档。力度下拉只列这个模型支持的档位
```

新增的标签文字（键 → 简体中文）：

```text
"Thinking need" → "思考需求"
"Invalid_routing"（t('Invalid', { context: 'routing' })）→ "无效"
```

其余五种语言（zh-TW、fr、ru、ja、vi）按 `i18n-translate` skill 补齐同义翻译，繁体中文用「思考需求」「無效」。

## 函数清单
### web/src/features/usage-logs/components/dialogs/smart-route-section.tsx
| 函数名 | 职责 |
|---|---|
| `labelNames` | 加 `effort_score` → `t('Thinking need')` |
| `labelValues` | 加 `effort_score` 的值映射：`invalid` → `t('Invalid', { context: 'routing' })` |
| `LABEL_ORDER` | 在 `reasoning` 后面插入 `effort_score` |

数字格式化沿用 `labelValueText` 现有的数字分支，不新写格式化逻辑。

### web/src/features/system-settings/request-policies/smart-routing/tier-effort-field.tsx
| 函数名 | 职责 |
|---|---|
| `TierEffortField` | `help` 里 `t(...)` 的键换成上面的新英文键 |

## 协作关系
日志数据来自后端主请求日志的 `labels`（形状见计划 01：`effort_score` 是数字或 `"invalid"`）。翻译先跑 `bun run i18n:sync`（在 `web/` 下），再按 `i18n-translate` skill 补齐七种语言；不再使用的旧说明键要从七个语言文件里删掉。

## 验证方式
- 测试入口：`cd web && bunx vitest run src/features/usage-logs src/features/system-settings/request-policies`；页面上：使用日志 → 打开智能路由请求的日志详情；系统设置 → 请求策略 → 智能路由 → 编辑虚拟模型 → 档位 → 鼠标移到「思考力度」旁的 ⓘ。
- 测试输入：日志 `other.smart_routing.labels` 分别为带 `"effort_score":0.72`、带 `"effort_score":0.5`、带 `"effort_score":"invalid"`、不带 `effort_score`；界面语言分别为中文和英文。
- 预期结果：
- [ ] 0.72 → 中文「打分标签」里「要想多深: 高」后面是「思考需求: 0.72」，英文是 "Thinking need: 0.72"；0.5 显示 0.50。
- [ ] `"invalid"` → 「思考需求: 无效」，英文 "Thinking need: Invalid"。
- [ ] 没有 `effort_score` → 不出现「思考需求」，其余各项和改动前一样。
- [ ] 工具续跑、打分超时的日志 → 没有「打分标签」行。
- [ ] ⓘ 说明在中文、英文界面分别显示上面成品定义里的全文；档位模型换成不支持思考的模型时，ⓘ 里是同一段文字。
- [ ] 七个语言文件都有新键，旧说明键已删除；`bunx tsc -b` 通过；前端 lint 通过。
