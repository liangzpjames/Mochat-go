# SaaS 新客户默认企业修复实施计划

> **执行要求：** 使用 executing-plans 按任务执行；每项代码改动遵守测试先行。

**目标：** 让 SaaS 新建客户和已有无企业客户在 Dashboard 登录后都至少拥有一个可访问的 Fake 企业。

**架构：** 开户事务负责未来数据，增量迁移负责存量数据与套餐额度修正。继续使用现有超级管理员按租户访问企业的授权模型，避免引入虚假的企微员工关系。

**技术栈：** Go、MySQL 5.7、SQL 增量迁移、Docker Compose、React Dashboard。

## 全局约束

- 所有用户审阅文档使用中文。
- 直接在 `main` 修改并部署到 Compose 项目 `mochat-go-desktop`。
- 默认企业使用 Fake 外部平台数据，不对接真实企微。
- 迁移和开户保障必须幂等。

---

### 任务一：用测试锁定迁移契约

**文件：**
- 修改：`internal/migration/migration_test.go`
- 修改：`deploy/standalone/docker-compose.yml`
- 创建：`deploy/standalone/migrations/0099_saas_tenant_default_corp.up.sql`
- 创建：`deploy/standalone/migrations/0099_saas_tenant_default_corp.down.sql`

**接口：**
- 产出：最新迁移版本 `0099_saas_tenant_default_corp`，新库初始化挂载编号 `099`。

- [ ] 修改迁移测试，预期最新版本为 `0099_saas_tenant_default_corp` 且 Compose 包含对应挂载。
- [ ] 执行 `go test ./internal/migration -run TestStandaloneComposeFreshInitMountsLatestMigration -count=1`，确认因缺少迁移而失败。
- [ ] 编写迁移：提升标准版企业额度、修正租户快照、幂等补齐无企业租户。
- [ ] 在 Compose MySQL 初始化卷中加入 0099 挂载。
- [ ] 再次执行迁移测试并确认通过。

### 任务二：用测试锁定未来开户行为

**文件：**
- 创建：`internal/store/saas_tenant_default_corp_test.go`
- 修改：`internal/store/mysql.go`

**接口：**
- 产出：`saasTenantDefaultCorpValues(tenantID int, tenantName string)` 和 `ensureSaaSAdminTenantDefaultCorpTx(...)`。

- [ ] 编写默认企业参数测试，要求企业名为清理空白后的租户名加“演示企业”，企业微信 ID 为 `fake_tenant_<租户ID>`。
- [ ] 执行目标测试，确认因函数不存在而失败。
- [ ] 实现参数函数和幂等事务插入函数。
- [ ] 在 `ProvisionSaaSAdminTenant` 创建租户后、提交事务前调用保障函数。
- [ ] 执行 `go test ./internal/store -run TestSaaSTenantDefaultCorpValues -count=1`，确认通过。

### 任务三：更新迁移健康基线并回归验证

**文件：**
- 修改：`internal/dashboard/saas_admin_system_health.go`
- 修改：与最新迁移总数或版本强绑定的测试和质量门禁文件。

**接口：**
- 产出：系统健康检查预期最新迁移版本为 0099。

- [ ] 搜索并更新只代表“最新迁移”的 0098/98 断言，保留历史迁移断言。
- [ ] 执行 `go test ./internal/migration ./internal/store ./internal/dashboard -count=1`。
- [ ] 执行 `go test ./... -count=1` 和相关前端构建，确认无回归。

### 任务四：部署并进行真实验收

**文件：**
- 使用：`scripts/deploy_docker_desktop.ps1`

**接口：**
- 产出：运行中的 `mochat-go-desktop` 服务及已应用的 0099 迁移。

- [ ] 构建 Linux 服务二进制并更新 Docker 镜像；若完整 Docker 构建网络正常，优先执行可复用部署脚本。
- [ ] 重新创建服务并执行迁移。
- [ ] 查询“测试”租户的套餐 `maxCorps=1`、有效企业数为 1、Fake 企业 ID 正确。
- [ ] 在浏览器打开 Dashboard，验证登录后企业可见、自动选中和基础页面展示。
- [ ] 检查 Git 差异，提交、推送 `main`。

