# Phase 3.1：圆弧对标菜单与会话边界

> 记录时间：2026-07-31  
> 对应分支：`phase3/yuanhu-benchmark-phase1`  
> 合并目标：`main`

## 交付范围

- Dashboard 品牌统一为 `MoChat AI`，左侧菜单默认收缩，菜单组可展开并按权限渲染。
- Dashboard 与 SaaS Admin 的浏览器会话存储、退出登录和 cookie 路径分离。
- 建立圆弧对标 Phase 1 验收门禁，覆盖品牌、菜单展开、刷新保持和浏览器控制台错误检查。
- 将运行时使用的圆弧菜单 manifest 从 `docs` 迁移至 `web/apps/dashboard/src/benchmark/manifest.json`；`docs` 仅保存设计、截图、验收和阶段记录。

## 验证入口

```bash
pnpm check:yuanhu-phase1
```

该命令执行 manifest 校验、Dashboard 类型检查/单元测试/构建、E2E 类型检查和 Phase 3.1 Playwright 门禁。

## 当前边界

Phase 3.1 验证的是可演示的菜单与会话基础能力，不代表所有业务菜单已经完成真实后端 CRUD。后续菜单按 manifest 的 owner、risk 和 batch 逐项实现。

## 部署说明

Docker 部署继续使用现有数据卷；运行时镜像通过 `web` 源码打包 manifest，不再复制或读取 `docs` 目录。
