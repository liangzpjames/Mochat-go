# MoChat Go 本地开发与验证

## 前置条件

- Docker Desktop 已安装，并已切换到 Linux containers。
- 仓库根目录是包含 `go.mod` 的目录。
- Windows 推荐使用 Git Bash 执行 shell 脚本；也可以在 WSL2 中使用已映射的 Docker Desktop。
- 本机不要求预装 Go，默认使用 `golang:1.26-alpine`。

## 日常验证

```bash
./scripts/dev_check.sh quick
```

该命令在固定 Go 容器中执行架构门禁、门禁自测、全包测试、`go vet`
和五个命令入口构建。依赖与构建缓存保存在 Docker named volume，不写入仓库。

只运行相同检查的 Compose 形式：

```bash
docker compose -f deploy/dev/docker-compose.yml run --rm tools
```

## 构建正式镜像

```bash
./scripts/dev_check.sh build
```

默认镜像名为 `mochat-go:phase0`，可通过 `MOCHAT_GO_IMAGE_TAG` 覆盖。

## Standalone 分组验收

```bash
./scripts/dev_check.sh standalone
```

依次执行 `core`、`saas` 和 `frontend`。这些验收会创建临时容器，耗时明显
长于 quick。真实企业微信、微信开放平台、双真实租户、生产域名和长时间
稳定性不属于本地验收，不能用本命令冒充生产证据。

## 运行角色

`MOCHAT_GO_RUNTIME_ROLE` 支持：

- `all`：API、worker、scheduler，默认本地兼容模式；
- `api`：只监听 HTTP；
- `worker`：只运行 Redis consumer；
- `scheduler`：只运行定时任务。

生产扩缩容时使用独立角色，本地开发默认保持 `all`。

## 常见问题

- 提示 Docker daemon 未就绪：启动 Docker Desktop 后重试。
- 拉取 Go 镜像或依赖超时：确认 Docker 网络和代理配置。
- PowerShell 无法直接执行 `.sh`：使用 Git Bash，或运行对应的
  `docker compose -f deploy/dev/docker-compose.yml run --rm tools`。
- 验收结束后检查残留：`docker compose -f deploy/standalone/docker-compose.yml down -v`。
