---
name: 04-capability-migration
description: 升级时一次性把内置表 32 个型号和卡片覆盖写进模型记录，缺记录时新建并复制命中记录资料，删除覆盖选项
---
# 思考能力搬迁

## 目标
从上一个版本升级后第一次启动，原内置表与「模型思考能力」卡片里的思考等级都落到模型记录上，智能路由行为不变；之后只跑这一次。

## 业务规则（来源：PRD 04-single-source-and-migration、02-capabilities-json-window C10）
- 搬迁数据：卡片覆盖（选项键 `smart_routing_setting.thinking_capabilities`，JSON 形如 `{"<模型名>":{"no_thinking":bool,"efforts":[...]}}`）+ 下方 32 个型号快照；同一型号两份都有时用覆盖值。`no_thinking: true` → `supported_efforts: []`。
- 写到该型号名的完全匹配记录：已有 → 只写思考等级（其他能力和资料不动）；没有 → 新建完全匹配记录，只带思考等级。
- 新建时若名字被前缀、后缀、包含规则的记录命中，复制命中记录的描述、图标、标签、厂商、接口配置、广场可见状态（status）、是否允许同步资料（sync_official）；没有命中时用页面手动新建记录的默认值。
- 只做一次：成功后写完成标记；之后再启动不新建记录，也不覆盖管理员后来改过的思考等级。中途失败时下次启动从头重做，结果与一次做完相同，不产生重复记录（整个搬迁在一个事务里）。
- 搬迁完成后从 options 表删除覆盖选项那一行。
- 搬迁前后，每个搬迁型号在智能路由里取到的思考能力一致。
- 在 SQLite、MySQL、PostgreSQL 上分别验证：全新库、上一版本库升级、连续启动两次。

## 涉及文件
- 新建 `model/model_capability_migration.go`
- 修改 `model/main.go`（`migrateDB` 末尾、仅主节点的一次性迁移段，约 :371-382，调用搬迁函数）
- 新建 `model/model_capability_migration_test.go`

## 成品定义
完成标记（options 表一行，写入用 `clause.OnConflict{DoNothing: true}`，先例 `model/user_access_token.go:305-320`）：
```go
const modelCapabilityMigrationDoneKey = "model_capability_migration_done" // 值为完成时的 Unix 秒
const legacyThinkingCapabilitiesOptionKey = "smart_routing_setting.thinking_capabilities"
```
32 个型号快照（搬迁专用的冻结数据，只在搬迁函数里读，运行时取值不查它）：
```go
// 2026-11-18 升级前 service/smartroute/capability.go 内置表的最终内容，出处注释见该文件 git 历史（PR #51）。
var legacyBuiltinThinkingEfforts = map[string][]string{
	"gpt-4.1":             {},
	"gpt-4o":              {},
	"gpt-4.1-mini":        {},
	"gpt-5.4":             {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.4-mini":        {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.4-nano":        {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.5":             {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.4-pro":         {"medium", "high", "xhigh"},
	"gpt-5.5-pro":         {"medium", "high", "xhigh"},
	"gpt-5.2-codex":       {"low", "medium", "high", "xhigh"},
	"gpt-5.3-codex":       {"low", "medium", "high", "xhigh"},
	"gpt-5.6":             {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-5.6-sol":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-5.6-terra":       {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-5.6-luna":        {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6-sol":           {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6-luna":          {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.1-terra":       {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6-astra":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.1-sol":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.2-sol":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.2-astra":       {"none", "low", "medium", "high", "xhigh", "max"},
	"glm-5.3":             {"low", "high", "max"},
	"glm-5.3-flash":       {"low", "high", "max"},
	"glm-5.3-flashx":      {"low", "high", "max"},
	"glm-5.2":             {"none", "high", "max"},
	"deepseek-v4-pro":     {"none", "low", "high", "max"},
	"deepseek-v4-flash":   {"none", "low", "high", "max"},
	"deepseek-v4.1-flash": {"none", "low", "high", "max"},
	"kimi-k3.1-pro":       {"none", "low", "high", "max"},
	"qwen3.8-max":         {"none", "low", "medium", "xhigh"},
	"qwen4-max":           {"none", "low", "high", "xhigh"},
}
```
空切片表示不支持思考（对应 `supported_efforts: []`）。

## 函数清单
### model/model_capability_migration.go
| 函数名 | 职责 |
|---|---|
| `MigrateLegacyThinkingCapabilities` | 已有完成标记则直接返回；否则在一个事务里读覆盖选项、合并快照、逐个写入记录、删覆盖选项、写完成标记 |
| `legacyThinkingCapabilityPlan` | 合并覆盖与快照得到"型号名 → 思考档位"（覆盖优先），覆盖 JSON 解析失败时只用快照并 `common.SysError` 记录 |
| `upsertExactRecordThinking` | 有完全匹配记录则只改其能力里的思考等级；没有则调 `NewExactModelRecord` 新建 |
| `NewExactModelRecord` | 新建完全匹配记录：若名字被非完全匹配记录命中（按 `resolveModelMetadata` 顺序）则复制该记录的描述、图标、标签、厂商、接口配置、status、sync_official，否则用手动新建的默认值；05 复用 |

### model/main.go
| 函数名 | 职责 |
|---|---|
| `migrateDB`（现有） | AutoMigrate 之后、仅主节点时调用 `MigrateLegacyThinkingCapabilities` |

## 协作关系
- 依赖 01 的能力类型、规范 JSON、`GetModelCapabilityByName` 行为（只改思考等级时保留已有其他能力）。
- 事务用 `DB.Transaction`；写入完成后 `BumpSyncTopic(SyncTopicModels)`，本实例启动流程随后刷新定价缓存。
- 选项删除用 GORM `Delete`，不写原生 SQL；`key` 列用 `commonKeyCol`。
- 03 删除 operation_setting 中的覆盖设置，本文件自带键名常量，不依赖它。

## 验证方式
- 测试入口：`model.MigrateLegacyThinkingCapabilities`（SQLite 内存库夹具，同 `model/request_policy_test.go:28`）；三库验证用真实 SQLite、MySQL、PostgreSQL 实例启动服务。
- 测试输入：覆盖选项 `{"glm-5.3":{"no_thinking":false,"efforts":["low","high"]},"my-model":{"no_thinking":true,"efforts":[]}}`；已有完全匹配记录 `gpt-5.5`（描述"x"、已配上下文 1000）；前缀记录 `gpt-6`（描述"GPT-6 家族"、接口配置、status=1）；无命中的 `glm-5.2`。
- 预期结果：
- [ ] 32 个快照型号与 `my-model` 都有完全匹配记录，思考等级正确；`glm-5.3` 为 low/high
- [ ] `my-model`、`gpt-4o` 为不支持思考
- [ ] `gpt-5.5` 描述仍是"x"、上下文仍是 1000，思考等级写入
- [ ] `gpt-6.2-sol` 新记录的描述、图标、标签、厂商、接口配置、status、sync_official 与 `gpt-6` 相同
- [ ] `glm-5.2` 新记录其他资料为默认值
- [ ] options 表里没有覆盖选项行，有完成标记
- [ ] 再跑一次：记录数不变；先把 `gpt-6.2-sol` 改成 low/high 再跑 → 仍是 low/high
- [ ] 事务中途注入失败 → 记录与选项都回到搬迁前，再跑一次结果与一次做完相同
- [ ] 对每个快照型号，搬迁后 `smartroute.EffectiveThinkingCapability` 的档位与快照一致
- [ ] SQLite、MySQL、PostgreSQL：全新库、上一版本库升级、连续启动两次均通过，记录引擎版本、命令和结果
