---
name: endpoint-chat-to-responses-policy
description: 保存检查算上「Chat 转 Responses」策略：命中 Codex 时勾 Chat 能保存，开了透传策略不算数；策略命中只有 MiniMax 的分组时勾 Chat 被拦下
category: smart-routing-endpoints
---
# 保存检查和 Chat 转 Responses 策略
## 简介
先打开「Chat 转 Responses」策略（只对 Codex 类型、模型 gpt-5.5），编辑 auto-gpt（普通档 gpt-5.5 只在 subscription，只有 Codex 渠道）勾上 Chat 保存；再打开全局透传后重新保存；最后关掉透传，把普通档分组换成只有 MiniMax 渠道的 team-a、接口只留 Chat 保存，然后把策略改成对所有渠道生效再保存。
## 测试目的
二期 PRD 03 验收 C6 的后半：开启「Chat 转 Responses」策略并命中这个 Codex 渠道 → 勾 Chat 也能保存；开了透传 → 策略不算数；分组里只有 MiniMax 渠道、策略命中它 → 勾 Chat 保存被拦下。策略和透传通过 /api/option/ 接口修改，结束时恢复成关闭。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "endpoint-chat-to-responses-policy",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PUT', '/api/option/', {key: 'global.chat_completions_to_responses_policy', value: JSON.stringify({\"enabled\": true, \"all_channels\": false, \"channel_types\": [57], \"model_patterns\": [\"^gpt-5\\\\.5$\"]})}); await call('PUT', '/api/option/', {key: 'global.pass_through_request_enabled', value: \"false\"}); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const got = row.querySelectorAll('td')[0].innerText.replace(/\\s+/g, ' ').trim(); if (got !== \"auto-gpt Responses / Chat 接口\") throw new Error(\"auto-gpt\" + ' 第 1 列是：' + got + '；期望：' + \"auto-gpt Responses / Chat 接口\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "policy-codex-chat-ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PUT', '/api/option/', {key: 'global.chat_completions_to_responses_policy', value: JSON.stringify({\"enabled\": true, \"all_channels\": false, \"channel_types\": [57], \"model_patterns\": [\"^gpt-5\\\\.5$\"]})}); await call('PUT', '/api/option/', {key: 'global.pass_through_request_enabled', value: \"true\"}); return 'ok'; }",
      "expect": "ok"
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
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Chat Completions 格式"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^普通/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); const row = panel.querySelector('[role=group][aria-label=\"分组顺序\"]'); if (!row || row.getAttribute('aria-invalid') !== 'true') throw new Error('分组这一行没有描红'); const msg = [...row.closest('[data-slot=form-item]').querySelectorAll('[data-slot=form-message]')].map((p) => p.textContent.trim()); if (!msg.includes(\"gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Chat Completions 格式\")) throw new Error('分组下面的红字是：' + msg.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "policy-passthrough-blocked"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PUT', '/api/option/', {key: 'global.chat_completions_to_responses_policy', value: JSON.stringify({\"enabled\": true, \"all_channels\": false, \"channel_types\": [57], \"model_patterns\": [\"^gpt-5\\\\.5$\"]})}); await call('PUT', '/api/option/', {key: 'global.pass_through_request_enabled', value: \"false\"}); return 'ok'; }",
      "expect": "ok"
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
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^普通/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 subscription\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"team-a\"s]"
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
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const got = row.querySelectorAll('td')[2].innerText.replace(/\\s+/g, ' ').trim(); if (got !== \"gpt-5.5 默认 不覆盖 · team-a\") throw new Error(\"auto-gpt\" + ' 第 3 列是：' + got + '；期望：' + \"gpt-5.5 默认 不覆盖 · team-a\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PUT', '/api/option/', {key: 'global.chat_completions_to_responses_policy', value: JSON.stringify({\"enabled\": true, \"all_channels\": true, \"model_patterns\": [\"^gpt-5\\\\.5$\"]})}); await call('PUT', '/api/option/', {key: 'global.pass_through_request_enabled', value: \"false\"}); return 'ok'; }",
      "expect": "ok"
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
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.5 在分组 team-a 里只有 MiniMax 渠道，接收不了 Chat Completions 格式"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^普通/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); const row = panel.querySelector('[role=group][aria-label=\"分组顺序\"]'); if (!row || row.getAttribute('aria-invalid') !== 'true') throw new Error('分组这一行没有描红'); const msg = [...row.closest('[data-slot=form-item]').querySelectorAll('[data-slot=form-message]')].map((p) => p.textContent.trim()); if (!msg.includes(\"gpt-5.5 在分组 team-a 里只有 MiniMax 渠道，接收不了 Chat Completions 格式\")) throw new Error('分组下面的红字是：' + msg.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "policy-minimax-blocked"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    }
  ],
  "teardown": [
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PUT', '/api/option/', {key: 'global.chat_completions_to_responses_policy', value: JSON.stringify({\"enabled\": false, \"all_channels\": true})}); await call('PUT', '/api/option/', {key: 'global.pass_through_request_enabled', value: \"false\"}); return 'ok'; }",
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
- [ ] 策略只对 Codex 生效时，普通档只有 Codex 渠道也能勾 Chat 保存，表格写「Responses / Chat 接口」
- [ ] 打开全局透传后再保存，普通档分组下面红字「gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Chat Completions 格式」
- [ ] 关掉透传、普通档换成 team-a（只有 MiniMax）、接口只留 Chat，策略不命中 MiniMax 时能保存
- [ ] 策略改成对所有渠道生效后再保存，红字「gpt-5.5 在分组 team-a 里只有 MiniMax 渠道，接收不了 Chat Completions 格式」
