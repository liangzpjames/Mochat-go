package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type result struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors"`
}

type method struct {
	directory   string
	packageName string
	receiver    string
	function    *ast.FuncDecl
}

type kindContract struct {
	present bool
	source  string
}

func main() {
	root := flag.String("root", ".", "repository root to inspect")
	flag.Parse()
	errors := checkArchiveStatuses(*root)
	_ = json.NewEncoder(os.Stdout).Encode(result{OK: len(errors) == 0, Errors: errors})
	if len(errors) > 0 {
		os.Exit(1)
	}
}

func checkArchiveStatuses(root string) []string {
	directory := filepath.Join(root, "internal", "modules", "providers", "archive")
	files := make([]string, 0)
	_ = filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != directory && (entry.Name() == "fixtures" || entry.Name() == "testdata" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	methods := make([]method, 0)
	methodKeys := make(map[string]bool)
	kinds := make(map[string]kindContract)
	errors := make([]string, 0)
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			return []string{fmt.Sprintf("%s: Go AST parse failed: %v", filepath.ToSlash(file), err)}
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || len(function.Recv.List) == 0 {
				continue
			}
			receiver := receiverTypeName(function.Recv.List[0].Type)
			if receiver == "" {
				if function.Name.Name == "Kind" || function.Name.Name == "Status" {
					errors = append(errors, fmt.Sprintf("%s: archive %s has an unsupported receiver type", filepath.ToSlash(file), function.Name.Name))
				}
				continue
			}
			packageName := parsed.Name.Name
			key := filepath.ToSlash(filepath.Join(filepath.Dir(file), packageName, receiver))
			if function.Name.Name == "Kind" {
				if _, exists := kinds[key]; exists {
					errors = append(errors, fmt.Sprintf("%s: duplicate archive Kind method for package %s receiver %s", filepath.ToSlash(file), packageName, receiver))
					continue
				}
				contract := kinds[key]
				contract.present = true
				contract.source = returnedSource(function.Body)
				kinds[key] = contract
			}
			if function.Name.Name == "Status" {
				if methodKeys[key] {
					errors = append(errors, fmt.Sprintf("%s: duplicate archive Status method for package %s receiver %s", filepath.ToSlash(file), packageName, receiver))
					continue
				}
				methodKeys[key] = true
				methods = append(methods, method{directory: filepath.ToSlash(filepath.Dir(file)), packageName: packageName, receiver: receiver, function: function})
			}
		}
	}
	for _, status := range methods {
		contract, present := kinds[filepath.ToSlash(filepath.Join(status.directory, status.packageName, status.receiver))]
		if !present {
			errors = append(errors, fmt.Sprintf("%s: external archive Status receiver %s has no matching Kind method", status.directory, status.receiver))
			continue
		}
		if contract.source == "simulated" {
			continue
		}
		if contract.source != "external" {
			errors = append(errors, fmt.Sprintf("%s: archive receiver %s Kind must directly return providers.SourceExternal", status.directory, status.receiver))
			continue
		}
		if status.function.Body == nil {
			errors = append(errors, "archive/wecom: external Status has no function body")
			continue
		}
		foundUnimplemented := false
		ast.Inspect(status.function.Body, func(node ast.Node) bool {
			returnNode, ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(returnNode.Results) != 1 {
				errors = append(errors, "archive/wecom: external Status must return one direct providers.Status literal")
				return true
			}
			literal, ok := returnNode.Results[0].(*ast.CompositeLit)
			if !ok || !isStatusType(literal.Type) {
				errors = append(errors, "archive/wecom: external Status return must be a direct providers.Status literal")
				return true
			}
			fields := statusFields(literal)
			state, ok := fields["State"]
			if !ok || !isStateLimitedOrUnavailable(state) {
				errors = append(errors, "archive/wecom: external Status must return StateLimited or StateUnavailable directly")
			}
			code, ok := fields["Code"]
			if !ok {
				errors = append(errors, "archive/wecom: external Status must return a stable Code directly")
			} else if value, ok := stringLiteral(code); !ok || !strings.HasPrefix(value, "archive.") {
				errors = append(errors, "archive/wecom: external Status Code must be a stable archive.* string literal")
			} else if value == "archive.getchatdata_unimplemented" {
				foundUnimplemented = true
			}
			return true
		})
		if !foundUnimplemented {
			errors = append(errors, "archive/wecom: external Status must expose archive.getchatdata_unimplemented for the unimplemented source")
		}
	}
	if len(methods) == 0 && len(kinds) > 0 {
		errors = append(errors, "archive: external source has no Status method")
	}
	return uniqueErrors(errors)
}

func receiverTypeName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverTypeName(value.X)
	case *ast.IndexExpr:
		return receiverTypeName(value.X)
	case *ast.IndexListExpr:
		return receiverTypeName(value.X)
	default:
		return ""
	}
}

func returnedSource(body *ast.BlockStmt) string {
	if body == nil {
		return "unknown"
	}
	found := ""
	valid := true
	ast.Inspect(body, func(node ast.Node) bool {
		returnNode, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		if len(returnNode.Results) != 1 {
			valid = false
			return true
		}
		if selector, ok := returnNode.Results[0].(*ast.SelectorExpr); ok {
			switch selector.Sel.Name {
			case "SourceExternal":
				if found != "" {
					valid = false
				}
				found = "external"
			case "SourceSimulated":
				if found != "" {
					valid = false
				}
				found = "simulated"
			default:
				valid = false
			}
		} else {
			valid = false
		}
		return true
	})
	if !valid || found == "" {
		return "unknown"
	}
	return found
}

func isStatusType(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Status"
}

func statusFields(literal *ast.CompositeLit) map[string]ast.Expr {
	fields := make(map[string]ast.Expr)
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if ok {
			fields[key.Name] = field.Value
		}
	}
	return fields
}

func isStateLimitedOrUnavailable(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return selector.Sel.Name == "StateLimited" || selector.Sel.Name == "StateUnavailable"
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

func uniqueErrors(errors []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(errors))
	for _, err := range errors {
		if !seen[err] {
			seen[err] = true
			result = append(result, err)
		}
	}
	return result
}
