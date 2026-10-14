---
name: key-registry
description: 后端自己翻译的文字（给 AI 客户端的错误、邮件、通知）的 key 用常量集中登记在 i18n/keys.go；网页控制台的提示不登记，直接用英文原文作 key。
---

# 后端翻译 key 集中登记表

## 目的

后端自己翻译的文字要有唯一的登记表：调用点靠编译器保证 key 存在，三份 yaml 语言文件不会和代码对不上。

## 逻辑

后端只翻译中间没有网页控制台的文字：返回给 AI 客户端的错误、邮件、通知（包括注册时自动建的令牌名）。这些文字的 key 以 `Msg<Domain><Concept>` 命名的导出常量集中定义在 `i18n/keys.go`，按用途分组（给 AI 客户端的错误 / 邮件 / 低额度通知 / 渠道通知），每组一个 `const` 块配一行分组注释。

常量值形如 `domain.snake_case`（例如 `"token.invalid"`、`"distributor.no_available_channel"`），和 `i18n/locales/{en,zh-CN,zh-TW}.yaml` 里的 key 逐字对应。

调用点写 `i18n.MsgXxx`（例如 `i18n.T(c, i18n.MsgTokenInvalid)`），不把 `"token.invalid"` 这类字面串直接传给翻译函数。

三份 yaml 的 key 集合必须和 `keys.go` 的常量完全一致。测试 `TestLocalesRenderEveryKey`（`i18n/i18n_test.go`）会逐个语言核对，并检查每个 key 都能渲染、渲染结果里没有残留的 `{{`。

网页控制台看到的提示不在这里登记：它们用英文原文作 key，由前端翻译，写法是 `common.NewMessage` / `common.ApiErrorT`。

## 约束

- 只有后端自己翻译的文字才能加进 `i18n/keys.go`；网页控制台的提示禁止新增常量。
- 业务包里禁写 `"xxx.yyy"` 形式的翻译 key 字面串。
- 常量只能在 `i18n/keys.go` 定义，禁在业务包里另建常量表。
- 常量名前缀固定为 `Msg`，值必须是 `domain.snake_case`。
- 新增常量必须同时给 en / zh-CN / zh-TW 三份 yaml 加对应 key；删除常量也要同时删掉三份 yaml 里的 key。
- 禁止同一个常量值承担两种业务含义。

## 例子

- 输入：给 AI 客户端返回一个新错误「令牌已过期」。
- 过程：在 `i18n/keys.go` 的「Errors returned to AI clients」组加 `MsgTokenExpired = "token.expired"`；三份 yaml 各加 `token.expired: "..."`；调用点写 `i18n.T(c, i18n.MsgTokenExpired)`。
- 预期结果：拼错常量名编译不过；漏加任何一份 yaml，`TestLocalesRenderEveryKey` 会失败。
- 反例：渠道管理接口要返回「渠道不存在」。这是网页控制台的提示，写 `common.ApiErrorT(c, "Channel does not exist")`，并把这句英文加进前端 7 个语言文件，不往 `i18n/keys.go` 加常量。

## 验收

- [ ] 全仓 grep `i18n\.T\(.*,\s*"[a-z]+\.[a-z_]+"` 命中数为 0。
- [ ] 定义 `Msg\w+ = "` 的位置只有 `i18n/keys.go`。
- [ ] `go test ./i18n/ -run TestLocalesRenderEveryKey` 通过。
- [ ] `i18n/keys.go` 里没有只被网页控制台接口使用的常量。
