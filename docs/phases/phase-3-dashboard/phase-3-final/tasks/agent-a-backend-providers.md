# 子代理 A 任务书：Phase 3 Final 后端 Provider 与 API（实现，不提交、不部署）

工作目录：`D:\workspace\mochat-go\mochat-go`（git 根目录）。设计契约以 `docs/phases/phase-3-dashboard/phase-3-final/2026-08-07-phase3-final-provider-onboarding-design.zh-CN.md` 为准，本任务书是范围与验收清单。

## 硬性约束

- 禁止 `git commit` / `git push` / `git add`；禁止启动或重建 Docker；禁止修改任何 `web/apps/dashboard/src/features/phase33`、`web/apps/dashboard/src/features/ai-insight` 之外的 React 组件（前端由子代理 B 负责，但 AI 洞察 API 类型定义你可改，若与 B 冲突由主任务仲裁）。
- 只改 Go 后端、`deploy/standalone/docker-compose.yml`（仅新增 env 映射）、`README.md`（仅新增 env 表行）、迁移与迁移相关常量、以及你新建的测试。
- 全部新增 Go 代码必须有对应测试；完成后运行 `go test ./...` 全量必须通过。

## 1. providers 包（新建 `internal/modules/providers`）

实现三类接口与实现：

- `AudioProvider`：`Put(ctx, key, io.Reader, PutOptions) error`、`Open(ctx, key) (io.ReadCloser, int64, error)`、`Delete(ctx, key) error`、`Status() Status`。
  - 本地实现 `internal/modules/providers/audio/local`：root 来自 `MOCHAT_FILE_STORAGE_ROOT`（fallback `./storage/upload/static`）；key 形如 `audio/{corpID}/{yyyy}/{mm}/{uuid}.{ext}`；路径穿越防护（`filepath.Clean` 后必须仍在 root 内）；Put 计算 sha256 并限制 50MB。
- `AIProvider`：`Chat(ctx, ChatRequest) (string, error)`、`Status() Status`。
  - 实现 `internal/modules/providers/ai/openai`：配置 `{BaseURL, APIKey, Model, Timeout}`；`POST {BaseURL}/chat/completions`（`Authorization: Bearer`），body `{model, messages:[{role:"user",content}], temperature:0.3}`，解析 `choices[0].message.content`；非 2xx/超时/JSON 错误均返回含状态的错误；`Status()` 在 Key 为空时 `limited`。
- `WeComArchiveProvider`：`Status() Status`、`Sync(ctx, SyncOptions) (SyncResult, error)`。
  - 实现 `internal/modules/providers/archive/wecom`：四件套（CorpID/Secret/PublicKey/PrivateKey）齐全才 `ready`，否则 `limited` 且 Reason 列出缺失项；`Sync` 在 limited 时返回 `ErrNotConfigured`。
  - 解密基础件：AES-CBC/PKCS7 + RSA 自洽 round-trip 单元测试（自生成密钥，不宣称企微线上兼容）。

## 2. 迁移 `0126_phase3_final_providers`

- 文件：`deploy/standalone/migrations/0126_phase3_final_providers.up.sql` / `.down.sql`，建 `mochat_go_audio_objects` 与 `mochat_go_ai_analysis`（结构见设计稿第 3 节）。
- 同步常量（搜索所有引用并更新，含测试）：
  - `internal/dashboard/saas_admin_system_health.go`：`SaaSAdminExpectedMigrationCount=126`、`SaaSAdminExpectedMigrationVersion="0126_phase3_final_providers"`。
  - `internal/migration/migration_test.go` 所有 latest version 断言 → `0126_phase3_final_providers`。
  - 全仓库 `rg -n "0125_bootstrap_role_remark_cn|SaaSAdminExpectedMigrationCount"` 检查剩余引用。

## 3. 音频 HTTP 模块（新建 `internal/modules/chat-media`）

参考 `internal/modules/scrm` 的分层与 `internal/app/bootstrap` 装配模式；注册：

- `GET /dashboard/chat/media`（`corpId`、`page`、`perPage`、`keyword`；按创建时间倒序，分页）
- `POST /dashboard/chat/media`（multipart `file`，仅音频 MIME，≤50MB；写盘 + 元数据入库；返回新对象）
- `GET /dashboard/chat/media/{id}/content`（JWT + corp 租户匹配 + 权限点 `/chat/file-audio#download`；软删返回 404；响应头含 Content-Type/Content-Length/nosniff/private cache）
- `DELETE /dashboard/chat/media/{id}`（软删：`deleted_at`、`deleted_by`）

依赖复用：PrincipalResolver / LeadAuthorizer 用 `cmd/mochat-go` 已装配的 SCRM 同款（新 bootstrap 函数 `RegisterChatMedia(router, enabled, deps)`，deps 含 DB、FileStorageRoot、PrincipalResolver、LeadAuthorizer）。装配点：`cmd/mochat-go`（仿照 `scrm.go` / `ai_debt_clearance.go` 的注册调用，默认启用）。

响应信封：`{code,msg,data}`；`playUrl` 返回 `/dashboard/chat/media/{id}/content`。

## 4. AI 洞察真实化（扩展现有 `internal/modules/ai-insight`）

- `Module.Dependencies` 增加 `DB *sql.DB` 与 `AIProvider providers.AIProvider`；bootstrap `RegisterAIInsight` 相应扩展（DB 必填，Provider 可空=未配置）。
- Handler：Provider 未配置或 Key 为空 → 现有 limited 目录，但 limitation 文案改为真实原因（未配置 AI Provider / 暂无归档会话数据）。
- Provider 已配置：
  - 从 `mc_work_message_1..10`（表不存在则跳过）取 `corp_id=? AND content_text IS NOT NULL AND deleted_at IS NULL`，按 `msg_data_time DESC` 限 20 条；为空 → limited“暂无归档会话数据”。
  - 5 分钟缓存：`mochat_go_ai_analysis` 存在该 corp+page 的 `status='succeeded'` 且 `created_at` 在 5 分钟内、无 `refresh=1` → 直接回读。
  - 否则按页面专用中文 prompt（会话分析=摘要+关键词；智能分析=意图+建议；情绪=情绪倾向+异常；员工评分=响应时效+沟通质量+得分；沟通关键词=高频词+敏感词+趋势）调用 AI，结果 JSON 落库，返回 `capability:"ready"`、`provider:"dashscope"`、`data:[{sessionId:"archive", summary, keywords, generatedAt}]`。
  - AI 失败/超时 → limited + 错误原因，不落成功行。
- `routes.go` 保持 5 个 GET 路由不变；响应结构保持 `InsightPage` 形状（前端兼容）。

## 5. 配置与文档

- `deploy/standalone/docker-compose.yml` app 服务新增：
  `MOCHAT_GO_AI_PROVIDER_BASE_URL`、`MOCHAT_GO_AI_PROVIDER_KEY`、`MOCHAT_GO_AI_PROVIDER_MODEL`、`MOCHAT_GO_AI_PROVIDER_TIMEOUT_SECONDS`、`MOCHAT_GO_WECOM_ARCHIVE_CORP_ID`、`MOCHAT_GO_WECOM_ARCHIVE_SECRET`、`MOCHAT_GO_WECOM_ARCHIVE_PUBLIC_KEY`、`MOCHAT_GO_WECOM_ARCHIVE_PRIVATE_KEY`（全部 `${VAR:-}` 模式）。
- `README.md` 环境变量表补上述 8 项（中文说明、默认值）。

## 6. 必跑验证与交付证据

- `go test ./...` 全绿（含新测试：audio local 落盘/路径穿越/限流、openai Chat 成功/失败/超时、archive 配置校验 + AES/RSA round-trip + 消息入库回读、chat-media handler 上传/列表/下载/软删、ai-insight ready/limited/缓存）。
- `git diff --check` 干净。
- 完成后报告：改动文件清单、测试数、`go test ./...` 输出摘要、以及你未触碰的目录确认。
