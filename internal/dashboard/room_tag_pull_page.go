package dashboard

import "net/http"

func NewRoomTagPullContactDetailPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomTagPullContactDetailPage)
}

func ServeRoomTagPullContactDetailPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(roomTagPullContactDetailPageHTML))
}

const roomTagPullContactDetailPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 标签建群客户明细</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #fff;
      --line: #d9dee7;
      --text: #17202a;
      --muted: #667085;
      --primary: #0f766e;
      --primary-strong: #115e59;
      --ok: #027a48;
      --warning: #b54708;
      --info: #1849a9;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
    }
    main {
      width: min(1180px, calc(100vw - 32px));
      margin: 0 auto;
      padding: 28px 0 36px;
    }
    header {
      display: flex;
      align-items: flex-end;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 16px;
    }
    h1 {
      margin: 0;
      font-size: 24px;
      letter-spacing: 0;
    }
    h2 {
      margin: 0 0 10px;
      font-size: 16px;
      letter-spacing: 0;
    }
    .subtitle {
      margin: 6px 0 0;
      color: var(--muted);
      font-size: 13px;
      line-height: 1.5;
    }
    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 14px;
      margin-bottom: 12px;
    }
    .tokenbar {
      display: grid;
      grid-template-columns: minmax(260px, 1fr) 132px 120px;
      gap: 10px;
      align-items: end;
    }
    .filterbar {
      display: grid;
      grid-template-columns: minmax(160px, 1fr) minmax(140px, .8fr) 132px 132px 132px;
      gap: 10px;
      align-items: end;
    }
    label {
      display: grid;
      gap: 6px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 650;
    }
    input, select, button {
      border: 1px solid var(--line);
      border-radius: 6px;
      font: inherit;
      letter-spacing: 0;
    }
    input, select {
      height: 38px;
      padding: 0 10px;
      color: var(--text);
      background: #fff;
    }
    button {
      height: 38px;
      padding: 0 14px;
      background: var(--primary);
      border-color: var(--primary);
      color: #fff;
      font-weight: 700;
      cursor: pointer;
      white-space: nowrap;
    }
    button:hover { background: var(--primary-strong); }
    button.secondary {
      background: #fff;
      color: var(--primary-strong);
      border-color: #99d6cf;
    }
    .status {
      min-height: 32px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      color: var(--muted);
      font-size: 13px;
      margin: 4px 0 10px;
    }
    .status strong { color: var(--text); }
    .stats {
      display: grid;
      grid-template-columns: repeat(6, minmax(110px, 1fr));
      gap: 10px;
      margin-bottom: 12px;
    }
    .stat {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fff;
      padding: 10px;
    }
    .stat span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 5px;
    }
    .stat strong {
      font-size: 20px;
      letter-spacing: 0;
    }
    .tablewrap {
      overflow-x: auto;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fff;
    }
    table {
      width: 100%;
      min-width: 940px;
      border-collapse: collapse;
      table-layout: fixed;
    }
    th, td {
      padding: 11px 10px;
      border-bottom: 1px solid var(--line);
      text-align: left;
      vertical-align: middle;
      font-size: 13px;
      overflow-wrap: anywhere;
    }
    th {
      color: var(--muted);
      background: #fbfcfe;
      font-size: 12px;
      font-weight: 750;
    }
    tr:last-child td { border-bottom: 0; }
    .pill {
      display: inline-flex;
      align-items: center;
      height: 24px;
      padding: 0 8px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 750;
      background: #eef4ff;
      color: var(--info);
      white-space: nowrap;
    }
    .pill.on { background: #ecfdf3; color: var(--ok); }
    .pill.off { background: #fff3e8; color: var(--warning); }
    .empty {
      padding: 30px 16px;
      color: var(--muted);
      text-align: center;
    }
    @media (max-width: 820px) {
      main { width: min(100vw - 20px, 1180px); padding-top: 18px; }
      header { display: block; }
      h1 { font-size: 20px; }
      .tokenbar, .filterbar, .stats { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 标签建群客户明细</h1>
        <p class="subtitle">独立 Go 页面，接管旧前端空路由并读取标签建群活动、客户发送结果和员工任务数据。</p>
      </div>
    </header>

    <section class="panel">
      <h2>访问凭证</h2>
      <div class="tokenbar">
        <label>Dashboard Token
          <input id="tokenInput" placeholder="Bearer ..." autocomplete="off">
        </label>
        <button id="saveToken" type="button">保存</button>
        <button id="reload" class="secondary" type="button">刷新</button>
      </div>
    </section>

    <section class="panel">
      <h2>筛选</h2>
      <div class="filterbar">
        <label>客户昵称
          <input id="contactName" placeholder="客户昵称">
        </label>
        <label>发送状态
          <select id="sendStatus">
            <option value="">全部</option>
            <option value="0">待发送</option>
            <option value="1">已发送</option>
            <option value="2">发送失败</option>
          </select>
        </label>
        <label>入群状态
          <select id="joinStatus">
            <option value="">全部</option>
            <option value="0">未入群</option>
            <option value="1">已入群</option>
          </select>
        </label>
        <button id="applyFilter" type="button">查询客户</button>
        <button id="loadTasks" class="secondary" type="button">员工任务</button>
      </div>
    </section>

    <section class="panel">
      <div class="status">
        <span id="activityName">活动加载中</span>
        <strong id="statusText">准备加载</strong>
      </div>
      <div class="stats" id="stats"></div>
    </section>

    <section class="panel">
      <h2>客户明细</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th>客户</th>
              <th>跟进员工</th>
              <th>发送状态</th>
              <th>目标群</th>
              <th>入群状态</th>
            </tr>
          </thead>
          <tbody id="contactRows"></tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <h2>员工任务</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th>员工</th>
              <th>企微 UserID</th>
              <th>任务状态</th>
              <th>应发送客户</th>
              <th>已邀请</th>
              <th>任务数</th>
            </tr>
          </thead>
          <tbody id="taskRows"></tbody>
        </table>
      </div>
    </section>
  </main>
  <script>
    const params = new URLSearchParams(window.location.search);
    const activityId = params.get("id") || params.get("roomTagPullId") || "917001";
    const tokenInput = document.getElementById("tokenInput");
    const statusText = document.getElementById("statusText");
    const activityName = document.getElementById("activityName");
    const stats = document.getElementById("stats");
    const contactRows = document.getElementById("contactRows");
    const taskRows = document.getElementById("taskRows");

    function readStoredToken() {
      const keys = ["mochat_go_room_tag_pull_token", "ACCESS_TOKEN"];
      for (const key of keys) {
        const raw = localStorage.getItem(key);
        if (!raw) continue;
        try {
          const parsed = JSON.parse(raw);
          if (typeof parsed === "string" && parsed.trim()) return parsed.trim();
        } catch (_) {
          if (raw.trim()) return raw.trim();
        }
      }
      return "";
    }

    function headers() {
      const token = tokenInput.value.trim() || readStoredToken();
      const result = { "Accept": "application/json" };
      if (token) result.Authorization = token;
      return result;
    }

    function esc(value) {
      return String(value == null ? "" : value)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;");
    }

    function pill(text, on) {
      return '<span class="pill ' + (on ? "on" : "off") + '">' + esc(text) + '</span>';
    }

    async function api(path) {
      const response = await fetch(path, { headers: headers(), credentials: "same-origin" });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok || payload.code >= 400) {
        throw new Error(payload.msg || payload.message || response.statusText || "请求失败");
      }
      return payload.data;
    }

    function setBusy(text) {
      statusText.textContent = text;
    }

    function renderStats(detail) {
      const items = [
        ["已入群", detail.join_room_num],
        ["未入群", detail.no_join_room_num],
        ["已邀请", detail.invite_num],
        ["未邀请", detail.no_invite_num],
        ["已发送", detail.send_num],
        ["未发送", detail.no_send_num]
      ];
      stats.innerHTML = items.map(item => '<div class="stat"><span>' + esc(item[0]) + '</span><strong>' + esc(item[1] || 0) + '</strong></div>').join("");
    }

    function renderContacts(data) {
      const list = data.list || [];
      if (!list.length) {
        contactRows.innerHTML = '<tr><td colspan="5"><div class="empty">暂无客户明细</div></td></tr>';
        return;
      }
      contactRows.innerHTML = list.map(item => {
        const sent = Number(item.send_status) === 1;
        const joined = Number(item.is_join_room) === 1;
        return '<tr>' +
          '<td>' + esc(item.contact_name) + '</td>' +
          '<td>' + esc(item.employee_name) + '</td>' +
          '<td>' + pill(sent ? "已发送" : "未发送", sent) + '</td>' +
          '<td>' + esc(item.room_name) + '</td>' +
          '<td>' + pill(joined ? "已入群" : "未入群", joined) + '</td>' +
          '</tr>';
      }).join("");
    }

    function renderTasks(data) {
      const list = data.list || [];
      if (!list.length) {
        taskRows.innerHTML = '<tr><td colspan="6"><div class="empty">暂无员工任务</div></td></tr>';
        return;
      }
      taskRows.innerHTML = list.map(item => {
        const done = Number(item.status) === 1;
        return '<tr>' +
          '<td>' + esc(item.name) + '</td>' +
          '<td>' + esc(item.wxUserId) + '</td>' +
          '<td>' + pill(done ? "已完成" : "未完成", done) + '</td>' +
          '<td>' + esc(item.contact_num) + '</td>' +
          '<td>' + esc(item.invite_num) + '</td>' +
          '<td>' + esc(item.task_num) + '</td>' +
          '</tr>';
      }).join("");
    }

    function contactQuery() {
      const query = new URLSearchParams({ id: activityId, type: "1", page: "1", perPage: "20" });
      const contactName = document.getElementById("contactName").value.trim();
      const sendStatus = document.getElementById("sendStatus").value;
      const joinStatus = document.getElementById("joinStatus").value;
      if (contactName) query.set("contact_name", contactName);
      if (sendStatus !== "") query.set("send_status", sendStatus);
      if (joinStatus !== "") query.set("is_join_room", joinStatus);
      return query;
    }

    async function loadContacts() {
      setBusy("加载客户明细");
      const contacts = await api("/dashboard/roomTagPull/showContact?" + contactQuery().toString());
      renderContacts(contacts);
      setBusy("客户明细已加载");
    }

    async function loadTasks() {
      setBusy("加载员工任务");
      const tasks = await api("/dashboard/roomTagPull/showContact?id=" + encodeURIComponent(activityId) + "&type=2");
      renderTasks(tasks);
      setBusy("员工任务已加载");
    }

    async function loadAll() {
      try {
        setBusy("加载中");
        const detail = await api("/dashboard/roomTagPull/show?id=" + encodeURIComponent(activityId));
        activityName.textContent = "活动 #" + activityId;
        renderStats(detail);
        await Promise.all([loadContacts(), loadTasks()]);
        setBusy("加载完成");
      } catch (err) {
        setBusy(err.message || "加载失败");
      }
    }

    document.getElementById("saveToken").addEventListener("click", () => {
      localStorage.setItem("mochat_go_room_tag_pull_token", tokenInput.value.trim());
      loadAll();
    });
    document.getElementById("reload").addEventListener("click", loadAll);
    document.getElementById("applyFilter").addEventListener("click", loadContacts);
    document.getElementById("loadTasks").addEventListener("click", loadTasks);
    tokenInput.value = readStoredToken();
    loadAll();
  </script>
</body>
</html>`
