---
name: delete-with-confirm
description: 从「…」菜单删除 auto-gpt，先弹确认框，确认后这一行消失，保存更改后刷新仍为空
category: smart-routing-config
---
# 从菜单删除虚拟模型
## 简介
在 auto-gpt 这一行打开「…」菜单点「删除」，在确认框里点「删除」，再保存更改并刷新。
## 测试目的
PRD 05-config-page 验收 C3：从「…」菜单删除时先弹确认框，确认后这一行消失。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "delete-with-confirm",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const res = await (await fetch('/api/option/request_policy', {method: 'PATCH', headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}, body: JSON.stringify({options: {'smart_routing_setting.routes': \"[{\\\"name\\\":\\\"auto-gpt\\\",\\\"tiers\\\":{\\\"simple\\\":{\\\"model\\\":\\\"gpt-5.4-mini\\\",\\\"groups\\\":[\\\"api\\\"],\\\"effort\\\":\\\"low\\\"},\\\"normal\\\":{\\\"model\\\":\\\"gpt-5.5\\\",\\\"groups\\\":[\\\"subscription\\\",\\\"api\\\"],\\\"effort\\\":\\\"medium\\\"},\\\"hard\\\":{\\\"model\\\":\\\"gpt-5.6-sol\\\",\\\"groups\\\":[\\\"subscription\\\",\\\"api\\\"],\\\"effort\\\":\\\"high\\\"}},\\\"default_tier\\\":\\\"normal\\\",\\\"scorer\\\":{\\\"model\\\":\\\"gpt-5.6-luna\\\",\\\"groups\\\":[\\\"subscription\\\",\\\"api\\\"],\\\"timeout_ms\\\":2000},\\\"advisor\\\":{\\\"model\\\":\\\"gpt-5.6-sol\\\",\\\"groups\\\":[\\\"subscription\\\",\\\"api\\\"],\\\"after_dissatisfied\\\":2},\\\"downgrade_max_history_tokens\\\":20000,\\\"confidence_threshold\\\":0.6,\\\"simple_max_message_chars\\\":2000}]\"}})})).json(); if (!res.success) throw new Error('写入智能路由配置失败：' + res.message); return 'ok'; }",
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
      "selector": "role=group[name=\"auto-gpt 的操作\"s] >> role=button[name=\"更多操作\"s]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"删除\"s]"
    },
    {
      "action": "waitFor",
      "text": "确定删除 auto-gpt 吗？点「保存更改」后生效。"
    },
    {
      "action": "assertText",
      "selector": "role=alertdialog",
      "text": "删除虚拟模型"
    },
    {
      "action": "screenshot",
      "name": "delete-confirm"
    },
    {
      "action": "click",
      "selector": "role=alertdialog >> role=button[name=\"删除\"s]"
    },
    {
      "action": "waitFor",
      "text": "还没有虚拟模型"
    },
    {
      "action": "evaluate",
      "script": "() => { if ([...document.querySelectorAll('tbody tr')].some((r) => r.textContent.includes('auto-gpt'))) throw new Error('auto-gpt 还在表格里'); return 'ok'; }",
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
      "text": "还没有虚拟模型"
    },
    {
      "action": "screenshot",
      "name": "delete-after-reload"
    }
  ],
  "teardown": [
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const res = await (await fetch('/api/option/request_policy', {method: 'PATCH', headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}, body: JSON.stringify({options: {'smart_routing_setting.routes': \"[]\"}})})).json(); if (!res.success) throw new Error('写入智能路由配置失败：' + res.message); return 'ok'; }",
      "expect": "ok"
    }
  ]
}
```
## 如何验收
- [ ] 点「删除」先弹出标题「删除虚拟模型」的确认框，写「确定删除 auto-gpt 吗？点「保存更改」后生效。」
- [ ] 确认后 auto-gpt 这一行消失、显示空状态
- [ ] 保存更改后刷新仍是空状态
