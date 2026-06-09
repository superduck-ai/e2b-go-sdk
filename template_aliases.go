package e2b

import (
	"context"

	roottmpl "github.com/superduck-ai/e2b-go-sdk/template"
)

type TemplateBase = roottmpl.TemplateBase
type TemplateFromImage = roottmpl.TemplateFromImage
type TemplateInfo = roottmpl.TemplateInfo
type ListTemplatesOpts = roottmpl.ListTemplatesOpts
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

func GetBuildStatus(ctx context.Context, buildInfo *BuildInfo, opts *GetBuildStatusOptions) (*TemplateBuildStatusResponse, error) {
	return roottmpl.GetBuildStatus(ctx, buildInfo, opts)
}

func toTemplateConnectionOpts(opts *ConnectionOpts) *roottmpl.ConnectionOpts {
	if opts == nil {
		return nil
	}

	return &roottmpl.ConnectionOpts{
		ApiKey:           opts.ApiKey,
		AccessToken:      opts.AccessToken,
		Domain:           opts.Domain,
		ApiUrl:           opts.ApiUrl,
		SandboxUrl:       opts.SandboxUrl,
		Debug:            opts.Debug,
		Signal:           opts.Signal,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Headers:          opts.Headers,
		Logger:           opts.Logger,
		Proxy:            opts.Proxy,
	}
}

func Exists(ctx context.Context, name string, opts *ConnectionOpts) (bool, error) {
	return roottmpl.Exists(ctx, name, toTemplateConnectionOpts(opts))
}

// AliasExists is deprecated. Use Exists instead.
func AliasExists(ctx context.Context, alias string, opts *ConnectionOpts) (bool, error) {
	return roottmpl.AliasExists(ctx, alias, toTemplateConnectionOpts(opts))
}

type BuildStatusReason = roottmpl.BuildStatusReason

func AssignTags(ctx context.Context, targetName string, tags any, opts *ConnectionOpts) (*TemplateTagInfo, error) {
	return roottmpl.AssignTags(ctx, targetName, tags, toTemplateConnectionOpts(opts))
}

func RemoveTags(ctx context.Context, name string, tags any, opts *ConnectionOpts) error {
	return roottmpl.RemoveTags(ctx, name, tags, toTemplateConnectionOpts(opts))
}

func GetTags(ctx context.Context, templateID string, opts *ConnectionOpts) ([]TemplateTag, error) {
	return roottmpl.GetTags(ctx, templateID, toTemplateConnectionOpts(opts))
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

func ListTemplates(ctx context.Context, opts *ListTemplatesOpts) ([]TemplateInfo, error) {
	return roottmpl.ListTemplates(ctx, opts)
}
