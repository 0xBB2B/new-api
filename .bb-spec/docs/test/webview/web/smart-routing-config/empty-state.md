---
name: empty-state
description: 没有虚拟模型时显示空状态的标题、说明和按钮，点按钮打开添加弹窗
category: smart-routing-config
---
# 没有虚拟模型时的空状态
## 简介
智能路由配置为空时打开智能路由页，看卡片里的空状态，再点空状态里的「添加虚拟模型」。
## 测试目的
PRD 05-config-page 验收 C4：显示空状态标题「还没有虚拟模型」、说明和「添加虚拟模型」按钮，点按钮打开添加弹窗；同时核对 C2 的卡片标题、数量 0 和说明文字。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "empty-state",
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
      "action": "assertText",
      "selector": "main >> nth=0",
      "text": "添加一个虚拟模型，比如 auto-gpt。用户在客户端里把模型名填成它就能用。"
    },
    {
      "action": "assertText",
      "selector": "main >> nth=0",
      "text": "每个虚拟模型配好三档模型、打分模型和顾问。用户在客户端里把模型名填成虚拟模型名就能用。"
    },
    {
      "action": "evaluate",
      "script": "() => { const card = document.querySelector('section[aria-label=\"虚拟模型\"]'); if (!card) throw new Error('没有「虚拟模型」卡片'); const badge = card.querySelector('h3').parentElement.innerText.replace(/\\s+/g, ' ').trim(); if (badge !== '虚拟模型 0') throw new Error('卡片标题和数量是：' + badge); const buttons = [...card.querySelectorAll('button')].filter((b) => b.textContent.trim() === '添加虚拟模型'); if (buttons.length !== 2) throw new Error('「添加虚拟模型」按钮有 ' + buttons.length + ' 个，期望顶部和空状态各一个'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "empty-state"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"添加虚拟模型\"s] >> nth=-1"
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
      "action": "screenshot",
      "name": "empty-state-add-dialog"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 卡片标题「虚拟模型」后面的数量是 0
- [ ] 空状态显示标题「还没有虚拟模型」和说明「添加一个虚拟模型，比如 auto-gpt。用户在客户端里把模型名填成它就能用。」
- [ ] 点空状态里的「添加虚拟模型」打开标题为「添加虚拟模型」的弹窗
