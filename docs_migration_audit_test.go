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

const upstreamDocsRoot = "/data/docs/docs"

var (
	docStatPattern         = regexp.MustCompile(`os\.Stat\("([^"]+\.mdx)"\)`)
	versionDirPattern      = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	generatedReferenceSDKs = map[string]struct{}{
		"cli":                         {},
		"js-sdk":                      {},
		"python-sdk":                  {},
		"code-interpreter-js-sdk":     {},
		"code-interpreter-python-sdk": {},
		"desktop-js-sdk":              {},
		"desktop-python-sdk":          {},
	}
)

// This audit keeps the Go docs migration aligned with the upstream docs tree.
// The upstream repository carries many versioned sdk-reference snapshots that
// are auto-generated for non-Go SDKs; those historical artifacts are excluded
// from this migration check.
func TestDocsMigrationCoversUpstreamCurrentScope(t *testing.T) {
	if _, err := os.Stat(upstreamDocsRoot); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("upstream docs source is not available at %s", upstreamDocsRoot)
		}
		t.Fatalf("stat upstream docs root: %v", err)
	}

	upstreamDocs := collectMDXRelativePaths(t, upstreamDocsRoot)
	localDocs := collectMDXRelativePaths(t, "docs")

	var missing []string
	for _, path := range missingPaths(upstreamDocs, localDocs) {
		if isGeneratedUpstreamReferenceSnapshot(path) {
			continue
		}
		missing = append(missing, path)
	}

	if len(missing) != 0 {
		t.Fatalf(
			"found %d upstream docs that should be migrated into docs/ but are missing:\n%s",
			len(missing),
			formatPathList(missing),
		)
	}
}

func TestDocsHomeMigratedWhenUpstreamAvailable(t *testing.T) {
	upstreamHome := "/data/docs/docs.mdx"

	if _, err := os.Stat(upstreamHome); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("upstream docs home page is not available at %s", upstreamHome)
		}
		t.Fatalf("stat upstream docs home page: %v", err)
	}

	if _, err := os.Stat("docs.mdx"); err != nil {
		t.Fatalf("local docs home page is missing while upstream %s exists: %v", upstreamHome, err)
	}
}

// This audit ensures every local documentation page is pinned by at least one
// root-level doc test through an explicit os.Stat("docs/...") assertion.
func TestDocsLocalPagesHaveDedicatedTests(t *testing.T) {
	localDocs := collectMDXRelativePaths(t, "docs")
	referencedDocs := collectReferencedDocsFromRootTests(t)

	missing := missingPaths(localDocs, referencedDocs)
	if len(missing) != 0 {
		t.Fatalf(
			"found %d local docs without a dedicated root test reference:\n%s",
			len(missing),
			formatPathList(missing),
		)
	}

	extra := missingPaths(referencedDocs, localDocs)
	if len(extra) != 0 {
		t.Fatalf(
			"found %d root test references to docs that do not exist:\n%s",
			len(extra),
			formatPathList(extra),
		)
	}
}

func collectMDXRelativePaths(t *testing.T, root string) map[string]struct{} {
	t.Helper()

	paths := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".mdx" {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		paths[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return paths
}

func collectReferencedDocsFromRootTests(t *testing.T) map[string]struct{} {
	t.Helper()

	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob root test files: %v", err)
	}

	refs := map[string]struct{}{}
	for _, path := range files {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}

		matches := docStatPattern.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) != 2 || !strings.HasPrefix(match[1], "docs/") {
				continue
			}
			refs[strings.TrimPrefix(match[1], "docs/")] = struct{}{}
		}
	}

	return refs
}

func missingPaths(want map[string]struct{}, got map[string]struct{}) []string {
	var missing []string
	for path := range want {
		if _, ok := got[path]; ok {
			continue
		}
		missing = append(missing, path)
	}
	sort.Strings(missing)
	return missing
}

func isGeneratedUpstreamReferenceSnapshot(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[0] != "sdk-reference" {
		return false
	}

	if _, ok := generatedReferenceSDKs[parts[1]]; !ok {
		return false
	}

	return versionDirPattern.MatchString(parts[2])
}

func formatPathList(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	lines := make([]string, 0, len(paths))
	for _, path := range paths {
		lines = append(lines, fmt.Sprintf("- %s", path))
	}
	return strings.Join(lines, "\n")
}
