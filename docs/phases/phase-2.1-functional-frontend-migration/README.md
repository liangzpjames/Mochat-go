# Phase 2.1：前端功能迁移

## 阶段目标

Phase 2.1 把 Phase 2 已完成的 React 路由接管继续推进为真实功能迁移。旧 Vue 源码中的字段、权限、API 和操作语义是迁移基线；React 页面采用当前组件体系和按功能域组织的代码结构。

完成标准不是“路由能打开”，而是对应功能能够读取真实 Go API 数据、完成适用的基础写操作，并通过自动化与人工浏览器点击验收。

## 功能矩阵

[`functional-matrix.csv`](functional-matrix.csv) 是唯一完成状态台账。每一条 manifest 路由必须且只能出现一次。

状态规则：

- `implementation` 只有 `functional` 才能通过。
- 能力完成写为 `passed`。
- 确实不适用的能力写为 `not-applicable:<原因>`。
- `missing`、`placeholder` 和 `route-only` 都会阻塞 Phase 2.1。

运行：

```powershell
pnpm check:phase2.1
```

该命令在迁移过程中应持续失败并准确列出剩余缺口；只有全部功能和浏览器证据完成后才允许通过。

## 外部平台边界

浏览器到 Go API、Go API 到 MariaDB/Redis 使用真实链路。企业微信、微信开放平台等外部依赖使用可控 fake 服务完成确定性验收，不作为本阶段展示和基础交互迁移的阻塞项。

## Phase 3 门禁

Phase 2.1 功能矩阵、自动化验收和人工浏览器台账全部通过前，不得开始 Phase 3。
