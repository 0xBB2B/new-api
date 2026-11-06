---
name: capability-table-sources
description: 「模型思考能力」卡片只列档位用到的模型、同一模型只一行，来源、档位小标签和操作按规则显示；内置表的 Claude、Gemini、DeepSeek 档位和代码一致，2025 年的模型显示「未确认」
category: smart-routing-capability
---
# 模型思考能力卡片
## 简介
配好 auto-gpt（gpt-5.4-mini / gpt-5.5 / gpt-5.6-sol）和 auto-chat（gpt-4.1 / glm-5.1 / gpt-5.6-sol），打开智能路由页核对「模型思考能力」卡片的标题、数量、说明和每一行；再加两个虚拟模型：auto-family 用今年发布的 claude-opus-4-6、gemini-3.8-flash、deepseek-v4-pro，auto-old 用 2025 年发布的 claude-sonnet-4-5、gemini-2.5-pro、gpt-5.2，刷新后核对这六行。
## 测试目的
二期 PRD 02 验收 C1（gpt-5.6-sol、一个 Claude、一个 Gemini、一个 DeepSeek 模型的档位和代码里写明来源的内置表一致，gpt-4.1 显示「不支持思考」）、C2（glm-5.1 和 2025 年发布的 claude-sonnet-4-5、gemini-2.5-pro、gpt-5.2 来源「未确认」、档位 low / medium / high。按 2026-11-06 定的口径，内置表只收 2026 年发布的模型，不支持思考的老模型如 gpt-4.1 除外）、C4（标题、数量、说明文字）、C5（共用的 gpt-5.6-sol 只一行，打分模型 gpt-5.6-luna 不在表里，三种来源的颜色和描边样式、操作列按钮）。期望的档位取自 service/smartroute/capability.go（BuiltinThinkingCapability、内置表）和 relaykit/relayconvert/reasoning 的 ClaudeSupportedEfforts / GeminiSupportedEfforts：claude-opus-4-6 支持关闭思考、有 effort 和 max、没有 xhigh；gemini-3.8-flash 是按级别取值的 Gemini 3，列 minimal / low / medium / high；和厂商文档逐条对照不在网页上做。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "capability-table-sources",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-chat\", \"endpoints\": [\"openai\"], \"tiers\": {\"simple\": {\"model\": \"gpt-4.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "text": "模型思考能力"
    },
    {
      "action": "waitFor",
      "text": "未确认"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const head = card.querySelector('h3').parentElement.innerText.replace(/\\s+/g, ' ').trim(); if (head !== '模型思考能力 5') throw new Error('卡片标题和数量：' + head); const desc = '智能路由按这里的信息决定能发哪些思考力度。内置值来自各厂商官方文档，可以按模型覆盖。'; if (!card.innerText.includes(desc)) throw new Error('说明文字不对'); const ths = [...card.querySelectorAll('thead th')].map((th) => th.innerText.trim()); if (JSON.stringify(ths) !== JSON.stringify(['模型', '来源', '支持的档位', '操作'])) throw new Error('表头：' + ths.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const got = rows.map(cells); const want = [[\"gpt-5.4-mini\", \"内置\", \"none low medium high xhigh\", \"覆盖\"], [\"gpt-5.5\", \"内置\", \"none low medium high xhigh\", \"覆盖\"], [\"gpt-5.6-sol\", \"内置\", \"none low medium high xhigh max\", \"覆盖\"], [\"gpt-4.1\", \"内置\", \"不支持思考\", \"覆盖\"], [\"glm-5.1\", \"未确认\", \"low medium high\", \"覆盖\"]]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('能力表是：' + JSON.stringify(got) + '；期望：' + JSON.stringify(want)); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const problems = []; const badge = (r) => r.querySelectorAll('td')[1].querySelector('span'); for (const m of ['gpt-5.4-mini', 'gpt-5.5', 'gpt-5.6-sol', 'gpt-4.1']) { if (!badge(rowOf(m)).className.includes('bg-secondary')) problems.push(m + ' 的「内置」不是灰色'); } const glm = rowOf('glm-5.1'); if (!badge(glm).className.includes('text-warning')) problems.push('glm-5.1 的「未确认」不是黄色'); const effortBadges = (r) => [...r.querySelectorAll('td')[2].querySelectorAll('span')]; if (effortBadges(glm).some((s) => !s.className.includes('border-border'))) problems.push('未确认的档位小标签不是描边样式'); if (effortBadges(rowOf('gpt-5.5')).some((s) => !s.className.includes('bg-secondary'))) problems.push('内置的档位小标签不是实心样式'); if (rows.some((r) => r.innerText.includes('gpt-5.6-luna'))) problems.push('打分模型 gpt-5.6-luna 出现在表里'); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "capability-five"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-chat\", \"endpoints\": [\"openai\"], \"tiers\": {\"simple\": {\"model\": \"gpt-4.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-family\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"claude-opus-4-6\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gemini-3.8-flash\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"deepseek-v4-pro\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-old\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"claude-sonnet-4-5\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gemini-2.5-pro\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.2\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "claude-sonnet-4-5"
    },
    {
      "action": "waitFor",
      "text": "未确认"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const head = card.querySelector('h3').parentElement.innerText.replace(/\\s+/g, ' ').trim(); if (head !== '模型思考能力 11') throw new Error('卡片标题和数量：' + head); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"claude-opus-4-6\"); if (!r) throw new Error('能力表里没有 ' + \"claude-opus-4-6\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"claude-opus-4-6\", \"内置\", \"none low medium high max\", \"覆盖\"])) throw new Error(\"claude-opus-4-6\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"gemini-3.8-flash\"); if (!r) throw new Error('能力表里没有 ' + \"gemini-3.8-flash\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"gemini-3.8-flash\", \"内置\", \"minimal low medium high\", \"覆盖\"])) throw new Error(\"gemini-3.8-flash\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"deepseek-v4-pro\"); if (!r) throw new Error('能力表里没有 ' + \"deepseek-v4-pro\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"deepseek-v4-pro\", \"内置\", \"none low high max\", \"覆盖\"])) throw new Error(\"deepseek-v4-pro\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"claude-sonnet-4-5\"); if (!r) throw new Error('能力表里没有 ' + \"claude-sonnet-4-5\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"claude-sonnet-4-5\", \"未确认\", \"low medium high\", \"覆盖\"])) throw new Error(\"claude-sonnet-4-5\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"gemini-2.5-pro\"); if (!r) throw new Error('能力表里没有 ' + \"gemini-2.5-pro\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"gemini-2.5-pro\", \"未确认\", \"low medium high\", \"覆盖\"])) throw new Error(\"gemini-2.5-pro\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"gpt-5.2\"); if (!r) throw new Error('能力表里没有 ' + \"gpt-5.2\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"gpt-5.2\", \"未确认\", \"low medium high\", \"覆盖\"])) throw new Error(\"gpt-5.2\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "expect": "ok",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); const rows = [...card.querySelectorAll('tbody tr')]; const problems = []; for (const m of ['claude-sonnet-4-5', 'gemini-2.5-pro', 'gpt-5.2']) { const r = rows.find((x) => x.querySelector('td').innerText.trim() === m); if (!r) { problems.push('没有 ' + m); continue; } if (!r.querySelectorAll('td')[1].querySelector('span').className.includes('text-warning')) problems.push(m + ' 的「未确认」不是黄色'); if ([...r.querySelectorAll('td')[2].querySelectorAll('span')].some((x) => !x.className.includes('border-border'))) problems.push(m + ' 的档位不是描边样式'); } if (problems.length) throw new Error(problems.join('；')); return 'ok'; }"
    },
    {
      "action": "screenshot",
      "name": "capability-families"
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
- [ ] 卡片标题「模型思考能力」后面数量 5，说明文字与 PRD 02 C4 一致，表头依次是 模型 / 来源 / 支持的档位 / 操作
- [ ] 5 行依次是 gpt-5.4-mini、gpt-5.5、gpt-5.6-sol（只一行）、gpt-4.1（不支持思考）、glm-5.1（未确认，low medium high），每行操作只有「覆盖」；gpt-5.6-luna 不在表里
- [ ] 「内置」灰色、「未确认」黄色；未确认行的档位小标签是描边样式，其他行是实心样式
- [ ] 再加两个虚拟模型后数量 11：claude-opus-4-6 为 none low medium high max，gemini-3.8-flash 为 minimal low medium high，deepseek-v4-pro 为 none low high max，来源都是「内置」
- [ ] 2025 年发布的 claude-sonnet-4-5、gemini-2.5-pro、gpt-5.2 来源「未确认」（黄色），档位 low medium high（描边样式）
