---
name: endpoint-mixed-metadata-saves
description: 三档分别用只支持 Responses 和只支持 Chat 的模型，勾两种接口、渠道都能接收时能保存，没有接口不一致的报错
category: smart-routing-endpoints
---
# 模型支持的接口不同也能保存
## 简介
auto-mixed 的简单档是 o3-pro（模型列表里只支持 Responses）、普通档是 glm-5.1（只支持 Chat），两者都在 OpenAI 类型渠道上。先读 /api/pricing 确认这两个模型的「支持的接口」，再编辑 auto-mixed 勾上 Chat 保存。
## 测试目的
二期 PRD 03 验收 C7：三档分别用只支持 Responses 的模型和只支持 Chat 的模型，接口勾两项，渠道都能接收 → 能保存，没有一期那种「接口不一致」的报错。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "endpoint-mixed-metadata-saves",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-mixed\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"o3-pro\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
      "expect": "ok"
    }
  ],
  "steps": [
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; const res = await call('GET', '/api/pricing'); const items = res.data || []; const ep = (m) => { const x = items.find((i) => i.model_name === m); return x ? JSON.stringify(x.supported_endpoint_types) : '没有这个模型'; }; if (ep('o3-pro') !== JSON.stringify(['openai-response'])) throw new Error('前提不成立：o3-pro 支持的接口是 ' + ep('o3-pro')); if (ep('glm-5.1') !== JSON.stringify(['openai'])) throw new Error('前提不成立：glm-5.1 支持的接口是 ' + ep('glm-5.1')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "auto-mixed"
    },
    {
      "action": "click",
      "selector": "role=group[name=\"auto-mixed 的操作\"s] >> role=button[name=\"编辑\"s]"
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
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-mixed\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-mixed\"); const want = [\"auto-mixed Responses / Chat 接口\", \"o3-pro 不覆盖 · api\", \"glm-5.1 默认 不覆盖 · api\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-mixed\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "mixed-metadata-saved"
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
- [ ] /api/pricing 里 o3-pro 只支持 openai-response，glm-5.1 只支持 openai
- [ ] 勾上 Chat 后保存，弹窗关闭，没有「接口不一致」之类的报错
- [ ] 表格写「auto-mixed Responses / Chat 接口」
