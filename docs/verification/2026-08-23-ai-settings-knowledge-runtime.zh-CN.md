# AI 设置知识运行时实施与自测报告

日期：2026-08-23

分支：`feat/ai-settings-menu-optimization`

工作树：`D:\workspace\mochat-go\mochat-go\.worktrees\ai-settings-menu-optimization`

## 1. 交付结论

本次已把“AI 知识库—会话分析助手—AI 洞察”从三个相互独立的配置页面闭环为一条真实运行链路：

1. AI 知识库支持上传、解析、列表、失败反馈和删除真实文档；文件私有保存，元数据与分段持久化到数据库。
2. 智能体固定为每个租户/企业一个系统级“会话分析助手”；不允许新增、删除或改名，只允许配置分析要求、关联知识库和运行状态。
3. AI 洞察会话分析读取该助手配置与已启用知识库的可用分段；设置或文档变化会进入来源指纹，触发后续分析重算。
4. 知识只作为背景资料，不能伪造真实消息证据，也不能覆盖系统约束、返回结构或消息范围。
5. 历史手工文档数已通过 `0157` 按真实文档账本重算，页面不再显示无文件支撑的 12/8 假计数。

设计依据与实施计划：

- `docs/superpowers/specs/2026-08-23-ai-settings-knowledge-runtime-design.zh-CN.md`
- `docs/superpowers/plans/2026-08-23-ai-settings-knowledge-runtime.zh-CN.md`

## 2. 实施范围

### 2.1 页面与路由

| 页面 | 路由 | 结果 |
| --- | --- | --- |
| AI 知识库 | `/ai-setting/ai-knowledge-base` | 真实计数、筛选、新增/编辑/启停、文档管理、上传解析、失败反馈、删除保护 |
| 智能体设置 | `/ai-setting/agent` | 固定唯一“会话分析助手”，移除新增/删除，名称只读，配置知识库和启停 |
| AI 洞察会话分析 | `/ai-insight/session-analysis` | 显示实际消费的助手、启用知识库数与可用文档数，并链接回配置页 |

### 2.2 接口

新增文档接口：

- `GET /dashboard/ai-settings/knowledge-bases/{id}/documents`
- `POST /dashboard/ai-settings/knowledge-bases/{id}/documents`
- `DELETE /dashboard/ai-settings/knowledge-bases/{id}/documents/{documentId}`

现有智能体接口收紧：

- `GET /dashboard/ai-settings/agents`：确保并只返回当前企业的系统助手。
- `PUT /dashboard/ai-settings/agents/{id}`：允许配置分析要求、知识库和状态，拒绝改名。
- `POST /dashboard/ai-settings/agents`、`DELETE /dashboard/ai-settings/agents/{id}`：明确返回 `405`。

所有读取与写入继续经过 Dashboard 登录态、RBAC、`tenant_id + corp_id` 范围校验和审计边界。知识库父级查询异常返回 `500`，不再把数据库故障伪装成 `404`。

### 2.3 迁移与存储

- `0156_ai_settings_knowledge_runtime`
  - 为智能体增加 `system_key` 与租户/企业唯一约束；
  - 新增 `mochat_go_ai_knowledge_documents`、`mochat_go_ai_knowledge_chunks`；
  - 为有效企业补建唯一“会话分析助手”；
  - 写入三条文档接口 RBAC 资源。
- `0157_ai_settings_document_count_reconcile`
  - 将历史 `document_count` 重算为真实未删除文档数量；
  - 显式处理历史知识库 ID 与新文档外键列的排序规则差异。
- 私有文件根目录：`/app/storage/upload/ai-knowledge-private`，不在 `/static` 下，无公开下载地址。

支持 `.txt`、`.md`、`.pdf`、`.docx`；单文件 20 MB，最多 500,000 字符。PDF 必须有文字层，旧 `.doc`、无效 UTF-8、空文档、扫描图片 PDF、加密或不可读文件诚实失败。解析失败但类型受支持时保存失败状态，不进入检索。

## 3. AI 洞察消费模型

- 每次运行读取系统助手及其已启用知识库。
- 本地确定性相关性检索按关键词与中文双字片段评分，最多选择 8 个分段、约 12,000 字符；不依赖额外向量 Provider。
- 助手分析要求和知识片段以低优先级背景提示注入模型；真实归档消息仍是唯一证据来源。
- `evidenceMessageIds` 仍只接受本次真实消息集合中的 ID。
- 助手停用时记录“会话分析助手已停用”的失败运行并跳过模型调用，不伪造成功结果。
- 配置与知识内容摘要进入来源指纹；修改、启停、上传或删除都会影响下一次分析的缓存判定。

## 4. 自动化验证

| 验证 | 命令 | 结果 |
| --- | --- | --- |
| 全量 Go | `go test ./... -count=1` | PASS，全部包通过 |
| Dashboard 全量测试 | `pnpm --filter @mochat/dashboard test -- --run` | PASS，140 个文件 / 830 个测试 |
| Dashboard typecheck | `pnpm --filter @mochat/dashboard typecheck` | PASS |
| 目标 lint | `pnpm --filter @mochat/dashboard exec eslint src/features/ai-settings src/features/ai-insight` | PASS |
| Dashboard production build | `pnpm --filter @mochat/dashboard build` | PASS；仅保留仓库已有 chunk size 提示 |
| Yuanhu benchmark | `pnpm check:yuanhu-benchmark` | PASS，53 个页面 |
| 页面 evidence 校验 | `pnpm check:dashboard-all-pages-evidence` | PASS，12/12 |
| RBAC/Provider/Scope 门禁 | `pnpm check:phase4-dashboard-page-rbac` | PASS；53 个页面，0 未映射 |
| 差异格式 | `git diff --check` | PASS |

Dashboard 全量 lint 仍为仓库基线失败：`86 errors / 0 warnings`，均位于本任务之外的既有页面或测试；本次修改覆盖的 `ai-settings`、`ai-insight` 目标 lint 为 0。测试中的 `jsdom window.getComputedStyle(elt, pseudoElt)` 为仓库共享 Modal 的既有非阻断 stderr，最终 830 个测试全部通过。

## 5. Docker 与迁移验收

- 使用项目名 `mochat-go-desktop` 和原有 MySQL/Redis/App 卷构建最终镜像；未删除任何卷。
- `mochat-migrate -action apply -project-root /app` 已应用到 `0157_ai_settings_document_count_reconcile`，重复执行校验通过。
- `mochat-go-desktop-app-1`、`mysql-1`、`redis-1` 均为 `healthy`；`GET /readyz` 返回 200。
- 数据库实查：系统助手未删除记录为 1；知识文档两张表存在；历史产品/话术知识库计数由 12/8 校正为 0/0。
- 保留数据库中存在一条本任务前已有的历史迁移账本 `0150_live_code_workspace`，同时当前源码使用 `0153_live_code_workspace`，因此保留卷账本总行数比当前迁移清单多 1。为避免越权删除历史账本，本任务仅验证最新版本与新迁移校验和，不修改该旧记录。

第一次未显式指定 Compose 项目名时曾生成 `standalone_*` 容器与未使用卷；已只移除冲突容器和网络，按要求保留所有卷不删除。最终运行环境始终使用 `mochat-go-desktop` 原卷。

## 6. 真实浏览器证据

使用已登录本地 Dashboard，在常规桌面和 `390 × 680` 窄屏/小高度下执行：

1. 知识库页刷新后显示真实总数 2、文档数 0，历史 12/8 已消失。
2. 新建临时验收知识库；筛选命中 1/3；编辑说明；停用后已启用数从 3 降到 2，再启用恢复 3。
3. 上传 `427 B` Markdown，服务端解析为 `150 字符 / 1 分段`，列表文档数变为 1；整页刷新后仍存在。
4. 上传 `.json` 得到明确错误“仅支持 TXT、Markdown、PDF 和 DOCX 文件”，未伪造成功记录。
5. 文档管理弹窗、编辑弹窗验证 Close、取消和 Escape；知识库与文档删除入口均展示对象名、影响说明和确认/取消。
6. 智能体页只有一个“会话分析助手”，无新增/删除入口；名称实际为 `readonly`；分析要求、知识库关联、停用/启用和刷新持久化通过。
7. AI 洞察会话分析页在关联阶段实际显示“1 个已启用知识库、1 份可用文档”；清理验收数据后回到“0 个、0 份”，没有遗留测试知识。
8. 390×680 下顶栏折叠、主内容单列、指标卡纵向排列，知识库与智能体页面可滚动到所有内容；桌面文档表宽度已收敛到弹窗内容区。

本次没有可声明通过的外部模型写入：保留卷中无可用会话分析结果，且任务不扩展真实企微会话存档。浏览器仅验证配置消费摘要与真实本地持久化；Provider 缺少凭证时仍按现有状态诚实受限。

验收专用知识库、文档、关联关系、私有对象和本地上传样本均已精确清理；原有用户知识库未删除。浏览器截图已在任务执行记录中于“真实计数”“文档解析成功”“固定助手”“AI 洞察关联”“390×680”五个关键节点输出，未将二进制测试证据提交到仓库。

## 7. 原子提交

本任务 AI 设置优化从 `5bf8e9b` 开始；知识运行时核心提交为：

- `d7f4c36` 设计与数据/安全边界
- `2943a87` 文档 schema
- `79fcc78` 私有文档解析
- `c43b692` 文档管理工作流
- `2e0a00e` 唯一系统助手
- `9cdd74b` AI 洞察消费助手知识
- `c379100` Dashboard 知识运行时界面
- `34ec60c`、`0838d30` 迁移健康与 RBAC 门禁
- `467a956` 历史文档计数校正
- `fcad8ff` 父级查询故障语义修复
- `f189fdc` 桌面文档弹窗适配

## 8. 回滚与已知边界

- 回滚 `0157` 不恢复历史手工计数，因为恢复无文档支撑的数值会重新制造假数据。
- 回滚 `0156` 会删除文档元数据、分段表和 `system_key`；回滚前必须先导出需要保留的私有文件和元数据。
- PDF 只提取文字层，不做 OCR；旧 `.doc` 不支持。
- 当前采用本地确定性检索，不包含语义向量召回；这是无需外部凭证、可审计且诚实可用的最小闭环。
- Phase 7 继续暂停；未新增真实企微会话存档能力，也未触碰旧 `web/saas-admin/`。
