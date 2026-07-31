# SCRM 机会、跟进与标签闭环设计

## 目标

为 Phase 3.2 补齐机会、跟进时间线和客户标签能力，形成可验证的真实页面与后端业务闭环。新增 `/customer/opportunity` 和 `/customer/tags` 两个 Dashboard 路由；跟进记录嵌入客户或机会详情，不新增独立菜单。

## 业务规则

- 机会必须绑定租户、企业和联系人；金额必须为非负数，预计结束日期不能早于开始日期。
- 机会阶段按合同定义流转；`won` 和 `lost` 是终态，终态机会不可再次改变阶段。
- 进入 `lost` 必须提供非空原因；进入 `won` 不需要原因。
- 跟进记录由服务端写入时间，按创建时间升序展示；已写入记录不可修改或重排，内容不能为空。
- 同一租户、企业范围内标签名称唯一；创建、重命名和批量绑定都必须校验权限和作用域。
- 冲突返回 409，参数或状态不合法返回 422，无权限返回 403；所有查询和写入均校验 tenant/corp scope。

## 接口与数据流

`ScrmApi` 增加：

- `listOpportunities`、`createOpportunity`、`changeOpportunityStage`
- `appendFollowUp`
- `listTags`、`createTag`、`renameTag`、`bindTags`

页面通过 API 客户端调用正式 Dashboard API。机会页负责筛选、创建和阶段操作；标签页负责创建、重命名和批量绑定；详情组件负责展示并追加跟进。写操作使用幂等键和版本字段，服务端以条件更新或唯一约束保证并发安全。

## 实现边界

- 新增 SCRM 领域模型、端口、MySQL 适配器、应用服务、HTTP Handler 和迁移。
- 前端组件保持受控状态，复用现有 PageState、错误映射和查询缓存模式。
- 不扩展订单、报表、AI 或剩余未完成菜单；不为跟进单独新增路由。

## 验证

- 先添加领域、应用、HTTP 和前端 RED 测试，再实现功能。
- 覆盖阶段/日期/负责人筛选、金额和日期校验、赢单/输单终态、输单原因、跟进时间顺序、标签唯一性、批量绑定、409/422 和权限状态。
- 通过 `go test ./internal/modules/scrm/...`、Dashboard SCRM 测试、typecheck、build，并在 `mochat-go-desktop` 保留数据卷的部署上执行迁移和路由健康检查。
