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
