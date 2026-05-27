package template

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func validateRelativePath(src string) error {
	if filepath.IsAbs(src) {
		return fmt.Errorf("Invalid source path %q: absolute paths are not allowed. Use a relative path within the context directory.", src)
	}
	clean := filepath.Clean(src)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Invalid source path %q: path escapes the context directory. The path must stay within the context directory.", src)
	}
	return nil
}

func readDockerignore(contextPath string) []string {
	dockerignorePath := filepath.Join(contextPath, ".dockerignore")
	content, err := os.ReadFile(dockerignorePath)
	if err != nil {
		return nil
	}

	var patterns []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func calculateFilesHash(src, dest, contextPath string, ignorePatterns []string, resolveSymlinks bool) (string, error) {
	files, err := getAllFilesInPath(src, contextPath, ignorePatterns, true, resolveSymlinks)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("No files found in %s", filepath.Join(contextPath, src))
	}

	h := sha256.New()
	h.Write([]byte("COPY " + src + " " + dest))

	for _, relPath := range files {
		fullPath := filepath.Join(contextPath, filepath.FromSlash(relPath))
		info, err := fileInfoForPath(fullPath, resolveSymlinks)
		if err != nil {
			return "", err
		}

		h.Write([]byte(relPath))
		h.Write([]byte(info.Mode().String()))
		h.Write([]byte(fmt.Sprintf("%d", info.Size())))

		if info.Mode()&os.ModeSymlink != 0 && !resolveSymlinks {
			target, err := os.Readlink(fullPath)
			if err != nil {
				return "", err
			}
			h.Write([]byte(target))
			continue
		}

		if info.Mode().IsRegular() {
			content, err := os.ReadFile(fullPath)
			if err != nil {
				return "", err
			}
			h.Write(content)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func padOctal(mode int) string {
	return fmt.Sprintf("%04o", mode)
}

func tarFileBytes(src, contextPath string, ignorePatterns []string, resolveSymlinks bool) ([]byte, error) {
	files, err := getAllFilesInPath(src, contextPath, ignorePatterns, true, resolveSymlinks)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for _, relPath := range files {
		fullPath := filepath.Join(contextPath, filepath.FromSlash(relPath))
		info, err := fileInfoForPath(fullPath, resolveSymlinks)
		if err != nil {
			return nil, err
		}

		link := ""
		lstat, err := os.Lstat(fullPath)
		if err == nil && lstat.Mode()&os.ModeSymlink != 0 && !resolveSymlinks {
			link, err = os.Readlink(fullPath)
			if err != nil {
				return nil, err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return nil, err
		}
		header.Name = relPath
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(fullPath)
			if err != nil {
				return nil, err
			}
			_, copyErr := io.Copy(tw, file)
			closeErr := file.Close()
			if copyErr != nil {
				return nil, copyErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func getAllFilesInPath(src, contextPath string, ignorePatterns []string, includeDirectories, resolveSymlinks bool) ([]string, error) {
	if err := validateRelativePath(src); err != nil {
		return nil, err
	}

	base := contextPath
	if base == "" {
		base = "."
	}
	base = filepath.Clean(base)
	sourcePattern := normalizePath(filepath.Clean(filepath.FromSlash(src)))

	files := map[string]struct{}{}
	addPath := func(fullPath string) error {
		info, err := fileInfoForPath(fullPath, resolveSymlinks)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(base, fullPath)
		if err != nil {
			return err
		}
		rel = normalizePath(rel)
		if shouldIgnore(rel, ignorePatterns) {
			return nil
		}

		if info.IsDir() {
			if includeDirectories {
				files[rel] = struct{}{}
			}
			return filepath.WalkDir(fullPath, func(walkPath string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if walkPath == fullPath {
					return nil
				}

				childRel, err := filepath.Rel(base, walkPath)
				if err != nil {
					return err
				}
				childRel = normalizePath(childRel)
				if shouldIgnore(childRel, ignorePatterns) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if d.IsDir() {
					if includeDirectories {
						files[childRel] = struct{}{}
					}
					return nil
				}
				files[childRel] = struct{}{}
				return nil
			})
		}

		files[rel] = struct{}{}
		return nil
	}

	if !hasGlobMeta(sourcePattern) {
		fullPath := filepath.Join(base, filepath.FromSlash(sourcePattern))
		if err := addPath(fullPath); err != nil {
			return nil, err
		}
		result := make([]string, 0, len(files))
		for file := range files {
			result = append(result, file)
		}
		sort.Strings(result)
		return result, nil
	}

	if err := filepath.WalkDir(base, func(walkPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(base, walkPath)
		if err != nil {
			return err
		}
		rel = normalizePath(rel)
		if rel != "." && shouldIgnore(rel, ignorePatterns) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "." && sourcePattern != "." {
			return nil
		}
		if !globMatch(sourcePattern, rel) {
			return nil
		}
		if err := addPath(walkPath); err != nil {
			return err
		}
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	sort.Strings(result)
	return result, nil
}

func hasGlobMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func fileInfoForPath(fullPath string, resolveSymlinks bool) (os.FileInfo, error) {
	if resolveSymlinks {
		return os.Stat(fullPath)
	}
	return os.Lstat(fullPath)
}

func shouldIgnore(relPath string, ignorePatterns []string) bool {
	normalized := normalizePath(relPath)
	baseName := path.Base(normalized)
	for _, pattern := range ignorePatterns {
		pattern = normalizePath(pattern)
		if globMatch(pattern, normalized) {
			return true
		}
		if !strings.Contains(pattern, "/") && globMatch(pattern, baseName) {
			return true
		}
		prefix := strings.TrimSuffix(pattern, "/")
		if prefix != "" && strings.HasPrefix(normalized, prefix+"/") {
			return true
		}
	}
	return false
}

func normalizePath(value string) string {
	return filepath.ToSlash(value)
}

func globMatch(pattern, value string) bool {
	pattern = strings.TrimPrefix(normalizePath(pattern), "./")
	value = strings.TrimPrefix(normalizePath(value), "./")
	return globPartsMatch(splitGlobPath(pattern), splitGlobPath(value))
}

func splitGlobPath(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}

func globPartsMatch(patternParts, valueParts []string) bool {
	if len(patternParts) == 0 {
		return len(valueParts) == 0
	}
	if patternParts[0] == "**" {
		if globPartsMatch(patternParts[1:], valueParts) {
			return true
		}
		for i := range valueParts {
			if globPartsMatch(patternParts[1:], valueParts[i+1:]) {
				return true
			}
		}
		return false
	}
	if len(valueParts) == 0 {
		return false
	}
	matched, err := path.Match(patternParts[0], valueParts[0])
	if err != nil || !matched {
		return false
	}
	return globPartsMatch(patternParts[1:], valueParts[1:])
}
