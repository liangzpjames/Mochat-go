# Task 5 实施报告：敏感词功能对标

## 结果

已完成 `/sensitive-word/index` 对应页面的真实前后端闭环，页面固定分为“敏感词记录”和“敏感词配置”。浏览器真实登录态证据按约定延后 Task 11，本任务未伪造浏览器证据。

## 实现范围

- 前端：记录组合筛选、命中详情、词组新增/改名、词条新增/移动/启停/删除、删除确认、统一成功/失败反馈、409/403/空态/加载失败 PageState。
- API：六类 mutation 全部携带 `version` 和 `idempotencyKey`；删除额外携带 `confirmed`；记录筛选完整序列化并规范化返回值。
- 后端：认证 tenant/corp 与逐接口 RBAC 隔离；mutation 元数据校验；配额、403、404、409 和扫描结果联通失败映射。
- 持久化：MariaDB 事务乐观锁、按 tenant/corp/actor/action/idempotencyKey 幂等重放、操作前后快照审计；列表返回稳定版本。
- 缓存：前端仅失效受影响的词组或词条查询键，记录查询不被配置 mutation 无差别清空。
- 验收：功能矩阵敏感词行和 smoke 合同已更新。

## 测试证据

- `go test ./internal/dashboard ./internal/store -run 'SensitiveWord' -count=1`：通过。
- `pnpm vitest run src/features/sensitive-word/sensitive-word-api.test.ts src/features/sensitive-word/sensitive-word-page.test.tsx`：2 个文件、7 个测试通过。
- `pnpm typecheck`：通过。
- `MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test ./internal/store -run 'TestIntegrationSensitiveWordMutation' -count=1`：真实 MariaDB fixture 通过，覆盖持久化、corp 隔离、幂等重放、版本冲突和审计。
- `go vet ./internal/dashboard ./internal/store`：通过（收敛前已执行，最终修改仅增加已由 Go 测试编译覆盖的幂等预检）。
- `docker run ... bash -n scripts/smoke_sensitive_word_dashboard.sh`：脚本语法通过。
- `git diff --check`：通过。
- Dashboard production build：收敛前已通过；最终前端修改仅为额度错误信息透传，随后 typecheck 与目标 Vitest 通过。全量 Dashboard 测试由控制器复跑。

## 环境限制与残余风险

- 宿主 `bash` 指向缺少 `/bin/bash` 的 WSL，无法直接执行完整 smoke；已使用容器 Bash 完成语法验证，并使用现有 MariaDB 服务完成真实持久化集成门禁。
- 未执行浏览器真实登录态验收，按计划留到 Task 11。
- 未执行 Dashboard 全量 Vitest，按控制器要求留待后续复跑。
