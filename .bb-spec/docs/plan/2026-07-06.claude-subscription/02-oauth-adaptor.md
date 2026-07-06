---
name: 02-oauth-adaptor
description: 新建 claude_oauth adaptor 包——OAuth 凭据解析、Bearer 鉴权头、身份 system 注入，请求/响应/计费复用标准 claude 包。
---
# OAuth 转发 adaptor

## 目标
type 59 渠道用 Claude Code OAuth 凭据向 `api.anthropic.com/v1/messages` 转发请求，鉴权用 Bearer + anthropic-beta，强制注入 Claude Code 身份 system；请求转换与计费复用标准 claude 管道。

## 业务规则（来源：spec claude-subscription/oauth-credential-format, oauth-request-headers, claude-code-system-prompt, request-billing-passthrough）
- 渠道 Key 存 Claude Code 原生 JSON `{"claudeAiOauth":{accessToken,refreshToken,expiresAt(毫秒)}}`;accessToken 必填,缺失/非 JSON 即报错不发上游。
- 上游头:`Authorization: Bearer <accessToken>`、`anthropic-beta`(含 `oauth-2025-04-20`,客户端已带则去重合并)、`anthropic-version`(缺省 `2023-06-01`);禁 `x-api-key`。
- system 序列首条恒为 `You are Claude Code, Anthropic's official CLI for Claude.`;客户端有 system 则身份串前置;渠道自定义 SystemPrompt 时顺序为「身份串 → 自定义 → 客户端 system」;不重复叠加身份串。
- 上游 `/v1/messages`;请求转换与 token 计费(input/output/cache)复用标准 claude;无固定订阅倍率;不做 usage 查询。

## 涉及文件
- `relay/channel/claude_oauth/adaptor.go` — 新建
- `relay/channel/claude_oauth/constants.go` — 新建
- `relay/channel/claude_oauth/oauth_key.go` — 新建

## 函数清单

### `relay/channel/claude_oauth/oauth_key.go`
| 函数名 | 职责 |
|---|---|
| `OAuthKey`(struct) | 承载 `claudeAiOauth` 内层字段 accessToken/refreshToken/expiresAt/scopes/subscriptionType(JSON tag 对齐原生驼峰) |
| `claudeCredentialEnvelope`(struct) | 外层 `{"claudeAiOauth": OAuthKey}` 包裹结构 |
| `ParseOAuthKey` | 解析渠道 Key 字符串为 OAuthKey;非 JSON/缺 claudeAiOauth/accessToken 空则返回 error |

### `relay/channel/claude_oauth/constants.go`
| 符号 | 职责 |
|---|---|
| `ChannelName` | 常量 `"claude_subscription"` |
| `ModelList` | 复用标准 claude 模型列表(claude 系列) |

### `relay/channel/claude_oauth/adaptor.go`
| 函数名 | 职责 |
|---|---|
| `Adaptor`(struct) | 实现 channel.Adaptor 完整接口方法集 |
| `Init` | 接口空实现 |
| `GetRequestURL` | 返回 `{ChannelBaseUrl}/v1/messages` |
| `SetupRequestHeader` | 解析 OAuthKey→设 Authorization Bearer、anthropic-beta(合并 oauth-2025-04-20)、anthropic-version;不设 x-api-key |
| `ConvertClaudeRequest` | 直通请求后调用身份注入 helper 处理 `request.System`(any:string/数组) |
| `ConvertOpenAIRequest` | 调 `claude.RequestOpenAI2ClaudeMessage` 转换后调用身份注入 helper |
| `prependClaudeCodeSystem` | 把身份串置为 System 首块;按 SystemPrompt/SystemPromptOverride 前置自定义;已存在身份串不重复(复用 codex ConvertOpenAIResponsesRequest 的 override 语义) |
| `DoRequest` | 委托共享传输 helper 发送 |
| `DoResponse` | IsStream 走 `claude.ClaudeStreamHandler`,否则 `claude.ClaudeHandler` |
| `ConvertOpenAIResponsesRequest` / `ConvertGeminiRequest` / embedding / rerank / audio / image 等 | 返回 not implemented(不裁剪接口) |
| `GetModelList` / `GetChannelName` | 返回 ModelList / ChannelName |

## 协作关系
- `SetupRequestHeader` → `ParseOAuthKey`(读 info.ApiKey)。
- `ConvertOpenAIRequest` → `claude.RequestOpenAI2ClaudeMessage`(跨包,已导出)→ `prependClaudeCodeSystem`。
- `prependClaudeCodeSystem` 读 `info.ChannelSetting.SystemPrompt` / `SystemPromptOverride`(`dto/channel_settings.go`)。
- `DoResponse` → `claude.ClaudeHandler` / `claude.ClaudeStreamHandler`(跨包,已导出),返回 `*dto.Usage` 走通用计费。
- 鉴权/解析错误包装为 `*types.NewAPIError`(遵守 error-handling/relay-error-wrap-newapierror)。
- 外部依赖:上游 `api.anthropic.com/v1/messages`。

## 验证方式
- [ ] 合法 `claudeAiOauth` JSON → 上游头含 `Authorization: Bearer ...`、`anthropic-beta` 含 `oauth-2025-04-20`,无 `x-api-key`。
- [ ] 非 JSON Key → SetupRequestHeader 返回 error,不发上游。
- [ ] 客户端无 system → 上游 system 首条为身份串;有 system → 身份串前置;配 SystemPrompt+Override → 顺序「身份串→自定义→客户端」。
- [ ] 非流式对话按上游 usage(input/output/cache)计费;流式经 ClaudeStreamHandler 正常解析。
- [ ] `go vet ./relay/channel/claude_oauth/...` 通过,接口方法集完整。
