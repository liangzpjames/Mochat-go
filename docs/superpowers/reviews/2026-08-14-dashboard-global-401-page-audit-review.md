# Dashboard 全局 401 与全页面验收审阅记录

日期：2026-08-14  
审阅范围：Dashboard 53 个 manifest 页面、统一认证与页面授权、页面基础交互、服务端部署和数据库迁移一致性。

## 审阅结论

本轮修复、全页面点击验收和服务器部署均已完成，结果通过。

- Dashboard 已授权页面不再因旧认证上下文或重复导航无端返回 401。
- 53 个页面均在真实服务器 `http://139.196.34.133` 登录后逐页访问，并执行了页面登记的安全点击动作。
- 53 个页面均保持登录态、企业名称和账户名称可见，点击后均能返回数据概览。
- 最终验收中异常 HTTP 响应为 0、浏览器 console error 为 0、page error 为 0。
- 最终服务器仅替换 `app` 容器；MySQL、Redis 容器未重建，四个数据卷未变化。

## 根因与正式修复

### 1. 已登录用户进入已授权页面仍返回 401

根因是部分 Dashboard 页面仍使用旧认证缓存或兼容身份上下文，而统一页面权限系统使用新的 Dashboard 会话及 `DashboardAccessContext`。重复导航或页面切换时，旧上下文会把有效登录误判为失效。

修复后，Dashboard 页面统一使用当前会话与访问档案；租户门槛失败和页面权限不足仍分别使用稳定 machine code，页面权限不足不会清除有效登录态。

### 2. 页面 500/503 并非权限问题，而是历史迁移账本漂移

逐页实测发现，服务器迁移账本显示部分 Phase 3 迁移已应用，但相关 DDL 曾因依赖顺序或历史执行中断而未真正落地。此前这些问题被页面未逐一访问所掩盖。

本轮增加四个正式、幂等、仅增量的修复迁移：

- `0134_reconcile_phase3_provider_schema`：恢复 Provider、风险、超时、消息拦截、静默客户、朋友圈、Phase 3.4 获客、录音和 AI 分析等物理表及后续字段。
- `0135_reconcile_material_schema`：恢复 `mc_medium` 的素材作用域、侧边栏可见性、状态字段与索引。
- `0136_reconcile_scrm_customer_schema`：恢复 SCRM 联系人公共池筛选字段、幂等表、分配历史表和查询索引。
- `0137_reconcile_ai_settings_schema`：恢复 AI 知识库和智能体配置表。

这些迁移均使用 `CREATE TABLE IF NOT EXISTS` 或 `information_schema + PREPARE/EXECUTE`，兼容 MySQL 5.7/MariaDB；不删除、不清空、不覆盖已有业务数据。down 文件有意保持非破坏性，因为这些结构本就是旧迁移账本承诺存在的生产结构。

## 验证证据

### 代码与迁移门禁

- 四组迁移契约测试：PASS。
- Dashboard TypeScript typecheck：PASS。
- Dashboard ESLint：PASS。
- Phase 4 catalog/completion gate：PASS，`53/48/5`，未映射 Dashboard API 为 0，`scopeRequired=97`。
- Docker 镜像构建：PASS；前端四个应用和 Go 二进制均在本地镜像构建阶段完成，服务器未编译。
- 每个修复迁移均在本地 MariaDB 一次性隔离 schema 中连续执行两次，验证幂等后删除临时 schema。

### 服务器 53 页真实点击验收

最终证据目录：

`D:\workspace\mochat-go\output\dashboard-auth-audit-20260814\server-final-1ab45be-immutable`

结果：

- manifest 页面：53/53。
- 页面截图：53 张。
- 页面登记点击动作：53/53。
- `unexpectedResponses`：0。
- `consoleErrors`：0。
- `pageErrors`：0。
- 点击后返回数据概览：53/53。
- 企业名称与账户名称保持可见：53/53。

机器可读清单为该目录下 `evidence.json`，其中不保存密码、JWT、Cookie 或 Secret。

## 部署结果

- Git `main` 最终代码：`1ab45be`。
- 最终镜像：`sha256:d5a1edc82a1cd36b8b389a46666e2eae6e24fa20d25417a71fe2b93eb151936a`。
- 服务器应用容器：healthy。
- `/readyz`：PASS。
- MySQL 容器 ID：部署前后相同。
- Redis 容器 ID：部署前后相同。
- 四个卷 `standalone_app-storage`、`standalone_audit-anchor-storage`、`standalone_mysql-data`、`standalone_redis-data` 的名称和挂载点均保持不变。
- 本次临时验收管理员的用户、身份、会话和激活记录已全部删除，数据库残留计数均为 0；本地及服务器临时凭据文件已删除。

## 审阅边界

本结论证明当前 manifest 中的 53 个 Dashboard 页面在最终服务器镜像上能够真实打开、执行安全基础操作并保持正确认证状态。高风险写操作仍由各页面自身的确认和业务验收约束，本轮没有为通过点击验收而自动执行不可逆业务写入。
