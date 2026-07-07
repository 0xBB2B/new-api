---
name: frontend-config-ui
description: 前端渠道类型下拉含该类型;Key 提示粘贴 Claude Code OAuth JSON;编辑抽屉提供刷新凭据按钮与合规免责声明;隐藏 batch/多 key 入口。
---

# 前端配置界面

## 目的
让管理员在 web 端能选择、配置并维护 Claude 订阅渠道,凭据输入与刷新操作清晰可用。

## 逻辑
- 渠道类型下拉中出现 `Claude Subscription` 选项。
- Key 输入区提示管理员粘贴 Claude Code 的 OAuth JSON 凭据(`claudeAiOauth`),而非普通 API key。
- 编辑抽屉为该类型渲染:
  - 「刷新凭据」按钮,点击触发后端手动刷新并回显结果。
  - 合规免责声明:提示 OAuth 凭据仅供个人订阅使用、需遵守 Anthropic 服务条款。
  - 凭据获取指引:按操作系统(macOS Keychain / Linux·Windows 文件)给出可复制的命令,命令产出可直接粘贴到 Key 字段的 `{"claudeAiOauth":{...}}`。
- 隐藏/禁用 batch 创建与多 key 输入(与单 key 约束一致)。

## 约束
- 渠道类型下拉可见并可选 `Claude Subscription`。
- 该类型的 Key 输入提示文案指向「Claude Code OAuth JSON 凭据」。
- 编辑该类型渠道时可见「刷新凭据」按钮与合规免责声明。
- 该类型抽屉展示按操作系统的凭据获取命令,可复制;命令产出 `{"claudeAiOauth":{...}}`。
- 该类型表单不出现 batch/多 key 入口。

## 例子
- 管理员打开新建渠道抽屉,类型选 `Claude Subscription` → Key 输入框提示「粘贴 Claude Code OAuth JSON 凭据」,下方出现免责声明,无批量创建入口。
- 编辑已存在的该类型渠道 → 抽屉内出现「刷新凭据」按钮,点击后调用后端刷新并提示成功/失败。

## 验收
- [ ] 渠道类型下拉含 `Claude Subscription` 且可选。
- [ ] 该类型 Key 输入提示为「Claude Code OAuth JSON 凭据」。
- [ ] 编辑该类型渠道时展示「刷新凭据」按钮与合规免责声明。
- [ ] 该类型抽屉展示 macOS/Linux/Windows 三条可复制的凭据获取命令,输出为 `{"claudeAiOauth":{...}}`。
- [ ] 该类型表单不出现 batch/多 key 入口。
