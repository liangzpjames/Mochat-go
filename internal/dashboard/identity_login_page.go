package dashboard

import (
	"html/template"
	"net"
	"net/http"
	"strings"
)

type IdentityLoginPageHandler struct {
	branding SaaSBrandingReader
	domains  SaaSTenantDomainReader
	tenantID int
	prefill  identityLoginPrefill
}

type identityLoginPrefill struct {
	phone    string
	password string
}

func NewIdentityLoginPageHandler(branding SaaSBrandingReader, tenantID int) http.Handler {
	if tenantID <= 0 {
		tenantID = 1
	}
	return IdentityLoginPageHandler{branding: branding, tenantID: tenantID}
}

func NewIdentityLoginPageHandlerWithDomains(branding SaaSBrandingReader, domains SaaSTenantDomainReader, tenantID int) http.Handler {
	return NewIdentityLoginPageHandlerWithDomainsAndPrefill(branding, domains, tenantID, "", "")
}

func NewIdentityLoginPageHandlerWithDomainsAndPrefill(branding SaaSBrandingReader, domains SaaSTenantDomainReader, tenantID int, phone, password string) http.Handler {
	if tenantID <= 0 {
		tenantID = 1
	}
	return IdentityLoginPageHandler{
		branding: branding,
		domains:  domains,
		tenantID: tenantID,
		prefill: identityLoginPrefill{
			phone:    strings.TrimSpace(phone),
			password: password,
		},
	}
}

func (h IdentityLoginPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	tenantID := h.tenantID
	domain, found, err := ResolveSaaSTenantDomainRequest(r.Context(), h.domains, r.Host)
	if err != nil {
		http.Error(w, "租户域名服务暂不可用", http.StatusServiceUnavailable)
		return
	}
	if found && !domain.RoutingActive() {
		http.Error(w, "租户域名尚未启用", http.StatusMisdirectedRequest)
		return
	}
	if found {
		tenantID = domain.TenantID
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	profile := DefaultSaaSBrandingProfile(tenantID, "")
	if h.branding != nil {
		if stored, err := h.branding.SaaSBrandingProfile(r.Context(), tenantID); err == nil && stored.Status == SaaSBrandingStatusActive {
			profile = NormalizeSaaSBrandingProfile(stored)
		}
	}
	view := identityLoginPageView{
		ProductName: profile.ProductName, ProductSubtitle: profile.ProductSubtitle,
		LogoURL: template.URL(profile.LogoURL), FaviconURL: template.URL(profile.FaviconURL),
		LoginBackgroundURL: template.URL(profile.LoginBackgroundURL),
		PrimaryColor:       template.CSS(profile.PrimaryColor), AccentColor: template.CSS(profile.AccentColor),
	}
	if identityLoginPrefillAllowed(r.Host) {
		view.DefaultPhone = h.prefill.phone
		view.DefaultPassword = h.prefill.password
	}
	if err := identityLoginPageTemplate.Execute(w, view); err != nil {
		return
	}
}

type identityLoginPageView struct {
	ProductName        string
	ProductSubtitle    string
	LogoURL            template.URL
	FaviconURL         template.URL
	LoginBackgroundURL template.URL
	PrimaryColor       template.CSS
	AccentColor        template.CSS
	DefaultPhone       string
	DefaultPassword    string
}

func identityLoginPrefillAllowed(requestHost string) bool {
	host := strings.TrimSpace(requestHost)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

var identityLoginPageTemplate = template.Must(template.New("identity-login").Parse(identityLoginPageHTML))

const identityLoginPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
  <title>登录 - {{.ProductName}}</title>
  <link rel="icon" href="{{.FaviconURL}}">
  <style>
    :root { color-scheme: light; --ink:#152231; --muted:#667585; --line:#dce3e8; --blue:{{.PrimaryColor}}; --blue-hover:{{.AccentColor}}; --danger:#b42318; --surface:#ffffff; }
    * { box-sizing:border-box; }
    html, body { margin:0; min-height:100%; font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif; color:var(--ink); letter-spacing:0; }
    body { min-height:100vh; background:#eaf0f3 url('{{.LoginBackgroundURL}}') center/cover no-repeat fixed; }
    main { min-height:100vh; display:grid; grid-template-columns:minmax(300px,1fr) minmax(400px,520px); }
    .brand { display:flex; align-items:flex-start; padding:clamp(34px,6vh,72px) clamp(28px,7vw,104px); background:rgba(7,25,38,.48); color:#fff; }
    .brand-lockup { display:flex; align-items:center; gap:16px; }
    .brand img { width:52px; height:52px; object-fit:contain; }
    .brand-name { margin:0; font-size:32px; line-height:1.1; font-weight:720; }
    .brand-subtitle { margin:7px 0 0; font-size:15px; opacity:.82; }
    .login-shell { min-height:100vh; display:flex; align-items:center; padding:40px clamp(32px,5vw,72px); background:rgba(255,255,255,.96); box-shadow:-18px 0 42px rgba(12,31,45,.14); }
    .login { width:100%; max-width:380px; margin:auto; }
    h1 { margin:0 0 8px; font-size:28px; line-height:1.25; font-weight:700; }
    .lead { margin:0 0 34px; color:var(--muted); font-size:14px; }
    .field { margin-bottom:20px; }
    label { display:block; margin-bottom:8px; font-size:14px; font-weight:600; }
    input { width:100%; height:46px; border:1px solid var(--line); border-radius:6px; background:#fff; padding:0 13px; color:var(--ink); font:inherit; outline:none; transition:border-color .15s,box-shadow .15s; }
    input:focus { border-color:var(--blue); box-shadow:0 0 0 3px rgba(23,105,170,.12); }
    input::placeholder { color:#9aa6b1; }
    button { width:100%; height:46px; border:0; border-radius:6px; background:var(--blue); color:#fff; font-family:inherit; font-size:15px; font-weight:600; cursor:pointer; transition:background .15s; }
    button:hover { background:var(--blue-hover); }
    button:disabled { cursor:wait; opacity:.58; }
    .error { min-height:22px; margin:2px 0 12px; color:var(--danger); font-size:13px; line-height:1.5; }
    .hidden { display:none; }
    .back { width:auto; height:auto; margin:0 0 24px; padding:0; background:transparent; color:var(--blue); font-weight:600; }
    .back:hover { background:transparent; text-decoration:underline; }
    .method { margin:0 0 24px; padding:12px 14px; border-left:3px solid var(--blue); background:#eef5fa; color:#425467; font-size:13px; line-height:1.55; }
    @media (max-width:760px) {
      body { background-position:38% center; }
      main { grid-template-columns:1fr; grid-template-rows:132px minmax(0,1fr); }
      .brand { min-height:132px; align-items:center; padding:24px; background:rgba(7,25,38,.58); }
      .brand img { width:42px; height:42px; }
      .brand-name { font-size:25px; }
      .brand-subtitle { font-size:13px; }
      .login-shell { min-height:calc(100vh - 132px); align-items:flex-start; padding:38px 24px max(38px,env(safe-area-inset-bottom)); box-shadow:none; }
      h1 { font-size:25px; }
    }
  </style>
</head>
<body>
<main>
  <section class="brand" aria-label="{{.ProductName}}">
    <div class="brand-lockup">
      <img src="{{.LogoURL}}" alt="">
      <div><p class="brand-name">{{.ProductName}}</p><p class="brand-subtitle">{{.ProductSubtitle}}</p></div>
    </div>
  </section>
  <section class="login-shell">
    <div class="login">
      <form id="password-form" novalidate>
        <h1>账号登录</h1>
        <p class="lead">使用平台账号进入工作台</p>
        <div class="field"><label for="phone">手机号</label><input id="phone" name="phone" inputmode="numeric" autocomplete="username" maxlength="32" value="{{.DefaultPhone}}" required></div>
        <div class="field"><label for="password">密码</label><input id="password" name="password" type="password" autocomplete="current-password" maxlength="200" value="{{.DefaultPassword}}" required></div>
        <p id="password-error" class="error" role="alert"></p>
        <button id="password-submit" type="submit">登录</button>
      </form>
      <form id="mfa-form" class="hidden" novalidate>
        <button id="back" class="back" type="button">返回账号登录</button>
        <h1>二次认证</h1>
        <p class="lead">请输入认证器动态码或一次性恢复码</p>
        <p class="method">动态码为 6 位数字；恢复码可保留中间连字符。</p>
        <div class="field"><label for="mfa-code">验证码</label><input id="mfa-code" name="code" autocomplete="one-time-code" maxlength="32" required></div>
        <p id="mfa-error" class="error" role="alert"></p>
        <button id="mfa-submit" type="submit">验证并登录</button>
      </form>
    </div>
  </section>
</main>
<script>
(() => {
  const passwordForm = document.getElementById('password-form');
  const mfaForm = document.getElementById('mfa-form');
  const passwordButton = document.getElementById('password-submit');
  const mfaButton = document.getElementById('mfa-submit');
  let challengeToken = '';

  const message = (id, value) => { document.getElementById(id).textContent = value || ''; };
  const payload = async (response) => {
    let body = {};
    try { body = await response.json(); } catch (_) {}
    if (!response.ok || (body.code && body.code >= 400)) throw new Error(body.msg || '请求失败，请稍后重试');
    return body.data || {};
  };
  const finish = (data) => {
    const token = data.token;
    if (!token) throw new Error('登录响应缺少令牌');
    const seconds = Number(data.expire) > 0 ? Number(data.expire) : 604800;
    const secure = location.protocol === 'https:' ? '; Secure' : '';
	const authorization = 'Bearer ' + token;
	try {
	  localStorage.setItem('ACCESS_TOKEN', JSON.stringify(authorization));
	  localStorage.setItem('mochat_go_saas_admin_token', authorization);
	} catch (_) {}
	document.cookie = 'ACCESS_TOKEN=' + encodeURIComponent(authorization) + '; Path=/; Max-Age=' + Math.floor(seconds) + '; SameSite=Lax' + secure;
    const requested = new URLSearchParams(location.search).get('redirect') || '/';
    let target = '/';
    try {
      const candidate = new URL(requested, location.origin);
      if (candidate.origin === location.origin) target = candidate.pathname + candidate.search + candidate.hash;
    } catch (_) {}
    location.assign(target);
  };

  passwordForm.addEventListener('submit', async (event) => {
    event.preventDefault(); message('password-error', ''); passwordButton.disabled = true;
    try {
      const response = await fetch('/dashboard/user/auth', { method:'POST', headers:{'Content-Type':'application/json','Accept':'application/json'}, body:JSON.stringify({phone:document.getElementById('phone').value.trim(),password:document.getElementById('password').value}) });
      const data = await payload(response);
      if (data.mfaRequired) {
        challengeToken = data.challengeToken || '';
        if (!challengeToken) throw new Error('二次认证挑战无效');
        passwordForm.classList.add('hidden'); mfaForm.classList.remove('hidden');
        document.getElementById('mfa-code').focus();
      } else { finish(data); }
    } catch (error) { message('password-error', error.message); }
    finally { passwordButton.disabled = false; }
  });

  mfaForm.addEventListener('submit', async (event) => {
    event.preventDefault(); message('mfa-error', ''); mfaButton.disabled = true;
    try {
      const response = await fetch('/dashboard/user/authMFA', { method:'POST', headers:{'Content-Type':'application/json','Accept':'application/json'}, body:JSON.stringify({challengeToken,code:document.getElementById('mfa-code').value.trim()}) });
      finish(await payload(response));
    } catch (error) { message('mfa-error', error.message); document.getElementById('mfa-code').select(); }
    finally { mfaButton.disabled = false; }
  });

  document.getElementById('back').addEventListener('click', () => {
    challengeToken = ''; document.getElementById('mfa-code').value = ''; message('mfa-error', '');
    mfaForm.classList.add('hidden'); passwordForm.classList.remove('hidden'); document.getElementById('password').focus();
  });
})();
</script>
</body>
</html>`
