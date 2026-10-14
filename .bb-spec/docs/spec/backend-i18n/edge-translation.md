---
name: edge-translation
description: 翻译在离读者最近的一层做：网页控制台提示后端只发英文原文和 message_key、由前端翻译；后端只翻译给 AI 客户端的错误、邮件和通知。
---

# 翻译在离读者最近的一层做

## 目的

让同一条提示只在一处被翻译，语言由真正的读者决定；避免后端把翻译好的文字层层往下传而丢了语言信息，也避免同一个页面中英文混杂。

## 逻辑

**网页控制台的提示**（管理接口、日志、任务失败原因）：后端不翻译，前端翻译。

- 深层代码（service、model、oauth 等）返回 `*common.Message`。它本身实现了 `error`，可以直接当错误往上返回；OAuth 错误把它放在 `OAuthError.Message` 里（`oauth/types.go`）。
- controller 用 `common.ApiError(c, err)`、`common.ApiErrorT(c, key, params)` 或 `common.ApiSuccessT(c, key, data, params)` 写响应。响应里 `message` 是英文渲染结果，`message_key` 是英文原文，有参数时带 `message_params`。不管请求是什么语言，后端发的内容都一样。
- 前端用 `getServerMessage`（`web/src/lib/server-error-message.ts`）优先读 `message_key` 和 `message_params`，交给 i18next 翻译。所以每个 key 都要加进前端 7 个语言文件。
- 日志内容以 `[]*common.Message` 的形式写进 `other.content_parts`，前端按当前语言渲染；`content` 列存英文渲染结果。网关写入的任务失败原因是固定的英文句子，它同时也是前端语言文件的 key，前端用 `translateServerText` 翻译。

**后端自己翻译的文字**（中间没有网页控制台）：返回给 AI 客户端的错误、邮件、通知。

- 拿得到 `*gin.Context` 的地方用 `i18n.T(c, key, args)`。读者的语言由 `i18n.StatedLang(c)` 决定，优先级依次是：已加载的用户设置 → 按用户 ID 懒加载的用户语言 → I18n 中间件从 `Accept-Language` 解析出的语言 → `Accept-Language` 请求头。都没有时用环境变量 `DEFAULT_LANGUAGE`，没设置就是英文。
- 没有请求的后台任务用 `i18n.Translate(lang, key, args)`，`lang` 取收件人保存的语言，例如渠道通知用 `RootUserLanguage()`。

**服务端日志**：写英文，经 `common.LogText(format, args...)` 输出。`DEFAULT_LANGUAGE=zh-CN` 时，在 `common/log_text.zh-CN.json` 里有中文的那些行会打印中文。

## 约束

- 网页控制台的提示禁止在后端翻译：禁用 `i18n.T` 渲染后再发给前端，禁在 `message` 里写中文。
- 网页控制台的提示禁止用 `c.JSON` 手写 `message`；必须经 `common.NewMessage` / `ApiErrorT` / `ApiSuccessT` 带上 `message_key`。
- 新增网页控制台提示时，必须把英文原文加进前端 7 个语言文件。
- 深层代码禁调用 `i18n.T` / `i18n.Translate` 翻译网页控制台的提示，只能返回 `*common.Message`。
- 后台任务翻译时必须明确 `lang` 从哪里来。
- 服务端日志行禁写中文字面串，统一写英文并经 `common.LogText`。

## 例子

- 输入一：管理员用 GitHub 登录时，连不上 GitHub。
- 过程：`oauth/github.go` 返回 `NewOAuthErrorWithRaw(common.NewMessage("Unable to connect to {{provider}} server, please try again later", map[string]any{"provider": "GitHub"}), err.Error())`；controller 用 `ApiError` 返回，响应里带 `message_key`。
- 预期结果：中文界面显示中文，英文界面显示英文；后端对两种请求返回的内容相同。
- 输入二：用户用 API 令牌调用 `/v1/chat/completions`，令牌无效。
- 过程：中间件调用 `common.TranslateMessage(c, i18n.MsgTokenInvalid)`，它按 `StatedLang(c)` 翻译。
- 预期结果：用户保存了中文就收到中文；没有任何语言说明时收到 `DEFAULT_LANGUAGE` 的语言。

## 验收

- [ ] 管理接口 controller 里 grep 不到 `"message": "<中文>"` 这类手写中文提示。
- [ ] 深层代码（无 `*gin.Context` 入参）grep `i18n\.T\(` 命中数为 0。
- [ ] 后台任务调用 `i18n.Translate` 时都明确了 `lang` 的来源。
- [ ] `go test ./i18n/` 通过：`TestWebConsoleMessageResponse` 锁住响应带 `message_key` / `message_params`，`TestBackendMessageLanguage` 锁住语言优先级。
