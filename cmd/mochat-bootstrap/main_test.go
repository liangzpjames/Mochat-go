package main

import "testing"

func TestDefaultCorpEmployeeProvisionIsTenantScopedAndIdempotent(t *testing.T) {
	contract := defaultCorpEmployeeContract(bootstrapOptions{TenantID: 1, TenantName: "默认租户", Phone: "13800000000"}, 1)
	if contract.CorpName != "默认租户演示企业" || contract.WXCorpID != "fake_tenant_1" {
		t.Fatalf("default corp = %#v", contract)
	}
	if contract.TenantID != 1 || contract.UserID != 1 || contract.EmployeeMobile != "13800000000" {
		t.Fatalf("scope = %#v", contract)
	}
	if contract.CorpLookupSQL == "" || contract.EmployeeLookupSQL == "" || contract.EmployeeInsertSQL == "" {
		t.Fatal("bootstrap contract must expose idempotent lookup/insert SQL")
	}
}
