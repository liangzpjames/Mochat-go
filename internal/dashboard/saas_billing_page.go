package dashboard

import "net/http"

func NewSaaSBillingPageHandler() http.Handler {
	return http.HandlerFunc(ServeSaaSBillingPage)
}

func ServeSaaSBillingPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(saasBillingPageHTML))
}

const saasBillingPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>MoChat Go 账单中心</title>
  <style>
    :root { color-scheme: light; --bg:#f4f6f8; --panel:#fff; --line:#d6dde5; --text:#1f2933; --muted:#667281; --accent:#087f72; --accent-dark:#06685f; --danger:#b42318; --warning:#a15c00; }
    * { box-sizing:border-box; }
    body { margin:0; background:var(--bg); color:var(--text); font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif; font-size:14px; letter-spacing:0; }
    button,input,select { font:inherit; letter-spacing:0; }
    main { width:min(1240px,calc(100% - 32px)); margin:0 auto; padding:24px 0 48px; }
    header { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-bottom:16px; }
    h1 { margin:0; font-size:24px; line-height:1.3; }
    h2 { margin:0; font-size:16px; line-height:1.4; }
    .muted { color:var(--muted); }
    .toolbar,.filters,.formgrid { display:grid; grid-template-columns:repeat(12,minmax(0,1fr)); gap:10px; align-items:end; }
    .toolbar,.filters,.formgrid,.tablewrap { background:var(--panel); border:1px solid var(--line); border-radius:6px; padding:14px; }
    .toolbar { margin-bottom:14px; }
    label { display:flex; flex-direction:column; gap:6px; color:#526070; font-weight:600; min-width:0; }
    input,select { width:100%; min-width:0; height:40px; border:1px solid var(--line); border-radius:5px; padding:0 11px; background:#fff; color:var(--text); }
    input:focus,select:focus { outline:2px solid rgba(8,127,114,.2); border-color:var(--accent); }
    button { height:40px; border:0; border-radius:5px; padding:0 16px; background:var(--accent); color:#fff; font-weight:700; cursor:pointer; white-space:nowrap; }
    button:hover { background:var(--accent-dark); }
    button.secondary { background:#eef2f5; color:#334155; border:1px solid var(--line); }
    button.danger { background:#fff1f0; color:var(--danger); border:1px solid #f3b7b2; }
    button.small { height:32px; padding:0 10px; font-size:13px; }
    .span2 { grid-column:span 2; } .span3 { grid-column:span 3; } .span4 { grid-column:span 4; } .span6 { grid-column:span 6; } .span8 { grid-column:span 8; } .span12 { grid-column:span 12; }
    .summary { display:grid; grid-template-columns:repeat(6,minmax(0,1fr)); gap:10px; margin:14px 0 20px; }
    .tile { background:var(--panel); border:1px solid var(--line); border-radius:6px; padding:14px; min-height:84px; }
    .tile .label { color:var(--muted); font-size:13px; }
    .tile .value { margin-top:9px; font-size:20px; font-weight:750; overflow-wrap:anywhere; }
    section { margin-top:22px; }
    .sectionhead { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:10px; }
    .status { min-height:20px; color:var(--muted); }
    .tablewrap { padding:0; overflow-x:auto; }
    table { width:100%; min-width:880px; border-collapse:collapse; }
    th,td { padding:11px 12px; text-align:left; border-bottom:1px solid #e7ebef; vertical-align:top; }
    th { color:#596677; font-size:13px; background:#fafbfc; }
    tbody tr:last-child td { border-bottom:0; }
    .empty { text-align:center; color:var(--muted); padding:28px; }
    .badge { display:inline-flex; align-items:center; min-height:24px; padding:2px 8px; border-radius:999px; background:#eef2f5; color:#475569; font-size:12px; }
    .badge.ok { background:#e6f5f1; color:#05695f; } .badge.warn { background:#fff4df; color:var(--warning); } .badge.bad { background:#fff0ef; color:var(--danger); }
    .money { font-variant-numeric:tabular-nums; white-space:nowrap; }
    a { color:var(--accent-dark); font-weight:650; }
    @media (max-width:900px) { .summary { grid-template-columns:repeat(3,minmax(0,1fr)); } .span2,.span3,.span4 { grid-column:span 6; } .span6,.span8 { grid-column:span 12; } }
    @media (max-width:560px) { main { width:calc(100% - 20px); padding-top:14px; } header { align-items:flex-start; } h1 { font-size:20px; } .summary { grid-template-columns:repeat(2,minmax(0,1fr)); } .toolbar,.filters,.formgrid { grid-template-columns:1fr; } .span2,.span3,.span4,.span6,.span8,.span12 { grid-column:1; } button { width:100%; } .tile .value { font-size:17px; } }
  </style>
</head>
<body>
<main>
  <header><div><h1>账单中心</h1><div class="muted" id="tenantLabel">订单、退款与发票</div></div><button id="refreshAll" type="button">刷新</button></header>
  <section class="toolbar" aria-label="账单访问">
    <label class="span8">Dashboard JWT<input id="token" type="password" autocomplete="off" placeholder="Bearer token 或纯 token"></label>
    <button class="span2 secondary" id="saveToken" type="button">保存凭证</button>
    <button class="span2" id="loadSummary" type="button">加载账单</button>
  </section>
  <div class="status" id="pageStatus"></div>
  <section class="summary" id="billingSummary" aria-label="账单汇总">
    <div class="tile"><div class="label">累计支付</div><div class="value">-</div></div>
    <div class="tile"><div class="label">累计退款</div><div class="value">-</div></div>
    <div class="tile"><div class="label">净支付</div><div class="value">-</div></div>
    <div class="tile"><div class="label">净开票</div><div class="value">-</div></div>
    <div class="tile"><div class="label">可申请开票</div><div class="value">-</div></div>
    <div class="tile"><div class="label">待红冲</div><div class="value">-</div></div>
  </section>

  <section aria-labelledby="profileTitle">
    <div class="sectionhead"><h2 id="profileTitle">开票资料</h2><span class="status" id="profileStatus"></span></div>
    <div class="formgrid">
      <label class="span3">发票类型<select id="profileType"><option value="normal">普通发票</option><option value="special">专用发票</option></select></label>
      <label class="span6">发票抬头<input id="profileTitleInput" maxlength="255"></label>
      <label class="span3">税号<input id="profileTaxId" maxlength="64"></label>
      <label class="span4">接收邮箱<input id="profileEmail" type="email" maxlength="255"></label>
      <label class="span4">联系电话<input id="profilePhone" maxlength="32"></label>
      <label class="span4">收件人<input id="profileRecipient" maxlength="128"></label>
      <label class="span6">注册地址<input id="profileAddress" maxlength="255"></label>
      <label class="span3">开户行<input id="profileBank" maxlength="255"></label>
      <label class="span3">银行账号<input id="profileBankAccount" maxlength="128"></label>
      <label class="span8">备注<input id="profileRemark" maxlength="255"></label>
      <button class="span4" id="saveProfile" type="button">保存开票资料</button>
    </div>
  </section>

  <section aria-labelledby="ordersTitle">
    <div class="sectionhead"><h2 id="ordersTitle">支付订单</h2><span class="status" id="orderCount"></span></div>
    <div class="tablewrap"><table aria-label="支付订单"><thead><tr><th>订单</th><th>套餐</th><th>支付</th><th>退款</th><th>开票</th><th>操作</th></tr></thead><tbody id="orders"><tr><td class="empty" colspan="6">暂无订单</td></tr></tbody></table></div>
  </section>

  <section aria-labelledby="requestTitle">
    <div class="sectionhead"><h2 id="requestTitle">申请开票</h2><span class="status" id="requestStatus"></span></div>
    <div class="formgrid">
      <label class="span4">支付订单<input id="invoiceOrderNo" placeholder="PAY-..."></label>
      <label class="span3">开票金额<input id="invoiceAmount" inputmode="decimal" placeholder="如 128.00"></label>
      <label class="span2">币种<input id="invoiceCurrency" value="CNY" maxlength="3"></label>
      <label class="span3">幂等键<input id="invoiceIdempotency" placeholder="留空自动生成"></label>
      <label class="span8">备注<input id="invoiceRemark" maxlength="255"></label>
      <button class="span4" id="requestInvoice" type="button">提交开票申请</button>
    </div>
  </section>

  <section aria-labelledby="invoicesTitle">
    <div class="sectionhead"><h2 id="invoicesTitle">发票单据</h2><span class="status" id="invoiceCount"></span></div>
    <div class="filters">
      <label class="span3">单据类型<select id="invoiceKind"><option value="all">全部</option><option value="invoice">蓝票</option><option value="credit_note">红票</option></select></label>
      <label class="span3">状态<select id="invoiceStatus"><option value="all">全部</option><option value="requested">待处理</option><option value="processing">处理中</option><option value="issued">已开具</option><option value="failed">失败</option><option value="canceled">已取消</option></select></label>
      <label class="span4">搜索<input id="invoiceKeyword" placeholder="单据号、订单、抬头或税号"></label>
      <button class="span2" id="loadInvoices" type="button">筛选</button>
    </div>
    <div class="tablewrap" style="margin-top:10px"><table aria-label="发票单据"><thead><tr><th>状态</th><th>单据</th><th>订单</th><th>金额</th><th>抬头</th><th>结果</th><th>操作</th></tr></thead><tbody id="invoices"><tr><td class="empty" colspan="7">暂无发票</td></tr></tbody></table></div>
  </section>

  <section aria-labelledby="refundsTitle">
    <div class="sectionhead"><h2 id="refundsTitle">退款记录</h2><span class="status" id="refundCount"></span></div>
    <div class="tablewrap"><table aria-label="退款记录"><thead><tr><th>退款单</th><th>订单</th><th>状态</th><th>金额</th><th>原因</th><th>时间</th></tr></thead><tbody id="refunds"><tr><td class="empty" colspan="6">暂无退款</td></tr></tbody></table></div>
  </section>
</main>
<script>
  const $ = id => document.getElementById(id);
  const tokenInput = $('token');
  let profileVersion = 0;
  tokenInput.value = localStorage.getItem('mochat_go_dashboard_token') || localStorage.getItem('token') || '';
  const esc = value => String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[ch]));
  const money = cents => (Number(cents || 0) / 100).toLocaleString('zh-CN', {minimumFractionDigits:2, maximumFractionDigits:2});
  const authHeader = () => { const token = tokenInput.value.trim().replace(/^Bearer\s+/i, ''); return token ? {'Authorization':'Bearer ' + token} : {}; };
  async function api(path, options = {}) {
    const headers = Object.assign({'Content-Type':'application/json'}, authHeader(), options.headers || {});
    const response = await fetch(path, Object.assign({}, options, {headers}));
    const payload = await response.json().catch(() => ({}));
    if (!response.ok || ![0,200].includes(Number(payload.code))) throw new Error(payload.msg || ('HTTP ' + response.status));
    return payload.data || {};
  }
  function badge(status) {
    const map = {requested:['待处理','warn'],processing:['处理中','warn'],issued:['已开具','ok'],failed:['失败','bad'],canceled:['已取消','bad'],paid:['已支付','ok'],succeeded:['已退款','ok']};
    const item = map[status] || [status || '-', ''];
    return '<span class="badge ' + item[1] + '">' + esc(item[0]) + '</span>';
  }
  function renderProfile(data) {
    const p = data || {}; profileVersion = Number(p.version || 0);
    $('profileType').value = p.invoiceType || 'normal'; $('profileTitleInput').value = p.invoiceTitle || '';
    $('profileTaxId').value = p.taxIdentifier || ''; $('profileEmail').value = p.email || '';
    $('profilePhone').value = p.phone || ''; $('profileRecipient').value = p.recipientName || '';
    $('profileAddress').value = p.registeredAddress || ''; $('profileBank').value = p.bankName || '';
    $('profileBankAccount').value = p.bankAccount || ''; $('profileRemark').value = p.remark || '';
    $('profileStatus').textContent = p.exists ? ('版本 ' + profileVersion) : '未配置';
  }
  function renderSummary(data) {
    const orderSummary = (((data || {}).paymentOrders || {}).summary || {});
    const values = [orderSummary.paidAmountCents, orderSummary.refundedAmountCents, orderSummary.netPaidAmountCents, orderSummary.netInvoicedCents, orderSummary.invoiceAvailableCents, orderSummary.creditNoteDueCents];
    $('billingSummary').innerHTML = ['累计支付','累计退款','净支付','净开票','可申请开票','待红冲'].map((label, i) => '<div class="tile"><div class="label">' + label + '</div><div class="value money">' + money(values[i]) + ' CNY</div></div>').join('');
  }
  function renderOrders(items) {
    $('orderCount').textContent = '共 ' + items.length + ' 条';
    if (!items.length) { $('orders').innerHTML = '<tr><td class="empty" colspan="6">暂无订单</td></tr>'; return; }
    $('orders').innerHTML = items.map(item => '<tr><td><strong>' + esc(item.orderNo) + '</strong><br><span class="muted">' + esc(item.paidAt || item.createdAt) + '</span></td><td>' + esc(item.packageName || item.packageCode) + '</td><td>' + badge(item.status) + '<br><span class="money">' + money(item.amountCents) + ' ' + esc(item.currency) + '</span></td><td>已退 ' + money(item.refundedAmountCents) + '<br><span class="muted">待退 ' + money(item.refundPendingCents) + '</span></td><td>净开 ' + money(item.netInvoicedCents) + '<br><span class="muted">可开 ' + money(item.invoiceAvailableCents) + ' / 待红 ' + money(item.creditNoteDueCents) + '</span></td><td>' + (Number(item.invoiceAvailableCents) > 0 ? '<button class="small invoice-from-order" data-order="' + esc(item.orderNo) + '" data-amount="' + Number(item.invoiceAvailableCents) + '" data-currency="' + esc(item.currency) + '">申请开票</button>' : '-') + '</td></tr>').join('');
  }
  function renderInvoices(items, summary) {
    $('invoiceCount').textContent = '共 ' + Number((summary || {}).documentCount || 0) + ' 条，净开票 ' + money((summary || {}).netIssuedAmountCents) + ' CNY';
    if (!items.length) { $('invoices').innerHTML = '<tr><td class="empty" colspan="7">暂无发票</td></tr>'; return; }
    $('invoices').innerHTML = items.map(item => '<tr><td>' + badge(item.status) + '</td><td><strong>' + esc(item.documentNo) + '</strong><br><span class="muted">' + (item.kind === 'credit_note' ? '红票' : '蓝票') + (item.originalDocumentNo ? ' / ' + esc(item.originalDocumentNo) : '') + '</span></td><td>' + esc(item.orderNo) + '</td><td class="money">' + money(item.amountCents) + ' ' + esc(item.currency) + '</td><td>' + esc(item.invoiceTitle) + '<br><span class="muted">' + esc(item.taxIdentifier) + '</span></td><td>' + (item.documentUrl ? '<a href="' + esc(item.documentUrl) + '" target="_blank" rel="noopener">查看单据</a>' : esc(item.providerDocumentNo || item.failureMessage || '-')) + '</td><td>' + (item.status === 'requested' && item.kind === 'invoice' ? '<button class="small danger cancel-invoice" data-no="' + esc(item.documentNo) + '" data-version="' + Number(item.version) + '">取消</button>' : '-') + '</td></tr>').join('');
  }
  function renderRefunds(items, summary) {
    $('refundCount').textContent = '已退款 ' + money((summary || {}).succeededAmountCents) + ' CNY';
    if (!items.length) { $('refunds').innerHTML = '<tr><td class="empty" colspan="6">暂无退款</td></tr>'; return; }
    $('refunds').innerHTML = items.map(item => '<tr><td><strong>' + esc(item.refundNo) + '</strong></td><td>' + esc(item.orderNo) + '</td><td>' + badge(item.status) + '</td><td class="money">' + money(item.amountCents) + ' ' + esc(item.currency) + '</td><td>' + esc(item.reason) + '</td><td>' + esc(item.succeededAt || item.failedAt || item.requestedAt) + '</td></tr>').join('');
  }
  async function loadSummary() {
    $('pageStatus').textContent = '加载中';
    try {
      const data = await api('/dashboard/saasBilling/summary');
      $('tenantLabel').textContent = '租户 ' + data.tenantId + ' · 订单、退款与发票';
      renderProfile(data.profile); renderSummary(data);
      renderOrders(((data.paymentOrders || {}).orders || []));
      renderInvoices(((data.invoices || {}).documents || []), ((data.invoices || {}).summary || {}));
      renderRefunds(((data.paymentRefunds || {}).refunds || []), ((data.paymentRefunds || {}).summary || {}));
      $('pageStatus').textContent = '已更新';
    } catch (err) { $('pageStatus').textContent = err.message || String(err); }
  }
  async function saveProfile() {
    $('profileStatus').textContent = '保存中';
    const body = {invoiceType:$('profileType').value,invoiceTitle:$('profileTitleInput').value.trim(),taxIdentifier:$('profileTaxId').value.trim(),email:$('profileEmail').value.trim(),phone:$('profilePhone').value.trim(),recipientName:$('profileRecipient').value.trim(),registeredAddress:$('profileAddress').value.trim(),bankName:$('profileBank').value.trim(),bankAccount:$('profileBankAccount').value.trim(),remark:$('profileRemark').value.trim(),expectedVersion:profileVersion};
    try { const data = await api('/dashboard/saasBilling/invoiceProfile',{method:'POST',body:JSON.stringify(body)}); renderProfile(data.profile); $('profileStatus').textContent = '已保存'; }
    catch (err) { $('profileStatus').textContent = err.message || String(err); }
  }
  async function requestInvoice() {
    $('requestStatus').textContent = '提交中';
    const orderNo = $('invoiceOrderNo').value.trim();
    const body = {orderNo,amount:$('invoiceAmount').value.trim(),currency:$('invoiceCurrency').value.trim(),idempotencyKey:$('invoiceIdempotency').value.trim() || ('billing-page:' + orderNo + ':' + Date.now()),remark:$('invoiceRemark').value.trim()};
    try { const data = await api('/dashboard/saasBilling/invoice',{method:'POST',body:JSON.stringify(body)}); $('invoiceIdempotency').value = body.idempotencyKey; $('requestStatus').textContent = '已提交 ' + data.document.documentNo; await loadSummary(); }
    catch (err) { $('requestStatus').textContent = err.message || String(err); }
  }
  async function loadInvoices() {
    const params = new URLSearchParams({kind:$('invoiceKind').value,status:$('invoiceStatus').value,keyword:$('invoiceKeyword').value.trim(),limit:'100'});
    try { const data = await api('/dashboard/saasBilling/invoices?' + params.toString()); renderInvoices(data.documents || [], data.summary || {}); }
    catch (err) { $('invoiceCount').textContent = err.message || String(err); }
  }
  $('saveToken').addEventListener('click', async () => { localStorage.setItem('mochat_go_dashboard_token', tokenInput.value.trim().replace(/^Bearer\s+/i,'')); $('pageStatus').textContent = '凭证已保存'; await loadSummary(); });
  $('loadSummary').addEventListener('click', loadSummary); $('refreshAll').addEventListener('click', loadSummary);
  $('saveProfile').addEventListener('click', saveProfile); $('requestInvoice').addEventListener('click', requestInvoice); $('loadInvoices').addEventListener('click', loadInvoices);
  $('orders').addEventListener('click', event => { const button = event.target.closest('.invoice-from-order'); if (!button) return; $('invoiceOrderNo').value = button.dataset.order; $('invoiceAmount').value = (Number(button.dataset.amount || 0) / 100).toFixed(2); $('invoiceCurrency').value = button.dataset.currency || 'CNY'; $('invoiceRemark').focus(); });
  $('invoices').addEventListener('click', async event => { const button = event.target.closest('.cancel-invoice'); if (!button || !confirm('确认取消该开票申请？')) return; try { await api('/dashboard/saasBilling/invoiceCancel',{method:'POST',body:JSON.stringify({documentNo:button.dataset.no,expectedVersion:Number(button.dataset.version),reason:'租户取消开票申请'})}); await loadSummary(); } catch (err) { $('invoiceCount').textContent = err.message || String(err); } });
  if (tokenInput.value.trim()) loadSummary();
</script>
</body>
</html>`
