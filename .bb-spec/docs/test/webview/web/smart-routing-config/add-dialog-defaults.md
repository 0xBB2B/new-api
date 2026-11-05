---
name: add-dialog-defaults
description: 添加弹窗里思考力度、默认档、超时、连续不满意次数和高级设置三个数字都是默认值
category: smart-routing-config
---
# 添加弹窗的默认值
## 简介
打开「添加虚拟模型」弹窗，不做任何修改，逐项读各字段的初始值。
## 测试目的
PRD 05-config-page 验收 C5：思考力度「不覆盖」、默认档「普通档」、超时 2000、连续不满意次数 2、降档上限 20000、可信度门槛 0.6、简单题长度上限 2000；接口提示「选好模型后自动判断」；思考力度选项是「不覆盖 / low / medium / high」。
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const res = await (await fetch('/api/option/request_policy', {method: 'PATCH', headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}, body: JSON.stringify({options: {'smart_routing_setting.routes': \"[]\"}})})).json(); if (!res.success) throw new Error('写入智能路由配置失败：' + res.message); return 'ok'; }",
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
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"高级设置\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const field = (text) => { const label = [...dlg.querySelectorAll('label')].find((l) => l.textContent.replace(/[\\s*]/g, '') === text); if (!label) throw new Error('找不到字段「' + text + '」'); const el = document.getElementById(label.htmlFor); if (!el) throw new Error('字段「' + text + '」没有输入框'); return el; }; const problems = []; const endpoint = field('接口'); if (endpoint.placeholder !== '选好模型后自动判断' || endpoint.value !== '' || !endpoint.readOnly) problems.push('接口：placeholder=' + endpoint.placeholder + ' value=' + endpoint.value + ' readOnly=' + endpoint.readOnly); const efforts = [...dlg.querySelectorAll('[role=combobox][aria-label=\"思考力度\"]')].map((e) => e.textContent.trim()); if (JSON.stringify(efforts) !== JSON.stringify(['不覆盖', '不覆盖', '不覆盖'])) problems.push('思考力度：' + efforts.join(',')); const tier = dlg.querySelector('[role=combobox][aria-label=\"默认档\"]'); if (!tier || tier.textContent.trim() !== '普通档') problems.push('默认档：' + (tier && tier.textContent.trim())); const numbers = {'超时（毫秒）': '2000', '连续不满意几次后交给顾问': '2', '降档时历史token上限': '20000', '可信度门槛': '0.6', '简单题消息长度上限（字）': '2000'}; for (const [name, want] of Object.entries(numbers)) { const v = field(name).value; if (v !== want) problems.push(name + '=' + v + '，期望 ' + want); } const models = [...dlg.querySelectorAll('input[placeholder=\"请选择模型\"]')].map((e) => e.value); if (models.length !== 5 || models.some(Boolean)) problems.push('模型框应为 5 个空框：' + JSON.stringify(models)); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"思考力度\"s] >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"low\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const want = ['不覆盖', 'low', 'medium', 'high']; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('思考力度选项：' + got.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "screenshot",
      "name": "add-dialog-defaults"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 接口框只读、为空，占位文字是「选好模型后自动判断」
- [ ] 三个思考力度都是「不覆盖」，选项依次是 不覆盖 / low / medium / high
- [ ] 默认档是「普通档」，超时 2000、连续不满意次数 2
- [ ] 展开高级设置后：降档上限 20000、可信度门槛 0.6、简单题长度上限 2000
