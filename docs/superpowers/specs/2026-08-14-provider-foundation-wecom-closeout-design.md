# Provider 基础与企业微信标准同步设计

## 目标与边界

本批次交付一个可独立验收的 Provider foundation + WeCom standard sync milestone：所有 Provider 通过统一的状态、来源、能力和错误码合同被注册；Dashboard 能在已认证 principal 的 tenant/corp 作用域内读取状态；企业微信员工/部门同步复用当前真实 HTTP client 合同；会话存档在没有真实 `getchatdata` 实现时明确保持 `limited`，不能因为四件凭据齐全而返回 `ready`。

本批次不调用真实外部企业微信账号，不写生产数据库，不操作服务器、Docker 或卷，不实现付费会话存档 `getchatdata`，不把模拟 Provider 变成真实 Provider，也不把营销页面从 `partial` 改成 `ready`。

## 现状结论

- `internal/modules/providers/providers.go` 只有 `State`、`Status` 和少量能力接口，缺少统一来源、稳定 machine code、动作提示和注册表。
- `internal/modules/providers/archive/wecom` 在四项归档凭据齐全时返回 `ready`，但 `Sync` 仍返回 activation pending；这是错误的 ready 自证，必须改为 `limited`/`unavailable`。
- `internal/dashboard.RoomWelcomeWeComClient` 已具备 `gettoken`、部门、员工、客户、客户群、联系我等真实 API 路径，员工同步 worker 已使用该 client；本批次不复制第二套 HTTP client，而是让标准同步合同显式复用这条路径并覆盖 fake HTTP server。
- Dashboard 已有 company profile 的 tenant/corp 隔离模式：服务端从 `DashboardPrincipal` 取作用域，mutation body 不接收 tenant/corp/actor；Provider 状态 API 沿用这个模式。

## 设计

### 1. 统一 Provider 模型与注册表

在 `internal/modules/providers` 内扩展公共模型：

- `State` 继续只允许 `ready`、`limited`、`unavailable`。
- `Source` 明确为 `external`、`simulated`、`local`、`code_only`；`external` 只表示真实外部 Provider 适配器，是否已配置和最近是否成功由状态字段表达。
- `Status` 增加稳定 `Code`、`Source`、`Action`、`Capabilities`、缺失配置标识和最近成功/失败时间/错误码。状态消息不放 secret、token、私钥、请求体或原始外部响应。
- `Registry` 只接受带有分类的 `Registration`，拒绝空 kind、未知 source、空 capabilities 或重复 kind；快照按 kind 排序，返回副本，避免调用方篡改注册表。
- `StatusProvider` 仍可由现有本地音频、OpenAI-compatible AI、archive adapter 实现；所有内置 Provider 在源码 completion gate 中必须有明确分类。

Provider status API 不接收租户、企业或 actor 参数。HTTP handler 只接受认证上下文中的 `DashboardPrincipal`，把 tenant/corp 传给注入的 `Resolver`；resolver 返回当前 principal 可见的 Provider 状态。superadmin 可以看到诊断字段，普通用户只得到页面/能力相关状态；当前 milestone 先把敏感字段物理排除在 `Status`/JSON 合同外，并以 resolver contract 测试锁住不可越权。

### 2. WeCom archive truthful limited

归档 Provider 的 `Status` 在凭据缺失时为 `limited` + `archive.credentials_missing`；凭据齐全但未实现真实 `getchatdata` 时仍为 `limited` + `archive.getchatdata_unimplemented`，`Action` 指向配置/接入真实归档 source。`Sync` 在两种情况下都 fail closed，分别返回 `ErrNotConfigured` 或稳定的 `ErrCapabilityUnavailable`，不返回 activation pending 文本作为业务契约，也不产生成功计数。

模拟归档 source 不在本批次注册为真实 archive Provider；已有 simulator 继续保持独立 namespace/source。以后接入真实付费 source 时只替换 archive source/持久化实现，不改变 Dashboard、权限和报表消费者的 Provider 状态合同。

### 3. WeCom standard sync contract

当前真实 `RoomWelcomeWeComClient` 是标准同步的运行时组件。新增的合同测试使用 `httptest.Server` 验证：先以 `corpid/corpsecret` 请求 `cgi-bin/gettoken`，再携带 token 请求 `cgi-bin/department/list` 与 `cgi-bin/user/list`；成功响应映射到部门/员工模型，WeCom 非零 `errcode`、非 2xx、缺 token 均转换为不泄漏 secret 的稳定错误类别。员工同步仍走现有 queue/worker 状态：`queued`、`running`/`syncing`、`succeeded`/`completed`、`failed`，请求方只触发当前 principal 的企业同步。

状态层将标准 WeCom 标记为 `external`；无企业凭据为 `limited`，有凭据但未验证为 `limited`，现有 verification 成功后才允许 `ready`。Archive Secret/RSA 不参与标准员工同步 ready 判定。

### 4. Dashboard 状态 API/UI

新增 `GET /dashboard/providers/status`，返回统一 envelope 中的 `providers` 数组与 scope-safe freshness。handler 认证失败返回 `SESSION_INVALID`，租户作用域错误 fail closed，resolver 错误返回稳定 machine code；不允许 query/body 覆盖 principal 的 tenant/corp/actor。Dashboard 增加轻量 Provider status API 与状态卡片，显示状态、来源、缺失配置/下一步动作、最近成功/失败和错误码；不显示 secret。普通页面只渲染 API 返回的可见状态，superadmin 诊断能力由服务端 resolver 决定。

## 错误与安全

- 所有可操作失败都使用 machine code，例如 `provider.credentials_missing`、`provider.capability_unavailable`、`provider.external_failed`、`provider.scope_denied`、`provider.internal`。
- 外部 HTTP 错误仅保留 HTTP 类别或 WeCom `errcode` 的非敏感映射，不记录 URL query 中的 token/secret，不把外部 body 原文返回 Dashboard。
- handler 的作用域只来自 `DashboardPrincipal`；body、query、header 中的 tenant/corp/actor 不是授权事实。
- Provider 状态是只读 API；不因读取状态触发外部测试请求，不消耗企业微信或 AI 配额。

## 验收

1. Provider registry 单元测试覆盖来源校验、重复注册、排序和状态 JSON 不含 secret。
2. archive 单元测试证明四凭据齐全时仍 `limited`，并证明 `Sync` fail closed。
3. WeCom fake HTTP server 合同测试覆盖 token、部门、员工、WeCom error、缺凭据与请求 query。
4. Provider HTTP handler 测试覆盖认证、tenant/corp scope、superadmin/普通用户可见性和禁止 body scope 注入。
5. Dashboard typecheck、lint、tests、build 通过；Go 运行相关包测试及既有全量测试并分别记录基线迁移期望失败。
6. `scripts/check_provider_completion.mjs` 解析真实源码注册/分类，不接受注释、fixture 或字符串自证；未分类 Provider 必须失败，archive 不得以 `ready` 通过。

## 明确未完成项

真实企业微信凭据、真实回调、真实付费会话存档 `getchatdata`、对象存储 S3 adapter、六个营销页面外部闭环、支付/短信/邮件/域名 Provider 均不在本批次完成声明内；没有 live 证据的能力继续显示 `limited`/`unavailable`。
