package dashboard

import "net/http"

func NewSensitiveWordPageHandler() http.Handler {
	return http.HandlerFunc(ServeSensitiveWordPage)
}

func ServeSensitiveWordPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(sensitiveWordPageHTML))
}

const sensitiveWordPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 敏感词管理</title>
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
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
    }
    main {
      width: min(1200px, calc(100vw - 32px));
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
    .toolbar {
      display: grid;
      grid-template-columns: 190px minmax(180px, 1fr) 116px 116px;
      gap: 10px;
      align-items: end;
    }
    .createbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 130px minmax(240px, 1.4fr) 132px;
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
    input, select, textarea, button {
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
    textarea {
      min-height: 72px;
      padding: 9px 10px;
      resize: vertical;
      line-height: 1.45;
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
    .tabs {
      display: flex;
      gap: 8px;
      margin: 14px 0 12px;
    }
    .tab {
      background: #fff;
      color: var(--primary-strong);
      border-color: #99d6cf;
    }
    .tab.active {
      background: var(--primary);
      color: #fff;
      border-color: var(--primary);
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
      color: #1849a9;
      white-space: nowrap;
    }
    .pill.on { background: #ecfdf3; color: var(--ok); }
    .pill.off { background: #fff3e8; color: var(--warning); }
    .empty {
      padding: 34px 16px;
      color: var(--muted);
      text-align: center;
    }
    .hidden { display: none; }
    .actions {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
    }
    @media (max-width: 760px) {
      main { width: min(100vw - 20px, 1200px); padding-top: 18px; }
      header { display: block; }
      h1 { font-size: 20px; }
      .tokenbar, .toolbar, .createbar { grid-template-columns: 1fr; }
      button { width: 100%; }
      .tabs { display: grid; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 敏感词管理</h1>
        <p class="subtitle">独立 Go 控制台，直接操作敏感词词库、分组和触发监控，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建敏感词">
      <label>新分组
        <input id="newGroupName" placeholder="例如：默认分组">
      </label>
      <button id="createGroup" class="secondary" type="button">新建分组</button>
      <label>新增敏感词
        <textarea id="newWordNames" placeholder="可用逗号、顿号或换行批量输入"></textarea>
      </label>
      <button id="createWords" type="button">新增词库</button>
    </section>

    <section class="panel toolbar" aria-label="筛选">
      <label>分组
        <select id="groupFilter"></select>
      </label>
      <label>关键词
        <input id="keywordFilter" placeholder="搜索敏感词">
      </label>
      <button id="searchWords" class="secondary" type="button">筛选词库</button>
      <button id="reloadMonitor" class="secondary" type="button">刷新监控</button>
    </section>

    <div class="tabs" role="tablist">
      <button id="tabWords" class="tab active" type="button">敏感词词库</button>
      <button id="tabMonitor" class="tab" type="button">触发监控</button>
    </div>

    <div class="status">
      <span id="statusText">等待加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section id="wordsPanel">
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:90px;">ID</th>
              <th>敏感词</th>
              <th style="width:160px;">分组</th>
              <th style="width:100px;">状态</th>
              <th style="width:110px;">员工触发</th>
              <th style="width:110px;">客户触发</th>
              <th style="width:170px;">创建时间</th>
              <th style="width:180px;">操作</th>
            </tr>
          </thead>
          <tbody id="wordsBody"></tbody>
        </table>
        <div id="wordsEmpty" class="empty hidden">暂无敏感词</div>
      </div>
    </section>

    <section id="monitorPanel" class="hidden">
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:90px;">ID</th>
              <th>触发词</th>
              <th style="width:100px;">来源</th>
              <th style="width:150px;">触发人</th>
              <th>触发场景</th>
              <th style="width:170px;">触发时间</th>
              <th style="width:120px;">操作</th>
            </tr>
          </thead>
          <tbody id="monitorBody"></tbody>
        </table>
        <div id="monitorEmpty" class="empty hidden">暂无触发记录</div>
      </div>
      <div id="messagePanel" class="panel hidden" style="margin-top:12px;">
        <strong>对话详情</strong>
        <div id="messageList" style="margin-top:10px;"></div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_sensitive_word_token";
    var state = { groups: [], wordsTotal: 0, monitorTotal: 0, tab: "words" };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newGroupName", "createGroup", "newWordNames", "createWords", "groupFilter", "keywordFilter", "searchWords", "reloadMonitor", "tabWords", "tabMonitor", "statusText", "summaryText", "wordsBody", "wordsEmpty", "monitorBody", "monitorEmpty", "wordsPanel", "monitorPanel", "messagePanel", "messageList"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadAll);
      nodes.createGroup.addEventListener("click", createGroup);
      nodes.createWords.addEventListener("click", createWords);
      nodes.searchWords.addEventListener("click", loadWords);
      nodes.reloadMonitor.addEventListener("click", loadMonitor);
      nodes.tabWords.addEventListener("click", function () { switchTab("words"); });
      nodes.tabMonitor.addEventListener("click", function () { switchTab("monitor"); });
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
      setStatus("正在加载敏感词管理数据...");
      return loadGroups().then(function () {
        return Promise.all([loadWords(), loadMonitor()]);
      }).then(function () {
        setStatus("加载完成", "词库 " + state.wordsTotal + " 条，触发 " + state.monitorTotal + " 条");
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function loadGroups() {
      return api("/dashboard/sensitiveWordGroup/select").then(function (groups) {
        state.groups = Array.isArray(groups) ? groups : [];
        renderGroups();
      });
    }

    function renderGroups() {
      var selected = nodes.groupFilter.value;
      nodes.groupFilter.innerHTML = "";
      var all = document.createElement("option");
      all.value = "0";
      all.textContent = "全部分组";
      nodes.groupFilter.appendChild(all);
      state.groups.forEach(function (group) {
        var option = document.createElement("option");
        option.value = String(group.groupId || group.id || 0);
        option.textContent = group.name || ("分组 " + option.value);
        nodes.groupFilter.appendChild(option);
      });
      if (selected) nodes.groupFilter.value = selected;
    }

    function loadWords() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var groupID = nodes.groupFilter.value || "0";
      if (groupID !== "0") params.set("groupId", groupID);
      var keyword = (nodes.keywordFilter.value || "").trim();
      if (keyword) params.set("keyWords", keyword);
      return api("/dashboard/sensitiveWord/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.wordsTotal = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderWords(list);
      });
    }

    function renderWords(list) {
      nodes.wordsBody.innerHTML = "";
      nodes.wordsEmpty.classList.toggle("hidden", list.length > 0);
      list.forEach(function (item) {
        var tr = document.createElement("tr");
        var status = Number(item.status) === 2 ? "off" : "on";
        tr.innerHTML =
          "<td>" + escapeHTML(item.sensitiveWordId || item.id || "") + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.groupName || "") + "</td>" +
          "<td><span class=\"pill " + status + "\">" + (status === "on" ? "开启" : "关闭") + "</span></td>" +
          "<td>" + escapeHTML(item.employeeNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.contactNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.createdAt || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"toggle\">切换</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"toggle\"]").addEventListener("click", function () {
          toggleWord(item.sensitiveWordId || item.id, status === "on" ? 2 : 1);
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deleteWord(item.sensitiveWordId || item.id);
        });
        nodes.wordsBody.appendChild(tr);
      });
    }

    function loadMonitor() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      return api("/dashboard/sensitiveWordsMonitor/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.monitorTotal = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderMonitor(list);
      });
    }

    function renderMonitor(list) {
      nodes.monitorBody.innerHTML = "";
      nodes.monitorEmpty.classList.toggle("hidden", list.length > 0);
      list.forEach(function (item) {
        var id = item.sensitiveWordsMonitorId || item.sensitiveWordMonitorId || item.id || "";
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.sensitiveWordName || "") + "</td>" +
          "<td><span class=\"pill\">" + escapeHTML(item.sourceText || item.source || "") + "</span></td>" +
          "<td>" + escapeHTML(item.triggerName || "") + "</td>" +
          "<td>" + escapeHTML(item.triggerScenario || "") + "</td>" +
          "<td>" + escapeHTML(item.triggerTime || "") + "</td>" +
          "<td><button class=\"secondary\" data-action=\"detail\">查看</button></td>";
        tr.querySelector("[data-action=\"detail\"]").addEventListener("click", function () {
          loadMessages(id);
        });
        nodes.monitorBody.appendChild(tr);
      });
    }

    function createGroup() {
      var name = (nodes.newGroupName.value || "").trim();
      if (!name) return setStatus("请填写分组名称");
      api("/dashboard/sensitiveWordGroup/store", {
        method: "POST",
        body: JSON.stringify({ name: name })
      }).then(function () {
        nodes.newGroupName.value = "";
        setStatus("分组已创建");
        return loadGroups();
      }).catch(function (err) {
        setStatus("创建分组失败：" + err.message);
      });
    }

    function createWords() {
      var name = (nodes.newWordNames.value || "").trim();
      var groupID = Number(nodes.groupFilter.value || "0");
      if (!groupID && state.groups.length > 0) groupID = Number(state.groups[0].groupId || state.groups[0].id || 0);
      if (!groupID) return setStatus("请先创建或选择分组");
      if (!name) return setStatus("请填写敏感词");
      api("/dashboard/sensitiveWord/store", {
        method: "POST",
        body: JSON.stringify({ groupId: groupID, name: name })
      }).then(function () {
        nodes.newWordNames.value = "";
        setStatus("敏感词已创建");
        return loadWords();
      }).catch(function (err) {
        setStatus("创建敏感词失败：" + err.message);
      });
    }

    function toggleWord(id, status) {
      api("/dashboard/sensitiveWord/statusUpdate", {
        method: "PUT",
        body: JSON.stringify({ sensitiveWordId: Number(id), status: status })
      }).then(function () {
        setStatus("状态已更新");
        return loadWords();
      }).catch(function (err) {
        setStatus("状态更新失败：" + err.message);
      });
    }

    function deleteWord(id) {
      if (!window.confirm("确认删除该敏感词？")) return;
      api("/dashboard/sensitiveWord/destroy", {
        method: "DELETE",
        body: JSON.stringify({ sensitiveWordId: Number(id) })
      }).then(function () {
        setStatus("敏感词已删除");
        return loadWords();
      }).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function loadMessages(id) {
      api("/dashboard/sensitiveWordsMonitor/show?sensitiveWordsMonitorId=" + encodeURIComponent(id)).then(function (items) {
        var list = Array.isArray(items) ? items : [];
        nodes.messagePanel.classList.remove("hidden");
        nodes.messageList.innerHTML = "";
        if (list.length === 0) {
          nodes.messageList.textContent = "暂无对话详情";
          return;
        }
        list.forEach(function (item) {
          var div = document.createElement("div");
          div.style.padding = "8px 0";
          div.style.borderBottom = "1px solid var(--line)";
          div.textContent = "[" + (item.sendTime || "") + "] " + (item.sender || "") + "：" + messageText(item.msgContent);
          nodes.messageList.appendChild(div);
        });
      }).catch(function (err) {
        setStatus("读取对话失败：" + err.message);
      });
    }

    function switchTab(tab) {
      state.tab = tab;
      nodes.tabWords.classList.toggle("active", tab === "words");
      nodes.tabMonitor.classList.toggle("active", tab === "monitor");
      nodes.wordsPanel.classList.toggle("hidden", tab !== "words");
      nodes.monitorPanel.classList.toggle("hidden", tab !== "monitor");
    }

    function messageText(value) {
      if (value == null) return "";
      if (typeof value === "string") return value;
      if (typeof value.content === "string") return value.content;
      try { return JSON.stringify(value); } catch (err) { return String(value); }
    }

    function escapeHTML(value) {
      return String(value == null ? "" : value).replace(/[&<>"']/g, function (ch) {
        return { "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" }[ch];
      });
    }

    document.addEventListener("DOMContentLoaded", init);
  })();
  </script>
</body>
</html>`
