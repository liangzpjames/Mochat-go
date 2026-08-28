# SaaS AI Key 与公司名称一致性验收记录

日期：2026-08-28
分支：`fix/saas-ai-key-company-name-20260828`
基线：`1d8e349`
实现提交：`95930d1`、`66d8c39`、`94a0e6e`、`b48e34d`、`296645e`、`c991179`、`2670970`、`11b5c4c`、`72c8b2f`

## 1. 验收结论

本次两项问题均已修复并完成运行态验收：

1. SaaS 租户 AI 配置保存失败时，页面保留本次输入的 API Key，管理员可修正其他字段后直接重试；仅在保存成功、取消、关闭、按 Escape 或切换 Provider 时清空。
2. SaaS 客户列表、客户详情及配置弹窗以租户绑定的 Dashboard 企业名称为主名称；`tenantName` 继续作为 SaaS 租户身份别名，不覆盖已有企业资料。

## 2. 根因与修复

### 2.1 API Key 被提前清空

原页面在 mutation 失败回调中无条件把 `apiKey` 置空，409 刷新配置时也会以服务端脱敏结果重建表单，因此一次普通保存错误就丢失管理员刚输入的密钥。

修复后失败路径不再清空 Key；409 会绕过默认缓存刷新原租户的非敏感配置和版本，但仅在同一租户、同一编辑会话内合并当前 Key 草稿。迟到的成功或 409 响应不会影响后来打开的配置窗口。安全的 4xx 校验错误会显示可操作原因，5xx 错误在进入 React Query MutationCache 前即被替换为通用错误。Key 不进入 Query cache、mutation variables、MutationCache 错误、日志或截图正文。

### 2.2 SaaS 名称与 Dashboard 名称混用

原 SaaS 接口只返回 `mc_tenant.name`，前端把它当作客户公司名；Dashboard 则读取绑定的 `mc_corp.name`，两者语义不同。

修复后服务端通过 `mochat_go_tenant_corp_bindings` 唯一绑定关联有效的 `mc_corp`，新增独立 `companyName`，缺少有效名称时才回退 `tenantName`。SaaS 以 `companyName` 为主展示，并在名称不同时附带显示 SaaS 租户别名。新开户默认企业名称使用清理后的租户名，不再自动追加“演示企业”；没有批量改写任何已有企业名称。

## 3. 自动化门禁

在隔离 worktree 中新鲜执行：

```powershell
go test ./... -count=1
corepack pnpm --filter @mochat/saas-admin test
corepack pnpm --filter @mochat/saas-admin lint
corepack pnpm --filter @mochat/saas-admin typecheck
corepack pnpm --filter @mochat/saas-admin build
git diff --check
```

结果：

- Go 全量测试通过，所有有测试的包均为 `ok`。
- SaaS Admin：6 个测试文件、57 个测试全部通过。
- lint、typecheck、Vite production build 均退出码 0。
- SaaS Admin production build：1903 modules transformed。
- `git diff --check` 退出码 0。

针对性 TDD 还覆盖了：500 错误保留并重试同一 Key、5xx 原始错误在 MutationCache 前完成脱敏、409 强制刷新并保留 Key、迟到响应的编辑会话隔离、成功/关闭/Escape/Provider 切换清空、SQL 唯一绑定连接及软删过滤、公司名回退、公司名关键字查询参数与前端主次名称展示。

## 4. Docker 安全部署

没有执行 `down`、`--volumes` 或数据库迁移。仅执行：

```powershell
docker compose ... build app
docker compose ... up -d --no-deps --force-recreate app
```

部署证据：

- `app`：`2b8077cbac67` → `830f73f6458e`，最终镜像 `sha256:2138ae5378e83022e396740b4172c9017dcb4f64081b7c8b12b81a6cc04291ec`。
- `mysql` 保持 `a42016b45ef0`，`redis` 保持 `916d012e20b3`，`archive-bridge` 保持 `dcffb939293a`。
- `app`、`mysql`、`redis`、`archive-bridge` 均为 healthy。
- `/readyz` 返回 HTTP 200。
- `mochat-go-desktop_app-storage` 与 `mochat-go-desktop_audit-anchor-storage` 仍挂载到新 `app`；数据库和 Redis 容器未替换，数据卷未删除。

## 5. 数据与浏览器验收

### 5.1 权威名称一致性

只读数据库回查租户 1：

- SaaS 租户身份名：`MoChat Test Enterprise`。
- 绑定企业 ID：`1`。
- Dashboard 权威公司名：`蓝鲸数字科技（上海）有限公司`。
- CorpID：`wwSIM00000000000001`。

真实浏览器中：

- SaaS 客户列表主名称显示 `蓝鲸数字科技（上海）有限公司`，次级身份显示 `SaaS 租户：MoChat Test Enterprise · 租户 ID 1`。
- SaaS 客户详情标题及 AI 配置弹窗均显示权威公司名。
- Dashboard `/company-setting/website` 顶栏、唯一企业绑定卡片和展示名称输入框均显示同一公司名。

### 5.2 API Key 失败保留与关闭清理

真实浏览器中输入测试占位 Key，使用 `http://127.0.0.1` 触发服务端 HTTPS/SSRF 校验失败。页面显示服务端返回的安全、可操作错误 `baseUrl must be an HTTPS URL without userinfo, query, or fragment` 后：

- API Key 密码框仍显示掩码，接口地址和模型名称也保持失败时的输入，可直接修正重试。
- 取消后重新打开配置弹窗，密码框恢复为空，说明失败保留与主动离开清理边界同时生效。
- 测试占位 Key 未写入验收文档、日志或截图正文。

最终容器日志扫描未发现测试占位 Key 或 panic 文本。

失败请求后的数据库只读回查仍为：`deepseek`、`https://api.deepseek.com`、`deepseek-chat`、Key 尾号 `5fee`、版本 `1`、状态 `active`、更新时间 `2026-08-25 13:30:06`。这证明验收错误没有覆盖现有配置或产生数据污染。

## 6. 范围边界

- 本次没有调用真实模型 Provider；保存接口本身按产品合同只更新加密配置，不执行模型连通性测试。
- 没有修改已有 `mc_corp.name`；名称一致性通过权威绑定读取和展示语义修复实现。
- 没有触碰主工作区中的用户未提交修改。

## 7. 最终复审

独立代码复审最初发现编辑会话竞态、409 缓存刷新和 5xx 错误缓存脱敏边界；均以独立提交修复并补充回归测试。最终复审结论为 `APPROVED`，没有 Critical 或 Important 级问题。
