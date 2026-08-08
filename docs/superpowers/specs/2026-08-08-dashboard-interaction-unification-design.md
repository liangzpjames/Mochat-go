# Dashboard 交互与视觉统一设计

## 1. 背景

当前 Dashboard manifest 共 53 个页面。2026-08-08 浏览器审计确认：所有页面均可访问并完成至少一次非写交互，但页面分别使用多套页面壳、筛选栏、按钮、空态、分页和弹层实现；部分交互存在数据安全、状态一致性和窄屏可用性问题。

本设计已由用户授权代理自行审阅，不再把文档确认作为开发阻塞点。最终以浏览器可见结果、真实交互和自动化验收为准。

## 2. 目标

1. 统一 53 个 Dashboard 页面的视觉层级和交互规则。
2. 优先修复确认前执行 mutation、密钥明文回填、状态切换无确认等高风险问题。
3. 统一页面标题区、筛选区、操作区、内容卡片、表格、空态、分页、Modal/Drawer 和 Provider 状态。
4. 桌面端保持现有信息架构；390px 窄屏不再出现页面级横向滚动。
5. 保留真实业务接口、URL、权限和数据语义，不用 fixture 或占位页替代业务功能。

## 3. 非目标

- 不重写 Dashboard 路由、鉴权或 API 层。
- 不做新的品牌视觉方案，不替换 React、Ant Design 或现有状态管理方案。
- 不以像素级复刻第三方页面为目标。
- 不修改 MySQL、Redis、应用存储和审计锚点的数据卷。

## 4. 设计原则

### 4.1 页面层级

每页固定采用以下层级：

1. `DashboardPageShell`：面包屑/分组、H1、说明、Provider 或数据源状态、页面主操作。
2. `DashboardFilterPanel`：业务筛选、查询、重置；查询为主按钮，重置为次按钮。
3. `DashboardMetricGrid`：只展示可解释的业务指标。
4. `DashboardContentCard`：表格、图表、配置或工作台内容。
5. `DashboardDataState`：loading、empty、error、forbidden、provider-unavailable。

同一页面不得同时出现两套页面标题、两套主操作或多种不一致的卡片圆角和阴影。

### 4.2 操作层级

- 页面只允许一个主操作，例如“新建角色”或“添加素材”。
- 查询按钮保持主色，重置/刷新使用次级样式，导出使用普通样式。
- 删除、停用、授权撤销必须通过统一 `ConfirmAction`，确认前不得触发 mutation。
- 行级操作的可访问名称必须包含对象，例如“停用 超级管理员”。
- 所有异步操作提供 pending、success、error；失败后保留用户输入。

### 4.3 表单与弹层

- 新建/编辑使用 Ant Design `Modal` 或 `Drawer`，不再用页面底部的伪 `role=dialog` 区块。
- 打开后焦点进入标题或首个字段；Esc/取消关闭；关闭后焦点回到触发按钮。
- 筛选状态和编辑状态必须分离，禁止共用同一 `useState`。
- 日期范围必须满足 `startDate <= endDate`，错误在字段附近展示且不发送请求。
- 员工、部门、标签、阶段和联系人使用业务选择器，不直接要求用户填写内部 ID 或版本号。

### 4.4 空态与 Provider 状态

- 空表仍保留表头和分页位置，空态显示在表体区域。
- Provider 未接入、权限不足、无数据和请求失败必须使用不同状态。
- Provider 状态必须给出原因、恢复条件和可用时的配置入口。
- 相同 Provider 在会话页、报表页和详情页使用同一状态映射。

### 4.5 响应式

- `> 1080px`：固定侧栏和内容区；表格可在卡片内部滚动。
- `769–1080px`：侧栏保持可折叠，指标卡降列，筛选栏换行。
- `<= 768px`：侧栏改为抽屉；内容占满视口；Header 分两行；表单单列。
- 页面根、Header、筛选栏和卡片不得产生横向滚动；宽表只允许表格容器内部滚动。

## 5. 共享组件边界

新增组件：

- `src/components/dashboard-page-shell.tsx`：统一页面标题、说明、状态和主操作。
- `src/components/dashboard-filter-panel.tsx`：统一筛选布局和查询/重置动作。
- `src/components/dashboard-data-state.tsx`：统一数据与 Provider 状态。
- `src/components/dashboard-dialog.tsx`：统一 Modal/Drawer 和焦点规则。
- `src/components/date-range-fields.tsx`：统一日期范围状态与校验。
- `src/components/row-action.tsx`：生成包含对象名称的行级操作。
- `src/components/confirm-action.tsx`：唯一危险操作确认入口。

旧组件逐页迁移后删除重复实现，不一次性重写所有页面业务逻辑。

## 6. 页面族整改顺序

1. 数据安全：风险行为、超时预警、企业密钥、员工/菜单/授权状态切换。
2. 全局框架：搜索、任务中心、企业切换、响应式侧栏。
3. 会话与风险预警：Provider 状态、规则配置、日期筛选。
4. 营销工具：主操作、页签语义、同步反馈、素材范围。
5. SCRM：订单布局、标签密度、公海/线索/商机内部 ID 交互。
6. 数据报表、AI 设置、企业设置：筛选器、弹层、行级操作与本地化。

## 7. 安全要求

- `ConfirmAction` 子按钮不得绑定写 mutation。
- 企业密钥接口不得向编辑表单回传现有明文；编辑时留空表示不修改。
- 停用员工、角色、菜单和授权节点必须显示目标与影响。
- 浏览器验收不提交真实删除、停用、授权或密钥修改。

## 8. 验收标准

1. manifest 53 个路由全部完成桌面与窄屏访问。
2. 每页至少验证首次加载、一次主交互、空态/数据态和错误/受限态。
3. 390×844 下页面根横向溢出为 0；宽表只在表格容器内部滚动。
4. 危险操作取消时请求数为 0，确认时请求数为 1。
5. 企业密钥不回填、不明文显示。
6. 顶部搜索可过滤并键盘跳转；任务中心不存在死链接。
7. `pnpm --filter @mochat/dashboard test`、`lint`、`typecheck`、`build` 通过。
8. 浏览器验收以当前 Docker 真实服务为准，不使用 fixture 证明完成。

## 9. Docker 数据卷约束

禁止执行 `docker compose down -v`、`docker volume rm`、`docker volume prune`。需要更新服务时只构建并重建 `app`：

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml build app
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --no-deps app
```

部署前后必须核对以下卷名不变：

- `mochat-go-desktop_mysql-data`
- `mochat-go-desktop_redis-data`
- `mochat-go-desktop_app-storage`
- `mochat-go-desktop_audit-anchor-storage`
