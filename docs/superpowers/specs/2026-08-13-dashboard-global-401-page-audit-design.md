# Dashboard 全局异常 401 与 53 页逐页验收设计

## 1. 背景与问题结论

当前生产环境可以用已授权的 Dashboard 超级管理员进入 `/ai-insight/v2/risk`，菜单和页面路由均正常，但页面读取 `GET /dashboard/risk/records` 时显示加载失败，随后其他页面请求失去登录态。

根因不是页面没有授权，而是认证事实发生了分叉：

- `DashboardRequestGuard` 已经验证 JWT、服务端 session、用户状态和企业状态，并把 `DashboardPrincipal` 注入请求上下文；
- `DashboardAccessGuard` 已经验证套餐门槛、页面资源和数据范围，并把 `DashboardAccessContext` 注入请求上下文；
- 风险行为、超时预警、消息拦截等旧 handler 仍再次调用 `UserCorpCache` 读取旧 Redis 登录缓存；
- 新 Dashboard 账号和新 session 不保证写入旧缓存，因此旧 handler 把一个已经通过统一认证和授权的请求错误返回为 `401`；
- 前端 `ApiClient` 对任何 `401` 都清理 session，所以一次错误的页面接口响应会让整个 Dashboard 进入“页面壳仍显示、下一次导航回登录页”的僵尸状态。

这属于新旧认证机制混用，不能通过放宽鉴权、把 401 改成 200、或在登录时补写旧缓存来绕过。

## 2. 目标

1. Dashboard 生产页面 API 只使用统一认证链产生的 `DashboardPrincipal` 和 `DashboardAccessContext`，不再把旧 Redis 登录缓存作为第二套身份事实。
2. 已认证且已授权的页面请求不得无端返回 `401`。
3. 状态语义稳定：
   - 真实未登录、token/session 无效：`401 UNAUTHORIZED`；
   - 租户/SaaS 门槛失败：`403 TENANT_ACCESS_DENIED`；
   - 页面或动作权限不足：`403 DASHBOARD_PERMISSION_DENIED`；
   - 企业资料未配置：维持既有 `CORP_CONFIGURATION_REQUIRED` 合同；
   - 外部 Provider 未开通：显示明确的能力未开通状态，不能伪装成登录失效。
4. 对当前 `web/apps/dashboard/src/benchmark/manifest.json` 中全部 53 页逐页进行浏览器简单点击检查，修复发现的同类认证、导航、运行时和请求合同问题。
5. 完成本地现有 Docker 服务验证后，只重建应用容器；最终部署到 `139.196.34.133`，服务器不编译，并再次执行 53 页生产验收。

## 3. 非目标与安全边界

- 不新增第二套认证服务、兼容 session 或 Redis 身份缓存。
- 不通过给普通用户扩大权限来消除 403。
- 不把外部企业微信、会话存档或 AI Provider 未开通误报为本次代码故障。
- 逐页点击默认只执行无副作用操作：导航、页签、查询、刷新、分页、打开/关闭详情、展开筛选。写操作只使用专用测试数据且必须经过确认，不触碰真实业务数据。
- 不新建 Docker 项目或平行服务；不删除或重建 MySQL、Redis 和四个数据卷。
- 主工作区现有未提交文件属于用户，禁止 `reset`、`clean` 或覆盖；开发在独立 worktree 完成。

## 4. 推荐架构

### 4.1 单一认证事实

所有 `/dashboard/*` 生产请求按下列顺序处理：

1. `DashboardRequestGuard` 校验 JWT、服务端 session、用户和企业身份；
2. `DashboardAccessGuard` 校验租户门槛、页面资源和数据范围；
3. 业务 handler 只读取请求上下文，不再次解析 token，也不再次查询 `UserCorpCache`；
4. handler 使用上下文中的 `UserID`、`TenantID`、`CorpID`、`WorkEmployeeID` 和 `AllowedEmployeeIDs`，请求参数中的 `tenantId/corpId/userId` 不能覆盖认证身份；
5. 上下文身份字段不一致或缺失时失败关闭，返回真实 `401 UNAUTHORIZED`；权限不足仍由统一 guard 返回 `403 DASHBOARD_PERMISSION_DENIED`。

新增一个内部共享解析器，例如：

```go
type DashboardHandlerIdentity struct {
    UserID         int
    TenantID       int
    CorpID         int
    WorkEmployeeID int
}

func ResolveDashboardHandlerIdentity(ctx context.Context) (DashboardHandlerIdentity, error)
```

该解析器同时读取 `DashboardPrincipal` 与 `DashboardAccessContext` 并校验 `UserID/TenantID/CorpID` 一致。风险行为、超时预警、消息拦截等旧 handler 复用该解析器，删除对 `LoginCache` 和请求 `corpId` 的身份依赖。构造函数可先保留兼容参数以缩小组合根变更，但生产处理路径不得再访问旧缓存；若清理构造参数不会扩大风险，则同步删除。

### 4.2 旧认证依赖清单门禁

新增源码门禁，扫描 Dashboard 的生产 handler 和 `cmd/mochat-go` 组合根：

- 页面 API handler 不得调用 `UserCorpCache`；
- 页面 API handler 不得直接从 Authorization header 重建 Dashboard 身份；
- 登录、激活、MFA、登出等明确认证端点可以列入精确豁免；
- 每个豁免必须绑定 method + route + handler symbol，禁止按文件或路径前缀放行。

门禁从真实路由注册和 handler symbol 提取事实，不能用注释、测试字符串或 catalog 自证。

### 4.3 前端真实 401 的统一收口

后端修复错误 401 后，前端仍保留真实 session 失效的统一处理：

- `ApiClient` 收到真实 `401` 时只执行一次 session 清理、QueryClient 清理和 `/login?returnTo=<current>` 导航；
- `403 DASHBOARD_PERMISSION_DENIED` 保留 session，展示 403 页面或局部无权限状态；
- `403 TENANT_ACCESS_DENIED` 清 session 并返回登录页；
- 并发请求同时返回 401 时处理必须幂等，不能形成重复导航或无限重载；
- 登录、MFA 客户端不复用该自动导航处理。

前端收口是防御措施，不能替代后端消除错误 401。

## 5. 53 页逐页验收合同

页面清单只取自 `web/apps/dashboard/src/benchmark/manifest.json`，当前基线为：

- 总页面：53；
- 普通页面：48；
- `superadmin_only`：5。

验收工具必须遍历 manifest，而不是手写另一份 53 页数组。每页至少执行：

1. 通过真实左侧菜单或直接路由进入页面；
2. 断言当前 URL、页面标题或 page shell 正确；
3. 断言页面无 React 运行时异常、白屏、无限 loading；
4. 点击页面存在的首个安全交互：页签、查询/刷新、展开筛选、分页、详情打开/关闭之一；
5. 收集本页发出的 Dashboard API 响应，已授权超级管理员不得出现意外 401/403/404/5xx；
6. 如果外部能力未开通，必须呈现明确的 Provider/配置缺失状态，且不得清除 session；
7. 点击后再返回“数据概览”，确认登录态、企业名、用户名和导航仍存在；
8. 记录 console error、pageerror、失败请求和截图证据。

额外矩阵：

- 普通用户访问未授权页面：稳定 403，session 保留；
- 真实失效 session：稳定 401，并一次性回登录页；
- 桌面与 390px：菜单和内容独立滚动，关键控件可见；
- 53 页完成后菜单集合仍与 access profile 精确一致，Dashboard 不出现 SaaS 管理链接。

验收结果以结构化 JSON/Markdown 记录每页 route、点击动作、请求状态、console/page error 和截图路径；任一页缺记录均失败。

## 6. TDD 与验证策略

### 6.1 后端 RED

- 给风险行为、超时预警、消息拦截构造已经含 `DashboardPrincipal + DashboardAccessContext` 的请求，同时注入会 panic/报错的旧 `LoginCache`；当前实现应 RED，修复后 handler 返回业务结果且证明未读旧缓存。
- 缺 principal、principal/access 身份不一致分别返回 `401 UNAUTHORIZED`。
- 页面权限不足由统一 access guard 返回 `403 DASHBOARD_PERMISSION_DENIED`，handler 不自行把它转换成 401。
- 源码门禁针对临时 fixture：新增一个页面 handler 的 `UserCorpCache` 调用必须失败；注释或测试字符串不能误报生产使用。

### 6.2 前端 RED

- 已授权页面 API 返回 403 时不清 session、不跳登录；
- 真实 401 清 session、清查询缓存并导航一次；
- 多个并发 401 只触发一次退出流程；
- 风险页面的 Provider 未开通或业务错误不触发全局退出。

### 6.3 全量门禁

- `go test ./...`；
- Dashboard `typecheck`、`lint`、完整测试、`build`；
- API client/auth 包测试；
- Dashboard catalog/completion gate；
- E2E typecheck/lint/test；
- 53 页本地真实浏览器点击矩阵；
- `git diff --check` 和工作树状态复核。

## 7. 本地 Docker 与服务器部署

### 7.1 本地

1. 记录应用、MySQL、Redis 容器 ID 和四卷名称/挂载点；
2. 在宿主机编译；
3. 只重建现有项目的 app 容器，不重建 MySQL/Redis；
4. 运行健康检查、登录和 53 页点击矩阵；
5. 对比容器/卷和关键表，确认数据保留。

### 7.2 服务器

1. 上传宿主机已编译的二进制和前端静态资源，服务器不得编译；
2. 记录 `/opt/mochat-go` 当前制品校验值、容器 ID、四卷和关键数据计数；
3. 备份被替换制品，原子替换；
4. 只重建 `standalone-app-1`，不得重建 `standalone-mysql-1`、`standalone-redis-1`；
5. 健康检查通过后，用真实账号执行 53 页逐页验收；
6. 任一硬门禁失败立即回滚 app 制品，不回滚数据库、不删除卷；
7. 最终证据不得包含密码、JWT、Secret 或完整 Authorization header。

## 8. 完成标准

只有同时满足以下条件才能宣称完成：

- 生产页面 handler 不再依赖旧 Redis 登录缓存；
- 风险行为原始复现路径不再出现错误 401；
- 53/53 页面均有真实点击和请求证据；
- 授权页无意外 401/403，未授权页 403 不清 session，真实 session 失效才 401；
- 本地和服务器均通过健康、登录、页面和数据保留检查；
- 服务器 MySQL、Redis 和四卷未删除、未重建；
- 所有新增代码、门禁、文档与证据经过主任务独立审阅。
