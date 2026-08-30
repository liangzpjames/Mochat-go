// Package integrationfixtureaudit provides AST-backed governance for MySQL
// integration fixtures. It intentionally understands Go structure while SQL
// object allowlists remain explicit at the test-function boundary.
package integrationfixtureaudit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const DSNEnvironmentVariable = "MOCHAT_GO_MYSQL_INTEGRATION_DSN"

type OperationAllowance struct {
	Ref       string
	Operation string
	Object    string
}

type Config struct {
	RootFunctions       []string
	AllowedRunnerRefs   []string
	AllowedOperations   []OperationAllowance
	AllowedLedgerDelete map[string]string
	ForbiddenCalls      []string
	AllowedCallRefs     []string
}

type parsedFunction struct {
	path        string
	pkg         string
	name        string
	declaration *ast.FuncDecl
	calls       map[string]bool
	globals     map[string]string
	directRoot  bool
}

var (
	createTablePattern    = regexp.MustCompile(`(?is)\bCREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+(?:(?:[%[:alnum:]_]+|` + "`" + `[^` + "`" + `]+` + "`" + `)\.)?[` + "`" + `]?([[:alnum:]_]+)` + "`" + `?`)
	dropTablePattern      = regexp.MustCompile(`(?is)\bDROP\s+TABLE(?:\s+IF\s+EXISTS)?\s+(?:(?:[%[:alnum:]_]+|` + "`" + `[^` + "`" + `]+` + "`" + `)\.)?[` + "`" + `]?([[:alnum:]_]+)` + "`" + `?`)
	createDatabasePattern = regexp.MustCompile(`(?is)\bCREATE\s+DATABASE(?:\s+IF\s+NOT\s+EXISTS)?\s+[` + "`" + `]?([[:alnum:]_]+)?` + "`" + `?`)
	ledgerPattern         = regexp.MustCompile(`(?is)\b(CREATE\s+TABLE|DROP\s+TABLE|INSERT\s+INTO|UPDATE|DELETE\s+FROM)\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?[` + "`" + `]?MOCHAT_GO_SCHEMA_MIGRATIONS` + "`" + `?`)
	versionPattern        = regexp.MustCompile(`\b[0-9]{4}_[[:alnum:]_]+\b`)
)

func Ref(path, function string) string {
	return filepath.ToSlash(path) + ":" + function
}

func LoadTestSources(root string, excludedRelativePaths ...string) (map[string][]byte, error) {
	excluded := map[string]bool{}
	for _, path := range excludedRelativePaths {
		excluded[filepath.ToSlash(filepath.Clean(path))] = true
	}
	sources := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if excluded[relative] {
			return nil
		}
		sources[relative] = body
		return nil
	})
	return sources, err
}

func Audit(sources map[string][]byte, config Config) []string {
	rootNames := stringSet(config.RootFunctions)
	allowedRunnerRefs := stringSet(config.AllowedRunnerRefs)
	forbiddenCalls := stringSet(config.ForbiddenCalls)
	allowedCallRefs := stringSet(config.AllowedCallRefs)
	allowedOperations := map[string]map[string]map[string]bool{}
	for _, allowance := range config.AllowedOperations {
		operation := strings.ToUpper(allowance.Operation)
		object := strings.ToLower(allowance.Object)
		if allowedOperations[allowance.Ref] == nil {
			allowedOperations[allowance.Ref] = map[string]map[string]bool{}
		}
		if allowedOperations[allowance.Ref][operation] == nil {
			allowedOperations[allowance.Ref][operation] = map[string]bool{}
		}
		allowedOperations[allowance.Ref][operation][object] = true
	}

	functions, parseIssues := parseFunctions(sources, rootNames)
	if len(parseIssues) > 0 {
		return parseIssues
	}
	governed := governedFunctionClosure(functions, rootNames)
	functionReturns := resolveFunctionReturns(functions)
	issues := []string{}
	seen := map[string]bool{}
	addIssue := func(issue string) {
		if !seen[issue] {
			seen[issue] = true
			issues = append(issues, issue)
		}
	}

	for index, function := range functions {
		if !governed[index] {
			continue
		}
		ref := Ref(function.path, function.name)
		locals := copyStringMap(function.globals)
		if returned, ok := functionReturns[function.pkg+":"+function.name]; ok {
			auditLedgerText(ref, returned, config.AllowedLedgerDelete, addIssue)
		}
		ast.Inspect(function.declaration.Body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.AssignStmt:
				for valueIndex, expression := range typed.Rhs {
					if valueIndex >= len(typed.Lhs) {
						break
					}
					name, ok := typed.Lhs[valueIndex].(*ast.Ident)
					if !ok {
						continue
					}
					if value, ok := resolveStringExpression(expression, locals, functionReturns, function.pkg); ok {
						locals[name.Name] = value
					}
				}
			case *ast.DeclStmt:
				resolveLocalStringDeclaration(typed.Decl, locals, functionReturns, function.pkg)
			case *ast.CallExpr:
				name := calledName(typed.Fun)
				if name == "NewRunner" {
					if !allowedRunnerRefs[ref] {
						addIssue(fmt.Sprintf("%s constructs a local migration runner", ref))
					} else if !isProductionPrefixRunner(function.declaration, typed) {
						addIssue(fmt.Sprintf("%s does not construct the exact production prefix from DefaultMigrations[:index+1]", ref))
					}
				}
				if forbiddenCalls[name] && !allowedCallRefs[ref+":"+name] {
					addIssue(fmt.Sprintf("%s bypasses the production registry through %s", ref, name))
				}
				auditLedgerCall(ref, typed, locals, functionReturns, function.pkg, config.AllowedLedgerDelete, addIssue)
			case *ast.CompositeLit:
				if isMigrationSlice(typed.Type) {
					addIssue(fmt.Sprintf("%s handwrites a migration slice", ref))
				}
			case *ast.BasicLit:
				if typed.Kind != token.STRING {
					break
				}
				text, err := strconv.Unquote(typed.Value)
				if err != nil {
					break
				}
				for _, operation := range sqlOperations(text) {
					if strings.EqualFold(operation.Object, "mochat_go_schema_migrations") {
						continue
					}
					if !allowedOperations[ref][operation.Name][strings.ToLower(operation.Object)] {
						addIssue(fmt.Sprintf("%s performs non-allowlisted %s on %s", ref, operation.Name, operation.Object))
					}
				}
			}
			return true
		})
	}
	sort.Strings(issues)
	return issues
}

func parseFunctions(sources map[string][]byte, rootNames map[string]bool) ([]parsedFunction, []string) {
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	functions := []parsedFunction{}
	issues := []string{}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, sources[path], 0)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s: parse fixture source: %v", path, err))
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			parsed := parsedFunction{path: path, pkg: file.Name.Name, name: function.Name.Name, declaration: function, calls: map[string]bool{}, globals: resolveFileStringValues(file)}
			if rootNames[parsed.name] {
				parsed.directRoot = true
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				switch typed := node.(type) {
				case *ast.CallExpr:
					name := calledName(typed.Fun)
					if name != "" {
						parsed.calls[name] = true
						if rootNames[name] {
							parsed.directRoot = true
						}
					}
					if isStructuredDatabaseSink(typed.Fun) {
						parsed.directRoot = true
					}
				case *ast.BasicLit:
					if typed.Kind == token.STRING {
						value, err := strconv.Unquote(typed.Value)
						if err == nil && value == DSNEnvironmentVariable {
							parsed.directRoot = true
						}
					}
				}
				return true
			})
			functions = append(functions, parsed)
		}
	}
	return functions, issues
}

func governedFunctionClosure(functions []parsedFunction, rootNames map[string]bool) map[int]bool {
	governed := map[int]bool{}
	governedNames := map[string]map[string]bool{}
	for index, function := range functions {
		if function.directRoot {
			governed[index] = true
			if governedNames[function.pkg] == nil {
				governedNames[function.pkg] = map[string]bool{}
			}
			governedNames[function.pkg][function.name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for index, function := range functions {
			if governed[index] {
				continue
			}
			for call := range function.calls {
				if rootNames[call] || governedNames[function.pkg][call] {
					governed[index] = true
					if governedNames[function.pkg] == nil {
						governedNames[function.pkg] = map[string]bool{}
					}
					governedNames[function.pkg][function.name] = true
					changed = true
					break
				}
			}
		}
	}
	entry := map[int]bool{}
	for index := range governed {
		entry[index] = true
	}
	local := map[string]map[string][]int{}
	for index, function := range functions {
		if local[function.pkg] == nil {
			local[function.pkg] = map[string][]int{}
		}
		local[function.pkg][function.name] = append(local[function.pkg][function.name], index)
	}
	queue := []int{}
	for index := range entry {
		queue = append(queue, index)
	}
	for len(queue) > 0 {
		index := queue[0]
		queue = queue[1:]
		function := functions[index]
		for call := range function.calls {
			for _, callee := range local[function.pkg][call] {
				if !governed[callee] {
					governed[callee] = true
					queue = append(queue, callee)
				}
			}
		}
	}
	return governed
}

type sqlOperation struct {
	Name   string
	Object string
}

func sqlOperations(text string) []sqlOperation {
	operations := []sqlOperation{}
	for _, match := range createTablePattern.FindAllStringSubmatch(text, -1) {
		operations = append(operations, sqlOperation{Name: "CREATE TABLE", Object: normalizedObject(match[1])})
	}
	for _, match := range dropTablePattern.FindAllStringSubmatch(text, -1) {
		operations = append(operations, sqlOperation{Name: "DROP TABLE", Object: normalizedObject(match[1])})
	}
	for _, match := range createDatabasePattern.FindAllStringSubmatch(text, -1) {
		operations = append(operations, sqlOperation{Name: "CREATE DATABASE", Object: normalizedObject(match[1])})
	}
	return operations
}

func normalizedObject(object string) string {
	if strings.TrimSpace(object) == "" {
		return "<dynamic>"
	}
	return strings.ToLower(object)
}

func auditLedgerCall(ref string, call *ast.CallExpr, locals, functionReturns map[string]string, pkg string, allowedDeletes map[string]string, addIssue func(string)) {
	values := []string{}
	for _, argument := range call.Args {
		if value, ok := resolveStringExpression(argument, locals, functionReturns, pkg); ok {
			values = append(values, value)
		}
	}
	auditLedgerText(ref, strings.Join(values, " "), allowedDeletes, addIssue)
}

func auditLedgerText(ref, text string, allowedDeletes map[string]string, addIssue func(string)) {
	match := ledgerPattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return
	}
	operation := strings.Join(strings.Fields(strings.ToUpper(match[1])), " ")
	if operation != "DELETE FROM" {
		addIssue(fmt.Sprintf("%s mutates the standard migration ledger through %s", ref, operation))
		return
	}
	versions := versionPattern.FindAllString(text, -1)
	allowedVersion := allowedDeletes[ref]
	if allowedVersion == "" || len(versions) != 1 || versions[0] != allowedVersion {
		addIssue(fmt.Sprintf("%s deletes the standard migration ledger without the exact controlled version", ref))
	}
}

func isProductionPrefixRunner(function *ast.FuncDecl, call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false
	}
	prefix, ok := call.Args[1].(*ast.SliceExpr)
	if !ok || prefix.Low != nil || prefix.Max != nil {
		return false
	}
	registry, ok := prefix.X.(*ast.Ident)
	if !ok {
		return false
	}
	high, ok := prefix.High.(*ast.BinaryExpr)
	if !ok || high.Op != token.ADD {
		return false
	}
	index, ok := high.X.(*ast.Ident)
	if !ok {
		return false
	}
	one, ok := high.Y.(*ast.BasicLit)
	if !ok || one.Kind != token.INT || one.Value != "1" {
		return false
	}
	loadsRegistry := false
	rangesRegistry := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for valueIndex, expression := range typed.Rhs {
				call, ok := expression.(*ast.CallExpr)
				if valueIndex >= len(typed.Lhs) || !ok || calledName(call.Fun) != "DefaultMigrations" {
					continue
				}
				if name, ok := typed.Lhs[valueIndex].(*ast.Ident); ok && name.Name == registry.Name {
					loadsRegistry = true
				}
			}
		case *ast.RangeStmt:
			ranged, ok := typed.X.(*ast.Ident)
			if !ok || ranged.Name != registry.Name {
				break
			}
			key, ok := typed.Key.(*ast.Ident)
			if ok && key.Name == index.Name {
				rangesRegistry = true
			}
		}
		return true
	})
	return loadsRegistry && rangesRegistry
}

func resolveFunctionReturns(functions []parsedFunction) map[string]string {
	resolved := map[string]string{}
	for changed := true; changed; {
		changed = false
		for _, function := range functions {
			key := function.pkg + ":" + function.name
			if _, exists := resolved[key]; exists {
				continue
			}
			locals := copyStringMap(function.globals)
			ast.Inspect(function.declaration.Body, func(node ast.Node) bool {
				switch typed := node.(type) {
				case *ast.AssignStmt:
					for valueIndex, expression := range typed.Rhs {
						if valueIndex >= len(typed.Lhs) {
							break
						}
						name, ok := typed.Lhs[valueIndex].(*ast.Ident)
						if !ok {
							continue
						}
						if value, ok := resolveStringExpression(expression, locals, resolved, function.pkg); ok {
							locals[name.Name] = value
						}
					}
				case *ast.DeclStmt:
					resolveLocalStringDeclaration(typed.Decl, locals, resolved, function.pkg)
				case *ast.ReturnStmt:
					if len(typed.Results) == 1 {
						if value, ok := resolveStringExpression(typed.Results[0], locals, resolved, function.pkg); ok {
							resolved[key] = value
							changed = true
						}
					}
				}
				return true
			})
		}
	}
	return resolved
}

func resolveFileStringValues(file *ast.File) map[string]string {
	values := map[string]string{}
	for changed := true; changed; {
		changed = false
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || (general.Tok != token.CONST && general.Tok != token.VAR) {
				continue
			}
			for _, specification := range general.Specs {
				valueSpec, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, expression := range valueSpec.Values {
					if index >= len(valueSpec.Names) {
						break
					}
					if _, exists := values[valueSpec.Names[index].Name]; exists {
						continue
					}
					if value, ok := resolveStringExpression(expression, values, nil, file.Name.Name); ok {
						values[valueSpec.Names[index].Name] = value
						changed = true
					}
				}
			}
		}
	}
	return values
}

func resolveLocalStringDeclaration(declaration ast.Decl, locals, functionReturns map[string]string, pkg string) {
	general, ok := declaration.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, specification := range general.Specs {
		valueSpec, ok := specification.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for index, expression := range valueSpec.Values {
			if index >= len(valueSpec.Names) {
				break
			}
			if value, ok := resolveStringExpression(expression, locals, functionReturns, pkg); ok {
				locals[valueSpec.Names[index].Name] = value
			}
		}
	}
}

func resolveStringExpression(expression ast.Expr, values, functionReturns map[string]string, pkg string) (string, bool) {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(typed.Value)
		return value, err == nil
	case *ast.Ident:
		value, ok := values[typed.Name]
		return value, ok
	case *ast.ParenExpr:
		return resolveStringExpression(typed.X, values, functionReturns, pkg)
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, leftOK := resolveStringExpression(typed.X, values, functionReturns, pkg)
		right, rightOK := resolveStringExpression(typed.Y, values, functionReturns, pkg)
		return left + right, leftOK && rightOK
	case *ast.CallExpr:
		value, ok := functionReturns[pkg+":"+calledName(typed.Fun)]
		return value, ok
	default:
		return "", false
	}
}

func copyStringMap(source map[string]string) map[string]string {
	copy := map[string]string{}
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func isStructuredDatabaseSink(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if selector.Sel.Name == "NewIsolated" || selector.Sel.Name == "ApplyThrough" || selector.Sel.Name == "ApplyLatest" {
		return true
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "sql" && selector.Sel.Name == "Open"
}

func isMigrationSlice(expression ast.Expr) bool {
	array, ok := expression.(*ast.ArrayType)
	if !ok {
		return false
	}
	element := array.Elt
	if star, ok := element.(*ast.StarExpr); ok {
		element = star.X
	}
	switch typed := element.(type) {
	case *ast.Ident:
		return typed.Name == "Migration"
	case *ast.SelectorExpr:
		return typed.Sel.Name == "Migration"
	default:
		return false
	}
}

func calledName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}
