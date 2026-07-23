# 领域模块开发约定

新增业务必须优先放入 `internal/modules/<domain>/`，不得继续向
`internal/dashboard/saas_admin_page.go`、`internal/dashboard/saas_admin.go`
或 `internal/store/mysql.go` 无边界追加逻辑。

标准目录：

```text
<domain>/
├── domain/          纯领域规则，不依赖数据库、HTTP 或旧业务包
├── application/     用例编排，只依赖 domain 与 ports
├── ports/           repository 和外部服务接口
├── adapters/        MySQL、Redis、企微、支付等接口实现
└── transport/http/  请求解析、鉴权和响应映射
```

依赖方向为 `transport/adapters -> application -> domain`，application
通过 ports 使用外部能力。`example` 只演示边界，不注册生产路由。

迁移现有业务时采用“触碰即迁移”：先写原行为回归测试，再把本次涉及的
用例迁到 application，把持久化契约放入 ports，旧 handler 作为 transport
adapter 调用新用例。禁止一次性复制整套业务形成双份实现。
