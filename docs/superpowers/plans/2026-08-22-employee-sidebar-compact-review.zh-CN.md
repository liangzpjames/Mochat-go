# Mochat 员工移动端紧凑化与本地验收入口实施计划

> **供智能执行者使用：** 必须使用 `executing-plans` 逐任务执行；每项行为变更都先使用 `test-driven-development` 完成 RED→GREEN。步骤使用复选框追踪。

**目标：** 在不改变 Sidebar 业务与生产鉴权语义的前提下收紧 12 路由的移动布局，并提供一个只绑定本机、复用现有专项夹具的可点击 Review Server。

**架构：** Sidebar 通过应用级 CSS token 覆盖和少量页面选择器实现紧凑化，不修改 `mobile-foundation` 公共默认值。Review Server 位于 `scripts/`，只服务 Sidebar production build，并从 JSON 夹具返回明确的本地验收响应；生产 Go 服务仍执行真实企业微信鉴权。Standalone Compose 显式传递浏览器可达的 API、Sidebar 和 Operation URL。

**技术栈：** React、CSS、Vitest、Playwright、Node.js `node:http`/`node:test`、Docker Compose、pnpm。

## 全局约束

- 只修改 Sidebar、必要的 E2E/验收脚本和 standalone 公开 URL 配置；不修改 Operation、Dashboard、SaaS Admin、Phase 7 或会话存档业务。
- 所有触控目标继续不小于 44×44px；320px 窄屏不得出现横向溢出。
- Review Server 只监听 `127.0.0.1`，不得写数据库、不得包含真实凭证、不得进入生产 Go 路由。
- 验收夹具必须标注“本地验收数据”，未知接口必须失败关闭，不能随机成功。
- 不删除或重建 MySQL、Redis 命名卷；仅重建必要的 app 服务。

---

### 任务 1：锁定紧凑布局的浏览器尺寸契约

**文件：**
- 修改：`web/e2e/tests/sidebar-employee-mobile.spec.ts`
- 修改：`web/apps/sidebar/src/styles.css`

**接口：**
- 消费：现有 `installFixtures(page)`、`injectSession(page)` 和 `/sidebar-app/` production 页面。
- 产出：`employee Sidebar compact density` Playwright 用例；CSS token `--sidebar-card-radius`、`--sidebar-page-gap`。

- [ ] **步骤 1：编写失败测试**

在 `sidebar-employee-mobile.spec.ts` 增加 360×800 密度断言：

```ts
test('keeps the workbench compact without shrinking touch targets', async ({ page }) => {
  await installFixtures(page);
  await injectSession(page);
  await page.goto('/sidebar-app/');
  const density = await page.evaluate(() => {
    const hero = document.querySelector('.mobile-shell__hero .mobile-card')?.getBoundingClientRect();
    const tiles = [...document.querySelectorAll('.sidebar-workbench__tiles .mobile-icon-tile')];
    const grid = document.querySelector('.sidebar-workbench__tiles');
    return {
      heroHeight: hero?.height ?? 999,
      maxTileHeight: Math.max(...tiles.map((item) => item.getBoundingClientRect().height)),
      gap: grid ? Number.parseFloat(getComputedStyle(grid).gap) : 999,
      minActionHeight: Math.min(...tiles.map((item) => item.getBoundingClientRect().height)),
    };
  });
  expect(density.heroHeight).toBeLessThanOrEqual(128);
  expect(density.maxTileHeight).toBeLessThanOrEqual(92);
  expect(density.gap).toBeLessThanOrEqual(12);
  expect(density.minActionHeight).toBeGreaterThanOrEqual(44);
});
```

- [ ] **步骤 2：运行 RED**

运行：

```powershell
corepack pnpm --filter @mochat/e2e exec playwright test tests/sidebar-employee-mobile.spec.ts --grep "keeps the workbench compact" --workers=1
```

预期：因当前 Hero 或入口卡片高度超过上限而 FAIL，不得因夹具或选择器错误失败。

- [ ] **步骤 3：实现最小紧凑样式**

在 Sidebar `:root` 覆盖应用级 token，并收紧显式页面间距：

```css
:root {
  --mobile-space-3: 0.625rem;
  --mobile-space-4: 0.75rem;
  --mobile-radius-card: 0.75rem;
  --sidebar-page-gap: 0.625rem;
}

.sidebar-workbench__tiles,
.contact-workspace,
.sidebar-form,
.sidebar-form-stack {
  gap: var(--sidebar-page-gap);
}

.sidebar-workbench__tiles .mobile-icon-tile__description {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
```

同步把客户摘要头像降到 56px、SOP 头像降到 44px、业务内容块 padding/radius 收至 10–12px，但保留按钮和输入控件的 44/48px 最小高度。

- [ ] **步骤 4：运行 GREEN 与 Sidebar 回归**

运行同一 Playwright 用例，再运行：

```powershell
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar typecheck
```

预期：专项密度用例 PASS；Sidebar 119 项既有测试不回退。

- [ ] **步骤 5：提交**

```powershell
git add web/apps/sidebar/src/styles.css web/e2e/tests/sidebar-employee-mobile.spec.ts
git commit -m "style(sidebar): tighten employee mobile density"
```

### 任务 2：建立同源的本地验收夹具

**文件：**
- 新建：`web/e2e/fixtures/sidebar-employee-review.json`
- 修改：`web/e2e/tests/sidebar-employee-mobile.spec.ts`

**接口：**
- 产出 JSON 字段：`agentId`、`token`、`contactDetail`、`contactSummary`、`tracks`、`portraitFields`、`tagGroups`、`tags`、`batch`、`contactSop`、`contactSopTips`、`roomSop`、`mediumGroups`、`mediumPage`。
- E2E 与 Review Server 都只读这一份初始业务夹具。

- [ ] **步骤 1：先修改 E2E 导入一个尚不存在的 JSON**

```ts
import reviewFixture from '../fixtures/sidebar-employee-review.json';
```

将 `installFixtures()` 的硬编码值替换为 `reviewFixture.contactSummary` 等字段。

- [ ] **步骤 2：运行 RED**

运行：

```powershell
corepack pnpm --filter @mochat/e2e typecheck
```

预期：FAIL，明确报告 `sidebar-employee-review.json` 不存在。

- [ ] **步骤 3：创建固定 JSON**

JSON 使用现有专项值，例如：

```json
{
  "agentId": "7",
  "token": "sidebar-review-token",
  "contactDetail": { "id": 23, "name": "专项验收客户", "avatar": null, "corpId": 9 },
  "contactSummary": { "name": "专项验收客户", "gender": 2, "genderText": "女", "businessNo": "C-23", "remark": "重点客户", "description": "偏好下午沟通", "tag": [{ "tagId": 7, "tagName": "高意向" }], "roomName": ["专项验收客户群"], "employeeName": ["员工甲"] }
}
```

完整迁移 E2E 中已有的所有固定夹具，不新增与页面无关的业务数字。

- [ ] **步骤 4：运行 GREEN**

```powershell
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e exec playwright test tests/sidebar-employee-mobile.spec.ts --workers=1
```

预期：typecheck PASS，Sidebar 专项用例全部 PASS。

- [ ] **步骤 5：提交**

```powershell
git add web/e2e/fixtures/sidebar-employee-review.json web/e2e/tests/sidebar-employee-mobile.spec.ts
git commit -m "test(sidebar): share employee review fixtures"
```

### 任务 3：实现仅本机 Review Server

**文件：**
- 新建：`scripts/sidebar_review_server.mjs`
- 新建：`scripts/sidebar_review_server.test.mjs`
- 修改：`package.json`

**接口：**
- `createSidebarReviewServer({ distRoot, fixture, host, port }): { server, url, reset }`
- `reviewAuthLocation(requestURL, fixture, origin): string | null`
- CLI：`corepack pnpm review:sidebar`，默认地址 `http://127.0.0.1:28083/sidebar-app/login?agentId=7&target=%2F`。

- [ ] **步骤 1：编写失败的 Node 测试**

测试必须覆盖：

```js
test('accepts only the fixed review agent and an internal target', () => {
  assert.equal(reviewAuthLocation('/sidebar/agent/auth?agentId=8&target=%2F', fixture, origin), null);
  assert.equal(reviewAuthLocation('/sidebar/agent/auth?agentId=7&target=https://evil.test', fixture, origin), null);
  assert.match(reviewAuthLocation('/sidebar/agent/auth?agentId=7&target=%2Fcontact', fixture, origin), /\/sidebar-app\/auth\?/);
});

test('requires the review bearer token and fails unknown APIs closed', async () => {
  assert.equal((await request('/sidebar/workContact/show')).status, 401);
  assert.equal((await authorized('/sidebar/unknown')).status, 404);
});
```

另验证备注写入后 `show` 回读新值、`reset()` 后恢复 JSON 初值，以及注入页面含“本地验收数据 · 重启后重置”。

- [ ] **步骤 2：运行 RED**

```powershell
node --test scripts/sidebar_review_server.test.mjs
```

预期：FAIL，报告模块不存在。

- [ ] **步骤 3：实现最小服务**

使用 `node:http` 和 `node:fs`：

```js
export function createSidebarReviewServer({ distRoot, fixture, host = '127.0.0.1', port = 28083 }) {
  let state = structuredClone(fixture);
  const server = createServer(async (request, response) => {
    const url = new URL(request.url ?? '/', `http://${request.headers.host}`);
    if (!allowedHost(request.headers.host, host, port)) return sendText(response, 403, 'review host rejected');
    if (url.pathname === '/sidebar/agent/auth') return redirectToReviewCallback(response, url, state, host, port);
    if (url.pathname.startsWith('/sidebar/')) return handleReviewAPI(request, response, url, state);
    return serveReviewAsset(response, url.pathname, distRoot, injectReviewBar);
  });
  return { server, url: `http://${host}:${port}/sidebar-app/login?agentId=${fixture.agentId}&target=%2F`, reset: () => { state = structuredClone(fixture); } };
}
```

只实现 Sidebar 当前调用的明确端点；所有响应使用 `{ code: 200, msg: 'ok', data, requestId: 'sidebar-local-review' }`。写入仅改变进程内 `state`。

- [ ] **步骤 4：运行 GREEN 和脚本 lint**

```powershell
node --test scripts/sidebar_review_server.test.mjs
node --check scripts/sidebar_review_server.mjs
```

预期：全部 PASS。

- [ ] **步骤 5：提交**

```powershell
git add scripts/sidebar_review_server.mjs scripts/sidebar_review_server.test.mjs package.json
git commit -m "feat(sidebar): add local review server"
```

### 任务 4：修正 standalone 浏览器公开 URL

**文件：**
- 新建：`scripts/check_standalone_public_urls.test.mjs`
- 修改：`deploy/standalone/docker-compose.yml`
- 修改：`deploy/standalone/README.md`

**接口：**
- Compose 环境变量：`MOCHAT_API_BASE_URL`、`MOCHAT_DASHBOARD_BASE_URL`、`MOCHAT_SIDEBAR_BASE_URL`、`MOCHAT_OPERATION_BASE_URL`。

- [ ] **步骤 1：编写失败测试**

读取 Compose 文本并断言 app 环境包含四个公开 URL，Sidebar 默认值引用 `${MOCHAT_SIDEBAR_PORT:-18081}` 而非内部监听地址：

```js
assert.match(compose, /MOCHAT_SIDEBAR_BASE_URL:.*MOCHAT_SIDEBAR_PORT/);
assert.match(compose, /MOCHAT_API_BASE_URL:.*MOCHAT_GO_PORT/);
```

- [ ] **步骤 2：运行 RED**

```powershell
node --test scripts/check_standalone_public_urls.test.mjs
```

预期：FAIL，当前 Compose 未传入公开 URL。

- [ ] **步骤 3：修改 Compose 与中文/现有部署说明**

在 app environment 增加：

```yaml
MOCHAT_API_BASE_URL: "${MOCHAT_API_BASE_URL:-http://127.0.0.1:${MOCHAT_GO_PORT:-18080}}"
MOCHAT_DASHBOARD_BASE_URL: "${MOCHAT_DASHBOARD_BASE_URL:-http://127.0.0.1:${MOCHAT_GO_PORT:-18080}}"
MOCHAT_SIDEBAR_BASE_URL: "${MOCHAT_SIDEBAR_BASE_URL:-http://127.0.0.1:${MOCHAT_SIDEBAR_PORT:-18081}}"
MOCHAT_OPERATION_BASE_URL: "${MOCHAT_OPERATION_BASE_URL:-http://127.0.0.1:${MOCHAT_OPERATION_PORT:-18082}}"
```

README 明确生产需覆盖为 HTTPS 可信域名，本地端口覆盖时也应同时覆盖公开 URL。

- [ ] **步骤 4：运行 GREEN 与 Compose 渲染**

```powershell
node --test scripts/check_standalone_public_urls.test.mjs
docker compose -p mochat-sidebar-mobile-acceptance --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml config --quiet
```

预期：测试与 Compose 配置渲染 PASS。

- [ ] **步骤 5：提交**

```powershell
git add scripts/check_standalone_public_urls.test.mjs deploy/standalone/docker-compose.yml deploy/standalone/README.md
git commit -m "fix(deploy): publish reachable mobile client urls"
```

### 任务 5：启动可点击入口并完成多尺寸验收

**文件：**
- 修改：`web/e2e/tests/sidebar-employee-mobile.spec.ts`
- 更新：`docs/reviews/evidence/employee-sidebar-mobile/*.png`

**接口：**
- 人工入口：`http://127.0.0.1:28083/sidebar-app/login?agentId=7&target=%2F`

- [ ] **步骤 1：构建并启动 Review Server**

```powershell
corepack pnpm --filter @mochat/sidebar build
corepack pnpm review:sidebar
```

进程必须持续运行；HTTP `/readyz` 返回 200，正文包含 `sidebar-local-review`。

- [ ] **步骤 2：浏览器验收**

验证登录按钮进入首页，并逐路由检查 360×800、390×844、430×932；补测 320×568 和 844×390。记录控制台、请求失败、横向溢出、触控面积、底部导航与输入聚焦。

- [ ] **步骤 3：重建 Docker app 并验证真实鉴权边界**

使用既有 acceptance Compose 项目和命名卷，只重建 app；确认 `/readyz` 200、公开 URL 指向主机映射端口、不存在的 `agentId=1` 仍诚实返回“应用不存在”。

- [ ] **步骤 4：更新截图证据并提交**

```powershell
git add docs/reviews/evidence/employee-sidebar-mobile web/e2e/tests/sidebar-employee-mobile.spec.ts
git commit -m "test(sidebar): refresh compact mobile evidence"
```

### 任务 6：完整门禁、中文报告与最终提交

**文件：**
- 修改：`docs/reviews/2026-08-22-employee-sidebar-mobile-acceptance.zh-CN.md`

- [ ] **步骤 1：运行所有受影响门禁**

```powershell
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar build
node --test scripts/sidebar_review_server.test.mjs scripts/check_standalone_public_urls.test.mjs
corepack pnpm check:mobile-clients-foundation
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e lint
corepack pnpm --filter @mochat/e2e exec playwright test tests/mobile-clients-foundation.spec.ts tests/sidebar-employee-mobile.spec.ts --workers=1
git diff --check 32ef7f83160e65bcde9a1be2a5fdcfaa7735e6a0..HEAD
```

预期：所有可控门禁为 PASS；真实企业微信凭证相关仍明确为 SKIP。

- [ ] **步骤 2：更新中文验收报告**

记录新入口、Review Server 边界、紧凑化前后尺寸、测试数、Docker 健康、命名卷和真实 OAuth SKIP，不把测试夹具写成生产数据。

- [ ] **步骤 3：提交并核对工作树**

```powershell
git add docs/reviews/2026-08-22-employee-sidebar-mobile-acceptance.zh-CN.md
git commit -m "docs: record compact sidebar review acceptance"
git status --short --branch
git diff --check 32ef7f83160e65bcde9a1be2a5fdcfaa7735e6a0..HEAD
```

预期：功能分支工作树干净，未修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
