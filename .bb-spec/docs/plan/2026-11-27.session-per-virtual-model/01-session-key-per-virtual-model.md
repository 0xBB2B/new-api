---
name: 01-session-key-per-virtual-model
description: 会话标识加入虚拟模型名，换虚拟模型按新会话处理，各虚拟模型的会话记录互不影响
---
# 会话标识按虚拟模型区分

## 目标
网关认会话时加上虚拟模型（路由配置里的名字），同一段对话换虚拟模型后按新会话处理，切回原模型时沿用它自己的记录。

## 业务规则（来源：PRD 01-session-per-virtual-model）
- 认会话：
  - 带 prompt_cache_key → 「密钥 + 虚拟模型 + prompt_cache_key」；
  - 不带 → 「密钥 + 虚拟模型 + 第一条用户消息的内容」；
  - 没有密钥的演练场请求，把密钥换成用户；
  - 第一条用户消息没有文字、也没有 prompt_cache_key → 不认会话（和现在一样）。
  - 虚拟模型取路由配置里的名字，不取客户端请求里写的原样。
- 同一个密钥、同一个虚拟模型、对话标识相同 → 同一个会话；虚拟模型不同 → 不同会话。
- 换到另一个虚拟模型、它没有会话记录时，按新会话处理：
  - 用户新消息直接用打分选出的档位，不做降档保护；
  - 工具结果续跑用新虚拟模型的默认档，力度按默认档自己的设置算，不沿用前一个虚拟模型的力度。
- 每个虚拟模型的记录各自更新、各自作废（3600 秒）。在 3600 秒内切回 A，续跑沿用 A 记录里的档位和力度，用户新消息按 A 记录里最近一次不是顾问的档位做降档保护。
- 只用一个虚拟模型的对话，行为和改动前完全一样。
- 升级后，旧格式算出的会话记录全部对不上，每段对话的下一条请求按新会话处理；不做迁移（用户 2026-11-27 接受）。

## 涉及文件
- 修改 `service/smartroute/inspect.go`
- 修改 `middleware/smart_route.go`
- 修改 `service/smartroute/smartroute_test.go`（扩展 `TestSessionKey`）
- 修改 `middleware/distributor_test.go`（已有调用 `SessionKey` 的用例跟着改；在已有智能路由分发用例里加换虚拟模型的场景）

## 成品定义
会话记录在缓存里的键是下面这段文字的 SHA-256 十六进制串（缓存前缀、记录内容、3600 秒有效期都不变）：

```text
# 带 prompt_cache_key
<owner>|vm|<虚拟模型在路由配置里的名字>|pck|<prompt_cache_key>
# 不带 prompt_cache_key，用第一条用户消息
<owner>|vm|<虚拟模型在路由配置里的名字>|first|<第一条用户消息的文字>
# <owner>：有令牌时是令牌 ID 的十进制，如 7；演练场是 "u" + 用户 ID，如 u12
# 例：令牌 7、虚拟模型 auto-gpt、prompt_cache_key k1 → sha256("7|vm|auto-gpt|pck|k1")
```

## 函数清单
### service/smartroute/inspect.go
| 函数名 | 职责 |
|---|---|
| `SessionKey`（修改） | 多收虚拟模型名，按成品定义算会话键 |
| `SessionKeyForUser`（修改） | 同上，owner 用 "u" + 用户 ID |
| `sessionKey`（修改） | 按成品定义拼种子文字并取 SHA-256；没有对话标识时返回空串 |

### middleware/smart_route.go
| 函数名 | 职责 |
|---|---|
| 智能路由分发函数里算会话键的分支（修改，约 93–98 行） | 调用时传入 `route.Name` |

## 协作关系
分发函数 → `FindSmartRoute` 取到 `route` → `SessionKey`/`SessionKeyForUser`（带 `route.Name`）→ `LoadSession` → `DecideRoute`（续跑沿用档位、降档保护，不改）→ `ResolveTierEffort`（续跑沿用力度，不改）→ 请求成功后 `SaveSessionAfterSuccess`（不改，用 RouteResult 里同一个会话键）。

## 验证方式
- 测试入口：`smartroute.SessionKey`、`smartroute.SessionKeyForUser`（包级）；`middleware/distributor_test.go` 已有的智能路由分发夹具（构造两个虚拟模型的路由 JSON、发请求、读上游请求体里的模型与力度、读日志里的智能路由结果）。
- 测试输入：令牌 7；虚拟模型 A、B；prompt_cache_key k1，或不带 prompt_cache_key、第一条用户消息相同；A 的会话记录为困难档、力度 high；B 的默认档普通档、普通档「固定 low」、降档时历史 token 上限 20,000；输入约 86,000 token 的长对话。
- 预期结果：
- [ ] `SessionKey(7, A, 带 k1)` 等于 sha256("7|vm|A|pck|k1")；不带 prompt_cache_key 时等于 sha256("7|vm|A|first|<第一条消息>")；同参数两次相同；A、B 两次不同；不带 prompt_cache_key、第一条消息相同时 A、B 不同；演练场 `SessionKeyForUser` 对 A、B 不同；没有对话标识时为空串。
- [ ] A 记录困难档；同一段对话请求 B 的用户新消息、86,000 token、打分简单档 → 用简单档，没有降档被拦。
- [ ] A 记录困难档、力度 high；请求 B 的工具结果续跑 → B 的普通档模型，上游力度 low。
- [ ] 在 B 上请求一轮后，再请求 A 的工具结果续跑 → A 的困难档模型，上游力度 high；A 的用户新消息 86,000 token、打分简单档 → 保留困难档，降档被拦。
- [ ] 只用 A：上一轮困难档、86,000 token、打分简单档 → 降档被拦；之后的续跑沿用困难档和本轮力度（已有用例改成新调用方式后仍通过）。
- [ ] `go test ./middleware/ ./service/smartroute/ -count=1` 通过；`gofmt -l` 无输出。
