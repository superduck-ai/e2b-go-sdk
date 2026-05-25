package api

import (
	"os"
	"runtime"
)

const Version = "1.0.0"

var DefaultHeaders = map[string]string{
	"lang":            "go",
	"lang_version":    runtime.Version(),
	"package_version": Version,
	"publisher":       "e2b",
	"sdk_runtime":     "go",
	"system":          runtime.GOOS,
}

func GetEnvVar(name string) string {
	return os.Getenv(name)
}
