# MoChat Go 独立化缺口清单

## 目标口径

用户最新验收口径不是“Go 网关 + PHP fallback”，而是：

- 一个能运行的独立 Go 项目。
- 运行时不依赖原 `mochat/` PHP/Vue 源码目录。
- 运行时不依赖 PHP Hyperf upstream。
- 全部 MoChat 功能迁移到 Go，未迁移路由不能靠 fallback 冒充完成。

因此后续验收必须用 standalone 模式判断：

```bash
MOCHAT_GO_STANDALONE=1 go run ./cmd/mochat-go
```

## 当前缺口快照（2026-07-18）

本地 Go standalone 与 SaaS 总后台的当前闭环事实：

- 当前迁移账本为 `93/0093_saas_payment_settlement_resolve_guard`；源码与验收指纹为 `ec0103a147f2cbb3baf9675ebf5837b7eeee579965337f8714caa4c2bc29f4a8`，运行镜像为 `sha256:d1a9045aadf58a43e99bd0e5394f1f2cb9118bdd7f8331cbe8692752a3473b40`，启动日志确认二者一致。
- 平台健康为 `31/31`，加密备份、校验和隔离恢复演练已通过，恢复库无残留。
- 通知、企业微信、微信开放平台和身份安全已拆分为四个独立密钥域并强制凭据加密；企业微信存量旧明文已完成轮换。
- 身份安全中心相关接口已从 501 收口为 200，桌面和移动浏览器回归无控制台错误或页面级横向溢出。
- SaaS 总后台已拆分为 8 个权限感知工作区，支持模块跳转、键盘切换、状态持久化和移动端当前标签自动定位；证据位于 `output/playwright/saas-workspace-navigation/`。
- 平台范围的客户规模与经营链路已统一为业务租户口径：总览、经营、续费、客户成功、运营待办、日报及其导出均排除平台控制租户；显式租户范围保留平台租户排障能力。真实依赖 smoke、鉴权 API 和 `1440x1000`/`390x844` 浏览器回归均通过，截图位于 `output/playwright/saas-admin-business-scope-*.png`。
- 快速门禁保持 smoke `106/106`、manifest 路由 `224/224`、功能模块 `29`、前端 API `209/209`、SaaS 指标 `26`。

仍需在生产目标环境完成的缺口：

- 配置异地备份副本并验证远端工件可恢复；当前仅完成本地加密备份与隔离恢复。
- 把本地 `0600` 运行配置中的密钥迁移到生产 KMS/Secret Manager，并建立轮换和审计流程。
- 完成管理员 MFA 与持久会话迁移后，再启用 `sessionEnforced=true` 和生产强制 MFA 策略。
- 补齐发布准备 `0/6` 对应的六类真实外部证据：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、至少两个真实 SaaS 租户、生产域名前端、目标环境短稳与外部监控。
- 按用户当前要求不执行 24 小时运行，稳定性用目标环境短稳回归、健康检查和外部监控记录证明。

因此当前口径是“本地 SaaS 闭环完成，生产化补证继续”，不能标记生产最终完成。

## 本轮独立化门禁结果（2026-07-04）

- standalone + `MOCHAT_MYSQL_DSN` + `MOCHAT_SIMPLE_JWT_SECRET` 会默认启用当前所有已迁移业务路由；`MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES=0` 仍可关闭默认值，单个 `MOCHAT_GO_MIGRATE_*` 仍可覆盖。
- standalone 模式会强制忽略外部残留的 `MOCHAT_PHP_UPSTREAM`、`MOCHAT_SOURCE_ROOT` 和 `MOCHAT_COMPAT_MANIFEST`，配置对象不会继续暴露 PHP upstream、原项目路径或外部 manifest 路径。
- 新增 `scripts/audit_standalone_independence.sh` 并接入 `scripts/test.sh`，静态阻止独立交付包或核心 standalone smoke 重新引入 PHP 服务、原 `../mochat` 挂载、外部 manifest、fake PHP 或逐路由迁移开关依赖。
- 新增 `scripts/audit_acceptance_suite_coverage.sh` 并接入 `scripts/test.sh`，静态确保所有 `scripts/smoke_*.sh` 都已纳入 `scripts/standalone_acceptance.sh`，并反查总验收入口没有引用已删除的 smoke。
- 新增 `scripts/audit_manifest_route_smoke_coverage.sh` 并接入 `scripts/test.sh`，静态要求内置 manifest 的 224 条原 PHP 路由都在 `scripts/smoke_*.sh` 中有直接覆盖痕迹，防止只证明 Go handler 已挂载但漏掉真实 smoke。
- 新增 `scripts/audit_login_corp_validation.sh` 并接入 `scripts/test.sh`，静态阻止 dashboard 业务 handler 绕过统一登录企业校验直接信任 `mc:user.{userId}` 缓存。
- 新增 `scripts/audit_queue_annotation_coverage.sh` 并接入 `scripts/test.sh`，迁移期默认优先扫描本地 PHP 源码、独立交付包内退回 Go 内置 manifest，也可通过 `MOCHAT_QUEUE_AUDIT_SOURCE=embedded` 强制只读取 Go 内置 manifest，静态验证 15 条 PHP `@AsyncQueueMessage` 注解已收敛到 11 个 Go 语义队列，并且每个队列都有 registry、Redis store、worker 启动入口、smoke 脚本和文档记录。
- 新增 `scripts/audit_worker_saas_usage_assertions.sh` 并接入 `scripts/test.sh`，静态要求 11 个 Go 语义队列的 smoke 同时断言租户 `queue_item` 执行历史和 `async_executions` SaaS 用量计数器，防止只证明队列消费但漏掉 SaaS 运营计量。
- 新增 `scripts/audit_saas_metric_coverage.sh` 并接入 `scripts/test.sh`，静态验证 26 个 SaaS 指标在 `saas_quota.go`、`cmd/mochat-bootstrap`、`internal/store/mysql.go`、额度拦截 smoke 和用量重刷 smoke 中保持一致，防止套餐资源漏计、漏刷或漏验收。
- 新增 `scripts/audit_saas_storage_reclaim_coverage.sh` 并接入 `scripts/test.sh`，静态抽取 `internal/store` 中 26 个 SaaS 存储回收触发函数，要求 `scripts/smoke_saas_storage_reclaim.sh` 保持 14 类真实触发式账本回收断言，防止后续新增上传文件引用入口后漏验收。
- `scripts/standalone_route_coverage.sh` 已验证 manifest 业务路由 `224/224` 均命中 Go handler，`missing_route_total=0`；2026-07-06 复跑后，Go 原生补充路由 `extra_route_total=183`。
- 2026-07-06 23:22 复跑 `env -u GOROOT ./scripts/standalone_acceptance.sh`，`suite=all` 通过；MySQL 5.7 静态 lint 通过，真实 MySQL 5.7 容器 smoke 因本机 arm64 按脚本策略跳过，仍需在 amd64 CI 补跑。
- 2026-07-06 23:29 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=saas ./scripts/standalone_acceptance.sh`，`suite=saas` 通过；其中 `scripts/smoke_saas_tenant_isolation.sh` 已加深到角色/权限读路径和双租户上传账本隔离。
- 2026-07-06 23:37 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=workers ./scripts/standalone_acceptance.sh`，`suite=workers` 通过；其中 `scripts/smoke_async_file_upload_saas_usage.sh` 已加深为双租户异步上传隔离，验证 A 租户通过 `tenantId/corpId` 执行两次并只在 A 租户触发 `async_executions` 超额告警，B 租户通过 `corpId` 执行一次且不产生 A 租户的执行量、存储账本或告警。
- 2026-07-06 23:46 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；其中 `scripts/smoke_sidebar_frontend_contact.sh` 已加深到 sidebar 素材库 `/medium?agentId=1`，验证 `mediumGroup/index`、`medium/index`、素材分组、文本素材页面渲染和旧 `/undefined/sidebar/*` API 前缀归一化。
- 2026-07-07 00:02 复跑 `env -u GOROOT ./scripts/standalone_acceptance.sh`，`suite=all` 通过；覆盖 quick gate、PHP 源码清单对齐、schema migration、standalone/compose、路由 `224/224`、SaaS、workers、cron、frontend 和 MySQL 5.7 静态 lint；真实 MySQL 5.7 容器 smoke 仍因本机 arm64 按脚本策略跳过。
- 2026-07-07 00:11 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；其中 `scripts/smoke_sidebar_frontend_contact.sh` 已加深到 sidebar 个人 SOP 详情页 `/contactSop?id=900001&agentId=1`，验证 `contactSop/getSopTipInfo`、`contactSop/getSopInfo`、Go `cron-sop-log` 生成的个人 SOP 触达记录、弹窗和详情页文本渲染，且内置 sidebar dist 的个人 SOP 初始态已补齐空 `task/contact` 结构，避免页面先报 `task.content` 空指针再恢复。
- 2026-07-07 00:27 复跑 `env -u GOROOT ./scripts/standalone_acceptance.sh`，`suite=all` 通过；覆盖 quick gate、PHP 源码清单对齐、schema migration、standalone/compose、路由 `224/224`、SaaS、workers、cron、frontend、MySQL 5.7 静态 lint，以及新增 sidebar 个人 SOP 提醒和详情页浏览器链路；真实 MySQL 5.7 容器 smoke 仍因本机 arm64 按脚本策略跳过。
- 2026-07-07 00:35 复跑 `env -u GOROOT ./scripts/smoke_operation_frontend_work_fission.sh` 通过；该 smoke 不再预置 Go operation session，而是启动 fake 微信开放平台 API，验证 `/auth/workFission` 生成公众号 OAuth 外跳、code 回调调用 `api_component_token`、`sns/oauth2/component/access_token`、`sns/userinfo` 写入 `MOCHAT_SESSION_ID`，再由 `openUserInfo/workFission` 和 H5 主流程读出回调用户并完成海报、助力进度和领奖。
- 2026-07-07 00:39 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、dashboard 61 个旧业务页和 Go 原生页面、sidebar 客户/个人 SOP/群 SOP/批量加好友/素材库，以及新增的 operation 任务宝 OAuth 外跳、code 回调写 session、活动页和助力进度页浏览器链路。
- 2026-07-07 00:45 复跑 `env -u GOROOT ./scripts/smoke_sidebar_frontend_contact.sh` 通过；该 smoke 不再手工预置 sidebar JWT，而是启动 fake 企业微信 API，验证旧 sidebar dist 的 `/login` 会进入 Go `/sidebar/agent/auth`，生成企业微信 OAuth 外跳，code 回调调用 `auth/getuserinfo` 签发 sidebar 员工 JWT，并由前端 `/auth` 写入 `token/agentId` cookie 后落回 `/contact?agentId=1`，再继续完成客户详情、个人 SOP、群 SOP、批量加好友和素材库浏览器链路。
- 2026-07-07 00:49 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖新增 sidebar `/login` 企业微信 OAuth/code 回调、operation 任务宝 OAuth/code 回调、三端静态资源、dashboard 登录业务页、sidebar 客户/SOP/批量加好友/素材库和 operation 活动页浏览器链路。
- 2026-07-07 00:57 补齐渠道活码创建失败回滚的 SaaS 存储回收：`DeleteChannelCode` 会读取 `welcome_message` 并解析 `messageDetail` 中的本地欢迎语素材路径，软删对应 `mochat_go_saas_storage_objects` 后刷新 `storage_mb`；`env -u GOROOT ./scripts/smoke_saas_storage_reclaim.sh` 已覆盖真实 Go/MySQL/Redis/fake 企业微信下 `POST /dashboard/channelCode/store` contact_way 失败回滚、`channel_codes` 归零和 `channelCode/rollback.png` 账本回收。
- 2026-07-07 01:11 补齐 sidebar/operation 主端口前缀托管：Go 静态层新增 `/sidebar-app/` 和 `/operation-app/` 前缀入口，运行时重写旧 dist 的根路径 JS/CSS、webpack publicPath、Vue Router base 和缺失构建变量导致的 API base；`env -u GOROOT ./scripts/smoke_frontend_static_browser.sh` 已用真实浏览器验证 dashboard `/login`、主端口 `/sidebar-app/contact`、`/operation-app/workFission`、独立端口 sidebar `/contact` 和 operation `/workFission` 的本地 JS/CSS 均可加载。
- 2026-07-07 01:15 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖新增主端口 `/sidebar-app/contact`、`/operation-app/workFission` 前缀静态入口，并继续覆盖 dashboard 业务页、sidebar OAuth/code 回调到客户/SOP/批量加好友/素材库、operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 01:23 补齐 SaaS 存储回收覆盖门禁和群裂变邀请配置替换回归：`scripts/audit_saas_storage_reclaim_coverage.sh` 已验证 26 个 store 回收函数和 14 类 smoke 覆盖对齐；`env -u GOROOT ./scripts/smoke_saas_storage_reclaim.sh` 已新增覆盖 `POST /dashboard/roomFission/invite` 替换邀请封面后回收旧 `roomFission/invite.png` 账本，并继续验证整场群裂变删除时回收剩余 poster、room、welcome、invite 存储账本。
- 2026-07-07 01:28 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=saas ./scripts/standalone_acceptance.sh`，`suite=saas` 通过；覆盖 SaaS provisioning、租户隔离、套餐额度拦截、存储校准、存储回收、用量重刷、租户级告警通知配置投递和公众号开放平台 ticket。
- 2026-07-07 补齐企微回调事件覆盖门禁：`scripts/audit_wework_callback_event_coverage.sh` 会抽取 `WeWorkCallbackWorker.Process` 的事件路径，要求每个事件都出现在 `scripts/smoke_wework_callback_worker.sh`，并要求 PHP 兼容 no-op 事件保留单测和 smoke；`change_external_chat.create/dismiss` 已补真实 worker smoke，验证客户群创建回调只同步指定 `ChatId`、幂等触发入群自动标签，客户群解散回调只软删指定 `ChatId`，且都不影响其他客户群。
- 2026-07-07 01:41 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=workers ./scripts/standalone_acceptance.sh`，`suite=workers` 通过；覆盖异步上传、MarkTags、自动标签关键词任务、消息提醒、客户群/客户/部门同步、素材 media_id、成员统计、客户标签远程写、企微回调和 EmployeeApply + SaaS 告警 worker。
- 2026-07-07 补齐 SOP 周期规则第一版：`cron-sop-log` 支持 `cycle` / `period` / `repeat` / `frequency` 的 `daily`、`weekly`、`monthly`，可用 `weekdays`、`monthDays`、`repeatDates` 限定日期；周期日志会写入 `_mochatGoOccurrence`，保证同一周期幂等且跨日期可再次触达。`env -u GOROOT ./scripts/smoke_sidebar_frontend_contact.sh` 已用真实 MySQL/Redis、fake 企业微信 API、Go standalone 和真实浏览器验证普通与周期个人/群 SOP 日志生成、sidebar 弹窗/详情页和群 SOP 完成状态。
- 2026-07-07 01:52 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；覆盖三端静态资源、dashboard 浏览器页面矩阵、sidebar 登录 OAuth/code 回调、客户详情、普通与周期 SOP、批量加好友、素材库，以及 operation 任务宝 OAuth/code 回调和活动页。
- 2026-07-07 补齐群 SOP 客户入群锚点第一版：`cron-sop-log` 的群 SOP 纯相对延迟规则支持 `targetAnchor` / `target_anchor` / `anchor` / `event` / `trigger` 配置 `room_join`、`customer_join_room`、`客户入群` 等值，按 `mc_work_contact_room.join_time` 生成 contact 级群 SOP 日志，并把 `mc_room_sop_log.contact` 纳入幂等，避免同群不同客户入群提醒互相覆盖。
- 2026-07-07 02:03 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；覆盖三端静态资源、dashboard 浏览器页面矩阵、sidebar 登录 OAuth/code 回调、客户详情、普通/周期 SOP 和入群后群 SOP、批量加好友、素材库，以及 operation 任务宝 OAuth/code 回调和活动页。
- 2026-07-07 补齐 SOP 企微回调目标级触发第一版：`wework-callback` worker 在 `change_external_contact.add_external_contact` 同步客户关系后，会按当前员工/客户目标触发个人 SOP 日志生成；在 `change_external_chat.create/update` 单群同步后，会按回调 `ChatId` 定位本地客户群并触发 `targetAnchor=room_join` 的群 SOP 日志生成，复用 `cron-sop-log` 的规则解析和幂等逻辑。
- 2026-07-07 02:19 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=workers ./scripts/standalone_acceptance.sh`，`suite=workers` 通过；覆盖异步上传、MarkTags、自动标签关键词任务、消息提醒、客户群/客户/部门同步、素材 media_id、成员统计、客户标签远程写、企微回调、回调触发个人/入群群 SOP 和 EmployeeApply + SaaS 告警 worker。
- 2026-07-07 02:30 新增并通过 `env -u GOROOT ./scripts/smoke_room_welcome_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用 `roomWelcome` 新增、列表、详情、选择、编辑、删除接口，验证链接封面走 `media/uploadimg`、小程序封面走 `media/upload`、入群欢迎语模板走 `group_welcome_template/add/edit/del`，并断言 `mc_room_welcome_template` 创建、更新和软删除闭环。
- 2026-07-07 02:42 新增并通过 `env -u GOROOT ./scripts/smoke_work_room_auto_pull_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用 `workRoomAutoPull` 新增、列表、详情、更新和兼容 move 接口，验证 `externalcontact/contact_way/create`、`externalcontact/contact_way/update` 请求体、`mc_work_room_auto_pull` 写入、员工/标签/客户群反查、拉人状态计算和 `work_room_auto_pulls` SaaS 用量刷新。
- 2026-07-07 02:47 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 02:55 新增并通过 `env -u GOROOT ./scripts/smoke_room_tag_pull_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用 `roomTagPull` 客户筛选、过滤预估、新建、列表、详情、客户明细、员工任务、客户群下拉、提醒发送和删除接口，验证 `media/uploadimg`、`externalcontact/add_msg_template`、`message/send` 请求体、`mc_room_tag_pull` / `mc_room_tag_pull_contact` 写入、客户入群状态计算和 `room_tag_pulls` SaaS 用量刷新。
- 2026-07-07 03:01 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、标签建群 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 03:07 新增并通过 `env -u GOROOT ./scripts/smoke_contact_message_batch_send_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用 `contactMessageBatchSend` 立即发送、列表、详情、消息预览、客户群详情、员工发送明细、客户接收明细、提醒和删除接口，验证 `media/upload`、`externalcontact/add_msg_template`、`message/send` 请求体、`mc_contact_message_batch_send` / 员工任务 / 客户接收任务写入、删除清理和 `contact_message_batches` SaaS 用量刷新。
- 2026-07-07 03:15 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、标签建群 dashboard 写链路、客户群发 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 03:21 新增并通过 `env -u GOROOT ./scripts/smoke_room_message_batch_send_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用 `roomMessageBatchSend` 立即发送、列表、详情、群主发送明细、群接收明细、提醒和删除接口，验证 `media/upload`、`externalcontact/add_msg_template`、`message/send` 请求体、`mc_room_message_batch_send` / 群主任务 / 客户群接收任务写入、删除清理和 `room_message_batches` SaaS 用量刷新。
- 2026-07-07 03:27 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、标签建群 dashboard 写链路、客户群发 dashboard 写链路、客户群群发 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 03:41 新增并通过 `env -u GOROOT ./scripts/smoke_work_fission_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用 `workFission` 新建、列表、详情、配置详情、统计、选择客户、邀请数据、邀请明细、邀请配置提交、更新和删除接口，验证 `media/uploadimg`、`externalcontact/add_contact_way`、`externalcontact/add_msg_template` 请求体、`mc_work_fission` / welcome / poster / push / invite 写入、参与客户反查、更新落库、删除软删和 `work_fissions` SaaS 用量刷新。
- 2026-07-07 03:47 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、标签建群 dashboard 写链路、客户群发 dashboard 写链路、客户群群发 dashboard 写链路、裂变活动 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 03:56 新增并通过 `env -u GOROOT ./scripts/smoke_greeting_dashboard.sh`；脚本启动独立 MySQL/Redis 和 Go standalone，登录后真实调用 `greeting` 全员/指定员工新增、列表、详情、更新和删除接口，验证 `mc_greeting` 写入、员工去重、素材完整静态 URL 回显、`mc_business_log` 创建/更新日志和迁移路由挂载。
- 2026-07-07 04:01 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、好友欢迎语 dashboard CRUD 写链路、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、标签建群 dashboard 写链路、客户群发 dashboard 写链路、客户群群发 dashboard 写链路、裂变活动 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 04:11 新增并通过 `env -u GOROOT ./scripts/smoke_channel_code_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用渠道活码分组创建/详情/更新/移动、渠道活码新建/更新、列表、详情、客户明细和统计接口，验证 `externalcontact/add_contact_way`、`externalcontact/update_contact_way` 请求体、`mc_channel_code` / `mc_channel_code_group` / `mc_business_log` 写入、标签/员工/客户反查、扫码客户统计和 `channel_codes` SaaS 用量刷新。
- 2026-07-07 04:16 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖三端静态资源、渠道活码 dashboard 写链路、好友欢迎语 dashboard CRUD 写链路、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、标签建群 dashboard 写链路、客户群发 dashboard 写链路、客户群群发 dashboard 写链路、裂变活动 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 04:23 新增并通过 `env -u GOROOT ./scripts/smoke_shop_code_dashboard.sh`；脚本启动独立 MySQL/Redis 和 Go standalone，登录后真实调用门店活码新建、列表、详情、位置、城市/地址检索、分享、页面设置、统计、客户明细、门店统计、员工更新、二维码更新、状态切换、批量打标签和删除接口，验证 `mc_shop_code` / `mc_shop_code_page` / `mc_shop_code_record` 写入、扫码记录统计和 `shop_codes` SaaS 用量刷新。
- 2026-07-07 04:29 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖新增门店活码 dashboard 写链路，并继续覆盖三端静态资源、渠道活码 dashboard 写链路、好友欢迎语 dashboard CRUD 写链路、入群欢迎语 dashboard 写链路、自动拉群 dashboard 写链路、标签建群 dashboard 写链路、客户群发 dashboard 写链路、客户群群发 dashboard 写链路、裂变活动 dashboard 写链路、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 新增并通过 `env -u GOROOT ./scripts/smoke_contact_transfer_dashboard.sh`；脚本启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后真实调用在职转接/离职继承待分配同步、在职客户列表、离职待分配客户列表、待分配群列表、客户接替、群接替和分配记录接口，验证 `externalcontact/get_unassigned_list`、`externalcontact/transfer_customer`、`externalcontact/groupchat/transfer` 请求体，以及 `mc_work_unassigned` / `mc_work_transfer_log` 落库。
- 2026-07-07 06:39 新增并通过 `env -u GOROOT ./scripts/smoke_contact_batch_add_dashboard.sh`；脚本启动独立 MySQL/Redis 和 Go standalone，登录后真实调用批量加好友后台设置、multipart CSV 导入、导入记录、客户列表筛选、统计、提醒、二次分配、单条删除和导入批次删除接口，验证 `mc_contact_batch_add_config` / `mc_contact_batch_add_import_record` / `mc_contact_batch_add_import` / `mc_contact_batch_add_allot` 落库、导入原文件写入 Go storage root、`mochat_go_saas_storage_objects` 账本和 `storage_mb` 回收闭环。
- 2026-07-07 06:48 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖新增批量加好友 dashboard 设置/导入/分配/删除/存储账本回收写链路，并继续覆盖三端静态资源、敏感词、渠道活码、门店活码、互动雷达、抽奖活动、群运营、群发、转接、裂变、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 07:01 新增并通过 `env -u GOROOT ./scripts/smoke_sop_dashboard.sh`；脚本启动独立 MySQL/Redis 和 Go standalone，登录后真实调用个人 SOP 与群 SOP 新建、列表、详情、范围设置、启停、更新和删除接口，验证 SOP 主表 JSON 字段、触达日志级联删除以及 `contact_sops` / `room_sops` SaaS 用量刷新。
- 2026-07-07 07:10 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖新增个人/群 SOP dashboard API 写链路，并继续覆盖三端静态资源、敏感词、渠道活码、门店活码、互动雷达、抽奖活动、群运营、群发、批量加好友、转接、裂变、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 07:23 新增并通过 `env -u GOROOT ./scripts/smoke_auto_tag_dashboard.sh`；脚本启动独立 MySQL/Redis 和 Go standalone，登录后真实调用自动标签三类规则新建、列表、详情、触发记录、启停、删除、会话成员筛选、会话列表和会话存档配置读写接口，验证 `mc_auto_tag` / `mc_auto_tag_record` / `mc_work_message_1` / `mc_corp` 会话存档字段落库。
- 2026-07-07 07:32 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖新增自动标签 dashboard API 写链路，并继续覆盖三端静态资源、敏感词、渠道活码、门店活码、互动雷达、抽奖活动、群运营、群发、批量加好友、转接、裂变、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 07:38 扩展并通过 `env -u GOROOT ./scripts/smoke_official_account_ticket.sh`；脚本在原有开放平台 AES URL 校验、ticket 持久化、取消授权和 `QUERY_AUTH_CODE` 客服消息闭环基础上，新增验证 `authRedirect` 写入授权公众号、调用 `api_get_authorizer_info` 回填资料、写入 `tenant_id`、刷新 `official_accounts` 用量，并覆盖 `officialAccount/index?type=2` 自动模块绑定和 `officialAccount/set?type=3` 手动模块绑定。
- 2026-07-07 07:42 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=saas ./scripts/standalone_acceptance.sh`，`suite=saas` 通过；统一入口已覆盖扩展后的公众号开放平台授权回跳、资料回填、租户归属、模块绑定和原有 SaaS provisioning、租户隔离、额度拦截、存储校准、存储回收、用量刷新、告警通知配置投递链路。
- 2026-07-07 07:51 新增并通过 `env -u GOROOT ./scripts/smoke_admin_core_dashboard.sh`；脚本启动独立 MySQL/Redis 和 Go standalone，登录并绑定企业后真实调用 `common/uploadFile`、`sidebar/common/upload`、素材分组、素材新建/查看/移动/删除、客户群分组、客户画像字段新增/更新/状态/批量/删除、菜单新增/更新/状态/删除、角色新增/更新/授权/状态/删除、子账号新增/详情/更新/状态/重置密码和当前账号改密，并用 MySQL 表状态和 SaaS 上传账本断言副作用。
- 2026-07-07 08:01 复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh`，`suite=frontend` 通过；统一入口已覆盖新增管理端基础写链路 smoke、三端静态资源、敏感词、渠道活码、门店活码、互动雷达、抽奖活动、群运营、SOP、自动标签、欢迎语、群发、批量加好友、转接、裂变、dashboard 页面矩阵、sidebar 登录/客户/SOP/批量加好友/素材库，以及 operation 任务宝 OAuth/code 回调和活动页链路。
- 2026-07-07 08:02 复跑 `env -u GOROOT ./scripts/test.sh` 通过；随后以非缓存方式复跑 `env -u GOROOT go test -count=1 ./internal/dashboard ./internal/store ./internal/server` 通过。直接路由覆盖抽样从 35 个未在 smoke 中直接出现的 manifest 路由降至 5 个，当时剩余为 `favicon.ico`、`/load/{params?}`、`WW_verify_*.txt` 和 `workContactTagGroup/update` 等通用或已迁移旁路。
- 2026-07-07 08:10 扩展并通过 `env -u GOROOT ./scripts/smoke_standalone.sh`、`env -u GOROOT ./scripts/smoke_work_contact_tag_remote_write.sh` 和 `env -u GOROOT ./scripts/smoke_operation_frontend_work_fission.sh`；新增覆盖 `/favicon.ico`、`WW_verify_*.txt`、`PUT /dashboard/workContactTagGroup/update`、`GET /load/{params?}` 和 `POST /load/{params?}`。复扫 manifest 路由 `224/224` 已全部在 `scripts/smoke_*.sh` 中直接出现，`not directly mentioned=0`。
- 2026-07-07 08:27 复跑受影响统一验收分组：`env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=core ./scripts/standalone_acceptance.sh`、`env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=workers ./scripts/standalone_acceptance.sh`、`env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh` 均通过；随后 `env -u GOROOT ./scripts/test.sh`、`env -u GOROOT go test -count=1 ./internal/server ./internal/dashboard ./internal/store ./internal/config`、脚本语法、尾随空白和直接路由覆盖复扫均通过。
- 2026-07-07 08:32 新增并通过 `env -u GOROOT ./scripts/audit_manifest_route_smoke_coverage.sh`，输出 `manifest_routes=224 directly_covered=224 smoke_scripts=69`；该审计已接入 `scripts/test.sh`。
- 2026-07-07 08:35 新增并短时验证 `scripts/standalone_soak_24h.sh`：`env -u GOROOT MOCHAT_SOAK_DURATION_SECONDS=12 MOCHAT_SOAK_INTERVAL_SECONDS=3 MOCHAT_SOAK_MIN_ITERATIONS=3 MOCHAT_SOAK_KEEP_WORK_DIR=0 ./scripts/standalone_soak_24h.sh` 通过，5 轮探测均维持 `routes=224`、`migrated=407`。该脚本默认运行 86400 秒并输出 `soak.ndjson`，短时验证只证明脚本可用，不能替代真实 24 小时稳定性结论。
- 2026-07-07 08:40 启动持续 run：`env -u GOROOT MOCHAT_SOAK_PROJECT=mochat-go-soak-live-20260707-084017 MOCHAT_SOAK_WORK_DIR=/tmp/mochat-go-soak-24h-live-20260707-084017/work MOCHAT_SOAK_LOG=/tmp/mochat-go-soak-24h-live-20260707-084017/soak.ndjson MOCHAT_SOAK_KEEP_WORK_DIR=1 ./scripts/standalone_soak_24h.sh`；2026-07-07 17:10 按用户要求停止，累计 502 轮探测均维持 `routes=224`、`migrated=407`，没有发现早期稳定性异常。
- 2026-07-07 19:07 按用户最新要求不再启动 24 小时 run，改跑全套非 PHP 本地短证据包：`env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过；本次证据写入 `docs/evidence/latest/`，覆盖快速门禁、core、SaaS、workers、cron、frontend、mysql57、运行时路由覆盖、阶段报告和生产证据检查。该证据包使用 `MOCHAT_QUEUE_AUDIT_SOURCE=embedded`、污染的 `MOCHAT_SOURCE_ROOT` / `MOCHAT_COMPAT_MANIFEST` 和 `MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1`，证明短验收不依赖原 PHP 源码或外部 manifest；其中 MySQL 5.7 在本机 arm64 仍仅通过 schema lint，真实容器门禁仍需 amd64 CI。随后补强 `scripts/production_evidence_check.sh`，要求生产证据文件非空并包含核验项关键词，对 MySQL 5.7 证据拒绝 arm64 skip；新增 `.github/workflows/mysql57-amd64.yml`，在 amd64 runner 上执行 `scripts/ci_mysql57_amd64.sh` 并上传可作为 `MOCHAT_EVIDENCE_MYSQL57_AMD64` 的日志 artifact；新增 `scripts/production_candidate_gate.sh`，把全套短验收和严格生产证据检查串成生产候选总门禁；新增 `scripts/init_production_evidence_pack.sh`，生成带防误用标记的生产证据模板，防止模板被误判为真实联调结果。
- 2026-07-07 19:31 新增并通过 `env -u GOROOT ./scripts/smoke_independent_package.sh`，并将其接入 `scripts/standalone_acceptance.sh` 的 core 套件和 `scripts/collect_standalone_evidence.sh`；脚本会复制当前 `mochat-go` 到没有原 `mochat/` 兄弟目录的临时目录，使用 embedded manifest 跑独立性审计、队列覆盖审计和 standalone smoke。随后用 `/tmp/mochat-go-evidence-core-independent` 输出目录复跑 `MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=core ./scripts/collect_standalone_evidence.sh` 通过，快速门禁、独立交付包 smoke、core、路由覆盖、阶段报告和生产证据检查均通过，`224/224` 路由仍完整覆盖；本轮没有启动 24 小时持续运行。
- 2026-07-07 19:57 补强 `scripts/production_evidence_check.sh` 和 `scripts/standalone_stage_report.sh`，把 `scripts/smoke_independent_package.sh` 纳入快速门禁并固定使用 embedded manifest 口径；随后复跑 `env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过，最新证据写入 `docs/evidence/latest/`。本次覆盖快速门禁、独立交付包 smoke、core、SaaS、workers、cron、frontend、mysql57、运行时路由覆盖、阶段报告和生产证据检查；阶段报告显示 smoke 脚本 `71` 个且 `standalone_acceptance.sh` 已引用 `71` 个，路由仍为 `224/224`，未发现 24 小时持续运行进程或正在运行的 `mochat-go` Docker 容器。
- 2026-07-07 新增 `scripts/audit_goal_completion.sh`，把用户目标拆成独立 Go 可运行、不依赖原 mochat/PHP、全部 manifest 功能本地迁移、SaaS/worker/cron/frontend 本地闭环、生产外部证据和可选 24 小时稳定性证据逐条审计；默认只报告，严格模式 `MOCHAT_GOAL_COMPLETION_STRICT=1` 在目标未完成时返回非 0，`MOCHAT_GOAL_COMPLETION_REQUIRE_24H=1` 可把满 24 小时 soak 纳入完成条件。该审计已接入 `scripts/collect_standalone_evidence.sh` 和 `scripts/production_candidate_gate.sh`，避免把局部 green check 误判为最终完成。
- 2026-07-07 20:41 新增并通过 `scripts/smoke_production_evidence_gate.sh`，验证生产证据模板会被拒绝、MySQL 5.7 arm64 skip 日志会被拒绝、含必填核验项和明确通过结论的证据文件才会通过；该脚本已接入 `scripts/test.sh` 和 `scripts/standalone_acceptance.sh` core 套件。本轮复跑 `env -u GOROOT ./scripts/test.sh` 通过，输出 `smoke_scripts=72 referenced=72`、manifest 路由 `224/224` 直接覆盖、功能模块 `28` 个通过；随后复跑 `env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=core MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1 ./scripts/standalone_acceptance.sh` 通过。本轮未启动 24 小时持续运行，且未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 2026-07-07 21:07 补强 `scripts/audit_goal_completion.sh`：目标完成度审计内部改用严格生产证据检查，因此在缺真实外部证据时 `当前门禁结果` 会明确显示 `失败 production evidence check`，默认报告仍可返回 0 供阶段交接使用，严格模式仍返回非 0。随后复跑 `env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过，最新证据包写入 `docs/evidence/latest/`：快速本地门禁、独立交付包、core、SaaS、workers、cron、frontend、mysql57、运行时路由覆盖、阶段报告、生产证据检查和目标完成度审计全部生成；路由仍为 `224/224`、缺失 `0`，目标完成度审计仍判定 `目标未完成，继续保留生产证据缺口`。本轮未启动 24 小时持续运行。
- 2026-07-07 继续补强 `scripts/collect_standalone_evidence.sh` 的汇总页语义：`docs/evidence/latest/index.md` 会把 `production-evidence` 和 `goal-completion` 标为“报告生成”，并新增关键结论小节引用 `production-evidence.md` 与 `goal-completion.md` 的 `当前结论`，避免把报告命令返回 0 误读成生产证据或目标完成。
- 2026-07-07 21:32 复跑 `env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过，并刷新 `docs/evidence/latest/index.md`：首页当前结论为 `本地短门禁通过，目标未完成，仍需补生产证据`，`production-evidence` 和 `goal-completion` 均显示为 `报告生成`，关键结论分别为 `仍缺生产证据，不能标记最终完成` 和 `目标未完成，继续保留生产证据缺口`。本次路由仍为 `224/224`、缺失 `0`，且未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 2026-07-07 继续补强 `scripts/production_candidate_gate.sh` 的候选报告：首页新增关键结论小节，直接摘取严格生产证据检查和严格目标完成度审计的 `当前结论`。后续生产候选失败时可以一眼区分是本地短验收失败，还是外部生产证据/目标完成度未满足。
- 2026-07-07 21:35 验证生产候选总门禁缺证失败路径：`env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh` 返回 `rc=1`，候选报告写入 `docs/evidence/production/candidate/index.md`，当前结论为 `未通过，不能标记最终完成`；关键结论显示严格生产证据检查 `仍缺生产证据，不能标记最终完成`，严格目标完成度审计 `目标未完成，继续保留生产证据缺口`。报告同时确认 24 小时持续运行未启动，且未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 2026-07-07 22:01 新增 `scripts/source_fingerprint.py`，为 Go 源码、验收脚本、部署配置、前端构建产物和关键构建文件生成 sha256 指纹，并接入 `scripts/collect_standalone_evidence.sh` 与 `scripts/audit_goal_completion.sh`。旧 latest 证据包缺少 `source-fingerprint.json` 时，目标完成度审计会把 `全部 manifest 功能已纳入 Go 本地验收` 和 `SaaS、worker、cron、frontend 本地闭环` 判为未完成；随后复跑 `env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过，`docs/evidence/latest/index.md` 显示源码与验收指纹 `443ee12dfdacaf7e1cffd0968e55f72bab6336d4c3c194abcbdeb4802b9e6cfe`、文件数 `589`，目标完成度审计显示 latest 源码指纹与当前源码指纹匹配为 `True`。本次路由仍为 `224/224`、缺失 `0`，且未启动 24 小时持续运行。
- 2026-07-07 22:55 继续补强 `scripts/production_candidate_gate.sh`：生产候选总门禁新增 `源码与验收指纹匹配` 命令和报告区块，默认本地短验收模式会对比 `docs/evidence/production/candidate/local/source-fingerprint.json`，`MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1` 时会对比 `docs/evidence/latest/source-fingerprint.json`，防止用旧证据包证明新代码。改动后旧 latest 指纹过期，已复跑 `env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过，latest 指纹刷新为 `3dd6b244f3867185a129e09b21b954e6549c78c12e068bdb8ecbaa89d3da79d8`、文件数 `589`，路由仍为 `224/224`、缺失 `0`。随后复跑 `env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh` 返回 `rc=1`，其中严格生产证据检查和严格目标完成度审计按预期失败，新增源码指纹匹配通过；本轮未启动 24 小时持续运行，且未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 2026-07-07 23:23 修正目标完成度审计的证据目录口径：`scripts/audit_goal_completion.sh` 新增 `MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR`，`scripts/collect_standalone_evidence.sh` 会把当前输出目录传给目标审计，`scripts/production_candidate_gate.sh` 默认模式会让严格目标审计读取 `docs/evidence/production/candidate/local`，避免生产候选门禁被旧 `docs/evidence/latest` 误影响。随后复跑 `env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过，latest 指纹刷新为 `8ee5729c11c7e5a6c561da445602d355e2ef02c830208b6c537c89675d04b396`、文件数 `589`，目标完成度审计显示本地短证据包为 `docs/evidence/latest`、源码指纹匹配 `True`，路由仍为 `224/224`、缺失 `0`。复跑 `env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh` 返回 `rc=1`，严格生产证据检查和严格目标完成度审计仍因真实外部生产证据缺失失败，源码指纹匹配通过；本轮未启动 24 小时持续运行，且候选报告显示无 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- `scripts/standalone_stack_check.sh` 已验证独立 MySQL/Redis、Go standalone `/readyz`、dashboard/sidebar/operation 前端入口和未知路由 501。
- `Dockerfile` 和 `deploy/standalone/docker-compose.yml` 的 `app` profile 已补齐完整容器独立栈，可构建并启动 Go app + MySQL + Redis，不启动 PHP，也不挂载原项目。
- `scripts/smoke_standalone_compose_app.sh` 已用于验证完整容器栈中的 Go standalone `/readyz`、内置 manifest、dashboard/sidebar/operation 前端入口、容器内 `mochat-migrate baseline`、`mochat-bootstrap`、真实 `POST /dashboard/user/auth` 登录、`loginShow`、`permissionByUser`、`corp/select`、`corp/bind`、Redis 企业选择缓存、`corpData/index`、`corpData/lineChat`、`workEmployee/searchCondition`、`workDepartment/index` 和 `workEmployee/index` 闭环。
- `scripts/smoke_frontend_static_browser.sh` 已用真实浏览器验证 Go standalone 托管的 dashboard `/login`、sidebar `/contact` 和 operation `/workFission` 三套内置前端入口可加载同源 JS/CSS 资源，不依赖原前端目录或外部静态代理。
- `scripts/smoke_admin_core_dashboard.sh` 已覆盖管理端基础写链路：dashboard/sidebar 上传、素材分组、素材新建/查看/移动/删除、客户群分组、客户画像字段新增/更新/状态/批量/删除、菜单新增/更新/状态/删除、角色新增/更新/授权/状态/删除、子账号新增/详情/更新/状态/重置密码和当前账号改密，并用独立 Go 栈的 MySQL 表状态和 SaaS 上传账本断言副作用。
- `scripts/smoke_dashboard_frontend_login.sh` 已改为直接调用 Node `playwright` 模块，不再依赖自定义 `playwright-cli`、PHP upstream、原 MoChat 源码或外部 manifest，并已在 Go standalone 下覆盖 Go 托管 dashboard 登录、`permissionByUser`、`corp/select`、`loginShow`、`corp/bind`，以及 `/corpData/index`、`/corp/index`、`/user/index`、`/passwordUpdate/index`、`/role/index`、`/role/permissionShow?roleId=910001`、`/menu/index`、`/department/index`、`/workEmployee/index`、`/workContact/index`、`/workContact/contactFieldPivot?contactId=910001&employeeId=2&isContact=1`、`/lossContact/index`、`/workContactTag/index`、`/workRoom/index`、`/workRoom/detail?workRoomId=910001`、`/workRoom/statistics?workRoomId=910001`、`/channelCode/index`、`/channelCode/statistics?channelCodeId=910001`、`/channelCode/store`、`/mediumGroup/index`、`/greeting/index`、`/greeting/store`、`/roomWelcome/index`、`/roomWelcome/create`、`/workRoomAutoPull/index`、`/workRoomAutoPull/store`、`/roomTagPull/index`、`/roomTagPull/create`、`/roomTagPull/detail?id=917001`、`/roomTagPull/contactDetail?id=917001`、`/autoTag/keywordIndex`、`/autoTag/keywordCreate`、`/autoTag/keywordShow?idRow=918001`、`/autoTag/joinRoomIndex`、`/autoTag/joinRoomCreate`、`/autoTag/joinRoomShow?idRow=918002`、`/autoTag/dayPartIndex`、`/autoTag/dayPartCreate`、`/autoTag/dayPartShow?idRow=918003`、`/contactMessageBatchSend/index`、`/contactMessageBatchSend/store`、`/contactMessageBatchSend/show?batchId=915001`、`/roomMessageBatchSend/index`、`/roomMessageBatchSend/store`、`/roomMessageBatchSend/show?batchId=916001`、`/workFission/taskpage`、`/workFission/create`、`/workFission/edit?id=985001`、`/workFission/invite?id=985001`、`/workFission/dataShow?id=985001`、`/officialAccount/index`、`/officialAccount/create`、`/contactField/index`、`/chatTool/customer`、`/chatTool/enhance`、`/statistics/contact`、`/statistics/employee`、`/contactTransfer/resignIndex`、`/contactTransfer/workIndex`、`/contactTransfer/workAllotRecord` 和 `/contactTransfer/resignAllotRecord` 共 61 个 dashboard 业务页面，并额外覆盖 Go 原生 `/dashboard/sensitiveWords/page` 敏感词管理页、`/dashboard/lottery/page` 抽奖活动页、`/dashboard/radar/page` 互动雷达页、`/dashboard/shopCode/page` 门店活码页、`/dashboard/contactSop/page` 个人 SOP 页、`/dashboard/roomSop/page` 群 SOP 页、`/dashboard/roomFission/page` 群裂变页、`/dashboard/roomQuality/page` 群质检页、`/dashboard/roomCalendar/page` 群日历页、`/dashboard/roomRemind/page` 客户群提醒页、`/dashboard/roomInfinitePull/page` 无限拉群页、`/dashboard/roomClockIn/page` 群打卡页和 `/dashboard/saasAlert/page` SaaS 告警管理页；脚本会等待对应 Go dashboard API 返回 200，并把页面渲染到 `/404`、同源 dashboard 接口 4xx/5xx、非导航取消类本地请求失败或本地静态资源 4xx/5xx 视为失败。
- `scripts/standalone_inventory_parity.sh` 已验证 Go 内置清单与本地原 PHP 项目扫描结果一致：224 条路由、70 张表、9 个定时任务、10 个事件处理器、15 条队列注解。
- `scripts/smoke_bootstrap_standalone.sh`、`scripts/smoke_saas_provisioning.sh`、`scripts/smoke_saas_tenant_isolation.sh`、`scripts/smoke_saas_quota_enforcement.sh`、`scripts/smoke_saas_storage_reconcile.sh`、`scripts/smoke_saas_storage_reclaim.sh`、`scripts/smoke_saas_usage_refresh.sh`、`scripts/smoke_official_account_ticket.sh` 已通过，证明空库初始化、多租户开通、跨租户企业绑定隔离、脏 Redis 企业选择缓存读路径隔离、角色/权限读路径按租户隔离、双租户上传文件账本与 `storage_mb` 用量分离、套餐额度拦截、存储账本、存储回收、用量刷新、公众号开放平台 AES 加密 GET URL 校验、AES 加密 ticket 持久化、签名拒绝、取消授权隐藏并释放 `official_accounts` 用量、授权回跳写入租户归属和公众号资料、模块公众号自动/手动绑定，以及 AES 加密 `QUERY_AUTH_CODE` 客服消息闭环可在 Go standalone 下运行。
- `scripts/smoke_queue_idempotency.sh`、`scripts/smoke_async_file_upload_worker.sh`、`scripts/smoke_async_file_upload_saas_usage.sh`、`scripts/smoke_mark_tags_worker.sh`、`scripts/smoke_message_remind_worker.sh`、`scripts/smoke_work_room_sync_worker.sh`、`scripts/smoke_work_contact_sync_worker.sh`、`scripts/smoke_work_department_list_worker.sh`、`scripts/smoke_media_id_update_worker.sh`、`scripts/smoke_employee_statistic_worker.sh`、`scripts/smoke_employee_apply_worker.sh`、`scripts/smoke_wework_callback_worker.sh` 已通过，证明 Redis 队列幂等、AsyncFileUpload 文件异步上传 worker、AsyncFileUpload SaaS 异步执行量/存储用量/超额告警按双租户隔离、MarkTags 客户打标签 worker、MessageRemind 企业应用消息提醒 worker、WorkRoomSync 客户群同步 worker、WorkContactSync 客户同步 worker、WorkDepartmentList 部门/成员同步 worker、MediaIdUpdate 素材临时 media_id 更新 worker、EmployeeStatisticApply 成员统计 worker、企业授权后通讯录同步 worker、企微回调 worker 和 SaaS 告警链路可独立运行。
- `scripts/smoke_work_contact_tag_remote_write.sh` 已通过，证明客户标签组本地改名、客户标签创建、同组改名、移动、删除和标签组删除会在 Go standalone 下调用 fake 企业微信 `add_corp_tag/edit_corp_tag/del_corp_tag`，并回写本地 `wx_group_id/wx_contact_tag_id`。
- 13 个定时/后台发送 smoke 已通过，其中包含 9 个 PHP crontab 基线、2 个群发定时发送链路和 2 个会话存档链路：`pullAgent`、`employeeStatistic`、`channelCode`、`ContactMessageBatchSendQueue/SendJob`、`RoomMessageBatchSendQueue/SendJob`、`ContactSyncSendResultTask`、`RoomSyncSendResultTask`、`RoomTagPull`、`corpData`、`mediaIdUpdate`、`TransferStateRefresh`、`workMessageArchiveSync`、`sensitiveWordsMonitor`。
- `env -u GOROOT go test -race ./...` 已通过，补充了全包并发测试信号。
- `scripts/smoke_mysql57_schema_migrate.sh` 在本机 arm64 按脚本策略跳过真实容器 smoke；MySQL 5.7 静态 lint 已通过，真实 MySQL 5.7 仍应在 amd64 CI 跑。
- 上述门禁证明当前独立 Go 底座、已迁移路由挂载和基础 SaaS 租户隔离成立，但还不能替代真实企业微信/微信开放平台账号、生产前端多页面和跨租户数据的全量业务验收。

## 当前已完成的独立化基础

- `MOCHAT_GO_STANDALONE=1` 已可启动 Go 服务。
- `/readyz` 在 standalone 模式下不再检查 `../mochat`、外部 manifest 或 PHP upstream。
- `/compat/routes` 使用编译进 Go 模块的内置迁移清单。
- `/static/*` 已由 Go 服务从 `MOCHAT_FILE_STORAGE_ROOT` 只读托管，本地上传资源、渠道码二维码、自动拉群二维码、标签建群头像等不再需要 PHP 或外部静态代理。
- 未迁移路由在 standalone 模式下返回 `501 route not yet migrated in standalone Go runtime`。
- 文件上传默认目录从 `../mochat/api-server/storage/upload/static` 改为 `./storage/upload/static`。
- 新增 `scripts/smoke_standalone.sh`，用于验证上述独立运行门槛；该脚本会故意注入无效 PHP upstream 和不存在的原项目/manifest 路径，防止 standalone 被 shell 环境残留污染。
- 数据库建表 SQL 已复制到 `deploy/standalone/schema/mochat.sql`，核心默认数据 seed 已拆到 `deploy/standalone/migrations/0002_seed_core_data.up.sql`。
- 新增 `cmd/mochat-bootstrap` 和 `scripts/smoke_bootstrap_standalone.sh`，用于在迁移后的空库创建默认租户、超级管理员、管理员角色、用户角色绑定和可用菜单权限绑定，并验证该账号可通过 Go standalone `auth` 换取 token。
- 新增 `deploy/standalone/docker-compose.yml`，默认可按需只启动 MySQL/Redis；启用 `--profile app` 时可同时启动 Go app 容器，不启动 PHP，也不挂载原 `../mochat`。
- 新增 `scripts/standalone_stack_check.sh`，用于验证独立依赖栈、内置 SQL、Redis 和 Go standalone 服务。
- 新增 `scripts/standalone_inventory_parity.sh`，用于在迁移工作区从 PHP 原项目重新扫描路由、表、定时任务、事件处理器和队列注解，并与 Go 内置 manifest 对比，防止迁移清单漏收原 PHP 功能。
- 新增 `scripts/audit_queue_annotation_coverage.sh`，用于把 PHP 队列注解逐项映射到 Go 语义队列，当前覆盖 `wework-callback`、`employee-apply`、`contact-welcome`、`async-file-upload`、`mark-tags`、`message-remind`、`work-room-sync`、`work-contact-sync`、`work-department-list`、`medium-media-id-update` 和 `employee-statistic-apply`。
- 新增 `scripts/standalone_route_coverage.sh`，用于在 standalone + 独立 MySQL/Redis/JWT secret 下验证当前所有已迁移业务路由会默认挂载，并审计 `/compat/routes` 中仍未迁移的路由。
- 新增 `MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER=1` 和 `scripts/smoke_wework_callback_worker.sh`，用于验证企微回调 Redis 队列能在独立 Go 栈内被消费，并触发通讯录同步落库。
- 新增 `MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER=1` 和 `scripts/smoke_employee_apply_worker.sh`，用于验证企业授权后的 `EmployeeApply` 通讯录同步队列能在独立 Go 栈内被消费。
- 新增 `MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER=1` 和 `scripts/smoke_async_file_upload_worker.sh`，用于验证 PHP `Common\QueueService\AsyncFileUpload` 文件异步上传队列能在独立 Go 栈内消费旧数组 payload，把 URL 或本地临时文件写入 `MOCHAT_FILE_STORAGE_ROOT` 下的目标相对路径，按 `unlink` 删除本地源文件，并在结构化 payload 带 `tenantId/corpId` 时写入 SaaS 存储账本。
- 新增 `MOCHAT_GO_ENABLE_MARK_TAGS_WORKER=1` 和 `scripts/smoke_mark_tags_worker.sh`，用于验证 PHP `WorkContact\QueueService\Tag\MarkTags` 客户打标签异步队列能在独立 Go 栈内消费位置参数 payload，过滤已有标签、写入本地标签 pivot、记录客户互动轨迹并调用企业微信 `externalcontact/mark_tag`。
- 新增 `MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER=1` 和 `scripts/smoke_message_remind_worker.sh`，用于验证 PHP `WorkAgent\QueueService\MessageRemind` 消息提醒队列能在独立 Go 栈内消费对象 payload，读取 `mc_work_agent` 提醒应用并调用企业微信 `message/send`。
- 新增 `MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER=1` 和 `scripts/smoke_work_room_sync_worker.sh`，用于验证 PHP `WorkRoom\QueueService\UpdateApply/UpdateCallback` 客户群同步队列能在独立 Go 栈内消费 `corpId` 或回调对象 payload，拉取企业微信客户群列表/详情并写入客户群和群成员。
- 新增 `MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER=1` 和 `scripts/smoke_work_contact_sync_worker.sh`，用于验证 PHP `WorkContact\QueueService\SyncContactApply/AdminSynContactApply` 客户同步队列能在独立 Go 栈内消费单成员或多成员旧数组 payload，拉取企业微信客户列表/详情，写入客户、员工客户关系和企业标签，并标记已经不存在的员工客户关系。
- 新增 `MOCHAT_GO_ENABLE_WORK_DEPARTMENT_LIST_WORKER=1` 和 `scripts/smoke_work_department_list_worker.sh`，用于验证 PHP `WorkDepartment\QueueService\ListApply` 部门列表同步队列能在独立 Go 栈内消费旧数组 payload，复用通讯录同步链路拉取企业微信部门、成员和外部联系人权限成员并落库。
- 新增 `MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_WORKER=1` 和 `scripts/smoke_media_id_update_worker.sh`，用于验证 PHP `Medium\Queue\MediaIdUpdateQueue::handle(corpId, mediumIds)` 素材临时 `media_id` 更新队列能在独立 Go 栈内消费旧数组 payload，上传本地素材文件并写回 `mc_medium.media_id/last_upload_time`。
- 新增 `MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_WORKER=1` 和 `scripts/smoke_employee_statistic_worker.sh`，用于验证 PHP `WorkEmployee\QueueService\EmployeeStatisticApply::handle()` 成员统计队列能在独立 Go 栈内兼容旧空数组 payload，并支持结构化 `corpId/tenantId` payload 限定统计企业范围，拉取前一天企业微信成员统计、写入 `mc_work_employee_statistic`、设置 Redis 去重 key，并在带租户上下文时刷新 `async_executions` 用量。
- 企微回调、`EmployeeApply`、`contact-welcome`、`AsyncFileUpload`、`MarkTags`、`MessageRemind`、`WorkRoomSync`、`WorkContactSync`、`WorkDepartmentList`、`MediaIDUpdate` 和 `EmployeeStatisticApply` Redis worker 已具备 processing 队列、成功 ack、失败重试、dead-letter 和 processing 超时恢复；`contact-welcome` 负责接管 PHP `WorkContact\QueueService\SendWelcome` 欢迎语异步发送队列，新增客户回调已按 PHP listener 优先级支持渠道码专属欢迎语优先、自动拉群 `State=workRoomAutoPullId-{id}` 欢迎语次之、通用欢迎语兜底。渠道码新增客户打标签 listener 已读取 `mc_channel_code.tags`；自动拉群欢迎语会读取 `mc_work_room_auto_pull.leading_words/rooms`，按房间配置上限和 `mc_work_room.room_max` 跳过满群，并发送首个可用群二维码图片；自动拉群新增客户打标签 listener 已读取 `mc_work_room_auto_pull.tags`。企微裂变 `State=fission-{id}` 新增客户打标签 listener 已读取 `mc_work_fission.contact_tags`，裂变完成后会按 `push_employee` 发送员工提醒、按 `push_contact` 通过企业微信 `externalcontact/add_msg_template` 给完成任务的上级客户创建客户群发消息；删除裂变客户时会标记参与人 `loss=1`，并对上级 `invite_count` 做非负回退。删除/流失客户事件会按 PHP 文案通过企业应用给对应员工发送提醒。三类打标签都复用本地 `mc_work_contact_tag_pivot.type=1`、客户互动轨迹和企业微信 `externalcontact/mark_tag` 同步链路；`MarkTags` worker 进一步接管手动/异步打标签队列的同一套写入和企微同步语义；`MessageRemind` worker 进一步接管通用企业应用提醒队列，支持 user/party/tag 接收方、文本内容和带 `media_id/path` 的媒体内容；`WorkRoomSync` worker 进一步接管客户群主动同步和客户群创建/更新回调同步队列；`WorkContactSync` worker 进一步接管客户主动同步和管理员批量客户同步队列；`WorkDepartmentList` worker 进一步接管部门列表同步队列；`MediaIDUpdate` worker 进一步接管素材临时 media_id 更新队列；`EmployeeStatisticApply` worker 进一步接管成员统计队列。
- 新增 `internal/taskrunner` 统一后台任务运行器，现有 `wework-callback`、`contact-welcome` 和 `employee-apply` worker 已通过同一 task group 启动；`/compat/status` 会返回带 `run_id` 的 `background_tasks`，worker smoke 已断言对应任务为 `running`。
- 新增 `MOCHAT_GO_ENABLE_PULL_AGENT_CRON=1` 和 `scripts/smoke_pull_agent_cron.sh`，用于验证 PHP `pullAgent` 企业微信应用同步定时任务已可在独立 Go 栈内刷新 `mc_work_agent` 应用详情。
- 新增 `MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_CRON=1` 和 `scripts/smoke_employee_statistic_cron.sh`，用于验证 PHP `employeeStatistic` 成员统计拉取定时任务已可在独立 Go 栈内拉取前一天企业微信成员统计、写入 `mc_work_employee_statistic` 并设置 Redis 去重 key。
- 新增 `MOCHAT_GO_ENABLE_CHANNEL_CODE_CRON=1` 和 `scripts/smoke_channel_code_cron.sh`，用于验证 PHP `channelCode` 渠道码联系我方式更新定时任务已可在独立 Go 栈内刷新企业微信 contact_way 并写回 `mc_channel_code` 二维码配置。
- 新增 `MOCHAT_GO_ENABLE_CONTACT_BATCH_SEND_CRON=1` 和 `scripts/smoke_contact_batch_send_cron.sh`，用于验证 PHP `ContactMessageBatchSendQueue/SendJob` 客户群发定时发送队列已可在独立 Go 栈内扫描到期批次、创建发送任务并提交企业微信。
- 新增 `MOCHAT_GO_ENABLE_ROOM_BATCH_SEND_CRON=1` 和 `scripts/smoke_room_batch_send_cron.sh`，用于验证 PHP `RoomMessageBatchSendQueue/SendJob` 客户群群发定时发送队列已可在独立 Go 栈内扫描到期批次、创建发送任务并提交企业微信。
- 新增 `MOCHAT_GO_ENABLE_CONTACT_SYNC_SEND_RESULT_CRON=1` 和 `scripts/smoke_contact_sync_send_result_cron.sh`，用于验证 PHP `ContactSyncSendResultTask` 客户群发结果同步定时任务已可在独立 Go 栈内同步最近一周成员发送状态、客户接收结果和批次汇总统计。
- 新增 `MOCHAT_GO_ENABLE_ROOM_SYNC_SEND_RESULT_CRON=1` 和 `scripts/smoke_room_sync_send_result_cron.sh`，用于验证 PHP `RoomSyncSendResultTask` 客户群群发结果同步定时任务已可在独立 Go 栈内同步最近一周成员发送状态、群接收结果和批次汇总统计。
- 新增 `MOCHAT_GO_ENABLE_ROOM_TAG_PULL_CRON=1` 和 `scripts/smoke_room_tag_pull_cron.sh`，用于验证 PHP `RoomTagPull` 标签建群结果同步定时任务已可在独立 Go 栈内同步 `wx_tid.status` 和 `mc_room_tag_pull_contact.send_status`。
- 新增 `MOCHAT_GO_ENABLE_CORP_DATA_CRON=1` 和 `scripts/smoke_corp_data_cron.sh`，用于验证 PHP `corpData` 首页数据统计定时任务已可在独立 Go 栈内刷新 `mc_corp_day_data` 和 `mc_work_update_time.type=6`。
- 新增 `MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_CRON=1` 和 `scripts/smoke_media_id_update_cron.sh`，用于验证 PHP `mediaIdUpdate` 素材库临时 `media_id` 更新定时任务已可在独立 Go 栈内上传本地素材文件并写回 `mc_medium`。
- 新增 `MOCHAT_GO_ENABLE_TRANSFER_STATE_REFRESH_CRON=1` 和 `scripts/smoke_transfer_state_refresh_cron.sh`，用于验证 PHP `TransferStateRefresh` 分配状态刷新定时任务已可在独立 Go 栈内按 Redis `log_id` 游标查询企业微信转接结果并写回 `mc_work_transfer_log.state`。
- 新增 `MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON=1` 和 `scripts/smoke_sensitive_word_monitor_cron.sh`，用于验证敏感词监控已可在独立 Go 栈内扫描会话存档分表、命中启用词库、写入 `mc_sensitive_words_monitor` 并维护 `mc_work_message_id.type=21..30` 分表游标。
- 新增 `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=1`、`MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL`、`MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN` 和 `scripts/smoke_work_message_archive_sync_cron.sh`，用于验证 Go standalone 可从内部官方 SDK bridge 拉取已解密会话消息，按 `seq` 分表写入 `mc_work_message_1` 至 `mc_work_message_10`，维护 `mc_work_message_id.type=40` 拉取游标，并让敏感词监控继续消费这些消息。
- 新增 `MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON=1`，用于把 SaaS 上传存储账本校准接入 Go 后台任务运行器；`scripts/smoke_saas_storage_reconcile.sh` 已验证手动维护命令和 run-on-start 定时任务都能修正账本并刷新 `storage_mb` 用量。
- 新增 `web/dashboard/dist`、`MOCHAT_DASHBOARD_DIST` 和 Go dashboard 静态托管层；`scripts/smoke_dashboard_frontend_login.sh` 已改为直接访问 Go 服务 `/login`，不再依赖 `../mochat/dashboard/dist` 或外部静态代理。
- 新增 `MOCHAT_SIDEBAR_DIST`、`MOCHAT_OPERATION_DIST`、`MOCHAT_GO_ENABLE_FRONTEND_SERVERS`、`MOCHAT_SIDEBAR_FRONTEND_ADDR`、`MOCHAT_OPERATION_FRONTEND_ADDR`，Go 进程现在既可在主监听地址通过 `/sidebar-app/`、`/operation-app/` 前缀托管 sidebar/operation 前端产物，也可用独立端口托管这两个入口；standalone smoke 已显式开启并覆盖三个前端入口，`scripts/smoke_frontend_static_browser.sh` 继续用真实浏览器验证三套 dist 的本地资源加载。
- 后台账号管理接口已迁移到 Go：
  - `GET /dashboard/user/index`
  - `GET /dashboard/user/show`
  - `POST /dashboard/user/store`
  - `PUT /dashboard/user/update`
  - `PUT /dashboard/user/statusUpdate`
  - `PUT /dashboard/user/passwordReset`
  - `PUT /dashboard/user/passwordUpdate`
- 客户/成员统计接口已迁移到 Go：
  - `GET /dashboard/statistic/index`
  - `GET /dashboard/statistic/topList`
  - `GET /dashboard/statistic/employeeCounts`
  - `GET /dashboard/statistic/employees`
  - `GET /dashboard/statistic/employeesTrend`
- 通用上传接口已迁移到 Go：
  - `POST /dashboard/common/upload`
  - `POST /dashboard/common/uploadFile`
  - `POST /sidebar/common/upload`
- 客户画像编辑接口已迁移到 Go：
  - `PUT /dashboard/contactFieldPivot/update`
  - `PUT /sidebar/contactFieldPivot/update`
- 角色/菜单 RBAC 写接口已迁移到 Go：
  - `POST /dashboard/role/store`
  - `PUT /dashboard/role/update`
  - `PUT /dashboard/role/statusUpdate`
  - `DELETE /dashboard/role/destroy`
  - `POST /dashboard/role/permissionStore`
  - `POST /dashboard/menu/store`
  - `PUT /dashboard/menu/update`
  - `PUT /dashboard/menu/statusUpdate`
  - `DELETE /dashboard/menu/destroy`
- 客户资料卡字段写接口已迁移到 Go：
  - `POST /dashboard/contactField/store`
  - `PUT /dashboard/contactField/update`
  - `PUT /dashboard/contactField/statusUpdate`
  - `DELETE /dashboard/contactField/destroy`
  - `PUT /dashboard/contactField/batchUpdate`
- 客户标签/标签组本地写接口已迁移到 Go：
  - `POST /dashboard/workContactTagGroup/store`
  - `PUT /dashboard/workContactTagGroup/update`
  - `DELETE /dashboard/workContactTagGroup/destroy`
  - `POST /dashboard/workContactTag/store`
  - `PUT /dashboard/workContactTag/update`
  - `DELETE /dashboard/workContactTag/destroy`
  - `PUT /dashboard/workContactTag/move`
  - `PUT /dashboard/workContactTag/synContactTag`
- 客户标签/标签组写接口已补齐 PHP 对企业微信标签 API 的远端副作用：创建标签会调用 `externalcontact/add_corp_tag` 并回写 `wx_group_id/wx_contact_tag_id`；同组改名会调用 `externalcontact/edit_corp_tag`；标签移动会先删除原企微标签再在目标分组重建；删除标签或标签组会调用 `externalcontact/del_corp_tag`，并同步清理本地 pivot 和空标签组。
- 新增 `scripts/smoke_work_contact_tag_remote_write.sh`，用于在独立 Go + MySQL + Redis + fake WeCom 栈内验证上述客户标签远端写副作用。
- 企微客户同步接口已迁移到 Go：
  - `PUT /dashboard/workContact/synContact`
- 企微客户群同步接口已迁移到 Go：
  - `PUT /dashboard/workRoom/syn`
- 企微成员/部门同步接口已迁移到 Go：
  - `PUT /dashboard/workEmployee/synEmployee`
- 渠道活码主表读写接口已迁移到 Go：
  - `GET /dashboard/channelCode/index`
  - `GET /dashboard/channelCode/show`
  - `GET /dashboard/channelCode/contact`
  - `GET /dashboard/channelCode/statistics`
  - `GET /dashboard/channelCode/statisticsIndex`
  - `POST /dashboard/channelCode/store`
  - `PUT /dashboard/channelCode/update`
- 渠道活码分组读写接口已迁移到 Go：
  - `GET /dashboard/channelCodeGroup/index`
  - `GET /dashboard/channelCodeGroup/detail`
  - `POST /dashboard/channelCodeGroup/store`
  - `PUT /dashboard/channelCodeGroup/update`
  - `PUT /dashboard/channelCodeGroup/move`
  - `scripts/smoke_channel_code_dashboard.sh` 已覆盖独立 Go 栈内分组创建/详情/更新/移动、渠道活码新建/更新/列表/详情/客户明细/统计、企业微信 `externalcontact/add_contact_way` / `externalcontact/update_contact_way` 请求和 `channel_codes` SaaS 用量刷新。
  - `scripts/smoke_shop_code_dashboard.sh` 已覆盖独立 Go 栈内门店活码新建、列表、详情、位置、城市/地址检索、分享、页面设置、统计、客户明细、门店统计、员工更新、二维码更新、状态切换、批量打标签、删除和 `shop_codes` SaaS 用量刷新。
- 侧边栏企业应用 OAuth/JSSDK 接口已迁移到 Go：
  - `GET /sidebar/agent/auth`
  - `POST /sidebar/agent/auth`
  - `GET /sidebar/agent/oauth`
  - `GET /sidebar/agent/jssdkConfig`
  - `GET /sidebar/wxJsSdk/config`
- 客户列表接口已迁移到 Go：
  - `GET /dashboard/workContact/index`
  - `GET /dashboard/workContact/lossContact`
- 素材库分组接口已迁移到 Go：
  - `GET /dashboard/mediumGroup/index`
  - `POST /dashboard/mediumGroup/store`
  - `PUT /dashboard/mediumGroup/update`
  - `DELETE /dashboard/mediumGroup/destroy`
  - `GET /sidebar/mediumGroup/index`
- 素材库主体本地接口已迁移到 Go：
  - `GET /dashboard/medium/index`
  - `GET /dashboard/medium/show`
  - `POST /dashboard/medium/store`
  - `PUT /dashboard/medium/update`
  - `DELETE /dashboard/medium/destroy`
  - `PUT /dashboard/medium/groupUpdate`
  - `GET /sidebar/medium/index`
  - `GET /sidebar/medium/mediaIdUpdate`
- 客户群分组接口已迁移到 Go：
  - `GET /dashboard/workRoomGroup/index`
  - `POST /dashboard/workRoomGroup/store`
  - `PUT /dashboard/workRoomGroup/update`
  - `DELETE /dashboard/workRoomGroup/destroy`
- 客户群统计接口已迁移到 Go：
  - `GET /dashboard/workRoom/index`
  - `GET /dashboard/workRoom/roomIndex`
  - `GET /dashboard/workRoom/statistics`
  - `GET /dashboard/workRoom/statisticsIndex`
  - `GET /sidebar/workRoom/roomManage`
- 自动拉群接口已迁移到 Go：
  - `GET /dashboard/workRoomAutoPull/index`
  - `GET /dashboard/workRoomAutoPull/show`
  - `POST /dashboard/workRoomAutoPull/store`
  - `PUT /dashboard/workRoomAutoPull/update`
  - `PUT /dashboard/workRoomAutoPull/move`
  - `scripts/smoke_work_room_auto_pull_dashboard.sh` 已覆盖独立 Go 栈内自动拉群新增、查看、更新、兼容 move，以及企业微信 `contact_way/create`、`contact_way/update` 请求和 SaaS 用量刷新。
- 标签建群第一批接口已迁移到 Go：
  - `GET /dashboard/roomTagPull/index`
  - `GET /dashboard/roomTagPull/show`
  - `GET /dashboard/roomTagPull/showContact`
  - `GET /dashboard/roomTagPull/roomList`
  - `GET /dashboard/roomTagPull/chooseContact`
  - `POST /dashboard/roomTagPull/store`
  - `POST /dashboard/roomTagPull/filterContact`
  - `GET /dashboard/roomTagPull/remindSend`
  - `DELETE /dashboard/roomTagPull/destroy`
  - `scripts/smoke_room_tag_pull_dashboard.sh` 已覆盖独立 Go 栈内标签建群筛客、过滤预估、新建、查看、客户明细、员工任务、客户群下拉、提醒发送、删除，以及企业微信 `media/uploadimg`、`externalcontact/add_msg_template`、`message/send` 请求和 SaaS 用量刷新。
- 客户群发第一批接口已迁移到 Go：
  - `GET /dashboard/contactMessageBatchSend/index`
  - `GET /dashboard/contactMessageBatchSend/show`
  - `GET /dashboard/contactMessageBatchSend/messageShow`
  - `GET /dashboard/contactMessageBatchSend/showRoom`
  - `GET /dashboard/contactMessageBatchSend/employeeSendIndex`
  - `GET /dashboard/contactMessageBatchSend/contactReceiveIndex`
  - `POST /dashboard/contactMessageBatchSend/store`
  - `POST /dashboard/contactMessageBatchSend/remind`
  - `DELETE /dashboard/contactMessageBatchSend/destroy`
  - `scripts/smoke_contact_message_batch_send_dashboard.sh` 已覆盖独立 Go 栈内客户群发立即发送、新建后列表/详情/预览/客户群详情/员工发送明细/客户接收明细反查、提醒发送、删除清理，以及企业微信 `media/upload`、`externalcontact/add_msg_template`、`message/send` 请求和 SaaS 用量刷新。
- 客户群群发第一批接口已迁移到 Go：
  - `GET /dashboard/roomMessageBatchSend/index`
  - `GET /dashboard/roomMessageBatchSend/show`
  - `GET /dashboard/roomMessageBatchSend/roomOwnerSendIndex`
  - `GET /dashboard/roomMessageBatchSend/roomReceiveIndex`
  - `POST /dashboard/roomMessageBatchSend/store`
  - `GET /dashboard/roomMessageBatchSend/remind`
  - `DELETE /dashboard/roomMessageBatchSend/destroy`
  - `scripts/smoke_room_message_batch_send_dashboard.sh` 已覆盖独立 Go 栈内客户群群发立即发送、新建后列表/详情/群主发送明细/群接收明细反查、提醒发送、删除清理，以及企业微信 `media/upload`、`externalcontact/add_msg_template`、`message/send` 请求和 SaaS 用量刷新。
- 公众号授权、本地配置和回调接口已迁移到 Go：
  - `GET /dashboard/officialAccount/index`
  - `GET /dashboard/officialAccount/set`
  - `GET /dashboard/officialAccount/getPreAuthUrl`
  - `GET /dashboard/officialAccount/authRedirect/`
  - `POST /dashboard/officialAccount/authRedirect/`
  - `GET /dashboard/officialAccount/authEventCallback`
  - `POST /dashboard/officialAccount/authEventCallback`
  - `GET /dashboard/{appId}/officialAccount/messageEventCallback`
  - `POST /dashboard/{appId}/officialAccount/messageEventCallback`
- 运营侧通用公众号 OAuth 入口已迁移到 Go：
  - `GET /load/{params?}`
  - `POST /load/{params?}`
- 裂变活动 dashboard 第一批接口已迁移到 Go：
  - `GET /dashboard/workFission/index`
  - `GET /dashboard/workFission/show`
  - `GET /dashboard/workFission/info`
  - `GET /dashboard/workFission/statistics`
  - `GET /dashboard/workFission/chooseContact`
  - `POST /dashboard/workFission/store`
  - `PUT /dashboard/workFission/update`
  - `POST /dashboard/workFission/invite`
  - `GET /dashboard/workFission/inviteData`
  - `GET /dashboard/workFission/inviteDetail`
  - `DELETE /dashboard/workFission/destroy`
  - `scripts/smoke_work_fission_dashboard.sh` 已覆盖新建、列表、详情、配置详情、统计、选择客户、邀请数据、邀请明细、二次邀请、更新、删除和 `work_fissions` SaaS 用量刷新。
- 裂变活动 operation 授权和 H5 业务接口已迁移到 Go：
  - `GET /operation/auth/workFission`
  - `POST /operation/auth/workFission`
  - `GET /operation/openUserInfo/workFission`
  - `GET /operation/workFission/inviteFriends`
  - `GET /operation/workFission/poster`
  - `GET /operation/workFission/taskData`
  - `PUT /operation/workFission/receive`
- 好友欢迎语接口已迁移到 Go：
  - `GET /dashboard/greeting/index`
  - `GET /dashboard/greeting/show`
  - `POST /dashboard/greeting/store`
  - `PUT /dashboard/greeting/update`
  - `DELETE /dashboard/greeting/destroy`
  - `scripts/smoke_greeting_dashboard.sh` 已覆盖全员/指定员工新建、列表状态、详情员工和素材展开、编辑、删除、业务日志和本地素材 URL 回显。
- 入群欢迎语接口已迁移到 Go：
  - `GET /dashboard/roomWelcome/index`
  - `GET /dashboard/clockIn/index`
  - `GET /dashboard/roomWelcome/select`
  - `GET /dashboard/roomWelcome/show`
  - `POST /dashboard/roomWelcome/store`
  - `PUT /dashboard/roomWelcome/update`
  - `DELETE /dashboard/roomWelcome/destroy`
  - `scripts/smoke_room_welcome_dashboard.sh` 已覆盖独立 Go 栈内新增/查看/编辑/删除入群欢迎语，以及企业微信图片上传、临时素材上传和模板 add/edit/del 请求。
- 在职转接/离职继承接口已迁移到 Go：
  - `GET /dashboard/contactTransfer/info`
  - `GET /dashboard/contactTransfer/unassignedList`
  - `GET /dashboard/contactTransfer/room`
  - `GET /dashboard/contactTransfer/log`
  - `GET /dashboard/contactTransfer/saveUnassignedList`
  - `POST /dashboard/contactTransfer/index`
  - `POST /dashboard/contactTransfer/room`
- 在职转接/离职继承后台状态刷新已迁移到 Go：
  - `TransferStateRefresh` 定时任务
- 群发结果同步后台任务已迁移到 Go：
  - `ContactSyncSendResultTask` 定时任务
  - `RoomSyncSendResultTask` 定时任务
- 群发定时发送后台任务已迁移到 Go：
  - `ContactMessageBatchSendQueue/SendJob` 到期客户群发扫描与提交
  - `RoomMessageBatchSendQueue/SendJob` 到期客户群群发扫描与提交
- 标签建群结果同步后台任务已迁移到 Go：
  - `RoomTagPull` 定时任务

## 当前仍未满足的全功能迁移要求

当前 224 条 manifest 路由已全部有 Go handler，但这还不能等同于完整独立版。主要缺口如下：

1. 路由覆盖已经补齐，仍需业务级全量回归
   - 清单基线为 224 条路由。
   - `scripts/standalone_inventory_parity.sh` 已核对 PHP 原项目重新扫描结果和 Go 内置 manifest：224 条路由、70 张表、9 个定时任务、10 个事件处理器、15 条队列注解均一致，当前没有清单漏收。
   - 最新 `scripts/standalone_route_coverage.sh` 审计结果为：命中 manifest 的 Go 路由 224 条，剩余未迁移路由 0 条。
   - standalone 模式下同时配置 `MOCHAT_MYSQL_DSN` 和 `MOCHAT_SIMPLE_JWT_SECRET` 时，当前所有已迁移业务路由会默认启用，不需要再逐个配置 `MOCHAT_GO_MIGRATE_*`；manifest 内业务路由不再因为未迁移返回 501。
   - 覆盖率应以后续 `scripts/standalone_route_coverage.sh` 的 `missing_route_total` 为准；该脚本可用 `MOCHAT_ROUTE_COVERAGE_MAX_MISSING` 设置失败阈值，防止缺口回升。
   - 后续必须继续用真实业务数据、微信/企微回调、前端页面和 SaaS 租户场景做逐链路验收，不能只看路由表。

2. PHP fallback 仍存在于兼容模式
   - 默认配置已不再自动设置 PHP upstream；只有显式配置 `MOCHAT_PHP_UPSTREAM` 时才会启用 reverse proxy fallback。
   - 这对灰度迁移有用，但不能作为独立版完成标准。
   - 后续应继续压缩文档和脚本里的 fallback 依赖，只保留为开发期对照工具。

3. 企业微信远程能力未完全迁移
   - `corp/store` 已迁移企业微信远程校验、`mc_corp` 创建和 `mc:user.{userId}` 企业选择缓存写入，并会向 `mochat-go:employee-apply` 写入企业授权后的通讯录同步任务。
   - `agent/store` 已完成 Go 迁移，可调用企业微信应用详情接口并写入 `mc_work_agent`。
   - 侧边栏 OAuth、JSSDK 签名、授权跳转已完成 Go 迁移。
   - 企微回调入口已完成 Go 迁移，可验签、解密 GET/POST 并把事件写入 Redis `mochat-go:wework-callback`。
   - 企微回调 Redis 消费 worker 已完成第一批 Go 迁移，并继续拆分细粒度 listener：客户群事件 `change_external_chat.create/update` 会按 `ChatId` 拉取单群详情并 upsert 单群，`dismiss` 会按 `wx_chat_id` 软删除本地客户群。通讯录成员事件 `change_contact.create_user/update_user/delete_user/create_party/update_party/delete_party` 已完成细粒度处理，成员和部门会按事件主键 upsert 或软删。外部联系人客户事件 `change_external_contact.add_external_contact/edit_external_contact/del_external_contact/del_follow_user` 会按 `UserID + ExternalUserID` 拉单客户、更新或恢复指定客户关系，并按 PHP 状态常量写入主动删除 `status=2`、被动删除 `status=3`，同时通过企业应用给对应员工发送“删除提醒”或“流失提醒”；其中新增客户事件已接通好友欢迎语 listener，若 `State=channelCode-{id}` 会优先读取 `mc_channel_code.welcome_message`，按特殊时期、周期、通用的 PHP 规则生成 `contact-welcome` 队列任务，若 `State=workRoomAutoPullId-{id}` 会读取自动拉群引导语和可用群二维码，若 `State=fission-{id}` 会兼容上游 listener 差异，按活动 ID 打标签、按上级参与人 ID 发送裂变图文欢迎语，更新裂变子参与人、上级助力数和完成状态，并在任务完成时按配置发送员工提醒和客户群发消息；删除裂变参与客户时会按 unionid/external_userid 标记 `mc_work_fission_contact.loss=1`，并对上级 `invite_count` 做非负回退。未匹配时再按指定成员欢迎语优先、全员欢迎语兜底的通用规则发送，并使用 `contact:welcome_status:{contactId}` 做 60 秒去重。外部联系人标签 `change_external_tag.create/update/delete` 已完成第一批细粒度 listener 迁移，会按 `tag_id/group_id` 拉目标标签片段并 upsert，删除时软删标签或标签组并同步软删客户标签 pivot。PHP 原项目仅映射、未绑定 listener 的 `add_half_external_contact`、`transfer_fail`、`change_external_tag.shuffle`、`change_contact.update_tag` 在 Go worker 中按 no-op 兼容，只做队列成功 ack 和执行记录。
   - `EmployeeApply` 后置队列已完成第一批 Go worker 迁移，可消费企业 ID 并执行部门、成员同步。
   - 客户群 create/update/dismiss、通讯录 create_user/update_user/delete_user/create_party/update_party/delete_party、外部联系人客户 add/edit/delete/del_follow、外部联系人标签 create/update/delete 已完成第一批细粒度 listener 迁移；`add_half_external_contact`、`transfer_fail`、标签 `shuffle/update_tag` 已按 PHP 原项目无 listener 口径做 no-op 兼容。企微回调 Redis 幂等键已从 raw XML 基线细化为按企业、事件路径、事件业务主键和 `CreateTime` 生成，缺少业务字段时才回退到 raw XML。

4. 队列与定时任务仍需生产级运行治理
   - 基线 9 个 PHP 定时任务已全部完成 Go 迁移：`pullAgent` 企业微信应用同步、`employeeStatistic` 成员统计拉取、`channelCode` 渠道码联系我方式更新、`ContactSyncSendResultTask` 客户群发结果同步、`RoomSyncSendResultTask` 客户群群发结果同步、`RoomTagPull` 标签建群结果同步、`corpData` 首页数据统计、`mediaIdUpdate` 素材库临时 `media_id` 更新和 `TransferStateRefresh` 分配状态刷新。
   - 基线有 10 个企微事件处理器。
   - 基线有 15 条队列注解。
   - 当前 Go 侧已迁移第一批企微回调 Redis worker、`EmployeeApply` Redis worker、`ContactWelcome` 欢迎语 Redis worker、`AsyncFileUpload` 文件上传 Redis worker、`MarkTags` 客户打标签 Redis worker、`MessageRemind` 企业应用消息提醒 Redis worker、`WorkRoomSync` 客户群同步 Redis worker、`WorkContactSync` 客户同步 Redis worker、`WorkDepartmentList` 部门/成员同步 Redis worker、`MediaIDUpdate` 素材临时 media_id 更新 Redis worker、`EmployeeStatisticApply` 成员统计 Redis worker、客户群发定时发送和客户群群发定时发送；十一类 Redis worker 已具备 processing、ack、retry、dead-letter 和 processing 超时恢复。
   - 已有基础后台任务运行器、周期调度包装器、`/compat/status.background_tasks` 内存状态、`mochat_go_background_tasks` 持久化任务状态表、`mochat_go_background_task_runs` 后台任务执行实例表、`mochat_go_background_task_executions` 执行历史表、第一版队列 payload 注册表和 10 分钟 Redis 幂等 key；企微回调幂等键按事件业务主键生成，定时任务会写入 `periodic_tick`，`employee-apply`、`wework-callback`、`contact-welcome`、`async-file-upload`、`mark-tags`、`message-remind`、`work-room-sync`、`work-contact-sync`、`work-department-list`、`medium-media-id-update` 和 `employee-statistic-apply` Redis worker 会写入或兼容 `queue_item` 执行状态，新入队任务会写入 `queue/payloadType/idempotencyKey/enqueuedAt` envelope，并继续兼容旧 raw JSON；`AsyncFileUpload` 兼容 PHP `file_upload_queue` 的旧数组 payload，同时支持结构化 payload 的 `tenantId/corpId` 在配置 MySQL 时写入租户归属、刷新 `async_executions` 并触发告警；`MarkTags` 兼容 PHP `handle(corpId, contactId, employeeId, tagIds)` 的位置参数 payload，`MessageRemind` 兼容 PHP `sendToEmployee(corpId, to, msgType, content, extra)` 位置参数 payload 和 `toUser/toParty/toTag` 对象 payload，`WorkRoomSync` 兼容 PHP `UpdateApply::handle(corpId)` 位置参数 payload 和 `UpdateCallback::handle(wxResponse)` 回调对象 payload，`WorkContactSync` 兼容 PHP `SyncContactApply::handle(employee, corpId, wxCorpid)`、`AdminSynContactApply::handle(employees, corpId)` 和分组客户 ID payload，`WorkDepartmentList` 兼容显式企业 ID 列表 payload，并支持由入口写入 `userId/tenantId` 后在 worker 内解析企业范围，`MediaIDUpdate` 兼容 PHP `handle(corpId, mediumIds)` 位置参数 payload，`EmployeeStatisticApply` 兼容 PHP `handle()` 的空数组 payload，并支持结构化 `corpId/tenantId` payload 限定统计范围和写入租户执行量。

5. 写接口覆盖不足
   - 后台账号创建、更新、状态切换、密码重置和当前账号改密已完成 Go 迁移。
   - 角色写接口、状态切换和权限保存已完成第一批 Go 迁移。
   - 菜单写接口、状态切换和删除已完成第一批 Go 迁移。
   - 客户画像字段新增、编辑、批量修改、状态切换、删除已完成第一批 Go 迁移。
   - 客户标签/标签组本地新增、编辑、删除、移动和企业微信远端标签同步已完成第一批 Go 迁移。
   - 客户主体 dashboard/sidebar 编辑接口、客户全量同步和客户群全量同步已迁移。

6. 营销插件仍需生产级回归和跨插件联动收口
   - 渠道活码主表读写接口和分组读写接口已完成；后续仍需迁移相关回调、同步或跨插件联动能力。
   - 素材库主体本地 CRUD、分组移动、dashboard/sidebar 列表、侧边栏单条临时 `media_id` 更新和后台 `mediaIdUpdate` 批量定时刷新已完成第一批 Go 迁移；已接入本地文件存在性检查、企业微信临时素材上传和 DB 写回。
   - 客户群分组本地 CRUD、客户群统计读接口和侧边栏群管理存在性校验已完成第一批 Go 迁移。
   - 好友欢迎语本地 CRUD、详情员工/素材展开、业务日志写入和 RBAC 数据权限过滤已完成第一批 Go 迁移；`scripts/smoke_greeting_dashboard.sh` 已覆盖全员/指定员工新建、列表状态、详情员工和素材展开、编辑、删除、业务日志和本地素材 URL 回显。企微新增客户回调已接通渠道码专属欢迎语和通用好友欢迎语发送链路，支持文本、图片、图文和小程序附件。
   - 入群欢迎语列表、详情、选择、新增、编辑、删除已迁移到 Go；新增/编辑/删除已接 Go 内置企业微信入群欢迎语模板接口，`scripts/smoke_room_welcome_dashboard.sh` 已用 fake 企业微信验证 `media/uploadimg`、`media/upload` 和 `group_welcome_template/add/edit/del` 真实请求与本地表状态。
   - 在职转接/离职继承列表、待分配客户/群、继承日志、待分配同步、客户接替和群接替已迁移到 Go；同步、接替和 `TransferStateRefresh` 状态刷新已接 Go 内置企业微信接口；`scripts/smoke_contact_transfer_dashboard.sh` 已用 fake 企业微信验证待分配同步、客户接替、群接替、分配记录、`mc_work_unassigned` / `mc_work_transfer_log` 落库和企业微信 `get_unassigned_list` / `transfer_customer` / `groupchat/transfer` 请求口径。
   - 客户群发列表、详情、客户群详情、员工发送明细、客户接收明细、新建、提醒、删除、定时发送和发送结果同步已完成第一批 Go 迁移；`scripts/smoke_contact_message_batch_send_dashboard.sh` 已用 fake 企业微信验证立即发送、列表/详情/预览/明细反查、提醒发送、删除清理、`media/upload`、`externalcontact/add_msg_template`、`message/send` 真实请求和 `contact_message_batches` 用量刷新。
   - 客户群群发列表、详情、群主发送明细、群接收明细、新建、提醒、删除、定时发送和发送结果同步已完成第一批 Go 迁移；`scripts/smoke_room_message_batch_send_dashboard.sh` 已用 fake 企业微信验证立即发送、列表/详情/群主/群接收明细反查、提醒发送、删除清理、`media/upload`、`externalcontact/add_msg_template`、`message/send` 真实请求和 `room_message_batches` 用量刷新。
   - 公众号授权列表、模块配置读取、模块公众号设置、预授权 URL、授权事件回调、授权回调和公众号消息事件回调已完成第一批 Go 迁移；`authEventCallback` 已支持保存微信开放平台周期推送的 `component_verify_ticket`，公众号预授权、授权回调、网页 OAuth 和开放平台测试消息会在静态环境变量缺失时从 Go 独立版 ticket 账本读取；`smoke_official_account_ticket.sh` 已覆盖 AES GET URL 校验、AES POST 验签解密、缺失或伪造 `msg_signature` 拒绝、`authRedirect` 调用 `api_query_auth` / `api_get_authorizer_info` 后写入 `tenant_id` 和公众号资料、`officialAccount/index?type=2` 默认绑定、`officialAccount/set?type=3` 手动绑定、`QUERY_AUTH_CODE`、`api_authorizer_token` 和客服文本发送；后续仍需补生产级微信开放平台回归、失败重试和异常告警。
   - 标签建群第一批读接口、新建、筛选客户、提醒发送和删除已完成；`scripts/smoke_room_tag_pull_dashboard.sh` 已用 fake 企业微信验证 `media/uploadimg`、`externalcontact/add_msg_template`、`message/send` 真实请求、本地表状态、客户入群状态、列表/详情/明细反查和 SaaS 用量刷新。
   - 自动拉群列表、详情、新建和更新已完成第一批 Go 迁移；`scripts/smoke_work_room_auto_pull_dashboard.sh` 已用 fake 企业微信验证 `contact_way/create/update` 真实请求、本地表状态、列表/详情反查和 SaaS 用量刷新。`/dashboard/workRoomAutoPull/move` 当前按兼容成功路径覆盖，未发现本地 PHP 源码中的独立移动实现可对照。
   - 裂变活动 dashboard 列表、详情、配置详情、统计、选择客户、新建、更新、邀请配置提交、邀请数据、邀请明细、删除，以及 operation 授权跳转、code 回调、openUserInfo、任务数据、邀请好友、海报和领奖已完成第一批 Go 迁移；`scripts/smoke_work_fission_dashboard.sh` 已用 fake 企业微信验证 `media/uploadimg`、`externalcontact/add_contact_way`、`externalcontact/add_msg_template` 真实请求、本地表状态、参与客户反查、更新落库和 SaaS 用量刷新。
   - 首页统计、客户群统计、渠道码统计、自动标签统计和裂变统计已完成第一批 Go 迁移；`scripts/smoke_dashboard_frontend_login.sh` 已把旧 dist 的密码修改页、客户群详情、客户群统计、渠道码统计、渠道码创建表单、欢迎语创建表单、入群欢迎语创建表单、自动拉群创建表单、标签建群创建表单、标签建群详情、三类自动标签创建页、三类自动标签详情页、角色权限树、客户画像字段、客户群发详情、客户群群发详情、裂变数据页和在职继承分配记录页面纳入真实浏览器回归；`scripts/smoke_admin_core_dashboard.sh` 已补管理端基础写接口真实 API 回归；`scripts/smoke_channel_code_dashboard.sh`、`scripts/smoke_shop_code_dashboard.sh`、`scripts/smoke_greeting_dashboard.sh`、`scripts/smoke_room_welcome_dashboard.sh`、`scripts/smoke_work_room_auto_pull_dashboard.sh`、`scripts/smoke_room_tag_pull_dashboard.sh`、`scripts/smoke_contact_message_batch_send_dashboard.sh`、`scripts/smoke_room_message_batch_send_dashboard.sh`、`scripts/smoke_contact_transfer_dashboard.sh` 和 `scripts/smoke_work_fission_dashboard.sh` 进一步覆盖渠道活码、门店活码、好友欢迎语、入群欢迎语、自动拉群、标签建群、客户群发、客户群群发、在职转接/离职继承和裂变活动的真实 dashboard API 写链路，后续仍需继续用真实业务数据补全更多统计、详情和跨模块联动页面。

7. 前端独立交付仍需继续收口
   - dashboard/sidebar/operation 的已构建 dist 已同步到本项目 `web/` 目录。
   - dashboard 已由 Go 服务通过 `MOCHAT_DASHBOARD_DIST=./web/dashboard/dist` 托管，standalone smoke 和真实浏览器登录 smoke 已覆盖 `/login`。
   - 当前 `web/dashboard/dist` 内置路由表约 61 个 dashboard 路由，真实浏览器登录 smoke 已覆盖其中 61 个主要页面，其中新增旧 dist 的 `/passwordUpdate/index`、`/channelCode/store`、`/greeting/store`、`/roomWelcome/create`、`/workRoomAutoPull/store`、`/roomTagPull/create`、`/autoTag/keywordCreate`、`/autoTag/keywordShow?idRow=918001`、`/autoTag/joinRoomCreate`、`/autoTag/joinRoomShow?idRow=918002`、`/autoTag/dayPartCreate`、`/autoTag/dayPartShow?idRow=918003`、`/workRoom/detail?workRoomId=910001`、`/workRoom/statistics?workRoomId=910001`、`/channelCode/statistics?channelCodeId=910001`、`/role/permissionShow?roleId=910001`、`/workContact/contactFieldPivot?contactId=910001&employeeId=2&isContact=1`、`/roomTagPull/detail?id=917001`、`/roomTagPull/contactDetail?id=917001`、`/contactMessageBatchSend/store`、`/contactMessageBatchSend/show?batchId=915001`、`/roomMessageBatchSend/store`、`/roomMessageBatchSend/show?batchId=916001`、`/workFission/create`、`/workFission/edit?id=985001`、`/workFission/invite?id=985001`、`/workFission/dataShow?id=985001`、`/officialAccount/create` 和 `/contactTransfer/workAllotRecord` 读路径回归；并新增 Go 原生 `/dashboard/sensitiveWords/page` 覆盖敏感词词库和触发监控的基本运营入口，新增 Go 原生 `/dashboard/lottery/page` 覆盖抽奖活动、奖品详情、分享链接和中奖客户入口，新增 Go 原生 `/dashboard/radar/page` 覆盖互动雷达素材、渠道、渠道链接和客户点击明细入口，新增 Go 原生 `/dashboard/shopCode/page` 覆盖门店活码 CRUD、页面设置、分享链接、地址检索和访问统计入口，新增 Go 原生 `/dashboard/contactSop/page` 覆盖个人 SOP 规则、员工范围、客户范围和启停入口，新增 Go 原生 `/dashboard/roomSop/page` 覆盖群 SOP 规则、客户群范围和启停入口，新增 Go 原生 `/dashboard/roomFission/page` 覆盖群裂变活动、海报欢迎语、邀请配置、群聊数据和参与客户入口，新增 Go 原生 `/dashboard/roomQuality/page` 覆盖群质检规则和触发记录入口，新增 Go 原生 `/dashboard/roomCalendar/page` 覆盖群日历列表和推送计划入口，新增 Go 原生 `/dashboard/roomRemind/page` 覆盖客户群提醒规则和启用任务入口，新增 Go 原生 `/dashboard/roomInfinitePull/page` 覆盖无限拉群活动和企微活码入口，新增 Go 原生 `/dashboard/roomClockIn/page` 覆盖群打卡活动、参与客户和打卡明细入口，新增 Go 原生 `/dashboard/saasAlert/page` 覆盖 SaaS 告警列表和通知配置入口；当前已迁移但旧 dist 未等价呈现的已知 `shopCode`、`contactSop`、`roomSop` 前端入口已由 Go 原生页面补齐。后续仍需恢复并重建旧前端完整路由，或继续用 Go 原生页补齐低频运营入口，并用真实业务数据回归，把 API 迁移完成转化为可运营的前端独立交付。
   - 按当前旧 dist 业务页口径，主要路由已全部纳入真实浏览器登录 smoke；旧失效入口 `/roomTagPull/contactDetail` 和源码中实际跳转的 `/roomTagPull/clientDetails` 由 Go 原生客户明细页接管。
   - 旧 dist 路由 `/roomTagPull/contactDetail` 原组件为空，`roomTagPull/detail.vue` 的客户详情按钮实际跳转到未注册的 `/roomTagPull/clientDetails`；当前 Go server 已在静态前端包装层保留这两个路径并统一路由到 `MoChat Go 标签建群客户明细` 原生页面，页面读取 `/dashboard/roomTagPull/show` 和 `/dashboard/roomTagPull/showContact` 展示活动概览、客户明细和员工任务。
   - sidebar/operation 的构建产物仍使用根路径 `/css`、`/js`，但当前 Go 静态托管层已提供 `/sidebar-app/` 和 `/operation-app/` 主端口前缀入口，并对 HTML 本地资源、webpack publicPath、Vue Router base 和旧 dist API base 做运行时重写，避免与 dashboard 根路径资源冲突；独立监听地址方案仍保留用于隔离部署。
   - 后续若要完全替代旧 dist，仍应恢复并重建前端源码的正式 publicPath 配置，或继续用 Go 原生页补齐低频运营入口；当前独立版已不再只能依赖独立端口/域名加载 sidebar/operation 壳。

8. 数据初始化、部署和 SaaS 开通底座已完成第一轮拆分，套餐限制已接入第一批写入口
   - `deploy/standalone/schema/mochat.sql` 已进入 Go 项目，且只保留建表结构。
   - 默认客户资料字段、侧边栏工具和 RBAC 菜单数据已拆到 `deploy/standalone/migrations/0002_seed_core_data.up.sql`，standalone compose 会在建表后执行该 seed，`cmd/mochat-migrate` 也会把它作为第二条迁移记录。
   - `0002_seed_core_data` 使用 `INSERT IGNORE`，迁移器已兼容拆分前的一体化 `0001_initial_schema` checksum；旧环境如果已经通过旧版 `0001` 建表并写入 seed，升级后执行 `apply` 可以接受 legacy checksum 并补齐 seed 迁移记录。
   - 新增 `deploy/standalone/migrations/0003_saas_provisioning.up.sql`，创建 `mochat_go_saas_packages`、`mochat_go_saas_tenant_packages`、`mochat_go_saas_usage_counters`、`mochat_go_seed_versions` 和 `mochat_go_tenant_provision_runs`，用于 Go 独立版 SaaS 套餐、租户套餐、用量、seed 版本和开通记录。
   - 新增 `deploy/standalone/migrations/0004_saas_storage_objects.up.sql`，创建 `mochat_go_saas_storage_objects` 上传文件账本，记录租户、用户、侧边栏员工、企业、上传入口、相对路径、MIME 和字节数，用于 `storage_mb` 额度统计。
   - 新增 `deploy/standalone/migrations/0005_background_tasks.up.sql`，创建 `mochat_go_background_tasks`、`mochat_go_background_task_runs` 和带 `tenant_id` 的 `mochat_go_background_task_executions` 后台任务账本；运行时 recorder 仍会兼容旧库自动补表、补列和补索引。
   - 新增 `deploy/standalone/migrations/0006_saas_alerts.up.sql`，创建 `mochat_go_saas_alerts` SaaS 运营告警账本，用于聚合记录租户级额度超限信号。
   - 新增 `deploy/standalone/migrations/0007_wechat_component_tickets.up.sql`，创建 `mochat_go_wechat_component_tickets` 微信开放平台 ticket 账本，用于持久化 `authEventCallback` 推送的 `component_verify_ticket`，避免公众号预授权和 OAuth 运行时依赖手工注入的静态 ticket。
   - 新增 `deploy/standalone/migrations/0008_saas_alert_notifications.up.sql`，创建 `mochat_go_saas_alert_notifications` SaaS 告警通知 outbox，记录 webhook 通知状态、尝试次数、失败原因和下次重试时间。
   - 新增 `deploy/standalone/migrations/0023_saas_alert_settings.up.sql`，创建 `mochat_go_saas_alert_settings` 租户级 SaaS 告警通知配置表，记录 webhook 开关、URL、签名密钥、模板、HTTP 重试和 outbox 重试策略；standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0009_contact_batch_add_rbac.up.sql`，补齐批量加好友后台页面和接口权限，standalone compose 初始化和 `cmd/mochat-migrate` 都会应用该 seed。
   - 新增 `deploy/standalone/migrations/0010_sensitive_words.up.sql`，创建敏感词词库、分组和触发监控业务表，standalone compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0011_sop_rbac.up.sql`，确保个人 SOP/群 SOP 主表和触达日志表存在，并补齐后台接口权限，standalone compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0012_shop_code.up.sql`，创建门店活码 `mc_shop_code`、`mc_shop_code_page`、`mc_shop_code_record` 三张业务表，并补齐门店活码页面菜单和 18 个后台接口权限，standalone compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0013_radar.up.sql`，创建互动雷达 `mc_radar`、`mc_radar_channel`、`mc_radar_channel_link`、`mc_radar_record` 四张业务表，并补齐互动雷达页面菜单和 13 个后台接口权限，standalone compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0014_auto_tag.up.sql`，创建自动标签 `mc_auto_tag`、`mc_auto_tag_record`、消息存档索引表 `mc_work_message_id` 和 `mc_work_message_1` 至 `mc_work_message_10` 分表，补齐 `mc_corp` 会话存档配置字段，并写入自动标签、消息存档页面菜单和后台接口权限；新增 `deploy/standalone/migrations/0022_work_message_archive_sync.up.sql`，将会话存档游标扩展为 bigint 并增加消息分表同步索引。standalone compose 初始化和 `cmd/mochat-migrate` 都会应用这些迁移。
   - 新增 `deploy/standalone/migrations/0015_lottery.up.sql`，创建抽奖活动 `mc_lottery`、`mc_lottery_contact`、`mc_lottery_contact_record`、`mc_lottery_prize` 四张业务表，并写入抽奖活动页面菜单和后台接口权限，standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0016_room_fission.up.sql`，创建群裂变 `mc_room_fission`、`mc_room_fission_contact`、`mc_room_fission_invite`、`mc_room_fission_poster`、`mc_room_fission_room`、`mc_room_fission_welcome` 六张业务表，并写入群裂变页面菜单和后台接口权限，standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0017_room_clock_in.up.sql`，创建群打卡 `mc_room_clock_in`、`mc_room_clock_in_contact`、`mc_room_clock_in_record` 三张业务表，并写入群打卡页面菜单和后台接口权限，standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0018_room_quality.up.sql`，创建群质检 `mc_room_quality`、`mc_room_quality_contact` 两张业务表，并写入群质检页面菜单和后台接口权限，standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0019_room_calendar.up.sql`，创建群日历 `mc_room_calendar`、`mc_room_calendar_push`、`mc_room_calendar_record` 三张业务表，并写入群日历页面菜单和后台接口权限，standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0020_room_remind.up.sql`，创建客户群提醒 `mc_room_remind`、`mc_room_remind_record` 两张业务表，并写入客户群提醒页面菜单和后台接口权限，standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - 新增 `deploy/standalone/migrations/0021_room_infinite_pull.up.sql`，创建无限拉群 `mc_room_infinite` 业务表，并写入无限拉群页面菜单和后台接口权限，standalone/local compose 初始化和 `cmd/mochat-migrate` 都会应用该迁移。
   - `cmd/mochat-bootstrap` 已提供首次独立部署和批量租户开通能力，可创建或恢复租户、超级管理员、管理员角色、用户角色绑定，并把当前未删除菜单同步到管理员角色；同时会写入 SaaS 套餐绑定、26 个 lifetime 用量指标、3 条 seed 版本记录和开通记录，并支持 `-batch-file` CSV 批量开通。
   - `scripts/smoke_bootstrap_standalone.sh` 已验证迁移后空库可 bootstrap 出可登录账号，并通过 Go standalone `POST /dashboard/user/auth` 换取 token，同时断言 SaaS 套餐、用量、seed 版本和开通记录写入正确。
   - 新增 `scripts/smoke_saas_provisioning.sh`，用 CSV 一次开通两个租户，验证两个租户的管理员、角色菜单、套餐、用量、seed 版本和开通记录互相隔离，并验证两个管理员都能通过 Go standalone 登录。
   - 已新增 `internal/dashboard/saas_quota.go` 和 MySQL 运行时用量计算，`POST /dashboard/corp/store`、`POST /dashboard/user/store`、`POST /dashboard/agent/store` 会按租户套餐限制拦截企业数、子账号数和应用数超额写入；客户和客户群同步写入层也已接入套餐限制，`SyncWorkContacts`、`SyncWorkContactForEmployee`、`SyncWorkRooms` 和 `SyncWorkRoom` 会覆盖后台回调 worker 的新增客户/客户群写入。
   - `POST /dashboard/common/upload`、`POST /dashboard/common/uploadFile` 和 `POST /sidebar/common/upload` 已接入上传文件账本和 `storage_mb` 套餐限制；上传前会按租户剩余额度拦截，上传成功后会写入 `mochat_go_saas_storage_objects` 并刷新 `mochat_go_saas_usage_counters.storage_mb`。
   - 渠道活码创建失败回滚已经纳入 SaaS 存储账本回收：`POST /dashboard/channelCode/store` 写入记录后如企业微信 contact_way 创建失败，会通过 `DeleteChannelCode` 软删业务记录、刷新 `channel_codes`，并解析 `welcome_message.messageDetail` 里的 `pic_url`、`imagePath`、`linkPic`、`coverPath` 等本地欢迎语素材路径，回收 `mochat_go_saas_storage_objects` 并刷新 `storage_mb`。
   - 渠道活码 `POST /dashboard/channelCode/store`、门店活码 `POST /dashboard/shopCode/store`、互动雷达 `POST /dashboard/radar/store`、抽奖活动 `POST /dashboard/lottery/store`、无限拉群 `POST /dashboard/roomInfinitePull/store`、群裂变 `POST /dashboard/roomFission/store`、群打卡 `POST /dashboard/roomClockIn/store`、群质检 `POST /dashboard/roomQuality/store`、群日历 `POST /dashboard/roomCalendar/store`、客户群提醒 `POST /dashboard/roomRemind/store`、个人 SOP `POST /dashboard/contactSop/store`、群 SOP `POST /dashboard/roomSop/store`、敏感词 `POST /dashboard/sensitiveWord/store`、客户群发 `POST /dashboard/contactMessageBatchSend/store`、客户群群发 `POST /dashboard/roomMessageBatchSend/store`、标签建群 `POST /dashboard/roomTagPull/store`、自动拉群 `POST /dashboard/workRoomAutoPull/store`、裂变活动 `POST /dashboard/workFission/store` 和公众号授权 `GET/POST /dashboard/officialAccount/authRedirect/` 已接入 `channel_codes`、`shop_codes`、`radars`、`lotteries`、`room_infinite_pulls`、`room_fissions`、`room_clock_ins`、`room_qualities`、`room_calendars`、`room_reminds`、`contact_sops`、`room_sops`、`sensitive_words`、`contact_message_batches`、`room_message_batches`、`room_tag_pulls`、`work_room_auto_pulls`、`work_fissions`、`official_accounts` 扩展套餐指标；开通时可通过 `cmd/mochat-bootstrap -channel-codes`、`-shop-codes`、`-radars`、`-lotteries`、`-room-infinite-pulls`、`-room-fissions`、`-room-clock-ins`、`-room-qualities`、`-room-calendars`、`-room-reminds`、`-contact-sops`、`-room-sops`、`-sensitive-words`、`-contact-message-batches`、`-room-message-batches`、`-room-tag-pulls`、`-work-room-auto-pulls`、`-work-fissions`、`-official-accounts` 或批量 CSV 的同名下划线字段配置额度，运行时新增前拦截，新增后刷新用量，维护命令可重算。新增 `deploy/standalone/migrations/0024_saas_package_extended_limits.up.sql`、`0025_saas_radar_limit.up.sql`、`0026_saas_lottery_limit.up.sql`、`0027_saas_room_infinite_pull_limit.up.sql`、`0028_saas_room_fission_limit.up.sql`、`0029_saas_room_clock_in_limit.up.sql`、`0030_saas_room_operation_limits.up.sql`、`0031_saas_sop_limits.up.sql` 和 `0032_saas_sensitive_word_limit.up.sql` 会补齐套餐表扩展额度列。渠道活码创建或定时刷新 contact_way 失败触发软删回滚时，会立即刷新 `channel_codes` 用量；门店活码删除会释放并刷新 `shop_codes` 用量，更新、删除门店活码或通过 `pageSet` 替换页面设置时，也会回收 `employee_qrcode`、`qw_code` 活码 JSON 和 `mc_shop_code_page.default` / `poster` 里的本地二维码/海报账本并刷新 `storage_mb`；互动雷达删除会释放并刷新 `radars` 用量；抽奖活动删除会释放并刷新 `lotteries` 用量，更新或删除抽奖活动也会回收奖品设置和中奖记录客服二维码里的本地素材账本并刷新 `storage_mb`；无限拉群删除会释放并刷新 `room_infinite_pulls` 用量；群裂变删除会释放并刷新 `room_fissions` 用量；群打卡删除会释放并刷新 `room_clock_ins` 用量；群质检、群日历、客户群提醒、个人 SOP、群 SOP 和敏感词删除会释放并刷新 `room_qualities`、`room_calendars`、`room_reminds`、`contact_sops`、`room_sops`、`sensitive_words` 用量；客户群发、客户群群发、标签建群、自动拉群和裂变活动删除或失败回滚后，也会刷新对应业务资源用量；公众号 `unauthorized` 取消授权和已有授权更新会刷新 `official_accounts`，取消授权账号不再出现在公众号列表且不继续占用套餐名额。
   - `employee-apply`、`wework-callback`、`contact-welcome`、可解析企业的业务 worker、带 `tenantId/corpId` 的 `async-file-upload` 和带 `corpId/tenantId` 的 `employee-statistic-apply` Redis worker 的 `queue_item` 执行历史已写入租户归属，`async_executions` 会按 tenant 统计已结束的异步队列处理量；`async-file-upload` 在写入 Go storage root 后还会按实际落盘字节写入 `mochat_go_saas_storage_objects`，并刷新 `storage_mb`，避免异步上传绕过 SaaS 存储计量。`scripts/smoke_async_file_upload_saas_usage.sh` 已用两个 bootstrap 租户验证 `tenantId` 直传和 `corpId` 反查两种 payload 都只刷新当前租户，A 租户超额时只产生 A 租户 `mochat_go_saas_alerts`，B 租户不会收到 A 的执行记录、存储账本或告警。开通时可通过 `cmd/mochat-bootstrap -async-executions`、环境变量 `MOCHAT_BOOTSTRAP_ASYNC_EXECUTIONS` 或批量 CSV 字段 `async_executions` 配置额度，worker 每次处理完成后会刷新运行时用量，维护命令也可重算；当前超额策略先不阻断企微回调处理，超过额度时写入运行日志和 `mochat_go_saas_alerts` 告警账本，并把通知状态写入 `mochat_go_saas_alert_notifications` outbox。通知发送优先读取 `mochat_go_saas_alert_settings` 租户级配置，支持 webhook URL、HMAC-SHA256 签名密钥、标题正文模板、HTTP retry、outbox 最大次数和失败重试间隔；未配置租户项时仍可用 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL`、`MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET`、`MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS`、`MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS`、`MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE`、`MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE`、`MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS` 和 `MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS` 作为全局兜底；重试耗尽仍只记日志不阻断 worker。
   - `scripts/smoke_saas_tenant_isolation.sh` 已用两个真实 bootstrap 租户验证企业绑定、Redis 企业选择缓存、首页统计、通讯录、角色列表、权限菜单和 dashboard 上传账本不会跨租户串数；两个租户分别上传文件后，`mochat_go_saas_storage_objects` 和 `storage_mb` 只刷新各自租户。
   - `scripts/smoke_saas_quota_enforcement.sh` 已用真实 Go standalone、MySQL、Redis、fake 企业微信/微信开放平台 API 和登录 token 验证企业数、子账号数、应用数、渠道活码数、门店活码数、互动雷达数、抽奖活动数、无限拉群数、群裂变数、群打卡数、群质检规则数、群日历数、客户群提醒数、个人 SOP 规则数、群 SOP 规则数、敏感词词库数、客户数、客户群数、客户群发任务数、客户群群发任务数、标签建群任务数、自动拉群活码数、裂变活动数、公众号授权数和素材存储达到上限时返回 400 且业务表或上传目录不新增记录。
   - 新增 `cmd/mochat-saas-maintenance -action reconcile-storage` 和 `scripts/smoke_saas_storage_reconcile.sh`，可按上传目录校准 `mochat_go_saas_storage_objects`：文件丢失或危险路径会标记软删除，文件大小漂移会更新 `size_bytes`，并刷新受影响租户的 `storage_mb` 用量。
   - 新增 `MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON`、`MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS` 和 `MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_RUN_ON_START`，Go standalone 可启动 `cron-saas-storage-reconcile` 定时任务，默认每天自动重算上传存储账本；smoke 已断言该任务写入 `mochat_go_background_tasks` 和 `mochat_go_background_task_executions`。
   - `POST /dashboard/channelCode/store` 在企业微信 contact_way 创建失败回滚时会解析 `welcome_message.messageDetail`，回收 `pic_url`、`imagePath`、`linkPic`、`coverPath` 等本地欢迎语素材账本；`scripts/smoke_saas_storage_reclaim.sh` 已用真实 Go standalone 验证该链路和既有触发式回收链路。
   - `PUT /dashboard/medium/update` 替换素材文件路径时会回收旧路径账本，`DELETE /dashboard/medium/destroy` 软删素材时会回收当前路径账本，路径字段包括 `imagePath`、`voicePath`、`videoPath` 和 `filePath`；`PUT /dashboard/shopCode/update` 和 `POST /dashboard/shopCode/updateQrcode` 替换或移除门店活码 `employee_qrcode`、`qw_code` 本地二维码时会回收旧路径账本，`POST /dashboard/shopCode/pageSet` 替换或移除页面设置 `default` / `poster` 中的本地二维码或海报时会回收旧路径账本，`DELETE /dashboard/shopCode/destroy` 软删门店活码时会回收当前二维码账本；`PUT /dashboard/radar/update` 替换互动雷达 `link_cover` 或 `pdf` 本地文件时会回收旧路径账本，`DELETE /dashboard/radar/destroy` 软删互动雷达时会回收当前雷达封面和 PDF 文件账本；`PUT /dashboard/lottery/update` 替换抽奖活动奖品、兑奖、限制或企业名片 JSON 中的本地素材时会回收旧路径账本，`DELETE /dashboard/lottery/destroy` 软删抽奖活动时会回收奖品设置和中奖记录客服二维码里的本地素材账本；`PUT /dashboard/roomClockIn/update` 替换群打卡 `employee_qrcode` 本地二维码时会回收旧路径账本，`DELETE /dashboard/roomClockIn/destroy` 软删群打卡时会回收当前领奖客服二维码账本；`PUT /dashboard/roomInfinitePull/update` 替换无限拉群 `avatar`、`logo` 或 `qw_code` 活码 JSON 中的本地二维码时会回收旧路径账本，`DELETE /dashboard/roomInfinitePull/destroy` 软删无限拉群时会回收当前头像、logo 和活码二维码账本；`PUT /dashboard/roomWelcome/update` 和 `DELETE /dashboard/roomWelcome/destroy` 会回收入群欢迎语 `msg_complex.pic` 本地图片账本；`DELETE /dashboard/contactMessageBatchSend/destroy` 和 `DELETE /dashboard/roomMessageBatchSend/destroy` 会回收群发内容里的本地 `pic_url` 图片账本；`DELETE /dashboard/contactBatchAdd/importDestroy` 会回收批量加好友导入记录 `file_url` 对应的本地原文件账本；`DELETE /dashboard/roomTagPull/destroy` 会回收标签建群 `rooms.image` 本地图片账本；`PUT/POST /dashboard/roomFission/update/invite` 替换群裂变海报、群二维码、欢迎语或邀请封面时会回收旧本地路径，`DELETE /dashboard/roomFission/destroy` 会在级联软删群裂变活动时回收 poster、room、welcome、invite 里的本地图片账本；`PUT /dashboard/workRoomAutoPull/update` 替换或移除自动拉群 `rooms.roomQrcodeUrl` 本地群二维码时会回收旧路径账本，`POST /dashboard/workRoomAutoPull/store` 在企业微信 contact_way 创建失败回滚时也会回收已写入记录里的本地群二维码；`DELETE /dashboard/workFission/destroy` 会在级联软删裂变活动时回收 poster、welcome、push、invite 里的本地图片账本；`scripts/smoke_saas_storage_reclaim.sh` 已用真实 Go standalone 验证这些触发式回收链路。
   - 新增 `cmd/mochat-saas-maintenance -action refresh-usage` 和 `scripts/smoke_saas_usage_refresh.sh`，可在套餐配置变更、计数器缺失或历史数据导入后按当前业务表和租户套餐配置重刷 26 个 lifetime 用量指标，修正 `used_value`、`limit_value` 和 `updated_by`，其中包括个人 SOP、群 SOP 规则数和敏感词词库数。
   - 新增 `cmd/mochat-saas-maintenance -action list-alerts/resolve-alert/dispatch-alert-notifications`，可按租户、指标、状态列出或解决 `mochat_go_saas_alerts` 当前告警，也可重发 `mochat_go_saas_alert_notifications` 到期通知并回写 delivered、failed 或 dead；新增 `MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON=1`、`MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS`、`MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_RUN_ON_START` 和 `MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT`，Go standalone 可启动 `cron-saas-alert-notification-dispatch` 自动扫描并重发 outbox 到期通知，且不再强制要求全局 webhook URL；新增 `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1`，可让租户超管访问 Go 自带的 `GET /dashboard/saasAlert/page` 管理页面，通过 `GET /dashboard/saasAlert/index` 分页查看本租户告警，通过 `PUT/POST /dashboard/saasAlert/resolve` 解决当前打开告警，并通过 `GET/PUT/POST /dashboard/saasAlert/setting` 查看和保存本租户通知策略；`scripts/smoke_employee_apply_worker.sh` 已验证异步执行量超额时写入告警账本、outbox 标记 delivered、fake webhook 首次 502 后重试成功、模板化 `title/body`、真实 JWT 可查询 dashboard 告警列表、Go 自带告警页面可访问、维护命令可列出并解决告警、outbox 到期通知可重发；`scripts/smoke_saas_alert_notification_cron.sh` 已验证内置 cron 可自动重发 pending 通知并写入后台任务执行历史；`scripts/smoke_saas_alert_setting_dispatch.sh` 已验证不设置全局 webhook URL 时，维护命令可仅凭租户配置表完成带签名的 webhook 投递。
   - `cmd/mochat-migrate` 已提供 schema migration 工具，支持 `apply/status/baseline/rollback`，会写入 `mochat_go_schema_migrations` 并校验 SQL checksum。
   - 后续增量迁移规范已补到 `deploy/standalone/migrations`：`*.up.sql` 必须配套 `*.down.sql`，`rollback` 只回滚最后一条带 down 脚本的增量迁移，`0001_initial_schema` 禁止回滚。
   - 已新增 `scripts/lint_mysql57_schema.sh`，静态扫描独立 schema 和增量迁移，拦截 MySQL 8 专属语法和 MySQL 5.7 不兼容 JSON 默认值。
   - 已新增 `scripts/smoke_mysql57_schema_migrate.sh` 和 `deploy/mysql57/docker-compose.yml`，用 `mysql:5.7` 真实执行初始 schema、增量迁移和 rollback，作为生产 MySQL 5.7 兼容门禁；arm64 本机默认跳过真实容器 smoke，避免 `mysql:5.7` amd64 镜像在 qemu 下初始化段错误，amd64 CI 或设置 `MOCHAT_FORCE_MYSQL57=1` 时执行真实容器验收。
   - `scripts/smoke_sidebar_frontend_contact.sh` 已覆盖 sidebar 真实客户详情页、个人 SOP 详情页、群 SOP、批量加好友和素材库浏览器链路：脚本使用独立 MySQL/Redis、fake 企业微信 API、真实 sidebar JWT cookie 和 Go standalone，访问 `/contact?agentId=1`、`/contactSop?id=900001&agentId=1`、`/roomSop?id=900001`、`/contactBatchAdd?batchId=900001` 与 `/medium?agentId=1`，验证 `agent/jssdkConfig`、客户详情、画像字段、互动轨迹、个人 SOP 提醒弹窗、个人 SOP 详情、群 SOP 详情、群 SOP 完成状态写库、批量加好友分配手机号、素材分组、素材列表、本地静态资源和旧构建产物里的 `/undefined/sidebar/*` API 前缀归一化；当前脚本已开启 `cron-sop-log`，个人/群 SOP 日志由 Go 自动生成，不再手工 seed 日志表，并覆盖带 `_mochatGoOccurrence` 的周期个人/群 SOP 日志和 `targetAnchor=room_join` 的客户入群后群 SOP 日志，断言 `mc_room_sop_log.contact` 写入外部联系人 ID，且内置 sidebar dist 已修复个人 SOP 详情页初始空数据导致的 `task.content` 运行时错误。
   - 批量加好友后台 `dashboard/contactBatchAdd/index/importIndex/importStore/allot/dataStatistic/destroy/importDestroy/settingEdit/settingUpdate/remind` 已完成第一批 Go 接管，覆盖 MySQL 列表、导入记录、JSON/表单/CSV/简单 XLSX 导入、分配记录、软删除、设置 upsert 和统计聚合；multipart 导入会把原文件写入 Go storage root，回填 `fileUrl`、记录 SaaS 存储账本并刷新 `storage_mb`，`importDestroy` 会回收导入原文件账本；`scripts/smoke_contact_batch_add_dashboard.sh` 已用真实 Go standalone 覆盖配置、CSV 导入、导入列表、客户筛选、统计、提醒、二次分配、单条删除、批次删除和存储账本回收，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；该功能原 PHP Action 扫描清单缺失，但 dashboard 前端 API、RBAC 菜单和 apidoc 均存在，因此作为完整独立功能补齐。
   - 敏感词后台 `dashboard/sensitiveWord/index/store/destroy/statusUpdate/move`、`dashboard/sensitiveWordGroup/select/store/update` 和 `dashboard/sensitiveWordsMonitor/index/show` 已完成第一批 Go 接管，覆盖分组、词库、批量拆词、状态、移动、软删除、监控列表和对话详情读取；新增 Go 原生 `/dashboard/sensitiveWords/page` 页面，使用 dashboard JWT 直接操作这些接口，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_sensitive_word_dashboard.sh` 已用真实 Go standalone 覆盖敏感词分组批量新建、更新、下拉选择，词库批量拆词去重、新增、移动、状态更新、删除、监控列表、监控详情会话展开和 `sensitive_words` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增敏感词已接入 `sensitive_words` SaaS 套餐额度，创建前按拆分后的词数拦截，创建/删除后刷新用量；新增 `cron-work-message-archive-sync` 可从内部会话存档 SDK bridge 拉取已解密消息，按 `seq` 幂等写入 `mc_work_message_1` 至 `mc_work_message_10` 并用 `mc_work_message_id.type=40` 维护拉取游标；新增 `cron-sensitive-word-monitor` 可扫描 `mc_work_message_1` 至 `mc_work_message_10` 新增会话存档消息，按启用敏感词幂等写入 `mc_sensitive_words_monitor`，并用 `mc_work_message_id.type=21..30` 维护分表游标。后续仍需部署接企业微信官方会话存档 SDK 的生产 bridge、覆盖更多消息类型语义提取和做真实企业账号回归。
   - SOP 后台 `dashboard/contactSop/index/store/setEmployee/state/info/destroy/update` 和 `dashboard/roomSop/index/store/setRoom/state/info/destroy/update` 已完成第一批 Go 接管，覆盖列表、详情、创建、编辑、启停、设置员工/群聊和删除；新增 Go 原生 `/dashboard/contactSop/page` 与 `/dashboard/roomSop/page` 页面，使用 dashboard JWT 直接管理个人 SOP 规则、员工范围、客户范围、群 SOP 规则、客户群范围和启停状态，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；新增规则已接入 `contact_sops` 和 `room_sops` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；规则配置继续按原 JSON 字段保存，复算 dashboard 前端 API 缺口从 130 降到 116，`contactSop.js` 和 `roomSop.js` 已无缺口。新增 `cron-sop-log` 可按启用规则解析 `setting`、员工/客户/群范围，向 `mc_contact_sop_log` 和 `mc_room_sop_log` 幂等生成已到期提醒，支持 `MOCHAT_GO_ENABLE_SOP_LOG_CRON`、`MOCHAT_GO_SOP_LOG_CRON_INTERVAL_SECONDS`、`MOCHAT_GO_SOP_LOG_CRON_RUN_ON_START`；当前已支持绝对提醒时间、任务内 `baseTime` / `startTime` / `anchorTime` / `createdAt` 锚点叠加 `delayMinutes`、`delayHours`、`delayDays`、`delay`、`after` 等相对延迟字段，也支持纯相对延迟规则按个人 SOP 客户添加时间 `mc_work_contact_employee.create_time` 或群 SOP 客户群创建时间 `mc_work_room.create_time` 逐目标计算到期状态；群 SOP 纯相对延迟还支持 `targetAnchor=room_join` / `customer_join_room` / `客户入群` 等客户入群锚点，按 `mc_work_contact_room.join_time` 为外部联系人成员生成带 `contact` 的群 SOP 日志，并把 `contact` 纳入幂等条件；周期规则支持 `cycle` / `period` / `repeat` / `frequency` 的 `daily`、`weekly`、`monthly`，可用 `weekdays`、`monthDays`、`repeatDates` 限定日期，并通过 `_mochatGoOccurrence` 实现同周期幂等和跨日期重复触达。当前已接 `change_external_contact.add_external_contact` 目标级触发个人 SOP、`change_external_chat.create/update` 目标级触发 `room_join` 群 SOP；后续仍需更多目标事件锚点和生产数据回归。
   - `scripts/smoke_sop_dashboard.sh` 已用真实 Go standalone 覆盖个人/群 SOP 新建、列表、详情、设置员工/群聊、启停、更新、删除、触达日志级联删除和 `contact_sops` / `room_sops` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`。
   - 门店活码后台 `dashboard/shopCode/location/addressKeyWordList/store/update/destroy/info/status/index/searchCity/share/pageInfo/pageSet/show/showContact/showShop/updateEmployee/updateQrcode/batchContactTags` 已完成第一批 Go 接管，覆盖门店 CRUD、启停、员工/二维码 JSON 字段更新、页面配置、地址/城市本地检索、分享链接和统计读取；新增 Go 原生 `/dashboard/shopCode/page` 页面，使用 dashboard JWT 直接管理门店、页面设置、分享链接、地址/城市建议、访问客户和门店统计，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_shop_code_dashboard.sh` 已用真实 Go standalone 覆盖门店活码新建、列表、详情、位置、城市/地址检索、分享、页面设置、统计、客户明细、门店统计、员工更新、二维码更新、状态切换、批量打标签、删除和 `shop_codes` SaaS 用量刷新；门店活码更新、删除或 `pageSet` 页面设置替换会回收 `employee_qrcode`、`qw_code` 活码 JSON 和 `mc_shop_code_page.default` / `poster` 中的本地二维码/海报账本并刷新 `storage_mb`；新增 `0012_shop_code` 后复算 dashboard 前端 API 缺口从 116 降到 98，`shopCode.js` 已无缺口。
   - 互动雷达后台 `dashboard/radar/store/update/index/destroy/info/storeChannel/storeChannelLink/indexChannel/indexChannelLink/show/showContact/showChannel/radarArticle` 已完成第一批 Go 接管，覆盖雷达 CRUD、渠道、渠道链接、分享链接、文章元数据回显和点击统计读取；新增 Go 原生 `/dashboard/radar/page` 页面，使用 dashboard JWT 直接管理雷达素材、渠道、渠道链接和客户点击明细，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_radar_dashboard.sh` 已用真实 Go standalone 覆盖雷达新建、列表、详情、渠道、渠道链接、客户点击明细、渠道统计、文章元数据回显、更新、删除和 `radars` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增活动已接入 `radars` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；雷达更新和删除会回收 `link_cover`、`pdf` 对应的本地上传文件账本并刷新 `storage_mb`；新增 `0013_radar` 后复算 dashboard 前端 API 缺口从 98 降到 83，`radar.js` 已无缺口。
   - 自动标签和消息存档后台 `dashboard/autoTag/store/index/destroy/onOff/show/showContactKeyWord/showContactRoom/showContactTime`、`Task/AutoTag/KeyWordTag`、`dashboard/Task/AutoTag/KeyWordTag`、`dashboard/workMessage/fromUsers/toUsers/index` 和 `dashboard/workMessageConfig/corpStore/corpShow/corpIndex/stepCreate/stepUpdate` 已完成第一批 Go 接管，覆盖自动标签规则 CRUD、启停、触发记录、关键词任务执行、会话成员筛选、会话列表、会话存档企业配置和会话同步入库；关键词任务现在会扫描 `mc_work_message_1` 至 `mc_work_message_10`，按员工范围、精确关键词和模糊关键词命中规则，达到触发次数后写入 `mc_auto_tag_record.status=0`，投递 `mark-tags` 队列，并由 worker 写入客户标签 pivot、客户互动轨迹、企业微信 `externalcontact/mark_tag` 和 `mc_auto_tag_record.status=1`/`mark_tag_count`；任务重跑会重投递未完成的 pending 记录。2026-07-06 已补入群行为自动标签第一版：`change_external_chat.create/update` 同步单群后，会按 `mc_auto_tag.type=2`、`on_off=1` 的 `tag_rule.rooms/tags` 匹配当前外部联系人成员，按 `auto_tag_id/tag_rule_id/contact_id/employee_id/contact_room_id` 幂等写入 `mc_auto_tag_record.status=0` 并投递 `mark-tags`，后续仍复用 MarkTags worker 落企业微信标签和回写状态。2026-07-06 已补分时段自动标签第一版：`change_external_contact.add_external_contact` 同步客户关系后，会按 `mc_auto_tag.type=3` 的 `employees`、`tag_rule.time_type/schedule/start_time/end_time/tags` 匹配 `mc_work_contact_employee.create_time`，按规则周期幂等写入 `mc_auto_tag_record.status=0` 并投递 `mark-tags`。`scripts/smoke_wework_callback_worker.sh` 已覆盖新增客户回调触发分时段自动标签记录、客户群回调触发入群自动标签记录、回调触发个人/入群群 SOP 日志和对应 `mark-tags` 入队。`scripts/smoke_work_message_archive_sync_cron.sh` 已覆盖 bridge 拉取、分表入库、游标推进和敏感词消费，`scripts/smoke_auto_tag_keyword_task.sh` 已覆盖新消息触发和 pending 记录恢复，`autoTag.js`、`workMessage.js`、`workMessageConfig.js` 已无缺口。2026-07-06 已在 `permissionByUser` 响应层补齐旧 dashboard hidden 自动标签列表页的路由注册兼容，`scripts/smoke_dashboard_frontend_login.sh` 已覆盖 `/autoTag/keywordIndex`、`/autoTag/keywordCreate`、`/autoTag/keywordShow?idRow=918001`、`/autoTag/joinRoomIndex`、`/autoTag/joinRoomCreate`、`/autoTag/joinRoomShow?idRow=918002`、`/autoTag/dayPartIndex`、`/autoTag/dayPartCreate`、`/autoTag/dayPartShow?idRow=918003`，并验证 `/dashboard/autoTag/index?type=1/2/3` 均返回 200 且页面不再渲染到 `/404`。
   - `scripts/smoke_auto_tag_dashboard.sh` 已用真实 Go standalone 覆盖三类自动标签规则新建、列表、详情、触发记录、启停、删除、会话成员筛选、会话列表和会话存档配置读写，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`。
   - 抽奖活动后台 `dashboard/lottery/index/store/showContact/show/destroy/share/update/info/writeOff/batchContactTags` 已完成第一批 Go 接管，覆盖活动 CRUD、客户列表、活动详情、分享链接、核销和批量打标签；新增 Go 原生 `/dashboard/lottery/page` 页面，使用 dashboard JWT 直接管理抽奖活动、奖品详情、分享链接和中奖客户，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_lottery_dashboard.sh` 已用真实 Go standalone 覆盖抽奖活动新建、列表、详情、分享、中奖客户、核销、批量打标签、更新、删除、级联软删和 `lotteries` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增活动已接入 `lotteries` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；活动更新和删除会回收 `prize_set`、`exchange_set`、`draw_set`、`win_set`、`corp_card` 和中奖记录 `receive_qr` 中的本地上传文件账本并刷新 `storage_mb`；`lottery.js` 已无缺口。
   - 群裂变后台 `dashboard/roomFission/index/store/info/update/destroy/invite/show/showRoom/showContact/writeOff` 已完成第一批 Go 接管，覆盖活动 CRUD、四步编辑数据、邀请配置、群聊数据、客户数据、分享链接和核销；新增 Go 原生 `/dashboard/roomFission/page` 页面，使用 dashboard JWT 直接管理群裂变活动、海报欢迎语、邀请配置、群聊数据、参与客户和核销状态，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_room_fission_dashboard.sh` 已用真实 Go standalone 覆盖群裂变新建、列表、详情、群聊、客户、邀请、更新、核销、删除、级联软删和 `room_fissions` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增活动已接入 `room_fissions` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；活动更新、邀请配置更新和删除会回收海报、群二维码、欢迎语、邀请封面等本地上传文件账本并刷新 `storage_mb`；`roomFission.js` 已无缺口。
   - 群打卡后台 `dashboard/roomClockIn/index/store/update/destroy/show/showContact/batchContactTags/info/dayDetail` 已完成第一批 Go 接管，覆盖活动 CRUD、客户列表、打卡天数明细和批量打标签；新增 Go 原生 `/dashboard/roomClockIn/page` 页面，使用 dashboard JWT 直接管理群打卡活动、参与客户和打卡明细，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_room_clock_in_dashboard.sh` 已用真实 Go standalone 覆盖群打卡新建、列表、详情、客户列表、天数明细、批量打标签、更新、停用状态、删除、级联软删和 `room_clock_ins` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增活动已接入 `room_clock_ins` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；群打卡更新和删除会回收 `employee_qrcode` 对应的本地上传文件账本并刷新 `storage_mb`；`roomClockIn.js` 已无缺口。
   - 群质检后台 `dashboard/roomQuality/index/store/status/info/update/showContact/destroy/contactDetail` 已完成第一批 Go 接管，覆盖规则 CRUD、启停、触发客户列表和客户触发详情；新增 Go 原生 `/dashboard/roomQuality/page` 页面，使用 dashboard JWT 直接管理质检规则、启停和触发客户记录，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_room_quality_dashboard.sh` 已用真实 Go standalone 覆盖群质检新建、列表、弹窗详情、触发客户列表、触发客户详情、启停、更新、删除、触发记录级联软删和 `room_qualities` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增规则已接入 `room_qualities` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；`roomQuality.js` 已无缺口。
   - 群日历后台 `dashboard/roomCalendar/index/addRoom/destroyRoom/store/destroy/show/update` 已完成第一批 Go 接管，覆盖日历 CRUD、群聊增删和推送内容保存；新增 Go 原生 `/dashboard/roomCalendar/page` 页面，使用 dashboard JWT 直接管理群日历列表、启停和推送计划，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_room_calendar_dashboard.sh` 已用真实 Go standalone 覆盖群日历新建、列表、详情、增加群聊、移除群聊、更新推送计划、删除、推送和记录级联软删、`room_calendar_push.room_calendar_id` 字符串兼容查询和 `room_calendars` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；列表推送数查询已改为数值比较，避免 `mc_room_calendar_push.room_calendar_id` 与 `CAST(c.id AS CHAR)` 在 MariaDB/MySQL 下出现 collation 冲突；新增日历已接入 `room_calendars` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；`roomCalendar.js` 已无缺口。
   - 客户群提醒后台 `dashboard/roomRemind/index/destroy/info/status/store/update` 和 `dashboard/task/roomRemind` 已完成第一批 Go 接管，覆盖规则 CRUD、GET 启停、任务查询和触发记录统计；新增 Go 原生 `/dashboard/roomRemind/page` 页面，使用 dashboard JWT 直接管理提醒规则、启停和启用任务视图，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_room_remind_dashboard.sh` 已用真实 Go standalone 覆盖客户群提醒新建、列表、详情、任务视图、启停、更新、删除、提醒记录级联软删和 `room_reminds` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增提醒规则已接入 `room_reminds` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；`roomRemind.js` 已无缺口。
   - 无限拉群后台 `dashboard/roomInfinitePull/index/info/update/destroy/store` 已完成第一批 Go 接管，覆盖活动 CRUD、企微活码 JSON、扫码人数和详情二维码链接；新增 Go 原生 `/dashboard/roomInfinitePull/page` 页面，使用 dashboard JWT 直接管理无限拉群活动、群名称/引导语显示配置和企微活码 JSON，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面，并已纳入 `scripts/smoke_dashboard_frontend_login.sh` 真实浏览器验收；`scripts/smoke_room_infinite_pull_dashboard.sh` 已用真实 Go standalone 覆盖无限拉群新建、列表、详情、更新、删除、企微活码 JSON、详情链接和 `room_infinite_pulls` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增活动已接入 `room_infinite_pulls` SaaS 套餐额度，创建前拦截、创建/删除后刷新用量；活动更新和删除会回收 `avatar`、`logo`、`qw_code` 活码 JSON 二维码字段对应的本地上传文件账本并刷新 `storage_mb`；新增 `0021_room_infinite_pull` 后继续补齐 `workDepartment/memberIndex` 旧别名、`contactMessageBatchSend/messageShow` 预览、`clockIn/index` 旧别名和 `workRoomAutoPull/move` 幂等兼容接口。2026-07-05 复算 dashboard 前端源码 API 去重口径为 `316` 个接口中 `316` 个已迁移，`missing=0`。
   - `scripts/smoke_operation_frontend_work_fission.sh` 已覆盖 operation 任务宝真实浏览器链路：脚本使用独立 MySQL/Redis、Go operation session 和 Go standalone，访问 `/workFission?id=900001` 与 `/speed`，验证 `openUserInfo/workFission` 同源别名、海报、任务数据、邀请好友、领奖写入、本地静态资源和旧构建产物里的 `/undefined/operation/*` API 前缀归一化。
   - 后续还需要把套餐额度继续接入更多运行时资源，继续覆盖更多引用上传文件的业务删除/替换入口，并把 SaaS 告警继续做成更细的租户级可配置自动处置策略；通知模板、webhook 和持久化重试队列已具备租户级后台配置第一版。

## 下一批执行优先级

1. 继续把 SaaS 套餐额度接入更多运行时资源，继续覆盖更多上传文件引用入口的触发式账本回收，并把 `mochat_go_saas_alerts` 的处置策略继续细化到租户级；通知模板、webhook 和持久化失败重试队列已具备租户级配置第一版。
2. 继续迁移企业微信细粒度业务事件 listener，并把现有全量同步 fallback 逐步替换为按事件业务主键处理；work-fission 后续重点转向更深度业务回归、异常告警和失败补偿。
3. 继续补通讯录、客户、标签、客户群同步的真实链路回归和细粒度事件迁移。
4. 继续迁移营销插件和风控模块；批量加好友后台、敏感词词库/监控、个人 SOP 和群 SOP 管理端已完成第一批接管，敏感词监控 cron、会话存档同步 cron 和 SOP 日志/提醒 cron 已补第一版，后续还要为其它插件建立“创建、查看、执行、同步结果、删除”的最小回归链路，并补企业微信官方会话存档 SDK bridge 的生产部署、真实企业账号回归，继续补 SOP 更多目标事件锚点和生产数据回归。
5. 对 dashboard/sidebar/operation 继续补真实浏览器 smoke，扩大已迁移营销插件、风控模块和 SaaS 运营页的页面级回归，并决定生产部署采用独立域名/端口还是重打包 publicPath 后单域名发布。

## 验收命令

当前独立运行基础验收：

```bash
./scripts/standalone_acceptance.sh
./scripts/smoke_standalone.sh
./scripts/standalone_stack_check.sh
./scripts/standalone_soak_24h.sh
./scripts/standalone_stage_report.sh
./scripts/smoke_standalone_compose_app.sh
./scripts/standalone_inventory_parity.sh
./scripts/audit_acceptance_suite_coverage.sh
./scripts/audit_manifest_route_smoke_coverage.sh
./scripts/audit_functional_module_matrix.sh
./scripts/audit_queue_annotation_coverage.sh
./scripts/audit_saas_metric_coverage.sh
./scripts/audit_saas_storage_reclaim_coverage.sh
./scripts/audit_wework_callback_event_coverage.sh
./scripts/ci_mysql57_amd64.sh
./scripts/production_evidence_check.sh
./scripts/audit_goal_completion.sh
./scripts/smoke_production_evidence_gate.sh
./scripts/source_fingerprint.py
./scripts/standalone_route_coverage.sh
./scripts/collect_standalone_evidence.sh
./scripts/smoke_independent_package.sh
./scripts/smoke_bootstrap_standalone.sh
./scripts/smoke_saas_provisioning.sh
./scripts/smoke_saas_tenant_isolation.sh
./scripts/smoke_saas_quota_enforcement.sh
./scripts/smoke_saas_storage_reconcile.sh
./scripts/smoke_saas_storage_reclaim.sh
./scripts/smoke_saas_usage_refresh.sh
./scripts/smoke_official_account_ticket.sh
./scripts/smoke_queue_idempotency.sh
./scripts/smoke_async_file_upload_worker.sh
./scripts/smoke_mark_tags_worker.sh
./scripts/smoke_auto_tag_keyword_task.sh
./scripts/smoke_message_remind_worker.sh
./scripts/smoke_work_room_sync_worker.sh
./scripts/smoke_work_contact_sync_worker.sh
./scripts/smoke_work_department_list_worker.sh
./scripts/smoke_media_id_update_worker.sh
./scripts/smoke_employee_statistic_worker.sh
./scripts/smoke_work_contact_tag_remote_write.sh
./scripts/lint_mysql57_schema.sh
./scripts/smoke_mysql57_schema_migrate.sh
./scripts/smoke_wework_callback_worker.sh
./scripts/smoke_employee_apply_worker.sh
./scripts/smoke_pull_agent_cron.sh
./scripts/smoke_employee_statistic_cron.sh
./scripts/smoke_channel_code_cron.sh
./scripts/smoke_contact_batch_send_cron.sh
./scripts/smoke_room_batch_send_cron.sh
./scripts/smoke_contact_sync_send_result_cron.sh
./scripts/smoke_room_sync_send_result_cron.sh
./scripts/smoke_room_tag_pull_cron.sh
./scripts/smoke_corp_data_cron.sh
./scripts/smoke_media_id_update_cron.sh
./scripts/smoke_transfer_state_refresh_cron.sh
./scripts/smoke_sensitive_word_monitor_cron.sh
./scripts/smoke_frontend_static_browser.sh
./scripts/smoke_dashboard_frontend_login.sh
./scripts/smoke_sidebar_frontend_contact.sh
./scripts/smoke_operation_frontend_work_fission.sh
./scripts/smoke_sensitive_word_dashboard.sh
./scripts/smoke_channel_code_dashboard.sh
./scripts/smoke_shop_code_dashboard.sh
./scripts/smoke_radar_dashboard.sh
./scripts/smoke_lottery_dashboard.sh
./scripts/smoke_room_fission_dashboard.sh
./scripts/smoke_room_infinite_pull_dashboard.sh
./scripts/smoke_room_clock_in_dashboard.sh
./scripts/smoke_room_quality_dashboard.sh
./scripts/smoke_room_calendar_dashboard.sh
./scripts/smoke_room_remind_dashboard.sh
./scripts/smoke_sop_dashboard.sh
./scripts/smoke_auto_tag_dashboard.sh
./scripts/smoke_greeting_dashboard.sh
./scripts/smoke_room_welcome_dashboard.sh
./scripts/smoke_work_room_auto_pull_dashboard.sh
./scripts/smoke_room_tag_pull_dashboard.sh
./scripts/smoke_contact_message_batch_send_dashboard.sh
./scripts/smoke_room_message_batch_send_dashboard.sh
./scripts/smoke_contact_batch_add_dashboard.sh
./scripts/smoke_room_fission_dashboard.sh
./scripts/smoke_work_fission_dashboard.sh
```

`scripts/standalone_acceptance.sh` 是后续长时间执行的统一入口：默认跑 `all`，覆盖快速门禁、迁移期清单对齐、schema migration、standalone 容器、路由覆盖、SaaS、worker、cron、前端和 MySQL 5.7 验证，不运行 PHP fallback 或真实 PHP 对照；`core` 套件会在本地存在 `MOCHAT_SOURCE_ROOT` 或默认 `../mochat` 源码时运行 `standalone_inventory_parity.sh`，没有源码时跳过该迁移期清单门禁，不影响 Go standalone 运行时独立性；独立交付证据包可设置 `MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1` 强制跳过 PHP 清单对齐，只用 Go 内置 manifest 和 standalone smoke 自证；core 还会运行生产证据门禁规则自测，确认模板证据和 MySQL 5.7 arm64 skip 日志不会被误判为有效生产证据；`frontend` 套件除三端浏览器 smoke 外，也会跑 `scripts/smoke_admin_core_dashboard.sh`、`scripts/smoke_sensitive_word_dashboard.sh`、`scripts/smoke_channel_code_dashboard.sh`、`scripts/smoke_shop_code_dashboard.sh`、`scripts/smoke_radar_dashboard.sh`、`scripts/smoke_lottery_dashboard.sh`、`scripts/smoke_room_fission_dashboard.sh`、`scripts/smoke_room_infinite_pull_dashboard.sh`、`scripts/smoke_room_clock_in_dashboard.sh`、`scripts/smoke_room_quality_dashboard.sh`、`scripts/smoke_room_calendar_dashboard.sh`、`scripts/smoke_room_remind_dashboard.sh`、`scripts/smoke_sop_dashboard.sh`、`scripts/smoke_greeting_dashboard.sh`、`scripts/smoke_room_welcome_dashboard.sh`、`scripts/smoke_work_room_auto_pull_dashboard.sh`、`scripts/smoke_room_tag_pull_dashboard.sh`、`scripts/smoke_contact_message_batch_send_dashboard.sh`、`scripts/smoke_room_message_batch_send_dashboard.sh`、`scripts/smoke_contact_batch_add_dashboard.sh` 和 `scripts/smoke_work_fission_dashboard.sh`，验证管理端基础 CRUD/upload、敏感词、渠道活码、门店活码、互动雷达、抽奖活动、群裂变、无限拉群、群打卡、群质检、群日历、客户群提醒、个人/群 SOP、好友欢迎语、入群欢迎语、自动拉群、标签建群、客户群发、客户群群发、批量加好友和任务宝裂变活动的 dashboard API 写链路；`scripts/audit_acceptance_suite_coverage.sh` 会防止新增 `smoke_*.sh` 没有接入该总验收入口，`scripts/audit_saas_metric_coverage.sh` 会防止新增 SaaS 指标没有同步到 bootstrap、运行时计数和 smoke，`scripts/audit_saas_storage_reclaim_coverage.sh` 会防止新增上传文件回收函数没有对应触发式 smoke，`scripts/audit_wework_callback_event_coverage.sh` 会防止企微回调新增事件路径没有对应 worker smoke；可用 `MOCHAT_ACCEPTANCE_SUITE=core|saas|workers|cron|frontend|mysql57` 分段执行。迁移期需要和 PHP 原系统对照时，单独执行 `MOCHAT_ACCEPTANCE_SUITE=php MOCHAT_ACCEPTANCE_INCLUDE_PHP=1 ./scripts/standalone_acceptance.sh`。

`scripts/standalone_soak_24h.sh` 是可选持续运行验收入口，默认 `MOCHAT_SOAK_DURATION_SECONDS=86400`、`MOCHAT_SOAK_INTERVAL_SECONDS=60`，会启动独立 MySQL/Redis 和 Go standalone，并在每轮探测中验证 `/readyz` 不依赖 PHP/source/manifest、`/compat/routes` 仍有 224 条业务路由、dashboard/sidebar/operation 前端入口可访问、未知路由返回 Go standalone 501、MySQL/Redis 仍健康，结果写入 `soak.ndjson`。当前阶段不把 24 小时 soak 作为继续开发的必跑项；需要发版前长稳证据时再单独启动。

`scripts/smoke_independent_package.sh` 会把当前 `mochat-go` 复制到没有原 `mochat/` 兄弟目录的临时目录，污染 `MOCHAT_SOURCE_ROOT` 和 `MOCHAT_COMPAT_MANIFEST`，再执行 standalone 独立性审计、embedded 队列注解覆盖审计和 standalone smoke；它用于证明交付出去的 Go 包不依赖本机原 PHP checkout，也不会启动 24 小时持续运行。

`scripts/audit_goal_completion.sh` 会按用户目标生成完成度审计报告，把独立运行、脱离原 PHP、PHP 源码清单对齐、全部功能本地迁移、SaaS/worker/cron/frontend、本地短证据包、生产外部证据和可选 24 小时稳定性证据放到同一张检查表里；报告内部会用严格生产证据检查标出真实外部证据缺口，默认读取 `docs/evidence/latest`，也可用 `MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR` 指向生产候选或自定义证据目录，用 `MOCHAT_GOAL_COMPLETION_SOURCE_ROOT` 指向原 PHP 源码目录。默认不阻断阶段交接，设置 `MOCHAT_GOAL_COMPLETION_STRICT=1` 后目标未完成会返回非 0。`scripts/collect_standalone_evidence.sh` 的 `index.md` 会把该报告标为“报告生成”并展示报告内结论，避免把报告返回码误判为目标完成。

`scripts/source_fingerprint.py` 会对 Go 源码、验收脚本、部署配置、前端构建产物和关键构建文件生成 sha256 指纹；`scripts/collect_standalone_evidence.sh` 会把指纹写入当前证据目录的 `source-fingerprint.json`，`scripts/audit_goal_completion.sh` 会对比指定证据包指纹和当前指纹，防止代码或验收脚本变更后继续沿用旧证据包。

`scripts/smoke_production_evidence_gate.sh` 会自测生产证据门禁规则：模板证据必须被拒绝、MySQL 5.7 arm64 skip 日志必须被拒绝、只有关键词但没有结构化证据记录的文件必须被拒绝，包含必填核验项、明确通过结论和结构化证据记录的证据文件才能通过。该脚本只验证证据文件规则，不启动 24 小时持续运行。

frontend 套件当前覆盖 dashboard 登录业务页、三端静态资源、管理端基础 CRUD/upload API 写链路、敏感词分组/词库/监控 API 写链路、渠道活码分组/新建/更新/列表/详情/客户明细/统计 API 写链路、门店活码新建/详情/位置/地址检索/分享/页面设置/统计/客户明细/员工和二维码更新/状态/删除 API 写链路、互动雷达新建/渠道/客户点击/更新/删除 API 写链路、抽奖活动新建/中奖客户/核销/打标签/更新/删除 API 写链路、群裂变新建/列表/详情/群聊/客户/邀请/更新/核销/删除 API 写链路、无限拉群新建/列表/详情/更新/删除 API 写链路、群打卡新建/客户列表/天数明细/打标签/更新/停用/删除 API 写链路、群质检新建/触发客户列表/触发客户详情/启停/更新/删除 API 写链路、群日历新建/增加群聊/移除群聊/更新推送计划/删除 API 写链路、客户群提醒新建/任务视图/启停/更新/删除 API 写链路、个人/群 SOP 新建/范围设置/启停/更新/删除 API 写链路、自动标签三类规则新建/触发记录/启停/删除/会话列表/会话存档配置 API 写链路、好友欢迎语全员/指定员工新增、列表、详情、编辑、删除 API 写链路、入群欢迎语新增/查看/编辑/删除 API 写链路、自动拉群新增/查看/更新 API 写链路、标签建群筛客/新建/查看/提醒/删除 API 写链路、客户群发立即发送/详情/明细/提醒/删除 API 写链路、客户群群发立即发送/详情/明细/提醒/删除 API 写链路、批量加好友设置/导入/分配/删除 API 写链路、任务宝裂变活动新建/读取/统计/邀请/更新/删除 API 写链路、sidebar `/login` 企业微信 OAuth/code 回调、sidebar 客户详情页、sidebar 个人 SOP 提醒和详情页、sidebar 群 SOP 页面、sidebar 批量加好友页面、sidebar 素材库页面、operation 任务宝 OAuth 外跳/code 回调和活动页。Go runtime 会把旧前端构建里可能出现的 `/undefined/dashboard/*`、`/undefined/sidebar/*`、`/undefined/operation/*` 归一化到实际 API 路径；operation 前端同源访问的 `/auth/workFission`、`/openUserInfo/workFission` 也会映射到已迁移 handler，避免独立部署时因为历史 publicPath/API base 注入问题把已迁移接口误判为静态资源。

完整独立版最终验收必须额外满足：

- `/compat/routes` 中 224 条业务路由都有 Go handler 或明确下线决策。
- standalone 模式下核心页面和业务接口不返回 501。
- 停掉 PHP 服务、移走原 `mochat/` 目录后，Go 服务、数据库、Redis/队列和前端仍能完成核心业务闭环。
- 使用真实企业微信/微信开放平台账号验证授权、回调解密、通讯录、客户、客户群、标签、素材、公众号和企微应用链路，而不仅是 fake API 或本地种子数据。
- dashboard、sidebar、operation 的生产构建产物要覆盖更多真实浏览器页面流转，至少覆盖登录、企业选择、首页、通讯录、客户、客户群、营销插件和 SaaS 管理页面的关键读写路径。
- SaaS 多租户隔离要用两个以上租户的真实业务数据做回归，验证菜单权限、企业归属、资源额度、上传账本、异步任务和告警只作用于当前租户。
- 本轮已按用户要求停止 24 小时持续 run；如后续需要生产稳定性补证，可重新执行 `scripts/standalone_soak_24h.sh` 并保留 `soak.ndjson`。
- MySQL 5.7 真实容器迁移 smoke 仍需在 amd64 CI 上执行通过；本机 arm64 跳过不能作为最终生产兼容证据。

## 阶段交接（2026-07-07 17:12）

当前开发阶段判断：

- 方案 1 的 Go 独立版主体迁移已经进入“收口验收/生产化补证”阶段，不是早期开发阶段。
- 独立运行底座已经成立：Go standalone 可在无 PHP upstream、无原 `mochat/` 源码目录、无外部 compat manifest 的情况下启动，并托管 dashboard/sidebar/operation 三端入口。
- 功能迁移覆盖已经进入强审计状态：manifest 224 条业务路由均命中 Go handler，`scripts/audit_manifest_route_smoke_coverage.sh` 要求 224 条路由都在 `scripts/smoke_*.sh` 中有直接覆盖痕迹，`scripts/audit_functional_module_matrix.sh` 进一步把 224 条 method+path 路由和 Go 运行时迁移路由归入 28 个业务功能模块，并检查每个模块的 Go 源码、acceptance smoke 和关键 SaaS/队列/企微事件证据。
- SaaS 基础能力已具备第一版闭环：多租户开通、租户隔离、套餐额度、上传账本、用量刷新、告警通知配置、公众号开放平台授权和双租户存储隔离均已有独立栈 smoke。
- 2026-07-07 08:40:28 CST 至 2026-07-07 17:10:07 CST 的持续 run 已按用户要求停止，累计 502 轮探测，`route_total` 始终为 `224`，`migrated_route_count` 始终为 `407`，RSS 范围为 `7824-19056 KB`；该记录证明早期稳定性良好，但不是 24 小时完成证据。
- `scripts/standalone_stage_report.sh` 可直接生成当前阶段报告，汇总 manifest、smoke/acceptance 覆盖、独立交付包动态 smoke、最近持续 run 记录、容器残留和未完成生产证据缺口。
- `scripts/production_evidence_check.sh` 可直接生成生产证据检查报告，并会纳入独立交付包动态 smoke 和功能模块矩阵审计；上线前应设置 `MOCHAT_PRODUCTION_EVIDENCE_STRICT=1` 并提供 MySQL 5.7 amd64、真实企微、真实微信开放平台、真实 SaaS 租户、生产前端和持续运行证据文件。真实联调类证据除关键词和通过结论外，还必须保留执行时间、证据记录、请求/响应摘要、截图/日志/工单引用或对应业务结构化记录项。
- `scripts/audit_goal_completion.sh` 可直接生成目标完成度审计报告；当前会把本地独立迁移证据判为通过，并用严格生产证据检查把真实外部生产证据判为失败/未完成，因此不能用于标记最终完成。自定义证据目录或生产候选本地证据包应通过 `MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR` 传入，避免误读旧 latest。
- `scripts/init_production_evidence_pack.sh` 可初始化生产证据模板；模板包含 `MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE`，必须在真实验收完成后删除该标记，否则严格证据检查会失败。
- `scripts/smoke_production_evidence_gate.sh` 可自测生产证据规则，确认模板证据、MySQL 5.7 arm64 skip 日志、只有关键词但缺少结构化证据记录的文件会被拒绝，只有含必填核验项、明确通过结论和结构化证据记录的证据才会被接受；该脚本不会启动 24 小时持续运行。
- `scripts/collect_production_evidence_pack.sh` 可把真实生产证据文件收拢成可复核包，默认从 `docs/evidence/production/` 复制标准文件到 `docs/evidence/production/current/`，生成 `env.production-evidence`、`manifest.json`、sha256 摘要和 `index.md`，先执行严格生产证据文件检查，再默认运行 skip-local 生产候选门禁；只想先检查证据文件时可设置 `MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE=0`。该脚本不会启动 24 小时持续运行。
- `scripts/smoke_independent_package.sh` 可验证独立交付包口径：复制当前 Go 项目到没有原 PHP 项目的临时目录，使用 embedded manifest 跑独立性审计、队列覆盖审计和 standalone smoke，不启动 24 小时持续运行。
- `scripts/collect_standalone_evidence.sh` 可生成本地短验收证据包，默认写入并重建 `docs/evidence/latest/`，包含快速门禁、独立交付包 smoke、迁移期 PHP 清单对齐、`MOCHAT_ACCEPTANCE_SUITE=<suite> MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1` 独立验收、运行时路由覆盖、阶段报告、生产证据检查、目标完成度审计、命令日志、运行残留检查和汇总 `index.md`；默认只跑 `MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=core`，需要扩大到全套非 PHP 短验收时可设置 `MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57'`，或使用 `all` 作为简写。该脚本默认 `MOCHAT_LOCAL_EVIDENCE_INVENTORY_PARITY=auto`，本地存在原 `../mochat/api-server` 或 `MOCHAT_LOCAL_EVIDENCE_SOURCE_ROOT` 指定源码时会记录 `inventory-parity`，用于证明 embedded manifest 与原 PHP 扫描清单一致；同时它仍会设置 `MOCHAT_QUEUE_AUDIT_SOURCE=embedded`，并把独立验收的 `MOCHAT_SOURCE_ROOT`、`MOCHAT_COMPAT_MANIFEST` 指到不存在的污染路径，强制证明独立交付包不需要原 PHP 源码或外部 manifest 也能自证。目标完成度审计读取当前输出目录，不会被旧 latest 证据影响；它不会启动 24 小时持续运行。
- `scripts/production_candidate_gate.sh` 可作为最终生产候选入口，默认先执行全套非 PHP 本地短验收证据包，再执行严格生产证据检查、严格目标完成度审计和源码/验收指纹匹配；只有本地短验收、全部外部生产证据、目标完成度审计和候选证据包指纹都通过时才返回 0。默认模式下目标审计读取 `docs/evidence/production/candidate/local`，可用 `MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1` 跳过本地短验收，只复核已经收集好的外部生产证据、目标完成度和 `docs/evidence/latest/source-fingerprint.json` 是否仍匹配当前源码；候选报告首页会展示两个严格报告的当前结论，并展示当前指纹、对比来源和匹配结果。

当前仍需补证的上线项：

- 真实企业微信/微信开放平台账号联调：授权、回调解密、通讯录、客户、客户群、标签、素材、公众号和企微应用链路需要用真实账号跑通。
- MySQL 5.7 真实容器迁移：本机 arm64 仍按脚本策略跳过，需要在 amd64 CI 或等价环境执行 `scripts/ci_mysql57_amd64.sh`；GitHub 环境可直接使用 `.github/workflows/mysql57-amd64.yml` 生成 `mysql57-amd64-evidence` artifact。
- SaaS 生产化验收：需要真实租户业务数据验证菜单权限、企业归属、资源额度、上传账本、异步任务、告警和运维处置只作用于当前租户。
- 生产前端回归：dashboard/sidebar/operation 还需要按真实业务操作路径扩大浏览器回归，覆盖更多生产页面流转和异常路径。

## 完成审计（2026-07-07 17:12）

按用户原始目标拆解，当前不能标记为最终完成：

- 独立 Go 运行时：证据较强。`MOCHAT_GO_STANDALONE=1`、无 PHP/source/manifest 残留、内置 manifest、独立 MySQL/Redis、三端前端入口和未知路由 501 已由 `scripts/test.sh`、`scripts/standalone_acceptance.sh`、`scripts/standalone_stack_check.sh`、`scripts/smoke_standalone_compose_app.sh` 和本轮 502 轮持续 run 共同覆盖。
- 全功能迁移：路由层证据较强但仍不是生产完成证明。内置 manifest 当前包含 224 条路由、70 张表、9 个 crontab、10 个事件处理器和 15 条异步队列注解；`scripts/audit_manifest_route_smoke_coverage.sh` 已证明 224 条 manifest 路由均有 smoke 直接覆盖痕迹，`scripts/standalone_route_coverage.sh` 已证明业务路由 `224/224` 命中 Go handler。但真实企业微信、微信开放平台和生产业务数据仍未完成最终验收。
- SaaS 化：基础能力证据较强。已有多租户开通、租户隔离、套餐额度、上传账本、用量刷新、告警通知配置、公众号开放平台授权和双租户存储隔离 smoke；但最终 SaaS 完成仍需要真实租户业务数据、生产权限矩阵、计费/运维流程和真实外部账号联调。
- 持续运行稳定性：本轮 24 小时 run 已按用户要求停止，形成 502 轮早期稳定性证据；后续如需生产稳定性证据，可重新跑满 `scripts/standalone_soak_24h.sh`。
- 生产兼容性：未完成。MySQL 5.7 真实容器 smoke 在本机 arm64 仍按策略跳过，必须在 amd64 CI 或等价环境执行 `scripts/ci_mysql57_amd64.sh` 通过后，才能作为生产基线兼容证据。

## 阶段补充（2026-07-07 23:52）

- 本轮未启动 24 小时持续运行；当前也未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 已强化 `scripts/production_evidence_check.sh` 和 `scripts/smoke_production_evidence_gate.sh`：真实生产证据除关键词和明确通过结论外，还必须包含结构化证据记录；模板证据、MySQL 5.7 arm64 skip 日志、只有关键词但缺少结构化记录的文件都会被拒绝。
- 已执行 `env -u GOROOT ./scripts/smoke_production_evidence_gate.sh`，生产证据门禁规则自测通过。
- 已执行全套非 PHP 本地短验收证据包：`env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh`，最新证据目录为 `docs/evidence/latest/`。
- 最新本地短证据结论：`本地短门禁通过，目标未完成，仍需补生产证据`；manifest 路由 `224`，已迁移 manifest 路由 `224`，未迁移 `0`，Go 额外运行时路由 `183`。
- 最新源码与验收指纹：`8ff8d99d7a7622cc2d4b315e6d07cc5cf8b584a9be7f3e71e58e77500bb10a07`，纳入文件数 `589`。
- 已执行 `env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`；生产候选门禁未通过，当前只确认仍缺真实生产证据，源码与最新本地短证据包指纹匹配。
- 2026-07-07 23:55 复核 `env -u GOROOT ./scripts/standalone_inventory_parity.sh` 通过，当前 PHP 源码扫描与 embedded manifest 对齐：routes `224/224`、tables `70/70`、crontabs `9/9`、event handlers `10/10`、async queues `15/15`。随后补强 `scripts/collect_standalone_evidence.sh`，默认在本地原 PHP 源码存在时把 `inventory-parity` 纳入证据包；补强 `scripts/audit_goal_completion.sh`，把 PHP 清单对齐纳入“全部 manifest 功能已纳入 Go 本地验收”的完成条件，并用 `MOCHAT_GOAL_COMPLETION_SOURCE_ROOT` 单独指定原 PHP 源码目录，避免被独立验收的污染 `MOCHAT_SOURCE_ROOT` 误伤。该补强不改变独立运行测试的污染环境口径，也不启动 24 小时持续运行。
- 2026-07-08 00:42 复跑 `env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 通过，最新证据写入 `docs/evidence/latest/`。本次证据新增 `inventory-parity`，目标完成度审计显示 `能运行的独立 Go 项目`、`不依赖原 mochat/PHP 项目`、`全部 manifest 功能已纳入 Go 本地验收`、`SaaS、worker、cron、frontend 本地闭环` 均通过；生产外部证据仍未完成。最新指纹为 `5d39dccadeef8539c31d71628df2ac0b28523658726cd792b7e9bb324e3577fe`、文件数 `589`，运行时路由仍为 `224/224`、缺失 `0`、Go 额外运行时路由 `183`。随后复跑 `env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh` 返回 `rc=1`，严格生产证据检查和严格目标完成度审计按预期因真实外部证据缺失失败，源码指纹匹配通过；本轮未启动 24 小时持续运行，未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 2026-07-08 00:48 新增 `scripts/collect_production_evidence_pack.sh`，用于把真实生产证据收拢到 `docs/evidence/production/current/`，生成 `env.production-evidence`、`manifest.json`、sha256 摘要和 `index.md`，并执行严格生产证据文件检查；默认还会运行 skip-local 生产候选门禁，且不会启动 24 小时持续运行。已将该脚本接入 `scripts/smoke_production_evidence_gate.sh`：模板证据包会失败，结构化样例证据包会通过。用当前 `docs/evidence/production/` 运行 `MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE=0 ./scripts/collect_production_evidence_pack.sh` 返回 `rc=1`，报告写入 `docs/evidence/production/current/index.md`，当前 6 类真实生产证据均缺失。

## 阶段补充（2026-07-08 01:58）

- 本轮按用户要求不再执行 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 修复 `scripts/smoke_async_file_upload_saas_usage.sh` 的异步源文件删除竞态：worker 会先写目标文件和存储账本，再删除本地源文件；smoke 现在通过 `wait_file_absent` 等待最终删除状态，避免把异步收尾时序误判为失败。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_async_file_upload_saas_usage.sh` 通过；随后执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=workers MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1 ./scripts/standalone_acceptance.sh`，`suite=workers` 于 `2026-07-08 01:11:30 CST` 通过。
- 首次重建全套短证据时 `cron` 因 `13325` 临时端口占用失败；端口释放后单独执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=cron MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1 ./scripts/standalone_acceptance.sh`，`suite=cron` 于 `2026-07-08 01:34:15 CST` 通过，确认不是 cron 功能失败。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 01:56:09 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。本次覆盖 `test`、独立交付包 smoke、PHP 清单对齐、`core/saas/workers/cron/frontend/mysql57`、运行时路由覆盖、阶段报告、生产证据检查和目标完成度审计。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `acf74e77c42f7f722bdc33efa40db130323bdea37ee84587f824fc6103cd7259`，纳入文件数 `590`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 01:56:59 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 01:57:45 CST`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 02:30）

- 本轮继续遵守用户最新要求，没有启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 尝试在当前 arm64 Docker Desktop 上用 `DOCKER_DEFAULT_PLATFORM=linux/amd64 MOCHAT_FORCE_MYSQL57=1 MOCHAT_MYSQL57_PORT=13372 ./scripts/smoke_mysql57_schema_migrate.sh` 生成 MySQL 5.7 amd64 真实容器证据，`mysql:5.7` 初始化阶段触发 `qemu: uncaught target signal 11 (Segmentation fault)`，验证本机不能作为 MySQL 5.7 amd64 生产证据来源；该失败日志未导入生产证据。
- 新增 `scripts/import_mysql57_amd64_evidence.sh`，用于校验并导入 `.github/workflows/mysql57-amd64.yml` 产出的 `mysql57-amd64.log` 或 artifact zip，要求包含 `mysql57 amd64 CI gate passed` 或 `mysql 5.7 schema migration smoke passed`，并拒绝 arm64 skip 日志；通过后会复制到 `docs/evidence/production/mysql57-amd64.log`，便于继续运行 `scripts/collect_production_evidence_pack.sh`。
- 已将 MySQL 5.7 artifact 导入说明补充到 `docs/evidence/production/README.md` 和 `README.md`；`scripts/smoke_production_evidence_gate.sh` 已新增导入脚本正反例校验，覆盖有效日志导入、artifact zip 导入和 arm64 skip 拒绝。
- 已执行 `bash -n scripts/import_mysql57_amd64_evidence.sh scripts/smoke_production_evidence_gate.sh scripts/collect_production_evidence_pack.sh scripts/production_evidence_check.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因新增验收脚本导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 02:28:50 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `cfaecf8ab312608ce3e597a5157c3c47c8479aa286585aa52b1b20b65ad230e0`，纳入文件数 `591`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 02:29:43 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 02:30:20 CST`；当前 6 类真实生产证据仍全部缺失。MySQL 5.7 证据的下一步必须从 amd64 CI artifact 导入，而不是使用本机 arm64/qemu 日志。

## 阶段补充（2026-07-08 03:01）

- 本轮继续遵守用户最新要求，没有启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 新增 `scripts/capture_prod_frontend_evidence.sh`，用于在真实生产 `MOCHAT_PROD_FRONTEND_BASE_URL` 下用 Playwright 访问配置的 dashboard、sidebar 和 operation 路径，采集页面截图、console error、同源请求失败和非预期 4xx/5xx，并生成符合 `MOCHAT_EVIDENCE_PROD_FRONTEND` 规则的 `docs/evidence/production/prod-frontend.md`。生产页面需要登录时可传入 `MOCHAT_PROD_FRONTEND_AUTH_STATE`。
- 已将生产前端证据采集命令补充到 `docs/evidence/production/README.md` 和 `README.md`；该脚本只做浏览器证据采集，不启动 24 小时持续运行。
- 已用临时本地 HTTP fixture 执行 `scripts/capture_prod_frontend_evidence.sh`，确认会生成 `结论：通过` 的 `prod-frontend.md`、截图目录和完整结构化记录；随后把该生成文件与其他有效样例证据一起运行 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1 MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 ./scripts/production_evidence_check.sh`，严格生产证据检查接受该生产前端证据格式。
- 已执行 `bash -n scripts/capture_prod_frontend_evidence.sh scripts/production_evidence_check.sh scripts/collect_production_evidence_pack.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因新增前端生产证据采集脚本导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 03:00:17 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `da16e05ae326ed44e983c8f8159ceb939f2274dca05767c132fb8bdce0799323`，纳入文件数 `592`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 03:01:04 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 03:01:45 CST`；当前 6 类真实生产证据仍全部缺失。

## 阶段补充（2026-07-08 03:32）

- 本轮按用户最新要求继续停止 24 小时持续运行；本地短证据包和生产候选报告均显示 `24 小时持续运行：未启动`，复核未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 新增 `scripts/capture_real_saas_tenants_evidence.sh`，用于在真实生产环境用两个租户的 dashboard token 采集 SaaS 多租户数据回归证据；脚本会校验两租户读路径、租户/企业标识、跨租户禁止访问路径，并要求补充额度拦截、上传账本、异步任务和告警证据，全部满足后生成 `docs/evidence/production/real-saas-tenants.md`。该脚本只采集生产证据，不启动 24 小时持续运行。
- 已将真实 SaaS 多租户数据回归采集命令补充到 `docs/evidence/production/README.md` 和 `README.md`；生产证据目录中仍要求 `MOCHAT_EVIDENCE_REAL_SAAS_TENANTS` 指向真实生产证据文件，不能用模板或本地假数据替代。
- 已执行 `bash -n scripts/capture_real_saas_tenants_evidence.sh scripts/capture_prod_frontend_evidence.sh scripts/import_mysql57_amd64_evidence.sh scripts/production_evidence_check.sh scripts/collect_production_evidence_pack.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过，确认生产证据门禁规则仍有效。
- 因新增 SaaS 生产证据采集脚本和文档导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 03:30:59 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `1339c249ee9ee73a3f7475b2d022e15048a486779d21782c28da82463ead0ad6`，纳入文件数 `593`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 03:32:06 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 03:32:06 CST`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 04:02）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告和生产证据包均显示 `24 小时持续运行：未启动`，复核未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 新增 `scripts/capture_real_wecom_evidence.sh`，用于用真实生产 dashboard token 访问企业微信相关读路径，并结合真实回调解密、企微应用消息、截图/日志/工单引用生成 `docs/evidence/production/real-wecom.md`。脚本要求授权、通讯录、客户、客户群、标签、素材请求和回调/应用消息证据全部满足后才输出 `结论：通过`。
- 新增 `scripts/capture_real_wechat_open_evidence.sh`，用于用真实生产 dashboard token 访问微信开放平台预授权和公众号资料路径，并结合 ticket、授权回跳、取消授权、消息回调、截图/日志/工单引用生成 `docs/evidence/production/real-wechat-open.md`。脚本要求所有真实联调证据满足后才输出 `结论：通过`。
- 已将真实企业微信和真实微信开放平台证据采集命令补充到 `docs/evidence/production/README.md` 和 `README.md`；`scripts/smoke_production_evidence_gate.sh` 已新增生产证据采集脚本 help 自测，确认这些脚本不会启动 24 小时持续运行。
- 已用本地 HTTP fixture 执行 `scripts/capture_real_wecom_evidence.sh` 和 `scripts/capture_real_wechat_open_evidence.sh`，生成的 `real-wecom.md` 与 `real-wechat-open.md` 和其他有效样例证据一起运行 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1 MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 ./scripts/production_evidence_check.sh` 通过；严格生产证据检查接受这两类新证据格式。
- 已执行 `bash -n scripts/capture_real_wecom_evidence.sh scripts/capture_real_wechat_open_evidence.sh scripts/smoke_production_evidence_gate.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因新增真实外部账号生产证据采集脚本和文档导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 04:01:45 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `6ae8de28822bd5ebeefdc0cb237e4e8e3155a2f0d74838e9a4d46ddabf8c9445`，纳入文件数 `595`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 04:02:47 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 04:02:47 CST`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 04:30）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告和生产证据包均显示 `24 小时持续运行：未启动`，复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程或正在运行的 `mochat-go` Docker 容器。
- 新增 `scripts/capture_stability_evidence.sh`，用于从目标部署环境已有 `soak.ndjson` 或外部监控摘要生成 `docs/evidence/production/stability.md`；脚本校验时间范围、持续运行时长、探测轮数、standalone 模式、路由覆盖、资源使用和日志/监控引用，只读取已有记录，不启动新的 24 小时持续运行。生产发版如仍要求完整长稳证据，可设置 `MOCHAT_STABILITY_MIN_DURATION_SECONDS=86400` 导入已有满 24 小时记录。
- 已将稳定性证据导入说明补充到 `docs/evidence/production/README.md` 和 `README.md`，并把 `capture_stability_evidence.sh --help` 纳入 `scripts/smoke_production_evidence_gate.sh` 自测。至此 6 类生产证据均已有采集或导入入口：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、真实 SaaS 多租户、生产前端、持续运行稳定性。
- 已用临时 `soak.ndjson` fixture 执行 `scripts/capture_stability_evidence.sh`，生成的 `stability.md` 与其他有效样例证据一起运行 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1 MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 ./scripts/production_evidence_check.sh` 通过；严格生产证据检查接受该稳定性证据格式。
- 已执行 `bash -n scripts/capture_stability_evidence.sh scripts/smoke_production_evidence_gate.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过，确认生产证据门禁规则仍有效。
- 因新增稳定性生产证据导入脚本和文档导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 04:28:44 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `2242bdff1fc054641fa4815fcfc88b7f3a1f970b3680badba621d47bb45fc4f2`，纳入文件数 `596`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 04:29:45 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 04:29:45 CST`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 05:00）

- 本轮继续按用户最新要求不启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程或正在运行的 `mochat-go` Docker 容器。
- 已补强生产证据脱敏门禁：`scripts/production_evidence_check.sh` 现在会拒绝疑似原始 `Authorization`、`Bearer`、`access_token`、`corpsecret`、`encodingaeskey`、`password` 等敏感值，要求真实证据只保留脱敏摘要、截图/日志/工单引用或 `<redacted>` 占位。
- 已补强生产证据采集脚本的输出脱敏：`scripts/capture_real_wecom_evidence.sh`、`scripts/capture_real_wechat_open_evidence.sh`、`scripts/capture_real_saas_tenants_evidence.sh` 和 `scripts/capture_prod_frontend_evidence.sh` 会在写入 Markdown 摘要前脱敏响应摘要、证据片段、最终 URL 和错误文本中的常见 token/secret/password 形态。
- 已把敏感值反例纳入 `scripts/smoke_production_evidence_gate.sh`：包含原始 `Authorization: Bearer ...` 的生产证据必须被严格门禁拒绝；模板证据、MySQL 5.7 arm64 skip、只有关键词但缺少结构化记录的证据仍会被拒绝。
- 已将脱敏要求补充到 `docs/evidence/production/README.md` 和 `README.md`，避免真实联调证据收集时把生产 token、secret 或密码落盘。
- 已执行 `bash -n scripts/production_evidence_check.sh scripts/smoke_production_evidence_gate.sh scripts/capture_real_wecom_evidence.sh scripts/capture_real_wechat_open_evidence.sh scripts/capture_real_saas_tenants_evidence.sh scripts/capture_prod_frontend_evidence.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因修改验收脚本导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 04:57:20 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `93f4ddcb6280e020947865f818ee5e8946e8acdd4ec27966210673d89e2cb913`，纳入文件数 `596`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 04:58:23 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 04:58:23 CST`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 05:35）

- 本轮继续按用户最新要求不启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程、生产证据脱敏 fixture 进程或正在运行的 `mochat-go` Docker 容器。
- 继续补强生产证据脱敏规则：`scripts/production_evidence_check.sh` 现在能识别 JSON / query 形态的 `access_token`、`component_access_token`、`authorizer_access_token`、`authorizer_refresh_token`、`pre_auth_code`、`verify_ticket`、`corpsecret`、`encodingaeskey`、`password` 等敏感值，避免标准 JSON 响应摘要绕过脱敏检查。
- 同步补强 `scripts/capture_real_wecom_evidence.sh`、`scripts/capture_real_wechat_open_evidence.sh`、`scripts/capture_real_saas_tenants_evidence.sh`、`scripts/capture_prod_frontend_evidence.sh` 和 `scripts/capture_stability_evidence.sh`：写入 Markdown 前会脱敏 JSON 字段、URL query token、Authorization/Bearer token 和常见 secret/password 字段。
- 已将 `scripts/smoke_production_evidence_gate.sh` 扩展为带本地 HTTP fixture 的脱敏自测：fixture 返回包含原始 `access_token`、`component_access_token`、`authorizer_refresh_token`、`corpsecret` 和 query `token` 的 JSON 响应；`capture_real_wecom_evidence.sh` 生成的证据必须包含 `<redacted>` 且不得包含原始敏感值。自测脚本同时修复了 fixture PID 清理问题，避免后台服务残留。
- 已将 JSON / query token 脱敏要求补充到 `docs/evidence/production/README.md` 和 `README.md`；生产证据采集脚本会自动脱敏常见响应摘要和 URL 参数，但导入真实证据前仍需人工复核。
- 已执行 `bash -n scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh scripts/capture_real_wecom_evidence.sh scripts/capture_real_wechat_open_evidence.sh scripts/capture_real_saas_tenants_evidence.sh scripts/capture_prod_frontend_evidence.sh scripts/capture_stability_evidence.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因修改验收脚本导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 05:33:40 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `ec400c3edf43164534c4ab70583e5fc3530536b147cdd435579916b91ce5175d`，纳入文件数 `596`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 05:34:49 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 05:34:49 CST`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 06:05）

- 本轮继续按用户最新要求不启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程、生产证据脱敏 fixture 进程或正在运行的 `mochat-go` Docker 容器。
- 修复生产证据包收拢的安全顺序：`scripts/collect_production_evidence_pack.sh` 现在会在复制源证据前扫描未脱敏敏感值，疑似包含原始 `Authorization`、`Bearer`、JSON / query token、secret 或 password 的源证据会被标记为 `敏感值拒绝，未复制`，不会落入 `docs/evidence/production/current/`。
- 已将该规则纳入 `scripts/smoke_production_evidence_gate.sh`：包含原始 Bearer token 的 `real-wecom.md` 既必须被严格门禁拒绝，也不能被 `collect_production_evidence_pack.sh` 复制进生产证据包。
- 已更新 `docs/evidence/production/README.md` 和 `README.md`，明确证据包收拢会在复制前拦截未脱敏源证据，避免“门禁失败但敏感证据已落盘”的问题。
- 已执行 `bash -n scripts/collect_production_evidence_pack.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因修改验收脚本导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 06:00:42 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `3dccedd72199d3ed95dd91274b19da60f9c574d62be8aedec8dd099e8dc2ac90`，纳入文件数 `596`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 06:03:27 CST` 按预期返回失败：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 06:03:27 CST`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 09:47）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告和生产证据包均显示 `24 小时持续运行：未启动`，复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程、生产证据脱敏 fixture 进程或正在运行的 `mochat-go` Docker 容器。
- 继续补强生产证据包收拢的安全边界：`scripts/collect_production_evidence_pack.sh` 在拒绝复制含未脱敏敏感值的源证据时，`env.production-evidence` 现在指向证据包内应存在但实际缺失的目标文件，不再引用敏感源文件，避免复跑生产门禁时重新读取含敏感值的原始证据。
- 已将该边界纳入 `scripts/smoke_production_evidence_gate.sh`：含原始 Bearer token 的源 `real-wecom.md` 必须被标记为 `敏感值拒绝，未复制`，不能被复制进证据包，且 `env.production-evidence` 不能泄露敏感源文件路径。
- 已执行 `bash -n scripts/collect_production_evidence_pack.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 首次重建全套短证据时 `core` 因 Docker 代理拉取 `golang:1.26-alpine` / `alpine:3.22` 元数据 TLS handshake timeout 失败；单独复跑 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=core MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1 MOCHAT_QUEUE_AUDIT_SOURCE=embedded MOCHAT_SOURCE_ROOT=docs/evidence/latest/should-not-read-php-source MOCHAT_COMPAT_MANIFEST=docs/evidence/latest/should-not-read-manifest.json ./scripts/standalone_acceptance.sh` 于 `2026-07-08 09:14:37 CST` 通过，确认是外部 Docker 网络瞬时失败，不是 Go 独立功能失败。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 09:45:26 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `6f075725528fc2e5fa3df7652f899b0c1a787722fa2c32f01db19e159bc9a7bb`，纳入文件数 `596`，目标完成度审计显示本地独立 Go 项目、不依赖原 mochat/PHP 项目、全部 manifest 功能本地验收、SaaS/worker/cron/frontend 本地闭环均通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 09:46:45 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 09:47:40 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 12:39）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包和生产证据采集诊断均显示 `24 小时持续运行：未启动`，复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程、生产证据脱敏 fixture 进程或正在运行的 `mochat-go` Docker 容器。
- 新增 `scripts/production_evidence_doctor.sh` 作为生产证据采集诊断入口：默认只检查 6 类标准证据文件、采集环境变量和严格证据门禁状态，报告写入 `docs/evidence/production/readiness.md`，不访问生产、不启动 24 小时持续运行；显式设置 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1` 后才会对环境变量已齐备的项目调用导入/采集脚本，`MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_STRICT=1` 可让缺任一证据或采集失败时返回非 0。
- 已将 doctor 接入 `scripts/smoke_production_evidence_gate.sh`：验证 help 文案、缺证据时 readiness 报告、`24 小时持续运行：未启动`、缺少关键采集变量和严格证据门禁未通过等状态；并修复 shell grep 断言中 Markdown 反引号导致的命令替换噪音。
- 已更新 `docs/evidence/production/README.md` 和 `README.md`，加入生产证据采集诊断入口、默认只诊断、显式 RUN 才采集、STRICT 严格退出、REFRESH 刷新已存在证据的使用说明。
- 已执行 `bash -n scripts/production_evidence_doctor.sh scripts/smoke_production_evidence_gate.sh scripts/collect_production_evidence_pack.sh scripts/production_evidence_check.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/production_evidence_doctor.sh` 返回 `rc=0` 并生成 `docs/evidence/production/readiness.md`；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_STRICT=1 ./scripts/production_evidence_doctor.sh` 按预期返回 `rc=1`；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因新增生产证据诊断脚本导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 10:21:46 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `2a08e512335638bdbca1116a561cbaae3ddf4f5b861255c3860308e72af498a0`，纳入文件数 `597`，目标完成度审计显示本地独立 Go 项目、不依赖原 mochat/PHP 项目、全部 manifest 功能本地验收、SaaS/worker/cron/frontend 本地闭环均通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 12:38:27 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 12:39:34 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 13:13）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包和生产证据采集诊断 JSON 均显示未启动 24 小时持续运行。
- 补强 `scripts/production_evidence_doctor.sh` 的机器可读交接能力：默认在 `docs/evidence/production/readiness.md` 之外同步输出 `docs/evidence/production/readiness.json`，包含 schema、生成时间、6 类证据标准文件状态、缺失采集变量、采集命令、严格证据门禁报告/日志、缺失数量和是否启动 24 小时运行；JSON 只记录变量名和文件状态，不记录 token、secret 或 Bearer 值。
- 已将 JSON 输出写入 `docs/evidence/production/README.md` 和 `README.md`，并把 JSON 解析纳入 `scripts/smoke_production_evidence_gate.sh`：断言 schema、6 类证据项、`started_24h_run=false`、严格门禁失败、缺少真实企微采集变量，并检查序列化 JSON 不包含 `dashboard-jwt` 或 `Bearer `。
- 已执行 `bash -n scripts/production_evidence_doctor.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh scripts/collect_production_evidence_pack.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/production_evidence_doctor.sh` 返回 `rc=0` 并生成 `readiness.md` / `readiness.json`；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因新增 JSON 交接能力导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 13:11:51 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `83ad32286ce5eed582db797c08a6e919700b3a18d04b6acea866b5353a953404`，纳入文件数 `597`。
- 最新 `docs/evidence/production/readiness.json` 生成时间为 `2026-07-08 13:12:15 CST`，当前 `missing_count=6`、`not_ready_count=6`、`gate_ok=false`、`started_24h_run=false`，明确列出 MySQL 5.7 amd64 artifact、真实企业微信、真实微信开放平台、真实 SaaS 多租户、生产前端和稳定性证据的缺失采集变量。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 13:12:50 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 13:13:37 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失。

## 阶段补充（2026-07-08 16:04）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包和生产证据采集诊断均显示 `24 小时持续运行：未启动`，复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程、生产证据脱敏 fixture 进程或正在运行的 `mochat-go` Docker 容器。
- 补强 `scripts/production_evidence_doctor.sh` 的生产交接输出：默认在 `readiness.md` 和 `readiness.json` 之外生成 `docs/evidence/production/readiness.env.todo`，按 6 类证据分组列出待填写环境变量、标准文件、采集命令和下一步；该文件只写变量名和空值，不写 token、secret、Bearer 或示例密钥。
- 已将 `readiness.env.todo` 接入 `scripts/smoke_production_evidence_gate.sh`：断言 env 清单存在、包含真实企微关键变量和后续采集/收拢命令，并拒绝出现 `dashboard-jwt` 或 `Bearer ` 这类敏感示例；同时将 env 清单路径写入 `readiness.json` 的 `env_template` 字段。
- 已更新 `docs/evidence/production/README.md` 和 `README.md`，明确 doctor 默认只诊断，env 清单可在受控终端 `set -a; . docs/evidence/production/readiness.env.todo; set +a` 后用于显式 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1` 采集，再执行 `./scripts/collect_production_evidence_pack.sh` 收拢证据。
- 已执行 `bash -n scripts/production_evidence_doctor.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh scripts/collect_production_evidence_pack.sh` 通过；已执行默认 doctor 返回 `rc=0` 并生成 `readiness.md` / `readiness.env.todo` / `readiness.json`；已执行 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_STRICT=1 ./scripts/production_evidence_doctor.sh` 按预期返回 `rc=1`；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因新增 env 待办清单和文档导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 13:48:30 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `acb0d6db4c2d572b5fb1db27c4cd837771fc739e4ddf77e68f2e9e79f1be761b`，纳入文件数 `597`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 最新 `docs/evidence/production/readiness.json` 生成时间为 `2026-07-08 16:04:32 CST`，当前 `missing_count=6`、`not_ready_count=6`、`gate_ok=false`、`started_24h_run=false`、`env_template=docs/evidence/production/readiness.env.todo`，仍明确列出 MySQL 5.7 amd64 artifact、真实企业微信、真实微信开放平台、真实 SaaS 多租户、生产前端和稳定性证据的缺失采集变量。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 13:49:45 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 16:04:09 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失。

## 阶段补充（2026-07-08 17:17）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包、doctor 和 preflight 报告均显示 `24 小时持续运行：未启动`，复核未发现 `standalone_soak_24h.sh` 进程、`/mochat-go` 进程、生产证据脱敏 fixture 进程或正在运行的 `mochat-go` Docker 容器。
- 新增 `scripts/production_evidence_env_preflight.sh`，用于在正式采集前读取 `docs/evidence/production/readiness.json` 和本地 env 文件，检查 6 类生产证据采集变量是否齐备、`@文件` 引用是否存在、URL 是否为 http(s)；脚本只做本地预检，不访问生产、不打印 token/secret/Bearer 值、不启动 24 小时持续运行。默认报告写入 `docs/evidence/production/preflight.md`，机器可读状态写入 `docs/evidence/production/preflight.json`，可用 `MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1` 让未就绪时返回非 0。
- 新增 `.gitignore` 保护 `docs/evidence/production/readiness.env.local`、`*.env.local` 和生产前端登录态文件；同时更新 doctor 输出，让 `readiness.env.todo` 明确只作为空值模板，要求先复制为 `readiness.env.local` 后填写，并先执行 `MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE=docs/evidence/production/readiness.env.local ./scripts/production_evidence_env_preflight.sh` 预检。
- 已将 preflight 接入 `scripts/smoke_production_evidence_gate.sh`：验证 help 文案、缺少本地 env 文件时严格预检必须失败、填齐 fixture env 时严格预检通过、报告和 JSON 不泄露 `dashboard-jwt`、租户 token 或 `Bearer `；同时继续覆盖模板证据拒绝、arm64 MySQL skip 拒绝、敏感值拒绝和有效证据通过。
- 已更新 `docs/evidence/production/README.md` 和 `README.md`，将生产采集流程改为：运行 doctor 生成 `readiness.env.todo`，复制为已忽略的 `readiness.env.local`，填写后先跑 preflight，再显式 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1` 采集，最后 `collect_production_evidence_pack.sh` 收拢。
- 已执行 `bash -n scripts/production_evidence_env_preflight.sh scripts/production_evidence_doctor.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh scripts/collect_production_evidence_pack.sh` 通过；已执行默认 doctor 和默认 preflight 返回 `rc=0` 并生成报告；已执行 `MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 ./scripts/production_evidence_env_preflight.sh` 按预期返回 `rc=1`；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 首次重建全套短证据时 `core` 因 Docker 代理拉取 `golang:1.26-alpine` / `alpine:3.22` 元数据 EOF 失败；单独复跑 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_ACCEPTANCE_SUITE=core MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1 MOCHAT_QUEUE_AUDIT_SOURCE=embedded MOCHAT_SOURCE_ROOT=docs/evidence/latest/should-not-read-php-source MOCHAT_COMPAT_MANIFEST=docs/evidence/latest/should-not-read-manifest.json ./scripts/standalone_acceptance.sh` 于 `2026-07-08 16:47:37 CST` 通过，确认是外部 Docker 网络瞬时失败，不是 Go 独立功能失败。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 17:14:10 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `8a29586c439c0ecb96a365416cbacb2660e0cf983fa2448b898a48b40e2f17c7`，纳入文件数 `598`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 最新 `docs/evidence/production/readiness.json` 生成时间为 `2026-07-08 17:14:57 CST`，当前 `missing_count=6`、`not_ready_count=6`、`gate_ok=false`、`started_24h_run=false`；最新 `docs/evidence/production/preflight.json` 生成时间同为 `2026-07-08 17:14:57 CST`，当前 `ready_count=0`、`not_ready_count=6`、`all_ready=false`、`started_24h_run=false`，因为 `readiness.env.local` 尚未填写。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 17:16:09 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 17:16:52 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失。

## 阶段补充（2026-07-08 18:00）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包、doctor 和 preflight 报告均显示 `24 小时持续运行：未启动`，复核未发现有效的 `standalone_soak_24h.sh` 进程、`/mochat-go` 业务进程、生产证据脱敏 fixture 进程或正在运行的 `mochat-go` Docker 容器。
- 继续补强 `scripts/production_evidence_env_preflight.sh`：预检现在会读取 `MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE` 指向的 `mysql57-amd64.log` 或 artifact zip，要求包含 `mysql57 amd64 CI gate passed` 或 `mysql 5.7 schema migration smoke passed`，并拒绝 arm64 skip 日志、缺少 `mysql57-amd64.log` 的 zip、无效 zip、空文件和缺少通过标记的日志。
- 修复 preflight 状态优先级：如果本地 env 显式提供了无效 MySQL 5.7 amd64 证据来源，即使标准证据文件已经存在，也不能被覆盖成“标准文件已存在”而通过严格预检。
- 已将上述规则纳入 `scripts/smoke_production_evidence_gate.sh`：有效 artifact zip 会通过严格预检，arm64 skip 日志和缺少通过标记的日志会失败；同时继续断言 preflight 报告和 JSON 不泄露 `dashboard-jwt`、租户 token 或 `Bearer `。
- 已更新 `docs/evidence/production/README.md` 和 `README.md`，明确生产采集前的 preflight 不仅检查变量、`@文件` 和 URL，也会校验 MySQL 5.7 amd64 日志或 artifact zip 内容，避免把本机 arm64/qemu 跳过日志误当成生产证据。
- 已执行 `bash -n scripts/production_evidence_env_preflight.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_doctor.sh scripts/production_evidence_check.sh scripts/collect_production_evidence_pack.sh` 通过；已执行默认 doctor 和默认 preflight 返回 `rc=0` 并生成报告；已执行 `MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 ./scripts/production_evidence_env_preflight.sh` 按预期返回 `rc=1`；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因修改验收脚本和文档导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 17:55:01 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `790e7303dcccc8e715cca94fa8ccd3b5c34b015d462ea4aaf90fb96ce6cb8b72`，纳入文件数 `598`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 最新 `docs/evidence/production/readiness.json` 生成时间为 `2026-07-08 17:59:35 CST`，当前 `missing_count=6`、`not_ready_count=6`、`gate_ok=false`、`started_24h_run=false`；最新 `docs/evidence/production/preflight.json` 生成时间为 `2026-07-08 17:59:36 CST`，当前 `ready_count=0`、`not_ready_count=6`、`all_ready=false`、`started_24h_run=false`、`accessed_production=false`，因为 `readiness.env.local` 尚未填写。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 17:58:15 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 17:59:23 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 18:47）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包、doctor 和 preflight 报告均显示 `24 小时持续运行：未启动`，且未发现 `standalone_soak_24h.sh` 进程或正在运行的 `mochat-go` Docker 容器。
- 新增 `scripts/audit_frontend_dist_api_coverage.sh`，直接扫描 `web/dashboard/dist`、`web/sidebar/dist`、`web/operation/dist` 中旧前端真实声明的静态 API，并与 `internal/server/server.go` 的 Go runtime dispatch/routes 对比；该审计已接入 `scripts/standalone_stage_report.sh`、`scripts/collect_standalone_evidence.sh` 和 `scripts/audit_goal_completion.sh`，严格模式发现缺失 API 会返回非 0。
- 最新 `docs/evidence/latest/frontend-dist-api-coverage.md` 生成时间为 `2026-07-08 18:44:35 CST`：旧前端 dist 共扫描 API 端点 `209` 个，缺失 `18` 个；其中 dashboard `168` 个缺 `1` 个、sidebar `19` 个缺 `0` 个、operation `22` 个缺 `17` 个。
- 当前缺失 API 为：`GET /dashboard/external/tenantIndex`；operation H5 的 `POST /operation/lottery/contactData`、`PUT /operation/lottery/contactLottery`、`PUT /operation/lottery/receive`、`GET /operation/openUserInfo/lottery`、`GET /operation/openUserInfo/roomClockIn`、`GET /operation/openUserInfo/roomFission`、`GET /operation/openUserInfo/shopCode`、`GET /operation/roomClockIn/clockInRanking`、`PUT /operation/roomClockIn/contactClockIn`、`GET /operation/roomClockIn/contactData`、`PUT /operation/roomClockIn/receive`、`GET /operation/roomFission/inviteFriends`、`GET /operation/roomFission/poster`、`GET /operation/roomFission/receive`、`GET /operation/roomInfinitePull/qrCode`、`GET /operation/shopCode/areaCode`、`GET /operation/shopCode/weChatSdkConfig`。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57' ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 18:45:23 CST`，结论修正为 `本地短门禁通过，目标未完成，仍需补前端 API、生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `183`；最新源码与验收指纹为 `e6371230a0a1ce471652b2c12e91281d23188c342f4903dbccee435c31fa02f1`，纳入文件数 `599`，目标完成度审计显示本地独立 Go 项目、不依赖原 mochat/PHP 项目、全部 manifest 功能本地验收均通过，但 `旧前端 dist API 全量承接` 和 `生产外部证据` 未完成。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 18:46:53 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，严格目标完成度审计结论为 `目标未完成，继续保留前端 API、生产证据缺口`，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 18:46:53 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失。
- 最新 `docs/evidence/production/readiness.json` 和 `docs/evidence/production/preflight.json` 生成时间均为 `2026-07-08 18:47:17 CST`；doctor 当前 `missing_count=6`、`not_ready_count=6`、`started_24h_run=false`，preflight 当前 `ready_count=0`、`not_ready_count=6`、`all_ready=false`、`started_24h_run=false`。
- 下一步编码优先级调整为先补旧前端 dist API：优先补 operation H5 的 lottery、roomClockIn、roomFission、roomInfinitePull、shopCode/openUserInfo/JSSDK 相关接口，再补 dashboard `external/tenantIndex`，补齐后以 `MOCHAT_FRONTEND_DIST_API_STRICT=1 ./scripts/audit_frontend_dist_api_coverage.sh` 作为硬门禁复核。

## 阶段补充（2026-07-08 19:40）

- 本轮按用户最新要求未启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程，也未发现正在运行的 `mochat-go` Docker 容器。
- 已补齐旧前端 dist 反查出的 18 个兼容 API：dashboard `GET /dashboard/external/tenantIndex`，operation H5 的 lottery、roomClockIn、roomFission、roomInfinitePull、shopCode/openUserInfo/JSSDK 相关接口。
- 已执行 `env -u GOROOT go test -count=1 ./internal/dashboard ./internal/store ./internal/server ./internal/config` 通过。
- 已执行 `MOCHAT_FRONTEND_DIST_API_STRICT=1 ./scripts/audit_frontend_dist_api_coverage.sh` 通过：旧前端 dist 共 `209` 个唯一 API，缺失 `0`；dashboard `168/0`、sidebar `19/0`、operation `22/0`。
- 已修正 `scripts/audit_functional_module_matrix.sh` 的模块归属，新增 operation H5 前缀归入抽奖、群打卡、群裂变、无限拉群、门店活码，`/dashboard/external` 归入租户/账号模块；复跑通过，结果为 `modules=28`、manifest `213/213`、runtime `406/406`。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；本地短门禁、独立交付包、inventory parity、core/saas/workers/cron/frontend/mysql57、路由覆盖、前端 dist API、阶段报告、生产证据报告和目标完成度报告均已刷新。
- 最新 `docs/evidence/latest/goal-completion.md` 生成时间为 `2026-07-08 19:40:39 CST`：本地独立 Go 项目、不依赖原 mochat/PHP 项目、全部 manifest 功能本地验收、旧前端 dist API 全量承接、SaaS/worker/cron/frontend 本地闭环均通过；目标仍未完成的原因仅剩 6 类生产外部证据缺口。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `82f6f3c8d080e3657d3b9b43658fe2e33d0fccc044af337dbc510d8c1f56b8fa`，纳入文件数 `602`，目标完成度审计显示源码指纹匹配。

## 阶段补充（2026-07-08 20:12）

- 本轮继续按用户最新要求不启动 24 小时持续运行；新增 `scripts/list_standalone_soak_processes.sh`，用于识别真实执行 `standalone_soak_24h.sh` 的进程，并避免把 `git add ... standalone_soak_24h.sh`、`rg`、`grep`、`python` 等命令误判为长稳运行。
- 已将新的 24 小时进程识别脚本接入 `scripts/collect_standalone_evidence.sh`、`scripts/production_candidate_gate.sh` 和 `scripts/audit_goal_completion.sh`；已执行 `bash -n scripts/list_standalone_soak_processes.sh scripts/collect_standalone_evidence.sh scripts/production_candidate_gate.sh scripts/audit_goal_completion.sh` 通过，当前未发现 `standalone_soak_24h.sh` 进程。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 20:11:10 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新目标完成度审计显示：本地独立 Go 项目、不依赖原 mochat/PHP 项目、全部 manifest 功能本地验收、旧前端 dist API 全量承接、SaaS/worker/cron/frontend 本地闭环均通过；未完成项仅剩 `生产外部证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `9236c26083e8cb71e3af08b4b6f0569ae9cb9d11c586a29afb451f755e40d75f`，纳入文件数 `603`，目标完成度审计显示源码指纹匹配。
- 已重新执行生产证据 doctor、preflight、生产候选门禁和生产证据包收拢；`docs/evidence/production/candidate/index.md` 和 `docs/evidence/production/current/index.md` 生成时间均为 `2026-07-08 20:12:11 CST`，按预期返回失败：6 类真实生产证据仍缺失，但报告已确认 `24 小时持续运行：未启动`，且未发现正在运行的 `mochat-go` Docker 容器。

## 阶段补充（2026-07-08 20:45）

- 本轮继续按用户最新要求不启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程，也未发现正在运行的 `mochat-go` Docker 容器。
- 补强 `scripts/production_evidence_env_preflight.sh` 和 `scripts/capture_stability_evidence.sh`：当稳定性证据走外部监控摘要而不是 soak.ndjson 时，预检和采集脚本现在要求摘要包含路由覆盖和 `224`、健康检查标记（如 `/readyz`、health、健康、探测或 `200`）、资源使用标记（如 RSS、memory、CPU、连接、内存或资源）以及明确起止日期；“monitor ok” 这类弱摘要会被拒绝。
- 修复生产证据 preflight 的优先级：即使标准证据文件已存在，只要本地 env 显式提供了无效采集变量，也不能被覆盖成“标准文件已存在”而通过；这避免后续刷新生产证据时把错误 env 混进采集流程。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增弱稳定性摘要失败和强结构化摘要通过的用例；已执行 `bash -n scripts/production_evidence_env_preflight.sh scripts/capture_stability_evidence.sh scripts/smoke_production_evidence_gate.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 已更新 `README.md` 和 `docs/evidence/production/README.md`，明确无 24 小时 run 的稳定性外部监控证据口径：不能只写结论短句，必须有路由、健康、资源和时间范围。
- 已执行默认 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`，报告分别刷新到 `docs/evidence/production/readiness.md`、`readiness.env.todo`、`readiness.json`、`preflight.md` 和 `preflight.json`；当前 `readiness.env.local` 仍未填写，采集环境就绪项仍为 `0/6`，且报告均显示未访问生产、未启动 24 小时运行。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 20:44:07 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新本地目标完成度审计显示：本地独立 Go 项目、不依赖原 mochat/PHP 项目、全部 manifest 功能本地验收、旧前端 dist API 全量承接、SaaS/worker/cron/frontend 本地闭环均通过；`production evidence check` 仍失败。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `70cb8e062d9a7d8990795fabb375d20f0bfbd54b17c386e4611a6244f8412840`，纳入文件数 `603`，目标完成度审计显示源码指纹匹配。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 20:45:07 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 20:45:07 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失。

## 阶段补充（2026-07-08 21:14）

- 本轮继续按用户最新要求不启动 24 小时持续运行；复核未发现 `standalone_soak_24h.sh` 进程，也未发现正在运行的 `mochat-go` Docker 容器。
- 继续补强 `scripts/production_evidence_env_preflight.sh`：生产取证 URL 现在默认拒绝 `example.com`、`example.org`、`example.net`、`localhost`、`127.0.0.1`、`0.0.0.0`、`::1`、`.test`、`.invalid` 等示例域名或本机地址，避免把占位地址误判为真实生产采集环境；仅本地 fixture/smoke 可显式设置 `MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_ALLOW_NON_PROD=1` 放行。
- 已更新 `scripts/smoke_production_evidence_gate.sh`：新增占位 URL 失败用例，并将通过用例改为生产样式域名；继续覆盖 MySQL 5.7 arm64 skip、缺 marker、弱稳定性摘要、敏感值泄露、模板证据和有效证据包等规则。
- 已同步更新 `README.md`、`docs/evidence/production/README.md` 以及 `capture_real_wecom_evidence.sh`、`capture_real_wechat_open_evidence.sh`、`capture_real_saas_tenants_evidence.sh`、`capture_prod_frontend_evidence.sh` 的 help 示例，避免继续引导填写 `https://example.com`。
- 已执行 `bash -n scripts/production_evidence_env_preflight.sh scripts/smoke_production_evidence_gate.sh scripts/capture_real_wecom_evidence.sh scripts/capture_real_wechat_open_evidence.sh scripts/capture_real_saas_tenants_evidence.sh scripts/capture_prod_frontend_evidence.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 已执行默认 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh` 刷新生产取证交接报告；最新 `docs/evidence/production/preflight.md` 显示 `允许非生产地址：False`、未访问生产、未启动 24 小时运行，且 `readiness.env.local` 尚未填写，采集环境就绪项仍为 `0/6`。
- 已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 21:13:04 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖仍为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `969eeaa9653149230ea984dc5a922d3fdb61ba26628166a4048f15b6f30cfb6e`，纳入文件数 `603`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 21:14:03 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失。

## 阶段补充（2026-07-08 22:08）

- 本轮按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包和 preflight 报告均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 新增 `scripts/production_url_guard.sh`，并接入 `capture_real_wecom_evidence.sh`、`capture_real_wechat_open_evidence.sh`、`capture_real_saas_tenants_evidence.sh` 和 `capture_prod_frontend_evidence.sh`；真实生产采集脚本现在默认拒绝 `example.com`、`localhost`、`127.0.0.1`、`.test`、`.invalid` 等占位或本机地址，仅本地 fixture/smoke 可显式设置 `MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS=1` 放行。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增四个真实采集脚本的占位 URL 拒绝用例，并保留本地 fixture 的显式放行路径；已执行 `bash -n scripts/production_url_guard.sh scripts/capture_real_wecom_evidence.sh scripts/capture_real_wechat_open_evidence.sh scripts/capture_real_saas_tenants_evidence.sh scripts/capture_prod_frontend_evidence.sh scripts/smoke_production_evidence_gate.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 首次全量刷新 `MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 时，`core` 因 Docker 代理拉取 `alpine:3.22` / `golang:1.26-alpine` 元数据 EOF 失败；随后单独复跑 `core` 于 `2026-07-08 21:43:53 CST` 通过，确认是外部 Docker 网络瞬时失败，不是 Go 独立功能失败。
- 已重新执行全量本地短证据刷新并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 22:06:19 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `db0d789edd87017527a80a591e80b85b7a69f7ba15190439a58c3b4709450619`，纳入文件数 `604`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 22:08:09 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 22:08:09 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 22:37）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包和 preflight 报告均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 补强最终生产证据门禁 `scripts/production_evidence_check.sh`：除继续拒绝模板、待验收、arm64 MySQL skip、缺结构化记录项和未脱敏敏感值外，现在会扫描证据正文中的 URL，并拒绝 `example.com`、`localhost`、`127.0.0.1`、`0.0.0.0`、`::1`、`.test`、`.invalid` 等示例域名或本机地址，防止手工证据绕过 capture/preflight 的生产 URL 校验。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增“结构完整但生产站点为 `127.0.0.1` 的证据必须失败”用例；已执行 `bash -n scripts/production_evidence_check.sh scripts/smoke_production_evidence_gate.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 已更新 `README.md` 和 `docs/evidence/production/README.md`，明确生产证据正文不能包含示例域名或本机地址；本地 fixture 只允许用于 smoke，不得作为最终生产证据。
- 因验收脚本变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 22:36:27 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `5ac504b3d4b196217039c4b15265718eab02404a250caffeec034576a5d2e0d7`，纳入文件数 `604`。
- 已刷新 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`；`docs/evidence/production/preflight.md` 生成时间为 `2026-07-08 22:36:39 CST`，当前 `readiness.env.local` 仍不存在，采集环境就绪项 `0/6`，未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 22:37:26 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 22:37:26 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 23:05）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包和 preflight 报告均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 继续补强生产证据收拢脚本 `scripts/collect_production_evidence_pack.sh`：复制源证据进入 `docs/evidence/production/current/` 前，现在不仅扫描未脱敏 token/secret/password，也会扫描证据正文中的非生产 URL；若源证据包含 `example.com`、`localhost`、`127.0.0.1`、`0.0.0.0`、`::1`、`.test`、`.invalid` 等示例域名或本机地址，会在 `index.md` 标记为 `非生产地址拒绝，未复制`，并让严格门禁失败，避免问题证据落入 current 包。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增“结构完整但指向 `127.0.0.1` 的源证据不能被复制进生产证据包”的用例，并校验 `env.production-evidence` 不泄露被拒绝源文件路径；已执行 `bash -n scripts/collect_production_evidence_pack.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 已更新 `README.md` 和 `docs/evidence/production/README.md`，明确证据包收拢会在复制前拒绝敏感值和非生产 URL 两类问题源证据。
- 因验收脚本变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 23:04:07 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `61ab1d656528fbc41826430cd784dd5f42b84226c5c2c6fe5a7b1a6663239241`，纳入文件数 `604`。
- 已刷新 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`；`docs/evidence/production/preflight.md` 生成时间为 `2026-07-08 23:04:27 CST`，当前 `readiness.env.local` 仍不存在，采集环境就绪项 `0/6`，未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 23:05:14 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 23:05:14 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-08 23:34）

- 本轮继续按用户最新要求不启动 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包、doctor 和 preflight 报告均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 补强 `scripts/production_evidence_doctor.sh` 与 `scripts/production_evidence_env_preflight.sh`：doctor 现在不只记录标准证据文件是否存在，还会读取严格生产证据门禁报告，给每个证据项输出 `evidence_valid`、`evidence_validation_status` 和 `evidence_issue`；模板、待验收、未脱敏、非生产 URL、缺结构化记录项等文件即使存在，也会在 readiness JSON 中标为无效。
- preflight 现在只有在标准证据文件通过严格门禁时才把它视为“标准文件已存在”；标准文件存在但无效时会显示 `标准文件无效`，除非本地 env 已经齐备、可执行 doctor RUN 刷新该证据。这样避免把占位模板或坏证据误判为生产采集已就绪。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增 doctor JSON 的 `standard_files_valid=false`、每个模板证据 `evidence_valid=false`、以及 preflight 对模板标准文件标记 `标准文件无效` 的断言；已执行 `bash -n scripts/production_evidence_doctor.sh scripts/production_evidence_env_preflight.sh scripts/smoke_production_evidence_gate.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 已更新 `README.md` 和 `docs/evidence/production/README.md`，明确 doctor/readiness 会区分“文件存在”和“文件有效”，preflight 只把有效标准文件当作就绪。
- 因验收脚本变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-08 23:33:03 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `2f8cfdd2fff86012eb68305e308f08a726b65682e4b1be54be0ba063e68e3a62`，纳入文件数 `604`。
- 已刷新 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`；`docs/evidence/production/readiness.md` 生成时间为 `2026-07-08 23:33:16 CST`，当前 `standard_files_valid=False`、`missing_count=6`、`not_ready_count=6`；`preflight.md` 同时显示 `readiness.env.local` 不存在，采集环境就绪项 `0/6`，未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-08 23:34:03 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-08 23:34:03 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 00:12）

- 本轮按用户最新要求未启动新的 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包、doctor 和 preflight 报告均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 补强 `scripts/production_evidence_check.sh`：6 类外部生产证据现在都必须包含当前 `scripts/source_fingerprint.py` 生成的 `源码指纹`，缺失或与当前源码/验收脚本指纹不一致都会被判定为过期证据；该校验在业务结构化记录之后执行，避免缺结构证据被误报成单纯缺指纹。
- 已更新生产证据采集与导入链路：`capture_real_wecom_evidence.sh`、`capture_real_wechat_open_evidence.sh`、`capture_real_saas_tenants_evidence.sh`、`capture_prod_frontend_evidence.sh` 和 `capture_stability_evidence.sh` 会自动写入当前源码指纹；`ci_mysql57_amd64.sh` 会在 amd64 CI 成功日志中输出源码指纹，`import_mysql57_amd64_evidence.sh` 会拒绝缺少当前源码指纹的 MySQL 5.7 artifact/log。
- 已更新 `scripts/init_production_evidence_pack.sh` 模板、`README.md`、`docs/evidence/production/README.md` 和 `scripts/smoke_production_evidence_gate.sh`；smoke 覆盖缺源码指纹、旧源码指纹、MySQL 通过 marker 但缺源码指纹、敏感值、非生产 URL、模板证据和有效证据包等场景。
- 已执行 `bash -n scripts/production_evidence_check.sh scripts/capture_real_wecom_evidence.sh scripts/capture_real_wechat_open_evidence.sh scripts/capture_real_saas_tenants_evidence.sh scripts/capture_prod_frontend_evidence.sh scripts/capture_stability_evidence.sh scripts/ci_mysql57_amd64.sh scripts/import_mysql57_amd64_evidence.sh scripts/init_production_evidence_pack.sh scripts/smoke_production_evidence_gate.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因验收脚本变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 00:09:26 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `4b26d3da258714fd13b8fb2f7c6416d92fd194bab832c5579baa6cb11d4a7a51`，纳入文件数 `604`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已刷新 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`；`docs/evidence/production/readiness.md` 生成时间为 `2026-07-09 00:09:50 CST`，`standard_files_valid=False`，采集环境变量或有效文件仍未齐备；`preflight.md` 生成时间为 `2026-07-09 00:09:57 CST`，采集环境就绪项仍为 `0/6`，未访问生产、未启动 24 小时运行。
- 已执行 `MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 env -u GOROOT ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-09 00:10:37 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-09 00:11:23 CST`，按预期返回 `rc=1`；当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 00:41）

- 本轮继续按用户最新要求不启动新的 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包、doctor 和 preflight 报告均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 继续补强 `scripts/collect_production_evidence_pack.sh`：收拢生产证据进入 `docs/evidence/production/current/` 前，除敏感值和非生产 URL 外，现在会先校验证据正文是否包含当前 `scripts/source_fingerprint.py` 生成的源码指纹；缺少源码指纹或指纹过期的源证据会被标记为 `源码指纹拒绝，未复制`，不会落入 current 包。
- `collect_production_evidence_pack.sh` 生成的 `manifest.json` 和 `index.md` 现在记录当前源码指纹，方便复核 current 包与当前工作区是否一致；`README.md` 和 `docs/evidence/production/README.md` 已同步说明复制前会拒绝敏感值、非生产 URL 和过期源码指纹三类问题证据。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增“结构完整但缺源码指纹/源码指纹过期的源证据不能被复制进生产证据包”的用例；已执行 `bash -n scripts/collect_production_evidence_pack.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因验收脚本变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 00:38:22 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `ec3ef36887381f88bed864a3822f57a3bbdbb22538199f35f739ee908c6db990`，纳入文件数 `604`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已刷新 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`；`docs/evidence/production/readiness.md` 与 `preflight.md` 生成时间均为 `2026-07-09 00:38:58 CST`，采集环境就绪项仍为 `0/6`，未访问生产、未启动 24 小时运行。
- 已执行 `MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 env -u GOROOT ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-09 00:39:41 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过。
- 已执行 `env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-09 00:40:31 CST`，按预期返回 `rc=1`；current 包已记录当前源码指纹 `ec3ef36887381f88bed864a3822f57a3bbdbb22538199f35f739ee908c6db990`，但当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 01:13）

- 本轮继续按用户最新要求不启动新的 24 小时持续运行；最新本地短证据包、生产候选报告、生产证据包、doctor 和 preflight 报告均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 补强 `scripts/audit_goal_completion.sh`：新增 `MOCHAT_GOAL_COMPLETION_JSON_OUT`，可在生成 Markdown 完成度审计的同时输出机器可读 JSON；JSON 只包含目标项状态、缺口、门禁摘要、日志路径、manifest/route 概要、源码指纹、生产证据缺口、soak 摘要和运行残留，不写完整命令输出或密钥。
- 修正目标完成度结论聚合：当 `docs/evidence/latest/source-fingerprint.json` 过期或本地短证据包不完整时，报告现在会明确显示 `本地验收` 缺口；避免在代码变更后仍只显示 `生产证据` 缺口。
- 已将 `goal-completion.json` 接入 `scripts/collect_standalone_evidence.sh` 和 `scripts/production_candidate_gate.sh`，本地短证据包与生产候选报告均会保留结构化目标完成度结果；`README.md` 已同步说明该 JSON 输出。
- 已执行 `bash -n scripts/audit_goal_completion.sh scripts/collect_standalone_evidence.sh scripts/production_candidate_gate.sh` 通过；临时执行 `MOCHAT_GOAL_COMPLETION_JSON_OUT=<tmp>/goal.json ./scripts/audit_goal_completion.sh` 验证 JSON 可解析，并确认在 latest 过期时会输出 `gap_labels=['本地验收','生产证据']`。
- 因验收脚本和文档变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 01:10:42 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `aea52ab7238b1ef54ebcb4c1a741fcb881faddcc7fae2e7ed372df7304caa9bb`，纳入文件数 `604`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 最新 `docs/evidence/latest/goal-completion.json` 可解析：`goal_complete=false`、`gap_labels=['生产证据']`、`source_fingerprint.matches=true`，未完成项仍为 6 类真实生产外部证据。
- 已刷新 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`；`docs/evidence/production/preflight.md` 生成时间为 `2026-07-09 01:11:20 CST`，采集环境就绪项仍为 `0/6`，未访问生产、未启动 24 小时运行。
- 已执行 `MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 env -u GOROOT ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-09 01:12:04 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过；`docs/evidence/production/candidate/goal-completion.json` 同样可解析，`gap_labels=['生产证据']`。
- 已执行 `env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-09 01:12:54 CST`，按预期返回 `rc=1`；current 包已记录当前源码指纹 `aea52ab7238b1ef54ebcb4c1a741fcb881faddcc7fae2e7ed372df7304caa9bb`，但当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 02:09）

- 本轮继续按用户最新要求不启动新的 24 小时持续运行；`docs/evidence/latest/index.md`、`docs/evidence/production/candidate/index.md` 和 `docs/evidence/production/current/index.md` 均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 补强 `scripts/production_evidence_check.sh`：新增 `MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT`，可在 Markdown 报告之外输出 `production-evidence.json`；JSON 记录 quick/evidence 总状态、6 类证据明细、缺失环境变量、失败原因、manifest 摘要和当前源码指纹，不写完整命令输出或密钥。
- 已将 `production-evidence.json` 接入 `scripts/collect_standalone_evidence.sh`、`scripts/production_candidate_gate.sh` 和 `scripts/collect_production_evidence_pack.sh`；`docs/evidence/latest/`、`docs/evidence/production/candidate/` 和 `docs/evidence/production/current/` 现在都会保留机器可读生产证据检查结果。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增模板证据失败 JSON、有效证据通过 JSON 和 current 包 JSON 的可解析断言，并校验序列化 JSON 不包含 Bearer/raw token；已执行 `bash -n scripts/production_evidence_check.sh scripts/production_candidate_gate.sh scripts/collect_production_evidence_pack.sh scripts/collect_standalone_evidence.sh scripts/smoke_production_evidence_gate.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因验收脚本和文档变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 02:06:18 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `0545b8434297f395e98c11c9975bc86cd9f11491863376d348788d272215e858`，纳入文件数 `604`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 最新 `docs/evidence/latest/production-evidence.json` 可解析：`schema=1`、`ok=false`、`evidence_ok=false`、`invalid_evidence=6`，当前缺口仍为 6 类真实生产外部证据；`docs/evidence/latest/goal-completion.json` 可解析：`goal_complete=false`、`gap_labels=['生产证据']`、`source_fingerprint.matches=true`。
- 已刷新 `production_evidence_doctor.sh` 和 `production_evidence_env_preflight.sh`；`docs/evidence/production/preflight.md` 仍显示采集环境就绪项 `0/6`，未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-09 02:07:32 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过；`docs/evidence/production/candidate/production-evidence.json` 和 `goal-completion.json` 均可解析。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-09 02:08:19 CST`，按预期返回 `rc=1`；current 包已记录当前源码指纹 `0545b8434297f395e98c11c9975bc86cd9f11491863376d348788d272215e858`，并保留 `production-evidence.json`，但当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 02:37）

- 本轮继续按用户最新要求不启动新的 24 小时持续运行；`docs/evidence/latest/index.md`、`docs/evidence/production/readiness.md`、`docs/evidence/production/candidate/index.md` 和 `docs/evidence/production/current/index.md` 均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 继续补强 `scripts/production_evidence_doctor.sh`：doctor 现在会给内部严格生产证据门禁设置 `MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT`，生成 `docs/evidence/production/readiness-production-evidence.json`，并优先从该 JSON 读取 6 类证据的有效/无效/缺失状态；Markdown 解析只作为兜底，避免报告文案变化影响 readiness 结论。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增 doctor `gate_json` 存在性、schema、`evidence_ok=false` 和 6 类无效证据断言；已更新 `README.md` 和 `docs/evidence/production/README.md`，明确 doctor 会保留 `readiness-production-evidence.md` 与 `readiness-production-evidence.json`。
- 已执行 `bash -n scripts/production_evidence_doctor.sh scripts/production_evidence_env_preflight.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_check.sh` 通过；已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因验收脚本和文档变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 02:35:44 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `af38e0d8bd577e801788d2e947b004a0d61dd4f31bfe79ed22b0d539499e9fd7`，纳入文件数 `604`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已刷新 `production_evidence_doctor.sh` 与 `production_evidence_env_preflight.sh`；`docs/evidence/production/readiness.md` 生成时间为 `2026-07-09 02:36:05 CST`，标准证据文件齐备 `False`、标准证据文件有效 `False`、采集环境变量齐备或有效文件已存在 `False`，并明确严格门禁 JSON 为 `docs/evidence/production/readiness-production-evidence.json`；`preflight.json` 显示 `ready_count=0`、`not_ready_count=6`，未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-09 02:36:51 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过；`docs/evidence/production/candidate/production-evidence.json` 显示 `ok=false`、`evidence_ok=false`、`invalid_evidence=6`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，按预期返回 `rc=1`；current 包 `manifest.json` 与 `production-evidence.json` 已记录当前源码指纹 `af38e0d8bd577e801788d2e947b004a0d61dd4f31bfe79ed22b0d539499e9fd7`，但当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 03:05）

- 本轮继续按用户最新要求不启动新的 24 小时持续运行；`docs/evidence/latest/index.md`、`docs/evidence/production/preflight.md`、`docs/evidence/production/candidate/index.md` 和 `docs/evidence/production/current/index.md` 均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 修正 `scripts/production_evidence_env_preflight.sh` 的就绪判定：此前 strict preflight 会硬性要求 env 文件存在，即使 6 类标准生产证据文件都已有效也会失败；现在每一类证据只要“有效标准文件已存在”或“采集变量齐备”即可计为 ready，6 类标准文件都有效时可不需要 env 文件直接进入 `collect_production_evidence_pack.sh`。
- preflight 报告和 JSON 新增 `standard_file_ready_count`、`capture_env_ready_count` 和更准确的结论文案，区分“标准生产证据文件均有效，可直接进入证据包收拢”和“采集环境变量齐备，可执行 doctor RUN 采集”。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，新增“6 类有效标准证据已存在但 env 文件不存在时，`MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1` 仍必须通过”的用例，并断言 `all_ready=true`、`env_file_exists=false`、`standard_file_ready_count=6`、`capture_env_ready_count=0`，且不访问生产、不启动 24 小时运行。
- 已更新 `README.md` 和 `docs/evidence/production/README.md`，明确标准证据文件有效时不再强制要求 env 文件存在；已执行 `bash -n scripts/production_evidence_env_preflight.sh scripts/smoke_production_evidence_gate.sh scripts/production_evidence_doctor.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因验收脚本和文档变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 03:04:18 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `6ebfea4a6ffbe77fd14e6a44f2b9fe1f1b5f953b59b42b1efdb7b0b42325ec6f`，纳入文件数 `604`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已刷新 `production_evidence_doctor.sh` 与 `production_evidence_env_preflight.sh`；`docs/evidence/production/preflight.md` 生成时间为 `2026-07-09 03:04:42 CST`，当前仍为 `ready_count=0`、`standard_file_ready_count=0`、`capture_env_ready_count=0`、`not_ready_count=6`，未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过；`docs/evidence/production/candidate/production-evidence.json` 显示 `ok=false`、`evidence_ok=false`、`invalid_evidence=6`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，按预期返回 `rc=1`；current 包 `manifest.json` 与 `production-evidence.json` 已记录当前源码指纹 `6ebfea4a6ffbe77fd14e6a44f2b9fe1f1b5f953b59b42b1efdb7b0b42325ec6f`，但当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 03:33）

- 本轮继续按用户最新要求不启动新的 24 小时持续运行；`docs/evidence/latest/index.md`、`docs/evidence/production/preflight.md`、`docs/evidence/production/candidate/index.md` 和 `docs/evidence/production/current/index.md` 均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 继续修正 `scripts/production_evidence_env_preflight.sh` 的提示逻辑：在 6 类标准生产证据文件都有效且 env 文件不再需要时，不再把 `readiness.env.local` 缺失作为 warning 写入 preflight JSON/Markdown，避免最终验收时出现无关提示；证据不齐时仍保留 env 文件缺失提示。
- 已更新 `scripts/smoke_production_evidence_gate.sh`，在“有效标准证据已存在但 env 文件不存在”的 strict preflight 用例中新增 `warnings=[]` 断言，确保该场景不会带无关 env 缺失 warning；已执行 `bash -n scripts/production_evidence_env_preflight.sh scripts/smoke_production_evidence_gate.sh` 通过，已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过。
- 因验收脚本变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 03:32:32 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `efa1e21541b14859dabd0ec397bc149e9a4979199edbf44f839c3591d4831bf1`，纳入文件数 `604`，`source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已刷新 `production_evidence_doctor.sh` 与 `production_evidence_env_preflight.sh`；`docs/evidence/production/preflight.md` 生成时间为 `2026-07-09 03:32:58 CST`，当前仍为 `ready_count=0`、`standard_file_ready_count=0`、`capture_env_ready_count=0`、`not_ready_count=6`，并因标准证据和采集变量都未就绪而保留 `readiness.env.local` 缺失提示；未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过；`docs/evidence/production/candidate/production-evidence.json` 显示 `ok=false`、`evidence_ok=false`、`invalid_evidence=6`。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，按预期返回 `rc=1`；current 包 `manifest.json` 与 `production-evidence.json` 已记录当前源码指纹 `efa1e21541b14859dabd0ec397bc149e9a4979199edbf44f839c3591d4831bf1`，但当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、持续运行稳定性记录。

## 阶段补充（2026-07-09 12:42）

- 本轮按用户最新要求未启动新的 24 小时持续运行；`docs/evidence/latest/index.md`、`docs/evidence/production/readiness.md`、`docs/evidence/production/preflight.md`、`docs/evidence/production/candidate/index.md` 和 `docs/evidence/production/current/index.md` 均显示 `24 小时持续运行：未启动`，复核 `./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 将稳定性证据口径从默认 24 小时长稳改为默认短稳/健康检查/外部监控记录，legacy `soak.ndjson` 仍可导入；已更新 `scripts/capture_stability_evidence.sh`、`scripts/production_evidence_check.sh`、`scripts/production_evidence_doctor.sh`、`scripts/production_evidence_env_preflight.sh`、`scripts/init_production_evidence_pack.sh`、`scripts/collect_production_evidence_pack.sh`、`scripts/audit_goal_completion.sh`、`scripts/collect_standalone_evidence.sh` 和 `scripts/standalone_stage_report.sh`，生产证据项名称统一为 `稳定性记录`，并优先提示 `MOCHAT_STABILITY_MONITOR_EVIDENCE`、`MOCHAT_STABILITY_HEALTH_EVIDENCE`、`MOCHAT_STABILITY_RESOURCE_EVIDENCE`、`MOCHAT_STABILITY_TIME_RANGE`、`MOCHAT_STABILITY_LOG_REF`。
- 补齐独立版 RC 交付材料：新增 `docs/release-candidate.md` 和 `deploy/standalone/.env.example`，更新 `deploy/standalone/docker-compose.yml` 支持 MySQL 库名、账号、密码和 root 密码从 env 注入，更新 `deploy/standalone/README.md` 与 `README.md`，明确复制 `.env.example` 为 `.env.local` 后用 `docker compose --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d` 启动。
- 已执行 `bash -n scripts/capture_stability_evidence.sh scripts/production_evidence_check.sh scripts/production_evidence_doctor.sh scripts/production_evidence_env_preflight.sh scripts/smoke_production_evidence_gate.sh scripts/init_production_evidence_pack.sh scripts/audit_goal_completion.sh scripts/collect_standalone_evidence.sh scripts/standalone_stage_report.sh scripts/collect_production_evidence_pack.sh` 通过；已执行 `docker compose --env-file deploy/standalone/.env.example -f deploy/standalone/docker-compose.yml config` 通过；已执行 `./scripts/capture_stability_evidence.sh --help` 和 `./scripts/audit_goal_completion.sh --help` 验证帮助文案已切换为短稳/外部监控默认口径。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_production_evidence_gate.sh` 通过，覆盖稳定性证据新字段、doctor/env preflight 提示和 legacy soak 兼容断言。
- 因交付文档、compose 和验收脚本变化导致源码指纹变化，已重新执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all ./scripts/collect_standalone_evidence.sh` 并通过，最新证据写入 `docs/evidence/latest/`；`index.md` 生成时间为 `2026-07-09 12:39:02 CST`，结论为 `本地短门禁通过，目标未完成，仍需补生产证据`。
- 最新运行时路由覆盖为 manifest 路由 `224`、已迁移 `224`、未迁移 `0`、Go 额外运行时路由 `209`；源码与验收指纹为 `e269711252a99067ca363548ea743eefa71bbd2eb74a38a6e989ba2e5b487a91`，纳入文件数 `605`，`env -u GOROOT ./scripts/source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json` 通过。
- 已刷新 `production_evidence_doctor.sh` 与 `production_evidence_env_preflight.sh`；`docs/evidence/production/readiness.md` 生成时间为 `2026-07-09 12:39:17 CST`，`docs/evidence/production/preflight.md` 生成时间为 `2026-07-09 12:39:22 CST`，当前仍为 `ready_count=0`、`standard_file_ready_count=0`、`capture_env_ready_count=0`、`not_ready_count=6`，未访问生产、未启动 24 小时运行。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh`，生产候选门禁于 `2026-07-09 12:40:25 CST` 按预期返回 `rc=1`：严格生产证据检查和严格目标完成度审计失败，源码与最新本地短证据包指纹匹配通过；`docs/evidence/production/candidate/production-evidence.json` 与 `goal-completion.json` 均可解析。
- 已执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/collect_production_evidence_pack.sh`，报告写入 `docs/evidence/production/current/index.md`，生成时间为 `2026-07-09 12:41:23 CST`，按预期返回 `rc=1`；current 包 `manifest.json` 与 `production-evidence.json` 已记录当前源码指纹 `e269711252a99067ca363548ea743eefa71bbd2eb74a38a6e989ba2e5b487a91`，但当前 6 类真实生产证据仍全部缺失：MySQL 5.7 amd64 真实容器门禁、真实企业微信账号联调、真实微信开放平台联调、真实 SaaS 多租户数据回归、生产前端浏览器回归、稳定性记录。

## 阶段补充（2026-07-09 14:13）

- 本轮按“执行建议”继续推进 SaaS 总后台能力，未启动新的 24 小时持续运行。
- 新增 SaaS 总后台只读总览：`MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 后启用 `GET/HEAD /dashboard/saasAdmin/page` 和 `GET /dashboard/saasAdmin/overview`；`MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID` 默认为 `1`，只有平台租户超级管理员可使用 `scope=platform` 查看全局，普通租户超级管理员只能查看本租户。
- 后端总览当前覆盖租户数、有效租户套餐数、启用套餐数、用户数、企业数、打开告警数、待发送/失败通知数、即将到期和已到期租户数；租户列表展示套餐、到期状态、打开告警数和最高用量指标；指标列表展示用量、额度、使用率和打开告警数。
- 新增 `scripts/smoke_saas_admin_dashboard.sh` 并接入 `MOCHAT_ACCEPTANCE_SUITE=saas`，脚本会开临时库、迁移、开平台租户和普通租户、注入到期/告警/待通知/用量样本，验证平台全局视图、平台按租户下钻、普通租户租户视图和普通租户越权 `scope=platform` 返回 `403`。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/config ./internal/store` 通过。
- 已尝试执行 `DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh`，但当前环境 Docker daemon 未启动，失败信息为 `Cannot connect to the Docker daemon at unix:///var/run/docker.sock`；因此本轮未刷新 `docs/evidence/latest/`，源码指纹相对最新证据包已因新增代码、脚本和文档发生变化。

## 阶段补充（2026-07-09 14:24）

- 继续推进 SaaS 总后台从只读看板向平台运营后台演进；本轮未启动新的 24 小时持续运行。
- 新增平台级套餐运营接口：`GET /dashboard/saasAdmin/packages` 返回可分配套餐和 26 项 SaaS 额度；`POST/PUT /dashboard/saasAdmin/tenantPackage` 支持平台租户超级管理员给指定租户调整套餐和到期时间。
- 租户套餐调整会读取 `mochat_go_saas_packages` 的当前额度，写入 `mochat_go_saas_tenant_packages.limits_json` 快照，并调用现有 `RefreshSaaSUsageCounters` 刷新该租户 26 项 `mochat_go_saas_usage_counters`；普通租户超级管理员调用套餐列表或套餐调整接口会返回 `403`。
- 内置 `/dashboard/saasAdmin/page` 已增加“租户套餐调整”操作条：平台管理员可输入租户 ID、选择套餐、填写到期时间并提交，提交成功后自动刷新 overview。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：在原有页面、平台 overview、租户 overview 和越权 `scope=platform` 验证之外，新增套餐列表、平台调整租户套餐、普通租户越权套餐接口返回 `403`、调整后 `users` 额度从 `10` 刷新为 `100` 的断言；该脚本仍接入 `MOCHAT_ACCEPTANCE_SUITE=saas`。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/config ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 仍停在 `Cannot connect to the Docker daemon at unix:///var/run/docker.sock`，所以本轮未刷新 `docs/evidence/latest/`。

## 阶段补充（2026-07-09 14:34）

- 继续执行 SaaS 总后台完善建议；本轮未启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 新增平台级套餐维护接口：`POST/PUT /dashboard/saasAdmin/package` 支持平台租户超级管理员创建、编辑、启用和停用套餐，并维护 26 项 SaaS 额度；普通租户超级管理员调用该接口返回 `403`，停用套餐不能被分配给租户。
- 内置 `/dashboard/saasAdmin/page` 已增加“套餐维护”区：平台管理员可录入套餐编码、套餐名称、状态和额度 JSON，保存后自动刷新套餐列表；选择已有套餐时会回填编辑表单。
- `scripts/smoke_saas_admin_dashboard.sh` 已扩展套餐生命周期断言：创建 `scale` 套餐、验证额度、普通租户越权创建返回 `403`、停用后分配返回 `400`、恢复后把 `maxUsers` 改为 `120`，再分配给租户并确认 `users` 用量额度刷新为 `120`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `POST/PUT /dashboard/saasAdmin/package` 加入 SaaS 总后台交付说明、路由表和短链路 smoke 说明。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/config ./internal/store` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json`，按预期因本轮源码、脚本和文档变化返回指纹不匹配；检查输出显示旧证据指纹为 `e269711252a99067ca363548ea743eefa71bbd2eb74a38a6e989ba2e5b487a91`，执行时当前指纹为 `8aee75b9cf9118002f809bc9cc2a2f76f1e2d5428717eaa5caf2a76697331c04`。

## 阶段补充（2026-07-09 14:49）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮未启动新的 24 小时持续运行。
- 新增迁移 `0033_saas_admin_operation_logs`，创建 `mochat_go_saas_admin_operation_logs`，用于记录 SaaS 总后台套餐维护、租户开停、租户套餐调整等平台运营动作。
- 新增平台级操作日志接口：`GET /dashboard/saasAdmin/operations`，支持平台租户超级管理员查看操作流水，并可用 `tenantId` 过滤；普通租户超级管理员访问返回 `403`。
- 新增租户状态接口：`POST/PUT /dashboard/saasAdmin/tenantStatus`，支持平台租户超级管理员把业务租户切换为正常或停用，并写入操作日志；平台管理租户不能被停用。
- 已将 `POST/PUT /dashboard/saasAdmin/package` 和 `POST/PUT /dashboard/saasAdmin/tenantPackage` 调整为写入 `mochat_go_saas_admin_operation_logs`，后续总后台可回看套餐维护和租户套餐调整历史。
- 内置 `/dashboard/saasAdmin/page` 已增加“租户状态”操作条和“操作记录”列表；页面会在保存套餐、更新租户状态、应用租户套餐后刷新操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：新增 `0033` 迁移断言、页面入口断言、平台开停租户、普通租户越权 `tenantStatus/operations` 返回 `403`、平台租户停用返回 `400`、操作日志包含 `tenant.status`、`package.upsert`、`tenant.package` 的断言。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/config ./internal/store` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`。
- 已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json`，按预期因本轮源码、脚本和文档变化返回指纹不匹配；检查输出显示旧证据指纹为 `e269711252a99067ca363548ea743eefa71bbd2eb74a38a6e989ba2e5b487a91`，执行时当前指纹为 `4fff520208bae7a5e54b9faadc7402bd92c15788eda83d4c660a4f4e07f67c85`。

## 阶段补充（2026-07-09 14:56）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 已把 `mc_tenant.status=2` 接入运行时用户链路：`UserAuthByPhone/UserAuthByID` 会读取租户状态，登录接口遇到停用租户返回 `403` 和 `租户已停用`；`UserByID` 会左连接 `mc_tenant`，停用租户用户按不可用处理，让已有 token 访问依赖 `UserByID` 的后台接口失效。
- `dashboard.User` 和 `dashboard.AuthUser` 已增加 `TenantStatus`；`/dashboard/user/loginShow` 在解析到停用租户时返回 `403`，正常响应中也带出 `tenantStatus`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：平台管理员停用业务租户后，业务租户用户访问 `/dashboard/saasAdmin/overview` 必须返回 `401`；平台管理员恢复租户后，同一用户访问恢复为 `200`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，明确租户停用不只是总后台展示状态，也会阻断新登录和已登录后台访问。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/config ./internal/store` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`。
- 已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --check docs/evidence/latest/source-fingerprint.json`，按预期因本轮源码、脚本和文档变化返回指纹不匹配；检查输出显示旧证据指纹为 `e269711252a99067ca363548ea743eefa71bbd2eb74a38a6e989ba2e5b487a91`，执行时当前指纹为 `f6ebaac76109fde171f2cdb94b574c8c35597fb6b2fdad096eb8e03aee0c91e4`。

## 阶段补充（2026-07-09 15:05）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增 `GET /dashboard/saasAdmin/tenant` 单租户详情接口：平台租户超级管理员可按 `tenantId` 查看任意租户，普通租户超级管理员只能查看本租户；接口聚合 `summary`、`tenant`、`metrics` 和近期 `operations`，复用现有 SaaS 总览、用量、告警和操作日志口径。
- 内置 `/dashboard/saasAdmin/page` 已增加“租户详情”区，填写租户 ID 或处于当前租户视角时会调用单租户详情接口，展示租户状态、当前套餐、到期状态、最高用量和近期操作数。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/tenant`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“租户详情”；平台管理员和租户管理员分别调用 `GET /dashboard/saasAdmin/tenant`；普通租户越权查看平台租户详情返回 `403`；详情响应断言覆盖租户名称、最高用量、指标额度和授权范围。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `GET /dashboard/saasAdmin/tenant` 加入 SaaS 总后台交付说明、路由表和短链路 smoke 说明。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`。
- 已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `c627ca620e4e364a122036166a46a783e655c3482ebb70daa4c85e236fad59db`。

## 阶段补充（2026-07-09 15:20）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增迁移 `0034_saas_billing_events`，创建 `mochat_go_saas_billing_events`，用于记录租户续费、金额、币种、支付时间、支付方式、外部订单号、前后到期时间和操作人。
- 新增平台级账单事件接口：`GET /dashboard/saasAdmin/billingEvents`，支持平台租户超级管理员查看续费账单流水，并可按 `tenantId` 过滤；普通租户超级管理员访问返回 `403`。
- 新增平台级租户续费接口：`POST/PUT /dashboard/saasAdmin/tenantRenewal`，支持平台租户超级管理员记录租户续费；服务端会在同一事务中更新 `mochat_go_saas_tenant_packages.expires_at`、写入 `mochat_go_saas_billing_events`，并写入 `tenant.renewal` 操作日志，提交后刷新租户 SaaS 用量额度。
- 内置 `/dashboard/saasAdmin/page` 已增加“续费账单”操作条和“账单事件”列表；页面可提交续费租户、套餐、到期时间、金额和订单号，并刷新 overview、操作记录和账单事件。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `GET /dashboard/saasAdmin/billingEvents` 和 `POST/PUT /dashboard/saasAdmin/tenantRenewal`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：新增 `0034` 迁移断言、页面“账单事件”断言、普通租户越权账单接口返回 `403`、平台管理员续费租户、账单事件字段、续费后到期时间和 `tenant.renewal` 操作日志断言。
- 已更新 `README.md`、`docs/release-candidate.md` 和 `deploy/standalone/migrations/README.md`，把续费账单能力加入 SaaS 总后台交付说明、路由表、短链路 smoke 说明和迁移规范。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `8fa479b16ca97df06dbfce8d099876c04dec030d64bb77ed321e4c8f9b07095e`。

## 阶段补充（2026-07-09 15:34）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 新增平台级平台开户接口：`POST/PUT /dashboard/saasAdmin/tenantProvision`，仅平台租户超级管理员可用；服务端会创建或恢复业务租户、初始超级管理员、超级管理员角色、角色菜单授权、租户套餐快照、开通记录和 `tenant.provision` 操作日志，并在提交后刷新该租户 26 项 SaaS 用量计数。
- 开户接口复用 dashboard 密码哈希口径，不把明文密码交给 store 或响应；管理员手机号如果已经被其他租户占用会返回 `400`，避免现有按手机号登录链路出现跨租户歧义。
- 内置 `/dashboard/saasAdmin/page` 已增加“新租户开户”操作条：平台管理员可填写租户名称、管理员手机、初始密码、套餐和到期时间，提交成功后自动切换到新租户并刷新 overview、操作记录和账单事件。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `POST/PUT /dashboard/saasAdmin/tenantProvision`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：新增页面“新租户开户”断言、普通租户越权平台开户返回 `403`、平台管理员真实平台开户、管理员登录、租户详情、租户自查、`mochat_go_tenant_provision_runs`、`users` 用量 `1/10` 和 `tenant.provision` 操作日志断言。
- 已更新 `README.md`、`docs/release-candidate.md` 和 `deploy/standalone/migrations/README.md`，把平台开户能力加入 SaaS 总后台交付说明、路由表、短链路 smoke 说明和操作日志迁移口径。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。

## 阶段补充（2026-07-09 15:43）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 增强 `GET /dashboard/saasAdmin/overview` 的租户列表筛选：新增 `keyword`、`tenantStatus=1/2`、`packageCode` 和 `dueState=all/normal/expiring/expired/no_package` 查询参数；筛选只作用于 `tenants` 列表，`summary` 和 `metrics` 继续按 `scope/tenantId` 原统计口径返回，避免平台总览指标被列表筛选误读。
- `overview` 响应新增 `filters` 回显，服务端会校验非法 `tenantStatus` 和 `dueState` 并返回 `400`；平台租户超级管理员可在 `scope=platform` 下组合筛选，普通租户仍受原有租户权限约束。
- 内置 `/dashboard/saasAdmin/page` 已增加“租户列表筛选”操作条：支持按租户名称/ID/套餐关键词、套餐、租户状态和到期状态筛选风险列表，并在状态栏显示当前筛选条件。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“租户列表筛选”；平台管理员会用关键词、套餐、租户状态和即将到期状态组合过滤普通租户，并断言过滤后的 `tenants` 只包含目标租户、`filters` 正确回显、`summary.tenantCount` 仍保持平台总览口径。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 SaaS 总后台 overview 筛选参数、summary 口径和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `adcbe2d4aaa61fc0275b863418f3bc867855f8261898edc4ed8cea1513cb607e`。

## 阶段补充（2026-07-09 15:51）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 增强 `GET /dashboard/saasAdmin/operations` 操作记录筛选：新增 `action`、`targetType` 和 `keyword` 查询参数；`keyword` 会匹配动作、目标类型、目标 ID、目标名称、备注和前后 JSON，响应新增 `filters` 回显，非法超长参数会返回 `400`。
- 增强 `GET /dashboard/saasAdmin/billingEvents` 账单流水筛选：新增 `eventType`、`packageCode` 和 `keyword` 查询参数；`keyword` 会匹配账单类型、套餐、支付方式、外部订单号、备注和 metadata，响应新增 `filters` 回显，非法超长参数会返回 `400`。
- 内置 `/dashboard/saasAdmin/page` 已增加“操作与账单筛选”操作条：平台管理员可在页面上按动作、目标类型、操作关键字、账单类型、账单套餐和账单关键字过滤操作记录与账单流水。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“操作与账单筛选”；操作记录会按 `tenantId + action=tenant.package + targetType=tenant + keyword=scale` 过滤并断言只返回目标租户套餐调整；账单流水会按 `tenantId + eventType=renewal + packageCode=scale + keyword=SMOKE-RENEWAL-*` 过滤并断言只返回目标续费订单。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把操作记录、账单流水的筛选参数和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `0c11e85afa7d803144d19c9c13afc71503cca1224ba354ef33388ba0120c03b7`。

## 阶段补充（2026-07-09 15:57）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 已把 SaaS 租户套餐到期接入运行时用户链路：`UserAuthByPhone/UserAuthByID` 会读取 `mochat_go_saas_tenant_packages.expires_at`，启用套餐已过期时设置 `TenantPackageExpired`；`POST /dashboard/user/auth` 会返回 `403` 和 `租户套餐已到期`。
- 已把到期租户接入旧 token 失效链路：`UserByID` 检测到启用套餐已过期时按用户不可用处理，让依赖 `UserByID` 的后台接口返回未授权；SaaS 表不存在或历史租户没有启用套餐时不强制拦截，避免非 SaaS/迁移中环境被误伤。
- `dashboard.User` 和 `dashboard.AuthUser` 已增加 `TenantPackageExpired/TenantPackageExpiresAt`；`/dashboard/user/loginShow` 在 store 显式返回到期状态时返回 `403` 和 `租户套餐已到期`，正常响应中也带出套餐到期字段。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：普通租户先验证登录成功，再把租户套餐到期时间改到昨天；新登录必须返回 `403` 且消息为 `租户套餐已到期`，旧用户访问 SaaS 总览必须返回 `401`；平台管理员通过 `tenantRenewal` 续费到未来时间后，新登录和租户 overview 恢复为 `200`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把套餐到期拦截、续费恢复和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `4135e7e14e26ab1076c3dda2e0a861b74d38dee07392f6a587bebdd740fedef1`。

## 阶段补充（2026-07-09 16:12）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 新增平台级 CSV 导出接口：`GET /dashboard/saasAdmin/export`，仅平台租户超级管理员可用；`type=tenants` 导出租户列表，`type=operations` 导出操作记录，`type=billingEvents` 导出账单流水，默认导出 1000 条，上限 5000 条。
- 导出接口复用现有筛选口径：租户 CSV 支持 `tenantId`、`keyword`、`tenantStatus`、`packageCode`、`dueState` 和 `expiringDays`；操作记录 CSV 支持 `tenantId`、`action`、`targetType`、`keyword`；账单流水 CSV 支持 `tenantId`、`eventType`、`packageCode`、`keyword`。
- 内置 `/dashboard/saasAdmin/page` 已增加“数据导出”操作条，可在页面用当前筛选条件下载租户、操作记录和账单流水 CSV；下载通过 `fetch` 携带 Dashboard JWT，避免浏览器原生下载无法带 Authorization header。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/export`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“数据导出”；普通租户调用 CSV 导出返回 `403`；平台管理员按筛选条件导出租户、操作记录和账单流水 CSV，并用 Python `csv` 标准库校验表头、目标租户、套餐调整和续费订单字段。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `GET /dashboard/saasAdmin/export`、CSV 导出筛选参数和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。

## 阶段补充（2026-07-09 16:23）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 新增单租户用量明细接口：`GET /dashboard/saasAdmin/usage`，权限与单租户详情一致；平台租户超级管理员可按 `tenantId` 查看任意租户，普通租户超级管理员只能查看本租户，跨租户访问返回 `403`。
- 用量明细响应会返回租户信息、用量 summary 和 `usageMetrics`，每个指标包含 `metric`、中文 label、`periodKey`、当前用量、额度、剩余额度、是否不限额、状态、打开告警数、更新来源和更新时间；状态统一由后端按 `unlimited/normal/warning/exceeded` 计算。
- MySQL store 新增 `SaaSAdminTenantUsage`，直接查询 `mochat_go_saas_usage_counters`，并按 `metric + period_key` 关联 `mochat_go_saas_alerts` 的打开告警数，不新增迁移表。
- 内置 `/dashboard/saasAdmin/page` 已增加“用量明细”表，选择租户或租户自查时会自动调用 `/dashboard/saasAdmin/usage`，展示 26 项指标的用量、剩余、状态、告警和更新时间。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/usage`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“用量明细”；平台管理员和租户管理员分别调用 `GET /dashboard/saasAdmin/usage`；普通租户越权查看平台租户用量返回 `403`；响应断言覆盖 `users` 当前用量、额度、剩余、状态、打开告警数和 26 项指标数量。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `GET /dashboard/saasAdmin/usage`、用量明细字段和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。

## 阶段补充（2026-07-09 16:48）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增 SaaS 总后台告警列表接口：`GET /dashboard/saasAdmin/alerts`，支持 `tenantId`、`status=open/resolved/all`、`metric`、`alertType`、`page`、`perPage/limit` 筛选；平台租户超级管理员可查看任意租户，普通租户超级管理员只能查看本租户，跨租户访问返回 `403`。
- 新增 SaaS 总后台告警解决接口：`POST/PUT /dashboard/saasAdmin/alertResolve`，按 `tenantId + metric + alertType + periodKey` 解决打开告警；平台租户超级管理员可处理任意租户，普通租户超级管理员只能处理本租户。
- MySQL store 新增 `ResolveSaaSAdminAlert`，在事务内锁定 `mochat_go_saas_alerts` 打开告警、更新为 `resolved`，并写入 `mochat_go_saas_admin_operation_logs`，动作名为 `tenant.alert.resolve`；未新增迁移表。
- 内置 `/dashboard/saasAdmin/page` 已增加“告警处置”筛选条和告警列表；页面可按状态、指标、告警类型读取告警，并对打开告警执行“解决”，成功后刷新总览、用量、告警和操作记录。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/alerts` 和 `go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/alertResolve`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“告警处置”；平台管理员和租户管理员分别调用 `GET /dashboard/saasAdmin/alerts`；普通租户越权查看平台租户告警返回 `403`；后段真实调用 `POST /dashboard/saasAdmin/alertResolve`，断言告警转为 `resolved`、用量明细 `users.openAlertCount` 归零，并出现 `tenant.alert.resolve` 操作日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `GET /dashboard/saasAdmin/alerts`、`POST/PUT /dashboard/saasAdmin/alertResolve`、页面告警处置和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。

## 阶段补充（2026-07-09 17:08）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增 SaaS 总后台通知 outbox 列表接口：`GET /dashboard/saasAdmin/notifications`，支持 `tenantId`、`status=pending/failed/delivered/dead/all`、`channel`、`keyword` 和 `limit` 筛选；平台租户超级管理员可查看任意租户，普通租户超级管理员只能查看本租户，跨租户访问返回 `403`。
- 新增 SaaS 总后台通知重试接口：`POST/PUT /dashboard/saasAdmin/notificationRetry`，按 `notificationId` 将 failed/dead 通知重置为 pending、`attempts=0`、清空 `last_error`、设置 `next_retry_at=NOW()`，实际发送仍由既有通知 dispatch cron 或维护命令完成。
- MySQL store 新增 `SaaSAdminAlertNotifications` 和 `RetrySaaSAdminAlertNotification`；重试动作在事务内锁定 `mochat_go_saas_alert_notifications`，并写入 `mochat_go_saas_admin_operation_logs`，动作名为 `tenant.notification.retry`；未新增迁移表。
- 内置 `/dashboard/saasAdmin/page` 已增加“通知重试”筛选条和通知列表；页面可按状态、通道、关键字查看通知 outbox，并对 failed/dead 通知执行“重试”，成功后刷新通知列表和操作记录。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/notifications` 和 `go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationRetry`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“通知重试”；平台管理员和租户管理员分别调用 `GET /dashboard/saasAdmin/notifications`；普通租户越权查看平台租户通知返回 `403`；后段真实调用 `POST /dashboard/saasAdmin/notificationRetry`，断言 dead 通知回到 pending、失败原因清空，并出现 `tenant.notification.retry` 操作日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `GET /dashboard/saasAdmin/notifications`、`POST/PUT /dashboard/saasAdmin/notificationRetry`、页面通知重试和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本轮未启动新的 24 小时持续运行。本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `34e5d417410c176a75c9abbe0cb900b2aaac2589ec831c6d143f5182232d8268`。

## 阶段补充（2026-07-09 17:18）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增 SaaS 总后台通知批量重试接口：`POST/PUT /dashboard/saasAdmin/notificationBulkRetry`，支持 `tenantId`、`status=failed/dead/all`、`channel`、`keyword`、`limit` 和 `remark`；接口只会批量重置 failed/dead 通知，不允许 delivered/pending 作为批量重试目标。
- MySQL store 新增 `BulkRetrySaaSAdminAlertNotifications`，在事务内按筛选条件锁定 `mochat_go_saas_alert_notifications`，逐条重置为 `pending`、`attempts=0`、清空 `last_error`、设置 `next_retry_at=NOW()`，并为每条通知写入 `tenant.notification.retry` 操作日志；未新增迁移表。
- 权限口径与单条通知重试一致：平台租户超级管理员可按筛选条件批量处理任意租户，普通租户超级管理员只能批量处理本租户，跨租户批量重试返回 `403`。
- 内置 `/dashboard/saasAdmin/page` 的“通知重试”区域已增加“批量重试”按钮，会提交当前租户、状态、通道和关键字筛选条件；批量成功后刷新通知列表、操作记录和总览。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationBulkRetry`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“批量重试”；普通租户越权批量重试平台租户通知返回 `403`；后段真实调用 `POST /dashboard/saasAdmin/notificationBulkRetry`，断言 2 条 failed 通知批量回到 pending、失败原因清空，并为每条通知生成带 `bulkRetry=true` 的 `tenant.notification.retry` 操作日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `POST/PUT /dashboard/saasAdmin/notificationBulkRetry`、页面批量重试和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `c4d394174e2388d7695e66905998bc4dd6850c1c8094ec5d28d8ea485e0c9a82`。

## 阶段补充（2026-07-09 17:36）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增 SaaS 总后台告警批量解决接口：`POST/PUT /dashboard/saasAdmin/alertBulkResolve`，支持 `tenantId`、`metric`、`alertType`、`periodKey`、`limit` 和 `remark`；接口只会批量解决 `open` 告警，不会改动已解决告警。
- MySQL store 新增 `BulkResolveSaaSAdminAlerts`，在事务内按筛选条件锁定 `mochat_go_saas_alerts`，逐条更新为 `resolved` 并写入 `resolved_at`；每条告警都会写入 `tenant.alert.resolve` 操作日志，`after_json` 标记 `bulkResolve=true`；未新增迁移表。
- 权限口径与单条告警解决一致：平台租户超级管理员可按筛选条件批量处理任意租户，普通租户超级管理员只能批量处理本租户，跨租户批量解决返回 `403`。
- 内置 `/dashboard/saasAdmin/page` 的“告警处置”区域已增加“批量解决”按钮，会提交当前租户、指标和告警类型筛选条件；批量成功后刷新总览、告警列表、操作记录和通知列表。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/alertBulkResolve`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“批量解决”；普通租户越权批量解决平台租户告警返回 `403`；后段真实调用 `POST /dashboard/saasAdmin/alertBulkResolve`，断言 2 条 open 告警批量转为 resolved，并为每条告警生成带 `bulkResolve=true` 的 `tenant.alert.resolve` 操作日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `POST/PUT /dashboard/saasAdmin/alertBulkResolve`、页面告警批量解决和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`DOCKER_CONFIG=/tmp/mochat-go-docker-config env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh` 失败信息为 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?`，所以仍未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `359fd578fcbd98bc0a250fd7af44f3ffc7810d665e9125dfeff2d1f57728b89d`。

## 阶段补充（2026-07-09 风险看板）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 新增 SaaS 总后台风险看板接口：`GET /dashboard/saasAdmin/risk`，复用 overview 的 `scope`、`tenantId`、`keyword`、`tenantStatus`、`packageCode`、`dueState`、`expiringDays` 和 `limit` 筛选口径，并新增 `highUsageRatio` 高用量阈值。
- 风险看板按停用、套餐到期、未开通套餐、打开告警和高用量/超额指标计算 `riskLevel`、`riskScore`、`riskReasons`、`suggestedAction` 和 `topUsageMetrics`；平台租户超级管理员可看全平台或任意租户，普通租户超级管理员只能看本租户，跨租户访问返回 `403`。
- 内置 `/dashboard/saasAdmin/page` 已增加“风险看板”区块和“高用量阈值”输入框；页面会调用 `/dashboard/saasAdmin/risk` 展示风险租户、风险原因、Top 风险指标和建议动作。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/risk`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“风险看板”；平台管理员和租户管理员分别调用 `GET /dashboard/saasAdmin/risk`；普通租户越权查看平台租户风险返回 `403`；响应断言覆盖风险等级、风险分、风险原因、建议动作和 Top 风险指标。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `GET /dashboard/saasAdmin/risk`、页面风险看板和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行 Docker 版短 smoke，也未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `8deef0e90e46ad825385cbe2b608438de4680a73aef5d5e998e9eb5b87461417`。

## 阶段补充（2026-07-09 风险导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行，`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=risk` 现在可导出风险看板 CSV，复用租户筛选条件，并支持 `highUsageRatio` 高用量阈值。
- 风险 CSV 输出 `tenantId`、`tenantName`、`riskLevel`、`riskScore`、`riskReasons`、`suggestedAction`、租户状态、套餐、到期、打开告警、最高用量指标和 `topUsageMetrics`，便于平台运营直接拉取风险租户跟进续费、扩容或客户成功动作。
- 权限口径与现有 CSV 导出一致：仅平台租户超级管理员可导出；普通租户超级管理员访问 `/dashboard/saasAdmin/export` 仍返回 `403`。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域已增加“导出风险 CSV”按钮，使用当前风险看板筛选条件下载 `mochat-saas-risk-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出风险 CSV”；在告警解决前真实调用 `GET /dashboard/saasAdmin/export?type=risk`，断言风险 CSV 表头、租户风险等级、风险分、原因、建议动作和 Top 风险指标。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `type=risk`、风险 CSV 字段口径和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行 Docker 版短 smoke，也未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `7c89b3f79e61222406472d9c362cb6077b647bad2d8a67da3a8ee1e3a04bf0a0`。

## 阶段补充（2026-07-09 17:56 风险跟进闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按建议推进风险跟进闭环，没有启动新的 24 小时持续运行。
- 新增 SaaS 总后台风险跟进接口：`POST/PUT /dashboard/saasAdmin/riskFollowUp`，仅平台租户超级管理员可用；普通租户超级管理员调用返回 `403`。
- 接口接收 `tenantId`、`status`、`owner`、`nextFollowUpAt` 和 `remark`，其中 `status` 限定为 `pending`、`contacted`、`renewal_pending`、`resolved`、`ignored`，`nextFollowUpAt` 复用总后台日期时间格式校验。
- MySQL store 新增 `RecordSaaSAdminRiskFollowUp`：先确认租户存在，再写入 `mochat_go_saas_admin_operation_logs`，动作名为 `tenant.risk.follow_up`，`after_json` 记录跟进状态、负责人、下次跟进时间和备注；未新增迁移表。
- 内置 `/dashboard/saasAdmin/page` 已增加风险跟进操作栏和风险租户行级“记录跟进”按钮；记录成功后刷新操作日志和单租户详情。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/riskFollowUp`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“风险跟进”；普通租户越权风险跟进返回 `403`；平台管理员真实调用 `POST /dashboard/saasAdmin/riskFollowUp`，再通过 `GET /dashboard/saasAdmin/operations?action=tenant.risk.follow_up&targetType=tenant&keyword=smoke-risk-follow-up` 查回操作日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `POST/PUT /dashboard/saasAdmin/riskFollowUp`、页面风险跟进和 smoke 验收说明补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行 Docker 版短 smoke，也未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `f47c001f6ccc9f1bd598c5f7ca2bf2f60699e49fabddc4d51077837abc3ca4ad`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 18:10 风险跟进状态回显）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 增强 `GET /dashboard/saasAdmin/risk`：平台租户超级管理员视图会按风险租户批量读取最新 `tenant.risk.follow_up` 操作日志，并在每个风险租户 payload 中回显 `followUp.status`、`owner`、`nextFollowUpAt`、`remark`、`operationId` 和 `createdAt`。
- 风险 summary 新增 `followUpTenantCount`、`pendingFollowUpCount`、`overdueFollowUpCount` 和 `renewalPendingCount`，用于总后台直接查看已跟进、待跟进、逾期跟进和续费中的风险租户数量；普通租户风险自查仍不暴露平台内部跟进备注。
- 增强 `GET /dashboard/saasAdmin/export?type=risk`：风险 CSV 现在输出 `followUpStatus`、`followUpOwner`、`nextFollowUpAt`、`followUpRemark` 和 `followUpCreatedAt`，并继续保留 `topUsageMetrics` 为最后一列。
- MySQL store 新增 `SaaSAdminLatestRiskFollowUps`，从 `mochat_go_saas_admin_operation_logs` 按租户取最新一条 `tenant.risk.follow_up`，解析 `after_json` 后回填风险看板；未新增迁移表。
- 内置 `/dashboard/saasAdmin/page` 的风险看板已增加“跟进”列，展示最新跟进状态、负责人、下次跟进时间和备注；风险计数区会显示已跟进、逾期和续费中的数量，记录跟进后会刷新风险看板。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：平台管理员记录风险跟进后，再次读取风险看板和风险 CSV，断言最新跟进状态、负责人、下次跟进时间和备注已回显；初始风险 CSV 断言同步适配新增字段。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把风险看板最新跟进回显、风险 CSV 跟进字段和普通租户不暴露平台内部备注的验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行 Docker 版短 smoke，也未刷新 `docs/evidence/latest/`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `7b3237c363386d6f2d93d673b93d0084f9c01c4a93546c97b23df97c9a3beb8d`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 风险跟进任务列表）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增 SaaS 总后台风险跟进任务接口：`GET /dashboard/saasAdmin/riskFollowUps`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 接口从 `mochat_go_saas_admin_operation_logs` 中按租户读取最新一条 `tenant.risk.follow_up`，解析 `after_json` 后返回任务列表；未新增迁移表。
- 支持筛选 `tenantId`、`status`、`owner`、`dueState=all/overdue/due_soon/future/no_date/closed`、`keyword` 和 `limit`，并返回任务汇总：总数、各跟进状态数量、逾期、7 天内、无日期和已关闭数量。
- 内置 `/dashboard/saasAdmin/page` 已增加“风险跟进任务”筛选条和任务表，平台运营可按负责人、状态、到期状态和关键字筛选；记录风险跟进后会刷新任务列表。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/riskFollowUps`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“风险跟进任务”；普通租户越权访问任务列表返回 `403`；平台管理员记录风险跟进后调用 `GET /dashboard/saasAdmin/riskFollowUps`，断言状态、负责人、下次跟进时间、备注、到期状态和操作 ID 回显正确。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `GET /dashboard/saasAdmin/riskFollowUps`、页面任务列表和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the Docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `d7a19495ddc739c7c2ac75bbc2d3e2b07522c49be562e9b830b6819d5d849c3d`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 风险跟进任务导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=riskFollowUps` 现在可导出风险跟进任务 CSV。
- 导出复用风险跟进任务筛选口径：`tenantId`、`status`、`owner`、`dueState`、`keyword` 和 `limit`；平台租户超级管理员可导出，普通租户超级管理员仍返回 `403`。
- 风险跟进任务 CSV 输出 `operationId`、`tenantId`、`tenantName`、`status`、`owner`、`nextFollowUpAt`、`dueState`、`overdue`、`daysUntil`、`remark` 和 `createdAt`，方便运营按负责人或逾期状态线下跟进。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域已增加“导出跟进 CSV”按钮，使用当前风险跟进任务筛选条件下载 `mochat-saas-riskFollowUps-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出跟进 CSV”；普通租户越权导出风险跟进任务返回 `403`；平台管理员记录风险跟进后调用 `GET /dashboard/saasAdmin/export?type=riskFollowUps`，断言表头、状态、负责人、下次跟进时间、备注、到期状态和操作 ID 回显正确。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `type=riskFollowUps`、风险跟进任务 CSV 字段和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the Docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `3d37808a34d6588dad359250c03a40309457a4dc9e0896b178dada2ab76e9f3e`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 用量明细导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=usage` 现在可导出用量明细 CSV。
- 导出复用租户筛选口径：`tenantId`、`keyword`、`tenantStatus`、`packageCode`、`dueState`、`expiringDays` 和 `limit`；平台租户超级管理员可导出，普通租户超级管理员仍返回 `403`。
- 用量 CSV 按“租户 + 指标”输出，字段包含 `tenantId`、`tenantName`、租户状态、套餐、到期时间、`metric`、指标中文名、周期、当前用量、额度、剩余、是否不限额、用量比例、状态、打开告警数、更新来源和更新时间。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域已增加“导出用量 CSV”按钮，使用当前租户列表筛选条件下载 `mochat-saas-usage-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出用量 CSV”；后段调用 `GET /dashboard/saasAdmin/export?type=usage`，断言目标租户 users 指标、额度、状态和打开告警字段。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `type=usage`、用量 CSV 字段和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the Docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `d56237b197aca843d30668a5358ff07b85097bc6e5a74849d6c0ccb66e60fbb6`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 套餐清单导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=packages` 现在可导出平台套餐清单 CSV，也兼容 `type=package/plans/plan`。
- 套餐 CSV 输出套餐编码、名称、说明、状态和 26 项 SaaS 额度字段，便于销售、运营和交付按实际套餐配置核对售卖口径。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域已增加“导出套餐 CSV”按钮，使用 Dashboard JWT 下载 `mochat-saas-packages-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出套餐 CSV”；后段调用 `GET /dashboard/saasAdmin/export?type=packages`，断言 `scale` 套餐名称、状态、用户额度、存储额度和异步执行额度字段。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `type=packages`、套餐 CSV 字段和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `fbb5b8b36c205ef3386468703997c3beaa7b6b1143a4350c9e8ff5166b7ff912`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 风险跟进批量关闭）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增平台级风险跟进批量关闭接口：`POST/PUT /dashboard/saasAdmin/riskFollowUpBulkClose`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 批量关闭复用风险跟进任务筛选口径：`tenantId`、`filterStatus`、`owner`、`dueState`、`keyword` 和 `limit`；`closeStatus` 仅支持 `resolved/ignored`，服务端会为每个命中的未关闭租户追加一条 `tenant.risk.follow_up` 操作日志，不改写历史日志。
- 内置 `/dashboard/saasAdmin/page` 的“风险跟进任务”筛选条已增加“批量关闭任务”按钮，按当前筛选条件把任务批量置为 `resolved`，并刷新风险看板、任务列表和操作记录。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/riskFollowUpBulkClose`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“批量关闭任务”；普通租户越权批量关闭返回 `403`；平台管理员记录风险跟进并校验列表/CSV 后调用批量关闭接口，断言最新任务状态变为 `resolved`、到期状态变为 `closed`，并写入新的 `tenant.risk.follow_up` 操作日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `riskFollowUpBulkClose`、页面批量关闭按钮和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `704484b56f19088cc2c3b72695a75f7098e6c71001cfde57cd06e58bc8b3a2f9`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 风险跟进负责人工作台）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增 SaaS 总后台风险跟进负责人聚合接口：`GET /dashboard/saasAdmin/riskFollowUpOwners`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 负责人工作台复用风险跟进任务筛选口径：`tenantId`、`status`、`owner`、`dueState`、`keyword` 和 `limit`；底层仍从 `mochat_go_saas_admin_operation_logs` 读取最新 `tenant.risk.follow_up` 记录并解析 `after_json`，未新增迁移表。
- 接口按负责人聚合任务，空负责人归为“未分配”，返回每个负责人总任务、打开任务、各跟进状态数量、逾期、7 天内、未来、无日期、已关闭、最近跟进时间和最早下次跟进时间；排序优先级为逾期数、7 天内数、打开任务数、总任务数和负责人名称。
- 内置 `/dashboard/saasAdmin/page` 的风险跟进区域已增加“负责人工作台”表格，展示负责人维度的打开任务、状态分布、到期分布、最近跟进和下次跟进；筛选、记录跟进和批量关闭后会同步刷新负责人工作台。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/riskFollowUpOwners`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“负责人工作台”；普通租户越权访问负责人工作台返回 `403`；平台管理员记录风险跟进后调用 `GET /dashboard/saasAdmin/riskFollowUpOwners`，断言负责人、打开任务、状态数量、最近跟进时间和下次跟进时间回显正确。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `riskFollowUpOwners`、页面负责人工作台和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `a4b4ce482cc8809c825d69ad7cd5af9498bdd880acd944ed300cd5d53a20c02f`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 运营日报）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增平台级运营日报接口：`GET /dashboard/saasAdmin/dailyReport`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 日报不新增迁移表，复用现有 SaaS 总后台数据源：`SaaSAdminOverview`、单租户用量明细、风险看板构建器、风险跟进任务、告警列表、通知 outbox、操作日志和账单流水。
- 接口支持 `date=YYYY-MM-DD`、`days`、`expiringDays`、`highUsageRatio`、`tenantLimit` 和 `limit`；返回当前风险租户、风险跟进负责人分布、打开告警、失败/耗尽通知，以及日期窗口内操作数、风险跟进动作、告警处置、通知重试、续费账单数量和账单金额。
- 内置 `/dashboard/saasAdmin/page` 已增加“运营日报”区块，展示风险租户、打开跟进、告警通知和今日流水；总览刷新、风险跟进、批量关闭、通知重试等动作后会同步刷新日报。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: GET /dashboard/saasAdmin/dailyReport`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“运营日报”；普通租户越权访问运营日报返回 `403`；平台管理员记录风险跟进后调用 `GET /dashboard/saasAdmin/dailyReport`，断言打开跟进、打开告警、失败通知、当日操作、风险跟进动作和负责人分布已回显。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `dailyReport`、页面运营日报和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `69b123f6674ad830a2313af93dac2bc7a0dc80ac92f5e4cf31a96664dac4844a`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 运营日报导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=dailyReport` 现在可导出运营日报 CSV，也兼容 `type=daily/report/daily_report`。
- 日报 JSON 和日报 CSV 现在共用 `buildDailyReport` 聚合链路，避免风险、告警、通知、操作日志和账单金额两套口径漂移；原 `GET /dashboard/saasAdmin/dailyReport` 响应字段保持不变。
- 运营日报 CSV 复用 `date`、`days`、`expiringDays`、`highUsageRatio`、`tenantLimit` 和 `limit` 参数，输出 `section`、`metric`、`value`、`remark` 四列，覆盖窗口参数、汇总指标、风险负责人、风险租户、失败/耗尽通知、日期窗口内操作动作和账单流水。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域已增加“导出日报 CSV”按钮，使用当前日报阈值下载 `mochat-saas-dailyReport-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出日报 CSV”；普通租户越权导出运营日报返回 `403`；平台管理员记录风险跟进后调用 `GET /dashboard/saasAdmin/export?type=dailyReport`，断言日报 CSV 表头、打开跟进 summary、负责人和当日操作动作回显正确。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `type=dailyReport`、日报 CSV 字段和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `7fc16b6ef30966f0fe239cb368f027c674a0b970775a6da7b8631364ece2d336`，纳入文件数为 `613`。

## 阶段补充（2026-07-09 运营日报窗口筛选）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 内置 `/dashboard/saasAdmin/page` 的“运营日报”区块已增加日报日期、统计天数和“刷新日报”控件；页面加载默认填入本地当天日期。
- `dailyReportParams` 现在会把页面选择的 `date` 和 `days` 同时带给 `GET /dashboard/saasAdmin/dailyReport` 和 `GET /dashboard/saasAdmin/export?type=dailyReport`，避免页面只能查看/导出默认当天日报。
- 页面单测新增 `TestSaaSAdminPageIncludesDailyReportWindowControls`，断言日报窗口控件、刷新按钮和 `date/days` 参数绑定存在。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“日报日期”“统计天数”“刷新日报”；日报 JSON 和日报 CSV 请求显式带 `date=$REPORT_DATE&days=1`，并断言 JSON `window` 与 CSV `window` 行回显相同日期和天数。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把页面日报日期窗口和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `e0c3962270f0149cadf7bd675d02e586d6eff84a873620a1630e5c911c393d23`，纳入文件数为 `614`。

## 阶段补充（2026-07-09 运营日报明细面板）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 内置 `/dashboard/saasAdmin/page` 的“运营日报”区块已新增“运营日报明细”面板，复用 `GET /dashboard/saasAdmin/dailyReport` 返回数据，不新增后端接口或迁移表。
- 日报明细面板包含四张表：日报负责人、日报通知、日报操作动作和日报账单流水，分别回显负责人打开任务/逾期/7 天内、失败或耗尽通知、日期窗口内操作动作汇总、日期窗口内账单流水。
- `renderDailyReport` 现在会同步调用 `renderDailyReportDetails`；无权限或加载失败时会清空日报明细并显示“仅平台管理员可查看”。
- 页面单测 `TestSaaSAdminPageIncludesDailyReportWindowControls` 已扩展断言日报明细面板、四个 tbody 和渲染函数绑定存在。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“运营日报明细”“日报负责人”“日报通知”“日报操作动作”和“日报账单流水”。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把日报明细面板和 smoke 页面断言口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `6f9b1d2a3414ba0432840f0300811e309ed5ccb7b539827afb95dd80fce57506`，纳入文件数为 `614`。

## 阶段补充（2026-07-09 套餐变更影响）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 增强平台级套餐维护接口：`POST/PUT /dashboard/saasAdmin/package` 原有套餐 payload 保持兼容，新增 `impact` 影响摘要。
- `impact` 会固定按 26 项 SaaS 额度计算变更，返回 `existing`、`assignedTenantCount`、`changedLimitCount`、升额/降额/新增限额/转不限额数量、变更明细 `changes`，以及降额或新增限额后会超新额度的租户样例 `overLimitTenants`。
- 影响计算会先读取当前套餐目录，再按 `packageCode` 聚合当前分配该套餐的租户；当新额度低于当前用量时，会按租户用量明细识别 `overLimitTenantCount`。套餐目录保存仍不自动改写已分配租户的套餐快照，payload 明确返回 `tenantSnapshotsUpdated=false`，避免平台运营误以为编辑套餐会立即影响存量租户。
- 内置 `/dashboard/saasAdmin/page` 已新增“套餐变更影响”面板，保存套餐后展示分配租户数、变更额度数、降额风险、超新额度租户数和租户套餐快照未自动改写提示，并展示每项额度的原值、新值、变化方向和 delta。
- 页面单测已扩展断言“套餐变更影响”面板、`packageImpactSummary`、`packageImpactChanges` 和 `renderPackageImpact(saved.impact)` 绑定存在。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“套餐变更影响”；平台管理员先把租户分配到 `scale` 套餐，再把套餐目录 `maxUsers` 从 `120` 降到 `5`，断言 `impact.assignedTenantCount=1`、`checkedTenantCount=1`、`decreasedLimitCount=1`、`overLimitTenantCount=1`、超新额度租户当前 users 为 `8/5`，随后把套餐恢复到 `120`，避免影响后续套餐 CSV 断言。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把套餐保存影响摘要、降额超额风险和租户套餐快照不自动改写的验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `a5cc63cf3f0e243fcf92a568bea283947d3885f782bbcb34598d12ccc5f13544`，纳入文件数为 `614`。

## 阶段补充（2026-07-09 套餐快照受控同步）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增平台级套餐快照受控同步接口：`POST/PUT /dashboard/saasAdmin/packageSync`，仅平台租户超级管理员可用，普通租户超级管理员访问返回 `403`。
- 接口支持 `packageCode`、可选 `tenantId`、`limit`、`dryRun`、`allowOverLimit` 和 `remark`；默认 `dryRun=true` 只预览命中租户、超额指标和同步影响，不写入租户套餐快照。
- 实际同步会先用当前套餐目录和租户用量识别超新额度租户；存在超额租户且未显式 `allowOverLimit=true` 时返回 `400` 并带回 `blocked=true`、超额租户样例和 `tenantSnapshotsUpdated=false`，避免运营误把降额直接推给存量客户。
- 显式允许同步后，接口复用既有 `UpdateSaaSAdminTenantPackage` 写路径逐租户刷新 `mochat_go_saas_tenant_packages.limits_json`，并同步刷新 26 项 `mochat_go_saas_usage_counters`，继续产生既有 `tenant.package` 操作日志。
- 内置 `/dashboard/saasAdmin/page` 已新增“套餐快照同步”操作区和结果表，支持选择套餐、指定租户、命中上限、超额策略，以及“预览同步”“应用同步”两个动作；结果会展示命中租户、到期时间、同步状态、刷新指标数和超额指标。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSync`。
- 已扩展页面单测、server 路由单测和 SaaS admin handler 单测，覆盖 dry-run 默认不写入、实际同步遇超额阻断、`allowOverLimit=true` 后写入并刷新 26 项指标、普通租户越权拒绝。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“套餐快照同步”；平台管理员把 `scale.maxUsers` 降到 `5` 后调用 `packageSync` 验证 dry-run 不写入、实际同步默认 `400` 阻断、允许超额后同步租户快照并刷新 26 项指标；随后把套餐恢复到 `120` 并再次同步，避免影响后续套餐 CSV 和用量断言。
- 已更新 `README.md` 和 `docs/release-candidate.md`，把 `POST/PUT /dashboard/saasAdmin/packageSync`、页面套餐快照同步和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `cb16954c56660a55595be9594de1ed4bab9396e7373877518c9c1b2a67511636`，纳入文件数为 `614`。

## 阶段补充（2026-07-09 套餐同步任务化）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 新增迁移 `0035_saas_admin_tasks`，创建 `mochat_go_saas_admin_tasks`，用于记录 SaaS 总后台运营任务的任务类型、状态、租户、套餐、操作人、请求参数、预览结果、执行结果、最近错误和应用时间。
- 新增平台级任务列表接口：`GET /dashboard/saasAdmin/tasks`，仅平台租户超级管理员可用；支持 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选，当前任务类型包含 `package_sync` 和 `tenant_renewal`。
- 新增套餐同步任务创建接口：`POST/PUT /dashboard/saasAdmin/packageSyncTask`。接口会强制先 dry-run 生成预览，不直接写租户快照；若存在超新额度租户且未允许超额，任务状态为 `blocked`，否则为 `pending`。
- 新增套餐同步任务应用接口：`POST/PUT /dashboard/saasAdmin/packageSyncTaskApply`。应用时会从任务 request 重新计算当前套餐与用量风险；若仍超额且未允许超额，会保持/更新为 `blocked`；成功后复用套餐快照同步写路径刷新租户快照和 26 项用量额度，并把任务更新为 `applied`，写入执行结果和 `appliedAt`。
- 内置 `/dashboard/saasAdmin/page` 已在“套餐快照同步”区域新增“套餐同步任务”操作区和任务表，支持创建任务、按任务 ID 应用任务、刷新最近任务，并展示任务状态、套餐、租户、预览/执行摘要和行内应用按钮。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `GET /dashboard/saasAdmin/tasks`、`POST/PUT /dashboard/saasAdmin/packageSyncTask` 和 `POST/PUT /dashboard/saasAdmin/packageSyncTaskApply`。
- 已扩展页面单测、server 路由单测、SaaS admin handler 单测和 MySQL store，实现并覆盖任务创建、blocked 预览、stored request 应用、任务状态更新和任务 payload。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：新增 `0035` 迁移断言、页面“套餐同步任务”断言、普通租户越权创建任务返回 `403`、平台管理员创建 `blocked` 同步任务、恢复套餐后创建 `pending` 同步任务、应用后变为 `applied`，并通过 `GET /dashboard/saasAdmin/tasks` 回看任务。
- 已更新 `README.md`、`docs/release-candidate.md` 和 `deploy/standalone/migrations/README.md`，把套餐同步任务接口、页面入口、任务表迁移和 smoke 验收口径补齐。
- 已执行 `bash -n scripts/smoke_schema_migrate.sh scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `858f4b8364d4c5c14225e065e363c428e6048fcb9bc81987dd10b3be400286aa`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 续费任务化）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮没有启动新的 24 小时持续运行。
- 扩展 SaaS 总后台运营任务中心：`GET /dashboard/saasAdmin/tasks` 的 `taskType` 现在支持 `package_sync` 和 `tenant_renewal`，任务表继续复用 `0035_saas_admin_tasks`，不新增迁移。
- 新增平台级续费任务创建接口：`POST/PUT /dashboard/saasAdmin/tenantRenewalTask`。接口会先解析并固化续费请求里的套餐编码、生成当前租户续费预览，不直接写账单；若新到期时间早于当前到期时间，任务状态为 `blocked`，否则为 `pending`。
- 新增平台级续费任务应用接口：`POST/PUT /dashboard/saasAdmin/tenantRenewalTaskApply`。应用时会从任务 request 重新读取当前租户与套餐状态，成功后复用 `RenewSaaSAdminTenant` 写路径更新租户套餐到期时间、写入续费账单、刷新 26 项用量额度，并把任务更新为 `applied`、写入执行结果和 `appliedAt`。
- 内置 `/dashboard/saasAdmin/page` 已在续费区域新增“续费任务”操作区和任务表，支持创建续费任务、按任务 ID 应用任务、刷新最近续费任务，并展示租户、套餐、任务状态、预览/执行摘要和行内应用按钮。
- 已将新接口接入 standalone server 路由、`cmd/mochat-go` 启用配置和迁移路由清单；启动日志新增 `POST/PUT /dashboard/saasAdmin/tenantRenewalTask` 和 `POST/PUT /dashboard/saasAdmin/tenantRenewalTaskApply`。
- 已扩展页面单测、server 路由单测、SaaS admin handler 单测和 MySQL store 任务类型校验，覆盖续费任务创建、当前套餐固化、stored request 应用、任务状态更新和续费结果 payload。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“续费任务”；普通租户越权创建续费任务返回 `403`；平台管理员创建续费任务后通过 `tenantRenewalTaskApply` 应用，并通过 `GET /dashboard/saasAdmin/tasks?taskType=tenant_renewal` 回看 `applied` 任务，同时校验账单事件和租户到期时间已更新。
- 已更新 `README.md`、`docs/release-candidate.md` 和 `deploy/standalone/migrations/README.md`，把续费任务接口、页面入口、任务表口径和 smoke 验收说明补齐。

## 阶段补充（2026-07-09 平台开户任务化）

- 扩展 SaaS 总后台运营任务中心：`GET /dashboard/saasAdmin/tasks` 的 `taskType` 现在支持 `package_sync`、`tenant_renewal` 和 `tenant_provision`，任务表继续复用 `0035_saas_admin_tasks`，不新增迁移。
- 新增平台级开户链接任务创建接口：`POST/PUT /dashboard/saasAdmin/tenantProvisionTask`。接口会先解析平台开户请求，立即把初始密码转换为 `adminPasswordHash` 后写入任务 request，生成套餐与开户链接预览；任务响应和任务列表只暴露 `hasAdminPasswordHash` 标记，不回显明文密码或哈希。
- 新增平台级开户链接任务应用接口：`POST/PUT /dashboard/saasAdmin/tenantProvisionTaskApply`。应用时从任务 request 读取已固化的密码哈希，复核套餐仍启用后复用 `ProvisionSaaSAdminTenant` 写路径创建租户、初始超级管理员、角色授权、套餐快照、开通记录和 26 项用量计数，并把任务更新为 `applied`、写入执行结果和 `appliedAt`。
- 内置 `/dashboard/saasAdmin/page` 已在“新租户开户”下方新增“平台开户任务”操作区和任务表，支持创建开户链接任务、按任务 ID 应用任务、刷新最近任务，并展示租户、管理员、套餐、任务状态和预览/执行摘要。
- 已扩展页面单测、server 路由单测、SaaS admin handler 单测和 MySQL store 任务类型校验，覆盖开户链接任务创建、密码哈希固化、响应脱敏、stored request 应用、任务状态更新和开户结果 payload。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“平台开户任务”；普通租户越权创建/应用开户链接任务返回 `403`；平台管理员创建 `pending` 开户任务后通过 `tenantProvisionTaskApply` 应用，并通过 `GET /dashboard/saasAdmin/tasks?taskType=tenant_provision` 回看 `applied` 任务，同时校验任务响应不含明文密码或哈希、新租户/管理员/开通记录已写入且管理员可登录。
- 已更新 `README.md`、`docs/release-candidate.md` 和 `deploy/standalone/migrations/README.md`，把平台开户任务接口、页面入口、任务表口径和 smoke 验收说明补齐。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）、`./scripts/list_standalone_soak_processes.sh`、`git diff --check` 均通过；源码指纹 `911687a81f4f542ccb0740a0e1033fa8c3750f7433908692554f66ae007400e4`，`file_count=616`。
- 未跑完整容器 smoke：本机 Docker daemon 未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。本阶段未继续执行 24 小时 run。
- 已执行 `bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh` 通过；已执行 `env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store` 通过；已执行 `env -u GOROOT go test ./...` 通过；已执行 `env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 通过，结果为 `smoke_scripts=73 referenced=73`；已执行 `git diff --check` 通过；已执行 `./scripts/list_standalone_soak_processes.sh`，未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info` 失败信息为 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `40ed4394019a50f532321c2a42965448cb5226b06b271cf47c135f4a00e87955`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 统一运营任务中心）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不再做 24 小时持续运行。
- 内置 `/dashboard/saasAdmin/page` 新增“运营任务中心”统一面板，可按任务类型、状态、租户 ID 和套餐筛选 `package_sync`、`tenant_provision`、`tenant_renewal` 三类 SaaS 总后台运营任务。
- 统一任务中心复用现有 `GET /dashboard/saasAdmin/tasks` 和三类任务应用接口，不新增数据表或迁移；行内“应用”按钮会根据任务类型路由到 `packageSyncTaskApply`、`tenantProvisionTaskApply` 或 `tenantRenewalTaskApply`。
- 页面仍保留套餐同步任务、平台开户任务和续费任务的专属操作区，统一面板作为跨类型运营队列，便于平台管理员从一个列表查看 `pending/blocked/failed/applied` 状态和执行摘要。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“运营任务中心”；三类任务应用后调用 `GET /dashboard/saasAdmin/tasks?taskType=all&status=applied&limit=50`，断言统一任务列表覆盖套餐同步、平台开户和租户续费三类 `applied` 任务，且平台开户任务仍不回显明文密码或 `adminPasswordHash`。
- 已更新页面单测，覆盖统一任务中心筛选控件、任务表、跨类型渲染函数和行内应用入口。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `bec46d96aab2ab3d57709dc51e70338f328f7d9bdb0609b64342a89dec5c4716`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 运营任务导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=tasks` 现在可导出统一运营任务中心任务 CSV，也兼容 `type=task/admin_task/admin_tasks`。
- 任务 CSV 复用 `GET /dashboard/saasAdmin/tasks` 的筛选口径：`taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit`，默认 1000 条，上限 5000 条。
- CSV 字段包含任务 ID、任务类型、状态、租户、套餐、操作人、备注、最近错误、应用时间、创建/更新时间、脱敏后的请求、预览和执行结果；平台开户任务请求导出时继续去除明文密码和 `adminPasswordHash`，只保留 `hasAdminPasswordHash` 标记。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域已增加“导出任务 CSV”按钮，使用运营任务中心当前筛选条件下载 `mochat-saas-tasks-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含任务导出入口；三类任务应用后调用 `GET /dashboard/saasAdmin/export?type=tasks&taskType=all&status=applied&limit=1000`，断言 CSV 覆盖套餐同步、平台开户和租户续费三类 `applied` 任务，且不泄漏明文密码或 `adminPasswordHash`。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）、`git diff --check` 均通过；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `a4db2208500bad9f275ffcc34a6de9f9e5f4616d5e93ac4860d55a632bc8dce9`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 运营任务取消）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 新增 SaaS 总后台运营任务取消接口：`POST/PUT /dashboard/saasAdmin/taskCancel`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 任务状态新增 `canceled`，任务列表、状态筛选、CSV 导出和页面状态文案均支持该状态；`pending/blocked/failed` 任务可取消，已应用任务不可取消，已取消任务不可再次应用。
- 取消任务复用 `0035_saas_admin_tasks` 和既有 `UpdateSaaSAdminTaskStatus` 写路径，不新增迁移；取消结果写入任务 `result_json`，包含取消标记、取消时间、操作人和备注。
- 内置 `/dashboard/saasAdmin/page` 的统一运营任务中心已新增行内“取消”按钮，取消后刷新统一任务中心和三类专属任务表。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `taskCancel`；普通租户越权取消返回 `403`；平台管理员取消一个 blocked 套餐同步任务后，`GET /dashboard/saasAdmin/tasks?status=canceled` 可回看 `canceled` 状态，随后再次应用该任务返回 `400 task canceled`。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）、`git diff --check` 均通过；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `5efdf3aed0b0e3a9a2e9a8e6b68ecf789606a3ae7deb2f72017e5e51e4448646`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 运营任务批量取消）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 新增 SaaS 总后台运营任务批量取消接口：`POST/PUT /dashboard/saasAdmin/taskBulkCancel`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 批量取消复用 `0035_saas_admin_tasks` 和既有 `SaaSAdminTasks`、`UpdateSaaSAdminTaskStatus` 写路径，不新增迁移；接口支持 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选，默认最多 50 条，页面最多提交 100 条。
- 接口只会把 `pending/blocked/failed` 更新为 `canceled`，自动跳过 `applied/canceled`，返回 `matchedCount`、`canceledCount`、`skippedAppliedCount`、`skippedCanceledCount`、筛选条件和已取消任务列表；每条取消结果写入 `result_json`，标记 `bulkCancel=true`、操作人、操作租户、取消时间、备注和筛选条件。
- 内置 `/dashboard/saasAdmin/page` 的统一运营任务中心已新增“批量取消”按钮，按当前任务类型、状态、租户和套餐筛选提交批量取消；成功后刷新统一任务中心和三类专属任务表。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `taskBulkCancel`；普通租户越权批量取消返回 `403`；平台管理员创建 pending 套餐同步任务后，按 `taskType=package_sync`、`packageCode=scale` 批量取消，断言 pending 任务变为 `canceled`，并且已应用和已取消任务会被跳过计数。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/server`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）、`git diff --check` 均通过；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `a0960ea2a917b02c1e36b74b7c8fe0e5c2e7bb019752ff603f9c151ee2b60e92`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 运营任务审计）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- SaaS 总后台运营任务新增统一审计日志：任务创建、应用、阻断、单条取消和批量取消都会写入 `mochat_go_saas_admin_operation_logs`，目标类型统一为 `admin_task`。
- 新增任务审计 action：`saas.admin.task.create`、`saas.admin.task.apply`、`saas.admin.task.block`、`saas.admin.task.cancel`、`saas.admin.task.bulk_cancel`；操作记录会保存任务变更前后 JSON、任务 ID、任务类型、操作者、备注和批量取消筛选条件。
- `POST/PUT /dashboard/saasAdmin/taskCancel` 和 `POST/PUT /dashboard/saasAdmin/taskBulkCancel` 的 smoke 已增加操作记录反查：分别用 `operations?action=saas.admin.task.cancel&targetType=admin_task` 和 `operations?action=saas.admin.task.bulk_cancel&targetType=admin_task` 校验取消动作可追溯。
- 已扩展 SaaS admin handler 单测，覆盖任务创建、应用、单条取消和批量取消写入 `admin_task` 操作记录；MySQL store 新增 `RecordSaaSAdminOperationLog`，复用现有操作日志表和缺表兼容逻辑。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `63de7bedb9b3d383fa6ea8e22f51d693acc9ee55106b390f4a68bb6027eeee75`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 运营任务追溯）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- SaaS 总后台页面的统一运营任务中心新增行内“追溯”按钮，平台管理员可从任务列表直接切到操作记录筛选，按 `targetType=admin_task` 和当前任务 ID 查看任务创建、阻断、应用、取消和批量取消记录。
- 该能力复用既有 `GET /dashboard/saasAdmin/operations`，不新增后端接口或迁移；页面会自动清空 action 筛选、设置目标类型为 `admin_task`、关键字为任务 ID 并刷新操作记录表。
- 已更新页面单测和 `scripts/smoke_saas_admin_dashboard.sh` 静态页面断言，覆盖 `traceAdminTaskOperations`、`data-action="trace-admin-task"` 和任务追溯入口。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `86f3a2f66e4a4056039ceca5ded04cf8bd2b38f48df7dbce19d43639eca4910e`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 操作记录详情）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- SaaS 总后台操作记录表新增“变更”列，可展开查看 before/after JSON 变更详情；无变更 JSON 时会显示空态，不影响原有操作记录筛选和 CSV 导出。
- 该能力复用 `GET /dashboard/saasAdmin/operations` 已返回的 `before` 和 `after` payload，不新增后端接口、数据库迁移或新权限点。
- 已更新页面单测和 `scripts/smoke_saas_admin_dashboard.sh` 静态页面断言，覆盖 `operationChangeDetails`、`查看变更` 和操作记录表“变更”列。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `b4f63f243889a25574871d24287354a5931f622c7a6ab404859300b470e5cb7f`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 操作记录审计验收）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 增强 `scripts/smoke_saas_admin_dashboard.sh` 的操作记录审计断言：单条取消任务会校验 `saas.admin.task.cancel` 操作记录里的 `before.status=blocked`、`before.canCancel=true`、`after.status=canceled`、`after.canApply=false` 和 `after.canCancel=false`。
- 增强批量取消任务的审计断言：`saas.admin.task.bulk_cancel` 操作记录会校验 `before.status=pending`、`before.canCancel=true`、`after.status=canceled`、`after.canApply=false`、`after.canCancel=false` 和 `after.bulkCancel=true`。
- 该能力只补强验收覆盖，复用既有 `admin_task` 操作记录和 before/after payload，不新增后端接口、数据库迁移或新权限点。
- 已更新 `README.md` 和 `docs/release-candidate.md`，明确 smoke 会校验取消审计记录的前后状态。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `9089369bda6fd9c4ac43dcbd4265ad2e9e8402805a8a496acfa15eb39eb2f160`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 运营任务重置）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 新增 SaaS 总后台运营任务重置接口：`POST/PUT /dashboard/saasAdmin/taskReset`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- `taskReset` 仅允许把 `failed/blocked` 任务重置为 `pending`，会清空任务 `result_json` 和 `last_error`，让任务重新进入可应用状态；`pending/applied/canceled` 任务会返回 `400`，避免误重置已完成或已取消任务。
- 重置动作复用既有 `0035_saas_admin_tasks` 和 `UpdateSaaSAdminTaskStatus` 写路径，不新增迁移；同时追加 `saas.admin.task.reset` 的 `admin_task` 操作记录，`before` 保存失败/阻断状态和错误，`after` 保存重置后的 `pending` 状态、`reset=true`、`resetAt` 和 `previousStatus`。
- 内置 `/dashboard/saasAdmin/page` 的统一运营任务中心已对 `blocked/failed` 任务新增行内“重置”按钮，成功后刷新统一任务中心、三类专属任务表和操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `taskReset` 和 `reset-admin-task`；普通租户越权重置返回 `403`；平台管理员创建 blocked 套餐同步任务后调用 `taskReset`，断言任务回到 `pending`、`lastError` 清空、`canApply/canCancel` 为 true，并通过 `operations?action=saas.admin.task.reset&targetType=admin_task` 校验 before/after 审计。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API、页面、smoke 和操作记录口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `8387fd43a4446b9853c654eb874aaa344668e7eb02c492bc11d8ddfac45d7fe4`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 运营任务批量重置）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 新增 SaaS 总后台运营任务批量重置接口：`POST/PUT /dashboard/saasAdmin/taskBulkReset`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- `taskBulkReset` 复用统一运营任务筛选条件：`taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit`；接口只会把 `failed/blocked` 任务重置为 `pending`，清空 `result_json` 和 `last_error`，并跳过 `pending/applied/canceled` 任务。
- 批量重置会为每个实际重置的任务追加 `saas.admin.task.bulk_reset` 的 `admin_task` 操作记录，`before` 保存阻断或失败状态，`after` 保存 `pending`、`reset=true`、`bulkReset=true`、`resetAt`、`previousStatus` 和本次筛选条件。
- 内置 `/dashboard/saasAdmin/page` 的统一运营任务中心新增“批量重置”按钮，按当前任务筛选最多处理 100 条阻断或失败任务，成功后刷新统一任务中心、套餐同步任务、平台开户任务、续费任务和操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `taskBulkReset` 和 `bulkResetAdminTasks`；普通租户越权批量重置返回 `403`；平台管理员创建 blocked 套餐同步任务后调用 `taskBulkReset`，断言任务回到 `pending`、`lastError` 清空、`canApply/canCancel` 为 true，并通过 `operations?action=saas.admin.task.bulk_reset&targetType=admin_task` 校验 before/after 审计；同时断言批量重置会跳过待应用、已应用和已取消任务。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API、页面、smoke 和操作记录口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `f00092d81983bb13ed0c09a63142ea9ec19566305e7f34ba7987fb084112d36e`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 账单筛选汇总）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- SaaS 总后台账单事件接口 `GET /dashboard/saasAdmin/billingEvents` 的 `summary` 改为独立聚合查询，包含当前筛选全量命中的 `eventCount`、`renewalCount` 和 `amountCents`，方便平台管理员按租户、套餐、订单号或支付方式过滤后直接查看不受列表 `limit` 影响的账单笔数和金额合计。
- 内置 `/dashboard/saasAdmin/page` 的账单事件区已把 `summary` 展示在标题旁，格式为“账单 N 条 / 续费 M 条 / 合计 X CNY”，当命中数大于当前列表数量时追加“显示 M 条”；空结果仍显示 0 汇总，接口失败时保持原有权限空态。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：续费后读取账单事件列表，断言 `summary.eventCount`、`summary.renewalCount`、`summary.amountCents` 和 `returnedCount`；过滤到单个续费订单时，断言金额合计等于该订单金额。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐账单事件 API、页面、smoke 和账单筛选汇总口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，但当前 `mochat-go` 目录在父 Git 仓库中未跟踪，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `ce04ba5a784a99c6fc62c98203af22ebaff8001e0df734460840c83fce30f01a`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 操作记录筛选汇总）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- SaaS 总后台操作记录接口 `GET /dashboard/saasAdmin/operations` 新增 `summary` 和 `returnedCount`：`summary` 通过独立聚合查询返回当前筛选全量命中的 `operationCount`、`tenantCount`、`actorUserCount`、`actionCount` 和 `targetTypeCount`，列表仍按 `limit` 截断。
- MySQL store 已把操作记录筛选条件抽为 `saasAdminOperationLogWhere`，列表查询、汇总查询和导出继续复用同一组 `tenantId`、`action`、`targetType`、`keyword` 语义；缺少 SaaS 操作日志表时保持返回空结果兼容。
- 内置 `/dashboard/saasAdmin/page` 的操作记录标题已展示当前筛选全量命中规模，格式为“操作 N 条 / 租户 M 个 / 动作 K 类 / 操作人 U 个”，当命中数大于当前列表数量时追加“显示 L 条”；空结果仍显示 0 汇总。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：读取操作记录和过滤后的操作记录时，断言 `summary.operationCount`、`summary.tenantCount`、`summary.actorUserCount`、`summary.actionCount`、`summary.targetTypeCount` 和 `returnedCount`，确保操作日志审计规模不受列表 `limit` 影响。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐操作记录 API、页面、smoke 和筛选汇总口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 本机 Docker daemon 当前仍未启动，`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，所以本轮未运行依赖 `docker compose` 的 `scripts/smoke_saas_admin_dashboard.sh` 实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `27061750b48507539c4c849acb01e9ad1701e67ba73a6784be2e6f6d74ef4e10`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 账单对账）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/billingReconciliation`，复用账单事件筛选条件 `tenantId`、`eventType`、`packageCode`、`keyword` 和 `limit`，并支持 `mismatchOnly=1` 只看异常。
- 对账逻辑会把 `mochat_go_saas_billing_events` 中的续费账单和当前 `mochat_go_saas_tenant_packages` 比对：当前套餐存在、启用、套餐编码一致，并且当前到期时间覆盖账单新到期时间时返回 `matched`；否则返回 `missing_package`、`inactive_package`、`package_mismatch` 或 `expires_mismatch`。
- 该能力不新增数据库迁移；MySQL store 使用同一组账单筛选条件做 summary 和列表查询，缺少 SaaS 账单或租户套餐表时保持空结果兼容。
- 内置 `/dashboard/saasAdmin/page` 新增“账单对账”表格和“只看对账异常”开关，展示已检查、正常、异常和当前列表数量，并把异常原因翻译为中文。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：续费后调用 `GET /dashboard/saasAdmin/billingReconciliation` 校验正常账单为 `matched`；再插入一条模拟套餐和到期漂移的账单，校验 `mismatchOnly=1` 返回 `package_mismatch` 和 `expires_mismatch`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API、页面、smoke 和验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `58c27f6894ff9ac3ed3595060278b2a00388a133c77894a651d456822a4a7e52`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 账单对账 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮不启动 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=billingReconciliation` 现在可导出账单对账结果，也兼容 `type=billing_reconciliation/billing_reconcile/reconciliation/reconcile`。
- 对账 CSV 复用 `tenantId`、`eventType`、`packageCode`、`keyword`、`mismatchOnly` 和 `limit` 筛选；字段包含账单 ID、租户、账单套餐、到期变化、金额、订单号、当前套餐、当前到期、对账状态、异常原因、操作人、备注和 metadata。
- 内置 `/dashboard/saasAdmin/page` 的数据导出栏新增“导出对账 CSV”，会带上当前账单筛选条件和“只看对账异常”开关，便于平台财务或客户成功直接下载异常清单。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出对账 CSV”；后段调用 `GET /dashboard/saasAdmin/export?type=billingReconciliation&...&mismatchOnly=1`，断言模拟漂移账单导出行包含 `growth` 账单套餐、当前 `scale` 套餐、`mismatch` 状态以及 `package_mismatch/expires_mismatch` 原因。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐对账 CSV 的 API、页面和 smoke 口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `e00da90fca8a964adeeaa4d32452811c7cad8e006c01df0087f0c6a3a6ca7f07`，纳入文件数为 `616`。

## 阶段补充（2026-07-09 账单对账异常跟进）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不再启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUp`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 跟进入口接收 `billingEventId`、`status`、`owner`、`nextFollowUpAt` 和 `remark`，会先读取原始 `mochat_go_saas_billing_events` 账单事件，再追加 `billing.reconciliation.follow_up` / `billing_event` 操作日志；接口不改写账单事件或当前租户套餐事实。
- 操作日志 `before` 保存原始账单事件 payload，`after` 保存账单事件 ID、租户 ID、跟进状态、负责人、下次跟进时间、备注、订单号、套餐编码、操作人和跟进时间，便于平台财务或客户成功从操作记录回看异常处理过程。
- 内置 `/dashboard/saasAdmin/page` 的“账单对账”表格新增操作列：正常账单显示“无需处理”，异常账单显示“跟进”按钮；点击后录入备注，写入跟进日志，并刷新账单对账、操作记录和运营日报。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `followUpBillingReconciliation` 和 `billingReconciliationFollowUp`；模拟漂移账单后读取真实 `billingEventId`，调用跟进入口，断言响应中的状态、负责人、下次跟进时间、备注和账单事件；随后通过操作记录断言 `billing.reconciliation.follow_up` 的 `targetType=billing_event`、`before.externalOrderNo` 和 `after.status/owner/billingEventId`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API、页面、smoke 和只追加操作日志、不改写事实数据的口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `a8791b7b03a1919b4698719cd75275c5d6cc8aa39e72600432a0f90f7088f97f`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 账单跟进任务与负责人工作台）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不再启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/billingReconciliationFollowUps`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 新增平台级负责人聚合接口 `GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 两个接口不新增迁移表，复用 `mochat_go_saas_admin_operation_logs` 中 `action=billing.reconciliation.follow_up`、`target_type=billing_event` 的最新日志派生任务；支持 `tenantId`、`status`、`owner`、`dueState=all/overdue/due_soon/future/no_date/closed`、`keyword` 和 `limit` 筛选。
- 任务 payload 回显 `billingEventId`、租户、状态、负责人、下次跟进、备注、套餐、金额、外部订单号、操作 ID、到期状态、逾期标记和 `daysUntil`；负责人工作台汇总打开任务、关闭任务、状态分布、逾期/7 天内/未来/无日期/已关闭分布、最近记录时间和最早下次跟进时间。
- 内置 `/dashboard/saasAdmin/page` 新增“账单跟进任务筛选”、“账单跟进任务”和“账单跟进负责人工作台”；记录账单对账跟进后会刷新账单对账、账单跟进任务、负责人工作台、操作记录和运营日报。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“账单跟进任务”和“账单跟进负责人工作台”；普通租户越权访问两个 GET 接口返回 `403`；模拟漂移账单后记录异常跟进，再调用任务列表和负责人工作台断言 `resolved/closed` 状态、订单号、金额、操作 ID、负责人和关闭分布。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API、页面和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `e9beedc398ec643aa770bfd389aa38f3a23adadaf885f98ec4decaa35f6afeac`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 账单跟进批量关闭）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不再启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 批量关闭接口复用账单跟进任务筛选条件：`tenantId`、`filterStatus`、`owner`、`dueState=all/overdue/due_soon/future/no_date/closed`、`keyword` 和 `limit`；`closeStatus` 仅支持 `resolved/ignored`。接口会跳过已关闭任务，并为每个命中的未关闭账单事件追加新的 `billing.reconciliation.follow_up` / `billing_event` 操作日志，不改写账单或套餐事实。
- `BillingReconciliationFollowUp` 的单条写入已抽出 `recordBillingReconciliationFollowUpOperation`，批量关闭和单条跟进共用同一套 before/after JSON 审计结构，确保操作记录里继续保留账单事件、状态、负责人、备注、操作人和平台租户信息。
- 内置 `/dashboard/saasAdmin/page` 的账单跟进任务筛选栏新增“批量关闭账单跟进”按钮，会按当前任务筛选调用批量关闭接口；成功后刷新账单跟进任务、负责人工作台、操作记录和运营日报。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“批量关闭账单跟进”、`bulkCloseBillingFollowUps` 和 `billingReconciliationFollowUpBulkClose`；普通租户越权调用批量关闭返回 `403`；平台管理员会先把模拟漂移账单重新打开为 `contacted`，再按状态、负责人、未来到期状态和备注关键字批量关闭，随后通过任务列表、操作记录和启动日志断言关闭结果与审计记录。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API、页面、smoke 和只追加操作日志、不改写事实数据的口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `e0ceb35f1c2aa2eb478df43b05a0168ce39c4c1a52d3175be7aee0d884e51fd4`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 账单跟进任务 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不再启动 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUps` 现在可导出账单对账跟进任务，也兼容 `type=billing_reconciliation_follow_ups/billingFollowUps/billing_follow_ups`。
- 导出复用账单跟进任务筛选口径：`tenantId`、`status`、`owner`、`dueState=all/overdue/due_soon/future/no_date/closed`、`keyword` 和 `limit`；平台租户超级管理员可导出，普通租户超级管理员仍返回 `403`。
- 账单跟进 CSV 输出 `operationId`、`billingEventId`、`tenantId`、`tenantName`、`status`、`owner`、`nextFollowUpAt`、`dueState`、`overdue`、`daysUntil`、`remark`、`packageCode`、`packageName`、`newExpiresAt`、`amountCents`、`currency`、`externalOrderNo` 和 `createdAt`，方便平台财务或客户成功按负责人、关闭状态和订单号线下核对。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域新增“导出账单跟进 CSV”按钮，会带上当前账单跟进任务筛选条件下载 `mochat-saas-billingReconciliationFollowUps-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出账单跟进 CSV”；普通租户越权导出返回 `403`；平台管理员在账单跟进批量关闭后调用 `GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUps`，断言表头、租户、状态、负责人、关闭状态、备注、账单套餐、金额和订单号。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐导出类型、页面按钮、CSV 字段和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `0196b55b928aabe0137ded7efd806ca04e15a1ce8433956b87b3d67a71a5c7de`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 负责人工作台 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=riskFollowUpOwners` 现在可导出风险跟进负责人工作台，也兼容 `risk_follow_up_owners/riskOwners/risk_owners` 等别名。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners` 现在可导出账单跟进负责人工作台，也兼容 `billing_reconciliation_follow_up_owners/billingOwners/billing_owners` 等别名。
- 两类负责人 CSV 均复用对应任务列表筛选条件：`tenantId`、`status`、`owner`、`dueState=all/overdue/due_soon/future/no_date/closed`、`keyword` 和 `limit`；导出时强制读取完整到期状态并套用导出上限，普通租户超级管理员访问仍返回 `403`。
- CSV 字段统一为 `owner,totalCount,openCount,pendingCount,contactedCount,renewalPendingCount,resolvedCount,ignoredCount,overdueCount,dueSoonCount,futureCount,noDateCount,closedCount,latestFollowUpAt,nextFollowUpAt`，便于平台运营、客户成功或财务按负责人线下核对任务量、状态分布和到期分布。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域新增“导出风险负责人 CSV”和“导出账单负责人 CSV”按钮，分别带上当前风险跟进任务筛选和账单跟进任务筛选下载负责人汇总。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含两个新导出按钮；普通租户越权导出两类负责人 CSV 返回 `403`；平台管理员会在风险跟进和账单跟进闭环中分别导出负责人 CSV，并断言表头、负责人、打开/关闭数量、状态分布、到期分布、最近跟进和下次跟进时间。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐导出类型、页面按钮、CSV 字段和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `5235eb191b303d844bca210b402094be146c523ff554c2779d7f66913b49ebb3`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 运营任务中心汇总）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 增强 SaaS 总后台运营任务接口：`GET /dashboard/saasAdmin/tasks` 现在返回 `summary` 和 `returnedCount`，列表继续按 `limit` 截断，汇总不受列表上限影响。
- 任务汇总复用现有 `taskId`、`taskType`、`status`、`tenantId` 和 `packageCode` 筛选条件；MySQL store 抽出统一 where 条件，列表查询和汇总查询保持同一口径，不新增数据库迁移。
- `summary` 输出 `taskCount`、`pendingCount`、`blockedCount`、`failedCount`、`appliedCount`、`canceledCount`、`actionableCount`、`packageSyncCount`、`tenantRenewalCount`、`tenantProvisionCount`、`tenantCount` 和 `actorUserCount`，方便平台运营判断任务是否卡在阻断、失败或待应用状态。
- 内置 `/dashboard/saasAdmin/page` 的“运营任务中心”标题已改为展示全量任务汇总：任务总数、待应用、阻断、失败、已应用、可处理数量，以及被 `limit` 截断时的显示条数。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：三类运营任务应用后读取 `GET /dashboard/saasAdmin/tasks?taskType=all&status=applied&limit=50`，断言 `summary`、`returnedCount`、状态分布、任务类型分布、命中租户数和操作人数；同时继续校验平台开户任务不泄漏明文密码或 `adminPasswordHash`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API 返回、页面汇总和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；`docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `01e95d80e439282d17f6853c8eb6683ffe2d25c66a7583986186b83cd601cba5`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 告警与通知筛选汇总）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 增强 SaaS 总后台告警接口：`GET /dashboard/saasAdmin/alerts` 现在返回 `summary` 和 `returnedCount`，列表继续按 `page/perPage` 分页，汇总不受分页影响。
- 告警汇总复用现有 `tenantId`、`status`、`metric` 和 `alertType` 筛选条件；MySQL store 通过独立聚合查询返回 `alertCount`、`openCount`、`resolvedCount`、`warningCount`、`criticalCount`、`metricCount` 和 `tenantCount`，不新增数据库迁移。
- 增强 SaaS 总后台通知接口：`GET /dashboard/saasAdmin/notifications` 现在返回 `summary` 和 `returnedCount`，列表继续按 `limit` 截断，汇总不受列表上限影响。
- 通知汇总抽出统一 where 条件，列表查询和汇总查询保持同一口径；`summary` 输出 `notificationCount`、`pendingCount`、`failedCount`、`deliveredCount`、`deadCount`、`retryableCount`、`tenantCount` 和 `channelCount`。
- 内置 `/dashboard/saasAdmin/page` 的“告警处置”和“通知重试”标题已改为展示筛选命中总量、当前显示条数和关键状态分布，避免运营人员只看到当前页数量。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：平台管理员和普通租户管理员读取告警/通知时断言 `summary`、`returnedCount`、状态分布、命中租户数、指标数和通道数，同时继续校验普通租户越权访问平台告警/通知返回 `403`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API 返回、页面汇总和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；route coverage 返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，`docker info --format '{{.ServerVersion}}'` 返回同类 daemon 连接失败。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `9792e7d6f1db1ccf72ac28e41d85f8f9bc452b63bf16abb932a8fcfd9eb6e29b`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 告警与通知 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=alerts` 现在可导出告警列表，也兼容 `type=alert/saasAlert/saas_alerts` 等别名。
- 告警导出复用现有 `tenantId`、`status=open/resolved/all`、`metric`、`alertType` 和 `limit` 筛选条件；CSV 输出 `alertKey`、租户、告警类型、严重级别、状态、指标、指标中文名、周期、当前用量、额度、追加量、触发次数、来源、消息、上下文和时间字段。
- 增强 SaaS 总后台 CSV 导出：`GET /dashboard/saasAdmin/export?type=notifications` 现在可导出告警通知 outbox，也兼容 `type=notification/alertNotification/alert_notifications` 等别名。
- 通知导出复用现有 `tenantId`、`status=pending/failed/delivered/dead/all`、`channel`、`keyword` 和 `limit` 筛选条件；CSV 输出通知 key、告警 key、租户、通道、状态、重试次数、指标、指标中文名、告警类型、周期、用量、失败原因、下次重试、送达和创建/更新时间。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域已增加“导出告警 CSV”和“导出通知 CSV”按钮，分别带上当前告警筛选和通知筛选下载 CSV。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：平台管理员按当前告警/通知筛选导出 CSV，并断言表头、租户、状态、指标、通道和失败原因；普通租户越权导出告警/通知 CSV 返回 `403`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐导出类型、页面按钮、CSV 字段和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `env -u GOROOT ./scripts/standalone_route_coverage.sh` 和依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 当前仍被本机 Docker daemon 阻断；route coverage 返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，`docker info --format '{{.ServerVersion}}'` 返回同类 daemon 连接失败。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `cff8b2344d794198e3fb5233a3e6dbd532c8f087351ede65683e8bcace22249b`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 租户生命周期审计）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/tenantLifecycle`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`，缺少 `tenantId` 返回 `400`，平台管理租户自身不需要生命周期审计。
- 生命周期接口按 `tenantId` 和 `limit` 聚合单租户详情、操作记录、账单事件、运营任务、告警和通知 outbox，并生成统一 `timeline`。时间线事件按时间倒序输出，`source` 覆盖 `operation/billing/task/alert/notification`，同时保留各来源原始 payload，便于平台运营或客户成功从一个入口追溯租户从开户、套餐、续费、任务到告警通知的全过程。
- 内置 `/dashboard/saasAdmin/page` 的“租户详情”区域新增“生命周期审计”表格，平台管理员打开租户详情后自动加载，页面也提供“刷新审计”按钮；普通租户视角显示“仅平台管理员可查看”。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“生命周期审计”和 `tenantLifecycle`；普通租户越权访问生命周期接口返回 `403`；三类运营任务和账单事件产生后，平台管理员调用 `GET /dashboard/saasAdmin/tenantLifecycle?tenantId=$TENANT_ID&limit=50`，断言 `summary`、`returnedEventCount`、目标租户归属和五类 timeline 来源，并在启动日志中确认路由启用。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐环境变量说明、API 表、页面能力和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `da0b2acd1b89cea15c5cd26bc7857825609737125d29c96a6462b6f42566f94f`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 客户成功健康队列）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/customerSuccess`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 客户成功队列不新增数据库表，复用现有租户总览、用量、风险看板、风险跟进操作日志、账单对账跟进操作日志、运营任务和告警通知 outbox，按租户聚合为待处理队列。
- 接口支持 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority=critical/high/medium/normal/all`；返回 `summary`、队列项优先级、健康分、负责人、到期状态、原因、下一步动作、风险详情、风险跟进、账单跟进摘要、运营任务摘要和失败/耗尽通知数量。
- 内置 `/dashboard/saasAdmin/page` 新增“客户成功队列”区块，会在总览刷新时读取 `customerSuccess`，展示优先级、健康分、租户、负责人、待处理信号和下一步动作，并可一键下钻租户详情与生命周期审计。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“客户成功队列”和 `customerSuccess`；普通租户越权访问客户成功队列返回 `403`；平台管理员在初始风险、打开告警和失败/耗尽通知存在时读取 `GET /dashboard/saasAdmin/customerSuccess`，断言队列汇总、目标租户、优先级、健康分、风险信息、原因、下一步动作和失败通知数量，并在启动日志中确认路由启用。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐环境变量说明、API 表、页面能力和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the Docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the Docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `f74308c7fbc4de18cfa37b57c2882824c8c791c3a9579428de9ef78713a734ce`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 客户成功筛选与 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/export` 新增 `type=customerSuccess`，也兼容 `customer_success`、`customer_success_queue` 和 `cs`；仅平台租户超级管理员可用，普通租户超级管理员访问仍返回 `403`。
- 客户成功 CSV 复用 `GET /dashboard/saasAdmin/customerSuccess` 的 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority=critical/high/medium/normal/all` 筛选，输出租户、优先级、健康分、负责人、到期状态、待处理原因、下一步动作、风险等级、风险分、套餐、到期、风险跟进、账单跟进数、可处理任务数、失败/耗尽通知数和 Top 用量指标。
- 内置 `/dashboard/saasAdmin/page` 的“客户成功队列”新增优先级、负责人和租户上限筛选，并在“数据导出”区域新增“导出客户成功 CSV”按钮，导出时带上当前客户成功筛选条件。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含客户成功筛选和导出控件；平台管理员导出 `GET /dashboard/saasAdmin/export?type=customerSuccess` 并断言 CSV 表头、目标租户、优先级、健康分、原因、下一步动作、风险和 Top 用量指标；普通租户导出客户成功 CSV 返回 `403`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐客户成功队列筛选、导出参数、CSV 字段和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0，另用 `rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `eb5f8ac6e7f3ae6d311fc9baf58314c14df5f5378b5fcb43c3550796159dd8a9`，纳入文件数为 `616`。

## 阶段补充（2026-07-10 客户成功批量分派）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/customerSuccessAssign`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 分派接口复用客户成功队列筛选条件 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority`；请求体支持 `owner/assignOwner/assignee`、`status=pending/contacted/renewal_pending`、`nextFollowUpAt` 和 `remark`，命中的队列租户会逐条写入现有风险跟进操作日志，不新增数据库表。
- 内置 `/dashboard/saasAdmin/page` 的“客户成功队列”新增“客户成功批量分派”控件，平台管理员可按当前队列筛选分派负责人、下次跟进时间和备注；分派后刷新客户成功队列、风险跟进任务和负责人工作台。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“客户成功批量分派”和 `assignCustomerSuccess`；普通租户调用分派接口返回 `403`；平台管理员调用 `customerSuccessAssign` 后断言目标租户负责人、状态、下次跟进时间、备注和操作 ID。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐接口参数、页面入口和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 当前 shell 的 `GOROOT` 指向不存在的 `/Users/lv/go-local/go`，本轮 Go 命令均使用 `env -u GOROOT` 执行；实际 Go 版本为 `go1.26.4 darwin/arm64`。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `1476f1d1a3e84ea857f0dffce270524bf95d59fad03cfff81a79953f56a59b03`，纳入文件数为 `616`，生成时间为 `2026-07-10 02:39:55 CST`。

## 阶段补充（2026-07-10 客户成功续费任务）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/customerSuccessRenewalTasks`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 续费任务接口复用客户成功队列筛选条件 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority`；请求体支持 `packageCode`、固定 `expiresAt` 或 `months/renewMonths` 顺延、`amount/amountCents`、`currency`、`paidAt`、`paymentMethod`、`externalOrderNoPrefix`、`remark` 和 `forceCreate`。
- 默认策略是沿用租户当前套餐、按当前到期日顺延 12 个月，并跳过已有待处理续费任务的租户；命中的队列租户会生成现有 `tenant_renewal` 运营任务，返回命中数、创建数、待应用/阻断数和跳过原因，不新增数据库表。
- 内置 `/dashboard/saasAdmin/page` 的“客户成功队列”新增“客户成功续费任务”控件，平台管理员可按当前队列筛选批量生成续费任务；生成后刷新客户成功队列、统一运营任务中心和操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“客户成功续费任务”和 `createCustomerSuccessRenewalTasks`；普通租户调用续费任务接口返回 `403`；平台管理员调用 `customerSuccessRenewalTasks` 后断言目标租户续费任务、request/preview、状态和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐接口参数、页面入口、默认跳过策略和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard`、`env -u GOROOT go test ./internal/server`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 当前 shell 的 `GOROOT` 仍指向不存在的 `/Users/lv/go-local/go`，本轮 Go 命令均使用 `env -u GOROOT` 执行；实际 Go 版本沿用本机 `go1.26.4 darwin/arm64`。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `3721e562e9118e48476576eea2b509abb9a1f70823d77c130af8504ed830f50f`，纳入文件数为 `616`，生成时间为 `2026-07-10 02:53:06 CST`。

## 阶段补充（2026-07-10 续费任务批量应用）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 批量应用接口复用统一运营任务中心筛选条件：`taskId`、`taskType=tenant_renewal`、`status`、`tenantId`、`packageCode` 和 `limit`；显式拒绝非 `tenant_renewal` 任务类型。
- 服务端逐条重新读取任务请求、当前租户和当前套餐状态：成功任务写入续费账单并置为 `applied`；当前状态不允许应用或续费预演阻断时置为 `blocked`；坏请求或执行失败时置为 `failed`；已应用和已取消任务计入跳过数。
- 每条成功应用或阻断任务都会写入 `saas.admin.task.apply` 或 `saas.admin.task.block` 的 `admin_task` 操作日志，并在 `after` 中带上 `bulkApply=true` 与本次筛选条件，便于从任务中心追溯批量操作来源。
- 内置 `/dashboard/saasAdmin/page` 的“运营任务中心”新增“批量应用续费”按钮；页面会按当前任务筛选批量处理续费任务，默认在 `status=all` 时只处理 `pending`，执行后刷新任务、续费、总览、操作记录、账单流水和告警。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `bulkApplyRenewalTasks` 和 `tenantRenewalTaskBulkApply`；普通租户调用返回 `403`；平台管理员先通过客户成功队列生成续费任务，再批量应用续费任务，并断言目标任务状态、账单事件、执行结果和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐环境变量路由清单、API 表、页面能力和 smoke 验收口径。
- 本轮已验证：`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 当前 shell 的 `GOROOT` 仍指向不存在的 `/Users/lv/go-local/go`，本轮 Go 命令均使用 `env -u GOROOT` 执行；实际 Go 版本沿用本机 `go1.26.4 darwin/arm64`。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the Docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。

## 阶段补充（2026-07-10 客户成功续费提醒）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 续费提醒接口复用客户成功队列筛选条件 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority`；请求体支持 `channel=webhook`、`reminderDays`、`maxAttempts`、`remark` 和 `forceCreate`。
- 接口按客户成功队列批量生成 `tenant_renewal_reminder` 通知，写入现有 `mochat_go_saas_alert_notifications` outbox；通知 metric 为 `tenant_renewal`，不新增数据库迁移。
- 默认按租户、到期日和通道去重，跳过已存在的续费提醒；`forceCreate=true` 时允许复用唯一 key 重置为 `pending` 重新投递。
- 每条成功生成的通知都会写入 `tenant.renewal.notify` 操作日志，`targetType=alert_notification`，after JSON 保留通知 payload、客户成功筛选条件和备注，方便从操作记录追溯提醒来源。
- 内置 `/dashboard/saasAdmin/page` 的“客户成功队列”新增“客户成功续费提醒”控件，平台管理员可按当前队列筛选生成 webhook outbox，生成后刷新客户成功队列、通知列表、操作记录和运营日报。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `createCustomerSuccessRenewalNotifications`；普通租户调用续费提醒接口返回 `403`；平台管理员调用后断言目标租户 pending 通知、`metric=tenant_renewal`、`alertType=tenant_renewal_reminder`、通知 key、message、channel、maxAttempts、remark 和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐环境变量路由清单、API 表、页面能力、默认去重策略、`forceCreate` 语义和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 当前 shell 的 `GOROOT` 仍指向不存在的 `/Users/lv/go-local/go`，本轮 Go 命令均使用 `env -u GOROOT` 执行；实际 Go 版本为 `go1.26.4 darwin/arm64`。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `2762a9eb0d2cd4e5735c7fe1c795a8ffc756005cabe5b0be1e4aa5843d639141`，纳入文件数为 `616`，生成时间为 `2026-07-10 03:27:39 CST`。

## 阶段补充（2026-07-10 客户成功负责人工作台）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/customerSuccessOwners`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 负责人工作台复用客户成功队列筛选条件 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority`，但内部先按导出上限读取完整客户成功队列再按负责人聚合，避免明细 `limit` 截断导致负责人统计失真。
- 响应返回负责人租户数、`critical/high/medium/normal` 优先级分布、逾期/7 天内/阻断租户数、账单跟进数、可处理运营任务数、可重试/失败/耗尽通知数、最高/平均健康分、最早下次跟进时间和 Top 租户。
- 内置 `/dashboard/saasAdmin/page` 新增“客户成功负责人工作台”区块和“导出客户成功负责人 CSV”按钮，客户成功分派、续费任务、续费提醒和筛选变更后会刷新负责人汇总。
- `GET /dashboard/saasAdmin/export` 新增 `type=customerSuccessOwners`，并兼容 `customer_success_owners`、`cs_owners` 等别名；CSV 复用同一筛选口径导出负责人汇总。
- 修正统一运营任务 `status=all` 归一化：现在会按“不筛选状态”处理，避免批量取消、批量重置和续费任务批量应用把 `all` 当成真实状态导致匹配不到任务。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含负责人工作台和导出按钮；普通租户访问负责人接口和导出均返回 `403`；平台管理员分派 `smoke-csm` 后读取负责人工作台和 CSV，断言负责人、目标租户、优先级、通知信号、健康分、Top 租户和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐接口、页面、CSV 导出、筛选参数和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 当前 shell 的 `GOROOT` 仍指向不存在的 `/Users/lv/go-local/go`，本轮 Go 命令均使用 `env -u GOROOT` 执行；实际 Go 版本为 `go1.26.4 darwin/arm64`。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `f00c6d7d8fbeebfcf1b92d3c01d6084c98c9428ba5c8ff816adb44ffeac5e90d`，纳入文件数为 `616`，生成时间为 `2026-07-10 03:48:29 CST`。

## 阶段补充（2026-07-10 经营指标看板）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/businessMetrics`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 经营指标不新增数据库表，复用全平台租户总览、租户用量、风险看板、套餐目录和最近续费账单，基于最近续费金额估算套餐 MRR，并在响应中明确 `estimated=true`。
- 接口支持 `tenantLimit`、`expiringDays`、`highUsageRatio` 和 `billingLimit`；响应返回估算 MRR、ARR、ARPA、风险收入、即将到期收入、已到期收入、未知价格租户数、近期账单金额、最近账单样例和套餐收入分布。
- 当前 MRR 只统计“租户未停用、套餐启用、未过期、且有历史续费金额”的租户；已过期租户金额单独计入过期收入，避免把已流失或逾期收入混进当前 MRR。
- 内置 `/dashboard/saasAdmin/page` 新增“经营指标”区块和套餐收入分布表，总览刷新、续费记录、风险阈值变化和客户成功租户上限变化后会同步刷新经营指标。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“经营指标”、`businessMetrics` 和 `businessPackages`；普通租户访问经营指标返回 `403`；平台管理员在续费任务应用并写入账单后读取经营指标，断言最近续费金额进入 scale 套餐估算 MRR/ARR、近期账单汇总和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐环境变量路由清单、API 表、页面能力和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- 当前 shell 的 `GOROOT` 仍指向不存在的 `/Users/lv/go-local/go`，本轮 Go 命令均使用 `env -u GOROOT` 执行；实际 Go 版本为 `go1.26.4 darwin/arm64`。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `fc6b662f3eb67050493f7d34093ebd936c28e5fbe5c63a916607d41d9f5bd713`，纳入文件数为 `616`，生成时间为 `2026-07-10 04:07:37 CST`。

## 阶段补充（2026-07-10 经营趋势与续费漏斗）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/businessTrends`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 经营趋势不新增数据库表，复用 `mochat_go_saas_billing_events` 和 `mochat_go_saas_admin_tasks` 已有账单事件与运营任务数据。
- 接口支持 `months`、`billingLimit` 和 `taskLimit`；响应返回月度账单金额、账单数、续费数、覆盖租户数、套餐数、每月套餐流水分布，以及续费任务 pending/blocked/failed/applied/canceled/actionable 漏斗和最近续费任务。
- 月度趋势按 `PaidAt` 优先、`CreatedAt` 兜底归月，只统计最近 `months` 窗口内账单，避免历史账单混进当前趋势。
- 内置 `/dashboard/saasAdmin/page` 新增“经营趋势”区块、月度趋势表和续费任务漏斗表；总览刷新、续费记录、续费任务创建和续费任务应用后会同步刷新趋势。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“经营趋势”、`businessTrends` 和 `businessRenewalFunnel`；普通租户访问经营趋势返回 `403`；平台管理员在续费任务应用并写入账单后读取趋势接口，断言最近续费金额进入月度趋势、scale 套餐流水和续费任务漏斗，并校验启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐 API 表、页面能力和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `0d0fb3b3d0852c8562c1d7bf0cbb33da329c901b69fd18772ffd1bc12afe846d`，纳入文件数为 `616`，生成时间为 `2026-07-10 04:19:42 CST`。

## 阶段补充（2026-07-10 续费预测）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/renewalForecast`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 续费预测不新增数据库表，复用全平台租户总览、最近续费账单事件和 `tenant_renewal` 运营任务。
- 接口支持 `tenantLimit`、`days`、`billingLimit` 和 `taskLimit`；响应返回预测续费金额、估算 MRR、已到期/30 天内/31-60 天/61-90 天/90 天以上到期桶、未知价格租户、续费任务漏斗、最近续费任务和最近续费账单样例。
- 预测金额按套餐最近一笔续费账单价格估算；没有续费价格的到期租户会进入未知价格计数，避免把无依据金额混入预测收入。
- 预测清单会合并同租户续费任务状态，回显最新续费任务，并按到期紧急度和预测金额排序，方便平台客户成功优先跟进。
- 内置 `/dashboard/saasAdmin/page` 新增“续费预测”区块、到期桶表和到期租户表；总览刷新、续费记录、续费任务创建和续费任务应用后会同步刷新预测。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“续费预测”、`renewalForecast` 和 `renewalForecastBuckets`；普通租户访问续费预测返回 `403`；平台管理员在续费任务应用并写入账单后读取预测接口，断言目标租户进入预测清单、最近续费价格进入预测收入、并回显最新续费任务和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐环境变量路由清单、API 表、页面能力和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `71a2b94f8a55cfe876c61585748a6ef76909f695484bde02ac36c01927cd7e70`，纳入文件数为 `616`，生成时间为 `2026-07-10 04:31:49 CST`。

## 阶段补充（2026-07-10 续费预测任务）

- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/renewalForecastTasks`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 该接口复用 `renewalForecast` 的 `tenantLimit`、`days`、`billingLimit`、`taskLimit` 筛选，按预测命中的到期租户批量生成 `tenant_renewal` 运营任务；金额默认取预测续费金额，套餐默认取租户当前套餐，支持固定 `expiresAt` 或按 `months/renewMonths` 顺延、订单前缀、付款字段、备注和 `forceCreate`。
- 默认跳过已有 `pending/blocked/failed` 待处理续费任务的租户，避免重复制造待办；`forceCreate=true` 时允许重建。每条创建成功的任务都会写入 `saas.admin.task.create` / `admin_task` 操作日志，并在 `after` payload 中标记 `source=renewal_forecast` 和本次预测筛选条件。
- 内置 `/dashboard/saasAdmin/page` 在“续费预测”区块新增“续费预测任务”操作条，可输入预测续费月数、订单前缀并选择强制重建；生成后会刷新续费预测、经营趋势、续费任务、运营任务中心和操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `createRenewalForecastTasks`；普通租户调用续费预测任务接口返回 `403`；平台管理员在读取续费预测后调用 `renewalForecastTasks`，断言目标租户生成待处理续费任务、订单号、金额、备注和启动日志；后续客户成功续费任务 smoke 使用 `forceCreate=true`，避免被新建预测任务跳过。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `9173832eb8e69214f9946c006690de756ecbce500f673433b2258ca0c2546c34`，纳入文件数为 `616`，生成时间为 `2026-07-10 04:49:39 CST`。

## 阶段补充（2026-07-10 续费预测 CSV 导出）

- 新增平台级导出能力 `GET /dashboard/saasAdmin/export?type=renewalForecast`，仅平台租户超级管理员可用；普通租户超级管理员导出返回 `403`。
- 导出类型兼容 `renewalForecast`、`renewal_forecast`、`renewalForecasts`、`renewal_forecasts`、`forecast` 和 `forecasts`，复用续费预测筛选 `tenantLimit`、`days`、`billingLimit` 和 `taskLimit`。
- CSV 输出到期租户维度的 `tenantId`、租户名、租户状态、套餐编码/名称、到期时间、到期桶、剩余天数、预测续费金额、估算 MRR、是否已定价、最近续费账单、任务计数、可处理任务数和最新续费任务状态，方便平台团队把预测清单带到客户成功跟进或财务复核。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域新增“导出续费预测 CSV”按钮，使用当前续费预测筛选条件下载 `mochat-saas-renewalForecast.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出续费预测 CSV”；普通租户导出续费预测 CSV 返回 `403`；平台管理员创建续费预测任务后导出 CSV，并断言目标租户套餐、预测金额、定价状态、最近账单、任务计数和最新 `pending` 任务状态。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐环境变量能力清单、API 表、页面能力和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `608d5994dd2c37cb3c7c9db33971cb6b3ab913fcfc2bdd52f9cc264c95880543`，纳入文件数为 `616`，生成时间为 `2026-07-10 05:00:08 CST`。

## 阶段补充（2026-07-10 续费预测筛选增强）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/renewalForecast`、`POST/PUT /dashboard/saasAdmin/renewalForecastTasks` 和 `GET /dashboard/saasAdmin/export?type=renewalForecast` 现在统一复用续费预测筛选：`tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`。
- `bucket` 支持 `all/expired/due_0_30/due_31_60/due_61_90/due_later`；`priced` 支持 `all/priced/unknown`；`taskStatus` 支持 `all/none/pending/blocked/failed/applied/canceled`，其中 `none` 用于筛出尚未创建续费任务的到期租户。
- 续费预测构建会读取目标租户的最新风险跟进快照，把客户成功负责人带入预测租户行；`owner` 筛选按该负责人匹配，空负责人统一显示为“未分配”。
- 续费预测 JSON 租户行新增 `owner` 和 `riskFollowUp`；续费预测 CSV 在原字段末尾新增 `owner`、`riskFollowUpStatus` 和 `riskFollowUpNextAt`，便于把预测清单带到客户成功跟进。
- 内置 `/dashboard/saasAdmin/page` 新增“续费预测筛选”操作条，支持预测天数、到期窗口、定价状态、套餐、负责人和任务状态筛选；刷新预测后会同步刷新经营趋势。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“续费预测筛选”和 `renewalForecastTaskStatus`；平台管理员会先读取未筛选预测，再用目标租户自身的 bucket、定价状态、套餐、负责人和最新任务状态拼出过滤请求，断言过滤结果仍命中目标租户且不比未筛选结果更宽；CSV 断言新增负责人和风险跟进列。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增筛选参数、页面能力、CSV 字段和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `3bb805a5ee2e5ea84d075201a14d256f6728b90251d4e9f58cd3ee51344a9fa1`，纳入文件数为 `616`，生成时间为 `2026-07-10 05:14:44 CST`。

## 阶段补充（2026-07-10 续费预测批量分派）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/renewalForecastAssign`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 该接口复用续费预测当前筛选：`tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`，对命中的到期租户逐条写入 `tenant.risk.follow_up` 风险跟进记录。
- 请求体支持 `owner/assignOwner`、`status=pending/contacted/renewal_pending`、`nextFollowUpAt` 和 `remark`；默认状态为 `renewal_pending`，默认备注为“续费预测批量分派”，不允许批量分派时直接写入 `resolved/ignored`。
- 内置 `/dashboard/saasAdmin/page` 在“续费预测”区块新增“续费预测分派”操作条，可填写负责人、下次跟进时间、分派状态和备注；提交后会刷新续费预测、客户成功队列、客户成功负责人工作台、风险跟进任务、风险负责人、运营日报和操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“续费预测分派”和 `assignRenewalForecast`；普通租户调用 `renewalForecastAssign` 返回 `403`；平台管理员会先读取未筛选预测，再用目标租户自身的 bucket、定价状态、套餐、负责人和最新任务状态拼出过滤请求，随后调用分派接口，断言目标租户写入 `forecast-csm`、`renewal_pending`、下次跟进时间和备注；续费预测 CSV 会回显最新负责人、跟进状态和跟进时间。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增接口、页面能力和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `0e10a599e843e2e33a69cb7b8709ac887b352cb92b832573c5a7bf6379965fa9`，纳入文件数为 `616`，生成时间为 `2026-07-10 05:30:48 CST`。

## 阶段补充（2026-07-10 续费预测负责人工作台）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/renewalForecast` 现在在原有 `summary`、到期桶和到期租户明细之外，新增 `owners` 负责人聚合。
- `owners` 完全复用当前续费预测筛选口径：`tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`；筛选后的负责人聚合不会混入全量租户。
- 负责人聚合输出每个负责人名下的预测续费金额、估算 MRR、定价/未知价格租户数、已到期/30 天内/31-60 天/61-90 天/90 天以上窗口分布、待处理/待应用/阻断/失败/已应用/已取消任务数、最早下次跟进时间和 Top 租户。
- 内置 `/dashboard/saasAdmin/page` 在“续费预测”区块新增“续费预测负责人工作台”表格，展示负责人、预测续费、到期窗口、任务压力和 Top 租户，分派后运营可以按负责人看预测收入和待办压力。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `renewalForecastOwners` 和“续费预测负责人工作台”；平台管理员读取续费预测时会断言负责人聚合命中目标租户、预测金额、定价数、已应用任务数和 Top 租户；按目标租户筛选后再次断言负责人聚合同步收窄。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐负责人工作台字段、页面能力和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `c2022089e0c41f9334ca17157bbc1debe1015487bde0efb20671fc1fcb2903b5`，纳入文件数为 `616`，生成时间为 `2026-07-10 05:39:08 CST`。

## 阶段补充（2026-07-10 续费预测负责人 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级导出能力 `GET /dashboard/saasAdmin/export?type=renewalForecastOwners`，仅平台租户超级管理员可用；普通租户超级管理员导出返回 `403`。
- 导出类型兼容 `renewalForecastOwners`、`renewal_forecast_owners`、`forecastOwners` 和 `forecast_owners` 等别名，复用续费预测筛选 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`。
- CSV 输出负责人维度的 `owner`、租户数、定价/未知价格租户数、预测续费金额、估算 MRR、已到期/30 天内/31-60 天/61-90 天/90 天以上窗口分布、可处理/待应用/阻断/失败/已应用/已取消任务数、最早下次跟进时间和 Top 租户。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域新增“导出续费预测负责人 CSV”按钮，使用当前续费预测筛选条件下载 `mochat-saas-renewalForecastOwners.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出续费预测负责人 CSV”；普通租户导出续费预测负责人 CSV 返回 `403`；平台管理员在分派预测客户并创建续费预测任务后导出负责人 CSV，并断言 `forecast-csm` 的预测金额、定价租户数、可处理任务、待处理任务、下次跟进和 Top 租户。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增导出类型、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `3d134c1efe93641d9cf8839fbf2da0c2fa58576f86e63d6d49869ab18252402a`，纳入文件数为 `616`，生成时间为 `2026-07-10 05:48:57 CST`。

## 阶段补充（2026-07-10 续费预测提醒）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/renewalForecastNotifications`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 该接口复用续费预测当前筛选：`tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`，对命中的到期租户批量生成续费提醒通知。
- 请求体支持 `channel=webhook`、`reminderDays`、`maxAttempts`、`remark` 和 `forceCreate`；默认同一租户、同一到期日、同一通道去重，`forceCreate=true` 时允许重新创建。
- 通知写入既有 `mochat_go_saas_alert_notifications` outbox，使用 `metric=tenant_renewal`、`alertType=tenant_renewal_reminder` 和 `renewal_YYYYMMDD` 续费周期键；通知上下文带入套餐、到期日、剩余天数、到期桶、预测续费金额、估算 MRR、负责人、最近续费账单和备注。
- 内置 `/dashboard/saasAdmin/page` 在“续费预测”区块新增“续费预测提醒”操作条，可设置提醒天数、最大重试次数、备注和强制重建；提交后刷新续费预测、通知 outbox、运营日报和操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“续费预测提醒”和 `createRenewalForecastNotifications`；普通租户调用续费预测提醒接口返回 `403`；平台管理员按 `owner=forecast-csm` 生成提醒，断言目标租户产生 `pending` 通知、`metric=tenant_renewal`、`alertType=tenant_renewal_reminder`、消息包含目标租户名、`notificationKey` 包含续费提醒键，并确认启动日志暴露该路由。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增接口、页面能力、通知 outbox 语义和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `5fa61de976f8fe3b326cfe123b3567e750ac76df5b1169f64c6e240ccf5e86e6`，纳入文件数为 `616`，生成时间为 `2026-07-10 06:08:07 CST`。

## 阶段补充（2026-07-10 生命周期审计筛选）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/tenantLifecycle` 在原有 `tenantId` 和 `limit` 基础上新增 timeline 筛选：`source=all/operation/billing/task/alert/notification`、`eventType`、`status` 和 `keyword`。
- `source` 会做合法性校验，非法值返回 `400`；`eventType`、`status` 和 `keyword` 采用大小写不敏感匹配，关键字会覆盖来源、事件、标题、状态、备注、引用 ID、操作人和 payload JSON。
- 生命周期通知事件的 `eventType` 现在优先使用通知携带的 `alertType`，例如 `tenant_renewal_reminder`，通道仍保留在通知 payload 中，便于运营人员按提醒类型定位事件。
- 响应 `filters` 回显实际筛选条件；`summary` 新增 `rawTimelineCount` 和 `filterActive`，并保留筛选后的 `timelineCount` 与截断后的 `returnedEventCount`，避免筛选视图被误读为租户生命周期全量事件。
- 内置 `/dashboard/saasAdmin/page` 的“生命周期审计”区域新增“生命周期筛选”操作条，可按来源、事件、状态和关键字刷新 timeline；筛选命中时标题显示“命中 / 全部 / 显示”。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“生命周期筛选”、`tenantLifecycleSource`、`tenantLifecycleEventType` 和 `tenantLifecycleKeyword`；平台管理员在生成续费预测提醒后调用 `tenantLifecycle?source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=tenant_renewal_reminder`，断言筛选条件回显、`filterActive=true`、`rawTimelineCount >= timelineCount >= 1`，并确认返回事件全部为目标租户的 pending 续费提醒通知。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增筛选参数、页面筛选入口、summary 字段和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `d2789acff5c5ffe418824360767a77c26c23f027a9218d61f20ee26ec9b01ba7`，纳入文件数为 `616`，生成时间为 `2026-07-10 06:17:29 CST`。

## 阶段补充（2026-07-10 生命周期审计 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级导出能力 `GET /dashboard/saasAdmin/export?type=tenantLifecycle`，仅平台租户超级管理员可用；普通租户超级管理员导出返回 `403`。
- 导出类型兼容 `tenantLifecycle`、`tenant_lifecycle`、`lifecycle` 等别名，复用 `tenantLifecycle` 当前筛选：`tenantId`、`limit`、`source`、`eventType`、`status`、`keyword` 和 `expiringDays`。
- CSV 输出筛选后的统一时间线，字段包含租户 ID、租户名、套餐编码/名称、来源、事件类型、标题、状态、发生时间、引用 ID、操作人、备注和 payload JSON，方便平台运营把单租户续费提醒、任务、账单、告警和操作审计导出留档。
- 内置 `/dashboard/saasAdmin/page` 的“生命周期筛选”操作条新增“导出审计 CSV”按钮，导出当前租户和当前筛选条件下的 timeline。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出审计 CSV”和 `exportTenantLifecycle`；普通租户导出生命周期审计 CSV 返回 `403`；平台管理员在生成续费预测提醒后按 `source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=tenant_renewal_reminder` 导出 CSV，并断言目标租户、来源、状态、事件类型、标题和 payload。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增导出类型、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `7d72576b450b4b531285b392556cb3499673df8d0f456d24f1734260a43e4a23`，纳入文件数为 `616`，生成时间为 `2026-07-10 06:27:04 CST`。

## 阶段补充（2026-07-10 经营指标与经营趋势 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级导出能力 `GET /dashboard/saasAdmin/export?type=businessMetrics` 和 `GET /dashboard/saasAdmin/export?type=businessTrends`，仅平台租户超级管理员可用；普通租户超级管理员导出返回 `403`。
- `businessMetrics` 导出复用经营指标筛选 `tenantLimit`、`expiringDays`、`highUsageRatio` 和 `billingLimit`，CSV 采用 `section/metric/value/remark` 结构，包含筛选条件、经营汇总、套餐估算 MRR/ARR/风险/到期收入和最近账单行。
- `businessTrends` 导出复用经营趋势筛选 `months`、`billingLimit` 和 `taskLimit`，CSV 采用 `section/metric/value/remark` 结构，包含筛选条件、趋势汇总、月度流水、套餐流水和续费任务漏斗。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域新增“导出经营指标 CSV”和“导出经营趋势 CSV”按钮，下载时沿用页面当前经营指标和趋势参数。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含两个导出按钮；普通租户导出经营指标和经营趋势均返回 `403`；平台管理员在续费任务应用并写入账单后导出经营指标和经营趋势 CSV，并断言筛选、汇总、scale 套餐收入、最近账单和续费任务漏斗行。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增导出类型、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminExportCSVBusinessMetricsAndTrendsAllowsPlatformAdmin|TestSaaSAdminBusinessMetricsSummarizesEstimatedRevenue|TestSaaSAdminBusinessTrendsSummarizesMonthlyBillingAndRenewalFunnel'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。

## 阶段补充（2026-07-10 运营待办队列）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/operationQueue`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 运营待办队列不新增数据库表，直接聚合既有客户成功队列、统一运营任务 SLA、账单跟进任务、失败/耗尽通知 outbox 和已关闭通知 outbox。
- 筛选参数包含 `source=all/customer_success/task_sla/billing_follow_up/notification/closed_notification`、`priority=critical/high/medium/normal/all`、`owner`、`keyword`、`tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`warningHours` 和 `overdueHours`；响应返回来源分布、优先级分布、租户数、未分配数量和排序后的待办明细。
- 新增导出类型 `GET /dashboard/saasAdmin/export?type=operationQueue`，复用同一筛选输出来源、优先级、租户、负责人、标题、原因、下一步动作、状态、对象、停留时间和备注，兼容 `operation_queue`、`ops_queue`、`admin_queue` 等别名。
- 内置 `/dashboard/saasAdmin/page` 新增“运营待办队列”区块，支持来源、优先级、负责人和关键字筛选；“数据导出”新增“导出待办 CSV”按钮，并在客户成功、任务 SLA 阈值、通知关闭/重试、风险跟进和账单跟进动作后刷新聚合待办。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“运营待办队列”和“导出待办 CSV”；普通租户访问接口和导出返回 `403`；平台管理员在通知关闭后读取 `operationQueue` 和 CSV，断言任务 SLA、关闭通知、严重优先级、目标租户和 CSV 表头；启动日志会暴露该路由。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、CSV、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminOperationQueue|TestSaaSAdminExportCSVOperationQueue|TestSaaSAdminPageContainsSaaSAdminControls|TestSaaSAdminPageIncludesDailyReportWindowControls'`、`env -u GOROOT go test ./internal/server -run TestSaaSAdminRoutesDispatchWhenConfigured`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue.json`，当前源码指纹为 `8d26f539f6c6d85bdea620feffb18a93ca78df2621af58b2be028fbb6fe7b0b5`，纳入文件数为 `616`，生成时间为 `2026-07-10 08:57:56 CST`。

## 阶段补充（2026-07-10 开户任务批量应用）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 批量接口复用统一运营任务筛选：`taskId`、`taskType=tenant_provision`、`status`、`tenantId`、`packageCode` 和 `limit`，当未传 `taskType` 时默认限定为 `tenant_provision`，传入其他任务类型返回 `400`。
- 每条平台开户任务应用前会从任务 request 读取已固化的 `adminPasswordHash`，重新复核套餐仍启用，再复用 `ProvisionSaaSAdminTenant` 写路径创建租户、初始超级管理员、角色授权、套餐快照、开通记录和 26 项用量计数。
- 执行成功的任务会更新为 `applied`、写入任务结果和 `appliedAt`，并记录带 `bulkApply=true` 和任务筛选条件的 `saas.admin.task.apply` 操作日志；已应用和已取消任务会跳过；坏请求或执行失败任务会更新为 `failed` 并进入响应 `errors`。
- 批量响应沿用 `SaaSAdminTaskBulkApplyResult`，返回 `appliedCount`、`blockedCount`、`failedCount`、`skippedAppliedCount`、`skippedCanceledCount`、`skippedUnsupportedCount`、`tasks` 和 `errors`；平台开户任务 request 仍只返回脱敏后的 `hasAdminPasswordHash`，不回显明文密码或哈希。
- 内置 `/dashboard/saasAdmin/page` 的“平台开户任务”区域新增“批量应用开户”按钮，会按当前任务中心租户/套餐筛选或当前开户链接套餐批量处理待应用开户链接任务，并刷新 overview、操作记录、任务中心和开户链接任务列表。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `bulkApplyProvisionTasks` 和 `tenantProvisionTaskBulkApply`；普通租户调用批量接口返回 `403`；平台管理员创建第二个开户链接任务后通过批量接口应用，并断言响应脱敏、新租户、管理员、套餐快照、开通记录、任务 `applied_at` 和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口、统一任务中心说明和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminTenantProvisionTaskBulkApply|TestSaaSAdminTenantProvisionTaskApplyUsesStoredHash|TestSaaSAdminTenantRenewalTaskBulkApply'`、`env -u GOROOT go test ./internal/server -run 'Test.*SaaSAdmin'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。

## 阶段补充（2026-07-10 套餐同步任务批量应用）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/packageSyncTaskBulkApply`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 批量接口复用统一运营任务筛选：`taskId`、`taskType=package_sync`、`status`、`tenantId`、`packageCode` 和 `limit`，当未传 `taskType` 时默认限定为 `package_sync`，传入其他任务类型返回 `400`。
- 每条套餐同步任务应用前会从任务 request 读取已固化的套餐同步参数，强制 `dryRun=false`，重新计算当前套餐、租户快照和超额风险；成功任务复用套餐快照同步写路径刷新租户套餐快照和 26 项用量额度。
- 仍存在超额租户且未允许超额的任务会更新为 `blocked`、写入预演结果和 `LastError`，并记录带 `bulkApply=true` 和筛选条件的 `saas.admin.task.block` 操作日志；执行成功任务会更新为 `applied`、写入执行结果和 `appliedAt`，并记录 `saas.admin.task.apply` 操作日志；坏请求或执行失败任务会更新为 `failed` 并进入响应 `errors`；已应用和已取消任务会跳过。
- 批量响应沿用 `SaaSAdminTaskBulkApplyResult`，返回 `appliedCount`、`blockedCount`、`failedCount`、`skippedAppliedCount`、`skippedCanceledCount`、`skippedUnsupportedCount`、`tasks` 和 `errors`。
- 内置 `/dashboard/saasAdmin/page` 的“套餐同步任务”区域新增“批量应用任务”按钮，会按当前套餐编码和租户 ID 筛选批量处理待应用套餐同步任务，并刷新 overview、操作记录、告警、账单、任务中心和套餐同步任务列表。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `bulkApplyPackageSyncTasks` 和 `packageSyncTaskBulkApply`；普通租户调用批量接口返回 `403`；平台管理员创建第二个套餐同步任务后通过批量接口应用，并断言任务结果、租户套餐快照、任务 `applied_at` 和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口、统一任务中心说明和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminPackageSyncTaskBulkApply|TestSaaSAdminPackageSyncTaskApplyUsesStoredRequest|TestSaaSAdminTenantProvisionTaskBulkApply|TestSaaSAdminTenantRenewalTaskBulkApply'`、`env -u GOROOT go test ./internal/server -run 'Test.*SaaSAdmin'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。

## 阶段补充（2026-07-10 运营任务负责人聚合）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/taskOwners`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 接口复用统一运营任务筛选：`taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit`，从既有 `mochat_go_saas_admin_tasks` 聚合，不新增迁移。
- 响应返回 `summary`、`ownerCount`、`returnedCount`、`scannedTaskCount`、`partial` 和 `owners`；负责人维度包含任务总数、可处理/阻断/失败/已应用分布、套餐同步/平台开户/续费分布、命中租户数、最近任务、最近应用时间和最近错误。
- 内置 `/dashboard/saasAdmin/page` 的统一运营任务中心新增“任务负责人”表和“刷新负责人”按钮，复用当前任务类型、状态、租户和套餐筛选；任务刷新、状态变化和批量操作后会同步刷新负责人聚合。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“任务负责人”、`adminTaskOwners` 和 `taskOwners`；普通租户访问 `taskOwners` 返回 `403`；三类任务应用后调用 `GET /dashboard/saasAdmin/taskOwners?taskType=all&status=applied&limit=20`，断言负责人聚合回显平台操作人、汇总只包含已应用任务、三类任务分布存在，并继续校验开户链接任务不泄漏明文密码或 `adminPasswordHash`。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminTaskOwners|TestSaaSAdminTasksReturnsSummaryAndReturnedCount|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/server -run 'Test.*SaaSAdmin'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `33160695d3acc5555167f8c192a652af77ea8f09bc0261e41380178482307a56`，纳入文件数为 `616`，生成时间为 `2026-07-10 07:16:51 CST`。

## 阶段补充（2026-07-10 运营任务 SLA/逾期视图）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/taskSla`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 接口复用统一运营任务筛选：`taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit`，额外支持 `warningHours` 和 `overdueHours`，默认预警 4 小时、逾期 24 小时，且 `warningHours` 必须小于 `overdueHours`。
- SLA 计算只把 `pending/blocked/failed` 作为活跃任务；`applied/canceled` 仍进入原始 `taskSummary`，但不进入时效告警，避免历史已完成任务误报超时。
- 响应返回 `taskSummary`、SLA `summary`、负责人聚合、最严重任务列表、扫描数量和 partial 标记；SLA 汇总包含正常、预警、逾期、未知时间、最大任务年龄、任务类型分布、状态分布、命中租户数和操作人数。
- 内置 `/dashboard/saasAdmin/page` 的任务中心新增 `SLA预警`、`SLA逾期` 输入、“刷新SLA”按钮和“任务SLA”表，复用当前任务筛选展示活跃任务时效；任务刷新后会同步刷新负责人和 SLA。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“任务SLA”、`adminTaskSla` 和 `taskSla`；普通租户访问 `taskSla` 返回 `403`；创建 blocked 套餐同步任务后调用 `GET /dashboard/saasAdmin/taskSla?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10`，断言活跃 blocked 任务进入 SLA 汇总、负责人聚合和任务列表。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminTaskSLA|TestSaaSAdminTaskOwners|TestSaaSAdminTasksReturnsSummaryAndReturnedCount|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/server -run 'Test.*SaaSAdmin'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `7f5fa7936c5e7b4da1a6d63cf60b2f64dcccb0657a367d54465ebe8d5ebfd717`，纳入文件数为 `616`，生成时间为 `2026-07-10 07:30:20 CST`。

## 阶段补充（2026-07-10 运营任务 SLA CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级导出能力 `GET /dashboard/saasAdmin/export?type=taskSla`，仅平台租户超级管理员可用；普通租户超级管理员导出返回 `403`。
- 导出复用统一运营任务和 SLA 筛选：`taskId`、`taskType`、`status`、`tenantId`、`packageCode`、`limit`、`warningHours` 和 `overdueHours`，并兼容 `task_sla`、`admin_task_sla`、`sla_tasks` 等别名。
- CSV 只输出 `pending/blocked/failed` 活跃任务，字段包含任务 ID、任务类型、状态、租户、套餐、负责人、操作人、SLA 状态、任务年龄、超时小时数、备注、错误、时间戳、脱敏请求、预览和结果，便于平台运营离线追踪逾期任务。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域新增“导出任务SLA CSV”按钮，会沿用任务中心当前筛选和 SLA 阈值下载 `mochat-saas-taskSla-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `exportTaskSla`；普通租户导出任务 SLA CSV 返回 `403`；创建 blocked 套餐同步任务后调用 `GET /dashboard/saasAdmin/export?type=taskSla&taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=1000`，解析 CSV 表头并断言 blocked `package_sync` 任务、套餐和 SLA 状态。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增导出类型、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminExportCSVTaskSLAAllowsPlatformAdmin|TestSaaSAdminExportCSVTasksAllowsPlatformAdmin|TestSaaSAdminTaskSLA|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `f880bb537e23ad00900411a5727e324694bf2ec86252d2ac90ac222679eff034`，纳入文件数为 `616`，生成时间为 `2026-07-10 07:38:58 CST`。

## 阶段补充（2026-07-10 运营任务 SLA 催办提醒）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/taskSlaNotifications`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 接口复用统一运营任务 SLA 筛选：`taskId`、`taskType`、`status`、`tenantId`、`packageCode`、`limit`、`warningHours` 和 `overdueHours`；请求体支持 `slaStatus=warning/overdue/fresh/unknown/all`、`maxAttempts`、`remark` 和 `forceCreate`，默认只催办预警和逾期活跃任务。
- 催办复用现有通知 outbox，不新增迁移；通知写入 `mochat_go_saas_alert_notifications`，`alertType=admin_task_sla_reminder`、`metric=admin_task_sla`、`source=saas_admin.task_sla.notification`，同一任务、SLA 状态和阈值默认去重，`forceCreate=true` 时重置为 pending。
- 每条成功入队的提醒会追加 `saas.admin.task.sla_notify` / `alert_notification` 操作日志，`after` 中包含通知、当前 SLA 筛选、任务 SLA payload、催办阈值和 `forceCreate`，可继续通过操作记录、通知列表、通知 CSV、通知重试和生命周期审计追踪。
- 内置 `/dashboard/saasAdmin/page` 的任务筛选区新增“生成SLA提醒”按钮，会按当前任务筛选和 SLA 阈值调用新接口，完成后刷新任务 SLA、通知列表和操作记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含 `createTaskSlaNotifications`；普通租户调用提醒接口返回 `403`；测试中将 blocked 套餐同步任务时间回拨到 6 小时前后，调用 `POST /dashboard/saasAdmin/taskSlaNotifications?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10`，断言 pending 通知、`admin_task_sla_reminder`、`admin_task_sla`、最大尝试次数和启动日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口、通知 outbox 复用和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server -run 'TestSaaSAdminTaskSLANotifications|TestSaaSAdminTaskSLA|TestSaaSAdminPageContainsSaaSAdminControls|TestServerRoutesSaaSAdmin'`、`env -u GOROOT go test ./internal/server -run TestSaaSAdminRoutesDispatchWhenConfigured`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `82229ac7dac21ada80d379a4e67f39ad3e2c7daba2f5411165775d6b9eb04028`，纳入文件数为 `616`，生成时间为 `2026-07-10 07:52:03 CST`。

## 阶段补充（2026-07-10 运营日报接入任务 SLA）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/dailyReport` 现在会默认按 4 小时预警、24 小时逾期统计统一运营任务 SLA，只把 `pending/blocked/failed` 作为活跃任务纳入日报，避免已应用或已取消任务误报超时。
- 日报响应新增 `summary.taskSlaActiveCount`、`taskSlaWarningCount`、`taskSlaOverdueCount`、`taskSlaOwnerCount` 和 `taskSlaMaxAgeHours`，并新增 `taskSla` 分区，返回任务 SLA 筛选、原始任务汇总、SLA 汇总、负责人聚合和最严重任务列表。
- 内置 `/dashboard/saasAdmin/page` 的“运营日报明细”新增“日报任务SLA”表；日报顶部卡片新增“任务SLA”，展示活跃、逾期和预警任务数，便于平台运营只看日报时也能发现阻断或失败任务积压。
- `GET /dashboard/saasAdmin/export?type=dailyReport` 新增 `taskSlaOwner` 和 `taskSlaTask` section，输出负责人任务压力、SLA 状态、任务年龄、超时小时数、任务 ID、租户、套餐、状态、错误和备注。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“日报任务SLA”；创建 blocked 套餐同步任务并回拨到 6 小时前后，再调用 `GET /dashboard/saasAdmin/dailyReport` 和 `export?type=dailyReport`，断言日报 summary、`taskSla` 分区、`taskSlaOwner` 和 `taskSlaTask` CSV 行回显预警任务。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐日报任务 SLA、日报 CSV section 和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminDailyReport|TestSaaSAdminExportCSVDailyReport|TestSaaSAdminTaskSLA|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/server -run 'Test.*SaaSAdmin'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint.json`，当前源码指纹为 `80fe5ddac7d74504ce69b3f33458ce6913a06ba640341f55bfd803cfd60844bc`，纳入文件数为 `616`，生成时间为 `2026-07-10 08:02:45 CST`。

## 阶段补充（2026-07-10 通知 outbox 关闭）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增通知状态 `closed`，用于平台运营主动终止待发送或失败通知；通知 dispatcher 只拉取 `pending/failed`，因此 `closed` 不会再次投递。
- 新增 `POST/PUT /dashboard/saasAdmin/notificationClose`，按 `notificationId` 将 pending/failed 通知关闭为 `closed`，清空 `next_retry_at`，把关闭备注写入 `last_error`，并追加 `tenant.notification.close` / `alert_notification` 操作日志。
- 新增 `POST/PUT /dashboard/saasAdmin/notificationBulkClose`，按 `tenantId`、`status=pending/failed/all`、`channel`、`keyword` 和 `limit` 批量关闭 pending/failed 通知；普通租户超级管理员只能处理本租户，跨租户返回 `403`。
- `GET /dashboard/saasAdmin/notifications` 现在支持 `status=closed`，summary 新增 `closedCount`；`dead` 仍保留为可重试耗尽状态，不归入关闭数量。
- 内置 `/dashboard/saasAdmin/page` 的通知表新增“已关闭”筛选、行内“关闭”按钮和“批量关闭”按钮；关闭后会刷新通知列表、操作记录、overview 和运营日报。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含批量关闭入口和已关闭状态；普通租户跨租户批量关闭返回 `403`；平台管理员在通知批量重试后调用单条关闭和批量关闭，断言通知进入 `closed`、`nextRetryAt` 清空、关闭备注写入 `lastError`，并写入 `tenant.notification.close` 操作日志。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdmin.*Notification|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/server -run 'Test.*SaaSAdmin'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。

## 阶段补充（2026-07-10 运营日报接入关闭通知）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/dailyReport` 已接入关闭通知复盘：日报 summary 新增 `closedNotificationCount` 和 `windowNotificationCloseCount`，通知 summary 新增 `closedCount`。
- 日报响应新增 `notifications.closedItems`，按日期窗口返回 `updatedAt` 落在窗口内的 `closed` 通知，按更新时间倒序排列，便于平台运营看出哪些通知是在当天被人工终止。
- 内置 `/dashboard/saasAdmin/page` 的日报明细表从“日报失败通知”调整为“日报通知”，同一表格合并展示可重试通知和已关闭通知，并用“已关闭”状态标记关闭项。
- `GET /dashboard/saasAdmin/export?type=dailyReport` 新增 `closedNotification` section，输出关闭通知状态、通知 key、租户、通道、重试次数、关闭备注和更新时间。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：在 `notificationClose` 和 `notificationBulkClose` 后再次读取显式日期窗口的日报 JSON 和日报 CSV，断言关闭数量、`closedItems` 和 `closedNotification` CSV 行回显关闭备注。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐日报关闭通知字段、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminDailyReport|TestSaaSAdminExportCSVDailyReport|TestSaaSAdmin.*Notification|TestSaaSAdminPageContainsSaaSAdminControls'`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-final.json`，当前源码指纹为 `7ec6d12b72f41159f425a5b13beb1817c1c1c165b3c81012cd31cf3704ae8e4f`，纳入文件数为 `616`，生成时间为 `2026-07-10 08:33:18 CST`。

## 阶段补充（2026-07-10 运营待办负责人工作台）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/operationQueueOwners`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 接口复用 `operationQueue` 的 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`source`、`priority`、`owner`、`keyword`、`warningHours` 和 `overdueHours` 筛选，按负责人聚合客户成功、任务 SLA、账单跟进、失败/耗尽通知和已关闭通知。
- 响应返回负责人待办数、租户数、来源数、优先级分布、来源分布、未分配数量、最大停留小时、Top 租户和 Top 待办；空负责人统一归为“未分配”，便于平台运营识别无人认领的通知和待办。
- 新增导出类型 `GET /dashboard/saasAdmin/export?type=operationQueueOwners`，复用同一筛选输出负责人 CSV，兼容 `operation_queue_owners`、`ops_queue_owners` 和 `admin_queue_owners` 等别名。
- 内置 `/dashboard/saasAdmin/page` 在“运营待办队列”下方新增“运营待办负责人工作台”，并在客户成功、续费预测、风险跟进、账单跟进、告警、通知和任务 SLA 阈值变化后同步刷新；“数据导出”新增“导出待办负责人 CSV”按钮。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“运营待办负责人工作台”和“导出待办负责人 CSV”；普通租户访问接口和导出返回 `403`；平台管理员在通知关闭后读取 `operationQueueOwners` 和 CSV，断言任务 SLA、关闭通知、Top 租户和 CSV 表头；启动日志会暴露该路由。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、导出类型、页面入口和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminOperationQueueOwners|TestSaaSAdminExportCSVOperationQueueOwners|TestSaaSAdminPageContainsSaaSAdminControls|TestSaaSAdminPageIncludesDailyReportWindowControls'`、`env -u GOROOT go test ./internal/server -run TestSaaSAdminRoutesDispatchWhenConfigured`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-owners.json`，当前源码指纹为 `d83e392cd1783db360245ef3f3f157f9913176570caef5986c2aa956983dc0bc`，纳入文件数为 `616`，生成时间为 `2026-07-10 09:15:14 CST`。

## 阶段补充（2026-07-10 运营待办批量分派）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/operationQueueAssign`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 接口复用运营待办筛选：`tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`source`、`priority`、`owner`、`keyword`、`warningHours` 和 `overdueHours`，请求体支持 `owner`、`assignOwner`、`assignee`、`status`、`nextFollowUpAt` 和 `remark`。
- `customer_success` 与 `billing_follow_up` 来源仍写入原业务审计链路：客户成功待办写入风险跟进记录，账单跟进待办写入账单对账跟进操作日志；没有原生负责人字段的 `task_sla`、失败通知和关闭通知写入 `saas.admin.operation_queue.assign` 运营队列级认领日志，并在后续待办队列和负责人工作台中覆盖 owner。
- 响应返回匹配数、可分派数、已分派数、跳过数、不支持来源数、客户成功写入数、账单跟进写入数、运营队列认领写入数，以及每条待办的原始队列项、写入结果或跳过原因，便于页面和 smoke 精确断言。
- 内置 `/dashboard/saasAdmin/page` 的“运营待办队列”新增“运营待办批量分派”工具条，支持填写负责人、跟进状态、下次跟进时间和备注；提交后会刷新待办、负责人工作台、客户成功、风险跟进、账单跟进、操作记录和运营日报。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“运营待办批量分派”和“分派当前待办”；普通租户调用分派接口返回 `403`；平台管理员调用 `POST /dashboard/saasAdmin/operationQueueAssign?source=customer_success&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24`，断言客户成功待办分派到 `ops-queue`，并在 `riskFollowUps` 和 `operationQueueOwners` 中回显新负责人；随后调用 `POST /dashboard/saasAdmin/operationQueueAssign?source=task_sla&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24`，断言任务 SLA 待办认领到 `ops-task-sla`，并在 `operationQueue`、`operationQueueOwners` 和 `operations` 中回显。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口、支持来源、跳过策略和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminOperationQueueAssign|TestSaaSAdminOperationQueueOwners|TestSaaSAdminExportCSVOperationQueueOwners|TestSaaSAdminPageContainsSaaSAdminControls|TestSaaSAdminPageIncludesDailyReportWindowControls'`、`env -u GOROOT go test ./internal/server -run TestSaaSAdminRoutesDispatchWhenConfigured`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assign.json`，当前源码指纹为 `bc9b994a4d3b546250d13b097a815fcaf56de27bb513be4339f66010b0a626af`，纳入文件数为 `616`，生成时间为 `2026-07-10 09:27:17 CST`。

## 阶段补充（2026-07-10 运营待办认领覆盖任务 SLA）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `POST/PUT /dashboard/saasAdmin/operationQueueAssign` 已从“只写客户成功/账单跟进”扩展为“全运营待办可认领”：`customer_success` 和 `billing_follow_up` 保持原业务写入，`task_sla`、失败通知和关闭通知写入 `saas.admin.operation_queue.assign` 操作日志。
- 新增运营队列级认领 payload：`source`、`objectType`、`objectId`、`owner`、`status`、`nextFollowUpAt`、`remark`、`actorUserId`、`actorTenantId` 和 `assignedAt`；响应中每条认领结果新增 `operationQueueAssignment`。
- `buildOperationQueue` 会读取最新 `saas.admin.operation_queue.assign` 操作日志，按 `source:objectId` 覆盖队列项的 `owner` 和 `remark`，并在 `operationQueue` 响应 item 中返回 `assignment`；因此 `operationQueueOwners`、负责人筛选和待办 CSV 都会复用同一认领结果。
- 这次没有新增数据库表，也没有伪造 `mochat_go_saas_admin_tasks` 或通知 outbox 的负责人字段；任务 SLA 的原 `actorUserId/actorTenantId` 仍代表创建/执行主体，运营认领只存在于总后台队列层审计中。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：客户成功分派 smoke 保持不变；新增 `source=task_sla` 分派到 `ops-task-sla`，并通过 `operationQueue?owner=ops-task-sla`、`operationQueueOwners?owner=ops-task-sla` 和 `operations?action=saas.admin.operation_queue.assign&targetType=admin_task` 回看。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐运营队列级认领日志、任务 SLA owner 覆盖和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminOperationQueueAssign|TestSaaSAdminOperationQueueOwners|TestSaaSAdminOperationQueueAllowsPlatformAdmin|TestSaaSAdminExportCSVOperationQueue'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assignment.json`，当前源码指纹为 `020af61cf741c30b2b367ceffc0947be92f712188629615a4bac9527cce70f9f`，纳入文件数为 `616`，生成时间为 `2026-07-10 09:40:22 CST`。

## 阶段补充（2026-07-10 运营待办认领记录视图）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/operationQueueAssignments`，仅平台租户超级管理员可查看；普通租户超级管理员访问返回 `403`。
- 接口从 `saas.admin.operation_queue.assign` 操作日志提取运营队列级认领记录，支持 `source`、`owner`、`status`、`objectType/targetType`、`objectId/targetId`、`tenantId`、`keyword` 和 `limit` 筛选；`tenantId` 同时在 store 查询和聚合层过滤，返回认领汇总、租户数、负责人数量、来源数量、任务 SLA 数、通知数和关闭通知数。
- 认领记录 payload 补齐 `tenantId` 和 `targetName`，并兼容旧日志中只写入 `targetType/targetID/targetName` 的场景，避免任务 SLA 或通知认领只存在操作日志时无法在独立视图回看。
- 内置 `/dashboard/saasAdmin/page` 在“运营待办批量分派”后新增“运营待办认领记录”表；待办筛选、来源切换、负责人/关键字回车、批量分派完成和 overview 刷新后都会同步刷新认领记录。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“运营待办认领记录”和 `operationQueueAssignments`；普通租户访问认领记录接口返回 `403`；任务 SLA 分派 smoke 会读取 `operationQueueAssignments?source=task_sla&owner=ops-task-sla&keyword=smoke-operation-queue-task-sla-assign`，断言认领汇总和记录回显负责人、来源和备注；启动日志会暴露该路由。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口、筛选字段和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminOperationQueueAssignments|TestSaaSAdminOperationQueueAssign|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/server -run TestSaaSAdminRoutesDispatchWhenConfigured`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assignments.json`，当前源码指纹为 `0b810844c2b22d922ef9c6c9d442f8922b754e92b2d6ba2544b21e7ea13273fb`，纳入文件数为 `616`，生成时间为 `2026-07-10 09:57:52 CST`。

## 阶段补充（2026-07-10 运营待办认领记录 CSV 导出）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级导出能力 `GET /dashboard/saasAdmin/export?type=operationQueueAssignments`，仅平台租户超级管理员可用；普通租户超级管理员导出返回 `403`。
- 导出复用 `operationQueueAssignments` 的筛选口径：`source`、`owner`、`status`、`objectType/targetType`、`objectId/targetId`、`tenantId`、`keyword` 和 `limit`，并兼容 `operation_queue_assignments`、`operationQueueAssigns`、`ops_queue_assignments` 等别名。
- CSV 字段包含操作 ID、租户、来源、对象类型、对象 ID、目标名称、负责人、状态、下次跟进、备注、操作者、操作者租户和认领时间，方便平台运营离线追踪任务 SLA、失败通知和关闭通知的队列级认领审计。
- 内置 `/dashboard/saasAdmin/page` 的“数据导出”区域新增“导出待办认领 CSV”按钮，复用当前运营待办来源、负责人和关键字筛选导出 `mochat-saas-operationQueueAssignments-*.csv`。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“导出待办认领 CSV”；普通租户导出认领记录返回 `403`；任务 SLA 分派到 `ops-task-sla` 后调用 `export?type=operationQueueAssignments&source=task_sla&owner=ops-task-sla&keyword=smoke-operation-queue-task-sla-assign`，解析 CSV 表头并断言认领来源、负责人和备注回显。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增导出类型、页面按钮、筛选字段和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminExportCSVOperationQueue|TestSaaSAdminOperationQueueAssignments|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assignment-export.json`，当前源码指纹为 `8a0547ee8c3ec7f236b21b8bc4a6d82fd068b2d4703018bb3622204fd333dd32`，纳入文件数为 `616`，生成时间为 `2026-07-10 10:06:11 CST`。

## 阶段补充（2026-07-10 运营日报接入待办认领）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/dailyReport` 已接入运营待办认领复盘：日报窗口内会提取 `saas.admin.operation_queue.assign` 操作日志，并按 `source`、负责人、状态、目标对象和认领时间生成 `operationQueueAssignments` 明细。
- 日报 summary 新增 `windowQueueAssignmentCount`、`windowTaskSlaAssignCount`、`windowNotificationAssignCount` 和 `windowClosedNotificationAssignCount`，用于直接判断当天是否有任务 SLA、失败通知或关闭通知被运营认领。
- 日报 JSON 新增 `operationQueueAssignments.filters`、`summary`、`assignmentCount`、`returnedCount` 和 `assignments`，复用独立认领记录视图的解析逻辑，避免同一条认领在列表、负责人工作台、日报和导出中口径不一致。
- 内置 `/dashboard/saasAdmin/page` 的“运营日报明细”新增“日报待办认领”表格，展示认领对象、负责人、状态、备注、租户、操作 ID、下次跟进时间和认领时间；顶部日报卡片同步显示“操作 / 认领 / 账单”的窗口计数。
- `GET /dashboard/saasAdmin/export?type=dailyReport` 新增 `operationQueueAssignment` section，输出来源、状态、负责人、租户、对象、目标名称、下次跟进、认领时间和备注，方便平台运营把当天分派动作随日报一起离线复盘。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：任务 SLA 分派到 `ops-task-sla` 后，会再次读取显式日期窗口的日报 JSON 和日报 CSV，断言 `operationQueueAssignments`、`windowQueueAssignmentCount/windowTaskSlaAssignCount` 和 CSV `operationQueueAssignment` 行回显分派负责人及备注。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐日报待办认领字段、页面入口、CSV section 和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminDailyReport|TestSaaSAdminExportCSVDailyReport|TestSaaSAdminOperationQueueAssign|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'redis:7-alpine': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-daily-operation-queue-assignments.json`，当前源码指纹为 `ae2073b246b06c585ea4eb5251a559199ad48ac910b639286234fb2f082555b1`，纳入文件数为 `616`，生成时间为 `2026-07-10 10:16:08 CST`。

## 阶段补充（2026-07-10 生命周期审计接入待办认领状态）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/tenantLifecycle` 的 operation timeline 已对 `saas.admin.operation_queue.assign` 做状态派生：事件 `status` 会从认领 payload 的 `status` 读取，页面“生命周期审计”和生命周期 CSV 的 `status` 列都能直接显示 `contacted/pending/resolved` 等认领状态。
- 平台管理员现在可用 `source=operation&eventType=operation_queue.assign&status=contacted&keyword=负责人或备注` 从单租户生命周期里定位任务 SLA、失败通知和关闭通知的队列级认领动作；该筛选同样适用于 `GET /dashboard/saasAdmin/export?type=tenantLifecycle`。
- 已补 `TestSaaSAdminTenantLifecycleFiltersOperationQueueAssignmentStatus`：断言生命周期筛选可命中 `saas.admin.operation_queue.assign`，timeline event 状态为 `contacted`，payload 回显负责人、来源和状态，生命周期 CSV 同步输出状态与备注。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：任务 SLA 分派到 `ops-task-sla` 后，会用 `tenantLifecycle?tenantId=$TENANT_ID&source=operation&status=contacted&eventType=operation_queue.assign&keyword=smoke-operation-queue-task-sla-assign` 和同筛选生命周期 CSV 回看，断言单租户审计 timeline、CSV、待办认领记录、日报和操作记录的认领口径一致。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐生命周期审计可按 operation 认领状态筛选、导出和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminTenantLifecycle|TestSaaSAdminExportCSVTenantLifecycle|TestSaaSAdminOperationQueueAssign'`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-lifecycle-operation-queue-assignments.json`，当前源码指纹为 `69684f5e972f49d6811d38b467d167525b3ed04b6bcf53ddc13f389b821bd586`，纳入文件数为 `616`，生成时间为 `2026-07-10 10:27:29 CST`。

## 阶段补充（2026-07-10 运营待办认领到期筛选）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/operationQueueAssignments` 新增 `dueState=all/overdue/due_soon/future/no_date/closed` 筛选，直接基于认领 payload 的 `nextFollowUpAt` 和 `status` 派生到期状态，不新增表、不改写任务或通知事实。
- 认领记录响应新增 `dueState` 字段；summary 新增 `overdueCount`、`dueSoonCount`、`futureCount`、`noDateCount`、`closedCount` 和最早 `nextFollowUpAt`，用于平台运营识别已认领但即将到期或已经逾期的待办。
- `GET /dashboard/saasAdmin/export?type=operationQueueAssignments` 新增 `dueState` CSV 列，并复用同一筛选；`GET /dashboard/saasAdmin/dailyReport` 的 `operationQueueAssignments.summary` 同步带出到期分布，日报 CSV 的 `operationQueueAssignment` remark 也会输出 `dueState`。
- 内置 `/dashboard/saasAdmin/page` 在“运营待办筛选”中新增“认领到期”下拉，认领记录标题会显示逾期、7 天内和最近跟进时间，认领记录行内同时展示业务状态和到期状态；“导出待办认领 CSV”复用当前认领到期筛选。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：任务 SLA 分派时动态生成 15 天后的 `nextFollowUpAt`，随后用 `operationQueueAssignments?dueState=future`、认领 CSV、日报 JSON、日报 CSV、生命周期审计和操作记录回看同一认领动作，断言负责人、来源、状态、到期状态和备注一致。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐认领到期筛选、CSV 字段、页面控件和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminOperationQueueAssignments|TestSaaSAdminExportCSVOperationQueueAssignments|TestSaaSAdminOperationQueueAssign|TestSaaSAdminPageContainsSaaSAdminControls'`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assignment-due-state.json`，当前源码指纹为 `651df6d3f364801a35f95988446a942ed7f4ff11534383f0f3745ccb31ab3fe8`，纳入文件数为 `616`，生成时间为 `2026-07-10 10:40:50 CST`。

## 阶段补充（2026-07-10 运营待办认领到期提醒）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/operationQueueAssignmentNotifications`，仅平台租户超级管理员可用；普通租户超级管理员访问返回 `403`。
- 接口复用 `operationQueueAssignments` 的 `source`、`owner`、`status`、`objectType/objectId`、`tenantId`、`keyword`、`limit` 和 `dueState` 筛选，只允许 `dueState=overdue/due_soon`，未传或传 `all` 时默认提醒逾期认领，避免把未来、无日期或已关闭认领误发提醒。
- 每条可提醒认领会生成 `metric=operation_queue_assignment`、`alertType=operation_queue_assignment_reminder` 的通知 outbox，`periodKey=operation_queue_assignment_{operationId}_{dueState}`，并按通知 key 跳过已有提醒；写入成功后追加 `saas.admin.operation_queue.assignment_notify` 操作日志，payload 包含通知、筛选条件、原认领记录和 `forceCreate`。
- 内置 `/dashboard/saasAdmin/page` 在“运营待办批量分派”区域新增“生成认领到期提醒”按钮，复用当前“认领到期”筛选；触发后刷新认领记录、通知 outbox、操作记录和运营日报。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面入口断言包含“生成认领到期提醒”和 `createOperationQueueAssignmentNotifications`；普通租户调用提醒接口返回 `403`；任务 SLA smoke 会额外造一条已逾期认领，再调用 `operationQueueAssignmentNotifications?source=task_sla&owner=ops-task-sla-overdue&dueState=overdue`，断言通知 outbox、`operation_queue_assignment_reminder` 和 `saas.admin.operation_queue.assignment_notify` 操作日志生成；启动日志会暴露该路由。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、页面入口、alert type、操作日志和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard -run 'TestSaaSAdminOperationQueueAssignmentNotifications|TestSaaSAdminOperationQueueAssignments|TestSaaSAdminPageContainsSaaSAdminControls'`、`env -u GOROOT go test ./internal/server -run 'TestSaaSAdminRoutesDispatchWhenConfigured|TestSaaSAdminRoutesList'`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`./scripts/list_standalone_soak_processes.sh` 未发现 `standalone_soak_24h.sh` 进程。
- `docker info --format '{{.ServerVersion}}'` 仍返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`；`env -u GOROOT ./scripts/standalone_route_coverage.sh` 仍被 Docker daemon 阻断，本轮返回 `unable to get image 'mariadb:10.6': Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮没有运行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke 或 route coverage。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assignment-notifications.json`，当前源码指纹为 `d1115a57957171dac03d2e8882aaf1b2b3d7b4b8980a371c03dc2c55fc1fcdbb`，纳入文件数为 `616`，生成时间为 `2026-07-10 11:26:19 CST`。

## 阶段补充（2026-07-10 运营待办认领关闭与最新状态）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- `GET /dashboard/saasAdmin/operationQueueAssignments` 新增 `currentOnly/current_only` 筛选；`currentOnly=true` 会先按 `source + objectId` 选出最新操作，再执行负责人、状态、到期、对象、租户和关键字筛选，避免旧负责人记录混入当前工作台。
- 新增平台级写入口 `POST/PUT /dashboard/saasAdmin/operationQueueAssignmentClose`，仅平台租户超级管理员可用；请求按当前认领 `operationId` 关闭为 `resolved` 或 `ignored`，清空 `nextFollowUpAt`，保留原负责人和对象用于审计，并追加 `saas.admin.operation_queue.assignment_close` 操作日志，`before/after` 记录前后状态和 `previousOperationId`。
- 关闭接口使用当前认领操作 ID 做并发保护；认领已被重新分派或更新时，旧 `operationId` 返回 `409`，不会覆盖新负责人。普通租户超级管理员访问返回 `403`。
- 统一运营待办在应用最新认领时会忽略已关闭状态，因此关闭后不再用旧认领覆盖队列负责人；页面“认领视图”默认只看当前认领，可切换全部历史，未关闭行提供“完成/忽略”操作。
- `POST/PUT /dashboard/saasAdmin/operationQueueAssignmentNotifications` 已强制 `currentOnly=true`，只提醒每个待办最新的逾期或即将到期认领；历史负责人、已关闭认领和已被重新分派的旧记录不会继续进入通知 outbox。
- 已更新 `scripts/smoke_saas_admin_dashboard.sh`：页面断言增加“认领视图”和关闭函数；普通租户关闭接口返回 `403`；平台 smoke 执行“任务 SLA 逾期认领、生成提醒、关闭认领、当前关闭视图回看、再次催办命中 0、旧负责人不再覆盖待办、关闭操作日志回看”的完整链路；启动日志暴露新增路由。
- 已更新 `README.md` 和 `docs/release-candidate.md`，补齐新增 API、当前认领筛选、并发保护、页面操作、通知去旧记录和 smoke 验收口径。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server`、`env -u GOROOT go test ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/smoke_schema_migrate.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=73 referenced=73`）均通过；`git diff --check` 返回 0；`rg -n "[[:blank:]]$"` 扫描本轮触达文件未发现行尾空白。
- `docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮无法执行依赖 `docker compose` 的完整 SaaS 总后台实际 smoke；该限制不影响已通过的 Go 单元/路由测试和脚本静态门禁，但不能替代容器级运行证据。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，未发现 `standalone_soak_24h.sh` 进程；本轮没有启动 24 小时持续运行。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assignment-close.json`，当前源码指纹为 `904354de57a41d8aa75261785cbb5124c7a233ca34bb0745f06c1b47e501cbf3`，纳入文件数为 `616`，生成时间为 `2026-07-10 11:48:38 CST`。

## 阶段补充（2026-07-10 运营待办认领自动提醒）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增 `SaaSAdminOperationQueueAssignmentReminderCron`，任务名为 `cron-saas-operation-queue-assignment-reminder`；每轮分别扫描最新 `overdue` 和 `due_soon` 认领，并复用手动接口的共享服务生成 `operation_queue_assignment_reminder` 通知 outbox。
- 新增 `MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON`、`MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS`、`MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_RUN_ON_START` 和 `MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT`；默认每小时执行，每个到期状态最多扫描 500 条，启用时要求 MySQL 和有效的平台管理员租户 ID。
- 自动任务固定 `currentOnly=true`，使用系统操作者 `actorUserId=0`、平台租户作为 `actorTenantId`，并写入 `saas.admin.operation_queue.assignment_notify`；历史负责人、未来认领、已关闭认领和已有同 `operationId + dueState + channel` 通知会被跳过。
- 自动任务只负责生成 outbox；实际 webhook 发送继续由 `cron-saas-alert-notification-dispatch` 执行并沿用租户级 webhook、签名、模板和重试策略。
- task runner 会把任务当前状态、运行实例和 periodic tick 写入 `mochat_go_background_tasks`、`mochat_go_background_task_runs`、`mochat_go_background_task_executions`，并通过 `/compat/status.background_tasks` 暴露运行状态。
- 新增 `scripts/smoke_saas_operation_queue_assignment_reminder_cron.sh` 并接入 `standalone_acceptance.sh` 的 cron 套件；smoke 使用真实 MySQL 造当前逾期、当前 7 天内、旧逾期被未来认领覆盖、逾期认领已关闭四类数据，只允许前两类生成通知，并在重启后验证 outbox 和审计日志不重复。
- 已更新 `README.md`、`deploy/standalone/.env.example`、`deploy/standalone/README.md` 和 `docs/release-candidate.md`，补齐启用参数、运行边界、任务名、幂等口径和验收命令。
- 本轮已验证：`env -u GOROOT go test ./...` 全仓通过；`env -u GOROOT go test ./internal/config -count=1` 通过；自动提醒 cron 的生成、最新认领筛选、已有通知去重、完整 notification key 精确查重、系统操作者审计和二次运行幂等均有单元测试覆盖；`bash -n scripts/smoke_saas_operation_queue_assignment_reminder_cron.sh scripts/standalone_acceptance.sh scripts/smoke_saas_admin_dashboard.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=74 referenced=74`）和 `git diff --check` 均通过。
- `docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮无法实际运行新增的真实 MySQL cron smoke；脚本已接入总验收，但容器级证据仍需在 Docker daemon 可用时补跑。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，未发现 `standalone_soak_24h.sh` 进程；本轮没有启动 24 小时持续运行。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-operation-queue-assignment-reminder-cron.json`，当前源码指纹为 `6b96a92ad23f5b3c45d527c31880bf454502e96654d3c01c4dc43d50ab7fbf0d`，纳入文件数为 `619`，生成时间为 `2026-07-10 12:08:28 CST`。

## 阶段补充（2026-07-10 租户通知策略中心）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级 `GET /dashboard/saasAdmin/notificationPolicies`，按 `tenantId`、`state=all/enabled/disabled/unconfigured`、`keyword`、`channel=webhook` 和 `limit` 查看跨租户通知策略；汇总不受列表 limit 影响，返回租户总数、已配置、已启用、已停用、未配置和当前状态命中数。
- 新增 `GET/POST/PUT /dashboard/saasAdmin/notificationPolicy`：平台租户超管可读取或代管租户 Webhook 开关、URL、Secret、请求超时、HTTP 重试、标题/正文模板和 outbox 重试策略；未传 Secret 保留旧值，只有 `clearWebhookSecret=true` 才清空。
- 策略响应只暴露 `webhookSecretConfigured`，不返回 Secret 明文；保存写入 `tenant.notification_policy.update` / `notification_policy` 审计日志，before/after JSON 同样不含密钥。
- 新增 `POST/PUT /dashboard/saasAdmin/notificationPolicyTest`：只允许对已配置且已启用的策略生成 `metric=notification_policy`、`alertType=notification_policy_test` 的 pending 测试通知，沿用租户级 outbox 最大尝试次数，并写入 `tenant.notification_policy.test` 审计；实际 webhook 投递仍由现有 dispatcher 处理。
- 三类通知策略能力均只允许平台租户超管访问，普通租户超管的列表、单项读取、保存和测试请求均返回 `403`。
- 内置 `/dashboard/saasAdmin/page` 新增“租户通知策略”筛选、汇总、租户列表、策略编辑和“生成测试通知”操作；编辑时会先载入租户当前策略，避免用空表单覆盖已有 URL 或密钥。
- 已扩展 `scripts/smoke_saas_admin_dashboard.sh`：从两个未配置租户开始，依次验证策略列表、平台代管保存、密钥脱敏、单项回看、已启用筛选、测试通知入 outbox、两类审计落库、页面入口、启动日志和普通租户越权拒绝。
- 本轮已验证：`env -u GOROOT go test ./internal/dashboard ./internal/server ./internal/store -count=1`、`env -u GOROOT go test ./...`、`env -u GOROOT go vet ./...`、`bash -n scripts/smoke_saas_admin_dashboard.sh scripts/standalone_acceptance.sh`、`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh`（`smoke_scripts=74 referenced=74`）和 `git diff --check` 均通过；本轮触达文件未发现行尾空白。
- `docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮无法实际运行扩展后的真实 MySQL SaaS admin smoke；脚本语法和总验收纳管已验证，但不用它们替代容器级运行证据。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，未发现 `standalone_soak_24h.sh` 进程；本轮没有启动 24 小时持续运行。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-notification-policy-center.json`，当前源码指纹为 `8d315cadfe4cc3f4bd335f6681f6faf2a3a386a5b05aa0f1ab760694b74dcc0a`，纳入文件数为 `621`，生成时间为 `2026-07-10 12:35:34 CST`。

## 阶段补充（2026-07-10 通知策略投递执行）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增迁移 `0036_saas_notification_policy_controls`：扩展租户通知策略的 `minimum_severity`、事件订阅 JSON、免打扰开关与起止时间、IANA 时区、每小时限流，并为通知 outbox 增加租户、通道、状态、送达时间联合索引；默认值保持升级前的全类型、warning 及以上、不启用免打扰和不限流行为。
- 新增共享投递决策引擎，即时告警投递和 outbox dispatcher 使用同一口径：策略停用、事件未订阅或低于最低严重级别写为 `suppressed`；命中免打扰或滚动一小时上限时保持 `pending`，推迟 `next_retry_at`；策略处理不增加 `attempts`。
- 每小时限流按租户和通道统计最近一小时实际 `delivered` 的业务通知，并返回下一可投递时间；`notification_policy_test` 不计入业务限流。策略测试通知会绕过事件订阅、最低严重级别、免打扰和限流，但仍经过 outbox、dispatcher、租户 Webhook、签名和真实 HTTP 投递。
- 租户自助告警页和 SaaS 总后台策略中心均已增加最低严重级别、常用/自定义事件订阅、免打扰起止时间、时区和每小时上限控件；总后台通知列表新增 `suppressed` 筛选与汇总，Secret 仍只返回是否已配置，保存和测试审计不包含密钥明文。
- `cmd/mochat-saas-maintenance -action dispatch-alert-notifications` 和通知 cron 日志新增 `deferred`、`suppressed` 计数；通知汇总、运营日报通知摘要和总后台页面同步暴露 `suppressedCount`。
- 已扩展 `scripts/smoke_saas_alert_setting_dispatch.sh`，覆盖签名投递、事件类型抑制、严重级别抑制、跨午夜免打扰延期、小时限流和策略测试通知绕过；已扩展 `scripts/smoke_saas_admin_dashboard.sh`，覆盖策略控制字段持久化、页面入口和 `suppressed` 状态查询；`scripts/smoke_schema_migrate.sh` 已按 36 个版本校验 0036 apply/status/baseline/legacy 升级及字段、索引结构。
- 本轮已验证：新增策略核心单元测试通过；`env -u GOROOT go test ./...` 全仓通过；`env -u GOROOT go vet ./...` 通过；`env -u GOROOT ./scripts/lint_mysql57_schema.sh` 返回 `mysql 5.7 schema lint passed (71 files)`；相关脚本 `bash -n` 通过；`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 返回 `smoke_scripts=74 referenced=74`；`git diff --check` 返回 0，本轮触达文件未发现行尾空白。
- `docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮未实际运行依赖 Docker 的迁移、通知投递和 SaaS 总后台真实 MySQL smoke；已完成的静态迁移门禁、单元测试和脚本纳管不能替代该容器级运行证据。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，未发现 `standalone_soak_24h.sh` 进程；本轮没有启动 24 小时持续运行。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-notification-policy-enforcement.json`，当前源码指纹为 `39efb9eb28f0d12c500a35de071fd1375ce5dd1f6cdc2c7785f9fbe2821d290a`，纳入文件数为 `625`，生成时间为 `2026-07-10 13:12:12 CST`。

## 阶段补充（2026-07-10 跨租户通知送达健康度中心）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/notificationHealth`，仅平台租户超级管理员可用；支持 `tenantId`、`keyword`、`state=all/healthy/warning/critical/no_data`、`hours`、`staleMinutes`、`channel=webhook` 和 `limit` 筛选，并返回跨租户健康汇总、租户明细和失败原因排行。
- 健康统计区分 `delivered`、`failed`、`dead`、`pending`、可立即重试、策略延期、积压超时、`closed` 和 `suppressed`；送达成功率只以已完成尝试的 `delivered + failed + dead` 为分母，策略抑制、已关闭和未来延期通知不被误算为投递失败。
- 健康状态规则已固定：存在死信、超时积压或样本不少于 5 且成功率低于 80% 为 `critical`；存在失败、可立即重试积压或样本不少于 5 且成功率低于 95% 为 `warning`；窗口内没有通知为 `no_data`；其余为 `healthy`。同时返回平均/最大送达延迟、总尝试次数、最后通知/送达/失败时间和脱敏后的主要失败原因。
- 新增 `GET /dashboard/saasAdmin/export?type=notificationHealth`，复用同一筛选与判定口径导出 CSV；内置 `/dashboard/saasAdmin/page` 新增健康窗口、状态、积压阈值和关键字筛选，以及租户健康表、失败原因表和“导出通知健康 CSV”入口。
- 新增迁移 `0037_saas_notification_health_index`，为通知 outbox 增加 `(tenant_id, channel, created_at, status)` 联合索引；`scripts/smoke_schema_migrate.sh` 已按 37 个版本覆盖 apply、checksum、status、重复执行、baseline、legacy 升级和索引结构检查。
- 已扩展 `scripts/smoke_saas_admin_dashboard.sh`：构造送达、失败、死信、超时 pending 和 suppressed 通知，断言健康汇总、成功率、延迟、失败原因、CSV、页面入口、启动路由和普通租户 `403`；验收套件继续完整纳管 74 个 smoke 脚本。
- 本轮已验证：通知健康度定向单元/路由/页面测试通过；`env -u GOROOT go test ./...` 全仓通过；`env -u GOROOT go vet ./...` 通过；`env -u GOROOT ./scripts/lint_mysql57_schema.sh` 返回 `mysql 5.7 schema lint passed (73 files)`；相关脚本 `bash -n` 通过；`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 返回 `smoke_scripts=74 referenced=74`；最终页面脚本经 `node --check` 校验通过；`git diff --check` 返回 0。
- `docker info --format '{{.ServerVersion}}'` 返回 `Cannot connect to the Docker daemon at unix:///Users/lv/.docker/run/docker.sock. Is the docker daemon running?`，因此本轮未实际运行依赖 Docker 的 0037 迁移和 SaaS 总后台真实 MySQL smoke；已通过的单元测试、MySQL 5.7 静态门禁和脚本纳管不能替代该容器级运行证据。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，未发现 `standalone_soak_24h.sh` 进程；本轮没有启动 24 小时持续运行。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-notification-health.json`，当前源码指纹为 `77f670ad28ed5eb9f4d8e62e85cd249ba73b9cd01371ce859a6a9d75caaa139e`，纳入文件数为 `629`，生成时间为 `2026-07-10 13:36:03 CST`。

## 阶段补充（2026-07-10 通知健康运营闭环与真实容器验收）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 统一运营待办新增 `notification_health` 来源：只纳入 `warning/critical` 租户，分别映射为高/紧急优先级；队列、负责人工作台、认领记录、CSV 和运营日报均统计通知健康待办及分派数量。
- `POST/PUT /dashboard/saasAdmin/operationQueueAssign` 已支持通知健康待办，认领写入 `saas.admin.operation_queue.assign`，总后台健康面板可直接填写负责人和复查时间并完成分派。
- 新增平台级 `POST/PUT /dashboard/saasAdmin/notificationHealthRecovery`：重新计算当前通知健康认领，只在租户变为 `healthy`、存在实际尝试且至少有一条成功送达时自动关闭；`warning`、`critical`、`no_data`、租户不存在及只有 suppressed/closed 而没有成功投递证据的租户保持打开。
- 自动恢复复用运营认领关闭链路，写入 `saas.admin.operation_queue.assignment_close`，`closeContext.reason=notification_health_recovered` 并保留健康快照；重复执行识别已关闭记录，不重复写入关闭审计。普通租户超级管理员调用恢复接口返回 `403`。
- 内置 `/dashboard/saasAdmin/page` 新增“分派健康待办”和“复查并关闭已恢复”操作，完成后同步刷新健康表、待办队列、负责人、认领记录、操作日志和运营日报。
- `scripts/smoke_saas_admin_dashboard.sh` 已改为真实 JWT 登录和 Redis 会话，不再使用开发身份请求头；续费日期改为相对当天生成，客户成功通知断言覆盖“首次入队或幂等跳过”，通知健康恢复后的队列检查限定目标租户，避免掩盖平台租户的独立告警。
- 已在 Docker Server `28.5.1` 上通过 `env -u GOROOT ./scripts/smoke_saas_admin_dashboard.sh`：从空库执行迁移、bootstrap、平台/租户登录，再完整验证 SaaS 总后台及通知健康分派、成功送达、自动恢复、重复恢复幂等和关闭审计。
- 已通过 `env -u GOROOT ./scripts/smoke_schema_migrate.sh`，37 个迁移的全新库 apply、checksum、重复执行、baseline、旧库升级及 `0037` 健康索引结构均通过真实 MySQL 校验。
- 已通过 `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh`：独立模式在无效 PHP upstream、无旧源码和无旧 manifest 条件下正常 ready，原 PHP 清单 `224/224` 路由由 Go 覆盖，缺口为 `0`。
- 最终门禁通过：`env -u GOROOT go test ./...`、`env -u GOROOT go test ./internal/dashboard ./internal/server -count=1`、`env -u GOROOT go vet ./...`、`env -u GOROOT ./scripts/lint_mysql57_schema.sh`（`73 files`）、相关脚本 `bash -n`、页面 JavaScript 语法检查、Go 格式检查和行尾空白检查；`env -u GOROOT ./scripts/audit_acceptance_suite_coverage.sh` 返回 `smoke_scripts=74 referenced=74`。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，确认未运行 `standalone_soak_24h.sh`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-notification-health-operations.json`，当前源码指纹为 `7bedea68a06b2c7528bbe425ec7f0d9c91c4a8831537f52d9e228e8c5e6627d6`，纳入文件数为 `631`，生成时间为 `2026-07-10 15:36:14 CST`。

## 阶段补充（2026-07-10 通知健康自动恢复与 compose 交付修复）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增 `SaaSAdminNotificationHealthRecoveryCron`，task runner 名称为 `cron-saas-notification-health-recovery`；默认每 15 分钟按 24 小时通知窗口和 15 分钟 pending 积压阈值复查当前 `notification_health` 认领。
- 自动任务复用手动 `notificationHealthRecovery` 服务，只关闭状态为 `healthy`、存在实际投递尝试且至少成功送达一次的租户；`warning/critical`、`no_data`、租户不存在和只有 suppressed/closed/延期而没有成功送达证据的认领保持打开。
- 自动关闭使用 `actorUserId=0` 和平台租户写入 `saas.admin.operation_queue.assignment_close`，保留 `closeContext.reason=notification_health_recovered`、健康快照和筛选窗口；已关闭认领在后续 tick 或进程重启后只计入 `already_closed`，不重复写入关闭审计。
- 新增 `MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON`、周期、run-on-start、窗口和积压阈值配置；启用时要求 MySQL 和正数平台租户 ID，窗口限制 1 至 720 小时，积压阈值限制 1 至 10080 分钟。任务状态、运行实例和 periodic tick 写入三张后台任务表并暴露在 `/compat/status`。
- 新增 `scripts/smoke_saas_notification_health_recovery_cron.sh` 并纳入 cron 验收套件；脚本从独立空库执行 37 个迁移，构造已恢复、仍异常、无数据、只有 suppressed 四类租户，只允许第一类自动关闭，并验证系统审计、任务状态、执行历史和重启幂等。真实 MySQL smoke 已通过。
- 修复 standalone compose 交付：`app` 服务现在显式透传 SaaS 总后台、认领提醒和通知健康自动恢复变量；MySQL 新卷初始化从原先只挂载到 `0032` 修正为继续挂载 `0033` 至 `0037`，避免全新部署缺少操作日志、账单、运营任务、策略控制和健康索引。
- `scripts/smoke_standalone_compose_app.sh` 已扩展为默认启用 SaaS 总后台和健康恢复任务，断言总后台页面、`0033/0037` baseline、操作日志表、健康索引、task runner 运行状态和启动即跑日志；全新 app + MySQL + Redis 镜像构建、bootstrap、真实登录和业务 API smoke 已通过。
- 将 `tenant_renewal`、`admin_task_sla`、`operation_queue_assignment` 和 `notification_policy` 从套餐配额常量拆为 `SaaSEventMetric*` 通知事件分类；套餐资源仍严格保持 26 项，不为通知事件伪造额度。`audit_saas_metric_coverage.sh` 已恢复为 `metrics=26` 通过。
- 功能模块矩阵新增“SaaS 总后台与平台运营”，将全部 73 条 `/dashboard/saasAdmin` 运行时路径归入源码与三层 smoke 证据；矩阵现为 `modules=29`、manifest `213/213`、runtime `479/479`。
- 最终通过 `env -u GOROOT ./scripts/test.sh`、`env -u GOROOT go test ./...`、`env -u GOROOT go vet ./...`、MySQL 5.7 schema lint、Go 格式、Shell 语法、行尾空白、功能模块矩阵、26 项配额覆盖和验收纳管审计；当前 `smoke_scripts=75 referenced=75`。
- 已复跑独立模式路由覆盖：原 PHP manifest `224/224` 已由 Go 覆盖，`missing_route_total=0`；未使用 PHP upstream、旧源码或外部 manifest。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，确认未运行 `standalone_soak_24h.sh`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-notification-health-recovery-cron.json`，当前源码指纹为 `1bca83ea259c027030bfe57395aa1c15808c27878197ba571c57eaeea5cf5edf`，纳入文件数为 `635`，生成时间为 `2026-07-10 16:06:23 CST`。

## 阶段补充（2026-07-10 通知 SLO 趋势与总后台完善）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增平台级只读接口 `GET /dashboard/saasAdmin/notificationSlo`，仅平台租户超级管理员可用；支持租户、套餐关键字、1 至 90 天窗口、送达成功率目标、目标送达秒数、时延达标率目标和返回上限筛选，普通租户访问返回 `403`。
- SLO 按通知 `created_at` cohort 聚合并补齐窗口内每个自然日，同时返回日趋势和租户排行。成功率为 `delivered / (delivered + failed + dead + closed)`，目标时延达标率为目标秒数内送达数除以送达数；`pending/suppressed` 单列，无实际尝试返回 `no_data`。人工关闭计入未送达，避免结案让历史失败消失。
- MySQL Store 同步返回数据库会话实际使用的窗口起止日，报告按该边界补齐自然日，不再用宿主 Go 时区自行推导 SQL 窗口；已补单元测试覆盖 MariaDB 为 UTC、Go 为 Asia/Shanghai 时北京时间凌晨不会错一天。
- 新增 `GET /dashboard/saasAdmin/export?type=notificationSlo`，复用同一查询和目标值，按 `summary/day/tenant` 三类行导出 CSV；API 和 CSV 共用报告构建逻辑。
- 内置 `/dashboard/saasAdmin/page` 新增 7/30/90 天切换、成功率目标、送达时延、时延达标率、租户搜索、日趋势、租户排行和 SLO CSV 导出。Playwright 在 1440x1000 与 390x844 视口完成检查；修复了桌面端导出按钮文本被固定末列截断的问题，移动端筛选区 `scrollWidth=clientWidth=368`，无横向溢出。
- 新增迁移 `0038_saas_notification_slo_index`，为通知 outbox 增加 `(channel, created_at, tenant_id, status, delivered_at)` 平台级窗口索引；standalone compose 已挂载 `0038`，迁移 smoke 覆盖 apply、checksum、status、重复执行、baseline、legacy 升级和 5 列索引结构。
- 新增 `scripts/smoke_saas_notification_slo.sh` 并纳入 SaaS 验收套件；脚本先断言 standalone compose 首启的默认 `mochat` 库已通过 `038-saas-notification-slo-index.sql` 建出 5 列索引，再从独立空库执行 38 个迁移，真实启动 Go + MySQL + Redis，开通平台、违约、达标和无数据租户，构造跨两天通知样本，并验证成功率、时延达标率、补齐无数据日期、租户排行、`closed` 口径、关键字筛选、CSV、普通租户 `403`、页面、路由和索引。加深后的真实 smoke 已通过。
- 已通过 `env -u GOROOT ./scripts/smoke_schema_migrate.sh`，38 个迁移的全新库 apply、checksum、status、重复执行、baseline、旧库升级及 `0038` SLO 索引结构均通过真实 MySQL 校验。
- 已通过 `env -u GOROOT ./scripts/test.sh`，包含全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性、26 项配额、功能模块矩阵和验收纳管审计；功能矩阵为 `modules=29`、manifest `213/213`、runtime `480/480`，验收纳管为 `smoke_scripts=76 referenced=76`，MySQL 5.7 schema lint 覆盖 `75 files`。
- 已复跑独立模式路由覆盖：原 PHP manifest `224/224` 已由 Go 覆盖，`missing_route_total=0`，本轮新增 SLO 路由属于 Go 独立版平台运营能力。
- `env -u GOROOT ./scripts/smoke_standalone_compose_app.sh` 连续两次在构建业务镜像前被外部 Docker 代理阻断：获取 `golang:1.26-alpine` 和 `alpine:3.22` manifest 均返回 EOF，本机也没有这两个基础镜像缓存，因此本轮无法重建完整 app 镜像。当前源码、页面、新 Go 二进制、真实 MySQL/Redis、compose 默认库 `0038` 初始化和 migration baseline 分别已有通过证据，但完整 app 镜像短链路仍需在镜像源恢复后复跑。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，确认本轮未运行 `standalone_soak_24h.sh`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-notification-slo.json`，当前源码指纹为 `1c98a91c52926a5398de3bc0e0c9d11a806f05448de5888221e2a3ca90196648`，纳入文件数为 `640`，生成时间为 `2026-07-10 17:23:05 CST`。

## 阶段补充（2026-07-10 订阅生命周期与访问控制闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求未启动 24 小时持续运行。
- 新增迁移 `0039_saas_subscription_lifecycle`，创建 `mochat_go_saas_subscriptions` 当前订阅表和 `mochat_go_saas_subscription_events` 事件表；当前表保存套餐快照、计费周期、试用/当前周期/宽限期时间、期末取消、最新账单、乐观锁版本和状态原因，事件表保存前后状态、操作人、来源、幂等键和载荷。
- 升级回填只处理已有租户套餐：无期套餐回填 `active/lifetime`，有限期套餐默认保留 7 天宽限期，停用租户或套餐回填 `suspended`；不为没有套餐的存量租户创建阻断访问的订阅，保留旧套餐到期兼容口径。
- 订阅六态已统一为 `trialing/active/grace/past_due/suspended/canceled`；`trialing/active/grace` 允许访问，`past_due/suspended/canceled` 同时拦截新登录和旧 token 的 `UserByID` 链路，分别返回欠费、暂停或取消原因。租户本身停用优先映射为有效 `suspended`，平台管理租户不能被迁移或校准到阻断访问状态。
- 新增平台级 `GET /dashboard/saasAdmin/subscriptions`、`GET /dashboard/saasAdmin/subscriptionEvents`、`POST/PUT /dashboard/saasAdmin/subscriptionTransition` 和 `POST/PUT /dashboard/saasAdmin/subscriptionReconcile`；列表支持状态、访问权、套餐、租户和关键字筛选，迁移支持 `expectedVersion` 乐观锁、`idempotencyKey` 幂等重放、六态迁移图和期间字段校验，订阅事件与 `tenant.subscription.transition/reconcile` 操作日志在同一事务落库。
- 订阅已与平台业务写链路联动：开户创建订阅，套餐调整同步套餐与周期快照，续费激活订阅并回写 `latest_billing_event_id`，租户停用转为 `suspended`，重新启用恢复停用前状态后再按当前时间重算。
- 新增 `cron-saas-subscription-reconcile`，默认每 300 秒扫描 500 条，支持 run-on-start；会把试用或正常订阅按到期时间转为宽限期/欠费，把宽限期结束转为欠费，把期末取消转为已取消。单个租户失败不阻断同批其他租户，任务状态、运行实例和 periodic tick 写入后台任务账本。
- 内置 `/dashboard/saasAdmin/page` 新增“订阅生命周期”区域，包含筛选、六态与访问权汇总、订阅列表、受控迁移表单、事件时间线、dry-run/应用校准和 `export?type=subscriptions` CSV。Playwright 已在 `1440x1000` 和 `390x844` 视口检查；修复了桌面端“应用迁移”按钮溢出，最终桌面迁移区 `scrollWidth=clientWidth=1218`，移动筛选与迁移区均为 `368/368`，页面无横向溢出或 JavaScript 运行错误。
- 新增 `scripts/smoke_saas_subscription_lifecycle.sh` 并纳入 SaaS 验收套件；真实 MySQL + Redis + Go smoke 已通过，覆盖默认 compose 库 `0039` 表、全 39 迁移、试用/宽限期/欠费登录、普通租户越权 `403`、乐观锁 `409`、幂等重放、租户开停、续费账单、预演/应用校准、平台租户保护、事件审计、CSV、页面路由与 cron 启动即跑。
- `env -u GOROOT ./scripts/smoke_schema_migrate.sh` 已通过，39 个迁移的全新库 apply、checksum、status、重复执行、baseline、rollback、legacy 升级和 `0039` 表/索引结构均通过真实 MySQL 校验。
- 最终通过 `env -u GOROOT ./scripts/test.sh`，包含全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性、MySQL 5.7 静态兼容、26 项配额、企微事件、队列和存储回收审计；功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `484/484`、runtime route entries `565`，SaaS 总后台模块包含 78 条运行时路径；验收纳管为 `smoke_scripts=77 referenced=77`，MySQL 5.7 schema lint 覆盖 `77 files`。
- `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh` 已通过；原 PHP manifest `224/224` 路由全部由 Go 独立模式承接，`missing_route_total=0`，新增平台路由作为 Go 独立版扩展能力不改动原清单。
- `docker compose ... config --quiet` 已通过；`env -u GOROOT ./scripts/smoke_standalone_compose_app.sh` 已使用当前源码成功重建 Go 业务镜像，启动 app + MySQL + Redis，完成 `0033` 至 `0039` 初始化、订阅两表、migration baseline、bootstrap、真实登录和业务 API 闭环；上一阶段 Docker 代理 EOF 已恢复，本轮不再存在完整 app 镜像未验证缺口。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-subscription-lifecycle.json`，当前源码指纹为 `89bb41cb112450b8a51cc9171567d0d6373a2648fdae81c9e1b11bbb0036b1c7`，纳入文件数为 `647`，生成时间为 `2026-07-10 18:19:55 CST`。

## 阶段补充（2026-07-10 支付收款与催缴闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求未启动 24 小时持续运行。
- 新增迁移 `0040_saas_payment_collection`，创建 `mochat_go_saas_payment_orders` 和 `mochat_go_saas_payment_webhook_events`；订单账本保存套餐、金额、币种、服务期、支付状态、催缴时间和乐观锁版本，回调账本保存提供方事件、载荷摘要和处理结果。
- 新增支付提供方中立入口 `POST /webhooks/saas/payment`，使用 `X-Mochat-Go-Payment-Timestamp` 和 `X-Mochat-Go-Payment-Signature` 完成 HMAC-SHA256 验签，按提供方事件 ID 幂等处理 `processing/succeeded/failed/canceled`；这个公开边界不依赖 Dashboard JWT，也不把具体支付渠道 SDK 耦合到核心收款域。
- 新增平台级 `paymentOrders`、`paymentWebhookEvents`、`paymentOrder`、`paymentOrderCancel` 和 `paymentDunning` 接口；创单支持幂等键，取消支持 `expectedVersion` 乐观锁，已支付订单忽略迟到失败/取消事件。成功回调在同一 MySQL 事务内结算订单、续费账单、租户套餐、订阅和平台操作日志，欠费租户在支付成功后恢复访问。
- 支付失败会把订单转为可催缴状态；手动 dry-run/应用和 `cron-saas-payment-dunning` 共用同一服务，幂等写入 `payment_failed_reminder` 通知 outbox 与审计日志。真实 smoke 暴露的宿主时区与 MySQL 会话时区 8 小时偏差已修复：催缴调度统一使用数据库 `NOW()` 和 `TIMESTAMPADD`。服务期、支付时间与事件时间也增加 MySQL 5.7 `TIMESTAMP` 范围校验。
- 内置 `/dashboard/saasAdmin/page` 新增收款汇总、订单筛选/创建/取消、催缴预演/应用、支付回调时间线和收款 CSV；租户通知策略与租户告警页都显式提供“支付失败”事件订阅。Playwright 已在 `1440x1000` 和 `390x844` 视口检查：桌面收款区为 `1220/1220`，移动筛选与表单为 `368/368`，只有表格包装层内部横向滚动，页面无全局溢出或 JavaScript 错误。
- 新增 `scripts/smoke_saas_payment_collection.sh` 并纳入 SaaS 验收套件；真实 MySQL + Redis + Go smoke 已通过，覆盖 compose 默认库的 0040 表、全 40 迁移、JWT 权限、欠费拦截、创单幂等/冲突、时间边界、签名失败/成功回调、重复事件、催缴、乐观锁取消、原子结算、访问恢复、列表、CSV、页面、路由和 cron 启动即跑。
- 最终通过 `env -u GOROOT ./scripts/test.sh`，包含全部 Go 测试、`go vet ./...`、五个命令构建、独立性和静态审计；验收纳管为 `smoke_scripts=78 referenced=78`，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `490/490`、runtime route entries `574`，SaaS 总后台模块包含 84 条运行时路径。
- `env -u GOROOT ./scripts/smoke_schema_migrate.sh` 已通过 40 个迁移的空库 apply、checksum、status、重复执行、baseline、rollback 和 legacy 升级；`env -u GOROOT ./scripts/smoke_standalone_compose_app.sh` 已使用当前源码重建并通过 app + MySQL + Redis 完整容器闭环；`MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh` 返回原 PHP manifest `224/224`、缺口 `0`。
- `env -u GOROOT ./scripts/lint_mysql57_schema.sh` 返回 `mysql 5.7 schema lint passed (79 files)`。本机为 arm64，`scripts/smoke_mysql57_schema_migrate.sh` 按设计跳过 `mysql:5.7` amd64/QEMU 容器，避免初始化段错误；真实 MySQL 5.7 容器门禁仍需在 amd64 CI 或显式设置 `MOCHAT_FORCE_MYSQL57=1` 时执行。
- `./scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，没有残留 `standalone_soak_24h.sh` 进程；支付 smoke 容器、临时页面预览服务和 Playwright 会话均已清理。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-payment-collection.json`，当前源码指纹为 `3c25b06089ef5980d01532ad0a3b4b7b5335c249580ed8bc23dee96f772fed29`，纳入文件数为 `654`，生成时间为 `2026-07-10 19:52:31 CST`。

## 阶段补充（2026-07-10 支付退款、财务冲销与订阅权益闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求未启动 24 小时持续运行。
- 新增迁移 `0041_saas_payment_refunds`：创建 `mochat_go_saas_payment_refunds` 退款账本，为支付订单增加退款预占金额、已退款金额和最近退款 ID，为支付回调增加退款 ID、平台退款单和渠道退款单；compose 新库初始化、迁移 apply/status/baseline/legacy 升级已纳管，并提供对应 down rollback 脚本。
- 新增平台级 `GET /dashboard/saasAdmin/paymentRefunds`、`POST/PUT /dashboard/saasAdmin/paymentRefund` 和 `POST/PUT /dashboard/saasAdmin/paymentRefundCancel`；仅平台租户超级管理员可用。退款申请支持平台退款单、渠道退款单、业务幂等键、部分/全额金额、原因、备注和 `expectedVersion` 乐观锁取消。
- 退款申请会在支付订单事务内预占可退金额，订单行锁与条件更新共同阻止并发超退；失败或取消退款释放预占，成功退款把预占转为累计已退款。相同幂等键重放返回原退款，不同请求复用幂等键返回冲突。
- 支付提供方中立回调入口已支持 `refund.processing/failed/canceled/succeeded`，继续使用时间戳与 HMAC-SHA256 验签、提供方事件 ID 和载荷摘要幂等。失败校验事件也会关联退款与支付订单，终态后的迟到事件标记为 ignored，重复成功事件不会重复写账单。
- 退款成功在同一 MySQL 事务写入正金额 `refund` 账单事件并按负向金额计入财务口径，同时更新退款、订单、回调、订阅事件和平台操作日志。支付订单汇总、经营指标、月度趋势、运营日报和页面统一展示毛收入、退款额与净收入；退款事件不参与续费价格估算。
- 退款后的订阅权益默认 `keep`，部分退款不能静默停服；只有补足整单全额退款且没有其他待处理预占时，才允许显式选择 `suspend` 或 `cancel`。全额退款回调会事务化迁移订阅并使新登录和旧 token 访问失效。
- 内置 `/dashboard/saasAdmin/page` 新增退款筛选、汇总、列表、申请表单、订单快捷发起、乐观锁取消、退款回调追溯和 `export?type=paymentRefunds` CSV；付款订单、付款汇总、回调表和账单视图同步显示可退、待退、已退及退款单信息。
- 新增 `scripts/smoke_saas_payment_refunds.sh` 并纳入 SaaS 验收套件。真实 Go + MySQL/MariaDB + Redis smoke 已通过，覆盖 compose 默认库 0041、全 41 迁移、普通租户 `403`、签名失败、并发超退、版本冲突、错误回调关联、失败释放、处理中、部分退款、重复/摘要冲突、迟到事件、超额拦截、全额退款取消订阅、旧 token 失效、净收入归零、列表、CSV、页面和路由。
- 回归已通过：`scripts/smoke_saas_payment_collection.sh`、`scripts/smoke_saas_subscription_lifecycle.sh`、`scripts/smoke_schema_migrate.sh` 和 `scripts/smoke_standalone_compose_app.sh`；当前源码已成功重建 app 镜像并完成 Go app + MySQL + Redis、0041 初始化、baseline、bootstrap、真实登录和业务 API 容器闭环。
- 最终 `scripts/test.sh` 通过，包含全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性与静态审计；验收纳管为 `smoke_scripts=79 referenced=79`，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `493/493`、runtime route entries `579`，SaaS 总后台模块包含 87 条运行时路径。MySQL 5.7 静态门禁覆盖 81 个 schema/迁移文件并通过。
- Playwright 已在 `1440x1000` 与 `390x844` 视口检查退款区域；两种视口均为 `scrollWidth=innerWidth`，无页面级横向溢出，移动端表单宽 370px、提交按钮宽 340px，未发现文本遮挡或 JavaScript 运行错误。临时预览中的 404 仅来自未提供业务 API 与 favicon，预览服务和浏览器会话已清理。
- `scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，确认没有残留 `standalone_soak_24h.sh` 进程。本机仍未把 arm64 下跳过的真实 MySQL 5.7 amd64 容器结果当作生产证据，严格门禁继续由 amd64 CI 承担。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-payment-refunds.json`，当前源码指纹为 `ddf8b59d19ffcf096b2bbc2fed5b4a922bff6c61158c4f4ba259f0fe2bf5c7db`，纳入文件数为 `660`，生成时间为 `2026-07-10 20:41:37 CST`。

## 阶段补充（2026-07-10 发票、红冲与租户账单中心闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮按用户要求不启动 24 小时持续运行。
- 新增迁移 `0042_saas_billing_invoices`：创建租户唯一的 `mochat_go_saas_billing_profiles` 开票资料表和 `mochat_go_saas_invoice_documents` 蓝票/红票单据表，为支付订单增加蓝票预占、已开蓝票、红票预占、已红冲和最近单据 ID；提供对应 down rollback，并挂入 standalone compose 全新库初始化。
- 新增租户自助账单中心开关 `MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL` 与 `/dashboard/saasBilling/page`。租户超级管理员可查看本租户净收款、订单、退款、可开票额和单据，维护开票资料、申请蓝票及取消待处理申请；服务端只使用 dashboard JWT 的租户 ID，忽略请求中的跨租户覆盖参数。
- 新增平台级 `invoiceProfile`、`invoiceDocuments`、`invoice`、`creditNote` 和 `invoiceTransition` API，以及 `export?type=invoiceDocuments` CSV；平台总后台新增开票资料代管、蓝票/红票筛选、申请、开具、失败、取消、提供方单号和单据追溯界面。
- 蓝票和红票都使用支付订单行锁、条件更新、金额预占、幂等键和乐观锁控制并发。蓝票只能基于成功收款申请；退款、待开与已开金额共同限制可开票额；红票只能关联已开蓝票和已成功退款，累计红冲不能超过蓝票金额或可红冲退款额。
- 状态机统一为 `pending/processing/issued/failed/canceled`。租户只能取消自己的 `pending` 蓝票；平台负责处理、开具、失败和取消。失败或取消会在同一事务释放预占，开具会把预占转为累计金额；蓝票减红票的净开票额始终与订单净收款口径对齐。
- MySQL 重复键已统一映射为业务 `409`：并发首次保存开票资料、重复提供方单号不再暴露原始数据库 `500`；单据状态、金额和订单累计更新保持整笔事务回滚。
- 平台支付、退款和发票接口的成功响应统一为业务 `code=200`，修复总后台把成功结果显示为字符串 `ok` 的问题；平台租户详情页不再错误请求自身业务租户生命周期。租户账单页保存 token 后自动刷新，退款成功状态显示为“已退款”。
- Playwright 已在 `1440x1000` 与 `390x844` 视口完成平台发票中心和租户账单中心检查；页面无全局横向溢出，移动端表单单列，表格只在自身容器内滚动，未发现 JavaScript 运行错误。
- 新增 `scripts/smoke_saas_billing_invoices.sh` 并纳入 SaaS 验收套件。真实 Go + MySQL/MariaDB + Redis smoke 已从空库通过，覆盖全 42 个迁移、开票资料版本冲突、蓝票申请/取消/开具、处理中限制、退款预占、红票开具、失败释放、跨租户 `404`、并发防超开、重复提供方单号 `409`、事务回滚、CSV、页面和运行时路由。
- `env -u GOROOT ./scripts/smoke_schema_migrate.sh` 已通过 42 个迁移的空库 apply、checksum、status、重复执行、baseline、rollback 和 legacy 升级；`0042` 两张表、订单开票金额列和唯一索引均通过真实数据库结构断言。
- 最终 `env -u GOROOT ./scripts/test.sh` 通过，包含全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性和静态审计；验收纳管为 `smoke_scripts=80 referenced=80`，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `506/506`、runtime route entries `602`，SaaS 总后台与平台运营模块包含 100 条运行时路径。MySQL 5.7 静态门禁覆盖 83 个 schema/迁移文件并通过。
- `env -u GOROOT ./scripts/smoke_standalone_compose_app.sh` 已使用当前源码成功重建业务镜像，启动 Go app + MySQL + Redis，完成 `0033` 至 `0042` 初始化、开票资料/单据表、migration baseline、bootstrap、真实登录和核心业务 API 容器闭环。
- `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 env -u GOROOT ./scripts/standalone_route_coverage.sh` 已通过，原 PHP manifest `224/224` 路由全部由 Go standalone 承接，`missing_route_total=0`；新增账单与发票路由属于 Go 独立 SaaS 扩展能力。
- `scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，确认没有运行或残留 `standalone_soak_24h.sh`；专项 smoke、Compose app、路由验证和 Playwright 临时环境均已清理。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-billing-invoices.json`，当前源码指纹为 `2b49af73cd6035474a6478bf8cad4eead56f51ef5caa13f937a7a93e53ddccb1`，纳入文件数为 `669`，生成时间为 `2026-07-10 21:55:28 CST`。

## 阶段补充（2026-07-10 支付渠道结算对账与关账闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮不启动新的 24 小时持续运行。
- 新增迁移 `0043_saas_payment_settlements`：创建 `mochat_go_saas_payment_settlement_batches` 渠道结算批次表和 `mochat_go_saas_payment_settlement_entries` 结算明细表，提供同名 down rollback，并挂入 standalone compose 新库初始化。批次保存来源 SHA-256、结算周期、匹配/差异/处理汇总、带符号金额和关账审计；明细保存渠道流水、支付/退款标识、内部匹配、预期值、差异代码、人工处理和原始 JSON。
- 新增平台级 `paymentSettlementBatches`、`paymentSettlementEntries`、`paymentSettlementImport`、`paymentSettlementReconcile`、`paymentSettlementResolve` 和 `paymentSettlementTransition` API。全部仅平台租户超级管理员可用，普通业务租户返回 `403`；导入支持 JSON 和真实 CSV，单次限制 8 MiB/5000 条，原始内容摘要、批次号、渠道批次号与渠道流水共同保证幂等和唯一性。
- 对账同时支持收款和退款，收款金额必须为正、退款金额必须为负，净额必须等于交易金额加手续费。支付按平台订单号/渠道订单号匹配，退款按平台退款单/渠道退款单匹配；双标识指向不同内部记录时标记 `identifier_conflict`，并区分 `missing_internal/status_mismatch/currency_mismatch/amount_mismatch`。同一内部支付订单或退款只能被一条结算明细占用。
- 导入和首次对账在单个 MySQL 事务内完成；正式重对账使用批次乐观锁，人工解决/忽略/重开使用明细乐观锁。重对账会保留仍属于同一差异类型的已解决/已忽略状态，差异类型变化时重新打开；匹配成功后清空人工处理状态。批次存在 `open` 差异时禁止关账，关闭后禁止重对账和修改明细，必须先重开。
- 平台总后台新增“渠道结算对账”工作台，包含渠道/批次筛选、六项汇总、批次列表、JSON/CSV 导入、批次选择、差异筛选、明细表、预演/重跑、解决/忽略/重开、关账/重开，以及批次/明细 CSV 导出。操作分别写入 `payment.settlement.import/reconcile/resolve/transition` 审计日志。
- 新增 `scripts/smoke_saas_payment_settlements.sh` 并纳入 SaaS 验收套件。真实 Go + MySQL/MariaDB + Redis smoke 已通过，覆盖 43 个迁移、平台/租户权限、JSON/CSV 导入、来源重放、批次冲突、收款/退款匹配、五类差异、人工处理、带差异关账拦截、关闭/重开、关闭态修改拦截、dry-run 不落库、正式重对账、重复渠道流水整批回滚、CSV、页面、路由与审计。冒烟循环中 `docker compose exec` 继承 stdin 导致只消费首条分录的问题已通过显式 `</dev/null` 隔离修复。
- `scripts/smoke_schema_migrate.sh` 已通过空库 apply、43 条 checksum、status、重复执行、baseline、rollback 和 legacy 升级；`0043` 两张表、关键列、来源/渠道流水/内部匹配唯一索引均通过真实数据库结构断言。`scripts/lint_mysql57_schema.sh` 返回 `mysql 5.7 schema lint passed (85 files)`；本机 arm64 仍不把 MySQL 5.7 amd64 容器跳过当作生产证据。
- `scripts/test.sh` 已通过全部 Go 单测、`go vet ./...`、五个命令构建、独立性和静态审计；验收纳管为 `smoke_scripts=81 referenced=81`，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `512/512`、runtime route entries `612`，SaaS 总后台与平台运营模块包含 106 条运行时路径。
- `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh` 返回 manifest 路由 `224/224`、`missing_route_total=0`；`scripts/smoke_standalone_compose_app.sh` 已用当前源码重建 app 镜像并完成 Go app + MySQL + Redis、`0043` 初始化、baseline、bootstrap、真实登录和核心 API 容器闭环。
- Playwright 已用真实结算数据在 `1440x1000` 和 `390x844` 视口验收。桌面批次、金额、差异和操作完整可见；移动筛选/汇总自动单列，批次表限制在 `overflow-x:auto` 容器内，页面 `documentWidth=clientWidth=390`，右侧操作按钮可横向滚动查看；批次选择和标识冲突筛选已真实交互，授权后的独立页面为 0 console error、0 warning。
- 专项 smoke、浏览器、Go 服务和 Docker 临时栈均已清理；未运行或保留 `standalone_soak_24h.sh`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-payment-settlements.json`，当前源码指纹为 `9f9b1ad6c16d512c571fa7ac29f4059855c47625bae6e68fd2b20af25a8d414f`，纳入文件数为 `675`，生成时间为 `2026-07-10 22:47:00 CST`。

## 阶段补充（2026-07-10 支付结算自动同步与运行治理闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮不启动新的 24 小时持续运行，本地脚本直接执行。
- 新增迁移 `0044_saas_payment_settlement_sync`：创建 `mochat_go_saas_payment_settlement_sync_states` 渠道游标/运行锁表和 `mochat_go_saas_payment_settlement_sync_runs` 运行历史表，并提供同名 down rollback。状态表按渠道唯一保存游标、当前运行、最近尝试/成功、错误和版本；运行表保存来源、状态、dry-run、前后游标、拉取/导入/幂等/差异计数、操作人和审计关联。
- 新增支付提供方中立的内部 Bridge 客户端。Go 固定调用 `GET /v1/payment-settlements?provider=...&cursor=...&limit=...`，可附带 Bearer token；渠道密钥、证书和渠道 SDK 留在 Bridge，不写入 SaaS 数据库或总后台。响应批次复用已有 JSON 结算导入结构和校验器，规范化 JSON SHA-256 继续提供来源幂等。
- 新增同步服务和 `cron-saas-payment-settlement-sync`。整次运行成功后才推进渠道游标；后续页失败时保留已导入批次并依靠幂等重拉，不提前推进游标。状态行和运行行统一按“渠道状态 → 运行记录”顺序加锁，同渠道人工/定时并发返回 `409`，15 分钟僵死运行可由新任务接管；请求取消后使用独立短上下文写回失败并释放运行锁。
- 新增平台级 `GET /dashboard/saasAdmin/paymentSettlementSyncRuns` 和 `POST/PUT /dashboard/saasAdmin/paymentSettlementSync`。前者按渠道、来源、状态返回配置状态、渠道游标和运行历史；后者支持 dry-run 预演或立即同步。普通业务租户均返回 `403`。
- 平台总后台新增“结算自动同步”工作区，包含渠道/来源/状态筛选、六项汇总、渠道游标与最近错误、运行记录、预演和立即同步；与既有渠道结算批次/明细工作台串联。同步失败写入 `payment_settlement_sync_failed`，成功导入但存在待处理差异时写入 `payment_settlement_issue` 通知 outbox，运行完成/失败均写 `payment.settlement.sync` 审计。
- 新增 `scripts/smoke_saas_payment_settlement_sync.sh` 并纳入 SaaS 验收套件。真实 Go + MySQL/MariaDB + Redis + fake Bridge smoke 已通过，覆盖 44 个迁移、启动即同步、Bearer/查询协议、游标推进、重复批次幂等、dry-run 不写入、同渠道并发 `409`、Bridge 失败、差异/失败通知、task runner 持久化、平台/租户权限、页面与路由。首次 smoke 暴露的 MariaDB `cursor` 关键字引用已统一加反引号并复跑通过。
- `scripts/smoke_schema_migrate.sh` 已通过 44 个迁移的空库 apply、checksum、status、重复执行、baseline、rollback 和 legacy 升级；`0044` 两张表、关键列与渠道/运行号唯一索引均通过真实数据库结构断言。验收脚本审计为 `smoke_scripts=82 referenced=82`，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `514/514`、runtime route entries `615`。
- `scripts/test.sh` 已通过全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性和静态审计；验收清单 direct coverage 为 `224/224`，MySQL 5.7 schema lint 覆盖 `87 files`。新增及受影响 Shell 脚本均通过 `bash -n`。
- 回归已通过 `scripts/smoke_saas_payment_settlements.sh`，确认自动同步没有破坏原有手工 JSON/CSV 导入、差异处理和关账链路；`scripts/smoke_standalone_compose_app.sh` 已使用当前源码完成 Go app + MySQL + Redis、`0044` 初始化、baseline、bootstrap、真实登录和业务 API 容器闭环。
- `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh` 已复跑通过：原 PHP manifest `224/224` 路由全部由 Go standalone 承接，`missing_route_total=0`；本轮自动同步 API 属于 Go 独立 SaaS 扩展能力。
- Playwright 已使用真实 JWT、真实 Go 服务和 fake Bridge 在 `1440x1000` 与 `390x844` 视口验收。桌面页面 `scrollWidth=clientWidth=1440`，移动页面 `scrollWidth=clientWidth=390`；移动表格仅在 `368px` 包装层内横向滚动，可完整到达右侧内容。真实点击“预演”生成 `previewed` 运行且游标保持不变，最终控制台为 0 error、0 warning。
- 专项 smoke、回归、路由验证、浏览器会话、Go 服务和 Docker 临时栈均已清理；目标端口 `13386/26436/18159/18160` 均空闲，`scripts/list_standalone_soak_processes.sh` 返回 0 且无输出，确认未运行或残留 `standalone_soak_24h.sh`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-payment-settlement-sync.json`，当前源码指纹为 `9c728377aff74c24b0546686da2e39b0d3b9175eca54b4ec6a6bdcc3b671b16d`，纳入文件数为 `682`，生成时间为 `2026-07-10 23:28:54 CST`。

## 阶段补充（2026-07-11 平台总后台 RBAC 与访问治理闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮不启动新的 24 小时持续运行，本地命令和脚本直接执行。
- 新增迁移 `0045_saas_admin_rbac`：创建平台角色、角色权限、成员访问状态和成员角色绑定四张表，内置平台运营、平台财务、客户成功、平台审计和只读观察员五类不可修改角色；自定义角色与成员授权均带乐观锁版本。
- 平台超级管理员保留隐式 `*` 全权，平台普通成员只获得启用角色授予的权限。当前 11 项权限覆盖平台概览、租户读写、运营读写、通知读写、财务读写、审计读取和访问治理；写权限自动包含对应读权限，未知总后台路由对非超级管理员默认拒绝。
- 新增 `accessProfile`、`accessRoles/accessRole` 和 `accessAssignments/accessAssignment` API。角色和成员写操作均校验平台租户归属、版本、内置角色不可变、自我授权/自我改角拦截及超级管理员不可被普通角色覆盖，并写入 `saas.admin.access.role.save` 或 `saas.admin.access.assignment.save` 审计日志。
- 总后台页面新增当前授权摘要、权限目录、自定义角色维护和平台成员角色分配；无 `platform.access.manage` 的成员不加载治理数据，按权限隐藏治理区并禁用静态写入口。平台财务受限账号已用真实 JWT 验证只加载财务能力，不再因无权运营接口产生控制台 `403` 错误。
- 新增 `scripts/smoke_saas_admin_access_rbac.sh` 并纳入验收套件。真实 Go + MariaDB + Redis 空库 smoke 已多次通过，覆盖 45 个迁移、内置角色 seed、未授权拒绝、动态授权与即时撤权、财务/运营/审计职责隔离、自定义访问治理角色、自我提权拦截、跨租户拦截、角色与授权版本冲突、页面、路由和审计计数。
- `scripts/smoke_schema_migrate.sh` 已通过 45 个迁移的空库 apply、checksum、status、重复执行、baseline、rollback 和 legacy 升级；四张 RBAC 表、关键索引、五个内置角色与权限 seed 均通过真实数据库断言。`scripts/smoke_standalone_compose_app.sh` 已用当前源码重建镜像并完成 Go app + MySQL + Redis、`0045` 初始化、baseline、bootstrap、真实登录和核心业务 API 容器闭环。
- 最终 `scripts/test.sh` 通过全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性和静态审计；验收纳管为 `smoke_scripts=83 referenced=83`，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `519/519`、runtime route entries `622`，SaaS 总后台与平台运营模块包含 113 条运行时路径。MySQL 5.7 静态门禁覆盖 89 个 schema/迁移文件并通过。
- `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh` 返回原 PHP manifest `224/224`、`missing_route_total=0`；新增平台 RBAC 路由属于 Go 独立 SaaS 扩展能力。
- Playwright 已在真实 Go 服务上完成 `1440x1000` 与 `390x844` 视口验收。桌面治理区宽 1220px、页面无全局横向溢出；移动端页面宽 390px、治理区宽 370px，角色与成员表只在各自 368px 包装层内横向滚动。受限财务账号最终独立会话为 0 console error、0 warning。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-saas-admin-rbac.json`，当前源码指纹为 `214682c3d871cf602d7dbc155e6155a9fe855cfc6069fa90a4eec96a4c0b79ca`，纳入文件数为 `688`，生成时间为 `2026-07-11 00:21:29 CST`。

## 阶段补充（2026-07-11 平台高风险审批与强制执行闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮不启动新的 24 小时持续运行，本地命令和脚本直接执行。
- 新增迁移 `0046_saas_admin_approvals`：创建 `mochat_go_saas_admin_approvals` 审批单和 `mochat_go_saas_admin_approval_events` 不可变事件表，并提供同名 down rollback。审批单保存规范化请求摘要、幂等键、目标、状态、发起/复核/执行人、有效期、执行租约、尝试次数、结果、错误和乐观锁版本。
- 平台权限由 11 项扩展为 14 项，新增 `platform.approvals.read/review/execute`；新增第六个内置角色 `platform_approver`。平台超级管理员继续保留隐式 `*`，普通平台成员按角色获得审批查看、复核和执行能力。
- 默认设置 `MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1` 后，退款创建、业务租户停用、结算关账、平台角色保存和成员授权保存五类高风险操作不能直写，原接口返回 `428`；必须先通过 `approvalRequest` 创建审批，再由非发起人 `approvalDecision` 和 `approvalExecute`。`0` 只保留为迁移期紧急回退。
- 审批链路支持待复核、已批准、已驳回、已撤回、已过期、执行中和已执行；请求按规范化 JSON SHA-256 与幂等键防重，全部转换写入不可变事件与平台操作审计。发起人不能复核或执行自己的申请，授予本人权限的申请也不能由本人复核或执行；业务动作脱离客户端断连上下文执行，五类业务写事务原子写入 `effect_applied_at/effect_operation_id`，执行失败回到已批准并保留错误，15 分钟租约恢复只补齐审批完成状态而不重复已提交副作用。
- 新增 `approvalPolicies`、`approvals`、`approvalEvents`、`approvalRequest`、`approvalDecision`、`approvalCancel` 和 `approvalExecute` 七条运行时路径。总后台新增审批筛选、动作/风险筛选、汇总、审批表、事件时间线，以及批准、驳回、撤销批准、撤回和执行操作；已有退款、租户状态、结算、角色和成员授权表单会在强制模式下自动改为提交审批。
- 新增 `scripts/smoke_saas_admin_approvals.sh` 并纳入 SaaS 验收套件。真实 Go + MariaDB + Redis 空库 smoke 已通过，覆盖 46 个迁移、直写 `428`、申请幂等、自批/自执行/自我授权拦截、批准、执行、拒绝、撤回、过期、退款预占、租户停用、结算关账、角色/授权变更、事件和操作审计；脚本还模拟“副作用已提交、审批完成前中断”的过期执行租约，确认恢复后退款记录与业务操作日志均不增加。
- 迁移、RBAC 与受影响业务回归均已通过：`smoke_schema_migrate.sh` 覆盖 apply/checksum/status/重复执行/baseline/rollback/legacy 升级，`smoke_saas_admin_access_rbac.sh` 覆盖原访问治理，退款、订阅、开票和渠道结算专项 smoke 均通过。`smoke_standalone_compose_app.sh` 已用当前源码重建 Go app 镜像，在新 MySQL/Redis 卷上验证 `0046` 初始化、baseline、bootstrap、真实登录、审批表和启动配置。
- Playwright 已在真实 Go 服务和真实审批数据上完成 `1440x1000` 与 `390x844` 验收。审批人页面真实执行“批准 -> 确认执行”，审批单最终为 `executed v4`，退款账本同步落库；移动页面 `documentWidth=bodyWidth=viewport=390`，表格限制在内部横向滚动区，无页面级横向溢出。首次未填 token 的预期 `401` 后，授权请求和审批写请求均返回 `200`，没有新增控制台错误。
- 最终 `env -u GOROOT ./scripts/test.sh` 通过全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性和静态审计；验收纳管为 `smoke_scripts=84 referenced=84`，MySQL 5.7 静态门禁覆盖 91 个 schema/迁移文件，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `526/526`、runtime route entries `633`，SaaS 总后台与平台运营模块包含 120 条运行时路径。
- `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh` 返回原 PHP manifest `224/224`、`missing_route_total=0`；新增七条审批路由属于 Go 独立 SaaS 扩展能力。专项服务、浏览器会话和 Docker 临时栈均已清理，没有运行或残留 `standalone_soak_24h.sh`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-saas-admin-approvals.json`，当前源码指纹为 `4aa89495cd1a1cc94da9689a80ea2dd5faaeef5fa6e0fa8d9a54e761e72da783`，纳入文件数为 `694`，生成时间为 `2026-07-11 01:29:45 CST`。

## 阶段补充（2026-07-11 审批策略、会签、委托与 SLA 治理闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮不启动新的 24 小时持续运行，本地命令和脚本直接执行。
- 新增迁移 `0047_saas_admin_approval_governance`：创建审批策略、不可变逐票决定和审批委托三张表，为审批单增加策略版本、法定票数、批准票数、SLA 到期、下次/最近提醒和提醒次数，并提供完整 down rollback。五类默认策略随迁移写入，退款默认 1 票，结算关账、角色和成员授权默认 2 票。
- 平台权限由 14 项扩展为 15 项，新增 `platform.approvals.manage` 并授予 `platform_approver`。审批治理成员可维护策略、委托和手动提醒；普通审批查看、复核、执行权限继续分离，平台超级管理员保留隐式 `*`。
- 固定审批门禁已升级为数据库策略。每类动作可配置启用状态、退款金额阈值、会签票数、SLA 分钟、提醒间隔和审批有效期；申请保存完整策略快照，在途审批不受后续配置变化影响。低于退款阈值或策略停用时原业务接口可直接执行，高于阈值及其他启用动作继续返回 `428` 并要求审批。
- 审批决定改为不可变逐票账本：每个实际复核人只能投一票，任一驳回立即结束，批准票达到策略快照法定人数后才进入已批准，未达法定人数不能执行。委托复核会保存委托人来源；委托人必须仍持有复核权，委托必须在有效时间内，发起人及其委托链都不能绕过自批和自我授权限制。
- 新增 `approvalPolicy`、`approvalDecisions`、`approvalDelegations`、`approvalDelegation` 和 `approvalReminders` 路径，并扩展原 `approvalPolicies/approvals/approvalRequest/approvalDecision/approvalExecute`。总后台新增策略行内编辑、法定票数、SLA/提醒、委托维护、审批提醒按钮、策略快照、会签进度、事件时间线和逐票明细。
- 新增 `cron-saas-admin-approval-reminder` 及四项环境变量。手动和自动提醒复用同一事务服务，在更新审批提醒状态的同时写入 `approval_sla_reminder` 通知 outbox 和平台审计；通知 key 使用审批 ID 与提醒次数幂等，启动即跑和周期 tick 都记录在后台任务账本。
- 新增 `scripts/smoke_saas_admin_approval_governance.sh` 并纳入 SaaS 验收套件。真实 Go + MariaDB + Redis 空库 smoke 已重复通过，覆盖 47 个迁移、小额退款直写、高额退款 `428`、两人会签、重复投票、法定票数前执行拦截、委托复核、委托链防自批、逐票明细、执行、手动提醒幂等、cron 第二次提醒、策略停用、页面、路由和审计。
- 迁移与受影响回归已通过：`smoke_schema_migrate.sh` 覆盖 47 个版本的 apply/checksum/status/重复执行/baseline/rollback/legacy 升级，`smoke_saas_admin_approvals.sh`、`smoke_saas_admin_access_rbac.sh`、退款、结算、订阅、收款、发票和 Compose app smoke 均通过；MySQL 5.7 静态门禁覆盖 93 个 schema/迁移文件。
- Playwright 已在真实 Go 服务、真实 JWT 和治理 smoke 数据上完成 `1440x1000` 与 `390x844` 验收。桌面和移动页面均满足 `scrollWidth=clientWidth`，审批策略表在移动端由 368px 容器内部横向滚动；策略、委托、3 条审批、2/2 会签、委托来源、5 条事件和 2 票明细真实显示。登录后总后台 50 个请求均为 `200`，控制台只保留填写 token 前的预期 `401`。
- 最终 `env -u GOROOT ./scripts/test.sh` 通过全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性和静态审计；验收纳管为 `smoke_scripts=85 referenced=85`，manifest 直接覆盖为 `224/224`，功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `531/531`、runtime route entries `641`，SaaS 总后台与平台运营模块包含 125 条运行时路径。
- `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 env -u GOROOT ./scripts/standalone_route_coverage.sh` 已通过：原 PHP manifest `224/224` 路由全部由 Go standalone 承接，`missing_route_total=0`；本轮新增治理路由属于 Go 独立 SaaS 扩展能力，没有启动或残留 `standalone_soak_24h.sh`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-saas-admin-approval-governance.json`，当前源码指纹为 `44be7049e23775f4cb5d457bfd62a2822e500ff66780f3a6c2845c69d86a13ee`，纳入文件数为 `700`，生成时间为 `2026-07-11 08:00:59 CST`。

## 阶段补充（2026-07-11 平台健康中心与事故治理闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮不启动 24 小时持续运行，本地命令和脚本直接执行。
- 新增迁移 `0048_saas_admin_system_health`：创建 `mochat_go_saas_admin_health_scans` 健康扫描表和 `mochat_go_saas_admin_system_incidents` 事故台账，新增 `platform.system.read/manage` 权限 seed，并提供删权限和两张表的完整 down rollback。
- 平台健康检查覆盖 MySQL、Redis、48 个迁移版本、失败后台任务与窗口内失败执行、通知耗尽/重试到期/待发积压、审批 SLA、失败/阻断运营任务和结算同步异常；运行时探针与数据库检查统一输出 `healthy/warning/critical`。
- 手动和 cron 扫描复用同一事务服务：扫描快照、新建/重开/升级事故、恢复事故、`system_health_incident` 通知 outbox 和 `saas.admin.system_health.scan` 审计同事务提交。稳定检查键保证重复扫描只增加发生次数，不重复通知；人工解决但条件仍异常时下次扫描自动重开。
- 新增 `systemHealth`、`systemHealthScans`、`systemIncidents`、`systemHealthScan` 和 `systemIncident` 五个路径，支持健康读取、历史、事故筛选、手动扫描和按乐观锁认领/分派/解决/重开。业务租户访问平台健康接口返回 `403`，审计角色只读，平台运营获得处置权限。
- 新增 `cron-saas-admin-system-health` 及启用、间隔、run-on-start、失败窗口和通知滞留阈值五项环境变量。修复了任务已注册但未计入“是否启动后台任务组”判定的接线缺口；只开启健康 cron 时也会正常运行并写入后台任务账本。
- 总后台新增“平台健康中心”，包含健康概览、失败/积压窗口、事故状态/严重度/负责人/关键词筛选、12 项检查明细、事故处置和扫描历史。写按钮受 `platform.system.manage` 控制，事故行操作使用最新版本防止覆盖并发处置。
- 新增 `scripts/smoke_saas_admin_system_health.sh` 并纳入 SaaS 验收套件。真实 Go + MariaDB + Redis 空库 smoke 已通过，覆盖 48 个迁移、cron 启动即跑、12 项检查、9 类故障注入、9 条首次通知、重复扫描去重、认领/分派/解决、旧版本 `409`、未恢复重开再通知、修复后 9 个事故自动恢复、RBAC、页面、路由和审计。
- `scripts/smoke_schema_migrate.sh` 已通过 48 个版本的空库 apply/checksum/status/重复执行/baseline/legacy 升级，并真实回滚 `0048` 后断言两张表和权限 seed 删除，再应用恢复。MySQL 5.7 静态门禁覆盖 95 个 schema/迁移文件并通过。
- `scripts/test.sh` 已通过全部 Go 单测、构建、独立性与静态审计；验收纳管为 `smoke_scripts=86 referenced=86`，manifest 直接覆盖 `224/224`，功能矩阵为 `modules=29`、runtime unique paths `536/536`、runtime route entries `648`，SaaS 总后台与平台运营模块包含 130 条运行时路径。
- standalone Compose app smoke 已用当前源码重建业务镜像，完成 Go app + MySQL + Redis、`0048` 初始化、baseline、bootstrap、真实登录和核心 API 闭环。当前本地预览保留在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`。
- Playwright 已用真实账号从标准 `/login` 页登录，再直接打开总后台；页面会兼容旧前端以 JSON 字符串保存的 `ACCESS_TOKEN`，去除多余引号后识别为“平台超级管理员 / 全权限”，无需手工粘贴 JWT。已在 `1440x1100` 和 `390x844` 视口验收，两种视口均满足 `documentScrollWidth=viewport`，移动端健康中心宽 `370px`，宽表只在内部容器滚动；真实点击“立即扫描”新增 1 条扫描和审计记录，最终健康摘要为 `12/12`、0 活跃事故，授权后控制台无请求错误。截图保存在 `output/playwright/system-health-desktop.png` 和 `output/playwright/system-health-mobile.png`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-system-health.json`，当前源码指纹为 `afc7b98c42d9a646cf1e48ce1f4b1620a3c7a1a11e9057a55cb1f34025353ab9`，纳入文件数为 `707`，生成时间为 `2026-07-11 09:13:03 CST`。

## 阶段补充（2026-07-11 租户服务账号与 API Key 集成治理闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮不启动 24 小时持续运行，本地命令和脚本直接执行。
- 新增迁移 `0049_saas_service_accounts`：创建 `mochat_go_saas_service_accounts` 和 `mochat_go_saas_service_account_keys`，服务账号绑定单一业务租户，保存 Scope、IP/CIDR 白名单、状态、过期时间、最后使用时间与来源 IP；密钥保存 active/retiring/revoked 生命周期、轮换宽限、过期时间、使用摘要和乐观锁版本。
- API Key 使用 `mch_live_<12 hex>_<43 base64url>` 格式。创建和轮换只在单次响应返回明文；数据库只保存以 `SimpleJWTSecret` 为 pepper 的 HMAC-SHA256 摘要、非敏感前缀和后四位。错误 Key、账号变化、过期和吊销统一返回通用未授权错误，避免向调用方泄露账号状态。生产环境变更 `MOCHAT_SIMPLE_JWT_SECRET` 会同时使现有 dashboard JWT 和全部 API Key 失效。
- 新增 `profile.read`、`usage.read` 和 `alerts.read` 三个服务账号 Scope，以及 `GET /api/saas/v1/whoami`、`usage`、`alerts` 三条公共 API。鉴权强制检查账号/密钥状态与过期、Scope、来源 IP/CIDR 和租户归属，业务查询始终使用 Key 绑定租户；成功后更新账号与密钥使用摘要。
- 平台权限由 17 项扩展为 19 项，新增 `platform.integrations.read/manage`。总后台提供服务账号筛选、概览、创建/编辑、Scope 与 IP 白名单维护、Key 列表、带宽限期轮换和吊销；一次性 Key 提供复制与主动隐藏，自定义 `<dialog>` 承担吊销确认，避免浏览器原生确认框阻断自动化和应用内浏览器。
- 新增 `scripts/smoke_saas_service_accounts.sh` 并纳入验收套件。真实 Go + MariaDB + Redis 空库 smoke 已重复通过，覆盖 49 个迁移、摘要存储、错误/缺失 Key、Scope 拒绝、IP 白名单、轮换宽限、立即退役、吊销、账号停启、乐观锁、平台读写 RBAC、业务租户拒绝、使用追踪、页面、运行时路由和五类操作审计。
- `scripts/smoke_schema_migrate.sh` 已通过 49 个版本的空库 apply/checksum/status/重复执行/baseline/legacy 升级，并真实回滚 `0049` 后断言两张服务账号表和两项权限 seed 删除，再应用恢复。MySQL 5.7 静态门禁覆盖 97 个 schema/迁移文件并通过；访问治理和系统健康回归 smoke 同步通过。
- 最终 `scripts/test.sh` 已通过全部 Go 单测、`go vet ./...`、五个命令构建、独立性和静态审计；验收纳管为 `smoke_scripts=87 referenced=87`，manifest 路由直接覆盖 `224/224`，功能矩阵为 `modules=29`、runtime unique paths `543/543`、runtime route entries `658`，SaaS 总后台与平台运营模块包含 137 条运行时路径。
- 真实 Compose 预览库已迁移到 `0049`。浏览器已验证创建 `browser_probe`、编辑、轮换、一次性 Key 复制和自定义吊销确认，数据库操作审计分别记录 create/update/rotate/revoke；桌面 `1440x1100` 与移动 `390x844` 均无页面级横向溢出，移动宽表只在内部滚动，授权后无新增请求错误。当前预览保留在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-saas-service-accounts.json`，当前源码指纹为 `165b1a14991b4a271464f5091b9df502d798eeb94ae6336b9df73a00e9322315`，纳入文件数为 `713`，生成时间为 `2026-07-11 10:29:00 CST`。

## 阶段补充（2026-07-11 加密备份与隔离恢复治理闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮本地命令和脚本直接执行，不启动 24 小时持续运行。
- 新增迁移 `0050_saas_backup_recovery`：创建全局备份策略、备份运行和隔离恢复演练三张表，保存工件 SHA-256、非敏感密钥 ID、迁移/表元数据、完整性校验、恢复目标指纹和审计关联；唯一活动槽阻止同类任务并发，并新增 `platform.backups.read/manage` 灾备权限。平台权限目录扩展为 21 项。
- 备份流水线为 `mysqldump -> gzip -> 分块 AES-256-GCM -> 原子改名`。工件权限固定为 `0600`，数据库不保存密钥、恢复 DSN 或明文密码；MySQL 客户端密码只进入随用随删的 `0600` defaults 文件。修复 Alpine MariaDB 11 客户端默认强制 TLS、而 MariaDB 10.6 源库未启用 TLS时的真实容器故障：客户端 SSL 模式现在严格跟随 Go DSN，默认 `ssl=0`，显式 TLS 配置才启用 SSL，并增加单元测试锁定该行为。
- 恢复演练只接受服务端预配置 DSN，目标库必须匹配安全前缀、不得与源库同名且必须为空。恢复前重新校验工件 SHA-256 和 GCM 认证标签，恢复后核对迁移版本、迁移数、表数和关键表；错密钥、非空库、危险前缀和同库目标均明确拒绝，不允许 API 请求覆盖恢复地址。
- 备份策略支持周期、保留天数、最少成功工件、最大备份年龄、恢复演练间隔和强制加密。cron、手工创建、重新校验、保留清理和隔离恢复共用同一管理器；进程异常遗留的 running 任务可恢复为失败。后台维护命令支持 `backup-create`、`backup-verify`、`backup-cleanup` 和 `restore-drill`，后台任务账本记录自动执行结果。
- 总后台新增“灾备中心”，展示六项运行配置就绪状态、策略、备份统计、工件台账和恢复演练台账，并提供策略保存、立即备份、重新校验、保留清理和带二次确认的恢复演练。失败备份与失败演练默认只显示“查看失败详情”，展开后再展示完整客户端错误，避免技术日志拉高整张运维表；运营角色可读写，审计角色只读，财务与业务租户拒绝访问，写操作均进入平台操作审计。
- 平台健康中心由 12 项扩展为 16 项检查，新增备份配置、最近成功备份年龄、最近验证状态和恢复演练时效。检查结果可沿用现有健康扫描、事故去重、通知和处置闭环。
- 新增 `scripts/smoke_saas_backup_recovery.sh` 并纳入 `standalone_acceptance.sh` 和功能矩阵。真实 Go + MariaDB + Redis 空库 smoke 覆盖 50 个迁移、cron 与手工加密备份、原子工件、`0600` 权限、明文隐藏、重新校验、错密钥失败后恢复、隔离恢复、四类危险目标拒绝、保留清理、维护命令、RBAC、审计、健康检查、页面和路由。
- `scripts/smoke_standalone_compose_app.sh` 已重建当前业务镜像，并在容器内实际完成一份加密备份，断言工件加密、完整性通过、迁移版本 `0050`、迁移数 50 和客户端工具就绪；这项真实检查覆盖了 MariaDB 客户端 SSL 回归，而不再只检查配置或二进制存在。
- 最终回归已通过 `scripts/smoke_schema_migrate.sh`、灾备专项 smoke、平台 RBAC、系统健康和 `scripts/test.sh`；全部 Go 单测、`go vet ./...`、五个命令构建、独立性与静态审计均通过。验收纳管为 `smoke_scripts=88 referenced=88`；功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `547/547`、runtime route entries `665`，原 PHP manifest 路由直接覆盖 `224/224`。MySQL 5.7 静态门禁覆盖 99 个 schema/迁移文件并通过；当前 arm64 本机未运行 amd64-only 的真实 MySQL 5.7 容器 smoke，避免依赖不稳定 QEMU 仿真，需由 amd64 CI 或部署机补充该项实库证据。
- 真实 Compose 预览库已迁移到 `0050`，并保留一份当前密钥生成且校验通过的加密工件。Playwright 已在桌面和 `390x844` 移动视口验收灾备中心、恢复确认弹窗及宽表内部滚动；截图保存在 `output/playwright/saas-backup-center-desktop.png`、`output/playwright/saas-restore-confirm-dialog.png` 和 `output/playwright/saas-backup-center-mobile.png`。当前预览保留在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-saas-backup-recovery.json`，当前源码指纹为 `c2449f1de163d553457f52f9641c4f4bf8cd8c5ea2e0d1ab8385309c5d79c229`，纳入文件数为 `724`，生成时间为 `2026-07-11 11:52:52 CST`。

## 阶段补充（2026-07-11 异地副本、密钥轮换与临时恢复库闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮本地命令和脚本直接执行，不启动 24 小时持续运行。
- 新增迁移 `0051_saas_backup_resilience` 及 down rollback：备份策略增加强制异地副本开关，备份运行增加提供方、桶、对象键、ETag、版本、SHA-256、大小、复制时间、校验时间和错误状态，恢复演练增加预配置/临时目标生命周期与销毁结果。
- 加密配置升级为当前密钥加历史密钥环。新备份只使用当前 ID，校验和恢复按每条运行保存的 ID 选择历史密钥；平台健康中心会扫描所有未删除且成功的加密备份，缺失历史密钥时直接报告严重异常。
- 新增 S3 兼容副本存储，使用 MinIO Go SDK 上传、读取、删除和探测。上传后不依赖 ETag 判定完整性，而是重新读取整个远端对象计算 SHA-256 与大小；本地工件缺失或损坏时从已验证副本下载到临时文件，校验后以 `0600` 权限原子发布。保留清理先删除远端对象，再删除本地工件和更新台账。
- 恢复演练支持由管理 DSN 自动创建随机 `mochat_restore_` 前缀临时库，完成迁移、表数和关键表校验后立即销毁；失败默认同样清理，可显式短期保留用于排障。原预配置空库模式继续保留，并继续拒绝源库、危险前缀和非空目标。
- 总后台灾备中心增加强制异地策略、密钥数量与可用性、副本状态、对象位置、复制失败详情、手工复制和成功副本检查/修复入口；恢复台账展示临时/预配置生命周期与清理状态。健康中心增加副本合规、密钥环和对象存储连通性检查。
- `scripts/smoke_saas_backup_recovery.sh` 已使用真实 MariaDB、Redis、MinIO 和 MySQL 客户端通过 51 个迁移与灾备专项验收，覆盖旧密钥备份在新密钥进程中校验、远端对象篡改检测和修复、本地删除后回源、自动临时库销毁、固定库真实数据恢复、远端保留清理、密钥与对象存储凭据不落审计和 19 项健康检查。
- 最终 `scripts/test.sh` 已通过全部 Go 单测、`go vet ./...`、五个命令构建、独立性与静态审计；验收纳管为 `smoke_scripts=88 referenced=88`，原 PHP manifest 路由直接覆盖 `224/224`，功能矩阵为 `modules=29`、runtime unique paths `547/547`、runtime route entries `665`，SaaS 总后台与平台运营模块包含 141 条运行时路径。MySQL 5.7 静态门禁覆盖 101 个 schema/迁移文件并通过，迁移 smoke 完成 51 个版本的 apply、checksum、status、重复执行、baseline、rollback 和 legacy 升级。
- 真实 Compose 预览已原位升级到 `0051`，保留 `preview-0050` 历史密钥工件并使用 `preview-0051` 创建当前工件，两份成功备份均已复制到真实 MinIO；自动临时恢复库演练成功且生成库已销毁，灾备策略已强制加密和异地副本，平台健康中心为 `19/19`。当前预览保留在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，测试账号为 `13800000090`。
- Playwright 已在真实服务完成桌面 `1200px` 与移动 `390px` 视口验收：两种视口均无页面级横向溢出，备份和恢复宽表只在内部容器滚动，远端副本检查/修复入口在移动端可达，授权后控制台为 0 error、0 warning。截图保存在 `output/playwright/saas-backup-resilience-desktop.png`、`output/playwright/saas-backup-resilience-mobile.png` 和 `output/playwright/saas-backup-resilience-mobile-actions.png`。

## 阶段补充（2026-07-11 租户数据合规生命周期闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮本地命令和脚本直接执行，不启动 24 小时持续运行。
- 新增迁移 `0052_saas_compliance_lifecycle` 及完整 down rollback：创建全局合规策略、租户法律保留、加密数据导出、擦除申请、持久化擦除步骤和匿名租户墓碑六张表；新增 `platform.compliance.read/manage`，平台权限目录扩展为 23 项，并增加默认两人复核的 `tenant.data.erase` 第六类高风险审批策略。
- 数据导出由静态清单覆盖 142 张租户相关表，按 JSONL、manifest 和租户物理文件打包，再使用分块 AES-256-GCM 加密、SHA-256 校验、`0600` 权限和原子改名发布。下载前重新校验工件摘要与 GCM 认证标签；数据库和审计只保存密钥 ID、摘要与非敏感元数据，不保存密钥或明文数据。
- 擦除门禁要求业务租户已停用、完整输入当前租户名、存在最近成功导出、没有活动法律保留、宽限期届满、财务保留期届满且审批完成。执行过程把数据集操作持久化为可恢复步骤，物理删除业务数据与文件，脱敏必须保留的结算和审计证据，最后仅生成不可逆墓碑、统计和验证 SHA-256；审批副作用标记与业务授权在同一事务提交，恢复执行不会重复授权。
- 总后台新增“租户数据合规中心”，提供配置与清单就绪度、六项汇总、策略编辑、法律保留创建/解除、导出申请/处理/下载/删除、擦除申请、审批恢复、处理/取消和逐步证据查看。查看和管理权限分离；合规管理角色必须同时拥有合规查看、审计查看和审批查看权限。
- 新增 `complianceOverview`、`compliancePolicy`、`complianceLegalHold`、`complianceExport`、`complianceExportDownload`、`complianceErasure` 和 `complianceErasureSteps` 路径；新增 `compliance-status/process/export-process/erasure-process` 维护动作和可选 cron。平台健康中心增加合规导出队列、擦除队列与运行配置检查，默认 21 项，配置 S3 探针时共 22 项。
- `scripts/smoke_saas_compliance_lifecycle.sh` 已在真实 Go + MariaDB + Redis 上通过，覆盖 52 个迁移、清单门禁、加密导出/解密、工件篡改拒绝、法律保留、精确确认、双人审批、财务保留阻断、145 步恢复执行、物理文件删除、证据脱敏、墓碑和维护命令。审批、审批治理、RBAC、系统健康、备份恢复、Compose app、schema migration 和 MySQL 5.7 lint 回归均通过；静态门禁覆盖 103 个 schema/迁移文件。
- 最终 `env -u GOROOT ./scripts/test.sh` 通过全部 Go 单元测试、`go vet ./...`、五个命令构建、独立性与静态审计。验收纳管为 `smoke_scripts=89 referenced=89`，manifest 路由直接覆盖 `224/224`；功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `554/554`、runtime route entries `676`，SaaS 总后台与平台运营模块包含 148 条运行时路径。严格运行时路由检查为 `missing_route_total=0`、`extra_route_total=209`。
- 真实 Compose 预览已原位升级到 `0052`，保留原 MySQL、Redis、MinIO、加密备份密钥环和历史工件。预览准备了一个成功加密导出且处于活动法律保留的租户，以及一个已完成注销前导出、停用、待双人审批擦除的租户；浏览器只创建审批单，没有执行预览租户擦除。平台健康中心为 `22/22`，当前预览保留在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`。
- Playwright 已在真实 JWT 和真实预览数据上完成 `1440x1100` 与 `390x844` 验收。桌面页面 `scrollWidth=clientWidth=1440`，合规区宽 1220px；移动页面 `scrollWidth=clientWidth=390`，合规区宽 370px，三张 1080px 宽表均限制在 368px 内部滚动容器。租户筛选、刷新和幂等发起擦除审批真实执行，授权页面为 0 console error、0 warning；截图保存在 `output/playwright/saas-compliance-center-desktop.png` 和 `output/playwright/saas-compliance-center-mobile.png`。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-saas-compliance-lifecycle.json`，当前源码指纹为 `83234c23f190b4f4b1de2abb9af655b2fca8da60e4839a1978ccb6324ab81e58`，纳入文件数为 740，生成时间为 `2026-07-11 14:40:19 CST`。

## 阶段补充（2026-07-11 身份与访问安全中心闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮本地命令和脚本直接执行，不启动 24 小时持续运行。
- 新增迁移 `0053_saas_identity_security` 及完整 down rollback：创建租户身份策略、用户登录安全状态、TOTP/恢复码凭据、二次认证挑战、可撤销登录会话、不可变登录事件和身份安全事故七张表；新增 `platform.identity.read/manage`，平台权限目录由 23 项扩展为 25 项。身份管理自动依赖身份查看和审计查看，运营角色可处置，审批、审计和只读角色可查看。
- 登录链路增加失败次数与定时锁定、管理员解锁、租户 IP/CIDR 门禁、新 IP 风险事件、强制 MFA 和并发会话策略。修复失败计数 SQL 更新顺序导致的锁定阈值提前一轮问题，当前策略设置 3 次时会在第三次失败后锁定；事件和安全事故与状态事务化写入。
- MFA 使用 TOTP 和 8 个一次性恢复码。明文密钥与恢复码只在开始绑定的单次响应出现；MFA 密钥以 AES-256-GCM 密钥环加密，恢复码、二次认证挑战、会话 token 和手机号事件索引只保存不可逆摘要。动态码时间步、恢复码和挑战均防重放，管理员重置会同时失效未完成挑战且不在操作审计中泄露凭据。
- JWT 登录升级为持久会话：保存认证方式、来源、签发/过期/空闲时间和最后活动，支持并发上限淘汰、单会话撤销、用户全部撤销和登出立即失效。修复身份安全关闭时把 `*Manager` 类型空指针装入会话接口导致 RBAC smoke panic 的接线问题；关闭身份模块时继续兼容原 JWT，开启强制检查后只接受活动会话。
- 新增 `/security/login`、`POST /dashboard/user/authMFA` 和 `GET/POST/PUT /dashboard/user/securityMFA`。修复前端静态 SPA 吞掉 `/security/login` 的路由优先级问题；安全登录页同时写入 Cookie 与旧 Vue 所需的 JSON 字符串 `ACCESS_TOKEN`，真实登录可直接进入业务后台，不再被旧前端重定向回 `/login`。
- 总后台新增“身份与访问安全中心”：支持租户、用户、会话状态、风险和关键词筛选，展示锁定账号、MFA 覆盖、活动/历史会话、登录事件和安全事故；可按乐观锁编辑失败阈值、锁定时长、会话 TTL/空闲超时/并发数、强制 MFA、IP/CIDR 和保留期，并执行账号解锁、撤销会话、重置 MFA、确认/分派/解决/重开事故。桌面和移动端均限制页面级横向溢出，宽表只在内部容器滚动。
- 身份数据已接入租户合规清单：策略、状态、MFA、挑战和会话执行删除，登录事件和事故执行脱敏。清单由 142 项扩展为 149 项，擦除流程由 145 步扩展为 152 步；为无 `id` 主键的身份策略/状态表增加显式稳定排序字段，修复真实加密导出时 `ORDER BY id` 失败。
- 新增 `identity-cleanup` 维护动作和 `cron-saas-identity-cleanup`，按租户策略过期挑战与活动会话，并分批清理超过保留期的挑战、会话和登录事件。平台健康中心增加身份密钥环与会话强制配置探针。
- `scripts/smoke_saas_identity_security.sh` 已在真实 Go + MariaDB + Redis 空库上重复通过，覆盖 53 个迁移、第三次失败锁定、解锁、IP 门禁、TOTP、恢复码、防重放、并发会话淘汰、撤销、登出、事件、事故、审计脱敏和保留清理。迁移、RBAC、系统健康、合规生命周期和备份恢复专项 smoke 均已通过；`scripts/smoke_schema_migrate.sh` 真实回滚 `0053` 和 `0052` 后顺序重放成功，MySQL 5.7 静态门禁覆盖 105 个 schema/迁移文件。
- 最终 `env -u GOROOT go test ./...` 与 `env -u GOROOT ./scripts/test.sh` 均通过。验收纳管为 `smoke_scripts=90 referenced=90`，原 PHP manifest 路由直接覆盖 `224/224`；功能矩阵为 `modules=29`、manifest unique paths `213/213`、runtime unique paths `566/566`、runtime route entries `696`，SaaS 总后台与平台运营模块包含 158 条运行时路径。总后台和安全登录页内联 JavaScript 均通过 `node --check`。
- 真实 Compose 预览已原位升级到 `53/0053_saas_identity_security`，保留原 MySQL、Redis、MinIO、合规/备份工件与密钥环。身份安全、持久会话和启动即清理已开启，当前密钥 ID 为 `identity-preview-v1`、可用密钥 3 把；平台健康中心为 `23/23`，7 张身份表和 25 项权限均已核对。当前预览为 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，安全登录入口为 `http://127.0.0.1:18090/security/login`。
- Playwright 已用真实账号完成安全登录和身份中心的桌面/移动验收，授权后可进入业务后台与总后台；截图保存在 `output/playwright/security-login-desktop.png`、`output/playwright/security-login-mobile.png`、`output/playwright/saas-identity-center-desktop-viewport.png` 和 `output/playwright/saas-identity-center-mobile-viewport.png`。旧业务前端仍会请求证书过期的外部二维码 `https://oss.mo.chat/github/contact-qr.png`，与本次安全登录和总后台接口无关，后续应替换为自有静态资源。
- 已执行 `env -u GOROOT ./scripts/source_fingerprint.py --out /tmp/mochat-go-current-fingerprint-saas-identity-security.json`，当前源码指纹为 `a2887472cc28aa8808953ec6ecf41d9643cff7a56ed0b2927a5b0a2f899c067d`，纳入文件数为 752，生成时间为 `2026-07-11 18:52:29 CST`。

## 阶段补充（2026-07-11 SaaS 品牌白标、外部依赖收口与本地证据恢复）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮本地命令和脚本直接执行，不启动 24 小时持续运行。
- 新增迁移 `0054_saas_branding_profiles` 及完整 down rollback：创建租户品牌档案，新增 `platform.branding.read/manage`，平台权限目录扩展为 27 项。品牌档案支持租户级产品名称、简称、副标题、Logo、Favicon、登录背景、主色、强调色、官网、支持入口、支持邮箱、文档入口、页脚、启停状态和乐观锁版本，写入操作审计。
- 新增 `brandingProfiles` 与 `brandingProfile` API，总后台增加品牌档案筛选、编辑、色板和登录页实时预览；安全登录页与总后台标题使用当前租户品牌。资源路径仅允许同源 `/img/`、`/static/`、`/favicon.ico`，链接仅允许 HTTPS 或同源地址，避免白标配置引入不受控脚本和混合内容。
- dashboard 根页面和内置前端统一改为同源 `/dashboard` API；Vue、Vue Router、Vuex、Axios 已随应用本地托管，移除旧 `api.mo.chat`、`oss.mo.chat`、外部框架 CDN 和百度统计依赖。浏览器核验未发现外部资源请求。
- 合规清单随品牌档案扩展为 150 项，擦除流程扩展为 153 步；GPL-3.0 许可证、通知、源码提供说明、修改说明和第三方通知已纳入独立发行包与镜像。
- 本轮完整本地证据包位于 `docs/evidence/latest`。14 个结果记录全部返回 0，包括测试、独立发行包、清单一致性、`core/saas/workers/cron/frontend/mysql57` 验收、路由覆盖、前端 API 覆盖、阶段报告、生产证据审计和目标完成度审计；验收脚本为 `91/91`，manifest 路由为 `224/224`，runtime unique paths 为 `568/568`，runtime route entries 为 `700`。源码指纹为 `89d7d5c1442b6f141bfc242b68194e707b6494d6c30869eee5e9f2a9afb5a58a`，纳入文件数 762，当前复核仍匹配。
- 修复证据脚本与保留预览共用 Compose 项目名的问题：`smoke_standalone_compose_app.sh` 只有在本脚本实际启动栈后才执行清理，`collect_standalone_evidence.sh` 的 core 套件使用独立项目名与独立端口。通知 cron 和审批治理 smoke 的日志断言也已同步当前字段，避免业务已通过但脚本因旧格式误报。
- 在修复隔离逻辑前，一次 core 验收清理误删了预览 app、MySQL、Redis 卷。已从 MinIO 中 SHA-256 校验通过的最新 AES-256-GCM 数据库工件恢复 53 个迁移、179 张表、3 个租户、3 个账号、备份/恢复台账和身份会话，再应用 `0054`；最新工件已从异地副本回源复验，临时恢复库演练核对 `53/53` 个迁移、`179/179` 张表并成功销毁。两个合规导出工件已重新生成，解密下载各包含 151 个归档文件。
- 恢复后的预览运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，安全登录入口为 `http://127.0.0.1:18090/security/login`。当前迁移为 `54/0054_saas_branding_profiles`，运行时路由 678 条，平台健康为 `23/23`，备份异地副本、隔离恢复、合规工件、身份强制会话、27 项权限和品牌审计均已核对。
- 浏览器已重新完成 `1440x1100` 与 `390x844` 验收。两种视口均满足 `scrollWidth=clientWidth`，品牌中心显示 3 个租户档案，当前产品名为 `MoChat Go SaaS`；控制台为 0 error、0 warning，外部资源为 0。恢复后截图保存在 `output/playwright/saas-preview-recovery-desktop.png` 和 `output/playwright/saas-preview-recovery-mobile.png`。
- 当前目标仍保留真实外部生产证据缺口：amd64 环境的 MySQL 5.7 实容器、真实企业微信、真实微信开放平台、至少两个真实 SaaS 租户、生产域名浏览器以及目标环境短时稳定性/监控证据。按用户要求，不再以 24 小时运行作为本轮动作。

## 阶段补充（2026-07-11 租户自定义域名与 Host 隔离闭环）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮本地命令和脚本直接执行，不启动 24 小时持续运行。
- 新增迁移 `0055_saas_tenant_domains` 及完整 down rollback：创建租户自定义域名表，保存唯一 Host 绑定、主域名、`pending/active/disabled` 状态、DNS TXT 校验令牌、校验结果、乐观锁版本和创建/更新人；新增 `platform.domains.read/manage`，平台权限目录扩展为 29 项。
- 域名生命周期已闭环：创建后生成 `_mochat.<hostname>` TXT 记录和值前缀 `mochat-domain-verification=`，只有所有权校验通过的域名才能启用；支持设为主域、停用、重新启用、令牌轮换、重新校验和软删除后跨租户重绑。租户级写操作先锁定租户，避免并发域名操作形成交叉死锁；非法状态迁移和旧版本写入会明确拒绝。
- 登录和 MFA 已按 Host 强制租户隔离。已登记但未启用的 Host 返回 `421`，不回落到默认租户；活动 Host 只在绑定租户内按手机号查找账号，并使用租户绑定的 MFA 挑战完成认证，阻断同手机号或挑战在租户间串用。安全登录页同步使用该租户品牌。
- 总后台新增“租户域名中心”，支持租户/状态/关键词筛选、创建、DNS 指引与复制、校验、主域、启停、令牌轮换和删除。危险操作使用应用内确认弹窗，查看和管理权限分离；桌面与移动端均无页面级横向溢出，宽表只在内部滚动。
- 域名表已纳入租户合规生命周期，当前数据清单为 151 项、可恢复擦除为 154 步。迁移器当前支持 55 个版本；`scripts/smoke_schema_migrate.sh` 已通过空库 apply、checksum、status、重复 apply、baseline、legacy 升级以及 `0055/0054/0053/0052` 顺序回滚和重放，并修正权限种类断言为当前 28 个已 seed 权限代码。
- 新增 `scripts/smoke_saas_tenant_domains.sh` 并纳入 SaaS 验收。真实 Go + MariaDB + Redis + 本地权威 DNS smoke 覆盖 TXT 失败/成功、未启用 Host `421`、租户品牌与账号隔离、域名冲突、主域、启停、令牌轮换与复验、软删除重绑、RBAC、审计、页面和运行时路由。
- 修复短证据收集器只给 core 使用隔离端口的问题：当前 `core/saas/workers/cron/frontend` 均使用独立 Go、MySQL 和 Redis 端口，保留 `18090/13318/26391` 预览栈时不会误报端口占用或清理预览数据；旧的 `MOCHAT_LOCAL_EVIDENCE_CORE_*` 环境变量继续兼容。
- 已重建 `docs/evidence/latest` 全套非 PHP 短证据包，14 条命令记录全部返回 0：快速门禁、独立交付包、PHP 清单对齐、`core/saas/workers/cron/frontend/mysql57`、运行时路由、前端 dist API、阶段报告、生产证据报告和目标完成度报告均通过或成功生成。验收纳管为 `92/92`，manifest 路由为 `224/224`、缺失 0，runtime unique paths 为 `570/570`、runtime route entries 为 703，前端 dist API 为 209 项、缺失 0。阶段报告已改为按审计结果动态判断前端 API 缺口，当前结论为“收口验收/生产化补证阶段”。
- 当前源码与验收指纹为 `91bf91c52ac0e0223818a65175e44b23073253bc805a8ac199e3c96c78789087`，纳入文件数 768；目标完成度报告显示指纹匹配 `True`。最终 `env -u GOROOT ./scripts/test.sh`、专项迁移、域名、RBAC、合规和 MySQL 5.7 静态门禁均通过。
- 真实 Compose 预览已原位升级到 `55/0055_saas_tenant_domains`，保留原 MySQL、Redis、MinIO、加密备份/副本、合规和身份数据。当前平台健康 `23/23`、问题 0，权限目录 29 项；域名中心保留租户 2 的 `login.tenant2.preview.test` pending 演示记录。预览地址为 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，安全登录入口为 `http://127.0.0.1:18090/security/login`。
- Playwright 已完成真实登录、域名创建、令牌轮换确认弹窗和桌面/移动验收；移动视口 `390px` 无页面级溢出，授权后控制台 0 error、0 warning。截图保存在 `output/playwright/saas-tenant-domains-desktop.png`、`output/playwright/saas-tenant-domains-mobile.png` 和 `output/playwright/saas-tenant-domain-dialog.png`。
- 当前本地独立 Go SaaS 与总后台功能阶段已收口，尚不能标记生产最终完成；剩余项是 amd64 MySQL 5.7 实容器、真实企业微信与微信开放平台、至少两个真实租户业务数据、生产域名浏览器回归和目标环境短稳/监控证据，不再默认执行 24 小时 run。

## 阶段补充（2026-07-12 发布证据工件完整性与 0058 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；按用户要求，本轮本地命令、测试、构建和服务重启直接执行，不启动 24 小时持续运行。
- 在 `0057_saas_release_readiness` 六类生产证据台账和不可变候选快照基础上，新增 `0058_saas_release_evidence_integrity` 及 down rollback：发布证据增加 `artifact_sha256`、`artifact_size_bytes` 和索引。旧证据即使状态为 `passed`，缺少工件 SHA-256 或非零字节数也会被判定为不完整，不能通过严格发布门禁。
- 发布证据 API、MySQL Store、平台操作审计和候选 `snapshot_json` 已同步固化工件身份。`missing` 会清空全部证据元数据；`in_progress/failed` 可选填完整的哈希与大小组合；`passed` 必须同时提供真实 HTTPS 地址、执行环境、当前源码指纹、64 位工件 SHA-256 和大于 0 的字节数。候选仍要求六项证据完整且源码指纹一致才可标记 `ready`。
- 总后台发布准备中心新增证据工件 SHA-256 和字节大小输入，证据列表展示短哈希与可读大小。浏览器已验证缺少工件字段时前端阻止保存，完整字段保存后显示 `4.00 KB`，测试记录随后恢复为六项 `missing`，未把本地样本保留为生产证据。
- 当前迁移基线统一升级为 `58/0058_saas_release_evidence_integrity`。`scripts/smoke_schema_migrate.sh` 已通过空库 apply、checksum、status、重复 apply、baseline、legacy 升级，并依次回滚 `0058`、`0057` 后重放；`scripts/lint_mysql57_schema.sh` 通过 115 个 schema/迁移文件。系统健康、备份恢复、Compose 初始化及各 SaaS 专项脚本中的迁移版本和数量断言均已同步。
- `scripts/smoke_saas_release_readiness.sh` 在真实 Go + MariaDB + Redis 上通过，覆盖缺工件拒绝、危险 URL/凭据拒绝、RBAC、乐观锁、六证据放行、工件字段快照、审计和双层 rollback。`scripts/smoke_saas_admin_system_health.sh`、`scripts/smoke_saas_backup_recovery.sh`、`scripts/test.sh` 和全部 Go 单测同步通过。
- 真实 Compose 预览库已在迁移前导出到 `/tmp/mochat-preview-before-0058.sql`，随后原位应用 `0058`、重建业务镜像并恢复健康。预览地址继续为 `http://127.0.0.1:18090/dashboard/saasAdmin/page`；MySQL、Redis 和 MinIO 原数据保留。
- Playwright 已在 `1440x1000` 与 `390x844` 视口完成发布准备中心验收，移动端编辑弹窗字段、按钮和长哈希无重叠；截图保存在 `output/playwright/saas0058/release-readiness-desktop.png`、`release-readiness-mobile.png` 和 `release-evidence-modal-mobile.png`。首次未填写 Dashboard JWT 时会出现预期 `401`，保存有效 JWT 后发布证据读取、校验、保存、刷新和审计请求均正常。
- 已重建 `docs/evidence/latest` 全套短证据包。快速门禁、独立交付包、迁移期清单对齐、`core/saas/workers/cron/frontend/mysql57`、运行时路由、前端 dist API、阶段报告、生产证据报告和目标完成度报告全部通过或成功生成；路由为 `224/224`、缺失 0，验收脚本接入 `93/93`，功能矩阵 runtime unique paths `576/576`、runtime route entries `712`。
- 当前源码与验收指纹为 `ef4501304aaba939b5dd4670db32f85039a53a33693c2e9d4ad1e124bc6a5fb2`，纳入文件数 784，latest 指纹匹配。目标完成度仍为 `goal_complete=false`，唯一缺口标签为“生产证据”：MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、真实 SaaS 多租户、生产前端浏览器和目标环境稳定性六项外部证据；这些证据进入总后台时还必须提供实际工件哈希与大小。

## 阶段补充（2026-07-12 运行版本权威指纹与发布可信链收口）

- 发布门禁不再信任客户端手工填写的源码指纹。新增 `internal/buildinfo` 构建信息入口，Docker 构建阶段在完整源码、部署配置、验收脚本和前端产物上下文中执行 `scripts/source_fingerprint.py`，再通过 Go linker 把同一指纹写入 `mochat-go`、`mochat-migrate`、`mochat-bootstrap` 和 `mochat-saas-maintenance` 四个交付二进制。
- 配置读取优先使用二进制内置指纹，并记录来源为 `build`；只有本地直接 `go run` 且构建值为空时才允许 `MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT` 作为 `environment` 回退。正式镜像的内置值不能被环境变量覆盖，非法或非 64 位小写 SHA-256 会在启动前被拒绝。
- 发布准备 API 会返回 `sourceFingerprintAuthoritative`、`sourceFingerprintSource` 和 `candidateGateEnabled`。查询参数、证据或候选提交的指纹与运行版本不一致时返回 `409` 且不写库；运行版本未配置权威指纹时，证据不能标记为 `passed`、候选不能生成并返回 `503`；候选未提交指纹时由服务端自动绑定当前运行版本。
- 总后台发布准备中心自动回填运行版本指纹并设为只读，展示“构建产物”来源；候选请求不再把浏览器字段作为信任输入。未配置权威指纹时按钮禁用并显示运行版本配置状态，平台 RBAC 仍继续控制读取和管理权限。
- `scripts/smoke_saas_release_readiness.sh` 已覆盖环境回退、伪造查询/证据/候选指纹拒绝、六项证据同指纹放行、自动绑定候选、工件完整性、审计和迁移回滚。`scripts/smoke_standalone_compose_app.sh` 会计算宿主指纹并断言 Docker 镜像日志和发布准备 API 返回完全相同的 `source=build` 指纹，防止构建上下文漏文件或镜像指纹漂移。
- 最终 `scripts/test.sh`、发布门禁专项 smoke 和完整 Compose app smoke 均通过；全套 `core/saas/workers/cron/frontend/mysql57` 短证据包已重建，14 条结果全部返回 0，路由 `224/224`、验收脚本 `93/93`、前端 dist API `209/209`、功能矩阵 runtime unique paths `576/576`、runtime route entries `712`。
- 当前源码与验收指纹为 `afa9d96b6feb1ed41c05b9ccaf5a9c0a41bd5a4c405aeb2cc2abcd49b085dacb`，纳入 785 个文件；`docs/evidence/latest/source-fingerprint.json` 与当前源码复核匹配。预览镜像已原位更新，日志确认 `source=build`，API 返回权威指纹和候选门禁启用。
- Playwright 已在 `1440x1000` 与 `390x844` 验证指纹自动填充、只读属性、来源标题、按钮状态和发布准备布局，控制台 0 error；截图保存在 `output/playwright/saas-release-fingerprint/desktop.png` 和 `mobile.png`。
- 当前预览继续保留在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，账号 `13800000090`。没有运行 `standalone_soak_24h.sh`；按用户要求，本轮不再执行 24 小时持续运行。
- 目标审计仍为 `goal_complete=false`，唯一缺口类别为外部生产证据：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/监控记录。六项证据必须使用当前运行版本指纹，并同时固化实际工件 SHA-256 与字节大小。

## 阶段补充（2026-07-12 服务账号 OpenAPI 限流与用量治理 0059 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、脚本、迁移、构建和浏览器测试均直接执行，未启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0059_saas_service_account_rate_limits` 及完整 down rollback：服务账号增加每分钟成功请求上限、每日成功请求上限、当前分钟/自然日窗口、成功请求与限流拒绝计数，并创建 `mochat_go_saas_service_account_usage_daily`，按服务账号、自然日和规范化 `METHOD path` 聚合成功/拒绝用量。默认值为每分钟 60、每日 10000；每日上限为 0 时不限。
- OpenAPI 鉴权后的用量消费使用 MySQL 事务和 `FOR UPDATE` 锁定服务账号与 API Key，在同一事务内重新验证租户、账号、密钥、过期状态，重置时间窗口、判断限额、更新账号/密钥使用摘要并写入路由日用量。多实例并发共享同一数据库窗口，不依赖单进程内存计数。
- 触发限流时返回 HTTP `429`，同时返回 `Retry-After`、`X-RateLimit-Limit/Remaining/Reset` 和每日限额响应头；响应体包含当前分钟/每日计数、剩余量、重置时间和 `minute/daily` 限制来源。API Key 明文与摘要均不进入用量表、列表响应或操作审计。
- 总后台“服务账号与 API Key”中心新增每分钟和每日限额编辑字段、今日成功请求、今日限流拒绝、当前分钟请求、触发限流账号四项概览，并在账号列表展示当日/当前分钟进度、拒绝次数、累计成功、最近来源 IP 以及前三条路由级用量。服务账号表使用明确列宽和内部横向滚动，手机端不会把七列内容压缩到不可读宽度。
- 新路由用量表已纳入租户合规导出与擦除清单。当前合规清单由 154 项扩展为 155 项，可恢复擦除由 157 步扩展为 158 步；真实合规 smoke 已验证用量 JSONL 进入加密导出，并在服务账号父记录删除前清除路由用量，不留下孤儿数据。
- 当前迁移基线为 `59/0059_saas_service_account_rate_limits`。`scripts/smoke_schema_migrate.sh` 已通过空库 apply、checksum、status、重复 apply、baseline、legacy 升级、0059 字段/表/索引检查、0059/0058/0057 等顺序回滚和完整重放；MySQL 5.7 静态门禁通过 117 个 schema/迁移文件。amd64 CI 脚本已增加 59/0059 和新表字段断言，本机 ARM 按规则不伪造 MySQL 5.7 amd64 生产证据。
- `scripts/smoke_saas_service_accounts.sh` 已在真实 Go + MariaDB + Redis 上通过，覆盖限额配置、两次放行、第三次 429、响应头、账号窗口计数、路由用量、非法限额拒绝、Scope/IP 门禁、密钥轮换/宽限/吊销和审计脱敏。全量 Go 测试、发布准备、备份恢复、系统健康、合规生命周期、独立交付包和功能矩阵审计均通过；功能矩阵仍为 manifest `224/224`、runtime unique paths `576/576`、runtime route entries `712`。
- 已用真实预览数据验证限额保存和 OpenAPI 调用：账号 `browser_probe` 配置为每分钟 75、每日 15000，当前日用量为 2，路由 `GET /api/saas/v1/whoami` 为 2 成功/0 拒绝。Playwright 在 `1440x1000` 与 `390x844` 视口完成编辑器、概览、路由用量和稳定列宽验收；截图位于 `output/playwright/saas0059-service-account/desktop.png`、`desktop-usage.png`、`mobile-editor.png` 和 `mobile-section.png`。
- 预览数据库迁移前备份为 `/tmp/mochat-preview-before-0059.sql`，大小 686948 字节，SHA-256 为 `058fb2ddee589380a27574982de4979c8ec52aeda8fe1e7bbb987eb5f31accf1`；原位应用 0059 并重建应用后，App、MariaDB 和 Redis 均健康。预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，数据库账本确认 59 个迁移且最新版本为 0059。
- 最终源码与验收指纹为 `016886fa48504708f9d1cfee442f9f289ce5e46aa03731371d1304476ca191c2`，纳入 787 个文件。Docker 日志确认正式预览镜像以 `source=build` 内置同一指纹；`docs/evidence/latest` 已按 `core/saas/workers/cron/frontend/mysql57` 六套短验收重建，`results.jsonl` 为 `14/14` 返回 0，且明确记录未发现 24 小时进程。
- 当前仍不能标记生产最终完成。`docs/evidence/production/current/index.md` 和 `docs/evidence/production/readiness.md` 继续列出六项外部证据缺口：MySQL 5.7 amd64 真实容器、真实企业微信、真实微信开放平台、真实 SaaS 多租户业务数据、生产前端浏览器回归和目标环境短稳/监控记录；不再默认要求本机执行 24 小时 run。

## 阶段补充（2026-07-12 服务账号历史用量与保留治理 0060 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、构建、测试和浏览器验收均直接执行，未启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0060_saas_service_account_usage_retention` 及完整 down rollback：租户合规策略增加服务账号用量保留天数，默认 90 天、有效范围 1–3650 天。当前迁移基线为 `60/0060_saas_service_account_usage_retention`，系统健康、备份恢复、Compose 初始化和 MySQL 5.7 静态断言已同步。
- 新增 `GET /dashboard/saasAdmin/serviceAccountUsage`，支持 7/30/90 天、租户和服务账号筛选，返回汇总、按日补零序列、路由排行、账号排行、最早记录和保留策略元数据。平台超管和审计角色可读，业务租户不能跨租户读取。
- 新增服务账号用量清理管理器、`cron-saas-service-account-usage-cleanup` 和维护动作 `cleanup-service-account-usage`。清理按租户策略计算包含当日的截止日期，分批删除过期日用量，对有效法律保留租户全量保护，并返回可清理、受保护、已删除和剩余行数；自动任务全部进入后台任务账本。
- SaaS 总后台“服务账号与 API Key”新增“OpenAPI 历史用量”：包含 7/30/90 天分段控件、账号筛选、六项概览、每日趋势、路由/账号排行；合规策略编辑器同步增加用量保留天数。零用量日不显示误导柱，宽表只在内部容器滚动。
- `scripts/smoke_saas_service_accounts.sh` 已在真实 Go + MariaDB + Redis 上通过，覆盖空数据和真实用量报告、7 天补零、路由/账号聚合、非法 91 天拒绝、租户/RBAC 门禁、真实维护清理、启动即跑 cron 记录、法律保留保护和 API Key 脱敏。迁移、合规、系统健康、发布准备和灾备回归均通过。
- `scripts/test.sh` 通过全部 Go 单测、`go vet ./...`、五个命令构建、独立性和静态审计；验收脚本 `93/93`，manifest 路由 `224/224`，功能矩阵 runtime unique paths `577/577`、runtime route entries `713`，SaaS 总后台与平台运营路径 169 条。`lint_mysql57_schema.sh` 已通过 119 个 schema/迁移文件；本机 ARM 仍不伪造 amd64 MySQL 5.7 实库证据。
- 真实预览库迁移前备份为 `/tmp/mochat-preview-before-0060.sql`，大小 700527 字节，SHA-256 为 `b0946258f16148ab104c12241ab08e6b638569e55db5a9adf9ced941a13b9485`。原位应用 0060 并重建应用后，App、MariaDB 和 Redis 均健康，策略值为 90；预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`。
- Playwright 已在 `1440x1000` 与 `390x844` 验收历史用量、7/30/90 天切换、账号筛选、保留策略和内部滚动。桌面端和移动端均满足 `pageScrollWidth=clientWidth`，手机端趋势图和两张宽表仅内部滚动，控制台 0 error/0 warning；截图位于 `output/playwright/saas0060-usage/desktop-history-stacked.png`、`mobile-history.png` 和 `mobile-compliance-policy.png`。
- 最终源码与验收指纹为 `98f74214994ac1cda582fb625629848615612f7c94926212d1987764050e4d11`，纳入 791 个文件；Docker 日志确认预览镜像以 `source=build` 内置同一指纹。`docs/evidence/latest` 已按 `core/saas/workers/cron/frontend/mysql57` 六套重建，`results.jsonl` 为 `14/14` 返回 0，并明确记录未发现 24 小时进程。
- 生产证据 doctor、preflight 和 `current` 包已用当前指纹刷新；`missing_count=6`、`not_ready_count=6`、未访问生产、未启动 24 小时运行。当前仍不能标记生产最终完成，唯一缺口类别仍是六项外部证据：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/监控记录。

## 阶段补充（2026-07-12 服务账号 OpenAPI 用量告警与通知闭环 0061 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、构建、测试和浏览器验收均直接执行，未启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0061_saas_service_account_usage_alerts` 及完整 down rollback：服务账号增加用量告警开关、日用量百分比阈值、每日限流拒绝数阈值、重复通知冷却时间、最近评估时间及两类最近通知时间，并增加按最近评估时间和账号 ID 扫描的索引。策略或每日限额变化时会清空对应评估/冷却时间，使新策略优先重新评估，不沿用旧策略的通知冷却。
- 新增账号级用量评估器和 `POST/PUT /dashboard/saasAdmin/serviceAccountUsageAlertEvaluate`。评估在事务内锁定服务账号，按自然日成功请求占比和限流拒绝数生成或恢复统一 `mochat_go_saas_alerts`，并通过现有租户通知策略写入 `mochat_go_saas_alert_notifications`；同类告警遵守冷却和通知 key 幂等。条件恢复时自动解决告警，同时关闭尚未送达的 `pending/failed` 过期通知、清空重试时间并返回 `notificationsClosed`，避免恢复后继续发送失效预警。
- 用量告警已接入维护动作 `evaluate-service-account-usage-alerts`、可选任务 `cron-saas-service-account-usage-alert` 及启用、间隔、启动即跑和单批上限配置。API、维护命令和 cron 共用同一评估服务，运行实例与 periodic tick 进入后台任务账本，人工评估写入 `saas.admin.service_account.usage_alert.evaluate` 操作审计。
- SaaS 总后台“服务账号与 API Key”新增预警策略编辑、启用预警账号概览、立即评估、评估结果、打开预警计数和“查看预警”联动；账号行展示阈值、冷却、最近评估与最近通知。`platform.integrations.read/manage` 分别控制读取和评估，预警联动仍要求 `platform.notifications.read`。
- `scripts/smoke_saas_service_accounts.sh` 已在真实 Go + MariaDB + Redis 上覆盖 61 个迁移、策略校验、策略变更优先重评、百分比与拒绝数双告警、冷却去重、维护命令、cron、RBAC、操作审计、自动恢复和两条过期通知关闭。`scripts/smoke_schema_migrate.sh` 覆盖 0061 字段/索引、空库 apply、checksum、status、重复 apply、baseline、legacy 升级、逐层 rollback 与重放；MySQL 5.7 静态门禁覆盖 121 个 schema/迁移文件。
- 全量 SaaS 验收首次暴露域名投递完成任务与域名生命周期命令之间的 MySQL `1213` 死锁。根因是完成/回调路径先锁 job，而域名生命周期路径先锁 tenant/domain/delivery，形成反向锁序。当前完成和回调统一先锁 tenant/domain/delivery、再锁 job，并对 MySQL `1205/1213` 最多重试 3 次；域名 smoke 增加日志断言，只要出现 `Deadlock found` 或 `Lock wait timeout exceeded` 即失败。专项域名 smoke 连续通过两次，最终完整 SaaS 套件再次通过且日志无死锁/锁等待。
- `docs/evidence/latest` 已用最终源码完整重建：`results.jsonl` 为 `14/14` 返回 0，`core/saas/workers/cron/frontend/mysql57`、运行时路由、前端 dist API、阶段报告、生产证据和目标审计均通过或成功生成。验收脚本为 `93/93`，manifest 路由 `224/224`、缺失 0，功能矩阵 runtime unique paths `578/578`、runtime route entries `715`，前端 dist API `209/209`、缺失 0。
- 最终源码与验收指纹为 `50bb58a0fec56b9777e4061a16629aa7f5997c78b5cd4b51b62123b7c0f16c0f`，纳入 797 个文件。真实预览库迁移前备份为 `/tmp/mochat-preview-before-0061.sql`，大小 706062 字节，SHA-256 为 `9518dceefbc8dfadca2afcb8129113a4b2844ce5656442d103bca8922586b2d1`；当前数据库账本为 `61/0061_saas_service_account_usage_alerts`，业务镜像为 `sha256:d4e347a2fa5f271f89fb4b3a82acc868e4be2c2d18b6c855255ccd027a8697e3`，App、MariaDB 和 Redis 均健康。
- Playwright 已用真实 JWT 在最新预览执行“立即评估”，页面返回“已评估 1 个，预警 0 个，入队 0 条，关闭过期通知 0 条”；服务账号 `browser_probe` 的历史用量、阈值、Key 生命周期和最近评估均真实显示。桌面与 `390x844` 移动端均无页面级横向溢出，移动端 `documentScrollWidth=documentClientWidth=390`，宽表仅在内部滚动。最新截图位于 `output/playwright/saas0061-usage-alerts/final-desktop-latest.png`、`final-mobile-latest.png` 和 `final-mobile-alert-center-latest.png`；控制台仅保留填写 JWT 前的预期 `401`，授权后请求均正常。
- 生产证据 doctor 与 `docs/evidence/production/current` 已用当前指纹重新生成；诊断结果为 `missing_count=6`、`not_ready_count=6`，严格证据检查和 skip-local 候选门禁按预期返回失败，且报告明确记录 24 小时持续运行未启动。该失败只表示六项外部生产证据缺失，不影响本地 `14/14` 短证据结论。
- 当前预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，账号 `13800000090`。本地独立 Go SaaS 和总后台增量已收口，但目标仍不能标记生产最终完成：尚缺 MySQL 5.7 amd64 真实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端浏览器和目标环境短稳/外部监控六项生产证据。

## 阶段补充（2026-07-13 总后台审计完整性治理 0062 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮直接执行本地命令、迁移、构建、真实数据库 smoke 和浏览器验收，未启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0062_saas_audit_integrity` 及完整 down rollback：操作日志增加前序摘要、当前摘要和算法版本；新增按受影响租户维护的摘要链状态表与校验历史表；`platform_operations` 获得 `platform.audit.manage`，`platform.audit.read` 保持既有角色分配不变。
- 既有日志首次封存时按租户生成 deterministic legacy 聚合锚点，后续日志写入 SHA-256 结构摘要链。受保护字段包括日志 ID、租户、操作人、动作、目标类型/ID 和创建时间；目标名称、前后 JSON 与备注不参与结构摘要，因此合规脱敏后链仍可验证。
- 新增审计完整性总后台 API、维护动作和可选 cron。人工、维护和自动任务共用同一校验服务，可发现 legacy 锚点漂移、前序摘要断链、内容摘要不匹配、链头/数量不匹配和未知算法版本，并将失败日志 ID、错误原因与链状态持久化。
- 总后台新增“审计完整性治理”区，展示健康/异常链、结构日志、legacy 日志、待封存租户、保留期清理预估、租户链头和校验历史。页面随总览并行加载；Playwright 在 `390x844` 下确认页面宽度与文档宽度均为 390、无全局横向溢出，授权后控制台 0 error。最终移动端截图为 `.playwright-cli/audit-integrity-final-mobile.png`。
- 新增两张表后，系统健康真实回归首先发现合规清单未登记风险。当前已把摘要链与校验历史作为审计留存数据纳入租户合规导出和擦除编排，清单由 155 项扩展为 157 项、可恢复擦除由 158 步扩展为 160 步；完整合规 lifecycle smoke 和系统健康 smoke 随后通过。
- `scripts/smoke_saas_audit_integrity.sh` 覆盖 62 版迁移、legacy 封存、cron 后台任务账本、读写 RBAC、业务租户越权拒绝、人工结构日志、直接篡改定位、恢复校验、可脱敏字段、维护命令与页面路由。RBAC 和域名专项中的权限目录固定计数同步从旧口径更新为 API 目录 32 项、数据库实际分配权限码 31 个。
- `scripts/test.sh` 通过全部 Go 测试、独立性与覆盖审计；验收脚本为 `94/94`，manifest 路由 `224/224`、缺失 0，功能矩阵 runtime unique paths `580/580`、runtime route entries `718`，SaaS 总后台与平台运营路径 172 条，前端 dist API `209/209`。
- 完整 `standalone_acceptance` SaaS 套件最终从头到尾通过，结束时间为 `2026-07-13 01:09:02 CST`。`docs/evidence/latest/results.jsonl` 已整理为 `14/14` 返回 0，指纹复核通过；最终源码与验收指纹为 `b1b130683b0fc39028242a22c766a5276a53fa660def6c4fbaa8d9aa980287e4`，纳入 805 个文件。
- 预览数据库迁移前备份为 `/tmp/mochat-preview-before-0062.sql`，SHA-256 为 `abfe9434186e7481acc54eaebbd398f9a7658cb4fc9cf12ab60dd6550a8e7953`。当前账本为 `62/0062_saas_audit_integrity`，最终镜像为 `sha256:2d0aeda6dd55b061b7ec5ad9b569d9c1b82fe1a6abf968b0003a27df66113191`，运行日志确认 `source=build` 且内置最终指纹；App、MariaDB 与 Redis 均健康。
- 预览维护复核扫描 4 条租户链，结果为健康 4、失败 0、legacy 62、结构日志 1。摘要链适合检测普通越权修改、删除和断链，但不能单独防范可同时改写日志、摘要与链状态的数据库高权限攻击者；该威胁模型仍需外部 WORM 或签名锚定。
- 当前预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，账号 `13800000090`。目标仍不能标记生产最终完成，唯一缺口类别仍是六项外部证据：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控记录。

## 阶段补充（2026-07-13 总后台审计签名锚点 0063 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、构建、真实数据库 smoke 和浏览器验收均直接执行。按用户要求，没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0063_saas_audit_anchor_signatures` 及完整 down rollback：新增签名检查点表，固化租户、摘要链版本、legacy 锚点、结构链头、签名日志数量、规范化载荷摘要、HMAC-SHA256 签名、密钥 ID、独立证据摘要和校验结果。当前数据库基线为 `63/0063_saas_audit_anchor_signatures`。
- 新增 `internal/saasauditanchor` 领域服务。创建流程读取健康摘要链头、生成确定性载荷和 HMAC-SHA256 签名、以 `0600` 权限原子写入独立 JSON 证据并回写摘要；校验流程支持历史密钥环，同时验证载荷、签名、证据文件、链头、日志数量和检查点顺序，可发现数据库检查点删除、数量回退和孤儿证据文件。
- 总后台新增 `GET /dashboard/saasAdmin/auditAnchors` 与 `POST/PUT /dashboard/saasAdmin/auditAnchor`，复用 `platform.audit.read/manage` 分离读取与执行权限。审计完整性区域增加锚点状态、HMAC 密钥 ID、独立证据、校验结果、回退检测、检查点列表和创建/校验操作；系统健康中心同步增加锚点配置与校验探针。
- 新增维护动作 `create-audit-anchor`、`verify-audit-anchor` 和可选任务 `cron-saas-admin-audit-anchor`。API、维护命令和 cron 共用同一服务，任务运行与 periodic tick 进入后台任务账本；standalone Compose 使用独立 `audit-anchor-storage` 卷，镜像显式创建并把 `/app/audit-anchors` 归属到 UID 10001，避免非 root 运行时无法写入新卷。
- 两张 0062 完整性表和一张 0063 签名检查点表均已纳入租户数据治理，合规清单扩展到 158 项，可恢复擦除为 161 步。签名检查点按审计留存，不随普通租户数据擦除；生产环境仍必须把证据目录放到数据库管理员不可改写的独立或 WORM 存储。
- 完整回归还修正了两个验收契约问题：域名生命周期允许较新的动作以明确原因取消旧任务，断言改为校验“成功加取消”的合法终态且禁止失败任务；S3 灾备健康检查增加独立副本存储探针后，专项 smoke 同步按 25 项检查并为审计锚点配置独立测试密钥和证据目录。
- `scripts/smoke_saas_audit_anchor.sh`、系统健康、合规生命周期、备份恢复、发布准备、RBAC、域名、完整 SaaS、worker、cron、frontend 和 MySQL 5.7 静态门禁均通过。迁移 smoke 覆盖 63 个版本的空库 apply、checksum、status、重复 apply、baseline、legacy 升级、0063 至 0052 逐层回滚与完整重放；静态门禁覆盖 125 个 schema/迁移文件。
- 最终覆盖为 smoke `95/95`、manifest 路由 `224/224`、runtime unique paths `582/582`、runtime route entries `721`、SaaS 总后台与平台运营路径 174 条、前端 dist API `209/209`。`docs/evidence/latest/results.jsonl` 在 `2026-07-13 03:06:38 CST` 至 `03:41:19 CST` 连续重建，14 组全部返回 0；SaaS 套件结束时间为 `03:22:02 CST`。
- 最终源码与验收指纹为 `8749791de0fcdcb3aa7a59e6d7698ade47090c3898c10b981f4c245bb12d9081`，纳入 814 个文件。预览镜像为 `sha256:78dbda776f682cc29f4cf4d85aa8d8542ca2859d9395f61619859959d22be00f`，运行日志确认 `source=build` 并启动签名锚点 cron；当前 7 个检查点全部通过、7 个证据文件均为 `0600` 且归属 UID 10001、异常 0、待验 0、孤儿文件 0。
- 预览迁移前备份为 `/tmp/mochat-preview-before-0063-20260713-020153.sql`，大小 745608 字节，SHA-256 为 `ebad5609535646e1bc9c7752f98cb64f8d89617193371401961298095d81ea65`，权限为 `0600`。Playwright 在 `1440x1000` 和 `390x844` 验证最新镜像，授权后控制台 0 error/0 warning，移动端 `documentScrollWidth=documentClientWidth=390`；截图位于 `output/playwright/saas0063-audit-anchor/final-current-desktop-anchor.png` 和 `final-current-mobile.png`。
- 生产证据 doctor 与 `docs/evidence/production/current` 已按最终指纹刷新，结果为 `missing_count=6`、`not_ready_count=6`，严格证据包按预期未通过。当前仍不能标记生产最终完成，唯一缺口类别仍是 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项证据。
- 当前预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`；登录凭据仅保存在本地受控配置，不在文档记录。

## 阶段补充（2026-07-13 审计锚点异地不可变证据 0064 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、构建、真实存储 smoke 和浏览器验收均直接执行。按用户要求，没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0064_saas_audit_anchor_remote_immutability` 及完整 down rollback。签名检查点增加远端 provider、bucket、object key、version ID、ETag、SHA-256、字节数、保留模式/到期时间、导出/校验时间和失败原因。当前数据库基线为 `64/0064_saas_audit_anchor_remote_immutability`。
- 新增 `s3-object-lock` 远端证据存储实现：启动探测要求 bucket 已存在且已启用 Object Lock；支持 compliance/governance 保留，默认 3650 天，有效范围 1–36500 天；上传会保存对象版本与 SHA-256 元数据，校验会精确比对字节、摘要、大小、版本和未到期保留策略。
- 远端列表使用对象版本而不只是当前 key，可检测数据库检查点删除/回退后遗留的孤儿证据。完整回归发现存量 0063 检查点不再是当前链头时不会自动上传，已新增历史待传查询和分批回填，按最早检查点优先上传，API、维护命令和 cron 都返回回填数。
- 总后台“审计完整性治理”新增远端强制状态、provider/bucket、保留策略、导出/失败/待传/孤儿统计和对象版本明细。系统健康新增 `audit_anchor_remote_store` 关键探针；当前预览探针状态为 `healthy`，详情为“连接正常”，迁移探针为 `64/64`。
- 真实 MinIO 以 `--with-lock` 创建 bucket，使用 7 天 compliance 保留通过专项 smoke：导出与校验成功，受保护版本删除尝试被存储服务拒绝，远端缺失和孤儿版本均被正确检出。`smoke_saas_audit_anchor.sh`、系统健康、迁移 rollback、单元/集成测试与维护命令全部通过。
- `scripts/smoke_schema_migrate.sh` 已覆盖 64 个版本的空库 apply、checksum、status、重复 apply、baseline、legacy 升级、0064 down/up 和完整重放；MySQL 5.7 静态门禁通过 127 个 schema/迁移文件。合规清单保持 158 项，可恢复擦除保持 161 步。
- `docs/evidence/latest` 已用最终源码从 `2026-07-13 04:46:47 CST` 至 `05:23:34 CST` 完整重建，`results.jsonl` 为 `14/14` 返回 0。覆盖结果为 smoke `95/95`、manifest 路由 `224/224`、runtime unique paths `582/582`、runtime route entries `721`、SaaS/总后台路径 `174`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `7781d0d8d5f16aa110af7f5a302591a0b84eb226b6ecbf8f80d5fec7980fef34`，纳入 818 个文件。预览最终镜像为 `sha256:a7395de448d46fdf4073a17fa70502afe76d3caf6a618a72066ae273a0f1b35e`，日志确认 `source=build` 并内置同一指纹；启动 cron 返回 `chains=4 created=1 existing=3 backfilled=0 exported=4 verify_passed=11 verify_failed=0`。
- 当前预览共 11 个检查点，11 个均已远端导出并校验通过，远端失败、待传、本地孤儿和远端孤儿均为 0。迁移前备份为 `/tmp/mochat-preview-before-0064-20260713-042217.sql`，大小 769749 字节，SHA-256 为 `e83af4f6383d160658910e68e59de16c34de2ec2d80a6b0df7248c11edb636ea`，权限为 `0600`。
- Playwright 已在 `1440x1000` 和 `390x844` 验收远端锚点总览和明细表，授权后控制台 0 error/0 warning，移动端 `documentWidth=bodyWidth=390`，无页面级横向溢出，宽表仅在内部滚动。截图位于 `output/playwright/saas0064-audit-anchor/final-desktop-remote-anchor.png`、`final-mobile-remote-anchor.png` 和 `final-mobile-remote-table.png`。
- 当前预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，登录凭据仅保存在本地受控配置。生产证据报告仍为 `missing_count=6`、`goal_complete=false`；唯一未收口类别是 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部生产证据。

## 阶段补充（2026-07-13 服务账号 API Key 独立 pepper 密钥环 0065 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、构建、真实数据库 smoke 和浏览器验收均直接执行。按用户要求，没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0065_saas_service_account_key_pepper_ring` 及 down rollback，服务账号 Key 增加 `hash_key_id`。新增 `internal/serviceaccountkey` 密钥环管理器，支持当前独立 pepper、JSON 历史密钥环和受控 `legacy-jwt` 兼容，生产模式可强制要求独立 pepper。
- 总后台服务账号页已展示活动 key ID、独立/兼容状态、待轮换旧 Key 和缺失 key ID；系统健康新增关键探针 `service_account_key_protection`。当前预览使用 `preview-service-account-v1`，`dedicatedConfigured=true`、`requireDedicated=true`、`legacyJwtEnabled=false`，可用独立 Key 1 个，可用旧 Key 0 个，缺失 key ID 0 个。
- `scripts/smoke_saas_service_accounts.sh` 已在真实 Go + MariaDB + Redis 上验证独立 Key 鉴权、JWT pepper 旧 Key 兼容、关闭兼容后 401、缺密钥时系统健康 critical、恢复兼容和最终轮换。迁移 smoke 覆盖 65 个版本，MySQL 5.7 静态门禁覆盖 129 个 schema/迁移文件。
- 一次 standalone Compose smoke 使用了与预览相同的 project name，清理时移除了预览 App/MySQL/Redis 卷。已从 `/tmp/mochat-preview-before-0064-20260713-042217.sql` 恢复到 0063 并重放 0064/0065；WORM 存储改用新世代前缀 `preview/audit-anchors-0065-recovered`，保留旧前缀不变。
- 恢复后加密备份 run 7 已验证成功并写入 MinIO 副本；明文恢复快照为 `/tmp/mochat-preview-recovered-0065-20260713-063420.sql`，权限 `0600`，大小 809437 字节，SHA-256 为 `c727d1755a22d88a269fc63c898488140be7eb03ca6a58aa946cf90d3108c14c`。当前审计锚点检查点 10 个，已全部远端导出并校验通过，失败 0。
- `docs/evidence/latest` 已用最终源码从 `2026-07-13 06:41:12 CST` 至 `07:18:56 CST` 完整重建，`results.jsonl` 为 `14/14` 返回 0。覆盖快速门禁、独立交付包、迁移期清单、`core/saas/workers/cron/frontend/mysql57`、运行时路由、前端 dist API、阶段报告、生产证据报告和目标完成度报告。smoke `95/95`、manifest 路由 `224/224`、runtime unique paths `582/582`、runtime route entries `721`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `9d548af5057ebed35cad779085adfd3a8e998ea6df75ef900900d98528f3708a`，纳入 822 个文件。预览镜像为 `sha256:f22b2ac3e8f646874fac7bfb5a0aed043a02751200f3f813d213dba97222dac4`，日志确认 `source=build` 且指纹一致；App、MariaDB 和 Redis 均健康。
- Playwright 已在 `1440x1000` 和 `390x844` 验收 API Key pepper 密钥环，授权后控制台 0 error/0 warning，移动端无页面级横向溢出；截图为 `output/playwright/saas0065-pepper/final-desktop-service-account.png` 和 `final-mobile-service-account.png`。
- 当前系统健康不是全绿：超过 SLA 的待审批单 1 条，统计窗口内失败后台执行 3 次。密钥环、迁移和远端审计锚点探针健康，但不将整体健康误报为全绿。
- 当前预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，登录凭据仅保存在本地受控配置。生产证据仍缺 6 项：MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控，因此目标仍不能标记生产最终完成。

## 阶段补充（2026-07-13 受信代理 CIDR 客户端 IP 安全收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、构建、真实数据库 smoke、独立 Compose 和浏览器验收均直接执行，没有启动 `standalone_soak_24h.sh` 或其他 24 小时运行。
- 审计发现两处真实生产边界缺口：服务账号 IP 白名单只读取 TCP `RemoteAddr`，部署在反向代理后无法识别真实客户端；身份安全开启代理头后会无条件信任 `X-Forwarded-For`/`X-Real-IP`，直接可达时存在来源伪造风险。
- 新增共享 `internal/clientip` 解析器。默认完全忽略转发头；只有 TCP 对端命中显式受信 CIDR 时才按反向代理链从右向左解析 `X-Forwarded-For`、跳过受信跳点并选择第一个非受信地址，再回退到 `X-Real-IP` 或 TCP 对端。解析器覆盖 IPv4、IPv6、IPv4-mapped、单 IP 归一化、CIDR 去重排序和非法配置测试。
- 新增 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS` 与共享 `MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS`，保留既有 `MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS`。任一代理头开关启用但受信 CIDR 为空时配置加载失败；非法 CIDR 始终拒绝，Compose、`.env.example`、根 README 与 standalone README 已同步部署契约。
- 身份安全登录、服务账号 OpenAPI IP/CIDR 白名单、服务账号用量台账和最近使用 IP 已统一使用解析后的客户端地址。总后台服务账号与身份安全概览均增加“来源 IP 解析”状态，API 返回代理头开关、规范化 CIDR 列表和数量；关闭代理头时列表稳定返回 `[]`。
- `scripts/smoke_saas_service_accounts.sh` 已在真实 Go + MariaDB + Redis 上验证 TCP 直连被 `10.0.0.0/8` 白名单拒绝、受信 `127.0.0.1/32` 代理转发 `10.23.45.67` 后允许并写入台账、非受信对端伪造同一头再次被拒绝。`scripts/smoke_saas_identity_security.sh` 同步验证受信代理解析和非受信直连防伪造；独立 Compose App smoke 使用独立项目与备用端口通过。
- 首次完整证据重建时，frontend 套件暴露 Sidebar OAuth 回调的 Playwright 导航竞态：业务回调已返回 `302`，但浏览器把目标页切换报告为导航中断。当前 smoke 以最终目标页到达为准并在失败时重试一次，仍严格检查回调 `302`、JWT/Agent Cookie、客户画像、个人/群 SOP、批量加好友、素材页和全部 Sidebar API；独立专项及完整 frontend 套件均通过。
- 最终 `docs/evidence/latest/results.jsonl` 于 `2026-07-13 11:10:02 CST` 至 `11:55:54 CST` 从头重建，`14/14` 返回 0。覆盖 smoke `95/95`、manifest 路由 `224/224`、runtime unique paths `582/582`、runtime route entries `721`、SaaS/总后台路径 `174`、前端 dist API `209/209`。
- Playwright 已在桌面和 `390x844` 移动端验收服务账号与身份安全来源 IP 状态，移动页面 `scrollWidth=clientWidth=390`，摘要卡片无溢出；控制台 0 error/0 warning，总后台授权请求全部为 `200`。截图位于 `output/playwright/saas0066-client-ip/desktop-service-account.png`、`desktop-identity-security.png`、`mobile-service-account.png` 和 `mobile-identity-security.png`。
- 最终源码与验收指纹为 `c872825e851e0344ab52822fde0634b76b2295c45b0260bb6c9533fe084f1dcc`，纳入 824 个文件。预览镜像为 `sha256:01604006f75aa37e7ddba33b83c5938ec5ccac61231c2520376772591d2546c5`，日志确认 `source=build`；数据库没有新增迁移，仍为 `65/0065_saas_service_account_key_pepper_ring`。当前 13 个审计锚点均已远端导出和校验通过。
- 当前系统健康为 `26/28`，保留 1 条逾期审批 critical 和统计窗口内 3 次失败后台执行 warning。生产证据 doctor、preflight 与 `docs/evidence/production/current` 已按最终指纹刷新，仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 项，因此目标继续保持未完成状态。

## 阶段补充（2026-07-13 SaaS 告警 Webhook SSRF 与出站边界收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、构建、真实数据库 smoke、完整短证据包和浏览器验收均直接执行。按用户要求，没有启动 `standalone_soak_24h.sh` 或其他 24 小时运行。
- 审计发现租户通知策略可持久化任意 Webhook URL，旧发送链路使用默认 HTTP Client，未限制私网、云元数据、环境代理、跨源重定向或 DNS 重绑定。新增共享 `internal/outboundhttp` 防护：默认仅 HTTPS，拒绝凭据、片段、localhost、私网/回环/链路本地/保留网段和已知云元数据端点。
- 域名请求先解析并校验全部 DNS 地址，再直接拨号已校验 IP，保留原 Host/TLS ServerName；Transport 禁用环境代理，重定向最多 10 次且只允许同 scheme、host、有效端口。显式 CIDR 例外支持合法内网通知，但 `/0` 和过宽网段被拒绝，云元数据端点永不受例外放行。
- 新增 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS` 和 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS`，同步 standalone Compose、`.env.example`、根 README、部署 README 与维护命令。全局 fallback、租户数据库策略、outbox、定时派发、员工申请 worker 和维护 CLI 使用同一策略；运行日志不再打印完整 Webhook URL。
- 总后台通知策略中心新增出站安全状态和逐 URL 静态校验结果，展示 HTTPS、私网/元数据阻断、CIDR 例外、DNS 绑定、同源重定向与直连出站。保存接口拒绝私网字面地址，旧数据库危险记录在测试和实际派发前会被运行时防护拒绝。
- 新包单元测试覆盖 IPv4/IPv6/IPv4-mapped、CIDR 规范化与过宽拒绝、混合 DNS 答案、校验后 IP 直连、跨源重定向和元数据硬阻断。`go test ./...`、`scripts/test.sh`、通知策略派发、通知 cron、员工申请 worker 和总后台专项 smoke 均通过。
- `docs/evidence/latest` 已以 `MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=all` 从头重建，`results.jsonl` 为 `14/14` 返回 0；smoke `95/95`、manifest `224/224`、runtime unique paths `582/582`、runtime route entries `721`、前端 dist API `209/209`。最终指纹为 `05dfa71e256233ea67159b4bd0e68b57f312ef10a969cfcefa12cecd2d4d7380`，纳入 827 个文件。
- 预览 App 已原位替换为镜像 `sha256:12e0fca3c7561045eff225eb3d4db2e6673a67cc087d3be95579254cadacd373`，MySQL、Redis、MinIO 与数据卷保留，迁移账本仍为 `65/0065_saas_service_account_key_pepper_ring`。日志确认 `source=build`、最终指纹一致和出站安全全开；真实预览 API 对私网 HTTPS URL 返回 `400`。
- Playwright 在 `1440x1000` 和 `390x844` 验证出站防护状态、通知策略列表与移动布局；移动端文档宽度 390、无页面级横向溢出，状态区不溢出，授权后新页面控制台 0 error/0 warning。截图保存在 `output/playwright/saas-webhook-egress/`。
- 预览重建前快照 `/tmp/mochat-preview-before-webhook-egress-20260713.sql` 为 `0600`、833330 字节、SHA-256 `2ff4803243819e915d4572a7a85982d34508da7354767a16a96538c0721a5f65`。当前系统健康仍为 `26/28`，保留逾期审批 critical 和失败后台执行 warning。
- 生产证据 doctor 与 `docs/evidence/production/current` 已按最终指纹刷新，严格门禁按预期未通过：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 项均缺失。证据包明确记录 24 小时持续运行未启动，目标仍不能标记生产最终完成。

## 阶段补充（2026-07-13 SaaS 告警凭据加密与密钥轮换 0066 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、构建、真实数据库 smoke、浏览器验收和短证据包均直接执行。按用户要求，没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0066_saas_alert_credential_encryption` 及 down rollback，为租户告警策略增加 Webhook 凭据密文和 Key ID 字段。新增 `internal/saasalertcredentials` AES-256-GCM 密钥环，支持活动/历史密钥、存量明文兼容读取、批量轮换、独立密钥强制和不可用密钥诊断。
- 修正配置回退的密钥域混用问题：专用告警、身份、合规和备份密钥现在按完整域选择，单 Key、历史密钥环和 Key ID 不会跨域拼接。主服务与维护命令共用选择器，显式 CLI Key 也会清理不匹配的回退域配置。
- 主服务、通知策略存储、系统健康和维护动作 `rotate-alert-credentials` 已接入同一加密管理器。总后台展示加密/强制/专用/健康状态、活动 Key 和数据分布；轮换按钮必须同时满足管理权限、可用密钥和待轮换数量大于 0，当前待轮换为 0 时禁用并提示“没有待轮换凭据”。
- 集成 smoke 已验证 `legacy plaintext -> q1 -> 缺历史 Key 安全失败 -> q1+q2 轮换 -> q2-only 正常读取`。系统健康、合规生命周期和备份恢复 smoke 已纳入 `saas_alert_credential_protection` 探针；定向测试、完整 `scripts/test.sh` 和三组修正后的健康/合规/灾备专项均通过。
- 最终 `docs/evidence/latest/results.jsonl` 从 `2026-07-13 17:12:42 CST` 至 `18:17:11 CST` 从头重建，`14/14` 返回 0。覆盖为 smoke `95/95`、manifest `224/224`、runtime unique paths `583/583`、runtime route entries `723`、extra routes `209`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `f70391ac1e90436178b8e0517d39cfb66eeb2ed9009eccc501ab2db3ce88ed9b`，纳入 834 个文件。预览镜像为 `sha256:afdbfa0aaa3b52104a106c84a463eb7630d33935060d4896dbcaeff60f9a7f05`，日志确认活动告警 Key 为 `preview-alert-v1`、专用强制加密开启且 `key_count=1`；迁移账本为 `66/0066_saas_alert_credential_encryption`。
- 当前预览凭据保护 API 为 healthy，凭据/密文/旧明文/待轮换/不可用均为 0；系统健康为 `27/29`，保留逾期审批 critical 和失败后台执行 warning。迁移前备份 `/tmp/mochat-preview-before-0066-20260713.sql` 为 `0600`、843695 字节、SHA-256 `7a380edce326b0650ce8000933abc12b99c03f3281bce5d356f5a4dd5d831353`。
- Playwright 在 `1440x1000` 和 `390x844` 验证总后台凭据保护状态、按钮门禁和响应式布局；无页面级溢出或元素重叠，控制台 0 error/0 warning。截图位于 `output/playwright/saas-alert-credential-encryption/desktop-1440x1000.png` 和 `mobile-390x844.png`。
- 生产证据 doctor、preflight 和 current 包已刷新到最终指纹：有效标准文件 `0/6`、采集环境就绪 `0/6`，严格收拢按预期返回 1。仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据，因此目标保持未完成。
- 当前预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`；登录凭据仅保存在本地受控配置，不在文档记录。

## 阶段补充（2026-07-14 企业微信凭据加密与密钥轮换 0067 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、真实数据库 smoke、完整短证据包、Compose 预览和浏览器验收均直接执行。按用户要求，没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0067_wecom_credential_encryption` 及 down rollback，为企业和企微应用凭据增加 AES-256-GCM 密文与 Key ID。新增 `internal/wecomcredentials` 专用密钥环，支持活动/历史密钥、独立密钥强制、旧明文兼容读取、批量轮换、重加密和缺 Key 安全失败。
- `mc_corp`、`mc_work_agent` 的创建和更新、自动标签、会话存档同步、主服务、系统健康和维护动作 `rotate-wecom-credentials` 已统一接入加密管理器。总后台新增企微凭据保护概览与轮换接口，读取/执行权限分离，轮换写入平台操作审计。
- 总后台页面将企微凭据状态改为权限资料加载后非阻塞请求，不再等待后续集成请求串行完成。Playwright 在桌面和 `390x844` 手机视口验证状态、按钮门禁和布局，无重叠或页面级溢出，授权后控制台 0 error/0 warning。
- `scripts/smoke_wecom_credential_encryption.sh` 在真实 Go + MariaDB + Redis 上通过，覆盖旧明文读取、Q1 轮换、历史 Key 缺失时读写与健康门禁、Q2 重加密、Q2-only 读取、新写入即加密、RBAC、维护命令、系统健康和页面路由。备份恢复与合规生命周期专项修正健康检查总数后均独立通过。
- 源码指纹与独立交付规则已排除 `.env`/`.env.*` 私密文件并保留 `.env.example`；独立包 smoke 会断言私密环境文件未复制、未进入指纹。独立 Compose App smoke 使用隔离项目与端口通过，未影响保留预览。
- 最终 `scripts/test.sh` 通过，smoke 纳管 `96/96`、manifest 路由 `224/224`、runtime unique paths `585/585`、runtime route entries `726`、SaaS/总后台路径 `177`、前端 dist API `209/209`。完整短证据包于 `2026-07-14 09:41:20 CST` 至 `10:35:42 CST` 重建，14 条结果全部返回 0。
- 最终源码与验收指纹为 `dece0dcd6fedb810558c5577390db8f2b870be5114958bd4dbd3109bc211241b`，纳入 845 个文件。预览镜像为 `sha256:41b9a9ef0a78117ef117c08d97808c546f615319d8f1b9fbd20a19922917a134`，日志确认 `source=build`、内置指纹一致、专用强制企微凭据加密开启且活动 Key 为 `preview-wecom-v1`。
- 预览迁移账本为 `67/0067_wecom_credential_encryption`；企业/应用明文行均为 0，企业密文行 1、应用密文行 0，待轮换与不可用均为 0。系统健康为 `28/30`，企微凭据探针 healthy，保留逾期审批 critical 和备份新鲜度 warning。
- 生产证据 doctor 与 `docs/evidence/production/current` 已按最终指纹刷新，仍为 `missing_count=6`、`not_ready_count=6`，严格证据和候选门禁按预期失败。目标仍不能标记生产最终完成，剩余是 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据。

## 阶段补充（2026-07-14 微信开放平台凭据加密与密钥轮换 0068 收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮本地命令、迁移、真实数据库 smoke、完整短证据包、Compose 预览和浏览器验收均直接执行。按用户要求，没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。
- 新增迁移 `0068_wechat_open_credential_encryption` 及 down rollback，为第三方平台 Ticket 和公众号授权凭据增加 AES-256-GCM 密文与 Key ID。新增 `internal/wechatopencredentials` 专用密钥环，支持活动/历史密钥、独立密钥强制、旧明文兼容读取、批量轮换、重加密和缺 Key 安全失败。
- Ticket 接收、公众号授权回跳、取消授权、消息回调、存储读取、主服务、系统健康和维护动作 `rotate-wechat-open-credentials` 已统一接入凭据管理器。总后台新增保护概览与轮换接口，沿用集成读取/管理 RBAC，轮换记录平台操作审计，接口和审计均不返回明文或密文。
- `scripts/smoke_official_account_ticket.sh` 已在真实 Go + MariaDB + Redis 上验证旧明文 Ticket/公众号凭据、总后台轮换、回调写密文、缺历史 Key 失败、双 Key 重加密和新 Key-only 读取。首次实库执行发现候选 `UNION` 的 MariaDB collation 冲突，现改为两条确定性排序查询并通过回归。
- `scripts/test.sh`、迁移 apply/down/up、release readiness、MySQL 5.7 静态门禁、独立包和完整 SaaS acceptance 均通过。最终 `docs/evidence/latest` 于 `2026-07-14 12:27:37 CST` 至 `13:14:49 CST` 重建，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `0cf803a1e598446f8e4af86eee5b3a1755f428eda70e915df38ecd23d70d2c1c`，纳入 855 个文件。预览 App 镜像为 `sha256:ad71ef9a03c4f7d89c3209bc67283a5b3540810b2c2eb012498714e7e756fe09`，启动日志确认 `source=build`、指纹一致、专用强制加密开启、活动 Key 为 `preview-wechat-open-v1` 且 `keyCount=1`。
- 预览迁移账本为 `68/0068_wechat_open_credential_encryption`，运行时路由 706 条。保护 API 为 healthy，Ticket、公众号、旧明文、待轮换和不可用数量均为 0；系统健康为 `29/31`，新探针 healthy，仍有 1 条逾期审批和备份过期两项 critical，未误报为全绿。
- 迁移前快照 `/tmp/mochat-preview-before-0068-20260714-112706.sql` 为 `0600`、900374 字节、SHA-256 `4ca48646c8551b83462d7973902e93e2fd41bb1ae641dd61b791a93cf0d2dc3a`。预览重建仅替换 App，MySQL、Redis、MinIO 和数据卷均保留。
- Playwright 在 `1440x1000` 和 `390x844` 验证凭据保护状态、按钮门禁和响应式布局；无页面级横向溢出，授权后控制台 0 error/0 warning。截图位于 `output/playwright/wechat-open-credential-encryption/`。
- 生产证据 doctor 与 current 包已刷新到最终指纹；严格生产证据、严格目标审计和候选门禁按预期阻断。剩余缺口仍是 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端、目标环境短稳/外部监控 6 项外部证据，目标不能标记生产最终完成。

## 阶段补充（2026-07-14 自动备份调度与健康中心收口）

- 继续执行“彻底 SaaS 化、总后台更加完善”；本轮所有本地命令、数据操作、构建、短验收和浏览器回归均直接执行，没有启动 `standalone_soak_24h.sh` 或任何 24 小时 run。
- 备份管理器配置状态新增自动调度开关、扫描间隔和 run-on-start，`backupOverview` 与总后台灾备中心会直接展示。平台健康新增 critical 检查 `backup_automation`，能识别“备份策略已启用但调度器关闭”和非法扫描间隔。
- 真实预览开启 5 分钟扫描和启动检查，`cron-saas-backup` 已进入 task runner。自动备份 `bkp_20260714T053833Z_1cd7a563e3da` 成功，加密 Key 为 `preview-0051`，迁移 `68/0068`、表 `190` 张、工件 154985 字节；本地完整性验证、S3 副本、SHA-256 和字节数校验均通过。
- 通过业务 API 撤回了超 SLA 的演示租户注销审批，审批状态为 `canceled`、版本 2；事件 `4` 与操作审计 `115/saas.admin.approval.cancel` 已落库。健康扫描 `HSC-20260714T054039-94314F17112A` 随后得到 `32/32 healthy`，问题、critical、warning 和活动事故均为 0。
- 完整本地短证据包现包含 14 条返回 0 的命令结果，`core/saas/workers/cron/frontend/mysql57` 六个套件全部归档；smoke `96/96`、manifest 路由 `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态门禁覆盖 135 个文件，ARM 跳过的实容器不被记为 amd64 证据。
- 最终指纹为 `357041312a9c86b48cc2e8830cb87f7a419ca275d1e3c925991d3fe6d5a608c0`，纳入 855 个文件；预览镜像为 `sha256:88ad810e68c14d07b43c3f0a626361256c3a80b22309fa87e664ae0f1b5d14f5`。证据执行期间 Docker Desktop 一度退出，恢复守护进程后原 MySQL/Redis 数据卷和 App 状态完整，当前三个容器均 healthy。
- Playwright 已在 `1440x1000` 和 `390x844` 验收自动调度状态，两种视口都无页面级横向溢出或操作重叠，控制台 0 error/0 warning。当前数据库快照为 `/tmp/mochat-preview-after-backup-automation-20260714-153824.sql`，权限 `0600`、大小 950112 字节、SHA-256 `c0d8667970ca410f37acf7d8e303e141cefe54503452281c57e0103e0ca28df2`。
- 当前已没有本地代码或预览环境中的备份新鲜度/审批 SLA 阻断项；剩余只是 6 类真实生产外部证据。`docs/evidence/production/current` 已刷新到最终指纹，严格候选门禁因这 6 项缺失而按预期返回失败，目标仍保持未完成。

## 阶段补充（2026-07-14 发布证据远端工件真实性收口）

- 已关闭发布准备中心的关键可信边界缺口：旧流程只校验证据 URL、SHA-256 和字节数是否填写，不能证明 URL 对应内容真实存在。现在保存 `passed` 前必须由 Go 服务端下载 HTTPS 工件并核对实际摘要与大小，任何状态码、超时、超限、摘要或大小不一致都返回 `422`，存储层也拒绝绕过校验器。
- 候选门禁会并行重验六项必需工件，记录实际摘要、大小、HTTP 元数据、校验时间和错误；提交前用行锁比较证据 key、version、URL、摘要与大小，避免网络校验期间发生 TOCTOU 替换。被阻断的候选仍持久化完整失败明细，便于审计和追责。
- 准备状态已拆为 `metadataReady` 和 `ready`。前者只表示元数据齐全，后者要求当前运行二进制的权威源码指纹下最近候选六项远端复核全部通过。总后台同步展示远端通过数量、最近候选编号/状态以及校验器安全策略。
- 新增发布证据专用出站校验配置：默认超时 30 秒、单工件最大 64 MiB，启用 HTTPS、私网/云元数据阻断、DNS 绑定、禁用环境代理和同源重定向限制；可显式放行最小内部 CIDR，并可加载只读自有 CA。生产默认配置不放行私网。
- `scripts/smoke_saas_release_readiness.sh` 使用临时私有 CA HTTPS 工件库与真实 MariaDB/Redis 验证保存时校验、候选时内容篡改阻断、恢复后 `6/6 ready`、候选快照、操作审计和 0068 至 0057 迁移回滚/重放。单元测试、全量 Go 测试、前端 API 审计和六套短验收均通过。
- `docs/evidence/latest` 于 `2026-07-14 16:47:47 CST` 至 `17:24:20 CST` 完整重建，14 条命令全部返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。最终指纹为 `fd9e06ae9e907309a9a44ec26cae9f4de719bd984e12214017a76d73466f1285`，纳入 857 个文件。
- 预览 App 镜像为 `sha256:42a3fbde4305b42003edbed9a3565140d0f7201fa639895abad359e86e120bb6`，迁移账本仍为 `68/0068`。Playwright 桌面、移动和移动弹窗回归通过，移动文档无页面级横向溢出，干净会话控制台 0 error/0 warning。
- 审计锚点当前 `26/26` 远端验证通过，远端失败、待传、缺失和孤儿为 0。平台健康的 1 条 warning 来自 MinIO 暂停期间两次真实后台执行失败，当前服务和锚点已恢复；不篡改这两条历史记录。
- 本轮没有启动任何 24 小时运行。当前预览中的六项发布证据仍缺失，`metadataReady=false`、`ready=false`；最终生产缺口继续是 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据。

## 阶段补充（2026-07-15 发布候选证据漂移失效收口）

- 审计并关闭历史 `ready` 候选在现行证据变化后仍可能保持可发布的时序缺口。候选快照现在必须与现行证据的身份、状态、URL、环境、权威源码指纹、工件摘要/大小、复核信息、备注和版本完全一致，且快照内远端工件复核必须自洽并通过。
- 新增候选有效状态 `stale`。原始状态不被改写，便于保留发布审计；摘要新增 `latestCandidateEffectiveStatus`、`latestCandidateSnapshotValid`、`latestCandidateEvidenceCurrent` 和 `latestCandidateDriftedEvidenceKeys`，候选列表同步返回有效状态与变化项。只要最近候选失效，总后台和 API 均强制 `ready=false`。
- 证据恢复不会复活旧候选，必须重跑六项远端门禁生成新候选。真实 Go + MariaDB + Redis smoke 已完整验证 ready、证据失败、旧候选 stale、证据恢复后仍 stale、新候选重新 ready，以及四次候选门禁和对应操作审计。
- `go test ./internal/dashboard ./internal/store`、发布准备专项 smoke、`scripts/test.sh`、运行时路由覆盖和前端 dist API `209/209` 均通过。完整短证据于 `2026-07-15 09:25:40 CST` 至 `10:12:25 CST` 重建，`results.jsonl` 14 条全部返回 0；没有执行 24 小时 soak。
- 最终源码指纹为 `026b3cef88d869a0e53ac6ce6c0c493d69d2102b981106808ec520ee1a084102`，文件数 857；预览镜像为 `sha256:f4936e44f96039fca314873a84973066e5d0a8e3f7bfa7922f056f80faab3733`，`/readyz` 为 200、运行时路由 706 条、PHP fallback 关闭，MySQL/Redis 健康且 MinIO 正常运行。
- Playwright 在 `1440x1000` 与 `390x844` 验收发布准备中心，页面无横向溢出或元素重叠，移动宽表只在内部滚动，控制台 0 error/0 warning。数据库快照为 `output/backups/mochat-preview-before-release-candidate-drift-20260715-0922.sql`，权限 `0600`、989138 字节、SHA-256 `a3f6973c877d6d3fc7dd8712a415f990412e4dc527f07f480cc191985bc4ca66`。
- 生产证据 doctor 和 current 包已按最终指纹刷新，`missing_count=6`、`not_ready_count=6`，严格候选包因六项外部证据缺失按预期返回 1。当前本地代码、预览与总后台增量已收口，但生产最终完成仍等待真实外部证据。

## 阶段补充（2026-07-15 发布候选双人审批与 0069 收口）

- 发布候选曾是总后台中少数可直接执行的高风险写动作。本轮新增 `0069_saas_release_candidate_approval`，将 `release.candidate.gate` 接入统一审批策略、会签、SLA、提醒、失效、执行租约和不可变事件链。
- 默认策略要求 `platform.release.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 6 小时失效。强制审批开启后，直接候选调用返回 `428`；审批申请固化版本、权威指纹和证据元数据，执行时再次远端校验六项工件。
- 候选事务原子写入审批副作用操作 ID，可在“业务已提交、审批完成回写中断”场景只恢复审批状态而不重复候选。真实 MariaDB/Redis/私有 CA HTTPS smoke 已覆盖双人会签、执行、审计、恢复以及 0069 rollback/reapply。
- 完整短证据包已从头重建，`results.jsonl` 为 `14/14` 返回 0；六套验收、manifest 路由 `224/224`、运行时唯一路径 `587/587`、运行时路由条目 `729`、前端 dist API `209/209` 全部通过。
- 最终指纹为 `f1eb06656144ab104dd58853b9d171c988eda161d40d18c0ed347aefc36f0385`、文件数 859；预览镜像为 `sha256:18b757460cdba0ae2076a19c8847b2210ad5f1d7a22e2723d8d57dd1cd8a427c`，运行健康，数据库为 `69/0069`。
- 最终 Playwright 桌面/移动回归确认发布按钮在 `0/6` 证据时禁用，审批策略为 `2/120/30/6`，页面无整体横向溢出，控制台 0 error/0 warning。
- 剩余缺口没有变化：MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端、目标环境短稳/外部监控六项外部证据。严格生产门禁继续阻断，未启动 24 小时运行。

## 阶段补充（2026-07-15 审批策略变更双人会签与 0070 收口）

- 审计关闭了总后台审批策略仍可直接修改、并可潜在关闭审批治理自身的绕过路径。新增 `0070_saas_approval_policy_change_guard`，默认插入 `approval.policy.update` 策略并强制 `enabled=1`、`required_approvals>=2`，请求校验拒绝关闭治理或把自身降级。
- 直接策略接口在强制审批开启时返回 `428`，申请固定完整策略载荷；两名不同审批人通过后，策略更新、审批效果标记和平台操作日志在同一事务中提交。异常恢复只补审批状态，不重复应用策略副作用。
- 总后台治理行显示“强制启用”，开关不可编辑，最低审批人数为 2，保存改为“提交审批”。真实 Go + MariaDB + Redis 治理 smoke、审批主 smoke、迁移 down/up、release readiness、全量 Go 测试和独立交付门禁全部通过。
- 完整短证据包已重新从头采集，`results.jsonl` 14 条命令全部返回 0；六套验收、manifest `224/224` 和前端 dist API `209/209` 均通过。最终指纹为 `f5ff1082489ff19f917bfa487b333031cfbf6f0783f674775ae915f1b46c3ce5`，文件数 861。
- 保留数据卷的预览已升级到镜像 `sha256:0708d78ee0f92a9d7754d0c2d030e9b8613d3141bb9c87a4f9cfe8dd4b910280` 与迁移 `70/0070_saas_approval_policy_change_guard`，审批策略 8 条。日志内置指纹与证据包一致，`/readyz` 为 200。
- 升级前备份 `output/backups/mochat-preview-before-0070-approval-policy-guard-20260715-122339.sql` 为 `0600`、1016263 字节、SHA-256 `6f3f45bfabb096452cb2a35e0ad9c3a31ebf93cd7f4fdabd10a24d2eb306d8e7`。
- Playwright 在 `1440x1000` 与 `390x844` 验证强制启用、双人审批值和提交按钮。页面无整体横向溢出，移动宽表仅内部滚动，控制台 0 error/0 warning。
- 本地代码、迁移、预览和短证据已收口。生产最终完成仍等待六类外部证据：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控。`require_24h=false`，未启动 24 小时运行。

## 阶段补充（2026-07-15 备份策略变更双人会签与 0071 收口）

- 审计关闭了总后台备份策略仍可直接修改的高风险绕过路径。新增 `0071_saas_backup_policy_change_guard`，将 `backup.policy.update` 纳入统一审批，默认要求 `platform.backups.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。
- 直接备份策略写入在强制审批开启时返回 `428` 且不改数据库；申请固化规范化策略载荷，执行时重新校验。策略更新、审批效果操作 ID 和平台操作日志在同一 MySQL 事务提交，恢复租约不会重复应用副作用。
- 总后台备份策略保存动作已改为“提交审批”，审批中心新增“变更备份策略 / `backup.policy.update`”。真实 Go + MariaDB + Redis smoke 覆盖独立申请人、双人会签、第三方执行、效果标记和操作审计。
- 71 版迁移完整 apply/rollback/replay、全量 Go 测试、MySQL 5.7 静态兼容和全部六个短验收套件通过。最终证据包为 `14/14`，manifest `224/224`、运行时唯一路径 `587/587`、运行时路由条目 `729`、前端 dist API `209/209`；指纹为 `db9b881455a40dabe44470f036cdd33e4b6200ba83dc311fed58a511c9221b8c`，文件数 863。
- 保留数据卷的预览已升级到镜像 `sha256:f4424e4529b54002b52355d78428d24342bf57b581d75663f6237ec24d9a47f6` 和迁移 `71/0071_saas_backup_policy_change_guard`，审批策略 9 条，`/readyz` 为 200。升级前备份为 `output/backups/mochat-preview-before-0071-backup-policy-guard-20260715-143642.sql`，权限 `0600`、大小 1049933 字节、SHA-256 `2e37885c98839fa22da0f705a190f9839d43f2504ed6e4e2631be7b8763f4dca`。
- Playwright 在 `1440x1000` 和 `390x844` 验证备份策略值、提交审批按钮和审批策略。页面无整体横向溢出，移动按钮完整可见，宽表仅内部滚动，控制台 0 error/0 warning。
- 本地 `0071` 已收口。生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据；严格 current 包和 doctor 继续阻断。`require_24h=false`，未启动 24 小时运行。
- 后续本地高风险写链路优先处理备份清理，但必须先解决文件系统、数据库与 S3/Object Lock 跨资源幂等和恢复语义，再接入统一审批。

## 阶段补充（2026-07-15 备份保留清理 Saga 与 0072 收口）

- `0072_saas_backup_cleanup_saga` 已关闭备份保留清理的最后一条直接删除路径。申请阶段由服务端冻结候选并绑定审批，默认要求 `platform.backups.manage` 和 2 名不同复核人；直接接口返回 `428`，cron 与 CLI 只恢复已批准任务。
- 清理任务把 S3 副本、本地工件和数据库台账拆成可持久化步骤。远端失败不会继续删除本地或台账；成功步骤可幂等重放，租约恢复只处理剩余步骤。被冻结备份同时禁止校验、复制和恢复，避免删除与其他灾备动作并发。
- 真实 MariaDB/Redis/MinIO smoke 已覆盖冻结范围、双人会签、远端/本地/台账删除、失败保留和重试恢复。72 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose、审批、发布准备和灾备专项全部通过；首次完整证据重建发现合规脚本仍断言旧健康总数 29，已修正为 30 并增加 `backup_retention_cleanup=healthy` 断言后从头重跑。
- 最终 `docs/evidence/latest/results.jsonl` 于 `2026-07-15 16:55:13 CST` 至 `17:34:32 CST` 生成，`14/14` 返回 0。smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；MySQL 5.7 静态检查 143 个文件，ARM 实容器跳过不冒充 amd64 证据。
- 最终指纹为 `5db23ab6d2310791eb375baba9b3040df9c7b0089e572560e134cfb54de54146`、文件数 867；预览镜像为 `sha256:592bc88d5292c384581640a8d6459c534e8ca5ddfd79cc0d9c4214573b8dcb89`，数据库为 `72/0072`、审批策略 10 条、`/readyz=200`。升级前快照为 `output/backups/mochat-preview-before-0072-backup-cleanup-saga-20260715-162328.sql`，权限 `0600`、1062706 字节、SHA-256 `54eca644f01eb5474d11e593cbbb531c237af11f03c8ac1eac03edf5e3c2965c`。
- 最终桌面和 `390x844` 浏览器回归无页面级横向溢出，移动宽表仅内部滚动，控制台 0 error/0 warning。预览健康为 `32/33`，仅保留统计窗口内 7 次真实失败执行；最新审计锚点任务已成功验证 `42/42`，清理队列探针健康。
- 本地 0072 已收口，但生产最终完成仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项证据。doctor、preflight、严格文件检查和候选门禁均按预期阻断，源码指纹匹配通过；`require_24h=false`，未启动 24 小时运行。

## 阶段补充（2026-07-15 合规导出提前删除审批 Saga 与 0073 收口）

- `0073_saas_compliance_export_deletion_saga` 已关闭合规导出保留期内提前删除的直接入口。`compliance.export.delete` 强制启用并要求双人会签；申请时冻结导出和工件快照，直接删除返回 `428`，法律保留或未终结擦除引用存在时返回 `409`。
- 删除任务按本地加密工件、数据库记录两步持久化执行。失败会保留记录、错误和尝试次数，人工与维护任务只继续未完成步骤；健康中心会识别失败、积压和过期租约。页面同步显示删除步骤、审批、错误与重试操作。
- 真实 Go + MariaDB + Redis smoke 已覆盖冻结快照、法律保留、双人会签、工件失败、台账保留和重试成功。73 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 和六套本地短验收全部通过。
- 最终 `docs/evidence/latest` 为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `cf3b0366e8309d3a2f1efad3a6f6f7325b0e3d75a4e9e6cf2f93c4b9660e6bf7`，文件数 871。
- 保留数据卷的预览已升级到镜像 `sha256:7047142568b9d899522dbe26c835025a64dff52ced81daf83cb3124366f2138b`、迁移 `73/0073_saas_compliance_export_deletion_saga` 和 11 条审批策略。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0073-compliance-export-deletion-saga-20260715-220412.sql`，SHA-256 `6b85c75c55d57fa41271fbfac014a8a867f19d27fff8456b2b4a2ee9f01ba6df`。
- Playwright 在 `1440x1000` 与 `390x844` 验证审批策略和合规中心，页面无整体横向溢出，移动宽表只在内部滚动，控制台 0 error/0 warning。当前健康 `32/33`，三个合规探针 healthy，唯一 critical 为历史后台失败窗口。
- 本地独立 Go、SaaS 总后台与 0073 删除治理已收口。生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据；严格门禁按预期阻断，未启动 24 小时运行。

## 阶段补充（2026-07-16 法律保留解除双人审批与 0074 收口）

- `0074_saas_compliance_legal_hold_release_guard` 已关闭法律保留可由单人直接解除的治理缺口。`compliance.legal_hold.release` 强制启用并至少双人会签；直接解除返回 `428`，审批治理不能关闭或降级该策略。
- 申请阶段冻结保留单、租户、状态、原因、有效期、版本和解除原因；执行阶段锁行复核冻结快照。状态更新、操作审计和审批效果标记同事务提交，租约恢复不会重复解除。
- 合规中心活动保留单显示“提交解除审批”，审批中心新增“解除法律保留”。真实 Go + MariaDB + Redis smoke 已覆盖快照冻结、直接阻断、双人复核、执行与效果标记，74 版迁移 apply/rollback/replay 和全部短时本地门禁均通过。
- 最终 `docs/evidence/latest` 为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `293e0c1a28b67a0a488a575b16abbacf22c8eee6758c038312ae71b12ccd70d4`，文件数 874。MySQL 5.7 静态检查覆盖 147 个文件，ARM 实容器跳过不冒充 amd64 证据。
- 保留数据卷的预览已升级到镜像 `sha256:9556f703a22bb0f5e6d11580f5289b6a76eac4cbcee3ab14e1bdf75fa00329ed`、迁移 `74/0074_saas_compliance_legal_hold_release_guard` 和 12 条审批策略。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0074-legal-hold-release-20260715-230139.sql`，大小 1131065 字节、SHA-256 `0e5b94961f2f5ac9df2095f32feeb96d4751c6a66c375bea4302cb9b314966c5`。
- Playwright 在 `1440x1000` 与 `390x844` 验证总后台、策略行和活动法律保留操作。页面无整体横向溢出，移动宽表只在内部滚动，干净重载无失败请求或页面异常，控制台 0 error/0 warning。
- 当前平台健康 `32/33`，唯一 critical 为统计窗口保留的 7 次历史审计锚点失败；同一任务当前已连续 3 次成功，历史记录未被清理。生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项证据；严格门禁按预期阻断，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 合规生命周期策略变更双人审批与 0075 收口）

- `0075_saas_compliance_policy_change_guard` 已关闭合规生命周期策略可由单人直接修改的治理缺口。`compliance.policy.update` 强制启用并至少双人会签；直接写入返回 `428` 且策略不变，审批治理不能关闭或降级该策略。
- 申请阶段冻结完整策略载荷和当前版本；执行前重新校验版本，拒绝陈旧申请覆盖新配置。策略变更、操作审计和审批效果标记同事务提交，租约恢复不会重复应用策略副作用。总后台合规策略保存已改为“提交审批”。
- 真实 Go + MariaDB + Redis smoke 已覆盖直接阻断、载荷冻结、旧版本冲突、双人复核、执行、操作审计和效果标记。75 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 与全部短时本地门禁均通过。
- 最终 `docs/evidence/latest` 于 `2026-07-16 00:30:51 CST` 至 `01:09:00 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `c08216de5937ffc23d95f4092a76d6ed99f35fdb7f90bd9f7761f2cf378ee229`，文件数 877。MySQL 5.7 静态检查覆盖 149 个文件，ARM 实容器跳过不冒充 amd64 证据。
- 保留数据卷的预览已升级到镜像 `sha256:f022f983c899f4197d30019326cb55f114676221fa2a06cb6880b7c331e4d81e`、迁移 `75/0075_saas_compliance_policy_change_guard` 和 13 条审批策略。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0075-compliance-policy-guard-20260716-011013.sql`，大小 1145672 字节、SHA-256 `c646b6400b2ee8dfde369d2f733af3abf4c37c6687e5f6f05bd71ef1213f74eb`。
- Playwright 在 `1440x1000` 与 `390x844` 验证合规表单和审批策略。页面无整体横向溢出，移动宽表只在 368px `.tablewrap` 内滚动，策略行强制启用且值为 `2/120/30/12`，控制台 0 error/0 warning，未发现失败请求。
- 当前平台健康 `32/33`，唯一 critical 为统计窗口保留的 7 次历史后台失败；当前审计锚点任务最新 4 次成功，45 个锚点全部远端导出并验证。生产 doctor、preflight、严格目标审计和 current 候选包已按最终指纹刷新，仍缺六项真实外部证据；源码指纹匹配，`require_24h=false`，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 身份安全策略变更双人审批与 0076 收口）

- `0076_saas_identity_policy_change_guard` 已关闭身份安全策略可由单人直接修改的治理缺口。`identity.policy.update` 强制启用并至少双人会签；直接写入返回 `428` 且策略不变，审批治理不能关闭或降级该策略。
- 申请阶段冻结完整策略载荷和当前版本，执行前重新校验版本。策略变更、操作审计和审批效果标记同事务提交，恢复执行不会重复应用业务副作用；总后台身份策略保存已改为“提交审批”。
- 真实 Go + MariaDB + Redis smoke 已覆盖直接阻断、载荷冻结、陈旧版本冲突、双人复核、执行、操作审计和效果标记。76 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 与全部短时本地门禁均通过。
- 最终 `docs/evidence/latest` 于 `2026-07-16 01:54:25 CST` 至 `02:31:27 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `147d3b51f5937ce18760102b2f30635c8aebcf13e327745189ffe0338dda27eb`，文件数 880。MySQL 5.7 静态检查覆盖 151 个文件，ARM 实容器跳过且不冒充 amd64 证据。
- 保留数据卷的预览已升级到镜像 `sha256:0a478816f72cf1713b35a14b6b2d4330c84bf96e354cfd70d7d8ad5ce5b29585`、迁移 `76/0076_saas_identity_policy_change_guard` 和 14 条审批策略。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0076-identity-policy-guard-20260716-023234.sql`，大小 1157151 字节、SHA-256 `aaa05a07fb77268fc72aeb6aa61f345acafe1c5fca06de172cfd14a944d7ec9b`。
- Playwright 在 `1440x1000` 与 `390x844` 验证身份策略表单和审批策略。页面无整体横向溢出，移动宽表只在内部滚动，干净授权会话 69 个请求全部返回 200，控制台 0 error/0 warning。
- 当前平台健康 `32/33`，唯一 critical 为统计窗口保留的 7 次历史后台失败；审计锚点最新 5 次成功，46 个锚点全部远端导出并验证。生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项证据；严格门禁按预期阻断，`require_24h=false`，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 租户停用强制双人审批与 0077 收口）

- `0077_saas_tenant_disable_approval_guard` 已关闭租户停用只需单人复核且可被审批治理关闭或降级的缺口。`tenant.disable` 现强制启用、至少双人会签，直接停用返回 `428`；第一票后保持 pending，第二票后才允许执行。
- 审批申请固定租户停用载荷并要求 `platform.tenants.manage`。专项测试和真实 Go + MariaDB + Redis smoke 已覆盖直接阻断、两名不同复核人、执行后登录阻断、拒绝、取消和恢复语义；治理接口尝试关闭或把审批人数降到 2 以下均返回 `400`。
- 77 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 与全部短时本地门禁均通过。最终 `docs/evidence/latest` 于 `2026-07-16 03:13:55 CST` 至 `03:51:22 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `096bf16c8dbf24dc9a95d69ae35c46af81c559c67f48866746ca97ca943c090b`，文件数 883。MySQL 5.7 静态检查覆盖 153 个文件，ARM 实容器跳过且不冒充 amd64 证据。
- 保留数据卷的预览已升级到镜像 `sha256:250747044e82e7d3d2c96a76a176d8b2f2656c559ec9dda28ec09fba0e8f8480`、迁移 `77/0077_saas_tenant_disable_approval_guard` 和 14 条审批策略。`tenant.disable` 实测为强制启用、`2/240/60/24`、`v2`。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0077-tenant-disable-approval-guard-20260716-035225.sql`，大小 1170212 字节、SHA-256 `64206ea8929cf196161a7d04d4d31cdbeac34bd20e7d032bcf5ccc3f8c1d8e70`。
- Playwright 在 `1440x1000` 与 `390x844` 验收审批策略。页面无整体横向溢出，移动表格只在 368px `.tablewrap` 内滚动并可查看最右侧操作列；控制台 0 error/0 warning，业务请求全部为 200。截图为 `output/playwright/saas-0077-tenant-disable-policy-*.png`。
- 当前平台健康 `32/33`，唯一 critical 为统计窗口保留的 7 次历史后台失败；47 个审计锚点全部导出并验证。生产 doctor、preflight 与 current 候选包仍显示六项真实外部证据缺失，严格门禁按预期返回 1；`require_24h=false`，本轮未启动 24 小时运行。

## 阶段补充（2026-07-16 严重风险审批策略基线防降级与 0078 收口）

- `0078_saas_critical_approval_policy_guard` 已关闭 critical 审批策略可被关闭、可通过金额阈值绕过或可降为单人的平台级缺口。13 条严重风险策略统一为 `enabled=1`、`amount_threshold_cents=0`、`required_approvals>=2`；退款新建默认从 1 人修正为 2 人，普通结算关账策略仍可配置。
- 迁移会修复存量不安全值，down 不会把已修复的生产策略重新降级。默认定义、存储读取、管理写入和有效策略计算均使用同一治理不变式；API 向前端提供锁定和下限元数据，页面不再硬编码 critical 动作。
- 迁移 apply/rollback/replay、审批主 smoke、审批治理 smoke、定向和全量 Go 测试均通过。小额退款不再有直通路径，第一票后仍 pending，第二票后才可执行；代表性 critical 降级请求返回 `400`，普通策略修改保持可用。
- 最终 `docs/evidence/latest` 为 `14/14`，smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查 155 个文件，ARM 实容器跳过不冒充 amd64 证据。最终指纹 `47d64ab13b5e31c69b5b4b3268e8389a106258615bd709cbc05356d912140fad`，文件数 886。
- 保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:c3d4f11393e5e079225c56241cf7d87cc0d54b5888fa85f58b6719f0e342f073`、迁移 `78/0078`和 14 条审批策略。升级前快照 `output/backups/mochat-preview-before-0078-critical-approval-policy-guard-20260716-050823.sql` 为 `0600`、1180572 字节，SHA-256 `1ffd5a5197fa348f8462834fd19c11075ccf2fa4822a67cba7a133f11871977a`。
- Playwright 在 `1440x1000` 和 `390x844` 验收通过：13 条 critical 锁定，退款为零阈值且会签下限 2，结算关账仍可编辑且下限 1。移动页无整体溢出，宽表仅在 368px 容器内滚动；70 个业务请求全部为 200，控制台 0 error/0 warning。
- 当前健康为 `32/33`，非健康项仍是统计窗口内 7 次历史后台失败。启动审计任务新增 1 个锚点，现有 `48/48` 全部本地导出、远端对象锁导出并校验通过。
- 生产 doctor、preflight 和 current 证据包已刷新到最终指纹，仍为 `0/6`：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、真实多租户、生产前端、目标环境短稳/外部监控六类证据缺失。严格证据和 skip-local 候选门禁按预期返回 1；没有启动 24 小时运行，目标仍保持未完成。

## 阶段补充（2026-07-16 服务账号 API Key 吊销双人审批与 0079 收口）

- `0079_saas_service_account_key_revoke_guard` 已关闭服务账号 Key 可由单人直接吊销的治理缺口。`service_account.key.revoke` 现为第 14 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接接口返回 `428`，治理接口不能关闭或降级该策略。
- 申请阶段冻结服务账号、Key、预期版本、目标和申请原因，执行前重新校验当前状态与版本。Key 吊销、操作审计和审批效果标记同事务提交，恢复执行不会重复产生副作用；总后台吊销弹窗已改为“提交吊销审批”。
- 真实 Go + MariaDB + Redis smoke 已覆盖直接阻断、两名不同复核人、执行、旧 Key 认证失败和效果标记。79 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 与全部短时本地门禁均通过。
- 最终 `docs/evidence/latest` 于 `2026-07-16 09:16:54 CST` 至 `10:27:59 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `929529920e3b3eccfa5b6b4a5c704443a60959141acef1f84a4b4d4f677995`，文件数 889。MySQL 5.7 静态检查覆盖 157 个文件，ARM 实容器跳过且不冒充 amd64 证据。
- 保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:27b1dfe860c8865ba8fb1d2d6981efaee8385b7ae766f5a8192b11107f81f865`、迁移 `79/0079_saas_service_account_key_revoke_guard` 和 15 条审批策略。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0079-service-account-key-revoke-guard-20260716-103008.sql`，大小 1206245 字节、SHA-256 `bf719a02bc6f2eb5aabb015865980db579263e1680a37bd46dcd10bdc76e37a9`。
- Playwright 在 `1440x1000` 与 `390x844` 验收策略行、有效 Key 和吊销申请弹窗。页面无整体横向溢出，移动审批表只在 368px `.tablewrap` 内滚动，352px 弹窗与提交按钮完整可见；最终干净重载 69 个业务请求均成功，控制台 0 error/0 warning。验收工件位于 `output/playwright/saas-0079-service-account-key-revoke/`。
- 当前平台健康 `32/33`，唯一 critical 是统计窗口内保留的 6 次历史后台任务失败；50 个审计锚点全部完成本地证据、异地 Object Lock 导出和校验。生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项证据；严格门禁按预期阻断，`require_24h=false`，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 服务账号配置变更双人审批与 0080 收口）

- `0080_saas_service_account_update_guard` 已关闭服务账号状态、作用域、CIDR、额度、预警和有效期可由单人直接修改的治理缺口。`service_account.update` 为第 15 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接有效更新返回 `428`，治理接口不能关闭或降级。
- 申请阶段规范化并冻结完整配置与当前版本，陈旧版本返回 `409`；执行阶段重新校验冻结快照。账号更新、操作审计和审批效果标记同事务提交，恢复执行不会重复产生副作用。总后台编辑既有账号时显示“提交修改审批”，新建入口不受影响。
- 真实 Go + MariaDB + Redis smoke 已覆盖五次配置变更的直写阻断、两名不同复核人、原子执行、旧版本冲突、操作日志和审批效果标记。80 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 与全部短时本地门禁均通过。
- 最终 `docs/evidence/latest` 于 `2026-07-16 11:23:35 CST` 至 `12:39:14 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `a52c324c86a302f06d63095d477aa259585b51f650462013dc88b0342c4378fe`，文件数 891。MySQL 5.7 静态检查覆盖 159 个文件，ARM 实容器跳过且不冒充 amd64 生产证据。
- 保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:c39d8a30362dbd83248ae87d1f790e34a67e13aebe3a3c324d3a7e8dde06b9ea`、迁移 `80/0080_saas_service_account_update_guard` 和 16 条审批策略。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0080-service-account-update-guard-20260716-124042.sql`，大小 1224462 字节、SHA-256 `a04faaaa47503f8957f23881868ff038a089318ddd07f000473b98b9d06a7b5f`。
- Playwright 在 `1440x1000` 与 `390x844` 验证服务账号表单和审批策略。页面无整体横向溢出，移动审批表只在 368px `.tablewrap` 内滚动到 760px，编辑按钮文字完整；授权会话 146 个动态请求全部为 2xx，控制台 0 error/0 warning。验收工件位于 `output/playwright/saas-0080-service-account-update/`。
- 当前平台健康 `32/33`，唯一 critical 是统计窗口保留的 6 次历史后台任务失败，当前无活跃事故；51 个审计锚点全部完成本地证据、异地 Object Lock 导出和校验。生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项证据；严格门禁按预期阻断，`require_24h=false`，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 服务账号 API Key 轮换双人审批与 0081 收口）

- `0081_saas_service_account_key_rotate_guard` 已关闭服务账号 API Key 可由单人直接轮换、且明文生成时点早于审批的治理缺口。`service_account.key.rotate` 为第 16 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接轮换返回 `428`，治理接口不能关闭或降级。
- 申请阶段只冻结服务账号、当前版本、Key 名称、过期时间和旧 Key 宽限分钟数，不生成明文 Key。执行阶段重新校验版本后才生成 Key，并把新 Key 写入、旧 Key 退役、账号版本更新、操作审计和审批效果标记放在同一事务中；陈旧版本返回 `409`，恢复执行不会重复轮换。
- 明文 Key 只在首次审批执行响应中返回。持久化审批结果移除 `plainTextKey` 并记录 `plainTextKeyDelivered=true`，因此审批列表、事件、日志和租约恢复均无法再次读取明文。总后台轮换入口已改为“提交轮换审批”，审批执行后才显示一次性 Key。
- 真实 Go + MariaDB + Redis smoke 已覆盖直接阻断、两名不同复核人、执行后新旧 Key 状态、一次性响应、审批请求与持久化结果不含明文、效果标记和第二次轮换。81 版迁移 apply/rollback/replay、全量 Go 测试、独立 Compose 与全部短时本地门禁均通过。
- 最终 `docs/evidence/latest` 于 `2026-07-16 13:34:00 CST` 至 `15:32:10 CST` 完整补齐，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `d4a07979b8cc5b7e7814f5debbb5488ba97ef6fab94c8ae1ef6bb876fa35e81f`，文件数 893。MySQL 5.7 静态检查覆盖 161 个迁移文件，ARM 实容器跳过且不冒充 amd64 生产证据。
- 保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:b3fb68ff27bc75576f333a8fb642a8e677630f8ae1796166b6d8f61edc81127e`、迁移 `81/0081_saas_service_account_key_rotate_guard` 和 17 条审批策略，其中 16 条 critical。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0081-service-account-key-rotate-guard-20260716-153441.sql`，大小 1250259 字节、SHA-256 `4bc7c7e94c443589ad425085372d2f7ba26ff3e11403c1db2a095f1776c890bf`。
- Playwright 在 `1440x1000` 与 `390x844` 验证轮换策略和服务账号编辑区。页面无整体横向溢出，移动审批表只在 368px 容器内滚动到 760px，服务账号表只在 368px 容器内滚动到 1390px；按钮完整显示“提交轮换审批”，一次性 Key 区域未提前显示。最终干净重载 70 个动态请求全部为 200，控制台 0 error/0 warning；验收工件位于 `output/playwright/saas-0081-service-account-key-rotate/`。
- 浏览器验收没有提交审批或修改预览数据，轮换审批单数量保持 0，验收账号版本保持 `v9`。当前平台健康 `32/33`，唯一 critical 是统计窗口保留的 6 次历史后台任务失败；52 个审计锚点全部完成本地证据、异地 Object Lock 导出和校验。
- 本地 0081 已收口，但生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项证据。目标完成度审计继续阻断最终完成；`require_24h=false`，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 服务账号创建双人审批与 0082 收口）

- `0082_saas_service_account_create_guard` 已关闭服务账号及首个 API Key 可由单人直接创建的治理缺口。`service_account.create` 为第 17 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接创建在任何 Key 生成前返回 `428`，治理接口不能关闭或降级。
- 申请阶段规范化并冻结租户、账号配置和首 Key 元数据，不生成明文；执行阶段重新校验后才生成首 Key。账号、Key 摘要、操作审计和审批效果标记同事务提交，持久化结果只记录 `plainTextKeyDelivered=true`，恢复执行不会重复创建或重新暴露明文。
- 总后台新建入口显示“提交创建审批”，审批首次执行后才显示一次性首密钥。专项单元测试和真实 Go + MariaDB + Redis smoke 已覆盖直写阻断、双人复核、停用租户、重复代码、原子执行、效果标记、持久化脱敏和第二次读取不含明文。
- 最终 `docs/evidence/latest` 于 `2026-07-16 17:45:58 CST` 至 `19:24:10 CST` 完成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；指纹 `8d6dbe55ec10e5ec647a02892ef9306380304cd98ef7ff7fae6638b8387481a9`，文件数 895。收集器新增可恢复模式，本轮中断后只补跑缺失步骤，并在最终源码上重跑快速门禁，结果无重复 `slug`。
- 保留数据卷的预览已升级到镜像 `sha256:c26ad3e42388a19cf89948c863a5e17ed26e9448a24f3654727c999a83c64d40`、迁移 `82/0082_saas_service_account_create_guard` 和 18 条审批策略，其中 17 条 critical。`/readyz` 与 `/compat/status` 均为 200，运行时路由 706 条，build 指纹与 latest 一致。升级前快照为 `output/backups/mochat-preview-before-0082-service-account-create-guard-20260716-174445.sql`，权限 `0600`、大小 1263802 字节、SHA-256 `303e99f672f38477e2b8fe1bd0ff3d4e268bd426c8aeda7a172aa1999b1c5d19`。
- Playwright 在 `1440x1000` 与 `390x844` 验证创建策略和新建表单。页面无整体横向溢出，移动审批表和服务账号表只在内部容器滚动；“提交创建审批”按钮完整，一次性 Key 区域未提前显示。69 个动态请求全部为 200，控制台 0 error/0 warning；验收工件位于 `output/playwright/saas-0082-service-account-create/`。
- 当前平台健康 `33/33`，审计锚点 `54/54` 已完成本地证据、异地 Object Lock 导出和校验。生产 doctor、preflight 与 current 包仍为 `0/6`，严格门禁只因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据缺失而阻断；`require_24h=false`，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 用户 MFA 重置双人审批与 0083 收口）

- `0083_saas_identity_mfa_reset_guard` 已关闭平台管理员可单人重置用户 MFA 的身份安全缺口。`identity.mfa.reset` 为第 18 条 critical 策略，固定强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；直接重置返回 `428`，审批治理不能关闭或降级。
- 申请阶段冻结目标用户、租户、MFA 凭据版本和标准化原因；执行阶段重新校验冻结快照，并在同一事务内禁用 MFA、清除 TOTP 密钥与恢复码、失效未完成挑战、撤销全部活动会话、写身份安全事件和平台操作审计、标记审批效果。恢复执行复用审批效果幂等，不会重复产生副作用。
- 83 版迁移 apply/rollback/replay、身份安全和审批专项、全量 Go 测试、独立 Compose 与六套本地短验收均通过。`docs/evidence/latest` 于 `2026-07-16 20:13:04 CST` 至 `21:15:04 CST` 从头生成，14 条记录的 `returncode` 全部为 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。最终指纹 `7e7d5073a0f2adb08789999156caf01c6bfb2a99fc9220e525506ec9606f1699`，文件数 898。
- 保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:889a374354f7de2c7c6ca060ffa54983643ae13197ed7b07e186ae4c713c3737`、迁移 `83/0083_saas_identity_mfa_reset_guard` 和 19 条审批策略，其中 18 条 critical。启动日志内置 build 指纹与 latest 一致，`/readyz` 返回 200。
- 升级前快照为 `output/backups/mochat-preview-before-0083-identity-mfa-reset-guard-20260716-200020.sql`，权限 `0600`、大小 1285262 字节、SHA-256 `275f412825fa7362e67b6d230ca342f860230d5454ac450efcbf15b824d0eecd`。
- Playwright 在 `1440x1000` 与 `390x844` 验收审批策略和系统健康页。页面无整体横向溢出，移动审批表只在内部 760px 表格容器滚动，操作按钮完整可见；干净授权会话无失败请求，控制台 0 error/0 warning。截图位于 `output/playwright/saas0083-mfa-reset/`。
- 当前平台健康 `33/33`，critical、warning、未处理问题和活跃事故均为 0。目标完成度审计仍因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据而阻断；`require_24h=false`，本轮没有启动 24 小时运行。

## 阶段补充（2026-07-16 平台套餐定义双人审批与 0084 收口）

- `0084_saas_package_definition_guard` 已关闭平台套餐可被单人直接创建或修改的治理缺口。`package.upsert` 为第 19 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接写入返回 `428`，审批治理不能关闭或降级。
- 申请阶段冻结当前套餐、目标套餐、预期版本和租户影响分析；执行阶段锁行并重新校验版本。套餐创建从版本 1 开始，后续更新递增；陈旧申请或重复创建返回 `409`。套餐写入、操作审计和审批效果标记同事务提交，失败和恢复均不会重复产生副作用。
- 总后台套餐区新增描述、只读版本和“提交套餐审批”入口，审批中心可查看冻结影响。专项单元测试、真实 Go + MariaDB + Redis smoke、84 版迁移 apply/rollback/replay、全量 Go 测试、SaaS 总后台 smoke 和功能矩阵均通过；本轮没有重建完整 `14/14` 证据包。
- 保留数据卷的预览已升级到镜像 `sha256:db5e921d2159891098365fd43fd7ddebe461ddb168e8940516c4e5c5d36e21c0`、迁移 `84/0084_saas_package_definition_guard` 和 20 条审批策略，其中 19 条 critical。`/readyz` 返回 200，平台健康 `33/33`，启动日志中的 build 指纹为 `8c3229b66db779af3e1cb67968a433b2b474af2615e45d0a794f31c1b55a18d5`。
- 升级前 `0600` 数据库快照为 `/tmp/mochat-preview-before-0084-20260716-214911.sql`，大小 1313702 字节、SHA-256 `bf4ac595665478aa41590cd0709625030f29025416a569e61d255ef3f420f0cc`。MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 在 `1440x1000` 与 `390x844` 验收套餐编辑区和审批策略。页面无整体横向溢出，移动端输入项和提交按钮完整可见；策略实测为强制启用、`2/120/30/12`，干净授权会话控制台 0 error/0 warning。截图位于 `output/playwright/saas-0084-package-approval-desktop.png`、`saas-0084-package-approval-mobile-390x844.png` 和 `saas-0084-package-policy-row-desktop.png`。
- 本地 0084 已收口，但生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类证据；本轮按要求未运行 24 小时测试。

## 阶段补充（2026-07-16 租户套餐分配双人审批与 0085 收口）

- `0085_saas_tenant_package_assignment_guard` 已关闭租户套餐与订阅权益可被单人直接调整的治理缺口。`tenant.package.update` 为第 20 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接分配返回 `428`，审批治理不能关闭或降级。
- 申请阶段冻结租户、当前套餐、目标套餐、订阅状态和套餐分配版本；执行阶段锁定租户并重新校验版本。续费、开户或套餐同步造成的版本漂移会使旧申请冲突失效，套餐分配、订阅更新、操作审计和审批效果标记同事务提交，恢复执行不会重复产生副作用。
- 总后台租户列表新增“调整套餐”，分配区展示当前版本、变更说明和“提交分配审批”。专项单元测试与真实 Go + MariaDB + Redis smoke 已覆盖直写阻断、双人复核、冻结快照、版本漂移、原子执行、操作审计和恢复幂等。
- 85 版迁移 apply/rollback/replay、全量 Go 测试、SaaS 总后台、审批主流程、审批治理、独立包、发布准备和功能矩阵短门禁均通过。`scripts/test.sh` 实测 smoke `98/98`、manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。本轮未重建完整 `14/14` 证据包。
- 最终源码指纹为 `4ba1c159ca7f944dee5f2701a5479328cb0c3858c50302467a6bafe5338e17b8`，文件数 906。保留数据卷的预览已升级到镜像 `sha256:52d362570a54f44172337127f8d12763f90d2424048adb6d9a32fc968d3677f0`、迁移 `85/0085_saas_tenant_package_assignment_guard` 和 21 条审批策略，其中 20 条 critical；运行时路由 706 条，PHP fallback 关闭，平台健康 `33/33`，build 指纹与最终源码一致。
- 升级前快照为 `/tmp/mochat-preview-before-0085-20260716-224216.sql`，权限 `0600`、大小 1323664 字节、SHA-256 `4473f0fcdf2df4001e0975fafe4f95f2f1f3ab950e5f4349d5d2ddf1f83c9386`。MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 在 `1440x1000` 与 `390x844` 验收分配表单和审批策略。点击“调整套餐”后租户 ID 1、当前版本 1 正确回填且没有提交审批；页面无整体横向溢出，移动表单和按钮完整，宽表只在内部容器滚动。69 个动态请求全部返回 200，控制台 0 error/0 warning；验收工件位于 `output/playwright/saas-0085-tenant-package-assignment/`。
- 本地 0085 已收口，但生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类证据。按用户要求，本轮没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。

## 阶段补充（2026-07-16 平台租户开户双人审批与 0086 收口）

- `0086_saas_tenant_provision_approval_guard` 已关闭平台租户、初始管理员和订阅权益可由单人直接创建的治理缺口。`tenant.provision` 为第 21 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接开户、直接应用开户任务和批量应用开户任务均返回 `428`。
- 开户任务新增乐观版本，审批申请冻结任务版本、请求 SHA-256、套餐版本、管理员密码 bcrypt 哈希和预览。页面和 API 不返回明文密码或密码哈希，只显示凭据已经固化；陈旧任务、取消任务、请求漂移和套餐版本漂移均会拒绝执行。
- 审批执行把租户、管理员、角色、菜单、套餐快照、订阅、开户运行记录、任务状态、平台操作审计和审批效果标记放在同一 MySQL 事务中。专项真实数据库 smoke 已覆盖直写阻断、任务申请、双人复核、原子执行、新租户登录、取消任务冲突、直接申请幂等和所有持久化载荷无明文密码。
- 86 版迁移 apply/rollback/replay、全量 Go 测试、SaaS 审批主流程、审批治理、租户套餐分配审批、独立包、功能矩阵和短门禁均通过。`scripts/test.sh` 与验收覆盖为 `99/99`、manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。本轮没有重建完整 `docs/evidence/latest 14/14`。
- 最终指纹为 `ab8bed7be3de30847ce7ccfff5d51e52eee66b1a2368cd91dca1f540a607a897`、文件数 910；保留数据卷的预览已升级到镜像 `sha256:f907708826f3de90382c612c4f80469c2b9244ce384d5bcba4905286a07ee595`、迁移 `86/0086_saas_tenant_provision_approval_guard` 和 22 条审批策略，其中 21 条 critical。`/readyz=200`，平台健康 `33/33`，运行时路由 706 条，build 指纹与当前源码一致。
- 升级前快照 `/tmp/mochat-preview-before-0086-20260716-234326.sql` 为 `0600`、1332265 字节，SHA-256 `51ccb9cb742d99fac89e2b942c8b8895b3058c3400b14076a4441e9a9c4a5509`。升级只重建 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 在 `1440x900` 与 `390x844` 验收开户入口和审批策略。页面无整体横向溢出，移动端无按钮裁切，策略行与开户表单文字完整，干净授权会话控制台 0 error/0 warning；截图位于 `output/playwright/mochat-saas-admin-0086-*.png`。
- 生产 doctor 与 preflight 已刷新，标准证据和采集环境仍为 `0/6`。生产最终完成继续等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类证据；按用户要求，本轮没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。

## 阶段补充（2026-07-17 租户续费双人审批与 0087 收口）

- `0087_saas_tenant_renewal_approval_guard` 已关闭租户续费可由单人直接改变套餐、订阅和账务的治理缺口。`tenant.renewal` 为第 22 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接续费、直接应用续费任务和批量应用均返回 `428`，待审批任务仍可创建。
- 申请阶段冻结租户状态、套餐分配版本、套餐代码与版本、订阅存在状态及版本、任务 ID 与版本、请求 SHA-256 和审批执行引用。执行阶段重新锁定并校验冻结快照；任务取消、请求变化、套餐或订阅版本漂移会返回 `409`。
- 套餐分配、续费账单、订阅生命周期、任务状态、平台操作审计和审批效果标记在同一 MySQL 事务提交，租约恢复不会重复续费。日期输入新增 MySQL 5.7 `TIMESTAMP` 上界校验，越界返回 `400`，合法边界日期已通过专项测试。
- 87 版迁移 apply/rollback/replay、定向和全量 Go 测试、续费审批专项真实 MariaDB/Redis smoke、审批主流程、审批治理、系统健康、发布准备、独立包、验收覆盖和功能矩阵短门禁均通过。验收脚本覆盖 `100/100`，29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`；本轮未重建完整 `docs/evidence/latest 14/14`。
- 最终指纹为 `529e3a748d82b3e767f79826695905bea6ddefeda5fbdc18e779db9c3543dafb`、文件数 914。保留数据卷的预览已升级到镜像 `sha256:68be686d5f1dc21a1be77b534f326a1f502a78ad5a3820f9feea635d108fd742`、迁移 `87/0087_saas_tenant_renewal_approval_guard` 和 23 条审批策略，其中 22 条 critical。`/readyz=200`，平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与当前源码一致。
- 升级前快照 `/tmp/mochat-preview-before-0087-20260717-003852.sql` 为 `0600`、1345367 字节，SHA-256 `567266aec4052424edfd07cb88bcf5e158af70b681bf48ac40eb6e9e4d86018d`。升级只重建 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 在 `1440x900` 与 `390x844` 验收续费入口和审批策略。页面无整体横向溢出，移动端续费相关按钮均为 340px 且无裁切，策略行和三个审批入口文字完整，控制台 0 error/0 warning；浏览器验收没有提交审批或修改预览数据。截图为 `output/playwright/mochat-saas-admin-0087-desktop.png`、`mochat-saas-admin-0087-mobile.png` 和 `mochat-saas-admin-0087-policy.png`。
- 生产 doctor、preflight 与严格 current 报告已刷新为 `missing_count=6`、`not_ready_count=6`，发布准备为 `0/6`、`ready=false`。生产最终完成继续等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类证据；本轮未启动 `standalone_soak_24h.sh` 或任何 24 小时运行。

## 阶段补充（2026-07-17 租户订阅状态迁移双人审批与 0088 收口）

- `0088_saas_subscription_transition_approval_guard` 已关闭租户订阅状态可由单人直接迁移的治理缺口。`tenant.subscription.transition` 为第 23 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接迁移返回 `428`，治理接口不能关闭或降级。
- 申请阶段冻结租户状态、订阅 ID 与版本、目标状态、试用/周期/宽限结束时间、周期末取消标记、规范化原因和审批执行引用。执行阶段锁定租户和订阅并重新校验；平台租户、租户状态漂移或订阅版本漂移会返回 `409`。
- 订阅更新、订阅事件、平台操作审计和审批效果标记在同一 MySQL 事务提交。暂停后旧 Token 返回 `401`，新登录返回 `403`；恢复有效后登录恢复。自动对账继续使用服务端派生状态的直接修复链路，不接受客户端借此绕过人工迁移审批。
- 88 版迁移 apply/rollback/replay、定向与全量 Go 测试、订阅迁移审批专项真实 MariaDB/Redis smoke、审批主流程、审批治理、系统健康、发布准备、功能矩阵和短时全量门禁均通过。验收覆盖为 `101/101`，manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。本轮未重建完整 `docs/evidence/latest 14/14`。
- 最终指纹为 `1bce67b2dd8031457cdf59dc69b556256377760843bc500014d1bac74146e417`、文件数 918。保留数据卷的预览已升级到镜像 `sha256:257fc6a0967a602c8cfeaa817f67eb86bf93735c455882876a0bf360edf81d9c`、迁移 `88/0088_saas_subscription_transition_approval_guard` 和 24 条审批策略，其中 23 条 critical。迁移 checksum 为 `54871987fbe6003b5bf0b616a316c305449ce76b8f00ec1325f48a246a3a8b50`，`/readyz=200`，平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与当前源码一致。
- 升级前快照 `/tmp/mochat-preview-before-0088-20260717-012431.sql` 为 `0600`、1355318 字节，SHA-256 `6d924ce2d2ed6cb831920c8208aa25f486cd93b665197a0444daa640c9205de2`。升级只重建 App 并应用 0088，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 在 `1440x900` 与 `390x844` 验收总后台、订阅迁移入口和审批策略。两种视口均无页面级横向溢出，移动表单与“提交订阅审批”按钮完整；策略行实测为强制启用、审批人数 2、SLA 120、提醒 30、有效期 12。授权后控制台 0 error、网络无 4xx/5xx，未提交审批或修改预览数据。截图位于 `output/playwright/mochat-saas-admin-0088-*.png`。
- 本地独立 Go、SaaS 总后台与 0088 订阅迁移治理已收口。生产 doctor、preflight 和 `docs/evidence/production/current` 已按当前指纹刷新，分别显示 `missing_count=6`、`not_ready_count=6` 和 `evidence_ok=false`；严格 current 因 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据缺失而按预期返回 1。发布准备为 `0/6`、`metadataReady=false`、`ready=false`。按用户要求，本轮没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行。

## 阶段补充（2026-07-17 发票正式开具双人审批与 0089 收口）

- `0089_saas_invoice_issue_approval_guard` 已关闭蓝票和红票可由单人直接正式开具并改变支付订单金额台账的治理缺口。`billing.invoice.issue` 为第 24 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接推进到 `issued` 返回 `428`，治理接口不能关闭或降级。
- 申请阶段冻结单据身份、类型、状态、版本、票面快照，以及支付订单版本和退款、开票、红冲金额；执行阶段重新锁定单据和订单并校验冻结引用。单据或订单漂移会返回 `409`，不会提前写入审批效果或账务副作用。
- 单据状态、支付订单开票或红冲金额、平台操作审计和审批效果标记在同一 MySQL 事务提交，恢复执行不会重复开具。`processing`、`failed` 和 `canceled` 仍可由财务直接处理，避免审批门禁阻断失败纠错。
- 89 版迁移 apply/rollback/replay、定向和全量 Go 测试、发票开具审批专项真实 MariaDB/Redis smoke、独立包、功能矩阵和短时全量门禁均通过。验收覆盖为 `102/102`，manifest `224/224`；功能矩阵为 29 个模块、运行时唯一路径 `587/587`、运行时路由条目 `729`。MySQL 5.7 静态检查覆盖 177 个文件；本机 ARM 的真实 5.7 容器 smoke 按规则跳过，amd64 证据仍为生产缺口。
- 最终指纹为 `23fbbafd118f6c679d3402666f8a680ce13ddc13068f2b2b8c4f35b12cc02145`、文件数 922，0089 迁移 checksum 为 `39a732c3088f65ad93410d019b269a4b1b97a9ad0132f329a18a924b929b78bd`。保留数据卷的预览已升级到镜像 `sha256:7183026feb6d0ba806f441b0a7833a58b92a2b39fd99f6ecaf041caae920a1a5`、迁移 `89/0089_saas_invoice_issue_approval_guard` 和 25 条审批策略，其中 24 条 critical；`/readyz=200`，平台健康 `33/33`，运行时路由 706 条，PHP fallback 关闭，build 指纹与当前源码一致。
- 升级前快照 `/tmp/mochat-preview-before-0089-20260717-021602.sql` 为 `0600`、1367156 字节，SHA-256 `344c3f92ab5ce983bb8482349296aaee7202e5e04b9a751d8fb27cc693deadcb`。升级只重建 App 并应用 0089，MySQL、Redis 容器和原数据卷保持不变；升级前后的 3 个租户和 3 个用户均保留。
- Playwright 在 `1440x900` 与 `390x844` 验收总后台发票处理表单和审批策略。页面无整体横向溢出；移动端“提交开具审批”按钮宽 338px 且文字完整，85 个宽表只在内部容器滚动，策略行可完整查看动作、会签人数、SLA、有效期和提交按钮。69 个动态请求全部返回 200，控制台 0 error/0 warning；截图位于 `output/playwright/saas-0089-invoice-issue/`。
- 生产 doctor、preflight 和 `docs/evidence/production/current` 已按最终指纹刷新，分别为 `missing_count=6`、`not_ready_count=6` 和 `evidence_ok=false`；严格证据检查与 skip-local 候选门禁均按预期返回 1。剩余六项仍是 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控。本轮没有启动 `standalone_soak_24h.sh` 或任何 24 小时运行，整体目标继续保持“本地 0089 收口、待生产补证”。

## 阶段补充（2026-07-18 0089 全量证据与预览收口）

- 最终本地证据包已从头生成并通过恢复机制补齐，15 条结果全部为 0。六套验收 `core/workers/cron/saas/frontend/mysql57` 全绿，smoke `102/102`、manifest `224/224`、前端 dist API `209/209`；源码指纹为 `7c5ef5690ec238b7330aeefd85fe1df81d659835c6895d1eba054b97cd0085f1`，文件数 922，验收前后指纹一致。
- 证据链已补齐早期指纹检查点、可恢复执行和结束稳定性校验；服务健康等待默认值调整为可配置 360 秒。bootstrap、provisioning 和 16 个 cron 类 smoke 已使用隔离 Redis，前端总后台、侧栏和运营页真实浏览器回归全部通过，避免宿主机服务和端口冲突造成伪失败。
- 本地 compose 预览已用全新数据卷重建并完整通过，地址为 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，登录凭据仅保存在本地受控配置。当前迁移为 `89/0089_saas_invoice_issue_approval_guard`，审批策略 `25/24 critical`，App/MySQL/Redis 均健康，build 权威指纹与 latest 证据一致。
- 本地 Go 独立交付、SaaS 总后台和 0089 治理当前没有新增代码级阻断。严格生产门禁仍仅缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端和目标环境短稳/外部监控六项外部证据；候选源码匹配已通过。
- 下一执行批次应直接转向真实环境补证：先在 amd64 CI 生成 MySQL 5.7 artifact，再接入真实企微与微信开放平台，随后用两个以上真实租户执行隔离回归和生产浏览器回归，最后收集短稳健康与外部监控记录。`require_24h=false`，不启动 24 小时持续运行。

## 阶段补充（2026-07-18 收款订单创建双人审批与 0090 收口）

- `0090_saas_payment_order_create_approval_guard` 已关闭收款订单可由单人直接创建，以及结算权益受后续套餐定义变化影响的治理缺口。`payment.order.create` 为第 25 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接创建返回 `428`，治理接口不能关闭或降级。
- 申请阶段冻结有效租户、租户版本、套餐定义版本与完整额度、金额、服务周期、收银台失效时间和确定性订单号；执行阶段重新锁定租户和套餐并校验快照。任一漂移返回 `409`，不会提前写入订单或审批副作用。
- 订单、平台操作审计和审批效果标记在同一 MySQL 事务提交。订单持久化套餐版本和额度 JSON，支付回调结算使用订单冻结快照；套餐定义在订单创建后发生变化，不会改变该订单最终授予的权益。
- 90 版迁移 apply/rollback/replay、定向 Go 测试、收款订单创建审批专项真实 MariaDB/Redis smoke、短时全量 `scripts/test.sh` 和 MySQL 5.7 静态门禁均通过。验收覆盖为 `103/103`，manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。MySQL 5.7 静态检查覆盖 179 个文件；amd64 实容器仍未执行，不能作为生产证据。
- 当前源码与 build 权威指纹为 `f2c01af52235f4fe67f7bf7f6a4de6645da53dbcb4b7e0407399654f2f2da6f2`、文件数 926，0090 迁移 checksum 为 `edd6cd74fc3410ba6a757c6e5bba0ccabd4a3cf7894fa056444c98239ebb2299`。保留数据卷的预览已升级到镜像 `sha256:ed6d123e71b3971ca83309181935964ea2721f8ea4b91ef7bb340e07e327f076`、迁移 `90/0090_saas_payment_order_create_approval_guard` 和 `26/25 critical` 审批策略；App、MySQL、Redis 均健康。
- 升级前快照 `/tmp/mochat-go-preview-pre-0090-20260718.sql` 已收紧为 `0600`，大小 516003 字节、SHA-256 为 `56046922adf892dd52fd33ce1b9e9a495421e21399cba604ddc49552624c81d1`。升级只重建 App，MySQL、Redis 和原数据卷保持不变。
- Playwright 桌面与 `390x844` 移动回归确认“提交收款审批”、套餐版本和额度快照完整可见，无页面级重叠或裁切；截图为 `output/playwright/0090-payment-desktop.png` 和 `output/playwright/0090-payment-mobile.png`。预览关闭可选身份安全模块时相关接口仍返回既有 `501`，不影响本轮收款审批验收。
- 本地代码级新增缺口已收口，整体目标仍不能标记生产最终完成。剩余六项外部证据不变：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端和目标环境短稳/外部监控；本轮未重建完整 15 项本地证据包，`require_24h=false`，没有运行 24 小时任务。

## 阶段补充（2026-07-18 渠道结算关账双人审批与 0091 收口）

- `0091_saas_payment_settlement_close_guard` 已关闭渠道结算批次可由单个财务管理员直接关账的治理缺口。`payment.settlement.close` 从 high 升级为第 26 条 critical 策略，强制启用、零绕过阈值并至少双人会签；直接关账返回 `428`，治理接口不能关闭或降级。
- 申请阶段按精确 `batchNo` 冻结批次 ID、账期、渠道、币种、来源 SHA-256、状态、完整对账计数、金额台账、版本和备注，只接受无未解决差异的 `reconciled` 批次。执行阶段重新锁定并逐项比较完整快照，版本漂移和绕过版本字段的金额漂移都会返回 `409`。
- 批次关账、平台操作审计和审批效果标记在同一 MySQL 事务提交，失败不会留下部分状态，审批恢复执行保持幂等。重开仍作为财务纠错动作保留直接执行路径。
- 91 版迁移 apply/checksum/status/baseline/legacy/安全 rollback/replay、定向 Go 测试、结算关账专项真实 MariaDB/Redis smoke、短时全量 `scripts/test.sh` 和 MySQL 5.7 静态门禁均通过。验收覆盖 `104/104`、manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。MySQL 5.7 静态检查覆盖 181 个文件；amd64 实容器仍未执行，不能作为生产证据。
- 当前源码与 build 权威指纹为 `704f43a147148e65a75ac2497a92bb4bfcbf67fc43daf16c0e7ef97932ac7be3`、文件数 930，0091 迁移 checksum 为 `bc47db225b65be316a7bdff0e25f4be7faab648c042eb486651a9547720d2cab`。保留数据卷的预览已升级到镜像 `sha256:fecb35b03a7b106be3a6679a91f98501f63289d821e061eeccba7822cdd1ec40`、迁移 `91/0091_saas_payment_settlement_close_guard` 和 `26/26 critical`；App、MySQL、Redis 及迁移健康探针均健康。
- 升级前快照 `output/backups/mochat-preview-before-0091-settlement-close-guard-20260718-142250.sql` 权限为 `0600`，大小 520117 字节、SHA-256 `237442df6e0a4eaad1bc7c25b9e24941ca97ce76b07f63c631596fdb48b293a7`。升级只替换 App 并应用 0091，原数据卷保持不变。
- Playwright 桌面与 `390x844` 移动回归确认结算关账策略为强制启用、两人会签、`240/60/24`；移动页面无整体横向溢出，审批宽表在内部滚动后右侧按钮完整可见。预览关闭可选身份安全模块时四个身份接口仍返回既有 `501`，不影响结算关账验收。
- 本地代码级新增缺口已收口，整体仍不能标记生产最终完成。剩余六项外部证据不变：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端和目标环境短稳/外部监控；本轮未重建完整 15 项证据包，`require_24h=false`，没有运行 24 小时任务。

## 阶段补充（2026-07-18 渠道结算重开双人审批与 0092 收口）

- `0092_saas_payment_settlement_reopen_guard` 已关闭已关账批次可由单个财务管理员直接重开的治理缺口。`payment.settlement.reopen` 为第 27 条 critical 策略，强制启用、零绕过阈值、至少双人会签；直接重开返回 `428`，治理接口不能关闭或降级。
- 申请阶段冻结完整批次账务、导入/对账元数据和关账审计；执行阶段锁行复核，任何账务、版本或关账审计漂移均返回 `409`。状态恢复、关账字段清空、操作审计和审批效果标记同事务提交，重放保持幂等；关账/重开 schema v2 同时兼容 0091 旧关账审批。
- 92 版迁移、定向与全量 Go 测试、关账和重开专项真实 MariaDB/Redis smoke、独立包、短门禁和 MySQL 5.7 静态检查均通过。验收覆盖 `105/105`，功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。
- 本轮另修复新装 Compose 只初始化到 0089 的缺口：0090、0091、0092 已加入 initdb，通用门禁确保全部 `*.up.sql` 必须被挂载。当前指纹 `b9670008712828fbf602f94ff3a820947011b6fc34056c422912619ecb5b9e55`、933 个文件；预览镜像 `sha256:f941527ccd96fb9f52d684e41a7a982bf371551aa6ac6e20797751765fbdbdeb`，迁移 `92/0092_saas_payment_settlement_reopen_guard`，策略 `27/27 critical`。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0092-settlement-reopen-guard-20260718-153137.sql`，大小 526679 字节、SHA-256 `c87464a87851fb7c94b52083ab75bc4a69f0e45e460112d3881e385df5facf3b`。App、MySQL、Redis 均 healthy，桌面和移动 Playwright 验收无页面级溢出，浏览器未提交审批或修改预览数据。
- 当前本地代码级增量已收口。平台健康的 3 个 critical 是自动备份未启用、隔离恢复演练缺失和历史企微凭据待加密轮换；生产发布准备仍为 `0/6`，继续等待 MySQL 5.7 amd64、真实企微、真实微信开放平台、两个以上真实租户、生产前端和目标环境短稳/外部监控。`require_24h=false`，不启动 24 小时持续运行。

## 阶段补充（2026-07-18 渠道结算差异处理双人审批与 0093 收口）

- `0093_saas_payment_settlement_resolve_guard` 已关闭解决、忽略或重新打开结算差异可由单个财务管理员直接执行的治理缺口。`payment.settlement.resolve` 为第 28 条 critical 策略，强制启用、零绕过阈值、至少双人会签；直接处理返回 `428`，治理接口不能关闭或降级。
- 申请在一致性事务中冻结完整差异条目和所属批次账务；执行按批次、条目固定顺序锁行，并校验版本、金额、匹配结果和处理审计。无版本字段变化的账务漂移同样返回 `409`；条目、批次汇总、操作审计和审批效果同事务提交。
- 93 版迁移、定向与全量 Go 测试、0093 专项真实 MariaDB/Redis smoke、关账/重开回归、独立包、短门禁和 MySQL 5.7 静态检查均通过。验收覆盖 `106/106`，功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。
- 当前指纹 `46217ddd6564fc2916dbdcda3f5addf1ac5b5449409e5ffdf3eca0c9628203c0`、937 个文件；0093 checksum `522051435bdf11f1fba800d803b38209a0afd8ecb166e4d2b202c8e1f5c481fd`。预览镜像 `sha256:3574cb3e1338e2951738c78b8a32e392aa37f542e8d39a50672354575c8d89e6`，迁移 `93/0093_saas_payment_settlement_resolve_guard`，策略 `28/28 critical`；App、MySQL、Redis 均 healthy。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0093-settlement-resolve-guard-20260718-164721.sql`，大小 533593 字节、SHA-256 `7183d424c198e6db0281fcff7284bdadc0bb08f54a76deb7d05c641e01c774d4`。升级只替换 App 并应用 0093，原数据卷保持不变。
- Playwright 桌面和移动验收无页面级横向溢出；移动审批表仅在内部容器滚动，结算筛选完整堆叠。证据位于 `output/playwright/saas-0093-settlement-resolve/`，浏览器未提交审批或修改预览数据。
- 当前本地代码级增量已收口。平台健康的 3 个 critical 仍是自动备份未启用、隔离恢复演练缺失和历史企微凭据待加密轮换；生产发布准备仍为 `0/6`，继续等待 MySQL 5.7 amd64、真实企微、真实微信开放平台、两个以上真实租户、生产前端和目标环境短稳/外部监控。`require_24h=false`，不启动 24 小时持续运行。

## 阶段补充（2026-07-18 租户上线准备度与总后台处置闭环）

- 总后台新增 `GET /dashboard/saasAdmin/tenantReadiness` 和“租户上线准备度”工作区。接口按 `tenantId`、`keyword`、`state=all/ready/attention/blocked`、`limit` 聚合租户状态，要求 `platform.tenants.read`；未认证返回 `401`，非法状态返回 `400`，业务租户越权由 RBAC 返回 `403`。
- 准备度采用 8 项上线必需条件和 3 项交付完善条件。必需项覆盖启用租户、有效超级管理员、有效套餐、有效订阅、三类基础种子、企业微信企业绑定及加密凭据、身份安全策略；建议项覆盖安全通知策略、品牌档案和已完成路由/证书交付的主域名。任一必需项失败为 `blocked`，仅缺建议项为 `attention`，全部通过才是 `ready`。
- 页面提供状态汇总、租户/关键字筛选、11 项检查明细和权限感知的“定位”动作。定位只切换工作区、滚动到处置模块并预填租户筛选，不自动修改数据；订阅、企微凭据、身份安全、通知、品牌和域名配置完成后会刷新准备度。本增量为只读聚合，没有新增迁移，迁移账本保持 `93/0093_saas_payment_settlement_resolve_guard`。
- 全量 `go test ./...`、内嵌 JavaScript 编译检查、`bash -n scripts/smoke_saas_admin_dashboard.sh` 和真实 MariaDB/Redis 的总后台专项 smoke 均通过。最终源码指纹为 `72285fb2f416e42c7f7e62e67c5c7d54fe59075d5e989f1a7773b756dc77b21a`，文件数 940。
- 保留数据卷的预览最终只重建 App，镜像为 `sha256:7d742d4d3f87e9621aaf57919b0484d2e0518265b6731ed909bf9750a3dd9b1d`。App、MySQL、Redis 均 healthy，`/readyz` 为纯 Go standalone、PHP fallback 关闭，运行时路由由 706 增至 707；租户、用户、企业、套餐和 93 条迁移均保留，恢复临时库为 0。
- 当前预览租户“容器验收租户”为 `blocked`、完成度 `64%`、必需项 `7/8`：唯一核心阻塞是没有订阅；通知策略、品牌档案、主域名交付为 3 项建议缺口。真实 API 的 `blocked` 筛选命中 1，`ready` 命中 0。
- Playwright 在 `1440x900` 和 `390x844` 验收通过，控制台 `0 error/0 warning`，浏览器内准备度请求为 `200`。两端均无页面级横向溢出；移动表格只在 368px 容器内滚动到 980px，响应式滚动留白确保定位后的标题不被粘性导航遮挡。截图位于 `output/playwright/saas-tenant-readiness/desktop-1440x900.png` 和 `mobile-390x844.png`。
- 生产发布边界未被本地预览冒充完成：发布准备接口仍为 `passed=0/required=6`、`missing=6`、`metadataReady=false`、`ready=false`。仍需 MySQL 5.7 amd64、真实企微、真实微信开放平台、两个以上真实租户、生产前端和目标环境短稳/外部监控证据；按当前要求未执行 24 小时运行。

## 阶段补充（2026-07-18 租户准备度业务租户口径修正）

- 上一节将预览中的 1 号租户“容器验收租户”误当成客户租户，并据此记录为 `blocked/64%`。该租户实际是 `platformAdminTenantId=1` 的平台控制面租户，本来就不参与订阅对账、续费和生命周期处理；旧结论已作废。
- `GET /dashboard/saasAdmin/tenantReadiness` 现在明确声明 `scope=business_tenants` 和 `platformAdminTenantId`。全量 SQL 始终排除平台管理租户，显式传入平台租户 ID 返回 `400` 和“平台管理租户不参与上线准备度”，避免控制面再次污染客户上线指标。
- 当前预览真实口径为业务租户 `0`、可上线 `0`、待完善 `0`、已阻塞 `0`、平均完成度 `0%`。数据库仍为租户/用户/企业/套餐 `1/1/1/1`、订阅 `0`、迁移 `93/0093_saas_payment_settlement_resolve_guard`，恢复临时库为 0；没有新增迁移或修改业务数据。
- 全量 `go test ./...`、内嵌 JavaScript 编译检查、`bash -n scripts/smoke_saas_admin_dashboard.sh` 和真实 MariaDB/Redis 总后台专项 smoke 通过。最终源码/build 指纹为 `fb41266d0f0df9adaabdfc6d73362128eec56974eb99736941d99f51eabdeae1`，文件数 940；预览镜像为 `sha256:4e32ea6e87290ea6c4d1dc61fe6e86464ce764c079000459efc0930e25bcaf5a`，App、MySQL、Redis 均 healthy，纯 Go 运行时路由 707 条且 PHP fallback 关闭。
- 真实鉴权接口验证覆盖业务租户全量查询 `200`、平台租户定向查询 `400` 和未认证查询 `401`。Playwright 在 `1440x900` 与 `390x844` 验收“业务租户 0”和“当前筛选没有业务租户”，页面无整体横向溢出，移动宽表仅在 368px 容器内滚动，控制台 0 error/0 warning；截图位于 `output/playwright/saas-tenant-readiness-business-scope/`。
- 这次修正提高了指标可信度，但没有补齐真实 SaaS 多租户证据。下一步仍需接入至少两个真实业务租户，验证隔离、开户、订阅、企微和生产域名前端，并采集目标环境短稳/外部监控证据；本轮没有执行 24 小时运行，当前不能标记生产最终完成。

## 阶段补充（2026-07-19 业务租户口径与正式短证据包收口）

- 平台范围的总览、租户指标、运营日报与待办、经营趋势、续费预测、客户成功及其告警、通知、任务、账单、跟进日志和 CSV 查询已统一排除平台控制租户；显式租户范围仍可定向查看平台租户。专项真实 MariaDB/Redis smoke 会注入平台专属高风险数据并断言业务口径无泄漏。
- 正式本地证据包已从头生成：`15/15` 命令成功，六套短验收 `core/saas/workers/cron/frontend/mysql57` 全绿，smoke 接入覆盖 `106/106`、manifest 路由 `224/224`、运行时唯一路径 `588/588`、前端 dist API `209/209`，验收前后源码指纹一致。
- 本轮同时补齐 `0093` 发布准备 rollback/replay 断言，修正审批过期用例的非法前置状态，并将 local Compose 与前端登录 smoke 的 MySQL/Redis 端口参数化；相关专项脚本及正式 frontend 套件均通过。
- 最终源码/build 指纹为 `4126eaa2835b908374aba231f4b10f87f793e215c8b399bbc0fd1b63428c9a05`，文件数 941；保留数据卷的预览 App 已重建为镜像 `sha256:c7af82a5d49e1ce659a1c8772722bf9d6253bd039704deab580bf5d606bb74b2`。App、MySQL、Redis 均 healthy，`/healthz`、`/readyz` 为 200，运行时路由 707 条，迁移账本为 `93/0093_saas_payment_settlement_resolve_guard`。
- 目标完成度审计仍为未完成：生产证据 `0/6`，缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控。`require_24h=false`，不启动 24 小时持续运行。

## 阶段补充（2026-07-19 租户域名路由双人审批与 0094 收口）

- `0094_saas_tenant_domain_command_guard` 已关闭主域切换、域名启停、DNS 校验令牌轮换和域名删除可由单个管理员直接执行的治理缺口。`tenant.domain.command` 为第 29 条 critical 策略，强制启用、零绕过阈值、至少双人会签；五类直接写返回 `428`，策略不能关闭或降为单人。
- 申请冻结目标域名和租户全部未删除域名的路由快照；执行按固定顺序锁定并校验全部版本、状态和主域关系。任一漂移返回 `409`，域名状态、交付任务、操作审计与审批效果同事务提交；轮换令牌延迟到执行时生成，审批记录不保存明文。
- 94 版迁移、定向与全量 Go 测试、`scripts/test.sh`、域名审批专项真实数据库 smoke、原域名兼容 smoke、审批主流程/治理、独立包和 MySQL 5.7 静态检查均通过。验收接入为 `107/107`，manifest `224/224`，运行时唯一路径 `588/588`、静态运行时路由条目 730；MySQL 5.7 静态检查覆盖 187 个文件。
- 当前指纹为 `111f573666625f1382b5908c9addefa56ac68ea16584a272a23ad95c1c339aa7`、945 个文件；0094 checksum 为 `a7f23b5c1d0c67abc18021bd20d7be68bea10c7f80dbc1943c0b680c0cb1ba1c`。最终完整本地证据包为 `15/15`，`core/saas/workers/cron/frontend/mysql57` 六套验收全绿，证据指纹与当前源码一致；同步修正独立 Compose 备份 smoke 中滞后的 `migrationCount=93` 断言。
- 预览升级前 `0600` 快照为 `output/backups/mochat-preview-before-0094-tenant-domain-command-guard-20260719-012928.sql`，大小 639408 字节、SHA-256 `ef006bae1e93c2ed2478419309e25469f56e826025f1a5c2390a2a1bb4714d06`。App 已单独重建为 `sha256:a6a28779d3f8567716e0b5342ff037af8bb581e663a740b3baef6ae4140e0053`；MySQL、Redis 和原数据卷保留，运行环境零差异。
- 运行态为迁移 `94/0094_saas_tenant_domain_command_guard`、策略 `29/29 critical`、系统健康 `31/31`、纯 Go 路由 707 条和 PHP fallback 关闭。Playwright 桌面与 `390x844` 移动回归无页面级横向溢出，审批宽表只在内部滚动；截图位于 `output/playwright/saas0094/`，浏览器没有创建域名或提交审批。
- 当前本地代码级增量已收口。生产发布仍为 `0/6`、`ready=false`，继续等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端和目标环境短稳/外部监控。`require_24h=false`，不启动 24 小时持续运行。

## 阶段补充（2026-07-19 租户域名新增双人审批与 0095 收口）

- `0095_saas_tenant_domain_create_guard` 已关闭租户域名可由单个管理员直接新增的治理缺口。`tenant.domain.create` 为第 30 条 critical 策略，强制启用、零绕过阈值、至少双人会签；直接新增返回 `428`，策略不能关闭或降为单人。
- 申请只冻结标准化租户和域名，不生成 DNS 校验令牌。批准执行时锁定租户并复核启用状态、全局域名唯一性和每租户 10 个域名配额，才生成令牌；域名、初始路由/TLS 交付状态、操作审计和审批效果同事务提交，持久化结果不保存令牌明文。
- 95 版迁移、定向与全量 Go 测试、迁移 apply/checksum/down/replay/baseline/legacy、域名新增/变更审批、原域名兼容、发布准备、备份恢复、审批主流程/治理和独立 Compose 均通过。最终本地证据包 `15/15`，六套验收全绿，源码指纹稳定为 `432cc0978954d869c44b0cf6cc782ca0381758f2ab0297e32b3d990028a1f23d`，文件数 947。
- 预览镜像为 `sha256:e1ddf1a69a13f4a3db6544ca3409fc87cfd43332c2a69b4cfa232c544b9e2677`，迁移 `95/0095_saas_tenant_domain_create_guard`，策略 `30/30 critical`，系统健康 `31/31`；App、MySQL、Redis 均 healthy，运行环境零差异。升级前 `0600` 快照大小 658052 字节，SHA-256 为 `6a171c22d397ae46577f63454c79ab7d9b91aa5ac87d8ec57347758776656073`。
- Playwright 桌面与 `390x844` 移动回归确认“提交添加审批”和“新增租户域名绑定”策略，控制台 0 error/0 warning，动态请求均为 `200`，页面无整体横向溢出，浏览器未提交审批或修改数据；证据位于 `output/playwright/saas0095/`。
- 当前本地可执行代码缺口已收口，但生产最终态仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控六项外部证据。严格生产门禁因此保持失败，发布准备 `0/6`、`ready=false`；`require_24h=false`，不启动 24 小时运行。

## 阶段补充（2026-07-19 租户重新启用双人审批与 0096 收口）

- `0096_saas_tenant_enable_approval_guard` 已关闭单个平台管理员可重新启用已停用业务租户的治理缺口。`tenant.enable` 为第 31 条 critical 策略，强制启用、零绕过阈值、至少双人会签；直接启用返回 `428`，治理接口不能关闭或降为单人。
- 申请冻结租户和订阅快照；执行重新锁行并校验租户名称、状态及订阅存在性、ID、状态、版本，任一漂移返回 `409`。租户状态、订阅恢复、操作审计和审批效果同事务提交；现有 `tenant.disable` 新申请复用同一快照计划，执行器兼容历史停用审批载荷。
- 96 版迁移、定向与全量 Go 测试、`scripts/test.sh`、租户启停审批专项、审批治理、备份恢复和独立 Compose 均通过。最终本地证据包 `15/15`，六套短验收全绿，验收接入 `107/107`、manifest `224/224`、运行时唯一路径 `588/588`、运行时路由条目 730。
- 当前指纹为 `ad95a7ba63dc2ec688355083c5cedbb9e81790ada25a4136ee01c01c06ae0e25`、950 个文件；0096 checksum 为 `9a5c47a06f395ef320399ad17100c47fdcae31fc372dd66fd5f588587ca1ed0b`。预览镜像为 `sha256:45a45ca918529beeb0765ed5d91c1c79b0d913094d0cd4709dda61df201435c1`，迁移 `96/0096_saas_tenant_enable_approval_guard`，策略 `31/31 critical`；App、MySQL、Redis 均 healthy。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0096-tenant-enable-guard-20260719-061642.sql`，大小 680393 字节、SHA-256 `34f1c68d9483f51741d5955d85fb5cae5320e70ad3a4c2d2af49795f8e647e0a`。升级只应用 0096 并替换 App，原数据卷保持不变。
- Playwright 桌面和移动验收无页面级横向溢出；移动审批表仅在内部容器滚动，租户状态表单和策略行均正确显示启用审批。干净会话控制台 0 error/0 warning，70 个动态请求均为 `200`；临时 QA 数据已清理，证据位于 `output/playwright/saas0096/`。
- 本地 0096 增量已经收口。生产证据包严格门禁仍按预期失败，剩余六项外部证据不变：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控；`require_24h=false`，不执行 24 小时运行。
