//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func setBootstrapPasswordFilePermissions(path string, restricted bool) error {
	user := os.Getenv("USERDOMAIN") + "\\" + os.Getenv("USERNAME")
	if strings.Trim(user, "\\") == "" {
		return fmt.Errorf("current Windows user is unavailable")
	}
	principal := user + ":F"
	if !restricted {
		principal = "Everyone:F"
	}
	command := exec.Command("icacls", path, "/inheritance:r", "/grant:r", principal)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("set test password file ACL: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
