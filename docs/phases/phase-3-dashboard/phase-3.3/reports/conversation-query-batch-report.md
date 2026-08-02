# Phase 3.3 批次 1：会话检索基础实施报告

## 范围

本批次完成三个会话菜单的真实查询页面接入：

| 路由 | 页面 | 实现方式 |
| --- | --- | --- |
| `/chat/v2-staff` | 员工会话 | 复用会话存档查询底座，固定 `conversationType=employee` |
| `/chat/v2-customer` | 客户会话 | 复用会话存档查询底座，固定 `conversationType=customer` |
| `/chat/v2-group` | 群聊会话 | 复用会话存档查询底座，固定 `conversationType=room` |

## 实现内容

- 扩展 `ConversationGlobalPage`，支持 `fixedConversationType`，共享现有筛选、分页、详情抽屉、授权错误、刷新恢复和空/异常状态。
- 在 `createBenchmarkP0Pages` 中为三条 Phase 3.3 路由注入真实页面，避免继续落到 `DemoPage`。
- 固定页面隐藏跨类型快捷筛选，并在查询时强制服务端收到对应类型，客户端 URL 不能覆盖菜单范围。
- 保留会话存档未开通 `40301` 与普通 RBAC `403` 的不同反馈。

## 复用边界

本批次复用以下既有后端入口：

- `/workMessage/toUsers?view=global`；
- `/workMessage/detail`；
- `WorkMessageToUsers`、`WorkMessageByArchiveID`；
- `WorkMessageArchiveAuthorized` 的企业存档授权判断。

本批次没有把旧接口直接暴露给三个页面以外的功能，也没有新增 migration。轨迹、媒体、导出、风险事件和规则配置不属于本批次，继续保持未完成状态。

## 验证证据

```text
pnpm exec vitest run
  56 test files passed
  382 tests passed

pnpm exec tsc --noEmit -p tsconfig.json
  passed

go test ./...
  passed
```

TDD 验证包含：

1. 先添加固定会话类型测试并确认失败；
2. 实现页面固定类型和注册注入；
3. 运行会话页面与 registry 测试，33 项通过；
4. 运行 Dashboard 全量测试和类型检查，382 项通过；
5. 运行 Go 全量回归，全部通过。

## 当前状态限制

本批次代码已具备自动化查询合同，但尚未执行真实登录态浏览器验收，也未将 manifest 中三页的 `implementation/backend/acceptance` 提升为完成状态。必须在具备有效会话存档授权、测试企业和浏览器证据后再更新 manifest。
