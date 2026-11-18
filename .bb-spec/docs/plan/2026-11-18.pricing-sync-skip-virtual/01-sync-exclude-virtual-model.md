---
name: 01-sync-exclude-virtual-model
description: 定价同步拉取接口的差异和价格列表跳过与当前虚拟模型同名的模型
---
# 同步结果排除虚拟模型名
## 目标
`POST /api/ratio_sync/fetch`（及同一处理函数的 `/api/channel/fetch_upstream_ratios`）返回的 `differences` 与 `prices` 里不出现和智能路由当前虚拟模型名完全一致的模型。
## 业务规则（来源：PRD 01-sync-exclude-virtual-model）
- 虚拟模型：管理员在智能路由里配置的模型名（`SmartRoute.Name`），请求时被换成真实档位模型计费，自身价格永远用不到；且价格表里有它的条目时，保存智能路由会被 `service/smartroute/validate.go:48` 拒绝。
- 差异列表不包含与拉取时虚拟模型名完全一致（区分大小写）的模型。
- 价格列表同样不包含它们，`include_unchanged` 为 true 时的完整列表也一样。
- 只在大小写或其他字符上不同的模型名照常返回（如虚拟模型 `auto`，上游 `Auto` 照常出现）。
- 没有任何虚拟模型时，结果和改动前完全一致。
- 被跳过名字的本地价格条目不被修改（拉取接口本身只读，测试确认本地条目仍在）。
- 判断用拉取那一刻的智能路由配置，改名后下一次拉取立即生效，不需要重启。
- 跳过时不提示，不改前端。
## 涉及文件
- 修改 `controller/ratio_sync.go`
- 修改 `controller/ratio_sync_test.go`
## 函数清单
### controller/ratio_sync.go
| 函数名 | 职责 |
|---|---|
| `buildDifferences` | 汇总 `allModels` 时跳过 `operation_setting.FindSmartRoute` 命中的模型名，差异列表因此不含虚拟模型名 |
| `FetchUpstreamRatios` | `include_unchanged` 补全 `models` 集合时同样跳过虚拟模型名，价格列表因此不含虚拟模型名 |
## 协作关系
- 两处都直接调用已有的 `operation_setting.FindSmartRoute`（完全一致比较，读原子指针里的当前配置），不新增函数。
- `prices` 的模型集合来自 `differences` 的键和 `include_unchanged` 补全两处，两处都跳过即覆盖全部输出。
## 验证方式
- 测试入口：`controller.FetchUpstreamRatios`，沿用 `controller/ratio_sync_test.go` 里 `modelManagementRequest` + `httptest.NewServer` 假上游的写法，扩展现有测试文件、不新建文件。
- 测试输入：
  - 用 `operation_setting.LoadSmartRoutesFromJSONString` 设置虚拟模型（如 `[{"name":"sync-virtual"}]`），`t.Cleanup` 恢复原配置（`SmartRoutingRoutesToString` 取旧值）。
  - 本地价格表 `model_ratio` 含 `sync-virtual: 3`（用 `ratio_setting.UpdateModelRatioByJSONString`，Cleanup 恢复）。
  - 假上游 `/pricing` 返回 `sync-virtual`（model_ratio 5）、`Sync-Virtual`（model_ratio 5）、普通模型 `sync-normal`（model_ratio 2）。
- 预期结果：
  - `include_unchanged` false：`differences`、`prices` 都不含 `sync-virtual`；含 `Sync-Virtual` 与 `sync-normal`。
  - `include_unchanged` true：`prices` 仍不含 `sync-virtual`。
  - 拉取后本地 `model_ratio` 里 `sync-virtual` 仍为 3。
  - 把虚拟模型改名为 `sync-renamed` 后再拉取：`sync-virtual` 出现在 `differences`。
  - 不配置虚拟模型时：`sync-virtual` 出现在 `differences`。
- [ ] `go test ./controller/ -run PricingSync` 通过
- [ ] `go build ./...` 通过，`gofmt` 无改动
