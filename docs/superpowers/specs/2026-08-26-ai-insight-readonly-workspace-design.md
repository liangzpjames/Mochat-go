# AI 洞察只读工作区可用性修复设计

日期：2026-08-26  
状态：已获用户批准（方案 A）

## 背景与现象

服务器 Dashboard 已逐一点击验收 53 个菜单页面。除 AI 洞察外，其余页面未发现页面崩溃、静态资源损坏、控制台报错或横向溢出。AI 洞察下列 5 个页面稳定失败：

- 会话分析
- 智能分析
- 情绪分析
- 员工评分
- 沟通关键词

这些页面请求 `/dashboard/ai-insight/*/records`、`status` 和 `filter-options` 时收到 HTTP 501。响应来自 standalone 兼容层，字段为 `code/message/path`；Dashboard API 客户端要求统一信封 `code/msg/data`，因此页面最终暴露了 Zod 校验原始错误。

根因位于 AI 洞察模块装配：当 `MOCHAT_GO_AI_INSIGHT_ENABLED=0` 时，租户 AI Provider Resolver 为 `nil`，`internal/modules/ai-insight/module.go` 因此不创建 WorkspaceHandler，也不注册工作区子路由。现有 WorkspaceHandler 已把模型调用与只读查询分开：records、detail、filter-options 等只读取数据库；status 在 resolver 为空时可返回 `unavailable`；只有手动运行分析需要 resolver。因此“无 Provider 就不注册整个工作区”与现有处理器能力不一致。

## 目标

1. 无论 AI Provider 是否启用，只要 AI debt clearance 模块启用且数据库依赖有效，就注册 AI 洞察只读工作区路由。
2. Provider 未启用或未配置时，5 个页面返回真实记录/真实空态以及明确的 Provider 不可用状态，不再返回兼容层 501。
3. 手动运行分析仍受现有权限和 Provider Resolver 双重保护；不得因本次修复触发模型调用或后台分析任务。
4. 保持租户、企业和员工范围授权逻辑不变，不引入跨租户或越权读取。
5. 修复后回归 AI 洞察 5 页，并对 Dashboard 全部 53 页重新进行浏览器冒烟。

## 非目标

- 不开启 `MOCHAT_GO_AI_INSIGHT_ENABLED`、日分析或启动即运行开关。
- 不创建、补造或修改业务数据。
- 不修改租户 AI Provider 密钥、模型配置或企微配置。
- 不把 `wecom.sync_stale` 等真实外部 Provider 状态伪装成成功。
- 不调整 AI 洞察页面视觉样式或扩展业务功能。

## 方案

在 `internal/modules/ai-insight/module.go` 中，只要数据库依赖存在，就始终构造 WorkspaceHandler：

```text
DB 存在
  └─ 创建 Repository 与 WorkspaceHandler
       ├─ resolver 存在：只读查询 + 状态解析 + 授权后的分析运行
       └─ resolver 为空：只读查询 + unavailable 状态；分析运行返回 503
```

保留 WorkspaceHandler 内现有安全边界：

- `runNow` 在 resolver 为空时返回统一信封 `503 / AI_PROVIDER_UNAVAILABLE`。
- `status` 在 resolver 为空时返回 Provider `unavailable` 状态。
- 页面只读接口不解析或调用模型，只访问当前认证租户/企业范围内的 Repository。
- 路由仍由 Dashboard 身份守卫、访问守卫和 AI 洞察 Authorizer 保护。

本次不在前端兼容非标准 501 响应，因为那只会隐藏未注册路由，无法恢复真实功能；也不通过开启 AI 模型开关修复，因为这会扩大运行时行为并可能触发外部调用。

## 测试设计

采用测试驱动方式：

1. 先在 AI 洞察模块测试中增加失败用例：DB 存在、Resolver 为 `nil` 时，WorkspaceHandler 必须存在并注册 records/status/run 等路由。
2. 验证无 Resolver 状态接口返回标准工作区信封，且 Provider 状态为 `unavailable`。
3. 验证无 Resolver 手动运行返回 503，且不调用任何模型。
4. 运行 AI 洞察、bootstrap 和 `cmd/mochat-go` 相关 Go 测试。
5. 运行全量 `go test ./...`、Go 格式检查以及仓库定义的相关门禁。
6. 构建与服务器一致的 Docker 镜像，在本地或隔离运行环境执行 `/healthz`、`/readyz` 与接口冒烟。
7. 部署后用已登录浏览器逐页复验 5 个 AI 洞察页面，再回归全部 53 个 Dashboard 页面；记录 HTTP 状态、页面错误、控制台日志和响应式布局结果。

## 部署与回滚

部署前记录当前容器、镜像 ID、Compose 配置路径、功能开关和健康状态，并为当前镜像增加只读回滚标签；备份实际将修改的 Compose/环境配置（预计本修复无需修改配置）。新镜像使用修复提交 SHA 标记并记录摘要。

部署采用现有 `/opt/mochat-go/deploy/standalone/docker-compose.yml` 流程，只替换 app 镜像，不删除数据库、Redis、命名卷或现有配置。部署后验证容器健康、`/healthz`、`/readyz`、关键日志、Nginx 代理和重启后状态。

若验证失败，恢复部署前镜像标签并重新创建 app 容器，随后再次检查健康端点和关键页面。数据库结构与数据不在本次变更范围内，因此不需要数据回滚。

## 成功标准

- 5 个 AI 洞察页面不再出现 HTTP 501 或 Zod 原始错误。
- Resolver 关闭时，页面展示真实空态/记录和明确的 Provider 不可用状态。
- 手动分析与日分析仍保持关闭，未发生外部模型调用。
- Dashboard 全部 53 页无新增明显回归。
- `/healthz`、`/readyz`、容器健康、关键日志及重启后状态均正常。
