---
name: single-key-only
description: Claude 订阅渠道为单账号单渠道,禁止 batch 创建与多 key;前端拒绝、后台刷新跳过 multi-key。
---

# 单 key 约束

## 目的
一份 OAuth 凭据对应一个订阅账号,渠道与账号一一对应,禁止把订阅渠道当作多 key 池或批量创建。

## 逻辑
- 该渠道类型不支持 multi-key(多 key 聚合)与 batch(批量创建多个渠道)。
- 前端在创建/编辑该类型渠道时隐藏或禁用 batch、多 key 入口,并在校验层拒绝多 key 提交。
- 后台自动刷新任务遇到被标记为 multi-key 的该类型渠道时跳过(不刷新)。

## 约束
- 该类型渠道无法以 batch 模式创建。
- 该类型渠道的 Key 只承载单份 OAuth 凭据,不接受多 key 分隔形态。
- 后台刷新对 multi-key 渠道跳过。

## 例子
- 管理员选择 `Claude Subscription` 类型 → 表单不出现「批量创建」入口;粘贴多行/多份凭据尝试作多 key → 校验拒绝并提示单账号单渠道。
- 某历史遗留被标记 multi-key 的该类型渠道 → 后台刷新 tick 跳过它,不尝试刷新。

## 验收
- [ ] 该类型渠道无法通过 batch 模式创建。
- [ ] 前端对该类型提交多 key 时校验拒绝。
- [ ] 后台自动刷新跳过 multi-key 的该类型渠道。
