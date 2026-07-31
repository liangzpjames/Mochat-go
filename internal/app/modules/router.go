package modules

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"jiyi/mochat-go/internal/nilcheck"
)

var ErrDuplicateRoute = errors.New("duplicate route")

type RouteRegistrar interface {
	Handle(method, pattern string, handler http.Handler) error
}

type routeKey struct {
	method string
	path   string
}

type Router struct {
	mu     sync.RWMutex
	routes map[routeKey]http.Handler
}

func NewRouter() *Router {
	return &Router{routes: make(map[routeKey]http.Handler)}
}

func (r *Router) Handle(method, pattern string, handler http.Handler) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return errors.New("route method is required")
	}
	if pattern == "" || !strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "?") {
		return errors.New("route pattern must be an absolute path without a query string")
	}
	if nilcheck.IsNil(handler) {
		return errors.New("route handler is required")
	}

	key := routeKey{method: method, path: pattern}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.routes[key]; exists {
		return ErrDuplicateRoute
	}
	r.routes[key] = handler
	return nil
}

func (r *Router) Match(req *http.Request) (http.Handler, bool) {
	if req == nil || req.URL == nil {
		return nil, false
	}
	method := strings.ToUpper(req.Method)
	r.mu.RLock()
	handler, ok := r.routes[routeKey{method: method, path: req.URL.Path}]
	if ok {
		r.mu.RUnlock()
		return handler, true
	}
	for key, candidate := range r.routes {
		if key.method == method && routePatternMatches(key.path, req.URL.Path) {
			r.mu.RUnlock()
			return candidate, true
		}
	}
	r.mu.RUnlock()
	return nil, false
}

func routePatternMatches(pattern, actual string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	actualParts := strings.Split(strings.Trim(actual, "/"), "/")
	if len(patternParts) != len(actualParts) {
		return false
	}
	for i := range patternParts {
		part := patternParts[i]
		if strings.HasPrefix(part, ":") || (strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")) {
			continue
		}
		if part != actualParts[i] {
			return false
		}
	}
	return true
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if handler, ok := r.Match(req); ok {
		handler.ServeHTTP(w, req)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    http.StatusNotFound,
		"message": "route not found",
	})
}
