---
name: i18n-labels
description: 切到 English 和日本語，智能路由页和添加弹窗的新增文字都有翻译
category: smart-routing-config
---
# 智能路由页的英文和日文翻译
## 简介
在已有 auto-gpt 的智能路由页，用右上角语言菜单切到 English，检查页面和展开高级设置后的添加弹窗；再切到日本語做同样检查；最后切回简体中文。
## 测试目的
PRD 05-config-page 验收 C10：新增文字都有翻译，不出现翻译键或中文残留（本用例覆盖 en、ja 两种语言）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "i18n-labels",
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
      "selector": "role=button[name=\"更改语言\"s]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"English\"s]"
    },
    {
      "action": "waitFor",
      "text": "Each virtual model has three tier models, a scorer model and an advisor."
    },
    {
      "action": "assertText",
      "selector": "main",
      "text": "Responses endpoint"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('main main') || document.querySelector('main'); if (!el) throw new Error('找不到要检查的区域'); const bad = (text) => { const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) return '出现翻译占位：' + keys.join(','); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); return cjk ? '出现中日文字：' + cjk.join(',') : ''; }; const problem = bad(el.innerText); if (problem) throw new Error(problem); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"Add virtual model\"s] >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"Advanced settings\"s]"
    },
    {
      "action": "assertText",
      "selector": "role=dialog",
      "text": "Keep client value"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const bad = (text) => { const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) return '出现翻译占位：' + keys.join(','); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); return cjk ? '出现中日文字：' + cjk.join(',') : ''; }; const problem = bad(el.innerText); if (problem) throw new Error(problem); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-en-dialog"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"Cancel\"s]"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"Change language\"s]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"日本語\"s]"
    },
    {
      "action": "waitFor",
      "text": "スマートルーティング"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('main main') || document.querySelector('main'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const zh = [\"智能路由\", \"虚拟模型\", \"保存更改\", \"可视化\", \"简单档\", \"普通档\", \"困难档\", \"打分模型\", \"顾问\", \"不覆盖\", \"分组顺序\", \"思考力度\", \"默认档\", \"高级设置\", \"次不满意\", \"毫秒\", \"接口\", \"默认\"].filter((s) => text.includes(s)); const en = [\"Smart routing\", \"Virtual models\", \"Add virtual model\", \"Keep client value\", \"Reasoning effort\", \"Group order\", \"Advisor\", \"Scorer model\", \"Default tier\", \"Advanced settings\", \"endpoint\", \"dissatisfied\"].filter((s) => text.includes(s)); const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (zh.length || en.length || keys) throw new Error('中文残留：' + zh.join(',') + '；英文残留：' + en.join(',') + '；占位：' + (keys || []).join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"仮想モデルを追加\"s] >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"詳細設定\"s]"
    },
    {
      "action": "assertText",
      "selector": "role=dialog",
      "text": "クライアントの値を維持"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const zh = [\"智能路由\", \"虚拟模型\", \"保存更改\", \"可视化\", \"简单档\", \"普通档\", \"困难档\", \"打分模型\", \"顾问\", \"不覆盖\", \"分组顺序\", \"思考力度\", \"默认档\", \"高级设置\", \"次不满意\", \"毫秒\", \"接口\", \"默认\"].filter((s) => text.includes(s)); const en = [\"Smart routing\", \"Virtual models\", \"Add virtual model\", \"Keep client value\", \"Reasoning effort\", \"Group order\", \"Advisor\", \"Scorer model\", \"Default tier\", \"Advanced settings\", \"endpoint\", \"dissatisfied\"].filter((s) => text.includes(s)); const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (zh.length || en.length || keys) throw new Error('中文残留：' + zh.join(',') + '；英文残留：' + en.join(',') + '；占位：' + (keys || []).join(',')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-ja-dialog"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"キャンセル\"s]"
    }
  ],
  "teardown": [
    {
      "action": "click",
      "selector": "role=button[name=/更改语言|Change language|言語を変更/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"简体中文\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const res = await (await fetch('/api/option/request_policy', {method: 'PATCH', headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}, body: JSON.stringify({options: {'smart_routing_setting.routes': \"[]\"}})})).json(); if (!res.success) throw new Error('写入智能路由配置失败：' + res.message); return 'ok'; }",
      "expect": "ok"
    }
  ]
}
```
## 如何验收
- [ ] English 下智能路由页和添加弹窗（含高级设置）里没有中文、日文和 {{…}} 占位
- [ ] 日本語下没有中文残留（如「虚拟模型」「不覆盖」「分组顺序」）、英文残留和占位
- [ ] 结束后语言切回简体中文
