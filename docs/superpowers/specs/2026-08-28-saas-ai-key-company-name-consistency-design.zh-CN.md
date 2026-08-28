# SaaS AI Key 保存与客户公司名称一致性设计

## 背景与根因

本次修复覆盖两个独立但同属 SaaS 客户治理页的问题。

1. 租户 AI Provider 弹窗在任意保存失败后都会执行 `apiKey: ''`。版本冲突、网络失败、服务端拒绝和前端日期校验都会丢弃尚未成功持久化的 Key，用户无法在原表单上修正并重试。错误文案还固定声称“API Key 已从页面清除”，掩盖了服务端返回的可操作原因。
2. SaaS 客户列表只读取 `mc_tenant.name`，Dashboard 唯一企业资料读取权威绑定公司的 `mc_corp.name`。运行态租户 1 的两个值分别为 `MoChat Test Enterprise` 与 `蓝鲸数字科技（上海）有限公司`，因此仅修改默认开户名称或批量覆盖数据库都不能解决真实公司改名后的长期一致性。

## 方案比较

### 方案 A：SaaS 展示绑定公司名，租户名保留为身份别名（采用）

- SaaS Overview API 同时返回 `tenantName` 和 `companyName`。
- `companyName` 来自 `mochat_go_tenant_corp_bindings -> mc_corp` 的权威唯一企业；不存在有效绑定时回退到 `tenantName`。
- SaaS 客户页以 `companyName` 作为客户主名称；两者不同时在辅助信息中保留 SaaS 租户别名和租户 ID。
- 新开户默认 Fake 企业名称直接等于清理空白后的客户名，不再追加“演示企业”。

优点是不会覆盖真实企微企业名称，Dashboard 修改公司资料后 SaaS 下次查询自动一致，同时保留租户身份和历史审计语义。

### 方案 B：把 `mc_corp.name` 强制同步为 `mc_tenant.name`（不采用）

该方案会把已经验证的真实公司名覆盖成平台内部租户别名。运行态租户 1 已证明这种覆盖会破坏真实业务数据。

### 方案 C：Dashboard 修改公司名时反向修改 `mc_tenant.name`（不采用）

该方案把企业资料权限扩大为 SaaS 租户身份修改权限，并会影响账单、审批、审计和激活交付语义，耦合范围过大。

## API Key 交互设计

Key 只存在于当前 React 表单状态和请求体中，不写入 React Query 查询缓存，也不作为 Mutation variables 保存。

- 保存成功：关闭弹窗并清空 Key。
- 用户取消、Esc 或关闭弹窗：清空 Key。
- 切换 Provider：清空 Key，防止把一个厂商的 Key 误用于另一个厂商。
- 前端校验、网络错误、4xx/5xx 保存失败：保留 Key，保留其他输入，展示可操作错误后允许原地重试。
- 409 版本冲突：刷新服务端版本和公开配置；本地 Key 保留，错误提示明确要求重新确认。
- 错误文案：400/409 使用服务端已经过接口边界处理的消息；5xx 保留通用失败文案，不展示内部堆栈或凭证信息。

关闭弹窗后必须无法从 DOM、Query cache 或 Mutation cache 找到 Key。

## 公司名称数据流

`SaaSAdminOverview` 查询继续以 `mc_tenant` 为租户、套餐和权限主体，同时左连接权威唯一企业绑定：

```text
mc_tenant.id
  -> mochat_go_tenant_corp_bindings.tenant_id
  -> mc_corp.(tenant_id,id)
  -> companyName
```

服务端关键字查询同时匹配租户名和公司名。API 返回：

```json
{
  "tenantId": 1,
  "tenantName": "MoChat Test Enterprise",
  "companyName": "蓝鲸数字科技（上海）有限公司"
}
```

前端使用 `companyName || tenantName` 作为客户主名称。租户别名不参与覆盖或反向写入。

## 失败处理与兼容

- 老数据库若暂时没有有效绑定，`companyName` 回退到 `tenantName`，SaaS 页面仍可使用。
- 绑定公司被软删除时不得作为展示名称。
- 不新增迁移，不批量修改既有真实企业数据。
- `tenantName` 字段保持不变，现有消费者无需改造。

## 测试与验收

1. TDD RED：SaaS 前端测试证明保存失败后 Key 仍在输入框，旧实现应失败。
2. TDD GREEN：成功、关闭、Esc、切换厂商仍清空；失败可重试；Query/Mutation cache 不含明文 Key。
3. 后端契约测试：Overview payload 同时输出 `tenantName` 与 `companyName`，无公司名时正确回退。
4. 存储测试：新开户默认企业名等于清理后的租户名；SQL 查询使用权威绑定并支持公司名关键字。
5. 前端测试：列表和详情主标题显示 `companyName`，租户别名只作为辅助信息。
6. 完整门禁：相关 Go 测试、SaaS Admin 全量测试、typecheck、lint、build、`go test ./...`、`git diff --check`。
7. Docker 与浏览器：仅重建并重建 `mochat-go-desktop-app-1`，保留 MySQL/Redis/存储卷；验证 Overview API 的公司名、SaaS 页面客户名、Dashboard 公司名三者一致，并用不记录真实 Key 的失败请求验证 Key 可重试。

## 非目标

- 不测试真实模型连接，不触发 AI 分析。
- 不改变租户身份、账单、审批或审计中的 `tenantName`。
- 不覆盖已验证的真实企微公司名称。
- 不删除或重建 Docker 数据卷。
