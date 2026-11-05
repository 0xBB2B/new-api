---
name: 12-log-effort-row
description: 使用日志详情的智能路由块在「档位」下面显示实际发出的思考力度和来源
---
# 日志详情的「思考力度」一行

## 目标
主请求的日志详情「智能路由」块在「档位」下面新增一行「思考力度」，按日志里记的力度、来源、降档前的值写出文字。

## 业务规则（来源：PRD 06-log-effort）
- 日志字段 `smart_routing.effort`：`{"value": "high", "source": "follow", "from": "xhigh"}`；`source` 取值 follow / fixed / client / client_unset / no_thinking；没降档时没有 `from`。
- C1 写法：
  - follow →「<value>（跟随打分）」；fixed →「<value>（固定）」；client →「<value>（不覆盖，客户端传入）」；
  - 有 `from` → 括号里在来源后加「，由 <from> 降档」（value 比 from 低）或「，由 <from> 升档」（value 比 from 高；七档顺序 none、minimal、low、medium、high、xhigh、max），例如「high（跟随打分，由 xhigh 降档）」「low（不覆盖，由 minimal 升档）」；
  - no_thinking →「未发送（模型不支持思考）」；client_unset →「未发送（客户端未指定）」。
- C2：工具结果续跑、压缩请求同样显示。C3：打分记录的详情不显示。C4：没有 `effort` 字段的旧日志不显示这一行、页面不报错。
- C5：用户和管理员看到的相同；七种语言都有翻译。

## 涉及文件
- 修改 `web/src/features/usage-logs/types.ts`、`web/src/features/usage-logs/lib/smart-route.ts`
- 修改 `web/src/features/usage-logs/components/dialogs/smart-route-section.tsx`
- 修改 7 个 `web/src/i18n/locales/*.json`
- 测试：扩展 `web/src/features/usage-logs/components/__tests__/detail-preview.test.tsx`

## 函数清单
### types.ts、lib/smart-route.ts
| 函数名 | 职责 |
|---|---|
| `SmartRouting`（类型） | 增加可选的 `effort` 对象 |
| `smartRouteEffortText` | 由 `effort` 对象和 `t` 生成这一行文字；没有 `effort` 返回空 |
### smart-route-section.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRouteSection` | 主请求、`effort` 存在时在「档位」下面加一行「思考力度」 |

## 协作关系
字段由后端 03 plan 写入。行的写法和现有的「档位」「选档原因」行相同。

## 验证方式
- 测试入口：渲染日志详情（沿用 `detail-preview.test.tsx` 构造智能路由日志的方式）；`smartRouteEffortText`
- 测试输入：七种 `effort` 组合；打分记录；没有 `effort` 的旧日志
- 预期结果：
  - [ ] follow / fixed / client / follow+from / client+from / client+from 升档 / no_thinking / client_unset → 依次显示「high（跟随打分）」「medium（固定）」「low（不覆盖，客户端传入）」「high（跟随打分，由 xhigh 降档）」「high（不覆盖，由 xhigh 降档）」「low（不覆盖，由 minimal 升档）」「未发送（模型不支持思考）」「未发送（客户端未指定）」
  - [ ] 「思考力度」一行紧跟在「档位」下面
  - [ ] 打分记录详情没有这一行；旧日志没有这一行且不报错
