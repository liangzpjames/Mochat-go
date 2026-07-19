# 07 工程质量、CI/CD 与本地环境详细设计

**状态：已确认**

**批准日期：2026-07-19**

## 1. 目标、范围与非目标

定义可重复的本地命令、测试基座、CI/CD 门禁、制品和前六份设计的证据追踪；不创建流水线或 Go 工程。

## 2. 上游依赖与下游使用者

输入为 01～06 全部契约；下游是 Go 底座实施计划、代码评审和发布审批。

## 3. 关键决策及被否决方案

工具版本入库、命令面统一、生成结果必须无差异、合并前硬门禁。否决依赖开发者全局工具、只在发布前测试及以整体覆盖率替代风险测试。

## 4. 包、文件和组件职责

`Makefile` 为命令入口，`tools/` 固定版本，`internal/testkit` 提供 fixture，`tests/{contract,integration,e2e}` 分层，CI 配置负责编排，构建目录只保存可追溯制品。

## 5. 对外接口与完整 Go 类型签名

```go
type TestEnvironment interface { MySQLDSN() string; RedisAddr() string; S3Endpoint() string; Close(context.Context) error }
type FixtureFactory interface { Tenant(context.Context, string) (int64, error); Reset(context.Context) error }
type ContractNormalizer interface { Normalize(status int, body []byte) (NormalizedResponse, error) }
type NormalizedResponse struct { Status int; JSON []byte; SideEffects []string }
```

## 6. 配置项、默认值和启动校验

工具清单固定 Go、SQLC、golang-migrate、staticcheck、OpenAPI diff、govulncheck、Syft、Grype 和 Cosign 精确版本及校验和。`MOCHAT_TESTCONTAINERS_REUSE=false`；`MOCHAT_TEST_TIMEOUT=10m`；测试 Secret 仅用短期随机值。版本或容器运行时缺失时 bootstrap 明确失败。

## 7. 正常数据流与关键时序

`make bootstrap` 校验工具 → `generate` → `lint` → 单元/集成/契约/race → build/image → SBOM/scan/sign。CI 的快速作业并行，集成与契约消费生成制品，发布只消费已签名镜像。

## 8. 事务、并发、幂等和一致性规则

每个集成测试独立 schema/租户；固定时钟和 ID；Migration 串行，测试可并行但不得共享可变 fixture。生成器连续两次输出一致；CI 重跑不得掩盖非确定失败。

## 9. 错误分类、超时、重试与降级

测试失败不自动重试；仅容器镜像拉取允许 2 次网络重试。lint/生成差异/安全严重级门禁不可跳过；外部扫描服务不可用时发布失败关闭。

## 10. 安全、租户隔离和敏感数据处理

PHP/Go 契约样本必须脱敏，禁止生产凭据和原始客户数据。CI 最小权限、OIDC 短期凭据、制品哈希和签名；Secret 扫描覆盖历史增量和构建上下文。

## 11. 日志、指标与 Trace 要求

CI 产出 JUnit、覆盖率、race、契约差异、migration、OpenAPI diff、SBOM、漏洞和镜像签名证据；失败日志含 job、命令、版本和可复现入口，不含 Secret。

## 12. 测试矩阵及具体验收场景

单元：领域/用例/重试/幂等；集成：MySQL 8、Redis、S3 Testcontainers、真实 SQL/Migration；契约：PHP/Go 规范化响应和副作用；E2E：启动/探针/停机与关键空业务链路；性能：启动、探针、Relay 背压。核心 domain/application 覆盖率不低于 80%。

## 13. 性能边界与容量假设

PR 快速门禁目标 10 分钟，完整门禁 30 分钟；单元测试 3 分钟，集成 15 分钟。超过目标需拆分分片，不允许删减门禁。

## 14. 发布、迁移、回退和兼容要求

命令定义：`bootstrap` 安装固定工具；`generate` 运行 SQLC/OpenAPI/mock；`lint` 格式/vet/staticcheck/架构；`test-unit|integration|contract|e2e|race` 分层；`test` 聚合；`build` 四入口；`image` 生成镜像/SBOM；`verify` 执行全部合并门禁。发布制品绑定 commit、工具链、SBOM、扫描和签名；回退使用上一已签名镜像。

## 15. 未解决问题

无未解决问题。

### CI 作业与追踪矩阵

| 要求所有者 | 命令/套件 | CI 作业 | 证据 | 负责人 |
|---|---|---|---|---|
| 01 包边界 | `make lint` | `architecture` | 导入违规报告 | 平台负责人 |
| 02 生命周期 | `make test-integration` | `runtime` | 探针/停机 JUnit | 平台负责人 |
| 03 HTTP 契约 | `make test-contract` | `contract` | PHP/Go diff | API 负责人 |
| 04 数据/迁移 | `make test-integration` | `database` | 空库/基线报告 | 数据负责人 |
| 05 异步 | `make test-race` | `async-race` | race/故障注入报告 | 消息负责人 |
| 06 安全/观测 | `make verify` | `security-telemetry` | 泄密测试、SBOM、扫描 | 安全负责人 |
| 07 构建发布 | `make image` | `release-artifact` | 签名镜像清单 | 发布负责人 |
