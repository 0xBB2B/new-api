---
name: task-display-name
description: 管理端任务日志列表每条记录附带 display_name；tasksToDto 改为按页批量查一次 users 同时回填 username 与 display_name。
---

# 任务日志响应附带显示名

## 目标

管理员请求任务日志列表时，每条任务带上 `display_name`；用户回填从「逐用户走缓存」改为「按页一次批量查 users」。

## 业务规则（来源：spec list-payload-display-name）

- 任务日志列表（管理员视角）每条记录新增 `display_name`（string），值为 users.display_name 当前值；不落库。
- 一页只对 users 表查一次：当前页任务的 user_id 去重后批量查询再回填。
- user_id 不存在（含软删除）或显示名为空时 `display_name` 键缺席，绝不为 null；前端按空串处理。
- users 表查询失败时整个列表请求返回错误。
- 用户自身视角任务列表不受影响，不新增字段。
- 分页总数、排序、筛选行为不变。

## 涉及文件

- `model/task.go` — 修改（`Task` 结构体加字段）
- `dto/task.go` — 修改（`TaskDto` 加字段）
- `relay/relay_task.go` — 修改（`TaskModel2Dto` 透传）
- `controller/task.go` — 修改（`tasksToDto` 与其管理员调用点）
- `controller/task_log_view_test.go` — 修改（追加回填用例）

## 成品定义（API 契约增量）

```text
GET /api/task/            (AdminAuth)  items[].display_name: string   // 新增，缺失用户时键缺席；username 语义不变
GET /api/task/self        (UserAuth)   响应不变
```

## 函数清单

### model/task.go（修改）

| 字段 | 职责 |
|---|---|
| `Task.DisplayName`（新增） | `json:"display_name,omitempty" gorm:"-"`；紧随 `Username` |

### dto/task.go（修改）

| 字段 | 职责 |
|---|---|
| `TaskDto.DisplayName`（新增） | `json:"display_name,omitempty"`；紧随 `Username` |

### relay/relay_task.go（修改）

| 函数 | 职责 |
|---|---|
| `TaskModel2Dto`（既有，加一行） | 把 `task.DisplayName` 赋给 `TaskDto.DisplayName` |

### controller/task.go（修改）

| 函数 | 职责 |
|---|---|
| `tasksToDto`（既有，改逻辑） | `fillUser` 为真时：收集任务 `UserId` 调 `model.GetUserNamesByIds` 一次，逐任务回填 `Username` 与 `DisplayName`；删除原先按用户逐个 `GetUserCache` 的循环与 `UserBase` map。查询失败向调用方返回 error（函数签名增加 error 返回，管理员列表处理器据此 `common.ApiError`；自身视角调用点 `fillUser=false` 不触发查询） |

## 协作关系

- 依赖 01 的 `model.GetUserNamesByIds`。
- 管理员列表处理器（`controller/task.go` 中调用 `tasksToDto(items, true, role)` 的函数）接住 error；自身视角处理器传 `false`，行为不变。
- 既有 `controller/task_log_view_test.go` 各用例均以 `fillUser=false` 调用，签名变化后随之调整调用方式即可。
- 消费方：05（任务日志前端）。

## 验证方式

- 测试入口：`controller` 包内调用 `tasksToDto`；DB 用与 `setupAccessTokenAudit` 相同的 SQLite 内存库初始化方式，种入 users。
- 测试输入：users `{7, zhangsan, 张三}`、`{8, lisi, ""}`；两个 `model.Task`，`UserId` 分别为 7 与 8，再加一个 `UserId=9`（无此用户）。
- 预期结果：
  - `fillUser=true, RoleAdminUser`：三个 DTO 的 `Username` 依次 `zhangsan`、`lisi`、`""`；`DisplayName` 依次 `张三`、`""`、`""`。
  - `fillUser=false, RoleCommonUser`：三个 DTO 的 `Username` 与 `DisplayName` 均为 `""`，且不触碰 users 表。
  - 序列化后 `display_name` 键仅出现在第一条。
- [ ] 上述断言通过；既有四个 `TestTaskLogDTO*` 用例通过。
- [ ] `go build ./... && go test ./controller/ -run 'TaskLog'` 通过。
