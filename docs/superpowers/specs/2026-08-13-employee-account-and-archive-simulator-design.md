# 员工账号生命周期与会话存档模拟器设计

## 目标

将企业微信员工资料与 Dashboard 登录身份建立显式、一对一、可审计的绑定流程，并提供一个不修改真实会话存档配置和游标、可重复运行和单独清理的测试数据生成工具。

## 员工账号生命周期

- 员工同步只更新 `mc_work_employee`，不会自动创建登录账号。
- 超级管理员在员工权限页看到本企业全部在职员工及账号状态：未开通、正常、停用。
- 开通账号时必须填写唯一登录手机号，可选择初始角色。服务端生成 12 位一次性临时密码，只在成功响应中展示一次，不写日志和审计正文。
- 服务端在一个事务内锁定认证 actor、员工和登录标识，创建同租户 `mc_user`、`mochat_go_dashboard_identities`，设置 `must_rotate_password=1`，更新员工 `log_user_id`，写权限关系与审计。
- 首次登录沿用现有 Dashboard `password-change` challenge，修改密码后才能建立普通会话。
- 停用同时停用 `mc_user` 和 Dashboard identity、递增 `auth_version` 并撤销现有 session；重新启用不改变密码。
- 重置密码生成新的临时密码，设置 `must_rotate_password=1`、递增 `auth_version` 并撤销 session。
- 所有 target 只按认证 actor 的 tenant/corp 查询；跨租户返回 404。客户端不能提交 tenant、actor、密码哈希或 superadmin 字段。
- 已有权限编辑继续使用用户聚合版本；账号生命周期使用员工/identity 当前状态，不绕过页面 RBAC。

## HTTP 合同

- `GET /dashboard/access/employees?page=&perPage=`：员工、部门、手机号掩码、账号与登录状态。
- `POST /dashboard/access/employees/{id}/account`：`{loginIdentifier, roleIds, requestId}`，返回账号摘要及一次性 `temporaryPassword`。
- `PUT /dashboard/access/employees/{id}/account/status`：`{status, requestId}`，仅允许 1/2。
- `POST /dashboard/access/employees/{id}/account/reset-password`：`{requestId}`，返回一次性 `temporaryPassword`。
- 以上接口永久 superadmin-only，稳定返回 400/401/403/404/409/500。

## 模拟存档隔离

- 新增 `mochat_go_archive_simulation_batches` 和 `mochat_go_archive_simulation_messages` 注册表；批次键在企业内唯一。
- 模拟器复用生产 `UpsertWorkMessageArchive` 的解析与分表写入，但使用保留前缀 `MOCHAT-SIM:<batch>:` 的 msgid，不更新 `mc_work_message_id` 游标，不修改 `mc_corp.chat_status`、Secret 或 RSA 配置。
- 一个标准批次覆盖员工↔客户、员工↔员工、群聊三类会话，以及文本、图片、文件、语音、视频、位置、名片、链接等展示数据，包含双向消息、时间顺序和可搜索关键字。
- 工具只引用本企业已有员工；必要的模拟客户与群使用同一批次注册表记录，保留前缀避免与真实企微 ID 冲突。
- 同一批次重复 apply 幂等；cleanup 只删除注册表列出的模拟消息、模拟客户和模拟群，不删除真实数据；默认不自动 cleanup。
- CLI：`mochat-archive-simulator apply --corp-id N --batch NAME`、`status`、`cleanup`。DSN 只从 `MOCHAT_MYSQL_DSN` 读取，命令输出不包含凭据。

## 页面

员工权限页以同步员工为主表。未开通员工显示“开通账号”；已开通员工显示“编辑权限、停用/启用、重置密码”。所有写操作先显示变更摘要并二次确认；临时密码使用只读结果对话框且明确只展示一次。移动端表格保持横向滚动可达。

## 验证

- Go 单元测试覆盖 actor 复核、跨租户 404、原子创建与绑定、登录标识冲突、停启用、重置、session 撤销、审计失败回滚、密码不进审计。
- 前端测试覆盖员工可见、开通表单、二次确认、临时密码一次展示、停启用、重置、错误保留与 390px。
- 模拟器集成测试覆盖 apply→apply 幂等、三类会话/全部消息类型、游标和企业配置不变、真实行保留、cleanup 仅删模拟行。

