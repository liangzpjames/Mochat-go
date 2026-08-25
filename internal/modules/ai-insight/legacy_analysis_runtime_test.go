package aiinsight

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLegacyAIAnalysisTableHasNoRuntimeReferences(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, directory := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(body)
			for _, forbidden := range []string{"AnalysisStore", "SQLAnalysisStore", "BuildAnalysisPrompt"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("legacy AI analysis runtime reference %q remains in %s", forbidden, path)
				}
			}
			if regexp.MustCompile(`mochat_go_ai_analysis(?:[[:space:]` + "`" + `]|$)`).MatchString(text) {
				t.Errorf("legacy AI analysis table reference remains in %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
