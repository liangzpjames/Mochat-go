# Phase 2.2 后端模块化与质量门禁设计

## 1. 背景

Phase 2.1 已完成前端功能迁移验收，Phase 3 将开始建设 SCRM 客户生命周期、营销闭环、报表、会话存档、风险质检和 AI 能力。现有 Go 后端具备可运行、可测试和可部署的工程基础，但生产业务仍主要集中在以下遗留结构：

- `internal/dashboard` 同时承载 HTTP、应用编排、业务规则和部分共享类型；
- `internal/store` 大量依赖 `internal/dashboard` 中的类型与接口；
- `internal/server/server.go` 逐 handler 保存字段、Option 和路由；
- `cmd/mochat-go/main.go` 集中装配全部基础设施、handler、worker 和 scheduler；
- `internal/modules` 只有示例模块，尚未承载生产业务。

项目已经约定新业务进入 `internal/modules/<domain>`，但当前 Phase 3 计划仍准备向 `dashboard` 和 `store` 添加 SCRM 文件。如果直接开工，Phase 3 会继续扩大遗留集中包，并使后续营销、报表、风险和 AI 模块共享同一变更热点。

Phase 2.2 不全面重写旧系统。它通过架构门禁、模块级装配机制和一个受控的 SCRM 纵向薄切片，建立 Phase 3 必须遵守的后端开发路径。

## 2. 阶段目标

Phase 2.2 必须交付以下结果：

1. 建立 Windows 和 Linux 都能执行的 Go 架构门禁；
2. 建立正式生产模块的目录、依赖和装配契约；
3. 让新模块自行注册路由，不再为每个新 handler 扩展 `server.Server`；
4. 用 SCRM“线索创建与查询”薄切片验证 domain、application、ports、MySQL adapter、HTTP transport 和模块装配；
5. 验证租户隔离、创建幂等、分页查询和 migration 生命周期；
6. 冻结四个遗留巨型文件的规模基线，禁止 Phase 3 新业务继续进入遗留集中包；
7. 修订 Phase 3 计划，使后续 SCRM 功能在 Phase 2.2 建立的模块中演进。

完成 Phase 2.2 不代表旧后端已经完成全面模块化，而是代表新增功能有一条被自动门禁保护、可独立测试和扩展的标准路径。

## 3. 非目标

Phase 2.2 不包含：

- 全面拆分 `internal/dashboard`、`internal/store`、`internal/server` 或 `cmd/mochat-go/main.go`；
- 迁移现有 API 或修改既有 URL、JSON、鉴权和运行行为；
- 实现正式 Phase 3 UI；
- 实现线索转化、联系人、负责人、协作人、商机或跟进；
- 引入 Kafka、NATS、微服务或新的 Web 框架；
- 实现报表、风险、会话存档或 AI；
- 对外承诺 SCRM 薄切片的产品 API。

## 4. 设计原则

### 4.1 模块化单体

继续使用单一 Go module、共享数据库和现有部署模型。业务边界通过包结构、接口、装配入口和自动化门禁维持，不以拆微服务代替模块设计。

### 4.2 依赖向内

生产模块采用以下依赖方向：

```text
transport/http ─┐
                ├─> application ─> domain
adapters/mysql ─┘         │
                          └─> ports <─ adapters
```

- `domain` 只包含领域实体、值对象、状态转换和领域错误；
- `application` 编排用例，通过 ports 使用持久化、时间和 ID 能力；
- `ports` 定义应用需要的外部能力，不包含 SQL、HTTP 或 provider 类型；
- `adapters` 实现 ports，负责 SQL、Redis 或外部 API 细节；
- `transport/http` 负责鉴权上下文转换、请求解析、错误映射和响应；
- `module.go` 是模块对 composition root 的唯一生产装配入口。

### 4.3 默认关闭

SCRM 薄切片默认不注册产品路由。只有显式配置或测试装配才能启用，避免 Phase 2.2 提前发布未完成的 Phase 3 能力。

### 4.4 遗留系统旁路

Phase 2.2 为新模块建立旁路，不要求先重写旧功能。旧功能继续通过现有 handler Options 运行；新模块通过模块级 registrar 注册。后续采用“触碰即迁移”，禁止新增双份业务实现。

## 5. 目标目录

```text
internal/
├── architecture/
│   ├── rules.go
│   ├── audit.go
│   ├── audit_test.go
│   └── testdata/
├── app/
│   └── modules/
│       ├── registry.go
│       └── registry_test.go
└── modules/
    └── scrm/
        ├── module.go
        ├── module_test.go
        ├── domain/
        │   ├── lead.go
        │   ├── errors.go
        │   └── lead_test.go
        ├── application/
        │   ├── service.go
        │   └── service_test.go
        ├── ports/
        │   ├── lead_repository.go
        │   ├── clock.go
        │   └── id_generator.go
        ├── adapters/
        │   └── mysql/
        │       ├── lead_repository.go
        │       └── lead_repository_integration_test.go
        └── transport/
            └── http/
                ├── routes.go
                ├── lead_handler.go
                └── lead_handler_test.go
```

迁移文件：

```text
deploy/standalone/migrations/
├── 0090_scrm_lead_foundation.up.sql
└── 0090_scrm_lead_foundation.down.sql
```

序号在实施时必须先检查主分支最新 migration；如果 `0090` 已被占用，应使用下一个连续序号，并同步修订 Phase 3 计划。

## 6. 模块装配契约

### 6.1 路由注册抽象

新增最小路由注册接口，避免模块依赖具体 `server.Server`：

```go
type RouteRegistrar interface {
    Handle(pattern string, handler http.Handler)
}
```

如果现有路由器需要方法或元数据，注册接口可以扩展为项目自有的 `Route` 描述，但不得暴露 `server.Server` 字段或逐 handler Option。

### 6.2 模块注册

composition root 使用模块级注册对象：

```go
type HTTPModule interface {
    RegisterRoutes(RouteRegistrar)
}

type Registry struct {
    modules []HTTPModule
}

func (r *Registry) RegisterRoutes(registrar RouteRegistrar)
```

`cmd/mochat-go` 只负责创建启用的模块并交给 registry。新增第二个 SCRM handler 时，不得修改 `server.Server` 的字段、构造 Option 或启动流程。

### 6.3 SCRM 模块入口

```go
type Dependencies struct {
    DB          *sql.DB
    Clock       ports.Clock
    IDGenerator ports.IDGenerator
}

func New(deps Dependencies) (*Module, error)
func (m *Module) RegisterRoutes(registrar appmodules.RouteRegistrar)
```

`Dependencies` 必须显式列出依赖。模块不得从全局变量、环境变量或 legacy store locator 获取能力。配置解析仍由 composition root 负责。

## 7. SCRM 线索薄切片

### 7.1 领域模型

`Lead` 至少包含：

- `ID`：服务端生成的稳定标识；
- `TenantID`：非零租户标识；
- `BusinessKey`：租户内稳定幂等键；
- `Name`：去除首尾空白后非空；
- `Source`：受控来源值；
- `Status`：Phase 2.2 固定为 `new`；
- `Version`：初始值为 `1`；
- `CreatedAt`、`UpdatedAt`：由注入的 Clock 生成。

领域层负责不变量，不负责读取鉴权 header、执行 SQL 或生成 HTTP 错误码。

### 7.2 应用用例

只提供两个用例：

```go
CreateLead(ctx context.Context, cmd CreateLeadCommand) (LeadView, error)
ListLeads(ctx context.Context, query ListLeadsQuery) (LeadPage, error)
```

约束：

- `TenantID` 必须来自服务端认证后的上下文转换，不信任 JSON 或 query 中的 tenant 参数；
- 相同租户和 `BusinessKey` 的重复创建返回同一资源；
- 不同租户可以使用相同 `BusinessKey`；
- 查询必须显式携带 `TenantID`；
- 分页使用明确的最大 page size；
- application 错误保持协议无关，由 transport 映射为 HTTP 状态。

### 7.3 HTTP 表面

测试路由使用版本化路径：

```text
POST /api/phase2-2/scrm/leads
GET  /api/phase2-2/scrm/leads
```

这些路由默认关闭，不加入正式 Phase 3 API 契约。transport 必须复用现有认证/租户上下文能力，但通过窄接口接入，不得导入 `internal/dashboard`。

### 7.4 持久化

表必须包含：

- 显式 `tenant_id`；
- 租户内唯一 `(tenant_id, business_key)`；
- `version`；
- 创建、更新时间；
- 面向租户分页的组合索引；
- 与当前项目数据库版本兼容的字段类型和索引长度。

幂等由数据库唯一约束兜底，不能仅依赖“先查再插”。adapter 必须把重复键映射为应用可理解的已存在结果，同时不泄漏 MySQL 错误类型。

## 8. 架构质量门禁

### 8.1 实现形式

新增 Go 实现的架构审计，核心规则通过 `go test ./internal/architecture/...` 执行。审计读取 Go AST/import graph 和版本化策略文件，不依赖 Bash、grep 或平台特定路径行为。

现有 shell 审计在过渡期保留，Go 门禁成为 Windows 与 Linux 的共同权威。两者规则重叠时，以 Go 门禁为准；完成迁移后再决定是否删除 shell 包装。

### 8.2 包依赖规则

门禁必须验证：

| 位置 | 允许依赖 | 禁止依赖 |
| --- | --- | --- |
| `modules/*/domain` | 标准库、本模块 domain | SQL、HTTP、config、dashboard、store、server、adapter、transport |
| `modules/*/ports` | 标准库、本模块 domain | adapter、transport、dashboard、store、server |
| `modules/*/application` | 标准库、本模块 domain/ports | SQL、HTTP、adapter、transport、dashboard、store、server |
| `modules/*/adapters` | 本模块 domain/ports，基础设施库 | dashboard、全局 store、其他模块 adapter/transport |
| `modules/*/transport` | 本模块 application/domain，公共认证契约 | dashboard、全局 store、其他模块 adapter |
| `modules/*/module.go` | 本模块各层、公共模块注册契约 | dashboard、全局 store |

模块间如需同步协作，必须依赖版本化的公共契约或消费方定义的 port，不得导入另一个模块的 adapters、transport 或私有实现。

### 8.3 遗留增长规则

以下文件保持现有字节上限，不允许提高基线：

- `internal/dashboard/saas_admin_page.go`
- `internal/dashboard/saas_admin.go`
- `internal/store/mysql.go`
- `cmd/mochat-go/main.go`

Phase 2.2 可以为一次模块注册在 `main.go` 增加最小装配代码，但必须通过抽取等量或更多 composition 逻辑抵消增长。禁止通过修改 baseline 掩盖失败。

门禁同时禁止新增以下 Phase 3 命名文件：

```text
internal/dashboard/scrm*.go
internal/store/scrm*.go
```

更通用的约束由 PR 模板和模块清单共同执行：新增业务域必须登记到生产模块清单。

### 8.4 例外策略

架构例外必须进入版本化 allowlist，每项包含：

- 精确规则 ID；
- 精确文件或 import；
- 业务原因；
- 负责人；
- 创建日期；
- 到期日期；
- 清理目标。

缺少字段、到期或匹配范围扩大时门禁失败。Phase 2.2 不为 SCRM 创建任何遗留依赖例外。

### 8.5 负向验证

测试 fixture 必须证明以下违规能够被检测：

- domain 导入 `database/sql`；
- application 导入 adapter；
- SCRM 导入 `internal/dashboard`；
- 一个模块导入另一个模块的 MySQL adapter；
- 新增 `internal/store/scrm.go`；
- allowlist 例外过期；
- 受保护巨型文件超过基线。

## 9. 测试策略

### 9.1 快速反馈层

每次本地检查和 PR 必须执行：

```bash
go test ./internal/architecture/... ./internal/app/modules/... ./internal/modules/...
go vet ./...
```

domain 和 application 测试不得启动数据库、Redis 或 HTTP server。

### 9.2 全量回归层

```bash
go test ./...
go vet ./...
```

必须保持现有测试全部通过，且 SCRM 模块关闭时遗留路由行为不变。

### 9.3 Race 层

```bash
go test -race ./internal/modules/...
```

Linux CI 必须执行。若 Windows runner 对 race 有工具链限制，Windows 仍执行非 race 单元测试，不能因此跳过 Linux race 门禁。

### 9.4 MySQL 集成层

MySQL adapter 测试使用明确 build tag 或独立测试命令，覆盖：

- 租户 A 无法读取租户 B 的线索；
- 相同租户和业务键重复创建只产生一条记录；
- 不同租户可使用相同业务键；
- 分页稳定；
- 并发重复创建不产生重复记录。

测试必须使用隔离数据库和确定性 fixture，不依赖开发者已有数据。

### 9.5 Migration 生命周期

必须验证：

```text
apply -> checksum -> rollback -> replay -> checksum
```

up/down 必须都存在，rollback 后不得遗留表、索引或约束。

## 10. CI 与开发流程

Phase 2.2 修改现有开发检查和 CI，使门禁至少分为：

1. `architecture`：跨平台架构规则及负向 fixture；
2. `unit`：全量 Go test 和 vet；
3. `race`：新生产模块 race test；
4. `mysql-integration`：adapter 与 migration 生命周期；
5. `legacy-compatibility`：现有 smoke/回归门禁。

PR 模板增加以下声明：

- 所属业务模块；
- 新增或变更的用例；
- 租户边界；
- 幂等策略；
- 并发控制；
- migration 生命周期；
- 外部依赖 fake；
- 执行过的验证命令；
- 架构例外及到期日期，没有则明确写“无”。

Phase 3 新功能 PR 若不属于 `internal/modules/<domain>`，必须先获得架构例外；“暂时放在 dashboard/store”不是可接受原因。

## 11. Phase 3 衔接

Phase 2.2 完成后，修订 Phase 3 实施计划：

- `internal/store/scrm.go` 改为 `internal/modules/scrm/adapters/mysql/...`；
- `internal/dashboard/scrm.go` 改为 `internal/modules/scrm/transport/http/...`；
- 业务规则进入 `domain` 和 `application`；
- 仓储接口进入 `ports`；
- 报表消费稳定领域事件或查询 port，不直接读取 SCRM adapter 私有实现；
- 后续联系人、分配、商机和跟进在同一 SCRM 模块中按子域拆分；
- 每个任务先运行架构门禁，再执行单元、集成和端到端验证。

SCRM 薄切片的数据表可以由 Phase 3 延续，但 Phase 3 必须先审查正式 API 和领域契约；Phase 2.2 的测试路由不自动成为产品兼容承诺。

## 12. 发布与回滚

Phase 2.2 不默认暴露新产品能力，因此发布风险主要来自装配和 migration：

- 模块功能开关默认关闭；
- migration 只新增隔离表，不修改旧表；
- 禁用模块不影响遗留 handler；
- 回滚应用版本前可先关闭模块；
- migration down 只在确认无 Phase 3 正式数据时执行；
- CI 或架构门禁异常可回滚检查器提交，不允许通过放宽规则直接恢复绿色。

## 13. 验收标准

全部满足后 Phase 2.2 才能完成：

- [ ] `internal/modules/scrm` 不导入 `internal/dashboard`、`internal/store` 或 `internal/server`；
- [ ] SCRM domain/application 测试不依赖数据库和 HTTP；
- [ ] 创建与查询线索的完整薄切片通过单元和 MySQL 集成测试；
- [ ] 租户隔离、数据库级幂等和并发重复创建测试通过；
- [ ] 新增第二个 SCRM handler 不需要修改 `server.Server` 字段或增加逐 handler Option；
- [ ] 模块关闭时全量既有测试通过；
- [ ] Windows 和 Linux 都能运行 Go 架构门禁；
- [ ] 所有负向架构 fixture 都能稳定触发预期失败；
- [ ] 遗留巨型文件基线没有提高；
- [ ] migration apply/checksum/rollback/replay 通过；
- [ ] `go test ./...` 和 `go vet ./...` 通过；
- [ ] Linux `go test -race ./internal/modules/...` 通过；
- [ ] Phase 3 实施计划已切换到模块化目录和装配方式；
- [ ] Phase 2.2 验收记录列出命令、环境、结果和剩余债务。

## 14. 完成定义

Phase 2.2 的完成定义不是“创建了一套目录”，而是：

1. 新功能的正确依赖方向由自动化测试强制；
2. 一条真实、租户隔离、可持久化的业务链路已在新结构中运行；
3. 新 handler 能在模块内部扩展，不再扩大集中式 Server；
4. Phase 3 计划和 PR 流程明确要求复用该结构；
5. 任意开发者在 Windows 或 Linux 上都能复现核心质量结论。
