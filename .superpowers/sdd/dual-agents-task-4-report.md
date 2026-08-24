# Task 4 收尾报告：Runner 独立加载与量化 Schema v2

## 实现范围与自审

- Runner 通过 `SystemAssistantRepository` 幂等确保并按 `session-analysis`、`smart-analysis` 分别加载助手上下文；说明、知识库、启停状态和 fingerprint 均按类型隔离。
- 会话流使用 `session-analysis` 当前启用规则版本；智能流继续使用 `default-smart-analysis` 当前启用规则版本。成功洞察保存对应规则 ID、版本 ID、版本号和名称快照。
- 单一助手停用或加载失败只记录该类型的失败 run，另一类型仍可执行；provider 不可用时仅记录两类失败 run，不保存伪造洞察。
- 会话自定义提示词置于低优先级 guidance 区段，固定证据、Schema 与安全约束在其后；智能流只使用智能规则 objective 和智能助手上下文。知识库仅是背景，解析器仍限制证据 ID 必须来自来源消息。
- 两类结构化请求开启 `JSONMode`；OpenAI-compatible 请求仅在该标记开启时发送 JSON object response format。可选 metadata 只保存 provider/model，不包含 key 或 base URL。
- Session/Smart Schema v2 维持 v1 可读：新增量化字段、nullable 分数、优先级与未解决问题校验，并拒绝超范围分数/权重、非法 level、空 name/title 和越源证据。
- Status handler 根据页面加载会话或智能助手。未改筛选 API、前端、migration、Docker/secret 或 simulator。

## 红绿证据

原代理记录的测试驱动过程：

- Parser 的 Session/Smart v2 边界与兼容性测试在实现前为红灯，完成版本分支、量化字段和证据校验后转绿。
- OpenAI JSONMode 测试在实现前为红灯，完成仅结构化请求写入 `response_format` 的改动后转绿。

本次接手后的新鲜验证（2026-08-24，Asia/Shanghai）：

- `go test ./internal/modules/ai-insight/... -count=1`：通过（`ai-insight`、`ai-insight/transport/http`）。
- `go test ./internal/modules/providers/ai/openai/... -count=1`：通过。
- `go test ./internal/app/bootstrap/... -count=1`：通过。
- `git diff --check`：退出码 0；仅有 Git 的 LF/CRLF 提示，无 whitespace 错误。

## 兼容性与关注点

- `SessionAssistantContext` 保留为 `SystemAssistantContext` 的 alias；构造器保留旧会话助手接口的兼容分支，生产接线使用系统助手能力。
- v1 智能洞察仍接受 `confidence: 0..1|null`，不会填充 v2 四项综合分；v2 明确拒绝 legacy `confidence`。
- 工作树的 `.superpowers/sdd/progress.md` 为既有进度记录，按任务要求不纳入本提交。

## 独立审查 Important 修复追加记录（2026-08-24）

本轮按 TDD 修复 Task 4 独立审查的全部 Important findings：

- 版本合同：新增版本专属 raw shape 检查。RED 时，v1 会接受 Session 的 `qualityScore`、量化 `dimensions`、未解决问题字段以及 Smart 的 v2 分数字段；v2 会接受缺失的 Session `qualityScore` 和量化维度 `weight`/`score`，并把 `weight:null` 当作 0。GREEN 后，v1 拒绝全部 v2-only 字段，v2 对 nullable 分数字段区分“缺失”与显式 `null`，量化 `weight` 必须为非 null 数字。
- JSON round-trip：RED 时，Session v2 的 `qualityScore:null` 以及 Smart v2 的四个显式空分数会被 `omitempty` 丢弃。GREEN 后，版本条件序列化保留 Session `qualityScore`、Smart 四项分数和量化 `score` 的显式 `null`，同时 v1 序列化不会注入 v2 字段。
- 会话规则缺失：RED 时，`CurrentEnabledRuleVersion(session)` 返回 `nil, nil` 后仍会以 `ruleVersionID=0` 执行并成功。GREEN 后，会话流先持久化“当前启用规则版本不存在”的真实 failed run，不调用会话模型；即使智能规则加载同时失败，也会先保存该会话失败。
- 助手初始化隔离：RED 时，`EnsureSystemAssistants` 返回错误会跳过两个 context 加载并使两类都失败。GREEN 后仍分别尝试 `session-analysis` 和 `smart-analysis`；可加载且启用的类型继续执行，仅加载失败或停用的类型记录失败。
- 状态兼容分支：RED 时，旧 `SessionAssistantRepository` 在 smart 页返回了会话助手。GREEN 后旧分支仅为 session 页返回助手，smart 页省略助手字段；系统助手分支仍按 page 加载对应 system key。

本轮新鲜验证：

- 聚焦 RED：新增回归用例均按上述预期失败，失败原因分别为版本污染未拒绝、必填字段缺失未拒绝、显式 `null` 丢失、会话规则 ID 0 运行、Ensure 错误阻断双加载、smart 页暴露会话助手。
- 聚焦 GREEN：`go test ./internal/modules/ai-insight -run 'Test(ParseAnalysisV1RejectsV2OnlyFields|ParseAnalysisV2RequiresNullableScoresAndDimensionFields|AnalysisResultRoundTripPreservesV2NullsWithoutPollutingV1|ConversationRunnerDoesNotRunSessionWithoutCurrentRuleVersion|ConversationRunnerLoadsBothContextsAfterEnsureFailure|ConversationRunnerEnsureFailureOnlyFailsActuallyUnavailableContext|WorkspaceSmartStatusDoesNotExposeLegacySessionAssistant)$' -count=1`：通过。
- 目标全量：`go test ./internal/modules/ai-insight/... ./internal/modules/providers/ai/openai/... -count=1`：通过。
- bootstrap：`go test ./internal/app/bootstrap/... -count=1`：通过。
- `git diff --check`：退出码 0；仅 Git 的 LF/CRLF 提示，无 whitespace 错误。

## 第二轮复审 Important 修复追加记录（2026-08-24）

- Smart 版本序列化：RED 时 v1 的 `confidence:null` 与 v2 的合法 `dimensions:[]` 会被 `omitempty` 删除，其中 v2 marshal 结果无法再次通过 parser。GREEN 后，v1 始终按版本合同输出 `confidence`，v2 始终输出 `dimensions`；两类结果均完成 parse→marshal→parse round-trip。
- v2 nested raw shape：RED 时，`QuantifiedDimension` 和 `UnresolvedIssue` 的 `evidenceMessageIds` 缺失或为 `null` 都会折叠成 nil slice并通过校验。GREEN 后，每个 nested item 都要求该字段存在、非 null 且为字符串数组，空数组仍合法。
- legacy 助手隔离：RED 时，`AssistantContextProvider` 的会话说明、知识库和 fingerprint 会进入 smart 流，且会话助手停用会同时停止 smart。GREEN 后，legacy context 与其加载失败/停用状态仅作用于 session；smart 继续无 context 执行。旧的 smart 不可用持久化测试改用真正支持双 system key 的 provider stub。
- 测试稳定性：完整包验证复现了 status 映射测试以 map 驱动请求却固定断言顺序的既有 flaky 行为；仅将测试输入改为有序表，未修改生产映射。

本轮红绿证据：

- 聚焦 RED：`TestSmartAnalysisVersionRoundTripsPreserveRequiredFields`、`TestParseAnalysisV2RequiresNestedEvidenceMessageIDArrays`、`TestConversationRunnerLegacyAssistantContextOnlyAppliesToSessionAnalysis`、`TestConversationRunnerLegacySessionAssistantDisabledDoesNotStopSmartAnalysis` 均按预期失败。
- 聚焦 GREEN：上述测试及修正后的 smart unavailable persistence 测试全部通过。
- 完整 `ai-insight`：`go test ./internal/modules/ai-insight/... -count=1` 通过。
