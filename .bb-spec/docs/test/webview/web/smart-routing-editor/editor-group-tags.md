---
name: editor-group-tags
description: 分组顺序是一行带序号的小标签：只有一个时拖不动，拖动和 Alt+←/→ 能调整顺序，满 5 个后「＋」变灰，出错时描红
category: smart-routing-editor
---
# 分组顺序的紧凑小标签
## 简介
新建虚拟模型，在「打分」Tab 的分组顺序里逐个添加分组，依次核对：一个分组时不能拖、光标不是抓手；添加到 5 个后「＋」变灰；把 api 拖到 subscription 前面；聚焦 api 按 Alt+→；全部删掉后保存，看这一行描红和红字。
## 测试目的
二期 PRD 04 验收 C9 的前四项：三个 Tab 的分组都是一行小标签（本用例用打分 Tab，档位和顾问 Tab 的分组由 add-and-save、model-dropdown-and-groups 覆盖）；拖动能调整顺序；Alt + ← / → 能移动；只有一个分组时拖不动；第 5 个之后「＋」变灰；没选分组时只显示「＋ 添加分组」，出错时描红并写红字。拖动用 HTML5 拖放事件模拟。密钥抽屉的分组顺序由 key-drawer-group-order 覆盖。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "editor-group-tags",
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const headers = {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}; const call = async (method, path, body) => { const res = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)}); const data = await res.json(); if (!data.success) throw new Error(method + ' ' + path + ' 失败：' + JSON.stringify(data.message)); return data; }; await call('PATCH', '/api/option/request_policy', {options: {'smart_routing_setting.routes': JSON.stringify([]), 'smart_routing_setting.thinking_capabilities': JSON.stringify({})}}); return 'ok'; }",
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
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"subscription\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = document.querySelector('[role=dialog] [role=tabpanel]'); const li = panel.querySelector('li[aria-keyshortcuts]'); if (!li || li.getAttribute('aria-label') !== '1. subscription') throw new Error('第一个标签不是「1. subscription」'); if (li.draggable) throw new Error('只有一个分组时标签还能拖动'); if (getComputedStyle(li).cursor === 'grab') throw new Error('只有一个分组时光标是抓手'); if (!li.querySelector('button[aria-label=\"移除 subscription\"]')) throw new Error('标签右边没有 ×'); const add = panel.querySelector('[role=combobox][aria-label=\"添加分组\"]'); if (add.innerText.trim() !== '') throw new Error('有分组后「＋」后面不应再有文字：' + add.innerText); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"team-a\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"team-b\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"default\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. subscription\", \"2. api\", \"3. team-a\", \"4. team-b\", \"5. default\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = document.querySelector('[role=dialog] [role=tabpanel]'); const add = panel.querySelector('[role=combobox][aria-label=\"添加分组\"]'); if (!add.disabled && add.getAttribute('aria-disabled') !== 'true') throw new Error('满 5 个后「＋」没有变灰'); const li = panel.querySelector('li[aria-keyshortcuts]'); if (!li.draggable) throw new Error('多个分组时标签不能拖动'); if (getComputedStyle(li).cursor !== 'grab') throw new Error('鼠标停在标签上光标不是抓手：' + getComputedStyle(li).cursor); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "group-tags-full"
    },
    {
      "action": "evaluate",
      "script": "async () => { const tick = () => new Promise((r) => setTimeout(r, 80)); const tags = () => [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')]; const find = (g) => tags().find((li) => li.getAttribute('aria-label').endsWith('. ' + g)); const src = find(\"api\"); const dst = find(\"subscription\"); if (!src || !dst) throw new Error('找不到要拖动的分组标签'); if (!src.draggable) throw new Error('分组标签不能拖动'); const dt = new DataTransfer(); src.dispatchEvent(new DragEvent('dragstart', {bubbles: true, cancelable: true, dataTransfer: dt})); await tick(); const rect = dst.getBoundingClientRect(); const x = rect.left + 2; const opts = {bubbles: true, cancelable: true, dataTransfer: dt, clientX: x, clientY: rect.top + rect.height / 2}; dst.dispatchEvent(new DragEvent('dragenter', opts)); dst.dispatchEvent(new DragEvent('dragover', opts)); await tick(); dst.dispatchEvent(new DragEvent('drop', opts)); src.dispatchEvent(new DragEvent('dragend', {bubbles: true, dataTransfer: dt})); await tick(); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. api\", \"2. subscription\", \"3. team-a\", \"4. team-b\", \"5. default\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const li = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].find((x) => x.getAttribute('aria-label') === \"1. api\"); if (!li) throw new Error('没有分组标签 ' + \"1. api\"); li.focus(); if (document.activeElement !== li) throw new Error('分组标签没有拿到焦点'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Alt+ArrowRight"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. subscription\", \"2. api\", \"3. team-a\", \"4. team-b\", \"5. default\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 subscription\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 api\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 team-a\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 team-b\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 default\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = []; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"保存\"s]"
    },
    {
      "action": "waitFor",
      "text": "至少选择一个分组"
    },
    {
      "action": "evaluate",
      "script": "() => { const panel = document.querySelector('[role=dialog] [role=tabpanel]'); const row = panel.querySelector('[role=group][aria-label=\"分组顺序\"]'); if (!row || row.getAttribute('aria-invalid') !== 'true') throw new Error('分组这一行没有描红'); const item = row.closest('[data-slot=form-item]'); const msg = item && [...item.querySelectorAll('[data-slot=form-message]')].find((p) => p.textContent.trim() === '至少选择一个分组'); if (!msg) throw new Error('红字不在分组这一行下面'); const add = row.querySelector('[role=combobox][aria-label=\"添加分组\"]'); if (add.innerText.trim() !== '添加分组') throw new Error('清空后没有显示「＋ 添加分组」'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "group-tags-error"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"取消\"s]"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 只有一个分组时标签是「1. subscription」，右边有 ×，不能拖动、光标不是抓手，「＋」后面没有文字
- [ ] 加满 5 个后「＋」变灰，标签可拖动、光标是抓手
- [ ] 把 api 拖到 subscription 左边后顺序是 api、subscription、team-a、team-b、default；聚焦 api 按 Alt+→ 后恢复 subscription、api
- [ ] 全部删掉后保存，分组这一行描红（aria-invalid），下面红字「至少选择一个分组」，并显示「＋ 添加分组」
