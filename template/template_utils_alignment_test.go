package template

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestValidateRelativePathMatchesJsAndPython(t *testing.T) {
	valid := []string{
		"foo",
		"foo/bar",
		"./foo",
		"foo/../bar",
		".",
		"*.txt",
		"**/*.ts",
		"src/**/*",
		".hidden",
		".config/settings",
		"..myconfig",
		"..cache",
		"...something",
		"foo/..myconfig",
	}
	for _, src := range valid {
		if err := validateRelativePath(src); err != nil {
			t.Fatalf("validateRelativePath(%q) returned error: %v", src, err)
		}
	}

	invalid := []string{
		"/absolute/path",
		"/",
		"../foo",
		"../../foo",
		"foo/../../bar",
		"./foo/../../../bar",
		"..",
		"./..",
		"a/b/c/../../../../escape",
	}
	for _, src := range invalid {
		if err := validateRelativePath(src); err == nil {
			t.Fatalf("validateRelativePath(%q) succeeded, want error", src)
		} else if !strings.Contains(err.Error(), src) {
			t.Fatalf("validateRelativePath(%q) error should include path, got %v", src, err)
		}
	}
}

func TestGetAllFilesInPathMatchesJsAndPythonGlobBehavior(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir, "file1.txt", "content1")
	writeTemplateFixture(t, dir, "file2.txt", "content2")
	writeTemplateFixture(t, dir, "file3.js", "content3")
	writeTemplateFixture(t, dir, ".env", "SECRET=123")
	writeTemplateFixture(t, dir, ".gitignore", "node_modules")

	files, err := getAllFilesInPath("*.txt", dir, nil, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath returned error: %v", err)
	}
	assertStringSet(t, files, []string{"file1.txt", "file2.txt"})

	files, err = getAllFilesInPath("*.missing", dir, nil, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath no-match glob returned error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no-match glob to return empty slice, got %#v", files)
	}

	files, err = getAllFilesInPath("*", dir, []string{".env"}, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath dotfile pattern returned error: %v", err)
	}
	assertStringSet(t, files, []string{".gitignore", "file1.txt", "file2.txt", "file3.js"})
}

func TestGetAllFilesInPathExactSourceDoesNotWalkWholeContext(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir, "package.json", "{}")

	unreadable := filepath.Join(dir, "aaa-unreadable")
	if err := os.Mkdir(unreadable, 0o755); err != nil {
		t.Fatalf("failed to create unreadable fixture directory: %v", err)
	}
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatalf("failed to chmod unreadable fixture directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unreadable, 0o755)
	})

	files, err := getAllFilesInPath("package.json", dir, nil, true, false)
	if err != nil {
		t.Fatalf("exact source should not walk unrelated unreadable directories: %v", err)
	}
	assertStringSet(t, files, []string{"package.json"})
}

func TestGetAllFilesInPathMatchesJsAndPythonRecursiveDirectoryBehavior(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir, "src/index.ts", "index content")
	writeTemplateFixture(t, dir, "src/components/Button.tsx", "button content")
	writeTemplateFixture(t, dir, "src/components/Button.test.tsx", "test content")
	writeTemplateFixture(t, dir, "src/components/Button.css", "css content")
	writeTemplateFixture(t, dir, "src/utils/helper.ts", "helper content")
	writeTemplateFixture(t, dir, "src/utils/helper.spec.ts", "spec content")
	writeTemplateFixture(t, dir, "README.md", "readme content")

	files, err := getAllFilesInPath("src", dir, []string{"**/*.test.*", "**/*.spec.*"}, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath directory returned error: %v", err)
	}
	assertStringSet(t, files, []string{
		"src",
		"src/components",
		"src/components/Button.css",
		"src/components/Button.tsx",
		"src/index.ts",
		"src/utils",
		"src/utils/helper.ts",
	})

	files, err = getAllFilesInPath("src/**/*", dir, nil, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath globstar returned error: %v", err)
	}
	assertStringSet(t, files, []string{
		"src/components",
		"src/components/Button.css",
		"src/components/Button.test.tsx",
		"src/components/Button.tsx",
		"src/index.ts",
		"src/utils",
		"src/utils/helper.spec.ts",
		"src/utils/helper.ts",
	})
}

func TestGetAllFilesInPathMatchesNestedIgnorePatterns(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir, "src/index.ts", "index content")
	writeTemplateFixture(t, dir, "src/components/ui/Button.tsx", "button content")
	writeTemplateFixture(t, dir, "src/components/ui/Button.test.tsx", "test content")
	writeTemplateFixture(t, dir, "src/components/forms/Input.tsx", "input content")
	writeTemplateFixture(t, dir, "src/utils/helper.ts", "helper content")

	files, err := getAllFilesInPath("src", dir, []string{"**/ui/**"}, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath nested ignore returned error: %v", err)
	}
	assertStringSet(t, files, []string{
		"src",
		"src/components",
		"src/components/forms",
		"src/components/forms/Input.tsx",
		"src/index.ts",
		"src/utils",
		"src/utils/helper.ts",
	})
}

func TestGetAllFilesInPathTreatsSlashlessIgnorePatternsAsRootRelativeLikeJsAndPython(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir, ".env", "root secret")
	writeTemplateFixture(t, dir, "temp.txt", "root temp")
	writeTemplateFixture(t, dir, "sub/.env", "nested secret")
	writeTemplateFixture(t, dir, "sub/temp.txt", "nested temp")
	writeTemplateFixture(t, dir, "sub/file.txt", "nested file")

	files, err := getAllFilesInPath("*", dir, []string{".env", "temp*"}, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath root-relative ignore returned error: %v", err)
	}
	assertStringSet(t, files, []string{
		"sub",
		"sub/.env",
		"sub/file.txt",
		"sub/temp.txt",
	})

	files, err = getAllFilesInPath("sub", dir, []string{".env", "temp*"}, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath nested root-relative ignore returned error: %v", err)
	}
	assertStringSet(t, files, []string{
		"sub",
		"sub/.env",
		"sub/file.txt",
		"sub/temp.txt",
	})
}

func TestGetAllFilesInPathDotPatternStaysWithinContextLikeJsAndPython(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir, "root.txt", "root")
	writeTemplateFixture(t, dir, "subdir/nested.txt", "nested")

	files, err := getAllFilesInPath(".", dir, nil, true, false)
	if err != nil {
		t.Fatalf("getAllFilesInPath dot pattern returned error: %v", err)
	}
	assertStringSet(t, files, []string{
		".",
		"root.txt",
		"subdir",
		"subdir/nested.txt",
	})
}

func TestTarFileBytesPreservesAndResolvesSymlinksLikeJsAndPython(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir, "original.txt", "original content")
	if err := os.Symlink("original.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	preserved, err := tarFileBytes("*.txt", dir, nil, false)
	if err != nil {
		t.Fatalf("tarFileBytes preserve symlink returned error: %v", err)
	}
	preservedEntries := readTarEntries(t, preserved)
	if preservedEntries["original.txt"].Body != "original content" {
		t.Fatalf("expected original file body, got %#v", preservedEntries["original.txt"])
	}
	if preservedEntries["link.txt"].Typeflag != tar.TypeSymlink || preservedEntries["link.txt"].Linkname != "original.txt" {
		t.Fatalf("expected link.txt symlink to be preserved, got %#v", preservedEntries["link.txt"])
	}

	resolved, err := tarFileBytes("*.txt", dir, nil, true)
	if err != nil {
		t.Fatalf("tarFileBytes resolve symlink returned error: %v", err)
	}
	resolvedEntries := readTarEntries(t, resolved)
	if resolvedEntries["original.txt"].Body != "original content" || resolvedEntries["link.txt"].Body != "original content" {
		t.Fatalf("expected symlink to be resolved to file content, got %#v", resolvedEntries)
	}
	if resolvedEntries["link.txt"].Linkname != "" {
		t.Fatalf("expected resolved link to be regular file, got %#v", resolvedEntries["link.txt"])
	}
}

func TestCalculateFilesHashStillErrorsWhenNoFilesMatch(t *testing.T) {
	_, err := calculateFilesHash("*.missing", "/app", t.TempDir(), nil, false)
	if err == nil {
		t.Fatal("expected calculateFilesHash to fail when no files match")
	}
	if !strings.Contains(err.Error(), "No files found") {
		t.Fatalf("unexpected calculateFilesHash error: %v", err)
	}
}

type tarEntry struct {
	Typeflag byte
	Linkname string
	Body     string
}

func readTarEntries(t *testing.T, data []byte) map[string]tarEntry {
	t.Helper()
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to open gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	entries := map[string]tarEntry{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}
		entry := tarEntry{Typeflag: header.Typeflag, Linkname: header.Linkname}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("failed to read tar body for %s: %v", header.Name, err)
			}
			entry.Body = string(body)
		}
		entries[strings.TrimSuffix(header.Name, "/")] = entry
	}
	return entries
}

func writeTemplateFixture(t *testing.T, dir string, rel string, contents string) {
	t.Helper()
	fullPath := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("failed to create parent directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", rel, err)
	}
}

func assertStringSet(t *testing.T, got []string, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("string set mismatch:\n got %#v\nwant %#v", got, want)
	}
}
