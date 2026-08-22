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

- [ ] 先写存储失败测试，要求群 ID 按输入顺序解析为同企业非空 `wx_chat_id`，缺失任一项返回错误。
- [ ] 运行 `go test ./internal/store -run 'TestWorkRoomAutoPullRoomWXChatIDs' -count=1`，确认因方法缺失而失败。
- [ ] 新增四个兼容字段的 up/down 迁移与基线 schema；历史默认 `contact_way`。
- [ ] 实现 room ID 解析、写入新字段和列表/详情读取；新记录旧列写入 `[]`、`[]`、空字符串和 `2`。
- [ ] 重跑目标测试，预期通过，并运行相关存储测试。

### Task 2: 实现企微加入群聊 Provider

**Files:**
- Create: `internal/dashboard/work_room_auto_pull_join_way.go`
- Create: `internal/dashboard/work_room_auto_pull_join_way_test.go`
- Modify: `internal/dashboard/work_room_auto_pull.go`

**Interfaces:**
- Produces: `WorkRoomAutoPullJoinWayClient.CreateJoinWay`、`UpdateJoinWay`、`DeleteJoinWay`；请求结构包含 `chat_id_list`、`auto_create_room`、`room_base_name`、`room_base_id`。

- [ ] 写四个 HTTP 合约测试，断言 create 后调用 get 取得 `qr_code`，并验证 update/delete endpoint 与 payload。
- [ ] 运行 `go test ./internal/dashboard -run 'TestWorkRoomAutoPullJoinWay' -count=1`，确认因客户端不存在而失败。
- [ ] 复用 `RoomWelcomeWeComClient` 的 access token 和 `postJSON`，实现 add/get/update/del join way。
- [ ] 对空 `config_id`、空 `qr_code` 和企微非零 `errcode` 返回明确错误。
- [ ] 重跑目标测试，预期通过。

### Task 3: 切换群活码写接口并固定校验

**Files:**
- Modify: `internal/dashboard/work_room_auto_pull.go`
- Modify: `internal/dashboard/work_room_auto_pull_test.go`
- Modify: `internal/server/dashboard_routes_test.go`

**Interfaces:**
- Consumes: `WorkRoomAutoPullJoinWayClient`、`WorkRoomAutoPullRoomWXChatIDs`。
- Produces: `POST /dashboard/workRoomAutoPull/store` 响应 `{workRoomAutoPullId, qrcodeUrl}`。

- [ ] 写失败测试：无 employees/tags/leadingWords 仍可创建；零群、超过 5 群、跨企业群、自动建群缺名称或序号非法均被拒绝。
- [ ] 写失败测试：Provider 失败不调用存储创建；数据库失败会调用远端删除补偿。
- [ ] 运行目标 Go 测试确认失败原因是旧解析器仍要求旧字段。
- [ ] 将创建流程改为校验业务字段、解析群聊、创建远端配置、持久化和失败补偿；保留历史记录读取。
- [ ] 重跑 handler 与 route 测试，预期通过。

### Task 4: 重构群活码表单和二维码结果

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`

**Interfaces:**
- Consumes: `GET /workRoom/roomIndex`、`POST /workRoomAutoPull/store`。
- Produces: `{corpId,qrcodeName,rooms,autoCreateRoom,roomBaseName,roomBaseId}` 和真实二维码结果层。

- [ ] 写前端失败测试，断言群表单没有“使用成员”“客户标签 ID”“入群引导语”，并存在自动建群开关与条件字段。
- [ ] 写失败测试，断言确认两群后提交业务 payload，成功响应中的 `qrcodeUrl` 显示为图片。
- [ ] 运行 `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx` 确认失败。
- [ ] 将员工查询仅用于渠道模式；群模式采用名称、群聊、自动建群字段，按钮改为“保存并生成二维码”。
- [ ] 解析写接口响应并渲染成功结果；失败时保留抽屉和选择状态。
- [ ] 重跑目标测试，预期通过。

### Task 5: 完成样式、全量验证和 Docker 浏览器验收

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `docs/superpowers/plans/2026-08-22-group-code-direct-join.md`

**Interfaces:**
- Produces: 圆弧 AI 风格紧凑抽屉、自动建群联动区、二维码结果层和验收记录。

- [ ] 先在现有样式契约测试中增加新类名断言并确认失败，再实现条件配置卡和结果层样式。
- [ ] 运行 `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1`。
- [ ] 运行 dashboard 目标测试、全量测试、typecheck、eslint 和 build，所有命令必须退出码为 0。
- [ ] 使用 `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app` 仅重建应用。
- [ ] 检查 app、MySQL、Redis 健康以及命名卷仍存在。
- [ ] 在 `http://127.0.0.1:18080/acquisition/group-code` 点击完成群聊选择取消/确认、自动建群开关与校验、失败保留和可用环境下的二维码成功反馈；同时回归渠道活码成员多选。
- [ ] 在常规桌面和 2560×1440 检查布局、控制台和关键请求，把实际结果写入本计划末尾的实施记录。

## 实施记录

实施完成后在此记录实际迁移、测试命令与结果、浏览器尺寸、容器健康、企微能力缺口及未触碰的用户改动。
