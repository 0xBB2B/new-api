---
name: 08-group-order-compact
description: 分组顺序组件新增一行紧凑样式：带序号的小标签、拖动排序、Alt 加方向键移动、加号添加
---
# 分组顺序紧凑样式

## 目标
`AutoGroupOrderEditor` 多一种紧凑样式，智能路由弹窗用它把分组顺序压成一行；密钥抽屉里的原样式不变。

## 业务规则（来源：PRD）
- 04-editor-layout C9：一行带序号的小标签（「1 subscription」「2 api」），每个标签左边一个拖动把手、右边一个 ×，最后一个「＋」，点开是可选分组的下拉。
  - 拖动标签调整顺序；鼠标停在标签上光标变成抓手；只有一个分组时不能拖。
  - 键盘聚焦标签后按 Alt + ← / → 移动顺序。
  - 满上限（默认 5 个）时「＋」变灰；没选分组时只显示「＋ 添加分组」；出错（`aria-invalid`）时这一行描红。
  - 增删、排序、上限的规则和原组件相同；只是样子不同。
- 写法照抄现有代码：小标签和序号圆点照组件里「继承全局 Auto 顺序」那种现成小标签；拖动照 `web/src/features/channels/components/dialogs/param-override-editor-dialog.tsx:1953-1982` 的 `draggable` + `GripVertical` + 拖动中 `opacity-50`；插入位置用一条主色细竖线。
- 项目前端规则：复用现有组件，不新建平行组件；不加新的第三方拖动库。

## 涉及文件
- 修改 `web/src/features/keys/components/auto-group-order-editor.tsx`
- 修改 7 个 `web/src/i18n/locales/*.json`（如有新文案，例如「Add group」已存在则复用）
- 测试：扩展 `web/src/features/keys/components/__tests__/auto-group-order-editor.test.tsx`

## 函数清单
### web/src/features/keys/components/auto-group-order-editor.tsx
| 函数名 | 职责 |
|---|---|
| `AutoGroupOrderEditor` | 新增 `variant` 属性（默认原样式，`'compact'` 为一行样式）；两种样式共用增删、排序、上限逻辑 |
| 紧凑样式的拖动处理 | 拖动开始 / 经过 / 放下 / 结束，算出插入位置后调用现有的排序逻辑 |
| 紧凑样式的键盘处理 | 标签聚焦时 Alt + ← / → 调用现有的上移、下移 |

## 协作关系
09 的弹窗用 `variant='compact'`。密钥抽屉 `api-keys-mutate-drawer.tsx` 不传 `variant`，保持原样。

## 验证方式
- 测试入口：渲染 `<AutoGroupOrderEditor variant='compact' ... />`（vitest + Testing Library，沿用现有测试文件写法）
- 测试输入：value=['subscription','api']，options 含 5 个以上分组；value=[]；value 满 5 个；`aria-invalid`
- 预期结果：
  - [ ] 显示「1 subscription」「2 api」两个标签和一个「＋」；value=[] 时只显示「＋ 添加分组」
  - [ ] 在 subscription 上触发拖动、放到 api 之后 → `onChange` 收到 ['api','subscription']
  - [ ] 聚焦 subscription 按 Alt + → → `onChange` 收到 ['api','subscription']；聚焦第一个按 Alt + ← → 不调用
  - [ ] 只有一个分组时标签不可拖（没有 draggable）
  - [ ] 点 × → 移除对应分组；点「＋」选一个分组 → 追加到末尾；满 5 个时「＋」禁用
  - [ ] 不传 `variant` 时渲染结果和改动前一致（现有测试全部通过）
