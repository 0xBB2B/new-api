---
name: claude-code-system-prompt
description: 强制注入 "You are Claude Code..." 作为第一条 system;管理员自定义 SystemPrompt 前置拼接在其之前。
---

# Claude Code 身份 system 注入

## 目的
满足 OAuth 鉴权的硬要求:请求首条 system 必须是 Claude Code 身份串,否则上游返回 401/403。

## 逻辑
- 转换发往上游的请求时,始终确保 system 序列的第一条为身份串:`You are Claude Code, Anthropic's official CLI for Claude.`
- 处理三种入参情况:
  - 客户端未带 system → 以身份串作为唯一/首条 system。
  - 客户端带了 system → 把身份串置于客户端 system 之前(身份串在最前,客户端内容随后)。
  - 已存在身份串(客户端恰好也传了同串)→ 不重复叠加。
- 管理员在渠道设置里配置了自定义 SystemPrompt → 顺序为:身份串 → 自定义 SystemPrompt → 客户端 system。身份串永远在最前。

## 约束
- 发往上游的请求,system 序列首条恒等于 `You are Claude Code, Anthropic's official CLI for Claude.`。
- 身份串在任何情况下都不被客户端或自定义内容顶替到非首位。
- 身份串不因客户端重复传入而叠加两次。

## 例子
- 客户端请求 system=`"你是翻译助手"` → 上游 system 首条为 `You are Claude Code, Anthropic's official CLI for Claude.`,其后接 `"你是翻译助手"`。
- 客户端未带 system,渠道自定义 SystemPrompt=`"输出简体中文"` → 上游 system 顺序:身份串,然后 `"输出简体中文"`。

## 验收
- [ ] 客户端无 system 时,上游 system 首条为身份串。
- [ ] 客户端有 system 时,身份串被置于其前,客户端内容保留。
- [ ] 配了自定义 SystemPrompt 时顺序为「身份串 → 自定义 → 客户端 system」。
- [ ] 客户端已含身份串时不产生重复身份串。
