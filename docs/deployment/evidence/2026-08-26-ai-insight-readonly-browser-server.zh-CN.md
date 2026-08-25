# AI 洞察只读工作区：浏览器与服务器脱敏验收证据

日期：2026-08-26（Asia/Shanghai）

本文件固化 2026-08-26 部署后的可复核验收记录。内容已脱敏：不包含客户端 IP、User-Agent、Cookie、登录凭据、环境变量值、密钥、Token 或业务响应正文。

## 证据边界与采样方法

- 浏览器为已登录的真实 Dashboard 会话，视口宽度为 1280 像素。
- 每个路由均通过主菜单的真实链接逐项点击；页面完成路由与接口加载后，采样 `main` 文本、图片、文档宽度及浏览器日志。
- 53 个路由按 7 批执行；每批的 `failures` 均为空。统计项含义为：`clickError` 为点击/跳转错误，`501/notMigrated` 为 HTTP 501 或未迁移提示，`Zod` 为 Zod 原始校验错误，`brokenImages` 为破图数量，`overflow` 为横向溢出数量。
- `.superpowers/sdd/evidence/ai-insight-session-analysis-post-restart.png` 仅是重启后“会话分析”单页的辅助证据；完整 5 页刷新和全部 53 页覆盖，以本文件的结构化点击记录与下文脱敏 Nginx 访问日志为准。

## 浏览器：53 路由点击记录

| 批次 | 路由数 | 路由（按实际点击顺序） | 结果 |
| --- | ---: | --- | --- |
| 1 | 8 | `/index`；`/chat/v2-all`；`/chat/v2-staff`；`/chat/v2-customer`；`/chat/v2-group`；`/chat/trajectory`；`/chat/export`；`/chat/file-audio` | `failures=[]`；`clickError=0`；`501/notMigrated=0`；`Zod=0`；`brokenImages=0`；`overflow=0` |
| 2 | 8 | `/chat/resign-staff`；`/chat/refuse-archive`；`/customer/inheritance`；`/ai-insight/v2/risk`；`/ai-insight/v2/sensitive-word`；`/ai-insight/v2/timeout`；`/ai-insight/v2/customer-loss`；`/ai-insight/v2/message-intercept` | `failures=[]`；`clickError=0`；`501/notMigrated=0`；`Zod=0`；`brokenImages=0`；`overflow=0` |
| 3 | 8 | `/ai-insight/v2/keyword-library`；`/ai-insight/v2/silent-customer`；`/ai-insight/session-analysis`；`/ai-insight/smart-analysis`；`/ai-insight/emotion`；`/ai-insight/employee-score`；`/ai-insight/communication-keyword`；`/acquisition/v2-channel-code` | `failures=[]`；`clickError=0`；`501/notMigrated=0`；`Zod=0`；`brokenImages=0`；`overflow=0` |
| 4 | 8 | `/acquisition/group-code`；`/acquisition/redirect-link`；`/acquisition/wechat-customer-service`；`/acquisition/live-code-short-chain`；`/acquisition/group-template`；`/acquisition/precise-group-send`；`/acquisition/friends-circle`；`/acquisition/material-management` | `failures=[]`；`clickError=0`；`501/notMigrated=0`；`Zod=0`；`brokenImages=0`；`overflow=0` |
| 5 | 8 | `/customer/clue/default`；`/customer/contact`；`/customer/friends`；`/customer/opportunity`；`/customer/public-sea`；`/customer/group`；`/customer/tags`；`/customer/order` | `failures=[]`；`clickError=0`；`501/notMigrated=0`；`Zod=0`；`brokenImages=0`；`overflow=0` |
| 6 | 8 | `/customer/settings`；`/data/customer`；`/data/employee`；`/data/conversion`；`/data/behavior`；`/data/report`；`/ai-setting/ai-knowledge-base`；`/ai-setting/agent` | `failures=[]`；`clickError=0`；`501/notMigrated=0`；`Zod=0`；`brokenImages=0`；`overflow=0` |
| 7 | 5 | `/company-setting/website`；`/company-setting/staff`；`/setting/role`；`/setting/additional`；`/setting/authorization` | `failures=[]`；`clickError=0`；`501/notMigrated=0`；`Zod=0`；`brokenImages=0`；`overflow=0` |

汇总：53/53 页面可达，合计 `clickError=0`、`501/notMigrated=0`、`Zod=0`、`brokenImages=0`、`overflow=0`，最终 `devLogs=[]`。

## 浏览器：5 个修复页刷新记录

以下页面在完成点击覆盖后另行逐页刷新。每页结果均为 `has501=false`、`hasZod=false`、`broken=0`、`overflow=false`、`logs=[]`：

| 页面 | 路由 | 页面状态 |
| --- | --- | --- |
| 会话分析 | `/ai-insight/session-analysis` | `AI 服务暂不可用：尚未运行`，真实 0 条空态 |
| 智能分析 | `/ai-insight/smart-analysis` | `AI 服务暂不可用：尚未运行`，真实空态 |
| 情绪识别 | `/ai-insight/emotion` | `AI 服务不可用：尚未运行`，真实空态 |
| 员工评分 | `/ai-insight/employee-score` | `AI 服务不可用：尚未运行`，真实空态 |
| 沟通关键词 | `/ai-insight/communication-keyword` | `AI 服务不可用：尚未运行`，真实空态 |

## 服务器：部署、重启与健康记录

| 检查项 | 已验证记录 |
| --- | --- |
| 部署后容器启动 | `2026-08-25T16:38:23.010839334Z` |
| 实际重启后容器启动 | `2026-08-25T16:52:23.542953795Z`（北京时间 2026-08-26 00:52:23） |
| 容器与镜像 | 部署后及实际重启后均为 `running / healthy`；镜像为 `sha256:fa56704a6ed676de2812b0899fecf39ad9fb8a0cea053f29e73d3d4a0ddb1673` |
| 重启后 HTTP 健康端点 | `HEALTHZ_HTTP=200`；`READYZ_HTTP=200` |
| 关键错误筛查 | 模式 `panic|fatal|migration.*(fail|error)|sql.*error|cross.?tenant` 无输出 |
| 镜像归档 | 大小 `80014336` 字节；本地与服务器端 SHA-256 均为 `e6cdc3b76fbca9c406919bb17113cd1c3f6d283230edc696778be4e4d66d2167` |

重启后再次采样“会话分析”：`heading=会话分析`，`status=AI 服务暂不可用：尚未运行`，`has501=false`，`hasZod=false`，`broken=0`，`overflow=false`，`logs=[]`。

## 服务器：脱敏 Nginx 访问日志摘录

采样窗口：2026-08-26 00:51:08–00:51:15（+08:00）。下列请求全部为 `GET`、HTTP 200。此处仅记录路径、方法和状态，未保留客户端标识、认证信息或响应正文。

| 页面 | 已确认的请求路径 |
| --- | --- |
| 会话分析 | `/dashboard/ai-insight/session-analysis/records?page=1`；`/dashboard/ai-insight/session-analysis/status` |
| 智能分析 | `/dashboard/ai-insight/smart-analysis/records?page=1`；`/dashboard/ai-insight/smart-analysis/status` |
| 情绪识别 | `/dashboard/ai-insight/emotion/records?page=1`；`/dashboard/ai-insight/emotion/filter-options?limit=100`；`/dashboard/ai-insight/emotion/status` |
| 员工评分 | `/dashboard/ai-insight/employee-score/records?page=1`；`/dashboard/ai-insight/employee-score/filter-options?limit=100`；`/dashboard/ai-insight/employee-score/status` |
| 沟通关键词 | `/dashboard/ai-insight/communication-keyword/records?page=1`；`/dashboard/ai-insight/communication-keyword/filter-options?limit=100`；`/dashboard/ai-insight/communication-keyword/status` |

这些访问记录证实修复后工作区接口可达且返回 HTTP 200；不以日志响应正文推断或披露任何业务数据。
