package dashboard

import "net/http"

func NewRadarPageHandler() http.Handler {
	return http.HandlerFunc(ServeRadarPage)
}

func ServeRadarPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(radarPageHTML))
}

const radarPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 互动雷达</title>
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
      grid-template-columns: 118px minmax(170px, 1fr) minmax(180px, 1fr) minmax(180px, 1fr) minmax(210px, 1.1fr) 132px;
      gap: 10px;
      align-items: end;
    }
    .channelbar {
      display: grid;
      grid-template-columns: minmax(170px, 1fr) minmax(92px, .5fr) minmax(92px, .5fr) minmax(92px, .5fr) 132px 132px;
      gap: 10px;
      align-items: end;
    }
    .filterbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 132px 116px;
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
      .tokenbar, .createbar, .channelbar, .filterbar { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 互动雷达</h1>
        <p class="subtitle">独立 Go 控制台，直接管理雷达素材、渠道、渠道链接和客户点击明细，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建雷达">
      <label>类型
        <select id="newType">
          <option value="1">链接</option>
          <option value="2">PDF</option>
          <option value="3">文章</option>
        </select>
      </label>
      <label>雷达标题
        <input id="newTitle" placeholder="例如：产品介绍雷达">
      </label>
      <label>访问链接
        <input id="newLink" placeholder="/radar/product.html">
      </label>
      <label>链接标题
        <input id="newLinkTitle" placeholder="客户看到的标题">
      </label>
      <label>摘要
        <input id="newDescription" placeholder="用于链接卡片或备注">
      </label>
      <button id="createRadar" type="button">新增雷达</button>
    </section>

    <section class="panel channelbar" aria-label="渠道和链接">
      <label>渠道名称
        <input id="newChannelName" placeholder="例如：朋友圈">
      </label>
      <label>雷达 ID
        <input id="linkRadarID" inputmode="numeric" placeholder="自动">
      </label>
      <label>渠道 ID
        <input id="linkChannelID" inputmode="numeric" placeholder="自动">
      </label>
      <label>员工 ID
        <input id="linkEmployeeID" inputmode="numeric" value="2">
      </label>
      <button id="createChannel" class="secondary" type="button">新增渠道</button>
      <button id="createChannelLink" type="button">生成链接</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>关键词
        <input id="searchTitle" placeholder="按标题、链接筛选">
      </label>
      <label>类型
        <select id="filterType">
          <option value="">全部</option>
          <option value="1">链接</option>
          <option value="2">PDF</option>
          <option value="3">文章</option>
        </select>
      </label>
      <button id="searchRadars" class="secondary" type="button">查询</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>互动雷达列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>标题</th>
              <th style="width:82px;">类型</th>
              <th>内容</th>
              <th style="width:96px;">点击</th>
              <th style="width:86px;">渠道</th>
              <th style="width:150px;">通知</th>
              <th style="width:120px;">创建人</th>
              <th style="width:132px;">创建时间</th>
              <th style="width:230px;">操作</th>
            </tr>
          </thead>
          <tbody id="radarsBody"></tbody>
        </table>
        <div id="radarsEmpty" class="empty">暂无互动雷达</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>雷达详情</h2>
        <span id="detailTitle" class="subtitle">未选择雷达</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">ID</th>
              <th>素材</th>
              <th>文章/标签</th>
              <th style="width:110px;">客户评分</th>
              <th style="width:110px;">点击人数</th>
              <th style="width:110px;">渠道数</th>
            </tr>
          </thead>
          <tbody id="detailBody"></tbody>
        </table>
        <div id="detailEmpty" class="empty">暂无雷达详情</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>渠道列表</h2>
        <span id="channelsTitle" class="subtitle">未选择雷达</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">渠道 ID</th>
              <th>渠道名称</th>
              <th style="width:140px;">创建人</th>
              <th style="width:160px;">创建时间</th>
            </tr>
          </thead>
          <tbody id="channelsBody"></tbody>
        </table>
        <div id="channelsEmpty" class="empty">暂无渠道</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>渠道链接</h2>
        <span id="linksTitle" class="subtitle">未选择雷达</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">链接 ID</th>
              <th>渠道</th>
              <th>员工</th>
              <th>链接</th>
              <th style="width:90px;">点击</th>
              <th style="width:90px;">人数</th>
              <th style="width:150px;">更新时间</th>
            </tr>
          </thead>
          <tbody id="linksBody"></tbody>
        </table>
        <div id="linksEmpty" class="empty">暂无渠道链接</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>客户点击明细</h2>
        <span id="recordsTitle" class="subtitle">未选择雷达</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">记录 ID</th>
              <th>客户</th>
              <th>渠道</th>
              <th>员工</th>
              <th style="width:90px;">点击数</th>
              <th>最近内容</th>
              <th style="width:150px;">最近点击</th>
            </tr>
          </thead>
          <tbody id="recordsBody"></tbody>
        </table>
        <div id="recordsEmpty" class="empty">暂无客户点击</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_radar_token";
    var state = { total: 0, selectedID: 0, firstChannelID: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newType", "newTitle", "newLink", "newLinkTitle", "newDescription", "createRadar", "newChannelName", "linkRadarID", "linkChannelID", "linkEmployeeID", "createChannel", "createChannelLink", "searchTitle", "filterType", "searchRadars", "statusText", "summaryText", "radarsBody", "radarsEmpty", "detailTitle", "detailBody", "detailEmpty", "channelsTitle", "channelsBody", "channelsEmpty", "linksTitle", "linksBody", "linksEmpty", "recordsTitle", "recordsBody", "recordsEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadRadars);
      nodes.searchRadars.addEventListener("click", loadRadars);
      nodes.createRadar.addEventListener("click", createRadar);
      nodes.createChannel.addEventListener("click", createChannel);
      nodes.createChannelLink.addEventListener("click", createChannelLink);
      loadRadars();
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

    function loadRadars() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var keyword = (nodes.searchTitle.value || "").trim();
      var type = (nodes.filterType.value || "").trim();
      if (keyword) params.set("title", keyword);
      if (type) params.set("type", type);
      setStatus("正在加载互动雷达...");
      return api("/dashboard/radar/index?" + params.toString()).then(function (data) {
        var list = pageList(data);
        state.total = pageTotal(data, list);
        renderRadars(list);
        setStatus("加载完成", "雷达 " + state.total + " 条");
        if (list.length > 0) {
          var firstID = radarID(list[0]);
          if (firstID) return loadDetail(firstID);
        }
        renderDetail(null);
        renderChannels([]);
        renderChannelLinks([]);
        renderRecords([]);
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function renderRadars(list) {
      nodes.radarsBody.innerHTML = "";
      nodes.radarsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = radarID(item);
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.title || item.name || "") + "</td>" +
          "<td><span class=\"pill\">" + escapeHTML(typeText(item.type)) + "</span></td>" +
          "<td>" + escapeHTML(contentText(item)) + "</td>" +
          "<td>" + escapeHTML(item.clickNum || item.click_num || 0) + " / " + escapeHTML(item.clickPersonNum || item.click_person_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.channelNum || item.channel_num || 0) + "</td>" +
          "<td>" + flagPill("名片", item.employeeCard || item.employee_card) + flagPill("行为", item.actionNotice || item.action_notice) + flagPill("动态", item.dynamicNotice || item.dynamic_notice) + "</td>" +
          "<td>" + escapeHTML(item.createUserName || item.create_user_name || "") + "</td>" +
          "<td>" + escapeHTML(item.createdAt || item.created_at || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"detail\">详情</button><button class=\"secondary\" data-action=\"toggle\">切换通知</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"detail\"]").addEventListener("click", function () {
          loadDetail(id);
        });
        tr.querySelector("[data-action=\"toggle\"]").addEventListener("click", function () {
          var current = Number(item.actionNotice || item.action_notice || 0);
          updateRadar(id, { actionNotice: current === 1 ? 0 : 1 });
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deleteRadar(id);
        });
        nodes.radarsBody.appendChild(tr);
      });
    }

    function loadDetail(id) {
      if (!id) return Promise.resolve();
      state.selectedID = Number(id);
      nodes.linkRadarID.value = String(id);
      setStatus("正在加载雷达详情...");
      return Promise.all([
        api("/dashboard/radar/show?id=" + encodeURIComponent(id)),
        api("/dashboard/radar/indexChannel?page=1&perPage=100"),
        api("/dashboard/radar/indexChannelLink?radarId=" + encodeURIComponent(id) + "&page=1&perPage=100"),
        api("/dashboard/radar/showChannel?radarId=" + encodeURIComponent(id) + "&page=1&perPage=20"),
        api("/dashboard/radar/showContact?radarId=" + encodeURIComponent(id) + "&page=1&perPage=20")
      ]).then(function (result) {
        renderDetail(result[0]);
        renderChannels(pageList(result[1]));
        renderChannelLinks(pageList(result[2]).concat(pageList(result[3])));
        renderRecords(pageList(result[4]));
        setStatus("详情加载完成", "雷达 " + state.total + " 条");
      }).catch(function (err) {
        renderDetail(null);
        renderChannels([]);
        renderChannelLinks([]);
        renderRecords([]);
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderDetail(item) {
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = item ? "none" : "block";
      nodes.detailTitle.textContent = item && item.title ? item.title : "未选择雷达";
      if (!item) return;
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(radarID(item)) + "</td>" +
        "<td>" + escapeHTML(materialText(item)) + "</td>" +
        "<td>" + escapeHTML("文章：" + compactJSON(item.articleRaw || item.article || []) + "；标签：" + compactJSON(item.contactTags || item.contact_tags || [])) + "</td>" +
        "<td>" + escapeHTML(compactJSON(item.contactGrade || item.contact_grade || [])) + "</td>" +
        "<td>" + escapeHTML(item.clickPersonNum || item.click_person_num || 0) + "</td>" +
        "<td>" + escapeHTML(item.channelNum || item.channel_num || 0) + "</td>";
      nodes.detailBody.appendChild(tr);
    }

    function renderChannels(list) {
      nodes.channelsBody.innerHTML = "";
      nodes.channelsEmpty.style.display = list.length > 0 ? "none" : "block";
      nodes.channelsTitle.textContent = state.selectedID ? ("雷达 " + state.selectedID) : "未选择雷达";
      state.firstChannelID = 0;
      list.forEach(function (item) {
        var id = channelID(item);
        if (!state.firstChannelID && id) {
          state.firstChannelID = Number(id);
          if (!nodes.linkChannelID.value) nodes.linkChannelID.value = String(id);
        }
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || item.channelName || item.channel_name || "") + "</td>" +
          "<td>" + escapeHTML(item.createUserName || item.create_user_name || "") + "</td>" +
          "<td>" + escapeHTML(item.createdAt || item.created_at || "") + "</td>";
        nodes.channelsBody.appendChild(tr);
      });
    }

    function renderChannelLinks(list) {
      nodes.linksBody.innerHTML = "";
      nodes.linksEmpty.style.display = list.length > 0 ? "none" : "block";
      nodes.linksTitle.textContent = state.selectedID ? ("雷达 " + state.selectedID) : "未选择雷达";
      var seen = {};
      list.forEach(function (item) {
        var id = channelLinkID(item);
        if (id && seen[id]) return;
        if (id) seen[id] = true;
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.channelName || item.channel_name || "") + "</td>" +
          "<td>" + escapeHTML(item.employeeName || item.employee_name || item.employeeId || item.employee_id || "") + "</td>" +
          "<td>" + escapeHTML(item.link || "") + "</td>" +
          "<td>" + escapeHTML(item.clickNum || item.click_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.clickPersonNum || item.click_person_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.updatedAt || item.updated_at || "") + "</td>";
        nodes.linksBody.appendChild(tr);
      });
    }

    function renderRecords(list) {
      nodes.recordsBody.innerHTML = "";
      nodes.recordsEmpty.style.display = list.length > 0 ? "none" : "block";
      nodes.recordsTitle.textContent = state.selectedID ? ("雷达 " + state.selectedID) : "未选择雷达";
      list.forEach(function (item) {
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(item.id || item.recordId || item.record_id || "") + "</td>" +
          "<td>" + escapeHTML(item.nickname || item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.channelName || item.channel_name || "") + "</td>" +
          "<td>" + escapeHTML(item.employeeName || item.employee_name || "") + "</td>" +
          "<td>" + escapeHTML(item.clickNum || item.click_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.content || clickInfoText(item.clickInfo || item.click_info || [])) + "</td>" +
          "<td>" + escapeHTML(item.createdAt || item.created_at || "") + "</td>";
        nodes.recordsBody.appendChild(tr);
      });
    }

    function createRadar() {
      var title = (nodes.newTitle.value || "").trim();
      if (!title) {
        setStatus("请输入雷达标题");
        return;
      }
      var link = (nodes.newLink.value || "").trim() || "/radar/" + Date.now() + ".html";
      var payload = {
        type: Number(nodes.newType.value || 1),
        title: title,
        link: link,
        linkTitle: (nodes.newLinkTitle.value || "").trim() || title,
        linkDescription: (nodes.newDescription.value || "").trim(),
        employeeCard: 1,
        actionNotice: 1,
        dynamicNotice: 1,
        contactTags: [],
        tagStatus: 0,
        contactGrade: [],
        article: []
      };
      setStatus("正在新增互动雷达...");
      api("/dashboard/radar/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newTitle.value = "";
        nodes.newLink.value = "";
        nodes.newLinkTitle.value = "";
        nodes.newDescription.value = "";
        return loadRadars();
      }).then(function () {
        setStatus("新增完成", "雷达 " + state.total + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function createChannel() {
      var name = (nodes.newChannelName.value || "").trim();
      if (!name) {
        setStatus("请输入渠道名称");
        return;
      }
      setStatus("正在新增雷达渠道...");
      api("/dashboard/radar/storeChannel", {
        method: "POST",
        body: JSON.stringify({ name: name })
      }).then(function () {
        nodes.newChannelName.value = "";
        if (state.selectedID) return loadDetail(state.selectedID);
        return loadRadars();
      }).catch(function (err) {
        setStatus("新增渠道失败：" + err.message);
      });
    }

    function createChannelLink() {
      var radarId = Number((nodes.linkRadarID.value || "").trim() || state.selectedID || 0);
      var channelId = Number((nodes.linkChannelID.value || "").trim() || state.firstChannelID || 0);
      var employeeId = Number((nodes.linkEmployeeID.value || "").trim() || 0);
      if (!radarId || !channelId || !employeeId) {
        setStatus("请确认雷达 ID、渠道 ID 和员工 ID");
        return;
      }
      setStatus("正在生成渠道链接...");
      api("/dashboard/radar/storeChannelLink", {
        method: "POST",
        body: JSON.stringify({ radarId: radarId, channelId: channelId, employeeId: employeeId, type: Number(nodes.newType.value || 1) })
      }).then(function () {
        return loadDetail(radarId);
      }).catch(function (err) {
        setStatus("生成链接失败：" + err.message);
      });
    }

    function updateRadar(id, patch) {
      if (!id) return;
      patch.id = Number(id);
      patch.radarId = Number(id);
      setStatus("正在更新互动雷达...");
      api("/dashboard/radar/update", {
        method: "PUT",
        body: JSON.stringify(patch)
      }).then(loadRadars).catch(function (err) {
        setStatus("更新失败：" + err.message);
      });
    }

    function deleteRadar(id) {
      if (!id || !window.confirm("确认删除该互动雷达？")) return;
      setStatus("正在删除互动雷达...");
      api("/dashboard/radar/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), radarId: Number(id) })
      }).then(loadRadars).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function pageList(data) {
      if (!data) return [];
      if (Array.isArray(data.list)) return data.list;
      if (Array.isArray(data.data)) return data.data;
      if (data.data && Array.isArray(data.data.data)) return data.data.data;
      return [];
    }

    function pageTotal(data, list) {
      if (data && data.page && data.page.total !== undefined) return Number(data.page.total);
      if (data && data.total !== undefined) return Number(data.total);
      return list.length;
    }

    function radarID(item) {
      return item.radarId || item.radar_id || item.id || 0;
    }

    function channelID(item) {
      return item.channelId || item.channel_id || item.id || 0;
    }

    function channelLinkID(item) {
      return item.channelLinkId || item.channel_link_id || item.id || 0;
    }

    function typeText(value) {
      var type = Number(value || 0);
      if (type === 1) return "链接";
      if (type === 2) return "PDF";
      if (type === 3) return "文章";
      return "未知";
    }

    function contentText(item) {
      if (Number(item.type || 0) === 2) return (item.pdfName || item.pdf_name || "") + " " + (item.pdf || "");
      if (Number(item.type || 0) === 3) return compactJSON(item.articleRaw || item.article || []);
      return (item.linkTitle || item.link_title || item.title || "") + " " + (item.link || "");
    }

    function materialText(item) {
      return typeText(item.type) + "；链接：" + (item.link || "") + "；PDF：" + (item.pdfName || item.pdf_name || item.pdf || "") + "；摘要：" + (item.linkDescription || item.link_description || "");
    }

    function flagPill(label, value) {
      var enabled = Number(value || 0) === 1;
      return "<span class=\"pill " + (enabled ? "on" : "off") + "\">" + label + (enabled ? "开" : "关") + "</span>";
    }

    function clickInfoText(value) {
      if (!Array.isArray(value) || value.length === 0) return "";
      var first = value[0] || {};
      return (first.createdAt || first.created_at || "") + " " + (first.content || "");
    }

    function compactJSON(value) {
      if (value === undefined || value === null || value === "") return "";
      if (typeof value === "string") {
        try { return JSON.stringify(JSON.parse(value)); } catch (err) { return value; }
      }
      try { return JSON.stringify(value); } catch (err) { return String(value); }
    }

    function escapeHTML(value) {
      return String(value === undefined || value === null ? "" : value)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
    }

    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", init);
    } else {
      init();
    }
  })();
  </script>
</body>
</html>`
