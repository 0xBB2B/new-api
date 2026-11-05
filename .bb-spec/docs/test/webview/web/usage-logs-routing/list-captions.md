---
name: list-captions
description: 使用日志列表里普通档、顾问、打分失败、路由打分四种小字的文字和颜色
category: usage-logs-routing
---
# 日志列表里的路由小字
## 简介
管理员打开使用日志，看经过智能路由的记录在模型列下面的小字，以及打分记录这一行。
## 测试目的
PRD 06-usage-log-routing 验收 C1（普通情况灰字、交给顾问黄字、打分失败红字）、C2（打分记录单独一行，小字「路由打分 · auto-gpt」灰字，紧挨在主请求下面）。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "list-captions",
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
      "action": "evaluate",
      "script": "() => { const rows = [...document.querySelectorAll('tbody tr')]; const find = (tokens) => { const i = rows.findIndex((r) => r.innerText.includes(tokens)); if (i < 0) throw new Error('没有 Token 为 ' + tokens + ' 的记录'); return i; }; const caption = (row, text) => [...row.querySelectorAll('span')].find((s) => s.textContent.trim() === text); const want = [['38,120 / 1,904', 'gpt-5.6-sol', 'auto-gpt · 困难档', 'text-muted-foreground'], ['12,880 / 2,210', 'gpt-5.6-sol', 'auto-gpt · 顾问', 'text-warning'], ['3,402 / 512', 'gpt-5.5', 'auto-gpt · 普通档 · 打分失败', 'text-destructive'], ['1,240 / 86', 'gpt-5.6-luna', '路由打分 · auto-gpt', 'text-muted-foreground']]; const problems = []; const colors = {}; for (const [tokens, model, text, cls] of want) { const row = rows[find(tokens)]; const span = caption(row, text); if (!row.innerText.includes(model)) problems.push(tokens + ' 的模型列不是 ' + model); if (!span) { problems.push(tokens + ' 没有小字「' + text + '」'); continue; } if (!span.classList.contains(cls)) problems.push('「' + text + '」颜色类是 ' + span.className); colors[cls] = getComputedStyle(span).color; } if (new Set(Object.values(colors)).size !== 3) problems.push('灰、黄、红三种颜色没有区分开：' + JSON.stringify(colors)); if (find('1,240 / 86') !== find('38,120 / 1,904') + 1) problems.push('打分记录没有紧挨在主请求下面'); if (problems.length) throw new Error(problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "list-captions"
    }
  ],
  "teardown": []
}
```
## 如何验收
- [ ] 困难档主请求：模型 gpt-5.6-sol，小字「auto-gpt · 困难档」灰字
- [ ] 交给顾问：小字「auto-gpt · 顾问」黄字
- [ ] 打分失败：小字「auto-gpt · 普通档 · 打分失败」红字
- [ ] 打分记录：模型 gpt-5.6-luna，小字「路由打分 · auto-gpt」灰字，紧挨在它的主请求下面
