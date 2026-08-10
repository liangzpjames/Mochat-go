# Phase 4 Dashboard 页面 RBAC 最终验收记录

日期：2026-08-10
执行分支：`phase4/dashboard-page-rbac`
代码验收基线：`c5b1ea7f5ea0f6b21ada7da9b2178fdfbac49dd0`
主线基线：`main@475f9255b453d005ec5193d064fdd60bcedf6d65`
唯一运行验收环境：本地 Docker Compose 项目 `mochat-go-desktop`

## 1. 最终结论

Phase 4 已完成设计、实现、兼容迁移、自动化门禁、真实 Docker API、真实浏览器和 SaaS 状态验收，并通过第二轮独立代码审阅。最终审阅结果为：Critical 0、Important 0、Minor 0。

- SaaS 只控制租户启停、套餐、额度、订阅和账号整体门禁，不参与 Dashboard 页面授权。
- Dashboard 超级管理员只能在本企业内管理用户直接权限和多角色权限。
- 普通用户有效权限为“直接权限 ∪ 所有已启用角色权限”；禁用角色不贡献权限。
- 数据范围按 `tenant > department > self` 合并。
- 53 个 Dashboard 页面中，49 个可授予普通用户，4 个永久 `superadmin_only`。
- 无权限用户看不到菜单和搜索入口；深链显示“无权访问”；未授权 API 返回 `403 + DASHBOARD_PERMISSION_DENIED`。
- Dashboard 不显示、不搜索、不跳转 SaaS 管理界面。
- Dashboard 超级管理员账号只有在 SaaS 租户、套餐、额度快照和订阅均有效时才可登录和访问；门禁失败返回 `403 + TENANT_ACCESS_DENIED`。
- `tenant_id` 只来自认证身份；跨企业用户或角色目标按不存在处理并返回 `404`，不读取、不修改其他企业。
- 验收全过程未执行 `down -v`、`volume rm`、`volume prune` 或 `system prune`，未删除或重建任何数据卷。

## 2. 实现范围

### 2.1 服务端与数据库

- `0127_dashboard_page_rbac` 建立 53 页权限目录、API 资源映射、用户直接权限、多角色关系、数据范围和追加式审计。
- `0128_dashboard_page_rbac_legacy_scope_fix` 在不修改已应用 0127 文件和 checksum 的前提下修正旧角色数据范围：
  - 旧菜单 `data_permission=2` 映射为 `tenant`；
  - 旧受控菜单 `permissionType=1` 映射为 `department`；
  - 旧受控菜单 `permissionType=2` 映射为 `self`；
  - 同一角色存在混合旧范围时采用更严格的 `self`，并写管理员复核审计；
  - 只更新有旧 `role_menu` 映射的授权，新建角色不受影响。
- 目录固定为 53 页、4 个保护页、238 条 API 资源，其中 97 条要求数据范围。
- 授权写入使用事务、`expectedVersion` 乐观锁和 append-only 审计。
- 用户与角色管理目标始终绑定认证租户；跨租户目标返回 `404`。
- 敏感词监控详情使用 `corp_id + trigger_user_id` 过滤；受限范围为空或员工不匹配时统一返回 `404`。
- 旧业务 handler 从 `DashboardAccessContext` 获取新权限事实；无法安全证明数据归属时失败关闭，不退回全企业放行。

### 2.2 Dashboard 前端

- route loader 读取 `/dashboard/access/profile`，不再以 manifest 全量放行。
- 菜单、搜索、深链和 API 使用同一 `allowedRoutes/effectivePermissions` 权限事实。
- 空 `allowedActions` 表示当前页面没有旧的逐动作契约，因此页面授权后保留该页业务操作；非空旧动作集仍按动作限制。
- 新增用户权限、多角色、角色权限、附加权限、授权审计和版本冲突界面。
- 四个永久超管页面：
  - `/company-setting/staff`
  - `/setting/role`
  - `/setting/additional`
  - `/setting/authorization`
- 390px 窄屏无页面级横向溢出。
- Dashboard DOM 中没有 SaaS 入口或 `/saas-admin/` 链接。

## 3. SaaS 门禁实证

租户 1 通过真实 SaaS 管理 API 和页面完成开通、续期、套餐修复和额度快照同步，最终只读回读如下：

| 项目 | 最终值 |
| --- | --- |
| tenant | `1`，启用 |
| package | `standard / 标准版`，启用 |
| 套餐定义版本 | `v3` |
| 租户套餐快照版本 | `v3` |
| 到期时间 | `2027-08-10 23:59:59` |
| 子账号 | `7 / 100` |
| 企业 | `2 / 10` |

SaaS 页面截图：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\saas\tenant-1-standard-active.png`
SaaS API 回读：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\saas\tenant-1-readback.json`

租户 52 作为门禁拒绝夹具并保持停用。其管理员密码正确时，Dashboard 登录仍返回 `403 + TENANT_ACCESS_DENIED`，且不签发会话。

## 4. 真实浏览器与 API 验收

Playwright 使用 `http://127.0.0.1:18080`，未拦截产品请求。真实用例在超级管理员 API 驱动下依次验证：

1. 普通用户获得 49 个可授予页面时，49 页可见，4 个保护页拒绝，菜单精确等于 49 条。
2. 单个直接权限只显示对应页面，来源为 `direct`。
3. 两个启用角色的权限按并集生效，同一权限保留两个角色来源。
4. 禁用角色不贡献权限，用户直接权限继续生效。
5. 零权限时不显示侧栏，53 个深链均显示“无权访问”，未登记 API 返回 `DASHBOARD_PERMISSION_DENIED`。
6. 超级管理员 53 页全部可见，包括 4 个永久管理页。
7. 390px 页面正常渲染且无页面级横向溢出。
8. 所有状态均确认 Dashboard 没有 SaaS 链接。
9. 租户 52 登录返回 `TENANT_ACCESS_DENIED`，浏览器不保留 token。

最终 live 执行结果：`1 passed / 5 mock skipped`，耗时约 1.2 分钟。普通夹具用户最终恢复为零直接权限、零角色关系。

真实服务补充安全实证：

| 检查 | 结果 |
| --- | --- |
| 授予用户 5 敏感词页面 `self` 权限 | `200` |
| 绑定租户 1 的企业 `1536612155` | `200` |
| 读取本人触发记录 | `200` |
| 读取同企业其他员工触发记录 | `404` |
| 租户 1 超管读取租户 52 用户 ID 10 | `404` |
| 恢复用户 5 零权限 | `200` |

保留两条敏感词范围验收记录用于复验；误建在其他租户企业下的临时员工和记录已软删除，当前无有效错误归属记录。

## 5. 自动化门禁

| 门禁 | 结果 |
| --- | --- |
| `go test ./... -count=1` | PASS |
| MariaDB `TestDashboardPageRBACIntegration` | PASS，使用 `mochat-go-desktop` MySQL |
| MariaDB `TestSensitiveWordsMonitorMessagesEnforcesEmployeeScope` | PASS，使用 `mochat-go-desktop` MySQL |
| `pnpm check:phase4-dashboard-page-rbac` | PASS：`53/49/4`、未映射 API 0、`scopeRequired=97` |
| Dashboard `typecheck` | PASS |
| Dashboard `lint` | PASS |
| Dashboard production `build` | PASS |
| Dashboard Vitest 全量 | PASS：92 files / 565 tests |
| E2E `typecheck`、`lint` | PASS |
| Playwright mock | PASS：5 passed / 1 live skipped |
| Playwright live | PASS：1 passed / 5 mock skipped |
| smoke/fixture/completion Node tests | PASS |
| `git diff --check` | PASS |

完整 smoke 合同：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\smoke\smoke-contract.json`
表计数变化：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\smoke\table-count-delta.json`

## 6. Docker、迁移和数据卷

最终容器均为 healthy：

| 服务 | 最终容器 ID | 结果 |
| --- | --- | --- |
| app | `ea43f10a634f83d9798449f6293ef191f3ff83314ee86ca776b76865c354d8a0` | 仅 app 被替换为 Phase 4 镜像 |
| mysql | `9d8498d7c1ea867ebbeaebf584407037091ec8297d958c2e35c918d63a2aaae7` | 与验收前一致 |
| redis | `791f65768a4b70bc130290c2f71ea06cc1b8c563b7cababab3a42c58d52af077` | 与验收前一致 |

四个具名卷验收前后均存在且名称一致：

- `mochat-go-desktop_app-storage`
- `mochat-go-desktop_audit-anchor-storage`
- `mochat-go-desktop_mysql-data`
- `mochat-go-desktop_redis-data`

迁移 ledger 回读：

| version | checksum | applied_at |
| --- | --- | --- |
| `0127_dashboard_page_rbac` | `90a77eb8735ab025dfe80f581ff62b3a79c16e2b9840e235d77c0649fb8e3a7f` | `2026-08-10 17:25:56` |
| `0128_dashboard_page_rbac_legacy_scope_fix` | `b17e557aa49f0ea8295bf85507d93851b4e2ea064dce4ccbee5e5ce35588336e` | `2026-08-10 19:01:30` |

角色 5、6、7 均无旧 `role_menu` 映射，因此 0128 未覆盖这些新角色权限；角色 5/6 保持启用，角色 7 保持停用。当前旧范围混合复核审计为 0。

Docker 最终状态：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\final-docker-state.json`
数据库回读：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\rbac-database-readback.json`

## 7. 夹具与工作区边界

- 原保留夹具的三个账号密码哈希与当前运行时密钥发生漂移；验收仅按既定夹具密码重新生成这三条哈希，没有修改账号状态、租户、套餐、角色或业务权限。
- 用户 5 最终为零直接权限、零角色关系；租户 52 继续停用；角色与审计夹具保留供用户复验。
- Phase 4 全部提交只存在于 `phase4/dashboard-page-rbac` 独立分支，未合并或改写主线。
- 主线仍为 `475f9255b453d005ec5193d064fdd60bcedf6d65`，原有 dirty 和未跟踪文件保持原状。
- 验收材料位于 `D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810`。
