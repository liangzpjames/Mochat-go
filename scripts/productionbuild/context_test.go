package productionbuild

import (
	"go/build"
	"reflect"
	"testing"
)

func TestForTargetPreservesHostBuildMetadataWhileSelectingProductionTarget(t *testing.T) {
	target, err := ForTarget("linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if target.GOOS != "linux" || target.GOARCH != "amd64" || target.Compiler != "gc" || target.CgoEnabled {
		t.Fatalf("target = %#v, want linux/amd64 gc with cgo disabled", target)
	}
	if !reflect.DeepEqual(target.ReleaseTags, build.Default.ReleaseTags) {
		t.Fatalf("ReleaseTags were not preserved: got=%v want=%v", target.ReleaseTags, build.Default.ReleaseTags)
	}
	if !reflect.DeepEqual(target.ToolTags, build.Default.ToolTags) {
		t.Fatalf("ToolTags were not preserved: got=%v want=%v", target.ToolTags, build.Default.ToolTags)
	}
}

func TestForTargetRejectsMissingTargetCoordinates(t *testing.T) {
	if _, err := ForTarget("", "amd64"); err == nil {
		t.Fatal("missing GOOS unexpectedly accepted")
	}
	if _, err := ForTarget("linux", ""); err == nil {
		t.Fatal("missing GOARCH unexpectedly accepted")
	}
}
