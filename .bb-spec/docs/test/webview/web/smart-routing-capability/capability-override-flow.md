---
name: capability-override-flow
description: 覆盖 gpt-5.6-sol 去掉 xhigh、max 后来源变「已覆盖」，表格里困难档标黄写「超出模型能力，实际按 high 发送」，存的配置不变；覆盖保存和恢复内置都立即提交并出现「保存成功」，恢复内置后标黄消失
category: smart-routing-capability
---
# 覆盖模型能力与标黄提示
## 简介
在「模型思考能力」里点 gpt-5.6-sol 的「覆盖」，核对弹窗的标题、说明、勾选框初值和「不支持思考」开关；取消勾选 xhigh、max 后保存（立即提交，出现「保存成功」），核对后刷新。核对能力表、虚拟模型表格的标黄提示和编辑弹窗里存的值。最后点「恢复内置」（立即提交，出现「保存成功」）并刷新。
## 测试目的
二期 PRD 02 验收 C7（打开覆盖弹窗时勾选框是当前生效的值，打开「不支持思考」后勾选框变灰）、C9（困难档存「跟随打分 medium～xhigh」，把模型覆盖成只到 high 后那一格标黄并写「超出模型能力，实际按 high 发送」，编辑弹窗里存的值仍是 medium～xhigh）、C5 的操作列（已覆盖的行有「修改」「恢复内置」，来源小标签用主色），以及 C10 前半（力度下拉只列模型当前支持的档位）；立即保存 PRD 2026-11-09.smart-routing-instant-save 01 C4（覆盖弹窗说明是"勾选这个模型支持的思考力度，保存后立即生效。"，保存立即提交并提示「保存成功」）、C5（「恢复内置」不弹确认框，立即提交并提示「保存成功」）。对应 PRD 02 验证路径 2、3。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "capability-override-flow",
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
      "selector": "role=group[name=\"gpt-5.6-sol 的操作\"s] >> role=button[name=\"覆盖\"s]"
    },
    {
      "action": "waitFor",
      "text": "覆盖 gpt-5.6-sol 的思考能力"
    },
    {
      "action": "assertText",
      "selector": "role=dialog",
      "text": "勾选这个模型支持的思考力度，保存后立即生效。"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const boxes = [...dlg.querySelectorAll('[role=checkbox]')]; const names = boxes.map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(names) !== JSON.stringify(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])) throw new Error('勾选框是：' + names.join(',')); const checked = boxes.filter((b) => b.getAttribute('aria-checked') === 'true').map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(checked) !== JSON.stringify([\"none\", \"low\", \"medium\", \"high\", \"xhigh\", \"max\"])) throw new Error('勾上的是：' + checked.join(',')); const isDisabled = (b) => b.getAttribute('aria-disabled') === 'true' || b.hasAttribute('data-disabled'); if (boxes.some((b) => isDisabled(b) !== false)) throw new Error('勾选框可用状态不对：' + boxes.map((b) => isDisabled(b)).join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const sw = document.querySelector('[role=dialog] [role=switch]'); if (!sw || sw.getAttribute('aria-checked') !== 'false') throw new Error('「不支持思考」开关应为关'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:has-text(\"不支持思考\")"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const boxes = [...dlg.querySelectorAll('[role=checkbox]')]; const names = boxes.map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(names) !== JSON.stringify(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])) throw new Error('勾选框是：' + names.join(',')); const checked = boxes.filter((b) => b.getAttribute('aria-checked') === 'true').map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(checked) !== JSON.stringify([\"none\", \"low\", \"medium\", \"high\", \"xhigh\", \"max\"])) throw new Error('勾上的是：' + checked.join(',')); const isDisabled = (b) => b.getAttribute('aria-disabled') === 'true' || b.hasAttribute('data-disabled'); if (boxes.some((b) => isDisabled(b) !== true)) throw new Error('勾选框可用状态不对：' + boxes.map((b) => isDisabled(b)).join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:has-text(\"不支持思考\")"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const boxes = [...dlg.querySelectorAll('[role=checkbox]')]; const names = boxes.map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(names) !== JSON.stringify(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])) throw new Error('勾选框是：' + names.join(',')); const checked = boxes.filter((b) => b.getAttribute('aria-checked') === 'true').map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(checked) !== JSON.stringify([\"none\", \"low\", \"medium\", \"high\", \"xhigh\", \"max\"])) throw new Error('勾上的是：' + checked.join(',')); const isDisabled = (b) => b.getAttribute('aria-disabled') === 'true' || b.hasAttribute('data-disabled'); if (boxes.some((b) => isDisabled(b) !== false)) throw new Error('勾选框可用状态不对：' + boxes.map((b) => isDisabled(b)).join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"xhigh\")"
    },
    {
      "action": "click",
      "selector": "role=dialog >> label:text-is(\"max\")"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const boxes = [...dlg.querySelectorAll('[role=checkbox]')]; const names = boxes.map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(names) !== JSON.stringify(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])) throw new Error('勾选框是：' + names.join(',')); const checked = boxes.filter((b) => b.getAttribute('aria-checked') === 'true').map((b) => b.closest('label').textContent.trim()); if (JSON.stringify(checked) !== JSON.stringify([\"none\", \"low\", \"medium\", \"high\"])) throw new Error('勾上的是：' + checked.join(',')); const isDisabled = (b) => b.getAttribute('aria-disabled') === 'true' || b.hasAttribute('data-disabled'); if (boxes.some((b) => isDisabled(b) !== false)) throw new Error('勾选框可用状态不对：' + boxes.map((b) => isDisabled(b)).join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "override-dialog"
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
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"gpt-5.6-sol\"); if (!r) throw new Error('能力表里没有 ' + \"gpt-5.6-sol\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"gpt-5.6-sol\", \"已覆盖\", \"none low medium high\", \"修改 恢复内置\"])) throw new Error(\"gpt-5.6-sol\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); if (!rowOf('gpt-5.6-sol').querySelectorAll('td')[1].querySelector('span').className.includes('bg-primary')) throw new Error('「已覆盖」不是主色'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const got = row.querySelectorAll('td')[3].innerText.replace(/\\s+/g, ' ').trim(); if (got !== \"gpt-5.6-sol 跟随打分 medium～xhigh · subscription → api 超出模型能力，实际按 high 发送\") throw new Error(\"auto-gpt\" + ' 第 4 列是：' + got + '；期望：' + \"gpt-5.6-sol 跟随打分 medium～xhigh · subscription → api 超出模型能力，实际按 high 发送\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-chat\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-chat\"); const got = row.querySelectorAll('td')[3].innerText.replace(/\\s+/g, ' ').trim(); if (got !== \"gpt-5.6-sol 固定 high · subscription → api\") throw new Error(\"auto-chat\" + ' 第 4 列是：' + got + '；期望：' + \"gpt-5.6-sol 固定 high · subscription → api\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => r.textContent.includes('超出模型能力')); const td = row.querySelectorAll('td')[3]; const span = [...td.querySelectorAll('span')].find((s) => s.textContent.trim() === '跟随打分 medium～xhigh'); if (!span || !span.className.includes('text-warning')) throw new Error('力度文字没有标黄'); const tip = [...td.querySelectorAll('div')].find((d) => d.textContent.trim() === '超出模型能力，实际按 high 发送'); if (!tip || !tip.className.includes('text-warning')) throw new Error('提示不是黄色小字'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "override-warning"
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
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"gpt-5.6-sol\"); if (!r) throw new Error('能力表里没有 ' + \"gpt-5.6-sol\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"gpt-5.6-sol\", \"已覆盖\", \"none low medium high\", \"修改 恢复内置\"])) throw new Error(\"gpt-5.6-sol\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const got = row.querySelectorAll('td')[3].innerText.replace(/\\s+/g, ' ').trim(); if (got !== \"gpt-5.6-sol 跟随打分 medium～xhigh · subscription → api 超出模型能力，实际按 high 发送\") throw new Error(\"auto-gpt\" + ' 第 4 列是：' + got + '；期望：' + \"gpt-5.6-sol 跟随打分 medium～xhigh · subscription → api 超出模型能力，实际按 high 发送\"); return 'ok'; }",
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
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "hover",
      "selector": "role=dialog >> role=tab[name=/^困难/]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.6-sol · 跟随打分 medium～xhigh"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^困难/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"最高\"s]"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"high\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; if (JSON.stringify(got) !== JSON.stringify([\"none\", \"low\", \"medium\", \"high\"])) problems.push('选项是 ' + got.join(' / ') + '，期望 ' + \"none / low / medium / high\"); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    },
    {
      "action": "click",
      "selector": "role=group[name=\"gpt-5.6-sol 的操作\"s] >> role=button[name=\"恢复内置\"s]"
    },
    {
      "action": "waitFor",
      "text": "保存成功"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"gpt-5.6-sol\"); if (!r) throw new Error('能力表里没有 ' + \"gpt-5.6-sol\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"gpt-5.6-sol\", \"内置\", \"none low medium high xhigh max\", \"覆盖\"])) throw new Error(\"gpt-5.6-sol\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { if (document.body.innerText.includes('超出模型能力')) throw new Error('恢复内置后仍有标黄提示'); return 'ok'; }",
      "expect": "ok"
    },
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
      "script": "() => { const card = document.querySelector('section[aria-label=\"模型思考能力\"]'); if (!card) throw new Error('没有「模型思考能力」卡片'); const rows = [...card.querySelectorAll('tbody tr')]; const cells = (r) => [...r.querySelectorAll('td')].map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); const rowOf = (model) => rows.find((r) => cells(r)[0] === model); const r = rowOf(\"gpt-5.6-sol\"); if (!r) throw new Error('能力表里没有 ' + \"gpt-5.6-sol\"); const got = cells(r); if (JSON.stringify(got) !== JSON.stringify([\"gpt-5.6-sol\", \"内置\", \"none low medium high xhigh max\", \"覆盖\"])) throw new Error(\"gpt-5.6-sol\" + ' 这一行是：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "override-restored"
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
- [ ] 覆盖弹窗标题「覆盖 gpt-5.6-sol 的思考能力」，说明「勾选这个模型支持的思考力度，保存后立即生效。」，勾选框初值是 none low medium high xhigh max；打开「不支持思考」后七个勾选框都变灰，关掉后恢复
- [ ] 去掉 xhigh、max 保存后出现「保存成功」，这一行来源「已覆盖」（主色），档位 none low medium high，操作「修改」「恢复内置」
- [ ] auto-gpt 困难档力度文字标黄，下面黄色小字「超出模型能力，实际按 high 发送」；auto-chat 困难档（固定 high）不标黄
- [ ] 刷新后仍是已覆盖；编辑弹窗里困难档悬停显示存的值「跟随打分 medium～xhigh」，「最高」下拉只列 none low medium high
- [ ] 点「恢复内置」后出现「保存成功」，回到内置值、标黄消失；刷新后仍是内置
