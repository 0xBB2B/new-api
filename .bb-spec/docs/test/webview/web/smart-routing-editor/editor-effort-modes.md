---
name: editor-effort-modes
description: 思考力度在不覆盖、固定、跟随打分之间切换时后面的小下拉变成零个、一个、两个；最低高于最高时报错；七档以外的值保存报错
category: smart-routing-editor
---
# 思考力度的三种方式
## 简介
编辑 auto-gpt，在简单档（gpt-5.4-mini）依次点「固定」「跟随打分」，看同一行后面的小下拉；把最低设成 high、最高设成 low 后保存，看报错；改成 high～xhigh 后保存成功。最后切到 JSON 视图，写一个七档以外的力度值保存，再用接口依次保存七档里的每个值。
## 测试目的
二期 PRD 04 验收 C6（「固定」「跟随打分」切换时后面的小下拉跟着变成一个或两个）；PRD 01 验收 C4 的「最低高于最高时保存报错」和 C1 的「后台接受七档里任一值，七档以外的值保存时报错」。上游实际收到的力度不在网页上验证。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-effort-modes",
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
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const group = dlg.querySelector('[role=group][aria-label=\"思考力度方式\"]'); if (!group) throw new Error(\"简单档\" + '没有思考力度方式按钮'); const got = [...group.querySelectorAll('button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(got) !== JSON.stringify([\"不覆盖\"])) throw new Error(\"简单档\" + '思考力度选中的是：' + got.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); const selects = [...panel.querySelectorAll('[role=combobox]')].map((b) => b.getAttribute('aria-label')).filter((n) => ['思考力度', '最低', '最高'].includes(n)); if (selects.length) throw new Error('不覆盖时不应有力度下拉：' + selects.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=group[name=\"思考力度方式\"s] >> role=button[name=\"固定\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const group = document.querySelector('[role=dialog] [role=group][aria-label=\"思考力度方式\"]'); const row = group.parentElement; const selects = [...row.querySelectorAll('[role=combobox]')].map((b) => b.getAttribute('aria-label')); if (JSON.stringify(selects) !== JSON.stringify(['思考力度'])) throw new Error('固定时同一行的下拉是：' + selects.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"思考力度\"s]"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"xhigh\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; if (JSON.stringify(got) !== JSON.stringify([\"none\", \"low\", \"medium\", \"high\", \"xhigh\"])) problems.push('选项是 ' + got.join(' / ') + '，期望 ' + \"none / low / medium / high / xhigh\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=group[name=\"思考力度方式\"s] >> role=button[name=\"跟随打分\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const group = document.querySelector('[role=dialog] [role=group][aria-label=\"思考力度方式\"]'); const row = group.parentElement; const selects = [...row.querySelectorAll('[role=combobox]')].map((b) => b.getAttribute('aria-label')); if (JSON.stringify(selects) !== JSON.stringify(['最低', '最高'])) throw new Error('跟随打分时同一行的下拉是：' + selects.join(',')); if (!row.textContent.includes('～')) throw new Error('最低和最高之间没有「～」'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-follow"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"最低\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"high\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"最高\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"low\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "最低力度不能高于最高力度"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); if (!dlg) throw new Error('弹窗关闭了'); const msg = [...dlg.querySelectorAll('[data-slot=form-message]')].find((p) => p.textContent.trim() === '最低力度不能高于最高力度'); if (!msg) throw new Error('没有红字'); const group = dlg.querySelector('[role=group][aria-label=\"思考力度方式\"]'); if (!msg.closest('[data-slot=form-item]').contains(group)) throw new Error('红字不在思考力度这一行'); const tab = [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith('简单')); if (!tab.textContent.includes('1 个错误')) throw new Error('「简单」没有错误数'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-min-over-max"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"最高\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"xhigh\"s]"
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
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const got = row.querySelectorAll('td')[1].innerText.replace(/\\s+/g, ' ').trim(); if (got !== \"gpt-5.4-mini 跟随打分 high～xhigh · api\") throw new Error(\"auto-gpt\" + ' 第 2 列是：' + got + '；期望：' + \"gpt-5.4-mini 跟随打分 high～xhigh · api\"); return 'ok'; }",
      "expect": "ok"
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
      "action": "fill",
      "selector": "role=textbox[name=\"智能路由\"s]",
      "value": "[\n  {\n    \"name\": \"auto-gpt\",\n    \"endpoints\": [\n      \"openai-response\"\n    ],\n    \"tiers\": {\n      \"simple\": {\n        \"model\": \"gpt-5.4-mini\",\n        \"groups\": [\n          \"api\"\n        ],\n        \"effort_mode\": \"fixed\",\n        \"effort\": \"ultra\",\n        \"effort_min\": \"\",\n        \"effort_max\": \"\"\n      },\n      \"normal\": {\n        \"model\": \"gpt-5.5\",\n        \"groups\": [\n          \"subscription\",\n          \"api\"\n        ],\n        \"effort_mode\": \"\",\n        \"effort\": \"\",\n        \"effort_min\": \"\",\n        \"effort_max\": \"\"\n      },\n      \"hard\": {\n        \"model\": \"gpt-5.6-sol\",\n        \"groups\": [\n          \"subscription\",\n          \"api\"\n        ],\n        \"effort_mode\": \"follow\",\n        \"effort\": \"\",\n        \"effort_min\": \"medium\",\n        \"effort_max\": \"xhigh\"\n      }\n    },\n    \"default_tier\": \"normal\",\n    \"scorer\": {\n      \"model\": \"gpt-5.6-luna\",\n      \"groups\": [\n        \"subscription\",\n        \"api\"\n      ],\n      \"timeout_ms\": 2000\n    },\n    \"advisor\": {\n      \"model\": \"gpt-5.6-sol\",\n      \"groups\": [\n        \"subscription\",\n        \"api\"\n      ],\n      \"after_dissatisfied\": 2\n    },\n    \"downgrade_max_history_tokens\": 20000,\n    \"confidence_threshold\": 0.6,\n    \"simple_max_message_chars\": 2000\n  }\n]"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"保存更改\"s]"
    },
    {
      "action": "waitFor",
      "text": "请选择有效的思考力度"
    },
    {
      "action": "screenshot",
      "name": "effort-invalid-json"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; const base = {\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}; for (const effort of ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max']) { const r = JSON.parse(JSON.stringify(base)); r.tiers.simple.effort_mode = 'fixed'; r.tiers.simple.effort = effort; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([r])}}); } return 'ok'; }",
      "expect": "ok"
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
- [ ] 不覆盖时没有力度下拉；点「固定」后同一行出现一个「思考力度」下拉，选项是 gpt-5.4-mini 支持的 none / low / medium / high / xhigh；点「跟随打分」后变成「最低 ～ 最高」两个下拉
- [ ] 最低 high、最高 low 时保存，弹窗不关，思考力度这一行下面红字「最低力度不能高于最高力度」，「简单」带「1 个错误」
- [ ] 改成 high～xhigh 后保存成功，表格简单档第二行是「跟随打分 high～xhigh · api」
- [ ] JSON 视图里写 effort "ultra" 保存，提示「请选择有效的思考力度」；用接口依次保存 none、minimal、low、medium、high、xhigh、max 都成功
