# Phase 0 验证记录

> 验证时间：2026-07-26
> 分支：`phase0/modular-monolith-foundation`

## 结论

Phase 0 新增的运行角色、模块模板、架构门禁和 Docker 开发入口已通过容器化测试与构建。仓库原 `scripts/test.sh` 已在纯 Linux 容器内原样退出 0；standalone core 的组成项也全部通过。

Windows 本机已安装并验证 Go 1.26.5、Docker Desktop 与 jq。SaaS 分组前 8 项通过，但 Git Bash 调用 Windows `curl.exe` 时会破坏中文 JSON 编码，总后台后续验收必须迁到 WSL2/Linux。本记录不把 Phase 0 描述为生产 readiness，也不改变原 `ready=false`。

## 已通过

### 运行角色与配置

```text
go test ./internal/app/runtime ./internal/config
go build ./cmd/mochat-go
```

Docker 容器退出 0。覆盖默认 `all`、`api`、`worker`、`scheduler`、非法角色和后台职责过滤。

### 架构与模块

```text
scripts/audit_architecture_boundaries.sh
scripts/test_audit_architecture_boundaries.sh
go test ./internal/modules/...
```

全部退出 0。门禁自测确认 domain 违规依赖和受保护文件增长会被拒绝。

### Docker quick

在 `golang:1.26-alpine` 容器内执行：

```text
go test ./...
go vet ./...
go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance
```

容器 `phase0-quick` 最终状态为 `exited 0`；所有 Go package 通过。

### Linux 完整快速门禁与 core

在 `golang:1.26-bookworm` 的 Linux named volume 中原样执行：

```text
bash scripts/test.sh
```

退出 0。core 分组的 schema migration、standalone smoke、独立交付包、生产证据规则、MySQL/Redis stack、compose app、224/224 路由覆盖、租户 bootstrap 和队列幂等均分别退出 0。

### Windows 本地 Go

```text
go version
go vet ./...
go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance
```

Go 版本为 `go1.26.5 windows/amd64`，vet 与五个命令构建通过。`go test ./...` 在 Windows 原生文件系统上仍有 Unix 路径分隔符和权限位断言失败，因此完整测试以 Linux 容器结果为准。

### SaaS 已通过部分

- provisioning；
- tenant isolation；
- quota enforcement；
- storage reconcile；
- storage reclaim；
- usage refresh；
- alert setting dispatch；
- WeCom credential encryption/rotation/admin governance。

其中 storage reclaim 与 WeCom 凭据 smoke 已在修正 Windows 编码定位和独立 SaaS Admin SPA 入口后重新退出 0。

### 原有静态/审计门禁

以下审计在 Git Bash + bundled Python 下通过：

- standalone independence；
- acceptance suite coverage：107/107 smoke 脚本已引用；
- manifest route smoke coverage：224/224；
- functional module matrix；
- login corp validation；
- queue annotation coverage；
- wework callback event coverage；
- worker SaaS usage assertion；
- SaaS metric coverage；
- SaaS storage reclaim coverage。

### 生产证据 smoke

Git Bash 直接调用 Windows Python 时，`/tmp` 被 Python 与 shell 解析到不同位置，导致 `template-pack/index.md` 在 shell 视图中不存在。根因是跨运行时路径映射，不是证据门禁逻辑。

将同一提交复制到 Docker named volume，并在 `python:3.13-bookworm` 纯 Linux 环境执行：

```text
bash scripts/smoke_production_evidence_gate.sh
```

容器 `phase0-production-gate` 最终状态为 `exited 0`。

### 正式镜像

镜像：

```text
mochat-go:phase0
sha256:9243654785b180abf280feece66be7382060e015e0e155b765e236533fd5a387
size: 88,993,758 bytes
```

使用 `MOCHAT_GO_RUNTIME_ROLE=invalid` 启动时按预期拒绝，并输出合法角色列表，证明镜像包含本轮角色实现。

## 未完成或未通过

| 项目 | 状态 | 原因/下一步 |
| --- | --- | --- |
| 单次原样执行 `scripts/test.sh` | 已通过 | 纯 Linux 容器退出 0 |
| standalone `core` | 已通过组成项 | Windows 外层容器无法直接共享 sibling Docker localhost，因此按组成项逐一执行 |
| standalone `saas` | 部分通过 | 前 8 项通过；Git Bash + Windows curl 会破坏中文 JSON，余项需 WSL2/Linux |
| standalone `frontend` | 未执行 | 等统一 Linux shell/Go/Docker 执行环境 |
| 真实生产证据 6 项 | 未具备 | 无服务器、域名、真实账号和监控资源 |

## 后续验证建议

在 Linux CI runner 或安装 Ubuntu 的 WSL2 中执行：

```bash
./scripts/test.sh
MOCHAT_ACCEPTANCE_SUITE=core ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=saas ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh
```

只有这些分组有新鲜通过记录后，才能把相应检查项标记完成。

## GitHub 替换记录

- 覆盖前 `main`：`3dcd216c188df34f2c3ed489b8e8b9473e635488`
- 可恢复备份：`backup/pre-phase0-main-20260723`
- Phase 0 工作分支：`phase0/modular-monolith-foundation`
- 首次替换后的 `main`：`9993080e9f666d51f08891369b6b8ed274e956d5`

替换使用带旧 SHA 的 `--force-with-lease`，避免覆盖操作期间出现的并发远端更新。
