# 企微会话存档、SaaS 对接模式与新租户激活专项验收报告

> 数据集：`MOCHAT-LOCAL-ACCEPTANCE-20260827`（本地验收 / 非生产）
>
> 验收日期：2026-08-27
> 结论边界：本报告中的 SDK 结果仅代表去敏 Finance SDK 合同夹具在本地生产代码路径通过，不代表真实企业微信线上联调通过。

## 1. 实施结果

- 新增 `mochat-archive-acceptance` CLI，提供 `bridge`、`seed`、`checkpoint`、`verify`、`cleanup`；输出仅含固定数据集标识、计数和非敏感事实，不输出 Bearer、SDK locator、数据库口令或密码。
- fixture bridge 使用真实 `ArchiveFixture.GetChatData/DecryptData/GetMediaData` 与 `NewAdminHandler`，随后进入生产 `BridgeSource → 0138 SyncService/MySQLStore → media worker → Dashboard media API`，没有直接向消息表或媒体表插入业务结果。
- 固定生成 9 类、共 10 条消息：文本、图片、语音、视频、文件、链接、位置、混合和未知类型；第 8 条为独立可见的 missing 图片，第 9 条 mixed 同时含 ready 与 corrupt 图片。共形成 7 个媒体对象，其中 5 个 ready、1 个 missing、1 个 corrupt。媒体按 7 字节分片，所有正常样本均超过 3 个分片。
- `verify` 核验同一 run、cursor=10、10 条消息的正式 Dashboard list/detail 投影、链接/位置/mixed/unknown、mixed 内全部媒体、同步审计、加密的 `self_built` current 与 `third_party_delegated` candidate、5 个对象文件的大小/SHA-256、locator 不泄漏，以及未登录 401、正式 Dashboard 登录后鉴权下载 200、missing/corrupt 404 和内容哈希一致。
- 独立 Compose 项目为 `mochat-wecom-acceptance-20260827`；随机数据库口令、JWT、bridge token、企微加密键、MFA 键及本地验收密码只写入 ignored runtime 目录 `.tmp-wecom-acceptance-runtime/`。
- `seed` 同时通过仓库受控 `mochat-bootstrap` 创建幂等 SaaS 平台验收账号；随后只通过正式 `/saas/auth/login` 428 challenge 与 `/saas/auth/password` 完成一次初始化改密，成功后才安全替换 ignored 密码文件。Windows ACL 仅保留当前用户、SYSTEM 与 Administrators。
- integration 通过正式 service 完成 candidate save/verify/switch/rollback 并保留 5 类审计；激活夹具通过正式 provisioning/auth/status/resend 服务生成 valid、expired、activated、revoked、invalid 五种可选择状态。

## 2. 自动化与 Docker 证据

| 验收项 | 结果 | 证据摘要 |
|---|---|---|
| CLI 单元测试 | PASS | 参数脱敏、bridge 鉴权、cleanup 范围、绝对对象路径、missing/corrupt、正式登录与媒体鉴权读取 |
| fixture / durable / media 相关 Go 测试 | PASS | Finance SDK JSON、分页 cursor、失败不推进、重放幂等、分片与 checkpoint/restart 合同 |
| Node 静态专项门禁 | PASS | DTO/locator、durable 与 legacy 互斥、激活 URL 清理、独立 Compose、密码文件和固定数据集 |
| Compose 配置渲染 | PASS | 固定 project，独立端口和命名卷，敏感项来自 ignored env/file |
| 验收库增量迁移顺序 | PASS | 静态合同要求 `0133 < 0138`、`0164 < 0166`；现有隔离库原位应用 0164 后，租户 AI Provider 正式 API 返回 HTTP/envelope 200 |
| MariaDB `0166 down → up` | PASS | down 后 integration/media 两表均不存在，up 后两表恢复 |
| `seed -defer-media` / 重放 | PASS | runId=8、cursor=10、10 messages、7 pending；再次调用正式 Sync 返回同一 runId 且 `syncIdempotent=true` |
| worker checkpoint/restart | PASS | 捕获 fetching `bytesReceived=154`、attempt/checkpointAttempt=1；SIGKILL 后精确过期 lease，scheduler-backed worker takeover；recoveredObjects=1、workerRuns=1 |
| 最终 `seed` 重放 | PASS | 10 messages；7 media；5 ready、1 missing、1 corrupt；同一 runId=8、`syncIdempotent=true`、`mediaProcessed=0` |
| `verify` | PASS | Dashboard 类型 `[1,2,3,4,5,6,7,9,100]`、10 条详情、mixed 全媒体、同步审计 6 条、integration 审计 5 类、五种激活状态 |
| `cleanup -DryRun` | PASS | 只报告本数据集计数，不写数据 |
| `cleanup` / 重复 cleanup | PASS | 首次删除精确业务行和 5 个对象文件；重复执行为全 0 且成功 |
| app + bridge + worker restart 后 verify | PASS | fresh acceptance image 后 worker recovered 事实及 cursor、消息、媒体、对象文件、Dashboard HTTP 均通过 |
| 应用内浏览器点击 | PASS | Dashboard 正式登录、10 条消息与媒体回读；SaaS 模式切换/回滚、失败关闭、激活全状态与重发安全提示均真实点击通过 |
| Chromium 响应式 | PASS | Dashboard 29 个页面与员工 Sidebar 360×800、390×844、430×932 等 74 个用例全部通过，无页面级横向溢出或不安全失败 |

## 3. 本地访问

- Dashboard：`http://127.0.0.1:19080/`
- SaaS 管理端：`http://127.0.0.1:19080/saas-admin/`
- Sidebar：`http://127.0.0.1:19081/`
- Operation：`http://127.0.0.1:19082/`
- fixture bridge health：`http://127.0.0.1:19091/healthz`
- MariaDB：`127.0.0.1:19016`
- Redis：`127.0.0.1:29089`

非敏感登录提示：Dashboard 账号为 `19008208270`；SaaS 账号为 `mochat-local-acceptance-admin`。两者随机密码分别只保存在 ignored 文件 `.tmp-wecom-acceptance-runtime/dashboard-acceptance-password` 与 `.tmp-wecom-acceptance-runtime/saas-admin-password`，报告与命令输出不记录密码。

## 4. 浏览器验收结果

1. Dashboard：正式登录后会话列表显示 1 个会话、10 条消息；逐项回读文本、图片、语音、视频、文件、链接、位置、mixed 与 unknown。图片的鉴权 Blob 实际解码为 2×2，音频和视频进入可播放就绪态，文件下载按钮可用；missing 独立显示“媒体已缺失”，mixed 同时显示可用图片和“媒体已损坏”。刷新后状态恢复，未发现控制台错误。
2. SaaS 对接模式：租户详情只显示凭据提示而不回显敏感值；真实点击 `自建应用 → 第三方代开发应用` 时确认框列出绑定、能力、凭据、媒体租约和 generation 前置检查，确认后切换成功；再真实点击回滚后恢复自建应用，审计时间线同步更新。未绑定企业的租户以中文业务阻塞原因失败关闭，不显示 `TARGET_NOT_FOUND` 等技术错误。
3. 新租户激活：valid 显示安全激活表单；expired、activated、revoked、invalid 分别显示准确终态；读取 token 后地址栏立即清理为 `/activate`。在 revoked 租户真实点击重发，确认框显示租户、账号和绑定版本；结果明确说明系统不会自动发送邮件、短信或企微消息，关闭一次性入口弹窗后 DOM 中不再保留激活码。
4. 响应式与稳定性：应用内浏览器完成桌面点击、刷新、空态和失败态；另以真实 Chromium 固定视口执行 Dashboard 与员工 Sidebar 响应式回归，含 390×844，74/74 PASS。SaaS 和 Dashboard 关键操作期间未发现控制台错误。

## 5. 回滚与清理

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_wecom_archive_saas_activation_acceptance.ps1 -Action cleanup -DryRun -NoBuild
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_wecom_archive_saas_activation_acceptance.ps1 -Action cleanup -NoBuild
```

cleanup 使用固定 tenant/corp/user/integration ID、固定数据集消息前缀、bootstrap request key 和精确 FK 顺序；对象路径必须位于配置的 `archive-media/` 根内。它不删除卷、不执行 `TRUNCATE`，也不使用宽泛 tenant 清理。若需要停止容器，可执行不带 `-v` 的 `docker compose stop`；不得用 `down -v` 清除卷。

## 6. 保留边界

- 真实企业微信 `GetChatData/DecryptData/GetMediaData`：SKIP（按本次范围不再调用真实企微）。
- 真实企微 CorpID、会话存档 Secret、RSA、可信 IP、线上授权企业与永久授权码：SKIP / 外部条件。
- 本地 MariaDB：PASS，使用独立 Docker MariaDB 10.6；这不能替代目标生产数据库版本与生产数据升级演练。
- 应用内浏览器：PASS；结论来自实际点击、刷新、媒体解码/就绪态和失败态检查，不以数据库计数或截图替代。
- 当前 Compose 保持运行，未执行 `down -v`，便于继续浏览器验收。
