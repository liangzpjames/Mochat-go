package dashboard

import "net/http"

func NewRoomCalendarPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomCalendarPage)
}

func ServeRoomCalendarPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(roomCalendarPageHTML))
}

const roomCalendarPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 群日历</title>
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
      grid-template-columns: minmax(170px, 1fr) minmax(180px, 1fr) minmax(240px, 1.2fr) 150px;
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
      min-width: 960px;
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
        <h1>MoChat Go 群日历</h1>
        <p class="subtitle">独立 Go 控制台，直接操作群日历、启停状态和推送计划，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建群日历">
      <label>日历名称
        <input id="newName" placeholder="例如：新品活动群日历">
      </label>
      <label>推送时间
        <input id="newDay" type="datetime-local">
      </label>
      <label>推送内容
        <input id="newContent" placeholder="例如：明天 10 点活动开始">
      </label>
      <button id="createCalendar" type="button">新增日历</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>日历名称
        <input id="searchName" placeholder="按日历名称筛选">
      </label>
      <label>状态
        <select id="onOffFilter">
          <option value="-1">全部</option>
          <option value="1">启用</option>
          <option value="2">停用</option>
        </select>
      </label>
      <button id="searchCalendars" class="secondary" type="button">查询</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>群日历列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>名称</th>
              <th style="width:86px;">状态</th>
              <th style="width:88px;">群数</th>
              <th style="width:88px;">推送</th>
              <th style="width:150px;">创建人</th>
              <th style="width:150px;">创建时间</th>
              <th style="width:190px;">操作</th>
            </tr>
          </thead>
          <tbody id="calendarsBody"></tbody>
        </table>
        <div id="calendarsEmpty" class="empty">暂无群日历</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>推送计划</h2>
        <span id="detailTitle" class="subtitle">未选择日历</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>名称</th>
              <th style="width:170px;">推送时间</th>
              <th>内容</th>
              <th style="width:86px;">启停</th>
              <th style="width:86px;">状态</th>
            </tr>
          </thead>
          <tbody id="pushesBody"></tbody>
        </table>
        <div id="pushesEmpty" class="empty">暂无推送计划</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_room_calendar_token";
    var state = { total: 0, selectedCalendarID: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newName", "newDay", "newContent", "createCalendar", "searchName", "onOffFilter", "searchCalendars", "statusText", "summaryText", "calendarsBody", "calendarsEmpty", "detailTitle", "pushesBody", "pushesEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.newDay.value = defaultDayValue();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadCalendars);
      nodes.createCalendar.addEventListener("click", createCalendar);
      nodes.searchCalendars.addEventListener("click", loadCalendars);
      loadCalendars();
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

    function loadCalendars() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var name = (nodes.searchName.value || "").trim();
      if (name) params.set("name", name);
      var onOff = nodes.onOffFilter.value || "-1";
      if (onOff !== "-1") params.set("onOff", onOff);
      setStatus("正在加载群日历...");
      return api("/dashboard/roomCalendar/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.total = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderCalendars(list);
        setStatus("加载完成", "群日历 " + state.total + " 条");
        if (list.length > 0) {
          var firstID = calendarID(list[0]);
          if (firstID) return loadDetail(firstID);
        }
        renderPushes(null, []);
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function renderCalendars(list) {
      nodes.calendarsBody.innerHTML = "";
      nodes.calendarsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = calendarID(item);
        var enabled = calendarEnabled(item);
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td><span class=\"pill " + (enabled ? "on" : "off") + "\">" + (enabled ? "已启用" : "已停用") + "</span></td>" +
          "<td>" + escapeHTML(item.roomNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.pushNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.createUserName || "") + "</td>" +
          "<td>" + escapeHTML(item.createdAt || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"detail\">详情</button><button class=\"secondary\" data-action=\"toggle\">" + (enabled ? "停用" : "启用") + "</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"detail\"]").addEventListener("click", function () {
          loadDetail(id);
        });
        tr.querySelector("[data-action=\"toggle\"]").addEventListener("click", function () {
          toggleCalendar(id, enabled ? 2 : 1);
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deleteCalendar(id);
        });
        nodes.calendarsBody.appendChild(tr);
      });
    }

    function loadDetail(id) {
      if (!id) return Promise.resolve();
      state.selectedCalendarID = Number(id);
      setStatus("正在加载推送计划...");
      return api("/dashboard/roomCalendar/show?id=" + encodeURIComponent(id)).then(function (item) {
        var pushes = item && (item.pushList || item.push) || [];
        renderPushes(item, Array.isArray(pushes) ? pushes : []);
        setStatus("详情加载完成", "群日历 " + state.total + " 条");
      }).catch(function (err) {
        renderPushes(null, []);
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderPushes(calendar, pushes) {
      nodes.detailTitle.textContent = calendar && calendar.name ? calendar.name : "未选择日历";
      nodes.pushesBody.innerHTML = "";
      nodes.pushesEmpty.style.display = pushes.length > 0 ? "none" : "block";
      pushes.forEach(function (item) {
        var enabled = Number(item.onOff || item.on_off || 1) === 1;
        var active = Number(item.status || 1) === 1;
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(item.id || item.pushId || "") + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.day || "") + "</td>" +
          "<td>" + escapeHTML(compactJSON(item.pushContent || item.push_content || "")) + "</td>" +
          "<td><span class=\"pill " + (enabled ? "on" : "off") + "\">" + (enabled ? "启用" : "停用") + "</span></td>" +
          "<td><span class=\"pill " + (active ? "on" : "off") + "\">" + (active ? "有效" : "无效") + "</span></td>";
        nodes.pushesBody.appendChild(tr);
      });
    }

    function createCalendar() {
      var name = (nodes.newName.value || "").trim();
      if (!name) {
        setStatus("请输入日历名称");
        return;
      }
      var content = (nodes.newContent.value || "").trim();
      var day = normalizeDay(nodes.newDay.value);
      var pushes = [];
      if (content || day) {
        pushes.push({
          name: name + " 推送",
          day: day || defaultDayText(),
          pushContent: [{ type: "text", content: content || name }],
          onOff: 1,
          status: 1
        });
      }
      var payload = {
        name: name,
        rooms: [],
        onOff: 1,
        pushes: pushes
      };
      setStatus("正在新增群日历...");
      api("/dashboard/roomCalendar/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newName.value = "";
        nodes.newContent.value = "";
        nodes.newDay.value = defaultDayValue();
        return loadCalendars();
      }).then(function () {
        setStatus("新增完成", "群日历 " + state.total + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function toggleCalendar(id, onOff) {
      if (!id) return;
      setStatus("正在更新状态...");
      api("/dashboard/roomCalendar/update", {
        method: "PUT",
        body: JSON.stringify({ id: Number(id), roomCalendarId: Number(id), onOff: onOff })
      }).then(loadCalendars).catch(function (err) {
        setStatus("更新状态失败：" + err.message);
      });
    }

    function deleteCalendar(id) {
      if (!id || !window.confirm("确认删除这条群日历？")) return;
      setStatus("正在删除群日历...");
      api("/dashboard/roomCalendar/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), roomCalendarId: Number(id) })
      }).then(loadCalendars).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function calendarID(item) {
      return item.roomCalendarId || item.room_calendar_id || item.calendarId || item.id || "";
    }

    function calendarEnabled(item) {
      return Number(item.onOff || item.on_off || 1) === 1;
    }

    function defaultDayValue() {
      var date = new Date(Date.now() + 24 * 60 * 60 * 1000);
      date.setMinutes(30, 0, 0);
      var pad = function (n) { return String(n).padStart(2, "0"); };
      return date.getFullYear() + "-" + pad(date.getMonth() + 1) + "-" + pad(date.getDate()) + "T" + pad(date.getHours()) + ":" + pad(date.getMinutes());
    }

    function defaultDayText() {
      return normalizeDay(defaultDayValue());
    }

    function normalizeDay(value) {
      if (!value) return "";
      var text = String(value).replace("T", " ");
      return text.length === 16 ? text + ":00" : text;
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
