---
name: list-payload-display-name
description: 管理端使用日志/任务日志/审计日志/看板流向接口每条记录附带 display_name；按页内去重用户 ID 批量查一次 users 表；缺失用户为空串。
---

# 列表响应附带显示名

## 目的

日志与审计记录只持久化 `user_id` 与 `username`，前端要显示显示名就必须由接口补齐；统一在服务端按页补齐，避免前端逐条请求用户信息。

## 逻辑

1. 覆盖四个管理端接口的响应记录：
   - 使用日志列表（管理员视角，含搜索）；
   - 任务日志列表（管理员视角）；
   - 审计日志列表（管理员视角）；
   - 看板流向数据（管理员 / root 视角）。
2. 每条记录新增字段 `display_name`（string），值为 `users.display_name` 的当前值；该字段不落库，仅在响应中出现。
3. 补齐方式：取当前页所有记录的 `user_id` 去重后，一次查询 users 表得到 `id → display_name` 映射，再回填到每条记录；一页只查一次，不随记录数增长。
4. `user_id` 在 users 表中不存在（含已软删除）时 `display_name` 为空串；`user_id` 为 0 的记录不参与查询，`display_name` 为空串。
5. 补齐失败（users 表查询出错）时整个列表请求返回错误，不返回缺字段的半成品。
6. 用户自己视角的日志 / 任务 / 审计 / 流向接口不受影响，不新增字段。

## 约束

- 响应记录含 `display_name` 键，类型 string，缺失用户为 `""` 而非 `null` 或省略。
- 一页 N 条记录对 users 表只产生 1 次查询（N ≥ 1）；页内无有效 `user_id` 时 0 次。
- 分页总数、排序、筛选行为与补齐前完全一致。
- 三种主库（SQLite、MySQL、PostgreSQL）行为一致。

## 例子

users 表：`id=7, username=zhangsan, display_name=张三`；`id=8, username=lisi, display_name=""`；`id=9` 已删除。
管理员请求使用日志第一页，得到三条记录 `user_id` 分别为 7、8、9、7。

- 对 users 表只执行一次 `WHERE id IN (7, 8, 9)` 查询。
- 四条记录的 `display_name` 依次为 `张三`、`""`、`""`、`张三`。
- 响应 `total` 与补齐前相同。

## 验收

- [ ] 管理员使用日志、任务日志、审计日志、看板流向四个接口的记录均含 `display_name`。
- [ ] 上例四条记录得到 `张三`、`""`、`""`、`张三`。
- [ ] 一页多条记录只触发一次 users 表查询；全部 `user_id=0` 时不查询。
- [ ] users 表查询失败时接口返回错误而非缺字段列表。
- [ ] 用户自身视角的对应接口响应不新增 `display_name`。
