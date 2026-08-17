# Phase 7 企业微信双链路 Demo 验收记录

> 日期：2026-08-18（Asia/Shanghai）  
> 运行镜像代码提交：`083198b87da1`  
> ECS：`139.196.34.133`  
> 结论：**Demo 基础设施和合成双链路 PASS；真实企业主动拉取与真实事件等待企业后台配置**

## PASS

- `go test ./internal/wecomarchivedemo ./cmd/wecom-archive-demo -count=1` 通过；
- Linux/amd64、`CGO_ENABLED=0` 本地交叉编译通过；
- 官方 Finance SDK v3 归档 SHA-256 校验通过；
- 镜像内官方 `.so` 装载、必需符号解析及 `NewSdk/DestroySdk` 生命周期通过；
- 临时容器 health smoke 通过，镜像在本地 `docker save` 后上传 ECS，服务器未执行编译；
- 最终镜像 tar SHA-256：`048f772af58ce6bf766dbfdb4a063c6de8bf33b3465d5ac5bdcf54eab4ac9a0`；
- ECS 容器 `wecom-archive-demo` 为 `running|healthy`；公网 `19090` 可达，管理端口只绑定 `127.0.0.1:19091`；
- 公网 `/admin/*` 返回 404，公网访问 `19091/admin/status` 不可达；
- 回调企业绑定与计数使用同一存储锁；主动拉取重复页去重，证据先写游标后写的恢复场景不会重复计数；拉取成功或失败均不会覆盖并发回调状态；
- 公网合成加密 GET：验签、AES 解密、ReceiveID 解析与 echostr 明文返回 PASS；
- 公网合成加密 POST：验签、AES 解密、事件分类 `event.change_contact.create_user`、JSONL 证据和 callback count PASS；
- 合成状态和证据已清除，当前 seq/callback_count/pull_count 均为 0，等待真实企业数据；
- 部署前后三个现有容器保持运行且 ID/状态快照一致：`standalone-app-1`、`standalone-redis-1`、`standalone-mysql-1`。

## WAITING_EXTERNAL_CONFIG

- 企业微信后台保存真实 Callback URL/Token/EncodingAESKey；
- 开启会话内容存档体验并配置 Demo RSA 公钥；
- 测试员工加入存档范围，ECS 出口 IP 加入允许范围；
- 通过交互脚本录入企业自己的 CorpID 和会话存档 Secret；
- 产生真实测试消息后验证 `GetChatData`、RSA PKCS#1、`DecryptData`、seq 推进；
- 触发至少一条真实企业微信事件并确认 callback count 和 evidence。

## 不属于本次 Demo 的完成范围

本次 Demo 不改现有 MoChat 数据链路，因此不能把 Phase 7 或 `wecom_archive` Provider 标为生产 `ready`。正式阶段仍需完成 0138 durable sync 接入、版本化 RSA keyring、媒体 `GetMediaData`、Dashboard external 数据回读、权限与恢复验收。

操作入口见 [Demo 使用说明](../../../runbooks/2026-08-18-wecom-archive-demo.zh-CN.md)。
