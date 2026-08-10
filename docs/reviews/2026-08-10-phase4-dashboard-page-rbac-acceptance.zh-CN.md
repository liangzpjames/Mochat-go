# Phase 4 Dashboard 页面 RBAC 最终验收记录

日期：2026-08-10  
执行分支：`phase4/dashboard-page-rbac`  
主线基线：`main@475f9255b453d005ec5193d064fdd60bcedf6d65`  
验证环境：本地 Docker Compose 项目 `mochat-go-desktop`

## 1. 验收结论

Phase 4 Dashboard 页面 RBAC 已完成设计、实现、迁移、自动化门禁、真实 Docker API、真实浏览器和 SaaS 状态验收，满足已确认的产品边界：

- SaaS 只控制租户启停、套餐、额度和订阅整体门槛，不参与 Dashboard 页面显示授权；
- Dashboard 超级管理员在本企业内管理用户的直接权限和多角色权限；普通用户有效权限为“直接权限 ∪ 所有启用角色权限”；
- 角色停用后立即停止贡献权限，用户直接权限保留；数据范围按 `tenant > department > self` 合并；
- 53 个 Dashboard 页面中，49 个可授予普通用户，4 个管理页永久 `superadmin_only`；
- 导航、搜索、深链和 Dashboard API 使用同一权限事实；无权限用户不显示菜单，深链显示“无权访问”，API 返回 `403 + DASHBOARD_PERMISSION_DENIED`；
- Dashboard 不显示、不搜索、不跳转 `/saas-admin/`；
- Dashboard 超级管理员账号只有在 SaaS 租户、套餐、额度快照和订阅均有效时才可登录和访问；门槛失败返回 `403 + TENANT_ACCESS_DENIED`；
- `tenant_id` 只来自认证身份，跨企业用户或角色目标按不存在处理并返回 `404`，不能读取或修改其他企业；
- 真实验证全程未执行 `down -v`、`volume rm`、`volume prune`、`system prune`，未删除、清理或重建任何数据卷。

## 2. 实现范围

### 2.1 数据与服务端

- 迁移 `0127_dashboard_page_rbac` 新增六张 `mochat_go_dashboard_*` 表，并给 `mc_user`、`mc_rbac_role` 增加集合乐观锁版本；
- 权限目录固定 seed 53 页，包含 49 个 grantable 页面和 4 个 `superadmin_only` 页面；
- 238 条 `HTTP method + path pattern` API 资源映射，其中 97 条要求数据范围；普通用户访问未登记 Dashboard API 默认拒绝；
- 统一解析 SaaS 门槛、直接授权、所有启用角色、来源和最终数据范围；
- 用户授权和角色权限写入使用事务、`expectedVersion` 乐观锁及 append-only 审计；
- 管理目标查询和写入始终带认证租户，跨租户目标返回 `404`；
- 角色和用户授权 API 支持分页、完整详情、角色启停、版本冲突 `409`、审计回读；
- 旧 Dashboard 业务 handler 已接入 `DashboardAccessContext`。对无法安全证明员工归属的受限范围写操作采取失败关闭，不退回旧首角色 `DataPermission` 或全企业放行。

### 2.2 Dashboard 前端

- React route loader 改为读取 `/dashboard/access/profile`，删除 manifest 全量放行；manifest 只保留页面结构和顺序；
- 菜单、搜索、深链和 API 使用同一 `allowedRoutes/effectivePermissions`；
- 新增用户权限、多角色、角色权限、附加权限、授权审计和版本冲突界面；
- 四个永久超管页面：
  - `/company-setting/staff`
  - `/setting/role`
  - `/setting/additional`
  - `/setting/authorization`
- 390px 窄屏权限界面无页面级横向溢出；
- Dashboard DOM 中没有 SaaS 入口或 SaaS URL。

## 3. SaaS 门槛实证

租户 1（Dashboard 超级管理员所属租户）通过真实 SaaS 管理 API 和 SaaS 页面完成开通、续期、套餐定义修复及租户套餐快照同步，最终只读回读为：

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

另创建租户 52 作为门槛拒绝夹具，并通过真实 SaaS `tenantStatus` 接口停用。该租户管理员密码正确时，Dashboard 登录仍返回 `403 + TENANT_ACCESS_DENIED`，不签发会话。

## 4. 真实浏览器验收

Playwright 使用 `http://127.0.0.1:18080`，未拦截产品请求。一个真实普通账号通过 Dashboard 超管管理 API 依次切换以下状态：

1. 49 个普通页面直接授权：49 页可见、4 个永久管理页拒绝，菜单精确等于 49 条；
2. 单个直接权限：只显示对应页面，来源为 `direct`；
3. 两个启用角色：同一权限同时显示两个角色来源，结果为并集；
4. 一个禁用角色加保留的直接权限：禁用角色不贡献，直接权限继续生效；
5. 零权限：侧栏不存在，53 个深链逐页显示“无权访问”，未注册 Dashboard API 返回 `403 + DASHBOARD_PERMISSION_DENIED`；
6. 超级管理员：53 页全部可见，包含 4 个永久管理页；
7. 390px：普通页面正常渲染且无页面级横向溢出；
8. 所有状态均检查 Dashboard 没有 SaaS 链接；
9. 租户 52 停用管理员登录返回 `TENANT_ACCESS_DENIED`，本地不保留 token。

执行结果：live 用例 `1 passed`，耗时约 1.3 分钟；mock 合同用例 `5 passed / 1 live skipped`。真实普通账号最终已恢复为零直接权限、零角色关联，避免残留过度授权。

## 5. 跨企业和并发安全实证

- 超管读取其他租户的用户 ID 10：`404`；
- 跨租户请求不产生关系行或审计之外的业务变更；
- 授权写入后使用旧 `expectedVersion` 重放：`409`；
- 成功变更只增加 1 条审计，其余被监控表计数不变；
- MariaDB 集成测试：`TestDashboardAccessIntegration`、`TestDashboardAccessScopeIntegration` 均通过，临时 schema 残留为 0；
- 数据范围集成覆盖 `tenant`、`department`、`self`。

## 6. 自动化与构建结果

| 门禁 | 结果 |
| --- | --- |
| `go test ./... -count=1` | PASS |
| `pnpm check:phase4-dashboard-page-rbac` | PASS，`53/49/4`，未映射前端 Dashboard API 为 0，`scopeRequired=97` |
| Dashboard `typecheck` | PASS |
| Dashboard `lint` | PASS |
| Dashboard production `build` | PASS |
| Dashboard Vitest 全量 | PASS，92 files / 565 tests |
| E2E `typecheck`、`lint` | PASS |
| Playwright mock | PASS，5 passed / 1 live skipped |
| Playwright live | PASS，1 passed / 5 mock skipped |
| smoke/fixture/completion Node tests | PASS，13/13 |
| `git diff --check` | PASS |

完整 smoke 合同：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\smoke\smoke-contract.json`  
表计数变化：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\smoke\table-count-delta.json`

## 7. Docker、迁移和数据卷

最终容器均为 healthy：

| 服务 | 最终容器 ID | 与验证前比较 |
| --- | --- | --- |
| app | `16d7a600e463c9f83d3c6dc4b2fd07e8bf6216bda1eb911c539a4ddc02c23e9d` | 为部署 Phase 4 仅重建 app |
| mysql | `9d8498d7c1ea867ebbeaebf584407037091ec8297d958c2e35c918d63a2aaae7` | 未变化 |
| redis | `791f65768a4b70bc130290c2f71ea06cc1b8c563b7cababab3a42c58d52af077` | 未变化 |

四个具名卷验证前后均存在且名称一致：

- `mochat-go-desktop_app-storage`
- `mochat-go-desktop_audit-anchor-storage`
- `mochat-go-desktop_mysql-data`
- `mochat-go-desktop_redis-data`

迁移 ledger 回读：

- version：`0127_dashboard_page_rbac`
- checksum：`90a77eb8735ab025dfe80f581ff62b3a79c16e2b9840e235d77c0649fb8e3a7f`
- applied_at：`2026-08-10 17:25:56`
- catalog：53；protected：4；resources：238；scope required：97。

Docker 最终状态：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\final-docker-state.json`  
数据库回读：`D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810\rbac-database-readback.json`

## 8. 安全边界说明

受限范围普通用户在部分旧业务写接口上，如果现有数据模型不能安全证明目标员工/客户属于 `self` 或 `department`，实现选择返回拒绝，不提升到 tenant 范围。当前明确失败关闭的动作包括超时审计/分配、风险审计、沉默客户动作、消息拦截审计、朋友圈 `Publish/ExportData` 和 SCRM 标签联系人写入。超级管理员或具有 `tenant` 范围的授权仍可按业务规则使用。该边界避免以功能可用性为由造成跨员工或跨部门越权。

## 9. 工作区和夹具保留

- Phase 4 全部提交只在 `phase4/dashboard-page-rbac` 独立分支；未合并、未改写主线；
- 主线仍为 `475f9255b453d005ec5193d064fdd60bcedf6d65`，原有 dirty 文件和未跟踪文件保持原状；
- 验收创建的用户、角色、审计、租户 52 和 SaaS 变更按要求保留，未执行清理；
- 真实普通账号最终为零权限，停用租户夹具继续用于用户复验；
- 验收材料位于 `D:\workspace\mochat-go\output\phase4-dashboard-rbac-acceptance-20260810`。
