# 企业微信会话内容存档真实试用 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提供可通过企业微信后台校验的专用会话存档回调 URL，并完成真实事件通知、官方 Finance SDK 主动拉取、主程序幂等落库和证据闭环。

**Architecture:** MoChat Go 公开 `/wecom/archive/callback`，复用现有企微验签解密能力并按 `cid` 隔离企业；`msgaudit_notify` 在 Redis worker 中触发与定时兜底共用的 `WorkMessageArchiveSyncCron`。同机 `wecom-archive-demo` 增加仅内网可访问、Bearer 鉴权的 bridge，使用启动时隐藏录入的 CorpID、Archive Secret 和 RSA 私钥执行官方 SDK 拉取，HTTP 请求不再传递这些敏感值。

**Tech Stack:** Go 1.26、标准库 HTTP/JSON/XML、Redis、MySQL、企业微信 Finance SDK v3、Docker、Nginx/现有反向代理、PowerShell、Bash。

## Global Constraints

- 对外试用 URL 固定为 `http://139.196.34.133/wecom/archive/callback?cid=4`；只有全部门禁通过后才交给用户填写。
- 在读写任何真实事件前核验 `mc_corp.id=4` 对应当前测试企业。
- Secret、RSA 私钥、SSH 口令、Token、EncodingAESKey 不进入聊天、Git、文档、命令参数、日志、截图或测试夹具。
- 回调 HTTP 请求不得同步等待 Finance SDK；HTTP 只验签、解密、入队和快速返回。
- 事件触发和定时兜底共用数据库 seq；失败不推进游标，重复拉取不重复落库。
- AI 自动分析在真实试用期间保持关闭。
- 不删除生产数据库、Docker 命名卷或现有配置；部署前创建可验证回滚点。
- 标准员工、客户、客户群 API 的域名阻断与会话存档试用分别记录。

---

## 文件结构与职责

- `internal/server/server.go`：注册会话存档专用公开路由并在兼容状态中声明。
- `internal/server/server_test.go`：验证新路由精确匹配 GET/POST，且不开放子路径或其他方法。
- `internal/dashboard/wework_callback_worker.go`：识别 `event.msgaudit_notify` 并调用可注入的 archive 同步触发器。
- `internal/dashboard/wework_callback_worker_test.go`：验证真实事件路径触发、无 trigger 兼容和错误重试语义。
- `internal/dashboard/work_message_archive_sync_cron.go`：提供按企业同步入口、串行化事件与周期拉取、移除 bridge 请求中的敏感字段。
- `internal/dashboard/work_message_archive_sync_cron_test.go`：验证按企业过滤、并发互斥、请求体不含凭据和游标幂等。
- `internal/wecomarchivedemo/archive.go`：增加无状态 `FetchPage`，按请求 seq 解密一页并返回主程序契约消息。
- `internal/wecomarchivedemo/archive_test.go`：验证 seq、limit、解密失败和不修改 Demo 持久游标。
- `internal/wecomarchivedemo/server.go`：提供 Bearer 鉴权的 `/work-message/archive/messages` bridge。
- `internal/wecomarchivedemo/server_test.go`：验证鉴权、CorpID 绑定、响应契约和错误脱敏。
- `cmd/mochat-go/main.go`：共享同一个 archive cron 给事件 worker 和周期任务。
- `deploy/standalone/docker-compose.yml`：声明主应用的 archive cron/bridge 配置；服务器部署配置按同一字段建立仅内部网络连通，不公开 bridge 管理端口。
- `docs/runbooks/2026-08-18-wecom-archive-demo.zh-CN.md`：更新专用 URL、交互式凭据输入、真实试用与回滚步骤。
- `docs/PROJECT_PROGRESS.zh-CN.md`：记录真实与模拟边界、当前结果和剩余风险。

### Task 1: 专用会话存档公开路由

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- Consumes: 已有 `Server.weWorkCallback http.Handler`。
- Produces: `GET/POST /wecom/archive/callback`，查询参数原样传入已有 handler。

- [ ] **Step 1: 写精确路由失败测试**

在 `internal/server/server_test.go` 的企微回调测试中加入：

```go
for _, path := range []string{
	"/weWork/callback?cid=4",
	"/dashboard/corp/weWorkCallback?cid=4",
	"/wecom/archive/callback?cid=4",
} {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req := httptest.NewRequest(method, path, nil)
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s %s status=%d, want %d", method, path, recorder.Code, http.StatusNoContent)
		}
	}
}
```

另加 `PUT /wecom/archive/callback` 和 `GET /wecom/archive/callback/extra` 返回 404 的断言。

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/server -run 'Test.*WeWorkCallback' -count=1`

Expected: FAIL，新路径返回 404。

- [ ] **Step 3: 添加最小路由与状态清单**

将 server 的路由条件改为：

```go
case (r.URL.Path == "/weWork/callback" ||
	r.URL.Path == "/dashboard/corp/weWorkCallback" ||
	r.URL.Path == "/wecom/archive/callback") &&
	(r.Method == http.MethodGet || r.Method == http.MethodPost) && s.weWorkCallback != nil:
	s.weWorkCallback.ServeHTTP(w, r)
```

并在已迁移路由清单中增加：

```go
"GET /wecom/archive/callback",
"POST /wecom/archive/callback",
```

- [ ] **Step 4: 运行路由测试确认 GREEN**

Run: `go test ./internal/server -run 'Test.*WeWorkCallback' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交路由变更**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat: add dedicated WeCom archive callback route"
```

### Task 2: 安全的 bridge 请求契约

**Files:**
- Modify: `internal/dashboard/work_message_archive_sync_cron.go`
- Modify: `internal/dashboard/work_message_archive_sync_cron_test.go`

**Interfaces:**
- Consumes: `WorkMessageArchiveCorp` 和现有 `/work-message/archive/messages`。
- Produces: bridge 请求只包含 `corp_id`、`wx_corpid`、`seq`、`limit`；Bearer 继续放在请求头。

- [ ] **Step 1: 写请求体不得携带凭据的失败测试**

在测试 bridge handler 中读取原始 JSON 并断言：

```go
var body map[string]any
if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
	t.Fatal(err)
}
for _, forbidden := range []string{"chat_secret", "rsa_public_key", "rsa_private_key"} {
	if _, ok := body[forbidden]; ok {
		t.Fatalf("bridge request leaked %s", forbidden)
	}
}
if body["wx_corpid"] != "ww-live" || int(body["corp_id"].(float64)) != 4 {
	t.Fatalf("bridge identity=%#v", body)
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/dashboard -run 'TestWorkMessageArchiveBridgeClient' -count=1`

Expected: FAIL，当前请求包含 `chat_secret` 和 RSA 密钥。

- [ ] **Step 3: 缩减 bridge 请求字段**

在 `FetchWorkMessageArchive` 中只构造：

```go
request := map[string]any{
	"corp_id":   corp.CorpID,
	"wx_corpid": corp.WXCorpID,
	"seq":       seq,
	"limit":     limit,
}
```

- [ ] **Step 4: 运行 client 与 cron 测试**

Run: `go test ./internal/dashboard -run 'TestWorkMessageArchive' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交安全契约**

```bash
git add internal/dashboard/work_message_archive_sync_cron.go internal/dashboard/work_message_archive_sync_cron_test.go
git commit -m "fix: keep archive credentials out of bridge requests"
```

### Task 3: Finance SDK bridge 返回解密消息

**Files:**
- Modify: `internal/wecomarchivedemo/archive.go`
- Modify: `internal/wecomarchivedemo/archive_test.go`
- Modify: `internal/wecomarchivedemo/server.go`
- Modify: `internal/wecomarchivedemo/server_test.go`

**Interfaces:**
- Produces: `ArchiveService.FetchPage(ctx context.Context, startSeq uint64, limit uint32) (ArchivePage, error)`。
- Produces: `POST /work-message/archive/messages`，请求 `{corp_id,wx_corpid,seq,limit}`，响应 `{errcode,errmsg,messages}`。

- [ ] **Step 1: 写 FetchPage 失败测试**

测试 fake SDK 返回一条加密消息，断言响应消息同时包含解密正文和外层 seq：

```go
page, err := service.FetchPage(context.Background(), 40, 10)
if err != nil {
	t.Fatal(err)
}
if page.StartSeq != 40 || page.NextSeq != 41 || len(page.Messages) != 1 {
	t.Fatalf("page=%#v", page)
}
var message map[string]any
if err := json.Unmarshal(page.Messages[0], &message); err != nil {
	t.Fatal(err)
}
if int(message["seq"].(float64)) != 41 || message["msgid"] != "msg-live-41" {
	t.Fatalf("message=%#v", message)
}
state, _ := store.LoadState()
if state.Seq != 0 {
	t.Fatalf("FetchPage advanced demo cursor to %d", state.Seq)
}
```

- [ ] **Step 2: 运行 ArchiveService 测试确认 RED**

Run: `go test ./internal/wecomarchivedemo -run 'TestArchiveServiceFetchPage' -count=1`

Expected: FAIL，`FetchPage` 尚不存在。

- [ ] **Step 3: 实现无状态一页拉取**

新增：

```go
type ArchivePage struct {
	StartSeq uint64            `json:"start_seq"`
	NextSeq  uint64            `json:"next_seq"`
	Messages []json.RawMessage `json:"messages"`
}

func (s *ArchiveService) FetchPage(ctx context.Context, startSeq uint64, limit uint32) (ArchivePage, error) {
	if ctx == nil {
		return ArchivePage{}, errors.New("context is required")
	}
	if limit == 0 || limit > 1000 {
		return ArchivePage{}, errors.New("limit must be between 1 and 1000")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetchPageLocked(ctx, startSeq, limit)
}
```

`fetchPageLocked` 使用以下完整流程；任一步失败均返回错误，不写持久 seq：

```go
func (s *ArchiveService) fetchPageLocked(ctx context.Context, startSeq uint64, limit uint32) (ArchivePage, error) {
	if err := ctx.Err(); err != nil {
		return ArchivePage{}, err
	}
	value, err := s.sdk.GetChatData(startSeq, limit, s.timeoutSeconds)
	if err != nil {
		return ArchivePage{}, err
	}
	var response getChatDataResponse
	if err := json.Unmarshal(value, &response); err != nil {
		return ArchivePage{}, fmt.Errorf("parse GetChatData response: %w", err)
	}
	if response.ErrCode != 0 {
		return ArchivePage{}, SDKError{Operation: "GetChatData", Code: response.ErrCode}
	}
	page := ArchivePage{StartSeq: startSeq, NextSeq: startSeq, Messages: make([]json.RawMessage, 0, len(response.ChatData))}
	for _, item := range response.ChatData {
		randomKey, err := decryptRandomKey(s.privateKey, item.EncryptedRandomKey)
		if err != nil {
			return ArchivePage{}, fmt.Errorf("decrypt random key for seq %d: %w", item.Seq, err)
		}
		plain, err := s.sdk.DecryptData(string(randomKey), item.EncryptedChatMsg)
		if err != nil {
			return ArchivePage{}, fmt.Errorf("decrypt chat message for seq %d: %w", item.Seq, err)
		}
		var message map[string]any
		if err := json.Unmarshal(plain, &message); err != nil {
			return ArchivePage{}, fmt.Errorf("parse chat message for seq %d: %w", item.Seq, err)
		}
		message["seq"] = item.Seq
		if strings.TrimSpace(fmt.Sprint(message["msgid"])) == "" {
			message["msgid"] = item.MsgID
		}
		raw, err := json.Marshal(message)
		if err != nil {
			return ArchivePage{}, fmt.Errorf("encode chat message for seq %d: %w", item.Seq, err)
		}
		page.Messages = append(page.Messages, raw)
		if item.Seq > page.NextSeq {
			page.NextSeq = item.Seq
		}
	}
	return page, nil
}
```

- [ ] **Step 4: 写 bridge handler 失败测试**

覆盖：无 Bearer 为 401、错误 CorpID 为 400、正确请求为 200、错误响应不包含 Secret/私钥/完整 SDK 配置：

```go
request := httptest.NewRequest(http.MethodPost, "/work-message/archive/messages",
	strings.NewReader(`{"corp_id":4,"wx_corpid":"ww-live","seq":40,"limit":10}`))
request.Header.Set("Authorization", "Bearer "+config.AdminToken)
recorder := httptest.NewRecorder()
handler.ServeHTTP(recorder, request)
if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"msgid":"msg-live-41"`) {
	t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
}
```

- [ ] **Step 5: 实现 bridge handler**

在 `NewAdminHandler` 注册：

```go
mux.HandleFunc("POST /work-message/archive/messages", func(w http.ResponseWriter, r *http.Request) {
	if archive == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"errcode": 409, "errmsg": "archive pull is not configured"})
		return
	}
	var input struct {
		CorpID   int    `json:"corp_id"`
		WXCorpID string `json:"wx_corpid"`
		Seq      uint64 `json:"seq"`
		Limit    uint32 `json:"limit"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input) != nil ||
		strings.TrimSpace(input.WXCorpID) != strings.TrimSpace(config.CorpID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errcode": 400, "errmsg": "archive corp binding mismatch"})
		return
	}
	page, err := archive.FetchPage(r.Context(), input.Seq, input.Limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"errcode": 502, "errmsg": sanitizeArchiveError(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "errmsg": "ok", "messages": page.Messages})
})

func sanitizeArchiveError(err error) string {
	var sdkErr SDKError
	if errors.As(err, &sdkErr) {
		return fmt.Sprintf("%s failed with code %d", sdkErr.Operation, sdkErr.Code)
	}
	return "archive bridge request failed"
}
```

- [ ] **Step 6: 运行 Demo 全包测试**

Run: `go test ./internal/wecomarchivedemo ./cmd/wecom-archive-demo -count=1`

Expected: PASS。

- [ ] **Step 7: 提交 bridge 实现**

```bash
git add internal/wecomarchivedemo/archive.go internal/wecomarchivedemo/archive_test.go internal/wecomarchivedemo/server.go internal/wecomarchivedemo/server_test.go
git commit -m "feat: serve decrypted WeCom archive bridge pages"
```

### Task 4: `msgaudit_notify` 触发按企业增量同步

**Files:**
- Modify: `internal/dashboard/work_message_archive_sync_cron.go`
- Modify: `internal/dashboard/work_message_archive_sync_cron_test.go`
- Modify: `internal/dashboard/wework_callback_worker.go`
- Modify: `internal/dashboard/wework_callback_worker_test.go`
- Modify: `cmd/mochat-go/main.go`

**Interfaces:**
- Produces: `RunCorp(ctx context.Context, corpID int) error`。
- Produces: `WorkMessageArchiveSyncTrigger` 和 `WithArchiveSyncTrigger(...)`。

- [ ] **Step 1: 写按企业同步和互斥测试**

用两个企业的 fake store 调用 `RunCorp(ctx, 4)`，断言 bridge 只收到企业 4；再让两个 goroutine 同时调用，fake client 的最大并发必须为 1。

```go
if err := cron.RunCorp(context.Background(), 4); err != nil {
	t.Fatal(err)
}
if !reflect.DeepEqual(client.corpIDs, []int{4}) {
	t.Fatalf("synced corps=%v, want [4]", client.corpIDs)
}
if client.maxConcurrent != 1 {
	t.Fatalf("max concurrent=%d, want 1", client.maxConcurrent)
}
```

- [ ] **Step 2: 运行 cron 测试确认 RED**

Run: `go test ./internal/dashboard -run 'TestWorkMessageArchiveSyncCronRunCorp' -count=1`

Expected: FAIL，`RunCorp` 尚不存在。

- [ ] **Step 3: 实现共享互斥和按企业过滤**

在 cron 中增加 `mu sync.Mutex`；`RunOnce` 和 `RunCorp` 都在入口持锁。`RunCorp` 从 `WorkMessageArchiveEnabledCorps` 中只选择 `CorpID == corpID`，不存在或未启用时返回 nil 并记录 `limited` 日志，不推进任何游标。

- [ ] **Step 4: 写事件触发失败测试**

```go
trigger := &archiveTriggerSpy{}
worker := NewWeWorkCallbackWorker(queue, store, client, "", logger).WithArchiveSyncTrigger(trigger)
err := worker.Process(context.Background(), WeWorkCallbackEvent{
	CorpID: 4, EventPath: "event.msgaudit_notify",
})
if err != nil || !reflect.DeepEqual(trigger.corpIDs, []int{4}) {
	t.Fatalf("err=%v trigger=%v", err, trigger.corpIDs)
}
```

另测 trigger 返回错误时 `Process` 返回错误，触发现有队列重试；未注入 trigger 时返回 nil，确保未启用 archive 的企业不进入死信。

- [ ] **Step 5: 实现 worker 注入点**

```go
type WorkMessageArchiveSyncTrigger interface {
	RunCorp(context.Context, int) error
}

func (w *WeWorkCallbackWorker) WithArchiveSyncTrigger(trigger WorkMessageArchiveSyncTrigger) *WeWorkCallbackWorker {
	w.archiveSyncTrigger = trigger
	return w
}
```

在 `Process` switch 中增加：

```go
case "event.msgaudit_notify":
	if w.archiveSyncTrigger == nil {
		return nil
	}
	return w.archiveSyncTrigger.RunCorp(ctx, corpID)
```

- [ ] **Step 6: 在 main 中共享同一个 cron**

创建一次：

```go
var workMessageArchiveCron *dashboard.WorkMessageArchiveSyncCron
if cfg.EnableWorkMessageArchiveSyncCron {
	workMessageArchiveCron = dashboard.NewWorkMessageArchiveSyncCron(
		getMySQLStore(),
		dashboard.NewWorkMessageArchiveBridgeClient(cfg.WorkMessageArchiveBridgeBaseURL, cfg.WorkMessageArchiveBridgeToken),
		log.Default(),
	).WithLimit(cfg.WorkMessageArchiveSyncLimit)
}
```

worker 启用时仅在 `workMessageArchiveCron != nil` 时调用 `WithArchiveSyncTrigger(workMessageArchiveCron)`，避免把 typed nil 指针装入接口；周期任务注册同一个对象的 `RunOnce`：

```go
worker := dashboard.NewWeWorkCallbackWorker(
	getRedisStore(), getMySQLStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), "", log.Default(),
)
if workMessageArchiveCron != nil {
	worker.WithArchiveSyncTrigger(workMessageArchiveCron)
}
if workMessageArchiveCron != nil {
	workerGroup.Add("cron-work-message-archive-sync", taskrunner.Periodic(taskrunner.PeriodicConfig{
		Name: "cron-work-message-archive-sync", Interval: cfg.WorkMessageArchiveSyncCronInterval,
		RunOnStart: cfg.WorkMessageArchiveSyncCronRunOnStart, Logger: log.Default(),
	}, workMessageArchiveCron.RunOnce))
}
```

- [ ] **Step 7: 运行相关包测试**

Run: `go test ./internal/dashboard ./cmd/mochat-go -run 'WeWorkCallback|WorkMessageArchive' -count=1`

Expected: PASS。

- [ ] **Step 8: 提交事件触发链路**

```bash
git add internal/dashboard/work_message_archive_sync_cron.go internal/dashboard/work_message_archive_sync_cron_test.go internal/dashboard/wework_callback_worker.go internal/dashboard/wework_callback_worker_test.go cmd/mochat-go/main.go
git commit -m "feat: trigger archive sync from WeCom notifications"
```

### Task 5: 本地完整门禁与发布物

**Files:**
- Verify: `scripts/build_wecom_archive_demo.ps1`
- Verify: `scripts/standalone_acceptance.sh`

**Interfaces:**
- Produces: 可追溯 MoChat Go 镜像、archive Demo 镜像及 SHA/摘要。

- [ ] **Step 1: 运行格式和单元测试**

Run: `gofmt -w internal/server/server.go internal/server/server_test.go internal/dashboard/work_message_archive_sync_cron.go internal/dashboard/work_message_archive_sync_cron_test.go internal/dashboard/wework_callback_worker.go internal/dashboard/wework_callback_worker_test.go internal/wecomarchivedemo/archive.go internal/wecomarchivedemo/archive_test.go internal/wecomarchivedemo/server.go internal/wecomarchivedemo/server_test.go cmd/mochat-go/main.go`

Run: `go test ./internal/server ./internal/dashboard ./internal/wecomarchivedemo ./cmd/wecom-archive-demo ./cmd/mochat-go -count=1`

Expected: 全部 PASS。

- [ ] **Step 2: 运行 archive smoke**

Run: `bash ./scripts/smoke_work_message_archive_sync_cron.sh`

Expected: fake bridge、真实 MySQL、seq、幂等和敏感词消费全部 PASS。

- [ ] **Step 3: 构建 Linux/amd64 发布物**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\build_wecom_archive_demo.ps1`

Run: `$sha = git rev-parse --short=12 HEAD; docker build --platform linux/amd64 -t "mochat-go:$sha" .`

Expected: SDK load check PASS；记录源码 SHA、镜像 ID 和 RepoDigest，不包含 `.git`、`node_modules`、未跟踪用户文件或凭据。

- [ ] **Step 4: 保存发布物与摘要**

Run: `$sha = git rev-parse --short=12 HEAD; docker save -o ".local-release/mochat-go-$sha.tar" "mochat-go:$sha"`

Run: `docker image inspect "mochat-go:$sha" --format '{{.Id}} {{join .RepoDigests ","}}'`

Expected: `.local-release` 已被 Git 忽略；tar 只包含镜像层，摘要写入本地验收记录，不把未跟踪工作区文件打包。

### Task 6: 服务器备份、部署和公网预验证

**Files:**
- Server only: `/opt/wecom-archive-demo/`
- Server only: 现有 MoChat Compose 与反向代理配置
- Evidence: `/opt/wecom-archive-demo/data/`

**Interfaces:**
- Produces: 公网专用 URL、内部 bridge、可恢复的旧镜像/配置快照。

- [ ] **Step 1: 交互式登录并创建回滚点**

在 Codex 打开的 SSH 终端执行 `ssh <文档确认的用户>@139.196.34.133`，由用户隐藏输入口令。只读记录：容器、镜像摘要、网络、卷、数据库、反向代理、出口 IP、`mc_corp.id=4` 的非敏感绑定字段。

Expected: `cid=4` 与当前测试 CorpID 一致；否则停止，不部署真实事件入口。

- [ ] **Step 2: 备份配置与镜像引用**

使用带 UTC 时间戳的服务器目录保存 Compose、反向代理配置、容器 inspect 和镜像摘要；密钥配置备份权限设为 `0600`，不下载到工作区。

Expected: 备份文件可读、旧镜像仍在本机、恢复命令可解析。

- [ ] **Step 3: 上传确定发布物并部署**

使用 `docker save`、交互式 `scp` 和服务器 `docker load` 传递 Task 5 的确定镜像；bridge 管理监听器仅加入 Docker 内部网络，不映射公网。主应用配置：

```text
MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=1
MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL=http://wecom-archive-demo:8081
MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS=60
```

Bearer token 只放服务器 secret/config，不能出现在命令参数或日志。

- [ ] **Step 4: 本机和公网合成验证**

使用服务器上临时、权限受限的合成脚本生成企业微信 GET/POST 加密请求，分别请求容器内部和公网：

```text
http://139.196.34.133/wecom/archive/callback?cid=4
```

Expected: GET 返回解密明文；POST 返回 `success`；任务账本出现 `event.msgaudit_notify`；无凭据进入日志。

- [ ] **Step 5: 重启后复验**

重启相关容器，检查 `/healthz`、`/readyz`、bridge 内部健康、回调公网合成请求和 archive cron 状态。

Expected: 全部恢复；seq 和证据目录保持不变。

### Task 7: 用户配合的真实企业微信试用

**Files:**
- Server secret/config only
- Evidence: `/opt/wecom-archive-demo/data/`

**Interfaces:**
- Consumes: 已通过合成门禁的专用 URL。
- Produces: 企业微信真实 URL 验证、真实事件和真实 SDK 消息证据。

- [ ] **Step 1: 交付后台填写值**

仅在 Task 6 全绿后，告诉用户填写 URL：

```text
http://139.196.34.133/wecom/archive/callback?cid=4
```

若编辑弹窗要求 Token/EncodingAESKey，由 MoChat 页面或交互式终端显示给用户复制；不得发到聊天或截图。

- [ ] **Step 2: 用户隐藏输入真实凭据**

Codex 打开配置脚本；用户在交互终端隐藏输入 CorpID 和会话内容存档 Secret。Codex 生成 RSA 2048 公钥，用户只把公钥粘贴到企业微信后台。

- [ ] **Step 3: 用户选择两名测试成员并开启试用**

用户确认范围、点击“立即开启”，再发送无隐私的唯一标记文本。至少覆盖员工双向单聊、知情测试客户双向单聊和一个满足条件的客户群。

- [ ] **Step 4: Codex 验证真实事件和拉取**

Expected:

- 真实 `event.msgaudit_notify` 到达；
- 回调 200；
- `GetChatData` 成功；
- 至少一条标记文本解密；
- MySQL 按 msgid/seq 幂等插入；
- 第二次拉取不重复；
- 重启后从已提交 seq 继续；
- Dashboard 回读与数据库一致。

- [ ] **Step 5: 记录失败与真实边界**

媒体未完成、客户未同意、开启范围不足、五日窗口、试用未生效或标准 API 域名阻断必须分别标记 `LIMITED`/`SKIP`/`BLOCKED_EXTERNAL`，不能用 mock 替代。

### Task 8: 中文交付记录与回滚演练

**Files:**
- Modify: `docs/runbooks/2026-08-18-wecom-archive-demo.zh-CN.md`
- Modify: `docs/phases/phase-7-wecom-archive/acceptance/2026-08-18-wecom-archive-demo.md`
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

**Interfaces:**
- Produces: 用户可审阅的完整中文证据与回滚步骤。

- [ ] **Step 1: 更新运行手册**

写明专用 URL、`cid` 含义、隐藏输入、RSA 公钥、两人范围、测试标记、错误码、证据目录和回滚命令类别；不写任何敏感值。

- [ ] **Step 2: 更新验收与总进度**

记录部署前后 Git SHA/镜像摘要、备份位置、真实事件时间、拉取 seq、消息类型、幂等、重启、Dashboard 回读、失败/跳过项和真实/模拟边界。

- [ ] **Step 3: 执行回滚可行性检查**

不实际破坏生产数据；验证旧镜像存在、旧 Compose/反向代理配置可读、关闭新 feature flag 可恢复旧行为、数据库迁移可向后兼容。

- [ ] **Step 4: 运行最终检查并提交文档**

Run: `git diff --check`

Run: `rg -n "填入密码|Secret:|PRIVATE KEY" docs/runbooks/2026-08-18-wecom-archive-demo.zh-CN.md docs/phases/phase-7-wecom-archive/acceptance/2026-08-18-wecom-archive-demo.md docs/PROJECT_PROGRESS.zh-CN.md`

Expected: 无占位符、无明文凭据；允许出现安全说明中的通用字段名，不出现实际值。

```bash
git add docs/runbooks/2026-08-18-wecom-archive-demo.zh-CN.md docs/phases/phase-7-wecom-archive/acceptance/2026-08-18-wecom-archive-demo.md docs/PROJECT_PROGRESS.zh-CN.md
git commit -m "docs: record WeCom archive live acceptance"
```

## 最终完成标准

- 企业微信后台实际保存专用 URL 成功。
- 真实 `msgaudit_notify`、真实 `GetChatData`、解密正文、seq、幂等、重启和 Dashboard 回读均有脱敏证据。
- 服务器重启后健康，部署可回滚。
- 任何未完成媒体、标准 API 或合规项均明确标注，不能宣称全量生产可用。
