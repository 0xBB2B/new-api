---
name: editor-no-scroll-1100
description: 在 1100px 高的窗口里打开 9 种状态（新建、编辑各档、顾问、各种出错），弹窗内容区都不出现滚动条
category: smart-routing-editor
---
# 1100px 高时弹窗不出滚动条
## 简介
在页面上放一个 1440×1100 的同源 iframe 打开智能路由页，在里面依次打开：新建弹窗；编辑 auto-gpt 的打分、档位简单、普通、困难（跟随打分两个下拉）、顾问；再删掉打分和普通档的分组、取消接口后保存，看出错后的打分、档位普通、顾问三种样子。每种状态都检查弹窗里有没有可滚动的区域。
## 测试目的
二期 PRD 04 验收 C12：在 1100px 高的窗口打开 9 种状态，弹窗内容区都不出现滚动条。动作词表里没有改窗口大小，用 iframe 模拟；iframe 里的点击用指针和鼠标事件模拟。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-no-scroll-1100",
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
      "action": "evaluate",
      "script": "async () => { document.getElementById('sr-probe')?.remove(); const f = document.createElement('iframe'); f.id = 'sr-probe'; f.src = '/system-settings/request-policies/smart-routing'; f.style.cssText = 'position:fixed;left:0;top:0;width:1440px;height:1100px;border:1px solid #888;background:#fff;z-index:2147483647'; document.body.appendChild(f); for (let i = 0; i < 300; i++) { const d = f.contentDocument; if (d && d.querySelector('[role=group][aria-label=\"auto-gpt 的操作\"] button')) return 'ok'; await new Promise((r) => setTimeout(r, 100)); } throw new Error('1440x1100 的窗口里，智能路由页 30 秒内没有加载出 auto-gpt'); }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "async () => { const f = document.getElementById('sr-probe'); const d = f.contentDocument; const w = f.contentWindow; const press = (el) => { const o = {bubbles: true, cancelable: true, view: w, button: 0}; el.dispatchEvent(new w.PointerEvent('pointerdown', {...o, pointerType: 'mouse'})); el.dispatchEvent(new w.MouseEvent('mousedown', o)); el.dispatchEvent(new w.PointerEvent('pointerup', {...o, pointerType: 'mouse'})); el.dispatchEvent(new w.MouseEvent('mouseup', o)); el.dispatchEvent(new w.MouseEvent('click', o)); }; const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const waitFor = async (fn, what) => { for (let i = 0; i < 100; i++) { const v = fn(); if (v) return v; await sleep(100); } throw new Error('等不到：' + what); }; const dlgEl = () => d.querySelector('[role=dialog]'); const tabOf = (prefix) => [...dlgEl().querySelectorAll('[role=tab]')].find((t) => t.textContent.trim().startsWith(prefix)); const btn = (label) => dlgEl().querySelector('button[aria-label=\"' + label + '\"]'); const textBtn = (root, text) => [...root.querySelectorAll('button')].find((b) => b.textContent.trim() === text); const problems = []; const checked = []; const check = (state) => { const dlg = d.querySelector('[role=dialog]'); if (!dlg) { problems.push(state + '：弹窗不见了'); return; } const over = [dlg, ...dlg.querySelectorAll('*')].filter((el) => { const s = w.getComputedStyle(el); return /(auto|scroll)/.test(s.overflowY) && el.scrollHeight > el.clientHeight + 1; }).map((el) => (el.getAttribute('data-slot') || el.tagName.toLowerCase()) + ' ' + el.scrollHeight + '>' + el.clientHeight); if (over.length) problems.push(state + '出现滚动条：' + over.join(', ')); if (dlg.getBoundingClientRect().bottom > 1100) problems.push(state + '弹窗超出窗口底部'); checked.push(state); }; const open = async (el, what) => { press(el); await waitFor(() => dlgEl(), what); await sleep(400); }; const closeDlg = async () => { press(textBtn(dlgEl(), '取消')); await waitFor(() => !dlgEl(), '弹窗关闭'); }; const go = async (prefix) => { press(tabOf(prefix)); await sleep(300); }; await open(textBtn(d, '添加虚拟模型'), '新建弹窗'); check('1 新建'); await closeDlg(); await open(d.querySelector('[role=group][aria-label=\"auto-gpt 的操作\"] button[aria-label=\"编辑\"]'), '编辑弹窗'); check('2 编辑-打分'); await go('档位'); check('3 编辑-简单'); await go('普通'); check('4 编辑-普通'); await go('困难'); check('5 编辑-困难（跟随打分）'); await go('顾问'); check('6 编辑-顾问'); await go('打分'); for (const g of ['subscription', 'api']) { press(btn('移除 ' + g)); await sleep(150); } await go('档位'); await go('普通'); for (const g of ['subscription', 'api']) { press(btn('移除 ' + g)); await sleep(150); } const resp = [...dlgEl().querySelectorAll('[role=group][aria-label=\"接口\"] button')].find((b) => b.textContent.trim() === 'Responses'); press(resp); await sleep(150); press(textBtn(dlgEl(), '保存')); await waitFor(() => dlgEl().textContent.includes('至少选择一种接口'), '出错提示'); await sleep(300); if (tabOf('打分').getAttribute('aria-selected') !== 'true') problems.push('出错后没有切到「打分」'); check('7 出错-打分'); await go('档位'); check('8 出错-档位普通'); await go('顾问'); check('9 出错-顾问'); if (problems.length) throw new Error(problems.join('；') + '（已检查：' + checked.join('、') + '）'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "no-scroll-1100"
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
- [ ] 1440×1100 的窗口里，新建、编辑-打分、编辑-简单、编辑-普通、编辑-困难（跟随打分两个下拉）、编辑-顾问六种状态，弹窗里都没有出现滚动条
- [ ] 删掉打分和普通档分组、取消接口后保存，自动切到「打分」；出错后的打分、档位普通、顾问三种状态也没有滚动条
