# Phase 3 Dashboard 完整化设计

日期：2026-07-31

上位约束：`docs/phase/phase-3-dashboard/PHASE3_CONSTRAINTS.zh-CN.md`

本设计必须遵守 Phase 3 主要约束。两者冲突时，以 Phase 3 主要约束为准；本文件只定义 Phase 3.2 的范围、架构和阶段验收。

## 1. 目标

Phase 3 采用分阶段交付，但最终完成定义保持不变：`web/apps/dashboard/src/benchmark/manifest.json` 登记的 53 个页面全部升级为真实可用页面，不保留生产 fixture、演示页或占位页。

Phase 3.2 先交付核心可运营 Dashboard，覆盖：

- Dashboard 基础能力和真实完成度治理；
- 数据概览；
- 全局消息；
- 敏感词；
- SCRM 核心链路：线索池、联系人、商机、公海、标签和跟进。

Phase 3.3 至 Phase 3.6 按业务域继续清零其余页面，Phase 3 Final 执行 53 项总验收。

## 2. 当前基线与功能差异

### 2.1 Phase 3 原计划

`docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md` 定义的是完整 SCRM 业务基础：

- SCRM 领域词汇和 API 契约；
- 租户隔离的持久层；
- 数据权限和 SCRM API；
- 联系人和线索页面；
- 商机和跟进流程；
- 业务事件和第一批报表；
- 端到端证据和发布门禁。

当前仓库已有 `0098_scrm_lead_foundation`、线索领域模型、repository、service 和 HTTP handler，但没有完成原计划中的客户生命周期、联系人、商机、公海、跟进和报表闭环。原计划预留的 `0099_scrm_customer_lifecycle` 也不能直接使用，因为 `0099` 已被 `0099_saas_tenant_default_corp` 占用。

### 2.2 Phase 3.1 已完成

Phase 3.1 交付了 Dashboard 对标基础：

- `MoChat AI` 品牌和圆弧风格 shell；
- 默认收缩、按组展开和按权限渲染的菜单；
- 53 个页面的 manifest 与 page registry；
- Dashboard 与 SaaS Admin 会话隔离；
- `/index` 数据概览真实闭环；
- `/chat/v2-all` 全局消息真实闭环；
- 9 个高保真 fixture 演示页；
- 41 个占位页；
- `check:yuanhu-phase1` 验收门禁。

Phase 3.1 的 `documented` 表示页面已登记，不代表业务完成。当前真实完成度为：

| 类型 | 数量 | 说明 |
| --- | ---: | --- |
| 真实页面 | 2 | 数据概览、全局消息 |
| 演示页面 | 9 | 使用固定 fixture，不产生真实业务结果 |
| 占位页面 | 41 | 只有标题、路由和建设中状态 |
| 未完成 P0 | 1 | 敏感词已有后端基础，但未接入新 Dashboard |

### 2.3 Phase 3.2 必须弥合的差异

- 将“路由可达”与“功能完成”分离记录；
- 完成第三个 P0 敏感词页面；
- 把联系人从 fixture 页面升级为真实业务页面；
- 补齐 Phase 3 原计划的 SCRM 客户生命周期；
- 建立页面级 API、权限、数据、测试和验收追踪；
- 把 Phase 3.1 导航验收升级为可验证真实业务结果的门禁。

## 3. 交付策略

采用业务域纵向闭环。每个批次同时交付：

```text
页面档案
→ 领域/数据模型
→ repository 与 migration
→ application service
→ HTTP API
→ React API adapter
→ 页面与交互
→ 权限和审计
→ 单元、集成、E2E 与视觉证据
```

不采用只按 P0/P1/P2 平推的方式，因为同一业务域会被拆散并重复建设接口。也不直接把旧版 168 个 Dashboard API 当作新产品契约；旧能力可以复用，但必须通过明确的兼容适配层满足新页面合同。

### 3.1 Phase 3.2 实施流程

Phase 3.2 的每个业务批次采用 Phase 3 标准实施流程 v2：

```text
范围冻结
→ 页面档案
→ 旧能力复用审计
→ 领域与 API 契约
→ 失败测试基线
→ 纵向闭环实现
→ 真实数据验收
→ 双租户、双企业和多角色验收
→ 性能与异常验收
→ 阶段门禁
→ Manifest 状态升级
→ 阶段报告
```

建议按以下批次执行：

1. Dashboard 完成度治理和共享页面状态；
2. 敏感词真实闭环；
3. SCRM 领域合同与持久层；
4. 线索池与联系人；
5. 公海与负责人/协作人；
6. 商机、阶段与跟进；
7. SCRM 标签；
8. 数据概览与全局消息加固；
9. Phase 3.2 跨域验收和证据收口。

每个批次都必须独立产生测试和可审查提交，不等待全部页面完成后再集中补测试。

## 4. Manifest 与完成度模型

53 个页面继续以运行时 manifest 为唯一菜单和路由事实来源。每个页面增加以下治理字段：

- `domain`：所属业务域；
- `implementation`：`placeholder`、`demo`、`legacy-adapter`、`native`；
- `backend`：`missing`、`partial`、`ready`；
- `acceptance`：`not-started`、`unit-passed`、`integration-passed`、`e2e-passed`；
- `phase`：计划完成阶段；
- `owner`：负责模块；
- `risk`：`low`、`medium`、`high`；
- `legacyRoutes`：可复用旧路由列表；
- `evidence`：页面档案与验收证据位置。

只有同时满足以下条件，页面才能标记为 `native` 或完成：

1. 生产路径没有 fixture 或 placeholder；
2. 核心查询或命令使用真实 Go API；
3. 数据经过租户、企业和数据范围过滤；
4. 写操作有校验、冲突处理和审计；
5. 刷新后业务结果仍存在；
6. 单元、API/集成和浏览器主流程通过；
7. 页面档案、接口合同和验收证据已更新。

manifest 检查器必须拒绝把 `demo` 或 `placeholder` 标记为已完成，也必须拒绝 Phase 3 Final 时仍存在非真实实现。

## 5. Phase 3.2 架构

### 5.1 Dashboard 基础层

保留 Phase 3.1 的 shell、会话隔离、权限菜单和 page registry。新增统一页面状态与业务组件：

- 页面标题、面包屑和操作区；
- 筛选栏与 URL 状态同步；
- 数据表、分页和批量操作；
- 加载、空数据、筛选无结果、403、404、409、422 和可重试错误；
- 抽屉、确认弹窗和表单反馈；
- mutation 防重复提交和成功后的定向缓存失效。

现有 `DemoPage` 只能服务仍未升级的页面，不能被 Phase 3.2 完成页面引用。当前占位页中的乱码必须作为基础层缺陷修复。

### 5.2 数据概览

保留 `/dashboard/corpData/index` 的真实聚合能力和现有 `/index` 页面。Phase 3.2 补充：

- 明确日期边界和租户时区；
- 无权限、空数据和服务错误状态；
- 大数据量查询执行计划验证；
- 两企业隔离测试；
- 浏览器日期筛选和刷新恢复证据。

### 5.3 全局消息

保留 `/dashboard/workMessage/toUsers?view=global` 和详情接口，继续使用真实归档分表。Phase 3.2 补充：

- MariaDB 集成测试；
- 十表 UNION、关键词 LIKE、排序和分页的执行计划验证；
- 不同员工数据范围的可见性测试；
- 群聊身份能力限制的明确提示；
- 搜索、分页、详情、404 和刷新恢复的 E2E。

### 5.4 敏感词

复用现有敏感词存储、handler、配额和定时扫描能力，新增圆弧 Dashboard feature module，完成：

- 词库分页、搜索和分组；
- 新增、移动、启停和删除；
- 分组新增和改名；
- 命中记录分页和消息详情；
- SaaS 配额提示；
- 并发修改冲突；
- 操作审计；
- 租户、企业和角色权限；
- 真实扫描结果与页面查询联通。

页面路由固定为 `/ai-insight/v2/sensitive-word`，不得继续显示 placeholder。

### 5.5 SCRM 客户生命周期

以现有 `0098_scrm_lead_foundation` 和 `internal/modules/scrm` 为起点，扩展以下模型：

- `Lead`：线索；
- `ContactProfile`：联系人；
- `CustomerAssignment`：负责人、协作人与公海状态；
- `Opportunity`：商机；
- `OpportunityStage`：可配置阶段；
- `FollowUp`：不可变跟进事件；
- 标签及资源关联。

状态规则：

- 线索：`new → qualified → converted`，或进入 `discarded`；
- 客户分配：`owned`、`collaborating`、`public_pool`；
- 商机：可配置中间阶段，终态为 `won` 或 `lost`；
- `lost` 必须填写原因；
- 跟进记录不可更新和删除，只能追加；
- 可变资源使用整数 `version` 乐观锁；
- 可重试写操作使用稳定幂等键；
- 公海领取必须原子化，竞争时只能有一个成功者。

正式 API 使用 `/dashboard/scrm` 前缀，覆盖：

- `/leads`；
- `/contacts`；
- `/assignments`；
- `/opportunities`；
- `/stages`；
- `/followUps`；
- `/tags`。

旧 Dashboard API 若能满足需求，通过 adapter 复用；不得让前端直接拼接多套不一致的旧响应。

### 5.6 权限边界

所有 Phase 3.2 资源必须包含 `tenant_id`；企业资源包含 `corp_id`。服务端从认证上下文解析租户和当前企业，不信任客户端提交的租户或负责人范围。

数据权限至少支持：

- 本人；
- 协作人；
- 本部门；
- 下级部门；
- 全企业；
- 全租户仅限明确的平台管理场景。

越权统一返回 `403`，不存在或为避免泄漏而隐藏的资源返回 `404`，版本冲突返回 `409`，非法状态流转返回 `422`。

## 6. Phase 3.2 页面交付

Phase 3.2 完成后至少以下 8 个页面为真实实现：

| 路由 | 页面 | 当前状态 | Phase 3.2 结果 |
| --- | --- | --- | --- |
| `/index` | 数据概览 | 真实 | 加固并验收 |
| `/chat/v2-all` | 全局消息 | 真实 | 加固并验收 |
| `/ai-insight/v2/sensitive-word` | 敏感词 | 占位 | 真实 |
| `/customer/clue/default` | 线索池 | 占位 | 真实 |
| `/customer/contact` | 联系人 | fixture 演示 | 真实 |
| `/customer/opportunity` | 商机 | 占位 | 真实 |
| `/customer/public-sea` | 公海 | 占位 | 真实 |
| `/customer/tags` | 标签 | 占位 | 真实 |

跟进作为联系人和商机详情的共享闭环交付，不为凑页面数量另建无产品依据的菜单。`/customer/group` 在 Phase 3.2 仍可保留演示状态，进入后续阶段。

### 6.1 逐页复用审计要求

每个目标页面在实施前必须形成复用审计记录，至少包含：

- 新菜单路由和旧 Dashboard 路由；
- 可复用的 HTTP API；
- handler、store、数据表和后台任务；
- 当前权限和租户过滤位置；
- 请求与响应字段差异；
- 可直接复用、需要 adapter、需要扩展和必须新增的能力；
- 旧路径兼容策略；
- 性能和数据迁移风险；
- 对应测试与证据位置。

复用审计完成前不得创建同义 API。旧能力不能满足新合同的部分，应通过正式领域 API 或 adapter 补齐，不在 React 页面内拼装临时兼容逻辑。

## 7. 测试与验收

### 7.1 自动化层级

1. manifest 合同测试：53 条路由、阶段、实现和验收状态一致；
2. Go 领域测试：状态流转、版本、幂等和权限规则；
3. repository 测试：查询条件和错误映射；
4. MariaDB 集成测试：租户隔离、乐观锁、公海并发领取、真实消息查询；
5. handler 合同测试：HTTP 方法、参数、响应和错误码；
6. React API 测试：请求序列化和错误映射；
7. React 页面测试：筛选、分页、表单、冲突和空/错状态；
8. Playwright：从登录、菜单进入到真实写入、刷新和再次读取；
9. 固定 `1440x1000` 视觉证据；
10. 构建、类型、lint 和架构边界检查。

### 7.2 Phase 3.2 门禁

新增 `check:phase3-2-dashboard`，至少串行执行：

- manifest 与完成度检查；
- Dashboard typecheck；
- Dashboard 单元测试；
- Dashboard production build；
- SCRM 和敏感词 Go 测试；
- MariaDB 集成测试；
- E2E typecheck；
- Phase 3.2 Playwright；
- 页面证据完整性检查。

Playwright 不能只验证页面可见。至少覆盖以下真实结果：

- 敏感词新增、启停或移动后刷新仍可读取；
- 线索创建并转化为联系人；
- 联系人进入公海后由唯一用户领取；
- 联系人建立商机并推进阶段；
- 追加跟进后时间线刷新仍存在；
- 标签绑定后列表和详情一致；
- 无权限账号不能读取或修改目标资源；
- 第二租户和第二企业看不到第一租户或第一企业的数据。

自动化测试不得发送真实消息、执行真实群发、购买或触发有破坏性的第三方操作。

### 7.3 Phase 3.2 完成标准

- 8 个目标页面全部使用真实数据；
- 敏感词三个 P0 页面目标全部清零；
- SCRM 可以完成线索创建、转化、负责人或协作人维护、进入/领取公海、建立商机、推进阶段、追加跟进和标签维护；
- 两租户、两企业和不同数据权限范围验证通过；
- 核心写操作刷新后可恢复；
- 不存在完成页面仍引用 `DemoPage`、`demo-fixtures` 或 `PlaceholderPage`；
- 新门禁全绿并生成可追溯证据。

## 8. 后续阶段

### Phase 3.3：会话与风控

升级会话域剩余页面：员工会话、客户会话、群聊会话、会话轨迹、会话导出、文件录音、离职员工、拒绝存档、客户继承。

升级风险预警域：风险行为、超时预警、客户流失、消息拦截、关键词库、沉默客户。

### Phase 3.4：营销工具

升级渠道活码、群活码、获客链接、微信客服、活码短链、一键加群、精准群发、朋友圈和素材管理。优先适配已有旧 Dashboard 能力，只为缺失的业务合同新增 API。

### Phase 3.5：SCRM 与数据报表

完成好友、客户群、订单和客户设置；完成客户分析、会话分析、转化分析、行为分析和数据报表。报表指标必须先定义分子、分母、事件时间、租户时区、去重和迟到事件规则。

### Phase 3.6：AI 与企业设置

完成智能分析、情绪识别、员工评分、沟通关键词、AI 知识库、智能体管理、企业信息、员工权限、角色管理、附加权限和授权管理。

AI 页面只有在使用真实模型或明确的生产规则引擎并产生可追溯结果后，才能标记为完成；静态生成内容不算完成。

### Phase 3 Final：53 项清零

- manifest 的 53 个页面全部为 `native` 或经批准的 `legacy-adapter`；
- `placeholder` 和 `demo` 数量均为 0；
- 每个页面至少有一条真实查询或命令主流程；
- 所有写页面验证权限、持久化、审计和异常；
- 每个业务域具有 MariaDB 集成测试和 Playwright 主流程；
- 全量菜单、直接 URL、刷新、前进后退和会话恢复正常；
- Dashboard 不读取 `docs` 作为运行时依赖；
- 全量发布证据可从仓库追溯。

## 9. 非目标与约束

- Phase 3.2 不一次性完成其余 45 个页面；
- 不在 Phase 3.2 实现 AI 功能；
- 不重写已通过验收的数据概览和全局消息，只补缺口；
- 不为追求新接口而复制已有等价后端能力；
- 不把旧版 URL 当作必须永久保留的产品合同，只有实际兼容需求才保留重定向或 adapter；
- 不提交真实访问令牌、客户数据或未脱敏截图；
- 不使用已占用的 `0099` migration 编号。

## 10. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 53 个页面只完成视觉，没有真实功能 | manifest 分离实现和验收状态，Final gate 拒绝 demo/placeholder |
| 旧 API 直接渗透新页面 | feature API adapter 统一响应合同 |
| SCRM 范围过大 | Phase 3.2 只交付客户生命周期主链路，订单、群和报表后置 |
| 跨租户或跨企业泄漏 | service 与 SQL 双层过滤，双租户双企业集成测试 |
| 公海并发重复领取 | 条件更新或行锁加版本校验，真实数据库并发测试 |
| 消息查询在大数据量下退化 | 预发布数据执行计划、索引和分页上限验证 |
| migration 编号冲突 | 从当前最新 migration 顺延，门禁检查唯一编号与生命周期 |
| Phase 3.1 fixture 被误当成完成 | 完成页禁止引用 demo fixture，manifest gate 强制验证 |

## 11. 设计完成定义

本设计完成后，实施计划必须：

- 先完成 Phase 3.2 的 8 个真实页面和共享基础；
- 为每个任务列出精确文件、接口、测试、命令和提交边界；
- 给出 migration 的实际可用编号；
- 把旧能力复用审计作为实现前置步骤；
- 为 Phase 3.3 至 Phase 3 Final 提供逐页面批次清单；
- 保持每个任务可独立测试和审查；
- 不包含 `TBD`、`TODO` 或无法验收的概括性步骤。

实施计划还必须为上述 9 个批次分别给出复用审计、RED、GREEN、集成验证、浏览器验证和提交步骤，不得把测试集中到最后一个任务。
