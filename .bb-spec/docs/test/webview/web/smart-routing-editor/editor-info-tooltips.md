---
name: editor-info-tooltips
description: 弹窗里不直接显示说明文字，悬停每个 ⓘ 显示对应说明；打分、顾问 Tab 各只有三行
category: smart-routing-editor
---
# 字段说明收进 ⓘ
## 简介
编辑 auto-gpt，在「打分」「档位」「顾问」三个 Tab 和展开的高级设置里，先确认说明文字都不在弹窗正文里，再逐个悬停字段名旁的 ⓘ 看提示；同时核对打分、顾问、困难档各有哪几行。
## 测试目的
二期 PRD 04 验收 C3（除了出错红字和不支持思考的灰字，看不到别的说明文字；悬停每个 ⓘ 显示对应说明）、C4（「打分」只有打分模型、超时、分组顺序三行，ⓘ 文字和约束一致）、C7（「顾问」只有三行，没有思考力度）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-info-tooltips",
  "category": "smart-routing-editor",
  "target": "web",
  "setup": [
    {
      "action": "navigate",
      "target": "/sign-in"
    },
    {
      "action": "evaluate",
      "script": "() => { localStorage.setItem('i18nextLng', 'zhCN'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "fill",
      "selector": "input[name=username]",
      "value": "root"
    },
    {
      "action": "fill",
      "selector": "input[name=password]",
      "value": "Test123456!"
    },
    {
      "action": "click",
      "selector": "button[type=submit]"
    },
    {
      "action": "waitFor",
      "selector": "a[href^=\"/usage-logs\"]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
      "expect": "ok"
    }
  ],
  "steps": [
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "Responses 接口"
    },
    {
      "action": "click",
      "selector": "role=group[name=\"auto-gpt 的操作\"s] >> role=button[name=\"编辑\"s]"
    },
    {
      "action": "waitFor",
      "text": "编辑虚拟模型"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const text = dlg.innerText; const hints = [\"客户端可以用勾选的任一种格式请求这个虚拟模型，其他格式返回 400；格式转换由渠道完成。至少选一种\", \"用户每发一条新消息调用一次。费用按打分模型的价格，单独记一条消费记录。\", \"超时就用默认档\", \"按顺序尝试这些分组，前一个分组的渠道都失败了再试下一个；最多 5 个。拖动分组调整顺序\", \"同一段对话里，用户连续多条消息表示不满意时，这一轮交给顾问模型回答。\", \"下面 4 种情况都用默认档：打分失败或超时；打分把握不足；工具续跑时没有会话；选中的档位没有渠道\", \"用户每发一条新消息，打分后落到其中一档\", \"不覆盖：保留客户端传来的值，模型不支持这一档时自动换成最接近的档位。跟随打分：按打分的「要想多深」取值（低→low、中→medium、高→high），再限制在最低和最高之间；打分失败时用最高。力度下拉只列这个模型支持的档位\", \"对话历史超过这个长度就不降档，避免按原价重读整段历史\", \"打分模型的把握低于它时用默认档\", \"最新消息超过这个长度就不算简单题\"]; const shown = hints.filter((h) => text.includes(h)); if (shown.length) throw new Error('弹窗里直接显示了说明文字：' + shown.join(' | ')); if (dlg.querySelector('[data-slot=form-description]')) throw new Error('弹窗里有字段说明（form-description）'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = document.querySelector('[role=dialog] [role=tabpanel]'); if (!panel) throw new Error('找不到当前 Tab 的内容'); const got = [...panel.querySelectorAll('label')].map((l) => l.textContent.trim()); const want = [\"打分模型\", \"超时（毫秒）\", \"分组顺序\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('这一栏的行是：' + got.join(' / ') + '，期望：' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"接口说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "客户端可以用勾选的任一种格式请求这个虚拟模型，其他格式返回 400；格式转换由渠道完成。至少选一种"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"打分模型说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "用户每发一条新消息调用一次。费用按打分模型的价格，单独记一条消费记录。"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"超时（毫秒）说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "超时就用默认档"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"分组顺序说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "按顺序尝试这些分组，前一个分组的渠道都失败了再试下一个；最多 5 个。拖动分组调整顺序"
    },
    {
      "action": "screenshot",
      "name": "info-scoring"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^顾问/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const text = dlg.innerText; const hints = [\"客户端可以用勾选的任一种格式请求这个虚拟模型，其他格式返回 400；格式转换由渠道完成。至少选一种\", \"用户每发一条新消息调用一次。费用按打分模型的价格，单独记一条消费记录。\", \"超时就用默认档\", \"按顺序尝试这些分组，前一个分组的渠道都失败了再试下一个；最多 5 个。拖动分组调整顺序\", \"同一段对话里，用户连续多条消息表示不满意时，这一轮交给顾问模型回答。\", \"下面 4 种情况都用默认档：打分失败或超时；打分把握不足；工具续跑时没有会话；选中的档位没有渠道\", \"用户每发一条新消息，打分后落到其中一档\", \"不覆盖：保留客户端传来的值，模型不支持这一档时自动换成最接近的档位。跟随打分：按打分的「要想多深」取值（低→low、中→medium、高→high），再限制在最低和最高之间；打分失败时用最高。力度下拉只列这个模型支持的档位\", \"对话历史超过这个长度就不降档，避免按原价重读整段历史\", \"打分模型的把握低于它时用默认档\", \"最新消息超过这个长度就不算简单题\"]; const shown = hints.filter((h) => text.includes(h)); if (shown.length) throw new Error('弹窗里直接显示了说明文字：' + shown.join(' | ')); if (dlg.querySelector('[data-slot=form-description]')) throw new Error('弹窗里有字段说明（form-description）'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = document.querySelector('[role=dialog] [role=tabpanel]'); if (!panel) throw new Error('找不到当前 Tab 的内容'); const got = [...panel.querySelectorAll('label')].map((l) => l.textContent.trim()); const want = [\"顾问模型\", \"连续不满意几次后交给顾问\", \"分组顺序\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('这一栏的行是：' + got.join(' / ') + '，期望：' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"顾问模型说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "同一段对话里，用户连续多条消息表示不满意时，这一轮交给顾问模型回答。"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const text = dlg.innerText; const hints = [\"客户端可以用勾选的任一种格式请求这个虚拟模型，其他格式返回 400；格式转换由渠道完成。至少选一种\", \"用户每发一条新消息调用一次。费用按打分模型的价格，单独记一条消费记录。\", \"超时就用默认档\", \"按顺序尝试这些分组，前一个分组的渠道都失败了再试下一个；最多 5 个。拖动分组调整顺序\", \"同一段对话里，用户连续多条消息表示不满意时，这一轮交给顾问模型回答。\", \"下面 4 种情况都用默认档：打分失败或超时；打分把握不足；工具续跑时没有会话；选中的档位没有渠道\", \"用户每发一条新消息，打分后落到其中一档\", \"不覆盖：保留客户端传来的值，模型不支持这一档时自动换成最接近的档位。跟随打分：按打分的「要想多深」取值（低→low、中→medium、高→high），再限制在最低和最高之间；打分失败时用最高。力度下拉只列这个模型支持的档位\", \"对话历史超过这个长度就不降档，避免按原价重读整段历史\", \"打分模型的把握低于它时用默认档\", \"最新消息超过这个长度就不算简单题\"]; const shown = hints.filter((h) => text.includes(h)); if (shown.length) throw new Error('弹窗里直接显示了说明文字：' + shown.join(' | ')); if (dlg.querySelector('[data-slot=form-description]')) throw new Error('弹窗里有字段说明（form-description）'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); if (!panel) throw new Error('找不到当前 Tab 的内容'); const got = [...panel.querySelectorAll('label')].map((l) => l.textContent.trim()); const want = [\"简单档模型\", \"思考力度\", \"分组顺序\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('这一栏的行是：' + got.join(' / ') + '，期望：' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"默认档说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "下面 4 种情况都用默认档：打分失败或超时；打分把握不足；工具续跑时没有会话；选中的档位没有渠道"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"简单档模型说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "用户每发一条新消息，打分后落到其中一档"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"思考力度说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "不覆盖：保留客户端传来的值，模型不支持这一档时自动换成最接近的档位。跟随打分：按打分的「要想多深」取值（低→low、中→medium、高→high），再限制在最低和最高之间；打分失败时用最高。力度下拉只列这个模型支持的档位"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"高级设置\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const text = dlg.innerText; const hints = [\"客户端可以用勾选的任一种格式请求这个虚拟模型，其他格式返回 400；格式转换由渠道完成。至少选一种\", \"用户每发一条新消息调用一次。费用按打分模型的价格，单独记一条消费记录。\", \"超时就用默认档\", \"按顺序尝试这些分组，前一个分组的渠道都失败了再试下一个；最多 5 个。拖动分组调整顺序\", \"同一段对话里，用户连续多条消息表示不满意时，这一轮交给顾问模型回答。\", \"下面 4 种情况都用默认档：打分失败或超时；打分把握不足；工具续跑时没有会话；选中的档位没有渠道\", \"用户每发一条新消息，打分后落到其中一档\", \"不覆盖：保留客户端传来的值，模型不支持这一档时自动换成最接近的档位。跟随打分：按打分的「要想多深」取值（低→low、中→medium、高→high），再限制在最低和最高之间；打分失败时用最高。力度下拉只列这个模型支持的档位\", \"对话历史超过这个长度就不降档，避免按原价重读整段历史\", \"打分模型的把握低于它时用默认档\", \"最新消息超过这个长度就不算简单题\"]; const shown = hints.filter((h) => text.includes(h)); if (shown.length) throw new Error('弹窗里直接显示了说明文字：' + shown.join(' | ')); if (dlg.querySelector('[data-slot=form-description]')) throw new Error('弹窗里有字段说明（form-description）'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"降档时历史 token 上限说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "对话历史超过这个长度就不降档，避免按原价重读整段历史"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"可信度门槛说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "打分模型的把握低于它时用默认档"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"简单题消息长度上限（字）说明\"s]"
    },
    {
      "action": "waitFor",
      "text": "最新消息超过这个长度就不算简单题"
    },
    {
      "action": "screenshot",
      "name": "info-advanced"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    }
  ],
  "teardown": [
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
      "expect": "ok"
    }
  ]
}
```
## 如何验收
- [ ] 三个 Tab 和高级设置展开后，弹窗正文里都看不到说明文字
- [ ] 「打分」只有 打分模型 / 超时（毫秒） / 分组顺序 三行；「顾问」只有 顾问模型 / 连续不满意几次后交给顾问 / 分组顺序 三行，没有思考力度
- [ ] 悬停 接口、打分模型、超时、分组顺序、顾问模型、默认档、档位模型、思考力度和高级设置三项的 ⓘ，提示文字与 PRD 04 约束一致
