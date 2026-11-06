---
name: editor-save-errors
description: 打分模型没价格、普通档渠道拒收所勾格式时保存：自动切到「打分」，红点和「1 个错误」在打分、档位、普通上；名称重名只在名称下写红字
category: smart-routing-editor
---
# 保存出错时的红点和跳转
## 简介
编辑 auto-gpt，把打分模型换成没有价格的 scorer-mini，接口勾上 Anthropic（普通档 gpt-5.5 在 subscription 里只有 Codex 渠道），停在「顾问」Tab 点保存；核对自动跳转、红点、错误数和红字。再把名称改成已存在的 auto-chat 保存，核对红字只在名称下面。
## 测试目的
二期 PRD 04 验收 C11：构造「打分模型没价格 + 普通档渠道拒收所勾格式」→ 切到「打分」，红点与「1 个错误」出现在「打分」「档位」和「普通」上；名称重名 → 红字在名称下面，左栏没有红点。同时覆盖 PRD 03 主路径里普通档分组一行的红字。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-save-errors",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-chat\", \"endpoints\": [\"openai\"], \"tiers\": {\"simple\": {\"model\": \"gpt-4.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "selector": "role=dialog >> role=combobox[name=\"打分模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"打分模型\"s]",
      "value": "scorer-mini"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"scorer-mini\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"Anthropic\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^顾问/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "scorer-mini 还没有设置价格，请先到「模型定价」里设置"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const problems = []; if (tabOf('打分').getAttribute('aria-selected') !== 'true') problems.push('没有自动切到「打分」'); for (const name of ['打分', '档位']) { const t = tabOf(name); if (!t.querySelector('.bg-destructive') || !t.textContent.includes('1 个错误')) problems.push('「' + name + '」没有红点或「1 个错误」'); } const adv = tabOf('顾问'); if (adv.querySelector('.bg-destructive') || adv.textContent.includes('个错误')) problems.push('「顾问」不应有错误'); const box = dlg.querySelector('[role=combobox][aria-label=\"打分模型\"]'); const msg = [...dlg.querySelectorAll('[data-slot=form-message]')].find((p) => p.textContent.includes('scorer-mini 还没有设置价格')); if (!msg || !msg.closest('[data-slot=form-item]').contains(box)) problems.push('价格红字不在打分模型下面'); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "save-errors-scoring"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const tiers = [...dlg.querySelectorAll('[role=tablist][aria-label=\"档位\"] [role=tab]')]; const marks = tiers.map((t) => [t.textContent.trim().slice(0, 2), !!t.querySelector('.bg-destructive'), t.textContent.includes('1 个错误')]); const want = [['简单', false, false], ['普通', true, true], ['困难', false, false]]; if (JSON.stringify(marks) !== JSON.stringify(want)) throw new Error('三档的错误标记：' + JSON.stringify(marks)); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^普通/]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Anthropic 格式"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = [...document.querySelectorAll('[role=dialog] [role=tabpanel] [role=tabpanel]')].find((p) => !p.hidden && p.offsetParent !== null); const row = panel.querySelector('[role=group][aria-label=\"分组顺序\"]'); if (row.getAttribute('aria-invalid') !== 'true') throw new Error('普通档分组这一行没有描红'); const text = 'gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Anthropic 格式'; const msg = [...row.closest('[data-slot=form-item]').querySelectorAll('[data-slot=form-message]')].find((p) => p.textContent.trim() === text); if (!msg) throw new Error('普通档分组下面没有红字：' + text); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "save-errors-normal"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
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
      "action": "fill",
      "selector": "role=dialog >> role=textbox[name=/^名称/]",
      "value": "auto-chat"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "已经有叫 auto-chat 的虚拟模型"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const name = dlg.querySelector('input[name=name]') || [...dlg.querySelectorAll('input')][0]; const msg = [...dlg.querySelectorAll('[data-slot=form-message]')].find((p) => p.textContent.trim() === '已经有叫 auto-chat 的虚拟模型'); if (!msg || !msg.closest('[data-slot=form-item]').contains(name)) throw new Error('重名红字不在名称下面'); const marked = [...dlg.querySelectorAll('[role=tab]')].filter((t) => t.querySelector('.bg-destructive') || t.textContent.includes('个错误')).map((t) => t.textContent.trim()); if (marked.length) throw new Error('名称出错时不应有 Tab 带红点：' + marked.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "save-errors-name"
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
- [ ] 保存后弹窗不关，从「顾问」自动切到「打分」，打分模型下面红字「scorer-mini 还没有设置价格，请先到「模型定价」里设置」
- [ ] 左栏「打分」「档位」带红点和「1 个错误」，「顾问」没有；档位里只有「普通」带红点和「1 个错误」
- [ ] 普通档分组一行描红，红字「gpt-5.5 在分组 subscription 里只有 ChatGPT Subscription (Codex) 渠道，接收不了 Anthropic 格式」
- [ ] 名称改成 auto-chat 保存，红字「已经有叫 auto-chat 的虚拟模型」在名称下面，左栏没有红点
