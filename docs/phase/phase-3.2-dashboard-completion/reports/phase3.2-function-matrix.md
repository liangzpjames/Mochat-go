# Phase 3.2 功能矩阵

本矩阵仅覆盖 Phase 3.2 的八个指定页面。每行是一项可独立验收的参考功能；`已对应` 仅表示已建立目标实现映射，不代表该项已完成。`待`、`TODO`、`TBD`、`unfinished`、`not started` 等状态会被完成门禁视为未闭合，不能支持 `e2e-passed`。

除十列原始合同外，`decisionReason`、`alternativeEntry`、`decisionVerification` 是“合理合并”与“不适用”的强制记录：分别说明原因、可进入的替代入口、验证方法。`tests` 与 `evidence` 在一行全部闭合时必须是存在的仓库相对文件路径；本矩阵没有把后续真实业务证据提前标为闭合。

| page | referenceFeature | decision | decisionReason | alternativeEntry | decisionVerification | mochatEntry | frontend | api | permission | persistence | tests | evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| /index | 关键经营指标与趋势 | 已对应 | - | - | - | 数据概览 | web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx | internal/dashboard/corp_data.go | internal/dashboard/corp_data.go（认证企业、RBAC 数据范围交集） | internal/store/mysql.go（企业日汇总） | internal/dashboard/corp_data_test.go | 待 Task 11：浏览器验收证据（本任务无可用本地登录凭据） |
| /chat/v2-all | 会话列表检索与详情 | 已对应 | - | - | - | 全局消息 | 待 Task 2：会话列表与详情抽屉 | 待 Task 3：会话检索接口 | 待 Task 3：会话读取权限 | 待 Task 3：会话与消息数据 | 待 Task 10：会话交互测试 | 待 Task 11：浏览器验收证据 |
| /ai-insight/v2/sensitive-word | 敏感词规则列表与维护 | 已对应 | - | - | - | 敏感词 | 待 Task 2：敏感词规则管理 | 待 Task 3：敏感词规则接口 | 待 Task 3：风险配置权限 | 待 Task 3：敏感词规则数据 | 待 Task 10：敏感词交互测试 | 待 Task 11：浏览器验收证据 |
| /customer/clue/default | 线索列表检索与分配 | 已对应 | - | - | - | 线索池 | 待 Task 4：线索列表与分配操作 | 待 Task 5：线索检索与分配接口 | 待 Task 5：线索读取与分配权限 | 待 Task 5：线索数据 | 待 Task 10：线索交互测试 | 待 Task 11：浏览器验收证据 |
| /customer/contact | 客户档案与跟进记录 | 已对应 | - | - | - | 客户管理 | 待 Task 4：客户档案与跟进记录 | 待 Task 5：客户详情与跟进接口 | 待 Task 5：客户读取权限 | 待 Task 5：客户与跟进数据 | 待 Task 10：客户交互测试 | 待 Task 11：浏览器验收证据 |
| /customer/opportunity | 商机看板与阶段推进 | 已对应 | - | - | - | 商机管理 | 待 Task 4：商机看板与阶段操作 | 待 Task 6：商机查询与更新接口 | 待 Task 6：商机读取与更新权限 | 待 Task 6：商机与阶段数据 | 待 Task 10：商机交互测试 | 待 Task 11：浏览器验收证据 |
| /customer/public-sea | 公海筛选与领取 | 已对应 | - | - | - | 公海客户 | 待 Task 4：公海筛选与领取操作 | 待 Task 6：公海查询与领取接口 | 待 Task 6：公海读取与领取权限 | 待 Task 6：公海客户数据 | 待 Task 10：公海交互测试 | 待 Task 11：浏览器验收证据 |
| /customer/tags | 客户标签维护与关联 | 已对应 | - | - | - | 客户标签 | 待 Task 4：标签维护与关联操作 | 待 Task 6：标签查询与维护接口 | 待 Task 6：标签读取与维护权限 | 待 Task 6：标签与关联数据 | 待 Task 10：标签交互测试 | 待 Task 11：浏览器验收证据 |
