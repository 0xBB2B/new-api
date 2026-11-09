---
name: 03-webview-cases
description: 改写依赖「保存更改」的智能路由网页交互用例，新增 JSON 保存与放弃确认用例
---
# 网页交互用例跟上立即保存

## 目标
`.bb-spec/docs/test/webview/` 下的智能路由用例按"确认后立即保存、JSON 框下方保存"的新行为可以直接运行，不再引用已经去掉的「保存更改」按钮。

## 业务规则（来源：PRD 01-visual-instant-save、02-json-save）
- 智能路由页标题右边没有「保存更改」。添加、编辑弹窗点「保存」，删除确认框点「删除」，覆盖弹窗点「保存」，点「恢复内置」，这些操作都立即提交，成功后顶部提示「保存成功」，刷新后仍是新值。
- 删除确认框说明是"确定删除 <名称> 吗？确认后立即生效。"。覆盖弹窗说明是"勾选这个模型支持的思考力度，保存后立即生效。"。
- JSON 视图：编辑框下方靠右有「保存」，内容和已保存的一样时变灰；改了以后能点；检查不通过时弹出原因，什么都不提交。
- JSON 有改动时点「可视化」，弹出「放弃未保存的修改？」，按钮「取消」「放弃修改」；没改动时直接切换。
- 密钥抽屉里的「保存更改」（`web/virtual-model-pickers/key-model-limits-virtual.md` 第 118 行）属于另一个页面，不改。

## 涉及文件
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-config/add-and-save.md`：去掉点「保存更改」的步骤；弹窗保存后等「保存成功」，刷新后这一行仍在；更新简介、测试目的和验收。
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-config/delete-with-confirm.md`：确认框文字换成新说明；去掉「保存更改」；确认后等「保存成功」，刷新后仍是空状态。
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-config/json-view-error.md`：改成点 JSON 框下方的「保存」。
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-config/menu-entry.md`：断言页面有标题「智能路由」，并且没有名为「保存更改」的按钮。
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-config/i18n-labels.md`：检查的中文词表里去掉「保存更改」，其他语言词表里对应的词也去掉。
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-editor/editor-effort-modes.md`、`routes-table-summary.md`、`.bb-spec/docs/test/webview/web/virtual-model-pickers/playground-chat-only.md`：去掉点「保存更改」的步骤，改成等弹窗保存后的「保存成功」；同步改简介和验收里的文字。
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-capability/capability-override-flow.md`：覆盖弹窗说明换成新文字；覆盖保存和恢复内置后等「保存成功」，去掉「保存更改」；刷新后核对。
- 删除 `.bb-spec/docs/test/webview/web/smart-routing-config/save-without-changes.md`、`unsaved-refresh.md`：它们验证的"没改动时点保存更改""不点保存更改就丢失"已经不存在。
- 修改 `.bb-spec/docs/test/webview/web/smart-routing-capability/capability-override-glm-and-unsaved.md`：改名为 `capability-override-glm.md`（frontmatter 的 name 和 JSON 里的 id 一起改），去掉"不点保存更改就刷新、覆盖丢失"那段，改成覆盖保存后刷新，glm-5.1 仍是「已覆盖」low、high。
- 新建 `.bb-spec/docs/test/webview/web/smart-routing-config/instant-save-refresh.md`：编辑 auto-gpt 的顾问 Tab，把连续不满意次数从 2 改成 3，弹窗保存后出现「保存成功」，刷新后顾问列是「连续 3 次不满意」。
- 新建 `.bb-spec/docs/test/webview/web/smart-routing-config/json-save-and-discard.md`：覆盖 PRD 02 的 C1、C2、C4、C6、C7。切到 JSON，「保存」变灰；点「格式化 JSON」后仍是灰的；把困难档改成 gpt-5.5，「保存」能点；点「可视化」弹出确认框，点「取消」留在 JSON；点「保存」后出现「保存成功」、按钮变灰；切回可视化不弹框，困难档是 gpt-5.5；刷新后不变。
- 修改 `.bb-spec/docs/test/webview/INDEX.md`：表格里同步上面每个用例的文件名、描述；改写过和新建的用例状态写「未运行（2026-11-09 按立即保存改写）」；删掉的两行去掉。

## 函数清单
不涉及代码。每份用例沿用同目录现有用例的格式：frontmatter（name、description、category）、简介、测试目的、「测试流程（固定 JSON，subagent 原样消费）」代码块（id、category、target、setup、steps、teardown）、如何验收。新用例的 setup 和 teardown 照抄 `delete-with-confirm.md`：登录 root、用接口写入 auto-gpt、结束时清空配置。

## 协作关系
- 用例由 `/test-webview` 交给 `webview-test-runner` 子代理执行，环境、灌数据方式写在 `INDEX.md` 头部，不改。
- 用例里的按钮和文字以 plan 01、02 实现后的中文界面为准：「保存」「正在保存...」「保存成功」「放弃未保存的修改？」「放弃修改」。

## 验证方式
- 测试入口：在 worktree 根目录执行下面的命令。
- 测试输入：本 plan 改动后的 `.bb-spec/docs/test/webview/` 目录。
- 预期结果：
- [ ] `grep -rn "保存更改" .bb-spec/docs/test/webview --include=*.md` 只剩 `key-model-limits-virtual.md` 里密钥抽屉的那一处。
- [ ] 每份改过或新建的用例里，「测试流程」代码块都能被 JSON 解析器读出来，`id` 和文件名一致：`python3 -c` 逐个读取 ```json 代码块并 `json.loads`，没有报错。
- [ ] `INDEX.md` 里每一行链接的文件都存在，`web/` 下每份用例在 `INDEX.md` 里都有一行。
- [ ] plan 01、02 实现后，在本地环境用 `/test-webview` 跑改写过和新建的用例，全部通过（由 `/exec` 收尾时执行，或者如实报告没跑成的原因）。
