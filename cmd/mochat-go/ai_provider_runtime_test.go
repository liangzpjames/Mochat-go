package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/modules/providers/catalog"
)

func TestProductionAICompositionUsesOneTenantResolverForWorkspaceAndDaily(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(filename), "ai_debt_clearance.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var resolverAssigned, moduleReceivesResolver, dailyReceivesResolver, dailyConfigReceivesResolver bool
	var enableInsightGate, dailyEnabledGate bool
	var forbidden []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.AssignStmt:
			for index, lhs := range value.Lhs {
				identifier, ok := lhs.(*ast.Ident)
				if !ok || identifier.Name != "aiResolver" || index >= len(value.Rhs) {
					continue
				}
				call, ok := value.Rhs[index].(*ast.CallExpr)
				selector, selected := callFunSelector(call)
				resolverAssigned = ok && selected && selector == "TenantAIProviderResolver"
			}
		case *ast.CompositeLit:
			for _, element := range value.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				key, keyOK := field.Key.(*ast.Ident)
				argument, argOK := field.Value.(*ast.Ident)
				if ok && keyOK && argOK && key.Name == "AIProviderResolver" && argument.Name == "aiResolver" {
					moduleReceivesResolver = true
				}
				if ok && keyOK && argOK && key.Name == "Resolver" && argument.Name == "resolver" {
					dailyConfigReceivesResolver = true
				}
			}
		case *ast.CallExpr:
			identifier, ok := value.Fun.(*ast.Ident)
			if ok && identifier.Name == "startAIInsightDailyAnalysis" && len(value.Args) == 4 {
				argument, argOK := value.Args[2].(*ast.Ident)
				dailyReceivesResolver = argOK && argument.Name == "aiResolver"
			}
		case *ast.SelectorExpr:
			if value.Sel.Name == "EnableAIInsight" {
				enableInsightGate = true
			}
			if value.Sel.Name == "AIInsightDailyAnalysisEnabled" {
				dailyEnabledGate = true
			}
		case *ast.Ident:
			if value.Name == "buildAIProvider" || value.Name == "StaticAIProviderResolver" {
				forbidden = append(forbidden, value.Name)
			}
		case *ast.BasicLit:
			if value.Kind == token.STRING {
				literal, _ := strconv.Unquote(value.Value)
				if strings.Contains(literal, "MOCHAT_GO_AI_PROVIDER_KEY") {
					forbidden = append(forbidden, "legacy provider key environment")
				}
			}
		}
		return true
	})
	if !resolverAssigned || !moduleReceivesResolver || !dailyReceivesResolver || !dailyConfigReceivesResolver || !enableInsightGate || !dailyEnabledGate {
		t.Fatalf("tenant resolver composition incomplete: assigned=%v module=%v daily=%v config=%v gates=%v/%v", resolverAssigned, moduleReceivesResolver, dailyReceivesResolver, dailyConfigReceivesResolver, enableInsightGate, dailyEnabledGate)
	}
	if len(forbidden) > 0 {
		t.Fatalf("production AI composition contains forbidden global fallback identifiers: %v", forbidden)
	}
}

func callFunSelector(call *ast.CallExpr) (string, bool) {
	if call == nil {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	return selector.Sel.Name, true
}

func TestDashboardAIStatusProviderDoesNotUseEnvironmentProviderFallback(t *testing.T) {
	const secret = "composition-test-secret"
	t.Setenv("MOCHAT_GO_AI_PROVIDER_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", secret)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_MODEL", "test-model")

	runtime, err := buildDashboardAIStatusProvider(config.Config{EnableAIDebtClearance: true, EnableAIInsight: true})
	if err != nil {
		t.Fatal(err)
	}
	status := runtime.Status()
	if status.State != providers.StateLimited || status.Code != "ai.tenant_scoped" {
		t.Fatalf("status=%#v, global runtime must remain tenant-scoped limited", status)
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{AI: runtime, AIEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, registered := range registry.Snapshot(nil) {
		if registered.Kind == "ai" && registered.State != providers.StateLimited {
			t.Fatalf("enabled AI state=%q, want tenant-scoped limited", registered.State)
		}
	}
	encoded, _ := json.Marshal(status)
	if string(encoded) == "" || string(encoded) == secret || containsSecret(string(encoded), secret) {
		t.Fatal("provider status leaked AI key")
	}
}

func TestDashboardAIStatusProviderDisabledIsCodeOnlyEvenWhenKeyExists(t *testing.T) {
	const secret = "disabled-composition-secret"
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", secret)

	runtime, err := buildDashboardAIStatusProvider(config.Config{EnableAIInsight: false})
	if err != nil {
		t.Fatal(err)
	}
	status := runtime.Status()
	if status.State != providers.StateLimited || status.Code != "ai.disabled" || status.Source != providers.SourceCodeOnly {
		t.Fatalf("status=%#v, disabled AI must be explicit code-only limited", status)
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{AI: runtime, AIEnabled: false})
	if err != nil {
		t.Fatal(err)
	}
	for _, registered := range registry.Snapshot(nil) {
		if registered.Kind == "ai" && (registered.Source != providers.SourceCodeOnly || registered.Code != "ai.disabled") {
			t.Fatalf("disabled AI registration=%#v, want code-only disabled", registered)
		}
	}
	encoded, _ := json.Marshal(status)
	if containsSecret(string(encoded), secret) {
		t.Fatal("disabled provider status leaked AI key")
	}
}

func TestDashboardAIStatusProviderRequiresBothAIFlags(t *testing.T) {
	const secret = "dual-flag-composition-secret"
	t.Setenv("MOCHAT_GO_AI_PROVIDER_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", secret)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_MODEL", "test-model")

	cases := []struct {
		name       string
		debt       bool
		insight    bool
		wantState  providers.State
		wantSource providers.Source
	}{
		{name: "debt disabled insight disabled", debt: false, insight: false, wantState: providers.StateLimited, wantSource: providers.SourceCodeOnly},
		{name: "debt disabled insight enabled", debt: false, insight: true, wantState: providers.StateLimited, wantSource: providers.SourceCodeOnly},
		{name: "debt enabled insight disabled", debt: true, insight: false, wantState: providers.StateLimited, wantSource: providers.SourceCodeOnly},
		{name: "both enabled", debt: true, insight: true, wantState: providers.StateLimited, wantSource: providers.SourceExternal},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			runtime, err := buildDashboardAIStatusProvider(config.Config{EnableAIDebtClearance: test.debt, EnableAIInsight: test.insight})
			if err != nil {
				t.Fatal(err)
			}
			status := runtime.Status()
			if status.State != test.wantState {
				t.Fatalf("status=%#v, want state=%q", status, test.wantState)
			}
			registry, err := catalog.NewRegistry(catalog.Dependencies{AI: runtime, AIEnabled: test.debt && test.insight})
			if err != nil {
				t.Fatal(err)
			}
			registered := findRegisteredProvider(registry.Snapshot(nil), "ai")
			if registered.Source != test.wantSource || registered.State != test.wantState {
				t.Fatalf("registered=%#v, want state=%q source=%q", registered, test.wantState, test.wantSource)
			}
			encoded, err := json.Marshal(status)
			if err != nil {
				t.Fatal(err)
			}
			if containsSecret(string(encoded), secret) {
				t.Fatal("AI key leaked into status")
			}
		})
	}
}

func findRegisteredProvider(statuses []providers.Status, kind string) providers.Status {
	for _, status := range statuses {
		if status.Kind == kind {
			return status
		}
	}
	return providers.Status{}
}

func containsSecret(value, secret string) bool {
	return secret != "" && len(value) > 0 && len(secret) > 0 && strings.Contains(value, secret)
}
