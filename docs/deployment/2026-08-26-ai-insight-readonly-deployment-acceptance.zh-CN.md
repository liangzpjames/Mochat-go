# AI 洞察只读工作区修复部署与验收报告

日期：2026-08-26（Asia/Shanghai）

服务器：`139.196.34.133`

验收范围：已登录 Dashboard 全菜单专项审计、AI 洞察修复、本地门禁、可回滚部署及复验

## 结论

本次修复已部署并通过验收。修复前，5 个 AI 洞察页面的工作区接口稳定返回 HTTP 501，Dashboard 将非标准错误信封显示为 Zod 原始校验错误。修复后，这些接口均返回 HTTP 200 和标准 `code/msg/data` 信封；页面显示真实 0 条空态及“AI 服务暂不可用：尚未运行”，未开启 AI 模型、日分析或启动即运行能力。

部署后已重新实际点击 Dashboard 全部 53 个菜单页面，结果为 53/53 可达；未发现 501、Zod 原始错误、破图、横向溢出或浏览器控制台错误。容器完成一次实际重启后仍为 healthy，`/healthz` 与 `/readyz` 均返回 200，重启后会话分析再次刷新通过。对应的时间戳、脱敏日志摘录、分批路由清单及接口/健康输出已固化至[浏览器与服务器脱敏验收证据](evidence/2026-08-26-ai-insight-readonly-browser-server.zh-CN.md)。

## 根因与修复

根因位于 `internal/modules/ai-insight/module.go`：当租户 AI Provider Resolver 为 `nil` 时，模块不创建 WorkspaceHandler，导致 records、status、filter-options 等只读工作区路由也没有注册，请求落入 standalone 兼容层并返回 501。

修复保持既有安全边界，只在数据库依赖存在时始终创建 WorkspaceHandler，并允许 Resolver 为 `nil`：

- records、detail、filter-options 等只读接口继续读取当前认证租户/企业范围内的真实 Repository 数据。
- status 返回 Provider `unavailable` 状态。
- 手动分析仍在 Resolver 为空时返回 `503 / AI_PROVIDER_UNAVAILABLE`。
- 未修改 RBAC、租户/企业/员工范围、模型运行和后台日任务逻辑。

## 版本与产物

| 项目 | 部署前 | 部署后 |
| --- | --- | --- |
| 可追溯代码提交 | `2fb9256aa0ede33d6e0aecd50e11c3b2200d6834` | `3d9341a4937d72cde730cb1ffc588e3ac3ba5097` |
| 文档分支基线 | `963e763edca509a4942a4d914acb1fabea759482` | 修复分支 `fix/ai-insight-readonly-20260826` |
| app 镜像 | `sha256:ef220d190470462c5de856b2c9fd657f463b562890e86f72365795c6e6523eaf` | `sha256:fa56704a6ed676de2812b0899fecf39ad9fb8a0cea053f29e73d3d4a0ddb1673` |
| 镜像标签 | `mochat-go-deploy:2fb9256aa0ed` | `mochat-go-deploy:3d9341a4937d` / `standalone-app:latest` |
| 镜像归档大小 | — | `80,014,336` 字节 |
| 镜像归档 SHA-256 | — | `e6cdc3b76fbca9c406919bb17113cd1c3f6d283230edc696778be4e4d66d2167` |

## 本地门禁

- 修复前基线：AI 洞察、bootstrap、`cmd/mochat-go` 相关包测试通过。
- TDD 红灯：`TestModuleRegistersWorkspaceRoutesWithoutAIProviderResolver` 首次失败，证据为 `workspace handler is nil`。
- TDD 绿灯：目标测试、全部 Workspace 测试及相关包测试通过。
- 全量 Go 门禁：`go test ./... -count=1`，共 92 个包，失败 0。
- 格式：`gofmt -l` 无输出。
- 静态检查：AI 洞察与 `cmd/mochat-go` 的 `go vet` 退出码 0。
- Docker：根 Dockerfile 完整构建 Go 服务及四个前端生产产物；构建成功，仅有既有的前端 bundle 大小警告。
- 独立任务审查：规格符合、代码质量通过，无 Critical、Important 或 Minor 缺陷。

## 服务器备份与回滚点

部署前状态：

- 容器：`standalone-app-1`，running / healthy，重启次数 0。
- 健康检查：`/healthz`、`/readyz` 均成功。
- 回滚镜像标签：`mochat-go-rollback:pre-ai-insight-readonly-20260826`，指向部署前镜像 `ef220d19…523eaf`。
- 配置备份：`/opt/mochat-go/backups/20260826-ai-insight-readonly-predeploy/`。
- Compose 与 `.env.local` 备份 SHA-256 分别与原文件完全一致；备份目录权限为 700，文件权限为 600。
- 上传归档：`/opt/mochat-go/deploy/standalone/releases/3d9341a4937d/mochat-go-3d9341a4937d.tar`，权限 600；服务器端大小和 SHA-256 与本地一致。

部署只加载新镜像、更新 `standalone-app:latest` 并使用现有 Compose 重新创建 app 服务。未重建或删除 MySQL、Redis、数据库、命名卷和现有配置。

## Dashboard 页面验收

### 修复前全菜单审计

已登录用户在 1280 像素宽视口下实际点击全部 53 个菜单页面。除 AI 洞察 5 页外，其余页面无崩溃、破图、控制台错误或横向溢出；空态、未接入提示和 Provider 状态均按服务器真实能力显示。

修复前故障页：

| 页面 | 修复前接口 | 修复前页面结果 |
| --- | --- | --- |
| 会话分析 | records/status 为 501 | Zod 原始错误 |
| 智能分析 | records/status 为 501 | Zod 原始错误 |
| 情绪识别 | records/status/filter-options 为 501 | Zod 原始错误 |
| 员工评分 | records/status/filter-options 为 501 | Zod 原始错误 |
| 沟通关键词 | records/status/filter-options 为 501 | Zod 原始错误 |

### 修复后专项复验

| 页面 | 接口结果 | 页面状态 | 刷新 | 控制台/资源/布局 |
| --- | --- | --- | --- | --- |
| 会话分析 | 200 | `AI 服务暂不可用：尚未运行`，真实 0 条 | 通过 | 0 日志、0 破图、无溢出 |
| 智能分析 | 200 | `AI 服务暂不可用：尚未运行`，真实空态 | 通过 | 0 日志、0 破图、无溢出 |
| 情绪识别 | 200 | `AI 服务不可用：尚未运行`，真实空态 | 通过 | 0 日志、0 破图、无溢出 |
| 员工评分 | 200 | `AI 服务不可用：尚未运行`，真实空态 | 通过 | 0 日志、0 破图、无溢出 |
| 沟通关键词 | 200 | `AI 服务不可用：尚未运行`，真实空态 | 通过 | 0 日志、0 破图、无溢出 |

Nginx 访问日志确认上述 records、status 和 filter-options 请求均为 200，不再出现 501；脱敏的路径、方法、状态及采样窗口见[浏览器与服务器脱敏验收证据](evidence/2026-08-26-ai-insight-readonly-browser-server.zh-CN.md#服务器脱敏-nginx-访问日志摘录)。容器日志未发现 panic、fatal、迁移失败、SQL 错误或跨租户异常。

### 修复后全菜单回归

将 53 个菜单路由分成 7 批逐一实际点击并采样；逐批的完整路由顺序、空失败集和每批计数见[浏览器：53 路由点击记录](evidence/2026-08-26-ai-insight-readonly-browser-server.zh-CN.md#浏览器53-路由点击记录)：

- 页面可达：53/53。
- 点击失败：0。
- 501 / 未迁移提示：0。
- Zod 原始错误：0。
- 破图：0。
- 横向溢出：0。
- 浏览器控制台日志：0。

## 真实数据与边界

- 本次没有创建或修改任何业务数据，也没有把 mock、测试夹具或页面可打开冒充真实业务验收。
- AI 洞察当前结果为真实 0 条；Provider Resolver 关闭，因此页面如实显示 AI 服务不可用/尚未运行。
- 全局消息中的“部分能力暂无系统数据”、营销 Provider 未配置等提示属于真实能力边界，未伪装为成功。
- 企业资料中的 `wecom.sync_stale` 是既有企微同步状态告警，本次按批准范围保留，未修改外部企微配置。
- 本次专项变更只重新验收已登录 Dashboard；SaaS Admin 与员工 Sidebar 未发生代码变更，未在本轮重复执行完整业务验收。

## 执行命令类别

实际执行的命令类别包括：Git/worktree 状态与提交核对、Go 依赖/测试/格式/vet、Docker build/save/load/tag/inspect、文件大小与 SHA-256 校验、受限权限备份、Compose 仅 app 重建、容器健康/重启检查、HTTP 健康端点、Nginx/容器日志筛查，以及已登录浏览器的菜单点击、刷新、页面状态、资源、控制台和布局检查。登录密码未写入命令、文件、仓库、报告或证据。

## 证据路径

- 设计：`docs/superpowers/specs/2026-08-26-ai-insight-readonly-workspace-design.md`
- 实现计划：`docs/superpowers/plans/2026-08-26-ai-insight-readonly-workspace.md`
- TDD 报告：`.superpowers/sdd/ai-insight-readonly-task-1-report.md`
- 本地门禁/镜像报告：`.superpowers/sdd/ai-insight-readonly-task-2-report.md`
- 浏览器与服务器脱敏验收证据：`docs/deployment/evidence/2026-08-26-ai-insight-readonly-browser-server.zh-CN.md`
- 重启后页面截图：`.superpowers/sdd/evidence/ai-insight-session-analysis-post-restart.png`，仅为“会话分析”单页辅助证据；完整 5 页刷新和 53 页覆盖以结构化点击记录与脱敏访问日志为证。
- 服务器访问日志：`/var/log/nginx/access.log`
- 服务器容器日志：`docker logs standalone-app-1`
- 服务器配置备份：`/opt/mochat-go/backups/20260826-ai-insight-readonly-predeploy/`
- 服务器镜像归档：`/opt/mochat-go/deploy/standalone/releases/3d9341a4937d/mochat-go-3d9341a4937d.tar`

## 回滚步骤

如需回滚，仅恢复 app 镜像并重建 app 服务：

```bash
docker image tag mochat-go-rollback:pre-ai-insight-readonly-20260826 standalone-app:latest
cd /opt/mochat-go/deploy/standalone
docker compose --env-file .env.local up -d --no-deps --no-build --force-recreate app

# 最多等待 50 秒，超时则保留容器状态和最近日志供排查。
deadline=$((SECONDS + 50))
while :; do
  app_id="$(docker compose --env-file .env.local ps -q app)"
  if [ -n "$app_id" ] && [ "$(docker inspect -f '{{.State.Health.Status}}' "$app_id")" = healthy ]; then
    break
  fi
  if [ "$SECONDS" -ge "$deadline" ]; then
    docker compose --env-file .env.local ps
    docker compose --env-file .env.local logs --since 5m app
    exit 1
  fi
  sleep 2
done

curl -fsS -o /dev/null -w 'HEALTHZ_HTTP=%{http_code}\n' http://127.0.0.1:18080/healthz
curl -fsS -o /dev/null -w 'READYZ_HTTP=%{http_code}\n' http://127.0.0.1:18080/readyz
docker inspect -f 'IMAGE={{.Image}} HEALTH={{.State.Health.Status}}' "$app_id"
docker compose --env-file .env.local logs --since 5m app
```

上述命令只将 `.env.local` 作为 Compose 输入，不读取或显示其内容。若健康等待、HTTP 检查或镜像核对失败，应停止回滚验收并保留已输出的状态和最近 5 分钟 app 日志。成功时，检查 `standalone-app-1` 镜像 ID 应恢复为 `ef220d19…523eaf`，并复查 `/healthz`、`/readyz` 和关键页面。数据库与配置未改动，无数据回滚步骤。

## 剩余风险与建议

- AI Provider 与分析任务仍按服务器当前配置关闭；如需产生 AI 洞察结果，应另行配置租户 Provider 并经过密钥、配额、费用和数据合规审批，不能视为本次页面修复的一部分。
- `wecom.sync_stale` 需要在外部企微同步链路中单独排查。
- Docker 构建仍有前端 bundle 大小警告，不影响本次发布，但可纳入后续性能优化。
- 临时服务器登录凭据应在本次工作完成后尽快轮换。
