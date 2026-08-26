//go:build !windows

package dashboard

import "os"

func archiveMediaIsReparsePoint(os.FileInfo) bool {
	return false
}
