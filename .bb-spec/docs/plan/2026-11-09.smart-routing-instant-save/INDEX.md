# smart-routing-instant-save 实施计划

## 阶段 1：前端行为
- [01-instant-save-visual](01-instant-save-visual.md) — 去掉右上角「保存更改」，可视化操作确认后立即提交
- [02-json-save](02-json-save.md) — JSON 框下方「保存」按钮与切回可视化的放弃确认 [依赖: 01-instant-save-visual]

## 阶段 2：网页交互用例
- [03-webview-cases](03-webview-cases.md) — 改写依赖「保存更改」的用例，新增 JSON 保存用例 [依赖: 02-json-save]
