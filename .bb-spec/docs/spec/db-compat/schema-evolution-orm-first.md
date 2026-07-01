---
name: schema-evolution-orm-first
description: 表结构演进优先由 ORM AutoMigrate 承担；跨方言列类型改动才写手工分支且保证幂等。
---

# 表结构演进 ORM 优先

## 目的

利用 ORM 自动迁移吸收大多数结构变化，把手工 DDL 缩到「各方言语法迥异到 ORM 无法安全处理」的极少数场景，并保证这些场景可重复执行不失败。

## 逻辑

ORM 的自动迁移能承担「新增列、新增索引、新建表」等大多数结构变化。但「已有列的类型修改」在各方言语法迥异（PostgreSQL 是 `ALTER COLUMN TYPE`、MySQL 是 `MODIFY COLUMN`、SQLite 不支持），ORM 无法安全统一处理。

做法：启动迁移的主路径调用 ORM `AutoMigrate`；对特定列类型迁移额外写手工的方言分支函数。分支函数必须先查 `information_schema` 或等价元数据判断列当前类型，已是目标类型就 `return`（幂等）；对未识别方言 no-op 而非报错（避免因方言扩展导致启动失败）；跳过或裸 DDL 集中在一个函数内、只暴露单一入口给启动流程。

## 约束

- 启动迁移主路径必须走 ORM AutoMigrate；裸 DDL 只出现在明确的跨方言类型迁移辅助函数中。
- 跨方言类型迁移函数必须先检查目标列当前类型，已是目标类型直接 `return`（可重复执行不失败）。
- 跨方言类型迁移函数对未识别方言必须 no-op，不得 panic。
- 业务代码路径禁止出现方言相关的裸 DDL。

## 例子

- 输入：一张已有表的某个 `VARCHAR` 列需要迁移为 `TEXT`。
- 过程：启动先跑 AutoMigrate；然后调用类型迁移函数——PostgreSQL 分支查 `information_schema.columns.data_type`，若已是 `text` 则返回，否则 `ALTER ... TYPE text`；MySQL 分支查 `COLUMN_TYPE`，若已是则返回，否则 `MODIFY COLUMN`；SQLite 因类型亲和性无需迁移，直接返回。
- 预期结果：三种方言各自的空库、旧库、已升级库上重复启动均不失败；日志能标识是否真正执行了迁移。

## 验收

- [ ] 启动迁移入口以 AutoMigrate 为主路径，手工 DDL 集中在辅助函数中。
- [ ] 类型迁移函数在「已是目标类型」时跳过，重复执行不产生副作用也不报错。
- [ ] 类型迁移函数对未识别方言 no-op，不 panic。
- [ ] 业务代码路径 grep 方言 DDL 关键字（`ALTER COLUMN` / `MODIFY COLUMN`）命中数为 0。
