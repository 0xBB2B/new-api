---
name: effort-continuation-scorer-legacy
description: 工具结果续跑显示沿用的力度且没有打分标签和打分记录；打分记录和一期旧日志的详情没有「思考力度」一行
category: usage-logs-routing
---
# 续跑、打分记录和旧日志的思考力度
## 简介
管理员依次打开工具结果续跑（req_8f2c77）、打分记录（req_8f2c41 的 gpt-5.6-luna）、一期写入没有思考力度的旧日志（req_eff_legacy）的详情。
## 测试目的
二期 PRD 06 验收 C2（一轮 high 之后的工具结果续跑 → 详情写「high（跟随打分）」，没有打分标签和打分记录）、C3（打分记录的详情没有「思考力度」一行）、C4（升级前写入的智能路由日志没有「思考力度」一行，页面不报错）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "effort-continuation-scorer-legacy",
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
      "selector": "tr:has-text(\"41,377 / 612\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"困难档\", \"思考力度\", \"high（跟随打分）\", \"选档原因\", \"工具结果续跑，沿用上一轮的档位\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-continuation"
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
      "selector": "tr:has-text(\"1,240 / 86\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"类型\", \"路由打分\", \"虚拟模型\", \"auto-gpt\", \"属于请求\", \"req_8f2c41（困难档 · gpt-5.6-sol）\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-scorer-record"
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
      "selector": "tr:has-text(\"4,680 / 357\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('旧日志详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); if (got.includes('思考力度')) throw new Error('旧日志出现了「思考力度」一行'); const head = ['虚拟模型', 'auto-gpt', '档位', '普通档', '选档原因', '未命中特殊规则', '打分标签']; if (JSON.stringify(got.slice(0, head.length)) !== JSON.stringify(head)) throw new Error('旧日志的智能路由块：' + got.join(' | ')); if (JSON.stringify(got.slice(-2)) !== JSON.stringify(['打分记录', '查看这次打分'])) throw new Error('旧日志缺少打分记录一行：' + got.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "assertConsoleNoError"
    },
    {
      "action": "screenshot",
      "name": "effort-legacy-log"
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
- [ ] 工具结果续跑：档位「困难档」下面「思考力度 high（跟随打分）」，选档原因「工具结果续跑，沿用上一轮的档位」，没有打分标签和打分记录
- [ ] 打分记录详情只有 类型 / 虚拟模型 / 属于请求，没有「思考力度」
- [ ] 旧日志详情有虚拟模型、档位、选档原因、打分标签、打分记录，没有「思考力度」，控制台没有报错
