---
name: table-driven-with-named-subtests
description: 多输入组合的测试用「struct 表 + t.Run(name)」组织，子测试有可读名并可独立跑。
---

# Table 驱动测试 + 命名子测试

## 目的

让 case 增删对齐同一模板；失败能定位到具体子测试名并单独 `-run` 重跑；避免长函数里靠注释区分场景。

## 逻辑

当同一被测行为要覆盖 ≥2 种输入组合（正常路径 + 边界 + 错误路径、不同上下游类型、不同配置分支）时，把输入与期望值抽成 struct 切片；`for range` 内嵌套 `t.Run(tt.name, func(t *testing.T){ ... })` 单独执行每条 case。

子测试名用领域术语（例如 "azure fallback"、"key change"、"priority wins"），失败时框架输出 `TestXxx/子名` 可直接 `-run` 定位。

单 case 的一次性测试写常规 `TestXxx` 即可，不强制套 table。

## 约束

- table 结构体必含描述性 `name` 字段。
- 循环体内必须使用 `t.Run(tt.name, func(t *testing.T){ ... })`；禁把所有 case 平铺在同一个 `t` 上导致失败只报第一次。
- 子测试名用领域术语，禁 `case1` / `test2` / 序号命名。
- 单 case 一次性测试可直接写 `TestXxx`，不强制套 table。

## 例子

- 输入：一个策略函数 `chooseProvider(cfg)` 在 4 种 cfg 组合下应分别返回 A、B、C 或错误。
- 过程：定义 `tests := []struct{ name string; cfg Config; want string; wantErr bool }{ ... }`；循环 `t.Run(tt.name, func(t *testing.T) { got, err := chooseProvider(tt.cfg); ... })`。
- 预期结果：`go test -run TestChooseProvider/azure_fallback` 可以只跑那一条；任一失败只影响该子测试，其余照常跑完。

## 验收

- [ ] 覆盖多输入组合的测试均以 struct 表 + `t.Run(name)` 组织。
- [ ] 子测试名均为领域术语，非 `case1` / `test2`。
- [ ] 任一子测试可用 `go test -run TestXxx/name` 独立执行。
- [ ] 失败时输出包含 `TestXxx/name`，可直接定位。
