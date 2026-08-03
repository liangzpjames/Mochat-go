# 消息拦截与关键词库实施报告

## 完成范围

- `/ai-insight/v2/keyword-library`：词库查询、创建/编辑、启停、删除、词条维护和不可变版本发布。
- `/ai-insight/v2/message-intercept`：规则查询与维护、词库版本绑定、同步消息评估、命中记录查询、详情和人工复核。
- 新增 0112 迁移及 7 张业务表，按租户和企业隔离。
- 页面已替换通用 Provider 未接入状态，空数据使用正常空状态。

## 安全边界

评估接口只返回决定并写入审计记录，不接管企微真实发送动作。规则固定关联已发布版本，后续编辑词库不会改写历史命中解释。

## 验证结果

- Go：`go test ./internal/dashboard ./internal/store ./internal/migration ./internal/server ./cmd/mochat-go` 通过。
- React：消息拦截/关键词库与页面注册测试共 20 项通过。
- TypeScript：`tsc --noEmit -p tsconfig.json` 通过。
- Docker：0112 已增量应用；`mochat-go-desktop-app-1` 健康，MySQL 与 Redis 容器及数据卷保留。
- 浏览器：两个页面均返回正常空状态，无“数据提供方未接入”，无控制台错误；桌面视口下搜索与按钮均横向显示。

## 截图

- `evidence/message-intercept-page.png`
- `evidence/keyword-library-page.png`
