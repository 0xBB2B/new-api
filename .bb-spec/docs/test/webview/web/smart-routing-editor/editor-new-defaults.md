---
name: editor-new-defaults
description: 新建虚拟模型停在「打分」、接口只选 Responses、分组只有「＋ 添加分组」；顶部只有名称和接口；左栏摘要随填写变化；高级设置在最底部默认收起
category: smart-routing-editor
---
# 新建弹窗的布局与左栏摘要
## 简介
从空配置打开「添加虚拟模型」，核对弹窗宽度、顶部一行、左栏三个 Tab 的初始状态和高级设置的位置；再依次填打分模型、三档模型、顾问模型，看左栏摘要的变化。最后配好一个 auto-gpt 并刷新，从表格的编辑图标进入，确认同样停在「打分」。
## 测试目的
二期 PRD 04 验收 C10（新建停在「打分」、接口只选 Responses、分组只显示「＋ 添加分组」；编辑时从表格编辑图标进入也停在「打分」）、C2 前半（左栏摘要：选了打分模型后变成模型名，档位从「0/3 已设置」变到「3/3 已设置」）、C1 前半（弹窗用 max-w-5xl，顶部只有名称和接口）、C8 前半（高级设置默认收起，在弹窗最底部、横跨两栏）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-new-defaults",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "text": "还没有虚拟模型"
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
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const problems = []; if (!dlg.className.includes('sm:max-w-5xl')) problems.push('弹窗宽度不是 max-w-5xl：' + dlg.className); const form = dlg.querySelector('form'); const top = form.firstElementChild; const topLabels = [...top.querySelectorAll('[data-slot=form-label]')].map((l) => l.textContent.replace(/[\\s*]/g, '')); if (JSON.stringify(topLabels) !== JSON.stringify(['名称', '接口'])) problems.push('顶部一行是：' + topLabels.join(' / ')); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); const tabs = [...left.querySelectorAll('[role=tab]')]; const names = tabs.map((t) => t.innerText.split('\\n').map((s) => s.trim()).filter(Boolean)); if (JSON.stringify(names) !== JSON.stringify([['打分', '未设置'], ['档位', '0/3 已设置'], ['顾问', '未设置']])) problems.push('左栏是：' + JSON.stringify(names)); if (tabs[0].getAttribute('aria-selected') !== 'true') problems.push('没有停在「打分」'); const pressed = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(pressed) !== JSON.stringify(['Responses'])) problems.push('接口选中的是：' + pressed.join(',')); const panel = dlg.querySelector('[role=tabpanel]'); if (panel.querySelector('li[aria-keyshortcuts]')) problems.push('分组顺序里已经有分组'); const add = panel.querySelector('[role=combobox][aria-label=\"添加分组\"]'); if (!add || add.innerText.trim() !== '添加分组') problems.push('分组顺序没有显示「＋ 添加分组」：' + (add && add.innerText)); const adv = [...dlg.querySelectorAll('button')].find((b) => b.textContent.trim() === '高级设置'); if (!adv) problems.push('没有高级设置'); else { if (adv.getAttribute('aria-expanded') !== 'false') problems.push('高级设置没有默认收起'); if (!(left.compareDocumentPosition(adv) & Node.DOCUMENT_POSITION_FOLLOWING)) problems.push('高级设置不在左右两栏下面'); const tabsBox = left.parentElement.getBoundingClientRect(); const advBox = adv.parentElement.getBoundingClientRect(); if (advBox.top < tabsBox.bottom) problems.push('高级设置不在最底部'); if (advBox.width < form.getBoundingClientRect().width * 0.95) problems.push('高级设置没有横跨两栏'); } if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "editor-new"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"打分模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"打分模型\"s]",
      "value": "gpt-5.6-luna"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.6-luna\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); if (!left) throw new Error('没有左栏 Tab'); const leftTabs = [...left.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const summary = (prefix) => tabOf(prefix).innerText.split('\\n').map((s) => s.trim()).filter(Boolean)[1] || ''; const got = summary(\"打分\"); if (got !== \"gpt-5.6-luna\") throw new Error('「' + \"打分\" + '」的摘要是：' + got + '，期望：' + \"gpt-5.6-luna\"); return 'ok'; }",
      "expect": "ok"
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
      "value": "gpt-5.4-mini"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.4-mini\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); if (!left) throw new Error('没有左栏 Tab'); const leftTabs = [...left.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const summary = (prefix) => tabOf(prefix).innerText.split('\\n').map((s) => s.trim()).filter(Boolean)[1] || ''; const got = summary(\"档位\"); if (got !== \"1/3 已设置\") throw new Error('「' + \"档位\" + '」的摘要是：' + got + '，期望：' + \"1/3 已设置\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^普通/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"普通档模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"普通档模型\"s]",
      "value": "gpt-5.5"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.5\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^困难/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"困难档模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"困难档模型\"s]",
      "value": "gpt-5.6-sol"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.6-sol\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); if (!left) throw new Error('没有左栏 Tab'); const leftTabs = [...left.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const summary = (prefix) => tabOf(prefix).innerText.split('\\n').map((s) => s.trim()).filter(Boolean)[1] || ''; const got = summary(\"档位\"); if (got !== \"3/3 已设置\") throw new Error('「' + \"档位\" + '」的摘要是：' + got + '，期望：' + \"3/3 已设置\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^顾问/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"顾问模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"顾问模型\"s]",
      "value": "gpt-5.6-sol"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.6-sol\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); if (!left) throw new Error('没有左栏 Tab'); const leftTabs = [...left.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const summary = (prefix) => tabOf(prefix).innerText.split('\\n').map((s) => s.trim()).filter(Boolean)[1] || ''; const got = summary(\"顾问\"); if (got !== \"gpt-5.6-sol\") throw new Error('「' + \"顾问\" + '」的摘要是：' + got + '，期望：' + \"gpt-5.6-sol\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "editor-summaries"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "action": "click",
      "selector": "role=group[name=\"auto-gpt 的操作\"s] >> role=button[name=\"编辑\"s]"
    },
    {
      "action": "waitFor",
      "text": "编辑虚拟模型"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); if (!left) throw new Error('没有左栏 Tab'); const leftTabs = [...left.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const summary = (prefix) => tabOf(prefix).innerText.split('\\n').map((s) => s.trim()).filter(Boolean)[1] || ''; if (leftTabs[0].getAttribute('aria-selected') !== 'true') throw new Error('从编辑图标进入没有停在「打分」'); return 'ok'; }",
      "expect": "ok"
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
- [ ] 弹窗用 max-w-5xl，顶部一行只有「名称」和「接口」
- [ ] 新建时停在「打分」，接口只选中 Responses，分组顺序只显示「＋ 添加分组」
- [ ] 左栏摘要：打分「未设置」→「gpt-5.6-luna」；档位「0/3 已设置」→「1/3 已设置」→「3/3 已设置」；顾问「未设置」→「gpt-5.6-sol」
- [ ] 高级设置默认收起，在左右两栏下面、横跨整个弹窗
- [ ] 从表格编辑图标打开已有的 auto-gpt，同样停在「打分」
