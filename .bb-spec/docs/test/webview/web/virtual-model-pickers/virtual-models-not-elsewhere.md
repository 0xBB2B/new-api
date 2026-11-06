---
name: virtual-models-not-elsewhere
description: 价格页、模型定价设置、档位模型下拉、渠道模型配置里搜不到虚拟模型；首页「首个 API 请求」的示例用第一个真实模型
category: virtual-model-pickers
---
# 虚拟模型不出现的地方
## 简介
配好 auto-gpt、auto-chat 后，依次打开价格页、「模型定价」设置、智能路由的档位模型下拉、新建渠道抽屉的模型下拉，确认都没有虚拟模型；最后打开首页，看「首个 API 请求」示例里的模型名。
## 测试目的
二期 PRD 05 验收 C5（价格页、模型定价设置、档位模型下拉、渠道模型配置里搜 auto-gpt 都搜不到）、C6（首页快速开始的示例代码里的模型名是第一个真实模型）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "virtual-models-not-elsewhere",
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
      "target": "/pricing"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.6-sol"
    },
    {
      "action": "evaluate",
      "script": "() => { const text = document.body.innerText; const found = ['auto-gpt', 'auto-chat'].filter((m) => text.includes(m)); if (found.length) throw new Error('价格页出现了虚拟模型：' + found.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "pricing-page"
    },
    {
      "action": "navigate",
      "target": "/system-settings/billing/model-pricing"
    },
    {
      "action": "waitFor",
      "selector": "input[placeholder=\"搜索模型...\"]"
    },
    {
      "action": "fill",
      "selector": "input[placeholder=\"搜索模型...\"] >> nth=0",
      "value": "auto-"
    },
    {
      "action": "evaluate",
      "script": "async () => { await new Promise((r) => setTimeout(r, 1000)); const text = (document.querySelector('main') || document.body).innerText; const found = ['auto-gpt', 'auto-chat'].filter((m) => text.includes(m)); if (found.length) throw new Error('模型定价里出现了虚拟模型：' + found.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "fill",
      "selector": "input[placeholder=\"搜索模型...\"] >> nth=0",
      "value": "gpt-5.6-sol"
    },
    {
      "action": "waitFor",
      "selector": "td:has-text(\"gpt-5.6-sol\")"
    },
    {
      "action": "screenshot",
      "name": "model-pricing"
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
      "selector": "role=button[name=\"添加虚拟模型\"s] >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]",
      "value": "auto-"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"auto-gpt\", \"auto-chat\"]) if (got.includes(m)) problems.push('不应出现选项 ' + m); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    },
    {
      "action": "navigate",
      "target": "/channels"
    },
    {
      "action": "click",
      "selector": "role=button[name=/创建渠道/]"
    },
    {
      "action": "click",
      "selector": "role=option[name=/^OpenAI/]"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"选择模型或添加自定义模型\"]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> input[aria-label=\"选择模型或添加自定义模型\"]",
      "value": "auto-gpt"
    },
    {
      "action": "waitFor",
      "selector": "role=option"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"auto-gpt\", \"auto-chat\"]) if (got.includes(m)) problems.push('不应出现选项 ' + m); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "channel-models"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "navigate",
      "target": "/dashboard"
    },
    {
      "action": "waitFor",
      "text": "首个 API 请求"
    },
    {
      "action": "navigate",
      "target": "/dashboard"
    },
    {
      "action": "waitFor",
      "text": "首个 API 请求"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; const res = await call('GET', '/api/user/models'); const list = res.data || []; const first = list[0]; if (!first || first.startsWith('auto-')) throw new Error('/api/user/models 第一个不是真实模型：' + list.join(',')); let shown = ''; for (let i = 0; i < 100; i++) { const m = document.body.innerText.match(/\"model\":\"([^\"]+)\"/); shown = m ? m[1] : ''; if (shown === first) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('首个 API 请求示例里的模型是 ' + shown + '，期望第一个真实模型 ' + first); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "dashboard-first-request"
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
- [ ] 价格页能看到 gpt-5.6-sol，看不到 auto-gpt、auto-chat
- [ ] 模型定价设置里搜「auto-」没有结果，搜 gpt-5.6-sol 有
- [ ] 智能路由档位模型下拉输入「auto-」没有 auto-gpt、auto-chat
- [ ] 新建渠道的模型下拉输入 auto-gpt，只有「添加自定义模型」一项，没有现成的 auto-gpt 选项
- [ ] 首页「首个 API 请求」示例里的模型名等于 /api/user/models 的第一个，且不是虚拟模型
