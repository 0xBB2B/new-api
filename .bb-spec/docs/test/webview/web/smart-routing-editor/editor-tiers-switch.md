---
name: editor-tiers-switch
description: 档位 Tab 的三档切换显示档名加模型名，默认档带「默认」标签，改默认档后标签跟着移动
category: smart-routing-editor
---
# 档位里的三档切换和默认档
## 简介
编辑 auto-gpt，打开「档位」，核对三档切换上的文字、「默认」标签和悬停提示；把右边的「默认档」从普通档改成困难档，看标签移动；再打开困难档核对下面三行。
## 测试目的
二期 PRD 04 验收 C5：三档切换显示「简单 · <模型名>」等，默认档带「默认」标签，悬停显示「<模型名> · <力度摘要>」；改「默认档」下拉后标签跟着移动；每档下面是 <档>模型、思考力度、分组顺序三行。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-tiers-switch",
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
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const list = dlg.querySelector('[role=tablist][aria-label=\"档位\"]'); if (!list) throw new Error('没有三档切换'); const got = [...list.querySelectorAll('[role=tab]')].map((t) => t.textContent.replace(/\\s+/g, '')); const want = ['简单·gpt-5.4-mini', '普通·gpt-5.5默认', '困难·gpt-5.6-sol']; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('三档切换是：' + got.join(' | ')); const box = dlg.querySelector('[role=combobox][aria-label=\"默认档\"]'); if (box.textContent.trim() !== '普通档') throw new Error('默认档下拉：' + box.textContent.trim()); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=tab[name=/^困难/]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.6-sol · 跟随打分 medium～xhigh"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=tab[name=/^简单/]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.4-mini · 不覆盖"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"默认档\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"困难档\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const list = document.querySelector('[role=dialog] [role=tablist][aria-label=\"档位\"]'); const withBadge = [...list.querySelectorAll('[role=tab]')].filter((t) => [...t.querySelectorAll('span')].some((s) => s.textContent.trim() === '默认')).map((t) => t.textContent.trim().slice(0, 2)); if (JSON.stringify(withBadge) !== JSON.stringify(['困难'])) throw new Error('带「默认」标签的是：' + withBadge.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^困难/]"
    },
    {
      "action": "waitFor",
      "text": "困难档模型"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); if (!panel) throw new Error('找不到当前 Tab 的内容'); const got = [...panel.querySelectorAll('label')].map((l) => l.textContent.trim()); const want = [\"困难档模型\", \"思考力度\", \"分组顺序\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('这一栏的行是：' + got.join(' / ') + '，期望：' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "tiers-switch"
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
- [ ] 三档切换依次是「简单 · gpt-5.4-mini」「普通 · gpt-5.5 默认」「困难 · gpt-5.6-sol」，右边默认档下拉是「普通档」
- [ ] 悬停困难显示「gpt-5.6-sol · 跟随打分 medium～xhigh」，悬停简单显示「gpt-5.4-mini · 不覆盖」
- [ ] 默认档改成困难档后，「默认」标签只在困难上
- [ ] 困难档下面是 困难档模型 / 思考力度 / 分组顺序 三行
