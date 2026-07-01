---
name: adaptor-interface-total-method-set
description: 每个 provider adapter 实现同一套完整接口方法集；不支持的模态返回 not implemented，不裁剪接口。
---

# Adapter 接口方法集封闭全集

## 目的

让 relay handler 无需类型断言即可以统一接口驱动多家 provider，模态差异用返回值表达而非接口分裂。

## 逻辑

存在一个统一 `Adaptor` interface，聚合所有模态方法：初始化、URL 组装、请求头装配、按 relay 模式的多种请求转换（文本/嵌入/音频/图像/重排/责任响应/跨家族格式）、请求执行、响应处理、模型清单、渠道名。

每个 provider adapter 类型必须实现该接口的**全部方法**。若 provider 天生不支持某模态，方法体保留但返回 `not implemented` 错误（或返回 nil 值 + 错误），禁止靠不实现方法或裁剪接口来表达能力差异。

handler 层根据 relay 模式调用对应方法。能力判定由方法返回值承载，不由接口存在性承载。

## 约束

- 接口方法集是封闭全集，禁为单个 provider 新增只它用的方法。
- provider 缺失能力用 error 返回值表达，禁止 panic 作为常态路径。
- 接口方法命名按模态划分（一模态一方法），禁用单一 `Convert` 靠参数分派。
- 新增 adapter 类型必须实现接口的全部方法，编译期强制。

## 例子

- 输入：新增一个只支持文本对话的上游 X。
- 过程：为 X 定义 Adaptor 类型，实现全部接口方法；文本相关方法真实转换，音频/图像/嵌入/重排等方法体返回 `errors.New("not implemented")`。
- 预期结果：handler 分派到 X 时走文本路径正常工作；误路由到音频/图像等模态时立即得到明确错误，handler 无需额外做 provider 能力探测。

## 验收

- [ ] 全仓所有 provider adapter 类型都实现同一 `Adaptor` 接口的完整方法集。
- [ ] 不支持的模态返回 `not implemented` 或等价错误，禁 panic。
- [ ] 接口方法命名按模态划分（例如 `ConvertOpenAIRequest` / `ConvertEmbeddingRequest` 分开）。
- [ ] 新增 provider 只需实现接口方法，handler 层零改动。
