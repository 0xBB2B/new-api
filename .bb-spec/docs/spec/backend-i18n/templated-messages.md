---
name: templated-messages
description: 带动态数据的提示用占位符 + 参数 map 传入，禁 fmt.Sprintf 或拼接；后端 yaml 用 {{.Xxx}}，网页控制台提示用 {{xxx}}。
---

# 带参数的提示

## 目的

让占位符的位置可以随语言的语序变化；保证同一条提示在各语言里的参数一致；避免把参数先拼进句子导致无法翻译。

## 逻辑

项目里有两种要翻译的提示，占位符写法不同。

**后端自己翻译的文字**（`i18n/locales/*.yaml`，给 AI 客户端的错误、邮件、通知）：用 go-i18n 模板语法 `{{.Xxx}}`，参数名首字母大写（例如 `"Insufficient user quota, remaining quota: {{.Remaining}}"`）。调用点写 `i18n.T(c, key, map[string]any{"Remaining": value})`，后台任务写 `i18n.Translate(lang, key, map[string]any{...})`。

**网页控制台的提示**（英文原文作 key，前端翻译）：占位符写 `{{xxx}}`，参数名用小写加下划线（例如 `{{quota}}`、`{{group_ratio}}`）。调用点写 `common.NewMessage("Daily check-in, received {{quota}}", map[string]any{"quota": logger.FormatQuota(q)})` 或 `common.ApiErrorT(c, key, params)`。

后端用 `Message.Error()` 把参数填进英文原文，给不经过网页的调用方看；响应里同时带 `message_key` 和 `message_params`，前端用 i18next 按当前语言翻译并填参数。参数要传可以直接显示的值，额度先用 `logger.FormatQuota` 格式化。

两种写法都禁止先翻译、再用 `fmt.Sprintf` 或字符串拼接塞参数，那样会把参数固定在某一种语序上。

## 约束

- 带动态数据的提示必须用占位符，禁用 `%s` / `%d` 这类 printf 写法。
- 禁把翻译结果再用 `fmt.Sprintf` 填参数；参数只能通过参数 map 传入。
- 后端 yaml：同一个 key 在 en / zh-CN / zh-TW 三份文件里的 `{{.Xxx}}` 集合必须相同，参数名首字母大写。
- 网页控制台提示：同一个 key 在前端 7 个语言文件里的译文必须保留全部 `{{xxx}}` 占位符，参数名小写加下划线。
- 参数名要表达业务含义，禁用 `p1` / `p2` 这类名字。

## 例子

- 输入一：令牌无权访问模型 `gpt-4o`，错误返回给 AI 客户端。
- 过程：yaml 定义 `distributor.token_model_forbidden: "该令牌无权访问模型 {{.Model}}"` 及英文对应；调用点 `i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelName})`。
- 预期结果：中文客户端收到「该令牌无权访问模型 gpt-4o」，英文客户端收到对应英文。
- 输入二：Claude 订阅渠道限额重置时，上游返回 HTTP 500。
- 过程：后端 `common.NewMessage("Upstream returned HTTP {{status}}", map[string]any{"status": 500})`；前端 zh.json 里有 `"Upstream returned HTTP {{status}}": "上游返回 HTTP {{status}}"`。
- 预期结果：响应 `message` 是 `Upstream returned HTTP 500`，`message_key` 是原文，`message_params` 是 `{"status":500}`；中文界面显示「上游返回 HTTP 500」。

## 验收

- [ ] 语言文件里没有用 `%s` / `%d` 作占位符的译文。
- [ ] 业务代码 grep `fmt.Sprintf\(.*i18n\.T\(` 命中数为 0。
- [ ] `go test ./i18n/ -run TestLocalesRenderEveryKey` 通过（yaml 渲染后不残留 `{{`）。
- [ ] 前端 7 个语言文件里，带 `{{xxx}}` 的 key 的译文都保留了同样的占位符。
