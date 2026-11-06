---
name: editor-i18n-seven-languages
description: 切换 English、Français、Русский、日本語、Tiếng Việt、繁體中文，智能路由页、编辑弹窗各 Tab 和覆盖弹窗的新增文字都有翻译
category: smart-routing-editor
---
# 二期新增文字的七种语言
## 简介
配好 auto-gpt、auto-chat，并把 gpt-5.6-sol 覆盖成只到 high（表格里出现标黄提示、能力表里有「已覆盖」「未确认」「不支持思考」）。依次切到六种非简体中文的界面语言，每种语言检查：页面（虚拟模型表格和模型思考能力卡片）、编辑弹窗的打分、档位（困难档跟随打分）、顾问 Tab，以及覆盖弹窗。简体中文由其他用例覆盖。
## 测试目的
二期 PRD 04 验收 C14、PRD 02 验收 C11：切换七种界面语言，新增文字都有翻译。界面没有翻译时会退回英文（fallbackLng=en），所以：English 查中日文字和占位；Français、Русский、Tiếng Việt 查中日文字、英文残留和占位；日本語、繁體中文查简体中文残留、英文残留和占位。ⓘ 提示只在悬停时出现，本用例不逐个悬停。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-i18n-seven-languages",
  "category": "smart-routing-editor",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}, {\"name\": \"auto-chat\", \"endpoints\": [\"openai\"], \"tiers\": {\"simple\": {\"model\": \"gpt-4.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"glm-5.1\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"fixed\", \"effort\": \"high\", \"effort_min\": \"\", \"effort_max\": \"\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({\"gpt-5.6-sol\": {\"no_thinking\": false, \"efforts\": [\"low\", \"medium\", \"high\"]}})}}); return 'ok'; }",
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
      "text": "超出模型能力，实际按 high 发送"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"English\"s]"
    },
    {
      "action": "waitFor",
      "text": "Model thinking capabilities"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = (document.querySelector('main main') || document.querySelector('main')); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); if (problems.length) throw new Error(\"en\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"auto-gpt\") >> [role=group] >> button >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); if (problems.length) throw new Error(\"en\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=1"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); if (problems.length) throw new Error(\"en\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=1 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); if (problems.length) throw new Error(\"en\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); if (problems.length) throw new Error(\"en\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-en-dialog"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim() === 'gpt-5.6-sol'; }); if (!row) throw new Error('能力表里没有 gpt-5.6-sol'); const tds = row.querySelectorAll('td'); const b = tds[tds.length - 1].querySelector('button'); if (!b) throw new Error('gpt-5.6-sol 没有操作按钮'); b.click(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "waitFor",
      "text": "Override thinking capability of gpt-5.6-sol"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); if (problems.length) throw new Error(\"en\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"Français\"s]"
    },
    {
      "action": "waitFor",
      "text": "Capacités de réflexion des modèles"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = (document.querySelector('main main') || document.querySelector('main')); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"fr\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"auto-gpt\") >> [role=group] >> button >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"fr\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=1"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"fr\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=1 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"fr\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"fr\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-fr-dialog"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim() === 'gpt-5.6-sol'; }); if (!row) throw new Error('能力表里没有 gpt-5.6-sol'); const tds = row.querySelectorAll('td'); const b = tds[tds.length - 1].querySelector('button'); if (!b) throw new Error('gpt-5.6-sol 没有操作按钮'); b.click(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "waitFor",
      "text": "Remplacer la capacité de réflexion de gpt-5.6-sol"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"fr\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"Русский\"s]"
    },
    {
      "action": "waitFor",
      "text": "Возможности рассуждения моделей"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = (document.querySelector('main main') || document.querySelector('main')); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"ru\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"auto-gpt\") >> [role=group] >> button >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"ru\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=1"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"ru\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=1 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"ru\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"ru\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-ru-dialog"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim() === 'gpt-5.6-sol'; }); if (!row) throw new Error('能力表里没有 gpt-5.6-sol'); const tds = row.querySelectorAll('td'); const b = tds[tds.length - 1].querySelector('button'); if (!b) throw new Error('gpt-5.6-sol 没有操作按钮'); b.click(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "waitFor",
      "text": "Переопределить возможности рассуждения gpt-5.6-sol"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"ru\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"日本語\"s]"
    },
    {
      "action": "waitFor",
      "text": "モデルの思考能力"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = (document.querySelector('main main') || document.querySelector('main')); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"ja\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"auto-gpt\") >> [role=group] >> button >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"ja\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=1"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"ja\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=1 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"ja\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"ja\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-ja-dialog"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim() === 'gpt-5.6-sol'; }); if (!row) throw new Error('能力表里没有 gpt-5.6-sol'); const tds = row.querySelectorAll('td'); const b = tds[tds.length - 1].querySelector('button'); if (!b) throw new Error('gpt-5.6-sol 没有操作按钮'); b.click(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.6-sol の思考能力を上書き"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"ja\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"Tiếng Việt\"s]"
    },
    {
      "action": "waitFor",
      "text": "Khả năng suy luận của mô hình"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = (document.querySelector('main main') || document.querySelector('main')); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"vi\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"auto-gpt\") >> [role=group] >> button >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"vi\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=1"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"vi\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=1 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"vi\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"vi\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-vi-dialog"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim() === 'gpt-5.6-sol'; }); if (!row) throw new Error('能力表里没有 gpt-5.6-sol'); const tds = row.querySelectorAll('td'); const b = tds[tds.length - 1].querySelector('button'); if (!b) throw new Error('gpt-5.6-sol 没有操作按钮'); b.click(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "waitFor",
      "text": "Ghi đè khả năng suy luận của gpt-5.6-sol"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const cjk = text.match(/[\\u3040-\\u30ff\\u4e00-\\u9fff]+/g); if (cjk) problems.push('中日文字：' + cjk.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); if (problems.length) throw new Error(\"vi\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"繁體中文\"s]"
    },
    {
      "action": "waitFor",
      "text": "虛擬模型"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = (document.querySelector('main main') || document.querySelector('main')); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"zhTW\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"auto-gpt\") >> [role=group] >> button >> nth=0"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"zhTW\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=1"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"zhTW\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=1 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"zhTW\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> [role=tablist] >> nth=0 >> [role=tab] >> nth=2"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"zhTW\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "i18n-zhTW-dialog"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const row = [...document.querySelectorAll('tbody tr')].find((r) => { const td = r.querySelector('td'); return td && td.innerText.trim() === 'gpt-5.6-sol'; }); if (!row) throw new Error('能力表里没有 gpt-5.6-sol'); const tds = row.querySelectorAll('td'); const b = tds[tds.length - 1].querySelector('button'); if (!b) throw new Error('gpt-5.6-sol 没有操作按钮'); b.click(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "waitFor",
      "selector": "role=dialog"
    },
    {
      "action": "waitFor",
      "text": "覆蓋 gpt-5.6-sol 的思考能力"
    },
    {
      "action": "evaluate",
      "script": "() => { const el = document.querySelector('[role=dialog]'); if (!el) throw new Error('找不到要检查的区域'); const text = el.innerText; const problems = []; const keys = text.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push('翻译占位：' + keys.join(',')); const en = [\"Virtual models\", \"Model thinking capabilities\", \"Built-in\", \"Unconfirmed\", \"Overridden\", \"Not overridden\", \"Follow scoring\", \"Not set\", \"/3 set\", \"Default tier\", \"Group order\", \"Reasoning effort\", \"Scorer model\", \"Advisor model\", \"Timeout (ms)\", \"Add group\", \"Advanced settings\", \"Supported efforts\", \"No thinking\", \"Exceeds the model capability\", \"Restore built-in\", \" endpoint\", \"Edit virtual model\", \"Override thinking capability\", \"Tick the thinking efforts\", \"Smart routing uses this\", \"dissatisfied\", \"Capability not confirmed\", \"Scoring\", \"Advisor\", \"Each virtual model has\"].filter((s) => text.includes(s)); if (en.length) problems.push('英文残留：' + en.join(',')); const zh = [\"档位\", \"虚拟模型\", \"覆盖\", \"内置\", \"未确认\", \"未设置\", \"已设置\", \"分组\", \"跟随打分\", \"默认\", \"顾问\", \"打分\", \"恢复\", \"编辑\", \"高级设置\", \"实际按\", \"发送\", \"接口\"].filter((s) => text.includes(s)); if (zh.length) problems.push('简体中文残留：' + zh.join(',')); if (problems.length) throw new Error(\"zhTW\" + ' 检查：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    }
  ],
  "teardown": [
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"简体中文\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
      "expect": "ok"
    }
  ]
}
```
## 如何验收
- [ ] 六种语言下，智能路由页（表格、标黄提示、模型思考能力卡片）没有翻译缺失的残留和 {{…}} 占位
- [ ] 六种语言下，编辑弹窗的打分、档位（困难档跟随打分）、顾问 Tab 和覆盖弹窗没有残留和占位
- [ ] 结束后语言切回简体中文
