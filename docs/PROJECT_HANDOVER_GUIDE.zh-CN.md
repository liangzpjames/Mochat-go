# MoChat Go 项目接手与商业化对标指南

> 文档版本：2026-07-22  
> 分析范围：当前 `mochat-go` 交付目录、现有测试与生产证据、圆弧公开官网资料  
> 目标读者：即将接手产品、后端、前端、测试、运维和商业化工作的负责人

## 1. 先说结论

本项目不是一个从零开始的企业微信 Demo，而是把开源 MoChat 的 PHP/Hyperf + Vue 系统迁移为可独立运行的 Go 版本，并在此基础上补充 SaaS 多租户、计费、审批、安全、备份、合规和平台总后台。

当前资产可以概括为：

- 业务功能覆盖广：客户、员工、客户群、标签、渠道活码、欢迎语、素材、群发、SOP、裂变、抽奖、雷达、门店活码、批量加好友、客户继承、会话存档、敏感词、统计等已有实现或兼容路由。
- SaaS 平台能力已超出普通开源 SCRM：有租户、套餐、配额、用量、订阅、支付、退款、发票、结算、域名、品牌、RBAC、双人审批、审计、服务账号、MFA、备份恢复、合规删除和发布证据等能力。
- 工程验证资产很多：443 个 Go 文件、181 个 Go 测试文件、96 个增量迁移、百余个 shell 验收/审计脚本；已有路由、队列、租户隔离、配额、前端浏览器和故障恢复等专项门禁。
- 仍不是可以直接承诺对外售卖的生产最终态：仓库自己的生产证据明确显示 `ready=false`，尚缺 6 类真实外部证据；真实企业微信、真实微信开放平台、双真实租户、生产域名和目标环境稳定性等不能由 fake server 或本地 smoke 替代。
- 最大技术风险不是“功能完全没有”，而是“复杂度集中”：`internal/dashboard/saas_admin_page.go`、`internal/store/mysql.go`、`internal/dashboard/saas_admin.go` 分别约 860 KB、840 KB、696 KB，入口 `cmd/mochat-go/main.go` 也承担大量装配和开关判断。继续堆功能会快速降低可维护性。
- 前端资产不对称：新 SaaS 总后台有 React/TypeScript 源码；原 dashboard、sidebar、operation 在当前交付中主要是编译后的 Vue 2 静态产物，不能像正常源码项目一样低成本迭代。这是接手后必须尽早解决的关键问题。
- 与圆弧的最大差距在 AI 销售闭环：当前项目有会话存档同步、敏感词和群质检等基础，但未发现完整的 LLM/向量检索/知识库/提示词评测/AI 成本治理实现。圆弧公开能力重点是会话摘要、情绪、客户需求、客户质量、流失风险、员工评分、话术建议、知识库、商机预测和经营洞察。

因此推荐的接手策略不是重写全部，也不是立即继续增加零散营销功能，而是先完成“真实生产闭环 + 可维护性止血”，随后用会话数据平台和 CRM 销售闭环承接 AI。

## 2. 需求背景与项目定位

### 2.1 原始背景

README 和独立化缺口文档说明，本项目源自 MoChat PHP 版本的迁移。迁移期曾支持把未迁移请求转发到 PHP upstream；现在的主验收口径是：

1. `MOCHAT_GO_STANDALONE=1` 独立运行；
2. 不依赖原 `mochat/` PHP/Vue 源码目录；
3. 不依赖 PHP Hyperf upstream；
4. 不依赖外部兼容清单；
5. 未迁移路由不能用 fallback 冒充完成。

项目仍保留兼容模式和 PHP 对照测试，是迁移工具，不应成为新生产部署的默认架构。

### 2.2 产品形态

系统实际上包含四个面向不同角色的产品面：

| 产品面 | 用户 | 入口/产物 | 主要职责 |
| --- | --- | --- | --- |
| 企业管理后台 | 企业管理员、运营、销售主管 | `web/dashboard/dist`，主服务 `/dashboard/*` | 企微配置、客户运营、群运营、营销活动、统计、权限 |
| 企微侧边栏 | 一线员工 | `web/sidebar/dist`，独立端口或 `/sidebar-app/` | 客户详情、互动轨迹、SOP、素材、批量加好友等 |
| 营销活动 H5 | 外部联系人/参与者 | `web/operation/dist`，独立端口或 `/operation-app/` | 裂变活动、OAuth、海报、助力和领奖 |
| SaaS 总后台 | 平台运营、财务、安全、交付人员 | `web/saas-admin` React 应用及 `/dashboard/saasAdmin/*` | 租户、套餐、经营、续费、财务、告警、审批、安全、交付 |

### 2.3 商业化目标

用户希望最终达到可对外售卖标准，并对标圆弧 AI 会话/SCRM。这里的“可售卖”至少同时包含：

- 可用：主流程真实可跑，不依赖模拟企微；
- 可管：有正常的平台总后台和租户后台；
- 可计量：套餐、席位、资源、AI Token、存储、调用量可核算；
- 可收费：订单、支付、退款、发票、续费、欠费处置闭环；
- 可交付：授权安装、域名、初始化、培训、迁移、验收标准化；
- 可运维：监控、告警、备份、恢复、升级、回滚、SLA；
- 可合规：会话存档授权、个人信息保护、数据保留和删除、审计；
- 可支持：客服、工单、使用文档、版本公告和客户成功体系。

## 3. 技术栈与具体框架

### 3.1 后端

| 领域 | 技术 | 说明 |
| --- | --- | --- |
| 语言 | Go 1.26（`go.mod`/Dockerfile 声明） | 当前 `go.mod` 的版本要求很新，CI/开发机必须统一工具链；正式接手时应确认该版本在目标构建环境实际可获得并锁定镜像摘要 |
| HTTP | Go 标准库 `net/http` | 未使用 Gin/Echo/Fiber；路由与 handler 由 `internal/server` 和大量 Option 装配 |
| 数据库 | MySQL/MariaDB，`go-sql-driver/mysql` | 主部署使用 MariaDB 10.6；另有 MySQL 5.7 兼容 lint/CI |
| 缓存/队列 | Redis 7，`go-redis/v9` | JWT 黑名单、会话缓存、异步语义队列和任务处理 |
| 对象存储 | 本地文件 + MinIO SDK | 本地静态上传目录；SaaS 存储账本；需核实生产是否真正接入 S3/MinIO |
| 密码学 | `x/crypto`、独立凭据管理包 | JWT、bcrypt/密码、凭据加密、MFA、审计签名、密钥轮换 |
| 配置 | 环境变量 | `internal/config/config.go` 体量很大，所有功能开关、密钥和 cron 参数集中解析 |
| 迁移 | 自研 `cmd/mochat-migrate` + SQL up/down | baseline、apply、rollback、checksum、replay；96 个增量 up migration |
| 后台任务 | 自研 Redis taskrunner + 进程内 cron | 队列和 cron 与 Web 服务同进程装配，可独立开关但尚未形成独立 worker 部署单元 |

后端不是典型的 Controller-Service-Repository 分层框架，而是以包内 handler + store 接口/实现为主：

```text
HTTP 请求
  -> internal/server 路由和兼容响应
  -> internal/dashboard 业务 Handler / Worker / Cron
  -> internal/store 接口及 MySQLStore/RedisStore
  -> MySQL、Redis、企业微信/微信开放平台、文件存储
```

### 3.2 前端

| 前端 | 技术 | 当前源码状态 |
| --- | --- | --- |
| SaaS 总后台 | React 19、TypeScript 5.8、Vite 6、TanStack Query 5、Tailwind CSS 4、Radix Dialog、Lucide、Sonner | 有完整源码，位于 `web/saas-admin/src` |
| 原企业后台 | Vue 2.6、Vue Router 3.1、Vuex 3.1、Axios 0.19、ECharts 等 | 当前目录主要只有 `web/dashboard/dist` 构建产物 |
| 企微侧边栏 | Vue 2 构建产物 | 当前主要只有 `web/sidebar/dist` |
| 营销活动 H5 | Vue 2 构建产物 | 当前主要只有 `web/operation/dist` |

特别注意：Go 静态托管层会为旧构建产物重写 publicPath、Router base、API base 等。这解决了独立交付问题，但不是可持续前端开发方式。后续若需要持续对标商业产品，应找回上游可修改源码并确认 GPL 交付义务，或者立项逐页迁移到新前端，而不是直接改 minified JS。

### 3.3 部署与基础设施

- Docker 多阶段构建，最终基于 Alpine 3.22，以 UID 10001 非 root 用户运行。
- standalone Compose 包含 `app + mariadb:10.6 + redis:7-alpine`。
- App 对外暴露 8080/8081/8082 三个服务口，健康检查是 `/readyz`。
- 数据卷包括应用上传、审计锚、MySQL、Redis。
- Dockerfile 同时产出主服务、迁移、初始化和 SaaS 维护四个二进制。
- 唯一可见 GitHub Workflow 是 MySQL 5.7 amd64 专项；尚未看到完整的主分支 CI/CD、镜像签名、漏洞扫描、灰度发布和生产部署流水线。

## 4. 目录结构与阅读顺序

```text
mochat-go/
├─ cmd/
│  ├─ mochat-go/                 # 主进程，依赖装配、路由开关、worker/cron 启动
│  ├─ mochat-migrate/            # 数据库迁移 CLI
│  ├─ mochat-bootstrap/          # 初始化租户/管理员/核心数据
│  ├─ mochat-inventory/          # PHP 迁移期清单生成与对齐工具
│  └─ mochat-saas-maintenance/   # 备份、恢复、合规、审计等维护命令
├─ internal/
│  ├─ dashboard/                 # 最大业务包：后台 API、SaaS、企微业务、worker、cron
│  ├─ store/                     # MySQL/Redis 数据访问及大量领域账本
│  ├─ server/                    # HTTP 路由注册、compat/fallback、静态入口
│  ├─ config/                    # 环境变量配置与默认开关
│  ├─ authjwt/ session/          # 登录、JWT、黑名单、会话
│  ├─ identitysecurity/          # MFA、持久会话、身份事件
│  ├─ taskrunner/                # Redis 队列执行器、幂等和处理状态
│  ├─ migration/ mysqlconn/      # 数据库连接和迁移内核
│  ├─ frontend/                  # 静态前端托管/重写辅助
│  ├─ wecomcredentials/          # 企业微信凭据加密与轮换
│  ├─ wechatopencredentials/     # 微信开放平台凭据加密与轮换
│  ├─ saasalertcredentials/      # 通知凭据加密与轮换
│  ├─ saasbackup/                # 加密备份、校验、恢复
│  ├─ saascompliance/            # 数据导出、legal hold、删除流程
│  ├─ saasauditanchor/           # 审计链、签名和远端不可变锚
│  ├─ serviceaccountkey/         # 平台服务账号 API Key 管理
│  ├─ outboundhttp/              # SSRF/出站访问保护
│  └─ clientip/ buildinfo/       # 代理 IP 解析和构建指纹
├─ web/
│  ├─ dashboard/dist/            # 旧企业后台编译产物
│  ├─ sidebar/dist/              # 旧企微侧边栏编译产物
│  ├─ operation/dist/            # 旧营销 H5 编译产物
│  └─ saas-admin/                # 新 React SaaS 总后台源码与 dist
├─ deploy/
│  ├─ standalone/                # 独立部署 Compose、完整 schema、96 个增量迁移
│  ├─ mysql57/                   # MySQL 5.7 专项验证环境
│  └─ local/                     # 迁移期 PHP/MySQL/Redis 联调环境
├─ scripts/                      # 单测门禁、审计、smoke、浏览器回归、生产证据采集
├─ docs/                         # 独立化、SaaS、发布、支付桥、生产证据
├─ storage/                      # 本地上传目录
├─ output/                       # 浏览器截图、备份、临时运行证据；不应作为源码依赖
├─ Dockerfile
├─ go.mod / go.sum
└─ README.md
```

建议新开发者按以下顺序阅读：

1. 本文档；
2. `docs/standalone-gap.md` 与 `docs/release-candidate.md`；
3. `cmd/mochat-go/main.go`，只看生命周期和模块装配；
4. `internal/server/server.go`，了解实际 URL 如何映射；
5. 选择一个小模块，阅读 `dashboard handler -> store interface -> MySQLStore -> smoke` 全链路；
6. 再读 SaaS 总后台，不要一开始硬啃三个超大文件；
7. 修改数据库前先读 `deploy/standalone/migrations/README.md` 和迁移测试。

## 5. 代码结构与运行机制

### 5.1 启动流程

`cmd/mochat-go/main.go` 大致执行：

1. 从环境变量读取配置并确定 standalone/兼容模式；
2. 初始化三类外部凭据加密器和安全状态；
3. 延迟创建 MySQLStore、RedisStore；
4. 初始化身份安全、客户端 IP、域名 DNS 验证、服务账号密钥等；
5. 根据数百个迁移/能力开关创建 handler 并追加 `server.Option`；
6. 组装企业微信 API、微信开放平台、通知、备份、合规等依赖；
7. 注册静态前端、健康检查、dashboard/sidebar/operation/SaaS 路由；
8. 按配置启动 Redis workers 和进程内 cron；
9. 启动主 HTTP 服务及可选的两个静态前端端口。

standalone + MySQL DSN + JWT Secret 时默认启用全部已迁移业务路由。单路由开关主要服务迁移、灰度和测试，不建议新功能继续无限增加同类布尔变量。

### 5.2 数据访问

项目有一定接口抽象，但 `internal/store/mysql.go` 已成为巨型实现，且围绕 SaaS、支付、安全又拆出若干 store 文件。典型约定包括：

- 旧 MoChat 表多以 `mc_` 开头；
- 新 Go/SaaS 控制表多以 `mochat_go_` 开头；
- 删除常采用软删除字段；
- 租户通过 tenant/corp 关联实现隔离；
- 文件写入后同步维护 SaaS 存储对象账本和 `storage_mb` 用量；
- 关键平台操作同时写操作审计、审批效果或账本。

新开发必须先确认：查询是否带租户范围、写操作是否刷新用量、删除是否回收存储、外部 API 失败是否回滚、重试是否幂等。

### 5.3 异步队列与定时任务

现有门禁记录 11 个 Go 语义队列，覆盖异步上传、打标签、自动标签关键词、消息提醒、客户群/客户/部门同步、素材 media_id、员工统计、企微客户标签远程写、企微回调、企业授权初始化等场景。

主要 cron 包括：企微应用拉取、员工统计、渠道码刷新、客户/群群发、发送结果同步、标签建群、企业日报、素材刷新、客户继承状态、会话存档同步、敏感词监控、SaaS 通知/运营任务/订阅/支付结算等。

当前 Web、worker、cron 可在同一二进制中启动。商业化规模增长后建议拆分角色：

```text
api deployment / worker deployment / scheduler singleton / maintenance job
```

否则扩容 API 会重复启动 cron，长任务也会影响请求延迟和发布优雅退出。

## 6. 已有业务能力地图

### 6.1 基础组织与权限

- 登录、登出、JWT、Redis 黑名单、MFA、持久会话；
- 企业切换、企业授权、员工和部门同步；
- 用户、角色、菜单、权限、成员管理；
- 企微 OAuth、微信开放平台 OAuth、JSSDK；
- 操作日志和平台审计。

### 6.2 客户与群运营

- 客户列表、详情、画像字段、互动轨迹、来源、标签；
- 客户群列表、详情、统计、分组；
- 客户标签、自动标签、关键词/入群/时段触发；
- 批量加好友、客户导入、分配和提醒；
- 在职/离职客户与客户群继承；
- 好友流失相关数据和统计基础；
- 入群欢迎语、群提醒、群日历、群质检、群打卡。

### 6.3 获客与营销

- 渠道活码及来源统计；
- 门店活码；
- 自动拉群、标签建群、无限拉群；
- 客户群发、客户群群发及发送结果；
- 素材库；
- 个人 SOP、群 SOP；
- 任务宝/员工裂变、群裂变；
- 抽奖、互动雷达。

### 6.4 会话存档与风控

- 企业微信会话存档 bridge 同步；
- 分表保存多种会话消息；
- 员工/客户/会话查询路由；
- 敏感词库、分组、启停、命中记录；
- 定时敏感词扫描；
- 群质检与运营统计基础。

需要区分“有会话同步/查询代码”与“已通过真实企微会话存档生产验证”。当前生产证据仍把真实企微联调列为缺失项。

### 6.5 SaaS 总后台

已有领域非常全面：

- 总览：租户规模、经营指标、趋势、风险和 readiness；
- 租户：创建、启停、续费、生命周期、套餐分配、配额和用量；
- 运营：待办队列、负责人分配、提醒、客户成功、风险跟进；
- 财务：订阅、支付订单、退款、发票、贷项、结算和对账桥；
- 通知：租户告警配置、持久 outbox、重试、健康、SLO；
- 治理：平台 RBAC、高风险双人审批、策略变更、乐观锁和快照；
- 安全：MFA、会话、登录事件、事件处置、服务账号、密钥轮换；
- 合规与韧性：数据导出、legal hold、删除、备份、异地副本抽象、隔离恢复；
- 交付：品牌、租户域名、DNS 验证、域名交付、发布候选和证据门禁。

React 前端现有页面按信息架构分为 Overview、Tenants、Packages、Team、System、Launch，后端能力比这 6 个源码页面表现出来的更宽，另有大体量 Go 原生 SaaS 页面。后续应统一前端，避免两个总后台视图长期并存。

## 7. 测试体系与正确使用方式

### 7.1 快速门禁

`scripts/test.sh` 包含：

- 独立性、验收清单、路由、功能模块、登录企业校验等静态审计；
- 队列/企微回调/用量/存储回收覆盖审计；
- 生产证据规则 smoke；
- `go test ./...`；
- `go vet ./...`；
- 5 个命令构建。

### 7.2 完整验收

`scripts/standalone_acceptance.sh` 支持：

- `core`：构建、迁移、独立包、Compose、路由、bootstrap、队列幂等；
- `saas`：租户隔离、配额、存储、总后台、财务、安全、备份、合规、审批；
- `workers`：Redis 语义队列；
- `cron`：企微同步、群发、会话存档、敏感词和平台任务；
- `frontend`：静态资源、API smoke、Playwright 真实浏览器页面；
- `mysql57`：兼容 lint 和真实迁移 smoke；
- `php`：仅迁移期对照，默认跳过。

### 7.3 接手后的测试金字塔

现有体系强在集成 smoke，后续要补齐：

1. 领域单元测试：规则、状态机、额度、评分、AI JSON 解析；
2. Repository 契约测试：租户隔离、事务、并发和幂等；
3. API 契约：OpenAPI/schema 与旧前端字段兼容；
4. 集成测试：真实 MySQL/Redis 和 fake 企微；
5. 真实沙箱 E2E：测试企业微信、微信开放平台和会话存档；
6. 浏览器 E2E：租户后台、侧边栏、总后台、移动端；
7. 非功能测试：性能、容量、混沌、备份恢复、升级回滚、安全扫描；
8. AI 评测：金标集、准确率、幻觉、敏感信息、延迟、Token 成本、模型回退。

每个新功能至少应有：成功、权限拒绝、跨租户拒绝、参数错误、外部 API 失败、重试幂等、额度耗尽、审计记录八类验证中的适用项。

## 8. 当前成熟度与主要风险

| 领域 | 判断 | 证据/风险 |
| --- | --- | --- |
| 功能覆盖 | 中高 | 224 条迁移清单路由和大量原生补充路由；但覆盖不等于真实业务可用性 |
| 单元/集成验证 | 中高 | 181 个 Go 测试文件和大量 smoke；执行耗时、稳定性及 Windows 可运行性需再标准化 |
| 真实外部验收 | 低至中 | 生产 readiness 明确缺 6 项，真实企微/微信开放平台/双租户等未闭环 |
| 后端可维护性 | 中低 | 巨型入口、巨型 handler/page/store；领域边界弱，改动影响面大 |
| 前端可维护性 | 低至中 | SaaS React 可维护；旧三端只有 dist，严重制约产品迭代 |
| 多租户与平台治理 | 中高 | 配额、用量、隔离、审批、审计、服务账号等较完整；仍需真实渗透和并发验证 |
| 运维与发布 | 中 | 有健康、备份、证据门禁；缺完整 CI/CD、可观测性平台、生产部署代码和演练证据 |
| 商业化闭环 | 中 | 套餐/订阅/支付/退款/发票/结算数据模型较全；支付提供商桥和真实资金对账仍需生产接入 |
| AI 能力 | 低 | 未见核心 AI 技术依赖和完整数据/评测/成本治理链路 |
| 许可证 | 高风险需法务确认 | GPL-3.0 衍生项目；对外分发二进制/镜像需提供对应源码并保留许可说明，闭源售卖策略必须先做专业法律评估 |

其他直接风险：

- 当前外层和内层目录都未检测到可用 `.git` 元数据，无法从此交付目录追溯提交、作者、分支和标签；正式接手必须获得权威 Git 仓库。
- README 很大且积累了大量时间线式变更记录，不适合作为唯一运行手册；需要拆成架构、配置、运维、API、测试、故障处理文档。
- 运行配置依赖数量巨大的环境变量，容易出现环境漂移和错误组合；应增加类型化配置文件/配置 schema、启动时强校验和按角色拆分。
- MariaDB 10.6 与 MySQL 5.7 双口径会限制 SQL、JSON、索引和迁移设计；应明确正式支持矩阵和退场计划。
- 会话数据、手机号、聊天文件属于高敏感数据；上线前必须完成数据分级、脱敏、最小权限、下载水印、导出审批、审计和保留策略。

## 9. 圆弧对标分析

### 9.1 对标对象公开定位

圆弧公开官网把产品定位为企业微信上的 AI 会话管理 + SCRM，强调：

- 全量会话存档、单聊/群聊/多媒体查询、全局搜索和导出；
- 敏感词、超时未回复、红包/名片等风险行为监控；
- 渠道活码、永久群码、客户/群群发、获客助手；
- 线索、联系人、商机、公海、订单、产品价格、回款的销售闭环；
- AI 会话摘要、情绪、需求、竞品、客户质量、流失风险、员工评分、销售建议；
- 企业知识库和定制智能体；
- 员工/部门/客户增长/标签词云/会话/质检 BI；
- 按存档员工席位售卖，支持试用、不同存储期限、移动/电脑查看和售后服务。

公开页面只是营销材料，不能证明其所有功能的实现深度和性能；以下差距分析按“本项目代码中能找到的实现”对比“对方公开承诺”。

### 9.2 能力矩阵

| 能力 | 本项目现状 | 对标判断 | 建议 |
| --- | --- | --- | --- |
| 企微组织/客户/群同步 | 已有较完整实现 | 接近基础盘 | 用真实测试企业闭环并建立同步延迟/失败率 SLO |
| 渠道活码/群运营/群发 | 覆盖较广 | 基础功能有竞争力 | 补模板、效果归因、A/B、频控和全链路漏斗 |
| SOP/裂变/抽奖/雷达 | 已有 | 功能宽度不弱 | 优先修 UX、数据效果和稳定性，不继续堆同类功能 |
| 会话存档 | 有同步 cron、存储和查询基础 | 尚未达到可承诺 | 接入官方 SDK/授权、媒体解密、检索、导出、留存、容量和真实压测 |
| 风险监听 | 敏感词和群质检基础 | 部分具备 | 增加超时回复、红包/名片/删除客户、规则引擎、处置闭环 |
| CRM 销售闭环 | 客户运营强，线索-商机-报价-订单-回款偏弱 | 明显差距 | 新建标准 Sales CRM 领域，不要塞入现有营销 handler |
| AI 会话分析 | 未见完整实现 | 核心差距 | 建 AI 平台层、会话特征层、知识库、评测和成本治理 |
| BI/经营洞察 | 有统计和 SaaS 经营数据 | 部分具备 | 建统一指标口径、数仓/OLAP、可配置报表 |
| SaaS 总后台 | 后端领域非常丰富 | 平台治理可能是优势 | 统一 React 总后台、完成真实支付/交付/工单/客户成功 |
| 多租户安全/审批/审计 | 已有较深实现 | 潜在优势 | 用渗透、并发、灾备和法务合规证据把优势坐实 |
| 移动体验 | 旧侧边栏/H5，部分移动回归 | 部分具备 | 建立企微工作台/侧边栏全场景移动设计系统 |
| 售后与交付 | 代码中有运营任务，产品化不足 | 商业差距 | 增加帮助中心、工单、在线诊断、客户健康和标准实施包 |

### 9.3 不建议照抄的地方

- 不要只做一个“调用大模型后展示一段摘要”的 AI 页面；可售卖 AI 必须能追溯原会话、可配置、可评测、可申诉、可控制成本。
- 不要以官网功能数量作为唯一目标；更关键的是企微授权成功率、同步延迟、消息完整率、搜索速度、误报率和续费率。
- 不要把平台总后台和租户业务后台混为一体。平台运营看跨租户经营和交付，租户管理员只看本企业数据。
- 不要声称“合规”而没有授权记录、数据处理协议、保留策略、导出审批、访问审计和删除证据。

## 10. 达到可售卖标准的目标架构

建议以“模块化单体 + 独立异步执行单元”作为近期目标，不必立即微服务化：

```text
                            ┌─ 企业管理后台
用户/企微工作台 ─ API Gateway ├─ 企微侧边栏/H5
                            └─ SaaS 平台总后台
                                    │
         ┌──────────────────────────┼──────────────────────────┐
         │                          │                          │
     Identity/Tenant          SCRM/Marketing             Billing/Admin
  认证、租户、RBAC、审计    客户、群、SOP、CRM         套餐、订单、审批、交付
         │                          │                          │
         └────────────────── Domain Events ───────────────────┘
                                    │
                  ┌─────────────────┼─────────────────┐
                  │                 │                 │
             Worker Pool       Scheduler         AI Analysis
           企微同步/群发      单例任务编排     摘要/质检/RAG/评测
                  │                 │                 │
              MySQL/Redis      Object Storage   Search/Vector/OLAP
```

近期仍可保持一个 Go module，但必须做到：

- 按领域拆包，不再向 `dashboard` 和 `mysql.go` 追加所有逻辑；
- handler 只做协议转换，service 负责用例，repository 负责持久化；
- 外部企业微信/支付/LLM 都定义 port + adapter；
- 状态机显式化，支付、订阅、审批、AI 任务不可依赖散落 if/字符串；
- 用 outbox/event 保证数据库写入与异步任务的一致性；
- API/worker/scheduler 使用同一代码但不同启动角色；
- 搜索和分析按容量引入 OpenSearch/Elasticsearch、ClickHouse 或同类组件，避免所有报表压 MySQL。

## 11. AI 会话分析落地设计

### 11.1 第一阶段能力

优先做能直接产生客户价值且可验证的能力：

1. 会话阶段摘要：按客户/群/日/跟进阶段增量总结；
2. 结构化需求抽取：需求、预算、时间、决策人、竞品、异议、下一步；
3. 情绪和流失风险：输出等级、理由和证据消息；
4. 员工会话质检：规则分 + AI 分，支持申诉和人工复核；
5. 下一步行动和话术建议：结合企业知识库并引用来源；
6. 自动标签和 CRM 字段回填：必须先由租户配置字段映射和置信度阈值。

### 11.2 必需的数据链路

```text
企微消息解密入库
 -> 标准消息模型（文本/图片 OCR/语音 ASR/文件解析）
 -> 会话切片与参与者/客户/商机关联
 -> 脱敏与授权检查
 -> AI 任务队列
 -> 模型路由和结构化输出校验
 -> 结果版本化保存（模型、提示词、输入哈希、证据）
 -> 人工反馈/申诉
 -> BI、CRM 回填和告警
```

### 11.3 AI 平台治理

- 租户级模型开关、数据范围、提示词模板和预算；
- 模型供应商适配，支持公有模型和私有化模型；
- PII 脱敏、敏感会话禁止出域、数据不用于供应商训练的合同约束；
- JSON Schema 校验、重试、降级和人工兜底；
- 每次分析记录模型/版本/Token/耗时/费用/输入输出哈希；
- 金标数据集、离线评测、线上抽检、误报/漏报和漂移监控；
- 知识库文档权限继承、分段版本、引用溯源和过期机制；
- AI 结果只作为建议，高风险审批、员工处罚、客户删除等不得自动执行。

## 12. 分阶段实施路线图

### Phase 0：接管与止血（2～4 周）

- 获取权威 Git 仓库、上游前端源码、部署密钥清单和现网拓扑；
- 固化可重复开发环境，确认 Go 版本和 Windows/WSL/Docker 路径；
- 跑通 `scripts/test.sh` 和分组 acceptance，记录基线耗时/失败率；
- 把所有密钥从文件/环境临时配置迁入 Secret Manager；
- 为三个超大领域文件建立拆分边界和禁止继续增长规则；
- 建立缺陷、需求、风险和生产证据四个台账。

退出标准：任何人可从空机器按文档启动；主门禁稳定；已知生产阻塞项有人负责、有截止时间。

### Phase 1：真实生产闭环（4～8 周）

- 完成现有 6 类生产证据：MySQL 5.7 amd64、真实企微、真实微信开放平台、双真实租户、生产域名前端、目标环境稳定性和外部监控；
- 完成真实会话存档授权、解密、媒体、检索、导出和留存验证；
- 接入监控、日志、trace、错误追踪、告警路由和 on-call；
- 完成异地备份、隔离恢复、RPO/RTO 演练；
- 完成安全测试、依赖/SBOM/镜像扫描和租户越权专项；
- 明确 GPL 商业模式和源码交付流程。

退出标准：生产 readiness 全绿；真实租户连续运行；P0/P1 缺陷清零；恢复演练达到目标。

### Phase 2：产品与总后台完善（6～10 周）

- 统一 SaaS React 总后台，覆盖租户、套餐、运营、财务、通知、治理、安全、交付；
- 恢复/迁移租户业务后台、侧边栏和 H5 源码，建立统一设计系统；
- 打通真实支付提供商、回调、退款、发票和日对账；
- 增加工单、公告、帮助中心、客户健康、试用转付费和流失分析；
- 建立席位/会话存档/存储/AI Token 套餐和超额策略。

退出标准：销售可开通试用、运营可交付、财务可对账、客服可处理问题、管理员可自助配置。

### Phase 3：CRM 销售闭环（8～12 周）

- 线索接入、查重、分配、回收；
- 联系人/企业 360 视图；
- 商机阶段、预测、协作、报价；
- 产品、SKU、价格和折扣权限；
- 订单、合同、回款和业绩目标；
- 销售漏斗、部门/员工绩效和渠道 ROI。

退出标准：从线索到回款可追踪，关键指标口径可审计。

### Phase 4：AI 对标与差异化（8～16 周，可与 Phase 3 后半并行）

- 完成摘要、需求/竞品/情绪、客户质量、流失风险、员工评分；
- 上线知识库 RAG、话术建议和下一步行动；
- AI 自动回填客户/商机字段；
- 建立 AI 评测后台、预算、模型路由和租户配置；
- 逐步做经营智能体，但保持证据引用和人工确认。

退出标准：评测指标、响应延迟和单次成本达标；AI 功能能提高跟进完成率或转化率，而非只有演示效果。

## 13. 可售卖发布门禁

正式 GA 前建议采用以下硬门禁：

### 功能

- 新租户从授权、同步、配置到首个运营任务全链路成功；
- 企微回调、会话存档、群发、标签和继承的异常可重试且不重复；
- 套餐、额度、续费、欠费、退款、发票和对账闭环；
- 平台总后台与租户后台权限完全分离。

### 质量

- 单元/契约/集成/E2E 全绿，关键路径覆盖率目标明确；
- 目标容量下 P95/P99、同步延迟、搜索延迟达标；
- 72 小时以上稳定性与故障注入通过；现有文档不要求 24 小时并不等于 GA 可以免除稳定性证据；
- 数据迁移、升级、回滚和兼容性验证通过。

### 安全与合规

- 无已知 Critical/High 漏洞；
- 完成租户越权、SSRF、上传、导出、服务账号和 OAuth 专项；
- 密钥进入 KMS/Secret Manager，轮换可演练；
- 数据处理协议、隐私政策、授权记录、保留/删除和审计流程齐全；
- GPL 和第三方许可交付经法务确认。

### 运维和服务

- SLO、SLA、RPO、RTO 有明确数字；
- 监控、告警、runbook、值班、事故复盘机制可用；
- 异地备份和真实隔离恢复通过；
- 帮助中心、培训材料、工单分级、售后响应时间和版本公告齐全。

## 14. 新需求开发规范

建议每个功能采用以下落地模板：

1. 写 PRD：角色、场景、边界、指标、权限、套餐、审计、异常；
2. 定义领域和数据所有权，不向巨型文件直接堆逻辑；
3. 写 API 契约和数据库 migration；
4. 先补失败测试，再写实现；
5. 外部 API 必须有 adapter、超时、重试、幂等和 fake；
6. 所有读写带租户范围；平台跨租户读必须走独立权限；
7. 资源创建/删除同步维护用量和存储账本；
8. 高风险动作写审计，必要时进入双人审批；
9. 补 Go test、store 契约、smoke 和浏览器 E2E；
10. 更新本文能力矩阵、运维手册、套餐说明和发布说明。

新模块推荐目录示意：

```text
internal/salescrm/
├─ domain/          # Entity、Value Object、状态机、领域规则
├─ application/     # Use case/service、命令、查询
├─ ports/           # Repository、企微、LLM、支付接口
├─ adapters/        # MySQL、Redis、HTTP provider 实现
└─ transport/http/  # 请求解析、鉴权、响应映射
```

## 15. 本地启动与常用命令

本仓库脚本以 Bash 为主；在 Windows 上建议使用 WSL2 或 Git Bash，并保证 Docker 可用。

```bash
# 快速静态、单测、vet、build 门禁
./scripts/test.sh

# 完整验收（耗时且会启动多个临时容器）
./scripts/standalone_acceptance.sh

# 分组验收
MOCHAT_ACCEPTANCE_SUITE=core ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=saas ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=workers ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=cron ./scripts/standalone_acceptance.sh
MOCHAT_ACCEPTANCE_SUITE=frontend ./scripts/standalone_acceptance.sh

# 独立 Compose
MOCHAT_SIMPLE_JWT_SECRET='replace-with-strong-secret' \
docker compose -f deploy/standalone/docker-compose.yml --profile app up -d --build

# SaaS React 总后台
cd web/saas-admin
pnpm install
pnpm run typecheck
pnpm run build
```

启动生产模式前不要直接复制 README 中的示例密钥；必须设置独立 JWT、sidebar JWT、企微/微信开放平台/告警凭据密钥、MFA 密钥、服务账号 pepper、审计签名和备份密钥。

## 16. 接手第一周检查表

- [ ] 获得权威 Git 远端、分支策略、提交历史和当前生产 tag；
- [ ] 获得旧 dashboard/sidebar/operation 的可编辑源码；
- [ ] 明确当前是否已有线上客户、域名、企微应用和会话存档授权；
- [ ] 在干净环境跑通 core、saas、frontend 三组验收；
- [ ] 查看 `docs/evidence/production/readiness.json` 的 6 项缺口并分配负责人；
- [ ] 列出全部环境变量、密钥所有者、轮换周期和生产存储位置；
- [ ] 核实 MySQL/MariaDB 正式支持矩阵和数据规模；
- [ ] 核实支付、短信/通知、对象存储、监控、工单的真实供应商；
- [ ] 完成 GPL/第三方依赖商业分发法律评估；
- [ ] 确认产品路线以“真实生产闭环 -> CRM -> AI”为主线；
- [ ] 冻结向三个巨型文件继续无边界追加代码；
- [ ] 建立 P0/P1 缺陷清单和发布门禁看板。

## 17. 资料与证据索引

### 仓库内

- `README.md`：完整环境变量和迁移时间线；
- `docs/standalone-gap.md`：独立化口径与当前生产缺口；
- `docs/saas-admin-mvp-scope.md`：SaaS 总后台 MVP 边界；
- `docs/release-candidate.md`：发布候选和证据口径；
- `docs/production-evidence.md`：真实生产证据采集；
- `docs/evidence/production/readiness.json`：机器可读的当前 readiness；
- `docs/payment-settlement-bridge.md`：支付结算桥接协议；
- `deploy/standalone/README.md`：独立部署；
- `MODIFICATIONS.md`：相对上游的修改；
- `LICENSE`、`NOTICE.md`、`SOURCE_OFFER.md`、`THIRD_PARTY_NOTICES.md`：许可与分发义务。

### 对标公开资料（检索于 2026-07-22）

- 圆弧 AI 分析：https://www.yuanhu.com/ai-analysis/
- 圆弧 AI 会话/会话存档：https://www.yuanhu.com/product-info/
- 圆弧 SCRM 产品：https://www.yuanhu.com/product/
- 圆弧 AI 助手：https://www.yuanhu.com/product/ai-assistant/
- 圆弧智能会话：https://www.yuanhu.com/product/smart-conversation/
- 圆弧智能销售：https://www.yuanhu.com/product/smart-sales/
- 圆弧价格与完整功能清单：https://www.yuanhu.com/price/

## 18. 最终接手判断

这个项目值得继续开发，原因是企微运营功能和 SaaS 治理底座已经积累得很深，直接推倒会损失大量边界处理和测试资产。但它当前更像“迁移完成度很高、平台治理很重的候选产品”，而不是已经完成市场验证的商业 SaaS。

最正确的投资顺序是：

1. 拿回版本历史和旧前端源码；
2. 完成真实企微、会话存档、双租户、生产域名和监控证据；
3. 拆分巨型代码并统一总后台；
4. 打通真实收费、交付和售后；
5. 补线索—商机—订单—回款 CRM；
6. 最后用可评测、可追溯、可计费的 AI 会话能力形成对标和差异化。

如果前两步没有完成，继续堆 AI 或营销页面会增加演示效果，但不会显著提升可售卖程度。
