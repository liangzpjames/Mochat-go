package dashboard

import (
	"net/http"
	"strings"
)

func NewContactSOPPageHandler() http.Handler {
	return http.HandlerFunc(ServeContactSOPPage)
}

func NewRoomSOPPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomSOPPage)
}

func ServeContactSOPPage(w http.ResponseWriter, r *http.Request) {
	serveSOPDashboardPage(w, r, sopPageConfig{
		Title:          "MoChat Go 个人 SOP",
		Subtitle:       "独立 Go 版个人 SOP 控制台，直接调用同源 dashboard API 管理规则、员工范围、客户范围和启停状态。",
		TokenKey:       "mochat_go_contact_sop_token",
		EntityName:     "个人 SOP",
		EntityIDName:   "contactSopId",
		IndexPath:      "/dashboard/contactSop/index",
		InfoPath:       "/dashboard/contactSop/info",
		StorePath:      "/dashboard/contactSop/store",
		UpdatePath:     "/dashboard/contactSop/update",
		ScopePath:      "/dashboard/contactSop/setEmployee",
		StatePath:      "/dashboard/contactSop/state",
		DestroyPath:    "/dashboard/contactSop/destroy",
		ScopeLabel:     "员工范围 JSON",
		ScopeField:     "employeeIds",
		ScopeDefault:   "[2]",
		ExtraLabel:     "客户范围 JSON",
		ExtraField:     "contactIds",
		ExtraDefault:   "[910001]",
		ScopeCountName: "员工数",
		ExtraCountName: "客户数",
	})
}

func ServeRoomSOPPage(w http.ResponseWriter, r *http.Request) {
	serveSOPDashboardPage(w, r, sopPageConfig{
		Title:          "MoChat Go 群 SOP",
		Subtitle:       "独立 Go 版群 SOP 控制台，直接调用同源 dashboard API 管理规则、客户群范围和启停状态。",
		TokenKey:       "mochat_go_room_sop_token",
		EntityName:     "群 SOP",
		EntityIDName:   "roomSopId",
		IndexPath:      "/dashboard/roomSop/index",
		InfoPath:       "/dashboard/roomSop/info",
		StorePath:      "/dashboard/roomSop/store",
		UpdatePath:     "/dashboard/roomSop/update",
		ScopePath:      "/dashboard/roomSop/setRoom",
		StatePath:      "/dashboard/roomSop/state",
		DestroyPath:    "/dashboard/roomSop/destroy",
		ScopeLabel:     "客户群范围 JSON",
		ScopeField:     "roomIds",
		ScopeDefault:   "[910001]",
		ExtraLabel:     "备用范围 JSON",
		ExtraField:     "",
		ExtraDefault:   "[]",
		ScopeCountName: "群聊数",
		ExtraCountName: "备用",
	})
}

type sopPageConfig struct {
	Title          string
	Subtitle       string
	TokenKey       string
	EntityName     string
	EntityIDName   string
	IndexPath      string
	InfoPath       string
	StorePath      string
	UpdatePath     string
	ScopePath      string
	StatePath      string
	DestroyPath    string
	ScopeLabel     string
	ScopeField     string
	ScopeDefault   string
	ExtraLabel     string
	ExtraField     string
	ExtraDefault   string
	ScopeCountName string
	ExtraCountName string
}

func serveSOPDashboardPage(w http.ResponseWriter, r *http.Request, cfg sopPageConfig) {
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
	_, _ = w.Write([]byte(renderSOPDashboardPage(cfg)))
}

func renderSOPDashboardPage(cfg sopPageConfig) string {
	return strings.NewReplacer(
		"{{TITLE}}", cfg.Title,
		"{{SUBTITLE}}", cfg.Subtitle,
		"{{TOKEN_KEY}}", cfg.TokenKey,
		"{{ENTITY_NAME}}", cfg.EntityName,
		"{{ENTITY_ID_NAME}}", cfg.EntityIDName,
		"{{INDEX_PATH}}", cfg.IndexPath,
		"{{INFO_PATH}}", cfg.InfoPath,
		"{{STORE_PATH}}", cfg.StorePath,
		"{{UPDATE_PATH}}", cfg.UpdatePath,
		"{{SCOPE_PATH}}", cfg.ScopePath,
		"{{STATE_PATH}}", cfg.StatePath,
		"{{DESTROY_PATH}}", cfg.DestroyPath,
		"{{SCOPE_LABEL}}", cfg.ScopeLabel,
		"{{SCOPE_FIELD}}", cfg.ScopeField,
		"{{SCOPE_DEFAULT}}", cfg.ScopeDefault,
		"{{EXTRA_LABEL}}", cfg.ExtraLabel,
		"{{EXTRA_FIELD}}", cfg.ExtraField,
		"{{EXTRA_DEFAULT}}", cfg.ExtraDefault,
		"{{SCOPE_COUNT_NAME}}", cfg.ScopeCountName,
		"{{EXTRA_COUNT_NAME}}", cfg.ExtraCountName,
	).Replace(sopDashboardPageTemplate)
}

const sopDashboardPageTemplate = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{TITLE}}</title>
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
      grid-template-columns: minmax(200px, 1fr) 132px 120px;
      gap: 10px;
      align-items: end;
    }
    .editbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 124px 124px 124px 124px;
      gap: 10px;
      align-items: end;
    }
    .jsonbar {
      display: grid;
      grid-template-columns: minmax(220px, 1fr) minmax(180px, .8fr) minmax(180px, .8fr);
      gap: 10px;
      align-items: stretch;
      margin-top: 10px;
    }
    label {
      display: grid;
      gap: 6px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 650;
    }
    input, textarea, button {
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
    textarea {
      min-height: 82px;
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
    .selected {
      outline: 2px solid #99d6cf;
      outline-offset: -2px;
    }
    .mono {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 12px;
    }
    @media (max-width: 900px) {
      main { width: min(100vw - 20px, 1180px); padding-top: 18px; }
      header, .status { align-items: flex-start; flex-direction: column; }
      .tokenbar, .filterbar, .editbar, .jsonbar { grid-template-columns: 1fr; }
      table { min-width: 900px; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>{{TITLE}}</h1>
        <p class="subtitle">{{SUBTITLE}}</p>
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
      <h2>筛选与编辑</h2>
      <div class="filterbar">
        <label>名称
          <input id="filterName" type="search" placeholder="按名称筛选">
        </label>
        <button id="search" type="button">查询</button>
        <button id="newDraft" type="button" class="secondary">新建草稿</button>
      </div>
      <div class="editbar" style="margin-top: 10px;">
        <label>{{ENTITY_NAME}} 名称
          <input id="sopName" type="text" placeholder="{{ENTITY_NAME}} 名称">
        </label>
        <button id="createSOP" type="button">新增</button>
        <button id="updateSOP" type="button" class="secondary">保存</button>
        <button id="enableSOP" type="button" class="secondary">启用</button>
        <button id="disableSOP" type="button" class="secondary">停用</button>
      </div>
      <div class="jsonbar">
        <label>规则 JSON
          <textarea id="settingJSON" spellcheck="false">[{"name":"首条规则","content":[{"type":"text","value":"SOP提醒"}]}]</textarea>
        </label>
        <label>{{SCOPE_LABEL}}
          <textarea id="scopeJSON" spellcheck="false">{{SCOPE_DEFAULT}}</textarea>
        </label>
        <label>{{EXTRA_LABEL}}
          <textarea id="extraJSON" spellcheck="false">{{EXTRA_DEFAULT}}</textarea>
        </label>
      </div>
      <div class="actions" style="margin-top: 10px;">
        <button id="saveScope" type="button" class="secondary">保存范围</button>
        <button id="deleteSOP" type="button" class="danger">删除当前</button>
      </div>
    </section>

    <div class="status">
      <span id="statusText">准备就绪</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>{{ENTITY_NAME}} 列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width: 76px;">ID</th>
              <th>名称</th>
              <th style="width: 110px;">状态</th>
              <th style="width: 120px;">{{SCOPE_COUNT_NAME}}</th>
              <th style="width: 120px;">{{EXTRA_COUNT_NAME}}</th>
              <th style="width: 130px;">创建人</th>
              <th style="width: 190px;">更新时间</th>
              <th style="width: 250px;">操作</th>
            </tr>
          </thead>
          <tbody id="itemsBody"></tbody>
        </table>
        <div id="itemsEmpty" class="empty">暂无{{ENTITY_NAME}}</div>
      </div>
    </section>

    <section class="panel">
      <h2>当前详情</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width: 76px;">ID</th>
              <th>名称</th>
              <th>规则</th>
              <th>{{SCOPE_LABEL}}</th>
              <th>{{EXTRA_LABEL}}</th>
            </tr>
          </thead>
          <tbody id="detailBody"></tbody>
        </table>
        <div id="detailEmpty" class="empty">请选择{{ENTITY_NAME}}</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    "use strict";

    var config = {
      title: "{{TITLE}}",
      entityName: "{{ENTITY_NAME}}",
      entityIDName: "{{ENTITY_ID_NAME}}",
      tokenKey: "{{TOKEN_KEY}}",
      indexPath: "{{INDEX_PATH}}",
      infoPath: "{{INFO_PATH}}",
      storePath: "{{STORE_PATH}}",
      updatePath: "{{UPDATE_PATH}}",
      scopePath: "{{SCOPE_PATH}}",
      statePath: "{{STATE_PATH}}",
      destroyPath: "{{DESTROY_PATH}}",
      scopeField: "{{SCOPE_FIELD}}",
      extraField: "{{EXTRA_FIELD}}",
      scopeDefault: "{{SCOPE_DEFAULT}}",
      extraDefault: "{{EXTRA_DEFAULT}}"
    };
    var state = { selectedID: 0, total: 0, items: [] };
    var nodes = {};

    document.addEventListener("DOMContentLoaded", init);

    function $(id) {
      return document.getElementById(id);
    }

    function init() {
      [
        "token", "saveToken", "clearToken", "reload", "filterName", "search", "newDraft",
        "sopName", "settingJSON", "scopeJSON", "extraJSON", "createSOP", "updateSOP",
        "enableSOP", "disableSOP", "saveScope", "deleteSOP", "statusText", "summaryText",
        "itemsBody", "itemsEmpty", "detailBody", "detailEmpty"
      ].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadItems);
      nodes.search.addEventListener("click", loadItems);
      nodes.newDraft.addEventListener("click", newDraft);
      nodes.createSOP.addEventListener("click", createSOP);
      nodes.updateSOP.addEventListener("click", updateSOP);
      nodes.enableSOP.addEventListener("click", function () { updateState(1); });
      nodes.disableSOP.addEventListener("click", function () { updateState(0); });
      nodes.saveScope.addEventListener("click", saveScope);
      nodes.deleteSOP.addEventListener("click", deleteSOP);
      loadItems();
    }

    function discoverToken() {
      var saved = localStorage.getItem(config.tokenKey) || "";
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
      localStorage.setItem(config.tokenKey, nodes.token.value.trim());
      setStatus("Token 已保存");
    }

    function clearToken() {
      localStorage.removeItem(config.tokenKey);
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

    function loadItems() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var name = (nodes.filterName.value || "").trim();
      if (name) params.set("name", name);
      setStatus("正在加载" + config.entityName + "...");
      return api(config.indexPath + "?" + params.toString()).then(function (data) {
        var list = pageList(data);
        state.items = list;
        state.total = pageTotal(data, list);
        renderItems(list);
        setStatus("加载完成", config.entityName + " " + state.total + " 条");
        if (list.length > 0) {
          return selectItem(itemID(list[0]));
        }
        state.selectedID = 0;
        renderDetail(null);
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function renderItems(list) {
      nodes.itemsBody.innerHTML = "";
      nodes.itemsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = itemID(item);
        var status = Number(item.state || item.status || 0);
        var tr = document.createElement("tr");
        tr.dataset.id = String(id);
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td>" + statusPill(status) + "</td>" +
          "<td>" + escapeHTML(scopeCount(item)) + "</td>" +
          "<td>" + escapeHTML(extraCount(item)) + "</td>" +
          "<td>" + escapeHTML(item.creatorName || item.creator || "") + "</td>" +
          "<td>" + escapeHTML(item.updatedAt || item.updated_at || item.createdAt || item.created_at || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"select\">详情</button><button class=\"secondary\" data-action=\"toggle\">" + (status === 1 ? "停用" : "启用") + "</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"select\"]").addEventListener("click", function () { selectItem(id); });
        tr.querySelector("[data-action=\"toggle\"]").addEventListener("click", function () {
          state.selectedID = id;
          updateState(status === 1 ? 0 : 1);
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          state.selectedID = id;
          deleteSOP();
        });
        nodes.itemsBody.appendChild(tr);
      });
      highlightSelected();
    }

    function selectItem(id) {
      if (!id) return Promise.resolve();
      state.selectedID = Number(id);
      highlightSelected();
      var params = new URLSearchParams();
      params.set("id", String(id));
      params.set(config.entityIDName, String(id));
      return api(config.infoPath + "?" + params.toString()).then(function (item) {
        fillForm(item);
        renderDetail(item);
      }).catch(function (err) {
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderDetail(item) {
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = item ? "none" : "block";
      if (!item) return;
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(itemID(item)) + "</td>" +
        "<td>" + escapeHTML(item.name || "") + "</td>" +
        "<td class=\"mono\">" + escapeHTML(compactJSON(item.setting || [])) + "</td>" +
        "<td class=\"mono\">" + escapeHTML(compactJSON(scopeValue(item))) + "</td>" +
        "<td class=\"mono\">" + escapeHTML(compactJSON(extraValue(item))) + "</td>";
      nodes.detailBody.appendChild(tr);
    }

    function fillForm(item) {
      if (!item) return;
      nodes.sopName.value = item.name || "";
      nodes.settingJSON.value = compactJSON(item.setting || []);
      nodes.scopeJSON.value = compactJSON(scopeValue(item));
      if (config.extraField) {
        nodes.extraJSON.value = compactJSON(extraValue(item));
        nodes.extraJSON.disabled = false;
      } else {
        nodes.extraJSON.value = config.extraDefault;
        nodes.extraJSON.disabled = true;
      }
    }

    function newDraft() {
      state.selectedID = 0;
      nodes.sopName.value = "";
      nodes.settingJSON.value = "[{\"name\":\"首条规则\",\"content\":[{\"type\":\"text\",\"value\":\"SOP提醒\"}]}]";
      nodes.scopeJSON.value = config.scopeDefault;
      nodes.extraJSON.value = config.extraDefault;
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = "block";
      highlightSelected();
      setStatus("已创建本地草稿");
    }

    function basePayload(requireName) {
      var name = (nodes.sopName.value || "").trim();
      if (requireName && !name) throw new Error("请输入" + config.entityName + "名称");
      var setting = requireJSON(nodes.settingJSON.value || "[]", "规则 JSON");
      var scope = requireJSON(nodes.scopeJSON.value || "[]", "{{SCOPE_LABEL}}");
      var payload = {
        name: name,
        setting: setting,
        state: 1
      };
      payload[config.scopeField] = scope;
      if (config.extraField) {
        payload[config.extraField] = requireJSON(nodes.extraJSON.value || "[]", "{{EXTRA_LABEL}}");
      }
      return payload;
    }

    function createSOP() {
      var payload;
      try {
        payload = basePayload(true);
      } catch (err) {
        setStatus(err.message);
        return;
      }
      setStatus("正在新增" + config.entityName + "...");
      api(config.storePath, { method: "POST", body: JSON.stringify(payload) }).then(loadItems).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function updateSOP() {
      if (!state.selectedID) return setStatus("请先选择" + config.entityName);
      var payload;
      try {
        payload = basePayload(true);
      } catch (err) {
        setStatus(err.message);
        return;
      }
      payload.id = state.selectedID;
      payload[config.entityIDName] = state.selectedID;
      delete payload.state;
      setStatus("正在保存" + config.entityName + "...");
      api(config.updatePath, { method: "PUT", body: JSON.stringify(payload) }).then(function () {
        return loadItems();
      }).catch(function (err) {
        setStatus("保存失败：" + err.message);
      });
    }

    function saveScope() {
      if (!state.selectedID) return setStatus("请先选择" + config.entityName);
      var scope;
      try {
        scope = requireJSON(nodes.scopeJSON.value || "[]", "{{SCOPE_LABEL}}");
      } catch (err) {
        setStatus(err.message);
        return;
      }
      var payload = { id: state.selectedID };
      payload[config.entityIDName] = state.selectedID;
      payload[config.scopeField] = scope;
      setStatus("正在保存范围...");
      api(config.scopePath, { method: "PUT", body: JSON.stringify(payload) }).then(function () {
        return selectItem(state.selectedID);
      }).catch(function (err) {
        setStatus("保存范围失败：" + err.message);
      });
    }

    function updateState(value) {
      if (!state.selectedID) return setStatus("请先选择" + config.entityName);
      var payload = { id: state.selectedID, state: value };
      payload[config.entityIDName] = state.selectedID;
      setStatus("正在更新状态...");
      api(config.statePath, { method: "PUT", body: JSON.stringify(payload) }).then(loadItems).catch(function (err) {
        setStatus("状态更新失败：" + err.message);
      });
    }

    function deleteSOP() {
      if (!state.selectedID) return setStatus("请先选择" + config.entityName);
      if (!window.confirm("确认删除当前" + config.entityName + "？")) return;
      var payload = { id: state.selectedID };
      payload[config.entityIDName] = state.selectedID;
      setStatus("正在删除" + config.entityName + "...");
      api(config.destroyPath, { method: "DELETE", body: JSON.stringify(payload) }).then(loadItems).catch(function (err) {
        setStatus("删除失败：" + err.message);
      });
    }

    function highlightSelected() {
      Array.prototype.forEach.call(nodes.itemsBody.querySelectorAll("tr"), function (tr) {
        tr.classList.toggle("selected", Number(tr.dataset.id) === state.selectedID);
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

    function itemID(item) {
      return Number(item && (item[config.entityIDName] || item.id || item.sopId || 0));
    }

    function scopeValue(item) {
      if (!item) return [];
      if (config.scopeField === "employeeIds") return item.employeeIds || item.employee_ids || [];
      if (config.scopeField === "roomIds") return item.roomIds || item.room_ids || [];
      return item[config.scopeField] || [];
    }

    function extraValue(item) {
      if (!item || !config.extraField) return [];
      if (config.extraField === "contactIds") return item.contactIds || item.contact_ids || [];
      return item[config.extraField] || [];
    }

    function scopeCount(item) {
      if (!item) return 0;
      if (item.employeeNum !== undefined) return item.employeeNum;
      if (item.roomNum !== undefined) return item.roomNum;
      return arrayLen(scopeValue(item));
    }

    function extraCount(item) {
      if (!item || !config.extraField) return 0;
      if (item.contactNum !== undefined) return item.contactNum;
      return arrayLen(extraValue(item));
    }

    function arrayLen(value) {
      return Array.isArray(value) ? value.length : 0;
    }

    function statusPill(status) {
      return "<span class=\"pill " + (status === 1 ? "on" : "off") + "\">" + (status === 1 ? "启用" : "停用") + "</span>";
    }

    function requireJSON(raw, label) {
      try {
        return JSON.parse(raw);
      } catch (err) {
        throw new Error(label + " 格式错误");
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
