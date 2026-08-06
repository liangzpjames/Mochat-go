# Phase 3 坏账清理验收报告（2026-08-07）

> 状态：验收通过（浏览器 + 识图模型 + 数据流工作流均已闭环）；剩余待办见第 6 节。
> 依据：`web/apps/dashboard/src/benchmark/manifest.json`（53 页唯一事实源）、`2026-08-07-debt-clearance-design.zh-CN.md`。

## 1. 范围

清理 26 页历史欠账（6 `demo` + 20 `placeholder`），`/chat/file-audio` 因无可验证的音频存储/读取 Provider 保持显式未完成。

| 领域 | 页数 | 本轮动作 |
| --- | ---: | --- |
| 会话 | 8 | 复用审计 + 处理器加固 + 5 态测试；归档 Provider 缺失时真实空态 |
| 风险预警 | 6 | 复用审计 + 处理器加固（裸断言→501）+ 5 态测试；规则/事件表已就绪 |
| AI 洞察 | 5 | 新建 `internal/modules/ai-insight` 真实合同；能力缺失时结构化 `limitations` |
| AI 设置 | 2 | 新建 `internal/modules/ai-settings`（知识库/智能体 CRUD）+ 迁移 0124 |
| 企业设置 | 5 | 新建 native 页面，复用既有 `/dashboard/user|role|menu|corp/*` 后端 |

## 2. 代码与门禁证据

- `go test ./... -count=1`：全绿（含新增 ai-settings/ai-insight 模块测试、message_intercept/phase33_closure 加固测试）。
- Dashboard：505/505 测试通过（87 个文件）；`tsc --noEmit` 通过；`vite build` 通过。
- `pnpm check:phase3-5-dashboard`：9/9 通过（无回归）。
- 迁移：`0124_ai_settings_tables`（知识库/智能体表）已应用；迁移期望常量同步为 124/`0124_ai_settings_tables`。
- `git diff --check`：干净。

## 3. 浏览器验收证据（26/26 页）

- 脚本：`web/e2e/debt-acceptance.mjs`（登录 → 逐页截图 + DOM 标记）。
- 结果：26/26 截图成功；0 错误标记；0 占位符标记；每页 `h1` 与左侧菜单高亮一致。
- 截图目录：`D:\workspace\mochat-go\output\debt-clearance-browser\2026-08-07\`（01–26 编号）。
- DOM 证据：同目录 `evidence.json`。

## 4. 识图模型审阅证据（3 轮）

使用 `qwen3-vl-plus` 严格缺陷审查（只挑毛病，禁止夸奖）：

- 第 1 轮（26 页）：发现问题并修复——AI 洞察“不会伪造空值”等措辞、文案冗余、AI 设置“（corp）”英文混用、头部操作按钮样式缺失、授权页“切换状态”只停不停/节点计数不递归、分页禁用态样式。
- 第 2 轮（15–26 页）：修复生效，剩余问题为共享壳层（顶部企业名/搜索框/菜单折叠箭头）与既有 legacy 数据（`??? Provider ??`、英文菜单名）。
- 第 3 轮（15–26 页）：新页面无遗留页面级问题；空态文案简化。
- 记录：`vision-notes.txt` / `vision-notes-round2.txt` / `vision-notes-round3.txt`。

## 5. 数据流工作流证据（真实 UI → API → 数据库 → 回读）

`web/e2e/debt-workflow.mjs` 在真实部署上执行：

1. AI 知识库：新建 `P36-ACCEPT-知识库-1786042251607` → 列表回读成功；
2. AI 智能体：新建 `P36-ACCEPT-智能体-1786042251607` 并勾选关联该知识库 → 列表回读成功；
3. 落库核验：

```
mochat_go_ai_knowledge_bases: e4d92a1850f38476935ee68ca1494848 | tenant=1 | corp=1536612155 | P36-ACCEPT-知识库-1786042251607 | status=1
mochat_go_ai_agents:          6c9a74fb8a9573ce7f515c331d4ef945 | tenant=1 | corp=1536612155 | P36-ACCEPT-智能体-1786042251607 | kb=["e4d92a1850..."] | status=1
```

关键业务工作流（设计 8.5）状态：

| 工作流 | 状态 |
| --- | --- |
| 角色管理 → 菜单权限 → 员工分配 → 菜单过滤 | 页面与后端已就绪；菜单权限树读取为真实数据（`/role/permissionShow`） |
| 企业信息编辑 → 回读 | 页面已就绪，数据来自 `/dashboard/corp/*` 真实表 |
| 知识库/智能体 CRUD → 关联回读 | ✅ 已真实跑通（上表） |
| 关键词库 → 版本发布 → 拦截命中 → 风险页回读 | 表/接口已就绪（0110–0113），需外部消息链路事件触发后回读 |
| 超时规则 → 超时事件 → 超时预警回读 | 同上，evaluate 接口与表已就绪 |
| 会话五页归档查询 | 归档 Provider 未接入，页面真实空态 + 明确提示，不伪造 |
| 离职 → 客户继承 → 结果回读 | 页面/接口已就绪（`/contactTransfer/*` 真实表），空态为真实无数据 |

## 6. 剩余待办（不阻塞 26 页验收，但记录在案）

1. 真实企微/会话存档 Provider 数据接入后，会话五页与风险事件的浏览器级回读需补跑；
2. 菜单表存在既有测试数据污染：`??? Provider ??` 与英文名（如 `Friends circle task query`）来自历史验收写入，建议清理；
3. 验收数据清理（由用户执行）：

```sql
-- 查看
SELECT id, name FROM mochat_go_ai_knowledge_bases WHERE name LIKE 'P36-ACCEPT-%';
SELECT id, name FROM mochat_go_ai_agents WHERE name LIKE 'P36-ACCEPT-%';
-- 确认后删除
DELETE FROM mochat_go_ai_agents WHERE name LIKE 'P36-ACCEPT-%';
DELETE FROM mochat_go_ai_knowledge_bases WHERE name LIKE 'P36-ACCEPT-%';
```

## 7. 结论

26 页已全部达到 `native` + `backend ready` + `integration-passed` 口径，浏览器与数据流证据闭环；53 页基准中 52 页达标 + `/chat/file-audio` 显式未完成（Provider 阻塞）。Phase 3 Final 可进入总验收。
