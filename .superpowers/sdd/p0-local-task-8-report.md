# P0 本地闭环 Task 8 实施报告：0165 受控迁移与供应链

## 1. 实施范围与边界

本任务只使用隔离 worktree、临时无命名卷 MariaDB/MySQL 5.7 容器、fixture 和本地测试。未调用真实企业微信、真实 AI Provider、生产服务器或真实租户凭据；未删除或修改任何既有 Docker 命名卷；未修改 `docs/PROJECT_PROGRESS.zh-CN.md`。

## 2. 0165 根因与方案

### 根因

历史 `0165_ai_daily_insight_unification.up.sql` 会删除重复日洞察，并删除 legacy `mochat_go_ai_analysis` 表；down 迁移只能重建空结构，无法恢复已删除数据。普通 migration runner 原本可以直接执行它，缺少执行前盘点、可验证备份、显式批准和并发冻结，因此“有 down 文件”并不等于可恢复。

### 为什么采用独立受控控制器

不能修改已经发布的 0165 SQL 或 checksum，否则会破坏历史 migration ledger。实现把 0165 注册为 controlled migration，并提供独立 `mochat-ai-insight-0165` 命令与 `go run scripts/preflight_0165_ai_daily_insight_unification.go` 入口。控制器按 `inventory → backup → preflight → apply → verify` 推进：

- inventory 记录源表、重复集、legacy 待删除集、逐表 count 与确定性 SHA-256；
- backup 创建独立备份表和 manifest，备份缺失、count/hash 漂移均 fail closed；
- preflight 生成绑定 request、schema、源/备份快照与 SQL checksum 的批准 token；
- apply 使用数据库 named lock 和临时写保护 trigger，批准后仍有并发写入会拒绝执行；
- verify 同时核对目标约束、源/备份快照与结果，最后才写 controlled success ledger 和标准 migration ledger；
- 已执行 0165、错误 schema、重复执行、错误 token、legacy/count/hash 漂移全部拒绝，不伪造历史恢复。

MySQL 5.7 不支持历史 SQL 中的 `ALTER ... ADD/DROP ... IF [NOT] EXISTS`。控制器先严格验证预期列/索引状态，仅在内存执行计划中移除这几个不兼容修饰符；磁盘上的 0165 文件和规范化 checksum `4575a0d89e59cf0b87059c0d60575be3e5cc566ee7338cc6fb6f616e8431520f` 保持不变。

受控迁移操作手册见 `docs/deployment/2026-08-29-0165-controlled-migration.zh-CN.md`。

## 3. 供应链根因与方案

原 Go patch、基础镜像 tag 和 Action major tag 均可漂移；前端锁图存在 1 critical、8 high、3 moderate。实施内容：

- `toolchain go1.26.7`，CI 显式 1.26.7；
- Node 24.20.0、Go 1.26.7、Alpine 3.22.5 固定多架构 index digest；
- workflow 全部 `uses:` 固定官方 peeled commit SHA；
- React Router 7.18.2、Vitest 3.2.6，并精确修复 brace-expansion、js-yaml、nanoid 传递路径；
- CI 增加 `govulncheck@v1.7.0`、`pnpm audit --audit-level high`、SPDX SBOM 与 SHA-256；
- migration gate 消费 `mochat-migrate inventory`，不再把 `0098` 当作“最新迁移”；
- 新增 `pnpm check:supply-chain`，以测试锁住上述合同。

完整依赖路径、digest、Action SHA 和签名边界见 `docs/deployment/2026-08-29-supply-chain-baseline.zh-CN.md`。

## 4. TDD 证据

- RED：0165 首个集成测试因 controlled API/常量不存在而编译失败；CLI 测试因参数解析不存在而失败；缺失目标 index 的 schema 测试先失败。
- GREEN：实现控制器和 CLI 后，MariaDB 10.6 与 MySQL 5.7 均通过完整生命周期、漂移拒绝和并发唯一提交测试。
- RED：供应链实况检查首次报告浮动 Actions、旧工具链/镜像、Vitest 2.1.9、React Router 7.18.1、缺少安全 override、缺少 audit/SBOM 以及 workflow 硬编码 0098。
- GREEN：配置升级后 `pnpm check:supply-chain` 通过。
- 回归 RED/GREEN：Vitest 3 首轮全量测试只发现 `@mochat/config` 仍断言旧版 2.1.9；更新该版本合同后整套前端测试通过。

## 5. 当前验证证据

### PASS

- `go version`：`go1.26.7 windows/amd64`；
- MariaDB 10.6：0165 lifecycle、snapshot/backup drift、错误 schema、已执行边界、并发 apply exactly once；
- MySQL 5.7：同一组 0165 测试全部通过；
- `pnpm audit --audit-level high`：`No known vulnerabilities found`；
- `govulncheck@v1.7.0 ./...`：可达漏洞 0；另报告 imported packages 2、required modules 3 个不可达漏洞，不伪装成依赖树零漏洞；
- `pnpm lint`、`pnpm typecheck`：通过；
- `pnpm test`：所有 workspace 测试通过，其中 Dashboard 147 files / 969 tests；既有 jsdom `window.getComputedStyle(..., pseudoElt)` stderr 不影响退出码；
- `pnpm build`：四端与共享包构建通过；
- `pnpm check:supply-chain`：策略单测与仓库实况检查通过。

### 待本任务收尾复核

- 精确提交前的 Go 全量 test/vet；
- Docker 多阶段构建与新 0165 CLI 二进制存在性；
- workflow YAML/动态 inventory smoke 最终合同；
- 独立 SBOM 运行产物（CI 接线已完成）。

### SKIP / NOT CONFIGURED

- GitHub 签名 artifact attestation / signed provenance：当前没有正式 release subject、OIDC 与签名权限，未配置，未伪造；
- 真实企业微信、真实 AI Provider、生产部署：不在本地闭环授权范围内。
