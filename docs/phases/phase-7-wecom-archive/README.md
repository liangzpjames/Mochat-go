# Phase 7：真实企业微信会话存档与 ECS 交付

> 启动日期：2026-08-17（Asia/Shanghai）
> 当前状态：**计划完成，实施未开始，Provider 继续保持 `limited`**
> 部署原则：本地构建 Linux amd64 镜像，上传阿里云 ECS 后只做 `docker load` 与 `docker compose --no-build`

## 阶段目标

Phase 7 交付真实企业微信会话存档闭环：

1. 使用企业微信官方 Linux x86_64 C SDK 主动拉取会话数据。
2. 支持 `GetChatData`、`DecryptData`、`GetMediaData`，保留版本化 RSA 私钥并处理媒体分片。
3. 复用 `0138` durable cursor/lease/source/audit，保证断点续传、幂等、失败不越过和租户/企业隔离。
4. 复用并加固现有企业微信 callback，实现 URL 验证、验签解密、快速应答和异步事件消费。
5. 在阿里云 ECS 上完成真实拉取、真实回调、Dashboard 回读、权限、安全、重启和回滚验收。

## 核心决策

- **SDK**：使用官方 C SDK，通过项目内薄 `cgo` 适配层接入；不把第三方 Go wrapper 作为生产信任根。
- **运行方式**：SDK 放入 Debian/glibc `archive-bridge` sidecar；主应用保持现有静态构建。
- **进程通信**：只使用同机 Unix socket，不暴露 TCP 端口；敏感凭据不进入日志、数据库明文或审计 JSON。
- **数据真相**：会话正文由 SDK 主动拉取；企业微信回调是独立的被动事件链路，不是会话正文推送。
- **发布方式**：本地 BuildKit 构建 app/bridge 两个 `linux/amd64` 镜像，生成不可变 tar 与 SHA-256，上传 ECS 后禁止编译。
- **状态口径**：配置、SDK health、fake 测试或 worker running 都不等于 `ready`；只有真实 pull、解密、落库、回调和页面回读全部成功才升级状态。

## 实施顺序

| 批次 | 交付 | 完成口径 |
| --- | --- | --- |
| A | SDK 供应链、Go 接口、cgo adapter | SDK checksum/架构/许可证门禁，资源释放和错误码测试通过 |
| B | Unix socket bridge | 不监听 TCP，fake page/media 通过，日志无凭据和明文 |
| C | RSA keyring、0140、durable source | 多版本私钥、真实 `ArchiveSource`、0138 cursor/lease/审计通过 |
| D | 媒体与 callback | 分片校验、鉴权回读、真实 callback 入队与 worker 审计通过 |
| E | 本地 release | app/bridge Linux amd64 镜像、全门禁、tar/digest/checksum 完整 |
| F | ECS live | 真实双向消息、群聊、撤回、媒体、回调、Dashboard、权限与回滚闭环 |

## ECS 安全边界

- 服务器地址、账号、口令、SSH 私钥和企业微信凭据不进入 Git。
- 已在会话中出现的登录口令需要在部署前轮换，并改用 SSH key。
- ECS 仅公开 HTTPS；SSH 限制管理来源；MySQL、Redis 与 bridge 不对公网开放。
- 部署前后记录容器 ID、镜像 digest、迁移 ledger 和 named volumes；禁止 `down -v`、`volume rm/prune` 和 `system prune`。
- 服务器不执行源码构建；失败时回滚 app/bridge 镜像，数据库迁移按 forward-compatible 策略处理。

## 企业微信后台准备

进入实施的 live 阶段前，需要在企业微信管理后台完成：

- 购买并开启会话内容存档；
- 把专用测试员工加入开启范围，并完成必要的员工/客户告知与授权；
- 配置 RSA-2048 公钥，记录 `publickey_ver`，保留对应私钥版本；
- 获取会话存档 Secret；
- 把 ECS 固定出口 IP 加入会话存档允许范围；
- 为企业微信事件配置公网 HTTPS callback URL、Token、EncodingAESKey，并通过 GET URL 验证。

私钥、Secret、Token、AES Key 只进入受保护的运行时 secret/加密凭据，不写入本文档。

## 最终验收矩阵

- 主动拉取：首次 `seq=0`、增量、翻页、空页、失败重试、重启续传。
- 消息类型：双向单聊、群聊、撤回、至少一种媒体；未知类型不静默丢弃。
- 媒体：分片、断点、MD5/大小、临时文件、鉴权读取、跨企业拒绝。
- 回调：真实 URL 验证、真实事件、重复投递、Redis/worker/audit、dead-letter。
- 数据：`source=external`，模拟数据不进入默认真实视图，cursor 单调，`msgid` 幂等。
- 权限：tenant/corp/employee 范围、深链、导出、检索一致；普通用户看不到敏感诊断。
- 运行：app/bridge 重启、日志脱敏、告警、备份、镜像回滚、卷不变。

## 文档入口

- [详细设计](../../superpowers/specs/2026-08-17-phase7-wecom-archive-design.md)
- [实施计划](../../superpowers/plans/2026-08-17-phase7-wecom-archive.md)
- 企业微信官方《获取会话内容》：<https://developer.work.weixin.qq.com/document/path/91774>
- 企业微信官方《接收消息与事件》：<https://developer.work.weixin.qq.com/document/path/90238>

## 当前未完成项

- 官方 SDK 文件尚未纳入本地受控构建输入，checksum 与许可证记录尚未生成。
- `archive-bridge`、cgo adapter、版本化 RSA keyring、0140 媒体账本尚未实现。
- ECS 尚未部署 Phase 7 镜像，未调用真实 `GetChatData/GetMediaData`。
- 尚未取得真实 callback 与 Dashboard external 数据回读证据。

因此当前 `wecom_archive` 必须继续显示 `limited`，不得标记为可生产使用。
