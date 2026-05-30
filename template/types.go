package template

import (
	"context"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
)

type TemplateOptions struct {
	FileContextPath    string
	FileIgnorePatterns []string
}

type McpServerName = string

type TemplateFromImage interface {
	FromDebianImage(variant ...string) *TemplateBase
	FromUbuntuImage(variant ...string) *TemplateBase
	FromPythonImage(version ...string) *TemplateBase
	FromNodeImage(variant ...string) *TemplateBase
	FromBunImage(variant ...string) *TemplateBase
	FromBaseImage() *TemplateBase
	FromImage(baseImage string, credentials ...*RegistryCredentials) *TemplateBase
	FromTemplate(template string) *TemplateBase
	FromDockerfile(dockerfileContentOrPath string) *TemplateBase
	FromAWSRegistry(image string, credentials *AWSRegistryCredentials) *TemplateBase
	FromGCPRegistry(image string, credentials *GCPRegistryCredentials) *TemplateBase
	SkipCache() *TemplateBase
}

type TemplateBuilder interface {
	Copy(src any, dest string, opts ...any) *TemplateBase
	CopyItems(items []CopyItem) *TemplateBase
	Remove(path any, opts ...any) *TemplateBase
	Rename(src, dest string, opts ...any) *TemplateBase
	MakeDir(path any, opts ...any) *TemplateBase
	MakeSymlink(src, dest string, opts ...any) *TemplateBase
	RunCmd(command any, opts ...any) *TemplateBase
	SetWorkdir(workdir string) *TemplateBase
	SetUser(user string) *TemplateBase
	PipInstall(args ...any) *TemplateBase
	NpmInstall(args ...any) *TemplateBase
	BunInstall(args ...any) *TemplateBase
	AptInstall(packages any, opts ...any) *TemplateBase
	AddMcpServer(servers ...any) *TemplateBase
	GitClone(url string, args ...any) *TemplateBase
	SetEnvs(envs map[string]string) *TemplateBase
	SkipCache() *TemplateBase
	SetStartCmd(startCommand string, readyCommand ...interface{}) *TemplateBase
	SetReadyCmd(readyCommand interface{}) *TemplateBase
	BetaDevContainerPrebuild(devcontainerDirectory string) *TemplateBase
	BetaSetDevContainerStart(devcontainerDirectory string) *TemplateBase
}

type TemplateFinal interface{}

type TemplateClass interface{}

type BasicBuildOptions struct {
	Alias       string
	Tags        []string
	CpuCount    int
	MemoryMB    int
	SkipCache   bool
	OnBuildLogs BuildLogger
}

type ConnectionOpts struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            *bool
	Signal           context.Context
	RequestTimeoutMs *int
	Headers          map[string]string
	Logger           api.Logger
	Proxy            string
}

type BuildOptions struct {
	BasicBuildOptions
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            *bool
	Signal           context.Context
	Headers          map[string]string
	RequestTimeoutMs *int
	Logger           api.Logger
	Proxy            string
}

type BuildInfo struct {
	Alias      string
	Name       string
	Tags       []string
	TemplateID string
	BuildID    string
}

type GetBuildStatusOptions struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            *bool
	Signal           context.Context
	LogsOffset       int
	RequestTimeoutMs *int
	Headers          map[string]string
	Logger           api.Logger
	Proxy            string
}

type TemplateBuildStatus string

const (
	BuildStatusBuilding TemplateBuildStatus = "building"
	BuildStatusWaiting  TemplateBuildStatus = "waiting"
	BuildStatusReady    TemplateBuildStatus = "ready"
	BuildStatusError    TemplateBuildStatus = "error"
)

type BuildStatusReason struct {
	Message    string
	Step       string
	LogEntries []LogEntry
}

type TemplateBuildStatusResponse struct {
	BuildID    string
	TemplateID string
	Status     TemplateBuildStatus
	LogEntries []LogEntry
	Logs       []string
	Reason     *BuildStatusReason
}

type TemplateTag struct {
	Tag       string
	BuildID   string
	CreatedAt time.Time
}

type TemplateTagInfo struct {
	BuildID string
	Tags    []string
}

type InstructionType string

const (
	InstructionCopy    InstructionType = "COPY"
	InstructionEnv     InstructionType = "ENV"
	InstructionRun     InstructionType = "RUN"
	InstructionWorkdir InstructionType = "WORKDIR"
	InstructionUser    InstructionType = "USER"
)

type Instruction struct {
	Type            InstructionType
	Args            []string
	Force           bool
	ForceUpload     bool
	FilesHash       string
	ResolveSymlinks bool
}

type CopyItem struct {
	Src             any
	Dest            string
	ForceUpload     bool
	User            string
	Mode            int
	ResolveSymlinks bool
}

type RegistryCredentials struct {
	Username string
	Password string
}

type AWSRegistryCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
}

type GCPRegistryCredentials struct {
	ServiceAccountJSON any
}
