# 0165 AI 日洞察统一受控迁移手册

## 1. 为什么必须受控

`0165_ai_daily_insight_unification.up.sql` 包含两类不可逆数据操作：

1. 同一租户、企业、分析类型、规则版本、会话和分析日只保留物理 ID 最大的一行；
2. 删除旧表 `mochat_go_ai_analysis`。

历史 down 只能重建空旧表，无法还原已删除的重复行和旧表内容。因此，本次不修改已经发布的 0165 SQL，也不新增 checksum alias，而是把 0165 注册为 controlled migration：普通 `mochat-migrate apply` 到达 0165 时停止，必须使用本手册中的独立工具完成盘点、备份、审批、执行和校验。

本工具只为尚未执行 0165 的环境建立安全流程。若迁移账本已经记录 0165，工具只会返回 `already applied` 和人工恢复边界；没有经验证的迁移前备份时，不得声称历史数据可恢复。

## 2. 安全边界

- 必须先停止应用、worker、scheduler 和所有能写入两张源表的维护任务。
- DSN 只能通过受限文件传入，禁止放入命令行参数、日志、报告或 Git。
- 工具在同一数据库内创建审计清单和备份表；生产操作还必须在此之前完成外部物理备份或受控 `mysqldump`，并验证恢复演练。
- 备份、preflight 和 apply 使用 MySQL named lock 防止两个工具实例并行执行。
- apply 会临时安装写保护 trigger。只有持有当前 request ID 的受控连接能修改源表；其他连接写入会失败。安装 trigger 前后仍会重算源 count 和 SHA-256 摘要。
- `approval-token` 绑定 request ID、schema、0165 原始 checksum、源/备份 count、源/备份摘要、重复行数和旧表行数。任一数据变化都会使审批失效。
- `--confirm-traffic-stopped` 是操作者对停流事实的显式确认，不是自动探测；它不能替代变更窗口和外部监控。
- 无真实生产环境、外部备份介质和恢复演练时，只能报告本地合同 PASS，不能报告生产迁移完成。

## 3. 固定对象

| 对象 | 用途 |
| --- | --- |
| `mochat_go_controlled_migration_0165` | 保存 request、原始 migration checksum、源/备份 count 和 SHA-256 摘要、状态与验证时间 |
| `mochat_go_backup_0165_ai_conversation_insights` | 迁移前洞察表完整结构和数据快照 |
| `mochat_go_backup_0165_ai_analysis` | 旧表存在时的完整结构和数据快照 |

备份对象一旦存在，新的 backup 请求会 fail closed。不得为了重跑而直接删除这些表；先归档、核验并由变更负责人决定恢复或在全新 schema 重演。

发布的 0165 checksum 固定为：

```text
4575a0d89e59cf0b87059c0d60575be3e5cc566ee7338cc6fb6f616e8431520f
```

## 4. 命令流程

以下命令可使用构建产物 `mochat-ai-insight-0165`。源码环境也可用：

```text
go run ./scripts/preflight_0165_ai_daily_insight_unification.go <action> [flags]
```

### 4.1 准备 DSN 文件

在工作区外创建只包含一行 DSN 的受限文件。DSN 必须指向目标 schema，并启用 `parseTime=true`；不要在文档或证据中复制其内容。

### 4.2 只读盘点

```text
mochat-ai-insight-0165 inventory \
  --dsn-file <受限DSN文件> \
  --project-root <当前精确SHA源码根>
```

记录输出中的：schema、0165 checksum、洞察行数/摘要、按 0165 逻辑会删除的重复行数、旧表行数/摘要。以下情况立即停止：

- 0165 已经应用；
- 迁移账本不存在或 checksum 不符；
- 源表/必要列缺失；
- 0165 新列已经出现但账本未记录；
- 指向了错误 schema。

### 4.3 外部备份和恢复演练

先完成数据库平台认可的物理备份或逻辑 dump，并在隔离实例恢复后核对源表 count/摘要。外部备份标识、存储位置、加密、保留期和恢复演练证据由发布变更单保存，不写入仓库。

### 4.4 创建数据库内快照

```text
mochat-ai-insight-0165 backup \
  --dsn-file <受限DSN文件> \
  --project-root <当前精确SHA源码根> \
  --request-id <稳定变更请求ID>
```

工具会在 named lock 内复制源表，复制完成后重新读取源数据，并要求源/备份 count 与 SHA-256 摘要完全一致。复制期间发生写入时，backup 返回失败且不得继续。

### 4.5 Preflight 与人工审批

```text
mochat-ai-insight-0165 preflight \
  --dsn-file <受限DSN文件> \
  --project-root <当前精确SHA源码根> \
  --request-id <稳定变更请求ID>
```

preflight 要求数据库内备份存在且未漂移，同时要求实时源快照仍与备份清单一致。输出包含：

- `approvalToken`；
- `destructiveApproval`，格式为 `duplicates=<N>,legacy=<M>`。

变更审批人必须核对 N 和 M、外部备份恢复证据以及业务停流窗口，再把这两个精确值交给执行人。不要手写推测值。

### 4.6 执行

```text
mochat-ai-insight-0165 apply \
  --dsn-file <受限DSN文件> \
  --project-root <当前精确SHA源码根> \
  --request-id <稳定变更请求ID> \
  --approval-token <preflight原样输出> \
  --approve-destructive "duplicates=<N>,legacy=<M>" \
  --confirm-traffic-stopped
```

apply 会再次完成全部 preflight、安装写保护、再次验证实时摘要，随后执行仓库中原始 0165 SQL。迁移后必须同时满足：

- 洞察行数等于 `源行数 - 重复行数`；
- `analysis_date` 和 previous snapshot 列存在；
- `uq_ai_conversation_daily` 存在，旧唯一索引不存在；
- 旧表不存在；
- 两张备份表的 count 和摘要未变化；
- 普通迁移账本记录原始 0165 checksum；
- 控制清单状态为 `verified`。

只有全部条件成立才返回 PASS。

### 4.7 独立复核

```text
mochat-ai-insight-0165 verify \
  --dsn-file <受限DSN文件> \
  --project-root <当前精确SHA源码根> \
  --request-id <稳定变更请求ID>
```

`verify` 可在 SQL 已执行但最后账本写入或输出中断时重试。它不会重新执行 0165 数据删除，只按备份清单验证 post-state，并在验证通过后补记原始 checksum。

## 5. 失败与恢复

- 在 apply 前失败：不执行 0165；保留控制清单和备份表供调查。
- 在 apply 中途失败：MySQL/MariaDB DDL 可能已经隐式提交。立即保持停流，禁止普通 migrate 继续，保存错误和数据库状态；从已经验证的外部备份恢复到新 schema，再核对源摘要。不要直接运行历史 down 冒充数据恢复。
- 已执行环境无迁移前备份：只能确认 schema 现状，无法证明被删行内容；恢复状态为 `SKIP/NOT RECOVERABLE FROM REPOSITORY`。
- 已执行环境有可信外部备份：在隔离实例恢复，比较账本前后、源/备份摘要和后续迁移影响，再制定人工恢复 SQL。不得自动覆盖现库。

## 6. 验收分层

- 单元/契约 PASS：approval 绑定、错误参数和 controlled registry。
- 隔离 MariaDB/MySQL 5.7 PASS：重复、旧表、缺备份、源/备份漂移、并发 apply、错误 schema、已执行边界、原 checksum。
- Docker 本地 PASS：当前精确 SHA 构建的二进制可运行 inventory→backup→preflight→apply→verify。
- 生产迁移、外部备份、真实停流和恢复演练：本任务不执行，保持 SKIP。
