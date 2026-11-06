---
name: key-drawer-group-order
description: 密钥抽屉里的自动分组顺序仍是原来的大列表样式，有上移、下移、拖动把手，没有紧凑小标签
category: smart-routing-editor
---
# 密钥抽屉的分组顺序不受影响
## 简介
在密钥页点「创建 API 密钥」，分组选 auto，在「自动分组顺序」里添加 api、subscription，核对每项有上移、下移和拖动把手，再把 subscription 上移。
## 测试目的
二期 PRD 04 验收 C9 最后一项：紧凑样式只用在智能路由弹窗，密钥抽屉里的分组顺序样子和行为不变。需要 seed 把 auto 放进用户可选分组。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "key-drawer-group-order",
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
    }
  ],
  "steps": [
    {
      "action": "navigate",
      "target": "/keys"
    },
    {
      "action": "click",
      "selector": "role=button[name=\"创建 API 密钥\"s]"
    },
    {
      "action": "waitFor",
      "text": "基本信息"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox >> nth=0"
    },
    {
      "action": "click",
      "selector": "role=option[name=/^auto/]"
    },
    {
      "action": "waitFor",
      "text": "自动分组顺序"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); if (dlg.querySelector('li[aria-keyshortcuts]')) throw new Error('密钥抽屉里出现了紧凑小标签'); if (![...dlg.querySelectorAll('button')].some((b) => b.textContent.trim() === '恢复全局 Auto')) throw new Error('没有「恢复全局 Auto」按钮'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox >> text=添加 Auto 分组"
    },
    {
      "action": "click",
      "selector": "role=option[name=/^api/]"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=combobox >> text=添加 Auto 分组"
    },
    {
      "action": "click",
      "selector": "role=option[name=/^subscription/]"
    },
    {
      "action": "evaluate",
      "script": "() => { const dlg = document.querySelector('[role=dialog]'); const labels = [...dlg.querySelectorAll('button[aria-label]')].map((b) => b.getAttribute('aria-label')); const must = ['将 api 上移', '将 api 下移', '拖动 api 以重新排序', '移除 api', '将 subscription 上移', '移除 subscription']; const missing = must.filter((m) => !labels.includes(m)); if (missing.length) throw new Error('缺少按钮：' + missing.join(',')); const order = labels.filter((l) => l.startsWith('移除 ')).map((l) => l.replace('移除 ', '')); if (JSON.stringify(order) !== JSON.stringify(['api', 'subscription'])) throw new Error('顺序是 ' + order.join(' → ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"将 subscription 上移\"s]"
    },
    {
      "action": "evaluate",
      "script": "() => { const order = [...document.querySelectorAll('[role=dialog] button[aria-label^=\"移除 \"]')].map((b) => b.getAttribute('aria-label').replace('移除 ', '')); if (JSON.stringify(order) !== JSON.stringify(['subscription', 'api'])) throw new Error('上移后顺序是 ' + order.join(' → ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "key-drawer-group-order"
    },
    {
      "action": "press",
      "key": "Escape"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 分组选 auto 后显示「自动分组顺序」和「恢复全局 Auto」，没有紧凑小标签
- [ ] 添加的每个分组都有「将 X 上移」「将 X 下移」「拖动 X 以重新排序」「移除 X」按钮
- [ ] 点「将 subscription 上移」后顺序变成 subscription → api
