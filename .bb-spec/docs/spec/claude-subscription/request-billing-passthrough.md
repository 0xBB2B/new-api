---
name: request-billing-passthrough
description: 上游 /v1/messages,复用标准 Claude 请求转换与 token 计费;无固定订阅倍率;不实现 usage 用量查询。
---

# 请求转换、计费与非目标

## 目的
明确 Claude 订阅渠道复用标准 Claude 管道处理请求与计费,并划定「不做用量查询」的边界。

## 逻辑
- 上游端点为 `/v1/messages`,请求/响应体沿用标准 Claude 消息格式。
- 请求转换复用标准 claude 渠道逻辑:OpenAI Chat 入参转 Claude 格式、Claude 原生入参直通;流式与非流式响应转换一致。
- 计费复用标准 claude 的 token 计量:从上游响应 usage 读取 input/output tokens 及 cache(创建/读取)tokens,走通用计费链路。
- 无固定订阅折扣/倍率:价格由管理员在模型倍率设置中为对应 claude 模型配置,与标准 claude 渠道一致。
- 非目标:不实现订阅用量(额度/速率窗口)查询接口——Anthropic 无公开的订阅用量 REST 端点,故不做 Codex 式 usage 弹窗与相关 controller/service/路由。

## 约束
- 上游请求打到 `{baseURL}/v1/messages`。
- 计费 token 来自上游响应 usage(含 cache tokens),不使用固定订阅倍率。
- 不提供 usage/用量查询端点或前端用量弹窗。

## 例子
- 客户端发一次非流式对话 → 转成 Claude `/v1/messages` 请求,上游返回 `usage.input_tokens=1200, output_tokens=300` → 按管理员配置的该模型倍率计费 1200+300 tokens。
- 管理员想查该订阅本月剩余额度 → 系统不提供此功能(非目标),需自行到 Claude 官方渠道查看。

## 验收
- [ ] 上游请求路径为 `/v1/messages`,请求/响应转换与标准 claude 一致。
- [ ] 计费按上游 usage 的 input/output/cache tokens 计量,无固定订阅倍率。
- [ ] 系统不暴露该渠道的订阅用量查询端点或前端用量弹窗。
