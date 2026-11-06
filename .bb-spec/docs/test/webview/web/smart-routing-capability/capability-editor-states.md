---
name: capability-editor-states
description: 编辑弹窗里档位选 gpt-5.5、glm-5.1、gpt-4.1 时，分别只列支持的档位、带「能力未确认」标签、显示不支持思考的灰字
category: smart-routing-capability
---
# 档位模型的思考能力提示
## 简介
打开添加弹窗的「档位 → 简单」，依次把模型选成 gpt-5.5、glm-5.1、gpt-4.1，看思考力度这一行的变化。
## 测试目的
二期 PRD 02 验收 C10：在编辑弹窗里给档位选 gpt-5.5、glm-5.1、gpt-4.1 → 分别看到只列它支持的档位、带「能力未确认」标签（悬停有提示）、显示灰字「该模型不支持思考力度，请求时不发送思考参数」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "capability-editor-states",
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
      "value": "gpt-5.5"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.5\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=group[name=\"思考力度方式\"s] >> role=button[name=\"固定\"s]"
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
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]",
      "value": "glm-5.1"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"glm-5.1\"s]"
    },
    {
      "action": "waitFor",
      "text": "能力未确认"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); const badge = [...panel.querySelectorAll('span')].find((s) => s.textContent.trim() === '能力未确认'); if (!badge || !badge.className.includes('text-warning')) throw new Error('「能力未确认」不是黄色小标签'); const group = panel.querySelector('[role=group][aria-label=\"思考力度方式\"]'); if (!group || group.parentElement !== badge.parentElement) throw new Error('「能力未确认」不在思考力度这一行末尾'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> text=能力未确认"
    },
    {
      "action": "waitFor",
      "text": "内置能力表里没有这个模型，可在「模型思考能力」里补充"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=heading[name=\"添加虚拟模型\"s]"
    },
    {
      "action": "screenshot",
      "name": "capability-unconfirmed"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]",
      "value": "gpt-4.1"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-4.1\"s]"
    },
    {
      "action": "waitFor",
      "text": "该模型不支持思考力度，请求时不发送思考参数"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); if (panel.querySelector('[role=group][aria-label=\"思考力度方式\"]')) throw new Error('不支持思考时还显示力度选项'); const p = [...panel.querySelectorAll('p')].find((x) => x.textContent.trim() === '该模型不支持思考力度，请求时不发送思考参数'); if (!p || !p.closest('.text-muted-foreground')) throw new Error('不支持思考的提示不是灰字'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "capability-no-thinking"
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
- [ ] 选 gpt-5.5 后「固定」的力度下拉只有 none low medium high xhigh
- [ ] 选 glm-5.1 后思考力度这一行末尾出现黄色小标签「能力未确认」，悬停提示「内置能力表里没有这个模型，可在「模型思考能力」里补充」
- [ ] 选 gpt-4.1 后力度那一行换成灰字「该模型不支持思考力度，请求时不发送思考参数」，没有力度选项
