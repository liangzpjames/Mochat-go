# 企业微信会话存档双链路 Demo 设计

## 目标

在不修改 ECS 现有 MoChat、Nginx、MySQL、Redis、容器网络和数据卷的前提下，部署一个可随时删除的独立 Demo，验证：

1. 企业微信回调 URL 校验及加密事件的被动接收；
2. 官方会话存档 SDK 的 `GetChatData`、RSA PKCS#1 会话密钥解密和 `DecryptData` 主动拉取链路。

## 已确认环境

- ECS：Ubuntu 24.04、Linux x86_64、Docker 29，约 20 GiB 可用空间；
- 现有服务使用 80、3306、6379、18080～18082；`19090` 未占用；
- ECS 本机防火墙未启用，公网是否可访问由阿里云安全组决定；
- 本地 Docker 为 Linux/amd64，可在本地完成 CGO 编译并通过 `docker save` 交付；
- SDK 使用企业微信官方 Linux x86 v3.0（2025-02-05，OpenSSL 3），固定 SHA-256：`afa8c017da2994ad2215933f2fcc6042d40d935663ad42d6e1e9d7716652f0d8`。

## 隔离边界

- 服务器目录固定为 `/opt/wecom-archive-demo`；
- 容器名固定为 `wecom-archive-demo`，镜像名固定为 `mochat/wecom-archive-demo:<git-sha>`；
- 公网只发布 `0.0.0.0:19090 -> 8080`，管理接口只发布 `127.0.0.1:19091 -> 9091`；
- 不加入现有 Compose project 或 Docker network，不连接任何现有数据库和 Redis；
- 配置只读挂载，运行证据写入独立 `data` 目录；部署脚本发现同名目录、容器或端口占用时失败退出；
- 卸载只删除上述精确容器、镜像和目录，不执行 prune、`down -v` 或通配删除。

## 公网接口

- `GET /healthz`：仅返回进程、SDK 装载和两条链路的高层状态；
- `GET /wecom/callback`：校验 `msg_signature`，AES-256-CBC 解密 `echostr`，原样返回明文；
- `POST /wecom/callback`：验签并解密事件，记录接收时间、事件类型、接收企业和内容摘要，返回 `success`；
- 首次有效回调会在 Token/AES Key 已验证的前提下绑定密文中的 ReceiveID，之后拒绝不一致企业。

## 本机管理接口

- `GET /admin/status`：返回最后一次回调和拉取结果、当前 seq、公钥指纹与配置状态；
- `POST /admin/pull`：立即执行一次 `GetChatData`，RSA PKCS#1 解密 `encrypt_random_key`，再调用官方 SDK `DecryptData`；
- 管理接口仅监听容器内 `9091` 并映射到 ECS 的 `127.0.0.1:19091`，同时要求随机 Bearer Token；
- `CorpID` 和会话存档 Secret 通过服务器上的交互脚本隐藏输入，不通过公网 HTTP、命令参数或聊天传输。

## 数据与密钥

- 自动生成 Callback Token、43 位 EncodingAESKey、RSA-2048 公私钥和 Admin Token；
- 公钥用于企业微信“会话内容存档”公钥配置，私钥只保存在本地发布包和 ECS `secrets` 目录，权限 `0600`；
- 回调证据写入 `data/callback-events.jsonl`；拉取证据写入 `data/archive-messages.jsonl`；状态和 seq 原子写入 `data/state.json`；
- 证据保留消息元数据、类型、至多 200 字符预览和 SHA-256，不在服务日志输出 Secret、私钥、完整密文或完整正文。

## 部署与验收

本地构建 Linux/amd64 镜像、执行单元测试和容器 smoke test，导出 `image.tar` 与 SHA-256 后上传 ECS。服务器只执行校验、`docker load` 和 `docker run --pull=never`。验收包括公网健康检查、合成加密回调、容器内 SDK 装载、端口隔离，以及在用户完成企业微信后台配置后执行真实回调和主动拉取。

## 已知外部前置条件

- 阿里云安全组需放行入方向 TCP `19090`；
- 用户需在企业微信管理后台开启 30 天会话内容存档体验，并把测试成员加入存档范围；
- 用户需提供其企业自己的 CorpID 和会话存档 Secret，且把 Demo 公钥配置到会话存档页面；
- 普通回调不会推送会话存档正文，因此“被动接收”验证的是企业微信事件回调，“主动拉取”才验证存档消息正文。
