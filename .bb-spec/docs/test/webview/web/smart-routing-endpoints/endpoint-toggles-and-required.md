---
name: endpoint-toggles-and-required
description: 接口是 Responses、Chat、Anthropic 三个可多选按钮，选中样式和左栏选中 Tab 相同，悬停显示完整路径；一个都不选就保存时接口下面写红字
category: smart-routing-endpoints
---
# 接口多选和至少选一种
## 简介
打开添加弹窗，核对接口按钮的初始选中、选中样式和悬停提示，把 Chat、Anthropic 也选上；再编辑已有的 auto-gpt，取消唯一的 Responses 后保存。
## 测试目的
二期 PRD 04 验收 C1 后半（接口三个按钮可以同时选中多个，选中样式是主色描边、浅色底、主色文字，和左栏选中的 Tab 相同；悬停显示完整路径，Responses 的提示写明压缩请求跟着它）；PRD 03 验收 C1（新建只选 Responses；三项都不选就保存 → 接口下面红字「至少选择一种接口」，弹窗不关）；PRD 04 C11 的「接口的错误写在顶部，不给左栏加红点」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "endpoint-toggles-and-required",
  "category": "smart-routing-endpoints",
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
      "selector": "role=button[name=\"添加虚拟模型\"s] >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const buttons = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')]; const btn = (name) => buttons.find((b) => b.textContent.trim() === name); const pressed = buttons.filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(pressed) !== JSON.stringify(['Responses'])) throw new Error('新建时接口选中的是：' + pressed.join(',')); const on = getComputedStyle(btn('Responses')); const off = getComputedStyle(btn('Chat')); const active = getComputedStyle([...dlg.querySelectorAll('[role=tab]')].find((t) => t.getAttribute('aria-selected') === 'true')); const problems = []; if (on.color === off.color) problems.push('选中和没选中的文字颜色一样'); if (on.backgroundColor === off.backgroundColor) problems.push('选中和没选中的底色一样'); for (const key of ['color', 'backgroundColor', 'borderTopColor']) { if (on[key] !== active[key]) problems.push('接口按钮和选中 Tab 的 ' + key + ' 不同：' + on[key] + ' / ' + active[key]); } if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"Chat\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"Anthropic\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const buttons = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')]; const btn = (name) => buttons.find((b) => b.textContent.trim() === name); const pressed = buttons.filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(pressed) !== JSON.stringify(['Responses', 'Chat', 'Anthropic'])) throw new Error('多选后选中的是：' + pressed.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"Responses\"s]"
    },
    {
      "action": "waitFor",
      "text": "OpenAI Responses (/v1/responses)；Responses 压缩请求 (/v1/responses/compact) 跟着它"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"Chat\"s]"
    },
    {
      "action": "waitFor",
      "text": "OpenAI (/v1/chat/completions)"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=button[name=\"Anthropic\"s]"
    },
    {
      "action": "waitFor",
      "text": "Anthropic (/v1/messages)"
    },
    {
      "action": "screenshot",
      "name": "endpoint-toggles"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
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
      "selector": "role=dialog >> role=button[name=\"Responses\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const buttons = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')]; const btn = (name) => buttons.find((b) => b.textContent.trim() === name); const pressed = buttons.filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (pressed.length) throw new Error('取消后仍选中：' + pressed.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "至少选择一种接口"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const buttons = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')]; const btn = (name) => buttons.find((b) => b.textContent.trim() === name); const pressed = buttons.filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (!dlg) throw new Error('弹窗关闭了'); const msg = [...dlg.querySelectorAll('[data-slot=form-message]')].find((p) => p.textContent.trim() === '至少选择一种接口'); if (!msg || !msg.closest('[data-slot=form-item]').contains(btn('Responses'))) throw new Error('红字不在接口下面'); const marked = [...dlg.querySelectorAll('[role=tab]')].filter((t) => t.querySelector('.bg-destructive') || t.textContent.includes('个错误')).map((t) => t.textContent.trim()); if (marked.length) throw new Error('不应有 Tab 带红点：' + marked.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "endpoint-required"
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
- [ ] 新建时只选中 Responses；选中按钮和左栏选中 Tab 的文字色、底色、描边色相同，和没选中的按钮不同
- [ ] Chat、Anthropic 可以和 Responses 同时选中
- [ ] 悬停分别显示「OpenAI Responses (/v1/responses)；Responses 压缩请求 (/v1/responses/compact) 跟着它」「OpenAI (/v1/chat/completions)」「Anthropic (/v1/messages)」
- [ ] 编辑时三个都不选就保存，弹窗不关，接口下面红字「至少选择一种接口」，左栏没有红点
