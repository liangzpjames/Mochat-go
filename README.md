# MoChat Go 独立迁移版

这是方案一的 Go 独立化实现，目标是把 MoChat 从 PHP 版本迁移为可独立运行的 Go 项目。当前代码仍保留迁移期兼容工具，但主验收口径是 `MOCHAT_GO_STANDALONE=1`：运行时不依赖原 `mochat/` PHP/Vue 源码目录、不依赖 PHP upstream、不依赖外部 compat manifest。

## 快速验收

```bash
cd /Users/lv/Documents/企业微信/mochat-go
./scripts/standalone_stack_check.sh
```

## 当前预览验收状态（2026-07-19）

- SaaS 总后台预览地址：`http://127.0.0.1:18090/saas-admin/`；旧地址 `/dashboard/saasAdmin/page` 会自动跳转。
- 当前 App 镜像为 `sha256:45a45ca918529beeb0765ed5d91c1c79b0d913094d0cd4709dda61df201435c1`，源码与验收指纹为 `ad95a7ba63dc2ec688355083c5cedbb9e81790ada25a4136ee01c01c06ae0e25`，数据库迁移账本为 `96/0096_saas_tenant_enable_approval_guard`。
- 已完成加密备份、工件校验和隔离恢复演练；恢复数据库已自动清理，没有 `mochat_restore_%` 残留。
- 平台健康为 `31/31`，critical、warning、未处理问题和活跃事故均为 0。
- 通知、企业微信、微信开放平台和身份安全已分别使用独立密钥域并强制凭据加密；企业微信现有凭据已迁移为密文，旧明文和待轮换数量均为 0。
- SaaS 总后台已拆分为总览、租户套餐、运营、财务、通知、治理、安全和交付 8 个权限感知工作区，支持工作区内模块跳转、状态持久化、RBAC 隐藏和移动端当前标签自动定位。
- 平台范围的总览、经营、续费、客户成功、运营待办、日报和导出统一使用“业务租户”口径，排除平台控制租户；显式租户范围仍可查看平台租户，便于排障和治理。
- 非平台业务租户重新启用已纳入 `tenant.enable` critical 双人审批；申请冻结租户与订阅快照，执行时锁行复核，任一名称、状态或订阅版本漂移返回 `409`，租户恢复、订阅恢复、操作审计和审批效果同事务提交。
- 租户域名新增已纳入 `tenant.domain.create` critical 双人审批；申请阶段只冻结标准化租户与域名，不生成 DNS 校验令牌，批准执行时重新锁定并校验租户状态、域名唯一性和 10 个域名配额后才生成令牌并原子创建绑定、交付状态、审计和审批效果。
- 租户域名主域切换、启停、DNS 校验令牌轮换和删除已纳入 `tenant.domain.command` critical 双人审批；申请冻结租户完整路由快照，执行时重新锁定并校验后，原子提交域名状态、交付任务、操作审计和审批效果。
- 桌面端 `1440x1000` 与移动端 `390x844` 已完成真实浏览器回归；移动页面无整体横向溢出，审批宽表只在内部容器滚动。干净鉴权会话的 70 个动态请求均为 `200`，控制台 0 error/0 warning；截图见 `output/playwright/saas0096/`。
- 0096 最终本地证据包已完整重建，`docs/phases/phase-pre0-standalone/evidence/latest/results.jsonl` 为 `15/15` 通过；包含全量 Go 测试、`scripts/test.sh`、迁移 apply/checksum/down/replay/baseline/legacy、真实 MariaDB/Redis 专项 smoke、`core/saas/workers/cron/frontend/mysql57` 六套验收、路由与前端产物覆盖及源码稳定性门禁；验收接入覆盖为 `107/107`。
- 当前仍不是生产最终态：发布准备为 `0/6`、`ready=false`，还缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控六项证据。按当前约定，本轮没有启动 24 小时运行。

## 开源许可与再发行

本项目沿用上游 MoChat 的 GPL-3.0 许可。源码许可、修改说明、第三方声明和对应源码交付说明分别见 `LICENSE`、`NOTICE.md`、`MODIFICATIONS.md`、`THIRD_PARTY_NOTICES.md` 和 `SOURCE_OFFER.md`。对外分发二进制、容器镜像或客户端时，不得移除这些文件或限制用户依 GPL-3.0 获取对应源码的权利。

## 运行方式

完整本地独立栈验收使用上方快速验收命令。

### Docker Desktop 一键部署

在 Windows PowerShell 中从仓库根目录执行：

```powershell
.\scripts\deploy_docker_desktop.ps1
```

脚本会使用固定 Compose 项目名 `mochat-go-desktop`，自动检查 Docker Desktop、构建并替换上一次部署的容器、等待 MySQL/Redis/应用健康、执行数据库迁移、初始化管理员，并检查 Dashboard、SaaS Admin、Sidebar、Operation 四个前端入口。默认管理员账号为 `13800000000`，默认密码为 `MochatLocal@123`，可通过 `-AdminPhone` 和 `-AdminPassword` 覆盖。

默认部署只替换容器，保留 MySQL、Redis、上传文件、备份和审计锚点等数据卷。只有明确需要清空全部项目数据时才执行：

```powershell
.\scripts\deploy_docker_desktop.ps1 -ResetData
```

`-ResetData` 会永久删除 `mochat-go-desktop` 项目的数据卷。端口被其他项目占用时，可显式指定端口：

```powershell
.\scripts\deploy_docker_desktop.ps1 `
  -DashboardPort 28080 `
  -SidebarPort 28081 `
  -OperationPort 28082 `
  -MySQLPort 23316 `
  -RedisPort 36389
```

其他常用参数包括 `-ProjectName`（Compose 项目名）、`-SkipHttpCheck`（跳过前端 HTTP 检查）和 `-DryRun`（只打印将执行的命令）。默认访问地址为：

- Dashboard：`http://127.0.0.1:18080/`
- SaaS Admin：`http://127.0.0.1:18080/saas-admin/`
- Sidebar：`http://127.0.0.1:18081/`
- Operation：`http://127.0.0.1:18082/`

完整容器独立栈可直接启动 Go app + MySQL + Redis：

```bash
cd /Users/lv/Documents/企业微信/mochat-go
MOCHAT_SIMPLE_JWT_SECRET='请替换成生产密钥' \
docker compose -f deploy/standalone/docker-compose.yml --profile app up -d --build
```

首次启动后，在 Go app 容器内记录 schema baseline 并创建管理员账号：

```bash
MOCHAT_SIMPLE_JWT_SECRET='请替换成生产密钥' \
docker compose -f deploy/standalone/docker-compose.yml --profile app exec app \
  mochat-migrate -action baseline -project-root /app

MOCHAT_SIMPLE_JWT_SECRET='请替换成生产密钥' \
docker compose -f deploy/standalone/docker-compose.yml --profile app exec app \
  mochat-bootstrap -phone '13800000000' -password '请替换成强密码'
```

手动启动 Go standalone：

```bash
cd /Users/lv/Documents/企业微信/mochat-go
env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR=127.0.0.1:18080 \
  MOCHAT_MYSQL_DSN='mochat:mochat_pass@tcp(127.0.0.1:13306)/mochat?parseTime=true&loc=Local' \
  MOCHAT_REDIS_ADDR=127.0.0.1:26379 \
  MOCHAT_SIMPLE_JWT_SECRET='3S6ybWbSy&23fFeq8' \
  go run ./cmd/mochat-go
```

迁移期需要和 PHP 原接口对照时，才使用兼容模式并显式配置 PHP upstream：

```bash
cd /Users/lv/Documents/企业微信/mochat-go
env -u GOROOT \
  MOCHAT_GO_ADDR=127.0.0.1:18080 \
  MOCHAT_SOURCE_ROOT=../mochat \
  MOCHAT_COMPAT_MANIFEST=../docs/migration/compat_manifest.json \
  MOCHAT_PHP_UPSTREAM=http://127.0.0.1:9501 \
  go run ./cmd/mochat-go
```

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MOCHAT_TIMEZONE` | `Asia/Shanghai`（standalone Compose / Docker 镜像） | 应用与 MariaDB 共用的 IANA 时区；MySQL 连接还会按 DSN `loc` 自动设置会话 `time_zone`，避免凌晨日报、审计和通知窗口漏数 |
| `MOCHAT_GO_ADDR` | `:8080` | Go 网关监听地址 |
| `MOCHAT_GO_STANDALONE` | 空 | 设为 `1` 时进入独立 Go 运行模式：不要求 PHP upstream、原 MoChat 源码目录或外部 manifest；未迁移路由返回 501 |
| `MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES` | standalone 且同时配置 `MOCHAT_MYSQL_DSN`、`MOCHAT_SIMPLE_JWT_SECRET` 时默认为 `1`，其他情况默认为空 | 设为 `1` 时默认启用当前所有已迁移业务路由；设为 `0` 可关闭该默认值，单个 `MOCHAT_GO_MIGRATE_*` 仍可单独覆盖 |
| `MOCHAT_PHP_UPSTREAM` | 空 | PHP Hyperf fallback 地址；默认不启用，迁移期需要灰度或对照时显式设置 |
| `MOCHAT_API_BASE_URL` / `API_BASE_URL` | `MOCHAT_GO_ADDR` 派生的本机 URL；显式设置 `MOCHAT_PHP_UPSTREAM` 时默认跟随 upstream | `chatTool/config` 输出给前端的 API 可信域名 |
| `MOCHAT_SIDEBAR_BASE_URL` / `SIDEBAR_BASE_URL` | `MOCHAT_GO_ADDR` 派生的本机 URL；显式设置 `MOCHAT_PHP_UPSTREAM` 时默认跟随 upstream | `chatTool/config` 生成侧边栏页面 URL 的基础域名 |
| `MOCHAT_OPERATION_BASE_URL` / `OPERATION_BASE_URL` | `MOCHAT_GO_ADDR` 派生的本机 URL；显式设置 `MOCHAT_PHP_UPSTREAM` 时默认跟随 upstream | 裂变等 operation 页面授权跳转 URL 的基础域名 |
| `MOCHAT_FILE_STORAGE_ROOT` / `FILE_STORAGE_ROOT` | standalone 为 `./storage/upload/static`；兼容模式为 `../mochat/api-server/storage/upload/static` | `txtVerifyUpload`、通用上传和 `AsyncFileUpload` 异步文件队列写入文件的本地存储根目录；Go 服务会把该目录以只读方式托管到 `/static/*` |
| `MOCHAT_GO_AI_PROVIDER_BASE_URL` | `https://dashscope.aliyuncs.com/compatible-mode/v1` | AI 洞察调用的 OpenAI 兼容模型服务地址 |
| `MOCHAT_GO_AI_PROVIDER_KEY` | 空 | AI Provider API Key；部署时通过环境注入，不写入仓库 |
| `MOCHAT_GO_AI_PROVIDER_MODEL` | `qwen-plus` | AI 洞察使用的模型名 |
| `MOCHAT_GO_AI_PROVIDER_TIMEOUT_SECONDS` | `30` | 单次 AI 分析调用超时秒数 |
| `MOCHAT_GO_WECOM_ARCHIVE_CORP_ID` | 空 | 企微会话存档企业 CorpId；四件套齐全后适配层状态为 ready |
| `MOCHAT_GO_WECOM_ARCHIVE_SECRET` | 空 | 企微会话存档 Secret |
| `MOCHAT_GO_WECOM_ARCHIVE_PUBLIC_KEY` | 空 | 企微会话存档 RSA 公钥 |
| `MOCHAT_GO_WECOM_ARCHIVE_PRIVATE_KEY` | 空 | 企微会话存档 RSA 私钥 |
| `MOCHAT_DASHBOARD_DIST` | `./web/dashboard/dist` | Go 服务托管 dashboard 前端静态产物的目录；目录不存在时不接管前端路由 |
| `MOCHAT_SAAS_ADMIN_DIST` | `./web/apps/saas-admin/dist` | Go 服务在 `/saas-admin/` 托管获客前 SaaS 总后台 MVP 的静态产物目录 |
| `MOCHAT_GO_LOGIN_PREFILL_PHONE` | 空 | 仅在 localhost/回环地址的登录页预填账号；生产和客户域名不输出 |
| `MOCHAT_GO_LOGIN_PREFILL_PASSWORD` | 空 | 仅在 localhost/回环地址的登录页预填密码；不要在生产环境配置 |
| `MOCHAT_SIDEBAR_DIST` | `./web/sidebar/dist` | Go 服务托管 sidebar 前端静态产物的目录；目录不存在时不启动 sidebar 前端入口 |
| `MOCHAT_OPERATION_DIST` | `./web/operation/dist` | Go 服务托管 operation 前端静态产物的目录；目录不存在时不启动 operation 前端入口 |
| `MOCHAT_GO_ENABLE_FRONTEND_SERVERS` | 空 | 设为 `1` 时在 standalone 下自动派生 sidebar/operation 前端独立监听地址 |
| `MOCHAT_SIDEBAR_FRONTEND_ADDR` | 空；开启 `MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1` 后由 `MOCHAT_GO_ADDR` 端口 +1 派生 | sidebar 前端独立监听地址，用于避开 `/css`、`/js` 根路径资源冲突 |
| `MOCHAT_OPERATION_FRONTEND_ADDR` | 空；开启 `MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1` 后由 `MOCHAT_GO_ADDR` 端口 +2 派生 | operation 前端独立监听地址，用于避开 `/css`、`/js` 根路径资源冲突 |
| `MOCHAT_WECHAT_API_BASE_URL` | `https://api.weixin.qq.com` | 微信开放平台 API 地址；烟测可指向 fake server |
| `MOCHAT_WECHAT_OPEN_PLATFORM_APP_ID` / `WECHAT_OPEN_PLATFORM_APP_ID` | 空 | 微信第三方平台 component appid；公众号表 `appid` 为空时使用 |
| `MOCHAT_WECHAT_OPEN_PLATFORM_SECRET` / `WECHAT_OPEN_PLATFORM_SECRET` | 空 | 微信第三方平台 component secret；公众号表 `secret` 为空时使用 |
| `MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET` / `WECHAT_COMPONENT_VERIFY_TICKET` | 空；也可由 `authEventCallback` 写入 `mochat_go_wechat_component_tickets` 后动态读取 | 微信开放平台 component_verify_ticket，公众号预授权、授权回调、网页授权 code 换用户信息和开放平台测试消息 `QUERY_AUTH_CODE` 链路需要 |
| `MOCHAT_SOURCE_ROOT` | 兼容模式默认 `../mochat`；standalone 强制为空并忽略该环境变量 | MoChat PHP/Vue 源码目录，仅迁移期清单扫描和兼容状态使用 |
| `MOCHAT_COMPAT_MANIFEST` | 兼容模式默认 `../docs/migration/compat_manifest.json`；standalone 强制为空并忽略该环境变量 | 迁移清单 JSON；standalone 运行时使用编译进 Go 的内置 manifest |
| `MOCHAT_PROXY_TIMEOUT_SECONDS` | `30` | fallback 超时时间 |
| `MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER` | 空 | 设为 `1` 时启动企微回调 Redis 消费 worker，并同时启动 `contact-welcome` 欢迎语发送 worker |
| `MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER` | 空 | 设为 `1` 时启动企业授权后通讯录同步 Redis 消费 worker |
| `MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER` | 空 | 设为 `1` 时启动 PHP `AsyncFileUpload` 同口径 Redis 消费 worker，消费 `mochat-go:async-file-upload`，把 URL 或本地临时文件写入 `MOCHAT_FILE_STORAGE_ROOT` 下的目标相对路径；该 worker 只要求 Redis，不要求 MySQL/JWT；若同时配置 `MOCHAT_MYSQL_DSN`，结构化 payload 的 `tenantId` 或 `corpId` 会按当前租户写入后台执行账本并刷新 `async_executions` 用量和告警，落盘文件也会写入当前租户的 SaaS 存储账本并刷新 `storage_mb` |
| `MOCHAT_GO_ENABLE_MARK_TAGS_WORKER` | 空 | 设为 `1` 时启动 PHP `WorkContact\QueueService\Tag\MarkTags` 同口径 Redis 消费 worker，消费 `mochat-go:mark-tags`，写入客户标签 pivot、客户互动轨迹并调用企业微信 `externalcontact/mark_tag`；该 worker 要求 MySQL/Redis 和企业外部联系人 secret，不要求 JWT |
| `MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER` | 空 | 设为 `1` 时启动 PHP `WorkAgent\QueueService\MessageRemind` 同口径 Redis 消费 worker，消费 `mochat-go:message-remind`，按提醒应用发送企业微信 `message/send` 应用消息；该 worker 要求 MySQL/Redis，不要求 JWT |
| `MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER` | 空 | 设为 `1` 时启动 PHP `WorkRoom\QueueService\UpdateApply/UpdateCallback` 同口径 Redis 消费 worker，消费 `mochat-go:work-room-sync`，拉取企业微信客户群列表/详情并写入 `mc_work_room`、`mc_work_contact_room`；该 worker 要求 MySQL/Redis 和客户联系 secret，不要求 JWT |
| `MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS` | `300` | Go worker 中 processing 队列任务超过该秒数未 ack 时恢复重试 |
| `MOCHAT_GO_ENABLE_PULL_AGENT_CRON` | 空 | 设为 `1` 时启动企业微信应用同步 Go 定时任务，按 PHP `pullAgent` 口径刷新 `mc_work_agent` 应用详情 |
| `MOCHAT_GO_PULL_AGENT_CRON_INTERVAL_SECONDS` | `3600` | 企业微信应用同步 Go 定时任务的执行间隔秒数，对齐 PHP `pullAgent` 每小时基线 |
| `MOCHAT_GO_PULL_AGENT_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次企业微信应用同步 |
| `MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_CRON` | 空 | 设为 `1` 时启动成员统计拉取 Go 定时任务，按 PHP `employeeStatistic` 口径拉取前一天企业微信成员统计并写入 `mc_work_employee_statistic` |
| `MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS` | `86400` | 成员统计拉取 Go 定时任务的执行间隔秒数，对齐 PHP `employeeStatistic` 每日基线 |
| `MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次成员统计拉取 |
| `MOCHAT_GO_ENABLE_CHANNEL_CODE_CRON` | 空 | 设为 `1` 时启动渠道码联系我方式更新 Go 定时任务，按 PHP `channelCode` 口径刷新企业微信 contact_way 配置 |
| `MOCHAT_GO_CHANNEL_CODE_CRON_INTERVAL_SECONDS` | `30` | 渠道码联系我方式更新 Go 定时任务的执行间隔秒数，对齐 PHP `channelCode` 每 30 秒基线 |
| `MOCHAT_GO_CHANNEL_CODE_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次渠道码联系我方式更新 |
| `MOCHAT_GO_ENABLE_CONTACT_BATCH_SEND_CRON` | 空 | 设为 `1` 时启动客户群发定时发送 Go 任务，接管 PHP `ContactMessageBatchSendQueue/SendJob` 延时发送链路 |
| `MOCHAT_GO_CONTACT_BATCH_SEND_CRON_INTERVAL_SECONDS` | `60` | 客户群发定时发送 Go 任务的扫描间隔秒数 |
| `MOCHAT_GO_CONTACT_BATCH_SEND_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即扫描一次到期客户群发 |
| `MOCHAT_GO_ENABLE_ROOM_BATCH_SEND_CRON` | 空 | 设为 `1` 时启动客户群群发定时发送 Go 任务，接管 PHP `RoomMessageBatchSendQueue/SendJob` 延时发送链路 |
| `MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS` | `60` | 客户群群发定时发送 Go 任务的扫描间隔秒数 |
| `MOCHAT_GO_ROOM_BATCH_SEND_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即扫描一次到期客户群群发 |
| `MOCHAT_GO_ENABLE_CONTACT_SYNC_SEND_RESULT_CRON` | 空 | 设为 `1` 时启动客户群发结果同步 Go 定时任务，按 PHP `ContactSyncSendResultTask` 口径同步最近一周客户群发执行结果 |
| `MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS` | `3600` | 客户群发结果同步 Go 定时任务的执行间隔秒数，对齐 PHP `ContactSyncSendResultTask` 每小时基线 |
| `MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次客户群发结果同步 |
| `MOCHAT_GO_ENABLE_ROOM_SYNC_SEND_RESULT_CRON` | 空 | 设为 `1` 时启动客户群群发结果同步 Go 定时任务，按 PHP `RoomSyncSendResultTask` 口径同步最近一周客户群群发执行结果 |
| `MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS` | `3600` | 客户群群发结果同步 Go 定时任务的执行间隔秒数，对齐 PHP `RoomSyncSendResultTask` 每小时基线 |
| `MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次客户群群发结果同步 |
| `MOCHAT_GO_ENABLE_ROOM_TAG_PULL_CRON` | 空 | 设为 `1` 时启动标签建群结果同步 Go 定时任务，按 PHP `RoomTagPull` 口径同步企业群发任务和客户发送结果 |
| `MOCHAT_GO_ROOM_TAG_PULL_CRON_INTERVAL_SECONDS` | `300` | 标签建群结果同步 Go 定时任务的执行间隔秒数，对齐 PHP `RoomTagPull` 每 5 分钟基线 |
| `MOCHAT_GO_ROOM_TAG_PULL_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次标签建群结果同步 |
| `MOCHAT_GO_ENABLE_CORP_DATA_CRON` | 空 | 设为 `1` 时启动首页数据统计 Go 定时任务，刷新 `mc_corp_day_data` 和 `mc_work_update_time.type=6` |
| `MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS` | `600` | 首页数据统计 Go 定时任务的执行间隔秒数，对齐 PHP `corpData` 每 10 分钟基线 |
| `MOCHAT_GO_CORP_DATA_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次首页数据统计，便于 standalone smoke 和冷启动补数 |
| `MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_CRON` | 空 | 设为 `1` 时启动素材库临时 `media_id` Go 定时任务，刷新过期图片/音频/视频/文件素材 |
| `MOCHAT_GO_MEDIA_ID_UPDATE_CRON_INTERVAL_SECONDS` | `300` | 素材库临时 `media_id` Go 定时任务的执行间隔秒数，对齐 PHP `mediaIdUpdate` 每 5 分钟基线 |
| `MOCHAT_GO_MEDIA_ID_UPDATE_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次素材库临时 `media_id` 更新 |
| `MOCHAT_GO_ENABLE_TRANSFER_STATE_REFRESH_CRON` | 空 | 设为 `1` 时启动分配状态刷新 Go 定时任务，按 PHP `TransferStateRefresh` 口径查询客户转接状态并写回 `mc_work_transfer_log.state` |
| `MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_INTERVAL_SECONDS` | `300` | 分配状态刷新 Go 定时任务的执行间隔秒数，对齐 PHP `TransferStateRefresh` 每 5 分钟基线 |
| `MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次分配状态刷新 |
| `MOCHAT_GO_ENABLE_SOP_LOG_CRON` | 空 | 设为 `1` 时启动个人/群 SOP 提醒生成 Go 定时任务，按启用规则向 `mc_contact_sop_log` 和 `mc_room_sop_log` 幂等生成到期提醒，支持绝对时间、目标锚点相对延迟、群 SOP 客户入群锚点和 daily/weekly/monthly 周期规则 |
| `MOCHAT_GO_SOP_LOG_CRON_INTERVAL_SECONDS` | `300` | SOP 提醒生成 Go 定时任务的执行间隔秒数 |
| `MOCHAT_GO_SOP_LOG_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次 SOP 提醒生成 |
| `MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON` | 空 | 设为 `1` 时启动敏感词监控 Go 定时任务，扫描 `mc_work_message_1` 至 `mc_work_message_10` 的新增会话存档消息并写入 `mc_sensitive_words_monitor` |
| `MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_INTERVAL_SECONDS` | `60` | 敏感词监控 Go 定时任务的执行间隔秒数；游标按 `mc_work_message_id.type=21..30` 为 10 张分表独立维护 |
| `MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次敏感词扫描 |
| `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON` | 空 | 设为 `1` 时启动会话存档同步 Go 定时任务，从内部 SDK bridge 拉取已解密消息并写入 `mc_work_message_1` 至 `mc_work_message_10` |
| `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS` | `60` | 会话存档同步 Go 定时任务的执行间隔秒数；游标使用 `mc_work_message_id.type=40` 维护企业微信 `seq` |
| `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次会话存档同步 |
| `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT` | `100` | 每次从 SDK bridge 拉取的会话存档消息条数 |
| `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL` | 空 | 会话存档 SDK bridge 的 HTTP 地址；启用同步 cron 时必填，由 bridge 集成企业微信官方会话存档 SDK 拉取和解密 |
| `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN` | 空 | 调用会话存档 SDK bridge 时附带的 Bearer token |
| `MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON` | 空 | 设为 `1` 时启动 SaaS 上传存储账本校准 Go 定时任务，扫描 `MOCHAT_FILE_STORAGE_ROOT` 并刷新 `storage_mb` 用量 |
| `MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS` | `86400` | SaaS 上传存储账本校准 Go 定时任务的执行间隔秒数，默认每天一次 |
| `MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次上传存储账本校准 |
| `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD` | 空 | 设为 `1` 时启用 SaaS 告警 dashboard 页面和 API：`GET /dashboard/saasAlert/page`、`GET /dashboard/saasAlert/index`、`PUT/POST /dashboard/saasAlert/resolve`、`GET/PUT/POST /dashboard/saasAlert/setting`；API 要求 MySQL 和真实 dashboard JWT/Redis 黑名单检查 |
| `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD` | 空 | 设为 `1` 时启用 SaaS 总后台页面和 API：`GET /dashboard/saasAdmin/page`、`GET /dashboard/saasAdmin/overview`、`GET /dashboard/saasAdmin/tenantReadiness`、`GET /dashboard/saasAdmin/tenant`、`GET /dashboard/saasAdmin/tenantLifecycle`、`GET /dashboard/saasAdmin/usage`、`GET /dashboard/saasAdmin/risk`、`GET /dashboard/saasAdmin/businessMetrics`、`GET /dashboard/saasAdmin/businessTrends`、`GET /dashboard/saasAdmin/renewalForecast`、`POST/PUT /dashboard/saasAdmin/renewalForecastTasks`、`POST/PUT /dashboard/saasAdmin/renewalForecastAssign`、`POST/PUT /dashboard/saasAdmin/renewalForecastNotifications`、`GET /dashboard/saasAdmin/customerSuccess`、`GET /dashboard/saasAdmin/customerSuccessOwners`、`POST/PUT /dashboard/saasAdmin/customerSuccessAssign`、`POST/PUT /dashboard/saasAdmin/customerSuccessRenewalTasks`、`POST/PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications`、`POST/PUT /dashboard/saasAdmin/riskFollowUp`、`GET /dashboard/saasAdmin/riskFollowUps`、`GET /dashboard/saasAdmin/riskFollowUpOwners`、`POST/PUT /dashboard/saasAdmin/riskFollowUpBulkClose`、`GET /dashboard/saasAdmin/alerts`、`GET /dashboard/saasAdmin/notifications`、`GET /dashboard/saasAdmin/notificationHealth`、`GET /dashboard/saasAdmin/notificationSlo`、`GET /dashboard/saasAdmin/packages`、`GET /dashboard/saasAdmin/operations`、`GET /dashboard/saasAdmin/billingEvents`、`GET /dashboard/saasAdmin/billingReconciliation`、`GET /dashboard/saasAdmin/tasks`、`POST/PUT /dashboard/saasAdmin/taskCancel`、`POST/PUT /dashboard/saasAdmin/taskBulkCancel`、`POST/PUT /dashboard/saasAdmin/taskBulkReset`、`POST/PUT /dashboard/saasAdmin/taskReset`、`GET /dashboard/saasAdmin/dailyReport`、`GET /dashboard/saasAdmin/export`、`POST/PUT /dashboard/saasAdmin/alertResolve`、`POST/PUT /dashboard/saasAdmin/alertBulkResolve`、`POST/PUT /dashboard/saasAdmin/notificationRetry`、`POST/PUT /dashboard/saasAdmin/notificationBulkRetry`、`POST/PUT /dashboard/saasAdmin/package`、`POST/PUT /dashboard/saasAdmin/packageSync`、`POST/PUT /dashboard/saasAdmin/packageSyncTask`、`POST/PUT /dashboard/saasAdmin/packageSyncTaskApply`、`POST/PUT /dashboard/saasAdmin/packageSyncTaskBulkApply`、`POST/PUT /dashboard/saasAdmin/tenantStatus`、`POST/PUT /dashboard/saasAdmin/tenantRenewal`、`POST/PUT /dashboard/saasAdmin/tenantRenewalTask`、`POST/PUT /dashboard/saasAdmin/tenantRenewalTaskApply`、`POST/PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply`、`POST/PUT /dashboard/saasAdmin/tenantProvision`、`POST/PUT /dashboard/saasAdmin/tenantProvisionTask`、`POST/PUT /dashboard/saasAdmin/tenantProvisionTaskApply`、`POST/PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply`、`POST/PUT /dashboard/saasAdmin/tenantPackage`；API 要求 MySQL 和 dashboard 超级管理员身份，平台级套餐、平台开户、租户上线准备度、租户开停、租户生命周期审计、经营指标、经营趋势、通知健康度、通知 SLO、续费预测、续费预测任务、续费预测分派、续费预测提醒、客户成功队列、客户成功负责人工作台、客户成功续费提醒、风险跟进、套餐同步任务、续费任务、开户任务、续费账单、运营日报、CSV 导出和操作记录仅平台租户超级管理员可用；其余接口和运营说明见下方路由表；业务租户停用或启用套餐到期后登录会返回 `403`，已登录用户依赖 `UserByID` 的后台接口会失效，平台续费后恢复 |
| `MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED` | `1` | Go 通用默认开启高风险审批；独立部署样例在只有一名平台管理员的获客前阶段默认设为 `0`，增加第二位平台管理员后应改回 `1` |
| `MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON` | 空 | 设为 `1` 时启动审批 SLA 提醒任务；按审批策略快照扫描到期的待复核审批，并把提醒写入通知 outbox |
| `MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS` | `300` | 审批 SLA 提醒任务的执行间隔秒数，默认每 5 分钟一次 |
| `MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即扫描一次审批 SLA |
| `MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT` | `100` | 每轮最多扫描的待复核审批数；同一审批和提醒次数按通知 key 幂等去重 |
| `MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON` | 空 | 设为 `1` 时启动平台健康扫描，持久化检查快照、聚合活跃事故并为新增、重开或升级事故写入通知 outbox |
| `MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_INTERVAL_SECONDS` | `300` | 平台健康扫描的执行间隔秒数，默认每 5 分钟一次 |
| `MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次平台健康扫描 |
| `MOCHAT_GO_SAAS_SYSTEM_HEALTH_FAILURE_WINDOW_HOURS` | `24` | 失败执行、结算同步等检查的回看窗口，允许 1 至 720 小时 |
| `MOCHAT_GO_SAAS_SYSTEM_HEALTH_NOTIFICATION_STALE_MINUTES` | `15` | 待发通知被视为积压的滞留阈值，允许 1 至 10080 分钟 |
| `MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY` | 空 | 设为 `1` 时启用租户身份安全策略、账号锁定、IP/CIDR 门禁、TOTP/恢复码、可撤销持久会话、登录事件、安全事故、`/security/login` 和总后台身份安全中心；要求 MySQL 和身份加密密钥 |
| `MOCHAT_GO_SAAS_IDENTITY_ENFORCE_SESSIONS` | 空 | 设为 `1` 时每个 dashboard JWT 都必须匹配活动持久会话；从关闭切换为开启会使切换前签发的无会话 JWT 失效，生产应安排重新登录窗口 |
| `MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS` | 空 | 设为 `1` 时，身份登录和会话风险判定才会在 TCP 对端命中受信 CIDR 后解析 `X-Forwarded-For`/`X-Real-IP`；必须同时配置 `MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS` |
| `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS` | 空 | 设为 `1` 时，服务账号 IP/CIDR 白名单、用量账本和最近来源 IP 使用受信代理链中的客户端 IP；必须同时配置 `MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS` |
| `MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS` | 空 | 逗号或换行分隔的受信反向代理 IP/CIDR；仅 TCP 直连对端命中这些网段时才采信转发头，其他请求伪造的代理头会被忽略 |
| `MOCHAT_GO_SAAS_IDENTITY_ISSUER` | `MoChat Go` | TOTP provisioning URI 中显示的签发方名称 |
| `MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY` | 空 | 单一 32 字节 base64 或 64 位 hex AES-256-GCM 密钥；只用于兼容单密钥部署，生产优先使用密钥环 |
| `MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS` | 空 | JSON 密钥环，键为非敏感版本 ID、值为 32 字节密钥；当前 ID 必须存在，仍有 MFA 凭据引用历史 ID 时不得删除旧密钥 |
| `MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID` | `primary` | 新 MFA 凭据使用的非敏感密钥版本标识，不是密钥本身 |
| `MOCHAT_GO_ENABLE_SAAS_IDENTITY_CLEANUP_CRON` | 空 | 设为 `1` 时启动 `cron-saas-identity-cleanup`，按租户策略过期挑战、会话并清理超过保留期的记录 |
| `MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_INTERVAL_SECONDS` | `3600` | 身份安全清理间隔秒数 |
| `MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_RUN_ON_START` | 空 | 设为 `1` 时进程启动后立即执行一次身份安全清理 |
| `MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT` | `1000` | 每个清理阶段单次最多处理的记录数，范围 1 至 5000 |
| `MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON` | 空 | 设为 `1` 时启动 `cron-saas-service-account-usage-cleanup`，按合规策略清理过期 OpenAPI 日用量，活动法律保留租户自动跳过 |
| `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_INTERVAL_SECONDS` | `86400` | 服务账号用量清理间隔秒数，默认每天一次 |
| `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_RUN_ON_START` | 空 | 设为 `1` 时进程启动后立即执行一次服务账号用量清理 |
| `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT` | `10000` | 每次最多删除的用量聚合行数，范围 1 至 1000000 |
| `MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON` | 空 | 设为 `1` 时启动 `cron-saas-service-account-usage-alert`，评估服务账号当日成功用量和限流拒绝预警 |
| `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_INTERVAL_SECONDS` | `300` | 服务账号用量预警评估间隔秒数 |
| `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_RUN_ON_START` | 空 | 设为 `1` 时进程启动后立即执行一次服务账号用量预警评估 |
| `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT` | `100` | 每轮最多评估的服务账号数，范围 1 至 500；按最久未评估优先，避免固定批次饥饿 |
| `MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON` | 空 | 设为 `1` 时启动 `cron-saas-backup`；每次 tick 根据数据库策略判断是否到期，完成加密备份、本地与异地完整性校验和保留清理 |
| `MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS` | `300` | 备份策略扫描间隔；实际备份周期由持久化策略的 `interval_minutes` 决定 |
| `MOCHAT_GO_SAAS_BACKUP_CRON_RUN_ON_START` | 空 | 设为 `1` 时进程启动后立即执行一次到期检查 |
| `MOCHAT_GO_SAAS_BACKUP_ROOT` | `./storage/backups` | 备份工件目录；容器默认为 `/app/storage/backups` 并挂载持久卷 |
| `MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY` | 空 | 32 字节 base64 或 64 位 hex AES-256-GCM 密钥；启用备份 cron 时必填，只允许从密钥管理系统注入 |
| `MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS` | 空 | 可选 JSON 密钥环，键为非敏感版本 ID、值为 32 字节密钥；当前 `ENCRYPTION_KEY_ID` 必须存在，历史备份仍在保留期时不得删除旧密钥 |
| `MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID` | `primary` | 写入台账的非敏感密钥版本标识，不是密钥本身 |
| `MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY` | `mysqldump` | `mysqldump` 兼容客户端路径或命令名 |
| `MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY` | `mysql` | `mysql` 兼容恢复客户端路径或命令名 |
| `MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN` | 空 | 服务端预配置的隔离空库 DSN；不接受 API 请求覆盖，不得指向源库 |
| `MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN` | 空 | 自动恢复模式使用的实例管理 DSN；数据库名应留空，并只授予创建、写入和销毁临时恢复库所需权限 |
| `MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION` | 空 | 设为 `1` 时为每次演练创建独立临时库并在校验后销毁；与固定 `RESTORE_DSN` 互斥 |
| `MOCHAT_GO_SAAS_BACKUP_RESTORE_KEEP_ON_FAILURE` | 空 | 自动恢复失败时是否保留临时库；默认 `0` 仍执行销毁，生产排障时才短期开启 |
| `MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX` | `mochat_restore_` | 恢复目标库必须匹配的安全前缀，只允许字母、数字和下划线 |
| `MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT`、`MOCHAT_GO_SAAS_BACKUP_S3_BUCKET` | 空 | S3、MinIO、Ceph RGW 等兼容对象存储端点与存储桶；配置后备份会上传异地副本并做全流 SHA-256 校验 |
| `MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID`、`MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY` | 空 | 对象存储凭据，必须与 endpoint/bucket 同时配置并从密钥管理系统注入 |
| `MOCHAT_GO_SAAS_BACKUP_S3_REGION`、`MOCHAT_GO_SAAS_BACKUP_S3_SESSION_TOKEN` | 空 | 可选区域和临时会话令牌 |
| `MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL`、`MOCHAT_GO_SAAS_BACKUP_S3_PREFIX` | `1`、`mochat-go/backups` | 是否使用 TLS 以及对象键前缀；endpoint 显式带 `http://` 或 `https://` 时以 URL 为准 |
| `MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL` | 空 | 设为 `1` 时启用租户自助账单中心 `GET /dashboard/saasBilling/page` 及其账单汇总、收款、退款、开票资料、发票申请/取消 API；所有数据严格取自当前 dashboard JWT 的 `tenantId`，请求参数不能覆盖租户归属。平台发票受理、红冲、开具、失败和 CSV 导出继续由总后台接口处理 |
| `MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID` | `1` | SaaS 平台管理员租户 ID。只有该租户下的超级管理员可使用 `scope=platform` 查看全局总览，普通租户超级管理员只能查看本租户；平台管理员也可用 `scope=tenant&tenantId=<租户ID>` 下钻单租户 |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL` | 空 | 可选的全局兜底 webhook。租户未在 `mochat_go_saas_alert_settings` 配置通知时，可解析租户的 Redis worker 在 `async_executions` 超额并写入 `mochat_go_saas_alerts` 后会向该绝对 URL 发送 JSON webhook；通知失败只记日志，不阻断企微回调处理 |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS` | `1` | 默认要求全局及租户级告警 Webhook 使用 HTTPS；仅本地开发或受控内网兼容时可显式设为 `0` |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS` | 空 | 告警 Webhook 私网/回环地址的最小 CIDR 例外，支持逗号、分号或换行分隔；默认阻断私网、回环、链路本地、保留网段和云元数据地址。元数据地址不能通过例外放行 |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET` | 空 | 配置后，SaaS 告警 webhook 会增加 `X-Mochat-Go-Timestamp` 和 `X-Mochat-Go-Signature: v1=<hmac-sha256>`，签名内容为 `timestamp.body` |
| `MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY` | 空 | 兼容单密钥部署的 32 字节 base64 或 64 位 hex AES-256-GCM 主密钥；生产优先使用独立密钥环，不应与 JWT、备份或 MFA 密钥共用生命周期 |
| `MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS` | 空 | JSON 历史密钥环，键为非敏感版本 ID、值为 32 字节主密钥；活动 ID 和仍被通知策略引用的历史 ID 必须保留 |
| `MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID` | `primary` | 新保存或轮换的租户 Webhook URL 与 Secret 使用的活动密钥 ID，不是密钥本身 |
| `MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION` | 空 | 设为 `1` 时禁止无可用密钥的凭据写入；升级期可先读取旧明文并执行轮换，生产完成轮换后应开启 |
| `MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY` | 空 | 兼容单密钥部署的 32 字节 base64 或 64 位 hex AES-256-GCM 主密钥；生产应使用独立企微凭据密钥，不与 JWT、备份、MFA 或通知凭据密钥共用 |
| `MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS` | 空 | 企业微信企业与应用凭据的 JSON 历史密钥环；活动 ID 和仍被 `mc_corp`、`mc_work_agent` 密文引用的历史 ID 必须保留 |
| `MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID` | `primary` | 新保存或轮换的企微企业 Secret、回调 Token/AES Key、会话存档 Secret 与应用 Secret 使用的活动密钥 ID |
| `MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION` | 空 | 设为 `1` 时禁止无可用密钥的企微凭据写入；存量明文完成轮换后，生产环境应开启 |
| `MOCHAT_GO_WECOM_CREDENTIAL_ROTATION_LIMIT` | `100` | `rotate-wecom-credentials` 单批最多处理的企业和应用凭据数，允许 `1..1000` |
| `MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY` | 空 | 兼容单密钥部署的 32 字节 base64 或 64 位 hex AES-256-GCM 主密钥；生产应为微信开放平台 Ticket 与公众号授权凭据配置独立密钥 |
| `MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEYS` | 空 | 微信开放平台凭据的 JSON 历史密钥环；活动 ID 和仍被组件 Ticket、公众号授权密文引用的历史 ID 必须保留 |
| `MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID` | `primary` | 新保存或轮换的组件 Ticket、组件认证资料、公众号授权码与刷新 Token 使用的活动密钥 ID |
| `MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_REQUIRE_ENCRYPTION` | 空 | 设为 `1` 时禁止无可用密钥的微信开放平台凭据写入；存量明文完成轮换后，生产环境应开启 |
| `MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ROTATION_LIMIT` | `100` | `rotate-wechat-open-credentials` 单批最多处理的组件 Ticket 和公众号授权凭据数，允许 `1..1000` |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS` | `5` | SaaS 告警 webhook 请求超时时间，必须为正整数秒 |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS` | `1` | SaaS 告警 webhook 失败后的总尝试次数，必须为正整数；默认保持单次发送，设为 `2` 或更高时会对网络错误和非 2xx 响应做有限重试 |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS` | `250` | SaaS 告警 webhook 两次尝试之间的等待毫秒数，必须为非负整数；设为 `0` 时立即重试 |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE` | `SaaS额度告警：租户 {{.TenantID}} {{.Metric}}` | SaaS 告警 webhook 的 `title` 文本模板，使用 Go `text/template` 语法；可用字段包括 `TenantID`、`Metric`、`CurrentValue`、`LimitValue`、`AdditionalValue`、`Source`、`Message`、`AlertType`、`Severity`、`PeriodKey`、`OccurredAt`、`Context` |
| `MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE` | `{{.Message}}（当前 {{.CurrentValue}} / 上限 {{.LimitValue}}，来源 {{.Source}}）` | SaaS 告警 webhook 的 `body` 文本模板，启动时会校验模板语法和字段，避免告警触发时才失败 |
| `MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS` | `3` | SaaS 告警通知 outbox 的默认最大调度次数；租户配置表可覆盖该值，worker 发送 webhook 前会写入 `mochat_go_saas_alert_notifications`，失败后保留待维护命令重试 |
| `MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS` | `300` | SaaS 告警通知 outbox 失败后的默认下次重试间隔秒数；租户配置表可覆盖该值 |
| `MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON` | 空 | 设为 `1` 时启动 SaaS 告警通知 outbox 到期重发 Go 定时任务；要求 `MOCHAT_MYSQL_DSN`，webhook 可来自租户配置表或全局兜底环境变量 |
| `MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS` | `300` | SaaS 告警通知 outbox 到期重发定时任务的执行间隔秒数 |
| `MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即扫描并重发一次到期通知 |
| `MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT` | `100` | 每次 outbox 到期重发最多扫描的通知条数 |
| `MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON` | 空 | 设为 `1` 时启动运营待办认领到期提醒 Go 定时任务；要求 MySQL 和有效的 `MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID`，每轮只处理最新认领并把逾期、7 天内到期认领写入通知 outbox |
| `MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS` | `3600` | 运营待办认领到期提醒定时任务的执行间隔秒数，默认每小时一次 |
| `MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即扫描一次最新逾期和 7 天内到期认领 |
| `MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT` | `500` | 每个到期状态单次最多扫描的当前认领数；同一认领和到期状态按通知 key 幂等去重 |
| `MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON` | 空 | 设为 `1` 时启动通知健康认领自动复查任务；要求 MySQL 和有效的平台管理员租户 ID，只关闭当前窗口内健康且已有成功送达证据的认领 |
| `MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS` | `900` | 通知健康认领自动复查间隔秒数，默认每 15 分钟一次 |
| `MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即复查一次当前通知健康认领 |
| `MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS` | `24` | 自动复查使用的通知统计窗口，范围 1 至 720 小时 |
| `MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES` | `15` | 自动复查判定 pending 积压的分钟阈值，范围 1 至 10080 分钟 |
| `MOCHAT_GO_ENABLE_SAAS_SUBSCRIPTION_RECONCILE_CRON` | 空 | 设为 `1` 时启动订阅生命周期校准任务；要求 MySQL 和有效的平台管理员租户 ID，会根据试用、当前周期、宽限期和期末取消时间落库有效状态 |
| `MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_INTERVAL_SECONDS` | `300` | 订阅生命周期校准间隔秒数，默认每 5 分钟一次 |
| `MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即执行一次订阅校准 |
| `MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_LIMIT` | `500` | 每轮最多扫描的订阅数，范围 1 至 5000 |
| `MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK` | 空 | 设为 `1` 时启用公开支付回调 `POST /webhooks/saas/payment`；回调不使用 Dashboard JWT，而是强制 HMAC-SHA256 验签 |
| `MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET` | 空 | 支付回调签名密钥；启用回调时至少 32 个字符 |
| `MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS` | `300` | 支付回调时间戳容差，范围 30 至 3600 秒 |
| `MOCHAT_GO_ENABLE_SAAS_PAYMENT_DUNNING_CRON` | 空 | 设为 `1` 时启动支付失败催缴任务，扫描到期失败订单并写入通知 outbox |
| `MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_INTERVAL_SECONDS` | `300` | 支付催缴扫描间隔秒数 |
| `MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即扫描一次 |
| `MOCHAT_GO_SAAS_PAYMENT_DUNNING_LIMIT` | `100` | 每轮最多扫描的支付订单数，范围 1 至 5000 |
| `MOCHAT_GO_SAAS_PAYMENT_DUNNING_RETRY_DELAY_SECONDS` | `86400` | 同一失败订单下次可催缴时间，范围 60 至 2592000 秒 |
| `MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON` | 空 | 设为 `1` 时启动支付结算自动同步任务；要求 MySQL、平台管理员租户 ID、Bridge 地址和至少一个渠道 |
| `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_INTERVAL_SECONDS` | `900` | 支付结算自动同步间隔秒数，默认每 15 分钟一次 |
| `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_RUN_ON_START` | 空 | 设为 `1` 时 Go 进程启动后立即同步一次全部已配置渠道 |
| `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL` | 空 | 内部支付结算 Bridge 的绝对 HTTP(S) 地址；固定调用 `GET /v1/payment-settlements`，支付渠道密钥由 Bridge 保管 |
| `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TOKEN` | 空 | 调用支付结算 Bridge 时附带的 Bearer token |
| `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS` | 空 | 允许同步的支付渠道，英文逗号分隔，例如 `wechat_pay,alipay`；与 Bridge 地址一起配置 |
| `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_LIMIT` | `100` | 每页最多拉取的结算批次数，范围 1 至 5000；单次运行最多 20 页 |
| `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TIMEOUT_SECONDS` | `10` | 单次 Bridge HTTP 请求超时，范围 1 至 120 秒 |

支付结算 Bridge 的请求、响应、幂等和安全边界见 [`docs/reference/protocols/payment-settlement-bridge.md`](docs/reference/protocols/payment-settlement-bridge.md)。
| `MOCHAT_GO_MIGRATE_AUTH` | 空 | 设为 `1` 时接管 `POST /dashboard/user/auth` |
| `MOCHAT_GO_MIGRATE_LOGIN_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/user/loginShow` |
| `MOCHAT_GO_MIGRATE_LOGOUT` | 空 | 设为 `1` 时接管 `PUT /dashboard/user/logout` |
| `MOCHAT_GO_MIGRATE_PERMISSION_BY_USER` | 空 | 设为 `1` 时接管 `GET /dashboard/role/permissionByUser` |
| `MOCHAT_GO_MIGRATE_CORP_SELECT` | 空 | 设为 `1` 时接管 `GET /dashboard/corp/select` |
| `MOCHAT_GO_MIGRATE_CORP_BIND` | 空 | 设为 `1` 时接管 `POST /dashboard/corp/bind` |
| `MOCHAT_GO_MIGRATE_CORP_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/corp/index` |
| `MOCHAT_GO_MIGRATE_CORP_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/corp/show` |
| `MOCHAT_GO_MIGRATE_CORP_STORE` | 空 | 设为 `1` 时接管 `POST /dashboard/corp/store` |
| `MOCHAT_GO_MIGRATE_CORP_UPDATE` | 空 | 设为 `1` 时接管 `PUT /dashboard/corp/update` |
| `MOCHAT_GO_MIGRATE_WEWORK_CALLBACK` | 空 | 设为 `1` 时接管 `GET/POST /weWork/callback` 和 `GET/POST /dashboard/corp/weWorkCallback` |
| `MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG` | 空 | 设为 `1` 时接管 `GET /dashboard/chatTool/config` |
| `MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY` | 空 | 设为 `1` 时接管 `GET /WW_verify_*.txt` |
| `MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY_UPLOAD` | 空 | 设为 `1` 时接管 `POST /dashboard/agent/txtVerifyUpload` |
| `MOCHAT_GO_MIGRATE_AGENT_STORE` | 空 | 设为 `1` 时接管 `POST /dashboard/agent/store` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_AUTH` | 空 | 设为 `1` 时接管 `GET/POST /sidebar/agent/auth` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_OAUTH` | 空 | 设为 `1` 时接管 `GET /sidebar/agent/oauth` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_JSSDK_CONFIG` | 空 | 设为 `1` 时接管 `GET /sidebar/agent/jssdkConfig` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_WX_JS_SDK_CONFIG` | 空 | 设为 `1` 时接管 `GET /sidebar/wxJsSdk/config` |
| `MOCHAT_GO_MIGRATE_ROLE_SELECT` | 空 | 设为 `1` 时接管 `GET /dashboard/role/select` |
| `MOCHAT_GO_MIGRATE_ROLE_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/role/index` |
| `MOCHAT_GO_MIGRATE_ROLE_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/role/show` |
| `MOCHAT_GO_MIGRATE_ROLE_PERMISSION_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/role/permissionShow` |
| `MOCHAT_GO_MIGRATE_ROLE_SHOW_EMPLOYEE` | 空 | 设为 `1` 时接管 `GET /dashboard/role/showEmployee` |
| `MOCHAT_GO_MIGRATE_MENU_ICON_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/menu/iconIndex` |
| `MOCHAT_GO_MIGRATE_MENU_SELECT` | 空 | 设为 `1` 时接管 `GET /dashboard/menu/select` |
| `MOCHAT_GO_MIGRATE_MENU_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/menu/index` |
| `MOCHAT_GO_MIGRATE_MENU_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/menu/show` |
| `MOCHAT_GO_MIGRATE_CORP_DATA_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/corpData/index` |
| `MOCHAT_GO_MIGRATE_CORP_DATA_LINE_CHAT` | 空 | 设为 `1` 时接管 `GET /dashboard/corpData/lineChat` |
| `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workEmployee/index` |
| `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SEARCH_CONDITION` | 空 | 设为 `1` 时接管 `GET /dashboard/workEmployee/searchCondition` |
| `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC` | 空 | 设为 `1` 时接管 `PUT /dashboard/workEmployee/synEmployee` |
| `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workDepartment/index` |
| `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_DEPARTMENT_MEMBER_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workEmployeeDepartment/memberIndex` 和旧前端别名 `GET /dashboard/workDepartment/memberIndex` |
| `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SELECT_BY_PHONE` | 空 | 设为 `1` 时接管 `GET /dashboard/workDepartment/selectByPhone` |
| `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_PAGE_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workDepartment/pageIndex` |
| `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SHOW_EMPLOYEE` | 空 | 设为 `1` 时接管 `GET /dashboard/workDepartment/showEmployee` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workContactTagGroup/index` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DETAIL` | 空 | 设为 `1` 时接管 `GET /dashboard/workContactTagGroup/detail` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_GROUP_INDEX` | 空 | 设为 `1` 时接管 `GET /sidebar/workContactTagGroup/index` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workContactTag/index` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DETAIL` | 空 | 设为 `1` 时接管 `GET /dashboard/workContactTag/detail` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_LIST` | 空 | 设为 `1` 时接管 `GET /dashboard/workContactTag/contactTagList` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_ALL` | 空 | 设为 `1` 时接管 `GET /dashboard/workContactTag/allTag` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC` | 空 | 设为 `1` 时接管 `PUT /dashboard/workContactTag/synContactTag` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC` | 空 | 设为 `1` 时接管 `PUT /dashboard/workContact/synContact` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/workContact/show` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_ALL` | 空 | 设为 `1` 时接管 `GET /sidebar/workContactTag/allTag` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_DETAIL` | 空 | 设为 `1` 时接管 `GET /sidebar/workContact/detail` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_SHOW` | 空 | 设为 `1` 时接管 `GET /sidebar/workContact/show` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TRACK` | 空 | 设为 `1` 时接管 `GET /sidebar/workContact/track` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_UPDATE` | 空 | 设为 `1` 时接管 `PUT /sidebar/workContact/update` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_INDEX` | 空 | 设为 `1` 时接管 `GET /sidebar/contactProcessStatus/index` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE` | 空 | 设为 `1` 时接管 `PUT /sidebar/contactProcessStatus/update` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workContact/index` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_LOSS` | 空 | 设为 `1` 时接管 `GET /dashboard/workContact/lossContact` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_SOURCE` | 空 | 设为 `1` 时接管 `GET /dashboard/workContact/source` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_TRACK` | 空 | 设为 `1` 时接管 `GET /dashboard/workContact/track` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE` | 空 | 设为 `1` 时接管 `PUT /dashboard/workContact/update` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING` | 空 | 设为 `1` 时接管 `POST /dashboard/workContact/batchLabeling` |
| `MOCHAT_GO_MIGRATE_WORK_CONTACT_ROOM_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workContactRoom/index` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workRoom/index` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_ROOM_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workRoom/roomIndex` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS` | 空 | 设为 `1` 时接管 `GET /dashboard/workRoom/statistics` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workRoom/statisticsIndex` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC` | 空 | 设为 `1` 时接管 `PUT /dashboard/workRoom/syn` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE` | 空 | 设为 `1` 时接管 `PUT /dashboard/workRoom/batchUpdate` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workRoomAutoPull/index` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/workRoomAutoPull/show` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE` | 空 | 设为 `1` 时接管 `POST /dashboard/workRoomAutoPull/store` |
| `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE` | 空 | 设为 `1` 时接管 `PUT /dashboard/workRoomAutoPull/update` 和幂等兼容 `PUT /dashboard/workRoomAutoPull/move` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/roomTagPull/index` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/roomTagPull/show` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW_CONTACT` | 空 | 设为 `1` 时接管 `GET /dashboard/roomTagPull/showContact` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_ROOM_LIST` | 空 | 设为 `1` 时接管 `GET /dashboard/roomTagPull/roomList` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_CHOOSE_CONTACT` | 空 | 设为 `1` 时接管 `GET /dashboard/roomTagPull/chooseContact` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE` | 空 | 设为 `1` 时接管 `POST /dashboard/roomTagPull/store` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_FILTER_CONTACT` | 空 | 设为 `1` 时接管 `POST /dashboard/roomTagPull/filterContact` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_REMIND_SEND` | 空 | 设为 `1` 时接管 `GET /dashboard/roomTagPull/remindSend` |
| `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY` | 空 | 设为 `1` 时接管 `DELETE /dashboard/roomTagPull/destroy` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/contactMessageBatchSend/index` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/contactMessageBatchSend/show` 和预览别名 `GET /dashboard/contactMessageBatchSend/messageShow` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW_ROOM` | 空 | 设为 `1` 时接管 `GET /dashboard/contactMessageBatchSend/showRoom` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_EMPLOYEE_SEND_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/contactMessageBatchSend/employeeSendIndex` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_CONTACT_RECEIVE_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/contactMessageBatchSend/contactReceiveIndex` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE` | 空 | 设为 `1` 时接管 `POST /dashboard/contactMessageBatchSend/store` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND` | 空 | 设为 `1` 时接管 `POST /dashboard/contactMessageBatchSend/remind` |
| `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY` | 空 | 设为 `1` 时接管 `DELETE /dashboard/contactMessageBatchSend/destroy` |
| `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/roomMessageBatchSend/index` |
| `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/roomMessageBatchSend/show` |
| `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_OWNER_SEND_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/roomMessageBatchSend/roomOwnerSendIndex` |
| `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_RECEIVE_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/roomMessageBatchSend/roomReceiveIndex` |
| `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE` | 空 | 设为 `1` 时接管 `POST /dashboard/roomMessageBatchSend/store` |
| `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_REMIND` | 空 | 设为 `1` 时接管 `GET /dashboard/roomMessageBatchSend/remind` |
| `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY` | 空 | 设为 `1` 时接管 `DELETE /dashboard/roomMessageBatchSend/destroy` |
| `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/officialAccount/index` |
| `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET` | 空 | 设为 `1` 时接管 `GET /dashboard/officialAccount/set` |
| `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_GET_PRE_AUTH_URL` | 空 | 设为 `1` 时接管 `GET /dashboard/officialAccount/getPreAuthUrl` |
| `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_REDIRECT` | 空 | 设为 `1` 时接管 `GET/POST /dashboard/officialAccount/authRedirect/` |
| `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_EVENT_CALLBACK` | 空 | 设为 `1` 时接管 `GET/POST /dashboard/officialAccount/authEventCallback` |
| `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_MESSAGE_EVENT_CALLBACK` | 空 | 设为 `1` 时接管 `GET/POST /dashboard/{appId}/officialAccount/messageEventCallback` |
| `MOCHAT_GO_MIGRATE_LOAD` | 空 | 设为 `1` 时接管 `GET/POST /load/{params?}` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/workFission/index` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/workFission/show` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_INFO` | 空 | 设为 `1` 时接管 `GET /dashboard/workFission/info` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_STATISTICS` | 空 | 设为 `1` 时接管 `GET /dashboard/workFission/statistics` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_CHOOSE_CONTACT` | 空 | 设为 `1` 时接管 `GET /dashboard/workFission/chooseContact` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_STORE` | 空 | 设为 `1` 时接管 `POST /dashboard/workFission/store` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_UPDATE` | 空 | 设为 `1` 时接管 `PUT /dashboard/workFission/update` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE` | 空 | 设为 `1` 时接管 `POST /dashboard/workFission/invite` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DATA` | 空 | 设为 `1` 时接管 `GET /dashboard/workFission/inviteData` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DETAIL` | 空 | 设为 `1` 时接管 `GET /dashboard/workFission/inviteDetail` |
| `MOCHAT_GO_MIGRATE_WORK_FISSION_DESTROY` | 空 | 设为 `1` 时接管 `DELETE /dashboard/workFission/destroy` |
| `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_INVITE_FRIENDS` | 空 | 设为 `1` 时接管 `GET /operation/workFission/inviteFriends` |
| `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_POSTER` | 空 | 设为 `1` 时接管 `GET /operation/workFission/poster` |
| `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_TASK_DATA` | 空 | 设为 `1` 时接管 `GET /operation/workFission/taskData` |
| `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_RECEIVE` | 空 | 设为 `1` 时接管 `PUT /operation/workFission/receive` |
| `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_AUTH` | 空 | 设为 `1` 时接管 `GET/POST /operation/auth/workFission` |
| `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_OPEN_USER_INFO` | 空 | 设为 `1` 时接管 `GET /operation/openUserInfo/workFission` |
| `MOCHAT_GO_MIGRATE_CONTACT_FIELD_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/contactField/index` |
| `MOCHAT_GO_MIGRATE_CONTACT_FIELD_SHOW` | 空 | 设为 `1` 时接管 `GET /dashboard/contactField/show` |
| `MOCHAT_GO_MIGRATE_CONTACT_FIELD_PORTRAIT` | 空 | 设为 `1` 时接管 `GET /dashboard/contactField/portrait` |
| `MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_INDEX` | 空 | 设为 `1` 时接管 `GET /dashboard/contactFieldPivot/index` |
| `MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE` | 空 | 设为 `1` 时接管 `PUT /dashboard/contactFieldPivot/update` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_INDEX` | 空 | 设为 `1` 时接管 `GET /sidebar/contactFieldPivot/index` |
| `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_UPDATE` | 空 | 设为 `1` 时接管 `PUT /sidebar/contactFieldPivot/update` |
| `MOCHAT_MYSQL_DSN` | 空 | MoChat MySQL DSN，接管 `loginShow` 时必填 |
| `MOCHAT_SIMPLE_JWT_SECRET` / `SIMPLE_JWT_SECRET` | 空 | PHP `qbhy/simple-jwt` secret；正式接管 `loginShow` 时必填 |
| `MOCHAT_SIMPLE_JWT_PREFIX` / `SIMPLE_JWT_PREFIX` | `default` | PHP JWT Redis 黑名单前缀 |
| `MOCHAT_SIMPLE_JWT_TTL` / `SIMPLE_JWT_TTL` | `604800` | 登录签发 token 的 TTL，单位秒，保持 PHP 配置口径 |
| `MOCHAT_SIMPLE_JWT_REFRESH_TTL` / `SIMPLE_JWT_REFRESH_TTL` | `604800` | logout 写入 JWT 黑名单的 TTL，单位秒 |
| `MOCHAT_SIDEBAR_JWT_SECRET` / `SIDEBAR_JWT_SECRET` | `Br3LXhp&Ysha1zRDh` | PHP 侧边栏 `sidebar` guard 的 JWT secret |
| `MOCHAT_SIDEBAR_JWT_PREFIX` / `SIDEBAR_JWT_PREFIX` | `default` | 侧边栏 JWT Redis 黑名单前缀 |
| `MOCHAT_REDIS_ADDR` | `REDIS_HOST:REDIS_PORT` 或 `localhost:6379` | Redis 地址；正式接管 `loginShow` 时用于读取 `mc:user.{id}` 和 JWT 黑名单 |
| `MOCHAT_REDIS_PASSWORD` / `REDIS_AUTH` | 空 | Redis 密码 |
| `MOCHAT_REDIS_DB` / `REDIS_DB` | `0` | Redis DB |
| `MOCHAT_GO_SKIP_JWT_BLACKLIST` | 空 | 设为 `1` 时跳过 Redis JWT 黑名单检查，仅用于本地临时验证 |
| `MOCHAT_GO_DEV_AUTH_HEADER` | 空 | 设为 `1` 时临时使用 `X-Mochat-Go-User-ID` 解析用户，仅用于开发期 |

## 内置接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET/POST/HEAD` | `/` | 兼容 PHP 根路径，返回 `Hello MoChat ` |
| `GET` | `/favicon.ico` | 兼容空 favicon |
| `GET` | `/healthz` | Go 网关存活检查 |
| `GET` | `/readyz` | Go 网关、源码、manifest、PHP upstream 准备度检查 |
| `GET` | `/compat/status` | 迁移状态 |
| `GET` | `/compat/routes` | 当前完整路由、数据表、异步任务清单 |
| `POST` | `/dashboard/user/auth` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_AUTH=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go。启用身份安全后会执行账号锁定、IP/CIDR、MFA 和并发会话策略；需要二次认证时返回 `202` 和短时挑战，不直接签发 JWT |
| `GET/HEAD` | `/security/login` | 仅启用 `MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY=1` 时挂载，提供密码、TOTP/恢复码二段登录页；成功后以旧 Vue 前端兼容格式写入本地 token 并进入业务后台 |
| `POST` | `/dashboard/user/authMFA` | 仅启用身份安全时挂载，使用一次性 `challengeToken` 和 TOTP 或恢复码完成二次认证；挑战、动态码时间步和恢复码均禁止重放，成功后签发 JWT 并创建持久会话 |
| `GET/POST/PUT` | `/dashboard/user/securityMFA` | 仅启用身份安全时挂载并要求真实 dashboard JWT；查看本人安全状态和策略，或执行 `begin/verify` 自助绑定 TOTP。明文密钥与恢复码只在开始绑定的单次响应显示 |
| `GET` | `/dashboard/saasAlert/page` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1` 时走 Go，提供独立托管的 SaaS 告警管理页面，可用 dashboard JWT 查询和解决告警 |
| `GET` | `/dashboard/saasAlert/index` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1` 时走 Go，租户超管可按 `status`、`metric`、`alertType` 分页查看本租户 `mochat_go_saas_alerts` 告警 |
| `PUT/POST` | `/dashboard/saasAlert/resolve` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1` 时走 Go，租户超管可按 `metric` 和可选 `alertType` 解决本租户打开告警 |
| `GET/PUT/POST` | `/dashboard/saasAlert/setting` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1` 时走 Go，租户超管可查看和保存本租户 webhook 通知开关、URL、签名密钥、模板、HTTP/outbox 重试、事件订阅、最低严重级别、免打扰时段、IANA 时区和每小时限流策略 |
| `GET/HEAD` | `/dashboard/saasAdmin/page` | 可选启用。跳转到 `/saas-admin/` 获客前 SaaS 总后台 MVP；API 仍要求平台权限 |
| `GET` | `/dashboard/saasAdmin/overview` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，返回租户、套餐、用户、企业、告警、待通知、到期和用量汇总；平台租户超级管理员可用 `scope=platform` 查看业务租户全局，响应返回 `tenantPopulation=business_tenants`，并从汇总、列表和指标中排除平台控制租户；显式 `scope=tenant` 返回 `tenantPopulation=selected_tenant`，仍可查看指定租户；普通租户超级管理员只能看本租户。租户列表支持 `keyword`、`tenantStatus=1/2`、`packageCode`、`dueState=all/normal/expiring/expired/no_package` 筛选 |
| `GET` | `/dashboard/saasAdmin/tenantReadiness` | 可选启用。平台总后台按 `tenantId`、`keyword`、`state=all/ready/attention/blocked` 和 `limit` 聚合业务租户上线准备度，响应声明 `scope=business_tenants` 并始终排除平台管理租户；显式查询平台管理租户返回 `400`。8 项核心条件覆盖租户状态、超级管理员、套餐、订阅、基础配置、企业微信企业与加密凭据、身份安全策略，缺失时标记 `blocked`；通知策略、品牌档案和主域名交付为 3 项交付完善条件，缺失时标记 `attention`。接口要求 `platform.tenants.read`，业务租户访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/tenant` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，按 `tenantId` 返回单租户详情，聚合租户状态、当前套餐、用量指标和近期操作；平台租户超级管理员可查任意租户，普通租户超级管理员只能查自己 |
| `GET` | `/dashboard/saasAdmin/tenantLifecycle` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员按 `tenantId`、`limit`、`source=all/operation/billing/task/alert/notification`、`eventType`、`status` 和 `keyword` 查看单租户生命周期审计，聚合租户详情、操作记录、账单事件、运营任务、告警、通知 outbox 和统一时间线；`summary` 会同时返回筛选后 `timelineCount`、原始 `rawTimelineCount`、`returnedEventCount` 和 `filterActive`；`GET /dashboard/saasAdmin/export?type=tenantLifecycle` 可按相同筛选导出审计 CSV；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/usage` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，按 `tenantId` 返回单租户 26 项 SaaS 用量明细，包含周期、当前用量、额度、剩余、是否不限额、状态、打开告警数、更新来源和更新时间；平台租户超级管理员可查任意租户，普通租户超级管理员只能查自己 |
| `GET` | `/dashboard/saasAdmin/risk` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，按 overview 同口径筛选租户并聚合风险看板，支持 `highUsageRatio` 高用量阈值，返回每个租户的风险等级、风险分、原因、建议动作和 Top 风险用量指标；平台租户超级管理员可看全平台或任意租户，并回显每个租户最新风险跟进状态、负责人、下次跟进时间和备注；普通租户超级管理员只能看自己，且不暴露平台内部跟进备注 |
| `GET` | `/dashboard/saasAdmin/businessMetrics` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可基于全平台租户、风险看板和最近续费账单估算经营指标，返回估算 MRR/ARR/ARPA、风险收入、即将到期收入、已到期收入、未知价格租户数、近期账单金额和套餐收入分布；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=businessMetrics` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantLimit`、`expiringDays`、`highUsageRatio` 和 `billingLimit` 导出经营指标 CSV，包含筛选、汇总、套餐估算收入和最近账单行；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/businessTrends` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `months`、`billingLimit` 和 `taskLimit` 查看最近月份账单趋势、套餐流水分布和续费任务漏斗；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=businessTrends` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `months`、`billingLimit` 和 `taskLimit` 导出经营趋势 CSV，包含筛选、趋势汇总、月度流水、套餐流水和续费任务漏斗；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/operationQueue` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `source=all/customer_success/task_sla/billing_follow_up/notification/closed_notification/notification_health`、`priority=critical/high/medium/normal/all`、`owner`、`keyword`、`tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`warningHours`、`overdueHours`、`healthWindowHours` 和 `healthStaleMinutes` 查看统一运营待办队列，聚合客户成功、任务 SLA、账单跟进、失败/耗尽通知、已关闭通知和租户级通知健康异常；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/operationQueueOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可复用运营待办筛选按负责人聚合待办数、租户数、来源数、优先级分布、客户成功、任务 SLA、账单跟进、通知、关闭通知和通知健康异常分布，并返回 Top 租户和 Top 待办；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/operationQueueAssignments` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可查看运营待办队列级认领记录，支持 `source`、`owner`、`status`、`dueState=all/overdue/due_soon/future/no_date/closed`、`objectType/targetType`、`objectId/targetId`、`tenantId`、`keyword`、`currentOnly/current_only` 和 `limit` 筛选；`currentOnly=true` 时每个待办只返回最新认领状态；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/operationQueueAssignmentClose` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按当前认领 `operationId` 将认领标记为 `resolved` 或 `ignored`，清空下次跟进时间并写入 `saas.admin.operation_queue.assignment_close` 前后状态审计；过期操作 ID 返回 `409`，避免覆盖更新后的负责人；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/operationQueueAssignmentNotifications` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营待办认领记录同口径筛选，把 `dueState=overdue/due_soon` 的最新认领生成 `operation_queue_assignment_reminder` 通知 outbox，并写入 `saas.admin.operation_queue.assignment_notify` 操作日志；未传 `dueState` 默认处理逾期认领，历史负责人和已关闭认领不会被提醒，普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/operationQueueAssign` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营待办当前筛选批量分派待办；`customer_success` 和 `billing_follow_up` 分别落到风险跟进和账单跟进操作日志，`task_sla`、失败通知、关闭通知和 `notification_health` 租户级健康异常落到 `saas.admin.operation_queue.assign` 运营队列级认领日志，并在待办队列、负责人工作台和 CSV 中覆盖负责人；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=operationQueue` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营待办队列同口径导出 CSV，输出来源、优先级、租户、负责人、原因、下一步动作、状态、对象和停留时间；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=operationQueueOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营待办负责人工作台同口径导出 CSV，输出负责人、待办数、租户数、来源数、优先级分布、来源分布、未分配数、最大停留小时、Top 租户和 Top 待办；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=operationQueueAssignments` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营待办认领记录同口径导出 CSV，输出操作 ID、租户、来源、对象、负责人、状态、到期状态、下次跟进、备注、操作者和认领时间；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/renewalForecast` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket=all/expired/due_0_30/due_31_60/due_61_90/due_later`、`priced=all/priced/unknown`、`packageCode`、`owner` 和 `taskStatus=all/none/pending/blocked/failed/applied/canceled` 查看未来到期租户的续费收入预测、到期桶、未知价格租户、客户成功负责人、负责人工作台和续费任务跟进状态；响应会按负责人聚合预测续费金额、到期窗口、未知价格、任务状态和 Top 租户；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=renewalForecast` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可复用续费预测筛选 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus` 导出到期租户 CSV，包含套餐、到期桶、剩余天数、预测续费金额、估算 MRR、是否已定价、最近账单、续费任务数量、最新任务状态、负责人和最新风险跟进状态；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=renewalForecastOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可复用续费预测筛选导出负责人聚合 CSV，包含负责人、租户数、定价/未知价格租户数、预测续费金额、估算 MRR、到期窗口分布、任务状态分布、最早下次跟进和 Top 租户；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/renewalForecastTasks` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按续费预测当前筛选批量生成租户续费运营任务，筛选参数复用 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`；请求体支持 `packageCode`、固定 `expiresAt` 或 `months/renewMonths` 顺延、`amount/amountCents`、`currency`、`paidAt`、`paymentMethod`、`externalOrderNoPrefix`、`remark` 和 `forceCreate`；默认跳过已有待处理续费任务的租户，普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/renewalForecastAssign` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按续费预测当前筛选批量分派到期租户，筛选参数复用 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`；请求体支持 `owner/assignOwner`、`status=pending/contacted/renewal_pending`、`nextFollowUpAt` 和 `remark`，默认状态为 `renewal_pending`，服务端会逐租户写入 `tenant.risk.follow_up`；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/renewalForecastNotifications` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按续费预测当前筛选批量生成续费提醒通知 outbox，筛选参数复用 `tenantLimit`、`days`、`billingLimit`、`taskLimit`、`bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus`；请求体支持 `channel=webhook`、`reminderDays`、`maxAttempts`、`remark` 和 `forceCreate`，默认同一租户、同一到期日、同一通道只生成一条 `tenant_renewal_reminder` 通知，`forceCreate=true` 会重置为 `pending` 重新投递；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/customerSuccess` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可查看客户成功健康队列，按 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner`、`priority=critical/high/medium/normal/all` 汇总风险、跟进、账单、运营任务和失败通知，返回待处理租户的优先级、健康分、负责人、到期状态、原因和下一步动作；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/customerSuccessOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按客户成功队列同口径筛选并按负责人聚合，返回负责人租户数、优先级分布、逾期/7 天内/阻断分布、账单跟进、可处理运营任务、失败/耗尽通知、最高/平均健康分、最早下次跟进时间和 Top 租户；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/customerSuccessAssign` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按客户成功队列当前筛选批量分派负责人，筛选参数复用 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner`、`priority`，请求体支持 `owner/assignOwner`、`status=pending/contacted/renewal_pending`、`nextFollowUpAt` 和 `remark`；服务端会为命中的租户逐条写入风险跟进操作日志，普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/customerSuccessRenewalTasks` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按客户成功队列当前筛选批量生成租户续费运营任务，筛选参数复用 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner`、`priority`；请求体支持 `packageCode`、固定 `expiresAt` 或 `months/renewMonths` 顺延、`amount/amountCents`、`currency`、`paidAt`、`paymentMethod`、`externalOrderNoPrefix`、`remark` 和 `forceCreate`；默认跳过已有待处理续费任务的租户，普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/customerSuccessRenewalNotifications` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按客户成功队列当前筛选批量生成续费提醒通知 outbox，筛选参数复用 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner`、`priority`；请求体支持 `channel=webhook`、`reminderDays`、`maxAttempts`、`remark` 和 `forceCreate`，默认同一租户、同一到期日、同一通道只生成一条 `tenant_renewal_reminder` 通知，`forceCreate=true` 会重置为 `pending` 重新投递；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/riskFollowUp` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可给风险租户记录跟进状态、负责人、下次跟进时间和备注，服务端写入 `tenant.risk.follow_up` 操作日志；记录后平台风险看板和风险 CSV 会回显最新跟进状态；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/riskFollowUps` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantId`、`status`、`owner`、`dueState=all/overdue/due_soon/future/no_date/closed`、`keyword` 和 `limit` 查看每个租户最新风险跟进任务，返回逾期、7 天内、关闭和状态汇总；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/riskFollowUpOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按风险跟进任务同口径筛选并按负责人聚合，返回每个负责人打开任务数、状态分布、逾期/7 天内/未来/无日期/已关闭分布、最近记录时间和最早下次跟进时间；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/riskFollowUpBulkClose` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantId`、`filterStatus`、`owner`、`dueState`、`keyword` 和 `limit` 批量关闭风险跟进任务，`closeStatus` 仅支持 `resolved/ignored`，服务端会为每个命中的未关闭租户追加 `tenant.risk.follow_up` 操作日志；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/alerts` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，可按 `tenantId`、`status=open/resolved/all`、`metric`、`alertType` 分页查看 SaaS 告警；响应会返回不受分页影响的 `summary` 和 `returnedCount`，汇总告警总数、打开/已解决数量、严重级别、命中指标数和租户数；平台租户超级管理员可查任意租户，普通租户超级管理员只能查自己 |
| `POST/PUT` | `/dashboard/saasAdmin/alertResolve` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，按 `tenantId + metric + alertType + periodKey` 解决打开告警，并写入 `tenant.alert.resolve` 操作日志；平台租户超级管理员可处理任意租户，普通租户超级管理员只能处理自己 |
| `POST/PUT` | `/dashboard/saasAdmin/alertBulkResolve` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，可按 `tenantId`、`metric`、`alertType`、`periodKey` 和 `limit` 批量解决打开告警，并为每条告警写入 `tenant.alert.resolve` 操作日志；平台租户超级管理员可批量处理任意租户，普通租户超级管理员只能批量处理自己 |
| `GET` | `/dashboard/saasAdmin/notifications` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，可按 `tenantId`、`status=pending/failed/delivered/dead/closed/suppressed/all`、`channel`、`keyword` 查看 SaaS 告警通知 outbox；响应会返回不受 `limit` 影响的 `summary` 和 `returnedCount`，汇总通知总数、待发/失败/已送达/耗尽/已关闭/策略抑制/可重试数量、命中租户数和通道数；平台租户超级管理员可查任意租户，普通租户超级管理员只能查自己 |
| `GET` | `/dashboard/saasAdmin/notificationHealth` | 可选启用。平台租户超级管理员可按 `tenantId`、`state=all/healthy/warning/critical/no_data`、`channel=webhook`、`keyword`、`windowHours=1..720`、`staleMinutes` 和 `limit` 查看跨租户通知送达健康度；响应返回健康分级、送达成功率、待投递/延期/积压、失败/耗尽、关闭/抑制、平均/最大延迟和 Top 失败原因，普通租户访问返回 `403`；`GET /dashboard/saasAdmin/export?type=notificationHealth` 复用同一筛选导出 CSV |
| `GET` | `/dashboard/saasAdmin/notificationSlo` | 可选启用。平台租户超级管理员可按 `tenantId`、`channel=webhook`、`keyword`、`days=1..90`、`successRateTarget=0.5..1`、`latencySecondsTarget=1..86400`、`latencyRateTarget=0.5..1` 和 `limit` 查看按通知创建日期聚合的跨租户 SLO；响应补齐窗口内每个自然日，分别返回日趋势和租户排行的 `met/breached/no_data`、送达成功率、目标时延内送达率及状态分布。成功率分母包含 `delivered/failed/dead/closed`，`pending/suppressed` 单列；`GET /dashboard/saasAdmin/export?type=notificationSlo` 导出同口径 summary/day/tenant CSV，普通租户访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/subscriptions` | 可选启用。仅平台租户超级管理员可用，支持 `tenantId`、`status=all/trialing/active/grace/past_due/suspended/canceled`、`access=all/allowed/blocked`、`packageCode`、`keyword` 和 `limit` 筛选；返回存储状态、当前有效状态、访问权、试用/周期/宽限期时间、版本和待校准标记；`GET /dashboard/saasAdmin/export?type=subscriptions` 按同口径导出 CSV，普通租户返回 `403` |
| `GET` | `/dashboard/saasAdmin/subscriptionEvents` | 可选启用。仅平台租户超级管理员可按租户、目标状态、来源、关键字和上限查看订阅状态事件，回显前后状态、操作人、幂等键、原因和事件载荷，普通租户返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/subscriptionTransition` | 可选启用。仅平台租户超级管理员可用，支持六态受控迁移、`monthly/yearly/custom/lifetime` 计费周期、试用/周期/宽限期时间、期末取消、`expectedVersion` 乐观锁和 `idempotencyKey` 幂等重放；订阅事件与总后台操作日志在同一事务落库，平台管理租户不能转为阻断访问的状态 |
| `POST/PUT` | `/dashboard/saasAdmin/subscriptionReconcile` | 可选启用。仅平台租户超级管理员可用，支持按单租户或批量、`limit`、`dryRun` 预演校准订阅有效状态；每个租户独立处理，返回扫描、待校准、变更、跳过和失败数，平台管理租户自动排除 |
| `GET` | `/dashboard/saasAdmin/paymentOrders` | 可选启用。仅平台租户超级管理员可用，支持租户、状态、支付提供方、套餐和关键字筛选，返回收款汇总与订单明细；`GET /dashboard/saasAdmin/export?type=paymentOrders` 按同口径导出 CSV |
| `GET` | `/dashboard/saasAdmin/paymentWebhookEvents` | 可选启用。仅平台租户超级管理员可按订单、事件、支付提供方和处理结果查看签名回调账本 |
| `POST/PUT` | `/dashboard/saasAdmin/paymentOrder` | 可选启用。仅平台租户超级管理员可创建待支付订单，支持套餐、金额、币种、计费周期、服务到期时间和 `idempotencyKey` 幂等重放 |
| `POST/PUT` | `/dashboard/saasAdmin/paymentOrderCancel` | 可选启用。仅平台租户超级管理员可取消未支付订单，要求 `expectedVersion` 乐观锁，已支付订单不会被迟到取消改写 |
| `POST/PUT` | `/dashboard/saasAdmin/paymentDunning` | 可选启用。仅平台租户超级管理员可用，支持 `dryRun` 预演和应用催缴，同一订单按下次催缴时间幂等写入 `payment_failed_reminder` 通知 outbox |
| `GET` | `/dashboard/saasAdmin/paymentRefunds` | 可选启用。仅平台租户超级管理员可按租户、状态、支付提供方、订单号和关键字查看退款汇总与明细；`GET /dashboard/saasAdmin/export?type=paymentRefunds` 按同口径导出 CSV |
| `POST/PUT` | `/dashboard/saasAdmin/paymentRefund`、`/dashboard/saasAdmin/paymentRefundCancel` | 可选启用。平台财务可申请部分/全额退款或按乐观锁版本取消待处理退款；强制审批开启时，创建退款必须改走 `approvalRequest`，直写返回 `428`，取消待处理退款仍按乐观锁直接执行 |
| `GET` | `/dashboard/saasAdmin/paymentSettlementBatches`、`/dashboard/saasAdmin/paymentSettlementEntries` | 可选启用。仅平台租户超级管理员可按渠道、批次状态、匹配状态、处理状态和关键字查看渠道结算批次与明细；汇总区分自动匹配、缺内部单、标识冲突、状态/币种/金额差异和待处理数量；`export?type=paymentSettlementBatches/paymentSettlementEntries` 按同口径导出 CSV |
| `POST/PUT` | `/dashboard/saasAdmin/paymentSettlementImport` | 可选启用。仅平台租户超级管理员可导入 JSON 或 `text/csv` 渠道结算单，单次最多 5000 条、8 MiB；收款金额为正、退款金额为负，净额必须等于交易金额加手续费；原始内容摘要、批次号、渠道批次号和渠道流水共同保证幂等与重复拦截，导入和首次对账在同一事务完成 |
| `POST/PUT` | `/dashboard/saasAdmin/paymentSettlementReconcile` | 可选启用。仅平台租户超级管理员可按 `batchNo`、`expectedVersion` 执行 dry-run 预演或正式重对账；支付和退款分别按平台单号与渠道单号匹配，双标识指向不同内部记录时标记冲突，同一内部订单或退款只能被一条结算明细占用 |
| `POST/PUT` | `/dashboard/saasAdmin/paymentSettlementResolve` | 可选启用。仅平台租户超级管理员可按明细乐观锁版本将差异标记为 `resolved/ignored`，或重新打开为 `open`；已匹配明细无需人工处理，已关闭批次必须先重开，处理原因和操作人写入审计日志 |
| `POST/PUT` | `/dashboard/saasAdmin/paymentSettlementTransition` | 可选启用。平台财务可按批次乐观锁版本执行 `close/reopen`；强制审批开启时 `close` 必须改走 critical 双人审批，直写返回 `428`，申请冻结批次身份、来源摘要、计数、金额台账和版本，执行时完整复核并原子写入关账、审计与审批效果；`reopen` 仍可直接执行，存在待处理差异时禁止关账 |
| `GET` | `/dashboard/saasAdmin/paymentSettlementSyncRuns` | 可选启用。仅平台租户超级管理员可按渠道、来源和状态查看同步游标、最近成功/错误和运行历史；响应同时返回 Bridge 是否启用及已配置渠道 |
| `POST/PUT` | `/dashboard/saasAdmin/paymentSettlementSync` | 可选启用。仅平台租户超级管理员可对已配置渠道执行 `dryRun` 预演或立即同步；同渠道并发运行返回 `409`，预演不导入且不推进游标 |
| `GET` | `/dashboard/saasAdmin/accessProfile` | 可选启用。返回当前平台总后台成员的超级管理员状态、角色、权限和授权版本；平台超级管理员拥有隐式 `*` 权限，普通平台成员必须命中已启用角色权限 |
| `GET` | `/dashboard/saasAdmin/accessRoles` | 可选启用。需要 `platform.access.manage`，返回 25 项权限目录、六个内置角色和自定义角色；内置角色只读 |
| `POST/PUT` | `/dashboard/saasAdmin/accessRole` | 可选启用。需要 `platform.access.manage`，创建或按 `expectedVersion` 更新自定义平台角色；强制审批开启时必须改走审批，直写返回 `428`；权限依赖会自动补齐对应读取权限 |
| `GET` | `/dashboard/saasAdmin/accessAssignments` | 可选启用。需要 `platform.access.manage`，返回平台租户成员、超级管理员状态、授权状态、角色与授权版本 |
| `POST/PUT` | `/dashboard/saasAdmin/accessAssignment` | 可选启用。需要 `platform.access.manage`，按 `expectedVersion` 启停成员总后台访问并覆盖角色集合；强制审批开启时必须改走审批，直写返回 `428`；不能修改自己的授权，且授予本人的申请不能由本人复核或执行 |
| `GET` | `/dashboard/saasAdmin/identityOverview`、`/dashboard/saasAdmin/identitySessions`、`/dashboard/saasAdmin/identityLoginEvents`、`/dashboard/saasAdmin/identityIncidents` | 仅启用身份安全时挂载。需要 `platform.identity.read`，按租户、用户、会话状态、风险和关键词查看策略摘要、账号锁定、MFA 覆盖、活动/历史会话、不可变登录事件与安全事故；响应不返回 MFA 密钥、恢复码或哈希 |
| `POST/PUT` | `/dashboard/saasAdmin/identityPolicy` | 需要 `platform.identity.manage`，按 `expectedVersion` 维护失败锁定、会话 TTL/空闲超时/并发数、强制 MFA、IP/CIDR 和事件/会话保留策略；管理权限自动依赖身份查看和审计查看 |
| `POST/PUT` | `/dashboard/saasAdmin/identitySession`、`/dashboard/saasAdmin/identityUser`、`/dashboard/saasAdmin/identityMFA` | 需要 `platform.identity.manage`，按乐观锁撤销单会话或用户全部会话、解锁账号、重置 MFA；原因和前后状态写入平台操作审计，敏感凭据不会进入审计载荷 |
| `POST/PUT` | `/dashboard/saasAdmin/identityIncident` | 需要 `platform.identity.manage`，按乐观锁确认、分派、解决或重开暴力尝试、新 IP 和 MFA 失败等身份安全事故 |
| `GET` | `/dashboard/saasAdmin/serviceAccounts` | 可选启用。需要 `platform.integrations.read`，按租户、状态和关键字查看服务账号、授权 Scope、IP/CIDR 白名单、过期时间、密钥状态、预警策略、打开告警数及最近评估/通知摘要 |
| `GET` | `/dashboard/saasAdmin/serviceAccountUsage` | 可选启用。需要 `platform.integrations.read`，按 1 至 90 天、租户或服务账号查看 OpenAPI 成功/拒绝趋势、账号排行、路由排行和用量保留状态 |
| `POST/PUT` | `/dashboard/saasAdmin/serviceAccount` | 可选启用。需要 `platform.integrations.manage`；创建租户服务账号时返回一次明文 API Key，按 `expectedVersion` 更新现有账号必须提交 critical 双人审批，直接更新返回 `428` |
| `POST/PUT` | `/dashboard/saasAdmin/serviceAccountKeyRotate`、`/dashboard/saasAdmin/serviceAccountKeyRevoke` | 可选启用。需要 `platform.integrations.manage`；轮换与吊销均必须提交 critical 双人审批，直接调用返回 `428`。轮换申请可冻结旧 Key 宽限期，批准执行后仅在该次 `approvalExecute` 响应显示一次新 Key 明文，审批持久化结果不保存明文 |
| `POST/PUT` | `/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate` | 可选启用。需要 `platform.integrations.manage`，可按单账号或批次立即评估当日用量/限流拒绝；命中策略时事务化写入统一告警和通知 outbox，条件恢复时自动解决对应告警并关闭尚未送达的旧通知 |
| `GET` | `/dashboard/saasAdmin/backupOverview` | 需要 `platform.backups.read`，返回策略、运行、密钥可用性、异地副本和恢复演练历史，以及源库、工具、对象存储、恢复目标与自动调度启用状态、扫描间隔、启动检查状态 |
| `POST/PUT` | `/dashboard/saasAdmin/backupPolicy` | 需要 `platform.backups.manage`，按 `expectedVersion` 乐观锁更新周期、保留、演练间隔、强制加密和强制异地副本策略 |
| `POST/PUT` | `/dashboard/saasAdmin/backupRun` | 需要 `platform.backups.manage`；`action=create/verify/replicate/cleanup` 分别执行原子加密备份、双端完整性校验、异地副本上传或修复，以及本地与远端一致清理 |
| `POST/PUT` | `/dashboard/saasAdmin/restoreDrill` | 需要 `platform.backups.manage`，对指定成功备份执行隔离恢复；支持服务端预配置空库或自动创建并销毁临时库，不接受请求传入 DSN |
| `GET` | `/dashboard/saasAdmin/complianceOverview`、`/dashboard/saasAdmin/complianceErasureSteps` | 需要 `platform.compliance.read`，查看合规策略、配置就绪度、数据清单覆盖、法律保留、加密导出、擦除请求及逐步证据 |
| `POST/PUT` | `/dashboard/saasAdmin/compliancePolicy`、`/dashboard/saasAdmin/complianceLegalHold` | 需要 `platform.compliance.manage`，按乐观锁维护导出/擦除/财务/审计保留策略，并创建或解除租户法律保留 |
| `POST/PUT/GET` | `/dashboard/saasAdmin/complianceExport`、`/dashboard/saasAdmin/complianceExportDownload` | 需要 `platform.compliance.manage`，申请、执行、删除或下载已校验的租户加密导出；下载前重新校验工件 SHA-256 与 GCM 认证 |
| `POST/PUT` | `/dashboard/saasAdmin/complianceErasure` | 需要 `platform.compliance.manage`，创建、取消或处理租户擦除。创建必须精确输入已停用租户名；处理必须通过 `tenant.data.erase` 两人审批、宽限期、法律保留、最近导出和财务保留门禁 |
| `GET` | `/api/saas/v1/whoami`、`/api/saas/v1/usage`、`/api/saas/v1/alerts` | 使用 `Authorization: Bearer mch_live_...` 鉴权；分别需要 `profile.read`、`usage.read` 和 `alerts.read` Scope，强制服务账号租户归属、状态/过期、Key 状态/过期和 IP/CIDR 白名单，不接受 dashboard JWT 代替 API Key |
| `GET` | `/dashboard/saasAdmin/systemHealth`、`/dashboard/saasAdmin/systemHealthScans`、`/dashboard/saasAdmin/systemIncidents` | 可选启用。需要 `platform.system.read`，返回 MySQL/Redis、迁移版本、后台任务、通知积压、审批 SLA、运营任务、结算同步、备份/恢复、合规导出与擦除队列、密钥和对象存储配置健康结果，支持按事故状态、严重度、来源、负责人和关键词筛选 |
| `POST/PUT` | `/dashboard/saasAdmin/systemHealthScan` | 需要 `platform.system.manage`，手动执行健康扫描；扫描、事故聚合、自动恢复、通知 outbox 和操作审计在同一数据库事务中提交，未发生状态变化的重复扫描不重复通知 |
| `POST/PUT` | `/dashboard/saasAdmin/systemIncident` | 需要 `platform.system.manage`，按 `expectedVersion` 乐观锁认领、分派、解决或重开事故；人工解决后若下次扫描仍异常会自动重开并再次通知 |
| `GET` | `/dashboard/saasAdmin/approvalPolicies`、`/dashboard/saasAdmin/approvals`、`/dashboard/saasAdmin/approvalEvents`、`/dashboard/saasAdmin/approvalDecisions`、`/dashboard/saasAdmin/approvalDelegations` | 可选启用。需要 `platform.approvals.read`，返回五类持久化审批策略、审批汇总/列表、不可变事件、逐票会签明细和有效委托；审批单保留申请时的策略版本、法定票数、SLA、提醒和有效期快照 |
| `POST/PUT` | `/dashboard/saasAdmin/approvalPolicy`、`/dashboard/saasAdmin/approvalDelegation`、`/dashboard/saasAdmin/approvalReminders` | 可选启用。需要 `platform.approvals.manage`，可按乐观锁维护审批开关、退款金额阈值、会签票数、SLA/提醒间隔、有效期和复核委托，也可手动生成到期提醒；委托只在委托人仍有复核权且有效时间命中时生效，提醒事务化写入 outbox 并幂等去重 |
| `POST/PUT` | `/dashboard/saasAdmin/approvalRequest` | 可选启用。发起人必须同时拥有目标动作所需业务权限，且当前动作命中启用策略或退款金额阈值；服务端会规范化请求、计算 SHA-256、保存策略快照并按幂等键复用审批单 |
| `POST/PUT` | `/dashboard/saasAdmin/approvalDecision`、`/dashboard/saasAdmin/approvalCancel` | 可选启用。复核需要 `platform.approvals.review` 或有效委托；每个实际复核人只能投一票，任一驳回立即结束，批准票达到策略快照法定人数后才进入已批准。发起人及其受托链不能自批，发起人可在执行前撤回，全部使用乐观锁版本并追加事件与操作审计 |
| `POST/PUT` | `/dashboard/saasAdmin/approvalExecute` | 可选启用。需要 `platform.approvals.execute`，仅非发起人可执行已达到法定票数的批准申请；业务动作脱离客户端断连上下文，并在目标业务事务原子写入副作用标记，15 分钟租约恢复不会重放已提交动作；执行失败回到已批准并保留错误，成功后进入已执行 |
| `GET/POST/PUT` | `/dashboard/saasAdmin/invoiceProfile` | 可选启用。仅平台租户超级管理员可按 `tenantId` 查看或代管租户开票抬头、税号、地址电话、开户行账号和接收邮箱，保存使用 `expectedVersion` 防止覆盖并发修改 |
| `GET` | `/dashboard/saasAdmin/invoiceDocuments` | 可选启用。仅平台租户超级管理员可按租户、蓝票/红票、状态、订单号、关键字和上限查看开票单；`GET /dashboard/saasAdmin/export?type=invoiceDocuments` 按同口径导出 CSV |
| `POST/PUT` | `/dashboard/saasAdmin/invoice`、`/dashboard/saasAdmin/creditNote` | 可选启用。仅平台租户超级管理员可为已支付订单申请蓝票，或针对已开蓝票及退款金额申请红票；金额会在订单或原蓝票内原子预占，幂等键和行锁阻止重复开票、超额开票或超额红冲 |
| `POST/PUT` | `/dashboard/saasAdmin/invoiceTransition` | 可选启用。仅平台租户超级管理员可将开票单推进到 `processing/issued/failed/canceled`；开具时保存提供方单号，失败或取消释放预占，蓝票与红票净额必须始终和订单净收款一致 |
| `GET/HEAD` | `/dashboard/saasBilling/page` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL=1` 时挂载，提供租户自助账单中心页面；页面和全部 API 要求真实 dashboard JWT |
| `GET` | `/dashboard/saasBilling/summary`、`/dashboard/saasBilling/paymentOrders`、`/dashboard/saasBilling/paymentRefunds`、`/dashboard/saasBilling/invoices` | 可选启用。返回当前 JWT 租户的净收款、可开票额、收款订单、退款和开票单；即使传入其他 `tenantId` 也不会越权 |
| `GET/POST/PUT` | `/dashboard/saasBilling/invoiceProfile` | 可选启用。租户超级管理员可查看和维护本租户开票资料，使用 `expectedVersion` 乐观锁保护并发更新 |
| `POST/PUT` | `/dashboard/saasBilling/invoice`、`/dashboard/saasBilling/invoiceCancel` | 可选启用。租户超级管理员可在当前可开票金额内申请蓝票，并只可取消本租户尚未进入处理中的申请；开具、失败和红冲仍由平台总后台处理 |
| `POST` | `/webhooks/saas/payment` | 支付提供方中立回调边界；仅在启用支付回调时挂载，使用 `X-Mochat-Go-Payment-Timestamp` 和 `X-Mochat-Go-Payment-Signature` 完成 HMAC-SHA256 验签，按事件 ID 幂等处理 `processing/succeeded/failed/canceled`；支付成功会事务化结算订单、续费账单、租户套餐和订阅 |
| `POST/PUT` | `/dashboard/saasAdmin/notificationHealthRecovery` | 可选启用。平台租户超级管理员可按 `tenantId`、`keyword`、`channel=webhook`、`windowHours` 和 `staleMinutes` 重新核验当前 `notification_health` 认领；只有当前窗口明确为 `healthy` 且至少存在一次成功送达的租户会自动写入 `resolved` 的 `saas.admin.operation_queue.assignment_close` 审计，`warning/critical/no_data` 以及只有抑制、关闭或延期而无成功送达的租户保持打开，重复执行不会重复关闭；普通租户访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/notificationPolicies` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantId`、`state=all/enabled/disabled/unconfigured`、`keyword`、`channel=webhook` 和 `limit` 查看跨租户通知策略；响应返回租户总数、已配置、已启用、已停用、未配置和当前筛选命中数，Webhook Secret 只返回是否已配置；普通租户访问返回 `403` |
| `GET/POST/PUT` | `/dashboard/saasAdmin/notificationPolicy` | 可选启用。平台租户超级管理员可按 `tenantId` 读取或代管租户 Webhook 开关、URL、Secret、请求超时、HTTP/outbox 重试、模板、事件订阅、最低严重级别、免打扰时段、IANA 时区和每小时限流策略；未传 `webhookSecret` 时保留旧密钥，`clearWebhookSecret=true` 才清空，保存后写入 `tenant.notification_policy.update` 审计日志且 before/after 不包含密钥；普通租户访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/notificationPolicyTest` | 可选启用。平台租户超级管理员可为已配置且启用的租户生成 `metric=notification_policy`、`alertType=notification_policy_test` 的 pending 测试通知，沿用租户 outbox 最大尝试次数并写入 `tenant.notification_policy.test` 审计日志；测试通知绕过事件订阅、最低严重级别、免打扰和小时限流，但仍由 SaaS 通知 dispatcher 通过 outbox 实际投递，普通租户访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/notificationCredentialRotation` | 可选启用。仅平台超级管理员可按可选 `tenantId` 和 `limit` 将旧明文或历史 Key 的通知凭据批量改写到活动 Key；响应和 `saas.admin.notification_credential.rotate` 审计只记录数量、Key ID 与保护状态，不返回 URL 或 Secret |
| `GET` | `/dashboard/saasAdmin/wecomCredentialProtection` | 可选启用。需要 `platform.integrations.read`，返回企业/应用凭据总数、密文、旧明文、待轮换、不可用 Key 和活动 Key 等保护统计；不返回任何企微 Secret、Token 或 AES Key |
| `POST/PUT` | `/dashboard/saasAdmin/wecomCredentialRotation` | 可选启用。需要 `platform.integrations.manage`，按可选 `tenantId` 和 `limit` 将旧明文或历史 Key 企微凭据改写到活动 Key；响应与 `saas.admin.wecom_credential.rotate` 审计只保存数量和非敏感 Key ID |
| `POST/PUT` | `/dashboard/saasAdmin/notificationRetry` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，按 `notificationId` 将 failed/dead 通知重置为 pending、清空失败原因并立即排队，写入 `tenant.notification.retry` 操作日志；平台租户超级管理员可处理任意租户，普通租户超级管理员只能处理自己 |
| `POST/PUT` | `/dashboard/saasAdmin/notificationBulkRetry` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，可按 `tenantId`、`status=failed/dead/all`、`channel`、`keyword` 和 `limit` 批量将 failed/dead 通知重置为 pending、清空失败原因并立即排队；平台租户超级管理员可批量处理任意租户，普通租户超级管理员只能批量处理自己 |
| `POST/PUT` | `/dashboard/saasAdmin/notificationClose` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，按 `notificationId` 将 pending/failed 通知关闭为 `closed`、清空下次重试时间，并写入 `tenant.notification.close` 操作日志；平台租户超级管理员可处理任意租户，普通租户超级管理员只能处理自己 |
| `POST/PUT` | `/dashboard/saasAdmin/notificationBulkClose` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，可按 `tenantId`、`status=pending/failed/all`、`channel`、`keyword` 和 `limit` 批量关闭 pending/failed 通知，阻止后续 outbox 投递；平台租户超级管理员可批量处理任意租户，普通租户超级管理员只能批量处理自己 |
| `GET` | `/dashboard/saasAdmin/packages` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可查看可分配套餐及 26 项 SaaS 额度 |
| `GET` | `/dashboard/saasAdmin/operations` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可查看套餐维护、租户开停、平台开户、续费、租户套餐调整、运营任务创建/应用/阻断/取消/批量取消/批量重置操作记录，可按 `tenantId`、`action`、`targetType`、`keyword` 过滤；`summary` 返回当前筛选全量命中的操作数、租户数、操作人数、动作类型数和目标类型数，列表仍按 `limit` 截断；`keyword` 会匹配动作、目标、备注和变更 JSON；页面操作记录表可展开查看 before/after JSON 变更详情 |
| `GET` | `/dashboard/saasAdmin/billingEvents` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可查看租户续费账单事件，可按 `tenantId`、`eventType`、`packageCode`、`keyword` 过滤，并返回当前筛选全量命中的账单笔数、续费笔数和金额合计，列表仍按 `limit` 截断；`keyword` 会匹配套餐、支付方式、外部订单号、备注和 metadata |
| `GET` | `/dashboard/saasAdmin/billingReconciliation` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按账单筛选条件对账续费账单和当前租户套餐，返回正常数、异常数、无当前套餐、当前套餐未启用、套餐不一致和到期未覆盖数量；`mismatchOnly=1` 只返回异常账单 |
| `POST/PUT` | `/dashboard/saasAdmin/billingReconciliationFollowUp` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可对指定 `billingEventId` 记录账单对账异常跟进状态、负责人、下次跟进时间和备注；接口不改写账单或套餐事实，只追加 `billing.reconciliation.follow_up` / `billing_event` 操作日志 |
| `GET` | `/dashboard/saasAdmin/billingReconciliationFollowUps` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantId`、`status`、`owner`、`dueState=all/overdue/due_soon/future/no_date/closed`、`keyword` 和 `limit` 查看每个账单事件最新对账跟进任务，返回状态、负责人、下次跟进、订单、套餐、金额、到期状态和操作 ID；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/billingReconciliationFollowUpOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按账单跟进任务同口径筛选并按负责人聚合，返回打开任务数、状态分布、逾期/7 天内/未来/无日期/已关闭分布、最近记录时间和最早下次跟进时间；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantId`、`filterStatus`、`owner`、`dueState`、`keyword` 和 `limit` 批量关闭账单对账跟进任务，`closeStatus` 仅支持 `resolved/ignored`，服务端会为每个命中的未关闭账单事件追加 `billing.reconciliation.follow_up` / `billing_event` 操作日志；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/tasks` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可查看 SaaS 总后台运营任务，可按 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 过滤；当前任务类型包含 `package_sync`、`tenant_provision` 和 `tenant_renewal`，任务状态包含 `pending`、`blocked`、`failed`、`canceled` 和 `applied`；响应会返回不受列表 `limit` 影响的 `summary` 和 `returnedCount`，汇总任务总数、各状态数量、可处理任务数、各任务类型数量、命中租户数和操作人数 |
| `GET` | `/dashboard/saasAdmin/taskOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营任务同一筛选口径聚合任务负责人，返回负责人任务数、可处理/阻断/失败/已应用分布、三类任务分布、最近任务、最近应用时间和最近错误；接口只读复用 `mochat_go_saas_admin_tasks`，不新增迁移；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/taskSla` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营任务同一筛选口径查看活跃任务 SLA，支持 `warningHours` 和 `overdueHours` 阈值；接口只把 `pending/blocked/failed` 作为活跃任务计算，返回正常、预警、逾期、未知时间、最大任务年龄、负责人聚合和最严重任务列表；只读复用 `mochat_go_saas_admin_tasks`，不新增迁移；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/taskSlaNotifications` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按运营任务 SLA 同一筛选生成 webhook 提醒 outbox；默认只提醒 `warning/overdue` 活跃任务，支持 `slaStatus=warning/overdue/fresh/unknown/all`、`maxAttempts`、`remark` 和 `forceCreate`；通知写入 `mochat_go_saas_alert_notifications`，`alertType=admin_task_sla_reminder`、`metric=admin_task_sla`，并追加 `saas.admin.task.sla_notify` 操作日志；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/taskCancel` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可取消 `pending/blocked/failed` 运营任务并写入取消结果，同时追加 `saas.admin.task.cancel` 操作日志；已应用任务不可取消，已取消任务不可再次应用 |
| `POST/PUT` | `/dashboard/saasAdmin/taskBulkCancel` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选批量取消未应用运营任务；接口会跳过 `applied/canceled` 任务并返回取消数和跳过数，并为每个取消任务追加 `saas.admin.task.bulk_cancel` 操作日志 |
| `POST/PUT` | `/dashboard/saasAdmin/taskBulkReset` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选批量把 `failed/blocked` 运营任务重置为 `pending`、清空失败结果和错误；接口会跳过 `pending/applied/canceled` 任务并返回重置数和跳过数，并为每个重置任务追加 `saas.admin.task.bulk_reset` 操作日志 |
| `POST/PUT` | `/dashboard/saasAdmin/taskReset` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可把 `failed/blocked` 运营任务重置为 `pending`、清空失败结果和错误，并追加 `saas.admin.task.reset` 操作日志；已应用、已取消和已待应用任务不可重置 |
| `GET` | `/dashboard/saasAdmin/dailyReport` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可查看运营日报，支持 `date=YYYY-MM-DD`、`days`、`expiringDays`、`highUsageRatio`、`tenantLimit` 和 `limit`，汇总当前风险租户、风险跟进负责人、运营任务 SLA、打开告警、失败/耗尽通知、日期窗口内操作、风险跟进动作、告警处置、通知重试、续费账单数量和账单金额；运营任务 SLA 默认按 4 小时预警、24 小时逾期统计 `pending/blocked/failed` 活跃任务；内置总后台页面提供日报日期和统计天数筛选，展示日报负责人、日报任务 SLA、失败通知、操作动作和账单流水明细，并会联动日报 CSV 导出；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可用 `type=tenants/usage/packages/risk/operationQueue/operationQueueOwners/operationQueueAssignments/customerSuccess/customerSuccessOwners/riskFollowUps/riskFollowUpOwners/alerts/notifications/tasks/taskSla/dailyReport/operations/billingEvents/billingReconciliation/billingReconciliationFollowUps/billingReconciliationFollowUpOwners` 导出 CSV；租户导出复用 `keyword`、`tenantStatus`、`packageCode`、`dueState` 筛选，用量导出按筛选租户逐项输出 26 项 SaaS 指标、额度、剩余、状态和打开告警数，套餐导出输出套餐状态和 26 项 SaaS 额度，风险导出额外支持 `highUsageRatio` 并输出风险等级、风险分、原因、建议动作、最新跟进状态、负责人、下次跟进时间、跟进备注和 Top 风险指标，运营待办导出复用 `operationQueue` 筛选并输出来源、优先级、租户、负责人、原因、下一步动作和对象，运营待办负责人导出复用同一组筛选并输出负责人待办数、租户数、来源数、优先级分布、来源分布、未分配数、最大停留小时、Top 租户和 Top 待办，运营待办认领导出复用 `source`、`owner`、`status`、`dueState`、对象、租户和关键字筛选并输出到期状态，客户成功负责人导出复用 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority` 筛选并输出负责人租户数、优先级分布、到期/阻断、账单、任务、通知、健康分和 Top 租户，风险跟进任务导出复用 `tenantId`、`status`、`owner`、`dueState` 和 `keyword` 筛选并输出到期状态、逾期标记和操作 ID，风险跟进负责人导出复用同一组筛选并输出负责人任务汇总、状态分布和到期分布，告警导出复用 `tenantId`、`status`、`metric`、`alertType` 和 `limit` 筛选并输出告警 key、状态、严重级别、指标、用量、来源、消息和上下文，通知导出复用 `tenantId`、`status`、`channel`、`keyword` 和 `limit` 筛选并输出通知 key、状态、重试次数、指标、失败原因和下次重试时间，账单跟进任务导出复用 `tenantId`、`status`、`owner`、`dueState`、`keyword` 和 `limit` 筛选并输出账单事件、套餐、金额、订单号、到期状态、逾期标记和操作 ID，账单跟进负责人导出复用同一组筛选并输出负责人任务汇总、状态分布和到期分布，运营任务导出复用 `taskId`、`taskType`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选并输出任务请求、预览和结果，平台开户任务请求会继续脱敏明文密码和 `adminPasswordHash`，任务 SLA 导出复用任务筛选和 `warningHours/overdueHours` 输出活跃任务时效，运营日报导出复用 `date`、`days`、`expiringDays`、`highUsageRatio`、`tenantLimit` 和 `limit`，按 `section/metric/value/remark` 输出窗口、汇总、负责人、风险租户、任务 SLA 负责人、任务 SLA 明细、失败通知、操作动作和账单流水，操作记录、账单流水和账单对账导出复用对应列表筛选，默认 1000 条，上限 5000 条 |
| `GET` | `/dashboard/saasAdmin/export?type=customerSuccess` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `tenantLimit`、`limit`、`expiringDays`、`highUsageRatio`、`owner` 和 `priority=critical/high/medium/normal/all` 导出客户成功队列 CSV，输出优先级、健康分、负责人、到期状态、待处理原因、下一步动作、风险、账单跟进、运营任务和失败通知信号；普通租户超级管理员访问返回 `403` |
| `GET` | `/dashboard/saasAdmin/export?type=customerSuccessOwners` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按客户成功队列同口径导出负责人工作台 CSV，输出负责人租户数、优先级分布、逾期/7 天内/阻断、账单跟进、可处理运营任务、失败/耗尽通知、最高/平均健康分、最早下次跟进时间和 Top 租户；普通租户超级管理员访问返回 `403` |
| `POST/PUT` | `/dashboard/saasAdmin/package` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可创建、编辑、启用或停用套餐，并维护 26 项 SaaS 额度 |
| `POST/PUT` | `/dashboard/saasAdmin/packageSync` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `packageCode`、可选 `tenantId` 和 `limit` 预览或应用套餐目录到存量租户套餐快照；默认 `dryRun=true` 不写入，实际同步遇到超新额度租户会阻断，显式 `allowOverLimit=true` 后才会调用租户套餐快照更新并刷新 26 项 SaaS 用量额度 |
| `POST/PUT` | `/dashboard/saasAdmin/packageSyncTask` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按套餐同步参数创建运营任务；服务端会先 dry-run 生成预览，遇到超额且未允许超额时任务状态为 `blocked`，否则为 `pending` |
| `POST/PUT` | `/dashboard/saasAdmin/packageSyncTaskApply` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId` 应用套餐同步任务；应用前会重新计算当前超额风险，成功后写入任务执行结果和 `appliedAt` |
| `POST/PUT` | `/dashboard/saasAdmin/packageSyncTaskBulkApply` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId`、`taskType=package_sync`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选批量应用套餐同步任务；每条任务应用前都会重新计算当前套餐、租户快照和超额风险，成功任务写入租户套餐快照和 26 项用量额度并置为 `applied`，仍超额且未允许超额的任务置为 `blocked`，坏请求或执行失败置为 `failed`，响应返回应用、阻断、失败和跳过数量 |
| `POST/PUT` | `/dashboard/saasAdmin/tenantStatus` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可把业务租户状态切换为正常或停用，并写入 SaaS 总后台操作日志；平台管理租户不能被停用；业务租户停用后新登录返回 `403`，旧 token 再访问依赖 `UserByID` 的后台接口会按用户不可用处理 |
| `POST/PUT` | `/dashboard/saasAdmin/tenantRenewal` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可记录租户续费，服务端会更新租户套餐到期时间、写入 `mochat_go_saas_billing_events` 并记录 `tenant.renewal` 操作日志；到期租户续费到未来时间后，新登录和依赖 `UserByID` 的后台接口会恢复 |
| `POST/PUT` | `/dashboard/saasAdmin/tenantRenewalTask` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按续费参数创建运营任务；服务端会固化解析后的套餐编码、生成当前租户续费预览，若新到期时间早于当前到期时间则任务状态为 `blocked`，否则为 `pending` |
| `POST/PUT` | `/dashboard/saasAdmin/tenantRenewalTaskApply` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId` 应用续费任务；应用前会重新读取当前租户状态和套餐状态，成功后写入续费账单、任务执行结果和 `appliedAt` |
| `POST/PUT` | `/dashboard/saasAdmin/tenantRenewalTaskBulkApply` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId`、`taskType=tenant_renewal`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选批量应用续费任务；每条任务应用前都会重新读取当前租户和套餐状态，成功任务写入续费账单并置为 `applied`，预演阻断置为 `blocked`，坏请求或执行失败置为 `failed`，响应返回应用、阻断、失败和跳过数量 |
| `POST/PUT` | `/dashboard/saasAdmin/tenantProvision` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可开通新业务租户和初始超级管理员，服务端会写入租户、管理员、角色、角色菜单、租户套餐快照、用量计数、开通记录和 `tenant.provision` 操作日志；管理员手机号如果已被其他租户占用会拒绝，避免登录歧义 |
| `POST/PUT` | `/dashboard/saasAdmin/tenantProvisionTask` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可创建平台开户任务；服务端会立即把初始密码转换成密码哈希后固化请求，任务响应和任务列表只返回 `hasAdminPasswordHash` 标记，不回显明文密码或哈希 |
| `POST/PUT` | `/dashboard/saasAdmin/tenantProvisionTaskApply` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId` 应用平台开户任务；应用前会复核套餐仍启用，成功后复用开户链接逻辑写入租户、管理员、角色授权、套餐快照、用量计数、开通记录和任务应用结果，失败会把任务更新为 `failed` |
| `POST/PUT` | `/dashboard/saasAdmin/tenantProvisionTaskBulkApply` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可按 `taskId`、`taskType=tenant_provision`、`status`、`tenantId`、`packageCode` 和 `limit` 筛选批量应用平台开户任务；每条任务应用前都会复核套餐仍启用，成功任务复用开户链接逻辑创建租户、管理员、角色授权、套餐快照、用量计数和开通记录并置为 `applied`，坏请求或执行失败置为 `failed`，响应返回应用、阻断、失败和跳过数量，任务请求仍只返回脱敏字段 |
| `POST/PUT` | `/dashboard/saasAdmin/tenantPackage` | 可选启用。只有设置 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1` 时走 Go，平台租户超级管理员可给指定租户调整套餐和到期时间，服务端会写入租户套餐快照并刷新全部 SaaS 用量额度 |
| `GET` | `/dashboard/user/loginShow` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_LOGIN_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/user/logout` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_LOGOUT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/role/permissionByUser` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_PERMISSION_BY_USER=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/corp/select` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_SELECT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/corp/bind` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_BIND=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/corp/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/corp/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/corp/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/corp/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET/POST` | `/dashboard/corp/weWorkCallback` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WEWORK_CALLBACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET/POST` | `/weWork/callback` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WEWORK_CALLBACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/chatTool/config` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/WW_verify_{16位字母数字}.txt` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/agent/txtVerifyUpload` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY_UPLOAD=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/agent/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_AGENT_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/agent/auth` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_AUTH=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/sidebar/agent/auth` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_AUTH=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/agent/oauth` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_OAUTH=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/agent/jssdkConfig` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_JSSDK_CONFIG=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/wxJsSdk/config` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_WX_JS_SDK_CONFIG=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/role/select` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROLE_SELECT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/role/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROLE_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/role/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROLE_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/role/permissionShow` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROLE_PERMISSION_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/role/showEmployee` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROLE_SHOW_EMPLOYEE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/menu/iconIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_MENU_ICON_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/menu/select` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_MENU_SELECT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/menu/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_MENU_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/menu/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_MENU_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/corpData/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_DATA_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/corpData/lineChat` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CORP_DATA_LINE_CHAT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workEmployee/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workEmployee/searchCondition` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SEARCH_CONDITION=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workEmployee/synEmployee` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workDepartment/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workEmployeeDepartment/memberIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_DEPARTMENT_MEMBER_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workDepartment/memberIndex` | 旧前端别名，跟随 `MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_DEPARTMENT_MEMBER_INDEX=1` 接管并复用部门成员列表逻辑 |
| `GET` | `/dashboard/workDepartment/selectByPhone` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SELECT_BY_PHONE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workDepartment/pageIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_PAGE_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workDepartment/showEmployee` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SHOW_EMPLOYEE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContactTagGroup/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContactTagGroup/detail` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DETAIL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/workContactTagGroup/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_GROUP_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContactTag/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContactTag/detail` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DETAIL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContactTag/contactTagList` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_LIST=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContactTag/allTag` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_ALL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workContactTag/synContactTag` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workContact/synContact` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContact/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/workContactTag/allTag` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_ALL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/workContact/detail` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_DETAIL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/workContact/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/workContact/track` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TRACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/sidebar/workContact/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/contactProcessStatus/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/sidebar/contactProcessStatus/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContact/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContact/lossContact` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_LOSS=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContact/source` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_SOURCE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContact/track` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_TRACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workContact/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/workContact/batchLabeling` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workContactRoom/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_CONTACT_ROOM_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workRoom/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workRoom/roomIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_ROOM_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workRoom/statistics` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workRoom/statisticsIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workRoom/syn` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workRoom/batchUpdate` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactTransfer/info` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INFO=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactTransfer/unassignedList` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_UNASSIGNED_LIST=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactTransfer/room` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactTransfer/log` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_LOG=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactTransfer/saveUnassignedList` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_SAVE_UNASSIGNED_LIST=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/contactTransfer/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/contactTransfer/room` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workRoomAutoPull/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workRoomAutoPull/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/workRoomAutoPull/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workRoomAutoPull/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workRoomAutoPull/move` | 旧前端兼容幂等接口，跟随 `MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE=1` 接管并复用更新权限 |
| `GET` | `/dashboard/roomTagPull/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomTagPull/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomTagPull/showContact` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW_CONTACT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomTagPull/roomList` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_ROOM_LIST=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomTagPull/chooseContact` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_CHOOSE_CONTACT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/roomTagPull/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/roomTagPull/filterContact` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_FILTER_CONTACT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomTagPull/remindSend` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_REMIND_SEND=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `DELETE` | `/dashboard/roomTagPull/destroy` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactMessageBatchSend/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactMessageBatchSend/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactMessageBatchSend/messageShow` | 预览别名，跟随 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW=1` 接管并返回详情内容预览 |
| `GET` | `/dashboard/contactMessageBatchSend/showRoom` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW_ROOM=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactMessageBatchSend/employeeSendIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_EMPLOYEE_SEND_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactMessageBatchSend/contactReceiveIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_CONTACT_RECEIVE_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/contactMessageBatchSend/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/contactMessageBatchSend/remind` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `DELETE` | `/dashboard/contactMessageBatchSend/destroy` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomMessageBatchSend/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomMessageBatchSend/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomMessageBatchSend/roomOwnerSendIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_OWNER_SEND_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomMessageBatchSend/roomReceiveIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_RECEIVE_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/roomMessageBatchSend/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/roomMessageBatchSend/remind` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_REMIND=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `DELETE` | `/dashboard/roomMessageBatchSend/destroy` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/officialAccount/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/officialAccount/set` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/officialAccount/getPreAuthUrl` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_GET_PRE_AUTH_URL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/officialAccount/authRedirect/` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_REDIRECT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/officialAccount/authRedirect/` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_REDIRECT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/officialAccount/authEventCallback` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_EVENT_CALLBACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/officialAccount/authEventCallback` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_EVENT_CALLBACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/{appId}/officialAccount/messageEventCallback` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_MESSAGE_EVENT_CALLBACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/{appId}/officialAccount/messageEventCallback` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_MESSAGE_EVENT_CALLBACK=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/load/{params?}` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_LOAD=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/load/{params?}` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_LOAD=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workFission/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workFission/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workFission/info` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_INFO=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workFission/statistics` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_STATISTICS=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workFission/chooseContact` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_CHOOSE_CONTACT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/workFission/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/workFission/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/workFission/invite` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workFission/inviteData` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DATA=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/workFission/inviteDetail` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DETAIL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `DELETE` | `/dashboard/workFission/destroy` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_WORK_FISSION_DESTROY=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/operation/workFission/inviteFriends` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_INVITE_FRIENDS=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/operation/workFission/poster` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_POSTER=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/operation/workFission/taskData` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_TASK_DATA=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/operation/workFission/receive` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_RECEIVE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/operation/auth/workFission` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_AUTH=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/operation/auth/workFission` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_AUTH=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/operation/openUserInfo/workFission` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_OPEN_USER_INFO=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactField/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_FIELD_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactField/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_FIELD_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactField/portrait` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_FIELD_PORTRAIT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/contactFieldPivot/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/contactFieldPivot/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/sidebar/contactFieldPivot/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/sidebar/contactFieldPivot/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/channelCode/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/channelCode/show` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_SHOW=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/channelCode/contact` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_CONTACT=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/channelCode/statistics` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/channelCode/statisticsIndex` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/channelCode/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/channelCode/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/channelCodeGroup/index` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_INDEX=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `GET` | `/dashboard/channelCodeGroup/detail` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_DETAIL=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `POST` | `/dashboard/channelCodeGroup/store` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_STORE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/channelCodeGroup/update` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_UPDATE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |
| `PUT` | `/dashboard/channelCodeGroup/move` | 可选接管。兼容模式下设置 `MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_MOVE=1` 时走 Go；生产 standalone 且启用全量已迁移路由时默认走 Go |

未在 Go 内置处理的路径默认不会自动连接 PHP；只有显式设置 `MOCHAT_PHP_UPSTREAM` 时才会通过 reverse proxy 转发到 PHP。

## 独立 Go 运行模式

当前项目正在从 PHP fallback 兼容网关推进到独立 Go 项目。独立运行门槛是：运行时不依赖 `../mochat` 源码目录、不依赖 PHP Hyperf upstream、不依赖外部 `compat_manifest.json`。

截至当前审计，`scripts/standalone_route_coverage.sh` 显示清单基线 224 条路由，已命中 Go manifest 的路由 224 条，剩余未迁移 0 条。路由覆盖已经补齐；完整独立版仍需要继续以队列、定时任务、异步回调 worker、前端构建产物和全业务回归作为验收标准。

轻量 standalone 可不配置数据库，只验证 Go 进程、内置 manifest 和前端静态产物：

```bash
cd /Users/lv/Documents/企业微信/mochat-go
env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR=127.0.0.1:18082 \
  go run ./cmd/mochat-go
```

生产 standalone 配置 `MOCHAT_MYSQL_DSN` 和 `MOCHAT_SIMPLE_JWT_SECRET` 后，会默认启用当前所有已迁移业务路由，不需要再逐个设置 `MOCHAT_GO_MIGRATE_*`：

```bash
env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1 \
  MOCHAT_GO_ADDR=127.0.0.1:18082 \
  MOCHAT_MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/mochat?parseTime=true&loc=Local' \
  MOCHAT_REDIS_ADDR=127.0.0.1:6379 \
  MOCHAT_SIMPLE_JWT_SECRET='请替换成生产密钥' \
  go run ./cmd/mochat-go
```

独立模式下：

- `/readyz` 只检查 Go 进程和内置清单，不检查 PHP upstream。
- `/compat/routes` 使用编译进 Go 模块的内置迁移清单。
- 配置 MySQL 和 dashboard JWT secret 后，默认启用当前所有已迁移业务路由；`MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES=0` 可关闭该默认值。
- `/compat/status` 会返回 `background_tasks`，用于观察已启用后台任务的 `run_id` 和 `running/failed/stopped` 状态。
- 未迁移路由返回 `501 route not yet migrated in standalone Go runtime`，不会 fallback 到 PHP。
- 文件上传默认写入 `./storage/upload/static`，不再默认写到 `../mochat/api-server/storage/upload/static`；Go standalone 会直接从该目录托管 `/static/*`，渠道码、自动拉群、标签建群、素材和头像等本地资源不需要 PHP 或外部 Nginx 代托管。
- dashboard 由主监听地址根路径托管；sidebar 和 operation 同时可在主监听地址通过 `/sidebar-app/`、`/operation-app/` 前缀加载，Go 静态托管层会重写旧 dist 中的根路径 JS/CSS、webpack publicPath、Vue Router base 和缺失构建变量导致的 API base，避免与 dashboard 根路径资源冲突。设置 `MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1` 或显式设置 `MOCHAT_SIDEBAR_FRONTEND_ADDR` / `MOCHAT_OPERATION_FRONTEND_ADDR` 后，sidebar 和 operation 仍可使用独立监听地址托管。三个入口共用同一个 Go API handler，但各自读取 `web/dashboard/dist`、`web/sidebar/dist`、`web/operation/dist`，不依赖原 `../mochat` 前端目录。

自动化检查：

```bash
./scripts/smoke_standalone.sh
./scripts/standalone_stack_check.sh
./scripts/smoke_schema_migrate.sh
./scripts/smoke_bootstrap_standalone.sh
./scripts/standalone_inventory_parity.sh
./scripts/standalone_route_coverage.sh
./scripts/audit_saas_storage_reclaim_coverage.sh
./scripts/audit_wework_callback_event_coverage.sh
./scripts/smoke_saas_storage_reconcile.sh
./scripts/smoke_saas_storage_reclaim.sh
./scripts/smoke_saas_usage_refresh.sh
./scripts/smoke_official_account_ticket.sh
./scripts/smoke_queue_idempotency.sh
./scripts/smoke_async_file_upload_worker.sh
./scripts/smoke_mark_tags_worker.sh
./scripts/smoke_auto_tag_keyword_task.sh
./scripts/smoke_auto_tag_dashboard.sh
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
./scripts/smoke_work_message_archive_sync_cron.sh
./scripts/smoke_sensitive_word_monitor_cron.sh
./scripts/smoke_frontend_static_browser.sh
./scripts/smoke_admin_core_dashboard.sh
./scripts/smoke_dashboard_frontend_login.sh
./scripts/smoke_sidebar_frontend_contact.sh
./scripts/smoke_operation_frontend_work_fission.sh
```

完整独立项目的历史缺口记录在 `docs/phases/phase-pre0-standalone/reports/standalone-gap.md`，当前开发进度统一查看 `docs/PROJECT_PROGRESS.zh-CN.md`。

## 可选接管 loginShow

当前 `loginShow` 已具备 Go 业务实现、MySQL 仓储、PHP `qbhy/simple-jwt` 兼容解析和 Redis 黑名单检查。为了避免未完成环境联调时破坏生产流量，默认不接管该接口。

正式接管需要沿用 PHP 的 `SIMPLE_JWT_SECRET` 和 Redis：

```bash
env -u GOROOT \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_MIGRATE_LOGOUT=1 \
  MOCHAT_GO_MIGRATE_PERMISSION_BY_USER=1 \
  MOCHAT_GO_MIGRATE_CORP_SELECT=1 \
  MOCHAT_GO_MIGRATE_CORP_BIND=1 \
  MOCHAT_GO_MIGRATE_CORP_INDEX=1 \
  MOCHAT_GO_MIGRATE_CORP_SHOW=1 \
  MOCHAT_GO_MIGRATE_CORP_STORE=1 \
  MOCHAT_GO_MIGRATE_CORP_UPDATE=1 \
  MOCHAT_GO_MIGRATE_WEWORK_CALLBACK=1 \
  MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG=1 \
  MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY=1 \
  MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY_UPLOAD=1 \
  MOCHAT_GO_MIGRATE_AGENT_STORE=1 \
  MOCHAT_GO_MIGRATE_ROLE_SELECT=1 \
  MOCHAT_GO_MIGRATE_ROLE_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROLE_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROLE_PERMISSION_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROLE_SHOW_EMPLOYEE=1 \
  MOCHAT_GO_MIGRATE_MENU_ICON_INDEX=1 \
  MOCHAT_GO_MIGRATE_MENU_SELECT=1 \
  MOCHAT_GO_MIGRATE_MENU_INDEX=1 \
  MOCHAT_GO_MIGRATE_MENU_SHOW=1 \
  MOCHAT_GO_MIGRATE_CORP_DATA_INDEX=1 \
  MOCHAT_GO_MIGRATE_CORP_DATA_LINE_CHAT=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SEARCH_CONDITION=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_DEPARTMENT_MEMBER_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SELECT_BY_PHONE=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_PAGE_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SHOW_EMPLOYEE=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DETAIL=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DETAIL=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_LIST=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_ALL=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_LOSS=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_SOURCE=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_SHOW=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TRACK=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_SHOW=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_PORTRAIT=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_SHOW=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW_CONTACT=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_ROOM_LIST=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_CHOOSE_CONTACT=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_FILTER_CONTACT=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_REMIND_SEND=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW_ROOM=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_EMPLOYEE_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_CONTACT_RECEIVE_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_OWNER_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_RECEIVE_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_REMIND=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_INDEX=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_GET_PRE_AUTH_URL=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_REDIRECT=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_EVENT_CALLBACK=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_MESSAGE_EVENT_CALLBACK=1 \
  MOCHAT_GO_MIGRATE_LOAD=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_INDEX=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_DETAIL=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_STORE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_MOVE=1 \
  MOCHAT_MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/mochat?parseTime=true&loc=Local' \
  MOCHAT_SIMPLE_JWT_SECRET='与 PHP SIMPLE_JWT_SECRET 一致' \
  MOCHAT_REDIS_ADDR='127.0.0.1:6379' \
  go run ./cmd/mochat-go
```

Go 会按 PHP 规则读取：

- `Authorization: Bearer <token>`，其次是请求参数 `token`。
- `header.payload.bcrypt(password_hash(md5(header.payload + secret)))` 签名。
- `exp`、`nbf`、`uid` payload。
- Redis 黑名单键 `[jwt:blacklist:{prefix}:{jti}][1]`，这是 PHP Doctrine RedisCache 的真实 key 形态。
- Redis 登录企业缓存 `mc:user.{userId}`。

`auth` 接管后会：

- 读取 `phone`、`password`。
- 查询 `mc_user.phone`，要求 `status = 1`。
- 用 PHP `PasswordHashEncrypter` 规则校验密码：`password_verify(md5(password + secret), password_hash)`。
- 签发可被 PHP `qbhy/simple-jwt` 解析的 token。
- 返回 `token` 和 `expire`，`expire` 保持 PHP `SIMPLE_JWT_TTL` 原值。

`logout` 接管后会：

- 解析并校验当前 PHP JWT。
- 删除 `mc:user.{userId}`。
- 写入 `jwt:blacklist:{prefix}:{jti}`，TTL 使用 `SIMPLE_JWT_REFRESH_TTL`。
- 黑名单 key 会按 PHP Doctrine RedisCache 形态写成 `[jwt:blacklist:{prefix}:{jti}][1]`，保证 PHP 也会拒绝 Go logout 后的旧 token。

`permissionByUser` 接管后会：

- 校验 PHP JWT 并读取 `uid`。
- 查询 `mc_user` 的 `tenant_id` 和 `isSuperAdmin`。
- 超级管理员读取全部 `mc_rbac_menu.is_page_menu = 1` 菜单。
- 普通用户读取当前租户下第一个角色、`mc_rbac_role_menu` 和对应菜单。
- 按 PHP 逻辑去掉 `linkUrl` 中的 `/dashboard`，并递归输出菜单树。

`role/select` 接管后会：

- 校验 PHP JWT 并读取当前用户 `tenantId`。
- 查询当前租户 `mc_rbac_role` 未删除角色。
- 输出 `{roleId,name}` 数组，用于子账户和角色选择下拉。

`role/index`、`role/show`、`role/permissionShow`、`role/showEmployee` 接管后会：

- 校验 PHP JWT 并读取当前登录企业上下文。
- 复用 `RBACResolver` 校验 PHP `PermissionMiddleware` 使用的 `path#method` 权限键。
- `role/index` 按租户、角色名和分页返回角色列表，并计算当前企业下每个角色的员工数。
- `role/show` 按租户返回角色详情，并从 `data_permission` JSON 中提取当前企业的数据权限。
- `role/permissionShow` 输出启用菜单权限树，并按 PHP 规则标记未选、全选、半选。
- `role/showEmployee` 输出角色下员工分页列表和部门名称。
- `role/store`、`role/update`、`role/statusUpdate`、`role/destroy` 和 `role/permissionStore` 已接管到 Go；这些是状态变更接口，不能搭配 `MOCHAT_GO_DEV_AUTH_HEADER=1` 或 `MOCHAT_GO_SKIP_JWT_BLACKLIST=1`。

`menu/iconIndex`、`menu/select` 接管后会：

- 校验 PHP JWT 并确认当前用户存在。
- `menu/iconIndex` 按菜单 `id ASC` 返回所有非空 `icon`。
- `menu/select` 按菜单 `id ASC` 读取 `id/name/level/parentId/dataPermission`，并递归输出菜单树。

`menu/index`、`menu/show` 接管后会：

- 校验 PHP JWT 并读取当前登录企业上下文。
- 复用 `RBACResolver` 校验 PHP `PermissionMiddleware` 使用的 `path#method` 权限键。
- `menu/index` 按 PHP `IndexLogic` 的规则构造菜单树，对顶层菜单分页，并输出 `menuId`、`menuPath`、`levelName`、`children`。
- `menu/show` 输出菜单详情，并按 PHP 规则把 `path` 拆为 `firstMenuId`、`secondMenuId`、`thirdMenuId`、`fourthMenuId`。
- 菜单写接口 `store/update/statusUpdate/destroy` 已接管到 Go；这些是状态变更接口，不能搭配 `MOCHAT_GO_DEV_AUTH_HEADER=1` 或 `MOCHAT_GO_SKIP_JWT_BLACKLIST=1`。

`corp/select` 接管后会：

- 校验 PHP JWT 并读取 `uid`。
- 超级管理员按当前用户 `tenant_id` 返回租户内企业。
- 普通用户按 `mc_work_employee.log_user_id` 返回归属企业。
- 支持 `corpName` 企业名称过滤。

`corp/bind` 接管后会：

- 校验 PHP JWT 并读取 `uid`。
- 超级管理员直接写入 `mc:user.{userId}={corpId}-0`。
- 普通用户先校验企业归属，再写入 `mc:user.{userId}={corpId}-{workEmployeeId}`。

`corp/index`、`corp/show`、`corp/store`、`corp/update` 接管后会：

- 校验 PHP JWT 并读取 `uid`。
- 复用 `RBACResolver` 校验 PHP `PermissionMiddleware` 使用的 `path#method` 权限键。
- `corp/index` 按租户、企业名、分页和当前用户企业范围返回 PHP 兼容的 `page/list` 数据。
- `corp/show` 返回企业授权详情，并保持 `eventCallback?cid={corpId}` 的 PHP 输出口径。
- `corp/store` 保持 PHP 版单企业限制，调用企业微信通讯录和外部联系人接口校验 `wxCorpId`、`employeeSecret`、`contactSecret`，写入 `mc_corp` 的回调地址、token、encodingAESKey，并设置 `mc:user.{userId}={corpId}-0`。创建成功后会向 Redis list `mochat-go:employee-apply` 写入 `EmployeeApply` 通讯录同步任务。
- `corp/update` 只更新 PHP 页面允许编辑的 `corpName`、`wxCorpId`、`employeeSecret`、`contactSecret`。
- `corp/store` 和 `corp/update` 是写接口，不能搭配 `MOCHAT_GO_DEV_AUTH_HEADER=1` 或 `MOCHAT_GO_SKIP_JWT_BLACKLIST=1`。

设置 `MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER=1` 后，Go 进程会启动 `EmployeeApply` Redis 消费 worker，消费 `mochat-go:employee-apply` 后执行部门和成员同步，替代 PHP `EmployeeApply` 队列的第一批核心行为。

`weWork/callback` 和 `dashboard/corp/weWorkCallback` 接管后会：

- 按企业微信回调协议校验 `msg_signature`、`timestamp`、`nonce` 和密文。
- 使用 `mc_corp.token`、`mc_corp.encoding_aes_key` 解密 `echostr` 或 POST XML。
- `GET` 返回明文 `echostr`，用于企业微信后台 URL 校验。
- `POST` 解密事件 XML，生成 PHP 队列服务同口径的 `MsgType.Event.ChangeType/EventKey` 事件路径，并写入 Redis list `mochat-go:wework-callback`。
- 设置 `MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER=1` 后，Go 进程会启动 Redis 消费 worker，对通讯录、外部联系人、客户标签和客户群事件触发 Go 侧同步，并对客户和客户群删除类事件按事件主键软删除本地关系。
- 当前 worker 已从第一批全量同步继续拆到细粒度 listener 迁移：Redis 消费具备 processing 队列、成功 ack、失败重试、dead-letter 和 processing 超时恢复，并已纳入 `internal/taskrunner` 统一后台任务运行器；Go 进程启用任意 worker/cron 时会维护 `mochat_go_background_tasks` 持久化状态表，记录任务状态、启动/停止时间、最近错误、启动次数和失败次数，同时维护 `mochat_go_background_task_runs` 执行实例表，用 `run_id` 记录每次后台任务启动到停止/失败的状态。`Periodic` 定时任务会把每次 cron tick 写入 `mochat_go_background_task_executions`，记录 `periodic_tick` 的开始、结束、耗时和失败原因；`employee-apply`、`wework-callback` 和 `contact-welcome` Redis worker 会把每次 delivery 处理写入同表的 `queue_item`，并带上可解析的 `tenant_id`，记录成功、失败、耗时和失败原因，用于 `async_executions` SaaS 用量重算；queue item 结束后会刷新 `mochat_go_saas_usage_counters.async_executions`，超额时写入日志、`mochat_go_saas_alerts` 告警账本，并可按 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL` 发送 `saas.quota_alert` webhook，不阻断企微回调处理。Go 队列已具备第一版 payload 注册表和 10 分钟 Redis 幂等 key，企微回调幂等键按企业、事件路径、事件业务主键和 `CreateTime` 生成，欢迎语队列按企业、客户、员工和 `welcome_code` 幂等；新入队任务会写入带 `queue/payloadType/idempotencyKey/enqueuedAt` 的 envelope，同时继续兼容旧 raw JSON。企微客户群 `change_external_chat.create/update/dismiss` 已按 PHP listener 口径细粒度处理：创建/更新按 `ChatId` 拉单群详情并 upsert 单群，解散按 `ChatId` 软删，不再对这些事件触发全量客户群同步；创建/更新事件同步本地客户群成员后，会目标级触发 `targetAnchor=room_join` 的群 SOP 日志生成。通讯录 `change_contact.create_user/update_user/delete_user/create_party/update_party/delete_party` 已完成第一批细粒度处理：成员创建/更新按 `UserID` 拉单员工详情并 upsert 员工、子账户和员工部门关系，成员删除按 `UserID` 软删员工、禁用同手机号子账户并软删员工部门关系，部门创建/更新/删除按 `Id` upsert 或软删 `mc_work_department`。外部联系人客户 `change_external_contact.add_external_contact/edit_external_contact/del_external_contact/del_follow_user` 已完成第一批细粒度处理：新增按 `UserID + ExternalUserID` 拉单客户详情并创建或恢复客户、客户员工关系和客户标签 pivot，并已接通个人 SOP 日志触发、渠道码专属欢迎语、自动拉群欢迎语和通用欢迎语 listener；`State=channelCode-{id}` 时优先读取 `mc_channel_code.welcome_message`，按特殊时期、周期、通用规则生成 `contact-welcome` 队列任务，同时读取 `mc_channel_code.tags` 复用本地 `mc_work_contact_tag_pivot.type=1`、客户互动轨迹和企业微信 `externalcontact/mark_tag` 打标签链路；`State=workRoomAutoPullId-{id}` 时读取 `mc_work_room_auto_pull.leading_words/rooms`，按配置人数上限和 `mc_work_room.room_max` 跳过满群并发送首个可用群二维码图片，同时读取 `mc_work_room_auto_pull.tags` 复用同一套打标签链路；`State=fission-{id}` 时兼容上游 listener 差异：打标签按活动 ID 读取 `mc_work_fission.contact_tags`，欢迎语和助力按上级参与人 ID 读取 `mc_work_fission_contact` 与 `mc_work_fission_welcome`，生成裂变图文欢迎语，并在事务内创建或恢复子参与人、按新客规则更新上级 `invite_count/level/status`，任务完成且 `push_employee=1` 时通过企业应用给服务员工发送文本提醒，`push_contact=1` 时通过企业微信 `externalcontact/add_msg_template` 给完成任务的上级客户创建客户群发消息；删除裂变参与客户时会按 unionid/external_userid 标记 `mc_work_fission_contact.loss=1`，并对上级 `invite_count` 做非负回退；未匹配时再按指定成员欢迎语优先、全员欢迎语兜底，使用 `contact:welcome_status:{contactId}` 进行 60 秒去重，发送文本、图片、图文和小程序附件；编辑只更新已有客户关系；成员删除客户写入 `status=2` 并软删指定关系；客户删除成员写入 `status=3` 并软删指定关系，且会按 PHP 文案通过企业应用发送“删除提醒”或“流失提醒”，无剩余跟进关系时软删客户本体。外部联系人标签 `change_external_tag.create/update/delete` 已完成第一批细粒度处理：按 `tag_id/group_id` 拉取目标标签片段并 upsert，删除时按微信 ID 软删标签或标签组并同步软删客户标签 pivot。PHP 原项目仅映射、未绑定 listener 的 `change_external_contact.add_half_external_contact/transfer_fail`、`change_external_tag.shuffle` 和 `change_contact.update_tag` 在 Go worker 中按 no-op 兼容，仍会完成队列 ack 和执行记录。
- 补充：`async-file-upload` 已支持结构化 `tenantId/corpId` 写入租户执行量，并在写入 Go storage root 后按实际落盘字节写入 `mochat_go_saas_storage_objects`、刷新 `storage_mb`；`employee-statistic-apply` 继续兼容 PHP 空数组 payload，并支持结构化 `corpId/tenantId` 限定统计企业范围、写入带租户归属的 `queue_item` 执行历史和刷新 `async_executions`。

`corpData/index`、`corpData/lineChat` 接管后会：

- 校验 PHP JWT，并按 `mc:user.{userId}` 或用户首个员工归属解析当前企业。
- 要求当前登录企业唯一；无唯一企业时按 PHP 行为返回“请先选择企业”。
- `corpData/index` 读取 `mc_corp_day_data`、`mc_work_contact_employee`、`mc_work_room`、`mc_work_contact_room`、`mc_work_employee` 和 `mc_work_update_time`，输出首页统计卡片、当日/昨日/月度指标和最后更新时间。
- `corpData/lineChat` 读取最近 32 天内的企业日数据，按日期升序输出折线图字段。
- 设置 `MOCHAT_GO_ENABLE_CORP_DATA_CRON=1` 后，Go 进程会启动 `cron-corp-data` 后台任务，每 10 分钟按 PHP `CorpDataLogic` 口径刷新所有企业当日新增客户、新增社群、新增入群、流失客户和退群人数，并写入 `mc_work_update_time.type=6`。

`workEmployee/index`、`workEmployee/searchCondition`、`workEmployee/synEmployee`、`workDepartment/index`、`workEmployeeDepartment/memberIndex`、`workDepartment/memberIndex`、`workDepartment/selectByPhone`、`workDepartment/pageIndex`、`workDepartment/showEmployee` 接管后会：

- 校验 PHP JWT，并按 `mc:user.{userId}` 或用户首个员工归属解析当前企业。
- `workEmployee/index` 接入 RBAC resolver，按企业、数据权限、员工名、状态、外部联系人权限分页读取员工，补齐统计字段、状态名、性别名、头像 URL 和外部联系人权限名。
- `workEmployee/searchCondition` 输出员工状态、外部联系人权限枚举，以及 `mc_work_update_time.type=1` 的员工同步时间。
- `workEmployee/synEmployee` 对齐 PHP 只要求 dashboard 登录态、不走菜单 RBAC；按超管租户或普通用户绑定员工解析可访问企业，调用企业微信 `department/list`、`user/list` 和 `externalcontact/get_follow_user_list`，在事务内同步 `mc_work_department`、`mc_work_employee`、`mc_user` 子账户、`mc_work_employee_department` 关系和 `mc_work_update_time.type=1`。
- `workDepartment/index` 要求当前登录企业唯一；读取 `mc_work_department` 构造 `son` 子部门树，读取当前企业已激活员工并补 `employeeId`。
- `workEmployeeDepartment/memberIndex` 要求 `departmentIds`；读取 `mc_work_employee_department`、`mc_work_department` 和已激活 `mc_work_employee`，返回部门成员的员工 ID、部门 ID、部门名和员工名；`workDepartment/memberIndex` 是旧前端别名，复用同一 handler。
- `workDepartment/selectByPhone` 要求 11 位手机号；读取当前企业手机号匹配的员工和员工部门关系，返回用户新增/编辑页需要的部门下拉项。
- `workDepartment/pageIndex` 接入 RBAC resolver，要求当前登录企业唯一；读取部门表后输出分页部门树、部门层级名称和 `departmentPath`。
- `workDepartment/showEmployee` 接入 RBAC resolver，要求 `departmentId`；按部门员工关系分页读取员工并补齐角色名。

`workContactTagGroup/index`、`workContactTagGroup/detail` 接管后会：

- 校验 PHP JWT，并按当前登录用户的企业集合读取 `mc_work_contact_tag_group`。
- `workContactTagGroup/index` 按 PHP 行为输出 `{groupId,groupName}` 列表；有分组时追加 `groupId=0` 的“未分组”。
- `workContactTagGroup/detail` 要求 `groupId`；按 PHP 行为返回 `{id,groupName}`，不存在时返回空数组。
- `sidebar/workContactTagGroup/index` 使用 sidebar 员工 token 的企业上下文读取标签分组并追加“未分组”，不走 dashboard RBAC。

`workContactTag/index`、`workContactTag/detail`、`workContactTag/contactTagList`、`workContactTag/allTag` 接管后会：

- 校验 PHP JWT，并按当前登录用户的企业集合读取 `mc_work_contact_tag`。
- `workContactTag/index` 按分组过滤、`updated_at desc` 分页输出标签列表、客户数和 `mc_work_update_time.type=3` 的标签同步时间。
- `workContactTag/detail` 要求 `tagId`；按 PHP 行为返回 `{tagId,tagName,groupId}`，不存在时返回空数组。
- `workContactTag/contactTagList` 按当前用户企业集合读取标签分组及分组下标签；可选 `name` 按标签名 `LIKE` 过滤，输出 `{id,wxGroupId,groupName,tags}`。
- `workContactTag/allTag` 可选 `groupId`；返回当前用户企业集合下的 `{id,name}` 标签数组，用于标签下拉。
- `workContactTag/synContactTag` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContactTag/synContactTag#put`；要求已选择单企业，调用企业微信 `externalcontact/get_corp_tag_list` 拉取企业标签库，在同一事务内新增/更新/软删 `mc_work_contact_tag_group` 和 `mc_work_contact_tag`，同步软删失效标签 pivot，并更新 `mc_work_update_time.type=3`。
- `workContact/synContact` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContact/synContact#put`；要求已选择单企业，读取当前企业有 `wx_user_id` 的成员，调用企业微信 `externalcontact/list` 和 `externalcontact/get` 拉取跟进客户，在事务内同步 `mc_work_contact`、`mc_work_contact_employee`、企业标签、客户标签 pivot，并更新 `mc_work_update_time.type=2`。企业微信返回 `84061` 时会按 PHP 行为把该成员的客户关系标记为删除。
- `workContact/source` 对齐 PHP `AddWay::$Enum`，返回 `{addWay,addWayText}` 客户来源枚举。
- `dashboard/workContact/show` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContact/show#get`，按 `contactId` 和 `employeeId` 读取客户基本信息、员工备注/描述、客户标签、所在客户群和归属员工企业名称，并补头像完整 URL。
- `dashboard/workContact/update` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContact/update#put`，按 PHP 行为更新客户备注、描述、客户编号；标签只新增缺失的 `mc_work_contact_tag_pivot`，不删除旧标签；同时写入互动轨迹，并通过企业微信 `externalcontact/remark`、`externalcontact/mark_tag` 同步备注描述和新增标签。
- `dashboard/workContact/batchLabeling` 使用 dashboard token 的当前员工上下文，按逗号分隔 `contactId`/`tagId` 批量补写缺失的 `mc_work_contact_tag_pivot`，已有客户-标签组合不重复插入。
- `dashboard/workRoom/syn` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workRoom/syn#put`；要求已选择单企业，调用企业微信 `externalcontact/groupchat/list` 和 `externalcontact/groupchat/get` 拉取客户群列表及详情，在事务内新增/更新/软删 `mc_work_room`，并新增/更新/退群 `mc_work_contact_room` 成员关系。群状态按 PHP 行为取自群列表，外部联系人成员只映射已有 `mc_work_contact`。
- `dashboard/workRoom/batchUpdate` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workRoom/batchUpdate#put`；要求已选择单企业，`workRoomGroupId=0` 时移出分组，非 0 时校验分组存在且归属当前企业，然后按逗号分隔 `workRoomIds` 批量更新当前企业客户群的 `room_group_id`。
- `sidebar/workContactTag/allTag` 使用 sidebar 员工 token 的企业上下文读取 `{id,name}` 标签数组，不走 dashboard RBAC。
- `sidebar/workContact/detail` 使用 sidebar 员工 token 做身份校验，按 PHP 行为通过 `wxExternalUserid` 读取 `mc_work_contact` 的客户详情并补头像完整 URL，不额外套 dashboard RBAC。
- `sidebar/workContact/show` 使用 sidebar 员工 token 做身份校验，按 `contactId` 读取客户基本信息、员工备注/描述、客户标签、所在客户群和归属员工企业名称，并补头像完整 URL。
- `sidebar/workContact/track` 使用 sidebar 员工 token 做身份校验，按 `contactId` 读取 `mc_contact_employee_track`，按 `created_at desc` 输出互动轨迹。
- `sidebar/workContact/update` 使用 sidebar 员工 token 做身份校验，忽略请求体中的 `employeeId`，强制使用当前侧边栏员工 ID 更新备注、描述、客户编号和新增标签，并同步企业微信。
- `sidebar/contactProcessStatus/index` 使用 sidebar 员工 token 的企业上下文读取 `mc_contact_process`；若当前企业没有跟进状态，按 PHP 行为创建“新客户/初步沟通/意向客户/付款客户/无意向客户”五个默认项后再返回。
- `sidebar/contactProcessStatus/update` 使用 sidebar 员工 token 做身份校验，按 PHP 行为记录“编辑用户跟进状态：{状态名}”轨迹，并在同一事务内更新 `mc_work_contact.follow_up_status`。
- `dashboard/contactBatchAdd/index/importIndex/importStore/allot/dataStatistic/destroy/importDestroy/settingEdit/settingUpdate/remind` 使用 dashboard token 和 RBAC resolver 接管批量加好友后台链路；列表、导入记录、设置和统计直接读取 MySQL，导入支持 JSON/表单/CSV/简单 XLSX 手机号解析，multipart 导入会把原文件写入 `MOCHAT_FILE_STORAGE_ROOT`、回填 `fileUrl`、记录 `mochat_go_saas_storage_objects` 并刷新 `storage_mb`，`importDestroy` 会软删导入记录和明细并回收对应原文件账本；分配会轮询写入成员、递增分配次数并记录 `mc_contact_batch_add_allot`，删除使用软删除，`0009_contact_batch_add_rbac` 补齐普通管理员所需菜单和接口权限。
- `dashboard/sensitiveWord/index/store/destroy/statusUpdate/move`、`dashboard/sensitiveWordGroup/select/store/update` 和 `dashboard/sensitiveWordsMonitor/index/show` 使用 dashboard token 和 RBAC resolver 接管敏感词词库与监控第一批后台链路；`0010_sensitive_words` 会创建分组、词库和触发监控表，词库列表按监控记录聚合员工/客户触发次数，新增支持中文逗号、顿号、换行和英文逗号批量拆词，删除使用软删除。新增 Go 原生 `/dashboard/sensitiveWords/page` 页面，使用 dashboard JWT 直接管理分组、词库和触发监控，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_sensitive_word_dashboard.sh` 已用真实 Go standalone 覆盖敏感词分组批量新建、更新、下拉选择，词库批量拆词去重、新增、移动、状态更新、删除、监控列表、监控详情会话展开和 `sensitive_words` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增敏感词会按 `sensitive_words` SaaS 套餐额度拦截，创建和删除后会刷新对应用量。`MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=1` 会启动 Go 内置 `cron-work-message-archive-sync`，从 `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL` 指向的官方 SDK bridge 拉取已解密会话消息，按 `seq` 分表写入 `mc_work_message_1` 至 `mc_work_message_10`，并用 `mc_work_message_id.type=40` 维护拉取游标；`MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON=1` 会启动 Go 内置 `cron-sensitive-word-monitor`，扫描 10 张会话存档分表的新增消息，按启用敏感词写入 `mc_sensitive_words_monitor`，并用 `mc_work_message_id.type=21..30` 维护分表游标；`scripts/smoke_work_message_archive_sync_cron.sh` 会用 fake SDK bridge + 真实 MySQL 验证会话同步、分表入库、游标推进、幂等和敏感词消费，`scripts/smoke_sensitive_word_monitor_cron.sh` 会继续验证监控扫描本身。
- `dashboard/contactSop/index/store/setEmployee/state/info/destroy/update` 和 `dashboard/roomSop/index/store/setRoom/state/info/destroy/update` 使用 dashboard token 和 RBAC resolver 接管个人 SOP 与群 SOP 管理端第一批链路；规则名称、推送设置、员工/客户/群范围按原表 JSON 字段保存，删除会硬删无 `deleted_at` 的 SOP 主表和触达日志，`0011_sop_rbac` 确保 SOP 主表/触达日志表存在并补齐普通管理员所需接口权限。新增 Go 原生 `/dashboard/contactSop/page` 与 `/dashboard/roomSop/page` 页面，使用 dashboard JWT 直接管理个人 SOP 规则、员工范围、客户范围、群 SOP 规则、客户群范围和启停状态，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；新增个人/群 SOP 会按 `contact_sops`、`room_sops` SaaS 套餐额度拦截，创建和删除后会刷新对应用量。
- `scripts/smoke_sop_dashboard.sh` 已用真实 Go standalone 覆盖个人/群 SOP 新建、列表、详情、设置员工/群聊、启停、更新、删除、触达日志级联删除和 `contact_sops` / `room_sops` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`。
- `MOCHAT_GO_ENABLE_SOP_LOG_CRON=1` 会启动 Go 内置 `cron-sop-log`，按 `mc_contact_sop` / `mc_room_sop` 中启用的规则解析 `setting`、员工/客户/群 JSON 范围，向 `mc_contact_sop_log` 和 `mc_room_sop_log` 幂等生成已到期的个人 SOP 和群 SOP 提醒；规则支持绝对提醒时间，也支持 `baseTime` / `startTime` / `anchorTime` / `createdAt` 等任务内锚点叠加 `delayMinutes`、`delayHours`、`delayDays`、`delay`、`after` 等相对延迟字段；纯相对延迟规则没有任务内锚点时，会按个人 SOP 的客户添加时间 `mc_work_contact_employee.create_time` 或群 SOP 的客户群创建时间 `mc_work_room.create_time` 逐目标计算到期状态；群 SOP 纯相对延迟可通过 `targetAnchor` / `target_anchor` / `anchor` / `event` / `trigger` 配置 `room_join`、`customer_join_room`、`客户入群` 等值，按 `mc_work_contact_room.join_time` 为每个外部联系人入群记录生成 contact 级群 SOP 日志，并把 `mc_room_sop_log.contact` 纳入幂等条件；周期规则支持 `cycle` / `period` / `repeat` / `frequency` 的 `daily`、`weekly`、`monthly`，并可用 `weekdays`、`monthDays`、`repeatDates` 等字段限定触发日期，生成日志时会写入内部 `_mochatGoOccurrence`，确保同一周期幂等且不同日期可再次触达；同一套解析和幂等逻辑也会被 `wework-callback` worker 在 `change_external_contact.add_external_contact` 后按员工/客户目标触发个人 SOP，在 `change_external_chat.create/update` 后按 `ChatId` 触发 `room_join` 群 SOP；可用 `MOCHAT_GO_SOP_LOG_CRON_INTERVAL_SECONDS` 和 `MOCHAT_GO_SOP_LOG_CRON_RUN_ON_START` 控制周期和启动即跑。
- `dashboard/shopCode/location/addressKeyWordList/store/update/destroy/info/status/index/searchCity/share/pageInfo/pageSet/show/showContact/showShop/updateEmployee/updateQrcode/batchContactTags` 使用 dashboard token 和 RBAC resolver 接管门店活码后台链路；`0012_shop_code` 会创建 `mc_shop_code`、`mc_shop_code_page`、`mc_shop_code_record` 并补齐门店活码页面菜单和 18 个接口权限，门店、页面设置、地址检索、分享链接和统计读取均由 Go 独立服务承接；新增 Go 原生 `/dashboard/shopCode/page` 页面，使用 dashboard JWT 直接管理门店、页面设置、分享链接、地址/城市建议、访问客户和门店统计，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；新增门店活码会按 `shop_codes` SaaS 套餐额度拦截，创建和删除后会刷新对应用量；更新、删除门店活码或通过 `pageSet` 替换页面设置时，会回收 `employee_qrcode`、`qw_code` 活码 JSON 和 `mc_shop_code_page.default` / `poster` 中的本地二维码/海报账本并刷新 `storage_mb`。
- `dashboard/radar/store/update/index/destroy/info/storeChannel/storeChannelLink/indexChannel/indexChannelLink/show/showContact/showChannel/radarArticle` 使用 dashboard token 和 RBAC resolver 接管互动雷达后台链路；`0013_radar` 会创建 `mc_radar`、`mc_radar_channel`、`mc_radar_channel_link`、`mc_radar_record` 并补齐互动雷达页面菜单和 13 个接口权限，雷达 CRUD、渠道、渠道链接、分享链接和点击统计均由 Go 独立服务承接；新增 Go 原生 `/dashboard/radar/page` 页面，使用 dashboard JWT 直接管理雷达素材、渠道、渠道链接和客户点击明细，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_radar_dashboard.sh` 已用真实 Go standalone 覆盖雷达新建、列表、详情、渠道、渠道链接、客户点击明细、渠道统计、文章元数据回显、更新、删除和 `radars` SaaS 用量刷新；新增互动雷达会按 `radars` SaaS 套餐额度拦截，创建和删除后会刷新对应用量；更新或删除雷达会回收 `link_cover`、`pdf` 指向的本地上传文件账本并刷新 `storage_mb`。
- `dashboard/autoTag/store/index/destroy/onOff/show/showContactKeyWord/showContactRoom/showContactTime`、`Task/AutoTag/KeyWordTag`、`dashboard/Task/AutoTag/KeyWordTag`、`dashboard/workMessage/fromUsers/toUsers/index` 和 `dashboard/workMessageConfig/corpStore/corpShow/corpIndex/stepCreate/stepUpdate` 使用 dashboard token 和 RBAC resolver 接管自动标签与消息存档后台链路；`0014_auto_tag` 会创建 `mc_auto_tag`、`mc_auto_tag_record`、`mc_work_message_id`、`mc_work_message_1` 至 `mc_work_message_10`，并补齐 `mc_corp` 会话存档配置字段、自动标签和消息存档页面菜单及接口权限，`0022_work_message_archive_sync` 会把会话存档游标扩展为 bigint 并增加消息分表同步索引，规则 CRUD、触发记录、会话列表、会话存档配置和会话同步入库均由 Go 独立服务承接。关键词任务会扫描会话存档消息、按员工范围和精确/模糊关键词命中规则、写入待打标签记录、入队 `mark-tags`，并由 worker 回写记录状态和标签统计；入群行为自动标签已接入企微 `change_external_chat.create/update` 事件，单群同步后按 `mc_auto_tag.type=2` 的 `tag_rule.rooms/tags` 匹配外部联系人成员，幂等写入待打标签记录并入队 `mark-tags`；分时段自动标签已接入企微 `change_external_contact.add_external_contact` 事件，客户关系同步后按 `mc_auto_tag.type=3` 的成员范围和时间段规则匹配客户添加时间，幂等写入待打标签记录并入队 `mark-tags`；`scripts/smoke_auto_tag_dashboard.sh` 已用真实 Go standalone 覆盖三类自动标签规则新建、列表、详情、触发记录、启停、删除、会话成员筛选、会话列表和会话存档配置读写，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；`scripts/smoke_auto_tag_keyword_task.sh` 覆盖新消息触发和 pending 记录重投递，`scripts/smoke_wework_callback_worker.sh` 覆盖新增客户分时段自动标签、客户群入群自动标签和 `mark-tags` 入队。
- `dashboard/lottery/index/store/showContact/show/destroy/share/update/info/writeOff/batchContactTags` 使用 dashboard token 和 RBAC resolver 接管抽奖活动后台链路；`0015_lottery` 会创建 `mc_lottery`、`mc_lottery_contact`、`mc_lottery_contact_record`、`mc_lottery_prize` 并补齐抽奖活动页面菜单和 10 个接口权限，活动 CRUD、客户列表、分享链接、核销和批量打标签均由 Go 独立服务承接；新增 Go 原生 `/dashboard/lottery/page` 页面，使用 dashboard JWT 直接管理抽奖活动、奖品详情、分享链接和中奖客户，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_lottery_dashboard.sh` 已用真实 Go standalone 覆盖抽奖活动新建、列表、详情、分享、中奖客户、核销、批量打标签、更新、删除、级联软删和 `lotteries` SaaS 用量刷新；新增抽奖活动会按 `lotteries` SaaS 套餐额度拦截，创建和删除后会刷新对应用量；更新或删除抽奖活动会回收 `prize_set`、`exchange_set`、`draw_set`、`win_set`、`corp_card` 和中奖记录 `receive_qr` 中的本地上传文件账本并刷新 `storage_mb`。
- `dashboard/roomFission/index/store/info/update/destroy/invite/show/showRoom/showContact/writeOff` 使用 dashboard token 和 RBAC resolver 接管群裂变后台链路；`0016_room_fission` 会创建 `mc_room_fission`、`mc_room_fission_contact`、`mc_room_fission_invite`、`mc_room_fission_poster`、`mc_room_fission_room`、`mc_room_fission_welcome` 并补齐群裂变页面菜单和 10 个接口权限，活动 CRUD、四步编辑数据、邀请配置、群聊数据、客户数据、分享链接和核销均由 Go 独立服务承接；新增 Go 原生 `/dashboard/roomFission/page` 页面，使用 dashboard JWT 直接管理群裂变活动、海报欢迎语、邀请配置、群聊数据、参与客户和核销状态，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_room_fission_dashboard.sh` 已用真实 Go standalone 覆盖群裂变新建、列表、详情、群聊、客户、邀请、更新、核销、删除、级联软删和 `room_fissions` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增群裂变会按 `room_fissions` SaaS 套餐额度拦截，创建和删除后会刷新对应用量。
- `dashboard/roomClockIn/index/store/update/destroy/show/showContact/batchContactTags/info/dayDetail` 使用 dashboard token 和 RBAC resolver 接管群打卡后台链路；`0017_room_clock_in` 会创建 `mc_room_clock_in`、`mc_room_clock_in_contact`、`mc_room_clock_in_record` 并补齐群打卡页面菜单和 9 个接口权限，活动 CRUD、客户列表、打卡天数明细和批量打标签均由 Go 独立服务承接；新增 Go 原生 `/dashboard/roomClockIn/page` 页面，使用 dashboard JWT 直接管理群打卡活动、参与客户和打卡明细，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_room_clock_in_dashboard.sh` 已用真实 Go standalone 覆盖群打卡新建、列表、详情、客户列表、天数明细、批量打标签、更新、停用状态、删除、级联软删和 `room_clock_ins` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增群打卡会按 `room_clock_ins` SaaS 套餐额度拦截，创建和删除后会刷新对应用量；更新或删除群打卡会回收 `employee_qrcode` 指向的本地上传文件账本并刷新 `storage_mb`。
- `dashboard/roomQuality/index/store/status/info/update/showContact/destroy/contactDetail` 使用 dashboard token 和 RBAC resolver 接管群质检后台链路；`0018_room_quality` 会创建 `mc_room_quality`、`mc_room_quality_contact` 并补齐群质检页面菜单和 8 个接口权限，规则 CRUD、启停、触发客户列表和客户触发详情均由 Go 独立服务承接；新增 Go 原生 `/dashboard/roomQuality/page` 页面，使用 dashboard JWT 直接管理质检规则、启停和触发客户记录，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_room_quality_dashboard.sh` 已用真实 Go standalone 覆盖群质检新建、列表、弹窗详情、触发客户列表、触发客户详情、启停、更新、删除、触发记录级联软删和 `room_qualities` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增群质检规则会按 `room_qualities` SaaS 套餐额度拦截，创建和删除后会刷新对应用量。
- `dashboard/roomCalendar/index/addRoom/destroyRoom/store/destroy/show/update` 使用 dashboard token 和 RBAC resolver 接管群日历后台链路；`0019_room_calendar` 会创建 `mc_room_calendar`、`mc_room_calendar_push`、`mc_room_calendar_record` 并补齐群日历页面菜单和 7 个接口权限，日历 CRUD、群聊增删和推送内容保存均由 Go 独立服务承接；新增 Go 原生 `/dashboard/roomCalendar/page` 页面，使用 dashboard JWT 直接管理群日历列表、启停和推送计划，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_room_calendar_dashboard.sh` 已用真实 Go standalone 覆盖群日历新建、列表、详情、增加群聊、移除群聊、更新推送计划、删除、推送和记录级联软删、`room_calendar_push.room_calendar_id` 字符串兼容查询和 `room_calendars` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；列表推送数查询已改为数值比较，避免 `mc_room_calendar_push.room_calendar_id` 与 `CAST(c.id AS CHAR)` 在 MariaDB/MySQL 下出现 collation 冲突；新增群日历会按 `room_calendars` SaaS 套餐额度拦截，创建和删除后会刷新对应用量。
- `dashboard/roomRemind/index/destroy/info/status/store/update` 和 `dashboard/task/roomRemind` 使用 dashboard token 接管客户群提醒后台链路；`0020_room_remind` 会创建 `mc_room_remind`、`mc_room_remind_record` 并补齐客户群提醒页面菜单和 6 个接口权限，规则 CRUD、GET 启停、任务查询和触发记录统计均由 Go 独立服务承接；新增 Go 原生 `/dashboard/roomRemind/page` 页面，使用 dashboard JWT 直接管理提醒规则、启停和启用任务视图，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_room_remind_dashboard.sh` 已用真实 Go standalone 覆盖客户群提醒新建、列表、详情、任务视图、启停、更新、删除、提醒记录级联软删和 `room_reminds` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增客户群提醒会按 `room_reminds` SaaS 套餐额度拦截，创建和删除后会刷新对应用量。
- `dashboard/roomInfinitePull/index/info/update/destroy/store` 使用 dashboard token 和 RBAC resolver 接管无限拉群后台链路；`0021_room_infinite_pull` 会创建 `mc_room_infinite` 并补齐无限拉群页面菜单和 5 个接口权限，活动 CRUD、企微活码 JSON、扫码人数和详情二维码链接均由 Go 独立服务承接；新增 Go 原生 `/dashboard/roomInfinitePull/page` 页面，使用 dashboard JWT 直接管理无限拉群活动、群名称/引导语显示配置和企微活码 JSON，不依赖原 MoChat 前端源码或 dashboard dist 中的旧页面；`scripts/smoke_room_infinite_pull_dashboard.sh` 已用真实 Go standalone 覆盖无限拉群新建、列表、详情、更新、删除、企微活码 JSON、详情链接和 `room_infinite_pulls` SaaS 用量刷新，并已接入 `MOCHAT_ACCEPTANCE_SUITE=frontend`；新增无限拉群会按 `room_infinite_pulls` SaaS 套餐额度拦截，创建和删除后会刷新对应用量；更新或删除无限拉群会回收 `avatar`、`logo` 和 `qw_code` 活码 JSON 二维码字段指向的本地上传文件账本并刷新 `storage_mb`。2026-07-05 复算 dashboard 前端源码 API 去重口径已达到 `316/316`，`missing=0`；旧前端别名 `dashboard/clockIn/index`、`dashboard/workDepartment/memberIndex`、预览接口 `dashboard/contactMessageBatchSend/messageShow` 和自动拉群兼容 `dashboard/workRoomAutoPull/move` 已纳入 Go 接管清单。
- `sidebar/contactBatchAdd/detail` 使用 sidebar 员工 token 读取当前员工被分配的批量加好友导入手机号，`batchId` 对应 `mc_contact_batch_add_import.record_id`，`status=4` 返回全部，`0/1/2/3` 分别返回待分配、待添加、待通过、已添加。
- `sidebar/contactSop/getSopInfo` 和 `sidebar/contactSop/getSopTipInfo` 使用 sidebar 员工 token 读取当前员工的个人 SOP 提醒日志，补齐客户信息和本地素材完整 URL；`scripts/smoke_sidebar_frontend_contact.sh` 会同时验证客户页个人 SOP 提醒弹窗和 `/contactSop?id=900001&agentId=1` 详情页渲染，内置 sidebar dist 也已补齐个人 SOP 详情页初始空 `task/contact` 结构，避免页面加载期出现 `task.content` 运行时错误。
- `sidebar/roomSop/getSopInfo` 和 `sidebar/roomSop/logState` 使用 sidebar 员工 token 读取并完成当前员工的群 SOP 提醒日志；`getSopInfo` 输出前端 `roomSop` 页面需要的创建人、提醒时间、群聊信息、任务内容和完成状态，`logState` 会在 `mc_room_sop_log` 中把当前员工对应日志标记为已完成。
- `dashboard/workContact/index` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContact/index#get`，按当前企业、数据权限、客户名、备注、画像、性别、来源、所在群、持群数、所属员工、添加时间和客户编号分页读取客户列表，并补齐客户资料、群名、归属成员、标签、同步时间和当前用户是否持有该客户。
- `dashboard/workContact/lossContact` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContact/lossContact#get`，按当前企业、数据权限和所属员工分页读取已删除/被动删除客户关系，包含软删除客户关系、软删除联系人和软删除标签 pivot，并补齐客户头像、客户名、员工企业名、员工备注和标签。
- `dashboard/channelCode/index/show/contact/statistics/statisticsIndex/store/update` 使用 dashboard token 和 RBAC resolver 校验对应权限，按 PHP 行为读取渠道活码列表、详情、扫码客户、统计折线和统计分页；新增/编辑会写入 `mc_channel_code`、`mc_business_log`，调用企业微信联系我二维码接口，并回写 `qrcode_url` / `wx_config_id`。渠道码创建或定时刷新 contact_way 失败触发软删回滚时，会同步刷新 `channel_codes` SaaS 用量，避免失败记录继续占用套餐额度；创建失败回滚还会解析 `welcome_message.messageDetail` 中的本地欢迎语素材路径，回收 `mochat_go_saas_storage_objects` 并刷新 `storage_mb`。
- 客户群发、客户群群发、标签建群、自动拉群和裂变活动删除或失败回滚后，会刷新对应 SaaS 业务资源用量；`scripts/smoke_saas_storage_reclaim.sh` 会同时验证文件账本回收和这些业务计数下降。
- `dashboard/channelCodeGroup/index/detail/store/update/move` 使用 dashboard token 的企业上下文读写渠道活码分组；列表按 PHP 行为追加 `{groupId:0,name:"未分组"}`，详情缺少 `groupId` 时返回“分组id必传”，创建要求唯一当前企业，更新禁止修改“未分组”，移动会更新 `mc_channel_code.group_id`。
- `sidebar/agent/auth` 和 `sidebar/agent/oauth` 按 PHP 行为生成企业微信 OAuth URL、用 code 换取企微 `userid`，分别签发 sidebar 员工 JWT 和 dashboard 用户 JWT；`sidebar/agent/jssdkConfig` 与 `sidebar/wxJsSdk/config` 会读取企业/应用密钥，获取企微 jsapi ticket 并返回 JSSDK 签名配置。
- `dashboard/workContact/track` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContact/track#get`，复用同一条互动轨迹查询链路。
- `dashboard/workContactRoom/index` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workContactRoom/index#get`，按 `workRoomId` 读取客户群成员、入群/退群统计、入群方式文案、所在其他群和客户员工关系。
- `dashboard/workRoom/index` 使用 dashboard token 和 RBAC resolver 校验 `/dashboard/workRoom/index#get`，按当前企业、数据权限、群主、分组、群名、状态和创建时间分页读取客户群列表，并补齐群主公司名、分组名、成员数、今日入群数和今日退群数。
- `dashboard/workRoom/roomIndex` 使用 dashboard token 和当前用户企业集合，按可选 `name`、`roomGroupId` 返回客户群下拉数据 `roomId/roomName/roomMax/currentNum`。
- `dashboard/workRoom/statistics` 和 `dashboard/workRoom/statisticsIndex` 使用 dashboard token 和 RBAC resolver，按 PHP 的 `type=1/2/3` 规则生成日/周/月统计区间，统计入群、退群、当前成员和累计退群数据。
`contactField/index`、`contactField/show`、`contactField/portrait`、`dashboard/sidebar contactFieldPivot/index` 和 `dashboard/sidebar contactFieldPivot/update` 接管后会：

- 校验 PHP JWT；dashboard `index/show/contactFieldPivot/index` 按当前登录企业上下文复用 `RBACResolver` 校验对应 `path#method` 权限键，`portrait` 和 `contactFieldPivot/update` 对齐 PHP 只做 `DashboardAuthMiddleware`，sidebar `contactFieldPivot/index/update` 对齐 `SidebarAuthMiddleware` 使用员工 token 且不做菜单 RBAC。
- `status=2` 表示全部状态；其他值按 `mc_contact_field.status` 过滤。
- 列表按 PHP 行为使用 ``order`` 倒序分页，详情按 `id` 查询；输出字段 `id/name/label/type/options/status/order/isSys/typeText`。
- 画像下拉只返回展示状态字段，追加“全部”和尾部空数组，并过滤图片类型字段。
- 用户画像详情会把 `mc_contact_field_pivot` 的客户字段值合并到展示字段，多选值按逗号拆成数组，图片值补 `pictureFlag`。
- 用户画像编辑按 PHP 行为解析 `userPortrait` JSON 字符串；多选值转逗号字符串，已有 pivot 更新、无 pivot 新增，字段值变化时写入 `mc_contact_employee_track` 的“编辑用户画像：...”轨迹。
- 详情不存在时返回 PHP 兼容错误文案 `无此条信息`。
- 客户画像字段新增、编辑、批量修改、状态切换和删除已接管；未列出的客户画像相关页面仍走 PHP fallback。

`chatTool/config` 接管后会：

- 校验 PHP JWT 并按 `mc:user.{userId}` 或用户首个员工归属解析当前企业。
- 读取当前企业未停用的 `mc_work_agent`，以及启用的 `mc_chat_tool`。
- 生成 `pageUrl={SIDEBAR_BASE_URL}/{pageFlag}?agentId={agentId}`，并保持 PHP 的 `customer -> contact`、`mediumGroup -> medium` 兼容映射。
- 没有企业应用时返回空数组，保持 PHP 页面用于判断“未配置应用”的数据形状。

`GET /WW_verify_*.txt` 接管后会：

- 仅匹配 `/WW_verify_` 加 16 位字母数字验证码的 `.txt` 路径。
- 直接返回验证码纯文本；不匹配的路径只有在显式设置 `MOCHAT_PHP_UPSTREAM` 时才会 fallback 到 PHP。

`txtVerifyUpload` 接管后会：

- 保持 PHP 的 multipart 字段名 `file`。
- 只接受 `text/plain` 文件。
- 写入 `{MOCHAT_FILE_STORAGE_ROOT}/wx_txt_verify/{filename}`。
- 成功时返回空数组，保持 PHP deprecated 接口的数据形状。

## RBAC 兼容基础设施

`internal/dashboard.RBACResolver` 已复刻 PHP `PermissionMiddleware` 的核心规则，供后续迁移业务接口复用：

- 权限键保持 `path#method`，method 为小写。
- 超级管理员直接通过，数据权限为全企业。
- 普通用户按 `mc_rbac_menu.link_url`、用户角色和角色菜单判断是否授权。
- 数据权限按菜单 `data_permission` 和角色 `data_permission` 的当前企业配置计算。
- 本部门权限会通过 `mc_work_employee_department` 和 `mc_work_department.path` 解析同部门及子部门员工。

该 resolver 目前已用于 `corp/index`、`corp/show`、`corp/update`、`role/index`、`role/show`、`role/permissionShow`、`role/showEmployee`、`menu/index`、`menu/show`、`workEmployee/index`、`workDepartment/pageIndex` 和 `workDepartment/showEmployee`；后续迁移更多业务接口时仍需抽成统一中间件，避免在 handler 内重复接线。

开发期启用示例：

```bash
env -u GOROOT \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_DEV_AUTH_HEADER=1 \
  MOCHAT_MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/mochat?parseTime=true&loc=Local' \
  go run ./cmd/mochat-go
```

请求时需要临时传入：

```bash
X-Mochat-Go-User-ID: 1
```

如果本机没有 Redis、但只想验证 JWT 验签和字段映射，可临时设置 `MOCHAT_GO_SKIP_JWT_BLACKLIST=1`。该模式不会读取 Redis 黑名单，也不会读取 `mc:user.{id}` 企业选择缓存，不能用于生产接管。

`MOCHAT_GO_MIGRATE_LOGOUT=1`、`MOCHAT_GO_MIGRATE_CORP_BIND=1`、`MOCHAT_GO_MIGRATE_CORP_STORE=1`、`MOCHAT_GO_MIGRATE_CORP_UPDATE=1`、`MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE=1`、`MOCHAT_GO_MIGRATE_SIDEBAR_ROOM_SOP_LOG_STATE=1`、`MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE=1`、`MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND=1`、`MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY=1`、`MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE=1`、`MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_REMIND=1`、`MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY=1`、`MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET=1`、`MOCHAT_GO_MIGRATE_SHOP_CODE_DASHBOARD=1` 不能搭配 `MOCHAT_GO_SKIP_JWT_BLACKLIST=1` 或 `MOCHAT_GO_DEV_AUTH_HEADER=1`，因为这些写接口必须使用真实 PHP JWT/侧边栏 JWT 和 Redis 黑名单检查。

## 测试

当前 shell 里可能有旧的 `GOROOT=/Users/lv/go-local/go`，测试脚本会清掉它。

```bash
cd /Users/lv/Documents/企业微信/mochat-go
./scripts/standalone_acceptance.sh
./scripts/test.sh
./scripts/audit_standalone_independence.sh
env -u GOROOT go test -race ./...
./scripts/smoke_standalone.sh
./scripts/standalone_stack_check.sh
./scripts/smoke_standalone_compose_app.sh
./scripts/standalone_inventory_parity.sh
./scripts/standalone_route_coverage.sh
./scripts/collect_standalone_evidence.sh
./scripts/smoke_independent_package.sh
./scripts/audit_goal_completion.sh
./scripts/source_fingerprint.py
./scripts/smoke_production_evidence_gate.sh
./scripts/smoke_bootstrap_standalone.sh
./scripts/smoke_saas_provisioning.sh
./scripts/smoke_saas_tenant_isolation.sh
./scripts/smoke_saas_quota_enforcement.sh
./scripts/smoke_saas_storage_reconcile.sh
./scripts/smoke_saas_storage_reclaim.sh
./scripts/smoke_saas_usage_refresh.sh
./scripts/smoke_queue_idempotency.sh
./scripts/smoke_async_file_upload_worker.sh
./scripts/smoke_mark_tags_worker.sh
./scripts/lint_mysql57_schema.sh
./scripts/smoke_mysql57_schema_migrate.sh
./scripts/smoke_wework_callback_worker.sh
./scripts/smoke_employee_apply_worker.sh
./scripts/smoke_fallback.sh
./scripts/local_stack_check.sh
./scripts/smoke_real_php_auth_chain.sh
./scripts/smoke_frontend_static_browser.sh
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
./scripts/smoke_contact_transfer_dashboard.sh
./scripts/smoke_room_fission_dashboard.sh
./scripts/smoke_work_fission_dashboard.sh
./scripts/smoke_dashboard_frontend_login.sh
./scripts/smoke_sidebar_frontend_contact.sh
./scripts/smoke_operation_frontend_work_fission.sh
```

说明：

- `scripts/standalone_acceptance.sh` 是独立 Go 版的长验收入口，默认执行 `all` 套件，串联快速门禁、迁移期清单对齐、schema migration、standalone 容器、路由覆盖、SaaS、worker、cron、前端和 MySQL 5.7 验证，并强制 `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0`；它默认不运行 PHP fallback/真实 PHP 对照，避免把迁移期依赖当成独立版验收。`core` 套件会在本地存在 `MOCHAT_SOURCE_ROOT` 或默认 `../mochat` 源码时运行 `standalone_inventory_parity.sh`，没有源码时跳过该迁移期清单门禁，不影响 Go standalone 运行时独立性；独立交付证据包可设置 `MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1` 强制跳过 PHP 清单对齐，只用 Go 内置 manifest 和 standalone smoke 自证；core 还会运行生产证据门禁规则自测，确认模板证据和 MySQL 5.7 arm64 skip 日志不会被误判为有效生产证据。`frontend` 套件会额外运行 `scripts/smoke_greeting_dashboard.sh`、`scripts/smoke_room_welcome_dashboard.sh`、`scripts/smoke_work_room_auto_pull_dashboard.sh`、`scripts/smoke_room_tag_pull_dashboard.sh`、`scripts/smoke_contact_message_batch_send_dashboard.sh`、`scripts/smoke_room_message_batch_send_dashboard.sh`、`scripts/smoke_room_fission_dashboard.sh` 和 `scripts/smoke_work_fission_dashboard.sh`，验证好友欢迎语全员/指定员工新增、列表、详情、编辑、删除、业务日志和素材静态 URL，入群欢迎语新增、列表、详情、选择、编辑、删除、企业微信图片上传、临时素材上传和模板 add/edit/del 请求，自动拉群新增、列表、详情、更新、兼容 move、企业微信 `contact_way/create`/`contact_way/update` 和 SaaS 用量刷新，标签建群客户筛选、新建、列表、详情、客户明细、员工任务、提醒发送、删除、企业微信 `media/uploadimg`、`externalcontact/add_msg_template`、`message/send` 和 `room_tag_pulls` 用量刷新，客户群发立即发送、列表、详情、消息预览、客户群详情、员工发送明细、客户接收明细、提醒、删除、企业微信 `media/upload`、`externalcontact/add_msg_template`、`message/send` 和 `contact_message_batches` 用量刷新，客户群群发立即发送、列表、详情、群主发送明细、群接收明细、提醒、删除、企业微信 `media/upload`、`externalcontact/add_msg_template`、`message/send` 和 `room_message_batches` 用量刷新，群裂变活动新建、列表、详情、群聊、客户、邀请、更新、核销、删除和 `room_fissions` 用量刷新，以及任务宝裂变活动新建、列表、详情、配置、统计、邀请、更新、删除、企业微信 `media/uploadimg`、`externalcontact/add_contact_way`、`externalcontact/add_msg_template` 和 `work_fissions` 用量刷新。可用 `MOCHAT_ACCEPTANCE_SUITE=core|saas|workers|cron|frontend|mysql57` 分段执行；确需迁移期对照时设置 `MOCHAT_ACCEPTANCE_SUITE=php MOCHAT_ACCEPTANCE_INCLUDE_PHP=1`。
- `frontend` 套件还会运行 `scripts/smoke_sensitive_word_dashboard.sh`，覆盖敏感词分组批量新建、更新、下拉选择，词库批量拆词去重、新增、移动、状态更新、删除、监控列表、监控详情会话展开和 `sensitive_words` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_channel_code_dashboard.sh`，覆盖渠道活码分组新建、详情、更新、移动，渠道活码新建、更新、列表、详情、客户明细、统计，企业微信 `externalcontact/add_contact_way` / `externalcontact/update_contact_way`，业务日志、扫码客户统计和 `channel_codes` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_shop_code_dashboard.sh`，覆盖门店活码新建、列表、详情、位置、城市/地址检索、分享、页面设置、统计、客户明细、门店统计、员工更新、二维码更新、状态切换、批量打标签、删除和 `shop_codes` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_radar_dashboard.sh`，覆盖互动雷达新建、列表、详情、渠道、渠道链接、客户点击明细、渠道统计、文章元数据回显、更新、删除和 `radars` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_lottery_dashboard.sh`，覆盖抽奖活动新建、列表、详情、分享、中奖客户、核销、批量打标签、更新、删除、级联软删和 `lotteries` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_room_infinite_pull_dashboard.sh`，覆盖无限拉群新建、列表、详情、更新、删除、企微活码 JSON、详情链接和 `room_infinite_pulls` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_room_clock_in_dashboard.sh`，覆盖群打卡新建、列表、详情、客户列表、天数明细、批量打标签、更新、停用状态、删除、级联软删和 `room_clock_ins` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_room_quality_dashboard.sh`，覆盖群质检新建、列表、弹窗详情、触发客户列表、触发客户详情、启停、更新、删除、触发记录级联软删和 `room_qualities` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_room_calendar_dashboard.sh`，覆盖群日历新建、列表、详情、增加群聊、移除群聊、更新推送计划、删除、推送和记录级联软删、`room_calendar_push.room_calendar_id` 字符串兼容查询和 `room_calendars` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_room_remind_dashboard.sh`，覆盖客户群提醒新建、列表、详情、任务视图、启停、更新、删除、提醒记录级联软删和 `room_reminds` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_sop_dashboard.sh`，覆盖个人/群 SOP 新建、列表、详情、设置员工/群聊、启停、更新、删除、触达日志级联删除和 `contact_sops` / `room_sops` SaaS 用量刷新。
- `frontend` 套件还会运行 `scripts/smoke_auto_tag_dashboard.sh`，覆盖三类自动标签规则新建、列表、详情、触发记录、启停、删除、会话成员筛选、会话列表和会话存档配置读写。
- `frontend` 套件还会运行 `scripts/smoke_contact_batch_add_dashboard.sh`，覆盖批量加好友设置、CSV 导入、导入记录、客户列表筛选、员工统计、二次分配、提醒、单条删除、导入批次删除和 `storage_mb` 账本回收。
- `frontend` 套件还会运行 `scripts/smoke_contact_transfer_dashboard.sh`，覆盖在职转接/离职继承待分配同步、在职客户列表、离职待分配客户列表、待分配群列表、客户接替、群接替、分配记录和企业微信 `get_unassigned_list` / `transfer_customer` / `groupchat/transfer` 请求口径。
- `scripts/test.sh` 会先执行 `scripts/audit_standalone_independence.sh`、`scripts/audit_acceptance_suite_coverage.sh`、`scripts/audit_manifest_route_smoke_coverage.sh`、`scripts/audit_functional_module_matrix.sh`、`scripts/audit_login_corp_validation.sh`、队列注解覆盖、worker SaaS 用量断言、SaaS 指标覆盖、SaaS 存储回收覆盖审计和生产证据门禁自测，再执行 `go test ./...`、`go vet ./...` 和 `cmd/mochat-go`、`cmd/mochat-inventory`、`cmd/mochat-migrate`、`cmd/mochat-bootstrap`、`cmd/mochat-saas-maintenance` 五个入口的 `go build`。
- `scripts/audit_standalone_independence.sh` 会静态审计独立交付包和核心 standalone smoke，阻止 `deploy/standalone` 定义 PHP 服务、挂载原 `../mochat`、配置 PHP fallback，阻止核心 standalone smoke 重新依赖 fake PHP、原项目路径、外部 manifest 或逐路由迁移开关。
- `scripts/audit_acceptance_suite_coverage.sh` 会静态扫描所有 `scripts/smoke_*.sh`，确保每个 smoke 都已纳入 `scripts/standalone_acceptance.sh`，并反查验收入口没有引用已删除的 smoke，防止新增验收脚本游离在独立版总验收之外。
- `scripts/audit_manifest_route_smoke_coverage.sh` 会读取内置 `compat_manifest_embedded.json`，要求 224 条原 PHP manifest 路由都能在 `scripts/smoke_*.sh` 中找到直接覆盖痕迹；动态路由会按 `{appId}`、`{params?}` 和 `WW_verify_*.txt` 生成可匹配变体。
- `scripts/audit_functional_module_matrix.sh` 会把内置 manifest 的 224 条 method+path 路由和 `internal/server/server.go` 中的 Go 运行时迁移路由归入 29 个业务功能模块，并检查每个模块是否具备 Go 源码、已纳入 `standalone_acceptance.sh` 的 smoke 证据，以及关键 SaaS 指标、上传账本回收、队列或企微回调门禁 token；它用于防止只用路由覆盖数误判“全功能迁移完成”，也防止新增 Go 兼容路由游离在模块验收之外。
- `scripts/audit_frontend_dist_api_coverage.sh` 会扫描 `web/dashboard/dist`、`web/sidebar/dist` 和 `web/operation/dist` 内置旧前端构建产物中的静态 API 声明，并与 `internal/server/server.go` 的 Go runtime dispatch/routes 对比；默认只报告，设置 `MOCHAT_FRONTEND_DIST_API_STRICT=1` 后发现缺失 API 会返回非 0。该脚本用于防止 `manifest 224/224` 已通过但旧前端真实调用的 API 仍未被 Go 承接。
- `scripts/audit_login_corp_validation.sh` 会静态审计 dashboard 业务 handler，禁止绕过 `ResolveValidatedLoginCorpInfoFromStore` 直接信任 `mc:user.{userId}` 企业选择缓存，防止脏 Redis 缓存重新造成跨租户读数据。
- `scripts/audit_saas_metric_coverage.sh` 会静态审计 26 个 SaaS 指标，确保 `saas_quota.go`、`cmd/mochat-bootstrap`、`internal/store/mysql.go`、额度拦截 smoke 和用量重刷 smoke 保持同一套指标，防止新增套餐资源后漏计、漏刷或漏验收。
- `scripts/audit_saas_storage_reclaim_coverage.sh` 会静态抽取 `internal/store` 中调用 `reclaimSaaSStorageObjects` 的 26 个业务函数，并检查 `scripts/smoke_saas_storage_reclaim.sh` 是否仍覆盖 14 类触发式账本回收链路；新增上传文件引用入口时，需要同步补真实 smoke 断言。
- `scripts/audit_wework_callback_event_coverage.sh` 会静态抽取 `WeWorkCallbackWorker.Process` 中的企微事件路径，要求每个事件都出现在 `scripts/smoke_wework_callback_worker.sh`，并要求 PHP 兼容 no-op 事件同时保留单测和 smoke 覆盖，防止新增或改动回调 case 后漏掉独立栈回归。
- `scripts/smoke_standalone.sh` 会故意注入无效 `MOCHAT_PHP_UPSTREAM`、不存在的 `MOCHAT_SOURCE_ROOT` 和 `MOCHAT_COMPAT_MANIFEST`，验证 `MOCHAT_GO_STANDALONE=1` 时不会读取这些迁移期依赖，dashboard `/login` 可由 Go 服务从 `web/dashboard/dist` 返回，`/favicon.ico` 可由 Go 独立服务响应，且未迁移路由返回 501。
- `docs/phases/phase-pre0-standalone/plans/release-candidate.md` 是独立交付候选说明，串联 `deploy/standalone/.env.example`、standalone compose 启动、migration baseline、`mochat-bootstrap` 初始化、本地短验收、生产证据采集和发布前检查；它用于把“本地能跑”收口成可交给部署/验收人员执行的 Go standalone RC。
- `scripts/smoke_independent_package.sh` 会把当前 `mochat-go` 复制到没有原 `mochat/` 兄弟目录的临时目录，污染 `MOCHAT_SOURCE_ROOT` 和 `MOCHAT_COMPAT_MANIFEST`，再执行 standalone 独立性审计、embedded 队列注解覆盖审计和 standalone smoke；它用于证明交付出去的 Go 包不依赖本机原 PHP checkout，也不会启动 24 小时持续运行。
- `scripts/standalone_stack_check.sh` 会启动只包含 MySQL/Redis 的独立依赖栈，并在污染 PHP/source/manifest 环境变量的情况下验证本项目内置 SQL 初始化、Redis、Go standalone `/readyz`、dashboard `/login` 和未迁移路由 501，不启动 PHP，也不挂载原 MoChat 源码。
- `scripts/standalone_soak_24h.sh` 是独立栈可选持续运行验收入口，默认 `MOCHAT_SOAK_DURATION_SECONDS=86400`、每 60 秒探测一次；它会启动独立 MySQL/Redis 和 Go standalone，反复验证 `/readyz` 没有采信 PHP/source/manifest 残留、`/compat/routes` 维持 224 条业务路由、dashboard/sidebar/operation 前端入口可访问、未知路由返回 Go standalone 501、MySQL/Redis 仍可用，并把每轮结果写入 `soak.ndjson`。当前阶段不把 24 小时 soak 作为继续开发必跑项；稳定性证据默认走短稳回归、健康检查和外部监控记录，只有上线另行要求满 24 小时长稳时才使用该入口或导入等价记录。
- `scripts/standalone_stage_report.sh` 会生成中文 Markdown 阶段报告，汇总内置 manifest、smoke/acceptance 覆盖、独立交付包动态 smoke、前端 dist API 覆盖、最近 `soak.ndjson`、当前 Docker 残留和仍不能标记最终完成的生产证据缺口；可用 `MOCHAT_STAGE_REPORT_OUT=docs/phases/phase-pre0-standalone/reports/final-stage-report.md ./scripts/standalone_stage_report.sh` 写入文件。
- `scripts/production_evidence_check.sh` 会生成中文 Markdown 生产证据检查报告，先执行独立性、独立交付包动态 smoke、acceptance 覆盖、manifest 路由、功能模块矩阵、队列、企微回调、worker SaaS、SaaS 指标和存储回收等快速门禁，再检查 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、真实 SaaS 租户、生产前端和稳定性证据文件；证据文件必须存在、非空且满足必要 marker、核验项关键词、明确通过结论、结构化证据记录项和当前源码指纹，MySQL 5.7 证据会拒绝 arm64 skip 日志，证据正文里出现 `example.com`、`localhost`、`127.0.0.1`、`.test`、`.invalid` 等示例域名或本机地址也会被拒绝。默认只报告缺口，设置 `MOCHAT_PRODUCTION_EVIDENCE_STRICT=1` 后缺任一生产证据会返回非 0；可用 `MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT` 同步输出不含完整命令输出和密钥的机器可读 JSON，记录 quick/evidence 总状态、6 类证据明细、缺失环境变量、失败原因和当前源码指纹。
- 生产证据必须脱敏；`scripts/production_evidence_check.sh` 会拒绝疑似原始 `Authorization`、`Bearer`、JSON / query 中的 `access_token`、`component_access_token`、`authorizer_refresh_token`、`corpsecret`、`encodingaeskey`、`password` 等敏感值。真实联调证据只保留脱敏摘要、必要业务字段、真实生产域名、截图/日志/工单引用或 `<redacted>` 占位；生产证据采集脚本会对常见响应摘要和 URL 参数做自动脱敏，但导入前仍需人工复核。
- `scripts/audit_goal_completion.sh` 会按用户目标生成完成度审计报告，把“能运行的独立 Go 项目”“不依赖原 mochat/PHP 项目”“全部 manifest 功能迁移”“旧前端 dist API 全量承接”“SaaS/worker/cron/frontend 本地闭环”“生产外部证据”和短稳/外部监控稳定性证据逐条判定；报告内部会用严格前端 dist API 覆盖和严格生产证据检查标出真实缺口，并校验本地短证据包的 `source-fingerprint.json` 是否匹配当前源码/验收脚本指纹，默认读取 `docs/phases/phase-pre0-standalone/evidence/latest`，可用 `MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR` 指向生产候选或自定义证据目录。默认只报告并返回 0，可用 `MOCHAT_GOAL_COMPLETION_JSON_OUT` 同步输出不含完整命令输出和密钥的机器可读 JSON；设置 `MOCHAT_GOAL_COMPLETION_STRICT=1` 后目标未完成会返回非 0；兼容开关 `MOCHAT_GOAL_COMPLETION_REQUIRE_24H=1` 只在另行要求满 24 小时长稳时使用。
- `scripts/init_production_evidence_pack.sh` 会初始化 `docs/phases/phase-pre0-standalone/evidence/production/` 下的生产证据模板和 `env.production-evidence.example`。模板带有 `MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE` 防误用标记；真实联调完成后必须删除该标记并填入实际证据和当前源码指纹，否则 `scripts/production_evidence_check.sh` 会拒绝。
- `scripts/smoke_production_evidence_gate.sh` 会自测生产证据门禁规则：模板证据必须被拒绝、MySQL 5.7 arm64 skip 日志必须被拒绝、只有关键词但没有结构化证据记录的文件必须被拒绝、缺少源码指纹或源码指纹过期的文件必须被拒绝，包含必填核验项、明确通过结论、结构化证据记录和当前源码指纹的证据文件才能通过；同时断言 `production-evidence.json` 在失败和通过场景都可解析且不包含 Bearer/raw token。该脚本只跳过重复快速门禁来验证证据文件规则，不启动 24 小时持续运行。
- `scripts/source_fingerprint.py` 会对 Go 源码、验收脚本、部署配置、前端构建产物和关键构建文件生成 sha256 指纹，排除 `docs/phases/phase-pre0-standalone/evidence/`、`storage/`、`output/`、`tmp/`、`node_modules` 以及私密 `.env`/`.env.*` 文件（保留 `.env.example`）；可用 `--check docs/phases/phase-pre0-standalone/evidence/latest/source-fingerprint.json` 判断 latest 证据包是否仍对应当前源码。
- `scripts/collect_standalone_evidence.sh` 会生成本地短验收证据包，默认写入并重建 `docs/phases/phase-pre0-standalone/evidence/latest/`：串联 `scripts/test.sh`、`scripts/smoke_independent_package.sh`、迁移期 PHP 清单对齐、`MOCHAT_ACCEPTANCE_SUITE=<suite> MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1 ./scripts/standalone_acceptance.sh`、`scripts/standalone_route_coverage.sh`、前端 dist API 覆盖审计、阶段报告、生产证据检查和目标完成度审计，保留每个命令的日志、`route-coverage.json`、`frontend-dist-api-coverage.md`、`source-fingerprint.json`、`current-stage-report.md`、`production-evidence.md`、`production-evidence.json`、`goal-completion.md`、`goal-completion.json` 和汇总 `index.md`。默认只跑 `MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=core`；需要扩大到全套非 PHP 短验收时可设置 `MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES='core saas workers cron frontend mysql57'`，或使用 `all` 作为简写。`MOCHAT_LOCAL_EVIDENCE_INVENTORY_PARITY=auto` 会在本地存在原 `../mochat/api-server` 或 `MOCHAT_LOCAL_EVIDENCE_SOURCE_ROOT` 指向源码时额外记录 `inventory-parity`，用于证明 embedded manifest 和原 PHP 扫描清单一致；独立验收套件仍会设置 `MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1` 并污染 `MOCHAT_SOURCE_ROOT`、`MOCHAT_COMPAT_MANIFEST`，强制证明独立交付包不需要原 PHP 源码或外部 manifest 也能自证。目标完成度审计会用当前 `MOCHAT_LOCAL_EVIDENCE_DIR` 作为 `MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR`，不会被旧 latest 证据影响；它不启动 24 小时持续运行。`index.md` 会把 `frontend-dist-api-coverage`、`production-evidence` 和 `goal-completion` 区分为“报告生成”，单独展示报告内当前结论，并展示源码与验收指纹，避免把前端 API 缺口、生产证据缺口或过期证据误判为目标完成。
- `scripts/production_candidate_gate.sh` 是生产候选总门禁：默认先跑全套非 PHP 本地短验收证据包，再执行严格生产证据检查、严格目标完成度审计和源码/验收指纹匹配，输出到 `docs/phases/phase-pre0-standalone/evidence/production/candidate/`；它要求 `MOCHAT_EVIDENCE_MYSQL57_AMD64`、真实企微、真实微信开放平台、真实 SaaS 多租户、生产前端、稳定性证据和候选证据包指纹全部有效后才返回 0。默认模式下严格生产证据检查会同时输出 `production-evidence.md` 和 `production-evidence.json`；严格目标完成度审计读取 `docs/phases/phase-pre0-standalone/evidence/production/candidate/local`，避免被旧 latest 证据影响，并同时输出 `goal-completion.md` 和 `goal-completion.json`；目标审计可用 `MOCHAT_GOAL_COMPLETION_SOURCE_ROOT` 指定原 PHP 源码目录，避免被独立运行污染路径影响清单对齐；可设置 `MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1` 只复核外部证据、严格目标完成度和 `docs/phases/phase-pre0-standalone/evidence/latest/source-fingerprint.json` 是否仍匹配当前源码。候选报告首页会摘取严格生产证据检查和严格目标完成度审计的当前结论，并展示当前指纹、对比来源和匹配结果，方便区分本地短验收通过与最终生产候选未通过。该入口不会启动 24 小时持续运行。
- `scripts/collect_production_evidence_pack.sh` 会把真实生产证据收拢成可复核包，默认从 `docs/phases/phase-pre0-standalone/evidence/production/` 读取标准文件名，复制到 `docs/phases/phase-pre0-standalone/evidence/production/current/`，生成 `env.production-evidence`、`manifest.json`、sha256 摘要、当前源码指纹、`index.md` 和 `production-evidence.json`，先执行严格生产证据文件检查，再默认用 `MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1` 跑生产候选门禁。标准文件包括 `mysql57-amd64.log`、`real-wecom.md`、`real-wechat-open.md`、`real-saas-tenants.md`、`prod-frontend.md` 和 `stability.md`；也可直接设置 `MOCHAT_EVIDENCE_*` 指向任意来源文件。复制前会扫描未脱敏敏感值、非生产 URL 和源码指纹，疑似包含原始 token、secret、password 的源证据会被标记为 `敏感值拒绝，未复制`，包含 `example.com`、`localhost`、`127.0.0.1`、`.test`、`.invalid` 等示例域名或本机地址的源证据会被标记为 `非生产地址拒绝，未复制`，缺少当前源码指纹或源码指纹过期的源证据会被标记为 `源码指纹拒绝，未复制`，不会落入 `current/` 包。只想检查证据文件本身时设置 `MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE=0`；该脚本不会启动 24 小时持续运行。
- `scripts/production_evidence_doctor.sh` 是生产证据采集诊断入口：默认只检查 6 类标准证据文件、采集环境变量和严格证据门禁状态，报告写入 `docs/phases/phase-pre0-standalone/evidence/production/readiness.md`，机器可读状态写入 `docs/phases/phase-pre0-standalone/evidence/production/readiness.json`，待填写环境变量清单写入 `docs/phases/phase-pre0-standalone/evidence/production/readiness.env.todo`，并保留严格证据门禁的 `readiness-production-evidence.md` 与 `readiness-production-evidence.json`，不访问生产也不启动 24 小时持续运行；JSON 会区分标准文件是否存在和是否通过严格生产证据规则，模板、待验收、未脱敏、非生产 URL、缺结构化记录项、缺源码指纹或源码指纹过期等文件即使存在，也会标记为无效；JSON 和 env 清单只记录变量名、标准文件路径、缺失项、采集命令和门禁返回码，不记录 token 或 secret 值。不要直接把密钥写入 `readiness.env.todo`，应复制为已加入 `.gitignore` 的 `docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local` 后填写；`scripts/production_evidence_env_preflight.sh` 可在采集前检查变量是否齐备、`@文件` 是否存在、URL 是否有效，默认拒绝 `example.com`、`localhost`、`127.0.0.1` 等示例域名或本机地址，真实采集脚本本身也会拒绝这些占位地址，避免跳过 preflight 后误采假生产环境；并校验 MySQL 5.7 amd64 日志或 artifact zip 是否包含通过标记、是否误用了 arm64 skip 日志；稳定性证据走外部监控摘要时，预检还会要求监控摘要包含路由覆盖和 224、健康证据包含 `/readyz`/health/健康/探测/200、资源证据包含 RSS/memory/CPU/连接/内存/资源，并要求时间范围有明确起止日期；标准证据文件只有在严格门禁判定有效时才会被 preflight 视为已就绪，存在但无效会标为“标准文件无效”，6 类标准文件都有效时不再要求 env 文件存在即可通过 strict 预检并进入证据包收拢。该预检不会访问生产或打印密钥值。设置 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1` 后才会对环境变量已齐备的项目调用导入/采集脚本，设置 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_STRICT=1` 可让缺任一证据或采集失败时返回非 0。
- `scripts/capture_real_wecom_evidence.sh` 会用真实生产 dashboard token 访问企业微信相关读路径，并结合真实回调解密、企微应用消息、截图/日志/工单引用生成符合生产证据规则的 `docs/phases/phase-pre0-standalone/evidence/production/real-wecom.md`；默认覆盖授权、通讯录、客户、客户群、标签和素材路径，可用 `MOCHAT_REAL_WECOM_*_PATHS` 覆盖生产路径。该脚本只采集生产证据，不启动 24 小时持续运行。
- `scripts/capture_real_wechat_open_evidence.sh` 会用真实生产 dashboard token 访问微信开放平台预授权和公众号资料路径，并结合 ticket、授权回跳、取消授权、消息回调、截图/日志/工单引用生成符合生产证据规则的 `docs/phases/phase-pre0-standalone/evidence/production/real-wechat-open.md`。该脚本只采集生产证据，不启动 24 小时持续运行。
- `scripts/capture_real_saas_tenants_evidence.sh` 会用两个真实生产租户的 dashboard token 访问配置的读路径，校验各自响应包含租户标识，并用跨租户资源路径验证 A token 访问 B 资源、B token 访问 A 资源会被拒绝；同时要求传入额度拦截、上传账本、异步任务和告警处置证据摘要或文件，全部满足后生成符合生产证据规则的 `docs/phases/phase-pre0-standalone/evidence/production/real-saas-tenants.md`。该脚本只采集生产证据，不启动 24 小时持续运行。
- `scripts/capture_prod_frontend_evidence.sh` 会用 Playwright 访问真实生产 `MOCHAT_PROD_FRONTEND_BASE_URL` 下配置的 dashboard、sidebar 和 operation 路径，采集截图、console error、同源请求失败和非预期 4xx/5xx，生成符合生产证据规则的 `docs/phases/phase-pre0-standalone/evidence/production/prod-frontend.md`；生产页面需要登录时可传入 `MOCHAT_PROD_FRONTEND_AUTH_STATE`。该脚本只做浏览器证据采集，不启动 24 小时持续运行。
- `scripts/capture_stability_evidence.sh` 会解析目标部署环境已有短稳回归、健康检查、外部监控摘要，或 legacy `soak.ndjson`，校验时间范围、standalone 模式、路由覆盖和资源使用，并生成符合生产证据规则的 `docs/phases/phase-pre0-standalone/evidence/production/stability.md`；外部监控摘要不能只写“monitor ok”，需要给出 route_total/missing_route_total 或等价路由覆盖、`/readyz`/health 结果、RSS/CPU/连接等资源记录和起止时间范围。默认最小时长为 300 秒，不再默认要求 24 小时；另行要求满 24 小时长稳时再设置 `MOCHAT_STABILITY_MIN_DURATION_SECONDS=86400`。该脚本只读取已有记录，不启动 24 小时持续运行。
- `scripts/smoke_standalone_compose_app.sh` 会使用 `deploy/standalone/docker-compose.yml --profile app` 构建并启动 Go app + MySQL + Redis 容器栈，验证容器内 Go standalone `/readyz`、内置 manifest、dashboard/sidebar/operation 前端入口、SaaS 总后台页面、`0033` 至 `0053` 初始化、订阅、支付订单、退款、开票资料、发票单、渠道结算批次/明细、结算同步状态/运行记录、平台角色/授权、审批单/事件/策略/会签/委托、健康扫描/系统事故、服务账号/API Key、备份/恢复、合规生命周期和身份安全台账与支付回调表、MariaDB 客户端实际加密备份、通知健康自动恢复和审批 SLA 提醒 task runner、容器内 `mochat-migrate baseline`、`mochat-bootstrap`、真实 `POST /dashboard/user/auth` 登录、`loginShow`、`permissionByUser`、`corp/select`、`corp/bind`、Redis 企业选择缓存、`corpData/index`、`corpData/lineChat`、`workEmployee/searchCondition`、`workDepartment/index` 和 `workEmployee/index` 闭环。
- `scripts/smoke_sidebar_frontend_contact.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，在故意污染 `MOCHAT_PHP_UPSTREAM`、`MOCHAT_SOURCE_ROOT`、`MOCHAT_COMPAT_MANIFEST` 的情况下先用真实浏览器访问 sidebar `/login?agentId=1&target=/contact?agentId=1`，验证旧 dist 会进入 Go `/sidebar/agent/auth`、生成企业微信 OAuth 外跳、用 code 回调 `auth/getuserinfo` 签发 sidebar 员工 JWT，并由前端 `/auth` 写入 `token/agentId` cookie 后落回 `/contact?agentId=1`；随后继续访问 `/contactSop?id=900001&agentId=1`、`/roomSop?id=900001`、`/contactBatchAdd?batchId=900001` 和 `/medium?agentId=1`，验证 `agent/jssdkConfig`、`workContact/detail/show/track`、`contactFieldPivot/index`、`contactSop/getSopTipInfo`、`contactSop/getSopInfo`、`roomSop/getSopInfo`、`roomSop/logState`、`contactBatchAdd/detail`、`mediumGroup/index`、`medium/index`、本地 `/static/*` 头像/画像资源，以及旧 dist 里 `/undefined/sidebar/*` API 前缀能被 Go runtime 兼容归一化；脚本会开启 `cron-sop-log` 并等待 Go 自动生成普通和周期个人/群 SOP 日志，以及 `targetAnchor=room_join` 入群后群 SOP 日志，断言周期日志带 `_mochatGoOccurrence`、入群后群 SOP 写入 `mc_room_sop_log.contact`，再断言个人 SOP 弹窗、个人 SOP 详情页、群 SOP 页面和 `mc_room_sop_log.state` 均可独立闭环。
- `scripts/smoke_operation_frontend_work_fission.sh` 会启动独立 MySQL/Redis、fake 微信开放平台 API 和 Go standalone，在无 PHP、无原源码、无外部 manifest 的情况下先验证 operation `/auth/workFission` 生成公众号 OAuth 外跳参数，再用 code 回调调用 fake 微信 `api_component_token`、`sns/oauth2/component/access_token`、`sns/userinfo` 写入 Go operation session；同一脚本也会验证兼容 `/load/{params?}` 的 GET OAuth 外跳和 POST code 回调。随后脚本用真实浏览器访问 `/workFission?id=900001` 和助力进度页，验证 `openUserInfo/workFission` 同源别名、`workFission/poster`、`taskData`、`inviteFriends`、`receive`、本地 `/static/*` 海报/头像/二维码资源，以及旧 dist 里 `/undefined/operation/*` API 前缀归一化。
- `scripts/standalone_inventory_parity.sh` 会从 `MOCHAT_SOURCE_ROOT` 指向的 PHP 原项目重新扫描路由、表、定时任务、事件处理器和队列注解，并和 Go 内置 `compat_manifest_embedded.json` 对比，防止迁移清单漏收原 PHP 功能；默认读取 `../mochat`，只作为迁移期清单门禁，不是 Go standalone 运行时依赖。
- `scripts/smoke_bootstrap_standalone.sh` 会在迁移后的空库上执行 `cmd/mochat-bootstrap`，创建默认租户、超级管理员、管理员角色、用户角色绑定、可用菜单权限绑定、SaaS 套餐绑定、用量计数、seed 版本记录和开通记录，再启动 Go standalone 的 `POST /dashboard/user/auth` 验证该账号可以直接换取 token。
- `scripts/smoke_saas_provisioning.sh` 会使用 `cmd/mochat-bootstrap -batch-file` 一次开通两个租户，验证 `mochat_go_saas_packages`、`mochat_go_saas_tenant_packages`、`mochat_go_saas_usage_counters`、`mochat_go_seed_versions` 和 `mochat_go_tenant_provision_runs` 写入正确，并验证两个租户管理员都能通过 Go standalone 登录。
- `scripts/smoke_saas_admin_dashboard.sh` 会启动独立 Go + MySQL，验证 SaaS 总后台页面、平台 overview、租户 overview、单租户详情、租户生命周期审计、租户用量明细、运营日报、经营指标、经营趋势、风险看板、客户成功队列、风险跟进、风险跟进任务列表、负责人工作台、风险跟进批量关闭、告警处置和批量解决、通知重试、批量重试、通知关闭和批量关闭、套餐列表、套餐维护、套餐变更影响、套餐快照同步、套餐同步任务、续费任务、平台开户、租户开停、套餐到期拦截、续费账单、账单筛选汇总、告警/通知筛选汇总、告警/通知 CSV 导出、操作记录、租户/生命周期审计/用量/风险/经营指标/经营趋势/风险跟进任务/风险负责人/运营日报/操作/账单/账单跟进/账单负责人 CSV 导出、平台调整租户套餐、调整后 26 项额度刷新，以及普通租户越权访问平台总览/生命周期审计/用量明细/运营日报/经营指标/风险看板/客户成功队列/风险跟进/风险跟进任务/负责人工作台/风险跟进批量关闭/告警/批量告警解决/通知/批量通知重试/批量通知关闭/套餐/操作记录/账单/CSV 导出/平台开户/租户开停/租户套餐调整均返回 `403`；页面断言包含经营指标、经营趋势、导出经营指标 CSV、导出经营趋势 CSV、日报日期、统计天数、刷新日报、运营日报明细、日报负责人、日报通知、日报操作动作、日报账单流水、客户成功队列、生命周期审计、生命周期筛选、导出审计 CSV、套餐变更影响、套餐快照同步入口、套餐同步任务入口和续费任务入口；脚本会真实调用 `GET /dashboard/saasAdmin/businessMetrics` 并通过 `GET /dashboard/saasAdmin/export?type=businessMetrics` 导出经营指标 CSV，断言最近续费账单会进入估算 MRR/ARR、近期账单金额和套餐收入分布；脚本会真实调用 `GET /dashboard/saasAdmin/risk`，断言租户风险等级、风险分、风险原因、建议动作和 Top 风险指标，并调用 `GET /dashboard/saasAdmin/customerSuccess` 断言待处理租户的优先级、健康分、原因、下一步动作和失败通知数量，再用 `GET /dashboard/saasAdmin/export?type=risk` 导出风险 CSV；真实调用 `POST /dashboard/saasAdmin/riskFollowUp`，断言写入 `tenant.risk.follow_up` 操作日志，并再次读取风险看板、`GET /dashboard/saasAdmin/riskFollowUps` 任务列表、`GET /dashboard/saasAdmin/riskFollowUpOwners` 负责人工作台、显式带 `date` 和 `days` 的 `GET /dashboard/saasAdmin/dailyReport` 运营日报、`GET /dashboard/saasAdmin/export?type=dailyReport` 日报 CSV、`GET /dashboard/saasAdmin/export?type=riskFollowUps` 跟进任务 CSV、`GET /dashboard/saasAdmin/export?type=riskFollowUpOwners` 风险负责人 CSV 和风险 CSV，确认最新跟进状态、负责人、下次跟进时间、备注、到期状态、操作 ID、负责人任务分布、打开跟进、打开告警、失败通知、日报窗口日期、日报 CSV summary 和当日操作已回显；随后真实调用 `POST /dashboard/saasAdmin/riskFollowUpBulkClose`，断言按筛选条件把最新任务置为 `resolved`、任务列表进入 `closed`、并追加新的 `tenant.risk.follow_up` 操作日志；后段还会调用 `GET /dashboard/saasAdmin/export?type=usage` 导出目标租户用量 CSV，断言 users 指标、额度、状态和打开告警数字段；脚本还会断言套餐保存 `impact` 返回额度变化、分配租户数、降额后的超新额度租户和租户套餐快照未自动改写口径，并调用 `POST /dashboard/saasAdmin/packageSync` 验证 dry-run 预览不写入、实际同步遇到超额会阻断、显式允许超额后会写入租户快照并刷新 26 项额度，再调用 `POST /dashboard/saasAdmin/packageSyncTask` 和 `POST /dashboard/saasAdmin/packageSyncTaskApply` 验证套餐同步任务可创建 `blocked/pending` 状态、应用后变为 `applied`，并可通过 `GET /dashboard/saasAdmin/tasks` 回看任务；调用 `POST /dashboard/saasAdmin/tenantRenewalTask` 和 `POST /dashboard/saasAdmin/tenantRenewalTaskApply` 验证续费任务先生成预览、应用后写入账单事件并变为 `applied`，并通过 `GET /dashboard/saasAdmin/tasks?taskType=tenant_renewal` 回看任务；还会真实调用 `POST /dashboard/saasAdmin/alertResolve`，断言告警转为 resolved、用量明细打开告警数归零，并生成 `tenant.alert.resolve` 操作日志；真实调用 `POST /dashboard/saasAdmin/alertBulkResolve`，断言匹配筛选条件的 open 告警批量转为 resolved，并为每条告警生成带 `bulkResolve=true` 的 `tenant.alert.resolve` 操作日志；真实调用 `POST /dashboard/saasAdmin/notificationRetry`，断言 dead 通知回到 pending、失败原因清空，并生成 `tenant.notification.retry` 操作日志；真实调用 `POST /dashboard/saasAdmin/notificationBulkRetry`，断言匹配筛选条件的 failed 通知批量回到 pending、失败原因清空，并为每条通知生成 `tenant.notification.retry` 操作日志；随后真实调用 `POST /dashboard/saasAdmin/notificationClose` 和 `POST /dashboard/saasAdmin/notificationBulkClose`，断言 pending 通知进入 `closed`、`nextRetryAt` 清空、失败原因写入关闭备注，并生成带 `tenant.notification.close` 和 `bulkClose=true` 的操作日志。
- SaaS 总后台 smoke 在关闭通知后还会重新读取显式日期窗口的 `GET /dashboard/saasAdmin/dailyReport` 和 `GET /dashboard/saasAdmin/export?type=dailyReport`，断言关闭通知进入 `notifications.closedItems`，`closedNotificationCount`、`windowNotificationCloseCount`、`notifications.summary.closedCount` 和日报 CSV 的 `closedNotification` section 均回显关闭备注；在运营待办认领后还会断言 `operationQueueAssignments?dueState=future`、认领 CSV、日报 `operationQueueAssignments` 分区、`windowQueueAssignmentCount/windowTaskSlaAssignCount`、日报 CSV 的 `operationQueueAssignment` section、`tenantLifecycle?source=operation&status=contacted` 和生命周期审计 CSV 都能回显任务 SLA 认领负责人、来源、状态、到期状态和备注。
- SaaS 总后台 smoke 还会断言页面包含“经营趋势”、`businessTrends`、`businessRenewalFunnel` 和“导出经营趋势 CSV”，普通租户访问 `GET /dashboard/saasAdmin/businessTrends` 与 `GET /dashboard/saasAdmin/export?type=businessTrends` 返回 `403`；平台管理员在续费任务应用并写入账单后读取趋势接口并导出经营趋势 CSV，确认最近账单进入月度趋势、scale 套餐流水和续费任务漏斗。
- SaaS 总后台 smoke 还会断言页面包含“续费预测”、`renewalForecast`、`renewalForecastBuckets`、`renewalForecastOwners`、“续费预测筛选”、“续费预测分派”、“续费预测提醒”、“续费预测负责人工作台”、“导出续费预测 CSV”和“导出续费预测负责人 CSV”，普通租户访问 `GET /dashboard/saasAdmin/renewalForecast`、`POST/PUT /dashboard/saasAdmin/renewalForecastAssign`、`POST/PUT /dashboard/saasAdmin/renewalForecastNotifications`、`GET /dashboard/saasAdmin/export?type=renewalForecast` 与 `GET /dashboard/saasAdmin/export?type=renewalForecastOwners` 返回 `403`；平台管理员在续费任务应用并写入账单后读取预测接口，确认目标租户进入到期预测、最近续费价格用于预测收入、负责人工作台按负责人聚合预测金额/到期窗口/任务状态/Top 租户，并回显负责人和最新续费任务；随后会用目标租户自身的 `bucket`、`priced`、`packageCode`、`owner` 和 `taskStatus` 再读取一次筛选后的预测接口，确认筛选口径收窄但仍命中目标租户且负责人聚合同步收窄；平台管理员按同一筛选调用 `renewalForecastAssign` 后会写入 `renewal_pending` 跟进、负责人和下次跟进时间；创建续费预测任务后还会按负责人生成续费预测提醒、导出续费预测 CSV 和续费预测负责人 CSV，断言目标租户的提醒 outbox、套餐、预测金额、定价状态、最近账单、任务计数、分派负责人、最新跟进状态、最新待处理任务状态，以及负责人 CSV 的预测金额、待处理任务、下次跟进和 Top 租户。
- 客户成功队列页面支持优先级、负责人和租户上限筛选；`GET /dashboard/saasAdmin/export?type=customerSuccess` 会按同一口径导出客户成功 CSV，smoke 会断言页面筛选入口、普通租户导出 `403`、目标租户优先级/健康分/负责人/待处理原因/下一步动作/Top 用量指标写入 CSV。
- 客户成功队列已补齐负责人工作台：`GET /dashboard/saasAdmin/customerSuccessOwners` 按当前队列筛选聚合负责人维度，返回负责人租户数、优先级分布、逾期/7 天内/阻断、账单跟进、运营任务、失败/耗尽通知、最高/平均健康分、最早下次跟进和 Top 租户；页面新增“客户成功负责人工作台”和“导出客户成功负责人 CSV”，`GET /dashboard/saasAdmin/export?type=customerSuccessOwners` 复用同一筛选导出。smoke 会断言普通租户访问和导出均为 `403`，平台管理员分派后 owner 汇总回显 `smoke-csm` 和目标租户。
- 客户成功队列支持批量分派：页面“客户成功批量分派”会按当前队列筛选调用 `POST/PUT /dashboard/saasAdmin/customerSuccessAssign`，为命中的租户逐条追加风险跟进，写入负责人、下次跟进时间和备注；smoke 会断言普通租户访问 `403`、平台管理员分派后返回目标租户操作 ID，并在风险跟进任务中继续回显。
- 客户成功队列支持批量生成续费任务：页面“客户成功续费任务”会按当前队列筛选调用 `POST/PUT /dashboard/saasAdmin/customerSuccessRenewalTasks`，默认沿用租户当前套餐、按当前到期日顺延 12 个月，并跳过已有待处理续费任务的租户；接口返回命中数、创建数、待应用/阻断数和跳过原因，创建结果进入统一运营任务中心，smoke 会断言普通租户访问 `403`、平台管理员生成续费任务后返回目标租户任务 request/preview 和启动日志。
- 客户成功队列支持批量生成续费提醒：页面“客户成功续费提醒”会按当前队列筛选调用 `POST/PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications`，写入现有 `mochat_go_saas_alert_notifications` outbox，通知 `alertType=tenant_renewal_reminder`、`metric=tenant_renewal`，默认同一租户、同一到期日、同一通道去重，`forceCreate=true` 可重置为 `pending` 重新投递；生成后会写入 `tenant.renewal.notify` 操作日志，并可继续通过通知列表、通知 CSV、重试和批量重试看投递状态。
- SaaS 总后台租户生命周期审计已接入 `GET /dashboard/saasAdmin/tenantLifecycle` 和页面“生命周期审计”：平台管理员可按 `tenantId`、`limit`、`source`、`eventType`、`status` 和 `keyword` 聚合租户详情、操作记录、账单事件、运营任务、告警、通知 outbox，并返回按时间倒序排列的统一 timeline；页面“生命周期筛选”可按来源、事件、状态和关键字定位单类事件，也可用“导出审计 CSV”通过 `GET /dashboard/saasAdmin/export?type=tenantLifecycle` 导出当前筛选后的时间线。运营待办认领日志 `saas.admin.operation_queue.assign` 会作为 `source=operation` 进入 timeline，并把认领 payload 的 `status` 派生为事件状态，因此可按 `source=operation&eventType=operation_queue.assign&status=contacted&keyword=负责人或备注` 回看任务 SLA、失败通知和关闭通知的队列级认领动作。smoke 会在套餐同步、平台开户、续费任务和账单事件产生后读取该接口，断言 timeline 同时包含 `operation/billing/task/alert/notification` 五类来源、各分区数据归属同一租户，并用 `source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=tenant_renewal_reminder` 定位续费提醒通知，再导出同筛选 CSV 断言租户、来源、状态、事件类型、标题和 payload，同时确认普通租户访问和导出均返回 `403`。
- SaaS 总后台账单对账复用账单筛选条件，`GET /dashboard/saasAdmin/billingReconciliation` 会比对续费账单和当前租户套餐；页面提供“账单对账”表格、“只看对账异常”开关、“跟进”按钮、“账单跟进任务”、“账单跟进负责人工作台”、“导出对账 CSV”、“导出账单跟进 CSV”和“导出账单负责人 CSV”。平台管理员可用 `POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUp` 对异常账单记录状态、负责人、下次跟进时间和备注，接口只追加 `billing.reconciliation.follow_up` / `billing_event` 操作日志，不改写账单或套餐事实；`GET /dashboard/saasAdmin/billingReconciliationFollowUps` 可按状态、负责人、到期状态和关键字查看每个账单事件最新跟进任务，`GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners` 可按负责人聚合打开、关闭和到期分布；`GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUps` 可按同口径导出账单跟进任务 CSV，包含账单事件、套餐、金额、订单号、到期状态、备注和操作 ID；`GET /dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners` 可按同口径导出账单负责人 CSV，包含负责人任务总数、打开/关闭数量、状态分布、到期分布、最近跟进和下次跟进时间；smoke 会验证正常续费订单为 `matched`，插入模拟漂移账单断言 `mismatchOnly=1` 返回 `package_mismatch` 和 `expires_mismatch`，再记录异常跟进并从操作日志、账单跟进任务和负责人工作台回看 before/after JSON、任务状态和负责人汇总，同时用 `GET /dashboard/saasAdmin/export?type=billingReconciliation`、`type=billingReconciliationFollowUps` 和 `type=billingReconciliationFollowUpOwners` 导出异常对账、账单跟进与账单负责人 CSV。
- SaaS 总后台账单跟进任务已支持批量关闭：页面“批量关闭账单跟进”会按当前跟进状态、负责人、到期状态、关键字和租户筛选调用 `POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose`，将命中的未关闭任务置为 `resolved/ignored` 并逐账单事件追加操作日志；smoke 会先把模拟漂移账单重新打开为 `contacted`，再批量关闭并通过任务列表、操作记录和普通租户 `403` 断言验证闭环。
- SaaS 总后台页面还提供统一“运营任务中心”，把套餐同步、平台开户和续费任务聚合到同一列表，可按任务类型、状态、租户 ID 和套餐筛选；任务中心标题会展示不受列表 `limit` 影响的任务总数、待应用、阻断、失败、已应用和可处理数量；“任务负责人”表复用同一筛选读取 `GET /dashboard/saasAdmin/taskOwners`，按操作人聚合任务量、可处理/阻断/失败/已应用分布、三类任务分布和最近任务；“任务SLA”表复用同一筛选读取 `GET /dashboard/saasAdmin/taskSla`，按 `warningHours` 和 `overdueHours` 把 `pending/blocked/failed` 活跃任务分为正常、预警和逾期；运营日报也会默认用 4/24 小时阈值展示“日报任务 SLA”并在日报 CSV 输出 `taskSlaOwner` 和 `taskSlaTask`，也会展示“日报待办认领”并在日报 CSV 输出 `operationQueueAssignment`；运营待办认领记录可按 `dueState=overdue/due_soon/future/no_date/closed` 筛选和导出，页面默认 `currentOnly=true` 只看每个待办的最新认领，切换“全部历史”可追溯历次负责人；认领记录行内“完成/忽略”会调用 `POST /dashboard/saasAdmin/operationQueueAssignmentClose`，关闭后解除该认领对待办负责人的覆盖并追加前后状态审计；页面“生成认领到期提醒”会调用 `POST /dashboard/saasAdmin/operationQueueAssignmentNotifications`，只把最新的逾期或 7 天内认领写入通知 outbox，历史负责人和已关闭认领不会被催办；页面“生成SLA提醒”会调用 `POST /dashboard/saasAdmin/taskSlaNotifications`，默认把预警和逾期任务写入通知 outbox，并继续联动通知列表、通知 CSV、重试和生命周期审计；smoke 会在 blocked 任务创建后调用 `GET /dashboard/saasAdmin/taskSla?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10`、`GET /dashboard/saasAdmin/export?type=taskSla&taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=1000`、`GET /dashboard/saasAdmin/dailyReport`、`GET /dashboard/saasAdmin/export?type=dailyReport`、`POST /dashboard/saasAdmin/taskSlaNotifications?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10`、`POST /dashboard/saasAdmin/operationQueueAssignmentNotifications?source=task_sla&dueState=overdue` 和 `POST /dashboard/saasAdmin/operationQueueAssignmentClose`；关闭后再次催办会断言 `matchedCount=0`，并在三类任务应用后调用 `GET /dashboard/saasAdmin/tasks?taskType=all&status=applied&limit=50` 和 `GET /dashboard/saasAdmin/taskOwners?taskType=all&status=applied&limit=20`，确认统一列表同时包含三类 `applied` 任务、负责人聚合回显平台操作人，断言 `summary`、`returnedCount`、状态分布和类型分布，并继续校验平台开户任务不回显明文密码或 `adminPasswordHash`。
- 统一运营任务中心支持 `GET /dashboard/saasAdmin/export?type=tasks` 导出任务 CSV，也支持 `GET /dashboard/saasAdmin/export?type=taskSla` 导出活跃任务 SLA CSV，复用 `taskId`、`taskType`、`status`、`tenantId`、`packageCode`、`limit`、`warningHours` 和 `overdueHours` 筛选；任务 CSV 包含任务请求、预览和结果，SLA CSV 额外包含负责人、SLA 状态、任务年龄和超时小时数，平台开户任务请求导出时仍会去除明文密码和 `adminPasswordHash`。
- 统一运营任务中心支持 `POST/PUT /dashboard/saasAdmin/taskCancel` 取消未应用任务；页面行内会对 `pending/blocked/failed` 任务显示“取消”，取消后任务状态变为 `canceled`，不会再允许应用。页面也支持 `POST/PUT /dashboard/saasAdmin/taskBulkCancel` 按当前任务筛选批量取消最多 100 条未应用任务，接口会跳过已应用和已取消任务并返回跳过数；`POST/PUT /dashboard/saasAdmin/taskBulkReset` 可按当前任务筛选批量把 `failed/blocked` 任务重置为 `pending`，跳过待应用、已应用和已取消任务；`POST/PUT /dashboard/saasAdmin/taskReset` 可把单条 `failed/blocked` 任务重置为 `pending` 以便重新应用；`POST/PUT /dashboard/saasAdmin/packageSyncTaskBulkApply` 可按当前任务筛选批量应用套餐同步任务，逐条重算超额风险后写入租户套餐快照和用量额度、阻断或失败结果；`POST/PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply` 可按当前任务筛选批量应用续费任务，逐条复核当前租户和套餐状态后写入账单、阻断或失败结果；`POST/PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply` 可按当前任务筛选批量应用平台开户任务，逐条复核套餐状态后创建租户和管理员、写入套餐快照、用量计数、开通记录和任务结果。任务创建、应用、阻断、取消、批量取消、批量重置和重置会写入 `admin_task` 操作记录。任务行内“追溯”按钮会自动把操作记录筛选设为 `targetType=admin_task` 和当前任务 ID，smoke 会分别通过 `operations?action=saas.admin.task.cancel&targetType=admin_task`、`operations?action=saas.admin.task.bulk_cancel&targetType=admin_task`、`operations?action=saas.admin.task.bulk_reset&targetType=admin_task` 和 `operations?action=saas.admin.task.reset&targetType=admin_task` 追溯任务动作，并校验操作记录 `before.status` 与 `after.status`。
- SaaS 总后台操作记录表支持展开 before/after JSON 变更详情；配合任务行内“追溯”按钮，平台管理员可以直接在页面核对运营任务创建、阻断、应用、取消、批量取消和批量重置的前后状态。
- `scripts/smoke_saas_tenant_isolation.sh` 会启动真实 MySQL/Redis/Go standalone，批量开通两个租户并种入各自企业、员工、部门和首页数据，验证超级管理员不能 `corp/bind` 其他租户企业、拒绝后不会写 Redis 企业选择缓存；脚本还会主动污染 `mc:user.{userId}` 为其他租户企业，断言 `loginShow`、`corpData/index` 和 `workEmployee/searchCondition` 会回退到当前租户企业；绑定本租户企业后 `corp/select`、`corpData/index`、`workEmployee/searchCondition`、`workDepartment/index` 和 `workEmployee/index` 只返回当前租户数据，并继续验证 `role/index` 只返回当前租户角色、`permissionByUser` 可正常返回当前租户权限菜单、两个租户分别上传文件后 `mochat_go_saas_storage_objects` 与 `storage_mb` 用量只归属各自租户。
- `scripts/smoke_saas_quota_enforcement.sh` 会启动真实 MySQL/Redis、fake 企业微信/微信开放平台 API 和 Go standalone，登录额度已满租户后分别调用企业、子账号、应用、渠道活码、门店活码、互动雷达、抽奖活动、无限拉群、群裂变、群打卡、群质检、群日历、客户群提醒、个人 SOP、群 SOP、敏感词、客户同步、客户群同步、客户群发、客户群群发、标签建群、自动拉群、裂变活动、公众号授权和上传写入口，验证这些资源达到套餐上限时返回 400 且业务表或上传目录不会新增记录。
- `scripts/smoke_admin_core_dashboard.sh` 会启动真实 MySQL/Redis/Go standalone，登录并绑定企业后覆盖 `WW_verify_*.txt` 校验文本、管理端基础写链路：`common/uploadFile`、`sidebar/common/upload`、素材分组和素材移动、客户群分组、客户画像字段新增/更新/状态/批量/删除、菜单新增/更新/状态/删除、角色新增/更新/授权/状态/删除、子账号新增/详情/更新/状态/重置密码和当前账号改密，并用 MySQL 表状态和上传文件账本断言副作用。
- `scripts/smoke_work_contact_tag_remote_write.sh` 会启动真实 MySQL/Redis、fake 企业微信 API 和 Go standalone，覆盖客户标签组新建/改名/删除、客户标签新建/改名/移动/删除，并断言本地表与企业微信标签 API 副作用。
- `cmd/mochat-saas-maintenance -action reconcile-storage` 可按上传存储根目录扫描 `mochat_go_saas_storage_objects`，把丢失文件和危险路径标记为软删除，修正漂移的 `size_bytes`，并刷新受影响租户的 `storage_mb` 用量；设置 `MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON=1` 后，Go standalone 会启动 `cron-saas-storage-reconcile` 定时任务。`scripts/smoke_saas_storage_reconcile.sh` 会用真实 MySQL 同时验证手动命令和 run-on-start 定时任务闭环。
- `PUT /dashboard/medium/update` 替换素材文件路径时会回收旧路径账本，`DELETE /dashboard/medium/destroy` 软删素材时会回收当前路径账本，路径字段包括 `imagePath`、`voicePath`、`videoPath` 和 `filePath`；`POST /dashboard/channelCode/store` 在企业微信 contact_way 创建失败回滚时会解析 `welcome_message.messageDetail`，回收 `pic_url`、`imagePath`、`linkPic`、`coverPath` 等本地欢迎语素材账本；`PUT /dashboard/shopCode/update` 和 `POST /dashboard/shopCode/updateQrcode` 替换或移除门店活码 `employee_qrcode`、`qw_code` 本地二维码时会回收旧路径账本，`POST /dashboard/shopCode/pageSet` 替换或移除页面设置 `default` / `poster` 中的本地二维码或海报时会回收旧路径账本，`DELETE /dashboard/shopCode/destroy` 软删门店活码时会回收当前二维码账本；`PUT /dashboard/radar/update` 替换互动雷达 `link_cover` 或 `pdf` 本地文件时会回收旧路径账本，`DELETE /dashboard/radar/destroy` 软删互动雷达时会回收当前雷达封面和 PDF 文件账本；`PUT /dashboard/lottery/update` 替换抽奖活动奖品、兑奖、限制或企业名片 JSON 中的本地素材时会回收旧路径账本，`DELETE /dashboard/lottery/destroy` 软删抽奖活动时会回收奖品设置和中奖记录客服二维码里的本地素材账本；`PUT /dashboard/roomClockIn/update` 替换群打卡 `employee_qrcode` 本地二维码时会回收旧路径账本，`DELETE /dashboard/roomClockIn/destroy` 软删群打卡时会回收当前领奖客服二维码账本；`PUT /dashboard/roomInfinitePull/update` 替换无限拉群 `avatar`、`logo` 或 `qw_code` 活码 JSON 中的本地二维码时会回收旧路径账本，`DELETE /dashboard/roomInfinitePull/destroy` 软删无限拉群时会回收当前头像、logo 和活码二维码账本；`PUT /dashboard/roomWelcome/update` 和 `DELETE /dashboard/roomWelcome/destroy` 会回收入群欢迎语 `msg_complex.pic` 本地图片账本；`DELETE /dashboard/contactMessageBatchSend/destroy` 和 `DELETE /dashboard/roomMessageBatchSend/destroy` 会回收群发内容里的本地 `pic_url` 图片账本；`DELETE /dashboard/contactBatchAdd/importDestroy` 会回收批量加好友导入记录 `file_url` 对应的本地原文件账本；`DELETE /dashboard/roomTagPull/destroy` 会回收标签建群 `rooms.image` 本地图片账本；`PUT /dashboard/roomFission/update` 替换群裂变海报、群二维码或欢迎语时会回收旧本地路径，`POST /dashboard/roomFission/invite` 替换群裂变邀请封面时会回收旧邀请图账本，`DELETE /dashboard/roomFission/destroy` 会在级联软删群裂变活动时回收 poster、room、welcome、invite 里的本地图片账本；`PUT /dashboard/workRoomAutoPull/update` 替换或移除自动拉群 `rooms.roomQrcodeUrl` 本地群二维码时会回收旧路径账本，`POST /dashboard/workRoomAutoPull/store` 在企业微信 contact_way 创建失败回滚时也会回收已写入记录里的本地群二维码；`DELETE /dashboard/workFission/destroy` 会在级联软删裂变活动时回收 poster、welcome、push、invite 里的本地图片账本。`scripts/smoke_saas_storage_reclaim.sh` 会用真实 Go standalone 验证这些触发式回收闭环。
- `cmd/mochat-saas-maintenance -action refresh-usage` 可按当前业务表和租户套餐配置重刷企业数、子账号数、客户数、客户群数、应用数、渠道活码数、门店活码数、互动雷达数、抽奖活动数、无限拉群数、群裂变数、群打卡数、群质检规则数、群日历数、客户群提醒数、个人 SOP 规则数、群 SOP 规则数、敏感词词库数、素材存储、客户群发任务数、客户群群发任务数、标签建群任务数、自动拉群活码数、裂变活动数、公众号授权数和异步执行量 26 个 lifetime 用量指标；`scripts/smoke_saas_usage_refresh.sh` 会制造脏计数器、缺失指标和套餐额度变化，并验证 `used_value`、`limit_value`、`updated_by` 都被修正。
- `cmd/mochat-saas-maintenance -action cleanup-service-account-usage` 按合规策略中的保留天数分批删除过期 OpenAPI 日聚合行，并报告候选、法律保留保护、删除和剩余行数。`-action evaluate-service-account-usage-alerts` 可按账号或批次立即评估用量和拒绝预警。设置对应 cleanup/alert cron 可分别启动每日清理和周期预警；`scripts/smoke_saas_service_accounts.sh` 使用真实 MariaDB 验证维护命令、cron 执行账本、冷却去重、通知 outbox、自动恢复和法律保留保护。
- `scripts/smoke_official_account_ticket.sh` 会启动真实 MySQL/Redis/Go standalone 和 fake 微信开放平台 API，先覆盖微信开放平台 AES 加密 GET `echostr` URL 校验，再以 AES 加密回调形式向 `authEventCallback` 推送 `component_verify_ticket`，随后在不设置静态 `MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET` 的情况下调用 `getPreAuthUrl`，并继续验证 `authRedirect` 调用 `api_query_auth` 和 `api_get_authorizer_info` 写入授权公众号资料、租户归属、`official_accounts` 用量、列表头像静态 URL、模块默认绑定和 `officialAccount/set` 手动绑定；之后投递 AES 加密的开放平台测试消息 `QUERY_AUTH_CODE`，验证 Go 会验签、解密、从 `mochat_go_wechat_component_tickets` 读取 ticket，完成预授权 URL、`api_query_auth`、`api_authorizer_token` 和客服文本发送闭环；同一 smoke 也会断言 AES 回调缺失或伪造 `msg_signature` 时返回 400，且不会覆盖 ticket 或触发客服消息，并验证 `unauthorized` 取消授权回调会把公众号从列表隐藏、刷新 `official_accounts` 用量。
- `cmd/mochat-saas-maintenance -action list-alerts` 可按租户、指标和状态列出 `mochat_go_saas_alerts` SaaS 告警，`-action resolve-alert -tenant-id <id> -metric <metric>` 可把当前打开告警标记为 resolved；`-action dispatch-alert-notifications` 会读取 `mochat_go_saas_alert_notifications` 的 pending/failed 到期通知，优先使用 `mochat_go_saas_alert_settings` 的租户级 webhook URL、签名、模板、HTTP/outbox retry、事件订阅、最低严重级别、免打扰和每小时限流配置，未配置租户项时再使用全局环境变量兜底。租户 Webhook URL 与 Secret 在 0066 后可由独立 AES-256-GCM 密钥环加密落库，`-action rotate-alert-credentials` 可按租户批量清理旧明文并重加密历史 Key。策略拒绝写为 `suppressed`；免打扰或限流保持 `pending` 并推迟 `next_retry_at`，不消耗发送尝试次数；维护命令和 cron 输出 delivered、deferred、suppressed、failed、dead。设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON=1` 后，Go standalone 会启动 `cron-saas-alert-notification-dispatch`，按间隔自动扫描并重发 outbox 到期通知。设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1` 后，租户超管还可访问 Go 自带的 `GET /dashboard/saasAlert/page` 告警管理页面，通过 `GET /dashboard/saasAlert/index` 查看本租户告警，通过 `PUT/POST /dashboard/saasAlert/resolve` 解决当前打开告警，并通过 `GET/PUT/POST /dashboard/saasAlert/setting` 查看和保存本租户通知策略。设置 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL` 后，worker 在租户未配置 webhook 时仍可用全局兜底发送 `saas.quota_alert` JSON webhook；设置 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET` 后会附带 HMAC-SHA256 签名头。`scripts/smoke_employee_apply_worker.sh` 会验证异步执行量超额时写入告警账本、outbox 标记送达、fake webhook 首次 502 后重试成功、模板化标题正文、真实 JWT 可查询 dashboard 告警列表、Go 自带告警页面可访问、维护命令可列出和解决该告警，并验证维护命令可重发 outbox 到期通知；`scripts/smoke_saas_alert_notification_cron.sh` 会验证内置 cron 可自动重发 pending 通知并写入后台任务执行历史；`scripts/smoke_saas_alert_setting_dispatch.sh` 会验证签名投递、策略控制、SSRF 防护、明文兼容、密文投递、历史密钥缺失拒绝和跨 Key 轮换。
- `0067_wecom_credential_encryption` 将 `mc_corp` 的企业 Secret、通讯录 Secret、回调 Token/AES Key、会话存档 Secret 和 `mc_work_agent` 的应用 Secret 作为 AES-256-GCM 密文保存。总后台 `wecomCredentialProtection` 只返回保护统计和非敏感 Key ID，`wecomCredentialRotation` 按平台集成治理 RBAC 轮换并写脱敏审计；维护动作 `rotate-wecom-credentials` 支持按租户分批轮换。`scripts/smoke_wecom_credential_encryption.sh` 使用真实 JWT、MariaDB 和 Redis 验证明文兼容、明文清空、历史 Key 缺失关键告警、跨 Key 重加密、新写入不落明文及只读角色拒绝。
- `0068_wechat_open_credential_encryption` 将组件 Ticket、组件 Secret/Token/AES Key、公众号授权码、预授权码和 `authorizer_refresh_token` 作为 AES-256-GCM 密文保存。总后台 `wechatOpenCredentialProtection` 只返回保护统计和非敏感 Key ID，`wechatOpenCredentialRotation` 按平台集成治理 RBAC 轮换并写脱敏审计；维护动作 `rotate-wechat-open-credentials` 支持按租户分批轮换。`scripts/smoke_official_account_ticket.sh` 使用真实 JWT、MariaDB、Redis 和 fake 微信开放平台覆盖旧明文轮换、加密回调写入、授权刷新、缺历史 Key 安全失败、跨 Key 重加密及解密后字段一致性。
- `0069_saas_release_candidate_approval` 将发布候选门禁纳入高风险审批中心。发布运营只能提交携带当前构建指纹的申请，默认需两名不同审批人会签且 6 小时内执行；执行器重新下载并校验六类生产证据，候选写入与审批副作用操作 ID 在同一数据库事务提交，单人直连接口返回 `428 Precondition Required`。
- `0070_saas_approval_policy_change_guard` 将审批策略的启停、会签人数、SLA、提醒和有效期变更纳入固定双人会签。治理策略自身必须保持启用且不得低于两人会签；审批执行、策略更新、操作审计与副作用标记在同一事务提交，关闭任一业务高风险门禁也不能由单个管理员直接完成。
- `0071_saas_backup_policy_change_guard` 将平台数据库备份策略变更纳入固定双人会签。备份启停、加密、异地副本、执行频率和保留参数不再允许单个管理员直接修改；审批执行、策略更新、操作审计与副作用标记在同一事务提交。
- `0072_saas_backup_cleanup_saga` 将备份保留清理纳入固定双人会签。申请阶段由服务端冻结策略版本、截止时间和候选备份快照；批准后创建持久化清理任务，按“异地副本、本地工件、数据库记录”顺序保存检查点。失败任务保留原冻结范围并支持续跑，清理中的备份禁止校验、复制和恢复；总后台展示任务进度与失败原因，系统健康中心持续检查积压和过期租约。
- `0073_saas_compliance_export_deletion_saga` 将未到保留期的合规导出删除纳入不可关闭的双人会签。申请由服务端冻结导出归属、工件名、SHA-256、字节数和保留期；执行前重新校验法律保留、未终结擦除引用和冻结快照，再按“加密工件、数据库台账”保存检查点。失败任务保留证据和错误，可从总后台重试；维护任务只自动恢复待执行或租约过期的已批准任务。
- `0074_saas_compliance_legal_hold_release_guard` 将解除法律保留纳入不可关闭的双人会签。申请由服务端冻结保留编号、租户、状态、保留原因、起止时间、版本和解除依据；执行时在同一事务内锁定并复核快照、解除保留、写入操作审计和审批副作用标记，阻止单个管理员绕过合规导出删除与租户擦除门禁。
- `0075_saas_compliance_policy_change_guard` 将租户数据合规生命周期策略变更纳入不可关闭的双人会签。直接写接口返回 428；申请校验并冻结导出、擦除、账务、审计和服务账号用量保留参数及当前版本，执行时将策略更新、操作审计和审批副作用标记原子提交。
- `0076_saas_identity_policy_change_guard` 将租户身份安全策略变更纳入不可关闭的双人会签。直接写接口返回 428；申请校验并冻结密码锁定、会话、MFA、登录 IP 白名单、身份数据保留参数及当前版本，执行时将策略更新、操作审计和审批副作用标记原子提交。
- `0077_saas_tenant_disable_approval_guard` 将 critical 级租户停用门禁固定为至少两名不同复核人会签。治理接口不能关闭或降到单人；总后台显示强制启用，既有租户状态、订阅同步、操作审计与审批副作用继续在同一事务提交。
- `0078_saas_critical_approval_policy_guard` 将全部 13 条 critical 审批策略统一固定为启用、零绕过阈值和至少两人会签，并把退款升级为全额双人审批；后端会钳制历史不安全配置，总后台由服务端治理元数据控制可编辑边界。
- `0079_saas_service_account_key_revoke_guard` 将服务账号 API Key 吊销纳入不可绕过的 critical 双人会签。直接吊销返回 `428`；申请冻结服务账号、Key 与当前版本，批准执行时把 Key 失效、操作审计和审批效果标记原子提交，总后台要求填写原因并提交吊销审批。
- `0080_saas_service_account_update_guard` 将服务账号配置变更纳入不可绕过的 critical 双人会签。直接更新返回 `428`；申请冻结账号状态、作用域、CIDR、额度、预警、有效期与当前版本，批准执行时把账号更新、操作审计和审批效果标记原子提交，总后台编辑入口改为提交修改审批。
- `0081_saas_service_account_key_rotate_guard` 将服务账号 API Key 轮换纳入不可绕过的 critical 双人会签。申请只冻结账号版本、Key 名称、有效期和旧 Key 宽限期，不生成或持久化新 Key；批准执行时才生成新 Key，并把轮换、操作审计和审批效果标记原子提交。明文只在执行响应显示一次，审批结果仅保存已交付标记。
- `0082_saas_service_account_create_guard` 将服务账号创建纳入不可绕过的 critical 双人会签。直接创建返回 `428`；申请冻结标准化账号配置、首个 Key 名称和有效期，但不生成密钥；批准执行时才生成首个 Key，并把账号、Key、操作审计和审批效果标记原子提交。明文只在执行响应显示一次，审批结果仅保存已交付标记。
- `0083_saas_identity_mfa_reset_guard` 将平台重置用户 MFA 纳入不可绕过的 critical 双人会签。申请冻结目标用户、租户和凭据版本；批准执行时原子禁用 MFA、清除认证材料、撤销活动会话、写安全事件和操作审计并标记审批效果。
- `0084_saas_package_definition_guard` 将平台套餐创建、启停、说明和全部额度变更纳入不可绕过的 critical 双人会签。申请冻结完整定义、当前版本和租户影响；批准执行时原子写入套餐、操作审计和审批效果。
- `0085_saas_tenant_package_assignment_guard` 将租户套餐分配纳入不可绕过的 critical 双人会签。绑定使用独立版本，申请冻结租户状态、当前绑定、目标套餐定义版本和额度影响；批准执行时原子更新绑定、同步订阅、写操作审计并标记审批效果。续费、平台开户和套餐同步会递增绑定版本，阻止陈旧审批覆盖新状态。
- `0086_saas_tenant_provision_approval_guard` 将平台开户纳入不可绕过的 critical 双人会签。申请冻结套餐定义、开户预览、运营任务版本和任务请求摘要；密码仅以 PHP 兼容 bcrypt 哈希持久化，API 不返回明文、哈希或凭据指纹。批准执行时原子创建租户、管理员、角色菜单、套餐、订阅和开通记录，并同步提交任务状态、操作审计与审批效果。
- `0087_saas_tenant_renewal_approval_guard` 将租户续费纳入不可绕过的 critical 双人会签。直接续费、单任务直接应用和批量直接应用均返回 `428`；申请冻结租户状态、当前套餐绑定版本、目标套餐版本、订阅存在性与版本，以及任务版本和请求摘要。批准执行时原子提交套餐权益、续费账单、订阅、任务状态、操作审计和审批效果；审批后的用量刷新失败只标记待补偿，不把已提交续费误报为失败。
- `0088_saas_subscription_transition_approval_guard` 将租户订阅状态迁移纳入不可绕过的 critical 双人会签。直接迁移返回 `428`；申请冻结租户状态、订阅 ID 与版本、目标状态、周期字段、取消标记和规范化原因。批准执行时原子提交订阅状态、订阅事件、操作审计与审批效果标记；平台租户、租户状态漂移和订阅版本漂移均会拒绝执行，自动对账仍保留服务端派生状态的直接修复链路。
- `0089_saas_invoice_issue_approval_guard` 将蓝票和红票的正式开具纳入不可绕过的 critical 双人会签。直接推进到 `issued` 返回 `428`；申请冻结单据身份、状态、版本、票面快照，以及支付订单版本和退款、开票、红冲金额台账。批准执行时原子更新单据、调整订单金额、写操作审计并标记审批效果；单据或订单漂移会拒绝执行，`processing/failed/canceled` 仍保留财务直接纠错链路。
- `0090_saas_payment_order_create_approval_guard` 将收款订单创建纳入不可绕过的 critical 双人会签。直接创建返回 `428`；申请冻结有效租户、套餐定义版本、套餐额度、金额、服务周期和收银台有效期，批准执行时重新锁定并校验租户与套餐版本，再把订单、操作审计和审批效果标记原子提交。订单持久化套餐版本与额度快照，后续套餐定义变化不会改变该订单结算时实际授予的权益。
- `0091_saas_payment_settlement_close_guard` 将渠道结算批次关账纳入不可绕过的 critical 双人会签。直接关账返回 `428`；申请冻结批次编号、账期、渠道、币种、状态、来源摘要、对账计数、金额台账、版本和备注，批准执行时重新锁定并完整校验，只允许关闭无未解决差异的已对账批次，再把关账、操作审计和审批效果标记原子提交。
- `0092_saas_payment_settlement_reopen_guard` 将已关账批次重开纳入独立、不可绕过的 critical 双人会签。直接重开返回 `428`；申请冻结完整批次账务、导入/对账元数据及关账人、关账时间、关账原因，批准执行时重新锁定并完整校验，再原子恢复为已对账状态、写操作审计并标记审批效果。新版冻结载荷带 `schemaVersion=2`，同时兼容 0091 已发起的旧关账审批。
- `0093_saas_payment_settlement_resolve_guard` 将结算差异的解决、忽略和重新打开纳入不可绕过的 critical 双人会签。直接处理返回 `428`；申请使用一致性事务冻结完整差异条目及所属批次账务，批准执行时按固定锁序完整复核，再把条目状态、批次差异计数、操作审计和审批效果原子提交。审批期间任一条目、账务或处理审计漂移都会返回 `409`。
- `0094_saas_tenant_domain_command_guard` 将主域名切换、域名启停、DNS 校验令牌轮换和域名删除纳入不可绕过的 critical 双人会签。直接操作返回 `428`；申请冻结目标域名与租户全部未删除域名的路由快照，批准执行时按固定顺序锁定并逐项校验，再原子提交域名状态、交付任务、操作审计和审批效果。轮换令牌只在批准执行时生成，不进入审批载荷。
- `0095_saas_tenant_domain_create_guard` 将租户域名新增纳入不可绕过的 critical 双人会签。直接新增返回 `428`；申请只冻结标准化后的租户和域名，不生成令牌，批准执行时重新锁定并复核租户状态、域名唯一性和 10 个域名配额，才生成一次性交付的 DNS 校验令牌，并把域名、初始交付状态、操作审计和审批效果原子提交。持久化审批结果不保存令牌明文。
- `0096_saas_tenant_enable_approval_guard` 将已停用业务租户的重新启用纳入不可绕过的 critical 双人会签。直接启用返回 `428`；申请冻结租户名称、状态及订阅 ID、状态和版本，批准执行时重新锁定并完整复核，再把租户恢复、订阅同步、操作审计和审批效果原子提交。任一快照漂移返回 `409`，恢复快照后可重试同一审批。
- SaaS 总后台的“租户通知策略”已把租户自助配置提升为平台运维工作台：平台超管可查看已启用、已停用和未配置租户，代管 Webhook URL、Secret、模板、两级重试、事件订阅、最低严重级别、免打扰与小时限流，并生成不绕过 outbox 的测试通知。策略读取、保存、测试、密钥脱敏和 `suppressed` 结果均已加入 `scripts/smoke_saas_admin_dashboard.sh`，保存与测试分别写入 `tenant.notification_policy.update` 和 `tenant.notification_policy.test` 审计日志。
- SaaS 总后台“通知送达健康度”按时间窗口把 outbox 聚合到租户维度：重试耗尽、超过积压阈值或足够样本下成功率低于 80% 标为严重，失败待重试、到期待投递或成功率低于 95% 标为预警；策略抑制和未来延期单独统计，不误算为真实发送失败。页面支持窗口、状态、积压阈值和租户搜索，并展示 Top 失败原因及同口径 CSV。
- `scripts/smoke_saas_notification_slo.sh`、订阅、收款、退款、发票和结算专项 smoke 均从独立空库执行当前 96 个迁移，使用真实 Go + MySQL/MariaDB + Redis 保持原有幂等、租户隔离、事务、审计、CSV、页面和 cron 验收。
- `scripts/smoke_saas_payment_settlement_sync.sh` 会使用真实 Go + MySQL + Redis 和 fake Bridge 验证启动即同步、Bearer/查询协议、游标推进、重复批次幂等、dry-run 不写入、同渠道并发 `409`、失败运行、差异/失败通知 outbox、task runner 持久化、平台/租户权限、总后台页面和运行时路由。
- `scripts/smoke_saas_admin_access_rbac.sh` 会从独立空库执行 96 个迁移，验证六个内置角色、平台集成治理等权限依赖、运营可管理、审计只读、财务与业务租户越权拒绝。
- `scripts/smoke_saas_admin_approvals.sh`、`scripts/smoke_saas_admin_approval_governance.sh`、`scripts/smoke_saas_package_definition_approval.sh`、`scripts/smoke_saas_tenant_package_assignment_approval.sh`、`scripts/smoke_saas_tenant_provision_approval.sh`、`scripts/smoke_saas_tenant_renewal_approval.sh`、`scripts/smoke_saas_subscription_transition_approval.sh`、`scripts/smoke_saas_invoice_issue_approval.sh`、`scripts/smoke_saas_payment_order_create_approval.sh`、`scripts/smoke_saas_payment_settlement_close_approval.sh`、`scripts/smoke_saas_payment_settlement_reopen_approval.sh`、`scripts/smoke_saas_payment_settlement_resolve_approval.sh`、`scripts/smoke_saas_tenant_domain_approval.sh` 与发布准备 smoke 在 96 个迁移上验证 31 类高风险策略全部为 critical 双人会签，以及租户停用后重新启用的租户/订阅快照漂移保护、域名新增执行期令牌生成、租户/唯一性/配额复核、完整路由快照漂移保护、平台开户密码脱敏、续费任务与订阅版本漂移、订阅状态迁移冻结与漂移保护、发票开具单据及订单账务冻结与漂移保护、收款订单套餐快照与结算权益冻结、结算关账、重开、差异处理、任务版本冲突、策略变更自保护、备份策略、合规生命周期策略、保留清理、合规导出提前删除、法律保留解除、服务账号配置变更、Key 创建/轮换/吊销、MFA 重置、套餐定义、租户套餐分配和续费保护、委托、SLA、业务副作用事务标记和恢复幂等。
- `scripts/smoke_saas_admin_system_health.sh` 验证身份安全关闭时的 24 项基础健康检查与 10 类事故聚合；审计签名锚点、域名交付队列和 TLS 生命周期作为独立检查。启用身份安全后增加配置探针，配置对象存储或域名 Bridge 后分别增加连通性或配置探针。
- `scripts/smoke_saas_audit_anchor.sh` 使用真实 Go + MariaDB + Redis 验证 legacy 锚点、租户摘要链、HMAC 签名检查点、独立证据文件、历史密钥状态、启动即跑 cron、读写 RBAC、文件篡改、孤儿证据、恢复校验、维护命令、页面和运行时路由。
- `scripts/smoke_saas_service_accounts.sh` 与 `scripts/smoke_saas_backup_recovery.sh` 均执行 96 个迁移，继续验证独立 pepper 与旧 JWT pepper 切换、Key 生命周期、服务账号创建/配置变更、API Key 创建/轮换/吊销双人审批、一次性明文交付、原子限流、429 响应头、路由用量、异地副本、回源、隔离恢复，以及审批后可续跑的保留清理 Saga。
- `scripts/smoke_saas_compliance_lifecycle.sh` 使用真实 Go + MariaDB + Redis 验证 96 个迁移、158 项租户数据清单覆盖、合规策略变更双人审批与原子生效、加密导出/解密/篡改拒绝、合规导出提前删除与法律保留解除的双人审批、法律保留与未终结擦除阻断、工件删除失败检查点和重试恢复，并继续覆盖精确确认、财务保留期阻断、161 步可恢复擦除、物理文件删除、脱敏审计、墓碑和维护命令。摘要链、校验历史和签名检查点按审计留存策略保留。
- `scripts/smoke_saas_identity_security.sh` 使用真实 Go + MariaDB + Redis 验证 96 个迁移、第三次失败触发锁定、管理员解锁、IP/CIDR 门禁、TOTP 与一次性恢复码、挑战和动态码防重放、并发会话淘汰、单会话撤销、登出失效、事件/事故、审计脱敏和保留清理。
- `scripts/smoke_saas_branding.sh` 使用真实 Go + MariaDB + Redis 验证 96 个迁移、品牌查看/管理 RBAC、乐观锁、审计、租户回显、安全登录页、本地资产 URL 限制和旧 `api.mo.chat`/`oss.mo.chat` 运行时依赖清理。
- `scripts/smoke_saas_tenant_domains.sh` 使用真实 Go + MariaDB + Redis 与本地权威 DNS TXT 服务验证审批关闭时的兼容直写、域名唯一绑定、所有权校验、主域名、停用/启用、令牌轮换、RBAC、审计，以及按 Host 绑定登录品牌、账号租户和 MFA 挑战；`scripts/smoke_saas_tenant_domain_approval.sh` 在审批开启时验证新增与五类路由操作的直接阻断、双人复核、租户/唯一性/配额复核、完整路由快照漂移、执行期令牌生成、持久化脱敏和事务原子性。
- 通知健康度已接入统一运营待办：`warning/critical` 租户会成为 `notification_health` 来源，页面可直接填写负责人和复查时间后分派；负责人工作台、认领记录、认领提醒、运营日报和 CSV 复用现有审计链。`notificationHealthRecovery` 只对重新核验为 `healthy` 且当前窗口至少有一次成功送达的认领自动结案，`no_data` 或只有抑制、关闭、延期的租户不会被误判为恢复。
- `0047_saas_admin_approval_governance` 将审批升级为持久化策略；`0052` 在原五类动作上增加 `tenant.data.erase`，共六类，擦除默认两人复核。申请使用策略快照，逐票只追加一次，达到法定票数后才允许执行。
- `0048_saas_admin_system_health` 新增健康扫描和系统事故台账，以稳定检查键聚合反复异常，保留首次/最近检测、发生次数、严重度、负责人、处置结论和乐观锁版本。`platform.system.read` 控制查看，`platform.system.manage` 控制扫描和事故处置；恢复检查自动结案，人工解决但仍异常的事故会在下次扫描重开。
- `0049_saas_service_accounts` 新增租户服务账号与 API Key 台账。`platform.integrations.read/manage` 分离查看与管理；Key 格式为 `mch_live_<12 hex>_<43 base64url>`，数据库只保存 HMAC-SHA256 摘要、非敏感前缀和后四位，明文只在创建或轮换响应中显示一次。
- `0050_saas_backup_recovery` 新增全局备份策略、备份运行和隔离恢复演练台账。工件使用分块 AES-256-GCM 认证加密、gzip 压缩、SHA-256 完整性和原子发布；MySQL 客户端密码只写入 `0600` 临时 defaults 文件，不进入命令参数。`platform.backups.read/manage` 分离查看与管理，恢复只能写入服务端预配置的安全前缀空库。
- `0051_saas_backup_resilience` 在灾备台账上增加强制异地副本策略、S3 兼容对象位置与强校验状态、历史密钥可用性，以及恢复目标生命周期和清理结果。当前密钥负责新备份，密钥环保留历史密钥；本地损坏可从已验证副本回源，恢复演练可自动创建并销毁临时库。
- `0052_saas_compliance_lifecycle` 新增合规策略、法律保留、加密导出、可恢复擦除步骤与匿名墓碑。实际擦除使用静态数据清单及运行时未登记表门禁，保留必须存续的财务证据但移除租户识别和敏感载荷。
- `0053_saas_identity_security` 新增租户身份策略、账号安全状态、TOTP/恢复码凭据、二次认证挑战、可撤销会话、不可变登录事件和身份安全事故七张表，以及 `platform.identity.read/manage` 权限。MFA 密钥使用 AES-256-GCM 密钥环加密，挑战、会话 token 和恢复码只保存不可逆摘要。
- `0054_saas_branding_profiles` 新增平台与租户品牌档案，支持产品名、简称、副标题、本地 logo/favicon/登录背景、主色、支持与文档入口、页脚及乐观锁，并新增 `platform.branding.read/manage` 权限。`GET /dashboard/saasAdmin/brandingProfiles`、`GET/POST/PUT /dashboard/saasAdmin/brandingProfile` 只允许 HTTPS 对外链接与同源本地资产，同时保留 GPL-3.0 许可声明。
- `0055_saas_tenant_domains` 新增租户自定义登录域名、DNS TXT 所有权校验、主域名、停用/启用、校验令牌轮换、乐观锁和 `platform.domains.read/manage` 权限。只有已验证且启用的域名才能按 Host 选择租户品牌与租户内账号；已登记但未启用的域名直接返回 `421`，不会回落到平台默认租户。
- `0056_saas_domain_delivery` 新增域名路由/TLS 当前状态、持久化任务与幂等回调事件；通过外部 Bridge 交付 Ingress 和证书，Bearer 令牌只在出站请求头中使用，回调使用 HMAC-SHA256 签名、时间窗口和 `eventId` 幂等。总后台区分 DNS 验证、应用 Host 放行与真实路由/TLS 就绪状态，并可查看、刷新和重试交付任务。
- `0057_saas_release_readiness` 新增六类生产证据台账和不可变发布候选快照。总后台要求真实 HTTPS 证据地址、执行环境、复核人和源码 SHA-256 指纹；只有 6 项必需证据全部通过且与候选指纹一致时，严格门禁才生成 `ready` 候选。
- `0058_saas_release_evidence_integrity` 为每项生产证据固化实际工件 SHA-256 和字节大小，审计与候选快照同步保存，防止证据 URL 内容变化后仍被当作已通过。
- `0059_saas_service_account_rate_limits` 为服务账号增加每分钟/每日成功请求限额、原子消费窗口、429 与重试响应头，以及按自然日和规范化路由聚合的成功/拒绝统计；总后台可配置限额并查看实时用量。
- `0060_saas_service_account_usage_retention` 将服务账号用量保留天数纳入合规策略。总后台新增 7/30/90 天趋势、拒绝率、账号排行和路由排行；`cmd/mochat-saas-maintenance -action cleanup-service-account-usage` 与可选 `cron-saas-service-account-usage-cleanup` 按策略分批删除过期聚合行，活动法律保留租户不会被清理。
- `0061_saas_service_account_usage_alerts` 为每个服务账号增加预警开关、每日用量百分比阈值、限流拒绝阈值和通知冷却。手动接口、维护命令和可选 cron 复用统一 SaaS 告警与通知 outbox；重复评估遵守冷却，条件消失后自动解决告警、关闭尚未送达的旧通知并重置通知时间。
- `0062_saas_audit_integrity` 为总后台操作日志增加按受影响租户维护的 legacy 锚点和 SHA-256 结构摘要链。`platform.audit.read` 可查看链状态、校验历史和保留期清理预估，`platform.audit.manage` 可执行即时校验；维护命令 `cmd/mochat-saas-maintenance -action verify-audit-integrity` 和可选 `cron-saas-admin-audit-integrity` 复用同一校验服务。链保护日志 ID、租户、操作人、动作、目标标识和创建时间，允许合规流程继续脱敏目标名称、变更快照和备注。
- `0063_saas_audit_anchor_signatures` 在健康摘要链上创建 HMAC-SHA256 签名检查点，并把规范化 JSON 证据以 `0600` 文件写入独立目录。`GET /dashboard/saasAdmin/auditAnchors` 查看配置和检查点，`POST/PUT /dashboard/saasAdmin/auditAnchor` 执行创建或校验；维护动作 `create-audit-anchor`、`verify-audit-anchor` 与 `cron-saas-admin-audit-anchor` 共用同一服务。校验同时检查数据库载荷、历史密钥、签名、证据摘要、链头存在性和孤儿证据，可发现数据库检查点删除或回退。
- `0064_saas_audit_anchor_remote_immutability` 把同一签名证据复制到启用 Object Lock 的 S3 兼容桶，并保存对象键、版本 ID、ETag、SHA-256、大小、留存模式和到期时间。启动或创建任务会按批次回填 0063 历史检查点；校验可发现远端版本缺失、内容变化、留存失效和数据库回退后遗留的孤儿对象。生产建议启用 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE=1` 和 `compliance` 留存模式。
- 从数据库备份恢复时，如果原 Object Lock 前缀仍含有数据库备份时间点之后的对象，应保留旧前缀并把 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_PREFIX` 切换到新的恢复代际后回填；禁止覆盖或删除内容不一致的同名 WORM 对象。
- `0065_saas_service_account_key_pepper_ring` 为每个 API Key 保存 `hash_key_id`，新 Key 使用 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER(S)` 配置的独立 32 字节 pepper 密钥环。升级期可启用 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_ALLOW_LEGACY_JWT_PEPPER=1` 继续验证旧 Key；总后台会显示待轮换数量和缺失密钥 ID，所有旧 Key 轮换完成后应关闭兼容。生产应设置 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER=1`，此后轮换 Dashboard JWT 不会使独立 pepper 的 API Key 失效。
- `0066_saas_alert_credential_encryption` 将租户通知策略的 Webhook URL 与 Secret 作为同一认证载荷使用 AES-256-GCM 加密，AAD 绑定 Key ID、租户和通道。升级时可继续读取旧明文；配置活动/历史密钥环后由总后台或 `rotate-alert-credentials` 批量轮换并清空明文字段，完成后开启 `MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION=1`。
- `0067_wecom_credential_encryption` 为企业和应用增加企微凭据密文、Key ID 与保护状态索引。AAD 绑定记录类型、记录 ID、企微标识和 Key ID；历史密钥缺失会阻断凭据读取并进入总后台关键健康检查。轮换清空旧明文字段后，不得在未准备受控恢复方案时直接执行 0067 down。
- `0068_wechat_open_credential_encryption` 为组件 Ticket 与公众号授权资料增加密文、Key ID 与保护状态索引。AAD 绑定记录类型、租户、组件或授权方 AppID 和 Key ID；历史密钥缺失会阻断读取并进入关键健康检查。轮换清空旧明文字段后，不得把 0068 down 当作凭据恢复方案。
- 生产反向代理后的来源 IP 判定使用共享受信边界。只有 TCP 对端命中 `MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS` 时，身份安全或服务账号才会采信转发头；`X-Forwarded-For` 按右到左跳过可信代理，首个不可信地址才是客户端。未命中可信网段的请求始终使用直连 IP，无法靠伪造转发头绕过 IP/CIDR 白名单。
- Docker 交付镜像会在构建阶段计算当前源码与验收配置指纹，并通过 linker 写入所有 Go 二进制。发布准备 API 以运行二进制的内置指纹为准，拒绝客户端提交的其他指纹；未内置指纹时不能把证据标记为通过，也不能生成发布候选。本地 `go run` 可用 `MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT` 注入同一指纹，正式镜像的构建值不可被环境变量覆盖。
- 发布证据保存为 `passed` 前必须远端下载工件并校验实际 SHA-256 与字节数；发布候选会并行重验六项工件，`metadataReady` 只表示元数据完整。准备状态会继续把候选不可变快照与当前六项证据的 ID、版本、状态、URL、摘要、大小和复核信息逐项绑定；任一证据变化都会把旧候选的 `effectiveStatus` 降为 `stale`，恢复证据后仍需重新运行门禁，只有当前源码指纹下最近候选快照完整且与现行证据一致时才返回 `ready`。校验器默认超时 30 秒、单工件最大 64 MiB，并启用 HTTPS、私网/云元数据阻断、DNS 绑定、禁用环境代理和同源重定向限制。内部证据库需用 `MOCHAT_GO_SAAS_RELEASE_EVIDENCE_ALLOWED_CIDRS` 显式放行最小网段，自有 CA 通过 `MOCHAT_GO_SAAS_RELEASE_EVIDENCE_CA_FILE` 提供。
- `scripts/smoke_saas_release_readiness.sh` 使用真实 Go + MariaDB + Redis 和临时私有 CA HTTPS 工件库，覆盖发布准备 RBAC、危险地址和凭据拒绝、保存时远端校验、候选时内容篡改阻断、恢复后六证据放行、不可变复核快照、审计，以及 `0096` 至 `0057` 顺序回滚再应用。
- 启用域名交付时设置 `MOCHAT_GO_ENABLE_SAAS_TENANT_DOMAIN_DELIVERY_CRON=1`、`MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_BASE_URL`和至少 16 字符的 `MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN`。异步控制面还需同时设置公网 `MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL` 与至少 32 字符的 `MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET`；回调入口固定为 `POST /webhooks/saas/domain-delivery`。Bridge 实现 ACME/云证书与 Ingress 编排，Go 业务库不接收或保存证书私钥。
- 设置 `MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON=1` 后，Go standalone 会启动 `cron-saas-admin-approval-reminder`，默认每 5 分钟扫描一次待复核审批并按策略快照的 `next_reminder_at` 生成 `approval_sla_reminder` 通知 outbox。手动 `approvalReminders` 和自动任务复用同一事务服务，同一审批提醒次数幂等，任务状态与 periodic tick 写入后台任务账本；`scripts/smoke_saas_admin_approval_governance.sh` 会验证手动提醒、启动即跑和第二次提醒。
- 设置 `MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON=1` 后，Go standalone 会启动 `cron-saas-admin-system-health`。扫描覆盖 MySQL/Redis、当前 96 个迁移、API Key pepper 密钥环、后台任务、通知、审批、运营、结算、备份/恢复、备份保留清理队列、合规导出删除队列、合规、审计签名锚点、域名交付队列和 TLS 生命周期；配置企微或微信开放平台凭据加密后还会校验旧明文、历史 Key 与待轮换状态。身份安全、各凭据保护、备份、审计锚点异地对象存储和域名 Bridge 按实际配置增加对应探针。
- 设置 `MOCHAT_GO_ENABLE_SAAS_AUDIT_INTEGRITY_CRON=1` 后，Go standalone 会启动 `cron-saas-admin-audit-integrity`。默认每小时最多校验 100 个租户链，可通过 `MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_INTERVAL_SECONDS`、`MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_RUN_ON_START` 和 `MOCHAT_GO_SAAS_AUDIT_INTEGRITY_LIMIT` 调整；发现断链或结构字段篡改时，校验记录和后台任务执行都会标记失败。
- 审计签名锚点使用独立密钥配置：`MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY` 接受 32 字节 base64 或 64 位 hex；轮换时改用 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEYS='{"旧key-id":"...","新key-id":"..."}'` 并将 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID` 指向新密钥，历史密钥必须继续保留。证据目录由 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT` 指定；standalone Compose 默认挂到独立 `audit-anchor-storage` 卷。生产环境应把该目录映射到数据库管理员无法改写的独立或 WORM 存储，否则 HMAC 仍能防伪造，但不能防同时删除数据库和同机证据文件。
- 设置 `MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON=1` 后启动 `cron-saas-admin-audit-anchor`，默认每天最多处理 100 条链并在创建后立即复核；可用 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS`、`MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_RUN_ON_START` 和 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT` 调整。启用 cron 时必须配置 HMAC 主密钥。
- 设置 `MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON=1` 后，Go standalone 会启动 `cron-saas-backup`。它按持久化策略判断是否到期，成功备份立即做解密、认证、解压和全流读取校验；配置对象存储后继续上传并重新读取整个远端对象计算 SHA-256。保留清理先删除远端副本再删除本地工件。超过 24 小时的备份或副本上传租约会先恢复为失败，防止进程异常后永久占用并发槽。总后台灾备中心会显示自动调度开关、扫描间隔和启动检查状态；持久化策略已启用但调度器未启用、扫描间隔无效或策略停用时，系统健康中心的 `backup_automation` 探针会按关键故障阻断健康收口。
- 设置 `MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON=1` 后，Go standalone 会启动 `cron-saas-operation-queue-assignment-reminder`，默认每小时扫描一次运营待办最新认领，分别为 `overdue` 和 `due_soon` 生成 `operation_queue_assignment_reminder` 通知 outbox，并以 `actorUserId=0`、平台租户为操作者写入 `saas.admin.operation_queue.assignment_notify` 审计。旧负责人、未来认领、已关闭认领和已有同状态提醒会被跳过；实际 webhook 投递继续由 `cron-saas-alert-notification-dispatch` 按租户通知配置处理。`scripts/smoke_saas_operation_queue_assignment_reminder_cron.sh` 会用真实 MySQL 验证 run-on-start、最新认领筛选、两级提醒、幂等重启和后台任务执行历史。
- 设置 `MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON=1` 后，Go standalone 会启动 `cron-saas-notification-health-recovery`，默认每 15 分钟按 24 小时窗口和 15 分钟积压阈值复查当前 `notification_health` 认领。任务只对状态为 `healthy`、存在实际投递尝试且至少成功送达一次的租户写入 `resolved` 关闭审计，系统操作者为 `actorUserId=0`、平台租户；仍异常、无数据、只有 suppressed/closed/延期而没有成功送达证据的认领保持打开。任务状态和 periodic tick 暴露在 `/compat/status` 及后台任务表，`scripts/smoke_saas_notification_health_recovery_cron.sh` 用真实 MySQL 验证四类租户判定、重启幂等和审计上下文。
- `scripts/smoke_schema_migrate.sh` 会启动独立 MySQL，验证当前 96 个版本的空库 apply、checksum、status、重复 apply、baseline、legacy 升级；依次回滚 `0096_saas_tenant_enable_approval_guard` 至 `0052_saas_compliance_lifecycle`，分别核对策略、字段、索引、表、权限与数据清理，并验证 0096 down 只删除租户启用策略、0095 down 删除域名新增策略、0094 down 删除域名路由策略、0093 down 删除差异处理策略、0092 down 删除重开策略、0091 down 不会把已修复的关账策略降级，最后顺序重放四十五个版本并核对迁移账本。
- `scripts/lint_mysql57_schema.sh` 会静态扫描独立 schema 和增量迁移，阻止 `utf8mb4_0900`、`CHECK`、MySQL 8 函数索引、窗口函数和 JSON 非空默认值等 MySQL 5.7 不兼容语法进入独立部署包。
- `scripts/smoke_mysql57_schema_migrate.sh` 会启动 `mysql:5.7`，验证同一套 schema migration、增量迁移和 rollback 能在生产基线 MySQL 5.7 上执行；本机 arm64 默认跳过真实容器 smoke，因为 `mysql:5.7` 官方镜像为 amd64，在 qemu 下初始化会段错误，amd64 CI 或设置 `MOCHAT_FORCE_MYSQL57=1` 时会执行真实容器验收。
- `scripts/ci_mysql57_amd64.sh` 是生产 MySQL 5.7 兼容的严格 CI 入口，只允许在 `amd64/x86_64` 主机运行；它会先执行 `scripts/lint_mysql57_schema.sh`，再执行 `MOCHAT_ACCEPTANCE_SUITE=mysql57 ./scripts/standalone_acceptance.sh`，完成后输出当前 `源码指纹` 和 `mysql57 amd64 CI gate passed` 作为生产证据 marker。非 amd64 主机会直接失败，避免把 arm64 跳过误当成生产证据；`.github/workflows/mysql57-amd64.yml` 会在 GitHub amd64 runner 上执行该入口并上传 `mysql57-amd64-evidence` artifact。
- `scripts/import_mysql57_amd64_evidence.sh` 会校验并导入 `.github/workflows/mysql57-amd64.yml` 产出的 `mysql57-amd64.log` 或 artifact zip，要求包含 `mysql57 amd64 CI gate passed` 或 `mysql 5.7 schema migration smoke passed`，并拒绝 arm64 skip 日志；通过后会复制到 `docs/phases/phase-pre0-standalone/evidence/production/mysql57-amd64.log`，便于继续运行 `scripts/collect_production_evidence_pack.sh`。
- `scripts/standalone_route_coverage.sh` 会在 standalone + MySQL + Redis + JWT secret 下验证默认启用的已迁移业务路由，并同时断言 PHP/source/manifest 环境残留不会出现在 `/readyz`，输出 `route_total`、`migrated_manifest_route_total`、`missing_route_total` 和缺失样例。设置 `MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0` 可把它变成最终全量迁移门禁。
- `scripts/smoke_queue_idempotency.sh` 会启动独立 Redis，运行 `internal/store` 的 Redis 集成测试，验证 `EmployeeApply`、`wework-callback`、`contact-welcome`、`async-file-upload`、`mark-tags`、`message-remind` 和 `work-room-sync` 队列新 envelope 会写入 `queue/payloadType/idempotencyKey/enqueuedAt`，同一 payload 或同一企微业务事件在幂等窗口内只入队一次，并可正常 dequeue/ack。
- `scripts/audit_worker_saas_usage_assertions.sh` 会静态检查 11 个语义队列对应 smoke，要求同时包含租户 `queue_item` 执行历史和 `async_executions` SaaS 用量计数器断言，并已接入 `scripts/test.sh`。
- `scripts/smoke_async_file_upload_worker.sh` 会启动独立 Redis、Go standalone 和临时 HTTP 文件源，不启动 PHP/MySQL，也不读取原 MoChat 源码或外部 manifest，向 `mochat-go:async-file-upload` 写入 PHP `file_upload_queue` 旧数组 payload，验证 Go worker 可复制本地文件和 HTTP URL 到 `MOCHAT_FILE_STORAGE_ROOT` 下的目标相对路径、按 `unlink` 删除本地源文件，并清空 source/processing/dead 队列。`scripts/smoke_async_file_upload_saas_usage.sh` 会额外启动 MySQL 和两个 bootstrap 租户，分别投递带 `tenantId` 和带 `corpId` 的结构化 payload，验证 `async-file-upload` 会写入当前租户的 `queue_item` 执行历史、刷新 `async_executions` 和 `storage_mb` 用量；A 租户超额时只写入 A 租户的 `mochat_go_saas_alerts`，B 租户不产生 A 租户的执行记录、存储账本或告警。
- `scripts/smoke_mark_tags_worker.sh` 会启动独立 MySQL/Redis、Go standalone 和 fake 企业微信 API，不启动 PHP，也不读取原 MoChat 源码或外部 manifest，向 `mochat-go:mark-tags` 写入 PHP `MarkTags::handle(corpId, contactId, employeeId, tagIds)` 位置参数 payload，验证 Go worker 过滤已有标签、写入 `mc_work_contact_tag_pivot.type=1`、写入 `mc_contact_employee_track.event=2`、调用企业微信 `externalcontact/mark_tag`，并记录带 `tenant_id` 的 `queue_item/succeeded` 执行历史。
- `scripts/smoke_message_remind_worker.sh` 会启动独立 MySQL/Redis、Go standalone 和 fake 企业微信 API，不启动 PHP，也不读取原 MoChat 源码或外部 manifest，向 `mochat-go:message-remind` 写入 PHP `MessageRemind` 同口径 payload，验证 Go worker 读取 `mc_work_agent` 提醒应用、发送企业微信 `message/send` 应用消息、清空 source/processing/dead 队列，并记录带 `tenant_id` 的 `queue_item/succeeded` 执行历史。
- `scripts/smoke_work_room_sync_worker.sh` 会启动独立 MySQL/Redis、Go standalone 和 fake 企业微信 API，不启动 PHP，也不读取原 MoChat 源码或外部 manifest，向 `mochat-go:work-room-sync` 写入 PHP `UpdateCallback::handle(wxResponse)` 同口径 payload，验证 Go worker 通过 `ToUserName` 定位企业、拉取企业微信 `groupchat/get/list`、写入客户群和群成员，并记录带 `tenant_id` 的 `queue_item/succeeded` 执行历史。
- `scripts/smoke_wework_callback_worker.sh` 会启动独立 MySQL/Redis、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `wework-callback` 和 `contact-welcome` 为 `running`，先向 `mochat-go:wework-callback` 写入 `change_contact.create_user/update_user` 事件，验证 Go worker 能按 `UserID` 拉单员工并写入部门、员工、员工账号、员工部门关系和同步时间；再写入 `change_contact.create_party/update_party/delete_party` 事件，验证 Go worker 能按部门 `Id` 新增、修改和软删本地部门；随后写入 `change_external_tag.create/update/delete` 事件，验证 Go worker 能按标签 `Id` 新增、修改和软删本地客户标签且不误删标签组；新增客户事件会分别带 `State=channelCode-900`、`State=workRoomAutoPullId-901`、`State=fission-903` 和 `WelcomeCode`，验证 Go worker 读取 `mc_channel_code.welcome_message` 发送渠道欢迎语，并读取 `mc_work_room_auto_pull.leading_words/rooms` 跳过满群、上传可用群二维码后通过 `contact-welcome` 调用企业微信 `send_welcome_msg`，同时读取 `mc_work_room_auto_pull.tags` 和 `mc_work_fission.contact_tags` 写入标签 pivot、客户互动轨迹并调用企业微信 `externalcontact/mark_tag`；裂变事件还会验证 `mc_work_fission_welcome` 图文欢迎语发送、子参与人创建、上级 `invite_count/level/status` 更新、完成后企业应用员工提醒，以及 `push_contact=1` 时通过 `externalcontact/add_msg_template` 给上级客户创建客户群发消息；随后删除该裂变客户，验证参与人 `loss=1`、上级 `invite_count` 回退、客户员工关系软删和员工删除提醒；再写入 `change_external_chat.create/update/dismiss` 事件，验证 Go worker 只按回调 `ChatId` upsert 或软删单个客户群、幂等触发入群自动标签记录，并保留其他既有客户群不被误删；最后写入 `change_contact.delete_user` 事件，验证员工、员工部门关系和同手机号子账户状态按 PHP listener 口径更新，并断言 `queue_item` 执行历史带 `tenant_id` 且 `async_executions` 运行时计数器已刷新。
- `scripts/smoke_employee_apply_worker.sh` 会启动独立 MySQL/Redis、Go standalone、fake 企业微信 API 和 fake SaaS 告警 webhook，断言 `/compat/status.background_tasks` 中 `employee-apply` 为 `running` 且带 `run_id`，先向 `mochat-go:employee-apply:processing` 写入一条带 `queue/payloadType/idempotencyKey/enqueuedAt` 元数据的过期 envelope，验证 Go worker 能恢复并消费该任务，写入部门、员工、员工账号、员工部门关系和同步时间，并在 `mochat_go_background_task_executions` 写入带 `tenant_id` 的 `queue_item/succeeded`，同步刷新 `async_executions` 运行时计数器；随后设置低额度并写入第二条有效任务，验证超额不阻断队列处理但会写入 `mochat_go_saas_alerts` 和 `mochat_go_saas_alert_notifications`、fake webhook 首次 502 后按配置重试成功并发送带 HMAC-SHA256 签名和模板化 `title/body` 的 `saas.quota_alert`，可用真实 dashboard JWT 查询 `/dashboard/saasAlert/index`，再用 `cmd/mochat-saas-maintenance` 列出和解决告警，并通过 `dispatch-alert-notifications` 验证 outbox 到期通知可重发；同时写入一条失败任务，验证 3 次重试后进入 `mochat-go:employee-apply:dead`，并写入 `queue_item/failed` 执行历史。
- `scripts/smoke_pull_agent_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-pull-agent` 为 `running`，并验证 Go 定时任务能按 PHP `pullAgent` 口径刷新 `mc_work_agent` 的名称、头像、描述、停用状态、可信域名、上报配置和主页 URL。
- `scripts/smoke_employee_statistic_cron.sh` 会启动独立 MySQL/Redis、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-employee-statistic` 为 `running`，并验证 Go 定时任务能按 PHP `employeeStatistic` 口径拉取前一天成员统计、写入 `mc_work_employee_statistic` 并设置 Redis 去重 key。
- `scripts/smoke_channel_code_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-channel-code` 为 `running`，并验证 Go 定时任务能按 PHP `channelCode` 口径创建企业微信 contact_way、写回 `mc_channel_code.qrcode_url` 和 `wx_config_id`。
- `scripts/smoke_contact_batch_send_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-contact-batch-send` 为 `running`，并验证 Go 定时任务能扫描到期 `sendWay=2` 客户群发、创建员工/客户发送任务、调用企业微信 `add_msg_template` 并写回批次发送状态。
- `scripts/smoke_room_batch_send_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-room-batch-send` 为 `running`，并验证 Go 定时任务能扫描到期 `sendWay=2` 客户群群发、创建群主/客户群发送任务、调用企业微信 `add_msg_template` 并写回批次发送状态。
- `scripts/smoke_contact_sync_send_result_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-contact-sync-send-result` 为 `running`，并验证 Go 定时任务能按 PHP `ContactSyncSendResultTask` 口径同步最近一周客户群发成员发送状态、客户接收结果和批次汇总统计。
- `scripts/smoke_room_sync_send_result_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-room-sync-send-result` 为 `running`，并验证 Go 定时任务能按 PHP `RoomSyncSendResultTask` 口径同步最近一周客户群群发成员发送状态、群接收结果和批次汇总统计。
- `scripts/smoke_room_tag_pull_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-room-tag-pull` 为 `running`，并验证 Go 定时任务能按 PHP `RoomTagPull` 口径同步标签建群 `wx_tid.status` 和 `mc_room_tag_pull_contact.send_status`。
- `scripts/smoke_corp_data_cron.sh` 会启动独立 MySQL 和 Go standalone，不启动 PHP/Redis，断言 `/compat/status.background_tasks` 中 `cron-corp-data` 为 `running` 且带 `run_id`，验证 Go 定时任务能按 PHP `CorpDataLogic` 口径写入 `mc_corp_day_data` 和 `mc_work_update_time.type=6`，并检查 `mochat_go_background_tasks`、`mochat_go_background_task_runs` 与 `mochat_go_background_task_executions` 已持久化该后台任务状态和单次 tick 历史。
- `scripts/smoke_media_id_update_cron.sh` 会启动独立 MySQL、Go standalone 和 fake 企业微信 API，不启动 PHP/Redis，断言 `/compat/status.background_tasks` 中 `cron-media-id-update` 为 `running`，并验证 Go 定时任务能上传本地素材文件、刷新 `mc_medium.media_id` 和 `last_upload_time`。
- `scripts/smoke_transfer_state_refresh_cron.sh` 会启动独立 MySQL/Redis、Go standalone 和 fake 企业微信 API，断言 `/compat/status.background_tasks` 中 `cron-transfer-state-refresh` 为 `running`，并验证 Go 定时任务能按 Redis `log_id` 游标查询转接结果、刷新 `mc_work_transfer_log.state`。
- `scripts/smoke_sensitive_word_monitor_cron.sh` 会启动独立 MySQL 和 Go standalone，不启动 PHP/Redis，断言 `/compat/status.background_tasks` 中 `cron-sensitive-word-monitor` 为 `running`，并验证 Go 定时任务能扫描会话存档分表、命中启用敏感词、写入 `mc_sensitive_words_monitor`、推进 `mc_work_message_id.type=21` 游标，并在重复 tick 下保持幂等。
- `scripts/smoke_work_message_archive_sync_cron.sh` 会启动独立 MySQL、fake 会话存档 SDK bridge 和 Go standalone，断言 `/compat/status.background_tasks` 中 `cron-work-message-archive-sync` 为 `running`，并验证 Go 定时任务能按 `mc_corp.chat_status/chat_secret` 拉取启用企业、调用 bridge 获取已解密消息、按 `seq` 写入对应 `mc_work_message_*` 分表、推进 `mc_work_message_id.type=40` 游标、重复 tick 不重复入库，且入库消息能被敏感词监控继续消费。
- `scripts/smoke_fallback.sh` 会启动 fake PHP upstream，并显式设置 `MOCHAT_PHP_UPSTREAM` 验证迁移期 fallback 仍可用。
- `scripts/local_stack_check.sh` 会启动 MariaDB/Redis 容器，验证初始化 SQL、Redis、当前迁移路由、TXT 验证/上传和 `/compat/status` 的迁移计数，默认退出时清理容器和卷。
- `scripts/smoke_real_php_auth_chain.sh` 会复用本地已构建 PHP 镜像或构建/启动真实 PHP Hyperf 容器，验证 Go 签发的 token 可被 PHP 识别，Go logout 写入的黑名单会被 PHP 拒绝，并覆盖普通用户企业归属限制、企业授权创建/列表/详情/更新、企业微信回调 GET 校验/POST 解密入队、首页统计/折线图、员工列表与 PHP 原接口字段对照、员工搜索条件、企微成员/部门同步、企微客户同步、企微客户群同步、部门成员选择树、部门成员列表、手机号匹配部门下拉、组织架构分页树、组织架构员工列表、客户标签分组列表/详情与 PHP 原接口字段对照、客户标签分页/详情/标签分组树/标签下拉与 PHP 原接口字段对照、客户列表、客户来源枚举、客户资料更新、客户批量打标签、客户群成员列表、客户群列表、客户群下拉、客户群统计折线和分页统计、客户群批量修改分组、侧边栏客户群管理校验、自动拉群列表/详情/新建/更新、标签建群列表/详情/客户明细/员工任务/客户群下拉/客户筛选/筛选客户对照/新建/提醒发送/删除、客户群发列表/详情/明细/新建/提醒/删除、客户群群发列表/详情/群主明细/群接收明细/新建/提醒/删除、公众号授权列表/模块配置读取/设置更新、裂变 dashboard 列表/详情/配置详情/统计/选择客户/邀请数据/邀请明细/删除、裂变 operation 授权跳转/code 回调/openUserInfo/任务数据/邀请好友/海报/领奖、侧边栏素材临时 media_id 更新、侧边栏跟进状态默认项创建和跟进状态更新事务、dashboard/sidebar 客户详情基本信息、dashboard/sidebar 客户互动轨迹、侧边栏客户详情和侧边栏客户资料更新与 PHP 原接口字段或共用数据库结果对照、客户画像字段列表/详情/画像下拉/画像值与 PHP 原接口字段对照、角色下拉/列表/详情/权限树/成员列表、菜单下拉/图标/列表/详情、侧边栏工具配置、企业应用创建、OAuth/JSSDK 配置和 TXT 验证/上传。
- `scripts/sync_frontend_dist.sh` 会把本机已构建的 `dashboard/sidebar/operation/dist` 同步到本项目 `web/` 目录。
- `scripts/smoke_frontend_static_browser.sh` 会启动 Go standalone 和 sidebar/operation 独立前端监听地址，用真实浏览器加载 dashboard `/login`、主端口前缀入口 `/sidebar-app/contact`、`/operation-app/workFission`，以及独立端口 sidebar `/contact` 和 operation `/workFission`，验证三套内置 `web/*/dist` 的同源 JS/CSS 资源都由 Go 项目独立托管成功；该脚本不替代带真实登录态和企微环境的业务页面回归。
- `scripts/smoke_channel_code_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/channelCodeGroup/store/detail/update/move/index` 与 `dashboard/channelCode/store/update/index/show/contact/statistics/statisticsIndex`，验证渠道活码分组、渠道码新建/更新、企业微信 `externalcontact/add_contact_way` / `externalcontact/update_contact_way`、`mc_channel_code` / `mc_business_log` 写入、客户明细、扫码统计和 `channel_codes` SaaS 用量刷新。
- `scripts/smoke_shop_code_dashboard.sh` 会启动独立 MySQL/Redis 和 Go standalone，登录后调用 `dashboard/shopCode/store/index/info/location/searchCity/addressKeyWordList/share/pageSet/pageInfo/show/showContact/showShop/updateEmployee/updateQrcode/update/status/batchContactTags/destroy`，验证门店活码 CRUD、页面设置、地址/城市检索、分享链接、扫码记录统计、员工/二维码 JSON 字段更新、`mc_shop_code` / `mc_shop_code_page` / `mc_shop_code_record` 写入和 `shop_codes` SaaS 用量刷新。
- `scripts/smoke_greeting_dashboard.sh` 会启动独立 MySQL/Redis 和 Go standalone，登录后调用 `dashboard/greeting/store/index/show/update/destroy`，验证好友欢迎语全员/指定员工新建、列表状态、详情员工和素材展开、编辑、删除、`mc_greeting` 写入、`mc_business_log` 创建/更新日志和本地素材静态 URL 回显。
- `scripts/smoke_room_welcome_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/roomWelcome/store/index/show/select/update/destroy`，验证链接封面走 `media/uploadimg`、小程序封面走 `media/upload`、入群欢迎语模板走 `group_welcome_template/add/edit/del`，并断言 `mc_room_welcome_template` 创建、更新和软删除闭环。
- `scripts/smoke_work_room_auto_pull_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/workRoomAutoPull/store/index/show/update/move`，验证自动拉群创建写入 `mc_work_room_auto_pull`、刷新 `work_room_auto_pulls` SaaS 用量、列表/详情反查员工/标签/客户群和拉人状态、更新群二维码配置，并断言企业微信 `externalcontact/contact_way/create` 与 `externalcontact/contact_way/update` 的 `skip_verify`、`state`、`user` 和 `config_id` 请求口径。
- `scripts/smoke_room_tag_pull_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/roomTagPull/chooseContact/filterContact/store/index/show/showContact/roomList/remindSend/destroy`，验证标签建群筛客、新建、列表、详情、客户明细、员工任务、客户群下拉、提醒发送和软删除闭环，断言 `mc_room_tag_pull`、`mc_room_tag_pull_contact` 和 `room_tag_pulls` SaaS 用量刷新，并校验企业微信 `media/uploadimg`、`externalcontact/add_msg_template` 与 `message/send` 请求口径。
- `scripts/smoke_contact_message_batch_send_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/contactMessageBatchSend/store/index/show/messageShow/showRoom/employeeSendIndex/contactReceiveIndex/remind/destroy`，验证客户群发立即发送、列表、详情、消息预览、客户群详情、员工发送明细、客户接收明细、提醒发送和软删除闭环，断言 `mc_contact_message_batch_send`、员工/客户发送任务和 `contact_message_batches` SaaS 用量刷新，并校验企业微信 `media/upload`、`externalcontact/add_msg_template` 与 `message/send` 请求口径。
- `scripts/smoke_room_message_batch_send_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/roomMessageBatchSend/store/index/show/roomOwnerSendIndex/roomReceiveIndex/remind/destroy`，验证客户群群发立即发送、列表、详情、群主发送明细、群接收明细、提醒发送和软删除闭环，断言 `mc_room_message_batch_send`、群主/客户群发送任务和 `room_message_batches` SaaS 用量刷新，并校验企业微信 `media/upload`、`externalcontact/add_msg_template` 与 `message/send` 请求口径。
- `scripts/smoke_sop_dashboard.sh` 会启动独立 MySQL/Redis 和 Go standalone，登录后调用 `dashboard/contactSop/store/index/info/setEmployee/state/update/destroy` 与 `dashboard/roomSop/store/index/info/setRoom/state/update/destroy`，验证个人/群 SOP 规则 JSON 字段、范围字段、启停状态、触达日志级联删除和 `contact_sops` / `room_sops` SaaS 用量刷新。
- `scripts/smoke_contact_batch_add_dashboard.sh` 会启动独立 MySQL/Redis 和 Go standalone，登录后调用 `dashboard/contactBatchAdd/settingEdit/settingUpdate/importStore/importIndex/index/dataStatistic/remind/allot/destroy/importDestroy`，验证批量加好友配置、multipart CSV 去重导入、导入记录文件 URL、客户列表筛选、标签/员工反查、员工分配统计、二次分配记录、提醒接口、单条软删、批次软删和导入原文件 `storage_mb` 账本回收。
- `scripts/smoke_contact_transfer_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/contactTransfer/saveUnassignedList/info/unassignedList/room/index/log` 以及 POST 版 `dashboard/contactTransfer/room`，验证离职待分配同步、在职客户列表、离职待分配客户列表、待分配群列表、客户接替、群接替、分配记录、`mc_work_unassigned` / `mc_work_transfer_log` 落库，并校验企业微信 `externalcontact/get_unassigned_list`、`externalcontact/transfer_customer` 和 `externalcontact/groupchat/transfer` 请求口径。
- `scripts/smoke_work_fission_dashboard.sh` 会启动独立 MySQL/Redis、fake 企业微信 API 和 Go standalone，登录后调用 `dashboard/workFission/store/index/show/info/statistics/chooseContact/inviteData/inviteDetail/invite/update/destroy`，验证裂变活动新建、配置读取、统计、筛选客户、邀请数据、邀请明细、二次邀请、更新和删除闭环，断言 `mc_work_fission` 及 poster/welcome/push/invite 子表、`work_fissions` SaaS 用量刷新，并校验企业微信 `media/uploadimg`、`externalcontact/add_contact_way` 与 `externalcontact/add_msg_template` 请求口径。
- `scripts/smoke_dashboard_frontend_login.sh` 会启动 MariaDB/Redis 和 Go standalone，不启动 PHP、不读取原 MoChat 源码或外部 manifest，用 Go 服务自身托管的 `web/dashboard/dist` 运行真实浏览器登录验证，覆盖菜单加载、首页 `corpData/index`/`lineChat`、`loginShow`、`corp/select`、超级管理员企业切换 `corp/bind`，并继续访问 `/corpData/index`、`/corp/index`、`/user/index`、`/passwordUpdate/index`、`/role/index`、`/role/permissionShow?roleId=910001`、`/menu/index`、`/department/index`、`/workEmployee/index`、`/workContact/index`、`/workContact/contactFieldPivot?contactId=910001&employeeId=2&isContact=1`、`/lossContact/index`、`/workContactTag/index`、`/workRoom/index`、`/workRoom/detail?workRoomId=910001`、`/workRoom/statistics?workRoomId=910001`、`/channelCode/index`、`/channelCode/statistics?channelCodeId=910001`、`/channelCode/store`、`/mediumGroup/index`、`/greeting/index`、`/greeting/store`、`/roomWelcome/index`、`/roomWelcome/create`、`/workRoomAutoPull/index`、`/workRoomAutoPull/store`、`/roomTagPull/index`、`/roomTagPull/create`、`/roomTagPull/detail?id=917001`、`/roomTagPull/contactDetail?id=917001`、`/autoTag/keywordIndex`、`/autoTag/keywordCreate`、`/autoTag/keywordShow?idRow=918001`、`/autoTag/joinRoomIndex`、`/autoTag/joinRoomCreate`、`/autoTag/joinRoomShow?idRow=918002`、`/autoTag/dayPartIndex`、`/autoTag/dayPartCreate`、`/autoTag/dayPartShow?idRow=918003`、`/contactMessageBatchSend/index`、`/contactMessageBatchSend/store`、`/contactMessageBatchSend/show?batchId=915001`、`/roomMessageBatchSend/index`、`/roomMessageBatchSend/store`、`/roomMessageBatchSend/show?batchId=916001`、`/workFission/taskpage`、`/workFission/create`、`/workFission/edit?id=985001`、`/workFission/invite?id=985001`、`/workFission/dataShow?id=985001`、`/officialAccount/index`、`/officialAccount/create`、`/contactField/index`、`/chatTool/customer`、`/chatTool/enhance`、`/statistics/contact`、`/statistics/employee`、`/contactTransfer/resignIndex`、`/contactTransfer/workIndex`、`/contactTransfer/workAllotRecord` 和 `/contactTransfer/resignAllotRecord` 共 61 个 dashboard 业务页面，并额外访问 Go 原生 `/dashboard/sensitiveWords/page` 敏感词管理页、`/dashboard/lottery/page` 抽奖活动页、`/dashboard/radar/page` 互动雷达页、`/dashboard/shopCode/page` 门店活码页、`/dashboard/contactSop/page` 个人 SOP 页、`/dashboard/roomSop/page` 群 SOP 页、`/dashboard/roomFission/page` 群裂变页、`/dashboard/roomQuality/page` 群质检页、`/dashboard/roomCalendar/page` 群日历页、`/dashboard/roomRemind/page` 客户群提醒页、`/dashboard/roomInfinitePull/page` 无限拉群页、`/dashboard/roomClockIn/page` 群打卡页和 `/dashboard/saasAlert/page` SaaS 告警管理页，等待对应 Go dashboard API 返回 200；脚本会把页面渲染到 `/404`、同源 dashboard 接口 4xx/5xx、非导航取消类本地请求失败和本地静态资源 4xx/5xx 视为失败。脚本直接调用 Node `playwright` 模块；如 Playwright 不在默认全局模块目录，可设置 `PLAYWRIGHT_NODE_PATH`。

smoke 脚本会先构建临时 `mochat-go` 二进制再后台运行，避免 `go run` 包装进程退出后遗留真实服务进程。

## 本地联调环境

如果本机没有 PHP 7.4+/Swoole/Redis，可用 Docker 启动 PHP/MySQL/Redis 联调环境：

```bash
cd /Users/lv/Documents/企业微信/mochat-go
docker compose -f deploy/local/docker-compose.yml up -d mysql redis
docker compose -f deploy/local/docker-compose.yml up -d php
```

如果宿主机 `9501` 已被占用，可改用：

```bash
MOCHAT_PHP_PORT=19501 docker compose -f deploy/local/docker-compose.yml up -d php
```

详细 Go 网关启动参数见 [deploy/local/README.md](deploy/local/README.md)。

## 重新生成清单

```bash
cd /Users/lv/Documents/企业微信/mochat-go
env -u GOROOT go run ./cmd/mochat-inventory \
  -source-root ../mochat \
  -out ../docs/migration \
  -source-revision 64df1ad3c9c2a2b0c82ee8a853a398cc3606947e
```
