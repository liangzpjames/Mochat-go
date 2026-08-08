# Dashboard 53 页交互与视觉统一验收报告

日期：2026-08-08

真实服务：`http://127.0.0.1:18080`

结论：**Task 1–8 全部完成。manifest 53 个页面均在当前 Docker 真实服务上通过桌面端与 390px 移动端交互验收。**

## 1. 任务与提交

| Task | 提交 | 状态 |
|---|---|---|
| Task 1 统一交互基础组件 | `4a604f1 feat(dashboard): add unified interaction primitives` | 完成 |
| Task 2 页面壳、搜索与企业切换 | `a3c91ee feat(dashboard): unify shell search and corp switching` | 完成 |
| Task 3 响应式 Drawer 布局 | `6779369 feat(dashboard): add responsive drawer layout` | 完成 |
| Task 4 P0 高风险操作确认 | `46b88ed fix(dashboard): confirm privileged mutations` | 完成 |
| Task 5 会话与营销工具交互统一 | `f87decf feat(dashboard): unify conversation and marketing interactions` | 完成 |
| Task 6 SCRM 业务流程统一 | `9818625 feat(dashboard): unify scrm business workflows` | 完成 |
| Task 7 报表与设置弹层统一 | `6ab7147 feat(dashboard): unify reports and settings dialogs` | 完成 |
| Task 8 真实服务 53 页验收 | 本提交 | 完成 |

Task 8 保留真实业务 API、URL、权限与 Provider 语义；验收脚本不注入 fixture、不拦截路由、不以路由可达或空态代替业务界面证明。

## 2. TDD 与修复记录

Task 8 的实现均先取得失败证据，再实施最小修复：

1. 首次规范加载因 Node JSON import attribute 失败；修正 manifest 导入方式后收集到 53 个用例。
2. `/index` 首次渲染时业务面尚未异步出现；用例改为等待可见业务界面，并要求每个视口至少收到一个页面级真实 `/dashboard/*` 业务响应。
3. `/chat/resign-staff` 的真实空结果页不满足旧表格选择器；统一为可见业务卡片、表格、表单、区域或 PageState，且仍要求真实 API 响应和安全交互。
4. `/ai-insight/v2/sensitive-word`、`/data/report` 的 manifest 标题与实际页面标题不一致；分别采用包含式业务标题断言，并将报表 manifest 标题修正为“综合报表”。
5. `/setting/authorization` 截图暴露 `Friends circle ...` 与 `??? Provider ??`；先新增失败组件测试，再映射为中文业务名称，未知项回退为 `未命名权限（ID）`。
6. 全 E2E lint 首次失败于既有 `phase3-2-dashboard.spec.ts` 的无效 `async` helper；移除无意义异步并更新统一界面后的精确定位器、新版报表 API 与业务选项 fixture。
7. Phase 3.2 `/index` 回归暴露 React 状态 updater 延后读取 `event.currentTarget`，导致多选框事件对象失效；先由 E2E 复现，再同步提取员工/部门选择值，最终 Phase 3.2 为 `8/8`。
8. 首版验收断言会把登录、企业选择、权限菜单等壳层请求以及 404 计为业务响应，且回退交互可能命中全局控件；加强断言后 `/chat/v2-all` 因真实权限响应 403 先失败，再将口径收紧为：排除壳层接口，拒绝 401、404 与 5xx，只接受 2xx/3xx 及如实的 403/409/422 业务状态，交互必须位于 `.dashboard-content`。
9. 复审继续指出“PageState + 聚焦控件”不能证明页面主交互；将聚焦回退改为失败后，`/ai-insight/v2/sensitive-word` 取得 RED。随后要求刷新/查询产生业务响应或筛选状态变化，并覆盖分段按钮、Tab、原生选择器、折叠项和安全内部导航。`/setting/authorization` 进一步真实打开危险操作 `Popconfirm`、点击“取消”并断言授权状态未变化；可见但 disabled 的查询/刷新按钮也必须进入统一回退链。最终加强版再次通过 53/53。

P0 防护继续由单元测试覆盖：确认前不 mutation、企业密钥不明文回填、状态切换与删除必须确认。

## 3. 串行质量门结果

| 命令 | 最终结果 |
|---|---|
| `pnpm --filter @mochat/dashboard lint` | PASS |
| `pnpm --filter @mochat/dashboard typecheck` | PASS |
| `pnpm --filter @mochat/dashboard build` | PASS，Vite 生产构建完成 |
| `pnpm --filter @mochat/dashboard test` | PASS，89 个测试文件、554/554 测试 |
| `pnpm --filter @mochat/e2e lint` | PASS |
| `pnpm --filter @mochat/e2e typecheck` | PASS |
| `pnpm --dir web/e2e exec playwright test tests/phase3-2-dashboard.spec.ts --project=chromium --workers=1` | PASS，8/8 |
| 53 页真实服务 Playwright 命令 | PASS，53/53 |
| `git diff --cached --check` | PASS |

Vitest 期间 Ant Design 在 jsdom 中会输出 `window.getComputedStyle(elt, pseudoElt)` 未实现提示；该提示未造成失败，最终退出码为 0。

## 4. 53 页真实浏览器结果

执行命令：

```powershell
$env:MOCHAT_E2E_LIVE_BASE='http://127.0.0.1:18080'
$env:PHASE35_ACCEPT_PHONE='13800000000'
$env:PHASE35_ACCEPT_PASSWORD='MochatLocal@123'
pnpm --dir web/e2e exec playwright test tests/dashboard-interaction-unification.spec.ts --project=chromium --workers=1
```

| 项目 | 结果 |
|---|---|
| manifest 用例收集 | 53/53 |
| 桌面 1440×1000 | 53/53 |
| 移动端 390×844 | 53/53 |
| 总 Playwright 用例 | 53/53，`53 passed (2.2m)` |
| 截图 | 106 张，9,955,470 字节 |
| 页面级横向溢出 | 0 |
| `pageerror` | 0 |
| 每视口页面级真实 Dashboard 业务响应 | 53/53 |

每个用例均执行桌面和移动两个视口，验证页面标题、统一内容壳、真实业务/Provider 界面、页面级真实 `/dashboard/*` 业务响应、运行时错误与文档级横向溢出。页面主交互必须在 `.dashboard-content` 内产生可观察结果：刷新/查询完成业务请求，或筛选值、分段按钮、Tab、折叠状态、内部导航发生可断言变化；危险操作只打开确认并取消，且断言状态不变。业务响应断言排除登录、企业选择、权限菜单等壳层接口；接受 2xx/3xx 和如实反映权限或业务约束的 403/409/422，拒绝 401、404 与 5xx。最终截图位于：

`web/e2e/artifacts/dashboard-interaction-unification/`

人工复核了数据概览桌面/移动、客户订单移动、授权管理移动等代表性截图；授权管理的英文权限名和异常 Provider 名已消除，390px 页面无横向溢出。

## 5. Docker、登录与卷证据

最终构建与部署命令均显式设置 JWT secret，且只操作 `app`：

```powershell
$env:MOCHAT_SIMPLE_JWT_SECRET='mochat-go-docker-desktop-local-secret'
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml build app
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --no-deps app
```

最终状态：

| 服务 | 完整容器 ID | 健康状态 |
|---|---|---|
| app | `5934e40470f3c26c3c510a134577405793cf0fe1745836406c4c7c48820b2a94` | healthy |
| mysql | `9d8498d7c1ea867ebbeaebf584407037091ec8297d958c2e35c918d63a2aaae7` | healthy，验收期间 ID 未变化 |
| redis | `791f65768a4b70bc130290c2f71ea06cc1b8c563b7cababab3a42c58d52af077` | healthy，验收期间 ID 未变化 |

- `GET /readyz`：HTTP 200。
- `POST /dashboard/user/auth`：HTTP 200，返回 token。
- app 容器环境：`MOCHAT_SIMPLE_JWT_SECRET=mochat-go-docker-desktop-local-secret`，未回落到 `change-me-standalone-secret`。
- 指定的四个数据卷均保持存在：
  - `mochat-go-desktop_mysql-data`
  - `mochat-go-desktop_redis-data`
  - `mochat-go-desktop_app-storage`
  - `mochat-go-desktop_audit-anchor-storage`

现场偏差说明：第一次在仓库根目录仅使用 `-f` 执行 `up` 时，Compose 将项目名推断为 `standalone`，因 18080 端口被现有服务占用而失败；没有替换 `mochat-go-desktop` 服务，也没有挂载或写入其四个现有卷。该命令自动创建了一个未启动的 `standalone-app-1` 容器和四个空卷：`standalone_mysql-data`、`standalone_redis-data`、`standalone_app-storage`、`standalone_audit-anchor-storage`。依据禁止删除卷的约束，本次未清理这些额外对象；随后所有 Compose 命令均显式使用 `-p mochat-go-desktop`。

## 6. 明确未执行的真实破坏性动作

本次明确未执行：

- `ResetData`、`mochat-bootstrap` 或任何账号/数据重置；
- `docker compose down -v` / `down --volumes`；
- `docker volume rm`、`docker volume prune`；
- 重建、替换、停止或清空 MySQL、Redis；
- 删除、迁移或清空任何现有数据卷；
- 页面中的真实删除、停用、授权、发布、状态流转等破坏性业务动作；
- fixture 注入、路由替身或绕过真实权限的 53 页验收；
- `git reset`、`git clean` 或覆盖用户原有未提交文件。

## 7. 剩余风险

- 部分真实业务范围当前数据量为 0，验收仍要求并取得真实 API 响应、可见业务/Provider 状态和安全交互，不将空态本身作为完成证明。
- 外部企业微信、会话归档、朋友圈发布和 AI Provider 缺少真实外部凭据时继续如实展示受限状态；本任务没有伪造外部成功。
- `standalone_*` 四个空卷及未启动容器属于本次 Compose 项目名误判产生的可清理对象，但按本任务禁令保留，需由后续获得明确清理授权的操作单独处理。
