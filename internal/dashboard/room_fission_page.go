package dashboard

import "net/http"

func NewRoomFissionPageHandler() http.Handler {
	return http.HandlerFunc(ServeRoomFissionPage)
}

func ServeRoomFissionPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(roomFissionPageHTML))
}

const roomFissionPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 群裂变</title>
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
      grid-template-columns: minmax(190px, 1fr) 136px 136px 136px 132px;
      gap: 10px;
      align-items: end;
    }
    .invitebar {
      display: grid;
      grid-template-columns: minmax(160px, .7fr) minmax(220px, 1.2fr) minmax(220px, 1.2fr) 132px 132px;
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
      .tokenbar, .createbar, .invitebar, .filterbar { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 群裂变</h1>
        <p class="subtitle">独立 Go 控制台，直接管理群裂变活动、海报欢迎语、群聊数据、参与客户和核销状态，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建群裂变">
      <label>活动名称
        <input id="newName" placeholder="例如：社群拉新裂变">
      </label>
      <label>结束时间
        <input id="newEnd" type="datetime-local">
      </label>
      <label>目标人数
        <input id="newTarget" inputmode="numeric" value="3">
      </label>
      <label>群人数上限
        <input id="newRoomMax" inputmode="numeric" value="200">
      </label>
      <button id="createFission" type="button">新增活动</button>
    </section>

    <section class="panel invitebar" aria-label="邀请与核销">
      <label>活动 ID
        <input id="selectedFissionID" inputmode="numeric" placeholder="自动">
      </label>
      <label>邀请文案
        <input id="inviteText" placeholder="邀请好友进群即可完成任务">
      </label>
      <label>邀请链接标题
        <input id="inviteTitle" placeholder="群裂变邀请">
      </label>
      <button id="saveInvite" class="secondary" type="button">保存邀请</button>
      <button id="finishFission" type="button">标记完成</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>活动名称
        <input id="searchName" placeholder="按活动名称筛选">
      </label>
      <button id="searchFissions" class="secondary" type="button">查询</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>群裂变活动列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>活动名称</th>
              <th style="width:90px;">状态</th>
              <th style="width:90px;">目标</th>
              <th style="width:100px;">客户</th>
              <th style="width:100px;">完成</th>
              <th style="width:86px;">群聊</th>
              <th style="width:160px;">结束时间</th>
              <th style="width:120px;">创建人</th>
              <th style="width:260px;">操作</th>
            </tr>
          </thead>
          <tbody id="fissionsBody"></tbody>
        </table>
        <div id="fissionsEmpty" class="empty">暂无群裂变活动</div>
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
              <th>基础配置</th>
              <th>海报</th>
              <th>欢迎语</th>
              <th>邀请设置</th>
              <th>分享链接</th>
            </tr>
          </thead>
          <tbody id="detailBody"></tbody>
        </table>
        <div id="detailEmpty" class="empty">暂无活动详情</div>
      </div>
    </section>

    <section class="panel">
      <div class="detailhead">
        <h2>群聊数据</h2>
        <span id="roomsTitle" class="subtitle">未选择活动</span>
      </div>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:80px;">记录 ID</th>
              <th>群聊</th>
              <th>二维码</th>
              <th style="width:90px;">上限</th>
              <th style="width:90px;">客户</th>
              <th style="width:90px;">入群</th>
              <th style="width:150px;">更新时间</th>
            </tr>
          </thead>
          <tbody id="roomsBody"></tbody>
        </table>
        <div id="roomsEmpty" class="empty">暂无群聊数据</div>
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
              <th style="width:88px;">邀请数</th>
              <th style="width:90px;">完成</th>
              <th style="width:90px;">领取</th>
              <th style="width:90px;">入群</th>
              <th style="width:90px;">核销</th>
              <th>员工</th>
              <th style="width:130px;">更新时间</th>
              <th style="width:116px;">操作</th>
            </tr>
          </thead>
          <tbody id="contactsBody"></tbody>
        </table>
        <div id="contactsEmpty" class="empty">暂无参与客户</div>
      </div>
    </section>
  </main>

  <script>
  (function () {
    var tokenKey = "mochat_go_room_fission_token";
    var state = { total: 0, selectedID: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newName", "newEnd", "newTarget", "newRoomMax", "createFission", "selectedFissionID", "inviteText", "inviteTitle", "saveInvite", "finishFission", "searchName", "searchFissions", "statusText", "summaryText", "fissionsBody", "fissionsEmpty", "detailTitle", "detailBody", "detailEmpty", "roomsTitle", "roomsBody", "roomsEmpty", "contactsTitle", "contactsBody", "contactsEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      setDefaultEnd();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadFissions);
      nodes.searchFissions.addEventListener("click", loadFissions);
      nodes.createFission.addEventListener("click", createFission);
      nodes.saveInvite.addEventListener("click", saveInvite);
      nodes.finishFission.addEventListener("click", function () {
        updateFission(Number(nodes.selectedFissionID.value || state.selectedID), { status: 2 });
      });
      loadFissions();
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

    function loadFissions() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var name = (nodes.searchName.value || "").trim();
      if (name) params.set("activeName", name);
      setStatus("正在加载群裂变活动...");
      return api("/dashboard/roomFission/index?" + params.toString()).then(function (data) {
        var list = pageList(data);
        state.total = pageTotal(data, list);
        renderFissions(list);
        setStatus("加载完成", "活动 " + state.total + " 条");
        if (list.length > 0) {
          var firstID = fissionID(list[0]);
          if (firstID) return loadDetail(firstID);
        }
        renderDetail(null, null);
        renderRooms([]);
        renderContacts([]);
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function renderFissions(list) {
      nodes.fissionsBody.innerHTML = "";
      nodes.fissionsEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = fissionID(item);
        var status = Number(item.status || 0);
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.activeName || item.active_name || item.name || "") + "</td>" +
          "<td><span class=\"pill " + (status === 2 ? "on" : "off") + "\">" + escapeHTML(item.statusText || item.status_text || statusText(status)) + "</span></td>" +
          "<td>" + escapeHTML(item.targetCount || item.target_count || 0) + "</td>" +
          "<td>" + escapeHTML(item.contactNum || item.contact_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.completeNum || item.complete_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.roomNum || item.room_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.endTime || item.end_time || "") + "</td>" +
          "<td>" + escapeHTML(item.createUserName || item.create_user_name || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"detail\">详情</button><button class=\"secondary\" data-action=\"finish\">完成</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"detail\"]").addEventListener("click", function () {
          loadDetail(id);
        });
        tr.querySelector("[data-action=\"finish\"]").addEventListener("click", function () {
          updateFission(id, { status: 2 });
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deleteFission(id);
        });
        nodes.fissionsBody.appendChild(tr);
      });
    }

    function loadDetail(id) {
      if (!id) return Promise.resolve();
      state.selectedID = Number(id);
      nodes.selectedFissionID.value = String(id);
      setStatus("正在加载活动详情...");
      return Promise.all([
        api("/dashboard/roomFission/info?id=" + encodeURIComponent(id)),
        api("/dashboard/roomFission/show?id=" + encodeURIComponent(id)),
        api("/dashboard/roomFission/showRoom?fissionId=" + encodeURIComponent(id) + "&page=1&perPage=20"),
        api("/dashboard/roomFission/showContact?fissionId=" + encodeURIComponent(id) + "&page=1&perPage=20")
      ]).then(function (result) {
        renderDetail(result[0], result[1]);
        renderRooms(pageList(result[2]));
        renderContacts(pageList(result[3]));
        var invite = result[0] && result[0].invite ? result[0].invite : {};
        nodes.inviteText.value = invite.text || "";
        nodes.inviteTitle.value = invite.linkTitle || invite.link_title || "";
        setStatus("详情加载完成", "活动 " + state.total + " 条");
      }).catch(function (err) {
        renderDetail(null, null);
        renderRooms([]);
        renderContacts([]);
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderDetail(info, show) {
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = info ? "none" : "block";
      var fission = info && info.fission ? info.fission : null;
      nodes.detailTitle.textContent = fission && fission.activeName ? fission.activeName : "未选择活动";
      if (!info || !fission) return;
      var overview = (show && show.overview) || info.overview || {};
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(fissionID(fission)) + "</td>" +
        "<td>" + escapeHTML("目标 " + (fission.targetCount || 0) + "；客户 " + (overview.contactNum || 0) + "；完成 " + (overview.completeNum || 0) + "；入群 " + (overview.joinRoomNum || 0)) + "</td>" +
        "<td>" + escapeHTML(compactJSON(info.poster || {})) + "</td>" +
        "<td>" + escapeHTML(compactJSON(info.welcome || {})) + "</td>" +
        "<td>" + escapeHTML(compactJSON(info.invite || {})) + "</td>" +
        "<td>" + escapeHTML(info.link || info.url || info.qrcodeUrl || "") + "</td>";
      nodes.detailBody.appendChild(tr);
    }

    function renderRooms(list) {
      nodes.roomsBody.innerHTML = "";
      nodes.roomsEmpty.style.display = list.length > 0 ? "none" : "block";
      nodes.roomsTitle.textContent = state.selectedID ? ("活动 " + state.selectedID) : "未选择活动";
      list.forEach(function (item) {
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(item.roomRecordId || item.id || "") + "</td>" +
          "<td>" + escapeHTML(roomText(item.room)) + "</td>" +
          "<td>" + escapeHTML(item.roomQrcode || item.room_qrcode || item.roomWxQrcode || item.room_wx_qrcode || "") + "</td>" +
          "<td>" + escapeHTML(item.roomMax || item.room_max || 0) + "</td>" +
          "<td>" + escapeHTML(item.contactNum || item.contact_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.joinNum || item.join_num || 0) + "</td>" +
          "<td>" + escapeHTML(item.updatedAt || item.updated_at || "") + "</td>";
        nodes.roomsBody.appendChild(tr);
      });
    }

    function renderContacts(list) {
      nodes.contactsBody.innerHTML = "";
      nodes.contactsEmpty.style.display = list.length > 0 ? "none" : "block";
      nodes.contactsTitle.textContent = state.selectedID ? ("活动 " + state.selectedID) : "未选择活动";
      list.forEach(function (item) {
        var id = item.contactRecordId || item.id || "";
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.nickname || item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.inviteCount || item.invite_count || 0) + "</td>" +
          "<td>" + statusPill(Number(item.status || 0), "已完成", "未完成") + "</td>" +
          "<td>" + statusPill(Number(item.receiveStatus || item.receive_status || 0), "已领取", "未领取") + "</td>" +
          "<td>" + statusPill(Number(item.joinStatus || item.join_status || 0), "已入群", "未入群") + "</td>" +
          "<td>" + statusPill(Number(item.writeOff || item.write_off || 0), "已核销", "未核销") + "</td>" +
          "<td>" + escapeHTML(item.employee || "") + "</td>" +
          "<td>" + escapeHTML(item.updatedAt || item.updated_at || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"writeoff\">核销</button></div></td>";
        tr.querySelector("[data-action=\"writeoff\"]").addEventListener("click", function () {
          writeOffContact(id);
        });
        nodes.contactsBody.appendChild(tr);
      });
    }

    function createFission() {
      var name = (nodes.newName.value || "").trim();
      if (!name) {
        setStatus("请输入活动名称");
        return;
      }
      var roomMax = Number(nodes.newRoomMax.value || 200);
      var payload = {
        fission: {
          officialAccountId: 0,
          activeName: name,
          endTime: normalizeDateTime(nodes.newEnd.value),
          targetCount: Number(nodes.newTarget.value || 1),
          newFriend: 1,
          deleteInvalid: 1,
          receiveEmployees: [2],
          autoPass: 1,
          status: 1
        },
        poster: {
          coverPic: "",
          avatarShow: 1,
          nicknameShow: 1,
          nicknameColor: "#17202a",
          qrcodeW: "120",
          qrcodeH: "120",
          qrcodeX: "40",
          qrcodeY: "40"
        },
        rooms: [{
          roomQrcode: "qrcode/front-room-fission.png",
          roomMax: roomMax,
          room: { id: 910001, name: "前端联调客户群" }
        }],
        welcome: {
          text: "欢迎参与群裂变活动",
          linkTitle: name,
          linkDesc: "邀请好友加入客户群",
          linkPic: "",
          linkWxUrl: "",
          templateId: ""
        },
        invite: {
          type: 2,
          employees: [],
          chooseContact: {},
          text: "邀请好友加入客户群即可完成任务",
          linkTitle: name,
          linkDesc: "群裂变邀请",
          linkPic: "",
          wxLinkPic: ""
        }
      };
      setStatus("正在新增群裂变活动...");
      api("/dashboard/roomFission/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newName.value = "";
        setDefaultEnd();
        return loadFissions();
      }).then(function () {
        setStatus("新增完成", "活动 " + state.total + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function saveInvite() {
      var id = Number(nodes.selectedFissionID.value || state.selectedID || 0);
      if (!id) {
        setStatus("请先选择活动");
        return;
      }
      var payload = {
        fissionId: id,
        invite: {
          type: 1,
          employees: [],
          chooseContact: {},
          text: (nodes.inviteText.value || "").trim() || "邀请好友进群即可完成任务",
          linkTitle: (nodes.inviteTitle.value || "").trim() || "群裂变邀请",
          linkDesc: "邀请好友加入客户群",
          linkPic: "",
          wxLinkPic: ""
        }
      };
      setStatus("正在保存邀请设置...");
      api("/dashboard/roomFission/invite", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        return loadDetail(id);
      }).catch(function (err) {
        setStatus("保存邀请失败：" + err.message);
      });
    }

    function updateFission(id, patch) {
      if (!id) return;
      var payload = { id: Number(id), fissionId: Number(id), fission: { id: Number(id) } };
      Object.keys(patch).forEach(function (key) {
        payload.fission[key] = patch[key];
      });
      setStatus("正在更新群裂变活动...");
      api("/dashboard/roomFission/update", {
        method: "PUT",
        body: JSON.stringify(payload)
      }).then(loadFissions).catch(function (err) {
        setStatus("更新失败：" + err.message);
      });
    }

    function writeOffContact(contactID) {
      if (!state.selectedID || !contactID) return;
      setStatus("正在核销参与客户...");
      var params = new URLSearchParams();
      params.set("fissionId", String(state.selectedID));
      params.set("contactId", String(contactID));
      api("/dashboard/roomFission/writeOff?" + params.toString()).then(function () {
        return loadDetail(state.selectedID);
      }).catch(function (err) {
        setStatus("核销失败：" + err.message);
      });
    }

    function deleteFission(id) {
      if (!id || !window.confirm("确认删除该群裂变活动？")) return;
      setStatus("正在删除群裂变活动...");
      api("/dashboard/roomFission/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), fissionId: Number(id) })
      }).then(loadFissions).catch(function (err) {
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

    function fissionID(item) {
      return item.fissionId || item.fission_id || item.id || 0;
    }

    function statusText(status) {
      if (status === 2) return "已完成";
      if (status === 0) return "未开始";
      return "进行中";
    }

    function statusPill(value, yes, no) {
      return "<span class=\"pill " + (value === 1 ? "on" : "off") + "\">" + (value === 1 ? yes : no) + "</span>";
    }

    function roomText(value) {
      if (!value) return "";
      if (typeof value === "string") {
        try { value = JSON.parse(value); } catch (err) { return value; }
      }
      if (value && value.name) return value.name + (value.id ? " #" + value.id : "");
      return compactJSON(value);
    }

    function compactJSON(value) {
      if (value === undefined || value === null || value === "") return "";
      if (typeof value === "string") {
        try { return JSON.stringify(JSON.parse(value)); } catch (err) { return value; }
      }
      try { return JSON.stringify(value); } catch (err) { return String(value); }
    }

    function normalizeDateTime(value) {
      if (!value) return "";
      return value.replace("T", " ") + (value.length === 16 ? ":00" : "");
    }

    function setDefaultEnd() {
      var end = new Date(Date.now() + 7 * 86400000);
      end.setMinutes(end.getMinutes() - end.getTimezoneOffset());
      nodes.newEnd.value = end.toISOString().slice(0, 16);
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
