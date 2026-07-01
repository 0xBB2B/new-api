---
name: translation-key-is-english-source-string
description: 翻译 key 直接用完整英文源字符串；禁 dot-namespace id（如 `page.login.title`）。
---

# 翻译 key 用英文源字符串

## 目的

让代码里 `t('English text')` 自身可读；缺失翻译时以英文源文本自然回退；无需查字典即可理解 UI 文案。

## 逻辑

选定英文为源语言与 fallback。所有可翻译文案在源码里以完整的英文自然语言字符串作为 key 传给 `t()`。base 语言（en）的 locale 文件里 key 与 value 恒等（identity 映射），非 base 语言用相同 key 映射到本地化 value。

不使用 `page.login.title` 这类语义 id、也不使用 keyPrefix / namespace 层级。框架层显式关闭 namespace 分隔符（例如 `nsSeparator: false`）以允许 key 中出现冒号等标点。

## 约束

- base 语言 locale 中 key 与 value 恒等（identity ratio 接近 100%，允许极少数品牌/占位差异）。
- `fallbackLng` 指向英文源语言。
- `t()` 调用参数为完整英文文案；不出现形如 `ns.sub.key` 的层级 id。
- i18n 框架初始化关闭 namespace 分隔符（`nsSeparator: false` 或等价配置），允许 key 内含标点。

## 例子

- 输入：组件需要显示按钮 `Sign In` 与错误提示 `Session expired!`。
- 过程：源码写 `t('Sign In')` / `t('Session expired!')`；base locale 里两条 key 的 value 与 key 相同；其他 locale 里 key 保持英文源串、value 是本地化译文。
- 预期结果：英文构建时 `t()` 直接回落到 key 本身；非英文构建时按 key 精确查到译文；缺失译文时也退化为可读英文。

## 验收

- [ ] base locale 中 `key == value` 的比例 ≥ 98%。
- [ ] i18n 配置 `fallbackLng` 为英文源语言、`nsSeparator` 为 `false`（或等价）。
- [ ] 全仓 `t()` 调用中 key 匹配 `\w+\.\w+\.\w+` 层级 id 模式的命中数为 0。
- [ ] 缺译回退到英文时页面无「键名字符串」形态泄漏（无 `page.login.title` 露出）。
