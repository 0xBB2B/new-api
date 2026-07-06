# quota-reset 实施计划

## 阶段 1：后端核心

- [global-rule-setting](01-global-rule-setting.md) — 全局规则配置模块 + option 校验
- [user-rule-storage](02-user-rule-storage.md) — 每用户规则存储 + 管理员写入端点
- [reset-engine](03-reset-engine.md) — 规则解析、时点计算、批量重置执行 [依赖: global-rule-setting, user-rule-storage]
- [schedule-and-manual](04-schedule-and-manual.md) — 定时 ticker + 手动触发端点 [依赖: reset-engine]

## 阶段 2：可见性与前端

- [self-api-visibility](05-self-api-visibility.md) — GetSelf 扩展 quota_reset 字段 [依赖: reset-engine]
- [admin-settings-ui](06-admin-settings-ui.md) — 系统设置全局规则 section [依赖: global-rule-setting]
- [admin-users-ui](07-admin-users-ui.md) — 用户抽屉规则配置 + 全员重置按钮 [依赖: user-rule-storage, schedule-and-manual]
- [wallet-ui](08-wallet-ui.md) — 钱包页下次重置展示 + i18n 收口 [依赖: self-api-visibility]
