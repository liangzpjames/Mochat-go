package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestClaimArchiveMessageSourceUsesDuplicateClaimThenLockedReadback(t *testing.T) {
	source, err := os.ReadFile("archive_sync.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "archive_sync.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "claimArchiveMessageSourceTx" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil {
		t.Fatal("claimArchiveMessageSourceTx must have a function body")
	}

	var calls []struct {
		name string
		sql  string
	}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil {
			return true
		}
		if selector.Sel.Name != "ExecContext" && selector.Sel.Name != "QueryRowContext" {
			return true
		}
		value := ""
		if len(call.Args) > 1 {
			if literal, ok := call.Args[1].(*ast.BasicLit); ok {
				value = literal.Value
			}
		}
		calls = append(calls, struct {
			name string
			sql  string
		}{name: selector.Sel.Name, sql: value})
		return true
	})

	insertIndex := -1
	readbackIndex := -1
	for index, call := range calls {
		sql := strings.Join(strings.Fields(strings.ToLower(call.sql)), " ")
		if call.name == "ExecContext" && strings.Contains(sql, "insert into mochat_go_archive_message_sources") {
			if !strings.Contains(sql, "on duplicate key update id = last_insert_id(id)") {
				t.Fatalf("source claim must use an idempotent duplicate-key claim, got %s", call.sql)
			}
			duplicateClause := strings.SplitN(sql, "on duplicate key update", 2)[1]
			if strings.Contains(duplicateClause, "run_id") || strings.Contains(duplicateClause, "source_kind") || strings.Contains(duplicateClause, "source_id") || strings.Contains(duplicateClause, "namespace") {
				t.Fatalf("duplicate source claim must not rewrite source identity or first-claim run_id, got %s", call.sql)
			}
			insertIndex = index
		}
		if call.name == "QueryRowContext" && strings.Contains(sql, "from mochat_go_archive_message_sources") {
			if !strings.Contains(sql, "for update") {
				t.Fatalf("source claim readback must lock the source row, got %s", call.sql)
			}
			readbackIndex = index
		}
	}
	if insertIndex < 0 {
		t.Fatal("source claim INSERT was not found")
	}
	if readbackIndex <= insertIndex {
		t.Fatalf("source claim must read back after duplicate-key claim: insert=%d readback=%d", insertIndex, readbackIndex)
	}
}
