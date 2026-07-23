# 对应源码提供说明

本项目包含基于 GPL-3.0 授权的 MoChat 派生改造代码。

对外分发 Docker 镜像、二进制、安装包、私有化部署包或其他可再部署版本时，应同时提供与该版本完全对应的源码，至少包括：

- Go 服务、迁移器、初始化工具和维护工具源码；
- dashboard、sidebar、operation 前端静态产物及本地化依赖；
- 数据库 schema、增量迁移、部署配置和构建脚本；
- `LICENSE`、`NOTICE.md`、`MODIFICATIONS.md` 和 `THIRD_PARTY_NOTICES.md`；
- 与交付版本匹配的 tag、commit 或 `docs/evidence/latest/source-fingerprint.json`。

当前对应源码即为本 `mochat-go` 目录。正式交付时应固定版本标识和源码获取地址，不应只提供运行镜像。
