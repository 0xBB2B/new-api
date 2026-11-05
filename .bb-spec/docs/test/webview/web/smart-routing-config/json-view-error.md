---
name: json-view-error
description: 在 JSON 视图里把困难档模型改成在 api 分组没有渠道的模型，点「保存更改」报同样的错误
category: smart-routing-config
---
# JSON 视图保存出错
## 简介
切到「JSON」视图，把整份配置改成困难档用 gpt-5.6-terra 的版本，点右上角「保存更改」。
## 测试目的
PRD 05-config-page 验收 C9：JSON 视图保存时按 C7 同样检查，报「gpt-5.6-terra 在分组 api 里没有可用渠道」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "json-view-error",
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
      "selector": "role=tab[name=\"JSON\"s]"
    },
    {
      "action": "waitFor",
      "selector": "role=textbox[name=\"智能路由\"s]"
    },
    {
      "action": "fill",
      "selector": "role=textbox[name=\"智能路由\"s]",
      "value": "[\n  {\n    \"name\": \"auto-gpt\",\n    \"tiers\": {\n      \"simple\": {\n        \"model\": \"gpt-5.4-mini\",\n        \"groups\": [\n          \"api\"\n        ],\n        \"effort\": \"low\"\n      },\n      \"normal\": {\n        \"model\": \"gpt-5.5\",\n        \"groups\": [\n          \"subscription\",\n          \"api\"\n        ],\n        \"effort\": \"medium\"\n      },\n      \"hard\": {\n        \"model\": \"gpt-5.6-terra\",\n        \"groups\": [\n          \"subscription\",\n          \"api\"\n        ],\n        \"effort\": \"high\"\n      }\n    },\n    \"default_tier\": \"normal\",\n    \"scorer\": {\n      \"model\": \"gpt-5.6-luna\",\n      \"groups\": [\n        \"subscription\",\n        \"api\"\n      ],\n      \"timeout_ms\": 2000\n    },\n    \"advisor\": {\n      \"model\": \"gpt-5.6-sol\",\n      \"groups\": [\n        \"subscription\",\n        \"api\"\n      ],\n      \"after_dissatisfied\": 2\n    },\n    \"downgrade_max_history_tokens\": 20000,\n    \"confidence_threshold\": 0.6,\n    \"simple_max_message_chars\": 2000\n  }\n]"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"保存更改\"s]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.6-terra 在分组 api 里没有可用渠道"
    },
    {
      "action": "screenshot",
      "name": "json-view-error"
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
- [ ] 点「保存更改」后提示里写「gpt-5.6-terra 在分组 api 里没有可用渠道」（和弹窗里的中文提示相同），配置没有保存
