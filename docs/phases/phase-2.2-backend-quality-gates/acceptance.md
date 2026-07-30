# Phase 2.2 后端模块化与质量门禁验收

## 结论

- 验收时间：2026-07-30T14:13:33+08:00
- 分支：`phase2.2-backend-quality-gates`
- 被验证源码提交：`3e9d91eb00ee0c1f255e1e5cd851c1509091e6d8`
- 被验证 Git tree：`4f50a2a7aa90fdc05ab9dd58b742629422c2ee12`
- Git archive SHA-256：`e733f1e4085da5d013cbfef73800b912f82d6b45ad11c9f20a0e3dc74fdb2476`
- Phase 3 backend ready: **yes**

架构、单元、全量、race、真实 MySQL 5.7 integration、migration 生命周期、负向架构、pilot 默认关闭和 legacy route 兼容性门禁均有客观通过证据。`.superpowers/` 已明确作为 SDD scratch 加入 `.gitignore`；源码证据提交后的 `git status --short` 输出为空。

## 验收环境

| 项目 | 实际值 |
| --- | --- |
| 主机 OS | Microsoft Windows 11 专业版 10.0.26200，64-bit |
| 主机 Go | `go1.26.5 windows/amd64` |
| Linux race/integration 容器 | `golang:1.26.5`，Linux amd64 |
| Linux migration smoke 容器 | Alpine Linux，`go1.26.3 linux/amd64` |
| MySQL integration | MySQL `5.7.44`, Linux x86_64 |
| Migration smoke 数据库 | MariaDB 10.6（`deploy/standalone/docker-compose.yml`） |
| Docker | Server 29.6.2，Compose v5.3.1 |

## 精确 Git tree 与 LF 容器输入

Windows checkout 使用 CRLF/mixed EOL，不能作为 Linux migration checksum 的权威字节输入。验收从精确源码提交构造 Git archive，再只在 Docker VM 临时副本执行 `dos2unix`；真实工作树不参与 integration 或 migration smoke。

实际执行的 PowerShell + Docker 源码准备命令：

```powershell
$commit = '3e9d91eb00ee0c1f255e1e5cd851c1509091e6d8'
$archive = Join-Path $env:TEMP "mochat-phase22-$commit.tar"
$source = "/tmp/mochat-phase22-evidence-$commit"

git -c core.autocrlf=false archive --format=tar --output=$archive $commit
Get-FileHash -Algorithm SHA256 -LiteralPath $archive

$resolvedArchive = (Resolve-Path -LiteralPath $archive).Path
docker run --rm `
  -v "${resolvedArchive}:/archive.tar:ro" `
  -v "${source}:/source" `
  alpine:3.23 tar -xf /archive.tar -C /source

docker run --rm -v "${source}:/source" alpine:3.23 sh -lc @'
find /source -type f -name '*.sh' -exec dos2unix '{}' '+'
find /source -type f -name '*.sql' -exec dos2unix '{}' '+'
find /source -type f -name '*.go' -exec dos2unix '{}' '+'
find /source -type f -name '*.yml' -exec dos2unix '{}' '+'
find /source -type f -name '*.yaml' -exec dos2unix '{}' '+'
dos2unix /source/go.mod /source/go.sum
'@
```

归一化后的关键文件原始 SHA-256 输出：

```text
3b97cf81629ce945e4df9dbb10bb2afc5bc65712e289ab1f66f696f748225a09  deploy/standalone/schema/mochat.sql
41c4ced7f3487b6fb49176a5573aa2bbec5dcd4e6da8807a2110f26cb3a1b3f9  deploy/standalone/migrations/0098_scrm_lead_foundation.up.sql
5461cd3b85fc23b258e90be1798ba2734017aef31e4bde710829d60776fae99d  scripts/smoke_schema_migrate.sh
466ca5b857374a5bebe46c6e78868da9e74441fc4c111ce44778b19e3cf067e8  internal/modules/scrm/adapters/mysql/integration_requirement_test.go
```

## 最终命令证据

| 命令 | 退出码 | 结果 |
| --- | ---: | --- |
| `git status --short`（源码证据提交后） | 0 | 输出为空。 |
| `go run ./cmd/mochat-architecture -root .` | 0 | `architecture boundaries passed`。 |
| `go test ./internal/architecture/... ./internal/app/modules/... ./internal/modules/...` | 0 | architecture、模块装配和 SCRM 各层通过。 |
| `docker run ... golang:1.26.5 go test -race ./internal/modules/...` | 0 | 精确 Git tree 的 Linux race 门禁通过。 |
| `go test ./...` | 0 | 全部 Go package 通过。 |
| `go vet ./...` | 0 | 无 finding。 |
| `go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture` | 0 | 六个命令入口构建成功。 |
| `MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql` | 0 | 精确 Git tree、MySQL 5.7.44、真实 0098 migration；无 skip。 |
| `bash ./scripts/smoke_schema_migrate.sh` | 0 | 精确 Git tree 的 Linux LF 容器输入；输出 `schema migration smoke passed`。 |
| `go test -count=1 ./internal/config -run '^TestPhase22SCRMPilotDisabledByDefault$'` | 0 | pilot flag 默认 false。 |
| legacy `/healthz` 与代表性 migrated routes 定向测试 | 0 | module router 未遮蔽 health route，login/work-fission migrated handlers 正常。 |
| backend quality gate contract | 0 | CI require env、verbose uncached integration command、数据库 lifecycle 均被 contract 固定。 |

Windows 原生 `go test -race` 因当前 `CGO_ENABLED=0` 报 `-race requires cgo`；Linux CI/容器是 race 的权威执行面。Windows checkout 的 CRLF/mixed EOL 也不是 migration checksum 的权威输入；Linux Git archive/LF 容器结果为准。

## MySQL integration 防 silent skip

TDD RED：

- 新增 resolver 测试后，首次执行因 `resolveMySQLIntegrationDSN` 未定义而 build failed；
- 新增 CI contract 断言后，首次执行报告缺少 `MOCHAT_REQUIRE_MYSQL_INTEGRATION: "1"` 和 verbose uncached 命令。

GREEN：

- `TestResolveMySQLIntegrationDSNRequiresDSNWhenStrict`、`AllowsDeveloperSkip`、`UsesConfiguredDSN` 全部通过；
- require=1 且 DSN 缺失时，integration test exit 1，并输出 `MOCHAT_MYSQL_DSN is required when MOCHAT_REQUIRE_MYSQL_INTEGRATION=1`；
- 普通开发模式无 DSN 时仍明确 SKIP、exit 0；
- CI atomic step 设置 `MOCHAT_REQUIRE_MYSQL_INTEGRATION: "1"` 并执行 `go test -v -count=1 -tags=integration ...`。

实际 MySQL 5.7 PowerShell + Docker 命令（`$source` 来自上一节）：

```powershell
$project = 'mochat-phase22-mysql57-3e9d91e'
$compose = 'deploy/mysql57/docker-compose.yml'
$env:MOCHAT_MYSQL57_PORT = '13333'
$dsn = 'mochat:mochat_pass@tcp(127.0.0.1:13333)/mochat?parseTime=true&loc=UTC'

try {
  docker compose -p $project -f $compose up -d mysql57
  $container = (docker compose -p $project -f $compose ps -q mysql57).Trim()
  $deadline = (Get-Date).AddSeconds(240)
  do {
    $health = (docker inspect --format '{{.State.Health.Status}}' $container 2>$null).Trim()
    if ($health -eq 'unhealthy') { throw 'mysql57 unhealthy' }
    if ($health -ne 'healthy') { Start-Sleep -Seconds 2 }
  } while ($health -ne 'healthy' -and (Get-Date) -lt $deadline)
  if ($health -ne 'healthy') { throw "mysql57 health timeout: $health" }

  docker run --rm --network host `
    -e "MOCHAT_MYSQL_DSN=$dsn" `
    -e MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 `
    -v "${source}:${source}" -w $source `
    golang:1.26.5 go run ./cmd/mochat-migrate `
    -dsn $dsn -project-root . -action apply

  docker compose -p $project -f $compose exec -T mysql57 `
    mysql -umochat -pmochat_pass -N -B mochat `
    -e "SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version = '0098_scrm_lead_foundation'"

  docker run --rm --network host `
    -e "MOCHAT_MYSQL_DSN=$dsn" `
    -e MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 `
    -v "${source}:${source}" -w $source `
    golang:1.26.5 go test -v -count=1 -tags=integration `
    ./internal/modules/scrm/adapters/mysql
} finally {
  docker compose -p $project -f $compose down -v --remove-orphans
}
```

真实 MySQL 5.7.44 原始 RUN/PASS 证据：

```text
MYSQL_HEALTH=healthy
MIGRATION_0098_COUNT=1
=== RUN   TestLeadRepositoryTenantIsolation
--- PASS: TestLeadRepositoryTenantIsolation (0.00s)
=== RUN   TestLeadRepositoryCreateOrGetIsIdempotent
--- PASS: TestLeadRepositoryCreateOrGetIsIdempotent (0.01s)
=== RUN   TestLeadRepositoryAllowsSameBusinessKeyAcrossTenants
--- PASS: TestLeadRepositoryAllowsSameBusinessKeyAcrossTenants (0.00s)
=== RUN   TestLeadRepositoryListUsesStableCreatedAtAndIDOrder
--- PASS: TestLeadRepositoryListUsesStableCreatedAtAndIDOrder (0.01s)
=== RUN   TestLeadRepositoryListIncludesMaximumMySQLTimestampOnFirstPage
--- PASS: TestLeadRepositoryListIncludesMaximumMySQLTimestampOnFirstPage (0.00s)
=== RUN   TestLeadRepositoryConcurrentCreateProducesOneRow
--- PASS: TestLeadRepositoryConcurrentCreateProducesOneRow (0.01s)
=== RUN   TestIntegrationRepositoryPreservesOtherRowsAndCleansOwnTenant
--- PASS: TestIntegrationRepositoryPreservesOtherRowsAndCleansOwnTenant (0.01s)
PASS
ok  	jiyi/mochat-go/internal/modules/scrm/adapters/mysql	0.040s
MYSQL57_VERBOSE_GATE_EXIT_CODE=0
MYSQL57_CLEANUP_EXIT_CODE=0
```

## Migration 生命周期

精确 Git tree smoke 的实际 Docker 命令：

```powershell
docker run --rm --network host `
  -e MOCHAT_STACK_PROJECT=mochat-phase22-schema-3e9d91e `
  -e MOCHAT_MYSQL_PORT=13331 `
  -v /var/run/docker.sock:/var/run/docker.sock `
  -v "${source}:${source}" `
  -w $source `
  docker:29-cli sh -lc `
  "apk add --no-cache bash go python3 lsof >/dev/null &&
   go version &&
   sha256sum deploy/standalone/schema/mochat.sql deploy/standalone/migrations/0098_scrm_lead_foundation.up.sql scripts/smoke_schema_migrate.sh &&
   bash ./scripts/smoke_schema_migrate.sh"
```

关键原始输出：

```text
go version go1.26.3 linux/amd64
3b97cf81629ce945e4df9dbb10bb2afc5bc65712e289ab1f66f696f748225a09  deploy/standalone/schema/mochat.sql
41c4ced7f3487b6fb49176a5573aa2bbec5dcd4e6da8807a2110f26cb3a1b3f9  deploy/standalone/migrations/0098_scrm_lead_foundation.up.sql
5461cd3b85fc23b258e90be1798ba2734017aef31e4bde710829d60776fae99d  scripts/smoke_schema_migrate.sh
schema migration smoke passed
EXACT_TREE_SCHEMA_SMOKE_EXIT_CODE=0
```

脚本内断言覆盖空库 apply 0001–0098、64 位 checksum、0098 表/列/唯一索引、重复 apply、0098 rollback、reapply/replay、baseline 和 legacy checksum 兼容路径。

## 架构、Pilot 与兼容性

- 正向 architecture CLI exit 0。
- 临时副本 `domain/bad.go` 导入 `database/sql` 时，CLI exit 1 并输出 `ARCH-DOMAIN-DEPENDENCY`；临时副本已删除，真实工作树无 `bad.go`。
- pilot 默认 false 的定向 config test exit 0。
- legacy `/healthz`、migrated login 和 work-fission routes 定向 server tests exit 0。
- Phase 2.2 `GET/POST /api/phase2-2/scrm/leads` 是默认关闭的 pilot，不是 Phase 3 正式 API 契约。

## Phase 3 计划修订

- 下一条 migration 为连续且无冲突的 `0099_scrm_customer_lifecycle`；
- legacy `internal/store/scrm*`、`internal/dashboard/scrm*` 路径为零；
- 持久化接口明确为 `ports` repositories、`adapters/mysql` implementations，由 application service 编排；
- Task 2 明确同步修改并提交 `scripts/smoke_schema_migrate.sh`，把 future latest/count 推进到 0099/99，覆盖 0099 apply/checksum/rollback/replay，并保留 0098 历史语义；
- Task 6 明确修改并提交 `transport/http/routes.go`，且要求模块级 metrics route registration test；
- 每个后端任务包含 architecture CLI；
- Task 7 显式包含 race、require MySQL integration 和 migration smoke。

## 剩余债务

- `internal/dashboard`、`internal/store`、`internal/server` 和 `cmd/mochat-go/main.go` 的存量巨型 legacy 代码仍在；Phase 2.2 已阻止新增 SCRM 业务进入这些位置。
- Windows 本地 race 仍需 CGO 工具链；Windows checkout EOL 策略仍可通过 `.gitattributes` 进一步统一，但不影响 Linux 权威门禁。
- review-fix commits 是正常、可审阅的修复历史，不是验收阻塞。

## 验收清单

- [x] 架构正向与负向门禁；
- [x] 聚焦、全量、vet 和 build；
- [x] 精确 Git tree Linux race；
- [x] MySQL 5.7 七场景明确 RUN/PASS，require 模式禁止 CI silent skip；
- [x] migration apply/checksum/rollback/replay；
- [x] pilot 默认 false；
- [x] legacy `/healthz` 与代表性 migrated routes；
- [x] Phase 3 0099、同步 lifecycle smoke、分层路径、Task 6 route test 和 Task 7 完整门禁；
- [x] `.superpowers/` scratch 被忽略，`git status --short` 为空。
