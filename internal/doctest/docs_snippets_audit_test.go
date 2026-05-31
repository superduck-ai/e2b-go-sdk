package doctest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	docSnippetPathPattern  = regexp.MustCompile(`os\.Stat\("([^"]+\.mdx)"\)`)
	docSnippetCountPattern = regexp.MustCompile(`if got := len\(snippets\); got != (\d+) \{`)
)

// This audit ensures the number of compile/prose snippet slots declared in the
// doctest files is never less than the number of Go code fences present in the
// corresponding documentation page.
func TestDocsGoSnippetCoverageAudit(t *testing.T) {
	expectedCounts := collectExpectedSnippetCounts(t)

	docFiles := []string{}
	if _, err := os.Stat("docs.mdx"); err == nil {
		docFiles = append(docFiles, "docs.mdx")
	}

	for path := range collectMDXRelativePaths(t, "docs") {
		docFiles = append(docFiles, filepath.ToSlash(filepath.Join("docs", path)))
	}
	sort.Strings(docFiles)

	var undercovered []string
	for _, path := range docFiles {
		actual := countGoCodeFences(t, path)
		expected, ok := expectedCounts[path]
		if !ok {
			undercovered = append(undercovered, fmt.Sprintf("%s -> missing len(snippets) assertion (doc has %d Go fences)", path, actual))
			continue
		}
		if expected < actual {
			undercovered = append(undercovered, fmt.Sprintf("%s -> doc has %d Go fences but tests declare only %d snippets", path, actual, expected))
		}
	}

	if len(undercovered) != 0 {
		t.Fatalf(
			"found %d docs with insufficient Go snippet coverage:\n- %s",
			len(undercovered),
			strings.Join(undercovered, "\n- "),
		)
	}
}

func collectExpectedSnippetCounts(t *testing.T) map[string]int {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(doctestDir(), "docs*_test.go"))
	if err != nil {
		t.Fatalf("glob doctest files: %v", err)
	}
	sort.Strings(files)

	expected := map[string]int{}
	for _, path := range files {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}

		text := string(content)
		pathMatches := docSnippetPathPattern.FindAllStringSubmatch(text, -1)
		countMatches := docSnippetCountPattern.FindAllStringSubmatch(text, -1)

		if len(countMatches) == 0 {
			continue
		}
		if len(pathMatches) != len(countMatches) {
			t.Fatalf(
				"docs test %s has %d doc paths but %d len(snippets) assertions",
				path,
				len(pathMatches),
				len(countMatches),
			)
		}

		for i, match := range pathMatches {
			if len(match) != 2 {
				t.Fatalf("unexpected doc path match shape in %s", path)
			}
			if len(countMatches[i]) != 2 {
				t.Fatalf("unexpected snippet-count match shape in %s", path)
			}

			count, convErr := strconv.Atoi(countMatches[i][1])
			if convErr != nil {
				t.Fatalf("parse snippet count in %s: %v", path, convErr)
			}

			docPath := filepath.ToSlash(match[1])
			if prev, ok := expected[docPath]; ok && prev != count {
				t.Fatalf("doc %s has conflicting snippet counts %d and %d", docPath, prev, count)
			}
			expected[docPath] = count
		}
	}

	return expected
}

func countGoCodeFences(t *testing.T, path string) int {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	count := 0
	inFence := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "```") {
			continue
		}

		if !inFence {
			inFence = true
			info := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			if info != "" && strings.Fields(info)[0] == "go" {
				count++
			}
			continue
		}

		inFence = false
	}

	return count
}
