---
name: effort-i18n-seven-languages
description: 七种界面语言下，日志详情的「思考力度」标签和降档、升档、客户端传入、固定、不支持思考、客户端未指定六种写法都有翻译
category: usage-logs-routing
---
# 日志思考力度的七种语言
## 简介
管理员打开使用日志，先在简体中文下核对六条日志详情里的「思考力度」，再依次切到 English、Français、Русский、日本語、Tiếng Việt、繁體中文，每种语言都重新打开这六条详情核对。期望文字由生成脚本按各语言的 web/src/i18n/locales/*.json 拼出来写进用例。
## 测试目的
二期 PRD 06 验收 C5 后半：切换七种语言，日志详情里「思考力度」的新增文字都有翻译。前半（普通用户和管理员看到的这一行相同）由 user-view-and-plain-log 和 effort-eight-kinds 一起覆盖：两边看 req_alice01 都是「medium（不覆盖，客户端传入）」。行按 Token 数字定位，数字的千分位写法随语言变化，脚本里先去掉分隔符再比较。
## 测试流程（固定 JSON，subagent 原样消费）
```json
{
  "id": "effort-i18n-seven-languages",
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
      "script": "async () => { const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const label = \"思考力度\"; const targets = [{\"tokens\": \"3402 / 512\", \"want\": \"high（跟随打分，由 xhigh 降档）\"}, {\"tokens\": \"1357 / 79\", \"want\": \"low（不覆盖，由 minimal 升档）\"}, {\"tokens\": \"12880 / 2210\", \"want\": \"low（不覆盖，客户端传入）\"}, {\"tokens\": \"86420 / 420\", \"want\": \"high（固定）\"}, {\"tokens\": \"2468 / 135\", \"want\": \"未发送（模型不支持思考）\"}, {\"tokens\": \"3579 / 246\", \"want\": \"未发送（客户端未指定）\"}]; const norm = (s) => s.replace(/(\\d)[\\s,.\\u00a0\\u202f](?=\\d{3}\\b)/g, '$1'); const problems = []; await sleep(1500); for (const t of targets) { let row = null; for (let i = 0; i < 100 && !row; i++) { row = [...document.querySelectorAll('tbody tr')].find((r) => norm(r.innerText).includes(t.tokens)); if (!row) await sleep(100); } if (!row) { problems.push('找不到 Token 为 ' + t.tokens + ' 的记录'); continue; } row.querySelector('button[title]').click(); let dlg = null; for (let i = 0; i < 100 && !dlg; i++) { dlg = document.querySelector('[role=dialog]'); if (!dlg || !dlg.innerText.includes(label)) { dlg = null; await sleep(100); } } if (!dlg) { problems.push(t.tokens + ' 的详情里没有「' + label + '」'); continue; } const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const i = lines.indexOf(label); if (lines[i + 1] !== t.want) problems.push(t.tokens + '：' + lines[i + 1] + '，期望 ' + t.want); const keys = dlg.innerText.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push(t.tokens + ' 出现翻译占位：' + keys.join(',')); dlg.querySelector('[data-slot=dialog-close]').click(); for (let k = 0; k < 50 && document.querySelector('[role=dialog]'); k++) await sleep(100); } if (problems.length) throw new Error(\"zh\" + '：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"English\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const label = \"Reasoning effort\"; const targets = [{\"tokens\": \"3402 / 512\", \"want\": \"high (Follow scoring, lowered from xhigh)\"}, {\"tokens\": \"1357 / 79\", \"want\": \"low (Not overridden, raised from minimal)\"}, {\"tokens\": \"12880 / 2210\", \"want\": \"low (Not overridden, sent by the client)\"}, {\"tokens\": \"86420 / 420\", \"want\": \"high (Fixed)\"}, {\"tokens\": \"2468 / 135\", \"want\": \"Not sent (model does not support thinking)\"}, {\"tokens\": \"3579 / 246\", \"want\": \"Not sent (client did not specify)\"}]; const norm = (s) => s.replace(/(\\d)[\\s,.\\u00a0\\u202f](?=\\d{3}\\b)/g, '$1'); const problems = []; await sleep(1500); for (const t of targets) { let row = null; for (let i = 0; i < 100 && !row; i++) { row = [...document.querySelectorAll('tbody tr')].find((r) => norm(r.innerText).includes(t.tokens)); if (!row) await sleep(100); } if (!row) { problems.push('找不到 Token 为 ' + t.tokens + ' 的记录'); continue; } row.querySelector('button[title]').click(); let dlg = null; for (let i = 0; i < 100 && !dlg; i++) { dlg = document.querySelector('[role=dialog]'); if (!dlg || !dlg.innerText.includes(label)) { dlg = null; await sleep(100); } } if (!dlg) { problems.push(t.tokens + ' 的详情里没有「' + label + '」'); continue; } const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const i = lines.indexOf(label); if (lines[i + 1] !== t.want) problems.push(t.tokens + '：' + lines[i + 1] + '，期望 ' + t.want); const keys = dlg.innerText.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push(t.tokens + ' 出现翻译占位：' + keys.join(',')); dlg.querySelector('[data-slot=dialog-close]').click(); for (let k = 0; k < 50 && document.querySelector('[role=dialog]'); k++) await sleep(100); } if (problems.length) throw new Error(\"en\" + '：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"Français\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const label = \"Effort de raisonnement\"; const targets = [{\"tokens\": \"3402 / 512\", \"want\": \"high (Suit le score, abaissé depuis xhigh)\"}, {\"tokens\": \"1357 / 79\", \"want\": \"low (Non remplacé, relevé depuis minimal)\"}, {\"tokens\": \"12880 / 2210\", \"want\": \"low (Non remplacé, envoyé par le client)\"}, {\"tokens\": \"86420 / 420\", \"want\": \"high (Fixe)\"}, {\"tokens\": \"2468 / 135\", \"want\": \"Non envoyé (le modèle ne gère pas la réflexion)\"}, {\"tokens\": \"3579 / 246\", \"want\": \"Non envoyé (non précisé par le client)\"}]; const norm = (s) => s.replace(/(\\d)[\\s,.\\u00a0\\u202f](?=\\d{3}\\b)/g, '$1'); const problems = []; await sleep(1500); for (const t of targets) { let row = null; for (let i = 0; i < 100 && !row; i++) { row = [...document.querySelectorAll('tbody tr')].find((r) => norm(r.innerText).includes(t.tokens)); if (!row) await sleep(100); } if (!row) { problems.push('找不到 Token 为 ' + t.tokens + ' 的记录'); continue; } row.querySelector('button[title]').click(); let dlg = null; for (let i = 0; i < 100 && !dlg; i++) { dlg = document.querySelector('[role=dialog]'); if (!dlg || !dlg.innerText.includes(label)) { dlg = null; await sleep(100); } } if (!dlg) { problems.push(t.tokens + ' 的详情里没有「' + label + '」'); continue; } const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const i = lines.indexOf(label); if (lines[i + 1] !== t.want) problems.push(t.tokens + '：' + lines[i + 1] + '，期望 ' + t.want); const keys = dlg.innerText.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push(t.tokens + ' 出现翻译占位：' + keys.join(',')); dlg.querySelector('[data-slot=dialog-close]').click(); for (let k = 0; k < 50 && document.querySelector('[role=dialog]'); k++) await sleep(100); } if (problems.length) throw new Error(\"fr\" + '：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"Русский\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const label = \"Глубина рассуждений\"; const targets = [{\"tokens\": \"3402 / 512\", \"want\": \"high (По оценке, понижено с xhigh)\"}, {\"tokens\": \"1357 / 79\", \"want\": \"low (Не переопределяется, повышено с minimal)\"}, {\"tokens\": \"12880 / 2210\", \"want\": \"low (Не переопределено, передано клиентом)\"}, {\"tokens\": \"86420 / 420\", \"want\": \"high (фиксированная)\"}, {\"tokens\": \"2468 / 135\", \"want\": \"Не отправлено (модель не поддерживает рассуждения)\"}, {\"tokens\": \"3579 / 246\", \"want\": \"Не отправлено (клиент не указал)\"}]; const norm = (s) => s.replace(/(\\d)[\\s,.\\u00a0\\u202f](?=\\d{3}\\b)/g, '$1'); const problems = []; await sleep(1500); for (const t of targets) { let row = null; for (let i = 0; i < 100 && !row; i++) { row = [...document.querySelectorAll('tbody tr')].find((r) => norm(r.innerText).includes(t.tokens)); if (!row) await sleep(100); } if (!row) { problems.push('找不到 Token 为 ' + t.tokens + ' 的记录'); continue; } row.querySelector('button[title]').click(); let dlg = null; for (let i = 0; i < 100 && !dlg; i++) { dlg = document.querySelector('[role=dialog]'); if (!dlg || !dlg.innerText.includes(label)) { dlg = null; await sleep(100); } } if (!dlg) { problems.push(t.tokens + ' 的详情里没有「' + label + '」'); continue; } const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const i = lines.indexOf(label); if (lines[i + 1] !== t.want) problems.push(t.tokens + '：' + lines[i + 1] + '，期望 ' + t.want); const keys = dlg.innerText.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push(t.tokens + ' 出现翻译占位：' + keys.join(',')); dlg.querySelector('[data-slot=dialog-close]').click(); for (let k = 0; k < 50 && document.querySelector('[role=dialog]'); k++) await sleep(100); } if (problems.length) throw new Error(\"ru\" + '：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"日本語\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const label = \"推論強度\"; const targets = [{\"tokens\": \"3402 / 512\", \"want\": \"high（スコアに追従、xhigh から引き下げ）\"}, {\"tokens\": \"1357 / 79\", \"want\": \"low（上書きしない、minimal から引き上げ）\"}, {\"tokens\": \"12880 / 2210\", \"want\": \"low（上書きなし、クライアントから送信）\"}, {\"tokens\": \"86420 / 420\", \"want\": \"high（固定）\"}, {\"tokens\": \"2468 / 135\", \"want\": \"未送信（モデルが思考に非対応）\"}, {\"tokens\": \"3579 / 246\", \"want\": \"未送信（クライアント未指定）\"}]; const norm = (s) => s.replace(/(\\d)[\\s,.\\u00a0\\u202f](?=\\d{3}\\b)/g, '$1'); const problems = []; await sleep(1500); for (const t of targets) { let row = null; for (let i = 0; i < 100 && !row; i++) { row = [...document.querySelectorAll('tbody tr')].find((r) => norm(r.innerText).includes(t.tokens)); if (!row) await sleep(100); } if (!row) { problems.push('找不到 Token 为 ' + t.tokens + ' 的记录'); continue; } row.querySelector('button[title]').click(); let dlg = null; for (let i = 0; i < 100 && !dlg; i++) { dlg = document.querySelector('[role=dialog]'); if (!dlg || !dlg.innerText.includes(label)) { dlg = null; await sleep(100); } } if (!dlg) { problems.push(t.tokens + ' 的详情里没有「' + label + '」'); continue; } const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const i = lines.indexOf(label); if (lines[i + 1] !== t.want) problems.push(t.tokens + '：' + lines[i + 1] + '，期望 ' + t.want); const keys = dlg.innerText.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push(t.tokens + ' 出现翻译占位：' + keys.join(',')); dlg.querySelector('[data-slot=dialog-close]').click(); for (let k = 0; k < 50 && document.querySelector('[role=dialog]'); k++) await sleep(100); } if (problems.length) throw new Error(\"ja\" + '：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"Tiếng Việt\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const label = \"Mức suy luận\"; const targets = [{\"tokens\": \"3402 / 512\", \"want\": \"high (Theo điểm, hạ từ xhigh)\"}, {\"tokens\": \"1357 / 79\", \"want\": \"low (Không ghi đè, nâng từ minimal)\"}, {\"tokens\": \"12880 / 2210\", \"want\": \"low (Không ghi đè, do máy khách gửi)\"}, {\"tokens\": \"86420 / 420\", \"want\": \"high (Cố định)\"}, {\"tokens\": \"2468 / 135\", \"want\": \"Không gửi (mô hình không hỗ trợ suy luận)\"}, {\"tokens\": \"3579 / 246\", \"want\": \"Không gửi (máy khách không chỉ định)\"}]; const norm = (s) => s.replace(/(\\d)[\\s,.\\u00a0\\u202f](?=\\d{3}\\b)/g, '$1'); const problems = []; await sleep(1500); for (const t of targets) { let row = null; for (let i = 0; i < 100 && !row; i++) { row = [...document.querySelectorAll('tbody tr')].find((r) => norm(r.innerText).includes(t.tokens)); if (!row) await sleep(100); } if (!row) { problems.push('找不到 Token 为 ' + t.tokens + ' 的记录'); continue; } row.querySelector('button[title]').click(); let dlg = null; for (let i = 0; i < 100 && !dlg; i++) { dlg = document.querySelector('[role=dialog]'); if (!dlg || !dlg.innerText.includes(label)) { dlg = null; await sleep(100); } } if (!dlg) { problems.push(t.tokens + ' 的详情里没有「' + label + '」'); continue; } const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const i = lines.indexOf(label); if (lines[i + 1] !== t.want) problems.push(t.tokens + '：' + lines[i + 1] + '，期望 ' + t.want); const keys = dlg.innerText.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push(t.tokens + ' 出现翻译占位：' + keys.join(',')); dlg.querySelector('[data-slot=dialog-close]').click(); for (let k = 0; k < 50 && document.querySelector('[role=dialog]'); k++) await sleep(100); } if (problems.length) throw new Error(\"vi\" + '：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"繁體中文\"s]"
    },
    {
      "action": "evaluate",
      "script": "async () => { const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); const label = \"思考力度\"; const targets = [{\"tokens\": \"3402 / 512\", \"want\": \"high（跟隨評分，由 xhigh 降檔）\"}, {\"tokens\": \"1357 / 79\", \"want\": \"low（不覆蓋，由 minimal 升檔）\"}, {\"tokens\": \"12880 / 2210\", \"want\": \"low（不覆蓋，用戶端傳入）\"}, {\"tokens\": \"86420 / 420\", \"want\": \"high（固定）\"}, {\"tokens\": \"2468 / 135\", \"want\": \"未發送（模型不支援思考）\"}, {\"tokens\": \"3579 / 246\", \"want\": \"未發送（用戶端未指定）\"}]; const norm = (s) => s.replace(/(\\d)[\\s,.\\u00a0\\u202f](?=\\d{3}\\b)/g, '$1'); const problems = []; await sleep(1500); for (const t of targets) { let row = null; for (let i = 0; i < 100 && !row; i++) { row = [...document.querySelectorAll('tbody tr')].find((r) => norm(r.innerText).includes(t.tokens)); if (!row) await sleep(100); } if (!row) { problems.push('找不到 Token 为 ' + t.tokens + ' 的记录'); continue; } row.querySelector('button[title]').click(); let dlg = null; for (let i = 0; i < 100 && !dlg; i++) { dlg = document.querySelector('[role=dialog]'); if (!dlg || !dlg.innerText.includes(label)) { dlg = null; await sleep(100); } } if (!dlg) { problems.push(t.tokens + ' 的详情里没有「' + label + '」'); continue; } const lines = dlg.innerText.split('\\n').map((s) => s.trim()).filter(Boolean); const i = lines.indexOf(label); if (lines[i + 1] !== t.want) problems.push(t.tokens + '：' + lines[i + 1] + '，期望 ' + t.want); const keys = dlg.innerText.match(/\\{\\{[^}]*\\}\\}/g); if (keys) problems.push(t.tokens + ' 出现翻译占位：' + keys.join(',')); dlg.querySelector('[data-slot=dialog-close]').click(); for (let k = 0; k < 50 && document.querySelector('[role=dialog]'); k++) await sleep(100); } if (problems.length) throw new Error(\"zh-TW\" + '：' + problems.join('；')); return 'ok'; }",
      "expect": "ok"
    },
    {
      "action": "screenshot",
      "name": "effort-i18n-last"
    }
  ],
  "teardown": [
    {
      "action": "click",
      "selector": "role=button[name=/^(更改语言|更改語言|Change language|Changer de langue|言語を変更|Изменить язык|Đổi ngôn ngữ)$/]"
    },
    {
      "action": "click",
      "selector": "role=menuitem[name=\"简体中文\"s]"
    }
  ]
}
```
## 如何验收
- [ ] 简体中文下六条详情依次是：high（跟随打分，由 xhigh 降档）、low（不覆盖，由 minimal 升档）、low（不覆盖，客户端传入）、high（固定）、未发送（模型不支持思考）、未发送（客户端未指定）
- [ ] 其他六种语言下，「思考力度」标签和六种写法都是该语言的译文，没有 {{…}} 占位
- [ ] 结束后语言切回简体中文
