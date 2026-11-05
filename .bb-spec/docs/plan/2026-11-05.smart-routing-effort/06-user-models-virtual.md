---
name: 06-user-models-virtual
description: 后台用户模型列表接口在末尾补上虚拟模型，可按请求格式只留接收该格式的虚拟模型
---
# 用户模型列表补上虚拟模型

## 目标
`/api/user/models` 返回的列表末尾带上虚拟模型名，后台各处的模型下拉因此都能选到；需要按格式过滤的页面（演练场、CC Switch 的 Gemini）可以只拿到接收该格式的虚拟模型。

## 业务规则（来源：PRD）
- 05-virtual-model-pickers C1：`/api/user/models`（带或不带 `group` 参数）返回列表末尾加上全部已配置的虚拟模型名，排在真实模型后面；不按用户分组过滤虚拟模型，和 `/v1/models` 一致。
- C2：CC Switch 选 Gemini 应用时不列虚拟模型；选 Claude、Codex 时列出全部。
- C4：演练场只列勾了 Chat Completions 的虚拟模型。
- 两条按格式过滤的规则统一成一个可选参数：带 `endpoint=<格式>` 时，只追加 `endpoints` 里含这个格式的虚拟模型；不带时追加全部。Gemini 格式没有虚拟模型会勾，所以 `endpoint=gemini` 时一个虚拟模型都不追加。
- 虚拟模型名已经和真实模型去重（保存时禁止重名），列表里不会重复。

## 涉及文件
- 修改 `controller/user.go`
- 测试：扩展 `controller/model_list_test.go`（或 `controller/user` 下现有的模型列表测试，取已有的那个）

## 成品定义
接口契约：

```
GET /api/user/models                       → data: [<真实模型...>, "auto-gpt", "auto-chat"]
GET /api/user/models?group=subscription    → data: [<该分组的真实模型...>, "auto-gpt", "auto-chat"]
GET /api/user/models?endpoint=openai       → data: [<真实模型...>, "auto-chat"]     # 只追加勾了 Chat 的
GET /api/user/models?endpoint=gemini       → data: [<真实模型...>]                  # 不追加虚拟模型
endpoint 可取值：openai-response、openai、anthropic、gemini；其他值按不追加虚拟模型处理
```

## 函数清单
### controller/user.go
| 函数名 | 职责 |
|---|---|
| `GetUserModels` | 取完真实模型后，按 `endpoint` 参数追加虚拟模型名 |

## 协作关系
读 `operation_setting.GetSmartRoutingSetting().Routes` 和 01 新增的 `Endpoints` 字段。前端在 11 里按页面传参。

## 验证方式
- 测试入口：`GET /api/user/models`
- 测试输入：配置 auto-gpt（endpoints Responses、Anthropic）和 auto-chat（endpoints Chat）；用户有若干真实模型
- 预期结果：
  - [ ] 不带参数 → 末尾依次是 auto-gpt、auto-chat，前面的真实模型和改动前完全一样
  - [ ] `group=subscription` → 末尾同样有两个虚拟模型
  - [ ] `endpoint=openai` → 只追加 auto-chat；`endpoint=anthropic` → 只追加 auto-gpt；`endpoint=gemini` → 不追加
  - [ ] 没有配置虚拟模型 → 结果和改动前完全一样
