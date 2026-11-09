---
name: capability-override-glm
description: 覆盖弹窗一档都不勾时报错；把 glm-5.1 覆盖成 low、high 后立即提交并出现「保存成功」，编辑弹窗里力度下拉只有 low、high，刷新后仍是已覆盖
category: smart-routing-capability
---
# 覆盖未确认的模型
## 简介
点 glm-5.1（未确认）的「覆盖」，取消全部勾选后保存看报错；再只勾 low、high 保存（立即提交，出现「保存成功」）。打开 auto-chat 的编辑弹窗，普通档（glm-5.1）选「固定」看力度下拉，取消弹窗。最后刷新页面，看覆盖还在。
## 测试目的
二期 PRD 02 验收 C3（把 glm-5.1 覆盖成支持 low / high → 来源「已覆盖」，配置弹窗里它的力度下拉只有 low、high）、C7（全部不勾就保存 → 出现「至少选择一个档位」，弹窗不关）；立即保存 PRD 2026-11-09.smart-routing-instant-save 01 C4（覆盖弹窗保存立即提交，提示「保存成功」）、C9（提交成功后刷新，看到的是刚保存的值）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "capability-override-glm",
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
      "text": "未确认"
    },
    {
      "action": "click",
      "selector": "role=group[name=\"glm-5.1 的操作\"s] >> role=button[name=\"覆盖\"s]"
    },
    {
      "action": "waitFor",
      "text": "覆盖 glm-5.1 的思考能力"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const boxes = [...dlg.querySelectorAll('[role=checkbox]')]; const names = boxes.map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(names) !== JSON.stringify(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])) throw new Error('勾选框是：' + names.join(',')); const checked = boxes.filter((b) => b.getAttribute('aria-checked') === 'true').map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(checked) !== JSON.stringify([\"low\", \"medium\", \"high\"])) throw new Error('勾上的是：' + checked.join(',')); const isDisabled = (b) => b.getAttribute('aria-disabled') === 'true' || b.hasAttribute('data-disabled'); if (boxes.some((b) => isDisabled(b) !== false)) throw new Error('勾选框可用状态不对：' + boxes.map((b) => isDisabled(b)).join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"low\")"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"medium\")"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"high\")"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "至少选择一个档位"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); if (!dlg) throw new Error('弹窗关闭了'); const p = [...dlg.querySelectorAll('p')].find((x) => x.textContent.trim() === '至少选择一个档位'); if (!p || !p.className.includes('text-destructive')) throw new Error('「至少选择一个档位」不是红字'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "override-empty-error"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"low\")"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"high\")"
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
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"glm-5.1\"); if (!r) throw new Error('能力表里没有 ' + \"glm-5.1\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"glm-5.1\", \"已覆盖\", \"low high\", \"修改 恢复内置\"])) throw new Error(\"glm-5.1\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=group[name=\"auto-chat 的操作\"s] >> role=button[name=\"编辑\"s]"
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
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^普通/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); if (panel.innerText.includes('能力未确认')) throw new Error('覆盖后仍显示「能力未确认」'); return 'ok'; }",
      "expect": "ok"
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
      "selector": "role=option[name=\"high\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; if (JSON.stringify(got) !== JSON.stringify([\"low\", \"high\"])) problems.push('选项是 ' + got.join(' / ') + '，期望 ' + \"low / high\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "screenshot",
      "name": "override-glm-dropdown"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    },
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "已覆盖"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"glm-5.1\"); if (!r) throw new Error('能力表里没有 ' + \"glm-5.1\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"glm-5.1\", \"已覆盖\", \"low high\", \"修改 恢复内置\"])) throw new Error(\"glm-5.1\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "override-glm-after-reload"
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
- [ ] glm-5.1 的覆盖弹窗初值勾着 low medium high；全部取消后保存，弹窗不关，红字「至少选择一个档位」
- [ ] 只勾 low、high 保存后出现「保存成功」，这一行来源「已覆盖」，档位 low high
- [ ] auto-chat 普通档（glm-5.1）覆盖后不显示「能力未确认」，选「固定」后力度下拉只有 low、high
- [ ] 刷新后 glm-5.1 仍是「已覆盖」、low high，操作「修改」「恢复内置」
