---
name: scorer-detail-back-to-main
description: 打分记录详情显示类型、虚拟模型、属于请求；点「属于请求」打开主请求详情
category: usage-logs-routing
---
# 打分记录详情与属于请求
## 简介
管理员直接打开打分记录（Token 1,240 / 86）的详情，核对「智能路由」块，再点「属于请求」。
## 测试目的
PRD 06-usage-log-routing 验收 C5：打分记录详情显示类型「路由打分」、虚拟模型、属于请求（请求 ID、档位、模型）；点击后打开主请求详情。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "scorer-detail-back-to-main",
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
      "name": "scorer-detail"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=/req_8f2c41/]"
    },
    {
      "action": "waitFor",
      "text": "查看这次打分"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"困难档\", \"思考力度\", \"medium（跟随打分）\", \"选档原因\", \"出错代价=高\", \"打分标签\", \"任务类型：分析或排查 · 要想多深：中 · 工作量：几步 · 出错代价：高 · 工作类型：财务 · 把握：0.86 · 用户反馈：中性\", \"思考内容\", \"上一轮用的渠道不同，已清理 2 段\", \"打分记录\", \"查看这次打分\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "assertText",
      "selector": "role=dialog",
      "text": "38,120"
    },
    {
      "action": "screenshot",
      "name": "back-to-main"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 打分记录详情的「智能路由」块：类型 路由打分、虚拟模型 auto-gpt、属于请求「req_8f2c41（困难档 · gpt-5.6-sol）」
- [ ] 点「属于请求」后显示主请求 req_8f2c41 的详情（输入 Token 38,120、思考力度 medium（跟随打分）、选档原因 出错代价=高）
- [ ] 打分记录详情里没有「思考力度」一行
