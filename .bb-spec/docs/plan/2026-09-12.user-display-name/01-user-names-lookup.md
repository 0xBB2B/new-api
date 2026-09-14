---
name: user-names-lookup
description: model 层新增按用户 ID 批量取 username/display_name 的查询；看板按用户分组查询改用它，去掉内联重复逻辑。
---

# 用户名称批量查询

## 目标

model 层提供一个「给一组用户 ID，一次查询返回每个 ID 的 username 与 display_name」的入口，供使用日志、任务日志、审计日志、看板流向与看板按用户分组五处回填共用。

## 业务规则（来源：spec list-payload-display-name）

- 一页 N 条记录对 users 表只产生 1 次查询；输入去重后为空（全是 0）时 0 次查询。
- users 表中不存在的 ID（含已软删除）不出现在结果中，调用方据此得到空串。
- users 表查询失败时向上返回错误，调用方整体失败，不返回缺字段的半成品。
- 三种主库（SQLite、MySQL、PostgreSQL）行为一致：只用 GORM `Table("users").Select(...).Where("id IN ?", ids)`，不写方言 SQL。

## 涉及文件

- `model/user.go` — 修改（新增查询函数与返回结构）
- `model/usedata.go` — 修改（`GetQuotaDataGroupByUser` 改用新查询，删除内联的 users 批量查询）
- `model/user_names_lookup_test.go` — 新建（本主题后端唯一新增测试文件；02 的日志/审计用例也放这里）
- `model/usedata_flow_test.go` — 修改（既有 `TestGetQuotaDataGroupByUserFillsDisplayName` 保持通过，不改断言）

## 函数清单

### model/user.go（修改）

| 函数 / 类型 | 职责 |
|---|---|
| `UserNames`（新增结构体） | 承载一个用户的 `Username` 与 `DisplayName` 两个字符串 |
| `GetUserNamesByIds`（新增） | 输入用户 ID 切片；内部去重并剔除 0；为空直接返回空 map 不查库；否则对主库 `DB` 的 users 表执行一次 `Select("id, username, display_name")` + `Where("id IN ?")`，返回 `map[int]UserNames`；查询失败返回 error |

### model/usedata.go（修改）

| 函数 | 职责 |
|---|---|
| `GetQuotaDataGroupByUser`（既有，改逻辑） | 收集行的 `UserID` 后调用 `GetUserNamesByIds`，用返回 map 回填 `DisplayName`；删除原先内联的 `userIDSet` / users 查询 / `displayNameByID` 三段代码 |

## 协作关系

- `GetUserNamesByIds` 只依赖主库 `DB`（users 表在主库）；日志 / 审计记录虽在 `LOG_DB`，回填时同样调用本函数查主库。
- 消费方：02（`GetAllLogs`、`GetAuditLogs`、流向查询）、03（`tasksToDto`）、本 plan 内的 `GetQuotaDataGroupByUser`。
- 软删除用户：users 表 `DeletedAt` 非空的行被 GORM 默认过滤，天然不出现在结果中，无需额外条件。

## 验证方式

- 测试入口：`model` 包内直接调用 `GetUserNamesByIds`、`GetQuotaDataGroupByUser`；测试 DB 为 `model` 包 `TestMain` 提供的 SQLite 内存库，每个用例用 `truncateTables(t)` 清理后自行种入 users。
- 测试输入：种入 `users`：`{id:7, username:"zhangsan", display_name:"张三"}`、`{id:8, username:"lisi", display_name:""}`；再创建 `{id:9, username:"deleted"}` 后软删除。
- 预期结果：
  - 输入 `[7, 8, 9, 7, 0]` → map 恰含 7 与 8 两个键，`7 → {zhangsan, 张三}`，`8 → {lisi, ""}`；9 与 0 不在 map 中。
  - 输入 `[0, 0]` 与空切片 → 返回空 map、无 error（可用 GORM `DryRun`/回调计数或断言不触碰 DB 的等价手段证明 0 次查询，若无稳定手段则只断言结果）。
  - `GetQuotaDataGroupByUser` 对既有用例 `TestGetQuotaDataGroupByUserFillsDisplayName` 结果不变。
- [ ] 上述三组输入的返回值与 error 均符合预期。
- [ ] `go build ./...` 通过；`go test ./model/ -run 'UserNames|QuotaDataGroupByUser'` 通过。
