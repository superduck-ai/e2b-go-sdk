package e2b

import (
	"context"
	"reflect"
	"testing"

	roottmpl "github.com/superduck-ai/e2b-go-sdk/template"
)

func TestRootAliasesExposeJsStyleTemplateHelpers(t *testing.T) {
	var _ *TemplateBase = (*roottmpl.TemplateBase)(nil)
	var _ TemplateFromImage = (*roottmpl.TemplateBase)(nil)
	var _ TemplateBuilder = (*roottmpl.TemplateBase)(nil)
	var _ TemplateFinal = (*roottmpl.TemplateBase)(nil)
	var _ TemplateClass = (*roottmpl.TemplateBase)(nil)
	var _ TemplateOptions = roottmpl.TemplateOptions{}
	var _ BasicBuildOptions = roottmpl.BasicBuildOptions{}
	var _ BuildOptions = roottmpl.BuildOptions{}
	var _ BuildInfo = roottmpl.BuildInfo{}
	var _ GetBuildStatusOptions = roottmpl.GetBuildStatusOptions{}
	var _ TemplateBuildStatus = roottmpl.BuildStatusReady
	var _ BuildStatusReason = roottmpl.BuildStatusReason{}
	var _ TemplateBuildStatusResponse = roottmpl.TemplateBuildStatusResponse{}
	var _ TemplateTag = roottmpl.TemplateTag{}
	var _ TemplateTagInfo = roottmpl.TemplateTagInfo{}
	var _ McpServerName = "exa"
	var _ InstructionType = roottmpl.InstructionRun
	var _ Instruction = roottmpl.Instruction{}
	var _ CopyItem = roottmpl.CopyItem{}
	var _ RegistryCredentials = roottmpl.RegistryCredentials{}
	var _ AWSRegistryCredentials = roottmpl.AWSRegistryCredentials{}
	var _ GCPRegistryCredentials = roottmpl.GCPRegistryCredentials{}

	var _ *ReadyCmd = (*roottmpl.ReadyCmd)(nil)
	var _ LogEntryLevel = roottmpl.LogLevelInfo
	var _ LogEntry = roottmpl.LogEntry{}
	var _ *LogEntryStart = (*roottmpl.LogEntryStart)(nil)
	var _ *LogEntryEnd = (*roottmpl.LogEntryEnd)(nil)
	var _ BuildLogger = roottmpl.DefaultBuildLogger()
}

func TestRootTemplateFunctionSignaturesAreAvailable(t *testing.T) {
	if got := reflect.TypeOf(Template); got.In(0) != reflect.TypeOf((*TemplateOptions)(nil)) {
		t.Fatalf("expected Template to accept *TemplateOptions, got %v", got.In(0))
	}

	if got := reflect.TypeOf(Build); got.Out(0) != reflect.TypeOf((*BuildInfo)(nil)) {
		t.Fatalf("expected Build to return *BuildInfo, got %v", got.Out(0))
	}

	if got := reflect.TypeOf(BuildInBackground); got.Out(0) != reflect.TypeOf((*BuildInfo)(nil)) {
		t.Fatalf("expected BuildInBackground to return *BuildInfo, got %v", got.Out(0))
	}

	if got := reflect.TypeOf(GetBuildStatus); got.Out(0) != reflect.TypeOf((*TemplateBuildStatusResponse)(nil)) {
		t.Fatalf("expected GetBuildStatus to return *TemplateBuildStatusResponse, got %v", got.Out(0))
	}

	if got := reflect.TypeOf(AssignTags); got.Out(0) != reflect.TypeOf((*TemplateTagInfo)(nil)) {
		t.Fatalf("expected AssignTags to return *TemplateTagInfo, got %v", got.Out(0))
	}

	if got := reflect.TypeOf(Exists); got.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		t.Fatalf("expected Exists to accept context.Context, got %v", got.In(0))
	}

	if got := reflect.TypeOf(WaitForPort); got.Out(0) != reflect.TypeOf((*ReadyCmd)(nil)) {
		t.Fatalf("expected WaitForPort to return *ReadyCmd, got %v", got.Out(0))
	}

	if got := reflect.TypeOf(DefaultBuildLogger); got.Out(0) != reflect.TypeOf((BuildLogger)(nil)) {
		t.Fatalf("expected DefaultBuildLogger to return BuildLogger, got %v", got.Out(0))
	}
}
