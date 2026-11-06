---
name: add-dialog-defaults
description: 添加弹窗里接口、三档思考力度、默认档、超时、连续不满意次数和高级设置三个数字都是默认值
category: smart-routing-config
---
# 添加弹窗的默认值
## 简介
打开「添加虚拟模型」弹窗，不做任何修改，依次在「打分」「档位」（简单、普通、困难）「顾问」三个 Tab 和展开的高级设置里读各字段的初始值。
## 测试目的
一期 PRD 05-config-page 验收 C5 的默认值（默认档「普通档」、超时 2000、连续不满意次数 2、降档上限 20000、可信度门槛 0.6、简单题长度上限 2000），按二期改版后的弹窗核对；同时覆盖二期 PRD 01 验收 C1 的前半句：新建时三档思考力度都是「不覆盖」。一期的「接口框只读、选好模型后自动判断」和「思考力度选项 不覆盖 / low / medium / high」已被二期 PRD 03 C1、PRD 01 C1 取代，改由 smart-routing-endpoints、smart-routing-editor 下的用例覆盖。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "add-dialog-defaults",
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
      "action": "assertText",
      "selector": "role=dialog",
      "text": "添加虚拟模型"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const field = (text) => { const label = [...dlg.querySelectorAll('label')].find((l) => l.textContent.replace(/[\\s*]/g, '') === text); if (!label) throw new Error('找不到字段「' + text + '」'); const el = document.getElementById(label.htmlFor); if (!el) throw new Error('字段「' + text + '」没有输入框'); return el; }; const pressed = ['Responses', 'Chat', 'Anthropic'].filter((name) => { const b = [...dlg.querySelectorAll('[role=group][aria-label=\"接口\"] button')].find((x) => x.textContent.trim() === name); if (!b) throw new Error('没有接口按钮 ' + name); return b.getAttribute('aria-pressed') === 'true'; }); const problems = []; if (JSON.stringify(pressed) !== JSON.stringify(['Responses'])) problems.push('接口选中的是：' + pressed.join(',')); const timeout = field('超时（毫秒）').value; if (timeout !== '2000') problems.push('超时=' + timeout); const scorer = dlg.querySelector('[role=combobox][aria-label=\"打分模型\"]'); if (!scorer || scorer.value !== '') problems.push('打分模型框应为空'); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const box = document.querySelector('[role=dialog] [role=combobox][aria-label=\"默认档\"]'); if (!box || box.textContent.trim() !== '普通档') throw new Error('默认档：' + (box && box.textContent.trim())); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const group = dlg.querySelector('[role=group][aria-label=\"思考力度方式\"]'); if (!group) throw new Error(\"简单档\" + '没有思考力度方式按钮'); const got = [...group.querySelectorAll('button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(got) !== JSON.stringify([\"不覆盖\"])) throw new Error(\"简单档\" + '思考力度选中的是：' + got.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^普通/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const group = dlg.querySelector('[role=group][aria-label=\"思考力度方式\"]'); if (!group) throw new Error(\"普通档\" + '没有思考力度方式按钮'); const got = [...group.querySelectorAll('button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(got) !== JSON.stringify([\"不覆盖\"])) throw new Error(\"普通档\" + '思考力度选中的是：' + got.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^困难/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const group = dlg.querySelector('[role=group][aria-label=\"思考力度方式\"]'); if (!group) throw new Error(\"困难档\" + '没有思考力度方式按钮'); const got = [...group.querySelectorAll('button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.textContent.trim()); if (JSON.stringify(got) !== JSON.stringify([\"不覆盖\"])) throw new Error(\"困难档\" + '思考力度选中的是：' + got.join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^顾问/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const field = (text) => { const label = [...dlg.querySelectorAll('label')].find((l) => l.textContent.replace(/[\\s*]/g, '') === text); if (!label) throw new Error('找不到字段「' + text + '」'); const el = document.getElementById(label.htmlFor); if (!el) throw new Error('字段「' + text + '」没有输入框'); return el; }; const v = field('连续不满意几次后交给顾问').value; if (v !== '2') throw new Error('连续不满意次数=' + v); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"高级设置\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const field = (text) => { const label = [...dlg.querySelectorAll('label')].find((l) => l.textContent.replace(/[\\s*]/g, '') === text); if (!label) throw new Error('找不到字段「' + text + '」'); const el = document.getElementById(label.htmlFor); if (!el) throw new Error('字段「' + text + '」没有输入框'); return el; }; const numbers = {'降档时历史token上限': '20000', '可信度门槛': '0.6', '简单题消息长度上限（字）': '2000'}; const problems = []; for (const [name, want] of Object.entries(numbers)) { const v = field(name).value; if (v !== want) problems.push(name + '=' + v + '，期望 ' + want); } if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "add-dialog-defaults"
    },
    {
      "action": "press",
      "key": "Escape"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 接口只选中 Responses；打分 Tab 的超时是 2000，打分模型为空
- [ ] 档位 Tab 的默认档是「普通档」，简单、普通、困难三档的思考力度都选中「不覆盖」
- [ ] 顾问 Tab 的连续不满意次数是 2
- [ ] 展开高级设置后：降档上限 20000、可信度门槛 0.6、简单题长度上限 2000
