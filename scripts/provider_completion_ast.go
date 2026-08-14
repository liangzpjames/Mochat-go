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
	"sort"
	"strings"

	"jiyi/mochat-go/scripts/productionbuild"
)

type result struct {
	OK       bool             `json:"ok"`
	Errors   []string         `json:"errors"`
	Statuses []statusEvidence `json:"statuses"`
}

type statusEvidence struct {
	File        string `json:"file"`
	PackagePath string `json:"packagePath"`
	PackageName string `json:"package"`
	Receiver    string `json:"receiver"`
	Kind        string `json:"kind"`
	Source      string `json:"source,omitempty"`
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
	goos := flag.String("goos", "linux", "production image GOOS")
	goarch := flag.String("goarch", "amd64", "production image GOARCH")
	flag.Parse()
	errors := checkArchiveStatuses(*root, *goos, *goarch)
	statuses, statusErrors := collectStatusEvidence(*root, *goos, *goarch)
	errors = append(errors, statusErrors...)
	_ = json.NewEncoder(os.Stdout).Encode(result{OK: len(errors) == 0, Errors: uniqueErrors(errors), Statuses: statuses})
	if len(errors) > 0 {
		os.Exit(1)
	}
}

func checkArchiveStatuses(root, goos, goarch string) []string {
	target, err := productionbuild.ForTarget(goos, goarch)
	if err != nil {
		return []string{err.Error()}
	}
	directory := filepath.Join(root, "internal", "modules", "providers", "archive")
	files, walkErr := scanProductionFiles(directory, target, true)
	if walkErr != nil {
		return []string{walkErr.Error()}
	}
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

func scanProductionFiles(directory string, target build.Context, requireDirectory bool) ([]string, error) {
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) && !requireDirectory {
			return []string{}, nil
		}
		return nil, fmt.Errorf("production Go directory %s is unavailable: %w", filepath.ToSlash(directory), err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("production Go path %s is not a directory", filepath.ToSlash(directory))
	}
	files := make([]string, 0)
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != directory && (entry.Name() == "fixtures" || entry.Name() == "testdata" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		matched, matchErr := target.MatchFile(filepath.Dir(path), entry.Name())
		if matchErr != nil {
			return matchErr
		}
		if matched {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan production Go files: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func collectStatusEvidence(root, goos, goarch string) ([]statusEvidence, []string) {
	target, err := productionbuild.ForTarget(goos, goarch)
	if err != nil {
		return nil, []string{err.Error()}
	}
	directories := []string{
		filepath.Join(root, "internal", "modules", "providers"),
		filepath.Join(root, "internal", "dashboard"),
	}
	files := make([]string, 0)
	errors := make([]string, 0)
	for _, directory := range directories {
		found, scanErr := scanProductionFiles(directory, target, false)
		if scanErr != nil {
			errors = append(errors, scanErr.Error())
			continue
		}
		files = append(files, found...)
	}
	sort.Strings(files)

	type receiverMethods struct {
		packagePath string
		packageName string
		receiver    string
		file        string
		kind        *ast.FuncDecl
		status      *ast.FuncDecl
	}
	methods := make(map[string]*receiverMethods)
	for _, file := range files {
		if filepath.ToSlash(filepath.Join(root, "internal", "modules", "providers", "catalog", "catalog.go")) == filepath.ToSlash(file) {
			continue
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			errors = append(errors, fmt.Sprintf("%s: Go AST parse failed: %v", filepath.ToSlash(file), parseErr))
			continue
		}
		relativeDirectory, relErr := filepath.Rel(root, filepath.Dir(file))
		if relErr != nil {
			errors = append(errors, fmt.Sprintf("%s: determine package path failed: %v", filepath.ToSlash(file), relErr))
			continue
		}
		packagePath := filepath.ToSlash(filepath.Join(relativeDirectory, parsed.Name.Name))
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || len(function.Recv.List) == 0 {
				continue
			}
			receiver := receiverTypeName(function.Recv.List[0].Type)
			if receiver == "" {
				continue
			}
			key := packagePath + "\x00" + receiver
			entry := methods[key]
			if entry == nil {
				entry = &receiverMethods{packagePath: packagePath, packageName: parsed.Name.Name, receiver: receiver, file: file}
				methods[key] = entry
			}
			switch function.Name.Name {
			case "Kind":
				if entry.kind != nil {
					errors = append(errors, fmt.Sprintf("%s: duplicate Status evidence Kind method for package %s receiver %s", filepath.ToSlash(file), entry.packagePath, entry.receiver))
				} else {
					entry.kind = function
				}
			case "Status":
				if entry.status != nil {
					errors = append(errors, fmt.Sprintf("%s: duplicate Status evidence Status method for package %s receiver %s", filepath.ToSlash(file), entry.packagePath, entry.receiver))
				} else {
					entry.status = function
				}
			}
		}
	}

	evidence := make([]statusEvidence, 0)
	for _, entry := range methods {
		if entry.status == nil {
			continue
		}
		if !returnsProvidersType(entry.status, "Status") {
			continue
		}
		if entry.kind != nil && !returnsProvidersType(entry.kind, "Source") {
			continue
		}
		kind, source, ok := directStatusEvidence(entry.status, entry.kind)
		if !ok {
			continue
		}
		relativeFile, relErr := filepath.Rel(root, entry.file)
		if relErr != nil {
			errors = append(errors, fmt.Sprintf("%s: determine evidence file failed: %v", filepath.ToSlash(entry.file), relErr))
			continue
		}
		evidence = append(evidence, statusEvidence{
			File:        filepath.ToSlash(relativeFile),
			PackagePath: entry.packagePath,
			PackageName: entry.packageName,
			Receiver:    entry.receiver,
			Kind:        kind,
			Source:      source,
		})
	}
	sort.Slice(evidence, func(i, j int) bool {
		if evidence[i].Kind != evidence[j].Kind {
			return evidence[i].Kind < evidence[j].Kind
		}
		return evidence[i].PackagePath+"/"+evidence[i].Receiver < evidence[j].PackagePath+"/"+evidence[j].Receiver
	})
	return evidence, uniqueErrors(errors)
}

func directStatusEvidence(status *ast.FuncDecl, kindMethod *ast.FuncDecl) (string, string, bool) {
	if status == nil || status.Body == nil || !returnsProvidersType(status, "Status") {
		return "", "", false
	}
	bindings := make(map[string]*ast.CompositeLit)
	var kinds []string
	source := ""
	valid := true
	ast.Inspect(status.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if len(value.Lhs) == 1 && len(value.Rhs) == 1 {
				if identifier, ok := value.Lhs[0].(*ast.Ident); ok {
					if literal, ok := value.Rhs[0].(*ast.CompositeLit); ok && isStatusType(literal.Type) {
						bindings[identifier.Name] = literal
					}
				}
			}
		case *ast.ValueSpec:
			spec := value
			for index, expression := range spec.Values {
				if index >= len(spec.Names) {
					continue
				}
				if literal, ok := expression.(*ast.CompositeLit); ok && isStatusType(literal.Type) {
					bindings[spec.Names[index].Name] = literal
				}
			}
		case *ast.ReturnStmt:
			if len(value.Results) != 1 {
				valid = false
				return true
			}
			literal, ok := value.Results[0].(*ast.CompositeLit)
			if !ok {
				identifier, identifierOK := value.Results[0].(*ast.Ident)
				literal, ok = bindings[identifierName(identifier)]
				if !identifierOK || !ok {
					valid = false
					return true
				}
			}
			if !isStatusType(literal.Type) {
				valid = false
				return true
			}
			fields := statusFields(literal)
			kind, kindOK := stringLiteral(fields["Kind"])
			if !kindOK || kind == "" {
				valid = false
				return true
			}
			kinds = append(kinds, kind)
			if sourceExpression, sourcePresent := fields["Source"]; sourcePresent {
				value, sourceOK := sourceName(sourceExpression)
				if !sourceOK {
					valid = false
					return true
				}
				if source != "" && source != value {
					valid = false
				}
				source = value
			}
		}
		return true
	})
	if kindMethod != nil {
		kindSource := returnedSource(kindMethod.Body)
		if source == "" {
			switch kindSource {
			case "external":
				source = "SourceExternal"
			case "simulated":
				source = "SourceSimulated"
			}
		}
	}
	if !valid || len(kinds) == 0 {
		return "", "", false
	}
	for _, kind := range kinds[1:] {
		if kind != kinds[0] {
			return "", "", false
		}
	}
	return kinds[0], source, true
}

func returnsProvidersType(function *ast.FuncDecl, typeName string) bool {
	if function == nil || function.Type == nil || function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return false
	}
	selector, ok := function.Type.Results.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "providers" && selector.Sel.Name == typeName
}

func identifierName(identifier *ast.Ident) string {
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

func sourceName(expression ast.Expr) (string, bool) {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok || packageName.Name != "providers" {
		return "", false
	}
	switch selector.Sel.Name {
	case "SourceExternal", "SourceSimulated", "SourceLocal", "SourceCodeOnly":
		return selector.Sel.Name, true
	default:
		return "", false
	}
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
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		returnNode, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		if len(returnNode.Results) != 1 {
			valid = false
			return true
		}
		if selector, ok := returnNode.Results[0].(*ast.SelectorExpr); ok {
			packageName, packageOK := selector.X.(*ast.Ident)
			if !packageOK || packageName.Name != "providers" {
				valid = false
				return true
			}
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
	if !ok {
		return false
	}
	packageName, packageOK := selector.X.(*ast.Ident)
	return packageOK && packageName.Name == "providers" && selector.Sel.Name == "Status"
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
	packageName, packageOK := selector.X.(*ast.Ident)
	if !packageOK || packageName.Name != "providers" {
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
