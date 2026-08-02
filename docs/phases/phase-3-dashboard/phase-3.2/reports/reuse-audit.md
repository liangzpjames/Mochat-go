# Phase 3.2 Dashboard 复用审计

## 范围

本审计覆盖 Phase 3.2 的 8 个目标路由。`manifest.json` 是路由、实现状态和验收证据的唯一登记来源；本阶段不把演示页或占位页标记为完成。

| 路由 | 现有能力 | 复用决策 | 缺口与后续闭环 |
| --- | --- | --- | --- |
| `/index` | `/dashboard/corpData/index` 与现有数据概览页 | 保留现有真实 API 与页面，补齐状态和验收证据 | 日期边界、空态、权限和刷新回读 |
| `/chat/v2-all` | `/dashboard/workMessage/toUsers`、`/dashboard/workMessage/detail` | 保留现有全局消息查询与详情适配器 | 数据范围、分页、404 和刷新回读 |
| `/ai-insight/v2/sensitive-word` | 敏感词、分组和命中监控旧接口 | 通过 typed adapter 复用旧接口 | 版本冲突、幂等、审计与真实页面 |
| `/customer/clue/default` | Phase 3 SCRM 线索基础模块 | 接入正式 `/dashboard/scrm/leads` 契约 | 转化、作废、数据范围与页面交互 |
| `/customer/contact` | 既有联系人接口可作字段和权限参考 | 通过 SCRM 生命周期服务提供正式接口 | 联系人详情、负责人、协作人与持久化 |
| `/customer/opportunity` | 当前仅有路由/占位基础 | 新建 SCRM 商机领域能力 | 阶段推进、赢单/丢单和版本冲突 |
| `/customer/public-sea` | 当前仅有路由/占位基础 | 复用 SCRM 分配模型，新增原子领取 | 并发领取唯一成功、数据权限与审计 |
| `/customer/tags` | 既有联系人标签接口可作参考 | 通过正式 SCRM 标签接口统一契约 | 租户唯一性、绑定关系和刷新回读 |

## 统一约束

- 服务端从认证上下文推导 tenant、corp 和数据范围，客户端提交的范围字段不可信。
- 完成页面不得依赖 `DemoPage`、`demo-fixtures` 或 `PlaceholderPage`。
- 可变资源使用版本字段；可重试写操作使用稳定幂等键。
- `403` 表示无权，`404` 表示不存在或隐藏，`409` 表示版本冲突，`422` 表示非法状态流转。
- 每个目标页面都需要单元测试、API/集成测试、浏览器主流程和可追溯证据。
