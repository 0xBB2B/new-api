---
name: 05-session-store
description: 会话记录的读写：本轮选定的模型、最近一次非顾问档位、上一次渠道，Redis 或内存二选一，3600 秒过期
---
# 会话记录

## 目标
按会话键读取和更新一条会话记录，供选档（降档保护、工具结果续跑）和清理思考内容（判断渠道是否变了）使用。

## 业务规则（来源：PRD 03-session-tier-switch C2、C3，04-advisor C4）
- 每个会话记三样东西：
  - 本轮选定的模型：档位名，或 `advisor`；
  - 最近一次不是顾问的档位；
  - 上一次请求实际用的渠道 ID。
- 记录在最后一次写入后 3600 秒过期，过期后当作新会话。
- 只在主请求成功（HTTP 状态 < 400）后写入。同一个会话的多个请求同时进行时，最后完成的那个覆盖前面的。
- 配了 Redis 就存 Redis，没配就存内存；内存记录在 new-api 重启后全部作废。
- 顾问那一轮：更新"本轮选定的模型"为 advisor、更新渠道，但"最近一次不是顾问的档位"保持不变。
- 工具结果续跑：只更新渠道（以及过期时间），不改另外两项。

## 涉及文件
- service/smartroute/session.go（新建）

## 成品定义
存储的值（JSON，用 `cachex.JSONCodec`）：

```go
type SessionState struct {
	CurrentRoute     string `json:"current_route"`       // simple / normal / hard / advisor
	LastNonAdvisor   string `json:"last_non_advisor"`    // simple / normal / hard
	LastChannelID    int    `json:"last_channel_id"`
}
```

缓存配置照 `service/channel_affinity.go:86-114` 的写法：命名空间 `new-api:smart_routing_session:v1`，Redis 判断条件与渠道亲和相同，内存用 hot LRU，容量 100000，默认 TTL 3600 秒。

## 函数清单
### service/smartroute/session.go
| 函数名 | 职责 |
|---|---|
| `getSessionCache` | 懒加载 `cachex.HybridCache[SessionState]`（照渠道亲和的懒加载写法） |
| `LoadSession` | 按会话键读取记录；会话键为空或没有记录时返回"不存在" |
| `SaveSessionAfterSuccess` | 按本次结果更新记录：用户新消息时写三项（顾问轮不改最近非顾问档位）；工具结果续跑只写渠道；TTL 都重置为 3600 秒 |

## 协作关系
- 07 在选档前调用 `LoadSession`，在 `c.Next()` 之后、状态码 < 400 时调用 `SaveSessionAfterSuccess`（位置照 `middleware/distributor.go:137` 的 `service.RecordChannelAffinity`）。
- 08 读取会话里的 `LastChannelID`，判断渠道有没有变。为了让 07 和 08 用同一份读取结果，07 把读到的记录放进 gin context（键名由 07 定义）。

## 验证方式
- 测试入口：`smartroute.LoadSession`、`smartroute.SaveSessionAfterSuccess`（写在 service/smartroute/smartroute_test.go；`common.RedisEnabled=false`，走内存）。
- 测试输入与预期：
- [ ] 空会话键 → 不存在；写入后再读 → 三项和写入的一致
- [ ] 顾问轮写入：CurrentRoute=advisor，LastNonAdvisor 保持原值
- [ ] 工具结果续跑写入：只有 LastChannelID 变
- [ ] TTL 改成 1 秒（测试内部的入口）写入，过期后读 → 不存在
