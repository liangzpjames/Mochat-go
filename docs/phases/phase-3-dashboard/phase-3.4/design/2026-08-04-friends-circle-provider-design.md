# 朋友圈 Provider 设计

日期：2026-08-04

## 设计结论

朋友圈能力拆为两层：本地持久化领域 Provider 负责任务、素材、草稿、状态和审计；`FriendsCirclePublisher` 负责正式企业微信发布。两层不得混用，本地写入成功不代表外部发布成功。

## 数据模型

- `mc_friends_circle_tasks`：企业、创建人、任务名称、发送方式、内容、目标员工、状态、完成/总数、开始/结束时间、外部任务号、失败原因和时间戳。
- `mc_friends_circle_materials`：企业、创建人、素材名称、类型、内容 JSON、状态和时间戳。
- 状态：`draft -> queued -> running -> partially_succeeded | succeeded | failed | cancelled`。
- 所有查询和写入以登录上下文中的 `corp_id` 为边界，不接受客户端覆盖企业 ID。

## HTTP 合同

- `GET /dashboard/friendsCircle/taskIndex`：按 `taskName`、`status` 分页查询任务。
- `GET /dashboard/friendsCircle/materialIndex`：按 `keyword`、`type` 分页查询素材。
- `POST /dashboard/friendsCircle/taskStore`：创建真实草稿任务。
- `POST /dashboard/friendsCircle/materialStore`：创建真实素材记录。
- `POST /dashboard/friendsCircle/publish`：仅调用正式发布适配器；未配置时返回 `503`，不得改变任务为成功。

## 安全与错误边界

- Handler 复用登录、企业选择和 RBAC 校验。
- 草稿创建校验名称、内容、发送方式和状态，不允许客户端写入外部任务号或成功计数。
- 发布先以 `draft -> publishing` 条件更新原子抢占任务，再调用外部适配器；失败进入 `failed`，外部成功但本地确认失败时保留 `publishing` 等待对账，禁止重复外发。本次不在自动化环境执行真实发布。
- Provider 未配置返回 `503`；资源不存在或跨企业隐藏返回 `404`；非法状态流转返回 `422`。

## 前端

朋友圈任务/素材标签分别调用真实查询端点，覆盖加载、空、失败重试、筛选、重置、列表和详情。新增入口创建草稿并刷新列表；正式发布与导出在对应 Provider 缺失时保持禁用。

## 自审

- 未把持久化草稿等同于企业微信发布。
- 覆盖企业隔离、权限、状态、错误和审计边界。
- 不依赖 fixture，不在自动化验收中外发。
- 与 Phase 3.4 已批准的朋友圈任务/素材双标签结构一致。
