---
env:
  up: docker compose -f docker-compose.dev.yml up -d --build && (cd web && bun run dev -- --host 0.0.0.0 --port 5173)
  down: docker compose -f docker-compose.dev.yml down -v && 停掉 5173 端口的前端开发服务器
  frontends:
    web: 5173
  seed: .bb-spec/docs/test/webview/fixtures/smart-routing-seed.sh
---

# 网页交互用例索引

运行前先执行 `fixtures/smart-routing-seed.sh` 灌数据（在仓库根执行 `.bb-spec/docs/test/webview/fixtures/smart-routing-seed.sh`，可选参数是后端地址，默认 http://localhost:3000）。脚本会建好 root / alice 两个账号、分组、渠道、模型价格和 9 条使用日志，并把智能路由配置置成空数组。日志的时间是执行脚本前 1~6 分钟，使用日志页默认只看当天，所以要在当天执行脚本后再跑用例。

登录和刷新会话共用每个 IP 20 分钟 20 次的限额，每次整页加载都会刷新一次会话，一个用例大约用掉 3~5 次。脚本最后会清零这个计数；一轮跑不完时，先再执行一次脚本，或在后端环境变量里设 `CRITICAL_RATE_LIMIT_ENABLE=false`。

## web（http://localhost:5173）

### smart-routing-config（智能路由配置页，PRD 05）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [menu-entry](web/smart-routing-config/menu-entry.md) | 管理员在请求策略菜单里看到「智能路由」，位于「渠道健康」之后，点进去打开智能路由页 | ✅ 通过（2026-11-05） |
| [empty-state](web/smart-routing-config/empty-state.md) | 没有虚拟模型时显示空状态的标题、说明和按钮，点按钮打开添加弹窗 | ✅ 通过（2026-11-05） |
| [add-dialog-defaults](web/smart-routing-config/add-dialog-defaults.md) | 添加弹窗里思考力度、默认档、超时、连续不满意次数和高级设置三个数字都是默认值 | ✅ 通过（2026-11-05） |
| [add-and-save](web/smart-routing-config/add-and-save.md) | 在添加弹窗里配好 auto-gpt，弹窗保存后点「保存更改」，表格出现这一行且刷新后仍在 | ✅ 通过（2026-11-05） |
| [save-without-changes](web/smart-routing-config/save-without-changes.md) | 已有 auto-gpt、没做任何修改时点「保存更改」，提示没有需要保存的更改 | ✅ 通过（2026-11-05） |
| [unsaved-refresh](web/smart-routing-config/unsaved-refresh.md) | 编辑 auto-gpt 并在弹窗里保存，但不点「保存更改」直接刷新，改动不生效 | ✅ 通过（2026-11-05） |
| [edit-save-error](web/smart-routing-config/edit-save-error.md) | 困难档换成在 api 分组没有渠道的模型，保存时字段下出现红字、弹窗不关；改回原模型后保存成功 | ✅ 通过（2026-11-05） |
| [json-view-error](web/smart-routing-config/json-view-error.md) | 在 JSON 视图里把困难档模型改成在 api 分组没有渠道的模型，点「保存更改」报同样的错误 | ✅ 通过（2026-11-05） |
| [model-dropdown-and-groups](web/smart-routing-config/model-dropdown-and-groups.md) | 模型下拉里没有所有分组都没渠道的模型；分组顺序能添加、上移、下移、删除 | ✅ 通过（2026-11-05） |
| [delete-with-confirm](web/smart-routing-config/delete-with-confirm.md) | 从「…」菜单删除 auto-gpt，先弹确认框，确认后这一行消失，保存更改后刷新仍为空 | ✅ 通过（2026-11-05） |
| [i18n-labels](web/smart-routing-config/i18n-labels.md) | 切到 English 和日本語，智能路由页和添加弹窗的新增文字都有翻译 | ✅ 通过（2026-11-05） |

### usage-logs-routing（使用日志里的路由信息，PRD 06）

| 用例 id | 说明 | last-run 状态 |
| --- | --- | --- |
| [list-captions](web/usage-logs-routing/list-captions.md) | 使用日志列表里普通档、顾问、打分失败、路由打分四种小字的文字和颜色 | ✅ 通过（2026-11-05） |
| [main-detail-and-score-link](web/usage-logs-routing/main-detail-and-score-link.md) | 主请求详情里「智能路由」块在「请求转换」下面，各行顺序和内容正确；点「查看这次打分」打开打分记录详情 | ✅ 通过（2026-11-05） |
| [detail-reasons](web/usage-logs-routing/detail-reasons.md) | 降档被拦、交给顾问、打分超时、工具结果续跑四条主请求详情里的选档原因；续跑没有打分标签和打分记录 | ✅ 通过（2026-11-05） |
| [scorer-detail-back-to-main](web/usage-logs-routing/scorer-detail-back-to-main.md) | 打分记录详情显示类型、虚拟模型、属于请求；点「属于请求」打开主请求详情 | ✅ 通过（2026-11-05） |
| [user-view-and-plain-log](web/usage-logs-routing/user-view-and-plain-log.md) | alice 看自己日志里的路由信息与管理员一致；看不到智能路由设置；没有经过智能路由的日志没有「智能路由」块 | ✅ 通过（2026-11-05） |
