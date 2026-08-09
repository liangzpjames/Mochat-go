# 企业微信生产同步与 Dashboard 验收记录（2026-08-10）

## 结论

服务器 `139.196.34.133` 已完成本地构建产物部署、企业微信真实数据同步、数据库结构校准和 Dashboard 浏览器验收，本次范围验收通过。

- 企业微信通讯录、客户标签、客户和客户群同步接口连续执行两轮，HTTP 状态及业务响应均为 `200`。
- 第二轮同步后的业务计数与第一轮一致；浏览器内再次执行同步后，计数仍与第二轮完全一致，未产生重复数据。
- 员工、企业根部门、客户和客户标签能够在 Dashboard 对应页面显示。
- 当前企业微信没有客户群，因此客户群页面显示“暂无数据”是当前真实状态，不伪造测试群数据。
- 数据概览接口及页面已恢复正常；当前线索、订单、经营行为数据为 `0`，这是 SCRM 业务数据的真实状态，与企业微信客户同步表不是同一统计口径。
- 会话归档和 AI 能力仍明确显示“未接入”，未将未配置能力伪装成成功。

## 部署与资源控制

本次没有在服务器执行 Go/Docker 编译。Linux/amd64 静态二进制在本地构建后上传，并在现有 `standalone-app-1` 容器内原子替换；MySQL、Redis 和 Docker volumes 均未重建。

| 项目 | 结果 |
| --- | --- |
| 最终代码提交 | `de4317a` |
| 源码指纹 | `ae8da9793928ac8e92b3c2926e5c539d53262077a4a414fcd118789892513d54` |
| 二进制 SHA-256 | `26e81f5b10de9d91960079444ecd6c3c7bd2146de5de6974884b9301138ae926` |
| 二进制大小 | `45,351,706` bytes |
| 应用容器 | `standalone-app-1`，healthy |
| MySQL 容器 | `standalone-mysql-1`，healthy |
| Redis 容器 | `standalone-redis-1`，healthy |
| 外部健康检查 | `/healthz` = `200`，`/readyz` = `200` |

最终检查未发现残留的 `docker compose build`、BuildKit、`go build` 或 `apk add` 进程。服务器内存约 `1.6 GiB`，最终可用约 `994 MiB`；未启用 swap。

保留的数据卷：

- `standalone_app-storage`
- `standalone_audit-anchor-storage`
- `standalone_mysql-data`
- `standalone_redis-data`

## 同步结果

执行接口：

- `PUT /dashboard/workEmployee/synEmployee`
- `PUT /dashboard/workContactTag/synContactTag`
- `PUT /dashboard/workContact/synContact`
- `PUT /dashboard/workRoom/syn`

当前选中企业（`corp_id=1`）的计数如下：

| 数据 | 同步前 | 第一轮 | 第二轮 | 浏览器交互后 |
| --- | ---: | ---: | ---: | ---: |
| 客户 | 0 | 1 | 1 | 1 |
| 客户-员工关系 | 0 | 1 | 1 | 1 |
| 客户-群关系 | 0 | 0 | 0 | 0 |
| 部门 | 1 | 1 | 1 | 1 |
| 员工 | 3 | 3 | 3 | 3 |
| 客户群 | 0 | 0 | 0 | 0 |
| 客户标签 | 3 | 3 | 3 | 3 |
| 客户标签组 | 1 | 1 | 1 | 1 |

服务器证据目录：`/opt/mochat-go/output/wecom-server-sync-20260809/`，其中包含两轮接口结果、各轮计数以及浏览器交互后的最终计数。

## 修复内容

1. 联系人同步只向企业微信请求具有客户联系权限的员工（`contact_auth = 1`），避免把本地引导管理员账号作为无效企业微信用户提交。提交：`3d7412c`。
2. 企业微信根部门的层级为 `level=0`。原页面树构建逻辑过滤了该节点，导致数据库已有部门但页面显示“暂无数据”；现保留该节点并显示为“企业根部门”。提交：`de4317a`。
3. 服务器迁移登记与实际表结构存在历史漂移，导致转化及概览报表返回 `500`。在数据库备份后，重新执行仓库内已有迁移 `0106`、`0119`、`0121`、`0123`；修复后转化、概览、健康检查均返回 `200`。

## 浏览器验收

已使用真实登录会话逐页识别并执行安全同步操作：

| 页面 | 验收结果 |
| --- | --- |
| `/index` | 正常加载，未再出现 `report query failed`；SCRM 空数据、AI 未接入、会话归档未接入均如实展示 |
| `/department/index` | 显示“测试公司 / 企业根部门”，最近同步时间正常 |
| `/workEmployee/index` | 显示 3 名员工及其状态、客户联系权限 |
| `/workContactTag/index` | 选择“客户等级”后显示“一般 / 重要 / 核心” |
| `/workContact/index` | 显示 1 条真实企业微信客户关系记录 |
| `/workRoom/index` | 显示“暂无数据”，与企业微信当前无客户群一致 |

截图位于本地 `output/wecom-server-sync-20260809/browser/`，包括：

- `overview.png`
- `department-root-visible.png`
- `employees.png`
- `contact-tags-selected.png`
- `contacts.png`
- `rooms.png`

验收结束后已退出 Dashboard 登录会话，临时密码已删除，管理员原密码哈希和原更新时间已精确恢复；服务器及容器内的临时令牌、令牌生成器、响应 JSON 和上传中间文件均已删除。

## 数据与回滚保护

| 备份 | SHA-256 |
| --- | --- |
| `pre-deploy-source.tar.gz` | `69204f18590edf666b9c6ecade891a34994244a711117ddc2a68cc074cdea50c` |
| `pre-sync.sql.gz` | `3879809bcc78affe3a2316863ac59ba7971b1f60a44a3f4292521ea477a97f5a` |
| `pre-report-schema-repair.sql.gz` | `2af2cb956206d92d54b9a364c8e9066f9bf60cd668e646ccab3ff9e6a52df9cfe` |
| 上一版应用二进制 | `690041037ade9553808a85c13139b99ccc0bcf7872d7cb68804221041802affd` |

## 验证门禁

- `go test ./internal/dashboard ./internal/store -count=1`：通过。
- `go test ./cmd/... ./internal/... -count=1`：在最终修复后完整通过。
- Dashboard Vitest：`89` 个文件、`557` 个测试通过。
- Dashboard TypeScript typecheck：通过。
- Dashboard production build：通过。
- `git diff --check`：无空白错误。
- `go test ./...` 会被用户已有的未跟踪目录 `scripts/decrypt_debug` 中重复 `main` 阻断；该目录不属于本次变更，未修改或删除。上述正式 `cmd` 与 `internal` 后端范围已经通过。
