# MoChat Go 本地可闭环 P0 发布底座设计

## 1. 背景与目标

本设计基于 `main@f2b57f31f2baa93dc871b9157a04b0a8f7e2ae36`、2026-08-29 全量代码审查报告和新鲜门禁复现。目标是在不调用真实企业微信、真实 AI Provider 或生产服务器的前提下，关闭能够用本地 fake、fixture、MariaDB、Redis、Linux/CGO、Docker 和浏览器证明的发布级缺口。

本次必须形成以下可独立验证的合同：

1. runtime role 按职责启动 API、durable/media worker 和 scheduled enqueuer；`worker` 不丢归档 worker，`scheduler` 不混跑归档 worker。
2. `mochat-archive-bridge` 具备正式的 Finance/DataZone driver 注册生命周期；未注册、租户/企业不匹配、初始化或关闭失败均 fail closed。fake 只证明注册合同，不等于真实 Provider 完成。
3. 旧企微 callback 只有在 DB durable inbox 接受后才 ACK；Redis/worker 可暂时不可用，DB 不可用必须让上游重试；重复投递最终只处理一次。
4. 三端监听使用显式 `http.Server`、统一根 context、超时和 graceful shutdown；`/healthz` 只表示进程存活，`/readyz` 短超时检查 MySQL、Redis、迁移版本和当前角色必要能力。
5. 订单、风险规则、风险计数和关键词版本具备租户/企业范围内的幂等、全量执行和事务原子性。
6. 九条红门禁恢复为当前真实合同；迁移 smoke 读取真实注册表，不再硬编码 0098；0165 采用受控备份/校验/恢复方案，不伪造可逆性。
7. 工具链、镜像、Actions、依赖扫描、SBOM 和 provenance 建立可追溯基线；没有签名基础设施时只报告“未签名”，不伪造签名完成。

非目标：不修改 `docs/PROJECT_PROGRESS.zh-CN.md`，不部署公网，不使用真实租户凭据，不删除或复用用户现有 Docker 命名卷，不把 fixture、健康检查或静态门禁称为生产完成。

## 2. 已复现根因

### 2.1 归档角色错误来自开关级裁剪

`internal/config/config.go` 的 `applyRuntimeRole` 在角色不运行 scheduler 时同时关闭 `EnableWorkMessageArchiveSyncCron` 和 `EnableDurableWorkMessageArchive`。但后者同时控制 durable queued worker、媒体 worker和自动 enqueuer。结果是：

- `worker` 角色被错误关闭 durable/media worker；
- `scheduler` 角色保留 durable 开关，主程序据此注册 durable/media worker，反而混跑 worker；
- 已有 `archiveRuntimePlan` 只覆盖 durable/cron 组合，没有覆盖 role 与职责的笛卡尔积。

修复不能再移动单个 if，而应先生成不可变 `RuntimeResponsibilities`，再由配置和主程序共同消费。

### 2.2 bridge 已有 Store 注册表，但生产启动未注册任何 driver

`internal/archivebridge.Store` 已支持按 `(tenant_id, corp_id)` 注册 Finance/DataZone driver，并严格校验 `wx_corpid` 与 `integration_mode`；handler 也已覆盖游标、媒体分片和 component locator。缺口在 `cmd/mochat-archive-bridge/main.go`：`MOCHAT_ARCHIVE_SDK_ENABLED` 只参与日志和 fixture 互斥，没有创建、注册或关闭任何生产 driver。

采用显式 `DriverRegistrar`：启动时从受控 binding source 构建 driver，全部注册成功后才监听；任一 binding 冲突/初始化失败则启动失败；shutdown 先停止 HTTP，再逆序关闭 driver。测试注入 fake registrar，生产 Finance factory 在 Linux 使用正式 SDK adapter；DataZone factory 保留正式接口和 fail-closed 实现，直到真实 DataZone 客户端/凭据具备。fixture manager 继续是单独模式，不能注入 production registrar。

### 2.3 callback ACK 与 durable 接受脱节

`internal/dashboard/wework_callback.go` 在验签、解密和解析后丢弃 `EnqueueWeWorkCallback` 错误并返回 `success`。Redis 去重只有 10 分钟，且 timestamp 没有新鲜度约束。更严重的是 standalone 默认可暴露 callback 路由，但 callback worker 默认可能关闭，形成“ACK 入 Redis、无人消费”。

采用 DB inbox/outbox 合同：

1. handler 校验 timestamp 偏差、签名、解密和企业绑定；
2. 用稳定业务标识计算 `event_key`，在 MySQL 事务中插入规范化事件；唯一键为 `(tenant_id, corp_id, event_key)`；
3. 插入成功或命中同 payload 的既有 receipt 后返回 `success`；DB 错误返回 503；同 key 不同 payload 返回 409/受控失败；
4. worker 以 lease/fencing 从 inbox 领取，处理成功后标记 completed；Redis 仅作为可选唤醒/加速，不再是 ACK 的唯一持久层；
5. replay 窗口由 timestamp 控制，长期幂等由 DB 唯一键控制。

表中只保存规范化事件 JSON，不保存加密 wrapper、签名、nonce 或凭据；日志不记录 RawXML/正文。

现有 Redis Lua 先 `SET NX` 再 `RPUSH`，在队列键类型错误等场景会留下“已有幂等键但事件未入队”的毒化状态。整改后 Redis 只负责在 DB receipt 提交后尽力唤醒 worker；唤醒失败不得改变 receipt，恢复后的扫描器仍能从 DB 找到待处理事件。

### 2.4 HTTP、readiness 与部署顺序没有统一生命周期

主程序对三个 listener 使用裸 `http.Serve`；后台 group 和 HTTP 使用不同 `context.Background()`；关闭信号不能先摘流再等任务退出。`/readyz` 只检查前端兼容状态。若直接把迁移状态加入 readiness，现有 Docker Desktop 脚本“先等 app healthy，再迁移”会死锁。

采用 `runtimegroup.ServiceGroup`：统一根 context，显式 server timeout，监听成功后注册，SIGTERM 后 readiness 立即转为 draining、停止领取新任务、并发 `Shutdown`，最后有界等待任务。部署改为 `migrate` 一次性服务成功后 app 才启动；不再由 app healthy 反向触发迁移。

readiness 每项最多 500ms、总预算 2s：

- MySQL `PingContext`；
- Redis `PING`；
- migration ledger 的最新版本/checksum 与代码发现器一致；
- 角色必要能力：worker 必须存在相应 task snapshot/注册项，scheduler 必须存在 enqueuer，API 不要求后台 worker；
- 前端 manifest 仅在对应静态服务启用时检查。

### 2.5 数据一致性缺口来自错误边界

- 订单：客户端重试生成新 UUID；服务端只保证 ID 唯一，没有用户意图键。采用 `Idempotency-Key` + payload SHA-256 + `(tenant_id, corp_id, key)` 唯一 receipt；同 payload 重放首次响应，不同 payload 409。
- 风险规则：执行路径复用 UI 的最多 100 条分页。新增执行专用游标接口，不设置静默总量上限。
- 风险记录：逐条 insert 后忽略 `trigger_count` 更新错误。事件、明细和计数放入一个事务。
- 关键词：entry 保存与 library version 更新分两次执行且忽略错误。锁定 library 后在同一事务写 entry、递增版本并检查 `RowsAffected`。

新增迁移均提供 up/down，使用 MySQL 5.7 支持的索引长度、`VARCHAR`、`JSON`/`LONGTEXT` 和显式唯一键；不依赖 MySQL 8 专有 DDL。

## 3. 运行角色与组件矩阵

| role | HTTP | ordinary workers | durable/media workers | scheduled enqueuers |
| --- | ---: | ---: | ---: | ---: |
| `all` | 是 | 是 | durable 开关决定 | cron 开关决定 |
| `api` | 是 | 否 | 否 | 否 |
| `worker` | 否 | 是 | durable 开关决定 | 否 |
| `scheduler` | 否 | 否 | 否 | cron 开关决定 |

`EnableDurableWorkMessageArchive` 只表达 durable 能力；`EnableWorkMessageArchiveSyncCron` 只表达自动计划。`all` 中两者可同时存在，但注册为不同 task；其他角色由职责矩阵裁剪。API 角色仍可按 durable 能力暴露人工同步和 component locator，但绝不因此注册 worker；worker 角色只消费 durable/media 队列；scheduler 角色只负责 scheduled enqueue。

## 4. 迁移设计

计划新增：

- `0172_wework_callback_inbox`：durable inbox、唯一 event key、payload fingerprint、状态/lease/fencing/attempt/audit timestamps。
- `0173_scrm_order_idempotency`：订单 idempotency key、payload fingerprint、首次响应或必要重放字段及租户/企业唯一约束。

每个 down 只删除本迁移创建的索引/列/表，不触碰既有业务数据。并发测试使用真实 MariaDB/MySQL 连接验证唯一冲突和首次响应重放。

0165 已经存在于历史链，不能修改 checksum 或声称 down 能恢复被删除数据。整改为：

1. 新增只读 preflight，输出重复行/将删除行/将 drop 字段计数；
2. 要求备份表和 checksum，经人工批准才允许 contract；
3. 新环境在 0165 前创建备份快照，迁移后校验计数；
4. 已执行环境通过备份恢复脚本恢复，而不是依赖 down 伪造数据；
5. 门禁检查 preflight、备份、验证和恢复文档均存在并可执行。

## 5. 门禁治理

九条红灯逐类处理：

- `phase34-lint`：把 299 个文件拆成固定大小批次调用 ESLint，规避 Windows 32K 命令长度，不减少文件集合。
- `phase3-5-dashboard` / `phase3-final`：统一消费同一 manifest 状态，`/chat/file-audio` 只定义一次，避免一处允许、一处拒绝。
- `dashboard-auth-context`：模块 handler 从代码注册表导出 route→handler→auth metadata；public activation status 显式标注，不靠 `parseBearer` 字符串猜测。
- `identity-single-corp`：把激活展示查询移到 identity repository/projection；不扩大 `mc_user` 直接读取例外。
- `wecom-archive-saas-activation`：检查新的 runtime responsibilities 和生产 registrar 合同，移除已废弃的 mutually-exclusive 文案要求。
- `docs:check`：修复真实相对链接。
- `phase2-progress`：由当前 manifest 生成报告；生成器输出稳定排序，check 只比较生成结果。
- `phase2.1`：将已删除 `/corp/index` 合同迁移到当前唯一企业资料路由/文件，不创建空兼容页。

CI 中 build 必须先于依赖 dist 的 audit。MySQL 5.7 smoke 从 Go migration discovery 输出版本列表和期望计数，不再复制 0001–0098 的 grep 清单。

## 6. 供应链决策

Go 1.26.6 是审查报告提出的安全下限；官方在 2026-08-19 已发布 1.26.7，并额外包含 `net/http` 修复，因此本分支采用 1.26.7。`go.mod` 固定 patch；Docker builder、Node 和 runtime 镜像在实施时从官方 registry manifest 解析并提交精确 tag+digest，禁止手写或猜测 digest。

GitHub Actions 从浮动 `@vN` 改为当前官方 tag 对应 commit SHA，并在注释保留版本。CI 增加：`govulncheck`、pnpm audit policy、CycloneDX/SPDX SBOM 和 GitHub artifact provenance 基础。签名需要外部 keyless/OIDC 或私钥治理，本地不伪造签名。

pnpm audit 按调用可达性分类：生产 bundle/runtime 可达项必须升级或阻断；仅构建/测试工具链项先尝试兼容升级，无法升级时记录 advisory、依赖路径、不可达理由、到期日和责任人。不能只用 `--ignore` 消音。

## 7. 验证与证据分层

1. RED：每个行为先运行最小失败测试，记录预期失败原因。
2. GREEN：目标测试、相关包测试、全量 Go/前端/门禁。
3. 数据库：独立 Compose project 启动 MariaDB/Redis，运行 0001–最新迁移、up/down、并发和故障注入；不使用现有四个命名卷。
4. Linux/CGO：在隔离容器执行 `go test -race` 和 Finance SDK ABI/生命周期合同；若官方 SDK 文件缺失，明确 SKIP 到具体缺失路径/动态库，而不是 PASS。
5. Docker：以精确 Git SHA 构建 tag，记录 image ID/digest、SBOM、source fingerprint；故障切断独立 DB/Redis 后验证 health/ready 和恢复。
6. 浏览器：只对本地受控 seed/API 数据执行 Dashboard、SaaS、Sidebar、Operation 核心流程；静态占位路由不算业务通过。
7. 外部边界：Finance/DataZone fake、fixture、MariaDB、Redis、Docker、浏览器均是本地证据；真实企微拉取、真实 DataZone、真实 AI、生产部署保持 SKIP/NOT RUN。

## 8. 设计自审

- [x] 没有修改进度台账，也没有把 fixture 注册写成生产完成。
- [x] 每个 High/Medium 根因都有一个源头级合同，不靠日志过滤、空文件或旧逻辑回退。
- [x] 明确了角色矩阵、ACK 边界、事务边界、迁移 up/down、MySQL 5.7 和并发要求。
- [x] readiness 与迁移启动顺序一起设计，避免部署死锁。
- [x] 供应链使用当前官方 patch，SBOM/provenance 与签名边界分开。
- [x] 本地、Linux/CGO、Docker、浏览器、真实 Provider 和生产证据分层陈述。
