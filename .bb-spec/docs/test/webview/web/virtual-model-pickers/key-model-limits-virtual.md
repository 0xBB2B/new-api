---
name: key-model-limits-virtual
description: 新建密钥时模型限制下拉末尾有虚拟模型，能和真实模型一起勾选保存，编辑时显示为已选标签并能点 × 移除
category: virtual-model-pickers
---
# 密钥模型限制里的虚拟模型
## 简介
在密钥页新建 vm-limit-key，在「高级设置 → 模型限制」里展开下拉，勾 gpt-5.5 和 auto-gpt 后保存；再编辑这个密钥，核对已选标签，点 auto-gpt 标签的 × 移除，然后关闭抽屉不保存。
## 测试目的
二期 PRD 05 验收 C3 的网页部分：模型限制下拉列出全部虚拟模型（排在真实模型后面），勾 auto-gpt 并保存后编辑时显示为已选，可以点 × 移除。「用它请求 auto-gpt 成功、请求白名单外的真实模型被拒」要发真实请求，不在本用例里。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "key-model-limits-virtual",
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
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; const list = await call('GET', '/api/token/?p=1&size=100'); const items = (list.data && (list.data.items || list.data.data || list.data)) || []; for (const t of (Array.isArray(items) ? items : [])) { if (t.name === 'vm-limit-key') await call('DELETE', '/api/token/' + t.id + '/'); } return 'ok'; }",
      "expect": "ok"
    }
  ],
  "steps": [
    {
      "action": "navigate",
      "target": "/keys"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"创建 API 密钥\"s]"
    },
    {
      "action": "waitFor",
      "text": "基本信息"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=textbox[name=\"名称\"s]",
      "value": "vm-limit-key"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=/^高级设置/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"选择模型（留空表示允许所有）\"]"
    },
    {
      "action": "press",
      "key": "ArrowDown"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"auto-chat\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.5\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); if (JSON.stringify(got.slice(-2)) !== JSON.stringify([\"auto-gpt\", \"auto-chat\"])) problems.push('最后几项是 ' + got.slice(-2).join(' / ') + '，期望 ' + \"auto-gpt / auto-chat\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.5\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"auto-gpt\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const chips = [...document.querySelectorAll('[role=dialog] [data-slot=combobox-chip]')].map((c) => c.textContent.trim()); if (JSON.stringify(chips) !== JSON.stringify(['gpt-5.5', 'auto-gpt'])) throw new Error('已选标签是：' + chips.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> text=模型限制"
    },
    {
      "action": "screenshot",
      "name": "limit-before-save"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存更改\"s]"
    },
    {
      "action": "waitFor",
      "text": "成功创建了 1 个 API 密钥"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "waitFor",
      "text": "vm-limit-key"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"vm-limit-key\") >> role=button[name=\"编辑\"s]"
    },
    {
      "action": "waitFor",
      "text": "更新 API 密钥"
    },
    {
      "action": "evaluate",
      "script": "async () => { const dlg = document.querySelector('[role=dialog]'); const trig = [...dlg.querySelectorAll('button[aria-expanded]')].find((b) => b.textContent.includes('高级设置')); if (!trig) throw new Error('没有高级设置'); if (trig.getAttribute('aria-expanded') !== 'true') trig.click(); for (let i = 0; i < 50; i++) { const chips = [...dlg.querySelectorAll('[data-slot=combobox-chip]')].map((c) => c.textContent.trim()); if (chips.length) { if (JSON.stringify(chips) !== JSON.stringify(['gpt-5.5', 'auto-gpt'])) throw new Error('编辑时已选标签是：' + chips.join(',')); return 'ok'; } await new Promise((r) => setTimeout(r, 100)); } throw new Error('编辑时没有已选标签'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const dlg = document.querySelector('[role=dialog]'); const chip = [...dlg.querySelectorAll('[data-slot=combobox-chip]')].find((c) => c.textContent.trim() === 'auto-gpt'); const x = chip && chip.querySelector('[data-slot=combobox-chip-remove]'); if (!x) throw new Error('auto-gpt 标签没有 ×'); x.click(); await new Promise((r) => setTimeout(r, 200)); const chips = [...dlg.querySelectorAll('[data-slot=combobox-chip]')].map((c) => c.textContent.trim()); if (JSON.stringify(chips) !== JSON.stringify(['gpt-5.5'])) throw new Error('移除后标签是：' + chips.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "limit-edit-removed"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"关闭\"s]"
    }
  ],
  "teardown": [
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; const list = await call('GET', '/api/token/?p=1&size=100'); const items = (list.data && (list.data.items || list.data.data || list.data)) || []; for (const t of (Array.isArray(items) ? items : [])) { if (t.name === 'vm-limit-key') await call('DELETE', '/api/token/' + t.id + '/'); } return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
      "expect": "ok"
    }
  ]
}
```
## 如何验收
- [ ] 模型限制下拉里有真实模型，最后两项是 auto-gpt、auto-chat
- [ ] 勾 gpt-5.5、auto-gpt 后显示两个已选标签，保存提示「成功创建了 1 个 API 密钥」
- [ ] 编辑 vm-limit-key 时模型限制显示 gpt-5.5、auto-gpt 两个已选标签；点 auto-gpt 的 × 后只剩 gpt-5.5
