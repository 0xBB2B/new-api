---
name: 03-credential-refresh
description: Claude OAuth 凭据刷新——service 三件套 + 后台任务注册 + 手动刷新 handler/路由 + 创建时单 key 与凭据校验。
---
# 凭据刷新与单 key 约束（后端）

## 目标
type 59 渠道的 accessToken 在过期前经 refresh_token 自动续期并回写渠道 Key;管理员可手动触发刷新;该类型禁 batch/multi-key。

## 业务规则（来源：spec claude-subscription/credential-auto-refresh, single-key-only）
- 刷新:`POST https://console.anthropic.com/v1/oauth/token`,JSON body `{"grant_type":"refresh_token","refresh_token":<当前>,"client_id":"9d1c250a-e61b-44d9-88ed-5944d1962f5e"}`;响应含新 access_token/refresh_token/expires_in(秒)。
- 回写:新 token 覆盖旧值(轮换 refresh token),`expiresAt = now + expires_in`(毫秒)写回 `claudeAiOauth`,GORM 更新 Key 后重建渠道缓存。
- 后台任务:仅 master、每 10 分钟 tick、只刷新剩余有效期 <24h(或 expiresAt 解析失败)的渠道、跳过 multi-key;单渠道失败不影响其它。无 refreshToken 则跳过。
- 单 key:该类型禁 batch 创建与多 key;创建/更新时校验凭据为合法 claudeAiOauth。

## 涉及文件
- `service/claude_oauth.go` — 新建
- `service/claude_credential_refresh.go` — 新建
- `service/claude_credential_refresh_task.go` — 新建
- `main.go` — 修改（注册后台任务）
- `controller/channel.go` — 修改（手动刷新 handler + 创建校验）
- `router/channel-router.go` — 修改（路由）

## 成品定义

### `service/claude_oauth.go` 常量
```go
const (
	claudeOAuthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	claudeOAuthTokenURL = "https://console.anthropic.com/v1/oauth/token"
)
```
说明:端点随 Anthropic 迁移可能变为 `https://platform.claude.com/v1/oauth/token`;exec 落盘前经官方渠道确认当时可用端点。

### `router/channel-router.go`（channel 路由表内新增一行）
```go
selfPart.POST("/:id/claude/refresh", middleware.AdminAuth(), controller.RefreshClaudeChannelCredential)
```
权限与 codex refresh 一致(`authz.ChannelSensitiveWrite`),按现有 router 写法对齐。

## 函数清单

### `service/claude_oauth.go`
| 函数名 | 职责 |
|---|---|
| `ClaudeOAuthKey`(struct) | service 层凭据结构(含 claudeAiOauth 字段),避免 import cycle |
| `parseClaudeOAuthKey` | 解析渠道 Key 为 ClaudeOAuthKey |
| `RefreshClaudeOAuthToken` | 以 refresh_token grant 调 token 端点,返回新 access/refresh + 过期时刻 |
| `RefreshClaudeOAuthTokenWithProxy` | 带渠道代理的刷新入口 |

### `service/claude_credential_refresh.go`
| 函数名 | 职责 |
|---|---|
| `ClaudeCredentialRefreshOptions`(struct) | 刷新选项(ResetCaches) |
| `RefreshClaudeChannelCredential` | 读渠道→刷新→回写 accessToken/refreshToken/expiresAt(毫秒)到 Key→按需重建缓存 |

### `service/claude_credential_refresh_task.go`
| 函数名 | 职责 |
|---|---|
| `StartClaudeCredentialAutoRefreshTask` | sync.Once + 仅 master;起 10min ticker,先跑一次再进循环 |
| `runClaudeCredentialAutoRefreshOnce` | atomic 防重入;分页扫 type 59 启用/自动禁用渠道;跳过 multi-key;对剩余 <24h 的调 RefreshClaudeChannelCredential |
| `shouldAutoRefreshClaudeChannelStatus` | 判定渠道状态是否纳入刷新 |

### `controller/channel.go`
| 函数名 | 职责 |
|---|---|
| `RefreshClaudeChannelCredential` | 手动刷新 handler,调 service(ResetCaches: true),返回管理端信封 |
| (创建/更新校验分支) | type 59 时校验 Key 为合法 claudeAiOauth 且禁 multi-key/batch |

### `main.go`
| 位置 | 职责 |
|---|---|
| 启动任务注册块(codex 注册之后) | 调 `service.StartClaudeCredentialAutoRefreshTask()` |

## 协作关系
- `RefreshClaudeChannelCredential`(service) → `RefreshClaudeOAuthTokenWithProxy` → 回写 `model.Channel.Key`(GORM Update)→ `model.InitChannelCache()`。
- 后台任务 → `RefreshClaudeChannelCredential`(ResetCaches:false),本轮末统一重建缓存。
- handler 错误走管理端信封(error-handling/admin-response-envelope);DB 写走 GORM(db-compat)。
- 外部依赖:Anthropic OAuth token 端点。

## 验证方式
- [ ] 手动刷新:POST /:id/claude/refresh → Key 内 accessToken/refreshToken 更新、expiresAt=now+expires_in(毫秒),返回 success。
- [ ] 后台任务仅 master、10min、只处理剩余 <24h、跳过 multi-key(单测覆盖判定函数)。
- [ ] 无 refreshToken 渠道被跳过、不报致命错。
- [ ] type 59 以 batch/multi-key 创建被拒;凭据非法被拒。
- [ ] `go build ./...` 通过。
