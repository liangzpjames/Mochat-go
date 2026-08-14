# Provider 基础与 WeCom 标准同步批次交付记录

## 交付范围

- 工作树：`D:\workspace\mochat-go\mochat-go\.worktrees\phase6-provider-foundation-wecom-closeout`
- 分支：`phase6/provider-foundation-wecom-closeout`
- 基线：`main` 的 `214d95b`
- 本批次建立 Provider `ready/limited/unavailable`、`external/simulated/local/code_only` 分类、统一注册表和源码级 completion gate。
- Provider 状态从 runtime 组件、认证 tenant/corp profile 和 tenant-scoped 员工同步状态读取；HTTP 只使用认证上下文 principal，不接受 query/body 的租户身份。
- 普通用户只获得稳定 machine code、能力和时间字段；配置缺失项、诊断原因和敏感值不下发。superadmin 才可见受控诊断字段。
- WeCom 会话存档在真实 `getchatdata` source 未实现前固定为 `limited`；凭据齐全也只返回 `ErrCapabilityUnavailable`，未声称 archive 完成。
- WeCom 标准员工/部门同步复用现有 HTTP client，使用 fake HTTP server 合同验证 token、部门、员工请求及错误码脱敏。
- Dashboard 在企业设置页展示 Provider 运行状态、状态码、能力、下一步动作和重试入口；不触发外部测试请求。

## 提交

1. `a277a8a` `docs(provider): define foundation and WeCom sync milestone`
2. `294f3ba` `feat(provider): add classified status registry`
3. `53142ee` `feat(provider): close WeCom archive and standard sync contract`
4. `0bc9e93` `feat(provider): expose scoped runtime status`
5. `4f5d557` `fix(provider): harden status evidence and completion gate`
6. `fd734a1` `fix(provider): bind completion gate to registry factory`
7. `4b28c15` `fix(provider): audit WeCom runtime adapter`
8. `204d109` `fix(provider): close completion gate bypasses`
9. `09b04f8` `fix(provider): constrain registry completion source`

## 验证结果

通过：

- `go test ./internal/companyprofile ./internal/providerstatus -count=1`
- `go test ./internal/modules/providers/... -count=1`
- `go test ./internal/dashboard -run 'TestRoomWelcomeWeComClientStandard.*Contract|TestRoomWelcomeWeComClientStandardSyncError|TestVerifyCompany|TestRoomWelcomeWeComClientStatusComesFromRuntimeComponent' -count=1`
- `node --test scripts/check_provider_completion.test.mjs`：4 个测试通过，包含无注册、注释/testdata 自证、deadRegistry 和 `NewRegistry` 不可达分支坏 fixture。
- `node scripts/check_provider_completion.mjs --root .`：`ok: true`；实际发现并分类 `ai`、`wecom_archive`、`audio_storage`、`wecom_standard`。
- `corepack pnpm --filter @mochat/dashboard typecheck`：通过。
- `corepack pnpm --filter @mochat/dashboard lint`：通过。
- `corepack pnpm --filter @mochat/dashboard test`：96 个测试文件、598 个测试通过。
- `corepack pnpm --filter @mochat/dashboard build`：通过。
- `git diff --check`：通过；最终工作树 clean。

`go test ./... -count=1` 仍以非零退出，且只观察到基线已有的 3 项失败，未修改其涉及的迁移常量：

1. `internal/dashboard/TestSaaSAdminSystemHealthMigrationExpectationMatchesRelease`：期望迁移数 `133`，实际发现 `137`。
2. `internal/migration/TestStandaloneComposeFreshInitUsesSchemaForCorpDataIndexes`：最新迁移为 `0137_reconcile_ai_settings_schema`，测试仍期望 `0133_archive_simulation_registry`。
3. `internal/migration/TestPhase35OrderProductizationMigrationIsForwardOnly`：最新迁移为 `0137_reconcile_ai_settings_schema`。

## 外部阻塞与未宣称范围

- 没有真实企微付费会话存档凭据或 live `getchatdata` 验收；ArchiveSource、持久化游标、幂等、重试、审计及多媒体 archive fixture 不在本批次完成范围。会话存档仍是 `limited`。
- 没有执行 live WeCom 请求；标准同步仅完成可注入 client、fake HTTP 合同、认证租户隔离和稳定错误码。真实账号闭环仍需后续凭据验收。
- 未运行服务器、未连接生产数据库、未操作 Docker、未删除卷、未执行 `down -v`/`volume rm`/`system prune`。

## 隔离核对

主工作区原有 dirty 文件未被覆盖；本分支相对基线未修改 `deploy/standalone/migrations` 或 `internal/migration`。与主工作区 dirty 文件无重叠。等待主任务独立代码审阅与最终验收。
