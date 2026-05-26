package template

import (
	"time"

	"github.com/e2b-dev/e2b-go-sdk/api"
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
	FromImage(baseImage string, credentials *RegistryCredentials) *TemplateBase
	FromTemplate(template string) *TemplateBase
	FromDockerfile(dockerfileContentOrPath string) *TemplateBase
	FromAWSRegistry(image string, credentials *AWSRegistryCredentials) *TemplateBase
	FromGCPRegistry(image string, credentials *GCPRegistryCredentials) *TemplateBase
	SkipCache() *TemplateBase
}

type TemplateBuilder interface {
	Copy(src, dest string, opts *struct {
		User            string
		Mode            string
		ForceUpload     bool
		ResolveSymlinks bool
	}) *TemplateBase
	CopyItems(items []CopyItem) *TemplateBase
	Remove(path string, opts *struct {
		Force bool
	}) *TemplateBase
	Rename(src, dest string, opts *struct {
		User            string
		Mode            string
		ForceUpload     bool
		ResolveSymlinks bool
	}) *TemplateBase
	MakeDir(path string, opts *struct {
		User            string
		Mode            string
		ForceUpload     bool
		ResolveSymlinks bool
	}) *TemplateBase
	MakeSymlink(src, dest string, opts *struct {
		User            string
		Mode            string
		ForceUpload     bool
		ResolveSymlinks bool
	}) *TemplateBase
	RunCmd(command string, opts *struct {
		Force bool
		User  string
	}) *TemplateBase
	SetWorkdir(workdir string) *TemplateBase
	SetUser(user string) *TemplateBase
	PipInstall(packages []string, opts *struct {
		Force bool
	}) *TemplateBase
	NpmInstall(packages []string, opts *struct {
		Force bool
	}) *TemplateBase
	BunInstall(packages []string, opts *struct {
		Force bool
	}) *TemplateBase
	AptInstall(packages []string, opts *struct {
		Force bool
	}) *TemplateBase
	AddMcpServer(servers ...string) *TemplateBase
	GitClone(url, path string, opts *struct {
		User            string
		Mode            string
		ForceUpload     bool
		ResolveSymlinks bool
	}) *TemplateBase
	SetEnvs(envs map[string]string) *TemplateBase
	SkipCache() *TemplateBase
	SetStartCmd(startCommand string, readyCommand interface{}) *TemplateBase
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

type BuildOptions struct {
	BasicBuildOptions
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	Headers          map[string]string
	RequestTimeoutMs *int
	Logger           api.Logger
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
	Debug            bool
	LogsOffset       int
	RequestTimeoutMs *int
	Headers          map[string]string
	Logger           api.Logger
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
	Src             string
	Dest            string
	ForceUpload     bool
	User            string
	Mode            string
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
	ServiceAccountJSON string
}
