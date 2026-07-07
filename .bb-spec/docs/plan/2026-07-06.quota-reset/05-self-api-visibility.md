---
name: self-api-visibility
description: GetSelf 返回体扩展 quota_reset 对象（周期、重置值、下次触发时点），无生效规则时省略。
---

# 用户自查接口扩展

## 目标

普通用户通过 `GET /api/user/self` 拿到自己的生效重置规则信息（下次重置时间与重置值），前端据此展示。

## 业务规则（来源：spec user-visibility / rule-resolution）

- 有生效规则的用户可见：下次重置时间（生效规则周期的下一个触发时点，服务器本地时区）与重置值。
- 展示信息来自该用户自己的生效规则：专属规则用户看到专属的周期与值，而非全局的。
- 无生效规则（全局关闭且无专属、退出自动重置标记、禁用）时不返回该字段——响应体省略而非空值。

## 涉及文件

- `controller/user.go` — 修改（GetSelf）

## 成品定义（API 契约增量）

```text
GET /api/user/self 响应 data 中新增可选字段：

"quota_reset": {
  "period": "weekly",              // 生效规则周期 daily | weekly | monthly
  "reset_value": 100000,           // 生效规则重置值（quota 内部单位，前端负责货币换算展示）
  "next_reset_time": 1784217600    // 下一个触发时点，秒级 Unix 时间戳
}

无生效规则时整个 quota_reset 字段缺席。
```

## 函数清单

### controller/user.go（修改）

| 函数 | 职责 |
|---|---|
| `GetSelf`（既有，加逻辑） | 用当前用户的 status 与 `user.GetSetting()` 调 `service.ResolveQuotaResetRule`；有生效规则则以 `service.NextQuotaResetTime` 算下次时点，向返回 map 添加 `quota_reset` 对象；无则不加 key |

## 协作关系

- 复用 03 产出的 `ResolveQuotaResetRule` 与 `NextQuotaResetTime`，解析口径与实际执行完全一致（同一函数），天然满足"展示时点 = 实际触发时点"。
- 消费方：钱包页展示 tile（08-wallet-ui）。

## 验证方式

- [ ] 全局规则启用时，普通用户 self 响应含 quota_reset，period/reset_value 与全局一致，next_reset_time 为正确的下一个本地 0 点边界。
- [ ] 专属规则用户返回专属的周期与值。
- [ ] 退出标记用户与全局关闭且无专属规则的用户，响应中无 quota_reset key。
