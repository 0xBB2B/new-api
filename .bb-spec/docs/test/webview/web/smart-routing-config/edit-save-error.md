---
name: edit-save-error
description: 困难档换成在 api 分组没有渠道的模型，保存时字段下出现红字、自动跳到出错的档位、Tab 带红点；改回原模型后保存成功
category: smart-routing-config
---
# 编辑时保存出错
## 简介
编辑 auto-gpt，在「档位 → 困难」把模型换成只在 subscription 分组有渠道的 gpt-5.6-terra（分组顺序仍是 subscription → api），切回「打分」Tab 后点弹窗里的「保存」；看到报错后改回 gpt-5.6-sol 再保存。
## 测试目的
一期 PRD 05-config-page 验收 C7 和边界用例：模型在所配分组里没有可用渠道时，弹窗不关，困难档模型下面红字写「gpt-5.6-terra 在分组 api 里没有可用渠道」；改正后保存，弹窗关闭、表格更新。同时检查出错的模型框仍显示模型名、下拉没有自动展开。出错的样子按二期 PRD 04 C11 核对：自动切到「档位 → 困难」，「档位」和「困难」都带红点和「1 个错误」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "edit-save-error",
  "category": "smart-routing-config",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"fixed\", \"effort\": \"low\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"medium\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "value": "gpt-5.6-terra"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.6-terra\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^打分/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.6-terra 在分组 api 里没有可用渠道"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); if (!dlg) throw new Error('弹窗关闭了'); const tabs = [...dlg.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => tabs.find((t) => t.textContent.trim().startsWith(prefix)); const problems = []; if (tabOf('档位').getAttribute('aria-selected') !== 'true') problems.push('没有自动切到「档位」'); if (tabOf('困难').getAttribute('aria-selected') !== 'true') problems.push('没有自动切到「困难」'); for (const name of ['档位', '困难']) { const t = tabOf(name); if (!t.querySelector('.bg-destructive') || !t.textContent.includes('1 个错误')) problems.push('「' + name + '」没有红点或「1 个错误」'); } for (const name of ['打分', '顾问', '简单', '普通']) { if (tabOf(name).textContent.includes('个错误')) problems.push('「' + name + '」不应带错误数'); } const box = dlg.querySelector('[role=combobox][aria-label=\"困难档模型\"]'); const msg = [...dlg.querySelectorAll('p')].find((p) => p.textContent.trim() === 'gpt-5.6-terra 在分组 api 里没有可用渠道'); if (!msg) problems.push('没有红字提示'); else { if (!msg.classList.contains('text-destructive')) problems.push('提示不是红字：' + msg.className); const block = msg.closest('[data-slot=form-item]') || msg.parentElement; if (!block.contains(box)) problems.push('红字不在困难档模型字段下面'); } if (box.value !== 'gpt-5.6-terra') problems.push('困难档模型框显示：' + box.value); if (box.getAttribute('aria-expanded') === 'true') problems.push('出错后模型下拉自动展开了'); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "edit-save-error"
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
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "assertText",
      "selector": "table",
      "text": "gpt-5.6-sol"
    },
    {
      "action": "screenshot",
      "name": "edit-save-fixed"
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
- [ ] 保存后弹窗不关，自动切到「档位 → 困难」，困难档模型下面红字「gpt-5.6-terra 在分组 api 里没有可用渠道」
- [ ] 左栏「档位」和档位里的「困难」带红点和「1 个错误」，其他 Tab 没有错误数
- [ ] 出错的模型框仍显示 gpt-5.6-terra，下拉没有自动展开
- [ ] 改回 gpt-5.6-sol 再保存，弹窗关闭
