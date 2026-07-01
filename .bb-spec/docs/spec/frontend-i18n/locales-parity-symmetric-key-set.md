---
name: locales-parity-symmetric-key-set
description: 所有支持语言的 locale key 集合完全对称；由同步脚本作提交前门槛。
---

# locale key 集合对称

## 目的

避免不同语言渲染时因 key 缺失回退到英文源串导致 UI 混语；把 key 增减集中在一次同步动作里，不散落到每次改文案。

## 逻辑

以 base locale（源语言，通常是英文）为主档。一次同步动作把 base 中新增的 key 补到所有其他 locale（未翻译则占位为英文源串），并把其他 locale 中 base 已删除的 key 一并清理。同步产出结构化报告，记录每个 locale 的 `missingCount` / `extrasCount` / `untranslatedCount`。

仓库提交时任一 locale 的 `missing` / `extras` 均应为 0；`untranslated` 提示后续翻译工作。

## 约束

- 所有 locale 的 key 集合与 base locale 严格相等（对称差为 ∅）。
- 同步脚本以 `npm` / `bun` script 形式可复现执行（例如 `bun run i18n:sync`）。
- 同步报告持久化到仓库（供 review 与增量对比）。
- 提交前门槛：报告中所有 locale 的 `missingCount == 0` 且 `extrasCount == 0`。

## 例子

- 输入：开发者在源码里新增 `t('New Setting')`，但只手工编辑了 en 与 zh。
- 过程：运行同步命令——扫描 base locale 与其他 locale 的对称差；把缺失 key 以英文源串占位补齐；把多余 key 移除；写出结构化同步报告。
- 预期结果：所有非 base locale 都新增 key `New Setting` 占位；报告标出 fr/ru/ja/vi 的 `untranslatedCount` 增加提醒后续翻译；对称差为 0。

## 验收

- [ ] 对任一 locale 与 base locale 做 key 集合对称差，结果为空。
- [ ] 同步脚本可在开发环境重复执行且幂等。
- [ ] 同步报告在仓库中可查阅，字段包含 `missingCount` / `extrasCount` / `untranslatedCount`。
- [ ] 提交流程中通过 CI 或本地 hook 校验 `missing/extras` 均为 0，否则阻断。
