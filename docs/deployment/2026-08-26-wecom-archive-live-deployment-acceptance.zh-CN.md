# MoChat Go 会话存档接入部署与三端验收报告

> 日期：2026-08-26（Asia/Shanghai）
> 服务器：`139.196.34.133`
> 结论：**部署与专用回调链路通过；真实企微拉取和三端登录态验收仍有外部配置/权限阻塞，不宣称全量成功。**

## 1. 版本与发布物

| 项目 | 部署前 | 部署后 |
| --- | --- | --- |
| 权威远端 `origin/main` | `c696aa66400b5cb7185f18a4152ea6c0ac593834` | 未改动，仍为 `c696aa66400b5cb7185f18a4152ea6c0ac593834` |
| 本次部署分支 | `fix/ai-insight-readonly-20260826` | `cacdc740a14ab63b6e798c3f33bda9a23ae97101` |
| 主应用镜像 | 旧运行镜像已做回滚标签和 tar 备份 | `mochat-go:cacdc740a14a`，镜像 ID `sha256:05a37da4a898c2033e0b9c859e1a41045dcaa7a7c7779ddbe5c5742610e2dc13` |
| 主应用 tar | — | SHA-256 `0bbcb308c57fa12167c0e8faaea8d1b1e10c6c1eac540241fa0409683f9115ca` |
| Finance SDK bridge | 原隔离 Demo 镜像和容器已备份 | `mochat/wecom-archive-demo:d0c9409df11a`，镜像 ID `sha256:93e954b83cf75f61300cf7e2c82e5caac0529d487cc577036526c9a7b32fc2b3` |
| bridge tar | — | SHA-256 `b2a8ba89f875cb590355636fe42db8a8ab68d0e2786fccacda1a0cca6aaa6068` |

主应用和 bridge 均在本地构建为 `linux/amd64` 后通过 `docker save` 交付。服务器只执行校验、`docker load` 和容器替换，**未在服务器编译**。

## 2. 服务器现状、备份与回滚点

- ECS 为 Ubuntu 24.04，约 1.6 GiB 内存、无 swap；因此严格禁止服务器构建。
- 主 Compose：`/opt/mochat-go/deploy/standalone/docker-compose.yml`；运行配置：`/opt/mochat-go/deploy/standalone/.env.local`。
- Nginx 继续把公网请求代理到主应用 `18080`；MySQL、Redis、既有命名卷和数据库均未删除或重建。
- 完整部署前回滚点：`/opt/mochat-go/backups/wecom-archive-e1546ad6ad90-predeploy-20260826T1838CST`。目录为 `0700`，敏感配置副本为 `0600`；数据库 gzip 备份、镜像 tar、Nginx/Compose/环境配置和 `SHA256SUMS` 均已校验。
- 原始回滚镜像：`mochat-go-rollback:pre-wecom-archive-e1546ad6ad90`、`mochat/wecom-archive-demo:pre-e1546ad6ad90`。
- 最终主应用发布目录：`/opt/mochat-go/releases/cacdc740a14a-wecom-archive-20260826T2140CST`。
- bridge 发布目录：`/opt/mochat-go/releases/d0c9409df11a-wecom-archive-20260826T2000CST`。
- 旧 bridge 容器保留为停止状态，没有删除其配置、证据目录或数据。

## 3. 实施内容

1. 新增主应用专用入口：`/wecom/archive/callback?cid=<企业内部 ID>`，GET/POST 精确匹配，其他方法和子路径失败关闭。
2. 修复两层路由抢占：生产 Module Router 和 Dashboard SPA 静态兜底均不能再把企微 GET 校验请求返回为 HTML。
3. `msgaudit_notify` 事件可触发与定时兜底共用的企业会话存档同步任务。
4. 主应用只向内网 bridge 传递分页/游标请求；会话存档 Secret、RSA 私钥和 bridge 管理 Token 不出现在业务请求或页面中。
5. Compose 显式转发归档同步开关、15 秒间隔、启动即跑、批量上限、bridge 内网地址和受保护 Token。
6. 企业配置页“会话存档配置”新增只读、可复制的“接收事件服务器 URL”，并明确其应填写到企业微信“管理工具 → 会话内容存档”。

实际执行的命令类别包括：Git/worktree 只读盘点、Go/Node 单元测试和门禁、本地 Docker 构建与 `docker save`、交互式 SSH/SCP、服务器 SHA-256 校验、`docker load`、app-only Compose 重建、bridge 精确替换、迁移检查、容器/端口/日志/HTTP 健康检查及应用内浏览器验收。任何命令、仓库或报告均未记录登录密码。

## 4. 本地门禁

- `go test ./... -count=1`：通过。
- 回调路由针对性和生产组合回归测试：通过。
- Dashboard lint、typecheck、production build：通过。
- Dashboard 全量测试：`142` 个测试文件、`918` 项测试全部通过。测试输出存在 jsdom 对伪元素 `getComputedStyle` 的既有提示，但退出码为 0、无失败测试。
- 会话存档 cron smoke：真实测试 MySQL + fake bridge 下的分页、游标、幂等、敏感词消费、加密凭据和禁止字段检查通过；它属于受控集成测试，不是生产企微证据。
- Finance SDK bridge 镜像内官方 SDK 装载与所需符号自检：通过。

## 5. 服务器部署与技术验收

| 检查项 | 结果 |
| --- | --- |
| app / bridge 容器 | 重启后均为 `running / healthy` |
| MySQL / Redis | 容器 ID 在 app-only 部署前后不变 |
| 迁移 | `165/165` 已应用，最新为 `0165_ai_daily_insight_unification` |
| `/healthz`、`/readyz` | 200 |
| `/index`、`/login`、`/saas-admin/`、`/contact`、`/workFission` | 200 |
| bridge 管理口 | 未鉴权 401；服务器本机携带受保护 Token 为 200；不公开管理 Token |
| 专用 callback GET | 伪造签名返回 `400 text/plain`，不再返回 SPA HTML |
| 企业微信真实 URL 校验 | 2026-08-26 20:27:30（CST）真实 GET 返回 200、响应 19 字节；证明 URL、Token 签名与 EncodingAESKey 解密/原文回包匹配，证据仅保留时间、方法、状态和字节数 |
| 企业微信真实事件通知 | 2026-08-26 21:23:46、21:24:05（CST）收到两次真实 POST，均返回 200；主应用识别 `corp=4` 后因 Dashboard/主库尚未启用归档凭据而明确跳过主库同步 |
| Finance SDK 真实直拉 | 官方 `GetChatData` 成功；真实游标从 `seq=0` 推进至 `seq=4`，解密并保存 4 条证据（1 条图片、3 条文本），`publickey_ver=1`，无拉取错误 |
| 幂等与重启续拉 | `seq=4` 空页复拉为 0；bridge 重启后仍从 `seq=4` 开始且返回 0，JSONL 证据保持 4 行，无重复写入 |
| callback POST | 非法 XML 返回 400，证明进入回调处理器 |
| callback 子路径 / PUT | 均为 404 |
| 定时同步 | 每 15 秒正常执行，当前 `corps=0`、`failed=0` |

可填写到企业微信后台的专用地址为：

`http://139.196.34.133/wecom/archive/callback?cid=4`

该地址的网络和路由链已经通过合成失败关闭验证；只有企业微信携带真实签名、Token/AES Key 与 MoChat 一致时，才能把“企业微信后台保存成功”计为通过。

## 6. 三端验收

### A. SaaS Admin

- `/saas-admin/` 静态资源和登录入口正常，未登录会明确进入“SaaS 管理员登录”。
- 当前浏览器没有 SaaS Admin 会话，且本次未读取、重置或创建平台管理员密码，因此租户列表/详情、AI 分析配置等登录后页面标记为 **BLOCKED_AUTH**。
- 未把 Dashboard 登录态冒充 SaaS Admin 登录态；没有提交登录表单或改变平台数据。

### B. Dashboard

- 现有登录态可刷新并稳定进入 `/company-setting/website`，页面无控制台 error/warn。
- 企业配置真实返回：CorpID 待验证、应用凭据已加密保存、会话存档未配置、标准企微能力未完成运行验证。
- 新增“接收事件服务器 URL”在生产页面显示为上述 `cid=4` 地址，字段只读，复制按钮有明确成功反馈；页面不回显会话存档 Secret 或 RSA 私钥。
- 当前账号只获“唯一企业资料”页面权限；访问 `/index`、`/ai-setting/agent`、`/ai-insight/session-analysis` 均被权限系统重定向回企业资料页。因此数据概览、AI 设置和 AI 洞察登录后业务验收标记为 **BLOCKED_RBAC**，未伪称通过。

### C. 员工移动端 Sidebar

- 12 条登记路由逐项访问：`/`、`/auth`、`/codeAuth`、`/contact`、`/contact/editDetail`、`/contact/remark`、`/contact/settingTag`、`/contactBatchAdd`、`/contactSop`、`/login`、`/medium`、`/roomSop`。
- 9 条受保护路由能保留目标地址并进入员工登录；3 条公开授权路由能展示明确失败态；控制台无 error/warn。
- 不带 AgentID 时明确提示“缺少有效的企业应用 ID”；带当前已配置 AgentID 后可出现“继续授权”。实际发起授权返回“应用不存在”，说明企业资料中的 AgentID/Secret 尚未形成 Sidebar 所需的应用记录/员工授权链。
- 应用内浏览器的临时 viewport 覆盖未实际变为 390×844（页面报告仍为 1280 宽），因此生产 390×844 视觉结果标记为 **SKIP_BROWSER_CAPABILITY**；代码级响应式/路由测试通过，但不能替代真实移动视口。
- 真实企微 OAuth/JSSDK、客户上下文和员工侧真实接口标记为 **BLOCKED_WECOM_APP**。本次没有把 mock、页面可打开或错误态当成真实员工业务通过。

## 7. 真实与模拟边界

- 真实：服务器容器、MySQL/Redis、迁移、Nginx、健康端点、生产静态资源、现有 Dashboard 会话和 RBAC、bridge 内网鉴权、专用回调路由。
- 合成：非法 GET 签名、非法 POST XML、cron fake bridge smoke 和 SDK 自检，仅证明失败关闭、路由和内部数据合同。
- 已有真实证据：企业微信后台真实 GET challenge、两次真实存档事件 POST、官方 Finance SDK `GetChatData/DecryptData`、`seq=0→4`、4 条脱敏证据、空页幂等和 bridge 重启续拉。
- 尚无真实证据：主程序 MySQL 消息落库、Dashboard 会话回读、媒体 `GetMediaData` 文件下载、真实 Sidebar 员工 OAuth/JSSDK。当前图片记录只证明图片类型元数据被拉取和解密，不证明媒体文件已经下载。
- 当前企业记录仍未在 Dashboard/主库启用完整的会话存档凭据，因此主应用在两次真实事件上均明确记录 `archive is not enabled`，cron 的 `corps=0` 是正确的受限状态；不能把隔离 bridge 的 4 条 JSONL 证据冒充主库数据。
- 真实证据回滚点：`/opt/mochat-go/backups/wecom-archive-live-evidence-20260826T212700CST`，目录权限 `0700`、文件权限 `0600`，`SHA256SUMS` 已验证。该目录含受限消息证据，只能留在服务器审计范围内，不得上传到仓库或普通报告。

## 8. 回滚步骤

主应用优先回滚到本次最终 UI 变更前的已健康版本：

```bash
cd /opt/mochat-go/deploy/standalone
docker image tag mochat-go-rollback:pre-cacdc740a14a standalone-app:latest
docker compose --env-file .env.local -f docker-compose.yml up -d --no-build --no-deps --force-recreate app
```

若需回到整个会话存档部署前状态：

1. 使用备份目录中的 Compose、环境文件和 Nginx 配置恢复原配置；恢复前再次核对目标路径和校验和。
2. 把 `mochat-go-rollback:pre-wecom-archive-e1546ad6ad90` 标记为 `standalone-app:latest`，只重建 app。
3. 停止当前 bridge，把保留的部署前 bridge 容器恢复原名并启动；不要删除 `/opt/wecom-archive-demo/data`、数据库或命名卷。
4. 仅当迁移兼容性检查明确要求且用户授权时才恢复数据库备份；本次没有新增数据库迁移，通常不需要数据库回滚。
5. 回滚后重新检查容器健康、`/healthz`、`/readyz`、关键页面和日志。

禁止使用 `docker compose down -v`、`docker volume prune`、`docker system prune`、`git reset --hard` 或清空生产数据库。

## 9. 下一步与剩余风险

1. 将本次已验证的会话存档 Secret、RSA 密钥对和 CorpID 通过受保护的服务端流程写入 MoChat 加密凭据存储，并启用 `mc_corp.id=4` 的归档状态；不得经聊天、HTTP 明文页面或命令参数传递 Secret。
2. 复验真实事件触发与 15 秒定时兜底共用主库游标，完成 MySQL 幂等落库、Dashboard 回读和失败不推进游标。
3. 修复/补齐“企业应用配置 → Sidebar 应用记录 → 员工 OAuth/JSSDK”的一致化链路；当前“应用不存在”是明确产品缺口。
4. 为验收账号授予数据概览、AI 设置、AI 洞察所需页面权限，或提供已登录的对应角色；另提供 SaaS Admin 已登录态。
5. 在真实 390×844 企微客户端中复验 Sidebar 三工作区、12 路由、客户上下文、刷新、权限、空态和失败态。
6. bridge JSONL 与 seq 已通过真实验证；待数据库消息、敏感词消费和 Dashboard 回读闭合后，再决定是否把会话存档 Provider 从 `limited` 提升。
7. 临时服务器登录密码已在会话中暴露，应立即轮换并改用 SSH Key；报告不记录该密码。
