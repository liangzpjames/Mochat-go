# Phase 3.4 marketing-tools 九页总验收报告

日期：2026-08-04

## 总结

Phase 3.4 的九个营销工具路由均已在 standalone Docker 和浏览器中逐页打开并完成主流程或明确失败路径取证。当前结果是“真实 Provider 与失败边界已收口，但存在外部凭据、本地业务数据和历史门禁阻塞”，不是九页全部可宣称产品级完成。

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

## 九页 Provider 状态

| 页面 | Manifest 结论 | 真实边界 |
| --- | --- | --- |
| 渠道活码 | `native / ready` | 真实查询与企业上下文；当前截图为空列表 |
| 群活码 | `native / partial` | 现有自动拉群查询可用，独立群活码/扫码统计合同仍缺失 |
| 获客链接 | `native / ready` | 本地 CRUD 与失败状态可验证；外部授权被明确阻断 |
| 微信客服 | `native / ready` | 本地 CRUD 与失败原因可验证；外部账号同步被明确阻断 |
| 活码短链 | `native / ready` | 本地创建、跳转、访问计数、停用和 `410` 已验证 |
| 一键加群 | `native / partial` | 真实模板写入前员工校验可用；本机无员工数据，无法进入外部创建 |
| 精准群发 | `native / partial` | 客户任务可保存并追踪 `执行中`；无外部回调，群聊缺少有效群主 |
| 朋友圈 | `native / partial` | 草稿、素材引用、失败状态和进度明细可用；正式发布适配器未配置 |
| 素材管理 | `native / ready` | 文本素材全链路、作用域、分组、移动、删除和引用保护已验证 |

详细截图见 `D:\workspace\mochat-go\output\phase34-browser-20260804`，批次报告见 [`batch4-material-foundation-report.md`](batch4-material-foundation-report.md)。

## 验证结果

- `go test ./...`：通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --filter @mochat/dashboard exec vitest run --reporter=dot`：通过，69 个文件、449 个测试。
- `pnpm --filter @mochat/dashboard build`：通过。
- `pnpm check:yuanhu-benchmark`：通过，53 页清单结构检查通过。
- `pnpm --filter @mochat/dashboard lint`：失败，175 个既有错误、0 个 warning；该失败没有被隐藏或重写。
- Docker `mochat-go-desktop`：app、MySQL、Redis healthy；`/healthz`、`/readyz` 均为 `200`。
- 迁移：`0116`、`0117`、`0118` 已在备份后应用并登记；正式 `apply` 仍受历史 `0001_initial_schema` checksum 漂移阻断。

## Manifest 规则

Manifest 已将素材管理从历史 `placeholder / missing / not-started` 修正为与证据一致的原生 Provider 状态，并为九页记录 2026-08-04 浏览器证据。外部 Provider 缺失的页面保持 `partial` 或仅提升到本地集成证据，未把“路由可达、fixture、单元测试通过”写成成功外发或产品级全线完成。

## 仍需外部验收

- 提供可用企业微信凭据后，验收获客链接授权、微信客服账号同步和朋友圈正式发布、回调、失败目标明细导出。
- 在测试企业导入真实员工、部门、群主和群聊数据后，验收一键加群、群聊群发和客户群发结果回调。
- 治理历史迁移 `0001_initial_schema` checksum 漂移后，再执行一次正式迁移 apply；本次不删除 Docker 数据卷。
