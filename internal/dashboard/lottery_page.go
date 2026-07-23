package dashboard

import "net/http"

func NewLotteryPageHandler() http.Handler {
	return http.HandlerFunc(ServeLotteryPage)
}

func ServeLotteryPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(lotteryPageHTML))
}

const lotteryPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go 抽奖活动</title>
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
      grid-template-columns: minmax(170px, 1fr) minmax(210px, 1fr) minmax(150px, .8fr) minmax(170px, 1fr) minmax(170px, 1fr) 132px;
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
      .tokenbar, .createbar, .filterbar { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go 抽奖活动</h1>
        <p class="subtitle">独立 Go 控制台，直接操作抽奖活动、奖品设置、分享链接和中奖客户，不依赖原 MoChat 前端源码。</p>
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

    <section class="panel createbar" aria-label="新建抽奖活动">
      <label>活动名称
        <input id="newName" placeholder="例如：会员福利抽奖">
      </label>
      <label>活动说明
        <input id="newDescription" placeholder="例如：完成任务后参与抽奖">
      </label>
      <label>有效期
        <select id="newTimeType">
          <option value="1">永久有效</option>
          <option value="2">指定时间</option>
        </select>
      </label>
      <label>开始时间
        <input id="newStart" type="datetime-local">
      </label>
      <label>结束时间
        <input id="newEnd" type="datetime-local">
      </label>
      <button id="createLottery" type="button">新增活动</button>
    </section>

    <section class="panel filterbar" aria-label="筛选">
      <label>活动名称
        <input id="searchName" placeholder="按活动名称筛选">
      </label>
      <button id="searchLotteries" class="secondary" type="button">查询</button>
    </section>

    <div class="status">
      <span id="statusText">准备加载</span>
      <strong id="summaryText"></strong>
    </div>

    <section class="panel">
      <h2>抽奖活动列表</h2>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th style="width:70px;">ID</th>
              <th>活动名称</th>
              <th>说明</th>
              <th style="width:90px;">模板</th>
              <th style="width:88px;">客户数</th>
              <th style="width:88px;">中奖数</th>
              <th style="width:150px;">活动时间</th>
              <th style="width:140px;">创建人</th>
              <th style="width:240px;">操作</th>
            </tr>
          </thead>
          <tbody id="lotteriesBody"></tbody>
        </table>
        <div id="lotteriesEmpty" class="empty">暂无抽奖活动</div>
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
              <th>奖品 JSON</th>
              <th>抽奖限制</th>
              <th>中奖限制</th>
              <th>兑奖设置</th>
              <th style="width:180px;">分享链接</th>
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
              <th style="width:86px;">抽奖数</th>
              <th style="width:86px;">中奖数</th>
              <th style="width:90px;">状态</th>
              <th style="width:90px;">核销</th>
              <th>奖品</th>
              <th>兑换码</th>
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
    var tokenKey = "mochat_go_lottery_token";
    var state = { total: 0, selectedID: 0 };
    var nodes = {};

    function $(id) { return document.getElementById(id); }

    function init() {
      ["token", "saveToken", "clearToken", "reload", "newName", "newDescription", "newTimeType", "newStart", "newEnd", "createLottery", "searchName", "searchLotteries", "statusText", "summaryText", "lotteriesBody", "lotteriesEmpty", "detailTitle", "detailBody", "detailEmpty", "contactsTitle", "contactsBody", "contactsEmpty"].forEach(function (id) {
        nodes[id] = $(id);
      });
      nodes.token.value = discoverToken();
      setDefaultTimes();
      nodes.saveToken.addEventListener("click", saveToken);
      nodes.clearToken.addEventListener("click", clearToken);
      nodes.reload.addEventListener("click", loadLotteries);
      nodes.searchLotteries.addEventListener("click", loadLotteries);
      nodes.createLottery.addEventListener("click", createLottery);
      loadLotteries();
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

    function loadLotteries() {
      var params = new URLSearchParams();
      params.set("page", "1");
      params.set("perPage", "20");
      var name = (nodes.searchName.value || "").trim();
      if (name) params.set("name", name);
      setStatus("正在加载抽奖活动...");
      return api("/dashboard/lottery/index?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        state.total = data && data.page && data.page.total ? Number(data.page.total) : list.length;
        renderLotteries(list);
        setStatus("加载完成", "活动 " + state.total + " 条");
        if (list.length > 0) {
          var firstID = lotteryID(list[0]);
          if (firstID) return loadDetail(firstID);
        }
        renderDetail(null, null);
        renderContacts([]);
      }).catch(function (err) {
        setStatus("加载失败：" + err.message);
      });
    }

    function renderLotteries(list) {
      nodes.lotteriesBody.innerHTML = "";
      nodes.lotteriesEmpty.style.display = list.length > 0 ? "none" : "block";
      list.forEach(function (item) {
        var id = lotteryID(item);
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(id) + "</td>" +
          "<td>" + escapeHTML(item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.description || "") + "</td>" +
          "<td>" + escapeHTML(templateText(item.type)) + "</td>" +
          "<td>" + escapeHTML(item.contactNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.winNum || 0) + "</td>" +
          "<td>" + escapeHTML(timeText(item)) + "</td>" +
          "<td>" + escapeHTML(item.createUserName || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"detail\">详情</button><button class=\"secondary\" data-action=\"show\">中奖展示</button><button class=\"danger\" data-action=\"delete\">删除</button></div></td>";
        tr.querySelector("[data-action=\"detail\"]").addEventListener("click", function () {
          loadDetail(id);
        });
        tr.querySelector("[data-action=\"show\"]").addEventListener("click", function () {
          updateLottery(id, { isShow: 1 });
        });
        tr.querySelector("[data-action=\"delete\"]").addEventListener("click", function () {
          deleteLottery(id);
        });
        nodes.lotteriesBody.appendChild(tr);
      });
    }

    function loadDetail(id) {
      if (!id) return Promise.resolve();
      state.selectedID = Number(id);
      setStatus("正在加载活动详情...");
      return Promise.all([
        api("/dashboard/lottery/show?id=" + encodeURIComponent(id)),
        api("/dashboard/lottery/share?id=" + encodeURIComponent(id))
      ]).then(function (result) {
        renderDetail(result[0], result[1]);
        setStatus("详情加载完成", "活动 " + state.total + " 条");
        return loadContacts(id);
      }).catch(function (err) {
        renderDetail(null, null);
        renderContacts([]);
        setStatus("详情加载失败：" + err.message);
      });
    }

    function renderDetail(item, share) {
      nodes.detailBody.innerHTML = "";
      nodes.detailEmpty.style.display = item ? "none" : "block";
      nodes.detailTitle.textContent = item && item.name ? item.name : "未选择活动";
      if (!item) return;
      var tr = document.createElement("tr");
      tr.innerHTML =
        "<td>" + escapeHTML(lotteryID(item)) + "</td>" +
        "<td>" + escapeHTML(compactJSON(item.prizeSet || item.prize_set || [])) + "</td>" +
        "<td>" + escapeHTML(compactJSON(item.drawSet || item.draw_set || {})) + "</td>" +
        "<td>" + escapeHTML(compactJSON(item.winSet || item.win_set || {})) + "</td>" +
        "<td>" + escapeHTML(compactJSON(item.exchangeSet || item.exchange_set || {})) + "</td>" +
        "<td>" + escapeHTML((share && (share.link || share.url || share.shareUrl)) || item.shareUrl || "") + "</td>";
      nodes.detailBody.appendChild(tr);
    }

    function loadContacts(id) {
      if (!id) return Promise.resolve();
      var params = new URLSearchParams();
      params.set("lotteryId", String(id));
      params.set("page", "1");
      params.set("perPage", "20");
      return api("/dashboard/lottery/showContact?" + params.toString()).then(function (data) {
        var list = data && Array.isArray(data.list) ? data.list : [];
        renderContacts(list);
      }).catch(function (err) {
        renderContacts([]);
        setStatus("客户列表加载失败：" + err.message);
      });
    }

    function renderContacts(list) {
      nodes.contactsBody.innerHTML = "";
      nodes.contactsEmpty.style.display = list.length > 0 ? "none" : "block";
      nodes.contactsTitle.textContent = state.selectedID ? ("活动 " + state.selectedID) : "未选择活动";
      list.forEach(function (item) {
        var contactID = contactRecordID(item);
        var writeOff = Number(item.writeOff || 0);
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + escapeHTML(contactID) + "</td>" +
          "<td>" + escapeHTML(item.nickname || item.name || "") + "</td>" +
          "<td>" + escapeHTML(item.drawNum || 0) + "</td>" +
          "<td>" + escapeHTML(item.winNum || 0) + "</td>" +
          "<td><span class=\"pill " + (Number(item.status || 0) === 1 ? "on" : "off") + "\">" + (Number(item.status || 0) === 1 ? "已完成" : "未完成") + "</span></td>" +
          "<td><span class=\"pill " + (writeOff === 1 ? "on" : "off") + "\">" + (writeOff === 1 ? "已核销" : "未核销") + "</span></td>" +
          "<td>" + escapeHTML(item.prizeName || "") + "</td>" +
          "<td>" + escapeHTML(item.receiveCode || "") + "</td>" +
          "<td>" + escapeHTML(item.updatedAt || "") + "</td>" +
          "<td><div class=\"actions\"><button class=\"secondary\" data-action=\"writeoff\">核销</button></div></td>";
        tr.querySelector("[data-action=\"writeoff\"]").addEventListener("click", function () {
          writeOffContact(contactID);
        });
        nodes.contactsBody.appendChild(tr);
      });
    }

    function createLottery() {
      var name = (nodes.newName.value || "").trim();
      if (!name) {
        setStatus("请输入活动名称");
        return;
      }
      var description = (nodes.newDescription.value || "").trim();
      var payload = {
        name: name,
        description: description || name + "说明",
        type: "roulette",
        timeType: Number(nodes.newTimeType.value || 1),
        startTime: normalizeDateTime(nodes.newStart.value),
        endTime: normalizeDateTime(nodes.newEnd.value),
        contactTags: [],
        prizeSet: [{ name: "一等奖", total: 1, probability: 100 }],
        isShow: 1,
        exchangeSet: { type: 2, code: "GO-LOTTERY" },
        drawSet: { daily: 1, total: 1 },
        winSet: { max: 1 },
        corpCard: {}
      };
      setStatus("正在新增抽奖活动...");
      api("/dashboard/lottery/store", { method: "POST", body: JSON.stringify(payload) }).then(function () {
        nodes.newName.value = "";
        nodes.newDescription.value = "";
        setDefaultTimes();
        return loadLotteries();
      }).then(function () {
        setStatus("新增完成", "活动 " + state.total + " 条");
      }).catch(function (err) {
        setStatus("新增失败：" + err.message);
      });
    }

    function updateLottery(id, patch) {
      if (!id) return;
      patch.id = Number(id);
      patch.lotteryId = Number(id);
      setStatus("正在更新抽奖活动...");
      api("/dashboard/lottery/update", {
        method: "PUT",
        body: JSON.stringify(patch)
      }).then(loadLotteries).catch(function (err) {
        setStatus("更新失败：" + err.message);
      });
    }

    function writeOffContact(contactID) {
      if (!state.selectedID || !contactID) return;
      setStatus("正在核销中奖客户...");
      var params = new URLSearchParams();
      params.set("lotteryId", String(state.selectedID));
      params.set("contactId", String(contactID));
      api("/dashboard/lottery/writeOff?" + params.toString()).then(function () {
        return loadContacts(state.selectedID);
      }).catch(function (err) {
        setStatus("核销失败：" + err.message);
      });
    }

    function deleteLottery(id) {
      if (!id || !window.confirm("确认删除这条抽奖活动？")) return;
      setStatus("正在删除抽奖活动...");
      api("/dashboard/lottery/destroy", {
        method: "DELETE",
        body: JSON.stringify({ id: Number(id), lotteryId: Number(id) })
      }).then(loadLotteries).catch(function (err) {
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

    function lotteryID(item) {
      return item.lotteryId || item.lottery_id || item.id || "";
    }

    function contactRecordID(item) {
      return item.contactRecordId || item.id || item.contact_record_id || "";
    }

    function templateText(value) {
      return value === "roulette" || !value ? "转盘" : value;
    }

    function timeText(item) {
      if (Number(item.timeType || item.time_type || 1) === 1) return "永久有效";
      return (item.startTime || item.start_time || "") + " 至 " + (item.endTime || item.end_time || "");
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
