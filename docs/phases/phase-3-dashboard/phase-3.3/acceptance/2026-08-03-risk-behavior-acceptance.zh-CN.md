# 3.3.10 风险行为验收记录

## 完成范围

- 风险规则列表、新增、编辑、启用、停用和删除。
- 风险策略支持关键词、通知类型和低/中/高风险等级。
- 风险记录按风险等级、行为、会话类型和规则筛选。
- 风险记录详情及批量确认、忽略审计。
- 会话消息风险评估接口，按启用规则生成幂等风险记录。
- 风险规则和记录强制按企业范围隔离。
- AI 摘要为可选字段，未接入外部模型时不影响风险识别。

## 接口

- `GET/POST/PUT/DELETE /dashboard/risk/rules`
- `PUT /dashboard/risk/rules/status`
- `GET /dashboard/risk/records`
- `POST /dashboard/risk/records/audit`
- `POST /dashboard/risk/evaluate`

## 验证

- Go 风险行为定向测试通过。
- Dashboard TypeScript 检查通过。
- Phase 3.3 与页面注册相关测试 39 个通过。
- Docker 部署后通过浏览器截图检查风险记录和规则配置页面。
