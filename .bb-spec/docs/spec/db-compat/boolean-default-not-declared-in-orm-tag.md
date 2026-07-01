---
name: boolean-default-not-declared-in-orm-tag
description: bool 字段禁用 ORM default tag 声明默认值；改由代码层（构造/归一化/hook）赋值。
---

# 布尔默认值不写在 ORM tag 里

## 目的

避免各方言对布尔默认值字面量的差异触发 AutoMigrate 反复 ALTER TABLE，同时让默认值语义留在代码层可被单测覆盖。

## 逻辑

不同方言对布尔默认值的存储表达差异明显：MySQL 常见为 `tinyint` 存 `0/1`、PostgreSQL 存 `true/false`、SQLite 无原生布尔。ORM 的 `default:true|false` tag 在 AutoMigrate 时会翻译成方言字面量；重启时若 ORM 读回来的默认值字面量与源码 tag 字面量在字符层面不完全一致（例如 MySQL 返回 `'1'` 而 tag 是 `'true'`），会被误判为 schema 漂移并反复发起 `ALTER TABLE`。

做法：结构体 tag 上不写布尔默认值。默认值语义由构造函数、请求 DTO 归一化、GORM 的 CreateHook 或 service 层保证。整数、字符串等基础类型的 `default` tag 不受此限。

## 约束

- 任何 `bool` 类型字段的 gorm tag 中不得出现 `default:true` 或 `default:false`。
- 布尔字段的默认值改由代码层显式赋值：结构体字面量初始化、请求 DTO 归一化、hook、或 service 层构造。
- 在 schema 与数据未变的情况下重启，AutoMigrate 不应对该字段发起 `ALTER TABLE`。
- 规则可通过 grep `bool` 字段 tag 中 `default:(true|false)` 命中数为 0 证伪。

## 例子

- 输入：一张表的 `enabled` 布尔字段业务默认为 `true`。
- 过程：结构体定义只写 `bool`，无 default tag；构造函数或 service 层在插入前把 `enabled` 显式设为 `true`；请求 DTO 归一化时若客户端未传，填 `true`。
- 预期结果：MySQL 与 PostgreSQL 上重复启动不出现幽灵 `ALTER TABLE`；未显式赋值的实体插入后数据库里存的仍是 `true`。

## 验收

- [ ] 全仓 grep `gorm:"...default:(true|false)..."` 且字段类型为 `bool` 的命中数为 0。
- [ ] 有默认值需求的布尔字段能通过单测证明：未显式赋值时构造/归一化/hook 会填入预期值。
- [ ] 同 schema 状态下重复启动，日志中该字段不出现 `ALTER TABLE`。
