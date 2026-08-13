# 两个移动端基础框架设计

## 1. 背景与结论

仓库中的两个移动端已经有明确产品边界：

- `web/apps/sidebar`：企业微信聊天侧边栏，供企业员工在会话中查看客户资料、SOP、素材和待办。
- `web/apps/operation`：微信营销活动 H5，供外部联系人或活动参与者完成任务宝、裂变、抽奖、群打卡、门店活码等流程。

当前两个 React 应用虽然接管了旧 URL，但核心实现仍是通用占位页，存在中文乱码、直接调用 `fetch`、认证模型不准确、缺少统一错误状态和路由边界等问题。本阶段不把占位页包装成“业务已完成”，而是建立后续可持续迁移的生产级基础框架，并交付各自一个可验证的入口纵切面。

## 2. 方案选择

采用“共享移动端基础包 + 两个独立业务应用”的方案。

未采用的方案：

- 两个应用各复制一套壳：短期快，但认证、错误处理、窄屏适配会持续漂移。
- 改成 React Native、跨端容器或 PWA：会扩大部署和企微 SDK 风险，本阶段没有必要。
- 合并为一个应用：Sidebar 与 Operation 的身份、受众和 URL 生命周期不同，合并会造成错误的会话复用。

## 3. 本阶段范围

### 3.1 共享基础

新增 `@mochat/mobile-foundation`，只承载与业务无关的能力：

- 移动端页面壳、顶部栏、内容区、底部安全操作区和安全区适配。
- 加载、空数据、错误、无权限、未找到和离线/网络失败状态。
- 统一 API 客户端：相对路径、`credentials: same-origin`、可选 Bearer token、JSON envelope、超时/取消、稳定错误类型和 `requestId`。
- 查询参数读取、返回地址校验和站内路径拼接，拒绝开放重定向。
- 应用级错误边界和可重试入口；日志不输出 token、OAuth code、secret 或完整敏感参数。
- 390px 至桌面预览宽度的响应式基础变量；44px 最小触控区域、safe-area 和文档级无横向溢出。

共享包不包含客户、活动、素材等业务组件，也不保存任何应用会话。

### 3.2 Sidebar

- 保留 manifest 中 12 条历史 URL、query、hash 和 `/sidebar-app` 挂载方式。
- `migration-routes.json` 是路由清单，路由注册代码必须消费该清单并校验每条路由有明确模块。
- 认证继续使用 Sidebar 员工身份：`token` 与 `agentId` cookie，API 前缀固定 `/sidebar`。
- 受保护路由无有效 token 时跳转 `/login?agentId=...&target=...`；`target` 仅允许同源站内路径。
- `/login` 生成真实 `/sidebar/agent/auth` OAuth 地址；`/auth` 解析后端回传状态，成功后写入受限 cookie 并回到目标页，失败显示可重试状态。
- `/contact` 作为第一个业务纵切面：通过领域 API 读取当前客户摘要，覆盖加载、成功、空数据、401、业务错误和重试；其他路由进入明确的“模块待迁移”状态，不提供假按钮或假成功数据。

### 3.3 Operation

- 保留 manifest 中 10 条历史 URL、query、hash 和 `/operation-app` 挂载方式。
- Operation 不读取 Dashboard 或 Sidebar token；活动身份继续由后端 OAuth 与同源 cookie 会话负责，API 前缀固定 `/operation`。
- 路由定义明确活动类型、必需参数和是否需要活动会话；缺少必需参数时展示可恢复的参数错误，不发送无效请求。
- `/workFission` 作为第一个业务纵切面：读取活动参与者与任务进度，覆盖加载、成功、空数据、未授权、活动失效和重试。
- 其余活动路由进入明确的模块状态，不生成随机奖品、虚假进度或伪造写操作。

## 4. 文件与依赖边界

```text
web/packages/mobile-foundation/
  src/api/                 # 请求、envelope 与错误
  src/navigation/          # 安全 target 与路径工具
  src/shell/               # 页面壳、状态和错误边界
  src/styles/              # tokens 与基础样式

web/apps/sidebar/src/
  app/                     # 应用组合、路由注册、Provider
  auth/                    # Sidebar cookie 与 OAuth
  features/contact/        # 首个客户纵切面
  routes/                  # manifest 到模块的显式映射

web/apps/operation/src/
  app/                     # 应用组合、路由注册、Provider
  auth/                    # Operation 活动会话边界
  features/work-fission/   # 首个任务宝纵切面
  routes/                  # manifest 到模块的显式映射
```

两端可以依赖 `@mochat/mobile-foundation`、`@tanstack/react-query` 和现有配置包，不互相依赖页面组件。业务响应在 feature API 边界转换，React 页面不直接依赖旧接口的偶然字段。

## 5. 数据流与认证

Sidebar：

```text
历史 URL -> 路由合同 -> SidebarAuthBoundary -> Query hook
-> /sidebar/* -> Go API -> 当前企业/员工数据
```

Operation：

```text
活动 URL -> 参数合同 -> OperationSessionBoundary -> Query hook
-> /operation/* -> Go API -> 活动会话与业务数据
```

两端的 `401` 都只清理各自会话或进入各自 OAuth，不得影响 Dashboard 登录。`403` 保留会话并显示无权状态；`404` 区分路由不存在和业务对象不存在；`409` 保留用户输入；网络/`5xx` 提供重试。

## 6. 测试与验收

按 TDD 实施，每个行为先出现可解释的 RED，再写最小实现。

- 共享包：API envelope、超时/网络错误、401/403、target 安全校验、错误边界和移动壳组件测试。
- Sidebar：12 条路由清单一致性、公开/受保护路由、OAuth 地址、回调成功/失败、客户纵切面和无假数据测试。
- Operation：10 条路由清单一致性、活动参数、会话边界、任务宝纵切面和无假结果测试。
- 静态门禁：两个应用均执行 lint、typecheck、unit test 和 production build；新增脚本检查乱码特征、直接 `fetch`、未登记路由和假业务文案。
- 浏览器验收：390×844 与 1280×900 打开全部 22 条 URL，断言页面不白屏、无文档级横向溢出、路由标题正确、未知路径显示 404、console error 为 0。

浏览器的真实 Go/OAuth/数据库业务闭环属于后续逐模块迁移验收；本阶段只宣称基础框架和两个入口纵切面完成。

## 7. 完成门禁

- 共享包职责不包含业务数据或会话持久化。
- Sidebar 12 条、Operation 10 条 URL 全部由显式注册表承接，未知路径不回落首页。
- 两种身份模型完全隔离，不读取 Dashboard 登录态。
- 中文源码、页面标题和错误文案无乱码。
- 页面代码不直接调用 `fetch`，不保留随机结果、固定假进度或“操作已完成”等伪成功。
- 390px 下无页面级横向溢出，关键触控元素不小于 44px。
- 两个应用 lint、typecheck、test、build 与移动端框架 completion gate 全部通过。
- 本阶段不改 Docker、Compose、生产数据库和服务器部署，不写 `output` 验收数据。
