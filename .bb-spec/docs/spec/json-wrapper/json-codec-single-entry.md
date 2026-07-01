---
name: json-codec-single-entry
description: 项目提供 common/json.go 作为 JSON 编解码包装入口；pkg 内 codec 层同属合法；业务代码两种写法并存为现状。
---

# JSON 编解码统一入口（现状描述）

## 目的

在项目里保留一个 JSON 编解码的统一切入点，为未来加日志、脱敏、统计、追踪、替换实现留下可修改点；同时如实描述本项目当前对该切入点的一致性状况。

## 逻辑

项目在 `common/json.go` 提供一层薄薄的 JSON 编解码入口，覆盖三种输入形态与序列化：

- 字节切片入口 `common.Marshal(v)` / `common.Unmarshal(data, v)`
- 字符串入口 `common.UnmarshalJsonStr(s, v)`
- 流入口 `common.DecodeJson(reader, v)`

此外，`pkg/ionet/jsonutil` 与 `pkg/cachex/codec` 属于**专用 codec 层**（JSON 编解码工具本身），功能上和 `common/json.go` 属同类：它们本身即包装层，不算「业务代码直调标准库」。

**类型引用不受此规则约束**：`json.RawMessage` / `json.Number` 作为字段类型、struct 上的 `json:"..."` tag，以及类型自身实现的 `MarshalJSON` / `UnmarshalJSON` 方法体内直接调用标准库均属合法用法。

**现状承诺**：本项目业务代码中「走 `common.*` 包装」与「直调 `encoding/json`」两种写法并存（当前统计约 490 处走包装、236 处直调）。新写或大改的代码推荐走 `common.*` 以享受统一切入点的价值；既有代码的直调不视为「违规」，属项目实况的一部分。

## 约束

- 项目必须提供覆盖字节、字符串、`io.Reader` 三类反序列化与至少一类序列化的 JSON 包装入口。
- 合法的 JSON codec 路径包括 `common/json.go`、`pkg/ionet/jsonutil`、`pkg/cachex/codec` 等专用 codec/工具层，以及类型自身的 `MarshalJSON` / `UnmarshalJSON` 方法体。
- 类型引用（`json.RawMessage` / `json.Number` / `json:"..."` tag）不受编解码入口约束。
- 新写或大改的业务代码推荐调用 `common.*` 包装函数；既有业务代码直调 `encoding/json` 属现状，不列为「违规」。

## 例子

- 输入：业务代码需要把 HTTP 响应体反序列化为一个 struct。
- 过程：新写代码里推荐 `common.DecodeJson(resp.Body, &out)`；接触既有 `json.NewDecoder(resp.Body).Decode(&out)` 时可顺手改为 `common.*`，也可保留原写法。
- 预期结果：新代码经统一入口获得未来横切能力（日志、脱敏、追踪）的一次性收益；既有代码的两种写法均能正确解码。

## 验收

- [ ] 包装包提供了字节、字符串、`io.Reader` 三类反序列化入口与至少一个序列化入口。
- [ ] `pkg/ionet/jsonutil` 与 `pkg/cachex/codec` 的直调标准库不被视为漂移。
- [ ] 类型自身 `MarshalJSON` / `UnmarshalJSON` 方法体内直调标准库不被视为漂移。
- [ ] 新代码若声明「统一入口带来的新增能力」（例如日志格式），能通过 `common.*` 全量生效。
