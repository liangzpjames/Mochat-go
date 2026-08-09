# 企业微信生产同步修复设计

## 目标

在保留现有租户、管理员账号、Docker 数据卷和企微凭据的前提下，使服务器 `139.196.34.133` 能完成成员/部门、客户标签、客户及客户群的真实同步，并在 Dashboard 对应页面正确显示同步结果。

## 已确认现状

- 运行项目为 `/opt/mochat-go/deploy/standalone/docker-compose.yml`，`app`、MySQL、Redis 健康。
- 有效企业为 `corp_id=1`；同一企微 `CorpID` 的 `corp_id=2` 已软删除，其历史数据不会作为当前企业展示。
- 成员/部门、客户标签、客户群同步已返回 HTTP 200。
- 客户同步把初始化占位员工 `bootstrap_1` 传给企微，企微返回 `60111 userid not found`，导致整批同步返回 HTTP 500。
- `bootstrap_1` 负责将管理员账号绑定到 Dashboard 企业，不能直接删除。
- 成员同步已经根据企微 `get_follow_user_list` 将真实客户联系成员标记为 `contact_auth=1`；占位员工为 `contact_auth=2`。

## 方案

修改 `MySQLStore.WorkContactSyncEmployees` 的查询，仅返回当前企业中 `contact_auth=1`、`wx_user_id` 非空且未删除的成员。这样客户同步只请求企微明确授权的客户联系成员，同时保留 Dashboard 管理员占位绑定。

不采用以下替代方案：

- 不清空或删除 `bootstrap_1`，避免破坏管理员企业绑定。
- 不在遇到 `60111` 时静默跳过任意成员，避免掩盖真实的企微通讯录或授权异常。
- 不修改企业微信真实数据，只从企微读取并回写 MoChat。

## 数据流

1. 成员同步读取企微部门、成员和客户联系成员列表。
2. MoChat 将真实客户联系成员保存为 `contact_auth=1`。
3. 客户同步查询 `corp_id=1 AND contact_auth=1` 的成员。
4. 对每个成员调用 `externalcontact/list`，再调用 `externalcontact/get` 拉取客户详情。
5. 在事务中写入客户、跟进关系、标签关系和同步时间。
6. Dashboard 的员工、组织架构、客户标签、客户和客户群页面从同一有效企业读取数据。

## 错误处理

- 任一真实客户联系成员的企微请求失败时仍返回明确错误，不吞掉异常。
- 同步前保留完整数据库备份及 SHA256。
- 部署时只重建 `app`，不删除或重建 MySQL、Redis 及命名数据卷。
- 保留服务器当前 `MOCHAT_SIMPLE_JWT_SECRET` 和企微加密配置，不在日志或文档中记录值。

## 测试与验收

- RED：新增查询契约测试，要求 SQL 包含 `contact_auth = 1`；实现前必须失败。
- GREEN：最小修改查询并通过目标测试。
- 回归：运行相关 Go 测试、`go test ./...`、Dashboard 测试/typecheck/build 和 `git diff --check`。
- 部署：从本地同步源码到 `/opt/mochat-go`，在服务器执行 Compose 构建，只重建 `app`，确认三容器健康且 `/healthz`、`/readyz` 为 200。
- 真实同步：依次执行成员/部门、客户标签、客户、客户群同步，两次执行验证幂等；核对数据库计数、企业归属和更新时间。
- 浏览器：登录 Dashboard 后检查组织架构、员工、客户标签、客户、客户群和数据概览页面，确认真实数据可见、无本地接口 4xx/5xx、无控制台错误。
