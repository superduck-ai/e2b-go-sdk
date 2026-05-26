package template

import (
	"os"
	"strings"
	"testing"
)

func TestTemplateInternalsDoNotExposeJsInternalHelpers(t *testing.T) {
	files := []string{
		"consts.go",
		"utils.go",
		"dockerfile_parser.go",
		"build_api.go",
		"template.go",
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		text := string(data)

		disallowed := []string{
			"FinalizeStepName",
			"BaseStepName",
			"StackTraceDepth",
			"ResolveSymlinks  =",
			"func CalculateFilesHash(",
			"func CheckAliasExists(",
			"func PadOctal(",
			"func ParseDockerfile(",
			"func RequestBuild(",
			"func TriggerBuild(",
			"func GetBuildStatusFromAPI(",
			"func WaitForBuildFinish(",
			"func GetTemplateTags(",
			"func NewTemplate(",
			"func GetBuildStatusByData(",
			"func AssignTemplateTags(",
			"func RemoveTemplateTags(",
		}

		for _, needle := range disallowed {
			if strings.Contains(text, needle) {
				t.Fatalf("did not expect template package to export %q in %s", needle, file)
			}
		}
	}
}
