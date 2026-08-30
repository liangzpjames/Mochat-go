package migrationhistory

import "testing"

func TestAuditedSupersededLiveCodeRequiresExactHistoryAndValidReplacement(t *testing.T) {
	if !IsAuditedSupersededLiveCode(SupersededLiveCodeVersion, "f89678394dea6164312152f9bbb5298111150d9dea8489252789ef7ed67117ed", true) {
		t.Fatal("audited LF predecessor was not recognized")
	}
	if IsAuditedSupersededLiveCode(SupersededLiveCodeVersion, "wrong", true) {
		t.Fatal("unknown predecessor checksum was recognized")
	}
	if IsAuditedSupersededLiveCode(SupersededLiveCodeVersion, "f89678394dea6164312152f9bbb5298111150d9dea8489252789ef7ed67117ed", false) {
		t.Fatal("predecessor without a valid replacement was recognized")
	}
}

func TestValidLiveCodeWorkspaceChecksums(t *testing.T) {
	for _, checksum := range []string{LegacyLiveCodeWorkspaceChecksum, currentLiveCodeWorkspaceLF, currentLiveCodeWorkspaceCRLF} {
		if !IsValidLiveCodeWorkspaceChecksum(checksum) {
			t.Fatalf("valid checksum %s was rejected", checksum)
		}
	}
	if IsValidLiveCodeWorkspaceChecksum("wrong") {
		t.Fatal("unknown checksum was accepted")
	}
}
