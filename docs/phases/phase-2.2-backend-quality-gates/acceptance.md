# Phase 2.2 后端模块化与质量门禁验收

## 结论

- 验收日期：2026-07-30
- 分支：`phase2.2-backend-quality-gates`
- 修复基线：`45cc401cdecd9477593bec9444461c901cd42af1`
- 被验证源码提交：`7f243043d7599d0e7e2e811d9d5a976cc2620984`
- 被验证 Git tree：`b3846dc5bb6ff86f4b20aedcd44e58696ef0727f`
- 原始 Git archive SHA-256：`9efc7a5a6f97e1cc3df50f17c121c8aaaa5d8b6a1990f0852a5cf4705bc9e5a2`
- 原始 Git archive 大小：`40960000` bytes
- Phase 3 backend ready: **yes**

最终审查指出的四个 Important finding 和一个 Minor finding 均已通过 TDD 修复。架构规则、fail-closed 治理、CI 生命周期、Phase 3 真实数据库计划、typed-nil 防御，以及全量、race、真实 MySQL 5.7、migration lifecycle 都有重新执行的通过证据。

## 最终修复提交

| 提交 | 内容 |
| --- | --- |
| `fe46ce2` | 架构审计改为逐层 allowlist；受保护文件和模块登记 fail-closed。 |
| `920ebf0` | CI 强制 vet、完整 migration lifecycle、strict integration 和可靠 contract；Phase 3 计划绑定真实 MySQL。 |
| `60aeea1` | Router/Server 统一拒绝或安全处理 typed-nil HTTP 边界。 |
| `7f24304` | 未知模块子目录 fail-closed；使用真实 YAML loader 将门禁绑定到唯一 job；计划变更触发 CI。 |

提交范围：`45cc401..7f24304`。

## 架构门禁

- domain、ports、application、adapters、transport、module 使用显式 allowlist。
- 标准库归属由 `go/build` 和 GOROOT 判定；`acme/sdk` 等无点第三方路径不会被误判。
- 共享基础设施只允许精确公共契约；精确例外不会扩散到相邻包。
- 负向 fixture 覆盖 domain→own ports、domain/application→内部基础设施、application→跨模块 domain/ports、domain→第三方库。
- 未知模块子目录不会继承 composition 权限；仅模块根文件可使用 module layer，adapter、transport、module composition 也有直接负向 fixture。
- 受保护文件缺失返回 `ARCH-PROTECTED-FILE-MISSING`。
- `internal/modules/<name>` 与 production/example policy 双向核对；未登记、幽灵、重复、重叠模块都失败。

实际结果：

```text
$ go run ./cmd/mochat-architecture -root .
architecture boundaries passed

$ go test ./internal/architecture/... ./internal/app/modules/... ./internal/modules/... -count=1
PASS
```

## CI 与开发门禁

- push 与 pull request path filters 覆盖 composition root、开发检查和 lifecycle 脚本。
- workflow 明确包含并约束 architecture、race、full test、vet、migration lifecycle、strict integration 的相对顺序。
- lifecycle step 使用隔离 compose project/port，并执行权威 `scripts/smoke_schema_migrate.sh`。
- contract 使用 `go.yaml.in/yaml/v3` 真实解析 YAML，选择唯一的 `mysql57-amd64` job，拒绝重复 step name，并只在该 job 内检查 gate 顺序和命令归属。
- Phase 3 计划路径同时存在于 push、pull request filters 与 contract required paths，计划单独修改也会触发门禁。
- `scripts/test.sh` 与 `scripts/dev_check.sh` 构建六个命令入口，包括 `mochat-architecture`。
- Phase 3 Task 2 明确 tagged integration 文件、三类真实 MySQL 场景、strict uncached 命令和禁止 require-mode skip；Task 7 包含全量 test/vet/build。

实际结果：

```text
$ docker run --rm -v "${PWD}:/src" -w /src golang:1.26-bookworm \
    sh ./scripts/test_backend_quality_gate_contract.sh
backend quality gate workflow contract passed

$ ruby YAML parser .github/workflows/mysql57-amd64.yml
22 steps parsed
```

## typed-nil HTTP 边界

- `internal/nilcheck.IsNil` 安全识别 nil interface 和 nil-capable reflect kinds。
- Router 注册时拒绝 typed-nil pointer 与 typed-nil `http.HandlerFunc`。
- Server option 不保存 typed-nil router；dispatch 对 typed-nil matched handler 也安全回落。
- 普通非 nil handler/router 行为保持不变。

## 全量 Go 门禁

以下命令在源码提交 `7f24304` 上重新执行，退出码均为 0：

```text
go test ./... -count=1
go vet ./...
go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture
```

Linux race：

```text
docker run --rm \
  -v "${PWD}:/src" \
  -v mochat-go-mod-cache:/go/pkg/mod \
  -v mochat-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.26-bookworm \
  sh -c 'go test -race -count=1 ./internal/modules/...'
```

所有 `internal/modules/...` package 通过。

## 真实 MySQL 5.7 strict integration

- compose project：`mochat-go-final-review-integration`
- host port：`13333`
- 数据库：MySQL 5.7
- migration：0001–0098 已 apply；0098 row 存在且 checksum 长度为 64
- require：`MOCHAT_REQUIRE_MYSQL_INTEGRATION=1`
- 命令：`go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql`
- 结果：退出码 0，无 skip

明确 RUN/PASS 的行为包括：

- tenant isolation；
- create-or-get idempotency；
- 不同 tenant 可使用相同 business key；
- `(created_at, id)` 稳定分页；
- MySQL 最大时间戳首屏；
- concurrent create 单行结果；
- 测试 namespace 隔离 cleanup；
- malformed cursor。

finally cleanup 执行 `docker compose down -v --remove-orphans`，容器、网络和卷均删除。

## Migration apply/checksum/rollback/replay

Windows checkout 可能受全局 `core.autocrlf=true` 影响，因此权威 lifecycle 输入来自：

```powershell
git -c core.autocrlf=false archive --format=tar 7f243043d7599d0e7e2e811d9d5a976cc2620984
```

该原始 archive 的 SHA-256 和大小分别为：

```text
9efc7a5a6f97e1cc3df50f17c121c8aaaa5d8b6a1990f0852a5cf4705bc9e5a2
40960000 bytes
```

archive 解压到 Docker named volume；runner 使用 Docker daemon 实际 mountpoint，避免 Windows client bind path 与 Linux daemon path 不一致。

- compose project：`mochat-go-final-review-lifecycle`
- host port：`13331`
- 权威命令：`bash ./scripts/smoke_schema_migrate.sh`
- 关键输出：`schema migration smoke passed`
- 覆盖：空库 apply、64 位 checksum、0098 schema/index、重复 apply、0098 rollback、reapply/replay、baseline 与 legacy checksum 兼容路径
- cleanup：trap 成功，临时 stack、source volume 和 archive 均删除

## Pilot、legacy 兼容与 Phase 3

- Phase 2.2 SCRM pilot 默认关闭。
- legacy `/healthz` 和代表性 migrated login/work-fission routes 不被 module router 遮蔽。
- Phase 2.2 lead routes 仍是默认关闭的 pilot，不是 Phase 3 正式 API 契约。
- Phase 3 下一条 migration 保持为 `0099_scrm_customer_lifecycle`。
- Phase 3 新持久化行为必须进入 `ports`/`adapters/mysql`，并通过真实 MySQL tenant isolation、optimistic lock、public-pool concurrent claim integration。

## 环境与证据边界

- Windows 原生 race 受本机 CGO 工具链限制；Linux amd64 容器是权威 race 执行面。
- Windows bind mount 下直接执行 shell 脚本可能受 CRLF 影响；本次没有把失败的 `scripts/test_dev_check.sh` 本地尝试声明为通过。
- lifecycle 的首次普通 archive、daemon 不可见 bind path 和一次 `bash -lc` PATH 尝试均不计作通过证据；最终成功路径如上，并已清理所有临时资源。
- 独立最终代码审查初审指出未知模块子目录、跨 job 文本解析和计划 path filter 三项 Important；`7f24304` 修复后复审结果为 Critical/Important/Minor 均无。

## 验收清单

- [x] 架构逐层 allowlist 和精确例外；
- [x] protected file 与 module registration fail-closed；
- [x] 结构化 CI contract、vet、完整 lifecycle 和六命令 build；
- [x] Phase 3 真实 MySQL integration 计划约束；
- [x] typed-nil handler/router 防御；
- [x] 聚焦、全量、vet、build；
- [x] Linux race；
- [x] MySQL 5.7 strict integration，无 skip；
- [x] migration apply/checksum/rollback/replay；
- [x] pilot 默认关闭和 legacy route 兼容；
- [x] 精确 source commit/tree/archive 证据；
- [x] 临时 Docker 资源清理。
