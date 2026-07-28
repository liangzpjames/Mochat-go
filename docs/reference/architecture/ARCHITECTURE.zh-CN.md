# MoChat Go 架构与新增需求开发指南

## 当前形态

项目采用渐进式模块化单体：一个 Go module、同一套业务数据和共享基础设施，通过模块边界控制复杂度。近期不拆微服务。

运行角色由 `MOCHAT_GO_RUNTIME_ROLE` 控制：

| 角色 | HTTP | Redis consumer | Cron |
| --- | --- | --- | --- |
| `all` | 是 | 是 | 是 |
| `api` | 是 | 否 | 否 |
| `worker` | 否 | 是 | 否 |
| `scheduler` | 否 | 否 | 是 |

本地默认 `all`；未来生产部署应拆成独立进程角色。

## 新模块边界

新业务放在 `internal/modules/<domain>/`，依赖方向如下：

```text
transport/http ─┐
                ├─> application ─> domain
adapters ───────┘         │
                          └─> ports <─ adapters
```

- `domain`：纯业务规则，不依赖数据库、HTTP、旧 `dashboard/store` 或外部 SDK。
- `application`：事务和用例编排，通过 ports 使用外部能力。
- `ports`：repository、企微、支付、对象存储、LLM 等契约。
- `adapters`：MySQL、Redis、HTTP provider 等实现。
- `transport/http`：协议转换、鉴权和响应映射，不放业务规则。

## 现有代码迁移

不一次性重写巨型文件。修改已有功能时：

1. 先写能复现原行为或新需求的失败测试；
2. 把本次涉及的规则迁到 domain/application；
3. 定义最小 repository/provider port；
4. 让旧 `MySQLStore` 或外部客户端暂时实现 port；
5. 旧 handler 只负责调用用例并保持原路由契约；
6. 跑单元、全包和相应 smoke；
7. 更新本目录中的需求、风险和验收记录。

## 强制门禁

`scripts/audit_architecture_boundaries.sh`：

- 阻止 domain 依赖 `cmd`、`dashboard`、`store`、`server`、`config`、`frontend`；
- 阻止三个巨型业务文件和主入口超过已记录字节基线。

确需调整基线时，必须先在 `RISK_REGISTER.zh-CN.md` 登记原因、负责人、回收动作和截止日期，再由评审单独更新基线。

## 新需求完成定义

- 明确角色、场景、边界、指标、权限、租户、套餐、审计与异常；
- API 与 migration 先定义，数据库变更有 up/down 和兼容测试；
- 外部 API 有 adapter、超时、重试、幂等和 fake；
- 所有业务读写带租户范围；
- 资源创建与删除同步更新用量/存储账本；
- 高风险动作写审计，必要时进入双人审批；
- 失败测试先于实现；
- `go test`、架构门禁和相关 smoke 通过；
- 文档和台账同步更新。
