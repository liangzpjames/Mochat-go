# Phase 2.2：后端模块化与质量门禁

## 状态

实施中。

后端功能门禁已通过原生命令或计划允许的 Linux Docker 等价验证；完整记录见 [Phase 2.2 后端验收](./acceptance.md)。当前仅剩工作树清洁度阻塞：既有 `.superpowers/sdd/*` 协调报告不在本任务提交范围内，尚未取得提交后空的 `git status --short`，因此不得标记为已完成或宣告 Phase 3 后端就绪。

## 目标

在 Phase 3 开工前，为 Go 后端建立可跨平台执行的架构质量门禁、生产模块装配机制和 SCRM 线索纵向薄切片。后续新增业务必须优先进入 `internal/modules/<domain>`，并通过依赖边界、租户隔离、幂等、migration、单元测试、集成测试和 race 门禁。

## 范围

- Go 架构依赖检查；
- 模块级路由注册；
- `internal/modules/scrm` 生产骨架；
- 默认关闭的线索创建与查询薄切片；
- MySQL 租户隔离与幂等验证；
- migration 生命周期；
- CI、开发检查与 PR 约束；
- Phase 3 实施计划修订。

## 非目标

- 全面重构遗留 `dashboard/store/server/main`；
- 发布正式 Phase 3 产品功能；
- 实现完整 SCRM、报表、风险、会话存档或 AI。

## 设计

正式设计见 [Phase 2.2 后端模块化与质量门禁设计](../../superpowers/specs/2026-07-30-phase2-2-backend-quality-gates-design.md)。

## 阶段门槛

Phase 2.2 验收通过前，不开始 Phase 3 正式后端功能开发。紧急修复和不新增业务边界的遗留维护不受此限制，但不得借维护继续扩大遗留巨型文件。
