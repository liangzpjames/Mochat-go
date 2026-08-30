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
	path          string
	pkg           string
	name          string
	receiver      string
	receiverVar   string
	declaration   *ast.FuncDecl
	calls         map[string]bool
	callAliases   map[string]string
	receiverTypes map[string]string
	globals       map[string]string
	directRoot    bool
}

func (function parsedFunction) referenceName() string {
	if function.receiver == "" {
		return function.name
	}
	return function.receiver + "." + function.name
}

func (function parsedFunction) key() string {
	return function.pkg + ":" + function.referenceName()
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
	stringFunctions := packageStringFunctions(functions)
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
		ref := Ref(function.path, function.referenceName())
		locals := copyStringMap(function.globals)
		if returned, ok := functionReturns[function.key()]; ok {
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
					if value, ok := resolveStringExpressionDeep(expression, locals, functionReturns, stringFunctions, function.pkg, 0); ok {
						locals[name.Name] = value
					}
				}
			case *ast.DeclStmt:
				resolveLocalStringDeclarationDeep(typed.Decl, locals, functionReturns, stringFunctions, function.pkg)
			case *ast.CallExpr:
				canonical := canonicalCall(typed.Fun, function.callAliases, function)
				name := callableLeaf(canonical)
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
				auditLedgerCall(ref, typed, locals, functionReturns, stringFunctions, function.pkg, config.AllowedLedgerDelete, addIssue)
				auditInvokedLedgerHelper(canonical, typed.Args, locals, functionReturns, stringFunctions, function.pkg, config.AllowedLedgerDelete, addIssue, 0)
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
			parsed := parsedFunction{
				path:        path,
				pkg:         file.Name.Name,
				name:        function.Name.Name,
				receiver:    receiverTypeName(function),
				receiverVar: receiverVariableName(function),
				declaration: function,
				calls:       map[string]bool{},
				globals:     resolveFileStringValues(file),
			}
			parsed.receiverTypes = resolveLocalReceiverTypes(function)
			parsed.callAliases = resolveCallAliases(function, parsed)
			if parsed.receiver == "" && rootNames[parsed.name] {
				parsed.directRoot = true
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				switch typed := node.(type) {
				case *ast.CallExpr:
					canonical := canonicalCall(typed.Fun, parsed.callAliases, parsed)
					if canonical != "" {
						parsed.calls[canonical] = true
						if !strings.Contains(canonical, ".") && rootNames[canonical] {
							parsed.directRoot = true
						}
					}
					if isStructuredDatabaseSink(canonical) {
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
			governedNames[function.pkg][function.referenceName()] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for index, function := range functions {
			if governed[index] {
				continue
			}
			for call := range function.calls {
				if (!strings.Contains(call, ".") && rootNames[call]) || governedNames[function.pkg][call] {
					governed[index] = true
					if governedNames[function.pkg] == nil {
						governedNames[function.pkg] = map[string]bool{}
					}
					governedNames[function.pkg][function.referenceName()] = true
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
		local[function.pkg][function.referenceName()] = append(local[function.pkg][function.referenceName()], index)
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

func auditLedgerCall(ref string, call *ast.CallExpr, locals, functionReturns map[string]string, stringFunctions map[string]parsedFunction, pkg string, allowedDeletes map[string]string, addIssue func(string)) {
	values := []string{}
	for _, argument := range call.Args {
		if value, ok := resolveStringExpressionDeep(argument, locals, functionReturns, stringFunctions, pkg, 0); ok {
			values = append(values, value)
		}
	}
	auditLedgerText(ref, strings.Join(values, " "), allowedDeletes, addIssue)
}

func auditInvokedLedgerHelper(canonical string, arguments []ast.Expr, callerLocals, functionReturns map[string]string, stringFunctions map[string]parsedFunction, pkg string, allowedDeletes map[string]string, addIssue func(string), depth int) {
	if depth > 16 || canonical == "" || strings.Contains(canonical, ".") {
		return
	}
	callee, found := stringFunctions[pkg+":"+canonical]
	if !found {
		return
	}
	locals := copyStringMap(callee.globals)
	argumentIndex := 0
	if callee.declaration.Type.Params != nil {
		for _, field := range callee.declaration.Type.Params.List {
			for _, name := range field.Names {
				if argumentIndex >= len(arguments) {
					break
				}
				if value, ok := resolveStringExpressionDeep(arguments[argumentIndex], callerLocals, functionReturns, stringFunctions, callee.pkg, depth+1); ok {
					locals[name.Name] = value
				}
				argumentIndex++
			}
		}
	}
	ref := Ref(callee.path, callee.referenceName())
	ast.Inspect(callee.declaration.Body, func(node ast.Node) bool {
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
				if value, ok := resolveStringExpressionDeep(expression, locals, functionReturns, stringFunctions, callee.pkg, depth+1); ok {
					locals[name.Name] = value
				}
			}
		case *ast.DeclStmt:
			resolveLocalStringDeclarationDeep(typed.Decl, locals, functionReturns, stringFunctions, callee.pkg)
		case *ast.CallExpr:
			canonicalCallName := canonicalCall(typed.Fun, callee.callAliases, callee)
			auditLedgerCall(ref, typed, locals, functionReturns, stringFunctions, callee.pkg, allowedDeletes, addIssue)
			auditInvokedLedgerHelper(canonicalCallName, typed.Args, locals, functionReturns, stringFunctions, callee.pkg, allowedDeletes, addIssue, depth+1)
		}
		return true
	})
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
			key := function.key()
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

func packageStringFunctions(functions []parsedFunction) map[string]parsedFunction {
	result := map[string]parsedFunction{}
	for _, function := range functions {
		if function.receiver == "" {
			result[function.pkg+":"+function.name] = function
		}
	}
	return result
}

func resolveLocalStringDeclarationDeep(declaration ast.Decl, locals, functionReturns map[string]string, stringFunctions map[string]parsedFunction, pkg string) {
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
			if value, ok := resolveStringExpressionDeep(expression, locals, functionReturns, stringFunctions, pkg, 0); ok {
				locals[valueSpec.Names[index].Name] = value
			}
		}
	}
}

func resolveStringExpressionDeep(expression ast.Expr, values, functionReturns map[string]string, stringFunctions map[string]parsedFunction, pkg string, depth int) (string, bool) {
	if depth > 16 {
		return "", false
	}
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
		return resolveStringExpressionDeep(typed.X, values, functionReturns, stringFunctions, pkg, depth+1)
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, leftOK := resolveStringExpressionDeep(typed.X, values, functionReturns, stringFunctions, pkg, depth+1)
		right, rightOK := resolveStringExpressionDeep(typed.Y, values, functionReturns, stringFunctions, pkg, depth+1)
		return left + right, leftOK && rightOK
	case *ast.CallExpr:
		name := calledName(typed.Fun)
		if value, ok := functionReturns[pkg+":"+name]; ok {
			return value, true
		}
		function, ok := stringFunctions[pkg+":"+name]
		if !ok {
			return "", false
		}
		return resolveParameterizedStringReturn(function, typed.Args, values, functionReturns, stringFunctions, depth+1)
	default:
		return "", false
	}
}

func resolveParameterizedStringReturn(function parsedFunction, arguments []ast.Expr, callerValues, functionReturns map[string]string, stringFunctions map[string]parsedFunction, depth int) (string, bool) {
	locals := copyStringMap(function.globals)
	argumentIndex := 0
	if function.declaration.Type.Params != nil {
		for _, field := range function.declaration.Type.Params.List {
			for _, name := range field.Names {
				if argumentIndex >= len(arguments) {
					return "", false
				}
				value, ok := resolveStringExpressionDeep(arguments[argumentIndex], callerValues, functionReturns, stringFunctions, function.pkg, depth+1)
				if !ok {
					return "", false
				}
				locals[name.Name] = value
				argumentIndex++
			}
		}
	}
	if argumentIndex != len(arguments) {
		return "", false
	}
	for _, statement := range function.declaration.Body.List {
		switch typed := statement.(type) {
		case *ast.AssignStmt:
			for valueIndex, expression := range typed.Rhs {
				if valueIndex >= len(typed.Lhs) {
					break
				}
				name, ok := typed.Lhs[valueIndex].(*ast.Ident)
				if !ok {
					continue
				}
				if value, ok := resolveStringExpressionDeep(expression, locals, functionReturns, stringFunctions, function.pkg, depth+1); ok {
					locals[name.Name] = value
				}
			}
		case *ast.DeclStmt:
			resolveLocalStringDeclarationDeep(typed.Decl, locals, functionReturns, stringFunctions, function.pkg)
		case *ast.ReturnStmt:
			if len(typed.Results) == 1 {
				return resolveStringExpressionDeep(typed.Results[0], locals, functionReturns, stringFunctions, function.pkg, depth+1)
			}
		}
	}
	return "", false
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

func isStructuredDatabaseSink(canonical string) bool {
	return canonical == "integrationtestdb.NewIsolated" ||
		canonical == "testharness.ApplyThrough" ||
		canonical == "testharness.ApplyLatest" ||
		canonical == "sql.Open"
}

func receiverTypeName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) != 1 {
		return ""
	}
	return typeExpressionName(function.Recv.List[0].Type)
}

func receiverVariableName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) != 1 || len(function.Recv.List[0].Names) != 1 {
		return ""
	}
	return function.Recv.List[0].Names[0].Name
}

func typeExpressionName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return typeExpressionName(typed.X)
	case *ast.IndexExpr:
		return typeExpressionName(typed.X)
	case *ast.IndexListExpr:
		return typeExpressionName(typed.X)
	case *ast.SelectorExpr:
		prefix := typeExpressionName(typed.X)
		if prefix == "" {
			return typed.Sel.Name
		}
		return prefix + "." + typed.Sel.Name
	default:
		return ""
	}
}

func resolveCallAliases(function *ast.FuncDecl, parsed parsedFunction) map[string]string {
	aliases := map[string]string{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for index, expression := range typed.Rhs {
				if index >= len(typed.Lhs) {
					break
				}
				name, ok := typed.Lhs[index].(*ast.Ident)
				if !ok {
					continue
				}
				canonical := canonicalCallableValue(expression, aliases, parsed)
				if canonical != "" {
					aliases[name.Name] = canonical
				}
			}
		case *ast.DeclStmt:
			general, ok := typed.Decl.(*ast.GenDecl)
			if !ok {
				break
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
					canonical := canonicalCallableValue(expression, aliases, parsed)
					if canonical != "" {
						aliases[valueSpec.Names[index].Name] = canonical
					}
				}
			}
		}
		return true
	})
	return aliases
}

func canonicalCallableValue(expression ast.Expr, aliases map[string]string, function parsedFunction) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		if canonical := aliases[typed.Name]; canonical != "" {
			return canonical
		}
		return typed.Name
	case *ast.SelectorExpr:
		return canonicalCall(typed, aliases, function)
	case *ast.ParenExpr:
		return canonicalCallableValue(typed.X, aliases, function)
	default:
		return ""
	}
}

func canonicalCall(expression ast.Expr, aliases map[string]string, function parsedFunction) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		if canonical := aliases[typed.Name]; canonical != "" {
			return canonical
		}
		return typed.Name
	case *ast.SelectorExpr:
		prefix := receiverExpressionType(typed.X, function.receiverTypes)
		if prefix == "" {
			prefix = selectorExpressionName(typed.X)
		}
		if prefix == function.receiverVar && function.receiver != "" {
			prefix = function.receiver
		}
		if prefix == "" {
			return typed.Sel.Name
		}
		return prefix + "." + typed.Sel.Name
	case *ast.ParenExpr:
		return canonicalCall(typed.X, aliases, function)
	default:
		return ""
	}
}

func resolveLocalReceiverTypes(function *ast.FuncDecl) map[string]string {
	types := map[string]string{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for index, expression := range typed.Rhs {
				if index >= len(typed.Lhs) {
					break
				}
				name, ok := typed.Lhs[index].(*ast.Ident)
				if !ok {
					continue
				}
				if receiverType := receiverExpressionType(expression, types); receiverType != "" {
					types[name.Name] = receiverType
				}
			}
		case *ast.DeclStmt:
			general, ok := typed.Decl.(*ast.GenDecl)
			if !ok {
				break
			}
			for _, specification := range general.Specs {
				valueSpec, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}
				declaredType := typeExpressionName(valueSpec.Type)
				for index, name := range valueSpec.Names {
					receiverType := declaredType
					if index < len(valueSpec.Values) {
						if inferred := receiverExpressionType(valueSpec.Values[index], types); inferred != "" {
							receiverType = inferred
						}
					}
					if receiverType != "" {
						types[name.Name] = receiverType
					}
				}
			}
		}
		return true
	})
	return types
}

func receiverExpressionType(expression ast.Expr, receiverTypes map[string]string) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return receiverTypes[typed.Name]
	case *ast.CompositeLit:
		return typeExpressionName(typed.Type)
	case *ast.UnaryExpr:
		if typed.Op == token.AND {
			return receiverExpressionType(typed.X, receiverTypes)
		}
		return ""
	case *ast.ParenExpr:
		return receiverExpressionType(typed.X, receiverTypes)
	default:
		return ""
	}
}

func selectorExpressionName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		prefix := selectorExpressionName(typed.X)
		if prefix == "" {
			return typed.Sel.Name
		}
		return prefix + "." + typed.Sel.Name
	case *ast.ParenExpr:
		return selectorExpressionName(typed.X)
	default:
		return ""
	}
}

func callableLeaf(canonical string) string {
	if index := strings.LastIndex(canonical, "."); index >= 0 {
		return canonical[index+1:]
	}
	return canonical
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
