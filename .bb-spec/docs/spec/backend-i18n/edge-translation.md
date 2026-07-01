---
name: edge-translation
description: 翻译在最外层执行；深层无 context 只回传 (key, args)，由上层拿到 c 后翻译。
---

# 翻译在边界层执行

## 目的

避免翻译串沿调用链穿透后无法回溯语言；避免不同 controller 出现 zh/en 语料混杂；把「用户语言」这个跨切面耦合收敛到边界一层。

## 逻辑

**边界层**（middleware、controller、可直接拿到 `*gin.Context` 的地方）：用 `i18n.T(c, key, map[string]any{...})` 或 `common.TranslateMessage(c, key)` 当场把 key 翻译成用户语言字符串再写响应。`TranslateMessage` 在 `i18n.Init` 时把 `T` 注入 `common` 包，用来避免 `model → i18n` 循环依赖。

**深层无 gin.Context 的库/服务**（如 oauth 各 provider、model 层）：不做翻译，而是把 `key + args map` 打包进领域错误结构透传（例如 `NewOAuthError(i18n.MsgOAuthInvalidCode, nil)`、`NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": name}, raw)`），由上游 controller 拿到 `c` 后统一翻译。

用户语言由 `i18n.GetLangFromContext(c)` 按此优先级解析：user setting → 懒加载用户设置 → I18n middleware 注入的 context key → `Accept-Language` header → 默认英语。

业务代码绝不用 `fmt.Sprintf("错误")` 或裸中文/英文串构造对外错误消息。

## 约束

- 深层库函数（无 `gin.Context` 入参）禁调用 `i18n.T` / `i18n.Translate`；只能返回带 key 的领域错误。
- 边界层禁把已翻译字符串再次塞进领域错误往下传，避免二次翻译丢语言上下文。
- 业务代码返回给客户端的 `message` 字段禁用裸字面串（zh 或 en），必须经 i18n 入口。
- 缺 context 场景（例如后台任务）应显式走 `i18n.Translate(lang, key)` 并说明 `lang` 来源。

## 例子

- 输入：`oauth/discord.go` 交换 token 时 HTTP 请求出错。
- 过程：discord.go 深层写 `return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())`；`controller/oauth.go` 边界拿到 `c` 后 `i18n.T(c, oauthErr.Key, oauthErr.Args)` 翻译并写回。
- 预期结果：中文用户看到「Discord 连接失败」；英文用户看到 "Discord connection failed"；语言由 `c` 决定，与深层无关。

## 验收

- [ ] 深层库函数（无 `*gin.Context` 入参）grep `i18n\.T\(\|i18n\.Translate\(` 命中数为 0。
- [ ] controller 返回给客户端的 `message` grep 出的裸中文/英文字面串为 0。
- [ ] 后台任务翻译均显式指定 `lang` 来源。
- [ ] OAuth provider 错误路径均走 `NewOAuthError` / `NewOAuthErrorWithRaw` 打包透传。
