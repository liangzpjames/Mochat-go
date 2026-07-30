# Phase 2.2 后端模块化与质量门禁验收

## 结论

- 验收时间：2026-07-30T13:46:30+08:00
- 分支：`phase2.2-backend-quality-gates`
- 被验证源码提交：`8524164fd105e00882d7199b4260dcee48556241`
- Phase 3 backend ready: **no**

所有后端功能门禁均已通过原生命令或计划允许的 Linux Docker 等价命令，但工作树清洁度条件尚未满足：Task 8 开始前已经存在其他任务产生的未跟踪 `.superpowers/sdd/*` 报告，本任务还必须生成 `task-8-report.md`。这些协调报告不在 Task 8 计划提交范围内，不能擅自删除或混入验收文档提交。因此本阶段保持“实施中”，待协调层妥善归档这些报告并取得空的 `git status --short` 后才能改为完成。

## 验收环境

| 项目 | 实际值 |
| --- | --- |
| 主机 OS | Microsoft Windows 11 专业版 10.0.26200，64-bit |
| 主机 Go | `go1.26.5 windows/amd64` |
| Linux race 容器 | `golang:1.26.5`，Linux amd64 |
| Linux migration smoke 容器 | Alpine Linux，`go1.26.3 linux/amd64` |
| MySQL integration | MySQL `5.7.44`, Linux x86_64 |
| Migration smoke 数据库 | MariaDB 10.6（`deploy/standalone/docker-compose.yml`） |
| Docker | Server 29.6.2，Compose v5.3.1 |

## 最终命令证据

| 命令 | 退出码 | 结果与说明 |
| --- | ---: | --- |
| `git status --short` | 0 | 不满足内容预期；除本次文档外，还列出既有未跟踪 `.superpowers/sdd/*` 报告。 |
| `go run ./cmd/mochat-architecture -root .` | 0 | 输出 `architecture boundaries passed`。 |
| `go test ./internal/architecture/... ./internal/app/modules/... ./internal/modules/...` | 0 | architecture、模块注册、example 和 SCRM 各层全部通过。 |
| `go test -race ./internal/modules/...`（Windows） | 1 | 如实失败：`-race requires cgo; enable cgo by setting CGO_ENABLED=1`。 |
| `docker run --rm -v "${PWD}:/src" -w /src golang:1.26.5 go test -race ./internal/modules/...` | 0 | 计划允许的 Linux Docker 等价 race 门禁；SCRM module/application/domain/MySQL adapter/HTTP transport 全部通过。 |
| `go test ./...` | 0 | 全部 Go package 通过。 |
| `go vet ./...` | 0 | 无 vet finding。 |
| `go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture` | 0 | 六个命令入口全部构建成功。 |
| `go test -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql` | 0 | 在独立 MySQL 5.7.44 容器、真实 0098 migration 上执行；输出 `ok ... 0.840s`，stack、network 和 volume 随后清理成功。`-count=1` 比计划命令更严格，避免缓存。 |
| `./scripts/smoke_schema_migrate.sh`（Windows Bash/真实 checkout） | 1/2 | WSL 无 `/bin/bash`；Linux 容器直接读取 Windows CRLF checkout 时先遇到 `pipefail\r`，仅归一化脚本后又准确暴露 schema checksum 的 CRLF/LF 差异。这些尝试未计为通过。 |
| `bash ./scripts/smoke_schema_migrate.sh`（Linux Docker 临时 LF 源副本） | 0 | 按计划允许的 Linux Docker 等价验证；输出 `schema migration smoke passed`。临时副本只做 EOL 归一化，不修改真实工作树，执行后已删除。 |

Windows checkout 的 `deploy/standalone/schema/mochat.sql` SHA-256 为 `b7dbd66b...`；Linux LF 临时副本为 `3b97cf81...`。直接用 Windows CRLF 文件运行 Linux migration smoke 会使 legacy checksum alias 与当前摘要不一致，因此验收采用 Linux LF 临时源副本模拟 CI checkout，并保留上述失败记录。

## SCRM 行为证据

真实 MySQL integration package 包含并通过以下未缓存场景：

- `TestLeadRepositoryTenantIsolation`：相同业务键在两个租户内各保留一条，列表只返回请求租户；
- `TestLeadRepositoryCreateOrGetIsIdempotent`：重复业务键返回原记录，不产生第二条；
- `TestLeadRepositoryAllowsSameBusinessKeyAcrossTenants`：租户范围内唯一，不跨租户误冲突；
- `TestLeadRepositoryListUsesStableCreatedAtAndIDOrder`：分页顺序稳定；
- `TestLeadRepositoryListIncludesMaximumMySQLTimestampOnFirstPage`：MySQL 时间边界可查询；
- `TestLeadRepositoryConcurrentCreateProducesOneRow`：并发创建最终仅一条记录；
- `TestIntegrationRepositoryPreservesOtherRowsAndCleansOwnTenant`：测试清理不会删除其他租户数据。

应用与 HTTP 单元测试还覆盖：命令先校验后调用依赖、服务端生成 ID/时间、principal 决定 tenant、客户端 tenant 字段被拒绝、严格 JSON、幂等重试、无效 cursor/page size、后端不可用映射，以及只注册 GET/POST pilot routes。

## Migration 生命周期

`scripts/smoke_schema_migrate.sh` 的 Linux Docker 等价运行完整退出 0，脚本内客观断言覆盖：

- 空库 apply 0001–0098；
- 每个 migration 的 64 位 checksum 记录；
- 0098 SCRM 表、九列结构和 `(tenant_id, business_key)` 唯一索引；
- 重复 apply 不重放；
- 0098 rollback 后表和 migration 记录消失；
- reapply/replay 后 0098 再次处于 applied 状态；
- standalone 已初始化库的 baseline 与 legacy checksum 兼容路径。

## 架构负向门禁

在 `C:\Users\lzpen\AppData\Local\Temp\mochat-task8-negative-8524164fd105e00882d7199b4260dcee48556241` 的临时 Git 源副本中新增：

```go
package domain

import "database/sql"

var _ *sql.DB
```

执行 `go run ./cmd/mochat-architecture -root <temporary-root>` 的观测结果：

- 退出码：1；
- finding：`ARCH-DOMAIN-DEPENDENCY internal/modules/scrm/domain/bad.go: imports "database/sql"`；
- 断言结果：通过；
- 临时副本：已删除；
- 真实工作树：未加入 `bad.go`。

## Pilot 与兼容性证据

- `go test -count=1 ./internal/config -run '^TestPhase22SCRMPilotDisabledByDefault$'`：退出码 0，证明 `MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT` 为空时 flag 为 false。
- `go test -count=1 ./internal/server -run '^(TestUnmatchedModuleRouteKeepsLegacyHealthRoute|TestLoginShowUsesMigratedHandlerWhenConfigured|TestWorkFissionRoutesUseMigratedHandlersWhenConfigured)$'`：退出码 0，证明 module router 不遮蔽 legacy `/healthz`，并证明代表性 migrated login/work-fission routes 仍由 Go handler 承接。
- Phase 2.2 `GET/POST /api/phase2-2/scrm/leads` 只是默认关闭的 pilot 验证面，不是 Phase 3 正式 API 契约。Phase 3 计划已明确正式契约以其任务 1 和任务 3 为准。

## Phase 3 计划修订证据

- `internal/store/scrm.go`、`internal/dashboard/scrm.go`、`internal/dashboard/scrm_metrics.go` 及对应 legacy test/glob 已从 Phase 3 计划清零；
- 后端实现改为 `internal/modules/scrm/domain`、`application`、`ports`、`adapters/mysql`、`transport/http` 的任务特定文件；
- architecture CLI 命令已加入 Phase 3 后端任务 2、3、6，以及任务 7 总发布门禁。

## 剩余债务与阻塞

- 必须由协调层处理未跟踪 `.superpowers/sdd/*` 报告并重新取得空的 `git status --short`；这是当前唯一阻止 Phase 3 backend ready 变为 yes 的验收项。
- 计划期望“八个 Phase 2.2 commits”，实际为了修复 review 发现形成了更多可审阅的小提交；应由集成者决定是否保留历史或在合并时整理。
- `internal/dashboard`、`internal/store`、`internal/server` 和 `cmd/mochat-go/main.go` 的存量巨型遗留代码仍在，Phase 2.2 只阻止新增 SCRM 业务继续进入这些位置。
- Windows 原生 race 仍依赖可用的 CGO 工具链；当前以 Linux Docker race 作为可重复门禁。
- Windows checkout 的 CRLF 会使 migration checksum smoke 失真；CI/Linux checkout 是权威执行面，后续可通过明确 `.gitattributes` EOL 策略降低本地摩擦。

## 验收清单

- [x] 架构正向门禁通过；
- [x] 架构负向门禁按 exit 1 和 finding code 失败；
- [x] 聚焦、全量、vet 和 build 门禁通过；
- [x] Linux race 等价门禁通过；
- [x] MySQL 5.7 tenant/isolation/idempotency/concurrency integration 通过；
- [x] migration apply/checksum/rollback/replay 通过；
- [x] pilot 默认 false 有独立测试证据；
- [x] legacy `/healthz` 和代表性 migrated routes 有独立测试证据；
- [x] Phase 3 legacy SCRM 后端路径和 pilot 契约说明已修订；
- [ ] 提交后 `git status --short` 为空。
