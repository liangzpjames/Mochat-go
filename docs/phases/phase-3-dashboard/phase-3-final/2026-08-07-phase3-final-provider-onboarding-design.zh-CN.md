# Phase 3 Final：Provider 接入与总验收设计（2026-08-07，定稿）

> 状态：设计已审阅定稿（主任务审阅/验收，开发子代理实现）
> 前置：坏账清理已完成（52/53 达标），唯一阻塞 `/chat/file-audio` 本轮解锁后目标 53/53。

## 1. 目标与非目标

目标：让 53 页基准获得真实数据闭环并完成总验收。

1. 本地音频存储/读取 Provider + `/chat/file-audio` 真实页面（上传→落盘→回读→播放→软删）。
2. AI Provider（阿里云百炼 OpenAI 兼容协议）+ AI 洞察 5 页真实分析流（取归档文本→AI 摘要/关键词→落库→页面回读）。
3. 企微会话存档 Provider 适配层（无真实凭证时注册为 `limited`，提供契约测试；真实凭证由用户后续提供后激活）。
4. 53 页总验收：门禁全绿 + 浏览器/识图验收 + 跨页数据流证据 + 文档收口。

非目标：不实现真实企微 getchatdata 线上解密（无凭证无法验收）；不接 S3；不做音频时长解析（`duration_seconds` 预留字段，本期可为 0）。

## 2. 架构决策

- 新增 `internal/modules/providers`：`AudioProvider`、`AIProvider`、`WeComArchiveProvider` 三类接口 + 实现。
- 音频本地实现：写 `{FileStorageRoot}/audio/{corpID}/{yyyy}/{mm}/{uuid}.{ext}`，元数据落 MySQL 新表 `mochat_go_audio_objects`；下载走鉴权接口（复用 Dashboard JWT + 租户/企业校验），返回带 `Content-Type`/`Content-Length` 的字节流。
- AI 实现：`POST {baseURL}/chat/completions`，`Authorization: Bearer`，解析 `choices[0].message.content`；结果落 `mochat_go_ai_analysis`，5 分钟缓存，`refresh=1` 强制重跑。
- AI 洞察现状：后端 `pageCatalog` 静态受限目录，前端 `AiInsightPage` 已支持 `ready` 态渲染 `data[]`（字段 `sessionId`/`summary`），保持该响应形状，前端改动最小。
- `/chat/file-audio` 现状：`phase33-operations-page.tsx` 中 `providerState:'unavailable'` 通用受限页；替换为独立 `file-audio-page.tsx`。
- manifest 现状：`/chat/file-audio` = `placeholder/missing/not-started`；完成后更新为 `native/ready/integration-passed`，`check_debt_clearance.mjs` 的 `allowedIncomplete` 清空。
- 模块装配：沿用 `internal/app/bootstrap` + `cmd/mochat-go` 注册模式；音频路由新注册 `/dashboard/chat/media*`，AI 洞察扩展现有模块依赖（DB + AIProvider）。

## 3. 数据库迁移（单个 `0126_phase3_final_providers`，up/down 成对）

`mochat_go_audio_objects`：

```sql
CREATE TABLE IF NOT EXISTS `mochat_go_audio_objects` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT 0,
  `user_id` int(10) unsigned NOT NULL DEFAULT 0,
  `employee_id` int(10) unsigned NOT NULL DEFAULT 0,
  `corp_id` int(10) unsigned NOT NULL DEFAULT 0,
  `original_name` varchar(255) NOT NULL DEFAULT '',
  `relative_path` varchar(512) NOT NULL DEFAULT '',
  `content_type` varchar(128) NOT NULL DEFAULT '',
  `size_bytes` bigint(20) unsigned NOT NULL DEFAULT 0,
  `duration_seconds` int(10) unsigned NOT NULL DEFAULT 0,
  `sha256` char(64) NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `deleted_by` int(10) unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_audio_objects_corp_created` (`corp_id`, `deleted_at`, `created_at`),
  KEY `idx_audio_objects_path` (`relative_path`(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

`mochat_go_ai_analysis`：

```sql
CREATE TABLE IF NOT EXISTS `mochat_go_ai_analysis` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT 0,
  `corp_id` int(10) unsigned NOT NULL DEFAULT 0,
  `page` varchar(64) NOT NULL DEFAULT '',
  `status` varchar(16) NOT NULL DEFAULT 'succeeded',
  `payload` json DEFAULT NULL,
  `error` varchar(1024) NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ai_analysis_corp_page_created` (`corp_id`, `page`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

迁移常量同步：`internal/dashboard/saas_admin_system_health.go` → `SaaSAdminExpectedMigrationCount=126`、`SaaSAdminExpectedMigrationVersion="0126_phase3_final_providers"`；`internal/migration/migration_test.go` 中所有 latest version 断言同步更新；如有其他计数断言一并搜索更新。

## 4. API 契约

统一信封沿用现有 `{code,msg,data}`。

### 音频（新增模块，注册前缀 `/dashboard/chat/media`）

- `GET /dashboard/chat/media?corpId=&page=1&perPage=20&keyword=`
  data：`{list:[{id, originalName, contentType, sizeBytes, durationSeconds, createdAt, playUrl}], total, page, perPage}`
  `playUrl` 为 `/dashboard/chat/media/{id}/content`（绝对路径，供 `<audio>` 直接使用）。
- `POST /dashboard/chat/media?corpId=` multipart `file`（字段名 `file`）
  校验：仅音频 MIME（`audio/mpeg|wav|ogg|mp4|x-m4a|aac|amr|webm`），≤50MB；同名覆盖策略：按内容写唯一路径，元数据可重复。
  成功返回 200 + 新建对象。
- `GET /dashboard/chat/media/{id}/content`
  鉴权：JWT 会话 + corp 租户匹配 + 权限点 `/chat/file-audio#download`；返回字节流、`Content-Type`、`Content-Length`、`X-Content-Type-Options: nosniff`、`Cache-Control: private, max-age=3600`；软删对象返回 404。
- `DELETE /dashboard/chat/media/{id}?corpId=`：软删（`deleted_at`+`deleted_by`），返回成功。
- 权限：沿用 `cmd/mochat-go` 已装配的 SCRM PrincipalResolver / LeadAuthorizer 模式（新 bootstrap 函数注入同一组依赖）。

### AI 洞察（扩展现有模块）

- `GET /dashboard/ai-insight/{page}?corpId=&refresh=`（page：session-analysis/smart-analysis/emotion/employee-score/communication-keyword）
  - Provider 未配置：保持 `capability:"limited"`，`limitations` 给出“未配置 AI Provider / 暂无归档会话数据”的真实原因，不伪造 0/成功。
  - Provider 已配置且存在归档文本（`mc_work_message_1..10` 按 `corp_id` 取 `content_text` 非空、未删除，按 `msg_data_time` 倒序限 20 条）：
    - 5 分钟内存在该 corp+page 的 `succeeded` 结果且无 `refresh=1` → 直接回读；
    - 否则按页面专用中文 prompt 调用 AI，结果 JSON 落 `mochat_go_ai_analysis`，返回 `capability:"ready"`、`provider:"dashscope"`、`data:[{sessionId:"archive", summary, keywords, generatedAt}]`。
  - 归档为空：`limited` + 原因“暂无归档会话数据”。
  - AI 调用失败/超时：`limited` + 错误原因（不落成功行）。
- 归档文本读取需处理 10 个分表：遍历 `mc_work_message_1..10`，任一表不存在时跳过。

## 5. 配置项（compose 映射 + README 文档）

```text
MOCHAT_GO_AI_PROVIDER_BASE_URL    默认 https://dashscope.aliyuncs.com/compatible-mode/v1
MOCHAT_GO_AI_PROVIDER_KEY         空（部署时注入，不入库/不入 git）
MOCHAT_GO_AI_PROVIDER_MODEL       默认 qwen-plus
MOCHAT_GO_AI_PROVIDER_TIMEOUT_SECONDS 默认 30
MOCHAT_GO_WECOM_ARCHIVE_CORP_ID   空
MOCHAT_GO_WECOM_ARCHIVE_SECRET     空
MOCHAT_GO_WECOM_ARCHIVE_PUBLIC_KEY 空
MOCHAT_GO_WECOM_ARCHIVE_PRIVATE_KEY 空
```

`MOCHAT_FILE_STORAGE_ROOT` 已存在（compose 中 `/app/storage/upload/static` + `app-storage` 卷），音频写入其下 `audio/` 子目录。

## 6. 企微会话存档适配层（本期边界）

- `internal/modules/providers/archive/wecom`：配置校验（四件套齐全才 `ready`，否则 `limited` + 缺项原因）；`Sync` 在 `limited` 时返回明确错误。
- 解密基础件：实现自洽的 AES-CBC/PKCS7 + RSA 解密函数，单元测试用自生成密钥做 round-trip（明文→加密→解密），不宣称与企微线上协议完全兼容。
- 契约测试：mock 解密后的消息载荷 → 规范化为 `mc_work_message_1` 行（`corp_id/msgid/seq/work_employee_id/to_user_type/to_user_id/sender_type/type/msg_type/content/content_text/room_id/status/msg_data_time`）→ 入库 → 回读断言。
- 无真实凭证时页面保持受限态；真实凭证由用户提供后在后续轮次激活并做浏览器级回读验收。

## 7. 验收标准（主任务执行）

1. 门禁全绿：`go test ./...`、Dashboard Vitest 全量、typecheck、生产 build、`pnpm check:phase3-5-dashboard`、`pnpm check:debt-clearance`（此时 53/53）、`git diff --check`。
2. 部署：重建 `mochat-go-desktop`（保留数据卷），迁移到 `0126`，`/readyz` 200，三容器 healthy。
3. 音频闭环：真实小 WAV 上传 → 落盘校验（容器卷内文件存在、sha256 一致）→ 列表回读 → 鉴权下载字节一致 → 浏览器 `<audio>` 可加载 → 软删后列表消失。
4. AI 闭环：注入百炼 key 与归档种子文本 → `session-analysis` 返回真实摘要并落库 → 页面回读展示；未配置时保持受限态（复验）。
5. 浏览器/识图：`/chat/file-audio` 与 AI 洞察页面截图 + `qwen3-vl-plus` 严格缺陷审阅。
6. 文档收口：总验收报告、矩阵、阶段 README、总进度、`check` 脚本同步。

## 8. 任务拆分与职责

- 子代理 A（后端）：`internal/modules/providers`、迁移 `0126`、音频 HTTP 模块、AI 洞察真实化、compose/README 配置、Go 测试；不部署、不提交。
- 子代理 B（前端）：`file-audio-page.tsx`、page-registry、manifest `/chat/file-audio` 状态、`check_debt_clearance.mjs`、AI 洞察类型小改、Vitest；不部署、不提交。
- 主任务（本会话）：设计定稿、门禁复跑、部署、浏览器/识图验收、数据流证据、文档收口、提交推送。

## 9. 风险与保留

- 企微真实解密依赖用户凭证，本期不宣称线上兼容，只交付适配层与契约测试。
- AI 结果质量依赖模型与归档文本，验收以“真实调用成功 + 落库回读 + 页面展示”为准，不评审分析内容本身。
- 音频时长未解析时为 0，前端显示 `--`，不伪造时长。
