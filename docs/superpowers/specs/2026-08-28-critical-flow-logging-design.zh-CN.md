# 系统关键流程日志完善设计

## 1. 目标与范围

本专项建立一套“开发人员先看日志即可确定故障处于哪一阶段、下一步该查什么”的关键日志体系。目标不是把每个函数、循环或数据行写入日志，而是统一等级、字段、脱敏、关键边界和部署保留策略，并修复会导致日志或任务执行账本无界增长的直接根因。

本设计只覆盖仓库中已有且能由代码证明的流程。新增日志集中在进程启动边界、HTTP 关键结果边界、外部 Provider 调用边界、回调验签边界、异步任务生命周期和少数高风险定时同步边界；业务审计账本仍承担“谁在何时改了什么”的合规职责，不用应用日志重复保存前后快照。

## 2. 代码审计结论

### 2.1 现有日志基础设施

- Go 服务主要使用标准库 `log` 和 `*log.Logger`，仓库中没有统一等级、格式、上下文字段或脱敏 Handler；审计时 `cmd/` 与 `internal/` 中相关调用约 731 处。
- `cmd/mochat-go/main.go` 在 INFO 级逐条输出约 565 条路由启用信息。它们只重复代码注册事实，启动时形成大段噪声，真正的配置错误和依赖失败容易被淹没。
- `internal/taskrunner/taskrunner.go` 已有 `task_name`、`run_id`、`execution_id`、`kind`、`status` 等有价值的生命周期数据，但文本日志没有统一等级，成功 periodic tick 也会进入持久执行历史。
- `internal/taskrunner/sql_recorder.go` 对每个 periodic tick 使用唯一 `execution_id` 写入 `mochat_go_background_task_executions`，也为每次进程启动写 `mochat_go_background_task_runs`；当前没有保留期或清理入口，长期运行会无界增长。
- 多个 cron 在扫描结果全为零时仍输出“finished” INFO。`internal/dashboard/work_message_archive_sync_cron.go` 每轮都打印汇总；现有部署证据 `docs/deployment/2026-08-26-wecom-archive-live-deployment-acceptance.zh-CN.md` 明确记录过每 15 秒持续出现 `corps=0/fetched=0` 日志。这是已经发生的噪声根因。
- `WorkMessageArchiveBridgeClient` 在 HTTP 非 2xx 时把外部响应正文拼入 error；解析缺少 `msgid` 时把原始消息 JSON 拼入 error。上层一旦记录 error，会泄露完整消息正文或外部敏感内容。

### 2.2 输出、归档和部署

- `Dockerfile` 没有应用内文件日志，主进程直接写容器 stdout/stderr；这是正确方向，容器采集不应再叠加应用内文件轮转。
- `deploy/standalone/docker-compose.yml` 当前没有为服务设置 Docker logging driver 的 `max-size`、`max-file`，Docker 默认 `json-file` 在长期运行时可能持续占用磁盘。
- `/healthz` 和 `/readyz` 已由 `internal/server/server.go` 实现，Compose 使用 `/readyz` 作为健康检查。成功健康轮询不应写应用日志；失败就绪结果应记录一次可检索的 ERROR/WARN 结果，供部署排障。

### 2.3 已由代码确认的主要流程

| 流程 | 代码证据 | 是否纳入 | 关键日志边界 |
| --- | --- | --- | --- |
| 启动、配置、依赖连接 | `cmd/mochat-go/main.go`、`internal/config/config.go` | 是 | 日志配置、运行角色、关键配置缺失、MySQL/Redis/Server 构建失败、监听就绪 |
| 数据库迁移 | `cmd/mochat-migrate/main.go`、`internal/migration/migration.go` | 是 | action 开始、完成、失败、耗时和迁移计数；不输出 DSN |
| SaaS 与 Dashboard 认证 | `internal/saasauth`、`internal/dashboardauth` | 是 | 登录/激活等关键 POST 的结果；401/403 统一 WARN |
| Dashboard 权限 | `internal/dashboard/dashboard_access_guard.go`、`internal/dashboard/saas_admin_access.go` | 是 | 权限拒绝的请求结果、请求 ID、路径和状态；不记录令牌或请求体 |
| SaaS 租户开通、激活、状态变更 | `internal/dashboard/saas_admin.go`、`internal/store/mysql.go` 中 tenant provision/activation 实现与迁移 `0086` | 是 | 写请求最终结果；业务审计继续记录 actor/action/target |
| 企微自建与第三方模式 | `internal/companyprofile`、`internal/wecomsuitecallback`、`internal/store/wecom_suite_callback.go` | 是 | 凭据配置写结果、第三方回调验签、授权交换与持久化结果 |
| 通讯录、客户、客户群同步 | `work_employee_sync*`、`work_contact_sync*`、`work_room_sync*` | 是 | 手动触发结果、异步重试/死信、外部调用失败；不逐部门、逐客户、逐群输出 |
| 会话存档正文与媒体 | `work_message_archive_sync_cron.go`、`internal/modules/providers/archive/sync.go`、`media_sync.go` | 是 | 有数据/失败汇总、媒体批次失败、游标和错误码；绝不记录正文、SDK 文件定位符、私钥 |
| 主动与定时同步 | 公司档案手动接口、各 `*_cron.go` | 是 | 手动写请求结果；periodic 空结果静默，失败保留 |
| 异步任务与重试 | `internal/taskrunner`、Redis worker 文件 | 是 | worker 启停、queue item 失败/重试/放弃、recorder 失败；成功空 tick 不写 INFO |
| Dashboard 关键写操作 | `internal/server/server.go` 注册的大量 POST/PUT/DELETE 路由 | 是 | HTTP 统一结果边界；不为每个 handler 重复增加一套日志 |
| AI Provider 调用 | `internal/modules/providers/ai/openai/openai.go` | 是 | provider/model、结果、状态码、耗时、稳定错误码；不记录 prompt/response/API Key/Base URL 路径 |
| 回调验签 | `internal/wecomsuitecallback/handler.go` 及 SaaS 支付/域名回调路由 | 是 | 验签拒绝 WARN、外部交换失败 WARN/ERROR、成功 INFO/DEBUG；不记录签名、nonce、密文、AuthCode |
| 健康与就绪 | `/healthz`、`/readyz` 与 Compose healthcheck | 是 | 成功轮询静默，失败才记录 |

## 3. 方案比较与决策

### 方案 A：一次性把全部 731 个调用点迁移为结构化日志

优点是形式最统一。缺点是变更面极大，容易改变低价值旧流程的行为，且大量时间会花在机械迁移而不是关键故障链路；同时无法自动判断每条旧日志的真实业务等级。本专项不采用。

### 方案 B：统一底座 + 关键边界结构化 + 兼容迁移（采用）

新增基于标准库 `log/slog` 的 `internal/observability`：统一格式、等级、脱敏和 HTTP 结果日志；标准 `log` 通过兼容 writer 进入同一输出，旧调用先保持行为，新建和本专项修改的调用必须显式选择等级。优先迁移启动、HTTP、任务、AI、回调、会话存档等关键边界，并移除已证明的空轮询与路由噪声。

优点是能快速形成稳定的排障主线，同时把风险限制在少数公共边界；以后可按模块逐步迁移。缺点是过渡期仍存在未显式分级的 legacy INFO，本设计把它列为已知边界，并用静态门禁禁止新增高风险模式。

### 方案 C：只依赖 Docker/采集平台解析旧文本

优点是应用改动少；缺点是平台无法可靠恢复租户、任务、步骤和错误码，也不能阻止应用先把正文或 Secret 输出。因此只作为采集与轮转层，不作为应用日志方案。

## 4. 日志等级边界

| 等级 | 使用条件 | 典型事件 | 明确禁止 |
| --- | --- | --- | --- |
| DEBUG | 仅开发排查需要的细节；关闭后不影响确认业务是否成功 | 路由注册明细、重复回调命中、periodic 空结果、外部调用开始 | 生产必需状态、正文/凭据、逐条循环数据 |
| INFO | 低频、可操作的关键阶段或成功状态转换 | 服务开始监听、迁移完成、租户开通成功、关键写请求成功、AI 调用成功、有数据的同步汇总 | 普通 GET、健康成功轮询、空队列、空扫描、每条数据成功 |
| WARN | 请求被拒绝、可恢复/预期失败、配置降级、将重试 | 401/403、Provider 4xx/超时、验签失败、队列重试、功能因配置缺失跳过 | 系统不可继续的错误、普通业务“未找到”一律当 ERROR |
| ERROR | 当前阶段无法完成、数据一致性异常、持久化失败、任务放弃或进程不能继续 | MySQL/Redis/迁移失败、任务 panic、死信、游标写入失败、Provider 响应不可解析、就绪失败 | 用户校验失败、正常空结果、可接受幂等重复 |

同一失败不在多层重复输出：底层返回带稳定错误码的 error，最接近“业务阶段结束”的边界只记录一次；HTTP 中间件只记录状态与路由，不记录响应正文。

## 5. 统一字段与事件文本

字段只在有真实值时出现：

- `event`：稳定、可搜索的 snake_case 事件名。
- `component`：`runtime`、`http`、`migration`、`taskrunner`、`wecom_sync`、`archive`、`ai_provider`、`callback` 等。
- `tenant_id`、`corp_id`：只使用服务端已经解析/持久化的整数 ID；不从不可信 query/body 猜测。
- `request_id`：沿用合法 `X-Request-ID`，否则生成 UUID，并写回响应头。
- `listener`、`listen_addr`：监听成功后记录 `main`、`sidebar`、`operation` 及实际 bind 地址；bind 失败只记录启动 ERROR，不得预报 ready。
- `task_name`、`run_id`、`execution_id`：来自现有 taskrunner runtime context。
- `object_type`、`object_id`：只用于租户、企业、任务、媒体等非敏感业务对象；不使用手机号、外部联系人 ID、完整微信 ID 作为默认对象 ID。
- `step`：`validate`、`connect_dependency`、`authorize`、`fetch`、`persist`、`retry`、`dead_letter`、`listen` 等。
- `result`：使用当前边界可直接理解的受控值，例如 `started`、`success`、`rejected`、`failed`、`retrying`、`stopped`、`skipped`；同一事件保持稳定，不写自由文本。
- `error_code`：稳定机器码或受控 HTTP 状态码，不用完整第三方响应替代。
- `duration_ms`：边界耗时，整数毫秒。
- 计数：`scanned`、`fetched`、`inserted`、`updated`、`skipped`、`failed`，只写批次汇总。

日志文本使用“业务动作 + 结果”，例如“AI Provider 调用失败”“企微第三方授权已保存”“后台任务本轮执行失败”，避免只有 `err=...`。每个新增事件在下表说明用途和后续动作。

## 6. 新增关键事件目录

| event | 等级 | 为什么需要 / 出现意味着什么 | 开发人员下一步检查 |
| --- | --- | --- | --- |
| `runtime_starting` | INFO | 进程已完成基础日志初始化，开始加载业务配置 | 核对 `runtime_role`、版本和后续依赖事件 |
| `runtime_config_invalid` | ERROR | 配置解析失败，进程不能启动 | 检查对应环境变量是否缺失或格式错误；不要在工单粘贴 Secret 值 |
| `runtime_listening` | INFO | 对应 `listener` 已成功 bind 并开始监听 | 若外部不可用，按 `listener/listen_addr` 检查端口映射、反向代理和 `/readyz` |
| `migration_started/completed/failed` | INFO/ERROR | 一次迁移动作开始、完成或中止 | 用 `action`、耗时和错误码核对 ledger、checksum、数据库权限 |
| `critical_request_completed` | INFO | 一个关键写请求成功 | 用 `request_id` 关联前端/网关，按 `component/path` 检查业务审计 |
| `request_rejected` | WARN | 认证或权限拒绝（401/403） | 检查 realm/session、用户状态、菜单/权限和租户范围，不检查密码正文 |
| `critical_request_failed` | ERROR | 关键请求返回 5xx | 用 `request_id` 查同时间的依赖、任务或持久化错误 |
| `critical_request_panicked` | ERROR | HTTP handler 发生未处理 panic，中间件已返回受控 500 | 用 `request_id/path` 检查对应 handler；日志不会写 panic 原值，避免敏感内容泄露 |
| `readiness_failed` | ERROR | `/readyz` 返回非成功，容器可能被判为不就绪 | 检查 manifest、上游、数据库/依赖和启动错误 |
| `background_task_started/stopped/failed` | INFO/ERROR | 长驻后台任务生命周期变化 | 检查 `task_name/run_id`、关闭信号或 panic/error |
| `periodic_task_failed` | ERROR | 某个定时 tick 失败；同一任务连续失败只在首次及每分钟一次输出，成功空 tick 不写 INFO | 检查 `task_name/execution_id/error_code` 和对应外部依赖；恢复后会看到 INFO 恢复事件 |
| `task_history_cleanup_failed` | WARN | 历史保留清理失败但主任务仍运行 | 检查数据库权限、锁等待和表大小 |
| `archive_sync_completed` | INFO | 会话存档本轮确实处理了企业或消息 | 检查 fetched/inserted/skipped/failed 与 cursor |
| `archive_sync_failed` | ERROR | 拉取、落库或游标推进失败 | 按 `corp_id/step/error_code` 检查 bridge、凭据和数据库；日志不含正文 |
| `archive_sync_skipped` | DEBUG | 主动事件命中了未启用存档的企业，属于可接受空态 | 若业务期望已启用，检查企业绑定和存档凭据状态 |
| `ai_provider_call_completed` | INFO | 外部模型调用成功且已拿到有效内容 | 用 provider/model/duration 评估延迟；正文需到受控业务数据查看 |
| `ai_provider_call_failed` | WARN/ERROR | Provider 未配置、网络失败、HTTP 拒绝或响应无效 | 检查租户 Provider 配置、出站网络、状态码和模型；不要打印 Key 或响应正文 |
| `wecom_callback_rejected` | WARN | 回调时间窗、验签、解密或事件格式不合法 | 检查平台回调配置、时钟、suite ID 和网络重放；不要记录签名/密文 |
| `wecom_authorization_exchange_failed` | WARN | 第三方授权码交换失败 | 检查 suite ticket 新鲜度、开放平台配置和 bridge 可用性 |
| `wecom_authorization_saved` | INFO | 第三方授权已成功换取并持久化 | 用 tenant/corp 核对激活状态与后续同步任务 |

## 7. 脱敏策略

1. 结构化字段名包含 `password`、`passwd`、`token`、`secret`、`cookie`、`authorization`、`api_key`、`private_key`、`credential` 时，字符串、字节和对象值统一替换为 `[REDACTED]`；布尔型 `*_configured` 可保留配置状态。
2. 文本消息和 error 再经过模式脱敏，覆盖 `Bearer`、`key=value`、URL userinfo、JWT 形态和 PEM 私钥块。
3. HTTP 中间件禁止读取或记录 request/response body、query、Cookie、Authorization；只记录 method、path、status、request_id、耗时和受控分类。
4. AI 日志禁止 prompt、system、模型响应、API Key、Authorization 和完整 Base URL。
5. 会话存档日志禁止完整消息正文、`RawJSON`、Chat Secret、RSA 私钥、公钥正文、`sdkfileid`、bridge Bearer；外部响应 body 不再进入 error。
6. 回调日志禁止 nonce、signature、echostr、Encrypt、Ticket、AuthCode、PermanentCode 和回调正文。

## 8. 噪声和增长治理

### 8.1 stdout 日志

- 路由、resolver、能力开关和任务配置明细降为 DEBUG；INFO 只保留进程启动、实际后台任务生命周期和监听就绪。
- 成功的 GET/HEAD/OPTIONS、`/healthz`、`/readyz` 静默；健康失败才输出。
- periodic 空结果不输出 INFO。会话存档只有处理到消息、出现失败或明确主动同步有结果时才输出汇总。
- 不在循环内记录逐条成功；失败只记录批次首个/汇总错误，已有 retry/dead-letter 生命周期保留。

### 8.2 后台任务数据库历史

- periodic 成功和连续失败分别改为每个 `task_name` 一条“最新成功/最新失败”稳定记录，后续同类结果覆盖它；失败日志首轮立即输出，连续失败最多每分钟输出一次，恢复时输出一次 INFO。手工 `RunCorp` 是用户触发的独立运行，失败每次记录，且不得消费、恢复或覆盖 periodic 的限流状态。这样既保留当前故障证据，也阻止短周期依赖故障同时放大 stdout 与执行账本。
- `mochat_go_background_task_runs` 和 `mochat_go_background_task_executions` 默认保留 14 天；每小时异步清理，每张表单批最多删除 10,000 条过期数据，避免阻塞当前执行记录和形成长事务。代码构造参数可覆盖保留期、间隔和批量；standalone 当前使用固定默认值，没有暴露环境变量。
- 当前状态表 `mochat_go_background_tasks` 不清理；它每任务固定一行。
- 清理使用独立 30 秒 context，失败只记 WARN，不阻塞主任务；下一小时重试。

### 8.3 迁移前任务误启动根因

Docker 实测发现全新数据库尚未应用 `0144` 会话导出任务表迁移时，standalone 默认每 2 秒启动 conversation export worker，持续产生“表不存在”错误和执行历史。这不是日志等级问题，不能靠降级或隐藏解决。Compose 和 `.env.example` 因此把 `MOCHAT_GO_ENABLE_CONVERSATION_EXPORT_WORKER` 默认值改为 `0`；部署人员必须先完成迁移并确认导出任务表存在，再显式设为 `1`。日志策略门禁会阻止该 worker 再次默认开启。

## 9. Docker、服务器采集和磁盘保护

- 应用只写容器 stdout，不新增应用日志目录或应用内轮转；这样不会与 Docker/服务器采集重复轮转。
- `deploy/standalone/docker-compose.yml` 使用 Docker `json-file` driver，默认 `max-size=20m`、`max-file=5`，即单容器约 100 MiB 上限；app、bridge、simulator、MySQL、Redis 使用同一策略，可由环境变量覆盖。
- 生产服务器使用 `docker logs`、journald 或现有采集 Agent 读取 stdout/stderr；平台只保留一层轮转。如果生产改用 `local`/journald driver，应删除 Compose 中与平台重复的轮转配置，而不是让应用写文件。
- 默认生产格式为 JSON、级别 INFO；本地可用 text，临时排障才切 DEBUG，排障后恢复 INFO。

## 10. 验证设计

### 10.1 自动化测试

- observability：等级解析、JSON 字段、敏感 key/text/error 脱敏、legacy writer 分级。
- HTTP：成功关键写、401/403、5xx、handler panic 受控恢复、健康成功静默、健康失败、request ID 生成/沿用、正文和凭据不进入日志。
- taskrunner：成功空 tick 不写 INFO、失败写 ERROR、periodic 最新成功/最新失败均使用稳定 execution ID、连续失败限流且恢复只记一次。
- SQL recorder：保留清理 SQL、每小时节流、独立 context 异步执行、清理失败不阻断记录。
- archive：空轮询无日志、有数据有汇总、失败有错误码；手工失败不共享 periodic 限流/恢复状态；bridge 非 2xx 和缺失 msgid 不泄露响应/消息正文。
- AI：成功、未配置、HTTP 失败、无内容等场景日志包含 provider/model/result/duration，不含 Key、prompt、response。
- callback：验签失败、重复、授权交换失败、保存失败、成功；业务对象统一为 `object_type/object_id`，且不含 Ticket/AuthCode/PermanentCode/密文。
- migration：成功/失败生命周期不输出 DSN。

### 10.2 运行验证

1. 运行 Go 全量测试、`go vet`、前端 test/typecheck/lint/build。
2. 运行日志策略静态扫描：禁止 INFO 路由逐条日志、禁止新增敏感字段、禁止 archive 外部正文进入 error。
3. 构建 standalone Docker 镜像与 Compose config，启动最小本地环境，实际读取 `docker logs`。
4. 实际触发：健康成功、配置缺失启动失败、关键写成功、权限拒绝、Provider 失败、异步失败/重试、archive 空轮询；逐项检查等级、字段、可搜索性和脱敏。
5. 检查 Docker inspect 的 logging driver/options，确认没有应用日志卷和重复轮转。

## 11. 已知边界

- 本专项不一次性重写全部 legacy `log.Printf`；未触及旧调用经兼容 writer 进入统一 JSON，默认视为 INFO。后续模块迁移时必须显式选择等级。
- HTTP 统一边界不从请求 body/query 猜 tenant/corp，避免信任不受控输入；只有业务层已经确认的 ID 才进入专用事件。
- 真实企业微信、真实 AI Provider 和真实生产采集平台需要外部凭据/环境，本地验证使用受控 fake Provider/回调/bridge；不得为“真实验证”把凭据写入命令、日志或仓库。
- 业务审计表的合规保留政策不由本专项更改；本专项只治理应用 stdout 和后台任务运行历史，避免误删具有法规意义的操作审计。

## 12. 设计文档自审

- [x] 定义 DEBUG/INFO/WARN/ERROR 边界、示例和禁止项，没有把普通业务状态统一提升为错误。
- [x] 第 6 节逐条说明新增事件的必要性、出现含义和下一检查方向，文本具备业务含义和稳定搜索键。
- [x] 只覆盖关键阶段、状态转换、外部依赖结果、任务生命周期、重试/恢复、权限拒绝、配置缺失与一致性异常，并明确禁止循环、逐行与空轮询噪声。
- [x] 字段来自当前 HTTP、taskrunner、租户/企业和业务对象架构；第 7 节列出敏感字段、正文、回调、Provider 与存档禁止项。
- [x] 第 9 节覆盖 stdout/stderr、Docker driver、大小、文件数、平台采集、单层轮转和磁盘保护。
- [x] 第 2.3 节先列代码证据，再决定启动/迁移、认证权限、SaaS、企微、自建/第三方、同步、存档、任务、Dashboard、AI、回调和健康流程是否纳入。
- [x] 第 8 节分析空轮询、稳定执行记录、连续失败和迁移前 worker 根因；修复不靠降级隐藏，测试与 Docker 验证设计覆盖回归。
