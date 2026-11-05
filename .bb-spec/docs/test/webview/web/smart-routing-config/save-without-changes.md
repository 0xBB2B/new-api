---
name: save-without-changes
description: 已有 auto-gpt、没做任何修改时点「保存更改」，提示没有需要保存的更改
category: smart-routing-config
---
# 没有改动时保存更改
## 简介
打开已有 auto-gpt 的智能路由页，不做任何修改，直接点右上角「保存更改」。
## 测试目的
PRD 05-config-page 验收 C8：没有改动时点「保存更改」，提示「没有需要保存的更改」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "save-without-changes",
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
      "selector": "role=button[name=\"保存更改\"s]"
    },
    {
      "action": "waitFor",
      "text": "没有需要保存的更改"
    },
    {
      "action": "screenshot",
      "name": "save-without-changes"
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
- [ ] 点「保存更改」后出现提示「没有需要保存的更改」
