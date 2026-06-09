package doctest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
)

var (
	doctestPackageDir string
	repoRootDir       string
)

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("doctest: failed to resolve caller path")
	}

	doctestPackageDir = filepath.Dir(filename)
	repoRootDir = filepath.Clean(filepath.Join(doctestPackageDir, "..", ".."))
}

func TestMain(m *testing.M) {
	if err := os.Chdir(repoRootDir); err != nil {
		panic(err)
	}

	_ = godotenv.Load(filepath.Join(repoRootDir, ".env"))

	os.Exit(m.Run())
}

func doctestDir() string {
	return doctestPackageDir
}
