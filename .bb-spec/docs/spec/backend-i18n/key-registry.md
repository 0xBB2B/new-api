---
name: key-registry
description: 后端 i18n 消息 key 用类型化常量集中在 i18n/keys.go 定义；业务代码只引用常量，禁传字面串。
---

# 消息 key 集中登记表

## 目的

让所有可翻译消息有唯一权威登记表；调用点靠编译器保证 key 存在；避免拼写漂移与 locale/代码脱节。

## 逻辑

所有可翻译消息 key 都以 `Msg<Domain><Concept>` 命名的 exported 常量集中定义在 `i18n/keys.go`。常量按业务域分组（Common / Auth / Token / User / Distributor / OAuth / ...），组内用 `const` block 包住并配一行分组注释。

常量值形如 `domain.snake_case`（例如 `"auth.not_logged_in"` / `"distributor.invalid_request"`），与 locale YAML 里的顶层 key 逐字对应。

调用点写 `i18n.MsgXxx`（例如 `i18n.T(c, i18n.MsgAuthNotLoggedIn)`），不允许把 `"auth.not_logged_in"` 这类字面串直接传给翻译入口。

locale 文件里的 key（en / zh-CN / zh-TW 三份）必须与常量表 1:1 对齐——多语言之间只允许译文差异、不允许 key 差异。

## 约束

- 业务包（middleware / controller / service / oauth / model）内禁写形如 `"xxx.yyy"` 的翻译 key 字面串。
- 常量必须在 `i18n/keys.go` 集中登记，禁在业务包本地新建独立常量表分流。
- 常量命名前缀 `Msg` 固定；常量值必须是 `domain.snake_case`。
- 新增常量必须同时给 en / zh-CN / zh-TW 三份 locale 添加对应 key。
- 禁复用同一常量值承担两种业务含义。

## 例子

- 输入：service 层需要抛出可翻译错误「订阅已过期」。
- 过程：先在 `i18n/keys.go` 的 Subscription 组添加 `MsgSubscriptionExpired = "subscription.expired"`；再在 `en.yaml` / `zh-CN.yaml` / `zh-TW.yaml` 三份 locale 里加 `subscription.expired: "..."`；业务代码写 `i18n.T(c, i18n.MsgSubscriptionExpired)`。
- 预期结果：编译期就能捕获拼写错误；漏加任一 locale 时该语言下的用户会看到原始 key 字符串，便于 review 发现。

## 验收

- [ ] 全仓 grep `i18n\.T\(.*,\s*"[a-z]+\.[a-z_]+"` 命中数为 0（业务包不直传字面串）。
- [ ] 定义 `Msg\w+ = "` 的位置仅 `i18n/keys.go`。
- [ ] 三份 locale 的顶层 key 集合与常量表 1:1 对齐。
- [ ] 常量分组注释真实反映业务域，无跨域混编。
