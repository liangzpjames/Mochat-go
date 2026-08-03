package dashboard

import "testing"

func TestValidateSilentRule(t *testing.T) {
	if ValidateSilentRule(SilentCustomerRule{Name: "30天沉默", SilentDays: 30, Status: "enabled"}) != nil {
		t.Fatal("valid rule rejected")
	}
	if ValidateSilentRule(SilentCustomerRule{Name: "x", SilentDays: 0, Status: "enabled"}) == nil {
		t.Fatal("invalid days accepted")
	}
}
func TestValidateRefuseArchive(t *testing.T) {
	if ValidateRefuseArchive(RefuseArchiveRecord{SubjectType: "customer", SubjectID: "wx1", AuthorizationStatus: "refused"}) != nil {
		t.Fatal("valid snapshot rejected")
	}
}
