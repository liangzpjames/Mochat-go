# Dashboard 交互统一主线合入记录

日期：2026-08-09

## 1. 结论

`feat/2026-08-08-data-calibre-unification` 已完整快进合入本地 `main`，并以非强制方式推送到 `origin/main`。

- 远端合入前：`b262be2`
- 本地 `main` 合入前：`9b6515e`
- 功能分支合入点：`0fb21ae`
- 换行稳定性修复：`e3a35eb`
- 首次主线推送范围：`b262be2..e3a35eb`
- 远端核验：`refs/heads/main` 指向 `e3a35eb52023106f6680cd80f9925a09a1a42eb5`

合入使用 `git merge --ff-only feat/2026-08-08-data-calibre-unification`，没有冲突、没有改写远端历史、没有使用 force push。

## 2. 纳入主线的工作

本次将数据口径统一、Dashboard 53 页交互与视觉统一，以及最终企业信息布局修复一并纳入主线。Dashboard Task 1–8 的独立提交为：

| Task | 提交 |
|---|---|
| Task 1 统一交互基础组件 | `4a604f1` |
| Task 2 页面壳、搜索与企业切换 | `a3c91ee` |
| Task 3 响应式 Drawer | `6779369` |
| Task 4 P0 高风险操作确认 | `46b88ed` |
| Task 5 会话与营销工具 | `f87decf` |
| Task 6 SCRM 业务流程 | `9818625` |
| Task 7 报表与设置弹层 | `6ab7147` |
| Task 8 53 页真实服务验收 | `ad14218` |

后续企业信息布局修复为 `0fb21ae`。数据口径统一及其计划、验收、进度文档也位于同一祖先链中，未遗漏或挑选性 cherry-pick。

Task 8 的完整浏览器、Docker、卷和未执行破坏性操作证据见：

`docs/phases/phase-3-dashboard/phase-3-final/2026-08-08-dashboard-interaction-unification-acceptance.zh-CN.md`

## 3. 合入前后验证

功能分支在合入前串行执行：

| 命令 | 结果 |
|---|---|
| `pnpm --filter @mochat/dashboard lint` | PASS |
| `pnpm --filter @mochat/dashboard typecheck` | PASS |
| `pnpm --filter @mochat/dashboard build` | PASS |
| `pnpm --filter @mochat/dashboard test` | PASS，89 个测试文件、557/557 测试 |

Task 8 已在真实 Docker 服务上完成 manifest 53 页桌面端与 390px 移动端验收：53/53 PASS，106 张截图。主线快进后的 Dashboard 源码与已验证的 `0fb21ae` 完全相同；新增提交只涉及迁移 SQL 换行规则和本记录。

干净主线 worktree 最终执行：

| 命令 | 结果 |
|---|---|
| `go test ./internal/migration -run TestHistorical0099ChecksumRemainsStable -count=1` | PASS |
| `go test ./...` | PASS |
| `git diff --check` | PASS |

## 4. Windows 换行门禁修复

第一次在原工作树执行 `go test ./...` 时，用户未跟踪的 `scripts/decrypt_debug/` 被 Go 纳入扫描，因多个调试入口重复声明 `main` 而失败；这些文件不属于 Git，也未被修改、删除或提交。

随后在干净 worktree 中发现历史迁移 `0099` checksum 会受全局 `core.autocrlf=true` 影响。仓库索引保存 LF，但该历史迁移要求：

- `0099_saas_tenant_default_corp.up.sql` 检出为 CRLF；
- `0099_saas_tenant_default_corp.down.sql` 检出为 LF。

提交 `e3a35eb` 在 `.gitattributes` 中逐文件固定上述规则。全新检出后两个 SHA-256 分别恢复为：

- up：`c409fc5562fa2336f9f559efc628b10ff42b0e2f2ebab6af30bf2607f38fd099`
- down：`de7efc7f22eb8b834d1b046916138b38f294023e4cdec9b828b05b6ae56f0e5a`

历史 SQL 内容与索引 blob 没有被改写。

## 5. 安全边界

本次主线合入与推送：

- 未执行 `git reset`、`git clean` 或 force push；
- 未提交原工作树中的 Dockerfile、Compose、部署脚本、调试脚本或其他未跟踪目录；
- 未运行 Docker 重建、数据库重置、卷删除或任何真实业务破坏性动作；
- 使用 `D:\workspace\mochat-go\output\mainline-handoff-20260809` 临时 worktree 操作 `main`，原功能工作树及用户未提交内容保持原状。

## 6. 推送命令与证据

```powershell
git fetch origin --prune
git pull --ff-only origin main
git merge --ff-only feat/2026-08-08-data-calibre-unification
git push origin main
git ls-remote origin refs/heads/main
```

代码主线推送输出：`b262be2..e3a35eb  main -> main`。
