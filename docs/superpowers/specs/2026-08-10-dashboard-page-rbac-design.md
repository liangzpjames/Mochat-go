# Phase 4 Dashboard 页面 RBAC 设计

状态：已由源任务确认全部产品边界，并授权执行任务自行审阅后持续实施。

日期：2026-08-10

## 1. 背景与目标

当前 Dashboard 的前端访问控制仍依赖旧 `mc_rbac_menu` 菜单树，且 `main.tsx` 把圆弧基准清单中的 53 条路由通过 `benchmarkRoutes` 全量加入 `allowedRoutes`。这会造成导航、深链和后端 API 三层权限事实不一致：页面可能因基准清单而被放行，API 则依赖零散的旧菜单 URL；未登记的新 API 也没有统一的普通用户默认拒绝策略。

Phase 4 建立一套独立于 SaaS 套餐功能项的企业内 Dashboard 页面权限体系，并使同一个有效权限事实同时驱动：

1. 53 个 Dashboard 页面逐页授权；
2. 左侧导航和搜索结果过滤；
3. 页面深链 `403`；
4. 与页面关联的 Dashboard API 授权；
5. 普通用户多角色、直接权限与数据范围合并；
6. 企业管理员对用户、角色、权限和审计的原子管理。

本设计不把 SaaS 套餐变成页面功能开关。SaaS 只负责租户、套餐、额度快照、订阅和企业整体可用性；Dashboard 页面目录及企业内授权完全由 Dashboard RBAC 管理。Dashboard 不展示或跳转 SaaS Admin。

## 2. 已确认的产品规则

### 2.1 SaaS 整体访问门槛

用户登录和每一次受保护的 Dashboard API 请求都重新校验租户整体访问门槛，任一条件不成立即失败关闭：

- `mc_tenant.status = 1` 且租户未删除；
- 存在唯一、启用、未删除的 `mochat_go_saas_tenant_packages` 记录；
- `starts_at` 为空或不晚于当前时间；
- `expires_at` 为空或晚于当前时间；
- `package_code` 非空，且 `limits_json` 是非 `null` 的 JSON object，能解码为当前额度快照结构；
- 存在订阅记录，按当前生命周期规则计算后允许访问。

订阅要求不会无意锁死合法历史租户：`0039_saas_subscription_lifecycle` 已把当时所有未删除的 tenant package 一一回填为 subscription，并为每个回填订阅写入 `migration:0039` 事件。0127 在建 RBAC 表前再次执行一致性检查：每个未删除、启用、有效期内的 tenant package 必须存在同 tenant 的未删除 subscription；发现缺失即用 `SIGNAL SQLSTATE '45000'` 中止迁移，要求先修复 SaaS 生命周期数据，不能静默跳过或在运行期才暴露。

门槛不沿用“缺套餐视为未过期”“缺订阅视为未托管，因此放行”的旧兼容语义。数据库查询失败、表缺失、JSON 无效或状态未知均不放行。订阅已经纳入生命周期时，以 `SaaSAdminEffectiveSubscriptionStatus` 和 `SaaSAdminSubscriptionAllowsAccess` 的现有规则为唯一状态判定，不另造第二套状态枚举。登录失败返回 `403`；已有会话在 API 门槛失效后也立即返回 `403`，前端清空会话并回到登录页。`/dashboard/user/auth`、MFA 登录完成接口在签发 token 前执行相同门槛。

### 2.2 页面权限

- 以 `web/apps/dashboard/src/benchmark/manifest.json` 的 53 个唯一 `path` 为权威页面清单。
- 49 个普通业务页可授予普通用户。
- `/company-setting/staff`、`/setting/role`、`/setting/additional`、`/setting/authorization` 永久标记为 `superadmin_only`，不能通过角色或直接授权下放。
- 普通用户有效页面权限为“直接权限 ∪ 所有启用角色的权限”。
- `isSuperAdmin = 1` 的用户在其认证租户内隐式拥有全部 53 页，不需要插入授权行；该规则不能跨租户。
- 停用角色即时停止贡献权限；用户直接权限保留。
- 页面授权同时控制导航、搜索结果和深链。移除 `benchmarkRoutes` 全量放行以及旧菜单树作为全局页面开关的职责。

### 2.3 API 权限

- `permission_resources` 显式登记 `HTTP method + 标准化 path pattern` 到一个页面权限。
- 同一 API 可登记到一个或多个页面权限；普通用户拥有其中任一有效页面权限即可访问。
- 管理接口 `/dashboard/access/catalog`、`/users*`、`/roles*`、`/audits*` 永久 `superadmin_only`。
- 普通用户请求未登记的受保护 Dashboard API 默认 `403`；superadmin 在同租户内可访问。
- 登录、登出、MFA、企业选择/绑定、`/dashboard/access/profile` 以及经明确审计的企微/公众号回调属于系统豁免资源，不参与页面映射；豁免表由代码常量维护并有完整测试，不能使用前缀式宽泛放行。
- `/operation-app`、`/sidebar-app`、`/saas-admin` 及其专用公开回调不属于 Dashboard 页面 RBAC。本阶段只保护 Dashboard 应用实际调用的 `/dashboard/*` API；系统豁免之外的 Dashboard API 对普通 Dashboard 会话默认拒绝。

### 2.4 数据范围

数据范围枚举沿用业务语义，并用稳定字符串传输：

- `self`：本人；
- `department`：本人所属部门范围；
- `tenant`：全企业。

直接权限默认 `self`。多个来源合并时取最大范围：`tenant > department > self`。角色停用后其范围即时移除；直接权限继续参与合并。解析出的有效范围通过请求 context 传给现有报表和业务 handler，逐步替代旧的单角色 `DataPermission` 推导。任何需要员工 ID 集合的数据查询都在认证租户和当前企业内求值，不能接受客户端提供 `tenant_id`。

## 3. 数据模型

迁移编号固定为 `0127_dashboard_page_rbac`，创建以下六张表。表名使用 `mochat_go_dashboard_` 前缀以保持与现有 Go 扩展表一致，领域名称仍分别为 `permissions`、`permission_resources`、`user_roles`、`role_permissions`、`user_permissions`、`permission_audits`。

0127 同时给两个权威聚合对象增加集合版本字段：

- `mc_user.dashboard_access_version bigint(20) unsigned NOT NULL DEFAULT 1`；
- `mc_rbac_role.dashboard_access_version bigint(20) unsigned NOT NULL DEFAULT 1`。

用户多角色和直接权限作为一个集合，以 `mc_user.dashboard_access_version` 为唯一乐观锁；角色元数据、状态和权限集作为一个集合，以 `mc_rbac_role.dashboard_access_version` 为唯一乐观锁。关系行不承担集合版本职责。

新表外键列必须逐字匹配现有父表类型：`tenant_id int(11)`（signed）、`user_id int(10) unsigned`、`role_id int(11)`（signed）、`permission_id bigint(20) unsigned`。迁移测试从当前 schema 定义核对这些类型，禁止为了表面统一擅自把 tenant 或 role 改成 unsigned，否则复合外键无法创建。

### 3.1 `mochat_go_dashboard_permissions`

全局、不可由租户改名的页面目录：

- `id bigint unsigned` 主键；
- `code varchar(96)` 唯一稳定代码；
- `path varchar(191)` 唯一页面路径；
- `name varchar(100)` 中文名称；
- `group_code varchar(64)`；
- `sort int`；
- `superadmin_only tinyint(1)`；
- `status tinyint`；
- `version bigint unsigned`；
- 标准时间戳及软删除字段。

迁移按 manifest 固定 seed 53 行。运行时不从数据库反向生成 React 路由；启动/测试门禁比较数据库 seed 清单与 manifest，发现数量、路径或 `superadmin_only` 漂移即失败。

### 3.2 `mochat_go_dashboard_permission_resources`

全局 API 映射：

- `id bigint unsigned`；
- `permission_id bigint unsigned` 外键到权限目录；
- `resource_type enum('api')`；
- `http_method varchar(10)`；
- `path_pattern varchar(191)`，只允许静态路径或命名参数段，如 `/dashboard/access/users/{id}`；
- `status tinyint`、`version`、时间戳和软删除；
- 唯一键 `(http_method, path_pattern, permission_id)`。

匹配器先做路径标准化，再按静态段优先、参数段其次匹配；不接受任意正则，避免宽泛映射误授权。

资源覆盖有硬门禁：构建脚本扫描 Dashboard React 应用实际发出的每一个 `/dashboard` 相对 API 调用，并与 server/module 注册的 Dashboard API 清单交叉核对。除精确系统豁免外，每个被 53 页使用的 `method + path pattern` 必须至少映射到一个页面权限；新增页面、修改 API 或新增 handler 时，如果 manifest、前端调用、server 注册和 catalog 任一侧漂移，测试直接失败。未映射资源不能通过旧菜单、前缀匹配或 fallback 放行。

### 3.3 `mochat_go_dashboard_user_roles`

租户内多角色关联：

- `tenant_id int(11)`；
- `user_id int(10) unsigned`；
- `role_id int(11)`，复用 `mc_rbac_role`；
- `created_at` 和 `updated_at`，不设软删除字段；
- 唯一键 `(tenant_id, user_id, role_id)`；
- 复合外键 `(tenant_id, user_id)` 与 `(tenant_id, role_id)`。

关联历史不依赖 nullable `deleted_at` 唯一键，因为 MariaDB 对唯一键中的多个 `NULL` 不提供“唯一活动行”语义。更新用户授权时，事务物理删除该用户当前关系行并按请求集合重新插入；变更前后快照由 append-only audit 永久保存。迁移先为 `mc_user(tenant_id,id)` 和 `mc_rbac_role(tenant_id,id)` 建立必要的复合唯一索引。回填旧 `mc_rbac_user_role` 前，必须验证 user 与 role 的 `tenant_id` 完全相同；发现跨租户脏关系时迁移直接失败，不跳过、不修猜测。

### 3.4 `mochat_go_dashboard_role_permissions`

- `tenant_id int(11)`、`role_id int(11)`、`permission_id bigint(20) unsigned`；
- `data_scope enum('self','department','tenant')`；
- 时间戳，不设软删除字段；
- 复合外键 `(tenant_id,role_id)` 和权限目录外键；
- 唯一键 `(tenant_id,role_id,permission_id)`。

更新角色权限时，事务物理删除该角色当前权限关系并重新插入，append-only audit 保存前后集合。旧 `mc_rbac_role_menu` 只用于一次兼容回填：把旧页面菜单 `link_url` 规范化后映射到 53 页；无法映射的旧条目保留在旧表但不成为新权限。四个 `superadmin_only` 页面不回填给普通角色。

### 3.5 `mochat_go_dashboard_user_permissions`

- `tenant_id int(11)`、`user_id int(10) unsigned`、`permission_id bigint(20) unsigned`；
- `effect enum('allow')`；本阶段不引入 deny，避免与并集规则冲突；
- `data_scope` 默认 `self`；
- 时间戳，不设软删除字段；
- 复合外键 `(tenant_id,user_id)` 和权限目录外键；
- 唯一键 `(tenant_id,user_id,permission_id)`。

更新直接权限时与 `user_roles` 在同一事务内物理替换；历史只存在 `permission_audits`，不存在可被误读为活动关系的软删除行。

### 3.6 `mochat_go_dashboard_permission_audits`

仅追加审计，记录：

- `tenant_id`、`actor_user_id`；
- `action`、`target_type`、`target_id`；
- `before_json`、`after_json`；
- `expected_version`、`result_version`；
- `request_id`、`created_at`。

审计不允许更新或软删除。授权写入和对应审计必须使用同一个 `sql.Tx`；任何一侧失败都回滚。

## 4. 认证、授权与租户隔离

### 4.1 认证身份是 tenant 唯一来源

JWT 仍只承载 `uid`。服务端通过 `uid` 查询 `mc_user` 得到 `tenant_id`，之后所有 SQL 都把该租户作为首个过滤条件。请求 body、query、path 中不接受 `tenant_id`，即使客户端提交也忽略或拒绝。

管理 API 的 `{id}` 可以是用户或角色 ID，但查询必须使用 `(tenant_id,id)`。目标存在于其他租户时返回 `404`，不能用 `403` 暴露对象存在性。superadmin 的隐式全权限也只在认证租户内有效。

### 4.2 统一解析器

新增 `DashboardAccessResolver`，一次解析产生：

- 认证用户及 tenant；
- SaaS 门槛结果；
- `isSuperAdmin`；
- 53 页有效权限集合；
- 每个权限的直接来源、角色来源和最终数据范围；
- 当前选择企业及员工映射。

`/dashboard/access/profile`、前端路由 loader 和 API middleware 使用同一解析器。解析器不缓存跨请求权限结果，确保角色停用和授权变更即时生效；只允许在单次请求内复用结果。

### 4.3 API 中间件顺序

受保护请求按以下顺序执行：

1. 解析 token 与用户；
2. 查询用户 tenant，验证账号启用；
3. 校验 SaaS 整体门槛；
4. 识别精确系统豁免；
5. 读取资源映射；
6. superadmin 同租户隐式放行，普通用户按有效权限判断；
7. 将 `DashboardAccessContext` 写入 request context；
8. 调用原 handler。

错误响应保持现有 envelope。未认证为 `401`，租户整体门槛或权限不足为 `403`，跨租户管理对象为 `404`，乐观锁冲突为 `409`，存储异常为 `500`。

## 5. 管理 API 契约

所有路径均位于 `/dashboard/access`。`/profile` 是已认证用户的自服务访问事实接口；其余管理端点永久仅 superadmin 可用。

### 5.1 读取接口

- `GET /profile`：当前用户、租户、superadmin 标记、可访问页面目录、有效权限、来源和数据范围；所有已认证用户可调用。
- `GET /catalog`：53 页完整目录及 API 资源数量；仅 superadmin 可调用。
- `GET /users`：租户用户分页、角色摘要、直接/继承/有效权限统计与 `version`。
- `GET /users/{id}`：用户的多角色、直接权限、继承来源、有效结果和版本。
- `GET /roles`：启用/停用角色、成员数、权限数、数据范围和版本。
- `GET /audits`：按 actor、target、action 和时间过滤的租户内审计分页。

### 5.2 原子写接口

- `PUT /users/{id}`：一次替换多角色和直接权限；body 包含 `roleIds[]`、`directPermissions[]`、`expectedVersion`。
- `POST /roles`：创建租户角色并原子写入初始权限集合；角色名称在 tenant 内唯一。
- `PUT /roles/{id}`：更新角色名称、备注和权限集合；body 包含 `permissions[]`、`expectedVersion`。
- `PUT /roles/{id}/status`：独立启用或停用角色；body 包含 `status`、`expectedVersion`，停用后请求级解析立即停止其贡献。
- `DELETE /roles/{id}`：删除无成员的非系统角色；body 包含 `expectedVersion`。仍有 `user_roles` 成员时返回 `409`，不能级联删除成员关系。

用户授权写入执行 `SELECT ... FROM mc_user WHERE tenant_id=? AND id=? FOR UPDATE`，以 `mc_user.dashboard_access_version` 比较并递增；角色 CRUD 执行同等的 `mc_rbac_role.dashboard_access_version` compare-and-increment。事务验证所有 role/user 均属于认证 tenant，拒绝四个 `superadmin_only` 权限，物理替换关系、更新聚合版本并追加审计后提交。任一 ID 越界返回 `404`，版本不一致或删除有成员角色返回 `409`，整个事务不产生部分结果。

用户账号生命周期不与“页面访问集合”混在一个 PUT：账号创建、基础资料修改、启停和密码重置继续走旧 `/dashboard/user/store`、`/dashboard/user/update`、`/dashboard/user/statusUpdate`、`/dashboard/user/passwordReset`，这些旧接口永久 superadmin-only；`PUT /dashboard/access/users/{id}` 只管理多角色与直接权限。旧 `/dashboard/role/*` 与 `/dashboard/menu/*` 管理读写接口也永久 superadmin-only，并停止作为 Phase 4 四页的权威写入口；前端角色管理改用上述完整 `/dashboard/access/roles*` CRUD。

## 6. 前端设计

### 6.1 Access profile

前端新增 `access-api.ts`，在路由 loader 中调用 `/access/profile`。`AccessContext` 改为保存：

- `catalog`；
- `effectivePermissions`；
- `allowedRoutes`；
- `permissionSources`；
- `dataScopes`；
- `isSuperAdmin`。

`benchmarkRoutes` 参数及其循环放行逻辑删除。菜单和搜索继续使用 manifest 的分组、标题与顺序，但仅渲染 `allowedRoutes`。深链不在集合中时由 loader 抛出 `403`。

任何 API 返回 SaaS 门槛 `403` 时，客户端清理查询缓存和会话，跳转登录并展示服务不可用原因；普通页面权限 `403` 保留当前会话并显示无权限页。

### 6.2 管理 UI

四个管理页仅 superadmin 可见：

- 员工权限：用户分页、状态筛选、多角色选择、直接权限编辑；
- 角色管理：角色启停、成员数和权限摘要；
- 附加权限：53 页树，显示直接、继承和有效来源，直接权限默认本人范围；
- 授权管理：角色权限树、范围选择、审计列表和版本冲突刷新提示。

页面使用同一 `PermissionTree`，节点显示中文页名、路径、来源标签和数据范围。保存前给出变更摘要，提交使用 `expectedVersion`；`409` 时不覆盖本地表单，提示刷新后重新确认。

Dashboard header 中删除 `/saas-admin/` 链接。390px 下权限树、角色多选、来源标签和审计表使用纵向卡片或内部滚动，不产生页面级横向溢出。

## 7. 兼容迁移策略

迁移在一个数据库事务中依次执行：

1. 检测有效 tenant package 缺少 0039 subscription 的关系；有任一条即失败；
2. 检测旧 user-role 跨租户关系；有任一条即 `SIGNAL SQLSTATE '45000'` 失败；
3. 增加两个 `dashboard_access_version` 聚合版本字段；
4. 建复合唯一索引和六张新表，所有 FK 列类型与父表完全一致；
5. seed 53 页及完整资源映射，并由覆盖门禁证明 53 页使用的每个 Dashboard API 已登记；
6. 把同租户旧 user-role 关系回填到 `user_roles`；
7. 把旧 role-menu 页面关系映射到 `role_permissions`；
8. 为回填写一条迁移审计摘要；
9. 提交。

迁移可重放，不覆盖已存在的新模型显式授权。down 只删除 0127 新表及新增索引，不修改旧 RBAC 数据。历史旧关联保留只读兼容，后续版本再单独移除。

## 8. 方案比较与选择

评估过三种实现方式：

1. 继续扩充 `mc_rbac_menu`：改动少，但页面、按钮、API 和菜单层级仍混在一起，无法清晰表达直接权限、多角色来源和 53 页目录；不采用。
2. 仅在前端维护 route allowlist：能隐藏导航，却不能防直接 API 调用，也无法满足失败关闭；不采用。
3. 新权限目录 + API 资源映射 + 统一服务端解析器：数据迁移与装配工作较多，但能让导航、深链、API、数据范围和审计共享同一事实；采用。

## 9. 测试与验收

所有行为变更遵循 RED → GREEN → REFACTOR，并保留每个 RED 的预期失败输出。最低自动化覆盖：

- 迁移 apply、rollback、replay；53 个 seed 与 4 个 `superadmin_only`；跨租户旧脏数据、缺失 0039 subscription 均导致迁移失败；
- FK 列 signed/unsigned 与父表一致；关系表直接唯一键；用户/角色聚合版本 compare-and-increment；
- 53 页实际使用的每个 Dashboard API 都有资源映射；manifest、前端调用、server 注册或 catalog 漂移均失败，不允许 fallback；
- SaaS 门槛：租户停用、套餐缺失/停用/未生效/过期、额度 JSON 无效、订阅缺失/不可访问、存储错误全部拒绝；
- 普通用户无权限、多角色并集、角色停用、直接权限保留、数据范围优先级；
- superadmin 同 tenant 53 页隐式全权限，跨 tenant 目标 `404`；
- 未映射 API 默认拒绝，映射 API 与页面权限一致，系统豁免精确；
- 角色 create/update/status/delete 完整 CRUD、成员角色删除 `409`、`expectedVersion` 冲突、同事务审计、失败回滚；
- 前端无 `benchmarkRoutes`，导航/搜索/深链一致，四个管理页仅 superadmin；
- 49 个普通页与 53 个超管页清单门禁；
- 桌面与 390px 浏览器真实交互，Dashboard 无 SaaS 链接。

最终只在 `mochat-go-desktop` 做 Docker 验证。验收前后记录四个具名卷与关键数据计数；不执行 `down -v`、`volume rm`、`system prune`，不删除、清理或重建卷，不重建 MySQL/Redis，只在主任务确认不会回退现有运行服务后重建 `app`。截图、API 结果、SQL 只读结果和日志写入 `D:\workspace\mochat-go\output`。

## 10. 完成边界

单元测试、路由测试或 53 条路由可达均不等于产品完成。只有迁移、后端门槛与授权、前端管理交互、真实租户隔离、Docker Desktop 数据保留、49/53 页面矩阵、桌面与 390px 浏览器证据全部成立，才可报告 Phase 4 Dashboard 页面 RBAC 完成。
