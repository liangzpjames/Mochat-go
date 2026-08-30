package migrationhistory

const (
	LiveCodeWorkspaceVersion = "0153_live_code_workspace"

	// LegacyLiveCodeWorkspaceChecksum is the immutable checksum written by the
	// first Windows Docker Desktop deployment of the current 0153 migration.
	LegacyLiveCodeWorkspaceChecksum = "f17df230c78b79ed0e23d77b87057a939fa8ef5d1ac97fa1db43b5aa34f7344c"
	currentLiveCodeWorkspaceLF      = "defea5a80600a0ca9f094ed95f5f35de89ab85ea97fa32b42f724d5a930e40ea"
	currentLiveCodeWorkspaceCRLF    = "2e2080313f6ad0d05497a6a2eea916e59d11e3daba40d17c8fd7876496e7ad5c"

	SupersededLiveCodeVersion      = "0150_live_code_workspace"
	supersededLiveCodeLFChecksum   = "f89678394dea6164312152f9bbb5298111150d9dea8489252789ef7ed67117ed"
	supersededLiveCodeCRLFChecksum = "5cce5bea0f7b89b7e607772c5aa02ffaf125af27027c79e18772642a4c7ab15d"
)

func IsValidLiveCodeWorkspaceChecksum(checksum string) bool {
	return checksum == LegacyLiveCodeWorkspaceChecksum || checksum == currentLiveCodeWorkspaceLF || checksum == currentLiveCodeWorkspaceCRLF
}

func IsAuditedSupersededLiveCode(version, checksum string, supersedingMigrationValid bool) bool {
	return supersedingMigrationValid && version == SupersededLiveCodeVersion &&
		(checksum == supersededLiveCodeLFChecksum || checksum == supersededLiveCodeCRLFChecksum)
}
