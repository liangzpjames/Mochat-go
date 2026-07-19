# 01 Go 工程骨架与依赖规则详细设计

**状态：已确认**

**批准日期：2026-07-19**

## 1. 目标、范围与非目标

定义 `gitee.com/mochat/mochat/api-server-go` 的目录、包所有权、组合根、模块与插件注册及静态依赖规则。范围仅为工程底座；不创建 Go 工程，不定义 HTTP、存储或业务逻辑。

## 2. 上游依赖与下游使用者

上游为已确认的 Go 后端总体架构。下游为文档 02～07，以及未来 `api`、`worker`、`scheduler`、`migrate` 四个入口。

## 3. 关键决策及被否决方案

采用模块化单体、显式构造函数注入和编译期插件注册。否决运行时 Go Plugin（部署和类型安全风险）、服务定位器（隐藏依赖）及按技术层平铺全仓库（破坏领域所有权）。

## 4. 包、文件和组件职责

```text
api-server-go/
  cmd/{api,worker,scheduler,migrate}/
  internal/platform/{bootstrap,clock,errors,feature}/
  internal/modules/{iam,corp,organization,contact,room,content,campaign,chatops,analytics,platform}/
  internal/modules/<name>/{domain,application,transport,httpinfra}/
  internal/integrations/{wecom,storage,messaging}/
  internal/plugins/channelcode/
  api/openapi/  db/{migrations,queries}/  tests/{contract,integration,e2e}/
```

`domain` 不依赖 I/O；`application` 编排端口；`transport` 定义进程无关 DTO；`httpinfra` 适配 HTTP。禁止全局 `common`、`helper`、`utils` 包。旧 PHP 核心包和插件按职责映射至上述十个领域，插件仍归其宿主领域拥有。

## 5. 对外接口与完整 Go 类型签名

所有类型归 `internal/platform/bootstrap`，`FeatureGate` 归 `internal/platform/feature`：

```go
type Module interface { Name() string; Register(Registry) error }
type Registry interface {
    AddHTTPRoutes(RouteContributor) error
    AddWorkerHandlers(...WorkerHandler) error
    AddScheduledJobs(...ScheduledJob) error
    AddHealthChecks(...HealthCheck) error
}
type Plugin interface { Module; Version() string; Dependencies() []PluginDependency }
type PluginDependency struct { Name string; VersionConstraint string }
type FeatureGate interface { Enabled(context.Context, int64, string) (bool, error) }
```

`RouteContributor` 由 03、`WorkerHandler`/`ScheduledJob` 由 05、`HealthCheck` 由 02 拥有。模块间只允许 Application API、领域事件或专用只读接口。

## 6. 配置项、默认值和启动校验

本设计仅拥有 `MOCHAT_ENABLED_PLUGINS`（逗号分隔，默认空，非敏感，不热重载）。启动时校验名称唯一、依赖存在、版本约束满足且无循环。

## 7. 正常数据流与关键时序

入口加载配置后创建 Registry，按平台模块、业务模块、插件顺序注册；Registry 冻结后构造进程所需贡献项。注册失败即启动失败，不允许部分启用。

## 8. 事务、并发、幂等和一致性规则

Registry 只在单线程启动阶段写入，冻结后只读；模块名和贡献项键唯一。事务与消息一致性分别由 04、05 定义。

## 9. 错误分类、超时、重试与降级

重复注册、缺失依赖、循环依赖、版本不兼容均为不可重试启动错误。插件启用检查失败时拒绝该租户请求，不默认放行。

## 10. 安全、租户隔离和敏感数据处理

FeatureGate 必须显式接收 `tenantID`；插件不得绕过模块公开 API 或读取其他模块表。编译产物不含未登记插件。

## 11. 日志、指标与 Trace 要求

注册日志含 `module`、`plugin`、`version`、`duration_ms`、`error_code`，不含配置值；指标由 06 统一命名。

## 12. 测试矩阵及具体验收场景

架构检查通过：`contact/application -> contact/domain`、`contact/application -> organization/application/public`。检查失败：跨模块 infrastructure、直接 integrations 客户端、domain 导入 `net/http`、`database/sql`、Redis 或第三方 SDK。CI 错误格式：`ARCH001 <importer> must not import <target> (rule <id>)`。

## 13. 性能边界与容量假设

模块注册只发生一次；100 个模块和 500 个贡献项下启动注册应低于 100ms，运行期无反射扫描。

## 14. 发布、迁移、回退和兼容要求

新增模块先注册为关闭状态；插件版本变更必须满足依赖约束。回退为恢复上一构建产物和功能开关，不依赖运行时卸载。

## 15. 未解决问题

无未解决问题。
