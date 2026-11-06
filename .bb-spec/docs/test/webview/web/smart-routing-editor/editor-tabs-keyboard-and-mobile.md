---
name: editor-tabs-keyboard-and-mobile
description: 左栏读作标签页、方向键直接切换 Tab；400px 宽时左栏变成顶部横排
category: smart-routing-editor
---
# 左栏 Tab 的键盘操作和手机宽度
## 简介
编辑 auto-gpt，核对左栏的标签页语义；点一下「打分」后按 ↓ 直接切换到「档位」，再按 ↓ 切换到「顾问」，按 ↑ 回到「档位」，全程不按回车。再在页面上放一个 400×900 的同源 iframe 打开智能路由页，在里面打开编辑弹窗，看左栏是否变成顶部横排的三个 Tab。
## 测试目的
二期 PRD 04 验收 C2 后半：方向键能切换 Tab、读屏软件读作标签页、400px 宽时左栏变成顶部横排。说明：左栏方向键移动焦点的同时就切换 Tab，不需要回车。动作词表里没有改窗口大小，400px 宽用 iframe 模拟。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-tabs-keyboard-and-mobile",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([{\"name\": \"auto-gpt\", \"endpoints\": [\"openai-response\"], \"tiers\": {\"simple\": {\"model\": \"gpt-5.4-mini\", \"groups\": [\"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"normal\": {\"model\": \"gpt-5.5\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"\", \"effort\": \"\", \"effort_min\": \"\", \"effort_max\": \"\"}, \"hard\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"effort_mode\": \"follow\", \"effort\": \"\", \"effort_min\": \"medium\", \"effort_max\": \"xhigh\"}}, \"default_tier\": \"normal\", \"scorer\": {\"model\": \"gpt-5.6-luna\", \"groups\": [\"subscription\", \"api\"], \"timeout_ms\": 2000}, \"advisor\": {\"model\": \"gpt-5.6-sol\", \"groups\": [\"subscription\", \"api\"], \"after_dissatisfied\": 2}, \"downgrade_max_history_tokens\": 20000, \"confidence_threshold\": 0.6, \"simple_max_message_chars\": 2000}]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); if (!left) throw new Error('没有左栏 Tab'); const leftTabs = [...left.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const summary = (prefix) => tabOf(prefix).innerText.split('\\n').map((s) => s.trim()).filter(Boolean)[1] || ''; if (left.getAttribute('aria-orientation') !== 'vertical') throw new Error('左栏 aria-orientation=' + left.getAttribute('aria-orientation')); if (leftTabs.length !== 3) throw new Error('左栏有 ' + leftTabs.length + ' 个 Tab'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=tab[name=/^打分/]"
    },
    {
      "action": "press",
      "key": "ArrowDown"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const a = document.activeElement; if (!a || a.getAttribute('role') !== 'tab' || !a.textContent.trim().startsWith('档位')) throw new Error('按 ↓ 后焦点在：' + (a && a.textContent.trim())); if (a.getAttribute('aria-selected') !== 'true') throw new Error('按 ↓ 后没有切到「档位」'); if (!dlg.querySelector('[role=combobox][aria-label=\\\"默认档\\\"]')) throw new Error('没有显示档位的内容'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "ArrowDown"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const left = dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'); if (!left) throw new Error('没有左栏 Tab'); const leftTabs = [...left.querySelectorAll('[role=tab]')]; const tabOf = (prefix) => [...dlg.querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const summary = (prefix) => tabOf(prefix).innerText.split('\\n').map((s) => s.trim()).filter(Boolean)[1] || ''; if (tabOf('顾问').getAttribute('aria-selected') !== 'true') throw new Error('没有切到「顾问」'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "ArrowUp"
    },
    {
      "action": "evaluate",
      "script": "() => { const a = document.activeElement; if (!a || !a.textContent.trim().startsWith('档位')) throw new Error('按 ↑ 后焦点在：' + (a && a.textContent.trim())); if (a.getAttribute('aria-selected') !== 'true') throw new Error('按 ↑ 后没有切到「档位」'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "tabs-keyboard"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { document.getElementById('sr-probe')?.remove(); const f = document.createElement('iframe'); f.id = 'sr-probe'; f.src = '/system-settings/request-policies/smart-routing'; f.style.cssText = 'position:fixed;left:0;top:0;width:400px;height:900px;border:1px solid #888;background:#fff;z-index:2147483647'; document.body.appendChild(f); for (let i = 0; i < 300; i++) { const d = f.contentDocument; if (d && d.querySelector('[role=group][aria-label=\"auto-gpt 的操作\"] button')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('400x900 的窗口里，智能路由页 30 秒内没有加载出 auto-gpt'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const f = document.getElementById('sr-probe'); const d = f.contentDocument; const w = f.contentWindow; const press = (el) => { const o = {bubbles: true, cancelable: true, view: w, button: 0}; el.dispatchEvent(new w.PointerEvent('pointerdown', {...o, pointerType: 'mouse'})); el.dispatchEvent(new w.MouseEvent('mousedown', o)); el.dispatchEvent(new w.PointerEvent('pointerup', {...o, pointerType: 'mouse'})); el.dispatchEvent(new w.MouseEvent('mouseup', o)); el.dispatchEvent(new w.MouseEvent('click', o)); }; const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const waitFor = async (fn, what) => { for (let i = 0; i < 100; i++) { const v = fn(); if (v) return v; await sleep(100); } throw new Error('等不到：' + what); }; press(d.querySelector('[role=group][aria-label=\"auto-gpt 的操作\"] button[aria-label=\"编辑\"]')); const dlg = await waitFor(() => d.querySelector('[role=dialog]'), '编辑弹窗'); const left = await waitFor(() => dlg.querySelector('[role=tablist][aria-label=\"虚拟模型设置\"]'), '左栏'); await sleep(300); const problems = []; if (left.getAttribute('aria-orientation') !== 'horizontal') problems.push('aria-orientation=' + left.getAttribute('aria-orientation')); const tabs = [...left.querySelectorAll('[role=tab]')].map((t) => t.getBoundingClientRect()); if (tabs.length !== 3) problems.push('Tab 数量 ' + tabs.length); if (Math.max(...tabs.map((r) => r.top)) - Math.min(...tabs.map((r) => r.top)) > 2) problems.push('三个 Tab 不在同一行'); const panel = dlg.querySelector('[role=tabpanel]'); if (panel && left.getBoundingClientRect().bottom > panel.getBoundingClientRect().top + 1) problems.push('Tab 不在内容上方'); if (problems.length) throw new Error('400px 宽时：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "tabs-400px"
    },
    {
      "action": "evaluate",
      "script": "() => { document.getElementById('sr-probe')?.remove(); return 'ok'; }",
      "expect": "ok"
    }
  ],
  "teardown": [
    {
      "action": "evaluate",
      "script": "() => { document.getElementById('sr-probe')?.remove(); return 'ok'; }",
      "expect": "ok"
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
- [ ] 左栏是 tablist（aria-orientation=vertical），三个 Tab 的 role 是 tab
- [ ] 聚焦「打分」按 ↓，「档位」直接被选中并显示档位内容；再按 ↓ 直接切到「顾问」；按 ↑ 直接切回「档位」，全程不按回车
- [ ] 400px 宽时左栏 aria-orientation=horizontal，三个 Tab 在同一行、位于内容上方
