package dashboard

import "net/http"

func NewShopCodePageHandler() http.Handler {
	return http.HandlerFunc(ServeShopCodePage)
}

func ServeShopCodePage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(shopCodePageHTML))
}

const shopCodePageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 门店活码</title>
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
    .filterbar {
      display: grid;
      grid-template-columns: 150px 150px minmax(180px, 1fr) minmax(150px, .8fr) 132px;
      gap: 10px;
      align-items: end;
    }
    .createbar {
      display: grid;
      grid-template-columns: 142px minmax(170px, 1fr) minmax(190px, 1.1fr) minmax(150px, .8fr) 132px 132px;
      gap: 10px;
      align-items: end;
    }
    .jsonbar {
      display: grid;
      grid-template-columns: minmax(170px, 1fr) minmax(170px, 1fr) minmax(170px, 1fr) 132px 132px 132px;
      gap: 10px;
      align-items: end;
    }
    .pagebar {
      display: grid;
      grid-template-columns: 118px minmax(160px, 1fr) 132px 118px minmax(170px, 1fr) 132px;
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
      min-height: 70px;
      padding: 8px 10px;
      color: var(--text);
      background: #fff;
      resize: vertical;
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
      min-width: 1080px;
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
    .grid2 {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 12px;
    }
    .metrics {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 10px;
      margin-bottom: 12px;
    }
    .metric {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fbfcfe;
      padding: 12px;
    }
    .metric span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 6px;
    }
    .metric strong {
      display: block;
      font-size: 22px;
      line-height: 1.1;
    }
    .mono {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 12px;
    }
    .selected {
      outline: 2px solid #99d6cf;
      outline-offset: -2px;
    }
    @media (max-width: 900px) {
      main { width: min(100vw - 20px, 1180px); padding-top: 18px; }
      header, .status { align-items: flex-start; flex-direction: column; }
      .tokenbar, .filterbar, .createbar, .jsonbar, .pagebar, .grid2, .metrics {
        grid-template-columns: 1fr;
      }
      table { min-width: 900px; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 门店活码</h1>
        <p class="subtitle">独立 Go 版门店活码控制台，直接调用同源 dashboard API 管理门店、页面设置、分享链接和访问数据。</p>
      </div>
      <button id="reload" type="button" class="secondary">刷新</button>
    </header>

    <section class="panel">
      <h2>Dashboard JWT</h2>
      <div class="tokenbar">
        <label>Authorization
          <input id="token" type="password" autocomplete="off" placeholder="Bearer token">
        </label>
        <button id="saveToken" type="button">保存 Token</button>
        <button id="clearToken" type="button" class="secondary">清空</button>
      </div>
    </section>

    <section class="panel">
      <h2>筛选与新增</h2>
      <div class="filterbar">
        <label>类型
          <select id="filterType">
            <option value="0">全部类型</option>
            <option value="1">添加店主</option>
            <option value="2">加入门店群</option>
            <option value="3">加入城市群</option>
          </select>
        </label>
        <label>状态
          <select id="filterStatus">
            <option value="">全部状态</option>
            <option value="1">开启</option>
            <option value="0">关闭</option>
          </select>
        </label>
        <label>名称或关键词
          <input id="filterName" type="search" placeholder="门店名称、地址、关键词">
        </label>
        <label>城市
          <input id="filterCity" type="search" placeholder="城市">
        </label>
        <button id="search" type="button">查询</button>
      </div>
      <div class="createbar" style="margin-top: 10px;">
        <label>新增类型
          <select id="newType">
            <option value="1">添加店主</option>
            <option value="2">加入门店群</option>
            <option value="3">加入城市群</option>
          </select>
        </label>
        <label>门店名称
          <input id="shopName" type="text" placeholder="门店名称">
        </label>
        <label>地址
          <input id="shopAddress" type="text" placeholder="门店地址">
        </label>
        <label>城市
          <input id="shopCity" type="text" placeholder="城市">
        </label>
        <button id="createShop" type="button">新增门店</button>
        <button id="updateShop" type="button" class="secondary">保存当前</button>
      </div>
      <div class="jsonbar" style="margin-top: 10px;">
        <label>店主 JSON
          <textarea id="employeeJSON" spellcheck="false">[{"id":2,"name":"前端切换员工"}]</textarea>
        </label>
        <label>店主二维码 JSON
          <textarea id="qrcodeJSON" spellcheck="false">[{"url":"qrcode/front-shop-owner.png"}]</textarea>
        </label>
        <label>拉群活码 JSON
          <textarea id="qwCodeJSON" spellcheck="false">[{"id":910001,"name":"前端联调客户群"}]</textarea>
        </label>
        <button id="saveEmployee" type="button" class="secondary">保存店主</button>
        <button id="saveQrcode" type="button" class="secondary">保存二维码</button>
        <button id="batchTags" type="button" class="secondary">批量标签</button>
      </div>
    </section>

    <section class="panel">
      <h2>页面设置</h2>
      <div class="pagebar">
        <label>类型
          <select id="pageType">
            <option value="1">添加店主</option>
            <option value="2">加入门店群</option>
            <option value="3">加入城市群</option>
          </select>
        </label>
        <label>页面标题
          <input id="pageTitle" type="text" placeholder="页面标题">
        </label>
        <label>展示方式
          <select id="showType">
            <option value="1">默认样式</option>
            <option value="2">自定义海报</option>
          </select>
        </label>
        <label>自动通过
          <select id="autoPass">
            <option value="0">关闭</option>
            <option value="1">开启</option>
          </select>
        </label>
        <label>海报
          <input id="poster" type="text" placeholder="海报路径">
        </label>
        <button id="savePage" type="button">保存页面</button>
      </div>
      <label style="margin-top: 10px;">默认样式 JSON
        <textarea id="defaultJSON" spellcheck="false">{"guide":"扫码添加店主","description":"门店活码独立页"}</textarea>
      </label>
    </section>

    <div class="status">
      <span id="statusText">准备就绪</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <div class="metrics">
        <div class="metric"><span>门店总数</span><strong id="shopTotal">0</strong></div>
        <div class="metric"><span>开启</span><strong id="openTotal">0</strong></div>
        <div class="metric"><span>关闭</span><strong id="closeTotal">0</strong></div>
        <div class="metric"><span>访问记录</span><strong id="recordTotal">0</strong></div>
      </div>
      <h2>门店列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width: 76px;">ID</th>
              <th>门店</th>
              <th style="width: 108px;">类型</th>
              <th style="width: 96px;">状态</th>
              <th>地址</th>
              <th style="width: 140px;">城市</th>
              <th style="width: 190px;">更新时间</th>
              <th style="width: 280px;">操作</th>
            </tr>
          </thead>
          <tbody id="shopsBody"></tbody>
        </table>
        <div id="shopsEmpty" class="empty">暂无门店活码</div>
      </div>
    </section>

    <section class="grid2">
      <div class="panel">
        <h2>当前门店详情</h2>
        <div class="tablewrap">
          <table>
            <thead>
              <tr>
                <th style="width: 76px;">ID</th>
                <th>门店</th>
                <th>店主</th>
                <th>二维码</th>
                <th>分享链接</th>
              </tr>
            </thead>
            <tbody id="detailBody"></tbody>
          </table>
          <div id="detailEmpty" class="empty">请选择门店</div>
        </div>
      </div>
      <div class="panel">
        <h2>地址与页面配置</h2>
        <div class="tablewrap">
          <table>
            <thead>
              <tr>
                <th>位置信息</th>
                <th>页面配置</th>
              </tr>
            </thead>
            <tbody id="configBody"></tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="grid2">
      <div class="panel">
        <h2>访问客户</h2>
        <div class="tablewrap">
          <table>
            <thead>
              <tr>
                <th style="width: 76px;">ID</th>
                <th>门店</th>
                <th style="width: 108px;">类型</th>
                <th style="width: 190px;">访问时间</th>
              </tr>
            </thead>
            <tbody id="recordsBody"></tbody>
          </table>
          <div id="recordsEmpty" class="empty">暂无访问记录</div>
        </div>
      </div>
      <div class="panel">
        <h2>门店统计</h2>
        <div class="tablewrap">
          <table>
            <thead>
              <tr>
                <th style="width: 76px;">ID</th>
                <th>门店</th>
                <th style="width: 108px;">状态</th>
                <th style="width: 108px;">访问数</th>
              </tr>
            </thead>
            <tbody id="statsBody"></tbody>
          </table>
          <div id="statsEmpty" class="empty">暂无门店统计</div>
        </div>
      </div>
    </section>

    <section class="grid2">
      <div class="panel">
        <h2>城市建议</h2>
        <div id="cities" class="mono"></div>
      </div>
      <div class="panel">
        <h2>地址建议</h2>
        <div id="addresses" class="mono"></div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    "use strict";

    var tokenKey = "mochat_go_shop_code_token";
    var state = { selectedID: 0, total: 0, shops: [] };
    var nodes = {};

    document.addEventListener("DOMContentLoaded", init);

    function $(id) {
      return document.getElementById(id);
    }

    function init() {
      [
        "token", "saveToken", "clearToken", "reload", "filterType", "filterStatus", "filterName", "filterCity", "search",
        "newType", "shopName", "shopAddress", "shopCity", "employeeJSON", "qrcodeJSON", "qwCodeJSON",
        "createShop", "updateShop", "saveEmployee", "saveQrcode", "batchTags",
        "pageType", "pageTitle", "showType", "autoPass", "poster", "defaultJSON", "savePage",
        "statusText", "summaryText", "shopTotal", "openTotal", "closeTotal", "recordTotal",
        "shopsBody", "shopsEmpty", "detailBody", "detailEmpty", "configBody", "recordsBody", "recordsEmpty",
        "statsBody", "statsEmpty", "cities", "addresses"
      ].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadDashboard);
      nodes.search.addEventListener("click", loadDashboard);
      nodes.createShop.addEventListener("click", createShop);
      nodes.updateShop.addEventListener("click", updateShop);
      nodes.saveEmployee.addEventListener("click", saveEmployee);
      nodes.saveQrcode.addEventListener("click", saveQrcode);
      nodes.batchTags.addEventListener("click", batchTags);
      nodes.savePage.addEventListener("click", savePageSetting);
      nodes.pageType.addEventListener("change", loadPageSetting);
      loadDashboard();
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

    function loadDashboard() {
      setStatus("正在加载门店活码...");
      return loadShops().then(function () {
        return Promise.all([
          loadOverview(),
          loadPageSetting(),
          loadCities(),
          loadAddresses()
        ]);
      }).then(function () {
        setStatus("加载完成", "门店 " + state.total + " 条");
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function listParams() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var codeType = Number(nodes.filterType.value || 0);
      if (codeType > 0) params.set("type", String(codeType));
      if (nodes.filterStatus.value !== "") params.set("status", nodes.filterStatus.value);
      var name = (nodes.filterName.value || "").trim();
      if (name) params.set("name", name);
      var city = (nodes.filterCity.value || "").trim();
      if (city) params.set("city", city);
      return params;
    }

    function currentType() {
      return Number(nodes.filterType.value || nodes.pageType.value || 1) || 1;
    }

    function loadShops() {
      return api("/dashboard/shopCode/index?" + listParams().toString()).then(function (data) {
        var list = pageList(data);
        state.shops = list;
        state.total = pageTotal(data, list);
        renderShops(list);
        if (list.length > 0) {
          return selectShop(shopID(list[0]));
        }
        state.selectedID = 0;
        renderDetail(null, null, null);
        renderRecords([]);
        renderStats([]);
        return Promise.resolve();
      });
    }

    function loadOverview() {
      var params = new URLSearchParams();
      var codeType = currentType();
      if (codeType > 0) params.set("type", String(codeType));
      return api("/dashboard/shopCode/show?" + params.toString()).then(function (data) {
        nodes.shopTotal.textContent = String(data.shopTotal || data.shop_total || 0);
        nodes.openTotal.textContent = String(data.openTotal || data.open_total || 0);
        nodes.closeTotal.textContent = String(data.closeTotal || data.close_total || 0);
        nodes.recordTotal.textContent = String(data.recordTotal || data.record_total || 0);
      });
    }

    function loadPageSetting() {
      var params = new URLSearchParams();
      params.set("type", nodes.pageType.value || "1");
      return api("/dashboard/shopCode/pageInfo?" + params.toString()).then(function (data) {
        nodes.pageTitle.value = data.title || "";
        nodes.showType.value = String(data.showType || data.show_type || 1);
        nodes.autoPass.value = String(data.autoPass || data.auto_pass || 0);
        nodes.poster.value = data.poster || "";
        nodes.defaultJSON.value = compactJSON(data.default || {});
        renderConfig(state.lastLocation || {}, data);
      });
    }

    function loadCities() {
      var params = new URLSearchParams();
      var name = (nodes.filterCity.value || nodes.filterName.value || "杭州").trim();
      if (name) params.set("name", name);
      return api("/dashboard/shopCode/searchCity?" + params.toString()).then(function (data) {
        nodes.cities.textContent = compactJSON(data || []);
      });
    }

    function loadAddresses() {
      var params = new URLSearchParams();
      params.set("name", (nodes.filterName.value || "前端").trim());
      var city = (nodes.filterCity.value || "杭州").trim();
      if (city) params.set("city", city);
      return api("/dashboard/shopCode/addressKeyWordList?" + params.toString()).then(function (data) {
        nodes.addresses.textContent = compactJSON(data || []);
      });
    }

    function selectShop(id) {
      if (!id) return Promise.resolve();
      state.selectedID = Number(id);
      highlightSelected();
      return Promise.all([
        api("/dashboard/shopCode/info?id=" + encodeURIComponent(id)),
        api("/dashboard/shopCode/share?id=" + encodeURIComponent(id)),
        api("/dashboard/shopCode/location?id=" + encodeURIComponent(id)),
        api("/dashboard/shopCode/showContact?shopCodeId=" + encodeURIComponent(id) + "&page=1&perPage=20"),
        api("/dashboard/shopCode/showShop?type=" + encodeURIComponent(currentType()) + "&page=1&perPage=20")
      ]).then(function (result) {
        state.lastLocation = result[2] || {};
        renderDetail(result[0], result[1], result[2]);
        renderConfig(result[2], null);
        renderRecords(pageList(result[3]));
        renderStats(pageList(result[4]));
        fillEditForm(result[0]);
      }).catch(function (err) {
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderShops(list) {
      nodes.shopsBody.innerHTML = "";
      nodes.shopsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = shopID(item);
        var status = Number(item.status || item.state || 0);
        var tr = document.createElement("tr");
        tr.dataset.id = String(id);
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td><span class=\"pill\">" + escapeHTML(typeName(item.type)) + "</span></td>" +
          "<td>" + statusPill(status) + "</td>" +
          "<td>" + escapeHTML(item.address || item.searchKeyword || item.search_keyword || "") + "</td>" +
          "<td>" + escapeHTML([item.province, item.city, item.district].filter(Boolean).join(" ")) + "</td>" +
          "<td>" + escapeHTML(item.updatedAt || item.updated_at || item.createdAt || item.created_at || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"select\">详情</button><button class=\"secondary\" data-action=\"toggle\">" + (status === 1 ? "关闭" : "开启") + "</button><button class=\"secondary\" data-action=\"share\">分享</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"select\"]").addEventListener("click", function () { selectShop(id); });
        tr.querySelector("[data-action=\"toggle\"]").addEventListener("click", function () { updateStatus(id, status === 1 ? 0 : 1); });
        tr.querySelector("[data-action=\"share\"]").addEventListener("click", function () { selectShop(id); });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () { deleteShop(id); });
        nodes.shopsBody.appendChild(tr);
      });
      highlightSelected();
    }

    function highlightSelected() {
      Array.prototype.forEach.call(nodes.shopsBody.querySelectorAll("tr"), function (tr) {
        tr.classList.toggle("selected", Number(tr.dataset.id) === state.selectedID);
      });
    }

    function renderDetail(info, share, location) {
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = info ? "none" : "block";
      if (!info) return;
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(shopID(info)) + "</td>" +
        "<td>" + escapeHTML(info.name || "") + "</td>" +
        "<td>" + escapeHTML(compactJSON(info.employee || [])) + "</td>" +
        "<td>" + escapeHTML(compactJSON(info.qrcode || info.employeeQrcode || info.employee_qrcode || [])) + "</td>" +
        "<td class=\"mono\">" + escapeHTML((share && (share.link || share.url || share.shareUrl)) || (location && location.address) || "") + "</td>";
      nodes.detailBody.appendChild(tr);
    }

    function renderConfig(location, pageSetting) {
      var currentPage = pageSetting || {
        title: nodes.pageTitle.value,
        showType: nodes.showType.value,
        autoPass: nodes.autoPass.value,
        poster: nodes.poster.value,
        default: safeJSON(nodes.defaultJSON.value || "{}")
      };
      nodes.configBody.innerHTML = "";
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(compactJSON(location || {})) + "</td>" +
        "<td>" + escapeHTML(compactJSON(currentPage || {})) + "</td>";
      nodes.configBody.appendChild(tr);
    }

    function renderRecords(list) {
      nodes.recordsBody.innerHTML = "";
      nodes.recordsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(item.id || "") + "</td>" +
          "<td>" + escapeHTML(item.shopName || item.shop_name || "") + "</td>" +
          "<td>" + escapeHTML(typeName(item.type)) + "</td>" +
          "<td>" + escapeHTML(item.createdAt || item.created_at || "") + "</td>";
        nodes.recordsBody.appendChild(tr);
      });
    }

    function renderStats(list) {
      nodes.statsBody.innerHTML = "";
      nodes.statsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(shopID(item)) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td>" + statusPill(Number(item.status || item.state || 0)) + "</td>" +
          "<td>" + escapeHTML(item.recordTotal || item.record_total || 0) + "</td>";
        nodes.statsBody.appendChild(tr);
      });
    }

    function fillEditForm(item) {
      if (!item) return;
      nodes.newType.value = String(item.type || 1);
      nodes.shopName.value = item.name || "";
      nodes.shopAddress.value = item.address || "";
      nodes.shopCity.value = item.city || "";
      nodes.employeeJSON.value = compactJSON(item.employee || []);
      nodes.qrcodeJSON.value = compactJSON(item.qrcode || item.employeeQrcode || item.employee_qrcode || []);
      nodes.qwCodeJSON.value = compactJSON(item.qwCode || item.qw_code || []);
    }

    function baseShopPayload() {
      var employee = requireJSON(nodes.employeeJSON.value || "[]", "店主 JSON");
      var qrcode = requireJSON(nodes.qrcodeJSON.value || "[]", "店主二维码 JSON");
      var qwCode = requireJSON(nodes.qwCodeJSON.value || "[]", "拉群活码 JSON");
      return {
        name: (nodes.shopName.value || "").trim(),
        type: Number(nodes.newType.value || 1),
        employee: employee,
        employeeQrcode: qrcode,
        qwCode: qwCode,
        searchKeyword: (nodes.filterName.value || nodes.shopName.value || "").trim(),
        address: (nodes.shopAddress.value || "").trim(),
        country: "中国",
        province: "浙江省",
        city: (nodes.shopCity.value || "").trim(),
        district: "",
        lat: "30.25",
        lng: "120.12",
        status: 1
      };
    }

    function createShop() {
      var payload;
      try {
        payload = baseShopPayload();
      } catch (err) {
        setStatus(err.message);
        return;
      }
      if (!payload.name) {
        setStatus("请输入门店名称");
        return;
      }
      setStatus("正在新增门店活码...");
      api("/dashboard/shopCode/store", { method: "POST", body: JSON.stringify(payload) }).then(loadDashboard).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function updateShop() {
      if (!state.selectedID) {
        setStatus("请先选择门店");
        return;
      }
      var payload;
      try {
        payload = baseShopPayload();
      } catch (err) {
        setStatus(err.message);
        return;
      }
      payload.id = state.selectedID;
      payload.shopCodeId = state.selectedID;
      setStatus("正在保存门店活码...");
      api("/dashboard/shopCode/update", { method: "PUT", body: JSON.stringify(payload) }).then(loadDashboard).catch(function (err) {
        setStatus("保存失败：" + err.message);
      });
    }

    function saveEmployee() {
      if (!state.selectedID) return setStatus("请先选择门店");
      var employee;
      try {
        employee = requireJSON(nodes.employeeJSON.value || "[]", "店主 JSON");
      } catch (err) {
        setStatus(err.message);
        return;
      }
      api("/dashboard/shopCode/updateEmployee", {
        method: "POST",
        body: JSON.stringify({ id: state.selectedID, shopCodeId: state.selectedID, employee: employee })
      }).then(function () {
        return selectShop(state.selectedID);
      }).catch(function (err) {
        setStatus("保存店主失败：" + err.message);
      });
    }

    function saveQrcode() {
      if (!state.selectedID) return setStatus("请先选择门店");
      var qrcode;
      var qwCode;
      try {
        qrcode = requireJSON(nodes.qrcodeJSON.value || "[]", "店主二维码 JSON");
        qwCode = requireJSON(nodes.qwCodeJSON.value || "[]", "拉群活码 JSON");
      } catch (err) {
        setStatus(err.message);
        return;
      }
      api("/dashboard/shopCode/updateQrcode", {
        method: "POST",
        body: JSON.stringify({ id: state.selectedID, shopCodeId: state.selectedID, qrcode: qrcode, qwCode: qwCode })
      }).then(function () {
        return selectShop(state.selectedID);
      }).catch(function (err) {
        setStatus("保存二维码失败：" + err.message);
      });
    }

    function updateStatus(id, status) {
      setStatus("正在更新状态...");
      api("/dashboard/shopCode/status", {
        method: "PUT",
        body: JSON.stringify({ id: Number(id), shopCodeId: Number(id), status: Number(status) })
      }).then(loadDashboard).catch(function (err) {
        setStatus("状态更新失败：" + err.message);
      });
    }

    function deleteShop(id) {
      if (!id || !window.confirm("确认删除该门店活码？")) return;
      setStatus("正在删除门店活码...");
      api("/dashboard/shopCode/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), shopCodeId: Number(id) })
      }).then(loadDashboard).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function savePageSetting() {
      var defaultValue;
      try {
        defaultValue = requireJSON(nodes.defaultJSON.value || "{}", "默认样式 JSON");
      } catch (err) {
        setStatus(err.message);
        return;
      }
      var payload = {
        type: Number(nodes.pageType.value || 1),
        title: (nodes.pageTitle.value || "").trim() || "门店活码",
        showType: Number(nodes.showType.value || 1),
        default: defaultValue,
        poster: (nodes.poster.value || "").trim(),
        autoPass: Number(nodes.autoPass.value || 0)
      };
      setStatus("正在保存页面配置...");
      api("/dashboard/shopCode/pageSet", { method: "POST", body: JSON.stringify(payload) }).then(loadPageSetting).then(function () {
        setStatus("页面配置已保存");
      }).catch(function (err) {
        setStatus("页面配置保存失败：" + err.message);
      });
    }

    function batchTags() {
      if (!state.selectedID) return setStatus("请先选择门店");
      setStatus("正在模拟批量标签...");
      api("/dashboard/shopCode/batchContactTags", {
        method: "PUT",
        body: JSON.stringify({ id: state.selectedID, shopCodeId: state.selectedID, tagIds: [910001] })
      }).then(function () {
        setStatus("批量标签请求已完成");
      }).catch(function (err) {
        setStatus("批量标签失败：" + err.message);
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

    function shopID(item) {
      return Number(item && (item.shopCodeId || item.shop_code_id || item.shopId || item.id || 0));
    }

    function typeName(value) {
      var type = Number(value || 0);
      if (type === 1) return "添加店主";
      if (type === 2) return "加入门店群";
      if (type === 3) return "加入城市群";
      return "未知";
    }

    function statusPill(status) {
      return "<span class=\"pill " + (status === 1 ? "on" : "off") + "\">" + (status === 1 ? "开启" : "关闭") + "</span>";
    }

    function requireJSON(raw, label) {
      try {
        return JSON.parse(raw);
      } catch (err) {
        throw new Error(label + " 格式错误");
      }
    }

    function safeJSON(raw) {
      try {
        return JSON.parse(raw);
      } catch (err) {
        return {};
      }
    }

    function compactJSON(value) {
      if (typeof value === "string") {
        try { value = JSON.parse(value); } catch (err) { return value; }
      }
      try {
        return JSON.stringify(value);
      } catch (err) {
        return "";
      }
    }

    function escapeHTML(value) {
      return String(value === undefined || value === null ? "" : value).replace(/[&<>"']/g, function (char) {
        return ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[char];
      });
    }
  })();
  </script>
</body>
</html>`
