# 风险台账

| ID | 风险 | 等级 | 状态 | 应对 | 负责人 | 截止 |
| --- | --- | --- | --- | --- | --- | --- |
| R-001 | 三个巨型文件继续增长导致回归面扩大 | 高 | 控制中 | 字节基线门禁；触碰即迁移 | 待指定 | 持续 |
| R-002 | `main.go` 装配复杂且角色边界仍是第一阶段包裹 | 高 | 控制中 | 后续逐块迁入 app/platform | 待指定 | Phase 2 |
| R-003 | 旧 dashboard/sidebar/operation 可编辑源码的许可证与部分资产来源未完全确认 | 高 | 部分缓解 | 源码已从备份恢复；blocked 资产未澄清前不得复制到 React | 待指定 | 每批迁移前 |
| R-004 | 无真实企微、微信开放平台和生产环境 | 高 | 未解决 | 保持 readiness 未完成，不用 fake 冒充 | 待指定 | 资源具备后 |
| R-005 | 本地仅依赖 Docker，首次拉取镜像/依赖较慢 | 中 | 已缓解 | 使用 named volume 缓存和固定镜像 | 待指定 | 已执行 |
| R-006 | Windows 缺 Git Bash 时不能直接运行 `.sh` | 中 | 已缓解 | 提供 Docker Compose 等价入口 | 待指定 | 已执行 |
| R-007 | 远端 `main` 替换造成旧历史难查 | 高 | 已缓解 | 已建立并验证 `origin/backup/pre-phase0-main-20260723` | 待指定 | 持续保留 |
| R-008 | 仓库可能混入运行证据中的环境信息 | 高 | 控制中 | 提交前文件名、token 模式和大文件扫描 | 待指定 | 每次推送 |
| R-009 | Windows Go 测试受路径分隔符与 POSIX mode 断言影响 | 中 | 已知限制 | 不在前端任务内修改；以 Linux CI/Docker 为权威门禁 | 待指定 | 后端专项 |
| R-010 | Dashboard 单包约 1.19 MB，后续页面会继续增大首屏资源 | 中 | 控制中 | Dashboard batch 2 起引入按页动态加载和 bundle 基线 | 待指定 | 下一批 |
| R-011 | Playwright/CI 首次安装浏览器耗时且可能受网络抖动影响 | 中 | 控制中 | lockfile hash 缓存 pnpm store 与浏览器；保留失败 trace | 待指定 | 持续 |
| R-012 | legacy E2E 当前主要验证路由与 document，不能完全证明旧业务页可操作 | 中 | 待处理 | 后续批次增加关键资源、启动错误和可见业务内容断言 | 待指定 | Dashboard batch 2 |
| R-013 | `main.go` 与 `internal/store/mysql.go` 已超过长期结构目标，直接把当前体积改成新目标会固化架构债务 | 高 | 控制中 | 负责人 `backend-platform`；长期目标分别保持 206017/840395 字节；以 `f2b57f31f2baa93dc871b9157a04b0a8f7e2ae36` 为债务基线，临时 ratchet 上限已随本次拆分下调到当前精确值 232325/913526；已将 callback、archive、credential 装配迁出 main，后续按触碰切片继续下调；门禁要求元数据完整、期限不超过 90 天且到期失败 | backend-platform | 2026-11-28 |
