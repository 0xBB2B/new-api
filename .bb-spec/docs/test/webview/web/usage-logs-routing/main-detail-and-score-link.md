---
name: main-detail-and-score-link
description: 主请求详情里「智能路由」块在「请求转换」下面，各行顺序和内容正确；点「查看这次打分」打开打分记录详情
category: usage-logs-routing
---
# 主请求详情与查看这次打分
## 简介
管理员打开困难档主请求（req_8f2c41）的详情，核对「智能路由」块，再点「查看这次打分」。
## 测试目的
PRD 06-usage-log-routing 验收 C3：「智能路由」块在「请求转换」下面，依次是虚拟模型、档位、选档原因、打分标签、思考内容、打分记录；点「查看这次打分」打开打分记录的详情。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "main-detail-and-score-link",
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
      "script": "() => { const lines = document.querySelector('[role=dialog]').innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const a = lines.indexOf('请求转换'), b = lines.indexOf('智能路由'), c = lines.indexOf('Token 明细'); if (!(a >= 0 && b > a && c > b)) throw new Error('块的顺序：请求转换=' + a + ' 智能路由=' + b + ' Token 明细=' + c); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"困难档\", \"选档原因\", \"出错代价=高\", \"打分标签\", \"任务类型：分析或排查 · 要想多深：中 · 工作量：几步 · 出错代价：高 · 工作类型：财务 · 把握：0.86 · 用户反馈：中性\", \"思考内容\", \"上一轮用的渠道不同，已清理 2 段\", \"打分记录\", \"查看这次打分\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "main-detail"
    },
    {
      "action": "click",
      "selector": "role=dialog >> role=button[name=\"查看这次打分\"s]"
    },
    {
      "action": "waitFor",
      "text": "req_8f2c41（困难档 · gpt-5.6-sol）"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"类型\", \"路由打分\", \"虚拟模型\", \"auto-gpt\", \"属于请求\", \"req_8f2c41（困难档 · gpt-5.6-sol）\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "assertText",
      "selector": "role=dialog",
      "text": "1,240"
    },
    {
      "action": "screenshot",
      "name": "score-detail"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 「智能路由」块在「请求转换」和「Token 明细」之间
- [ ] 块内依次是：虚拟模型 auto-gpt、档位 困难档、选档原因 出错代价=高、打分标签（七项）、思考内容「上一轮用的渠道不同，已清理 2 段」、打分记录「查看这次打分」
- [ ] 点「查看这次打分」后显示打分记录的详情（类型 路由打分、输入 Token 1,240）
