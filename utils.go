package e2b

import (
	"crypto/sha256"
	"encoding/base64"
	"math"
	"regexp"
	"strings"
	"time"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func sha256Hash(data string) string {
	h := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(h[:])
}

func timeoutToSeconds(timeout int) int {
	return int(math.Ceil(float64(timeout) / 1000.0))
}

func stripAnsi(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

func wait(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
