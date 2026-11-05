---
name: user-view-and-plain-log
description: alice 看自己日志里的路由信息与管理员一致；看不到智能路由设置；没有经过智能路由的日志没有「智能路由」块
category: usage-logs-routing
---
# 普通用户的日志与普通日志
## 简介
普通用户 alice 登录，打开使用日志核对自己那条 auto-gpt 请求的小字和详情，从详情点进打分记录；再打开一条直接请求真实模型的日志；最后确认侧边栏没有系统设置入口、直接访问智能路由页被拒绝。
## 测试目的
PRD 05-config-page 验收 C1（没有请求策略权限的账号看不到「智能路由」）；PRD 06-usage-log-routing 验收 C6（普通用户看到的路由信息与管理员相同、不含对话文字）、C7（没有经过智能路由的日志没有「智能路由」块）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "user-view-and-plain-log",
  "category": "usage-logs-routing",
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
      "value": "alice"
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
      "target": "/usage-logs/common"
    },
    {
      "action": "waitFor",
      "text": "auto-gpt · 普通档"
    },
    {
      "action": "evaluate",
      "script": "() => { const text = document.querySelector('tbody').innerText; for (const s of ['req_8f2c41', 'auto-gpt · 困难档', 'auto-gpt · 顾问', '打分失败']) if (text.includes(s)) throw new Error('alice 看到了别人的记录：' + s); const span = [...document.querySelectorAll('tbody span')].find((s) => s.textContent.trim() === 'auto-gpt · 普通档'); if (!span || !span.classList.contains('text-muted-foreground')) throw new Error('「auto-gpt · 普通档」不是灰字'); if (!text.includes('路由打分 · auto-gpt')) throw new Error('没有自己的打分记录'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"2,100 / 380\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"普通档\", \"选档原因\", \"未命中特殊规则\", \"打分标签\", \"任务类型：改写或整理 · 要想多深：低 · 工作量：一步 · 出错代价：低 · 工作类型：其他 · 把握：0.82 · 用户反馈：满意\", \"打分记录\", \"查看这次打分\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "alice-main-detail"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"查看这次打分\"s]"
    },
    {
      "action": "waitFor",
      "text": "req_alice01（普通档 · gpt-5.5）"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"类型\", \"路由打分\", \"虚拟模型\", \"auto-gpt\", \"属于请求\", \"req_alice01（普通档 · gpt-5.5）\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "press",
      "key": "Escape"
    },
    {
      "action": "waitFor",
      "text": "未命中特殊规则"
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
      "selector": "tr:has-text(\"5,200 / 300\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const lines = document.querySelector('[role=dialog]').innerText.split('\\n').map((s) => s.trim()); if (!lines.includes('req_plain01')) throw new Error('打开的不是 req_plain01'); if (!lines.includes('Token 明细') || !lines.includes('计费详情')) throw new Error('普通日志缺少原有的块'); if (lines.includes('智能路由')) throw new Error('普通日志里出现了「智能路由」块'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "plain-log-detail"
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
      "script": "() => { const links = [...document.querySelectorAll('a')]; if (links.some((a) => a.textContent.trim() === '智能路由' || (a.getAttribute('href') || '').startsWith('/system-settings'))) throw new Error('alice 看到了系统设置或智能路由入口'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "navigate",
      "target": "/system-settings/request-policies/smart-routing"
    },
    {
      "action": "waitFor",
      "text": "禁止访问"
    },
    {
      "action": "assertUrl",
      "contains": "/403"
    },
    {
      "action": "screenshot",
      "name": "alice-settings-forbidden"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] alice 只看到自己的记录：「auto-gpt · 普通档」灰字和「路由打分 · auto-gpt」
- [ ] 详情「智能路由」块各行与管理员看到的写法一致，只有标签和原因，没有对话文字；打分记录的属于请求是「req_alice01（普通档 · gpt-5.5）」
- [ ] req_plain01 的详情有「Token 明细」「计费详情」，没有「智能路由」块
- [ ] 侧边栏没有系统设置和「智能路由」入口，直接访问智能路由页跳到 403「禁止访问」
