# Phase 2.2：后端模块化与质量门禁

## 状态

已完成（最终修复源码：`39b79fc`）。

最终审查发现的架构 allowlist、fail-closed 治理、CI lifecycle、Phase 3 真实数据库计划和 typed-nil 防御缺口均已通过 TDD 修复。全量 test/vet/build、Linux race、真实 MySQL 5.7 strict integration 与精确 Git archive 的 migration apply/checksum/rollback/replay 已重新执行通过。完整证据见 [Phase 2.2 后端验收](./acceptance.md)。Phase 3 后端可以开始，但必须遵守修订后的 `internal/modules/scrm` 分层路径与正式 API 契约流程。

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

## 最终门禁摘要

- 架构审计按 layer 显式 allowlist，并对 protected file 与 module registration fail-closed；
- 禁止依赖使用 exact-or-subpackage 路径边界，`database/sql/driver`、`net/http/httptest` 等子包不能绕过；
- CI 强制 architecture、race、full test、vet、migration lifecycle 与 strict integration，shell 命令由 Bash AST 验证而非字符串搜索；
- Phase 3 新 MySQL persistence 必须有 tagged、uncached、require-mode integration；
- Router/Server 拒绝或安全处理 typed-nil HTTP 边界；
- 被验证源码提交、tree、archive 哈希和真实数据库证据记录在验收文档。
