---
name: routes-table-summary
description: 表格名称下写所勾接口的简称，档位第二行写力度摘要和分组顺序；不支持思考、固定、跟随打分三种摘要；格子太窄时末尾省略
category: smart-routing-editor
---
# 虚拟模型表格的第二行文字
## 简介
编辑 auto-gpt：接口勾上 Anthropic，困难档改成「跟随打分 medium～xhigh」，把分组从 api → subscription 拖成 subscription → api；在弹窗里保存（立即提交，出现「保存成功」），核对表格后刷新再核对一次。同一张表里还有 auto-chat（简单档 gpt-4.1 不支持思考、困难档固定 high），一起核对摘要写法。
## 测试目的
二期 PRD 04 验收 C13：auto-gpt 名称下写「Responses / Anthropic 接口」，困难档第二行写「跟随打分 medium～xhigh · subscription → api」；力度摘要另有「不覆盖」「固定 high」「不支持思考」；第二行格子用省略号截断。对应 PRD 04 验证路径 2。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "routes-table-summary",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"api\", \"subscription\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-chat\", \"endpoints\": [\"openai\"], \"tiers\": {\"simple\": {\"model\": \"gpt-4.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "text": "不支持思考 · api"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-chat\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-chat\"); const want = [\"auto-chat Chat 接口\", \"gpt-4.1 不支持思考 · api\", \"glm-5.1 默认 不覆盖 · api\", \"gpt-5.6-sol 固定 high · subscription → api\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-chat\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
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
      "selector": "role=dialog >> role=button[name=\"Anthropic\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^困难/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=group[name=\"思考力度方式\"s] >> role=button[name=\"跟随打分\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"最低\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"medium\"s]"
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
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. api\", \"2. subscription\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const tick = () => new Promise((r) => setTimeout(r, 80)); const tags = () => [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')]; const find = (g) => tags().find((li) => li.getAttribute('aria-label').endsWith('. ' + g)); const src = find(\"subscription\"); const dst = find(\"api\"); if (!src || !dst) throw new Error('找不到要拖动的分组标签'); if (!src.draggable) throw new Error('分组标签不能拖动'); const dt = new DataTransfer(); src.dispatchEvent(new DragEvent('dragstart', {bubbles: true, cancelable: true, dataTransfer: dt})); await tick(); const rect = dst.getBoundingClientRect(); const x = rect.left + 2; const opts = {bubbles: true, cancelable: true, dataTransfer: dt, clientX: x, clientY: rect.top + rect.height / 2}; dst.dispatchEvent(new DragEvent('dragenter', opts)); dst.dispatchEvent(new DragEvent('dragover', opts)); await tick(); dst.dispatchEvent(new DragEvent('drop', opts)); src.dispatchEvent(new DragEvent('dragend', {bubbles: true, dataTransfer: dt})); await tick(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. subscription\", \"2. api\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
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
      "action": "waitFor",
      "text": "保存成功"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const want = [\"auto-gpt Responses / Anthropic 接口\", \"glm-5.1 不覆盖 · api\", \"gpt-5.5 默认 不覆盖 · api\", \"gpt-5.6-sol 跟随打分 medium～xhigh · subscription → api\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-gpt\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => r.textContent.includes('跟随打分 medium～xhigh')); const code = row.querySelectorAll('td')[3].querySelector('code'); let el = code; while (el && el !== row) { const s = getComputedStyle(el); if (s.textOverflow === 'ellipsis' && s.whiteSpace === 'nowrap') return 'ok'; el = el.parentElement; } throw new Error('困难档第二行没有设置末尾省略'); }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "table-after-save"
    },
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "Responses / Anthropic 接口"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const want = [\"auto-gpt Responses / Anthropic 接口\", \"glm-5.1 不覆盖 · api\", \"gpt-5.5 默认 不覆盖 · api\", \"gpt-5.6-sol 跟随打分 medium～xhigh · subscription → api\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-gpt\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "table-after-reload"
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
- [ ] auto-chat 一行：名称下「Chat 接口」，简单档「不支持思考 · api」，普通档「不覆盖 · api」，困难档「固定 high · subscription → api」
- [ ] 弹窗保存后出现「保存成功」，auto-gpt 名称下写「Responses / Anthropic 接口」，困难档第二行写「跟随打分 medium～xhigh · subscription → api」
- [ ] 第二行文字设置了单行末尾省略（text-overflow: ellipsis）
- [ ] 刷新后显示不变
