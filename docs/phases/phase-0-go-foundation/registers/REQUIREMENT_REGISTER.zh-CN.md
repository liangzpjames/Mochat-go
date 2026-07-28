# 需求台账

| ID | 需求 | 阶段 | 优先级 | 状态 | 验收 |
| --- | --- | --- | --- | --- | --- |
| REQ-001 | 当前项目完整保存到独立 Git 分支 | Phase 0 | P0 | 已完成 | 远端分支 SHA 可读取 |
| REQ-002 | 建立渐进式模块化单体底座 | Phase 0 | P0 | 已完成 | 角色、模块模板和门禁存在 |
| REQ-003 | 本地 Docker 可重复测试与构建 | Phase 0 | P0 | 进行中 | Docker Linux Go test/vet 已通过；宿主 Bash quick 尚未执行 |
| REQ-004 | 后续接手与开发文档统一归档 | Phase 0 | P0 | 已完成 | `docs/handle/README.md` 生效 |
| REQ-005 | 替换远端 `main` 为当前项目 | Phase 0 | P0 | 已完成 | 备份引用存在且新 main SHA 一致 |
| REQ-006 | 补齐真实生产证据 | Phase 1 | P0 | 未开始 | readiness 六项全绿 |
| REQ-007 | 恢复或迁移旧三端源码 | Phase 1/2 | P0 | 进行中 | 源码已恢复；首条 Dashboard 路由已迁移，其余按批次交付 |
| REQ-008 | 建立统一前端 workspace 与共享契约 | Phase 1 | P0 | 已完成 | frozen install、lint、typecheck、unit/contract、build 全绿 |
| REQ-009 | 建立 Dashboard React 单入口与受控 legacy 路由 | Phase 1 | P0 | 已完成 | manifest 1 React / 60 legacy；未知路径不 fallback |
| REQ-010 | 建立前端 CI 与真实浏览器门禁 | Phase 1 | P0 | 验收中 | Windows 门禁与 Playwright 9/9；Linux CI 配置已提交，远端运行待确认 |
