# 系统关键流程日志完善验证与交付记录

## 1. 结论

本专项已建立基于 `log/slog` 的统一日志底座，完成等级、字段、脱敏、HTTP 关键结果、异步任务生命周期、会话存档、AI Provider、企微第三方回调、迁移和 Docker 轮转治理。实现没有把所有代码细节改成日志：生产 INFO 只保留关键阶段、状态转换、外部调用结果和失败；路由、resolver、能力开关、正常空轮询等明细均在 DEBUG 或完全静默。

真实 Docker 验证发现并修复了一个功能性根因：全新数据库尚未应用 `0144` 时，conversation export worker 原默认每 2 秒运行，持续产生“表不存在”错误、stdout 和任务执行历史。最终将 worker 默认值改为关闭，要求迁移成功后显式启用，同时把连续 periodic 失败压缩为稳定记录并限流。最终复审又消除了 archive 内外双重失败日志、侧端口绕过 HTTP 中间件和历史清理阻塞写路径三个放大点。

## 2. 变更构成

- `internal/observability`：JSON/text Handler、等级配置、敏感字段和文本脱敏、标准 `log` 兼容桥、HTTP request ID、关键结果与 panic 受控恢复中间件。
- `cmd/mochat-go`：启动/监听/失败结构化事件；成功 bind 后才记录 listening；路由、认证 resolver、能力/worker/cron 启用明细降为 DEBUG；主、sidebar、operation 三个监听入口共用 HTTP 中间件。
- `cmd/mochat-migrate`：迁移开始、完成、失败、action、计数与耗时，不输出 DSN。
- `internal/taskrunner`：长驻任务生命周期、periodic 空成功静默、成功/失败稳定执行记录、连续失败限流、恢复事件、14 天历史异步分批清理。
- 会话存档、AI Provider、企微第三方回调：在真实业务边界记录受控结果，移除外部响应正文、原始消息和回调敏感内容。
- standalone Compose：stdout JSON、统一 `json-file` 轮转、每容器默认约 100 MiB 上限；迁移前不默认启动 conversation export worker。
- `scripts/check_logging_policy.mjs`：阻止 INFO 启动明细、正文泄露、敏感字段、缺失轮转、日志目录挂载和 worker 默认误开启回归。

## 3. 等级、字段与排查约定

| 等级 | 边界 | 例子 | 开发人员下一步 |
| --- | --- | --- | --- |
| DEBUG | 仅临时开发细节；关闭后不影响判断成败 | 路由/resolver 注册、能力开关、可接受空态 | 临时开启后按 `component/event` 缩小范围，排查完恢复 INFO |
| INFO | 低频关键阶段、成功状态转换、有数据同步 | `runtime_listening`、迁移完成、关键写成功、任务启动、AI 成功 | 按 `request_id/run_id/object_id` 关联业务审计或下游记录 |
| WARN | 请求拒绝、可恢复失败、即将重试或配置降级 | 401/403、Provider 4xx/网络失败、回调验签拒绝 | 检查登录态、租户/权限、配置和外部依赖；不要查日志中的凭据值 |
| ERROR | 当前阶段无法完成、持久化/一致性失败、任务放弃、进程不能启动 | 5xx、迁移失败、periodic 失败、配置/数据库失败 | 按 `error_code`、`step`、`request_id/run_id` 定位对应边界 |

统一字段按现有架构取值：`event`、`component`、`tenant_id`、`corp_id`、`request_id`、`task_name`、`run_id`、`execution_id`、`object_type`、`object_id`、`step`、`result`、`error_code`、`status_code`、`duration_ms` 和批次计数。字段没有真实可信值时不输出；HTTP 层不从 query/body 猜租户或企业。

新增事件的“为什么需要、出现意味着什么、下一步检查什么”逐项定义在中文设计文档第 6 节。日志文本均使用“业务动作 + 结果 + 下一检查方向”，机器检索使用稳定 `event/error_code`。

## 4. 主要流程覆盖矩阵

| 主要流程 | 代码证据与覆盖边界 | 验证 |
| --- | --- | --- |
| 启动、配置、迁移 | `cmd/mochat-go/logging.go`、`cmd/mochat-migrate/main.go`；启动/监听/失败和迁移生命周期；主、sidebar、operation 均在成功 bind 后记录监听 | 单元测试；Docker 启动成功、secret 路径错误、数据库认证错误、非法日志等级 |
| 认证与权限 | HTTP 401/403 统一 WARN，稳定 `HTTP_401/403`，不读取 header/body/query | HTTP 测试；Docker 未授权写请求得到 401 和 `request_rejected` |
| SaaS 租户开通、激活 | 已存在的 provision/activate 写路由由 HTTP 关键写边界覆盖，详细变更继续进入业务审计 | HTTP 成功/拒绝/失败测试；SaaS 前端全量测试 |
| 企微自建/第三方对接 | 自建配置写请求由 HTTP 边界覆盖；第三方 callback 的验签、ticket、授权交换、保存显式记录，业务对象统一使用 `object_type/object_id` | `internal/wecomsuitecallback` 测试 |
| 通讯录、客户、客户群同步 | 手动触发由 `wecom_sync` HTTP 分类覆盖；异步执行由 taskrunner 生命周期覆盖 | Docker employee-sync 401；taskrunner 测试 |
| 会话存档正文与媒体 | archive cron 只记录非空/失败汇总；连续失败由 archive 单边界限流并在恢复时记录一次，通用 Periodic 不重复输出；手工 `RunCorp` 失败每次独立记录且不改变 periodic 状态；已启用企业缺少凭据时以受控错误和 `corp_id` 报告；bridge 错误不含响应正文/原始消息 | archive 与 taskrunner 测试；Docker 空轮询超过 3 周期稳定 |
| 主动/定时同步 | 手动写结果由 HTTP 边界覆盖；periodic 成功空 tick 静默、失败限流、恢复一次，带 `execution_id` | taskrunner 与 archive 测试；Docker 稳定记录查询 |
| 异步任务与重试 | 长驻任务启停、panic/失败、periodic 失败/恢复；SQL 历史 14 天、每小时异步清理每张表最多 10,000 条 | taskrunner、延迟 SQL recorder 非阻塞回归测试 |
| Dashboard 关键写操作 | POST/PUT/PATCH/DELETE 统一记录最终结果；普通 GET 和健康成功静默；handler panic 返回受控 500 | HTTP/panic 测试；三个 Docker 监听入口分别实测 401 日志；前端 147 文件/966 Dashboard 测试 |
| AI Provider 调用 | 成功、未配置、网络/4xx/5xx、解析/空内容按边界分级；HTTP 链路带 `request_id`；不含 prompt/response/key/base URL | OpenAI Provider + HTTP middleware 集成测试 |
| 回调验签 | 非法时间窗/验签/解密、重复、交换/保存、成功事件；不含签名、nonce、密文、授权码 | callback 测试 |
| 健康/就绪 | 成功 `/healthz`、`/readyz` 静默；5xx 才记录存活/就绪失败 | HTTP 503 测试；Docker 健康成功与静默实测 |

## 5. 脱敏与噪声验证

### 5.1 自动化脱敏

- Handler 测试覆盖敏感字段名、Bearer、`key=value`、URL userinfo、JWT 和 PEM。
- HTTP 测试携带 Authorization、Cookie、query token 和非法 request ID，日志均不包含原值。
- taskrunner 测试把敏感错误文本送入失败路径，最终日志只保留脱敏文本。
- archive 测试验证 HTTP 非 2xx 不带响应 body，缺失 msgid 不带原始消息 JSON；已启用企业配置不完整会失败并指出 `corp_id`，但不输出企业号、公私钥或 Secret。
- AI 测试验证 API Key、prompt、模型响应、Base URL 不进入日志。
- callback 测试验证 ticket、auth/permanent code、签名、nonce、加密 XML 不进入日志，并断言使用 `object_type/object_id` 而不是非标准 `business_object`。
- `pnpm check:logging` 同时扫描禁止字段和高风险代码模式，结果 PASS。

### 5.2 噪声与增长

- 约 565 条路由注册、认证 resolver、能力开关和 cron/worker 配置明细降为 DEBUG。
- Docker INFO 启动实际共 5 行：`runtime_starting` 1 行、带 `listener=sidebar|operation|main` 的 `runtime_listening` 3 行、实际后台任务启动 1 行。
- 连续执行健康/就绪请求并等待 7 秒（archive 间隔 2 秒，超过 3 个周期）：`LOG_LINES_BEFORE=5`、`LOG_LINES_AFTER_POLLS=5`。
- 同期计数：`ARCHIVE_SYNC_RESULT_COUNT=0`、`AUTH_RESOLVER_INFO_COUNT=0`、`ROUTE_REGISTERED_COUNT=0`、`CONVERSATION_EXPORT_EVENT_COUNT=0`、`PERIODIC_FAILURE_COUNT=0`。
- 数据库实查 `cron-work-message-archive-sync-periodic-latest-success` 只有 1 行，状态 `succeeded`，多轮成功覆盖而不增长。
- 连续失败测试证明 `periodic-latest-failure` 只有稳定记录，ERROR 首次立即输出、连续失败每分钟最多一次、恢复 INFO 一次；archive 连续三次 periodic 失败也只输出一条业务 ERROR，恢复只输出一条 INFO；期间的手工失败仍独立输出，手工成功不会提前产生 periodic 恢复日志。

## 6. 代表性日志读取结果

以下均读取实际 logger 输出，不以静态搜索代替：

| 场景 | 结果 |
| --- | --- |
| 启动成功 | Docker JSON INFO：`runtime_starting` 后读取到恰好三条 `runtime_listening`，分别含 `listener=sidebar|operation|main`、component/step/result/listen_addr，证明均在 bind 成功后可检索 |
| 配置缺失/错误 | secret 相对路径被误挂成目录时输出单条 `runtime_failed`，明确“读取密钥文件失败”；非法 `MOCHAT_LOG_LEVEL=verbose` 输出中文受控提示并以 1 退出；会话存档启用但凭据不完整时输出 `archive_sync_failed`、`corp_id` 和失败计数 |
| 外部依赖配置错误 | 使用不匹配的测试数据库凭据时输出单条 `runtime_failed`，指出 MySQL 认证失败，不输出密码 |
| 权限拒绝 | 最终 Docker 分别 POST 主、sidebar、operation 三个端口，均返回 401；三条 WARN `request_rejected` 分别含请求 ID、`HTTP_401`、`wecom_sync`、path、status 和耗时 |
| 关键写成功/5xx/panic/ready 失败 | `httptest` 实际执行中间件，分别读到 INFO `critical_request_completed`、ERROR `critical_request_failed/HTTP_500`、ERROR `critical_request_panicked`、ERROR `readiness_failed` |
| AI 成功/失败 | fake HTTP Provider 实际请求后读取 INFO/WARN/ERROR，字段含 provider/model/request_id/status/duration，敏感请求响应均不存在 |
| 回调验签/授权 | fake cryptor/store/provider 实际执行 handler 后读取拒绝、交换失败、持久化失败和成功日志 |
| 重试与恢复 | fake periodic task 连续失败并恢复，读取限流 ERROR 和单次 INFO 恢复事件 |
| 正常空轮询 | Docker archive cron 2 秒间隔运行超过 3 周期，stdout 行数不变，数据库最新成功保持 1 行 |

## 7. Docker、归档与磁盘保护证据

- 最终验证 project：`mochat-critical-flow-logging-final`；独立端口 28080/28081/28082、23316、36389；没有操作既有 `mochat-go-desktop`。
- 镜像：`mochat-critical-flow-logging-final-app:latest` 与 archive bridge 镜像，从最终工作树 Dockerfile 完整构建成功。第一次基础镜像 metadata 请求遇到 Docker Hub `EOF`，原命令重试后成功，属于外部网络瞬态失败。
- 容器：app 状态 `healthy`，`/healthz` 与 `/readyz` 均为 200。
- app stdout 实查 `LISTENING_COUNT=3`，地址分别为 8081、8082、8080，字段分别为 `listener=sidebar|operation|main`；单元回归同时断言三条事件契约。
- `docker inspect`：app、archive-bridge、MariaDB、Redis 均为 `{"Type":"json-file","Config":{"max-file":"5","max-size":"20m"}}`。
- 主端口 28080、sidebar 28081、operation 28082 对同一未授权写请求分别返回 401；app stdout 恰好出现三条对应 `request_rejected`，证明侧监听没有绕过中间件。
- app、bridge、simulator 统一接收 `MOCHAT_LOG_LEVEL/FORMAT/SOURCE`；app、bridge、simulator、MySQL、Redis 均引用相同 rotation anchor。
- 应用只写 stdout/stderr，Compose 没有挂载应用日志目录；默认每容器 20 MiB × 5，约 100 MiB。
- 生产采集应从 Docker stdout/stderr 读取。若平台改用 `local`/journald 并承担保留，须调整 Compose，保持只有一层轮转；不得再让应用写文件重复轮转。
- taskrunner 运行历史与业务合规审计分离：前者默认保留 14 天并分批清理，后者不由本专项删除。

## 8. 自动化门禁结果

| 命令 | 结果 |
| --- | --- |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `pnpm lint` | PASS |
| `pnpm typecheck` | PASS |
| `pnpm test` | PASS；Dashboard 147 个文件、966 个测试通过 |
| `pnpm build` | PASS；仅有既有 chunk size warning |
| `pnpm check:logging` | PASS |
| `docker compose ... config --quiet` | PASS，退出码 0 |
| `docker build ...` | PASS |
| `git diff --check` | PASS；仅输出仓库换行策略的 LF→CRLF warning，无空白错误 |
| 最终只读代码复审 | READY；Critical/Important/Minor 均为 0 |

`pnpm test` 的 Dashboard 测试仍输出既有 jsdom `window.getComputedStyle(..., pseudoElt) not implemented` stderr，但测试全部通过，本专项未修改该前端路径。

`pnpm docs:check` 仍为仓库既有失败：`docs/phases/phase-3-dashboard/benchmark/README.md` 指向不存在的 `../phase-2.1-functional-frontend-migration/evidence/README.md`。该问题在本专项基线已存在，与日志变更无关，因此没有越权修改。

Windows 本机 `go test -race` 未执行成功，Go 工具明确提示 `-race requires cgo; enable cgo by setting CGO_ENABLED=1`；这不是普通测试失败，当前环境无可用 CGO race 配置。专项使用普通全量测试、延迟 SQL 并发边界测试与 Docker 运行证据替代，未把 race 声称为 PASS。

## 9. 根因修复

### 9.1 空存档轮询

根因是 cron 每轮无条件输出汇总且每个 tick 写唯一执行记录。修复为业务空结果不输出 INFO、成功使用稳定 latest execution ID，并保留真实非空/失败边界。

### 9.2 连续失败放大

根因是短周期依赖故障为每次失败创建唯一历史并逐轮输出 ERROR。修复为稳定 latest-failure 记录、首轮错误、每分钟限流、恢复通知；没有隐藏失败。

### 9.3 迁移前导出 worker

真实 Docker 首轮观察到 `mochat_go_work_message_export_tasks` 不存在时 conversation export worker 每 2 秒失败，属于任务启用顺序错误。修复为 Compose 和 `.env.example` 默认关闭；部署步骤明确先迁移、核表、再显式启用，策略门禁阻止回归。

### 9.4 会话正文进入 error

根因是 bridge 非 2xx 拼接 response body、解析失败拼接 raw JSON。修复为只返回受控 HTTP 状态和固定解析错误；上层可以安全记录。

### 9.5 archive 双重失败日志

根因是 archive cron 每轮先输出业务 ERROR，再把 error 返回给通用 Periodic 输出第二层 ERROR；通用层即使限流，业务层仍可每 2 秒增长。修复为 archive 自身持有 periodic 业务失败、限流和恢复日志，Periodic 对该任务只维护稳定执行记录并显式抑制重复结果日志。手工 `RunCorp` 不共享该状态：手工失败每次记录，手工成功不把仍在持续的 periodic 故障误报为恢复。

### 9.6 历史清理阻塞记录写路径

根因是过期 DELETE 与当前 execution INSERT 曾共用 2 秒记录 context。修复为先提交当前记录，再用独立 30 秒 context 启动单实例异步清理；延迟 250ms 的 SQL mock 证明记录调用在 100ms 内返回，清理失败仍只输出 WARN。

## 10. 已知边界

1. 仍有未逐模块迁移的 legacy `log` 调用，它们通过兼容 bridge 输出统一 JSON 并接受文本脱敏；本专项只显式迁移高风险边界，没有把 731 处调用全部机械改造成“伪结构化”日志。
2. 当前 standalone `/readyz` 在进程运行中不实时探测数据库；暂停 MySQL 后仍可能为 200。因此 Docker 证据只证明现有 ready contract 与健康成功静默，数据库 503 的日志由中间件运行测试证明，不宣称 ready 等价于数据库可用。
3. 本地使用 fake AI、fake 回调和无消息 archive bridge，没有使用真实企业微信或生产 Provider 凭据；真实业务连通性不属于本次日志体系的证明范围。
4. 本专项没有引入分布式 trace exporter；`request_id` 是当前架构可安全落地的 HTTP 关联字段，taskrunner 使用已有 `run_id/execution_id`。
5. 应用日志保留由 Docker/平台负责；业务审计的法定保留和清理政策不在本专项修改范围。
6. HTTP writer 提供标准 `Unwrap`，`http.ResponseController` 可透传到底层；仓库当前没有 WebSocket/SSE handler。未来若引入直接断言 `http.Flusher/Hijacker/Pusher` 的 handler，应先补对应兼容测试和显式透传。

## 11. 环境保护与提交

- 所有开发在 `chore/critical-flow-logging-20260828` 的隔离 worktree 完成。
- 主检出中的用户未提交文件、其他分支、worktree 和 `mochat-go-desktop` 容器/卷均未修改。
- 最终专项 Docker project 在验证结束后按 project label 核验并执行 `down -v`；清理后专项容器、网络、卷计数均为 0。临时目录的绝对路径与 worktree 预期路径完全一致后才递归删除，删除后 `Test-Path=False`；其中只有本专项生成的两个测试密钥文件和一个测试 env 文件，不可恢复且不含用户数据。
- 清理后再次检查，既有 `mochat-go-desktop` app、MySQL、Redis、archive-bridge 四个容器仍为 healthy。
- 精确最终提交 SHA 在提交完成后由交付回复记录。
