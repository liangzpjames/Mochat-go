package frontend

import (
	"mime"
	"net/http"
	"os"
	urlpath "path"
	"path/filepath"
	"strings"
)

type DashboardConfig struct {
	DistDir string
}

type AppConfig struct {
	DistDir   string
	MountPath string
}

func WrapDashboard(next http.Handler, cfg DashboardConfig) http.Handler {
	return WrapApp(next, AppConfig{DistDir: cfg.DistDir})
}

func WrapApp(next http.Handler, cfg AppConfig) http.Handler {
	handler := &appHandler{next: next, distDir: cleanDistDir(cfg.DistDir), mountPath: cleanMountPath(cfg.MountPath)}
	if handler.distDir == "." || handler.distDir == "" || !fileExists(filepath.Join(handler.distDir, "index.html")) {
		return next
	}
	return handler
}

func DistAvailable(distDir string) bool {
	distDir = cleanDistDir(distDir)
	return distDir != "." && distDir != "" && fileExists(filepath.Join(distDir, "index.html"))
}

type appHandler struct {
	next      http.Handler
	distDir   string
	mountPath string
}

func (h *appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.next.ServeHTTP(w, r)
		return
	}
	pathForFile, ok := h.pathForFileLookup(w, r)
	if !ok {
		return
	}
	if h.mountPath == "" && reservedPath(r.URL.Path) {
		h.next.ServeHTTP(w, r)
		return
	}
	target, ok := h.fileForPath(pathForFile)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if h.serveRewrittenFile(w, r, target) {
		return
	}
	http.ServeFile(w, r, target)
}

func (h *appHandler) pathForFileLookup(w http.ResponseWriter, r *http.Request) (string, bool) {
	if h.mountPath == "" {
		return r.URL.Path, true
	}
	trimmedMount := strings.TrimSuffix(h.mountPath, "/")
	if r.URL.Path == trimmedMount {
		http.Redirect(w, r, h.mountPath, http.StatusMovedPermanently)
		return "", false
	}
	if !strings.HasPrefix(r.URL.Path, h.mountPath) {
		h.next.ServeHTTP(w, r)
		return "", false
	}
	rel := strings.TrimPrefix(r.URL.Path, h.mountPath)
	if rel == "" {
		return "/", true
	}
	return "/" + rel, true
}

func (h *appHandler) fileForPath(path string) (string, bool) {
	rel := strings.TrimPrefix(path, "/")
	if rel == "" {
		rel = "index.html"
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == "." {
		cleanRel = "index.html"
	}
	if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
		return "", false
	}
	target := filepath.Join(h.distDir, cleanRel)
	if fileExists(target) {
		return target, true
	}
	if filepath.Ext(cleanRel) != "" {
		return "", false
	}
	return filepath.Join(h.distDir, "index.html"), true
}

func (h *appHandler) serveRewrittenFile(w http.ResponseWriter, r *http.Request, target string) bool {
	ext := strings.ToLower(filepath.Ext(target))
	if ext != ".html" && ext != ".js" {
		return false
	}
	body, err := os.ReadFile(target)
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	if ext == ".js" {
		contentType = "application/javascript; charset=utf-8"
	}
	rewritten := h.rewriteFrontend(string(body), ext)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Method == http.MethodHead {
		return true
	}
	_, _ = w.Write([]byte(rewritten))
	return true
}

func (h *appHandler) rewriteFrontend(body string, ext string) string {
	if ext == ".js" {
		// The upstream dashboard has a second Axios client pinned to api.mo.chat.
		// Keep product metadata on the standalone origin and its Go handler.
		body = strings.ReplaceAll(body, `baseURL:"//api.mo.chat"`, `baseURL:"/dashboard"`)
	}
	if ext == ".html" {
		body = strings.NewReplacer(
			`src="http://startupscrmmochat.duoduoker.com/cdn/vue@2.6.10distvue.min.js"`, `src="/vendor/vue-2.6.10.min.js"`,
			`src="http://startupscrmmochat.duoduoker.com/cdn/vue-router@3.1.3distvue-router.min.js"`, `src="/vendor/vue-router-3.1.3.min.js"`,
			`src="http://startupscrmmochat.duoduoker.com/cdn/vuex@3.1.1distvuex.min.js"`, `src="/vendor/vuex-3.1.1.min.js"`,
			`src="http://startupscrmmochat.duoduoker.com/cdn/axios@0.19.0distaxios.min.js"`, `src="/vendor/axios-0.19.0.min.js"`,
		).Replace(body)
		body = stripLegacyDashboardAnalytics(body)
	}
	prefix := h.mountPath
	if prefix == "" {
		return body
	}
	if ext == ".html" {
		replacer := strings.NewReplacer(
			`href="/favicon.ico"`, `href="`+prefix+`favicon.ico"`,
			`href="/assets/`, `href="`+prefix+`assets/`,
			`href="/css/`, `href="`+prefix+`css/`,
			`href="/js/`, `href="`+prefix+`js/`,
			`href="/img/`, `href="`+prefix+`img/`,
			`src="/assets/`, `src="`+prefix+`assets/`,
			`src="/js/`, `src="`+prefix+`js/`,
			`src="/img/`, `src="`+prefix+`img/`,
			`src="/vendor/`, `src="`+prefix+`vendor/`,
		)
		return replacer.Replace(body)
	}
	replacer := strings.NewReplacer(
		`.p="/"`, `.p="`+prefix+`"`,
		`base:"/"`, `base:"`+prefix+`"`,
		`Object(f["b"])("/")`, `Object(f["b"])("`+prefix+`")`,
		`Object({NODE_ENV:"production",BASE_URL:"/"}).VUE_APP_API_BASE_URL+"/sidebar"`, `"/sidebar"`,
		`Object({NODE_ENV:"production",BASE_URL:"/"}).VUE_APP_API_BASE_URL+"/operation"`, `"/operation"`,
	)
	return replacer.Replace(body)
}

func stripLegacyDashboardAnalytics(body string) string {
	const marker = `<script>var _hmt = _hmt || [];`
	start := strings.Index(body, marker)
	if start < 0 {
		return body
	}
	relativeEnd := strings.Index(body[start:], `</script>`)
	if relativeEnd < 0 {
		return body
	}
	end := start + relativeEnd + len(`</script>`)
	return body[:start] + body[end:]
}

func cleanDistDir(distDir string) string {
	return filepath.Clean(strings.TrimSpace(distDir))
}

func cleanMountPath(mountPath string) string {
	mountPath = strings.TrimSpace(mountPath)
	if mountPath == "" || mountPath == "/" {
		return ""
	}
	mountPath = "/" + strings.Trim(mountPath, "/")
	mountPath = urlpath.Clean(mountPath)
	if mountPath == "/" || mountPath == "." {
		return ""
	}
	return strings.TrimRight(mountPath, "/") + "/"
}

func reservedPath(path string) bool {
	if path == "/favicon.ico" {
		return false
	}
	if strings.HasPrefix(path, "/WW_verify_") && strings.HasSuffix(path, ".txt") {
		return true
	}
	if path == "/healthz" || path == "/readyz" || path == "/compat/status" || path == "/compat/routes" {
		return true
	}
	if path == "/roomTagPull/clientDetails" {
		return true
	}
	for _, prefix := range []string{
		"/api/",
		"/r/",
		"/dashboard/",
		"/security/",
		"/sidebar/",
		"/operation/",
		"/undefined/dashboard/",
		"/undefined/sidebar/",
		"/undefined/operation/",
		"/weWork/",
		"/Task/",
		"/load/",
		"/auth/",
		"/openUserInfo/",
		"/static/",
		"/compat/",
		"/_legacy/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
