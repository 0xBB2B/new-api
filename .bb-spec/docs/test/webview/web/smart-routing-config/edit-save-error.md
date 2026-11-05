---
name: edit-save-error
description: 困难档换成在 api 分组没有渠道的模型，保存时字段下出现红字、弹窗不关；改回原模型后保存成功
category: smart-routing-config
---
# 编辑时保存出错
## 简介
编辑 auto-gpt，把困难档模型换成只在 subscription 分组有渠道的 gpt-5.6-terra（分组顺序仍是 subscription → api），点弹窗里的「保存」；看到报错后改回 gpt-5.6-sol 再保存。
## 测试目的
PRD 05-config-page 验收 C7 和边界用例：模型在所配分组里没有可用渠道时，弹窗不关，困难档模型下面红字写「gpt-5.6-terra 在分组 api 中没有可用渠道」；改正后保存，弹窗关闭、表格更新。同时检查出错的模型框仍显示模型名、下拉没有自动展开。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "edit-save-error",
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
      "selector": "role=group[name=\"auto-gpt 的操作\"s] >> role=button[name=\"编辑\"s]"
    },
    {
      "action": "waitFor",
      "text": "编辑虚拟模型"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"困难档模型\"s]"
    },
    {
      "action": "fill",
      "selector": "role=dialog >> role=combobox[name=\"困难档模型\"s]",
      "value": "gpt-5.6-terra"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"gpt-5.6-terra\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "gpt-5.6-terra 在分组 api 中没有可用渠道"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); if (!dlg) throw new Error('弹窗关闭了'); const box = dlg.querySelector('[role=combobox][aria-label=\"困难档模型\"]'); const msg = [...dlg.querySelectorAll('p')].find((p) => p.textContent.trim() === 'gpt-5.6-terra 在分组 api 中没有可用渠道'); if (!msg) throw new Error('没有红字提示'); if (!msg.classList.contains('text-destructive')) throw new Error('提示不是红字：' + msg.className); const block = msg.closest('[data-slot=form-item]') || msg.parentElement; if (!block.contains(box)) throw new Error('红字不在困难档模型字段下面'); if (box.value !== 'gpt-5.6-terra') throw new Error('困难档模型框显示：' + box.value); if (box.getAttribute('aria-expanded') === 'true') throw new Error('出错后模型下拉自动展开了'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "edit-save-error"
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
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { for (let i = 0; i < 50; i++) { if (!document.querySelector('[role=dialog]')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('弹窗没有关闭'); }",
      "expect": "ok"
    },
    {
      "action": "assertText",
      "selector": "table",
      "text": "gpt-5.6-sol"
    },
    {
      "action": "screenshot",
      "name": "edit-save-fixed"
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
- [ ] 保存后弹窗不关，困难档模型下面红字「gpt-5.6-terra 在分组 api 中没有可用渠道」
- [ ] 出错的模型框仍显示 gpt-5.6-terra，下拉没有自动展开
- [ ] 改回 gpt-5.6-sol 再保存，弹窗关闭
