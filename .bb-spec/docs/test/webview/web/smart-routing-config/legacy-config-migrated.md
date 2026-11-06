---
name: legacy-config-migrated
description: 用一期格式的旧配置启动后，配置页显示困难档「固定 high」、接口只勾 Responses
category: smart-routing-config
---
# 一期配置升级后的显示
## 简介
seed 脚本把一期格式的 auto-gpt（困难档只有 effort=high，没有接口字段）直接写进数据库并重启后端，启动时的迁移把它改成新格式。本用例先读接口确认迁移结果，再打开智能路由页和编辑弹窗核对显示。本用例必须在执行 seed 之后第一个跑，其他用例会改掉这份配置。
## 测试目的
二期 PRD 01 验收 C10（一期保存的困难档 effort=high 升级后显示「固定 high」，不需要重新保存）、PRD 03 验收 C2（一期配置升级后接口只勾 Responses）。「请求按固定 high 发送」「Responses 请求照常路由」要抓上游请求，不在本用例里。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "legacy-config-migrated",
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
    }
  ],
  "steps": [
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; const res = await call('GET', '/api/option/request_policy'); const routes = JSON.parse(res.data.options['smart_routing_setting.routes'] || '[]'); const r = routes.find((x) => x.name === 'auto-gpt'); if (!r || !r.tiers || !r.tiers.hard || r.tiers.hard.effort !== 'high') throw new Error('没有找到 seed 写入的一期配置（可能被其他用例改掉了），请重新执行 seed 脚本后先跑本用例。当前配置：' + JSON.stringify(routes)); const problems = []; if (JSON.stringify(r.endpoints) !== JSON.stringify(['openai-response'])) problems.push('endpoints=' + JSON.stringify(r.endpoints)); if (r.tiers.hard.effort_mode !== 'fixed') problems.push('困难档 effort_mode=' + r.tiers.hard.effort_mode); if (r.tiers.simple.effort_mode !== '' || r.tiers.normal.effort_mode !== '') problems.push('简单档、普通档 effort_mode 应为空：' + r.tiers.simple.effort_mode + ' / ' + r.tiers.normal.effort_mode); if (problems.length) throw new Error('启动迁移的结果不对：' + problems.join('；')); return 'ok'; }",
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
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim().split('\\n')[0].trim() === \"auto-gpt\"; }); if (!row) throw new Error('表格里没有 ' + \"auto-gpt\"); const want = [\"auto-gpt Responses 接口\", \"gpt-5.4-mini 不覆盖 · api\", \"gpt-5.5 默认 不覆盖 · subscription → api\", \"gpt-5.6-sol 固定 high · subscription → api\", \"gpt-5.6-luna subscription → api · 超时 2000 毫秒\", \"gpt-5.6-sol 连续 2 次不满意\"]; const got = [...row.querySelectorAll('td')].slice(0, want.length).map((td) => td.innerText.replace(/\\s+/g, ' ').trim()); if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(\"auto-gpt\" + ' 这一行：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "legacy-table"
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
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const pressed = ['Responses', 'Chat', 'Anthropic'].filter((name) => { const b = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')].find((x) => x.textContent.trim() === name); if (!b) throw new Error('没有接口按钮 ' + name); return b.getAttribute('aria-pressed') === 'true'; }); if (JSON.stringify(pressed) !== JSON.stringify(['Responses'])) throw new Error('接口选中的是：' + pressed.join(',')); return 'ok'; }",
      "expect": "ok"
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
      "action": "waitFor",
      "selector": "role=dialog >> role=combobox[name=\"思考力度\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const group = dlg.querySelector('[role=group][aria-label=\"思考力度方式\"]'); if (!group) throw new Error(\"困难档\" + '没有思考力度方式按钮'); const got = [...group.querySelectorAll('button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(got) !== JSON.stringify([\"固定\"])) throw new Error(\"困难档\" + '思考力度选中的是：' + got.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const box = document.querySelector('[role=dialog] [role=combobox][aria-label=\"思考力度\"]'); if (!box || box.textContent.trim() !== 'high') throw new Error('困难档固定力度显示：' + (box && box.textContent.trim())); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "legacy-dialog-hard"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 接口返回的配置里 auto-gpt 的 endpoints 是 ["openai-response"]，困难档 effort_mode 是 fixed，简单档、普通档 effort_mode 为空
- [ ] 表格里 auto-gpt 名称下写「Responses 接口」，困难档第二行是「固定 high · subscription → api」
- [ ] 编辑弹窗里接口只选中 Responses；困难档思考力度选中「固定」，下拉显示 high
