package inventory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Route struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	File   string `json:"file"`
}

type Table struct {
	Name string `json:"name"`
}

type Crontab struct {
	Name string `json:"name"`
	Rule string `json:"rule"`
	Memo string `json:"memo"`
	File string `json:"file"`
}

type EventHandler struct {
	EventPath string `json:"event_path"`
	File      string `json:"file"`
}

type AsyncQueue struct {
	Pool string `json:"pool"`
	File string `json:"file"`
}

type Report struct {
	SourceRevision string         `json:"source_revision"`
	Routes         []Route        `json:"routes"`
	Tables         []Table        `json:"tables"`
	Crontabs       []Crontab      `json:"crontabs"`
	EventHandlers  []EventHandler `json:"event_handlers"`
	AsyncQueues    []AsyncQueue   `json:"async_queues"`
}

var (
	requestMappingRe = regexp.MustCompile(`(?m)^[ \t]*\*\s*@RequestMapping\(path="([^"]+)",\s*methods="?([^")]+)`)
	manualAddRouteRe = regexp.MustCompile(`Router::addRoute\(\[([^\]]+)\],\s*'([^']+)'`)
	manualGetRouteRe = regexp.MustCompile(`Router::get\('([^']+)'`)
	createTableRe    = regexp.MustCompile("CREATE TABLE IF NOT EXISTS `([^`]+)`")
	crontabRe        = regexp.MustCompile(`(?m)^[ \t]*\*\s*@Crontab\(([^)]*)\)`)
	eventHandlerRe   = regexp.MustCompile(`(?m)^[ \t]*\*\s*@WeChatEventHandler(?:\(eventPath="([^"]*)"\))?`)
	asyncQueueRe     = regexp.MustCompile(`(?m)^[ \t]*\*\s*@AsyncQueueMessage\(pool="([^"]+)"`)
	quotedFieldRe    = regexp.MustCompile(`%s="([^"]*)"`)
)

func Scan(sourceRoot, sourceRevision string) (Report, error) {
	routes, err := ScanRoutes(sourceRoot)
	if err != nil {
		return Report{}, err
	}
	tables, err := ScanTables(filepath.Join(sourceRoot, "api-server", "storage", "install", "mochat.sql"))
	if err != nil {
		return Report{}, err
	}
	crontabs, err := ScanCrontabs(sourceRoot)
	if err != nil {
		return Report{}, err
	}
	events, err := ScanEventHandlers(sourceRoot)
	if err != nil {
		return Report{}, err
	}
	queues, err := ScanAsyncQueues(sourceRoot)
	if err != nil {
		return Report{}, err
	}

	return Report{
		SourceRevision: sourceRevision,
		Routes:         routes,
		Tables:         tables,
		Crontabs:       crontabs,
		EventHandlers:  events,
		AsyncQueues:    queues,
	}, nil
}

func ScanRoutes(sourceRoot string) ([]Route, error) {
	var routes []Route
	err := walkPHP(sourceRoot, func(path string, content []byte) error {
		rel := relativePath(sourceRoot, path)
		for _, match := range requestMappingRe.FindAllSubmatch(content, -1) {
			pathValue := string(match[1])
			for _, method := range splitMethods(string(match[2])) {
				routes = append(routes, Route{Method: method, Path: pathValue, File: rel})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	manualRoutes, err := scanManualRoutes(sourceRoot)
	if err != nil {
		return nil, err
	}
	routes = append(routes, manualRoutes...)
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

func scanManualRoutes(sourceRoot string) ([]Route, error) {
	routesPath := filepath.Join(sourceRoot, "api-server", "config", "routes.php")
	content, err := os.ReadFile(routesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	rel := relativePath(sourceRoot, routesPath)
	var routes []Route
	for _, match := range manualAddRouteRe.FindAllSubmatch(content, -1) {
		methodList := strings.ReplaceAll(string(match[1]), "'", "")
		for _, method := range splitMethods(methodList) {
			routes = append(routes, Route{Method: method, Path: string(match[2]), File: rel})
		}
	}
	for _, match := range manualGetRouteRe.FindAllSubmatch(content, -1) {
		routes = append(routes, Route{Method: "GET", Path: string(match[1]), File: rel})
	}
	return routes, nil
}

func ScanTables(sqlPath string) ([]Table, error) {
	content, err := os.ReadFile(sqlPath)
	if err != nil {
		return nil, err
	}
	var tables []Table
	for _, match := range createTableRe.FindAllSubmatch(content, -1) {
		tables = append(tables, Table{Name: string(match[1])})
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })
	return tables, nil
}

func ScanCrontabs(sourceRoot string) ([]Crontab, error) {
	var crontabs []Crontab
	err := walkPHP(sourceRoot, func(path string, content []byte) error {
		rel := relativePath(sourceRoot, path)
		for _, match := range crontabRe.FindAllSubmatch(content, -1) {
			body := string(match[1])
			crontabs = append(crontabs, Crontab{
				Name: field(body, "name"),
				Rule: field(body, "rule"),
				Memo: field(body, "memo"),
				File: rel,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(crontabs, func(i, j int) bool { return crontabs[i].Name < crontabs[j].Name })
	return crontabs, nil
}

func ScanEventHandlers(sourceRoot string) ([]EventHandler, error) {
	var handlers []EventHandler
	err := walkPHP(sourceRoot, func(path string, content []byte) error {
		rel := relativePath(sourceRoot, path)
		for _, match := range eventHandlerRe.FindAllSubmatch(content, -1) {
			handlers = append(handlers, EventHandler{EventPath: string(match[1]), File: rel})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(handlers, func(i, j int) bool {
		if handlers[i].EventPath == handlers[j].EventPath {
			return handlers[i].File < handlers[j].File
		}
		return handlers[i].EventPath < handlers[j].EventPath
	})
	return handlers, nil
}

func ScanAsyncQueues(sourceRoot string) ([]AsyncQueue, error) {
	var queues []AsyncQueue
	err := walkPHP(sourceRoot, func(path string, content []byte) error {
		rel := relativePath(sourceRoot, path)
		for _, match := range asyncQueueRe.FindAllSubmatch(content, -1) {
			queues = append(queues, AsyncQueue{Pool: string(match[1]), File: rel})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(queues, func(i, j int) bool {
		if queues[i].Pool == queues[j].Pool {
			return queues[i].File < queues[j].File
		}
		return queues[i].Pool < queues[j].Pool
	})
	return queues, nil
}

func WriteJSON(report Report, path string) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}

func WriteMarkdownReports(report Report, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "接口清单.md"), []byte(renderRoutes(report)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "数据库清单.md"), []byte(renderTables(report)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "异步任务清单.md"), []byte(renderAsync(report)), 0644); err != nil {
		return err
	}
	return WriteJSON(report, filepath.Join(outDir, "compat_manifest.json"))
}

func renderRoutes(report Report) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# MoChat 接口清单\n\n")
	fmt.Fprintf(&b, "- 上游版本：`%s`\n", report.SourceRevision)
	fmt.Fprintf(&b, "- 注解路由数量：`%d`\n", len(report.Routes))
	fmt.Fprintf(&b, "- 独立化策略：standalone 运行时使用 Go 内置清单，清单内业务路由必须由 Go handler 承接；PHP upstream 仅作为迁移期接口对照工具，不作为独立版完成标准。\n\n")
	fmt.Fprintf(&b, "| 方法 | 路径 | PHP 源文件 |\n")
	fmt.Fprintf(&b, "| --- | --- | --- |\n")
	for _, route := range report.Routes {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` |\n", route.Method, route.Path, route.File)
	}
	return b.String()
}

func renderTables(report Report) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# MoChat 数据库清单\n\n")
	fmt.Fprintf(&b, "- 上游版本：`%s`\n", report.SourceRevision)
	fmt.Fprintf(&b, "- 主安装 SQL 表数量：`%d`\n", len(report.Tables))
	fmt.Fprintf(&b, "- 独立化要求：Go 版继续兼容原 `mc_` 业务表结构，并由 `deploy/standalone` 内置 schema 和 migrations 初始化；运行时不依赖 PHP 进程或原项目 SQL 文件挂载。\n\n")
	fmt.Fprintf(&b, "| 序号 | 表名 |\n")
	fmt.Fprintf(&b, "| ---: | --- |\n")
	for i, table := range report.Tables {
		fmt.Fprintf(&b, "| %d | `%s` |\n", i+1, table.Name)
	}
	return b.String()
}

func renderAsync(report Report) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# MoChat 异步任务清单\n\n")
	fmt.Fprintf(&b, "- 上游版本：`%s`\n", report.SourceRevision)
	fmt.Fprintf(&b, "- 定时任务数量：`%d`\n", len(report.Crontabs))
	fmt.Fprintf(&b, "- 企微事件处理器数量：`%d`\n", len(report.EventHandlers))
	fmt.Fprintf(&b, "- 异步队列注解数量：`%d`\n\n", len(report.AsyncQueues))

	fmt.Fprintf(&b, "## 定时任务\n\n")
	fmt.Fprintf(&b, "| 名称 | 规则 | 说明 | PHP 源文件 |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- |\n")
	for _, task := range report.Crontabs {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | `%s` |\n", task.Name, task.Rule, task.Memo, task.File)
	}

	fmt.Fprintf(&b, "\n## 企微事件处理器\n\n")
	fmt.Fprintf(&b, "| 事件路径 | PHP 源文件 |\n")
	fmt.Fprintf(&b, "| --- | --- |\n")
	for _, handler := range report.EventHandlers {
		eventPath := handler.EventPath
		if eventPath == "" {
			eventPath = "(默认处理器)"
		}
		fmt.Fprintf(&b, "| `%s` | `%s` |\n", eventPath, handler.File)
	}

	fmt.Fprintf(&b, "\n## 异步队列\n\n")
	fmt.Fprintf(&b, "| 队列池 | PHP 源文件 |\n")
	fmt.Fprintf(&b, "| --- | --- |\n")
	for _, queue := range report.AsyncQueues {
		fmt.Fprintf(&b, "| `%s` | `%s` |\n", queue.Pool, queue.File)
	}
	return b.String()
}

func walkPHP(sourceRoot string, fn func(path string, content []byte) error) error {
	roots := []string{
		filepath.Join(sourceRoot, "api-server", "app"),
		filepath.Join(sourceRoot, "api-server", "plugin"),
	}
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".php" {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return fn(path, content)
		}); err != nil {
			return err
		}
	}
	return nil
}

func splitMethods(raw string) []string {
	parts := strings.Split(raw, ",")
	methods := make([]string, 0, len(parts))
	for _, part := range parts {
		method := strings.ToUpper(strings.TrimSpace(strings.Trim(part, `"`)))
		if method != "" {
			methods = append(methods, method)
		}
	}
	return methods
}

func field(body, name string) string {
	re := regexp.MustCompile(fmt.Sprintf(quotedFieldRe.String(), regexp.QuoteMeta(name)))
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
