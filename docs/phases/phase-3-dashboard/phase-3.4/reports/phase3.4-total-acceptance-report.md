# Phase 3.4 marketing-tools 九页总验收报告

日期：2026-08-04

## 总结

Phase 3.4 的九个营销工具路由均已在 standalone Docker 和浏览器中逐页打开，并完成本地主流程或明确失败路径取证。当前结论是“真实 Provider、租户边界、持久化状态和失败边界已收口，但仍存在外部凭据与本地业务数据阻塞”，不是九页全部产品级完成。

## 交付提交

| 提交 | 内容 |
| --- | --- |
| `4bbcc77` | Provider 收口设计与批次边界 |
| `ba04057` | 获客基础 Provider、持久化、权限和失败状态 |
| `9e64d33` | 转化与触达 Provider 业务闭环 |
| `7d9e7b6` | 朋友圈本地持久化与发布审计状态机 |
| `18367af` | 素材引用持久化与企业隔离校验 |
| `52699bb` | 获客授权失败状态持久化 |
| `b0861fe` | 获客链接失败状态刷新 |
| `cee987f` | 微信客服同步失败状态刷新 |
| `d31b31d` | 短链 `/r/` 跳转绕过前端 fallback |
| `7edbfcd` | 素材选择器作用域 SQL 括号修复 |
| `90fed45` | Phase 3.4 lint 门禁、迁移漂移兼容、抽屉布局和 Manifest 语义修正 |

## 九页 Provider 状态

| 页面 | Manifest 结论 | 真实边界 |
| --- | --- | --- |
| 渠道活码 | `native / ready` | 真实查询与企业上下文；当前测试企业列表为空，但没有 fixture 冒充数据 |
| 群活码 | `native / partial` | 复用自动拉群查询可用，独立群活码/扫码统计合同仍缺失 |
| 获客链接 | `native / partial` | 本地 CRUD、租户隔离、授权失败状态和刷新可验证；正式外部授权未配置 |
| 微信客服 | `native / partial` | 本地 CRUD、同步失败原因和刷新可验证；正式客服账号同步未配置 |
| 活码短链 | `native / ready` | 本地创建、站内跳转、访问计数、停用和 `410` 已验证 |
| 一键加群 | `native / partial` | 真实模板写入前员工/标签校验及 Provider 错误可验证；本机没有真实员工/群主数据 |
| 精准群发 | `native / partial` | 客户任务可保存并追踪 `执行中`；无外部回调，群聊缺少有效群主 |
| 朋友圈 | `native / partial` | 草稿、素材引用、发布状态、进度、失败明细和导出合同可用；正式发布适配器未配置 |
| 素材管理 | `native / ready` | 文本素材作用域、分组、创建、移动、批量删除、引用保护和跨页选择器已验证 |

这里的 `backend=ready` 不是“企业微信外部闭环已经成功”的同义词，而是本项目允许在无外部依赖的本地 Provider 已具备完整合同时使用的状态。对于设计文件明确要求外部授权/同步才能达到 ready 的获客链接和微信客服，本次已按测试和设计门槛降为 `partial`；绝不以保存本地草稿代替外部成功。

详细截图见 `D:\workspace\mochat-go\output\phase34-browser-20260804`，批次报告见 [`batch4-material-foundation-report.md`](batch4-material-foundation-report.md)。

## Lint 审计

Dashboard 全仓库 ESLint 基线在修复前为 `175 errors / 0 warnings / 29 files`；本次移除 2 个 Phase 3.4 新增错误后，最终复跑为 `173 errors / 0 warnings / 27 files`，剩余全部属于历史文件。按最终结果规则分布：

| 规则 | 数量 |
| --- | ---: |
| `@typescript-eslint/no-unnecessary-type-assertion` | 44 |
| `@typescript-eslint/no-unsafe-assignment` | 35 |
| `@typescript-eslint/unbound-method` | 33 |
| `@typescript-eslint/no-base-to-string` | 24 |
| `@typescript-eslint/no-unsafe-member-access` | 10 |
| `@typescript-eslint/no-explicit-any` | 8 |
| `@typescript-eslint/restrict-template-expressions` | 8 |
| `@typescript-eslint/no-misused-promises` | 4 |
| `@typescript-eslint/no-unused-expressions` | 4 |
| `@typescript-eslint/no-unused-vars` | 1 |
| `@typescript-eslint/require-await` | 2 |

按最终文件分布：

| 文件 | 错误数 |
| --- | ---: |
| `features/scrm/contact-page.test.tsx` | 28 |
| `features/scrm/scrm-api.ts` | 20 |
| `features/scrm/lead-page.test.tsx` | 15 |
| `features/sensitive-word/sensitive-word-page.test.tsx` | 12 |
| `features/scrm/opportunity-page.test.tsx` | 11 |
| `features/sensitive-word/sensitive-word-api.ts` | 11 |
| `features/scrm/tag-page.test.tsx` | 10 |
| `features/scrm/contact-api.ts` | 9 |
| `features/scrm/contact-page.tsx` | 9 |
| `features/phase33/message-intercept-pages.tsx` | 8 |
| `features/phase33/risk-behavior-page.tsx` | 7 |
| `features/scrm/scrm-api.test.ts` | 5 |
| `features/scrm/lead-api.ts` | 5 |
| `features/phase33/phase33-closure-pages.tsx` | 4 |
| `features/scrm/contact-api.test.ts` | 3 |
| `features/phase33/customer-loss-page.tsx` | 2 |
| `features/phase33/customer-transfer-page.tsx` | 2 |
| `features/scrm/public-pool-page.test.tsx` | 2 |
| `features/sensitive-word/sensitive-word-page.tsx` | 2 |
| `components/page-state/page-state.test.tsx` | 1 |
| `features/conversation-global/conversation-export-page.test.tsx` | 1 |
| `features/conversation-global/conversation-export-page.tsx` | 1 |
| `features/dashboard-overview/dashboard-overview-api.test.ts` | 1 |
| `features/dashboard-overview/dashboard-overview-page.test.tsx` | 1 |
| `features/phase33/timeout-warning-page.tsx` | 1 |
| `features/scrm/follow-up-timeline.test.tsx` | 1 |
| `layout/dashboard-layout.test.tsx` | 1 |

完整原始 JSON 证据在 `D:\workspace\mochat-go\output\phase34-lint-audit\eslint-final.json`。

本次 Phase 3.4 新增/修改代码原有 2 个错误已修复：`features/phase34/content-reach-pages.tsx` 的未使用 `taskID` 和 `features/phase34/conversion-pages.tsx` 的未使用 `conversionID`。新增门禁：

```text
PHASE34_LINT_BASE=13cd9cc pnpm check:phase34-lint
```

该门禁按基线、工作区 diff 和未跟踪 Dashboard TS/TSX 文件选择 changed files；最终选中的 Phase 3.4/Manifest 测试与生产文件 lint 全绿。全仓库历史债务没有被配置忽略，也没有被误报为本批通过。

## 迁移 checksum 漂移审计与处理

审计和修复均未删除 Docker 数据卷、未修改迁移账本旧记录，且在 apply 前保留了 D 盘逻辑备份：

- `D:\workspace\mochat-go\output\docker-backups\mochat-go-desktop-20260804-phase34-final\mochat-before-phase34-migrations.sql`
- `D:\workspace\mochat-go\output\docker-backups\mochat-go-desktop-20260804-phase34-checksum-audit\mochat-before-0001-alias.sql`

精确差异如下：

1. `0001_initial_schema` 账本保存 `b7dbd66b24b93a4be64e33fa51d2e1a1fcbc0d305532145644c37ed1a26075e9`，当前 `deploy/standalone/schema/mochat.sql` 为 `03425c87c5584e82b7991d7f5fe4c75f8918b31799e191dda7160a0cad150ace`。当前 Git 历史中没有可安全还原的旧组合源码；该旧 hash 在迁移前备份账本中得到独立确认。因此仅在 `0001` 版本的 `ChecksumAliases` 接受这个精确历史值，不接受任意 hash，也没有篡改账本。
2. `0002_seed_core_data` 账本为 `5154da0c40c71eeb9f649e073a5fc04d2d5f09f5dc944b3a83c4f278bfb0f48a`，当前 LF 文件为 `de6513fb142d38fbd0205ecaa6e6ddbdd9d765e5c455ab20bd2afdd158d276ed`；把同一内容转换为 CRLF 后正好得到账本 hash，根因是换行符漂移。
3. `0100_scrm_customer_lifecycle` 账本为 `803b9aeb698a63701efa9ce6ca54746c5ed7a2a3335570e3d17a9dd97c1bb3e3`；当前 Windows 工作树 CRLF 为 `adb37a32ba6230cf10a85d127a34182721539b457057f1bd8bd3c23b266c7bae`，Git 历史 LF blob 与账本 hash 相同，同样是换行符漂移。
4. 数据库原有 `0106_work_message_global_search_indexes`，而当前源码使用版本名 `0106_scrm_lead_parity`；现场 schema 已有最终 lead 列和索引。新增的 lead parity migration 在检测到最终结构时只执行安全 no-op，再由正式 Runner 登记当前版本；旧 `0106` 记录保持不变。
5. `0107_scrm_contact_lifecycle_idempotency` 的表和索引已存在但账本缺记录，新增兼容 guard 后由正式 apply 登记；`0108_scrm_public_pool_parity` 对已有列、表和索引逐对象检查，只补缺失项并正式登记。

正式命令在当前镜像中执行两次均退出 `0`：

```text
docker compose -f deploy/standalone/docker-compose.yml -p mochat-go-desktop exec -T app /usr/local/bin/mochat-migrate -action apply -project-root /app
```

当前 `mochat_go_schema_migrations` 共 `119` 条，范围 `0001_initial_schema` 至 `0118_material_selector_references`；`0001`、`0002`、`0100` 的原账本 hash 均由版本限定 alias 校验通过，`0106`、`0107`、`0108` 及后续迁移均已正式登记，第二次 apply 为幂等复核。

## 截图证据审计

原始目录中的 60 张截图全部为 `1280x720`，文件扩展名为 `.png` 但实际编码为 JPEG。逐张检查后，以下 7 张是明显空白/错误页证据，保留作审计痕迹但不再作为验收证据：

`19-live-code-short-chain-disabled.png`、`21-wechat-customer-service-failed-refreshed.png`、`23-group-template-create-drawer.png`、`24-group-template-provider-failed.png`、`26-material-management-empty.png`、`28-material-saved.png`、`29-material-group-created.png`（约 `14,271–14,300` 字节，主内容为空）。

已在修复抽屉布局并统一浏览器尺寸后重拍，7 张正式替代证据均为 `1280x720`，且人工检查了页面内容、遮罩、布局和失败反馈：

- [70-live-code-short-chain-disabled-1280x720.png](D:/workspace/mochat-go/output/phase34-browser-20260804/70-live-code-short-chain-disabled-1280x720.png)：停用短链与持久化状态。
- [71-wechat-customer-service-failed-1280x720.png](D:/workspace/mochat-go/output/phase34-browser-20260804/71-wechat-customer-service-failed-1280x720.png)：刷新后失败客服记录仍可见。
- [72-group-template-create-drawer-1280x720.png](D:/workspace/mochat-go/output/phase34-browser-20260804/72-group-template-create-drawer-1280x720.png)：创建抽屉、遮罩和修复后的纵向表单布局。
- [73-group-template-provider-failed-1280x720.png](D:/workspace/mochat-go/output/phase34-browser-20260804/73-group-template-provider-failed-1280x720.png)：无效员工 ID 触发真实 `使用者信息错误`，没有假二维码或成功状态。
- [74-material-management-empty-1280x720.png](D:/workspace/mochat-go/output/phase34-browser-20260804/74-material-management-empty-1280x720.png)：搜索无结果空态。
- [75-material-saved-1280x720.png](D:/workspace/mochat-go/output/phase34-browser-20260804/75-material-saved-1280x720.png)：已保存文本素材。
- [76-material-group-created-1280x720.png](D:/workspace/mochat-go/output/phase34-browser-20260804/76-material-group-created-1280x720.png)：已创建分组及分组内素材。

## 验证结果

- `go test ./...`：通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --filter @mochat/dashboard exec vitest run --reporter=dot`：通过，70 个文件、450 个测试。
- `pnpm --filter @mochat/dashboard build`：通过。
- `pnpm check:yuanhu-benchmark`：通过，53 页清单结构检查通过。
- `PHASE34_LINT_BASE=13cd9cc pnpm check:phase34-lint`：通过，10 个 changed Dashboard TS/TSX 文件全绿；全仓库 lint 最终仍为 173 个历史错误、0 个 warning。
- Docker `mochat-go-desktop`：app、MySQL、Redis healthy；`/healthz`、`/readyz` 均返回 `200`，readyz 的 `migrated_route_count` 为 `712`。

## Manifest 规则

Manifest 已将素材管理从历史 `placeholder / missing / not-started` 修正为与证据一致的原生 Provider 状态。按设计中“外部授权/发布未配置时不得标 `backend=ready`”的规则，获客链接和微信客服已由 `ready` 降为 `partial`；群活码、一键加群、精准群发和朋友圈继续保持 `partial`。本地 Provider 的持久化和失败路径不被误写为外部成功，`pnpm check:yuanhu-benchmark` 只在真实清单结构和证据字段满足时通过。

## 仍需外部验收

- 提供可用企业微信凭据和授权后，验收获客链接正式授权、微信客服账号同步，以及朋友圈正式发布、回调、失败目标明细导出。
- 在测试企业导入真实员工、部门、群主和群聊数据后，验收一键加群、群聊群发和客户群发的真实外部结果回调。
- 朋友圈、客户群发等页面目前只记录本地失败/执行中状态；没有外部回调时，不将任务提升为成功。
