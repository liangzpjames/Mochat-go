# 风险预警查询页面批次报告

## 范围

本批次只覆盖以下六个风险预警路径：

- `/ai-insight/v2/risk`
- `/ai-insight/v2/timeout`
- `/ai-insight/v2/customer-loss`
- `/ai-insight/v2/message-intercept`
- `/ai-insight/v2/keyword-library`
- `/ai-insight/v2/silent-customer`

## Provider 与数据边界

| 路径 | 页面实现 | 数据来源 | 状态 |
| --- | --- | --- | --- |
| `/ai-insight/v2/customer-loss` | 原生风险预警页面 | `GET /workContact/lossContact` | 已接入真实只读查询 |
| 其余五个路径 | 原生风险预警页面 | 无明确后端 Provider | 明确显示“数据提供方未接入” |

客户流失接口当前只声明 `employeeId`、`page` 和 `perPage` 查询参数；页面仅向它提交这些参数。关键词、日期和状态筛选控件明确禁用并说明该 Provider 未声明对应能力，避免显示未经后端支持的筛选结果。

未接入页面不请求接口，不展示 DemoPage fixture、风险命中、规则、下载链接或虚构客户记录。

## 验证记录

- 定向 Vitest：`risk-warning-pages.test.tsx` 与 `page-registry.test.tsx`，26 项通过。
- Dashboard TypeScript：`pnpm --filter @mochat/dashboard typecheck` 通过。
- Dashboard 生产构建：`pnpm --filter @mochat/dashboard build` 通过。
- `git diff --check` 通过。

## 浏览器检查

本地 Vite 服务能够启动，但直接访问风险预警路径会进入登录页。当前没有可审计的已授权企业会话，未绕过认证或注入模拟会话，因此未采集页面截图。组件测试已覆盖已接入、空数据、查询失败重试和五个未接入 Provider 状态。
