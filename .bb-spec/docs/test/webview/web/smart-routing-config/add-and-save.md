---
name: add-and-save
description: 在添加弹窗的三个 Tab 里配好 auto-gpt，弹窗保存后点「保存更改」，表格出现这一行且刷新后仍在
category: smart-routing-config
---
# 添加 auto-gpt 并保存更改
## 简介
从空配置开始，在添加弹窗的「打分」「档位」（简单、普通、困难）「顾问」三个 Tab 里依次配好模型和分组顺序，保存弹窗后再点页面右上角「保存更改」，最后刷新页面确认配置已生效。
## 测试目的
一期 PRD 05-config-page 验收 C3（表格各列两行文字、默认档带「默认」标签）、C8（点了「保存更改」才生效），以及主路径用例：接口列显示 Responses。档位列第二行按二期 PRD 04 C13 的写法核对：「<力度摘要> · <分组顺序>」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "add-and-save",
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
      "action": "fill",
      "selector": "role=dialog >> role=textbox[name=/^名称/]",
      "value": "auto-gpt"
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
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"subscription\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
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
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
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
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"subscription\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
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
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"subscription\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
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
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"subscription\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const pressed = ['Responses', 'Chat', 'Anthropic'].filter((name) => { const b = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')].find((x) => x.textContent.trim() === name); if (!b) throw new Error('没有接口按钮 ' + name); return b.getAttribute('aria-pressed') === 'true'; }); if (JSON.stringify(pressed) !== JSON.stringify(['Responses'])) throw new Error('接口选中的是：' + pressed.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "add-and-save-dialog"
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
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const want = [\"auto-gpt Responses 接口\", \"gpt-5.4-mini 不覆盖 · api\", \"gpt-5.5 默认 不覆盖 · subscription → api\", \"gpt-5.6-sol 不覆盖 · subscription → api\", \"gpt-5.6-luna subscription → api · 超时 2000 毫秒\", \"gpt-5.6-sol 连续 2 次不满意\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-gpt\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"保存更改\"s]"
    },
    {
      "action": "waitFor",
      "text": "保存成功"
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
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const want = [\"auto-gpt Responses 接口\", \"gpt-5.4-mini 不覆盖 · api\", \"gpt-5.5 默认 不覆盖 · subscription → api\", \"gpt-5.6-sol 不覆盖 · subscription → api\", \"gpt-5.6-luna subscription → api · 超时 2000 毫秒\", \"gpt-5.6-sol 连续 2 次不满意\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-gpt\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "add-and-save-after-reload"
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
- [ ] 弹窗里接口只选中 Responses
- [ ] 弹窗保存后弹窗关闭，表格出现 auto-gpt 一行：名称下写「Responses 接口」，三档第二行是「不覆盖 · <分组顺序>」，「默认」标签只在普通档
- [ ] 点「保存更改」提示「保存成功」，刷新后这一行仍在
