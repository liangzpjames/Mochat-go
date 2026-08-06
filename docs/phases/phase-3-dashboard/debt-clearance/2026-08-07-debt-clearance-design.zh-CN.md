# Phase 3 坏账清理设计（2026-08-07）

> 状态：验收通过（2026-08-07），见 `2026-08-07-debt-clearance-acceptance.zh-CN.md`
> 依据：`web/apps/dashboard/src/benchmark/manifest.json`（53 页唯一事实源）、`page-registry.tsx`、Phase 3.1–3.5 验收口径

## 1. 背景与结论

53 页基准中 26 页达标（`native`/`legacy-adapter`），剩余 **27 页未达标**（6 `demo` + 21 `placeholder`），分布：

| 领域 | 页数 | 现状 |
| --- | ---: | --- |
| 会话 | 9 | 5 页已有页面组件（员工会话/客户会话/群会话/轨迹/导出），4 页已有业务组件（离职继承/拒绝存档/客户继承等）；`/chat/file-audio` 无音频 Provider |
| 风险预警 | 6 | 页面组件已注册（风险行为/超时/流失/拦截/关键词/沉默客户），后端 Provider 多为 partial/missing |
| AI 洞察 | 5 | `/ai-insight/session-analysis` 仍是 DemoPage；其余 4 页未注册 |
| AI 设置 | 2 | 未注册 |
| 企业设置 | 5 | 未注册 |

**结论**：坏账清理 = 把 27 页推到产品交付口径；其中 `/chat/file-audio` 保持显式未完成（无可验证的音频存储/读取 Provider，沿用 Phase 3.3 结论），其余 26 页必须全部 `native` + `backend ready` + 验收证据闭环。

## 2. 范围分类

### 2.1 已有组件、需产品化与真实数据流（14 页）

| 路由 | 现有组件 | 依赖 |
| --- | --- | --- |
| `/chat/v2-staff` | `EmployeeConversationPage` | 会话归档 Provider |
| `/chat/v2-customer` | `ConversationGlobalPage(fixed=customer)` | 会话归档 Provider |
| `/chat/v2-group` | `ConversationGlobalPage(fixed=room)` | 会话归档 Provider |
| `/chat/trajectory` | `ConversationTrajectoryPage` | 会话归档 Provider |
| `/chat/export` | `ConversationExportPage` | 会话归档 Provider |
| `/chat/resign-staff` | `CustomerTransferPage(mode=resign)` | 企微员工/客户数据 |
| `/chat/refuse-archive` | `RefuseArchivePage` | 会话归档 Provider |
| `/customer/inheritance` | `CustomerTransferPage(mode=inheritance)` | 企微员工/客户数据 |
| `/ai-insight/v2/risk` | `RiskBehaviorPage` | 会话归档/风险规则 |
| `/ai-insight/v2/timeout` | `TimeoutWarningPage` | 会话归档/超时规则 |
| `/ai-insight/v2/customer-loss` | `CustomerLossPage` | 会话归档 |
| `/ai-insight/v2/message-intercept` | `MessageInterceptPage` | 消息拦截规则 |
| `/ai-insight/v2/keyword-library` | `KeywordLibraryPage` | 关键词库 |
| `/ai-insight/v2/silent-customer` | `SilentCustomerPage` | 会话归档 |

### 2.2 未注册、需新建（11 页）

| 路由 | 说明 |
| --- | --- |
| `/ai-insight/session-analysis` | 替换 DemoPage 为真实页面 |
| `/ai-insight/smart-analysis` | 新建（AI 能力受限态） |
| `/ai-insight/emotion` | 新建（AI 能力受限态） |
| `/ai-insight/employee-score` | 新建（AI 能力受限态） |
| `/ai-insight/communication-keyword` | 新建（AI 能力受限态） |
| `/ai-setting/ai-knowledge-base` | 新建知识库 CRUD |
| `/ai-setting/agent` | 新建智能体 CRUD |
| `/company-setting/website` | 新建企业信息 CRUD |
| `/company-setting/staff` | 新建员工权限管理 |
| `/setting/role` | 新建角色管理 |
| `/setting/additional` | 新建附加权限 |
| `/setting/authorization` | 新建授权管理 |

### 2.3 显式保持未完成（1 页）

- `/chat/file-audio`：无可验证的音频存储/读取 Provider，保持 `placeholder`，不升级状态。

## 3. 分批执行计划

### 批次 0：企业设置 + AI 设置（7 页，先做，内部 CRUD 不依赖外部 Provider）

复用现有组织/权限模型（`mc_user`、角色、授权、菜单）与新增最小表：
- `/company-setting/website`：企业信息展示/编辑。
- `/company-setting/staff`：员工列表、启用/停用、角色分配。
- `/setting/role`：角色 CRUD + 菜单/权限勾选。
- `/setting/additional`：附加权限维护。
- `/setting/authorization`：授权管理（应用/能力开关）。
- `/ai-setting/ai-knowledge-base`：知识库 CRUD（名称/描述/文档数/关联智能体）。
- `/ai-setting/agent`：智能体 CRUD（名称/描述/关联知识库）。

### 批次 1：AI 洞察（5 页）

- `/ai-insight/session-analysis`：替换 DemoPage，用真实会话分析任务/结果合同；Provider 缺时结构化 `limitations`。
- 其余 4 页：真实页面骨架 + 后端合同；AI/存档能力缺失时展示受限态，不得伪造 0。

### 批次 2：风险预警（6 页）

- 对现有 phase33 页面逐页复用审计：确认表/API/Provider/权限；补齐 Go 合同与真实数据流（超时规则、拦截规则、关键词库、沉默客户、流失、风险行为）。
- 事件写入走真实业务事件（如消息拦截命中、超时触发），页面回读真实数据。

### 批次 3：会话（9 页，`/chat/file-audio` 除外）

- 对现有 conversation-global 页面复用审计：统一 `/workMessage/*` 或既有归档合同的 Go 侧实现；员工/客户/群会话、轨迹、导出、离职/客户继承、拒绝存档逐页验证。
- 无归档 Provider 时以 `limitations` 呈现，不伪造数据。

## 4. 统一要求（每页同 Phase 3.5 门禁）

1. **实现**：`native`，复用现有 MoChat 风格组件/样式；禁止直出内部 key。
2. **契约**：Go 侧真实合同，tenant/corp/员工数据范围限制；写操作有权限、校验、幂等/版本与审计。
3. **测试**：失败测试先行（RED→GREEN）；每页至少加载/数据/空/错误/受限状态测试；`go test ./...`、Dashboard Vitest、typecheck、build 全绿。
4. **manifest**：`implementation=native`、`backend=ready`、`acceptance=integration-passed` 或 `e2e-passed`，并更新证据字段。
5. **浏览器验收**：逐页截图 + 第三方视觉模型严格审阅（只挑毛病）；关键跨页工作流（角色→菜单过滤、离职→客户继承、拦截/超时→风险页回读）必须真实跑通。
6. **禁止项**：fixture/demo/placeholder/静态成功/仅路由可达不算完成；Provider 缺失以 `limitations` 呈现。

## 5. 职责划分

- **主任务（本会话）**：设计审阅、验收门禁复核、浏览器 + 视觉模型验收、manifest 与文档同步、最终提交。
- **开发会话**：按本设计分批实现代码与测试；**禁止**重建/部署 Docker、**禁止**提交 commit、**禁止**删除数据卷；每批完成后回报文件清单与测试结果。

## 6. 验收数据与清理

- 允许创建受控验收数据（沿用 `P35-ACCEPT-` 风格，新前缀 `P36-ACCEPT-`），tenant/corp 隔离，提供只命中前缀的清理步骤。
- 截图与证据输出到 `D:\workspace\mochat-go\output\`。

## 7. 完成定义

53 页中 52 页达标 + `/chat/file-audio` 显式未完成（Provider 阻塞），且全量门禁与浏览器/视觉验收证据闭合后，Phase 3 Final 可进入总验收。

## 8. 主任务审阅结论（2026-08-07）

### 8.1 事实核对结果

- manifest 现状与设计一致：26 页待实现（6 `demo` + 20 `placeholder`，另 1 页 `/chat/file-audio` 阻塞）。
- **重要修正**：Phase 3.3 代码领先 manifest——关键词库、消息拦截、沉默客户、拒绝存档的 Provider/版本发布/页面组件已完成（见 `phase-3.3/reports/phase3.3-function-matrix.md`），但 manifest 仍为 `placeholder/missing`。因此风险预警与会话两批以“复用审计 + 补齐 Go 合同 + 真实数据流 + 同步 manifest”为主，不是从零新建。
- 前端 14 页组件已注册（`conversationGlobalApi` / `businessWorkbenchApi` 注入），12 页未注册。
- 企业设置所需旧后端 CRUD 已存在（`/dashboard/user/*`、`/dashboard/role/*`、`/dashboard/menu/*`、`/dashboard/corp/*`），且 React 侧已有对应 API/页面（`features/role`、`features/user-admin`、`features/menu-admin`、`features/corp`）可复用。

### 8.2 架构决策

1. **新后端一律走新模块模式**（`internal/modules/<module>` + `internal/app/bootstrap/Register<MODULE>`，参照 `scrm`），禁止新增 `internal/dashboard` 处理器。
2. 新建模块：`internal/modules/ai-settings`（AI 知识库、智能体 CRUD）、`internal/modules/ai-insight`（5 页真实合同 + 能力受限态）。企业设置复用既有旧接口，仅在缺口处按新模块模式补充。
3. 会话/风险：完成并验证既有接口（`work_message` 存档、超时预警、拦截/关键词库等），Provider 缺失时页面呈现结构化 `limitations`，禁止伪造 0/空成功。
4. 迁移编号预分配（up/down 成对，命名风格参照 `0123_phase35_order_collation_align`）：Agent A 独占 `0124–0126`；Agent B 独占 `0127–0129`。

### 8.3 文件归属（防冲突）

- **Agent A（企业设置 5 + AI 设置 2 + AI 洞察 5）**：`internal/modules/ai-settings/**`、`internal/modules/ai-insight/**`、`web/apps/dashboard/src/features/ai-settings/**`、`features/ai-insight/**`、`features/company-settings/**`、`page-registry.tsx`、`main.tsx`（新增 API 注入）、迁移 `0124–0126`。
- **Agent B（会话 8 + 风险预警 6）**：`web/apps/dashboard/src/features/phase33/**`、`features/conversation-global/**`、相关 `internal/store` / `internal/dashboard` 既有处理器、迁移 `0127–0129`。
- **仅主任务**：`manifest.json`、`compat_manifest_embedded.json`、全量门禁、浏览器/视觉验收、文档同步、最终提交。

### 8.4 前端规范

- 新页面统一沿用 Phase 3.5 卡片体系（`phase35-*` 类）与线索池按钮风格（主按钮 `#536bf4` 白字、8px 圆角；次按钮白底 `#59647a` 字；小号翻页按钮）。
- 禁止直出内部 key/UUID；无数据与能力缺失必须显式呈现，不得伪装成功。
- 每页至少覆盖：加载/数据/空/错误/受限 5 态（Vitest），Go 侧覆盖权限、校验、审计。

### 8.5 必须真实跑通的数据流工作流（验收时逐条回读）

1. 角色管理 → 勾选菜单权限 → 员工分配角色 → 登录后菜单过滤生效；
2. 企业信息编辑 → 回读；知识库/智能体 CRUD → 关联回读；
3. 关键词库 → 版本发布 → 消息拦截命中 → 风险行为/消息拦截页回读；
4. 超时规则 → 超时事件 → 超时预警页回读；
5. 员工/客户/群会话、轨迹、导出 → 归档合同（无 Provider 时 `limitations`）；
6. 离职 → 客户继承 → 结果回读。

### 8.6 验收数据与门禁顺序

- 验收数据前缀 `P36-ACCEPT-`，tenant/corp 隔离，验收后提供只命中前缀的清理 SQL。
- 门禁顺序（主任务执行，开发会话只跑定向测试）：`go test ./...` → dashboard typecheck/test/build → `pnpm check:phase3-5-dashboard` → `check_debt_clearance.mjs` → 浏览器逐页截图 + 视觉模型严格审阅 → manifest/文档同步。
