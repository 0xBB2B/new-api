---
name: model-dropdown-and-groups
description: 模型下拉里没有所有分组都没渠道的模型；分组顺序能添加、上移、下移、删除
category: smart-routing-config
---
# 模型下拉与分组顺序
## 简介
打开添加弹窗，展开简单档的模型下拉看选项；再在简单档的分组顺序里添加两个分组，依次上移、下移、删除。
## 测试目的
PRD 05-config-page 验收 C6：所有分组都没有可用渠道的 gpt-5.6-ghost 不在模型下拉里；分组顺序可以添加、上移、下移、删除（拖动不测）。
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
      "script": "async () => { const loginRes = await fetch('/api/user/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({username: 'root', password: 'Test123456!'})}); if (!loginRes.ok) throw new Error('管理员登录失败 HTTP ' + loginRes.status); const login = await loginRes.json(); if (!login.success) throw new Error('管理员登录失败：' + login.message); const res = await (await fetch('/api/option/request_policy', {method: 'PATCH', headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + login.data.access_token, 'New-Api-User': String(login.data.user.id)}, body: JSON.stringify({options: {'smart_routing_setting.routes': \"[]\"}})})).json(); if (!res.success) throw new Error('写入智能路由配置失败：' + res.message); return 'ok'; }",
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
      "selector": "role=dialog >> role=combobox[name=\"简单档模型\"s]"
    },
    {
      "action": "waitFor",
      "selector": "role=option[name=\"gpt-5.5\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=option]')].map((o) => o.textContent.trim()); const must = ['gpt-5.4-mini', 'gpt-5.5', 'gpt-5.6-sol', 'gpt-5.6-luna', 'gpt-5.6-terra', 'scorer-mini']; const missing = must.filter((m) => !got.includes(m)); if (missing.length) throw new Error('下拉里缺少：' + missing.join(',')); if (got.includes('gpt-5.6-ghost')) throw new Error('下拉里出现了没有渠道的 gpt-5.6-ghost'); return 'ok'; }",
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
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s] >> nth=0"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"subscription\"s]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox[name=\"添加分组\"s] >> nth=0"
    },
    {
      "action": "click",
      "selector": "role=option[name=\"api\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] button[aria-label^=\"移除 \"]')].map((b) => b.getAttribute('aria-label').replace('移除 ', '')); const want = [\"subscription\", \"api\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组顺序是 ' + got.join(' → ') + '，期望 ' + want.join(' → ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"将 api 上移\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] button[aria-label^=\"移除 \"]')].map((b) => b.getAttribute('aria-label').replace('移除 ', '')); const want = [\"api\", \"subscription\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组顺序是 ' + got.join(' → ') + '，期望 ' + want.join(' → ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"将 api 下移\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] button[aria-label^=\"移除 \"]')].map((b) => b.getAttribute('aria-label').replace('移除 ', '')); const want = [\"subscription\", \"api\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组顺序是 ' + got.join(' → ') + '，期望 ' + want.join(' → ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"移除 subscription\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const got = [...document.querySelectorAll('[role=dialog] button[aria-label^=\"移除 \"]')].map((b) => b.getAttribute('aria-label').replace('移除 ', '')); const want = [\"api\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('分组顺序是 ' + got.join(' → ') + '，期望 ' + want.join(' → ')); return 'ok'; }",
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
- [ ] 添加 subscription、api 后顺序是 subscription → api
- [ ] api 上移后是 api → subscription，下移后恢复，删除 subscription 后只剩 api
