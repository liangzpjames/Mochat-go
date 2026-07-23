package dashboard

import "net/http"

func NewRoomRemindPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomRemindPage)
}

func ServeRoomRemindPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(roomRemindPageHTML))
}

const roomRemindPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 客户群提醒</title>
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
      grid-template-columns: minmax(180px, 1fr) minmax(180px, 1fr) 150px 116px 116px;
      gap: 10px;
      align-items: end;
    }
    .createbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) minmax(170px, 1fr) minmax(280px, 1.4fr) 132px;
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
    .checks {
      display: grid;
      grid-template-columns: repeat(5, minmax(76px, 1fr));
      gap: 8px;
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
    @media (max-width: 780px) {
      main { width: min(100vw - 20px, 1180px); padding-top: 18px; }
      header { display: block; }
      h1 { font-size: 20px; }
      .tokenbar, .filterbar, .createbar, .checks { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 客户群提醒</h1>
        <p class="subtitle">独立 Go 控制台，直接操作客户群提醒规则、启停状态和任务查询，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建提醒">
      <label>规则名称
        <input id="newName" placeholder="例如：群内关键词提醒">
      </label>
      <label>关键词
        <input id="newKeyword" placeholder="例如：退款">
      </label>
      <label>触发类型
        <span class="checks">
          <span class="check"><input id="newIsKeyword" type="checkbox" checked>关键词</span>
          <span class="check"><input id="newIsQrcode" type="checkbox" checked>二维码</span>
          <span class="check"><input id="newIsLink" type="checkbox">链接</span>
          <span class="check"><input id="newIsMiniprogram" type="checkbox">小程序</span>
          <span class="check"><input id="newIsCard" type="checkbox">名片</span>
        </span>
      </label>
      <button id="createRule" type="button">新增提醒</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>规则名称
        <input id="searchName" placeholder="按规则名称筛选">
      </label>
      <label>提醒关键词
        <input id="searchKeyword" placeholder="按提醒关键词筛选">
      </label>
      <label>状态
        <select id="statusFilter">
          <option value="-1">全部</option>
          <option value="1">启用</option>
          <option value="0">停用</option>
        </select>
      </label>
      <button id="searchRules" class="secondary" type="button">查询</button>
      <button id="reloadTasks" class="secondary" type="button">任务视图</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2 style="margin:0 0 10px;font-size:16px;">提醒规则</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>名称</th>
              <th>关键词</th>
              <th style="width:170px;">触发类型</th>
              <th style="width:86px;">状态</th>
              <th style="width:88px;">群数</th>
              <th style="width:88px;">记录</th>
              <th style="width:150px;">创建时间</th>
              <th style="width:160px;">操作</th>
            </tr>
          </thead>
          <tbody id="rulesBody"></tbody>
        </table>
        <div id="rulesEmpty" class="empty">暂无提醒规则</div>
      </div>
    </section>

    <section class="panel">
      <h2 style="margin:0 0 10px;font-size:16px;">启用任务</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>名称</th>
              <th>关键词</th>
              <th style="width:170px;">触发类型</th>
              <th style="width:86px;">状态</th>
              <th style="width:88px;">群数</th>
              <th style="width:88px;">记录</th>
              <th style="width:150px;">更新时间</th>
            </tr>
          </thead>
          <tbody id="tasksBody"></tbody>
        </table>
        <div id="tasksEmpty" class="empty">暂无启用任务</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_room_remind_token";
    var state = { rulesTotal: 0, tasksTotal: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newName", "newKeyword", "newIsKeyword", "newIsQrcode", "newIsLink", "newIsMiniprogram", "newIsCard", "createRule", "searchName", "searchKeyword", "statusFilter", "searchRules", "reloadTasks", "statusText", "summaryText", "rulesBody", "rulesEmpty", "tasksBody", "tasksEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadAll);
      nodes.createRule.addEventListener("click", createRule);
      nodes.searchRules.addEventListener("click", loadRules);
      nodes.reloadTasks.addEventListener("click", loadTasks);
      loadAll();
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

    function loadAll() {
      setStatus("正在加载客户群提醒数据...");
      return Promise.all([loadRules(), loadTasks()]).then(function () {
        setStatus("加载完成", "规则 " + state.rulesTotal + " 条，启用任务 " + state.tasksTotal + " 条");
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function loadRules() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var name = (nodes.searchName.value || "").trim();
      if (name) params.set("name", name);
      var keyword = (nodes.searchKeyword.value || "").trim();
      if (keyword) params.set("remindKeyword", keyword);
      var status = nodes.statusFilter.value || "-1";
      if (status !== "-1") params.set("status", status);
      return api("/dashboard/roomRemind/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.rulesTotal = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderRows(nodes.rulesBody, nodes.rulesEmpty, list, true);
      });
    }

    function loadTasks() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      return api("/dashboard/task/roomRemind?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.tasksTotal = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderRows(nodes.tasksBody, nodes.tasksEmpty, list, false);
      });
    }

    function renderRows(body, empty, list, withActions) {
      body.innerHTML = "";
      empty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = item.roomRemindId || item.remindId || item.id || "";
        var enabled = Number(item.status) !== 0;
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.keyword || "") + "</td>" +
          "<td>" + renderTypePills(item.enabledTypes || []) + "</td>" +
          "<td><span class=\"pill " + (enabled ? "on" : "off") + "\">" + (item.statusText || (enabled ? "已启用" : "已停用")) + "</span></td>" +
          "<td>" + escapeHTML(item.roomNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.recordNum || item.messageRecordNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.createdAt || item.updatedAt || "") + "</td>" +
          (withActions ? "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"toggle\">" + (enabled ? "停用" : "启用") + "</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>" : "");
        if (withActions) {
          tr.querySelector("[data-action=\"toggle\"]").addEventListener("click", function () {
            toggleRule(id, enabled ? 0 : 1);
          });
          tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
            deleteRule(id);
          });
        }
        body.appendChild(tr);
      });
    }

    function renderTypePills(types) {
      var labels = {
        qrcode: "二维码",
        link: "链接",
        miniprogram: "小程序",
        card: "名片",
        keyword: "关键词"
      };
      if (!Array.isArray(types) || types.length === 0) return "<span class=\"pill off\">未配置</span>";
      return types.map(function (type) {
        return "<span class=\"pill\">" + escapeHTML(labels[type] || type) + "</span>";
      }).join("");
    }

    function createRule() {
      var name = (nodes.newName.value || "").trim();
      if (!name) {
        setStatus("请输入规则名称");
        return;
      }
      var payload = {
        name: name,
        keyword: (nodes.newKeyword.value || "").trim(),
        rooms: [],
        isKeyword: nodes.newIsKeyword.checked ? 1 : 0,
        isQrcode: nodes.newIsQrcode.checked ? 1 : 0,
        isLink: nodes.newIsLink.checked ? 1 : 0,
        isMiniprogram: nodes.newIsMiniprogram.checked ? 1 : 0,
        isCard: nodes.newIsCard.checked ? 1 : 0,
        status: 1
      };
      setStatus("正在新增提醒规则...");
      api("/dashboard/roomRemind/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newName.value = "";
        nodes.newKeyword.value = "";
        return loadAll();
      }).then(function () {
        setStatus("新增完成", "规则 " + state.rulesTotal + " 条，启用任务 " + state.tasksTotal + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function toggleRule(id, status) {
      if (!id) return;
      setStatus("正在更新状态...");
      api("/dashboard/roomRemind/status?id=" + encodeURIComponent(id) + "&status=" + encodeURIComponent(status)).then(loadAll).catch(function (err) {
        setStatus("更新状态失败：" + err.message);
      });
    }

    function deleteRule(id) {
      if (!id || !window.confirm("确认删除这条提醒规则？")) return;
      setStatus("正在删除提醒规则...");
      api("/dashboard/roomRemind/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), roomRemindId: Number(id) })
      }).then(loadAll).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
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
