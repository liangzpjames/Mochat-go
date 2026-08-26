# 任务 3 实施报告：新租户激活预检与安全入口

## 结果

已完成激活状态预检、精确公开路由、SaaS 开户/重发一次性交付字段、审批持久化脱敏，以及 Dashboard 激活页的 fragment/query 同步清理与全状态 UI。

## RED / GREEN

- 初始 RED：Go 定向测试因缺少 `DashboardActivationStatus`、状态 store 方法、`ActivationPath` 和 `ActivationExpiresAt` 失败；前端测试因缺少 `inspectDashboardActivation` 失败。
- 路由 RED：组合后的 `PublicDashboardRouteContracts` 未包含新状态路由，精确公开路由测试返回 401。
- UI RED：密码不一致后表单被错误隐藏，回归测试无法再次找到密码输入和激活按钮。
- 安全审查 RED：审批 `result_json` 会通过 `activationPath` 持久化 raw token；真实 `ApiError.machineCode` 未被前端保留。
- 最终 GREEN：上述测试均通过；Dashboard 全量测试 142 个文件、926 个用例通过，Go 定向套件、Server 路由套件和 Dashboard typecheck 通过。

## 状态映射

| 条件 | status | primaryAction |
| --- | --- | --- |
| digest 不存在 | `invalid` | `contact_admin` |
| 未消费、未过期且 identity/user/tenant 均有效 | `valid` | `activate` |
| 未消费但已过期 | `expired` | `contact_admin` |
| 已消费且 `activated_at` 非空 | `activated` | `login` |
| 已消费但 `activated_at` 为空 | `revoked` | `contact_admin` |
| identity、user 或 tenant 不可用 | `revoked` | `contact_admin` |

所有已命中 digest 的响应只包含租户安全展示名、掩码账号、到期时间和动作；不返回 token、digest、密码或 MFA secret。业务状态统一 HTTP 200 和稳定码，解码错误返回 400。

## 一次性交付与安全清理

- 新入口仅生成 `/activate#token=<encoded>`，不读取 Host，不生成 `?token=`。
- 开户/重发使用同一个 `activationExpiresAt` 值写入数据库并返回；幂等重放清空 token、path 和 expiresAt。
- 审批首次执行响应可一次性交付 token/path，但持久化 `result_json` 同时剔除 `activationToken` 和 `activationPath`，仅保留交付标记；审计 payload 只记录 `activationIssued`。
- Dashboard 同步读取 hash，并兼容旧 `token`/`activationToken` query，随后立即 `replaceState({}, '', '/activate')`。
- token 仅存在组件内存；未调用 localStorage/sessionStorage。预检和激活错误均使用去敏文案，预检错误保留稳定 `errorCode`。
- 激活成功后立即清空 token、密码和确认密码，显示成功状态及“前往登录”，不自动携带 token 跳转。

## UI 与可访问性

- 覆盖加载、valid、expired、activated、revoked、invalid、预检失败重试、密码不一致和激活成功。
- 加载状态使用 `aria-live="polite"`，错误使用 `role="alert"`，CTA 使用原生可聚焦按钮。
- 390px 下卡片使用 `width: 100%` 与 `min-width: 0`，沿用移动端单列和内边距规则，避免横向溢出。

## 自审

- 只读代码审查最初发现审批 path 持久化和真实错误码字段两个问题；均已增加失败测试并修复，复核后无 Critical/Important 问题。
- 已检查新增代码不存在 token query 生成、storage 写入、日志/错误回显或审计持久化。
- 保留工作树中原有 `.superpowers/sdd/progress.md` 修改，未纳入本任务提交。

## Concerns

- brief 指定的 pnpm `test -- <files>` 脚本实际运行 Dashboard 全量测试；过程中现有 jsdom 对伪元素 `getComputedStyle` 的非失败 stderr 仍会出现，但最终 926/926 用例通过。
- 未进行真实浏览器像素级截图验收；390px 由 DOM、交互和 CSS 契约测试覆盖。
