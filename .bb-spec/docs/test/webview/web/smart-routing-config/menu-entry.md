---
name: menu-entry
description: 管理员在请求策略菜单里看到「智能路由」，位于「渠道健康」之后，点进去打开智能路由页
category: smart-routing-config
---
# 请求策略菜单里的智能路由入口
## 简介
管理员进入系统设置的请求策略分区，看左侧菜单里新增的「智能路由」，点它打开智能路由页。
## 测试目的
PRD 05-config-page 验收 C1：请求策略菜单在「渠道健康」后面有「智能路由」。没有请求策略权限的账号看不到，由 usage-logs-routing/user-view-and-plain-log 覆盖。立即保存 PRD 2026-11-09.smart-routing-instant-save 01 C1：智能路由页标题右边没有「保存更改」按钮。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "menu-entry",
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
    }
  ],
  "steps": [
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/filtering"
    },
    {
      "action": "waitFor",
      "text": "渠道健康"
    },
    {
      "action": "evaluate",
      "script": "() => { const names = [...new Set([...document.querySelectorAll('a[href^=\"/system-settings/request-policies/\"]')].map((a) => a.textContent.trim()))]; const i = names.indexOf('渠道健康'); const j = names.indexOf('智能路由'); if (i < 0 || j !== i + 1) throw new Error('请求策略菜单顺序：' + names.join(' / ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=link[name=\"智能路由\"s]"
    },
    {
      "action": "waitFor",
      "text": "每个虚拟模型配好三档模型、打分模型和顾问。用户在客户端里把模型名填成虚拟模型名就能用。"
    },
    {
      "action": "assertUrl",
      "contains": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "assertText",
      "selector": "main",
      "text": "智能路由"
    },
    {
      "action": "evaluate",
      "script": "() => { const names = [...document.querySelectorAll('button')].map((b) => b.textContent.trim()); if (names.includes('保存更改')) throw new Error('页面上还有「保存更改」按钮'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "menu-entry"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 请求策略菜单依次是：请求检查、会话与重试、渠道健康、智能路由
- [ ] 点「智能路由」后地址是 /system-settings/request-policies/smart-routing，页面有标题「智能路由」，没有「保存更改」按钮
