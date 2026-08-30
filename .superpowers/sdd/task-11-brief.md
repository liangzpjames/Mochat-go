# Task 11：callback side-effect unknown 生产恢复闭环

## 基线与前置

- 仅在 `D:\workspace\mochat-go\mochat-go\.worktrees\p0-local-closure-20260829` 工作。
- 开始前读取当前 HEAD、Git 状态，以及以下文件的完整内容：
  - `docs/superpowers/specs/2026-08-29-p0-local-production-readiness-design.zh-CN.md`
  - `.superpowers/sdd/p0-followup-sideeffect-design.md`
  - Task 10 的报告与提交（若已存在）。
- Task 10 必须已完成并由主代理复验；不得覆盖或回退其改动。
- 禁止修改 `docs/PROJECT_PROGRESS.zh-CN.md`，禁止 reset/clean/force，禁止触碰其他 worktree、用户文件和 Docker 命名卷。
- 不调用真实企业微信、真实 AI Provider 或生产服务器。Provider 行为只允许 fake/契约测试，并清楚标注证据边界。

## 目标

为 `0174` 引入的 callback 外部副作用 `unknown` 状态建立可生产操作的恢复闭环：

1. 受 Dashboard principal 与 `dashboard.company_setting.website` 权限保护的 unknown 列表、详情和人工 reconcile 接口。
2. 两种决议：
   - `confirm_sent`：人工证据证明 Provider 已发送，管理 API 只将该 action 标为 sent，绝不再次调用 Provider。
   - `confirm_not_sent_and_retry`：人工证据证明 Provider 未发送，只将该 action 重置为 pending，由 durable worker 后续重试。
3. tenant/corp 只能来自服务端 principal/single-corp binding；客户端不得传入或覆盖作用域。
4. request idempotency receipt、payload fingerprint、首次响应重放、同 key 异 payload 冲突。
5. action、receipt、审计、inbox 复活必须在同一 MySQL 事务；失败注入证明任一点失败全回滚。
6. worker Begin/Complete 与人工恢复都绑定 inbox lease token/fence，统一锁顺序，阻止过期 worker 写回。
7. 双 action 独立推进：只改变目标 action；仍有 unknown 时不复活 inbox；最后一个 unknown 解决后才复活。
8. 活跃 lease、隔离期、版本/fence 冲突、状态冲突、未知 action、DB 不可用均有稳定错误语义。
9. 新增 MySQL 5.7 兼容的 `0176` up/down 迁移，并接入动态 registry、路由合同、RBAC 和 ready/migration 最新版本检查；不得硬编码错误 count。`0175` 已由 Task 10 的联系人批次标题迁移占用。
10. 新增中文运维说明，明确人工证据要求、重复发送风险、两种决议区别、审计与无真实 Provider 的验证边界。

## TDD 要求

先写并运行失败测试、记录 RED，再写实现。至少覆盖：

- 无 principal、无权限、跨 tenant/corp、body 注入 scope 在进入 Store 前拒绝。
- 列表/详情稳定分页且不泄露 callback 正文、token/secret。
- `confirm_sent` 与 `confirm_not_sent_and_retry` 的合法/非法转换。
- 同 request 同 payload 返回首次响应；同 request 异 payload 409；并发请求只产生一次状态变化、一次审计和一次 inbox 复活。
- 活跃 lease、过期 lease、旧 fence、旧 worker Begin/Complete/CompleteInbox。
- 双 action 分别为 unknown/sent/pending 时的独立恢复和 worker 重放。
- receipt/action/inbox/audit/commit 故障注入的事务回滚。
- fake Provider 调用次数：confirm_sent 为 0；confirm_not_sent_and_retry 恢复后恰 1；未知结果绝不自动二次外发。
- MariaDB 10.6 与 MySQL 5.7 上 0176 up/down、并发和完整 registry lifecycle。

不能降低既有断言、不能把 fixture 注册当生产完成、不能声称 Provider exactly-once。若设计报告中的细节与当前代码存在矛盾，先追根因并在 Task 报告说明调整依据。

## 交付

- 提交所有代码、迁移、测试和中文运维文档；不要只留未提交工作树。
- 在 `.superpowers/sdd/task-11-report.md` 记录：精确基线/提交、RED、根因、改动、MariaDB/MySQL5.7/fake 合同证据、FAIL/SKIP、真实环境边界。
- 至少运行相关 package 全量测试、`go test ./...`、`go vet ./...`、`git diff --check`，以及 migration lifecycle/diff/check；环境具备时运行适用 race。
