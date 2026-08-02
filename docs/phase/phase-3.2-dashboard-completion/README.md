# Phase 3.2：Dashboard 八页完整实现

## 当前状态

- 工作分支：`phase3.2/dashboard-eight-real-pages`
- 固定范围：八个页面，不扩展到其他页面。
- 非浏览器门禁：已通过。
- Docker Desktop：使用 Compose 项目 `mochat-go-desktop` 部署，保留既有数据卷。
- 最终浏览器验收：由用户在本地部署环境中执行，结果尚未归档前不宣告 Phase 3.2 最终完成。

## 八页验收入口

| 页面 | 路由 |
| --- | --- |
| 数据概览 | `/index` |
| 全局消息 | `/chat/v2-all` |
| 敏感词 | `/ai-insight/v2/sensitive-word` |
| 线索池 | `/customer/clue/default` |
| 联系人 | `/customer/contact` |
| 商机 | `/customer/opportunity` |
| 客户公海 | `/customer/public-sea` |
| 客户标签 | `/customer/tags` |

本地 Dashboard 入口：`http://localhost:18080/`。登录后依次访问上述路由，重点检查菜单可达、刷新恢复、筛选与分页、详情、写操作后重新读取，以及不同数据权限下的结果。

## 文档目录

### 设计与计划

- `design/2026-07-31-phase3-dashboard-completion-design.md`：本阶段设计基线。
- `plans/2026-08-01-phase3.2-eight-page-parity-plan.md`：八页对标实施计划。
- `plans/2026-07-31-phase3.2-dashboard-implementation-plan.md`：早期实施计划，保留作历史依据。

### 验收与证据

- `acceptance/PHASE3.2_ACCEPTANCE.zh-CN.md`：验收口径、门禁及浏览器验收状态。
- `reports/phase3.2-function-matrix.md`：八页功能、前后端实现和测试证据矩阵。
- `reports/query-performance.md`：查询性能记录。
- `reports/reuse-audit.md`：复用审计记录。
- `evidence/`：页面截图证据；最终验收截图应继续归档在此目录。

### 实施报告

- `reports/tasks/task-1-report.md`：范围基线与门禁。
- `reports/tasks/task-2-report.md`：统一 Dashboard 外壳和视觉组件。
- `reports/tasks/task-3-report.md`：数据概览。
- `reports/tasks/task-4-report.md`：全局消息。
- `reports/tasks/task-5-report.md`：敏感词。
- `reports/tasks/task-6-report.md`：线索池。
- `reports/tasks/task-7-report.md`：联系人。
- `reports/tasks/task-8-report.md`：商机。
- `reports/tasks/task-9-report.md`：客户公海。
- `reports/tasks/task-10-report.md`：客户标签。
- `reports/tasks/task-11-report.md`：八页非浏览器收敛与最终门禁。

## 归档规则

本轮可长期审阅的设计、计划、验收、报告和截图统一保存在本目录。`.superpowers/sdd/` 中未跟踪的 brief、review package 和进度草稿属于临时协作材料，不进入主线文档归档。
