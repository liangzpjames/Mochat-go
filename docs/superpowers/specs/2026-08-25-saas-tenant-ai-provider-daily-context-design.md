# SaaS 租户级 AI Provider、日分析与历史上下文统一设计

## 1. 背景与交付目标

当前 AI 分析使用进程级单一 OpenAI-compatible Provider；不同 SaaS 客户不能分别配置厂商、模型、API Key 和有效期。分析结果又存在两条持久化链路：现代会话级结构化结果写入 `mochat_go_ai_conversation_insights`，旧版页面级自然语言摘要写入 `mochat_go_ai_analysis`。旧链路不包含租户、会话或证据维度，已不被三个优化页面消费。

本次交付目标：

1. SaaS 管理员可在每个客户租户详情中独立配置 AI 厂商、OpenAI-compatible Base URL、模型、API Key、生效时间、失效时间和启停状态。
2. 租户配置缺失、未生效、已过期、已停用、无法解密或字段无效时失败关闭，不回退平台公共 API Key。
3. AI 只读取 Asia/Shanghai 当前自然日 00:00 至执行时刻的真实归档消息。
4. 每次请求增加同一会话、同一分析类型、同一规则上一自然日最近成功结果的评分、摘要和生成时间，作为不可引用为证据的低优先级连续性参考。
5. 统一使用 `mochat_go_ai_conversation_insights`，删除旧表 `mochat_go_ai_analysis` 及其代码链路；情绪、员工评分、关键词继续由一次结构化会话分析投影，不再额外发三次旧摘要请求。
6. 保持自动日分析和启动即分析的部署开关语义；配置 Provider 本身不会自动打开外部调用。

## 2. 范围矩阵

| 范围 | 现状 | 本次结果 |
| --- | --- | --- |
| Provider 配置 | 进程级环境变量 | 租户级数据库配置，运行时按租户解析 |
| 厂商协议 | OpenAI-compatible Chat Completions | 保持统一协议；提供 DeepSeek、OpenAI、DashScope、自定义预设 |
| API Key | 环境变量或文件 | 独立加密域密文保存，读取接口永不回显 |
| 有效期 | 无 | `effective_at` 与 `expires_at` 硬门禁 |
| 分析时间 | 默认 30 天或规则回看天数 | Asia/Shanghai 当日 00:00 至当前时刻 |
| 历史连续性 | 请求不含上次结果 | 加入上一自然日同范围成功快照 |
| 结构化结果 | `mochat_go_ai_conversation_insights` | 保留并增加分析日期、上次结果审计快照 |
| 旧摘要结果 | `mochat_go_ai_analysis` | 停止读写并通过新迁移删除 |
| 三个优化页面 | 读取会话分析投影 | 不变，继续同源 |
| Phase 7 | 暂停 | 保持暂停，不宣称真实企微归档完成 |

## 3. 方案选择

### 3.1 采用方案：配置仓储 + Provider Resolver

新增租户配置仓储和 `TenantAIProviderResolver`。运行器在进入每个租户/企业分析前解析一次 Provider，将解析出的客户端用于该企业全部候选会话。Provider Resolver 负责校验租户范围、启停状态、有效期、解密密钥和连接参数；失败时返回结构化不可用状态，并为该租户记录失败运行。

相比在 `ChatRequest` 中塞入租户 ID，此方案避免传输请求与租户配置耦合；相比为每家厂商写独立 SDK，统一适配器避免重复实现。厂商预设只提供 Base URL 默认值，模型由管理员显式填写，避免模型目录随外部平台变化而失真。

### 3.2 未采用方案

- 全局 Provider 作为回退：会混用客户额度、费用和数据边界，违反已确认的失败关闭要求。
- 每个页面独立 Provider 和结果表：会重复调用模型并重新制造不一致口径。
- 强制数学限分：可能掩盖当日真实服务变化。本设计只提供历史参考并要求大幅变化有当日消息证据，不篡改模型的合法结果。

## 4. 数据模型与迁移

### 4.1 租户 AI Provider 配置

新增 `mochat_go_saas_tenant_ai_providers`：

| 字段 | 语义 |
| --- | --- |
| `tenant_id` | SaaS 客户租户，唯一配置范围 |
| `provider_code` | `deepseek`、`openai`、`dashscope` 或 `custom` |
| `base_url` | OpenAI-compatible API 基础地址，不含 `/chat/completions` |
| `model` | 管理员明确配置的模型名 |
| `api_key_ciphertext` | AES-GCM 认证加密密文 |
| `api_key_key_id` | 加密主密钥版本 |
| `api_key_hint` | 仅用于界面的末四位提示，不具备认证能力 |
| `effective_at` | 生效时刻，UTC 持久化 |
| `expires_at` | 失效时刻，UTC 持久化，必须晚于生效时刻 |
| `status` | `active` 或 `disabled` |
| `version` | 乐观锁版本 |
| 创建/修改人和时间 | 审计元数据 |

每个租户仅有一条当前配置。修改 API Key 时生成新密文；更新其他字段且 API Key 留空时保留原密文。读取接口只返回 `apiKeyConfigured`、`apiKeyHint` 和加密状态，永不返回明文或密文。

### 4.2 AI Key 加密

新增独立的 AI Provider 凭证管理器，使用 AES-GCM、随机 nonce、租户 ID 与配置用途作为附加认证数据，并使用独立派生上下文。优先读取 AI 专用 key/key-ring；可从现有平台凭证主密钥取得原始材料，但仍通过独立派生上下文形成密码学隔离。没有可用加密密钥时禁止保存和运行，不兼容明文落库。

### 4.3 会话结果日快照

在 `mochat_go_ai_conversation_insights` 增加：

- `analysis_date`：Asia/Shanghai 自然日。
- `previous_insight_id`：本次请求引用的上一自然日记录 ID，缺失为 0。
- `previous_score`：会话分析取 `employeeQa.score`，智能分析取 `matchScore`，缺失为 NULL。
- `previous_summary`：发送给模型的历史摘要快照。
- `previous_generated_at`：历史结果生成时间。

唯一键调整为租户、企业、分析类型、规则版本、会话和分析日期。同一天归档消息新增后重跑会更新当日记录，不产生重复页面行；来源指纹仍用于判断当日消息与助手配置是否变化。

迁移先回填 `analysis_date`，再按新唯一范围保留每日最新记录并删除同日旧重复，最后建立新唯一键。历史跨日结果继续保留。

### 4.4 删除旧表

删除所有 `SQLAnalysisStore`、旧 `BuildAnalysisPrompt`、旧三页面日摘要循环及相关测试后，通过迁移执行：

```sql
DROP TABLE IF EXISTS mochat_go_ai_analysis;
```

旧表只有企业/页面级自然语言摘要，无法诚实映射为会话级结构化证据，因此不伪造迁移。down 迁移只可重建空表结构，不能恢复已删除数据；生产回滚如需旧数据必须使用迁移前数据库备份。

## 5. Provider 解析与失败关闭

Provider Resolver 按以下顺序处理：

1. 以明确的 `tenant_id` 查询唯一配置。
2. 校验配置存在且 `status=active`。
3. 校验 `effective_at <= now < expires_at`。
4. 校验厂商代码、HTTPS Base URL 和模型非空。
5. 使用对应 key ID 解密 API Key。
6. 创建 OpenAI-compatible 客户端；不缓存明文 API Key，不记录 Authorization 请求头。

任一步失败均返回脱敏错误码，例如 `AI_PROVIDER_NOT_CONFIGURED`、`AI_PROVIDER_NOT_EFFECTIVE`、`AI_PROVIDER_EXPIRED`、`AI_PROVIDER_DISABLED`、`AI_PROVIDER_CREDENTIAL_UNAVAILABLE`。运行记录和页面状态只显示错误码及安全说明，不输出 Base URL 查询参数、密文或 API Key。

厂商预设：

| 厂商 | 默认 Base URL |
| --- | --- |
| DeepSeek | `https://api.deepseek.com` |
| OpenAI | `https://api.openai.com/v1` |
| DashScope | `https://dashscope.aliyuncs.com/compatible-mode/v1`，管理员可改为业务空间专属地址 |
| 自定义 | 必须手工填写 HTTPS 地址 |

参考官方资料：DeepSeek OpenAI-compatible 基础地址与模型参数见 <https://api-docs.deepseek.com/>；OpenAI Chat Completions 端点见 <https://platform.openai.com/docs/api-reference/chat>；DashScope 兼容接口与业务空间地址见 <https://help.aliyun.com/zh/model-studio/qwen-api-via-openai-chat-completions>。

## 6. 当日分析与上一次结果参数

### 6.1 当日窗口

所有自动分析固定使用 `Asia/Shanghai` 当前日期的 `[00:00:00, now]`。规则的会话类型、目标范围和最小消息数继续生效，`lookback_days` 不再影响执行窗口；规则界面将其显示为固定“当日”。显式历史回填命令不再作为租户正常分析入口。

### 6.2 上一次结果选择

对每个候选会话，在调用模型前查询：

- 同租户、同企业；
- 同 `analysis_type`；
- 同 `rule_version_id`；
- 同 `conversation_key`；
- `analysis_date < 当前分析日期`；
- `status='succeeded'`；
- 按 `analysis_date DESC, generated_at DESC, id DESC` 取一条。

会话分析提取 `employeeQa.score` 和 `summary`；智能分析提取 `matchScore` 和 `conclusion`。历史 JSON 无法解析、评分类型不合法或无记录时，只写“无可用上次结果”，不阻止当日分析。

### 6.3 请求文本

在来源消息之后增加：

```text
上一次分析参考（仅用于保持指标连续性，不能作为事实或 evidenceMessageIds）：
- generatedAt=...
- score=...
- summary=...
要求：以当日消息为唯一事实依据；证据不足以支持显著变化时保持合理连续；确有显著变化时可以变化，但原因必须能由当日消息解释。
```

上一次结果不加入允许证据 ID 集合。首次结构纠错请求保留该参考和约束。保存当日结果时同步保存实际发送的历史快照，便于审计。

## 7. SaaS 管理 API 与权限

新增接口：

- `GET /dashboard/saasAdmin/tenantAIProvider?tenantId={id}`：需要 `platform.integrations.read`。
- `PUT /dashboard/saasAdmin/tenantAIProvider`：需要 `platform.integrations.manage`。

PUT 使用 `version` 乐观锁；首次创建版本为 1。API Key 首次创建必填，后续留空表示保持，不能用掩码文本覆盖。厂商、地址、模型、有效期、状态均进行服务端验证。

保存前后写入现有 SaaS 操作审计，审计只记录厂商、模型、Base URL 主机、有效期、状态、Key 是否变更和版本，不记录 API Key、密文或完整 Authorization 信息。平台超级管理员仍需经过同一业务校验；普通租户管理员不能访问 SaaS 平台接口。

## 8. SaaS 管理界面

在“客户租户 → 租户详情”中增加“AI 分析配置”区：

- 当前状态：未配置、未生效、可用、即将过期、已过期、已停用或凭证不可用。
- 厂商选择器自动填入预设 Base URL，管理员仍可检查并修改。
- 模型文本框明确提示模型名由厂商账户决定。
- API Key 使用密码框；编辑时留空保持，提交后立即从 React 状态清除。
- 生效时间和失效时间使用本地时间输入，提交为 RFC3339。
- 启停状态、版本及最近更新时间可见。
- 无 `platform.integrations.read` 时整个区域不请求、不渲染；只有 read 权限时只读；manage 权限才显示保存按钮。

保存错误保留非敏感表单值，但清空 API Key。关闭弹窗、保存成功、Escape 或遮罩关闭时均清空 API Key。桌面、390px 窄屏和小高度保持字段、按钮和错误信息可操作。

## 9. 安全与租户隔离

- 运行时必须由业务调用方显式传入租户和企业，Provider Resolver 不接受客户端传来的任意租户覆盖。
- 配置查询、结果查询和历史参考查询全部同时限定租户与企业。
- API Key 不进入 JSON 响应、日志、审计、错误、测试快照、Git、URL 或命令行。
- Base URL 只允许 HTTPS，禁止内网、环回、链路本地和包含用户信息的 URL，降低 SSRF 风险；连接仍使用现有出站 HTTP 安全策略。
- 失效边界使用 `now >= expires_at`，到期瞬间立即失败关闭。
- 保存配置不会触发模型测试或分析，避免未授权费用；实际调用只由已有显式/调度分析入口触发。

## 10. 验收标准

1. 租户 A、B 可保存不同厂商、模型和加密 API Key；读取时均不回显 Key，跨租户不可读写。
2. 缺失、未生效、过期、停用、解密失败均不调用外部 Provider，且状态诚实可见。
3. 当日窗口严格按 Asia/Shanghai，昨日消息不进入来源消息列表。
4. 第二天同会话请求含上一日评分、摘要和生成时间，但历史内容不能成为证据 ID。
5. 同日重跑更新同一日记录；三个优化页面和数据概览继续读取统一表。
6. 代码中无 `mochat_go_ai_analysis` 运行时引用，迁移后该表不存在。
7. SaaS 页面完成加载、创建、更新、留空保留 Key、有效期校验、启停、错误恢复、Escape/遮罩关闭与响应式点击测试。
8. 相关 Go、SaaS 管理端、Dashboard、迁移、真实 MariaDB、Docker、lint、typecheck、production build、凭证扫描和 `git diff --check` 全部通过。

## 11. 自审结论

本设计没有待定项。删除目标精确限定为旧表 `mochat_go_ai_analysis`；规则、运行审计和会话结构化结果表继续保留。历史参考的范围、时区、失败行为、凭证生命周期、权限和回滚数据边界均已明确。
