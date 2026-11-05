---
name: detail-reasons
description: 降档被拦、交给顾问、打分超时、工具结果续跑四条主请求详情里的选档原因；续跑没有打分标签和打分记录
category: usage-logs-routing
---
# 四种选档原因
## 简介
管理员依次打开降档被拦（req_90a1d3）、交给顾问（req_a7310e）、打分超时（req_b2c9f0）、工具结果续跑（req_8f2c77）四条记录的详情，核对「智能路由」块。
## 测试目的
PRD 06-usage-log-routing 验收 C4：四种请求的选档原因文字与约束一致，打分失败原因为红字；续跑请求没有打分标签和打分记录两行。另覆盖 C3 的档位写法（打分结果、默认档）和打分超时时的「没有拿到用量，没有扣费」。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "detail-reasons",
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
      "selector": "tr:has-text(\"86,420 / 420\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"困难档（打分结果：简单档）\", \"选档原因\", \"对话历史 86,420 token，超过降档上限 20,000，保留上一轮的困难档\", \"打分标签\", \"任务类型：闲聊或简单问答 · 要想多深：低 · 工作量：一步 · 出错代价：低 · 工作类型：代码 · 把握：0.91 · 用户反馈：满意\", \"打分记录\", \"查看这次打分\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "reason-downgrade-blocked"
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
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"顾问\", \"选档原因\", \"用户连续 2 条消息不满意\", \"打分标签\", \"任务类型：生成或修改内容 · 要想多深：中 · 工作量：几步 · 出错代价：低 · 工作类型：运营 · 把握：0.78 · 用户反馈：不满意\", \"打分记录\", \"查看这次打分\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "reason-advisor"
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
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"普通档（默认档）\", \"选档原因\", \"打分超时（2,000 毫秒），使用默认档\", \"打分记录\", \"没有拿到用量，没有扣费\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "evaluate",
      "script": "() => { const red = [...document.querySelector('[role=dialog]').querySelectorAll('.text-destructive')].some((s) => s.textContent.trim() === '打分超时（2,000 毫秒），使用默认档'); if (!red) throw new Error('打分超时原因不是红字'); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "reason-scorer-timeout"
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
      "selector": "tr:has-text(\"41,377 / 612\") >> role=button[name=/^标准/]"
    },
    {
      "action": "waitFor",
      "text": "日志详情"
    },
    {
      "action": "evaluate",
      "script": "() => { const dialogs = [...document.querySelectorAll('[role=dialog]')]; const dlg = dialogs[dialogs.length - 1]; if (!dlg) throw new Error('没有打开详情弹窗'); const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const start = lines.indexOf('智能路由'); const end = lines.indexOf('Token 明细'); if (start < 0) throw new Error('详情里没有「智能路由」块'); const got = lines.slice(start + 1, end < 0 ? undefined : end); const want = [\"虚拟模型\", \"auto-gpt\", \"档位\", \"困难档\", \"选档原因\", \"工具结果续跑，沿用上一轮的档位\"]; if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error('智能路由块不一致。实际：' + got.join(' | ') + '；期望：' + want.join(' | ')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "reason-tool-continuation"
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
- [ ] 降档被拦：档位「困难档（打分结果：简单档）」，原因「对话历史 86,420 token，超过降档上限 20,000，保留上一轮的困难档」
- [ ] 交给顾问：档位「顾问」，原因「用户连续 2 条消息不满意」
- [ ] 打分超时：档位「普通档（默认档）」，原因「打分超时（2,000 毫秒），使用默认档」红字，打分记录「没有拿到用量，没有扣费」
- [ ] 工具结果续跑：原因「工具结果续跑，沿用上一轮的档位」，没有打分标签和打分记录两行
