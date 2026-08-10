# SaaS 与 Dashboard 身份域隔离及单企业绑定设计

日期：2026-08-11

状态：已确认
适用范围：MoChat Go SaaS Admin、Dashboard、租户开户、企业微信配置、认证与会话

## 1. 背景

当前系统虽然已有独立的 SaaS Admin 与 Dashboard 前端，但两者的服务端身份仍耦合在 `mc_user`：

- Dashboard 通过 `mc_user.phone/password` 登录，并从 `mc_user.tenant_id` 取得租户。
- SaaS Admin 也通过 `mc_user` 识别平台用户，再叠加 `mochat_go_saas_admin_user_access`。
- 两端共用 `MOCHAT_SIMPLE_JWT_SECRET`、基础 JWT 解析器和部分会话设施。
- SaaS Admin 登录仍跳转 `/security/login`，实际上借用 Dashboard 身份。
- Dashboard Session 保存 `corpId`，应用启动时加载企业列表，并调用 `/dashboard/corp/bind` 切换企业。
- 企业信息页仍是列表、新建、编辑模型，允许一个租户创建多家企业。

这些行为与目标产品边界冲突：SaaS 是平台运营面，Dashboard 是单租户业务面；SaaS 负责开通租户、套餐、订阅与 Dashboard 超级管理员，Dashboard 只维护本租户唯一企业及其企业微信凭据。

## 2. 已确认决策

1. SaaS 与 Dashboard 使用两个完全独立的身份域，不共享登录用户、密码、Token、MFA 会话或授权主体。
2. SaaS 初始平台管理员由项目初始化命令创建；初始化过程不创建 Dashboard 用户。
3. Dashboard 超级管理员只能由 SaaS 开户或超级管理员治理流程授予、替换、停用和恢复。
4. 一个 Dashboard 租户只能绑定一个企业；Dashboard 不能新建、删除或切换企业。
5. Dashboard 登录账号全局唯一且只属于一个租户，因此登录不需要租户代码或企业选择。
6. 同一个手机号不能同时成为多个租户的 Dashboard 登录身份。若未来需要一人管理多租户，应另立项目设计，不在本方案预留弱化约束。
7. Dashboard 企业设置只能维护权威绑定企业的资料、企业微信凭据、应用凭据和会话存档凭据。
8. 企业微信 CorpID 首次验证绑定后不可由 Dashboard 直接更换；重新绑定只能由 SaaS 高风险治理流程执行。
9. 本次采用维护窗口一次性切换，不实现双读、双写、共享表增加 `realm`、前端隐藏按钮等临时方案。

## 3. 设计目标

- 身份、Token、会话、权限和审计在 SaaS 与 Dashboard 之间形成可验证的隔离边界。
- SaaS 可以安全初始化平台管理员，并能完整开通 Dashboard 租户及超级管理员。
- Dashboard 登录后由服务端直接解析唯一企业，不再加载或保存企业选择状态。
- 企业资料与所有 Secret 均按唯一企业维护，客户端永远不能提交 `tenant_id` 或任意 `corp_id` 选择目标。
- 数据库约束、应用事务、接口守卫和自动化门禁共同保证一租户一企业。
- 迁移失败可诊断，跨租户脏数据、重复登录身份和多企业歧义不得静默修复。

## 4. 非目标

- 不支持一个 Dashboard 账号管理多个租户。
- 不支持一个租户绑定多个企业微信企业。
- 不实现 SaaS 与 Dashboard 单点登录。
- 不开放 Dashboard 自助注册、企业新增或超级管理员提升。
- 不把企业微信 Secret、会话存档 Secret 或激活令牌写入日志、审计正文或可重复读取的响应。
- 不继续兼容 `/dashboard/corp/select`、`/dashboard/corp/bind`、`POST /dashboard/corp/store` 的正常业务调用。

## 5. 总体架构

系统形成三个明确边界：

### 5.1 平台身份域

SaaS Admin 使用 `mochat_go_saas_admin_users` 作为唯一用户事实，使用独立登录、JWT、MFA、会话和权限关系。平台用户没有 `tenant_id`，其平台访问范围由 SaaS Admin RBAC 决定。

### 5.2 租户身份域

Dashboard 业务用户资料继续存放在 `mc_user`，但认证凭据迁入 `mochat_go_dashboard_identities`。一个 Dashboard Identity 一对一映射一个 `mc_user`，并通过该用户唯一归属一个租户。

### 5.3 单企业业务域

`mochat_go_tenant_corp_bindings` 是租户与企业的权威绑定表。请求认证后，服务端从租户绑定中解析内部 `corp_id` 并写入 `DashboardPrincipal`；页面和业务 API 不再从 Session、Query、Path 或 Body 接受企业选择。

## 6. 数据模型

### 6.1 `mochat_go_saas_admin_users`

字段：

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | `int(10) unsigned` | 主键、自增 |
| `login_name` | `varchar(64)` | 非空、全局唯一、规范化小写 |
| `phone` | `varchar(32)` | 可空、非空值唯一 |
| `password_hash` | `varchar(255)` | 非空，只存密码哈希 |
| `name` | `varchar(255)` | 非空 |
| `status` | `tinyint unsigned` | `1=正常`、`2=禁用` |
| `must_rotate_password` | `tinyint unsigned` | 初始化用户固定为 `1` |
| `auth_version` | `bigint unsigned` | 默认 `1`，注销全部会话时递增 |
| `mfa_required` | `tinyint unsigned` | 默认 `1` |
| `created_at/updated_at` | `timestamp` | 审计时间 |

平台用户不软删除；离职或撤销时置为禁用并递增 `auth_version`。这样登录名不会被重新分配给另一自然人。

现有 `mochat_go_saas_admin_user_access`、SaaS Admin 角色关系和平台审计 actor 全部改为引用该表。历史平台管理员从平台租户的 `mc_user` 复制并保留原数值 ID，以保持历史 actor ID 可解释。

### 6.2 `mochat_go_dashboard_identities`

字段：

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `user_id` | `int(10) unsigned` | 主键，外键到 `mc_user.id` |
| `login_identifier` | `varchar(64)` | 全局唯一，当前为规范化手机号 |
| `password_hash` | `varchar(255)` | 非空 |
| `status` | `tinyint unsigned` | `1=正常`、`2=禁用` |
| `must_rotate_password` | `tinyint unsigned` | 激活后为 `0` |
| `auth_version` | `bigint unsigned` | 默认 `1` |
| `mfa_required` | `tinyint unsigned` | 按租户策略写入 |
| `activated_at` | `timestamp NULL` | 首次激活时间 |
| `created_at/updated_at` | `timestamp` | 审计时间 |

`mc_user` 只保留业务资料、租户归属、状态和 Dashboard RBAC 信息。完成切换后删除 `mc_user.password`，所有密码设置、重置和校验只访问 Dashboard Identity Store。

### 6.3 `mochat_go_dashboard_identity_activations`

保存一次性激活令牌的 SHA-256 摘要、`user_id`、失效时间、消费时间、创建 actor 和 request ID。原始令牌只在 SaaS 授权成功响应中出现一次，不落日志、不落审计正文。令牌默认 24 小时失效，消费后立即不可复用。

### 6.4 `mochat_go_tenant_corp_bindings`

字段：

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `tenant_id` | `int(10) unsigned` | 主键，外键到 `mc_tenant.id` |
| `corp_id` | `int(10) unsigned` | 唯一，复合外键到 `mc_corp(tenant_id,id)` |
| `status` | `tinyint unsigned` | `1=待配置`、`2=已验证`、`3=暂停` |
| `version` | `bigint unsigned` | 乐观锁版本，默认 `1` |
| `verified_wx_corpid` | `varchar(255) NULL` | 首次验证成功后写入，非空值全局唯一；之后不可由 Dashboard 修改 |
| `verified_corp_name` | `varchar(255)` | 企业微信返回的权威名称 |
| `verified_at` | `timestamp NULL` | 验证时间 |
| `created_at/updated_at` | `timestamp` | 审计时间 |

为建立真实复合外键，迁移先把 `mc_user.tenant_id`、`mc_corp.tenant_id` 规范为与 `mc_tenant.id` 一致的 `int(10) unsigned`，并为 `mc_corp` 增加唯一索引 `(tenant_id,id)`。迁移必须先从 `information_schema` 枚举所有引用这两个字段的外键、索引和 0127 RBAC 关系表；在同一个分阶段迁移中删除依赖约束、统一所有关联 `tenant_id` 类型、重建约束并校验，禁止只改主表造成复合外键失效。

每个启用租户必须且只能有一条 binding。SaaS 开户事务创建内部 `mc_corp` 占位记录和 binding；Dashboard 不创建 `mc_corp`。

### 6.5 凭据存储

继续使用现有 AES-256-GCM 企业凭据加密设施，但明确以下规则：

- `employee_secret`、`contact_secret`、回调 Token、EncodingAESKey、会话存档 Secret 只写入 `wecom_credentials_ciphertext`。
- 迁移验证密文成功后，将对应明文字段置空；运行代码不得回退读取明文字段。
- `mc_work_agent.wx_secret` 迁入新的加密列，明文字段置空并停止读取。
- API 读取只返回 `configured`、`keyId`、`updatedAt` 等元数据，不返回 Secret。
- 审计只记录“哪些凭据字段发生变化”和版本，不记录旧值、新值、哈希或密文。

## 7. 认证与 Token 边界

### 7.1 独立配置

- SaaS：`MOCHAT_SAAS_ADMIN_JWT_SECRET`、`MOCHAT_SAAS_ADMIN_JWT_PREFIX`、`MOCHAT_SAAS_ADMIN_JWT_TTL`。
- Dashboard：`MOCHAT_DASHBOARD_JWT_SECRET`、`MOCHAT_DASHBOARD_JWT_PREFIX`、`MOCHAT_DASHBOARD_JWT_TTL`。
- 两个 Secret 必须同时配置、必须不同，启动时相同则失败。
- `MOCHAT_SIMPLE_JWT_SECRET` 不再用于 SaaS 或 Dashboard 登录，仅在尚未迁移的非本方案组件中保留；完成本方案后门禁禁止两端引用它。

### 7.2 Token Claims

SaaS Token：

- `sub=saas-user:<id>`
- `realm=saas_admin`
- `aud=mochat-saas-admin`
- `iss=mochat-go/saas-auth`
- `uid`、`jti`、`iat`、`nbf`、`exp`、`auth_version`

Dashboard Token：

- `sub=dashboard-user:<user_id>`
- `realm=dashboard`
- `aud=mochat-dashboard`
- `iss=mochat-go/dashboard-auth`
- `uid`、`jti`、`iat`、`nbf`、`exp`、`auth_version`

解析器必须同时校验签名、`realm`、`aud`、`iss`、会话和 `auth_version`。SaaS Token 请求 Dashboard 或 Dashboard Token 请求 SaaS 均返回 401，不进入业务授权。

### 7.3 前端 Session

- SaaS 使用专用 Auth Store 和 `mochat_saas_admin_*` 存储键。
- Dashboard 使用 `mochat_dashboard_*` 存储键，但 Session 删除 `corpId`。
- Dashboard Session 只保存 Token、用户显示信息和失效时间；租户和企业每次由服务端认证事实解析。
- 两个前端登出只清理自己的 Session，不影响另一个身份域。

## 8. 平台初始化

`cmd/mochat-bootstrap` 只负责平台基础数据和首个 SaaS 平台管理员：

- 登录名从 `MOCHAT_BOOTSTRAP_SAAS_ADMIN_LOGIN` 读取。
- 初始密码只允许通过 Docker Secret 文件 `MOCHAT_BOOTSTRAP_SAAS_ADMIN_PASSWORD_FILE` 读取。
- 不接受命令行明文密码，不在日志打印登录凭据或激活令牌。
- 初始化写入密码哈希，`must_rotate_password=1`、`mfa_required=1`。
- 初始化具有幂等 request key；重复执行只验证既有主体，不重置密码。
- 初始化过程不创建 Dashboard 租户、企业、员工或超级管理员。

## 9. SaaS 授权 Dashboard

### 9.1 开户事务

SaaS `tenant.provision` 在一个应用事务中完成：

1. 创建或锁定租户。
2. 写入套餐、26 项额度快照和订阅生命周期。
3. 创建唯一 `mc_corp` 占位记录。
4. 创建 `mochat_go_tenant_corp_bindings(status=待配置)`。
5. 创建 Dashboard 超级管理员 `mc_user`。
6. 创建 Dashboard Identity 和一次性激活令牌摘要。
7. 写入 Dashboard RBAC 超级管理员事实，不创建普通角色授权行。
8. 写入 SaaS 开户审计和授权审计。
9. 提交后只向有权限的 SaaS actor 返回一次性激活 URL。

任一步失败必须整体回滚。重复 idempotency key 返回原业务结果，但不再次返回原始激活令牌；需要重新激活时必须执行独立的高风险“重发激活”操作。

### 9.2 超级管理员治理

SaaS 提供替换、停用、恢复和重发激活操作：

- 每次写入要求 `expectedVersion`、高风险审批、request ID 和审计。
- 一个启用租户至少保留一个正常、已激活的 Dashboard 超级管理员。
- 新超级管理员激活成功前，不撤销最后一个既有超级管理员。
- Dashboard 的用户和角色管理 API 永久禁止修改 `isSuperAdmin`。

## 10. Dashboard 登录与请求上下文

### 10.1 登录

Dashboard 登录只提交手机号和密码。`login_identifier` 全局唯一，因此不选择租户或企业。认证顺序：

1. 查询 Dashboard Identity。
2. 校验身份状态、密码、MFA 和 `auth_version`。
3. 查询 `mc_user` 并校验用户状态和租户归属。
4. 执行套餐、订阅、租户状态门槛。
5. 解析唯一 tenant-corp binding。
6. 签发 Dashboard Token。

租户不可访问仍返回 `TENANT_ACCESS_DENIED`；企业尚未配置不阻止超级管理员登录，但业务页面访问返回 `CORP_CONFIGURATION_REQUIRED` 并引导到企业设置。

### 10.2 `DashboardPrincipal`

每个已认证请求生成：

```go
type DashboardPrincipal struct {
    UserID         int
    TenantID       int
    CorpID         int
    CorpStatus     CorpBindingStatus
    IsSuperAdmin   bool
    AuthVersion    uint64
}
```

所有 Dashboard handler、service 和 store 只从 Context 读取该 Principal。客户端提交的 `tenantId`、`corpId`、`actorUserId` 一律严格 JSON 解码拒绝；历史必须携带 `corpId` 的外部回调协议在独立边界校验，不复用 Dashboard Session。

## 11. 企业信息与凭据管理

### 11.1 页面模型

企业信息页改为唯一企业详情，不再展示：

- 新建企业按钮
- 企业列表、搜索、总数和分页
- 企业选择器
- 删除、切换、绑定其他企业

页面分为：基本信息、企业微信绑定、通讯录与客户联系凭据、应用凭据、会话存档、连接状态与审计摘要。

### 11.2 API

- `GET /dashboard/company/profile`：返回唯一企业、绑定状态、凭据配置状态和版本。
- `PUT /dashboard/company/profile`：修改允许的显示资料，要求 `expectedVersion`。
- `PUT /dashboard/company/wecom-credentials`：轮换通讯录、客户联系和回调凭据，不返回 Secret。
- `PUT /dashboard/company/agent-credentials`：维护应用 AgentID 与 Secret。
- `PUT /dashboard/company/archive-credentials`：轮换会话存档 Secret 和私钥。
- `POST /dashboard/company/verify`：调用企业微信验证 CorpID 和凭据；首次成功后写入权威 CorpID、企业名称和 `status=已验证`。
- `GET /dashboard/company/audits`：分页读取当前企业配置审计。

这些接口全部为 Dashboard `superadmin_only`，目标企业固定来自 `DashboardPrincipal.CorpID`。跨企业 ID 不存在于请求合同中。

### 11.3 CorpID 不可变

待配置状态可提交候选 CorpID 并验证。第一次验证成功后：

- Dashboard 更新请求不再接受 CorpID。
- Secret 可以轮换并重新验证。
- 企业微信返回的权威名称单独保存；显示名称可在 Dashboard 修改。
- 更换 CorpID 只能从 SaaS 发起 `tenant.corp.rebind` 高风险审批，执行前要求数据影响预检和完整审计。

### 11.4 企业微信员工同步

员工可以通过企业微信同步到 MoChat。唯一企业完成 CorpID、通讯录 Secret 等凭据验证后，Dashboard 超级管理员可执行首次全量同步，后续由同一租户和企业边界下的 `work-contact-sync`、`work-room-sync` 等任务执行增量同步：

- 同步目标固定为 `DashboardPrincipal` 解析出的唯一 `tenant_id + corp_id`，不接受前端选择企业。
- 企业微信通讯录成员写入现有员工与部门业务表；匹配到 Dashboard 用户时只关联业务主体，不自动创建登录密码或提升超级管理员。
- 员工离职、部门调整和账号状态以企业微信事实更新业务资料，但 Dashboard 登录身份的停用与角色权限仍按本系统治理流程执行。
- 凭据未验证、企业绑定暂停或租户门槛失败时，同步任务失败关闭并记录不含 Secret 的可诊断状态。
- 页面显示最近同步时间、游标、成功/失败数量和脱敏错误，并提供可审计的“立即同步”；不会通过同步创建第二家企业。

## 12. 旧接口与旧代码处置

最终版本必须：

- 取消注册 `GET /dashboard/corp/select`。
- 取消注册 `POST /dashboard/corp/bind`。
- 取消注册 `POST /dashboard/corp/store`。
- 将旧 `/dashboard/corp/index/show/update` 从 Dashboard 生产调用迁移到新的唯一企业 API；旧路径普通和超级管理员均默认拒绝。
- 删除 Dashboard `CorpProvider`、企业切换器、`corpId` Session 字段和按企业切换 Query Cache 的逻辑。
- 删除 SaaS Admin 到 `/security/login` 的跳转，改用 SaaS 专用登录页。
- SaaS Store 不得再查询 `mc_user` 识别平台 actor。
- Dashboard Auth Store 不得再读取或写入 `mc_user.password`。

不保留旧接口 fallback、双读或隐藏开关。

## 13. 错误合同

| HTTP | machine code | 含义 |
| --- | --- | --- |
| 400 | `INVALID_REQUEST` | 严格 JSON、字段或格式错误 |
| 401 | `AUTH_REALM_MISMATCH` | Token 身份域不匹配 |
| 401 | `SESSION_INVALID` | 会话、版本或身份状态失效 |
| 403 | `TENANT_ACCESS_DENIED` | 租户、套餐或订阅不可访问 |
| 403 | `CORP_CONFIGURATION_REQUIRED` | 唯一企业尚未完成验证配置 |
| 403 | `DASHBOARD_PERMISSION_DENIED` | 页面或操作权限不足 |
| 404 | `RESOURCE_NOT_FOUND` | 同租户目标不存在；不泄漏跨租户事实 |
| 409 | `VERSION_CONFLICT` | 乐观锁冲突，客户端保留表单 |
| 409 | `LOGIN_IDENTIFIER_CONFLICT` | Dashboard 登录身份已被其他租户占用 |
| 409 | `CORP_ALREADY_BOUND` | CorpID 已绑定或当前租户已完成绑定 |
| 422 | `WECOM_CREDENTIAL_INVALID` | 企业微信连接或凭据验证失败 |
| 500 | `INTERNAL_ERROR` | 未分类内部错误，不返回 Secret |

## 14. 迁移与切换

### 14.1 迁移前预检

所有数据一致性预检必须在第一条 DDL 前完成：

- 活跃 Dashboard 用户手机号为空、格式非法或全局重复。
- `mc_user.tenant_id`、`mc_corp.tenant_id` 为负数、目标租户缺失或类型越界。
- 一个租户存在零个或多个有效企业的歧义。
- SaaS actor、角色、访问关系和审计引用无法映射到平台身份。
- 企业明文凭据无法使用当前 key ring 加密或既有密文无法解密。
- 现有 Dashboard 超级管理员跨租户、停用或缺少有效租户。

零企业租户由迁移创建一个占位企业；恰好一个企业直接建立 binding；多个企业必须由 SaaS 操作者在维护窗口前提交明确的 tenant→corp 清单，迁移不得选择最小 ID 或静默丢弃其他企业。

### 14.2 维护窗口

由于 MySQL/MariaDB DDL 会隐式提交，且本设计拒绝双读双写，发布使用维护窗口：

1. 记录容器、四卷、迁移 ledger、关键表计数并完成数据库备份。
2. 停止 `app`，保持 MySQL、Redis 和卷运行。
3. 在隔离临时数据库执行真实 apply→down→apply 和脏数据失败测试。
4. 在业务库执行全部预检。
5. 应用 schema、backfill、cutover 三个迁移；ledger 仅在各迁移全部语句成功后记账。
6. 部署只支持新身份域和唯一企业模型的应用。
7. 执行 SaaS、Dashboard、跨域 Token、单企业与数据保留验收。

本次不通过旧应用回滚数据库。上线后若产生新身份或新审计，使用前向修复；只有在新系统未开放写入前，才允许执行 down 并恢复旧镜像。

## 15. 安全要求

- 初始化密码、JWT Secret、激活令牌、企业微信 Secret 和会话存档私钥不得出现在命令行、日志、错误、审计或 evidence 正文。
- 密码和激活令牌使用常量时间校验；登录失败不区分用户不存在与密码错误。
- SaaS 开户、超管替换、CorpID 重绑和凭据轮换必须审计且支持 request ID/idempotency key。
- 所有写事务在读取目标前重新锁定并校验 actor 身份状态与权限，防止 TOCTOU。
- Dashboard 企业配置写事务锁定 `(tenant_id,corp_id)` binding，比较并递增聚合版本，再写密文和审计。
- Secret 更新响应只包含版本和配置状态。

## 16. 测试与完成门禁

### 16.1 自动化测试

- JWT：错误 Secret、realm、audience、issuer、auth_version 全部 401。
- Identity Store：SaaS/Dashboard 表互不读取；Dashboard 登录名全局唯一。
- SaaS 开户：租户、套餐、订阅、唯一企业、超管、Identity、激活和审计同事务。
- 单企业：一个租户不能建立第二条 binding；CorpID 不能重复绑定。
- Dashboard 登录：无企业选择，唯一企业由服务端解析。
- 企业配置：无 `tenantId/corpId/actor` 输入；Secret 不回显；版本冲突不覆盖。
- 迁移：真实 MariaDB apply→down→apply，重复手机号、多企业歧义、跨租户和不可解密凭据在第一条 DDL 前失败。
- 前端：SaaS/Dashboard 登录页和 Session Key 独立；企业页没有新增、列表、分页或切换器。

### 16.2 静态门禁

门禁扫描生产源码并在以下情况失败：

- SaaS Auth/Access Store 引用 `mc_user`。
- Dashboard Auth Store 引用 `mc_user.password`。
- SaaS 与 Dashboard 使用同一 JWT 配置。
- Dashboard 前端 Session 包含 `corpId`。
- 生产路由注册 corp select/bind/store。
- 企业信息页出现“新建企业”、企业列表或企业选择器。
- 企业配置 API Body 包含 `tenantId`、`corpId` 或 actor 字段。
- Secret 字段出现在 JSON 响应、审计 payload 或日志格式串。

### 16.3 真实验收

- 项目初始化只产生 SaaS 平台管理员，首次登录强制改密和 MFA。
- SaaS Token 访问 Dashboard 为 401；Dashboard Token 访问 SaaS 为 401。
- SaaS 开户后生成一次性 Dashboard 激活链接；激活后超级管理员可登录。
- 用相同手机号为第二租户开户返回 409 且零写入。
- Dashboard 登录页不要求企业，登录后无企业选择器。
- 未配置企业时只能进入企业设置；验证成功后业务页面恢复。
- 企业设置只显示唯一企业，不能新增或更换 CorpID；Secret 不回显。
- 超管替换、停用、恢复和重发激活符合审批、版本与审计要求。
- Desktop 与 390px 页面可用，控制台无错误，意外 4xx/5xx 为 0。
- MySQL、Redis 容器和四个数据卷在 app-only 重建前后保持不变。

## 17. 完成定义

只有同时满足以下条件才能宣称完成：

- 两个身份域的数据表、Store、JWT、Session、登录页面和权限主体均已分离。
- 旧共享身份、旧 corp 选择/绑定/新增生产路径已删除，不存在 fallback。
- 所有租户都有且只有一个权威企业 binding。
- Dashboard 超级管理员只能由 SaaS 治理。
- 企业微信与会话存档 Secret 全链路加密、掩码、审计且不泄漏。
- 迁移、单元、集成、前端、E2E、静态门禁和 Docker 数据保留验收全部通过。
