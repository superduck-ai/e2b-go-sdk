package template

type TemplateOptions struct {
	FileContextPath    string
	FileIgnorePatterns []string
}

type BasicBuildOptions struct {
	Tags        []string
	CpuCount    int
	MemoryMB    int
	SkipCache   bool
	OnBuildLogs BuildLogger
}

type BuildOptions struct {
	BasicBuildOptions
	ApiKey      string
	AccessToken string
	Domain      string
	ApiUrl      string
	Debug       bool
	Headers     map[string]string
}

type BuildInfo struct {
	Name       string
	Tags       []string
	TemplateID string
	BuildID    string
}

type GetBuildStatusOptions struct {
	ApiKey      string
	AccessToken string
	Domain      string
	ApiUrl      string
	Debug       bool
	LogsOffset  int
}

type TemplateBuildStatus string

const (
	BuildStatusBuilding TemplateBuildStatus = "building"
	BuildStatusWaiting  TemplateBuildStatus = "waiting"
	BuildStatusReady    TemplateBuildStatus = "ready"
	BuildStatusError    TemplateBuildStatus = "error"
)

type TemplateBuildStatusResponse struct {
	BuildID    string
	TemplateID string
	Status     TemplateBuildStatus
	Logs       string
	Reason     string
}

type TemplateTag struct {
	Tag       string
	BuildID   string
	CreatedAt string
}

type TemplateTagInfo struct {
	BuildID string
	Tags    []string
}

type InstructionType string

const (
	InstructionCopy    InstructionType = "copy"
	InstructionEnv     InstructionType = "env"
	InstructionRun     InstructionType = "run"
	InstructionWorkdir InstructionType = "workdir"
	InstructionUser    InstructionType = "user"
)

type Instruction struct {
	Type            InstructionType
	Args            string
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
	AccessKeyID    string
	SecretAccessKey string
	SessionToken   string
	Region         string
}

type GCPRegistryCredentials struct {
	ServiceAccountJSON string
}
