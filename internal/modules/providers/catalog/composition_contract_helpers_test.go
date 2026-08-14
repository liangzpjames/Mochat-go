package catalog

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	pathpkg "path"
	"strconv"
)

func validateProviderFactorySource(source, functionName string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "catalog.go", source, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse source: %w", err)
	}
	var factory *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == functionName {
			factory = candidate
			break
		}
	}
	if factory == nil || factory.Body == nil {
		return fmt.Errorf("%s function is missing", functionName)
	}

	registryNames := map[string]bool{}
	registerCalls := make([]*ast.CallExpr, 0)
	returnedRegistryNames := map[string]bool{}
	walkReachableStatements(factory.Body.List, func(node ast.Node) {
		switch value := node.(type) {
		case *ast.AssignStmt:
			for index, right := range value.Rhs {
				call, ok := right.(*ast.CallExpr)
				if !ok || !isProviderRegistryConstructor(call) || index >= len(value.Lhs) {
					continue
				}
				if name, ok := value.Lhs[index].(*ast.Ident); ok {
					registryNames[name.Name] = true
				}
			}
		case *ast.CallExpr:
			if selector, ok := value.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Register" {
				registerCalls = append(registerCalls, value)
			}
		case *ast.ReturnStmt:
			for _, expression := range value.Results {
				if identifier, ok := expression.(*ast.Ident); ok {
					returnedRegistryNames[identifier.Name] = true
				}
			}
		}
	})
	if len(registryNames) != 1 {
		return fmt.Errorf("factory must create exactly one reachable providers.NewRegistry result")
	}
	var registryName string
	for name := range registryNames {
		registryName = name
	}
	if len(registerCalls) == 0 {
		return fmt.Errorf("factory has no reachable Register call")
	}
	for _, call := range registerCalls {
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return fmt.Errorf("Register receiver is not an identifier")
		}
		identifier, ok := selector.X.(*ast.Ident)
		if !ok || identifier.Name != registryName {
			return fmt.Errorf("Register does not use the factory registry %q", registryName)
		}
	}
	if !returnedRegistryNames[registryName] {
		return fmt.Errorf("factory does not return its reachable registry %q", registryName)
	}
	return nil
}

func validateProductionProviderWiring(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse source: %w", err)
	}
	aliases, err := requiredProductionImportAliases(file)
	if err != nil {
		return err
	}
	mainFunction := findFunction(file, "main")
	if mainFunction == nil || mainFunction.Body == nil {
		return fmt.Errorf("production composition root main function is missing")
	}

	assignments := make([]*ast.AssignStmt, 0)
	calls := make([]*ast.CallExpr, 0)
	walkReachableStatements(mainFunction.Body.List, func(node ast.Node) {
		switch value := node.(type) {
		case *ast.AssignStmt:
			assignments = append(assignments, value)
		case *ast.CallExpr:
			calls = append(calls, value)
		}
	})

	registryVar := ""
	for _, assignment := range assignments {
		for index, right := range assignment.Rhs {
			call, ok := right.(*ast.CallExpr)
			if !ok || !isQualifiedCall(call, aliases["jiyi/mochat-go/internal/modules/providers/catalog"], "NewRegistry") || index >= len(assignment.Lhs) {
				continue
			}
			identifier, ok := assignment.Lhs[index].(*ast.Ident)
			if !ok {
				return fmt.Errorf("catalog factory result is not assigned to a local identifier")
			}
			if registryVar != "" && registryVar != identifier.Name {
				return fmt.Errorf("production main calls catalog factory into multiple identifiers")
			}
			registryVar = identifier.Name
		}
	}
	if registryVar == "" {
		return fmt.Errorf("reachable production main does not call the real catalog factory")
	}

	sourceVars := map[string]bool{}
	hasSourceFlow := false
	for _, call := range calls {
		if isQualifiedCall(call, aliases["jiyi/mochat-go/internal/companyprofile"], "NewProviderStatusSource") && callHasDirectIdentifier(call, registryVar) {
			hasSourceFlow = true
		}
	}
	for _, assignment := range assignments {
		for index, right := range assignment.Rhs {
			call, ok := right.(*ast.CallExpr)
			if !ok || !isQualifiedCall(call, aliases["jiyi/mochat-go/internal/companyprofile"], "NewProviderStatusSource") || !callHasDirectIdentifier(call, registryVar) || index >= len(assignment.Lhs) {
				continue
			}
			if identifier, ok := assignment.Lhs[index].(*ast.Ident); ok {
				sourceVars[identifier.Name] = true
			}
		}
	}
	if !hasSourceFlow {
		return fmt.Errorf("catalog factory result does not reach companyprofile.NewProviderStatusSource")
	}

	serviceVars := map[string]bool{}
	hasServiceFlow := false
	for _, call := range calls {
		if !isQualifiedCall(call, aliases["jiyi/mochat-go/internal/providerstatus"], "NewService") || !expressionHasServiceSource(call, aliases["jiyi/mochat-go/internal/companyprofile"], registryVar, sourceVars) {
			continue
		}
		hasServiceFlow = true
		for _, assignment := range assignments {
			if assignmentContainsCall(assignment, call) {
				for _, left := range assignment.Lhs {
					if identifier, ok := left.(*ast.Ident); ok {
						serviceVars[identifier.Name] = true
					}
				}
			}
		}
	}
	if !hasServiceFlow {
		return fmt.Errorf("companyprofile.NewProviderStatusSource result does not reach providerstatus.NewService")
	}

	for _, call := range calls {
		if !isQualifiedCall(call, aliases["jiyi/mochat-go/internal/server"], "WithProviderStatusHandler") {
			continue
		}
		for _, argument := range call.Args {
			if expressionHasHTTPHandlerWithService(argument, aliases["jiyi/mochat-go/internal/providerstatus"], aliases["jiyi/mochat-go/internal/companyprofile"], registryVar, serviceVars) {
				return nil
			}
		}
	}
	return fmt.Errorf("provider status service does not reach WithProviderStatusHandler")
}

func requiredProductionImportAliases(file *ast.File) (map[string]string, error) {
	aliases := make(map[string]string)
	for _, importSpec := range file.Imports {
		path, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid production import path: %w", err)
		}
		alias := pathpkg.Base(path)
		if importSpec.Name != nil {
			if importSpec.Name.Name == "_" {
				continue
			}
			alias = importSpec.Name.Name
		}
		aliases[path] = alias
	}
	for _, required := range []string{
		"jiyi/mochat-go/internal/modules/providers/catalog",
		"jiyi/mochat-go/internal/companyprofile",
		"jiyi/mochat-go/internal/providerstatus",
		"jiyi/mochat-go/internal/server",
	} {
		if aliases[required] == "" {
			return nil, fmt.Errorf("production main is missing real import %q", required)
		}
	}
	return aliases, nil
}

func findFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil && function.Name.Name == name {
			return function
		}
	}
	return nil
}

func isProviderRegistryConstructor(call *ast.CallExpr) bool {
	return isQualifiedCall(call, "providers", "NewRegistry")
}

func isQualifiedCall(call *ast.CallExpr, alias, name string) bool {
	if call == nil || alias == "" {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == alias
}

func callHasDirectIdentifier(call *ast.CallExpr, name string) bool {
	for _, argument := range call.Args {
		if identifier, ok := argument.(*ast.Ident); ok && identifier.Name == name {
			return true
		}
	}
	return false
}

func expressionHasServiceSource(expression ast.Expr, companyAlias, registryVar string, sourceVars map[string]bool) bool {
	if identifier, ok := expression.(*ast.Ident); ok && sourceVars[identifier.Name] {
		return true
	}
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && isQualifiedCall(call, companyAlias, "NewProviderStatusSource") && callHasDirectIdentifier(call, registryVar) {
			found = true
			return false
		}
		return true
	})
	return found
}

func expressionHasHTTPHandlerWithService(expression ast.Expr, providerStatusAlias, companyAlias, registryVar string, serviceVars map[string]bool) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isQualifiedCall(call, providerStatusAlias, "NewHTTPHandler") {
			return true
		}
		for _, argument := range call.Args {
			if expressionHasServiceValue(argument, providerStatusAlias, companyAlias, registryVar, serviceVars) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func expressionHasServiceValue(expression ast.Expr, providerStatusAlias, companyAlias, registryVar string, serviceVars map[string]bool) bool {
	if identifier, ok := expression.(*ast.Ident); ok && serviceVars[identifier.Name] {
		return true
	}
	var found bool
	ast.Inspect(expression, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isQualifiedCall(call, providerStatusAlias, "NewService") {
			return true
		}
		for _, argument := range call.Args {
			if expressionHasServiceSource(argument, companyAlias, registryVar, nil) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func assignmentContainsCall(assignment *ast.AssignStmt, target *ast.CallExpr) bool {
	for _, right := range assignment.Rhs {
		found := false
		ast.Inspect(right, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && call == target {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

func walkReachableStatements(statements []ast.Stmt, visit func(ast.Node)) bool {
	for _, statement := range statements {
		if !walkReachableStatement(statement, visit) {
			return false
		}
	}
	return true
}

func walkReachableStatement(statement ast.Stmt, visit func(ast.Node)) bool {
	switch value := statement.(type) {
	case *ast.BlockStmt:
		return walkReachableStatements(value.List, visit)
	case *ast.ReturnStmt:
		inspectReachableStatement(value, visit)
		return false
	case *ast.IfStmt:
		if value.Init != nil {
			inspectReachableStatement(value.Init, visit)
		}
		if condition, known := staticBoolean(value.Cond); known {
			if condition {
				return walkReachableStatement(value.Body, visit)
			}
			if value.Else == nil {
				return true
			}
			return walkReachableStatement(value.Else, visit)
		}
		thenContinues := walkReachableStatement(value.Body, visit)
		elseContinues := true
		if value.Else != nil {
			elseContinues = walkReachableStatement(value.Else, visit)
		}
		return thenContinues || elseContinues
	case *ast.ForStmt:
		if value.Init != nil {
			inspectReachableStatement(value.Init, visit)
		}
		if value.Post != nil {
			inspectReachableStatement(value.Post, visit)
		}
		walkReachableStatement(value.Body, visit)
		return true
	case *ast.RangeStmt:
		inspectReachableExpression(value.X, visit)
		walkReachableStatement(value.Body, visit)
		return true
	case *ast.SwitchStmt:
		if value.Init != nil {
			inspectReachableStatement(value.Init, visit)
		}
		if value.Tag != nil {
			inspectReachableExpression(value.Tag, visit)
		}
		for _, clause := range value.Body.List {
			if caseClause, ok := clause.(*ast.CaseClause); ok {
				walkReachableStatements(caseClause.Body, visit)
			}
		}
		return true
	default:
		inspectReachableStatement(statement, visit)
		return true
	}
}

func inspectReachableStatement(statement ast.Stmt, visit func(ast.Node)) {
	ast.Inspect(statement, func(node ast.Node) bool {
		if _, functionLiteral := node.(*ast.FuncLit); functionLiteral {
			return false
		}
		visit(node)
		return true
	})
}

func inspectReachableExpression(expression ast.Expr, visit func(ast.Node)) {
	if expression == nil {
		return
	}
	ast.Inspect(expression, func(node ast.Node) bool {
		visit(node)
		return true
	})
}

func staticBoolean(expression ast.Expr) (bool, bool) {
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return false, false
	}
	switch identifier.Name {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}
