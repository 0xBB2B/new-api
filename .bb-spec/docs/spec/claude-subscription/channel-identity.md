---
name: channel-identity
description: Claude 订阅是独立渠道类型,默认 base URL api.anthropic.com,由专属 OAuth adaptor 分派,与标准 claude 渠道并存。
---

# Claude 订阅渠道身份与注册

## 目的
把「用 Claude Max/Pro 订阅 OAuth 凭据作上游」定义为一个独立渠道类型,与标准 claude(x-api-key)渠道并存互不影响。

## 逻辑
- 新增一个专属渠道类型常量,显示名固定为 `Claude Subscription`,默认 base URL 为 `https://api.anthropic.com`。
- 该类型映射到一个专属 OAuth adaptor(而非复用标准 claude adaptor 的鉴权路径);relay 分派链根据该类型选中此 adaptor。
- 该 adaptor 支持流式,需登记进流式支持白名单。
- 标准 claude 渠道类型与行为不被改动:两类渠道在类型枚举、adaptor 选择、鉴权方式上完全区分。

## 约束
- 存在一个与标准 claude 不同值的渠道类型;其显示名等于 `Claude Subscription`。
- 该类型未配置 base URL 时,上游请求打到 `https://api.anthropic.com`。
- 该类型选中的 adaptor 走 OAuth Bearer 鉴权,不是 x-api-key 鉴权。
- 该类型在流式白名单中登记为支持流式。

## 例子
- 管理员新建渠道,类型选 `Claude Subscription`,base URL 留空 → 该渠道发起的对话请求上游地址前缀为 `https://api.anthropic.com`,且经由 OAuth adaptor 构造请求头。
- 同一部署里另有一个标准 `Claude` 渠道(填 x-api-key)→ 两者互不干扰,各走各的鉴权。

## 验收
- [ ] 渠道类型枚举含一个显示名为 `Claude Subscription` 的新类型,值不同于标准 claude。
- [ ] 该类型未填 base URL 时上游默认 `https://api.anthropic.com`。
- [ ] 该类型的请求由专属 OAuth adaptor 处理,标准 claude 渠道行为不变。
- [ ] 该类型可发起流式请求且被正常处理。
