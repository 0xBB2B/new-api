---
name: translation-access-hook-first-runtime-fallback
description: 组件用 useTranslation() 的 t；非 React 上下文才用 i18n 单例的 t；禁 <Trans>。
---

# 翻译访问入口分工

## 目的

让组件享受 React 语言切换重渲染；同时给请求拦截器、纯 TS 工具等无 React 上下文的模块留出直接翻译错误文案的路径；保持翻译入口单一，避免 `t()` 与 `<Trans>` 双入口。

## 逻辑

React 组件（`.tsx`）内所有翻译调用统一来自 `useTranslation()` 返回的 `t`。不在组件内直接 import i18n 单例调用 `i18n.t()`。

非 React 模块（`.ts` 工具、hook 内部的异步回调、请求拦截器等）无法调用 React hook，允许直接调用 i18n 单例的 `t()`。

不使用 `<Trans>` 之类的插值组件（本项目组织选择，避免 `t()` / `<Trans>` 双入口）。同一份 key 无论走哪个入口都取自同一份 locale 资源。

## 约束

- 组件文件（`.tsx`）中出现的翻译调用均通过 `useTranslation()` 返回的 `t`。
- 组件文件内禁直接 `import` i18n 单例并调用 `i18n.t()`。
- 非组件文件（`.ts`）允许 i18n 单例 `t()` 调用，但不允许调用 React hook。
- 项目内 `<Trans>` 使用数为 0（保持单入口）。

## 例子

- 输入：① 一个 React 组件要显示按钮文本；② 一个 axios 响应拦截器要在 401 时显示 `Session expired!` 的 toast。
- 过程：① 组件顶端 `const { t } = useTranslation()`；JSX 里 `{t('Save')}`。② 拦截器文件 `import` i18n 单例；回调里 `toast.error(i18n.t('Session expired!'))`。
- 预期结果：① 用户切换语言时组件因 hook 订阅自动重渲染；② 拦截器每次触发时按当前语言取译文，不需要 React 上下文。

## 验收

- [ ] `.tsx` 文件里 grep `i18n\.t\(` / `i18next\.t\(` 命中数为 0。
- [ ] `.tsx` 文件里 grep `useTranslation\(\)` 覆盖所有含翻译文本的组件。
- [ ] 全仓 `.tsx` 中 `<Trans>` 使用数为 0。
- [ ] `.ts` 文件的翻译调用不引入 React hook。
