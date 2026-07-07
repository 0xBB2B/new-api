# Spec 索引

> 每条一行。读者先扫此页判断相关性，再打开具体文件。

## backend-i18n

- [edge-translation](backend-i18n/edge-translation.md) — 翻译在最外层执行；深层无 context 只回传 (key, args)，由上层拿到 c 后翻译。
- [key-registry](backend-i18n/key-registry.md) — 后端 i18n 消息 key 用类型化常量集中在 i18n/keys.go 定义；业务代码只引用常量，禁传字面串。
- [templated-messages](backend-i18n/templated-messages.md) — 带动态数据的翻译消息用 {{.Placeholder}} 模板 + map[string]any 注入；禁 fmt.Sprintf 或拼接。

## backend-test-quality

- [deterministic-exact-expected-values](backend-test-quality/deterministic-exact-expected-values.md) — 断言必须给出确定的具体期望值；禁「不为空/长度 > 0/包含任意错误」等模糊断言。
- [explicit-fixture-setup-per-test](backend-test-quality/explicit-fixture-setup-per-test.md) — 依赖 DB/context/settings/cache 的测试必须在用例内部显式初始化，禁靠前一个用例残留。
- [no-fake-behavior-tests](backend-test-quality/no-fake-behavior-tests.md) — Test* 函数禁随机输入、sleep 计时、大循环、仅日志断言等无契约保护的伪测试手段。
- [table-driven-with-named-subtests](backend-test-quality/table-driven-with-named-subtests.md) — 多输入组合的测试用「struct 表 + t.Run(name)」组织，子测试有可读名并可独立跑。
- [test-helpers-mark-t-helper](backend-test-quality/test-helpers-mark-t-helper.md) — 测试脚手架函数（签名含 *testing.T）首行必须 t.Helper()；命名带业务语义。
- [testify-required-for-assertions](backend-test-quality/testify-required-for-assertions.md) — 后端测试断言统一走 testify；require 用于致命前置，assert 用于值检查；禁手写 t.Fatalf。

## brand-protection

- [preserve-project-identity](brand-protection/preserve-project-identity.md) — 项目名与组织身份的引用/元数据受保护，禁止修改、删除、替换、匿名化。

## claude-subscription

- [channel-identity](claude-subscription/channel-identity.md) — Claude 订阅是独立渠道类型，默认 base URL api.anthropic.com，由专属 OAuth adaptor 分派，与标准 claude 渠道并存。
- [claude-code-system-prompt](claude-subscription/claude-code-system-prompt.md) — 强制注入 "You are Claude Code..." 作为第一条 system；自定义 SystemPrompt 沿用渠道全局语义，不由本渠道特殊处理。
- [credential-auto-refresh](claude-subscription/credential-auto-refresh.md) — refresh_token 换新并轮换回写 Key；后台仅 master、10min tick、剩余 <24h 才刷新、跳过 multi-key；另有手动刷新入口。
- [frontend-config-ui](claude-subscription/frontend-config-ui.md) — 前端类型下拉含该类型；Key 提示粘贴 Claude Code OAuth JSON；编辑抽屉提供刷新凭据按钮与合规免责声明；隐藏 batch/多 key 入口。
- [oauth-credential-format](claude-subscription/oauth-credential-format.md) — 渠道 Key 存 Claude Code 原生 OAuth JSON；解析 accessToken/refreshToken/expiresAt，accessToken 必填，expiresAt 为毫秒时间戳。
- [oauth-request-headers](claude-subscription/oauth-request-headers.md) — 上游鉴权头用 Authorization Bearer + anthropic-beta oauth-2025-04-20 + anthropic-version；禁带 x-api-key。
- [request-billing-passthrough](claude-subscription/request-billing-passthrough.md) — 上游 /v1/messages，复用标准 Claude 请求转换与 token 计费；无固定订阅倍率；不实现 usage 用量查询。
- [single-key-only](claude-subscription/single-key-only.md) — Claude 订阅渠道为单账号单渠道，禁止 batch 创建与多 key；前端拒绝、后台刷新跳过 multi-key。

## db-compat

- [boolean-default-not-declared-in-orm-tag](db-compat/boolean-default-not-declared-in-orm-tag.md) — bool 字段禁用 ORM default tag 声明默认值；改由代码层（构造/归一化/hook）赋值。
- [dialect-dispatch-via-capability-predicate](db-compat/dialect-dispatch-via-capability-predicate.md) — 跨方言分支必须走集中判定函数；禁止在业务或迁移代码里直接与方言常量做等值比较。
- [reserved-word-column-quoted-via-shared-helper](db-compat/reserved-word-column-quoted-via-shared-helper.md) — SQL 保留字作列名时必须走统一的方言感知引号常量；禁在拼接处硬编码引号。
- [schema-evolution-orm-first](db-compat/schema-evolution-orm-first.md) — 表结构演进优先由 ORM AutoMigrate 承担；跨方言列类型改动才写手工分支且保证幂等。
- [sqlite-schema-change-add-column-only](db-compat/sqlite-schema-change-add-column-only.md) — SQLite 分支的表结构演进只允许 ADD COLUMN 或整表重建，禁 ALTER COLUMN。

## error-handling

- [admin-response-envelope](error-handling/admin-response-envelope.md) — 管理端 HTTP 响应统一 {success, message, data?} 三字段信封；错误默认 HTTP 200 由 success:false 承载。
- [error-code-catalog-central](error-handling/error-code-catalog-central.md) — ErrorCode/ErrorType 枚举集中在 types 包；业务包只引用常量，禁自造字面串。
- [relay-error-wrap-newapierror](error-handling/relay-error-wrap-newapierror.md) — relay 及其上下游返回的错误统一包装成 *types.NewAPIError，携带 ErrorCode、StatusCode 与重试/日志选项。

## frontend-i18n

- [locale-json-flat-under-translation-namespace](frontend-i18n/locale-json-flat-under-translation-namespace.md) — locale 文件顶层为单一 namespace，其内为扁平 key → string；禁多级嵌套。
- [locales-parity-symmetric-key-set](frontend-i18n/locales-parity-symmetric-key-set.md) — 所有支持语言的 locale key 集合完全对称；由同步脚本作提交前门槛。
- [translation-access-hook-first-runtime-fallback](frontend-i18n/translation-access-hook-first-runtime-fallback.md) — 组件用 useTranslation() 的 t；非 React 上下文才用 i18n 单例的 t；禁 <Trans>。
- [translation-key-is-english-source-string](frontend-i18n/translation-key-is-english-source-string.md) — 翻译 key 直接用完整英文源字符串；禁 dot-namespace id（如 `page.login.title`）。

## json-wrapper

- [json-codec-single-entry](json-wrapper/json-codec-single-entry.md) — 项目提供 common/json.go 作为 JSON 编解码包装入口；pkg 内 codec 层同属合法；业务代码两种写法并存为现状。
- [json-raw-message-shape-dispatch](json-wrapper/json-raw-message-shape-dispatch.md) — 可变形态 JSON 字段用「首字节判形」分派，禁用试错式反序列化。

## quota-reset

- [manual-trigger](quota-reset/manual-trigger.md) — 管理端「立即按规则重置」入口：仅管理员可用，对全体参与用户执行与定时触发完全相同的重置。
- [reset-semantics](quota-reset/reset-semantics.md) — 单次额度重置动作的语义：quota 覆盖写为重置值（≥0 整数），统计字段不动，每用户记一条系统日志。
- [rule-resolution](quota-reset/rule-resolution.md) — 额度重置规则的生效解析：全局默认 + 每用户专属完全覆盖；退出标记最优先；仅启用状态用户参与。
- [schedule-trigger](quota-reset/schedule-trigger.md) — 额度重置的定时触发：每天/每周/每月三种周期的触发时点按服务器本地时区计算；错过不追赶。
- [user-visibility](quota-reset/user-visibility.md) — 有生效重置规则的用户在个人页可见下次重置时间与重置值；无生效规则的用户不展示。

## relay-adapter-pattern

- [adaptor-interface-total-method-set](relay-adapter-pattern/adaptor-interface-total-method-set.md) — 每个 provider adapter 实现同一套完整接口方法集；不支持的模态返回 not implemented，不裁剪接口。
- [shared-http-transport-helper](relay-adapter-pattern/shared-http-transport-helper.md) — adapter 的 HTTP 执行阶段委托到共享传输 helper；禁自行手写请求构造与发送。
- [stream-options-capability-whitelist](relay-adapter-pattern/stream-options-capability-whitelist.md) — 是否支持 OpenAI stream_options 由 relay-info 集中白名单决定；adapter 只读能力标志不自行决策。

## relay-request-shape

- [optional-scalar-nullable-forwarding](relay-request-shape/optional-scalar-nullable-forwarding.md) — 顶层主请求 DTO 的可选数值/布尔字段用指针 + omitempty，保留显式零值；嵌套结构不受此约束。
- [unparsed-passthrough-raw-message](relay-request-shape/unparsed-passthrough-raw-message.md) — 网关不理解的 provider 扩展字段用原始 JSON 字节容器承载；逐字节透传到上游。
