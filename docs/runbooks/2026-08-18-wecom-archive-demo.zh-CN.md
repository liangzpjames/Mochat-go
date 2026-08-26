# 企业微信会话存档双链路 Demo 使用说明

## 2026-08-26 当前集成入口

当前应优先使用 MoChat 主应用的专用接收事件地址，而不是下文原隔离 Demo 的 `19090` 地址：

`http://139.196.34.133/wecom/archive/callback?cid=4`

该地址已经通过公网路由、失败关闭和重启后验证，并在 Dashboard“唯一企业资料 → 会话存档配置”中以只读字段展示。企业微信后台的 Token、EncodingAESKey 必须与 MoChat 当前企业回调配置一致；会话存档 Secret 和 RSA 私钥只在受保护的服务端配置中使用，不得填写到 URL、文档或普通请求。

当前 bridge 镜像为 `mochat/wecom-archive-demo:d0c9409df11a`，只允许主应用通过 Docker 内网调用；管理口仍仅绑定 `127.0.0.1:19091` 且必须鉴权。2026-08-26 20:27:30（CST）企业微信真实 GET URL 校验已返回 200；这只证明 URL、Token/AES 链路。真实 CorpID/会话存档 Secret、RSA 公钥、允许 IP、测试员工范围和真实消息配置完成前，不能宣称真实拉取成功。

下文 `19090/wecom/callback` 是 2026-08-18 原隔离 Demo 的历史操作说明，保留用于回滚和对照，不应与当前 MoChat 集成入口混用。

## 已部署资源

- 公网 IP：`139.196.34.133`
- 公网端口：`19090/TCP`（已从公网验证可达）
- 回调路径：`/wecom/callback`
- 健康检查：`http://139.196.34.133:19090/healthz`
- ECS 目录：`/opt/wecom-archive-demo`
- 容器：`wecom-archive-demo`
- 镜像：`mochat/wecom-archive-demo:083198b87da1`
- 管理端口：`127.0.0.1:19091`，不对公网开放

实际 Token、EncodingAESKey、RSA 公钥和管理 Token 不进入 Git。完整填写值位于：

- 本地：`output/wecom-archive-demo-release/secrets/wecom-fill.txt`
- ECS：`/opt/wecom-archive-demo/wecom-fill.txt`
- RSA 公钥：`/opt/wecom-archive-demo/public_key.pem`
- RSA 私钥：`/opt/wecom-archive-demo/secrets/private_key.pem`（`0600`，不得填入企微或对外发送）
- 管理 Token：`/opt/wecom-archive-demo/secrets/admin-token.txt`（`0600`）

## 企业微信后台配置

### 1. 被动回调

在截图所示页面填写 `wecom-fill.txt` 中的三项：

- URL：`http://139.196.34.133:19090/wecom/callback`
- Token：复制 `Token:` 后的完整值
- EncodingAESKey：复制 `EncodingAESKey:` 后的完整 43 位值

点击保存时，企业微信会发起加密 GET 校验。保存成功后，Demo 会把密文里的真实 CorpID 绑定为 ReceiveID；之后接收到的加密 POST 事件会记录到 `/opt/wecom-archive-demo/data/callback-events.jsonl`。

### 2. 会话内容存档主动拉取

1. 开启 30 天免费体验，把专用测试员工加入存档范围；
2. 在会话内容存档的公钥配置处，完整粘贴 `public_key.pem`，包括 BEGIN/END 行；
3. 记录企微返回的公钥版本号；
4. 把 ECS 出口 IP `139.196.34.133` 加入会话存档允许 IP；
5. 获取该企业的 CorpID 和“会话内容存档 Secret”；
6. 登录 ECS 并运行：

```bash
ssh root@139.196.34.133
/opt/wecom-archive-demo/configure_wecom_archive_demo.sh
```

脚本会交互读取 CorpID，并隐藏输入 Secret；不会把 Secret 放在命令参数或 shell history 中。它只重建 `wecom-archive-demo` 自身，然后立即执行一次主动拉取。首次拉取从 `seq=0` 开始，成功后将 seq 写入独立状态文件。

## 验证命令

公网健康：

```powershell
Invoke-RestMethod http://139.196.34.133:19090/healthz
```

ECS 详细状态：

```bash
admin_token=$(tr -d '\r\n' < /opt/wecom-archive-demo/secrets/admin-token.txt)
curl -fsS -H "Authorization: Bearer $admin_token" http://127.0.0.1:19091/admin/status
```

再次主动拉取：

```bash
admin_token=$(tr -d '\r\n' < /opt/wecom-archive-demo/secrets/admin-token.txt)
curl -fsS -X POST -H "Authorization: Bearer $admin_token" http://127.0.0.1:19091/admin/pull
```

证据文件：

```bash
tail -n 20 /opt/wecom-archive-demo/data/callback-events.jsonl
tail -n 20 /opt/wecom-archive-demo/data/archive-messages.jsonl
cat /opt/wecom-archive-demo/data/state.json
docker logs --tail 100 wecom-archive-demo
```

`archive-messages.jsonl` 保存 seq、msgid、公钥版本、消息类型、参与方、至多 200 字符文本预览和正文 SHA-256；服务日志不输出 Secret、私钥或完整消息正文。

## 状态解释

- `callback_configured=true`：Token/AES Key 已装载，不代表企微后台已保存成功；
- `callback_count>0`：至少收到一条真实加密 POST；
- `active_pull_configured=true` 与 `sdk_loaded=true`：CorpID/Secret 已装载，官方 SDK 初始化成功；
- `pull_count>0`：至少一次 `GetChatData` 成功；
- `pulled_message_count>0`：至少一条消息完成 RSA PKCS#1 和 `DecryptData` 解密；
- `last_pull_error`：主动拉取错误；错误码 `10009` 通常表示 IP 未加入允许范围。

## 隔离与卸载

Demo 不连接现有数据库、Redis、Nginx、Compose project 或 Docker network。服务器只新增了容器、镜像和 `/opt/wecom-archive-demo`。

如需卸载，先备份证据，再精确删除 Demo 自身：

```bash
tar -czf /root/wecom-archive-demo-evidence.tgz -C /opt/wecom-archive-demo data public_key.pem wecom-fill.txt
docker rm -f wecom-archive-demo
docker image rm mochat/wecom-archive-demo:083198b87da1
mv /opt/wecom-archive-demo /opt/wecom-archive-demo.removed-$(date +%Y%m%d%H%M%S)
```

禁止执行 `docker system prune`、`docker volume prune`、`docker compose down -v` 或针对 `/opt` 的递归删除。
