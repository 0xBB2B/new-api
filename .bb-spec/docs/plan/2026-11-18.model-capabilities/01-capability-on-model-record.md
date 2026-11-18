---
name: 01-capability-on-model-record
description: 模型记录新增能力列，定义能力 JSON 的规范化与校验，增改接口保存并校验，内存缓存按名字解析能力
---
# 模型记录上的能力存储、校验与解析

## 目标
模型记录能保存五项能力；服务器拒绝非法能力；任何模块都能按模型名取到命中记录上的能力，保存后立即可见。

## 业务规则（来源：PRD 01-capability-on-model-record、03-v1-models-capabilities）
- 五项能力：`context_length`、`top_provider.max_completion_tokens`（1 到 9007199254740991 的整数）；`architecture.input_modalities`（text、image、file、audio）；`architecture.output_modalities`（text、image、audio）；`reasoning.supported_efforts`（none、minimal、low、medium、high、xhigh、max，`[]` 表示不支持思考）。
- 每项都可以未配置；未配置的字段不出现在 JSON 里；全部未配置为 `{}`。`null` 和空模态数组 `[]` 按未配置处理。
- 只认上面五个位置，其他字段一律拒绝；不认识的值、不满足范围的数字一律拒绝，错误信息写字段路径和原因。服务器错误是英文原文 key 加参数（`common.NewMessage`），前端翻译成「architecture.input_modalities 里有不认识的值 video，可用的值：text、image、file、audio」这类中文；三种 key：`{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}`、`{{path}} must be an integer from 1 to 9007199254740991`、`{{path}} is not a capability field`。
- 保存时去掉重复值；思考档位按 none→max 顺序保存。
- 能力跟这条记录的其他资料一起保存；「同步官方资料」不改能力。
- 按名字取能力时沿用现有命中顺序：完全匹配优先，其次前缀、后缀、包含，同规则取第一条命中的；整条记录一起生效。
- 编辑保存后，下一次请求就能取到新值（本实例立即生效，其他实例经现有同步主题刷新）。

## 涉及文件
- 修改 `model/model_meta.go`（Model 新增 Capabilities 列；更新函数 Select 白名单加入该列）
- 新建 `model/model_capability.go`（能力类型、规范化、校验、按名字解析）
- 修改 `model/pricing.go`（定价缓存刷新时一并缓存全部模型记录，供按名字解析）
- 修改 `controller/model_meta.go`（创建、更新时校验能力；保存成功后刷新本实例缓存并发同步主题）
- 新建 `model/model_capability_test.go`

## 成品定义
数据库列（GORM 标签，`AutoMigrate` 自动加列，三种数据库都是 TEXT，不设默认值）：
```go
// Capabilities 存规范化后的能力 JSON（形状与 /v1/models 一致），空串表示五项都未配置。
Capabilities string `json:"capabilities,omitempty" gorm:"type:text"`
```
规范化后的能力 JSON（存储、接口传输、前端 JSON 模式共用；键顺序固定如下，未配置的键省略）：
```json
{
  "context_length": 1050000,
  "top_provider": { "max_completion_tokens": 128000 },
  "architecture": { "input_modalities": ["text", "image"], "output_modalities": ["text"] },
  "reasoning": { "supported_efforts": ["none", "low", "medium", "high", "xhigh", "max"] }
}
```
- `top_provider`、`architecture`、`reasoning` 内部一个字段都没有时省略该对象。
- 模态按上面枚举列出的顺序排序；思考档位按 none、minimal、low、medium、high、xhigh、max 排序。
- 全部未配置时存空串，接口返回时字段省略；前端 JSON 模式显示 `{}`。

## 函数清单
### model/model_capability.go
| 函数名 | 职责 |
|---|---|
| `ModelCapability`（类型） | 五项能力的结构体，各字段为指针或切片指针以区分未配置 |
| `ParseModelCapability` | 把任意 JSON 文本解析成能力，拒绝多余字段、不认识的值、越界数字，返回逐条错误 |
| `(ModelCapability) Normalize` | 去重、排序、把空模态数组和 null 变成未配置 |
| `(ModelCapability) MarshalCanonical` | 输出上面成品定义的规范 JSON，全部未配置时为空串 |
| `(ModelCapability) IsEmpty` | 五项是否都未配置 |
| `ModelThinkingSupport` | 返回记录上的思考配置：未配置 / 不支持思考 / 档位列表 |
| `GetModelCapabilityByName` | 从缓存的全部模型记录里按命中顺序找到记录，返回其能力；没有命中返回未配置 |
| `ValidateModelCapabilityText` | 给接口用：解析加规范化，返回规范 JSON 或错误 |

### model/model_meta.go
| 函数名 | 职责 |
|---|---|
| 更新函数（现有，约 :268 的 Select 白名单） | 白名单加入 `capabilities` |

### model/pricing.go
| 函数名 | 职责 |
|---|---|
| `updatePricing`（现有） | 读出全部模型记录时同时存一份供 `GetModelCapabilityByName` 用的快照 |

### controller/model_meta.go
| 函数名 | 职责 |
|---|---|
| `CreateModelMeta` / `UpdateModelMeta`（现有） | 调 `ValidateModelCapabilityText`，非法返回 400 和错误文案；写入规范 JSON；成功后调用现有 `RefreshPricing` 并 `BumpSyncTopic(SyncTopicModels)` |

## 协作关系
- `GetModelCapabilityByName` 复用 `resolveModelMetadata` 的命中顺序，读 `updatePricing` 存下的记录快照；`RefreshPricing` 强制重建快照。
- 被 02（/v1/models）、03（智能路由）、04（搬迁）、05（能力配置 JSON 接口）调用。
- `MetadataRecordVersion` 按 Model 的 JSON 求哈希，新列自动参与；同步官方资料的应用逻辑只更新 `MetadataSyncFields` 列表里的列，不得把 capabilities 加进去。
- 错误文案走 `common.NewMessage` / `common.ApiErrorT`（英文原文作 key），前端 7 个 locale 文件补对应 key（由 06 一并补）。

## 验证方式
- 测试入口：`model` 包的 `ParseModelCapability`、`GetModelCapabilityByName`；管理接口 `POST /api/models/`、`PUT /api/models/`（沿用 `controller/model_management_test.go` 的 `modelManagementDB` / `modelManagementRequest`）。
- 测试输入：上面成品 JSON；含 `"video"`、`"ultra"`、`"foo":1`、`context_length` 为 0、-5、1.5、9007199254740992 的 JSON；`["high","low","high"]`；`input_modalities: []`；`null` 字段。
- 预期结果：合法输入保存后读回规范 JSON；非法输入 400 且记录不变；重复与乱序被规范化；空数组和 null 变未配置；全未配置存空串。
- [ ] 合法五项保存后 GET 单条返回规范 JSON
- [ ] 每类非法输入都 400，错误信息含字段路径
- [ ] 完全匹配 `gpt-6.2-sol`（未配上下文）与前缀 `gpt-6`（上下文 1050000）并存：按名 `gpt-6.2-sol` 取不到上下文，`gpt-6.2-luna` 取到 1050000
- [ ] 更新后不重启，`GetModelCapabilityByName` 立即返回新值
- [ ] 对配置过能力的记录执行同步官方资料应用，能力不变
- [ ] SQLite、MySQL、PostgreSQL 各跑一次加列与增改（按 AGENTS.md 数据库验证规则，记录版本与命令）
