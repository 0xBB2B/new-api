---
name: 11-frontend-config-page
description: 请求策略新增「智能路由」分区：虚拟模型卡片、表格、可视化/JSON 切换、空状态、删除确认、保存更改
---
# 智能路由配置页

## 目标
管理员在「系统设置 → 请求策略 → 智能路由」能看到、增删虚拟模型，并通过页面右上角「保存更改」保存到 `smart_routing_setting.routes`。

## 业务规则（来源：PRD 05-config-page C1~C4、C8~C10，以及用户确认过的预览页 2026-11-04.smart-routing.html）
- 菜单：左侧「请求策略」下，在「渠道健康」后面新增「智能路由」，能看到其他请求策略分区的人才能看到它。
- 页面标题「智能路由」，右上角「保存更改」。正文是一张「虚拟模型」卡片：
  - 标题后面是数量徽标；
  - 说明"每个虚拟模型配好三档模型、打分模型和顾问。用户在客户端里把模型名填成虚拟模型名就能用。"；
  - 「可视化 / JSON」切换，右侧「添加虚拟模型」按钮；
  - 下面是表格。

  写法和「会话规则」卡片（`features/system-settings/general/channel-affinity/index.tsx`）相同，但没有「填充模板」按钮，也没有底部的缓存统计栏。
- 表格列：名称、简单档、普通档、困难档、打分模型、顾问、操作。
  - 名称列：第一行虚拟模型名，第二行"<接口> 接口"（Responses / Chat Completions，多个用 " / " 连接）。
  - 三个档位列：第一行模型名，默认档后面带「默认」徽标；第二行"分组 → 分组 · 思考力度"，思考力度为空时显示"不覆盖"。
  - 打分模型列：第二行"分组 → 分组 · 超时 X 毫秒"。
  - 顾问列：第二行"连续 N 次不满意"。
  - 操作列：编辑图标，以及「…」菜单里的「删除」；删除前用 `ConfirmDialog` 确认，确认后这一行从当前表单里移除（点「保存更改」才真正生效）。
- 空状态：用 `EmptyState`，标题"还没有虚拟模型"，说明"添加一个虚拟模型，比如 auto-gpt。用户在客户端里把模型名填成它就能用。"，按钮「添加虚拟模型」。
- 「JSON」视图用 `JsonCodeEditor` 显示同一份数组，可以直接编辑。切回可视化前先检查是不是数组，不是就提示错误、不切换（照会话规则的写法）。
- 「保存更改」：和这一页其他分区一样，与基线比较，没有改动就提示"没有需要保存的更改"；有改动就通过 `PATCH /api/option/request_policy` 提交 `{ "smart_routing_setting.routes": "<JSON 字符串>" }`。后端整体检查失败时，用 `handleServerError` 提示后端返回的消息。
- 接口显示的来源：同一模型在定价数据里的 `supported_endpoint_types`（`features/pricing/hooks/use-pricing-data.ts` 的 `usePricingData`）。取三档和顾问的交集，只保留 `openai` / `openai-response`。
- 所有新增文字都要有 en、zh、zh-TW、fr、ru、ja、vi 七种翻译：按仓库的 i18n-translate skill，用 `add-missing-keys.mjs` 一次写入七种语言，再跑 `bun run i18n:sync`，不手改 JSON。`titleKey` 这类不是以 `t('...')` 字面量出现的文字，要登记到 `src/i18n/static-keys.ts`。

## 涉及文件
- web/src/features/system-settings/request-policies/section-registry.tsx（修改：在 health 后面加 `{ id: 'smart-routing', titleKey: 'Smart routing', build: () => <SmartRoutingSection /> }`）
- web/src/features/system-settings/request-policies/smart-routing/index.tsx（新建：分区组件，负责读取、保存）
- web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx（新建：卡片 + 切换 + 表格 + 空状态 + 删除确认）
- web/src/features/system-settings/request-policies/smart-routing/types.ts（新建：与后端成品一致的 TS 类型）
- web/src/features/system-settings/request-policies/smart-routing/lib.ts（新建：解析和序列化配置、接口交集、表格显示文字）
- web/src/i18n/locales/*.json、web/src/i18n/static-keys.ts（修改）
- web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx（新建，本功能前端唯一新增的测试文件，12 的用例也写在这里）

## 成品定义
TS 类型（与后端 01 的 JSON 一致）：

```ts
export type SmartTierId = 'simple' | 'normal' | 'hard'
export type SmartRouteTier = { model: string; groups: string[]; effort: '' | 'low' | 'medium' | 'high' }
export type SmartRoute = {
  name: string
  tiers: Record<SmartTierId, SmartRouteTier>
  default_tier: SmartTierId
  scorer: { model: string; groups: string[]; timeout_ms: number }
  advisor: { model: string; groups: string[]; after_dissatisfied: number }
  downgrade_max_history_tokens: number
  confidence_threshold: number
  simple_max_message_chars: number
}
```

新增翻译键（英文原文 → 简体中文）：

| key | zh |
|---|---|
| Smart routing | 智能路由 |
| Virtual models | 虚拟模型 |
| Each virtual model has three tier models, a scorer model and an advisor. Users enter the virtual model name as the model in their client. | 每个虚拟模型配好三档模型、打分模型和顾问。用户在客户端里把模型名填成虚拟模型名就能用。 |
| Add virtual model | 添加虚拟模型 |
| Simple tier | 简单档 |
| Normal tier | 普通档 |
| Hard tier | 困难档 |
| Scorer model | 打分模型 |
| Advisor | 顾问 |
| Default | 默认 |
| {{endpoint}} endpoint | {{endpoint}} 接口 |
| Keep client value | 不覆盖 |
| Timeout {{ms}} ms | 超时 {{ms}} 毫秒 |
| After {{count}} dissatisfied messages | 连续 {{count}} 次不满意 |
| No virtual models yet | 还没有虚拟模型 |
| Add a virtual model such as auto-gpt. Users enter it as the model name in their client. | 添加一个虚拟模型，比如 auto-gpt。用户在客户端里把模型名填成它就能用。 |
| Delete virtual model | 删除虚拟模型 |
| Delete {{name}}? It takes effect after you save changes. | 确定删除 {{name}} 吗？点「保存更改」后生效。 |

（"Visual"、"JSON"、"Edit"、"Delete"、"No changes to save"、"Save Changes" 等现有键直接复用。）

## 函数清单
### smart-routing/index.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRoutingSection` | 读取请求策略配置，维护虚拟模型列表的本地状态与基线，挂 `SettingsPageFormActions`，保存时比较改动并调用 `useSavePolicy` |

### smart-routing/routes-card.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRoutesCard` | 卡片外壳：标题、数量、说明、可视化/JSON 切换、添加按钮；打开 12 的编辑弹窗 |
| `SmartRoutesTable` | `StaticDataTable` 表格与空状态，行操作为编辑和删除 |

### smart-routing/lib.ts
| 函数名 | 职责 |
|---|---|
| `parseSmartRoutes` / `serializeSmartRoutes` | option 字符串与数组之间互转；解析失败时返回空数组并给出错误 |
| `routeEndpoints` | 按定价数据求接口交集 |
| `tierSummary` | 生成档位列第二行的文字 |

## 协作关系
- 读：`GET /api/option/request_policy`（与会话分区共用 queryKey `['request-policy']`）。写：`useSavePolicy`（`PATCH /api/option/request_policy`）。
- 编辑弹窗由 12 提供：`RouteEditorDialog`（打开时传入当前虚拟模型或 null，以及其他虚拟模型的名称列表）。

## 验证方式
- 测试入口：渲染 `RequestPolicies` 或 `SmartRoutingSection`，mock 写法照 `request-policies/__tests__/routing.test.tsx`（`vi.spyOn(api, 'get' | 'patch')`，包 `QueryClient`、`RouterContextProvider`、`SettingsPageProvider`）。
- 测试输入：options 里 `smart_routing_setting.routes` 为空数组 / 含 auto-gpt 和 auto-chat 两条。
- 预期结果：
- [ ] 菜单里有「Smart routing」，位于「Channel health」之后
- [ ] 空数组 → 显示 "No virtual models yet" 和添加按钮
- [ ] 两条数据 → 数量徽标为 2，各列两行文字与成品一致，默认档带 "Default" 徽标
- [ ] 删除 → 先弹确认；确认后点保存更改，PATCH 提交的数组里没有这一条
- [ ] 没有改动时点保存更改 → 提示 "No changes to save"，没有发 PATCH
- [ ] JSON 视图写入非数组再切回可视化 → 报错且停留在 JSON 视图
- [ ] PATCH 返回错误 → 显示后端消息
- [ ] 七种语言文件都有新增键（`bun run i18n:sync` 后没有缺失报告）
