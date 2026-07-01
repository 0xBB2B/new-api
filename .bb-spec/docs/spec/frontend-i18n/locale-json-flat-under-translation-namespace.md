---
name: locale-json-flat-under-translation-namespace
description: locale 文件顶层为单一 namespace，其内为扁平 key → string；禁多级嵌套。
---

# locale JSON 扁平结构

## 目的

让 key 与源码里 `t()` 的字面参数一一对应、可 grep 可 diff；避免嵌套 id 带来的路径分叉与文件形态漂移。

## 逻辑

每份 locale JSON 只有一个顶层字段（默认 namespace，例如 `translation`）。该 namespace 内所有值均为字符串（可含 `{{var}}` 插值），禁 object / array / 嵌套 namespace。

base 与所有非 base locale 结构完全同构。新增 key 时不引入嵌套分组，仅追加到扁平表末尾。

## 约束

- 每份 locale 文件顶层字段数 == 1，且为默认 namespace 名。
- namespace 内每一个 value 的类型均为 `string`（非 object / array）。
- 所有 locale 文件行数保持一致（作为扁平结构对齐的粗校验信号）。

## 例子

- 输入：新增 6 种语言的 `Delete` 按钮文案。
- 过程：在每份 locale 的 `translation` 对象里追加同一个 key `Delete`，值为对应语言的字符串；不新建 `buttons` 或 `common` 之类的嵌套分组。
- 预期结果：任何 locale 用 `json.load(...)["translation"]["Delete"]` 都能取到字符串；顶层结构与 key 集合在所有 locale 间保持一致。

## 验收

- [ ] 每份 locale 顶层 keys 数 == 1，值为默认 namespace。
- [ ] 该 namespace 下所有 value 的 JSON 类型为 `string`（用 `jq 'to_entries | map(.value|type) | unique'` 验证仅得 `["string"]`）。
- [ ] 所有 locale 文件行数一致。
- [ ] 新增 key 时的 diff：所有 locale 均新增同名 key 且值均为 string。
