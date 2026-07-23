# MoChat Go 独立版阶段报告

- 生成时间：`2026-07-20 10:14:37 CST`
- 当前结论：**获客前 SaaS 总后台 MVP 已完成并可本地上线；生产发布仍待六项外部证据**

## 本轮 SaaS 总后台 MVP 最终版

- 参考 Nuwa Admin 的后台外壳、导航密度和组件组织方式，重新实现 React 19 + Vite 6 总后台；保留 Nuwa Apache-2.0 许可与第三方声明，不引入其 GVA 后端。
- 主导航从原先超前的成熟 SaaS 功能收口为 `概览 / 客户租户 / 套餐 / 平台账号 / 系统运维 / 上线检查` 六个首发工作区，全部读取现有 Go API，没有模拟业务数据。
- 获客前独立部署默认单人运营模式：`MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=0`。添加第二名平台管理员后改为 `1`，恢复高风险操作双人审批；Go 通用配置默认值仍保持审批开启。
- 新入口为 `/saas-admin/`，原 `/dashboard/saasAdmin/page` 返回 `307` 跳转，兼容旧书签。当前预览地址为 `http://127.0.0.1:18090/saas-admin/`。
- 本机登录页通过运行时环境变量预填预览账号；预填值仅在 `localhost`、IPv4/IPv6 回环地址输出，生产和客户域名响应不包含账号或密码。
- Playwright 已遍历六个工作区：`1440x1000` 和 `390x844` 均无页面级横向溢出，失败请求 `0`、HTTP 4xx/5xx `0`、控制台 error/warning `0`；移动端最近操作使用独立可读列表。
- 生产发布边界没有放宽：上线检查仍为 `0/6`、`ready=false`。本轮按要求不运行 24 小时测试。

## 已具备的主体证据

- 内置 manifest：路由 `224` 条，表 `70` 张，crontab `9` 个，事件处理器 `10` 个，异步队列注解 `15` 条。
- smoke 脚本：`107` 个；`standalone_acceptance.sh` 已引用 `107` 个。
- 未纳入 acceptance 的 smoke：`0` 个。
- 独立性门禁应由 `scripts/test.sh`、`scripts/audit_standalone_independence.sh`、`scripts/smoke_independent_package.sh`、`scripts/audit_manifest_route_smoke_coverage.sh` 和 `scripts/standalone_route_coverage.sh` 共同证明。

## 当前快速门禁

- 通过 `独立运行静态门禁`：standalone independence audit passed
- 通过 `独立交付包动态 smoke`：independent package smoke passed
- 通过 `acceptance smoke 接入覆盖`：acceptance suite coverage audit passed: smoke_scripts=107 referenced=107
- 通过 `manifest 路由 smoke 直接覆盖`：manifest route smoke coverage audit passed: manifest_routes=224 directly_covered=224 smoke_scripts=107
- 通过 `功能模块矩阵`：functional module matrix audit passed: modules=29 manifest_unique_paths=213/213 manifest_route_entries=224 runtime_unique_paths=589/589 runtime_route_entries=731
- 通过 `前端 dist API 覆盖`：frontend dist API coverage audit passed: surfaces=4 endpoints=234 missing=0
- 通过 `队列注解覆盖`：queue annotation coverage source=embedded:internal/server/compat_manifest_embedded.json / queue annotation coverage passed
- 通过 `企微回调事件覆盖`：wework callback event coverage audit passed: process_events=20 smoke_events=20 unit_events=18
- 通过 `worker SaaS 用量断言`：worker SaaS usage assertion audit passed
- 通过 `SaaS 指标覆盖`：SaaS metric coverage audit passed: metrics=26
- 通过 `上传存储回收覆盖`：SaaS storage reclaim coverage audit passed: functions=26 groups=14

> 说明：本节是报告生成时的当前快速审计结果；`standalone_route_coverage.sh` 会启动独立 MySQL/Redis 做运行时路由覆盖验证，仍需作为独立命令保留。

## 源码与验收指纹

- 当前指纹：`122258e6a15b7cbc66e4ba29ae0d0ab8f5b09f5a354e35a4a300934b771390a6`
- 纳入文件数：`978`
- 指纹范围：Go 源码、验收脚本、部署配置、前端构建产物和关键构建文件；排除 `docs/evidence/`、`storage/`、`output/`、`tmp/`、`node_modules` 以及私密 `.env`/`.env.*` 文件，保留 `.env.example`。

## 最近稳定性证据

- 本轮完成 Nuwa 风格总后台构建、Go 静态入口与重定向接入、前端类型检查/生产构建、全量 Go 测试、`go vet`、独立交付包动态 smoke、生产依赖漏洞审计、Docker 镜像固化和桌面/移动真实浏览器回归；`require_24h=false`，未启动 `standalone_soak_24h.sh` 或其他 24 小时运行。

## 当前运行残留

- `mochat-go-compose-app-check-app-1` 使用镜像 `sha256:4ee631989a12519fc76455911ab85b556cb214ece98feeaef3d55560f37103b6` 运行，容器健康，对外端口为 `18090/18091/18092`；`/healthz` 和 `/readyz` 返回 200，build 指纹与当前源码一致。
- `mochat-go-compose-app-check-mysql-1` 和 `mochat-go-compose-app-check-redis-1` 健康，数据库迁移账本为 `97/0097_saas_release_evidence_action_tracking`；当前 31 条审批策略在单人运营模式下返回 `required=false`。
- 最新平台健康为 `31/31`，critical、warning、未处理问题和活跃事故均为 0。发布准备仍为 `0/6`、`ready=false`，目标源码指纹来源为 build。

## 当前增量门禁与历史证据包

- 当前源码已通过 `scripts/test.sh`、全量 Go 测试、`go vet ./...`、`scripts/smoke_independent_package.sh`、四前端严格 API 覆盖、SaaS Admin 生产构建和 npm 生产依赖漏洞审计。
- `docs/evidence/latest/results.jsonl` 是 0095 阶段的历史完整证据包，仍保留用于审计，但其指纹不是当前 `122258e6...`，不能作为本轮最终源码的完整证据包。
- 本轮没有重跑完整 `standalone_acceptance.sh` 或 24 小时持续运行；实际完成的是与总后台变更直接相关的短时门禁、最终 Docker 镜像和真实浏览器验收。
- 正式证据检查仍为 `evidence_ok=false`：缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控 6 项外部证据。

## 本轮总后台客户指标业务租户口径统一

- 修复的不只是首页租户数，而是平台控制租户进入客户经营统计的系统性口径问题。平台范围现在统一声明 `tenantPopulation=business_tenants`，显式租户范围声明 `tenantPopulation=selected_tenant`；平台控制租户继续可被定向查看，但不再计入客户规模和客户经营结果。
- 统一口径已覆盖总览汇总、租户列表与指标、经营指标与趋势、续费预测、客户成功、运营待办、运营日报，以及这些链路使用的风险跟进、任务、告警、通知、通知健康、操作记录、账单和 CSV 导出查询。平台级 `tenant_id=0` 控制面操作日志仍保留在治理审计中，但不会被当成业务租户。
- 真实 MariaDB/Redis smoke 会额外写入仅属于平台租户的告警、失败通知、超大续费账单、阻断续费任务和风险跟进日志，并断言平台总览、日报、客户成功、运营待办、经营趋势、续费预测和导出均不泄漏这些记录；同时验证 `limit=1` 和显式平台租户范围仍可用。脚本输出为 `SaaS admin dashboard smoke passed`。
- 全量 `env -u GOROOT go test ./...` 通过；新增 store 查询测试和 dashboard 传播测试覆盖所有排除条件。当前预览鉴权 API 中，平台范围业务租户、客户用户、客户企业、经营租户、续费租户、客户成功租户和待办租户均为 0；显式查看平台租户仍返回 1 个租户、1 个用户、1 个企业和 1 个套餐，证明两种 population 没有混用。
- Playwright 在 `1440x1000` 和 `390x844` 下确认首页显示“业务租户 0”，页面根节点无横向溢出，控制台为 0 error/0 warning。截图位于 `output/playwright/saas-admin-business-scope-desktop.png`、`output/playwright/saas-admin-business-scope-mobile.png` 和 `output/playwright/saas-admin-business-scope-mobile-summary.png`。
- 当前预览数据库仍只有平台控制租户，没有真实客户租户，因此这证明的是口径和隔离正确，不是真实多租户生产验收。发布准备仍为 `0/6`、`ready=false`，下一步需要至少两个真实业务租户和其余目标环境证据；本轮明确不执行 24 小时运行。

## 本轮租户准备度业务租户口径修正

- 纠正上一轮把平台管理租户当成客户租户的错误口径。平台租户属于控制面，本来就不参与订阅对账、续费和生命周期处理，因此不能因没有订阅被判定为 `blocked/64%`。
- `GET /dashboard/saasAdmin/tenantReadiness` 现在明确返回 `scope=business_tenants` 和 `platformAdminTenantId`，存储查询始终排除平台管理租户；显式查询平台租户返回 `400` 和“平台管理租户不参与上线准备度”。
- 当前预览数据库只有 1 个平台管理租户、1 个用户、1 个企业、1 个套餐和 0 个订阅，所以真实结论是业务租户 `0`、可上线 `0`、待完善 `0`、已阻塞 `0`，而不是存在 1 个被阻塞的客户租户。这也不构成真实多租户验收证据。
- 本增量没有新增迁移或修改预览数据，迁移账本保持 `93/0093_saas_payment_settlement_resolve_guard`，恢复临时库残留为 0。全量 `go test ./...`、内嵌 JavaScript 编译检查、脚本语法检查和真实 MariaDB/Redis 总后台 smoke 均通过。
- 鉴权 API 验证结果为：业务租户全量查询 `200` 且返回空列表，平台租户定向查询 `400`，未认证查询 `401`。Playwright 在 `1440x900` 与 `390x844` 下确认新空态、页面无整体横向溢出、移动宽表仅在内部滚动，刷新后的控制台为 0 error/0 warning。
- 最终截图位于 `output/playwright/saas-tenant-readiness-business-scope/`。下一项真实缺口是接入至少两个真实业务租户并采集隔离、开户、订阅、企微和生产前端证据；当前仍不能标记生产最终完成，本轮未执行 24 小时运行。

## 本轮总后台工作区导航收口

- 总后台由连续长页面拆分为总览、租户套餐、运营、财务、通知、治理、安全和交付 8 个工作区；租户与范围筛选继续作为全局控制保留。
- 工作区与模块下拉按 32 项平台权限目录动态显示；权限加载完成前不会覆盖用户保存的偏好，权限确认后才恢复或回退到可访问工作区。
- 支持工作区内模块跳转、左右方向键切换和 `localStorage` 状态持久化。移动端局部横向滚动会自动把当前标签定位到可见区域，页面根节点在 `390x844` 下保持 `scrollWidth=clientWidth=390`。
- 已验证财务工作区跨重载保持、安全工作区 8 个模块可见、备份治理模块跳转位置不被粘性导航遮挡，以及安全工作区在移动端跨重载保持。
- `env -u GOROOT go test ./internal/dashboard ./internal/server -count=1`、`node --check`、`bash -n scripts/smoke_saas_admin_dashboard.sh` 均通过。最终浏览器控制台为 0 error/0 warning。
- 真实预览只读 API 全部返回 200：平台健康 `31/31`；备份 `2/2` 成功并校验通过、恢复演练 `1/1` 成功；身份中心返回会话、事件和事故数据；通知策略与平台超级管理员权限目录可读。
- 桌面和移动截图位于 `output/playwright/saas-workspace-navigation/`。含登录态的 `.playwright-cli` 临时快照已清理，未保留 JWT 文本。
- 该收口不改变生产边界：发布准备仍缺 6 类真实外部证据，异地备份副本未配置，身份安全仍为 `sessionEnforced=false`；因此当前仍不能标记生产最终完成。

## 本轮 0093 后灾备与凭据安全闭环

- 修正灾备恢复 smoke 的旧迁移断言并完整通过；变更前数据库快照为 `output/backups/mochat-preview-before-backup-health-closure-20260718-173621.sql`，权限 `0600`，SHA-256 为 `0d78f9bc4993e5b9d526be842a289cbc84484f8cbe74e6c547f138432456aad5`。
- 预览已启用 5 分钟备份调度检查、专用 `mochat_restore` 恢复账号和恢复库自动创建/清理。最终运行配置位于 `output/runtime/mochat-preview-app-runtime-20260718-181700.env`，权限为 `0600`。
- 加密备份工件 `bkp_20260718T095541Z_a87bef16cdc8.sql.gz.mgbk` 已通过摘要和迁移账本校验，随后完成隔离恢复演练；恢复结果为 `0093/93`，检查全部通过，临时恢复库清理成功且无残留。
- 现有企业微信企业凭据已从旧明文轮换到专用密钥密文，最终统计为 configured 1、encrypted 1、legacy 0、rotation required 0。通知和微信开放平台当前没有已配置业务凭据，已在零迁移风险下切换到各自专用密钥并强制加密。
- 身份安全中心、登录事件、会话和事件接口已启用并返回 200，消除了总后台原有四处 501；身份清理定时任务已启用。预览仍保留 `sessionEnforced=false` 兼容模式，生产启用强制持久会话前需先完成管理员 MFA 与会话迁移。
- 通知、企业微信、微信开放平台和身份安全四个密钥域均独立；平台健康最终为 `31/31`，critical、warning、未处理问题和活跃事故均为 0。
- `env -u GOROOT go test ./internal/saasbackup ./internal/store ./internal/dashboard`、`env -u GOROOT go test ./internal/identitysecurity ./internal/config ./internal/dashboard`、`scripts/smoke_independent_package.sh` 和 `scripts/smoke_saas_backup_recovery.sh` 已通过。快速审计为 smoke `106/106`、manifest 路由 `224/224`、功能模块 `29`、前端 API `209/209`、SaaS 指标 `26`。
- Playwright 已完成 `1440x900` 和 `390x844` 回归，页面无全局横向溢出，控制台 0 error/0 warning；证据位于 `output/playwright/saas-backup-health-closure/` 和 `output/runtime/saas-health-closure-20260718-1755/`。
- 当前备份未配置异地副本；本地运行密钥仍应在生产迁移到 KMS/Secret Manager。生产发布证据仍为 `0/6`，缺 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、至少两个真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控。本轮未启动 24 小时运行，当前不能标记生产最终完成。

## 本轮 0065 里程碑

- 新增迁移 `0065_saas_service_account_key_pepper_ring` 及完整 down rollback，为服务账号 API Key 增加 `hash_key_id`，将 Key 摘要从 JWT 签名密钥中独立出来。
- 新增 `internal/serviceaccountkey` 密钥环管理器，支持当前 pepper、JSON 历史密钥环、独立 key ID 与受控 `legacy-jwt` 兼容。生产模式可强制配置独立 pepper，全部可用旧 Key 轮换后可关闭 JWT 兼容。
- 总后台服务账号区域已展示活动 pepper key ID、独立/兼容状态、待轮换旧 Key 和缺失 key ID；系统健康新增关键探针 `service_account_key_protection`。当前预览使用 `preview-service-account-v1`，独立 pepper 已强制、JWT 兼容已关闭，活动可用 Key 1 个，可用 `legacy-jwt` Key 0 个，缺失 key ID 为 0。
- 真实 Go + MariaDB + Redis smoke 已验证独立 Key 鉴权、旧 JWT pepper 兼容、关闭兼容后 401、缺密钥时健康告警、恢复兼容和最终轮换。迁移 smoke 覆盖 65 个版本，MySQL 5.7 静态门禁覆盖 129 个 schema/迁移文件。
- 本轮一次使用了与预览相同的 Compose project name，清理时移除了预览 App/MySQL/Redis 卷。已用 `/tmp/mochat-preview-before-0064-20260713-042217.sql` 恢复到 0063，再正常应用 0064/0065；由于 WORM 对象不允许同 key 改写，已切换到新的审计锪存储世代前缀 `preview/audit-anchors-0065-recovered`，保留旧前缀不变。
- 恢复后新增加密备份 run 7，迁移账本 `65/65`、MinIO 副本成功；同时保留 `0600` 明文快照 `/tmp/mochat-preview-recovered-0065-20260713-063420.sql`，大小 809437 字节，SHA-256 为 `c727d1755a22d88a269fc63c898488140be7eb03ca6a58aa946cf90d3108c14c`。
- 完整本地短证据包于 `2026-07-13 06:41:12 CST` 至 `07:18:56 CST` 重建，`14/14` 返回 0；smoke `95/95`、manifest 路由 `224/224`、运行时唯一路径 `582/582`、运行时路由条目 `721`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `9d548af5057ebed35cad779085adfd3a8e998ea6df75ef900900d98528f3708a`，纳入 822 个文件；预览镜像为 `sha256:f22b2ac3e8f646874fac7bfb5a0aed043a02751200f3f813d213dba97222dac4`，运行日志确认 `source=build` 且指纹一致。
- Playwright 已在 `1440x1000` 和 `390x844` 验收 API Key pepper 总览和密钥台账，授权后控制台 0 error/0 warning，移动端无页面级横向溢出，宽表仅在内部滚动。
- 当前系统健康不是全绿：仍有 1 条超过 SLA 的待审批单和统计窗口内 3 次失败后台执行记录。API Key pepper、迁移和远端审计锚点探针正常。
- 本轮没有启动 24 小时运行。生产证据诊断仍缺 6 类真实外部证据，因此当前阶段不能标记生产最终完成。

## 本轮受信代理 CIDR 客户端 IP 安全收口

- 新增共享 `internal/clientip` 解析器。默认只使用 TCP 对端地址；仅当代理头开关启用且 TCP 对端属于显式受信 CIDR 时，才会从右向左解析 `X-Forwarded-For`、跳过受信代理跳点并选择第一个非受信地址，随后才回退到 `X-Real-IP` 或 TCP 对端。IPv4、IPv6、IPv4-mapped 地址、单 IP 到 `/32`/`/128` 归一化、CIDR 去重排序和非法配置均有单元测试。
- 身份安全和服务账号 OpenAPI 分别使用 `MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS`、`MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS` 控制是否读取代理头，共用 `MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS`。任一代理头开关启用但 CIDR 为空时启动失败；非法 CIDR 始终拒绝，避免把可伪造请求头当成真实来源。
- 服务账号 IP/CIDR 白名单、用量台账 IP 和最近使用 IP 已统一使用解析后的客户端地址；总后台服务账号和身份安全概览均展示“来源 IP 解析”、开关状态和受信 CIDR 数量。当前预览为 `TCP 直连`，两处 API 都返回空数组而不是 `null`。
- 真实服务账号 smoke 已验证：TCP 直连请求不满足 `10.0.0.0/8` 时返回 `403`，来自受信 `127.0.0.1/32` 代理且转发 `10.23.45.67` 时通过，并把 `10.23.45.67` 写入使用台账；把 TCP 对端移出受信 CIDR 后，同一伪造头再次被忽略并返回 `403`。身份安全 smoke 同步验证受信代理解析与非受信直连防伪造。
- 独立 Compose App smoke 使用独立项目名与备用端口通过，未影响保留预览。完整本地短证据包于 `2026-07-13 11:10:02 CST` 至 `11:55:54 CST` 重建，`14/14` 返回 0；smoke `95/95`、manifest 路由 `224/224`、运行时唯一路径 `582/582`、运行时路由条目 `721`、前端 dist API `209/209`。
- 首次证据重建的 frontend 套件暴露 Sidebar OAuth 回调的 Playwright 导航竞态。已将回调 smoke 改为以目标页到达为准、失败时重试一次，同时保留回调 `302`、JWT/Agent Cookie、目标页和全部 Sidebar API 严格断言；独立专项及完整 frontend 套件均通过，最终证据包重新从头构建。
- Playwright 已在桌面和 `390x844` 移动视口验证服务账号、身份安全两处来源 IP 状态；移动端 `document.scrollWidth=document.clientWidth=390`，摘要卡片无内部溢出。控制台 0 error/0 warning，授权后的总后台动态接口全部为 `200`。截图位于 `output/playwright/saas0066-client-ip/`。
- 最终源码与验收指纹为 `c872825e851e0344ab52822fde0634b76b2295c45b0260bb6c9533fe084f1dcc`，纳入 824 个文件；预览镜像为 `sha256:01604006f75aa37e7ddba33b83c5938ec5ccac61231c2520376772591d2546c5`，日志确认 `source=build` 且内置指纹一致。数据库未新增迁移，账本仍为 `65/0065_saas_service_account_key_pepper_ring`。
- 当前系统健康为 `26/28`：1 条逾期审批为 critical，统计窗口内 3 次失败后台执行为 warning。生产证据 doctor 与当前证据包仍为 `missing_count=6`、`not_ready_count=6`，没有启动 24 小时运行，因此仍不能标记生产最终完成。

## 本轮 SaaS 告警 Webhook 出站安全收口

- 审计确认租户通知策略可保存任意 Webhook URL，旧实现使用默认 HTTP Client，存在私网/回环/云元数据访问、环境代理劫持、跨源重定向和 DNS 重绑定风险。新增共享 `internal/outboundhttp` 出站防护，默认仅允许 HTTPS，拒绝 URL 凭据、片段、localhost、私网、回环、链路本地、保留网段和云元数据地址。
- 域名发送时会解析并校验全部 DNS 返回地址，再直接拨号已校验 IP，同时保留原 Host 与 TLS ServerName；因此 DNS 校验与连接之间不存在二次解析窗口。HTTP Transport 禁用环境代理，重定向最多 10 次且必须保持 scheme、host 和有效端口一致。
- 通过 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS` 与 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS` 支持部署策略和显式内网例外。CIDR 会规范化、去重、排序并拒绝 `/0` 或过宽网段；已知云元数据地址即使落入例外 CIDR 仍然硬阻断。
- 全局告警、租户数据库策略、通知 outbox、定时派发、员工申请 worker 和 `mochat-saas-maintenance` 共用同一防护实例。启动日志只记录策略与“端点已配置”，不再打印完整 Webhook URL，避免路径或查询参数中的令牌进入日志。
- 总后台通知策略 API 与页面新增“出站防护”状态，展示 HTTPS、私网/元数据、CIDR 例外、DNS 绑定、同源重定向和直连出站；每条已配置 URL 返回静态安全校验结果。保存时拒绝不安全字面地址，发送时再次执行 DNS 与拨号级校验，兼容旧数据库记录但不会向危险目标发起连接。
- `env -u GOROOT go test ./...`、`scripts/test.sh`、通知策略派发、通知 cron、员工申请 worker 和总后台专项 smoke 均通过。完整非 24 小时短证据包于 `2026-07-13 13:22:05 CST` 至 `14:11:24 CST` 重建，`14/14` 返回 0；smoke `95/95`、manifest 路由 `224/224`、运行时唯一路径 `582/582`、运行时路由条目 `721`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `05dfa71e256233ea67159b4bd0e68b57f312ef10a969cfcefa12cecd2d4d7380`，纳入 827 个文件；预览镜像为 `sha256:12e0fca3c7561045eff225eb3d4db2e6673a67cc087d3be95579254cadacd373`，日志确认 `source=build` 且指纹一致。真实预览 API 对 `https://127.0.0.1/internal` 返回 `400`，当前策略为仅 HTTPS、无 CIDR 例外。
- Playwright 已在 `1440x1000` 与 `390x844` 验收通知策略和出站防护状态；移动端 `documentScrollWidth=documentClientWidth=390`，状态区无内部溢出，授权后新页面控制台 0 error/0 warning。截图位于 `output/playwright/saas-webhook-egress/desktop-notification-policy.png` 和 `mobile-notification-policy.png`。
- 重建前数据库快照为 `/tmp/mochat-preview-before-webhook-egress-20260713.sql`，权限 `0600`、大小 833330 字节、SHA-256 为 `2ff4803243819e915d4572a7a85982d34508da7354767a16a96538c0721a5f65`。本轮无数据库迁移，MySQL、Redis、MinIO 和原数据卷均保留。
- 系统健康仍为 `26/28`：1 条逾期审批 critical、统计窗口内失败后台执行 warning；未把它误报为全绿。生产证据包已按最终指纹刷新，6 项外部证据仍全部缺失；`require_24h=false`、持续运行进程为空，因此目标继续保持未完成状态。

## 本轮 0066 SaaS 告警凭据加密收口

- 新增迁移 `0066_saas_alert_credential_encryption` 及完整 down rollback，为租户通知策略增加 `webhook_credentials_ciphertext` 和 `webhook_credentials_key_id`，Webhook URL 与签名 Secret 不再以明文作为主存储。迁移账本已在保留数据的预览库正常推进到 `66/66`。
- 新增 `internal/saasalertcredentials` AES-256-GCM 密钥环管理器，支持活动密钥、历史密钥兼容读取、独立密钥强制门禁、存量明文轮换和不可用密钥诊断。配置层与维护命令共用同一密钥域选择逻辑，专用告警密钥、身份密钥、合规密钥和备份密钥只会整域回退，不再混合单 Key、历史环和 Key ID。
- 主服务、通知策略存储、系统健康和 `mochat-saas-maintenance rotate-alert-credentials` 已接入同一管理器。总后台通知策略区域展示加密启用、强制加密、专用密钥、活动 Key、密文/旧明文/待轮换/不可用数量，并保留 RBAC 与操作审计。轮换按钮同时受权限、密钥可用性和待轮换数量约束；待轮换为 0 时保持禁用并提示“没有待轮换凭据”。
- 集成 smoke 已在真实 Go + MariaDB + Redis 上验证完整兼容链路：旧明文读取并轮换到 `q1`、缺失历史 Key 时安全失败、同时提供 `q1+q2` 后轮换到 `q2`、仅保留 `q2` 后继续正常读取。系统健康、合规生命周期和备份恢复脚本已同步新增通知凭据探针并修正检查总数。
- `scripts/test.sh` 已通过全部 Go 单测、`go vet`、五个命令构建、独立性和静态审计。最终短证据包从 `2026-07-13 17:12:42 CST` 至 `18:17:11 CST` 从头重建，`results.jsonl` 为 `14/14` 返回 0；smoke `95/95`、manifest 路由 `224/224`、runtime unique paths `583/583`、runtime route entries `723`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `f70391ac1e90436178b8e0517d39cfb66eeb2ed9009eccc501ab2db3ce88ed9b`，纳入 834 个文件。预览镜像为 `sha256:afdbfa0aaa3b52104a106c84a463eb7630d33935060d4896dbcaeff60f9a7f05`；启动日志确认 `source=build`、指纹一致、`require_encryption=true`、`dedicated_configured=true`、活动 Key 为 `preview-alert-v1` 且 `key_count=1`。
- 真实预览 API 返回 `encryptionConfigured=true`、`requireEncryption=true`、`dedicatedConfigured=true`、`healthy=true`、`rotationRequiredCount=0`；系统健康为 `27/29`，通知凭据加密探针为 healthy。整体仍保留 1 条逾期审批 critical 和 1 类失败后台执行 warning，不误报为全绿。
- 迁移前快照为 `/tmp/mochat-preview-before-0066-20260713.sql`，权限 `0600`、大小 843695 字节、SHA-256 为 `7a380edce326b0650ce8000933abc12b99c03f3281bce5d356f5a4dd5d831353`。MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 与 `390x844` 验收凭据保护状态和轮换按钮门禁，页面无横向溢出或元素重叠，控制台 0 error/0 warning。截图位于 `output/playwright/saas-alert-credential-encryption/desktop-1440x1000.png` 和 `mobile-390x844.png`。
- 生产证据 doctor、preflight 和 `docs/evidence/production/current` 已按最终指纹刷新：采集环境与有效标准文件均为 `0/6`，严格证据包按预期返回 1。缺口仍为 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；本轮没有启动任何 24 小时运行。
- 当前预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`；登录凭据仅保存在本地受控配置，不在文档记录。

## 本轮 0067 企业微信凭据加密与总后台治理收口

- 新增迁移 `0067_wecom_credential_encryption` 及完整 down rollback，为 `mc_corp` 和 `mc_work_agent` 增加 AES-256-GCM 密文与 Key ID 字段。企业 Secret、回调 Token/AES Key、会话存档 Secret 和企微应用 Secret 不再以明文作为主存储。
- 新增 `internal/wecomcredentials` 专用密钥环，支持活动密钥、历史密钥兼容读取、独立密钥强制、存量明文轮换、跨 Key 重加密和不可用 Key 诊断。企业与应用凭据的创建、更新、自动标签和会话存档同步均已接入统一加解密存储路径。
- 总后台新增 `GET /dashboard/saasAdmin/wecomCredentialProtection` 与 `POST/PUT /dashboard/saasAdmin/wecomCredentialRotation`，展示加密、强制、专用、活动 Key、密文、旧明文、待轮换和不可用数量；读取与轮换继续受平台 RBAC 和操作审计约束。页面在权限资料加载后非阻塞请求企微凭据状态，避免被后续集成请求串行阻塞。
- 真实 Go + MariaDB + Redis smoke 已验证 `legacy plaintext -> Q1 -> 缺历史 Key 安全失败 -> Q1+Q2 重加密 -> Q2-only 正常读取`，并覆盖只读角色可查看但不能轮换、新企业/应用写入即加密、维护命令轮换、系统健康 critical/healthy 切换和页面路由。
- `scripts/test.sh` 再次通过全部 Go 单测、`go vet`、五个命令构建、独立性与静态审计。完整非 24 小时短证据包于 `2026-07-14 09:41:20 CST` 至 `10:35:42 CST` 从头重建，`results.jsonl` 为 `14/14` 返回 0；smoke `96/96`、manifest 路由 `224/224`、runtime unique paths `585/585`、runtime route entries `726`、前端 dist API `209/209`。
- 独立交付与 Compose App smoke 均通过。源码指纹已排除私密 `.env`/`.env.*` 文件但保留 `.env.example`，最终指纹为 `dece0dcd6fedb810558c5577390db8f2b870be5114958bd4dbd3109bc211241b`，纳入 845 个文件；预览启动日志确认 `source=build` 且内置指纹一致。
- 预览数据库为 `67/0067`，企业凭据明文行 0、应用凭据明文行 0、企业密文行 1、应用密文行 0。企微凭据保护 API 返回专用强制加密开启、活动 Key `preview-wecom-v1`、healthy、待轮换 0、不可用 0；系统健康为 `28/30`，企微凭据探针 healthy，保留 `approval_sla_overdue` critical 和 `backup_freshness` warning，不误报为全绿。
- Playwright 已在 `1440x1000` 和 `390x844` 视口验证企微凭据保护状态、轮换按钮门禁和响应式布局；授权后控制台 0 error/0 warning，截图位于 `output/playwright/wecom-credential-desktop-1440x1000.png` 和 `output/playwright/wecom-credential-mobile-390x844.png`。
- 生产证据 doctor 与 `docs/evidence/production/current` 已刷新到最终指纹，`missing_count=6`、`not_ready_count=6`，严格证据与 skip-local 候选门禁按预期返回 1。当前仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据；没有启动 24 小时运行。

## 不能标记最终完成的原因

- 真实企业微信/微信开放平台账号联调尚未形成最终证据：授权、回调解密、通讯录、客户、客户群、标签、素材、公众号和企微应用链路仍需真实账号验证。
- MySQL 5.7 真实容器迁移 smoke 需要在 amd64 CI 或等价环境执行；本机 arm64 跳过不能作为生产兼容证据。
- SaaS 生产化仍需使用两个以上真实租户业务数据验证菜单权限、企业归属、资源额度、上传账本、异步任务、告警和运维处置。
- 生产域名前端仍需真实浏览器回归，目标环境仍需短时稳定性与外部监控记录；不再默认要求本机 24 小时 run。

## 建议下一步

1. 在 amd64 CI 执行 `env -u GOROOT ./scripts/ci_mysql57_amd64.sh`。
2. 准备真实企业微信和微信开放平台测试账号，补真实外部链路验收记录。
3. 用两个以上真实租户数据执行 SaaS 权限、额度、上传、异步任务和告警回归。
4. 汇总生产证据文件后执行 `MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 ./scripts/production_evidence_check.sh`。
5. 上线前再执行一次完整 `env -u GOROOT ./scripts/standalone_acceptance.sh`。

## 本轮 0068 微信开放平台凭据加密与总后台治理收口

- 新增迁移 `0068_wechat_open_credential_encryption` 及 down rollback，为第三方平台 `component_verify_ticket` 和公众号授权凭据增加 AES-256-GCM 密文与 Key ID 字段。新增 `internal/wechatopencredentials` 专用密钥环，支持活动/历史密钥、专用密钥强制、旧明文兼容读取、批量轮换、跨 Key 重加密和缺失历史 Key 时安全失败。
- Ticket 接收、公众号授权回跳、取消授权与消息回调、存储读取、主服务和维护动作 `rotate-wechat-open-credentials` 已接入同一凭据管理器。总后台新增 `GET /dashboard/saasAdmin/wechatOpenCredentialProtection` 与 `POST/PUT /dashboard/saasAdmin/wechatOpenCredentialRotation`，读取使用 `platform.integrations.read`，轮换使用 `platform.integrations.manage`，执行结果写入 `saas.admin.wechat_open_credential.rotate` 审计且不暴露明文或密文。
- 真实 Go + MariaDB + Redis smoke 覆盖旧明文 Ticket/公众号凭据、总后台轮换、授权回调写密文、缺历史 Key 安全失败、历史密钥环重加密和最终仅保留新 Key 后解密成功。实库 smoke 暴露的 `UNION` 字符集排序规则冲突已通过拆分为两条确定性有序查询修正。
- `scripts/test.sh`、schema migrate、release readiness、MySQL 5.7 静态兼容、独立交付包和完整 SaaS acceptance 均通过。最终短证据包于 `2026-07-14 12:27:37 CST` 至 `13:14:49 CST` 从头重建，`14/14` 返回 0；smoke `96/96`、manifest 路由 `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `0cf803a1e598446f8e4af86eee5b3a1755f428eda70e915df38ecd23d70d2c1c`，纳入 855 个文件。预览 App 已原位重建为镜像 `sha256:ad71ef9a03c4f7d89c3209bc67283a5b3540810b2c2eb012498714e7e756fe09`，日志确认 `source=build` 且内置指纹一致；MySQL、Redis、MinIO 与原数据卷均保留。
- 预览迁移账本为 `68/0068_wechat_open_credential_encryption`，运行时路由为 706 条。微信开放平台保护 API 返回强制加密、专用密钥、活动 Key `preview-wechat-open-v1`、`keyCount=1`、healthy；当前 Ticket、公众号、旧明文、待轮换和不可用数量均为 0。系统健康探针 `wechat_open_credential_protection` 为 healthy。
- 当前平台健康为 `29/31`，不是全绿：`approval_sla_overdue` 与 `backup_freshness` 均为 critical；前者是 1 条超 SLA 审批，后者是最近备份已超过新鲜度阈值。迁移前快照 `/tmp/mochat-preview-before-0068-20260714-112706.sql` 权限为 `0600`、大小 900374 字节、SHA-256 为 `4ca48646c8551b83462d7973902e93e2fd41bb1ae641dd61b791a93cf0d2dc3a`。
- Playwright 已在 `1440x1000` 与 `390x844` 验收微信开放平台凭据保护区域；页面无横向溢出，授权后控制台 0 error/0 warning。截图位于 `output/playwright/wechat-open-credential-encryption/desktop-1440x1000.png` 和 `output/playwright/wechat-open-credential-encryption/mobile-390x844.png`。
- 生产证据 doctor 与 `docs/evidence/production/current` 已刷新到最终指纹，严格生产证据与 skip-local 候选门禁只因 6 项真实外部证据缺失而按预期失败。`require_24h=false`，未启动 `standalone_soak_24h.sh`；目标继续保持“本地收口完成、待生产补证”。

## 本轮自动备份调度与平台健康收口

- 备份管理器已把自动调度开关、扫描间隔和启动检查状态暴露到 `backupOverview`和灾备中心。系统健康新增关键检查 `backup_automation`：备份策略启用但调度器未启用，或扫描间隔无效时，会直接报 critical。
- 保留数据卷的预览环境已启用 `MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON=1`、`MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL=5m` 和 run-on-start。启动日志确认 `cron-saas-backup` 持续运行；恢复 Docker 后每 5 分钟扫描均正确判定“backup not due”，未制造重复备份。
- 真实自动备份 run `8` 为 `bkp_20260714T053833Z_1cd7a563e3da`，状态 `succeeded`，使用加密 Key `preview-0051`，迁移账本 `68/0068`、数据表 `190` 张，工件 `154985` 字节；本地验证与 S3 副本均通过，SHA-256 和字节数一致。
- 通过真实总后台 API 撤回了超 SLA 的演示租户注销审批 `APR-20260711T063655-5B0AE24BA28B`，状态由 `pending` 转为 `canceled`、版本升为 `2`；审批事件 `4` 与操作审计 `115/saas.admin.approval.cancel` 已落库。
- 随后手动执行健康扫描 `HSC-20260714T054039-94314F17112A`，结果为 `healthy` 且 `32/32`，问题、critical、warning、新建/重开/恢复事故和通知数均为 0。`approval_sla_overdue`、`backup_automation` 和 `backup_freshness` 全部 healthy。
- `go test`/`go vet` 定向检查、三个健康/灾备脚本语法检查、备份恢复专项 smoke 和 `scripts/test.sh` 均通过。`docs/evidence/latest` 已收口为 14 条返回 0 的结果，包含 `core/saas/workers/cron/frontend/mysql57`；MySQL 5.7 完成 135 个 schema 静态检查，ARM 上的实容器按规则跳过且不充当 amd64 生产证据。
- 最终源码与验收指纹为 `357041312a9c86b48cc2e8830cb87f7a419ca275d1e3c925991d3fe6d5a608c0`，纳入 855 个文件；预览镜像为 `sha256:88ad810e68c14d07b43c3f0a626361256c3a80b22309fa87e664ae0f1b5d14f5`，App、MariaDB 和 Redis 当前均健康。
- Playwright 已在 `1440x1000` 和 `390x844` 验收“自动调度 5 分钟 / 启动检查就绪”。两种视口均无页面级横向溢出，按钮无重叠，控制台 0 error/0 warning；截图位于 `output/playwright/backup-automation/desktop-backup-operations.png` 和 `mobile-backup-operations.png`。
- 当前数据库快照为 `/tmp/mochat-preview-after-backup-automation-20260714-153824.sql`，权限 `0600`，大小 950112 字节，SHA-256 为 `c0d8667970ca410f37acf7d8e303e141cefe54503452281c57e0103e0ca28df2`。
- `docs/evidence/production/current` 已按最终指纹刷新，严格证据和 skip-local 候选门禁仅因 6 类真实外部证据缺失而失败。本轮未启动 24 小时运行，目标保持“本地 SaaS 收口完成、待生产环境补证”。

## 本轮发布证据远端工件真实性收口

- 修复发布证据可仅凭 URL、SHA-256 和字节数元数据标记 `passed` 的关键缺口。证据保存为 `passed` 前，服务端现在必须通过 HTTPS 流式下载真实工件，并精确比对实际 SHA-256 与字节数；失败返回 `422`，存储层同时拒绝绕过校验器直接写入通过状态。
- 创建发布候选时会并行重新下载六项必需工件。候选快照保存每项实际摘要、大小、HTTP 状态、内容类型、ETag、Last-Modified、校验时间和错误；事务提交前再次锁定并比较证据版本、URL、摘要和大小，防止校验期间证据被替换。只有当前权威源码指纹下六项远端复核全部通过才生成 `ready`。
- 出站校验器沿用 HTTPS、私网/云元数据阻断、DNS 绑定、禁用环境代理和同源重定向限制，默认超时 30 秒、单工件最大 64 MiB。内部证据库必须显式配置最小 CIDR 例外，自有 CA 必须通过只读证书文件配置；当前预览无 CIDR 例外、无自有 CA。
- 总后台发布准备中心已区分 `metadataReady` 与最终 `ready`，展示远端复核数量、最近候选状态和校验器策略。当前预览六项外部证据均未录入、发布候选为 0，因此 `metadataReady=false`、`ready=false`，不会把本地验收结果冒充生产证据。
- `scripts/smoke_saas_release_readiness.sh` 使用临时私有 CA HTTPS 工件库和真实 Go + MariaDB + Redis 验证六项真实工件：保存时远端校验、内容被篡改后候选 `5/6 blocked`、恢复后候选 `6/6 ready`、不可变快照、操作审计及迁移回滚/重放均通过。`go test ./...`、`scripts/test.sh`、前端 API `209/209` 和六个短验收套件均通过。
- 最终短证据包于 `2026-07-14 16:47:47 CST` 至 `17:24:20 CST` 重建，`14/14` 返回 0；smoke `96/96`、manifest 路由 `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。最终指纹为 `fd9e06ae9e907309a9a44ec26cae9f4de719bd984e12214017a76d73466f1285`，纳入 857 个文件。
- 预览 App 镜像为 `sha256:42a3fbde4305b42003edbed9a3565140d0f7201fa639895abad359e86e120bb6`，迁移仍为 `68/0068`、数据表 190 张。重建前快照 `/tmp/mochat-preview-before-release-artifact-verifier-20260714-162214.sql` 权限为 `0600`、大小 952925 字节、SHA-256 为 `d1b304454ec7d1e50ac803a9292ad9e2e852637c1d3ef07355f824c49431b275`。
- Playwright 已完成 `1440x1000` 和 `390x844` 发布准备中心回归，移动端文档宽度为 `390/390`、弹窗无溢出，干净会话控制台 0 error/0 warning。截图位于 `output/playwright/release-artifact-verifier/`。
- 最新健康扫描为 32 项、0 critical、1 warning；warning 来自 24 小时窗口内两次已恢复的 MinIO DNS 失败，当前审计锚点已重新创建并验证为 `26/26`，远端异常计数均为 0。该历史记录不影响本轮代码门禁通过，但必须作为预览环境运行事实保留。
- 本轮未启动 24 小时运行。仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类真实外部证据，因此不能标记生产最终完成。

## 本轮发布候选证据状态漂移失效收口

- 审计发现一个发布门禁时序缺口：候选曾以六项证据通过并写入 `ready` 后，如果现行证据被改为失败、缺失、替换或随后恢复，旧候选仍可能被摘要接口当成可发布候选。该行为会让历史快照代替现行证据，不能满足严格生产发布语义。
- 发布候选现增加 `stale` 有效状态。数据库中的原始 `ready` 状态继续保留用于审计，但摘要和候选列表会逐项比较候选快照与现行证据的 ID、key、状态、证据 URL、执行环境、源码指纹、工件 SHA-256、工件字节数、复核时间、复核人、备注和版本，并校验快照内远端工件复核结果。任一项漂移都会返回 `effectiveStatus=stale`、`evidenceCurrent=false`、具体 `driftedEvidenceKeys`，总摘要强制 `ready=false`。
- 证据恢复为 `passed` 不会让旧候选自动复活，必须重新运行发布门禁生成新候选。专项 smoke 已验证“候选 ready -> stability 证据失败 -> 旧候选 stale -> 证据恢复后仍 stale -> 新候选重新 ready”，并确认历史候选继续可审计。
- 总后台发布准备中心新增“证据已变化”和“证据变化，需重跑门禁”状态，候选表展示变化项；当前预览未录入六项外部证据，API 返回 `metadataReady=false`、`ready=false`、`missingCount=6`、远端校验器已启用，未把本地短验收冒充生产证据。
- `go test ./internal/dashboard ./internal/store`、`scripts/smoke_saas_release_readiness.sh`、`scripts/test.sh` 和前端 dist API 审计均通过。`docs/evidence/latest` 于 `2026-07-15 09:25:40 CST` 至 `10:12:25 CST` 重新完整采集，14 条结果全部 `returncode=0`；六个验收套件、manifest 路由 `224/224`、前端 dist API `209/209` 均通过。
- 最终源码与验收指纹为 `026b3cef88d869a0e53ac6ce6c0c493d69d2102b981106808ec520ee1a084102`，纳入 857 个文件。预览 App 镜像为 `sha256:f4936e44f96039fca314873a84973066e5d0a8e3f7bfa7922f056f80faab3733`；启动日志确认 `source=build`、内置指纹一致、发布证据保存时校验和候选时重验均启用。`/readyz` 返回 `200`，运行时路由 706 条，PHP fallback 关闭。
- Playwright 已在 `1440x1000` 和 `390x844` 验收发布准备中心。桌面和移动端均无页面级横向溢出，移动宽表仅在 368px 内部容器滚动，控制台为 0 error/0 warning；截图位于 `output/playwright/release-candidate-drift/release-readiness-desktop.png` 和 `release-readiness-mobile.png`。
- 预览数据库快照保存在 `output/backups/mochat-preview-before-release-candidate-drift-20260715-0922.sql`，权限 `0600`、大小 989138 字节、SHA-256 为 `a3f6973c877d6d3fc7dd8712a415f990412e4dc527f07f480cc191985bc4ca66`。MySQL、Redis、MinIO 和原数据卷均保留。
- 生产证据 doctor、顶层报告和 `docs/evidence/production/current` 已刷新到最终指纹；严格生产候选包因六项真实外部证据全部缺失而按预期返回 1。没有启动任何 24 小时运行，目标继续保持“本地 SaaS 与总后台收口、待真实生产环境补证”。

## 本轮 0069 发布候选双人审批与预览收口

- 新增迁移 `0069_saas_release_candidate_approval`，把 `release.candidate.gate` 纳入高风险审批中心。默认风险级别为 critical，要求 `platform.release.manage` 权限、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 6 小时失效。
- 开启强制审批后，直接调用 `POST /dashboard/saasAdmin/releaseCandidate` 返回 `428 Precondition Required`。审批申请会固化发布版本、当前权威源码指纹和六项证据元数据；执行时重新下载并核对六项远端工件，再创建不可变发布候选。
- 候选事务会原子写入审批副作用操作 ID。业务提交后若审批完成回写中断，租约恢复只补审批状态，不重复生成候选；单元测试和真实 MariaDB/Redis/私有 CA HTTPS 工件库 smoke 已覆盖该恢复语义。
- 总后台发布准备中心会在证据元数据完整且指纹一致后提交审批，而不是直接生成候选。当前预览六项证据均为 `missing`、候选为 0，因此 `metadataReady=false`、`candidateGateEnabled=false`、按钮禁用，未把本地回归冒充生产证据。
- 完整短证据包于 `2026-07-15 11:50:21 CST` 收口，14 条命令全部返回 0；`core/saas/workers/cron/frontend/mysql57` 六套验收、manifest 路由 `224/224`、运行时唯一路径 `587/587`、运行时路由条目 `729`、前端 dist API `209/209` 均通过。
- 最终源码指纹为 `f1eb06656144ab104dd58853b9d171c988eda161d40d18c0ed347aefc36f0385`，纳入 859 个文件；预览镜像为 `sha256:18b757460cdba0ae2076a19c8847b2210ad5f1d7a22e2723d8d57dd1cd8a427c`，`/readyz` 返回 200，迁移账本为 `69/0069`。
- 升级前数据库快照为 `output/backups/mochat-preview-before-0069-release-candidate-approval-20260715-104108.sql`，权限 `0600`、大小 997621 字节、SHA-256 为 `6024720157d26bdbffe660f5222cbcaeea769333660572e8ad2399b64ee3683e`。
- Playwright 已在 `1440x1000` 与 `390x844` 复核最终镜像；移动端文档宽度为 `390/390`，证据宽表仅在 368px 内部容器滚动，干净会话控制台 0 error/0 warning。截图为 `output/playwright/release-center-final-desktop.png` 和 `release-center-final-mobile.png`。
- `docs/evidence/production/current` 已按最终指纹刷新，严格文件检查和 skip-local 生产候选门禁只因六项外部证据缺失而返回 1。当前仍不能标记生产最终完成，本轮没有启动任何 24 小时运行。

## 本轮 0070 审批策略变更双人会签与原子生效收口

- 新增迁移 `0070_saas_approval_policy_change_guard` 及 down rollback，把 `approval.policy.update` 纳入统一高风险审批。治理策略强制启用，不允许关闭或把自身降级，默认要求 2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。
- 总后台直接调用审批策略写接口现在返回 `428 Precondition Required`。申请会固化完整策略载荷；双人会签后执行时，策略更新、审批副作用时间和平台操作日志在同一数据库事务提交，恢复租约不会重复写入业务效果。
- 页面将治理行固定显示为“强制启用”，启用开关只读，最低审批人数为 2，保存动作统一改为“提交审批”。API 与真实数据库 smoke 验证策略总数为 8，治理策略值为 `enabled=1`、`required_approvals=2`、`sla=120`、`reminder=30`、`expiry=12`。
- 迁移、回滚重放、MySQL 5.7 静态兼容、release readiness、审批专项、全量 Go 测试和独立交付门禁均通过。完整短证据包于 `2026-07-15 13:58:08 CST` 收口，`docs/evidence/latest/results.jsonl` 为 `14/14` 返回 0；六套验收、manifest `224/224` 和前端 dist API `209/209` 全部通过。
- 最终源码指纹为 `f5ff1082489ff19f917bfa487b333031cfbf6f0783f674775ae915f1b46c3ce5`，纳入 861 个文件。预览镜像为 `sha256:0708d78ee0f92a9d7754d0c2d030e9b8613d3141bb9c87a4f9cfe8dd4b910280`，日志确认 `source=build` 且指纹一致；数据库为 `70/0070`，策略为 8 条。
- 升级前快照为 `output/backups/mochat-preview-before-0070-approval-policy-guard-20260715-122339.sql`，权限 `0600`、大小 1016263 字节、SHA-256 `6f3f45bfabb096452cb2a35e0ad9c3a31ebf93cd7f4fdabd10a24d2eb306d8e7`。
- Playwright 已在 `1440x1000` 与 `390x844` 验收治理行。桌面无页面级横向溢出；移动端文档宽度为 `390/390`，策略表只在 368px 内部容器滚动，右侧提交按钮完整可见；控制台 0 error/0 warning。最终截图位于 `output/playwright/0070-approval-policy-*-final.png`。
- 当前仍缺 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类生产证据。`require_24h=false`，本轮未启动 24 小时运行，目标继续保持“本地收口、待生产补证”。

## 本轮 0071 备份策略变更双人会签与原子生效收口

- 新增迁移 `0071_saas_backup_policy_change_guard` 及完整 down rollback，把 `backup.policy.update` 纳入统一高风险审批。默认风险为 critical，要求 `platform.backups.manage` 权限、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。
- 强制审批开启后，直接调用备份策略写接口返回 `428 Precondition Required`，且数据库不发生变更。审批申请会规范化并固化完整策略载荷；执行阶段再次校验配置，避免无效或被篡改的载荷进入生效事务。
- MySQL 存储层把策略更新、审批副作用操作 ID 和平台操作日志放在同一事务提交。真实 Go + MariaDB + Redis 治理 smoke 已覆盖独立申请人、两名不同审批人、第三方执行人、直接写入阻断、业务效果和操作审计，租约恢复不会重复应用策略。
- 灾备中心的策略保存按钮已改为“提交审批”，页面加载后按服务端权威策略渲染。审批中心新增“变更备份策略 / `backup.policy.update`”筛选项和第 9 条审批策略。
- `go test ./...`、71 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、MySQL 5.7 静态兼容、功能模块矩阵和审批治理专项均通过。完整短证据包于 `2026-07-15 14:48:31 CST` 至 `15:27:32 CST` 从头重建，14 条命令全部返回 0；六套验收、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209` 均通过。
- 最终源码与验收指纹为 `db9b881455a40dabe44470f036cdd33e4b6200ba83dc311fed58a511c9221b8c`，纳入 863 个文件。保留数据卷的预览已升级到镜像 `sha256:f4424e4529b54002b52355d78428d24342bf57b581d75663f6237ec24d9a47f6`，日志确认 `source=build` 且内置指纹一致；`/readyz` 返回 200，迁移账本为 `71/0071_saas_backup_policy_change_guard`，审批策略为 9 条。
- 升级前快照为 `output/backups/mochat-preview-before-0071-backup-policy-guard-20260715-143642.sql`，权限 `0600`、大小 1049933 字节、SHA-256 `2e37885c98839fa22da0f705a190f9839d43f2504ed6e4e2631be7b8763f4dca`。MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 与 `390x844` 验收最终预览。两种视口均无页面级横向溢出，移动端按钮完整可见，宽表只在内部 `tablewrap` 滚动；授权后控制台 0 error/0 warning。截图为 `output/playwright/0071-backup-policy-desktop-final.png` 和 `output/playwright/0071-backup-policy-mobile-390-final.png`。
- `docs/evidence/production/current` 和严格生产证据 doctor 已按最终指纹刷新，仍为 `missing_count=6`、`not_ready_count=6`。MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端、目标环境短稳/外部监控六项外部证据缺失，严格门禁按预期返回 1；本轮没有启动任何 24 小时运行。
- 备份清理尚未接入该审批里程碑。它同时涉及本地文件、数据库和 S3/Object Lock 的幂等副作用，需要单独设计可恢复执行协议后再纳入高风险审批，不能复用单纯数据库事务假装原子完成。

## 本轮 0072 备份保留清理审批 Saga 与跨资源恢复收口

- 新增迁移 `0072_saas_backup_cleanup_saga` 及完整 down rollback，为备份记录增加 `cleanup_run_id`，新增清理任务与步骤表，并加入第 10 条高风险策略 `backup.retention.cleanup`。默认要求 `platform.backups.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。
- 服务端在申请审批时按当前策略计算并冻结候选范围，始终保留最新成功备份和策略要求的最小成功份数；审批通过后不重新扩大范围。冻结中的备份禁止校验、复制、恢复和直接删除，避免审批对象与实际副作用漂移。
- 执行器按“异地副本 -> 本地工件 -> 数据库台账”顺序推进，每一步持久化状态、尝试次数和错误。远端删除失败时保留本地文件与数据库记录；进程中断、租约过期或部分失败后只重试未完成步骤，不重复删除已完成资源。直接清理在强制审批开启时返回 `428`，cron 和维护命令只恢复已获批任务，不自行创建删除范围。
- 灾备中心新增保留清理任务表、冻结范围、执行进度、失败详情和重试操作；备份概览新增清理统计。系统健康新增关键探针 `backup_retention_cleanup`，当前预览为 `healthy`、运行中 0、异常 0。
- 真实 Go + MariaDB + Redis + MinIO smoke 已验证：加密备份与异地副本、篡改修复、隔离恢复、直接清理 `428`、双人审批、冻结 1 个候选、Saga 成功删除远端/本地/数据库记录，以及远端失败时保留本地与台账后可恢复重试。单元测试另覆盖最新备份保留、无候选、重复执行和租约恢复。
- `go test ./...`、72 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、审批治理、发布准备、功能矩阵、独立包和独立 Compose App smoke 均通过。完整短证据包于 `2026-07-15 16:55:13 CST` 至 `17:34:32 CST` 从头重建，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 143 个文件，本机 ARM 的实容器明确跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `5db23ab6d2310791eb375baba9b3040df9c7b0089e572560e134cfb54de54146`，纳入 867 个文件。保留数据卷的预览已升级到镜像 `sha256:592bc88d5292c384581640a8d6459c534e8ca5ddfd79cc0d9c4214573b8dcb89`；`/readyz` 返回 200，日志确认 `source=build` 且指纹一致，迁移账本为 `72/0072_saas_backup_cleanup_saga`，审批策略为 10 条。
- 升级前快照为 `output/backups/mochat-preview-before-0072-backup-cleanup-saga-20260715-162328.sql`，权限 `0600`、大小 1062706 字节、SHA-256 `54eca644f01eb5474d11e593cbbb531c237af11f03c8ac1eac03edf5e3c2965c`。MySQL、Redis、MinIO 和原数据卷均保留。
- 预览真实 `.env.local` 已恢复自动备份、专用凭据密钥、历史备份 Key、服务账号 pepper 和审计锚点历史 HMAC Key。平台健康为 `32/33`：当前配置与 0072 清理探针均健康，最新审计锚点任务验证 `42/42`；唯一 critical 是 24 小时窗口内 7 次真实失败执行，未删除或改写历史记录。
- Playwright 已在 `1440x1000` 与 `390x844` 验收最终镜像。页面宽度分别为 `1440/1440`、`390/390`，移动宽表只在 368px 内部容器滚动，清理按钮显示“提交清理审批”，控制台 0 error/0 warning。截图位于 `output/playwright/0072-backup-cleanup-*-final.png`。
- `docs/evidence/production/current`、doctor、preflight 和 skip-local 候选门禁已按最终指纹刷新，严格检查均因六项外部证据缺失按预期返回 1；源码与 latest 证据指纹匹配通过。仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控。本轮未启动任何 24 小时运行，目标继续保持“本地收口、待生产补证”。

## 本轮 0073 合规导出提前删除审批 Saga 与跨资源恢复收口

- 新增迁移 `0073_saas_compliance_export_deletion_saga` 及完整 down rollback，为合规导出增加删除审批、冻结快照、执行租约、工件步骤、数据库步骤、尝试次数和最后错误等持久化状态，并加入第 11 条高风险策略 `compliance.export.delete`。该策略强制启用，至少需要 2 名不同复核人，SLA 120 分钟、提醒 30 分钟、12 小时失效，审批治理接口不能关闭或降级。
- 申请阶段由服务端冻结租户、导出编号、工件路径/名称、SHA-256、字节数和保留期；执行与重试阶段重新校验法律保留、未终结的数据擦除引用和冻结快照。保留期内直接删除返回 `428 Precondition Required`，法律保留或擦除引用存在时返回 `409`。
- 删除执行按“本地加密工件 -> 数据库记录”两步持久化推进。工件失败会保留数据库记录、错误和尝试次数；人工 `delete-retry` 只继续未完成步骤，维护任务会恢复待执行或租约过期的已批准任务。系统健康的 `compliance_export_queue` 会识别失败、积压和过期租约，避免跨文件系统与数据库删除被误当成单事务。
- 合规中心已显示待审批擦除、删除状态、工件/记录步骤、审批 ID、尝试次数、错误和“提交删除审批/继续删除/重试删除”操作。审批中心新增“删除合规导出工件 / `compliance.export.delete`”，页面使用服务端权威策略，不提供绕过入口。
- 全量 Go 测试、73 版迁移 apply/rollback/replay、合规生命周期、审批治理、审批主流程、系统健康、独立包、独立 Compose、功能矩阵和六个本地短验收套件均通过。最终 `docs/evidence/latest/results.jsonl` 于 `2026-07-15 20:51:26 CST` 至 `22:02:43 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 145 个文件，本机 ARM 的实容器按规则跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `cf3b0366e8309d3a2f1efad3a6f6f7325b0e3d75a4e9e6cf2f93c4b9660e6bf7`，纳入 871 个文件。保留数据卷的预览已升级到镜像 `sha256:7047142568b9d899522dbe26c835025a64dff52ced81daf83cb3124366f2138b`，启动日志确认 `source=build` 且内置指纹一致；`/readyz` 返回 200，运行时路由 706 条，迁移账本为 `73/0073_saas_compliance_export_deletion_saga`，审批策略 11 条。
- 升级前快照为 `output/backups/mochat-preview-before-0073-compliance-export-deletion-saga-20260715-220412.sql`，权限 `0600`、大小 1117833 字节、SHA-256 `6b85c75c55d57fa41271fbfac014a8a867f19d27fff8456b2b4a2ee9f01ba6df`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 和 `390x844` 验收审批策略与合规中心。桌面和移动端均无页面级横向溢出，移动宽表只在 368px 内部容器滚动，控制台 0 error/0 warning，授权后总后台动态请求全部返回 200。截图为 `output/playwright/0073-compliance-export-delete-policy-desktop.png`、`0073-compliance-center-desktop.png`、`0073-compliance-export-delete-mobile-390.png` 和 `0073-compliance-center-mobile-390.png`。
- 当前平台健康为 `32/33`，新增三个合规探针均为 healthy；唯一 critical 是统计窗口内保留的 7 次历史后台失败执行。最新审计锚点已远端导出并验证 `43/43`，发布准备中心仍为 `0/6`、`ready=false`，权威指纹来源为 build。
- 生产证据 doctor 与 `docs/evidence/production/current` 已于 `2026-07-15 22:20:00 CST` 按最终指纹刷新。严格生产证据、严格目标审计和 skip-local 候选门禁只因六项真实外部证据缺失而按预期失败，源码与 latest 指纹匹配通过。本轮未启动任何 24 小时运行，目标继续保持“本地 0073 收口、待生产补证”。

## 本轮 0074 法律保留解除双人审批与原子执行收口

- 新增迁移 `0074_saas_compliance_legal_hold_release_guard` 和第 12 条高风险策略 `compliance.legal_hold.release`。策略强制启用，要求 `platform.compliance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；审批治理不能关闭该策略或把审批人数降到 2 以下。
- 申请阶段由服务端冻结保留单号、租户、状态、原因、起止时间、版本和解除原因。直接解除在强制审批开启时返回 `428 Precondition Required`；执行阶段锁定当前保留记录并重新比对冻结快照，快照或版本漂移会拒绝执行。
- 法律保留状态更新、平台操作日志和审批效果操作 ID 在同一 MySQL 事务内提交。审批租约恢复只补齐审批完成状态，不会重复解除保留；总后台活动保留单的操作已改为“提交解除审批”。
- 全量 Go 测试、74 版迁移 apply/rollback/replay、合规生命周期、审批主流程、审批治理、发布准备、系统健康、独立包、独立 Compose 和六套本地短验收均通过。最终证据包于 `2026-07-15 23:04:25 CST` 至 `23:50:18 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 147 个文件，本机 ARM 实容器按规则跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `293e0c1a28b67a0a488a575b16abbacf22c8eee6758c038312ae71b12ccd70d4`，纳入 874 个文件。保留数据卷的预览已原位升级到镜像 `sha256:9556f703a22bb0f5e6d11580f5289b6a76eac4cbcee3ab14e1bdf75fa00329ed`；启动日志确认 `source=build` 且内置指纹一致，`/readyz` 返回 200、运行时路由 706 条，迁移账本为 `74/0074_saas_compliance_legal_hold_release_guard`，审批策略为 12 条。
- 升级前快照为 `output/backups/mochat-preview-before-0074-legal-hold-release-20260715-230139.sql`，权限 `0600`、大小 1131065 字节、SHA-256 `0e5b94961f2f5ac9df2095f32feeb96d4751c6a66c375bea4302cb9b314966c5`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 和 `390x844` 验收总后台、法律保留区和审批策略。两种视口均无页面级横向溢出，移动宽表只在内部滚动；干净重载无 4xx/5xx 和 page error，控制台 0 error/0 warning。策略行实测为强制启用、不可关闭、审批人数 2，活动保留单显示“提交解除审批”。截图位于 `output/playwright/saas-0074-*.png`。
- 当前平台健康为 `32/33`。唯一 critical 来自统计窗口内保留的 7 次历史审计锚点失败；当前同一任务已连续 3 次成功，没有删除或改写失败历史。发布准备仍为 `0/6`、`ready=false`，权威源码指纹来源为 build。
- 生产证据 doctor 和目标完成度审计已按最终指纹刷新，仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六项外部证据。本轮没有启动 24 小时运行，目标继续保持“本地 0074 收口、待生产补证”。

## 本轮 0075 合规生命周期策略变更双人会签与原子执行收口

- 新增迁移 `0075_saas_compliance_policy_change_guard` 和第 13 条高风险策略 `compliance.policy.update`。策略强制启用，要求 `platform.compliance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；审批治理不能关闭该策略或把审批人数降到 2 以下。
- 合规策略写接口会先执行完整业务校验，再由统一高风险门禁返回 `428 Precondition Required`，且不修改策略。审批申请会规范化并冻结全部保留期配置、状态、最近导出要求和当前版本；执行前重新规划当前版本，旧版本申请会以冲突拒绝，避免覆盖后续策略变更。
- 策略更新、平台操作日志和审批效果操作 ID 在同一 MySQL 事务内提交。审批执行租约恢复只补齐审批状态，不会重复写入合规策略；总后台合规策略按钮已改为“提交审批”，审批中心新增“变更合规生命周期策略 / `compliance.policy.update`”。
- 全量 Go 测试、75 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、合规生命周期、审批主流程、审批治理、发布准备、系统健康、独立包、独立 Compose、功能矩阵和六套本地短验收均通过。最终证据包于 `2026-07-16 00:30:51 CST` 至 `01:09:00 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 149 个文件，本机 ARM 实容器按规则跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `c08216de5937ffc23d95f4092a76d6ed99f35fdb7f90bd9f7761f2cf378ee229`，纳入 877 个文件。保留数据卷的预览已原位升级到镜像 `sha256:f022f983c899f4197d30019326cb55f114676221fa2a06cb6880b7c331e4d81e`；启动日志确认 `source=build` 且内置指纹一致，`/readyz` 返回 200、运行时路由 706 条，迁移账本为 `75/0075_saas_compliance_policy_change_guard`，审批策略为 13 条。
- 升级前快照为 `output/backups/mochat-preview-before-0075-compliance-policy-guard-20260716-011013.sql`，权限 `0600`、大小 1145672 字节、SHA-256 `c646b6400b2ee8dfde369d2f733af3abf4c37c6687e5f6f05bd71ef1213f74eb`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 和 `390x844` 验收合规策略表单与审批策略行。两种视口均无页面级横向溢出；移动审批表只在 368px 的 `.tablewrap` 内滚动。策略行实测为强制启用且值为 `2/120/30/12`，合规表单显示“提交审批”；控制台 0 error/0 warning，未发现失败请求。截图为 `output/playwright/saas-0075-compliance-policy-desktop.png`、`saas-0075-compliance-form-desktop.png` 和 `saas-0075-compliance-policy-mobile.png`。
- 当前平台健康为 `32/33`。唯一 critical 仍是统计窗口内保留的 7 次历史后台失败；审计锚点任务当前最新 4 次均成功，45 个锚点全部完成远端导出与验证，历史失败没有被删除或改写。
- 生产证据 doctor、preflight 和 `docs/evidence/production/current` 已按最终指纹刷新，仍为 `missing_count=6`、`not_ready_count=6`。严格证据检查、严格目标审计和 skip-local 候选门禁只因六项真实外部证据缺失而按预期返回 1，源码与 latest 指纹匹配；`require_24h=false`，本轮没有启动 24 小时运行，目标继续保持“本地 0075 收口、待生产补证”。

## 本轮 0076 身份安全策略变更双人审批与原子生效收口

- 新增迁移 `0076_saas_identity_policy_change_guard` 和第 14 条高风险策略 `identity.policy.update`。该策略强制启用，要求 `platform.identity.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；审批治理不能关闭或降低双人审批门槛。
- 身份安全策略直接写入现在返回 `428 Precondition Required`。申请阶段规范化并冻结完整策略载荷和当前版本，执行阶段重新校验版本；策略变更、审批效果操作 ID 和平台操作日志在同一 MySQL 事务提交，租约恢复不会重复应用业务副作用。
- 总后台身份安全中心的保存动作已改为“提交审批”，审批中心显示“变更身份安全策略”。专项单元测试覆盖直接阻断、冻结载荷、陈旧版本冲突、双人审批执行和恢复幂等；真实 Go + MariaDB + Redis smoke 覆盖独立申请人、两名复核人和业务审计闭环。
- 全量 Go 测试、76 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、审批治理、身份安全、独立包、独立 Compose 和六套本地短验收均通过。最终 `docs/evidence/latest/results.jsonl` 于 `2026-07-16 01:54:25 CST` 至 `02:31:27 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 151 个文件，本机 ARM 实容器按规则跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `147d3b51f5937ce18760102b2f30635c8aebcf13e327745189ffe0338dda27eb`，纳入 880 个文件。保留数据卷的预览已升级到镜像 `sha256:0a478816f72cf1713b35a14b6b2d4330c84bf96e354cfd70d7d8ad5ce5b29585`；`/readyz` 返回 200，运行时路由 706 条，迁移账本为 `76/0076_saas_identity_policy_change_guard`，审批策略为 14 条。
- 升级前快照为 `output/backups/mochat-preview-before-0076-identity-policy-guard-20260716-023234.sql`，权限 `0600`、大小 1157151 字节、SHA-256 `aaa05a07fb77268fc72aeb6aa61f345acafe1c5fca06de172cfd14a944d7ec9b`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 与 `390x844` 验收身份策略表单和审批策略。两种视口均无页面级横向溢出；移动端 85 个宽表均只在内部可滚动容器溢出，审批表可横向滚动到最右侧；干净授权会话中 69 个请求全部返回 200，控制台 0 error/0 warning。截图位于 `output/playwright/saas-0076-identity-policy-desktop.png`、`saas-0076-approval-policy-desktop.png`、`saas-0076-identity-policy-mobile.png`、`saas-0076-approval-policy-mobile.png` 和 `saas-0076-approval-policy-mobile-right.png`。
- 当前平台健康为 `32/33`，唯一 critical 是统计窗口保留的 7 次历史后台失败；当前审计锚点任务最新 5 次连续成功，46 个锚点均已远端导出并验证。生产 evidence doctor、preflight 和 current 候选包已按最终指纹刷新，仍为 `0/6`，严格门禁只因六项真实外部证据缺失而阻断；源码指纹匹配，`require_24h=false`，本轮未启动 24 小时运行。

## 本轮 0077 租户停用强制双人审批收口

- 审计确认 `tenant.disable` 虽已进入高风险审批，但历史默认仍只要求 1 名复核人，审批治理还可关闭或降低该策略。`0077_saas_tenant_disable_approval_guard` 现将该动作固定为启用、金额阈值为 0、至少 2 名不同复核人；治理接口不能关闭或降到 2 人以下。
- 租户停用直接写入继续返回 `428 Precondition Required`。审批申请固定租户、目标状态和备注，要求 `platform.tenants.manage`；第一名复核人通过后保持 pending，第二名复核人通过后才可执行。真实 Go + MariaDB + Redis smoke 已验证停用后会阻断该租户登录，并覆盖拒绝、取消和恢复执行语义。
- 全量 Go 测试、77 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、审批主流程、审批治理、独立包、独立 Compose、功能矩阵和六套短验收均通过。最终 `docs/evidence/latest` 于 `2026-07-16 03:13:55 CST` 至 `03:51:22 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 153 个文件，本机 ARM 实容器明确跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `096bf16c8dbf24dc9a95d69ae35c46af81c559c67f48866746ca97ca943c090b`，纳入 883 个文件。保留数据卷的预览已升级到镜像 `sha256:250747044e82e7d3d2c96a76a176d8b2f2656c559ec9dda28ec09fba0e8f8480`；启动日志确认 `source=build` 且内置指纹一致，`/readyz` 返回 200、运行时路由 706 条，迁移账本为 `77/0077_saas_tenant_disable_approval_guard`，审批策略仍为 14 条，`tenant.disable` 为 `enabled=1`、`2/240/60/24`、`v2`。
- 升级前快照为 `output/backups/mochat-preview-before-0077-tenant-disable-approval-guard-20260716-035225.sql`，权限 `0600`、大小 1170212 字节、SHA-256 `64206ea8929cf196161a7d04d4d31cdbeac34bd20e7d032bcf5ccc3f8c1d8e70`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 与 `390x844` 验收审批策略。桌面和移动端均无页面级横向溢出；移动表格只在 368px 的 `.tablewrap` 内滚动，可完整滚到右侧查看 SLA 与提交按钮。策略启用框只读、审批人数下限为 2；干净授权会话控制台 0 error/0 warning，业务请求全部返回 200。截图位于 `output/playwright/saas-0077-tenant-disable-policy-desktop.png`、`saas-0077-tenant-disable-policy-row.png`、`saas-0077-tenant-disable-policy-mobile.png` 和 `saas-0077-tenant-disable-policy-mobile-right.png`。
- 当前平台健康为 `32/33`，唯一 critical 是过去 24 小时窗口内保留的 7 次历史后台失败；47 个审计锚点均已导出并验证。生产 doctor、preflight 和 `docs/evidence/production/current` 已按最终指纹刷新，严格候选包因 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端、目标环境短稳/外部监控六项证据缺失而返回 1。`require_24h=false`，本轮未启动 24 小时运行，目标继续保持“本地 0077 收口、待生产补证”。

## 本轮 0078 严重风险审批策略基线防降级收口

- 审计发现退款策略的新建默认仍只需 1 人，且严重风险策略的强制开启、零绕过阈值和双人会签下限未统一成平台级不变式。现已将 13 条 critical 策略固定为 `enabled=true`、`amountThresholdCents=0`、`requiredApprovals>=2`；普通的 `payment.settlement.close` 仍可配置，会签下限保持 1。
- 新增迁移 `0078_saas_critical_approval_policy_guard`。服务端对默认、存储读取、写入校验和 API 输出均做防御性夹紧；页面根据 `governanceLocked`、`minimumApprovals` 和 `amountThresholdLocked` 渲染，不再依赖前端硬编码动作列表。down 只恢复全新默认退款策略的历史值，不会把已修复的生产数据重新降级。
- 迁移 apply/rollback/replay、审批主流程和治理专项 smoke 均通过。退款第一票后仍为 pending，第二票后才可执行；任意小额退款直接请求仍返回 `428`。对退款、租户停用、人员授权和发布门禁等代表性 critical 策略的降级请求均返回 `400`，普通结算策略修改可正常进入审批。
- `docs/evidence/latest` 于 `2026-07-16 04:28:05 CST` 至 `05:07:11 CST` 从头重建，14 条命令全部返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 155 个文件，ARM 上跳过的 amd64 实容器不计入生产证据。最终指纹为 `47d64ab13b5e31c69b5b4b3268e8389a106258615bd709cbc05356d912140fad`，纳入 886 个文件。
- 保留数据卷的预览已升级到镜像 `sha256:c3d4f11393e5e079225c56241cf7d87cc0d54b5888fa85f58b6719f0e342f073`和迁移 `78/0078_saas_critical_approval_policy_guard`，`/readyz` 返回 200，镜像内置指纹与 latest 一致。升级前 `0600` 快照为 `output/backups/mochat-preview-before-0078-critical-approval-policy-guard-20260716-050823.sql`，大小 1180572 字节，SHA-256 `1ffd5a5197fa348f8462834fd19c11075ccf2fa4822a67cba7a133f11871977a`。
- Playwright 在 `1440x1000` 和 `390x844` 视口验证 14 条策略及 13 条 critical 锁定。桌面页无整体溢出；移动端文档宽度为 390，宽表只在 368px 容器内滚动到 760px；干净授权会话的 70 个业务请求全部为 200，控制台 0 error/0 warning。截图位于 `output/playwright/0078-approval-policies-*.png`。
- 当前平台健康仍为 `32/33`，唯一 critical 是 24 小时统计窗口内保留的 7 次历史后台失败，未改写历史。本次启动新增 1 个审计锚点，当前 `48/48` 的本地工件、远端对象锁和校验状态均通过。预览继续运行在 `http://127.0.0.1:18090/dashboard/saasAdmin/page`。
- 生产证据 doctor、preflight 和 `docs/evidence/production/current` 已按最终指纹刷新：采集环境和标准证据均为 `0/6`，严格证据检查与 skip-local 生产候选门禁均返回 1。仍缺 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控 6 类外部证据。本轮未启动 24 小时运行，因此当前不标记生产最终完成。

## 本轮 0079 服务账号 API Key 吊销双人审批收口

- 新增迁移 `0079_saas_service_account_key_revoke_guard` 和第 15 条审批策略 `service_account.key.revoke`。该动作属于第 14 条 critical 策略，固定为强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效，审批治理不能关闭或降级。
- 服务账号 Key 直接吊销现在返回 `428 Precondition Required`。审批申请会校验并冻结 `serviceAccountId`、`keyId`、`expectedVersion`、目标和申请原因，陈旧版本、已吊销 Key 或账号不匹配均会拒绝；审批执行把 Key 吊销、平台操作审计和审批效果操作 ID 放在同一 MySQL 事务中，租约恢复不会重复产生业务副作用。
- 总后台吊销弹窗要求填写申请原因，并根据有效策略显示“提交吊销审批”。专项单元测试和真实 Go + MariaDB + Redis smoke 已覆盖直接阻断、两名不同复核人、执行、效果标记以及吊销后旧 Key 返回 `401`。
- 全量 Go 测试、79 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、审批治理、服务账号、发布准备、独立包、独立 Compose、功能矩阵和六套本地短验收均通过。`docs/evidence/latest` 于 `2026-07-16 09:16:54 CST` 至 `10:27:59 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 157 个文件，本机 ARM 实容器按规则跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `929529920e3b3eccfa5b6b4a5c704443a60959141acef1f84a4b4d4f677995`，纳入 889 个文件。保留数据卷的预览已升级到镜像 `sha256:27b1dfe860c8865ba8fb1d2d6981efaee8385b7ae766f5a8192b11107f81f865`；`/readyz` 返回 200，迁移账本为 `79/0079_saas_service_account_key_revoke_guard`，审批策略为 15 条，目标策略实测为 `enabled=1`、零阈值、`2/120/30/12`、`v1`。
- 升级前快照为 `output/backups/mochat-preview-before-0079-service-account-key-revoke-guard-20260716-103008.sql`，权限 `0600`、大小 1206245 字节、SHA-256 `bf719a02bc6f2eb5aabb015865980db579263e1680a37bd46dcd10bdc76e37a9`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 和 `390x844` 验收策略行、服务账号 Key 和吊销申请弹窗。桌面与移动端均无页面级横向溢出；移动审批表只在 368px `.tablewrap` 内滚动，352px 宽弹窗和提交按钮完整可见。最终干净重载的 69 个业务请求均成功，控制台 0 error/0 warning；截图位于 `output/playwright/saas-0079-service-account-key-revoke/`。
- 当前平台健康为 `32/33`，唯一 critical 是 24 小时统计窗口内保留的 6 次历史后台任务失败；本轮没有删除或改写历史。50 个审计签名锚点均已生成本地独立证据、导出到异地 Object Lock 并校验通过。
- 生产 evidence doctor、preflight 和 current 候选证据仍为 `0/6`。缺口是 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端以及目标环境短稳/外部监控；严格门禁按预期阻断，`require_24h=false`，本轮没有启动 24 小时运行。

## 本轮 0080 服务账号配置变更双人审批收口

- 新增迁移 `0080_saas_service_account_update_guard` 和第 16 条审批策略 `service_account.update`。该动作属于第 15 条 critical 策略，固定为强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效，审批治理不能关闭或降级。
- 服务账号有效配置的直接更新现在返回 `428 Precondition Required`。申请阶段由服务端规范化并冻结账号状态、作用域、CIDR、分钟/每日额度、用量与拒绝预警、冷却时间、有效期和当前版本；陈旧版本申请返回 `409`，无效输入仍在审批门禁前返回 `400`。
- 审批执行重新校验冻结版本，并将服务账号更新、平台操作审计和审批效果操作 ID 放在同一 MySQL 事务中；租约恢复不会重复应用业务副作用。总后台编辑既有账号时显示“提交修改审批”，新建账号保持直接创建语义。
- 全量 Go 测试、80 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、服务账号、审批治理、发布准备、系统健康、独立包、功能矩阵和六套本地短验收均通过。最终证据包于 `2026-07-16 11:23:35 CST` 至 `12:39:14 CST` 从头生成，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 159 个文件，本机 ARM 实容器按规则跳过且不充当 amd64 证据。
- 最终源码与验收指纹为 `a52c324c86a302f06d63095d477aa259585b51f650462013dc88b0342c4378fe`，纳入 891 个文件。保留数据卷的预览已升级到镜像 `sha256:c39d8a30362dbd83248ae87d1f790e34a67e13aebe3a3c324d3a7e8dde06b9ea`；`/readyz` 返回 200，迁移账本为 `80/0080_saas_service_account_update_guard`，审批策略为 16 条，目标策略实测为 `enabled=1`、零阈值、`2/120/30/12`、`v1`。
- 升级前快照为 `output/backups/mochat-preview-before-0080-service-account-update-guard-20260716-124042.sql`，权限 `0600`、大小 1224462 字节、SHA-256 `a04faaaa47503f8957f23881868ff038a089318ddd07f000473b98b9d06a7b5f`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 已在 `1440x1000` 与 `390x844` 验收策略行、服务账号编辑与提交入口。两种视口均无页面级横向溢出；移动审批表外层为 368px、内部表格为 760px 且只在 `.tablewrap` 内滚动，“提交修改审批”按钮文本完整。最终授权会话累计 146 个动态请求均为 2xx，控制台 0 error/0 warning；截图位于 `output/playwright/saas-0080-service-account-update/`。
- 当前平台健康为 `32/33`，唯一 critical 是 24 小时统计窗口内保留的 6 次历史后台任务失败，当前无活跃事故；51 个审计签名锚点均已生成本地工件、导出到异地 Object Lock 并校验通过。
- 发布门禁读取的新构建指纹与 latest 证据一致，但生产 evidence 仍为 `0/6`。缺口是 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；严格门禁按预期阻断，`require_24h=false`，本轮没有启动 24 小时运行。

## 本轮 0081 服务账号 API Key 轮换双人审批与一次性密钥收口

- 新增迁移 `0081_saas_service_account_key_rotate_guard` 和第 17 条审批策略 `service_account.key.rotate`。该动作属于第 16 条 critical 策略，固定为强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；审批治理不能关闭或降低会签门槛。
- 服务账号 Key 直接轮换现在返回 `428 Precondition Required`。申请阶段只冻结账号 ID、当前版本、新 Key 名称、过期时间和旧 Key 宽限分钟数，不生成或保存明文 Key；陈旧版本申请返回 `409`。审批执行重新校验版本后才生成新 Key，并把新 Key 入库、旧 Key 退役、账号版本更新、平台操作审计和审批效果操作 ID 放在同一 MySQL 事务中。
- 明文 Key 只在首次成功执行的 HTTP 响应中返回。持久化审批结果会移除 `plainTextKey`，只保留 `plainTextKeyDelivered=true`；总后台执行审批后显示一次性 Key 区域，后续查询、租约恢复和操作日志均不能重新取回明文。总后台轮换入口显示“提交轮换审批”。
- 全量 Go 测试、81 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、服务账号、审批主流程、审批治理、发布准备、系统健康、独立包、独立 Compose、功能矩阵和六套本地短验收均通过。最终 `docs/evidence/latest` 于 `2026-07-16 13:34:00 CST` 至 `15:32:10 CST` 完整补齐，`14/14` 返回 0；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。MySQL 5.7 静态检查覆盖 161 个迁移文件，本机 ARM 上的 amd64 实容器按规则跳过且不充当生产证据。
- 最终源码与验收指纹为 `d4a07979b8cc5b7e7814f5debbb5488ba97ef6fab94c8ae1ef6bb876fa35e81f`，纳入 893 个文件。保留数据卷的预览已升级到镜像 `sha256:b3fb68ff27bc75576f333a8fb642a8e677630f8ae1796166b6d8f61edc81127e`；`/readyz` 返回 200，运行时路由 706 条，PHP fallback 关闭，迁移账本为 `81/0081_saas_service_account_key_rotate_guard`。审批策略共 17 条，其中 16 条 critical；目标策略实测为强制启用、零阈值、`2/120/30/12`、`v1`。
- 升级前快照为 `output/backups/mochat-preview-before-0081-service-account-key-rotate-guard-20260716-153441.sql`，权限 `0600`、大小 1250259 字节、SHA-256 `4bc7c7e94c443589ad425085372d2f7ba26ff3e11403c1db2a095f1776c890bf`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留；启动日志确认 `source=build` 且内置指纹与 latest 一致。
- Playwright 已在 `1440x1000` 与 `390x844` 验收轮换策略、服务账号列表和轮换编辑区。页面无整体横向溢出；移动审批表只在 368px 容器内滚动到 760px，服务账号表只在 368px 容器内滚动到 1390px；“提交轮换审批”按钮完整，一次性明文 Key 区域在未执行审批时保持隐藏。最终干净重载 70 个总后台动态请求全部返回 200，无失败请求，控制台 0 error/0 warning。验收工件位于 `output/playwright/saas-0081-service-account-key-rotate/`。
- 浏览器验收只打开轮换编辑区，没有提交审批或修改预览数据；数据库中 `service_account.key.rotate` 审批单仍为 0，验收账号版本仍为 `v9`。当前平台健康为 `32/33`，唯一 critical 是统计窗口内保留的 6 次历史后台任务失败；52 个审计锚点均已生成本地工件、导出到异地 Object Lock 并校验通过。
- 本轮目标完成度审计仍明确列出 6 类外部生产证据缺口：MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控。`require_24h=false`，本轮没有启动 24 小时运行，因此整体目标继续保持“本地 0081 收口、待生产补证”。

## 本轮 0082 服务账号创建双人审批与一次性首密钥收口

- 新增迁移 `0082_saas_service_account_create_guard` 和第 18 条审批策略 `service_account.create`。该动作属于第 17 条 critical 策略，固定为强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；审批治理不能关闭或降低会签门槛。
- 服务账号直接创建现在会在任何 Key 生成之前返回 `428 Precondition Required`。申请阶段只规范化并冻结租户、账号代码、名称、Scope、CIDR、额度、预警、有效期以及首个 Key 的名称和过期时间，不生成明文 Key；停用租户、重复代码和无效载荷会在申请时拒绝。
- 审批执行阶段重新校验租户和冻结载荷后才生成首个 Key，并将服务账号、Key 摘要、平台操作审计和审批效果操作 ID 放在同一 MySQL 事务中。持久化审批结果会移除 `plainTextKey`，只保留 `plainTextKeyDelivered=true`；租约恢复和后续查询不能重新取得明文。
- 总后台新建模式的按钮已改为“提交创建审批”。审批完成并首次执行后才显示一次性首密钥；执行前页面不渲染明文 Key 区域，执行后关闭或刷新即不可恢复。
- 定向 Go 单测、`scripts/test.sh`、82 版迁移 apply/rollback/replay、服务账号、审批主流程、审批治理、系统健康、备份恢复和独立交付 smoke 均通过。最终短证据包于 `2026-07-16 19:24:10 CST` 收口，为 `14/14`；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`；源码指纹为 `8d6dbe55ec10e5ec647a02892ef9306380304cd98ef7ff7fae6638b8387481a9`，文件数 895。
- `scripts/collect_standalone_evidence.sh` 新增 `MOCHAT_LOCAL_EVIDENCE_RESUME=1`：会保留已有证据，按唯一 `slug` 跳过成功项，并原子替换失败或重跑结果。本轮在 frontend 完成后恢复执行 `mysql57`、路由、前端覆盖、阶段报告、生产证据和目标审计，最终无重复记录、14 项全部通过。
- 保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:c26ad3e42388a19cf89948c863a5e17ed26e9448a24f3654727c999a83c64d40`、迁移 `82/0082_saas_service_account_create_guard` 和 18 条审批策略，其中 17 条 critical。`service_account.create` 实测为 `enabled=1`、零阈值、`2/120/30/12`、`v1`；运行时路由 706 条，PHP fallback 关闭。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0082-service-account-create-guard-20260716-174445.sql`，大小 1263802 字节、SHA-256 `303e99f672f38477e2b8fe1bd0ff3d4e268bd426c8aeda7a172aa1999b1c5d19`。
- Playwright 在 `1440x1000` 与 `390x844` 验收创建策略和服务账号新建区。桌面与移动端均无页面级横向溢出；移动审批表只在 368px 容器内滚动到 760px，服务账号表只在 368px 容器内滚动到 1390px；69 个总后台动态请求全部返回 200，控制台 0 error/0 warning。验收工件位于 `output/playwright/saas-0082-service-account-create/`。
- 生产证据 doctor、preflight 和 current 包已按最终指纹刷新，采集环境与有效标准证据仍为 `0/6`，严格证据包按预期返回 1。缺口仍是 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；`require_24h=false`，本轮未启动任何 24 小时运行。

## 本轮 0083 用户 MFA 重置双人审批收口

- 新增迁移 `0083_saas_identity_mfa_reset_guard` 和第 19 条审批策略 `identity.mfa.reset`。该动作属于第 18 条 critical 策略，固定为强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；审批治理不能关闭或降低会签门槛。
- 管理员直接重置用户 MFA 现在返回 `428 Precondition Required`。申请阶段校验并冻结目标用户、租户、MFA 凭据版本和标准化原因；执行阶段重新校验冻结快照，版本漂移或归属变化会拒绝执行。
- MFA 禁用、TOTP 密钥与恢复码清除、未完成挑战失效、全部活动会话撤销、身份安全事件、平台操作审计和审批效果操作 ID 在同一 MySQL 事务内提交。审批恢复执行复用既有效果标记，不会重复重置或重复写入业务副作用。
- 定向 Go 单测、`scripts/test.sh`、83 版迁移 apply/rollback/replay、身份安全、审批主流程、审批治理、系统健康、独立交付和六套本地短验收均通过。最终 `docs/evidence/latest` 于 `2026-07-16 20:13:04 CST` 至 `21:15:04 CST` 从头生成，14 条记录全部 `returncode=0`；smoke `96/96`、manifest `224/224`、runtime unique paths `587/587`、runtime route entries `729`、前端 dist API `209/209`。
- 最终源码与验收指纹为 `7e7d5073a0f2adb08789999156caf01c6bfb2a99fc9220e525506ec9606f1699`，纳入 898 个文件。保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:889a374354f7de2c7c6ca060ffa54983643ae13197ed7b07e186ae4c713c3737`；`/readyz` 返回 200，迁移账本为 `83/0083_saas_identity_mfa_reset_guard`，审批策略为 19 条，其中 18 条 critical，启动日志中的 build 指纹与 latest 一致。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0083-identity-mfa-reset-guard-20260716-200020.sql`，大小 1285262 字节、SHA-256 `275f412825fa7362e67b6d230ca342f860230d5454ac450efcbf15b824d0eecd`。升级只替换 App，MySQL、Redis、MinIO 和原数据卷均保留。
- Playwright 在 `1440x1000` 与 `390x844` 验收目标策略和系统健康页。页面无整体横向溢出；移动审批表只在内部 760px 容器滚动，操作按钮完整可见。干净授权会话没有失败请求，控制台 0 error/0 warning；截图位于 `output/playwright/saas0083-mfa-reset/`。
- 当前平台健康为 `33/33`，critical、warning、未处理问题和活跃事故均为 0。目标完成度审计仍明确列出 6 类外部生产证据缺口：MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控。`require_24h=false`，本轮没有启动 24 小时运行，因此整体目标继续保持“本地 0083 收口、待生产补证”。

## 本轮 0084 平台套餐定义双人审批与版本冻结收口

- 新增迁移 `0084_saas_package_definition_guard` 和审批动作 `package.upsert`。该动作是第 19 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.tenants.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；当前总策略数为 20，其中 critical 19 条。
- 平台套餐创建和修改的直接写入现在返回 `428 Precondition Required`。申请阶段由服务端规范化并冻结目标套餐、当前套餐快照、`expectedVersion`、目标限制以及受影响租户和超限情况；执行阶段重新读取并锁定套餐，版本漂移或重复创建会返回 `409 Conflict`。
- 套餐创建从版本 1 开始，后续更新逐次递增。套餐写入、平台操作日志和审批效果操作 ID 在同一 MySQL 事务内提交；若执行因版本漂移失败，不会提前写审批效果或操作日志，租约恢复也不会重复应用套餐变更。
- 总后台套餐区新增描述和只读版本字段，保存入口改为“提交套餐审批”。审批中心可查看冻结的当前值、目标值和影响分析，避免审批期间套餐或租户数据变化导致复核对象漂移。
- 本轮短门禁已通过：`go test ./internal/dashboard ./internal/store`、`scripts/test.sh`、84 版迁移 apply/rollback/replay、SaaS 总后台 smoke、套餐定义审批专项真实数据库 smoke 和功能矩阵审计。专项 smoke 覆盖直写阻断、双人复核、创建与更新、冻结影响、陈旧申请冲突、事务原子性和恢复幂等。本轮未重建 `docs/evidence/latest` 的完整 `14/14` 证据包，因此不沿用上一轮完整证据数字作为 0084 结论。
- 保留 MySQL、Redis、MinIO 和原数据卷的预览已原位升级到镜像 `sha256:db5e921d2159891098365fd43fd7ddebe461ddb168e8940516c4e5c5d36e21c0`。`/readyz` 返回 200，迁移账本为 `84/0084_saas_package_definition_guard`，策略为 `20/19 critical`，平台健康 `33/33`；启动日志确认 `source=build`，内置源码指纹为 `8c3229b66db779af3e1cb67968a433b2b474af2615e45d0a794f31c1b55a18d5`。
- 升级前 `0600` 数据库快照为 `/tmp/mochat-preview-before-0084-20260716-214911.sql`，大小 1313702 字节、SHA-256 `bf4ac595665478aa41590cd0709625030f29025416a569e61d255ef3f420f0cc`。升级只替换 App 并应用 0084 迁移，原数据卷保持不变。
- Playwright 已在 `1440x1000` 和 `390x844` 验收套餐编辑区与审批策略。页面无整体横向溢出，移动端字段和“提交套餐审批”按钮完整可见；策略行显示强制启用和 `2/120/30/12`，干净授权会话控制台 0 error/0 warning。截图为 `output/playwright/saas-0084-package-approval-desktop.png`、`saas-0084-package-approval-mobile-390x844.png` 和 `saas-0084-package-policy-row-desktop.png`。
- 本地 0084 套餐治理已经收口，但生产最终完成仍缺 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类外部证据。本轮按要求未运行 24 小时测试，整体目标继续保持“本地 0084 收口、待生产补证”。

## 本轮 0085 租户套餐分配双人审批与权益版本冻结收口

- 新增迁移 `0085_saas_tenant_package_assignment_guard` 和审批动作 `tenant.package.update`。该动作是第 20 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.tenants.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；当前总策略数为 21，其中 critical 20 条。
- 租户套餐分配的直接写入现在返回 `428 Precondition Required`。申请阶段冻结租户、当前套餐、目标套餐、订阅状态和 `expectedAssignmentVersion`；执行阶段重新锁定租户并校验版本，续费、开户和套餐同步造成的权益版本漂移会拒绝旧申请，避免审批对象被后续操作覆盖。
- 套餐分配、订阅权益更新、平台操作审计和审批效果操作 ID 在同一 MySQL 事务内提交。失败不会提前留下审批效果，租约恢复不会重复应用套餐或订阅副作用；相关续费、开户和同步链路会递增租户套餐分配版本。
- 总后台租户列表新增“调整套餐”入口，表单展示只读当前版本、变更说明和“提交分配审批”。审批中心可查看冻结的租户与套餐快照，不再允许单人直接改变租户权益。
- 本轮短门禁已通过：定向 Go 测试、`scripts/test.sh`、85 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、套餐分配审批专项真实数据库 smoke、SaaS 总后台、审批主流程、审批治理、独立包、发布准备和功能矩阵。`scripts/test.sh` 实测 smoke 覆盖 `98/98`、manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。本轮没有重建完整 `docs/evidence/latest` 的 `14/14` 包，也没有启动 24 小时运行。
- 最终源码指纹为 `4ba1c159ca7f944dee5f2701a5479328cb0c3858c50302467a6bafe5338e17b8`，纳入 906 个文件。保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:52d362570a54f44172337127f8d12763f90d2424048adb6d9a32fc968d3677f0`；迁移账本为 `85/0085_saas_tenant_package_assignment_guard`，策略为 `21/20 critical`，运行时路由 706 条，PHP fallback 关闭，平台健康 `33/33`，启动构建指纹与最终源码一致。
- 升级前 `0600` 数据库快照为 `/tmp/mochat-preview-before-0085-20260716-224216.sql`，大小 1323664 字节、SHA-256 `4473f0fcdf2df4001e0975fafe4f95f2f1f3ab950e5f4349d5d2ddf1f83c9386`。升级只替换 App 并应用 0085 迁移，原数据卷保持不变。
- Playwright 已在 `1440x1000` 与 `390x844` 验收套餐分配表单和审批策略。点击“调整套餐”后租户 ID 1 和当前版本 1 正确回填，但未提交审批或修改预览数据；两种视口均无页面级横向溢出，移动表单宽度 340px 且按钮完整，审批宽表只在内部容器滚动。69 个动态请求全部返回 200，控制台 0 error/0 warning；截图位于 `output/playwright/saas-0085-tenant-package-assignment/`。
- 本地 0085 租户套餐治理已经收口。生产最终完成仍缺 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类外部证据；当前目标继续保持“本地 0085 收口、待生产补证”。

## 本轮 0086 平台租户开户双人审批与凭据冻结收口

- 新增迁移 `0086_saas_tenant_provision_approval_guard` 和审批动作 `tenant.provision`。该动作是第 21 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.tenants.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；当前总策略数为 22，其中 critical 21 条。
- 平台直接开户、单任务直接应用和批量直接应用在强制审批开启时均返回 `428 Precondition Required`。总后台入口统一改为“提交开户审批”“提交任务审批”和“批量提交开户审批”，不再保留单人直接产生租户、管理员和订阅权益的入口。
- 开户任务新增乐观版本；审批申请会冻结任务版本、请求 SHA-256、套餐代码与版本、管理员密码哈希和开户预览。管理员初始密码使用与 PHP 兼容的 bcrypt 哈希持久化，任务、审批、事件、操作日志和 API 响应均不返回明文密码或 `adminPasswordHash`，只暴露 `hasAdminPasswordHash=true`。
- 审批执行会重新校验任务版本、请求摘要和套餐版本，并将租户、管理员、角色、菜单、套餐快照、订阅、开户运行记录、平台操作审计、任务状态和审批效果操作 ID 放在同一 MySQL 事务提交。任务取消、请求漂移或套餐版本变化会返回冲突，恢复执行不会重复开户。
- 本轮短门禁已通过：`go test ./...`、`scripts/test.sh`、86 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、平台开户审批专项真实数据库 smoke、SaaS 审批主流程、审批治理、租户套餐分配审批、独立包、功能模块矩阵和验收覆盖审计。`scripts/test.sh` 与验收覆盖为 `99/99`、manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`、SaaS 总后台运行时路径 179 条。本轮没有重建完整 `docs/evidence/latest` 的 `14/14` 包。
- 最终源码指纹为 `ab8bed7be3de30847ce7ccfff5d51e52eee66b1a2368cd91dca1f540a607a897`，纳入 910 个文件。保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:f907708826f3de90382c612c4f80469c2b9244ce384d5bcba4905286a07ee595`；迁移账本为 `86/0086_saas_tenant_provision_approval_guard`，策略为 `22/21 critical`，运行时路由 706 条，PHP fallback 关闭，平台健康 `33/33`，启动日志确认 `source=build` 且内置指纹与当前源码一致。运行时复核确认登录成功、策略 API 为 `22/21`、直接开户返回 `428` 且副作用为 0。
- 升级前 `0600` 数据库快照为 `/tmp/mochat-preview-before-0086-20260716-234326.sql`，大小 1332265 字节、SHA-256 `51ccb9cb742d99fac89e2b942c8b8895b3058c3400b14076a4441e9a9c4a5509`。升级仅替换 App 并应用 0086 迁移，原数据卷保持不变。
- Playwright 已在 `1440x900` 与 `390x844` 验收最终预览。两种视口均无页面级横向溢出，移动端无按钮文字裁切，开户审批按钮和 `tenant.provision` 策略行完整，干净授权会话控制台 0 error/0 warning。截图位于 `output/playwright/mochat-saas-admin-0086-desktop.png`、`mochat-saas-admin-0086-mobile.png`、`mochat-saas-admin-0086-provision-form.png` 和 `mochat-saas-admin-0086-tenant-provision-policy.png`。
- 生产证据 doctor 与 preflight 已刷新，仍为有效标准证据 `0/6`、采集环境就绪 `0/6`。剩余缺口是 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；本轮没有启动任何 24 小时运行，因此整体目标继续保持“本地 0086 收口、待生产补证”。

## 本轮 0087 租户续费双人审批与原子账务收口

- 新增迁移 `0087_saas_tenant_renewal_approval_guard` 和审批动作 `tenant.renewal`。该动作是第 22 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.tenants.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；当前总策略数为 23，其中 critical 22 条。
- 直接续费、单任务直接应用和批量直接应用在强制审批开启时均返回 `428 Precondition Required`。总后台入口统一为“提交续费审批”“提交任务审批”和“批量提交续费审批”，仍允许先创建待审批续费任务，不保留单人直接改变套餐、订阅和账务的入口。
- 审批申请冻结租户状态、套餐分配版本、套餐代码与版本、订阅是否存在及版本、续费任务 ID 与版本、请求 SHA-256 和审批执行引用。执行阶段重新锁定并校验冻结快照，任务取消、请求变化、套餐或订阅版本漂移均返回 `409`。
- 套餐分配、续费账单、订阅生命周期、续费任务状态、平台操作审计和审批效果操作 ID 在同一 MySQL 事务提交，恢复执行不会重复产生账单或重复延长权益。日期校验同时限制 MySQL 5.7 `TIMESTAMP` 上界 `2038-01-18 23:59:59`，越界输入返回 `400`，合法边界日期已通过专项测试。
- 本轮短门禁已通过：定向与全量 Go 测试、87 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、续费审批专项真实 MariaDB/Redis smoke、审批主流程、审批治理、系统健康、发布准备、独立包和功能矩阵。专项 smoke 覆盖直写阻断、任务与批量阻断、双人复核、冻结摘要、原子执行、取消任务冲突、订阅漂移、直接申请幂等和 `2038-01-01` 合法日期。验收脚本覆盖为 `100/100`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。本轮未重建完整 `docs/evidence/latest` 的 `14/14` 包。
- 最终源码指纹为 `529e3a748d82b3e767f79826695905bea6ddefeda5fbdc18e779db9c3543dafb`，纳入 914 个文件。保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:68be686d5f1dc21a1be77b534f326a1f502a78ad5a3820f9feea635d108fd742`；迁移账本为 `87/0087_saas_tenant_renewal_approval_guard`，迁移 checksum 为 `a45c279c34caa1502f095b7abb764544405a57616ee649ce0f9a031c1496ffa8`，运行时路由 706 条，PHP fallback 关闭，build 指纹与当前源码一致。
- 升级前 `0600` 数据库快照为 `/tmp/mochat-preview-before-0087-20260717-003852.sql`，大小 1345367 字节、SHA-256 `567266aec4052424edfd07cb88bcf5e158af70b681bf48ac40eb6e9e4d86018d`。升级仅替换 App 并应用 0087 迁移，数据服务容器和原数据卷保持不变。
- 运行态真实登录复核通过：审批策略为 `23/22 critical`，`tenant.renewal` 为强制启用和 `2/120/30/12`；平台健康 `33/33`、活跃事故 0；发布准备使用 build 权威指纹，仍为 `0/6`、`ready=false`。预览地址继续为 `http://127.0.0.1:18090/dashboard/saasAdmin/page`。
- Playwright 已在 `1440x900` 与 `390x844` 验收续费表单、任务入口和审批策略。两种视口均无页面级横向溢出，移动端四个续费相关按钮均为 340px 宽且无裁切，控制台 0 error/0 warning；浏览器验收未提交审批或修改预览数据。截图为 `output/playwright/mochat-saas-admin-0087-desktop.png`、`mochat-saas-admin-0087-mobile.png` 和 `mochat-saas-admin-0087-policy.png`。
- 生产证据 doctor、preflight 和严格 current 检查已刷新：`missing_count=6`、`not_ready_count=6`，严格门禁按预期返回 1。剩余项仍是 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；本轮未启动任何 24 小时运行，整体目标继续保持“本地 0087 收口、待生产补证”。

## 本轮 0088 租户订阅状态迁移双人审批与冻结执行收口

- 新增迁移 `0088_saas_subscription_transition_approval_guard` 和审批动作 `tenant.subscription.transition`。该动作是第 23 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；当前总策略数为 24，其中 critical 23 条。
- 租户订阅状态的直接迁移现在返回 `428 Precondition Required`。申请阶段由服务端冻结租户状态、订阅 ID 与版本、目标状态、试用/周期/宽限结束时间、周期末取消标记、规范化原因和审批执行引用；平台租户不能进入迁移申请。
- 审批执行阶段重新锁定订阅和租户，并校验冻结的订阅 ID、版本与租户状态。任一漂移都会返回 `409`；订阅状态、订阅事件、平台操作审计和审批效果操作 ID 在同一 MySQL 事务提交。暂停后旧会话失效、新登录返回 `403`，恢复有效后可重新登录；自动对账仍只应用服务端计算出的派生状态，不经过人工审批入口。
- 本轮短门禁已通过：定向与全量 `go test ./...`、88 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、订阅迁移审批专项真实 MariaDB/Redis smoke、审批主流程、审批治理、系统健康、发布准备、功能矩阵和 `scripts/test.sh`。专项 smoke 覆盖直接阻断、请求幂等、双人复核、原子执行、旧 Token 失效、版本漂移、租户状态漂移、平台租户阻断和自动对账。验收覆盖为 `101/101`，manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`。本轮未重建完整 `docs/evidence/latest 14/14`。
- 最终源码指纹为 `1bce67b2dd8031457cdf59dc69b556256377760843bc500014d1bac74146e417`，纳入 918 个文件。保留 MySQL、Redis、MinIO 和原数据卷的预览已升级到镜像 `sha256:257fc6a0967a602c8cfeaa817f67eb86bf93735c455882876a0bf360edf81d9c`；迁移账本为 `88/0088_saas_subscription_transition_approval_guard`，迁移 checksum 为 `54871987fbe6003b5bf0b616a316c305449ce76b8f00ec1325f48a246a3a8b50`，审批策略为 `24/23 critical`，运行时路由 706 条，PHP fallback 关闭，平台健康 `33/33`，build 权威指纹与当前源码一致。
- 升级前 `0600` 数据库快照为 `/tmp/mochat-preview-before-0088-20260717-012431.sql`，大小 1355318 字节、SHA-256 `6d924ce2d2ed6cb831920c8208aa25f486cd93b665197a0444daa640c9205de2`。升级只替换 App 并应用 0088 迁移，数据服务容器和原数据卷保持不变。
- Playwright 已在 `1440x900` 与 `390x844` 验收总后台、订阅迁移表单和审批策略。两种视口的页面宽度均等于视口宽度，无页面级横向溢出；移动表单和“提交订阅审批”按钮完整可见，策略行实测为强制启用、`2/120/30/12`。授权后控制台 0 error，网络无 4xx/5xx，浏览器验收未提交审批或修改预览数据。截图为 `output/playwright/mochat-saas-admin-0088-desktop.png`、`mochat-saas-admin-0088-mobile.png`、`mochat-saas-admin-0088-transition-desktop.png`、`mochat-saas-admin-0088-transition-mobile.png` 和 `mochat-saas-admin-0088-policy-desktop.png`。
- 生产证据 doctor、preflight 和 `docs/evidence/production/current` 已按当前指纹刷新：`missing_count=6`、`not_ready_count=6`、`evidence_ok=false`；严格 current 按预期返回 1。发布准备仍为 `0/6`、`metadataReady=false`、`ready=false`。生产最终完成继续等待 MySQL 5.7 amd64 实容器、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类证据；本轮未启动 `standalone_soak_24h.sh` 或任何 24 小时运行，整体目标保持“本地 0088 收口、待生产补证”。

## 本轮 0089 发票正式开具双人审批与账务冻结收口

- 新增迁移 `0089_saas_invoice_issue_approval_guard` 和审批动作 `billing.invoice.issue`。该动作是第 24 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；当前总策略数为 25，其中 critical 24 条。
- 蓝票或红票直接推进到 `issued` 现在返回 `428 Precondition Required`。申请阶段由服务端冻结单据身份、类型、状态、版本、票面信息，以及支付订单版本、退款金额、已开票金额和已红冲金额；执行前重新锁定单据与订单，任一冻结引用漂移均返回 `409`。
- 单据状态、支付订单开票或红冲金额、平台操作审计和审批效果操作 ID 在同一 MySQL 事务提交。执行失败不会提前留下账务副作用，审批租约恢复不会重复开具；`processing`、`failed` 和 `canceled` 仍保留财务直接处理链路。
- 定向 Go 测试、真实 Go + MariaDB + Redis 发票开具审批 smoke、89 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、`scripts/test.sh`、独立包、功能矩阵和脚本语法门禁均通过。专项 smoke 覆盖直接开具阻断、非开具状态直通、申请幂等、双人复核、原子账务、操作审计、单据漂移和订单漂移。短时总门禁为 smoke `102/102`、manifest `224/224`、运行时唯一路径 `587/587`、运行时路由条目 `729`；MySQL 5.7 静态检查覆盖 177 个文件，本机 ARM 的真实 5.7 容器按规则跳过，不计作 amd64 生产证据。
- 最终源码与验收指纹为 `23fbbafd118f6c679d3402666f8a680ce13ddc13068f2b2b8c4f35b12cc02145`，纳入 922 个文件；0089 迁移 checksum 为 `39a732c3088f65ad93410d019b269a4b1b97a9ad0132f329a18a924b929b78bd`。
- 保留数据卷的预览已升级到镜像 `sha256:7183026feb6d0ba806f441b0a7833a58b92a2b39fd99f6ecaf041caae920a1a5`。升级前 `0600` 数据库快照为 `/tmp/mochat-preview-before-0089-20260717-021602.sql`，大小 1367156 字节、SHA-256 `344c3f92ab5ce983bb8482349296aaee7202e5e04b9a751d8fb27cc693deadcb`；只重建 App 并应用 0089，MySQL、Redis 容器 ID 与启动时间均未变化。
- 运行态复核为迁移 `89/0089_saas_invoice_issue_approval_guard`、策略 `25/24 critical`、运行时路由 706 条、PHP fallback 关闭、平台健康 `33/33`。升级前后均为 3 个租户和 3 个用户，现有发票及支付订单数量保持不变；启动日志确认权威指纹来源为 build。
- Playwright 在 `1440x900` 与 `390x844` 验收发票处理表单和审批策略。桌面与移动端均无页面级横向溢出；移动端“提交开具审批”按钮宽 338px、文字完整，85 个宽表只在内部容器滚动，策略可从动作列滚到 SLA 和提交列。干净会话 69 个动态请求全部返回 200，控制台 0 error/0 warning；截图位于 `output/playwright/saas-0089-invoice-issue/`。
- 生产 doctor、preflight 和 `docs/evidence/production/current` 已按最终指纹刷新：`missing_count=6`、`not_ready_count=6`、`evidence_ok=false`，严格证据检查和 skip-local 候选门禁均按预期返回 1。剩余六类外部证据仍是 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；本轮未启动任何 24 小时运行，整体目标保持“本地 0089 收口、待生产补证”。

## 本轮 0089 最终本地证据重建与预览复核

- `docs/evidence/latest` 已于 `2026-07-18 09:37:58 CST` 至 `11:45:03 CST` 完成最终重建，15 条命令记录全部 `returncode=0`。六套独立验收 `core/workers/cron/saas/frontend/mysql57` 全部通过，快速门禁、独立交付包、PHP 清单对齐、阶段报告、生产证据报告和目标完成度报告也均成功生成。
- 验收覆盖为 smoke `102/102`、PHP manifest 路由 `224/224`、未迁移路由 `0`、Go 额外运行时路由 `209`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`，前端 dist API 为 `209/209`。
- 最终源码与验收指纹为 `7c5ef5690ec238b7330aeefd85fe1df81d659835c6895d1eba054b97cd0085f1`，纳入 922 个文件。证据收集器会在长验收开始前写入指纹检查点，恢复执行时复用成功项，结束前再次验证工作区未漂移；本轮指纹稳定检查通过。
- 验收脚本的服务健康等待已统一为可配置的 360 秒默认值；16 个独立 cron/provisioning smoke 与 bootstrap smoke 已显式启动隔离 Redis 并传入 `MOCHAT_REDIS_ADDR`，避免宿主机 Redis 或端口状态污染结果。前端浏览器验收通过真实总后台登录、侧栏联系人和运营裂变页面回归。
- 严格生产证据检查和 skip-local 生产候选门禁已刷新到同一指纹。源码匹配通过，但门禁按预期返回 1，唯一阻断仍是六项真实外部证据：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控。`require_24h=false`，本轮未启动任何 24 小时运行。
- 本地预览已使用全新验收卷重建并通过 `smoke_standalone_compose_app.sh`。访问地址为 `http://127.0.0.1:18090/dashboard/saasAdmin/page`，登录凭据仅保存在本地受控配置；App、MySQL 和 Redis 均健康。运行态为 `standalone-go`，迁移账本 `89/0089_saas_invoice_issue_approval_guard`，审批策略 `25/24 critical`，`billing.invoice.issue` 强制双人审批存在；镜像 `sha256:6b9b10d08f5763215c4e5687e3e589b413fca4fccbbffdeb9dc6fd0c37c62a75` 使用 build 权威指纹且与最终证据一致。

## 本轮 0090 收款订单创建双人审批与套餐快照收口

- 新增迁移 `0090_saas_payment_order_create_approval_guard` 和审批动作 `payment.order.create`。该动作是第 25 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；当前总策略数为 26，其中 critical 25 条。
- 收款订单直接创建现在返回 `428 Precondition Required`。申请阶段由服务端冻结有效租户、租户版本、套餐代码与定义版本、完整额度、金额、服务周期、收银台失效时间和幂等订单号；执行阶段重新锁定租户与套餐，任一版本或状态漂移均返回 `409`。
- 订单、平台操作审计和审批效果操作 ID 在同一 MySQL 事务提交。订单新增 `package_version` 与 `package_limits_json`，支付回调结算使用下单时冻结的套餐版本与额度；后续修改套餐定义不会改变已创建订单实际授予的权益，旧订单保留兼容回退。
- 定向 Go 测试、真实 Go + MariaDB + Redis 收款订单创建审批 smoke、90 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、`scripts/test.sh` 和 MySQL 5.7 静态门禁均通过。专项 smoke 覆盖直接阻断、申请幂等、双人复核、订单/审计/审批效果原子提交、套餐快照结算和审批后套餐漂移拒绝。短时总门禁为 smoke `103/103`、manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`，MySQL 5.7 静态检查覆盖 179 个文件。
- 当前源码与 build 权威指纹为 `f2c01af52235f4fe67f7bf7f6a4de6645da53dbcb4b7e0407399654f2f2da6f2`，纳入 926 个文件；0090 迁移 checksum 为 `edd6cd74fc3410ba6a757c6e5bba0ccabd4a3cf7894fa056444c98239ebb2299`。本轮没有重建完整 15 项本地证据包，也没有把本机 ARM 的静态检查记作 MySQL 5.7 amd64 生产证据。
- 保留数据卷的预览已升级到镜像 `sha256:ed6d123e71b3971ca83309181935964ea2721f8ea4b91ef7bb340e07e327f076`、迁移 `90/0090_saas_payment_order_create_approval_guard` 和 `26/25 critical` 审批策略；只重建 App，MySQL、Redis 和原数据卷保持不变。升级前快照 `/tmp/mochat-go-preview-pre-0090-20260718.sql` 权限为 `0600`、大小 516003 字节、SHA-256 为 `56046922adf892dd52fd33ce1b9e9a495421e21399cba604ddc49552624c81d1`。
- Playwright 已在桌面和 `390x844` 移动视口验收收款订单表单，“提交收款审批”按钮、套餐版本与额度快照均完整可见，无页面级重叠或裁切；截图为 `output/playwright/0090-payment-desktop.png` 和 `output/playwright/0090-payment-mobile.png`。首次未授权加载产生预期 `401`，预览未启用可选身份安全模块时四个身份接口返回既有 `501`，收款审批流程本身没有新增浏览器错误。
- 本地 0090 增量已收口，但生产最终完成仍等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类证据。`require_24h=false`，本轮没有启动任何 24 小时运行。

## 本轮 0091 渠道结算关账双人审批与完整台账冻结收口

- 新增迁移 `0091_saas_payment_settlement_close_guard`，并将既有审批动作 `payment.settlement.close` 从 high 升级为第 26 条 critical 策略。该策略固定强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、240 分钟 SLA、60 分钟提醒和 24 小时失效；当前总策略数 26，全部为 critical。
- 渠道结算批次直接关账现在返回 `428 Precondition Required`。申请阶段只接受精确批次编号，要求批次为 `reconciled` 且未解决差异为 0，并冻结批次 ID/编号、账期、渠道、渠道结算号、币种、状态、来源 SHA-256、全部对账计数、金额台账、版本和备注。
- 审批执行重新锁定批次并逐项比较完整冻结快照，既能拒绝常规版本漂移，也能识别未递增版本的金额篡改。批次关账、平台操作审计和审批效果操作 ID 在同一 MySQL 事务提交；失败不会提前关闭批次或留下已生效标记，重放保持幂等。批次重开仍保留财务直接纠错链路。
- 定向 Go 测试、结算关账专项真实 Go + MariaDB + Redis smoke、91 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、`scripts/test.sh` 和 MySQL 5.7 静态门禁全部通过。专项 smoke 覆盖直写 `428`、未解决差异阻断、申请幂等、双人复核、完整台账冻结、原子关账、无版本金额漂移拒绝和精确 `batchNo` 查询。短时验收覆盖为 `104/104`、manifest `224/224`；功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`，MySQL 5.7 静态检查覆盖 181 个文件。
- 当前源码与 build 权威指纹为 `704f43a147148e65a75ac2497a92bb4bfcbf67fc43daf16c0e7ef97932ac7be3`，纳入 930 个文件；0091 迁移 checksum 为 `bc47db225b65be316a7bdff0e25f4be7faab648c042eb486651a9547720d2cab`。本轮未重建完整 15 项本地证据包，也未把本机 ARM 静态检查视为 MySQL 5.7 amd64 生产证据。
- 保留数据卷的预览已升级到镜像 `sha256:fecb35b03a7b106be3a6679a91f98501f63289d821e061eeccba7822cdd1ec40`、迁移 `91/0091_saas_payment_settlement_close_guard` 和 `26/26 critical` 审批策略；App、MySQL、Redis 均健康，系统健康的迁移探针为 healthy，启动日志确认 `source=build` 且指纹一致。升级只替换 App，MySQL、Redis 和原数据卷保持不变。
- 升级前快照为 `output/backups/mochat-preview-before-0091-settlement-close-guard-20260718-142250.sql`，权限 `0600`、大小 520117 字节、SHA-256 `237442df6e0a4eaad1bc7c25b9e24941ca97ce76b07f63c631596fdb48b293a7`。
- Playwright 已在 `1440x1000` 和 `390x844` 验收审批策略。桌面策略行完整显示强制启用、2 人、`240/60/24` 和提交审批；移动端页面宽度与视口同为 390px，审批宽表只在内部容器滚动，滚到最右后按钮边界为 `262..369px`，没有页面级横向溢出或操作重叠。截图为 `output/playwright/mochat-go-0091-approval-policy-desktop.png`、`mochat-go-0091-approval-policy-mobile.png` 和 `mochat-go-0091-approval-policy-mobile-actions.png`。可选身份安全模块关闭时四个接口仍返回既有 `501`，结算审批没有新增浏览器错误。
- 本地 0091 增量已经收口，但整体仍不能标记生产最终完成。剩余六类外部证据不变：MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控；`require_24h=false`，本轮没有启动 24 小时运行。

## 本轮 0092 渠道结算重开双人审批与关账审计冻结收口

- 新增迁移 `0092_saas_payment_settlement_reopen_guard` 和审批动作 `payment.settlement.reopen`。该动作是第 27 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、240 分钟 SLA、60 分钟提醒和 24 小时失效；当前 27 条策略全部为 critical。
- 结算重开直接请求现在返回 `428 Precondition Required`。申请阶段只接受具有完整关账审计的 `closed` 批次，并冻结批次身份、账期、渠道、币种、导入与对账元数据、全部金额台账、版本、备注，以及关账人、关账时间和关账原因。关账与重开共用 `schemaVersion=2` 冻结载荷，同时兼容 0091 已经发起的旧版关账审批。
- 执行阶段重新锁定批次并逐项校验冻结快照；关账审计或金额字段即使绕过版本递增发生变化，也会返回 `409`。重开状态恢复、关账字段清空、平台操作审计和审批效果标记在同一 MySQL 事务提交，失败不会产生部分副作用，批准重放保持幂等。
- 定向 Go 测试、重开专项真实 Go + MariaDB + Redis smoke、既有关账专项 smoke、92 版迁移 apply/checksum/status/baseline/legacy/rollback/replay、`scripts/test.sh` 和 MySQL 5.7 静态门禁均通过。专项 smoke 覆盖直接阻断、非法状态和缺失关账审计、申请幂等、双人复核、原子重开、关账原因无版本漂移拒绝；短时覆盖为 `105/105`、manifest `224/224`，功能矩阵为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`，MySQL 5.7 静态检查覆盖 183 个文件。
- 同步修复独立 Compose 新库初始化只挂载到 0089 的发布缺口：0090、0091、0092 均已加入 `/docker-entrypoint-initdb.d`，并新增通用门禁，要求每个 `*.up.sql` 都必须有 Compose 初始化挂载。当前 91 个增量迁移和初始 schema 共 92 个初始化入口，独立交付包验收通过。
- 当前源码与 build 权威指纹为 `b9670008712828fbf602f94ff3a820947011b6fc34056c422912619ecb5b9e55`，纳入 933 个文件；0092 迁移 checksum 为 `3638ba7a955befb5e8ff28f891eeda717000d0128e3568a6cdc06fb1f6451d66`。保留数据卷的预览已升级到镜像 `sha256:f941527ccd96fb9f52d684e41a7a982bf371551aa6ac6e20797751765fbdbdeb`、迁移 `92/0092_saas_payment_settlement_reopen_guard` 和 `27/27 critical`；App、MySQL、Redis 均 healthy，启动日志确认 `source=build` 且指纹一致。
- 升级前快照为 `output/backups/mochat-preview-before-0092-settlement-reopen-guard-20260718-153137.sql`，权限 `0600`、大小 526679 字节、SHA-256 `c87464a87851fb7c94b52083ab75bc4a69f0e45e460112d3881e385df5facf3b`。升级只应用 0092 并替换 App，MySQL、Redis 和原数据卷均保留。
- Playwright 已在 `1440x900` 与 `390x844` 验收审批中心。桌面策略行完整显示强制启用、2 人和 `240/60/24`；移动端页面宽度与视口同为 390px，760px 审批表只在 368px 内部容器滚动，左右两侧内容和提交按钮均可触达。截图为 `output/playwright/mochat-go-0092-approval-policy-desktop.png`、`mochat-go-0092-approval-policy-mobile.png` 和 `mochat-go-0092-approval-policy-mobile-controls.png`。日志仅有保存令牌前的预期 `401` 和身份安全模块关闭时既有的 4 个 `501`，结算审批没有新增浏览器错误。
- 当前系统健康为 `27/30`；3 个 critical 分别是预览未启用自动备份、尚无成功隔离恢复演练、1 份历史企微凭据仍待加密轮换，均属于环境与运营证据，不是 0092 回归。发布准备仍为 `0/6`、`ready=false`，继续等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类外部证据；`require_24h=false`，未启动 24 小时运行。

## 本轮 0093 渠道结算差异处理双人审批与预览收口

- 新增迁移 `0093_saas_payment_settlement_resolve_guard` 和审批动作 `payment.settlement.resolve`，关闭财务管理员可直接解决、忽略或重新打开结算差异的治理缺口。该动作是第 28 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.finance.manage`、2 名不同复核人、240 分钟 SLA、60 分钟提醒和 24 小时失效；直接处理返回 `428`。
- 申请阶段在一致性只读事务中冻结完整差异条目和所属批次账务。执行阶段按批次、条目的固定顺序锁行，并逐项校验金额、匹配、处理审计和版本；合法迁移仅为 `open -> resolved/ignored` 与 `resolved/ignored -> open`。无版本字段变化的账务漂移同样返回 `409`，条目状态、批次汇总、操作审计和审批效果在同一事务提交。
- 定向与全量 Go 测试、`scripts/test.sh`、0093 专项真实 MariaDB/Redis smoke、关账与重开回归 smoke、结算主流程、审批治理、93 版迁移 apply/rollback/replay、独立包和 MySQL 5.7 静态门禁均通过。验收覆盖为 `106/106`；功能矩阵仍为 29 个模块、manifest 唯一路径 `213/213`、运行时唯一路径 `587/587`、运行时路由条目 `729`，MySQL 5.7 静态检查覆盖 185 个文件。
- 当前源码与 build 权威指纹为 `46217ddd6564fc2916dbdcda3f5addf1ac5b5449409e5ffdf3eca0c9628203c0`，纳入 937 个文件；0093 checksum 为 `522051435bdf11f1fba800d803b38209a0afd8ecb166e4d2b202c8e1f5c481fd`。保留数据卷的预览已升级到镜像 `sha256:3574cb3e1338e2951738c78b8a32e392aa37f542e8d39a50672354575c8d89e6`、迁移 `93/0093_saas_payment_settlement_resolve_guard` 和 `28/28 critical`；App、MySQL、Redis 均 healthy，181 项运行环境配置在切换前后零差异，启动日志确认 `source=build` 且指纹一致。
- 升级前快照为 `output/backups/mochat-preview-before-0093-settlement-resolve-guard-20260718-164721.sql`，权限 `0600`、大小 533593 字节、SHA-256 `7183d424c198e6db0281fcff7284bdadc0bb08f54a76deb7d05c641e01c774d4`。升级只应用 0093 并替换 App，MySQL、Redis 和原数据卷均保留。
- Playwright 已在 `1440x900` 与 `390x844` 验收总后台、审批策略和结算筛选。移动页面 `scrollWidth/clientWidth=390/390`，审批宽表只在 368px 内部容器横向滚动，结算筛选完整单列显示；桌面策略行完整显示强制启用、2 人和 `240/60/24`。截图、脱敏快照、控制台与 70 条网络记录位于 `output/playwright/saas-0093-settlement-resolve/`。0093 关键接口均为 `200`；仅保留填 JWT 前的预期 `401` 和身份安全关闭时既有的 4 个 `501`，浏览器未提交审批或修改预览数据。
- 当前系统健康仍为 `27/30`。3 个 critical 继续是自动备份调度未启用、尚无成功隔离恢复演练、1 份历史企微凭据待加密轮换；发布准备仍为 `0/6`、`ready=false`。生产完成继续等待 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产域名前端和目标环境短稳/外部监控六类外部证据；`require_24h=false`，没有启动 24 小时运行。

## 本轮 0094 租户域名路由变更双人审批与预览收口

- 新增迁移 `0094_saas_tenant_domain_command_guard` 和审批动作 `tenant.domain.command`，关闭主域名切换、域名启停、DNS 校验令牌轮换和域名删除可由单个域名管理员直接执行的治理缺口。该动作是第 29 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.domains.manage`、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效；五类直接操作返回 `428`，DNS 所有权验证不受影响。
- 申请阶段冻结目标域名与租户全部未删除域名的完整路由快照和摘要；执行阶段按固定顺序锁定并重新校验全部记录。任一版本、主域或候选路由漂移返回 `409`；域名状态、路由/TLS 交付任务、平台操作审计和审批效果标记在同一 MySQL 事务提交。轮换令牌只在批准执行时生成，不进入审批载荷或持久化结果。
- 定向与全量 Go 测试、`scripts/test.sh`、0094 专项真实 MariaDB/Redis smoke、原域名兼容 smoke、审批主流程和治理 smoke、94 版迁移 apply/checksum/down/replay/baseline/legacy、独立包与 MySQL 5.7 静态门禁均通过。验收接入覆盖为 `107/107`、manifest `224/224`、功能矩阵 29 个模块、运行时唯一路径 `588/588`、运行时路由 730 条；MySQL 5.7 静态检查覆盖 187 个文件。
- 当前源码与 build 权威指纹为 `111f573666625f1382b5908c9addefa56ac68ea16584a272a23ad95c1c339aa7`，纳入 945 个文件；0094 checksum 为 `a7f23b5c1d0c67abc18021bd20d7be68bea10c7f80dbc1943c0b680c0cb1ba1c`。最终 `15/15` 本地证据包与当前指纹一致。
- 升级前 `0600` 快照为 `output/backups/mochat-preview-before-0094-tenant-domain-command-guard-20260719-012928.sql`，大小 639408 字节、SHA-256 `ef006bae1e93c2ed2478419309e25469f56e826025f1a5c2390a2a1bb4714d06`。保留数据卷的预览只应用 0094 并单独重建 App，最终镜像为 `sha256:a6a28779d3f8567716e0b5342ff037af8bb581e663a740b3baef6ae4140e0053`；App、MySQL、Redis 均 healthy，重建前后 183 项运行环境配置零差异。
- 运行态复核为迁移 `94/0094_saas_tenant_domain_command_guard`、策略 `29/29 critical`、纯 Go standalone、PHP fallback 关闭、运行时路由 707 条；系统健康 `31/31`，critical、warning、未处理问题和活跃事故均为 0。发布准备读取 build 指纹并继续返回 `0/6`、`missing=6`、`ready=false`。
- Playwright 在 `1440x1000` 与 `390x844` 验收审批策略和租户自定义域名中心。页面无整体横向溢出，移动审批表为 368px 可视容器和 760px 内部宽表；策略行显示“变更租户域名路由”、强制启用和 `2/120/30/12`。截图位于 `output/playwright/saas0094/`；预览当前没有租户域名记录，浏览器未创建域名、提交审批或修改业务数据，唯一控制台错误是保存 JWT 前的预期 `401`。
- 本地 0094 增量已经收口，但生产最终完成仍缺 MySQL 5.7 amd64、真实企业微信、真实微信开放平台、两个以上真实 SaaS 租户、生产前端浏览器和目标环境短稳/外部监控六类外部证据。`require_24h=false`，本轮没有启动 24 小时运行。

## 本轮 0095 租户域名新增双人审批与预览收口

- `tenant.domain.create` 已成为第 30 条 critical 策略，固定强制启用、零绕过阈值、2 名不同复核人、120 分钟 SLA、30 分钟提醒和 12 小时失效。直接新增返回 `428`，DNS 所有权验证保持直接执行。
- 申请只冻结标准化后的 `tenantId + hostname`，不生成或持久化 DNS 校验令牌；批准执行时锁定租户并重新校验启用状态、全局域名唯一性和每租户 10 个域名配额，随后才生成令牌。域名、初始路由/TLS 交付状态、平台操作审计和审批效果标记同事务提交，持久化审批结果仅保留 `verificationTokenDelivered=true`。
- 95 版迁移、定向与全量 Go 测试、迁移 apply/checksum/down/replay/baseline/legacy、域名新增/变更审批专项、原域名兼容、发布准备、备份恢复、审批主流程/治理和独立 Compose 均通过。0095 checksum 为 `639cfdd943495ae04bcc100864258c71ae7f1e16f798c994b282ebaee572b6fd`。
- 最终本地证据包从 `2026-07-19 03:57:59 CST` 运行至 `05:22:22 CST`，结果 `15/15`；六套验收全绿，smoke `107/107`、manifest `224/224`、运行时唯一路径 `588/588`、运行时路由条目 `730`、前端 dist API `209/209`，验收前后源码指纹稳定。
- 预览升级前 `0600` 快照为 `output/backups/mochat-preview-before-0095-tenant-domain-create-guard-20260719-034908.sql`，大小 658052 字节、SHA-256 `6a171c22d397ae46577f63454c79ab7d9b91aa5ac87d8ec57347758776656073`。升级仅应用 0095 并替换 App，MySQL、Redis、数据卷与运行环境保持不变，环境差异为 0。
- 运行态为迁移 `95/0095_saas_tenant_domain_create_guard`、策略 `30/30 critical`、系统健康 `31/31`、纯 Go 路由 707 条和 PHP fallback 关闭。桌面 `1440x1000` 与移动 `390x844` 浏览器回归控制台 0 error/0 warning，动态请求均为 `200`，移动页面无整体横向溢出；浏览器未提交审批或修改数据，截图位于 `output/playwright/saas0095/`。
- 生产证据包已于 `2026-07-19 05:24:47 CST` 刷新，严格门禁按预期未通过，唯一原因仍是六类外部证据缺失。发布准备为 `0/6`、`ready=false`，`require_24h=false`，不启动 24 小时持续运行。

## 本轮 0096 租户重新启用双人审批与预览收口

- 新增迁移 `0096_saas_tenant_enable_approval_guard` 和审批动作 `tenant.enable`，关闭单个平台管理员可直接恢复已停用业务租户的治理缺口。该动作是第 31 条 critical 策略，固定强制启用、零绕过阈值、要求 `platform.tenants.manage`、2 名不同复核人、240 分钟 SLA、60 分钟提醒和 24 小时失效；审批开启时直接启用返回 `428`。
- 申请阶段冻结租户 ID、名称、状态和订阅存在性、ID、状态、版本；批准执行时重新锁定租户与订阅，任一快照漂移返回 `409`。租户状态、订阅恢复、平台操作审计和审批效果标记在同一 MySQL 事务提交；恢复原快照后可安全重试，旧版停用审批载荷继续兼容。
- 定向与全量 Go 测试、`scripts/test.sh`、96 版迁移 apply/checksum/down/replay/baseline/legacy、租户启停审批专项、审批治理、备份恢复和独立 Compose 均通过。最终本地证据包于 `2026-07-19 06:33:12 CST` 至 `07:40:06 CST` 完成，结果 `15/15`；六套短验收全绿，smoke `107/107`、manifest `224/224`、运行时唯一路径 `588/588`、运行时路由条目 730，验收前后指纹稳定。
- 当前源码与 build 权威指纹为 `ad95a7ba63dc2ec688355083c5cedbb9e81790ada25a4136ee01c01c06ae0e25`，纳入 950 个文件；0096 checksum 为 `9a5c47a06f395ef320399ad17100c47fdcae31fc372dd66fd5f588587ca1ed0b`。
- 保留数据卷的预览只应用 0096 并替换 App，最终镜像为 `sha256:45a45ca918529beeb0765ed5d91c1c79b0d913094d0cd4709dda61df201435c1`；运行态为迁移 `96/0096_saas_tenant_enable_approval_guard`、策略 `31/31 critical`、纯 Go standalone、PHP fallback 关闭和 707 条运行时路由。升级前 `0600` 快照大小 680393 字节、SHA-256 `34f1c68d9483f51741d5955d85fb5cae5320e70ad3a4c2d2af49795f8e647e0a`。
- Playwright 在 `1440x1000` 与 `390x844` 验收“提交启用审批”和 `tenant.enable` 治理行。页面无整体横向溢出，移动审批表仅在内部滚动；干净鉴权会话控制台 0 error/0 warning，70 个动态请求全部返回 `200`。临时 QA 租户和关联记录已清理，截图位于 `output/playwright/saas0096/`。
- 生产证据包已于 `2026-07-19 07:41:29 CST` 刷新，当前源码指纹匹配；严格证据检查和 skip-local 候选门禁按预期失败，唯一阻断是六类外部证据。发布准备仍为 `0/6`、`ready=false`，本轮没有启动 24 小时运行。
