# Task 1：前端客户工作台 API 合同报告

## 结果

- 状态：DONE
- Commit：`203d2ab`（`feat: add customer conversation api contracts`）
- 目标测试：通过，10 tests passed

## TDD 记录

### RED

先在 `conversation-global-api.test.ts` 加入客户目录、客户会话列表和客户详情的请求序列化测试，以及客户目录部分数据拒绝测试。

命令：

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts
```

结果摘要：

```text
Test Files  1 failed
Tests       10 tests | 2 failed
```

失败原因符合预期：客户 API 方法尚不存在，调用可选方法得到 `undefined`，请求断言未发生；严格解析测试也因方法不存在而无法取得 Promise。

### GREEN

补充客户 API 类型、严格解析器和三个 API 方法后再次运行同一命令：

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts
```

结果：

```text
Test Files  1 passed (1)
Tests       10 passed (10)
```

另运行：

```powershell
git diff --check -- web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts
```

无格式错误。

## 改动文件

- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
  - 新增客户目录、客户会话列表、客户详情输入/输出类型。
  - 新增固定分页、枚举、计数、稳定 ID、关系/成员状态、统计、消息和 capability 校验。
  - 新增 canonical customerDirectory/customerConversations/customerDetail 请求方法及查询参数序列化。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`
  - 新增请求 endpoint/参数顺序测试和客户目录部分响应拒绝测试。

## 风险与疑问

- 未发现阻塞问题。仅运行了简报指定的目标 Vitest；未修改后端或页面组件。
- 工作树中其他既有改动未纳入本次提交。

## 复核问题修复（2026-08-19）

### RED

新增回归覆盖：客户会话列表空白 `conversationId`；客户详情缺失 `profile`、非法 `targetType`、响应 `conversationId` 与请求不一致；详情 `stats`、`messages`、`capabilities` 失败路径；客户目录固定页大小；客户会话模式枚举与固定页大小。

命令：

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts
```

结果摘要：

```text
Test Files  1 failed (1)
Tests       13 tests | 2 failed
```

失败符合预期：旧实现接受空白稳定 `conversationId`，并接受缺少 `profile` 的客户详情。

### GREEN

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts
```

结果：

```text
Test Files  1 passed (1)
Tests       13 passed (13)
```

类型检查：`corepack pnpm --filter @mochat/dashboard typecheck`，`tsc --noEmit -p tsconfig.json` 通过。

差异检查：`git diff --check -- web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`，无格式错误。尝试使用通配符运行同目录 API 测试时 Vitest 未发现匹配文件；目标 API 测试即该目录现有 API 测试文件，已完整通过。

### 改动

- 客户会话列表每项强制要求非空白 `conversationId`。
- 客户详情新增必需 `profile`（ID、名称、头像、资料状态）校验，限制 `targetType` 为 `customer` 或 `room`。
- 客户详情校验响应 `customerId` 与请求客户 ID、响应 `conversationId` 与请求会话 ID 一致，并统一下游 stats/message/capability 解析失败错误。
- 新增回归测试用例组，共 13 项目标测试通过。
- 未改变员工 API 解析或请求行为。

### Commit

修复提交：`75fcd54`（`fix: tighten customer conversation api contracts`）。

### Concerns

- 工作树中其他既有改动未纳入本次提交。
- 客户详情合同现要求后端提供 `profile`；若后端缺失该字段，前端将按设计拒绝响应。
