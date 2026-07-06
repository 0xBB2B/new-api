---
name: 04-frontend-config
description: 前端 type 59 配置 UI——类型下拉/凭据提示/图标、凭据校验、刷新按钮与免责声明、隐藏 batch、6 语言文案。
---
# 前端渠道配置 UI

## 目标
管理员在 web/default 能选择并配置 Claude Subscription 渠道:粘贴 Claude Code OAuth JSON、看到刷新按钮与合规免责声明、无 batch/多 key 入口;文案六语齐全。

## 业务规则（来源：spec claude-subscription/frontend-config-ui, single-key-only）
- 渠道类型下拉含 `Claude Subscription`(type 59)且可选。
- Key 输入提示指向「粘贴 Claude Code OAuth JSON 凭据」。
- 编辑该类型渠道时展示「刷新凭据」按钮(调后端刷新)与合规免责声明(仅个人订阅使用、遵守 Anthropic 条款)。
- 该类型表单不出现 batch/多 key 入口;提交多 key 被校验拒绝。

## 涉及文件
- `web/default/src/features/channels/constants.ts` — 修改
- `web/default/src/features/channels/lib/channel-form.ts` — 修改
- `web/default/src/features/channels/lib/channel-utils.ts` — 修改
- `web/default/src/features/channels/lib/channel-type-config.ts` — 修改（可选，加默认 base URL 元数据）
- `web/default/src/features/channels/api.ts` — 修改
- `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx` — 修改
- `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json` — 修改

## 函数清单

### `constants.ts`
| 符号 | 职责 |
|---|---|
| `CHANNEL_TYPES` | 新增 `59: 'Claude Subscription'` |
| `CHANNEL_TYPE_DISPLAY_ORDER` | 59 排入 Anthropic(14)邻近位置 |
| `TYPE_TO_KEY_PROMPT` | 新增 59 → 「粘贴 Claude Code OAuth JSON 凭据」提示 |

### `lib/channel-form.ts`
| 符号 | 职责 |
|---|---|
| `isClaudeCredential` | 校验解析 JSON 含 claudeAiOauth.accessToken(+refreshToken) |
| (superRefine type 59 分支) | 禁 batch/multi-key(报错文案),凭据走 isClaudeCredential 校验 |
| (`buildSettingsJSON` type 59 分支) | 归入 Anthropic 设置组(claude_beta_query 等透传开关) |

### `lib/channel-utils.ts`
| 符号 | 职责 |
|---|---|
| `getChannelTypeIcon`(TYPE_TO_ICON) | 新增 `59: 'Claude'`(渲染 Claude 图标) |

### `lib/channel-type-config.ts`（可选）
| 符号 | 职责 |
|---|---|
| `CHANNEL_TYPE_CONFIGS` | 可选新增 59 元数据(icon:'anthropic'、defaultBaseUrl 'https://api.anthropic.com') |

### `api.ts`
| 符号 | 职责 |
|---|---|
| `refreshClaudeCredential` | `POST /api/channel/${id}/claude/refresh`,复用 channelActionConfig |
| `ClaudeCredentialRefreshResponse`(type) | 刷新响应类型 |

### `components/drawers/channel-mutate-drawer.tsx`
| 符号 | 职责 |
|---|---|
| (import) | 引入 refreshClaudeCredential |
| `isClaudeCredentialRefreshing`(state) | 刷新中标志 |
| `handleRefreshClaudeCredential` | 调 refreshClaudeCredential,toast,失效 channelsQueryKeys.detail |
| `supportsMultiKeyAddMode` | 扩展排除 type 59 |
| (type 59 专属区块) | 说明文案 + 刷新按钮(isEditing&&channelId 时)+ 免责声明 Alert |
| (nav/passthrough gating) | 59 归入 Anthropic 分组 |

## 协作关系
- `handleRefreshClaudeCredential` → `api.ts:refreshClaudeCredential` → 后端 `POST /:id/claude/refresh`(03 提供)。
- 类型下拉/图标/提示均按 type 59 字面量判断(与 codex 的 57 同风格)。
- 新增 `t('...')` key 必须同步 6 个 locale;走 i18n-translate skill 补全。

## 验证方式
- [ ] 新建渠道选 `Claude Subscription` → Key 提示为 OAuth JSON、无 batch 入口、显示免责声明。
- [ ] 编辑该类型 → 「刷新凭据」按钮可见,点击调后端并 toast 结果。
- [ ] 提交多 key/batch 被前端校验拒绝。
- [ ] 类型图标渲染为 Claude 而非 OpenAI 默认。
- [ ] 6 个 locale 新增 key 对称(i18n:sync 门槛通过);`bun run build` 通过。
