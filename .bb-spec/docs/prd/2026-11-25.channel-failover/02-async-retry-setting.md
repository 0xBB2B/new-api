---
name: 02-async-retry-setting
description: 「失败重试次数」只管异步任务，设置页标签和说明写清异步任务与其他请求各自怎么重试
date: 2026-11-25
---

# 重试次数设置只管异步任务

## 用户故事
### P0：作为管理员，我想要从设置页看懂「失败重试次数」管什么，以便不会以为调大它就能让对话请求多试几次

**用例（主路径）**
- 场景：管理员用中文界面打开系统设置 → 请求策略 → 会话与重试。
- 操作：查看重试预算卡片。
- 预期：第一个字段的标签是「异步任务最多重试次数」，输入框下面的说明是「不含首次，按分组计算。其他请求会把提供该模型的渠道各试一次：先试 Claude 订阅、ChatGPT 订阅渠道，再试其他渠道。」

**用例（英文界面）**
- 场景：界面语言切到英文。
- 操作：同上。
- 预期：标签是 `Maximum retries for async tasks`，说明是 `Excludes the first attempt. Counted per group. Other requests try each channel that serves the model once: Claude subscription and ChatGPT subscription channels first, then other channels.`

### P0：作为提交视频任务的用户，我的任务重试行为和以前一样

**用例（主路径）**
- 场景：「失败重试次数」为 1，视频模型 `v1` 有 3 个渠道，前两个被选中的都返回 500。
- 操作：提交一个视频任务。
- 预期：一共请求 2 次（首次 1 次，重试 1 次），然后返回错误。

## 约束
- C1：「失败重试次数」只决定 async task（异步任务）的重试次数，规则和改动前一样（不含首次，按分组计算）；同步请求不读这个设置。
- C2：第一个字段的标签和说明换成下表的文字。七种界面语言都要有翻译；中文、英文按下表写，其他语言照这个意思翻译。

  | 位置 | 英文 | 中文 |
  |---|---|---|
  | 标签 | `Maximum retries for async tasks` | 异步任务最多重试次数 |
  | 说明 | `Excludes the first attempt. Counted per group. Other requests try each channel that serves the model once: Claude subscription and ChatGPT subscription channels first, then other channels.` | 不含首次，按分组计算。其他请求会把提供该模型的渠道各试一次：先试 Claude 订阅、ChatGPT 订阅渠道，再试其他渠道。 |

- C3：卡片的其他部分不变：卡片标题「重试预算」、第二个字段「自动重试状态码」的标签、占位文字和说明、页面布局。第一个输入框仍然只能填 0～99 的整数，超出范围时的报错和保存行为也不变。
- C4：旧的标签 `Maximum retries` 和旧的说明 `Excludes the first attempt. Counted per group.`，在没有别处使用的情况下，从各语言的翻译文件里删掉。

## 验收
- [ ] C1：「失败重试次数」为 1，视频任务前两个渠道失败 → 一共请求 2 次后返回错误；同一设置下，同步请求仍然试完全部渠道
- [ ] C2：中文界面、英文界面分别显示表里的标签和说明；七个语言的翻译文件里都有这两个键
- [ ] C3：卡片标题、第二个字段、布局和改动前一致；填 100 → 字段报错，页面提示修正后再保存
- [ ] C4：搜索翻译文件，旧的两个键已经删掉（另有使用的除外）

## 验证路径
1. 用中文界面打开系统设置 → 请求策略 → 会话与重试 → 看到新的标签和说明。
2. 切到英文界面 → 看到英文的标签和说明。
3. 把第一个字段改成 100 → 字段报错；改回 2，保存 → 保存成功，刷新后值是 2。
4. 「失败重试次数」设为 1，提交视频任务并让前两个渠道失败 → 只请求了 2 次。
