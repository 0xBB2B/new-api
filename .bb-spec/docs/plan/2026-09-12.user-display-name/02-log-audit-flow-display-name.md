---
name: log-audit-flow-display-name
description: 使用日志、审计日志、看板流向三个管理端列表响应的记录附带 display_name（缺失时键缺席）；按页去重批量回填；self 视角不变。
---

# 日志 / 审计 / 流向响应附带显示名

## 目标

管理员请求使用日志列表、审计日志列表、看板流向数据时，每条记录带上 `display_name`，前端无需再逐条查用户。

## 业务规则（来源：spec list-payload-display-name）

- 覆盖：使用日志列表（管理员视角，含搜索条件）、审计日志列表（管理员视角）、看板流向数据（管理员 / root 视角）。
- 每条记录新增 `display_name`（string），值为 users.display_name 当前值；不落库，仅出现在响应中。
- 一页只对 users 表查一次：取当前页记录的 user_id 去重后批量查询再回填。
- user_id 不存在（含软删除）、为 0 或显示名为空时该键缺席（Go 侧为空串 + omitempty），绝不为 null；前端按空串处理。
- users 表查询失败时整个列表请求返回错误。
- 用户自身视角接口（`/api/log/self`、`/api/audit/self`、`/api/data/flow/self`）不受影响，不新增字段。
- 分页总数、排序、筛选行为与补齐前完全一致。

## 涉及文件

- `model/log.go` — 修改（`Log` 结构体加字段；`GetAllLogs` 回填）
- `model/audit_log.go` — 修改（`AuditLog` 结构体加字段；`GetAuditLogs` 非 SelfView 回填）
- `model/usedata_flow.go` — 修改（`FlowQuotaData` 结构体加字段；admin / root 查询回填）
- `model/user_names_lookup_test.go` — 修改（01 新建，追加日志 / 审计用例）
- `model/usedata_flow_test.go` — 修改（追加流向 display_name 用例）

## 成品定义（API 契约增量）

```text
GET /api/log            (AdminAuth)   items[].display_name: string   // 新增，缺失用户时键缺席
GET /api/audit          (AdminAuth)   items[].display_name: string   // 新增，缺失用户时键缺席
GET /api/data/flow      (AdminAuth)   data[].display_name: string    // 新增；user_id 为 0 的聚合行键缺席

GET /api/log/self、GET /api/audit/self、GET /api/data/flow/self：响应不变，无 display_name 键。
```

## 函数清单

### model/log.go（修改）

| 函数 / 字段 | 职责 |
|---|---|
| `Log.DisplayName`（新增字段） | `json:"display_name,omitempty" gorm:"-"`；放在 `Username` 之后 |
| `GetAllLogs`（既有，加逻辑） | 在渠道名回填之后、返回之前：收集 `logs[i].UserId` 调 `GetUserNamesByIds`，把 `DisplayName` 回填到每条；查询出错返回该 error |

`GetUserLogs`、`GetLogByTokenId` 等用户视角函数不改。

### model/audit_log.go（修改）

| 函数 / 字段 | 职责 |
|---|---|
| `AuditLog.DisplayName`（新增字段） | `json:"display_name,omitempty" gorm:"-"`；放在 `Username` 之后 |
| `GetAuditLogs`（既有，加逻辑） | `Find` 成功后，若 `!filter.SelfView` 则收集 `UserId` 调 `GetUserNamesByIds` 回填 `DisplayName`；SelfView 时保持字段零值 |

三个新增字段统一用 `json:"display_name,omitempty"`：self 视角不回填因而键缺席；管理端视角缺失用户时键同样缺席，前端按空串处理。

### model/usedata_flow.go（修改）

| 函数 / 字段 | 职责 |
|---|---|
| `FlowQuotaData.DisplayName`（新增字段） | `json:"display_name,omitempty" gorm:"-"`；放在 `Username` 之后 |
| `fillFlowDisplayNames`（新增） | 收集行 `UserID` 调 `GetUserNamesByIds` 回填 `DisplayName`；与既有 `fillFlowTokenNames` 同风格 |
| `getAdminFlowQuotaData`、`getRootFlowQuotaData`（既有，加调用） | 现有回填链末尾追加 `fillFlowDisplayNames` |

`getSelfFlowQuotaData` 不改。

## 协作关系

- 三处回填均调用 01 的 `GetUserNamesByIds`（主库 `DB`）；日志与审计表在 `LOG_DB`，两库分离部署（含 ClickHouse 日志库）时也成立。
- `controller/log.go` `GetAllLogs`、`controller/access_token.go` `GetAuditLogs`、`controller/usedata.go` `GetAllFlowQuotaDates` 无需改动，字段随结构体序列化。
- 消费方：05（使用日志前端）、06（审计前端）、07（流向图前端）。

## 验证方式

- 测试入口：`model` 包内调用 `GetAllLogs`、`GetAuditLogs`、`GetFlowQuotaData`；SQLite 内存库，`truncateTables(t)` 后种入数据。
- 测试输入：users `{7, zhangsan, 张三}`、`{8, lisi, ""}`；user 9 已软删除。
  - logs：四条记录 user_id 依次 7、8、9、7。
  - audit_logs：三条记录 user_id 7、9、0。
  - quota_data：两行 user_id 7 与 8，`use_group` 非空。
- 预期结果：
  - `GetAllLogs` 返回四条 `DisplayName` 依次 `张三`、`""`、`""`、`张三`，`total` 为 4。
  - `GetAuditLogs(filter{SelfView:false}, …, RoleAdminUser)` 三条 `DisplayName` 为 `张三`、`""`、`""`；`filter{SelfView:true, UserId:7}` 返回记录 `DisplayName` 为 `""`。
  - `GetFlowQuotaData(…, role=RoleAdminUser)` 行 `DisplayName` 为 `张三` 与 `""`；`role=RoleCommonUser, userID=7` 的行 `DisplayName` 为 `""`。
- [ ] 上述三组断言通过（Go 侧 `DisplayName` 字段值：缺失用户为空串，序列化后键缺席）；既有 `TestGetFlowQuotaDataUsesQuotaDataRoleSpecificDimensions` 与 `controller/access_token_audit_test.go` 全部通过。
- [ ] `go test ./model/ ./controller/ -run 'Log|Audit|Flow'` 通过。
