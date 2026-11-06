---
name: playground-chat-only
description: 演练场模型下拉只列勾了 Chat 的虚拟模型；给 auto-gpt 勾上 Chat 并保存更改后刷新，演练场出现 auto-gpt
category: virtual-model-pickers
---
# 演练场里的虚拟模型
## 简介
配好 auto-gpt（Responses、Anthropic）和 auto-chat（Chat），打开演练场的模型下拉；再到智能路由页给 auto-gpt 勾上 Chat、保存更改，回到演练场重新打开下拉。
## 测试目的
二期 PRD 05 验收 C4：演练场模型下拉里有 auto-chat，没有 auto-gpt；把 auto-gpt 也勾上 Chat 并保存更改 → 刷新后演练场出现 auto-gpt。验证路径 3 里「选它发一条消息正常回答」要真实上游，不在本用例里。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "playground-chat-only",
  "category": "virtual-model-pickers",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\", \"anthropic\"], \"tiers\": {\"simple\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-chat\", \"endpoints\": [\"openai\"], \"tiers\": {\"simple\": {\"model\": \"gpt-4.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
      "expect": "ok"
    }
  ],
  "steps": [
    {
      "action": "navigate",
      "target": "/playground"
    },
    {
      "action": "waitFor",
      "selector": "button[role=combobox]:visible:has(svg[class*=\"lucide-cpu\"]) >> nth=0"
    },
    {
      "action": "click",
      "selector": "button[role=combobox]:visible:has(svg[class*=\"lucide-cpu\"]) >> nth=0"
    },
    {
      "action": "waitFor",
      "text": "模型分组"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"auto-chat\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; if (!got.includes('auto-chat')) problems.push('没有 auto-chat'); if (got.includes('auto-gpt')) problems.push('出现了没勾 Chat 的 auto-gpt'); const virtual = got.map((m, i) => [m, i]).filter(([m]) => m.startsWith('auto-')); const real = got.map((m, i) => [m, i]).filter(([m]) => !m.startsWith('auto-')); if (real.length && virtual.length && Math.min(...virtual.map(([, i]) => i)) < Math.max(...real.map(([, i]) => i))) problems.push('虚拟模型没有排在真实模型后面：' + got.join(' / ')); if (problems.length) throw new Error(problems.join('；') + '（选项：' + got.join(' / ') + '）'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "playground-before"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "Responses / Anthropic 接口"
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
      "selector": "role=dialog >> role=button[name=\"Chat\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"保存更改\"s]"
    },
    {
      "action": "waitFor",
      "text": "保存成功"
    },
    {
      "action": "navigate",
      "target": "/playground"
    },
    {
      "action": "waitFor",
      "selector": "button[role=combobox]:visible:has(svg[class*=\"lucide-cpu\"]) >> nth=0"
    },
    {
      "action": "click",
      "selector": "button[role=combobox]:visible:has(svg[class*=\"lucide-cpu\"]) >> nth=0"
    },
    {
      "action": "waitFor",
      "text": "模型分组"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"auto-gpt\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of ['auto-gpt', 'auto-chat']) if (!got.includes(m)) problems.push('没有 ' + m); const virtual = got.map((m, i) => [m, i]).filter(([m]) => m.startsWith('auto-')); const real = got.map((m, i) => [m, i]).filter(([m]) => !m.startsWith('auto-')); if (real.length && virtual.length && Math.min(...virtual.map(([, i]) => i)) < Math.max(...real.map(([, i]) => i))) problems.push('虚拟模型没有排在真实模型后面：' + got.join(' / ')); if (problems.length) throw new Error(problems.join('；') + '（选项：' + got.join(' / ') + '）'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "playground-after"
    },
    {
      "action": "press",
      "key": "Escape"
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
- [ ] 演练场模型下拉里有 auto-chat，没有 auto-gpt
- [ ] 给 auto-gpt 勾上 Chat 并保存更改后，演练场模型下拉里出现 auto-gpt
- [ ] 有真实模型时，虚拟模型排在真实模型后面
