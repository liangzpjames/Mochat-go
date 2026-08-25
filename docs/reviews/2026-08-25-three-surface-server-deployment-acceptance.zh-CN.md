# MoChat Go 本地构建、服务器部署与三端验收报告

日期：2026-08-25（Asia/Shanghai）

## 1. 最终结论

- **部署结果：成功。** 服务器应用已从旧镜像切换为最终审阅修复提交 `2fb9256aa0ede33d6e0aecd50e11c3b2200d6834` 对应产物；应用、MySQL、Redis 均健康，`/healthz`、`/readyz`、SaaS Admin、Dashboard、Sidebar 和 Operation 入口在部署后及应用容器重启后均返回 200。
- **数据库迁移：成功。** 服务器从 `0139` 升级到 `0165`，新应用复验为 165 条迁移全部 applied、0 pending、0 checksum mismatch。迁移前后的用户、企业、租户数量均为 `3 / 3 / 2`。
- **三端公开入口和受控回归：通过。** 相关 Playwright 用例 77/77 通过；Dashboard 和 Sidebar 在 390×844 下无页面级横向溢出；Sidebar 12 路由的服务器静态可达性为 12/12。
- **三端真实受保护业务验收：未完成，不能宣称生产业务全通过。** 服务器浏览器没有 SaaS Admin 或 Dashboard 有效会话；部署配置中的 Dashboard 引导凭据真实登录返回 401；Sidebar 缺少有效企业微信应用 ID。租户详情、租户 AI Provider 页面、Dashboard 主框架和真实员工侧业务接口因此分别记录为阻塞或外部条件跳过。

本次没有把测试夹具、模拟接口、页面能打开或 HTTP 200 冒充真实生产业务验收。

## 2. Git、隔离工作区与发布版本

- 权威远端主线：`origin/main = c696aa66400b5cb7185f18a4152ea6c0ac593834`。
- 隔离 worktree：`D:\workspace\mochat-go\mochat-go\.worktrees\deploy-three-surfaces-20260825`。
- 隔离分支：`deploy/three-surfaces-20260825`。
- 初始部署阻断修复提交：`e79456cab0c20540ebe5a2174b891f69c05ec341`，提交说明为 `fix: preserve deployed migration checksum compatibility`。
- 最终审阅修复提交：`2fb9256aa0ede33d6e0aecd50e11c3b2200d6834`，提交说明为 `fix: constrain legacy migration checksum alias`；最终服务器镜像以此提交构建。
- 最终报告提交后，该隔离分支相对 `origin/main` 共包含 4 个本地提交，未推送、未合并；原主工作树和其他已有 worktree 的未提交/未跟踪内容均未修改。
- 服务器 `/opt/mochat-go` 不是 Git 工作树；可追溯源码归档保存在服务器 release 目录中。

最小修复内容：

1. `0106_scrm_lead_parity` 的历史已部署 SQL 含混合 LF/CRLF，去除 `\r` 后与当前 SQL 完全一致，但迁移器此前只兼容纯 LF 或纯 CRLF。
2. 先新增失败测试复现历史 checksum 不被接受，再只为 `0106` 增加已验证的历史 checksum 别名；代码审阅后又新增负例，确保只有当前 `0106` 规范化内容仍等于已验证 SHA-256 时才接受该历史值，任意 SQL 改写都会恢复为 mismatch。
3. 未修改迁移 SQL，未更新数据库 ledger，未放宽其他版本的 checksum 校验；同时修正过期的 Dashboard E2E 断言/夹具，并把 Sidebar 测试名称明确改为 fixture-backed，避免把夹具称为真实接口。

## 3. 本地构建与门禁

工具链：Node.js 22.18.0、pnpm 11.17.0、Go 1.26.5、Docker 29.6.2、Docker Compose 5.3.1。

实际执行的命令类别（不含任何密码）：

- Git：fetch、主线 SHA、status、worktree 清单和隔离 worktree 检查。
- 依赖：`pnpm install --frozen-lockfile`、`go mod download`。
- 前端：lint、typecheck、unit test、production build。
- Go：相关包测试、`go test ./... -count=1`。
- 浏览器：Dashboard auth/responsive 与 Sidebar employee mobile Playwright 套件。
- 容器：生产 Dockerfile build、Compose app-only recreate、health/readiness/静态入口检查。

结果：

- `pnpm lint`：通过。
- `pnpm typecheck`：通过。
- `pnpm test`：通过；其中 Dashboard 142 files / 918 tests、Sidebar 14 files / 153 tests、SaaS Admin 6 files / 26 tests 均通过。
- `go test ./... -count=1`：通过。
- 修复后 `internal/migration` 定向红/绿测试和整包测试：通过。
- 修复后 Playwright 三端相关套件：77/77 通过，耗时约 1 分钟。
- E2E typecheck：通过。
- 生产 Docker build：通过；Dashboard 主包约 980.46 kB、Sidebar 主包约 371.19 kB，仅有 bundle size warning。
- 最终本地应用容器镜像：`sha256:ef220d190470462c5de856b2c9fd657f463b562890e86f72365795c6e6523eaf`，状态 healthy、restart count=0。
- 本地 `/healthz`、`/readyz`、SaaS Admin 登录入口、Dashboard 登录入口、Sidebar（18091）、Operation（18092）均可用；本地迁移到 `0165` 且无 mismatch。`/dashboard/*` 是受保护 API 命名空间，未认证访问返回 401，不作为 Dashboard 静态入口。

受控 E2E 的边界：

- Dashboard：3 条认证边界用例和 29 条 390px 响应式路由用例；API 由 E2E contract fixture 提供。
- Sidebar：12 路由 × 3 视口共 36 条，加 1 条紧凑度和 8 条业务/空态/失败态/持久化状态用例，共 45 条；“客户 / 会话 / 我的”和业务数据均来自明确标注的 review fixture。

## 4. 服务器部署前现状

- 服务器：`139.196.34.133`，Ubuntu 24.04，正确 SSH 用户为 `root`。
- Compose 项目：`standalone`。
- Nginx：80 端口，`server_name 139.196.34.133 _`，`/` 代理到 `127.0.0.1:18080`。
- 应用端口：18080；Sidebar：18081；Operation：18082。
- 旧应用镜像：`sha256:ecd7effe2ddc55f533584481f493607257e3e883be13a8ac30ad5c129039fcd8`，构建于 2026-08-16，仅包含到 `0139`。
- MySQL 和 Redis 容器已连续运行约两周；本次没有重建它们。
- 保留的命名卷：`standalone_app-storage`、`standalone_audit-anchor-storage`、`standalone_mysql-data`、`standalone_redis-data`。
- 部署前数据基线：`mc_user=3`、`mc_corp=3`、`mc_tenant=2`。

## 5. 备份与回滚点

服务器回滚目录：

`/opt/mochat-go/output/deploy-backups/20260825T183621+0800`

已备份并验证：

- MariaDB 单事务全量转储：`mochat.sql.gz`，`gzip -t` 通过。
- 转储 SHA-256：`dccd9306a029c8093f4bd2f459f3835cf04f0eee06fba0db327f5c26feb06c05`。
- 旧镜像回滚标签：`mochat-go-rollback:20260825t183621p0800`，对应旧镜像 ID `sha256:ecd7effe...`。
- 最终审阅修复部署前还保留了中间镜像标签 `mochat-go-review-rollback:e79456cab0c2`，仅用于快速回到已完成 0165 迁移的中间应用版本；它不能替代 0139 数据库回滚点。
- Compose、`.env.local`、MFA key 文件、Nginx 配置、容器/卷清单、迁移状态和数据计数。
- 回滚目录权限为 700；环境文件、数据库转储权限为 600；备份文件有 `SHA256SUMS`。

## 6. 发布产物与服务器部署

最终发布代码提交：`2fb9256aa0ede33d6e0aecd50e11c3b2200d6834`。

- 本地镜像 ID / 服务器镜像 ID：`sha256:ef220d190470462c5de856b2c9fd657f463b562890e86f72365795c6e6523eaf`。
- 镜像 tar SHA-256：`9011427fd198e5f18b297b81055ef3b1490da8aaaef918fa0ae2449bc3f87a64`。
- Git 源码归档 SHA-256：`be64279adce8f581548506fcd1bb75d956ff33a4e6a69fe20a7f0d5fbc38f594`。
- source fingerprint：`6c6289c9098bc71c99896b348d00318a04926208d081a9b41c8cfd9de96d6e5f`。
- 最终服务器 release 目录：`/opt/mochat-go/releases/2fb9256aa0ed-20260825t1932p0800`。
- 源码归档扫描未包含 `.env.local`、node_modules、tmp、test-results 或 playwright-report。

服务器执行类别：只读盘点、数据库转储、旧镜像打标签、上传后 SHA-256 比对、`docker load`、迁移 status/apply、最新 Compose 校验与安装、app-only `compose up -d --no-build --no-deps app`、健康/就绪/日志/端口检查、应用容器 restart 复验。最终审阅修复镜像部署前再次执行真实数据库 status，结果为 0 pending / 0 mismatch；该次替换没有再次执行迁移。

迁移说明：

- 修复前首次 apply 被 `0106` checksum 门禁安全拦截，返回非零且没有应用新迁移。
- 中间修复镜像预检为 0 mismatch，成功应用 `0140`–`0165` 共 26 条；最终审阅修复镜像只做 status 和 app-only 替换。
- `0165` 会清理旧 `mochat_go_ai_analysis` 表；迁移前该表真实行数为 0，新目标表尚不存在，因此没有生产分析数据被删除。
- 部署后为 165 migrations、0 pending、0 mismatch；数据基线仍为 `3 / 3 / 2`。

部署后和 app restart 后：

- app 最终镜像 ID 与本地产物一致，running + healthy，restart 后仍 healthy、restart count=0。
- MySQL/Redis 容器 ID、运行时长和命名卷未改变。
- `/healthz`、`/readyz`、SaaS Admin 登录入口、Dashboard `/login`、Sidebar 和 Operation 均为 200；Sidebar 12 路由为 12/12。
- 应用启动日志未发现 error、panic、fatal 或 checksum mismatch。

## 7. 三端逐项验收

### A. SaaS Admin

| 项目 | 结果 | 证据与边界 |
|---|---|---|
| 公网登录入口 | 通过 | 浏览器加载 `http://139.196.34.133/saas-admin/?login=1`，标题和表单正常 |
| 静态资源、刷新、破图 | 通过 | JS `index-NOl_Unuj.js`、CSS `index-LYO_hPeB.css` 加载；刷新后仍正常；broken image=0 |
| 未认证 API 保护 | 通过 | overview、tenant detail、tenantAIProvider 均返回 401，未泄露数据 |
| 服务器租户列表/详情 | 阻塞 | 浏览器无有效 SaaS Admin 会话或凭据，不能越过登录门禁 |
| 服务器 AI 分析配置页 | 阻塞/空态 | 同上；服务器 `mochat_go_saas_tenant_ai_providers` 真实行数为 0 |
| API Key/密文保护 | 通过（存储/未认证边界） | 未认证接口 401；数据库只检查密文/提示长度元数据，未输出任何 Key、密文或提示值 |

补充：本地环境已有真实登录会话，已只读打开 3 个本地租户、租户详情和 AI Provider 配置弹窗；API Key 输入保持空白，仅显示末尾提示。此证据仅代表本地真实数据，不代表服务器生产租户验收。

### B. Dashboard

| 项目 | 结果 | 证据与边界 |
|---|---|---|
| 公网登录入口 | 通过 | `http://139.196.34.133/login` 正常渲染 |
| 静态资源、刷新 | 通过 | JS `index-CbQ457jt.js`、CSS `index-5tH_iMaR.css` 正常；broken image=0 |
| 390×844 响应式 | 通过 | innerWidth/scrollWidth 均为 390，无横向溢出 |
| 未认证 API 保护 | 通过 | statistic、company profile、AI setting 等代表接口返回 401 |
| 真实登录 | 阻塞 | 使用部署配置中的引导凭据在服务器内存中真实调用登录 API，返回 401；未重置密码、未暴力尝试 |
| 主框架、数据概览、企业资料、AI 设置/洞察 | 阻塞 | 无有效会话，不能宣称服务器受保护业务页通过 |
| 受控回归 | 通过 | Dashboard auth 3 条、390px responsive 29 条通过；明确属于 fixture/contract 测试 |

服务器真实数据边界：`corp_day_data=0`、`ai_agents=2`、`ai_knowledge_bases=0`、`ai_conversation_insights=0`。这些是数据库实况，不是模拟数据。

### C. 员工移动端 Sidebar

| 项目 | 结果 | 证据与边界 |
|---|---|---|
| 公网入口 | 通过 | `http://139.196.34.133:18081/contact` 可访问并跳转到登录 |
| 390×844 | 通过 | innerWidth/scrollWidth 均为 390，无破图、无横向溢出 |
| 12 路由服务器静态可达 | 通过 | `/`、`/auth`、`/codeAuth`、`/contact`、3 个 contact 子路由、`/contactBatchAdd`、`/contactSop`、`/login`、`/medium`、`/roomSop` 全部 200 |
| “客户 / 会话 / 我的” | 受控通过 | fixture-backed Playwright 在 390×844 覆盖三个工作区、刷新、空态、失败态、重试和持久化；不计作生产真实接口 |
| 真实员工接口 | 阻塞 | 无 Sidebar JWT；真实接口返回 401。服务器确有 `work_employee=5`、`work_contact=2`、`work_room=3` |
| 企业微信 OAuth/JSSDK | 跳过（外部条件） | 页面明确提示“缺少有效的企业应用 ID”；当前只有 HTTP/IP，无企微可信 HTTPS 域名和真实客户端上下文 |

## 8. 证据路径

本地发布产物与浏览器证据：

- `D:\workspace\mochat-go\output\deploy-20260825-2fb9256aa0ed`
- `D:\workspace\mochat-go\output\deploy-20260825-2fb9256aa0ed\browser\server-saas-login-final.png`
- `D:\workspace\mochat-go\output\deploy-20260825-2fb9256aa0ed\browser\server-dashboard-login-390x844-final.png`
- `D:\workspace\mochat-go\output\deploy-20260825-2fb9256aa0ed\browser\server-sidebar-login-error-390x844-final.png`
- 受控 Sidebar 截图：`D:\workspace\mochat-go\mochat-go\.worktrees\deploy-three-surfaces-20260825\docs\reviews\evidence\2026-08-23-sidebar-reference-replica`
- 失败定位 trace 解包：`D:\workspace\mochat-go\output\trace-inspect-8d60f3358600441c964a669798e6701b`

服务器证据：

- 最终 release：`/opt/mochat-go/releases/2fb9256aa0ed-20260825t1932p0800`
- 最终 release 内：`migration-preflight.txt`、`migration-post.txt`、`migration-final.txt`、对应 summary、`data-counts-post.txt`、`containers-post.txt`、`volumes-post.txt`、`restart-verify.txt`、`sidebar-routes-final.txt`、关键日志扫描结果。
- 原始迁移 apply 证据：`/opt/mochat-go/releases/e79456cab0c2-20260825t1900p0800`
- `SHA256SUMS`
- 回滚备份：`/opt/mochat-go/output/deploy-backups/20260825T183621+0800`

## 9. 回滚步骤

由于本次包含数据库迁移，不能只把镜像标签切回去就宣称完整回滚。若必须回滚，应在维护窗口内执行：

1. 先对当前 0165 数据库再做一次新的全量转储并校验，保留故障现场。
2. 仅停止 app 服务，不删除 MySQL/Redis 容器和任何命名卷。
3. 从回滚目录取回旧 `docker-compose.yml`、`.env.local` 和受保护密钥文件，先在隔离位置核对原权限和 DSN，不立即覆盖运行配置。
4. 在明确批准数据库恢复后，二选一执行：优先新建一个空数据库（保持原字符集、排序规则和授权）并将 0139 转储恢复到该空库；或在维护窗口、再次确认破坏性操作后，删除并重建当前应用数据库再恢复。**不得把 0139 转储直接导入仍含 0140–0165 对象的现有数据库。**
5. 将旧 DSN 指向已恢复的空库，恢复旧 Compose/配置，将 `mochat-go-rollback:20260825t183621p0800` 重新标记为 `standalone-app:latest`，再 app-only 启动。
6. 复验 ledger 仅到 `0139`，并检查 0140–0165 新增的表、索引和列没有残留；随后复验数据计数、`/healthz`、`/readyz`、日志、Nginx 和三端入口。

镜像回滚而不恢复数据库未验证为兼容路径，不建议用于生产回滚。

## 10. 剩余风险与待办

1. **高：服务器只有 HTTP/IP，没有受信任 HTTPS 域名。** 这阻断真实企业微信 OAuth/JSSDK，也不满足生产凭据和会话传输要求。
2. **高：3306 和 6379 当前监听 `0.0.0.0`/`[::]`。** 应在确认运维访问来源后收紧安全组、防火墙或改为 loopback/internal network；本次未擅自改变网络策略。
3. **高：三端真实受保护业务验收仍缺有效账号/会话。** 需要用户提供或现场操作有效 SaaS Admin、Dashboard 凭据；若启用 MFA，由用户完成 MFA。
4. **高：Sidebar 真实验收缺企微应用 ID、可信域名、回调/JSSDK 配置和真实企微客户端。** 应完成后单独复验真实 OAuth、JSSDK 和员工权限。
5. **中：服务器 AI Provider 为 0 条、AI 洞察为 0 条、概览日数据为 0 条。** 当前只能验证空态和权限边界，不能验证真实 AI 调用质量或生产指标。
6. **中：服务器部署目录没有 `.git`。** 当前通过 release 源码归档、commit SHA、镜像 ID 和 SHA-256 保证追溯；后续应固化标准发布清单。
7. **中：修复提交仍在隔离分支，未推送、未合并。** 本轮独立代码审阅提出的 checksum 兼容范围和回滚流程问题已经修复并复验，但若不把 `2fb9256` 合入权威主线，后续直接从 `origin/main` 构建仍会重新遇到服务器历史 checksum 阻断。
8. **低：Dashboard 和 Sidebar 构建仍有 bundle size warning。** 不阻断本次部署，但应后续做代码拆分。
9. 完成后应轮换本次临时服务器登录密码。
