package dashboard

import "net/http"

func NewSaaSAlertPageHandler() http.Handler {
	return http.HandlerFunc(ServeSaaSAlertPage)
}

func ServeSaaSAlertPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(saasAlertPageHTML))
}

const saasAlertPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go SaaS 告警管理</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f7f8fa;
      --panel: #ffffff;
      --line: #d8dee8;
      --text: #17202a;
      --muted: #667085;
      --primary: #0f766e;
      --primary-strong: #115e59;
      --danger: #b42318;
      --warning: #b54708;
      --ok: #027a48;
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
      margin-bottom: 18px;
    }
    h1 {
      margin: 0;
      font-size: 24px;
      font-weight: 700;
      letter-spacing: 0;
    }
    .subtitle {
      margin: 6px 0 0;
      color: var(--muted);
      font-size: 13px;
      line-height: 1.5;
    }
    .toolbar, .tokenbar, .settings {
      display: grid;
      grid-template-columns: 1.2fr 150px 190px 100px;
      gap: 10px;
      align-items: end;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 14px;
      margin-bottom: 12px;
    }
    .tokenbar {
      grid-template-columns: minmax(240px, 1fr) 120px;
    }
    .settings {
      grid-template-columns: 150px minmax(240px, 1fr) 170px 150px;
    }
    .settings .wide {
      grid-column: span 2;
    }
    .settings .actions {
      display: flex;
      gap: 10px;
      align-items: end;
    }
    .alerttypes {
      min-height: 38px;
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px 16px;
    }
    label {
      display: grid;
      gap: 6px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 600;
    }
    input, select, textarea, button {
      height: 38px;
      border-radius: 6px;
      border: 1px solid var(--line);
      font: inherit;
      letter-spacing: 0;
    }
    input, select, textarea {
      width: 100%;
      padding: 0 10px;
      background: #fff;
      color: var(--text);
    }
    textarea {
      height: 72px;
      padding: 9px 10px;
      resize: vertical;
      line-height: 1.45;
    }
    .checkline {
      min-height: 38px;
      display: flex;
      align-items: center;
      gap: 8px;
      color: var(--text);
      font-size: 13px;
      font-weight: 600;
    }
    .checkline input {
      width: 16px;
      height: 16px;
      padding: 0;
    }
    button {
      padding: 0 14px;
      background: var(--primary);
      color: #fff;
      border-color: var(--primary);
      font-weight: 650;
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
    button:disabled {
      cursor: not-allowed;
      opacity: .55;
    }
    .status {
      min-height: 38px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      color: var(--muted);
      font-size: 13px;
      margin: 8px 0 10px;
    }
    .status strong { color: var(--text); }
    .tablewrap {
      overflow-x: auto;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
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
      font-weight: 700;
    }
    tr:last-child td { border-bottom: 0; }
    .pill {
      display: inline-flex;
      align-items: center;
      height: 24px;
      padding: 0 8px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 700;
      background: #eef4ff;
      color: #1849a9;
      white-space: nowrap;
    }
    .pill.open { background: #fff3e8; color: var(--warning); }
    .pill.resolved { background: #ecfdf3; color: var(--ok); }
    .empty {
      padding: 36px 16px;
      color: var(--muted);
      text-align: center;
    }
    @media (max-width: 760px) {
      main { width: min(100vw - 20px, 1180px); padding-top: 18px; }
      header { display: block; }
      h1 { font-size: 20px; }
      .toolbar, .tokenbar, .settings { grid-template-columns: 1fr; }
      .settings .wide { grid-column: auto; }
      .settings .actions { display: grid; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>MoChat Go SaaS 告警管理</h1>
        <p class="subtitle">读取当前租户的额度告警，支持筛选、刷新和解决打开的告警。</p>
      </div>
      <button id="reload" type="button">刷新告警</button>
    </header>

    <section class="tokenbar" aria-label="鉴权">
      <label>Dashboard JWT
        <input id="token" type="password" autocomplete="off" placeholder="Bearer token 或纯 token">
      </label>
      <button id="saveToken" class="secondary" type="button">保存 Token</button>
    </section>

    <section class="settings" aria-label="通知配置">
      <label>启用通知
        <span class="checkline"><input id="settingEnabled" type="checkbox"> Webhook</span>
      </label>
      <label>Webhook URL
        <input id="webhookUrl" placeholder="Webhook 绝对 URL">
      </label>
      <label>Webhook Secret
        <input id="webhookSecret" type="password" autocomplete="off" placeholder="留空保持不变">
      </label>
      <label>清空密钥
        <span class="checkline"><input id="clearWebhookSecret" type="checkbox"> 清空</span>
      </label>
      <label>请求超时秒数
        <input id="webhookTimeoutSeconds" type="number" min="1" max="300" step="1">
      </label>
      <label>HTTP 重试次数
        <input id="webhookRetryAttempts" type="number" min="1" max="10" step="1">
      </label>
      <label>HTTP 重试间隔毫秒
        <input id="webhookRetryDelayMs" type="number" min="0" max="600000" step="1">
      </label>
      <label>Outbox 最大次数
        <input id="notificationMaxAttempts" type="number" min="1" max="20" step="1">
      </label>
      <label class="wide">标题模板
        <textarea id="webhookTitleTemplate"></textarea>
      </label>
      <label class="wide">正文模板
        <textarea id="webhookBodyTemplate"></textarea>
      </label>
      <label>Outbox 重试秒数
        <input id="notificationRetryDelaySeconds" type="number" min="0" max="86400" step="1">
      </label>
      <label>最低严重级别
        <select id="minimumSeverity">
          <option value="warning">Warning</option>
          <option value="critical">Critical</option>
        </select>
      </label>
      <label class="wide">通知事件
        <span class="alerttypes">
          <span class="checkline"><input id="alertTypeQuota" type="checkbox"> 额度告警</span>
          <span class="checkline"><input id="alertTypeRenewal" type="checkbox"> 续费提醒</span>
          <span class="checkline"><input id="alertTypePaymentFailed" type="checkbox"> 支付失败</span>
          <span class="checkline"><input id="alertTypeTaskSla" type="checkbox"> 任务 SLA</span>
          <span class="checkline"><input id="alertTypeOperationQueue" type="checkbox"> 待办认领</span>
        </span>
      </label>
      <label class="wide">其他事件
        <input id="alertTypeOther" placeholder="custom_alert, another_alert">
      </label>
      <label>免打扰
        <span class="checkline"><input id="quietHoursEnabled" type="checkbox"> 启用</span>
      </label>
      <label>免打扰开始
        <input id="quietHoursStart" type="time" value="22:00">
      </label>
      <label>免打扰结束
        <input id="quietHoursEnd" type="time" value="08:00">
      </label>
      <label>时区
        <input id="settingTimezone" list="settingTimezoneOptions" value="Asia/Shanghai">
        <datalist id="settingTimezoneOptions">
          <option value="Asia/Shanghai"></option>
          <option value="Asia/Hong_Kong"></option>
          <option value="UTC"></option>
          <option value="America/New_York"></option>
        </datalist>
      </label>
      <label>每小时上限
        <input id="hourlyLimit" type="number" min="0" max="10000" step="1" value="0">
      </label>
      <div class="actions">
        <button id="loadSetting" class="secondary" type="button">读取配置</button>
        <button id="saveSetting" type="button">保存配置</button>
      </div>
    </section>

    <section class="toolbar" aria-label="筛选">
      <label>指标
        <input id="metric" value="async_executions" placeholder="async_executions">
      </label>
      <label>状态
        <select id="statusFilter">
          <option value="open">打开</option>
          <option value="all">全部</option>
          <option value="resolved">已解决</option>
        </select>
      </label>
      <label>告警类型
        <input id="alertType" value="quota_exceeded" placeholder="quota_exceeded">
      </label>
      <button id="apply" type="button">查询</button>
    </section>

    <div class="status">
      <span id="message">准备读取告警。</span>
      <strong id="count">0 条</strong>
    </div>

    <section class="tablewrap" aria-label="告警列表">
      <table>
        <thead>
          <tr>
            <th style="width: 84px;">状态</th>
            <th style="width: 150px;">指标</th>
            <th style="width: 110px;">当前/上限</th>
            <th style="width: 82px;">次数</th>
            <th style="width: 150px;">来源</th>
            <th>信息</th>
            <th style="width: 160px;">最后出现</th>
            <th style="width: 96px;">操作</th>
          </tr>
        </thead>
        <tbody id="rows"></tbody>
      </table>
      <div id="empty" class="empty">暂无数据</div>
    </section>
  </main>

  <script>
    (function () {
      var storageKey = "mochat_go_saas_alert_token";
      var tokenInput = document.getElementById("token");
      var metricInput = document.getElementById("metric");
      var statusFilter = document.getElementById("statusFilter");
      var alertTypeInput = document.getElementById("alertType");
      var settingEnabled = document.getElementById("settingEnabled");
      var webhookUrl = document.getElementById("webhookUrl");
      var webhookSecret = document.getElementById("webhookSecret");
      var clearWebhookSecret = document.getElementById("clearWebhookSecret");
      var webhookTimeoutSeconds = document.getElementById("webhookTimeoutSeconds");
      var webhookRetryAttempts = document.getElementById("webhookRetryAttempts");
      var webhookRetryDelayMs = document.getElementById("webhookRetryDelayMs");
      var webhookTitleTemplate = document.getElementById("webhookTitleTemplate");
      var webhookBodyTemplate = document.getElementById("webhookBodyTemplate");
      var notificationMaxAttempts = document.getElementById("notificationMaxAttempts");
      var notificationRetryDelaySeconds = document.getElementById("notificationRetryDelaySeconds");
      var minimumSeverity = document.getElementById("minimumSeverity");
      var alertTypeQuota = document.getElementById("alertTypeQuota");
      var alertTypeRenewal = document.getElementById("alertTypeRenewal");
      var alertTypePaymentFailed = document.getElementById("alertTypePaymentFailed");
      var alertTypeTaskSla = document.getElementById("alertTypeTaskSla");
      var alertTypeOperationQueue = document.getElementById("alertTypeOperationQueue");
      var alertTypeOther = document.getElementById("alertTypeOther");
      var quietHoursEnabled = document.getElementById("quietHoursEnabled");
      var quietHoursStart = document.getElementById("quietHoursStart");
      var quietHoursEnd = document.getElementById("quietHoursEnd");
      var settingTimezone = document.getElementById("settingTimezone");
      var hourlyLimit = document.getElementById("hourlyLimit");
      var rows = document.getElementById("rows");
      var empty = document.getElementById("empty");
      var message = document.getElementById("message");
      var count = document.getElementById("count");

      function setMessage(text) {
        message.textContent = text;
      }

      function tokenFromStorage() {
        var saved = localStorage.getItem(storageKey);
        if (saved) return saved;
        var names = ["Authorization", "authorization", "dashboard_token", "mochat_token", "token", "access_token"];
        for (var i = 0; i < names.length; i++) {
          var value = localStorage.getItem(names[i]);
          if (value) return value;
        }
        for (var j = 0; j < localStorage.length; j++) {
          var key = localStorage.key(j) || "";
          var candidate = localStorage.getItem(key) || "";
          var lower = key.toLowerCase();
          if ((lower.indexOf("token") >= 0 || lower.indexOf("authorization") >= 0) && candidate.length > 20) {
            return candidate;
          }
        }
        return "";
      }

      function authHeaders() {
        var token = tokenInput.value.trim();
        if (!token) return {};
        if (token.toLowerCase().indexOf("bearer ") !== 0) token = "Bearer " + token;
        return { "Authorization": token };
      }

      function apiError(payload, fallback) {
        if (payload && payload.message) return payload.message;
        return fallback;
      }

      function formatTime(value) {
        if (!value) return "-";
        return String(value).replace("T", " ").replace("+08:00", "");
      }

      function cell(text) {
        var td = document.createElement("td");
        td.textContent = text == null || text === "" ? "-" : String(text);
        return td;
      }

      function render(items) {
        rows.textContent = "";
        count.textContent = items.length + " 条";
        empty.style.display = items.length ? "none" : "block";
        items.forEach(function (item) {
          var tr = document.createElement("tr");
          var status = document.createElement("td");
          var pill = document.createElement("span");
          pill.className = "pill " + (item.status || "");
          pill.textContent = item.status || "-";
          status.appendChild(pill);
          tr.appendChild(status);
          tr.appendChild(cell(item.metric));
          tr.appendChild(cell(String(item.currentValue) + " / " + String(item.limitValue)));
          tr.appendChild(cell(item.occurrenceCount));
          tr.appendChild(cell(item.source));
          tr.appendChild(cell(item.message));
          tr.appendChild(cell(formatTime(item.lastSeenAt || item.updatedAt)));
          var action = document.createElement("td");
          var button = document.createElement("button");
          button.type = "button";
          button.className = "danger";
          button.textContent = "解决";
          button.disabled = item.status !== "open";
          button.addEventListener("click", function () {
            resolveAlert(item.metric, item.alertType || "quota_exceeded");
          });
          action.appendChild(button);
          tr.appendChild(action);
          rows.appendChild(tr);
        });
      }

      function fillSetting(setting) {
        setting = setting || {};
        settingEnabled.checked = !!setting.enabled;
        webhookUrl.value = setting.webhookUrl || "";
        webhookSecret.value = "";
        webhookSecret.placeholder = setting.webhookSecretConfigured ? "已配置，留空保持不变" : "留空不配置";
        clearWebhookSecret.checked = false;
        webhookTimeoutSeconds.value = setting.webhookTimeoutSeconds || 5;
        webhookRetryAttempts.value = setting.webhookRetryAttempts || 1;
        webhookRetryDelayMs.value = setting.webhookRetryDelayMs == null ? 250 : setting.webhookRetryDelayMs;
        webhookTitleTemplate.value = setting.webhookTitleTemplate || "SaaS额度告警：租户 {{.TenantID}} {{.Metric}}";
        webhookBodyTemplate.value = setting.webhookBodyTemplate || "{{.Message}}（当前 {{.CurrentValue}} / 上限 {{.LimitValue}}，来源 {{.Source}}）";
        notificationMaxAttempts.value = setting.notificationMaxAttempts || 3;
        notificationRetryDelaySeconds.value = setting.notificationRetryDelaySeconds == null ? 300 : setting.notificationRetryDelaySeconds;
        minimumSeverity.value = setting.minimumSeverity || "warning";
        var alertTypes = Array.isArray(setting.alertTypes) ? setting.alertTypes : [];
        var allTypes = alertTypes.length === 0;
        alertTypeQuota.checked = allTypes || alertTypes.indexOf("quota_exceeded") >= 0;
        alertTypeRenewal.checked = allTypes || alertTypes.indexOf("tenant_renewal_reminder") >= 0;
        alertTypePaymentFailed.checked = allTypes || alertTypes.indexOf("payment_failed_reminder") >= 0;
        alertTypeTaskSla.checked = allTypes || alertTypes.indexOf("admin_task_sla_reminder") >= 0;
        alertTypeOperationQueue.checked = allTypes || alertTypes.indexOf("operation_queue_assignment_reminder") >= 0;
        alertTypeOther.value = alertTypes.filter(function (value) {
          return ["quota_exceeded", "tenant_renewal_reminder", "payment_failed_reminder", "admin_task_sla_reminder", "operation_queue_assignment_reminder"].indexOf(value) < 0;
        }).join(", ");
        quietHoursEnabled.checked = !!setting.quietHoursEnabled;
        quietHoursStart.value = setting.quietHoursStart || "22:00";
        quietHoursEnd.value = setting.quietHoursEnd || "08:00";
        settingTimezone.value = setting.timezone || "Asia/Shanghai";
        hourlyLimit.value = setting.hourlyLimit == null ? 0 : setting.hourlyLimit;
      }

      function selectedAlertTypes() {
        var values = [];
        if (alertTypeQuota.checked) values.push("quota_exceeded");
        if (alertTypeRenewal.checked) values.push("tenant_renewal_reminder");
        if (alertTypePaymentFailed.checked) values.push("payment_failed_reminder");
        if (alertTypeTaskSla.checked) values.push("admin_task_sla_reminder");
        if (alertTypeOperationQueue.checked) values.push("operation_queue_assignment_reminder");
        alertTypeOther.value.split(",").forEach(function (value) {
          value = value.trim();
          if (value && values.indexOf(value) < 0) values.push(value);
        });
        if (values.length === 5 && !alertTypeOther.value.trim()) return [];
        return values;
      }

      function numberValue(input, fallback) {
        var value = Number(input.value);
        return Number.isFinite(value) ? value : fallback;
      }

      function loadSetting() {
        setMessage("正在读取通知配置...");
        fetch("/dashboard/saasAlert/setting", { headers: authHeaders() })
          .then(function (res) { return res.json().then(function (payload) { return { ok: res.ok, payload: payload }; }); })
          .then(function (result) {
            if (!result.ok || result.payload.code !== 200) throw new Error(apiError(result.payload, "读取配置失败"));
            fillSetting((result.payload.data || {}).setting || {});
            setMessage("通知配置已读取。");
          })
          .catch(function (err) {
            setMessage(err.message || "读取配置失败");
          });
      }

      function saveSetting() {
        var body = {
          channel: "webhook",
          enabled: settingEnabled.checked,
          webhookUrl: webhookUrl.value.trim(),
          clearWebhookSecret: clearWebhookSecret.checked,
          webhookTimeoutSeconds: numberValue(webhookTimeoutSeconds, 5),
          webhookRetryAttempts: numberValue(webhookRetryAttempts, 1),
          webhookRetryDelayMs: numberValue(webhookRetryDelayMs, 250),
          webhookTitleTemplate: webhookTitleTemplate.value.trim(),
          webhookBodyTemplate: webhookBodyTemplate.value.trim(),
          notificationMaxAttempts: numberValue(notificationMaxAttempts, 3),
          notificationRetryDelaySeconds: numberValue(notificationRetryDelaySeconds, 300),
          minimumSeverity: minimumSeverity.value,
          alertTypes: selectedAlertTypes(),
          quietHoursEnabled: quietHoursEnabled.checked,
          quietHoursStart: quietHoursStart.value,
          quietHoursEnd: quietHoursEnd.value,
          timezone: settingTimezone.value.trim(),
          hourlyLimit: numberValue(hourlyLimit, 0)
        };
        if (webhookSecret.value.trim()) body.webhookSecret = webhookSecret.value.trim();
        setMessage("正在保存通知配置...");
        fetch("/dashboard/saasAlert/setting", {
          method: "PUT",
          headers: Object.assign({ "Content-Type": "application/json" }, authHeaders()),
          body: JSON.stringify(body)
        })
          .then(function (res) { return res.json().then(function (payload) { return { ok: res.ok, payload: payload }; }); })
          .then(function (result) {
            if (!result.ok || result.payload.code !== 200) throw new Error(apiError(result.payload, "保存配置失败"));
            fillSetting((result.payload.data || {}).setting || {});
            setMessage("通知配置已保存。");
          })
          .catch(function (err) {
            setMessage(err.message || "保存配置失败");
          });
      }

      function loadAlerts() {
        var params = new URLSearchParams();
        var metric = metricInput.value.trim();
        var status = statusFilter.value;
        var alertType = alertTypeInput.value.trim();
        if (metric) params.set("metric", metric);
        if (status) params.set("status", status);
        if (alertType) params.set("alertType", alertType);
        setMessage("正在读取告警...");
        fetch("/dashboard/saasAlert/index?" + params.toString(), { headers: authHeaders() })
          .then(function (res) { return res.json().then(function (payload) { return { ok: res.ok, payload: payload }; }); })
          .then(function (result) {
            if (!result.ok || result.payload.code !== 200) throw new Error(apiError(result.payload, "读取失败"));
            var data = result.payload.data || {};
            var items = data.list || [];
            render(items);
            setMessage("已读取当前筛选条件下的 SaaS 告警。");
          })
          .catch(function (err) {
            render([]);
            setMessage(err.message || "读取失败");
          });
      }

      function resolveAlert(metric, alertType) {
        if (!metric) return;
        setMessage("正在解决告警...");
        fetch("/dashboard/saasAlert/resolve", {
          method: "PUT",
          headers: Object.assign({ "Content-Type": "application/json" }, authHeaders()),
          body: JSON.stringify({ metric: metric, alertType: alertType || "quota_exceeded" })
        })
          .then(function (res) { return res.json().then(function (payload) { return { ok: res.ok, payload: payload }; }); })
          .then(function (result) {
            if (!result.ok || result.payload.code !== 200) throw new Error(apiError(result.payload, "解决失败"));
            setMessage("告警已解决，正在刷新列表。");
            loadAlerts();
          })
          .catch(function (err) {
            setMessage(err.message || "解决失败");
          });
      }

      document.getElementById("saveToken").addEventListener("click", function () {
        localStorage.setItem(storageKey, tokenInput.value.trim());
        setMessage("Token 已保存到当前浏览器。");
      });
      document.getElementById("loadSetting").addEventListener("click", loadSetting);
      document.getElementById("saveSetting").addEventListener("click", saveSetting);
      document.getElementById("apply").addEventListener("click", loadAlerts);
      document.getElementById("reload").addEventListener("click", loadAlerts);
      tokenInput.value = tokenFromStorage();
      loadSetting();
      loadAlerts();
    })();
  </script>
</body>
</html>
`
