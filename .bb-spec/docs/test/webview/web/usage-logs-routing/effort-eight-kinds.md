---
name: effort-eight-kinds
description: 八种思考力度来源在日志详情「智能路由」块的「档位」下面各写成约定的样子
category: usage-logs-routing
---
# 日志详情里的八种思考力度
## 简介
管理员打开使用日志，依次打开 seed 造的八条主请求的详情，核对「智能路由」块里紧跟在「档位」下面的「思考力度」一行。
## 测试目的
二期 PRD 06 验收 C1：跟随打分、固定、不覆盖带值、顾问带值、降档、升档、不支持思考、不覆盖不带值八种请求，详情里「思考力度」一行依次是约束里的八种写法。日志由 seed 直接写入，字段格式按 service/smartroute/log.go；真实请求产生这些值由后端测试覆盖。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "effort-eight-kinds",
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
      "target": "/usage-logs/common"
    },
    {
      "action": "waitFor",
      "text": "路由打分 · auto-gpt"
    },
    {
      "action": "click",
      "selector": "tr:has-text(\"38,120 / 1,904\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-gpt\" || got[i - 2] !== '档位' || got[i - 1] !== \"困难档\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"medium（跟随打分）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"medium（跟随打分）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-38120-1904"
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
      "selector": "tr:has-text(\"86,420 / 420\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-gpt\" || got[i - 2] !== '档位' || got[i - 1] !== \"困难档（打分结果：简单档）\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"high（固定）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"high（固定）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-86420-420"
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
      "selector": "tr:has-text(\"2,100 / 380\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-gpt\" || got[i - 2] !== '档位' || got[i - 1] !== \"普通档\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"medium（不覆盖，客户端传入）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"medium（不覆盖，客户端传入）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-2100-380"
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
      "selector": "tr:has-text(\"12,880 / 2,210\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-gpt\" || got[i - 2] !== '档位' || got[i - 1] !== \"顾问\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"low（不覆盖，客户端传入）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"low（不覆盖，客户端传入）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-12880-2210"
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
      "selector": "tr:has-text(\"3,402 / 512\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-gpt\" || got[i - 2] !== '档位' || got[i - 1] !== \"普通档（默认档）\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"high（跟随打分，由 xhigh 降档）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"high（跟随打分，由 xhigh 降档）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-3402-512"
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
      "selector": "tr:has-text(\"1,357 / 79\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-gpt\" || got[i - 2] !== '档位' || got[i - 1] !== \"简单档\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"low（不覆盖，由 minimal 升档）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"low（不覆盖，由 minimal 升档）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-1357-79"
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
      "selector": "tr:has-text(\"2,468 / 135\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-chat\" || got[i - 2] !== '档位' || got[i - 1] !== \"简单档\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"未发送（模型不支持思考）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"未发送（模型不支持思考）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-2468-135"
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
      "selector": "tr:has-text(\"3,579 / 246\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const i = got.indexOf('思考力度'); if (i < 0) throw new Error('没有「思考力度」一行：' + got.join(' | ')); if (got[1] !== \"auto-gpt\" || got[i - 2] !== '档位' || got[i - 1] !== \"普通档\") throw new Error('「思考力度」不在档位下面，或虚拟模型/档位不对：' + got.join(' | ')); if (got[i + 1] !== \"未发送（客户端未指定）\") throw new Error('思考力度是：' + got[i + 1] + '，期望：' + \"未发送（客户端未指定）\"); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-3579-246"
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
  "teardown": []
}
```
## 如何验收
- [ ] 跟随打分（38,120 / 1,904）：「档位」困难档 下面一行「思考力度」medium（跟随打分）
- [ ] 固定（86,420 / 420）：「档位」困难档（打分结果：简单档） 下面一行「思考力度」high（固定）
- [ ] 不覆盖、客户端带值（2,100 / 380）：「档位」普通档 下面一行「思考力度」medium（不覆盖，客户端传入）
- [ ] 顾问、客户端带值（12,880 / 2,210）：「档位」顾问 下面一行「思考力度」low（不覆盖，客户端传入）
- [ ] 降档（3,402 / 512）：「档位」普通档（默认档） 下面一行「思考力度」high（跟随打分，由 xhigh 降档）
- [ ] 升档（1,357 / 79）：「档位」简单档 下面一行「思考力度」low（不覆盖，由 minimal 升档）
- [ ] 不支持思考（2,468 / 135）：「档位」简单档 下面一行「思考力度」未发送（模型不支持思考）
- [ ] 不覆盖、客户端没带值（3,579 / 246）：「档位」普通档 下面一行「思考力度」未发送（客户端未指定）
