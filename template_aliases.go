package e2b

import (
	"context"

	roottmpl "github.com/e2b-dev/e2b-go-sdk/template"
)

type TemplateBase = roottmpl.TemplateBase
type TemplateFromImage = roottmpl.TemplateFromImage
type TemplateBuilder = roottmpl.TemplateBuilder
type TemplateFinal = roottmpl.TemplateFinal
type TemplateClass = roottmpl.TemplateClass
type TemplateOptions = roottmpl.TemplateOptions
type BasicBuildOptions = roottmpl.BasicBuildOptions
type BuildOptions = roottmpl.BuildOptions
type BuildInfo = roottmpl.BuildInfo
type GetBuildStatusOptions = roottmpl.GetBuildStatusOptions
type TemplateBuildStatus = roottmpl.TemplateBuildStatus
type TemplateBuildStatusResponse = roottmpl.TemplateBuildStatusResponse
type TemplateTag = roottmpl.TemplateTag
type TemplateTagInfo = roottmpl.TemplateTagInfo
type InstructionType = roottmpl.InstructionType
type Instruction = roottmpl.Instruction
type CopyItem = roottmpl.CopyItem
type McpServerName = roottmpl.McpServerName
type RegistryCredentials = roottmpl.RegistryCredentials
type AWSRegistryCredentials = roottmpl.AWSRegistryCredentials
type GCPRegistryCredentials = roottmpl.GCPRegistryCredentials

type ReadyCmd = roottmpl.ReadyCmd

type LogEntryLevel = roottmpl.LogEntryLevel
type LogEntry = roottmpl.LogEntry
type LogEntryStart = roottmpl.LogEntryStart
type LogEntryEnd = roottmpl.LogEntryEnd
type BuildLogger = roottmpl.BuildLogger

func Template(options *TemplateOptions) *TemplateBase {
	return roottmpl.Template(options)
}

func Build(ctx context.Context, template *TemplateBase, name string, opts *BuildOptions) (*BuildInfo, error) {
	return roottmpl.Build(ctx, template, name, opts)
}

func BuildInBackground(ctx context.Context, template *TemplateBase, name string, opts *BuildOptions) (*BuildInfo, error) {
	return roottmpl.BuildInBackground(ctx, template, name, opts)
}

func ToJSON(template *TemplateBase, computeHashes ...bool) (string, error) {
	return roottmpl.ToJSON(template, computeHashes...)
}

func ToDockerfile(template *TemplateBase) (string, error) {
	return roottmpl.ToDockerfile(template)
}

func GetBuildStatus(ctx context.Context, templateID, buildID string, opts *GetBuildStatusOptions) (*TemplateBuildStatusResponse, error) {
	return roottmpl.GetBuildStatus(ctx, templateID, buildID, opts)
}

func Exists(ctx context.Context, name string, opts *BuildOptions) (bool, error) {
	return roottmpl.Exists(ctx, name, opts)
}

// AliasExists is deprecated. Use Exists instead.
func AliasExists(ctx context.Context, alias string, opts *BuildOptions) (bool, error) {
	return roottmpl.AliasExists(ctx, alias, opts)
}

type BuildStatusReason = roottmpl.BuildStatusReason

func AssignTags(ctx context.Context, targetName string, tags []string, opts *BuildOptions) (*TemplateTagInfo, error) {
	return roottmpl.AssignTags(ctx, targetName, tags, opts)
}

func RemoveTags(ctx context.Context, name string, tags []string, opts *BuildOptions) error {
	return roottmpl.RemoveTags(ctx, name, tags, opts)
}

func GetTags(ctx context.Context, templateID string, opts *BuildOptions) ([]TemplateTag, error) {
	return roottmpl.GetTags(ctx, templateID, opts)
}

func WaitForPort(port int) *ReadyCmd {
	return roottmpl.WaitForPort(port)
}

func WaitForURL(url string, statusCode ...int) *ReadyCmd {
	return roottmpl.WaitForURL(url, statusCode...)
}

func WaitForProcess(processName string) *ReadyCmd {
	return roottmpl.WaitForProcess(processName)
}

func WaitForFile(filename string) *ReadyCmd {
	return roottmpl.WaitForFile(filename)
}

func WaitForTimeout(timeoutMs int) *ReadyCmd {
	return roottmpl.WaitForTimeout(timeoutMs)
}

func NewLogEntryStart(message string) *LogEntryStart {
	return roottmpl.NewLogEntryStart(message)
}

func NewLogEntryEnd(message string) *LogEntryEnd {
	return roottmpl.NewLogEntryEnd(message)
}

func DefaultBuildLogger() BuildLogger {
	return roottmpl.DefaultBuildLogger()
}
