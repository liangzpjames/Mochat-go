# Mobile 视觉优化 Task 4 验证报告

## 完成门禁

- 新增 7 类独立 RED 坏 fixture：Sidebar 三栏导航缺失、Operation 误引入员工导航、参考品牌复制、静态外链图片、激活导航断言缺失、底栏遮挡断言缺失、Operation 无导航断言缺失。
- 新增截图脚本结构门禁：必须使用 `MOCHAT_MOBILE_VISUAL_OUTPUT`、Playwright Chromium，并且精确生成 7 个规定文件名。
- 门禁从生产 TS/TSX/CSS、真实 route registry 与 E2E 源码取证，不依赖同一 JSON 自证。
- 最终 completion：Sidebar 12、Operation 10、direct fetch 0、Dashboard session reference 0、mojibake 0、fake outcome 0；门禁测试 49/49。
- 终审发现底部导航和工作台卡片的原生链接未消费 React Router basename；已先补 RED，再使用 `useHref` 修复，浏览器测试真实点击并确认仍在 `/sidebar-app`。

## 浏览器矩阵

- Playwright Chromium：48/48 通过，包含 390×844 与 1280×900。
- Sidebar 12 条路由在两视口都已验证：已认证业务页显示客户/会话/我的导航，当前项 `aria-current=page`，auth/unknown 不显示员工导航，最后内容不被底栏遮挡。
- Operation 10 条路由在两视口都已验证无员工导航。
- 所有页面均无文档级横向溢出，可操作控件保持 44px；console、pageerror、requestfailed、意外 4xx/5xx 与未期望 API 请求均为 0。
- 任务宝仍实测 raw Go envelope、session-derived unionid 与 raw `[]` OAuth 边界。

## 包级验证

```text
@mochat/mobile-foundation lint/typecheck/build: PASS
@mochat/mobile-foundation test: 5 files / 30 tests PASS

@mochat/sidebar lint/typecheck/build: PASS
@mochat/sidebar test: 6 files / 62 tests PASS

@mochat/operation lint/typecheck/build: PASS
@mochat/operation test: 5 files / 74 tests PASS

@mochat/e2e lint/typecheck: PASS
mobile-clients-foundation Playwright: 48/48 PASS
completion fixture tests: 57/57 PASS
```

完成门禁会在独立临时目录真实执行截图脚本，严格核对 7 个 PNG 文件名、文件大小及 390×844 / 1280×900 尺寸，并验证 PNG chunk CRC、IDAT 解压与实际 raster；仅伪造文件头或输出自述合同但不生成图片的脚本都会失败。浏览器启动或关闭失败时，本地随机端口服务也会在嵌套 `finally` 中关闭。

## 视觉证据

目录：`D:\workspace\mochat-go\output\mobile-visual-refresh-20260814`

- `sidebar-contact-390.png`
- `sidebar-workbench-390.png`
- `sidebar-pending-390.png`
- `operation-work-fission-390.png`
- `operation-pending-390.png`
- `sidebar-contact-1280.png`
- `operation-work-fission-1280.png`

已逐张视觉检查：390px 证据严格为 390×844 固定视口，无横溢，底部导航不遮挡内容；1280px 下保持移动应用窄栏，不将卡片拉伸成桌面 Dashboard。截图脚本使用当前 dist 和随机本地端口，不再复用可能陈旧的 4174 服务。

## 范围声明

- 这次完成的是两个移动端框架及已有真实页的视觉基线；Sidebar 除客户资料外的 10 个入口、Operation 除任务宝外的 9 个入口仍保持诚实“待迁移”，未冒充业务完成。
- 未操作 Docker、数据库或服务器。
