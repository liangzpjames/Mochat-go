package dashboard

import "net/http"

func NewSaaSAdminPageHandler() http.Handler {
	return http.HandlerFunc(ServeSaaSAdminPage)
}

func ServeSaaSAdminPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/saas-admin/", http.StatusTemporaryRedirect)
}

// ServeSaaSAdminLegacyPage keeps the previous self-contained page testable while
// production traffic uses the smaller React MVP mounted at /saas-admin/.
func ServeSaaSAdminLegacyPage(w http.ResponseWriter, r *http.Request) {
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
	_, _ = w.Write([]byte(saasAdminPageHTML))
}

const saasAdminPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MoChat Go SaaS 总后台</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
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
      background: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    main {
      width: min(1220px, calc(100% - 32px));
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
    .bar {
      display: grid;
      grid-template-columns: minmax(240px, 1fr) 135px 120px 110px 120px 110px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
	    .filterbar {
	      display: grid;
	      grid-template-columns: minmax(220px, 1fr) minmax(180px, .8fr) 130px 150px 110px;
	      gap: 10px;
	      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .accessbar {
	      display: grid;
	      grid-template-columns: 120px minmax(150px, .7fr) minmax(220px, 1fr) 120px 110px 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .access-permissions {
	      display: flex;
	      flex-wrap: wrap;
	      gap: 6px;
	      min-height: 28px;
	      margin: 8px 0 14px;
	    }
	    .access-permission {
	      display: inline-flex;
	      align-items: center;
	      min-height: 28px;
	      padding: 4px 8px;
	      border: 1px solid var(--line);
	      border-radius: 6px;
	      background: var(--panel);
	      color: var(--muted);
	      font-size: 12px;
	    }
	    .access-permission.write { border-color: #f6b27a; color: #9a3412; }
	    .access-checks {
	      display: grid;
	      grid-template-columns: repeat(3, minmax(0, 1fr));
	      gap: 8px;
	      padding: 12px 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .access-checks label {
	      display: flex;
	      align-items: flex-start;
	      gap: 8px;
	      min-width: 0;
	      color: var(--text);
	      font-size: 13px;
	      line-height: 1.4;
	    }
	    .access-checks input { width: auto; margin-top: 2px; }
	    .access-role-select { min-width: 220px; min-height: 82px; }
	    .approvalgovernance {
	      display: grid;
	      grid-template-columns: repeat(4, minmax(0, 1fr));
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin: 12px 0;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .approvalgovernance .wide { grid-column: span 2; }
	    .approvalgovernance .checkline { min-height: 38px; display: flex; align-items: center; gap: 8px; }
	    .approvalgovernance input[type="checkbox"] { width: 16px; height: 16px; padding: 0; }
	    .systemhealthbar {
	      display: grid;
	      grid-template-columns: 140px 150px 130px 130px minmax(160px, .8fr) minmax(180px, 1fr) 110px 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .auditintegritybar {
	      display: grid;
	      grid-template-columns: minmax(160px, .8fr) 130px 150px 110px 120px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .audit-hash {
	      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	      font-size: 12px;
	    }
	    .identitybar {
	      display: grid;
	      grid-template-columns: 120px 120px 140px 130px minmax(210px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .brandingbar {
	      display: grid;
	      grid-template-columns: 130px 160px minmax(220px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .brandingworkspace {
	      display: grid;
	      grid-template-columns: minmax(360px, .8fr) minmax(0, 1.2fr);
	      gap: 14px;
	      align-items: start;
	      margin-bottom: 14px;
	    }
	    .brandingtable { min-width: 620px; }
	    .brandingeditor {
	      display: grid;
	      grid-template-columns: repeat(2, minmax(0, 1fr));
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .brandingeditor[hidden] { display: none; }
	    .brandingeditor .wide { grid-column: 1 / -1; }
	    .brandingswatch input[type="color"] { width: 100%; height: 38px; padding: 3px; cursor: pointer; }
	    .brandpreview {
	      min-height: 190px;
	      overflow: hidden;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: #eaf0f3 center/cover no-repeat;
	    }
	    .brandpreview-overlay {
	      min-height: 190px;
	      display: grid;
	      grid-template-columns: minmax(0, 1fr) 44%;
	      background: rgba(7, 25, 38, .48);
	    }
	    .brandpreview-lockup { display: flex; gap: 12px; align-items: flex-start; padding: 24px; color: #fff; }
	    .brandpreview-lockup img { width: 42px; height: 42px; object-fit: contain; }
	    .brandpreview-lockup strong { display: block; font-size: 20px; line-height: 1.2; }
	    .brandpreview-lockup span { display: block; margin-top: 5px; font-size: 12px; opacity: .82; }
	    .brandpreview-login { display: grid; place-items: center; padding: 20px; background: rgba(255, 255, 255, .96); }
	    .brandpreview-button { width: min(180px, 100%); height: 38px; border-radius: 6px; color: #fff; font-size: 13px; font-weight: 600; display: grid; place-items: center; }
	    .branding-license { margin: 0; color: var(--muted); font-size: 12px; line-height: 1.5; }
	    .domainbar {
	      display: grid;
	      grid-template-columns: 130px 150px minmax(220px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 10px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .domaincreate {
	      display: grid;
	      grid-template-columns: 130px minmax(260px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: #f8fafc;
	    }
	    .domain-table { min-width: 1040px; }
	    .domain-deliverybar {
	      display: grid;
	      grid-template-columns: 180px 110px;
	      gap: 10px;
	      align-items: end;
	      margin: 14px 0 10px;
	    }
	    .domain-delivery-table { min-width: 1180px; }
	    .domain-delivery-detail { display: grid; gap: 4px; min-width: 190px; }
	    .domain-delivery-detail .domain-error { margin-top: 0; }
	    .dns-record { display: grid; gap: 6px; min-width: 260px; }
	    .dns-record-row { display: grid; grid-template-columns: 42px minmax(0, 1fr) auto; gap: 6px; align-items: center; }
	    .dns-record-row code { overflow-wrap: anywhere; color: var(--text); font-size: 11px; line-height: 1.45; }
	    .dns-record-row button { width: auto; min-height: 28px; height: 28px; padding: 0 7px; font-size: 11px; }
	    .domain-actions { display: flex; flex-wrap: wrap; gap: 6px; min-width: 170px; }
	    .domain-actions button { width: auto; min-height: 30px; height: 30px; padding: 0 8px; font-size: 11px; }
	    .domain-actions .secondary { color: var(--primary); background: #fff; border-color: var(--primary); }
	    .domain-actions .danger { color: var(--danger); background: #fff; border-color: var(--danger); }
	    .domain-error { display: block; max-width: 240px; margin-top: 5px; color: var(--danger); font-size: 11px; line-height: 1.45; }
	    .releasebar {
	      display: grid;
	      grid-template-columns: 180px minmax(320px, 1fr) 110px 140px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .release-evidence-table { min-width: 1180px; }
	    .release-action-table { min-width: 1120px; }
	    .release-candidate-table { min-width: 940px; }
	    .release-fingerprint { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; }
	    .releaseeditor { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-top: 14px; }
	    .releaseeditor .wide { grid-column: 1 / -1; }
	    .releaseeditor textarea { min-height: 88px; }
	    .identitypolicy {
	      display: grid;
	      grid-template-columns: repeat(5, minmax(0, 1fr));
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin: 12px 0 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .identitypolicy .wide { grid-column: span 3; }
	    .identity-actions { display: flex; flex-wrap: wrap; gap: 6px; }
	    .identity-actions button { width: auto; min-height: 32px; height: 32px; padding: 0 9px; font-size: 12px; }
	    .identity-actions .secondary { color: var(--primary); background: #fff; border-color: var(--primary); }
	    .identity-actions .danger { color: var(--danger); background: #fff; border-color: var(--danger); }
	    .identity-user-table, .identity-session-table, .identity-incident-table, .identity-event-table { min-width: 1080px; }
	    .serviceaccountbar {
	      display: grid;
	      grid-template-columns: 130px 150px minmax(220px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .wecomcredentialbar {
	      display: grid;
	      grid-template-columns: 150px 150px 110px 110px;
	      gap: 10px;
	      align-items: end;
	      justify-content: start;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .backupbar {
	      display: grid;
	      grid-template-columns: minmax(260px, 1fr) 130px 130px 130px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .backuppolicy {
	      display: grid;
	      grid-template-columns: repeat(4, minmax(0, 1fr));
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin: 12px 0 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .backuppolicy .checkline { min-height: 38px; display: flex; align-items: center; gap: 8px; }
	    .backuppolicy input[type="checkbox"] { width: 16px; height: 16px; padding: 0; }
	    .backup-actions { display: flex; flex-wrap: wrap; gap: 6px; }
	    .backup-actions button { width: auto; min-height: 32px; height: 32px; padding: 0 9px; font-size: 12px; }
	    .backup-actions .secondary { color: var(--primary); background: #fff; border-color: var(--primary); }
	    .backup-run-table { min-width: 1120px; }
	    .backup-cleanup-table { min-width: 1040px; }
	    .backup-drill-table { min-width: 900px; }
	    .compliancebar {
	      display: grid;
	      grid-template-columns: 130px minmax(220px, 1fr) minmax(220px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .compliancepolicy {
	      display: grid;
	      grid-template-columns: repeat(4, minmax(0, 1fr));
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin: 12px 0 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .compliancepolicy .wide { grid-column: span 2; }
	    .compliancepolicy .checkline { min-height: 38px; display: flex; align-items: center; gap: 8px; }
	    .compliancepolicy input[type="checkbox"] { width: 16px; height: 16px; padding: 0; }
	    .compliance-actions { display: flex; flex-wrap: wrap; gap: 6px; }
	    .compliance-actions button { width: auto; min-height: 32px; height: 32px; padding: 0 9px; font-size: 12px; }
	    .compliance-actions .secondary { color: var(--primary); background: #fff; border-color: var(--primary); }
	    .compliance-actions .danger { color: var(--danger); background: #fff; border-color: var(--danger); }
	    .compliance-table { min-width: 1080px; }
	    .compliance-progress { width: 120px; height: 8px; margin: 6px 0 3px; overflow: hidden; border-radius: 4px; background: #e8edf3; }
	    .compliance-progress > span { display: block; height: 100%; background: var(--primary); }
	    .compliance-step-dialog { width: min(980px, calc(100vw - 28px)); max-height: min(760px, calc(100vh - 28px)); overflow: auto; }
	    .compliance-step-table { min-width: 760px; }
	    .serviceaccountedit {
	      display: grid;
	      grid-template-columns: repeat(4, minmax(0, 1fr));
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin: 12px 0 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .serviceaccountedit .wide { grid-column: span 2; }
	    .serviceaccountedit .full { grid-column: 1 / -1; }
	    .service-account-scopes {
	      display: flex;
	      flex-wrap: wrap;
	      gap: 8px 18px;
	      min-height: 38px;
	      align-items: center;
	    }
	    .service-account-scopes label {
	      display: flex;
	      grid-template-columns: none;
	      align-items: center;
	      gap: 7px;
	      color: var(--text);
	      font-weight: 500;
	    }
	    .service-account-scopes input { width: 16px; height: 16px; padding: 0; }
	    .service-account-secret {
	      display: grid;
	      grid-template-columns: minmax(0, 1fr) 110px 90px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin: 12px 0 14px;
	      border: 1px solid #f6b27a;
	      border-radius: 8px;
	      background: #fffaeb;
	    }
	    .service-account-secret[hidden] { display: none; }
	    .service-account-secret .full { grid-column: 1 / -1; }
	    .service-account-key-list { display: grid; gap: 8px; min-width: 290px; }
	    .service-account-key-row { padding-bottom: 8px; border-bottom: 1px solid var(--line); }
	    .service-account-key-row:last-child { padding-bottom: 0; border-bottom: 0; }
	    .service-account-usage { min-width: 240px; line-height: 1.55; }
	    .service-account-route-list { display: grid; gap: 3px; margin-top: 6px; }
	    .service-account-route-list span { overflow-wrap: anywhere; }
	    .service-account-history {
	      padding: 14px 0 4px;
	      margin: 14px 0;
	      border-top: 1px solid var(--line);
	      border-bottom: 1px solid var(--line);
	    }
	    .service-account-historybar {
	      display: grid;
	      grid-template-columns: minmax(260px, 1fr) minmax(220px, 320px) minmax(290px, auto);
	      gap: 10px;
	      align-items: end;
	      margin: 10px 0 12px;
	    }
	    .service-account-history-actions { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
	    .service-account-segments {
	      display: grid;
	      grid-template-columns: repeat(3, minmax(0, 1fr));
	      height: 38px;
	      border: 1px solid var(--line);
	      border-radius: 6px;
	      overflow: hidden;
	      background: #fff;
	    }
	    .service-account-segments button {
	      min-height: 36px;
	      height: 36px;
	      padding: 0 10px;
	      border: 0;
	      border-right: 1px solid var(--line);
	      border-radius: 0;
	      color: var(--muted);
	      background: #fff;
	    }
	    .service-account-segments button:last-child { border-right: 0; }
	    .service-account-segments button[aria-pressed="true"] { color: #fff; background: var(--primary); }
	    .service-account-chart-scroll { min-height: 160px; overflow-x: auto; padding: 4px 0 8px; }
	    .service-account-chart {
	      display: flex;
	      align-items: flex-end;
	      gap: 5px;
	      height: 148px;
	      min-width: 680px;
	      padding: 12px 8px 0;
	      border-bottom: 1px solid var(--line);
	      background: linear-gradient(to bottom, transparent 24%, var(--line) 25%, transparent 26%, transparent 49%, var(--line) 50%, transparent 51%, transparent 74%, var(--line) 75%, transparent 76%);
	    }
	    .service-account-chart-day { display: grid; grid-template-rows: 110px 22px; flex: 1 0 8px; min-width: 8px; align-items: end; }
	    .service-account-chart-bars { position: relative; display: flex; align-items: flex-end; height: 110px; }
	    .service-account-chart-request { width: 100%; min-height: 0; background: #287a5d; }
	    .service-account-chart-rejected { position: absolute; right: 0; bottom: 0; width: 42%; min-height: 0; background: var(--danger); }
	    .service-account-chart-date { overflow: hidden; color: var(--muted); font-size: 10px; line-height: 22px; text-align: center; white-space: nowrap; }
	    .service-account-usage-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; margin: 12px 0 14px; }
	    .service-account-usage-grid > section { min-width: 0; }
	    .service-account-usage-grid .tablewrap { max-height: 360px; }
	    .service-account-usage-table { table-layout: fixed; }
	    .service-account-usage-route-table { min-width: 650px; }
	    .service-account-usage-route-table th:nth-child(1) { width: 250px; }
	    .service-account-usage-route-table th:nth-child(2), .service-account-usage-route-table th:nth-child(3), .service-account-usage-route-table th:nth-child(4) { width: 62px; }
	    .service-account-usage-route-table th:nth-child(5) { width: 175px; }
	    .service-account-usage-account-table { min-width: 760px; }
	    .service-account-usage-account-table th:nth-child(1) { width: 180px; }
	    .service-account-usage-account-table th:nth-child(2) { width: 150px; }
	    .service-account-usage-account-table th:nth-child(3), .service-account-usage-account-table th:nth-child(4), .service-account-usage-account-table th:nth-child(5) { width: 60px; }
	    .service-account-usage-account-table th:nth-child(6) { width: 175px; }
	    .service-account-usage-table td { overflow-wrap: normal; word-break: normal; }
	    .service-account-table { min-width: 1390px; }
	    .service-account-table th:nth-child(1) { width: 190px; }
	    .service-account-table th:nth-child(2) { width: 150px; }
	    .service-account-table th:nth-child(3) { width: 180px; }
	    .service-account-table th:nth-child(4) { width: 180px; }
	    .service-account-table th:nth-child(5) { width: 270px; }
	    .service-account-table th:nth-child(6) { width: 300px; }
	    .service-account-table th:nth-child(7) { width: 120px; }
	    .service-account-actions { display: flex; flex-wrap: wrap; gap: 6px; }
	    .service-account-actions button { width: auto; min-height: 32px; height: 32px; padding: 0 9px; font-size: 12px; }
	    .service-account-actions .secondary { color: var(--primary); background: #fff; border-color: var(--primary); }
	    .service-account-actions .danger { color: var(--danger); background: #fff; border-color: var(--danger); }
	    .service-account-dialog {
	      width: min(460px, calc(100vw - 28px));
	      padding: 18px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	      color: var(--text);
	    }
	    .service-account-dialog::backdrop { background: rgba(23, 32, 42, .42); }
	    .service-account-dialog p { margin: 8px 0 16px; color: var(--muted); font-size: 13px; line-height: 1.55; }
	    .service-account-dialog .service-account-actions { justify-content: flex-end; }
	    .incident-actions {
	      display: flex;
	      flex-wrap: wrap;
	      gap: 6px;
	    }
	    .incident-actions button {
	      min-height: 32px;
	      padding: 0 9px;
	      font-size: 12px;
	    }
	    .dailyreportbar {
	      display: grid;
	      grid-template-columns: 170px 120px 110px;
	      gap: 10px;
	      align-items: end;
	      margin: 10px 0 12px;
	    }
	    .customersuccessbar {
	      display: grid;
	      grid-template-columns: 140px minmax(180px, 1fr) 120px 110px;
	      gap: 10px;
	      align-items: end;
	      margin: 10px 0 12px;
	    }
	    .operationqueuebar {
	      display: grid;
	      grid-template-columns: 150px 150px minmax(180px, 1fr) minmax(180px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      margin: 10px 0 12px;
	    }
	    .logfilterbar {
	      display: grid;
	      grid-template-columns: 150px 130px minmax(180px, 1fr) 130px minmax(170px, .8fr) minmax(180px, 1fr) 110px;
	      gap: 10px;
      align-items: end;
      padding: 14px;
      margin: 14px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
	    .notification-slo-bar {
	      grid-template-columns: 150px 130px 170px 130px minmax(180px, 1fr) 130px 150px;
	    }
	    .exportbar {
	      display: grid;
	      grid-template-columns: repeat(4, minmax(0, 1fr));
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .exportbar button {
      background: #344054;
      border-color: #344054;
    }
	    .exportbar button:hover {
	      background: #1d2939;
	    }
	    .taskcenterbar {
	      display: grid;
	      grid-template-columns: 150px 140px 130px minmax(180px, 1fr) 110px 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .provisionbar {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 140px 130px 130px minmax(180px, 1fr) 160px 120px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .packagebar {
      display: grid;
      grid-template-columns: 140px minmax(200px, 1fr) 110px 180px minmax(180px, 1fr) 150px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .packageedit {
      display: grid;
      grid-template-columns: 130px minmax(160px, .8fr) minmax(200px, 1fr) 90px 110px minmax(280px, 1.4fr) 140px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .packageimpact {
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .packagesync {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 140px 120px 150px 130px 130px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .packagesynctask, .provisiontask, .renewaltask {
      display: grid;
      grid-template-columns: 140px 140px 140px 140px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .packagesyncresult {
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .tenantbar {
      display: grid;
      grid-template-columns: 150px 130px minmax(260px, 1fr) 120px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .subscriptionfilter {
      display: grid;
      grid-template-columns: 150px 140px minmax(200px, 1fr) 120px 120px 120px 140px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .subscriptiontransition {
      display: grid;
      grid-template-columns: 110px 120px 90px repeat(3, minmax(140px, 1fr)) minmax(160px, 1fr) 110px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin: 14px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .subscriptiontransition .checkline {
      min-height: 38px;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .subscriptiontransition input[type="checkbox"] {
      width: 16px;
      height: 16px;
      padding: 0;
    }
    .paymentfilter {
      display: grid;
      grid-template-columns: 140px 140px minmax(220px, 1fr) repeat(4, 120px);
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .paymentcreate {
      display: grid;
      grid-template-columns: repeat(4, minmax(150px, 1fr));
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin: 14px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .paymentcreate .wide {
      grid-column: span 2;
    }
    .paymentcreate .full {
      grid-column: 1 / -1;
    }
    .renewalbar {
      display: grid;
      grid-template-columns: 130px minmax(180px, 1fr) 160px 130px minmax(180px, 1fr) 120px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .riskfollowbar {
      display: grid;
      grid-template-columns: 140px 150px 170px minmax(220px, 1fr);
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin-bottom: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .risktaskbar {
      display: grid;
      grid-template-columns: 140px 140px 150px minmax(220px, 1fr) 110px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin: 14px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .notificationpolicybar {
      display: grid;
      grid-template-columns: 170px minmax(220px, 1fr) 130px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin: 14px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .notificationcredentialbar {
      display: grid;
      grid-template-columns: minmax(360px, 1fr) 150px 120px 140px;
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin: 14px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .notificationpolicyedit {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 10px;
      align-items: end;
      padding: 14px;
      margin: 14px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .notificationpolicyedit .wide { grid-column: span 2; }
    .notificationpolicyedit .checkline,
    .notificationpolicyedit .policytypes {
      min-height: 38px;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .notificationpolicyedit .policytypes {
      flex-wrap: wrap;
      gap: 8px 16px;
    }
    .notificationpolicyedit input[type="checkbox"] {
      width: 16px;
      height: 16px;
      padding: 0;
    }
    label {
      display: grid;
      gap: 6px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 650;
    }
    .checkline {
      min-height: 38px;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .checkline input[type="checkbox"] {
      flex: 0 0 16px;
      width: 16px;
      height: 16px;
      padding: 0;
    }
    input, select, textarea, button {
      width: 100%;
      border-radius: 6px;
      border: 1px solid var(--line);
      font: inherit;
      letter-spacing: 0;
    }
    input, select, button {
      height: 38px;
    }
    input, select, textarea {
      padding: 0 10px;
      background: #fff;
      color: var(--text);
    }
    textarea {
      min-height: 92px;
      padding: 8px 10px;
      resize: vertical;
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
	    button:disabled {
	      opacity: .45;
	      cursor: not-allowed;
	    }
	    .workspace-nav {
	      position: sticky;
	      top: 0;
	      z-index: 30;
	      display: grid;
	      grid-template-columns: minmax(0, 1fr) 230px;
	      gap: 12px;
	      align-items: center;
	      padding: 10px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: rgba(255, 255, 255, .96);
	      backdrop-filter: blur(8px);
	    }
	    .workspace-tabs {
	      display: flex;
	      gap: 4px;
	      min-width: 0;
	      overflow-x: auto;
	      scrollbar-width: thin;
	    }
	    .workspace-tab {
	      flex: 0 0 auto;
	      width: auto;
	      min-width: 70px;
	      height: 36px;
	      padding: 0 12px;
	      border-color: transparent;
	      background: transparent;
	      color: var(--muted);
	    }
	    .workspace-tab:hover {
	      border-color: var(--line);
	      background: #f8fafc;
	      color: var(--text);
	    }
	    .workspace-tab[aria-selected="true"] {
	      border-color: #84c7bf;
	      background: #e8f5f3;
	      color: var(--primary-strong);
	    }
	    .workspace-tab[hidden] { display: none; }
	    .workspace-jump {
	      display: grid;
	      grid-template-columns: auto minmax(0, 1fr);
	      gap: 8px;
	      align-items: center;
	      white-space: nowrap;
	    }
	    .workspace-jump select { min-width: 0; }
	    .workspace-section-hidden { display: none !important; }
	    main > section[data-workspace] { scroll-margin-top: 76px; }
	    .tenantreadinessbar {
	      display: grid;
	      grid-template-columns: 150px 130px minmax(220px, 1fr) 110px;
	      gap: 10px;
	      align-items: end;
	      padding: 14px;
	      margin-bottom: 14px;
	      border: 1px solid var(--line);
	      border-radius: 8px;
	      background: var(--panel);
	    }
	    .tenant-readiness-table { min-width: 980px; }
	    .readiness-progress {
	      display: block;
	      width: 100%;
	      height: 8px;
	      margin-top: 7px;
	      accent-color: var(--primary);
	    }
	    .readiness-checks {
	      display: flex;
	      flex-wrap: wrap;
	      gap: 5px;
	      align-items: center;
	    }
	    .summary {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 10px;
      margin-bottom: 14px;
    }
    .tile {
      min-height: 86px;
      padding: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    .tile .label {
      color: var(--muted);
      font-size: 12px;
      font-weight: 650;
    }
    .tile .value {
      margin-top: 8px;
      font-size: 24px;
      font-weight: 750;
    }
    .detail .tile .value {
      font-size: 18px;
      line-height: 1.35;
    }
    .status {
      min-height: 34px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      color: var(--muted);
      font-size: 13px;
      margin-bottom: 10px;
    }
	    .grid {
	      display: grid;
	      grid-template-columns: minmax(0, 1.35fr) minmax(360px, .65fr);
	      gap: 14px;
	    }
	    .dailyreportdetail {
	      display: grid;
	      grid-template-columns: repeat(2, minmax(0, 1fr));
	      gap: 14px;
	      margin-top: 12px;
	    }
	    .dailyreportdetail > section,
	    .grid > * {
	      min-width: 0;
	    }
	    .tablewrap {
	      overflow-x: auto;
	      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
    }
    table {
      width: 100%;
      min-width: 760px;
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
    details {
      display: grid;
      gap: 8px;
    }
    summary {
      cursor: pointer;
      color: var(--primary);
      font-weight: 650;
    }
    details.inline-error {
      display: block;
      margin-top: 6px;
    }
    details.inline-error summary {
      color: var(--danger);
      font-size: 12px;
      font-weight: 700;
    }
    details.inline-error pre {
      max-width: 360px;
      max-height: 160px;
      color: var(--danger);
    }
    pre {
      max-height: 220px;
      margin: 8px 0 0;
      padding: 10px;
      overflow: auto;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #f8fafc;
      color: #1d2939;
      font-size: 12px;
      line-height: 1.45;
      white-space: pre-wrap;
    }
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
    .pill.warning { background: #fff3e8; color: var(--warning); }
    .pill.danger { background: #fef3f2; color: var(--danger); }
    .pill.ok { background: #ecfdf3; color: var(--ok); }
    .ok { color: var(--ok); font-weight: 700; }
    .bad { color: var(--danger); font-weight: 700; }
	    .empty {
      padding: 34px 16px;
      color: var(--muted);
      text-align: center;
    }
	    @media (min-width: 901px) and (max-width: 1250px) {
	      .systemhealthbar { grid-template-columns: repeat(4, minmax(0, 1fr)); }
	    }
	    @media (max-width: 900px) {
		      main { width: min(1220px, calc(100% - 20px)); padding-top: 18px; }
	      header { display: block; }
	      h1 { font-size: 20px; }
	      .bar, .filterbar, .accessbar, .access-checks, .approvalgovernance, .brandingbar, .brandingworkspace, .brandingeditor, .domainbar, .domaincreate, .domain-deliverybar, .releasebar, .releaseeditor, .identitybar, .identitypolicy, .systemhealthbar, .auditintegritybar, .serviceaccountbar, .wecomcredentialbar, .service-account-historybar, .service-account-usage-grid, .backupbar, .backuppolicy, .compliancebar, .compliancepolicy, .serviceaccountedit, .service-account-secret, .dailyreportbar, .customersuccessbar, .operationqueuebar, .dailyreportdetail, .logfilterbar, .exportbar, .taskcenterbar, .tenantreadinessbar, .provisionbar, .provisiontask, .packagebar, .packageedit, .packageimpact, .packagesync, .packagesynctask, .packagesyncresult, .tenantbar, .subscriptionfilter, .subscriptiontransition, .paymentfilter, .paymentcreate, .renewalbar, .renewaltask, .riskfollowbar, .risktaskbar, .notificationpolicybar, .notificationcredentialbar, .notificationpolicyedit, .summary, .grid { grid-template-columns: 1fr; }
	      .brandingeditor .wide { grid-column: auto; }
	      .brandpreview-overlay { grid-template-columns: 1fr; }
	      .brandpreview-login { min-height: 92px; }
	      #tenantDomainCenter > .status { align-items: flex-start; flex-direction: column; gap: 4px; }
	      .service-account-history > .status { align-items: flex-start; flex-direction: column; gap: 4px; }
	      #tenantDomains .empty, #tenantDomainDeliveryJobs .empty { position: sticky; left: 0; text-align: left; }
	      #releaseReadinessCenter > .status { align-items: flex-start; flex-direction: column; gap: 4px; }
	      #releaseEvidenceActions .empty, #releaseEvidence .empty, #releaseCandidates .empty { position: sticky; left: 0; text-align: left; }
	      .releaseeditor .wide { grid-column: auto; }
	      .identitypolicy .wide { grid-column: auto; }
	      .paymentcreate .wide, .paymentcreate .full { grid-column: auto; }
	      .notificationpolicyedit .wide { grid-column: auto; }
	      .serviceaccountedit .wide, .serviceaccountedit .full, .service-account-secret .full { grid-column: auto; }
	      .compliancepolicy .wide { grid-column: auto; }
	      main > section[data-workspace] { scroll-margin-top: 118px; }
	      #tenantReadinessCenter > .status { align-items: flex-start; flex-direction: column; gap: 4px; }
	      button { width: 100%; }
	      .workspace-nav { grid-template-columns: 1fr; padding: 8px; }
	      .workspace-tab { width: auto; min-width: 66px; padding: 0 10px; }
	      .workspace-jump { grid-template-columns: 48px minmax(0, 1fr); }
	    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
	        <h1 id="brandAppTitle">MoChat Go SaaS 总后台</h1>
        <p class="subtitle">租户、套餐、用量、到期和告警的运营总览。</p>
      </div>
      <button id="reload" type="button">刷新</button>
    </header>

    <section class="bar" aria-label="筛选">
      <label>Dashboard JWT
        <input id="token" type="password" autocomplete="off" placeholder="Bearer token 或纯 token">
      </label>
      <label>范围
        <select id="scope">
          <option value="tenant">当前租户</option>
          <option value="platform">全平台</option>
        </select>
      </label>
      <label>租户 ID
        <input id="tenantId" inputmode="numeric" placeholder="可选">
      </label>
      <label>到期天数
        <input id="expiringDays" inputmode="numeric" value="30">
      </label>
      <label>高用量阈值
        <input id="riskHighUsageRatio" inputmode="decimal" value="0.8">
      </label>
      <button id="saveToken" type="button">保存</button>
    </section>

	    <section class="filterbar" aria-label="租户列表筛选">
      <label>租户搜索
        <input id="filterKeyword" placeholder="租户名称、ID 或套餐">
      </label>
      <label>筛选套餐
        <select id="filterPackageCode">
          <option value="">全部套餐</option>
        </select>
      </label>
      <label>租户状态
        <select id="filterTenantStatus">
          <option value="">全部</option>
          <option value="1">正常</option>
          <option value="2">停用</option>
        </select>
      </label>
      <label>到期状态
        <select id="filterDueState">
          <option value="all">全部</option>
          <option value="normal">正常</option>
          <option value="expiring">即将到期</option>
          <option value="expired">已到期</option>
          <option value="no_package">未开通套餐</option>
        </select>
      </label>
		      <button id="applyFilters" type="button">筛选</button>
		    </section>

	    <nav id="workspaceNav" class="workspace-nav" aria-label="总后台工作区">
	      <div class="workspace-tabs" role="tablist" aria-label="工作区切换">
	        <button class="workspace-tab" type="button" role="tab" data-workspace="overview" aria-selected="true">总览</button>
	        <button class="workspace-tab" type="button" role="tab" data-workspace="tenant" aria-selected="false">租户套餐</button>
	        <button class="workspace-tab" type="button" role="tab" data-workspace="operations" aria-selected="false">运营</button>
	        <button class="workspace-tab" type="button" role="tab" data-workspace="finance" aria-selected="false">财务</button>
	        <button class="workspace-tab" type="button" role="tab" data-workspace="notifications" aria-selected="false">通知</button>
	        <button class="workspace-tab" type="button" role="tab" data-workspace="governance" aria-selected="false">治理</button>
	        <button class="workspace-tab" type="button" role="tab" data-workspace="security" aria-selected="false">安全</button>
	        <button class="workspace-tab" type="button" role="tab" data-workspace="delivery" aria-selected="false">交付</button>
	      </div>
	      <label class="workspace-jump">当前模块
	        <select id="workspaceSectionJump" aria-label="当前工作区模块"></select>
	      </label>
	    </nav>

		    <section aria-label="当前平台权限">
	      <div class="status"><strong>当前平台权限</strong><span id="accessProfileState">等待登录</span></div>
	      <div id="accessPermissionSummary" class="access-permissions"></div>
	    </section>

	    <section id="accessGovernance" aria-label="平台权限治理" hidden>
	      <div class="status"><strong>平台权限治理</strong><span id="accessGovernanceState">等待加载</span></div>
	      <section class="accessbar" aria-label="平台岗位维护">
	        <label>岗位 ID
	          <input id="accessRoleId" inputmode="numeric" value="0" readonly>
	        </label>
	        <label>岗位编码
	          <input id="accessRoleCode" placeholder="custom_finance">
	        </label>
	        <label>岗位名称
	          <input id="accessRoleName" placeholder="自定义岗位">
	        </label>
	        <label>状态
	          <select id="accessRoleStatus"><option value="1">启用</option><option value="2">停用</option></select>
	        </label>
	        <button id="newAccessRole" type="button">新建</button>
	        <button id="saveAccessRole" type="button">保存</button>
	        <label style="grid-column:1/-1">岗位说明
	          <input id="accessRoleDescription" placeholder="岗位职责和授权边界">
	        </label>
	        <input id="accessRoleVersion" type="hidden" value="0">
	      </section>
	      <div id="accessPermissionOptions" class="access-checks" aria-label="岗位权限"></div>
	      <div class="tablewrap"><table aria-label="平台岗位">
	        <thead><tr><th>岗位</th><th>状态</th><th>权限</th><th>人员</th><th>版本/操作</th></tr></thead>
	        <tbody id="accessRoles"><tr><td colspan="5" class="empty">暂无平台岗位</td></tr></tbody>
	      </table></div>
	      <section class="accessbar" aria-label="平台人员授权筛选">
	        <label style="grid-column:1/4">人员搜索
	          <input id="accessAssignmentKeyword" placeholder="姓名、手机号或用户 ID">
	        </label>
	        <button id="loadAccessAssignments" type="button">刷新人员</button>
	      </section>
	      <div class="tablewrap"><table aria-label="平台人员授权">
	        <thead><tr><th>人员</th><th>账号状态</th><th>岗位</th><th>有效权限</th><th>版本/操作</th></tr></thead>
	        <tbody id="accessAssignments"><tr><td colspan="5" class="empty">暂无平台人员</td></tr></tbody>
		      </table></div>
			    </section>

			    <section id="brandingCenter" aria-label="品牌与白标中心" hidden>
			      <div class="status"><strong>品牌与白标</strong><span id="brandingState">等待加载</span></div>
			      <section class="brandingbar" aria-label="品牌档案筛选">
			        <label>租户 ID<input id="brandingTenantFilter" inputmode="numeric" placeholder="全部租户"></label>
			        <label>配置状态<select id="brandingStatusFilter"><option value="all">全部</option><option value="configured">已配置</option><option value="unconfigured">未配置</option><option value="active">启用</option><option value="disabled">停用</option></select></label>
			        <label>搜索<input id="brandingKeyword" placeholder="租户、产品名称或 ID"></label>
			        <button id="loadBrandingProfiles" type="button">刷新</button>
			      </section>
			      <div class="brandingworkspace">
			        <div class="tablewrap"><table class="brandingtable" aria-label="租户品牌档案">
			          <thead><tr><th>租户</th><th>产品</th><th>状态</th><th>颜色</th><th>版本/操作</th></tr></thead>
			          <tbody id="brandingProfiles"><tr><td colspan="5" class="empty">暂无品牌档案</td></tr></tbody>
			        </table></div>
			        <section id="brandingEditor" class="brandingeditor" aria-label="品牌档案编辑" hidden>
			          <input id="brandingVersion" type="hidden" value="0"><input id="brandingConfigured" type="hidden" value="0">
			          <label>租户 ID<input id="brandingTenantId" inputmode="numeric" readonly></label>
			          <label>状态<select id="brandingStatus"><option value="active">启用</option><option value="disabled">停用</option></select></label>
			          <label>产品名称<input id="brandingProductName" maxlength="80"></label>
			          <label>产品简称<input id="brandingProductShortName" maxlength="32"></label>
			          <label class="wide">产品副标题<input id="brandingProductSubtitle" maxlength="160"></label>
			          <label>Logo 路径<input id="brandingLogoUrl" placeholder="/img/logo.png"></label>
			          <label>Favicon 路径<input id="brandingFaviconUrl" placeholder="/favicon.ico"></label>
			          <label class="wide">登录背景路径<input id="brandingLoginBackgroundUrl" placeholder="/static/login.png"></label>
			          <label class="brandingswatch">主色<input id="brandingPrimaryColor" type="color" value="#1769AA"></label>
			          <label class="brandingswatch">强调色<input id="brandingAccentColor" type="color" value="#0F578F"></label>
			          <label>官网链接<input id="brandingWebsiteUrl" placeholder="https://example.com"></label>
			          <label>支持入口<input id="brandingSupportUrl" placeholder="/support"></label>
			          <label>支持二维码路径<input id="brandingSupportQrUrl" placeholder="/img/support.png"></label>
			          <label>支持邮箱<input id="brandingSupportEmail" type="email" maxlength="254"></label>
			          <label class="wide">文档入口<input id="brandingDocsUrl" placeholder="/docs"></label>
			          <label class="wide">页脚版权<input id="brandingFooterText" maxlength="160"></label>
			          <div class="wide brandpreview" id="brandingPreview" aria-label="登录页品牌预览">
			            <div class="brandpreview-overlay"><div class="brandpreview-lockup"><img id="brandingPreviewLogo" alt=""><div><strong id="brandingPreviewName">MoChat Go</strong><span id="brandingPreviewSubtitle">企业微信客户运营平台</span></div></div><div class="brandpreview-login"><span id="brandingPreviewButton" class="brandpreview-button">登录</span></div></div>
			          </div>
			          <p id="brandingLicense" class="wide branding-license">GPL-3.0</p>
			          <button id="saveBrandingProfile" class="wide" type="button">保存品牌档案</button>
			        </section>
			      </div>
			    </section>

			    <section id="tenantDomainCenter" aria-label="租户自定义域名中心" hidden>
			      <div class="status"><strong>租户自定义域名</strong><span id="tenantDomainState">等待加载</span></div>
			      <section class="domainbar" aria-label="租户域名筛选">
			        <label>租户 ID<input id="tenantDomainTenantFilter" inputmode="numeric" placeholder="全部租户"></label>
			        <label>域名状态<select id="tenantDomainStatusFilter"><option value="all">全部</option><option value="pending">待验证</option><option value="active">已启用</option><option value="disabled">已停用</option></select></label>
			        <label>搜索<input id="tenantDomainKeyword" placeholder="域名、租户名称或 ID"></label>
			        <button id="loadTenantDomains" type="button">刷新</button>
			      </section>
			      <section class="domaincreate" aria-label="添加租户域名">
			        <label>租户 ID<input id="tenantDomainCreateTenantId" inputmode="numeric" placeholder="必填"></label>
			        <label>登录域名<input id="tenantDomainCreateHostname" autocomplete="off" placeholder="login.customer.example.com"></label>
			        <button id="createTenantDomain" type="button">添加域名</button>
			      </section>
			      <div class="tablewrap"><table class="domain-table" aria-label="租户自定义域名列表">
			        <thead><tr><th>租户</th><th>域名</th><th>域名状态</th><th>路由 / TLS 交付</th><th>DNS TXT 校验</th><th>验证时间</th><th>操作</th></tr></thead>
			        <tbody id="tenantDomains"><tr><td colspan="7" class="empty">暂无租户域名</td></tr></tbody>
			      </table></div>
			      <div class="status"><strong>域名交付任务</strong><span id="tenantDomainDeliveryState">等待加载</span></div>
			      <section class="domain-deliverybar" aria-label="域名交付任务筛选">
			        <label>任务状态<select id="tenantDomainDeliveryStatus"><option value="all">全部</option><option value="pending">待处理</option><option value="processing">交付中</option><option value="waiting">等待回调</option><option value="failed">失败</option><option value="succeeded">已完成</option><option value="canceled">已取消</option></select></label>
			        <button id="loadTenantDomainDeliveryJobs" type="button">刷新</button>
			      </section>
			      <div class="tablewrap"><table class="domain-delivery-table" aria-label="域名交付任务列表">
			        <thead><tr><th>任务</th><th>租户 / 域名</th><th>动作</th><th>执行状态</th><th>Provider</th><th>时间</th><th>结果</th></tr></thead>
			        <tbody id="tenantDomainDeliveryJobs"><tr><td colspan="7" class="empty">暂无交付任务</td></tr></tbody>
			      </table></div>
			      <dialog id="tenantDomainActionDialog" class="service-account-dialog" aria-labelledby="tenantDomainActionTitle">
			        <strong id="tenantDomainActionTitle">确认域名操作</strong>
			        <p id="tenantDomainActionMessage">请确认当前操作。</p>
			        <div class="service-account-actions"><button id="cancelTenantDomainAction" type="button" class="secondary">取消</button><button id="confirmTenantDomainAction" type="button" class="danger">确认</button></div>
			      </dialog>
			    </section>

			    <section id="releaseReadinessCenter" aria-label="发布准备中心" hidden>
			      <div class="status"><strong>发布准备中心</strong><span id="releaseReadinessState">等待加载</span></div>
			      <section class="releasebar" aria-label="发布门禁">
			        <label>发布版本<input id="releaseVersion" maxlength="64" placeholder="v1.0.0"></label>
			        <label>源码 SHA-256 指纹<input id="releaseSourceFingerprint" class="release-fingerprint" maxlength="64" autocomplete="off" placeholder="64 位源码指纹"></label>
			        <button id="loadReleaseReadiness" type="button">刷新</button>
			        <button id="runReleaseGate" type="button">运行发布门禁</button>
			      </section>
			      <section id="releaseReadinessSummary" class="summary detail" aria-label="发布准备概览"></section>
			      <div class="status"><strong>生产补证行动清单</strong><span id="releaseEvidenceActionCount"></span></div>
			      <div class="tablewrap"><table class="release-action-table" aria-label="生产补证行动清单">
			        <thead><tr><th>证据项</th><th>证据 / 处置</th><th>负责人</th><th>截止时间</th><th>下一步</th><th>更新</th><th>操作</th></tr></thead>
			        <tbody id="releaseEvidenceActions"><tr><td colspan="7" class="empty">暂无补证行动</td></tr></tbody>
			      </table></div>
			      <div class="status"><strong>生产证据</strong><span id="releaseEvidenceCount"></span></div>
			      <div class="tablewrap"><table class="release-evidence-table" aria-label="生产发布证据">
			        <thead><tr><th>证据项</th><th>状态</th><th>执行环境</th><th>源码指纹</th><th>证据</th><th>复核</th><th>操作</th></tr></thead>
			        <tbody id="releaseEvidence"><tr><td colspan="7" class="empty">暂无发布证据</td></tr></tbody>
			      </table></div>
			      <div class="status" style="margin-top:12px"><strong>候选快照</strong><span id="releaseCandidateCount"></span></div>
			      <div class="tablewrap"><table class="release-candidate-table" aria-label="发布候选快照">
			        <thead><tr><th>候选编号</th><th>版本</th><th>状态</th><th>门禁计数</th><th>源码指纹</th><th>创建</th></tr></thead>
			        <tbody id="releaseCandidates"><tr><td colspan="6" class="empty">暂无发布候选</td></tr></tbody>
			      </table></div>
			      <dialog id="releaseEvidenceDialog" class="service-account-dialog" aria-labelledby="releaseEvidenceTitle">
			        <strong id="releaseEvidenceTitle">维护发布证据</strong>
			        <section class="releaseeditor" aria-label="发布证据编辑">
			          <input id="releaseEvidenceKey" type="hidden"><input id="releaseEvidenceVersion" type="hidden">
			          <label>状态<select id="releaseEvidenceStatus"><option value="missing">缺失</option><option value="in_progress">进行中</option><option value="passed">已通过</option><option value="failed">未通过</option></select></label>
			          <label>执行环境<input id="releaseEvidenceEnvironment" maxlength="80" placeholder="production / amd64"></label>
			          <label class="wide">证据 HTTPS 地址<input id="releaseEvidenceUrl" type="url" maxlength="1000" autocomplete="off" placeholder="https://evidence.company.com/path"></label>
			          <label class="wide">源码 SHA-256 指纹<input id="releaseEvidenceFingerprint" class="release-fingerprint" maxlength="64" autocomplete="off"></label>
			          <label class="wide">证据工件 SHA-256<input id="releaseEvidenceArtifactFingerprint" class="release-fingerprint" maxlength="64" autocomplete="off" placeholder="实际日志、报告或压缩包的 64 位哈希"></label>
			          <label>证据工件大小（字节）<input id="releaseEvidenceArtifactSize" type="number" min="1" step="1" inputmode="numeric" placeholder="大于 0"></label>
			          <label class="wide">复核备注<textarea id="releaseEvidenceNote" maxlength="1000"></textarea></label>
			        </section>
			        <div class="service-account-actions"><button id="cancelReleaseEvidence" type="button" class="secondary">取消</button><button id="saveReleaseEvidence" type="button">保存证据</button></div>
			      </dialog>
			      <dialog id="releaseEvidenceActionDialog" class="service-account-dialog" aria-labelledby="releaseEvidenceActionTitle">
			        <strong id="releaseEvidenceActionTitle">分派生产补证</strong>
			        <section class="releaseeditor" aria-label="生产补证行动编辑">
			          <input id="releaseEvidenceActionKey" type="hidden"><input id="releaseEvidenceActionVersion" type="hidden">
			          <label>负责人<select id="releaseEvidenceActionOwner"><option value="0">未分配</option></select></label>
			          <label>截止时间<input id="releaseEvidenceActionDueAt" type="datetime-local"></label>
			          <label class="wide">下一步<textarea id="releaseEvidenceActionNext" maxlength="500"></textarea></label>
			          <label class="wide">协作备注<textarea id="releaseEvidenceActionNote" maxlength="1000"></textarea></label>
			        </section>
			        <div class="service-account-actions"><button id="cancelReleaseEvidenceAction" type="button" class="secondary">取消</button><button id="saveReleaseEvidenceAction" type="button">保存行动</button></div>
			      </dialog>
			    </section>

			    <section id="identitySecurityCenter" aria-label="身份与访问安全中心" hidden>
		      <div class="status"><strong>身份与访问安全</strong><span id="identitySecurityState">等待加载</span></div>
		      <section class="identitybar" aria-label="身份安全筛选">
		        <label>租户 ID<input id="identityTenantFilter" inputmode="numeric" placeholder="必填"></label>
		        <label>用户 ID<input id="identityUserFilter" inputmode="numeric" placeholder="全部用户"></label>
		        <label>会话状态
		          <select id="identitySessionStatus"><option value="">全部</option><option value="active">活动</option><option value="revoked">已撤销</option><option value="expired">已过期</option></select>
		        </label>
		        <label>事件风险
		          <select id="identityEventRisk"><option value="">全部</option><option value="critical">严重</option><option value="warning">预警</option><option value="normal">普通</option></select>
		        </label>
		        <label>搜索<input id="identityKeyword" placeholder="姓名、手机号、IP、设备或事故"></label>
		        <button id="loadIdentitySecurity" type="button">刷新</button>
		      </section>
		      <section id="identitySecuritySummary" class="summary detail" aria-label="身份安全概览"></section>
		      <section id="identityPolicyEditor" class="identitypolicy" aria-label="租户身份安全策略" hidden>
		        <label>策略状态<select id="identityPolicyStatus"><option value="active">启用</option><option value="disabled">停用</option></select></label>
		        <label>失败锁定阈值<input id="identityMaxFailedAttempts" type="number" min="3" max="20"></label>
		        <label>锁定分钟<input id="identityLockoutMinutes" type="number" min="1" max="1440"></label>
		        <label>会话有效分钟<input id="identitySessionTtlMinutes" type="number" min="15" max="43200"></label>
		        <label>空闲超时分钟<input id="identityIdleTimeoutMinutes" type="number" min="5" max="43200"></label>
		        <label>并发会话上限<input id="identityMaxConcurrentSessions" type="number" min="1" max="50"></label>
		        <label>登录事件保留天数<input id="identityLoginEventRetentionDays" type="number" min="30" max="3650"></label>
		        <label>会话记录保留天数<input id="identitySessionRetentionDays" type="number" min="7" max="3650"></label>
		        <label class="checkline"><input id="identityRequireMfa" type="checkbox">强制 MFA</label>
		        <label class="wide">登录 IP 白名单<textarea id="identityAllowedIpCidrs" placeholder="每行一个 CIDR；留空表示不限制"></textarea></label>
		        <input id="identityPolicyVersion" type="hidden" value="0">
		        <button id="saveIdentityPolicy" type="button">保存策略</button>
		      </section>
		      <div class="status"><strong>租户账号</strong><span id="identityUserCount"></span></div>
		      <div class="tablewrap"><table class="identity-user-table" aria-label="身份安全用户列表">
		        <thead><tr><th>用户</th><th>账号状态</th><th>MFA</th><th>失败/最近登录</th><th>活动会话</th><th>操作</th></tr></thead>
		        <tbody id="identityUsers"><tr><td colspan="6" class="empty">暂无用户</td></tr></tbody>
		      </table></div>
		      <div class="status" style="margin-top:12px"><strong>登录会话</strong><span id="identitySessionCount"></span></div>
		      <div class="tablewrap"><table class="identity-session-table" aria-label="身份安全会话列表">
		        <thead><tr><th>会话</th><th>用户/租户</th><th>状态/方式</th><th>来源</th><th>有效期</th><th>操作</th></tr></thead>
		        <tbody id="identitySessions"><tr><td colspan="6" class="empty">暂无会话</td></tr></tbody>
		      </table></div>
		      <div class="status" style="margin-top:12px"><strong>安全事故</strong><span id="identityIncidentCount"></span></div>
		      <div class="tablewrap"><table class="identity-incident-table" aria-label="身份安全事故列表">
		        <thead><tr><th>事故</th><th>用户/租户</th><th>级别/状态</th><th>发生情况</th><th>负责人/结论</th><th>操作</th></tr></thead>
		        <tbody id="identityIncidents"><tr><td colspan="6" class="empty">暂无安全事故</td></tr></tbody>
		      </table></div>
		      <div class="status" style="margin-top:12px"><strong>登录与安全事件</strong><span id="identityEventCount"></span></div>
		      <div class="tablewrap"><table class="identity-event-table" aria-label="身份安全事件列表">
		        <thead><tr><th>时间/事件</th><th>用户/租户</th><th>结果/风险</th><th>来源</th><th>原因</th><th>上下文</th></tr></thead>
		        <tbody id="identityEvents"><tr><td colspan="6" class="empty">暂无安全事件</td></tr></tbody>
		      </table></div>
		    </section>

		    <section id="systemHealthCenter" aria-label="平台健康中心" hidden>
		      <div class="status"><strong>平台健康中心</strong><span id="systemHealthState">等待加载</span></div>
		      <section class="systemhealthbar" aria-label="平台健康筛选">
		        <label>失败窗口（小时）
		          <input id="systemHealthFailureWindow" inputmode="numeric" value="24">
		        </label>
		        <label>通知滞留（分钟）
		          <input id="systemHealthNotificationStale" inputmode="numeric" value="15">
		        </label>
		        <label>事故状态
		          <select id="systemIncidentStatus"><option value="active">活跃</option><option value="open">待处置</option><option value="acknowledged">已认领</option><option value="resolved">已解决</option><option value="all">全部</option></select>
		        </label>
		        <label>严重程度
		          <select id="systemIncidentSeverity"><option value="all">全部</option><option value="critical">严重</option><option value="warning">预警</option></select>
		        </label>
		        <label>负责人
		          <input id="systemIncidentOwner" placeholder="姓名或班组">
		        </label>
		        <label>事故搜索
		          <input id="systemIncidentKeyword" placeholder="标题、来源或分类">
		        </label>
		        <button id="loadSystemHealth" type="button">刷新</button>
		        <button id="runSystemHealthScan" type="button">立即扫描</button>
		      </section>
		      <section id="systemHealthSummary" class="summary detail" aria-label="平台健康概览"></section>
		      <div class="status"><strong>健康检查</strong><span id="systemHealthCheckCount"></span></div>
		      <div class="tablewrap"><table aria-label="平台健康检查">
		        <thead><tr><th>检查项</th><th>分类</th><th>状态</th><th>当前/阈值</th><th>详情</th></tr></thead>
		        <tbody id="systemHealthChecks"><tr><td colspan="5" class="empty">暂无检查结果</td></tr></tbody>
		      </table></div>
		      <div class="status" style="margin-top:12px"><strong>系统事故</strong><span id="systemIncidentCount"></span></div>
		      <div class="tablewrap"><table aria-label="系统事故">
		        <thead><tr><th>事故</th><th>级别/状态</th><th>当前/阈值</th><th>负责人</th><th>处置备注</th><th>检测时间</th><th>操作</th></tr></thead>
		        <tbody id="systemIncidents"><tr><td colspan="7" class="empty">暂无系统事故</td></tr></tbody>
		      </table></div>
		      <div class="status" style="margin-top:12px"><strong>扫描历史</strong><span id="systemHealthScanCount"></span></div>
		      <div class="tablewrap"><table aria-label="平台健康扫描历史">
		        <thead><tr><th>扫描</th><th>触发</th><th>健康状态</th><th>问题</th><th>事故变化</th><th>通知</th><th>完成时间</th></tr></thead>
		        <tbody id="systemHealthScans"><tr><td colspan="7" class="empty">暂无扫描记录</td></tr></tbody>
		      </table></div>
	    </section>

	    <section id="backupCenter" aria-label="备份与恢复治理" hidden>
	      <div class="status"><strong>备份与恢复治理</strong><span id="backupState">等待加载</span></div>
	      <section class="backupbar" aria-label="备份操作">
	        <div><strong>运行配置</strong><br><span id="backupConfigState" class="muted">等待检测</span></div>
	        <button id="loadBackupOverview" type="button">刷新</button>
	        <button id="createBackup" type="button">立即备份</button>
	        <button id="cleanupBackups" type="button">执行保留清理</button>
	      </section>
	      <section id="backupSummary" class="summary detail" aria-label="备份恢复概览"></section>
	      <section id="backupPolicyEditor" class="backuppolicy" aria-label="备份策略" hidden>
	        <label>策略状态<select id="backupPolicyStatus"><option value="active">启用</option><option value="disabled">停用</option></select></label>
	        <label>备份间隔（分钟）<input id="backupIntervalMinutes" type="number" min="5" max="10080"></label>
	        <label>保留天数<input id="backupRetentionDays" type="number" min="1" max="3650"></label>
	        <label>最少成功备份<input id="backupMinSuccessful" type="number" min="1" max="365"></label>
	        <label>最大备份年龄（分钟）<input id="backupMaxAgeMinutes" type="number" min="5" max="43200"></label>
	        <label>恢复演练间隔（天）<input id="backupDrillIntervalDays" type="number" min="1" max="365"></label>
	        <label class="checkline"><input id="backupRequireEncryption" type="checkbox" checked>强制加密</label>
	        <label class="checkline"><input id="backupRequireOffsiteReplica" type="checkbox">必须异地副本</label>
	        <input id="backupPolicyVersion" type="hidden" value="0">
	        <button id="saveBackupPolicy" type="button">保存策略</button>
	      </section>
	      <div class="status"><strong>备份运行</strong><span id="backupRunCount"></span></div>
	      <div class="tablewrap"><table class="backup-run-table" aria-label="备份运行列表">
	        <thead><tr><th>备份</th><th>触发/状态</th><th>工件</th><th>迁移/表</th><th>完整性</th><th>异地副本</th><th>时间</th><th>操作</th></tr></thead>
	        <tbody id="backupRuns"><tr><td colspan="8" class="empty">暂无备份运行</td></tr></tbody>
	      </table></div>
	      <div class="status" style="margin-top:12px"><strong>保留清理任务</strong><span id="backupCleanupRunCount"></span></div>
	      <div class="tablewrap"><table class="backup-cleanup-table" aria-label="备份保留清理任务列表">
	        <thead><tr><th>任务</th><th>状态</th><th>冻结范围</th><th>执行进度</th><th>审批/尝试</th><th>时间</th><th>失败详情</th><th>操作</th></tr></thead>
	        <tbody id="backupCleanupRuns"><tr><td colspan="8" class="empty">暂无保留清理任务</td></tr></tbody>
	      </table></div>
	      <div class="status" style="margin-top:12px"><strong>恢复演练</strong><span id="restoreDrillCount"></span></div>
	      <div class="tablewrap"><table class="backup-drill-table" aria-label="恢复演练列表">
	        <thead><tr><th>演练</th><th>备份</th><th>状态</th><th>目标</th><th>迁移/表</th><th>耗时</th><th>完成时间</th></tr></thead>
	        <tbody id="restoreDrills"><tr><td colspan="7" class="empty">暂无恢复演练</td></tr></tbody>
	      </table></div>
	      <dialog id="backupRestoreDialog" class="service-account-dialog" aria-labelledby="backupRestoreTitle">
	        <strong id="backupRestoreTitle">执行隔离恢复演练</strong>
	        <p id="backupRestoreMessage">恢复只会写入服务端控制的隔离目标。</p>
	        <div class="service-account-actions"><button id="cancelBackupRestore" type="button" class="secondary">取消</button><button id="confirmBackupRestore" type="button">确认演练</button></div>
	      </dialog>
	    </section>

	    <section id="complianceCenter" aria-label="租户数据合规中心" hidden>
	      <div class="status"><strong>租户数据合规</strong><span id="complianceState">等待加载</span></div>
	      <section class="compliancebar" aria-label="合规中心筛选">
	        <label>租户 ID<input id="complianceTenantFilter" inputmode="numeric" placeholder="全部租户"></label>
	        <div><strong>运行配置</strong><br><span id="complianceConfigState" class="muted">等待检测</span></div>
	        <div><strong>数据清单</strong><br><span id="complianceInventoryState" class="muted">等待检测</span></div>
	        <button id="loadComplianceOverview" type="button">刷新</button>
	      </section>
	      <section id="complianceSummary" class="summary detail" aria-label="合规概览"></section>
	      <section id="compliancePolicyEditor" class="compliancepolicy" aria-label="合规策略" hidden>
	        <label>策略状态<select id="compliancePolicyStatus"><option value="active">启用</option><option value="disabled">停用</option></select></label>
	        <label>导出保留天数<input id="complianceExportRetentionDays" type="number" min="1" max="3650"></label>
	        <label>擦除等待天数<input id="complianceErasureGraceDays" type="number" min="0" max="365"></label>
	        <label>最近导出有效天数<input id="complianceRecentExportDays" type="number" min="1" max="365"></label>
	        <label>财务保留天数<input id="complianceBillingRetentionDays" type="number" min="0" max="3650"></label>
	        <label>审计保留天数<input id="complianceAuditRetentionDays" type="number" min="0" max="3650"></label>
	        <label>服务账号用量保留天数<input id="complianceServiceAccountUsageRetentionDays" type="number" min="1" max="3650"></label>
	        <label class="checkline wide"><input id="complianceRequireRecentExport" type="checkbox">擦除前必须有最近成功导出</label>
	        <input id="compliancePolicyVersion" type="hidden" value="0">
	        <button id="saveCompliancePolicy" type="button">保存策略</button>
	      </section>
	      <section id="complianceActions" class="compliancepolicy" aria-label="合规操作" hidden>
	        <label>租户 ID<input id="complianceActionTenantId" inputmode="numeric" placeholder="业务租户"></label>
	        <label class="wide">原因<input id="complianceActionReason" placeholder="申请或保留依据"></label>
	        <label>保留开始<input id="complianceHoldStartsAt" type="datetime-local"></label>
	        <label>保留到期<input id="complianceHoldExpiresAt" type="datetime-local"></label>
	        <button id="createComplianceHold" type="button">创建法律保留</button>
	        <button id="requestComplianceExport" type="button">申请加密导出</button>
	        <label class="wide">擦除确认租户名<input id="complianceErasureConfirmation" autocomplete="off" placeholder="必须完整输入当前租户名"></label>
	        <button id="requestComplianceErasure" type="button" class="danger">申请不可逆擦除</button>
	      </section>
	      <div class="status"><strong>法律保留</strong><span id="complianceHoldCount"></span></div>
	      <div class="tablewrap"><table class="compliance-table" aria-label="法律保留列表">
	        <thead><tr><th>保留单</th><th>租户</th><th>状态</th><th>原因</th><th>有效期</th><th>版本</th><th>操作</th></tr></thead>
	        <tbody id="complianceHolds"><tr><td colspan="7" class="empty">暂无法律保留</td></tr></tbody>
	      </table></div>
	      <div class="status" style="margin-top:12px"><strong>租户数据导出</strong><span id="complianceExportCount"></span></div>
	      <div class="tablewrap"><table class="compliance-table" aria-label="租户数据导出列表">
	        <thead><tr><th>导出单</th><th>租户</th><th>状态</th><th>数据</th><th>加密/完整性</th><th>有效期</th><th>错误</th><th>操作</th></tr></thead>
	        <tbody id="complianceExports"><tr><td colspan="8" class="empty">暂无导出记录</td></tr></tbody>
	      </table></div>
	      <div class="status" style="margin-top:12px"><strong>租户数据擦除</strong><span id="complianceErasureCount"></span></div>
	      <div class="tablewrap"><table class="compliance-table" aria-label="租户数据擦除列表">
	        <thead><tr><th>擦除单</th><th>租户</th><th>状态</th><th>审批</th><th>进度</th><th>结果</th><th>错误</th><th>操作</th></tr></thead>
	        <tbody id="complianceErasures"><tr><td colspan="8" class="empty">暂无擦除记录</td></tr></tbody>
	      </table></div>
	      <dialog id="complianceStepsDialog" class="service-account-dialog compliance-step-dialog" aria-labelledby="complianceStepsTitle">
	        <div class="status"><strong id="complianceStepsTitle">擦除步骤</strong><span id="complianceStepsState"></span></div>
	        <div class="tablewrap"><table class="compliance-step-table" aria-label="擦除步骤列表">
	          <thead><tr><th>顺序</th><th>数据集</th><th>动作</th><th>状态</th><th>影响行</th><th>时间/错误</th></tr></thead>
	          <tbody id="complianceSteps"><tr><td colspan="6" class="empty">暂无步骤</td></tr></tbody>
	        </table></div>
	        <div class="service-account-actions"><button id="closeComplianceSteps" type="button" class="secondary">关闭</button></div>
	      </dialog>
	    </section>

	    <section id="weComCredentialCenter" aria-label="企业微信凭据保护" hidden>
	      <div class="status"><strong>企业微信凭据保护</strong><span id="weComCredentialState">等待加载</span></div>
	      <section class="wecomcredentialbar" aria-label="企业微信凭据轮换">
	        <label>租户 ID<input id="weComCredentialTenantId" inputmode="numeric" placeholder="全部租户"></label>
	        <label>批量上限<input id="weComCredentialLimit" type="number" min="1" max="1000" value="100"></label>
	        <button id="loadWeComCredentialProtection" type="button">刷新</button>
	        <button id="rotateWeComCredentials" type="button" disabled>轮换密钥</button>
	      </section>
	      <section id="weComCredentialSummary" class="summary detail" aria-label="企业微信凭据保护概览"></section>
	    </section>

	    <section id="weChatOpenCredentialCenter" aria-label="微信开放平台凭据保护" hidden>
	      <div class="status"><strong>微信开放平台凭据保护</strong><span id="weChatOpenCredentialState">等待加载</span></div>
	      <section class="wecomcredentialbar" aria-label="微信开放平台凭据轮换">
	        <label>租户 ID<input id="weChatOpenCredentialTenantId" inputmode="numeric" placeholder="全部租户"></label>
	        <label>批量上限<input id="weChatOpenCredentialLimit" type="number" min="1" max="1000" value="100"></label>
	        <button id="loadWeChatOpenCredentialProtection" type="button">刷新</button>
	        <button id="rotateWeChatOpenCredentials" type="button" disabled>轮换密钥</button>
	      </section>
	      <section id="weChatOpenCredentialSummary" class="summary detail" aria-label="微信开放平台凭据保护概览"></section>
	    </section>

	    <section id="serviceAccountCenter" aria-label="服务账号与 API Key" hidden>
		      <div class="status"><strong>服务账号与 API Key</strong><span id="serviceAccountState">等待加载</span></div>
		      <section class="serviceaccountbar" aria-label="服务账号筛选">
		        <label>租户 ID
		          <input id="serviceAccountTenantFilter" inputmode="numeric" placeholder="全部租户">
		        </label>
		        <label>账号状态
		          <select id="serviceAccountStatusFilter"><option value="all">全部</option><option value="active">启用</option><option value="disabled">停用</option></select>
		        </label>
		        <label>搜索
		          <input id="serviceAccountKeyword" placeholder="账号名称、编码或租户">
		        </label>
		        <button id="loadServiceAccounts" type="button">刷新</button>
		      </section>
		      <section id="serviceAccountSummary" class="summary detail" aria-label="服务账号概览"></section>
		      <section class="service-account-history" aria-label="OpenAPI 历史用量">
		        <div class="status"><strong>OpenAPI 历史用量</strong><span id="serviceAccountUsageState">等待加载</span></div>
		        <div class="service-account-historybar">
		          <div>
		            <span class="muted">时间范围</span>
		            <div id="serviceAccountUsageSegments" class="service-account-segments" role="group" aria-label="用量时间范围">
		              <button type="button" data-usage-days="7" aria-pressed="true">7 天</button>
		              <button type="button" data-usage-days="30" aria-pressed="false">30 天</button>
		              <button type="button" data-usage-days="90" aria-pressed="false">90 天</button>
		            </div>
		          </div>
		          <label>服务账号<select id="serviceAccountUsageAccountFilter"><option value="0">全部账号</option></select></label>
		          <div class="service-account-history-actions">
		            <button id="loadServiceAccountUsage" type="button">刷新用量</button>
		            <button id="evaluateServiceAccountUsageAlerts" type="button">立即评估</button>
		            <button id="viewServiceAccountUsageAlerts" type="button" class="secondary">查看预警</button>
		          </div>
		        </div>
		        <section id="serviceAccountUsageSummary" class="summary detail" aria-label="OpenAPI 用量概览"></section>
		        <div id="serviceAccountUsageChartScroll" class="service-account-chart-scroll"><div id="serviceAccountUsageChart" class="service-account-chart" aria-label="OpenAPI 每日请求趋势"></div></div>
		        <div class="service-account-usage-grid">
		          <section aria-label="OpenAPI 路由排行">
		            <div class="status"><strong>路由用量</strong><span id="serviceAccountUsageRouteCount"></span></div>
		            <div class="tablewrap"><table class="service-account-usage-table service-account-usage-route-table"><thead><tr><th>路由</th><th>请求</th><th>拒绝</th><th>账号</th><th>最后调用</th></tr></thead><tbody id="serviceAccountUsageRoutes"><tr><td colspan="5" class="empty">暂无用量</td></tr></tbody></table></div>
		          </section>
		          <section aria-label="OpenAPI 账号排行">
		            <div class="status"><strong>账号用量</strong><span id="serviceAccountUsageAccountCount"></span></div>
		            <div class="tablewrap"><table class="service-account-usage-table service-account-usage-account-table"><thead><tr><th>账号</th><th>租户</th><th>请求</th><th>拒绝</th><th>路由</th><th>最后调用</th></tr></thead><tbody id="serviceAccountUsageAccounts"><tr><td colspan="6" class="empty">暂无用量</td></tr></tbody></table></div>
		          </section>
		        </div>
		      </section>
		      <section id="serviceAccountSecret" class="service-account-secret" aria-label="一次性 API Key" hidden>
		        <div class="full"><strong>一次性 API Key</strong><br><span id="serviceAccountSecretState" class="muted">仅在本次响应中显示</span></div>
		        <label>API Key
		          <input id="serviceAccountPlainTextKey" readonly autocomplete="off">
		        </label>
		        <button id="copyServiceAccountKey" type="button">复制</button>
		        <button id="dismissServiceAccountKey" type="button">关闭</button>
		      </section>
		      <section id="serviceAccountEditor" class="serviceaccountedit" aria-label="服务账号编辑" hidden>
		        <input id="serviceAccountId" type="hidden" value="0"><input id="serviceAccountVersion" type="hidden" value="0">
		        <label>租户 ID<input id="serviceAccountTenantId" inputmode="numeric" placeholder="必填"></label>
		        <label>账号编码<input id="serviceAccountCode" placeholder="data_sync"></label>
		        <label>账号名称<input id="serviceAccountName" placeholder="数据同步"></label>
		        <label>账号状态<select id="serviceAccountStatus"><option value="active">启用</option><option value="disabled">停用</option></select></label>
		        <label>每分钟请求上限<input id="serviceAccountRateLimitPerMinute" type="number" min="1" max="60000" step="1" value="60"></label>
		        <label>每日请求上限（0 不限）<input id="serviceAccountDailyRequestLimit" type="number" min="0" max="100000000" step="1" value="10000"></label>
		        <label class="checkline"><input id="serviceAccountUsageAlertEnabled" type="checkbox" checked>启用用量预警</label>
		        <label>日用量预警线（%）<input id="serviceAccountUsageWarningPercent" type="number" min="1" max="100" step="1" value="80"></label>
		        <label>每日限流拒绝预警（0 关闭）<input id="serviceAccountRejectionWarningCount" type="number" min="0" max="100000000" step="1" value="1"></label>
		        <label>重复通知冷却（分钟）<input id="serviceAccountUsageAlertCooldownMinutes" type="number" min="5" max="10080" step="1" value="60"></label>
		        <label class="wide">账号说明<input id="serviceAccountDescription" placeholder="集成用途和负责人"></label>
		        <label class="wide">账号过期时间<input id="serviceAccountExpiresAt" placeholder="YYYY-MM-DD HH:MM:SS，留空表示不过期"></label>
		        <label class="wide">IP / CIDR 白名单<textarea id="serviceAccountAllowedCidrs" placeholder="127.0.0.1/32&#10;10.0.0.0/8"></textarea></label>
		        <label class="wide">初始密钥名称<input id="serviceAccountInitialKeyName" placeholder="生产主密钥"></label>
		        <label class="wide">初始密钥过期时间<input id="serviceAccountInitialKeyExpiresAt" placeholder="YYYY-MM-DD HH:MM:SS，留空默认 90 天"></label>
		        <div class="full"><span class="muted">授权作用域</span><div id="serviceAccountScopes" class="service-account-scopes"></div></div>
		        <button id="newServiceAccount" type="button">新建</button>
		        <button id="saveServiceAccount" type="button">提交创建审批</button>
		      </section>
		      <section id="serviceAccountKeyEditor" class="serviceaccountedit" aria-label="API Key 轮换" hidden>
		        <label>服务账号<input id="serviceAccountRotateAccount" readonly placeholder="从列表选择"></label>
		        <label>新密钥名称<input id="serviceAccountRotateName" placeholder="2026-Q3"></label>
		        <label>新密钥过期时间<input id="serviceAccountRotateExpiresAt" placeholder="YYYY-MM-DD HH:MM:SS"></label>
		        <label>旧密钥宽限（分钟）<input id="serviceAccountRotateGrace" type="number" min="0" max="10080" value="60"></label>
		        <input id="serviceAccountRotateId" type="hidden" value="0"><input id="serviceAccountRotateVersion" type="hidden" value="0">
		        <button id="rotateServiceAccountKey" type="button">提交轮换审批</button>
		      </section>
		      <div class="status"><strong>服务账号</strong><span id="serviceAccountCount"></span></div>
		      <div class="tablewrap"><table class="service-account-table" aria-label="服务账号列表">
		        <thead><tr><th>账号</th><th>租户</th><th>状态/作用域</th><th>网络/过期</th><th>使用</th><th>API Key</th><th>操作</th></tr></thead>
		        <tbody id="serviceAccounts"><tr><td colspan="7" class="empty">暂无服务账号</td></tr></tbody>
		      </table></div>
		      <dialog id="serviceAccountRevokeDialog" class="service-account-dialog" aria-labelledby="serviceAccountRevokeTitle">
		        <strong id="serviceAccountRevokeTitle">吊销 API Key</strong>
		        <p id="serviceAccountRevokeMessage">吊销后该密钥立即无法使用。</p>
		        <label>申请原因<input id="serviceAccountRevokeReason" maxlength="255" placeholder="说明吊销原因和影响范围"></label>
		        <div class="service-account-actions"><button id="cancelServiceAccountKeyRevoke" type="button" class="secondary">取消</button><button id="confirmServiceAccountKeyRevoke" type="button" class="danger">提交吊销审批</button></div>
		      </dialog>
		    </section>

		    <section id="approvalCenter" aria-label="高风险审批中心" hidden>
		      <div class="status"><strong>高风险审批</strong><span id="approvalState">等待加载</span></div>
		      <section class="accessbar" aria-label="审批筛选">
		        <label>状态
		          <select id="approvalStatus">
		            <option value="all">全部</option><option value="pending">待复核</option><option value="approved">已批准</option>
		            <option value="executing">执行中</option><option value="executed">已执行</option><option value="rejected">已驳回</option>
		            <option value="canceled">已撤回</option><option value="expired">已过期</option>
		          </select>
		        </label>
		        <label>动作
		          <select id="approvalAction"><option value="">全部动作</option></select>
		        </label>
		        <label>风险
		          <select id="approvalRisk"><option value="all">全部</option><option value="critical">严重</option><option value="high">高</option></select>
		        </label>
		        <label>搜索
		          <input id="approvalKeyword" placeholder="审批单、目标或发起人">
		        </label>
		        <button id="loadApprovals" type="button">刷新</button>
		      </section>
	      <section id="approvalSummary" class="summary detail" aria-label="审批汇总"></section>
	      <div class="status"><strong>审批策略</strong><span>在途审批使用申请时的策略快照</span><button id="createApprovalReminders" type="button">生成到期提醒</button></div>
	      <div class="tablewrap"><table aria-label="审批策略">
	        <thead><tr><th>动作</th><th>启用/金额阈值</th><th>会签</th><th>SLA/提醒</th><th>有效期</th><th>版本/操作</th></tr></thead>
	        <tbody id="approvalPolicies"><tr><td colspan="6" class="empty">暂无审批策略</td></tr></tbody>
	      </table></div>
	      <div class="status" style="margin-top:12px"><strong>审批委托</strong><span id="approvalDelegationState">等待加载</span></div>
	      <section class="approvalgovernance" aria-label="审批委托维护">
	        <input id="approvalDelegationId" type="hidden" value="0"><input id="approvalDelegationVersion" type="hidden" value="0">
	        <label>委托人用户 ID<input id="approvalDelegatorUserId" inputmode="numeric" placeholder="具有复核权限"></label>
	        <label>受托人用户 ID<input id="approvalDelegateUserId" inputmode="numeric" placeholder="平台成员"></label>
	        <label>开始时间<input id="approvalDelegationStartsAt" type="datetime-local"></label>
	        <label>结束时间<input id="approvalDelegationEndsAt" type="datetime-local"></label>
	        <label>状态<select id="approvalDelegationStatus"><option value="1">启用</option><option value="2">停用</option></select></label>
	        <label class="wide">委托原因<input id="approvalDelegationReason" placeholder="休假、出差或轮值交接"></label>
	        <button id="newApprovalDelegation" type="button">新建</button>
	        <button id="saveApprovalDelegation" type="button">保存</button>
	      </section>
	      <div class="tablewrap"><table aria-label="审批委托">
	        <thead><tr><th>委托人</th><th>受托人</th><th>有效时间</th><th>状态/原因</th><th>版本/操作</th></tr></thead>
	        <tbody id="approvalDelegations"><tr><td colspan="5" class="empty">暂无审批委托</td></tr></tbody>
	      </table></div>
	      <div class="tablewrap"><table aria-label="高风险审批单">
	        <thead><tr><th>审批单</th><th>动作/目标</th><th>发起</th><th>状态</th><th>会签/执行</th><th>SLA/有效期</th><th>操作</th></tr></thead>
		        <tbody id="approvals"><tr><td colspan="7" class="empty">暂无审批</td></tr></tbody>
		      </table></div>
		      <div class="status" style="margin-top:12px"><strong>审批事件</strong><span id="approvalEventState">选择审批单查看</span></div>
	      <div class="tablewrap"><table aria-label="审批事件">
		        <thead><tr><th>事件</th><th>状态</th><th>操作人</th><th>原因</th><th>时间</th></tr></thead>
		        <tbody id="approvalEvents"><tr><td colspan="5" class="empty">暂无事件</td></tr></tbody>
		      </table></div>
		    </section>

		    <section class="taskcenterbar" aria-label="运营任务中心筛选">
	      <label>任务类型
	        <select id="adminTaskType">
	          <option value="all">全部任务</option>
	          <option value="package_sync">套餐同步</option>
	          <option value="tenant_provision">平台开户</option>
	          <option value="tenant_renewal">租户续费</option>
	        </select>
	      </label>
	      <label>任务状态
	        <select id="adminTaskStatus">
	          <option value="all">全部</option>
	          <option value="pending">待应用</option>
		          <option value="blocked">已阻断</option>
		          <option value="failed">失败</option>
		          <option value="canceled">已取消</option>
		          <option value="applied">已应用</option>
	        </select>
	      </label>
	      <label>任务租户
	        <input id="adminTaskTenantId" inputmode="numeric" placeholder="可空">
	      </label>
	      <label>任务套餐
	        <select id="adminTaskPackageCode">
	          <option value="">全部套餐</option>
	        </select>
			      </label>
	      <label>SLA预警
	        <input id="adminTaskSlaWarningHours" inputmode="numeric" value="4">
	      </label>
	      <label>SLA逾期
	        <input id="adminTaskSlaOverdueHours" inputmode="numeric" value="24">
	      </label>
			      <button id="loadAdminTasks" type="button">刷新任务</button>
			      <button id="loadAdminTaskOwners" type="button">刷新负责人</button>
			      <button id="loadAdminTaskSla" type="button">刷新SLA</button>
			      <button id="createTaskSlaNotifications" type="button">生成SLA提醒</button>
			      <button id="bulkCancelAdminTasks" type="button">批量取消</button>
			      <button id="bulkResetAdminTasks" type="button">批量重置</button>
			      <button id="bulkApplyRenewalTasks" type="button">批量应用续费</button>
			    </section>

	    <section class="packagesyncresult" aria-label="运营任务中心">
	      <div class="status"><strong>运营任务中心</strong><span id="adminTaskSummary">按类型和状态查看平台运营任务</span></div>
	      <div class="tablewrap">
	        <table>
	          <thead><tr><th>任务</th><th>类型/状态</th><th>租户/套餐</th><th>摘要</th><th>错误/时间</th><th>操作</th></tr></thead>
	          <tbody id="adminTasks"><tr><td colspan="6" class="empty">暂无运营任务</td></tr></tbody>
	        </table>
	      </div>
	    </section>

	    <section class="packagesyncresult" aria-label="运营任务负责人">
	      <div class="status"><strong>任务负责人</strong><span id="adminTaskOwnerSummary">按当前任务筛选聚合负责人负载</span></div>
	      <div class="tablewrap">
	        <table>
	          <thead><tr><th>负责人</th><th>待处理</th><th>类型分布</th><th>最近任务</th><th>最近动态</th></tr></thead>
	          <tbody id="adminTaskOwners"><tr><td colspan="5" class="empty">暂无负责人任务</td></tr></tbody>
	        </table>
	      </div>
	    </section>

	    <section class="packagesyncresult" aria-label="运营任务SLA">
	      <div class="status"><strong>任务SLA</strong><span id="adminTaskSlaSummary">按当前任务筛选查看活跃任务时效</span></div>
	      <div class="tablewrap">
	        <table>
	          <thead><tr><th>任务</th><th>SLA</th><th>类型/状态</th><th>负责人</th><th>摘要</th></tr></thead>
	          <tbody id="adminTaskSla"><tr><td colspan="5" class="empty">暂无活跃任务</td></tr></tbody>
	        </table>
	      </div>
	    </section>

	    <section id="tenantReadinessCenter" aria-label="租户上线准备度">
	      <div class="status"><strong>租户上线准备度</strong><span id="tenantReadinessState">等待加载</span></div>
	      <section class="tenantreadinessbar" aria-label="租户上线准备度筛选">
	        <label>准备状态
	          <select id="tenantReadinessFilterState">
	            <option value="all">全部</option>
	            <option value="blocked">已阻塞</option>
	            <option value="attention">待完善</option>
	            <option value="ready">可上线</option>
	          </select>
	        </label>
	        <label>租户 ID
	          <input id="tenantReadinessTenantId" inputmode="numeric" placeholder="全部租户">
	        </label>
	        <label>搜索
	          <input id="tenantReadinessKeyword" placeholder="租户名称或 ID">
	        </label>
	        <button id="loadTenantReadiness" type="button">刷新</button>
	      </section>
	      <section id="tenantReadinessSummary" class="summary detail" aria-label="租户上线准备度汇总">
	        <div class="tile"><div class="label">租户</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">可上线</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">待完善</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">已阻塞</div><div class="value">-</div></div>
	      </section>
	      <div class="tablewrap"><table class="tenant-readiness-table" aria-label="租户上线准备度列表">
	        <thead><tr><th>状态</th><th>租户</th><th>完成度</th><th>核心条件</th><th>待处理项</th><th>操作</th></tr></thead>
	        <tbody id="tenantReadinessTenants"><tr><td colspan="6" class="empty">暂无准备度数据</td></tr></tbody>
	      </table></div>
	    </section>

	    <section class="provisionbar" aria-label="新租户开户">
      <label>租户名称
        <input id="provisionTenantName" placeholder="客户公司名称">
      </label>
      <label>管理员手机
        <input id="provisionAdminPhone" inputmode="numeric" placeholder="11 位手机号">
      </label>
      <label>管理员姓名
        <input id="provisionAdminName" placeholder="超级管理员">
      </label>
      <label>初始密码
        <input id="provisionPassword" type="password" autocomplete="new-password" placeholder="字母或数字">
      </label>
      <label>开通套餐
        <select id="provisionPackageCode">
          <option value="">加载套餐中</option>
        </select>
      </label>
      <label>到期时间
        <input id="provisionExpiresAt" placeholder="YYYY-MM-DD，可空">
      </label>
      <button id="provisionTenant" type="button">提交开户审批</button>
    </section>

    <section class="provisiontask" aria-label="平台开户任务">
      <label>开户任务 ID
        <input id="provisionTaskId" inputmode="numeric" placeholder="可空">
      </label>
      <button id="createProvisionTask" type="button">创建开户任务</button>
      <button id="applyProvisionTask" type="button">提交任务审批</button>
      <button id="bulkApplyProvisionTasks" type="button">批量提交开户审批</button>
      <button id="loadProvisionTasks" type="button">刷新开户任务</button>
    </section>

    <section class="packagesyncresult" aria-label="平台开户任务结果">
      <div class="status"><strong>平台开户任务</strong><span id="provisionTaskSummary">创建任务后显示预览</span></div>
      <div class="tablewrap">
        <table>
          <thead><tr><th>任务</th><th>租户</th><th>管理员</th><th>套餐</th><th>状态</th><th>操作</th></tr></thead>
          <tbody id="provisionTasks"><tr><td colspan="6" class="empty">暂无开户任务</td></tr></tbody>
        </table>
      </div>
    </section>

    <section class="packagebar" aria-label="租户套餐调整">
      <label>调整租户 ID
        <input id="packageTenantId" inputmode="numeric" placeholder="租户 ID">
      </label>
      <label>套餐
        <select id="packageCode">
          <option value="">加载套餐中</option>
        </select>
      </label>
      <label>到期时间
        <input id="packageExpiresAt" placeholder="YYYY-MM-DD，可空">
      </label>
      <label>当前版本
        <input id="packageAssignmentVersion" inputmode="numeric" value="0" readonly>
      </label>
      <label>变更说明
        <input id="packageAssignmentRemark" placeholder="升级、降级或权益调整原因">
      </label>
      <button id="applyPackage" type="button">提交分配审批</button>
    </section>

    <section class="packageedit" aria-label="套餐维护">
      <label>套餐编码
        <input id="editPackageCode" placeholder="scale">
      </label>
      <label>套餐名称
        <input id="editPackageName" placeholder="规模版">
      </label>
      <label>套餐说明
        <input id="editPackageDescription" placeholder="适用客户与权益边界">
      </label>
      <label>版本
        <input id="editPackageVersion" inputmode="numeric" value="0" readonly>
      </label>
      <label>状态
        <select id="editPackageStatus">
          <option value="1">启用</option>
          <option value="2">停用</option>
        </select>
      </label>
      <label>额度 JSON
        <textarea id="editPackageLimits" spellcheck="false" placeholder="{&quot;maxUsers&quot;:120,&quot;channelCodes&quot;:33,&quot;asyncExecutions&quot;:1000}"></textarea>
      </label>
      <button id="savePackage" type="button">提交套餐审批</button>
    </section>

    <section class="packageimpact" aria-label="套餐变更影响">
      <div class="status"><strong>套餐变更影响</strong><span id="packageImpactSummary">保存套餐后显示影响</span></div>
      <div class="tablewrap">
        <table>
          <thead><tr><th>指标</th><th>变化</th><th>原额度</th><th>新额度</th><th>方向</th></tr></thead>
          <tbody id="packageImpactChanges"><tr><td colspan="5" class="empty">暂无变更</td></tr></tbody>
        </table>
      </div>
    </section>

    <section class="packagesync" aria-label="套餐快照同步">
      <label>同步套餐
        <select id="syncPackageCode">
          <option value="">加载套餐中</option>
        </select>
      </label>
      <label>指定租户
        <input id="syncTenantId" inputmode="numeric" placeholder="可空">
      </label>
      <label>命中上限
        <input id="syncLimit" inputmode="numeric" value="100">
      </label>
      <label>超额策略
        <select id="syncAllowOverLimit">
          <option value="false">阻断</option>
          <option value="true">允许</option>
        </select>
      </label>
      <button id="previewPackageSync" type="button">预览同步</button>
      <button id="applyPackageSync" type="button">应用同步</button>
    </section>

    <section class="packagesynctask" aria-label="套餐同步任务">
      <label>任务 ID
        <input id="syncTaskId" inputmode="numeric" placeholder="可空">
      </label>
      <button id="createPackageSyncTask" type="button">创建任务</button>
      <button id="applyPackageSyncTask" type="button">应用任务</button>
      <button id="bulkApplyPackageSyncTasks" type="button">批量应用任务</button>
      <button id="loadPackageSyncTasks" type="button">刷新任务</button>
    </section>

    <section class="packagesyncresult" aria-label="套餐快照同步结果">
      <div class="status"><strong>套餐快照同步</strong><span id="packageSyncSummary">预览后显示命中租户</span></div>
      <div class="tablewrap">
        <table>
          <thead><tr><th>租户</th><th>到期</th><th>状态</th><th>刷新</th><th>超额指标</th></tr></thead>
          <tbody id="packageSyncTenants"><tr><td colspan="5" class="empty">暂无同步结果</td></tr></tbody>
        </table>
      </div>
      <div class="tablewrap">
        <table>
          <thead><tr><th>任务</th><th>套餐</th><th>租户</th><th>状态</th><th>结果</th><th>操作</th></tr></thead>
          <tbody id="packageSyncTasks"><tr><td colspan="6" class="empty">暂无同步任务</td></tr></tbody>
        </table>
      </div>
    </section>

    <section class="tenantbar" aria-label="租户状态">
      <label>状态租户 ID
        <input id="statusTenantId" inputmode="numeric" placeholder="租户 ID">
      </label>
      <label>租户状态
        <select id="tenantStatus">
          <option value="1">正常</option>
          <option value="2">停用</option>
        </select>
      </label>
      <label>备注
        <input id="tenantStatusRemark" placeholder="如：欠费停用、续费恢复">
      </label>
      <button id="updateTenantStatus" type="button">更新状态</button>
    </section>

    <section style="margin-top:14px" aria-label="订阅生命周期">
      <div class="status"><strong>订阅生命周期</strong><span id="subscriptionCount">等待加载</span></div>
      <section class="subscriptionfilter" aria-label="订阅筛选">
        <label>有效状态
          <select id="subscriptionStatus">
            <option value="all">全部</option>
            <option value="trialing">试用</option>
            <option value="active">有效</option>
            <option value="grace">宽限期</option>
            <option value="past_due">欠费</option>
            <option value="suspended">暂停</option>
            <option value="canceled">取消</option>
          </select>
        </label>
        <label>访问状态
          <select id="subscriptionAccess">
            <option value="all">全部</option>
            <option value="allowed">允许访问</option>
            <option value="blocked">阻断访问</option>
          </select>
        </label>
        <label>租户搜索
          <input id="subscriptionKeyword" placeholder="租户、ID、套餐或原因">
        </label>
        <button id="loadSubscriptions" type="button">刷新订阅</button>
        <button id="previewSubscriptionReconcile" type="button">预览对账</button>
        <button id="applySubscriptionReconcile" type="button">执行对账</button>
        <button id="exportSubscriptions" type="button">导出订阅 CSV</button>
      </section>
      <section id="subscriptionSummary" class="summary detail" aria-label="订阅概览">
        <div class="tile"><div class="label">有效订阅</div><div class="value">-</div></div>
        <div class="tile"><div class="label">试用/宽限</div><div class="value">-</div></div>
        <div class="tile"><div class="label">欠费</div><div class="value">-</div></div>
        <div class="tile"><div class="label">暂停/取消</div><div class="value">-</div></div>
        <div class="tile"><div class="label">允许/阻断</div><div class="value">-</div></div>
        <div class="tile"><div class="label">待对账</div><div class="value">-</div></div>
      </section>
      <div class="tablewrap"><table aria-label="订阅列表">
        <thead><tr><th>状态</th><th>租户</th><th>套餐</th><th>周期</th><th>访问</th><th>版本/原因</th><th>操作</th></tr></thead>
        <tbody id="subscriptions"><tr><td colspan="7" class="empty">暂无订阅数据</td></tr></tbody>
      </table></div>
      <section class="subscriptiontransition" aria-label="订阅状态迁移">
        <label>租户 ID
          <input id="subscriptionTenantId" inputmode="numeric" placeholder="选择租户">
        </label>
        <label>目标状态
          <select id="subscriptionTransitionStatus">
            <option value="active">有效</option>
            <option value="trialing">试用</option>
            <option value="grace">宽限期</option>
            <option value="past_due">欠费</option>
            <option value="suspended">暂停</option>
            <option value="canceled">取消</option>
          </select>
        </label>
        <label>当前版本
          <input id="subscriptionVersion" inputmode="numeric" placeholder="版本">
        </label>
        <label>试用结束
          <input id="subscriptionTrialEndsAt" type="datetime-local">
        </label>
        <label>周期结束
          <input id="subscriptionPeriodEndsAt" type="datetime-local">
        </label>
        <label>宽限结束
          <input id="subscriptionGraceEndsAt" type="datetime-local">
        </label>
        <label>原因
          <input id="subscriptionReason" placeholder="回款、暂停或恢复原因">
        </label>
        <button id="transitionSubscription" type="button">提交订阅审批</button>
        <label class="checkline"><input id="subscriptionCancelAtPeriodEnd" type="checkbox"> 周期末取消</label>
      </section>
      <div class="status"><strong>订阅事件</strong><span id="subscriptionEventCount"></span></div>
      <div class="tablewrap"><table aria-label="订阅事件">
        <thead><tr><th>时间</th><th>租户</th><th>事件</th><th>状态变化</th><th>来源</th><th>原因</th></tr></thead>
        <tbody id="subscriptionEvents"><tr><td colspan="6" class="empty">选择租户查看事件</td></tr></tbody>
      </table></div>
    </section>

    <section style="margin-top:14px" aria-label="支付收款">
      <div class="status"><strong>支付收款</strong><span id="paymentOrderCount">等待加载</span></div>
      <section class="paymentfilter" aria-label="支付订单筛选">
        <label>订单状态
          <select id="paymentOrderStatus">
            <option value="all">全部</option>
            <option value="pending">待支付</option>
            <option value="processing">处理中</option>
            <option value="paid">已支付</option>
            <option value="failed">失败</option>
            <option value="canceled">已取消</option>
          </select>
        </label>
        <label>支付渠道
          <input id="paymentProvider" placeholder="全部渠道">
        </label>
        <label>订单搜索
          <input id="paymentKeyword" placeholder="订单、租户、渠道订单或备注">
        </label>
        <button id="loadPaymentOrders" type="button">刷新订单</button>
        <button id="previewPaymentDunning" type="button">预览催缴</button>
        <button id="applyPaymentDunning" type="button">执行催缴</button>
        <button id="exportPaymentOrders" type="button">导出订单 CSV</button>
      </section>
      <section id="paymentSummary" class="summary detail" aria-label="支付概览">
        <div class="tile"><div class="label">订单总数</div><div class="value">-</div></div>
        <div class="tile"><div class="label">待收/处理中</div><div class="value">-</div></div>
        <div class="tile"><div class="label">已收金额</div><div class="value">-</div></div>
        <div class="tile"><div class="label">失败/待催缴</div><div class="value">-</div></div>
        <div class="tile"><div class="label">应收敞口</div><div class="value">-</div></div>
        <div class="tile"><div class="label">租户/渠道</div><div class="value">-</div></div>
      </section>
      <div class="tablewrap"><table aria-label="支付订单">
        <thead><tr><th>状态</th><th>订单/租户</th><th>套餐</th><th>金额</th><th>收银台</th><th>催缴</th><th>版本/操作</th></tr></thead>
        <tbody id="paymentOrders"><tr><td colspan="7" class="empty">暂无支付订单</td></tr></tbody>
      </table></div>
      <section class="paymentcreate" aria-label="创建支付订单">
        <label>租户 ID
          <input id="paymentTenantId" inputmode="numeric" placeholder="租户 ID">
        </label>
        <label>支付渠道
          <input id="paymentCreateProvider" value="gateway" placeholder="gateway">
        </label>
        <label>套餐
          <select id="paymentPackageCode"><option value="">加载套餐中</option></select>
        </label>
        <label>计费周期
          <select id="paymentBillingCycle">
            <option value="yearly">年付</option>
            <option value="monthly">月付</option>
            <option value="custom">自定义</option>
            <option value="lifetime">永久</option>
          </select>
        </label>
        <label>服务到期时间
          <input id="paymentServiceExpiresAt" type="datetime-local">
        </label>
        <label>金额
          <input id="paymentAmount" inputmode="decimal" placeholder="如 12800.00">
        </label>
        <label>币种
          <input id="paymentCurrency" value="CNY" maxlength="3">
        </label>
        <label>最大催缴次数
          <input id="paymentMaxDunningAttempts" inputmode="numeric" value="3">
        </label>
        <label class="wide">收银台地址
          <input id="paymentCheckoutUrl" type="url" placeholder="https://">
        </label>
        <label>收银台到期时间
          <input id="paymentCheckoutExpiresAt" type="datetime-local">
        </label>
        <label>订单号
          <input id="paymentOrderNo" placeholder="留空自动生成">
        </label>
        <label class="wide">幂等键
          <input id="paymentIdempotencyKey" placeholder="业务请求唯一键">
        </label>
        <label class="wide">备注
          <input id="paymentRemark" placeholder="合同、销售或回款备注">
        </label>
        <button id="createPaymentOrder" type="button">创建支付订单</button>
      </section>
	  <div class="status"><strong>退款管理</strong><span id="paymentRefundCount"></span></div>
	  <section class="paymentfilter" aria-label="退款筛选">
	    <label>退款状态
	      <select id="paymentRefundStatus">
	        <option value="all">全部</option>
	        <option value="requested">待处理</option>
	        <option value="processing">处理中</option>
	        <option value="succeeded">已退款</option>
	        <option value="failed">退款失败</option>
	        <option value="canceled">已取消</option>
	      </select>
	    </label>
	    <label>支付订单
	      <input id="paymentRefundOrderFilter" placeholder="全部支付订单">
	    </label>
	    <label>退款搜索
	      <input id="paymentRefundKeyword" placeholder="退款单、租户、原因或渠道单">
	    </label>
	    <button id="loadPaymentRefunds" type="button">刷新退款</button>
	    <button id="exportPaymentRefunds" type="button">导出退款 CSV</button>
	  </section>
	  <section id="paymentRefundSummary" class="summary detail" aria-label="退款概览">
	    <div class="tile"><div class="label">退款单</div><div class="value">-</div></div>
	    <div class="tile"><div class="label">待处理</div><div class="value">-</div></div>
	    <div class="tile"><div class="label">已退款</div><div class="value">-</div></div>
	    <div class="tile"><div class="label">失败/取消</div><div class="value">-</div></div>
	    <div class="tile"><div class="label">租户/订单</div><div class="value">-</div></div>
	  </section>
	  <div class="tablewrap"><table aria-label="支付退款">
	    <thead><tr><th>状态</th><th>退款/支付单</th><th>租户</th><th>金额</th><th>权益动作</th><th>时间/结果</th><th>版本/操作</th></tr></thead>
	    <tbody id="paymentRefunds"><tr><td colspan="7" class="empty">暂无退款</td></tr></tbody>
	  </table></div>
	  <section class="paymentcreate" aria-label="创建支付退款">
	    <label>支付订单
	      <input id="paymentRefundOrderNo" placeholder="PAY-...">
	    </label>
	    <label>退款金额
	      <input id="paymentRefundAmount" inputmode="decimal" placeholder="如 128.00">
	    </label>
	    <label>币种
	      <input id="paymentRefundCurrency" value="CNY" maxlength="3">
	    </label>
	    <label>退款后权益
	      <select id="paymentRefundEntitlement">
	        <option value="keep">保留订阅</option>
	        <option value="suspend">暂停订阅</option>
	        <option value="cancel">取消订阅</option>
	      </select>
	    </label>
	    <label>渠道退款单
	      <input id="paymentProviderRefundNo" placeholder="可选">
	    </label>
	    <label>平台退款单
	      <input id="paymentRefundNo" placeholder="留空自动生成">
	    </label>
	    <label class="wide">幂等键
	      <input id="paymentRefundIdempotencyKey" placeholder="退款请求唯一键">
	    </label>
	    <label class="wide">退款原因
	      <input id="paymentRefundReason" placeholder="必填">
	    </label>
	    <label class="wide">备注
	      <input id="paymentRefundRemark" placeholder="客服工单、合同或审批信息">
	    </label>
		    <button id="createPaymentRefund" type="button">创建退款申请</button>
		  </section>
		  <div class="status"><strong>发票与红冲</strong><span id="invoiceDocumentCount"></span></div>
		  <section class="paymentfilter" aria-label="发票台账筛选">
		    <label>单据类型
		      <select id="invoiceKindFilter">
		        <option value="all">全部</option>
		        <option value="invoice">蓝票</option>
		        <option value="credit_note">红票</option>
		      </select>
		    </label>
		    <label>单据状态
		      <select id="invoiceStatusFilter">
		        <option value="all">全部</option>
		        <option value="requested">待处理</option>
		        <option value="processing">处理中</option>
		        <option value="issued">已开具</option>
		        <option value="failed">失败</option>
		        <option value="canceled">已取消</option>
		      </select>
		    </label>
		    <label>租户 ID
		      <input id="invoiceTenantFilter" inputmode="numeric" placeholder="全部租户">
		    </label>
		    <label>订单/单据搜索
		      <input id="invoiceKeywordFilter" placeholder="订单、单据、抬头或税号">
		    </label>
		    <button id="loadInvoiceDocuments" type="button">刷新台账</button>
		    <button id="exportInvoiceDocuments" type="button">导出发票 CSV</button>
		  </section>
		  <section id="invoiceDocumentSummary" class="summary detail" aria-label="发票台账概览">
		    <div class="tile"><div class="label">单据</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">待处理</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">已开具</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">蓝票/红票</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">净开票额</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">租户/订单</div><div class="value">-</div></div>
		  </section>
		  <div class="tablewrap"><table aria-label="发票台账">
		    <thead><tr><th>类型/状态</th><th>单据/原票</th><th>租户/订单</th><th>金额/余额</th><th>抬头/渠道</th><th>时间/结果</th><th>版本/操作</th></tr></thead>
		    <tbody id="invoiceDocuments"><tr><td colspan="7" class="empty">暂无发票单据</td></tr></tbody>
		  </table></div>
		  <section class="paymentcreate" aria-label="租户开票资料">
		    <label>租户 ID
		      <input id="invoiceProfileTenantId" inputmode="numeric" placeholder="租户 ID">
		    </label>
		    <label>发票类型
		      <select id="invoiceProfileType"><option value="normal">普票</option><option value="special">专票</option></select>
		    </label>
		    <label>发票抬头
		      <input id="invoiceProfileTitle" placeholder="企业全称">
		    </label>
		    <label>纳税人识别号
		      <input id="invoiceProfileTaxId" placeholder="统一社会信用代码">
		    </label>
		    <label>接收邮箱
		      <input id="invoiceProfileEmail" type="email" placeholder="invoice@example.com">
		    </label>
		    <label>联系电话
		      <input id="invoiceProfilePhone" placeholder="可选">
		    </label>
		    <label class="wide">注册地址
		      <input id="invoiceProfileAddress" placeholder="专票必填">
		    </label>
		    <label>开户行
		      <input id="invoiceProfileBankName" placeholder="专票必填">
		    </label>
		    <label>银行账号
		      <input id="invoiceProfileBankAccount" placeholder="专票必填">
		    </label>
		    <label>收件人
		      <input id="invoiceProfileRecipient" placeholder="可选">
		    </label>
		    <label>当前版本
		      <input id="invoiceProfileVersion" inputmode="numeric" value="0" readonly>
		    </label>
		    <label class="wide">资料备注
		      <input id="invoiceProfileRemark" placeholder="合同或财务备注">
		    </label>
		    <button id="loadInvoiceProfile" type="button">读取资料</button>
		    <button id="saveInvoiceProfile" type="button">保存资料</button>
		  </section>
		  <section class="paymentcreate" aria-label="创建发票单据">
		    <label>单据类型
		      <select id="invoiceCreateKind"><option value="invoice">蓝票</option><option value="credit_note">红票</option></select>
		    </label>
		    <label>租户 ID
		      <input id="invoiceCreateTenantId" inputmode="numeric" placeholder="租户 ID">
		    </label>
		    <label>支付订单
		      <input id="invoiceCreateOrderNo" placeholder="PAY-...">
		    </label>
		    <label>原蓝票单号
		      <input id="invoiceOriginalDocumentNo" placeholder="红票必填">
		    </label>
		    <label>金额
		      <input id="invoiceCreateAmount" inputmode="decimal" placeholder="如 128.00">
		    </label>
		    <label>币种
		      <input id="invoiceCreateCurrency" value="CNY" maxlength="3">
		    </label>
		    <label>开票渠道
		      <input id="invoiceCreateProvider" value="manual" placeholder="manual">
		    </label>
		    <label>单据号
		      <input id="invoiceCreateDocumentNo" placeholder="留空自动生成">
		    </label>
		    <label class="wide">幂等键
		      <input id="invoiceCreateIdempotencyKey" placeholder="业务请求唯一键">
		    </label>
		    <label class="wide">申请备注
		      <input id="invoiceCreateRemark" placeholder="合同、退款或审批信息">
		    </label>
		    <button id="createInvoiceDocument" type="button">创建单据</button>
		  </section>
		  <section class="paymentcreate" aria-label="处理发票单据">
		    <label>单据号
		      <input id="invoiceTransitionDocumentNo" placeholder="INV-/CRN-...">
		    </label>
		    <label>当前版本
		      <input id="invoiceTransitionVersion" inputmode="numeric" placeholder="版本">
		    </label>
		    <label>目标状态
		      <select id="invoiceTransitionStatus"><option value="processing">处理中</option><option value="issued">已开具</option><option value="failed">失败</option><option value="canceled">已取消</option></select>
		    </label>
		    <label>开票渠道
		      <input id="invoiceTransitionProvider" value="manual" placeholder="manual">
		    </label>
		    <label>渠道单号
		      <input id="invoiceProviderDocumentNo" placeholder="开具时必填">
		    </label>
		    <label class="wide">电子票地址
		      <input id="invoiceDocumentUrl" type="url" placeholder="https://">
		    </label>
		    <label>开具时间
		      <input id="invoiceIssuedAt" type="datetime-local">
		    </label>
		    <label>失败代码
		      <input id="invoiceFailureCode" placeholder="可选">
		    </label>
		    <label class="wide">失败原因
		      <input id="invoiceFailureMessage" placeholder="失败时必填">
		    </label>
		    <label class="wide">处理备注
		      <input id="invoiceTransitionRemark" placeholder="财务处理说明">
		    </label>
		    <button id="transitionInvoiceDocument" type="button">应用状态</button>
		  </section>
		  <div class="status"><strong>结算自动同步</strong><span id="paymentSettlementSyncState"></span></div>
		  <section class="paymentfilter" aria-label="结算自动同步筛选">
		    <label>同步渠道
		      <select id="paymentSettlementSyncProvider"><option value="">全部渠道</option></select>
		    </label>
		    <label>运行来源
		      <select id="paymentSettlementSyncSource"><option value="all">全部</option><option value="cron">定时任务</option><option value="manual">人工触发</option></select>
		    </label>
		    <label>运行状态
		      <select id="paymentSettlementSyncStatus"><option value="all">全部</option><option value="running">运行中</option><option value="succeeded">成功</option><option value="previewed">预演</option><option value="failed">失败</option></select>
		    </label>
		    <button id="loadPaymentSettlementSyncRuns" type="button">刷新同步</button>
		    <button id="previewPaymentSettlementSync" type="button">预演</button>
		    <button id="runPaymentSettlementSync" type="button">立即同步</button>
		  </section>
		  <section id="paymentSettlementSyncSummary" class="summary detail" aria-label="结算同步概览">
		    <div class="tile"><div class="label">渠道状态</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">同步运行</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">导入批次</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">同步明细</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">同步差异</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">失败运行</div><div class="value">-</div></div>
		  </section>
		  <div class="tablewrap"><table aria-label="结算同步渠道状态">
		    <thead><tr><th>渠道</th><th>运行状态</th><th>增量游标</th><th>最近尝试/成功</th><th>最近错误</th></tr></thead>
		    <tbody id="paymentSettlementSyncStates"><tr><td colspan="5" class="empty">暂无同步渠道</td></tr></tbody>
		  </table></div>
		  <div class="tablewrap"><table aria-label="结算同步运行记录">
		    <thead><tr><th>状态</th><th>运行号</th><th>渠道/来源</th><th>游标</th><th>批次/明细</th><th>差异</th><th>时间/结果</th></tr></thead>
		    <tbody id="paymentSettlementSyncRuns"><tr><td colspan="7" class="empty">暂无同步记录</td></tr></tbody>
		  </table></div>
		  <div class="status"><strong>渠道结算对账</strong><span id="paymentSettlementBatchCount"></span></div>
		  <section class="paymentfilter" aria-label="渠道结算批次筛选">
		    <label>支付渠道
		      <input id="paymentSettlementProvider" placeholder="gateway">
		    </label>
		    <label>批次状态
		      <select id="paymentSettlementStatus"><option value="all">全部</option><option value="reconciled">待关闭</option><option value="closed">已关闭</option></select>
		    </label>
		    <label>批次搜索
		      <input id="paymentSettlementKeyword" placeholder="平台批次、渠道批次或备注">
		    </label>
		    <button id="loadPaymentSettlementBatches" type="button">刷新批次</button>
		    <button id="exportPaymentSettlementBatches" type="button">导出批次</button>
		  </section>
		  <section id="paymentSettlementSummary" class="summary detail" aria-label="渠道结算概览">
		    <div class="tile"><div class="label">结算批次</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">结算明细</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">自动匹配</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">待处理差异</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">渠道净结算</div><div class="value">-</div></div>
		    <div class="tile"><div class="label">账本差额</div><div class="value">-</div></div>
		  </section>
		  <div class="tablewrap"><table aria-label="渠道结算批次">
		    <thead><tr><th>状态</th><th>批次</th><th>渠道/周期</th><th>明细</th><th>金额</th><th>差异</th><th>版本/操作</th></tr></thead>
		    <tbody id="paymentSettlementBatches"><tr><td colspan="7" class="empty">暂无结算批次</td></tr></tbody>
		  </table></div>
		  <section class="paymentcreate" aria-label="导入渠道结算单">
		    <label class="wide">结算 CSV 文件
		      <input id="paymentSettlementFile" type="file" accept=".csv,text/csv">
		    </label>
		    <label class="wide">导入状态
		      <input id="paymentSettlementImportState" value="等待导入" readonly>
		    </label>
		    <label class="full">结算 CSV
		      <textarea id="paymentSettlementCSV" spellcheck="false" placeholder="batchNo,provider,providerSettlementNo,periodStart,periodEnd,currency,lineNo,providerTransactionNo,transactionType,orderNo,providerOrderNo,refundNo,providerRefundNo,amountCents,feeCents,netAmountCents,occurredAt,remark"></textarea>
		    </label>
		    <button id="importPaymentSettlement" type="button">导入并对账</button>
		  </section>
		  <div class="status"><strong>结算差异明细</strong><span id="paymentSettlementEntryCount"></span></div>
		  <section class="paymentfilter" aria-label="渠道结算明细筛选">
		    <label>当前批次
		      <input id="paymentSettlementSelectedBatch" placeholder="选择批次" readonly>
		    </label>
		    <label>匹配状态
		      <select id="paymentSettlementReconciliationStatus"><option value="all">全部</option><option value="matched">已匹配</option><option value="missing_internal">缺内部单</option><option value="identifier_conflict">标识冲突</option><option value="status_mismatch">状态不符</option><option value="currency_mismatch">币种不符</option><option value="amount_mismatch">金额不符</option></select>
		    </label>
		    <label>处理状态
		      <select id="paymentSettlementHandlingStatus"><option value="all">全部</option><option value="open">待处理</option><option value="resolved">已解决</option><option value="ignored">已忽略</option><option value="none">无需处理</option></select>
		    </label>
		    <label>明细搜索
		      <input id="paymentSettlementEntryKeyword" placeholder="渠道流水、订单或租户">
		    </label>
		    <button id="loadPaymentSettlementEntries" type="button">刷新明细</button>
		    <button id="exportPaymentSettlementEntries" type="button">导出明细</button>
		  </section>
		  <div class="tablewrap"><table aria-label="渠道结算明细">
		    <thead><tr><th>匹配/处理</th><th>渠道流水</th><th>内部账本</th><th>渠道金额</th><th>预期/差额</th><th>问题</th><th>版本/操作</th></tr></thead>
		    <tbody id="paymentSettlementEntries"><tr><td colspan="7" class="empty">请选择结算批次</td></tr></tbody>
		  </table></div>
	      <div class="status"><strong>支付回调事件</strong><span id="paymentWebhookEventCount"></span></div>
      <div class="tablewrap"><table aria-label="支付回调事件">
        <thead><tr><th>状态</th><th>事件</th><th>订单</th><th>渠道</th><th>发生/处理时间</th><th>结果</th></tr></thead>
        <tbody id="paymentWebhookEvents"><tr><td colspan="6" class="empty">暂无支付回调事件</td></tr></tbody>
      </table></div>
    </section>

    <section class="renewalbar" aria-label="续费账单">
      <label>续费租户 ID
        <input id="renewalTenantId" inputmode="numeric" placeholder="租户 ID">
      </label>
      <label>续费套餐
        <select id="renewalPackageCode">
          <option value="">沿用当前套餐</option>
        </select>
      </label>
      <label>新到期时间
        <input id="renewalExpiresAt" placeholder="YYYY-MM-DD">
      </label>
      <label>金额
        <input id="renewalAmount" inputmode="decimal" placeholder="如 12800">
      </label>
      <label>订单号
        <input id="renewalOrderNo" placeholder="可选">
      </label>
      <button id="applyRenewal" type="button">记录续费</button>
    </section>

    <section class="renewaltask" aria-label="续费任务">
      <label>续费任务 ID
        <input id="renewalTaskId" inputmode="numeric" placeholder="可空">
      </label>
      <button id="createRenewalTask" type="button">创建续费任务</button>
      <button id="applyRenewalTask" type="button">应用续费任务</button>
      <button id="loadRenewalTasks" type="button">刷新续费任务</button>
    </section>

    <section class="packagesyncresult" aria-label="续费任务结果">
      <div class="status"><strong>续费任务</strong><span id="renewalTaskSummary">创建任务后显示预览</span></div>
      <div class="tablewrap">
        <table>
          <thead><tr><th>任务</th><th>租户</th><th>套餐</th><th>状态</th><th>结果</th><th>操作</th></tr></thead>
          <tbody id="renewalTasks"><tr><td colspan="6" class="empty">暂无续费任务</td></tr></tbody>
        </table>
      </div>
    </section>

	    <div id="status" class="status">等待加载</div>
	    <section id="summary" class="summary" aria-label="概览"></section>
	    <section style="margin-top:14px" aria-label="经营指标">
	      <div class="status"><strong>经营指标</strong><span id="businessMetricsHint"></span></div>
	      <section id="businessMetrics" class="summary detail">
	        <div class="tile"><div class="label">估算 MRR</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">估算 ARR</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">风险收入</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">到期收入</div><div class="value">-</div></div>
	      </section>
	      <div class="tablewrap"><table>
	        <thead><tr><th>套餐</th><th>租户</th><th>估算 MRR</th><th>风险/到期</th><th>最近账单</th></tr></thead>
	        <tbody id="businessPackages"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
	      </table></div>
	      <div class="status" style="margin-top:12px"><strong>会签明细</strong><span id="approvalDecisionState">选择审批单查看</span></div>
	      <div class="tablewrap"><table aria-label="审批会签明细">
	        <thead><tr><th>审批人</th><th>委托来源</th><th>决定</th><th>意见</th><th>时间</th></tr></thead>
	        <tbody id="approvalDecisions"><tr><td colspan="5" class="empty">暂无会签决定</td></tr></tbody>
	      </table></div>
	    </section>
	    <section style="margin-top:14px" aria-label="经营趋势">
	      <div class="status"><strong>经营趋势</strong><span id="businessTrendsHint"></span></div>
	      <section id="businessTrends" class="summary detail">
	        <div class="tile"><div class="label">趋势流水</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">续费笔数</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">待处理续费</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">已应用任务</div><div class="value">-</div></div>
	      </section>
	      <div class="tablewrap"><table>
	        <thead><tr><th>月份</th><th>账单流水</th><th>账单/续费</th><th>租户/套餐</th><th>Top 套餐</th></tr></thead>
	        <tbody id="businessTrendMonths"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
	      </table></div>
	      <div class="tablewrap"><table>
	        <thead><tr><th>任务</th><th>租户</th><th>套餐</th><th>状态</th><th>错误</th></tr></thead>
	        <tbody id="businessRenewalFunnel"><tr><td colspan="5" class="empty">暂无续费任务</td></tr></tbody>
	      </table></div>
	    </section>
	    <section style="margin-top:14px" aria-label="运营待办队列">
	      <div class="status"><strong>运营待办队列</strong><span id="operationQueueCount"></span></div>
	      <section class="operationqueuebar" aria-label="运营待办筛选">
	        <label>待办来源
	          <select id="operationQueueSource">
	            <option value="all">全部</option>
	            <option value="customer_success">客户成功</option>
	            <option value="task_sla">任务SLA</option>
	            <option value="billing_follow_up">账单跟进</option>
	            <option value="notification">失败通知</option>
	            <option value="closed_notification">关闭通知</option>
	            <option value="notification_health">通知健康</option>
	          </select>
	        </label>
	        <label>优先级
	          <select id="operationQueuePriority">
	            <option value="all">全部</option>
	            <option value="critical">严重</option>
	            <option value="high">高</option>
	            <option value="medium">中</option>
	            <option value="normal">普通</option>
	          </select>
	        </label>
	        <label>负责人
	          <input id="operationQueueOwner" placeholder="负责人">
	        </label>
	        <label>认领到期
	          <select id="operationQueueAssignmentDueState">
	            <option value="all">全部</option>
	            <option value="overdue">已逾期</option>
	            <option value="due_soon">7天内</option>
	            <option value="future">未来</option>
	            <option value="no_date">无日期</option>
	            <option value="closed">已关闭</option>
	          </select>
	        </label>
	        <label>认领视图
	          <select id="operationQueueAssignmentCurrentOnly">
	            <option value="true">当前认领</option>
	            <option value="false">全部历史</option>
	          </select>
	        </label>
	        <label>关键字
	          <input id="operationQueueKeyword" placeholder="租户、原因、动作或对象">
	        </label>
	        <button id="applyOperationQueue" type="button">刷新待办</button>
	      </section>
	      <section class="operationqueuebar" aria-label="运营待办批量分派">
	        <label>分派给
	          <input id="operationQueueAssignOwner" placeholder="负责人">
	        </label>
	        <label>跟进状态
	          <select id="operationQueueAssignStatus">
	            <option value="pending">待跟进</option>
	            <option value="contacted">已联系</option>
	            <option value="renewal_pending">续费待确认</option>
	          </select>
	        </label>
	        <label>下次跟进
	          <input id="operationQueueAssignNextAt" placeholder="2026-07-15 10:00:00">
	        </label>
	        <label>备注
	          <input id="operationQueueAssignRemark" placeholder="页面运营待办批量分派">
	        </label>
	        <button id="assignOperationQueue" type="button">分派当前待办</button>
	        <button id="createOperationQueueAssignmentNotifications" type="button">生成认领到期提醒</button>
	      </section>
	      <div class="tablewrap"><table>
	        <thead><tr><th>待办</th><th>租户</th><th>优先级</th><th>负责人</th><th>下一步</th></tr></thead>
	        <tbody id="operationQueue"><tr><td colspan="5" class="empty">暂无待办</td></tr></tbody>
	      </table></div>
	      <section style="margin-top:14px" aria-label="运营待办负责人工作台">
	        <div class="status"><strong>运营待办负责人工作台</strong><span id="operationQueueOwnerCount"></span></div>
	        <div class="tablewrap"><table>
	          <thead><tr><th>负责人</th><th>优先级</th><th>来源</th><th>Top 租户</th><th>Top 待办</th></tr></thead>
	          <tbody id="operationQueueOwners"><tr><td colspan="5" class="empty">暂无负责人</td></tr></tbody>
	        </table></div>
	      </section>
	      <section style="margin-top:14px" aria-label="运营待办认领记录">
	        <div class="status"><strong>运营待办认领记录</strong><span id="operationQueueAssignmentCount"></span></div>
	        <div class="tablewrap"><table>
	          <thead><tr><th>认领对象</th><th>负责人</th><th>状态</th><th>备注</th><th>操作</th></tr></thead>
	          <tbody id="operationQueueAssignments"><tr><td colspan="5" class="empty">暂无认领记录</td></tr></tbody>
	        </table></div>
	      </section>
	    </section>
	    <section style="margin-top:14px" aria-label="续费预测">
	      <div class="status"><strong>续费预测</strong><span id="renewalForecastHint"></span></div>
	      <section class="renewaltask" aria-label="续费预测筛选">
	        <label>预测天数
	          <input id="renewalForecastDays" inputmode="numeric" value="90">
	        </label>
	        <label>到期窗口
	          <select id="renewalForecastBucket">
	            <option value="all">全部</option>
	            <option value="expired">已到期</option>
	            <option value="due_0_30">30 天内</option>
	            <option value="due_31_60">31-60 天</option>
	            <option value="due_61_90">61-90 天</option>
	            <option value="due_later">90 天以上</option>
	          </select>
	        </label>
	        <label>定价状态
	          <select id="renewalForecastPriced">
	            <option value="all">全部</option>
	            <option value="priced">已定价</option>
	            <option value="unknown">未知价格</option>
	          </select>
	        </label>
	        <label>套餐
	          <input id="renewalForecastPackageCode" placeholder="全部">
	        </label>
	        <label>负责人
	          <input id="renewalForecastOwner" placeholder="全部">
	        </label>
	        <label>任务状态
	          <select id="renewalForecastTaskStatus">
	            <option value="all">全部</option>
	            <option value="none">无任务</option>
	            <option value="pending">待应用</option>
	            <option value="blocked">阻断</option>
	            <option value="failed">失败</option>
	            <option value="applied">已应用</option>
	            <option value="canceled">已取消</option>
	          </select>
	        </label>
	        <button id="applyRenewalForecastFilters" type="button">刷新预测</button>
	      </section>
	      <section class="renewaltask" aria-label="续费预测分派">
	        <label>分派负责人
	          <input id="renewalForecastAssignOwner" placeholder="客户成功或销售">
	        </label>
	        <label>下次跟进
	          <input id="renewalForecastAssignNextAt" placeholder="YYYY-MM-DD">
	        </label>
	        <label>分派状态
	          <select id="renewalForecastAssignStatus">
	            <option value="renewal_pending">续费跟进</option>
	            <option value="pending">待跟进</option>
	            <option value="contacted">已联系</option>
	          </select>
	        </label>
	        <label>分派备注
	          <input id="renewalForecastAssignRemark" placeholder="分派说明">
	        </label>
	        <button id="assignRenewalForecast" type="button">分派预测客户</button>
	      </section>
	      <section id="renewalForecast" class="summary detail">
	        <div class="tile"><div class="label">预测续费</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">30天内</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">已到期</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">待处理任务</div><div class="value">-</div></div>
	      </section>
	      <section class="renewaltask" aria-label="续费预测任务">
	        <label>预测续费月数
	          <input id="renewalForecastTaskMonths" inputmode="numeric" value="12">
	        </label>
	        <label>订单前缀
	          <input id="renewalForecastTaskOrderPrefix" placeholder="可选">
	        </label>
	        <label class="checkline"><input id="renewalForecastTaskForce" type="checkbox"> 强制重建</label>
	        <button id="createRenewalForecastTasks" type="button">生成预测续费任务</button>
	      </section>
	      <section class="renewaltask" aria-label="续费预测提醒">
	        <label>提醒窗口
	          <input id="renewalForecastReminderDays" inputmode="numeric" value="30">
	        </label>
	        <label>投递次数
	          <input id="renewalForecastMaxAttempts" inputmode="numeric" value="3">
	        </label>
	        <label>提醒备注
	          <input id="renewalForecastNotifyRemark" placeholder="预测提醒说明">
	        </label>
	        <label class="checkline"><input id="renewalForecastForceNotify" type="checkbox"> 强制重建</label>
	        <button id="createRenewalForecastNotifications" type="button">生成预测续费提醒</button>
	      </section>
	      <div class="tablewrap"><table>
	        <thead><tr><th>窗口</th><th>租户</th><th>预测续费</th><th>未知价格</th><th>待处理任务</th></tr></thead>
	        <tbody id="renewalForecastBuckets"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
	      </table></div>
	      <div class="tablewrap"><table aria-label="续费预测负责人工作台">
	        <thead><tr><th>负责人</th><th>预测续费</th><th>到期窗口</th><th>任务</th><th>Top 租户</th></tr></thead>
	        <tbody id="renewalForecastOwners"><tr><td colspan="5" class="empty">暂无负责人</td></tr></tbody>
	      </table></div>
	      <div class="tablewrap"><table>
	        <thead><tr><th>租户</th><th>到期</th><th>套餐</th><th>预测续费</th><th>任务</th></tr></thead>
	        <tbody id="renewalForecastTenants"><tr><td colspan="5" class="empty">暂无到期租户</td></tr></tbody>
	      </table></div>
	    </section>
	    <section style="margin-top:14px" aria-label="运营日报">
	      <div class="status"><strong>运营日报</strong><span id="dailyReportHint"></span></div>
	      <section class="dailyreportbar" aria-label="运营日报筛选">
	        <label>日报日期
	          <input id="dailyReportDate" placeholder="YYYY-MM-DD">
	        </label>
	        <label>统计天数
	          <input id="dailyReportDays" inputmode="numeric" value="1">
	        </label>
	        <button id="applyDailyReport" type="button">刷新日报</button>
	      </section>
	      <div id="dailyReport" class="summary detail">
	        <div class="tile"><div class="label">风险租户</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">打开跟进</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">通知失败</div><div class="value">-</div></div>
	        <div class="tile"><div class="label">今日流水</div><div class="value">-</div></div>
	      </div>
	      <section class="dailyreportdetail" aria-label="运营日报明细">
	        <section>
	          <div class="status"><strong>日报负责人</strong><span id="dailyReportOwnerCount"></span></div>
	          <div class="tablewrap"><table>
	            <thead><tr><th>负责人</th><th>打开任务</th><th>到期分布</th><th>下次跟进</th></tr></thead>
	            <tbody id="dailyReportOwners"><tr><td colspan="4" class="empty">暂无数据</td></tr></tbody>
	          </table></div>
	        </section>
	        <section>
	          <div class="status"><strong>日报任务SLA</strong><span id="dailyReportTaskSlaCount"></span></div>
	          <div class="tablewrap"><table>
	            <thead><tr><th>任务</th><th>SLA</th><th>负责人</th><th>结果</th></tr></thead>
	            <tbody id="dailyReportTaskSla"><tr><td colspan="4" class="empty">暂无数据</td></tr></tbody>
	          </table></div>
	        </section>
	        <section>
	          <div class="status"><strong>日报通知</strong><span id="dailyReportNotificationCount"></span></div>
	          <div class="tablewrap"><table>
	            <thead><tr><th>租户</th><th>指标</th><th>状态</th><th>失败原因</th></tr></thead>
	            <tbody id="dailyReportNotifications"><tr><td colspan="4" class="empty">暂无数据</td></tr></tbody>
	          </table></div>
	        </section>
	        <section>
	          <div class="status"><strong>日报待办认领</strong><span id="dailyReportQueueAssignmentCount"></span></div>
	          <div class="tablewrap"><table>
	            <thead><tr><th>认领对象</th><th>负责人</th><th>状态</th><th>备注</th></tr></thead>
	            <tbody id="dailyReportQueueAssignments"><tr><td colspan="4" class="empty">暂无数据</td></tr></tbody>
	          </table></div>
	        </section>
	        <section>
	          <div class="status"><strong>日报操作动作</strong><span id="dailyReportActionCount"></span></div>
	          <div class="tablewrap"><table>
	            <thead><tr><th>动作</th><th>次数</th><th>窗口</th></tr></thead>
	            <tbody id="dailyReportActions"><tr><td colspan="3" class="empty">暂无数据</td></tr></tbody>
	          </table></div>
	        </section>
	        <section>
	          <div class="status"><strong>日报账单流水</strong><span id="dailyReportBillingCount"></span></div>
	          <div class="tablewrap"><table>
	            <thead><tr><th>时间</th><th>租户</th><th>套餐</th><th>金额/订单</th></tr></thead>
	            <tbody id="dailyReportBilling"><tr><td colspan="4" class="empty">暂无数据</td></tr></tbody>
	          </table></div>
	        </section>
	      </section>
	    </section>
	    <section style="margin-top:14px" aria-label="客户成功队列">
	      <div class="status"><strong>客户成功队列</strong><span id="customerSuccessCount"></span></div>
	      <section class="customersuccessbar" aria-label="客户成功队列筛选">
	        <label>优先级
	          <select id="customerSuccessPriority">
	            <option value="">待处理</option>
	            <option value="critical">紧急</option>
	            <option value="high">高</option>
	            <option value="medium">中</option>
	            <option value="normal">正常</option>
	            <option value="all">全部</option>
	          </select>
	        </label>
	        <label>负责人
	          <input id="customerSuccessOwner" placeholder="客户成功或销售">
	        </label>
	        <label>租户上限
	          <input id="customerSuccessTenantLimit" inputmode="numeric" value="200">
	        </label>
	        <button id="applyCustomerSuccess" type="button">筛队列</button>
	      </section>
	      <section class="customersuccessbar" aria-label="客户成功批量分派">
	        <label>分派负责人
	          <input id="customerSuccessAssignOwner" placeholder="客户成功或销售">
	        </label>
	        <label>下次跟进
	          <input id="customerSuccessAssignNextAt" placeholder="YYYY-MM-DD">
	        </label>
	        <label>分派备注
	          <input id="customerSuccessAssignRemark" placeholder="分派说明">
	        </label>
	        <button id="assignCustomerSuccess" type="button">批量分派</button>
	      </section>
	      <section class="customersuccessbar" aria-label="客户成功续费任务">
	        <label>续费月数
	          <input id="customerSuccessRenewMonths" inputmode="numeric" value="12">
	        </label>
	        <label>续费金额
	          <input id="customerSuccessRenewAmount" placeholder="0.00">
	        </label>
	        <label>任务备注
	          <input id="customerSuccessRenewRemark" placeholder="续费任务说明">
	        </label>
	        <button id="createCustomerSuccessRenewalTasks" type="button">生成续费任务</button>
	      </section>
	      <section class="customersuccessbar" aria-label="客户成功续费提醒">
	        <label>提醒窗口
	          <input id="customerSuccessRenewalReminderDays" inputmode="numeric" value="30">
	        </label>
	        <label>投递次数
	          <input id="customerSuccessRenewalMaxAttempts" inputmode="numeric" value="3">
	        </label>
	        <label>提醒备注
	          <input id="customerSuccessRenewalNotifyRemark" placeholder="续费提醒说明">
	        </label>
	        <label class="checkline"><input id="customerSuccessRenewalForceNotify" type="checkbox"> 强制重建</label>
	        <button id="createCustomerSuccessRenewalNotifications" type="button">生成续费提醒</button>
	      </section>
		      <div class="tablewrap"><table>
		        <thead><tr><th>优先级</th><th>租户</th><th>负责人</th><th>待处理</th><th>下一步</th><th>操作</th></tr></thead>
		        <tbody id="customerSuccess"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
		      </table></div>
		      <section style="margin-top:14px" aria-label="客户成功负责人工作台">
		        <div class="status"><strong>客户成功负责人工作台</strong><span id="customerSuccessOwnerCount"></span></div>
		        <div class="tablewrap"><table>
		          <thead><tr><th>负责人</th><th>租户/健康</th><th>优先级</th><th>待处理信号</th><th>下次/重点租户</th></tr></thead>
		          <tbody id="customerSuccessOwners"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
		        </table></div>
		      </section>
		    </section>
	    <section style="margin-top:14px" aria-label="风险看板">
      <div class="status"><strong>风险看板</strong><span id="riskCount"></span></div>
      <section class="riskfollowbar" aria-label="风险跟进">
        <label>跟进状态
          <select id="riskFollowStatus">
            <option value="contacted">已联系</option>
            <option value="pending">待跟进</option>
            <option value="renewal_pending">续费中</option>
            <option value="resolved">已解决</option>
            <option value="ignored">忽略</option>
          </select>
        </label>
        <label>负责人
          <input id="riskFollowOwner" placeholder="客户成功或销售">
        </label>
        <label>下次跟进
          <input id="riskFollowNextAt" placeholder="YYYY-MM-DD">
        </label>
        <label>备注
          <input id="riskFollowRemark" placeholder="跟进结论、续费/扩容动作">
        </label>
      </section>
      <div class="tablewrap"><table>
        <thead><tr><th>风险</th><th>租户</th><th>原因</th><th>最高风险指标</th><th>建议动作</th><th>跟进</th><th>操作</th></tr></thead>
        <tbody id="riskTenants"><tr><td colspan="7" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="risktaskbar" aria-label="风险跟进任务筛选">
      <label>跟进状态
        <select id="riskTaskStatus">
          <option value="">全部</option>
          <option value="pending">待跟进</option>
          <option value="contacted">已联系</option>
          <option value="renewal_pending">续费中</option>
          <option value="resolved">已解决</option>
          <option value="ignored">忽略</option>
        </select>
      </label>
      <label>到期状态
        <select id="riskTaskDueState">
          <option value="all">全部</option>
          <option value="overdue">已逾期</option>
          <option value="due_soon">7 天内</option>
          <option value="future">未来</option>
          <option value="no_date">无日期</option>
          <option value="closed">已关闭</option>
        </select>
      </label>
      <label>负责人
        <input id="riskTaskOwner" placeholder="负责人">
      </label>
      <label>任务关键字
        <input id="riskTaskKeyword" placeholder="租户、备注或负责人">
      </label>
      <button id="applyRiskTaskFilters" type="button">筛任务</button>
      <button id="bulkCloseRiskFollowUps" type="button">批量关闭任务</button>
    </section>
    <section style="margin-top:14px" aria-label="风险跟进任务">
      <div class="status"><strong>风险跟进任务</strong><span id="riskTaskCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>租户</th><th>状态</th><th>负责人</th><th>下次跟进</th><th>备注</th><th>记录</th></tr></thead>
        <tbody id="riskFollowUps"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section style="margin-top:14px" aria-label="风险跟进负责人工作台">
      <div class="status"><strong>负责人工作台</strong><span id="riskOwnerCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>负责人</th><th>打开任务</th><th>状态分布</th><th>到期分布</th><th>最近/下次</th></tr></thead>
        <tbody id="riskFollowUpOwners"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section aria-label="租户详情">
      <div class="status"><strong>租户详情</strong><span id="tenantDetailHint"></span></div>
      <div id="tenantDetail" class="summary detail">
        <div class="tile"><div class="label">租户</div><div class="value">-</div></div>
        <div class="tile"><div class="label">套餐</div><div class="value">-</div></div>
        <div class="tile"><div class="label">最高用量</div><div class="value">-</div></div>
        <div class="tile"><div class="label">近期操作</div><div class="value">-</div></div>
      </div>
	    </section>
	    <section style="margin-top:14px" aria-label="生命周期审计">
	      <div class="status"><strong>生命周期审计</strong><span id="tenantLifecycleCount"></span></div>
	      <div class="logfilterbar" aria-label="生命周期筛选">
	        <label>来源
	          <select id="tenantLifecycleSource">
	            <option value="all">全部</option>
	            <option value="operation">操作</option>
	            <option value="billing">账单</option>
	            <option value="task">任务</option>
	            <option value="alert">告警</option>
	            <option value="notification">通知</option>
	          </select>
	        </label>
	        <label>事件
	          <input id="tenantLifecycleEventType" placeholder="tenant_renewal">
	        </label>
	        <label>状态
	          <input id="tenantLifecycleStatus" placeholder="pending">
	        </label>
	        <label>关键字
	          <input id="tenantLifecycleKeyword" placeholder="订单、提醒或备注">
	        </label>
	        <button id="loadTenantLifecycle" type="button">刷新审计</button>
	        <button id="exportTenantLifecycle" type="button">导出审计 CSV</button>
	      </div>
	      <div class="tablewrap"><table>
	        <thead><tr><th>时间</th><th>来源</th><th>事件</th><th>状态</th><th>备注</th></tr></thead>
	        <tbody id="tenantLifecycle"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section style="margin-top:14px" aria-label="用量明细">
      <div class="status"><strong>用量明细</strong><span id="usageCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>指标</th><th>状态</th><th>用量</th><th>剩余</th><th>告警</th><th>更新</th></tr></thead>
        <tbody id="usageMetrics"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="logfilterbar" aria-label="告警处置">
      <label>告警状态
        <select id="alertStatus">
          <option value="open">打开</option>
          <option value="resolved">已解决</option>
          <option value="all">全部</option>
        </select>
      </label>
      <label>告警指标
        <input id="alertMetric" placeholder="users">
      </label>
      <label>告警类型
        <input id="alertType" value="quota_exceeded" placeholder="quota_exceeded">
      </label>
      <button id="applyAlertFilters" type="button">查告警</button>
      <button id="bulkResolveAlerts" type="button">批量解决</button>
    </section>
    <section style="margin-top:14px" aria-label="告警列表">
      <div class="status"><strong>告警处置</strong><span id="alertCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>时间</th><th>租户</th><th>指标</th><th>状态</th><th>用量</th><th>操作</th></tr></thead>
        <tbody id="alerts"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="logfilterbar" aria-label="通知送达健康度筛选">
      <label>统计窗口
        <select id="notificationHealthWindowHours">
          <option value="1">最近 1 小时</option>
          <option value="6">最近 6 小时</option>
          <option value="24" selected>最近 24 小时</option>
          <option value="72">最近 3 天</option>
          <option value="168">最近 7 天</option>
          <option value="720">最近 30 天</option>
        </select>
      </label>
      <label>健康状态
        <select id="notificationHealthState">
          <option value="all">全部</option>
          <option value="critical">严重</option>
          <option value="warning">预警</option>
          <option value="healthy">健康</option>
          <option value="no_data">无数据</option>
        </select>
      </label>
      <label>积压阈值（分钟）
        <input id="notificationHealthStaleMinutes" type="number" min="1" max="10080" value="15">
      </label>
      <label>租户搜索
        <input id="notificationHealthKeyword" placeholder="租户名称、ID 或套餐">
      </label>
      <label>异常负责人
        <input id="notificationHealthAssignOwner" placeholder="负责人">
      </label>
      <label>复查时间
        <input id="notificationHealthAssignNextAt" placeholder="2026-07-15 10:00:00">
      </label>
      <button id="loadNotificationHealth" type="button">刷新健康度</button>
      <button id="exportNotificationHealth" type="button">导出健康度 CSV</button>
      <button id="assignNotificationHealthQueue" type="button">分派异常待办</button>
      <button id="recoverNotificationHealthQueue" type="button">结案已恢复待办</button>
    </section>
    <section style="margin-top:14px" aria-label="通知送达健康度">
      <div class="status"><strong>通知送达健康度</strong><span id="notificationHealthCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>健康</th><th>租户</th><th>策略</th><th>成功率</th><th>状态分布</th><th>待投递</th><th>延迟</th><th>最近事件</th></tr></thead>
        <tbody id="notificationHealthTenants"><tr><td colspan="8" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section style="margin-top:14px" aria-label="通知失败原因">
      <div class="status"><strong>Top 失败原因</strong><span id="notificationFailureReasonCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>失败原因</th><th>通知数</th><th>租户数</th><th>最近发生</th></tr></thead>
        <tbody id="notificationFailureReasons"><tr><td colspan="4" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="logfilterbar notification-slo-bar" aria-label="通知 SLO 筛选">
      <label>统计周期
        <select id="notificationSloWindowDays">
          <option value="7" selected>最近 7 天</option>
          <option value="30">最近 30 天</option>
          <option value="90">最近 90 天</option>
        </select>
      </label>
      <label>成功率目标（%）
        <input id="notificationSloSuccessTarget" type="number" min="50" max="100" step="0.1" value="95">
      </label>
      <label>送达时延（秒）
        <input id="notificationSloLatencySeconds" type="number" min="1" max="86400" value="300">
      </label>
      <label>时延达标率（%）
        <input id="notificationSloLatencyTarget" type="number" min="50" max="100" step="0.1" value="95">
      </label>
      <label>租户搜索
        <input id="notificationSloKeyword" placeholder="租户名称、ID 或套餐">
      </label>
      <button id="loadNotificationSlo" type="button">刷新 SLO</button>
      <button id="exportNotificationSlo" type="button">导出 SLO CSV</button>
    </section>
    <section style="margin-top:14px" aria-label="通知 SLO 日趋势">
      <div class="status"><strong>通知 SLO 日趋势</strong><span id="notificationSloCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>日期</th><th>状态</th><th>送达成功率</th><th>时延达标率</th><th>结果分布</th><th>送达时延</th></tr></thead>
        <tbody id="notificationSloDays"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section style="margin-top:14px" aria-label="通知 SLO 租户排行">
      <div class="status"><strong>通知 SLO 租户排行</strong><span id="notificationSloTenantCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>状态</th><th>租户</th><th>送达成功率</th><th>时延达标率</th><th>结果分布</th><th>待处理</th><th>送达时延</th></tr></thead>
        <tbody id="notificationSloTenants"><tr><td colspan="7" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="notificationpolicybar" aria-label="租户通知策略筛选">
      <label>策略状态
        <select id="notificationPolicyState">
          <option value="all">全部</option>
          <option value="enabled">已启用</option>
          <option value="disabled">已停用</option>
          <option value="unconfigured">未配置</option>
        </select>
      </label>
      <label>策略搜索
        <input id="notificationPolicyKeyword" placeholder="租户名称、ID 或套餐">
      </label>
      <div><strong>出站防护</strong><br><span id="notificationPolicySecurity" class="muted">等待加载</span></div>
      <button id="loadNotificationPolicies" type="button">查策略</button>
    </section>
    <section class="notificationcredentialbar" aria-label="通知凭据保护">
      <div><strong>凭据保护</strong><br><span id="notificationCredentialProtection" class="muted">等待加载</span></div>
      <label>轮换租户 ID
        <input id="notificationCredentialTenantId" type="number" min="0" value="0" inputmode="numeric">
      </label>
      <label>轮换批次
        <input id="notificationCredentialLimit" type="number" min="1" max="1000" value="100" inputmode="numeric">
      </label>
      <button id="rotateNotificationCredentials" type="button" disabled>轮换凭据</button>
    </section>
    <section style="margin-top:14px" aria-label="租户通知策略">
      <div class="status"><strong>租户通知策略</strong><span id="notificationPolicyCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>租户</th><th>套餐</th><th>策略</th><th>Webhook</th><th>更新</th><th>操作</th></tr></thead>
        <tbody id="notificationPolicies"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="notificationpolicyedit" aria-label="通知策略编辑">
      <label>策略租户 ID
        <input id="notificationPolicyTenantId" inputmode="numeric" placeholder="选择租户">
      </label>
      <label><input id="notificationPolicyEnabled" type="checkbox"> 启用 Webhook</label>
      <label class="wide">Webhook URL
        <input id="notificationPolicyWebhookUrl" placeholder="https://example.com/webhook">
      </label>
      <label>Webhook Secret
        <input id="notificationPolicyWebhookSecret" type="password" autocomplete="off" placeholder="留空保持不变">
      </label>
      <label><input id="notificationPolicyClearSecret" type="checkbox"> 清空 Secret</label>
      <label>请求超时（秒）
        <input id="notificationPolicyTimeout" type="number" min="1" max="300" value="5">
      </label>
      <label>HTTP 尝试次数
        <input id="notificationPolicyHttpAttempts" type="number" min="1" max="10" value="1">
      </label>
      <label>HTTP 间隔（毫秒）
        <input id="notificationPolicyHttpDelay" type="number" min="0" max="600000" value="250">
      </label>
      <label>Outbox 尝试次数
        <input id="notificationPolicyMaxAttempts" type="number" min="1" max="20" value="3">
      </label>
      <label>Outbox 间隔（秒）
        <input id="notificationPolicyRetryDelay" type="number" min="0" max="86400" value="300">
      </label>
      <label>最低严重级别
        <select id="notificationPolicyMinimumSeverity">
          <option value="warning">Warning</option>
          <option value="critical">Critical</option>
        </select>
      </label>
      <label class="wide">通知事件
        <span class="policytypes">
          <span class="checkline"><input id="notificationPolicyAlertQuota" type="checkbox"> 额度告警</span>
          <span class="checkline"><input id="notificationPolicyAlertRenewal" type="checkbox"> 续费提醒</span>
          <span class="checkline"><input id="notificationPolicyAlertPaymentFailed" type="checkbox"> 支付失败</span>
          <span class="checkline"><input id="notificationPolicyAlertTaskSla" type="checkbox"> 任务 SLA</span>
          <span class="checkline"><input id="notificationPolicyAlertOperationQueue" type="checkbox"> 待办认领</span>
        </span>
      </label>
      <label class="wide">其他事件
        <input id="notificationPolicyAlertOther" placeholder="custom_alert, another_alert">
      </label>
      <label><span class="checkline"><input id="notificationPolicyQuietEnabled" type="checkbox"> 免打扰</span></label>
      <label>免打扰开始
        <input id="notificationPolicyQuietStart" type="time" value="22:00">
      </label>
      <label>免打扰结束
        <input id="notificationPolicyQuietEnd" type="time" value="08:00">
      </label>
      <label>时区
        <input id="notificationPolicyTimezone" list="notificationPolicyTimezoneOptions" value="Asia/Shanghai">
        <datalist id="notificationPolicyTimezoneOptions">
          <option value="Asia/Shanghai"></option>
          <option value="Asia/Hong_Kong"></option>
          <option value="UTC"></option>
          <option value="America/New_York"></option>
        </datalist>
      </label>
      <label>每小时上限
        <input id="notificationPolicyHourlyLimit" type="number" min="0" max="10000" value="0">
      </label>
      <label class="wide">标题模板
        <textarea id="notificationPolicyTitleTemplate"></textarea>
      </label>
      <label class="wide">正文模板
        <textarea id="notificationPolicyBodyTemplate"></textarea>
      </label>
      <label class="wide">操作备注
        <input id="notificationPolicyRemark" placeholder="平台代管通知策略">
      </label>
      <button id="saveNotificationPolicy" type="button">保存策略</button>
      <button id="testNotificationPolicy" type="button">生成测试通知</button>
    </section>
    <section class="logfilterbar" aria-label="通知重试">
      <label>通知状态
        <select id="notificationStatus">
          <option value="">全部</option>
          <option value="pending">待发送</option>
          <option value="failed">失败待重试</option>
          <option value="dead">已耗尽</option>
          <option value="closed">已关闭</option>
          <option value="suppressed">策略过滤</option>
          <option value="delivered">已送达</option>
        </select>
      </label>
      <label>通知通道
        <input id="notificationChannel" value="webhook" placeholder="webhook">
      </label>
      <label>通知关键字
        <input id="notificationKeyword" placeholder="错误、指标或通知 key">
      </label>
      <button id="applyNotificationFilters" type="button">查通知</button>
      <button id="bulkRetryNotifications" type="button">批量重试</button>
      <button id="bulkCloseNotifications" type="button">批量关闭</button>
    </section>
    <section style="margin-top:14px" aria-label="通知列表">
      <div class="status"><strong>通知重试</strong><span id="notificationCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>时间</th><th>租户</th><th>指标</th><th>状态</th><th>错误/下次</th><th>操作</th></tr></thead>
        <tbody id="notifications"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="grid" aria-label="租户风险与额度">
      <div>
        <div class="status"><strong>租户风险</strong><span id="tenantCount"></span></div>
        <div class="tablewrap"><table>
          <thead><tr><th>租户</th><th>套餐</th><th>到期</th><th>告警</th><th>最高用量</th><th>操作</th></tr></thead>
          <tbody id="tenants"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
        </table></div>
      </div>
      <div>
        <div class="status"><strong>额度指标</strong><span id="metricCount"></span></div>
        <div class="tablewrap"><table>
          <thead><tr><th>指标</th><th>用量</th><th>告警</th></tr></thead>
          <tbody id="metrics"><tr><td colspan="3" class="empty">暂无数据</td></tr></tbody>
        </table></div>
      </div>
    </section>
    <section class="logfilterbar" aria-label="操作与账单筛选">
      <label>操作动作
        <input id="operationAction" placeholder="tenant.renewal">
      </label>
      <label>目标类型
        <input id="operationTargetType" placeholder="tenant">
      </label>
      <label>操作关键字
        <input id="operationKeyword" placeholder="目标、备注或变更内容">
      </label>
      <label>账单类型
        <select id="billingEventType">
          <option value="">全部</option>
          <option value="renewal">续费</option>
        </select>
      </label>
      <label>账单套餐
        <select id="billingPackageCode">
          <option value="">全部套餐</option>
        </select>
      </label>
      <label>账单关键字
        <input id="billingKeyword" placeholder="订单号、支付方式或备注">
      </label>
      <label class="checkline"><input id="billingMismatchOnly" type="checkbox"> 只看对账异常</label>
      <button id="applyLogFilters" type="button">筛流水</button>
    </section>
    <section class="exportbar" aria-label="数据导出">
      <button id="exportTenants" type="button">导出租户 CSV</button>
      <button id="exportUsage" type="button">导出用量 CSV</button>
	      <button id="exportRisk" type="button">导出风险 CSV</button>
	      <button id="exportCustomerSuccess" type="button">导出客户成功 CSV</button>
	      <button id="exportCustomerSuccessOwners" type="button">导出客户成功负责人 CSV</button>
	      <button id="exportRenewalForecast" type="button">导出续费预测 CSV</button>
	      <button id="exportRenewalForecastOwners" type="button">导出续费预测负责人 CSV</button>
	      <button id="exportRiskFollowUps" type="button">导出跟进 CSV</button>
	      <button id="exportRiskFollowUpOwners" type="button">导出风险负责人 CSV</button>
	      <button id="exportDailyReport" type="button">导出日报 CSV</button>
	      <button id="exportBusinessMetrics" type="button">导出经营指标 CSV</button>
	      <button id="exportBusinessTrends" type="button">导出经营趋势 CSV</button>
	      <button id="exportOperationQueue" type="button">导出待办 CSV</button>
	      <button id="exportOperationQueueOwners" type="button">导出待办负责人 CSV</button>
	      <button id="exportOperationQueueAssignments" type="button">导出待办认领 CSV</button>
      <button id="exportTasks" type="button">导出任务 CSV</button>
      <button id="exportTaskSla" type="button">导出任务SLA CSV</button>
	      <button id="exportPackages" type="button">导出套餐 CSV</button>
      <button id="exportAlerts" type="button">导出告警 CSV</button>
      <button id="exportNotifications" type="button">导出通知 CSV</button>
	      <button id="exportOperations" type="button">导出操作 CSV</button>
	      <button id="exportBilling" type="button">导出账单 CSV</button>
	      <button id="exportBillingReconciliation" type="button">导出对账 CSV</button>
	      <button id="exportBillingFollowUps" type="button">导出账单跟进 CSV</button>
	      <button id="exportBillingFollowUpOwners" type="button">导出账单负责人 CSV</button>
	    </section>
	<section id="auditIntegrityCenter" style="margin-top:14px" aria-label="审计完整性治理" hidden>
	  <div class="status"><strong>审计完整性</strong><span id="auditIntegrityState">尚未加载</span></div>
	  <div class="auditintegritybar">
	    <label>租户 ID
	      <input id="auditIntegrityTenantId" type="number" min="0" value="0" placeholder="0 表示全部">
	    </label>
	    <label>链数量
	      <input id="auditIntegrityLimit" type="number" min="1" max="500" value="50">
	    </label>
	    <label>校验记录
	      <input id="auditIntegrityVerificationLimit" type="number" min="1" max="200" value="20">
	    </label>
	    <button id="loadAuditIntegrity" type="button" class="secondary">刷新</button>
	    <button id="verifyAuditIntegrity" type="button">立即校验</button>
	  </div>
	  <div id="auditIntegritySummary" class="summary">
	    <div class="tile"><div class="label">摘要链</div><div id="auditIntegrityChainSummary" class="value">0</div></div>
	    <div class="tile"><div class="label">结构日志</div><div id="auditIntegrityLogSummary" class="value">0</div></div>
	    <div class="tile"><div class="label">待封存租户</div><div id="auditIntegrityUnsealedSummary" class="value">0</div></div>
	    <div class="tile"><div class="label">保留期清理预估</div><div id="auditIntegrityRetentionSummary" class="value">0</div></div>
	  </div>
	  <div class="status"><strong>租户摘要链</strong><span id="auditIntegrityChainCount"></span></div>
	  <div class="tablewrap"><table>
	    <thead><tr><th>租户</th><th>状态</th><th>Legacy 锚点</th><th>当前链头</th><th>最近校验</th><th>异常</th></tr></thead>
	    <tbody id="auditIntegrityChains"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
	  </table></div>
	  <div class="status" style="margin-top:14px"><strong>校验历史</strong><span id="auditIntegrityVerificationCount"></span></div>
	  <div class="tablewrap"><table>
	    <thead><tr><th>完成时间</th><th>来源</th><th>租户</th><th>状态</th><th>校验范围</th><th>异常</th></tr></thead>
	    <tbody id="auditIntegrityVerifications"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
	  </table></div>
	  <div class="status" style="margin-top:14px"><strong>签名锚点</strong><span id="auditAnchorState">尚未加载</span></div>
	  <div class="auditintegritybar">
	    <button id="loadAuditAnchors" type="button" class="secondary">刷新锚点</button>
	    <button id="createAuditAnchor" type="button">创建锚点</button>
	    <button id="verifyAuditAnchor" type="button" class="secondary">校验锚点</button>
	  </div>
	  <div class="summary">
		    <div class="tile"><div class="label">HMAC 密钥</div><div id="auditAnchorKeySummary" class="value">-</div></div>
		    <div class="tile"><div class="label">本地独立证据</div><div id="auditAnchorArtifactSummary" class="value">0</div></div>
		    <div class="tile"><div class="label">异地 Object Lock</div><div id="auditAnchorRemoteSummary" class="value">-</div></div>
		    <div class="tile"><div class="label">远端留存</div><div id="auditAnchorRetentionSummary" class="value">-</div></div>
		    <div class="tile"><div class="label">校验结果</div><div id="auditAnchorVerifySummary" class="value">0</div></div>
		    <div class="tile"><div class="label">回退检测</div><div id="auditAnchorRollbackSummary" class="value">0</div></div>
	  </div>
	  <div class="status"><strong>签名检查点</strong><span id="auditAnchorCount"></span></div>
	  <div class="tablewrap"><table>
		    <thead><tr><th>检查点</th><th>租户</th><th>签名</th><th>链头</th><th>本地证据</th><th>异地不可变证据</th><th>校验</th><th>时间</th></tr></thead>
		    <tbody id="auditAnchors"><tr><td colspan="8" class="empty">暂无签名检查点</td></tr></tbody>
	  </table></div>
	</section>
    <section style="margin-top:14px" aria-label="操作记录">
      <div class="status"><strong>操作记录</strong><span id="operationCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>时间</th><th>动作</th><th>目标</th><th>操作人</th><th>备注</th><th>变更</th></tr></thead>
        <tbody id="operations"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section style="margin-top:14px" aria-label="账单事件">
      <div class="status"><strong>账单事件</strong><span id="billingCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>时间</th><th>租户</th><th>套餐</th><th>到期变化</th><th>金额/订单</th></tr></thead>
        <tbody id="billingEvents"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section style="margin-top:14px" aria-label="账单对账">
      <div class="status"><strong>账单对账</strong><span id="billingReconciliationCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>账单</th><th>租户</th><th>账单权益</th><th>当前权益</th><th>状态</th><th>操作</th></tr></thead>
        <tbody id="billingReconciliation"><tr><td colspan="6" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section class="risktaskbar" aria-label="账单跟进任务筛选">
      <label>跟进状态
        <select id="billingFollowStatus">
          <option value="">全部</option>
          <option value="pending">待跟进</option>
          <option value="contacted">已联系</option>
          <option value="renewal_pending">续费中</option>
          <option value="resolved">已解决</option>
          <option value="ignored">忽略</option>
        </select>
      </label>
      <label>到期状态
        <select id="billingFollowDueState">
          <option value="all">全部</option>
          <option value="overdue">已逾期</option>
          <option value="due_soon">7 天内</option>
          <option value="future">未来</option>
          <option value="no_date">无日期</option>
          <option value="closed">已关闭</option>
        </select>
      </label>
      <label>负责人
        <input id="billingFollowOwner" placeholder="负责人">
      </label>
      <label>任务关键字
        <input id="billingFollowKeyword" placeholder="订单、租户、备注或负责人">
      </label>
      <button id="applyBillingFollowFilters" type="button">筛账单跟进</button>
      <button id="bulkCloseBillingFollowUps" type="button">批量关闭账单跟进</button>
    </section>
    <section style="margin-top:14px" aria-label="账单跟进任务">
      <div class="status"><strong>账单跟进任务</strong><span id="billingFollowUpCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>账单</th><th>租户</th><th>状态</th><th>负责人</th><th>下次跟进</th><th>备注</th><th>记录</th></tr></thead>
        <tbody id="billingFollowUps"><tr><td colspan="7" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
    <section style="margin-top:14px" aria-label="账单跟进负责人工作台">
      <div class="status"><strong>账单跟进负责人工作台</strong><span id="billingFollowOwnerCount"></span></div>
      <div class="tablewrap"><table>
        <thead><tr><th>负责人</th><th>打开任务</th><th>状态分布</th><th>到期分布</th><th>最近/下次</th></tr></thead>
        <tbody id="billingFollowUpOwners"><tr><td colspan="5" class="empty">暂无数据</td></tr></tbody>
      </table></div>
    </section>
  </main>
	  <script>
	    const tokenInput = document.getElementById('token');
	    const workspaceNavEl = document.getElementById('workspaceNav');
	    const workspaceTabsEl = workspaceNavEl.querySelector('.workspace-tabs');
	    const workspaceSectionJumpInput = document.getElementById('workspaceSectionJump');
	    const workspaceTabEls = [...document.querySelectorAll('.workspace-tab[data-workspace]')];
	    const scopeInput = document.getElementById('scope');
    const tenantInput = document.getElementById('tenantId');
    const expiringInput = document.getElementById('expiringDays');
    const riskHighUsageInput = document.getElementById('riskHighUsageRatio');
	    const filterKeywordInput = document.getElementById('filterKeyword');
	    const filterPackageCodeInput = document.getElementById('filterPackageCode');
	    const filterTenantStatusInput = document.getElementById('filterTenantStatus');
	    const filterDueStateInput = document.getElementById('filterDueState');
	    const accessProfileStateEl = document.getElementById('accessProfileState');
	    const accessPermissionSummaryEl = document.getElementById('accessPermissionSummary');
	    const accessGovernanceEl = document.getElementById('accessGovernance');
	    const accessGovernanceStateEl = document.getElementById('accessGovernanceState');
	    const accessRoleIdInput = document.getElementById('accessRoleId');
	    const accessRoleCodeInput = document.getElementById('accessRoleCode');
	    const accessRoleNameInput = document.getElementById('accessRoleName');
	    const accessRoleStatusInput = document.getElementById('accessRoleStatus');
	    const accessRoleDescriptionInput = document.getElementById('accessRoleDescription');
	    const accessRoleVersionInput = document.getElementById('accessRoleVersion');
	    const accessPermissionOptionsEl = document.getElementById('accessPermissionOptions');
	    const accessRolesEl = document.getElementById('accessRoles');
		    const accessAssignmentKeywordInput = document.getElementById('accessAssignmentKeyword');
		    const accessAssignmentsEl = document.getElementById('accessAssignments');
		    const brandAppTitleEl = document.getElementById('brandAppTitle');
		    const brandingCenterEl = document.getElementById('brandingCenter');
		    const brandingStateEl = document.getElementById('brandingState');
		    const brandingTenantFilterInput = document.getElementById('brandingTenantFilter');
		    const brandingStatusFilterInput = document.getElementById('brandingStatusFilter');
		    const brandingKeywordInput = document.getElementById('brandingKeyword');
		    const brandingProfilesEl = document.getElementById('brandingProfiles');
		    const brandingEditorEl = document.getElementById('brandingEditor');
		    const brandingTenantIdInput = document.getElementById('brandingTenantId');
		    const brandingStatusInput = document.getElementById('brandingStatus');
		    const brandingProductNameInput = document.getElementById('brandingProductName');
		    const brandingProductShortNameInput = document.getElementById('brandingProductShortName');
		    const brandingProductSubtitleInput = document.getElementById('brandingProductSubtitle');
		    const brandingLogoUrlInput = document.getElementById('brandingLogoUrl');
		    const brandingFaviconUrlInput = document.getElementById('brandingFaviconUrl');
		    const brandingLoginBackgroundUrlInput = document.getElementById('brandingLoginBackgroundUrl');
		    const brandingPrimaryColorInput = document.getElementById('brandingPrimaryColor');
		    const brandingAccentColorInput = document.getElementById('brandingAccentColor');
		    const brandingWebsiteUrlInput = document.getElementById('brandingWebsiteUrl');
		    const brandingSupportUrlInput = document.getElementById('brandingSupportUrl');
		    const brandingSupportQrUrlInput = document.getElementById('brandingSupportQrUrl');
		    const brandingSupportEmailInput = document.getElementById('brandingSupportEmail');
		    const brandingDocsUrlInput = document.getElementById('brandingDocsUrl');
		    const brandingFooterTextInput = document.getElementById('brandingFooterText');
		    const brandingVersionInput = document.getElementById('brandingVersion');
		    const brandingConfiguredInput = document.getElementById('brandingConfigured');
		    const brandingPreviewEl = document.getElementById('brandingPreview');
		    const brandingPreviewLogoEl = document.getElementById('brandingPreviewLogo');
		    const brandingPreviewNameEl = document.getElementById('brandingPreviewName');
		    const brandingPreviewSubtitleEl = document.getElementById('brandingPreviewSubtitle');
		    const brandingPreviewButtonEl = document.getElementById('brandingPreviewButton');
		    const brandingLicenseEl = document.getElementById('brandingLicense');
		    const tenantDomainCenterEl = document.getElementById('tenantDomainCenter');
		    const tenantDomainStateEl = document.getElementById('tenantDomainState');
		    const tenantDomainTenantFilterInput = document.getElementById('tenantDomainTenantFilter');
		    const tenantDomainStatusFilterInput = document.getElementById('tenantDomainStatusFilter');
		    const tenantDomainKeywordInput = document.getElementById('tenantDomainKeyword');
		    const tenantDomainCreateTenantIdInput = document.getElementById('tenantDomainCreateTenantId');
		    const tenantDomainCreateHostnameInput = document.getElementById('tenantDomainCreateHostname');
		    const tenantDomainsEl = document.getElementById('tenantDomains');
		    const tenantDomainDeliveryStateEl = document.getElementById('tenantDomainDeliveryState');
		    const tenantDomainDeliveryStatusInput = document.getElementById('tenantDomainDeliveryStatus');
		    const tenantDomainDeliveryJobsEl = document.getElementById('tenantDomainDeliveryJobs');
		    const tenantDomainActionDialogEl = document.getElementById('tenantDomainActionDialog');
		    const tenantDomainActionTitleEl = document.getElementById('tenantDomainActionTitle');
		    const tenantDomainActionMessageEl = document.getElementById('tenantDomainActionMessage');
		    const releaseReadinessCenterEl = document.getElementById('releaseReadinessCenter');
		    const releaseReadinessStateEl = document.getElementById('releaseReadinessState');
		    const releaseVersionInput = document.getElementById('releaseVersion');
		    const releaseSourceFingerprintInput = document.getElementById('releaseSourceFingerprint');
		    const releaseReadinessSummaryEl = document.getElementById('releaseReadinessSummary');
		    const releaseEvidenceActionCountEl = document.getElementById('releaseEvidenceActionCount');
		    const releaseEvidenceActionsEl = document.getElementById('releaseEvidenceActions');
		    const releaseEvidenceCountEl = document.getElementById('releaseEvidenceCount');
		    const releaseEvidenceEl = document.getElementById('releaseEvidence');
		    const releaseCandidateCountEl = document.getElementById('releaseCandidateCount');
		    const releaseCandidatesEl = document.getElementById('releaseCandidates');
		    const releaseEvidenceDialogEl = document.getElementById('releaseEvidenceDialog');
		    const releaseEvidenceTitleEl = document.getElementById('releaseEvidenceTitle');
		    const releaseEvidenceKeyInput = document.getElementById('releaseEvidenceKey');
		    const releaseEvidenceVersionInput = document.getElementById('releaseEvidenceVersion');
		    const releaseEvidenceStatusInput = document.getElementById('releaseEvidenceStatus');
		    const releaseEvidenceEnvironmentInput = document.getElementById('releaseEvidenceEnvironment');
		    const releaseEvidenceUrlInput = document.getElementById('releaseEvidenceUrl');
		    const releaseEvidenceFingerprintInput = document.getElementById('releaseEvidenceFingerprint');
		    const releaseEvidenceArtifactFingerprintInput = document.getElementById('releaseEvidenceArtifactFingerprint');
		    const releaseEvidenceArtifactSizeInput = document.getElementById('releaseEvidenceArtifactSize');
		    const releaseEvidenceNoteInput = document.getElementById('releaseEvidenceNote');
		    const releaseEvidenceActionDialogEl = document.getElementById('releaseEvidenceActionDialog');
		    const releaseEvidenceActionTitleEl = document.getElementById('releaseEvidenceActionTitle');
		    const releaseEvidenceActionKeyInput = document.getElementById('releaseEvidenceActionKey');
		    const releaseEvidenceActionVersionInput = document.getElementById('releaseEvidenceActionVersion');
		    const releaseEvidenceActionOwnerInput = document.getElementById('releaseEvidenceActionOwner');
		    const releaseEvidenceActionDueAtInput = document.getElementById('releaseEvidenceActionDueAt');
		    const releaseEvidenceActionNextInput = document.getElementById('releaseEvidenceActionNext');
		    const releaseEvidenceActionNoteInput = document.getElementById('releaseEvidenceActionNote');
		    const identitySecurityCenterEl = document.getElementById('identitySecurityCenter');
	    const identitySecurityStateEl = document.getElementById('identitySecurityState');
	    const identityTenantFilterInput = document.getElementById('identityTenantFilter');
	    const identityUserFilterInput = document.getElementById('identityUserFilter');
	    const identitySessionStatusInput = document.getElementById('identitySessionStatus');
	    const identityEventRiskInput = document.getElementById('identityEventRisk');
	    const identityKeywordInput = document.getElementById('identityKeyword');
	    const identitySecuritySummaryEl = document.getElementById('identitySecuritySummary');
	    const identityPolicyEditorEl = document.getElementById('identityPolicyEditor');
	    const identityPolicyStatusInput = document.getElementById('identityPolicyStatus');
	    const identityMaxFailedAttemptsInput = document.getElementById('identityMaxFailedAttempts');
	    const identityLockoutMinutesInput = document.getElementById('identityLockoutMinutes');
	    const identitySessionTtlMinutesInput = document.getElementById('identitySessionTtlMinutes');
	    const identityIdleTimeoutMinutesInput = document.getElementById('identityIdleTimeoutMinutes');
	    const identityMaxConcurrentSessionsInput = document.getElementById('identityMaxConcurrentSessions');
	    const identityLoginEventRetentionDaysInput = document.getElementById('identityLoginEventRetentionDays');
	    const identitySessionRetentionDaysInput = document.getElementById('identitySessionRetentionDays');
	    const identityRequireMfaInput = document.getElementById('identityRequireMfa');
	    const identityAllowedIpCidrsInput = document.getElementById('identityAllowedIpCidrs');
	    const identityPolicyVersionInput = document.getElementById('identityPolicyVersion');
	    const identityUserCountEl = document.getElementById('identityUserCount');
	    const identityUsersEl = document.getElementById('identityUsers');
	    const identitySessionCountEl = document.getElementById('identitySessionCount');
	    const identitySessionsEl = document.getElementById('identitySessions');
	    const identityIncidentCountEl = document.getElementById('identityIncidentCount');
	    const identityIncidentsEl = document.getElementById('identityIncidents');
	    const identityEventCountEl = document.getElementById('identityEventCount');
	    const identityEventsEl = document.getElementById('identityEvents');
	    const systemHealthCenterEl = document.getElementById('systemHealthCenter');
	    const systemHealthStateEl = document.getElementById('systemHealthState');
	    const systemHealthFailureWindowInput = document.getElementById('systemHealthFailureWindow');
	    const systemHealthNotificationStaleInput = document.getElementById('systemHealthNotificationStale');
	    const systemIncidentStatusInput = document.getElementById('systemIncidentStatus');
	    const systemIncidentSeverityInput = document.getElementById('systemIncidentSeverity');
	    const systemIncidentOwnerInput = document.getElementById('systemIncidentOwner');
	    const systemIncidentKeywordInput = document.getElementById('systemIncidentKeyword');
	    const systemHealthSummaryEl = document.getElementById('systemHealthSummary');
	    const systemHealthCheckCountEl = document.getElementById('systemHealthCheckCount');
	    const systemHealthChecksEl = document.getElementById('systemHealthChecks');
	    const systemIncidentCountEl = document.getElementById('systemIncidentCount');
	    const systemIncidentsEl = document.getElementById('systemIncidents');
	    const systemHealthScanCountEl = document.getElementById('systemHealthScanCount');
	    const systemHealthScansEl = document.getElementById('systemHealthScans');
	    const backupCenterEl = document.getElementById('backupCenter');
	    const backupStateEl = document.getElementById('backupState');
	    const backupConfigStateEl = document.getElementById('backupConfigState');
	    const backupSummaryEl = document.getElementById('backupSummary');
	    const backupPolicyEditorEl = document.getElementById('backupPolicyEditor');
	    const backupPolicyStatusInput = document.getElementById('backupPolicyStatus');
	    const backupIntervalMinutesInput = document.getElementById('backupIntervalMinutes');
	    const backupRetentionDaysInput = document.getElementById('backupRetentionDays');
	    const backupMinSuccessfulInput = document.getElementById('backupMinSuccessful');
	    const backupMaxAgeMinutesInput = document.getElementById('backupMaxAgeMinutes');
	    const backupDrillIntervalDaysInput = document.getElementById('backupDrillIntervalDays');
	    const backupRequireEncryptionInput = document.getElementById('backupRequireEncryption');
	    const backupRequireOffsiteReplicaInput = document.getElementById('backupRequireOffsiteReplica');
	    const backupPolicyVersionInput = document.getElementById('backupPolicyVersion');
	    const backupRunCountEl = document.getElementById('backupRunCount');
	    const backupRunsEl = document.getElementById('backupRuns');
	    const backupCleanupRunCountEl = document.getElementById('backupCleanupRunCount');
	    const backupCleanupRunsEl = document.getElementById('backupCleanupRuns');
	    const restoreDrillCountEl = document.getElementById('restoreDrillCount');
	    const restoreDrillsEl = document.getElementById('restoreDrills');
	    const backupRestoreDialogEl = document.getElementById('backupRestoreDialog');
	    const backupRestoreMessageEl = document.getElementById('backupRestoreMessage');
	    const complianceCenterEl = document.getElementById('complianceCenter');
	    const complianceStateEl = document.getElementById('complianceState');
	    const complianceTenantFilterInput = document.getElementById('complianceTenantFilter');
	    const complianceConfigStateEl = document.getElementById('complianceConfigState');
	    const complianceInventoryStateEl = document.getElementById('complianceInventoryState');
	    const complianceSummaryEl = document.getElementById('complianceSummary');
	    const compliancePolicyEditorEl = document.getElementById('compliancePolicyEditor');
	    const compliancePolicyStatusInput = document.getElementById('compliancePolicyStatus');
	    const complianceExportRetentionDaysInput = document.getElementById('complianceExportRetentionDays');
	    const complianceErasureGraceDaysInput = document.getElementById('complianceErasureGraceDays');
	    const complianceRecentExportDaysInput = document.getElementById('complianceRecentExportDays');
	    const complianceBillingRetentionDaysInput = document.getElementById('complianceBillingRetentionDays');
	    const complianceAuditRetentionDaysInput = document.getElementById('complianceAuditRetentionDays');
	    const complianceServiceAccountUsageRetentionDaysInput = document.getElementById('complianceServiceAccountUsageRetentionDays');
	    const complianceRequireRecentExportInput = document.getElementById('complianceRequireRecentExport');
	    const compliancePolicyVersionInput = document.getElementById('compliancePolicyVersion');
	    const complianceActionsEl = document.getElementById('complianceActions');
	    const complianceActionTenantIdInput = document.getElementById('complianceActionTenantId');
	    const complianceActionReasonInput = document.getElementById('complianceActionReason');
	    const complianceHoldStartsAtInput = document.getElementById('complianceHoldStartsAt');
	    const complianceHoldExpiresAtInput = document.getElementById('complianceHoldExpiresAt');
	    const complianceErasureConfirmationInput = document.getElementById('complianceErasureConfirmation');
	    const complianceHoldCountEl = document.getElementById('complianceHoldCount');
	    const complianceHoldsEl = document.getElementById('complianceHolds');
	    const complianceExportCountEl = document.getElementById('complianceExportCount');
	    const complianceExportsEl = document.getElementById('complianceExports');
	    const complianceErasureCountEl = document.getElementById('complianceErasureCount');
	    const complianceErasuresEl = document.getElementById('complianceErasures');
	    const complianceStepsDialogEl = document.getElementById('complianceStepsDialog');
	    const complianceStepsStateEl = document.getElementById('complianceStepsState');
	    const complianceStepsEl = document.getElementById('complianceSteps');
	    const weComCredentialCenterEl = document.getElementById('weComCredentialCenter');
	    const weComCredentialStateEl = document.getElementById('weComCredentialState');
	    const weComCredentialTenantIdInput = document.getElementById('weComCredentialTenantId');
	    const weComCredentialLimitInput = document.getElementById('weComCredentialLimit');
	    const weComCredentialSummaryEl = document.getElementById('weComCredentialSummary');
	    const rotateWeComCredentialsButton = document.getElementById('rotateWeComCredentials');
	    const weChatOpenCredentialCenterEl = document.getElementById('weChatOpenCredentialCenter');
	    const weChatOpenCredentialStateEl = document.getElementById('weChatOpenCredentialState');
	    const weChatOpenCredentialTenantIdInput = document.getElementById('weChatOpenCredentialTenantId');
	    const weChatOpenCredentialLimitInput = document.getElementById('weChatOpenCredentialLimit');
	    const weChatOpenCredentialSummaryEl = document.getElementById('weChatOpenCredentialSummary');
	    const rotateWeChatOpenCredentialsButton = document.getElementById('rotateWeChatOpenCredentials');
	    const serviceAccountCenterEl = document.getElementById('serviceAccountCenter');
	    const serviceAccountStateEl = document.getElementById('serviceAccountState');
	    const serviceAccountTenantFilterInput = document.getElementById('serviceAccountTenantFilter');
	    const serviceAccountStatusFilterInput = document.getElementById('serviceAccountStatusFilter');
	    const serviceAccountKeywordInput = document.getElementById('serviceAccountKeyword');
	    const serviceAccountSummaryEl = document.getElementById('serviceAccountSummary');
	    const serviceAccountUsageStateEl = document.getElementById('serviceAccountUsageState');
	    const serviceAccountUsageSegmentsEl = document.getElementById('serviceAccountUsageSegments');
	    const serviceAccountUsageAccountFilterInput = document.getElementById('serviceAccountUsageAccountFilter');
	    const serviceAccountUsageSummaryEl = document.getElementById('serviceAccountUsageSummary');
	    const serviceAccountUsageChartEl = document.getElementById('serviceAccountUsageChart');
	    const serviceAccountUsageRouteCountEl = document.getElementById('serviceAccountUsageRouteCount');
	    const serviceAccountUsageRoutesEl = document.getElementById('serviceAccountUsageRoutes');
	    const serviceAccountUsageAccountCountEl = document.getElementById('serviceAccountUsageAccountCount');
	    const serviceAccountUsageAccountsEl = document.getElementById('serviceAccountUsageAccounts');
	    const serviceAccountSecretEl = document.getElementById('serviceAccountSecret');
	    const serviceAccountSecretStateEl = document.getElementById('serviceAccountSecretState');
	    const serviceAccountPlainTextKeyInput = document.getElementById('serviceAccountPlainTextKey');
	    const serviceAccountEditorEl = document.getElementById('serviceAccountEditor');
	    const serviceAccountIdInput = document.getElementById('serviceAccountId');
	    const serviceAccountVersionInput = document.getElementById('serviceAccountVersion');
	    const serviceAccountTenantIdInput = document.getElementById('serviceAccountTenantId');
	    const serviceAccountCodeInput = document.getElementById('serviceAccountCode');
	    const serviceAccountNameInput = document.getElementById('serviceAccountName');
	    const serviceAccountStatusInput = document.getElementById('serviceAccountStatus');
	    const serviceAccountRateLimitPerMinuteInput = document.getElementById('serviceAccountRateLimitPerMinute');
	    const serviceAccountDailyRequestLimitInput = document.getElementById('serviceAccountDailyRequestLimit');
	    const serviceAccountUsageAlertEnabledInput = document.getElementById('serviceAccountUsageAlertEnabled');
	    const serviceAccountUsageWarningPercentInput = document.getElementById('serviceAccountUsageWarningPercent');
	    const serviceAccountRejectionWarningCountInput = document.getElementById('serviceAccountRejectionWarningCount');
	    const serviceAccountUsageAlertCooldownMinutesInput = document.getElementById('serviceAccountUsageAlertCooldownMinutes');
	    const serviceAccountDescriptionInput = document.getElementById('serviceAccountDescription');
	    const serviceAccountExpiresAtInput = document.getElementById('serviceAccountExpiresAt');
	    const serviceAccountAllowedCidrsInput = document.getElementById('serviceAccountAllowedCidrs');
	    const serviceAccountInitialKeyNameInput = document.getElementById('serviceAccountInitialKeyName');
	    const serviceAccountInitialKeyExpiresAtInput = document.getElementById('serviceAccountInitialKeyExpiresAt');
	    const serviceAccountScopesEl = document.getElementById('serviceAccountScopes');
	    const serviceAccountKeyEditorEl = document.getElementById('serviceAccountKeyEditor');
	    const serviceAccountRotateAccountInput = document.getElementById('serviceAccountRotateAccount');
	    const serviceAccountRotateNameInput = document.getElementById('serviceAccountRotateName');
	    const serviceAccountRotateExpiresAtInput = document.getElementById('serviceAccountRotateExpiresAt');
	    const serviceAccountRotateGraceInput = document.getElementById('serviceAccountRotateGrace');
	    const serviceAccountRotateIdInput = document.getElementById('serviceAccountRotateId');
	    const serviceAccountRotateVersionInput = document.getElementById('serviceAccountRotateVersion');
	    const serviceAccountCountEl = document.getElementById('serviceAccountCount');
	    const serviceAccountsEl = document.getElementById('serviceAccounts');
	    const serviceAccountRevokeDialogEl = document.getElementById('serviceAccountRevokeDialog');
	    const serviceAccountRevokeMessageEl = document.getElementById('serviceAccountRevokeMessage');
	    const serviceAccountRevokeReasonInput = document.getElementById('serviceAccountRevokeReason');
	    const approvalCenterEl = document.getElementById('approvalCenter');
	    const approvalStateEl = document.getElementById('approvalState');
	    const approvalStatusInput = document.getElementById('approvalStatus');
	    const approvalActionInput = document.getElementById('approvalAction');
	    const approvalRiskInput = document.getElementById('approvalRisk');
	    const approvalKeywordInput = document.getElementById('approvalKeyword');
	    const approvalSummaryEl = document.getElementById('approvalSummary');
	    const approvalPoliciesEl = document.getElementById('approvalPolicies');
	    const approvalsEl = document.getElementById('approvals');
	    const approvalEventsEl = document.getElementById('approvalEvents');
	    const approvalEventStateEl = document.getElementById('approvalEventState');
	    const approvalDecisionsEl = document.getElementById('approvalDecisions');
	    const approvalDecisionStateEl = document.getElementById('approvalDecisionState');
	    const approvalDelegationsEl = document.getElementById('approvalDelegations');
	    const approvalDelegationStateEl = document.getElementById('approvalDelegationState');
	    const approvalDelegationIdInput = document.getElementById('approvalDelegationId');
	    const approvalDelegationVersionInput = document.getElementById('approvalDelegationVersion');
	    const approvalDelegatorUserIdInput = document.getElementById('approvalDelegatorUserId');
	    const approvalDelegateUserIdInput = document.getElementById('approvalDelegateUserId');
	    const approvalDelegationStartsAtInput = document.getElementById('approvalDelegationStartsAt');
	    const approvalDelegationEndsAtInput = document.getElementById('approvalDelegationEndsAt');
	    const approvalDelegationStatusInput = document.getElementById('approvalDelegationStatus');
	    const approvalDelegationReasonInput = document.getElementById('approvalDelegationReason');
	    const adminTaskTypeInput = document.getElementById('adminTaskType');
	    const adminTaskStatusInput = document.getElementById('adminTaskStatus');
	    const adminTaskTenantInput = document.getElementById('adminTaskTenantId');
	    const adminTaskPackageCodeInput = document.getElementById('adminTaskPackageCode');
	    const adminTaskSlaWarningInput = document.getElementById('adminTaskSlaWarningHours');
	    const adminTaskSlaOverdueInput = document.getElementById('adminTaskSlaOverdueHours');
	    const adminTaskSummaryEl = document.getElementById('adminTaskSummary');
	    const adminTasksEl = document.getElementById('adminTasks');
	    const adminTaskOwnerSummaryEl = document.getElementById('adminTaskOwnerSummary');
	    const adminTaskOwnersEl = document.getElementById('adminTaskOwners');
	    const adminTaskSlaSummaryEl = document.getElementById('adminTaskSlaSummary');
	    const adminTaskSlaEl = document.getElementById('adminTaskSla');
	    const tenantReadinessCenterEl = document.getElementById('tenantReadinessCenter');
	    const tenantReadinessStateEl = document.getElementById('tenantReadinessState');
	    const tenantReadinessFilterStateInput = document.getElementById('tenantReadinessFilterState');
	    const tenantReadinessTenantIdInput = document.getElementById('tenantReadinessTenantId');
	    const tenantReadinessKeywordInput = document.getElementById('tenantReadinessKeyword');
	    const tenantReadinessSummaryEl = document.getElementById('tenantReadinessSummary');
	    const tenantReadinessTenantsEl = document.getElementById('tenantReadinessTenants');
	    const provisionTenantNameInput = document.getElementById('provisionTenantName');
    const provisionAdminPhoneInput = document.getElementById('provisionAdminPhone');
    const provisionAdminNameInput = document.getElementById('provisionAdminName');
	const provisionPasswordInput = document.getElementById('provisionPassword');
	const provisionPackageCodeInput = document.getElementById('provisionPackageCode');
	const provisionExpiresInput = document.getElementById('provisionExpiresAt');
	const provisionTaskIdInput = document.getElementById('provisionTaskId');
	const provisionTaskSummaryEl = document.getElementById('provisionTaskSummary');
	const provisionTasksEl = document.getElementById('provisionTasks');
	const packageTenantInput = document.getElementById('packageTenantId');
    const packageCodeInput = document.getElementById('packageCode');
    const packageExpiresInput = document.getElementById('packageExpiresAt');
    const packageAssignmentVersionInput = document.getElementById('packageAssignmentVersion');
    const packageAssignmentRemarkInput = document.getElementById('packageAssignmentRemark');
    const editPackageCodeInput = document.getElementById('editPackageCode');
    const editPackageNameInput = document.getElementById('editPackageName');
    const editPackageDescriptionInput = document.getElementById('editPackageDescription');
    const editPackageVersionInput = document.getElementById('editPackageVersion');
    const editPackageStatusInput = document.getElementById('editPackageStatus');
    const editPackageLimitsInput = document.getElementById('editPackageLimits');
    const packageImpactSummaryEl = document.getElementById('packageImpactSummary');
    const packageImpactChangesEl = document.getElementById('packageImpactChanges');
    const syncPackageCodeInput = document.getElementById('syncPackageCode');
    const syncTenantIdInput = document.getElementById('syncTenantId');
    const syncLimitInput = document.getElementById('syncLimit');
    const syncAllowOverLimitInput = document.getElementById('syncAllowOverLimit');
    const syncTaskIdInput = document.getElementById('syncTaskId');
    const packageSyncSummaryEl = document.getElementById('packageSyncSummary');
    const packageSyncTenantsEl = document.getElementById('packageSyncTenants');
    const packageSyncTasksEl = document.getElementById('packageSyncTasks');
    const statusTenantInput = document.getElementById('statusTenantId');
    const tenantStatusInput = document.getElementById('tenantStatus');
    const tenantStatusRemarkInput = document.getElementById('tenantStatusRemark');
    const tenantStatusButton = document.getElementById('updateTenantStatus');
    const subscriptionStatusInput = document.getElementById('subscriptionStatus');
    const subscriptionAccessInput = document.getElementById('subscriptionAccess');
    const subscriptionKeywordInput = document.getElementById('subscriptionKeyword');
    const subscriptionSummaryEl = document.getElementById('subscriptionSummary');
    const subscriptionsEl = document.getElementById('subscriptions');
    const subscriptionCountEl = document.getElementById('subscriptionCount');
    const subscriptionEventsEl = document.getElementById('subscriptionEvents');
    const subscriptionEventCountEl = document.getElementById('subscriptionEventCount');
    const subscriptionTenantInput = document.getElementById('subscriptionTenantId');
    const subscriptionTransitionStatusInput = document.getElementById('subscriptionTransitionStatus');
    const subscriptionVersionInput = document.getElementById('subscriptionVersion');
    const subscriptionTrialEndsInput = document.getElementById('subscriptionTrialEndsAt');
    const subscriptionPeriodEndsInput = document.getElementById('subscriptionPeriodEndsAt');
    const subscriptionGraceEndsInput = document.getElementById('subscriptionGraceEndsAt');
    const subscriptionCancelAtPeriodEndInput = document.getElementById('subscriptionCancelAtPeriodEnd');
    const subscriptionReasonInput = document.getElementById('subscriptionReason');
    const paymentOrderStatusInput = document.getElementById('paymentOrderStatus');
    const paymentProviderInput = document.getElementById('paymentProvider');
    const paymentKeywordInput = document.getElementById('paymentKeyword');
    const paymentSummaryEl = document.getElementById('paymentSummary');
    const paymentOrdersEl = document.getElementById('paymentOrders');
    const paymentOrderCountEl = document.getElementById('paymentOrderCount');
    const paymentWebhookEventsEl = document.getElementById('paymentWebhookEvents');
    const paymentWebhookEventCountEl = document.getElementById('paymentWebhookEventCount');
    const paymentTenantInput = document.getElementById('paymentTenantId');
    const paymentCreateProviderInput = document.getElementById('paymentCreateProvider');
    const paymentPackageCodeInput = document.getElementById('paymentPackageCode');
    const paymentBillingCycleInput = document.getElementById('paymentBillingCycle');
    const paymentServiceExpiresInput = document.getElementById('paymentServiceExpiresAt');
    const paymentAmountInput = document.getElementById('paymentAmount');
    const paymentCurrencyInput = document.getElementById('paymentCurrency');
    const paymentMaxDunningAttemptsInput = document.getElementById('paymentMaxDunningAttempts');
    const paymentCheckoutUrlInput = document.getElementById('paymentCheckoutUrl');
    const paymentCheckoutExpiresInput = document.getElementById('paymentCheckoutExpiresAt');
    const paymentOrderNoInput = document.getElementById('paymentOrderNo');
    const paymentIdempotencyKeyInput = document.getElementById('paymentIdempotencyKey');
    const paymentRemarkInput = document.getElementById('paymentRemark');
	const paymentRefundStatusInput = document.getElementById('paymentRefundStatus');
	const paymentRefundOrderFilterInput = document.getElementById('paymentRefundOrderFilter');
	const paymentRefundKeywordInput = document.getElementById('paymentRefundKeyword');
	const paymentRefundSummaryEl = document.getElementById('paymentRefundSummary');
	const paymentRefundsEl = document.getElementById('paymentRefunds');
	const paymentRefundCountEl = document.getElementById('paymentRefundCount');
	const paymentRefundOrderInput = document.getElementById('paymentRefundOrderNo');
	const paymentRefundAmountInput = document.getElementById('paymentRefundAmount');
	const paymentRefundCurrencyInput = document.getElementById('paymentRefundCurrency');
	const paymentRefundEntitlementInput = document.getElementById('paymentRefundEntitlement');
	const paymentProviderRefundNoInput = document.getElementById('paymentProviderRefundNo');
	const paymentRefundNoInput = document.getElementById('paymentRefundNo');
	const paymentRefundIdempotencyInput = document.getElementById('paymentRefundIdempotencyKey');
		const paymentRefundReasonInput = document.getElementById('paymentRefundReason');
		const paymentRefundRemarkInput = document.getElementById('paymentRefundRemark');
		const invoiceKindFilterInput = document.getElementById('invoiceKindFilter');
		const invoiceStatusFilterInput = document.getElementById('invoiceStatusFilter');
		const invoiceTenantFilterInput = document.getElementById('invoiceTenantFilter');
		const invoiceKeywordFilterInput = document.getElementById('invoiceKeywordFilter');
		const invoiceDocumentSummaryEl = document.getElementById('invoiceDocumentSummary');
		const invoiceDocumentsEl = document.getElementById('invoiceDocuments');
		const invoiceDocumentCountEl = document.getElementById('invoiceDocumentCount');
		const invoiceProfileTenantInput = document.getElementById('invoiceProfileTenantId');
		const invoiceProfileTypeInput = document.getElementById('invoiceProfileType');
		const invoiceProfileTitleInput = document.getElementById('invoiceProfileTitle');
		const invoiceProfileTaxIDInput = document.getElementById('invoiceProfileTaxId');
		const invoiceProfileEmailInput = document.getElementById('invoiceProfileEmail');
		const invoiceProfilePhoneInput = document.getElementById('invoiceProfilePhone');
		const invoiceProfileAddressInput = document.getElementById('invoiceProfileAddress');
		const invoiceProfileBankNameInput = document.getElementById('invoiceProfileBankName');
		const invoiceProfileBankAccountInput = document.getElementById('invoiceProfileBankAccount');
		const invoiceProfileRecipientInput = document.getElementById('invoiceProfileRecipient');
		const invoiceProfileVersionInput = document.getElementById('invoiceProfileVersion');
		const invoiceProfileRemarkInput = document.getElementById('invoiceProfileRemark');
		const invoiceCreateKindInput = document.getElementById('invoiceCreateKind');
		const invoiceCreateTenantInput = document.getElementById('invoiceCreateTenantId');
		const invoiceCreateOrderInput = document.getElementById('invoiceCreateOrderNo');
		const invoiceOriginalDocumentInput = document.getElementById('invoiceOriginalDocumentNo');
		const invoiceCreateAmountInput = document.getElementById('invoiceCreateAmount');
		const invoiceCreateCurrencyInput = document.getElementById('invoiceCreateCurrency');
		const invoiceCreateProviderInput = document.getElementById('invoiceCreateProvider');
		const invoiceCreateDocumentInput = document.getElementById('invoiceCreateDocumentNo');
		const invoiceCreateIdempotencyInput = document.getElementById('invoiceCreateIdempotencyKey');
		const invoiceCreateRemarkInput = document.getElementById('invoiceCreateRemark');
		const invoiceTransitionDocumentInput = document.getElementById('invoiceTransitionDocumentNo');
		const invoiceTransitionVersionInput = document.getElementById('invoiceTransitionVersion');
		const invoiceTransitionStatusInput = document.getElementById('invoiceTransitionStatus');
		const invoiceTransitionProviderInput = document.getElementById('invoiceTransitionProvider');
		const invoiceProviderDocumentInput = document.getElementById('invoiceProviderDocumentNo');
		const invoiceDocumentURLInput = document.getElementById('invoiceDocumentUrl');
		const invoiceIssuedAtInput = document.getElementById('invoiceIssuedAt');
		const invoiceFailureCodeInput = document.getElementById('invoiceFailureCode');
			const invoiceFailureMessageInput = document.getElementById('invoiceFailureMessage');
			const invoiceTransitionRemarkInput = document.getElementById('invoiceTransitionRemark');
			const paymentSettlementSyncProviderInput = document.getElementById('paymentSettlementSyncProvider');
			const paymentSettlementSyncSourceInput = document.getElementById('paymentSettlementSyncSource');
			const paymentSettlementSyncStatusInput = document.getElementById('paymentSettlementSyncStatus');
			const paymentSettlementSyncSummaryEl = document.getElementById('paymentSettlementSyncSummary');
			const paymentSettlementSyncStatesEl = document.getElementById('paymentSettlementSyncStates');
			const paymentSettlementSyncRunsEl = document.getElementById('paymentSettlementSyncRuns');
			const paymentSettlementSyncStateEl = document.getElementById('paymentSettlementSyncState');
			const previewPaymentSettlementSyncButton = document.getElementById('previewPaymentSettlementSync');
			const runPaymentSettlementSyncButton = document.getElementById('runPaymentSettlementSync');
			const paymentSettlementProviderInput = document.getElementById('paymentSettlementProvider');
		const paymentSettlementStatusInput = document.getElementById('paymentSettlementStatus');
		const paymentSettlementKeywordInput = document.getElementById('paymentSettlementKeyword');
		const paymentSettlementSummaryEl = document.getElementById('paymentSettlementSummary');
		const paymentSettlementBatchesEl = document.getElementById('paymentSettlementBatches');
		const paymentSettlementBatchCountEl = document.getElementById('paymentSettlementBatchCount');
		const paymentSettlementFileInput = document.getElementById('paymentSettlementFile');
		const paymentSettlementCSVInput = document.getElementById('paymentSettlementCSV');
		const paymentSettlementImportStateInput = document.getElementById('paymentSettlementImportState');
		const paymentSettlementSelectedBatchInput = document.getElementById('paymentSettlementSelectedBatch');
		const paymentSettlementReconciliationStatusInput = document.getElementById('paymentSettlementReconciliationStatus');
		const paymentSettlementHandlingStatusInput = document.getElementById('paymentSettlementHandlingStatus');
		const paymentSettlementEntryKeywordInput = document.getElementById('paymentSettlementEntryKeyword');
		const paymentSettlementEntriesEl = document.getElementById('paymentSettlementEntries');
		const paymentSettlementEntryCountEl = document.getElementById('paymentSettlementEntryCount');
		let selectedPaymentSettlementVersion = 0;
	    const renewalTenantInput = document.getElementById('renewalTenantId');
    const renewalPackageCodeInput = document.getElementById('renewalPackageCode');
	    const renewalExpiresInput = document.getElementById('renewalExpiresAt');
	    const renewalAmountInput = document.getElementById('renewalAmount');
	    const renewalOrderInput = document.getElementById('renewalOrderNo');
	    const renewalTaskIdInput = document.getElementById('renewalTaskId');
	    const renewalTaskSummaryEl = document.getElementById('renewalTaskSummary');
	    const renewalTasksEl = document.getElementById('renewalTasks');
	    const dailyReportDateInput = document.getElementById('dailyReportDate');
	    const dailyReportDaysInput = document.getElementById('dailyReportDays');
	    const riskFollowStatusInput = document.getElementById('riskFollowStatus');
	    const riskFollowOwnerInput = document.getElementById('riskFollowOwner');
    const riskFollowNextAtInput = document.getElementById('riskFollowNextAt');
    const riskFollowRemarkInput = document.getElementById('riskFollowRemark');
    const riskTaskStatusInput = document.getElementById('riskTaskStatus');
    const riskTaskDueStateInput = document.getElementById('riskTaskDueState');
    const riskTaskOwnerInput = document.getElementById('riskTaskOwner');
    const riskTaskKeywordInput = document.getElementById('riskTaskKeyword');
    const operationActionInput = document.getElementById('operationAction');
    const operationTargetTypeInput = document.getElementById('operationTargetType');
    const operationKeywordInput = document.getElementById('operationKeyword');
    const alertStatusInput = document.getElementById('alertStatus');
    const alertMetricInput = document.getElementById('alertMetric');
    const alertTypeInput = document.getElementById('alertType');
    const notificationHealthWindowInput = document.getElementById('notificationHealthWindowHours');
    const notificationHealthStateInput = document.getElementById('notificationHealthState');
    const notificationHealthStaleInput = document.getElementById('notificationHealthStaleMinutes');
    const notificationHealthKeywordInput = document.getElementById('notificationHealthKeyword');
    const notificationHealthAssignOwnerInput = document.getElementById('notificationHealthAssignOwner');
    const notificationHealthAssignNextAtInput = document.getElementById('notificationHealthAssignNextAt');
    const notificationSloDaysInput = document.getElementById('notificationSloWindowDays');
    const notificationSloSuccessTargetInput = document.getElementById('notificationSloSuccessTarget');
    const notificationSloLatencySecondsInput = document.getElementById('notificationSloLatencySeconds');
    const notificationSloLatencyTargetInput = document.getElementById('notificationSloLatencyTarget');
    const notificationSloKeywordInput = document.getElementById('notificationSloKeyword');
    const notificationPolicyStateInput = document.getElementById('notificationPolicyState');
    const notificationPolicyKeywordInput = document.getElementById('notificationPolicyKeyword');
    const notificationPolicyTenantInput = document.getElementById('notificationPolicyTenantId');
    const notificationPolicyEnabledInput = document.getElementById('notificationPolicyEnabled');
    const notificationPolicyWebhookUrlInput = document.getElementById('notificationPolicyWebhookUrl');
    const notificationPolicyWebhookSecretInput = document.getElementById('notificationPolicyWebhookSecret');
    const notificationPolicyClearSecretInput = document.getElementById('notificationPolicyClearSecret');
    const notificationPolicyTimeoutInput = document.getElementById('notificationPolicyTimeout');
    const notificationPolicyHttpAttemptsInput = document.getElementById('notificationPolicyHttpAttempts');
    const notificationPolicyHttpDelayInput = document.getElementById('notificationPolicyHttpDelay');
    const notificationPolicyMaxAttemptsInput = document.getElementById('notificationPolicyMaxAttempts');
    const notificationPolicyRetryDelayInput = document.getElementById('notificationPolicyRetryDelay');
    const notificationPolicyMinimumSeverityInput = document.getElementById('notificationPolicyMinimumSeverity');
    const notificationPolicyAlertQuotaInput = document.getElementById('notificationPolicyAlertQuota');
    const notificationPolicyAlertRenewalInput = document.getElementById('notificationPolicyAlertRenewal');
    const notificationPolicyAlertPaymentFailedInput = document.getElementById('notificationPolicyAlertPaymentFailed');
    const notificationPolicyAlertTaskSlaInput = document.getElementById('notificationPolicyAlertTaskSla');
    const notificationPolicyAlertOperationQueueInput = document.getElementById('notificationPolicyAlertOperationQueue');
    const notificationPolicyAlertOtherInput = document.getElementById('notificationPolicyAlertOther');
    const notificationPolicyQuietEnabledInput = document.getElementById('notificationPolicyQuietEnabled');
    const notificationPolicyQuietStartInput = document.getElementById('notificationPolicyQuietStart');
    const notificationPolicyQuietEndInput = document.getElementById('notificationPolicyQuietEnd');
    const notificationPolicyTimezoneInput = document.getElementById('notificationPolicyTimezone');
    const notificationPolicyHourlyLimitInput = document.getElementById('notificationPolicyHourlyLimit');
    const notificationPolicyTitleTemplateInput = document.getElementById('notificationPolicyTitleTemplate');
    const notificationPolicyBodyTemplateInput = document.getElementById('notificationPolicyBodyTemplate');
    const notificationPolicyRemarkInput = document.getElementById('notificationPolicyRemark');
    const notificationStatusInput = document.getElementById('notificationStatus');
    const notificationChannelInput = document.getElementById('notificationChannel');
    const notificationKeywordInput = document.getElementById('notificationKeyword');
	    const billingEventTypeInput = document.getElementById('billingEventType');
	    const billingPackageCodeInput = document.getElementById('billingPackageCode');
	    const billingKeywordInput = document.getElementById('billingKeyword');
	    const billingMismatchOnlyInput = document.getElementById('billingMismatchOnly');
	    const billingFollowStatusInput = document.getElementById('billingFollowStatus');
	    const billingFollowDueStateInput = document.getElementById('billingFollowDueState');
	    const billingFollowOwnerInput = document.getElementById('billingFollowOwner');
	    const billingFollowKeywordInput = document.getElementById('billingFollowKeyword');
	    const statusEl = document.getElementById('status');
	    const summaryEl = document.getElementById('summary');
	    const businessMetricsEl = document.getElementById('businessMetrics');
	    const businessMetricsHintEl = document.getElementById('businessMetricsHint');
	    const businessPackagesEl = document.getElementById('businessPackages');
	const businessTrendsEl = document.getElementById('businessTrends');
	const businessTrendsHintEl = document.getElementById('businessTrendsHint');
	const businessTrendMonthsEl = document.getElementById('businessTrendMonths');
	const businessRenewalFunnelEl = document.getElementById('businessRenewalFunnel');
	const operationQueueSourceInput = document.getElementById('operationQueueSource');
	const operationQueuePriorityInput = document.getElementById('operationQueuePriority');
	const operationQueueOwnerInput = document.getElementById('operationQueueOwner');
	const operationQueueAssignmentDueStateInput = document.getElementById('operationQueueAssignmentDueState');
	const operationQueueAssignmentCurrentOnlyInput = document.getElementById('operationQueueAssignmentCurrentOnly');
	const operationQueueKeywordInput = document.getElementById('operationQueueKeyword');
	const operationQueueAssignOwnerInput = document.getElementById('operationQueueAssignOwner');
	const operationQueueAssignStatusInput = document.getElementById('operationQueueAssignStatus');
	const operationQueueAssignNextAtInput = document.getElementById('operationQueueAssignNextAt');
	const operationQueueAssignRemarkInput = document.getElementById('operationQueueAssignRemark');
	const operationQueueEl = document.getElementById('operationQueue');
	const operationQueueCountEl = document.getElementById('operationQueueCount');
	const operationQueueOwnersEl = document.getElementById('operationQueueOwners');
	const operationQueueOwnerCountEl = document.getElementById('operationQueueOwnerCount');
	const operationQueueAssignmentsEl = document.getElementById('operationQueueAssignments');
	const operationQueueAssignmentCountEl = document.getElementById('operationQueueAssignmentCount');
	const renewalForecastEl = document.getElementById('renewalForecast');
	const renewalForecastHintEl = document.getElementById('renewalForecastHint');
	const renewalForecastBucketsEl = document.getElementById('renewalForecastBuckets');
	const renewalForecastOwnersEl = document.getElementById('renewalForecastOwners');
	const renewalForecastTenantsEl = document.getElementById('renewalForecastTenants');
	const renewalForecastDaysInput = document.getElementById('renewalForecastDays');
	const renewalForecastBucketInput = document.getElementById('renewalForecastBucket');
	const renewalForecastPricedInput = document.getElementById('renewalForecastPriced');
	const renewalForecastPackageCodeInput = document.getElementById('renewalForecastPackageCode');
	const renewalForecastOwnerInput = document.getElementById('renewalForecastOwner');
	const renewalForecastTaskStatusInput = document.getElementById('renewalForecastTaskStatus');
	const renewalForecastAssignOwnerInput = document.getElementById('renewalForecastAssignOwner');
	const renewalForecastAssignNextAtInput = document.getElementById('renewalForecastAssignNextAt');
	const renewalForecastAssignStatusInput = document.getElementById('renewalForecastAssignStatus');
	const renewalForecastAssignRemarkInput = document.getElementById('renewalForecastAssignRemark');
	const renewalForecastTaskMonthsInput = document.getElementById('renewalForecastTaskMonths');
	const renewalForecastTaskOrderPrefixInput = document.getElementById('renewalForecastTaskOrderPrefix');
	const renewalForecastTaskForceInput = document.getElementById('renewalForecastTaskForce');
	const renewalForecastReminderDaysInput = document.getElementById('renewalForecastReminderDays');
	const renewalForecastMaxAttemptsInput = document.getElementById('renewalForecastMaxAttempts');
	const renewalForecastNotifyRemarkInput = document.getElementById('renewalForecastNotifyRemark');
	const renewalForecastForceNotifyInput = document.getElementById('renewalForecastForceNotify');
	const dailyReportEl = document.getElementById('dailyReport');
	    const dailyReportHintEl = document.getElementById('dailyReportHint');
	    const dailyReportOwnersEl = document.getElementById('dailyReportOwners');
	    const dailyReportTaskSlaEl = document.getElementById('dailyReportTaskSla');
	    const dailyReportNotificationsEl = document.getElementById('dailyReportNotifications');
	    const dailyReportQueueAssignmentsEl = document.getElementById('dailyReportQueueAssignments');
	    const dailyReportActionsEl = document.getElementById('dailyReportActions');
	    const dailyReportBillingEl = document.getElementById('dailyReportBilling');
	const tenantDetailEl = document.getElementById('tenantDetail');
	    const tenantDetailHintEl = document.getElementById('tenantDetailHint');
	    const tenantLifecycleEl = document.getElementById('tenantLifecycle');
		    const tenantLifecycleCountEl = document.getElementById('tenantLifecycleCount');
	    const tenantLifecycleSourceInput = document.getElementById('tenantLifecycleSource');
	    const tenantLifecycleEventTypeInput = document.getElementById('tenantLifecycleEventType');
	    const tenantLifecycleStatusInput = document.getElementById('tenantLifecycleStatus');
	    const tenantLifecycleKeywordInput = document.getElementById('tenantLifecycleKeyword');
		    const customerSuccessEl = document.getElementById('customerSuccess');
	    const customerSuccessOwnersEl = document.getElementById('customerSuccessOwners');
	    const customerSuccessCountEl = document.getElementById('customerSuccessCount');
	    const customerSuccessOwnerCountEl = document.getElementById('customerSuccessOwnerCount');
	    const customerSuccessPriorityInput = document.getElementById('customerSuccessPriority');
    const customerSuccessOwnerInput = document.getElementById('customerSuccessOwner');
    const customerSuccessTenantLimitInput = document.getElementById('customerSuccessTenantLimit');
    const customerSuccessAssignOwnerInput = document.getElementById('customerSuccessAssignOwner');
    const customerSuccessAssignNextAtInput = document.getElementById('customerSuccessAssignNextAt');
    const customerSuccessAssignRemarkInput = document.getElementById('customerSuccessAssignRemark');
    const customerSuccessRenewMonthsInput = document.getElementById('customerSuccessRenewMonths');
    const customerSuccessRenewAmountInput = document.getElementById('customerSuccessRenewAmount');
    const customerSuccessRenewRemarkInput = document.getElementById('customerSuccessRenewRemark');
    const customerSuccessRenewalReminderDaysInput = document.getElementById('customerSuccessRenewalReminderDays');
    const customerSuccessRenewalMaxAttemptsInput = document.getElementById('customerSuccessRenewalMaxAttempts');
    const customerSuccessRenewalNotifyRemarkInput = document.getElementById('customerSuccessRenewalNotifyRemark');
    const customerSuccessRenewalForceNotifyInput = document.getElementById('customerSuccessRenewalForceNotify');
    const riskTenantsEl = document.getElementById('riskTenants');
    const riskFollowUpsEl = document.getElementById('riskFollowUps');
    const riskFollowUpOwnersEl = document.getElementById('riskFollowUpOwners');
    const tenantsEl = document.getElementById('tenants');
    const metricsEl = document.getElementById('metrics');
    const usageMetricsEl = document.getElementById('usageMetrics');
    const alertsEl = document.getElementById('alerts');
    const notificationHealthTenantsEl = document.getElementById('notificationHealthTenants');
    const notificationHealthCountEl = document.getElementById('notificationHealthCount');
    const notificationFailureReasonsEl = document.getElementById('notificationFailureReasons');
    const notificationFailureReasonCountEl = document.getElementById('notificationFailureReasonCount');
    const notificationSloDaysEl = document.getElementById('notificationSloDays');
    const notificationSloTenantsEl = document.getElementById('notificationSloTenants');
    const notificationSloCountEl = document.getElementById('notificationSloCount');
    const notificationSloTenantCountEl = document.getElementById('notificationSloTenantCount');
    const notificationPoliciesEl = document.getElementById('notificationPolicies');
    const notificationPolicyCountEl = document.getElementById('notificationPolicyCount');
    const notificationPolicySecurityEl = document.getElementById('notificationPolicySecurity');
    const notificationCredentialProtectionEl = document.getElementById('notificationCredentialProtection');
    const notificationCredentialTenantInput = document.getElementById('notificationCredentialTenantId');
    const notificationCredentialLimitInput = document.getElementById('notificationCredentialLimit');
    const rotateNotificationCredentialsButton = document.getElementById('rotateNotificationCredentials');
    const notificationsEl = document.getElementById('notifications');
	const auditIntegrityCenterEl = document.getElementById('auditIntegrityCenter');
	const auditIntegrityStateEl = document.getElementById('auditIntegrityState');
	const auditIntegrityTenantIdInput = document.getElementById('auditIntegrityTenantId');
	const auditIntegrityLimitInput = document.getElementById('auditIntegrityLimit');
	const auditIntegrityVerificationLimitInput = document.getElementById('auditIntegrityVerificationLimit');
	const auditIntegrityChainsEl = document.getElementById('auditIntegrityChains');
	const auditIntegrityVerificationsEl = document.getElementById('auditIntegrityVerifications');
	const auditAnchorStateEl = document.getElementById('auditAnchorState');
	const auditAnchorsEl = document.getElementById('auditAnchors');
    const operationsEl = document.getElementById('operations');
    const billingEventsEl = document.getElementById('billingEvents');
    const billingReconciliationEl = document.getElementById('billingReconciliation');
    const billingFollowUpsEl = document.getElementById('billingFollowUps');
    const billingFollowUpOwnersEl = document.getElementById('billingFollowUpOwners');
	    function normalizeStoredToken(value) {
	      let raw = String(value || '').trim();
	      if (!raw) return '';
	      try {
	        const parsed = JSON.parse(raw);
	        if (typeof parsed === 'string') raw = parsed.trim();
	      } catch (_) {}
	      return raw;
	    }
	    const savedToken = normalizeStoredToken(localStorage.getItem('mochat_go_saas_admin_token') || localStorage.getItem('ACCESS_TOKEN'));
	    let packageCache = {};
	    let tenantOverviewCache = {};
	    let notificationPolicyLoadedTenantId = 0;
	    let platformAccessControlled = false;
	    let notificationCredentialProtection = {};
	    let currentPlatformPermissions = [];
		    let accessPermissionCatalog = [];
		    let accessRoleCache = [];
		    let brandingProfileCache = [];
		    let tenantDomainCache = [];
		    let tenantDomainDeliveryJobCache = [];
		    let pendingTenantDomainAction = null;
		    let releaseEvidenceCache = [];
		    let releaseEvidenceActionCache = [];
		    let releaseEvidenceOwnerCache = [];
		    let releaseGateEnabled = false;
		    let releaseGateDisabledReason = '发布门禁尚未就绪';
	    let identityPolicyCache = {};
	    let identityUserCache = [];
	    let currentPlatformTenantID = 0;
	    let backupRunCache = [];
	    let backupConfigCache = {};
	    let backupPolicyCache = {};
	    let pendingBackupRestore = null;
	    let compliancePolicyCache = {};
	    let complianceExportCache = [];
	    let complianceErasureCache = [];
	    let weComCredentialProtection = {};
	    let weChatOpenCredentialProtection = {};
	    let serviceAccountCache = [];
	    let serviceAccountScopeCatalog = [];
	    let serviceAccountUsageDays = 7;
	    let pendingServiceAccountRevoke = null;
	    let currentPlatformUserID = 0;
	    let approvalRequired = true;
	    let approvalPolicies = [];
	    let approvalDelegations = [];
	    let tenantReadinessCache = [];
	    let provisionTaskCache = [];
	    let renewalTaskCache = [];
	    tokenInput.value = savedToken;
		    dailyReportDateInput.value = localDateString(new Date());

	    const workspaceStorageKey = 'mochat_go_saas_admin_workspace';
	    const workspaceCategories = new Set(workspaceTabEls.map(button => button.dataset.workspace));
	    const workspacePersistentLabels = new Set(['筛选', '租户列表筛选']);
	    const workspacePermissionMap = {
	      overview: ['platform.overview.read'],
	      tenant: ['platform.tenants.read'],
	      operations: ['platform.operations.read'],
	      finance: ['platform.finance.read'],
	      notifications: ['platform.notifications.read'],
	      governance: ['platform.access.manage', 'platform.approvals.read', 'platform.audit.read'],
	      security: ['platform.system.read', 'platform.backups.read', 'platform.compliance.read', 'platform.identity.read', 'platform.integrations.read', 'platform.audit.read'],
	      delivery: ['platform.branding.read', 'platform.domains.read', 'platform.release.read'],
	    };
	    const workspaceSections = [...document.querySelectorAll('main > section')].filter(section => !workspacePersistentLabels.has(section.getAttribute('aria-label') || ''));
	    let currentWorkspace = 'overview';
	    let preferredWorkspace = 'overview';

	    function workspaceForSection(section) {
	      const label = section.getAttribute('aria-label') || '';
	      if (/品牌与白标|租户自定义域名|发布准备/.test(label)) return 'delivery';
	      if (/身份与访问安全|平台健康|备份与恢复|租户数据合规|企业微信凭据|微信开放平台凭据|服务账号|审计完整性/.test(label)) return 'security';
	      if (/平台权限治理|高风险审批|操作记录|数据导出/.test(label)) return 'governance';
	      if (/租户上线准备度|新租户开户|平台开户|租户套餐|套餐维护|套餐变更|套餐快照|套餐同步|租户状态|订阅生命周期/.test(label)) return 'tenant';
	      if (/支付收款|续费账单|续费任务|操作与账单|账单事件|账单对账|账单跟进/.test(label)) return 'finance';
	      if (/告警|通知/.test(label)) return 'notifications';
	      if (/运营任务|运营待办|续费预测|运营日报|客户成功|风险/.test(label)) return 'operations';
	      return 'overview';
	    }

	    function workspaceSectionLabel(section, index) {
	      const label = String(section.getAttribute('aria-label') || '').trim();
	      if (label) return label;
	      const heading = section.querySelector(':scope > .status strong, :scope > strong');
	      return heading && heading.textContent.trim() ? heading.textContent.trim() : '模块 ' + (index + 1);
	    }

	    function accessibleWorkspaceSections(workspace) {
	      const requiredPermissions = workspacePermissionMap[workspace] || [];
	      if (platformAccessControlled && requiredPermissions.length && !requiredPermissions.some(hasPlatformPermission)) return [];
	      return workspaceSections.filter(section => section.dataset.workspace === workspace && !section.hidden);
	    }

	    function renderWorkspaceSectionJump() {
	      const sections = accessibleWorkspaceSections(currentWorkspace);
	      workspaceSectionJumpInput.innerHTML = sections.map((section, index) =>
	        '<option value="' + esc(section.id) + '">' + esc(workspaceSectionLabel(section, index)) + '</option>'
	      ).join('');
	      workspaceSectionJumpInput.disabled = sections.length === 0;
	    }

	    function revealCurrentWorkspaceTab(smooth) {
	      const activeTab = workspaceTabEls.find(button => button.dataset.workspace === currentWorkspace && !button.hidden);
	      if (!activeTab) return;
	      const left = Math.max(0, activeTab.offsetLeft - ((workspaceTabsEl.clientWidth - activeTab.offsetWidth) / 2));
	      workspaceTabsEl.scrollTo({ left, behavior: smooth ? 'smooth' : 'auto' });
	    }

	    function applyWorkspaceView(workspace, scrollToTop, persistSelection = true) {
	      if (!workspaceCategories.has(workspace)) workspace = 'overview';
	      const availableWorkspaces = workspaceTabEls.filter(button => accessibleWorkspaceSections(button.dataset.workspace).length > 0);
	      if (!accessibleWorkspaceSections(workspace).length && availableWorkspaces.length) workspace = availableWorkspaces[0].dataset.workspace;
	      currentWorkspace = workspace;
	      workspaceSections.forEach(section => {
	        section.classList.toggle('workspace-section-hidden', section.dataset.workspace !== currentWorkspace);
	      });
	      workspaceTabEls.forEach(button => {
	        const available = accessibleWorkspaceSections(button.dataset.workspace).length > 0;
	        button.hidden = !available;
	        button.setAttribute('aria-selected', button.dataset.workspace === currentWorkspace ? 'true' : 'false');
	        button.tabIndex = button.dataset.workspace === currentWorkspace ? 0 : -1;
	      });
	      renderWorkspaceSectionJump();
	      if (persistSelection) {
	        preferredWorkspace = currentWorkspace;
	        try { localStorage.setItem(workspaceStorageKey, currentWorkspace); } catch (_) {}
	      }
	      requestAnimationFrame(() => {
	        revealCurrentWorkspaceTab(scrollToTop);
	        if (scrollToTop) workspaceNavEl.scrollIntoView({ behavior: 'smooth', block: 'start' });
	      });
	    }

	    function refreshWorkspaceNavigation() {
	      applyWorkspaceView(preferredWorkspace, false);
	    }

	    function initWorkspaceNavigation() {
	      workspaceSections.forEach((section, index) => {
	        section.dataset.workspace = workspaceForSection(section);
	        if (!section.id) section.id = 'saasWorkspaceSection' + (index + 1);
	      });
	      try {
	        const savedWorkspace = localStorage.getItem(workspaceStorageKey);
	        if (workspaceCategories.has(savedWorkspace)) preferredWorkspace = savedWorkspace;
	      } catch (_) {}
	      currentWorkspace = preferredWorkspace;
	      workspaceTabEls.forEach(button => button.addEventListener('click', () => applyWorkspaceView(button.dataset.workspace, true)));
	      workspaceTabEls.forEach(button => button.addEventListener('keydown', event => {
	        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
	        const visibleTabs = workspaceTabEls.filter(item => !item.hidden);
	        const currentIndex = visibleTabs.indexOf(button);
	        const offset = event.key === 'ArrowRight' ? 1 : -1;
	        const next = visibleTabs[(currentIndex + offset + visibleTabs.length) % visibleTabs.length];
	        if (next) { next.focus(); next.click(); }
	      }));
	      workspaceSectionJumpInput.addEventListener('change', () => {
	        const section = document.getElementById(workspaceSectionJumpInput.value);
	        if (section) section.scrollIntoView({ behavior: 'smooth', block: 'start' });
	      });
	      applyWorkspaceView(preferredWorkspace, false, false);
	    }

		    function authHeader() {
	      const raw = normalizeStoredToken(tokenInput.value);
	      if (!raw) return {};
	      return { Authorization: raw.toLowerCase().startsWith('bearer ') ? raw : 'Bearer ' + raw };
	    }
	    function hasPlatformPermission(permission) {
	      if (!platformAccessControlled) return true;
	      return currentPlatformPermissions.includes('*') || currentPlatformPermissions.includes(permission);
	    }
	    function permissionDefinition(code) {
	      return accessPermissionCatalog.find(item => item.code === code) || { code, name: code, category: '', description: '', write: false };
	    }
	    function renderAccessProfile(profile) {
	      profile = profile || {};
	      currentPlatformUserID = Number(profile.userId || 0);
	      currentPlatformPermissions = Array.isArray(profile.permissions) ? profile.permissions : [];
	      const roles = Array.isArray(profile.roles) ? profile.roles : [];
	      const roleNames = roles.map(role => role.name).filter(Boolean);
	      accessProfileStateEl.textContent = profile.isPlatformSuperAdmin ? '平台超级管理员 / 全权限' : (roleNames.join('、') || (platformAccessControlled ? '未分配平台岗位' : '租户管理员'));
	      const permissions = currentPlatformPermissions.includes('*')
	        ? accessPermissionCatalog.map(item => item.code)
	        : currentPlatformPermissions;
	      accessPermissionSummaryEl.innerHTML = permissions.length
	        ? permissions.map(code => {
	            const item = permissionDefinition(code);
	            return '<span class="access-permission' + (item.write ? ' write' : '') + '" title="' + esc(item.description) + '">' + esc(item.name) + '</span>';
	          }).join('')
	        : '<span class="muted">暂无平台权限</span>';
	      tenantReadinessCenterEl.hidden = platformAccessControlled && !hasPlatformPermission('platform.tenants.read');
		      accessGovernanceEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.access.manage');
		      brandingCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.branding.read');
		      const canManageBranding = platformAccessControlled && hasPlatformPermission('platform.branding.manage');
		      [...brandingEditorEl.querySelectorAll('input, select')].forEach(element => { element.disabled = !canManageBranding; });
		      brandingTenantIdInput.disabled = true;
		      tenantDomainCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.domains.read');
		      const canManageDomains = platformAccessControlled && hasPlatformPermission('platform.domains.manage');
		      tenantDomainCreateTenantIdInput.disabled = !canManageDomains;
		      tenantDomainCreateHostnameInput.disabled = !canManageDomains;
		      releaseReadinessCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.release.read');
		      identitySecurityCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.identity.read');
	      identityPolicyEditorEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.identity.manage');
	      systemHealthCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.system.read');
	      backupCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.backups.read');
	      backupPolicyEditorEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.backups.manage');
	      complianceCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.compliance.read');
	      compliancePolicyEditorEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.compliance.manage');
	      complianceActionsEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.compliance.manage');
	      weComCredentialCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.integrations.read');
	      weChatOpenCredentialCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.integrations.read');
	      serviceAccountCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.integrations.read');
	      serviceAccountEditorEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.integrations.manage');
	      serviceAccountKeyEditorEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.integrations.manage');
	      auditIntegrityCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.audit.read');
		      approvalCenterEl.hidden = !platformAccessControlled || !hasPlatformPermission('platform.approvals.read');
		      applyPlatformWriteControl();
		      refreshWorkspaceNavigation();
		    }
	    function applyPlatformWriteControl() {
	      const groups = {
	        'platform.tenants.manage': ['provisionTenant', 'createProvisionTask', 'applyProvisionTask', 'bulkApplyProvisionTasks', 'applyPackage', 'savePackage', 'previewPackageSync', 'applyPackageSync', 'createPackageSyncTask', 'applyPackageSyncTask', 'bulkApplyPackageSyncTasks', 'updateTenantStatus', 'renewTenant', 'createRenewalTask', 'applyRenewalTask', 'bulkApplyRenewalTasks'],
	        'platform.operations.manage': ['createTaskSlaNotifications', 'bulkCancelAdminTasks', 'bulkResetAdminTasks', 'assignOperationQueue', 'closeOperationQueueAssignment', 'notifyOperationQueueAssignment', 'assignCustomerSuccess', 'createCustomerSuccessRenewalTasks', 'createCustomerSuccessRenewalNotifications', 'saveRiskFollowUp', 'bulkCloseRiskFollowUps'],
	        'platform.notifications.manage': ['recoverNotificationHealth', 'saveNotificationPolicy', 'testNotificationPolicy', 'rotateNotificationCredentials', 'retryNotification', 'bulkRetryNotifications', 'closeNotification', 'bulkCloseNotifications', 'resolveAlert', 'bulkResolveAlerts'],
	        'platform.finance.manage': ['transitionSubscription', 'previewSubscriptionReconcile', 'applySubscriptionReconcile', 'createPaymentOrder', 'previewPaymentDunning', 'applyPaymentDunning', 'createPaymentRefund', 'saveInvoiceProfile', 'createInvoiceDocument', 'transitionInvoiceDocument', 'previewPaymentSettlementSync', 'runPaymentSettlementSync', 'importPaymentSettlement'],
		        'platform.access.manage': ['newAccessRole', 'saveAccessRole', 'loadAccessAssignments'],
		        'platform.branding.manage': ['saveBrandingProfile'],
		        'platform.domains.manage': ['createTenantDomain'],
		        'platform.release.manage': ['runReleaseGate', 'saveReleaseEvidence', 'saveReleaseEvidenceAction'],
		        'platform.identity.manage': ['saveIdentityPolicy'],
	        'platform.system.manage': ['runSystemHealthScan'],
	        'platform.backups.manage': ['createBackup', 'cleanupBackups', 'saveBackupPolicy', 'confirmBackupRestore'],
	        'platform.compliance.manage': ['saveCompliancePolicy', 'createComplianceHold', 'requestComplianceExport', 'requestComplianceErasure'],
	        'platform.integrations.manage': ['rotateWeComCredentials', 'rotateWeChatOpenCredentials', 'newServiceAccount', 'saveServiceAccount', 'rotateServiceAccountKey', 'confirmServiceAccountKeyRevoke', 'evaluateServiceAccountUsageAlerts'],
	        'platform.audit.manage': ['verifyAuditIntegrity', 'createAuditAnchor', 'verifyAuditAnchor'],
	        'platform.notifications.read': ['viewServiceAccountUsageAlerts'],
	        'platform.approvals.manage': ['createApprovalReminders', 'newApprovalDelegation', 'saveApprovalDelegation'],
	      };
	      Object.entries(groups).forEach(([permission, ids]) => {
	        ids.forEach(id => {
	          const element = document.getElementById(id);
	          if (!element) return;
	          element.disabled = !hasPlatformPermission(permission);
	          if (element.disabled) element.title = '缺少平台权限 ' + permission;
	          else element.removeAttribute('title');
	        });
	      });
	      applyNotificationCredentialRotationControl();
	      applyWeComCredentialRotationControl();
	      applyWeChatOpenCredentialRotationControl();
	      applyReleaseGateControl();
	    }
	    async function loadAccessProfile() {
	      if (!platformAccessControlled) return;
	      try {
	        const res = await fetch('/dashboard/saasAdmin/accessProfile', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        accessPermissionCatalog = Array.isArray(data.permissions) ? data.permissions : accessPermissionCatalog;
	        renderAccessProfile(data.profile || {});
	      if (hasPlatformPermission('platform.integrations.read')) {
	        void loadWeComCredentialProtection();
	        void loadWeChatOpenCredentialProtection();
	      }
		      if (hasPlatformPermission('platform.access.manage')) await loadAccessGovernance();
		      if (hasPlatformPermission('platform.branding.read')) await loadBrandingProfiles();
		      if (hasPlatformPermission('platform.domains.read')) await loadTenantDomains();
		      if (hasPlatformPermission('platform.release.read')) await loadReleaseReadiness();
		      if (hasPlatformPermission('platform.identity.read')) await loadIdentitySecurity();
	      if (hasPlatformPermission('platform.system.read')) await loadSystemHealth();
	      if (hasPlatformPermission('platform.backups.read')) await loadBackupOverview();
	      if (hasPlatformPermission('platform.compliance.read')) await loadComplianceOverview();
	      if (hasPlatformPermission('platform.integrations.read')) await loadServiceAccounts();
	        if (hasPlatformPermission('platform.approvals.read')) await loadApprovalPolicies();
	      } catch (err) {
	        accessProfileStateEl.textContent = err.message || String(err);
	      }
	    }
	    function renderAccessPermissionOptions(selected) {
	      const selectedSet = new Set(selected || []);
	      accessPermissionOptionsEl.innerHTML = accessPermissionCatalog.map(item =>
	        '<label title="' + esc(item.description) + '"><input type="checkbox" value="' + esc(item.code) + '"' + (selectedSet.has(item.code) ? ' checked' : '') + '><span><strong>' + esc(item.name) + '</strong><br><span class="muted">' + esc(item.category) + '</span></span></label>'
	      ).join('');
	    }
	    function resetAccessRoleEditor() {
	      accessRoleIdInput.value = '0';
	      accessRoleCodeInput.value = '';
	      accessRoleNameInput.value = '';
	      accessRoleDescriptionInput.value = '';
	      accessRoleStatusInput.value = '1';
	      accessRoleVersionInput.value = '0';
	      accessRoleCodeInput.disabled = false;
	      accessRoleNameInput.disabled = false;
	      accessRoleDescriptionInput.disabled = false;
	      accessRoleStatusInput.disabled = false;
	      document.getElementById('saveAccessRole').disabled = false;
	      renderAccessPermissionOptions(['platform.overview.read']);
	    }
	    function selectAccessRole(roleId) {
	      const role = accessRoleCache.find(item => Number(item.id) === Number(roleId));
	      if (!role) return;
	      accessRoleIdInput.value = String(role.id || 0);
	      accessRoleCodeInput.value = role.code || '';
	      accessRoleNameInput.value = role.name || '';
	      accessRoleDescriptionInput.value = role.description || '';
	      accessRoleStatusInput.value = String(role.status || 1);
	      accessRoleVersionInput.value = String(role.version || 0);
	      const locked = !!role.isSystem;
	      accessRoleCodeInput.disabled = locked;
	      accessRoleNameInput.disabled = locked;
	      accessRoleDescriptionInput.disabled = locked;
	      accessRoleStatusInput.disabled = locked;
	      document.getElementById('saveAccessRole').disabled = locked;
	      renderAccessPermissionOptions(role.permissions || []);
	      [...accessPermissionOptionsEl.querySelectorAll('input')].forEach(input => { input.disabled = locked; });
	      accessGovernanceStateEl.textContent = locked ? '内置岗位只读' : ('正在编辑 ' + (role.name || role.code));
	    }
	    function renderAccessRoles(items) {
	      if (!items.length) {
	        accessRolesEl.innerHTML = '<tr><td colspan="5" class="empty">暂无平台岗位</td></tr>';
	        return;
	      }
	      accessRolesEl.innerHTML = items.map(role => {
	        const permissions = (role.permissions || []).map(code => permissionDefinition(code).name).join('、');
	        return '<tr><td><strong>' + esc(role.name) + '</strong><br><span class="muted">' + esc(role.code) + (role.isSystem ? ' / 内置' : ' / 自定义') + '</span></td>' +
	          '<td>' + (Number(role.status) === 1 ? '<span class="good">启用</span>' : '<span class="bad">停用</span>') + '</td>' +
	          '<td>' + esc(permissions || '无') + '</td><td>' + fmt(role.assignmentCount || 0) + '</td>' +
	          '<td>v' + fmt(role.version || 0) + '<br><button type="button" class="secondary" data-access-role="' + fmt(role.id) + '">' + (role.isSystem ? '查看' : '编辑') + '</button></td></tr>';
	      }).join('');
	    }
	    function renderAccessAssignments(items) {
	      if (!items.length) {
	        accessAssignmentsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无平台人员</td></tr>';
	        return;
	      }
	      const activeRoles = accessRoleCache.filter(role => Number(role.status) === 1);
	      accessAssignmentsEl.innerHTML = items.map(item => {
	        const selected = new Set((item.roles || []).map(role => Number(role.id)));
	        const roleOptions = activeRoles.map(role => '<option value="' + fmt(role.id) + '"' + (selected.has(Number(role.id)) ? ' selected' : '') + '>' + esc(role.name) + '</option>').join('');
	        const permissionNames = (item.permissions || []).map(code => code === '*' ? '全权限' : permissionDefinition(code).name).join('、');
	        const roleControl = item.isSuperAdmin ? '<span class="good">平台超级管理员</span>' : '<select multiple class="access-role-select" data-access-user-roles="' + fmt(item.userId) + '">' + roleOptions + '</select>';
	        const action = item.isSuperAdmin ? '' : '<button type="button" data-access-assignment-save="' + fmt(item.userId) + '" data-version="' + fmt(item.version || 0) + '">保存授权</button>';
	        return '<tr><td><strong>' + esc(item.userName || ('用户 ' + item.userId)) + '</strong><br><span class="muted">ID ' + fmt(item.userId) + ' / ' + esc(item.phone || '-') + '</span></td>' +
	          '<td>' + (Number(item.status) === 1 ? '<span class="good">正常</span>' : '<span class="bad">停用</span>') + '</td><td>' + roleControl + '</td>' +
	          '<td>' + esc(permissionNames || '无平台权限') + '</td><td>v' + fmt(item.version || 0) + '<br>' + action + '</td></tr>';
	      }).join('');
	    }
	    async function loadAccessGovernance() {
	      if (!hasPlatformPermission('platform.access.manage')) return;
	      accessGovernanceStateEl.textContent = '正在加载';
	      const params = new URLSearchParams({ limit: '100' });
	      if (accessAssignmentKeywordInput.value.trim()) params.set('keyword', accessAssignmentKeywordInput.value.trim());
	      try {
	        const [roleRes, assignmentRes] = await Promise.all([
	          fetch('/dashboard/saasAdmin/accessRoles', { headers: authHeader() }),
	          fetch('/dashboard/saasAdmin/accessAssignments?' + params.toString(), { headers: authHeader() }),
	        ]);
	        const [roleBody, assignmentBody] = await Promise.all([roleRes.json(), assignmentRes.json()]);
	        if (!roleRes.ok || roleBody.code !== 200) throw new Error(roleBody.msg || 'HTTP ' + roleRes.status);
	        if (!assignmentRes.ok || assignmentBody.code !== 200) throw new Error(assignmentBody.msg || 'HTTP ' + assignmentRes.status);
	        accessRoleCache = (roleBody.data || {}).roles || [];
	        accessPermissionCatalog = (roleBody.data || {}).permissions || accessPermissionCatalog;
	        renderAccessRoles(accessRoleCache);
	        renderAccessAssignments((assignmentBody.data || {}).assignments || []);
	        if (!Number(accessRoleIdInput.value)) resetAccessRoleEditor();
	        accessGovernanceStateEl.textContent = fmt(accessRoleCache.length) + ' 个岗位 / ' + fmt(((assignmentBody.data || {}).assignments || []).length) + ' 个人员';
	      } catch (err) {
	        accessGovernanceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function saveAccessRole() {
	      const permissions = [...accessPermissionOptionsEl.querySelectorAll('input:checked')].map(input => input.value);
	      const payload = {
	        id: Number(accessRoleIdInput.value || 0), code: accessRoleCodeInput.value.trim(), name: accessRoleNameInput.value.trim(),
	        description: accessRoleDescriptionInput.value.trim(), status: Number(accessRoleStatusInput.value || 1),
	        expectedVersion: Number(accessRoleVersionInput.value || 0), permissions,
	      };
	      accessGovernanceStateEl.textContent = '正在保存岗位';
	      try {
	        if (approvalActionRequired('access.role.save', 0)) {
	          await requestHighRiskApproval('access.role.save', payload, '变更平台岗位：' + (payload.name || payload.code));
	          accessGovernanceStateEl.textContent = '岗位变更已提交审批';
	          return;
	        }
	        const res = await fetch('/dashboard/saasAdmin/accessRole', { method: payload.id ? 'PUT' : 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        accessGovernanceStateEl.textContent = '岗位已保存';
	        resetAccessRoleEditor();
	        await loadAccessGovernance();
	        loadOperations();
	      } catch (err) {
	        accessGovernanceStateEl.textContent = err.message || String(err);
	      }
	    }
		    async function saveAccessAssignment(userId, version) {
	      const select = accessAssignmentsEl.querySelector('[data-access-user-roles="' + userId + '"]');
	      if (!select) return;
	      const roleIds = [...select.selectedOptions].map(option => Number(option.value));
	      accessGovernanceStateEl.textContent = '正在保存人员授权';
	      try {
	        const payload = { userId: Number(userId), roleIds, expectedVersion: Number(version || 0) };
	        if (approvalActionRequired('access.assignment.save', 0)) {
	          await requestHighRiskApproval('access.assignment.save', payload, '变更平台用户 ' + userId + ' 的岗位授权');
	          accessGovernanceStateEl.textContent = '人员授权变更已提交审批';
	          return;
	        }
	        const res = await fetch('/dashboard/saasAdmin/accessAssignment', { method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        accessGovernanceStateEl.textContent = '人员授权已保存';
	        await loadAccessGovernance();
	        loadOperations();
	      } catch (err) {
	        accessGovernanceStateEl.textContent = err.message || String(err);
		      }
		    }
		    function brandingSafeAsset(value, fallback) {
		      const path = String(value || '').trim();
		      if (path === '/favicon.ico' || path.startsWith('/img/') || path.startsWith('/static/')) return path;
		      return fallback;
		    }
		    function brandingSafeColor(value, fallback) {
		      const color = String(value || '').trim().toUpperCase();
		      return /^#[0-9A-F]{6}$/.test(color) ? color : fallback;
		    }
		    function renderBrandingPreview() {
		      const primary = brandingSafeColor(brandingPrimaryColorInput.value, '#1769AA');
		      const accent = brandingSafeColor(brandingAccentColorInput.value, '#0F578F');
		      brandingPreviewNameEl.textContent = brandingProductNameInput.value.trim() || 'MoChat Go';
		      brandingPreviewSubtitleEl.textContent = brandingProductSubtitleInput.value.trim() || '企业微信客户运营平台';
		      brandingPreviewLogoEl.src = brandingSafeAsset(brandingLogoUrlInput.value, '/img/logo-no-word.c30823d0.png');
		      const background = brandingSafeAsset(brandingLoginBackgroundUrlInput.value, '/img/background.e06f03d5.png');
		      brandingPreviewEl.style.backgroundImage = 'url(' + JSON.stringify(background) + ')';
		      brandingPreviewButtonEl.style.backgroundColor = primary;
		      brandingPreviewButtonEl.style.boxShadow = 'inset 0 -2px 0 ' + accent;
		    }
		    function updatePlatformBrandTitle(profile) {
		      if (Number((profile || {}).tenantId) !== Number(currentPlatformTenantID)) return;
		      const productName = profile.status === 'active' ? String(profile.productName || '').trim() : '';
		      const baseTitle = productName || 'MoChat Go';
		      const title = /saas$/i.test(baseTitle) ? baseTitle + ' 总后台' : baseTitle + ' SaaS 总后台';
		      brandAppTitleEl.textContent = title;
		      document.title = title;
		    }
		    function fillBrandingEditor(profile) {
		      profile = profile || {};
		      brandingEditorEl.hidden = !Number(profile.tenantId || 0);
		      if (brandingEditorEl.hidden) return;
		      brandingTenantIdInput.value = String(profile.tenantId || '');
		      brandingStatusInput.value = profile.status || 'active';
		      brandingProductNameInput.value = profile.productName || 'MoChat Go';
		      brandingProductShortNameInput.value = profile.productShortName || profile.productName || 'MoChat Go';
		      brandingProductSubtitleInput.value = profile.productSubtitle || '企业微信客户运营平台';
		      brandingLogoUrlInput.value = profile.logoUrl || '/img/logo-no-word.c30823d0.png';
		      brandingFaviconUrlInput.value = profile.faviconUrl || '/favicon.ico';
		      brandingLoginBackgroundUrlInput.value = profile.loginBackgroundUrl || '/img/background.e06f03d5.png';
		      brandingPrimaryColorInput.value = brandingSafeColor(profile.primaryColor, '#1769AA');
		      brandingAccentColorInput.value = brandingSafeColor(profile.accentColor, '#0F578F');
		      brandingWebsiteUrlInput.value = profile.websiteUrl || '';
		      brandingSupportUrlInput.value = profile.supportUrl || '';
		      brandingSupportQrUrlInput.value = profile.supportQrUrl || '';
		      brandingSupportEmailInput.value = profile.supportEmail || '';
		      brandingDocsUrlInput.value = profile.docsUrl || '';
		      brandingFooterTextInput.value = profile.footerText || '';
		      brandingVersionInput.value = String(profile.version || 0);
		      brandingConfiguredInput.value = profile.configured ? '1' : '0';
		      brandingStateEl.textContent = '正在编辑 ' + (profile.tenantName || ('租户 ' + profile.tenantId)) + ' / v' + fmt(profile.version || 0);
		      renderBrandingPreview();
		      updatePlatformBrandTitle(profile);
		    }
		    function renderBrandingProfiles(items) {
		      brandingProfileCache = Array.isArray(items) ? items : [];
		      if (!brandingProfileCache.length) {
		        brandingProfilesEl.innerHTML = '<tr><td colspan="5" class="empty">暂无匹配品牌档案</td></tr>';
		        brandingEditorEl.hidden = true;
		        return;
		      }
		      brandingProfilesEl.innerHTML = brandingProfileCache.map(item => {
		        const configured = item.configured ? pill('已配置', 'ok') : pill('默认值', '');
		        const status = item.status === 'disabled' ? pill('已停用', 'danger') : pill('启用', 'ok');
		        const primary = brandingSafeColor(item.primaryColor, '#1769AA');
		        const accent = brandingSafeColor(item.accentColor, '#0F578F');
		        const swatches = '<span title="' + esc(primary) + '" style="display:inline-block;width:18px;height:18px;border:1px solid #cbd5e1;border-radius:3px;background:' + esc(primary) + '"></span> ' +
		          '<span title="' + esc(accent) + '" style="display:inline-block;width:18px;height:18px;border:1px solid #cbd5e1;border-radius:3px;background:' + esc(accent) + '"></span>';
		        return '<tr><td><strong>' + esc(item.tenantName || ('租户 ' + item.tenantId)) + '</strong><br><span class="muted">ID ' + fmt(item.tenantId) + '</span></td>' +
		          '<td><strong>' + esc(item.productName || 'MoChat Go') + '</strong><br><span class="muted">' + esc(item.productSubtitle || '-') + '</span></td>' +
		          '<td>' + status + '<br>' + configured + '</td><td>' + swatches + '</td>' +
		          '<td>v' + fmt(item.version || 0) + '<br><button type="button" class="secondary" data-branding-tenant="' + fmt(item.tenantId) + '">' + (hasPlatformPermission('platform.branding.manage') ? '编辑' : '查看') + '</button></td></tr>';
		      }).join('');
		    }
		    function selectBrandingProfile(tenantId) {
		      const profile = brandingProfileCache.find(item => Number(item.tenantId) === Number(tenantId));
		      if (profile) fillBrandingEditor(profile);
		    }
		    async function loadBrandingProfiles() {
		      if (!hasPlatformPermission('platform.branding.read')) return;
		      brandingStateEl.textContent = '正在加载';
		      const params = new URLSearchParams({ status: brandingStatusFilterInput.value || 'all', limit: '100' });
		      if (brandingTenantFilterInput.value.trim()) params.set('tenantId', brandingTenantFilterInput.value.trim());
		      if (brandingKeywordInput.value.trim()) params.set('keyword', brandingKeywordInput.value.trim());
		      const selectedTenantID = Number(brandingTenantIdInput.value || 0);
		      try {
		        const res = await fetch('/dashboard/saasAdmin/brandingProfiles?' + params.toString(), { headers: authHeader() });
		        const body = await res.json();
		        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		        const data = body.data || {};
		        renderBrandingProfiles(data.profiles || []);
		        const license = data.license || {};
		        brandingLicenseEl.textContent = [license.licenseType || 'GPL-3.0', license.licenseNote || ''].filter(Boolean).join(' / ');
		        const selected = brandingProfileCache.find(item => Number(item.tenantId) === selectedTenantID) ||
		          brandingProfileCache.find(item => Number(item.tenantId) === Number(currentPlatformTenantID)) || brandingProfileCache[0];
		        if (selected) fillBrandingEditor(selected);
		        brandingStateEl.textContent = fmt(brandingProfileCache.length) + ' 个租户品牌档案';
		      } catch (err) {
		        brandingStateEl.textContent = err.message || String(err);
		      }
		    }
		    async function saveBrandingProfile() {
		      const payload = {
		        tenantId: Number(brandingTenantIdInput.value || 0), status: brandingStatusInput.value || 'active',
		        productName: brandingProductNameInput.value.trim(), productShortName: brandingProductShortNameInput.value.trim(),
		        productSubtitle: brandingProductSubtitleInput.value.trim(), logoUrl: brandingLogoUrlInput.value.trim(),
		        faviconUrl: brandingFaviconUrlInput.value.trim(), loginBackgroundUrl: brandingLoginBackgroundUrlInput.value.trim(),
		        primaryColor: brandingPrimaryColorInput.value, accentColor: brandingAccentColorInput.value,
		        websiteUrl: brandingWebsiteUrlInput.value.trim(), supportUrl: brandingSupportUrlInput.value.trim(),
		        supportQrUrl: brandingSupportQrUrlInput.value.trim(), supportEmail: brandingSupportEmailInput.value.trim(),
		        docsUrl: brandingDocsUrlInput.value.trim(), footerText: brandingFooterTextInput.value.trim(),
		        expectedVersion: Number(brandingVersionInput.value || 0),
		      };
		      brandingStateEl.textContent = '正在保存品牌档案';
		      try {
		        const method = brandingConfiguredInput.value === '1' ? 'PUT' : 'POST';
		        const res = await fetch('/dashboard/saasAdmin/brandingProfile', { method, headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		        const body = await res.json();
		        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		        const result = body.data || {};
		        if (result.profile) fillBrandingEditor(result.profile);
		        brandingStateEl.textContent = '品牌档案已保存 / 审计 ' + fmt(result.operationId || 0);
		        await loadBrandingProfiles();
		        void loadTenantReadiness();
		        if (hasPlatformPermission('platform.audit.read')) loadOperations();
		      } catch (err) {
		        brandingStateEl.textContent = err.message || String(err);
		      }
		    }
		    function tenantDomainStatus(value) {
		      if (value === 'active') return ['已启用', 'ok'];
		      if (value === 'pending') return ['待验证', 'warning'];
		      if (value === 'disabled') return ['已停用', 'danger'];
		      return [value || '未知', ''];
		    }
		    function tenantDomainActionButton(item, action, label, style) {
		      if (action !== 'verify' && approvalActionRequired('tenant.domain.command', 0)) {
		        label = ({ set_primary: '提交主域名审批', enable: '提交启用审批', disable: '提交停用审批', rotate_token: '提交轮换审批', delete: '提交删除审批' })[action] || ('提交' + label + '审批');
		      }
		      return '<button type="button" class="' + esc(style || 'secondary') + '" data-domain-command="' + esc(action) + '" data-domain-id="' + fmt(item.id) + '" data-domain-version="' + fmt(item.version || 0) + '">' + esc(label) + '</button>';
		    }
		    function tenantDomainDeliveryStatus(value) {
		      if (value === 'ready') return ['已就绪', 'ok'];
		      if (value === 'degraded') return ['需关注', 'warning'];
		      if (value === 'pending' || value === 'provisioning') return [value === 'pending' ? '待交付' : '交付中', 'warning'];
		      if (value === 'failed') return ['交付失败', 'danger'];
		      if (value === 'disabled') return ['已停用', ''];
		      if (value === 'deleted') return ['已删除', ''];
		      return ['未配置', ''];
		    }
		    function tenantDomainDeliveryJobStatus(value) {
		      if (value === 'succeeded') return ['已完成', 'ok'];
		      if (value === 'failed') return ['失败', 'danger'];
		      if (value === 'processing' || value === 'waiting' || value === 'pending') return [{ processing: '交付中', waiting: '等待回调', pending: '待处理' }[value], 'warning'];
		      if (value === 'canceled') return ['已取消', ''];
		      return [value || '未知', ''];
		    }
		    function tenantDomainDeliveryActionLabel(value) {
		      return ({ provision: '首次下发', refresh: '刷新配置', disable: '停用', delete: '删除' })[value] || value || '-';
		    }
		    function tenantDomainDeliveryActionButton(item, action, label) {
		      return '<button type="button" class="secondary" data-domain-delivery-command="' + esc(action) + '" data-domain-id="' + fmt(item.id) + '">' + esc(label) + '</button>';
		    }
		    function renderTenantDomains(items) {
		      tenantDomainCache = Array.isArray(items) ? items : [];
		      if (!tenantDomainCache.length) {
		        tenantDomainsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无匹配租户域名</td></tr>';
		        return;
		      }
		      const canManage = hasPlatformPermission('platform.domains.manage');
		      tenantDomainsEl.innerHTML = tenantDomainCache.map(item => {
		        const state = tenantDomainStatus(item.status);
		        const primary = item.isPrimary ? '<br>' + pill('主域名', 'ok') : '';
		        const routing = item.routingActive ? '<br><span class="muted">应用 Host 已放行</span>' : '';
		        const delivery = item.delivery || {};
		        const deliveryState = tenantDomainDeliveryStatus(delivery.deliveryStatus);
		        const deliveryError = delivery.lastError ? '<span class="domain-error">' + esc(delivery.lastError) + '</span>' : '';
		        const deliveryDetail = '<div class="domain-delivery-detail">' + pill(deliveryState[0], deliveryState[1]) +
		          '<span class="muted">路由 ' + esc(delivery.routingStatus || '-') + ' / 证书 ' + esc(delivery.certificateStatus || '-') + '</span>' +
		          (delivery.provider ? '<span class="muted">' + esc(delivery.provider) + (delivery.providerRequestId ? ' / ' + esc(delivery.providerRequestId) : '') + '</span>' : '') +
		          (delivery.certificateExpiresAt ? '<span class="muted">证书到期 ' + esc(delivery.certificateExpiresAt) + '</span>' : '') + deliveryError + '</div>';
		        const dns = '<div class="dns-record">' +
		          '<div class="dns-record-row"><span class="muted">名称</span><code>' + esc(item.verificationRecordName || '') + '</code><button type="button" class="secondary" data-copy-domain="' + esc(item.verificationRecordName || '') + '">复制</button></div>' +
		          '<div class="dns-record-row"><span class="muted">值</span><code>' + esc(item.verificationRecordValue || '') + '</code><button type="button" class="secondary" data-copy-domain="' + esc(item.verificationRecordValue || '') + '">复制</button></div></div>';
		        let actions = '<span class="muted">只读</span>';
		        if (canManage) {
		          const buttons = [];
		          if (item.status === 'pending') buttons.push(tenantDomainActionButton(item, 'verify', '验证', 'secondary'));
		          if (item.status === 'active' && !item.isPrimary) buttons.push(tenantDomainActionButton(item, 'set_primary', '设为主域名', 'secondary'));
		          if (item.status === 'active') buttons.push(tenantDomainActionButton(item, 'disable', '停用', 'danger'));
		          if (item.status === 'active' && item.verifiedAt) buttons.push(tenantDomainDeliveryActionButton(item, delivery.deliveryStatus === 'unconfigured' ? 'provision' : 'refresh', delivery.deliveryStatus === 'unconfigured' ? '下发路由/TLS' : '刷新交付'));
		          if (item.status === 'disabled' && item.verifiedAt) buttons.push(tenantDomainActionButton(item, 'enable', '启用', 'secondary'));
		          if (item.status !== 'deleted') buttons.push(tenantDomainActionButton(item, 'rotate_token', '轮换令牌', 'secondary'));
		          if (item.status !== 'active') buttons.push(tenantDomainActionButton(item, 'delete', '删除', 'danger'));
		          actions = buttons.join('') || '<span class="muted">暂无操作</span>';
		        }
		        const verifyError = item.verificationError ? '<span class="domain-error">' + esc(item.verificationError) + '</span>' : '';
		        return '<tr><td><strong>' + esc(item.tenantName || ('租户 ' + item.tenantId)) + '</strong><br><span class="muted">ID ' + fmt(item.tenantId) + '</span></td>' +
		          '<td><strong>' + esc(item.hostname) + '</strong>' + primary + routing + '</td>' +
		          '<td>' + pill(state[0], state[1]) + '<br><span class="muted">v' + fmt(item.version || 0) + '</span>' + verifyError + '</td>' +
		          '<td>' + deliveryDetail + '</td><td>' + dns + '</td><td>' + esc(item.verifiedAt || item.lastVerificationAt || '-') + '</td>' +
		          '<td><div class="domain-actions">' + actions + '</div></td></tr>';
		      }).join('');
		    }
		    function renderTenantDomainDeliveryJobs(items) {
		      tenantDomainDeliveryJobCache = Array.isArray(items) ? items : [];
		      if (!tenantDomainDeliveryJobCache.length) {
		        tenantDomainDeliveryJobsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无匹配交付任务</td></tr>';
		        return;
		      }
		      const canManage = hasPlatformPermission('platform.domains.manage');
		      tenantDomainDeliveryJobsEl.innerHTML = tenantDomainDeliveryJobCache.map(item => {
		        const state = tenantDomainDeliveryJobStatus(item.status);
		        const provider = item.provider || ((item.delivery || {}).provider) || '-';
		        const requestId = item.providerRequestId || ((item.delivery || {}).providerRequestId) || '';
		        const retry = canManage && item.status === 'failed' ? '<button type="button" class="secondary" data-domain-delivery-retry="' + fmt(item.id) + '">重试</button>' : '';
		        const error = item.lastError || ((item.delivery || {}).lastError) || '';
		        return '<tr><td><strong>' + esc(item.jobNo || ('#' + item.id)) + '</strong><br><span class="muted">ID ' + fmt(item.id) + '</span></td>' +
		          '<td><strong>' + esc((item.domain || {}).hostname || '-') + '</strong><br><span class="muted">' + esc((item.domain || {}).tenantName || ('租户 ' + item.tenantId)) + ' / ID ' + fmt(item.tenantId) + '</span></td>' +
		          '<td>' + esc(tenantDomainDeliveryActionLabel(item.action)) + '</td>' +
		          '<td>' + pill(state[0], state[1]) + '<br><span class="muted">尝试 ' + fmt(item.attempts || 0) + ' / ' + fmt(item.maxAttempts || 0) + '</span></td>' +
		          '<td>' + esc(provider) + (requestId ? '<br><span class="muted">' + esc(requestId) + '</span>' : '') + '</td>' +
		          '<td><span class="muted">下次 ' + esc(item.nextAttemptAt || '-') + '</span><br><span class="muted">完成 ' + esc(item.finishedAt || '-') + '</span></td>' +
		          '<td>' + (error ? '<span class="domain-error">' + esc(error) + '</span>' : '<span class="muted">-</span>') + (retry ? '<div class="domain-actions">' + retry + '</div>' : '') + '</td></tr>';
		      }).join('');
		    }
		    async function loadTenantDomainDeliveryJobs() {
		      if (!hasPlatformPermission('platform.domains.read')) return;
		      tenantDomainDeliveryStateEl.textContent = '正在加载';
		      const params = new URLSearchParams({ status: tenantDomainDeliveryStatusInput.value || 'all', limit: '100' });
		      if (tenantDomainTenantFilterInput.value.trim()) params.set('tenantId', tenantDomainTenantFilterInput.value.trim());
		      try {
		        const res = await fetch('/dashboard/saasAdmin/tenantDomainDeliveryJobs?' + params.toString(), { headers: authHeader() });
		        const body = await res.json();
		        if (!res.ok || Number(body.code || 0) >= 400) throw new Error(body.msg || 'HTTP ' + res.status);
		        renderTenantDomainDeliveryJobs((body.data || {}).items || []);
		        tenantDomainDeliveryStateEl.textContent = fmt(tenantDomainDeliveryJobCache.length) + ' 个交付任务';
		      } catch (err) {
		        tenantDomainDeliveryStateEl.textContent = err.message || String(err);
		      }
		    }
		    async function loadTenantDomains() {
		      if (!hasPlatformPermission('platform.domains.read')) return;
		      tenantDomainStateEl.textContent = '正在加载';
		      const params = new URLSearchParams({ status: tenantDomainStatusFilterInput.value || 'all', limit: '100' });
		      if (tenantDomainTenantFilterInput.value.trim()) params.set('tenantId', tenantDomainTenantFilterInput.value.trim());
		      if (tenantDomainKeywordInput.value.trim()) params.set('keyword', tenantDomainKeywordInput.value.trim());
		      try {
		        const res = await fetch('/dashboard/saasAdmin/tenantDomains?' + params.toString(), { headers: authHeader() });
		        const body = await res.json();
		        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		        renderTenantDomains((body.data || {}).domains || []);
		        if (!tenantDomainCreateTenantIdInput.value.trim() && tenantDomainTenantFilterInput.value.trim()) tenantDomainCreateTenantIdInput.value = tenantDomainTenantFilterInput.value.trim();
		        tenantDomainStateEl.textContent = fmt(tenantDomainCache.length) + ' 个租户域名';
		        await loadTenantDomainDeliveryJobs();
		      } catch (err) {
		        tenantDomainStateEl.textContent = err.message || String(err);
		      }
		    }
		    async function applyTenantDomainDeliveryAction(payload) {
		      if (!payload) return;
		      tenantDomainDeliveryStateEl.textContent = '正在提交交付任务';
		      try {
		        const res = await fetch('/dashboard/saasAdmin/tenantDomainDelivery', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		        const body = await res.json();
		        if (!res.ok || Number(body.code || 0) >= 400) throw new Error(body.msg || 'HTTP ' + res.status);
		        await loadTenantDomains();
		        const data = body.data || {};
		        tenantDomainDeliveryStateEl.textContent = (data.reused ? '已复用正在执行的任务' : '交付任务已创建') + ' / 审计 ' + fmt(data.operationId || 0);
		        void loadTenantReadiness();
		        if (hasPlatformPermission('platform.audit.read')) loadOperations();
		      } catch (err) {
		        tenantDomainDeliveryStateEl.textContent = err.message || String(err);
		      }
		    }
		    async function createTenantDomain() {
		      const payload = {
		        action: 'create', tenantId: Number(tenantDomainCreateTenantIdInput.value || 0),
		        hostname: tenantDomainCreateHostnameInput.value.trim(),
		      };
		      const requiresApproval = approvalActionRequired('tenant.domain.create', 0);
		      tenantDomainStateEl.textContent = requiresApproval ? '正在冻结租户与域名并提交审批' : '正在添加租户域名';
		      try {
		        if (requiresApproval) {
		          const approval = await requestHighRiskApproval('tenant.domain.create', payload, '新增租户域名绑定：' + payload.hostname + ' / 租户 ' + payload.tenantId);
		          tenantDomainCreateHostnameInput.value = '';
		          tenantDomainTenantFilterInput.value = String(payload.tenantId);
		          tenantDomainStateEl.textContent = '域名创建审批已提交，等待两人复核 / ' + (approval.requestNo || '审批单');
		          return;
		        }
		        const res = await fetch('/dashboard/saasAdmin/tenantDomain', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		        const body = await res.json();
		        if (!res.ok || Number(body.code || 0) >= 400) throw new Error(body.msg || 'HTTP ' + res.status);
		        tenantDomainCreateHostnameInput.value = '';
		        tenantDomainTenantFilterInput.value = String(payload.tenantId);
		        await loadTenantDomains();
		        tenantDomainStateEl.textContent = '域名已添加，请配置 DNS TXT 后执行验证 / 审计 ' + fmt((body.data || {}).operationId || 0);
		        void loadTenantReadiness();
		        if (hasPlatformPermission('platform.audit.read')) loadOperations();
		      } catch (err) {
		        tenantDomainStateEl.textContent = err.message || String(err);
		      }
		    }
		    async function applyTenantDomainAction(action) {
		      if (!action) return;
		      const requiresApproval = action.action !== 'verify' && approvalActionRequired('tenant.domain.command', 0);
		      tenantDomainStateEl.textContent = requiresApproval ? '正在冻结租户域名路由快照并提交审批' : '正在执行域名操作';
		      try {
		        if (requiresApproval) {
		          const domain = tenantDomainCache.find(item => Number(item.id) === Number(action.id)) || {};
		          const actionLabel = ({ set_primary: '切换主域名', enable: '启用域名', disable: '停用域名', rotate_token: '轮换 DNS 校验令牌', delete: '删除域名' })[action.action] || action.action;
		          const approval = await requestHighRiskApproval('tenant.domain.command', { action: action.action, id: action.id, expectedVersion: action.version }, '变更租户域名路由：' + (domain.hostname || ('域名记录 ' + action.id)) + ' / ' + actionLabel);
		          tenantDomainStateEl.textContent = '域名变更审批已提交，等待两人复核 / ' + (approval.requestNo || '审批单');
		          renderTenantDomains(tenantDomainCache);
		          return;
		        }
		        const res = await fetch('/dashboard/saasAdmin/tenantDomain', { method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify({ action: action.action, id: action.id, expectedVersion: action.version }) });
		        const body = await res.json();
		        const failed = !res.ok || Number(body.code || 0) >= 400;
		        await loadTenantDomains();
		        if (failed) {
		          tenantDomainStateEl.textContent = body.msg || ('HTTP ' + res.status);
		          return;
		        }
		        tenantDomainStateEl.textContent = '域名操作已完成 / 审计 ' + fmt((body.data || {}).operationId || 0);
		        void loadTenantReadiness();
		        if (hasPlatformPermission('platform.audit.read')) loadOperations();
		      } catch (err) {
		        tenantDomainStateEl.textContent = err.message || String(err);
		      }
		    }
		    function requestTenantDomainAction(button) {
		      const action = { action: button.dataset.domainCommand, id: Number(button.dataset.domainId || 0), version: Number(button.dataset.domainVersion || 0) };
		      const domain = tenantDomainCache.find(item => Number(item.id) === action.id) || {};
		      const confirmations = {
		        set_primary: ['切换租户主域名', '批准并执行后，该域名会成为租户新的主登录域名。'],
		        enable: ['启用租户域名', '批准并执行后，该域名会重新进入登录路由；没有主域名时将自动成为主域名。'],
		        disable: ['停用租户域名', '停用后该域名上的登录请求会立即被拒绝。'],
		        rotate_token: ['轮换 DNS 校验令牌', '轮换后域名会立即退出路由，必须更新 DNS TXT 并重新验证。'],
		        delete: ['删除租户域名', '删除后域名绑定会释放，当前操作不可撤销。'],
		      };
		      if (!confirmations[action.action]) {
		        applyTenantDomainAction(action);
		        return;
		      }
		      pendingTenantDomainAction = action;
		      tenantDomainActionTitleEl.textContent = confirmations[action.action][0];
		      tenantDomainActionMessageEl.textContent = (domain.hostname || ('域名记录 ' + action.id)) + '：' + confirmations[action.action][1];
		      document.getElementById('confirmTenantDomainAction').textContent = approvalActionRequired('tenant.domain.command', 0) ? '提交审批' : '确认';
		      tenantDomainActionDialogEl.showModal();
		    }
		    async function copyTenantDomainValue(value) {
		      try {
		        if (!navigator.clipboard) throw new Error('clipboard unavailable');
		        await navigator.clipboard.writeText(value);
		      } catch (_) {
		        const textarea = document.createElement('textarea');
		        textarea.value = value; textarea.setAttribute('readonly', ''); textarea.style.position = 'fixed'; textarea.style.opacity = '0';
		        document.body.appendChild(textarea); textarea.select(); document.execCommand('copy'); textarea.remove();
		      }
		      tenantDomainStateEl.textContent = 'DNS TXT 内容已复制';
		    }
		    function releaseEvidenceState(value) {
		      if (value === 'passed') return ['已通过', 'ok'];
		      if (value === 'in_progress') return ['进行中', 'warning'];
		      if (value === 'failed') return ['未通过', 'danger'];
		      return ['缺失', ''];
		    }
		    function releaseEvidenceActionState(value) {
		      if (value === 'resolved') return ['已完成', 'ok'];
		      if (value === 'blocked') return ['已阻塞', 'danger'];
		      if (value === 'in_progress') return ['处理中', 'warning'];
		      return ['待分派', ''];
		    }
		    function releaseEvidenceActionDueState(value) {
		      if (value === 'resolved') return ['已完成', 'ok'];
		      if (value === 'overdue') return ['已逾期', 'danger'];
		      if (value === 'due_soon') return ['72 小时内', 'warning'];
		      if (value === 'scheduled') return ['已排期', 'ok'];
		      return ['未排期', ''];
		    }
		    function releaseCandidateState(value) {
		      if (value === 'ready') return ['可发布', 'ok'];
		      if (value === 'stale') return ['证据已变化', 'warning'];
		      return ['已阻断', 'danger'];
		    }
		    function shortFingerprint(value) {
		      value = String(value || '');
		      return value.length === 64 ? value.slice(0, 12) + '…' : (value || '-');
		    }
		    function formatArtifactSize(value) {
		      const bytes = Number(value || 0);
		      if (!Number.isFinite(bytes) || bytes <= 0) return '-';
		      const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		      let size = bytes;
		      let unit = 0;
		      while (size >= 1024 && unit < units.length - 1) { size /= 1024; unit += 1; }
		      return (unit === 0 ? Math.round(size) : size.toFixed(size >= 10 ? 1 : 2)) + ' ' + units[unit];
		    }
		    function releaseFingerprintSourceLabel(value) {
		      if (value === 'build') return '构建产物';
		      if (value === 'environment') return '运行环境';
		      if (value === 'runtime') return '运行版本';
		      return '未配置';
		    }
		    function applyReleaseGateControl() {
		      const button = document.getElementById('runReleaseGate');
		      if (!button) return;
		      const allowed = hasPlatformPermission('platform.release.manage');
		      const approvalNeeded = approvalActionRequired('release.candidate.gate', 0);
		      const canRequestApproval = hasPlatformPermission('platform.approvals.read');
		      button.disabled = !allowed || !releaseGateEnabled || (approvalNeeded && !canRequestApproval);
		      if (!allowed) button.title = '缺少平台权限 platform.release.manage';
		      else if (!releaseGateEnabled) button.title = releaseGateDisabledReason;
		      else if (approvalNeeded && !canRequestApproval) button.title = '缺少平台权限 platform.approvals.read';
		      else button.removeAttribute('title');
		    }
		    function renderReleaseReadiness(data) {
		      data = data || {};
		      const summary = data.summary || {};
		      const authoritative = summary.sourceFingerprintAuthoritative === true;
		      const targetFingerprint = String(summary.targetSourceFingerprint || '').trim().toLowerCase();
		      const verifier = summary.artifactVerifier || {};
		      releaseGateEnabled = summary.candidateGateEnabled === true;
		      releaseGateDisabledReason = !authoritative ? '当前运行版本未内置源码指纹' : (!verifier.configured ? '远端证据工件校验器未配置' : (!summary.metadataReady ? '六类生产证据尚未全部通过或源码指纹不一致' : '发布门禁尚未就绪'));
		      if (targetFingerprint && (authoritative || !releaseSourceFingerprintInput.value.trim())) releaseSourceFingerprintInput.value = targetFingerprint;
		      releaseSourceFingerprintInput.readOnly = authoritative;
		      if (authoritative) releaseSourceFingerprintInput.title = '来源：' + releaseFingerprintSourceLabel(summary.sourceFingerprintSource);
		      else releaseSourceFingerprintInput.removeAttribute('title');
		      applyReleaseGateControl();
		      releaseEvidenceCache = Array.isArray(data.evidence) ? data.evidence : [];
		      releaseEvidenceActionCache = Array.isArray(data.actions) ? data.actions : [];
		      releaseEvidenceOwnerCache = Array.isArray(data.owners) ? data.owners : [];
		      const candidates = Array.isArray(data.candidates) ? data.candidates : [];
		      const actionSummary = data.actionSummary || {};
		      const tiles = [
		        ['发布状态', summary.ready ? '候选已复核' : (summary.latestCandidateEffectiveStatus === 'stale' ? '证据变化，需重跑门禁' : (summary.metadataReady ? '待运行远端门禁' : '门禁未通过'))],
		        ['证据元数据', fmt(summary.passedCount || 0) + ' / ' + fmt(summary.requiredCount || 0)],
		        ['远端工件复核', fmt(summary.remoteVerifiedCount || 0) + ' / ' + fmt(summary.requiredCount || 0)],
		        ['指纹匹配', fmt(summary.fingerprintMatchedCount || 0) + ' / ' + fmt(summary.requiredCount || 0) + ' · ' + releaseFingerprintSourceLabel(summary.sourceFingerprintSource)],
		        ['待处理', fmt(summary.missingCount || 0) + ' 缺失 / ' + fmt(summary.inProgressCount || 0) + ' 进行中 / ' + fmt(summary.failedCount || 0) + ' 失败'],
		        ['补证分派', fmt(actionSummary.assignedCount || 0) + ' 已分派 / ' + fmt(actionSummary.unassignedCount || 0) + ' 待分派'],
		        ['补证时效', fmt(actionSummary.overdueCount || 0) + ' 逾期 / ' + fmt(actionSummary.dueSoonCount || 0) + ' 临期 / ' + fmt(actionSummary.resolvedCount || 0) + ' 完成'],
		        ['工件校验器', verifier.configured ? ('保存时校验 / 候选时重验 · ' + fmt(verifier.timeoutSeconds || 0) + 's / ' + formatArtifactSize(verifier.maxBytes)) : '未配置'],
		      ];
		      releaseReadinessSummaryEl.innerHTML = tiles.map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
		      releaseEvidenceActionCountEl.textContent = fmt(actionSummary.unresolvedCount || 0) + ' 待处理 / ' + fmt(actionSummary.resolvedCount || 0) + ' 已完成';
		      releaseEvidenceActionsEl.innerHTML = releaseEvidenceActionCache.length ? releaseEvidenceActionCache.map(item => {
		        const evidenceState = releaseEvidenceState(item.evidenceStatus);
		        const actionState = releaseEvidenceActionState(item.state);
		        const dueState = releaseEvidenceActionDueState(item.dueState);
		        const owner = item.ownerUserId > 0
		          ? esc(item.ownerName || ('用户 ' + item.ownerUserId)) + '<br><span class="muted">' + esc(item.ownerPhone || ('ID ' + item.ownerUserId)) + (item.ownerActive ? '' : ' / 已失效') + '</span>'
		          : '<span class="muted">未分配</span>';
		        const edit = hasPlatformPermission('platform.release.manage') ? '<button type="button" class="secondary" data-release-action-edit="' + esc(item.key) + '">分派</button>' : '<span class="muted">只读</span>';
		        return '<tr><td><strong>' + esc(item.title || item.key) + '</strong><br><span class="muted">' + esc(item.key) + ' / v' + fmt(item.version || 0) + '</span></td>' +
		          '<td>' + pill(evidenceState[0], evidenceState[1]) + ' ' + pill(actionState[0], actionState[1]) + '</td>' +
		          '<td>' + owner + '</td><td>' + pill(dueState[0], dueState[1]) + '<br><span class="muted">' + esc(item.dueAt || '-') + '</span></td>' +
		          '<td>' + esc(item.nextAction || '-') + (item.note ? '<br><span class="muted">' + esc(item.note) + '</span>' : '') + '</td>' +
		          '<td>' + esc(item.updatedAt || item.createdAt || '-') + '<br><span class="muted">用户 ' + fmt(item.updatedBy || item.createdBy || 0) + '</span></td>' +
		          '<td><div class="domain-actions">' + edit + '</div></td></tr>';
		      }).join('') : '<tr><td colspan="7" class="empty">暂无补证行动</td></tr>';
		      releaseEvidenceCountEl.textContent = fmt(releaseEvidenceCache.length) + ' 项';
		      releaseEvidenceEl.innerHTML = releaseEvidenceCache.length ? releaseEvidenceCache.map(item => {
		        const state = releaseEvidenceState(item.status);
		        const link = item.evidenceUrl ? '<a href="' + esc(item.evidenceUrl) + '" target="_blank" rel="noopener noreferrer">查看证据</a>' : '<span class="muted">-</span>';
		        const artifact = item.artifactSha256 ? '<br><span class="muted release-fingerprint" title="' + esc(item.artifactSha256) + '">工件 ' + esc(shortFingerprint(item.artifactSha256)) + ' / ' + esc(formatArtifactSize(item.artifactSizeBytes)) + '</span>' : '';
		        const edit = hasPlatformPermission('platform.release.manage') ? '<button type="button" class="secondary" data-release-evidence-edit="' + esc(item.key) + '">维护</button>' : '<span class="muted">只读</span>';
		        return '<tr><td><strong>' + esc(item.title) + '</strong><br><span class="muted">' + esc(item.category || '-') + ' / ' + esc(item.key) + '</span></td>' +
		          '<td>' + pill(state[0], state[1]) + '<br><span class="muted">v' + fmt(item.version || 0) + '</span></td>' +
		          '<td>' + esc(item.environment || '-') + '</td><td><span class="release-fingerprint" title="' + esc(item.sourceFingerprint || '') + '">' + esc(shortFingerprint(item.sourceFingerprint)) + '</span></td>' +
		          '<td>' + link + artifact + (item.note ? '<br><span class="muted">' + esc(item.note) + '</span>' : '') + '</td>' +
		          '<td>' + (item.checkedAt ? esc(item.checkedAt) + '<br><span class="muted">用户 ' + fmt(item.checkedBy || 0) + '</span>' : '<span class="muted">-</span>') + '</td><td><div class="domain-actions">' + edit + '</div></td></tr>';
		      }).join('') : '<tr><td colspan="7" class="empty">暂无发布证据</td></tr>';
		      releaseCandidateCountEl.textContent = fmt(candidates.length) + ' 个';
		      releaseCandidatesEl.innerHTML = candidates.length ? candidates.map(item => {
		        const state = releaseCandidateState(item.effectiveStatus || item.status);
		        const drift = Array.isArray(item.driftedEvidenceKeys) && item.driftedEvidenceKeys.length ? '<br><span class="muted">变化项：' + esc(item.driftedEvidenceKeys.join(', ')) + '</span>' : '';
		        return '<tr><td><strong>' + esc(item.candidateNo) + '</strong><br><span class="muted">审计 ' + fmt(item.operationId || 0) + '</span></td>' +
		          '<td>' + esc(item.releaseVersion) + '</td><td>' + pill(state[0], state[1]) + '<br><span class="muted">' + esc(item.gateMessage || '') + '</span>' + drift + '</td>' +
		          '<td>' + fmt(item.passedCount || 0) + ' 远端通过 / ' + fmt(item.matchedCount || 0) + ' 匹配 / ' + fmt(item.requiredCount || 0) + ' 必需</td>' +
		          '<td><span class="release-fingerprint" title="' + esc(item.sourceFingerprint || '') + '">' + esc(shortFingerprint(item.sourceFingerprint)) + '</span></td>' +
		          '<td>' + esc(item.createdAt || '-') + '<br><span class="muted">用户 ' + fmt(item.createdBy || 0) + '</span></td></tr>';
		      }).join('') : '<tr><td colspan="6" class="empty">暂无发布候选</td></tr>';
		    }
		    async function loadReleaseReadiness() {
		      if (!hasPlatformPermission('platform.release.read')) return;
		      const fingerprint = releaseSourceFingerprintInput.value.trim().toLowerCase();
		      if (fingerprint && !/^[0-9a-f]{64}$/.test(fingerprint)) {
		        releaseReadinessStateEl.textContent = '源码指纹必须是 64 位 SHA-256';
		        return;
		      }
		      releaseReadinessStateEl.textContent = '正在加载';
		      const params = new URLSearchParams();
		      if (fingerprint) params.set('sourceFingerprint', fingerprint);
		      try {
		        const suffix = params.toString() ? '?' + params.toString() : '';
		        const res = await fetch('/dashboard/saasAdmin/releaseReadiness' + suffix, { headers: authHeader() });
		        const body = await res.json();
		        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		        renderReleaseReadiness(body.data || {});
		        const summary = (body.data || {}).summary || {};
		        releaseReadinessStateEl.textContent = !summary.sourceFingerprintAuthoritative
		          ? '当前运行版本未内置源码指纹，发布候选已禁用'
		          : (!(summary.artifactVerifier || {}).configured
		            ? '远端证据工件校验器未配置，发布候选已禁用'
		            : (summary.ready ? '当前运行版本已有远端复核通过的发布候选' : (summary.latestCandidateEffectiveStatus === 'stale' ? '现行证据已变化，旧候选失效，请重新提交发布审批' : (summary.metadataReady ? '证据元数据完整，等待提交发布审批' : '生产证据尚未满足发布门禁'))));
		      } catch (err) {
		        releaseReadinessStateEl.textContent = err.message || String(err);
		      }
		    }
		    function openReleaseEvidenceEditor(key) {
		      const item = releaseEvidenceCache.find(candidate => candidate.key === key);
		      if (!item) return;
		      releaseEvidenceTitleEl.textContent = item.title || '维护发布证据';
		      releaseEvidenceKeyInput.value = item.key || '';
		      releaseEvidenceVersionInput.value = String(item.version || 0);
		      releaseEvidenceStatusInput.value = item.status || 'missing';
		      releaseEvidenceEnvironmentInput.value = item.environment || '';
		      releaseEvidenceUrlInput.value = item.evidenceUrl || '';
		      releaseEvidenceFingerprintInput.value = releaseSourceFingerprintInput.value.trim().toLowerCase() || item.sourceFingerprint || '';
		      releaseEvidenceArtifactFingerprintInput.value = item.artifactSha256 || '';
		      releaseEvidenceArtifactSizeInput.value = item.artifactSizeBytes > 0 ? String(item.artifactSizeBytes) : '';
		      releaseEvidenceNoteInput.value = item.note || '';
		      releaseEvidenceDialogEl.showModal();
		    }
		    async function saveReleaseEvidence() {
		      const payload = {
		        key: releaseEvidenceKeyInput.value,
		        status: releaseEvidenceStatusInput.value,
		        evidenceUrl: releaseEvidenceUrlInput.value.trim(),
		        environment: releaseEvidenceEnvironmentInput.value.trim(),
		        sourceFingerprint: releaseEvidenceFingerprintInput.value.trim().toLowerCase(),
		        artifactSha256: releaseEvidenceArtifactFingerprintInput.value.trim().toLowerCase(),
		        artifactSizeBytes: Math.trunc(Number(releaseEvidenceArtifactSizeInput.value || 0)),
		        note: releaseEvidenceNoteInput.value.trim(),
		        expectedVersion: Number(releaseEvidenceVersionInput.value || 0),
		      };
		      if (payload.status === 'passed' && (!/^[0-9a-f]{64}$/.test(payload.artifactSha256) || payload.artifactSizeBytes <= 0)) {
		        releaseReadinessStateEl.textContent = '证据通过时必须填写工件 SHA-256 和字节大小';
		        return;
		      }
		      releaseReadinessStateEl.textContent = '正在保存发布证据';
		      try {
		        const res = await fetch('/dashboard/saasAdmin/releaseEvidence', { method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		        const body = await res.json();
		        if (!res.ok || Number(body.code || 0) >= 400) throw new Error(body.msg || 'HTTP ' + res.status);
		        releaseEvidenceDialogEl.close();
		        await loadReleaseReadiness();
		        const verified = ((body.data || {}).artifactVerification || {}).verified === true;
		        releaseReadinessStateEl.textContent = (verified ? '远端工件已校验并保存' : '发布证据已保存') + ' / 审计 ' + fmt((body.data || {}).operationId || 0);
		        if (hasPlatformPermission('platform.audit.read')) loadOperations();
		      } catch (err) {
		        releaseReadinessStateEl.textContent = err.message || String(err);
		      }
		    }
		    function openReleaseEvidenceActionEditor(key) {
		      const item = releaseEvidenceActionCache.find(candidate => candidate.key === key);
		      if (!item) return;
		      releaseEvidenceActionTitleEl.textContent = item.title || '分派生产补证';
		      releaseEvidenceActionKeyInput.value = item.key || '';
		      releaseEvidenceActionVersionInput.value = String(item.version || 0);
		      const owners = releaseEvidenceOwnerCache.slice();
		      if (item.ownerUserId > 0 && !owners.some(owner => Number(owner.userId) === Number(item.ownerUserId))) {
		        owners.push({ userId: item.ownerUserId, name: item.ownerName || ('用户 ' + item.ownerUserId), phone: item.ownerPhone || '', inactive: true });
		      }
		      releaseEvidenceActionOwnerInput.innerHTML = '<option value="0">未分配</option>' + owners.map(owner =>
		        '<option value="' + esc(owner.userId) + '">' + esc(owner.name || ('用户 ' + owner.userId)) + (owner.phone ? ' / ' + esc(owner.phone) : '') + (owner.inactive ? ' / 已失效' : '') + '</option>'
		      ).join('');
		      releaseEvidenceActionOwnerInput.value = String(item.ownerUserId || 0);
		      releaseEvidenceActionDueAtInput.value = subscriptionDateTimeInput(item.dueAt || '');
		      releaseEvidenceActionNextInput.value = item.nextAction || '';
		      releaseEvidenceActionNoteInput.value = item.note || '';
		      releaseEvidenceActionDialogEl.showModal();
		    }
		    async function saveReleaseEvidenceAction() {
		      const ownerUserId = Number(releaseEvidenceActionOwnerInput.value || 0);
		      const dueAt = ownerUserId > 0 ? subscriptionDateTimePayload(releaseEvidenceActionDueAtInput.value) : '';
		      const payload = {
		        key: releaseEvidenceActionKeyInput.value,
		        ownerUserId: ownerUserId,
		        dueAt: dueAt,
		        nextAction: releaseEvidenceActionNextInput.value.trim(),
		        note: releaseEvidenceActionNoteInput.value.trim(),
		        expectedVersion: Number(releaseEvidenceActionVersionInput.value || 0),
		      };
		      if (!payload.nextAction) {
		        releaseReadinessStateEl.textContent = '下一步必填';
		        return;
		      }
		      if (payload.ownerUserId > 0 && !payload.dueAt) {
		        releaseReadinessStateEl.textContent = '分配负责人时必须设置截止时间';
		        return;
		      }
		      releaseReadinessStateEl.textContent = '正在保存补证行动';
		      try {
		        const res = await fetch('/dashboard/saasAdmin/releaseEvidenceAction', { method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		        const body = await res.json();
		        if (!res.ok || Number(body.code || 0) >= 400) throw new Error(body.msg || 'HTTP ' + res.status);
		        releaseEvidenceActionDialogEl.close();
		        await loadReleaseReadiness();
		        releaseReadinessStateEl.textContent = '补证行动已保存 / 审计 ' + fmt((body.data || {}).operationId || 0);
		        if (hasPlatformPermission('platform.audit.read')) loadOperations();
		      } catch (err) {
		        releaseReadinessStateEl.textContent = err.message || String(err);
		      }
		    }
		    async function runReleaseGate() {
		      if (!releaseGateEnabled) {
		        releaseReadinessStateEl.textContent = releaseGateDisabledReason;
		        return;
		      }
		      const payload = { releaseVersion: releaseVersionInput.value.trim() };
		      if (!payload.releaseVersion) {
		        releaseReadinessStateEl.textContent = '发布版本必填';
		        return;
		      }
		      const approvalPayload = {
		        releaseVersion: payload.releaseVersion,
		        sourceFingerprint: releaseSourceFingerprintInput.value.trim().toLowerCase(),
		      };
		      if (approvalActionRequired('release.candidate.gate', 0)) {
		        try {
		          const approval = await requestHighRiskApproval('release.candidate.gate', approvalPayload, '运行发布候选门禁：' + payload.releaseVersion);
		          releaseReadinessStateEl.textContent = '发布门禁审批已提交 / ' + (approval.requestNo || '审批单') + ' / 需 ' + fmt(approval.requiredApprovals || 2) + ' 人会签后执行';
		        } catch (err) {
		          releaseReadinessStateEl.textContent = err.message || String(err);
		        }
		        return;
		      }
		      releaseReadinessStateEl.textContent = '正在运行发布门禁';
		      try {
		        const res = await fetch('/dashboard/saasAdmin/releaseCandidate', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		        const body = await res.json();
		        if (!res.ok || Number(body.code || 0) >= 400) throw new Error(body.msg || 'HTTP ' + res.status);
		        const candidate = (body.data || {}).candidate || {};
		        await loadReleaseReadiness();
		        releaseReadinessStateEl.textContent = (candidate.status === 'ready' ? '发布门禁通过' : '发布门禁已阻断') + ' / ' + (candidate.candidateNo || '') + ' / 审计 ' + fmt((body.data || {}).operationId || 0);
		        if (hasPlatformPermission('platform.audit.read')) loadOperations();
		      } catch (err) {
		        releaseReadinessStateEl.textContent = err.message || String(err);
		      }
		    }
		    function identityRiskState(value) {
	      if (value === 'critical') return ['严重', 'danger'];
	      if (value === 'warning') return ['预警', 'warning'];
	      return ['普通', 'ok'];
	    }
	    function identitySessionState(value) {
	      if (value === 'active') return ['活动', 'ok'];
	      if (value === 'revoked') return ['已撤销', 'danger'];
	      if (value === 'expired') return ['已过期', ''];
	      return [value || '未知', ''];
	    }
	    function identityIncidentState(value) {
	      if (value === 'open') return ['待处置', 'danger'];
	      if (value === 'acknowledged') return ['已认领', 'warning'];
	      if (value === 'resolved') return ['已解决', 'ok'];
	      return [value || '未知', ''];
	    }
	    function identityEventLabel(value) {
	      const labels = {
	        password_failed: '密码失败', password_verified: '密码已验证', login_blocked: '登录已阻断',
	        mfa_required: '需要 MFA', mfa_failed: 'MFA 失败', login_succeeded: '登录成功', session_revoked: '会话撤销',
	        mfa_enrollment_began: 'MFA 开始绑定', mfa_enabled: 'MFA 已启用', mfa_reset: 'MFA 已重置',
	      };
	      return labels[value] || value || '未知事件';
	    }
	    function identityResultState(value) {
	      if (value === 'succeeded' || value === 'verified') return [value === 'verified' ? '已验证' : '成功', 'ok'];
	      if (value === 'required') return ['需要处理', 'warning'];
	      if (value === 'failed' || value === 'blocked' || value === 'revoked') return [value === 'blocked' ? '已阻断' : (value === 'revoked' ? '已撤销' : '失败'), 'danger'];
	      return [value || '未知', ''];
	    }
	    function identityTimeIsFuture(value) {
	      if (!value) return false;
	      const parsed = Date.parse(String(value).replace(' ', 'T'));
	      return Number.isFinite(parsed) && parsed > Date.now();
	    }
	    function renderIdentitySecuritySummary(summary, policy, config) {
	      summary = summary || {}; policy = policy || {}; config = config || {};
	      const tiles = [
	        ['账号保护', fmt(summary.lockedUsers || 0) + ' 锁定 / ' + fmt(summary.activeUsers || 0) + ' 启用'],
	        ['MFA 覆盖', fmt(summary.mfaEnrolledUsers || 0) + ' 已绑定 / ' + fmt(policy.mfaMissingUserCount || 0) + ' 待绑定'],
	        ['活动会话', fmt(summary.activeSessions || 0) + ' / 上限 ' + fmt(policy.maxConcurrentSessions || 0)],
	        ['24h 登录', fmt(summary.successfulLogins24h || 0) + ' 成功 / ' + fmt(summary.failedLogins24h || 0) + ' 失败'],
	        ['安全事故', fmt(summary.openIncidents || 0) + ' 打开 / ' + fmt(summary.criticalIncidents || 0) + ' 严重'],
	        ['会话校验', config.sessionEnforced ? '强制校验' : '兼容模式'],
	        ['MFA 密钥', config.mfaEncryptionReady ? '已就绪 / ' + fmt(config.mfaEncryptionKeyCount || 0) + ' 把' : '未就绪'],
	        ['来源 IP 解析', config.trustedProxyHeaders ? '受信代理 / ' + fmt(config.trustedProxyCidrCount || 0) + ' 段' : 'TCP 直连'],
	        ['策略版本', 'v' + fmt(policy.version || 0) + ' / ' + (policy.status === 'disabled' ? '已停用' : '已启用')],
	      ];
	      identitySecuritySummaryEl.innerHTML = tiles.map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
	    }
	    function renderIdentityPolicy(policy) {
	      identityPolicyCache = policy || {};
	      identityPolicyStatusInput.value = policy.status || 'active';
	      identityMaxFailedAttemptsInput.value = String(policy.maxFailedAttempts || 5);
	      identityLockoutMinutesInput.value = String(policy.lockoutMinutes || 30);
	      identitySessionTtlMinutesInput.value = String(policy.sessionTtlMinutes || 10080);
	      identityIdleTimeoutMinutesInput.value = String(policy.idleTimeoutMinutes || 1440);
	      identityMaxConcurrentSessionsInput.value = String(policy.maxConcurrentSessions || 5);
	      identityLoginEventRetentionDaysInput.value = String(policy.loginEventRetentionDays || 180);
	      identitySessionRetentionDaysInput.value = String(policy.sessionRetentionDays || 90);
	      identityRequireMfaInput.checked = !!policy.requireMfa;
	      identityAllowedIpCidrsInput.value = Array.isArray(policy.allowedIpCidrs) ? policy.allowedIpCidrs.join('\n') : '';
	      identityPolicyVersionInput.value = String(policy.version || 0);
	      renderIdentityPolicyApprovalState();
	    }
	    function renderIdentityPolicyApprovalState() {
	      const button = document.getElementById('saveIdentityPolicy');
	      if (button) button.textContent = approvalActionRequired('identity.policy.update', 0) ? '提交审批' : '保存策略';
	    }
	    function renderIdentityUsers(items) {
	      const userID = Number(identityUserFilterInput.value || 0);
	      const keyword = identityKeywordInput.value.trim().toLowerCase();
	      identityUserCache = Array.isArray(items) ? items : [];
	      items = identityUserCache.filter(item => {
	        if (userID > 0 && Number(item.userId) !== userID) return false;
	        if (!keyword) return true;
	        return [item.userName, item.phone, item.userId, item.lastFailedIp, item.lastSuccessIp].some(value => String(value || '').toLowerCase().includes(keyword));
	      });
	      identityUserCountEl.textContent = fmt(items.length) + ' 个账号';
	      if (!items.length) {
	        identityUsersEl.innerHTML = '<tr><td colspan="6" class="empty">暂无匹配账号</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.identity.manage');
	      identityUsersEl.innerHTML = items.map(item => {
	        const locked = identityTimeIsFuture(item.lockedUntil);
	        const account = Number(item.userStatus) === 1 ? (locked ? pill('已锁定', 'danger') : pill('正常', 'ok')) : pill('已停用', 'danger');
	        const mfa = item.mfaStatus === 'active' ? pill('已启用', 'ok') : (item.mfaStatus === 'pending' ? pill('待验证', 'warning') : pill('未启用', ''));
	        const actions = [];
	        if (canManage && locked && Number(item.version) > 0) actions.push('<button type="button" data-identity-user-action="unlock" data-user-id="' + fmt(item.userId) + '" data-version="' + fmt(item.version) + '">解锁</button>');
	        if (canManage && Number(item.mfaVersion) > 0 && (item.mfaStatus === 'active' || item.mfaStatus === 'pending')) {
	          const resetLabel = approvalActionRequired('identity.mfa.reset', 0) ? '提交重置审批' : '重置 MFA';
	          actions.push('<button type="button" class="danger" data-identity-user-action="mfa-reset" data-user-id="' + fmt(item.userId) + '" data-version="' + fmt(item.mfaVersion) + '">' + resetLabel + '</button>');
	        }
	        if (canManage && Number(item.activeSessions) > 0) actions.push('<button type="button" class="secondary" data-identity-user-action="revoke-all" data-user-id="' + fmt(item.userId) + '">撤销全部会话</button>');
	        return '<tr><td><strong>' + esc(item.userName || ('用户 ' + item.userId)) + '</strong><br><span class="muted">ID ' + fmt(item.userId) + ' / ' + esc(item.phone || '-') + '</span></td>' +
	          '<td>' + account + (locked ? '<br><span class="muted">至 ' + esc(item.lockedUntil) + '</span>' : '') + '</td>' +
	          '<td>' + mfa + '<br><span class="muted">恢复码 ' + fmt(item.mfaRecoveryCodes || 0) + '</span></td>' +
	          '<td>' + fmt(item.failedAttempts || 0) + ' 次<br><span class="muted">' + esc(item.lastSuccessAt || '尚无成功登录') + '<br>' + esc(item.lastSuccessIp || '-') + '</span></td>' +
	          '<td>' + fmt(item.activeSessions || 0) + '</td><td><div class="identity-actions">' + (actions.join('') || '<span class="muted">无操作</span>') + '</div></td></tr>';
	      }).join('');
	    }
	    function renderIdentitySessions(items) {
	      items = Array.isArray(items) ? items : [];
	      identitySessionCountEl.textContent = fmt(items.length) + ' 条会话';
	      if (!items.length) {
	        identitySessionsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无匹配会话</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.identity.manage');
	      identitySessionsEl.innerHTML = items.map(item => {
	        const state = identitySessionState(item.status);
	        const action = canManage && item.status === 'active'
	          ? '<button type="button" class="danger" data-identity-session-revoke="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">撤销会话</button>'
	          : '<span class="muted">无操作</span>';
	        return '<tr><td><strong>#' + fmt(item.id) + '</strong><br><span class="muted">' + esc(String(item.sessionJti || '').slice(0, 16) || '-') + '</span></td>' +
	          '<td><strong>' + esc(item.userName || ('用户 ' + item.userId)) + '</strong><br><span class="muted">' + esc(item.tenantName || ('租户 ' + item.tenantId)) + '</span></td>' +
	          '<td>' + pill(state[0], state[1]) + '<br><span class="muted">' + esc(item.authMethod || '-') + '</span></td>' +
	          '<td>' + esc(item.ipAddress || '-') + '<br><span class="muted">' + esc(String(item.userAgent || '-').slice(0, 90)) + '</span></td>' +
	          '<td>' + esc(item.issuedAt || '-') + '<br><span class="muted">到期 ' + esc(item.expiresAt || '-') + '<br>最后活动 ' + esc(item.lastSeenAt || '-') + '</span></td>' +
	          '<td><div class="identity-actions">' + action + '</div></td></tr>';
	      }).join('');
	    }
	    function renderIdentityIncidents(items) {
	      items = Array.isArray(items) ? items : [];
	      identityIncidentCountEl.textContent = fmt(items.length) + ' 个事故';
	      if (!items.length) {
	        identityIncidentsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无匹配事故</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.identity.manage');
	      identityIncidentsEl.innerHTML = items.map(item => {
	        const risk = identityRiskState(item.severity);
	        const state = identityIncidentState(item.status);
	        const actions = [];
	        if (canManage && item.status === 'open') actions.push('<button type="button" data-identity-incident-action="acknowledge" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">认领</button>');
	        if (canManage && item.status !== 'resolved') actions.push('<button type="button" class="secondary" data-identity-incident-action="assign" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '" data-assigned-to="' + esc(item.assignedTo || '') + '">分派</button>');
	        if (canManage && item.status !== 'resolved') actions.push('<button type="button" data-identity-incident-action="resolve" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">解决</button>');
	        if (canManage && item.status === 'resolved') actions.push('<button type="button" class="secondary" data-identity-incident-action="reopen" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">重开</button>');
	        return '<tr><td><strong>' + esc(item.title || item.incidentNo) + '</strong><br><span class="muted">' + esc(item.incidentNo || ('#' + item.id)) + ' / ' + esc(item.incidentType || '-') + '</span></td>' +
	          '<td>' + esc(item.userName || ('用户 ' + item.userId)) + '<br><span class="muted">' + esc(item.tenantName || ('租户 ' + item.tenantId)) + '</span></td>' +
	          '<td>' + pill(risk[0], risk[1]) + ' ' + pill(state[0], state[1]) + '</td>' +
	          '<td>' + fmt(item.occurrenceCount || 0) + ' 次<br><span class="muted">' + esc(item.latestDetail || '-') + '<br>' + esc(item.lastOccurredAt || '-') + '</span></td>' +
	          '<td>' + esc(item.assignedTo || '未分派') + '<br><span class="muted">' + esc(item.resolution || '尚无结论') + '</span></td>' +
	          '<td><div class="identity-actions">' + (actions.join('') || '<span class="muted">无操作</span>') + '</div></td></tr>';
	      }).join('');
	    }
	    function renderIdentityEvents(items) {
	      items = Array.isArray(items) ? items : [];
	      identityEventCountEl.textContent = fmt(items.length) + ' 条事件';
	      if (!items.length) {
	        identityEventsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无匹配事件</td></tr>';
	        return;
	      }
	      identityEventsEl.innerHTML = items.map(item => {
	        const result = identityResultState(item.result);
	        const risk = identityRiskState(item.riskLevel);
	        const metadata = item.metadata && Object.keys(item.metadata).length ? JSON.stringify(item.metadata, null, 2) : '';
	        return '<tr><td>' + esc(item.occurredAt || '-') + '<br><strong>' + esc(identityEventLabel(item.eventType)) + '</strong></td>' +
	          '<td>' + esc(item.userName || (item.userId ? ('用户 ' + item.userId) : '未识别账号')) + '<br><span class="muted">' + esc(item.tenantName || (item.tenantId ? ('租户 ' + item.tenantId) : '-')) + '</span></td>' +
	          '<td>' + pill(result[0], result[1]) + ' ' + pill(risk[0], risk[1]) + '</td>' +
	          '<td>' + esc(item.ipAddress || '-') + '<br><span class="muted">' + esc(String(item.userAgent || '-').slice(0, 90)) + '</span></td>' +
	          '<td>' + esc(item.reasonCode || '-') + '</td><td>' + (metadata ? '<details><summary>查看</summary><pre>' + esc(metadata) + '</pre></details>' : '<span class="muted">-</span>') + '</td></tr>';
	      }).join('');
	    }
	    function identityFilterParams(kind) {
	      const tenantID = Number(identityTenantFilterInput.value || 0);
	      const userID = Number(identityUserFilterInput.value || 0);
	      if (!Number.isInteger(tenantID) || tenantID <= 0) throw new Error('请填写有效租户 ID');
	      if (!Number.isInteger(userID) || userID < 0) throw new Error('用户 ID 无效');
	      const params = new URLSearchParams({ tenantId: String(tenantID), limit: kind === 'overview' ? '200' : '100' });
	      if (userID > 0 && kind !== 'overview') params.set('userId', String(userID));
	      if (identityKeywordInput.value.trim() && kind !== 'overview') params.set('keyword', identityKeywordInput.value.trim());
	      if (kind === 'sessions' && identitySessionStatusInput.value) params.set('status', identitySessionStatusInput.value);
	      if (kind === 'events' && identityEventRiskInput.value) params.set('risk', identityEventRiskInput.value);
	      if (kind === 'incidents' && ['critical', 'warning'].includes(identityEventRiskInput.value)) params.set('severity', identityEventRiskInput.value);
	      return params;
	    }
	    async function loadIdentitySecurity() {
	      if (!hasPlatformPermission('platform.identity.read')) return;
	      identitySecurityStateEl.textContent = '正在加载';
	      try {
	        const [overviewRes, sessionRes, eventRes, incidentRes] = await Promise.all([
	          fetch('/dashboard/saasAdmin/identityOverview?' + identityFilterParams('overview').toString(), { headers: authHeader() }),
	          fetch('/dashboard/saasAdmin/identitySessions?' + identityFilterParams('sessions').toString(), { headers: authHeader() }),
	          fetch('/dashboard/saasAdmin/identityLoginEvents?' + identityFilterParams('events').toString(), { headers: authHeader() }),
	          fetch('/dashboard/saasAdmin/identityIncidents?' + identityFilterParams('incidents').toString(), { headers: authHeader() }),
	        ]);
	        const [overviewBody, sessionBody, eventBody, incidentBody] = await Promise.all([overviewRes.json(), sessionRes.json(), eventRes.json(), incidentRes.json()]);
	        if (!overviewRes.ok || overviewBody.code !== 200) throw new Error(overviewBody.msg || 'HTTP ' + overviewRes.status);
	        if (!sessionRes.ok || sessionBody.code !== 200) throw new Error(sessionBody.msg || 'HTTP ' + sessionRes.status);
	        if (!eventRes.ok || eventBody.code !== 200) throw new Error(eventBody.msg || 'HTTP ' + eventRes.status);
	        if (!incidentRes.ok || incidentBody.code !== 200) throw new Error(incidentBody.msg || 'HTTP ' + incidentRes.status);
	        const data = overviewBody.data || {};
	        renderIdentitySecuritySummary(data.summary || {}, data.policy || {}, data.config || {});
	        renderIdentityPolicy(data.policy || {});
	        renderIdentityUsers(data.users || []);
	        renderIdentitySessions((sessionBody.data || {}).sessions || []);
	        renderIdentityEvents((eventBody.data || {}).events || []);
	        renderIdentityIncidents((incidentBody.data || {}).incidents || []);
	        identitySecurityStateEl.textContent = ((data.policy || {}).tenantName || ('租户 ' + identityTenantFilterInput.value)) + ' / ' + ((data.config || {}).sessionEnforced ? '持久会话已强制' : '持久会话兼容模式');
	      } catch (err) {
	        identitySecurityStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function identitySecurityWrite(path, payload) {
	      const res = await fetch(path, { method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
	      const body = await res.json();
	      if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	      return body.data || {};
	    }
	    async function saveIdentityPolicy() {
	      if (!hasPlatformPermission('platform.identity.manage')) return;
	      const payload = {
	        tenantId: Number(identityTenantFilterInput.value || 0), status: identityPolicyStatusInput.value,
	        maxFailedAttempts: Number(identityMaxFailedAttemptsInput.value), lockoutMinutes: Number(identityLockoutMinutesInput.value),
	        sessionTtlMinutes: Number(identitySessionTtlMinutesInput.value), idleTimeoutMinutes: Number(identityIdleTimeoutMinutesInput.value),
	        maxConcurrentSessions: Number(identityMaxConcurrentSessionsInput.value), requireMfa: identityRequireMfaInput.checked,
	        allowedIpCidrs: identityAllowedIpCidrsInput.value.split(/[\s,]+/).map(value => value.trim()).filter(Boolean),
	        loginEventRetentionDays: Number(identityLoginEventRetentionDaysInput.value), sessionRetentionDays: Number(identitySessionRetentionDaysInput.value),
	        expectedVersion: Number(identityPolicyVersionInput.value || 0),
	      };
	      if (payload.requireMfa && !identityPolicyCache.requireMfa && !window.confirm('强制 MFA 后，未完成绑定的账号将无法登录。确认保存？')) return;
	      const requiresApproval = approvalActionRequired('identity.policy.update', 0);
	      identitySecurityStateEl.textContent = requiresApproval ? '正在提交身份安全策略变更审批' : '正在保存身份安全策略';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval('identity.policy.update', payload, '变更租户 ' + payload.tenantId + ' 的身份安全策略');
	          identitySecurityStateEl.textContent = '身份安全策略变更已提交双人审批';
	          return;
	        }
	        await identitySecurityWrite('/dashboard/saasAdmin/identityPolicy', payload);
	        await loadIdentitySecurity();
	        void loadTenantReadiness();
	        loadOperations();
	      } catch (err) {
	        identitySecurityStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function updateIdentityUser(button) {
	      if (!hasPlatformPermission('platform.identity.manage')) return;
	      const action = button.dataset.identityUserAction;
	      const userID = Number(button.dataset.userId || 0);
	      let path = '', payload = {}, reason = '', approvalAction = '';
	      if (action === 'unlock') {
	        reason = window.prompt('请输入账号解锁原因', '已核验账号所有者') || '';
	        path = '/dashboard/saasAdmin/identityUser';
	        payload = { userId: userID, expectedVersion: Number(button.dataset.version || 0), reason };
	      } else if (action === 'mfa-reset') {
	        if (!window.confirm('重置 MFA 后，现有动态码、恢复码和全部活动会话将立即失效。确认继续？')) return;
	        reason = window.prompt('请输入 MFA 重置原因', '用户无法使用原认证器') || '';
	        path = '/dashboard/saasAdmin/identityMFA';
	        payload = { userId: userID, expectedVersion: Number(button.dataset.version || 0), reason };
	        approvalAction = 'identity.mfa.reset';
	      } else if (action === 'revoke-all') {
	        reason = window.prompt('请输入撤销全部会话的原因', '账号安全处置') || '';
	        path = '/dashboard/saasAdmin/identitySession';
	        payload = { userId: userID, reason };
	      }
	      if (!reason.trim() || !path) return;
	      const requiresApproval = approvalAction && approvalActionRequired(approvalAction, 0);
	      identitySecurityStateEl.textContent = requiresApproval ? '正在提交 MFA 重置审批' : '正在执行账号安全操作';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval(approvalAction, payload, '重置用户 ' + userID + ' 的 MFA 并撤销全部活动会话：' + reason.trim());
	          identitySecurityStateEl.textContent = 'MFA 重置已提交双人审批';
	          return;
	        }
	        await identitySecurityWrite(path, payload);
	        await loadIdentitySecurity();
	        loadOperations();
	      } catch (err) {
	        identitySecurityStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function revokeIdentitySession(button) {
	      if (!hasPlatformPermission('platform.identity.manage')) return;
	      const reason = window.prompt('请输入撤销会话的原因', '管理员强制下线') || '';
	      if (!reason.trim()) return;
	      identitySecurityStateEl.textContent = '正在撤销会话';
	      try {
	        await identitySecurityWrite('/dashboard/saasAdmin/identitySession', { sessionId: Number(button.dataset.identitySessionRevoke || 0), expectedVersion: Number(button.dataset.version || 0), reason });
	        await loadIdentitySecurity();
	        loadOperations();
	      } catch (err) {
	        identitySecurityStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function updateIdentityIncident(button) {
	      if (!hasPlatformPermission('platform.identity.manage')) return;
	      const action = button.dataset.identityIncidentAction;
	      const payload = { id: Number(button.dataset.incidentId || 0), action, expectedVersion: Number(button.dataset.version || 0), assignedTo: '', resolution: '' };
	      if (action === 'assign') {
	        payload.assignedTo = window.prompt('请输入安全事故负责人', button.dataset.assignedTo || '') || '';
	        if (!payload.assignedTo.trim()) return;
	      }
	      if (action === 'resolve') {
	        payload.resolution = window.prompt('请输入安全事故处置结论') || '';
	        if (!payload.resolution.trim()) return;
	      }
	      if (action === 'reopen' && !window.confirm('确认重开该安全事故？')) return;
	      identitySecurityStateEl.textContent = '正在更新安全事故';
	      try {
	        await identitySecurityWrite('/dashboard/saasAdmin/identityIncident', payload);
	        await loadIdentitySecurity();
	        loadOperations();
	      } catch (err) {
	        identitySecurityStateEl.textContent = err.message || String(err);
	      }
	    }
	    function systemHealthState(value) {
	      if (value === 'critical') return ['严重', 'danger'];
	      if (value === 'warning') return ['预警', 'warning'];
	      return ['健康', 'ok'];
	    }
	    function systemIncidentState(value) {
	      if (value === 'open') return ['待处置', 'danger'];
	      if (value === 'acknowledged') return ['已认领', 'warning'];
	      if (value === 'resolved') return ['已解决', 'ok'];
	      return [value || '未知', ''];
	    }
	    function renderSystemHealthSummary(summary, activeIncidentCount) {
	      summary = summary || {};
	      const state = systemHealthState(summary.healthState);
	      const tiles = [
	        ['平台状态', pill(state[0], state[1])],
	        ['健康检查', esc(summary.healthyCount || 0) + ' / ' + esc(summary.checkCount || 0)],
	        ['问题分布', esc(summary.criticalCount || 0) + ' 严重 / ' + esc(summary.warningCount || 0) + ' 预警'],
	        ['活跃事故', esc(activeIncidentCount || 0)],
	      ];
	      systemHealthSummaryEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	    }
	    function renderSystemHealthChecks(items) {
	      items = items || [];
	      systemHealthCheckCountEl.textContent = fmt(items.length) + ' 项';
	      if (!items.length) {
	        systemHealthChecksEl.innerHTML = '<tr><td colspan="5" class="empty">暂无检查结果</td></tr>';
	        return;
	      }
	      systemHealthChecksEl.innerHTML = items.map(item => {
	        const state = systemHealthState(item.status);
	        return '<tr><td><strong>' + esc(item.name || item.code) + '</strong><br><span class="muted">' + esc(item.code) + '</span></td>' +
	          '<td>' + esc(item.category || '-') + '</td><td>' + pill(state[0], state[1]) + '</td>' +
	          '<td>' + fmt(item.current || 0) + ' / ' + fmt(item.threshold || 0) + '</td><td>' + esc(item.detail || '-') + '</td></tr>';
	      }).join('');
	    }
	    function renderSystemIncidents(items) {
	      items = items || [];
	      systemIncidentCountEl.textContent = fmt(items.length) + ' 条';
	      if (!items.length) {
	        systemIncidentsEl.innerHTML = '<tr><td colspan="7" class="empty">当前筛选下无系统事故</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.system.manage');
	      systemIncidentsEl.innerHTML = items.map(item => {
	        const severity = systemHealthState(item.severity);
	        const state = systemIncidentState(item.status);
	        const owner = canManage
	          ? '<input data-system-incident-owner="' + fmt(item.id) + '" value="' + esc(item.owner || '') + '" placeholder="姓名或班组">'
	          : esc(item.owner || '未分派');
	        const note = canManage
	          ? '<input data-system-incident-note="' + fmt(item.id) + '" placeholder="处置原因或结论">'
	          : esc(item.resolutionNote || '-');
	        const actions = [];
	        if (canManage && item.status === 'open') actions.push('<button type="button" data-system-incident-action="acknowledge" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">认领</button>');
	        if (canManage && item.status !== 'resolved') {
	          actions.push('<button type="button" data-system-incident-action="assign" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">分派</button>');
	          actions.push('<button type="button" data-system-incident-action="resolve" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">解决</button>');
	        }
	        if (canManage && item.status === 'resolved') actions.push('<button type="button" data-system-incident-action="reopen" data-incident-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">重开</button>');
	        return '<tr><td><strong>' + esc(item.title || item.incidentKey) + '</strong><br><span class="muted">' + esc(item.source || '-') + ' / ' + esc(item.category || '-') + ' / 累计 ' + fmt(item.occurrenceCount || 0) + ' 次</span><br><span class="muted">' + esc(item.detail || '-') + '</span></td>' +
	          '<td>' + pill(severity[0], severity[1]) + '<br>' + pill(state[0], state[1]) + '</td><td>' + fmt(item.currentValue || 0) + ' / ' + fmt(item.thresholdValue || 0) + '</td>' +
	          '<td>' + owner + '</td><td>' + note + '</td><td>' + esc(item.lastDetectedAt || '-') + '<br><span class="muted">首次 ' + esc(item.firstDetectedAt || '-') + '</span></td>' +
	          '<td><div class="incident-actions">' + (actions.join('') || '<span class="muted">只读</span>') + '</div></td></tr>';
	      }).join('');
	    }
	    function renderSystemHealthScans(items) {
	      items = items || [];
	      systemHealthScanCountEl.textContent = fmt(items.length) + ' 次';
	      if (!items.length) {
	        systemHealthScansEl.innerHTML = '<tr><td colspan="7" class="empty">暂无扫描记录</td></tr>';
	        return;
	      }
	      systemHealthScansEl.innerHTML = items.map(item => {
	        const state = systemHealthState(item.healthState);
	        return '<tr><td><strong>' + esc(item.scanNo || ('#' + item.id)) + '</strong><br><span class="muted">#' + fmt(item.id) + ' / 操作 ' + fmt(item.operationId || 0) + '</span></td>' +
	          '<td>' + esc(item.triggerType === 'cron' ? '定时' : '手动') + '<br><span class="muted">用户 ' + fmt(item.actorUserId || 0) + '</span></td><td>' + pill(state[0], state[1]) + '</td>' +
	          '<td>' + fmt(item.issueCount || 0) + ' / ' + fmt(item.checkCount || 0) + '<br><span class="muted">严重 ' + fmt(item.criticalCount || 0) + ' / 预警 ' + fmt(item.warningCount || 0) + '</span></td>' +
	          '<td>新增 ' + fmt(item.openedCount || 0) + ' / 重开 ' + fmt(item.reopenedCount || 0) + '<br><span class="muted">恢复 ' + fmt(item.recoveredCount || 0) + '</span></td>' +
	          '<td>' + fmt(item.notificationCount || 0) + '</td><td>' + esc(item.finishedAt || item.createdAt || '-') + '</td></tr>';
	      }).join('');
	    }
	    function systemHealthOptionsParams() {
	      const failureWindowHours = Number(systemHealthFailureWindowInput.value.trim());
	      const notificationStaleMinutes = Number(systemHealthNotificationStaleInput.value.trim());
	      if (!Number.isInteger(failureWindowHours) || failureWindowHours < 1 || failureWindowHours > 720) throw new Error('失败窗口必须在 1 至 720 小时之间');
	      if (!Number.isInteger(notificationStaleMinutes) || notificationStaleMinutes < 1 || notificationStaleMinutes > 10080) throw new Error('通知滞留阈值必须在 1 至 10080 分钟之间');
	      return { failureWindowHours, notificationStaleMinutes };
	    }
	    async function loadSystemHealth() {
	      if (!hasPlatformPermission('platform.system.read')) return;
	      systemHealthStateEl.textContent = '正在加载';
	      try {
	        const options = systemHealthOptionsParams();
	        const healthParams = new URLSearchParams({ failureWindowHours: String(options.failureWindowHours), notificationStaleMinutes: String(options.notificationStaleMinutes) });
	        const incidentParams = new URLSearchParams({ status: systemIncidentStatusInput.value || 'active', severity: systemIncidentSeverityInput.value || 'all', limit: '100' });
	        if (systemIncidentOwnerInput.value.trim()) incidentParams.set('owner', systemIncidentOwnerInput.value.trim());
	        if (systemIncidentKeywordInput.value.trim()) incidentParams.set('keyword', systemIncidentKeywordInput.value.trim());
	        const [healthRes, incidentRes, scanRes] = await Promise.all([
	          fetch('/dashboard/saasAdmin/systemHealth?' + healthParams.toString(), { headers: authHeader() }),
	          fetch('/dashboard/saasAdmin/systemIncidents?' + incidentParams.toString(), { headers: authHeader() }),
	          fetch('/dashboard/saasAdmin/systemHealthScans?limit=30', { headers: authHeader() }),
	        ]);
	        const [healthBody, incidentBody, scanBody] = await Promise.all([healthRes.json(), incidentRes.json(), scanRes.json()]);
	        if (!healthRes.ok || healthBody.code !== 200) throw new Error(healthBody.msg || 'HTTP ' + healthRes.status);
	        if (!incidentRes.ok || incidentBody.code !== 200) throw new Error(incidentBody.msg || 'HTTP ' + incidentRes.status);
	        if (!scanRes.ok || scanBody.code !== 200) throw new Error(scanBody.msg || 'HTTP ' + scanRes.status);
	        const healthData = healthBody.data || {};
	        const incidents = (incidentBody.data || {}).items || [];
	        const scans = (scanBody.data || {}).items || [];
	        renderSystemHealthSummary(healthData.summary || {}, (healthData.incidents || []).length);
	        renderSystemHealthChecks(healthData.checks || []);
	        renderSystemIncidents(incidents);
	        renderSystemHealthScans(scans);
	        const state = systemHealthState((healthData.summary || {}).healthState);
	        systemHealthStateEl.textContent = state[0] + ' / ' + fmt((healthData.summary || {}).issueCount || 0) + ' 项问题 / ' + fmt((healthData.incidents || []).length) + ' 个活跃事故';
	      } catch (err) {
	        systemHealthStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function runSystemHealthScan() {
	      if (!hasPlatformPermission('platform.system.manage')) return;
	      systemHealthStateEl.textContent = '正在扫描';
	      try {
	        const options = systemHealthOptionsParams();
	        const res = await fetch('/dashboard/saasAdmin/systemHealthScan', {
	          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	          body: JSON.stringify({ failureWindowHours: options.failureWindowHours, notificationStaleMinutes: options.notificationStaleMinutes, notify: true }),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        systemHealthStateEl.textContent = '扫描完成：新增 ' + fmt(data.openedCount || 0) + ' / 重开 ' + fmt(data.reopenedCount || 0) + ' / 恢复 ' + fmt(data.recoveredCount || 0);
	        await loadSystemHealth();
	        loadOperations();
	      } catch (err) {
	        systemHealthStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function updateSystemIncident(button) {
	      if (!hasPlatformPermission('platform.system.manage')) return;
	      const incidentId = Number(button.dataset.incidentId || 0);
	      const action = button.dataset.systemIncidentAction || '';
	      const ownerInput = systemIncidentsEl.querySelector('[data-system-incident-owner="' + incidentId + '"]');
	      const noteInput = systemIncidentsEl.querySelector('[data-system-incident-note="' + incidentId + '"]');
	      const owner = ownerInput ? ownerInput.value.trim() : '';
	      const note = noteInput ? noteInput.value.trim() : '';
	      if ((action === 'acknowledge' || action === 'assign') && !owner) {
	        systemHealthStateEl.textContent = '认领或分派事故时必须填写负责人';
	        return;
	      }
	      if ((action === 'resolve' || action === 'reopen') && !note) {
	        systemHealthStateEl.textContent = '解决或重开事故时必须填写处置备注';
	        return;
	      }
	      const rowButtons = button.closest('tr').querySelectorAll('button');
	      rowButtons.forEach(item => { item.disabled = true; });
	      systemHealthStateEl.textContent = '正在更新事故';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/systemIncident', {
	          method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	          body: JSON.stringify({ incidentId, action, expectedVersion: Number(button.dataset.version || 0), owner, note }),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        await loadSystemHealth();
	        loadOperations();
	      } catch (err) {
	        rowButtons.forEach(item => { item.disabled = false; });
	        systemHealthStateEl.textContent = err.message || String(err);
	      }
	    }
	    function backupRunStatusState(value) {
	      if (value === 'succeeded') return ['成功', 'ok'];
	      if (value === 'running') return ['执行中', 'warning'];
	      if (value === 'failed') return ['失败', 'danger'];
	      if (value === 'deleted') return ['已清理', ''];
	      return [value || '未知', ''];
	    }
	    function backupVerificationState(value) {
	      if (value === 'passed') return ['校验通过', 'ok'];
	      if (value === 'failed') return ['校验失败', 'danger'];
	      return ['待校验', 'warning'];
	    }
	    function backupReplicaState(value) {
	      if (value === 'succeeded') return ['副本通过', 'ok'];
	      if (value === 'uploading') return ['上传中', 'warning'];
	      if (value === 'pending') return ['待复制', 'warning'];
	      if (value === 'failed') return ['副本失败', 'danger'];
	      if (value === 'deleted') return ['已删除', ''];
	      return ['未配置', ''];
	    }
	    function backupFormatBytes(value) {
	      const size = Number(value || 0);
	      if (size < 1024) return fmt(size) + ' B';
	      if (size < 1048576) return (size / 1024).toFixed(1) + ' KiB';
	      return (size / 1048576).toFixed(1) + ' MiB';
	    }
	    function renderBackupConfig(config) {
	      backupConfigCache = config || {};
	      const cronSeconds = Number(config.cronIntervalSeconds || 0);
	      const cronInterval = cronSeconds > 0 && cronSeconds % 60 === 0 ? fmt(cronSeconds / 60) + ' 分钟' : fmt(cronSeconds) + ' 秒';
	      const cronLabel = config.cronEnabled ? '自动调度 ' + cronInterval + (config.cronRunOnStart ? ' / 启动检查' : '') : '自动调度';
	      const states = [
	        [cronLabel, config.cronEnabled],
	        ['源库', config.sourceConfigured], ['目录', config.backupRootConfigured],
	        ['密钥环 ' + fmt(config.encryptionKeyCount || 0), config.encryptionConfigured],
	        ['异地副本', config.replicaConfigured], ['导出工具', config.dumpToolReady],
	        [config.restoreAutoProvision ? '临时恢复' : '恢复目标', config.restoreConfigured], ['恢复工具', config.restoreToolReady],
	      ];
	      backupConfigStateEl.innerHTML = states.map(item => pill(item[0] + (item[1] ? '就绪' : '未就绪'), item[1] ? 'ok' : 'danger')).join(' ');
	      const canManage = hasPlatformPermission('platform.backups.manage');
	      const createReady = config.sourceConfigured && config.backupRootConfigured && config.dumpToolReady &&
	        (!backupPolicyCache.requireEncryption || config.encryptionConfigured) &&
	        (!backupPolicyCache.requireOffsiteReplica || config.replicaConfigured);
	      document.getElementById('createBackup').disabled = !canManage || !createReady;
	      document.getElementById('createBackup').title = createReady ? '' : '备份源、密钥、异地副本或导出工具未满足当前策略';
	    }
	    function renderBackupPolicy(policy) {
	      policy = policy || {};
	      backupPolicyCache = policy;
	      backupPolicyStatusInput.value = policy.status || 'active';
	      backupIntervalMinutesInput.value = String(policy.intervalMinutes || 1440);
	      backupRetentionDaysInput.value = String(policy.retentionDays || 30);
	      backupMinSuccessfulInput.value = String(policy.minSuccessfulBackups || 7);
	      backupMaxAgeMinutesInput.value = String(policy.maxBackupAgeMinutes || 1800);
	      backupDrillIntervalDaysInput.value = String(policy.restoreDrillIntervalDays || 30);
	      backupRequireEncryptionInput.checked = policy.requireEncryption !== false;
	      backupRequireOffsiteReplicaInput.checked = policy.requireOffsiteReplica === true;
	      backupPolicyVersionInput.value = String(policy.version || 0);
	      renderBackupPolicyApprovalState();
	    }
	    function renderBackupPolicyApprovalState() {
	      const button = document.getElementById('saveBackupPolicy');
	      if (!button) return;
	      button.textContent = approvalActionRequired('backup.policy.update', 0) ? '提交审批' : '保存策略';
	      const cleanupButton = document.getElementById('cleanupBackups');
	      if (cleanupButton) cleanupButton.textContent = approvalActionRequired('backup.retention.cleanup', 0) ? '提交清理审批' : '执行保留清理';
	    }
	    function renderBackupSummary(summary) {
	      summary = summary || {};
	      const tiles = [
	        ['备份运行', fmt(summary.runCount || 0)], ['成功', fmt(summary.successfulCount || 0)],
	        ['校验通过', fmt(summary.verifiedCount || 0)], ['失败', fmt(summary.failedCount || 0)],
	        ['异地副本', fmt(summary.replicaSucceededCount || 0) + ' 通过 / ' + fmt(summary.replicaFailedCount || 0) + ' 失败'],
	        ['运行中', fmt(summary.runningCount || 0)], ['恢复演练', fmt(summary.successfulDrillCount || 0) + ' / ' + fmt(summary.drillCount || 0)],
	        ['清理任务', fmt(summary.cleanupRunningCount || 0) + ' 运行 / ' + fmt(summary.cleanupFailedCount || 0) + ' 异常'],
	      ];
	      backupSummaryEl.innerHTML = tiles.map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
	    }
	    function renderInlineErrorDetails(value) {
	      const message = String(value || '').trim();
	      if (!message) return '';
	      return '<details class="inline-error"><summary>查看失败详情</summary><pre>' + esc(message) + '</pre></details>';
	    }
	    function renderBackupRuns(items) {
	      items = items || [];
	      backupRunCountEl.textContent = fmt(items.length) + ' 次';
	      if (!items.length) {
	        backupRunsEl.innerHTML = '<tr><td colspan="8" class="empty">暂无备份运行</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.backups.manage');
	      const restoreReady = backupConfigCache.restoreConfigured && backupConfigCache.restoreToolReady;
	      backupRunsEl.innerHTML = items.map(item => {
	        const state = backupRunStatusState(item.status);
	        const verify = backupVerificationState(item.verificationStatus);
	        const replica = backupReplicaState(item.replicaStatus);
	        const actions = [];
	        if (canManage && item.status === 'succeeded' && !Number(item.cleanupRunId || 0)) {
	          actions.push('<button type="button" class="secondary" data-backup-action="verify" data-backup-run-id="' + fmt(item.id) + '">校验</button>');
	          if (backupConfigCache.replicaConfigured && item.replicaStatus !== 'uploading') {
	            const replicaAction = item.replicaStatus === 'succeeded' ? '检查/修复副本' : '复制异地';
	            actions.push('<button type="button" class="secondary" data-backup-action="replicate" data-backup-run-id="' + fmt(item.id) + '">' + replicaAction + '</button>');
	          }
	          if (restoreReady && item.encryptionKeyReady) actions.push('<button type="button" data-backup-action="restore" data-backup-run-id="' + fmt(item.id) + '">恢复演练</button>');
	        }
	        return '<tr><td><strong>' + esc(item.backupNo || ('#' + item.id)) + '</strong><br><span class="muted">#' + fmt(item.id) + ' / v' + fmt(item.version || 0) + '</span></td>' +
	          '<td>' + pill(state[0], state[1]) + '<br><span class="muted">' + esc(item.triggerType === 'cron' ? '定时' : (item.triggerType === 'maintenance' ? '维护命令' : '手动')) + '</span></td>' +
	          '<td>' + pill(item.encrypted ? '已加密' : '未加密', item.encrypted ? 'ok' : 'danger') + ' ' + backupFormatBytes(item.sizeBytes) + '<br>' + pill(item.encryptionKeyReady ? '密钥可用' : '密钥缺失', item.encryptionKeyReady ? 'ok' : 'danger') + '<br><span class="muted">' + esc(item.artifactName || '-') + '</span></td>' +
	          '<td>' + esc(item.migrationVersion || '-') + ' / ' + fmt(item.migrationCount || 0) + '<br><span class="muted">' + fmt(item.tableCount || 0) + ' 张表</span></td>' +
	          '<td>' + pill(verify[0], verify[1]) + '<br><span class="muted">SHA ' + esc((item.sha256 || '-').slice(0, 12)) + '</span></td>' +
	          '<td>' + pill(replica[0], replica[1]) + '<br><span class="muted">' + esc(item.replicaProvider ? (item.replicaProvider + ' / ' + item.replicaBucket) : '-') + '</span>' + renderInlineErrorDetails(item.replicaError) + '</td>' +
	          '<td>' + esc(item.finishedAt || item.startedAt || '-') + renderInlineErrorDetails(item.errorMessage || item.verificationError) + '</td>' +
	          '<td><div class="backup-actions">' + (actions.join('') || (Number(item.cleanupRunId || 0) ? '<span class="muted">清理任务 #' + fmt(item.cleanupRunId) + '</span>' : '<span class="muted">只读</span>')) + '</div></td></tr>';
	      }).join('');
	    }
	    function backupCleanupStatusState(value) {
	      if (value === 'succeeded') return ['已完成', 'ok'];
	      if (value === 'running') return ['执行中', 'warning'];
	      if (value === 'pending') return ['待执行', 'warning'];
	      if (value === 'partial') return ['部分成功', 'danger'];
	      if (value === 'failed') return ['失败', 'danger'];
	      return [value || '未知', ''];
	    }
	    function renderBackupCleanupRuns(items) {
	      items = items || [];
	      backupCleanupRunCountEl.textContent = fmt(items.length) + ' 次';
	      if (!items.length) {
	        backupCleanupRunsEl.innerHTML = '<tr><td colspan="8" class="empty">暂无保留清理任务</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.backups.manage');
	      backupCleanupRunsEl.innerHTML = items.map(item => {
	        const state = backupCleanupStatusState(item.status);
	        const retry = canManage && (item.status === 'failed' || item.status === 'partial')
	          ? '<button type="button" class="secondary" data-backup-cleanup-retry="' + fmt(item.id) + '">重试未完成项</button>'
	          : '<span class="muted">只读</span>';
	        return '<tr><td><strong>' + esc(item.cleanupNo || ('#' + item.id)) + '</strong><br><span class="muted">#' + fmt(item.id) + ' / v' + fmt(item.version || 0) + '</span></td>' +
	          '<td>' + pill(state[0], state[1]) + '</td>' +
	          '<td>' + fmt(item.candidateCount || 0) + ' 个候选<br><span class="muted">扫描 ' + fmt(item.scannedCount || 0) + ' / 保留 ' + fmt(item.preservedCount || 0) + '<br>截止 ' + esc(item.cutoffAt || '-') + '</span></td>' +
	          '<td>' + fmt(item.deletedCount || 0) + ' 已清理 / ' + fmt(item.failedCount || 0) + ' 失败<br><span class="muted">异地 ' + fmt(item.replicasDeletedCount || 0) + ' / 本地缺失 ' + fmt(item.missingFilesCount || 0) + '</span></td>' +
	          '<td>' + (Number(item.approvalId || 0) ? '审批 #' + fmt(item.approvalId) : '审批未启用') + '<br><span class="muted">尝试 ' + fmt(item.attempts || 0) + ' 次</span></td>' +
	          '<td>' + esc(item.finishedAt || item.startedAt || item.createdAt || '-') + '</td>' +
	          '<td>' + (item.lastError ? renderInlineErrorDetails(item.lastError) : '<span class="muted">-</span>') + '</td>' +
	          '<td><div class="backup-actions">' + retry + '</div></td></tr>';
	      }).join('');
	    }
	    function renderRestoreDrills(items) {
	      items = items || [];
	      restoreDrillCountEl.textContent = fmt(items.length) + ' 次';
	      if (!items.length) {
	        restoreDrillsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无恢复演练</td></tr>';
	        return;
	      }
	      restoreDrillsEl.innerHTML = items.map(item => {
	        const state = backupRunStatusState(item.status);
	        const tableMatch = Number(item.actualTableCount || 0) === Number(item.expectedTableCount || 0);
	        return '<tr><td><strong>' + esc(item.drillNo || ('#' + item.id)) + '</strong><br><span class="muted">#' + fmt(item.id) + '</span></td>' +
	          '<td>#' + fmt(item.backupRunId || 0) + '</td><td>' + pill(state[0], state[1]) + '</td>' +
	          '<td>' + esc(item.targetDatabase || '-') + '<br><span class="muted">' + esc(item.targetLifecycle === 'ephemeral' ? '临时库' : '预配置库') + ' / ' + esc(item.targetCleanupStatus || '-') + '</span><br><span class="muted">' + esc((item.targetFingerprint || '').slice(0, 12)) + '</span>' + renderInlineErrorDetails(item.targetCleanupError) + '</td>' +
	          '<td>' + esc(item.actualMigrationVersion || '-') + ' / ' + fmt(item.actualMigrationCount || 0) + '<br>' + pill((tableMatch ? '表数一致 ' : '表数不一致 ') + fmt(item.actualTableCount || 0) + '/' + fmt(item.expectedTableCount || 0), tableMatch ? 'ok' : 'danger') + '</td>' +
	          '<td>' + fmt(item.durationMs || 0) + ' ms</td><td>' + esc(item.finishedAt || item.startedAt || '-') + renderInlineErrorDetails(item.errorMessage) + '</td></tr>';
	      }).join('');
	    }
	    async function loadBackupOverview() {
	      if (!hasPlatformPermission('platform.backups.read')) return;
	      backupStateEl.textContent = '正在加载';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/backupOverview?limit=100', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        backupRunCache = Array.isArray(data.runs) ? data.runs : [];
	        renderBackupPolicy(data.policy || {});
	        renderBackupConfig(data.config || {});
	        renderBackupSummary(data.summary || {});
	        renderBackupRuns(backupRunCache);
	        renderBackupCleanupRuns(Array.isArray(data.cleanupRuns) ? data.cleanupRuns : []);
	        renderRestoreDrills(Array.isArray(data.drills) ? data.drills : []);
	        backupStateEl.textContent = '已加载 ' + fmt(backupRunCache.length) + ' 次备份 / ' + fmt((data.cleanupRuns || []).length) + ' 次清理 / ' + fmt((data.drills || []).length) + ' 次演练';
	      } catch (err) {
	        backupStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function saveBackupPolicy() {
	      if (!hasPlatformPermission('platform.backups.manage')) return;
	      const payload = {
	        status: backupPolicyStatusInput.value,
	        intervalMinutes: Number(backupIntervalMinutesInput.value || 0),
	        retentionDays: Number(backupRetentionDaysInput.value || 0),
	        minSuccessfulBackups: Number(backupMinSuccessfulInput.value || 0),
	        maxBackupAgeMinutes: Number(backupMaxAgeMinutesInput.value || 0),
	        restoreDrillIntervalDays: Number(backupDrillIntervalDaysInput.value || 0),
	        requireEncryption: backupRequireEncryptionInput.checked,
	        requireOffsiteReplica: backupRequireOffsiteReplicaInput.checked,
	        expectedVersion: Number(backupPolicyVersionInput.value || 0),
	      };
	      const requiresApproval = approvalActionRequired('backup.policy.update', 0);
	      backupStateEl.textContent = requiresApproval ? '正在提交备份策略变更审批' : '正在保存备份策略';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval('backup.policy.update', payload, '变更平台数据库备份策略');
	          backupStateEl.textContent = '备份策略变更已提交双人审批';
	          return;
	        }
	        const res = await fetch('/dashboard/saasAdmin/backupPolicy', {
	          method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        await loadBackupOverview();
	        backupStateEl.textContent = '备份策略已保存';
	        loadOperations();
	      } catch (err) {
	        backupStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function requestBackupCleanup() {
	      if (!hasPlatformPermission('platform.backups.manage')) return;
	      if (!approvalActionRequired('backup.retention.cleanup', 0)) {
	        await runBackupAction('cleanup');
	        return;
	      }
	      backupStateEl.textContent = '正在冻结候选范围并提交清理审批';
	      try {
	        await requestHighRiskApproval('backup.retention.cleanup', {}, '按当前保留策略清理已过期的平台数据库备份');
	        backupStateEl.textContent = '备份保留清理已提交双人审批';
	      } catch (err) {
	        backupStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function runBackupAction(action, backupRunId, cleanupRunId) {
	      if (!hasPlatformPermission('platform.backups.manage')) return;
	      const payload = { action };
	      if (backupRunId) payload.backupRunId = Number(backupRunId);
	      if (cleanupRunId) payload.cleanupRunId = Number(cleanupRunId);
	      backupStateEl.textContent = action === 'create' ? '正在创建加密备份' : (action === 'verify' ? '正在校验备份与异地副本' : (action === 'replicate' ? '正在复制并校验异地副本' : (action === 'cleanup-retry' ? '正在重试未完成清理步骤' : '正在执行保留清理')));
	      try {
	        const res = await fetch('/dashboard/saasAdmin/backupRun', {
	          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	        });
	        const body = await res.json();
	        if (!res.ok || (body.code !== 200 && body.code !== 201)) throw new Error(body.msg || 'HTTP ' + res.status);
	        await loadBackupOverview();
	        backupStateEl.textContent = action === 'create' ? '备份、校验与异地副本已完成' : (action === 'verify' ? '本地与异地副本校验已完成' : (action === 'replicate' ? '异地副本已完成并通过校验' : (action === 'cleanup-retry' ? '清理任务重试已完成' : '保留清理已完成')));
	        loadSystemHealth();
	        loadOperations();
	      } catch (err) {
	        backupStateEl.textContent = err.message || String(err);
	      }
	    }
	    function requestBackupRestore(backupRunId) {
	      if (!hasPlatformPermission('platform.backups.manage')) return;
	      const run = backupRunCache.find(item => Number(item.id) === Number(backupRunId));
	      if (!run) return;
	      pendingBackupRestore = run;
	      const targetText = backupConfigCache.restoreAutoProvision ? '自动创建临时隔离库，校验后立即销毁' : '写入预配置隔离空库';
	      backupRestoreMessageEl.textContent = '使用 ' + (run.backupNo || ('备份 #' + run.id)) + ' ' + targetText + '，并核对迁移和表结构？';
	      backupRestoreDialogEl.showModal();
	    }
	    function cancelBackupRestore() {
	      pendingBackupRestore = null;
	      backupRestoreDialogEl.close();
	    }
	    async function confirmBackupRestore() {
	      if (!hasPlatformPermission('platform.backups.manage') || !pendingBackupRestore) return;
	      const backupRunId = Number(pendingBackupRestore.id || 0);
	      const button = document.getElementById('confirmBackupRestore');
	      button.disabled = true;
	      backupStateEl.textContent = '正在执行隔离恢复演练';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/restoreDrill', {
	          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify({ backupRunId }),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        pendingBackupRestore = null;
	        backupRestoreDialogEl.close();
	        await loadBackupOverview();
	        backupStateEl.textContent = '隔离恢复演练已通过';
	        loadSystemHealth();
	        loadOperations();
	      } catch (err) {
	        backupStateEl.textContent = err.message || String(err);
	      } finally {
	        button.disabled = false;
	      }
	    }
	    function complianceStatusState(value) {
	      const states = {
	        active: ['生效', 'ok'], released: ['已解除', ''], expired: ['已过期', ''],
	        pending: ['待处理', 'warning'], running: ['执行中', 'warning'], succeeded: ['已成功', 'ok'],
	        failed: ['失败', 'danger'], deleted: ['已删除', ''], pending_approval: ['待审批', 'warning'],
	        approved: ['已授权', 'ok'], waiting: ['宽限期', 'warning'], blocked: ['已阻断', 'danger'], canceled: ['已取消', ''],
	      };
	      return states[value] || [value || '未知', ''];
	    }
	    function renderComplianceConfig(config) {
	      config = config || {};
	      const states = [
	        ['导出目录', config.artifactRootConfigured], ['租户文件', config.fileStorageConfigured],
	        ['加密密钥 ' + fmt(config.encryptionKeyCount || 0), config.encryptionConfigured],
	      ];
	      complianceConfigStateEl.innerHTML = states.map(item => pill(item[0] + (item[1] ? '就绪' : '未就绪'), item[1] ? 'ok' : 'danger')).join(' ');
	      const unknown = Array.isArray(config.inventoryUnknownTables) ? config.inventoryUnknownTables : [];
	      complianceInventoryStateEl.innerHTML = pill((config.inventoryVersion || '-') + ' / ' + fmt(config.inventoryTableCount || 0) + ' 张表', unknown.length ? 'danger' : 'ok') +
	        (unknown.length ? '<br><span class="bad">未登记：' + esc(unknown.join('、')) + '</span>' : '<br><span class="muted">覆盖门禁通过</span>');
	    }
	    function renderCompliancePolicy(policy) {
	      policy = policy || {};
	      compliancePolicyCache = policy;
	      compliancePolicyStatusInput.value = policy.status || 'active';
	      complianceExportRetentionDaysInput.value = String(policy.exportRetentionDays || 30);
	      complianceErasureGraceDaysInput.value = String(Number(policy.erasureGraceDays || 0));
	      complianceRecentExportDaysInput.value = String(policy.recentExportMaxAgeDays || 7);
	      complianceBillingRetentionDaysInput.value = String(Number(policy.billingRetentionDays || 0));
	      complianceAuditRetentionDaysInput.value = String(Number(policy.auditRetentionDays || 0));
	      complianceServiceAccountUsageRetentionDaysInput.value = String(Number(policy.serviceAccountUsageRetentionDays || 90));
	      complianceRequireRecentExportInput.checked = policy.requireRecentExport !== false;
	      compliancePolicyVersionInput.value = String(policy.version || 0);
	      renderCompliancePolicyApprovalState();
	    }
	    function renderCompliancePolicyApprovalState() {
	      const button = document.getElementById('saveCompliancePolicy');
	      if (button) button.textContent = approvalActionRequired('compliance.policy.update', 0) ? '提交审批' : '保存策略';
	    }
	    function renderComplianceSummary(data) {
	      const holds = data.holds || [];
	      const exports = data.exports || [];
	      const erasures = data.erasures || [];
	      const tiles = [
	        ['活动保留', fmt(holds.filter(item => item.status === 'active').length)],
	        ['成功导出', fmt(exports.filter(item => item.status === 'succeeded').length)],
	        ['导出/删除异常', fmt(exports.filter(item => item.status === 'failed' || item.deletionStatus === 'failed').length)],
	        ['待审批擦除', fmt(erasures.filter(item => item.status === 'pending_approval').length)],
	        ['阻断/失败', fmt(erasures.filter(item => item.status === 'blocked' || item.status === 'failed').length)],
	        ['已完成擦除', fmt(erasures.filter(item => item.status === 'succeeded').length)],
	      ];
	      complianceSummaryEl.innerHTML = tiles.map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
	    }
	    function renderComplianceHolds(items) {
	      items = items || [];
	      complianceHoldCountEl.textContent = fmt(items.length) + ' 条';
	      if (!items.length) {
	        complianceHoldsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无法律保留</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.compliance.manage');
	      complianceHoldsEl.innerHTML = items.map(item => {
	        const state = complianceStatusState(item.status);
	        const action = canManage && item.status === 'active'
	          ? '<button type="button" class="danger" data-compliance-hold-release="' + fmt(item.id) + '" data-hold-no="' + esc(item.holdNo || ('#' + item.id)) + '" data-version="' + fmt(item.version || 0) + '">' + (approvalActionRequired('compliance.legal_hold.release', 0) ? '提交解除审批' : '解除保留') + '</button>'
	          : '<span class="muted">只读</span>';
	        return '<tr><td><strong>' + esc(item.holdNo || ('#' + item.id)) + '</strong><br><span class="muted">#' + fmt(item.id) + '</span></td>' +
	          '<td>#' + fmt(item.tenantId) + ' ' + esc(item.tenantName || '-') + '</td><td>' + pill(state[0], state[1]) + '</td>' +
	          '<td>' + esc(item.reason || '-') + (item.releaseReason ? '<br><span class="muted">解除：' + esc(item.releaseReason) + '</span>' : '') + '</td>' +
	          '<td>' + esc(item.startsAt || '-') + '<br><span class="muted">至 ' + esc(item.expiresAt || '无限期') + '</span></td>' +
	          '<td>v' + fmt(item.version || 0) + '<br><span class="muted">' + esc(item.updatedAt || item.createdAt || '-') + '</span></td>' +
	          '<td><div class="compliance-actions">' + action + '</div></td></tr>';
	      }).join('');
	    }
	    function renderComplianceExports(items) {
	      items = items || [];
	      complianceExportCache = items;
	      complianceExportCountEl.textContent = fmt(items.length) + ' 次';
	      if (!items.length) {
	        complianceExportsEl.innerHTML = '<tr><td colspan="8" class="empty">暂无导出记录</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.compliance.manage');
	      complianceExportsEl.innerHTML = items.map(item => {
	        const state = complianceStatusState(item.status);
	        const deletionState = complianceStatusState(item.deletionStatus);
	        const actions = [];
	        if (canManage && !item.deletionStatus && (item.status === 'pending' || item.status === 'failed')) actions.push('<button type="button" data-compliance-export-action="process" data-export-id="' + fmt(item.id) + '">执行</button>');
	        if (canManage && !item.deletionStatus && item.status === 'succeeded' && item.encryptionKeyReady) actions.push('<button type="button" class="secondary" data-compliance-export-action="download" data-export-id="' + fmt(item.id) + '">下载</button>');
	        if (canManage && !item.deletionStatus && !['pending', 'running', 'deleted'].includes(item.status)) actions.push('<button type="button" class="danger" data-compliance-export-action="delete" data-export-id="' + fmt(item.id) + '">' + (approvalActionRequired('compliance.export.delete', 0) ? '提交删除审批' : '删除') + '</button>');
	        if (canManage && item.deletionStatus === 'pending') actions.push('<button type="button" class="secondary" data-compliance-export-action="delete-retry" data-export-id="' + fmt(item.id) + '">继续删除</button>');
	        if (canManage && item.deletionStatus === 'failed') actions.push('<button type="button" class="danger" data-compliance-export-action="delete-retry" data-export-id="' + fmt(item.id) + '">重试删除</button>');
	        const deletion = item.deletionStatus
	          ? '<br>' + pill('删除：' + deletionState[0], deletionState[1]) + '<span class="muted"> 文件 ' + esc(item.deletionArtifactStatus || '-') + ' / 台账 ' + esc(item.deletionRecordStatus || '-') + ' / 尝试 ' + fmt(item.deletionAttempts || 0) + '</span>'
	          : '';
	        const error = item.deletionLastError || item.errorMessage || '';
	        return '<tr><td><strong>' + esc(item.exportNo || ('#' + item.id)) + '</strong><br><span class="muted">#' + fmt(item.id) + ' / ' + esc(item.inventoryVersion || '-') + '</span></td>' +
	          '<td>#' + fmt(item.tenantId) + ' ' + esc(item.tenantName || '-') + '</td><td>' + pill(state[0], state[1]) + deletion + '</td>' +
	          '<td>' + fmt(item.tableCount || 0) + ' 表 / ' + fmt(item.rowCount || 0) + ' 行<br><span class="muted">' + fmt(item.fileCount || 0) + ' 文件 / ' + backupFormatBytes(item.fileSizeBytes) + '</span></td>' +
	          '<td>' + pill(item.encrypted ? '已加密' : '未加密', item.encrypted ? 'ok' : 'danger') + ' ' + pill(item.encryptionKeyReady ? '密钥可用' : '密钥缺失', item.encryptionKeyReady ? 'ok' : 'danger') +
	          '<br><span class="muted">SHA ' + esc((item.sha256 || '-').slice(0, 12)) + ' / ' + backupFormatBytes(item.sizeBytes) + '</span></td>' +
	          '<td>' + esc(item.finishedAt || item.startedAt || '-') + '<br><span class="muted">至 ' + esc(item.expiresAt || '-') + ' / 下载 ' + fmt(item.downloadCount || 0) + '</span></td>' +
	          '<td>' + (error ? renderInlineErrorDetails(error) : '<span class="muted">-</span>') + (item.deletionApprovalId ? '<br><span class="muted">审批 #' + fmt(item.deletionApprovalId) + '</span>' : '') + '</td>' +
	          '<td><div class="compliance-actions">' + (actions.join('') || '<span class="muted">只读</span>') + '</div></td></tr>';
	      }).join('');
	    }
	    function renderComplianceErasures(items) {
	      items = items || [];
	      complianceErasureCache = items;
	      complianceErasureCountEl.textContent = fmt(items.length) + ' 条';
	      if (!items.length) {
	        complianceErasuresEl.innerHTML = '<tr><td colspan="8" class="empty">暂无擦除记录</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.compliance.manage');
	      complianceErasuresEl.innerHTML = items.map(item => {
	        const state = complianceStatusState(item.status);
	        const total = Number(item.totalSteps || 0);
	        const complete = Number(item.completedSteps || 0);
	        const percent = total > 0 ? Math.min(100, Math.round(complete * 100 / total)) : 0;
	        const actions = [];
	        if (total > 0 || item.status === 'succeeded') actions.push('<button type="button" class="secondary" data-compliance-erasure-action="steps" data-request-id="' + fmt(item.id) + '">步骤</button>');
	        if (canManage && item.status === 'pending_approval') actions.push('<button type="button" data-compliance-erasure-action="approval" data-request-id="' + fmt(item.id) + '">发起/查看审批</button>');
	        if (canManage && ['approved', 'waiting', 'failed', 'blocked', 'running'].includes(item.status)) actions.push('<button type="button" class="danger" data-compliance-erasure-action="process" data-request-id="' + fmt(item.id) + '">执行擦除</button>');
	        if (canManage && ['pending_approval', 'approved', 'waiting', 'failed', 'blocked'].includes(item.status)) actions.push('<button type="button" class="secondary" data-compliance-erasure-action="cancel" data-request-id="' + fmt(item.id) + '">取消</button>');
	        return '<tr><td><strong>' + esc(item.requestNo || ('#' + item.id)) + '</strong><br><span class="muted">#' + fmt(item.id) + ' / ' + esc(item.inventoryVersion || '-') + '</span></td>' +
	          '<td>#' + fmt(item.tenantId) + ' ' + esc(item.tenantName || '-') + '<br><span class="muted">可执行 ' + esc(item.eligibleAt || '-') + '</span></td>' +
	          '<td>' + pill(state[0], state[1]) + '</td><td>' + (item.approvalId ? '审批 #' + fmt(item.approvalId) : '未授权') + '<br><span class="muted">' + esc(item.approvedAt || '-') + '</span></td>' +
	          '<td><strong>' + fmt(complete) + ' / ' + fmt(total) + '</strong><div class="compliance-progress"><span style="width:' + percent + '%"></span></div><span class="muted">' + percent + '%</span></td>' +
	          '<td>删除 ' + fmt(item.deletedRows || 0) + ' / 脱敏 ' + fmt(item.redactedRows || 0) + '<br><span class="muted">SHA ' + esc((item.verificationSha256 || '-').slice(0, 12)) + '</span></td>' +
	          '<td>' + (item.lastError ? renderInlineErrorDetails(item.lastError) : '<span class="muted">-</span>') + '</td>' +
	          '<td><div class="compliance-actions">' + (actions.join('') || '<span class="muted">只读</span>') + '</div></td></tr>';
	      }).join('');
	    }
	    async function loadComplianceOverview() {
	      if (!hasPlatformPermission('platform.compliance.read')) return;
	      complianceStateEl.textContent = '正在加载';
	      const tenantId = Number(complianceTenantFilterInput.value || 0);
	      try {
	        const res = await fetch('/dashboard/saasAdmin/complianceOverview?tenantId=' + tenantId + '&limit=100', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        renderCompliancePolicy(data.policy || {});
	        renderComplianceConfig(data.config || {});
	        renderComplianceSummary(data);
	        renderComplianceHolds(data.holds || []);
	        renderComplianceExports(data.exports || []);
	        renderComplianceErasures(data.erasures || []);
	        if (tenantId > 0) complianceActionTenantIdInput.value = String(tenantId);
	        complianceStateEl.textContent = '已加载 / 策略 v' + fmt((data.policy || {}).version || 0);
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function compliancePost(path, payload) {
	      const res = await fetch(path, {
	        method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	      });
	      const body = await res.json();
	      if (!res.ok || Number(body.code || 0) < 200 || Number(body.code || 0) >= 300) throw new Error(body.msg || 'HTTP ' + res.status);
	      return body.data || {};
	    }
	    function complianceRFC3339(value) {
	      if (!value) return '';
	      const date = new Date(value);
	      return Number.isNaN(date.getTime()) ? '' : date.toISOString();
	    }
	    async function saveCompliancePolicy() {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      const payload = {
	        status: compliancePolicyStatusInput.value,
	        exportRetentionDays: Number(complianceExportRetentionDaysInput.value || 0),
	        erasureGraceDays: Number(complianceErasureGraceDaysInput.value || 0),
	        requireRecentExport: complianceRequireRecentExportInput.checked,
	        recentExportMaxAgeDays: Number(complianceRecentExportDaysInput.value || 0),
	        billingRetentionDays: Number(complianceBillingRetentionDaysInput.value || 0),
	        auditRetentionDays: Number(complianceAuditRetentionDaysInput.value || 0),
	        serviceAccountUsageRetentionDays: Number(complianceServiceAccountUsageRetentionDaysInput.value || 0),
	        expectedVersion: Number(compliancePolicyVersionInput.value || 0),
	      };
	      const requiresApproval = approvalActionRequired('compliance.policy.update', 0);
	      complianceStateEl.textContent = requiresApproval ? '正在提交合规策略变更审批' : '正在保存合规策略';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval('compliance.policy.update', payload, '变更租户数据合规生命周期策略');
	          complianceStateEl.textContent = '合规策略变更已提交双人审批';
	          return;
	        }
	        await compliancePost('/dashboard/saasAdmin/compliancePolicy', payload);
	        await loadComplianceOverview();
	        complianceStateEl.textContent = '合规策略已保存';
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function createComplianceHold() {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      complianceStateEl.textContent = '正在创建法律保留';
	      try {
	        await compliancePost('/dashboard/saasAdmin/complianceLegalHold', {
	          action: 'create', tenantId: Number(complianceActionTenantIdInput.value || 0), reason: complianceActionReasonInput.value.trim(),
	          startsAt: complianceRFC3339(complianceHoldStartsAtInput.value), expiresAt: complianceRFC3339(complianceHoldExpiresAtInput.value),
	        });
	        await loadComplianceOverview();
	        complianceStateEl.textContent = '法律保留已创建';
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function releaseComplianceHold(button) {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      const reason = window.prompt('请输入解除法律保留的依据');
	      if (!reason) return;
	      const payload = {
	        holdId: Number(button.dataset.complianceHoldRelease || 0), reason: reason.trim(), expectedVersion: Number(button.dataset.version || 0),
	      };
	      const requiresApproval = approvalActionRequired('compliance.legal_hold.release', 0);
	      complianceStateEl.textContent = requiresApproval ? '正在冻结法律保留快照并提交解除审批' : '正在解除法律保留';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval('compliance.legal_hold.release', payload, '解除法律保留：' + (button.dataset.holdNo || payload.holdId));
	        } else {
	          await compliancePost('/dashboard/saasAdmin/complianceLegalHold', Object.assign({ action: 'release' }, payload));
	        }
	        await loadComplianceOverview();
	        complianceStateEl.textContent = requiresApproval ? '法律保留解除审批已提交' : '法律保留已解除';
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function requestComplianceExport() {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      complianceStateEl.textContent = '正在创建租户数据导出';
	      try {
	        await compliancePost('/dashboard/saasAdmin/complianceExport', {
	          action: 'request', tenantId: Number(complianceActionTenantIdInput.value || 0), reason: complianceActionReasonInput.value.trim(),
	        });
	        await loadComplianceOverview();
	        complianceStateEl.textContent = '加密导出已进入队列';
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function runComplianceExportAction(action, exportId) {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      if (action === 'delete' && !window.confirm('确认提交该加密导出工件的双人删除审批？审批通过后会冻结并删除工件。')) return;
	      complianceStateEl.textContent = action === 'process' ? '正在生成加密导出' : (action === 'delete-retry' ? '正在重试已审批删除任务' : '正在冻结导出快照并提交删除审批');
	      try {
	        if (action === 'delete' && approvalActionRequired('compliance.export.delete', 0)) {
	          await requestHighRiskApproval('compliance.export.delete', { exportId: Number(exportId || 0) }, '删除合规导出加密工件');
	        } else {
	          await compliancePost('/dashboard/saasAdmin/complianceExport', { action, exportId: Number(exportId || 0) });
	        }
	        await loadComplianceOverview();
	        complianceStateEl.textContent = action === 'process' ? '加密导出已生成并校验' : (action === 'delete-retry' ? '已重试删除任务' : (approvalActionRequired('compliance.export.delete', 0) ? '删除审批已提交' : '导出工件已删除'));
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	        await loadComplianceOverview();
	      }
	    }
	    async function downloadComplianceExport(exportId) {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      complianceStateEl.textContent = '正在校验并解密导出';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/complianceExportDownload?exportId=' + Number(exportId || 0), { headers: authHeader() });
	        if (!res.ok) {
	          const body = await res.json();
	          throw new Error(body.msg || 'HTTP ' + res.status);
	        }
	        const blob = await res.blob();
	        const disposition = res.headers.get('Content-Disposition') || '';
	        const match = disposition.match(/filename="([^"]+)"/);
	        const link = document.createElement('a');
	        link.href = URL.createObjectURL(blob);
	        link.download = match ? match[1] : ('mochat-compliance-export-' + exportId + '.tar.gz');
	        document.body.appendChild(link);
	        link.click();
	        link.remove();
	        URL.revokeObjectURL(link.href);
	        complianceStateEl.textContent = '完整性校验通过，已生成解密包';
	        loadComplianceOverview();
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function requestComplianceErasure() {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      const tenantId = Number(complianceActionTenantIdInput.value || 0);
	      const confirmation = complianceErasureConfirmationInput.value.trim();
	      const reason = complianceActionReasonInput.value.trim();
	      if (!window.confirm('确认为租户 #' + tenantId + ' 创建不可逆数据擦除申请？')) return;
	      complianceStateEl.textContent = '正在创建擦除申请和双人审批单';
	      try {
	        const data = await compliancePost('/dashboard/saasAdmin/complianceErasure', { action: 'request', tenantId, confirmation, reason });
	        const erasure = data.erasure || {};
	        await requestComplianceErasureApproval(erasure, reason, false);
	        complianceErasureConfirmationInput.value = '';
	        await loadComplianceOverview();
	        complianceStateEl.textContent = '擦除申请已创建，等待两人复核';
	        if (hasPlatformPermission('platform.approvals.read')) loadApprovalPolicies();
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	        await loadComplianceOverview();
	      }
	    }
	    async function requestComplianceErasureApproval(erasure, reason, refresh) {
	      erasure = erasure || {};
	      const requestId = Number(erasure.id || 0);
	      if (!requestId) throw new Error('擦除请求 ID 缺失');
	      const data = await compliancePost('/dashboard/saasAdmin/approvalRequest', {
	        actionType: 'tenant.data.erase', payload: { requestId }, reason: String(reason || erasure.reason || '租户数据擦除复核').slice(0, 255),
	        idempotencyKey: 'compliance-erasure-' + requestId,
	      });
	      if (refresh !== false) {
	        complianceStateEl.textContent = '擦除审批单已创建或已存在';
	        if (hasPlatformPermission('platform.approvals.read')) await loadApprovalPolicies();
	      }
	      return data;
	    }
	    async function recoverComplianceErasureApproval(requestId) {
	      const item = complianceErasureCache.find(erasure => Number(erasure.id) === Number(requestId));
	      if (!item) return;
	      complianceStateEl.textContent = '正在幂等创建擦除审批单';
	      try {
	        await requestComplianceErasureApproval(item, item.reason, true);
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function runComplianceErasureAction(action, requestId) {
	      if (!hasPlatformPermission('platform.compliance.manage')) return;
	      let reason = '';
	      if (action === 'process' && !window.confirm('确认执行不可逆擦除？系统会再次检查审批、宽限期、法律保留和财务保留期。')) return;
	      if (action === 'cancel') {
	        reason = window.prompt('请输入取消擦除申请的原因') || '';
	        if (!reason.trim()) return;
	      }
	      complianceStateEl.textContent = action === 'process' ? '正在执行可恢复擦除步骤' : '正在取消擦除申请';
	      try {
	        await compliancePost('/dashboard/saasAdmin/complianceErasure', { action, requestId: Number(requestId || 0), reason: reason.trim() });
	        await loadComplianceOverview();
	        complianceStateEl.textContent = action === 'process' ? '擦除已完成并生成验证摘要' : '擦除申请已取消';
	        loadSystemHealth();
	        loadOperations();
	      } catch (err) {
	        complianceStateEl.textContent = err.message || String(err);
	        await loadComplianceOverview();
	      }
	    }
	    async function loadComplianceErasureSteps(requestId) {
	      complianceStepsStateEl.textContent = '正在加载';
	      complianceStepsEl.innerHTML = '<tr><td colspan="6" class="empty">正在加载步骤</td></tr>';
	      complianceStepsDialogEl.showModal();
	      try {
	        const res = await fetch('/dashboard/saasAdmin/complianceErasureSteps?requestId=' + Number(requestId || 0), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const steps = (body.data || {}).steps || [];
	        complianceStepsStateEl.textContent = fmt(steps.length) + ' 步 / 完成 ' + fmt(steps.filter(item => item.status === 'succeeded').length);
	        complianceStepsEl.innerHTML = steps.length ? steps.map(item => {
	          const state = complianceStatusState(item.status);
	          return '<tr><td>' + fmt(item.stepOrder || 0) + '</td><td><strong>' + esc(item.stepKey || '-') + '</strong><br><span class="muted">' + esc(item.tableName || '-') + '</span></td>' +
	            '<td>' + esc(item.action || '-') + '</td><td>' + pill(state[0], state[1]) + '</td><td>' + fmt(item.affectedRows || 0) + '</td>' +
	            '<td>' + esc(item.finishedAt || item.startedAt || '-') + (item.errorMessage ? renderInlineErrorDetails(item.errorMessage) : '') + '</td></tr>';
	        }).join('') : '<tr><td colspan="6" class="empty">暂无步骤</td></tr>';
	      } catch (err) {
	        complianceStepsStateEl.textContent = err.message || String(err);
	        complianceStepsEl.innerHTML = '<tr><td colspan="6" class="empty">步骤加载失败</td></tr>';
	      }
	    }
	    function applyWeComCredentialRotationControl() {
	      const canManage = hasPlatformPermission('platform.integrations.manage');
	      const canRotate = weComCredentialProtection.rotationAvailable === true && Number(weComCredentialProtection.rotationRequiredCount || 0) > 0;
	      rotateWeComCredentialsButton.disabled = !canManage || !canRotate;
	      if (!canManage) rotateWeComCredentialsButton.title = '缺少平台权限 platform.integrations.manage';
	      else if (!weComCredentialProtection.encryptionConfigured) rotateWeComCredentialsButton.title = '未配置企业微信凭据加密密钥';
	      else if (!canRotate) rotateWeComCredentialsButton.title = '当前没有待轮换凭据';
	      else rotateWeComCredentialsButton.removeAttribute('title');
	    }
	    function renderWeComCredentialProtection(protection) {
	      weComCredentialProtection = protection || {};
	      const unavailable = Number(weComCredentialProtection.unavailableKeyCount || 0);
	      const decryptFailures = Number(weComCredentialProtection.decryptFailureCount || 0);
	      const configured = Number(weComCredentialProtection.configuredCredentialCount || 0);
	      const encrypted = Number(weComCredentialProtection.encryptedCredentialCount || 0);
	      const tiles = [
	        ['凭据总数', fmt(configured)],
	        ['企业凭据', fmt(weComCredentialProtection.corpCredentialCount || 0)],
	        ['应用凭据', fmt(weComCredentialProtection.agentCredentialCount || 0)],
	        ['加密覆盖', fmt(encrypted) + ' / ' + fmt(configured)],
	        ['历史明文', fmt(weComCredentialProtection.legacyPlaintextCount || 0)],
	        ['待轮换', fmt(weComCredentialProtection.rotationRequiredCount || 0)],
	        ['活动密钥', esc(weComCredentialProtection.activeKeyId || '-')],
	        ['异常', fmt(unavailable + decryptFailures)],
	      ];
	      weComCredentialSummaryEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	      if (weComCredentialProtection.supported !== true) {
	        weComCredentialStateEl.textContent = '当前运行模式不支持';
	      } else if (!weComCredentialProtection.encryptionConfigured) {
	        weComCredentialStateEl.textContent = '未配置加密密钥';
	      } else if (unavailable > 0 || decryptFailures > 0) {
	        const missing = Array.isArray(weComCredentialProtection.unavailableKeyIds) ? weComCredentialProtection.unavailableKeyIds.join('、') : '';
	        weComCredentialStateEl.textContent = '密钥异常 ' + fmt(unavailable) + ' / 解密失败 ' + fmt(decryptFailures) + (missing ? ' / 缺失 ' + missing : '');
	      } else if (Number(weComCredentialProtection.legacyPlaintextCount || 0) > 0) {
	        weComCredentialStateEl.textContent = '发现历史明文 ' + fmt(weComCredentialProtection.legacyPlaintextCount) + ' 条';
	      } else if (Number(weComCredentialProtection.rotationRequiredCount || 0) > 0) {
	        weComCredentialStateEl.textContent = '待轮换 ' + fmt(weComCredentialProtection.rotationRequiredCount) + ' 条';
	      } else {
	        weComCredentialStateEl.textContent = '密钥环正常 / ' + esc(weComCredentialProtection.activeKeyId || '-');
	      }
	      applyWeComCredentialRotationControl();
	    }
	    async function loadWeComCredentialProtection() {
	      if (!hasPlatformPermission('platform.integrations.read')) return;
	      weComCredentialStateEl.textContent = '正在加载';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/wecomCredentialProtection', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        renderWeComCredentialProtection((body.data || {}).credentialProtection || {});
	      } catch (err) {
	        weComCredentialProtection = {};
	        weComCredentialStateEl.textContent = err.message || String(err);
	        weComCredentialSummaryEl.innerHTML = '';
	        applyWeComCredentialRotationControl();
	      }
	    }
	    async function rotateWeComCredentials() {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      const tenantValue = weComCredentialTenantIdInput.value.trim();
	      const tenantId = tenantValue ? Number(tenantValue) : 0;
	      const limit = Number(weComCredentialLimitInput.value || 0);
	      if (!Number.isInteger(tenantId) || tenantId < 0) {
	        weComCredentialStateEl.textContent = '租户 ID 必须是非负整数';
	        return;
	      }
	      if (!Number.isInteger(limit) || limit < 1 || limit > 1000) {
	        weComCredentialStateEl.textContent = '批量上限必须是 1 到 1000 的整数';
	        return;
	      }
	      rotateWeComCredentialsButton.disabled = true;
	      weComCredentialStateEl.textContent = '正在轮换';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/wecomCredentialRotation', {
	          method: 'POST',
	          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	          body: JSON.stringify({ tenantId, limit, remark: '总后台轮换企业微信凭据密钥' }),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        const rotation = data.rotation || {};
	        renderWeComCredentialProtection(data.credentialProtection || {});
	        weComCredentialStateEl.textContent = '已轮换 ' + fmt(rotation.rotatedCount || 0) + ' 条（企业 ' + fmt(rotation.corpRotatedCount || 0) + ' / 应用 ' + fmt(rotation.agentRotatedCount || 0) + '），剩余 ' + fmt(weComCredentialProtection.rotationRequiredCount || 0) + ' 条';
	        void loadTenantReadiness();
	        if (hasPlatformPermission('platform.system.read')) loadSystemHealth();
	        if (hasPlatformPermission('platform.audit.read')) loadOperations();
	      } catch (err) {
	        weComCredentialStateEl.textContent = err.message || String(err);
	      } finally {
	        applyWeComCredentialRotationControl();
	      }
	    }
	    function applyWeChatOpenCredentialRotationControl() {
	      const canManage = hasPlatformPermission('platform.integrations.manage');
	      const canRotate = weChatOpenCredentialProtection.rotationAvailable === true && Number(weChatOpenCredentialProtection.rotationRequiredCount || 0) > 0;
	      rotateWeChatOpenCredentialsButton.disabled = !canManage || !canRotate;
	      if (!canManage) rotateWeChatOpenCredentialsButton.title = '缺少平台权限 platform.integrations.manage';
	      else if (!weChatOpenCredentialProtection.encryptionConfigured) rotateWeChatOpenCredentialsButton.title = '未配置微信开放平台凭据加密密钥';
	      else if (!canRotate) rotateWeChatOpenCredentialsButton.title = '当前没有待轮换凭据';
	      else rotateWeChatOpenCredentialsButton.removeAttribute('title');
	    }
	    function renderWeChatOpenCredentialProtection(protection) {
	      weChatOpenCredentialProtection = protection || {};
	      const unavailable = Number(weChatOpenCredentialProtection.unavailableKeyCount || 0);
	      const decryptFailures = Number(weChatOpenCredentialProtection.decryptFailureCount || 0);
	      const configured = Number(weChatOpenCredentialProtection.configuredCredentialCount || 0);
	      const encrypted = Number(weChatOpenCredentialProtection.encryptedCredentialCount || 0);
	      const tiles = [
	        ['凭据总数', fmt(configured)],
	        ['平台 Ticket', fmt(weChatOpenCredentialProtection.componentTicketCount || 0)],
	        ['公众号授权', fmt(weChatOpenCredentialProtection.officialAccountCount || 0)],
	        ['加密覆盖', fmt(encrypted) + ' / ' + fmt(configured)],
	        ['历史明文', fmt(weChatOpenCredentialProtection.legacyPlaintextCount || 0)],
	        ['待轮换', fmt(weChatOpenCredentialProtection.rotationRequiredCount || 0)],
	        ['活动密钥', esc(weChatOpenCredentialProtection.activeKeyId || '-')],
	        ['异常', fmt(unavailable + decryptFailures)],
	      ];
	      weChatOpenCredentialSummaryEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	      if (weChatOpenCredentialProtection.supported !== true) {
	        weChatOpenCredentialStateEl.textContent = '当前运行模式不支持';
	      } else if (!weChatOpenCredentialProtection.encryptionConfigured) {
	        weChatOpenCredentialStateEl.textContent = '未配置加密密钥';
	      } else if (unavailable > 0 || decryptFailures > 0) {
	        const missing = Array.isArray(weChatOpenCredentialProtection.unavailableKeyIds) ? weChatOpenCredentialProtection.unavailableKeyIds.join('、') : '';
	        weChatOpenCredentialStateEl.textContent = '密钥异常 ' + fmt(unavailable) + ' / 解密失败 ' + fmt(decryptFailures) + (missing ? ' / 缺失 ' + missing : '');
	      } else if (Number(weChatOpenCredentialProtection.legacyPlaintextCount || 0) > 0) {
	        weChatOpenCredentialStateEl.textContent = '发现历史明文 ' + fmt(weChatOpenCredentialProtection.legacyPlaintextCount) + ' 条';
	      } else if (Number(weChatOpenCredentialProtection.rotationRequiredCount || 0) > 0) {
	        weChatOpenCredentialStateEl.textContent = '待轮换 ' + fmt(weChatOpenCredentialProtection.rotationRequiredCount) + ' 条';
	      } else {
	        weChatOpenCredentialStateEl.textContent = '密钥环正常 / ' + esc(weChatOpenCredentialProtection.activeKeyId || '-');
	      }
	      applyWeChatOpenCredentialRotationControl();
	    }
	    async function loadWeChatOpenCredentialProtection() {
	      if (!hasPlatformPermission('platform.integrations.read')) return;
	      weChatOpenCredentialStateEl.textContent = '正在加载';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/wechatOpenCredentialProtection', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        renderWeChatOpenCredentialProtection((body.data || {}).credentialProtection || {});
	      } catch (err) {
	        weChatOpenCredentialProtection = {};
	        weChatOpenCredentialStateEl.textContent = err.message || String(err);
	        weChatOpenCredentialSummaryEl.innerHTML = '';
	        applyWeChatOpenCredentialRotationControl();
	      }
	    }
	    async function rotateWeChatOpenCredentials() {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      const tenantValue = weChatOpenCredentialTenantIdInput.value.trim();
	      const tenantId = tenantValue ? Number(tenantValue) : 0;
	      const limit = Number(weChatOpenCredentialLimitInput.value || 0);
	      if (!Number.isInteger(tenantId) || tenantId < 0) {
	        weChatOpenCredentialStateEl.textContent = '租户 ID 必须是非负整数';
	        return;
	      }
	      if (!Number.isInteger(limit) || limit < 1 || limit > 1000) {
	        weChatOpenCredentialStateEl.textContent = '批量上限必须是 1 到 1000 的整数';
	        return;
	      }
	      rotateWeChatOpenCredentialsButton.disabled = true;
	      weChatOpenCredentialStateEl.textContent = '正在轮换';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/wechatOpenCredentialRotation', {
	          method: 'POST',
	          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	          body: JSON.stringify({ tenantId, limit, remark: '总后台轮换微信开放平台凭据密钥' }),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        const rotation = data.rotation || {};
	        renderWeChatOpenCredentialProtection(data.credentialProtection || {});
	        weChatOpenCredentialStateEl.textContent = '已轮换 ' + fmt(rotation.rotatedCount || 0) + ' 条（Ticket ' + fmt(rotation.componentTicketRotatedCount || 0) + ' / 公众号 ' + fmt(rotation.officialAccountRotatedCount || 0) + '），剩余 ' + fmt(weChatOpenCredentialProtection.rotationRequiredCount || 0) + ' 条';
	        if (hasPlatformPermission('platform.system.read')) loadSystemHealth();
	        if (hasPlatformPermission('platform.audit.read')) loadOperations();
	      } catch (err) {
	        weChatOpenCredentialStateEl.textContent = err.message || String(err);
	      } finally {
	        applyWeChatOpenCredentialRotationControl();
	      }
	    }
	    function serviceAccountStatusState(value) {
	      if (value === 'active') return ['启用', 'ok'];
	      if (value === 'disabled') return ['停用', 'danger'];
	      return [value || '未知', ''];
	    }
	    function serviceAccountKeyRetireExpired(key) {
	      const raw = String((key || {}).retireAt || '').trim();
	      if (!raw) return false;
	      const timestamp = Date.parse(raw.includes('T') ? raw : raw.replace(' ', 'T'));
	      return Number.isFinite(timestamp) && timestamp <= Date.now();
	    }
	    function serviceAccountKeyStatusState(key) {
	      const value = (key || {}).status;
	      if (value === 'active') return ['生效', 'ok'];
	      if (value === 'retiring') return serviceAccountKeyRetireExpired(key) ? ['宽限已结束', 'danger'] : ['宽限中', 'warning'];
	      if (value === 'revoked') return ['已吊销', 'danger'];
	      return [value || '未知', ''];
	    }
	    function serviceAccountDateTimeAfter(days) {
	      const date = new Date(Date.now() + Number(days || 0) * 86400000);
	      const pad = value => String(value).padStart(2, '0');
	      return date.getFullYear() + '-' + pad(date.getMonth() + 1) + '-' + pad(date.getDate()) + ' ' + pad(date.getHours()) + ':' + pad(date.getMinutes()) + ':' + pad(date.getSeconds());
	    }
	    function selectedServiceAccountScopes() {
	      return [...serviceAccountScopesEl.querySelectorAll('input:checked')].map(input => input.value);
	    }
	    function renderServiceAccountScopeOptions(selected) {
	      const selectedSet = new Set(selected || []);
	      serviceAccountScopesEl.innerHTML = serviceAccountScopeCatalog.map(item =>
	        '<label title="' + esc(item.description || '') + '"><input type="checkbox" value="' + esc(item.code) + '"' + (selectedSet.has(item.code) ? ' checked' : '') + '><span>' + esc(item.name || item.code) + '</span></label>'
	      ).join('');
	    }
	    function serviceAccountCIDRs() {
	      return serviceAccountAllowedCidrsInput.value.split(/[\n,]+/).map(value => value.trim()).filter(Boolean);
	    }
	    function renderServiceAccountSummary(summary, protection, clientIPResolution) {
	      summary = summary || {};
	      protection = protection || {};
	      clientIPResolution = clientIPResolution || {};
	      const missingKeyIds = Array.isArray(protection.missingKeyIds) ? protection.missingKeyIds : [];
	      const tiles = [
	        ['服务账号', fmt(summary.accountCount || 0)],
	        ['账号状态', fmt(summary.activeAccountCount || 0) + ' 启用 / ' + fmt(summary.disabledAccountCount || 0) + ' 停用'],
	        ['API Key', fmt(summary.keyCount || 0) + ' 枚'],
	        ['密钥台账', fmt(summary.activeKeyCount || 0) + ' 生效 / ' + fmt(summary.retiringKeyCount || 0) + ' 宽限记录 / ' + fmt(summary.revokedKeyCount || 0) + ' 吊销'],
	        ['pepper 主密钥', esc(protection.activeKeyId || '未配置') + ' / ' + (protection.dedicatedConfigured ? '独立' : '兼容 JWT')],
	        ['待轮换旧 Key', fmt(protection.legacyUsableKeyCount || 0) + ' 枚'],
	        ['pepper 异常', missingKeyIds.length ? esc(missingKeyIds.join('、')) : '0'],
	        ['来源 IP 解析', clientIPResolution.trustProxyHeaders ? '受信代理 / ' + fmt(clientIPResolution.trustedProxyCidrCount || 0) + ' 段' : 'TCP 直连'],
	        ['今日成功请求', fmt(summary.todayRequestCount || 0)],
	        ['今日限流拒绝', fmt(summary.todayRejectedCount || 0)],
	        ['当前分钟请求', fmt(summary.currentMinuteRequestCount || 0)],
	        ['触发限流账号', fmt(summary.limitedAccountCount || 0)],
	        ['启用用量预警', fmt(summary.usageAlertAccountCount || 0)],
	      ];
	      serviceAccountSummaryEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	    }
	    function renderServiceAccountUsageAccountOptions(items) {
	      const selected = Number(serviceAccountUsageAccountFilterInput.value || 0);
	      serviceAccountUsageAccountFilterInput.innerHTML = '<option value="0">全部账号</option>' + (items || []).map(item =>
	        '<option value="' + fmt(item.id) + '">' + esc((item.name || item.code) + ' / ' + (item.tenantName || ('租户 ' + item.tenantId))) + '</option>'
	      ).join('');
	      serviceAccountUsageAccountFilterInput.value = (items || []).some(item => Number(item.id) === selected) ? String(selected) : '0';
	    }
	    function renderServiceAccountUsage(report) {
	      report = report || {};
	      const requests = Number(report.requestCount || 0);
	      const rejected = Number(report.rejectedCount || 0);
	      const total = requests + rejected;
	      const rejectionRate = total > 0 ? (rejected * 100 / total).toFixed(2) + '%' : '0.00%';
	      const retention = Number(report.retentionDays || 0);
	      const summary = [
	        ['成功请求', fmt(requests)],
	        ['限流拒绝', fmt(rejected) + ' / ' + rejectionRate],
	        ['活跃账号', fmt(report.activeAccountCount || 0)],
	        ['受限账号', fmt(report.limitedAccountCount || 0)],
	        ['打开预警', fmt(report.openAlertCount || 0)],
	        ['用量保留', retention ? fmt(retention) + ' 天' : '-'],
	        ['最早记录', esc(report.oldestUsageDate || '暂无') + ' / ' + fmt(report.storedRowCount || 0) + ' 行'],
	      ];
	      serviceAccountUsageSummaryEl.innerHTML = summary.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');

	      const daily = Array.isArray(report.daily) ? report.daily : [];
	      const maximum = Math.max(1, ...daily.map(item => Number(item.requestCount || 0) + Number(item.rejectedCount || 0)));
	      serviceAccountUsageChartEl.style.minWidth = Math.max(680, daily.length * (daily.length > 30 ? 13 : 28)) + 'px';
	      serviceAccountUsageChartEl.innerHTML = daily.map((item, index) => {
	        const success = Number(item.requestCount || 0);
	        const denied = Number(item.rejectedCount || 0);
	        const requestHeight = Math.max(success > 0 ? 2 : 0, Math.round((success + denied) * 100 / maximum));
	        const rejectedHeight = Math.max(denied > 0 ? 2 : 0, Math.round(denied * 100 / maximum));
	        const showDate = daily.length <= 14 || index === 0 || index === daily.length - 1 || index % 7 === 0;
	        const label = String(item.usageDate || '').slice(5);
	        const title = esc((item.usageDate || '-') + '：' + fmt(success) + ' 成功 / ' + fmt(denied) + ' 拒绝 / ' + fmt(item.activeAccountCount || 0) + ' 账号');
	        return '<div class="service-account-chart-day" title="' + title + '"><div class="service-account-chart-bars"><span class="service-account-chart-request" style="height:' + requestHeight + '%"></span><span class="service-account-chart-rejected" style="height:' + rejectedHeight + '%"></span></div><span class="service-account-chart-date">' + (showDate ? esc(label) : '') + '</span></div>';
	      }).join('');

	      const routes = Array.isArray(report.routes) ? report.routes : [];
	      serviceAccountUsageRouteCountEl.textContent = '前 ' + fmt(routes.length) + ' 条';
	      serviceAccountUsageRoutesEl.innerHTML = routes.length ? routes.map(item =>
	        '<tr><td><strong>' + esc(item.routeKey || '-') + '</strong></td><td>' + fmt(item.requestCount || 0) + '</td><td>' + fmt(item.rejectedCount || 0) + '</td><td>' + fmt(item.activeAccountCount || 0) + '</td><td>' + esc(item.lastUsedAt || '-') + '</td></tr>'
	      ).join('') : '<tr><td colspan="5" class="empty">当前范围暂无路由用量</td></tr>';

	      const accounts = Array.isArray(report.accounts) ? report.accounts : [];
	      serviceAccountUsageAccountCountEl.textContent = '前 ' + fmt(accounts.length) + ' 个';
	      serviceAccountUsageAccountsEl.innerHTML = accounts.length ? accounts.map(item =>
	        '<tr><td><strong>' + esc(item.serviceAccountName || item.serviceAccountCode || '-') + '</strong><br><span class="muted">' + esc(item.serviceAccountCode || '-') + ' / #' + fmt(item.serviceAccountId || 0) + '</span></td><td>' + esc(item.tenantName || ('租户 ' + item.tenantId)) + '<br><span class="muted">ID ' + fmt(item.tenantId || 0) + '</span></td><td>' + fmt(item.requestCount || 0) + '</td><td>' + fmt(item.rejectedCount || 0) + '</td><td>' + fmt(item.routeCount || 0) + '</td><td>' + esc(item.lastUsedAt || '-') + '</td></tr>'
	      ).join('') : '<tr><td colspan="6" class="empty">当前范围暂无账号用量</td></tr>';
	    }
	    async function loadServiceAccountUsage() {
	      if (!hasPlatformPermission('platform.integrations.read')) return;
	      serviceAccountUsageStateEl.textContent = '正在加载';
	      try {
	        const params = new URLSearchParams({ days: String(serviceAccountUsageDays), limit: '10' });
	        if (serviceAccountTenantFilterInput.value.trim()) params.set('tenantId', serviceAccountTenantFilterInput.value.trim());
	        if (Number(serviceAccountUsageAccountFilterInput.value || 0) > 0) params.set('serviceAccountId', serviceAccountUsageAccountFilterInput.value);
	        const res = await fetch('/dashboard/saasAdmin/serviceAccountUsage?' + params.toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const report = body.data || {};
	        renderServiceAccountUsage(report);
	        serviceAccountUsageStateEl.textContent = (report.dateFrom || '-') + ' 至 ' + (report.dateTo || '-');
	      } catch (err) {
	        serviceAccountUsageStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function evaluateServiceAccountUsageAlerts() {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      const payload = { limit: 100 };
	      if (serviceAccountTenantFilterInput.value.trim()) payload.tenantId = Number(serviceAccountTenantFilterInput.value.trim());
	      if (Number(serviceAccountUsageAccountFilterInput.value || 0) > 0) payload.serviceAccountId = Number(serviceAccountUsageAccountFilterInput.value);
	      serviceAccountUsageStateEl.textContent = '正在评估预警';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate', {
	          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const result = body.data || {};
	        await loadServiceAccounts();
	        loadAlerts();
	        loadNotifications();
	        loadOperations();
	        serviceAccountUsageStateEl.textContent = '已评估 ' + fmt(result.scannedAccounts || 0) + ' 个，预警 ' + fmt(Number(result.usageWarningAccounts || 0) + Number(result.rejectionWarningAccounts || 0)) + ' 个，入队 ' + fmt(result.notificationsQueued || 0) + ' 条，关闭过期通知 ' + fmt(result.notificationsClosed || 0) + ' 条';
	      } catch (err) {
	        serviceAccountUsageStateEl.textContent = err.message || String(err);
	      }
	    }
	    function viewServiceAccountUsageAlerts() {
	      if (!hasPlatformPermission('platform.notifications.read')) return;
	      alertStatusInput.value = 'open';
	      alertMetricInput.value = 'service_account_usage';
	      alertTypeInput.value = '';
	      loadAlerts();
	      const section = alertsEl.closest('section');
	      if (section) section.scrollIntoView({ behavior: 'smooth', block: 'start' });
	    }
	    function renderServiceAccounts(items) {
	      items = items || [];
	      serviceAccountCountEl.textContent = '显示 ' + fmt(items.length) + ' 个';
	      if (!items.length) {
	        serviceAccountsEl.innerHTML = '<tr><td colspan="7" class="empty">当前筛选下无服务账号</td></tr>';
	        return;
	      }
	      const canManage = hasPlatformPermission('platform.integrations.manage');
	      serviceAccountsEl.innerHTML = items.map(item => {
	        const accountState = serviceAccountStatusState(item.status);
	        const scopeNames = (item.scopes || []).map(code => {
	          const scope = serviceAccountScopeCatalog.find(entry => entry.code === code);
	          return scope ? scope.name : code;
	        });
	        const keys = (item.keys || []).map(key => {
	          const keyState = serviceAccountKeyStatusState(key);
	          const revoke = canManage && key.status !== 'revoked'
	            ? '<div class="service-account-actions"><button type="button" class="danger" data-service-account-action="revoke" data-account-id="' + fmt(item.id) + '" data-key-id="' + fmt(key.id) + '" data-key-version="' + fmt(key.version) + '">吊销</button></div>'
	            : '';
	          const pepper = key.legacyPepper ? pill('旧 JWT pepper', 'warning') : '<span class="pill ok">' + esc(key.hashKeyId || '-') + '</span>';
	          const retireText = key.retireAt ? (serviceAccountKeyRetireExpired(key) ? ' / 宽限结束 ' : ' / 宽限至 ') + esc(key.retireAt) : '';
	          return '<div class="service-account-key-row"><strong>' + esc(key.name || 'API Key') + '</strong> ' + pill(keyState[0], keyState[1]) + ' ' + pepper + '<br><span>' + esc(key.display || '-') + '</span><br><span class="muted">过期 ' + esc(key.expiresAt || '不过期') + ' / 使用 ' + fmt(key.useCount || 0) + ' 次' + retireText + '</span>' + revoke + '</div>';
	        }).join('') || '<span class="muted">无密钥</span>';
	        const actions = canManage
	          ? '<div class="service-account-actions"><button type="button" class="secondary" data-service-account-action="edit" data-account-id="' + fmt(item.id) + '">编辑</button><button type="button" data-service-account-action="rotate" data-account-id="' + fmt(item.id) + '">轮换</button></div>'
	          : '<span class="muted">只读</span>';
	        const dailyLimit = Number(item.dailyRequestLimit || 0);
	        const minuteLimit = Number(item.rateLimitPerMinute || 0);
	        const routeUsage = (item.todayRoutes || []).slice(0, 3).map(route =>
	          '<span class="muted">' + esc(route.routeKey || '-') + '：' + fmt(route.requestCount || 0) + ' 成功 / ' + fmt(route.rejectedCount || 0) + ' 拒绝</span>'
	        ).join('');
	        const usage = '<div class="service-account-usage"><strong>今日 ' + fmt(item.dailyRequestCount || 0) + ' / ' + (dailyLimit > 0 ? fmt(dailyLimit) : '不限') + '</strong><br>' +
	          '<span class="muted">本分钟 ' + fmt(item.minuteRequestCount || 0) + ' / ' + fmt(minuteLimit) + '，今日拒绝 ' + fmt(item.dailyRejectedCount || 0) + '</span>' +
	          '<br><span class="muted">预警 ' + (item.usageAlertEnabled === false ? '关闭' : ('用量 ' + fmt(item.usageWarningPercent || 80) + '% / 拒绝 ' + fmt(item.rejectionWarningCount == null ? 1 : item.rejectionWarningCount) + ' / 冷却 ' + fmt(item.usageAlertCooldownMinutes || 60) + ' 分钟')) + '</span>' +
	          (routeUsage ? '<div class="service-account-route-list">' + routeUsage + '</div>' : '<br><span class="muted">今日暂无路由调用</span>') +
	          '<br><span class="muted">最近评估 ' + esc(item.usageAlertLastEvaluatedAt || '尚未评估') + '，最近通知 ' + esc(item.usageAlertLastNotifiedAt || item.rejectionAlertLastNotifiedAt || '尚未通知') + '</span>' +
	          '<br><span class="muted">累计成功 ' + fmt(item.useCount || 0) + '，最近 ' + esc(item.lastUsedAt || '尚未使用') + (item.lastUsedIp ? ' / ' + esc(item.lastUsedIp) : '') + '</span></div>';
	        return '<tr><td><strong>' + esc(item.name || item.code) + '</strong><br><span class="muted">' + esc(item.code) + ' / #' + fmt(item.id) + ' / v' + fmt(item.version) + '</span><br><span class="muted">' + esc(item.description || '-') + '</span></td>' +
	          '<td><strong>' + esc(item.tenantName || ('租户 ' + item.tenantId)) + '</strong><br><span class="muted">ID ' + fmt(item.tenantId) + '</span></td>' +
	          '<td>' + pill(accountState[0], accountState[1]) + '<br><span class="muted">' + esc(scopeNames.join('、') || '无作用域') + '</span></td>' +
	          '<td>' + esc((item.allowedCidrs || []).join(', ') || '不限制') + '<br><span class="muted">账号过期 ' + esc(item.expiresAt || '不过期') + '</span></td>' +
	          '<td>' + usage + '</td>' +
	          '<td><div class="service-account-key-list">' + keys + '</div></td><td>' + actions + '</td></tr>';
	      }).join('');
	    }
	    async function loadServiceAccounts() {
	      if (!hasPlatformPermission('platform.integrations.read')) return;
	      serviceAccountStateEl.textContent = '正在加载';
	      try {
	        const params = new URLSearchParams({ status: serviceAccountStatusFilterInput.value || 'all', limit: '100' });
	        if (serviceAccountTenantFilterInput.value.trim()) params.set('tenantId', serviceAccountTenantFilterInput.value.trim());
	        if (serviceAccountKeywordInput.value.trim()) params.set('keyword', serviceAccountKeywordInput.value.trim());
	        const res = await fetch('/dashboard/saasAdmin/serviceAccounts?' + params.toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        const previouslySelected = selectedServiceAccountScopes();
	        serviceAccountCache = Array.isArray(data.items) ? data.items : [];
	        serviceAccountScopeCatalog = Array.isArray(data.scopes) ? data.scopes : [];
	        if (Number(serviceAccountIdInput.value || 0) > 0) {
	          const selected = serviceAccountCache.find(item => Number(item.id) === Number(serviceAccountIdInput.value));
	          renderServiceAccountScopeOptions(selected ? selected.scopes : previouslySelected);
	        } else {
	          renderServiceAccountScopeOptions(previouslySelected.length ? previouslySelected : serviceAccountScopeCatalog.map(item => item.code));
	        }
	        const protection = data.keyProtection || {};
	        renderServiceAccountSummary(data.summary || {}, protection, data.clientIPResolution || {});
	        renderServiceAccounts(serviceAccountCache);
	        renderServiceAccountUsageAccountOptions(serviceAccountCache);
	        const missingKeyIds = Array.isArray(protection.missingKeyIds) ? protection.missingKeyIds : [];
	        if (missingKeyIds.length) {
	          serviceAccountStateEl.textContent = 'pepper 缺失：' + missingKeyIds.join('、');
	        } else if (Number(protection.legacyUsableKeyCount || 0) > 0) {
	          serviceAccountStateEl.textContent = '已加载 ' + fmt(data.returnedCount || serviceAccountCache.length) + ' 个账号 / ' + fmt(protection.legacyUsableKeyCount) + ' 枚旧 Key 待轮换';
	        } else {
	          serviceAccountStateEl.textContent = '已加载 ' + fmt(data.returnedCount || serviceAccountCache.length) + ' 个账号 / pepper 密钥环正常';
	        }
	        await loadServiceAccountUsage();
	      } catch (err) {
	        serviceAccountStateEl.textContent = err.message || String(err);
	      }
	    }
	    function resetServiceAccountEditor() {
	      serviceAccountIdInput.value = '0';
	      serviceAccountVersionInput.value = '0';
	      serviceAccountTenantIdInput.value = serviceAccountTenantFilterInput.value.trim();
	      serviceAccountCodeInput.value = '';
	      serviceAccountNameInput.value = '';
	      serviceAccountStatusInput.value = 'active';
	      serviceAccountRateLimitPerMinuteInput.value = '60';
	      serviceAccountDailyRequestLimitInput.value = '10000';
	      serviceAccountUsageAlertEnabledInput.checked = true;
	      serviceAccountUsageWarningPercentInput.value = '80';
	      serviceAccountRejectionWarningCountInput.value = '1';
	      serviceAccountUsageAlertCooldownMinutesInput.value = '60';
	      serviceAccountDescriptionInput.value = '';
	      serviceAccountExpiresAtInput.value = '';
	      serviceAccountAllowedCidrsInput.value = '';
	      serviceAccountInitialKeyNameInput.value = '初始密钥';
	      serviceAccountInitialKeyExpiresAtInput.value = '';
	      serviceAccountTenantIdInput.readOnly = false;
	      serviceAccountCodeInput.readOnly = false;
	      serviceAccountInitialKeyNameInput.disabled = false;
	      serviceAccountInitialKeyExpiresAtInput.disabled = false;
	      renderServiceAccountSaveApprovalState();
	      renderServiceAccountScopeOptions(serviceAccountScopeCatalog.map(item => item.code));
	      serviceAccountStateEl.textContent = '已切换到新建模式';
	    }
	    function renderServiceAccountSaveApprovalState() {
	      const updating = Number(serviceAccountIdInput.value || 0) > 0;
	      document.getElementById('saveServiceAccount').textContent = updating
	        ? (approvalActionRequired('service_account.update', 0) ? '提交修改审批' : '保存账号')
	        : (approvalActionRequired('service_account.create', 0) ? '提交创建审批' : '创建账号');
	    }
	    function selectServiceAccount(accountId) {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      const item = serviceAccountCache.find(account => Number(account.id) === Number(accountId));
	      if (!item) return;
	      serviceAccountIdInput.value = String(item.id || 0);
	      serviceAccountVersionInput.value = String(item.version || 0);
	      serviceAccountTenantIdInput.value = String(item.tenantId || '');
	      serviceAccountCodeInput.value = item.code || '';
	      serviceAccountNameInput.value = item.name || '';
	      serviceAccountStatusInput.value = item.status || 'active';
	      serviceAccountRateLimitPerMinuteInput.value = String(item.rateLimitPerMinute || 60);
	      serviceAccountDailyRequestLimitInput.value = String(item.dailyRequestLimit == null ? 10000 : item.dailyRequestLimit);
	      serviceAccountUsageAlertEnabledInput.checked = item.usageAlertEnabled !== false;
	      serviceAccountUsageWarningPercentInput.value = String(item.usageWarningPercent || 80);
	      serviceAccountRejectionWarningCountInput.value = String(item.rejectionWarningCount == null ? 1 : item.rejectionWarningCount);
	      serviceAccountUsageAlertCooldownMinutesInput.value = String(item.usageAlertCooldownMinutes || 60);
	      serviceAccountDescriptionInput.value = item.description || '';
	      serviceAccountExpiresAtInput.value = item.expiresAt || '';
	      serviceAccountAllowedCidrsInput.value = (item.allowedCidrs || []).join('\n');
	      serviceAccountInitialKeyNameInput.value = '';
	      serviceAccountInitialKeyExpiresAtInput.value = '';
	      serviceAccountTenantIdInput.readOnly = true;
	      serviceAccountCodeInput.readOnly = true;
	      serviceAccountInitialKeyNameInput.disabled = true;
	      serviceAccountInitialKeyExpiresAtInput.disabled = true;
	      renderServiceAccountSaveApprovalState();
	      renderServiceAccountScopeOptions(item.scopes || []);
	      serviceAccountStateEl.textContent = '正在编辑 ' + (item.name || item.code);
	    }
	    function serviceAccountPayload() {
	      const common = {
	        name: serviceAccountNameInput.value.trim(),
	        description: serviceAccountDescriptionInput.value.trim(),
	        status: serviceAccountStatusInput.value,
	        scopes: selectedServiceAccountScopes(),
	        allowedCidrs: serviceAccountCIDRs(),
	        rateLimitPerMinute: Number(serviceAccountRateLimitPerMinuteInput.value),
	        dailyRequestLimit: Number(serviceAccountDailyRequestLimitInput.value),
	        usageAlertEnabled: serviceAccountUsageAlertEnabledInput.checked,
	        usageWarningPercent: Number(serviceAccountUsageWarningPercentInput.value),
	        rejectionWarningCount: Number(serviceAccountRejectionWarningCountInput.value),
	        usageAlertCooldownMinutes: Number(serviceAccountUsageAlertCooldownMinutesInput.value),
	        expiresAt: serviceAccountExpiresAtInput.value.trim(),
	      };
	      const id = Number(serviceAccountIdInput.value || 0);
	      if (id > 0) return Object.assign(common, { id, expectedVersion: Number(serviceAccountVersionInput.value || 0) });
	      return Object.assign(common, {
	        tenantId: Number(serviceAccountTenantIdInput.value || 0),
	        code: serviceAccountCodeInput.value.trim(),
	        keyName: serviceAccountInitialKeyNameInput.value.trim(),
	        keyExpiresAt: serviceAccountInitialKeyExpiresAtInput.value.trim(),
	      });
	    }
	    function showOneTimeServiceAccountKey(value, message) {
	      serviceAccountPlainTextKeyInput.value = value || '';
	      serviceAccountSecretStateEl.textContent = message || '仅在本次响应中显示，关闭后无法恢复';
	      serviceAccountSecretEl.hidden = !value;
	    }
	    async function saveServiceAccount() {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      const payload = serviceAccountPayload();
	      const updating = Number(serviceAccountIdInput.value || 0) > 0;
	      if (!payload.name || !payload.scopes.length || (!updating && (!payload.tenantId || !payload.code))) {
	        serviceAccountStateEl.textContent = '请填写租户、编码、名称并至少选择一个作用域';
	        return;
	      }
	      if (!Number.isInteger(payload.rateLimitPerMinute) || payload.rateLimitPerMinute < 1 || payload.rateLimitPerMinute > 60000 ||
	          !Number.isInteger(payload.dailyRequestLimit) || payload.dailyRequestLimit < 0 || payload.dailyRequestLimit > 100000000) {
	        serviceAccountStateEl.textContent = '请求限额不合法：每分钟 1 至 60000，每日 0 至 100000000';
	        return;
	      }
	      if (!Number.isInteger(payload.usageWarningPercent) || payload.usageWarningPercent < 1 || payload.usageWarningPercent > 100 ||
	          !Number.isInteger(payload.rejectionWarningCount) || payload.rejectionWarningCount < 0 || payload.rejectionWarningCount > 100000000 ||
	          !Number.isInteger(payload.usageAlertCooldownMinutes) || payload.usageAlertCooldownMinutes < 5 || payload.usageAlertCooldownMinutes > 10080) {
	        serviceAccountStateEl.textContent = '预警策略不合法：用量线 1 至 100%，拒绝数 0 至 100000000，冷却 5 至 10080 分钟';
	        return;
	      }
	      const approvalAction = updating ? 'service_account.update' : 'service_account.create';
	      const requiresApproval = approvalActionRequired(approvalAction, 0);
	      serviceAccountStateEl.textContent = requiresApproval
	        ? (updating ? '正在提交服务账号变更审批' : '正在提交服务账号创建审批')
	        : (updating ? '正在保存服务账号' : '正在创建服务账号');
	      try {
	        if (requiresApproval) {
	          if (updating) {
	            await requestHighRiskApproval('service_account.update', payload, '变更服务账号：' + (payload.name || ('#' + payload.id)));
	          } else {
	            await requestHighRiskApproval('service_account.create', payload, '创建服务账号：' + (payload.name || payload.code));
	          }
	          serviceAccountStateEl.textContent = updating ? '服务账号变更已提交双人审批' : '服务账号创建已提交双人审批；执行时才会生成并一次性展示首个 Key';
	          return;
	        }
	        const res = await fetch('/dashboard/saasAdmin/serviceAccount', {
	          method: updating ? 'PUT' : 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	        });
	        const body = await res.json();
	        if (!res.ok || (body.code !== 200 && body.code !== 201)) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        if (data.plainTextKey) showOneTimeServiceAccountKey(data.plainTextKey, '账号已创建，API Key 仅显示本次');
	        const accountId = Number((data.account || {}).id || 0);
	        await loadServiceAccounts();
	        if (updating && accountId) selectServiceAccount(accountId);
	        else resetServiceAccountEditor();
	        serviceAccountStateEl.textContent = updating ? '服务账号已保存' : '服务账号已创建';
	        loadOperations();
	      } catch (err) {
	        serviceAccountStateEl.textContent = err.message || String(err);
	      }
	    }
	    function selectServiceAccountRotation(accountId) {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      const item = serviceAccountCache.find(account => Number(account.id) === Number(accountId));
	      if (!item) return;
	      const now = new Date();
	      serviceAccountRotateIdInput.value = String(item.id || 0);
	      serviceAccountRotateVersionInput.value = String(item.version || 0);
	      serviceAccountRotateAccountInput.value = (item.name || item.code) + ' / #' + item.id + ' / v' + item.version;
	      serviceAccountRotateNameInput.value = now.getFullYear() + '-Q' + (Math.floor(now.getMonth() / 3) + 1);
	      serviceAccountRotateExpiresAtInput.value = serviceAccountDateTimeAfter(90);
	      serviceAccountRotateGraceInput.value = '60';
	      document.getElementById('rotateServiceAccountKey').textContent = approvalActionRequired('service_account.key.rotate', 0) ? '提交轮换审批' : '轮换密钥';
	      serviceAccountStateEl.textContent = '已选择 ' + (item.name || item.code) + ' 进行密钥轮换';
	    }
	    async function rotateServiceAccountKey() {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      const payload = {
	        serviceAccountId: Number(serviceAccountRotateIdInput.value || 0),
	        expectedVersion: Number(serviceAccountRotateVersionInput.value || 0),
	        name: serviceAccountRotateNameInput.value.trim(),
	        expiresAt: serviceAccountRotateExpiresAtInput.value.trim(),
	        graceMinutes: Number(serviceAccountRotateGraceInput.value || 0),
	      };
	      if (!payload.serviceAccountId || !payload.name || !payload.expiresAt || !Number.isInteger(payload.graceMinutes) || payload.graceMinutes < 0 || payload.graceMinutes > 10080) {
	        serviceAccountStateEl.textContent = '请选择账号并填写有效的密钥名称、过期时间和宽限期';
	        return;
	      }
	      const requiresApproval = approvalActionRequired('service_account.key.rotate', 0);
	      serviceAccountStateEl.textContent = requiresApproval ? '正在提交轮换审批' : '正在轮换 API Key';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval('service_account.key.rotate', payload, '轮换服务账号密钥：' + (payload.name || ('#' + payload.serviceAccountId)));
	          serviceAccountStateEl.textContent = '密钥轮换已提交双人审批；执行时才会生成并一次性展示新 Key';
	          return;
	        }
	        const res = await fetch('/dashboard/saasAdmin/serviceAccountKeyRotate', {
	          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	        });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        showOneTimeServiceAccountKey(data.plainTextKey, '密钥已轮换，旧密钥宽限 ' + fmt(payload.graceMinutes) + ' 分钟');
	        await loadServiceAccounts();
	        selectServiceAccountRotation(payload.serviceAccountId);
	        serviceAccountStateEl.textContent = 'API Key 已轮换';
	        loadOperations();
	      } catch (err) {
	        serviceAccountStateEl.textContent = err.message || String(err);
	      }
	    }
	    function revokeServiceAccountKey(button) {
	      if (!hasPlatformPermission('platform.integrations.manage')) return;
	      pendingServiceAccountRevoke = {
	        serviceAccountId: Number(button.dataset.accountId || 0), keyId: Number(button.dataset.keyId || 0), expectedVersion: Number(button.dataset.keyVersion || 0),
	      };
	      const requiresApproval = approvalActionRequired('service_account.key.revoke', 0);
	      serviceAccountRevokeMessageEl.textContent = '确认吊销 API Key #' + pendingServiceAccountRevoke.keyId + '？' +
	        (requiresApproval ? '审批通过并执行后，该密钥将立即无法使用。' : '吊销后立即无法使用。');
	      serviceAccountRevokeReasonInput.value = '吊销 API Key #' + pendingServiceAccountRevoke.keyId;
	      document.getElementById('confirmServiceAccountKeyRevoke').textContent = requiresApproval ? '提交吊销审批' : '确认吊销';
	      serviceAccountRevokeDialogEl.showModal();
	    }
	    function cancelServiceAccountKeyRevoke() {
	      pendingServiceAccountRevoke = null;
	      serviceAccountRevokeReasonInput.value = '';
	      serviceAccountRevokeDialogEl.close();
	    }
	    async function confirmRevokeServiceAccountKey() {
	      if (!hasPlatformPermission('platform.integrations.manage') || !pendingServiceAccountRevoke) return;
	      const payload = pendingServiceAccountRevoke;
	      const reason = serviceAccountRevokeReasonInput.value.trim();
	      if (!reason) {
	        serviceAccountStateEl.textContent = '请填写吊销原因';
	        serviceAccountRevokeReasonInput.focus();
	        return;
	      }
	      const confirmButton = document.getElementById('confirmServiceAccountKeyRevoke');
	      confirmButton.disabled = true;
	      const requiresApproval = approvalActionRequired('service_account.key.revoke', 0);
	      serviceAccountStateEl.textContent = requiresApproval ? '正在提交吊销审批' : '正在吊销 API Key';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval('service_account.key.revoke', payload, reason);
	        } else {
	          const res = await fetch('/dashboard/saasAdmin/serviceAccountKeyRevoke', {
	            method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	          });
	          const body = await res.json();
	          if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        }
	        pendingServiceAccountRevoke = null;
	        serviceAccountRevokeReasonInput.value = '';
	        serviceAccountRevokeDialogEl.close();
	        await loadServiceAccounts();
	        serviceAccountStateEl.textContent = requiresApproval ? '吊销审批已提交，等待两人复核' : 'API Key 已吊销';
	        loadOperations();
	      } catch (err) {
	        serviceAccountStateEl.textContent = err.message || String(err);
	      } finally {
	        confirmButton.disabled = false;
	      }
	    }
	    async function copyServiceAccountKey() {
	      const value = serviceAccountPlainTextKeyInput.value;
	      if (!value) return;
	      try {
	        if (navigator.clipboard && window.isSecureContext) await navigator.clipboard.writeText(value);
	        else {
	          serviceAccountPlainTextKeyInput.select();
	          document.execCommand('copy');
	        }
	        serviceAccountSecretStateEl.textContent = 'API Key 已复制，请立即存入密钥管理器';
	      } catch (err) {
	        serviceAccountSecretStateEl.textContent = '复制失败，请手动复制';
	      }
	    }
	    function dismissServiceAccountKey() {
	      serviceAccountPlainTextKeyInput.value = '';
	      serviceAccountSecretEl.hidden = true;
	    }
	    function approvalPolicy(actionType) {
	      return approvalPolicies.find(item => item.actionType === actionType) || { actionType, name: actionType, riskLevel: 'high' };
	    }
	    function approvalActionRequired(actionType, amountCents) {
	      if (!approvalRequired) return false;
	      const policy = approvalPolicy(actionType);
	      if (policy.enabled === false) return false;
	      if (actionType === 'payment.refund.create' && Number(policy.amountThresholdCents || 0) > 0) {
	        return Number(amountCents || 0) >= Number(policy.amountThresholdCents || 0);
	      }
	      return true;
	    }
	    function localDateTimeInputValue(value) {
	      const date = value instanceof Date ? value : new Date(value);
	      if (!Number.isFinite(date.getTime())) return '';
	      const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
	      return local.toISOString().slice(0, 16);
	    }
	    function parsePageDate(value) {
	      return new Date(String(value || '').replace(' ', 'T'));
	    }
	    function hasActiveApprovalDelegation() {
	      const now = Date.now();
	      return approvalDelegations.some(item => Number(item.status) === 1 && Number(item.delegateUserId) === currentPlatformUserID &&
	        parsePageDate(item.startsAt).getTime() <= now && parsePageDate(item.endsAt).getTime() > now);
	    }
	    function canReviewApprovals() {
	      return hasPlatformPermission('platform.approvals.review') || hasActiveApprovalDelegation();
	    }
	    function renderApprovalPolicies(items) {
	      const canManage = hasPlatformPermission('platform.approvals.manage');
	      if (!items.length) {
	        approvalPoliciesEl.innerHTML = '<tr><td colspan="6" class="empty">暂无审批策略</td></tr>';
	        return;
	      }
	      approvalPoliciesEl.innerHTML = items.map(item => {
	        const governancePolicy = item.governanceLocked === true;
	        const minimumApprovals = Number(item.minimumApprovals || (governancePolicy ? 2 : 1));
	        const amountThresholdLocked = item.amountThresholdLocked === true;
	        const disabled = canManage ? '' : ' disabled';
	        const enabledDisabled = canManage && !governancePolicy ? '' : ' disabled';
	        const thresholdDisabled = canManage && !amountThresholdLocked ? '' : ' disabled';
	        const threshold = item.actionType === 'payment.refund.create'
	          ? '<input data-policy-field="amountThresholdCents" type="number" min="0" step="1" value="' + fmt(item.amountThresholdCents || 0) + '"' + thresholdDisabled + '>'
	          : '<span class="muted">不适用</span>';
	        return '<tr data-policy-action="' + esc(item.actionType) + '"><td><strong>' + esc(item.name) + '</strong><br><span class="muted">' + esc(item.actionType) + '</span></td>' +
	          '<td><label class="checkline"><input data-policy-field="enabled" type="checkbox"' + (item.enabled !== false ? ' checked' : '') + enabledDisabled + '>' + (governancePolicy ? '强制启用' : '启用') + '</label>' + threshold + '</td>' +
	          '<td><input data-policy-field="requiredApprovals" type="number" min="' + minimumApprovals + '" max="5" value="' + fmt(item.requiredApprovals || minimumApprovals) + '"' + disabled + '></td>' +
	          '<td><input data-policy-field="slaMinutes" type="number" min="15" max="10080" value="' + fmt(item.slaMinutes || 240) + '"' + disabled + '><br><input data-policy-field="reminderMinutes" type="number" min="5" max="10080" value="' + fmt(item.reminderMinutes || 60) + '"' + disabled + '></td>' +
	          '<td><input data-policy-field="expiryHours" type="number" min="1" max="168" value="' + fmt(item.expiryHours || 24) + '"' + disabled + '></td>' +
	          '<td>v' + fmt(item.version || 1) + '<br>' + (canManage ? '<button type="button" data-policy-save="' + esc(item.actionType) + '" data-version="' + fmt(item.version || 1) + '">' + (approvalActionRequired('approval.policy.update') ? '提交审批' : '保存') + '</button>' : '<span class="muted">只读</span>') + '</td></tr>';
	      }).join('');
	    }
	    async function saveApprovalPolicy(button) {
	      const row = button.closest('tr[data-policy-action]');
	      if (!row) return;
	      const field = name => row.querySelector('[data-policy-field="' + name + '"]');
	      const payload = {
	        actionType: row.dataset.policyAction, enabled: field('enabled').checked,
	        amountThresholdCents: field('amountThresholdCents') ? Number(field('amountThresholdCents').value || 0) : 0,
	        requiredApprovals: Number(field('requiredApprovals').value || 1), slaMinutes: Number(field('slaMinutes').value || 0),
	        reminderMinutes: Number(field('reminderMinutes').value || 0), expiryHours: Number(field('expiryHours').value || 0),
	        expectedVersion: Number(button.dataset.version || 0),
	      };
	      const targetPolicy = approvalPolicy(payload.actionType);
	      const minimumApprovals = Number(targetPolicy.minimumApprovals || (targetPolicy.governanceLocked ? 2 : 1));
	      if (targetPolicy.governanceLocked && (!payload.enabled || payload.requiredApprovals < minimumApprovals || (targetPolicy.amountThresholdLocked && payload.amountThresholdCents !== 0))) {
	        approvalStateEl.textContent = '严重风险门禁必须启用、不得设置绕过阈值且至少需要两人会签';
	        return;
	      }
	      button.disabled = true;
	      approvalStateEl.textContent = approvalActionRequired('approval.policy.update') ? '正在提交策略变更审批' : '正在保存审批策略';
	      try {
	        if (approvalActionRequired('approval.policy.update')) {
	          const target = approvalPolicy(payload.actionType);
	          await requestHighRiskApproval('approval.policy.update', payload, '变更审批策略：' + (target.name || payload.actionType));
	          approvalStateEl.textContent = '策略变更已提交双人审批';
	          return;
	        }
	        const res = await fetch('/dashboard/saasAdmin/approvalPolicy', { method: 'PUT', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        approvalStateEl.textContent = '审批策略已保存';
	        await loadApprovalPolicies();
	        loadOperations();
	      } catch (err) { approvalStateEl.textContent = err.message || String(err); }
	      finally { button.disabled = false; }
	    }
	    function resetApprovalDelegationEditor() {
	      const start = new Date();
	      const end = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000);
	      approvalDelegationIdInput.value = '0';
	      approvalDelegationVersionInput.value = '0';
	      approvalDelegatorUserIdInput.value = currentPlatformUserID ? String(currentPlatformUserID) : '';
	      approvalDelegateUserIdInput.value = '';
	      approvalDelegationStartsAtInput.value = localDateTimeInputValue(start);
	      approvalDelegationEndsAtInput.value = localDateTimeInputValue(end);
	      approvalDelegationStatusInput.value = '1';
	      approvalDelegationReasonInput.value = '';
	    }
	    function renderApprovalDelegations(items) {
	      const canManage = hasPlatformPermission('platform.approvals.manage');
	      if (!items.length) {
	        approvalDelegationsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无审批委托</td></tr>';
	        return;
	      }
	      approvalDelegationsEl.innerHTML = items.map(item => '<tr><td>' + esc(item.delegatorName || ('用户 ' + item.delegatorUserId)) + '<br><span class="muted">#' + fmt(item.delegatorUserId) + '</span></td>' +
	        '<td>' + esc(item.delegateName || ('用户 ' + item.delegateUserId)) + '<br><span class="muted">#' + fmt(item.delegateUserId) + '</span></td>' +
	        '<td>' + esc(item.startsAt || '-') + '<br><span class="muted">至 ' + esc(item.endsAt || '-') + '</span></td>' +
	        '<td>' + pill(Number(item.status) === 1 ? '启用' : '停用', Number(item.status) === 1 ? 'ok' : '') + '<br><span class="muted">' + esc(item.reason || '-') + '</span></td>' +
	        '<td>v' + fmt(item.version || 1) + (canManage ? '<br><button type="button" class="secondary" data-delegation-edit="' + fmt(item.id) + '">编辑</button>' : '') + '</td></tr>').join('');
	    }
	    async function loadApprovalDelegations() {
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalDelegations?limit=100', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        approvalDelegations = (body.data || {}).delegations || [];
	        renderApprovalDelegations(approvalDelegations);
	        approvalDelegationStateEl.textContent = fmt(approvalDelegations.length) + ' 条委托';
	        if (!Number(approvalDelegationIdInput.value)) resetApprovalDelegationEditor();
	      } catch (err) {
	        approvalDelegationStateEl.textContent = err.message || String(err);
	        approvalDelegationsEl.innerHTML = '<tr><td colspan="5" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
	      }
	    }
	    function selectApprovalDelegation(id) {
	      const item = approvalDelegations.find(value => Number(value.id) === Number(id));
	      if (!item) return;
	      approvalDelegationIdInput.value = String(item.id || 0);
	      approvalDelegationVersionInput.value = String(item.version || 0);
	      approvalDelegatorUserIdInput.value = String(item.delegatorUserId || '');
	      approvalDelegateUserIdInput.value = String(item.delegateUserId || '');
	      approvalDelegationStartsAtInput.value = String(item.startsAt || '').replace(' ', 'T').slice(0, 16);
	      approvalDelegationEndsAtInput.value = String(item.endsAt || '').replace(' ', 'T').slice(0, 16);
	      approvalDelegationStatusInput.value = String(item.status || 1);
	      approvalDelegationReasonInput.value = item.reason || '';
	    }
	    async function saveApprovalDelegation() {
	      const payload = {
	        id: Number(approvalDelegationIdInput.value || 0), expectedVersion: Number(approvalDelegationVersionInput.value || 0),
	        delegatorUserId: Number(approvalDelegatorUserIdInput.value || 0), delegateUserId: Number(approvalDelegateUserIdInput.value || 0),
	        startsAt: approvalDelegationStartsAtInput.value, endsAt: approvalDelegationEndsAtInput.value,
	        status: Number(approvalDelegationStatusInput.value || 1), reason: approvalDelegationReasonInput.value.trim(),
	      };
	      approvalDelegationStateEl.textContent = '正在保存委托';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalDelegation', { method: payload.id ? 'PUT' : 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        approvalDelegationStateEl.textContent = '审批委托已保存';
	        resetApprovalDelegationEditor();
	        await loadApprovalDelegations();
	        loadOperations();
	      } catch (err) { approvalDelegationStateEl.textContent = err.message || String(err); }
	    }
	    async function createApprovalReminders() {
	      approvalStateEl.textContent = '正在扫描到期审批';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalReminders', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify({ limit: 100, maxAttempts: 3 }) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        approvalStateEl.textContent = '已扫描 ' + fmt(data.scanned || 0) + ' 条，入队 ' + fmt(data.enqueued || 0) + ' 条';
	        await loadApprovals();
	      } catch (err) { approvalStateEl.textContent = err.message || String(err); }
	    }
	    function approvalStatusView(status) {
	      const values = {
	        pending: ['待复核', 'warning'], approved: ['已批准', 'ok'], rejected: ['已驳回', 'danger'],
	        canceled: ['已撤回', ''], expired: ['已过期', 'danger'], executing: ['执行中', 'warning'], executed: ['已执行', 'ok'],
	      };
	      return values[status] || [status || '未知', 'danger'];
	    }
	    async function requestHighRiskApproval(actionType, payload, reason, options) {
	      options = options || {};
	      if (!hasPlatformPermission('platform.approvals.read')) throw new Error('缺少平台权限 platform.approvals.read');
	      approvalStateEl.textContent = '正在提交审批';
	      const res = await fetch('/dashboard/saasAdmin/approvalRequest', {
	        method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	        body: JSON.stringify({
	          actionType, payload, reason,
	          expiresInHours: Number(approvalPolicy(actionType).expiryHours || 24),
	          idempotencyKey: options.idempotencyKey || ('page:' + actionType + ':' + Date.now()),
	        }),
	      });
	      const body = await res.json();
	      if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	      const approval = (body.data || {}).approval || {};
	      approvalStateEl.textContent = '已提交 ' + (approval.requestNo || '审批单');
	      if (options.refresh !== false) await loadApprovals();
	      return approval;
	    }
	    async function loadApprovalPolicies() {
	      if (!hasPlatformPermission('platform.approvals.read')) return;
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalPolicies', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        approvalRequired = data.required !== false;
		        approvalPolicies = Array.isArray(data.policies) ? data.policies : [];
		        renderTenantStatusApprovalState();
		        renderServiceAccountSaveApprovalState();
		        document.getElementById('rotateServiceAccountKey').textContent = approvalActionRequired('service_account.key.rotate', 0) ? '提交轮换审批' : '轮换密钥';
		        document.getElementById('savePackage').textContent = approvalActionRequired('package.upsert', 0) ? '提交套餐审批' : '保存套餐';
		        document.getElementById('provisionTenant').textContent = approvalActionRequired('tenant.provision', 0) ? '提交开户审批' : '开通租户';
		        document.getElementById('applyProvisionTask').textContent = approvalActionRequired('tenant.provision', 0) ? '提交任务审批' : '应用开户任务';
		        document.getElementById('bulkApplyProvisionTasks').textContent = approvalActionRequired('tenant.provision', 0) ? '批量提交开户审批' : '批量应用开户';
		        document.getElementById('applyRenewal').textContent = approvalActionRequired('tenant.renewal', 0) ? '提交续费审批' : '记录续费';
		        document.getElementById('applyRenewalTask').textContent = approvalActionRequired('tenant.renewal', 0) ? '提交任务审批' : '应用续费任务';
			        document.getElementById('bulkApplyRenewalTasks').textContent = approvalActionRequired('tenant.renewal', 0) ? '批量提交续费审批' : '批量应用续费';
		        document.getElementById('transitionSubscription').textContent = approvalActionRequired('tenant.subscription.transition', 0) ? '提交订阅审批' : '应用迁移';
		        document.getElementById('createPaymentOrder').textContent = approvalActionRequired('payment.order.create', 0) ? '提交收款审批' : '创建支付订单';
		        document.getElementById('createTenantDomain').textContent = approvalActionRequired('tenant.domain.create', 0) ? '提交添加审批' : '添加域名';
		        renderInvoiceIssueApprovalState();
		        renderBackupPolicyApprovalState();
		        renderCompliancePolicyApprovalState();
		        renderIdentityPolicyApprovalState();
		        renderIdentityUsers(identityUserCache);
		        renderTenantDomains(tenantDomainCache);
	        approvalActionInput.innerHTML = '<option value="">全部动作</option>' + approvalPolicies.map(item => '<option value="' + esc(item.actionType) + '">' + esc(item.name) + '</option>').join('');
	        renderApprovalPolicies(approvalPolicies);
	        approvalStateEl.textContent = approvalRequired ? '高风险动作强制审批' : '审批门禁已临时关闭';
	        await loadApprovalDelegations();
	        await loadApprovals();
	      } catch (err) {
	        approvalStateEl.textContent = err.message || String(err);
	      }
	    }
	    function approvalParams() {
	      const params = new URLSearchParams({ status: approvalStatusInput.value || 'all', riskLevel: approvalRiskInput.value || 'all', limit: '100' });
	      if (approvalActionInput.value) params.set('actionType', approvalActionInput.value);
	      if (approvalKeywordInput.value.trim()) params.set('keyword', approvalKeywordInput.value.trim());
	      return params;
	    }
	    function renderApprovalSummary(summary) {
	      summary = summary || {};
	      approvalSummaryEl.innerHTML = [
	        ['审批总数', fmt(summary.total || 0)], ['待复核', fmt(summary.pending || 0)], ['已批准', fmt(summary.approved || 0)],
	        ['执行中', fmt(summary.executing || 0)], ['已执行', fmt(summary.executed || 0)], ['严重风险', fmt(summary.critical || 0)],
	      ].map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
	    }
	    function renderApprovals(items) {
	      if (!items.length) {
	        approvalsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无审批</td></tr>';
	        return;
	      }
	      approvalsEl.innerHTML = items.map(item => {
	        const policy = approvalPolicy(item.actionType);
	        const state = approvalStatusView(item.status);
	        const isRequester = Number(item.requesterUserId) === currentPlatformUserID;
	        const actions = ['<button type="button" class="secondary" data-approval-action="events" data-id="' + fmt(item.id) + '">事件</button>'];
	        if (item.status === 'pending' && !isRequester && canReviewApprovals()) {
	          actions.push('<button type="button" data-approval-action="approve" data-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">批准</button>');
	          actions.push('<button type="button" class="danger" data-approval-action="reject" data-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">驳回</button>');
	        }
	        if (item.status === 'approved' && !isRequester && hasPlatformPermission('platform.approvals.execute')) {
	          actions.push('<button type="button" data-approval-action="execute" data-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">执行</button>');
	        }
	        if (item.status === 'approved' && !isRequester && canReviewApprovals()) {
	          actions.push('<button type="button" class="danger" data-approval-action="reject" data-current-status="approved" data-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">撤销批准</button>');
	        }
	        if ((item.status === 'pending' || item.status === 'approved') && isRequester) {
	          actions.push('<button type="button" class="secondary" data-approval-action="cancel" data-id="' + fmt(item.id) + '" data-version="' + fmt(item.version) + '">撤回</button>');
	        }
	        const slaDue = parsePageDate(item.slaDueAt).getTime();
	        const slaOverdue = item.status === 'pending' && Number.isFinite(slaDue) && slaDue <= Date.now();
	        return '<tr><td><strong>' + esc(item.requestNo || '-') + '</strong><br><span class="muted">#' + fmt(item.id) + ' / v' + fmt(item.version) + ' / 策略 v' + fmt(item.policyVersion || 1) + '</span></td>' +
	          '<td>' + esc(policy.name) + '<br><span class="muted">' + esc(item.targetName || item.targetId || '-') + '</span></td>' +
	          '<td>' + esc(item.requesterName || ('用户 ' + item.requesterUserId)) + '<br><span class="muted">' + esc(item.reason || '-') + '</span></td>' +
	          '<td>' + pill(state[0], state[1]) + '<br><span class="muted">' + (item.riskLevel === 'critical' ? '严重风险' : '高风险') + '</span></td>' +
	          '<td><strong>' + fmt(item.approvalCount || 0) + '/' + fmt(item.requiredApprovals || 1) + '</strong><br><span class="muted">最近 ' + esc(item.reviewerName || '-') + ' / 执行 ' + esc(item.executionUserName || '-') + '</span></td>' +
	          '<td>' + (slaOverdue ? pill('已超时', 'danger') : esc(item.slaDueAt || '-')) + '<br><span class="muted">有效至 ' + esc(item.expiresAt || '-') + ' / 提醒 ' + fmt(item.reminderCount || 0) + '</span></td>' +
	          '<td>' + actions.join(' ') + (item.lastError ? '<br><span class="bad">' + esc(item.lastError) + '</span>' : '') + '</td></tr>';
	      }).join('');
	    }
	    async function loadApprovals() {
	      if (!hasPlatformPermission('platform.approvals.read')) return;
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvals?' + approvalParams().toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        renderApprovalSummary(data.summary || {});
	        renderApprovals(data.items || []);
	        approvalStateEl.textContent = (approvalRequired ? '强制审批' : '门禁关闭') + ' / 显示 ' + fmt((data.items || []).length) + ' 条';
	      } catch (err) {
	        approvalsEl.innerHTML = '<tr><td colspan="7" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
	      }
	    }
	    async function loadApprovalEvents(approvalId) {
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalEvents?approvalId=' + encodeURIComponent(approvalId) + '&limit=200', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const items = (body.data || {}).events || [];
	        approvalEventStateEl.textContent = '审批 #' + approvalId + ' / ' + items.length + ' 条事件';
	        approvalEventsEl.innerHTML = items.length ? items.map(item => '<tr><td>' + esc(item.eventType || '-') + '</td><td>' + esc((item.fromStatus || '-') + ' → ' + (item.toStatus || '-')) + '</td><td>' + esc(item.actorName || (item.actorUserId ? ('用户 ' + item.actorUserId) : '系统')) + '</td><td>' + esc(item.reason || '-') + '</td><td>' + esc(item.createdAt || '-') + '</td></tr>').join('') : '<tr><td colspan="5" class="empty">暂无事件</td></tr>';
	      } catch (err) {
	        approvalEventStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function loadApprovalDecisions(approvalId) {
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalDecisions?approvalId=' + encodeURIComponent(approvalId) + '&limit=100', { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const items = (body.data || {}).decisions || [];
	        approvalDecisionStateEl.textContent = '审批 #' + approvalId + ' / ' + items.length + ' 票';
	        approvalDecisionsEl.innerHTML = items.length ? items.map(item => '<tr><td>' + esc(item.reviewerName || ('用户 ' + item.reviewerUserId)) + '</td>' +
	          '<td>' + esc(item.delegatedFromName || (item.delegatedFromUserId ? ('用户 ' + item.delegatedFromUserId) : '-')) + '</td>' +
	          '<td>' + pill(item.decision === 'approve' ? '批准' : '驳回', item.decision === 'approve' ? 'ok' : 'danger') + '</td>' +
	          '<td>' + esc(item.reason || '-') + '</td><td>' + esc(item.createdAt || '-') + '</td></tr>').join('') : '<tr><td colspan="5" class="empty">暂无会签决定</td></tr>';
	      } catch (err) {
	        approvalDecisionStateEl.textContent = err.message || String(err);
	      }
	    }
	    async function decideApproval(button, decision) {
	      const revokeApproved = decision === 'reject' && button.dataset.currentStatus === 'approved';
	      const promptTitle = decision === 'approve' ? '请输入批准意见' : (revokeApproved ? '请输入撤销批准原因' : '请输入驳回原因');
	      const promptDefault = decision === 'approve' ? '复核通过' : (revokeApproved ? '执行前复核结论变化' : '申请信息不完整');
	      const reason = window.prompt(promptTitle, promptDefault);
	      if (reason === null || !reason.trim()) return;
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalDecision', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify({ approvalId: Number(button.dataset.id), expectedVersion: Number(button.dataset.version), decision, reason: reason.trim() }) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const approval = (body.data || {}).approval || {};
	        approvalStateEl.textContent = decision === 'approve' ? ('会签已记录 ' + fmt(approval.approvalCount || 0) + '/' + fmt(approval.requiredApprovals || 1)) : (revokeApproved ? '批准已撤销' : '审批已驳回');
	        await loadApprovals();
	        loadOperations();
	      } catch (err) { approvalStateEl.textContent = err.message || String(err); }
	    }
	    async function cancelApproval(button) {
	      const reason = window.prompt('请输入撤回原因', '申请内容需要调整');
	      if (reason === null || !reason.trim()) return;
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalCancel', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify({ approvalId: Number(button.dataset.id), expectedVersion: Number(button.dataset.version), reason: reason.trim() }) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        approvalStateEl.textContent = '审批已撤回';
	        await loadApprovals();
	        loadOperations();
	      } catch (err) { approvalStateEl.textContent = err.message || String(err); }
	    }
	    async function executeApproval(button) {
	      if (!window.confirm('确认执行该已批准的高风险操作？')) return;
	      approvalStateEl.textContent = '正在执行审批';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/approvalExecute', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify({ approvalId: Number(button.dataset.id), expectedVersion: Number(button.dataset.version) }) });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const result = (body.data || {}).result || {};
	        if (result.plainTextKey) {
	          const initialKey = result.initialKey === true;
	          showOneTimeServiceAccountKey(result.plainTextKey, initialKey
	            ? '服务账号已创建，首个 Key 仅显示本次'
	            : '密钥已轮换，旧密钥宽限 ' + fmt(result.graceMinutes || 0) + ' 分钟；新 Key 仅显示本次');
	          await loadServiceAccounts();
	          if (initialKey) resetServiceAccountEditor();
	        }
	        approvalStateEl.textContent = result.plainTextKey
	          ? (result.initialKey === true ? '服务账号创建已执行，首个 Key 仅显示本次' : '密钥轮换已执行，新 Key 仅显示本次')
	          : '高风险操作已执行';
	        await loadApprovals();
	        loadOverview();
	      } catch (err) { approvalStateEl.textContent = err.message || String(err); }
	    }
	    function localDateString(date) {
	      const year = date.getFullYear();
	      const month = String(date.getMonth() + 1).padStart(2, '0');
	      const day = String(date.getDate()).padStart(2, '0');
	      return year + '-' + month + '-' + day;
	    }
    function tenantExportParams(limit) {
      const params = new URLSearchParams({
        type: 'tenants',
        scope: 'platform',
        expiringDays: expiringInput.value || '30',
        limit: String(limit || 1000),
      });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (filterKeywordInput.value.trim()) params.set('keyword', filterKeywordInput.value.trim());
      if (filterPackageCodeInput.value.trim()) params.set('packageCode', filterPackageCodeInput.value.trim());
      if (filterTenantStatusInput.value.trim()) params.set('tenantStatus', filterTenantStatusInput.value.trim());
      if (filterDueStateInput.value && filterDueStateInput.value !== 'all') params.set('dueState', filterDueStateInput.value);
      return params;
    }
    function operationParams(limit) {
      const params = new URLSearchParams({ limit: String(limit || 30) });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (operationActionInput.value.trim()) params.set('action', operationActionInput.value.trim());
      if (operationTargetTypeInput.value.trim()) params.set('targetType', operationTargetTypeInput.value.trim());
      if (operationKeywordInput.value.trim()) params.set('keyword', operationKeywordInput.value.trim());
      return params;
    }
    function billingParams(limit) {
      const params = new URLSearchParams({ limit: String(limit || 30) });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (billingEventTypeInput.value.trim()) params.set('eventType', billingEventTypeInput.value.trim());
      if (billingPackageCodeInput.value.trim()) params.set('packageCode', billingPackageCodeInput.value.trim());
      if (billingKeywordInput.value.trim()) params.set('keyword', billingKeywordInput.value.trim());
      return params;
    }
    function billingReconciliationParams(limit) {
      const params = billingParams(limit || 30);
      if (billingMismatchOnlyInput.checked) params.set('mismatchOnly', '1');
      return params;
    }
    function billingFollowParams(limit) {
      const params = new URLSearchParams({ limit: String(limit || 50) });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (billingFollowStatusInput.value.trim()) params.set('status', billingFollowStatusInput.value.trim());
      if (billingFollowDueStateInput.value && billingFollowDueStateInput.value !== 'all') params.set('dueState', billingFollowDueStateInput.value);
      if (billingFollowOwnerInput.value.trim()) params.set('owner', billingFollowOwnerInput.value.trim());
      if (billingFollowKeywordInput.value.trim()) params.set('keyword', billingFollowKeywordInput.value.trim());
      return params;
    }
    function alertParams(limit) {
      const params = new URLSearchParams({ perPage: String(limit || 30) });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (alertStatusInput.value.trim()) params.set('status', alertStatusInput.value.trim());
      if (alertMetricInput.value.trim()) params.set('metric', alertMetricInput.value.trim());
      if (alertTypeInput.value.trim()) params.set('alertType', alertTypeInput.value.trim());
      return params;
    }
    function notificationPolicyParams(limit) {
      const params = new URLSearchParams({
        channel: 'webhook',
        state: notificationPolicyStateInput.value || 'all',
        limit: String(limit || 50),
      });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (notificationPolicyKeywordInput.value.trim()) params.set('keyword', notificationPolicyKeywordInput.value.trim());
      return params;
    }
    function notificationHealthParams(limit) {
      const params = new URLSearchParams({
        channel: 'webhook',
        state: notificationHealthStateInput.value || 'all',
        windowHours: notificationHealthWindowInput.value || '24',
        staleMinutes: notificationHealthStaleInput.value || '15',
        limit: String(limit || 50),
      });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (notificationHealthKeywordInput.value.trim()) params.set('keyword', notificationHealthKeywordInput.value.trim());
      return params;
    }
    function notificationSloParams(limit) {
      const successTarget = Number(notificationSloSuccessTargetInput.value || 95) / 100;
      const latencyTarget = Number(notificationSloLatencyTargetInput.value || 95) / 100;
      const params = new URLSearchParams({
        channel: 'webhook',
        days: notificationSloDaysInput.value || '7',
        successRateTarget: String(successTarget),
        latencySecondsTarget: notificationSloLatencySecondsInput.value || '300',
        latencyRateTarget: String(latencyTarget),
        limit: String(limit || 50),
      });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (notificationSloKeywordInput.value.trim()) params.set('keyword', notificationSloKeywordInput.value.trim());
      return params;
    }
    function notificationParams(limit) {
      const params = new URLSearchParams({ limit: String(limit || 30) });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (notificationStatusInput.value.trim()) params.set('status', notificationStatusInput.value.trim());
      if (notificationChannelInput.value.trim()) params.set('channel', notificationChannelInput.value.trim());
      if (notificationKeywordInput.value.trim()) params.set('keyword', notificationKeywordInput.value.trim());
      return params;
    }
	    function riskParams(limit) {
	      const params = new URLSearchParams({
	        scope: scopeInput.value || 'tenant',
	        expiringDays: expiringInput.value || '30',
	        highUsageRatio: riskHighUsageInput.value || '0.8',
	        limit: String(limit || 50),
	      });
	      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
	      if (filterKeywordInput.value.trim()) params.set('keyword', filterKeywordInput.value.trim());
	      if (filterPackageCodeInput.value.trim()) params.set('packageCode', filterPackageCodeInput.value.trim());
	      if (filterTenantStatusInput.value.trim()) params.set('tenantStatus', filterTenantStatusInput.value.trim());
	      if (filterDueStateInput.value && filterDueStateInput.value !== 'all') params.set('dueState', filterDueStateInput.value);
	      return params;
	    }
	    function customerSuccessParams(limit) {
	      const params = new URLSearchParams({
	        expiringDays: expiringInput.value || '30',
	        highUsageRatio: riskHighUsageInput.value || '0.8',
	        tenantLimit: customerSuccessTenantLimitInput.value || '200',
	        limit: String(limit || 30),
	      });
	      if (customerSuccessPriorityInput.value.trim()) params.set('priority', customerSuccessPriorityInput.value.trim());
	      if (customerSuccessOwnerInput.value.trim()) params.set('owner', customerSuccessOwnerInput.value.trim());
	      return params;
	    }
	    function riskTaskParams(limit) {
	      const params = new URLSearchParams({ limit: String(limit || 50) });
	      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
	      if (riskTaskStatusInput.value.trim()) params.set('status', riskTaskStatusInput.value.trim());
	      if (riskTaskDueStateInput.value && riskTaskDueStateInput.value !== 'all') params.set('dueState', riskTaskDueStateInput.value);
	      if (riskTaskOwnerInput.value.trim()) params.set('owner', riskTaskOwnerInput.value.trim());
	      if (riskTaskKeywordInput.value.trim()) params.set('keyword', riskTaskKeywordInput.value.trim());
	      return params;
	    }
	    function dailyReportParams(limit) {
	      const params = new URLSearchParams({
	        expiringDays: expiringInput.value || '30',
	        highUsageRatio: riskHighUsageInput.value || '0.8',
	        tenantLimit: '200',
	        limit: String(limit || 20),
	      });
	      if (dailyReportDateInput.value.trim()) params.set('date', dailyReportDateInput.value.trim());
	      if (dailyReportDaysInput.value.trim()) params.set('days', dailyReportDaysInput.value.trim());
	      return params;
	    }
	    function businessMetricsParams() {
	      return new URLSearchParams({
	        expiringDays: expiringInput.value || '30',
	        highUsageRatio: riskHighUsageInput.value || '0.8',
	        tenantLimit: customerSuccessTenantLimitInput.value || '500',
	        billingLimit: '1000',
	      });
	    }
	function businessTrendsParams() {
	  return new URLSearchParams({
	    months: '6',
	    billingLimit: '1000',
	    taskLimit: '1000',
	  });
	}
	function operationQueueParams(limit) {
	  const params = new URLSearchParams({
	    expiringDays: expiringInput.value || '30',
	    highUsageRatio: riskHighUsageInput.value || '0.8',
	    tenantLimit: customerSuccessTenantLimitInput.value || '200',
	    limit: String(limit || 50),
	    warningHours: adminTaskSlaWarningInput.value || '4',
	    overdueHours: adminTaskSlaOverdueInput.value || '24',
	    healthWindowHours: notificationHealthWindowInput.value || '24',
	    healthStaleMinutes: notificationHealthStaleInput.value || '15',
	  });
	  if (operationQueueSourceInput.value && operationQueueSourceInput.value !== 'all') params.set('source', operationQueueSourceInput.value);
	  if (operationQueuePriorityInput.value && operationQueuePriorityInput.value !== 'all') params.set('priority', operationQueuePriorityInput.value);
	  if (operationQueueOwnerInput.value.trim()) params.set('owner', operationQueueOwnerInput.value.trim());
	  if (operationQueueKeywordInput.value.trim()) params.set('keyword', operationQueueKeywordInput.value.trim());
	  return params;
	}
	function operationQueueAssignmentParams(limit) {
	  const params = operationQueueParams(limit);
	  params.set('currentOnly', operationQueueAssignmentCurrentOnlyInput.value || 'true');
	  if (operationQueueAssignmentDueStateInput.value && operationQueueAssignmentDueStateInput.value !== 'all') {
	    params.set('dueState', operationQueueAssignmentDueStateInput.value);
	  }
	  return params;
	}
	function renewalForecastParams() {
	  const params = new URLSearchParams({
	    tenantLimit: customerSuccessTenantLimitInput.value || '500',
	    days: renewalForecastDaysInput.value || '90',
	    billingLimit: '1000',
	    taskLimit: '1000',
	  });
	  if (renewalForecastBucketInput.value && renewalForecastBucketInput.value !== 'all') {
	    params.set('bucket', renewalForecastBucketInput.value);
	  }
	  if (renewalForecastPricedInput.value && renewalForecastPricedInput.value !== 'all') {
	    params.set('priced', renewalForecastPricedInput.value);
	  }
	  if (renewalForecastPackageCodeInput.value.trim()) {
	    params.set('packageCode', renewalForecastPackageCodeInput.value.trim());
	  }
	  if (renewalForecastOwnerInput.value.trim()) {
	    params.set('owner', renewalForecastOwnerInput.value.trim());
	  }
		  if (renewalForecastTaskStatusInput.value && renewalForecastTaskStatusInput.value !== 'all') {
		    params.set('taskStatus', renewalForecastTaskStatusInput.value);
		  }
		  return params;
		}
	    function tenantLifecycleParams(limit) {
	      const params = new URLSearchParams({
	        tenantId: String(tenantInput.value.trim() || ''),
	        expiringDays: expiringInput.value || '30',
	        limit: String(limit || 1000),
	      });
	      const source = tenantLifecycleSourceInput.value || 'all';
	      const eventType = tenantLifecycleEventTypeInput.value.trim();
	      const status = tenantLifecycleStatusInput.value.trim();
	      const keyword = tenantLifecycleKeywordInput.value.trim();
	      if (source && source !== 'all') params.set('source', source);
	      if (eventType) params.set('eventType', eventType);
	      if (status) params.set('status', status);
	      if (keyword) params.set('keyword', keyword);
	      return params;
	    }
    function subscriptionParams(limit) {
      const params = new URLSearchParams({ limit: String(limit || 100) });
      const status = subscriptionStatusInput.value || 'all';
      const access = subscriptionAccessInput.value || 'all';
      const keyword = subscriptionKeywordInput.value.trim();
      if (status !== 'all') params.set('status', status);
      if (access !== 'all') params.set('access', access);
      if (keyword) params.set('keyword', keyword);
      return params;
    }
    function paymentOrderParams(limit) {
      const params = new URLSearchParams({ limit: String(limit || 100) });
      const status = paymentOrderStatusInput.value || 'all';
      const provider = paymentProviderInput.value.trim();
      const keyword = paymentKeywordInput.value.trim();
      if (status !== 'all') params.set('status', status);
      if (provider) params.set('provider', provider);
      if (keyword) params.set('keyword', keyword);
      return params;
    }
    function paymentWebhookEventParams(limit) {
      const params = new URLSearchParams({ limit: String(limit || 50) });
      const provider = paymentProviderInput.value.trim();
      const keyword = paymentKeywordInput.value.trim();
      if (provider) params.set('provider', provider);
      if (keyword) params.set('keyword', keyword);
      return params;
    }
		function paymentRefundParams(limit) {
	  const params = new URLSearchParams({ limit: String(limit || 100) });
	  const status = paymentRefundStatusInput.value || 'all';
	  const provider = paymentProviderInput.value.trim();
	  const orderNo = paymentRefundOrderFilterInput.value.trim();
	  const keyword = paymentRefundKeywordInput.value.trim();
	  if (status !== 'all') params.set('status', status);
	  if (provider) params.set('provider', provider);
	  if (orderNo) params.set('orderNo', orderNo);
	  if (keyword) params.set('keyword', keyword);
		  return params;
		}
		function invoiceDocumentParams(limit) {
		  const params = new URLSearchParams({ limit: String(limit || 100) });
		  const kind = invoiceKindFilterInput.value || 'all';
		  const status = invoiceStatusFilterInput.value || 'all';
		  const tenantId = invoiceTenantFilterInput.value.trim();
		  const keyword = invoiceKeywordFilterInput.value.trim();
		  if (kind !== 'all') params.set('kind', kind);
		  if (status !== 'all') params.set('status', status);
		  if (tenantId) params.set('tenantId', tenantId);
		  if (keyword) params.set('keyword', keyword);
		  return params;
		}
		function paymentSettlementBatchParams(limit) {
		  const params = new URLSearchParams({ limit: String(limit || 100) });
		  const provider = paymentSettlementProviderInput.value.trim();
		  const status = paymentSettlementStatusInput.value || 'all';
		  const keyword = paymentSettlementKeywordInput.value.trim();
		  if (provider) params.set('provider', provider);
		  if (status !== 'all') params.set('status', status);
		  if (keyword) params.set('keyword', keyword);
		  return params;
		}
		function paymentSettlementEntryParams(limit) {
		  const params = new URLSearchParams({ limit: String(limit || 100) });
		  const batchNo = paymentSettlementSelectedBatchInput.value.trim();
		  const reconciliationStatus = paymentSettlementReconciliationStatusInput.value || 'all';
		  const handlingStatus = paymentSettlementHandlingStatusInput.value || 'all';
		  const keyword = paymentSettlementEntryKeywordInput.value.trim();
		  if (batchNo) params.set('batchNo', batchNo);
		  if (reconciliationStatus !== 'all') params.set('reconciliationStatus', reconciliationStatus);
		  if (handlingStatus !== 'all') params.set('handlingStatus', handlingStatus);
		  if (keyword) params.set('keyword', keyword);
		  return params;
		}
		    function filenameFromDisposition(res, fallback) {
	      const disposition = res.headers.get('Content-Disposition') || '';
	      const match = disposition.match(/filename="?([^"]+)"?/);
      return match ? match[1] : fallback;
    }
    async function downloadCSV(kind) {
		      const configs = {
		        tenants: { params: tenantExportParams(1000), fallback: 'mochat-saas-tenants.csv' },
	        tenantLifecycle: { params: tenantLifecycleParams(1000), fallback: 'mochat-saas-tenantLifecycle.csv' },
		        usage: { params: tenantExportParams(1000), fallback: 'mochat-saas-usage.csv' },
			        risk: { params: riskParams(1000), fallback: 'mochat-saas-risk.csv' },
			        customerSuccess: { params: customerSuccessParams(1000), fallback: 'mochat-saas-customerSuccess.csv' },
			        customerSuccessOwners: { params: customerSuccessParams(1000), fallback: 'mochat-saas-customerSuccessOwners.csv' },
			        renewalForecast: { params: renewalForecastParams(), fallback: 'mochat-saas-renewalForecast.csv' },
			        renewalForecastOwners: { params: renewalForecastParams(), fallback: 'mochat-saas-renewalForecastOwners.csv' },
				        riskFollowUps: { params: riskTaskParams(1000), fallback: 'mochat-saas-riskFollowUps.csv' },
			        riskFollowUpOwners: { params: riskTaskParams(1000), fallback: 'mochat-saas-riskFollowUpOwners.csv' },
			        dailyReport: { params: dailyReportParams(100), fallback: 'mochat-saas-dailyReport.csv' },
			        businessMetrics: { params: businessMetricsParams(), fallback: 'mochat-saas-businessMetrics.csv' },
			        businessTrends: { params: businessTrendsParams(), fallback: 'mochat-saas-businessTrends.csv' },
			        operationQueue: { params: operationQueueParams(1000), fallback: 'mochat-saas-operationQueue.csv' },
			        operationQueueOwners: { params: operationQueueParams(1000), fallback: 'mochat-saas-operationQueueOwners.csv' },
			        operationQueueAssignments: { params: operationQueueAssignmentParams(1000), fallback: 'mochat-saas-operationQueueAssignments.csv' },
		        tasks: { params: adminTaskParams(1000), fallback: 'mochat-saas-tasks.csv' },
		        taskSla: { params: adminTaskSlaParams(1000), fallback: 'mochat-saas-taskSla.csv' },
		        packages: { params: new URLSearchParams({ type: 'packages' }), fallback: 'mochat-saas-packages.csv' },
		        subscriptions: { params: subscriptionParams(1000), fallback: 'mochat-saas-subscriptions.csv' },
			        paymentOrders: { params: paymentOrderParams(1000), fallback: 'mochat-saas-payment-orders.csv' },
			        paymentRefunds: { params: paymentRefundParams(1000), fallback: 'mochat-saas-payment-refunds.csv' },
			        invoiceDocuments: { params: invoiceDocumentParams(1000), fallback: 'mochat-saas-invoice-documents.csv' },
			        paymentSettlementBatches: { params: paymentSettlementBatchParams(1000), fallback: 'mochat-saas-payment-settlement-batches.csv' },
			        paymentSettlementEntries: { params: paymentSettlementEntryParams(1000), fallback: 'mochat-saas-payment-settlement-entries.csv' },
        alerts: { params: alertParams(1000), fallback: 'mochat-saas-alerts.csv' },
	        notifications: { params: notificationParams(1000), fallback: 'mochat-saas-notifications.csv' },
	        notificationHealth: { params: notificationHealthParams(1000), fallback: 'mochat-saas-notificationHealth.csv' },
	        notificationSlo: { params: notificationSloParams(1000), fallback: 'mochat-saas-notificationSlo.csv' },
				        operations: { params: operationParams(1000), fallback: 'mochat-saas-operations.csv' },
				        billingEvents: { params: billingParams(1000), fallback: 'mochat-saas-billingEvents.csv' },
				        billingReconciliation: { params: billingReconciliationParams(1000), fallback: 'mochat-saas-billingReconciliation.csv' },
				        billingReconciliationFollowUps: { params: billingFollowParams(1000), fallback: 'mochat-saas-billingReconciliationFollowUps.csv' },
				        billingReconciliationFollowUpOwners: { params: billingFollowParams(1000), fallback: 'mochat-saas-billingReconciliationFollowUpOwners.csv' },
			      };
      const config = configs[kind];
      if (!config) return;
      config.params.set('type', kind);
      statusEl.textContent = 'CSV 生成中';
      try {
        const res = await fetch('/dashboard/saasAdmin/export?' + config.params.toString(), { headers: authHeader() });
        if (!res.ok) {
          let message = 'HTTP ' + res.status;
          try {
            const body = await res.clone().json();
            message = body.msg || message;
          } catch (_) {}
          throw new Error(message);
        }
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = filenameFromDisposition(res, config.fallback);
        document.body.appendChild(link);
        link.click();
        link.remove();
        window.setTimeout(() => URL.revokeObjectURL(url), 1000);
        statusEl.textContent = 'CSV 已生成';
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    function fmt(v) {
      if (v === null || v === undefined || v === '') return '-';
      return String(v);
    }
    function esc(v) {
      return fmt(v).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }
    function pct(value) {
      if (!Number.isFinite(Number(value)) || Number(value) <= 0) return '-';
      return Math.round(Number(value) * 100) + '%';
    }
    function moneyCents(value) {
      const cents = Number(value || 0);
      if (!Number.isFinite(cents)) return '-';
      return (cents / 100).toFixed(2);
    }
    function pill(text, type) {
      return '<span class="pill ' + (type || '') + '">' + esc(text) + '</span>';
    }
	    function renderSummary(summary, tenantPopulation) {
	      const tenantLabel = tenantPopulation === 'business_tenants' ? '业务租户' : '租户数';
	      const tiles = [
	        [tenantLabel, summary.tenantCount],
	        ['启用套餐', summary.enabledPackageCount],
	        ['开通记录', summary.activeTenantPackageCount],
        ['企业数', summary.corpCount],
        ['子账号', summary.userCount],
        ['打开告警', summary.openAlertCount],
        ['待发通知', summary.pendingNotificationCount],
	        ['即将/已经到期', String(summary.expiringSoonTenantCount) + '/' + String(summary.expiredTenantCount)],
	      ];
	      summaryEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + esc(value) + '</div></div>').join('');
	    }
	    function renderBusinessMetrics(data) {
	      const summary = (data || {}).summary || {};
	      businessMetricsHintEl.textContent = (data || {}).estimated ? '基于最近续费账单估算' : '';
	      const tiles = [
	        ['估算 MRR', esc(moneyCents(summary.estimatedMrrCents)) + '<br><span class="subtitle">ARPA ' + esc(moneyCents(summary.estimatedArpaCents)) + '</span>'],
	        ['估算 ARR', esc(moneyCents(summary.estimatedArrCents)) + '<br><span class="subtitle">已定价租户 ' + esc(summary.pricedTenantCount || 0) + '</span>'],
	        ['风险收入', esc(moneyCents(summary.atRiskMrrCents)) + '<br><span class="subtitle">风险租户 ' + esc(summary.atRiskTenantCount || 0) + '</span>'],
	        ['到期收入', esc(moneyCents((summary.expiringSoonMrrCents || 0) + (summary.expiredMrrCents || 0))) + '<br><span class="subtitle">即将/已到期 ' + esc(summary.expiringSoonTenantCount || 0) + '/' + esc(summary.expiredTenantCount || 0) + '</span>'],
	        ['近期净收入', esc(moneyCents(summary.recentBillingAmountCents)) + '<br><span class="subtitle">毛收入 ' + esc(moneyCents(summary.recentGrossAmountCents)) + ' / 退款 ' + esc(moneyCents(summary.recentRefundAmountCents)) + '</span>'],
	        ['未知价格', esc(summary.unknownPriceTenantCount || 0) + '<br><span class="subtitle">缺价格套餐 ' + esc(summary.missingBillingPackageCount || 0) + '</span>'],
	      ];
	      businessMetricsEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	      const packages = (data || {}).packages || [];
	      if (!packages.length) {
	        businessPackagesEl.innerHTML = '<tr><td colspan="5" class="empty">暂无数据</td></tr>';
	        return;
	      }
	      businessPackagesEl.innerHTML = packages.map(item => {
	        const status = item.packageStatus === 1 ? pill('启用', 'ok') : pill('停用', 'danger');
	        const risks = '风险 ' + fmt(item.atRiskTenantCount || 0) + ' / 即将 ' + fmt(item.expiringSoonTenantCount || 0) + ' / 已到期 ' + fmt(item.expiredTenantCount || 0);
	        const riskMrr = moneyCents((item.atRiskMrrCents || 0) + (item.expiringSoonMrrCents || 0) + (item.expiredMrrCents || 0));
	        return '<tr>' +
	          '<td>' + esc(item.packageName || item.packageCode || '-') + '<br><span class="subtitle">' + esc(item.packageCode || '-') + ' ' + status + '</span></td>' +
	          '<td>' + esc(item.tenantCount || 0) + '<br><span class="subtitle">定价 ' + esc(item.pricedTenantCount || 0) + ' / 未知 ' + esc(item.unknownPriceTenantCount || 0) + '</span></td>' +
	          '<td>' + esc(moneyCents(item.estimatedMrrCents)) + '<br><span class="subtitle">ARR ' + esc(moneyCents(item.estimatedArrCents)) + '</span></td>' +
	          '<td>' + esc(risks) + '<br><span class="subtitle">敞口 ' + esc(riskMrr) + '</span></td>' +
	          '<td>' + esc(moneyCents(item.latestAmountCents)) + '<br><span class="subtitle">#' + esc(item.latestBillingEventId || '-') + ' ' + esc(item.latestBillingAt || '-') + '</span></td>' +
	        '</tr>';
	      }).join('');
	    }
	    function renderBusinessTrends(data) {
	      const summary = (data || {}).summary || {};
	      const filters = (data || {}).filters || {};
	      businessTrendsHintEl.textContent = '最近 ' + esc(filters.months || summary.monthCount || 0) + ' 个月';
	      const tiles = [
	        ['趋势净收入', esc(moneyCents(summary.billingAmountCents)) + '<br><span class="subtitle">毛收入 ' + esc(moneyCents(summary.grossAmountCents)) + ' / 退款 ' + esc(moneyCents(summary.refundAmountCents)) + '</span>'],
	        ['覆盖租户', esc(summary.tenantCount || 0) + '<br><span class="subtitle">套餐 ' + esc(summary.packageCount || 0) + '</span>'],
	        ['待处理续费', esc(summary.actionableTaskCount || 0) + '<br><span class="subtitle">待应用/阻断/失败 ' + esc(summary.pendingTaskCount || 0) + '/' + esc(summary.blockedTaskCount || 0) + '/' + esc(summary.failedTaskCount || 0) + '</span>'],
	        ['已应用任务', esc(summary.appliedTaskCount || 0) + '<br><span class="subtitle">已取消 ' + esc(summary.canceledTaskCount || 0) + '</span>'],
	      ];
	      businessTrendsEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	      const months = (data || {}).months || [];
	      if (!months.length) {
	        businessTrendMonthsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无数据</td></tr>';
	      } else {
	        businessTrendMonthsEl.innerHTML = months.map(item => {
	          const packages = (item.packages || []).slice(0, 3).map(pkg => (pkg.packageName || pkg.packageCode || '-') + ' ' + moneyCents(pkg.amountCents)).join(' / ');
	          return '<tr>' +
	            '<td>' + esc(item.month || '-') + '</td>' +
	            '<td>' + esc(moneyCents(item.amountCents)) + '</td>' +
	            '<td>' + esc(item.eventCount || 0) + ' / 续费 ' + esc(item.renewalCount || 0) + ' / 退款 ' + esc(item.refundCount || 0) + '</td>' +
	            '<td>' + esc(item.tenantCount || 0) + ' / ' + esc(item.packageCount || 0) + '</td>' +
	            '<td>' + esc(packages || '-') + '</td>' +
	          '</tr>';
	        }).join('');
	      }
	      const funnel = (data || {}).renewalFunnel || {};
	      const tasks = funnel.recentTasks || [];
	      if (!tasks.length) {
	        businessRenewalFunnelEl.innerHTML = '<tr><td colspan="5" class="empty">暂无续费任务</td></tr>';
	      } else {
	        businessRenewalFunnelEl.innerHTML = tasks.map(task => {
	          const statusType = task.status === 'applied' ? 'ok' : (task.status === 'failed' || task.status === 'blocked' ? 'danger' : '');
	          return '<tr>' +
	            '<td>#' + esc(task.id) + '<br><span class="subtitle">' + esc(task.createdAt || '-') + '</span></td>' +
	            '<td>' + esc(task.tenantId || '-') + '</td>' +
	            '<td>' + esc(task.packageCode || '-') + '</td>' +
	            '<td>' + pill(task.status || '-', statusType) + '</td>' +
	            '<td>' + esc(task.lastError || '-') + '</td>' +
	          '</tr>';
	        }).join('');
	      }
	    }
	function renderRenewalForecast(data) {
	  const summary = (data || {}).summary || {};
	  const filters = (data || {}).filters || {};
	  const filterParts = ['未来 ' + esc(filters.days || 0) + ' 天'];
	  if (filters.bucket && filters.bucket !== 'all') filterParts.push('窗口 ' + esc(filters.bucket));
	  if (filters.priced && filters.priced !== 'all') filterParts.push('定价 ' + esc(filters.priced));
	  if (filters.packageCode) filterParts.push('套餐 ' + esc(filters.packageCode));
	  if (filters.owner) filterParts.push('负责人 ' + esc(filters.owner));
	  if (filters.taskStatus && filters.taskStatus !== 'all') filterParts.push('任务 ' + esc(filters.taskStatus));
	  renewalForecastHintEl.textContent = filterParts.join(' / ');
	  const tiles = [
	    ['预测续费', esc(moneyCents(summary.renewalAmountCents)) + '<br><span class="subtitle">租户 ' + esc(summary.forecastTenantCount || 0) + ' / 定价 ' + esc(summary.pricedTenantCount || 0) + '</span>'],
	    ['30天内', esc(moneyCents(summary.dueWithin30AmountCents)) + '<br><span class="subtitle">租户 ' + esc(summary.dueWithin30TenantCount || 0) + '</span>'],
	    ['已到期', esc(moneyCents(summary.expiredAmountCents)) + '<br><span class="subtitle">租户 ' + esc(summary.expiredTenantCount || 0) + '</span>'],
	    ['待处理任务', esc(summary.actionableTaskCount || 0) + '<br><span class="subtitle">待/阻/失败 ' + esc(summary.pendingTaskCount || 0) + '/' + esc(summary.blockedTaskCount || 0) + '/' + esc(summary.failedTaskCount || 0) + '</span>'],
	    ['未知价格', esc(summary.unknownPriceTenantCount || 0) + '<br><span class="subtitle">MRR ' + esc(moneyCents(summary.estimatedMrrCents)) + '</span>'],
	  ];
	  renewalForecastEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	  const buckets = (data || {}).buckets || [];
	  if (!buckets.length) {
	    renewalForecastBucketsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无数据</td></tr>';
	  } else {
	    renewalForecastBucketsEl.innerHTML = buckets.map(item => {
	      return '<tr>' +
	        '<td>' + esc(item.label || item.bucket || '-') + '</td>' +
	        '<td>' + esc(item.tenantCount || 0) + '<br><span class="subtitle">定价 ' + esc(item.pricedTenantCount || 0) + '</span></td>' +
	        '<td>' + esc(moneyCents(item.renewalAmountCents)) + '<br><span class="subtitle">MRR ' + esc(moneyCents(item.estimatedMrrCents)) + '</span></td>' +
	        '<td>' + esc(item.unknownPriceTenantCount || 0) + '</td>' +
	        '<td>' + esc(item.actionableTaskCount || 0) + '</td>' +
	      '</tr>';
	    }).join('');
	  }
	  const owners = (data || {}).owners || [];
	  if (!owners.length) {
	    renewalForecastOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">暂无负责人</td></tr>';
	  } else {
	    renewalForecastOwnersEl.innerHTML = owners.map(item => {
	      const dueText = '已到期 ' + fmt(item.expiredTenantCount || 0) + ' / 30天内 ' + fmt(item.dueWithin30TenantCount || 0);
	      const laterText = '31-60 ' + fmt(item.due31To60TenantCount || 0) + ' / 61-90 ' + fmt(item.due61To90TenantCount || 0) + ' / 更远 ' + fmt(item.dueLaterTenantCount || 0);
	      const taskText = '待处理 ' + fmt(item.actionableTaskCount || 0) + ' / 已应用 ' + fmt(item.appliedTaskCount || 0);
	      const taskDetail = '待/阻/失败 ' + fmt(item.pendingTaskCount || 0) + '/' + fmt(item.blockedTaskCount || 0) + '/' + fmt(item.failedTaskCount || 0);
	      const topTenants = (item.topTenants || []).map(tenant => (tenant.tenantName || '-') + ' ' + (tenant.priced ? moneyCents(tenant.renewalAmountCents) : '未知')).join(' / ');
	      return '<tr>' +
	        '<td>' + esc(item.owner || '未分配') + '<br><span class="subtitle">租户 ' + esc(item.tenantCount || 0) + ' / 下次 ' + esc(item.nextFollowUpAt || '-') + '</span></td>' +
	        '<td>' + esc(moneyCents(item.renewalAmountCents)) + '<br><span class="subtitle">定价 ' + esc(item.pricedTenantCount || 0) + ' / 未知 ' + esc(item.unknownPriceTenantCount || 0) + '</span></td>' +
	        '<td>' + esc(dueText) + '<br><span class="subtitle">' + esc(laterText) + '</span></td>' +
	        '<td>' + esc(taskText) + '<br><span class="subtitle">' + esc(taskDetail) + '</span></td>' +
	        '<td>' + esc(topTenants || '-') + '</td>' +
	      '</tr>';
	    }).join('');
	  }
	  const tenants = (data || {}).tenants || [];
	  if (!tenants.length) {
	    renewalForecastTenantsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无到期租户</td></tr>';
	  } else {
	    renewalForecastTenantsEl.innerHTML = tenants.map(item => {
	      const taskSummary = item.taskSummary || {};
	      const latestTask = item.latestTask || {};
	      const taskText = latestTask.id ? ('#' + fmt(latestTask.id) + ' ' + fmt(latestTask.status || '-')) : '无任务';
	      const taskDetail = '待处理 ' + fmt(taskSummary.actionableCount || 0) + ' / 已应用 ' + fmt(taskSummary.appliedCount || 0);
	      const priceText = item.priced ? moneyCents(item.renewalAmountCents) : '未知';
	      return '<tr>' +
	        '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId || '-') + ' / ' + esc(item.owner || '未分配') + '</span></td>' +
	        '<td>' + esc(item.bucketLabel || '-') + '<br><span class="subtitle">' + esc(item.expiresAt || '-') + ' / ' + esc(item.daysUntil) + ' 天</span></td>' +
	        '<td>' + esc(item.packageName || item.packageCode || '-') + '<br><span class="subtitle">' + esc(item.packageCode || '-') + '</span></td>' +
	        '<td>' + esc(priceText) + '<br><span class="subtitle">账单 #' + esc(item.latestBillingEventId || '-') + '</span></td>' +
	        '<td>' + esc(taskText) + '<br><span class="subtitle">' + esc(taskDetail) + '</span></td>' +
	      '</tr>';
	    }).join('');
	  }
	}
	    function renderDailyReport(data) {
	      const summary = (data && data.summary) || {};
	      const windowData = (data && data.window) || {};
	      const amount = Number(summary.windowBillingAmountCents || 0) > 0 ? (Number(summary.windowBillingAmountCents) / 100).toFixed(2) : '0.00';
	      const windowText = windowData.date ? ('日期 ' + windowData.date + ' / ' + esc(windowData.days || 1) + ' 天') : '';
	      dailyReportHintEl.textContent = windowText;
	      const tiles = [
	        ['风险租户', esc(summary.riskTenantCount || 0) + '<br><span class="subtitle">高危 ' + esc(summary.criticalRiskTenantCount || 0) + ' / 高 ' + esc(summary.highRiskTenantCount || 0) + '</span>'],
	        ['打开跟进', esc(summary.openRiskFollowUpCount || 0) + '<br><span class="subtitle">逾期 ' + esc(summary.overdueRiskFollowUpCount || 0) + ' / 7 天内 ' + esc(summary.dueSoonRiskFollowUpCount || 0) + '</span>'],
	        ['任务SLA', esc(summary.taskSlaActiveCount || 0) + '<br><span class="subtitle">逾期 ' + esc(summary.taskSlaOverdueCount || 0) + ' / 预警 ' + esc(summary.taskSlaWarningCount || 0) + '</span>'],
	        ['告警通知', esc(summary.openAlertCount || 0) + '<br><span class="subtitle">失败 ' + esc(summary.retryableNotificationCount || 0) + ' / 待发 ' + esc(summary.pendingNotificationCount || 0) + '</span>'],
		['今日流水', esc(amount) + '<br><span class="subtitle">操作 ' + esc(summary.windowOperationCount || 0) + ' / 认领 ' + esc(summary.windowQueueAssignmentCount || 0) + ' / 账单 ' + esc(summary.windowBillingEventCount || 0) + '</span>'],
	      ];
	      dailyReportEl.innerHTML = tiles.map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
	      renderDailyReportDetails(data || {});
	    }
	    function clearDailyReportDetails(message) {
	      const text = esc(message || '暂无数据');
	      dailyReportOwnersEl.innerHTML = '<tr><td colspan="4" class="empty">' + text + '</td></tr>';
	      dailyReportTaskSlaEl.innerHTML = '<tr><td colspan="4" class="empty">' + text + '</td></tr>';
	      dailyReportNotificationsEl.innerHTML = '<tr><td colspan="4" class="empty">' + text + '</td></tr>';
	      dailyReportQueueAssignmentsEl.innerHTML = '<tr><td colspan="4" class="empty">' + text + '</td></tr>';
	      dailyReportActionsEl.innerHTML = '<tr><td colspan="3" class="empty">' + text + '</td></tr>';
	      dailyReportBillingEl.innerHTML = '<tr><td colspan="4" class="empty">' + text + '</td></tr>';
	      document.getElementById('dailyReportOwnerCount').textContent = '';
	      document.getElementById('dailyReportTaskSlaCount').textContent = '';
	      document.getElementById('dailyReportNotificationCount').textContent = '';
	      document.getElementById('dailyReportQueueAssignmentCount').textContent = '';
	      document.getElementById('dailyReportActionCount').textContent = '';
	      document.getElementById('dailyReportBillingCount').textContent = '';
	    }
	    function renderDailyReportDetails(data) {
	      const windowData = data.window || {};
	      const owners = ((data.riskFollowUps || {}).owners || []);
	      document.getElementById('dailyReportOwnerCount').textContent = owners.length ? owners.length + ' 个负责人' : '';
	      if (!owners.length) {
	        dailyReportOwnersEl.innerHTML = '<tr><td colspan="4" class="empty">暂无数据</td></tr>';
	      } else {
	        dailyReportOwnersEl.innerHTML = owners.map(item => {
	          const overdue = Number(item.overdueCount || 0);
	          const dueSoon = Number(item.dueSoonCount || 0);
	          const open = Number(item.openCount || 0);
	          const openBadge = overdue > 0 ? pill(open + ' 打开', 'danger') : (dueSoon > 0 ? pill(open + ' 打开', 'warning') : pill(open + ' 打开', open > 0 ? '' : 'ok'));
	          return '<tr>' +
	            '<td>' + esc(item.owner || '未分配') + '<br><span class="subtitle">总计 ' + esc(item.totalCount || 0) + '</span></td>' +
	            '<td>' + openBadge + '</td>' +
	            '<td>' + esc(overdue) + ' 逾期<br><span class="subtitle">' + esc(dueSoon) + ' 个 7 天内</span></td>' +
	            '<td>' + esc(item.nextFollowUpAt || '-') + '<br><span class="subtitle">' + esc(item.latestFollowUpAt || '-') + '</span></td>' +
	          '</tr>';
	        }).join('');
	      }

	      const taskSla = data.taskSla || {};
	      const taskSlaTasks = taskSla.tasks || [];
	      const taskSlaSummary = taskSla.summary || {};
	      const taskSlaText = '活跃 ' + esc(taskSlaSummary.taskCount || 0) + ' / 逾期 ' + esc(taskSlaSummary.overdueCount || 0) + ' / 预警 ' + esc(taskSlaSummary.warningCount || 0);
	      document.getElementById('dailyReportTaskSlaCount').textContent = taskSlaTasks.length ? taskSlaText : '';
	      if (!taskSlaTasks.length) {
	        dailyReportTaskSlaEl.innerHTML = '<tr><td colspan="4" class="empty">暂无数据</td></tr>';
	      } else {
	        dailyReportTaskSlaEl.innerHTML = taskSlaTasks.map(item => {
	          const task = item.task || {};
	          const breach = item.breachHours > 0 ? '<br><span class="subtitle">超时 ' + esc(item.breachHours) + ' 小时</span>' : '';
	          const status = item.slaStatus === 'overdue' ? pill('逾期', 'danger') : (item.slaStatus === 'warning' ? pill('预警', 'warning') : pill(adminTaskSlaStatusLabel(item.slaStatus), item.slaStatus === 'fresh' ? 'ok' : ''));
	          return '<tr>' +
	            '<td>#' + esc(task.id || '-') + '<br><span class="subtitle">' + esc(adminTaskTypeLabel(task.taskType)) + ' / ' + esc(packageSyncTaskStatusLabel(task.status)) + '</span></td>' +
	            '<td>' + status + '<br><span class="subtitle">已过 ' + esc(item.ageHours || 0) + ' 小时</span>' + breach + '</td>' +
	            '<td>' + esc(item.owner || '未分配') + '<br><span class="subtitle">用户 ' + esc(task.actorUserId || 0) + ' / 租户 ' + esc(task.actorTenantId || 0) + '</span></td>' +
	            '<td>' + esc(adminTaskResultText(task)) + '<br><span class="subtitle">' + esc(task.createdAt || '-') + '</span></td>' +
	          '</tr>';
	        }).join('');
	      }

	      const notificationData = data.notifications || {};
	      const notifications = notificationData.items || [];
	      const closedNotifications = notificationData.closedItems || [];
	      const dailyNotificationRows = notifications.concat(closedNotifications);
	      const notificationSummary = notificationData.summary || {};
	      const notificationText = '可重试 ' + esc(notificationSummary.retryableCount || 0) + ' / 已关 ' + esc(notificationSummary.closedCount || 0);
	      document.getElementById('dailyReportNotificationCount').textContent = dailyNotificationRows.length ? notificationText : '';
	      if (!dailyNotificationRows.length) {
	        dailyReportNotificationsEl.innerHTML = '<tr><td colspan="4" class="empty">暂无数据</td></tr>';
	      } else {
	        dailyReportNotificationsEl.innerHTML = dailyNotificationRows.map(item => {
	          const status = item.status === 'dead' ? pill('已耗尽', 'danger') : (item.status === 'failed' ? pill('失败', 'warning') : (item.status === 'closed' ? pill('已关闭', '') : pill(item.status || '-', '')));
	          return '<tr>' +
	            '<td>ID ' + esc(item.tenantId) + '<br><span class="subtitle">' + esc(item.notificationKey || '-') + '</span></td>' +
	            '<td>' + esc(item.metricLabel || item.metric || '-') + '<br><span class="subtitle">' + esc(item.alertType || '-') + '</span></td>' +
	            '<td>' + status + '<br><span class="subtitle">' + esc(item.attempts || 0) + '/' + esc(item.maxAttempts || 0) + '</span></td>' +
	            '<td>' + esc(item.lastError || '-') + '<br><span class="subtitle">' + esc(item.nextRetryAt || item.updatedAt || '-') + '</span></td>' +
	          '</tr>';
	        }).join('');
	      }

	      const queueAssignmentData = data.operationQueueAssignments || {};
	      const queueAssignments = queueAssignmentData.assignments || [];
	      const queueAssignmentSummary = queueAssignmentData.summary || {};
	      const queueAssignmentText = '认领 ' + esc(queueAssignmentSummary.assignmentCount || 0) + ' / 任务SLA ' + esc(queueAssignmentSummary.taskSlaCount || 0) + ' / 通知 ' + esc((queueAssignmentSummary.notificationCount || 0) + (queueAssignmentSummary.closedNotificationCount || 0));
	      document.getElementById('dailyReportQueueAssignmentCount').textContent = queueAssignments.length ? queueAssignmentText : '';
	      if (!queueAssignments.length) {
	        dailyReportQueueAssignmentsEl.innerHTML = '<tr><td colspan="4" class="empty">暂无数据</td></tr>';
	      } else {
	        dailyReportQueueAssignmentsEl.innerHTML = queueAssignments.map(item => {
	          const status = riskFollowStatusText(item.status || '');
	          return '<tr>' +
	            '<td>' + esc(item.targetName || item.objectId || '-') + '<br><span class="subtitle">' + esc(item.source || '-') + ' / 租户 ' + esc(item.tenantId || '-') + ' / 操作 ' + esc(item.operationId || '-') + '</span></td>' +
	            '<td>' + esc(item.owner || '-') + '<br><span class="subtitle">操作者 ' + esc(item.actorUserId || '-') + '</span></td>' +
	            '<td>' + pill(status[0], status[1]) + '<br><span class="subtitle">' + esc(item.nextFollowUpAt || '-') + '</span></td>' +
	            '<td>' + esc(item.remark || '-') + '<br><span class="subtitle">' + esc(item.assignedAt || '-') + '</span></td>' +
	          '</tr>';
	        }).join('');
	      }

	      const actions = ((data.operations || {}).summary || []);
	      document.getElementById('dailyReportActionCount').textContent = actions.length ? actions.length + ' 类动作' : '';
	      if (!actions.length) {
	        dailyReportActionsEl.innerHTML = '<tr><td colspan="3" class="empty">暂无数据</td></tr>';
	      } else {
	        const windowLabel = (windowData.startDate || windowData.date || '-') + ' 至 ' + (windowData.endDate || windowData.date || '-');
	        dailyReportActionsEl.innerHTML = actions.map(item => {
	          return '<tr>' +
	            '<td>' + esc(item.action || '-') + '</td>' +
	            '<td>' + esc(item.count || 0) + '</td>' +
	            '<td>' + esc(windowLabel) + '</td>' +
	          '</tr>';
	        }).join('');
	      }

	      const billingEvents = ((data.billing || {}).billingEvents || []);
	      document.getElementById('dailyReportBillingCount').textContent = billingEvents.length ? billingEvents.length + ' 条' : '';
	      if (!billingEvents.length) {
	        dailyReportBillingEl.innerHTML = '<tr><td colspan="4" class="empty">暂无数据</td></tr>';
	      } else {
	        dailyReportBillingEl.innerHTML = billingEvents.map(item => {
	          const eventAmount = Number(item.amountCents || 0) > 0 ? (Number(item.amountCents) / 100).toFixed(2) + ' ' + esc(item.currency || 'CNY') : '-';
	          return '<tr>' +
	            '<td>' + esc(item.createdAt || item.paidAt || '-') + '</td>' +
	            '<td>ID ' + esc(item.tenantId) + '<br><span class="subtitle">' + esc(item.eventType || '-') + '</span></td>' +
	            '<td>' + esc(item.packageName || item.packageCode || '-') + '</td>' +
	            '<td>' + esc(eventAmount) + '<br><span class="subtitle">' + esc(item.externalOrderNo || item.paymentMethod || '-') + '</span></td>' +
	          '</tr>';
	        }).join('');
	      }
	    }
	    function renderTenantDetail(data) {
      if (!data || !data.tenant) {
        tenantDetailHintEl.textContent = '';
        tenantDetailEl.innerHTML = '<div class="tile"><div class="label">租户</div><div class="value">-</div></div>' +
          '<div class="tile"><div class="label">套餐</div><div class="value">-</div></div>' +
          '<div class="tile"><div class="label">最高用量</div><div class="value">-</div></div>' +
          '<div class="tile"><div class="label">近期操作</div><div class="value">-</div></div>';
        renderUsageMetrics([]);
        return;
      }
      const tenant = data.tenant || {};
      const operations = data.operations || [];
      const tenantStatus = tenant.tenantStatus === 2 ? '停用' : '正常';
      const due = tenant.expired ? '已到期' : (tenant.expiringSoon ? '即将到期' : '正常');
      const usage = tenant.maxUsageLimit > 0 ? fmt(tenant.maxUsageLabel) + ' ' + fmt(tenant.maxUsageCurrent) + '/' + fmt(tenant.maxUsageLimit) + ' ' + pct(tenant.maxUsageRatio) : '-';
      tenantDetailHintEl.textContent = 'ID ' + fmt(tenant.tenantId);
      tenantDetailEl.innerHTML = [
        ['租户', esc(tenant.tenantName) + '<br><span class="subtitle">' + esc(tenantStatus) + '</span>'],
        ['套餐', esc(tenant.packageName || tenant.packageCode) + '<br><span class="subtitle">' + esc(due) + ' ' + esc(tenant.expiresAt) + '</span>'],
        ['最高用量', esc(usage) + '<br><span class="subtitle">打开告警 ' + esc(tenant.openAlertCount) + '</span>'],
        ['近期操作', esc(operations.length) + '<br><span class="subtitle">' + esc(operations[0] ? operations[0].action : '-') + '</span>'],
      ].map(([label, value]) => '<div class="tile"><div class="label">' + esc(label) + '</div><div class="value">' + value + '</div></div>').join('');
    }
    function lifecycleSourceText(source) {
      if (source === 'operation') return '操作';
      if (source === 'billing') return '账单';
      if (source === 'task') return '任务';
      if (source === 'alert') return '告警';
      if (source === 'notification') return '通知';
      return source || '-';
    }
	    function renderTenantLifecycle(data) {
	      const summary = (data || {}).summary || {};
	      const items = (data || {}).timeline || [];
	      if (summary.timelineCount === undefined) {
	        tenantLifecycleCountEl.textContent = '';
	      } else if (summary.filterActive) {
	        tenantLifecycleCountEl.textContent = '命中 ' + esc(summary.timelineCount) + ' / 全部 ' + esc(summary.rawTimelineCount || 0) + ' / 显示 ' + esc(summary.returnedEventCount || items.length);
	      } else {
	        tenantLifecycleCountEl.textContent = '命中 ' + esc(summary.timelineCount) + ' / 显示 ' + esc(summary.returnedEventCount || items.length);
	      }
	      if (!items.length) {
	        tenantLifecycleEl.innerHTML = '<tr><td colspan="5" class="empty">暂无数据</td></tr>';
	        return;
      }
      tenantLifecycleEl.innerHTML = items.map(item => {
        return '<tr>' +
          '<td>' + esc(item.occurredAt || '-') + '<br><span class="subtitle">#' + esc(item.referenceId || '-') + '</span></td>' +
          '<td>' + esc(lifecycleSourceText(item.source)) + '</td>' +
          '<td>' + esc(item.title || item.eventType || '-') + '<br><span class="subtitle">' + esc(item.eventType || '-') + '</span></td>' +
          '<td>' + esc(item.status || '-') + '</td>' +
          '<td>' + esc(item.remark || '-') + '</td>' +
        '</tr>';
      }).join('');
    }
    function usageStatusText(status) {
      if (status === 'exceeded') return ['已超额', 'danger'];
      if (status === 'warning') return ['接近上限', 'warning'];
      if (status === 'unlimited') return ['不限额', 'ok'];
      return ['正常', 'ok'];
    }
    function renderUsageMetrics(items) {
      document.getElementById('usageCount').textContent = items.length ? items.length + ' 项' : '';
      if (!items.length) {
        usageMetricsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      usageMetricsEl.innerHTML = items.map(item => {
        const status = usageStatusText(item.status);
        const used = item.unlimited ? esc(item.current) + '/不限' : esc(item.current) + '/' + esc(item.limit) + ' ' + esc(pct(item.usageRatio));
        const remaining = item.unlimited ? '不限' : esc(item.remaining);
        return '<tr>' +
          '<td>' + esc(item.label || item.metric) + '<br><span class="subtitle">' + esc(item.metric) + ' / ' + esc(item.periodKey) + '</span></td>' +
          '<td>' + pill(status[0], status[1]) + '</td>' +
          '<td>' + used + '</td>' +
          '<td>' + remaining + '</td>' +
          '<td>' + (item.openAlertCount > 0 ? pill(item.openAlertCount, 'warning') : pill('0', 'ok')) + '</td>' +
          '<td>' + esc(item.updatedAt || '-') + '<br><span class="subtitle">' + esc(item.updatedBy || '-') + '</span></td>' +
        '</tr>';
      }).join('');
    }
    function riskLevelText(level) {
      if (level === 'critical') return ['紧急', 'danger'];
      if (level === 'high') return ['高', 'warning'];
      if (level === 'medium') return ['中', 'warning'];
      return ['正常', 'ok'];
    }
    function riskFollowStatusText(status) {
      if (status === 'pending') return ['待跟进', 'warning'];
      if (status === 'contacted') return ['已联系', 'ok'];
      if (status === 'renewal_pending') return ['续费中', 'warning'];
      if (status === 'resolved') return ['已解决', 'ok'];
      if (status === 'ignored') return ['忽略', 'ok'];
      return ['未跟进', ''];
    }
    function riskTaskDueStateText(state) {
      if (state === 'overdue') return ['已逾期', 'danger'];
      if (state === 'due_soon') return ['7 天内', 'warning'];
      if (state === 'future') return ['未来', 'ok'];
      if (state === 'closed') return ['已关闭', 'ok'];
      if (state === 'blocked') return ['阻断', 'danger'];
      if (state === 'normal') return ['正常', 'ok'];
      return ['无日期', ''];
    }
    function customerSuccessPriorityText(priority) {
      if (priority === 'critical') return ['紧急', 'danger'];
      if (priority === 'high') return ['高', 'warning'];
      if (priority === 'medium') return ['中', 'warning'];
      return ['正常', 'ok'];
    }
	function operationQueueSourceText(source) {
	  if (source === 'customer_success') return '客户成功';
	  if (source === 'task_sla') return '任务SLA';
	  if (source === 'billing_follow_up') return '账单跟进';
	  if (source === 'notification') return '失败通知';
	  if (source === 'closed_notification') return '关闭通知';
	  if (source === 'notification_health') return '通知健康';
	  return source || '-';
	}
	function operationQueuePriorityText(priority) {
	  if (priority === 'critical') return ['严重', 'danger'];
	  if (priority === 'high') return ['高', 'warning'];
	  if (priority === 'medium') return ['中', 'warning'];
	  return ['普通', 'ok'];
	}
	function renderOperationQueue(data) {
	  const items = data.items || [];
	  const summary = data.summary || {};
	  const queueCount = Number(summary.queueCount || 0);
	  const returned = Number(summary.returnedCount || items.length || 0);
	  let text = '命中 ' + fmt(queueCount) + ' / 显示 ' + fmt(returned);
	  if (Number(summary.criticalCount || 0) > 0) text += '，严重 ' + fmt(summary.criticalCount);
	  if (Number(summary.highCount || 0) > 0) text += '，高 ' + fmt(summary.highCount);
	  if (Number(summary.notificationCount || 0) > 0) text += '，失败通知 ' + fmt(summary.notificationCount);
	  if (Number(summary.closedNotificationCount || 0) > 0) text += '，关闭通知 ' + fmt(summary.closedNotificationCount);
	  if (Number(summary.notificationHealthCount || 0) > 0) text += '，健康异常 ' + fmt(summary.notificationHealthCount);
	  if (Number(summary.unassignedCount || 0) > 0) text += '，未分配 ' + fmt(summary.unassignedCount);
	  operationQueueCountEl.textContent = text;
	  if (!items.length) {
	    operationQueueEl.innerHTML = '<tr><td colspan="5" class="empty">暂无待办</td></tr>';
	    return;
	  }
	  operationQueueEl.innerHTML = items.map(item => {
	    const priority = operationQueuePriorityText(item.priority);
	    const due = riskTaskDueStateText(item.dueState);
	    const objectLine = [item.objectType, item.objectId].filter(Boolean).join(' # ') || '-';
	    const timeLine = item.updatedAt || item.createdAt || '-';
	    return '<tr>' +
	      '<td>' + esc(operationQueueSourceText(item.source)) + '<br><span class="subtitle">' + esc(item.title || '-') + '</span><br><span class="subtitle">' + esc(item.reason || '-') + '</span></td>' +
	      '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId || '-') + ' / ' + esc(objectLine) + '</span></td>' +
	      '<td>' + pill(priority[0], priority[1]) + '<br><span class="subtitle">' + pill(due[0], due[1]) + ' / ' + esc(item.status || '-') + '</span></td>' +
	      '<td>' + esc(item.owner || '未分配') + '<br><span class="subtitle">已停留 ' + esc(item.ageHours || 0) + ' 小时</span></td>' +
	      '<td>' + esc(item.nextAction || '-') + '<br><span class="subtitle">' + esc(timeLine) + '</span></td>' +
	    '</tr>';
	  }).join('');
	}
	function renderOperationQueueOwners(data) {
	  const owners = data.owners || [];
	  const summary = data.summary || {};
	  const ownerCount = data.ownerCount === undefined ? owners.length : Number(data.ownerCount || 0);
	  const returned = data.returnedCount === undefined ? owners.length : Number(data.returnedCount || 0);
	  let text = ownerCount ? (ownerCount + ' 个负责人 / 显示 ' + returned + ' 个 / 队列 ' + fmt(summary.queueCount || 0)) : '';
	  if (Number(summary.criticalCount || 0) > 0) text += '，严重 ' + fmt(summary.criticalCount);
	  if (Number(summary.unassignedCount || 0) > 0) text += '，未分配 ' + fmt(summary.unassignedCount);
	  operationQueueOwnerCountEl.textContent = text;
	  if (!owners.length) {
	    operationQueueOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">暂无负责人</td></tr>';
	    return;
	  }
	  operationQueueOwnersEl.innerHTML = owners.map(item => {
	    const priority = [
	      '严重 ' + fmt(item.criticalCount || 0),
	      '高 ' + fmt(item.highCount || 0),
	      '中 ' + fmt(item.mediumCount || 0),
	      '普通 ' + fmt(item.normalCount || 0),
	    ].join(' / ');
	    const sources = [
	      '客户成功 ' + fmt(item.customerSuccessCount || 0),
	      'SLA ' + fmt(item.taskSlaCount || 0),
	      '账单 ' + fmt(item.billingFollowUpCount || 0),
	      '通知 ' + fmt(item.notificationCount || 0),
	      '关闭 ' + fmt(item.closedNotificationCount || 0),
	      '健康 ' + fmt(item.notificationHealthCount || 0),
	    ].join(' / ');
	    const topTenants = (item.topTenants || []).map(tenant => (tenant.tenantName || '-') + '#' + (tenant.tenantId || '-') + '(' + (tenant.queueCount || 0) + ')').join('，') || '-';
	    const topItems = (item.topItems || []).map(queueItem => operationQueueSourceText(queueItem.source) + '：' + (queueItem.title || queueItem.id || '-')).join('；') || '-';
	    return '<tr>' +
	      '<td>' + esc(item.owner || '未分配') + '<br><span class="subtitle">待办 ' + esc(item.queueCount || 0) + ' / 租户 ' + esc(item.tenantCount || 0) + ' / 最久 ' + esc(item.maxAgeHours || 0) + ' 小时</span></td>' +
	      '<td>' + esc(priority) + '</td>' +
	      '<td>' + esc(sources) + '</td>' +
	      '<td>' + esc(topTenants) + '</td>' +
	      '<td>' + esc(topItems) + '</td>' +
	    '</tr>';
	  }).join('');
	}
	function renderOperationQueueAssignments(data) {
	  const items = data.assignments || [];
	  const summary = data.summary || {};
	  const total = data.assignmentCount === undefined ? Number(summary.assignmentCount || 0) : Number(data.assignmentCount || 0);
	  const returned = data.returnedCount === undefined ? items.length : Number(data.returnedCount || 0);
	  let text = total ? ('认领 ' + fmt(total) + ' / 显示 ' + fmt(returned)) : '';
	  if (Number(summary.taskSlaCount || 0) > 0) text += '，任务SLA ' + fmt(summary.taskSlaCount);
	  if (Number(summary.notificationCount || 0) > 0) text += '，失败通知 ' + fmt(summary.notificationCount);
	  if (Number(summary.closedNotificationCount || 0) > 0) text += '，关闭通知 ' + fmt(summary.closedNotificationCount);
	  if (Number(summary.notificationHealthCount || 0) > 0) text += '，健康异常 ' + fmt(summary.notificationHealthCount);
	  if (Number(summary.overdueCount || 0) > 0) text += '，逾期 ' + fmt(summary.overdueCount);
	  if (Number(summary.dueSoonCount || 0) > 0) text += '，7天内 ' + fmt(summary.dueSoonCount);
	  if (Number(summary.closedCount || 0) > 0) text += '，已关闭 ' + fmt(summary.closedCount);
	  if (summary.nextFollowUpAt) text += '，最近跟进 ' + esc(summary.nextFollowUpAt);
	  operationQueueAssignmentCountEl.textContent = text;
	  if (!items.length) {
	    operationQueueAssignmentsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无认领记录</td></tr>';
	    return;
	  }
	  operationQueueAssignmentsEl.innerHTML = items.map(item => {
	    const status = riskFollowStatusText(item.status);
	    const due = riskTaskDueStateText(item.dueState);
	    const objectLine = [item.objectType, item.objectId].filter(Boolean).join(' # ') || '-';
	    const closeActions = item.dueState === 'closed' ? '' :
	      '<br><button type="button" data-operation-queue-assignment-close="resolved" data-operation-id="' + esc(item.operationId || '') + '">完成</button> ' +
	      '<button type="button" data-operation-queue-assignment-close="ignored" data-operation-id="' + esc(item.operationId || '') + '">忽略</button>';
	    return '<tr>' +
	      '<td>' + esc(operationQueueSourceText(item.source)) + '<br><span class="subtitle">' + esc(item.targetName || objectLine) + '</span><br><span class="subtitle">租户 ' + esc(item.tenantId || '-') + ' / ' + esc(objectLine) + '</span></td>' +
	      '<td>' + esc(item.owner || '-') + '<br><span class="subtitle">操作人 ' + esc(item.actorUserId || '-') + ' / 租户 ' + esc(item.actorTenantId || '-') + '</span></td>' +
	      '<td>' + pill(status[0], status[1]) + ' ' + pill(due[0], due[1]) + '<br><span class="subtitle">下次 ' + esc(item.nextFollowUpAt || '-') + '</span></td>' +
	      '<td>' + esc(item.remark || '-') + '</td>' +
	      '<td>#' + esc(item.operationId || '-') + '<br><span class="subtitle">' + esc(item.assignedAt || '-') + '</span>' + closeActions + '</td>' +
	    '</tr>';
	  }).join('');
	}
	    function renderCustomerSuccess(data) {
	      const items = data.items || [];
	      const summary = data.summary || {};
      const total = summary.queueCount !== undefined ? Number(summary.queueCount) : items.length;
      const returned = summary.returnedCount !== undefined ? Number(summary.returnedCount) : items.length;
      let text = '待处理 ' + total + ' / 显示 ' + returned;
      if (Number(summary.criticalCount || 0) > 0) text += '，紧急 ' + summary.criticalCount;
      if (Number(summary.highCount || 0) > 0) text += '，高 ' + summary.highCount;
      if (Number(summary.overdueCount || 0) > 0) text += '，逾期 ' + summary.overdueCount;
      if (Number(summary.unassignedCount || 0) > 0) text += '，未分配 ' + summary.unassignedCount;
      customerSuccessCountEl.textContent = text;
      if (!items.length) {
        customerSuccessEl.innerHTML = '<tr><td colspan="6" class="empty">暂无需要处理的客户成功事项</td></tr>';
        return;
      }
      customerSuccessEl.innerHTML = items.map(item => {
        const priority = customerSuccessPriorityText(item.priority);
        const due = riskTaskDueStateText(item.dueState);
        const taskSummary = item.adminTaskSummary || {};
        const signals = [
          '账单跟进 ' + fmt(item.billingFollowUpCount || 0),
          '任务 ' + fmt(taskSummary.actionableCount || 0),
          '通知 ' + fmt(item.retryableNotificationCount || 0),
        ].join(' / ');
        const reasons = (item.reasons || []).length ? item.reasons.join('；') : '-';
        return '<tr>' +
          '<td>' + pill(priority[0], priority[1]) + '<br><span class="subtitle">健康分 ' + esc(item.healthScore || 0) + '</span></td>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + ' / ' + esc((item.tenant || {}).packageName || (item.tenant || {}).packageCode || '未开套餐') + '</span></td>' +
          '<td>' + esc(item.owner || '未分配') + '<br><span class="subtitle">' + pill(due[0], due[1]) + '</span></td>' +
          '<td>' + esc(reasons) + '<br><span class="subtitle">' + esc(signals) + '</span></td>' +
          '<td>' + esc(item.nextAction || '-') + '</td>' +
          '<td><button type="button" data-action="customer-success-tenant" data-tenant-id="' + esc(item.tenantId) + '">查看租户</button></td>' +
        '</tr>';
	      }).join('');
	    }
	    function renderCustomerSuccessOwners(data) {
	      const owners = data.owners || [];
	      const summary = data.summary || {};
	      const ownerCount = data.ownerCount === undefined ? owners.length : Number(data.ownerCount || 0);
	      const returned = data.returnedCount === undefined ? owners.length : Number(data.returnedCount || 0);
	      let text = ownerCount ? (ownerCount + ' 个负责人 / 显示 ' + returned + ' 个 / 队列 ' + fmt(summary.queueCount || 0)) : '';
	      if (Number(summary.unassignedCount || 0) > 0) text += '，未分配 ' + summary.unassignedCount;
	      customerSuccessOwnerCountEl.textContent = text;
	      if (!owners.length) {
	        customerSuccessOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">暂无负责人汇总</td></tr>';
	        return;
	      }
	      customerSuccessOwnersEl.innerHTML = owners.map(item => {
	        const priorityParts = [
	          '紧急 ' + fmt(item.criticalCount || 0),
	          '高 ' + fmt(item.highCount || 0),
	          '中 ' + fmt(item.mediumCount || 0),
	          '正常 ' + fmt(item.normalCount || 0),
	        ].join(' / ');
	        const signals = [
	          '账单 ' + fmt(item.billingFollowUpCount || 0),
	          '任务 ' + fmt(item.actionableTaskCount || 0),
	          '通知 ' + fmt(item.retryableNotificationCount || 0),
	          '阻断 ' + fmt(item.blockedCount || 0),
	        ].join(' / ');
	        const health = '最高 ' + fmt(item.maxHealthScore || 0) + ' / 均值 ' + fmt(item.averageHealthScore || 0);
	        const topTenants = (item.topTenants || []).map(tenant => (tenant.tenantName || '-') + '#' + (tenant.tenantId || '-')).join('，') || '-';
	        const overdue = Number(item.overdueCount || 0);
	        const dueSoon = Number(item.dueSoonCount || 0);
	        const tenantPill = overdue > 0 ? pill(item.tenantCount + ' 个租户', 'danger') : (dueSoon > 0 ? pill(item.tenantCount + ' 个租户', 'warning') : pill(item.tenantCount + ' 个租户', item.tenantCount > 0 ? '' : 'ok'));
	        return '<tr>' +
	          '<td>' + esc(item.owner || '未分配') + '</td>' +
	          '<td>' + tenantPill + '<br><span class="subtitle">' + esc(health) + '</span></td>' +
	          '<td>' + esc(priorityParts) + '<br><span class="subtitle">逾期 ' + esc(overdue) + ' / 7 天 ' + esc(dueSoon) + '</span></td>' +
	          '<td>' + esc(signals) + '<br><span class="subtitle">失败 ' + esc(item.failedNotificationCount || 0) + ' / 耗尽 ' + esc(item.deadNotificationCount || 0) + '</span></td>' +
	          '<td>' + esc(item.nextFollowUpAt || '-') + '<br><span class="subtitle">' + esc(topTenants) + '</span></td>' +
	        '</tr>';
	      }).join('');
	    }
	    function renderRiskFollowUps(items, summary) {
	      const total = summary && summary.totalCount !== undefined ? Number(summary.totalCount) : items.length;
      const overdue = summary && summary.overdueCount !== undefined ? Number(summary.overdueCount) : items.filter(item => item.overdue).length;
      const dueSoon = summary && summary.dueSoonCount !== undefined ? Number(summary.dueSoonCount) : 0;
      let text = total + ' 条任务';
      if (overdue > 0) text += '，逾期 ' + overdue;
      if (dueSoon > 0) text += '，7 天内 ' + dueSoon;
      document.getElementById('riskTaskCount').textContent = text;
      if (!items.length) {
        riskFollowUpsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      riskFollowUpsEl.innerHTML = items.map(item => {
        const status = riskFollowStatusText(item.status);
        const due = riskTaskDueStateText(item.dueState);
        const daysText = item.nextFollowUpAt ? (item.daysUntil === 0 ? '今天' : (item.daysUntil > 0 ? item.daysUntil + ' 天后' : Math.abs(item.daysUntil) + ' 天前')) : '-';
        return '<tr>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + '</span></td>' +
          '<td>' + pill(status[0], status[1]) + '<br><span class="subtitle">' + pill(due[0], due[1]) + '</span></td>' +
          '<td>' + esc(item.owner || '-') + '</td>' +
          '<td>' + esc(item.nextFollowUpAt || '-') + '<br><span class="subtitle">' + esc(daysText) + '</span></td>' +
          '<td>' + esc(item.remark || '-') + '</td>' +
          '<td>' + esc(item.createdAt || '-') + '<br><span class="subtitle">操作 ' + esc(item.operationId || '-') + '</span></td>' +
        '</tr>';
      }).join('');
    }
    function renderRiskFollowUpOwners(items, summary) {
      const total = summary && summary.totalCount !== undefined ? Number(summary.totalCount) : 0;
      const ownerCount = items.length;
      document.getElementById('riskOwnerCount').textContent = ownerCount ? (ownerCount + ' 个负责人，' + total + ' 条任务') : '';
      if (!items.length) {
        riskFollowUpOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">暂无数据</td></tr>';
        return;
      }
      riskFollowUpOwnersEl.innerHTML = items.map(item => {
        const overdue = Number(item.overdueCount || 0);
        const dueSoon = Number(item.dueSoonCount || 0);
        const open = Number(item.openCount || 0);
        const openPill = overdue > 0 ? pill(open + ' 打开', 'danger') : (dueSoon > 0 ? pill(open + ' 打开', 'warning') : pill(open + ' 打开', open > 0 ? '' : 'ok'));
        const statusParts = [
          '待 ' + fmt(item.pendingCount || 0),
          '联 ' + fmt(item.contactedCount || 0),
          '续 ' + fmt(item.renewalPendingCount || 0),
          '关 ' + fmt((item.resolvedCount || 0) + (item.ignoredCount || 0)),
        ];
        const dueParts = [
          '逾期 ' + fmt(overdue),
          '7 天 ' + fmt(dueSoon),
          '未来 ' + fmt(item.futureCount || 0),
          '无日期 ' + fmt(item.noDateCount || 0),
          '已关 ' + fmt(item.closedCount || 0),
        ];
        return '<tr>' +
          '<td>' + esc(item.owner || '未分配') + '</td>' +
          '<td>' + openPill + '<br><span class="subtitle">全部 ' + esc(item.totalCount || 0) + '</span></td>' +
          '<td>' + esc(statusParts.join(' / ')) + '</td>' +
          '<td>' + esc(dueParts.join(' / ')) + '</td>' +
          '<td>' + esc(item.latestFollowUpAt || '-') + '<br><span class="subtitle">下次 ' + esc(item.nextFollowUpAt || '-') + '</span></td>' +
        '</tr>';
      }).join('');
    }
    function renderRisk(items, summary) {
      const total = summary && summary.evaluatedTenantCount !== undefined ? summary.evaluatedTenantCount : items.length;
      const risky = summary && summary.riskTenantCount !== undefined ? summary.riskTenantCount : items.filter(item => item.riskLevel !== 'normal').length;
      const followCount = summary && summary.followUpTenantCount !== undefined ? Number(summary.followUpTenantCount) : 0;
      const overdueCount = summary && summary.overdueFollowUpCount !== undefined ? Number(summary.overdueFollowUpCount) : 0;
      const renewalCount = summary && summary.renewalPendingCount !== undefined ? Number(summary.renewalPendingCount) : 0;
      let riskText = risky + '/' + total + ' 个租户有风险';
      if (followCount > 0) riskText += '，已跟进 ' + followCount;
      if (overdueCount > 0) riskText += '，逾期 ' + overdueCount;
      if (renewalCount > 0) riskText += '，续费中 ' + renewalCount;
      document.getElementById('riskCount').textContent = riskText;
      if (!items.length) {
        riskTenantsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无数据</td></tr>';
        return;
      }
      riskTenantsEl.innerHTML = items.map(item => {
        const level = riskLevelText(item.riskLevel);
        const topMetric = (item.topUsageMetrics || [])[0] || {};
        const usage = topMetric.metric ? esc(topMetric.label || topMetric.metric) + ' ' + esc(topMetric.current) + '/' + (topMetric.unlimited ? '不限' : esc(topMetric.limit)) + ' ' + esc(pct(topMetric.usageRatio)) : '-';
        const reasons = (item.riskReasons || []).length ? item.riskReasons.join('；') : '无';
        const follow = item.followUp || null;
        const followStatus = riskFollowStatusText(follow ? follow.status : '');
        const followText = follow
          ? pill(followStatus[0], followStatus[1]) + '<br><span class="subtitle">' + esc(follow.owner || '-') + ' / ' + esc(follow.nextFollowUpAt || '-') + '</span><br><span class="subtitle">' + esc(follow.remark || '-') + '</span>'
          : '<span class="subtitle">未跟进</span>';
        return '<tr>' +
          '<td>' + pill(level[0], level[1]) + '<br><span class="subtitle">分 ' + esc(item.riskScore || 0) + '</span></td>' +
          '<td>' + esc(item.tenantName) + '<br><span class="subtitle">ID ' + esc(item.tenantId) + ' / ' + esc(item.packageName || item.packageCode || '未开套餐') + '</span></td>' +
          '<td>' + esc(reasons) + '</td>' +
          '<td>' + usage + '<br><span class="subtitle">打开告警 ' + esc(item.openAlertCount || 0) + '</span></td>' +
          '<td>' + esc(item.suggestedAction || '-') + '</td>' +
          '<td>' + followText + '</td>' +
          '<td><button type="button" data-action="risk-follow-up" data-tenant-id="' + esc(item.tenantId) + '">记录跟进</button></td>' +
        '</tr>';
      }).join('');
    }
    function renderTenants(items) {
	      document.getElementById('tenantCount').textContent = items.length + ' 条';
      tenantOverviewCache = {};
      if (!items.length) {
        tenantsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      items.forEach(item => { tenantOverviewCache[String(item.tenantId || '')] = item; });
      tenantsEl.innerHTML = items.map(item => {
        const due = item.expired ? pill('已到期', 'danger') : (item.expiringSoon ? pill('即将到期', 'warning') : pill('正常', 'ok'));
        const usage = item.maxUsageLimit > 0 ? esc(item.maxUsageLabel) + ' ' + esc(item.maxUsageCurrent) + '/' + esc(item.maxUsageLimit) + ' ' + esc(pct(item.maxUsageRatio)) : '-';
        return '<tr>' +
          '<td>' + esc(item.tenantName) + '<br><span class="subtitle">ID ' + esc(item.tenantId) + '</span></td>' +
          '<td>' + esc(item.packageName || item.packageCode || '未分配') + '<br><span class="subtitle">版本 ' + esc(item.packageVersion || 0) + '</span></td>' +
          '<td>' + due + '<br><span class="subtitle">' + esc(item.expiresAt) + '</span></td>' +
          '<td>' + (item.openAlertCount > 0 ? pill(item.openAlertCount, 'warning') : pill('0', 'ok')) + '</td>' +
          '<td>' + usage + '</td>' +
          '<td><button type="button" data-action="adjust-tenant-package" data-tenant-id="' + esc(item.tenantId) + '">调整套餐</button></td>' +
        '</tr>';
      }).join('');
    }
    function subscriptionStatusView(status) {
      const values = {
        trialing: ['试用', ''], active: ['有效', 'ok'], grace: ['宽限期', 'warning'],
        past_due: ['欠费', 'danger'], suspended: ['暂停', 'danger'], canceled: ['取消', 'danger'],
      };
      return values[status] || [status || '未知', 'danger'];
    }
    function subscriptionDateTimeInput(value) {
      return value ? String(value).replace(' ', 'T').slice(0, 16) : '';
    }
    function subscriptionDateTimePayload(value) {
      value = String(value || '').trim();
      if (!value) return '';
      value = value.replace('T', ' ');
      return value.length === 16 ? value + ':00' : value;
    }
    function renderSubscriptionSummary(summary) {
      summary = summary || {};
      subscriptionSummaryEl.innerHTML = [
        ['有效订阅', fmt(summary.activeCount || 0)],
        ['试用 / 宽限', fmt(summary.trialingCount || 0) + ' / ' + fmt(summary.graceCount || 0)],
        ['欠费', fmt(summary.pastDueCount || 0)],
        ['暂停 / 取消', fmt(summary.suspendedCount || 0) + ' / ' + fmt(summary.canceledCount || 0)],
        ['允许 / 阻断', fmt(summary.accessAllowedCount || 0) + ' / ' + fmt(summary.accessBlockedCount || 0)],
        ['待对账', fmt(summary.reconciliationDueCount || 0)],
      ].map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
      subscriptionCountEl.textContent = '订阅 ' + fmt(summary.subscriptionCount || 0) + ' / 租户 ' + fmt(summary.tenantCount || 0) + ' / 套餐 ' + fmt(summary.packageCount || 0);
    }
    function renderSubscriptions(items) {
      if (!items.length) {
        subscriptionsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无订阅数据</td></tr>';
        return;
      }
      subscriptionsEl.innerHTML = items.map(item => {
        const effective = subscriptionStatusView(item.effectiveStatus);
        const stored = item.status !== item.effectiveStatus ? '<br><span class="subtitle">存储 ' + esc(item.status) + '，待对账</span>' : '';
        const access = item.accessAllowed ? pill('允许', 'ok') : pill('阻断', 'danger');
        const period = item.effectiveStatus === 'trialing'
          ? '试用至 ' + esc(item.trialEndsAt || '-')
          : '周期至 ' + esc(item.currentPeriodEndsAt || '长期');
        const grace = item.graceEndsAt ? '<br><span class="subtitle">宽限至 ' + esc(item.graceEndsAt) + '</span>' : '';
        const cancel = item.cancelAtPeriodEnd ? '<br>' + pill('期末取消', 'warning') : '';
        return '<tr>' +
          '<td>' + pill(effective[0], effective[1]) + stored + '</td>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + '</span></td>' +
          '<td>' + esc(item.packageName || item.packageCode || '-') + '<br><span class="subtitle">' + esc(item.billingCycle || '-') + '</span></td>' +
          '<td>' + period + grace + cancel + '</td>' +
          '<td>' + access + '<br><span class="subtitle">' + esc(item.accessReason || '-') + '</span></td>' +
          '<td>v' + esc(item.version || 0) + '<br><span class="subtitle">' + esc(item.stateReason || '-') + '</span></td>' +
          '<td><button type="button" data-action="select-subscription"' +
            ' data-tenant-id="' + esc(item.tenantId) + '" data-status="' + esc(item.effectiveStatus) + '" data-version="' + esc(item.version || 0) + '"' +
            ' data-trial-ends-at="' + esc(item.trialEndsAt || '') + '" data-period-ends-at="' + esc(item.currentPeriodEndsAt || '') + '"' +
            ' data-grace-ends-at="' + esc(item.graceEndsAt || '') + '" data-cancel-at-period-end="' + (item.cancelAtPeriodEnd ? 'true' : 'false') + '">管理</button></td>' +
        '</tr>';
      }).join('');
    }
    function renderSubscriptionEvents(items) {
      subscriptionEventCountEl.textContent = items.length + ' 条';
      if (!items.length) {
        subscriptionEventsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无订阅事件</td></tr>';
        return;
      }
      subscriptionEventsEl.innerHTML = items.map(item => {
        const from = item.fromStatus ? subscriptionStatusView(item.fromStatus)[0] : '新建';
        const to = subscriptionStatusView(item.toStatus)[0];
        return '<tr>' +
          '<td>' + esc(item.effectiveAt || item.createdAt || '-') + '</td>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + '</span></td>' +
          '<td>' + esc(item.eventType || '-') + '<br><span class="subtitle">#' + esc(item.id) + '</span></td>' +
          '<td>' + esc(from) + ' → ' + esc(to) + '</td>' +
          '<td>' + esc(item.source || '-') + '<br><span class="subtitle">用户 ' + esc(item.actorUserId || 0) + '</span></td>' +
          '<td>' + esc(item.reason || '-') + '</td>' +
        '</tr>';
      }).join('');
    }
    async function loadSubscriptionEvents(tenantId) {
      tenantId = Number(tenantId || subscriptionTenantInput.value.trim());
      const params = new URLSearchParams({ limit: '50' });
      if (tenantId) params.set('tenantId', String(tenantId));
      try {
        const res = await fetch('/dashboard/saasAdmin/subscriptionEvents?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderSubscriptionEvents((body.data || {}).events || []);
      } catch (err) {
        subscriptionEventsEl.innerHTML = '<tr><td colspan="6" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        subscriptionEventCountEl.textContent = '';
      }
    }
    async function loadSubscriptions() {
      try {
        const res = await fetch('/dashboard/saasAdmin/subscriptions?' + subscriptionParams(100).toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderSubscriptionSummary(data.summary || {});
        renderSubscriptions(data.subscriptions || []);
        loadSubscriptionEvents();
      } catch (err) {
        subscriptionsEl.innerHTML = '<tr><td colspan="7" class="empty">仅平台管理员可查看订阅</td></tr>';
        subscriptionCountEl.textContent = err.message || String(err);
      }
    }
    function selectSubscription(button) {
      subscriptionTenantInput.value = button.dataset.tenantId || '';
      subscriptionTransitionStatusInput.value = button.dataset.status || 'active';
      subscriptionVersionInput.value = button.dataset.version || '';
      subscriptionTrialEndsInput.value = subscriptionDateTimeInput(button.dataset.trialEndsAt || '');
      subscriptionPeriodEndsInput.value = subscriptionDateTimeInput(button.dataset.periodEndsAt || '');
      subscriptionGraceEndsInput.value = subscriptionDateTimeInput(button.dataset.graceEndsAt || '');
      subscriptionCancelAtPeriodEndInput.checked = button.dataset.cancelAtPeriodEnd === 'true';
      loadSubscriptionEvents(button.dataset.tenantId);
      document.querySelector('.subscriptiontransition').scrollIntoView({ block: 'center', behavior: 'smooth' });
    }
    async function transitionSubscription() {
      const tenantId = Number(subscriptionTenantInput.value.trim());
      const expectedVersion = Number(subscriptionVersionInput.value.trim());
      if (!tenantId || !expectedVersion) {
        statusEl.textContent = '请选择订阅并保留当前版本';
        return;
      }
      const payload = {
        tenantId,
        status: subscriptionTransitionStatusInput.value,
        expectedVersion,
        cancelAtPeriodEnd: subscriptionCancelAtPeriodEndInput.checked,
        reason: subscriptionReasonInput.value.trim(),
        idempotencyKey: 'page:' + tenantId + ':' + expectedVersion + ':' + subscriptionTransitionStatusInput.value,
      };
      const trialEndsAt = subscriptionDateTimePayload(subscriptionTrialEndsInput.value);
      const currentPeriodEndsAt = subscriptionDateTimePayload(subscriptionPeriodEndsInput.value);
      const graceEndsAt = subscriptionDateTimePayload(subscriptionGraceEndsInput.value);
      if (trialEndsAt) payload.trialEndsAt = trialEndsAt;
      if (currentPeriodEndsAt) payload.currentPeriodEndsAt = currentPeriodEndsAt;
      if (graceEndsAt) payload.graceEndsAt = graceEndsAt;
	      const requiresApproval = approvalActionRequired('tenant.subscription.transition', 0);
	      statusEl.textContent = requiresApproval ? '正在提交订阅迁移审批' : '正在迁移订阅状态';
	      try {
	        if (requiresApproval) {
	          await requestHighRiskApproval(
	            'tenant.subscription.transition',
	            payload,
	            '迁移租户 ' + tenantId + ' 的订阅状态为 ' + payload.status
	          );
	          statusEl.textContent = '订阅状态迁移已提交双人审批';
	          return;
	        }
	        const res = await fetch('/dashboard/saasAdmin/subscriptionTransition', {
          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const result = body.data || {};
        const item = result.subscription || {};
        subscriptionVersionInput.value = String(item.version || expectedVersion);
        statusEl.textContent = result.idempotent ? '该订阅操作已处理' : '订阅状态已更新为 ' + fmt(item.effectiveStatus || item.status);
        await loadSubscriptions();
        loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function reconcileSubscriptions(dryRun) {
      const tenantId = Number(subscriptionTenantInput.value.trim());
      if (!dryRun && !window.confirm(tenantId ? '确认执行该租户订阅对账？' : '确认执行全平台订阅对账？')) return;
      statusEl.textContent = dryRun ? '正在预览订阅对账' : '正在执行订阅对账';
      const payload = { limit: 500, dryRun: Boolean(dryRun) };
      if (tenantId) payload.tenantId = tenantId;
      try {
        const res = await fetch('/dashboard/saasAdmin/subscriptionReconcile', {
          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        statusEl.textContent = (dryRun ? '对账预览' : '订阅对账完成') + '：扫描 ' + fmt(data.scannedCount || 0) + '，待处理 ' + fmt(data.reconciliationDue || 0) + '，已变更 ' + fmt(data.changedCount || 0) + '，失败 ' + fmt(data.failedCount || 0);
        if (!dryRun) {
          await loadSubscriptions();
          loadOperations();
        }
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    function paymentOrderStatusView(status) {
      const values = {
        pending: ['待支付', 'warning'], processing: ['处理中', ''], paid: ['已支付', 'ok'],
        failed: ['失败', 'danger'], canceled: ['已取消', 'danger'],
      };
      return values[status] || [status || '未知', 'danger'];
    }
    function paymentWebhookStatusView(status) {
      const values = {
        received: ['已接收', 'warning'], processed: ['已处理', 'ok'], ignored: ['已忽略', ''], failed: ['失败', 'danger'],
      };
      return values[status] || [status || '未知', 'danger'];
    }
	function paymentRefundStatusView(status) {
	  const values = {
	    requested: ['待处理', 'warning'], processing: ['处理中', 'warning'], succeeded: ['已退款', 'ok'],
	    failed: ['失败', 'danger'], canceled: ['已取消', ''],
	  };
	  return values[status] || [status || '未知', 'danger'];
	}
    function renderPaymentSummary(summary) {
      summary = summary || {};
      paymentSummaryEl.innerHTML = [
        ['订单总数', fmt(summary.orderCount || 0)],
        ['待收 / 处理中', fmt(summary.pendingCount || 0) + ' / ' + fmt(summary.processingCount || 0)],
		['毛收入', moneyCents(summary.paidAmountCents) + ' CNY'],
		['退款中', moneyCents(summary.refundPendingCents) + ' CNY'],
		['已退款', moneyCents(summary.refundedAmountCents) + ' CNY'],
		['净收入', moneyCents(summary.netPaidAmountCents) + ' CNY'],
        ['失败 / 待催缴', fmt(summary.failedCount || 0) + ' / ' + fmt(summary.dunningDueCount || 0)],
        ['应收敞口', moneyCents(summary.outstandingCents) + ' CNY'],
        ['租户 / 渠道', fmt(summary.tenantCount || 0) + ' / ' + fmt(summary.providerCount || 0)],
      ].map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
      paymentOrderCountEl.textContent = '命中 ' + fmt(summary.orderCount || 0) + '，可收款 ' + fmt(summary.collectibleCount || 0) + '，已支付 ' + fmt(summary.paidCount || 0);
    }
    function renderPaymentOrders(items) {
      if (!items.length) {
        paymentOrdersEl.innerHTML = '<tr><td colspan="7" class="empty">暂无支付订单</td></tr>';
        return;
      }
      paymentOrdersEl.innerHTML = items.map(item => {
        const state = paymentOrderStatusView(item.status);
        const failure = item.failureMessage ? '<br><span class="subtitle">' + esc(item.failureCode || 'failed') + '：' + esc(item.failureMessage) + '</span>' : '';
        const providerOrder = item.providerOrderNo ? '<br><span class="subtitle">渠道单 ' + esc(item.providerOrderNo) + '</span>' : '';
        const checkout = item.checkoutUrl
          ? '<a href="' + esc(item.checkoutUrl) + '" target="_blank" rel="noopener noreferrer">打开收银台</a><br><span class="subtitle">至 ' + esc(item.checkoutExpiresAt || '-') + '</span>'
          : '<span class="subtitle">未配置</span>';
        const dunning = esc(item.dunningAttempts || 0) + '/' + esc(item.maxDunningAttempts || 0) +
          '<br><span class="subtitle">下次 ' + esc(item.nextDunningAt || '-') + '</span>' +
          (item.dunningDue ? '<br>' + pill('待催缴', 'warning') : '');
		const actions = [];
		if (item.status === 'pending' || item.status === 'processing' || item.status === 'failed') {
		  actions.push('<button type="button" data-action="cancel-payment-order" data-order-no="' + esc(item.orderNo) + '" data-version="' + esc(item.version || 0) + '">取消</button>');
		}
			if (item.status === 'paid' && Number(item.refundableAmountCents || 0) > 0) {
			  actions.push('<button type="button" data-action="select-payment-refund" data-order-no="' + esc(item.orderNo) + '" data-amount-cents="' + esc(item.refundableAmountCents || 0) + '" data-currency="' + esc(item.currency || 'CNY') + '">退款</button>');
			}
			if (item.status === 'paid' && Number(item.invoiceAvailableCents || 0) > 0) {
			  actions.push('<button type="button" data-action="select-payment-invoice" data-tenant-id="' + esc(item.tenantId) + '" data-order-no="' + esc(item.orderNo) + '" data-amount-cents="' + esc(item.invoiceAvailableCents || 0) + '" data-currency="' + esc(item.currency || 'CNY') + '">开票</button>');
			}
			const actionHTML = actions.length ? actions.join(' ') : '-';
			const refund = '<br><span class="subtitle">已退 ' + esc(moneyCents(item.refundedAmountCents)) + ' / 可退 ' + esc(moneyCents(item.refundableAmountCents)) + '</span>';
			const invoice = '<br><span class="subtitle">净开票 ' + esc(moneyCents(item.netInvoicedCents)) + ' / 可开 ' + esc(moneyCents(item.invoiceAvailableCents)) + ' / 待红冲 ' + esc(moneyCents(item.creditNoteDueCents)) + '</span>';
        return '<tr>' +
          '<td>' + pill(state[0], state[1]) + failure + '</td>' +
          '<td>' + esc(item.orderNo) + '<br><span class="subtitle">' + esc(item.tenantName || '-') + ' / ID ' + esc(item.tenantId) + '</span></td>' +
          '<td>' + esc(item.packageName || item.packageCode || '-') + '<br><span class="subtitle">' + esc(item.billingCycle || '-') + ' / 至 ' + esc(item.serviceExpiresAt || '长期') + '</span></td>' +
			  '<td>' + esc(moneyCents(item.amountCents)) + ' ' + esc(item.currency || 'CNY') + refund + invoice + '<br><span class="subtitle">' + esc(item.provider || '-') + providerOrder + '</span></td>' +
          '<td>' + checkout + '</td>' +
          '<td>' + dunning + '</td>' +
		  '<td>v' + esc(item.version || 0) + '<br>' + actionHTML + '</td>' +
        '</tr>';
      }).join('');
    }
	function renderPaymentRefundSummary(summary) {
	  summary = summary || {};
	  paymentRefundSummaryEl.innerHTML = [
	    ['退款单', fmt(summary.refundCount || 0)],
	    ['待处理 / 处理中', fmt(summary.requestedCount || 0) + ' / ' + fmt(summary.processingCount || 0)],
	    ['待退金额', moneyCents(summary.pendingAmountCents) + ' CNY'],
	    ['已退金额', moneyCents(summary.succeededAmountCents) + ' CNY'],
	    ['失败 / 取消', fmt(summary.failedCount || 0) + ' / ' + fmt(summary.canceledCount || 0)],
	    ['租户 / 订单', fmt(summary.tenantCount || 0) + ' / ' + fmt(summary.orderCount || 0)],
	  ].map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
	  paymentRefundCountEl.textContent = '命中 ' + fmt(summary.refundCount || 0) + '，已退 ' + fmt(summary.succeededCount || 0) + '，净退款 ' + moneyCents(summary.succeededAmountCents) + ' CNY';
	}
	function renderPaymentRefunds(items) {
	  if (!items.length) {
	    paymentRefundsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无退款</td></tr>';
	    return;
	  }
	  paymentRefundsEl.innerHTML = items.map(item => {
	    const state = paymentRefundStatusView(item.status);
	    const failure = item.failureMessage ? '<br><span class="subtitle">' + esc(item.failureCode || 'failed') + '：' + esc(item.failureMessage) + '</span>' : '';
	    const providerRefund = item.providerRefundNo ? '<br><span class="subtitle">渠道单 ' + esc(item.providerRefundNo) + '</span>' : '';
	    const entitlement = item.entitlementAction === 'cancel' ? pill('取消订阅', 'danger') : (item.entitlementAction === 'suspend' ? pill('暂停订阅', 'warning') : pill('保留订阅', 'ok'));
	    const cancel = item.status === 'requested'
	      ? '<button type="button" data-action="cancel-payment-refund" data-refund-no="' + esc(item.refundNo) + '" data-version="' + esc(item.version || 0) + '">取消</button>'
	      : '-';
	    return '<tr>' +
	      '<td>' + pill(state[0], state[1]) + failure + '</td>' +
	      '<td>' + esc(item.refundNo) + '<br><span class="subtitle">支付单 ' + esc(item.orderNo || '-') + '</span></td>' +
	      '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + '</span></td>' +
	      '<td>' + esc(moneyCents(item.amountCents)) + ' ' + esc(item.currency || 'CNY') + providerRefund + '<br><span class="subtitle">原因 ' + esc(item.reason || '-') + '</span></td>' +
	      '<td>' + entitlement + '<br><span class="subtitle">原单已退 ' + esc(moneyCents(item.orderRefundedCents)) + ' / 可退 ' + esc(moneyCents(item.orderRefundableCents)) + '</span></td>' +
	      '<td>' + esc(item.succeededAt || item.failedAt || item.canceledAt || item.requestedAt || '-') + '<br><span class="subtitle">账单 #' + esc(item.billingEventId || '-') + '</span></td>' +
	      '<td>v' + esc(item.version || 0) + '<br>' + cancel + '</td>' +
	    '</tr>';
	  }).join('');
	}
	async function loadPaymentRefunds() {
	  try {
	    const res = await fetch('/dashboard/saasAdmin/paymentRefunds?' + paymentRefundParams(100).toString(), { headers: authHeader() });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    renderPaymentRefundSummary(data.summary || {});
	    renderPaymentRefunds(data.refunds || []);
	  } catch (err) {
	    paymentRefundsEl.innerHTML = '<tr><td colspan="7" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
	    paymentRefundCountEl.textContent = '';
	  }
	}
	function selectPaymentRefund(button) {
	  paymentRefundOrderInput.value = button.dataset.orderNo || '';
	  paymentRefundOrderFilterInput.value = button.dataset.orderNo || '';
	  paymentRefundAmountInput.value = (Number(button.dataset.amountCents || 0) / 100).toFixed(2);
	  paymentRefundCurrencyInput.value = button.dataset.currency || 'CNY';
	  paymentRefundReasonInput.focus();
	}
	async function createPaymentRefund() {
	  const orderNo = paymentRefundOrderInput.value.trim();
	  const amount = paymentRefundAmountInput.value.trim();
	  const reason = paymentRefundReasonInput.value.trim();
	  const entitlementAction = paymentRefundEntitlementInput.value || 'keep';
	  if (!orderNo || !amount || !reason) {
	    statusEl.textContent = '请填写支付订单、退款金额和退款原因';
	    return;
	  }
	  if (entitlementAction !== 'keep' && !window.confirm('确认在全额退款成功后' + (entitlementAction === 'cancel' ? '取消' : '暂停') + '该租户订阅？')) return;
	  const idempotencyKey = paymentRefundIdempotencyInput.value.trim() || ('page-refund:' + orderNo + ':' + Date.now());
	  paymentRefundIdempotencyInput.value = idempotencyKey;
	  const payload = {
	    orderNo, amount, reason, entitlementAction, idempotencyKey,
	    currency: paymentRefundCurrencyInput.value.trim() || 'CNY',
	    providerRefundNo: paymentProviderRefundNoInput.value.trim(),
	    remark: paymentRefundRemarkInput.value.trim(),
	  };
	  if (paymentRefundNoInput.value.trim()) payload.refundNo = paymentRefundNoInput.value.trim();
	  statusEl.textContent = '正在创建退款申请';
	  try {
	    if (approvalActionRequired('payment.refund.create', Math.round(Number(amount || 0) * 100))) {
	      await requestHighRiskApproval('payment.refund.create', payload, reason);
	      statusEl.textContent = '退款申请已提交审批';
	      return;
	    }
	    const res = await fetch('/dashboard/saasAdmin/paymentRefund', {
	      method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    const refund = data.refund || {};
	    paymentRefundNoInput.value = refund.refundNo || paymentRefundNoInput.value;
	    statusEl.textContent = data.idempotent ? '该退款申请已存在' : '退款申请已创建：' + fmt(refund.refundNo);
	    await loadPaymentOrders();
	    loadOperations();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
		async function cancelPaymentRefund(button) {
	  const refundNo = button.dataset.refundNo || '';
	  const expectedVersion = Number(button.dataset.version || 0);
	  const reason = window.prompt('请输入取消退款原因', '退款申请撤回');
	  if (reason === null || !reason.trim()) return;
	  statusEl.textContent = '正在取消退款申请';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/paymentRefundCancel', {
	      method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify({ refundNo, expectedVersion, reason: reason.trim() }),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    statusEl.textContent = '退款申请已取消：' + refundNo;
	    await loadPaymentOrders();
	    loadOperations();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
		  }
		}
		function invoiceStatusView(status) {
		  const values = {
		    requested: ['待处理', 'warning'], processing: ['处理中', 'warning'], issued: ['已开具', 'ok'],
		    failed: ['失败', 'danger'], canceled: ['已取消', ''],
		  };
		  return values[status] || [status || '未知', 'danger'];
		}
		function invoiceKindView(kind) {
		  return kind === 'credit_note' ? ['红票', 'danger'] : ['蓝票', ''];
		}
		function renderInvoiceDocumentSummary(summary) {
		  summary = summary || {};
		  invoiceDocumentSummaryEl.innerHTML = [
		    ['单据', fmt(summary.documentCount || 0)],
		    ['待处理 / 处理中', fmt(summary.requestedCount || 0) + ' / ' + fmt(summary.processingCount || 0)],
		    ['已开具 / 失败', fmt(summary.issuedCount || 0) + ' / ' + fmt(summary.failedCount || 0)],
		    ['蓝票 / 红票', moneyCents(summary.issuedInvoiceAmountCents) + ' / ' + moneyCents(summary.issuedCreditAmountCents)],
		    ['净开票额', moneyCents(summary.netIssuedAmountCents) + ' CNY'],
		    ['租户 / 订单', fmt(summary.tenantCount || 0) + ' / ' + fmt(summary.orderCount || 0)],
		  ].map(item => '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>').join('');
		  invoiceDocumentCountEl.textContent = '命中 ' + fmt(summary.documentCount || 0) + '，待处理 ' + fmt(summary.requestedCount || 0) + '，净开票 ' + moneyCents(summary.netIssuedAmountCents) + ' CNY';
		}
		function renderInvoiceDocuments(items) {
		  if (!items.length) {
		    invoiceDocumentsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无发票单据</td></tr>';
		    return;
		  }
		  invoiceDocumentsEl.innerHTML = items.map(item => {
		    const state = invoiceStatusView(item.status);
		    const kind = invoiceKindView(item.kind);
		    const original = item.originalDocumentNo ? '<br><span class="subtitle">原票 ' + esc(item.originalDocumentNo) + '</span>' : '';
		    const provider = item.providerDocumentNo ? esc(item.providerDocumentNo) : '-';
		    const documentLink = item.documentUrl ? '<br><a href="' + esc(item.documentUrl) + '" target="_blank" rel="noopener noreferrer">查看电子票</a>' : '';
		    const failure = item.failureMessage ? '<br><span class="bad">' + esc(item.failureCode || 'failed') + '：' + esc(item.failureMessage) + '</span>' : '';
		    const actions = [
		      '<button type="button" data-action="select-invoice-document" data-document-no="' + esc(item.documentNo) + '" data-version="' + esc(item.version || 0) + '" data-provider="' + esc(item.provider || 'manual') + '">选择</button>',
		      '<button type="button" data-action="load-invoice-profile" data-tenant-id="' + esc(item.tenantId) + '">资料</button>',
		    ];
		    if (item.status === 'requested') actions.push('<button type="button" data-action="transition-invoice-document" data-status="processing" data-document-no="' + esc(item.documentNo) + '" data-version="' + esc(item.version || 0) + '" data-provider="' + esc(item.provider || 'manual') + '">处理</button>');
			    if (item.status === 'requested' || item.status === 'processing') {
			      actions.push('<button type="button" data-action="transition-invoice-document" data-status="issued" data-document-no="' + esc(item.documentNo) + '" data-version="' + esc(item.version || 0) + '" data-provider="' + esc(item.provider || 'manual') + '">' + (approvalActionRequired('billing.invoice.issue', 0) ? '提交开具审批' : '开具') + '</button>');
		      actions.push('<button type="button" data-action="transition-invoice-document" data-status="failed" data-document-no="' + esc(item.documentNo) + '" data-version="' + esc(item.version || 0) + '" data-provider="' + esc(item.provider || 'manual') + '">失败</button>');
		      actions.push('<button type="button" data-action="transition-invoice-document" data-status="canceled" data-document-no="' + esc(item.documentNo) + '" data-version="' + esc(item.version || 0) + '" data-provider="' + esc(item.provider || 'manual') + '">取消</button>');
		    }
		    if (item.kind === 'invoice' && item.status === 'issued' && Number(item.orderCreditNoteDueCents || 0) > 0) {
		      actions.push('<button type="button" data-action="create-credit-note" data-document-no="' + esc(item.documentNo) + '" data-tenant-id="' + esc(item.tenantId) + '" data-order-no="' + esc(item.orderNo) + '" data-amount-cents="' + esc(Math.min(Number(item.amountCents || 0), Number(item.orderCreditNoteDueCents || 0))) + '" data-currency="' + esc(item.currency || 'CNY') + '">红冲</button>');
		    }
		    return '<tr>' +
		      '<td>' + pill(kind[0], kind[1]) + '<br>' + pill(state[0], state[1]) + '</td>' +
		      '<td>' + esc(item.documentNo) + original + '</td>' +
		      '<td>' + esc(item.tenantName || '-') + ' / ID ' + esc(item.tenantId) + '<br><span class="subtitle">' + esc(item.orderNo || '-') + '</span></td>' +
		      '<td>' + esc(moneyCents(item.amountCents)) + ' ' + esc(item.currency || 'CNY') + '<br><span class="subtitle">净收 ' + esc(moneyCents(item.orderNetPaidCents)) + ' / 净开 ' + esc(moneyCents(item.orderNetInvoicedCents)) + '</span><br><span class="subtitle">可开 ' + esc(moneyCents(item.orderInvoiceAvailableCents)) + ' / 待红冲 ' + esc(moneyCents(item.orderCreditNoteDueCents)) + '</span></td>' +
		      '<td>' + esc(item.invoiceTitle || '-') + '<br><span class="subtitle">' + esc(item.provider || 'manual') + ' / ' + provider + '</span>' + documentLink + '</td>' +
		      '<td>' + esc(item.issuedAt || item.failedAt || item.canceledAt || item.processingAt || item.requestedAt || '-') + failure + '</td>' +
		      '<td>v' + esc(item.version || 0) + '<br>' + actions.join(' ') + '</td>' +
		    '</tr>';
		  }).join('');
		}
		async function loadInvoiceDocuments() {
		  try {
		    const res = await fetch('/dashboard/saasAdmin/invoiceDocuments?' + invoiceDocumentParams(100).toString(), { headers: authHeader() });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const data = body.data || {};
		    renderInvoiceDocumentSummary(data.summary || {});
		    renderInvoiceDocuments(data.documents || []);
		  } catch (err) {
		    invoiceDocumentsEl.innerHTML = '<tr><td colspan="7" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
		    invoiceDocumentCountEl.textContent = '';
		  }
		}
		function fillInvoiceProfile(profile) {
		  profile = profile || {};
		  invoiceProfileTenantInput.value = profile.tenantId || invoiceProfileTenantInput.value || '';
		  invoiceProfileTypeInput.value = profile.invoiceType || 'normal';
		  invoiceProfileTitleInput.value = profile.invoiceTitle || '';
		  invoiceProfileTaxIDInput.value = profile.taxIdentifier || '';
		  invoiceProfileEmailInput.value = profile.email || '';
		  invoiceProfilePhoneInput.value = profile.phone || '';
		  invoiceProfileAddressInput.value = profile.registeredAddress || '';
		  invoiceProfileBankNameInput.value = profile.bankName || '';
		  invoiceProfileBankAccountInput.value = profile.bankAccount || '';
		  invoiceProfileRecipientInput.value = profile.recipientName || '';
		  invoiceProfileVersionInput.value = profile.version || 0;
		  invoiceProfileRemarkInput.value = profile.remark || '';
		}
		async function loadInvoiceProfile(tenantId) {
		  tenantId = Number(tenantId || invoiceProfileTenantInput.value.trim());
		  if (!tenantId) {
		    statusEl.textContent = '请填写开票资料租户 ID';
		    return;
		  }
		  invoiceProfileTenantInput.value = String(tenantId);
		  try {
		    const res = await fetch('/dashboard/saasAdmin/invoiceProfile?tenantId=' + encodeURIComponent(tenantId), { headers: authHeader() });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    fillInvoiceProfile((body.data || {}).profile || { tenantId: tenantId });
		    statusEl.textContent = ((body.data || {}).profile || {}).exists === false ? '该租户尚未维护开票资料' : '开票资料已读取';
		  } catch (err) {
		    statusEl.textContent = err.message || String(err);
		  }
		}
		async function saveInvoiceProfile() {
		  const tenantId = Number(invoiceProfileTenantInput.value.trim());
		  if (!tenantId || !invoiceProfileTitleInput.value.trim() || !invoiceProfileTaxIDInput.value.trim() || !invoiceProfileEmailInput.value.trim()) {
		    statusEl.textContent = '请填写租户、发票抬头、税号和接收邮箱';
		    return;
		  }
		  const payload = {
		    tenantId, invoiceType: invoiceProfileTypeInput.value, invoiceTitle: invoiceProfileTitleInput.value.trim(),
		    taxIdentifier: invoiceProfileTaxIDInput.value.trim(), email: invoiceProfileEmailInput.value.trim(),
		    phone: invoiceProfilePhoneInput.value.trim(), registeredAddress: invoiceProfileAddressInput.value.trim(),
		    bankName: invoiceProfileBankNameInput.value.trim(), bankAccount: invoiceProfileBankAccountInput.value.trim(),
		    recipientName: invoiceProfileRecipientInput.value.trim(), expectedVersion: Number(invoiceProfileVersionInput.value || 0),
		    remark: invoiceProfileRemarkInput.value.trim(),
		  };
		  statusEl.textContent = '正在保存开票资料';
		  try {
		    const res = await fetch('/dashboard/saasAdmin/invoiceProfile', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    fillInvoiceProfile((body.data || {}).profile || {});
		    statusEl.textContent = (body.data || {}).created ? '开票资料已创建' : '开票资料已更新';
		    loadOperations();
		  } catch (err) {
		    statusEl.textContent = err.message || String(err);
		  }
		}
			function selectInvoiceDocument(button) {
			  invoiceTransitionDocumentInput.value = button.dataset.documentNo || '';
			  invoiceTransitionVersionInput.value = button.dataset.version || '';
			  invoiceTransitionProviderInput.value = button.dataset.provider || 'manual';
			}
			function renderInvoiceIssueApprovalState() {
			  const button = document.getElementById('transitionInvoiceDocument');
			  if (!button) return;
			  button.textContent = invoiceTransitionStatusInput.value === 'issued' && approvalActionRequired('billing.invoice.issue', 0)
			    ? '提交开具审批'
			    : '应用状态';
			}
		function prefillCreditNote(button) {
		  invoiceCreateKindInput.value = 'credit_note';
		  invoiceCreateTenantInput.value = button.dataset.tenantId || '';
		  invoiceCreateOrderInput.value = button.dataset.orderNo || '';
		  invoiceOriginalDocumentInput.value = button.dataset.documentNo || '';
		  invoiceCreateAmountInput.value = (Number(button.dataset.amountCents || 0) / 100).toFixed(2);
		  invoiceCreateCurrencyInput.value = button.dataset.currency || 'CNY';
		  invoiceCreateIdempotencyInput.value = '';
		  invoiceCreateRemarkInput.focus();
		}
		function prefillInvoiceFromOrder(button) {
		  invoiceCreateKindInput.value = 'invoice';
		  invoiceCreateTenantInput.value = button.dataset.tenantId || '';
		  invoiceCreateOrderInput.value = button.dataset.orderNo || '';
		  invoiceOriginalDocumentInput.value = '';
		  invoiceCreateAmountInput.value = (Number(button.dataset.amountCents || 0) / 100).toFixed(2);
		  invoiceCreateCurrencyInput.value = button.dataset.currency || 'CNY';
		  invoiceCreateIdempotencyInput.value = '';
		  invoiceCreateRemarkInput.focus();
		}
		async function createInvoiceDocument() {
		  const kind = invoiceCreateKindInput.value || 'invoice';
		  const tenantId = Number(invoiceCreateTenantInput.value.trim());
		  const orderNo = invoiceCreateOrderInput.value.trim();
		  const amount = invoiceCreateAmountInput.value.trim();
		  const originalDocumentNo = invoiceOriginalDocumentInput.value.trim();
		  if (!tenantId || !orderNo || !amount || (kind === 'credit_note' && !originalDocumentNo)) {
		    statusEl.textContent = kind === 'credit_note' ? '红票必须填写租户、支付订单、原蓝票和金额' : '蓝票必须填写租户、支付订单和金额';
		    return;
		  }
		  const idempotencyKey = invoiceCreateIdempotencyInput.value.trim() || ('page-' + kind + ':' + orderNo + ':' + Date.now());
		  invoiceCreateIdempotencyInput.value = idempotencyKey;
		  const payload = {
		    tenantId, orderNo, amount, originalDocumentNo, idempotencyKey,
		    currency: invoiceCreateCurrencyInput.value.trim() || 'CNY', provider: invoiceCreateProviderInput.value.trim() || 'manual',
		    remark: invoiceCreateRemarkInput.value.trim(),
		  };
		  if (invoiceCreateDocumentInput.value.trim()) payload.documentNo = invoiceCreateDocumentInput.value.trim();
		  statusEl.textContent = '正在创建' + (kind === 'credit_note' ? '红票' : '蓝票') + '申请';
		  try {
		    const endpoint = kind === 'credit_note' ? '/dashboard/saasAdmin/creditNote' : '/dashboard/saasAdmin/invoice';
		    const res = await fetch(endpoint, { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const data = body.data || {};
		    const document = data.document || {};
		    invoiceCreateDocumentInput.value = document.documentNo || invoiceCreateDocumentInput.value;
		    statusEl.textContent = data.idempotent ? '该发票申请已存在' : '发票申请已创建：' + fmt(document.documentNo);
		    await Promise.all([loadInvoiceDocuments(), loadPaymentOrders()]);
		    loadOperations();
		  } catch (err) {
		    statusEl.textContent = err.message || String(err);
		  }
		}
		async function transitionInvoiceDocument(statusOverride, button) {
		  if (button) selectInvoiceDocument(button);
		  const documentNo = invoiceTransitionDocumentInput.value.trim();
		  const expectedVersion = Number(invoiceTransitionVersionInput.value || 0);
		  const status = statusOverride || invoiceTransitionStatusInput.value;
		  if (!documentNo || !expectedVersion) {
		    statusEl.textContent = '请选择单据并确认当前版本';
		    return;
		  }
			  if (statusOverride) {
			    invoiceTransitionStatusInput.value = status;
			    renderInvoiceIssueApprovalState();
			  }
		  if (status === 'issued' && !invoiceProviderDocumentInput.value.trim()) {
		    const providerNo = window.prompt('请输入渠道发票号', documentNo);
		    if (providerNo === null || !providerNo.trim()) return;
		    invoiceProviderDocumentInput.value = providerNo.trim();
		  }
		  if (status === 'failed' && !invoiceFailureMessageInput.value.trim()) {
		    const message = window.prompt('请输入开票失败原因', '开票资料或渠道处理失败');
		    if (message === null || !message.trim()) return;
		    invoiceFailureMessageInput.value = message.trim();
		  }
		  if (status === 'canceled' && !window.confirm('确认取消发票申请 ' + documentNo + '？')) return;
		  const payload = {
		    documentNo, expectedVersion, status, provider: invoiceTransitionProviderInput.value.trim() || 'manual',
		    providerDocumentNo: invoiceProviderDocumentInput.value.trim(), documentUrl: invoiceDocumentURLInput.value.trim(),
		    failureCode: invoiceFailureCodeInput.value.trim(), failureMessage: invoiceFailureMessageInput.value.trim(),
		    remark: invoiceTransitionRemarkInput.value.trim(),
			  };
			  const issuedAt = subscriptionDateTimePayload(invoiceIssuedAtInput.value);
			  if (issuedAt) payload.issuedAt = issuedAt;
			  if (status === 'issued' && approvalActionRequired('billing.invoice.issue', 0)) {
			    statusEl.textContent = '正在提交开具审批';
			    try {
			      await requestHighRiskApproval(
			        'billing.invoice.issue',
			        payload,
			        payload.remark || ('正式开具发票单据 ' + documentNo)
			      );
			      statusEl.textContent = '开具审批已提交，等待两人复核';
			    } catch (err) {
			      statusEl.textContent = err.message || String(err);
			    }
			    return;
			  }
			  statusEl.textContent = '正在更新发票状态';
		  try {
		    const res = await fetch('/dashboard/saasAdmin/invoiceTransition', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const document = (body.data || {}).document || {};
		    invoiceTransitionVersionInput.value = document.version || expectedVersion;
		    statusEl.textContent = '发票状态已更新：' + fmt(document.documentNo) + ' -> ' + fmt(document.status);
		    await Promise.all([loadInvoiceDocuments(), loadPaymentOrders()]);
		    loadOperations();
		  } catch (err) {
		    statusEl.textContent = err.message || String(err);
		  }
		}
			function paymentSettlementSyncStatusView(status) {
			  const views = {
			    running: ['运行中', 'warning'], succeeded: ['成功', 'ok'], previewed: ['预演', ''], failed: ['失败', 'danger'],
			  };
			  return views[status] || [status || '-', ''];
			}
			function paymentSettlementSyncParams(limit) {
			  const params = new URLSearchParams({ limit: String(limit || 100) });
			  if (paymentSettlementSyncProviderInput.value) params.set('provider', paymentSettlementSyncProviderInput.value);
			  if (paymentSettlementSyncSourceInput.value) params.set('source', paymentSettlementSyncSourceInput.value);
			  if (paymentSettlementSyncStatusInput.value) params.set('status', paymentSettlementSyncStatusInput.value);
			  return params;
			}
			function renderPaymentSettlementSyncSummary(summary) {
			  summary = summary || {};
			  const values = [
			    [summary.providerCount || 0, '运行中 ' + fmt(summary.activeCount || 0)],
			    [summary.runCount || 0, '成功/预演 ' + fmt(summary.succeededCount || 0) + '/' + fmt(summary.previewedCount || 0)],
			    [summary.importedCount || 0, '幂等 ' + fmt(summary.idempotentCount || 0)],
			    [summary.entryCount || 0, '渠道流水'],
			    [summary.openIssueCount || 0, '累计差异 ' + fmt(summary.issueCount || 0)],
			    [summary.failedCount || 0, '需检查 Bridge'],
			  ];
			  [...paymentSettlementSyncSummaryEl.querySelectorAll('.tile')].forEach((tile, index) => {
			    const value = values[index] || ['-', ''];
			    tile.querySelector('.value').innerHTML = esc(value[0]) + '<br><span class="subtitle">' + esc(value[1]) + '</span>';
			  });
			}
			function renderPaymentSettlementSyncProviders(providers) {
			  const selected = paymentSettlementSyncProviderInput.value;
			  const normalized = Array.from(new Set((providers || []).filter(Boolean)));
			  paymentSettlementSyncProviderInput.innerHTML = '<option value="">全部渠道</option>' + normalized.map(provider => '<option value="' + esc(provider) + '">' + esc(provider) + '</option>').join('');
			  if (normalized.includes(selected)) paymentSettlementSyncProviderInput.value = selected;
			  else if (normalized.length === 1) paymentSettlementSyncProviderInput.value = normalized[0];
			}
			function renderPaymentSettlementSyncStates(items) {
			  if (!items.length) {
			    paymentSettlementSyncStatesEl.innerHTML = '<tr><td colspan="5" class="empty">暂无同步渠道</td></tr>';
			    return;
			  }
			  paymentSettlementSyncStatesEl.innerHTML = items.map(item => {
			    const running = Number(item.activeRunId || 0) > 0;
			    return '<tr>' +
			      '<td><strong>' + esc(item.provider || '-') + '</strong><br><span class="subtitle">v' + esc(item.version || 0) + '</span></td>' +
			      '<td>' + pill(running ? '运行中' : '空闲', running ? 'warning' : 'ok') + '<br><span class="subtitle">run ' + esc(item.activeRunId || '-') + '</span></td>' +
			      '<td>' + esc(item.cursor || '初始游标') + '</td>' +
			      '<td>' + esc(item.lastAttemptAt || '-') + '<br><span class="subtitle">成功 ' + esc(item.lastSuccessAt || '-') + '</span></td>' +
			      '<td>' + (item.lastError ? '<span class="bad">' + esc(item.lastError) + '</span>' : '<span class="ok">无错误</span>') + '</td>' +
			    '</tr>';
			  }).join('');
			}
			function renderPaymentSettlementSyncRuns(items) {
			  if (!items.length) {
			    paymentSettlementSyncRunsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无同步记录</td></tr>';
			    return;
			  }
			  paymentSettlementSyncRunsEl.innerHTML = items.map(item => {
			    const state = paymentSettlementSyncStatusView(item.status);
			    const result = item.errorMessage ? '<span class="bad">' + esc(item.errorMessage) + '</span>' : '<span class="ok">完成</span>';
			    return '<tr>' +
			      '<td>' + pill(state[0], state[1]) + (item.dryRun ? '<br><span class="subtitle">不写入</span>' : '') + '</td>' +
			      '<td>' + esc(item.runNo || '-') + '<br><span class="subtitle">ID ' + esc(item.id || '-') + '</span></td>' +
			      '<td>' + esc(item.provider || '-') + '<br><span class="subtitle">' + (item.source === 'cron' ? '定时任务' : '人工触发') + '</span></td>' +
			      '<td>' + esc(item.cursorBefore || '初始') + '<br><span class="subtitle">至 ' + esc(item.cursorAfter || '-') + '</span></td>' +
			      '<td>拉取 ' + esc(item.fetchedBatchCount || 0) + ' / 导入 ' + esc(item.importedBatchCount || 0) + '<br><span class="subtitle">幂等 ' + esc(item.idempotentBatchCount || 0) + ' / 明细 ' + esc(item.entryCount || 0) + '</span></td>' +
			      '<td>' + esc(item.openIssueCount || 0) + '<br><span class="subtitle">累计 ' + esc(item.issueCount || 0) + '</span></td>' +
			      '<td>' + esc(item.startedAt || '-') + '<br><span class="subtitle">' + result + '</span></td>' +
			    '</tr>';
			  }).join('');
			}
			async function loadPaymentSettlementSyncRuns() {
			  paymentSettlementSyncStateEl.textContent = '正在读取';
			  try {
			    const res = await fetch('/dashboard/saasAdmin/paymentSettlementSyncRuns?' + paymentSettlementSyncParams(100).toString(), { headers: authHeader() });
			    const body = await res.json();
			    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
			    const data = body.data || {};
			    renderPaymentSettlementSyncProviders(data.configuredProviders || []);
			    renderPaymentSettlementSyncSummary(data.summary || {});
			    renderPaymentSettlementSyncStates(data.states || []);
			    renderPaymentSettlementSyncRuns(data.runs || []);
			    const enabled = !!data.enabled;
			    previewPaymentSettlementSyncButton.disabled = !enabled;
			    runPaymentSettlementSyncButton.disabled = !enabled;
			    paymentSettlementSyncStateEl.textContent = enabled ? ('已配置 ' + fmt((data.configuredProviders || []).length) + ' 个渠道') : 'Bridge 未配置';
			  } catch (err) {
			    paymentSettlementSyncStatesEl.innerHTML = '<tr><td colspan="5" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
			    paymentSettlementSyncRunsEl.innerHTML = '<tr><td colspan="7" class="empty">读取失败</td></tr>';
			    paymentSettlementSyncStateEl.textContent = '读取失败';
			  }
			}
			async function triggerPaymentSettlementSync(dryRun) {
			  const provider = paymentSettlementSyncProviderInput.value;
			  if (!provider) {
			    paymentSettlementSyncStateEl.textContent = '请选择具体同步渠道';
			    return;
			  }
			  if (!dryRun && !window.confirm('确认立即同步渠道 ' + provider + ' 的结算流水？')) return;
			  previewPaymentSettlementSyncButton.disabled = true;
			  runPaymentSettlementSyncButton.disabled = true;
			  paymentSettlementSyncStateEl.textContent = dryRun ? '正在预演' : '正在同步';
			  try {
			    const res = await fetch('/dashboard/saasAdmin/paymentSettlementSync', {
			      method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
			      body: JSON.stringify({ provider, dryRun: !!dryRun }),
			    });
			    const body = await res.json();
			    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
			    const run = ((body.data || {}).run || {});
			    paymentSettlementSyncStateEl.textContent = dryRun ? ('预演完成，拉取 ' + fmt(run.fetchedBatchCount || 0) + ' 批') : ('同步完成，导入 ' + fmt(run.importedBatchCount || 0) + ' 批');
			    await Promise.all([loadPaymentSettlementSyncRuns(), loadPaymentSettlementBatches()]);
			    loadOperations();
			  } catch (err) {
			    paymentSettlementSyncStateEl.textContent = err.message || String(err);
			    await loadPaymentSettlementSyncRuns();
			  } finally {
			    previewPaymentSettlementSyncButton.disabled = false;
			    runPaymentSettlementSyncButton.disabled = false;
			  }
			}
			function paymentSettlementBatchStatusView(status) {
		  return status === 'closed' ? ['已关闭', 'ok'] : ['待关闭', 'warn'];
		}
		function paymentSettlementReconciliationView(status) {
		  const views = {
		    matched: ['已匹配', 'ok'], missing_internal: ['缺内部单', 'bad'], identifier_conflict: ['标识冲突', 'bad'],
		    status_mismatch: ['状态不符', 'warn'], currency_mismatch: ['币种不符', 'warn'], amount_mismatch: ['金额不符', 'bad'],
		  };
		  return views[status] || [status || '-', 'muted'];
		}
		function paymentSettlementHandlingView(status) {
		  const views = { none: ['无需处理', 'muted'], open: ['待处理', 'bad'], resolved: ['已解决', 'ok'], ignored: ['已忽略', 'warn'] };
		  return views[status] || [status || '-', 'muted'];
		}
		function renderPaymentSettlementSummary(summary) {
		  summary = summary || {};
		  const values = [
		    [summary.batchCount || 0, '已关闭 ' + fmt(summary.closedCount || 0)],
		    [summary.entryCount || 0, '收款/退款 ' + fmt(summary.paymentCount || 0) + '/' + fmt(summary.refundCount || 0)],
		    [summary.matchedCount || 0, '差异 ' + fmt(summary.issueCount || 0)],
		    [summary.openIssueCount || 0, '解决/忽略 ' + fmt(summary.resolvedIssueCount || 0) + '/' + fmt(summary.ignoredIssueCount || 0)],
		    [moneyCents(summary.totalNetCents || 0), '手续费 ' + moneyCents(summary.totalFeeCents || 0)],
		    [moneyCents(summary.differenceAmountCents || 0), '渠道 ' + fmt(summary.providerCount || 0)],
		  ];
		  [...paymentSettlementSummaryEl.querySelectorAll('.tile')].forEach((tile, index) => {
		    const value = values[index] || ['-', ''];
		    tile.querySelector('.value').innerHTML = esc(value[0]) + '<br><span class="subtitle">' + esc(value[1]) + '</span>';
		  });
		}
		function renderPaymentSettlementBatches(items) {
		  if (!items.length) {
		    paymentSettlementBatchesEl.innerHTML = '<tr><td colspan="7" class="empty">暂无结算批次</td></tr>';
		    return;
		  }
		  paymentSettlementBatchesEl.innerHTML = items.map(item => {
		    const state = paymentSettlementBatchStatusView(item.status);
		    const actions = [
		      '<button type="button" data-action="select-settlement" data-batch-no="' + esc(item.batchNo) + '" data-version="' + esc(item.version || 0) + '">明细</button>',
		    ];
		    if (item.status === 'closed') {
		      actions.push('<button type="button" data-action="transition-settlement" data-transition="reopen" data-batch-no="' + esc(item.batchNo) + '" data-version="' + esc(item.version || 0) + '">重开</button>');
		    } else {
		      actions.push('<button type="button" data-action="reconcile-settlement" data-dry-run="true" data-batch-no="' + esc(item.batchNo) + '" data-version="' + esc(item.version || 0) + '">预演</button>');
		      actions.push('<button type="button" data-action="reconcile-settlement" data-dry-run="false" data-batch-no="' + esc(item.batchNo) + '" data-version="' + esc(item.version || 0) + '">重跑</button>');
		      actions.push('<button type="button" data-action="transition-settlement" data-transition="close" data-batch-no="' + esc(item.batchNo) + '" data-version="' + esc(item.version || 0) + '"' + (Number(item.openIssueCount || 0) > 0 ? ' disabled' : '') + '>关闭</button>');
		    }
		    return '<tr>' +
		      '<td>' + pill(state[0], state[1]) + '<br><span class="subtitle">v' + esc(item.version || 0) + '</span></td>' +
		      '<td>' + esc(item.batchNo) + '<br><span class="subtitle">' + esc(item.providerSettlementNo || '-') + '</span></td>' +
		      '<td>' + esc(item.provider || '-') + ' / ' + esc(item.currency || 'CNY') + '<br><span class="subtitle">' + esc(item.periodStart || '-') + ' 至 ' + esc(item.periodEnd || '-') + '</span></td>' +
		      '<td>' + esc(item.entryCount || 0) + '<br><span class="subtitle">匹配 ' + esc(item.matchedCount || 0) + ' / 差异 ' + esc(item.issueCount || 0) + '</span></td>' +
		      '<td>' + esc(moneyCents(item.totalNetCents || 0)) + '<br><span class="subtitle">交易 ' + esc(moneyCents(item.totalAmountCents || 0)) + ' / 费 ' + esc(moneyCents(item.totalFeeCents || 0)) + '</span></td>' +
		      '<td><span class="' + (Number(item.openIssueCount || 0) ? 'bad' : 'ok') + '">待处理 ' + esc(item.openIssueCount || 0) + '</span><br><span class="subtitle">差额 ' + esc(moneyCents(item.differenceAmountCents || 0)) + '</span></td>' +
		      '<td>' + actions.join(' ') + '</td>' +
		    '</tr>';
		  }).join('');
		}
		async function loadPaymentSettlementBatches() {
		  try {
		    const res = await fetch('/dashboard/saasAdmin/paymentSettlementBatches?' + paymentSettlementBatchParams(100).toString(), { headers: authHeader() });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const data = body.data || {};
		    renderPaymentSettlementSummary(data.summary || {});
		    renderPaymentSettlementBatches(data.batches || []);
		    paymentSettlementBatchCountEl.textContent = '命中 ' + fmt((data.summary || {}).batchCount || 0) + '，待处理差异 ' + fmt((data.summary || {}).openIssueCount || 0);
		  } catch (err) {
		    paymentSettlementBatchesEl.innerHTML = '<tr><td colspan="7" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
		    paymentSettlementBatchCountEl.textContent = '';
		  }
		}
		function selectPaymentSettlementBatch(button) {
		  paymentSettlementSelectedBatchInput.value = button.dataset.batchNo || '';
		  selectedPaymentSettlementVersion = Number(button.dataset.version || 0);
		  loadPaymentSettlementEntries();
		}
		function renderPaymentSettlementEntries(items, summary) {
		  summary = summary || {};
		  paymentSettlementEntryCountEl.textContent = '明细 ' + fmt(summary.entryCount || items.length || 0) + '，匹配 ' + fmt(summary.matchedCount || 0) + '，待处理 ' + fmt(summary.openIssueCount || 0);
		  if (!items.length) {
		    paymentSettlementEntriesEl.innerHTML = '<tr><td colspan="7" class="empty">暂无命中明细</td></tr>';
		    return;
		  }
		  paymentSettlementEntriesEl.innerHTML = items.map(item => {
		    const reconciliation = paymentSettlementReconciliationView(item.reconciliationStatus);
		    const handling = paymentSettlementHandlingView(item.handlingStatus);
			    const internalIdentifier = item.transactionType === 'refund' ? item.refundNo : item.orderNo;
			    const providerIdentifier = item.transactionType === 'refund' ? item.providerRefundNo : item.providerOrderNo;
		    const actions = [];
			    if (item.reconciliationStatus !== 'matched' && item.batchStatus !== 'closed') {
			      if (item.handlingStatus === 'open') {
			        actions.push('<button type="button" data-action="resolve-settlement-entry" data-handling="resolved" data-entry-id="' + esc(item.id) + '" data-version="' + esc(item.version || 0) + '">解决提审</button>');
			        actions.push('<button type="button" data-action="resolve-settlement-entry" data-handling="ignored" data-entry-id="' + esc(item.id) + '" data-version="' + esc(item.version || 0) + '">忽略提审</button>');
			      } else {
			        actions.push('<button type="button" data-action="resolve-settlement-entry" data-handling="open" data-entry-id="' + esc(item.id) + '" data-version="' + esc(item.version || 0) + '">重开提审</button>');
		      }
		    }
		    return '<tr>' +
		      '<td>' + pill(reconciliation[0], reconciliation[1]) + '<br>' + pill(handling[0], handling[1]) + '</td>' +
		      '<td>' + esc(item.providerTransactionNo || '-') + '<br><span class="subtitle">' + esc(item.transactionType || '-') + ' / 行 ' + esc(item.lineNo || 0) + '</span></td>' +
			      '<td>' + esc(item.matchedInternalNo || '-') + '<br><span class="subtitle">' + esc(item.matchedTenantName || '-') + ' / ' + esc(internalIdentifier || '-') + ' / ' + esc(providerIdentifier || '-') + '</span></td>' +
		      '<td>' + esc(moneyCents(item.amountCents || 0)) + ' ' + esc(item.currency || 'CNY') + '<br><span class="subtitle">净额 ' + esc(moneyCents(item.netAmountCents || 0)) + ' / 费 ' + esc(moneyCents(item.feeCents || 0)) + '</span></td>' +
		      '<td>' + esc(moneyCents(item.expectedAmountCents || 0)) + ' ' + esc(item.expectedCurrency || '-') + '<br><span class="subtitle">差额 ' + esc(moneyCents(item.differenceAmountCents || 0)) + ' / ' + esc(item.expectedStatus || '-') + '</span></td>' +
		      '<td>' + esc(item.issueMessage || '-') + (item.handlingReason ? '<br><span class="subtitle">' + esc(item.handlingReason) + '</span>' : '') + '</td>' +
		      '<td>v' + esc(item.version || 0) + (actions.length ? '<br>' + actions.join(' ') : '') + '</td>' +
		    '</tr>';
		  }).join('');
		}
		async function loadPaymentSettlementEntries() {
		  if (!paymentSettlementSelectedBatchInput.value.trim()) {
		    paymentSettlementEntriesEl.innerHTML = '<tr><td colspan="7" class="empty">请选择结算批次</td></tr>';
		    paymentSettlementEntryCountEl.textContent = '';
		    return;
		  }
		  try {
		    const res = await fetch('/dashboard/saasAdmin/paymentSettlementEntries?' + paymentSettlementEntryParams(200).toString(), { headers: authHeader() });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const data = body.data || {};
		    renderPaymentSettlementEntries(data.entries || [], data.summary || {});
		  } catch (err) {
		    paymentSettlementEntriesEl.innerHTML = '<tr><td colspan="7" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
		    paymentSettlementEntryCountEl.textContent = '';
		  }
		}
		async function importPaymentSettlement() {
		  const csvText = paymentSettlementCSVInput.value.trim();
		  if (!csvText) {
		    statusEl.textContent = '请选择或粘贴结算 CSV';
		    return;
		  }
		  paymentSettlementImportStateInput.value = '正在导入';
		  try {
		    const res = await fetch('/dashboard/saasAdmin/paymentSettlementImport', { method: 'POST', headers: Object.assign({ 'Content-Type': 'text/csv; charset=utf-8' }, authHeader()), body: csvText });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const data = body.data || {};
		    const batch = data.batch || {};
		    paymentSettlementSelectedBatchInput.value = batch.batchNo || '';
		    selectedPaymentSettlementVersion = Number(batch.version || 0);
		    paymentSettlementImportStateInput.value = data.idempotent ? '批次已存在' : '导入完成';
		    statusEl.textContent = (data.idempotent ? '结算批次已存在：' : '结算批次已导入：') + fmt(batch.batchNo || '-');
		    await Promise.all([loadPaymentSettlementBatches(), loadPaymentSettlementEntries()]);
		    loadOperations();
		  } catch (err) {
		    paymentSettlementImportStateInput.value = '导入失败';
		    statusEl.textContent = err.message || String(err);
		  }
		}
		async function reconcilePaymentSettlement(dryRun, button) {
		  if (button) {
		    paymentSettlementSelectedBatchInput.value = button.dataset.batchNo || '';
		    selectedPaymentSettlementVersion = Number(button.dataset.version || 0);
		  }
		  const batchNo = paymentSettlementSelectedBatchInput.value.trim();
		  if (!batchNo || !selectedPaymentSettlementVersion) return;
		  statusEl.textContent = dryRun ? '正在预演结算对账' : '正在重跑结算对账';
		  try {
		    const res = await fetch('/dashboard/saasAdmin/paymentSettlementReconcile', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify({ batchNo, expectedVersion: selectedPaymentSettlementVersion, dryRun: !!dryRun }) });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const data = body.data || {};
		    const batch = data.batch || {};
		    const entries = data.entries || {};
		    renderPaymentSettlementEntries(entries.entries || [], entries.summary || {});
		    statusEl.textContent = dryRun ? '对账预演完成，未写入' : '结算对账已重跑';
		    if (!dryRun) {
		      selectedPaymentSettlementVersion = Number(batch.version || selectedPaymentSettlementVersion);
		      await loadPaymentSettlementBatches();
		      loadOperations();
		    }
		  } catch (err) {
		    statusEl.textContent = err.message || String(err);
		  }
		}
		async function resolvePaymentSettlementEntry(button) {
		  const handlingStatus = button.dataset.handling;
		  const labels = { resolved: '解决', ignored: '忽略', open: '重新打开' };
			  const reason = window.prompt('请输入' + (labels[handlingStatus] || '处理') + '原因', '财务核验处理');
			  if (reason === null || !reason.trim()) return;
			  try {
			    const payload = { entryId: Number(button.dataset.entryId || 0), expectedVersion: Number(button.dataset.version || 0), handlingStatus, reason: reason.trim() };
			    if (approvalActionRequired('payment.settlement.resolve', 0)) {
			      await requestHighRiskApproval('payment.settlement.resolve', payload, reason.trim());
			      statusEl.textContent = '结算差异' + (labels[handlingStatus] || '处理') + '已提交双人审批';
			      return;
			    }
			    const res = await fetch('/dashboard/saasAdmin/paymentSettlementResolve', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const batch = (body.data || {}).batch || {};
		    selectedPaymentSettlementVersion = Number(batch.version || selectedPaymentSettlementVersion);
		    statusEl.textContent = '结算差异已' + (labels[handlingStatus] || '处理');
		    await Promise.all([loadPaymentSettlementBatches(), loadPaymentSettlementEntries()]);
		    loadOperations();
		  } catch (err) {
		    statusEl.textContent = err.message || String(err);
		  }
		}
		async function transitionPaymentSettlement(action, button) {
		  const batchNo = button.dataset.batchNo || paymentSettlementSelectedBatchInput.value.trim();
		  const expectedVersion = Number(button.dataset.version || selectedPaymentSettlementVersion || 0);
		  if (!batchNo || !expectedVersion) return;
		  const reason = window.prompt(action === 'close' ? '请输入关闭说明' : '请输入重开说明', action === 'close' ? '结算差异已处理完成' : '重新核对渠道结算');
		  if (reason === null || !reason.trim()) return;
		  try {
		    const payload = { batchNo, expectedVersion, action, reason: reason.trim() };
			    const approvalAction = action === 'close' ? 'payment.settlement.close' : 'payment.settlement.reopen';
			    if (approvalActionRequired(approvalAction, 0)) {
			      await requestHighRiskApproval(approvalAction, payload, reason.trim());
			      statusEl.textContent = action === 'close' ? '结算关账已提交审批' : '结算重开已提交审批';
			      return;
			    }
		    const res = await fetch('/dashboard/saasAdmin/paymentSettlementTransition', { method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload) });
		    const body = await res.json();
		    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		    const batch = (body.data || {}).batch || {};
		    paymentSettlementSelectedBatchInput.value = batch.batchNo || batchNo;
		    selectedPaymentSettlementVersion = Number(batch.version || 0);
		    statusEl.textContent = action === 'close' ? '结算批次已关闭' : '结算批次已重开';
		    await Promise.all([loadPaymentSettlementBatches(), loadPaymentSettlementEntries()]);
		    loadOperations();
		  } catch (err) {
		    statusEl.textContent = err.message || String(err);
		  }
		}
	    function renderPaymentWebhookEvents(items) {
      paymentWebhookEventCountEl.textContent = items.length + ' 条';
      if (!items.length) {
        paymentWebhookEventsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无支付回调事件</td></tr>';
        return;
      }
      paymentWebhookEventsEl.innerHTML = items.map(item => {
        const state = paymentWebhookStatusView(item.status);
        const result = item.lastError ? '<span class="bad">' + esc(item.lastError) + '</span>' : '<span class="ok">已校验</span>';
        return '<tr>' +
          '<td>' + pill(state[0], state[1]) + '<br><span class="subtitle">尝试 ' + esc(item.attempts || 0) + '</span></td>' +
          '<td>' + esc(item.eventType || '-') + '<br><span class="subtitle">' + esc(item.eventId || '-') + '</span></td>' +
		  '<td>' + esc(item.orderNo || '-') + '<br><span class="subtitle">' + (item.refundNo ? ('退款 ' + esc(item.refundNo)) : ('ID ' + esc(item.orderId || '-'))) + '</span></td>' +
		  '<td>' + esc(item.provider || '-') + '<br><span class="subtitle">' + esc(item.providerRefundNo || item.providerOrderNo || '-') + '</span></td>' +
          '<td>' + esc(item.occurredAt || item.createdAt || '-') + '<br><span class="subtitle">处理 ' + esc(item.processedAt || '-') + '</span></td>' +
          '<td>' + result + '</td>' +
        '</tr>';
      }).join('');
    }
    async function loadPaymentWebhookEvents() {
      try {
        const res = await fetch('/dashboard/saasAdmin/paymentWebhookEvents?' + paymentWebhookEventParams(50).toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderPaymentWebhookEvents((body.data || {}).events || []);
      } catch (err) {
        paymentWebhookEventsEl.innerHTML = '<tr><td colspan="6" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        paymentWebhookEventCountEl.textContent = '';
      }
    }
    async function loadPaymentOrders() {
      try {
        const res = await fetch('/dashboard/saasAdmin/paymentOrders?' + paymentOrderParams(100).toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderPaymentSummary(data.summary || {});
        renderPaymentOrders(data.orders || []);
		loadPaymentRefunds();
        loadPaymentWebhookEvents();
      } catch (err) {
        paymentOrdersEl.innerHTML = '<tr><td colspan="7" class="empty">仅平台管理员可查看支付订单</td></tr>';
        paymentOrderCountEl.textContent = err.message || String(err);
      }
    }
    async function createPaymentOrder() {
      const tenantId = Number(paymentTenantInput.value.trim());
      const packageCode = paymentPackageCodeInput.value.trim();
      const amount = paymentAmountInput.value.trim();
      const billingCycle = paymentBillingCycleInput.value;
      if (!tenantId || !packageCode || !amount) {
        statusEl.textContent = '请填写租户、套餐和金额';
        return;
      }
      const serviceExpiresAt = subscriptionDateTimePayload(paymentServiceExpiresInput.value);
      if (billingCycle !== 'lifetime' && !serviceExpiresAt) {
        statusEl.textContent = '非永久套餐必须填写服务到期时间';
        return;
      }
      const idempotencyKey = paymentIdempotencyKeyInput.value.trim() || ('page-payment:' + tenantId + ':' + Date.now());
      paymentIdempotencyKeyInput.value = idempotencyKey;
      const payload = {
        tenantId,
        provider: paymentCreateProviderInput.value.trim() || 'gateway',
        packageCode,
        billingCycle,
        amount,
        currency: paymentCurrencyInput.value.trim() || 'CNY',
        maxDunningAttempts: Number(paymentMaxDunningAttemptsInput.value.trim() || 3),
        checkoutUrl: paymentCheckoutUrlInput.value.trim(),
        idempotencyKey,
        remark: paymentRemarkInput.value.trim(),
      };
      if (paymentOrderNoInput.value.trim()) payload.orderNo = paymentOrderNoInput.value.trim();
      if (serviceExpiresAt) payload.serviceExpiresAt = serviceExpiresAt;
      const checkoutExpiresAt = subscriptionDateTimePayload(paymentCheckoutExpiresInput.value);
      if (checkoutExpiresAt) payload.checkoutExpiresAt = checkoutExpiresAt;
	      const requiresApproval = approvalActionRequired('payment.order.create', 0);
	      if (requiresApproval && !payload.checkoutExpiresAt) {
	        paymentCheckoutExpiresInput.value = localDateTimeInputValue(new Date(Date.now() + 30 * 60 * 1000));
	        payload.checkoutExpiresAt = subscriptionDateTimePayload(paymentCheckoutExpiresInput.value);
	      }
	      statusEl.textContent = requiresApproval ? '正在提交收款审批' : '正在创建支付订单';
      try {
	        if (requiresApproval) {
	          const approval = await requestHighRiskApproval(
	            'payment.order.create', payload, paymentRemarkInput.value.trim() || '复核租户、套餐、服务期和应收金额',
	            { idempotencyKey: 'payment-order:' + idempotencyKey },
	          );
	          statusEl.textContent = '收款订单审批已提交：' + fmt(approval.requestNo || '审批单');
	          return;
	        }
        const res = await fetch('/dashboard/saasAdmin/paymentOrder', {
          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()), body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        const order = data.order || {};
        paymentOrderNoInput.value = order.orderNo || paymentOrderNoInput.value;
        statusEl.textContent = data.idempotent ? '该支付订单已存在' : '支付订单已创建：' + fmt(order.orderNo);
        await loadPaymentOrders();
        loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function cancelPaymentOrder(button) {
      const orderNo = button.dataset.orderNo || '';
      const expectedVersion = Number(button.dataset.version || 0);
      const reason = window.prompt('请输入取消原因', '平台管理员取消');
      if (reason === null) return;
      statusEl.textContent = '正在取消支付订单';
      try {
        const res = await fetch('/dashboard/saasAdmin/paymentOrderCancel', {
          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ orderNo, expectedVersion, reason: reason.trim() }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '支付订单已取消：' + orderNo;
        await loadPaymentOrders();
        loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function runPaymentDunning(dryRun) {
      if (!dryRun && !window.confirm('确认对当前到期失败订单执行催缴？')) return;
      statusEl.textContent = dryRun ? '正在预览支付催缴' : '正在执行支付催缴';
      try {
        const res = await fetch('/dashboard/saasAdmin/paymentDunning', {
          method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ limit: 100, dryRun: Boolean(dryRun), retryDelaySeconds: 86400, notificationMaxAttempts: 5 }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        statusEl.textContent = (dryRun ? '催缴预览' : '催缴完成') + '：命中 ' + fmt(data.matchedCount || 0) + '，入队 ' + fmt(data.enqueuedCount || 0) + '，耗尽 ' + fmt(data.exhaustedCount || 0) + '，失败 ' + fmt(data.failedCount || 0);
        if (!dryRun) {
          await loadPaymentOrders();
          loadNotifications();
          loadOperations();
        }
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    function renderMetrics(items) {
      document.getElementById('metricCount').textContent = items.length + ' 项';
      if (!items.length) {
        metricsEl.innerHTML = '<tr><td colspan="3" class="empty">暂无数据</td></tr>';
        return;
      }
      metricsEl.innerHTML = items.map(item => {
        const used = item.limit > 0 ? esc(item.current) + '/' + esc(item.limit) + ' ' + esc(pct(item.usageRatio)) : esc(item.current) + '/不限';
        return '<tr>' +
          '<td>' + esc(item.label) + '</td>' +
          '<td>' + used + '</td>' +
          '<td>' + (item.openAlertCount > 0 ? pill(item.openAlertCount, 'warning') : pill('0', 'ok')) + '</td>' +
        '</tr>';
      }).join('');
    }
    function renderAlerts(items, summary, returnedCount) {
      summary = summary || {};
      const shown = returnedCount === undefined ? items.length : returnedCount;
      const total = summary.alertCount === undefined ? items.length : summary.alertCount;
      document.getElementById('alertCount').textContent = '命中 ' + total + ' 条 / 显示 ' + shown + ' 条 / 打开 ' + (summary.openCount || 0) + ' / 已解决 ' + (summary.resolvedCount || 0) + ' / 严重 ' + (summary.criticalCount || 0);
      if (!items.length) {
        alertsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      alertsEl.innerHTML = items.map(item => {
        const status = item.status === 'open' ? pill('打开', 'warning') : pill('已解决', 'ok');
        const used = item.limitValue > 0 ? esc(item.currentValue) + '/' + esc(item.limitValue) : esc(item.currentValue) + '/不限';
        const action = item.status === 'open'
          ? '<button type="button" data-action="resolve-alert" data-tenant-id="' + esc(item.tenantId) + '" data-metric="' + esc(item.metric) + '" data-alert-type="' + esc(item.alertType || 'quota_exceeded') + '" data-period-key="' + esc(item.periodKey || 'lifetime') + '">解决</button>'
          : '-';
        return '<tr>' +
          '<td>' + esc(item.lastSeenAt || item.createdAt || '-') + '<br><span class="subtitle">' + esc(item.source || '-') + '</span></td>' +
          '<td>ID ' + esc(item.tenantId) + '<br><span class="subtitle">' + esc(item.alertKey || '-') + '</span></td>' +
          '<td>' + esc(item.metricLabel || item.metric) + '<br><span class="subtitle">' + esc(item.metric) + ' / ' + esc(item.alertType) + '</span></td>' +
          '<td>' + status + '<br><span class="subtitle">触发 ' + esc(item.occurrenceCount || 0) + ' 次</span></td>' +
          '<td>' + used + '<br><span class="subtitle">' + esc(item.message || '-') + '</span></td>' +
          '<td>' + action + '</td>' +
        '</tr>';
      }).join('');
    }
    function renderNotificationWebhookSecurity(security) {
      security = security || {};
      const transport = security.requireHttps ? pill('仅 HTTPS', 'ok') : pill('允许 HTTP', 'warning');
      const networks = security.privateNetworksBlocked && security.metadataAddressesBlocked ? pill('私网/元数据已阻断', 'ok') : pill('网络边界异常', 'danger');
      const exceptions = Number(security.allowedCidrCount || 0) > 0 ? pill('例外 ' + esc(security.allowedCidrCount) + ' 段', 'warning') : pill('无 CIDR 例外', 'ok');
      notificationPolicySecurityEl.innerHTML = transport + ' ' + networks + ' ' + exceptions + '<br><span class="subtitle">DNS 绑定 / 同源重定向 / 直连出站</span>';
    }
    function renderNotificationCredentialProtection(protection) {
      notificationCredentialProtection = protection || {};
      if (!notificationCredentialProtection.supported) {
        notificationCredentialProtectionEl.innerHTML = pill('运行时不支持', 'danger');
		applyNotificationCredentialRotationControl();
        return;
      }
      const configured = notificationCredentialProtection.encryptionConfigured ? pill('加密已启用', 'ok') : pill('加密未启用', 'danger');
      const required = notificationCredentialProtection.requireEncryption ? pill('强制加密', 'ok') : pill('兼容明文写入', 'warning');
      const dedicated = notificationCredentialProtection.dedicatedConfigured ? pill('专用密钥', 'ok') : pill('共享密钥回退', 'warning');
      const healthy = notificationCredentialProtection.healthy ? pill('数据健康', 'ok') : pill('需要处理', 'danger');
      const unavailable = Number(notificationCredentialProtection.unavailableKeyCount || 0) + Number(notificationCredentialProtection.decryptFailureCount || 0);
      const keyDetail = '活动 Key ' + esc(notificationCredentialProtection.activeKeyId || '-') + ' / 密钥 ' + esc(notificationCredentialProtection.keyCount || 0) + ' 个';
      const dataDetail = '凭据 ' + esc(notificationCredentialProtection.configuredCredentialCount || 0) + ' / 密文 ' + esc(notificationCredentialProtection.encryptedCredentialCount || 0) + ' / 旧明文 ' + esc(notificationCredentialProtection.legacyPlaintextCount || 0) + ' / 待轮换 ' + esc(notificationCredentialProtection.rotationRequiredCount || 0) + ' / 不可用 ' + esc(unavailable);
      const missingKeys = Array.isArray(notificationCredentialProtection.unavailableKeyIds) && notificationCredentialProtection.unavailableKeyIds.length
        ? '<br><span class="subtitle">缺失 Key：' + esc(notificationCredentialProtection.unavailableKeyIds.join(', ')) + '</span>'
        : '';
      notificationCredentialProtectionEl.innerHTML = configured + ' ' + required + ' ' + dedicated + ' ' + healthy + '<br><span class="subtitle">' + keyDetail + '<br>' + dataDetail + '</span>' + missingKeys;
	  applyNotificationCredentialRotationControl();
	}
	function applyNotificationCredentialRotationControl() {
      const canManage = hasPlatformPermission('platform.notifications.manage');
	  const supported = Boolean(notificationCredentialProtection.supported);
	  const canRotate = supported && Boolean(notificationCredentialProtection.rotationAvailable) && Number(notificationCredentialProtection.rotationRequiredCount || 0) > 0;
      rotateNotificationCredentialsButton.disabled = !canManage || !canRotate;
	  rotateNotificationCredentialsButton.title = !canManage ? '缺少通知处置权限' : (!supported ? '运行时不支持凭据轮换' : (!notificationCredentialProtection.rotationAvailable ? '未配置可用加密密钥' : (canRotate ? '将旧明文或旧 Key 凭据改写到活动 Key' : '没有待轮换凭据')));
    }
    function renderNotificationPolicies(items, summary, returnedCount) {
      summary = summary || {};
      const shown = returnedCount === undefined ? items.length : returnedCount;
      notificationPolicyCountEl.textContent = '租户 ' + (summary.tenantCount || 0) + ' / 已配置 ' + (summary.configuredCount || 0) + ' / 启用 ' + (summary.enabledCount || 0) + ' / 停用 ' + (summary.disabledCount || 0) + ' / 未配置 ' + (summary.unconfiguredCount || 0) + ' / 显示 ' + shown;
      if (!items.length) {
        notificationPoliciesEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      notificationPoliciesEl.innerHTML = items.map(item => {
        const setting = item.setting || {};
        const urlSecurity = setting.urlSecurity || {};
        const state = !item.configured ? pill('未配置', 'warning') : (setting.enabled ? pill('已启用', 'ok') : pill('已停用', 'warning'));
        const webhook = setting.webhookUrl
          ? esc(setting.webhookUrl) + '<br>' + (urlSecurity.allowed ? pill('安全策略允许', 'ok') : pill('安全策略阻断', 'danger')) + ' ' + (setting.webhookCredentialProtection === 'encrypted' ? pill('凭据已加密', 'ok') : pill('旧明文', 'warning')) + '<br><span class="subtitle">Secret ' + (setting.webhookSecretConfigured ? '已配置' : '未配置') + (setting.webhookCredentialKeyId ? ' / Key ' + esc(setting.webhookCredentialKeyId) : '') + (urlSecurity.error ? ' / ' + esc(urlSecurity.error) : '') + '</span>'
          : '<span class="subtitle">未配置 URL</span>';
        return '<tr>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + ' / ' + (Number(item.tenantStatus) === 1 ? '正常' : '停用') + '</span></td>' +
          '<td>' + esc(item.packageName || item.packageCode || '未开套餐') + '</td>' +
          '<td>' + state + '<br><span class="subtitle">' + esc(setting.minimumSeverity || 'warning') + ' / 每小时 ' + esc(setting.hourlyLimit || '不限') + '</span><br><span class="subtitle">HTTP ' + esc(setting.webhookRetryAttempts || 1) + ' / Outbox ' + esc(setting.notificationMaxAttempts || 3) + '</span></td>' +
          '<td>' + webhook + '</td>' +
          '<td>' + esc(setting.updatedAt || '-') + '</td>' +
          '<td><button type="button" data-action="edit-notification-policy" data-tenant-id="' + esc(item.tenantId) + '">编辑</button></td>' +
        '</tr>';
      }).join('');
    }
    function renderNotificationHealth(data) {
      data = data || {};
      const summary = data.summary || {};
      const items = data.tenants || [];
      const rate = Number(summary.attemptedCount || 0) > 0 ? Math.round(Number(summary.deliverySuccessRate || 0) * 1000) / 10 + '%' : '-';
      notificationHealthCountEl.textContent = '租户 ' + (summary.tenantCount || 0) + ' / 命中 ' + (summary.matchedTenantCount || 0) + ' / 严重 ' + (summary.criticalTenantCount || 0) + ' / 预警 ' + (summary.warningTenantCount || 0) + ' / 健康 ' + (summary.healthyTenantCount || 0) + ' / 无数据 ' + (summary.noDataTenantCount || 0) + ' / 送达率 ' + rate + ' / 积压 ' + (summary.stalePendingCount || 0);
      if (!items.length) {
        notificationHealthTenantsEl.innerHTML = '<tr><td colspan="8" class="empty">暂无数据</td></tr>';
      } else {
        notificationHealthTenantsEl.innerHTML = items.map(item => {
          const state = item.healthState === 'critical' ? pill('严重', 'danger') : (item.healthState === 'warning' ? pill('预警', 'warning') : (item.healthState === 'healthy' ? pill('健康', 'ok') : pill('无数据', '')));
          const successRate = Number(item.attemptedCount || 0) > 0 ? Math.round(Number(item.deliverySuccessRate || 0) * 1000) / 10 + '%' : '-';
          const policy = item.policyConfigured ? (item.policyEnabled ? pill('已启用', 'ok') : pill('已停用', 'warning')) : pill('未配置', '');
          const reasons = Array.isArray(item.reasons) && item.reasons.length ? item.reasons.join('；') : '-';
          return '<tr>' +
            '<td>' + state + '<br><span class="subtitle">' + esc(reasons) + '</span></td>' +
            '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + ' / ' + esc(item.packageName || item.packageCode || '未开套餐') + '</span></td>' +
            '<td>' + policy + '</td>' +
            '<td>' + esc(successRate) + '<br><span class="subtitle">尝试 ' + esc(item.attemptedCount || 0) + ' / 总尝试 ' + esc(item.totalAttempts || 0) + '</span></td>' +
            '<td>送达 ' + esc(item.deliveredCount || 0) + ' / 失败 ' + esc(item.failedCount || 0) + ' / 耗尽 ' + esc(item.deadCount || 0) + '<br><span class="subtitle">抑制 ' + esc(item.suppressedCount || 0) + ' / 关闭 ' + esc(item.closedCount || 0) + '</span></td>' +
            '<td>待投 ' + esc(item.readyPendingCount || 0) + ' / 延期 ' + esc(item.deferredCount || 0) + '<br><span class="subtitle">积压 ' + esc(item.stalePendingCount || 0) + ' / 最早 ' + esc(item.oldestPendingAt || '-') + '</span></td>' +
            '<td>平均 ' + esc(item.averageDeliverySeconds || 0) + 's / 最大 ' + esc(item.maxDeliverySeconds || 0) + 's</td>' +
            '<td>送达 ' + esc(item.lastDeliveredAt || '-') + '<br><span class="subtitle">失败 ' + esc(item.lastFailureAt || '-') + '</span></td>' +
          '</tr>';
        }).join('');
      }
      const failureReasons = data.failureReasons || [];
      notificationFailureReasonCountEl.textContent = failureReasons.length + ' 项';
      if (!failureReasons.length) {
        notificationFailureReasonsEl.innerHTML = '<tr><td colspan="4" class="empty">暂无失败原因</td></tr>';
      } else {
        notificationFailureReasonsEl.innerHTML = failureReasons.map(item => '<tr>' +
          '<td>' + esc(item.reason || '-') + '</td>' +
          '<td>' + esc(item.count || 0) + '</td>' +
          '<td>' + esc(item.tenantCount || 0) + '</td>' +
          '<td>' + esc(item.lastOccurredAt || '-') + '</td>' +
        '</tr>').join('');
      }
    }
    function notificationSloRate(value, denominator) {
      if (Number(denominator || 0) <= 0) return '-';
      return (Math.round(Number(value || 0) * 1000) / 10) + '%';
    }
    function notificationSloState(state) {
      if (state === 'breached') return pill('违约', 'danger');
      if (state === 'met') return pill('达标', 'ok');
      return pill('无数据', '');
    }
    function renderNotificationSlo(data) {
      data = data || {};
      const summary = data.summary || {};
      const objectives = data.objectives || {};
      const days = data.days || [];
      const tenants = data.tenants || [];
      const successRate = notificationSloRate(summary.deliverySuccessRate, summary.attemptedCount);
      const latencyRate = notificationSloRate(summary.latencyAttainmentRate, summary.deliveredCount);
      const successTarget = Math.round(Number(objectives.successRateTarget || 0) * 1000) / 10;
      const latencyTarget = Math.round(Number(objectives.latencyRateTarget || 0) * 1000) / 10;
      notificationSloCountEl.innerHTML = notificationSloState(summary.sloState) + ' / 日达标 ' + esc(summary.metDayCount || 0) + ' / 违约 ' + esc(summary.breachedDayCount || 0) + ' / 无数据 ' + esc(summary.noDataDayCount || 0) + ' / 总成功率 ' + esc(successRate) + ' / 时延达标率 ' + esc(latencyRate) + ' / 目标 ' + esc(successTarget) + '%，' + esc(objectives.latencySecondsTarget || 0) + 's 内 ' + esc(latencyTarget) + '%';
      notificationSloTenantCountEl.textContent = '租户 ' + (summary.tenantCount || 0) + ' / 达标 ' + (summary.metTenantCount || 0) + ' / 违约 ' + (summary.breachedTenantCount || 0) + ' / 无数据 ' + (summary.noDataTenantCount || 0) + ' / 显示 ' + tenants.length;
      if (!days.length) {
        notificationSloDaysEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
      } else {
        notificationSloDaysEl.innerHTML = days.map(item => '<tr>' +
          '<td>' + esc(item.day || '-') + '<br><span class="subtitle">租户 ' + esc(item.tenantCount || 0) + '</span></td>' +
          '<td>' + notificationSloState(item.sloState) + '</td>' +
          '<td>' + esc(notificationSloRate(item.deliverySuccessRate, item.attemptedCount)) + '<br><span class="subtitle">送达 ' + esc(item.deliveredCount || 0) + ' / 尝试 ' + esc(item.attemptedCount || 0) + '</span></td>' +
          '<td>' + esc(notificationSloRate(item.latencyAttainmentRate, item.deliveredCount)) + '<br><span class="subtitle">目标内 ' + esc(item.deliveredWithinTarget || 0) + ' / 送达 ' + esc(item.deliveredCount || 0) + '</span></td>' +
          '<td>失败 ' + esc(item.failedCount || 0) + ' / 耗尽 ' + esc(item.deadCount || 0) + ' / 关闭 ' + esc(item.closedCount || 0) + '<br><span class="subtitle">待投 ' + esc(item.pendingCount || 0) + ' / 抑制 ' + esc(item.suppressedCount || 0) + '</span></td>' +
          '<td>平均 ' + esc(item.averageDeliverySeconds || 0) + 's / 最大 ' + esc(item.maxDeliverySeconds || 0) + 's</td>' +
        '</tr>').join('');
      }
      if (!tenants.length) {
        notificationSloTenantsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无数据</td></tr>';
      } else {
        notificationSloTenantsEl.innerHTML = tenants.map(item => '<tr>' +
          '<td>' + notificationSloState(item.sloState) + '</td>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId) + ' / ' + esc(item.packageName || item.packageCode || '未开套餐') + '</span></td>' +
          '<td>' + esc(notificationSloRate(item.deliverySuccessRate, item.attemptedCount)) + '<br><span class="subtitle">送达 ' + esc(item.deliveredCount || 0) + ' / 尝试 ' + esc(item.attemptedCount || 0) + '</span></td>' +
          '<td>' + esc(notificationSloRate(item.latencyAttainmentRate, item.deliveredCount)) + '<br><span class="subtitle">目标内 ' + esc(item.deliveredWithinTarget || 0) + '</span></td>' +
          '<td>失败 ' + esc(item.failedCount || 0) + ' / 耗尽 ' + esc(item.deadCount || 0) + ' / 关闭 ' + esc(item.closedCount || 0) + '</td>' +
          '<td>待投 ' + esc(item.pendingCount || 0) + ' / 抑制 ' + esc(item.suppressedCount || 0) + '</td>' +
          '<td>平均 ' + esc(item.averageDeliverySeconds || 0) + 's / 最大 ' + esc(item.maxDeliverySeconds || 0) + 's</td>' +
        '</tr>').join('');
      }
    }
    function fillNotificationPolicy(policy) {
      const setting = (policy || {}).setting || {};
      notificationPolicyLoadedTenantId = Number(policy && policy.tenantId ? policy.tenantId : 0);
      notificationPolicyTenantInput.value = policy && policy.tenantId ? policy.tenantId : '';
      notificationPolicyEnabledInput.checked = Boolean(setting.enabled);
      notificationPolicyWebhookUrlInput.value = setting.webhookUrl || '';
      notificationPolicyWebhookSecretInput.value = '';
      notificationPolicyWebhookSecretInput.placeholder = setting.webhookSecretConfigured ? '已配置，留空保持不变' : '留空不配置';
      notificationPolicyClearSecretInput.checked = false;
      notificationPolicyTimeoutInput.value = setting.webhookTimeoutSeconds || 5;
      notificationPolicyHttpAttemptsInput.value = setting.webhookRetryAttempts || 1;
      notificationPolicyHttpDelayInput.value = setting.webhookRetryDelayMs === undefined ? 250 : setting.webhookRetryDelayMs;
      notificationPolicyMaxAttemptsInput.value = setting.notificationMaxAttempts || 3;
      notificationPolicyRetryDelayInput.value = setting.notificationRetryDelaySeconds === undefined ? 300 : setting.notificationRetryDelaySeconds;
      notificationPolicyMinimumSeverityInput.value = setting.minimumSeverity || 'warning';
      const alertTypes = Array.isArray(setting.alertTypes) ? setting.alertTypes : [];
      const allTypes = alertTypes.length === 0;
      notificationPolicyAlertQuotaInput.checked = allTypes || alertTypes.includes('quota_exceeded');
      notificationPolicyAlertRenewalInput.checked = allTypes || alertTypes.includes('tenant_renewal_reminder');
      notificationPolicyAlertPaymentFailedInput.checked = allTypes || alertTypes.includes('payment_failed_reminder');
      notificationPolicyAlertTaskSlaInput.checked = allTypes || alertTypes.includes('admin_task_sla_reminder');
      notificationPolicyAlertOperationQueueInput.checked = allTypes || alertTypes.includes('operation_queue_assignment_reminder');
      notificationPolicyAlertOtherInput.value = alertTypes.filter(value => !['quota_exceeded', 'tenant_renewal_reminder', 'payment_failed_reminder', 'admin_task_sla_reminder', 'operation_queue_assignment_reminder'].includes(value)).join(', ');
      notificationPolicyQuietEnabledInput.checked = Boolean(setting.quietHoursEnabled);
      notificationPolicyQuietStartInput.value = setting.quietHoursStart || '22:00';
      notificationPolicyQuietEndInput.value = setting.quietHoursEnd || '08:00';
      notificationPolicyTimezoneInput.value = setting.timezone || 'Asia/Shanghai';
      notificationPolicyHourlyLimitInput.value = setting.hourlyLimit === undefined ? 0 : setting.hourlyLimit;
      notificationPolicyTitleTemplateInput.value = setting.webhookTitleTemplate || '';
      notificationPolicyBodyTemplateInput.value = setting.webhookBodyTemplate || '';
    }
    function selectedNotificationPolicyAlertTypes() {
      const values = [];
      if (notificationPolicyAlertQuotaInput.checked) values.push('quota_exceeded');
      if (notificationPolicyAlertRenewalInput.checked) values.push('tenant_renewal_reminder');
      if (notificationPolicyAlertPaymentFailedInput.checked) values.push('payment_failed_reminder');
      if (notificationPolicyAlertTaskSlaInput.checked) values.push('admin_task_sla_reminder');
      if (notificationPolicyAlertOperationQueueInput.checked) values.push('operation_queue_assignment_reminder');
      notificationPolicyAlertOtherInput.value.split(',').forEach(value => {
        value = value.trim();
        if (value && !values.includes(value)) values.push(value);
      });
      if (values.length === 5 && !notificationPolicyAlertOtherInput.value.trim()) return [];
      return values;
    }
    function renderNotifications(items, summary, returnedCount) {
      summary = summary || {};
      const shown = returnedCount === undefined ? items.length : returnedCount;
      const total = summary.notificationCount === undefined ? items.length : summary.notificationCount;
      document.getElementById('notificationCount').textContent = '命中 ' + total + ' 条 / 显示 ' + shown + ' 条 / 待发 ' + (summary.pendingCount || 0) + ' / 失败 ' + (summary.failedCount || 0) + ' / 耗尽 ' + (summary.deadCount || 0) + ' / 关闭 ' + (summary.closedCount || 0) + ' / 过滤 ' + (summary.suppressedCount || 0);
      if (!items.length) {
        notificationsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      notificationsEl.innerHTML = items.map(item => {
        const retryable = item.status === 'failed' || item.status === 'dead';
        const closable = item.status === 'pending' || item.status === 'failed';
        const status = item.status === 'delivered' ? pill('已送达', 'ok') : (item.status === 'suppressed' ? pill('策略过滤', '') : (item.status === 'closed' ? pill('已关闭', '') : (item.status === 'dead' ? pill('已耗尽', 'danger') : (item.status === 'failed' ? pill('失败', 'warning') : pill('待发送', 'warning')))));
        const used = item.limitValue > 0 ? esc(item.currentValue) + '/' + esc(item.limitValue) : esc(item.currentValue || '-') + '/不限';
        const actions = [];
        if (retryable) actions.push('<button type="button" data-action="retry-notification" data-notification-id="' + esc(item.id) + '">重试</button>');
        if (closable) actions.push('<button type="button" data-action="close-notification" data-notification-id="' + esc(item.id) + '">关闭</button>');
        const action = actions.length ? actions.join(' ') : '-';
        return '<tr>' +
          '<td>' + esc(item.updatedAt || item.createdAt || '-') + '<br><span class="subtitle">' + esc(item.channel || '-') + '</span></td>' +
          '<td>ID ' + esc(item.tenantId) + '<br><span class="subtitle">' + esc(item.notificationKey || '-') + '</span></td>' +
          '<td>' + esc(item.metricLabel || item.metric || '-') + '<br><span class="subtitle">' + esc(item.alertType || '-') + ' ' + esc(used) + '</span></td>' +
          '<td>' + status + '<br><span class="subtitle">' + esc(item.attempts || 0) + '/' + esc(item.maxAttempts || 0) + '</span></td>' +
          '<td>' + esc(item.lastError || '-') + '<br><span class="subtitle">' + esc(item.nextRetryAt || item.deliveredAt || '-') + '</span></td>' +
          '<td>' + action + '</td>' +
        '</tr>';
      }).join('');
    }
    function operationJSONText(value) {
      if (value === null || value === undefined || value === '') return '';
      if (typeof value === 'string') return value;
      try {
        return JSON.stringify(value, null, 2);
      } catch (_) {
        return String(value);
      }
    }
    function operationChangeDetails(item) {
      const beforeText = operationJSONText(item.before);
      const afterText = operationJSONText(item.after);
      if (!beforeText && !afterText) return '<span class="subtitle">无变更 JSON</span>';
      const beforeBlock = beforeText
        ? '<div class="subtitle">Before</div><pre>' + esc(beforeText) + '</pre>'
        : '<div class="subtitle">Before：空</div>';
      const afterBlock = afterText
        ? '<div class="subtitle">After</div><pre>' + esc(afterText) + '</pre>'
        : '<div class="subtitle">After：空</div>';
      return '<details><summary>查看变更</summary>' + beforeBlock + afterBlock + '</details>';
    }
    function renderOperations(items, summary) {
      summary = summary || {};
      const count = summary.operationCount !== undefined ? Number(summary.operationCount || 0) : items.length;
      const tenantCount = Number(summary.tenantCount || 0);
      const actionCount = Number(summary.actionCount || 0);
      const actorCount = Number(summary.actorUserCount || 0);
      const shown = items.length < count ? ' / 显示 ' + items.length + ' 条' : '';
      document.getElementById('operationCount').textContent = count + ' 条 / 租户 ' + tenantCount + ' 个 / 动作 ' + actionCount + ' 类 / 操作人 ' + actorCount + ' 个' + shown;
      if (!items.length) {
        operationsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      operationsEl.innerHTML = items.map(item => {
        return '<tr>' +
          '<td>' + esc(item.createdAt) + '</td>' +
          '<td>' + esc(item.action) + '</td>' +
          '<td>' + esc(item.targetName || item.targetId) + '<br><span class="subtitle">' + esc(item.targetType) + ' ' + esc(item.targetId) + '</span></td>' +
          '<td>' + esc(item.actorUserId) + '<br><span class="subtitle">租户 ' + esc(item.actorTenantId) + '</span></td>' +
          '<td>' + esc(item.remark) + '</td>' +
          '<td>' + operationChangeDetails(item) + '</td>' +
        '</tr>';
      }).join('');
    }
	function renderBillingEvents(items, summary) {
		summary = summary || {};
		const count = summary.eventCount !== undefined ? Number(summary.eventCount || 0) : items.length;
		const renewalCount = Number(summary.renewalCount || 0);
		const amount = (Number(summary.amountCents || 0) / 100).toFixed(2);
		const shown = items.length < count ? ' / 显示 ' + items.length + ' 条' : '';
		const refundCount = Number(summary.refundCount || 0);
		const gross = (Number(summary.grossAmountCents || 0) / 100).toFixed(2);
		const refunded = (Number(summary.refundAmountCents || 0) / 100).toFixed(2);
		document.getElementById('billingCount').textContent = count + ' 条 / 续费 ' + renewalCount + ' / 退款 ' + refundCount + ' / 毛收入 ' + gross + ' / 退款 ' + refunded + ' / 净收入 ' + amount + ' CNY' + shown;
		if (!items.length) {
			billingEventsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无数据</td></tr>';
			return;
      }
      billingEventsEl.innerHTML = items.map(item => {
		const signedAmount = item.eventType === 'refund' ? -Number(item.amountCents || 0) : Number(item.amountCents || 0);
		const amount = signedAmount !== 0 ? (signedAmount / 100).toFixed(2) + ' ' + esc(item.currency || 'CNY') : '-';
        return '<tr>' +
          '<td>' + esc(item.createdAt) + '</td>' +
          '<td>ID ' + esc(item.tenantId) + '<br><span class="subtitle">' + esc(item.eventType) + '</span></td>' +
          '<td>' + esc(item.packageName || item.packageCode) + '</td>' +
          '<td>' + esc(item.previousExpiresAt || '-') + '<br><span class="subtitle">' + esc(item.newExpiresAt || '-') + '</span></td>' +
          '<td>' + esc(amount) + '<br><span class="subtitle">' + esc(item.externalOrderNo || item.paymentMethod || '-') + '</span></td>' +
        '</tr>';
      }).join('');
    }
    function billingReconciliationReasonText(reason) {
      const labels = {
        missing_package: '无当前套餐',
        inactive_package: '当前套餐未启用',
        package_mismatch: '套餐不一致',
        expires_mismatch: '到期未覆盖'
      };
      return labels[reason] || reason;
    }
    function renderBillingReconciliation(items, summary) {
      summary = summary || {};
      const checked = Number(summary.checkedCount || 0);
      const mismatched = Number(summary.mismatchedCount || 0);
      const matched = Number(summary.matchedCount || 0);
      const shown = items.length < checked ? ' / 显示 ' + items.length + ' 条' : '';
      document.getElementById('billingReconciliationCount').textContent = checked + ' 条 / 正常 ' + matched + ' 条 / 异常 ' + mismatched + ' 条' + shown;
      if (!items.length) {
        billingReconciliationEl.innerHTML = '<tr><td colspan="6" class="empty">暂无数据</td></tr>';
        return;
      }
      billingReconciliationEl.innerHTML = items.map(item => {
        const amount = Number(item.amountCents || 0) > 0 ? (Number(item.amountCents) / 100).toFixed(2) + ' ' + esc(item.currency || 'CNY') : '-';
        const reasons = (item.mismatchReasons || []).map(billingReconciliationReasonText).join('、');
        const status = item.reconcileStatus === 'matched' ? '正常' : '异常';
        const statusClass = item.reconcileStatus === 'matched' ? 'ok' : 'bad';
        const currentPackage = item.currentPackageFound
          ? esc(item.currentPackageName || item.currentPackageCode) + '<br><span class="subtitle">' + esc(item.currentExpiresAt || '长期有效') + ' / 状态 ' + esc(item.currentPackageStatus) + '</span>'
          : '<span class="bad">无当前套餐</span>';
        const action = item.reconcileStatus === 'matched'
          ? '<span class="subtitle">无需处理</span>'
          : '<button type="button" data-action="follow-up-billing-reconciliation" data-billing-event-id="' + esc(item.id) + '">跟进</button>';
        return '<tr>' +
          '<td>#' + esc(item.id) + '<br><span class="subtitle">' + esc(item.externalOrderNo || item.paymentMethod || '-') + ' / ' + esc(amount) + '</span></td>' +
          '<td>ID ' + esc(item.tenantId) + '<br><span class="subtitle">' + esc(item.tenantName || '-') + '</span></td>' +
          '<td>' + esc(item.packageName || item.packageCode) + '<br><span class="subtitle">' + esc(item.newExpiresAt || '长期有效') + '</span></td>' +
          '<td>' + currentPackage + '</td>' +
          '<td><span class="' + statusClass + '">' + status + '</span><br><span class="subtitle">' + esc(reasons || '已覆盖') + '</span></td>' +
          '<td>' + action + '</td>' +
        '</tr>';
      }).join('');
    }
    function renderBillingReconciliationFollowUps(items, summary) {
      const total = summary && summary.totalCount !== undefined ? Number(summary.totalCount) : items.length;
      const overdue = summary && summary.overdueCount !== undefined ? Number(summary.overdueCount) : items.filter(item => item.overdue).length;
      const closed = summary && summary.closedCount !== undefined ? Number(summary.closedCount) : 0;
      let text = total + ' 条任务';
      if (overdue > 0) text += '，逾期 ' + overdue;
      if (closed > 0) text += '，已关闭 ' + closed;
      document.getElementById('billingFollowUpCount').textContent = text;
      if (!items.length) {
        billingFollowUpsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无数据</td></tr>';
        return;
      }
      billingFollowUpsEl.innerHTML = items.map(item => {
        const status = riskFollowStatusText(item.status);
        const due = riskTaskDueStateText(item.dueState);
        const amount = Number(item.amountCents || 0) > 0 ? (Number(item.amountCents) / 100).toFixed(2) + ' ' + esc(item.currency || 'CNY') : '-';
        const daysText = item.nextFollowUpAt ? (item.daysUntil === 0 ? '今天' : (item.daysUntil > 0 ? item.daysUntil + ' 天后' : Math.abs(item.daysUntil) + ' 天前')) : '-';
        return '<tr>' +
          '<td>#' + esc(item.billingEventId || '-') + '<br><span class="subtitle">' + esc(item.externalOrderNo || '-') + ' / ' + esc(amount) + '</span></td>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId || '-') + '</span></td>' +
          '<td>' + pill(status[0], status[1]) + '<br><span class="subtitle">' + pill(due[0], due[1]) + '</span></td>' +
          '<td>' + esc(item.owner || '-') + '</td>' +
          '<td>' + esc(item.nextFollowUpAt || '-') + '<br><span class="subtitle">' + esc(daysText) + '</span></td>' +
          '<td>' + esc(item.remark || '-') + '<br><span class="subtitle">' + esc(item.packageName || item.packageCode || '-') + '</span></td>' +
          '<td>' + esc(item.createdAt || '-') + '<br><span class="subtitle">操作 ' + esc(item.operationId || '-') + '</span></td>' +
        '</tr>';
      }).join('');
    }
    function renderBillingReconciliationFollowUpOwners(items, summary) {
      const total = summary && summary.totalCount !== undefined ? Number(summary.totalCount) : 0;
      const ownerCount = items.length;
      document.getElementById('billingFollowOwnerCount').textContent = ownerCount ? (ownerCount + ' 个负责人，' + total + ' 条任务') : '';
      if (!items.length) {
        billingFollowUpOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">暂无数据</td></tr>';
        return;
      }
      billingFollowUpOwnersEl.innerHTML = items.map(item => {
        const overdue = Number(item.overdueCount || 0);
        const dueSoon = Number(item.dueSoonCount || 0);
        const open = Number(item.openCount || 0);
        const openPill = overdue > 0 ? pill(open + ' 打开', 'danger') : (dueSoon > 0 ? pill(open + ' 打开', 'warning') : pill(open + ' 打开', open > 0 ? '' : 'ok'));
        const statusParts = [
          '待 ' + fmt(item.pendingCount || 0),
          '联 ' + fmt(item.contactedCount || 0),
          '续 ' + fmt(item.renewalPendingCount || 0),
          '关 ' + fmt((item.resolvedCount || 0) + (item.ignoredCount || 0)),
        ];
        const dueParts = [
          '逾期 ' + fmt(overdue),
          '7 天 ' + fmt(dueSoon),
          '未来 ' + fmt(item.futureCount || 0),
          '无日期 ' + fmt(item.noDateCount || 0),
          '已关 ' + fmt(item.closedCount || 0),
        ];
        return '<tr>' +
          '<td>' + esc(item.owner || '未分配') + '</td>' +
          '<td>' + openPill + '<br><span class="subtitle">全部 ' + esc(item.totalCount || 0) + '</span></td>' +
          '<td>' + esc(statusParts.join(' / ')) + '</td>' +
          '<td>' + esc(dueParts.join(' / ')) + '</td>' +
          '<td>' + esc(item.latestFollowUpAt || '-') + '<br><span class="subtitle">下次 ' + esc(item.nextFollowUpAt || '-') + '</span></td>' +
        '</tr>';
      }).join('');
    }
    function renderPackages(items) {
      packageCache = {};
	      if (!items.length) {
	        provisionPackageCodeInput.innerHTML = '<option value="">暂无可用套餐</option>';
	        filterPackageCodeInput.innerHTML = '<option value="">全部套餐</option>';
	        adminTaskPackageCodeInput.innerHTML = '<option value="">全部套餐</option>';
	        billingPackageCodeInput.innerHTML = '<option value="">全部套餐</option>';
        packageCodeInput.innerHTML = '<option value="">暂无可用套餐</option>';
        syncPackageCodeInput.innerHTML = '<option value="">暂无可用套餐</option>';
        paymentPackageCodeInput.innerHTML = '<option value="">暂无可用套餐</option>';
        renewalPackageCodeInput.innerHTML = '<option value="">沿用当前套餐</option>';
        editPackageVersionInput.value = '0';
        clearPackageImpact('暂无套餐影响数据');
        clearPackageSync('暂无套餐同步数据');
        return;
      }
      items.forEach(item => { packageCache[item.code] = item; });
      const packageOptions = items.map(item => {
        const suffix = item.status === 1 ? '' : '（停用）';
        return '<option value="' + esc(item.code) + '"' + (item.status === 1 ? '' : ' disabled') + '>' + esc(item.name || item.code) + suffix + '</option>';
      }).join('');
      const filterOptions = items.map(item => {
        const suffix = item.status === 1 ? '' : '（停用）';
        return '<option value="' + esc(item.code) + '">' + esc(item.name || item.code) + suffix + '</option>';
      }).join('');
	      provisionPackageCodeInput.innerHTML = '<option value="">请选择套餐</option>' + packageOptions;
	      filterPackageCodeInput.innerHTML = '<option value="">全部套餐</option>' + filterOptions;
	      adminTaskPackageCodeInput.innerHTML = '<option value="">全部套餐</option>' + filterOptions;
	      billingPackageCodeInput.innerHTML = '<option value="">全部套餐</option>' + filterOptions;
      packageCodeInput.innerHTML = '<option value="">请选择套餐</option>' + packageOptions;
      syncPackageCodeInput.innerHTML = '<option value="">请选择套餐</option>' + packageOptions;
      paymentPackageCodeInput.innerHTML = '<option value="">请选择套餐</option>' + packageOptions;
      renewalPackageCodeInput.innerHTML = '<option value="">沿用当前套餐</option>' + packageOptions;
    }
    function fillPackageEditor(code) {
      const item = packageCache[code];
      if (!item) {
        editPackageVersionInput.value = '0';
        clearPackageImpact('新增套餐将在审批后生效');
        return;
      }
      editPackageCodeInput.value = item.code || '';
      editPackageNameInput.value = item.name || item.code || '';
      editPackageDescriptionInput.value = item.description || '';
      editPackageVersionInput.value = String(item.version || 0);
      editPackageStatusInput.value = String(item.status || 1);
      editPackageLimitsInput.value = JSON.stringify(item.limits || {}, null, 2);
      syncPackageCodeInput.value = item.code || '';
      clearPackageImpact('编辑并保存后显示影响');
    }
    function clearPackageImpact(message) {
      packageImpactSummaryEl.textContent = message || '保存套餐后显示影响';
      packageImpactChangesEl.innerHTML = '<tr><td colspan="5" class="empty">暂无变更</td></tr>';
    }
    function packageImpactDirectionLabel(direction) {
      switch (direction) {
        case 'increase': return '升额';
        case 'decrease': return '降额';
        case 'newly_limited': return '新增限额';
        case 'newly_unlimited': return '改为不限额';
        default: return direction || '-';
      }
    }
    function renderPackageImpact(impact) {
      if (!impact) {
        clearPackageImpact('保存套餐后显示影响');
        return;
      }
      const parts = [
        (impact.existing ? '编辑已有套餐' : '新增套餐'),
        '当前分配租户 ' + fmt(impact.assignedTenantCount || 0),
        '变更额度 ' + fmt(impact.changedLimitCount || 0),
        '降额 ' + fmt((impact.decreasedLimitCount || 0) + (impact.newlyLimitedCount || 0)),
        '超新额度租户 ' + fmt(impact.overLimitTenantCount || 0),
        impact.tenantSnapshotsUpdated ? '已同步租户快照' : '租户套餐快照未自动改写'
      ];
      if (impact.statusChanged) {
        parts.push('状态 ' + fmt(impact.beforeStatus || 0) + ' -> ' + fmt(impact.afterStatus || 0));
      }
      packageImpactSummaryEl.textContent = parts.join(' / ');
      const changes = impact.changes || [];
      if (!changes.length) {
        packageImpactChangesEl.innerHTML = '<tr><td colspan="5" class="empty">额度未变化</td></tr>';
        return;
      }
      packageImpactChangesEl.innerHTML = changes.map(item => {
        return '<tr>' +
          '<td>' + esc(item.label || item.metric || item.field) + '<br><span class="subtitle">' + esc(item.field || item.metric || '-') + '</span></td>' +
          '<td>' + esc(packageImpactDirectionLabel(item.direction)) + '</td>' +
          '<td>' + esc(item.before) + '</td>' +
          '<td>' + esc(item.after) + '</td>' +
          '<td>' + esc(Number(item.delta || 0) > 0 ? '+' + item.delta : item.delta) + '</td>' +
        '</tr>';
      }).join('');
    }
    function clearPackageSync(message) {
      packageSyncSummaryEl.textContent = message || '预览后显示命中租户';
      packageSyncTenantsEl.innerHTML = '<tr><td colspan="5" class="empty">暂无同步结果</td></tr>';
    }
    function packageSyncTenantStatus(item) {
      if (item.synced) return '已同步';
      if (item.skipped) return '已跳过';
      return '待同步';
    }
    function packageSyncOverLimitText(item) {
      const metrics = item.overLimitMetrics || [];
      if (!metrics.length) return '-';
      return metrics.map(metric => esc(metric.label || metric.metric) + ' ' + esc(metric.current) + '/' + esc(metric.limit)).join('<br>');
    }
    function renderPackageSyncResult(result) {
      if (!result) {
        clearPackageSync('预览后显示命中租户');
        return;
      }
      const parts = [
        result.dryRun ? '预览' : '应用',
        fmt(result.packageName || result.packageCode || '-'),
        '命中 ' + fmt(result.matchedTenantCount || 0),
        '检查 ' + fmt(result.checkedTenantCount || 0),
        '超额 ' + fmt(result.overLimitTenantCount || 0),
        '已同步 ' + fmt(result.syncedTenantCount || 0),
        '刷新指标 ' + fmt(result.metricsRefreshed || 0),
        result.tenantSnapshotsUpdated ? '租户快照已更新' : '租户快照未更新'
      ];
      if (result.blocked) {
        parts.push('已阻断');
      }
      packageSyncSummaryEl.textContent = parts.join(' / ');
      const tenants = result.tenants || [];
      if (!tenants.length) {
        packageSyncTenantsEl.innerHTML = '<tr><td colspan="5" class="empty">没有命中租户</td></tr>';
        return;
      }
      packageSyncTenantsEl.innerHTML = tenants.map(item => {
        return '<tr>' +
          '<td>' + esc(item.tenantName || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId || '-') + '</span></td>' +
          '<td>' + esc(item.expiresAt || '-') + '</td>' +
          '<td>' + esc(packageSyncTenantStatus(item)) + '</td>' +
          '<td>' + esc(item.metricsRefreshed || 0) + '</td>' +
          '<td>' + packageSyncOverLimitText(item) + '</td>' +
        '</tr>';
      }).join('');
    }
	    function packageSyncTaskStatusLabel(status) {
	      switch (status) {
	        case 'pending': return '待应用';
	        case 'blocked': return '已阻断';
	        case 'applied': return '已应用';
        case 'failed': return '失败';
        case 'canceled': return '已取消';
		        default: return status || '-';
	      }
	    }
	    function adminTaskTypeLabel(taskType) {
	      switch (taskType) {
	        case 'package_sync': return '套餐同步';
	        case 'tenant_provision': return '平台开户';
	        case 'tenant_renewal': return '租户续费';
	        default: return taskType || '-';
	      }
	    }
	    function adminTaskApplyEndpoint(taskType) {
	      switch (taskType) {
	        case 'package_sync': return '/dashboard/saasAdmin/packageSyncTaskApply';
	        case 'tenant_provision': return '/dashboard/saasAdmin/tenantProvisionTaskApply';
	        case 'tenant_renewal': return '/dashboard/saasAdmin/tenantRenewalTaskApply';
	        default: return '';
	      }
	    }
	    function adminTaskResultText(task) {
	      switch (task.taskType) {
	        case 'package_sync': return packageSyncTaskSummary(task);
	        case 'tenant_provision': return provisionTaskResultText(task);
	        case 'tenant_renewal': return renewalTaskResultText(task);
	        default: {
	          const result = task.result || task.preview || {};
	          const parts = [];
	          if (result.packageCode) parts.push('套餐 ' + fmt(result.packageCode));
	          if (result.tenantName) parts.push(fmt(result.tenantName));
	          if (result.metricsRefreshed !== undefined) parts.push('刷新 ' + fmt(result.metricsRefreshed));
	          if (task.lastError) parts.push(task.lastError);
	          return parts.length ? parts.join(' / ') : '-';
	        }
	      }
	    }
	    function adminTaskTenantText(task) {
	      const result = task.result || task.preview || {};
	      const tenantName = result.tenantName || '';
	      const tenantId = task.tenantId || result.tenantId || '';
	      const top = tenantName || (tenantId ? 'ID ' + tenantId : '全部租户');
	      return esc(top) + '<br><span class="subtitle">' + esc(task.packageCode || result.packageCode || '-') + '</span>';
	    }
	    function renderAdminTasks(tasks, summary, returnedCount) {
	      tasks = tasks || [];
	      summary = summary || {};
	      const total = summary.taskCount || 0;
	      const shown = returnedCount === undefined ? tasks.length : returnedCount;
	      const summaryText = '任务 ' + fmt(total) +
	        ' 条 / 待应用 ' + fmt(summary.pendingCount || 0) +
	        ' / 阻断 ' + fmt(summary.blockedCount || 0) +
	        ' / 失败 ' + fmt(summary.failedCount || 0) +
	        ' / 已应用 ' + fmt(summary.appliedCount || 0) +
	        ' / 可处理 ' + fmt(summary.actionableCount || 0) +
	        (total > shown ? ' / 显示 ' + fmt(shown) + ' 条' : '');
	      if (!tasks.length) {
	        adminTasksEl.innerHTML = '<tr><td colspan="6" class="empty">暂无运营任务</td></tr>';
	        adminTaskSummaryEl.textContent = summaryText;
	        return;
	      }
	      adminTaskSummaryEl.textContent = summaryText;
	      adminTasksEl.innerHTML = tasks.map(item => {
	        const endpoint = adminTaskApplyEndpoint(item.taskType);
	        const actions = [];
	        if (item.canApply && endpoint) {
	          const applyLabel =
	            (item.taskType === 'tenant_provision' && approvalActionRequired('tenant.provision', 0)) ||
	            (item.taskType === 'tenant_renewal' && approvalActionRequired('tenant.renewal', 0))
	              ? '提交审批'
	              : '应用';
	          actions.push('<button type="button" data-action="apply-admin-task" data-task-id="' + esc(item.id) + '" data-task-version="' + esc(item.version || 0) + '" data-task-type="' + esc(item.taskType) + '">' + applyLabel + '</button>');
	        }
	        if (item.canCancel) {
	          actions.push('<button type="button" data-action="cancel-admin-task" data-task-id="' + esc(item.id) + '">取消</button>');
	        }
	        if (item.status === 'blocked' || item.status === 'failed') {
	          actions.push('<button type="button" data-action="reset-admin-task" data-task-id="' + esc(item.id) + '">重置</button>');
	        }
	        actions.push('<button type="button" data-action="trace-admin-task" data-task-id="' + esc(item.id) + '">追溯</button>');
	        const action = actions.length ? actions.join(' ') : '-';
	        const timeText = item.lastError ? item.lastError : (item.appliedAt || item.updatedAt || item.createdAt || '-');
	        return '<tr>' +
	          '<td>#' + esc(item.id) + '<br><span class="subtitle">' + esc(item.createdAt || '-') + '</span></td>' +
	          '<td>' + esc(adminTaskTypeLabel(item.taskType)) + '<br><span class="subtitle">' + esc(packageSyncTaskStatusLabel(item.status)) + '</span></td>' +
	          '<td>' + adminTaskTenantText(item) + '</td>' +
	          '<td>' + esc(adminTaskResultText(item)) + '</td>' +
	          '<td>' + esc(timeText) + '</td>' +
	          '<td>' + action + '</td>' +
	        '</tr>';
	      }).join('');
	    }
		    function adminTaskParams(limit) {
		      const params = new URLSearchParams({ taskType: adminTaskTypeInput.value || 'all', limit: String(limit || 20) });
		      if (adminTaskStatusInput.value && adminTaskStatusInput.value !== 'all') params.set('status', adminTaskStatusInput.value);
		      if (adminTaskTenantInput.value.trim()) params.set('tenantId', adminTaskTenantInput.value.trim());
		      if (adminTaskPackageCodeInput.value.trim()) params.set('packageCode', adminTaskPackageCodeInput.value.trim());
	      return params;
	    }
	    function adminTaskSlaParams(limit) {
	      const params = adminTaskParams(limit);
	      if (adminTaskSlaWarningInput.value.trim()) params.set('warningHours', adminTaskSlaWarningInput.value.trim());
	      if (adminTaskSlaOverdueInput.value.trim()) params.set('overdueHours', adminTaskSlaOverdueInput.value.trim());
	      return params;
	    }
	    async function loadAdminTasks() {
	      try {
		        const res = await fetch('/dashboard/saasAdmin/tasks?' + adminTaskParams(20).toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        renderAdminTasks(data.tasks || [], data.summary || {}, data.returnedCount);
	        loadAdminTaskOwners();
	        loadAdminTaskSla();
	      } catch (err) {
	        adminTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看运营任务</td></tr>';
	        adminTaskSummaryEl.textContent = err.message || String(err);
	        adminTaskOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看任务负责人</td></tr>';
	        adminTaskOwnerSummaryEl.textContent = err.message || String(err);
	        adminTaskSlaEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看任务SLA</td></tr>';
	        adminTaskSlaSummaryEl.textContent = err.message || String(err);
	      }
	    }
	    function renderAdminTaskOwners(owners, summary, ownerCount, returnedCount, scannedTaskCount, partial) {
	      owners = owners || [];
	      summary = summary || {};
	      const shown = returnedCount === undefined ? owners.length : returnedCount;
	      const totalOwners = ownerCount === undefined ? owners.length : ownerCount;
	      const summaryText = '负责人 ' + fmt(totalOwners) +
	        ' 人 / 可处理 ' + fmt(summary.actionableCount || 0) +
	        ' / 阻断 ' + fmt(summary.blockedCount || 0) +
	        ' / 失败 ' + fmt(summary.failedCount || 0) +
	        ' / 扫描任务 ' + fmt(scannedTaskCount || 0) +
	        (totalOwners > shown ? ' / 显示 ' + fmt(shown) + ' 人' : '') +
	        (partial ? ' / 超出扫描上限' : '');
	      adminTaskOwnerSummaryEl.textContent = summaryText;
	      if (!owners.length) {
	        adminTaskOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">暂无负责人任务</td></tr>';
	        return;
	      }
	      adminTaskOwnersEl.innerHTML = owners.map(item => {
	        const ownerSummary = item.summary || {};
	        const recent = (item.recentTasks || []).map(task => {
	          return '#' + esc(task.id) + ' ' + esc(adminTaskTypeLabel(task.taskType)) + ' / ' + esc(packageSyncTaskStatusLabel(task.status));
	        }).join('<br>');
	        const lastText = item.lastError || item.lastAppliedAt || item.lastTaskAt || '-';
	        return '<tr>' +
	          '<td>' + esc(item.owner || '-') + '<br><span class="subtitle">用户 ' + esc(item.actorUserId || 0) + ' / 租户 ' + esc(item.actorTenantId || 0) + '</span></td>' +
	          '<td>可处理 ' + esc(ownerSummary.actionableCount || 0) + '<br><span class="subtitle">待 ' + esc(ownerSummary.pendingCount || 0) + ' / 阻断 ' + esc(ownerSummary.blockedCount || 0) + ' / 失败 ' + esc(ownerSummary.failedCount || 0) + '</span></td>' +
	          '<td>套餐同步 ' + esc(ownerSummary.packageSyncCount || 0) + '<br><span class="subtitle">开户 ' + esc(ownerSummary.tenantProvisionCount || 0) + ' / 续费 ' + esc(ownerSummary.tenantRenewalCount || 0) + '</span></td>' +
	          '<td>' + (recent || '-') + '</td>' +
	          '<td>' + esc(lastText) + '<br><span class="subtitle">任务 ' + esc(ownerSummary.taskCount || 0) + ' / 租户 ' + esc(ownerSummary.tenantCount || 0) + '</span></td>' +
	        '</tr>';
	      }).join('');
	    }
	    async function loadAdminTaskOwners() {
	      try {
		        const res = await fetch('/dashboard/saasAdmin/taskOwners?' + adminTaskParams(20).toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        renderAdminTaskOwners(data.owners || [], data.summary || {}, data.ownerCount, data.returnedCount, data.scannedTaskCount, data.partial);
	      } catch (err) {
	        adminTaskOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看任务负责人</td></tr>';
	        adminTaskOwnerSummaryEl.textContent = err.message || String(err);
	      }
	    }
	    function adminTaskSlaStatusLabel(status) {
	      switch (status) {
	        case 'overdue': return '逾期';
	        case 'warning': return '预警';
	        case 'fresh': return '正常';
	        case 'unknown': return '时间未知';
	        default: return status || '-';
	      }
	    }
	    function renderAdminTaskSla(items, summary, taskSummary, returnedCount, ownerCount, scannedTaskCount, partial) {
	      items = items || [];
	      summary = summary || {};
	      taskSummary = taskSummary || {};
	      const shown = returnedCount === undefined ? items.length : returnedCount;
	      const summaryText = '活跃 ' + fmt(summary.taskCount || 0) +
	        ' 条 / 逾期 ' + fmt(summary.overdueCount || 0) +
	        ' / 预警 ' + fmt(summary.warningCount || 0) +
	        ' / 正常 ' + fmt(summary.freshCount || 0) +
	        ' / 最大 ' + fmt(summary.maxAgeHours || 0) + ' 小时' +
	        ' / 负责人 ' + fmt(ownerCount || 0) +
	        ' / 原始任务 ' + fmt(taskSummary.taskCount || 0) +
	        (summary.taskCount > shown ? ' / 显示 ' + fmt(shown) + ' 条' : '') +
	        (partial ? ' / 超出扫描上限' : '') +
	        ' / 扫描 ' + fmt(scannedTaskCount || 0);
	      adminTaskSlaSummaryEl.textContent = summaryText;
	      if (!items.length) {
	        adminTaskSlaEl.innerHTML = '<tr><td colspan="5" class="empty">暂无活跃任务</td></tr>';
	        return;
	      }
	      adminTaskSlaEl.innerHTML = items.map(item => {
	        const task = item.task || {};
	        const breach = item.breachHours > 0 ? '<br><span class="subtitle">超时 ' + esc(item.breachHours) + ' 小时</span>' : '';
	        return '<tr>' +
	          '<td>#' + esc(task.id || '-') + '<br><span class="subtitle">' + esc(task.createdAt || '-') + '</span></td>' +
	          '<td>' + esc(adminTaskSlaStatusLabel(item.slaStatus)) + '<br><span class="subtitle">已过 ' + esc(item.ageHours || 0) + ' 小时</span>' + breach + '</td>' +
	          '<td>' + esc(adminTaskTypeLabel(task.taskType)) + '<br><span class="subtitle">' + esc(packageSyncTaskStatusLabel(task.status)) + '</span></td>' +
	          '<td>' + esc(item.owner || '-') + '<br><span class="subtitle">用户 ' + esc(task.actorUserId || 0) + ' / 租户 ' + esc(task.actorTenantId || 0) + '</span></td>' +
	          '<td>' + esc(adminTaskResultText(task)) + '</td>' +
	        '</tr>';
	      }).join('');
	    }
	    async function loadAdminTaskSla() {
	      try {
		        const res = await fetch('/dashboard/saasAdmin/taskSla?' + adminTaskSlaParams(20).toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        renderAdminTaskSla(data.tasks || [], data.summary || {}, data.taskSummary || {}, data.returnedCount, data.ownerCount, data.scannedTaskCount, data.partial);
	      } catch (err) {
	        adminTaskSlaEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看任务SLA</td></tr>';
	        adminTaskSlaSummaryEl.textContent = err.message || String(err);
	      }
	    }
	    async function createTaskSlaNotifications() {
	      statusEl.textContent = '正在生成运营任务SLA提醒';
	      try {
		        const res = await fetch('/dashboard/saasAdmin/taskSlaNotifications?' + adminTaskSlaParams(50).toString(), {
	          method: 'POST',
	          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	          body: JSON.stringify({ slaStatus: 'warning', remark: '运营任务SLA催办' }),
	        });
	        const body = await res.json();
	        const data = body.data || {};
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        statusEl.textContent = 'SLA提醒已生成 ' + fmt(data.enqueuedCount || 0) +
	          ' 条 / 已存在 ' + fmt(data.skippedExistingCount || 0) +
	          ' / 未达阈值 ' + fmt(data.skippedStatusCount || 0);
	        loadAdminTaskSla();
	        loadNotifications();
	        loadOperations();
	      } catch (err) {
	        statusEl.textContent = err.message || String(err);
	      }
	    }
	    async function applyAdminTask(taskId, taskType, taskVersion) {
	      taskId = Number(taskId || 0);
	      if (taskType === 'tenant_provision' && approvalActionRequired('tenant.provision', 0)) {
	        await applyProvisionTask(taskId, taskVersion);
	        return;
	      }
	      if (taskType === 'tenant_renewal' && approvalActionRequired('tenant.renewal', 0)) {
	        await applyRenewalTask(taskId, taskVersion);
	        return;
	      }
	      const endpoint = adminTaskApplyEndpoint(taskType);
	      if (!taskId || !endpoint) {
	        statusEl.textContent = '缺少可应用的任务';
	        return;
	      }
	      statusEl.textContent = '正在应用' + adminTaskTypeLabel(taskType) + '任务';
	      try {
	        const res = await fetch(endpoint, {
	          method: 'POST',
	          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	          body: JSON.stringify({ taskId }),
	        });
	        const body = await res.json();
	        const data = body.data || {};
	        if (data.task) renderAdminTasks([data.task]);
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        statusEl.textContent = adminTaskTypeLabel(taskType) + '任务已应用：#' + fmt((data.task || {}).id || taskId);
	        loadAdminTasks();
	        loadPackageSyncTasks();
	        loadProvisionTasks();
	        loadRenewalTasks();
	        loadOverview();
	        loadOperations();
	        loadBillingEvents();
	        loadAlerts();
	      } catch (err) {
	        statusEl.textContent = err.message || String(err);
	      }
	    }
	    async function cancelAdminTask(taskId) {
	      taskId = Number(taskId || 0);
	      if (!taskId) {
	        statusEl.textContent = '缺少可取消的任务';
	        return;
	      }
	      statusEl.textContent = '正在取消运营任务';
	      try {
	        const res = await fetch('/dashboard/saasAdmin/taskCancel', {
	          method: 'POST',
	          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	          body: JSON.stringify({ taskId, remark: '页面取消任务' }),
	        });
	        const body = await res.json();
	        const data = body.data || {};
	        if (data.task) renderAdminTasks([data.task]);
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        statusEl.textContent = '运营任务已取消：#' + fmt((data.task || {}).id || taskId);
	        loadAdminTasks();
	        loadPackageSyncTasks();
	        loadProvisionTasks();
	        loadRenewalTasks();
		      } catch (err) {
		        statusEl.textContent = err.message || String(err);
		      }
		    }
			    async function bulkCancelAdminTasks() {
		      const payload = {
		        taskType: adminTaskTypeInput.value || 'all',
		        status: adminTaskStatusInput.value || 'all',
		        limit: 100,
		        remark: '页面批量取消运营任务',
		      };
		      if (adminTaskTenantInput.value.trim()) payload.tenantId = Number(adminTaskTenantInput.value.trim());
		      if (adminTaskPackageCodeInput.value.trim()) payload.packageCode = adminTaskPackageCodeInput.value.trim();
		      if (!window.confirm('确认批量取消当前筛选下最多 100 条未应用任务？')) return;
		      statusEl.textContent = '正在批量取消运营任务';
		      try {
		        const res = await fetch('/dashboard/saasAdmin/taskBulkCancel', {
		          method: 'POST',
		          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
		          body: JSON.stringify(payload),
		        });
		        const body = await res.json();
		        const data = body.data || {};
		        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		        statusEl.textContent = '已批量取消 ' + fmt(data.canceledCount || 0) + ' 条运营任务，跳过已应用 ' + fmt(data.skippedAppliedCount || 0) + ' 条';
		        loadAdminTasks();
		        loadPackageSyncTasks();
	        loadProvisionTasks();
	        loadRenewalTasks();
	        loadOperations();
		      } catch (err) {
			        statusEl.textContent = err.message || String(err);
			      }
			    }
			    async function bulkResetAdminTasks() {
			      const payload = {
			        taskType: adminTaskTypeInput.value || 'all',
			        status: adminTaskStatusInput.value || 'all',
			        limit: 100,
			        remark: '页面批量重置运营任务',
			      };
			      if (adminTaskTenantInput.value.trim()) payload.tenantId = Number(adminTaskTenantInput.value.trim());
			      if (adminTaskPackageCodeInput.value.trim()) payload.packageCode = adminTaskPackageCodeInput.value.trim();
			      if (!window.confirm('确认批量重置当前筛选下最多 100 条阻断或失败任务？')) return;
			      statusEl.textContent = '正在批量重置运营任务';
			      try {
			        const res = await fetch('/dashboard/saasAdmin/taskBulkReset', {
			          method: 'POST',
			          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
			          body: JSON.stringify(payload),
			        });
			        const body = await res.json();
			        const data = body.data || {};
			        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
			        statusEl.textContent = '已批量重置 ' + fmt(data.resetCount || 0) + ' 条运营任务，跳过待应用 ' + fmt(data.skippedPendingCount || 0) + ' 条';
			        loadAdminTasks();
			        loadPackageSyncTasks();
		        loadProvisionTasks();
		        loadRenewalTasks();
		        loadOperations();
			      } catch (err) {
			        statusEl.textContent = err.message || String(err);
			      }
			    }
			    async function bulkApplyRenewalTasks() {
			      const payload = {
			        taskType: 'tenant_renewal',
			        status: adminTaskStatusInput.value && adminTaskStatusInput.value !== 'all' ? adminTaskStatusInput.value : 'pending',
			        limit: 100,
			        remark: '页面批量应用续费任务',
			      };
			      if (adminTaskTenantInput.value.trim()) payload.tenantId = Number(adminTaskTenantInput.value.trim());
			      if (adminTaskPackageCodeInput.value.trim()) payload.packageCode = adminTaskPackageCodeInput.value.trim();
			      const requiresApproval = approvalActionRequired('tenant.renewal', 0);
			      if (!window.confirm(requiresApproval ? '确认按当前筛选为最多 100 条续费任务逐条提交双人审批？' : '确认批量应用当前筛选下最多 100 条租户续费任务？')) return;
			      statusEl.textContent = requiresApproval ? '正在批量提交续费审批' : '正在批量应用续费任务';
			      try {
			        if (requiresApproval) {
			          const params = new URLSearchParams({ taskType: 'tenant_renewal', limit: String(payload.limit || 100) });
			          if (payload.status && payload.status !== 'all') params.set('status', payload.status);
			          if (payload.tenantId) params.set('tenantId', String(payload.tenantId));
			          if (payload.packageCode) params.set('packageCode', payload.packageCode);
			          const tasksRes = await fetch('/dashboard/saasAdmin/tasks?' + params.toString(), { headers: authHeader() });
			          const tasksBody = await tasksRes.json();
			          if (!tasksRes.ok || tasksBody.code !== 200) throw new Error(tasksBody.msg || 'HTTP ' + tasksRes.status);
			          const tasks = (((tasksBody.data || {}).tasks) || []).filter(task => task.canApply && Number(task.version || 0) > 0);
			          let submitted = 0;
			          const failures = [];
			          for (const task of tasks) {
			            try {
			              await requestHighRiskApproval(
			                'tenant.renewal',
			                { taskId: Number(task.id), expectedTaskVersion: Number(task.version || 0) },
			                '批量执行租户续费任务 #' + fmt(task.id),
			                { refresh: false, idempotencyKey: 'page:tenant.renewal:task:' + fmt(task.id) + ':v' + fmt(task.version || 0) + ':' + Date.now() },
			              );
			              submitted++;
			            } catch (err) {
			              failures.push('#' + fmt(task.id) + ' ' + (err.message || String(err)));
			            }
			          }
			          await loadApprovals();
			          statusEl.textContent = '已提交 ' + fmt(submitted) + ' 条续费审批，失败 ' + fmt(failures.length) + ' 条' +
			            (failures.length ? '：' + failures.slice(0, 3).join('；') : '');
			          await loadAdminTasks();
			          await loadRenewalTasks();
			          return;
			        }
			        const res = await fetch('/dashboard/saasAdmin/tenantRenewalTaskBulkApply', {
			          method: 'POST',
			          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
			          body: JSON.stringify(payload),
			        });
			        const body = await res.json();
			        const data = body.data || {};
			        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
			        statusEl.textContent = '已批量应用 ' + fmt(data.appliedCount || 0) + ' 条续费任务，阻断 ' + fmt(data.blockedCount || 0) + ' 条，失败 ' + fmt(data.failedCount || 0) + ' 条';
			        loadAdminTasks();
			        loadRenewalTasks();
			        loadOverview();
			        loadOperations();
			        loadBillingEvents();
			        loadAlerts();
			      } catch (err) {
			        statusEl.textContent = err.message || String(err);
			      }
			    }
			    async function resetAdminTask(taskId) {
		      taskId = Number(taskId || 0);
		      if (!taskId) {
		        statusEl.textContent = '缺少可重置的任务';
		        return;
		      }
		      statusEl.textContent = '正在重置运营任务';
		      try {
		        const res = await fetch('/dashboard/saasAdmin/taskReset', {
		          method: 'POST',
		          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
		          body: JSON.stringify({ taskId, remark: '页面重置任务' }),
		        });
		        const body = await res.json();
		        const data = body.data || {};
		        if (data.task) renderAdminTasks([data.task]);
		        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		        statusEl.textContent = '运营任务已重置为待应用：#' + fmt((data.task || {}).id || taskId);
		        loadAdminTasks();
		        loadPackageSyncTasks();
		        loadProvisionTasks();
		        loadRenewalTasks();
		        loadOperations();
		      } catch (err) {
		        statusEl.textContent = err.message || String(err);
		      }
		    }
		    function traceAdminTaskOperations(taskId) {
		      taskId = Number(taskId || 0);
		      if (!taskId) {
			        statusEl.textContent = '缺少可追溯的任务';
			        return;
			      }
			      operationActionInput.value = '';
			      operationTargetTypeInput.value = 'admin_task';
			      operationKeywordInput.value = String(taskId);
			      statusEl.textContent = '正在追溯运营任务 #' + fmt(taskId);
			      loadOperations();
			    }
			    function packageSyncTaskSummary(task) {
	      const result = task.result || task.preview || {};
      const parts = [];
      if (result.matchedTenantCount !== undefined) parts.push('命中 ' + fmt(result.matchedTenantCount));
      if (result.overLimitTenantCount !== undefined) parts.push('超额 ' + fmt(result.overLimitTenantCount));
      if (result.syncedTenantCount !== undefined) parts.push('同步 ' + fmt(result.syncedTenantCount));
      if (result.metricsRefreshed !== undefined) parts.push('刷新 ' + fmt(result.metricsRefreshed));
      if (result.blocked) parts.push('阻断');
      if (task.lastError) parts.push(task.lastError);
      return parts.length ? parts.join(' / ') : '-';
    }
    function renderPackageSyncTasks(tasks) {
      tasks = tasks || [];
      if (!tasks.length) {
        packageSyncTasksEl.innerHTML = '<tr><td colspan="6" class="empty">暂无同步任务</td></tr>';
        return;
      }
      packageSyncTasksEl.innerHTML = tasks.map(item => {
        const action = item.canApply
          ? '<button type="button" data-action="apply-package-sync-task" data-task-id="' + esc(item.id) + '">应用</button>'
          : '-';
        return '<tr>' +
          '<td>#' + esc(item.id) + '<br><span class="subtitle">' + esc(item.createdAt || '-') + '</span></td>' +
          '<td>' + esc(item.packageCode || '-') + '</td>' +
          '<td>' + esc(item.tenantId || '全部') + '</td>' +
          '<td>' + esc(packageSyncTaskStatusLabel(item.status)) + '</td>' +
          '<td>' + esc(packageSyncTaskSummary(item)) + '</td>' +
          '<td>' + action + '</td>' +
        '</tr>';
      }).join('');
    }
    function packageSyncInputPayload(dryRun, remark) {
      const packageCode = syncPackageCodeInput.value.trim();
      if (!packageCode) {
        statusEl.textContent = '请选择同步套餐';
        return null;
      }
      const tenantId = Number(syncTenantIdInput.value.trim() || '0');
      const limit = Number(syncLimitInput.value.trim() || '100');
      return {
        packageCode,
        tenantId,
        limit,
        dryRun,
        allowOverLimit: syncAllowOverLimitInput.value === 'true',
        remark,
      };
    }
    async function syncPackage(dryRun) {
      const payload = packageSyncInputPayload(dryRun, dryRun ? '页面预览套餐快照同步' : '页面应用套餐快照同步');
      if (!payload) return;
      statusEl.textContent = dryRun ? '正在预览套餐快照同步' : '正在应用套餐快照同步';
      try {
        const res = await fetch('/dashboard/saasAdmin/packageSync', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        const data = body.data || {};
        renderPackageSyncResult(data);
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = dryRun ? '套餐快照同步预览完成' : '套餐快照同步已应用';
        if (!dryRun) {
          loadOverview();
          loadOperations();
          loadAlerts();
          loadBillingEvents();
        }
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function createPackageSyncTask() {
      const payload = packageSyncInputPayload(true, '页面创建套餐快照同步任务');
      if (!payload) return;
      statusEl.textContent = '正在创建套餐同步任务';
      try {
        const res = await fetch('/dashboard/saasAdmin/packageSyncTask', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        const data = body.data || {};
        if (data.result) renderPackageSyncResult(data.result);
        if (data.task) {
          syncTaskIdInput.value = String(data.task.id || '');
          renderPackageSyncTasks([data.task]);
        }
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        statusEl.textContent = '套餐同步任务已创建：#' + fmt((data.task || {}).id);
	        loadAdminTasks();
	        loadPackageSyncTasks();
	      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function applyPackageSyncTask(taskId) {
      taskId = Number(taskId || syncTaskIdInput.value.trim());
      if (!taskId) {
        statusEl.textContent = '请填写任务 ID';
        return;
      }
      statusEl.textContent = '正在应用套餐同步任务';
      try {
        const res = await fetch('/dashboard/saasAdmin/packageSyncTaskApply', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ taskId }),
        });
        const body = await res.json();
        const data = body.data || {};
        if (data.result) renderPackageSyncResult(data.result);
        if (data.task) renderPackageSyncTasks([data.task]);
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '套餐同步任务已应用：#' + fmt((data.task || {}).id);
        loadOverview();
        loadOperations();
	        loadAlerts();
	        loadBillingEvents();
	        loadAdminTasks();
	        loadPackageSyncTasks();
	      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function bulkApplyPackageSyncTasks() {
      const payload = {
        taskType: 'package_sync',
        status: 'pending',
        limit: 100,
        remark: '页面批量应用套餐同步任务',
      };
      if (syncPackageCodeInput.value.trim()) payload.packageCode = syncPackageCodeInput.value.trim();
      if (syncTenantIdInput.value.trim()) payload.tenantId = Number(syncTenantIdInput.value.trim());
      if (!window.confirm('确认批量应用当前筛选下最多 100 条套餐同步任务？')) return;
      statusEl.textContent = '正在批量应用套餐同步任务';
      try {
        const res = await fetch('/dashboard/saasAdmin/packageSyncTaskBulkApply', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        const data = body.data || {};
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '已批量应用 ' + fmt(data.appliedCount || 0) + ' 条套餐同步任务，阻断 ' + fmt(data.blockedCount || 0) + ' 条，失败 ' + fmt(data.failedCount || 0) + ' 条';
        loadOverview();
        loadOperations();
        loadAlerts();
        loadBillingEvents();
        loadAdminTasks();
        loadPackageSyncTasks();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function loadPackageSyncTasks() {
      const params = new URLSearchParams({ taskType: 'package_sync', limit: '10' });
      if (syncPackageCodeInput.value.trim()) params.set('packageCode', syncPackageCodeInput.value.trim());
      if (syncTenantIdInput.value.trim()) params.set('tenantId', syncTenantIdInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/tasks?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderPackageSyncTasks((body.data || {}).tasks || []);
      } catch (err) {
        packageSyncTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看同步任务</td></tr>';
      }
    }
    function renewalTaskResultText(task) {
      const result = task.result || task.preview || {};
      const parts = [];
      if (result.previousExpiresAt || result.expiresAt) {
        parts.push(fmt(result.previousExpiresAt || '-') + ' -> ' + fmt(result.expiresAt || '-'));
      }
      if (result.amountCents !== undefined) parts.push('金额分 ' + fmt(result.amountCents));
      if (result.billingEventId) parts.push('账单 #' + fmt(result.billingEventId));
      if (result.metricsRefreshed !== undefined) parts.push('刷新 ' + fmt(result.metricsRefreshed));
      if (result.blocked) parts.push(result.blockReason || '已阻断');
      if (task.lastError) parts.push(task.lastError);
      return parts.length ? parts.join(' / ') : '-';
    }
    function renderRenewalTasks(tasks) {
      tasks = tasks || [];
      renewalTaskCache = tasks.slice();
      if (!tasks.length) {
        renewalTasksEl.innerHTML = '<tr><td colspan="6" class="empty">暂无续费任务</td></tr>';
        renewalTaskSummaryEl.textContent = '创建任务后显示预览';
        return;
      }
      const first = tasks[0];
      renewalTaskSummaryEl.textContent = '最近任务 #' + fmt(first.id) + ' / ' + packageSyncTaskStatusLabel(first.status) + ' / ' + renewalTaskResultText(first);
      renewalTasksEl.innerHTML = tasks.map(item => {
        const action = item.canApply
          ? '<button type="button" data-action="apply-renewal-task" data-task-id="' + esc(item.id) + '" data-task-version="' + esc(item.version || 0) + '">' +
            (approvalActionRequired('tenant.renewal', 0) ? '提交审批' : '应用') + '</button>'
          : '-';
        const result = item.result || item.preview || {};
        return '<tr>' +
          '<td>#' + esc(item.id) + '<br><span class="subtitle">' + esc(item.createdAt || '-') + '</span></td>' +
          '<td>' + esc(result.tenantName || item.tenantId || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId || '-') + '</span></td>' +
          '<td>' + esc(result.packageName || item.packageCode || '-') + '<br><span class="subtitle">' + esc(item.packageCode || '-') + '</span></td>' +
          '<td>' + esc(packageSyncTaskStatusLabel(item.status)) + '</td>' +
          '<td>' + esc(renewalTaskResultText(item)) + '</td>' +
          '<td>' + action + '</td>' +
        '</tr>';
      }).join('');
    }
    function renewalPayload(remark) {
      const tenantId = Number(renewalTenantInput.value.trim());
      if (!tenantId || !renewalExpiresInput.value.trim()) {
        statusEl.textContent = '请填写续费租户 ID 和新到期时间';
        return null;
      }
      return {
        tenantId,
        packageCode: renewalPackageCodeInput.value.trim(),
        expiresAt: renewalExpiresInput.value.trim(),
        amount: renewalAmountInput.value.trim(),
        externalOrderNo: renewalOrderInput.value.trim(),
        remark,
      };
    }
    async function createRenewalTask() {
      const payload = renewalPayload('页面创建续费任务');
      if (!payload) return;
      statusEl.textContent = '正在创建续费任务';
      try {
        const res = await fetch('/dashboard/saasAdmin/tenantRenewalTask', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        const data = body.data || {};
        if (data.task) {
          renewalTaskIdInput.value = String(data.task.id || '');
          renderRenewalTasks([data.task]);
        }
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        statusEl.textContent = '续费任务已创建：#' + fmt((data.task || {}).id);
	        loadAdminTasks();
	        loadRenewalTasks();
	        loadBusinessTrends();
	        loadRenewalForecast();
	      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function applyRenewalTask(taskId, taskVersion) {
      taskId = Number(taskId || renewalTaskIdInput.value.trim());
      if (!taskId) {
        statusEl.textContent = '请填写续费任务 ID';
        return;
      }
      const requiresApproval = approvalActionRequired('tenant.renewal', 0);
      statusEl.textContent = requiresApproval ? '正在提交续费任务审批' : '正在应用续费任务';
      try {
        if (requiresApproval) {
          let version = Number(taskVersion || 0);
          let task = renewalTaskCache.find(item => Number(item.id) === taskId);
          if (!version && task) version = Number(task.version || 0);
          if (!version) {
            const taskRes = await fetch('/dashboard/saasAdmin/tasks?taskId=' + encodeURIComponent(taskId) + '&limit=1', { headers: authHeader() });
            const taskBody = await taskRes.json();
            if (!taskRes.ok || taskBody.code !== 200) throw new Error(taskBody.msg || 'HTTP ' + taskRes.status);
            task = (((taskBody.data || {}).tasks) || [])[0];
            version = Number((task || {}).version || 0);
          }
          if (!version) throw new Error('续费任务版本缺失，请刷新任务后重试');
          const approval = await requestHighRiskApproval(
            'tenant.renewal',
            { taskId, expectedTaskVersion: version },
            '执行租户续费任务 #' + taskId,
          );
          statusEl.textContent = '续费任务已提交双人审批：' + fmt(approval.requestNo || '');
          await loadAdminTasks();
          await loadRenewalTasks();
          return;
        }
        const res = await fetch('/dashboard/saasAdmin/tenantRenewalTaskApply', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ taskId }),
        });
        const body = await res.json();
        const data = body.data || {};
        if (data.task) renderRenewalTasks([data.task]);
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        tenantInput.value = String((data.result || {}).tenantId || renewalTenantInput.value.trim());
        packageExpiresInput.value = (data.result || {}).expiresAt || renewalExpiresInput.value.trim();
        statusEl.textContent = '续费任务已应用：#' + fmt((data.task || {}).id);
	        loadOverview();
	        loadOperations();
	        loadBillingEvents();
	        loadAdminTasks();
	        loadRenewalTasks();
	        loadBusinessTrends();
	        loadRenewalForecast();
	      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function loadRenewalTasks() {
      const params = new URLSearchParams({ taskType: 'tenant_renewal', limit: '10' });
      if (renewalTenantInput.value.trim()) params.set('tenantId', renewalTenantInput.value.trim());
      if (renewalPackageCodeInput.value.trim()) params.set('packageCode', renewalPackageCodeInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/tasks?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderRenewalTasks((body.data || {}).tasks || []);
      } catch (err) {
        renewalTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看续费任务</td></tr>';
      }
    }
    function tenantReadinessState(value) {
      switch (value) {
        case 'ready': return ['可上线', 'ok'];
        case 'attention': return ['待完善', 'warning'];
        case 'blocked': return ['已阻塞', 'danger'];
        default: return [value || '未知', ''];
      }
    }
    function tenantReadinessTargetSection(workspace, sectionLabel) {
      return workspaceSections.find(section =>
        section.dataset.workspace === workspace && workspaceSectionLabel(section, 0) === sectionLabel
      );
    }
    function tenantReadinessActionAvailable(check) {
      if (!check || !check.actionWorkspace || !check.actionSectionLabel) return false;
      const section = tenantReadinessTargetSection(check.actionWorkspace, check.actionSectionLabel);
      return !!section && accessibleWorkspaceSections(check.actionWorkspace).includes(section);
    }
    function renderTenantReadiness(data) {
      data = data || {};
      const summary = data.summary || {};
      const items = Array.isArray(data.tenants) ? data.tenants : [];
      tenantReadinessCache = items.slice();
      const tiles = [
        ['业务租户', summary.tenantCount || 0],
        ['可上线', summary.readyCount || 0],
        ['待完善', summary.attentionCount || 0],
        ['已阻塞', summary.blockedCount || 0],
      ];
      tenantReadinessSummaryEl.innerHTML = tiles.map(item =>
        '<div class="tile"><div class="label">' + esc(item[0]) + '</div><div class="value">' + esc(item[1]) + '</div></div>'
      ).join('');
      tenantReadinessStateEl.textContent =
        '业务租户核心就绪 ' + fmt(summary.coreReadyCount || 0) + '/' + fmt(summary.tenantCount || 0) +
        ' / 平均完成度 ' + fmt(summary.averageCompletionPercent || 0) + '%' +
        ' / 当前命中 ' + fmt(summary.filteredCount || 0) +
        (data.truncated ? ' / 结果已截断' : '') +
        (data.generatedAt ? ' / ' + data.generatedAt : '');
      if (!items.length) {
        tenantReadinessTenantsEl.innerHTML = '<tr><td colspan="6" class="empty">当前筛选没有业务租户</td></tr>';
        return;
      }
      tenantReadinessTenantsEl.innerHTML = items.map(item => {
        const state = tenantReadinessState(item.state);
        const checks = Array.isArray(item.checks) ? item.checks : [];
        const pending = checks.filter(check => !check.passed);
        const pendingHTML = pending.length
          ? '<div class="readiness-checks">' + pending.slice(0, 4).map(check =>
              '<span class="pill ' + (check.required ? 'danger' : 'warning') + '" title="' + esc(check.detail || '') + '">' + esc(check.label || check.code) + '</span>'
            ).join('') + (pending.length > 4 ? '<span class="muted">+' + esc(pending.length - 4) + '</span>' : '') + '</div>'
          : pill('全部通过', 'ok');
        const target = pending.find(tenantReadinessActionAvailable);
        const action = target
          ? '<button type="button" class="secondary" data-readiness-tenant="' + esc(item.tenantId) + '" data-readiness-check="' + esc(target.code) + '" data-readiness-workspace="' + esc(target.actionWorkspace) + '" data-readiness-section="' + esc(target.actionSectionLabel) + '">定位</button>'
          : (pending.length ? '<button type="button" class="secondary" disabled title="当前岗位无对应模块权限">定位</button>' : '-');
        return '<tr>' +
          '<td>' + pill(state[0], state[1]) + '</td>' +
          '<td><strong>' + esc(item.tenantName || '-') + '</strong><br><span class="subtitle">ID ' + esc(item.tenantId) + ' / ' + esc(item.packageCode || '未分配套餐') + ' / ' + esc(item.subscriptionStatus || '无订阅') + '</span></td>' +
          '<td><strong>' + esc(item.completionPercent || 0) + '%</strong><progress class="readiness-progress" max="100" value="' + esc(item.completionPercent || 0) + '"></progress></td>' +
          '<td>' + esc(item.requiredPassedCheckCount || 0) + '/' + esc(item.requiredCheckCount || 0) + '<br><span class="subtitle">阻塞项 ' + esc(item.failedCheckCount || 0) + '</span></td>' +
          '<td>' + pendingHTML + '</td>' +
          '<td>' + action + '</td>' +
        '</tr>';
      }).join('');
    }
    async function loadTenantReadiness() {
      if (platformAccessControlled && !hasPlatformPermission('platform.tenants.read')) return;
      tenantReadinessStateEl.textContent = '加载中';
      const params = new URLSearchParams({
        state: tenantReadinessFilterStateInput.value || 'all',
        limit: '100',
      });
      if (tenantReadinessTenantIdInput.value.trim()) params.set('tenantId', tenantReadinessTenantIdInput.value.trim());
      if (tenantReadinessKeywordInput.value.trim()) params.set('keyword', tenantReadinessKeywordInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/tenantReadiness?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderTenantReadiness(body.data || {});
      } catch (err) {
        tenantReadinessStateEl.textContent = err.message || String(err);
        tenantReadinessTenantsEl.innerHTML = '<tr><td colspan="6" class="empty">准备度加载失败</td></tr>';
      }
    }
    function focusTenantReadinessAction(button) {
      const tenantId = Number(button.dataset.readinessTenant || 0);
      const checkCode = button.dataset.readinessCheck || '';
      const workspace = button.dataset.readinessWorkspace || 'tenant';
      const sectionLabel = button.dataset.readinessSection || '';
      if (!tenantId) return;
      const tenantValue = String(tenantId);
      tenantInput.value = tenantValue;
      scopeInput.value = 'platform';
      if (checkCode === 'tenant_active') statusTenantInput.value = tenantValue;
      if (checkCode === 'package') packageTenantInput.value = tenantValue;
      if (checkCode === 'subscription') {
        subscriptionTenantInput.value = tenantValue;
        subscriptionKeywordInput.value = tenantValue;
        void loadSubscriptions();
      }
      if (checkCode === 'baseline_seed') syncTenantIdInput.value = tenantValue;
      if (checkCode === 'wecom_corp' || checkCode === 'wecom_credentials') {
        weComCredentialTenantIdInput.value = tenantValue;
        void loadWeComCredentialProtection();
      }
      if (checkCode === 'identity_policy') {
        identityTenantFilterInput.value = tenantValue;
        void loadIdentitySecurity();
      }
      if (checkCode === 'notification_policy') {
        notificationPolicyTenantInput.value = tenantValue;
        notificationPolicyKeywordInput.value = tenantValue;
        void loadNotificationPolicies();
      }
      if (checkCode === 'branding') {
        brandingTenantFilterInput.value = tenantValue;
        void loadBrandingProfiles();
      }
      if (checkCode === 'primary_domain') {
        tenantDomainTenantFilterInput.value = tenantValue;
        tenantDomainCreateTenantIdInput.value = tenantValue;
        void loadTenantDomains();
      }
      applyWorkspaceView(workspace, false);
      requestAnimationFrame(() => {
        const section = tenantReadinessTargetSection(workspace, sectionLabel);
        if (section) section.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    }
    async function loadPackages() {
      try {
        const res = await fetch('/dashboard/saasAdmin/packages', { headers: authHeader() });
        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        renderPackages((body.data || {}).packages || []);
	        if (hasPlatformPermission('platform.operations.read')) {
	          loadAdminTasks();
	          loadProvisionTasks();
	          loadRenewalTasks();
	        }
	      } catch (err) {
        provisionPackageCodeInput.innerHTML = '<option value="">仅平台管理员可加载套餐</option>';
        filterPackageCodeInput.innerHTML = '<option value="">全部套餐</option>';
	        billingPackageCodeInput.innerHTML = '<option value="">全部套餐</option>';
	        adminTaskPackageCodeInput.innerHTML = '<option value="">全部套餐</option>';
        packageCodeInput.innerHTML = '<option value="">仅平台管理员可加载套餐</option>';
        syncPackageCodeInput.innerHTML = '<option value="">仅平台管理员可加载套餐</option>';
        paymentPackageCodeInput.innerHTML = '<option value="">仅平台管理员可加载套餐</option>';
        renewalPackageCodeInput.innerHTML = '<option value="">仅平台管理员可加载套餐</option>';
        clearPackageImpact('仅平台管理员可查看套餐影响');
	        clearPackageSync('仅平台管理员可同步套餐快照');
	        adminTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看运营任务</td></tr>';
	        packageSyncTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看同步任务</td></tr>';
	        provisionTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看开户任务</td></tr>';
	        renewalTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看续费任务</td></tr>';
	      }
	    }
	function auditIntegrityStatusBadge(status) {
	  if (status === 'healthy') return pill('完整', 'ok');
	  if (status === 'failed') return pill('异常', 'danger');
	  return pill('待封存', 'warning');
	}
	function auditIntegrityHash(value) {
	  const hash = String(value || '').trim();
	  if (!hash) return '-';
	  return '<span class="audit-hash" title="' + esc(hash) + '">' + esc(hash.slice(0, 12)) + '...</span>';
	}
	function renderAuditIntegrity(data) {
	  const summary = data.summary || {};
	  const chains = Array.isArray(data.chains) ? data.chains : [];
	  const verifications = Array.isArray(data.verifications) ? data.verifications : [];
	  document.getElementById('auditIntegrityChainSummary').innerHTML = fmt(summary.chainCount || 0) + '<br><span class="subtitle">完整 ' + fmt(summary.healthyChainCount || 0) + ' / 异常 ' + fmt(summary.failedChainCount || 0) + '</span>';
	  document.getElementById('auditIntegrityLogSummary').innerHTML = fmt(summary.signedLogCount || 0) + '<br><span class="subtitle">Legacy ' + fmt(summary.legacyLogCount || 0) + '</span>';
	  document.getElementById('auditIntegrityUnsealedSummary').textContent = fmt(summary.unsealedTenantCount || 0);
	  document.getElementById('auditIntegrityRetentionSummary').innerHTML = fmt(summary.retentionEligibleLogCount || 0) + '<br><span class="subtitle">' + fmt(summary.retentionDays || 0) + ' 天 / ' + esc(summary.retentionCutoff || '-') + '</span>';
	  document.getElementById('auditIntegrityChainCount').textContent = chains.length + ' 条';
	  document.getElementById('auditIntegrityVerificationCount').textContent = verifications.length + ' 条';
	  auditIntegrityStateEl.className = Number(summary.failedChainCount || 0) > 0 ? 'bad' : 'good';
	  auditIntegrityStateEl.textContent = Number(summary.failedChainCount || 0) > 0
	    ? fmt(summary.failedChainCount) + ' 条摘要链异常'
	    : '摘要链状态正常';
	  auditIntegrityChainsEl.innerHTML = chains.length ? chains.map(item => {
	    const failure = item.lastVerificationError
	      ? '日志 #' + fmt(item.lastFailedLogId || 0) + '<br><span class="bad">' + esc(item.lastVerificationError) + '</span>'
	      : '-';
	    return '<tr>' +
	      '<td>' + esc(item.tenantName || ('租户 #' + item.tenantId)) + '<br><span class="subtitle">ID ' + fmt(item.tenantId) + '</span></td>' +
	      '<td>' + auditIntegrityStatusBadge(item.status) + '<br><span class="subtitle">v' + fmt(item.version || 0) + '</span></td>' +
	      '<td>#' + fmt(item.anchorLogId || 0) + ' / ' + fmt(item.legacyLogCount || 0) + '<br>' + auditIntegrityHash(item.anchorHash) + '</td>' +
	      '<td>#' + fmt(item.lastLogId || 0) + ' / ' + fmt(item.signedLogCount || 0) + '<br>' + auditIntegrityHash(item.lastHash) + '</td>' +
	      '<td>' + esc(item.lastVerifiedAt || '尚未校验') + '<br><span class="subtitle">封存 ' + esc(item.sealedAt || '-') + '</span></td>' +
	      '<td>' + failure + '</td>' +
	    '</tr>';
	  }).join('') : '<tr><td colspan="6" class="empty">暂无摘要链，执行校验后自动封存</td></tr>';
	  const sourceNames = { admin_manual: '总后台', manual: '手动', maintenance: '维护命令', cron: '定时任务' };
	  auditIntegrityVerificationsEl.innerHTML = verifications.length ? verifications.map(item => {
	    const failure = item.errorMessage
	      ? '日志 #' + fmt(item.failedLogId || 0) + '<br><span class="bad">' + esc(item.errorMessage) + '</span>'
	      : '-';
	    return '<tr>' +
	      '<td>' + esc(item.finishedAt || '-') + '<br><span class="subtitle">' + esc(item.startedAt || '-') + '</span></td>' +
	      '<td>' + esc(sourceNames[item.source] || item.source || '-') + '<br><span class="subtitle">操作人 ' + fmt(item.actorUserId || 0) + '</span></td>' +
	      '<td>' + esc(item.tenantName || ('租户 #' + item.tenantId)) + '<br><span class="subtitle">ID ' + fmt(item.tenantId) + '</span></td>' +
	      '<td>' + auditIntegrityStatusBadge(item.status) + '</td>' +
	      '<td>' + fmt(item.verifiedLogCount || 0) + ' 条<br><span class="subtitle">Legacy ' + fmt(item.legacyLogCount || 0) + ' / 签名 ' + fmt(item.signedLogCount || 0) + '</span></td>' +
	      '<td>' + failure + '</td>' +
	    '</tr>';
	  }).join('') : '<tr><td colspan="6" class="empty">暂无校验记录</td></tr>';
	}
	async function loadAuditIntegrity() {
	  if (!hasPlatformPermission('platform.audit.read')) return;
	  const params = new URLSearchParams({
	    tenantId: String(Number(auditIntegrityTenantIdInput.value || 0)),
	    limit: String(Number(auditIntegrityLimitInput.value || 50)),
	    verificationLimit: String(Number(auditIntegrityVerificationLimitInput.value || 20)),
	  });
	  auditIntegrityStateEl.className = '';
	  auditIntegrityStateEl.textContent = '加载中';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/auditIntegrity?' + params.toString(), { headers: authHeader() });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    renderAuditIntegrity(body.data || {});
	  } catch (err) {
	    auditIntegrityStateEl.className = 'bad';
	    auditIntegrityStateEl.textContent = err.message || String(err);
	    auditIntegrityChainsEl.innerHTML = '<tr><td colspan="6" class="empty">审计完整性数据加载失败</td></tr>';
	    auditIntegrityVerificationsEl.innerHTML = '<tr><td colspan="6" class="empty">暂无校验记录</td></tr>';
	  }
	}
	async function verifyAuditIntegrity() {
	  const button = document.getElementById('verifyAuditIntegrity');
	  if (!hasPlatformPermission('platform.audit.manage')) return;
	  button.disabled = true;
	  auditIntegrityStateEl.className = '';
	  auditIntegrityStateEl.textContent = '校验中';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/auditIntegrityVerify', {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify({
	        tenantId: Number(auditIntegrityTenantIdInput.value || 0),
	        limit: Number(auditIntegrityLimitInput.value || 50),
	        verificationLimit: Number(auditIntegrityVerificationLimitInput.value || 20),
	      }),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const result = body.data || {};
	    await loadAuditIntegrity();
	    auditIntegrityStateEl.className = Number(result.failedChains || 0) > 0 ? 'bad' : 'good';
	    auditIntegrityStateEl.textContent = '已校验 ' + fmt(result.scannedChains || 0) + ' 条，完整 ' + fmt(result.healthyChains || 0) + '，异常 ' + fmt(result.failedChains || 0) + '，审计 #' + fmt(result.operationId || 0);
	  } catch (err) {
	    auditIntegrityStateEl.className = 'bad';
	    auditIntegrityStateEl.textContent = err.message || String(err);
	  } finally {
	    button.disabled = !hasPlatformPermission('platform.audit.manage');
	  }
	}
		function auditAnchorStatusBadge(status, kind) {
		  if (status === 'exported' || status === 'passed') return pill(kind === 'remote' ? '已锁定' : (kind === 'artifact' ? '已写入' : '通过'), 'ok');
		  if (status === 'failed') return pill('异常', 'danger');
		  if (status === 'disabled') return pill('未启用', 'warning');
		  return pill('待校验', 'warning');
		}
	function renderAuditAnchors(data) {
	  const config = data.config || {};
	  const summary = data.summary || {};
	  const items = Array.isArray(data.checkpoints) ? data.checkpoints : [];
	  const missingKeys = Array.isArray(config.missingKeyIds) ? config.missingKeyIds : [];
	  document.getElementById('auditAnchorKeySummary').innerHTML = (config.hmacConfigured ? esc(config.hmacKeyId || '-') : '<span class="bad">未配置</span>') +
	    '<br><span class="subtitle">密钥环 ' + fmt(config.hmacKeyCount || 0) + ' / 缺失 ' + fmt(missingKeys.length) + '</span>';
		  document.getElementById('auditAnchorArtifactSummary').innerHTML = fmt(summary.exportedCount || 0) +
		    '<br><span class="subtitle">失败 ' + fmt(summary.artifactFailedCount || 0) + '</span>';
		  document.getElementById('auditAnchorRemoteSummary').innerHTML = config.remoteConfigured
		    ? fmt(summary.remoteExportedCount || 0) + '<br><span class="subtitle">' + esc(config.remoteProvider || '-') + ' / ' + esc(config.remoteBucket || '-') + '<br>失败 ' + fmt(summary.remoteFailedCount || 0) + '</span>'
		    : (config.remoteRequired ? '<span class="bad">未配置</span><br><span class="subtitle">生产门禁要求启用</span>' : '可选<br><span class="subtitle">当前仅使用本地证据</span>');
		  document.getElementById('auditAnchorRetentionSummary').innerHTML = config.remoteConfigured
		    ? esc(config.remoteRetentionMode || '-') + '<br><span class="subtitle">' + fmt(config.remoteRetentionDays || 0) + ' 天 / 待传 ' + fmt(summary.remotePendingCount || 0) + '</span>'
		    : '-<br><span class="subtitle">Object Lock 未启用</span>';
		  document.getElementById('auditAnchorVerifySummary').innerHTML = fmt(summary.verificationPassedCount || 0) +
	    '<br><span class="subtitle">异常 ' + fmt(summary.verificationFailedCount || 0) + ' / 待验 ' + fmt(summary.pendingVerificationCount || 0) + '</span>';
		  document.getElementById('auditAnchorRollbackSummary').innerHTML = fmt(summary.orphanArtifactCount || 0) + ' / ' + fmt(summary.orphanRemoteCount || 0) +
		    '<br><span class="subtitle">本地 / 远端孤儿证据</span>';
	  document.getElementById('auditAnchorCount').textContent = items.length + ' 条';
		  const unhealthy = !config.hmacConfigured || missingKeys.length > 0 || (config.remoteRequired && !config.remoteConfigured) ||
		    Number(summary.artifactFailedCount || 0) > 0 || Number(summary.remoteFailedCount || 0) > 0 ||
		    Number(summary.remotePendingCount || 0) > 0 || Number(summary.verificationFailedCount || 0) > 0 ||
		    Number(summary.orphanArtifactCount || 0) > 0 || Number(summary.orphanRemoteCount || 0) > 0;
	  auditAnchorStateEl.className = unhealthy ? 'bad' : 'good';
	  auditAnchorStateEl.textContent = !config.hmacConfigured ? 'HMAC 密钥未配置' : (unhealthy ? '签名锚点存在异常' : '签名锚点状态正常');
		  auditAnchorsEl.innerHTML = items.length ? items.map(item => {
		    const artifactError = item.artifactError ? '<br><span class="bad">' + esc(item.artifactError) + '</span>' : '';
		    const remoteError = item.remoteError ? '<br><span class="bad">' + esc(item.remoteError) + '</span>' : '';
		    const verificationError = item.verificationError ? '<br><span class="bad">' + esc(item.verificationError) + '</span>' : '';
		    const remoteName = String(item.remoteObjectKey || '-').split('/').pop();
		    return '<tr>' +
	      '<td>' + esc(item.checkpointNo || '-') + '<br><span class="subtitle">#' + fmt(item.id || 0) + '</span></td>' +
	      '<td>' + esc(item.tenantName || ('租户 #' + item.tenantId)) + '<br><span class="subtitle">ID ' + fmt(item.tenantId) + '</span></td>' +
	      '<td>' + esc(item.signatureAlgorithm || '-') + '<br><span class="subtitle">' + esc(item.keyId || '-') + ' / ' + auditIntegrityHash(item.signature) + '</span></td>' +
	      '<td>#' + fmt(item.chainHeadLogId || 0) + ' / ' + fmt(item.signedLogCount || 0) + '<br>' + auditIntegrityHash(item.chainHeadHash) + '</td>' +
		      '<td>' + auditAnchorStatusBadge(item.artifactStatus, 'artifact') + '<br><span class="subtitle">' + esc(item.artifactName || '-') + '</span>' + artifactError + '</td>' +
		      '<td>' + auditAnchorStatusBadge(item.remoteStatus, 'remote') + '<br><span class="subtitle">' + esc(item.remoteProvider || '-') + ' / ' + esc(item.remoteBucket || '-') + '<br>' + esc(remoteName) + '</span>' +
		        (item.remoteRetentionMode ? '<br><span class="subtitle">' + esc(item.remoteRetentionMode) + ' 至 ' + esc(item.remoteRetainUntil || '-') + '</span>' : '') + remoteError + '</td>' +
		      '<td>' + auditAnchorStatusBadge(item.verificationStatus, 'verify') + '<br><span class="subtitle">' + esc(item.lastVerifiedAt || '尚未校验') + '</span>' + verificationError + '</td>' +
	      '<td>' + esc(item.signedAt || '-') + '<br><span class="subtitle">' + esc(item.source || '-') + '</span></td>' +
	    '</tr>';
		  }).join('') : '<tr><td colspan="8" class="empty">暂无签名检查点，先校验摘要链再创建锚点</td></tr>';
	}
	async function loadAuditAnchors() {
	  if (!hasPlatformPermission('platform.audit.read')) return;
	  const params = new URLSearchParams({
	    tenantId: String(Number(auditIntegrityTenantIdInput.value || 0)),
	    limit: String(Number(auditIntegrityLimitInput.value || 50)),
	  });
	  auditAnchorStateEl.className = '';
	  auditAnchorStateEl.textContent = '加载中';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/auditAnchors?' + params.toString(), { headers: authHeader() });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    renderAuditAnchors(body.data || {});
	  } catch (err) {
	    auditAnchorStateEl.className = 'bad';
	    auditAnchorStateEl.textContent = err.message || String(err);
		    auditAnchorsEl.innerHTML = '<tr><td colspan="8" class="empty">签名锚点数据加载失败</td></tr>';
	  }
	}
	async function runAuditAnchorAction(action) {
	  if (!hasPlatformPermission('platform.audit.manage')) return;
	  const button = document.getElementById(action === 'create' ? 'createAuditAnchor' : 'verifyAuditAnchor');
	  button.disabled = true;
	  auditAnchorStateEl.className = '';
	  auditAnchorStateEl.textContent = action === 'create' ? '正在创建签名锚点' : '正在校验签名锚点';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/auditAnchor', {
	      method: 'POST', headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify({ action, tenantId: Number(auditIntegrityTenantIdInput.value || 0), limit: Number(auditIntegrityLimitInput.value || 50) }),
	    });
	    const body = await res.json();
	    if (!res.ok || (body.code !== 200 && body.code !== 201)) throw new Error(body.msg || 'HTTP ' + res.status);
	    const result = (body.data || {}).result || {};
	    await Promise.all([loadAuditIntegrity(), loadAuditAnchors()]);
	    auditAnchorStateEl.className = Number(result.failedCheckpoints || result.failedArtifacts || 0) > 0 ? 'bad' : 'good';
	    auditAnchorStateEl.textContent = action === 'create'
		      ? '已创建 ' + fmt(result.createdCheckpoints || 0) + ' 条，复用 ' + fmt(result.existingCheckpoints || 0) + ' 条，历史回填 ' + fmt(result.backfilledCheckpoints || 0) + ' 条，远端锁定 ' + fmt(result.remoteExportedArtifacts || 0) + ' 条，审计 #' + fmt(result.operationId || 0)
	      : '已校验 ' + fmt(result.scannedCheckpoints || 0) + ' 条，通过 ' + fmt(result.passedCheckpoints || 0) + '，异常 ' + fmt(result.failedCheckpoints || 0) + '，审计 #' + fmt(result.operationId || 0);
	  } catch (err) {
	    auditAnchorStateEl.className = 'bad';
	    auditAnchorStateEl.textContent = err.message || String(err);
	  } finally {
	    button.disabled = !hasPlatformPermission('platform.audit.manage');
	  }
	}
    async function loadOperations() {
      const params = operationParams(30);
      try {
        const res = await fetch('/dashboard/saasAdmin/operations?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderOperations(data.operations || [], data.summary || {});
      } catch (err) {
        operationsEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看操作记录</td></tr>';
        document.getElementById('operationCount').textContent = '';
      }
    }
	    async function loadBillingEvents() {
	      const params = billingParams(30);
	      try {
	        const res = await fetch('/dashboard/saasAdmin/billingEvents?' + params.toString(), { headers: authHeader() });
	        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
		const data = body.data || {};
		renderBillingEvents(data.billingEvents || [], data.summary || {});
        loadBillingReconciliation();
      } catch (err) {
	        billingEventsEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看账单事件</td></tr>';
        document.getElementById('billingCount').textContent = '';
        billingReconciliationEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看账单对账</td></tr>';
        document.getElementById('billingReconciliationCount').textContent = '';
        billingFollowUpsEl.innerHTML = '<tr><td colspan="7" class="empty">仅平台管理员可查看账单跟进任务</td></tr>';
        document.getElementById('billingFollowUpCount').textContent = '';
        billingFollowUpOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看账单负责人工作台</td></tr>';
        document.getElementById('billingFollowOwnerCount').textContent = '';
	      }
	    }
    async function loadBillingReconciliation() {
      const params = billingReconciliationParams(30);
      try {
        const res = await fetch('/dashboard/saasAdmin/billingReconciliation?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderBillingReconciliation(data.items || [], data.summary || {});
      } catch (err) {
        billingReconciliationEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看账单对账</td></tr>';
        document.getElementById('billingReconciliationCount').textContent = '';
      }
    }
    async function loadBillingReconciliationFollowUps() {
      const params = billingFollowParams(50);
      try {
        const res = await fetch('/dashboard/saasAdmin/billingReconciliationFollowUps?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderBillingReconciliationFollowUps(data.followUps || [], data.summary || {});
      } catch (err) {
        billingFollowUpsEl.innerHTML = '<tr><td colspan="7" class="empty">仅平台管理员可查看账单跟进任务</td></tr>';
        document.getElementById('billingFollowUpCount').textContent = '';
      }
    }
    async function loadBillingReconciliationFollowUpOwners() {
      const params = billingFollowParams(1000);
      try {
        const res = await fetch('/dashboard/saasAdmin/billingReconciliationFollowUpOwners?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderBillingReconciliationFollowUpOwners(data.owners || [], data.summary || {});
      } catch (err) {
        billingFollowUpOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看账单负责人工作台</td></tr>';
        document.getElementById('billingFollowOwnerCount').textContent = '';
      }
    }
    async function followUpBillingReconciliation(billingEventId) {
      const id = Number(billingEventId);
      if (!id) {
        statusEl.textContent = '缺少账单事件 ID';
        return;
      }
      const remark = window.prompt('账单跟进备注', '已跟进对账异常');
      if (remark === null) return;
      statusEl.textContent = '正在记录账单跟进';
      try {
        const res = await fetch('/dashboard/saasAdmin/billingReconciliationFollowUp', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({
            billingEventId: id,
            status: 'resolved',
            remark: (remark || '').trim() || '账单对账跟进',
          }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        statusEl.textContent = '账单跟进已记录，操作 ID：' + fmt(data.operationId);
        loadBillingReconciliation();
        loadBillingReconciliationFollowUps();
        loadBillingReconciliationFollowUpOwners();
        loadOperations();
        loadDailyReport();
        loadOperationQueue();
        loadOperationQueueOwners();
      } catch (err) {
	        statusEl.textContent = err.message || String(err);
	      }
	    }
    async function bulkCloseBillingReconciliationFollowUps() {
      statusEl.textContent = '正在批量关闭账单跟进任务';
      const payload = {
        filterStatus: billingFollowStatusInput.value.trim(),
        dueState: billingFollowDueStateInput.value || 'all',
        owner: billingFollowOwnerInput.value.trim(),
        keyword: billingFollowKeywordInput.value.trim(),
        limit: 50,
        closeStatus: 'resolved',
        remark: '页面批量关闭账单跟进任务',
      };
      if (tenantInput.value.trim()) payload.tenantId = Number(tenantInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '已批量关闭 ' + fmt((body.data || {}).closedCount || 0) + ' 条账单跟进任务';
        loadBillingReconciliationFollowUps();
        loadBillingReconciliationFollowUpOwners();
        loadOperations();
        loadDailyReport();
        loadOperationQueue();
        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
	    async function loadDailyReport() {
	      const params = dailyReportParams(20);
	      try {
	        const res = await fetch('/dashboard/saasAdmin/dailyReport?' + params.toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        renderDailyReport(body.data || {});
	      } catch (err) {
	        dailyReportHintEl.textContent = '';
	        dailyReportEl.innerHTML = '<div class="tile"><div class="label">运营日报</div><div class="value">仅平台管理员可查看</div></div>';
	        clearDailyReportDetails('仅平台管理员可查看');
	      }
	    }
	    async function loadBusinessMetrics() {
	      const params = businessMetricsParams();
	      try {
	        const res = await fetch('/dashboard/saasAdmin/businessMetrics?' + params.toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        renderBusinessMetrics(body.data || {});
	      } catch (err) {
	        businessMetricsHintEl.textContent = '';
	        businessMetricsEl.innerHTML = '<div class="tile"><div class="label">经营指标</div><div class="value">仅平台管理员可查看</div></div>';
	        businessPackagesEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看</td></tr>';
	      }
	    }
	async function loadBusinessTrends() {
	  const params = businessTrendsParams();
	  try {
	    const res = await fetch('/dashboard/saasAdmin/businessTrends?' + params.toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        renderBusinessTrends(body.data || {});
	      } catch (err) {
	        businessTrendsHintEl.textContent = '';
	        businessTrendsEl.innerHTML = '<div class="tile"><div class="label">经营趋势</div><div class="value">仅平台管理员可查看</div></div>';
	        businessTrendMonthsEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看</td></tr>';
	    businessRenewalFunnelEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看</td></tr>';
	  }
	}
	async function loadOperationQueue() {
	  const params = operationQueueParams(50);
	  try {
	    const res = await fetch('/dashboard/saasAdmin/operationQueue?' + params.toString(), { headers: authHeader() });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    renderOperationQueue(body.data || {});
	  } catch (err) {
	    operationQueueCountEl.textContent = '';
	    operationQueueEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看运营待办队列</td></tr>';
	  }
	}
	async function loadOperationQueueOwners() {
	  const params = operationQueueParams(50);
	  try {
	    const res = await fetch('/dashboard/saasAdmin/operationQueueOwners?' + params.toString(), { headers: authHeader() });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    renderOperationQueueOwners(body.data || {});
	  } catch (err) {
	    operationQueueOwnerCountEl.textContent = '';
	    operationQueueOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看运营待办负责人工作台</td></tr>';
	  }
	}
	async function loadOperationQueueAssignments() {
	  const params = operationQueueAssignmentParams(50);
	  try {
	    const res = await fetch('/dashboard/saasAdmin/operationQueueAssignments?' + params.toString(), { headers: authHeader() });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    renderOperationQueueAssignments(body.data || {});
	  } catch (err) {
	    operationQueueAssignmentCountEl.textContent = '';
	    operationQueueAssignmentsEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看运营待办认领记录</td></tr>';
	  }
	}
	async function assignOperationQueue() {
	  return assignOperationQueueWithLimit(50);
	}
	async function assignOperationQueueWithLimit(limit) {
	  const owner = operationQueueAssignOwnerInput.value.trim();
	  if (!owner) {
	    statusEl.textContent = '请填写待办负责人';
	    return;
	  }
	  const params = operationQueueParams(limit || 50);
	  const payload = {
	    owner,
	    status: operationQueueAssignStatusInput.value || 'pending',
	    nextFollowUpAt: operationQueueAssignNextAtInput.value.trim(),
	    remark: operationQueueAssignRemarkInput.value.trim() || '页面运营待办批量分派',
	  };
	  statusEl.textContent = '正在分派运营待办';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/operationQueueAssign?' + params.toString(), {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify(payload),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    statusEl.textContent = '已分派 ' + fmt(data.assignedCount || 0) + ' 条运营待办，跳过 ' + fmt(data.skippedCount || 0) + ' 条';
	    operationQueueOwnerInput.value = owner;
	    customerSuccessOwnerInput.value = owner;
	    billingFollowOwnerInput.value = owner;
	    await loadOperationQueue();
	    await loadOperationQueueOwners();
	    await loadOperationQueueAssignments();
	    await loadCustomerSuccess();
	    await loadCustomerSuccessOwners();
	    await loadRiskFollowUps();
	    await loadRiskFollowUpOwners();
	    await loadBillingReconciliationFollowUps();
	    await loadBillingReconciliationFollowUpOwners();
	    await loadOperations();
	    await loadDailyReport();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
	async function assignNotificationHealthQueue() {
	  const state = notificationHealthStateInput.value || 'all';
	  if (state === 'healthy' || state === 'no_data') {
	    statusEl.textContent = '健康或无数据租户不会生成异常待办';
	    return;
	  }
	  const owner = notificationHealthAssignOwnerInput.value.trim();
	  if (!owner) {
	    statusEl.textContent = '请填写通知健康异常负责人';
	    return;
	  }
	  operationQueueSourceInput.value = 'notification_health';
	  operationQueuePriorityInput.value = state === 'critical' ? 'critical' : (state === 'warning' ? 'high' : 'all');
	  operationQueueOwnerInput.value = '';
	  operationQueueKeywordInput.value = notificationHealthKeywordInput.value.trim();
	  operationQueueAssignOwnerInput.value = owner;
	  operationQueueAssignStatusInput.value = 'pending';
	  operationQueueAssignNextAtInput.value = notificationHealthAssignNextAtInput.value.trim();
	  operationQueueAssignRemarkInput.value = '页面通知健康异常分派';
	  await assignOperationQueueWithLimit(5000);
	}
	async function recoverNotificationHealthQueue() {
	  const params = notificationHealthParams(5000);
	  params.delete('state');
	  statusEl.textContent = '正在核验并结案已恢复的通知健康待办';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/notificationHealthRecovery?' + params.toString(), {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify({ remark: '页面核验通知健康恢复并结案' }),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    statusEl.textContent = '已结案 ' + fmt(data.closedCount || 0) + ' 条恢复待办，仍异常 ' + fmt(data.unhealthyCount || 0) + ' 条，无送达证据 ' + fmt(Number(data.noDataCount || 0) + Number(data.noDeliveryEvidenceCount || 0)) + ' 条';
	    await loadNotificationHealth();
	    await loadOperationQueue();
	    await loadOperationQueueOwners();
	    await loadOperationQueueAssignments();
	    await loadOperations();
	    await loadDailyReport();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
	async function createOperationQueueAssignmentNotifications() {
	  const params = operationQueueAssignmentParams(50);
	  const selectedDueState = operationQueueAssignmentDueStateInput.value || 'all';
	  if (selectedDueState !== 'overdue' && selectedDueState !== 'due_soon') {
	    params.set('dueState', 'overdue');
	  }
	  const payload = {
	    remark: '页面运营待办认领到期提醒',
	  };
	  statusEl.textContent = '正在生成认领到期提醒';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/operationQueueAssignmentNotifications?' + params.toString(), {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify(payload),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    statusEl.textContent = '已生成 ' + fmt(data.enqueuedCount || 0) + ' 条认领到期提醒，重复跳过 ' + fmt(data.skippedExistingCount || 0) + ' 条';
	    await loadOperationQueueAssignments();
	    await loadNotifications();
	    await loadOperations();
	    await loadDailyReport();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
	async function closeOperationQueueAssignment(operationId, closeStatus) {
	  const id = Number(operationId || 0);
	  if (!id) {
	    statusEl.textContent = '认领操作 ID 无效';
	    return;
	  }
	  const actionText = closeStatus === 'ignored' ? '忽略' : '完成';
	  if (!window.confirm('确认' + actionText + '这条运营待办认领？')) return;
	  statusEl.textContent = '正在' + actionText + '运营待办认领';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/operationQueueAssignmentClose', {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify({
	        operationId: id,
	        closeStatus,
	        remark: '页面' + actionText + '运营待办认领',
	      }),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    statusEl.textContent = data.alreadyClosed ? '该认领已关闭' : ('已' + actionText + '运营待办认领');
	    await loadOperationQueue();
	    await loadOperationQueueOwners();
	    await loadOperationQueueAssignments();
	    await loadOperations();
	    await loadDailyReport();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
	async function loadRenewalForecast() {
	  const params = renewalForecastParams();
	  try {
	    const res = await fetch('/dashboard/saasAdmin/renewalForecast?' + params.toString(), { headers: authHeader() });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    renderRenewalForecast(body.data || {});
	  } catch (err) {
	    renewalForecastHintEl.textContent = '';
	    renewalForecastEl.innerHTML = '<div class="tile"><div class="label">续费预测</div><div class="value">仅平台管理员可查看</div></div>';
	    renewalForecastBucketsEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看</td></tr>';
	    renewalForecastOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看</td></tr>';
	    renewalForecastTenantsEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看</td></tr>';
	  }
	}
	async function assignRenewalForecast() {
	  const owner = renewalForecastAssignOwnerInput.value.trim();
	  if (!owner) {
	    statusEl.textContent = '请填写续费预测分派负责人';
	    return;
	  }
	  const params = renewalForecastParams();
	  const payload = {
	    status: renewalForecastAssignStatusInput.value || 'renewal_pending',
	    owner,
	    nextFollowUpAt: renewalForecastAssignNextAtInput.value.trim(),
	    remark: renewalForecastAssignRemarkInput.value.trim() || '页面续费预测批量分派',
	  };
	  statusEl.textContent = '正在分派续费预测客户';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/renewalForecastAssign?' + params.toString(), {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify(payload),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    statusEl.textContent = '已分派预测客户 ' + fmt(data.assignedCount || 0) + ' / 命中 ' + fmt(data.matchedCount || 0);
	    renewalForecastOwnerInput.value = owner;
	    customerSuccessOwnerInput.value = owner;
	    await loadRenewalForecast();
	    await loadCustomerSuccess();
	    await loadCustomerSuccessOwners();
	    await loadOperationQueue();
	    await loadOperationQueueOwners();
	    await loadRiskFollowUps();
	    await loadRiskFollowUpOwners();
	    await loadDailyReport();
	    await loadOperations();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
	async function createRenewalForecastTasks() {
	  const params = renewalForecastParams();
	  const payload = {
	    months: Number(renewalForecastTaskMonthsInput.value || 12),
	    externalOrderNoPrefix: renewalForecastTaskOrderPrefixInput.value.trim(),
	    remark: '页面续费预测生成任务',
	    forceCreate: renewalForecastTaskForceInput.checked,
	  };
	  statusEl.textContent = '正在生成预测续费任务';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/renewalForecastTasks?' + params.toString(), {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify(payload),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    statusEl.textContent = '已生成预测续费任务 ' + fmt(data.createdCount || 0) + ' 条，跳过已有 ' + fmt(data.skippedExistingCount || 0) + ' 条';
	    await loadRenewalForecast();
	    await loadBusinessTrends();
	    await loadOperationQueue();
	    await loadOperationQueueOwners();
	    await loadRenewalTasks();
	    await loadAdminTasks();
	    await loadOperations();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
	async function createRenewalForecastNotifications() {
	  const params = renewalForecastParams();
	  const payload = {
	    channel: 'webhook',
	    reminderDays: Number(renewalForecastReminderDaysInput.value || renewalForecastDaysInput.value || 30),
	    maxAttempts: Number(renewalForecastMaxAttemptsInput.value || 3),
	    remark: renewalForecastNotifyRemarkInput.value.trim() || '页面续费预测生成提醒',
	    forceCreate: renewalForecastForceNotifyInput.checked,
	  };
	  statusEl.textContent = '正在生成预测续费提醒';
	  try {
	    const res = await fetch('/dashboard/saasAdmin/renewalForecastNotifications?' + params.toString(), {
	      method: 'POST',
	      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify(payload),
	    });
	    const body = await res.json();
	    if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	    const data = body.data || {};
	    statusEl.textContent = '已生成预测续费提醒 ' + fmt(data.enqueuedCount || 0) + ' 条，跳过已有 ' + fmt(data.skippedExistingCount || 0) + ' 条';
	    await loadRenewalForecast();
	    await loadNotifications();
	    await loadDailyReport();
	    await loadOperationQueue();
	    await loadOperationQueueOwners();
	    await loadOperations();
	  } catch (err) {
	    statusEl.textContent = err.message || String(err);
	  }
	}
	async function loadAlerts() {
      const params = alertParams(30);
      try {
        const res = await fetch('/dashboard/saasAdmin/alerts?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderAlerts(data.alerts || [], data.summary || {}, data.returnedCount);
      } catch (err) {
        alertsEl.innerHTML = '<tr><td colspan="6" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        document.getElementById('alertCount').textContent = '';
      }
    }
    async function loadNotificationPolicies() {
      const params = notificationPolicyParams(100);
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationPolicies?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderNotificationWebhookSecurity(data.webhookSecurity || {});
        renderNotificationCredentialProtection(data.credentialProtection || {});
        renderNotificationPolicies(data.policies || [], data.summary || {}, data.returnedCount);
      } catch (err) {
        notificationPoliciesEl.innerHTML = '<tr><td colspan="6" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        notificationPolicyCountEl.textContent = '';
        notificationPolicySecurityEl.textContent = '加载失败';
        notificationCredentialProtectionEl.textContent = '加载失败';
        rotateNotificationCredentialsButton.disabled = true;
      }
    }
    async function loadNotificationHealth() {
      const params = notificationHealthParams(100);
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationHealth?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderNotificationHealth(body.data || {});
      } catch (err) {
        notificationHealthTenantsEl.innerHTML = '<tr><td colspan="8" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        notificationFailureReasonsEl.innerHTML = '<tr><td colspan="4" class="empty">暂无数据</td></tr>';
        notificationHealthCountEl.textContent = '';
        notificationFailureReasonCountEl.textContent = '';
      }
    }
    async function loadNotificationSlo() {
      const params = notificationSloParams(100);
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationSlo?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderNotificationSlo(body.data || {});
      } catch (err) {
        notificationSloDaysEl.innerHTML = '<tr><td colspan="6" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        notificationSloTenantsEl.innerHTML = '<tr><td colspan="7" class="empty">暂无数据</td></tr>';
        notificationSloCountEl.textContent = '';
        notificationSloTenantCountEl.textContent = '';
      }
    }
    async function loadNotificationPolicy(tenantId) {
      const id = Number(tenantId || notificationPolicyTenantInput.value || 0);
      if (!id) {
        statusEl.textContent = '请选择策略租户';
        return false;
      }
      try {
        const params = new URLSearchParams({ tenantId: String(id), channel: 'webhook' });
        const res = await fetch('/dashboard/saasAdmin/notificationPolicy?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        fillNotificationPolicy(data.policy || {});
        renderNotificationWebhookSecurity(data.webhookSecurity || {});
        renderNotificationCredentialProtection(data.credentialProtection || notificationCredentialProtection);
        statusEl.textContent = '已载入租户 ' + id + ' 通知策略';
        return true;
      } catch (err) {
        statusEl.textContent = err.message || String(err);
        return false;
      }
    }
    function notificationPolicyPayload() {
      const payload = {
        tenantId: Number(notificationPolicyTenantInput.value || 0),
        channel: 'webhook',
        enabled: notificationPolicyEnabledInput.checked,
        webhookUrl: notificationPolicyWebhookUrlInput.value.trim(),
        clearWebhookSecret: notificationPolicyClearSecretInput.checked,
        webhookTimeoutSeconds: Number(notificationPolicyTimeoutInput.value || 5),
        webhookRetryAttempts: Number(notificationPolicyHttpAttemptsInput.value || 1),
        webhookRetryDelayMs: Number(notificationPolicyHttpDelayInput.value || 0),
        webhookTitleTemplate: notificationPolicyTitleTemplateInput.value.trim(),
        webhookBodyTemplate: notificationPolicyBodyTemplateInput.value.trim(),
        notificationMaxAttempts: Number(notificationPolicyMaxAttemptsInput.value || 3),
        notificationRetryDelaySeconds: Number(notificationPolicyRetryDelayInput.value || 0),
        minimumSeverity: notificationPolicyMinimumSeverityInput.value,
        alertTypes: selectedNotificationPolicyAlertTypes(),
        quietHoursEnabled: notificationPolicyQuietEnabledInput.checked,
        quietHoursStart: notificationPolicyQuietStartInput.value,
        quietHoursEnd: notificationPolicyQuietEndInput.value,
        timezone: notificationPolicyTimezoneInput.value.trim(),
        hourlyLimit: Number(notificationPolicyHourlyLimitInput.value || 0),
        remark: notificationPolicyRemarkInput.value.trim() || '平台代管通知策略',
      };
      if (notificationPolicyWebhookSecretInput.value.trim()) payload.webhookSecret = notificationPolicyWebhookSecretInput.value.trim();
      return payload;
    }
    async function saveNotificationPolicy() {
      const tenantId = Number(notificationPolicyTenantInput.value || 0);
      if (!tenantId) {
        statusEl.textContent = '请选择策略租户';
        return;
      }
      if (notificationPolicyLoadedTenantId !== tenantId) {
        if (await loadNotificationPolicy(tenantId)) statusEl.textContent = '策略已载入，请确认后保存';
        return;
      }
      statusEl.textContent = '正在保存通知策略';
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationPolicy', {
          method: 'PUT',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(notificationPolicyPayload()),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        fillNotificationPolicy(data.policy || {});
        renderNotificationWebhookSecurity(data.webhookSecurity || {});
        renderNotificationCredentialProtection(data.credentialProtection || notificationCredentialProtection);
        statusEl.textContent = '通知策略已保存';
        await loadNotificationPolicies();
		void loadTenantReadiness();
        await loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function testNotificationPolicy() {
      const tenantId = Number(notificationPolicyTenantInput.value || 0);
      if (!tenantId) {
        statusEl.textContent = '请选择策略租户';
        return;
      }
      statusEl.textContent = '正在生成测试通知';
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationPolicyTest', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({
            tenantId,
            channel: 'webhook',
            remark: notificationPolicyRemarkInput.value.trim() || '页面生成通知策略测试消息',
          }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '测试通知已进入 Outbox';
        await loadNotifications();
        await loadNotificationHealth();
        await loadNotificationSlo();
        await loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function rotateNotificationCredentials() {
      const tenantId = Number(notificationCredentialTenantInput.value || 0);
      const limit = Number(notificationCredentialLimitInput.value || 0);
      if (tenantId < 0 || limit < 1 || limit > 1000) {
        statusEl.textContent = '轮换租户 ID 必须为非负整数，批次必须为 1 到 1000';
        return;
      }
      const scope = tenantId > 0 ? '租户 ' + tenantId : '全部租户';
      const pending = Number(notificationCredentialProtection.rotationRequiredCount || 0);
      if (!window.confirm('确认轮换' + scope + '的通知凭据？当前待轮换 ' + pending + ' 条，本批最多 ' + limit + ' 条。')) return;
      rotateNotificationCredentialsButton.disabled = true;
      statusEl.textContent = '正在轮换通知凭据';
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationCredentialRotation', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ tenantId, limit, remark: '总后台轮换通知 webhook 凭据' }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderNotificationCredentialProtection(data.credentialProtection || {});
        const result = data.rotation || {};
        statusEl.textContent = '凭据轮换完成：扫描 ' + (result.scannedCount || 0) + '，已轮换 ' + (result.rotatedCount || 0);
        await loadNotificationPolicies();
		void loadTenantReadiness();
        await loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
        renderNotificationCredentialProtection(notificationCredentialProtection);
      }
    }
    async function loadNotifications() {
      const params = notificationParams(30);
      try {
        const res = await fetch('/dashboard/saasAdmin/notifications?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderNotifications(data.notifications || [], data.summary || {}, data.returnedCount);
      } catch (err) {
        notificationsEl.innerHTML = '<tr><td colspan="6" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        document.getElementById('notificationCount').textContent = '';
      }
    }
    async function loadRisk() {
      const params = riskParams(50);
      try {
        const res = await fetch('/dashboard/saasAdmin/risk?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderRisk(data.riskTenants || [], data.summary || {});
      } catch (err) {
        riskTenantsEl.innerHTML = '<tr><td colspan="7" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        document.getElementById('riskCount').textContent = '';
      }
    }
	    async function loadCustomerSuccess() {
	      const params = customerSuccessParams(30);
	      try {
	        const res = await fetch('/dashboard/saasAdmin/customerSuccess?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderCustomerSuccess(body.data || {});
      } catch (err) {
        customerSuccessEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看客户成功队列</td></tr>';
	        customerSuccessCountEl.textContent = '';
	      }
	    }
	    async function loadCustomerSuccessOwners() {
	      const params = customerSuccessParams(50);
	      try {
	        const res = await fetch('/dashboard/saasAdmin/customerSuccessOwners?' + params.toString(), { headers: authHeader() });
	        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        renderCustomerSuccessOwners(body.data || {});
	      } catch (err) {
	        customerSuccessOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看客户成功负责人工作台</td></tr>';
	        customerSuccessOwnerCountEl.textContent = '';
	      }
	    }
	    async function assignCustomerSuccess() {
	      const owner = customerSuccessAssignOwnerInput.value.trim();
      if (!owner) {
        statusEl.textContent = '请填写分派负责人';
        return;
      }
      const params = customerSuccessParams(100);
      const payload = {
        status: 'pending',
        owner,
        nextFollowUpAt: customerSuccessAssignNextAtInput.value.trim(),
        remark: customerSuccessAssignRemarkInput.value.trim() || '页面批量分派客户成功队列',
      };
      statusEl.textContent = '正在分派客户成功队列';
      try {
        const res = await fetch('/dashboard/saasAdmin/customerSuccessAssign?' + params.toString(), {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        statusEl.textContent = '已分派 ' + fmt(data.assignedCount || 0) + ' 个租户给 ' + owner;
	        customerSuccessOwnerInput.value = owner;
	        await loadCustomerSuccess();
	        await loadCustomerSuccessOwners();
	        await loadOperationQueue();
	        await loadOperationQueueOwners();
	        await loadRiskFollowUps();
	        await loadRiskFollowUpOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function createCustomerSuccessRenewalTasks() {
      const params = customerSuccessParams(100);
      const payload = {
        months: Number(customerSuccessRenewMonthsInput.value || 12),
        amount: customerSuccessRenewAmountInput.value.trim(),
        remark: customerSuccessRenewRemarkInput.value.trim() || '页面批量生成客户成功续费任务',
      };
      statusEl.textContent = '正在生成客户成功续费任务';
      try {
        const res = await fetch('/dashboard/saasAdmin/customerSuccessRenewalTasks?' + params.toString(), {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        statusEl.textContent = '已生成 ' + fmt(data.createdCount || 0) + ' 条续费任务，跳过已有 ' + fmt(data.skippedExistingCount || 0) + ' 条';
	        await loadCustomerSuccess();
	        await loadCustomerSuccessOwners();
	        await loadOperationQueue();
	        await loadOperationQueueOwners();
	        await loadAdminTasks();
	        await loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function createCustomerSuccessRenewalNotifications() {
      const params = customerSuccessParams(100);
      const payload = {
        channel: 'webhook',
        reminderDays: Number(customerSuccessRenewalReminderDaysInput.value || expiringInput.value || 30),
        maxAttempts: Number(customerSuccessRenewalMaxAttemptsInput.value || 3),
        remark: customerSuccessRenewalNotifyRemarkInput.value.trim() || '页面批量生成客户成功续费提醒',
        forceCreate: customerSuccessRenewalForceNotifyInput.checked,
      };
      statusEl.textContent = '正在生成客户成功续费提醒';
      try {
        const res = await fetch('/dashboard/saasAdmin/customerSuccessRenewalNotifications?' + params.toString(), {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
	        statusEl.textContent = '已生成 ' + fmt(data.enqueuedCount || 0) + ' 条续费提醒，跳过已有 ' + fmt(data.skippedExistingCount || 0) + ' 条';
	        await loadCustomerSuccess();
	        await loadCustomerSuccessOwners();
	        await loadOperationQueue();
	        await loadOperationQueueOwners();
	        await loadNotifications();
        await loadOperations();
        await loadDailyReport();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function loadRiskFollowUps() {
      const params = riskTaskParams(50);
      try {
        const res = await fetch('/dashboard/saasAdmin/riskFollowUps?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderRiskFollowUps(data.followUps || [], data.summary || {});
      } catch (err) {
        riskFollowUpsEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看风险跟进任务</td></tr>';
        document.getElementById('riskTaskCount').textContent = '';
      }
    }
    async function loadRiskFollowUpOwners() {
      const params = riskTaskParams(1000);
      try {
        const res = await fetch('/dashboard/saasAdmin/riskFollowUpOwners?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderRiskFollowUpOwners(data.owners || [], data.summary || {});
      } catch (err) {
        riskFollowUpOwnersEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看负责人工作台</td></tr>';
        document.getElementById('riskOwnerCount').textContent = '';
      }
    }
    async function bulkCloseRiskFollowUps() {
      statusEl.textContent = '正在批量关闭风险跟进任务';
      const payload = {
        filterStatus: riskTaskStatusInput.value.trim(),
        dueState: riskTaskDueStateInput.value || 'all',
        owner: riskTaskOwnerInput.value.trim(),
        keyword: riskTaskKeywordInput.value.trim(),
        limit: 50,
        closeStatus: 'resolved',
        remark: '页面批量关闭风险跟进任务',
      };
      if (tenantInput.value.trim()) payload.tenantId = Number(tenantInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/riskFollowUpBulkClose', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '已批量关闭 ' + fmt((body.data || {}).closedCount || 0) + ' 条风险跟进任务';
	        loadRisk();
	        loadRiskFollowUps();
	        loadRiskFollowUpOwners();
	        loadOperations();
	        loadDailyReport();
	        loadOperationQueue();
	        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function recordRiskFollowUp(tenantId) {
      const id = Number(tenantId);
      if (!id) {
        statusEl.textContent = '缺少租户 ID';
        return;
      }
      statusEl.textContent = '正在记录风险跟进';
      try {
        const res = await fetch('/dashboard/saasAdmin/riskFollowUp', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({
            tenantId: id,
            status: riskFollowStatusInput.value.trim(),
            owner: riskFollowOwnerInput.value.trim(),
            nextFollowUpAt: riskFollowNextAtInput.value.trim(),
            remark: riskFollowRemarkInput.value.trim(),
          }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        tenantInput.value = String(id);
        statusEl.textContent = '风险跟进已记录，操作 ID：' + fmt(data.operationId);
	        loadRisk();
	        loadRiskFollowUps();
	        loadRiskFollowUpOwners();
	        loadOperations();
	        loadTenantDetail({ tenantId: id });
	        loadDailyReport();
	        loadOperationQueue();
	        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function resolveAlert(tenantId, metric, alertType, periodKey) {
      statusEl.textContent = '正在解决告警';
      try {
        const res = await fetch('/dashboard/saasAdmin/alertResolve', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ tenantId: Number(tenantId), metric, alertType, periodKey, remark: '页面解决告警' }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = (body.data || {}).resolved ? '告警已解决' : '没有打开的匹配告警';
        loadOverview();
        loadNotifications();
        loadOperationQueue();
        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function bulkResolveAlerts() {
      statusEl.textContent = '正在批量解决告警';
      const payload = {
        metric: alertMetricInput.value.trim(),
        alertType: alertTypeInput.value.trim(),
        limit: 50,
        remark: '页面批量解决告警',
      };
      if (tenantInput.value.trim()) payload.tenantId = Number(tenantInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/alertBulkResolve', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '已批量解决 ' + fmt((body.data || {}).resolvedCount || 0) + ' 条告警';
        loadOverview();
        loadAlerts();
        loadOperations();
        loadNotifications();
        loadOperationQueue();
        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function retryNotification(notificationId) {
      statusEl.textContent = '正在重新排队通知';
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationRetry', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ notificationId: Number(notificationId), remark: '页面重试通知' }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = (body.data || {}).retried ? '通知已重新排队' : '通知未重试';
	        loadNotifications();
	        loadNotificationHealth();
	        loadNotificationSlo();
	        loadOperations();
	        loadDailyReport();
	        loadOperationQueue();
	        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function closeNotification(notificationId) {
      statusEl.textContent = '正在关闭通知';
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationClose', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ notificationId: Number(notificationId), remark: '页面关闭通知' }),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = (body.data || {}).closed ? '通知已关闭' : '通知未关闭';
        loadNotifications();
        loadNotificationHealth();
        loadOperations();
        loadOverview();
        loadDailyReport();
        loadOperationQueue();
        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function bulkRetryNotifications() {
      statusEl.textContent = '正在批量重新排队通知';
      const payload = {
        status: notificationStatusInput.value.trim() || 'all',
        channel: notificationChannelInput.value.trim(),
        keyword: notificationKeywordInput.value.trim(),
        limit: 50,
        remark: '页面批量重试通知',
      };
      if (tenantInput.value.trim()) payload.tenantId = Number(tenantInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationBulkRetry', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '已批量重新排队 ' + fmt((body.data || {}).retriedCount || 0) + ' 条通知';
	        loadNotifications();
	        loadNotificationHealth();
	        loadOperations();
	        loadOverview();
	        loadDailyReport();
	        loadOperationQueue();
	        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function bulkCloseNotifications() {
      statusEl.textContent = '正在批量关闭通知';
      const rawStatus = notificationStatusInput.value.trim();
      const closeStatus = rawStatus === 'pending' || rawStatus === 'failed' ? rawStatus : 'all';
      const payload = {
        status: closeStatus,
        channel: notificationChannelInput.value.trim(),
        keyword: notificationKeywordInput.value.trim(),
        limit: 50,
        remark: '页面批量关闭通知',
      };
      if (tenantInput.value.trim()) payload.tenantId = Number(tenantInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/notificationBulkClose', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '已批量关闭 ' + fmt((body.data || {}).closedCount || 0) + ' 条通知';
        loadNotifications();
        loadNotificationHealth();
        loadOperations();
        loadOverview();
        loadDailyReport();
        loadOperationQueue();
        loadOperationQueueOwners();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function loadTenantDetail(seed) {
      const tenantId = tenantInput.value.trim() || (seed && seed.tenantId ? String(seed.tenantId) : '');
      if (!tenantId || tenantId === '0') {
        renderTenantDetail(null);
        return;
      }
      const params = new URLSearchParams({
        tenantId,
        expiringDays: expiringInput.value || '30',
        operationLimit: '5',
      });
      try {
        const res = await fetch('/dashboard/saasAdmin/tenant?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        renderTenantDetail(data);
        loadTenantUsage(tenantId);
		if (data.canPlatformScope) {
		  loadTenantLifecycle(tenantId);
		} else {
          tenantLifecycleCountEl.textContent = '';
          tenantLifecycleEl.innerHTML = '<tr><td colspan="5" class="empty">仅平台管理员可查看</td></tr>';
        }
      } catch (err) {
        tenantDetailHintEl.textContent = '';
        tenantDetailEl.innerHTML = '<div class="tile"><div class="label">租户</div><div class="value">-</div></div>' +
          '<div class="tile"><div class="label">详情</div><div class="value">' + esc(err.message || String(err)) + '</div></div>';
        renderUsageMetrics([]);
        renderTenantLifecycle(null);
      }
    }
    async function loadTenantLifecycle(seedTenantId) {
      const tenantId = String(seedTenantId || tenantInput.value.trim() || '').trim();
      if (!tenantId || tenantId === '0') {
        renderTenantLifecycle(null);
        return;
      }
	      const params = tenantLifecycleParams(30);
	      params.set('tenantId', tenantId);
	      try {
	        const res = await fetch('/dashboard/saasAdmin/tenantLifecycle?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderTenantLifecycle(body.data || {});
      } catch (err) {
        tenantLifecycleCountEl.textContent = '';
        tenantLifecycleEl.innerHTML = '<tr><td colspan="5" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
      }
    }
    async function loadTenantUsage(tenantId) {
      const params = new URLSearchParams({
        tenantId,
        expiringDays: expiringInput.value || '30',
      });
      try {
        const res = await fetch('/dashboard/saasAdmin/usage?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderUsageMetrics((body.data || {}).usageMetrics || []);
      } catch (err) {
        usageMetricsEl.innerHTML = '<tr><td colspan="6" class="empty">' + esc(err.message || String(err)) + '</td></tr>';
        document.getElementById('usageCount').textContent = '';
      }
    }
    async function savePackage() {
      const code = editPackageCodeInput.value.trim();
      const name = editPackageNameInput.value.trim();
      if (!code || !name) {
        statusEl.textContent = '请填写套餐编码和名称';
        return;
      }
      let limits = {};
      try {
        const rawLimits = editPackageLimitsInput.value.trim();
        limits = rawLimits ? JSON.parse(rawLimits) : {};
        if (!limits || Array.isArray(limits) || typeof limits !== 'object') throw new Error('额度 JSON 必须是对象');
      } catch (err) {
        statusEl.textContent = err.message || '额度 JSON 格式错误';
        return;
      }
      const payload = {
        code,
        name,
        description: editPackageDescriptionInput.value.trim(),
        status: Number(editPackageStatusInput.value || '1'),
        limits,
        expectedVersion: Number(editPackageVersionInput.value || '0'),
      };
      const requiresApproval = approvalActionRequired('package.upsert', 0);
      statusEl.textContent = requiresApproval ? '正在提交套餐定义审批' : '正在保存套餐';
      try {
        if (requiresApproval) {
          const approval = await requestHighRiskApproval('package.upsert', payload, '变更平台套餐定义：' + name);
          statusEl.textContent = '套餐定义变更已提交双人审批：' + fmt(approval.requestNo || '');
          renderPackageImpact(((approval.request || {}).impact));
          return;
        }
        const res = await fetch('/dashboard/saasAdmin/package', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const saved = body.data || {};
        statusEl.textContent = '套餐已保存：' + fmt(saved.name || saved.code);
        await loadPackages();
        fillPackageEditor(saved.code || code);
        renderPackageImpact(saved.impact);
        loadOperations();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    function renderTenantStatusApprovalState() {
      const tenantId = Number(statusTenantInput.value.trim() || 0);
      const status = Number(tenantStatusInput.value || '1');
		const actionType = status === 2 ? 'tenant.disable' : (status === 1 ? 'tenant.enable' : '');
      if (actionType && approvalActionRequired(actionType, 0)) {
        tenantStatusButton.textContent = actionType === 'tenant.disable' ? '提交停用审批' : '提交启用审批';
        return;
      }
      tenantStatusButton.textContent = '更新状态';
    }
    async function updateTenantStatus() {
      const tenantId = Number(statusTenantInput.value.trim());
      const status = Number(tenantStatusInput.value || '1');
      if (!tenantId) {
        statusEl.textContent = '请填写租户 ID';
        return;
      }
      statusEl.textContent = '正在更新租户状态';
      try {
	    const payload = { tenantId, status, remark: tenantStatusRemarkInput.value.trim() };
	    const actionType = status === 2 ? 'tenant.disable' : (status === 1 ? 'tenant.enable' : '');
	    if (actionType && approvalActionRequired(actionType, 0)) {
	      const actionLabel = actionType === 'tenant.disable' ? '停用' : '启用';
	      await requestHighRiskApproval(actionType, payload, payload.remark || (actionLabel + '租户 ' + tenantId));
	      statusEl.textContent = '租户' + actionLabel + '已提交审批';
	      return;
	    }
        const res = await fetch('/dashboard/saasAdmin/tenantStatus', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
	      body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        tenantInput.value = String(tenantId);
        statusEl.textContent = '租户状态已更新：' + (status === 1 ? '正常' : '停用');
        loadOverview();
        loadOperations();
        loadAlerts();
        loadBillingEvents();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    function provisionPayload(remark) {
      const tenantName = provisionTenantNameInput.value.trim();
      const adminPhone = provisionAdminPhoneInput.value.trim();
      const password = provisionPasswordInput.value.trim();
      const packageCode = provisionPackageCodeInput.value.trim();
      if (!tenantName || !adminPhone || !password || !packageCode) {
        statusEl.textContent = '请填写租户名称、管理员手机、初始密码并选择套餐';
        return null;
      }
      return {
        tenantName,
        adminPhone,
        adminName: provisionAdminNameInput.value.trim(),
        password,
        packageCode,
        expiresAt: provisionExpiresInput.value.trim(),
        configCopyMode: 'missing',
        remark,
      };
    }
    async function provisionTenant() {
      const payload = provisionPayload('页面平台开户');
      if (!payload) return;
      const requiresApproval = approvalActionRequired('tenant.provision', 0);
      statusEl.textContent = requiresApproval ? '正在提交平台开户审批' : '正在开通租户';
      try {
        if (requiresApproval) {
          const approval = await requestHighRiskApproval('tenant.provision', payload, '开通业务租户：' + payload.tenantName);
          provisionPasswordInput.value = '';
          statusEl.textContent = '平台开户已提交双人审批：' + fmt(approval.requestNo || '');
          return;
        }
        const res = await fetch('/dashboard/saasAdmin/tenantProvision', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const data = body.data || {};
        tenantInput.value = String(data.tenantId || '');
        packageTenantInput.value = String(data.tenantId || '');
        statusTenantInput.value = String(data.tenantId || '');
        renewalTenantInput.value = String(data.tenantId || '');
        provisionPasswordInput.value = '';
        statusEl.textContent = '租户已开通：ID ' + fmt(data.tenantId) + '，管理员 ' + fmt(data.adminPhone);
        loadOverview();
        loadOperations();
        loadBillingEvents();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    function provisionTaskResultText(task) {
      const result = task.result || task.preview || {};
      const parts = [];
      if (result.tenantName) parts.push(fmt(result.tenantName));
      if (result.adminPhone) parts.push('管理员 ' + fmt(result.adminPhone));
      if (result.packageName || result.packageCode) parts.push('套餐 ' + fmt(result.packageName || result.packageCode));
      if (result.expiresAt) parts.push('到期 ' + fmt(result.expiresAt));
      if (result.operationId) parts.push('操作 #' + fmt(result.operationId));
      if (result.metricsRefreshed !== undefined) parts.push('刷新 ' + fmt(result.metricsRefreshed));
      if ((task.request || {}).hasAdminPasswordHash) parts.push('凭据已固化');
      if (result.blocked) parts.push(result.blockReason || '已阻断');
      if (task.lastError) parts.push(task.lastError);
      return parts.length ? parts.join(' / ') : '-';
    }
    function renderProvisionTasks(tasks) {
      tasks = tasks || [];
      provisionTaskCache = tasks.slice();
      if (!tasks.length) {
        provisionTasksEl.innerHTML = '<tr><td colspan="6" class="empty">暂无开户任务</td></tr>';
        provisionTaskSummaryEl.textContent = '创建任务后显示预览';
        return;
      }
      const first = tasks[0];
      provisionTaskSummaryEl.textContent = '最近任务 #' + fmt(first.id) + ' / ' + packageSyncTaskStatusLabel(first.status) + ' / ' + provisionTaskResultText(first);
      provisionTasksEl.innerHTML = tasks.map(item => {
        const result = item.result || item.preview || {};
        const action = item.canApply
          ? '<button type="button" data-action="apply-provision-task" data-task-id="' + esc(item.id) + '" data-task-version="' + esc(item.version || 0) + '">' +
            (approvalActionRequired('tenant.provision', 0) ? '提交审批' : '应用') + '</button>'
          : '-';
        return '<tr>' +
          '<td>#' + esc(item.id) + '<br><span class="subtitle">' + esc(item.createdAt || '-') + '</span></td>' +
          '<td>' + esc(result.tenantName || item.tenantId || '-') + '<br><span class="subtitle">ID ' + esc(item.tenantId || '-') + '</span></td>' +
          '<td>' + esc(result.adminPhone || '-') + '<br><span class="subtitle">' + esc(result.adminName || '-') + '</span></td>' +
          '<td>' + esc(result.packageName || item.packageCode || '-') + '<br><span class="subtitle">' + esc(item.packageCode || '-') + '</span></td>' +
          '<td>' + esc(packageSyncTaskStatusLabel(item.status)) + '<br><span class="subtitle">' + esc(provisionTaskResultText(item)) + '</span></td>' +
          '<td>' + action + '</td>' +
        '</tr>';
      }).join('');
    }
    async function createProvisionTask() {
      const payload = provisionPayload('页面创建平台开户任务');
      if (!payload) return;
      statusEl.textContent = '正在创建开户任务';
      try {
        const res = await fetch('/dashboard/saasAdmin/tenantProvisionTask', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        const data = body.data || {};
        if (data.task) {
          provisionTaskIdInput.value = String(data.task.id || '');
          renderProvisionTasks([data.task]);
        }
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        provisionPasswordInput.value = '';
	        statusEl.textContent = '开户任务已创建：#' + fmt((data.task || {}).id);
	        loadAdminTasks();
	        loadProvisionTasks();
	      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function applyProvisionTask(taskId, taskVersion) {
      taskId = Number(taskId || provisionTaskIdInput.value.trim());
      if (!taskId) {
        statusEl.textContent = '请填写开户任务 ID';
        return;
      }
      const requiresApproval = approvalActionRequired('tenant.provision', 0);
      statusEl.textContent = requiresApproval ? '正在提交开户任务审批' : '正在应用开户任务';
      try {
        if (requiresApproval) {
          let version = Number(taskVersion || 0);
          let task = provisionTaskCache.find(item => Number(item.id) === taskId);
          if (!version && task) version = Number(task.version || 0);
          if (!version) {
            const taskRes = await fetch('/dashboard/saasAdmin/tasks?taskId=' + encodeURIComponent(taskId) + '&limit=1', { headers: authHeader() });
            const taskBody = await taskRes.json();
            if (!taskRes.ok || taskBody.code !== 200) throw new Error(taskBody.msg || 'HTTP ' + taskRes.status);
            task = (((taskBody.data || {}).tasks) || [])[0];
            version = Number((task || {}).version || 0);
          }
          if (!version) throw new Error('开户任务版本缺失，请刷新任务后重试');
          const approval = await requestHighRiskApproval(
            'tenant.provision',
            { taskId, expectedTaskVersion: version },
            '执行平台开户任务 #' + taskId,
          );
          statusEl.textContent = '开户任务已提交双人审批：' + fmt(approval.requestNo || '');
          await loadAdminTasks();
          await loadProvisionTasks();
          return;
        }
        const res = await fetch('/dashboard/saasAdmin/tenantProvisionTaskApply', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify({ taskId }),
        });
        const body = await res.json();
        const data = body.data || {};
        if (data.task) renderProvisionTasks([data.task]);
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const result = data.result || {};
        tenantInput.value = String(result.tenantId || '');
        packageTenantInput.value = String(result.tenantId || '');
        statusTenantInput.value = String(result.tenantId || '');
        renewalTenantInput.value = String(result.tenantId || '');
        statusEl.textContent = '开户任务已应用：#' + fmt((data.task || {}).id);
	        loadOverview();
	        loadOperations();
	        loadBillingEvents();
	        loadAdminTasks();
	        loadProvisionTasks();
	      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function bulkApplyProvisionTasks() {
      const payload = {
        taskType: 'tenant_provision',
        status: 'pending',
        limit: 100,
        remark: '页面批量应用平台开户任务',
      };
      if (adminTaskTenantInput.value.trim()) payload.tenantId = Number(adminTaskTenantInput.value.trim());
      const packageCode = adminTaskPackageCodeInput.value.trim() || provisionPackageCodeInput.value.trim();
      if (packageCode) payload.packageCode = packageCode;
      const requiresApproval = approvalActionRequired('tenant.provision', 0);
      if (!window.confirm(requiresApproval ? '确认按当前筛选为最多 100 条平台开户任务逐条提交双人审批？' : '确认批量应用当前筛选下最多 100 条平台开户任务？')) return;
      statusEl.textContent = requiresApproval ? '正在批量提交平台开户审批' : '正在批量应用平台开户任务';
      try {
        if (requiresApproval) {
          const params = new URLSearchParams({ taskType: 'tenant_provision', status: 'pending', limit: String(payload.limit || 100) });
          if (payload.tenantId) params.set('tenantId', String(payload.tenantId));
          if (payload.packageCode) params.set('packageCode', payload.packageCode);
          const tasksRes = await fetch('/dashboard/saasAdmin/tasks?' + params.toString(), { headers: authHeader() });
          const tasksBody = await tasksRes.json();
          if (!tasksRes.ok || tasksBody.code !== 200) throw new Error(tasksBody.msg || 'HTTP ' + tasksRes.status);
          const tasks = ((tasksBody.data || {}).tasks) || [];
          let submitted = 0;
          const failures = [];
          for (const task of tasks) {
            try {
              await requestHighRiskApproval(
                'tenant.provision',
                { taskId: Number(task.id), expectedTaskVersion: Number(task.version || 0) },
                '批量执行平台开户任务 #' + fmt(task.id),
                { refresh: false, idempotencyKey: 'page:tenant.provision:task:' + fmt(task.id) + ':v' + fmt(task.version || 0) + ':' + Date.now() },
              );
              submitted++;
            } catch (err) {
              failures.push('#' + fmt(task.id) + ' ' + (err.message || String(err)));
            }
          }
          await loadApprovals();
          statusEl.textContent = '已提交 ' + fmt(submitted) + ' 条开户审批，失败 ' + fmt(failures.length) + ' 条' +
            (failures.length ? '：' + failures.slice(0, 3).join('；') : '');
          await loadAdminTasks();
          await loadProvisionTasks();
          return;
        }
        const res = await fetch('/dashboard/saasAdmin/tenantProvisionTaskBulkApply', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        const data = body.data || {};
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        statusEl.textContent = '已批量应用 ' + fmt(data.appliedCount || 0) + ' 条开户任务，阻断 ' + fmt(data.blockedCount || 0) + ' 条，失败 ' + fmt(data.failedCount || 0) + ' 条';
        loadOverview();
        loadOperations();
        loadAdminTasks();
        loadProvisionTasks();
        loadAlerts();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function loadProvisionTasks() {
      const params = new URLSearchParams({ taskType: 'tenant_provision', limit: '10' });
      if (provisionPackageCodeInput.value.trim()) params.set('packageCode', provisionPackageCodeInput.value.trim());
      try {
        const res = await fetch('/dashboard/saasAdmin/tasks?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        renderProvisionTasks((body.data || {}).tasks || []);
      } catch (err) {
        provisionTasksEl.innerHTML = '<tr><td colspan="6" class="empty">仅平台管理员可查看开户任务</td></tr>';
      }
    }
    async function applyRenewal() {
      const payload = renewalPayload('页面续费');
      if (!payload) return;
      const requiresApproval = approvalActionRequired('tenant.renewal', 0);
      statusEl.textContent = requiresApproval ? '正在提交续费审批' : '正在记录续费';
      try {
        if (requiresApproval) {
          const approval = await requestHighRiskApproval(
            'tenant.renewal',
            payload,
            '租户续费：' + fmt(payload.tenantId) + ' / ' + fmt(payload.expiresAt),
          );
          statusEl.textContent = '租户续费已提交双人审批：' + fmt(approval.requestNo || '');
          return;
        }
        const res = await fetch('/dashboard/saasAdmin/tenantRenewal', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        tenantInput.value = String(payload.tenantId);
        packageExpiresInput.value = (body.data || {}).expiresAt || renewalExpiresInput.value.trim();
        statusEl.textContent = '续费已记录，账单事件 ID：' + fmt((body.data || {}).billingEventId);
        loadOverview();
        loadOperations();
        loadBillingEvents();
        loadBusinessMetrics();
        loadBusinessTrends();
        loadRenewalForecast();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    function fillTenantPackageEditor(item) {
      item = item || {};
      packageTenantInput.value = String(item.tenantId || packageTenantInput.value || '');
      packageAssignmentVersionInput.value = String(item.packageVersion || 0);
      if (item.packageCode && packageCache[item.packageCode] && packageCache[item.packageCode].status === 1) {
        packageCodeInput.value = item.packageCode;
      }
      packageExpiresInput.value = item.expiresAt || '';
    }
    async function resolveTenantPackageSnapshot(tenantId) {
      const params = new URLSearchParams({ tenantId: String(tenantId), expiringDays: expiringInput.value || '30', operationLimit: '1' });
      const res = await fetch('/dashboard/saasAdmin/tenant?' + params.toString(), { headers: authHeader() });
      const body = await res.json();
      if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
      const tenant = (body.data || {}).tenant || null;
      if (!tenant || Number(tenant.tenantId || 0) !== tenantId) throw new Error('租户不存在或不可见');
      tenantOverviewCache[String(tenantId)] = tenant;
      packageAssignmentVersionInput.value = String(tenant.packageVersion || 0);
      return tenant;
    }
    async function applyPackage() {
      const tenantId = Number(packageTenantInput.value.trim());
      const packageCode = packageCodeInput.value.trim();
      if (!tenantId || !packageCode) {
        statusEl.textContent = '请填写租户 ID 并选择套餐';
        return;
      }
      statusEl.textContent = '正在校验租户套餐版本';
      try {
        const tenant = await resolveTenantPackageSnapshot(tenantId);
        const payload = {
          tenantId,
          packageCode,
          expiresAt: packageExpiresInput.value.trim(),
          remark: packageAssignmentRemarkInput.value.trim(),
          expectedVersion: Number(tenant.packageVersion || 0),
        };
        const target = packageCache[packageCode] || {};
        const reason = payload.remark || ('变更租户 ' + tenantId + ' 套餐为 ' + (target.name || packageCode));
        const requiresApproval = approvalActionRequired('tenant.package.update', 0);
        statusEl.textContent = requiresApproval ? '正在提交租户套餐分配审批' : '正在应用套餐';
        if (requiresApproval) {
          const approval = await requestHighRiskApproval('tenant.package.update', payload, reason);
          statusEl.textContent = '租户套餐变更已提交双人审批：' + fmt(approval.requestNo || '');
          renderPackageImpact(((approval.request || {}).impact));
          return;
        }
        const res = await fetch('/dashboard/saasAdmin/tenantPackage', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, authHeader()),
          body: JSON.stringify(payload),
        });
        const body = await res.json();
        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
        const result = body.data || {};
        tenantInput.value = String(tenantId);
        packageAssignmentVersionInput.value = String(result.version || payload.expectedVersion + 1);
        statusEl.textContent = result.metricsRefreshPending
          ? '套餐已应用，额度指标刷新已进入补偿处理'
          : '套餐已应用，已刷新 ' + fmt(result.metricsRefreshed) + ' 个额度指标';
        renderPackageImpact(result.impact);
        loadOverview();
        loadOperations();
        loadBillingEvents();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
    async function loadOverview() {
      statusEl.textContent = '加载中';
      const params = new URLSearchParams({
        scope: scopeInput.value || 'tenant',
        expiringDays: expiringInput.value || '30',
        limit: '50',
      });
      if (tenantInput.value.trim()) params.set('tenantId', tenantInput.value.trim());
      if (filterKeywordInput.value.trim()) params.set('keyword', filterKeywordInput.value.trim());
      if (filterPackageCodeInput.value.trim()) params.set('packageCode', filterPackageCodeInput.value.trim());
      if (filterTenantStatusInput.value.trim()) params.set('tenantStatus', filterTenantStatusInput.value.trim());
      if (filterDueStateInput.value && filterDueStateInput.value !== 'all') params.set('dueState', filterDueStateInput.value);
      try {
	        const res = await fetch('/dashboard/saasAdmin/overview?' + params.toString(), { headers: authHeader() });
        const body = await res.json();
	        if (!res.ok || body.code !== 200) throw new Error(body.msg || 'HTTP ' + res.status);
	        const data = body.data || {};
	        scopeInput.querySelector('option[value="platform"]').disabled = !data.canPlatformScope;
	        platformAccessControlled = !!data.canPlatformScope;
	        currentPlatformTenantID = Number(data.platformAdminTenantId || currentPlatformTenantID || 0);
	        renderTenantStatusApprovalState();
	        if (!identityTenantFilterInput.value.trim()) identityTenantFilterInput.value = tenantInput.value.trim() || String(currentPlatformTenantID || '');
	        renderAccessProfile(data.access || {});
	        renderSummary(data.summary || {}, data.tenantPopulation || '');
        renderTenants(data.tenants || []);
        renderMetrics(data.metrics || []);
	        if (hasPlatformPermission('platform.tenants.read')) {
	          loadPackages();
	          if (data.canPlatformScope) loadTenantReadiness();
	        }
        const filters = data.filters || {};
	        const filterText = [filters.keyword, filters.packageCode, filters.tenantStatus ? ('状态 ' + filters.tenantStatus) : '', filters.dueState && filters.dueState !== 'all' ? filters.dueState : ''].filter(Boolean).join(' / ');
	        statusEl.innerHTML = '<span>范围：' + (data.scope === 'platform' ? '全平台' : '当前租户') + '，生成时间：' + esc(data.generatedAt) + (filterText ? '，筛选：' + esc(filterText) : '') + '</span>';
	        if (hasPlatformPermission('platform.overview.read')) {
	          loadBusinessMetrics();
	          loadBusinessTrends();
	          loadDailyReport();
	        }
	        if (hasPlatformPermission('platform.operations.read')) {
	          loadOperationQueue();
	          loadOperationQueueOwners();
	          loadOperationQueueAssignments();
	          loadRenewalForecast();
	          loadRisk();
	          loadCustomerSuccess();
	          loadCustomerSuccessOwners();
	          loadRiskFollowUps();
	          loadRiskFollowUpOwners();
	          loadPackageSyncTasks();
	          loadProvisionTasks();
	          loadAdminTasks();
	        }
	        if (hasPlatformPermission('platform.tenants.read')) loadTenantDetail(data);
	        if (hasPlatformPermission('platform.audit.read')) {
	          loadOperations();
	          loadAuditIntegrity();
	          loadAuditAnchors();
	        }
	        if (hasPlatformPermission('platform.notifications.read')) {
	          loadAlerts();
	          loadNotifications();
	          if (data.canPlatformScope) loadNotificationPolicies();
	          if (data.canPlatformScope) loadNotificationHealth();
	          if (data.canPlatformScope) loadNotificationSlo();
	        }
	        if (hasPlatformPermission('platform.finance.read')) {
	          if (data.canPlatformScope) loadSubscriptions();
	          if (data.canPlatformScope) loadPaymentOrders();
	          if (data.canPlatformScope) loadInvoiceDocuments();
	          if (data.canPlatformScope) loadPaymentSettlementSyncRuns();
	          if (data.canPlatformScope) loadPaymentSettlementBatches();
	          loadBillingEvents();
	          loadBillingReconciliationFollowUps();
	          loadBillingReconciliationFollowUpOwners();
	        }
	        if (data.canPlatformScope) loadAccessProfile();
	      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    }
	    document.getElementById('saveToken').addEventListener('click', () => {
	      localStorage.setItem('mochat_go_saas_admin_token', tokenInput.value.trim());
	      loadOverview();
	    });
		    document.getElementById('reload').addEventListener('click', loadOverview);
		    document.getElementById('applyFilters').addEventListener('click', loadOverview);
		    document.getElementById('loadTenantReadiness').addEventListener('click', loadTenantReadiness);
		    tenantReadinessFilterStateInput.addEventListener('change', loadTenantReadiness);
		    tenantReadinessTenantIdInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadTenantReadiness(); });
		    tenantReadinessKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadTenantReadiness(); });
		    tenantReadinessTenantsEl.addEventListener('click', event => {
		      const button = event.target.closest('button[data-readiness-tenant]');
		      if (button) focusTenantReadinessAction(button);
		    });
		    document.getElementById('newAccessRole').addEventListener('click', resetAccessRoleEditor);
		    document.getElementById('saveAccessRole').addEventListener('click', saveAccessRole);
		    document.getElementById('loadAccessAssignments').addEventListener('click', loadAccessGovernance);
		    accessAssignmentKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadAccessGovernance(); });
		    accessRolesEl.addEventListener('click', event => {
		      const button = event.target.closest('[data-access-role]');
		      if (button) selectAccessRole(button.dataset.accessRole);
		    });
		    accessAssignmentsEl.addEventListener('click', event => {
		      const button = event.target.closest('[data-access-assignment-save]');
		      if (button) saveAccessAssignment(button.dataset.accessAssignmentSave, button.dataset.version);
		    });
		    document.getElementById('loadBrandingProfiles').addEventListener('click', loadBrandingProfiles);
		    document.getElementById('saveBrandingProfile').addEventListener('click', saveBrandingProfile);
		    brandingStatusFilterInput.addEventListener('change', loadBrandingProfiles);
		    brandingTenantFilterInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadBrandingProfiles(); });
		    brandingKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadBrandingProfiles(); });
		    brandingProfilesEl.addEventListener('click', event => {
		      const button = event.target.closest('button[data-branding-tenant]');
		      if (button) selectBrandingProfile(button.dataset.brandingTenant);
		    });
		    [brandingProductNameInput, brandingProductSubtitleInput, brandingLogoUrlInput, brandingLoginBackgroundUrlInput, brandingPrimaryColorInput, brandingAccentColorInput].forEach(input => input.addEventListener('input', renderBrandingPreview));
		    document.getElementById('loadTenantDomains').addEventListener('click', loadTenantDomains);
		    document.getElementById('createTenantDomain').addEventListener('click', createTenantDomain);
		    document.getElementById('loadTenantDomainDeliveryJobs').addEventListener('click', loadTenantDomainDeliveryJobs);
		    tenantDomainDeliveryStatusInput.addEventListener('change', loadTenantDomainDeliveryJobs);
		    tenantDomainStatusFilterInput.addEventListener('change', loadTenantDomains);
		    tenantDomainTenantFilterInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadTenantDomains(); });
		    tenantDomainKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadTenantDomains(); });
		    tenantDomainCreateHostnameInput.addEventListener('keydown', event => { if (event.key === 'Enter') createTenantDomain(); });
		    tenantDomainsEl.addEventListener('click', event => {
		      const copyButton = event.target.closest('button[data-copy-domain]');
		      if (copyButton) { copyTenantDomainValue(copyButton.dataset.copyDomain || ''); return; }
		      const actionButton = event.target.closest('button[data-domain-command]');
		      if (actionButton) { requestTenantDomainAction(actionButton); return; }
		      const deliveryButton = event.target.closest('button[data-domain-delivery-command]');
		      if (deliveryButton) applyTenantDomainDeliveryAction({ action: deliveryButton.dataset.domainDeliveryCommand, domainId: Number(deliveryButton.dataset.domainId || 0) });
		    });
		    tenantDomainDeliveryJobsEl.addEventListener('click', event => {
		      const retryButton = event.target.closest('button[data-domain-delivery-retry]');
		      if (retryButton) applyTenantDomainDeliveryAction({ action: 'retry', jobId: Number(retryButton.dataset.domainDeliveryRetry || 0) });
		    });
		    document.getElementById('cancelTenantDomainAction').addEventListener('click', () => tenantDomainActionDialogEl.close());
		    document.getElementById('confirmTenantDomainAction').addEventListener('click', () => {
		      const action = pendingTenantDomainAction;
		      pendingTenantDomainAction = null;
		      tenantDomainActionDialogEl.close();
		      applyTenantDomainAction(action);
		    });
		    tenantDomainActionDialogEl.addEventListener('close', () => { pendingTenantDomainAction = null; });
		    document.getElementById('loadReleaseReadiness').addEventListener('click', loadReleaseReadiness);
		    document.getElementById('runReleaseGate').addEventListener('click', runReleaseGate);
		    document.getElementById('saveReleaseEvidence').addEventListener('click', saveReleaseEvidence);
		    document.getElementById('cancelReleaseEvidence').addEventListener('click', () => releaseEvidenceDialogEl.close());
		    document.getElementById('saveReleaseEvidenceAction').addEventListener('click', saveReleaseEvidenceAction);
		    document.getElementById('cancelReleaseEvidenceAction').addEventListener('click', () => releaseEvidenceActionDialogEl.close());
		    releaseSourceFingerprintInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadReleaseReadiness(); });
		    releaseVersionInput.addEventListener('keydown', event => { if (event.key === 'Enter') runReleaseGate(); });
		    releaseEvidenceEl.addEventListener('click', event => {
		      const button = event.target.closest('button[data-release-evidence-edit]');
		      if (button) openReleaseEvidenceEditor(button.dataset.releaseEvidenceEdit || '');
		    });
		    releaseEvidenceActionsEl.addEventListener('click', event => {
		      const button = event.target.closest('button[data-release-action-edit]');
		      if (button) openReleaseEvidenceActionEditor(button.dataset.releaseActionEdit || '');
		    });
	    document.getElementById('loadIdentitySecurity').addEventListener('click', loadIdentitySecurity);
	    document.getElementById('saveIdentityPolicy').addEventListener('click', saveIdentityPolicy);
	    identityTenantFilterInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadIdentitySecurity(); });
	    identityUserFilterInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadIdentitySecurity(); });
	    identityKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadIdentitySecurity(); });
	    identitySessionStatusInput.addEventListener('change', loadIdentitySecurity);
	    identityEventRiskInput.addEventListener('change', loadIdentitySecurity);
	    identityUsersEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-identity-user-action]');
	      if (button) updateIdentityUser(button);
	    });
	    identitySessionsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-identity-session-revoke]');
	      if (button) revokeIdentitySession(button);
	    });
	    identityIncidentsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-identity-incident-action]');
	      if (button) updateIdentityIncident(button);
	    });
	    document.getElementById('loadSystemHealth').addEventListener('click', loadSystemHealth);
	    document.getElementById('runSystemHealthScan').addEventListener('click', runSystemHealthScan);
	    systemIncidentStatusInput.addEventListener('change', loadSystemHealth);
	    systemIncidentSeverityInput.addEventListener('change', loadSystemHealth);
	    systemHealthFailureWindowInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadSystemHealth(); });
	    systemHealthNotificationStaleInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadSystemHealth(); });
	    systemIncidentOwnerInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadSystemHealth(); });
	    systemIncidentKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadSystemHealth(); });
	    systemIncidentsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-system-incident-action]');
	      if (button) updateSystemIncident(button);
	    });
	    document.getElementById('loadBackupOverview').addEventListener('click', loadBackupOverview);
	    document.getElementById('createBackup').addEventListener('click', () => runBackupAction('create'));
	    document.getElementById('cleanupBackups').addEventListener('click', requestBackupCleanup);
	    document.getElementById('saveBackupPolicy').addEventListener('click', saveBackupPolicy);
	    document.getElementById('cancelBackupRestore').addEventListener('click', cancelBackupRestore);
	    document.getElementById('confirmBackupRestore').addEventListener('click', confirmBackupRestore);
	    backupRestoreDialogEl.addEventListener('close', () => { pendingBackupRestore = null; });
	    backupRunsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-backup-action]');
	      if (!button) return;
	      if (button.dataset.backupAction === 'verify') runBackupAction('verify', button.dataset.backupRunId);
	      if (button.dataset.backupAction === 'replicate') runBackupAction('replicate', button.dataset.backupRunId);
	      if (button.dataset.backupAction === 'restore') requestBackupRestore(button.dataset.backupRunId);
	    });
	    backupCleanupRunsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-backup-cleanup-retry]');
	      if (button) runBackupAction('cleanup-retry', 0, button.dataset.backupCleanupRetry);
	    });
	    document.getElementById('loadComplianceOverview').addEventListener('click', loadComplianceOverview);
	    document.getElementById('saveCompliancePolicy').addEventListener('click', saveCompliancePolicy);
	    document.getElementById('createComplianceHold').addEventListener('click', createComplianceHold);
	    document.getElementById('requestComplianceExport').addEventListener('click', requestComplianceExport);
	    document.getElementById('requestComplianceErasure').addEventListener('click', requestComplianceErasure);
	    document.getElementById('closeComplianceSteps').addEventListener('click', () => complianceStepsDialogEl.close());
	    complianceTenantFilterInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadComplianceOverview(); });
	    complianceHoldsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-compliance-hold-release]');
	      if (button) releaseComplianceHold(button);
	    });
	    complianceExportsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-compliance-export-action]');
	      if (!button) return;
	      if (button.dataset.complianceExportAction === 'download') downloadComplianceExport(button.dataset.exportId);
	      else runComplianceExportAction(button.dataset.complianceExportAction, button.dataset.exportId);
	    });
	    complianceErasuresEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-compliance-erasure-action]');
	      if (!button) return;
	      if (button.dataset.complianceErasureAction === 'steps') loadComplianceErasureSteps(button.dataset.requestId);
	      else if (button.dataset.complianceErasureAction === 'approval') recoverComplianceErasureApproval(button.dataset.requestId);
	      else runComplianceErasureAction(button.dataset.complianceErasureAction, button.dataset.requestId);
	    });
	    document.getElementById('loadWeComCredentialProtection').addEventListener('click', loadWeComCredentialProtection);
	    rotateWeComCredentialsButton.addEventListener('click', rotateWeComCredentials);
	    weComCredentialTenantIdInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadWeComCredentialProtection(); });
	    document.getElementById('loadWeChatOpenCredentialProtection').addEventListener('click', loadWeChatOpenCredentialProtection);
	    rotateWeChatOpenCredentialsButton.addEventListener('click', rotateWeChatOpenCredentials);
	    weChatOpenCredentialTenantIdInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadWeChatOpenCredentialProtection(); });
	    document.getElementById('loadServiceAccounts').addEventListener('click', loadServiceAccounts);
	    document.getElementById('loadServiceAccountUsage').addEventListener('click', loadServiceAccountUsage);
	    document.getElementById('evaluateServiceAccountUsageAlerts').addEventListener('click', evaluateServiceAccountUsageAlerts);
	    document.getElementById('viewServiceAccountUsageAlerts').addEventListener('click', viewServiceAccountUsageAlerts);
	    serviceAccountUsageAccountFilterInput.addEventListener('change', loadServiceAccountUsage);
	    serviceAccountUsageSegmentsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-usage-days]');
	      if (!button) return;
	      serviceAccountUsageDays = Number(button.dataset.usageDays || 7);
	      serviceAccountUsageSegmentsEl.querySelectorAll('button[data-usage-days]').forEach(item => item.setAttribute('aria-pressed', item === button ? 'true' : 'false'));
	      loadServiceAccountUsage();
	    });
	    document.getElementById('newServiceAccount').addEventListener('click', resetServiceAccountEditor);
	    document.getElementById('saveServiceAccount').addEventListener('click', saveServiceAccount);
	    document.getElementById('rotateServiceAccountKey').addEventListener('click', rotateServiceAccountKey);
	    document.getElementById('copyServiceAccountKey').addEventListener('click', copyServiceAccountKey);
	    document.getElementById('dismissServiceAccountKey').addEventListener('click', dismissServiceAccountKey);
	    document.getElementById('cancelServiceAccountKeyRevoke').addEventListener('click', cancelServiceAccountKeyRevoke);
	    document.getElementById('confirmServiceAccountKeyRevoke').addEventListener('click', confirmRevokeServiceAccountKey);
	    serviceAccountRevokeDialogEl.addEventListener('close', () => { pendingServiceAccountRevoke = null; });
	    serviceAccountStatusFilterInput.addEventListener('change', loadServiceAccounts);
	    serviceAccountTenantFilterInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadServiceAccounts(); });
	    serviceAccountKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadServiceAccounts(); });
	    serviceAccountsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-service-account-action]');
	      if (!button) return;
	      if (button.dataset.serviceAccountAction === 'edit') selectServiceAccount(button.dataset.accountId);
	      if (button.dataset.serviceAccountAction === 'rotate') selectServiceAccountRotation(button.dataset.accountId);
	      if (button.dataset.serviceAccountAction === 'revoke') revokeServiceAccountKey(button);
	    });
	    document.getElementById('loadApprovals').addEventListener('click', loadApprovals);
	    document.getElementById('createApprovalReminders').addEventListener('click', createApprovalReminders);
	    document.getElementById('newApprovalDelegation').addEventListener('click', resetApprovalDelegationEditor);
	    document.getElementById('saveApprovalDelegation').addEventListener('click', saveApprovalDelegation);
	    approvalPoliciesEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-policy-save]');
	      if (button) saveApprovalPolicy(button);
	    });
	    approvalDelegationsEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-delegation-edit]');
	      if (button) selectApprovalDelegation(button.dataset.delegationEdit);
	    });
		    approvalStatusInput.addEventListener('change', loadApprovals);
		    approvalActionInput.addEventListener('change', loadApprovals);
		    approvalRiskInput.addEventListener('change', loadApprovals);
		    approvalKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadApprovals(); });
		    approvalsEl.addEventListener('click', event => {
		      const button = event.target.closest('button[data-approval-action]');
		      if (!button) return;
	      if (button.dataset.approvalAction === 'events') {
	        loadApprovalEvents(button.dataset.id);
	        loadApprovalDecisions(button.dataset.id);
	      }
		      if (button.dataset.approvalAction === 'approve') decideApproval(button, 'approve');
		      if (button.dataset.approvalAction === 'reject') decideApproval(button, 'reject');
		      if (button.dataset.approvalAction === 'cancel') cancelApproval(button);
		      if (button.dataset.approvalAction === 'execute') executeApproval(button);
		    });
		    document.getElementById('loadAdminTasks').addEventListener('click', loadAdminTasks);
		    document.getElementById('loadAdminTaskOwners').addEventListener('click', loadAdminTaskOwners);
		    document.getElementById('loadAdminTaskSla').addEventListener('click', loadAdminTaskSla);
		    document.getElementById('createTaskSlaNotifications').addEventListener('click', createTaskSlaNotifications);
		    document.getElementById('bulkCancelAdminTasks').addEventListener('click', bulkCancelAdminTasks);
		    document.getElementById('bulkResetAdminTasks').addEventListener('click', bulkResetAdminTasks);
	    document.getElementById('bulkApplyRenewalTasks').addEventListener('click', bulkApplyRenewalTasks);
	    document.getElementById('loadAuditIntegrity').addEventListener('click', loadAuditIntegrity);
	    document.getElementById('verifyAuditIntegrity').addEventListener('click', verifyAuditIntegrity);
	    document.getElementById('loadAuditAnchors').addEventListener('click', loadAuditAnchors);
	    document.getElementById('createAuditAnchor').addEventListener('click', () => runAuditAnchorAction('create'));
	    document.getElementById('verifyAuditAnchor').addEventListener('click', () => runAuditAnchorAction('verify'));
	    auditIntegrityTenantIdInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadAuditIntegrity(); });
	    document.getElementById('applyLogFilters').addEventListener('click', () => {
      loadOperations();
      loadBillingEvents();
    });
    document.getElementById('applyAlertFilters').addEventListener('click', loadAlerts);
    document.getElementById('bulkResolveAlerts').addEventListener('click', bulkResolveAlerts);
    document.getElementById('loadNotificationPolicies').addEventListener('click', loadNotificationPolicies);
    rotateNotificationCredentialsButton.addEventListener('click', rotateNotificationCredentials);
    document.getElementById('saveNotificationPolicy').addEventListener('click', saveNotificationPolicy);
    document.getElementById('testNotificationPolicy').addEventListener('click', testNotificationPolicy);
    document.getElementById('applyNotificationFilters').addEventListener('click', loadNotifications);
	    document.getElementById('bulkRetryNotifications').addEventListener('click', bulkRetryNotifications);
	    document.getElementById('bulkCloseNotifications').addEventListener('click', bulkCloseNotifications);
	    document.getElementById('applyDailyReport').addEventListener('click', loadDailyReport);
	    document.getElementById('applyRiskTaskFilters').addEventListener('click', () => {
	      loadRiskFollowUps();
	      loadRiskFollowUpOwners();
	    });
		    document.getElementById('applyBillingFollowFilters').addEventListener('click', () => {
		      loadBillingReconciliationFollowUps();
		      loadBillingReconciliationFollowUpOwners();
		    });
		    document.getElementById('bulkCloseRiskFollowUps').addEventListener('click', bulkCloseRiskFollowUps);
		    document.getElementById('bulkCloseBillingFollowUps').addEventListener('click', bulkCloseBillingReconciliationFollowUps);
		    document.getElementById('exportTenants').addEventListener('click', () => downloadCSV('tenants'));
	    document.getElementById('exportUsage').addEventListener('click', () => downloadCSV('usage'));
		    document.getElementById('exportRisk').addEventListener('click', () => downloadCSV('risk'));
		    document.getElementById('exportCustomerSuccess').addEventListener('click', () => downloadCSV('customerSuccess'));
		    document.getElementById('exportCustomerSuccessOwners').addEventListener('click', () => downloadCSV('customerSuccessOwners'));
		    document.getElementById('exportRenewalForecast').addEventListener('click', () => downloadCSV('renewalForecast'));
		    document.getElementById('exportRenewalForecastOwners').addEventListener('click', () => downloadCSV('renewalForecastOwners'));
		    document.getElementById('exportRiskFollowUps').addEventListener('click', () => downloadCSV('riskFollowUps'));
		    document.getElementById('exportRiskFollowUpOwners').addEventListener('click', () => downloadCSV('riskFollowUpOwners'));
		    document.getElementById('exportDailyReport').addEventListener('click', () => downloadCSV('dailyReport'));
		    document.getElementById('exportBusinessMetrics').addEventListener('click', () => downloadCSV('businessMetrics'));
		    document.getElementById('exportBusinessTrends').addEventListener('click', () => downloadCSV('businessTrends'));
		    document.getElementById('exportOperationQueue').addEventListener('click', () => downloadCSV('operationQueue'));
		    document.getElementById('exportOperationQueueOwners').addEventListener('click', () => downloadCSV('operationQueueOwners'));
		    document.getElementById('exportOperationQueueAssignments').addEventListener('click', () => downloadCSV('operationQueueAssignments'));
	        document.getElementById('exportTasks').addEventListener('click', () => downloadCSV('tasks'));
	    document.getElementById('exportTaskSla').addEventListener('click', () => downloadCSV('taskSla'));
		    document.getElementById('exportPackages').addEventListener('click', () => downloadCSV('packages'));
		    document.getElementById('exportSubscriptions').addEventListener('click', () => downloadCSV('subscriptions'));
			    document.getElementById('exportPaymentOrders').addEventListener('click', () => downloadCSV('paymentOrders'));
			    document.getElementById('exportPaymentRefunds').addEventListener('click', () => downloadCSV('paymentRefunds'));
			    document.getElementById('exportInvoiceDocuments').addEventListener('click', () => downloadCSV('invoiceDocuments'));
			    document.getElementById('exportPaymentSettlementBatches').addEventListener('click', () => downloadCSV('paymentSettlementBatches'));
			    document.getElementById('exportPaymentSettlementEntries').addEventListener('click', () => downloadCSV('paymentSettlementEntries'));
    document.getElementById('exportAlerts').addEventListener('click', () => downloadCSV('alerts'));
    document.getElementById('exportNotifications').addEventListener('click', () => downloadCSV('notifications'));
    document.getElementById('exportNotificationHealth').addEventListener('click', () => downloadCSV('notificationHealth'));
    document.getElementById('exportNotificationSlo').addEventListener('click', () => downloadCSV('notificationSlo'));
				    document.getElementById('exportOperations').addEventListener('click', () => downloadCSV('operations'));
				    document.getElementById('exportBilling').addEventListener('click', () => downloadCSV('billingEvents'));
				    document.getElementById('exportBillingReconciliation').addEventListener('click', () => downloadCSV('billingReconciliation'));
				    document.getElementById('exportBillingFollowUps').addEventListener('click', () => downloadCSV('billingReconciliationFollowUps'));
				    document.getElementById('exportBillingFollowUpOwners').addEventListener('click', () => downloadCSV('billingReconciliationFollowUpOwners'));
	    document.getElementById('provisionTenant').addEventListener('click', provisionTenant);
	    document.getElementById('createProvisionTask').addEventListener('click', createProvisionTask);
	    document.getElementById('applyProvisionTask').addEventListener('click', () => applyProvisionTask());
	    document.getElementById('bulkApplyProvisionTasks').addEventListener('click', bulkApplyProvisionTasks);
	    document.getElementById('loadProvisionTasks').addEventListener('click', loadProvisionTasks);
	    document.getElementById('applyPackage').addEventListener('click', applyPackage);
    document.getElementById('applyRenewal').addEventListener('click', applyRenewal);
    document.getElementById('createRenewalTask').addEventListener('click', createRenewalTask);
    document.getElementById('applyRenewalTask').addEventListener('click', () => applyRenewalTask());
    document.getElementById('loadRenewalTasks').addEventListener('click', loadRenewalTasks);
    document.getElementById('applyRenewalForecastFilters').addEventListener('click', () => {
      loadRenewalForecast();
      loadBusinessTrends();
    });
	    document.getElementById('assignRenewalForecast').addEventListener('click', assignRenewalForecast);
	    document.getElementById('createRenewalForecastTasks').addEventListener('click', createRenewalForecastTasks);
	    document.getElementById('createRenewalForecastNotifications').addEventListener('click', createRenewalForecastNotifications);
    document.getElementById('savePackage').addEventListener('click', savePackage);
    document.getElementById('previewPackageSync').addEventListener('click', () => syncPackage(true));
    document.getElementById('applyPackageSync').addEventListener('click', () => syncPackage(false));
    document.getElementById('createPackageSyncTask').addEventListener('click', createPackageSyncTask);
    document.getElementById('applyPackageSyncTask').addEventListener('click', () => applyPackageSyncTask());
    document.getElementById('bulkApplyPackageSyncTasks').addEventListener('click', bulkApplyPackageSyncTasks);
    document.getElementById('loadPackageSyncTasks').addEventListener('click', loadPackageSyncTasks);
    tenantStatusButton.addEventListener('click', updateTenantStatus);
    tenantStatusInput.addEventListener('change', renderTenantStatusApprovalState);
    statusTenantInput.addEventListener('input', renderTenantStatusApprovalState);
    document.getElementById('loadSubscriptions').addEventListener('click', loadSubscriptions);
    document.getElementById('previewSubscriptionReconcile').addEventListener('click', () => reconcileSubscriptions(true));
    document.getElementById('applySubscriptionReconcile').addEventListener('click', () => reconcileSubscriptions(false));
    document.getElementById('transitionSubscription').addEventListener('click', transitionSubscription);
    subscriptionsEl.addEventListener('click', event => {
      const button = event.target.closest('[data-action="select-subscription"]');
      if (button) selectSubscription(button);
    });
    subscriptionStatusInput.addEventListener('change', loadSubscriptions);
    subscriptionAccessInput.addEventListener('change', loadSubscriptions);
    subscriptionKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadSubscriptions();
    });
    document.getElementById('loadPaymentOrders').addEventListener('click', loadPaymentOrders);
    document.getElementById('previewPaymentDunning').addEventListener('click', () => runPaymentDunning(true));
    document.getElementById('applyPaymentDunning').addEventListener('click', () => runPaymentDunning(false));
    document.getElementById('createPaymentOrder').addEventListener('click', createPaymentOrder);
		document.getElementById('loadPaymentRefunds').addEventListener('click', loadPaymentRefunds);
		document.getElementById('createPaymentRefund').addEventListener('click', createPaymentRefund);
		document.getElementById('loadInvoiceDocuments').addEventListener('click', loadInvoiceDocuments);
		document.getElementById('loadInvoiceProfile').addEventListener('click', () => loadInvoiceProfile());
		document.getElementById('saveInvoiceProfile').addEventListener('click', saveInvoiceProfile);
		document.getElementById('createInvoiceDocument').addEventListener('click', createInvoiceDocument);
		document.getElementById('transitionInvoiceDocument').addEventListener('click', () => transitionInvoiceDocument());
		invoiceTransitionStatusInput.addEventListener('change', renderInvoiceIssueApprovalState);
    paymentOrdersEl.addEventListener('click', event => {
	  const cancelButton = event.target.closest('[data-action="cancel-payment-order"]');
	  if (cancelButton) cancelPaymentOrder(cancelButton);
		  const refundButton = event.target.closest('[data-action="select-payment-refund"]');
		  if (refundButton) selectPaymentRefund(refundButton);
		  const invoiceButton = event.target.closest('[data-action="select-payment-invoice"]');
		  if (invoiceButton) prefillInvoiceFromOrder(invoiceButton);
	    });
	paymentRefundsEl.addEventListener('click', event => {
	  const button = event.target.closest('[data-action="cancel-payment-refund"]');
		  if (button) cancelPaymentRefund(button);
		});
		invoiceDocumentsEl.addEventListener('click', event => {
		  const button = event.target.closest('button[data-action]');
		  if (!button) return;
		  if (button.dataset.action === 'select-invoice-document') selectInvoiceDocument(button);
		  if (button.dataset.action === 'load-invoice-profile') loadInvoiceProfile(button.dataset.tenantId);
		  if (button.dataset.action === 'create-credit-note') prefillCreditNote(button);
		  if (button.dataset.action === 'transition-invoice-document') transitionInvoiceDocument(button.dataset.status, button);
		});
		paymentSettlementBatchesEl.addEventListener('click', event => {
		  const button = event.target.closest('button[data-action]');
		  if (!button) return;
		  if (button.dataset.action === 'select-settlement') selectPaymentSettlementBatch(button);
		  if (button.dataset.action === 'reconcile-settlement') reconcilePaymentSettlement(button.dataset.dryRun === 'true', button);
		  if (button.dataset.action === 'transition-settlement') transitionPaymentSettlement(button.dataset.transition, button);
		});
			paymentSettlementEntriesEl.addEventListener('click', event => {
		  const button = event.target.closest('button[data-action="resolve-settlement-entry"]');
		  if (button) resolvePaymentSettlementEntry(button);
			});
			document.getElementById('loadPaymentSettlementSyncRuns').addEventListener('click', loadPaymentSettlementSyncRuns);
			previewPaymentSettlementSyncButton.addEventListener('click', () => triggerPaymentSettlementSync(true));
			runPaymentSettlementSyncButton.addEventListener('click', () => triggerPaymentSettlementSync(false));
			paymentSettlementSyncProviderInput.addEventListener('change', loadPaymentSettlementSyncRuns);
			paymentSettlementSyncSourceInput.addEventListener('change', loadPaymentSettlementSyncRuns);
			paymentSettlementSyncStatusInput.addEventListener('change', loadPaymentSettlementSyncRuns);
			document.getElementById('loadPaymentSettlementBatches').addEventListener('click', loadPaymentSettlementBatches);
		document.getElementById('loadPaymentSettlementEntries').addEventListener('click', loadPaymentSettlementEntries);
		document.getElementById('importPaymentSettlement').addEventListener('click', importPaymentSettlement);
		paymentSettlementFileInput.addEventListener('change', async () => {
		  const file = paymentSettlementFileInput.files && paymentSettlementFileInput.files[0];
		  if (!file) return;
		  paymentSettlementCSVInput.value = await file.text();
		  paymentSettlementImportStateInput.value = file.name;
		});
		paymentSettlementStatusInput.addEventListener('change', loadPaymentSettlementBatches);
		paymentSettlementProviderInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadPaymentSettlementBatches(); });
		paymentSettlementKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadPaymentSettlementBatches(); });
		paymentSettlementReconciliationStatusInput.addEventListener('change', loadPaymentSettlementEntries);
		paymentSettlementHandlingStatusInput.addEventListener('change', loadPaymentSettlementEntries);
		paymentSettlementEntryKeywordInput.addEventListener('keydown', event => { if (event.key === 'Enter') loadPaymentSettlementEntries(); });
    paymentOrderStatusInput.addEventListener('change', loadPaymentOrders);
    [paymentProviderInput, paymentKeywordInput].forEach(input => {
      input.addEventListener('keydown', event => {
        if (event.key === 'Enter') loadPaymentOrders();
      });
    });
	paymentRefundStatusInput.addEventListener('change', loadPaymentRefunds);
		[paymentRefundOrderFilterInput, paymentRefundKeywordInput].forEach(input => {
	  input.addEventListener('keydown', event => {
	    if (event.key === 'Enter') loadPaymentRefunds();
		});
		invoiceKindFilterInput.addEventListener('change', loadInvoiceDocuments);
		invoiceStatusFilterInput.addEventListener('change', loadInvoiceDocuments);
		[invoiceTenantFilterInput, invoiceKeywordFilterInput].forEach(input => {
		  input.addEventListener('keydown', event => {
		    if (event.key === 'Enter') loadInvoiceDocuments();
		  });
		});
	});
    packageCodeInput.addEventListener('change', () => fillPackageEditor(packageCodeInput.value));
    packageTenantInput.addEventListener('change', () => {
      packageAssignmentVersionInput.value = '0';
    });
    editPackageCodeInput.addEventListener('change', () => fillPackageEditor(editPackageCodeInput.value.trim()));
    scopeInput.addEventListener('change', loadOverview);
    expiringInput.addEventListener('change', loadOverview);
	    riskHighUsageInput.addEventListener('change', () => {
	      loadBusinessMetrics();
	      loadRisk();
	      loadCustomerSuccess();
	      loadCustomerSuccessOwners();
	      loadOperationQueue();
	      loadOperationQueueOwners();
	    });
    tenantInput.addEventListener('change', loadOverview);
	    document.getElementById('applyCustomerSuccess').addEventListener('click', loadCustomerSuccess);
	    document.getElementById('applyCustomerSuccess').addEventListener('click', loadCustomerSuccessOwners);
	    document.getElementById('applyCustomerSuccess').addEventListener('click', loadOperationQueue);
	    document.getElementById('applyCustomerSuccess').addEventListener('click', loadOperationQueueOwners);
	    document.getElementById('applyOperationQueue').addEventListener('click', loadOperationQueue);
	    document.getElementById('applyOperationQueue').addEventListener('click', loadOperationQueueOwners);
	    document.getElementById('applyOperationQueue').addEventListener('click', loadOperationQueueAssignments);
	    document.getElementById('assignOperationQueue').addEventListener('click', assignOperationQueue);
	    document.getElementById('createOperationQueueAssignmentNotifications').addEventListener('click', createOperationQueueAssignmentNotifications);
	    document.getElementById('assignNotificationHealthQueue').addEventListener('click', assignNotificationHealthQueue);
	    document.getElementById('recoverNotificationHealthQueue').addEventListener('click', recoverNotificationHealthQueue);
	    operationQueueAssignmentsEl.addEventListener('click', event => {
	      const button = event.target.closest('[data-operation-queue-assignment-close]');
	      if (!button) return;
	      closeOperationQueueAssignment(button.dataset.operationId, button.dataset.operationQueueAssignmentClose);
	    });
	    document.getElementById('assignCustomerSuccess').addEventListener('click', assignCustomerSuccess);
    document.getElementById('createCustomerSuccessRenewalTasks').addEventListener('click', createCustomerSuccessRenewalTasks);
    document.getElementById('createCustomerSuccessRenewalNotifications').addEventListener('click', createCustomerSuccessRenewalNotifications);
	    customerSuccessPriorityInput.addEventListener('change', () => {
	      loadCustomerSuccess();
	      loadCustomerSuccessOwners();
	      loadOperationQueue();
	      loadOperationQueueOwners();
	    });
	    customerSuccessOwnerInput.addEventListener('keydown', event => {
	      if (event.key === 'Enter') {
	        loadCustomerSuccess();
	        loadCustomerSuccessOwners();
	        loadOperationQueue();
	        loadOperationQueueOwners();
	      }
	    });
	customerSuccessTenantLimitInput.addEventListener('keydown', event => {
	  if (event.key === 'Enter') {
	    loadBusinessMetrics();
	    loadRenewalForecast();
	    loadCustomerSuccess();
	    loadCustomerSuccessOwners();
	    loadOperationQueue();
	    loadOperationQueueOwners();
	  }
	});
	operationQueueSourceInput.addEventListener('change', () => {
	  loadOperationQueue();
	  loadOperationQueueOwners();
	  loadOperationQueueAssignments();
	});
	operationQueuePriorityInput.addEventListener('change', () => {
	  loadOperationQueue();
	  loadOperationQueueOwners();
	});
	operationQueueAssignmentDueStateInput.addEventListener('change', loadOperationQueueAssignments);
	operationQueueAssignmentCurrentOnlyInput.addEventListener('change', loadOperationQueueAssignments);
	[operationQueueOwnerInput, operationQueueKeywordInput].forEach(input => {
	  input.addEventListener('keydown', event => {
	    if (event.key === 'Enter') {
	      loadOperationQueue();
	      loadOperationQueueOwners();
	      loadOperationQueueAssignments();
	    }
	  });
	});
    riskTaskStatusInput.addEventListener('change', () => {
      loadRiskFollowUps();
      loadRiskFollowUpOwners();
    });
    riskTaskDueStateInput.addEventListener('change', () => {
      loadRiskFollowUps();
      loadRiskFollowUpOwners();
    });
    riskTaskOwnerInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') {
        loadRiskFollowUps();
        loadRiskFollowUpOwners();
      }
    });
    riskTaskKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') {
        loadRiskFollowUps();
        loadRiskFollowUpOwners();
      }
    });
	    filterKeywordInput.addEventListener('keydown', event => {
	      if (event.key === 'Enter') loadOverview();
	    });
	    filterPackageCodeInput.addEventListener('change', loadOverview);
	    filterTenantStatusInput.addEventListener('change', loadOverview);
	    filterDueStateInput.addEventListener('change', loadOverview);
	    adminTaskTypeInput.addEventListener('change', loadAdminTasks);
	    adminTaskStatusInput.addEventListener('change', loadAdminTasks);
		    adminTaskPackageCodeInput.addEventListener('change', loadAdminTasks);
		    adminTaskSlaWarningInput.addEventListener('change', loadAdminTaskSla);
		    adminTaskSlaWarningInput.addEventListener('change', loadOperationQueue);
		    adminTaskSlaWarningInput.addEventListener('change', loadOperationQueueOwners);
		    adminTaskSlaOverdueInput.addEventListener('change', loadAdminTaskSla);
		    adminTaskSlaOverdueInput.addEventListener('change', loadOperationQueue);
		    adminTaskSlaOverdueInput.addEventListener('change', loadOperationQueueOwners);
		    adminTaskTenantInput.addEventListener('keydown', event => {
		      if (event.key === 'Enter') loadAdminTasks();
		    });
		    [adminTaskSlaWarningInput, adminTaskSlaOverdueInput].forEach(input => {
		      input.addEventListener('keydown', event => {
		        if (event.key === 'Enter') {
		          loadAdminTaskSla();
		          loadOperationQueue();
		          loadOperationQueueOwners();
		        }
		      });
		    });
	    document.getElementById('loadTenantLifecycle').addEventListener('click', () => loadTenantLifecycle());
	    document.getElementById('exportTenantLifecycle').addEventListener('click', () => downloadCSV('tenantLifecycle'));
	    tenantLifecycleSourceInput.addEventListener('change', () => loadTenantLifecycle());
	    [tenantLifecycleEventTypeInput, tenantLifecycleStatusInput, tenantLifecycleKeywordInput].forEach(input => {
	      input.addEventListener('keydown', event => {
	        if (event.key === 'Enter') loadTenantLifecycle();
	      });
	    });
	    operationActionInput.addEventListener('keydown', event => {
	      if (event.key === 'Enter') loadOperations();
	    });
    operationTargetTypeInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadOperations();
    });
    operationKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadOperations();
    });
    billingEventTypeInput.addEventListener('change', loadBillingEvents);
    billingPackageCodeInput.addEventListener('change', loadBillingEvents);
    billingMismatchOnlyInput.addEventListener('change', loadBillingReconciliation);
    billingKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadBillingEvents();
    });
    billingFollowStatusInput.addEventListener('change', () => {
      loadBillingReconciliationFollowUps();
      loadBillingReconciliationFollowUpOwners();
    });
    billingFollowDueStateInput.addEventListener('change', () => {
      loadBillingReconciliationFollowUps();
      loadBillingReconciliationFollowUpOwners();
    });
    billingFollowOwnerInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') {
        loadBillingReconciliationFollowUps();
        loadBillingReconciliationFollowUpOwners();
      }
    });
    billingFollowKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') {
        loadBillingReconciliationFollowUps();
        loadBillingReconciliationFollowUpOwners();
      }
    });
    alertStatusInput.addEventListener('change', loadAlerts);
    alertMetricInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadAlerts();
    });
    alertTypeInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadAlerts();
    });
    notificationPolicyStateInput.addEventListener('change', loadNotificationPolicies);
    notificationPolicyKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotificationPolicies();
    });
    notificationPolicyTenantInput.addEventListener('change', () => loadNotificationPolicy());
    document.getElementById('loadNotificationHealth').addEventListener('click', loadNotificationHealth);
    document.getElementById('loadNotificationSlo').addEventListener('click', loadNotificationSlo);
    notificationHealthWindowInput.addEventListener('change', loadNotificationHealth);
    notificationHealthStateInput.addEventListener('change', loadNotificationHealth);
    notificationHealthStaleInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotificationHealth();
    });
    notificationHealthKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotificationHealth();
    });
    notificationSloDaysInput.addEventListener('change', loadNotificationSlo);
    notificationSloSuccessTargetInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotificationSlo();
    });
    notificationSloLatencySecondsInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotificationSlo();
    });
    notificationSloLatencyTargetInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotificationSlo();
    });
    notificationSloKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotificationSlo();
    });
    notificationStatusInput.addEventListener('change', loadNotifications);
    notificationChannelInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotifications();
    });
    notificationKeywordInput.addEventListener('keydown', event => {
      if (event.key === 'Enter') loadNotifications();
    });
    alertsEl.addEventListener('click', event => {
      const button = event.target.closest('button[data-action="resolve-alert"]');
      if (!button) return;
      resolveAlert(button.dataset.tenantId, button.dataset.metric, button.dataset.alertType, button.dataset.periodKey);
    });
    notificationPoliciesEl.addEventListener('click', event => {
      const button = event.target.closest('button[data-action="edit-notification-policy"]');
      if (!button) return;
      loadNotificationPolicy(button.dataset.tenantId);
    });
    customerSuccessEl.addEventListener('click', event => {
      const button = event.target.closest('button[data-action="customer-success-tenant"]');
      if (!button) return;
      tenantInput.value = button.dataset.tenantId || '';
      loadTenantDetail({ tenantId: button.dataset.tenantId });
      loadTenantLifecycle(button.dataset.tenantId);
    });
    riskTenantsEl.addEventListener('click', event => {
      const button = event.target.closest('button[data-action="risk-follow-up"]');
      if (!button) return;
      recordRiskFollowUp(button.dataset.tenantId);
    });
    tenantsEl.addEventListener('click', event => {
      const button = event.target.closest('button[data-action="adjust-tenant-package"]');
      if (!button) return;
      const tenant = tenantOverviewCache[String(button.dataset.tenantId || '')];
      if (!tenant) return;
      fillTenantPackageEditor(tenant);
      packageAssignmentRemarkInput.focus();
    });
	    notificationsEl.addEventListener('click', event => {
	      const retryButton = event.target.closest('button[data-action="retry-notification"]');
	      if (retryButton) {
	        retryNotification(retryButton.dataset.notificationId);
	        return;
	      }
	      const closeButton = event.target.closest('button[data-action="close-notification"]');
	      if (closeButton) closeNotification(closeButton.dataset.notificationId);
	    });
    billingReconciliationEl.addEventListener('click', event => {
      const button = event.target.closest('button[data-action="follow-up-billing-reconciliation"]');
      if (!button) return;
      followUpBillingReconciliation(button.dataset.billingEventId);
    });
	    adminTasksEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-action]');
	      if (!button) return;
		      if (button.dataset.action === 'apply-admin-task') {
		        applyAdminTask(button.dataset.taskId, button.dataset.taskType, button.dataset.taskVersion);
		      } else if (button.dataset.action === 'cancel-admin-task') {
		        cancelAdminTask(button.dataset.taskId);
		      } else if (button.dataset.action === 'reset-admin-task') {
		        resetAdminTask(button.dataset.taskId);
		      } else if (button.dataset.action === 'trace-admin-task') {
		        traceAdminTaskOperations(button.dataset.taskId);
		      }
		    });
	    packageSyncTasksEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-action="apply-package-sync-task"]');
	      if (!button) return;
	      syncTaskIdInput.value = button.dataset.taskId || '';
	      applyPackageSyncTask(button.dataset.taskId);
	    });
	    provisionTasksEl.addEventListener('click', event => {
	      const button = event.target.closest('button[data-action="apply-provision-task"]');
	      if (!button) return;
	      provisionTaskIdInput.value = button.dataset.taskId || '';
	      applyProvisionTask(button.dataset.taskId, button.dataset.taskVersion);
	    });
	    renewalTasksEl.addEventListener('click', event => {
      const button = event.target.closest('button[data-action="apply-renewal-task"]');
      if (!button) return;
      renewalTaskIdInput.value = button.dataset.taskId || '';
      applyRenewalTask(button.dataset.taskId, button.dataset.taskVersion);
    });
		    initWorkspaceNavigation();
		    resetServiceAccountEditor();
		    loadOverview();
  </script>
</body>
</html>`
