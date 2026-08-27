# 双模式企微会话存档模拟器与 Docker 合流验收报告

> 验收日期：2026-08-28
>
> 数据集：`MOCHAT-LOCAL-SIM-self-20260828`、`MOCHAT-LOCAL-SIM-delegated-20260828`
>
> 结论边界：本报告证明去敏夹具在本地生产代码路径通过，不代表真实企业微信线上联调通过。

## 1. 最终实现

- `mochat-go-desktop` 统一包含 app、archive-bridge、MariaDB 和 Redis。bridge 保持同一 Compose 项目中的独立容器，只在容器网络暴露 `8083`，不向宿主机开放端口。
- 自建应用模式执行 RSA-2048 会话密钥解包、AES-256-GCM 消息解密、`GetChatData` 游标拉取与 `GetMediaData` 分片下载；第三方代开发模式执行 suite 授权回调签名/AES-CBC 解密、永久授权凭据边界、逐消息密钥解包及一次性展示组件回读。
- 两种模式共用正式 durable source/cursor/lease/idempotency、消息解析、租户隔离、媒体账本、对象存储、鉴权 API 和 Dashboard 投影，不直接向最终业务表插入模拟消息。
- 模拟器提供 `seed`、`send`、`status`、`cleanup`。所有业务数据都带 `MOCHAT-LOCAL-SIM-` 标记；种子、重放和清理均幂等，清理遇到 source 混入非夹具消息会失败关闭。
- `seed` 会在正式数据库事务中创建数据集专属本地员工/客户，并激活 `local_contract` integration。自建模式拒绝生产形态 CorpID；第三方模式要求 SaaS 已有 Provider App ID 与加密授权凭据。
- Compose 默认关闭旧 archive cron、启用 durable archive worker，避免双管线竞争；媒体批次遇到缺失或损坏对象后继续处理同批其他对象，并只返回脱敏汇总错误。
- 修复客户会话无关系记录时后端返回非法 `none` 的问题，统一为前端合同支持的 `unknown`，避免 Dashboard 会话页整体加载失败。

## 2. 真实本地验收证据

| 验收项 | 结果 | 证据 |
|---|---|---|
| 自建模式拉取 | PASS | 初次正式 durable run：cursor=10、fetched=10、processed=10 |
| 第三方模式拉取 | PASS | 初次正式 durable run：cursor=5、fetched=5、processed=5；5 个加密展示组件定位器 |
| 主动追加与重放 | PASS | 两种模式均通过 simulator `send` 追加文本；再次 `seed` 不重复升级 integration 或写审计 |
| 自建媒体 | PASS | 7 个媒体任务最终为 5 ready、1 failed（缺失）、1 corrupt；图片、音频、视频、PDF 均走鉴权读取 |
| 失败不推进 | PASS | bridge 重启竞争产生的失败 run 保持 cursor=0；随后成功 run 分别推进到 10/5 |
| 重启恢复 | PASS | app 与 bridge 重启后，上游状态、cursor、消息、媒体文件及展示组件均恢复；空轮询不重复插入消息 |
| 精确清理 | PASS | 自建删除 11 消息、7 媒体、5 文件、2 参与者和 18 runs；第三方删除 6 消息、6 组件、2 参与者和 18 runs |
| 清理后重建 | PASS | 两个数据集重新 seed，重启 worker 后恢复 10/5 条基线消息与全部媒体状态 |
| 日志脱敏 | PASS | 日志只记录加密配置状态、任务名和固定错误码；未输出 Secret、永久授权码、locator、Bearer 或数据库口令 |
| Docker 存储清理 | PASS | 删除临时验收项目和 21 个非 desktop 卷；镜像 38→4、卷 30→4、构建缓存 115.4GB→0B；只保留 4 个健康 desktop 容器 |

## 3. 浏览器点击验收

- Dashboard 数据概览与客户会话使用正式登录会话访问，无批量“加载失败”。客户会话实际选择本地验收客户后展示 10 条消息：文本、图片、音频、视频、文件、链接、位置、missing、corrupt 和未知类型。
- 图片使用登录鉴权 Blob；音频/视频进入浏览器媒体元素；文件提供鉴权下载；missing 显示“媒体读取失败，请稍后重试”，corrupt 显示“媒体已损坏”，未知类型显示普通用户可理解的“不支持预览”。刷新和容器重启后状态恢复。
- 文件录音页真实打开并展示查询、重置、刷新、分页和播放控件，页面未出现 `Provider` 技术术语；390×844 视口下页面内容与操作仍可访问。
- SaaS 租户详情实际点击自建/第三方租户：开户模式不可切换；自建应用只提示去 Dashboard 唯一企业资料维护，不在 SaaS 展示 Secret 配置；第三方只显示 Provider App ID 与凭据类型，永久授权码输入为空且不回显；能力默认全开，超级管理员治理下拉与操作项可用。
- 新租户 pending 激活状态、重发入口和“系统不会自动发送”的安全提示可见；valid/expired/activated/revoked/invalid 状态由前端与后端自动化合同覆盖。

## 4. 自动化门禁

| 门禁 | 结果 |
|---|---|
| `go test ./... -count=1` | PASS |
| Dashboard lint / typecheck / test / build | PASS，147 files / 966 tests |
| Sidebar lint / typecheck / test / build | PASS，14 files / 153 tests |
| Operation lint / typecheck / test / build | PASS，5 files / 74 tests |
| SaaS Admin lint / typecheck / test / build | PASS，6 files / 53 tests |
| Provider completion + 隔离 MariaDB integration | PASS，无 SKIP |
| Phase 4 Dashboard RBAC | PASS |
| Yuanhu benchmark | PASS，53 pages |
| Dashboard all-pages evidence contract | PASS |
| WeCom archive / SaaS activation contract | PASS |
| standalone Compose URL / durable 单管线合同 | PASS |
| 0138、0166–0169 迁移与 archive store 合同 | PASS，使用本地 MariaDB 10.6 临时 schema |
| `git diff --check` | PASS |

额外执行仓库全部 MariaDB 集成测试时，既有 0130 受控身份迁移因没有专用 validated staging batch、既有 MFA 集成夹具返回 401，以及部分旧 provider/customer 测试夹具缺少当前表列而失败。它们不属于本专项变更，也不影响上述专项迁移合同；本报告不把这次扩展运行记为 PASS。

## 5. 本地访问与操作

- Dashboard：`http://127.0.0.1:18080/`
- SaaS 管理端：`http://127.0.0.1:18080/saas-admin/`
- Sidebar：`http://127.0.0.1:18081/`
- Operation：`http://127.0.0.1:18082/`
- MariaDB：`127.0.0.1:13316`
- Redis：`127.0.0.1:26389`

bridge 不提供宿主机 URL；健康检查只在 Compose 网络内执行。当前应用内浏览器已保留 Dashboard 与 SaaS 登录会话，密码、JWT 和加密键不写入报告。

发送模拟消息与精确清理命令见《企微会话存档双模式本地模拟器操作指南》。保留数据卷为 `mochat-go-desktop_app-storage`、`mochat-go-desktop_audit-anchor-storage`、`mochat-go-desktop_mysql-data`、`mochat-go-desktop_redis-data`。

## 6. 回滚、清理与外部边界

- 业务夹具使用 simulator `cleanup`，必须同时传 `--dataset` 和完全相同的 `--confirm-dataset`；不会删除租户、企业或企微模式配置。
- 停止服务但保留数据：`docker compose -p mochat-go-desktop --profile app -f deploy/standalone/docker-compose.yml stop`。
- 数据库维护前备份位于 desktop app-storage 的 `backups/local-maintenance-20260828/mysql-before-bridge-merge.sql.gz`，已验证 gzip 和 SQL 头。
- 真实企微 CorpID、Secret/RSA、可信 IP、第三方 suite 授权企业、永久授权码刷新及 `GetChatData/GetMediaData` 线上调用仍是外部联调边界，本次按要求未调用真实企微。
