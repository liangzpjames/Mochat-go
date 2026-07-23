# 第三方依赖声明

精确依赖版本以 [go.mod](go.mod)、[go.sum](go.sum) 和随包前端文件名为准。本文件列出运行时直接依赖；间接 Go 依赖由 `go.sum` 锁定，并应在正式发行物的 SBOM 中完整展开。

## Go 运行时直接依赖

| 组件 | 版本 | 许可证 |
| --- | --- | --- |
| `github.com/go-sql-driver/mysql` | `v1.10.0` | MPL-2.0 |
| `github.com/minio/minio-go/v7` | `v7.2.1` | Apache-2.0 |
| `github.com/redis/go-redis/v9` | `v9.21.0` | BSD-2-Clause |
| `golang.org/x/crypto` | `v0.53.0` | BSD-3-Clause |

## Dashboard 本地化运行时

| 组件 | 版本 | 许可证 | 本地文件 |
| --- | --- | --- | --- |
| Vue | `2.6.10` | MIT | `web/dashboard/dist/vendor/vue-2.6.10.min.js` |
| Vue Router | `3.1.3` | MIT | `web/dashboard/dist/vendor/vue-router-3.1.3.min.js` |
| Vuex | `3.1.1` | MIT | `web/dashboard/dist/vendor/vuex-3.1.1.min.js` |
| Axios | `0.19.0` | MIT | `web/dashboard/dist/vendor/axios-0.19.0.min.js` |

dashboard、sidebar 和 operation 的既有生产静态产物还包含其构建时依赖。正式发行前应从对应构建锁文件和产物生成 SBOM，并随发行物保存各依赖许可证文本。

## SaaS 总后台前端

SaaS 总后台的信息架构和交互模式参考 Nuwa Admin，并针对 MoChat Go 的真实 API、权限和获客前 MVP 范围重新实现。Nuwa Admin 采用 Apache-2.0 许可证，原始声明为 `Copyright 2019 北京翻转极光科技有限责任公司`；许可证副本见 [web/saas-admin/NUWA-APACHE-2.0.txt](web/saas-admin/NUWA-APACHE-2.0.txt)。

| 组件 | 锁定版本 | 许可证 |
| --- | --- | --- |
| Nuwa Admin | 本地输入包 | Apache-2.0 |
| React / React DOM | `19.2.7` | MIT |
| Radix UI Dialog | `1.1.19` | MIT |
| TanStack Query | `5.101.2` | MIT |
| Lucide React | `1.25.0` | ISC |
| Sonner | `1.7.4` | MIT |
| clsx | `2.1.1` | MIT |
| tailwind-merge | `2.6.1` | MIT |

前端精确依赖树由 `web/saas-admin/pnpm-lock.yaml` 锁定。
