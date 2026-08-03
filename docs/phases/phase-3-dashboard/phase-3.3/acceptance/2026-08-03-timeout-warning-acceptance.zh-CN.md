# Phase 3.3 超时预警 Provider 验收记录

## 验收范围

- 页面：`/ai-insight/v2/timeout`
- 数据库迁移：`0111_timeout_warning_provider`
- 后端：规则、设置、记录、审计、分派、评估器和通知意图 Provider
- 前端：超时记录、规则配置、高级设置三个页签

## 自动化验证

- `go test ./internal/dashboard ./internal/migration ./internal/store ./internal/server ./cmd/mochat-go -count=1`：通过。
- `pnpm --filter @mochat/dashboard exec tsc --noEmit -p tsconfig.json`：通过。
- phase3.3、页面注册及 API 定向 Vitest：5 个测试文件、39 项测试通过。
- `git diff --check`：通过。

## Docker 验证

- 仅执行 `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --build --no-deps app`。
- 未执行 `down -v`，现有 MariaDB、Redis 及应用数据卷均保留。
- 0111 迁移已精确应用到当前 MariaDB，八张 `mochat_go_timeout_*` 表均存在。
- `http://localhost:18080/readyz` 返回 HTTP 200。

## 浏览器验收

在当前登录企业中完成以下真实操作：

1. 新增超时规则，设置单聊范围、3 分钟阈值和中风险等级。
2. 停用规则、编辑规则名称并重新启用。
3. 保存结束语词表 `好的,谢谢;收到` 和消息类型白名单 `image,file`，重新读取后内容一致。
4. 展示一条 10 分钟中风险超时记录，详情可查看触发消息、客户、责任员工、AI 摘要及处置状态。
5. 将记录分派给员工 ID 9，再执行“确认超时”。
6. 数据库核对审计流水：`tenant_id=3`、`corp_id=2`，依次记录 `assigned` 和 `confirmed`，记录最终状态为 `confirmed`。
7. 截图检查三个页签、中文字段、筛选按钮和表格布局，未出现“数据提供方未接入”。

## 数据清理

验收创建的规则、策略、记录、审计流水、通知意图和高级设置均按精确标识删除，复核规则和记录剩余数量均为 0；未删除任何既有业务数据或数据卷。

## 范围说明

本轮按设计不包含定时扫描全量会话、实际发送外部通知和调用外部 AI 摘要服务。评估器接口、幂等记录写入及待发送通知意图已经就绪，可供后续会话任务调用。
