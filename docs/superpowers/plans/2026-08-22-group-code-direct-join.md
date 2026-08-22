# 群活码直连企微客户群 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将群活码从企微“联系我”模式切换为真实“加入群聊”二维码，并以圆弧 AI 风格业务表单替代员工、标签和引导语字段。

**Architecture:** 新增独立 Join Way Provider 客户端，服务层把当前企业的内部 room ID 全量解析成企微 `wx_chat_id` 后再调用 Provider。数据库保留历史字段并新增 Provider 类型及自动建群配置；前端只提交业务字段，保存成功展示接口返回的真实二维码。

**Tech Stack:** Go、MySQL 8、React 18、TypeScript、TanStack Query、Vitest、Testing Library、Docker Compose、企微客户联系 API。

## Global Constraints

- 群活码新建仅使用企微加入群聊接口，渠道活码继续使用联系我接口。
- 最多选择 5 个当前企业已同步且具有非空 `wx_chat_id` 的群聊。
- 客户标签、使用成员、入群引导语和群聊 JSON 不得出现在群活码业务表单。
- 二维码只接受企微真实返回值，Provider 失败不得生成模拟二维码。
- 历史 `contact_way` 记录不得被 SQL 伪迁移为 `join_way`。
- 不修改全局菜单和顶部 banner；Docker 不删除 MySQL、Redis 或命名卷。

---

### Task 1: 增加兼容迁移与存储契约

**Files:**
- Create: `deploy/standalone/migrations/0152_group_code_direct_join.up.sql`
- Create: `deploy/standalone/migrations/0152_group_code_direct_join.down.sql`
- Modify: `deploy/standalone/schema/mochat.sql`
- Modify: `internal/dashboard/work_room_auto_pull.go`
- Modify: `internal/store/mysql.go`
- Test: `internal/store/mysql_work_room_auto_pull_test.go`

**Interfaces:**
- Produces: `WorkRoomAutoPullWrite.AutoCreateRoom bool`、`RoomBaseName string`、`RoomBaseID int`、`ProviderKind string`；`WorkRoomAutoPullRoomWXChatIDs(ctx, corpID, roomIDs)`。

- [x] 先写存储失败测试，要求群 ID 按输入顺序解析为同企业非空 `wx_chat_id`，缺失任一项返回错误。
- [x] 运行 `go test ./internal/store -run 'TestWorkRoomAutoPullRoomWXChatIDs' -count=1`，确认因方法缺失而失败。
- [x] 新增四个兼容字段的 up/down 迁移与基线 schema；历史默认 `contact_way`。
- [x] 实现 room ID 解析、写入新字段和列表/详情读取；新记录旧列写入 `[]`、`[]`、空字符串和 `2`。
- [x] 重跑目标测试，预期通过，并运行相关存储测试。

### Task 2: 实现企微加入群聊 Provider

**Files:**
- Create: `internal/dashboard/work_room_auto_pull_join_way.go`
- Create: `internal/dashboard/work_room_auto_pull_join_way_test.go`
- Modify: `internal/dashboard/work_room_auto_pull.go`

**Interfaces:**
- Produces: `WorkRoomAutoPullJoinWayClient.CreateJoinWay`、`UpdateJoinWay`、`DeleteJoinWay`；请求结构包含 `chat_id_list`、`auto_create_room`、`room_base_name`、`room_base_id`。

- [x] 写四个 HTTP 合约测试，断言 create 后调用 get 取得 `qr_code`，并验证 update/delete endpoint 与 payload。
- [x] 运行 `go test ./internal/dashboard -run 'TestWorkRoomAutoPullJoinWay' -count=1`，确认因客户端不存在而失败。
- [x] 复用 `RoomWelcomeWeComClient` 的 access token 和 `postJSON`，实现 add/get/update/del join way。
- [x] 对空 `config_id`、空 `qr_code` 和企微非零 `errcode` 返回明确错误。
- [x] 重跑目标测试，预期通过。

### Task 3: 切换群活码写接口并固定校验

**Files:**
- Modify: `internal/dashboard/work_room_auto_pull.go`
- Modify: `internal/dashboard/work_room_auto_pull_test.go`
- Modify: `internal/server/dashboard_routes_test.go`

**Interfaces:**
- Consumes: `WorkRoomAutoPullJoinWayClient`、`WorkRoomAutoPullRoomWXChatIDs`。
- Produces: `POST /dashboard/workRoomAutoPull/store` 响应 `{workRoomAutoPullId, qrcodeUrl}`。

- [x] 写失败测试：无 employees/tags/leadingWords 仍可创建；零群、超过 5 群、跨企业群、自动建群缺名称或序号非法均被拒绝。
- [x] 写失败测试：Provider 失败不调用存储创建；数据库失败会调用远端删除补偿。
- [x] 运行目标 Go 测试确认失败原因是旧解析器仍要求旧字段。
- [x] 将创建流程改为校验业务字段、解析群聊、创建远端配置、持久化和失败补偿；保留历史记录读取。
- [x] 重跑 handler 与 route 测试，预期通过。

### Task 4: 重构群活码表单和二维码结果

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`

**Interfaces:**
- Consumes: `GET /workRoom/roomIndex`、`POST /workRoomAutoPull/store`。
- Produces: `{corpId,qrcodeName,rooms,autoCreateRoom,roomBaseName,roomBaseId}` 和真实二维码结果层。

- [x] 写前端失败测试，断言群表单没有“使用成员”“客户标签 ID”“入群引导语”，并存在自动建群开关与条件字段。
- [x] 写失败测试，断言确认两群后提交业务 payload，成功响应中的 `qrcodeUrl` 显示为图片。
- [x] 运行 `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx` 确认失败。
- [x] 将员工查询仅用于渠道模式；群模式采用名称、群聊、自动建群字段，按钮改为“保存并生成二维码”。
- [x] 解析写接口响应并渲染成功结果；失败时保留抽屉和选择状态。
- [x] 重跑目标测试，预期通过。

### Task 5: 完成样式、全量验证和 Docker 浏览器验收

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `docs/superpowers/plans/2026-08-22-group-code-direct-join.md`

**Interfaces:**
- Produces: 圆弧 AI 风格紧凑抽屉、自动建群联动区、二维码结果层和验收记录。

- [x] 先在现有样式契约测试中增加新类名断言并确认失败，再实现条件配置卡和结果层样式。
- [x] 运行 `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1`。
- [x] 运行 dashboard 目标测试、全量测试、typecheck、eslint 和 build，所有命令必须退出码为 0。
- [x] 使用 `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app` 仅重建应用。
- [x] 检查 app、MySQL、Redis 健康以及命名卷仍存在。
- [x] 在 `http://127.0.0.1:18080/acquisition/group-code` 点击完成群聊选择取消/确认、自动建群开关与校验；同时回归渠道活码成员多选。真实二维码成功结果由 Provider HTTP 合约和前端响应测试覆盖，浏览器验收未提交真实企微配置。
- [x] 在常规桌面和 2560×1440 检查布局、控制台和关键请求，把实际结果写入本计划末尾的实施记录。

## 实施记录

### 数据库与兼容

- 新增并应用 `0152_group_code_direct_join`，为 `mc_work_room_auto_pull` 增加 `provider_kind`、`auto_create_room`、`room_base_name`、`room_base_id`；数据库账本已显示 `applied_now`。
- 初始化 schema 的字段位置通过契约测试锁定，确认不会误加到 `mc_channel_code`。
- 为升级前已部署的 `0001_initial_schema` 精确 checksum `03425c87...` 增加兼容别名，未改写已有迁移账本。
- 历史记录默认 `contact_way`；新版记录写入 `join_way`。新版编辑请求在任何数据修改前返回 409，避免误走旧版联系我接口。

### 自动化验证

- `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1`：通过。
- Dashboard 目标测试：`21/21` 通过。
- Dashboard 全量测试：`137` 个测试文件、`779` 项测试全部通过。
- TypeScript typecheck、目标 ESLint、生产 build：退出码均为 `0`。构建仅保留既有的大 chunk 警告，无失败。
- 失败路径覆盖 Provider 失败不落库、数据库失败清理远端、读取二维码失败或空二维码清理远端、企微非零 `errcode`、缺失二维码时前端保留表单。

### Docker 与浏览器验收

- 仅重建 `mochat-go-desktop-app-1`；app、MySQL、Redis 均为 `healthy`。
- `app-storage`、`audit-anchor-storage`、`mysql-data`、`redis-data` 四个命名卷均保留。
- 浏览器完成群聊双栏多选、取消不写回、确认写回、自动建群字段联动与按钮启停；渠道活码连续选择两名成员后显示“已选择 2 人”。
- 默认桌面与 `2560×1440` 均检查通过，抽屉宽度、字段分区、底部按钮和遮罩无溢出；控制台错误/警告为 `0`。
- 为避免在验收环境留下真实企微配置，浏览器未点击最终提交；真实 add/get/del 契约和二维码成功结果分别由 Go HTTP 合约测试与 Vitest 覆盖。

### 未触碰内容

- 未修改或纳入提交的用户工作包括 `docs/PROJECT_PROGRESS.zh-CN.md`、`.workbuddy/`、`scripts/decrypt_debug/`、`scripts/wecom_verify_*.py`、`tmp/`、`web/saas-admin/` 及另一份 `2026-08-22-live-code-selector-json.md`。
