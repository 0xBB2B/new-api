---
name: 01-utilization-api
description: 后端新增管理员接口 GET /api/data/utilization，返回开通人数与深度/经常/中间/非活跃四类人数
---
# 利用率统计接口

## 目标
管理员调用 `GET /api/data/utilization`，拿到开通人数和四类人数；普通用户调用会被拒绝。

## 业务规则（来源：PRD 02-utilization-section）
- 开通用户：`role = 1`（普通用户）、`status = 1`（启用）、没被软删除。管理员（10）、Root（100）、已禁用、已删除的用户，既不计入开通人数，也不计入任何一类。
- 每人的 Token 用量 = `quota_data.token_used` 之和。`token_used` 记录的是输入 + 输出 Token；按次计费的任务写入的是 0。
- 两个滚动时间窗，以请求时刻 `now` 为准：近 7 天 = `created_at >= now - 7*24*3600`；近 30 天 = `created_at >= now - 30*24*3600`。`quota_data.created_at` 是按小时取整的桶起点，所以窗口边界的精度是 1 小时。
- 深度使用：近 7 天 > 100,000,000。正好 1 亿不算。
- 经常使用：近 7 天 > 1,000,000，包含深度使用。正好 100 万不算。
- 中间段（moderate）：近 7 天 ≤ 1,000,000，且近 30 天 > 1,000,000。
- 非活跃：近 30 天 ≤ 1,000,000，含没有任何记录的开通用户。计算方式为 开通人数 − 经常使用 − 中间段。
- 恒等式：深度使用 + (经常使用 − 深度使用) + 中间段 + 非活跃 = 开通人数。
- 数据看板开关关闭时照常统计，不在后端特殊处理；提示由前端显示。

## 涉及文件
- `model/usedata.go` 修改：新增结构体和统计函数
- `model/usedata_test.go` 新建：统计口径测试，包括三种数据库
- `controller/usedata.go` 修改：新增处理函数
- `router/api-router.go` 修改：在 `/data` 分组注册路由
- `middleware/access_token_routes.go` 修改：登记访问令牌的权限范围

## 成品定义

路由注册（`router/api-router.go`，放在 `dataRoute.GET("/users", ...)` 之后）：
```go
dataRoute.GET("/utilization", middleware.AdminAuth(), controller.GetUserUtilization)
```

访问令牌权限登记（`middleware/access_token_routes.go`，放在 `"GET /api/data/users"` 之后，和它一样用 `log:read`）：
```go
"GET /api/data/utilization":                 accessTokenScopeRule("log:read"),
```

API 契约：
```
GET /api/data/utilization
鉴权：AdminAuth（管理员、Root）；访问令牌需要 log:read
请求参数：无（时间窗以服务端当前时间为准）

200 成功：
{
  "success": true,
  "message": "",
  "data": {
    "total_users": 36,      // 开通人数
    "deep_users": 13,       // 深度使用：近 7 天 > 1 亿
    "frequent_users": 21,   // 经常使用：近 7 天 > 100 万，含深度使用
    "moderate_users": 13,   // 中间段：近 7 天 ≤ 100 万 且 近 30 天 > 100 万
    "inactive_users": 2     // 非活跃：近 30 天 ≤ 100 万，含无记录
  }
}

数据库出错：沿用 common.ApiError，返回 {"success": false, "message": "<错误>"}
普通用户 / 未登录：由 AdminAuth 拒绝，拿不到 data
```

Go 结构体（`model/usedata.go`）：
```go
type UserUtilization struct {
	TotalUsers    int64 `json:"total_users"`
	DeepUsers     int64 `json:"deep_users"`
	FrequentUsers int64 `json:"frequent_users"`
	ModerateUsers int64 `json:"moderate_users"`
	InactiveUsers int64 `json:"inactive_users"`
}
```

## 函数清单
### model/usedata.go
| 函数名 | 职责 |
|---|---|
| `GetUserUtilization` | 按调用方传入的参考时刻做统计，返回 `*UserUtilization`。分两步：用 `DB.Model(&User{})` 数开通人数（GORM 会自动排除软删除的用户）；再用 `quota_data` 联表 `users`，只取近 30 天的记录，筛出开通用户，按 `quota_data.user_id` 分组，用 `SUM(CASE WHEN quota_data.created_at >= ? THEN token_used ELSE 0 END)` 算近 7 天，用 `SUM(token_used)` 算近 30 天。最后在 Go 里按阈值分类。阈值和时间窗写成函数内的局部常量 |

### controller/usedata.go
| 函数名 | 职责 |
|---|---|
| `GetUserUtilization` | 用 `common.GetTimestamp()` 作参考时刻，调用 `model.GetUserUtilization`，按契约返回 JSON；出错时用 `common.ApiError` |

## 协作关系
`router → middleware.AdminAuth → controller.GetUserUtilization → model.GetUserUtilization → DB(users, quota_data)`。
SQL 约束：
- 联表条件写 `users.id = quota_data.user_id`，并显式加上 `users.deleted_at IS NULL`。
- `role`、`status`、`deleted_at` 都不是保留字，列名一律带表名前缀，避免歧义。
- 不用方言专属函数。`CASE WHEN` 和 `SUM` 在三种数据库上都能用。
- 布尔值和保留字列不涉及。

## 验证方式
- 测试入口：包级函数 `model.GetUserUtilization`（传入固定参考时刻 `now`），以及 `go test ./router/` 里已有的 `TestAccessTokenRouteRulesCoverEveryDashboardRoute`。
- 测试输入（`now = 1_800_000_000`，记录写在 `now - 3600` 这类整点之前的时刻，各用一条或多条 `quota_data` 记录）：
  - 13 个深度用户：近 7 天 2 亿；8 个经常用户：近 7 天 5,000,000；13 个中间用户：8 天前 2,000,000；2 个非活跃用户：近 30 天 500,000。
  - 边界用户 A：近 7 天正好 100,000,000。边界用户 B：近 7 天正好 1,000,000（近 30 天也正好 1,000,000）。
  - 用户 C：没有任何记录。用户 D：只有 `token_used = 0` 的记录。用户 E：31 天前 1,000,000,000。
  - 被排除的用户：已禁用普通用户、软删除普通用户、管理员、Root，每人近 7 天各 500,000,000。
- 预期结果：
  - 基础 36 人：`{total 36, deep 13, frequent 21, moderate 13, inactive 2}`。
  - 每加一组边界用户，都能单独断言它落在哪一类：A → frequent +1、deep 不变；B → inactive +1；C、D、E → inactive 各 +1。
  - 被排除的用户不改变任何数字。
  - 没有任何开通用户时，结果全是 0。
- 三种数据库：同一组用例用 `TEST_MYSQL_DSN`、`TEST_POSTGRES_DSN` 再跑一遍，环境变量没设时 `t.Skip`。写法参考 `model/migration_dialector_test.go` 的 `TestMigrationSchemaStability`：用 `chooseDB` 打开连接，对 `User`、`QuotaData` 做 `AutoMigrate`，临时把全局 `DB` 换成这个连接，用 `t.Cleanup` 还原并清理数据。
- [ ] SQLite 下全部用例通过
- [ ] MySQL 8 和 PostgreSQL 16 用 docker 起真实实例，设好 DSN 后全部通过；记录版本、命令和结果
- [ ] `go test ./router/` 通过（新路由已登记访问令牌权限）
- [ ] 启动服务后，管理员 curl `/api/data/utilization` 返回 200 和上面的结构；普通用户会话调用被拒绝
