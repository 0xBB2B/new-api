---
name: sqlite-schema-change-add-column-only
description: SQLite 分支的表结构演进只允许 ADD COLUMN 或整表重建，禁 ALTER COLUMN。
---

# SQLite 表结构演进仅允许 ADD COLUMN

## 目的

针对 SQLite `ALTER TABLE` 只支持有限操作的现实，明确 SQLite 分支能做什么、不能做什么，避免手写出运行时报错的 DDL。

## 逻辑

SQLite 的 `ALTER TABLE` 仅支持 `ADD COLUMN` / `RENAME` / `DROP COLUMN`（新版），不支持修改列类型或约束。凡涉及 SQLite 且要「改列」的场景，合法路径只有两条：

1. 绕开——利用 SQLite 类型亲和性自动兼容，代码 no-op；
2. 显式判断表是否存在，写死 `CREATE TABLE` 一次性建好新结构，再对已存在旧表用 `ADD COLUMN` 补缺失列。

跨方言的列类型迁移函数在 SQLite 分支必须 no-op，避免调用 `ALTER COLUMN` 之类会抛错的 DDL。

## 约束

- 跨方言列类型迁移函数的 SQLite 分支必须 `return`（no-op）。
- SQLite 补齐已有表的新列只允许 `ALTER TABLE ... ADD COLUMN`。
- SQLite 新建表走独立的 `CREATE TABLE` 分支，不依赖 ORM 迁移隐式生成的差异 DDL。
- SQLite 分支中不出现 `ALTER COLUMN` 关键字。
- 规则可通过 grep SQLite 分支代码块内 `ALTER COLUMN` 命中数为 0 证伪。

## 例子

- 输入：某张表在三种方言上都可能已存在，需要新增两列且旧列的类型从 `float` 迁到 `decimal`。
- 过程：SQLite 分支——若表不存在，走一次性写死 DDL 的 `CREATE TABLE`；若表存在，用 `PRAGMA table_info` 查已有列，缺失列逐一 `ADD COLUMN`；`float → decimal` 这一段跳过，因为 SQLite 类型亲和性会自动接受。
- 预期结果：SQLite 上重复启动不因不支持的 `ALTER COLUMN` 报错；结构与 MySQL/PostgreSQL 上 ORM 生成的差异不影响可用性；旧行数据无需搬迁。

## 验收

- [ ] 全仓 SQLite 分支代码块中 grep `ALTER COLUMN` 命中数为 0。
- [ ] 涉及列类型修改的路径在 SQLite 上 no-op（走「跳过」而非「尝试改」）。
- [ ] SQLite 新建表通过独立 `CREATE TABLE` 完成，不依赖 ORM 隐式差异。
- [ ] 补齐旧表新列时使用 `ADD COLUMN`；重复启动不报错。
