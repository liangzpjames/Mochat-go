# 前端逐页迁移 Runbook

## 单页迁移流程

1. 从 `docs/handle/frontend-audit/pages.csv` 领取一条 `candidate`；`blocked` 项先解除许可证、账号、API 或资产阻塞。
2. 在 pages、routes、apis、permissions、assets、dependencies 六类审计矩阵中补齐 API、权限和企业/租户基线；浏览器截图、视觉差异和命令输出记录到 `docs/evidence/frontend/<route-slug>/`，并在验收表引用。
3. 先写失败测试：页面单元测试、API 契约测试，以及对应的 Playwright 场景。
4. 在所属 React app 实现页面；服务端数据交给 TanStack Query，客户端瞬时状态才进入 Zustand。
5. 依次运行单页测试、契约测试、`frontend_check quick`、`frontend_check build` 和 `frontend_check e2e`。
6. 只把该页面的一条 manifest 记录从 `legacy` 切到 `react`；禁止默认 fallback。
7. 同一提交更新 pages/routes/apis/permissions CSV 和风险台账，并记录验证证据。
8. 使用独立提交交付，确保可单独发布和回滚。

`audit_legacy_frontend_inventory.mjs --refresh` 当前会把 pages 的 `status/owner/risk/batch` 重置为默认值。开始批量维护 134 条迁移元数据前，必须先为生成器增加受版本控制的 metadata override 输入及防覆盖测试；在此之前不得直接运行 `--refresh` 后提交生成结果。

## 发布与回滚

- 发布前确认 manifest、TypeScript 与 Go 读取同一份 JSON，并检查未知路径仍返回 React 404。
- 单页失败时只把对应 manifest 项改回 `legacy`，保留 React 实现和失败证据。
- API、权限或租户语义不确定时不得切换 manifest。
- 回滚必须更新风险台账，写明影响路由、证据、负责人和重新启用条件。

## 后续批次

排序规则：先解除 `blocked`；其余按风险从低到高、共享 API 覆盖率从高到低、页面依赖从少到多排列。

| 批次 | 范围 | 独立发布/回滚边界 | 开始条件 |
| --- | --- | --- | --- |
| Dashboard batch 2 | 高复用基础 CRUD 与列表页 | 每个路由单独 manifest 切换 | 先为 134 条未分配页面补齐 owner/risk/batch |
| Dashboard batch 3 | 素材、富文本、图表、二维码等高依赖页面 | 资源能力按包交付，页面逐条切换 | 许可证与资源来源清晰 |
| Dashboard batch 4 | 复杂营销、异步任务和跨页面流程 | 按业务流程拆分，可整组回滚 manifest | 异步状态和真实账号证据具备 |
| Sidebar migration | Sidebar React app 与共享契约 | 独立 app 发布，保留旧 dist 回滚 | Dashboard 共享包稳定 |
| Operation migration | Operation React app 与共享契约 | 独立 app 发布，保留旧 dist 回滚 | Sidebar 模式复用验证完成 |
| legacy 移除 | 删除旧源码运行引用与 dist | 单独清理提交，可恢复上一发布物 | manifest 无 legacy、观测窗口完成、许可证归档完成 |

## 每批完成定义

- frozen lockfile、quick、build、E2E 和 Linux Go/架构门禁通过；
- 无新增未登记 API、权限、资产或依赖；
- 有发布、监控、回滚负责人；
- 台账与交接文档指向下一条可直接领取的审计记录。
