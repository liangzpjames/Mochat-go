# Task 10 独立审查整改报告

日期：2026-08-30
整改基线：`a0b7d127ce6b69d375f5c115e8bf1f39865b98e3`

## 1. 结论

Task 10 审查提出的 I1、I3、M1、M2 已按生产调用链闭环；I2 中属于 current Store 合同的 callback inbox、archive source read、archive sync 已改为 `integrationtestdb + production registry latest + scenario seed`，并增加静态合同拒绝这些 current fixture 手写业务 `CREATE TABLE` 或标准 migration ledger。

权威 fresh-split 证据不再由 generic `testharness.ApplyLatest` 冒充。新的权威测试位于 `cmd/mochat-identity-migrate/registry_integration_test.go`，从真正空 schema 开始，完整复用正式 SaaS Bootstrap/ChangePassword、identity CLI、0165 controller 与普通 runner。generic helper 保留为明确标注的 legacy platform scenario seed，只服务既有场景，不作为 fresh-split 权威证据。

MariaDB 10.6、原生 MySQL 5.7 的完整 `internal/store`、`internal/migration` 均为 PASS；新增 command full-registry 集成在两种数据库也均为 PASS。无 DSN Go test、Go vet、静态合同与 diff check 均为 PASS。

## 2. I3：MFA 错误分类

### 根因

`CompleteMFAChallenge` 把 challenge/credential 的所有 `QueryRowContext` 错误折叠为 `ErrMFAChallengeInvalid`，导致 HTTP 无法区分业务竞争与数据库故障；`FindMFAChallenge` 的依赖错误也被 HTTP 统一返回 401。

### 整改

- 只有 `sql.ErrNoRows`、状态转换拒绝、TOTP step 重放、版本/受影响行数竞争映射为 `ErrMFAChallengeInvalid`。
- 连接、超时、驱动、`RowsAffected` 等基础设施错误原样上浮。
- HTTP Find/Complete 对依赖故障返回 503 `AUTH_UNAVAILABLE`。
- Find/Complete 依赖故障均断言 `RecordMFAFailure` 调用次数为 0。
- sqlmock 覆盖数据库读取故障与 `sql.ErrNoRows` 的分类边界；真实 MariaDB/MySQL 完整 store suite 覆盖正式 persistence/schema 合同。

RED：challenge 读取故障实际得到 `saas MFA challenge invalid`；Find 依赖故障实际返回 401。
GREEN：聚焦 store、saasauth 测试通过，随后两种数据库完整 store suite 通过。

## 3. I1：fresh full-registry 与 generic helper 边界

### 权威 empty-schema 闭环

`TestFullRegistryFromEmptySchemaViaProductionAPIs` 只用 `integrationtestdb.NewIsolated` 创建/删除隔离 database，不手写任何业务 DDL 或 migration ledger：

1. ordinary `Runner.Apply` 精确 fail closed 于 0130；
2. 正式 `SaaSIdentityStore.Bootstrap` 创建 bootstrap root，正式 `ChangePassword` 完成密码轮换；
3. 0130 使用独立 request/confirmation 调用正式 `runMigration("up")` 与 `encrypt-credentials`；
4. ordinary runner 精确 fail closed 于 0131；
5. 0131 使用另一独立 request/confirmation 调用正式 `runMigration("cutover")`；复用 0130 confirmation 必须拒绝；
6. ordinary runner 精确 fail closed 于 0165；
7. 0165 依次执行 Inventory、Backup、Preflight、Apply、Verify，并验证空集合 digest 非空；
8. 最终动态 inventory、`StatusReadOnly`、逐项 checksum、ledger 行数完全一致；
9. 0130、0131 相同 request 重放均幂等，不新增伪 evidence。

### generic helper

- `ControlledEvidence` 改为 `IdentityBackfillRequestID` 与 `IdentityCutoverRequestID`，分别生成 `-0130`、`-0131` request。
- `ApplyLatest` 在 controlled version 已完整登记时，通过 ordinary runner 重新校验 checksum/baseline evidence 后继续，不再重复插入 legacy platform tenant。
- 源码注释明确：该 helper 是 legacy platform scenario seed，不是 authoritative fresh-split evidence。

## 4. I2：DSN fixture 审计与整改边界

### 已整改的 current Store 合同

- callback inbox：`newWeWorkCallbackInboxIntegrationStore` 改为 `newCurrentStoreIntegrationDB`，只 seed tenant/corp/binding。
- archive source read：两个读取合同改为 latest registry，只 seed tenant/corp/binding、员工、客户和消息行；删除员工、客户、群、十个消息分表的手写 DDL。
- archive sync：原子 upsert、lease fencing、并发 message identity 三个 current Store 合同改为 latest registry，只 seed 场景业务行；事务 trigger 仍是明确的故障注入对象，不是业务 schema fixture。
- 新增 AST 静态合同，锁定上述 current helper/test；发现业务 `CREATE TABLE` 或对 `mochat_go_schema_migrations` 的 INSERT/UPDATE/DELETE/DROP/CREATE 即失败。

### 保留但不作为 current/fresh 权威证据的历史 fixture

下列手写对象保留，因为它们本身就是被测历史状态、故障注入状态或 migration down/recovery 边界；本次没有把它们伪装成 fresh/full-registry 证据：

- `internal/migration/identity_realms_single_corp_backfill_integration_test.go`：验证 0130 历史 legacy 数据映射、部分 DDL、down/reapply，以及“identity evidence 已提交但标准 ledger 尚未恢复”的故障恢复窗口。其手写 ledger 只用于构造该故障窗口；权威 full-registry 已由 command 新测试取代。
- `internal/migration/wecom_capability_ledger_contract_test.go`：外部 FK probe、无效 schema、down/rollback 与残余 ledger 故障注入是测试对象本身。
- `internal/migration/archive_runner_integration_test.go`、`dashboard_page_rbac_integration_test.go`：验证指定历史 migration 在缺表、预存在目标表和 down/reapply 状态下的行为，不属于 current Store fixture。
- `internal/store/archive_sync_integration_test.go` 中其余手写表：用于 0138 migration guard 的缺表/错表/残余 FK/审计失败注入；current Store 的三条正式写入合同已迁至 latest helper。
- `cmd/mochat-identity-migrate/main_integration_test.go`：保留 CLI 历史 down/restore/reapply 场景；不再承担 authoritative empty-schema full-registry 声明。

这些保留项不能用于证明生产 fresh install；本报告的权威证据只指向新增 empty-schema command test。静态门禁当前聚焦 current Store fixture，避免误拒绝上述明确的 migration/failure probe。

## 5. M1：0175 真实 lifecycle

新增真实数据库集成测试，执行：

`0174 production prefix -> 0175 up -> RollbackLast -> 0175 reapply`

每一步核对：

- `batch_title` 的 `varchar(100)`、`utf8mb4_unicode_ci`、`NOT NULL`、空字符串 default；
- down 后 `information_schema.columns` 无该列；
- up/down/reapply 时标准 ledger 行存在/不存在；
- ledger checksum 与动态 `DefaultInventory` 完全一致。

MariaDB 10.6 与 MySQL 5.7 均 PASS。

## 6. M2：0131 statement index

`ApplyCutover` 改为通过 `executeCutoverStatements` 使用 `for index, statement := range statements`，并把真实 index 传给 `phaseFailureWithCause`。中段失败注入在第二条语句失败时断言 `StatementIndex == 1`，同时保留底层 cause 且不暴露到公开错误文本。

## 7. 原子提交

```text
998c749fba29dbf1c3d9947e85c5dea204cedcaf fix(saasauth): preserve MFA dependency failures
8981c720168f6185c78ada25c041eb677bab25b3 fix(migration): retain cutover statement index
bfbee46ea94c870eb2129cb25bb17e0b8866d1a1 test(migration): prove fresh full registry lifecycle
3d08a066552765ceb776fa5a79179e9982654b62 test(migration): execute contact title rollback lifecycle
7387854c3fd7082559c4121f528fd7b26c76b88d test(store): run archive contracts on current registry
```

## 8. 验证证据

### MariaDB 10.6

```text
go test ./internal/store ./internal/migration -count=1
ok  jiyi/mochat-go/internal/store      313.358s
ok  jiyi/mochat-go/internal/migration   34.676s

go test ./cmd/mochat-identity-migrate -run '^TestFullRegistryFromEmptySchemaViaProductionAPIs$' -count=1
ok  jiyi/mochat-go/cmd/mochat-identity-migrate  6.831s
```

0175 targeted：PASS，test body 5.58s。
generic ApplyLatest reentrant targeted：PASS，test body 5.54s。

### MySQL 5.7

```text
go test ./internal/store ./internal/migration -count=1
ok  jiyi/mochat-go/internal/store      529.495s
ok  jiyi/mochat-go/internal/migration  114.146s

go test ./cmd/mochat-identity-migrate -run '^TestFullRegistryFromEmptySchemaViaProductionAPIs$' -count=1
ok  jiyi/mochat-go/cmd/mochat-identity-migrate  10.470s
```

0175 targeted：PASS，test body 9.25s。

### 无 DSN

```text
go test ./internal/sqlscript ./internal/identitymigration ./internal/saasauth ./internal/integrationtestdb ./internal/migration/testharness ./internal/store ./internal/migration ./cmd/mochat-identity-migrate -count=1
PASS: 1.770s / 1.985s / 3.103s / 1.750s / 1.942s / 2.192s / 2.034s / 1.780s

go vet ./internal/sqlscript ./internal/identitymigration ./internal/saasauth ./internal/integrationtestdb ./internal/migration/testharness ./internal/store ./internal/migration ./cmd/mochat-identity-migrate
exit 0

git diff --check
exit 0
```

无 DSN 时数据库集成测试为准确 SKIP，不计为数据库 PASS；MariaDB/MySQL 证据来自上面的真实 DSN 命令。

## 9. 安全与边界

- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 未执行 reset、clean、force push、compose down 或删除命名卷。
- 未实施 Task 11。
- 数据库测试只创建并删除 `integrationtestdb` 验证过的随机独立 schema；未打印 DSN、root password 或 credential key。
