package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root := flag.String("root", ".", "repository root")
	directory := flag.String("dir", "", "directory relative to root")
	goos := flag.String("goos", "linux", "production image GOOS")
	goarch := flag.String("goarch", "amd64", "production image GOARCH")
	flag.Parse()
	if strings.TrimSpace(*directory) == "" {
		fail("--dir is required")
	}
	files, err := productionGoFiles(*root, *directory, *goos, *goarch)
	if err != nil {
		fail(err.Error())
	}
	if err := json.NewEncoder(os.Stdout).Encode(files); err != nil {
		fail(err.Error())
	}
}

func productionGoFiles(root, relativeDirectory, goos, goarch string) ([]string, error) {
	target, err := productionBuildContext(goos, goarch)
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(root, filepath.FromSlash(relativeDirectory))
	files := make([]string, 0)
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return files, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat production Go directory: %w", err)
	}
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != directory && (entry.Name() == "fixtures" || entry.Name() == "testdata" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		matched, matchErr := target.MatchFile(filepath.Dir(path), entry.Name())
		if matchErr != nil {
			return matchErr
		}
		if matched {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan production Go files: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func productionBuildContext(goos, goarch string) (build.Context, error) {
	if strings.TrimSpace(goos) == "" || strings.TrimSpace(goarch) == "" {
		return build.Context{}, fmt.Errorf("production build target requires GOOS and GOARCH")
	}
	return build.Context{GOOS: goos, GOARCH: goarch, Compiler: "gc", CgoEnabled: false}, nil
}

func fail(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
