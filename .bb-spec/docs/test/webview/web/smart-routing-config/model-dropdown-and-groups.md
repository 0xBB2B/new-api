---
name: model-dropdown-and-groups
description: 模型下拉里没有所有分组都没渠道的模型；分组顺序能添加、用 Alt+←/→ 调整顺序、删除
category: smart-routing-config
---
# 模型下拉与分组顺序
## 简介
打开添加弹窗，切到「档位」，展开简单档的模型下拉看选项；再在简单档的分组顺序里添加两个分组，聚焦分组标签后用 Alt+→、Alt+← 调整顺序，最后点标签上的 × 删除一个。
## 测试目的
一期 PRD 05-config-page 验收 C6：所有分组都没有可用渠道的 gpt-5.6-ghost 不在模型下拉里；分组顺序可以添加、调整顺序、删除。二期 PRD 04 C9 把分组改成一行小标签、去掉上移/下移按钮，所以调整顺序改用键盘 Alt+←/→；拖动、5 个上限等由 smart-routing-editor/editor-group-tags 覆盖。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "model-dropdown-and-groups",
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
      "selector": "role=dialog >> role=tab[name=/^档位/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"gpt-5.5\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].filter((o) => (o.checkVisibility ? o.checkVisibility() : o.offsetParent !== null)).map((o) => o.textContent.trim()); const problems = []; for (const m of [\"gpt-5.4-mini\", \"gpt-5.5\", \"gpt-5.6-sol\", \"gpt-5.6-luna\", \"gpt-5.6-terra\", \"scorer-mini\"]) if (!got.includes(m)) problems.push('缺少选项 ' + m); for (const m of [\"gpt-5.6-ghost\"]) if (got.includes(m)) problems.push('不应出现选项 ' + m); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "model-dropdown"
    },
    {
      "action": "press",
      "key": "Escape"
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
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s]"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. subscription\", \"2. api\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const li = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].find((x) => x.getAttribute('aria-label') === \"1. subscription\"); if (!li) throw new Error('没有分组标签 ' + \"1. subscription\"); li.focus(); if (document.activeElement !== li) throw new Error('分组标签没有拿到焦点'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Alt+ArrowRight"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. api\", \"2. subscription\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Alt+ArrowLeft"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. subscription\", \"2. api\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 subscription\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] li[aria-keyshortcuts]')].map((li) => li.getAttribute('aria-label')); const want = [\"1. api\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组标签是 ' + got.join(' / ') + '，期望 ' + want.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "group-order"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 模型下拉里有 gpt-5.4-mini、gpt-5.5、gpt-5.6-sol、gpt-5.6-luna、gpt-5.6-terra、scorer-mini，没有 gpt-5.6-ghost
- [ ] 添加 subscription、api 后标签依次是「1. subscription」「2. api」
- [ ] 聚焦 subscription 按 Alt+→ 后是 api → subscription，再按 Alt+← 恢复
- [ ] 点 subscription 的 × 后只剩「1. api」
