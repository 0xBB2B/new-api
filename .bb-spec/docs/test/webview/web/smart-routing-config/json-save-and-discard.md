---
name: json-save-and-discard
description: JSON 视图里「保存」在内容没变（含只点「格式化 JSON」）时是灰的，改了才能点；有改动时点「可视化」先弹「放弃未保存的修改？」；保存后出现「保存成功」、按钮变灰，切回可视化不弹框，刷新后不变
category: smart-routing-config
---
# JSON 视图的保存和放弃修改
## 简介
切到「JSON」视图，看编辑框下方的「保存」是灰的；点「格式化 JSON」后仍是灰的。把困难档模型从 gpt-5.6-sol 改成 gpt-5.5，「保存」变得能点。点「可视化」标签弹出确认框，点「取消」留在 JSON。点「保存」，出现「保存成功」后按钮变回灰色、停在 JSON。再点「可视化」直接切换，困难档是 gpt-5.5；刷新后不变。
## 测试目的
立即保存 PRD 2026-11-09.smart-routing-instant-save 02 C1（JSON 编辑框下方靠右有「保存」）、C2（内容和已保存的一样时变灰，点「格式化 JSON」也算一样；改了内容就能点）、C4（成功后提示「保存成功」，停在 JSON 视图，「保存」变回灰色，切回可视化看到刚保存的配置）、C6 的「取消」一支（有改动时点「可视化」弹出「放弃未保存的修改？」，点「取消」留在 JSON、改动还在）、C7（没有改动时点「可视化」直接切换），以及 01 C9（刷新后是刚保存的值）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "json-save-and-discard",
  "category": "smart-routing-config",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"fixed\", \"effort\": \"low\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"medium\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "selector": "role=tab[name=\"JSON\"s]"
    },
    {
      "action": "waitFor",
      "selector": "role=textbox[name=\"智能路由\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const want = true; let b = null; for (let i = 0; i < 50; i++) { b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '保存' && x.checkVisibility()); if (b && b.disabled === want) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } if (!b) throw new Error('没有 JSON 框下方的「保存」按钮'); throw new Error('「保存」应是灰的'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"格式化 JSON\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const want = true; let b = null; for (let i = 0; i < 50; i++) { b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '保存' && x.checkVisibility()); if (b && b.disabled === want) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } if (!b) throw new Error('没有 JSON 框下方的「保存」按钮'); throw new Error('「保存」应是灰的'); }",
      "expect": "ok"
    },
    {
      "action": "fill",
      "selector": "role=textbox[name=\"智能路由\"s]",
      "value": "[\n  {\n    \"name\": \"auto-gpt\",\n    \"endpoints\": [\n      \"openai-response\"\n    ],\n    \"tiers\": {\n      \"simple\": {\n        \"model\": \"gpt-5.4-mini\",\n        \"groups\": [\n          \"api\"\n        ],\n        \"effort_mode\": \"fixed\",\n        \"effort\": \"low\",\n        \"effort_min\": \"\",\n        \"effort_max\": \"\"\n      },\n      \"normal\": {\n        \"model\": \"gpt-5.5\",\n        \"groups\": [\n          \"subscription\",\n          \"api\"\n        ],\n        \"effort_mode\": \"fixed\",\n        \"effort\": \"medium\",\n        \"effort_min\": \"\",\n        \"effort_max\": \"\"\n      },\n      \"hard\": {\n        \"model\": \"gpt-5.5\",\n        \"groups\": [\n          \"subscription\",\n          \"api\"\n        ],\n        \"effort_mode\": \"fixed\",\n        \"effort\": \"high\",\n        \"effort_min\": \"\",\n        \"effort_max\": \"\"\n      }\n    },\n    \"default_tier\": \"normal\",\n    \"scorer\": {\n      \"model\": \"gpt-5.6-luna\",\n      \"groups\": [\n        \"subscription\",\n        \"api\"\n      ],\n      \"timeout_ms\": 2000\n    },\n    \"advisor\": {\n      \"model\": \"gpt-5.6-sol\",\n      \"groups\": [\n        \"subscription\",\n        \"api\"\n      ],\n      \"after_dissatisfied\": 2\n    },\n    \"downgrade_max_history_tokens\": 20000,\n    \"confidence_threshold\": 0.6,\n    \"simple_max_message_chars\": 2000\n  }\n]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const want = false; let b = null; for (let i = 0; i < 50; i++) { b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '保存' && x.checkVisibility()); if (b && b.disabled === want) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } if (!b) throw new Error('没有 JSON 框下方的「保存」按钮'); throw new Error('「保存」应是能点'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=tab[name=\"可视化\"s]"
    },
    {
      "action": "waitFor",
      "text": "放弃未保存的修改？"
    },
    {
      "action": "assertText",
      "selector": "role=alertdialog",
      "text": "JSON 里的改动还没保存，切回可视化会丢掉这些改动。"
    },
    {
      "action": "assertVisible",
      "selector": "role=alertdialog >> role=button[name=\"放弃修改\"s]"
    },
    {
      "action": "screenshot",
      "name": "json-discard-confirm"
    },
    {
      "action": "click",
      "selector": "role=alertdialog >> role=button[name=\"取消\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=alertdialog]')) break; await new Promise((r) => setTimeout(r, 100)); } if (document.querySelector('[role=alertdialog]')) throw new Error('确认框没有关闭'); const tab = document.querySelector('[role=tablist][aria-label=\"虚拟模型编辑模式\"] [role=tab][aria-selected=true]'); if (!tab || tab.textContent.trim() !== 'JSON') throw new Error('没有停在 JSON 视图，当前是：' + (tab && tab.textContent.trim())); const ta = document.querySelector('textarea[aria-label=\"智能路由\"]'); const hard = JSON.parse(ta.value)[0].tiers.hard.model; if (hard !== 'gpt-5.5') throw new Error('JSON 里困难档是 ' + hard + '，改动丢了'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "保存成功"
    },
    {
      "action": "evaluate",
      "script": "async () => { const want = true; let b = null; for (let i = 0; i < 50; i++) { b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '保存' && x.checkVisibility()); if (b && b.disabled === want) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } if (!b) throw new Error('没有 JSON 框下方的「保存」按钮'); throw new Error('「保存」应是灰的'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=alertdialog]')) break; await new Promise((r) => setTimeout(r, 100)); } if (document.querySelector('[role=alertdialog]')) throw new Error('确认框没有关闭'); const tab = document.querySelector('[role=tablist][aria-label=\"虚拟模型编辑模式\"] [role=tab][aria-selected=true]'); if (!tab || tab.textContent.trim() !== 'JSON') throw new Error('没有停在 JSON 视图，当前是：' + (tab && tab.textContent.trim())); const ta = document.querySelector('textarea[aria-label=\"智能路由\"]'); const hard = JSON.parse(ta.value)[0].tiers.hard.model; if (hard !== 'gpt-5.5') throw new Error('JSON 里困难档是 ' + hard + '，改动丢了'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "json-saved"
    },
    {
      "action": "click",
      "selector": "role=tab[name=\"可视化\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { await new Promise((r) => setTimeout(r, 500)); if (document.querySelector('[role=alertdialog]')) throw new Error('没有改动时不应弹确认框'); const tab = document.querySelector('[role=tablist][aria-label=\"虚拟模型编辑模式\"] [role=tab][aria-selected=true]'); if (!tab || tab.textContent.trim() !== '可视化') throw new Error('没有切到可视化，当前是：' + (tab && tab.textContent.trim())); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 auto-gpt'); const got = row.querySelectorAll('td')[3].innerText.replace(/\\s+/g, ' ').trim(); if (!got.startsWith(\"gpt-5.5 \")) throw new Error('auto-gpt 困难档是：' + got + '；期望模型 ' + \"gpt-5.5\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "Responses 接口"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 auto-gpt'); const got = row.querySelectorAll('td')[3].innerText.replace(/\\s+/g, ' ').trim(); if (!got.startsWith(\"gpt-5.5 \")) throw new Error('auto-gpt 困难档是：' + got + '；期望模型 ' + \"gpt-5.5\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "json-save-after-reload"
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
- [ ] 刚切到 JSON 时编辑框下方的「保存」是灰的；点「格式化 JSON」后仍是灰的
- [ ] 把困难档改成 gpt-5.5 后「保存」能点
- [ ] 点「可视化」弹出确认框，标题「放弃未保存的修改？」，说明「JSON 里的改动还没保存，切回可视化会丢掉这些改动。」，有「取消」「放弃修改」；点「取消」后留在 JSON，改动还在
- [ ] 点「保存」后出现「保存成功」，「保存」变回灰色，停在 JSON 视图
- [ ] 再点「可视化」不弹确认框直接切换，表格里困难档是 gpt-5.5；刷新后仍是 gpt-5.5
