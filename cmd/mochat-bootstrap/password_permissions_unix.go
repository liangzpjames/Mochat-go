//go:build !windows

package main

import (
	"fmt"
	"os"
)

func validateBootstrapPasswordFilePermissions(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("bootstrap password file permissions must restrict group and other access")
	}
	return nil
}
