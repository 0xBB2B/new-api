---
name: capability-empty-after-delete
description: 删掉唯一的虚拟模型后，「模型思考能力」卡片显示空状态「还没有虚拟模型用到的模型」
category: smart-routing-capability
---
# 没有虚拟模型时的能力卡片
## 简介
只有 auto-gpt 时打开智能路由页，能力卡片列出 3 个模型；从「…」菜单删除 auto-gpt 后，看能力卡片。
## 测试目的
二期 PRD 02 验收 C6：删光虚拟模型 → 卡片显示「还没有虚拟模型用到的模型」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "capability-empty-after-delete",
  "category": "smart-routing-capability",
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
      "text": "模型思考能力"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); if (rows.length !== 3) throw new Error('能力表有 ' + rows.length + ' 行'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=group[name=\"auto-gpt 的操作\"s] >> role=button[name=\"更多操作\"s]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"删除\"s]"
    },
    {
      "action": "click",
      "selector": "role=alertdialog >> role=button[name=\"删除\"s]"
    },
    {
      "action": "waitFor",
      "text": "还没有虚拟模型用到的模型"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); const head = card.querySelector('h3').parentElement.innerText.replace(/\\s+/g, ' ').trim(); if (head !== '模型思考能力 0') throw new Error('卡片标题和数量：' + head); if (card.querySelector('table')) throw new Error('空状态时还有表格'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "capability-empty"
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
- [ ] 删除 auto-gpt 后，能力卡片数量变成 0，显示空状态「还没有虚拟模型用到的模型」，没有表格
