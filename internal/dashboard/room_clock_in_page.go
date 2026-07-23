package dashboard

import "net/http"

func NewRoomClockInPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomClockInPage)
}

func ServeRoomClockInPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(roomClockInPageHTML))
}

const roomClockInPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 群打卡</title>
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
      grid-template-columns: minmax(160px, 1fr) minmax(160px, 1fr) minmax(180px, 1fr) minmax(180px, 1fr) 118px 132px;
      gap: 10px;
      align-items: end;
    }
    .filterbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 150px 116px;
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
      min-width: 1040px;
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
    .pill.done { background: #eef4ff; color: var(--info); }
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
    @media (max-width: 900px) {
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
        <h1>MoChat Go 群打卡</h1>
        <p class="subtitle">独立 Go 控制台，直接操作群打卡活动、客户打卡记录和领奖状态，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建群打卡">
      <label>活动名称
        <input id="newName" placeholder="例如：7 天学习打卡">
      </label>
      <label>开始时间
        <input id="newStart" type="datetime-local">
      </label>
      <label>结束时间
        <input id="newEnd" type="datetime-local">
      </label>
      <label>活动类型
        <select id="newType">
          <option value="1">连续打卡</option>
          <option value="2">累计打卡</option>
        </select>
      </label>
      <label>奖励天数
        <input id="newTaskDay" type="number" min="1" step="1" value="1">
      </label>
      <button id="createClockIn" type="button">新增活动</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>活动名称
        <input id="searchName" placeholder="按活动名称筛选">
      </label>
      <label>状态
        <select id="statusFilter">
          <option value="">全部</option>
          <option value="1">进行中</option>
          <option value="0">已停用</option>
          <option value="2">已结束</option>
        </select>
      </label>
      <button id="searchClockIns" class="secondary" type="button">查询</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>群打卡列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>活动名称</th>
              <th style="width:90px;">类型</th>
              <th style="width:90px;">状态</th>
              <th style="width:88px;">客户数</th>
              <th style="width:88px;">打卡数</th>
              <th style="width:88px;">领奖数</th>
              <th style="width:130px;">开始时间</th>
              <th style="width:130px;">结束时间</th>
              <th style="width:220px;">操作</th>
            </tr>
          </thead>
          <tbody id="clockInsBody"></tbody>
        </table>
        <div id="clockInsEmpty" class="empty">暂无群打卡活动</div>
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
              <th>活动说明</th>
              <th>任务 JSON</th>
              <th>标签 JSON</th>
              <th>分享路径</th>
              <th style="width:130px;">更新时间</th>
            </tr>
          </thead>
          <tbody id="detailBody"></tbody>
        </table>
        <div id="detailEmpty" class="empty">暂无活动详情</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>参与客户</h2>
        <span id="contactsTitle" class="subtitle">未选择活动</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">记录 ID</th>
              <th>客户昵称</th>
              <th style="width:90px;">天数</th>
              <th style="width:90px;">状态</th>
              <th style="width:90px;">领奖</th>
              <th>标签 JSON</th>
              <th style="width:150px;">最后打卡</th>
              <th style="width:120px;">操作</th>
            </tr>
          </thead>
          <tbody id="contactsBody"></tbody>
        </table>
        <div id="contactsEmpty" class="empty">暂无参与客户</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>打卡明细</h2>
        <span id="daysTitle" class="subtitle">未选择客户</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:90px;">记录 ID</th>
              <th style="width:90px;">活动 ID</th>
              <th style="width:100px;">客户记录</th>
              <th>UnionID</th>
              <th style="width:150px;">打卡日期</th>
              <th style="width:150px;">更新时间</th>
            </tr>
          </thead>
          <tbody id="daysBody"></tbody>
        </table>
        <div id="daysEmpty" class="empty">暂无打卡明细</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_room_clock_in_token";
    var state = { total: 0, selectedID: 0, selectedContactID: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newName", "newStart", "newEnd", "newType", "newTaskDay", "createClockIn", "searchName", "statusFilter", "searchClockIns", "statusText", "summaryText", "clockInsBody", "clockInsEmpty", "detailTitle", "detailBody", "detailEmpty", "contactsTitle", "contactsBody", "contactsEmpty", "daysTitle", "daysBody", "daysEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      setDefaultTimes();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadClockIns);
      nodes.searchClockIns.addEventListener("click", loadClockIns);
      nodes.createClockIn.addEventListener("click", createClockIn);
      loadClockIns();
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

    function loadClockIns() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var name = (nodes.searchName.value || "").trim();
      if (name) params.set("activeName", name);
      var status = nodes.statusFilter.value;
      if (status !== "") params.set("status", status);
      setStatus("正在加载群打卡活动...");
      return api("/dashboard/roomClockIn/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.total = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderClockIns(list);
        setStatus("加载完成", "活动 " + state.total + " 条");
        if (list.length > 0) {
          var firstID = clockInID(list[0]);
          if (firstID) return loadDetail(firstID);
        }
        renderDetail(null);
        renderContacts([]);
        renderDays([]);
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function renderClockIns(list) {
      nodes.clockInsBody.innerHTML = "";
      nodes.clockInsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = clockInID(item);
        var status = Number(item.status || 0);
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.activeName || item.active_name || item.name || "") + "</td>" +
          "<td>" + escapeHTML(typeText(item.type)) + "</td>" +
          "<td><span class=\"pill " + statusClass(status) + "\">" + escapeHTML(item.statusText || statusText(status)) + "</span></td>" +
          "<td>" + escapeHTML(item.contactNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.clockInNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.receiveNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.startTime || item.start_time || "") + "</td>" +
          "<td>" + escapeHTML(item.endTime || item.end_time || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"detail\">详情</button><button class=\"secondary\" data-action=\"status\">" + (status === 1 ? "停用" : "启用") + "</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"detail\"]").addEventListener("click", function () {
          loadDetail(id);
        });
        tr.querySelector("[data-action=\"status\"]").addEventListener("click", function () {
          updateStatus(id, status === 1 ? 0 : 1);
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deleteClockIn(id);
        });
        nodes.clockInsBody.appendChild(tr);
      });
    }

    function loadDetail(id) {
      if (!id) return Promise.resolve();
      state.selectedID = Number(id);
      setStatus("正在加载活动详情...");
      return api("/dashboard/roomClockIn/show?id=" + encodeURIComponent(id)).then(function (item) {
        renderDetail(item);
        setStatus("详情加载完成", "活动 " + state.total + " 条");
        return loadContacts(id);
      }).catch(function (err) {
        renderDetail(null);
        renderContacts([]);
        renderDays([]);
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderDetail(item) {
      var detail = item && item.clockIn ? item.clockIn : item;
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = detail ? "none" : "block";
      nodes.detailTitle.textContent = detail && (detail.activeName || detail.active_name) ? (detail.activeName || detail.active_name) : "未选择活动";
      if (!detail) return;
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(clockInID(detail)) + "</td>" +
        "<td>" + escapeHTML(detail.description || "") + "</td>" +
        "<td>" + escapeHTML(compactJSON(detail.tasks || [])) + "</td>" +
        "<td>" + escapeHTML(compactJSON(detail.contactTags || detail.contact_tags || [])) + "</td>" +
        "<td>" + escapeHTML(item.shareUrl || item.link || item.url || "") + "</td>" +
        "<td>" + escapeHTML(item.updatedAt || item.updated_at || "") + "</td>";
      nodes.detailBody.appendChild(tr);
    }

    function loadContacts(id) {
      if (!id) return Promise.resolve();
      var params = new URLSearchParams();
      params.set("clockInId", String(id));
      params.set("page", "1");
      params.set("perPage", "20");
      return api("/dashboard/roomClockIn/showContact?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        renderContacts(list);
        if (list.length > 0) {
          var firstID = contactRecordID(list[0]);
          if (firstID) return loadDayDetail(id, firstID, list[0].nickname || list[0].name || "");
        }
        renderDays([]);
      }).catch(function (err) {
        renderContacts([]);
        renderDays([]);
        setStatus("客户列表加载失败：" + err.message);
      });
    }

    function renderContacts(list) {
      nodes.contactsBody.innerHTML = "";
      nodes.contactsEmpty.style.display = list.length > 0 ? "none" : "block";
      nodes.contactsTitle.textContent = state.selectedID ? ("活动 " + state.selectedID) : "未选择活动";
      list.forEach(function (item) {
        var id = contactRecordID(item);
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.nickname || item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.dayCount || item.day_count || 0) + "</td>" +
          "<td><span class=\"pill " + (Number(item.status || 0) === 1 ? "done" : "off") + "\">" + escapeHTML(item.statusText || contactStatusText(item.status)) + "</span></td>" +
          "<td>" + escapeHTML(Number(item.writeOff || item.write_off || 0) === 1 ? "已核销" : "未核销") + "</td>" +
          "<td>" + escapeHTML(compactJSON(item.contactTags || item.contact_tags || [])) + "</td>" +
          "<td>" + escapeHTML(item.lastClockAt || item.last_clock_at || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"days\">明细</button></div></td>";
        tr.querySelector("[data-action=\"days\"]").addEventListener("click", function () {
          loadDayDetail(state.selectedID, id, item.nickname || item.name || "");
        });
        nodes.contactsBody.appendChild(tr);
      });
    }

    function loadDayDetail(clockInIDValue, contactID, contactName) {
      if (!clockInIDValue || !contactID) return Promise.resolve();
      state.selectedContactID = Number(contactID);
      var params = new URLSearchParams();
      params.set("clockInId", String(clockInIDValue));
      params.set("contactId", String(contactID));
      params.set("page", "1");
      params.set("perPage", "31");
      return api("/dashboard/roomClockIn/dayDetail?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        nodes.daysTitle.textContent = contactName ? contactName : ("客户记录 " + contactID);
        renderDays(list);
      }).catch(function (err) {
        renderDays([]);
        setStatus("打卡明细加载失败：" + err.message);
      });
    }

    function renderDays(list) {
      nodes.daysBody.innerHTML = "";
      nodes.daysEmpty.style.display = list.length > 0 ? "none" : "block";
      if (list.length === 0 && !state.selectedContactID) nodes.daysTitle.textContent = "未选择客户";
      list.forEach(function (item) {
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(item.id || "") + "</td>" +
          "<td>" + escapeHTML(item.clockInId || item.clock_in_id || "") + "</td>" +
          "<td>" + escapeHTML(item.contactId || item.contact_id || "") + "</td>" +
          "<td>" + escapeHTML(item.unionId || item.union_id || "") + "</td>" +
          "<td>" + escapeHTML(item.day || "") + "</td>" +
          "<td>" + escapeHTML(item.updatedAt || item.updated_at || "") + "</td>";
        nodes.daysBody.appendChild(tr);
      });
    }

    function createClockIn() {
      var name = (nodes.newName.value || "").trim();
      if (!name) {
        setStatus("请输入活动名称");
        return;
      }
      var day = Math.max(1, Number(nodes.newTaskDay.value || 1));
      var payload = {
        activeName: name,
        description: name + "说明",
        type: Number(nodes.newType.value || 1),
        startTime: normalizeDateTime(nodes.newStart.value),
        endTime: normalizeDateTime(nodes.newEnd.value),
        tasks: [{ day: day, name: "第" + day + "天打卡", prize: "资料包" }],
        employeeQrcode: "",
        contactTags: [],
        corpCardStatus: 0,
        corpCard: {},
        status: 1
      };
      setStatus("正在新增群打卡活动...");
      api("/dashboard/roomClockIn/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newName.value = "";
        nodes.newTaskDay.value = "1";
        setDefaultTimes();
        return loadClockIns();
      }).then(function () {
        setStatus("新增完成", "活动 " + state.total + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function updateStatus(id, status) {
      if (!id) return;
      setStatus("正在更新活动状态...");
      api("/dashboard/roomClockIn/update", {
        method: "PUT",
        body: JSON.stringify({ id: Number(id), clockInId: Number(id), status: status })
      }).then(loadClockIns).catch(function (err) {
        setStatus("更新失败：" + err.message);
      });
    }

    function deleteClockIn(id) {
      if (!id || !window.confirm("确认删除这条群打卡活动？")) return;
      setStatus("正在删除群打卡活动...");
      api("/dashboard/roomClockIn/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), clockInId: Number(id) })
      }).then(loadClockIns).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function setDefaultTimes() {
      var start = new Date();
      var end = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000);
      nodes.newStart.value = toLocalInputValue(start);
      nodes.newEnd.value = toLocalInputValue(end);
    }

    function toLocalInputValue(date) {
      function pad(num) { return String(num).padStart(2, "0"); }
      return date.getFullYear() + "-" + pad(date.getMonth() + 1) + "-" + pad(date.getDate()) + "T" + pad(date.getHours()) + ":" + pad(date.getMinutes());
    }

    function normalizeDateTime(value) {
      if (!value) return "";
      return value.replace("T", " ") + ":00";
    }

    function clockInID(item) {
      return item.clockInId || item.clock_in_id || item.activityId || item.activity_id || item.id || "";
    }

    function contactRecordID(item) {
      return item.contactRecordId || item.id || item.contact_record_id || "";
    }

    function typeText(value) {
      return Number(value || 1) === 2 ? "累计" : "连续";
    }

    function statusClass(value) {
      if (Number(value) === 1) return "on";
      if (Number(value) === 2) return "done";
      return "off";
    }

    function statusText(value) {
      if (Number(value) === 0) return "已停用";
      if (Number(value) === 2) return "已结束";
      return "进行中";
    }

    function contactStatusText(value) {
      return Number(value) === 1 ? "已完成" : "未完成";
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
