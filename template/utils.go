package template

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

func ValidateRelativePath(src string) error {
	if filepath.IsAbs(src) {
		return fmt.Errorf("path must be relative, got: %s", src)
	}
	clean := filepath.Clean(src)
	if strings.HasPrefix(clean, "..") {
		return fmt.Errorf("path traversal detected: %s", src)
	}
	return nil
}

func CalculateFilesHash(paths []string) string {
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func PadOctal(mode int) string {
	return fmt.Sprintf("%04o", mode)
}
