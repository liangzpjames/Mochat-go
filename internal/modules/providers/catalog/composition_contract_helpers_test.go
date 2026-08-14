package catalog

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
	returnRegistry := false
	ast.Inspect(factory.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.AssignStmt:
			for index, right := range value.Rhs {
				call, ok := right.(*ast.CallExpr)
				if !ok || !isProviderSelectorCall(call, "NewRegistry") || index >= len(value.Lhs) {
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
				if identifier, ok := expression.(*ast.Ident); ok && registryNames[identifier.Name] {
					returnRegistry = true
				}
			}
		}
		return true
	})
	if len(registryNames) != 1 {
		return fmt.Errorf("factory must create exactly one providers.NewRegistry result")
	}
	var registryName string
	for name := range registryNames {
		registryName = name
	}
	if len(registerCalls) == 0 {
		return fmt.Errorf("factory has no Register call")
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
	if !returnRegistry {
		return fmt.Errorf("factory does not return its registry %q", registryName)
	}
	for index, statement := range factory.Body.List {
		if _, ok := statement.(*ast.ReturnStmt); !ok {
			continue
		}
		for _, later := range factory.Body.List[index+1:] {
			if containsRegisterCall(later) {
				return fmt.Errorf("Register call is after an unconditional return")
			}
		}
	}
	return nil
}

func validateProductionProviderWiring(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse source: %w", err)
	}
	called := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		called = called || isProviderSelectorCall(call, "NewRegistry")
		return true
	})
	if !called {
		return fmt.Errorf("production composition root does not call providercatalog.NewRegistry")
	}
	return nil
}

func isProviderSelectorCall(call *ast.CallExpr, selectorName string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name != ""
}

func containsRegisterCall(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "Register" {
			found = true
			return false
		}
		return true
	})
	return found
}
