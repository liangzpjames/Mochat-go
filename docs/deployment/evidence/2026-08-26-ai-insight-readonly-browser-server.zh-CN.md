# AI 洞察只读工作区：浏览器与服务器脱敏验收证据

日期：2026-08-26（Asia/Shanghai）

本文件记录最终提交 `552c0a18f5f5…` 和最终镜像 `4b50c04b…cdb9fb` 的复核结果。内容不包含客户端 IP、User-Agent、Cookie、登录凭据、环境变量值、密钥、Token 或业务响应正文。

## 证据边界与方法

- 浏览器复用用户已登录的真实 Dashboard 会话，视口宽度 1280 像素。
- 先展开所有菜单组，再对 53 个可见真实菜单链接逐一执行 click；每个路由加载后采样路径、标题、页面文本、图片与文档宽度。
- 5 个故障页另行逐页 click、reload，采样 warning/error 日志。
- `.superpowers/sdd/evidence/ai-insight-session-analysis-final-552c-post-restart.png` 仅为重启后会话分析单页辅助截图；完整覆盖以结构化点击结果、Nginx 状态和数据库查询为准。
- 第一版试运行镜像 `fa56704a…0ddb1673` 的既有记录已经撤回，不作为最终验收证据。

## 浏览器：53 路由点击记录

| 批次 | 数量 | 实际点击路由 | 结果 |
| --- | ---: | --- | --- |
| 1 | 8 | `/index`；`/chat/v2-all`；`/chat/v2-staff`；`/chat/v2-customer`；`/chat/v2-group`；`/chat/trajectory`；`/chat/export`；`/chat/file-audio` | 8/8；失败 0；501/Zod/破图/溢出均 0 |
| 2 | 8 | `/chat/resign-staff`；`/chat/refuse-archive`；`/customer/inheritance`；`/ai-insight/v2/risk`；`/ai-insight/v2/sensitive-word`；`/ai-insight/v2/timeout`；`/ai-insight/v2/customer-loss`；`/ai-insight/v2/message-intercept` | 8/8；失败 0；501/Zod/破图/溢出均 0 |
| 3 | 8 | `/ai-insight/v2/keyword-library`；`/ai-insight/v2/silent-customer`；`/ai-insight/session-analysis`；`/ai-insight/smart-analysis`；`/ai-insight/emotion`；`/ai-insight/employee-score`；`/ai-insight/communication-keyword`；`/acquisition/v2-channel-code` | 8/8；失败 0；501/Zod/破图/溢出均 0 |
| 4 | 8 | `/acquisition/group-code`；`/acquisition/redirect-link`；`/acquisition/wechat-customer-service`；`/acquisition/live-code-short-chain`；`/acquisition/group-template`；`/acquisition/precise-group-send`；`/acquisition/friends-circle`；`/acquisition/material-management` | 8/8；失败 0；501/Zod/破图/溢出均 0 |
| 5 | 8 | `/customer/clue/default`；`/customer/contact`；`/customer/friends`；`/customer/opportunity`；`/customer/public-sea`；`/customer/group`；`/customer/tags`；`/customer/order` | 8/8；失败 0；501/Zod/破图/溢出均 0 |
| 6 | 8 | `/customer/settings`；`/data/customer`；`/data/employee`；`/data/conversion`；`/data/behavior`；`/data/report`；`/ai-setting/ai-knowledge-base`；`/ai-setting/agent` | 8/8；失败 0；501/Zod/破图/溢出均 0 |
| 7 | 5 | `/company-setting/website`；`/company-setting/staff`；`/setting/role`；`/setting/additional`；`/setting/authorization` | 5/5；失败 0；501/Zod/破图/溢出均 0 |

汇总：53/53；`failures=[]`；click/path 错误 0；501/未迁移 0；Zod 原始错误 0；破图 0；横向溢出 0；最终 warning/error 日志 `[]`。

## 浏览器：5 个故障页专项刷新

采样开始：`2026-08-25T17:27:39.707Z`（北京时间 2026-08-26 01:27:39）。

| 页面 | 路由 | 页面状态 | 结果 |
| --- | --- | --- | --- |
| 会话分析 | `/ai-insight/session-analysis` | `AI 服务暂不可用：尚未运行`，0 条 | 501=false；Zod=false；破图=0；溢出=false |
| 智能分析 | `/ai-insight/smart-analysis` | `AI 服务暂不可用：尚未运行`，0 条 | 501=false；Zod=false；破图=0；溢出=false |
| 情绪识别 | `/ai-insight/emotion` | `AI 服务不可用：尚未运行`，0 条 | 501=false；Zod=false；破图=0；溢出=false |
| 员工评分 | `/ai-insight/employee-score` | `AI 服务不可用：尚未运行`，0 条 | 501=false；Zod=false；破图=0；溢出=false |
| 沟通关键词 | `/ai-insight/communication-keyword` | `AI 服务不可用：尚未运行`，0 条 | 501=false；Zod=false；破图=0；溢出=false |

专项期间 warning/error 日志为 `[]`。

## 服务器：产物、健康与重启

| 检查项 | 最终记录 |
| --- | --- |
| 代码提交 | `552c0a18f5f569759e09d1032b1e1b042b16a544` |
| 镜像 | `sha256:4b50c04bf2911ba7eecee2f775958466627c9f52a4324799463c042464cdb9fb` |
| 归档 | `80,013,312` 字节；SHA-256 `cc62a0126219fd2f6bb9a5b082ea0407a119305b188cca34802eba10aabb4702` |
| 最终部署开始 | `2026-08-26T01:25:27+08:00` |
| 最终 app 启动 | `2026-08-25T17:25:29.020731635Z` |
| 部署后健康 | running/healthy；18080 与反向代理的 healthz/readyz 均 200 |
| 实际重启 | 请求 `2026-08-26T01:34:09+08:00`；启动 `2026-08-25T17:34:10.174164729Z` |
| 重启后健康 | running/healthy；`HEALTHZ=200`；`READYZ=200` |
| 错误日志筛查 | `panic|fatal|migration.*(fail|error)|sql.*error|cross.?tenant` 无输出 |

重启后会话分析再次 click + reload：路径正确、标题“会话分析”、不可用/尚未运行状态存在，501=false、Zod=false、破图 0、无溢出、warning/error `[]`。

## Nginx 脱敏访问状态

最终部署后的 records、status 和 filter-options 请求均为 GET 200：

- `/dashboard/ai-insight/session-analysis/records?page=1`
- `/dashboard/ai-insight/session-analysis/status`
- `/dashboard/ai-insight/smart-analysis/records?page=1`
- `/dashboard/ai-insight/smart-analysis/status`
- `/dashboard/ai-insight/emotion/records?page=1`
- `/dashboard/ai-insight/emotion/filter-options?limit=100`
- `/dashboard/ai-insight/emotion/status`
- `/dashboard/ai-insight/employee-score/records?page=1`
- `/dashboard/ai-insight/employee-score/filter-options?limit=100`
- `/dashboard/ai-insight/employee-score/status`
- `/dashboard/ai-insight/communication-keyword/records?page=1`
- `/dashboard/ai-insight/communication-keyword/filter-options?limit=100`
- `/dashboard/ai-insight/communication-keyword/status`

## 数据库只读核查

最终部署窗口起点：`2026-08-26 01:25:27 +08:00`。在 5 页刷新、53 页点击、容器重启和重启后刷新完成后（最后核查 `2026-08-26 01:35:27 +08:00`），新增数为：

```text
agents    0
rules     0
versions  0
audits    0
```

对应表为 `mochat_go_ai_agents`、`mochat_go_ai_analysis_rules`、`mochat_go_ai_analysis_rule_versions`、`mochat_go_ai_settings_audits`。查询仅读取创建时间和计数，不输出业务内容或密文。

第一版试运行的事后核查也确认：现有系统助手、规则、版本均创建于 `2026-08-25 19:00:21`，早于试部署，试运行窗口无新增审计；不存在需要回滚的数据变更。
