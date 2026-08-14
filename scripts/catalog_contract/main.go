package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type registration struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
}

type result struct {
	OK            bool           `json:"ok"`
	Errors        []string       `json:"errors"`
	Registrations []registration `json:"registrations"`
}

type composition struct {
	registryName  string
	registrations map[string][]registration
	ranges        map[string]string
	seen          []registration
	registerPos   []token.Pos
	returnPos     []token.Pos
	errors        []string
}

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	result := check(*root)
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if !result.OK {
		os.Exit(1)
	}
}

func check(root string) result {
	path := filepath.Join(root, "internal", "modules", "providers", "catalog", "catalog.go")
	matched, err := build.Default.MatchFile(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return result{Errors: []string{fmt.Sprintf("catalog build-set check failed: %v", err)}}
	}
	if !matched {
		return result{Errors: []string{"catalog.go is not in the production build set"}}
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return result{Errors: []string{fmt.Sprintf("catalog AST parse failed: %v", err)}}
	}
	var factory *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "NewRegistry" {
			if factory != nil {
				return result{Errors: []string{"catalog NewRegistry is duplicated"}}
			}
			factory = function
		}
	}
	if factory == nil || factory.Body == nil {
		return result{Errors: []string{"catalog NewRegistry is missing"}}
	}
	composition := composition{
		registrations: make(map[string][]registration),
		ranges:        make(map[string]string),
	}
	collectDeclarations(factory.Body, &composition)
	findRegistry(factory.Body, &composition)
	if composition.registryName == "" {
		composition.errors = append(composition.errors, "catalog NewRegistry does not construct a named providers.Registry")
	}
	walkStatements(factory.Body.List, &composition, false, "")
	if len(composition.registerPos) == 0 {
		composition.errors = append(composition.errors, "catalog NewRegistry must contain an unconditional reachable registry.Register path")
	}
	for _, position := range composition.returnPos {
		for _, registerPosition := range composition.registerPos {
			if position < registerPosition {
				composition.errors = append(composition.errors, "catalog NewRegistry returns its registry before the provider registration path")
				break
			}
		}
	}
	if len(composition.seen) == 0 {
		composition.errors = append(composition.errors, "catalog NewRegistry has no statically bound Provider registrations")
	}
	return result{OK: len(composition.errors) == 0, Errors: unique(composition.errors), Registrations: uniqueRegistrations(composition.seen)}
}

func findRegistry(body *ast.BlockStmt, composition *composition) {
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "NewRegistry" {
			if identifier, ok := assignment.Lhs[0].(*ast.Ident); ok {
				composition.registryName = identifier.Name
			}
		}
		return true
	})
}

func collectDeclarations(body *ast.BlockStmt, composition *composition) {
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		identifier, ok := assignment.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		literal, ok := assignment.Rhs[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		values := registrationLiterals(literal)
		if len(values) > 0 {
			composition.registrations[identifier.Name] = values
		}
		return true
	})
}

func walkStatements(statements []ast.Stmt, composition *composition, conditional bool, rangeValue string) {
	for _, statement := range statements {
		switch statement := statement.(type) {
		case *ast.BlockStmt:
			walkStatements(statement.List, composition, conditional, rangeValue)
		case *ast.IfStmt:
			if statement.Init != nil {
				walkStatements([]ast.Stmt{statement.Init}, composition, conditional, rangeValue)
			}
			walkStatements(statement.Body.List, composition, true, rangeValue)
			if elseBlock, ok := statement.Else.(*ast.BlockStmt); ok {
				walkStatements(elseBlock.List, composition, true, rangeValue)
			}
		case *ast.ForStmt:
			if statement.Init != nil {
				walkStatements([]ast.Stmt{statement.Init}, composition, conditional, rangeValue)
			}
			walkStatements(statement.Body.List, composition, true, rangeValue)
		case *ast.RangeStmt:
			value := ""
			if identifier, ok := statement.Value.(*ast.Ident); ok {
				value = identifier.Name
			}
			collection, ok := statement.X.(*ast.Ident)
			if !ok || len(composition.registrations[collection.Name]) == 0 {
				walkStatements(statement.Body.List, composition, true, value)
				continue
			}
			composition.ranges[value] = collection.Name
			walkStatements(statement.Body.List, composition, conditional, value)
		case *ast.ExprStmt:
			inspectRegisterCall(statement.X, composition, conditional, rangeValue)
		case *ast.AssignStmt:
			for _, expression := range statement.Rhs {
				inspectRegisterCall(expression, composition, conditional, rangeValue)
			}
		case *ast.ReturnStmt:
			if len(statement.Results) > 0 {
				if identifier, ok := statement.Results[0].(*ast.Ident); ok {
					switch identifier.Name {
					case composition.registryName:
						composition.returnPos = append(composition.returnPos, statement.Pos())
					case "nil":
					default:
						composition.errors = append(composition.errors, "catalog NewRegistry returns another registry value")
					}
				}
			}
		}
	}
}

func inspectRegisterCall(expression ast.Expr, composition *composition, conditional bool, rangeValue string) {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Register" {
		return
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok || receiver.Name != composition.registryName {
		return
	}
	if conditional {
		return
	}
	composition.registerPos = append(composition.registerPos, call.Pos())
	if len(call.Args) != 1 {
		composition.errors = append(composition.errors, "catalog Register call must have one statically bound registration")
		return
	}
	switch argument := call.Args[0].(type) {
	case *ast.Ident:
		if rangeCollection, ok := composition.ranges[argument.Name]; ok {
			composition.seen = append(composition.seen, composition.registrations[rangeCollection]...)
			return
		}
		composition.seen = append(composition.seen, composition.registrations[argument.Name]...)
	case *ast.CompositeLit:
		composition.seen = append(composition.seen, registrationLiterals(argument)...)
	default:
		composition.errors = append(composition.errors, "catalog Register call must use a direct registration or static registration range")
	}
}

func registrationLiterals(literal *ast.CompositeLit) []registration {
	if isRegistrationType(literal.Type) {
		value, ok := registrationFromLiteral(literal)
		if !ok {
			return nil
		}
		return []registration{value}
	}
	array, ok := literal.Type.(*ast.ArrayType)
	if !ok || !isRegistrationType(array.Elt) {
		return nil
	}
	result := make([]registration, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		composite, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		if value, ok := registrationFromLiteral(composite); ok {
			result = append(result, value)
		}
	}
	return result
}

func registrationFromLiteral(literal *ast.CompositeLit) (registration, bool) {
	value := registration{}
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Kind":
			value.Kind, _ = stringLiteral(field.Value)
		case "Source":
			value.Source = sourceName(field.Value)
		}
	}
	return value, value.Kind != "" && value.Source != ""
}

func isRegistrationType(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Registration"
}

func sourceName(expression ast.Expr) string {
	if call, ok := expression.(*ast.CallExpr); ok {
		if selector, ok := call.Fun.(*ast.Ident); ok && selector.Name == "aiSource" {
			return "SourceExternal"
		}
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return selector.Sel.Name
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind.String() != "STRING" {
		return "", false
	}
	value := strings.TrimSpace(literal.Value)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}
	return strings.Trim(value, "\""), true
}

func unique(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func uniqueRegistrations(values []registration) []registration {
	seen := make(map[string]bool)
	result := make([]registration, 0, len(values))
	for _, value := range values {
		key := value.Kind + "\x00" + value.Source
		if !seen[key] {
			seen[key] = true
			result = append(result, value)
		}
	}
	return result
}
