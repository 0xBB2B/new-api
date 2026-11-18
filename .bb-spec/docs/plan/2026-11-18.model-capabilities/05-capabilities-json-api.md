---
name: 05-capabilities-json-api
description: 能力配置 JSON 的读取、预览、整体替换保存三个管理接口，保存在事务里并做版本检查
---
# 能力配置 JSON 接口

## 目标
管理员能一次读出全部完全匹配记录的能力，提交新 JSON 后先预览差异，再整体替换保存；预览后数据变了就拒绝保存。

## 业务规则（来源：PRD 02-capabilities-json-window）
- 读取：按模型名排序，列出名称规则为完全匹配、且至少配了一项能力的记录；另列出没列进来的记录：非完全匹配记录（名字 + 规则）、完全匹配但一项能力都没配的记录（名字）。
- 提交的 JSON 形如 `{"<模型名>": {<单个模型能力>}}`，单个模型能力的形状、取值、null 与空模态数组含义同 01。
- 整体替换：列进 JSON 的模型，能力等于 JSON 里写的（没写、写 null 的字段清空）；当前配过能力、却不在 JSON 里的完全匹配记录，能力全部清空、记录保留；非完全匹配记录一律不改。
- 模型名没有完全匹配记录时新建完全匹配记录（复制命中的非完全匹配记录资料，规则同 04 的 `NewExactModelRecord`）。
- 预览：按服务器当前数据对比，每个模型给出 新建 / 更新 / 清空 / 不变 和字段级改前改后；任一条目校验失败时给出逐条错误，且不能保存。
- 保存：要么全成功，要么全不改（一个事务）。保存时重新计算版本，与预览时不同（涉及的记录被改、删，或同名完全匹配记录被新建，或作为复制来源的命中记录变了）→ 返回冲突，什么都不改。
- 只有能管理模型资料的管理员能用，服务器端校验权限（沿用 `/api/models` 组的 `AdminAuth`），并在访问令牌作用域表登记。
- 保存后其他模块下一次请求就能读到新值。

## 涉及文件
- 修改 `router/api-router.go`（`/api/models` 组约 :421-433 下新增三条路由）
- 修改 `middleware/access_token_routes.go`（约 :285-291 登记三条路由，作用域同模型资料读写）
- 新建 `controller/model_capability.go`
- 新建 `model/model_capability_batch.go`
- 新建 `model/model_capability_batch_test.go`（模型层规则）；在 `controller/model_management_test.go` 追加接口用例

## 成品定义
路由（均在 `/api/models` 组内，组级 `AdminAuth`）：
```text
GET  /api/models/capabilities           -> controller.GetModelCapabilities
POST /api/models/capabilities/preview   -> controller.PreviewModelCapabilities
PUT  /api/models/capabilities           -> controller.ReplaceModelCapabilities
```
响应契约（外层沿用项目 `{success, message, data}`）：
```jsonc
// GET data
{
  "capabilities": { "gpt-6.2-sol": { "context_length": 1050000 } },   // 规范 JSON 对象，按名排序
  "non_exact": [ { "model_name": "gpt-6", "name_rule": 1 } ],          // name_rule 沿用 NameRule 数值
  "unconfigured": [ "glm-5.2" ]
}
// POST preview 请求体 / PUT 请求体
{ "capabilities": { "<模型名>": { } }, "version": "<仅 PUT 必填，取自 preview 响应>" }
// POST preview data
{
  "version": "<sha256 十六进制>",
  "items": [
    { "model_name": "gpt-6.3-sol", "action": "create",    "before": null, "after": { "context_length": 1050000 } },
    { "model_name": "gpt-6.2-sol", "action": "update",    "before": { }, "after": { } },
    { "model_name": "qwen4-max",   "action": "clear",     "before": { }, "after": { } },
    { "model_name": "gpt-4o",      "action": "unchanged", "before": { }, "after": { } }
  ],
  "errors": [ { "model_name": "foo", "message": { "key": "{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}", "params": { "path": "reasoning.supported_efforts", "value": "ultra", "allowed": "none, minimal, low, medium, high, xhigh, max" } } }  // 结构沿用 common.NewMessage 的序列化形状 ],
  "summary": { "create": 1, "update": 1, "clear": 1, "unchanged": 1 }
}
// PUT 成功 data
{ "create": 1, "update": 1, "clear": 1 }
```
- `errors` 非空时 PUT 返回 400，什么都不改。
- 版本不一致时 PUT 返回 HTTP 409，`message` 为英文原文 `Model metadata changed. Preview again before saving.`（前端 locale 补译文）。

## 函数清单
### model/model_capability_batch.go
| 函数名 | 职责 |
|---|---|
| `ListExactModelCapabilities` | 读全部记录，分出已配置的完全匹配记录、非完全匹配记录、未配置的完全匹配记录 |
| `PlanModelCapabilityReplace` | 按整体替换规则对比当前数据，产出 items、errors、summary 和涉及记录集合 |
| `modelCapabilityReplaceVersion` | 对涉及记录（按 id 排序）的 `MetadataRecordVersion`、待新建的名字及其复制来源记录求 sha256 |
| `ApplyModelCapabilityReplace` | 在 `metadataTransaction` 里加锁重读、重算计划与版本，不一致返回冲突错误，一致则执行新建、更新、清空 |

### controller/model_capability.go
| 函数名 | 职责 |
|---|---|
| `GetModelCapabilities` | 调 `ListExactModelCapabilities` 返回 |
| `PreviewModelCapabilities` | 解析请求体，调 `PlanModelCapabilityReplace` 返回计划 |
| `ReplaceModelCapabilities` | 调 `ApplyModelCapabilityReplace`；冲突 409、校验错误 400；成功后 `RefreshPricing` 并返回计数 |

## 协作关系
- 依赖 01 的 `ParseModelCapability`、规范 JSON；依赖 04 的 `NewExactModelRecord`。
- 事务骨架与版本思路参照 `model/model_metadata_sync.go`：`metadataTransaction`（进程锁 + options 锚点行加锁，提交后自动 `BumpSyncTopic`）、`lockForUpdate`、`ErrMetadataSyncConflict` → 409 的映射方式（`controller/model_sync.go:455`）。
- 请求体解析用 `common.DecodeJson`，不直接用 `encoding/json`。
- 不写原生 SQL；三种数据库都要跑接口用例。

## 验证方式
- 测试入口：`GET /api/models/capabilities`、`POST /api/models/capabilities/preview`、`PUT /api/models/capabilities`（`modelManagementDB` + `modelManagementRequest`）；无权限用户用普通用户令牌请求。
- 测试输入：完全匹配记录 `gpt-6.2-sol`（上下文、模态）、`qwen4-max`（思考）、`glm-5.2`（未配置）；前缀记录 `gpt-6`（描述、接口配置）；提交 JSON：改 `gpt-6.2-sol` 上下文为 900000 并去掉 architecture、删掉 `qwen4-max`、新增 `gpt-6.3-sol` 与 `foo-model`。
- 预期结果：
- [ ] GET 只列 `gpt-6.2-sol`、`qwen4-max`（排序），`non_exact` 有 `gpt-6`，`unconfigured` 有 `glm-5.2`
- [ ] preview：`gpt-6.3-sol`、`foo-model` 为 create，`gpt-6.2-sol` update（architecture 清空），`qwen4-max` clear，summary 正确
- [ ] PUT 后：`gpt-6.3-sol` 复制了 `gpt-6` 的资料，`foo-model` 为默认资料；`qwen4-max` 记录在、能力为空；`gpt-6` 能力不变
- [ ] 含 `"ultra"` 的提交：preview 有 errors；PUT 返回 400，所有记录不变
- [ ] preview 后另行修改 `qwen4-max` 再 PUT → 409，所有记录不变；重新 preview 后 PUT 成功
- [ ] preview 后另行新建同名完全匹配记录 `gpt-6.3-sol` 再 PUT → 409
- [ ] 模拟事务内一条写入失败 → 全部回滚
- [ ] 普通用户请求三条接口均被拒绝；访问令牌作用域测试 `go test ./router/ ./middleware/` 通过
- [ ] SQLite、MySQL、PostgreSQL 各跑一遍接口用例
