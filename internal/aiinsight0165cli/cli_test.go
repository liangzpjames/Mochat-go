package aiinsight0165cli

import (
	"strings"
	"testing"
)

func TestParseOptionsRequiresSecretFileAndActionSpecificEvidence(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"dsn file", []string{"inventory"}, "--dsn-file is required"},
		{"request id", []string{"backup", "--dsn-file", "dsn.txt"}, "--request-id is required"},
		{"approval token", []string{"apply", "--dsn-file", "dsn.txt", "--request-id", "r1", "--approve-destructive", "duplicates=1,legacy=1", "--confirm-traffic-stopped"}, "--approval-token is required"},
		{"destructive counts", []string{"apply", "--dsn-file", "dsn.txt", "--request-id", "r1", "--approval-token", "token", "--confirm-traffic-stopped"}, "--approve-destructive is required"},
		{"traffic stopped", []string{"apply", "--dsn-file", "dsn.txt", "--request-id", "r1", "--approval-token", "token", "--approve-destructive", "duplicates=1,legacy=1"}, "--confirm-traffic-stopped is required"},
		{"unknown action", []string{"destroy", "--dsn-file", "dsn.txt"}, "action must be one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseOptions(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseOptions(%q) error = %v, want %q", tt.args, err, tt.want)
			}
		})
	}
}

func TestParseOptionsAcceptsControlledLifecycleActions(t *testing.T) {
	for _, action := range []string{"inventory", "backup", "preflight", "verify"} {
		args := []string{action, "--dsn-file", "dsn.txt", "--project-root", "repo"}
		if action != "inventory" {
			args = append(args, "--request-id", "task8-0165")
		}
		options, err := ParseOptions(args)
		if err != nil {
			t.Fatalf("ParseOptions(%q): %v", args, err)
		}
		if options.Action != action || options.DSNFile != "dsn.txt" || options.ProjectRoot != "repo" {
			t.Fatalf("options = %+v", options)
		}
	}

	options, err := ParseOptions([]string{
		"apply", "--dsn-file", "dsn.txt", "--project-root", "repo", "--request-id", "task8-0165",
		"--approval-token", "approval-v1:abc", "--approve-destructive", "duplicates=1,legacy=1", "--confirm-traffic-stopped",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !options.ConfirmTrafficStopped || options.ApprovalToken != "approval-v1:abc" {
		t.Fatalf("apply options = %+v", options)
	}
}
