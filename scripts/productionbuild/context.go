package productionbuild

import (
	"fmt"
	"go/build"
	"strings"
)

// ForTarget starts from the host Go build context so release/tool tags and
// other compiler metadata stay identical to the image toolchain. Only the
// image contract is overridden by callers.
func ForTarget(goos, goarch string) (build.Context, error) {
	if strings.TrimSpace(goos) == "" || strings.TrimSpace(goarch) == "" {
		return build.Context{}, fmt.Errorf("production build target requires GOOS and GOARCH")
	}
	target := build.Default
	target.GOOS = goos
	target.GOARCH = goarch
	target.CgoEnabled = false
	target.Compiler = "gc"
	return target, nil
}
