//go:build !windows

package main

import "os"

func setBootstrapPasswordFilePermissions(path string, restricted bool) error {
	if restricted {
		return os.Chmod(path, 0600)
	}
	return os.Chmod(path, 0644)
}
