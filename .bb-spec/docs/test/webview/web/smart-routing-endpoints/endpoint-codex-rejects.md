---
name: endpoint-codex-rejects
description: 普通档 gpt-5.5 在 subscription 里只有 Codex 渠道时勾 Anthropic 保存被拦下，换成有 OpenAI 渠道的分组后能保存
category: smart-routing-endpoints
---
# 渠道拒收所勾格式时保存被拦下
## 简介
编辑 auto-gpt（普通档 gpt-5.5，分组 subscription → api），接口勾上 Anthropic 后保存，核对红字和跳转；再把普通档分组里的 subscription 换成 team-b（team-b 里 gpt-5.5 同时有 Codex 和 OpenAI 渠道）后保存。
## 测试目的
二期 PRD 03 主路径和验收 C6 的第一句：普通档 gpt-5.5 在 subscription 里只有 Codex 渠道，勾 Anthropic 保存 → 弹窗不关，左栏「档位」和「普通」带红点，普通档分组那一行描红写「gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Anthropic 格式」；分组里有一个能收的渠道就能保存（PRD 写的是往 subscription 里加一个 OpenAI 渠道，这里用 seed 里已有这种组合的 team-b 分组，避免等渠道缓存刷新）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "endpoint-codex-rejects",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "selector": "role=dialog >> role=button[name=\"Anthropic\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Anthropic 格式"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const problems = []; for (const name of ['档位', '普通']) { const t = tabOf(name); if (t.getAttribute('aria-selected') !== 'true') problems.push('没有自动切到「' + name + '」'); if (!t.querySelector('.bg-destructive')) problems.push('「' + name + '」没有红点'); } for (const name of ['打分', '顾问', '简单', '困难']) { if (tabOf(name).querySelector('.bg-destructive')) problems.push('「' + name + '」不应有红点'); } if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); const row = panel.querySelector('[role=group][aria-label=\"分组顺序\"]'); if (!row || row.getAttribute('aria-invalid') !== 'true') throw new Error('分组这一行没有描红'); const msg = [...row.closest('[data-slot=form-item]').querySelectorAll('[data-slot=form-message]')].map((p) => p.textContent.trim()); if (!msg.includes(\"gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Anthropic 格式\")) throw new Error('分组下面的红字是：' + msg.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "codex-rejects-anthropic"
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
      "selector": "role=option[name=\"team-b\"s]"
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
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const want = [\"auto-gpt Responses / Anthropic 接口\", \"glm-5.1 不覆盖 · api\", \"gpt-5.5 默认 不覆盖 · api → team-b\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-gpt\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "codex-rejects-fixed"
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
- [ ] 勾 Anthropic 保存后弹窗不关，自动切到「档位 → 普通」，「档位」「普通」带红点，其他 Tab 没有
- [ ] 普通档分组一行描红，红字「gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Anthropic 格式」
- [ ] 把 subscription 换成 team-b 后保存成功，表格写「Responses / Anthropic 接口」，普通档第二行「不覆盖 · api → team-b」
