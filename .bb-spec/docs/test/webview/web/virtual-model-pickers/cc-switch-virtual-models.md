---
name: cc-switch-virtual-models
description: 「填入 CC Switch」选 Claude、Codex 时模型下拉末尾有 auto-gpt、auto-chat，选 Gemini 时没有虚拟模型
category: virtual-model-pickers
---
# CC Switch 里的虚拟模型
## 简介
配好 auto-gpt（Responses、Anthropic）和 auto-chat（Chat）。在密钥页打开 cc-switch-key 的「CC Switch」，应用 Claude 时依次展开主模型、Haiku、Sonnet、Opus 四个下拉，再切到 Codex、Gemini 展开主模型下拉。
## 测试目的
二期 PRD 05 验收 C2：CC Switch 选 Codex → 主模型下拉末尾有两个虚拟模型；选 Claude → 四个下拉末尾都有；选 Gemini → 没有虚拟模型。需要 seed 建好的 cc-switch-key。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "cc-switch-virtual-models",
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
      "target": "/keys"
    },
    {
      "action": "waitFor",
      "text": "cc-switch-key"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"cc-switch-key\") >> role=button[name=\"打开菜单\"s]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"CC Switch\"s]"
    },
    {
      "action": "waitFor",
      "text": "填入 CC Switch"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"主模型\"]"
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
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.5\", \"gpt-5.6-sol\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); if (JSON.stringify(got.slice(-2)) !== JSON.stringify([\"auto-gpt\", \"auto-chat\"])) problems.push('最后几项是 ' + got.slice(-2).join(' / ') + '，期望 ' + \"auto-gpt / auto-chat\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"Haiku 模型\"]"
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
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.5\", \"gpt-5.6-sol\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); if (JSON.stringify(got.slice(-2)) !== JSON.stringify([\"auto-gpt\", \"auto-chat\"])) problems.push('最后几项是 ' + got.slice(-2).join(' / ') + '，期望 ' + \"auto-gpt / auto-chat\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"Sonnet 模型\"]"
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
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.5\", \"gpt-5.6-sol\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); if (JSON.stringify(got.slice(-2)) !== JSON.stringify([\"auto-gpt\", \"auto-chat\"])) problems.push('最后几项是 ' + got.slice(-2).join(' / ') + '，期望 ' + \"auto-gpt / auto-chat\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"Opus 模型\"]"
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
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.5\", \"gpt-5.6-sol\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); if (JSON.stringify(got.slice(-2)) !== JSON.stringify([\"auto-gpt\", \"auto-chat\"])) problems.push('最后几项是 ' + got.slice(-2).join(' / ') + '，期望 ' + \"auto-gpt / auto-chat\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "screenshot",
      "name": "cc-switch-claude"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"Codex\")"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"主模型\"]"
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
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.5\", \"gpt-5.6-sol\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); if (JSON.stringify(got.slice(-2)) !== JSON.stringify([\"auto-gpt\", \"auto-chat\"])) problems.push('最后几项是 ' + got.slice(-2).join(' / ') + '，期望 ' + \"auto-gpt / auto-chat\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "cc-switch-codex"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"Gemini\")"
    },
    {
      "action": "click",
      "selector": "role=dialog >> input[aria-label=\"主模型\"]"
    },
    {
      "action": "press",
      "key": "ArrowDown"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"gpt-5.5\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.5\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); for (const m of [\"auto-gpt\", \"auto-chat\"]) if (got.includes(m)) problems.push('不应出现选项 ' + m); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "cc-switch-gemini"
    },
    {
      "action": "press",
      "key": "Escape"
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
- [ ] Claude 的主模型、Haiku 模型、Sonnet 模型、Opus 模型四个下拉里先是真实模型，最后两项是 auto-gpt、auto-chat
- [ ] Codex 的主模型下拉最后两项是 auto-gpt、auto-chat
- [ ] Gemini 的主模型下拉里有真实模型，没有 auto-gpt、auto-chat
