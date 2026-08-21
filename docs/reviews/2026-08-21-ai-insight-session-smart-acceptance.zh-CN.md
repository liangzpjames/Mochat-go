# AI 洞察会话分析与智能分析实施验收

## 1. 验收范围

- `http://127.0.0.1:18080/ai-insight/session-analysis`
- `http://127.0.0.1:18080/ai-insight/smart-analysis`

本轮只覆盖上述两个菜单。情绪识别、员工评分、沟通关键词及风险预警菜单未改动。

## 2. 交付内容

- 会话分析改为真实归档会话结果列表，支持会话类型、关键词、员工 ID、日期筛选、固定 20 条分页、重置、刷新、导出和详情入口。
- 智能分析拆分“分析结果/分析规则”两个工作区，支持结果筛选、规则新增、编辑生成不可变版本、启停和删除。
- 后台跨消息分表归并真实会话，模型输出使用严格 JSON Schema 校验，证据消息必须属于来源窗口；页面读取不调用模型。
- 新增 `0148_ai_conversation_insights` 数据模型和 `0151_ai_conversation_insight_api_resources` 权限资源回填迁移。0151 用于修复已执行 0148 的存量数据库接口资源缺失问题，两个迁移均可幂等执行。
- 头像和空状态使用稳定名称/首字 fallback，不渲染固定“请”字，不再显示旧版“AI 能力未接入”三卡片和大段能力说明。

## 3. 自动化验证

以下命令在 2026-08-21 20:00 左右执行并通过：

```text
go test ./internal/modules/ai-insight ./internal/modules/ai-insight/transport/http ./internal/app/bootstrap ./cmd/mochat-go -count=1
go test ./internal/dashboard ./internal/migration ./internal/server -count=1
corepack pnpm --filter @mochat/dashboard test                 # 136 files / 764 tests passed
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard exec eslint src/features/ai-insight src/benchmark/page-registry.tsx src/main.tsx
git diff --check
```

迁移状态已由容器内 `mochat-migrate -action apply -project-root /app` 验证到：

```text
0151_ai_conversation_insight_api_resources  applied_now
```

## 4. Docker 与运行环境

- 只重建 `mochat-go-desktop-app-1`，未执行 `down -v`，MySQL、Redis 及命名卷保留。
- `mochat-go-desktop-app-1`、`mochat-go-desktop-mysql-1`、`mochat-go-desktop-redis-1` 均为 healthy。
- `GET http://127.0.0.1:18080/readyz` 返回 `200`。
- 构建阶段通过 Dashboard、Sidebar、Operation、SaaS Admin 四个前端生产构建及 Go Linux 二进制构建。

## 5. 浏览器真实点击验收

浏览器视口分别使用 `2560×1440` 与 `1366×900`：

| 页面 | 操作 | 结果 |
| --- | --- | --- |
| 会话分析 | 打开页面、查询关键词、重置、刷新 | URL 状态正确恢复；列表空态可读；无权限拒绝、无旧能力 banner |
| 会话分析 | 检查宽屏与常规桌面 | `scrollWidth` 分别等于 `innerWidth`，无横向滚动和控件挤压 |
| 智能分析 | 打开分析结果、关键词查询、重置 | URL 状态正确；无权限拒绝、无旧能力 banner |
| 智能分析 | 切换分析规则、打开新增规则抽屉 | 抽屉、字段、取消/保存按钮和空规则态正常 |
| 智能分析 | 新增一条验收规则后清理 | 保存后列表显示版本 `v1`；验收结束后通过精确 SQL 删除验收规则及其版本，未留下测试数据 |
| 两页 | 读取控制台 | 浏览器 error/warn 数量为 0 |

当前开发库没有新增会话级分析结果，因此页面展示真实空态；未使用前端静态业务数据填充列表。

## 6. 生产专项（只记录，不在页面重复展示）

以下事项不能由本地页面开发替代，需正式环境专项闭环：

1. 企业微信会话存档生产凭据、授权范围、持续同步和数据留存证据；
2. AI Provider Key、模型配额、脱敏/数据处理协议、保留期限与成本告警；
3. 真实消息送入外部模型前的企业授权、告知/同意与审计流程。

本地 Provider 未生成新分析结果不代表生产能力不可用；页面只读取已落库结果，后台任务负责生成并记录失败/积压状态。

## 7. 相关提交与文档

- `f6b6550`：会话级 AI 洞察迁移
- `680e2de`：结构化洞察契约与严格解析
- `ab0a89e`：洞察仓储
- `f67803c`：会话分析与智能分析工作台、后台任务、接口和前端页面
- 设计文档：`docs/superpowers/specs/2026-08-21-ai-insight-session-smart-yuanhu-design.md`
- 实施计划：`docs/superpowers/plans/2026-08-21-ai-insight-session-smart-yuanhu.md`
