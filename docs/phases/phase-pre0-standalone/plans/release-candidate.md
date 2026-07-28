# MoChat Go 独立交付候选说明

本文档用于把当前 Go 独立版交给部署或验收人员使用。它只覆盖不依赖原 PHP 项目的 Go standalone 交付；真实企业微信、微信开放平台和生产 SaaS 证据仍需要目标环境补齐。

## 当前结论

- 本地短门禁已通过，最新证据见 `docs/phases/phase-pre0-standalone/evidence/latest/index.md`。
- Go standalone 不依赖原 `mochat/` PHP checkout，独立交付包 smoke 已通过。
- manifest 运行时路由覆盖为 `224/224`，旧前端 dist API 覆盖审计已通过。
- 发布候选生成已纳入 `release.candidate.gate` 双人审批；直接绕过审批调用返回 `428`。
- 服务账号创建、配置变更、Key 轮换和 Key 吊销均已纳入强制双人审批；创建与轮换只在审批执行阶段生成一次性明文 Key。
- 合规导出保留期内提前删除已纳入 `compliance.export.delete` 双人审批和可恢复 Saga；直接删除返回 `428`，失败任务可人工或维护任务重试。
- 生产候选仍未通过，原因是 6 类真实生产证据尚未齐备。
- 当前不再把 24 小时持续运行作为默认开发动作；稳定性证据默认使用短稳回归、健康检查和外部监控记录。

## 2026-07-18 预览候选快照

- 预览 App 镜像为 `sha256:d1a9045aadf58a43e99bd0e5394f1f2cb9118bdd7f8331cbe8692752a3473b40`，源码与验收指纹为 `ec0103a147f2cbb3baf9675ebf5837b7eeee579965337f8714caa4c2bc29f4a8`，迁移账本为 `93/0093_saas_payment_settlement_resolve_guard`。
- 已创建并校验加密备份，隔离恢复演练通过，临时恢复数据库已自动清理；平台健康为 `31/31`。
- 通知、企业微信、微信开放平台和身份安全使用四个独立密钥域并强制凭据加密；企业微信存量凭据的旧明文和待轮换数量均为 0。
- SaaS 总后台已按 RBAC 拆分为 8 个工作区，支持模块跳转、跨重载状态保持和移动端当前标签自动定位；无权限工作区不会显示。
- 平台范围的总览、经营、续费、客户成功、运营待办、日报和导出已统一使用业务租户口径并排除平台控制租户；显式租户范围仍可查看平台租户。真实 MariaDB/Redis smoke 和鉴权 API 断言均通过。
- SaaS 总后台已在桌面 `1440x1000` 和移动 `390x844` 完成真实浏览器回归，页面无整体横向溢出，控制台 0 error/0 warning，证据见 `output/playwright/saas-admin-business-scope-desktop.png`、`output/playwright/saas-admin-business-scope-mobile.png` 和 `output/playwright/saas-admin-business-scope-mobile-summary.png`。
- 该快照只代表本地预览候选：发布准备为 `0/6`、`ready=false`，异地备份副本尚未配置，身份安全仍为 `sessionEnforced=false`。本轮未启动 24 小时运行。

## 交付内容

- Go 服务入口：`cmd/mochat-go`
- 初始化入口：`cmd/mochat-bootstrap`
- 数据库迁移入口：`cmd/mochat-migrate`
- 独立部署栈：`deploy/standalone/docker-compose.yml`
- 环境变量样例：`deploy/standalone/.env.example`
- 前端构建产物：`web/dashboard/dist`、`web/sidebar/dist`、`web/operation/dist`
- 本地验收脚本：`scripts/collect_standalone_evidence.sh`
- 生产证据采集说明：`docs/phases/phase-pre0-standalone/evidence/production/README.md`

## 部署步骤

复制并填写本地 env 文件：

```bash
cp deploy/standalone/.env.example deploy/standalone/.env.local
```

替换 `.env.local` 中所有 `CHANGE_ME` 值，至少包括 MySQL 密码、dashboard JWT 密钥、sidebar JWT 密钥和管理员初始密码。不要把 `.env.local` 提交到仓库。

启动完整 Go standalone 栈：

```bash
docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app up -d --build
```

记录 schema baseline：

```bash
docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app exec app \
  mochat-migrate -action baseline -project-root /app
```

创建默认租户和管理员：

```bash
set -a
. deploy/standalone/.env.local
set +a

docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app exec app \
  mochat-bootstrap \
    -tenant-id "$MOCHAT_BOOTSTRAP_TENANT_ID" \
    -tenant-name "$MOCHAT_BOOTSTRAP_TENANT_NAME" \
    -phone "$MOCHAT_BOOTSTRAP_PHONE" \
    -password "$MOCHAT_BOOTSTRAP_PASSWORD" \
    -package-code "$MOCHAT_BOOTSTRAP_PACKAGE_CODE" \
    -package-name "$MOCHAT_BOOTSTRAP_PACKAGE_NAME"
```

默认访问地址：

- Dashboard/API：`http://127.0.0.1:18080`
- Sidebar：`http://127.0.0.1:18081`
- Operation：`http://127.0.0.1:18082`
- SaaS 总后台：启用 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 后访问 `http://127.0.0.1:18080/dashboard/saasAdmin/page`

SaaS 总后台包含租户、套餐、用户、企业、告警、通知 outbox、待通知、到期、用量汇总、单租户详情、租户生命周期审计、租户用量明细、运营日报、经营指标、风险看板、客户成功队列、风险跟进、风险跟进任务、负责人工作台、风险跟进批量关闭、告警处置、告警批量解决、通知重试、批量通知重试、套餐同步任务、续费任务、续费预测任务、续费预测分派、续费预测提醒、续费预测负责人工作台、运营任务中心、运营任务批量取消、运营任务批量重置、运营任务重置、续费账单、账单筛选汇总、账单对账、账单跟进任务、账单跟进负责人工作台、操作记录筛选汇总和 CSV 导出，并提供平台级套餐与租户运营能力。`MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID` 指定平台租户，只有该租户下的超级管理员可以用 `scope=platform` 查看全局、读取套餐列表、查看任意租户详情、生命周期审计、用量明细、运营日报、经营指标、风险看板、客户成功队列、记录风险跟进并在风险看板/风险 CSV 回显最新跟进状态、按负责人聚合查看风险跟进任务分布、按筛选条件批量关闭风险跟进任务、告警处置和通知 outbox、创建/编辑/停用套餐、查看套餐保存影响摘要、受控同步套餐目录到存量租户快照、创建并应用套餐同步任务、创建并应用续费任务、按续费预测批量生成续费任务、按续费预测批量分派负责人、按续费预测批量生成提醒、开通新业务租户和初始超级管理员、开停业务租户、查看操作记录、导出租户/生命周期审计/用量/套餐/风险/跟进/告警/通知/运营日报/运营待办/运营待办负责人/运营待办认领/续费预测/续费预测负责人/操作/账单/对账 CSV、记录续费账单、给指定租户调整套餐和到期时间；经营指标通过 `GET /dashboard/saasAdmin/businessMetrics` 基于全平台租户、风险看板和最近续费账单估算 MRR、ARR、ARPA、风险收入、到期收入、未知价格租户和套餐收入分布；续费预测通过 `GET /dashboard/saasAdmin/renewalForecast` 查看未来到期租户的预测续费金额、到期桶、未知价格租户、客户成功负责人、负责人工作台和续费任务跟进状态，并可通过 `POST/PUT /dashboard/saasAdmin/renewalForecastTasks` 按同一预测筛选生成 `tenant_renewal` 运营任务，也可通过 `POST/PUT /dashboard/saasAdmin/renewalForecastAssign` 按同一预测筛选写入续费跟进负责人、状态、下次跟进时间和备注，还可通过 `POST/PUT /dashboard/saasAdmin/renewalForecastNotifications` 按同一预测筛选生成 `tenant_renewal_reminder` 通知 outbox，默认跳过已有待处理续费任务；客户成功队列通过 `GET /dashboard/saasAdmin/customerSuccess` 汇总风险、风险跟进、账单跟进、运营任务和失败/耗尽通知，输出待处理租户优先级、健康分、负责人、到期状态、原因和下一步动作；套餐保存会返回 26 项额度变更、分配租户数、降额后会超新额度的租户样例，并明确租户套餐快照不会被自动改写；平台管理员可通过套餐快照同步先 `dryRun` 预览命中租户和超额指标，实际同步默认会阻断超额租户，显式 `allowOverLimit=true` 后才会写入租户快照并刷新 26 项用量额度；也可通过 `packageSyncTask` 创建带预览结果的套餐同步任务，通过 `packageSyncTaskApply` 在应用前重新计算超额风险，成功后记录执行结果和应用时间；也可通过 `tenantRenewalTask` 先固化续费请求并生成当前租户续费预览，通过 `tenantRenewalTaskApply` 在应用前复核当前租户和套餐状态，成功后写入续费账单、任务执行结果和应用时间，并通过 `tasks` 回看任务状态；账单对账可复用账单筛选条件比对续费账单和当前租户套餐，`mismatchOnly=1` 只看套餐缺失、套餐未启用、套餐不一致或当前到期未覆盖账单到期的异常，可追加异常跟进操作日志，并可按状态、负责人、到期状态和关键字查看账单跟进任务及负责人分布；统一运营任务中心可行内取消单条未应用任务，也可通过 `taskBulkCancel` 按当前任务筛选批量取消未应用任务并跳过已应用/已取消任务，可通过 `taskBulkReset` 按当前任务筛选批量重置 `failed/blocked` 任务并跳过待应用/已应用/已取消任务，还可通过 `taskReset` 把 `failed/blocked` 任务重置为 `pending` 后重新应用，任务创建、应用、阻断、取消、批量取消、批量重置和重置均会写入 `admin_task` 操作记录，页面任务行内可直接追溯该任务的操作记录；运营日报可汇总当前风险、风险跟进负责人、打开告警、失败/耗尽通知、日期窗口内操作和账单金额，页面支持日报日期和统计天数筛选，展示日报负责人、失败通知、操作动作和账单流水明细，并可通过 `export?type=dailyReport` 导出按 `section/metric/value/remark` 组织的日报 CSV；普通租户超级管理员只能查看本租户数据、本租户详情、本租户用量明细、本租户风险、本租户告警和本租户通知，并只能解决或批量解决本租户告警、重试或批量重试本租户通知，且不会看到平台内部风险跟进备注。业务租户停用后，新登录会返回 `403`，旧 token 访问依赖 `UserByID` 的后台接口会失效。

`0045_saas_admin_rbac` 让平台访问不再只依赖“平台租户超级管理员”单一身份：平台超级管理员保留隐式全权，平台普通成员必须处于启用状态并通过角色获得对应权限。该版本先覆盖平台概览、租户、运营、通知、财务、审计和访问治理，并提供五个内置角色；未知总后台路由对非超级管理员默认拒绝，自定义角色和成员授权使用乐观锁，访问治理操作写入独立审计动作。角色和权限总数以紧随其后的 `0046` 最新口径为准。

自 `0046_saas_admin_approvals` 起，平台权限扩展为 14 项并新增第六个内置角色“平台审批人”。退款创建、业务租户停用、结算关账、平台角色保存和成员授权保存默认必须走审批单，由非发起人复核并执行；自批、自执行和给本人提权均被服务端拒绝。审批单、不可变事件、操作审计和目标业务写入形成可追溯链路，五类业务事务会原子写入副作用标记；业务已提交但审批完成回写中断时，15 分钟租约恢复只补审批状态，不重放业务动作。失败执行回到已批准并可重试。`MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=0` 仅作为迁移期紧急回退，不是生产默认口径。

租户订阅生命周期已从“套餐到期字段”升级为可运营域模型：`trialing/active/grace` 允许登录和旧 token 访问，`past_due/suspended/canceled` 同时拦截两条链路；无订阅记录的存量租户仍使用旧套餐到期兼容口径。平台超管可通过 `subscriptions`、`subscriptionEvents`、`subscriptionTransition`、`subscriptionReconcile` 和 `export?type=subscriptions` 查看、迁移、校准和导出；迁移带 `expectedVersion` 乐观锁和 `idempotencyKey` 幂等键，订阅事件与平台操作日志事务化落库。开户、套餐调整、续费和租户开停会同步订阅；`cron-saas-subscription-reconcile` 默认每 5 分钟校准到期、宽限期结束和期末取消。

支付收款已从“人工记录续费账单”升级为平台级收款与退款域模型：平台超管可通过 `paymentOrders`、`paymentWebhookEvents`、`paymentOrder`、`paymentOrderCancel`、`paymentDunning`、`paymentRefunds`、`paymentRefund`、`paymentRefundCancel` 和对应 CSV 查看、创建、取消、催缴、退款与导出。`POST /webhooks/saas/payment` 作为支付提供方中立适配边界，使用时间戳和 HMAC-SHA256 签名，按提供方事件 ID 幂等处理付款与退款事件；付款成功在同一事务结算订单、续费账单、租户套餐与订阅，退款成功原子写入退款账单冲销并更新订单退款累计。部分退款默认保留权益，全额退款可显式保留、暂停或取消订阅；失败退款释放金额预占，订单汇总、经营指标、月度趋势和运营日报按毛收入、退款额与净收入展示。普通租户不能访问这些平台接口。

统一运营任务中心已补齐负责人聚合视图：`GET /dashboard/saasAdmin/taskOwners` 复用 `tasks` 的 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选，按任务操作人聚合任务总数、可处理/阻断/失败/已应用分布、三类任务分布、最近任务、最近应用时间和最近错误；该接口只读复用 `mochat_go_saas_admin_tasks`，不新增迁移，普通租户超级管理员访问返回 `403`。页面“任务负责人”表会随任务中心筛选刷新，便于平台运营按人收口待处理任务。

统一运营任务中心已补齐 SLA/逾期视图、SLA CSV 导出和 SLA 催办提醒：`GET /dashboard/saasAdmin/taskSla` 复用 `tasks` 的筛选条件，并支持 `warningHours` 和 `overdueHours` 阈值；接口只把 `pending/blocked/failed` 作为活跃任务计算，输出正常、预警、逾期、未知时间、最大任务年龄、负责人聚合和最严重任务列表；`GET /dashboard/saasAdmin/export?type=taskSla` 按同一筛选导出活跃任务 SLA CSV，包含负责人、SLA 状态、任务年龄和超时小时数；`POST/PUT /dashboard/saasAdmin/taskSlaNotifications` 默认把预警和逾期活跃任务写入通知 outbox，`alertType=admin_task_sla_reminder`、`metric=admin_task_sla`，并追加 `saas.admin.task.sla_notify` 操作日志，不新增迁移，普通租户超级管理员访问返回 `403`。页面“任务SLA”表可按当前任务筛选和阈值刷新，数据导出区提供“导出任务SLA CSV”，任务筛选区提供“生成SLA提醒”，便于平台运营识别超时未处理任务并离线跟进。

运营日报已接入任务 SLA 摘要和明细：`GET /dashboard/saasAdmin/dailyReport` 会默认用 4 小时预警、24 小时逾期统计 `pending/blocked/failed` 活跃运营任务，返回 `summary.taskSla*`、`taskSla.summary`、负责人聚合和最严重任务列表；内置页面新增“日报任务SLA”明细表，`export?type=dailyReport` 也会输出 `taskSlaOwner` 和 `taskSlaTask` section，便于平台运营在日报和日报 CSV 中同步看到超时任务压力。

运营日报已接入关闭通知复盘：`GET /dashboard/saasAdmin/dailyReport` 的 `notifications.summary` 新增 `closedCount`，`summary` 新增 `closedNotificationCount` 和 `windowNotificationCloseCount`，`notifications.closedItems` 返回日期窗口内被关闭的通知；内置页面“日报通知”会同时展示可重试通知和已关闭通知，`export?type=dailyReport` 新增 `closedNotification` section，便于平台运营在日报中复盘主动终止的通知 outbox。

运营日报已接入待办认领复盘：`GET /dashboard/saasAdmin/dailyReport` 会从日期窗口内的 `saas.admin.operation_queue.assign` 操作日志提取队列级认领记录，`summary` 新增 `windowQueueAssignmentCount`、`windowTaskSlaAssignCount`、`windowNotificationAssignCount`、`windowClosedNotificationAssignCount` 和 `windowNotificationHealthAssignCount`，响应新增 `operationQueueAssignments` 分区，并按 `nextFollowUpAt` 汇总认领记录的 `overdue/due_soon/future/no_date/closed` 到期状态；内置页面新增“日报待办认领”表，`export?type=dailyReport` 新增 `operationQueueAssignment` section，便于平台运营在日报中复盘当天任务 SLA、失败通知、关闭通知和通知健康异常的认领动作。

SaaS 总后台已接入运营待办队列：`GET /dashboard/saasAdmin/operationQueue` 复用客户成功、任务 SLA、账单跟进、通知 outbox、关闭通知和通知健康度，不新增任务表；平台管理员可按来源、优先级、负责人、关键字、租户上限、到期窗口、高用量阈值、SLA 阈值、通知健康窗口和积压阈值筛选，响应返回队列汇总、来源分布、优先级分布、未分配数量和待办明细。`warning/critical` 租户会进入 `notification_health` 来源，健康和无数据租户不会生成异常待办。内置页面新增“运营待办队列”，支持来源、优先级、负责人和关键字筛选；`GET /dashboard/saasAdmin/export?type=operationQueue` 可按同口径导出待办 CSV，便于平台运营线下分派和复盘；普通租户访问和导出返回 `403`。

SaaS 总后台已接入运营待办负责人工作台：`GET /dashboard/saasAdmin/operationQueueOwners` 复用运营待办筛选条件，按负责人聚合待办数、租户数、来源数、优先级分布、客户成功、任务 SLA、账单跟进、通知、关闭通知、通知健康异常、未分配数量、最大停留小时、Top 租户和 Top 待办；`GET /dashboard/saasAdmin/export?type=operationQueueOwners` 按同口径导出负责人 CSV。内置页面在“运营待办队列”下方新增“运营待办负责人工作台”，并在客户成功、续费预测、风险跟进、账单跟进、告警、通知和任务 SLA 阈值变化后同步刷新；普通租户访问和导出返回 `403`。

运营待办已补齐可写分派入口和认领记录视图：`POST/PUT /dashboard/saasAdmin/operationQueueAssign` 复用运营待办筛选条件和 `owner/status/nextFollowUpAt/remark` 请求体，按当前筛选批量分派待办；`customer_success` 和 `billing_follow_up` 写入原业务审计链路，分别落到风险跟进记录和账单跟进操作日志；`task_sla`、失败通知、关闭通知和 `notification_health` 没有原生负责人字段，因此落到 `saas.admin.operation_queue.assign` 运营队列级认领日志，并在后续 `operationQueue`、`operationQueueOwners` 和 CSV 中覆盖负责人。`GET /dashboard/saasAdmin/operationQueueAssignments` 会把认领日志提取为独立视图，支持按来源、负责人、状态、认领到期状态 `dueState`、对象、租户和关键字筛选；`GET /dashboard/saasAdmin/export?type=operationQueueAssignments` 按同口径导出认领 CSV，输出操作 ID、租户、来源、对象、负责人、状态、到期状态、备注、操作者和认领时间。页面“运营待办批量分派”会按当前来源、优先级、负责人和关键字筛选提交，并刷新待办、负责人、认领记录、客户成功、风险跟进、账单跟进、操作记录和日报；通知健康度工具条可直接填写异常负责人和复查时间后分派当前严重/预警租户；数据导出区新增“导出待办认领 CSV”；普通租户访问和导出返回 `403`。

运营待办认领已接入到期提醒：`POST/PUT /dashboard/saasAdmin/operationQueueAssignmentNotifications` 复用 `operationQueueAssignments` 的来源、负责人、状态、租户、对象、关键字和 `dueState` 筛选，只允许 `dueState=overdue/due_soon`，未传时默认提醒逾期认领；接口会生成 `metric=operation_queue_assignment`、`alertType=operation_queue_assignment_reminder` 的通知 outbox，并写入 `saas.admin.operation_queue.assignment_notify` 操作日志，响应返回匹配数、可提醒数、入队数、重复跳过数和跳过原因。内置页面新增“生成认领到期提醒”按钮，复用当前认领到期筛选并刷新认领记录、通知、操作记录和日报；普通租户访问返回 `403`。

经营指标和经营趋势 CSV 已补齐：平台管理员可用 `export?type=businessMetrics` 和 `export?type=businessTrends` 按现有筛选导出经营汇总、套餐收入估算、月度流水和续费任务漏斗；普通租户导出返回 `403`。

账单跟进任务已补齐批量关闭能力：平台管理员可通过 `POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose` 按任务状态、负责人、到期状态、关键字和租户筛选，将命中的未关闭账单跟进批量置为 `resolved/ignored`，并逐账单事件继续追加 `billing.reconciliation.follow_up` / `billing_event` 操作日志；页面提供“批量关闭账单跟进”入口，普通租户访问该写入口返回 `403`。

账单跟进任务也已接入 CSV 导出：`GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUps` 复用账单跟进任务筛选条件，输出账单事件 ID、租户、状态、负责人、下次跟进、到期状态、备注、套餐、金额、订单号和操作 ID；页面“数据导出”区域提供“导出账单跟进 CSV”。

风险和账单负责人工作台也已接入 CSV 导出：`GET /dashboard/saasAdmin/export?type=riskFollowUpOwners` 与 `GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners` 分别复用对应任务筛选条件，输出负责人任务总数、打开/关闭数量、状态分布、到期分布、最近跟进和下次跟进时间；页面“数据导出”区域提供“导出风险负责人 CSV”和“导出账单负责人 CSV”。

告警和通知 outbox 已接入 CSV 导出：`GET /dashboard/saasAdmin/export?type=alerts` 复用告警筛选条件导出告警 key、状态、严重级别、指标、用量、来源、消息和上下文；`GET /dashboard/saasAdmin/export?type=notifications` 复用通知筛选条件导出通知 key、状态、通道、重试次数、指标、失败原因和下次重试时间；页面“数据导出”区域提供“导出告警 CSV”和“导出通知 CSV”。

## 本地验收

完整短验收：

```bash
DOCKER_CONFIG=/tmp/mochat-go-docker-config \
env -u GOROOT \
MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all \
./scripts/collect_standalone_evidence.sh
```

关键输出：

- `docs/phases/phase-pre0-standalone/evidence/latest/index.md`
- `docs/phases/phase-pre0-standalone/evidence/latest/source-fingerprint.json`
- `docs/phases/phase-pre0-standalone/evidence/latest/production-evidence.json`
- `docs/phases/phase-pre0-standalone/evidence/latest/goal-completion.json`

只验证 SaaS 总后台短链路时可执行：

```bash
DOCKER_CONFIG=/tmp/mochat-go-docker-config \
env -u GOROOT \
./scripts/smoke_saas_admin_dashboard.sh
```

只验证订阅生命周期和访问控制时可执行：

```bash
DOCKER_CONFIG=/tmp/mochat-go-docker-config \
env -u GOROOT \
./scripts/smoke_saas_subscription_lifecycle.sh
```

只验证支付收款和催缴闭环时可执行：

```bash
DOCKER_CONFIG=/tmp/mochat-go-docker-config \
env -u GOROOT \
./scripts/smoke_saas_payment_collection.sh
```

只验证支付退款、净收入和订阅权益联动时可执行：

```bash
DOCKER_CONFIG=/tmp/mochat-go-docker-config \
env -u GOROOT \
./scripts/smoke_saas_payment_refunds.sh
```

只验证开票资料、蓝票、红冲和租户账单中心时可执行：

```bash
DOCKER_CONFIG=/tmp/mochat-go-docker-config \
env -u GOROOT \
./scripts/smoke_saas_billing_invoices.sh
```

该 smoke 会验证 `GET /dashboard/saasAdmin/overview`、`GET /dashboard/saasAdmin/tenant`、`GET /dashboard/saasAdmin/tenantLifecycle`、`GET /dashboard/saasAdmin/usage`、`GET /dashboard/saasAdmin/risk`、`GET /dashboard/saasAdmin/customerSuccess`、`GET /dashboard/saasAdmin/customerSuccessOwners`、`POST/PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications`、`POST/PUT /dashboard/saasAdmin/renewalForecastTasks`、`POST/PUT /dashboard/saasAdmin/renewalForecastAssign`、`POST/PUT /dashboard/saasAdmin/renewalForecastNotifications`、`POST/PUT /dashboard/saasAdmin/riskFollowUp`、`GET /dashboard/saasAdmin/riskFollowUps`、`GET /dashboard/saasAdmin/riskFollowUpOwners`、`GET /dashboard/saasAdmin/dailyReport`、`POST/PUT /dashboard/saasAdmin/riskFollowUpBulkClose`、`GET /dashboard/saasAdmin/alerts`、`GET /dashboard/saasAdmin/notifications`、`GET /dashboard/saasAdmin/packages`、`GET /dashboard/saasAdmin/operations`、`GET /dashboard/saasAdmin/billingEvents`、`GET /dashboard/saasAdmin/billingReconciliation`、`POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUp`、`GET /dashboard/saasAdmin/billingReconciliationFollowUps`、`GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners`、`GET /dashboard/saasAdmin/tasks`、`GET /dashboard/saasAdmin/taskSla`、`GET /dashboard/saasAdmin/export`、`POST/PUT /dashboard/saasAdmin/alertResolve`、`POST/PUT /dashboard/saasAdmin/alertBulkResolve`、`POST/PUT /dashboard/saasAdmin/notificationRetry`、`POST/PUT /dashboard/saasAdmin/notificationBulkRetry`、`POST/PUT /dashboard/saasAdmin/package`、`POST/PUT /dashboard/saasAdmin/packageSync`、`POST/PUT /dashboard/saasAdmin/packageSyncTask`、`POST/PUT /dashboard/saasAdmin/packageSyncTaskApply`、`POST/PUT /dashboard/saasAdmin/packageSyncTaskBulkApply`、`POST/PUT /dashboard/saasAdmin/taskReset`、`POST/PUT /dashboard/saasAdmin/taskBulkReset`、`POST/PUT /dashboard/saasAdmin/tenantStatus`、`POST/PUT /dashboard/saasAdmin/tenantRenewal`、`POST/PUT /dashboard/saasAdmin/tenantRenewalTask`、`POST/PUT /dashboard/saasAdmin/tenantRenewalTaskApply`、`POST/PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply`、`POST/PUT /dashboard/saasAdmin/tenantProvision`、`POST/PUT /dashboard/saasAdmin/tenantProvisionTask`、`POST/PUT /dashboard/saasAdmin/tenantProvisionTaskApply`、`POST/PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply` 和 `POST /dashboard/saasAdmin/tenantPackage`：平台管理员和租户管理员都可读取授权范围内的租户详情、用量明细、风险看板、告警和通知 outbox，平台 overview 的租户列表可按 `keyword`、`tenantStatus`、`packageCode` 和 `dueState` 筛选且 `summary` 保持全局统计口径，用量明细可下钻 26 项指标、剩余额度、状态、打开告警数和更新时间，风险看板会按停用、到期、未开套餐、打开告警和高用量阈值返回租户 `riskLevel`、`riskScore`、风险原因、建议动作和 Top 风险指标，并可通过 `export?type=risk` 导出风险 CSV；客户成功队列会在同一 smoke 中读取 `customerSuccess`，断言队列汇总、待处理租户、优先级、健康分、原因、下一步动作和失败通知数量，普通租户访问返回 `403`，客户成功负责人工作台会读取 `customerSuccessOwners`，并按负责人聚合租户数、优先级分布、逾期/阻断/通知信号和 Top 租户，也可通过 `export?type=customerSuccessOwners` 导出 CSV；平台管理员还可通过 `customerSuccessRenewalNotifications` 和 `renewalForecastNotifications` 生成 `tenant_renewal_reminder` 通知 outbox；平台管理员可通过 `riskFollowUp` 记录风险跟进状态、负责人、下次跟进时间和备注，并写入 `tenant.risk.follow_up` 操作日志，随后风险看板、`riskFollowUps` 任务列表、`riskFollowUpOwners` 负责人工作台、显式带 `date/days` 的 `dailyReport` 运营日报、日报明细面板、`export?type=dailyReport` 日报 CSV、`export?type=riskFollowUps` 跟进任务 CSV 和风险 CSV 会回显最新跟进状态、负责人、下次跟进时间、备注、到期状态、操作 ID、负责人任务分布、打开跟进、打开告警、失败通知、日报窗口日期、日报 CSV summary 和当日操作，再通过 `riskFollowUpBulkClose` 按筛选条件把任务批量置为 `resolved/ignored` 并继续追加 `tenant.risk.follow_up` 操作日志，普通租户调用风险跟进写入、任务列表、负责人工作台、运营日报和批量关闭接口均返回 `403` 且普通租户风险自查不暴露平台跟进备注；告警可按租户、状态、指标和告警类型筛选，并通过 `alertResolve` 解决单条打开告警、通过 `alertBulkResolve` 按筛选条件批量解决打开告警，解决后打开告警数归零并写入 `tenant.alert.resolve` 操作日志；通知 outbox 可按租户、状态、通道和关键字筛选，并通过 `notificationRetry` 将单条 failed/dead 通知重新排队，通过 `notificationBulkRetry` 将筛选命中的 failed/dead 通知批量重新排队，重试后状态回到 pending、失败原因清空并写入 `tenant.notification.retry` 操作日志；操作记录可按 `action`、`targetType`、`keyword` 追溯关键动作并返回不受列表 `limit` 影响的全量命中操作数、租户数、操作人数和动作类型汇总，运营任务可用具体 `saas.admin.task.*` action 与 `targetType=admin_task` 追溯任务创建/应用/阻断/取消/批量取消/批量重置/重置，账单流水可按 `eventType`、`packageCode`、`keyword` 定位续费订单并返回筛选金额汇总，账单对账会复用相同筛选条件校验正常续费订单为 `matched`，并通过模拟漂移账单断言 `mismatchOnly=1` 返回 `package_mismatch` 和 `expires_mismatch` 异常；随后记录异常跟进并通过操作日志、`billingReconciliationFollowUps` 和 `billingReconciliationFollowUpOwners` 回看任务状态、操作 ID、负责人和关闭分布，并可按相同筛选条件导出租户、生命周期审计、用量明细、套餐清单、风险看板、客户成功队列、客户成功负责人、风险跟进任务、运营日报、操作记录和账单流水 CSV；用量 CSV 会逐租户输出 26 项 SaaS 指标、额度、剩余、状态和打开告警数，套餐 CSV 会输出套餐状态和 26 项 SaaS 额度；平台管理员创建/停用/恢复套餐时会校验 `impact` 中的额度变化、分配租户数、降额超新额度租户和租户套餐快照未自动改写口径，并通过 `packageSync` 验证 dry-run 预览不写入、实际同步遇到超额会阻断、显式允许超额后会写入租户快照并刷新 26 项用量额度，再通过 `packageSyncTask`、`packageSyncTaskApply`、`packageSyncTaskBulkApply`、`taskBulkReset`、`taskReset`、`taskSla` 和 `tasks` 验证套餐同步任务的 `blocked/pending/applied` 状态、执行结果、SLA 活跃任务汇总、批量重置、单条重置回 pending 和应用时间留痕；通过 `tenantRenewalTask`、`tenantRenewalTaskApply`、`tenantRenewalTaskBulkApply` 和 `tasks?taskType=tenant_renewal` 验证续费任务会先生成预览、应用后写入账单事件并更新为 `applied`，也可按筛选批量应用客户成功和续费预测生成的续费任务；通过 `tenantProvisionTask`、`tenantProvisionTaskApply`、`tenantProvisionTaskBulkApply` 和 `tasks?taskType=tenant_provision` 验证开户链接任务会先固化脱敏请求、单条或批量应用后创建租户和管理员并更新为 `applied`，任务响应不回显明文密码或哈希；平台管理员还会开通新租户和初始超级管理员、开停业务租户、验证套餐到期后新登录返回 `403` 且旧用户访问 SaaS 总览返回 `401`、续费到未来时间后恢复访问，停用后业务租户自身访问 SaaS 总览返回 `401`、恢复后访问回到 `200`，再调整租户套餐和记录续费后，`mochat_go_saas_tenant_packages.limits_json` 与 26 项 `mochat_go_saas_usage_counters` 会同步刷新，平台开户会写入 `mochat_go_tenant_provision_runs`，续费会写入 `mochat_go_saas_billing_events`，关键动作会写入 `mochat_go_saas_admin_operation_logs`。

该 smoke 还会验证 `POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose`：先将模拟漂移账单的最新跟进重新打开为 `contacted`，再按状态、负责人、未来到期状态和备注关键字批量关闭，随后从账单跟进任务、操作记录和启动日志确认关闭结果与审计记录；普通租户调用该接口返回 `403`。

该 smoke 同时会验证 `GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUps`：普通租户导出返回 `403`，平台管理员按批量关闭后的账单跟进筛选导出 CSV，并断言表头、租户、状态、负责人、关闭状态、备注、账单套餐、金额和订单号。

该 smoke 同时会验证 `GET /dashboard/saasAdmin/export?type=riskFollowUpOwners` 和 `GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners`：普通租户导出返回 `403`，平台管理员按当前任务筛选导出负责人汇总 CSV，并断言负责人、任务总数、打开/关闭数量、状态分布、到期分布、最近跟进和下次跟进时间。

该 smoke 同时会验证 `GET /dashboard/saasAdmin/export?type=alerts` 和 `GET /dashboard/saasAdmin/export?type=notifications`：普通租户导出返回 `403`，平台管理员按当前告警/通知筛选导出 CSV，并断言表头、租户、状态、指标、通道和失败原因。

SaaS 总后台已新增跨租户通知策略中心：`GET /dashboard/saasAdmin/notificationPolicies` 可按租户、关键字和 `enabled/disabled/unconfigured` 状态筛选并返回平台汇总；`GET/POST/PUT /dashboard/saasAdmin/notificationPolicy` 可读取或代管租户 Webhook 开关、URL、Secret、超时、HTTP/outbox 重试、模板、事件订阅、最低严重级别、免打扰时段、IANA 时区和每小时限流策略；`POST/PUT /dashboard/saasAdmin/notificationPolicyTest` 会生成 `notification_policy_test` pending 通知并继续由现有 dispatcher 投递。即时投递和 dispatcher 使用同一策略决策：未订阅或低于最低级别写为 `suppressed`，免打扰或达到小时上限保持 `pending` 并推迟 `next_retry_at`，均不消耗发送尝试次数；测试通知绕过四项业务控制但不绕过 outbox 和真实 Webhook。响应和 `tenant.notification_policy.update` / `tenant.notification_policy.test` 审计均不包含 Secret 明文，普通租户访问三类能力均返回 `403`。内置页面新增策略状态筛选、租户列表、策略编辑和“生成测试通知”入口；`scripts/smoke_saas_admin_dashboard.sh` 覆盖保存、密钥脱敏、控制字段、测试 outbox、`suppressed` 汇总、审计落库和越权拒绝，`scripts/smoke_saas_alert_setting_dispatch.sh` 覆盖真实 MySQL 下的签名投递、两类抑制、免打扰延期、小时限流和测试通知绕过。

SaaS 总后台已新增平台级通知送达健康度中心：`GET /dashboard/saasAdmin/notificationHealth` 按租户、关键字、1 至 720 小时窗口、积压分钟数和 `healthy/warning/critical/no_data` 状态聚合 outbox；成功率分母只包含实际尝试的 delivered/failed/dead，策略抑制、关闭和未来延期单列。重试耗尽、超过积压阈值或至少 5 次尝试下成功率低于 80% 标为严重；失败待重试、到期待投递或成功率低于 95% 标为预警。响应同时返回策略配置状态、平均/最大送达延迟、最近送达/失败时间和 Top 失败原因；页面与 `export?type=notificationHealth` CSV 使用同一筛选。严重/预警租户会进入统一运营待办的 `notification_health` 来源，可沿用负责人、认领记录、到期提醒、日报和 CSV；`POST/PUT /dashboard/saasAdmin/notificationHealthRecovery` 会重新核验当前认领，只对明确为 `healthy` 且当前窗口至少存在一次成功送达的租户写入 `resolved` 关闭审计，`warning/critical/no_data` 以及只有抑制、关闭或延期而无成功送达的租户保持打开，重复执行幂等。普通租户访问健康度、恢复接口和导出均返回 `403`，`scripts/smoke_saas_admin_dashboard.sh` 会用真实 MySQL 验证严重租户、积压、成功率、失败原因、CSV、异常分派、负责人统计、恢复结案、幂等、页面和启动路由。

SaaS 总后台已新增通知 SLO 趋势中心：`GET /dashboard/saasAdmin/notificationSlo` 按通知创建日期聚合最近 1 至 90 天的每日趋势和租户排行，可配置送达成功率、目标送达秒数和时延达标率。成功率分母包含 `delivered/failed/dead/closed`，人工关闭不会让历史失败消失；`pending/suppressed` 单列，无实际尝试返回 `no_data`。内置页面提供 7/30/90 天切换、目标值、租户搜索、日趋势、租户排行和 `export?type=notificationSlo` CSV；普通租户访问返回 `403`。`0038_saas_notification_slo_index` 为平台级聚合提供通道/创建时间窗口索引，`scripts/smoke_saas_notification_slo.sh` 会从空库用真实 Go + MySQL + Redis 验证迁移、跨日聚合、达标/违约/无数据、筛选、CSV、权限和页面路由。

SaaS 总后台已新增订阅生命周期中心：`0039_saas_subscription_lifecycle` 创建 `mochat_go_saas_subscriptions` 和 `mochat_go_saas_subscription_events`，从已有租户套餐回填状态并为有限期套餐默认保留 7 天宽限期。内置页面提供状态/访问权/关键字筛选、六态汇总、订阅表、受控迁移表单、事件时间线、dry-run/应用校准和订阅 CSV。`scripts/smoke_saas_subscription_lifecycle.sh` 会在真实 MySQL/Redis/Go 下验证六态、新旧 token 拦截、乐观锁冲突、幂等重放、开停联动、续费激活与账单关联、平台租户保护、页面/CSV 和 cron 启动即跑。

该 smoke 在 `notificationClose` 和 `notificationBulkClose` 后会再次读取显式日期窗口的 `GET /dashboard/saasAdmin/dailyReport` 和 `export?type=dailyReport`，断言关闭通知进入 `closedItems`，`closedNotificationCount`、`windowNotificationCloseCount`、`notifications.summary.closedCount` 和日报 CSV 的 `closedNotification` section 回显关闭备注。

该 smoke 还会在通知关闭后读取 `GET /dashboard/saasAdmin/operationQueue` 和 `export?type=operationQueue`，断言任务 SLA、关闭通知和租户通知健康异常进入同一运营待办队列，校验 `taskSlaCount`、`closedNotificationCount`、`notificationHealthCount`、严重优先级和 CSV 表头；普通租户访问运营待办接口和导出均返回 `403`，启动日志也会暴露该路由。

该 smoke 同时会读取 `GET /dashboard/saasAdmin/operationQueueOwners` 和 `export?type=operationQueueOwners`，断言负责人聚合复用同一运营待办口径，至少包含任务 SLA、已关闭通知和通知健康异常来源，校验 `taskSlaCount`、`closedNotificationCount`、`notificationHealthCount`、Top 租户和 CSV 表头；普通租户访问负责人工作台接口和导出均返回 `403`，启动日志也会暴露该路由。

该 smoke 同时会调用 `POST /dashboard/saasAdmin/operationQueueAssign?source=customer_success`，断言客户成功待办可按当前筛选分派到 `ops-queue`，并在 `riskFollowUps` 和 `operationQueueOwners` 中回显新负责人；还会调用 `POST /dashboard/saasAdmin/operationQueueAssign?source=task_sla`，断言任务 SLA 待办通过 `saas.admin.operation_queue.assign` 操作日志认领到 `ops-task-sla`，并在 `operationQueue`、`operationQueueOwners`、`operationQueueAssignments?dueState=future`、`export?type=operationQueueAssignments&dueState=future`、`dailyReport.operationQueueAssignments`、日报 CSV 的 `operationQueueAssignment` section、`tenantLifecycle?source=operation&status=contacted`、生命周期审计 CSV 和 `operations` 中回显；还会把任务 SLA 认领到一个已逾期跟进时间，再调用 `POST /dashboard/saasAdmin/operationQueueAssignmentNotifications?source=task_sla&dueState=overdue`，断言 `operation_queue_assignment_reminder` 通知 outbox 和 `saas.admin.operation_queue.assignment_notify` 操作日志生成。普通租户调用分派、认领记录、认领提醒接口和认领记录导出返回 `403`，启动日志也会暴露这些入口。

SaaS 总后台租户生命周期审计已接入 `GET /dashboard/saasAdmin/tenantLifecycle` 和页面“生命周期审计”：平台管理员可按 `tenantId` 聚合租户详情、操作记录、账单事件、运营任务、告警、通知 outbox，并返回统一时间线；也可按 `source=all/operation/billing/task/alert/notification`、`eventType`、`status` 和 `keyword` 筛选 timeline，响应 summary 同时返回筛选后 `timelineCount`、原始 `rawTimelineCount`、`returnedEventCount` 和 `filterActive`，页面“生命周期筛选”支持按来源、事件、状态和关键字定位单类事件，并可通过“导出审计 CSV”使用 `GET /dashboard/saasAdmin/export?type=tenantLifecycle` 导出当前筛选后的时间线；普通租户访问和导出返回 `403`。`saas.admin.operation_queue.assign` 认领日志在生命周期中属于 `source=operation`，事件状态会从认领 payload 派生，因此可用 `source=operation&eventType=operation_queue.assign&status=contacted&keyword=负责人或备注` 定位并导出任务 SLA、失败通知和关闭通知的队列级认领动作。smoke 会在套餐同步、平台开户、续费任务和账单事件产生后读取该接口，断言 timeline 同时包含 `operation/billing/task/alert/notification` 五类来源、各分区数据归属同一租户，并用 `source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=tenant_renewal_reminder` 定位续费提醒通知，再导出同筛选 CSV 断言租户、来源、状态、事件类型、标题和 payload，并确认启动日志暴露该路由。

SaaS 总后台经营指标已接入 `GET /dashboard/saasAdmin/businessMetrics` 和页面“经营指标”：平台管理员可按 `tenantLimit`、`expiringDays`、`highUsageRatio` 和 `billingLimit` 基于全平台租户、风险看板和最近续费账单估算 MRR、ARR、ARPA、风险收入、即将到期收入、已到期收入、未知价格租户数、近期账单金额和套餐收入分布；也可通过 `GET /dashboard/saasAdmin/export?type=businessMetrics` 导出包含筛选、汇总、套餐估算收入和最近账单行的经营指标 CSV；普通租户访问和导出返回 `403`。smoke 会在续费任务应用并写入账单事件后读取该接口并导出 CSV，断言最近续费金额进入套餐估算 MRR/ARR、近期账单汇总、套餐行和启动日志。

SaaS 总后台经营趋势已接入 `GET /dashboard/saasAdmin/businessTrends` 和页面“经营趋势”：平台管理员可按 `months`、`billingLimit` 和 `taskLimit` 查看最近月份账单趋势、套餐流水分布和续费任务漏斗；也可通过 `GET /dashboard/saasAdmin/export?type=businessTrends` 导出包含筛选、趋势汇总、月度流水、套餐流水和续费任务漏斗的经营趋势 CSV；普通租户访问和导出返回 `403`。smoke 会在续费任务应用并写入账单事件后读取该接口并导出 CSV，断言最近账单进入月度趋势、scale 套餐流水和续费任务漏斗，并确认启动日志暴露该路由。

SaaS 总后台续费预测已接入 `GET /dashboard/saasAdmin/renewalForecast` 和页面“续费预测”：平台管理员可按 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus` 查看未来到期租户的预测续费金额、到期桶、未知价格租户、客户成功负责人、负责人工作台和续费任务跟进状态；负责人工作台会按当前筛选聚合负责人名下的预测金额、到期窗口、未知价格、任务状态、下次跟进和 Top 租户；普通租户访问返回 `403`。smoke 会在续费任务应用并写入账单事件后读取该接口，断言目标租户进入预测清单、最近续费价格进入预测收入、负责人聚合命中目标租户并回显最新续费任务，并用目标租户自身的 bucket、定价状态、套餐、负责人和最新任务状态再读取筛选后的预测接口，确认负责人聚合也同步收窄。

SaaS 总后台续费预测任务已接入 `POST/PUT /dashboard/saasAdmin/renewalForecastTasks` 和页面“续费预测任务”：平台管理员可按续费预测当前筛选批量生成 `tenant_renewal` 运营任务，筛选条件包含到期桶、定价状态、套餐、负责人和最新任务状态；金额默认取预测续费金额，支持固定到期时间或按月顺延、订单前缀、备注和 `forceCreate`；默认跳过已有待处理续费任务的租户，普通租户访问返回 `403`。smoke 会在续费预测后调用该接口，断言目标租户生成待处理续费任务、订单号、金额和启动日志。

SaaS 总后台续费预测分派已接入 `POST/PUT /dashboard/saasAdmin/renewalForecastAssign` 和页面“续费预测分派”：平台管理员可按续费预测当前筛选批量写入风险跟进，筛选条件复用 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`；请求体支持 `owner/assignOwner`、`status=pending/contacted/renewal_pending`、`nextFollowUpAt` 和 `remark`，默认状态为 `renewal_pending`，普通租户访问返回 `403`。smoke 会先按目标租户筛选预测清单，再调用该接口，断言目标租户写入负责人、续费跟进状态、下次跟进时间和备注，并在续费预测 CSV 中回显最新跟进状态。

SaaS 总后台续费预测提醒已接入 `POST/PUT /dashboard/saasAdmin/renewalForecastNotifications` 和页面“续费预测提醒”：平台管理员可按续费预测当前筛选批量生成 `tenant_renewal_reminder` 通知 outbox，筛选条件复用 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`；请求体支持 `channel=webhook`、`reminderDays`、`maxAttempts`、`remark` 和 `forceCreate`，默认同一租户、同一到期日、同一通道去重，普通租户访问返回 `403`。smoke 会在分派预测客户并创建预测续费任务后按 `owner=forecast-csm` 生成提醒，断言目标租户进入 pending 通知、metric/alertType/message/notificationKey 和启动日志。

SaaS 总后台续费预测已接入 CSV 导出：`GET /dashboard/saasAdmin/export?type=renewalForecast` 复用 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`，导出到期租户的套餐、到期桶、剩余天数、预测续费金额、估算 MRR、是否已定价、最近续费账单、任务计数、最新续费任务状态、负责人和最新风险跟进状态；`GET /dashboard/saasAdmin/export?type=renewalForecastOwners` 复用同一筛选导出负责人聚合 CSV，包含负责人、租户数、定价/未知价格租户数、预测续费金额、估算 MRR、到期窗口分布、任务状态分布、最早下次跟进和 Top 租户；页面“数据导出”区域提供“导出续费预测 CSV”和“导出续费预测负责人 CSV”。smoke 会断言普通租户导出返回 `403`，平台管理员在分派预测客户并创建续费预测任务后导出两份 CSV，并校验目标租户、预测金额、定价状态、最近账单、任务计数、分派负责人、最新 `renewal_pending` 跟进状态、最新 `pending` 任务状态，以及负责人 CSV 的预测金额、待处理任务、下次跟进和 Top 租户。

SaaS 总后台客户成功队列已接入 `GET /dashboard/saasAdmin/customerSuccess` 和页面“客户成功队列”：平台管理员可用 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority` 聚合租户风险、风险跟进、账单跟进、运营任务、失败/耗尽通知，返回 `summary`、队列项优先级、健康分、负责人、到期状态、原因和下一步动作；普通租户访问返回 `403`。smoke 会在初始风险、打开告警和失败通知写入后读取该接口，并确认启动日志暴露该路由。

客户成功队列已继续补齐页面筛选和 CSV 导出：页面提供优先级、负责人和租户上限筛选；`GET /dashboard/saasAdmin/export?type=customerSuccess` 复用 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority`，导出优先级、健康分、负责人、到期状态、原因、下一步动作、风险、账单跟进、运营任务和失败通知信号。smoke 会断言页面入口、普通租户导出 `403` 和目标租户 CSV 字段。

客户成功队列已补齐负责人工作台：`GET /dashboard/saasAdmin/customerSuccessOwners` 复用客户成功队列筛选条件，但内部用完整队列上限聚合负责人，避免明细 `limit` 截断影响统计；响应返回负责人租户数、优先级分布、逾期/7 天内/阻断、账单跟进、可处理运营任务、失败/耗尽通知、最高/平均健康分、最早下次跟进时间和 Top 租户。页面新增“客户成功负责人工作台”和“导出客户成功负责人 CSV”，`GET /dashboard/saasAdmin/export?type=customerSuccessOwners` 复用同一筛选导出，普通租户访问和导出均返回 `403`。smoke 会在批量分派后读取 owner 汇总，断言 `smoke-csm`、目标租户、通知信号和启动日志。

客户成功队列已继续接入批量分派：`POST/PUT /dashboard/saasAdmin/customerSuccessAssign` 复用客户成功队列筛选条件，平台管理员可提交负责人、打开状态、下次跟进时间和备注；接口会为命中的租户逐条追加风险跟进操作日志，从而让队列负责人、风险跟进任务和负责人工作台同步回显。页面新增“客户成功批量分派”，普通租户访问返回 `403`，smoke 会验证目标租户分派结果、操作 ID 和启动日志。

客户成功队列已进一步接入续费任务生成：`POST/PUT /dashboard/saasAdmin/customerSuccessRenewalTasks` 复用客户成功队列筛选条件，平台管理员可按队列批量生成 `tenant_renewal` 运营任务；默认沿用租户当前套餐、按当前到期日顺延 12 个月、跳过已有待处理续费任务的租户，并返回创建数、待应用/阻断数和跳过原因。页面新增“客户成功续费任务”，smoke 会验证普通租户 `403`、平台管理员生成目标租户续费任务、任务 request/preview 和启动日志。

客户成功队列已接入续费提醒 outbox：`POST/PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications` 复用客户成功队列筛选条件，平台管理员可批量生成 `tenant_renewal_reminder` 通知，写入现有 `mochat_go_saas_alert_notifications`；默认按租户、到期日和通道去重，`forceCreate=true` 可重置为 `pending` 重新投递，并写入 `tenant.renewal.notify` 操作日志。页面新增“客户成功续费提醒”，smoke 会验证普通租户 `403`、平台管理员生成目标租户 pending 通知、metric/alertType/message 和启动日志。

套餐同步任务已补齐批量应用闭环：`POST/PUT /dashboard/saasAdmin/packageSyncTaskBulkApply` 可按任务筛选批量应用 `package_sync` 任务，平台管理员可以在统一运营任务中心一键处理当前筛选下的套餐同步任务；服务端会逐条重新计算当前套餐、租户快照和超额风险，成功写入租户套餐快照和 26 项用量额度并置为 `applied`，仍超额且未允许超额的任务置为 `blocked`，坏请求或执行失败置为 `failed`，已应用和已取消任务会被跳过。smoke 会创建第二个套餐同步任务后通过批量接口应用，并断言任务结果、租户套餐快照、任务 `applied_at`、普通租户 `403` 和启动日志。

续费任务已补齐批量应用闭环：`POST/PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply` 可按任务筛选批量应用 `tenant_renewal` 任务，平台管理员可以在统一运营任务中心一键处理当前筛选下的续费任务；服务端会逐条重新读取当前租户和套餐状态，成功写入续费账单并置为 `applied`，预演阻断置为 `blocked`，坏请求或执行失败置为 `failed`，响应返回应用、阻断、失败和跳过数量。smoke 会在客户成功队列生成续费任务后立即批量应用，并断言目标租户任务结果、账单事件和启动日志。

平台开户任务已补齐批量应用闭环：`POST/PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply` 可按任务筛选批量应用 `tenant_provision` 任务，平台管理员可以在统一运营任务中心一键处理当前筛选下的开户链接任务；服务端会逐条复核套餐状态，成功复用开户链接逻辑写入租户、管理员、角色授权、套餐快照、用量计数、开通记录和任务结果，坏请求或执行失败置为 `failed`，已应用和已取消任务会被跳过，响应返回应用、阻断、失败和跳过数量。smoke 会创建第二个平台开户任务后通过批量接口应用，并断言任务脱敏响应、新租户、管理员、套餐快照、开通记录、任务 `applied_at` 和启动日志。

SaaS 总后台页面已补充统一“运营任务中心”：页面可按任务类型、状态、租户 ID 和套餐筛选套餐同步、平台开户和续费任务；`GET /dashboard/saasAdmin/tasks` 会额外返回不受列表 `limit` 影响的 `summary` 和 `returnedCount`，页面标题展示任务总数、待应用、阻断、失败、已应用和可处理数量；`GET /dashboard/saasAdmin/taskOwners` 复用同一筛选按操作人聚合任务负责人负载，返回负责人任务量、状态分布、类型分布和最近任务；`GET /dashboard/saasAdmin/taskSla` 复用同一筛选按 SLA 阈值聚合活跃任务时效，返回正常、预警、逾期、负责人聚合和最严重任务；`POST/PUT /dashboard/saasAdmin/taskSlaNotifications` 复用同一筛选把预警/逾期任务生成 webhook pending 通知，并可通过通知列表、通知 CSV、通知重试和生命周期审计继续跟进；smoke 会通过 `GET /dashboard/saasAdmin/taskSla?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10` 回看 blocked 活跃任务，通过 `POST /dashboard/saasAdmin/taskSlaNotifications?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10` 生成 `admin_task_sla_reminder` 通知，通过 `GET /dashboard/saasAdmin/tasks?taskType=all&status=applied&limit=50` 回看三类已应用任务，并通过 `GET /dashboard/saasAdmin/taskOwners?taskType=all&status=applied&limit=20` 回看平台操作人负责人聚合，断言任务汇总、返回数量、状态分布和类型分布，并继续验证平台开户任务不泄漏明文密码或 `adminPasswordHash`。

SaaS 总后台告警和通知列表已补充筛选全量汇总：`GET /dashboard/saasAdmin/alerts` 返回 `summary` 和 `returnedCount`，汇总告警总数、打开/已解决、严重级别、命中指标数和租户数；`GET /dashboard/saasAdmin/notifications` 返回通知总数、待发/失败/已送达/耗尽/可重试数量、命中租户数和通道数。页面标题会展示命中总量、当前显示和关键状态分布；smoke 会在平台管理员和普通租户管理员视角同时断言告警/通知的 `summary`、`returnedCount` 和租户隔离口径。

统一运营任务中心已接入任务 CSV 与任务 SLA CSV 导出：`GET /dashboard/saasAdmin/export?type=tasks` 复用 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选，输出任务请求、预览、结果和应用时间；`GET /dashboard/saasAdmin/export?type=taskSla` 额外复用 `warningHours` 和 `overdueHours` 阈值，输出活跃任务负责人、SLA 状态、任务年龄和超时小时数；平台开户任务请求在 CSV 中继续去除明文密码和 `adminPasswordHash`。

统一运营任务中心已支持取消和重置未完成任务：`POST/PUT /dashboard/saasAdmin/taskCancel` 可把 `pending/blocked/failed` 任务更新为 `canceled`，页面行内提供“取消”操作；`POST/PUT /dashboard/saasAdmin/taskBulkReset` 可按当前任务筛选批量把 `failed/blocked` 任务重置为 `pending`，跳过待应用、已应用和已取消任务；`POST/PUT /dashboard/saasAdmin/taskReset` 可把单条 `failed/blocked` 任务重置为 `pending`，清空失败结果和错误后重新应用；已应用任务不可取消或重置，已取消任务不可再次应用。任务创建、应用、阻断、取消、批量取消、批量重置和重置会追加 `admin_task` 操作记录，其中单条取消使用 `saas.admin.task.cancel`，批量取消使用 `saas.admin.task.bulk_cancel`，批量重置使用 `saas.admin.task.bulk_reset`，重置使用 `saas.admin.task.reset`；页面行内“追溯”会复用操作记录筛选，按 `targetType=admin_task` 和任务 ID 查回任务全过程。smoke 会校验取消、批量重置与重置审计记录里的 `before.status` 和 `after.status`，确保操作记录保留前后状态。

SaaS 总后台操作记录表已支持展开 before/after JSON 变更详情；平台管理员从运营任务中心点“追溯”后，可以在操作记录列表直接核对任务创建、阻断、应用、取消、批量取消和批量重置的前后状态。操作记录接口会额外返回 `summary.operationCount`、`tenantCount`、`actorUserCount`、`actionCount` 和 `targetTypeCount`，页面标题展示当前筛选全量命中规模，列表仍按 `limit` 展示。

SaaS 总后台账单对账已接入 `GET /dashboard/saasAdmin/billingReconciliation`、`POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUp`、`GET /dashboard/saasAdmin/billingReconciliationFollowUps` 和 `GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners`。对账接口只读，不新增迁移；平台管理员可按租户、账单类型、套餐、关键字和 `mismatchOnly=1` 过滤续费账单，并和当前 `mochat_go_saas_tenant_packages` 比对，返回正常、异常、缺当前套餐、套餐未启用、套餐不一致和当前到期未覆盖账单到期的统计。跟进入口只对指定 `billingEventId` 追加 `billing.reconciliation.follow_up` / `billing_event` 操作日志，记录状态、负责人、下次跟进时间和备注，不改写账单或套餐事实；任务列表和负责人工作台从每个 `billing_event` 最新跟进日志派生，支持状态、负责人、到期状态和关键字筛选，不新增数据库表。内置页面新增“账单对账”表格、“只看对账异常”开关、“跟进”按钮、“账单跟进任务”、“账单跟进负责人工作台”和“导出对账 CSV”，`GET /dashboard/saasAdmin/export?type=billingReconciliation` 可复用相同筛选条件导出对账结果，smoke 会覆盖正常续费账单、模拟漂移账单、异常跟进操作日志、账单跟进任务、负责人汇总和异常对账 CSV。

账单跟进任务已继续接入 `POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose` 和页面“批量关闭账单跟进”按钮；接口不新增表，按现有任务筛选条件找到未关闭账单事件并追加新的跟进操作日志，适合平台财务或客户成功批量收口已处理的异常对账任务。

账单跟进任务 CSV 已继续接入 `GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUps` 和页面“导出账单跟进 CSV”按钮；接口不新增表，直接从最新账单跟进日志派生任务导出，便于财务或客户成功按负责人、关闭状态和订单号线下核对。

负责人汇总 CSV 已继续接入 `GET /dashboard/saasAdmin/export?type=riskFollowUpOwners`、`GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners` 和页面“导出风险负责人 CSV”“导出账单负责人 CSV”按钮；接口不新增表，继续从最新跟进日志派生负责人维度的任务量、状态和到期分布。

运营待办认领已补齐关闭和最新状态闭环：`GET /dashboard/saasAdmin/operationQueueAssignments` 支持 `currentOnly=true`，先按来源和对象去重再执行负责人、状态、到期和关键字筛选；`POST/PUT /dashboard/saasAdmin/operationQueueAssignmentClose` 以当前认领 `operationId` 做并发保护，可将认领关闭为 `resolved` 或 `ignored`，清空下次跟进时间并写入 `saas.admin.operation_queue.assignment_close` 前后状态审计，旧操作 ID 返回 `409`。页面“认领视图”默认只看当前认领，认领行提供“完成/忽略”；关闭后该认领不再覆盖运营待办负责人。认领到期提醒也强制只扫描最新认领，因此历史负责人、已关闭认领和已被重新分派的旧记录不会继续收到催办。smoke 会执行“逾期认领 -> 生成提醒 -> 完成认领 -> 当前关闭视图回看 -> 再次催办命中 0 -> 操作日志回看”的完整链路，并断言普通租户调用关闭接口返回 `403`。

运营待办认领提醒已接入自动调度：设置 `MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON=1` 后，task runner 启动 `cron-saas-operation-queue-assignment-reminder`，默认每小时分别扫描当前 `overdue` 和 `due_soon` 认领，通过与手动接口相同的共享服务生成 `operation_queue_assignment_reminder` outbox。自动任务使用系统操作者 `actorUserId=0` 和平台租户写入 `saas.admin.operation_queue.assignment_notify`，同一 `operationId + dueState + channel` 幂等去重；旧负责人、未来认领和已关闭认领不会生成通知。任务状态、运行实例和 periodic tick 会写入三张后台任务表并暴露在 `/compat/status`。独立 smoke 会造当前逾期、当前 7 天内、已被覆盖的旧逾期和已关闭四类数据，验证只生成两条 outbox，并在进程重启后保持幂等。

通知健康认领已接入自动恢复调度：设置 `MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON=1` 后，task runner 启动 `cron-saas-notification-health-recovery`，默认每 15 分钟按 24 小时窗口和 15 分钟积压阈值复查当前通知健康认领。自动任务复用手动恢复服务，只关闭状态为 `healthy`、存在投递尝试且至少成功送达一次的认领，并以 `actorUserId=0`、平台租户写入 `saas.admin.operation_queue.assignment_close`；仍异常、无数据和只有抑制通知的租户保持打开，进程重启不会重复关闭。独立 smoke 会从空库执行当前 49 个迁移，构造已恢复、仍异常、无数据、只有 suppressed 四类租户，并验证 `/compat/status`、periodic tick、关闭上下文和重启幂等。standalone compose `app` 服务已显式透传总后台、审批门禁、审批 SLA 提醒、平台健康扫描、租户账单门户、认领提醒、通知健康自动恢复、订阅校准、支付回调、催缴和支付结算同步变量，并把 `0033` 至 `0049` 挂入全新 MySQL 初始化顺序；compose app smoke 会验证配置实际进入容器、最新表和索引存在、任务启动即跑成功。

高风险审批治理已接入 `0047_saas_admin_approval_governance`。五类审批策略从固定代码配置升级为数据库策略，可分别维护启用状态、退款金额阈值、会签票数、SLA、提醒间隔和审批有效期；申请会保存完整策略快照，后续调整不改写在途审批。新增 `platform.approvals.manage` 后，审批治理成员可维护策略、审批委托和手动 SLA 提醒。

会签决定使用不可变逐票账本：每个实际复核人只能投一票，任一驳回立即结束，批准票达到申请快照的法定人数后才进入已批准；执行前再次校验法定票数。有效委托允许受托人代为复核，同时保留委托来源；委托人必须仍持有复核权，发起人和经其产生的委托关系都不能绕过自批限制。退款直写是否返回 `428` 由策略启用状态和金额阈值共同决定，其他四类动作由各自策略开关决定。

审批 SLA 提醒同时提供 `POST/PUT /dashboard/saasAdmin/approvalReminders` 和 `cron-saas-admin-approval-reminder`。两者复用同一事务，在更新提醒次数和下次提醒时间的同时写入 `approval_sla_reminder` 通知 outbox 与平台审计，并按审批 ID 和提醒次数幂等。总后台新增策略编辑、委托维护、会签票数、SLA 状态、提醒次数、审批事件和逐票决定明细；真实 JWT 浏览器验收确认桌面和移动端无页面级横向溢出，宽表只在内部容器滚动。

平台健康中心已接入 `0048_saas_admin_system_health`。实时检查覆盖 MySQL/Redis、迁移版本、后台任务和执行、通知耗尽/重试/积压、审批 SLA、运营任务和结算同步。手动与 cron 扫描会持久化快照，按检查键聚合 `open/acknowledged/resolved` 事故；新增、重开或升级事故生成 `system_health_incident` outbox，重复扫描不重复通知，恢复检查自动结案。

总后台新增健康概览、12 项检查明细、事故筛选、负责人/处置备注、认领/分派/解决/重开和扫描历史。`platform.system.read/manage` 使权限目录扩展为 17 项；真实 MariaDB/Redis smoke 验证 9 类故障注入、通知去重、乐观锁、越权拒绝、未恢复重开和修复后自动恢复。真实 JWT 浏览器验收确认 1440px 桌面与 390px 移动端无页面级横向溢出，扫描可真实落库，登录后控制台 0 错误。

租户服务账号与 API Key 已接入 `0049_saas_service_accounts`。总后台使用 `platform.integrations.read/manage` 分离查看和写入权限，可按租户维护 Scope、IP/CIDR 白名单、账号状态与过期时间，并对 Key 执行带宽限期轮换或立即吊销；权限目录因此扩展为 19 项。服务账号只能访问自身 Scope 允许的 `/api/saas/v1/whoami`、`usage` 和 `alerts`，所有业务查询强制使用 Key 绑定的租户 ID。

API Key 使用 `mch_live_<12 hex>_<43 base64url>` 格式，明文只在创建或轮换响应中显示一次。数据库只保存使用 `SimpleJWTSecret` 作为 pepper 的 HMAC-SHA256 摘要、非敏感前缀和后四位；生产环境必须稳定保管 `MOCHAT_SIMPLE_JWT_SECRET`，变更会同时让现有 dashboard JWT 和 API Key 失效。专项 smoke 已覆盖摘要存储、错误 Key 统一拒绝、Scope、IP 白名单、过期、轮换宽限、退役、吊销、使用追踪、RBAC、租户隔离和审计。

指纹复核：

```bash
env -u GOROOT ./scripts/source_fingerprint.py --check docs/phases/phase-pre0-standalone/evidence/latest/source-fingerprint.json
```

生产候选复核当前会失败，这是预期结果，直到真实生产证据齐备：

```bash
DOCKER_CONFIG=/tmp/mochat-go-docker-config \
env -u GOROOT \
MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 \
./scripts/production_candidate_gate.sh
```

## 生产证据

生产候选需要 6 类证据：

- MySQL 5.7 amd64 真实容器门禁。
- 真实企业微信账号联调。
- 真实微信开放平台联调。
- 真实 SaaS 多租户数据回归。
- 生产前端浏览器回归。
- 稳定性记录。

先生成待填写清单：

```bash
./scripts/production_evidence_doctor.sh
cp docs/phases/phase-pre0-standalone/evidence/production/readiness.env.todo docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local
```

填写 `readiness.env.local` 后做预检：

```bash
MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE=docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local \
./scripts/production_evidence_env_preflight.sh
```

预检通过后，在受控终端采集并收拢：

```bash
set -a
. docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local
set +a
MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh
./scripts/collect_production_evidence_pack.sh
```

## 稳定性证据口径

默认不启动新的 24 小时持续运行。稳定性证据推荐使用目标环境已有的短稳回归和外部监控记录：

```bash
MOCHAT_STABILITY_MONITOR_EVIDENCE=@docs/phases/phase-pre0-standalone/evidence/production/stability-monitor-run.txt \
MOCHAT_STABILITY_HEALTH_EVIDENCE=@docs/phases/phase-pre0-standalone/evidence/production/stability-health-run.txt \
MOCHAT_STABILITY_RESOURCE_EVIDENCE=@docs/phases/phase-pre0-standalone/evidence/production/stability-resource-run.txt \
MOCHAT_STABILITY_TIME_RANGE="2026-07-09 10:00:00 CST 至 2026-07-09 10:30:00 CST" \
MOCHAT_STABILITY_LOG_REF="监控面板/日志/工单引用" \
./scripts/capture_stability_evidence.sh
```

监控摘要必须包含路由覆盖和 `224`，健康证据必须包含 `/readyz`、health、健康、探测或 `200`，资源证据必须包含 RSS、memory、CPU、连接、内存或资源。旧 `soak.ndjson` 仍可作为 legacy 输入导入，但不是默认验收动作。

## 0064 候选增量（历史）

- 当前迁移基线为 `64/0064_saas_audit_anchor_remote_immutability`。0063 的 HMAC-SHA256 签名检查点保持不变；0064 增加 S3 兼容 Object Lock 异地不可变存储，记录 provider、bucket、object key、version ID、ETag、SHA-256、字节数、保留模式和保留到期时间。
- 远端探测要求 bucket 已存在且已启用 Object Lock；可选 `compliance/governance` 模式，默认保留 3650 天，配置范围为 1–36500 天。生产应使用与业务数据库分离、限制删除权限的对象存储账号。
- 创建会以确定性日期路径上传证据，并精确校验远端字节、摘要、大小、版本和保留期。远端版本列举会发现数据库回退后的孤儿证据；历史本地检查点会按时间顺序分批回填远端，不只处理当前链头。
- `GET /dashboard/saasAdmin/auditAnchors`、`POST/PUT /dashboard/saasAdmin/auditAnchor`、维护命令和 `cron-saas-admin-audit-anchor` 共用同一领域服务。总后台展示远端 provider/bucket、保留策略、版本 ID、远端验证时间、失败数和孤儿数；系统健康新增 `audit_anchor_remote_store` 关键探针。
- 真实 MinIO 以 `--with-lock` 创建 bucket，并用 7 天 compliance 保留完成专项 smoke：受保护版本删除被拒绝，远端对象缺失和孤儿版本均能被发现，维护命令和历史回填同时通过。
- `scripts/smoke_schema_migrate.sh` 已覆盖 64 个版本的空库 apply、checksum、status、重复 apply、baseline、legacy 升级、0064 down/up 与完整重放；MySQL 5.7 静态门禁覆盖 127 个 schema/迁移文件。本机 arm64 不伪造 MySQL 5.7 amd64 实容器证据。
- 当前功能矩阵为 smoke `95/95`、manifest `224/224`、运行时唯一路径 `582/582`、路由条目 `721`、SaaS/总后台路径 `174`、前端 dist API `209/209`。合规清单保持 158 项，可恢复擦除保持 161 步。
- `docs/phases/phase-pre0-standalone/evidence/latest/results.jsonl` 于 `2026-07-13 04:46:47 CST` 至 `05:23:34 CST` 按最终源码完整重建，`14/14` 返回 0。源码与验收指纹为 `7781d0d8d5f16aa110af7f5a302591a0b84eb226b6ecbf8f80d5fec7980fef34`，纳入 818 个文件。
- 真实 Compose 预览数据库账本为 `64/0064_saas_audit_anchor_remote_immutability`，最终镜像为 `sha256:a7395de448d46fdf4073a17fa70502afe76d3caf6a618a72066ae273a0f1b35e`。运行日志确认 `source=build` 且内置指纹一致；当前 11 个检查点均已远端导出并校验通过，远端失败、待传、本地孤儿和远端孤儿都是 0，Object Lock 健康探针返回“连接正常”。
- 迁移前备份为 `/tmp/mochat-preview-before-0064-20260713-042217.sql`，大小 769749 字节，SHA-256 为 `e83af4f6383d160658910e68e59de16c34de2ec2d80a6b0df7248c11edb636ea`，文件权限为 `0600`。Playwright 已在 `1440x1000` 和 `390x844` 验证远端锚点页面，授权后控制台 0 error/0 warning，移动端无页面级横向溢出，宽表仅在内部滚动。
- 生产证据检查仍明确缺少 6 类外部证据：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端浏览器和目标环境短稳/监控记录。候选检查继续以这些外部证据为准，本轮未启动 24 小时运行。

## 当前候选增量（0067）

- 当前迁移基线为 `67/0067_wecom_credential_encryption`。0067 为企业和企微应用凭据增加 AES-256-GCM 密文与 Key ID，主服务、自动标签、会话存档同步、维护命令、系统健康和总后台统一接入专用密钥环。
- 总后台新增企微凭据保护概览与轮换接口，展示专用强制加密、活动 Key、密文/旧明文/待轮换/不可用数量；读取与执行权限分离，轮换写入平台操作审计。
- 完整本地短证据包于 `2026-07-14 09:41:20 CST` 至 `10:35:42 CST` 重建，`14/14` 返回 0。smoke 纳管 `96/96`、manifest 路由 `224/224`、runtime unique paths `585/585`、runtime route entries `726`、前端 dist API `209/209`。
- 最终源码指纹为 `dece0dcd6fedb810558c5577390db8f2b870be5114958bd4dbd3109bc211241b`，纳入 845 个文件。预览镜像 `sha256:41b9a9ef0a78117ef117c08d97808c546f615319d8f1b9fbd20a19922917a134` 健康，启动日志确认 `source=build` 且指纹一致。
- 预览数据库已清空企业与应用旧明文字段；当前企业密文行 1、应用密文行 0，企微凭据保护 healthy、待轮换 0、不可用 0。系统健康 `28/30`，保留逾期审批 critical 与备份新鲜度 warning。
- Playwright 已完成桌面和移动端企微凭据治理区域回归；生产证据 doctor 与 current 包已刷新到最终指纹。六项外部证据仍缺失，本轮未启动 24 小时运行，因此该候选只能标记为“本地收口完成、待生产补证”。

## 当前候选增量（0068）

- 当前迁移基线为 `68/0068_wechat_open_credential_encryption`。第三方平台 Ticket 与公众号授权凭据已改为 AES-256-GCM 密文主存储，并由独立微信开放平台密钥环提供活动/历史 Key、专用密钥强制、存量轮换和缺 Key 安全失败能力。
- Ticket 接收、授权回跳、取消授权与消息回调、存储读取、主服务、维护命令和系统健康已统一接入；总后台新增微信开放平台凭据保护概览与轮换接口，读取/执行权限分离并保留无敏感值的操作审计。
- 完整本地短证据包于 `2026-07-14 12:27:37 CST` 至 `13:14:49 CST` 重建，`14/14` 返回 0。smoke 纳管 `96/96`、manifest 路由 `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。
- 最终源码指纹为 `0cf803a1e598446f8e4af86eee5b3a1755f428eda70e915df38ecd23d70d2c1c`，纳入 855 个文件。预览镜像 `sha256:ad71ef9a03c4f7d89c3209bc67283a5b3540810b2c2eb012498714e7e756fe09` 健康，启动日志确认 `source=build`、内置指纹一致、专用强制加密启用。
- 预览数据库为 `68/0068`，运行时路由 706 条；微信开放平台凭据保护 healthy、待轮换 0、不可用 0。系统健康为 `29/31`，新凭据探针 healthy，仍保留逾期审批和备份过期两项 critical。
- Playwright 已完成 `1440x1000` 与 `390x844` 总后台回归，页面无横向溢出，授权后控制台 0 error/0 warning。生产证据 doctor 与 current 包已刷新到最终指纹；六项外部证据仍缺失，本轮未启动 24 小时运行，因此候选状态仍为“本地收口完成、待生产补证”。

## 当前候选增量（0068 + 自动备份调度）

- 灾备中心已显示自动调度开关、扫描间隔和启动检查；平台健康新增 critical 检查 `backup_automation`，用于防止“策略已启用但备份调度器未启动”被误报为健康。
- 预览环境已启用 5 分钟扫描与 run-on-start，真实自动备份 `bkp_20260714T053833Z_1cd7a563e3da` 完成 AES 加密、本地校验和 S3 副本校验，迁移账本 `68/0068`、数据表 190 张、工件 154985 字节。
- 超 SLA 的演示审批已通过受控 API 撤回，事件与操作审计完整落库。最新手动健康扫描为 `32/32 healthy`，`approval_sla_overdue`、`backup_automation` 和 `backup_freshness` 均 healthy，当前活动问题为 0。
- 最终本地证据包的 14 条命令记录均返回 0，包含 `core/saas/workers/cron/frontend/mysql57` 六个套件、路由 `224/224`、前端 dist API `209/209` 和 smoke `96/96`。MySQL 5.7 的 135 个 schema 静态检查通过，ARM 上跳过的实容器未被当作 amd64 证据。
- 当前指纹为 `357041312a9c86b48cc2e8830cb87f7a419ca275d1e3c925991d3fe6d5a608c0`，纳入 855 个文件；镜像 `sha256:88ad810e68c14d07b43c3f0a626361256c3a80b22309fa87e664ae0f1b5d14f5` 与 App/MariaDB/Redis 当前 healthy。桌面和移动浏览器回归无溢出、无重叠、无控制台错误或警告。
- 当前候选只剩 6 类目标生产环境的外部证据。`docs/phases/phase-pre0-standalone/evidence/production/current/index.md` 已记录这 6 项缺失和严格门禁失败；本轮不要求且未执行 24 小时 run。

## 当前候选增量（0068 + 发布证据远端工件复核）

- 发布证据不再把提交的 URL、SHA-256 和大小当作已验证事实。保存 `passed` 时必须由服务端流式下载 HTTPS 工件并精确核对摘要与字节数；存储层拒绝缺少成功校验结果的直接写入。
- 创建候选时并行重验六项工件，并在数据库事务中锁定证据版本和工件元数据，避免校验后替换。候选快照包含每项实际校验结果；`passedCount` 表示远端复核通过数，而非元数据填写数。
- `metadataReady` 只表示六项元数据完整。只有当前权威源码指纹下最近候选为 `ready` 且六项远端复核通过，发布准备 API 才返回 `ready=true`。当前预览六项外部证据缺失、候选为 0，因此两项状态均为 false。
- 校验器默认只允许 HTTPS，启用私网/云元数据阻断、DNS 绑定、禁用环境代理和同源重定向限制，超时 30 秒、单工件最大 64 MiB。显式 CIDR 例外和自有 CA 只用于受控内部证据库；当前预览均未配置。
- 真实数据库专项通过“六工件保存 -> 篡改一项 -> 候选 `5/6 blocked` -> 恢复 -> 候选 `6/6 ready`”全链路。最终短证据包为 `14/14` 返回 0，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。
- 当前源码指纹为 `fd9e06ae9e907309a9a44ec26cae9f4de719bd984e12214017a76d73466f1285`，纳入 857 个文件；预览镜像为 `sha256:42a3fbde4305b42003edbed9a3565140d0f7201fa639895abad359e86e120bb6`，迁移账本仍为 `68/0068`。
- Playwright 桌面、移动和移动弹窗回归通过，页面无横向溢出或重叠，干净会话控制台 0 error/0 warning。当前审计锚点已恢复为 `26/26` 远端验证通过；健康中心保留两次已恢复的 MinIO DNS 后台执行失败，最新扫描为 0 critical、1 warning。
- 生产候选仍被六项真实外部证据阻断。本轮不要求且未执行 24 小时运行。

## 发布前检查

- [x] 本地 Go 单测、命令构建、独立性、72 版迁移、发布工件远端复核、审批策略双人会签、备份保留清理 Saga、全部六个短验收套件、浏览器和静态审计通过。
- [x] 本地 Compose 预览运行 0072，源码指纹与镜像内置指纹一致，`/readyz` 返回 200。
- [ ] 目标部署环境 `.env.local` 已替换全部占位值，并通过生产配置预检。
- [ ] MySQL 5.7 amd64 真实容器证据已导入。
- [ ] 真实企业微信和微信开放平台联调证据已导入。
- [ ] 两个以上真实 SaaS 租户业务数据回归证据已导入。
- [ ] 生产域名前端浏览器回归证据已导入。
- [ ] 目标环境短稳与外部监控证据已导入。
- [ ] 严格生产证据报告为 `evidence_ok=true`，目标完成度报告为 `goal_complete=true`。

## 回滚和备份

停止服务：

```bash
docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app down
```

数据库和上传文件使用 Docker volume 保存，volume 名由 compose project 决定。生产环境应使用平台级快照或外部数据库备份，不要只依赖本地 Docker volume。

## 本轮候选门禁增量（2026-07-15）

- 发布候选现在与现行六项证据逐字段绑定，不再只信任历史 `ready` 记录。候选快照与现行证据任一项不一致时，原始状态保留为 `ready` 供审计，有效状态改为 `stale`，发布摘要强制 `ready=false` 并返回变化项。
- 将证据从 `passed` 改为失败后再恢复，不会自动恢复旧候选；必须重新下载并复核六项远端工件、生成新候选，才能重新进入 `ready`。总后台已展示“证据已变化”和“需重跑门禁”。
- 专项真实数据库 smoke、完整 `scripts/test.sh`、六套短验收和前端浏览器回归均通过；最新本地证据为 `14/14`，源码指纹为 `026b3cef88d869a0e53ac6ce6c0c493d69d2102b981106808ec520ee1a084102`，预览镜像为 `sha256:f4936e44f96039fca314873a84973066e5d0a8e3f7bfa7922f056f80faab3733`。
- 当前生产候选仍不可发布：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端、目标环境短稳/外部监控六项标准证据均缺失，严格证据包按预期阻断。没有启动 24 小时运行。

## 当前候选增量（0069 + 发布候选双人审批）

- `release.candidate.gate` 已纳入高风险审批中心，默认要求 2 名不同复核人，SLA 120 分钟、提醒 30 分钟、6 小时失效；审批全局开关启用时，直接调用候选接口返回 `428`。
- 审批申请会固定发布版本、权威源码指纹和六项证据元数据。审批执行阶段重新下载并核验六项远端工件，候选创建与审批副作用标记在同一数据库事务提交，恢复逻辑不会重复创建候选。
- 总后台会在六项证据元数据完整后提交审批；当前生产证据为 `0/6`、候选为 0，发布按钮保持禁用。审批策略 API 返回 `required=true`，策略参数为 `2/120/30/6`。
- 最终短证据包为 `14/14` 返回 0，六套验收、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209` 均通过。
- 当前源码指纹为 `f1eb06656144ab104dd58853b9d171c988eda161d40d18c0ed347aefc36f0385`，文件数 859；预览镜像为 `sha256:18b757460cdba0ae2076a19c8847b2210ad5f1d7a22e2723d8d57dd1cd8a427c`，迁移账本为 `69/0069_saas_release_candidate_approval`。
- 升级前快照为 `output/backups/mochat-preview-before-0069-release-candidate-approval-20260715-104108.sql`，权限 `0600`、大小 997621 字节、SHA-256 `6024720157d26bdbffe660f5222cbcaeea769333660572e8ad2399b64ee3683e`。
- 桌面和移动端最终浏览器回归通过，控制台 0 error/0 warning；生产证据包仍缺六项真实外部证据，因此候选不可发布。没有执行 24 小时运行。

## 当前候选增量（0070 + 审批策略变更双人会签）

- `approval.policy.update` 已纳入高风险审批中心，治理策略自身强制启用且不可降级，要求 2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；直接策略写入返回 `428`。
- 审批申请固化策略载荷；执行阶段把策略更新、审批副作用标记和平台操作日志放在同一事务中提交。总后台治理行只读显示“强制启用”，所有策略保存按钮统一为“提交审批”。
- 最终短证据包 14 条命令全部返回 0，六套验收、manifest `224/224`、前端 dist API `209/209` 均通过。源码指纹为 `f5ff1082489ff19f917bfa487b333031cfbf6f0783f674775ae915f1b46c3ce5`，文件数 861。
- 预览镜像为 `sha256:0708d78ee0f92a9d7754d0c2d030e9b8613d3141bb9c87a4f9cfe8dd4b910280`，迁移账本为 `70/0070_saas_approval_policy_change_guard`，策略总数 8；启动日志内置指纹与当前源码一致。
- 升级前快照为 `output/backups/mochat-preview-before-0070-approval-policy-guard-20260715-122339.sql`，权限 `0600`、大小 1016263 字节、SHA-256 `6f3f45bfabb096452cb2a35e0ad9c3a31ebf93cd7f4fdabd10a24d2eb306d8e7`。
- 桌面与 `390x844` 浏览器回归通过，无页面级横向溢出或操作重叠，控制台 0 error/0 warning。生产候选仍被六项真实外部证据阻断；`require_24h=false`，没有执行 24 小时运行。

## 当前候选增量（0071 + 备份策略变更双人会签）

- `backup.policy.update` 已纳入高风险审批中心，默认要求 `platform.backups.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。强制审批开启时，直接策略写接口返回 `428` 且不产生数据库写入。
- 审批申请固化规范化策略载荷，执行阶段重新校验；策略更新、审批副作用操作 ID 和平台操作日志在同一事务提交。真实 MariaDB/Redis smoke 已覆盖独立申请人、双人会签、第三方执行和恢复幂等。
- 最终短证据包于 `2026-07-15 15:27:32 CST` 收口，14 条命令全部返回 0；六套验收、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209` 均通过。源码指纹为 `db9b881455a40dabe44470f036cdd33e4b6200ba83dc311fed58a511c9221b8c`，文件数 863。
- 预览镜像为 `sha256:f4424e4529b54002b52355d78428d24342bf57b581d75663f6237ec24d9a47f6`，迁移账本为 `71/0071_saas_backup_policy_change_guard`，审批策略总数 9；`/readyz` 返回 200，日志内置指纹与当前源码一致。
- 升级前快照为 `output/backups/mochat-preview-before-0071-backup-policy-guard-20260715-143642.sql`，权限 `0600`、大小 1049933 字节、SHA-256 `2e37885c98839fa22da0f705a190f9839d43f2504ed6e4e2631be7b8763f4dca`。
- 桌面与 `390x844` 浏览器回归通过，备份策略按钮显示“提交审批”，页面无整体横向溢出，移动按钮完整可见，控制台 0 error/0 warning。截图为 `output/playwright/0071-backup-policy-desktop-final.png` 和 `output/playwright/0071-backup-policy-mobile-390-final.png`。
- 生产候选仍被六项真实外部证据阻断；严格 current 包和 doctor 均按预期返回 1。`require_24h=false`，没有执行 24 小时运行。备份清理审批因跨数据库、文件系统和 S3/Object Lock 的可恢复幂等要求，保留为下一独立里程碑。

## 当前候选增量（0072 + 备份保留清理审批 Saga）

- `backup.retention.cleanup` 已纳入高风险审批中心，默认要求 `platform.backups.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。服务端在申请时冻结候选范围，直接清理返回 `428`，审批后不重新扩大删除范围。
- 清理执行按 S3 副本、本地工件、数据库台账三阶段持久化。远端失败保留后两者，成功步骤可幂等重放，租约恢复只继续未完成步骤；被冻结备份禁止校验、复制和恢复。总后台新增清理任务、进度、失败详情和重试操作，系统健康新增 `backup_retention_cleanup`。
- 真实 MariaDB/Redis/MinIO 灾备 smoke 已验证双人会签、冻结候选、远端/本地/台账删除及失败重试；72 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 和全部六套短验收通过。最终证据包为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。
- 当前源码指纹为 `5db23ab6d2310791eb375baba9b3040df9c7b0089e572560e134cfb54de54146`，文件数 867；预览镜像为 `sha256:592bc88d5292c384581640a8d6459c534e8ca5ddfd79cc0d9c4214573b8dcb89`，迁移账本为 `72/0072_saas_backup_cleanup_saga`，审批策略总数 10，`/readyz` 返回 200。
- 升级前快照为 `output/backups/mochat-preview-before-0072-backup-cleanup-saga-20260715-162328.sql`，权限 `0600`、大小 1062706 字节、SHA-256 `54eca644f01eb5474d11e593cbbb531c237af11f03c8ac1eac03edf5e3c2965c`。
- 桌面与 `390x844` 最终浏览器回归通过，清理按钮显示“提交清理审批”，页面无整体横向溢出，移动表格仅在 368px 容器内部滚动，控制台 0 error/0 warning。截图为 `output/playwright/0072-backup-cleanup-*-final.png`。
- 预览当前为 `32/33` 健康，唯一 critical 来自 24 小时窗口内 7 次真实失败执行；最新审计锚点执行成功并验证 `42/42`，没有改写历史失败。严格生产证据、严格目标审计和 skip-local 候选门禁仍因六项外部证据缺失返回 1，源码与 latest 指纹匹配通过；没有执行 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0073 + 合规导出提前删除审批 Saga）

- `compliance.export.delete` 已纳入高风险审批中心并强制启用，默认要求 `platform.compliance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。保留期内直接删除返回 `428`；法律保留或未终结的数据擦除仍引用导出时返回 `409`。
- 审批申请冻结租户、导出编号、工件路径/名称、SHA-256、字节数和保留期。执行器按本地加密工件、数据库记录两阶段持久化，失败后保留错误与尝试次数；人工重试和维护任务只继续未完成步骤，避免文件已删但台账状态丢失。
- 合规中心新增删除审批、步骤状态、审批 ID、尝试次数、错误和重试操作；系统健康会识别删除失败、积压和过期租约。真实 MariaDB/Redis smoke 已覆盖直接删除阻断、法律保留、冻结快照、双人会签、工件失败、记录保留和重试成功。
- 73 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 和全部六套短验收通过。最终证据包为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `cf3b0366e8309d3a2f1efad3a6f6f7325b0e3d75a4e9e6cf2f93c4b9660e6bf7`，文件数 871。
- 当前预览镜像为 `sha256:7047142568b9d899522dbe26c835025a64dff52ced81daf83cb3124366f2138b`，迁移账本为 `73/0073_saas_compliance_export_deletion_saga`，审批策略总数 11；`/readyz` 返回 200，日志确认内置指纹来源为 build 且与当前源码一致。
- 升级前快照为 `output/backups/mochat-preview-before-0073-compliance-export-deletion-saga-20260715-220412.sql`，权限 `0600`、大小 1117833 字节、SHA-256 `6b85c75c55d57fa41271fbfac014a8a867f19d27fff8456b2b4a2ee9f01ba6df`。
- 桌面与 `390x844` 浏览器回归通过，页面无整体横向溢出，移动宽表仅内部滚动，控制台 0 error/0 warning，总后台请求全部为 200。当前健康为 `32/33`，合规探针均正常；严格生产门禁仍被 6 类真实外部证据阻断，源码指纹匹配通过，没有执行 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0074 + 法律保留解除双人会签）

- `compliance.legal_hold.release` 已纳入高风险审批中心并强制启用，默认要求 `platform.compliance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接解除返回 `428`，治理接口不能关闭或降低双人审批门槛。
- 审批申请冻结完整法律保留快照；执行时锁行复核版本和快照。解除状态、审批效果操作 ID 与平台操作日志同事务提交，恢复执行不会重复产生业务副作用。
- 74 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 和六套短验收通过。最终证据为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `293e0c1a28b67a0a488a575b16abbacf22c8eee6758c038312ae71b12ccd70d4`，文件数 874。
- 当前预览镜像为 `sha256:9556f703a22bb0f5e6d11580f5289b6a76eac4cbcee3ab14e1bdf75fa00329ed`，迁移账本为 `74/0074_saas_compliance_legal_hold_release_guard`，审批策略总数 12；`/readyz` 返回 200，构建指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0074-legal-hold-release-20260715-230139.sql`，权限 `0600`、大小 1131065 字节、SHA-256 `0e5b94961f2f5ac9df2095f32feeb96d4751c6a66c375bea4302cb9b314966c5`。
- 桌面与 `390x844` 浏览器回归通过，活动保留单显示“提交解除审批”，策略行强制启用且审批人数为 2；页面无整体横向溢出，移动宽表仅内部滚动，干净重载无失败请求，控制台 0 error/0 warning。
- 当前健康为 `32/33`，唯一 critical 是 24 小时统计窗口内 7 次历史审计锚点失败，当前任务已连续 3 次成功。严格生产门禁仍被 6 类真实外部证据阻断，发布准备为 `0/6`、`ready=false`；未执行 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0075 + 合规生命周期策略变更双人会签）

- `compliance.policy.update` 已纳入高风险审批中心并强制启用，默认要求 `platform.compliance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接策略写入返回 `428` 且不改数据库，治理接口不能关闭或降低双人审批门槛。
- 审批申请规范化并冻结完整合规策略及当前版本；执行阶段重新校验当前版本，版本漂移会返回冲突。策略更新、审批效果操作 ID 和平台操作日志同事务提交，恢复执行不会重复产生业务副作用。
- 75 版迁移 apply/rollback/replay、全量 Go 测试、合规与审批专项、独立 Compose 和六套短验收通过。最终证据为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `c08216de5937ffc23d95f4092a76d6ed99f35fdb7f90bd9f7761f2cf378ee229`，文件数 877，MySQL 5.7 静态检查覆盖 149 个文件。
- 当前预览镜像为 `sha256:f022f983c899f4197d30019326cb55f114676221fa2a06cb6880b7c331e4d81e`，迁移账本为 `75/0075_saas_compliance_policy_change_guard`，审批策略总数 13；`/readyz` 返回 200，运行时路由 706 条，构建指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0075-compliance-policy-guard-20260716-011013.sql`，权限 `0600`、大小 1145672 字节、SHA-256 `c646b6400b2ee8dfde369d2f733af3abf4c37c6687e5f6f05bd71ef1213f74eb`。
- 桌面与 `390x844` 浏览器回归通过，合规策略保存显示“提交审批”，策略行强制启用且值为 `2/120/30/12`；页面无整体横向溢出，移动宽表仅在 368px 内部容器滚动，控制台 0 error/0 warning，未发现失败请求。
- 当前健康为 `32/33`，唯一 critical 是统计窗口内 7 次历史后台失败；审计锚点最新 4 次执行成功，45 个锚点均已远端导出并验证。严格生产门禁仍被 6 类真实外部证据阻断，源码与 latest 指纹匹配；`require_24h=false`，未执行 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0076 + 身份安全策略变更双人会签）

- `identity.policy.update` 已纳入高风险审批中心并强制启用，默认要求 `platform.identity.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接策略写入返回 `428` 且不改数据库，治理接口不能关闭或降低双人审批门槛。
- 审批申请规范化并冻结完整身份安全策略及当前版本；执行阶段重新校验当前版本，版本漂移会返回冲突。策略更新、审批效果操作 ID 和平台操作日志同事务提交，恢复执行不会重复产生业务副作用。
- 76 版迁移 apply/rollback/replay、全量 Go 测试、身份安全与审批专项、独立 Compose 和六套短验收通过。最终证据为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `147d3b51f5937ce18760102b2f30635c8aebcf13e327745189ffe0338dda27eb`，文件数 880，MySQL 5.7 静态检查覆盖 151 个文件。
- 当前预览镜像为 `sha256:0a478816f72cf1713b35a14b6b2d4330c84bf96e354cfd70d7d8ad5ce5b29585`，迁移账本为 `76/0076_saas_identity_policy_change_guard`，审批策略总数 14；`/readyz` 返回 200，运行时路由 706 条，构建指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0076-identity-policy-guard-20260716-023234.sql`，权限 `0600`、大小 1157151 字节、SHA-256 `aaa05a07fb77268fc72aeb6aa61f345acafe1c5fca06de172cfd14a944d7ec9b`。
- 桌面与 `390x844` 浏览器回归通过，身份策略保存显示“提交审批”，策略行强制启用且值为 `2/120/30/12`；页面无整体横向溢出，移动宽表仅在内部滚动，干净授权会话中 69 个请求全部为 200，控制台 0 error/0 warning。
- 当前健康为 `32/33`，唯一 critical 是统计窗口内 7 次历史后台失败；审计锚点最新 5 次执行成功，46 个锚点均已远端导出并验证。严格生产门禁仍被 6 类真实外部证据阻断，源码与 latest 指纹匹配；`require_24h=false`，未执行 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0077 + 租户停用强制双人会签）

- `tenant.disable` 现固定为高风险强制审批，要求 `platform.tenants.manage`、2 名不同复核人、240 分钟 SLA、60 分钟提醒和 24 小时失效。审批治理不能关闭该策略或把审批人数降到 2 以下；直接停用继续返回 `428`。
- 审批主流程已验证第一票后仍为 pending，第二票后才批准并允许执行；租户停用后登录被阻断。治理专项同时验证关闭和降级请求返回 `400`，策略保持 `enabled=1`、`2/240/60/24`。
- 77 版迁移 apply/rollback/replay、全量 Go 测试、审批专项、独立 Compose 和六套短验收通过。最终证据为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `096bf16c8dbf24dc9a95d69ae35c46af81c559c67f48866746ca97ca943c090b`，文件数 883，MySQL 5.7 静态检查覆盖 153 个文件。
- 当前预览镜像为 `sha256:250747044e82e7d3d2c96a76a176d8b2f2656c559ec9dda28ec09fba0e8f8480`，迁移账本为 `77/0077_saas_tenant_disable_approval_guard`，审批策略总数 14；`/readyz` 返回 200，运行时路由 706 条，构建指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0077-tenant-disable-approval-guard-20260716-035225.sql`，权限 `0600`、大小 1170212 字节、SHA-256 `64206ea8929cf196161a7d04d4d31cdbeac34bd20e7d032bcf5ccc3f8c1d8e70`。
- 桌面与 `390x844` 浏览器回归通过，策略行强制启用、审批人数下限为 2；页面无整体横向溢出，移动宽表仅在 368px 内部容器滚动，控制台 0 error/0 warning，业务请求全部为 200。
- 当前健康为 `32/33`，唯一 critical 是统计窗口内 7 次历史后台失败；47 个审计锚点均已导出并验证。严格生产候选包仍因 6 类真实外部证据缺失返回 1，源码与 latest 指纹匹配；`require_24h=false`，未执行 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0078 + 严重风险审批策略基线防降级）

- 已新增 `0078_saas_critical_approval_policy_guard`，对全部 13 条 critical 策略强制 `enabled=1`、绕过阈值为 0、会签下限为 2。退款默认已从单人改为双人；非 critical 的结算关账仍可正常配置。
- 服务端、存储层、治理接口和总后台使用同一不变式。迁移 apply/rollback/replay、退款双人主流程、严重风险降级拒绝和普通策略更新均已通过真实 MariaDB/Redis smoke。
- 最终本地证据包为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查 155 个文件；指纹为 `47d64ab13b5e31c69b5b4b3268e8389a106258615bd709cbc05356d912140fad`，文件数 886。
- 预览镜像为 `sha256:c3d4f11393e5e079225c56241cf7d87cc0d54b5888fa85f58b6719f0e342f073`，迁移账本 `78/0078`，应用健康且 `/readyz=200`。升级前备份为 `output/backups/mochat-preview-before-0078-critical-approval-policy-guard-20260716-050823.sql`，权限 `0600`，SHA-256 `1ffd5a5197fa348f8462834fd19c11075ccf2fa4822a67cba7a133f11871977a`。
- 桌面和 `390x844` 移动浏览器验收通过：页面无整体溢出，宽表仅容器内滚动，70 个业务请求全部返回 200，控制台 0 error/0 warning。当前 `48/48` 审计锚点全部本地导出、远端对象锁导出并校验通过。
- 当前健康仍为 `32/33`，非健康项是历史统计窗口中的 7 次后台失败，不是 0078 迁移失败。生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、真实多租户、生产前端和目标环境短稳/外部监控 6 类证据缺失而被严格门禁阻断。`require_24h=false`，本轮没有启动 24 小时运行。

## 当前候选增量（0079 + 服务账号 API Key 吊销双人会签）

- `service_account.key.revoke` 已纳入高风险审批中心并固定为 critical：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接吊销返回 `428`，审批治理不能关闭或降低会签门槛。
- 审批申请校验并冻结服务账号、Key、预期版本、目标和原因；执行阶段重新检查版本和吊销状态。Key 吊销、平台操作审计与审批效果操作 ID 同事务提交，恢复执行不会重复吊销。总后台弹窗已显示“提交吊销审批”。
- 79 版迁移 apply/rollback/replay、全量 Go 测试、服务账号与审批专项、独立 Compose 和六套本地短验收均通过。最终证据为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `929529920e3b3eccfa5b6b4a5c704443a60959141acef1f84a4b4d4f677995`，文件数 889，MySQL 5.7 静态检查覆盖 157 个文件。
- 当前预览镜像为 `sha256:27b1dfe860c8865ba8fb1d2d6981efaee8385b7ae766f5a8192b11107f81f865`，迁移账本为 `79/0079_saas_service_account_key_revoke_guard`，审批策略总数 15；`/readyz` 返回 200，构建指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0079-service-account-key-revoke-guard-20260716-103008.sql`，权限 `0600`、大小 1206245 字节、SHA-256 `bf719a02bc6f2eb5aabb015865980db579263e1680a37bd46dcd10bdc76e37a9`。
- 桌面与 `390x844` 浏览器回归通过：策略值为 `2/120/30/12`，有效 Key 可打开带原因输入的审批弹窗；页面无整体横向溢出，移动宽表仅在 368px 内部容器滚动，弹窗与提交按钮完整可见。最终 69 个业务请求均成功，控制台 0 error/0 warning。
- 当前健康为 `32/33`，唯一 critical 是统计窗口内保留的 6 次历史后台任务失败；50 个审计锚点的本地证据、异地 Object Lock 工件和校验状态全部通过。生产候选仍因 6 类真实外部证据缺失被严格门禁阻断；`require_24h=false`，本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0080 + 服务账号配置变更双人会签）

- `service_account.update` 已纳入高风险审批中心并固定为 critical：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。有效配置的直接更新返回 `428`，审批治理不能关闭或降低会签门槛。
- 审批申请规范化并冻结服务账号状态、作用域、CIDR、额度、预警、冷却、有效期和当前版本；版本漂移返回 `409`。执行阶段把账号更新、平台操作审计与审批效果操作 ID 同事务提交，恢复执行不会重复应用配置。总后台既有账号的保存入口已显示“提交修改审批”。
- 80 版迁移 apply/rollback/replay、全量 Go 测试、服务账号与审批专项、独立包和六套本地短验收均通过。最终证据于 `2026-07-16 11:23:35 CST` 至 `12:39:14 CST` 生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `a52c324c86a302f06d63095d477aa259585b51f650462013dc88b0342c4378fe`，文件数 891，MySQL 5.7 静态检查覆盖 159 个文件。
- 当前预览镜像为 `sha256:c39d8a30362dbd83248ae87d1f790e34a67e13aebe3a3c324d3a7e8dde06b9ea`，迁移账本为 `80/0080_saas_service_account_update_guard`，审批策略总数 16，其中 15 条 critical；`/readyz` 返回 200，构建指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0080-service-account-update-guard-20260716-124042.sql`，权限 `0600`、大小 1224462 字节、SHA-256 `a04faaaa47503f8957f23881868ff038a089318ddd07f000473b98b9d06a7b5f`。
- 桌面与 `390x844` 浏览器回归通过：策略值为 `2/120/30/12`，既有账号显示“提交修改审批”；页面无整体横向溢出，移动审批宽表仅在 368px 内部容器滚动。授权会话累计 146 个动态请求全部为 2xx，控制台 0 error/0 warning。
- 当前健康为 `32/33`，唯一 critical 是统计窗口内 6 次历史后台任务失败；当前无活跃事故，51 个审计锚点均已本地导出、异地对象锁导出并验证。生产候选仍因 6 类真实外部证据缺失被严格门禁阻断；`require_24h=false`，本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0081 + 服务账号 API Key 轮换双人会签）

- `service_account.key.rotate` 已纳入高风险审批中心并固定为 critical：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接轮换返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结服务账号版本和轮换参数，不提前生成明文 Key；执行阶段重新校验版本后才生成新 Key。新 Key 入库、旧 Key 宽限退役、账号版本更新、操作审计和审批效果标记同事务提交，恢复执行不会重复应用业务副作用。
- 明文 Key 只在首次执行响应中返回，审批持久化结果不保存 `plainTextKey`，只记录已交付标记。总后台轮换入口显示“提交轮换审批”，审批执行后才展示一次性 Key。
- 81 版迁移 apply/rollback/replay、全量 Go 测试、服务账号与审批专项、独立包和六套本地短验收均通过。最终证据于 `2026-07-16 13:34:00 CST` 至 `15:32:10 CST` 完整补齐，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `d4a07979b8cc5b7e7814f5debbb5488ba97ef6fab94c8ae1ef6bb876fa35e81f`，文件数 893，MySQL 5.7 静态检查覆盖 161 个迁移文件。
- 当前预览镜像为 `sha256:b3fb68ff27bc75576f333a8fb642a8e677630f8ae1796166b6d8f61edc81127e`，迁移账本为 `81/0081_saas_service_account_key_rotate_guard`，审批策略总数 17，其中 16 条 critical；`/readyz` 返回 200，PHP fallback 关闭，构建指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0081-service-account-key-rotate-guard-20260716-153441.sql`，权限 `0600`、大小 1250259 字节、SHA-256 `4bc7c7e94c443589ad425085372d2f7ba26ff3e11403c1db2a095f1776c890bf`。
- 桌面与 `390x844` 浏览器回归通过：策略强制启用且值为 `2/120/30/12`，轮换编辑区显示“提交轮换审批”，一次性 Key 区域未提前暴露；页面无整体横向溢出，移动审批表和服务账号表只在内部容器滚动。最终 70 个总后台动态请求全部为 200，控制台 0 error/0 warning，验收工件位于 `output/playwright/saas-0081-service-account-key-rotate/`。
- 当前健康为 `32/33`，唯一 critical 是统计窗口内 6 次历史后台任务失败；52 个审计锚点均已本地导出、异地对象锁导出并验证。生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据缺失而被阻断；`require_24h=false`，本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0082 + 服务账号创建双人会签）

- `service_account.create` 已纳入高风险审批中心并固定为 critical：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接创建在任何 Key 生成前返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段只冻结规范化的服务账号配置和首 Key 元数据，不保存明文；执行阶段重新校验租户状态和唯一账号代码后才生成首 Key。服务账号、Key 摘要、操作审计和审批效果操作 ID 同事务提交，恢复执行不会重复创建。
- 首 Key 明文只在首次审批执行响应中返回。审批持久化结果删除 `plainTextKey`，仅记录 `plainTextKeyDelivered=true`；总后台新建入口显示“提交创建审批”，执行成功后才展示一次性首密钥。
- 82 版迁移 apply/rollback/replay、全量 Go 测试、服务账号与审批专项、系统健康、备份恢复、独立交付和六套短验收均通过。最终证据为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `8d6dbe55ec10e5ec647a02892ef9306380304cd98ef7ff7fae6638b8387481a9`，文件数 895。
- 本地证据收集器支持 `MOCHAT_LOCAL_EVIDENCE_RESUME=1`，可在中断后保留成功项、重跑失败或缺失项并按唯一 `slug` 更新结果。本轮最终 14 条结果无重复记录。
- 当前预览镜像为 `sha256:c26ad3e42388a19cf89948c863a5e17ed26e9448a24f3654727c999a83c64d40`，迁移账本为 `82/0082_saas_service_account_create_guard`，审批策略总数 18，其中 17 条 critical；`/readyz` 返回 200，运行时路由 706 条，PHP fallback 关闭，build 指纹与证据一致。升级前快照为 `output/backups/mochat-preview-before-0082-service-account-create-guard-20260716-174445.sql`，权限 `0600`、大小 1263802 字节、SHA-256 `303e99f672f38477e2b8fe1bd0ff3d4e268bd426c8aeda7a172aa1999b1c5d19`。
- 桌面与 `390x844` 浏览器回归通过：创建策略强制启用且值为 `2/120/30/12`，新建区显示“提交创建审批”，一次性 Key 区域未提前暴露；页面无整体横向溢出，移动宽表只在内部容器滚动。69 个总后台动态请求全部返回 200，控制台 0 error/0 warning。
- 当前健康为 `33/33`，54 个审计锚点均已本地导出、异地对象锁导出并验证。生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据缺失而被阻断；`require_24h=false`，本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0083 + 用户 MFA 重置双人会签）

- `identity.mfa.reset` 已纳入高风险审批中心并固定为 critical：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。管理员直接重置返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结目标用户、租户、MFA 凭据版本和标准化原因；执行前重新校验快照。禁用 MFA、清除 TOTP 密钥与恢复码、失效挑战、撤销活动会话、身份安全事件、平台操作审计和审批效果标记在同一事务提交，恢复执行保持幂等。
- 83 版迁移 apply/rollback/replay、全量 Go 测试、身份安全与审批专项、独立包、独立 Compose 和六套本地短验收均通过。最终证据于 `2026-07-16 20:13:04 CST` 至 `21:15:04 CST` 从头生成，14 条命令记录全部 `returncode=0`；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `7e7d5073a0f2adb08789999156caf01c6bfb2a99fc9220e525506ec9606f1699`，文件数 898。
- 当前预览镜像为 `sha256:889a374354f7de2c7c6ca060ffa54983643ae13197ed7b07e186ae4c713c3737`，迁移账本为 `83/0083_saas_identity_mfa_reset_guard`，审批策略总数 19，其中 18 条 critical；`/readyz` 返回 200，启动日志中的 build 指纹与 latest 一致。升级前快照为 `output/backups/mochat-preview-before-0083-identity-mfa-reset-guard-20260716-200020.sql`，权限 `0600`、大小 1285262 字节、SHA-256 `275f412825fa7362e67b6d230ca342f860230d5454ac450efcbf15b824d0eecd`。
- 桌面与 `390x844` 浏览器回归通过：目标策略强制启用且值为 `2/120/30/12`；页面无整体横向溢出，移动宽表只在内部容器滚动，操作按钮完整可见。干净会话无失败请求，控制台 0 error/0 warning，验收工件位于 `output/playwright/saas0083-mfa-reset/`。
- 当前平台健康为 `33/33`，未处理问题和活跃事故均为 0。生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据缺失而被严格门禁阻断；`require_24h=false`，本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0084 + 平台套餐定义双人会签）

- `package.upsert` 已纳入高风险审批中心并固定为 critical：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。平台套餐直接创建或修改返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结当前套餐、目标套餐、预期版本和租户影响；执行阶段锁行并重新校验版本。套餐写入、平台操作审计和审批效果标记同事务提交，陈旧申请、重复创建和恢复执行不会覆盖新配置或重复产生副作用。
- 84 版迁移 apply/rollback/replay、定向与全量 Go 测试、套餐专项真实数据库 smoke、SaaS 总后台和功能矩阵短门禁均通过。当前预览镜像为 `sha256:db5e921d2159891098365fd43fd7ddebe461ddb168e8940516c4e5c5d36e21c0`，迁移账本为 `84/0084_saas_package_definition_guard`，策略为 `20/19 critical`，平台健康 `33/33`。
- 桌面与 `390x844` 浏览器回归通过：套餐区显示描述、只读版本和“提交套餐审批”，策略值为 `2/120/30/12`；页面无整体横向溢出，移动表单完整，控制台 0 error/0 warning。
- 本轮没有重建完整 `14/14` 证据包，也没有启动 24 小时运行。生产候选仍被六类真实外部证据阻断，因此不可发布。

## 当前候选增量（0085 + 租户套餐分配双人会签）

- `tenant.package.update` 已纳入高风险审批中心并固定为第 20 条 critical 策略：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。租户套餐直接分配返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结租户、当前套餐、目标套餐、订阅状态和 `expectedAssignmentVersion`；执行阶段锁定租户并重新校验版本。续费、开户和套餐同步造成的版本漂移会拒绝旧申请，套餐分配、订阅权益、操作审计与审批效果标记同事务提交，恢复执行保持幂等。
- 85 版迁移 apply/rollback/replay、定向与全量 Go 测试、套餐分配专项真实数据库 smoke、SaaS 总后台、审批主流程、审批治理、独立包、发布准备和功能矩阵短门禁均通过。smoke 覆盖为 `98/98`、manifest `224/224`，本轮未重建完整 `14/14` 证据包。
- 最终源码指纹为 `4ba1c159ca7f944dee5f2701a5479328cb0c3858c50302467a6bafe5338e17b8`，文件数 906。当前预览镜像为 `sha256:52d362570a54f44172337127f8d12763f90d2424048adb6d9a32fc968d3677f0`，迁移账本为 `85/0085_saas_tenant_package_assignment_guard`，审批策略总数 21，其中 20 条 critical；平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与源码一致。
- 升级前快照为 `/tmp/mochat-preview-before-0085-20260716-224216.sql`，权限 `0600`、大小 1323664 字节、SHA-256 `4473f0fcdf2df4001e0975fafe4f95f2f1f3ab950e5f4349d5d2ddf1f83c9386`；升级保留 MySQL、Redis、MinIO 和原数据卷。
- 桌面与 `390x844` 浏览器回归通过：点击“调整套餐”后租户 ID 1 和当前版本 1 正确回填但未提交审批；页面无整体横向溢出，移动表单与按钮完整，宽表仅在内部容器滚动。69 个动态请求全部返回 200，控制台 0 error/0 warning，验收工件位于 `output/playwright/saas-0085-tenant-package-assignment/`。
- 生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据缺失而被阻断。本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0086 + 平台租户开户双人会签）

- `tenant.provision` 已纳入高风险审批中心并固定为第 21 条 critical 策略：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。平台直接开户、单任务直接应用和批量直接应用均返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结任务版本、请求 SHA-256、套餐版本、管理员密码 bcrypt 哈希和开户预览；API、任务、审批、事件和审计只暴露密码哈希存在标记。执行阶段重新校验冻结快照，并将租户、管理员、角色、菜单、套餐快照、订阅、开户运行记录、任务状态、操作审计和审批效果标记同事务提交，任务或套餐漂移会拒绝旧申请。
- 86 版迁移 apply/rollback/replay、全量 Go 测试、平台开户审批专项真实数据库 smoke、审批主流程、审批治理、租户套餐分配审批、独立包和功能矩阵短门禁均通过。`scripts/test.sh` 与验收覆盖为 `99/99`、manifest `224/224`；本轮未重建完整 `14/14` 证据包。
- 最终源码指纹为 `ab8bed7be3de30847ce7ccfff5d51e52eee66b1a2368cd91dca1f540a607a897`，文件数 910。当前预览镜像为 `sha256:f907708826f3de90382c612c4f80469c2b9244ce384d5bcba4905286a07ee595`，迁移账本为 `86/0086_saas_tenant_provision_approval_guard`，审批策略总数 22，其中 21 条 critical；平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与源码一致。
- 升级前快照为 `/tmp/mochat-preview-before-0086-20260716-234326.sql`，权限 `0600`、大小 1332265 字节、SHA-256 `51ccb9cb742d99fac89e2b942c8b8895b3058c3400b14076a4441e9a9c4a5509`；升级保留 MySQL、Redis、MinIO 和原数据卷。
- 桌面 `1440x900` 与移动 `390x844` 浏览器回归通过：页面无整体横向溢出，移动端按钮无裁切，开户表单显示“提交开户审批”，策略行显示强制启用、双人审批和“提交审批”，控制台 0 error/0 warning。验收工件位于 `output/playwright/mochat-saas-admin-0086-*.png`。
- 生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据缺失而被阻断。doctor 与 preflight 均为 `0/6`，本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0087 + 租户续费双人会签）

- `tenant.renewal` 已纳入高风险审批中心并固定为第 22 条 critical 策略：强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接续费、单任务直接应用和批量直接应用均返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结租户、套餐分配、套餐版本、订阅版本、任务版本和请求 SHA-256；执行阶段锁行并重新校验。套餐权益、账单事件、订阅、任务状态、操作审计和审批效果标记同事务提交，任务取消或任一冻结引用漂移都会拒绝旧申请，恢复执行保持幂等。
- 87 版迁移 apply/rollback/replay、定向与全量 Go 测试、续费审批专项真实数据库 smoke、审批主流程、审批治理、系统健康、发布准备、独立包和功能矩阵短门禁均通过。验收脚本覆盖为 `100/100`，manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`；本轮未重建完整 `14/14` 证据包。
- 最终源码指纹为 `529e3a748d82b3e767f79826695905bea6ddefeda5fbdc18e779db9c3543dafb`，文件数 914。当前预览镜像为 `sha256:68be686d5f1dc21a1be77b534f326a1f502a78ad5a3820f9feea635d108fd742`，迁移账本为 `87/0087_saas_tenant_renewal_approval_guard`，审批策略总数 23，其中 22 条 critical；平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与源码一致。
- 升级前快照为 `/tmp/mochat-preview-before-0087-20260717-003852.sql`，权限 `0600`、大小 1345367 字节、SHA-256 `567266aec4052424edfd07cb88bcf5e158af70b681bf48ac40eb6e9e4d86018d`；升级保留 MySQL、Redis、MinIO 和原数据卷。
- 桌面 `1440x900` 与移动 `390x844` 浏览器回归通过：续费表单显示“提交续费审批”，任务区显示“提交任务审批”，策略行显示 `tenant.renewal` 和双人会签；页面无整体横向溢出，移动端按钮无裁切，控制台 0 error/0 warning。验收工件为 `output/playwright/mochat-saas-admin-0087-*.png`。
- 生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据缺失而被严格门禁阻断。发布准备为 `0/6`、`ready=false`，本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0088 + 租户订阅状态迁移双人会签）

- `tenant.subscription.transition` 已纳入高风险审批中心并固定为第 23 条 critical 策略：强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。订阅直接迁移返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结租户状态、订阅 ID 与版本、目标状态、周期字段、取消标记和规范化原因；执行阶段锁定租户与订阅并重新校验。订阅状态、订阅事件、操作审计和审批效果标记同事务提交，平台租户或任一冻结引用漂移都会拒绝旧申请，恢复执行保持幂等。
- 88 版迁移 apply/rollback/replay、定向与全量 Go 测试、订阅迁移审批专项真实数据库 smoke、审批主流程、审批治理、系统健康、发布准备、功能矩阵和短时全量门禁均通过。验收覆盖为 `101/101`、manifest `224/224`，运行时唯一路径 `587/587`、运行时路由条目 `729`；本轮未重建完整 `14/14` 证据包。
- 当前源码指纹为 `1bce67b2dd8031457cdf59dc69b556256377760843bc500014d1bac74146e417`，文件数 918。当前预览镜像为 `sha256:257fc6a0967a602c8cfeaa817f67eb86bf93735c455882876a0bf360edf81d9c`，迁移账本为 `88/0088_saas_subscription_transition_approval_guard`，审批策略总数 24，其中 23 条 critical；平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与源码一致。
- 升级前快照为 `/tmp/mochat-preview-before-0088-20260717-012431.sql`，权限 `0600`、大小 1355318 字节、SHA-256 `6d924ce2d2ed6cb831920c8208aa25f486cd93b665197a0444daa640c9205de2`；升级保留 MySQL、Redis、MinIO 和原数据卷。
- 桌面 `1440x900` 与移动 `390x844` 浏览器回归通过：订阅迁移表单显示“提交订阅审批”，策略行显示强制启用和 `2/120/30/12`；页面无整体横向溢出，移动字段与按钮完整，授权后控制台 0 error 且网络无 4xx/5xx。验收工件为 `output/playwright/mochat-saas-admin-0088-*.png`。
- 生产证据 doctor、preflight 和 current 包已刷新到当前指纹，结果为 `missing_count=6`、`not_ready_count=6`、`evidence_ok=false`，严格 current 按预期返回 1。生产候选仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据缺失而被阻断。发布准备为 `0/6`、`metadataReady=false`、`ready=false`；本轮未启动 24 小时运行，因此候选仍不可发布。

## 当前候选增量（0089 + 发票正式开具双人会签）

- `billing.invoice.issue` 已纳入高风险审批中心并固定为第 24 条 critical 策略：强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。蓝票和红票直接推进到 `issued` 返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结单据身份、类型、状态、版本、票面信息，以及支付订单版本和退款、开票、红冲金额台账；执行阶段锁行并重新校验。单据状态、订单开票或红冲金额、操作审计和审批效果标记同事务提交，单据或订单漂移会拒绝旧申请，恢复执行保持幂等；处理中、失败和取消仍可由财务直接纠错。
- 89 版迁移 apply/rollback/replay、定向与全量 Go 测试、发票开具审批专项真实数据库 smoke、独立包、功能矩阵和短时总门禁均通过。验收脚本覆盖为 `102/102`、manifest `224/224`，运行时唯一路径 `587/587`、运行时路由条目 `729`；MySQL 5.7 静态检查覆盖 177 个文件，本机 ARM 实容器按规则跳过且不作为生产证据。
- 最终源码指纹为 `23fbbafd118f6c679d3402666f8a680ce13ddc13068f2b2b8c4f35b12cc02145`，文件数 922。当前预览镜像为 `sha256:7183026feb6d0ba806f441b0a7833a58b92a2b39fd99f6ecaf041caae920a1a5`，迁移账本为 `89/0089_saas_invoice_issue_approval_guard`，迁移 checksum 为 `39a732c3088f65ad93410d019b269a4b1b97a9ad0132f329a18a924b929b78bd`，审批策略总数 25，其中 24 条 critical；平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与源码一致。
- 升级前快照为 `/tmp/mochat-preview-before-0089-20260717-021602.sql`，权限 `0600`、大小 1367156 字节、SHA-256 `344c3f92ab5ce983bb8482349296aaee7202e5e04b9a751d8fb27cc693deadcb`。升级只替换 App 并应用 0089，MySQL、Redis 和原数据卷均保留。
- 桌面 `1440x900` 与移动 `390x844` 浏览器回归通过：选择“已开具”后表单显示“提交开具审批”，策略行显示 `billing.invoice.issue`、强制启用和 `2/120/30/12`；页面无整体横向溢出，移动按钮无裁切，宽表仅在内部容器滚动。69 个动态请求全部为 200，控制台 0 error/0 warning，验收工件位于 `output/playwright/saas-0089-invoice-issue/`。
- 生产 doctor、preflight 和 current 包已刷新到最终指纹，结果仍为 `missing_count=6`、`not_ready_count=6`、`evidence_ok=false`；严格证据检查和 skip-local 生产候选门禁均返回 1。生产候选继续被 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据阻断。本轮未启动 24 小时运行，因此候选仍不可发布。

## 最终本地候选快照（2026-07-18）

- 最终本地证据包包含 15 条成功记录，六套独立验收 `core/workers/cron/saas/frontend/mysql57` 全绿；smoke 覆盖 `102/102`、manifest 路由 `224/224`、前端 dist API `209/209`，源码指纹稳定检查通过。
- 当前候选指纹为 `7c5ef5690ec238b7330aeefd85fe1df81d659835c6895d1eba054b97cd0085f1`，文件数 922。生产 current 包和 skip-local 候选门禁均使用该指纹，候选源码匹配结果为 `True`。
- 本地 compose 候选已用全新数据卷完成端到端验收。预览地址 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，登录凭据仅保存在本地受控配置；App、MySQL、Redis 健康，迁移账本为 `89/0089_saas_invoice_issue_approval_guard`，审批策略为 `25/24 critical`，镜像为 `sha256:6b9b10d08f5763215c4e5687e3e589b413fca4fccbbffdeb9dc6fd0c37c62a75`。
- 本地候选已经具备进入真实环境补证的条件，但尚不能标记生产可发布。严格门禁只剩六项外部证据，且 `require_24h=false`：稳定性只要求目标环境短稳回归、健康检查和外部监控记录，不再默认执行 24 小时持续运行。

## 当前候选增量（0090 + 收款订单创建双人会签）

- `payment.order.create` 已纳入高风险审批中心并固定为第 25 条 critical 策略：强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接创建收款订单返回 `428`，审批治理不能关闭或降低会签门槛。
- 申请阶段冻结有效租户、租户版本、套餐定义版本与完整额度、金额、服务周期、收银台失效时间和幂等订单号；执行阶段锁行并重新校验。订单、操作审计和审批效果标记同事务提交，套餐或租户漂移会拒绝旧申请，恢复执行保持幂等。
- 支付订单持久化 `package_version` 和 `package_limits_json`，回调结算应用下单时冻结的权益快照，不受后续套餐定义调整影响。90 版迁移 apply/rollback/replay、定向 Go 测试、专项真实数据库 smoke、`scripts/test.sh` 与 MySQL 5.7 静态门禁均通过；验收覆盖为 `103/103`、manifest `224/224`、运行时唯一路径 `587/587`、运行时路由条目 `729`，静态检查覆盖 179 个文件。
- 当前源码与 build 权威指纹为 `f2c01af52235f4fe67f7bf7f6a4de6645da53dbcb4b7e0407399654f2f2da6f2`，文件数 926。当前预览镜像为 `sha256:ed6d123e71b3971ca83309181935964ea2721f8ea4b91ef7bb340e07e327f076`，迁移账本为 `90/0090_saas_payment_order_create_approval_guard`，迁移 checksum 为 `edd6cd74fc3410ba6a757c6e5bba0ccabd4a3cf7894fa056444c98239ebb2299`，审批策略总数 26，其中 25 条 critical；App、MySQL 和 Redis 健康。
- 升级前快照为 `/tmp/mochat-go-preview-pre-0090-20260718.sql`，权限 `0600`、大小 516003 字节、SHA-256 `56046922adf892dd52fd33ce1b9e9a495421e21399cba604ddc49552624c81d1`。升级只替换 App 并应用 0090，MySQL、Redis 和原数据卷均保留。
- 桌面与 `390x844` 移动浏览器回归通过：表单显示“提交收款审批”，套餐快照字段和按钮无重叠或裁切，验收工件为 `output/playwright/0090-payment-desktop.png` 与 `output/playwright/0090-payment-mobile.png`。预览未启用的可选身份安全接口仍返回既有 `501`，不属于 0090 回归。
- 本地候选已完成 0090 增量验证，但仍不可标记生产可发布。严格门禁继续被 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类证据阻断；本轮未重建完整 15 项证据包，也未启动 24 小时运行。

## 当前候选增量（0091 + 渠道结算关账双人会签）

- `payment.settlement.close` 已升级为第 26 条 critical 策略，当前 26 类高风险动作全部固定强制启用、零绕过阈值且至少双人会签。直接关账返回 `428`，审批治理不能关闭、设置金额绕过阈值或降为单人。
- 申请阶段冻结精确批次身份、账期、渠道、币种、来源摘要、状态、全部对账计数、金额台账、版本和备注，并拒绝非已对账批次或仍有未解决差异的批次。执行阶段重新锁定并逐字段校验，未递增版本的金额漂移同样返回 `409`。
- 关账状态、操作审计和审批效果标记在同一事务提交，失败不产生部分副作用，批准重放保持幂等。专项真实数据库 smoke、91 版迁移 apply/安全 rollback/replay、定向 Go 测试、`scripts/test.sh` 和 MySQL 5.7 静态门禁均通过；验收覆盖为 `104/104`、manifest `224/224`、运行时唯一路径 `587/587`、运行时路由条目 `729`，静态检查覆盖 181 个文件。
- 当前源码与 build 权威指纹为 `704f43a147148e65a75ac2497a92bb4bfcbf67fc43daf16c0e7ef97932ac7be3`，文件数 930。预览镜像为 `sha256:fecb35b03a7b106be3a6679a91f98501f63289d821e061eeccba7822cdd1ec40`，迁移账本为 `91/0091_saas_payment_settlement_close_guard`，迁移 checksum 为 `bc47db225b65be316a7bdff0e25f4be7faab648c042eb486651a9547720d2cab`，审批策略为 `26/26 critical`；App、MySQL、Redis 和迁移健康探针均健康。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0091-settlement-close-guard-20260718-142250.sql`，大小 520117 字节、SHA-256 `237442df6e0a4eaad1bc7c25b9e24941ca97ce76b07f63c631596fdb48b293a7`。升级只应用 0091 并替换 App，MySQL、Redis 和原数据卷均保留。
- 桌面与 `390x844` 浏览器回归通过。策略行完整显示强制启用、两人会签和 `240/60/24`，移动端无页面级横向溢出，审批宽表内部滚动后右侧操作仍完整可见。可选身份安全接口的既有 `501` 仍是预览配置边界，不属于 0091 回归。
- 该候选已完成本地 0091 增量验证，但尚不能标记生产可发布。严格门禁继续等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类证据；本轮未重建完整 15 项证据包，也未启动 24 小时运行。

## 当前候选增量（0092 + 渠道结算重开双人会签）

- `payment.settlement.reopen` 已成为第 27 条 critical 策略，当前 27 类高风险动作全部强制启用、零绕过阈值且至少双人会签。直接重开返回 `428`，只允许对具有完整关账审计的已关闭批次发起申请。
- 申请冻结完整账务、导入/对账元数据和关账人、时间、原因；执行阶段锁行并重新校验，账务或关账审计漂移返回 `409`。批次恢复为已对账、关账字段清空、操作审计和审批效果标记同事务提交。新版关账/重开载荷使用 `schemaVersion=2`，0091 已发起的旧关账审批仍可执行。
- 定向与全量 Go 测试、关账和重开真实数据库 smoke、92 版迁移完整 apply/rollback/replay、独立包、MySQL 5.7 静态检查、`105/105` 验收覆盖与功能矩阵均通过。Compose 新库初始化已补挂 0090 至 0092，并由通用静态门禁保证全部 91 个增量迁移均有初始化挂载。
- 当前源码与 build 权威指纹为 `b9670008712828fbf602f94ff3a820947011b6fc34056c422912619ecb5b9e55`，文件数 933。预览镜像为 `sha256:f941527ccd96fb9f52d684e41a7a982bf371551aa6ac6e20797751765fbdbdeb`，迁移账本为 `92/0092_saas_payment_settlement_reopen_guard`，checksum 为 `3638ba7a955befb5e8ff28f891eeda717000d0128e3568a6cdc06fb1f6451d66`，审批策略为 `27/27 critical`；App、MySQL、Redis 均 healthy。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0092-settlement-reopen-guard-20260718-153137.sql`，大小 526679 字节、SHA-256 `c87464a87851fb7c94b52083ab75bc4a69f0e45e460112d3881e385df5facf3b`。升级只应用 0092 并替换 App，数据服务和原数据卷均保留。
- 桌面与 `390x844` 浏览器回归通过，页面无整体横向溢出，移动审批表可在内部滚动查看动作和右侧操作；验收截图位于 `output/playwright/mochat-go-0092-approval-policy-*.png`。可选身份安全接口的 4 个既有 `501` 和首次未授权 `401` 属于预览配置边界，不是 0092 回归。
- 该候选已完成本地 0092 增量验证，但仍不可标记生产可发布。发布准备为 `0/6`、`ready=false`，继续缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；`require_24h=false`，不执行 24 小时运行。

## 当前候选增量（0093 + 渠道结算差异处理双人会签）

- `payment.settlement.resolve` 已成为第 28 条 critical 策略，当前 28 类高风险动作全部强制启用、零绕过阈值且至少双人会签。直接解决、忽略或重新打开差异返回 `428`；申请冻结完整差异条目和批次账务，批准执行时按固定锁序复核，任一账务、处理审计或无版本漂移返回 `409`。
- 条目状态、批次差异汇总、平台操作审计和审批效果标记同事务提交。合法迁移只允许 `open -> resolved/ignored` 与 `resolved/ignored -> open`，匹配明细和已关闭批次不能进入人工处理链路。
- 定向与全量 Go 测试、0093 真实数据库 smoke、93 版迁移完整 apply/rollback/replay、独立包、MySQL 5.7 静态检查、`106/106` 验收覆盖与功能矩阵均通过。功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。
- 当前源码与 build 权威指纹为 `46217ddd6564fc2916dbdcda3f5addf1ac5b5449409e5ffdf3eca0c9628203c0`，文件数 937。预览镜像为 `sha256:3574cb3e1338e2951738c78b8a32e392aa37f542e8d39a50672354575c8d89e6`，迁移账本为 `93/0093_saas_payment_settlement_resolve_guard`，checksum 为 `522051435bdf11f1fba800d803b38209a0afd8ecb166e4d2b202c8e1f5c481fd`，审批策略为 `28/28 critical`；App、MySQL、Redis 均 healthy。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0093-settlement-resolve-guard-20260718-164721.sql`，大小 533593 字节、SHA-256 `7183d424c198e6db0281fcff7284bdadc0bb08f54a76deb7d05c641e01c774d4`。升级只应用 0093 并替换 App，数据服务和原数据卷均保留。
- 桌面与 `390x844` 浏览器回归通过，页面无整体横向溢出，移动审批表可在内部滚动查看动作、`240/60/24` 和右侧操作；证据位于 `output/playwright/saas-0093-settlement-resolve/`。70 条网络记录中 0093 关键接口全部 `200`，仅有保存 JWT 前的预期 `401` 和身份安全关闭时既有的 4 个 `501`，未提交审批或修改预览数据。
- 该候选已完成本地 0093 增量验证，但仍不可标记生产可发布。系统健康为 `27/30`，3 个环境 critical 不变；发布准备为 `0/6`、`ready=false`，继续缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控。`require_24h=false`，不执行 24 小时运行。

## 当前候选增量（租户上线准备度与处置导航）

- 新增 RBAC 保护的 `GET /dashboard/saasAdmin/tenantReadiness`。平台可按租户、关键字和 `ready/attention/blocked` 状态查看 8 项必需上线条件与 3 项建议交付条件；接口只读、无新迁移，要求 `platform.tenants.read`。
- 总后台“租户套餐”工作区新增准备度汇总、筛选、检查明细和“定位”动作。定位会跳到订阅、企业微信凭据、身份安全、通知策略、品牌档案或域名交付模块并预填租户上下文，不直接写入业务状态。
- 全量 Go 测试、JavaScript 编译检查和真实 MariaDB/Redis 总后台专项 smoke 通过。最终源码/build 指纹为 `72285fb2f416e42c7f7e62e67c5c7d54fe59075d5e989f1a7773b756dc77b21a`，940 个文件；最终预览镜像为 `sha256:7d742d4d3f87e9621aaf57919b0484d2e0518265b6731ed909bf9750a3dd9b1d`。
- App、MySQL、Redis 均 healthy；纯 Go 运行时路由为 707 条，PHP fallback 关闭，迁移仍为 `93/0093_saas_payment_settlement_resolve_guard`，原数据卷和业务记录保留。当前租户准备度为 `blocked/64%`，核心只缺订阅，建议项缺通知策略、品牌档案和主域名交付。
- `1440x900` 与 `390x844` Playwright 回归通过，控制台 `0 error/0 warning`，准备度请求 `200`，页面无整体横向溢出，移动宽表只在内部滚动；截图位于 `output/playwright/saas-tenant-readiness/`。
- 候选仍不可标记生产发布。发布准备接口确认 `passed=0/required=6`、`missing=6`、`metadataReady=false`、`ready=false`；六类外部证据未补齐，本轮按要求不执行 24 小时运行。

## 当前候选修正（准备度仅统计业务租户）

- 上一节记录的 `blocked/64%` 对象实际是平台管理租户，不是客户租户。平台控制面不参与订阅、续费和生命周期，该旧准备度结论作废。
- 准备度 API 现在固定返回 `scope=business_tenants` 和 `platformAdminTenantId`，全量查询排除平台租户，显式查询平台租户返回 `400`。当前预览没有业务租户，因此汇总为业务租户、可上线、待完善、已阻塞均为 0；这不是一个“已就绪租户”结论，也不能充当真实多租户证据。
- 全量 Go 测试、JavaScript 编译检查、脚本语法检查和真实 MariaDB/Redis 总后台 smoke 通过。最终源码/build 指纹为 `fb41266d0f0df9adaabdfc6d73362128eec56974eb99736941d99f51eabdeae1`，940 个文件；最终预览镜像为 `sha256:4e32ea6e87290ea6c4d1dc61fe6e86464ce764c079000459efc0930e25bcaf5a`，迁移仍为 `93/0093_saas_payment_settlement_resolve_guard`。
- `1440x900` 与 `390x844` Playwright 回归确认新空态、无页面级横向溢出和 0 error/0 warning，截图位于 `output/playwright/saas-tenant-readiness-business-scope/`。候选仍缺至少两个真实业务租户及其隔离、开户、订阅、企微、生产前端和目标环境证据；本轮未执行 24 小时运行。

## 最终本地候选快照（2026-07-19）

- 总后台客户经营口径已统一为“业务租户”，平台控制租户不再进入总览、经营、续费、客户成功、运营待办、日报和导出；显式租户范围仍可用于平台租户排障。真实 MariaDB/Redis 专项 smoke、全量 Go 测试和桌面/移动 Playwright 均通过。
- 正式本地证据包 `docs/phases/phase-pre0-standalone/evidence/latest` 已从头生成，15 条命令全部成功，六套短验收 `core/saas/workers/cron/frontend/mysql57` 全绿，验收前后源码指纹稳定。发布准备、审批过期与前端登录端口隔离三个验收缺口已修正并纳入该证据包。
- 最终候选指纹为 `4126eaa2835b908374aba231f4b10f87f793e215c8b399bbc0fd1b63428c9a05`，文件数 941。预览镜像为 `sha256:c7af82a5d49e1ce659a1c8772722bf9d6253bd039704deab580bf5d606bb74b2`，App、MySQL、Redis 均 healthy，健康探针返回 200，迁移账本为 `93/0093_saas_payment_settlement_resolve_guard`。
- 该快照可以进入真实环境补证，但不能标记生产可发布。发布准备仍为 `0/6`、`ready=false`，缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控；`require_24h=false`，不执行 24 小时运行。

## 当前候选增量（0094 + 租户域名路由变更双人会签）

- `tenant.domain.command` 已成为第 29 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.domains.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。主域名切换、域名启停、DNS 校验令牌轮换和删除的直接请求返回 `428`；DNS 所有权验证保持直接执行。
- 申请冻结目标域名与租户全部未删除域名的完整路由快照；批准执行时按固定顺序锁定并重新校验。任何域名版本、主域关系或候选路由漂移返回 `409`；域名状态、路由/TLS 交付任务、操作审计与审批效果标记同事务提交。轮换令牌只在执行时生成，审批请求和持久化结果均不保存明文。
- 定向与全量 Go 测试、域名审批专项真实 MariaDB/Redis smoke、原域名兼容 smoke、审批主流程/治理、94 版迁移完整 apply/down/replay、`scripts/test.sh`、独立包和 MySQL 5.7 静态门禁均通过。验收接入覆盖为 `107/107`、manifest `224/224`、运行时唯一路径 `588/588`、静态运行时路由条目 730；最终完整本地证据包 `15/15` 通过，六套验收全绿，源码稳定性门禁通过。
- 当前源码与 build 权威指纹为 `111f573666625f1382b5908c9addefa56ac68ea16584a272a23ad95c1c339aa7`，文件数 945。预览镜像为 `sha256:a6a28779d3f8567716e0b5342ff037af8bb581e663a740b3baef6ae4140e0053`，迁移账本为 `94/0094_saas_tenant_domain_command_guard`，checksum 为 `a7f23b5c1d0c67abc18021bd20d7be68bea10c7f80dbc1943c0b680c0cb1ba1c`，审批策略为 `29/29 critical`；App、MySQL、Redis 均 healthy，系统健康 `31/31`。
- 全量证据收集中发现并修正独立 Compose 备份 smoke 仍断言 93 个迁移的问题；`migrationCount=94` 的独立重跑与整包重跑均通过。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0094-tenant-domain-command-guard-20260719-012928.sql`，大小 639408 字节、SHA-256 `ef006bae1e93c2ed2478419309e25469f56e826025f1a5c2390a2a1bb4714d06`。升级只应用 0094 并替换 App，数据服务、运行配置和原数据卷均保留。
- 桌面 `1440x1000` 与移动 `390x844` 浏览器回归通过。策略行显示“变更租户域名路由”、强制启用和 `2/120/30/12`，移动页面无整体横向溢出，审批宽表只在内部容器滚动；截图位于 `output/playwright/saas0094/`。预览没有租户域名记录，浏览器未创建记录或提交审批。
- 该候选已完成本地 0094 增量验证，但仍不可标记生产可发布。发布准备为 `0/6`、`missing=6`、`ready=false`，继续缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控；`require_24h=false`，不执行 24 小时运行。

## 当前候选增量（0095 + 租户域名新增双人会签）

- `tenant.domain.create` 已成为第 30 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.domains.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接新增返回 `428`，DNS 所有权验证保持直接执行。
- 申请只冻结标准化后的租户和域名，不生成 DNS 校验令牌；批准执行时重新锁定并校验租户状态、域名唯一性与每租户 10 个域名配额。令牌只在执行时生成并通过响应交付一次，域名、初始交付状态、操作审计与审批效果同事务提交，持久化审批结果不保存令牌明文。
- 定向与全量 Go 测试、95 版迁移完整 apply/checksum/down/replay/baseline/legacy、域名新增/变更审批专项、原域名兼容、发布准备、备份恢复、审批主流程/治理和独立 Compose 均通过。正式本地证据包 `15/15`，六套短验收全绿；验收覆盖 `107/107`、manifest `224/224`、运行时唯一路径 `588/588`、静态运行时路由条目 730、前端 dist API `209/209`。
- 当前源码与 build 权威指纹为 `432cc0978954d869c44b0cf6cc782ca0381758f2ab0297e32b3d990028a1f23d`，文件数 947，0095 checksum 为 `639cfdd943495ae04bcc100864258c71ae7f1e16f798c994b282ebaee572b6fd`。预览镜像为 `sha256:e1ddf1a69a13f4a3db6544ca3409fc87cfd43332c2a69b4cfa232c544b9e2677`，迁移账本为 `95/0095_saas_tenant_domain_create_guard`，审批策略为 `30/30 critical`；App、MySQL、Redis 均 healthy，系统健康 `31/31`。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0095-tenant-domain-create-guard-20260719-034908.sql`，大小 658052 字节、SHA-256 `6a171c22d397ae46577f63454c79ab7d9b91aa5ac87d8ec57347758776656073`。升级只应用 0095 并替换 App，数据服务、运行配置和原数据卷均保留，环境差异为 0。
- 桌面 `1440x1000` 与移动 `390x844` 浏览器回归通过。策略行显示“新增租户域名绑定”、强制启用和 `2/120/30/12`，域名中心显示“提交添加审批”；控制台 0 error/0 warning，动态请求均为 `200`，移动页面无整体横向溢出，截图位于 `output/playwright/saas0095/`。浏览器未创建记录或提交审批。
- 生产证据包已刷新，严格门禁按预期失败且只缺六类外部证据。该候选已完成本地 0095 增量验证，但仍不可标记生产可发布：发布准备为 `0/6`、`missing=6`、`ready=false`，继续缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控；`require_24h=false`，不执行 24 小时运行。

## 当前候选增量（0096 + 租户重新启用双人会签）

- `tenant.enable` 已成为第 31 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.tenants.manage`、2 名不同复核人、240 分钟 SLA、60 分钟提醒和 24 小时失效。审批开启时，非平台业务租户直接重新启用返回 `428`；平台控制租户保留既有控制面边界。
- 申请冻结租户名称、状态和订阅存在性、ID、状态、版本；执行阶段锁行复核，任一漂移返回 `409`。租户恢复、订阅恢复、操作审计和审批效果同事务提交，失败不产生部分副作用，恢复快照后可重试；0096 前已创建的旧停用审批仍可执行。
- 定向与全量 Go 测试、迁移完整 apply/checksum/down/replay/baseline/legacy、租户启停审批专项、审批治理、备份恢复、独立 Compose 与 `scripts/test.sh` 均通过。正式本地证据包 `15/15`，六套短验收全绿；验收覆盖 `107/107`、manifest `224/224`、运行时唯一路径 `588/588`、静态运行时路由条目 730。
- 当前源码与 build 权威指纹为 `ad95a7ba63dc2ec688355083c5cedbb9e81790ada25a4136ee01c01c06ae0e25`，文件数 950，0096 checksum 为 `9a5c47a06f395ef320399ad17100c47fdcae31fc372dd66fd5f588587ca1ed0b`。预览镜像为 `sha256:45a45ca918529beeb0765ed5d91c1c79b0d913094d0cd4709dda61df201435c1`，迁移账本为 `96/0096_saas_tenant_enable_approval_guard`，审批策略为 `31/31 critical`；App、MySQL、Redis 均 healthy。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0096-tenant-enable-guard-20260719-061642.sql`，大小 680393 字节、SHA-256 `34f1c68d9483f51741d5955d85fb5cae5320e70ad3a4c2d2af49795f8e647e0a`。升级仅应用 0096 并替换 App，数据服务和原数据卷均保留。
- 桌面 `1440x1000` 与移动 `390x844` 浏览器回归通过。租户状态表单显示“提交启用审批”，策略行显示强制启用和 `2/240/60/24`；页面无整体横向溢出，移动宽表只在内部滚动，控制台 0 error/0 warning，70 个动态请求全部为 `200`，截图位于 `output/playwright/saas0096/`。
- 生产 current 包已刷新到同一指纹，严格门禁按预期只因六项外部证据返回失败。该候选已完成本地 0096 增量验证，但仍不可标记生产可发布：发布准备为 `0/6`、`missing=6`、`ready=false`；`require_24h=false`，不执行 24 小时运行。
