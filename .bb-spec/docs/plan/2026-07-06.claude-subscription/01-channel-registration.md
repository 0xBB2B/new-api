---
name: 01-channel-registration
description: 注册 Claude 订阅渠道类型 59 与 APITypeClaudeSubscription，登记显示名/默认 base URL/adaptor 分派/流式白名单。
---
# 渠道类型注册

## 目标
系统识别一个新渠道类型「Claude Subscription」，未填 base URL 时上游默认 `https://api.anthropic.com`，其请求被分派到专属 OAuth adaptor 且支持流式；标准 claude(14)渠道不受影响。

## 业务规则（来源：spec claude-subscription/channel-identity）
- 存在独立渠道类型，显示名固定 `Claude Subscription`，值不同于标准 claude(14)与 codex(57)。
- 该类型默认 base URL `https://api.anthropic.com`。
- 该类型经专属 OAuth adaptor 分派(区别于标准 claude 的 x-api-key 路径),并登记为支持流式。

## 涉及文件
- `constant/channel.go` — 修改
- `constant/api_type.go` — 修改
- `common/api_type.go` — 修改
- `relay/relay_adaptor.go` — 修改
- `relay/common/relay_info.go` — 修改

## 成品定义

### `constant/channel.go`
在 `ChannelTypeDummy` 之前新增常量(使 Dummy 顺延为 60):
```go
ChannelTypeClaudeSubscription = 59 // Claude Max/Pro 订阅（OAuth 凭据）
```
`ChannelBaseURLs` 切片下标 59 处填入(原下标 59 为 Dummy 占位，需在正确位置插入使下标对齐):
```go
"https://api.anthropic.com", //59
```
`ChannelTypeNames` map 新增:
```go
ChannelTypeClaudeSubscription: "Claude Subscription",
```

### `constant/api_type.go`
在 `APITypeDummy` 之前新增:
```go
APITypeClaudeSubscription
```

### `common/api_type.go`（`ChannelType2APIType` switch，AdvancedCustom case 之前）
```go
case constant.ChannelTypeClaudeSubscription:
	apiType = constant.APITypeClaudeSubscription
```

### `relay/relay_adaptor.go`（`GetAdaptor` switch，需 import 新包）
```go
case constant.APITypeClaudeSubscription:
	return &claude_oauth.Adaptor{}
```

### `relay/common/relay_info.go`（`streamSupportedChannels` map，按 channel type）
```go
constant.ChannelTypeClaudeSubscription: true,
```

## 协作关系
- `ChannelType2APIType` 把渠道 type 59 映射到 `APITypeClaudeSubscription`;`GetAdaptor` 据该 APIType 返回 `claude_oauth.Adaptor`(该 adaptor 由 02 提供)。
- `streamSupportedChannels[59]` 决定 RelayInfo.SupportStreamOptions。
- 无外部依赖(DB/MQ/第三方)。

## 验证方式
- [ ] `go build ./...` 通过(引用 `claude_oauth.Adaptor` 需 02 落地后一并编译)。
- [ ] 新建 type 59 渠道、base URL 留空 → RelayInfo 上游前缀为 `https://api.anthropic.com`。
- [ ] `ChannelType2APIType(59) == APITypeClaudeSubscription`,`GetAdaptor` 返回 claude_oauth adaptor。
- [ ] type 14 标准 claude 渠道行为不变。
