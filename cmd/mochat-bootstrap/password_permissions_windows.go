//go:build windows

package main

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func validateBootstrapPasswordFilePermissions(path string, _ os.FileInfo) error {
	securityDescriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("inspect bootstrap password file permissions: %w", err)
	}
	if securityDescriptor == nil {
		return fmt.Errorf("bootstrap password file has no security descriptor")
	}
	owner, _, err := securityDescriptor.Owner()
	if err != nil || owner == nil {
		return fmt.Errorf("inspect bootstrap password file owner")
	}
	dacl, _, err := securityDescriptor.DACL()
	if err != nil || dacl == nil {
		return fmt.Errorf("bootstrap password file must have a restricted DACL")
	}
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil {
			return fmt.Errorf("inspect bootstrap password file DACL")
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(uintptr(unsafe.Pointer(ace)) + unsafe.Offsetof(ace.SidStart)))
		if !sid.IsValid() {
			return fmt.Errorf("bootstrap password file contains an invalid DACL principal")
		}
		if sid.Equals(owner) || sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) || sid.IsWellKnown(windows.WinCreatorOwnerSid) {
			continue
		}
		return fmt.Errorf("bootstrap password file DACL grants access beyond the owner and system administrators")
	}
	return nil
}
