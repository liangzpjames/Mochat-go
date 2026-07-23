# Phase 0 验证记录

> 验证时间：2026-07-23
> 分支：`phase0/modular-monolith-foundation`

## 结论

Phase 0 新增的运行角色、模块模板、架构门禁和 Docker 开发入口已通过容器化测试与构建。仓库原 `scripts/test.sh` 的组成项已分别执行；在 Git Bash 中运行到生产证据 smoke 时受 Windows Python `/tmp` 路径语义影响失败，相同生产证据 smoke 随后在纯 Linux Python 容器内退出 0。

standalone `core/saas/frontend` 分组尚未完成本轮新鲜执行，因此本记录不把 Phase 0 描述为生产 readiness，也不改变原 `ready=false`。

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
| 单次原样执行 `scripts/test.sh` | 环境组合限制 | Git Bash + Windows Python 的 `/tmp` 不一致；各组成项已在适合的容器中分别通过 |
| standalone `core` | 未执行完成 | 入口强制先跑上述单次 `scripts/test.sh` |
| standalone `saas` | 未执行 | 等统一 Linux shell/Go/Docker 执行环境 |
| standalone `frontend` | 未执行 | 等统一 Linux shell/Go/Docker 执行环境 |
| 真实生产证据 6 项 | 未具备 | 无服务器、域名、真实账号和监控资源 |

## 后续验证建议

在 Linux CI runner 或带完整 WSL2 工具链的机器执行：

```bash
./scripts/test.sh
MOCHAT_ACCEPTANCE_SUITE=core ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=saas ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh
```

只有这些分组有新鲜通过记录后，才能把相应检查项标记完成。
