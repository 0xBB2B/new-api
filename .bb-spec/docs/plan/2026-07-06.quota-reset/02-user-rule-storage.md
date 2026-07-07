---
name: user-rule-storage
description: dto.UserSetting 新增专属规则与退出标记字段，新增管理员专用端点写入，自服务路径保持不可写。
---

# 每用户规则存储与管理员写入端点

## 目标

管理员可为单个用户配置专属重置规则（周期 + 重置值）与退出自动重置标记；普通用户无法自改这两项配置。

## 业务规则（来源：spec rule-resolution / reset-semantics）

- 每用户专属规则含周期（daily/weekly/monthly 三选一）与重置值（≥0 整数），对全局规则是整体覆盖。
- 退出自动重置标记优先于专属规则：同时存在时该用户无生效规则。
- 专属规则仅管理员可设置；管理员操作用户受角色层级约束（不得操作同级/更高角色）。

## 涉及文件

- `dto/user_settings.go` — 修改（UserSetting 加字段）
- `controller/user.go` — 修改（新增端点 handler）
- `controller/audit.go` — 修改（auditContentTemplates 加 action）
- `router/api-router.go` — 修改（adminRoute 加路由）

## 成品定义（API 契约）

```text
POST /api/user/quota_reset_rule        （挂 adminRoute，middleware.AdminAuth()）

请求体：
{
  "user_id": 123,                       // 目标用户
  "rule": {                             // 可空：null 表示清除专属规则
    "period": "weekly",                 // daily | weekly | monthly
    "value": 100000                     // ≥0 整数，quota 内部单位
  },
  "opt_out": false                      // 退出自动重置标记
}

响应：既有管理端信封 {success, message}；校验失败与权限不足均 HTTP 200 + success:false。
```

## 函数清单

### dto/user_settings.go（修改）

| 结构/字段 | 职责 |
|---|---|
| `QuotaResetRule` | 新 struct：`Period string`（json `period`）、`Value int`（json `value`） |
| `UserSetting.QuotaResetRule` | 新字段 `*QuotaResetRule`，json `quota_reset_rule,omitempty`；nil = 未配置专属规则 |
| `UserSetting.QuotaResetOptOut` | 新字段 `bool`，json `quota_reset_opt_out,omitempty` |

### controller/user.go（修改）

| 函数 | 职责 |
|---|---|
| `UpdateUserQuotaResetRule`（新增） | 解析上方契约请求体；校验 period 经 `operation_setting.IsValidQuotaResetPeriod`、value ≥0；取目标用户并过 `canManageTargetRole`；`user.GetSetting()` 改两字段后 `model.UpdateUserSetting` 落库（自动刷新 setting 缓存）；`recordManageAuditFor(c, uid, "user.quota_reset_rule", {rule 摘要})` 审计 |

### controller/audit.go（修改）

| 位置 | 职责 |
|---|---|
| `auditContentTemplates` | 加 `"user.quota_reset_rule"` 英文兜底模板（含设置/清除与 opt_out 状态占位符） |

### router/api-router.go（修改）

| 位置 | 职责 |
|---|---|
| `adminRoute`（/api/user 管理区段） | 加 `POST /quota_reset_rule` → `controller.UpdateUserQuotaResetRule` |

## 协作关系

- 落库唯一入口 `model.UpdateUserSetting`（既有）：Marshal 整段 setting → `Update("setting")` → `updateUserSettingCache` 刷 Redis 哈希单字段。
- **负向约束（不改即合规，exec 时验证）**：`UpdateUserSettingRequest` 白名单（`PUT /api/user/setting`）与 `UpdateSelf` 的 sidebar_modules/language special-case 均不加新字段；`EditWithTx` 的 updates 白名单不扩——普通用户与管理员 `PUT /api/user/` 均无法触碰这两个字段。
- 下游消费方：规则解析读 `UserSetting.QuotaResetRule` / `QuotaResetOptOut`（03-reset-engine）；用户管理 UI 调本端点（07-admin-users-ui）。

## 验证方式

- [ ] 管理员对普通用户设置 `{period: "weekly", value: 100000}` 成功，`GET /api/user/:id` 的 setting 里可见新字段。
- [ ] `rule: null` 清除专属规则后 setting 中 `quota_reset_rule` 字段消失。
- [ ] period 传 `"hourly"` 或 value 传 `-1` 返回 success:false，setting 未变。
- [ ] 普通管理员对 Root 用户调用返回 success:false（角色层级校验）。
- [ ] 普通用户以自己身份 `PUT /api/user/setting` / `PUT /api/user/self` 携带 `quota_reset_opt_out: true` 提交后，setting 中该字段不变。
