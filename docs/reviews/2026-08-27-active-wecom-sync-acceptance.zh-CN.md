# 企微主动同步实施与验收报告

日期：2026-08-27
验收数据集：`MOCHAT-LOCAL-ACCEPTANCE-20260827`（本地验收、非生产）

## 实施结果

- 唯一企业资料页新增统一“数据同步”区域，提供“立即同步人员”和“立即同步会话”，自建应用与第三方代开发模式均可看到同步状态；第三方模式不展示自建应用配置。
- 当前正式可访问的“员工账号与权限”页新增“立即同步人员”，提交前二次确认；未完成企微授权时保留空列表，并显示业务提示，不把页面变成加载失败。
- 全局消息页新增“同步最新会话”；缺少会话存档授权时按钮关闭并说明原因，消息读取保持正常空态。
- 会话同步通过 0138 durable source、cursor、lease、run ledger 和幂等键入队，由现有 all-in-one 应用内 worker 执行；HTTP 请求路径不直接调用 bridge。
- 新增状态接口只返回排队、同步中、完成、失败、数量和时间，不向操作员暴露 source、cursor、lease、幂等键、原始错误码或凭据。
- 新增 0168 权限迁移，将会话同步 API 分别绑定到唯一企业资料和全局消息页面权限；本地验收增量迁移清单同步升级到 0168。

## 自动化验证

- `go test ./... -count=1`：通过。
- 四前端 `lint`、`typecheck`、`test`、`build`：通过；Dashboard 为 143 个测试文件、946 项测试全部通过。
- Phase 4 Dashboard RBAC：通过，53 个页面、49 个普通页面、4 个超级管理员页面、0 个未映射 API；completion gate 为 53/49/4，`scopeRequired=145`。
- Provider completion：通过。
- Dashboard 页面证据合同：12/12 通过。
- 圆弧/Yuanhu benchmark：53 个页面通过。
- 企微归档/SaaS 激活静态合同：11/11 通过。
- `git diff --check`：通过，仅有 Windows 工作区换行提示，无空白错误。
- 未设置 `MOCHAT_GO_MYSQL_INTEGRATION_DSN`，隔离 MariaDB integration gate 明确为 **SKIP**，未记为 PASS；本次另在本地 Docker MariaDB 中完成 0168 实际升级和业务验收。

## Docker 与正式链路验收

- Compose project：`mochat-wecom-acceptance-20260827`。
- 应用：`http://127.0.0.1:19080`，健康。
- Finance SDK 边界 bridge：`http://127.0.0.1:19091`，健康，仅供本地去敏契约夹具使用。
- MariaDB：`127.0.0.1:19016`；Redis：`127.0.0.1:29089`。
- 数据库已记录 `0168_company_archive_sync_rbac`，checksum 长度 64。
- 使用 Dashboard 本地验收账号经正式登录 API 提交手动会话同步：状态由 `queued` 变为 `completed`；重启应用后 durable checkpoint 恢复为 `ready`。
- SDK 契约结果：10 条消息、7 个媒体对象、5 个 ready、1 个 missing、1 个 corrupt；文本、图片、语音、视频、文件等正式解析/入库/回读链路通过，缺失和损坏媒体鉴权读取均返回 404。

## 浏览器验收

- 唯一企业资料页：第三方代开发租户只显示企业绑定、数据同步和审计，不显示自建应用配置；人员/会话未授权原因以业务语言显示，页面无控制台错误。
- 员工账号与权限页：实际点击“立即同步人员”并检查确认框；当前未授权租户提交后失败关闭为“请先完成企业微信授权”，员工列表保持 0 条，不出现整页加载失败。
- 全局消息页：存在“同步最新会话”；未授权租户按钮关闭，最终显示“会话归档未开通”空态，无控制台错误。
- 刷新与应用重启后页面、登录态、同步入口和空态恢复正常。
- 390×844：页面 body 无横向溢出；员工表格在独立滚动容器内横向滚动，主菜单切换为移动端入口。

## 真实边界与清理

- 结论仅为“本地 SDK/媒体合同与生产代码路径通过”，不代表真实企业微信线上授权、网络、IP 白名单或 Finance SDK 服务已验收。
- 人员主动同步在未配置真实企业微信凭据的租户上只验证了按钮、确认、鉴权和失败关闭；真实人员数据仍需有权租户联调。
- 本地账号密码保存在被忽略的 `.tmp-wecom-acceptance-runtime/` 文件中，不写入文档、日志或 Git。
- 停止但保留数据：`docker compose -p mochat-wecom-acceptance-20260827 --env-file .tmp-wecom-acceptance-runtime/.env.local -f deploy/local-acceptance/docker-compose.yml stop`。
- 清理验收业务夹具：`powershell -ExecutionPolicy Bypass -File scripts/run_wecom_archive_saas_activation_acceptance.ps1 -Action cleanup -NoBuild`。
- 完整删除验收容器与命名卷：`docker compose -p mochat-wecom-acceptance-20260827 --env-file .tmp-wecom-acceptance-runtime/.env.local -f deploy/local-acceptance/docker-compose.yml down -v`。
