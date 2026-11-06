---
env:
  up: docker compose -f docker-compose.dev.yml up -d --build && (cd web && bun run dev -- --host 0.0.0.0 --port 5173)
  down: docker compose -f docker-compose.dev.yml down -v && 停掉 5173 端口的前端开发服务器
  frontends:
    web: 5173
  seed: .bb-spec/docs/test/webview/fixtures/smart-routing-seed.sh
---

# 网页交互用例索引

运行前先执行 `fixtures/smart-routing-seed.sh` 灌数据（在仓库根执行 `.bb-spec/docs/test/webview/fixtures/smart-routing-seed.sh`，可选参数是后端地址，默认 http://localhost:3000）。脚本会建好 root / alice 两个账号和 root 的密钥 cc-switch-key、6 个分组（含 team-a / team-b / team-c）和用户可选的 auto 分组、Codex / OpenAI / MiniMax 三种类型的渠道、模型价格、13 条使用日志（10 条主请求里 9 条带思考力度，覆盖八种来源和工具结果续跑，另 1 条是没有思考力度的一期旧日志；2 条打分记录；1 条没经过智能路由的普通日志），并关掉「Chat 转 Responses」策略和全局透传。日志的时间是执行脚本前 1~8 分钟，使用日志页默认只看当天，所以要在当天执行脚本后再跑用例。

脚本会把一期格式的智能路由配置直接写进数据库，再重启后端容器（`docker compose restart new-api`），让启动迁移把它改成新格式。`legacy-config-migrated` 核对的就是这份迁移结果，所以它排在第一个，必须在执行脚本后第一个跑；其他用例会在 setup 里改写智能路由配置，teardown 里清空。

登录和刷新会话共用每个 IP 20 分钟 20 次的限额，每次整页加载都会刷新一次会话，一个用例大约用掉 3~5 次，多语言和 iframe 用例用得更多。脚本最后会清零这个计数；用例总数较多，建议在后端环境变量里设 `CRITICAL_RATE_LIMIT_ENABLE=false`，或一轮跑不完时先再执行一次脚本（再跑脚本会重置智能路由配置，`legacy-config-migrated` 需要重新先跑）。连续跑全量时，root 的会话数会累计到上限，登录返回 409；seed 脚本开头会清空 user_sessions，一轮跑到中途遇到时，可以重跑 seed，或手动清理会话（`docker compose -f docker-compose.dev.yml exec -T postgres psql -U root -d new-api -c "delete from user_sessions"`）。

## web（http://localhost:5173）

### smart-routing-config（智能路由配置页，一期 PRD 05；升级迁移，二期 PRD 01 / 03）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [legacy-config-migrated](web/smart-routing-config/legacy-config-migrated.md) | 用一期格式的旧配置启动后，配置页显示困难档「固定 high」、接口只勾 Responses（必须在 seed 后第一个跑） | ✅ 通过（2026-11-06） |
| [menu-entry](web/smart-routing-config/menu-entry.md) | 管理员在请求策略菜单里看到「智能路由」，位于「渠道健康」之后，点进去打开智能路由页 | ✅ 通过（2026-11-06） |
| [empty-state](web/smart-routing-config/empty-state.md) | 没有虚拟模型时显示空状态的标题、说明和按钮，点按钮打开添加弹窗 | ✅ 通过（2026-11-06） |
| [add-dialog-defaults](web/smart-routing-config/add-dialog-defaults.md) | 添加弹窗里接口、三档思考力度（不覆盖）、默认档、超时、连续不满意次数和高级设置三个数字都是默认值 | ✅ 通过（2026-11-06） |
| [add-and-save](web/smart-routing-config/add-and-save.md) | 在添加弹窗的三个 Tab 里配好 auto-gpt，弹窗保存后点「保存更改」，表格出现这一行且刷新后仍在 | ✅ 通过（2026-11-06） |
| [save-without-changes](web/smart-routing-config/save-without-changes.md) | 已有 auto-gpt、没做任何修改时点「保存更改」，提示没有需要保存的更改 | ✅ 通过（2026-11-06） |
| [unsaved-refresh](web/smart-routing-config/unsaved-refresh.md) | 编辑 auto-gpt（顾问 Tab）并在弹窗里保存，但不点「保存更改」直接刷新，改动不生效 | ✅ 通过（2026-11-06） |
| [edit-save-error](web/smart-routing-config/edit-save-error.md) | 困难档换成在 api 分组没有渠道的模型，保存时字段下出现红字、自动跳到「档位 → 困难」、Tab 带红点；改回原模型后保存成功 | ✅ 通过（2026-11-06） |
| [json-view-error](web/smart-routing-config/json-view-error.md) | 在 JSON 视图里把困难档模型改成在 api 分组没有渠道的模型，点「保存更改」报同样的错误 | ✅ 通过（2026-11-06） |
| [model-dropdown-and-groups](web/smart-routing-config/model-dropdown-and-groups.md) | 模型下拉里没有所有分组都没渠道的模型；分组小标签能添加、用 Alt+←/→ 调整顺序、删除 | ✅ 通过（2026-11-06） |
| [delete-with-confirm](web/smart-routing-config/delete-with-confirm.md) | 从「…」菜单删除 auto-gpt，先弹确认框，确认后这一行消失，保存更改后刷新仍为空 | ✅ 通过（2026-11-06） |
| [i18n-labels](web/smart-routing-config/i18n-labels.md) | 切到 English 和日本語，智能路由页和添加弹窗（档位 Tab、高级设置）的文字都有翻译 | ✅ 通过（2026-11-06） |

### smart-routing-editor（配置弹窗改版，二期 PRD 04）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [editor-new-defaults](web/smart-routing-editor/editor-new-defaults.md) | 新建停在「打分」、接口只选 Responses、分组只有「＋ 添加分组」；顶部只有名称和接口；左栏摘要随填写变化；高级设置在最底部默认收起 | ✅ 通过（2026-11-06） |
| [editor-tabs-keyboard-and-mobile](web/smart-routing-editor/editor-tabs-keyboard-and-mobile.md) | 左栏读作标签页，方向键移动焦点、回车切换；400px 宽（iframe 模拟）时左栏变成顶部横排 | ✅ 通过（2026-11-06） |
| [editor-info-tooltips](web/smart-routing-editor/editor-info-tooltips.md) | 弹窗里不直接显示说明文字，悬停每个 ⓘ 显示对应说明；打分、顾问 Tab 各只有三行 | ✅ 通过（2026-11-06） |
| [editor-tiers-switch](web/smart-routing-editor/editor-tiers-switch.md) | 三档切换显示档名加模型名，默认档带「默认」标签，改默认档后标签跟着移动 | ✅ 通过（2026-11-06） |
| [editor-effort-modes](web/smart-routing-editor/editor-effort-modes.md) | 思考力度不覆盖 / 固定 / 跟随打分时后面的小下拉是零个 / 一个 / 两个；最低高于最高报错；七档以外的值保存报错（二期 PRD 01 C1、C4） | ✅ 通过（2026-11-06） |
| [editor-group-tags](web/smart-routing-editor/editor-group-tags.md) | 分组小标签：一个时拖不动，拖动和 Alt+←/→ 调整顺序，满 5 个「＋」变灰，出错时描红 | ✅ 通过（2026-11-06） |
| [key-drawer-group-order](web/smart-routing-editor/key-drawer-group-order.md) | 密钥抽屉里的自动分组顺序仍是原来的样子：上移、下移、拖动把手 | ✅ 通过（2026-11-06） |
| [editor-save-errors](web/smart-routing-editor/editor-save-errors.md) | 打分模型没价格 + 普通档渠道拒收所勾格式：自动切到「打分」，红点和「1 个错误」在打分、档位、普通上；名称重名只在名称下写红字 | ✅ 通过（2026-11-06） |
| [editor-no-scroll-1100](web/smart-routing-editor/editor-no-scroll-1100.md) | 1100px 高（iframe 模拟）时 9 种状态的弹窗都不出现滚动条 | ✅ 通过（2026-11-06） |
| [routes-table-summary](web/smart-routing-editor/routes-table-summary.md) | 表格名称下写所勾接口的简称，档位第二行写力度摘要和分组顺序（不覆盖 / 固定 / 跟随打分 / 不支持思考），末尾省略 | ✅ 通过（2026-11-06） |
| [editor-i18n-seven-languages](web/smart-routing-editor/editor-i18n-seven-languages.md) | 六种非简体中文界面语言下，智能路由页、编辑弹窗各 Tab 和覆盖弹窗的新增文字都有翻译（二期 PRD 02 C11、PRD 04 C14） | ✅ 通过（2026-11-06） |

### smart-routing-capability（模型思考能力，二期 PRD 02）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [capability-table-sources](web/smart-routing-capability/capability-table-sources.md) | 能力卡片只列档位用到的模型、同一模型一行；来源、档位小标签、操作按规则显示；今年发布的 Claude / Gemini / DeepSeek 内置档位和代码一致，2025 年发布的模型显示「未确认」 | 未运行（口径变更） |
| [capability-override-flow](web/smart-routing-capability/capability-override-flow.md) | 覆盖 gpt-5.6-sol 去掉 xhigh、max 后「已覆盖」，表格标黄「超出模型能力，实际按 high 发送」；保存更改后生效；恢复内置后标黄消失 | ✅ 通过（2026-11-06） |
| [capability-override-glm-and-unsaved](web/smart-routing-capability/capability-override-glm-and-unsaved.md) | 覆盖弹窗一档不勾报错；glm-5.1 覆盖成 low、high 后力度下拉只有这两档；不保存更改就刷新，覆盖丢失 | ✅ 通过（2026-11-06） |
| [capability-empty-after-delete](web/smart-routing-capability/capability-empty-after-delete.md) | 删掉唯一的虚拟模型后，能力卡片显示「还没有虚拟模型用到的模型」 | ✅ 通过（2026-11-06） |
| [capability-editor-states](web/smart-routing-capability/capability-editor-states.md) | 档位选 gpt-5.5 / glm-5.1 / gpt-4.1 时，分别只列支持的档位、带「能力未确认」、显示不支持思考的灰字 | ✅ 通过（2026-11-06） |

### smart-routing-endpoints（多接口与保存检查，二期 PRD 03）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [endpoint-toggles-and-required](web/smart-routing-endpoints/endpoint-toggles-and-required.md) | 接口三个按钮可多选，选中样式和选中 Tab 相同，悬停显示完整路径；一个都不选时接口下面红字（二期 PRD 04 C1、PRD 03 C1） | ✅ 通过（2026-11-06） |
| [endpoint-codex-rejects](web/smart-routing-endpoints/endpoint-codex-rejects.md) | 普通档 gpt-5.5 在 subscription 里只有 Codex 渠道时勾 Anthropic 被拦下，换成有 OpenAI 渠道的分组后能保存 | ✅ 通过（2026-11-06） |
| [endpoint-chat-to-responses-policy](web/smart-routing-endpoints/endpoint-chat-to-responses-policy.md) | 保存检查算上「Chat 转 Responses」策略和透传：命中 Codex 能保存、开透传被拦、命中只有 MiniMax 的分组被拦 | ✅ 通过（2026-11-06） |
| [endpoint-mixed-metadata-saves](web/smart-routing-endpoints/endpoint-mixed-metadata-saves.md) | 只支持 Responses 和只支持 Chat 的模型放在一起，勾两种接口能保存，没有接口不一致的报错 | ✅ 通过（2026-11-06） |

### virtual-model-pickers（各处可选虚拟模型，二期 PRD 05）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [cc-switch-virtual-models](web/virtual-model-pickers/cc-switch-virtual-models.md) | 「填入 CC Switch」选 Claude、Codex 时模型下拉末尾有虚拟模型，选 Gemini 时没有 | ✅ 通过（2026-11-06） |
| [key-model-limits-virtual](web/virtual-model-pickers/key-model-limits-virtual.md) | 密钥模型限制能勾虚拟模型并保存，编辑时显示为已选标签、能点 × 移除 | ✅ 通过（2026-11-06） |
| [playground-chat-only](web/virtual-model-pickers/playground-chat-only.md) | 演练场只列勾了 Chat 的虚拟模型；给 auto-gpt 勾上 Chat 并保存更改后出现 | ✅ 通过（2026-11-06） |
| [virtual-models-not-elsewhere](web/virtual-model-pickers/virtual-models-not-elsewhere.md) | 价格页、模型定价、档位模型下拉、渠道模型配置里没有虚拟模型；首页示例用第一个真实模型 | ✅ 通过（2026-11-06） |

### usage-logs-routing（使用日志里的路由信息，一期 PRD 06；思考力度，二期 PRD 06）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [list-captions](web/usage-logs-routing/list-captions.md) | 使用日志列表里普通档、顾问、打分失败、路由打分四种小字的文字和颜色 | ✅ 通过（2026-11-06） |
| [main-detail-and-score-link](web/usage-logs-routing/main-detail-and-score-link.md) | 主请求详情里「智能路由」块在「请求转换」下面，各行顺序和内容正确（含思考力度）；点「查看这次打分」打开打分记录详情 | ✅ 通过（2026-11-06） |
| [detail-reasons](web/usage-logs-routing/detail-reasons.md) | 降档被拦、交给顾问、打分超时、工具结果续跑四条主请求详情里的思考力度和选档原因；续跑没有打分标签和打分记录 | ✅ 通过（2026-11-06） |
| [scorer-detail-back-to-main](web/usage-logs-routing/scorer-detail-back-to-main.md) | 打分记录详情显示类型、虚拟模型、属于请求（没有思考力度）；点「属于请求」打开主请求详情 | ✅ 通过（2026-11-06） |
| [user-view-and-plain-log](web/usage-logs-routing/user-view-and-plain-log.md) | alice 看自己日志里的路由信息（含思考力度）与管理员一致；看不到智能路由设置；没有经过智能路由的日志没有「智能路由」块 | ✅ 通过（2026-11-06） |
| [effort-eight-kinds](web/usage-logs-routing/effort-eight-kinds.md) | 八种思考力度来源在详情「档位」下面各写成约定的样子 | ✅ 通过（2026-11-06） |
| [effort-continuation-scorer-legacy](web/usage-logs-routing/effort-continuation-scorer-legacy.md) | 工具结果续跑写沿用的力度；打分记录和一期旧日志没有「思考力度」一行 | ✅ 通过（2026-11-06） |
| [effort-i18n-seven-languages](web/usage-logs-routing/effort-i18n-seven-languages.md) | 七种界面语言下，日志详情「思考力度」的标签和六种写法都有翻译 | ✅ 通过（2026-11-06） |
