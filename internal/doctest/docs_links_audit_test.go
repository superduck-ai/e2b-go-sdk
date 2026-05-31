package doctest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var docsInternalLinkPattern = regexp.MustCompile(`/docs(?:/[A-Za-z0-9._#?=/-]*)?`)

// This audit ensures internal /docs/... links resolve to local documentation
// pages in this repository.
func TestDocsInternalLinksResolve(t *testing.T) {
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

		matches := docsInternalLinkPattern.FindAllString(string(content), -1)
		for _, match := range matches {
			if docsLinkExists(match) {
				continue
			}
			missing = append(missing, fmt.Sprintf("%s -> %s", path, match))
		}
	}

	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf(
			"found %d broken internal documentation links:\n- %s",
			len(missing),
			strings.Join(missing, "\n- "),
		)
	}
}

func docsLinkExists(target string) bool {
	path := strings.SplitN(target, "#", 2)[0]
	path = strings.SplitN(path, "?", 2)[0]
	path = strings.TrimRight(path, "/")

	if path == "/docs" {
		_, err := os.Stat("docs.mdx")
		return err == nil
	}

	rel := strings.Trim(strings.TrimPrefix(path, "/docs/"), "/")
	if rel == "" {
		_, err := os.Stat("docs.mdx")
		return err == nil
	}

	candidates := []string{
		filepath.Join("docs", rel+".mdx"),
		filepath.Join("docs", rel, "index.mdx"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}

	return false
}
