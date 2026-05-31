package e2b_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var docsImagePathPattern = regexp.MustCompile(`/images/[^)"'\s>]+`)

// This audit keeps local documentation assets in sync with the paths referenced
// from docs.mdx and docs/**/*.mdx.
func TestDocsImageReferencesExist(t *testing.T) {
	docFiles := []string{}

	if _, err := os.Stat("docs.mdx"); err == nil {
		docFiles = append(docFiles, "docs.mdx")
	}

	err := filepath.WalkDir("docs", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".mdx" {
			return nil
		}
		docFiles = append(docFiles, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs directory: %v", err)
	}

	sort.Strings(docFiles)

	var missing []string
	for _, path := range docFiles {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}

		matches := docsImagePathPattern.FindAllString(string(content), -1)
		for _, match := range matches {
			assetPath := strings.TrimPrefix(match, "/")
			if _, statErr := os.Stat(assetPath); statErr == nil {
				continue
			}
			missing = append(missing, fmt.Sprintf("%s -> %s", path, match))
		}
	}

	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf(
			"found %d missing documentation assets:\n- %s",
			len(missing),
			strings.Join(missing, "\n- "),
		)
	}
}
