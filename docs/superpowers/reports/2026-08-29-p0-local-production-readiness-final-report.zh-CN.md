# 2026-08-29 本地 P0 生产准备最终验收报告

日期：2026-08-29

分支：`fix/p0-local-closure-20260829`

隔离 worktree：`D:\workspace\mochat-go\mochat-go\.worktrees\p0-local-closure-20260829`

实施基线：`f2b57f31f2baa93dc871b9157a04b0a8f7e2ae36`

代码与供应链收口提交：`95c26a0c7f0c086e6c4956e2ff37c50589eb5e11`

设计：`docs/superpowers/specs/2026-08-29-p0-local-production-readiness-design.zh-CN.md`

实施计划：`docs/superpowers/plans/2026-08-29-p0-local-production-readiness.zh-CN.md`

## 1. 结论

本次完成了“不依赖真实企业微信、真实 AI Provider 或生产服务器”的 P0 代码闭环，并把本地证据与真实 Provider 证据分开记录。没有修改 `docs/PROJECT_PROGRESS.zh-CN.md`，没有调用真实租户凭据，没有部署公网，没有执行 `reset`、`clean` 或删除 Docker 命名卷。

可安全进入主线审阅的代码范围包括：归档运行角色、正式 bridge 注册骨架、callback durable inbox、三端 HTTP 生命周期与 readiness、periodic task panic 恢复、订单幂等、风险/关键词事务、九条陈旧门禁、迁移注册表和 0165 受控迁移、工具链与供应链门禁。

以下内容不能被本报告宣称为生产完成：真实企业微信 Finance/DataZone 拉取、真实媒体下载、真实租户绑定、真实 AI Provider、生产部署/恢复、签名基础设施和首客证据。这些均为明确 `SKIP` 或待生产受控验收边界。

## 2. 根因、方案与实现

### 2.1 归档运行角色与正式 bridge 接线

根因是 durable worker、媒体 worker、scheduled enqueuer 和 legacy scheduler 的注册条件互相裁剪，且 bridge 启动只具备 fixture 路径，无法证明正式 driver 的 fail-closed 装配。

采用职责矩阵而不是继续增加开关特例：API 不启动 worker；worker 只消费 durable/media；scheduler 只发现并入队；all 组合两者；legacy 只在 durable 关闭且 cron 开启时生效。bridge 增加 MySQL 权威 binding source、加密凭据解析、Linux Finance SDK factory、DataZone 注册入口、反向关闭与 drain；fixture 只能显式开启，绝不作为生产 fallback。

本地 fake/契约测试覆盖启动、租户/企业解析、游标、媒体、driver 顺序关闭、部分启动失败回滚和无配置 fail-closed。真实 SDK 网络调用未执行。

### 2.2 企业微信 callback durable ACK

根因是旧 callback 将 enqueue 错误吞掉后仍返回 success，导致上游停止重试而事件未被持久接受；Redis 队列也不能单独承担权威接收账本。

新增迁移 `0172_wework_callback_inbox`，以租户/企业范围的稳定事件键建立数据库 durable inbox。只有数据库返回 `accepted` 或 `duplicate` 才 ACK；数据库故障返回失败以触发上游重试。Redis 仅作可恢复的唤醒加速，不再决定 ACK。lease/fence、失败重试、dead 状态、重放去重和日志脱敏均有故障注入与并发测试。

### 2.3 HTTP 生命周期、readiness 与 periodic task

根因是三端监听方式不一致、缺少统一根 context 和完整 graceful shutdown；旧 `/readyz` 不能稳定区分存活与依赖就绪；periodic goroutine 的单次 panic 会永久退出。

三端统一使用显式 `http.Server`，设置 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`，共享根 context、drain 和有界等待。`/healthz` 只表示进程存活；`/readyz` 以短超时检查静态资源、MySQL、Redis、迁移版本和必要后台任务，并只公开稳定错误码。periodic runner 在每轮边界恢复 panic、记录连续失败和 stale/hung 状态，后续轮次仍能继续运行。

### 2.4 订单幂等

根因是订单创建缺少贯穿 HTTP、service、repository 和数据库的稳定幂等键，并发重试可能产生重复订单或返回不同首次响应。

迁移 `0173_scrm_order_idempotency` 增加租户/企业范围唯一约束和不可变 receipt。幂等键按 opaque bytes 处理，不做会改变语义的大小写或空白归一；同键同请求重放首次响应，同键不同请求拒绝。创建、receipt 和业务写入在同一事务完成，32 路并发测试只保留一份订单。

### 2.5 风险规则与关键词原子性

根因分别是风险扫描固定只取前 100 条、风险记录与 `trigger_count` 分开提交，以及关键词条目保存和 library version 更新没有同一锁顺序与事务。

风险扫描先按主键锁定全部适用规则，再按 high-water/keyset 分批读取数据；策略选择稳定且单一；风险记录、幂等去重和计数在一个事务提交。关键词路径固定按 library → entry 加锁，条目写入和版本递增同事务完成。超过 100 条、重复事件、失败回滚和并发版本均在真实 MariaDB 上通过。

### 2.6 门禁与迁移治理

九条红门禁的共同根因不是业务需要回退，而是脚本绑定旧文件计数、旧路由清单、旧文档目录或 Windows 超长命令。修复后：

- phase34 lint 按八批执行，规避 Windows 命令长度限制；
- Phase 3/3.5、Dashboard auth、single-corp 和 archive activation 从当前运行注册表/行为合同派生；
- phase2、phase2.1 和 docs 检查绑定当前 82 个 manifest/正确链接；
- CI 调整为 build-before-audit；
- MySQL 5.7 smoke 从真实 `0001` 到最新迁移注册表生成，本次为 173 个迁移，覆盖 apply、status、status-read-only、baseline、down/up 生命周期；
- 修复 `0131` baseline 的实际后置条件，确保 `mc_corp.tenant_id` 非空合同被验证。

### 2.7 0165 与供应链

根因是 `0165_ai_daily_insight_unification` 含数据裁剪却缺少备份内容校验和恢复账本；workflow 也可能在过宽权限下运行。

新增受控 0165 工具：备份记录行数、最大 ID 和稳定有序内容 hash；schema ledger 与 verified 状态同事务提交；Apply/Status/StatusReadOnly/Baseline 均拒绝不完整证据。workflow 路径包含 CLI/包装器，并强制顶层 `contents: read`，拒绝 write-all、OIDC、attestations 或 job 权限覆盖。

Go 工具链升级到 1.26.7（包含 1.26.6 后续安全修复）；Golang、Node、Alpine 构建镜像固定 patch 和 index digest；GitHub Actions 固定 commit SHA。React Router/Vitest 完成安全升级。新增 govulncheck、pnpm audit、Syft SPDX SBOM、checksum 和无签名基础设施时的明确 unsigned attestation `SKIP`。

## 3. 测试驱动与审阅

每个实现任务均先提交或运行可复现的失败测试，再实现到 GREEN；任务之间进行了规格与质量复核。累计审阅发现并修复了：bridge fixture fallback、callback 权威身份和 lease fence、HTTP drain/stale 判定、风险规则锁顺序、`0131` baseline 后置条件、0165 证据原子性/内容 hash，以及 workflow 权限过宽等问题。

最终代码收口 `95c26a0c` 后的独立复核未发现 Critical/Important 问题；最终报告提交后仍需以最新 HEAD 再运行 `git diff --check` 和合并可达性检查。

## 4. PASS 证据

### 4.1 全量代码门禁

- `go test ./... -count=1`：PASS。
- `go vet ./...`：PASS。
- `corepack pnpm lint`：PASS。
- `corepack pnpm typecheck`：PASS。
- `corepack pnpm test`：PASS；Dashboard 147 个测试文件、969 项测试通过，SaaS Admin、Sidebar、Operation 和共享包全部通过。
- `corepack pnpm build`：PASS；四端生产构建完成，仅有既有 chunk size 警告。
- 九条目标门禁：PASS；phase34 为 301 文件/8 批，Phase 3.5 为 9/9，Phase 3 final 为 27/27，auth 合同为 597 路由且 missing=0/legacy=0，phase2 为 82/82，phase2.1 为 82/40。
- `docs:check`、依赖合同、供应链 2/2、logging 和 backend quality contract：PASS。
- `git diff --check`：在最终提交后复跑确认。

### 4.2 漏洞与供应链

- `govulncheck ./...`：PASS；可达漏洞为 0。报告的 imported package/module 漏洞均无可达调用路径。
- `corepack pnpm audit --audit-level high`：PASS；`No known vulnerabilities`。
- Syft 1.51.1：SPDX JSON SBOM 与 checksum 生成 PASS。
- Docker 镜像和 Actions pin 合同：PASS。

### 4.3 数据库与并发

- MariaDB 10.6：风险、关键词、callback inbox 聚焦集成测试 PASS。
- MariaDB 10.6：超过 100 条规则、重复、回滚、并发版本测试 PASS。
- MySQL 5.7：订单 repository/idempotency migration 集成测试（`-tags integration`）PASS。
- MySQL 5.7：真实 173 个迁移 apply/status/status-read-only/baseline/down/up 生命周期 PASS。
- 0165 受控备份、验证、回滚与恢复测试 PASS。

### 4.4 Linux/CGO race 与 Finance 合同

本机缓存的 `golang:1.26.7-bookworm` Linux/CGO 环境运行：

`go test -race ./internal/archivebridge ./internal/wecomarchivedemo ./internal/store ./internal/migration ./internal/taskrunner ./internal/server ./cmd/mochat-go ./cmd/mochat-archive-bridge`

结果：全部 PASS。两个 Alpine 尝试因 apk 镜像网络失败而中止，临时容器已自动清理；改用本地缓存的 Debian 工具链后完成等价 race 覆盖。

### 4.5 精确 SHA Docker 与运行态

镜像：`mochat-go:p0-local-95c26a0`

镜像 ID：`sha256:5d25cb01b4eb5dcca06b8e0b1547160f2f91d97d82e1609047f018fb9d77b842`

OCI revision：`95c26a0c7f0c086e6c4956e2ff37c50589eb5e11`

- 构建四端前端、全部 Go 二进制和 source fingerprint：PASS。
- MySQL 5.7 迁移状态：173/173 applied，`/readyz` 显示 migration_current=true。
- `/healthz` 200；`/readyz` 200；Dashboard、SaaS Admin、Sidebar、Operation 四入口均返回有效 HTML。
- json-file 日志轮转配置：`max-size=1m`、`max-file=2`。
- Redis 停止：health=200、ready=503 且仅 redis_connection=false；恢复后 ready=200。
- MySQL 暂停：health=200、ready=503 且 mysql_connection/migration_current=false；恢复后 ready=200。
- `docker stop --timeout 70`：优雅退出码 0；重新启动后 ready=200。
- 本次临时 app、Redis 容器和临时网络已删除；两个本地数据库测试账户已删除；版本化镜像保留；所有既有命名卷未删除。

### 4.6 浏览器

使用内置浏览器访问精确镜像的本地四端：

- Dashboard：重定向 `/login?returnTo=%2F`，显示手机号/密码登录页，控制台 error=0。
- SaaS Admin：重定向 `/saas-admin/?login=1`，显示平台管理员登录页，控制台 error=0。
- Sidebar：未提供真实企业应用 ID 时进入登录页并明确显示“缺少有效的企业应用 ID”，控制台 error=0；这是预期 fail-closed，不是伪登录成功。
- Operation：营销活动入口可打开，控制台 error=0；页面如实显示尚待后续业务迁移的当前合同。

未在浏览器输入真实或持久化凭据，因此认证后的租户业务流仍在边界中。

## 5. FAIL 与未完成边界

### 5.1 历史全量 store/migration MariaDB 集成：FAIL

设置 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 后直接运行历史全量 `go test ./internal/store ./internal/migration` 会失败。根因是多组旧集成 fixture 只构造当时所需的局部表/列，与当前 173 个迁移和 `0130` 受控身份 staging 合同不一致；这不是本次 P0 聚焦实现的运行路径失败，也不能记为 PASS。

本次没有用临时补丁放宽迁移或伪造旧表来骗过全量测试。P0 涉及的风险、关键词、callback、订单、0165 和全生命周期迁移均在隔离数据库单独通过。后续应建立“从真实 migration registry 创建 schema，再加载场景 seed”的统一 integration harness，逐步替换历史手写局部 schema。

## 6. SKIP 证据

- 真实企业微信 Finance/DataZone：SKIP；无真实租户凭据、无生产网络调用、无本地 Finance C SDK 动态库。Linux 编译、race、fake/契约和 fail-closed 注册已 PASS，但不等于 Provider 成功。
- 真实媒体：SKIP；只验证 fixture 媒体、checkpoint、cursor 和失败关闭。
- 真实 AI Provider：SKIP；相关功能关闭且未提供 key。
- 生产服务器、公网部署、真实告警/恢复和首客：SKIP；任务明确禁止。
- 产物签名：SKIP；仓库尚无受控签名身份和密钥基础设施。本次只交付 SBOM、checksum 和 provenance 基础，未伪造签名完成。
- 浏览器认证后真实租户流程：SKIP；不使用真实凭据，Sidebar 正确 fail-closed。

## 7. 提交清单

- `8f1bb914`：中文设计与实施计划。
- `22445e11`：归档运行职责矩阵。
- `b7f1a2e..7491b72`：正式 bridge registrar、凭据与关闭语义。
- `923c639..aae8f62`：callback durable inbox、ACK、幂等、重试与 fence。
- `a54ba1b..289558a`：三端 HTTP 生命周期、readiness、panic 恢复与受控迁移。
- `18fe290..9621ebf`：订单端到端幂等。
- `71fdb36..446f8c1`：风险完整扫描与关键词事务。
- `710f94c..50bf187`：九条门禁与 MySQL 5.7 全迁移生命周期。
- `4dc19ca..1498093`：0165 受控迁移与供应链升级。
- `d6755fe`：补齐 `0131` baseline 后置条件。
- `a4d9114`：修复共享迁移门禁解析与生命周期合同。
- `b1c3ace`：0165 证据账本原子性和内容 hash。
- `95c26a0`：workflow 最小权限与路径覆盖。

最终交付 HEAD 以本报告提交后的新鲜 `git rev-parse HEAD` 为准。

## 8. 合入判断

分支从实施时最新 `main/origin/main` 的 `f2b57f31` 创建，主工作树用户修改未受影响。代码和聚焦生产合同具备合入条件，但在真正合入前仍必须重新 `fetch`，确认 main/origin/main 和全部 worktree 状态，并对任何新增上游提交运行 merge-tree/冲突检查。

由于历史全量 MariaDB integration harness 仍为 FAIL，合入说明必须保留该风险，不能把本分支描述为“所有历史集成测试全绿”；它不阻断本次 P0 聚焦修复，但应作为后续独立根因整改项。
