# 2026-08-29 供应链基线与发布证明边界

## 1. 结论

本基线把可变的工具链、容器基础镜像和 GitHub Actions 引用收敛为可审计输入，并把当前 `pnpm audit` 的 critical/high 清零。CI 生成 SPDX JSON 软件物料清单（SBOM）及 SHA-256 校验文件，但这两者只是可复核的未签名证据，不是签名、不可抵赖证明或生产发布 attestation。

查询与复核时间为 2026-08-29（Asia/Shanghai）。所有 SHA 与 digest 均来自官方仓库 ref 或 Registry manifest，不由人工猜测。

## 2. 根因与决策

### 2.1 Go 工具链

原配置仅声明 `go 1.26`，本机、CI 与 Docker builder 可以解析到不同 patch，无法证明安全修复是否实际进入构建。Go 官方在 2026-08-19 发布 1.26.7，且它晚于 2026-08-13 的 1.26.6 安全发布，因此采用当前更高的累积 patch `toolchain go1.26.7`，CI 也显式安装 1.26.7。

来源：[Go release history](https://go.dev/doc/devel/release)、[Go downloads](https://go.dev/dl/)。

### 2.2 容器镜像

仅写 `node:24-alpine`、`golang:1.26-alpine` 或 `alpine:3.22` 时，同名 tag 可移动。Dockerfile 固定多架构 index digest，因为它允许 amd64/arm64 构建器各自选择受该 index 约束的 child manifest；固定 amd64 child digest 会把 Dockerfile 不必要地锁死在单架构。

| 镜像 | Dockerfile 固定的 index digest | 复核用 amd64 child digest |
| --- | --- | --- |
| `golang:1.26.7-alpine`（当前映射 Alpine 3.24） | `sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468` | `sha256:bf9573d7c1d2b09992e4f893ea1ef30842854846bdb8ae390468f95ea6b09062` |
| `node:24.20.0-alpine` | `sha256:e67514e5d0f6c46656005e1b693b2ec9d52e80b641307de684d4a015ba7a4eaf` | `sha256:4caaaf42195bcd6f6f3559a413b20cb8f8ad089e231ee874cf7701643966689f` |
| `alpine:3.22.5` | `sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce` | `sha256:7c8cb692ae09657cbc4a3f3cbd0e8d5a2690ba38386aaaf252dbb060bf5eb2e6` |

来源：[Docker Hub golang](https://hub.docker.com/_/golang)、[Docker Hub node](https://hub.docker.com/_/node)、[Docker Hub alpine](https://hub.docker.com/_/alpine)。

### 2.3 GitHub Actions

major tag 是可移动 ref，不能作为长期审计标识。workflow 使用完整 commit SHA，并保留已核实 tag 注释：

| Action | 官方 tag | peeled commit SHA |
| --- | --- | --- |
| `actions/checkout` | `v4.4.0` | `11d5960a326750d5838078e36cf38b85af677262` |
| `actions/setup-go` | `v5.6.0` | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| `pnpm/action-setup` | `v4` / `v4.3.0` | `b906affcce14559ad1aafd4ab0e942779e9f58b1` |
| `actions/setup-node` | `v4.4.0` | `49933ea5288caeca8642d1e84afbd3f7d6820020` |
| `actions/cache` | `v4.3.0` | `0057852bfaa89a56745cba8c7296529d2fc39830` |
| `anchore/sbom-action` | `v0.24.0` | `e22c389904149dbc22b58101806040fa8d37a610` |
| `actions/upload-artifact` | `v4.6.2` | `ea165f8d65b6e75b540449e92b4886f43607fa02` |

供应链策略测试会拒绝任何非 40 位 SHA 的 `uses:`，也会拒绝这些已核实引用发生静默漂移。

## 3. JavaScript 漏洞定界与修复

升级前审计基线为 1 critical、8 high、3 moderate。critical/high 路径及处理如下：

| 组件 | 旧版本/路径 | 可达性 | 处理 |
| --- | --- | --- | --- |
| `vitest` | 十个 workspace manifest 直接依赖 `2.1.9` | 测试与 CI 可达，不进入生产运行时 | 全部升级为 `3.2.6`，全量前端测试回归 |
| `vite` | Vitest 2 传递引入 `5.4.21` | 构建/测试可达 | Vitest 3 解析到已修复的 `vite 6.4.3`；应用原有 `vite 8.1.5` 不回退 |
| `react-router` | Dashboard、Operation、Sidebar 直接依赖 `7.18.1` | 三端生产运行时可达 | 升级为 `7.18.2` |
| `brace-expansion` | ESLint → minimatch → `1.1.16`；typescript-eslint → minimatch → `2.1.2` | lint/CI 可达 | 精确 override 到 `1.1.18`、`2.1.4` |
| `js-yaml` | ESLint → `@eslint/eslintrc` → `4.3.0` | lint/CI 可达 | 精确 override 到 `4.3.1` |
| `nanoid` | Vite → PostCSS → `3.3.16` | 前端构建可达 | 精确 override 到 `3.3.18` |

修复后 `pnpm why` 只解析到上述安全版本，`pnpm audit --audit-level high` 输出 `No known vulnerabilities found`。精确 override 只替换已出现的脆弱版本，不把未来未知大版本强行压到不兼容版本；后续新增风险由审计门禁继续 fail closed。

## 4. SBOM、checksum 与 provenance 边界

CI 使用固定的 `anchore/sbom-action` 和 Syft `v1.51.1` 从检出源码生成 `mochat-go.spdx.json`，再生成 `mochat-go.spdx.json.sha256` 并作为 workflow artifact 保存。workflow 权限保持 `contents: read`，没有授予 `id-token: write` 或 `attestations: write`。

因此当前状态明确分层：

- SPDX SBOM：已接线，可在 CI 生成；
- SHA-256 checksum：已接线，只证明下载后内容是否变化，不证明发布者身份；
- 当前精确 Git SHA、工具链版本与镜像 digest：可复核构建输入；
- GitHub artifact attestation / 签名 provenance：`SKIP / NOT CONFIGURED`。当前 workflow 不是正式发布流程，也没有 OIDC/签名基础设施，禁止把普通 artifact 上传或 checksum 称为签名完成。

正式发布启用 attestation 前，必须先确定最终二进制/镜像 subject、受保护 environment 与发布权限，再为独立 release job 授予最小的 `id-token: write`、`attestations: write`，并固定官方 `actions/attest` 的完整 SHA。参考：[GitHub artifact attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations)、[actions/attest](https://github.com/actions/attest)。

## 5. 持续门禁

`pnpm check:supply-chain` 同时运行策略单测和仓库实况检查，拒绝：

1. Go patch、基础镜像 index digest 或 Action SHA 漂移；
2. Vitest/React Router 回退以及四个传递依赖安全 override 消失；
3. workflow 移除 `govulncheck@v1.7.0`、high 级 pnpm audit、SBOM/checksum；
4. migration workflow 回退到硬编码 `0098`，不再消费真实 migration inventory。
