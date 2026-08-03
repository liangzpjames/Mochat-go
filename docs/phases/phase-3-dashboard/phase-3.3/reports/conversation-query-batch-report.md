# Phase 3.3 首个页面：员工会话改造记录

## 本轮范围

本轮先实现 `/chat/v2-staff`，用于确认页面结构和交互方向；在该页面审核通过后，继续接入 `/chat/v2-customer` 和 `/chat/v2-group`。其他 Phase 3.3 菜单仍暂不展开。

## 设计修正

对比圆弧页面后，员工会话的正确语义不是将会话类型固定为“员工”，而是先选择员工，再查看该员工关联的客户、客户群和同事会话。因此页面采用三栏结构：

1. 左栏：企业员工搜索、员工列表和当前选中状态。
2. 中栏：按员工查询会话，支持全部、客户、客户群、同事筛选和分页。
3. 右栏：查看选中会话的消息详情，并保留“暂无会话”“暂无详情”“加载失败”等状态。

## 已实现功能

- 新增员工列表接口适配：`/workMessage/fromUsers?page=1&perPage=100&name=...`。
- 员工选择会真实传递 `employeeIds` 到 `/workMessage/toUsers?view=global`。
- 会话筛选默认使用全部类型，不再错误地传递 `conversationType=employee`。
- 使用 `/workMessage/detail?id=...` 加载右侧消息详情。
- 员工、会话、详情三层查询均支持加载、空数据、错误和刷新状态。
- 查询状态保存在 URL 中，刷新页面后可以保留员工、会话类型、页码和选中会话。
- 保留存档权限和后端数据范围，不新增写入或导出动作。

## 验证结果

```text
pnpm exec vitest run src/features/conversation-global/employee-conversation-page.test.tsx
  2 tests passed

pnpm exec vitest run src/features/conversation-global/conversation-global-api.test.ts src/benchmark/page-registry.test.tsx
  20 tests passed

pnpm exec tsc --noEmit -p tsconfig.json
  passed
```

## 待审核事项

请重点确认以下方向：

- 三栏布局是否符合系统使用习惯；
- 左侧员工选择、中间会话列表、右侧消息详情的层级是否清晰；
- “全部 / 客户 / 客户群 / 同事”筛选是否足够，是否需要补充内部群；
- 页面密度、颜色和状态提示是否需要继续向圆弧页面靠拢。

本轮不更新 benchmark manifest 的完成状态，也不推进其他 Phase 3.3 页面。

## 批次 1 扩展

员工会话页面审核通过后，已将同一真实查询组件接入客户会话和群聊会话：

- `/chat/v2-customer` 固定传递 `conversationType=customer`；
- `/chat/v2-group` 固定传递 `conversationType=room`；
- 两个页面继续复用筛选、分页、详情、权限错误和刷新状态；
- 本扩展不新增数据库表、不改变数据卷，也不更新 manifest 完成状态。

## 批次 2 首个页面

已接入 `/chat/trajectory` 会话轨迹页面：

- 使用真实会话列表和详情接口；
- 支持关键词查询、会话选择和消息时间线；
- 每条消息保留发送方、收发方向、时间和原始内容；
- 明确标注当前数据能力为消息时间线，未伪造尚未存在的业务事件；
- 媒体访问和异步导出仍等待对应 provider、任务状态和审计接口完成后实现。

## 批次 2：会话导出页面

已接入 `/chat/export`，本阶段采用“当前查询结果页 CSV 导出”的实现范围：

- 复用真实会话查询接口，支持关键字筛选、加载中、空数据、通用错误和会话内容存档未授权状态；
- 将当前查询结果导出为 CSV 文件，字段包括会话对象、员工、会话类型、最近消息和时间；
- 页面明确说明当前实现范围，不伪造异步导出任务、永久下载链接或媒体导出能力；
- 后续补齐导出任务接口、provider 能力和审计要求后，再升级为异步任务中心模式。

## 会话运营剩余页面

- `/chat/resign-staff`：读取 `/contactTransfer/info`，保留进入真实交接流程的入口；
- `/customer/inheritance`：读取 `/contactTransfer/unassignedList`，保留进入真实客户交接流程的入口；
- `/chat/file-audio`：当前没有独立媒体 provider，页面明确展示未接入状态，不提供虚构文件或下载链接；
- `/chat/refuse-archive`：后续批次已补齐独立授权状态 Provider、跟进与审计页面，详见 `silent-customer-refuse-archive-report.md`。

四个页面均复用企业权限上下文，并覆盖加载、空数据、服务异常和刷新状态。
