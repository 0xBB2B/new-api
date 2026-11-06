---
name: 03-endpoint-formats
description: 虚拟模型的接口改成管理员多选 Responses、Chat、Anthropic，新增 Anthropic 格式请求的路由，保存时拦下渠道拒收的格式
date: 2026-11-05
---

# 多接口与 Anthropic 格式

## 用户故事
### P0：作为用 Claude Code 的员工，我想用虚拟模型，以便和用 Codex 的同事一样按难度自动选模型

**用例（主路径）**
- 场景：auto-claude 接口勾了 Anthropic，三档分别是 Claude 订阅渠道里的 haiku、sonnet、opus。
- 操作：员工在 Claude Code 里把模型填成 auto-claude，发「这个函数在哪个文件里」。
- 预期：回答来自简单档的模型；使用日志里这条记录的模型是简单档的真实模型，下面小字写「auto-claude · 简单档」，另有一条打分记录。

**用例（边界 / 异常）**
- 场景：auto-gpt 只勾了 Responses。
- 操作：用 Anthropic 格式（`POST /v1/messages`）请求 auto-gpt。
- 预期：返回 400，错误信息「虚拟模型 auto-gpt 只支持这些接口：Responses」；不打分、不扣费。

### P0：作为管理员，我想在保存时就知道某档的渠道接收不了所勾的格式，以便不把必然失败的配置发到线上

**用例（主路径）**
- 场景：auto-gpt 普通档 gpt-5.5，分组顺序 subscription → api；subscription 分组里 gpt-5.5 只有 Codex 渠道。
- 操作：接口勾上 Anthropic，点弹窗里的「保存」。
- 预期：弹窗不关，左栏「档位」和「普通」带红点，普通档分组那一行描红并写「gpt-5.5 在分组 subscription 里只有 Codex 渠道，接收不了 Anthropic 格式」。

## 约束
- C1：虚拟模型的接口由管理员多选，可选三项：Responses（`/v1/responses`，Responses 压缩请求 `/v1/responses/compact` 跟着它）、Chat Completions（`/v1/chat/completions`）、Anthropic（`/v1/messages`）。至少选一项。新建虚拟模型默认只选 Responses。
- C2：一期已保存的虚拟模型，接口按一期自动判断出的结果迁移，迁移后不需要管理员重新保存。升级时判断不出接口（例如渠道都停用、没有价格）就先不写，下次启动再判断；没写接口期间，请求和整页保存都按模型的接口信息现场判断。
- C3：客户端用的格式在所勾接口里 → 照常路由；不在 → 返回 400，错误信息「虚拟模型 <名称> 只支持这些接口：<所勾接口，用逗号分隔>」，这次请求不打分、不扣费。
- C4：路由后的请求交给选中渠道的适配器，按渠道现有的转换规则转成上游格式；扣费、重试和直接用同一格式请求同一真实模型时相同。
- C5：Anthropic 格式请求按下面的规则接入一期的路由流程，其余规则（打分标签、选档、降档保护、顾问、会话记录、计费、日志）和 Responses、Chat Completions 相同：
  - 最后一条消息是用户写的文字 → 用户新消息，打分；最后一条消息只包含工具结果（`tool_result`）→ 工具结果续跑，不打分。
  - 发给打分模型的内容和一期相同：最新一条用户消息、前几条用户消息的开头、本地算出的特征；`system`、工具结果、思考内容不发。
  - 会话识别：用「密钥 + 第一条用户消息的内容」算出的指纹，和不带 prompt_cache_key 的 Chat Completions 请求相同。
  - 请求体里的模型名换成选中档位的真实模型名；思考力度按「档位思考力度」的规则写进请求。
  - 每次发往上游之前（包括重试换渠道之后），这次用的渠道和会话里记的上一次渠道不同 → 去掉历史 assistant 消息里的 `thinking`、`redacted_thinking` 块；渠道相同时原样发送。日志记下清理了几段，写法和 Responses 相同。
- C6：配置弹窗点「保存」时，对三档和顾问逐个检查：在它的每个分组里，这个模型的可用渠道如果全都是明确拒收某个所勾接口格式的渠道类型 → 不能保存，在这一档的分组那一行写红字「<模型> 在分组 <分组> 里只有 <渠道类型> 渠道，接收不了 <接口> 格式」。「明确拒收」以渠道适配器代码里写死返回「不支持」或收到就出错为准（例如 Codex 渠道拒收 Chat Completions 和 Anthropic，Claude 订阅渠道拒收 Responses，百度、智谱等渠道收到 Anthropic 格式就出错）。检查按请求时真正走的规则逐个渠道判断：Chat Completions 和 Anthropic 格式的请求，如果命中「Chat 转 Responses」策略、又没开透传，会先转成 Responses 再发给渠道，这时看渠道收不收 Responses；其他情况看渠道收不收原格式。分组里有一个渠道能收就能保存；请求时只从能收这次请求格式的渠道里选，重试换渠道也一样，所以同一分组里混挂能收和不能收的渠道时，请求不会落到不能收的渠道上。打分模型不做这项检查，打分请求选渠道也不受这条限制。
- C7：一期 C7「三档和顾问的模型接口不一致」这项保存检查去掉，由 C6 代替。
- C8：模型列表接口（`/v1/models`）和后台模型列表里，虚拟模型的「支持的接口」写所勾的接口。

## 验收
- [ ] C1：新建虚拟模型 → 接口只选了 Responses；三项都不选就保存 → 接口下面红字「至少选择一种接口」，弹窗不关。
- [ ] C2：用一期保存、自动判断为 Responses 的配置启动 → 配置页显示只勾 Responses，Responses 请求照常路由。
- [ ] C3：auto-gpt 勾 Responses 和 Anthropic → 两种格式都能路由；用 Chat Completions 请求 → 400，信息「虚拟模型 auto-gpt 只支持这些接口：Responses, Anthropic」，没有打分记录和消费记录。
- [ ] C4：auto-gpt 勾 Anthropic、普通档 gpt-5.5 走 OpenAI 渠道，用 Anthropic 格式请求 → 上游收到 OpenAI 格式请求，扣费等于用 Anthropic 格式直接请求 gpt-5.5 的扣费。
- [ ] C5：Anthropic 格式下：用户新消息有打分记录、工具结果续跑没有；发给打分模型的内容里没有 system 和工具结果；同一段对话的两次请求被认成同一会话；上游请求的模型名是真实模型名；换了渠道时历史里的 thinking 块被去掉、日志写明清理了几段，渠道没换时原样发送。
- [ ] C6：普通档 gpt-5.5 在 subscription 里只有 Codex 渠道，勾 Anthropic 保存 → 出现用例里的红字；在 subscription 里再加一个 OpenAI 类型的 gpt-5.5 渠道 → 能保存。开启「Chat 转 Responses」策略并命中这个 Codex 渠道 → 勾 Chat 或 Anthropic 也能保存；分组里只有 MiniMax 渠道、策略命中它 → 勾 Chat 保存被拦下；开了透传 → 策略不算数。
- [ ] C7：三档分别用只支持 Responses 的模型和只支持 Chat 的模型，接口勾两项，渠道都能接收 → 能保存，不再报「接口不一致」。
- [ ] C8：auto-gpt 勾 Responses 和 Anthropic → `/v1/models` 里 auto-gpt 的支持接口是这两项。

## 验证路径
1. 新建 auto-claude，接口只勾 Anthropic，三档用 Claude 订阅渠道的模型，保存并保存更改。
2. 用 Claude Code 依次发一条简单问题、一条高风险问题、一次工具结果续跑 → 档位依次是简单档、困难档、困难档；续跑没有打分记录。
3. 用 Responses 格式请求 auto-claude → 400，信息写明只支持 Anthropic。
4. 编辑 auto-gpt，勾上 Anthropic，档位用只有 Codex 渠道的分组 → 保存被拦下并出现 C6 的红字。
