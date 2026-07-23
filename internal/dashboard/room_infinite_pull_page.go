package dashboard

import "net/http"

func NewRoomInfinitePullPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomInfinitePullPage)
}

func ServeRoomInfinitePullPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(roomInfinitePullPageHTML))
}

const roomInfinitePullPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 无限拉群</title>
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
      --danger: #b42318;
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
    .createbar {
      display: grid;
      grid-template-columns: minmax(160px, 1fr) minmax(170px, 1fr) minmax(210px, 1.2fr) 110px 132px;
      gap: 10px;
      align-items: end;
    }
    .filterbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 116px;
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
    .check {
      min-height: 38px;
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 0 9px;
      border: 1px solid var(--line);
      border-radius: 6px;
      color: var(--text);
      background: #fff;
      font-size: 13px;
      font-weight: 600;
    }
    .check input {
      width: 16px;
      height: 16px;
      padding: 0;
    }
    input, button {
      border: 1px solid var(--line);
      border-radius: 6px;
      font: inherit;
      letter-spacing: 0;
    }
    input {
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
    button.danger {
      background: #fff;
      color: var(--danger);
      border-color: #f2b8b5;
    }
    button:disabled { opacity: .55; cursor: not-allowed; }
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
    .tablewrap {
      overflow-x: auto;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fff;
    }
    table {
      width: 100%;
      min-width: 980px;
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
      margin: 0 4px 4px 0;
    }
    .pill.on { background: #ecfdf3; color: var(--ok); }
    .pill.off { background: #fff3e8; color: var(--warning); }
    .empty {
      padding: 30px 16px;
      color: var(--muted);
      text-align: center;
    }
    .actions {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
    }
    .detailhead {
      display: flex;
      justify-content: space-between;
      gap: 12px;
      align-items: center;
      margin-bottom: 10px;
    }
    @media (max-width: 780px) {
      main { width: min(100vw - 20px, 1180px); padding-top: 18px; }
      header, .detailhead { display: block; }
      h1 { font-size: 20px; }
      .tokenbar, .createbar, .filterbar { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 无限拉群</h1>
        <p class="subtitle">独立 Go 控制台，直接操作无限拉群活动、展示配置和企微活码，不依赖原 MoChat 前端源码。</p>
      </div>
      <button id="reload" type="button">刷新</button>
    </header>

    <section class="panel tokenbar" aria-label="鉴权">
      <label>Dashboard JWT
        <input id="token" type="password" autocomplete="off" placeholder="Bearer token 或纯 token">
      </label>
      <button id="saveToken" class="secondary" type="button">保存 Token</button>
      <button id="clearToken" class="secondary" type="button">清空</button>
    </section>

    <section class="panel createbar" aria-label="新建无限拉群">
      <label>活动名称
        <input id="newName" placeholder="例如：福利群自动拉群">
      </label>
      <label>群名称
        <input id="newTitle" placeholder="例如：福利群">
      </label>
      <label>入群引导语
        <input id="newDescribe" placeholder="例如：扫码进群领取资料">
      </label>
      <label>活码上限
        <input id="newLimit" type="number" min="1" step="1" value="200">
      </label>
      <button id="createPull" type="button">新增活动</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>活动名称
        <input id="searchName" placeholder="按活动名称筛选">
      </label>
      <button id="searchPulls" class="secondary" type="button">查询</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>无限拉群列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>名称</th>
              <th style="width:92px;">群名称</th>
              <th style="width:92px;">引导语</th>
              <th>企微活码</th>
              <th style="width:92px;">扫码人数</th>
              <th style="width:130px;">创建人</th>
              <th style="width:150px;">创建时间</th>
              <th style="width:250px;">操作</th>
            </tr>
          </thead>
          <tbody id="pullsBody"></tbody>
        </table>
        <div id="pullsEmpty" class="empty">暂无无限拉群活动</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>活动详情</h2>
        <span id="detailTitle" class="subtitle">未选择活动</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">ID</th>
              <th>群名称</th>
              <th>引导语</th>
              <th>企微活码 JSON</th>
              <th>访问路径</th>
              <th style="width:140px;">更新时间</th>
            </tr>
          </thead>
          <tbody id="detailBody"></tbody>
        </table>
        <div id="detailEmpty" class="empty">暂无活动详情</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_room_infinite_pull_token";
    var state = { total: 0, selectedID: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newName", "newTitle", "newDescribe", "newLimit", "createPull", "searchName", "searchPulls", "statusText", "summaryText", "pullsBody", "pullsEmpty", "detailTitle", "detailBody", "detailEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadPulls);
      nodes.searchPulls.addEventListener("click", loadPulls);
      nodes.createPull.addEventListener("click", createPull);
      loadPulls();
    }

    function discoverToken() {
      var saved = localStorage.getItem(tokenKey) || "";
      if (saved) return saved;
      var raw = localStorage.getItem("ACCESS_TOKEN") || "";
      if (!raw) return "";
      try {
        var parsed = JSON.parse(raw);
        if (typeof parsed === "string") return parsed;
        if (parsed && parsed.token) return parsed.token;
      } catch (err) {
        return raw;
      }
      return raw;
    }

    function normalizedToken() {
      var token = (nodes.token.value || "").trim();
      if (!token) return "";
      if (/^Bearer\s+/i.test(token)) return token;
      return "Bearer " + token;
    }

    function saveToken() {
      localStorage.setItem(tokenKey, nodes.token.value.trim());
      setStatus("Token 已保存");
    }

    function clearToken() {
      localStorage.removeItem(tokenKey);
      nodes.token.value = "";
      setStatus("Token 已清空");
    }

    function setStatus(text, strong) {
      nodes.statusText.textContent = text || "";
      nodes.summaryText.textContent = strong || "";
    }

    function api(path, options) {
      var opts = options || {};
      var headers = opts.headers || {};
      headers.Accept = "application/json";
      var token = normalizedToken();
      if (token) headers.Authorization = token;
      if (opts.body && !headers["Content-Type"]) headers["Content-Type"] = "application/json";
      opts.headers = headers;
      return fetch(path, opts).then(function (resp) {
        return resp.text().then(function (text) {
          var payload = {};
          if (text) {
            try { payload = JSON.parse(text); } catch (err) { payload = { msg: text }; }
          }
          if (!resp.ok || (payload.code && Number(payload.code) >= 400)) {
            throw new Error(payload.msg || ("HTTP " + resp.status));
          }
          return payload.data;
        });
      });
    }

    function loadPulls() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var name = (nodes.searchName.value || "").trim();
      if (name) params.set("name", name);
      setStatus("正在加载无限拉群活动...");
      return api("/dashboard/roomInfinitePull/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.total = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderPulls(list);
        setStatus("加载完成", "活动 " + state.total + " 条");
        if (list.length > 0) {
          var firstID = pullID(list[0]);
          if (firstID) return loadDetail(firstID);
        }
        renderDetail(null);
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function renderPulls(list) {
      nodes.pullsBody.innerHTML = "";
      nodes.pullsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = pullID(item);
        var titleOn = Number(item.titleStatus || item.title_status || 0) === 1;
        var describeOn = Number(item.describeStatus || item.describe_status || 0) === 1;
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td><span class=\"pill " + (titleOn ? "on" : "off") + "\">" + (titleOn ? "展示" : "隐藏") + "</span></td>" +
          "<td><span class=\"pill " + (describeOn ? "on" : "off") + "\">" + (describeOn ? "展示" : "隐藏") + "</span></td>" +
          "<td>" + escapeHTML(qwCodeSummary(item.qwCode || item.qw_code || [])) + "</td>" +
          "<td>" + escapeHTML(item.totalNum || item.total_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.createUserName || "") + "</td>" +
          "<td>" + escapeHTML(item.createdAt || item.created_at || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"detail\">详情</button><button class=\"secondary\" data-action=\"title\">" + (titleOn ? "隐藏群名" : "展示群名") + "</button><button class=\"secondary\" data-action=\"describe\">" + (describeOn ? "隐藏引导" : "展示引导") + "</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"detail\"]").addEventListener("click", function () {
          loadDetail(id);
        });
        tr.querySelector("[data-action=\"title\"]").addEventListener("click", function () {
          updatePull(id, { titleStatus: titleOn ? 0 : 1 });
        });
        tr.querySelector("[data-action=\"describe\"]").addEventListener("click", function () {
          updatePull(id, { describeStatus: describeOn ? 0 : 1 });
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deletePull(id);
        });
        nodes.pullsBody.appendChild(tr);
      });
    }

    function loadDetail(id) {
      if (!id) return Promise.resolve();
      state.selectedID = Number(id);
      setStatus("正在加载活动详情...");
      return api("/dashboard/roomInfinitePull/info?id=" + encodeURIComponent(id)).then(function (item) {
        renderDetail(item);
        setStatus("详情加载完成", "活动 " + state.total + " 条");
      }).catch(function (err) {
        renderDetail(null);
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderDetail(item) {
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = item ? "none" : "block";
      nodes.detailTitle.textContent = item && item.name ? item.name : "未选择活动";
      if (!item) return;
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(pullID(item)) + "</td>" +
        "<td>" + escapeHTML(item.title || "") + "</td>" +
        "<td>" + escapeHTML(item.describe || "") + "</td>" +
        "<td>" + escapeHTML(compactJSON(item.qwCode || item.qw_code || [])) + "</td>" +
        "<td>" + escapeHTML(item.link || "") + "</td>" +
        "<td>" + escapeHTML(item.updatedAt || item.updated_at || "") + "</td>";
      nodes.detailBody.appendChild(tr);
    }

    function createPull() {
      var name = (nodes.newName.value || "").trim();
      if (!name) {
        setStatus("请输入活动名称");
        return;
      }
      var title = (nodes.newTitle.value || "").trim();
      var describe = (nodes.newDescribe.value || "").trim();
      var limit = Math.max(1, Number(nodes.newLimit.value || 200));
      var payload = {
        name: name,
        avatar: "",
        titleStatus: 1,
        title: title || name,
        describeStatus: 1,
        describe: describe,
        logo: "",
        qwCode: [{ qrcode: "", upper_limit: limit, status: 1 }],
        totalNum: 0
      };
      setStatus("正在新增无限拉群活动...");
      api("/dashboard/roomInfinitePull/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newName.value = "";
        nodes.newTitle.value = "";
        nodes.newDescribe.value = "";
        nodes.newLimit.value = "200";
        return loadPulls();
      }).then(function () {
        setStatus("新增完成", "活动 " + state.total + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function updatePull(id, patch) {
      if (!id) return;
      patch.id = Number(id);
      patch.roomInfinitePullId = Number(id);
      setStatus("正在更新活动...");
      api("/dashboard/roomInfinitePull/update", {
        method: "PUT",
        body: JSON.stringify(patch)
      }).then(loadPulls).catch(function (err) {
        setStatus("更新失败：" + err.message);
      });
    }

    function deletePull(id) {
      if (!id || !window.confirm("确认删除这条无限拉群活动？")) return;
      setStatus("正在删除活动...");
      api("/dashboard/roomInfinitePull/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), roomInfinitePullId: Number(id) })
      }).then(loadPulls).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function pullID(item) {
      return item.roomInfinitePullId || item.room_infinite_pull_id || item.infiniteId || item.id || "";
    }

    function qwCodeSummary(value) {
      var list = Array.isArray(value) ? value : [];
      if (list.length === 0) return "0 个活码";
      var active = list.filter(function (item) { return Number(item.status || 0) === 1; }).length;
      return list.length + " 个活码，拉人中 " + active + " 个";
    }

    function compactJSON(value) {
      if (value == null || value === "") return "";
      if (typeof value === "string") {
        try { return JSON.stringify(JSON.parse(value)); } catch (err) { return value; }
      }
      return JSON.stringify(value);
    }

    function escapeHTML(value) {
      return String(value == null ? "" : value).replace(/[&<>"']/g, function (char) {
        return { "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" }[char];
      });
    }

    document.addEventListener("DOMContentLoaded", init);
  })();
  </script>
</body>
</html>`
