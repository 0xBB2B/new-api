---
name: 05-retry-setting-copy
description: 系统设置重试预算卡片第一个字段改为异步任务专用的标签和说明，补七种语言翻译
---
# 重试设置文案

## 目标
系统设置 → 请求策略 → 会话与重试 → 重试预算卡片的第一个字段，标签和说明写清它只管异步任务、其他请求怎么换渠道。

## 业务规则（来源：PRD 02-async-retry-setting，预览页已经用户确认）
- 只改 `web/src/features/system-settings/request-policies/retry-section.tsx` 第一个字段的标签和说明两处文字，样式、布局、校验（0～99 的整数）、保存行为都不变。
- 新文案（英文为翻译键原文）：

  | 位置 | 英文 | 中文（zh） |
  |---|---|---|
  | 标签 | `Maximum retries for async tasks` | 异步任务最多重试次数 |
  | 说明 | `Excludes the first attempt. Counted per group. Other requests try each channel that serves the model once: Claude subscription and ChatGPT subscription channels first, then other channels.` | 不含首次，按分组计算。其他请求会把提供该模型的渠道各试一次：先试 Claude 订阅、ChatGPT 订阅渠道，再试其他渠道。 |

- 七个语言文件 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json` 都加这两个键；zh-TW、fr、ru、ja、vi 按中文意思翻译，"Claude""ChatGPT"保持原文。
- 旧键 `Maximum retries`、`Excludes the first attempt. Counted per group.` 在 `web/src` 里没有别处使用后，从七个语言文件里删掉。
- 卡片标题「重试预算」、第二个字段「自动重试状态码」的标签、占位文字、说明都不动。
- 先读 `web/AGENTS.md`，按 i18n-translate skill 增删键，用 `bun run i18n:sync`（在 `web/` 下）检查。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/retry-section.tsx`
- 修改 `web/src/i18n/locales/en.json`、`zh.json`、`zh-TW.json`、`fr.json`、`ru.json`、`ja.json`、`vi.json`
- 修改 `web/src/features/system-settings/request-policies/__tests__/settings.test.tsx`、`__tests__/routing.test.tsx`（测试，Red 阶段：按可访问名 `Maximum retries` 找输入框的地方改成新标签）

## 函数清单
### web/src/features/system-settings/request-policies/retry-section.tsx
| 函数名 | 职责 |
|---|---|
| `RetrySection`（现有组件） | 第一个字段的 `t(...)` 键换成新英文原文 |

## 协作关系
无新增调用关系；只改文案与翻译。

## 验证方式
- 测试入口：在 `web/` 下运行 `bun run test -- request-policies`，以及 `bun run build`、`bun run i18n:sync`。
- 测试输入与预期：
  - 渲染重试设置区，按可访问名 `Maximum retries for async tasks` 能找到数字输入框，说明文字等于上表英文原文。
  - 现有用例（改值、超范围报错、保存）改用新可访问名后全部通过。
  - 中文环境下标签为「异步任务最多重试次数」。
- [ ] 七个语言文件都含两个新键，且不再含两个旧键
- [ ] `bun run build` 通过，`bun run i18n:sync` 无缺失
