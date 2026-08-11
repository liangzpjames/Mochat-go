package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSaaSApprovalCompositionUsesGovernanceActorProjection(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "dashboardadmin.NewSaaSApprovalExecutionActor(actorUserID)") {
		t.Fatal("SaaS approval composition must use the resolved governance actor projection")
	}
	if strings.Contains(text, "dashboardadmin.Actor{UserID: actorUserID, Active: true}") {
		t.Fatal("SaaS approval composition must not drop the governance permission projection")
	}
}
