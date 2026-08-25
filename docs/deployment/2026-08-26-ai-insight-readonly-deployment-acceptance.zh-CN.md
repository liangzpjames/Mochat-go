# AI 洞察只读工作区修复部署与验收报告

日期：2026-08-26（Asia/Shanghai）

服务器：`139.196.34.133`

验收范围：已登录 Dashboard 全部 53 个菜单页面、5 个 AI 洞察故障页、本地门禁、可回滚部署、数据库只读边界与重启复验。

## 结论

方案 A 已完成设计、实现、部署和自测。最终部署代码为 `552c0a18f5f569759e09d1032b1e1b042b16a544`，镜像为 `sha256:4b50c04bf2911ba7eecee2f775958466627c9f52a4324799463c042464cdb9fb`。

修复前，5 个 AI 洞察工作区页面的 records、status、filter-options 接口落入 standalone 兼容层并返回 HTTP 501，前端显示 Zod 原始校验错误。最终修复后，这些接口均返回 HTTP 200 和标准信封；页面如实显示 AI Provider 不可用、尚未运行及真实空态。Dashboard 全部 53 个菜单路由经真实链接逐一点击，53/53 可达；没有 501/未迁移、Zod 原始错误、破图、横向溢出或浏览器 warning/error。

最终部署后执行了容器实际重启。重启后 app 为 running/healthy，`/healthz` 与 `/readyz` 均为 200，会话分析再次刷新通过。自最终部署开始至页面验收、重启和复验结束，AI 助手、规则、规则版本、AI 设置审计四类新增记录均为 0，证明本次 GET 状态读取没有初始化或修改业务配置。

## 根因与最终修复

第一层根因位于 `internal/modules/ai-insight/module.go`：租户 AI Provider Resolver 为 `nil` 时不创建 WorkspaceHandler，导致只读工作区路由也没有注册。最终实现改为数据库依赖存在时始终创建 WorkspaceHandler，Resolver 可为空。

独立终审随后发现第一版修复存在潜在只读副作用：状态接口会调用系统助手初始化；对已有企业通常因 `INSERT IGNORE` 成为无操作，但新企业可能在 GET 请求下写入助手、规则、版本和审计。发现后立即停止相关页面调用，将生产 app 回滚到部署前镜像，并对数据库进行只读核查；试运行窗口内没有实际新增上述记录。

最终提交 `552c0a18` 在 `internal/modules/ai-insight/workspace_handler.go` 收紧边界：只有 Resolver 存在时才执行系统助手初始化和加载。新增回归测试证明：

- Resolver 为空时，GET status 返回 200、Provider unavailable，助手初始化/加载调用次数为 0。
- Resolver 为空时，手动 run 在助手初始化前返回 `503 / AI_PROVIDER_UNAVAILABLE`。
- records、detail、filter-options 等只读接口继续读取认证租户/企业范围内的真实 Repository 数据。
- 未修改 RBAC、租户/企业/员工范围、模型运行或后台日任务逻辑。

## 版本与产物

| 项目 | 部署前/回滚点 | 最终部署 |
| --- | --- | --- |
| 可追溯代码 | 服务器旧镜像对应版本 `2fb9256aa0ed…` | `552c0a18f5f569759e09d1032b1e1b042b16a544` |
| 修复分支 | — | `fix/ai-insight-readonly-20260826` |
| app 镜像 | `sha256:ef220d190470462c5de856b2c9fd657f463b562890e86f72365795c6e6523eaf` | `sha256:4b50c04bf2911ba7eecee2f775958466627c9f52a4324799463c042464cdb9fb` |
| 镜像标签 | `mochat-go-rollback:pre-ai-insight-readonly-20260826` | `mochat-go-deploy:552c0a18f5f5` / `standalone-app:latest` |
| 镜像归档大小 | — | `80,013,312` 字节 |
| 镜像归档 SHA-256 | — | `cc62a0126219fd2f6bb9a5b082ea0407a119305b188cca34802eba10aabb4702` |

第一版试运行镜像 `fa56704a…0ddb1673` 已撤回，不是最终生产镜像。其归档为审计目的保留，不能作为本报告的最终部署产物。

## 本地门禁

- TDD 红灯：最初路由注册测试失败，证据为 `workspace handler is nil`；只读副作用回归测试随后红灯，证据为 status GET 触发了 1 次助手初始化。
- TDD 绿灯：路由注册、status 无写入、run 提前失败及 AI 洞察相关测试全部通过。
- 全量 Go：`go test ./... -count=1`，92 个包，失败 0。
- 格式：目标 Go 文件 `gofmt -l` 无输出。
- 静态检查：AI 洞察与 `cmd/mochat-go` 的 `go vet` 退出码 0。
- Docker：根 Dockerfile 完整构建通过；仅有既有前端 bundle 大小警告。
- 独立终审：最终代码无 Critical、Important 或 Minor 缺陷，可合并。

## 服务器备份、部署与回滚点

部署前与安全回滚后状态：旧 app 镜像 `ef220d19…523eaf` 为 running/healthy，公网健康和就绪端点均为 200。

- 回滚镜像：`mochat-go-rollback:pre-ai-insight-readonly-20260826`，仍指向旧镜像。
- 配置备份：`/opt/mochat-go/backups/20260826-ai-insight-readonly-predeploy/`。
- Compose 与 `.env.local` 备份 SHA-256 与原文件一致；目录权限 700、文件权限 600。
- 最终归档：`/opt/mochat-go/deploy/standalone/releases/552c0a18f5f5/mochat-go-552c0a18f5f5.tar`，权限 600；服务器端大小和 SHA-256 与本地一致。
- 最终切换开始：`2026-08-26T01:25:27+08:00`。
- 最终 app 启动：`2026-08-25T17:25:29.020731635Z`。
- 实际重启请求：`2026-08-26T01:34:09+08:00`；重启后启动时间 `2026-08-25T17:34:10.174164729Z`。

部署仅加载、校验并标记新镜像，使用现有 Compose 强制重建 app 服务。没有重建或删除 MySQL、Redis、数据库、命名卷和现有配置。

## Dashboard 页面验收

### 5 个修复页

| 页面 | 接口 | 页面/刷新 | 控制台、资源、布局 |
| --- | --- | --- | --- |
| 会话分析 | records/status 200 | `AI 服务暂不可用：尚未运行`，真实 0 条；刷新通过 | warning/error 0、破图 0、无溢出 |
| 智能分析 | records/status 200 | `AI 服务暂不可用：尚未运行`，真实空态；刷新通过 | warning/error 0、破图 0、无溢出 |
| 情绪识别 | records/status/filter-options 200 | `AI 服务不可用：尚未运行`，真实空态；刷新通过 | warning/error 0、破图 0、无溢出 |
| 员工评分 | records/status/filter-options 200 | `AI 服务不可用：尚未运行`，真实空态；刷新通过 | warning/error 0、破图 0、无溢出 |
| 沟通关键词 | records/status/filter-options 200 | `AI 服务不可用：尚未运行`，真实空态；刷新通过 | warning/error 0、破图 0、无溢出 |

Nginx 脱敏日志确认上述请求均为 GET 200。重启后再次点击并刷新会话分析，仍为 200 和真实空态。

### 全菜单回归

在 1280 像素宽视口下展开全部菜单组，将 53 个真实菜单链接分 7 批逐一点击：

- 页面可达：53/53。
- 点击或路径不一致：0。
- 501 / 未迁移：0。
- Zod 原始错误：0。
- 破图：0。
- 横向溢出：0。
- 浏览器 warning/error：0。

逐批路由和检查项见[浏览器与服务器脱敏验收证据](evidence/2026-08-26-ai-insight-readonly-browser-server.zh-CN.md)。

## 数据库只读边界

最终部署前以 `2026-08-26 01:20:00 +08:00` 为窗口核查，四类新增记录均为 0。最终部署后，以 `2026-08-26 01:25:27 +08:00` 为窗口，在完成 5 页刷新、53 页点击、容器重启和重启后刷新后再次核查：

| 表/业务对象 | 新增数 |
| --- | ---: |
| `mochat_go_ai_agents` | 0 |
| `mochat_go_ai_analysis_rules` | 0 |
| `mochat_go_ai_analysis_rule_versions` | 0 |
| `mochat_go_ai_settings_audits` | 0 |

第一版试运行产生写入风险时也已核对：现有系统助手、规则和版本创建时间均为 2026-08-25 19:00:21，早于试部署；试运行窗口没有新审计或新配置记录。因此没有业务数据需要回滚。

## 真实数据与验收边界

- 未创建、编辑或删除业务数据，未使用 mock 或测试夹具冒充生产验收。
- AI 洞察结果为服务器真实 0 条；Provider Resolver 关闭，页面如实显示不可用/尚未运行。
- 企业资料中的 `wecom.sync_stale` 是既有企微同步告警，本次未修改外部企微配置。
- 本次用户要求针对已登录 MoChat Dashboard 挨页点击和修复；SaaS Admin 与员工 Sidebar 没有本次代码改动，未在本轮重复完整业务验收，不能用本报告替代两端专项验收。
- 本轮桌面验收视口为 1280 像素；未把该结果冒充移动端 390×844 验收。

## 执行命令类别

实际执行了 Git/worktree/SHA 核对、Go 测试/格式/vet、Docker build/save/load/tag/inspect、文件大小与 SHA-256 校验、权限受限备份、Compose app-only 重建、容器健康与实际重启、HTTP 健康端点、Nginx/容器日志脱敏筛查、数据库只读查询，以及已登录浏览器的菜单链接点击、刷新、文本状态、资源、控制台和布局检查。登录凭据未写入命令、文件、仓库、报告、截图或证据。

## 证据路径

- 设计：`docs/superpowers/specs/2026-08-26-ai-insight-readonly-workspace-design.md`
- 实现计划：`docs/superpowers/plans/2026-08-26-ai-insight-readonly-workspace.md`
- 最终本地门禁：`.superpowers/sdd/ai-insight-readonly-final-verify-report.md`
- 浏览器与服务器脱敏证据：`docs/deployment/evidence/2026-08-26-ai-insight-readonly-browser-server.zh-CN.md`
- 重启后辅助截图：`.superpowers/sdd/evidence/ai-insight-session-analysis-final-552c-post-restart.png`
- 服务器访问日志：`/var/log/nginx/access.log`
- 服务器 app 日志：`docker logs standalone-app-1`
- 服务器配置备份：`/opt/mochat-go/backups/20260826-ai-insight-readonly-predeploy/`
- 服务器最终镜像归档：`/opt/mochat-go/deploy/standalone/releases/552c0a18f5f5/mochat-go-552c0a18f5f5.tar`

## 回滚步骤

如需回滚，只恢复 app 镜像并重建 app 服务：

```bash
docker image tag mochat-go-rollback:pre-ai-insight-readonly-20260826 standalone-app:latest
cd /opt/mochat-go/deploy/standalone
docker compose --env-file .env.local up -d --no-deps --no-build --force-recreate app
```

随后等待 `standalone-app-1` healthy，核对镜像恢复为 `ef220d19…523eaf`，检查 `http://127.0.0.1:18080/healthz`、`/readyz` 和关键页面。数据库与配置未改动，无数据回滚步骤；禁止删除 MySQL/Redis 卷或生产数据库。

## 剩余风险与建议

- AI Provider 与分析任务仍按服务器当前配置关闭；如需产生 AI 结果，应另行完成密钥、配额、费用和数据合规审批。
- `wecom.sync_stale` 需在外部企微同步链路中另行处理。
- Docker 构建仍有既有前端 bundle 大小警告，可纳入后续性能优化。
- 临时服务器登录凭据应在本次工作完成后尽快轮换。
