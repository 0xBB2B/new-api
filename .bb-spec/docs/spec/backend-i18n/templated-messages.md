---
name: templated-messages
description: 带动态数据的翻译消息用 {{.Placeholder}} 模板 + map[string]any 注入；禁 fmt.Sprintf 或拼接。
---

# 模板化翻译消息

## 目的

让占位符位置随语言语序变化（中英句式差异）；保证同一 key 在所有 locale 里的参数集合一致；避免多语言下的参数漂移。

## 逻辑

locale YAML 中带动态部分的消息，用 `go-i18n/v2` 模板语法写占位符 `{{.Xxx}}`（例如 `"批量请求数量过多，最多 {{.Max}} 条"` / `"Too many items in batch request, maximum is {{.Max}}"`）。

调用点用 `i18n.T(c, key, map[string]any{"Xxx": value})` 传入同名 key 的参数 map。

同一 message key 在 en / zh-CN / zh-TW 三份 locale 里的占位符名称必须完全一致——只有译文变、占位符不变。

禁在调用方用 `fmt.Sprintf(i18n.T(c, key), value)` 或字符串拼接把动态部分塞进翻译串——这会把参数固定到某个语序上。

占位符命名首字母大写（`Error` / `Model` / `Group` / `Provider` / `Max`），与 Go 结构体字段导出约定一致，便于阅读。

## 约束

- 带动态数据的消息必须在 locale 用 `{{.Xxx}}`，禁用 `%s` / `%d` 之类 printf 风格。
- 禁把已翻译串再走 `fmt.Sprintf` 二次填充；动态数据只能通过 args map 传入。
- 所有 locale 文件里同一 key 的占位符集合必须完全相同（三份对齐）。
- 占位符 key 大写驼峰、与业务语义一致；禁 `p1` / `p2` 之类无语义命名。

## 例子

- 输入：用户 token 无权访问模型 `gpt-4o`。
- 过程：locale 定义 `distributor.token_model_forbidden: "该令牌无权访问模型 {{.Model}}"` 与英文对应；业务代码 `i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelRequest.Model})`。
- 预期结果：中文得到「该令牌无权访问模型 gpt-4o」；英文得到 "Token has no access to model gpt-4o"；语序自然。

## 验收

- [ ] locale 文件里 grep `%s` / `%d` 用于翻译串的命中数为 0。
- [ ] 业务代码 grep `fmt.Sprintf\(.*i18n\.T\(` 命中数为 0。
- [ ] 三份 locale 里同一 key 的 `{{.Xxx}}` 占位符集合完全相同（可写脚本对比 diff）。
- [ ] 占位符命名均首字母大写领域词，无 `p1` / `p2` 之类无语义名。
