# Phase 3.3 剩余页面验收记录

## 页面范围

本次覆盖会话运营四页和风险预警六页：

- `/chat/file-audio`
- `/chat/resign-staff`
- `/chat/refuse-archive`
- `/customer/inheritance`
- `/ai-insight/v2/risk`
- `/ai-insight/v2/timeout`
- `/ai-insight/v2/customer-loss`
- `/ai-insight/v2/message-intercept`
- `/ai-insight/v2/keyword-library`
- `/ai-insight/v2/silent-customer`

## 验证命令

```text
pnpm exec vitest run src/features/phase33 src/features/conversation-global src/benchmark/page-registry.test.tsx
64 tests passed

pnpm exec tsc --noEmit -p tsconfig.json
passed

git diff --check
passed
```

全量 Vitest 在当前 120 秒命令窗口内未完成，因此本记录不将其标记为通过；本批次相关页面及会话回归范围已通过 64 个定向测试。

## 能力边界

离职员工、客户继承和客户流失页面使用现有后端读取接口。文件录音、拒绝存档以及风险行为、超时、消息拦截、关键词库、沉默客户页面当前没有可验证的 provider 或任务接口，因此页面只提供筛选/刷新入口和明确的“能力未接入”状态，不生成演示命中记录，不创建下载链接，也不执行未经授权的写操作。

## Docker 验证

使用最新分支构建镜像，并通过 `docker compose ... up -d --no-build app` 更新应用；未执行 `down -v`。`/readyz` 返回 200，应用容器健康，以下数据卷保持存在并继续挂载：

- `mochat-go-desktop_mysql-data`
- `mochat-go-desktop_redis-data`
- `mochat-go-desktop_app-storage`
- `mochat-go-desktop_audit-anchor-storage`
