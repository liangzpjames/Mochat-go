# 数据概览单页优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 `/index` 六区域业务驾驶舱并部署到 `mochat-go-desktop` 供用户验收。

**Architecture:** 扩展现有 Dashboard overview 前端数据合同以保留 Go 接口已有汇总字段，页面将汇总与趋势投影为独立业务模块。没有真实来源的数据保持显式零值/空态，不添加演示数据。

**Tech Stack:** React、TypeScript、TanStack Query、Vitest、CSS、Go、Docker Compose。

## Global Constraints

- 仅修改 `/index` 相关实现和 Phase 3.2 文档。
- 保留现有权限、查询参数、刷新、分页和 CSV 导出合同。
- 必须先写失败测试，再实现生产代码。
- 最终使用 `mochat-go-desktop` 原有数据卷重建部署。

---

### Task 1: 扩展数据合同

- [ ] 在 API 测试中断言汇总字段被保留并先观察失败。
- [ ] 扩展 `DashboardOverview` 类型和解析器。
- [ ] 运行 API 测试通过。

### Task 2: 实现六区域页面

- [ ] 在页面测试中断言无数据时仍展示六个业务区域并先观察失败。
- [ ] 重构页面组件，保留原查询、刷新、导出、分页和错误状态。
- [ ] 运行页面测试通过。

### Task 3: 统一视觉和响应式布局

- [ ] 增加样式合同测试并观察失败。
- [ ] 完成指标卡、模块卡、趋势、排行、轨迹和响应式样式。
- [ ] 运行 Dashboard 全量测试、类型检查和构建。

### Task 4: 部署与浏览器校验

- [ ] 使用原 Compose 项目名重建应用且不删除数据卷。
- [ ] 检查容器健康和 `/readyz`。
- [ ] 在本地浏览器刷新 `/index`，对比视觉与六区域可达性。
