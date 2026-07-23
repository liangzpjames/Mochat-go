package dashboard

import "net/http"

func NewRoomQualityPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomQualityPage)
}

func ServeRoomQualityPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(roomQualityPageHTML))
}

const roomQualityPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 群质检</title>
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
      grid-template-columns: minmax(170px, 1fr) minmax(170px, 1fr) minmax(200px, 1.2fr) 132px;
      gap: 10px;
      align-items: end;
    }
    .filterbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 150px 116px 116px;
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
      .tokenbar, .createbar, .filterbar { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 群质检</h1>
        <p class="subtitle">独立 Go 控制台，直接操作群质检规则和触发客户记录，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建规则">
      <label>规则名称
        <input id="newName" placeholder="例如：群内敏感动作">
      </label>
      <label>规则说明
        <input id="newDescription" placeholder="例如：出现违规词后记录">
      </label>
      <label>触发关键词
        <input id="newKeyword" placeholder="例如：退款">
      </label>
      <button id="createRule" type="button">新增规则</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>规则名称
        <input id="searchName" placeholder="按规则名称筛选">
      </label>
      <label>状态
        <select id="statusFilter">
          <option value="-1">全部</option>
          <option value="1">启用</option>
          <option value="0">停用</option>
        </select>
      </label>
      <button id="searchRules" class="secondary" type="button">查询</button>
      <button id="reloadContacts" class="secondary" type="button">触发记录</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>质检规则</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>名称</th>
              <th>说明</th>
              <th style="width:180px;">规则</th>
              <th style="width:86px;">状态</th>
              <th style="width:88px;">群数</th>
              <th style="width:88px;">触发</th>
              <th style="width:150px;">创建时间</th>
              <th style="width:210px;">操作</th>
            </tr>
          </thead>
          <tbody id="rulesBody"></tbody>
        </table>
        <div id="rulesEmpty" class="empty">暂无群质检规则</div>
      </div>
    </section>

    <section class="panel">
      <h2>触发客户记录</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">记录ID</th>
              <th>客户</th>
              <th>客户群</th>
              <th>触发内容</th>
              <th style="width:90px;">消息类型</th>
              <th style="width:86px;">状态</th>
              <th style="width:150px;">触发时间</th>
            </tr>
          </thead>
          <tbody id="contactsBody"></tbody>
        </table>
        <div id="contactsEmpty" class="empty">暂无触发记录</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_room_quality_token";
    var state = { rulesTotal: 0, contactsTotal: 0, selectedQualityID: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newName", "newDescription", "newKeyword", "createRule", "searchName", "statusFilter", "searchRules", "reloadContacts", "statusText", "summaryText", "rulesBody", "rulesEmpty", "contactsBody", "contactsEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadAll);
      nodes.createRule.addEventListener("click", createRule);
      nodes.searchRules.addEventListener("click", loadRules);
      nodes.reloadContacts.addEventListener("click", function () { loadContacts(state.selectedQualityID); });
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
      setStatus("正在加载群质检数据...");
      return loadRules().then(function () {
        return loadContacts(state.selectedQualityID);
      }).then(function () {
        setStatus("加载完成", "规则 " + state.rulesTotal + " 条，触发 " + state.contactsTotal + " 条");
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
      var status = nodes.statusFilter.value || "-1";
      if (status !== "-1") params.set("status", status);
      return api("/dashboard/roomQuality/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.rulesTotal = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        if (!state.selectedQualityID && list.length > 0) {
          state.selectedQualityID = Number(list[0].roomQualityId || list[0].qualityId || list[0].id || 0);
        }
        renderRules(list);
      });
    }

    function loadContacts(qualityID) {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      if (qualityID) params.set("roomQualityId", String(qualityID));
      return api("/dashboard/roomQuality/showContact?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.contactsTotal = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderContacts(list);
      });
    }

    function renderRules(list) {
      nodes.rulesBody.innerHTML = "";
      nodes.rulesEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = item.roomQualityId || item.qualityId || item.id || "";
        var enabled = Number(item.status) !== 0;
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.description || "") + "</td>" +
          "<td>" + escapeHTML(compactJSON(item.rule)) + "</td>" +
          "<td><span class=\"pill " + (enabled ? "on" : "off") + "\">" + (item.statusText || (enabled ? "已启用" : "已停用")) + "</span></td>" +
          "<td>" + escapeHTML(item.roomNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.contactNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.createdAt || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"contacts\">记录</button><button class=\"secondary\" data-action=\"toggle\">" + (enabled ? "停用" : "启用") + "</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"contacts\"]").addEventListener("click", function () {
          state.selectedQualityID = Number(id);
          setStatus("正在加载规则 " + id + " 的触发记录...");
          loadContacts(id).then(function () {
            setStatus("触发记录已加载", "规则 " + state.rulesTotal + " 条，触发 " + state.contactsTotal + " 条");
          }).catch(function (err) {
            setStatus("加载触发记录失败：" + err.message);
          });
        });
        tr.querySelector("[data-action=\"toggle\"]").addEventListener("click", function () {
          toggleRule(id, enabled ? 0 : 1);
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deleteRule(id);
        });
        nodes.rulesBody.appendChild(tr);
      });
    }

    function renderContacts(list) {
      nodes.contactsBody.innerHTML = "";
      nodes.contactsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var handled = Number(item.status) === 1;
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(item.contactRecordId || item.id || "") + "</td>" +
          "<td>" + escapeHTML(item.nickname || item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.roomName || item.roomId || "") + "</td>" +
          "<td>" + escapeHTML(item.content || "") + "</td>" +
          "<td>" + escapeHTML(item.msgType || "") + "</td>" +
          "<td><span class=\"pill " + (handled ? "on" : "off") + "\">" + (handled ? "已处理" : "待处理") + "</span></td>" +
          "<td>" + escapeHTML(item.triggerAt || item.createdAt || "") + "</td>";
        nodes.contactsBody.appendChild(tr);
      });
    }

    function createRule() {
      var name = (nodes.newName.value || "").trim();
      if (!name) {
        setStatus("请输入规则名称");
        return;
      }
      var keyword = (nodes.newKeyword.value || "").trim();
      var rule = keyword ? [{ type: "keyword", keyword: keyword }] : [];
      var payload = {
        name: name,
        description: (nodes.newDescription.value || "").trim(),
        rule: rule,
        rooms: [],
        status: 1
      };
      setStatus("正在新增群质检规则...");
      api("/dashboard/roomQuality/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newName.value = "";
        nodes.newDescription.value = "";
        nodes.newKeyword.value = "";
        state.selectedQualityID = 0;
        return loadAll();
      }).then(function () {
        setStatus("新增完成", "规则 " + state.rulesTotal + " 条，触发 " + state.contactsTotal + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function toggleRule(id, status) {
      if (!id) return;
      setStatus("正在更新状态...");
      api("/dashboard/roomQuality/status", {
        method: "PUT",
        body: JSON.stringify({ id: Number(id), roomQualityId: Number(id), status: status })
      }).then(loadAll).catch(function (err) {
        setStatus("更新状态失败：" + err.message);
      });
    }

    function deleteRule(id) {
      if (!id || !window.confirm("确认删除这条群质检规则？")) return;
      setStatus("正在删除群质检规则...");
      api("/dashboard/roomQuality/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), roomQualityId: Number(id) })
      }).then(function () {
        state.selectedQualityID = 0;
        return loadAll();
      }).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function compactJSON(value) {
      if (value == null || value === "") return "";
      try {
        return JSON.stringify(value);
      } catch (err) {
        return String(value);
      }
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
