package template

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/e2b-dev/e2b-go-sdk/api"
)

type TemplateBase struct {
	baseImage      string
	baseTemplate   string
	registryConfig *registryConfigPayload
	startCmd       string
	readyCmd       string
	force          bool
	forceNextLayer bool
	instructions   []Instruction
	options        *TemplateOptions
}

func Template(options *TemplateOptions) *TemplateBase {
	return newTemplate(options)
}

func newTemplate(options *TemplateOptions) *TemplateBase {
	return &TemplateBase{options: options}
}

// From* methods

func (t *TemplateBase) FromImage(baseImage string, credentials *RegistryCredentials) *TemplateBase {
	t.baseImage = baseImage
	t.baseTemplate = ""
	if credentials != nil {
		t.registryConfig = &registryConfigPayload{
			Type:     "registry",
			Username: credentials.Username,
			Password: credentials.Password,
		}
	} else {
		t.registryConfig = nil
	}
	if t.forceNextLayer {
		t.force = true
	}
	return t
}

func (t *TemplateBase) FromDebianImage(variant ...string) *TemplateBase {
	v := "bookworm-slim"
	if len(variant) > 0 {
		v = variant[0]
	}
	return t.FromImage("debian:"+v, nil)
}

func (t *TemplateBase) FromUbuntuImage(variant ...string) *TemplateBase {
	v := "24.04"
	if len(variant) > 0 {
		v = variant[0]
	}
	return t.FromImage("ubuntu:"+v, nil)
}

func (t *TemplateBase) FromPythonImage(version ...string) *TemplateBase {
	v := "3.12"
	if len(version) > 0 {
		v = version[0]
	}
	return t.FromImage("python:"+v, nil)
}

func (t *TemplateBase) FromNodeImage(variant ...string) *TemplateBase {
	v := "22"
	if len(variant) > 0 {
		v = variant[0]
	}
	return t.FromImage("node:"+v, nil)
}

func (t *TemplateBase) FromBunImage(variant ...string) *TemplateBase {
	v := "latest"
	if len(variant) > 0 {
		v = variant[0]
	}
	return t.FromImage("oven/bun:"+v, nil)
}

func (t *TemplateBase) FromBaseImage() *TemplateBase {
	return t.FromImage("e2bdev/base", nil)
}

func (t *TemplateBase) FromTemplate(templateName string) *TemplateBase {
	t.baseTemplate = templateName
	t.baseImage = ""
	t.registryConfig = nil
	if t.forceNextLayer {
		t.force = true
	}
	return t
}

func (t *TemplateBase) FromDockerfile(content string) *TemplateBase {
	if fileContent, err := t.resolveDockerfileInput(content); err == nil {
		content = fileContent
	}
	parseDockerfile(content, t)
	return t
}

func (t *TemplateBase) FromAWSRegistry(image string, credentials *AWSRegistryCredentials) *TemplateBase {
	t.baseImage = image
	t.baseTemplate = ""
	if credentials != nil {
		t.registryConfig = &registryConfigPayload{
			Type:               "aws",
			AwsAccessKeyID:     credentials.AccessKeyID,
			AwsSecretAccessKey: credentials.SecretAccessKey,
			AwsSessionToken:    credentials.SessionToken,
			AwsRegion:          credentials.Region,
		}
	} else {
		t.registryConfig = nil
	}
	if t.forceNextLayer {
		t.force = true
	}
	return t
}

func (t *TemplateBase) FromGCPRegistry(image string, credentials *GCPRegistryCredentials) *TemplateBase {
	t.baseImage = image
	t.baseTemplate = ""
	if credentials != nil {
		t.registryConfig = &registryConfigPayload{
			Type:               "gcp",
			ServiceAccountJSON: credentials.ServiceAccountJSON,
		}
	} else {
		t.registryConfig = nil
	}
	if t.forceNextLayer {
		t.force = true
	}
	return t
}

// Builder methods

func (t *TemplateBase) Copy(src, dest string, opts *struct {
	User            string
	Mode            string
	ForceUpload     bool
	ResolveSymlinks bool
}) *TemplateBase {
	args := []string{src, dest, "", ""}
	forceUpload := false
	resolveSymlinks := false
	if opts != nil {
		args[2] = opts.User
		args[3] = opts.Mode
		forceUpload = opts.ForceUpload
		resolveSymlinks = opts.ResolveSymlinks
	}
	t.instructions = append(t.instructions, Instruction{
		Type:            InstructionCopy,
		Args:            args,
		Force:           forceUpload || t.forceNextLayer,
		ForceUpload:     forceUpload,
		ResolveSymlinks: resolveSymlinks,
	})
	return t
}

func (t *TemplateBase) CopyItems(items []CopyItem) *TemplateBase {
	for _, item := range items {
		t.Copy(item.Src, item.Dest, nil)
	}
	return t
}

func (t *TemplateBase) Remove(path string, opts *struct {
	Force bool
}) *TemplateBase {
	force := ""
	if opts != nil && opts.Force {
		force = "-f "
	}
	t.instructions = append(t.instructions, Instruction{
		Type:  InstructionRun,
		Args:  []string{"rm -r " + force + path},
		Force: t.forceNextLayer,
	})
	return t
}

func (t *TemplateBase) Rename(src, dest string, opts *struct {
	User            string
	Mode            string
	ForceUpload     bool
	ResolveSymlinks bool
}) *TemplateBase {
	args := []string{"mv " + src + " " + dest}
	if opts != nil && opts.User != "" {
		args = append(args, opts.User)
	}
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: args, Force: t.forceNextLayer})
	return t
}

func (t *TemplateBase) MakeDir(path string, opts *struct {
	User            string
	Mode            string
	ForceUpload     bool
	ResolveSymlinks bool
}) *TemplateBase {
	cmd := "mkdir -p " + path
	if opts != nil && opts.Mode != "" {
		cmd = "mkdir -p -m " + opts.Mode + " " + path
	}
	args := []string{cmd}
	if opts != nil && opts.User != "" {
		args = append(args, opts.User)
	}
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: args, Force: t.forceNextLayer})
	return t
}

func (t *TemplateBase) MakeSymlink(src, dest string, opts *struct {
	User            string
	Mode            string
	ForceUpload     bool
	ResolveSymlinks bool
}) *TemplateBase {
	cmd := "ln -s " + src + " " + dest
	args := []string{cmd}
	if opts != nil && opts.User != "" {
		args = append(args, opts.User)
	}
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: args, Force: t.forceNextLayer})
	return t
}

func (t *TemplateBase) RunCmd(command string, opts *struct {
	Force bool
	User  string
}) *TemplateBase {
	args := []string{command}
	force := t.forceNextLayer
	if opts != nil {
		force = force || opts.Force
		if opts.User != "" {
			args = append(args, opts.User)
		}
	}
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: args, Force: force})
	return t
}

func (t *TemplateBase) SetWorkdir(workdir string) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionWorkdir, Args: []string{workdir}, Force: t.forceNextLayer})
	return t
}

func (t *TemplateBase) SetUser(user string) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionUser, Args: []string{user}, Force: t.forceNextLayer})
	return t
}

func (t *TemplateBase) PipInstall(packages []string, opts *struct {
	Force bool
}) *TemplateBase {
	if len(packages) == 0 {
		return t.RunCmd("pip install -r requirements.txt", nil)
	}
	return t.RunCmd("pip install "+joinPackages(packages), nil)
}

func (t *TemplateBase) NpmInstall(packages []string, opts *struct {
	Force bool
}) *TemplateBase {
	if len(packages) == 0 {
		return t.RunCmd("npm install", nil)
	}
	return t.RunCmd("npm install "+joinPackages(packages), nil)
}

func (t *TemplateBase) BunInstall(packages []string, opts *struct {
	Force bool
}) *TemplateBase {
	if len(packages) == 0 {
		return t.RunCmd("bun install", nil)
	}
	return t.RunCmd("bun add "+joinPackages(packages), nil)
}

func (t *TemplateBase) AptInstall(packages []string, opts *struct {
	Force bool
}) *TemplateBase {
	return t.RunCmd("apt-get update && apt-get install -y "+joinPackages(packages), nil)
}

func (t *TemplateBase) AddMcpServer(servers ...string) *TemplateBase {
	if len(servers) == 0 {
		return t
	}
	return t.RunCmd("mcp-gateway pull "+strings.Join(servers, " "), &struct {
		Force bool
		User  string
	}{User: "root"})
}

func (t *TemplateBase) GitClone(url, path string, opts *struct {
	User            string
	Mode            string
	ForceUpload     bool
	ResolveSymlinks bool
}) *TemplateBase {
	cmd := "git clone " + url
	if path != "" {
		cmd += " " + path
	}
	runOpts := &struct {
		Force bool
		User  string
	}{}
	if opts != nil {
		runOpts.User = opts.User
	}
	return t.RunCmd(cmd, runOpts)
}

func (t *TemplateBase) SetEnvs(envs map[string]string) *TemplateBase {
	if len(envs) == 0 {
		return t
	}
	for k, v := range envs {
		t.instructions = append(t.instructions, Instruction{Type: InstructionEnv, Args: []string{k, v}, Force: t.forceNextLayer})
	}
	return t
}

func (t *TemplateBase) SkipCache() *TemplateBase {
	t.forceNextLayer = true
	return t
}

func (t *TemplateBase) SetStartCmd(startCommand string, readyCommand interface{}) *TemplateBase {
	t.startCmd = startCommand
	if cmd, ok := resolveReadyCommand(readyCommand); ok {
		t.readyCmd = cmd
	}
	return t
}

func (t *TemplateBase) SetReadyCmd(readyCommand interface{}) *TemplateBase {
	if cmd, ok := resolveReadyCommand(readyCommand); ok {
		t.readyCmd = cmd
	}
	return t
}

func (t *TemplateBase) BetaDevContainerPrebuild(devcontainerDirectory string) *TemplateBase {
	return t.RunCmd("devcontainer build --workspace-folder "+devcontainerDirectory, &struct {
		Force bool
		User  string
	}{User: "root"})
}

func (t *TemplateBase) BetaSetDevContainerStart(devcontainerDirectory string) *TemplateBase {
	return t.SetStartCmd(
		"sudo devcontainer up --workspace-folder "+devcontainerDirectory+
			" && sudo /prepare-exec.sh "+devcontainerDirectory+
			" | sudo tee /devcontainer.sh > /dev/null && sudo chmod +x /devcontainer.sh && sudo touch /devcontainer.up",
		WaitForFile("/devcontainer.up"),
	)
}

func (t *TemplateBase) instructionsList() []Instruction {
	return t.instructions
}

func (t *TemplateBase) fileContextPath() string {
	if t.options != nil && t.options.FileContextPath != "" {
		return t.options.FileContextPath
	}
	return "."
}

func (t *TemplateBase) fileIgnorePatterns() []string {
	var patterns []string
	if t.options != nil && len(t.options.FileIgnorePatterns) > 0 {
		patterns = append(patterns, t.options.FileIgnorePatterns...)
	}
	patterns = append(patterns, readDockerignore(t.fileContextPath())...)
	return patterns
}

func ToJSON(template *TemplateBase, computeHashes ...bool) (string, error) {
	if template == nil {
		template = Template(nil)
	}

	serialized := template.serialize()
	if len(computeHashes) > 0 && computeHashes[0] {
		steps, err := template.instructionsWithHashes()
		if err != nil {
			return "", err
		}
		serialized = template.serializeWithSteps(steps)
	}

	data, err := json.MarshalIndent(serialized, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func ToDockerfile(template *TemplateBase) (string, error) {
	if template == nil {
		return "", fmt.Errorf("No base image specified for template")
	}
	if template.baseTemplate != "" {
		return "", fmt.Errorf("Cannot convert template built from another template to Dockerfile. Templates based on other templates can only be built using the E2B API.")
	}
	if template.baseImage == "" {
		return "", fmt.Errorf("No base image specified for template")
	}

	var dockerfile strings.Builder
	dockerfile.WriteString("FROM ")
	dockerfile.WriteString(template.baseImage)
	dockerfile.WriteByte('\n')

	for _, instruction := range template.instructionsList() {
		switch instruction.Type {
		case InstructionRun:
			if len(instruction.Args) == 0 {
				continue
			}
			dockerfile.WriteString("RUN ")
			dockerfile.WriteString(instruction.Args[0])
			dockerfile.WriteByte('\n')
		case InstructionCopy:
			if len(instruction.Args) >= 2 {
				dockerfile.WriteString("COPY ")
				dockerfile.WriteString(instruction.Args[0])
				dockerfile.WriteByte(' ')
				dockerfile.WriteString(instruction.Args[1])
				dockerfile.WriteByte('\n')
			}
		case InstructionEnv:
			if len(instruction.Args) > 0 {
				pairs := make([]string, 0, len(instruction.Args)/2)
				for i := 0; i+1 < len(instruction.Args); i += 2 {
					pairs = append(pairs, instruction.Args[i]+"="+instruction.Args[i+1])
				}
				if len(pairs) > 0 {
					dockerfile.WriteString("ENV ")
					dockerfile.WriteString(strings.Join(pairs, " "))
					dockerfile.WriteByte('\n')
				}
			}
		case InstructionWorkdir:
			if len(instruction.Args) > 0 {
				dockerfile.WriteString("WORKDIR ")
				dockerfile.WriteString(instruction.Args[0])
				dockerfile.WriteByte('\n')
			}
		case InstructionUser:
			if len(instruction.Args) > 0 {
				dockerfile.WriteString("USER ")
				dockerfile.WriteString(instruction.Args[0])
				dockerfile.WriteByte('\n')
			}
		default:
			if len(instruction.Args) > 0 {
				dockerfile.WriteString(string(instruction.Type))
				dockerfile.WriteByte(' ')
				dockerfile.WriteString(strings.Join(instruction.Args, " "))
				dockerfile.WriteByte('\n')
			}
		}
	}
	if template.startCmd != "" {
		dockerfile.WriteString("ENTRYPOINT ")
		dockerfile.WriteString(template.startCmd)
		dockerfile.WriteByte('\n')
	}

	return dockerfile.String(), nil
}

func (t *TemplateBase) serialize() triggerBuildTemplate {
	return t.serializeWithSteps(t.instructionsList())
}

func (t *TemplateBase) serializeWithSteps(steps []Instruction) triggerBuildTemplate {
	payloads := make([]instructionPayload, len(steps))
	for i, inst := range steps {
		payloads[i] = instructionPayload{
			Type:            inst.Type,
			Args:            inst.Args,
			Force:           inst.Force,
			ForceUpload:     inst.ForceUpload,
			FilesHash:       inst.FilesHash,
			ResolveSymlinks: inst.ResolveSymlinks,
		}
	}

	return triggerBuildTemplate{
		StartCmd:          t.startCmd,
		ReadyCmd:          t.readyCmd,
		Steps:             payloads,
		Force:             t.force,
		FromImage:         t.baseImage,
		FromTemplate:      t.baseTemplate,
		FromImageRegistry: t.registryConfig,
	}
}

func (t *TemplateBase) instructionsWithHashes() ([]Instruction, error) {
	steps := make([]Instruction, len(t.instructions))
	copy(steps, t.instructions)

	for i, instruction := range steps {
		if instruction.Type != InstructionCopy {
			continue
		}
		if len(instruction.Args) < 2 {
			return nil, fmt.Errorf("Source path and destination path are required")
		}

		resolve := instruction.ResolveSymlinks
		if !instruction.ResolveSymlinks {
			resolve = resolveSymlinks
		}

		filesHash, err := calculateFilesHash(
			instruction.Args[0],
			instruction.Args[1],
			t.fileContextPath(),
			t.fileIgnorePatterns(),
			resolve,
		)
		if err != nil {
			return nil, err
		}
		steps[i].FilesHash = filesHash
	}

	return steps, nil
}

func (t *TemplateBase) uploadCopySources(ctx context.Context, client *api.ApiClient, templateID string, steps []Instruction) error {
	for _, instruction := range steps {
		if instruction.Type != InstructionCopy {
			continue
		}
		if len(instruction.Args) < 1 || instruction.FilesHash == "" {
			return fmt.Errorf("Source path and files hash are required")
		}

		link, err := getFileUploadLink(ctx, client, templateID, instruction.FilesHash)
		if err != nil {
			return err
		}
		if link == nil || link.URL == "" {
			continue
		}
		if !instruction.ForceUpload && link.Present {
			continue
		}

		resolve := instruction.ResolveSymlinks
		if !instruction.ResolveSymlinks {
			resolve = resolveSymlinks
		}
		archive, err := tarFileBytes(
			instruction.Args[0],
			t.fileContextPath(),
			t.fileIgnorePatterns(),
			resolve,
		)
		if err != nil {
			return err
		}
		if err := uploadFile(ctx, link.URL, archive); err != nil {
			return err
		}
	}
	return nil
}

func (t *TemplateBase) resolveDockerfileInput(content string) (string, error) {
	candidates := []string{content}
	if t.options != nil && t.options.FileContextPath != "" {
		candidates = append(candidates, filepathJoinIfRelative(t.options.FileContextPath, content))
	}

	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("dockerfile path not found")
}

func resolveReadyCommand(readyCommand interface{}) (string, bool) {
	switch value := readyCommand.(type) {
	case nil:
		return "", false
	case string:
		return value, true
	case *ReadyCmd:
		if value == nil {
			return "", false
		}
		return value.Command, true
	default:
		return "", false
	}
}

func filepathJoinIfRelative(base, value string) string {
	if base == "" || value == "" {
		return value
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return base + string(os.PathSeparator) + value
}

// newApiClientFromBuildOptions creates an ApiClient from BuildOptions.
func newApiClientFromBuildOptions(opts *BuildOptions) (*api.ApiClient, error) {
	config := &api.ClientConfig{
		ApiKey:      opts.ApiKey,
		AccessToken: opts.AccessToken,
		Domain:      opts.Domain,
		ApiUrl:      opts.ApiUrl,
		Headers:     opts.Headers,
		Logger:      opts.Logger,
	}
	if opts.RequestTimeoutMs != nil {
		config.RequestTimeoutMs = *opts.RequestTimeoutMs
	}
	if config.Domain == "" {
		config.Domain = "e2b.dev"
	}
	return api.NewApiClient(config, api.WithRequireApiKey())
}

// newApiClientFromStatusOptions creates an ApiClient from GetBuildStatusOptions.
func newApiClientFromStatusOptions(opts *GetBuildStatusOptions) (*api.ApiClient, error) {
	config := &api.ClientConfig{
		ApiKey:      opts.ApiKey,
		AccessToken: opts.AccessToken,
		Domain:      opts.Domain,
		ApiUrl:      opts.ApiUrl,
		Headers:     opts.Headers,
		Logger:      opts.Logger,
	}
	if opts.RequestTimeoutMs != nil {
		config.RequestTimeoutMs = *opts.RequestTimeoutMs
	}
	if config.Domain == "" {
		config.Domain = "e2b.dev"
	}
	return api.NewApiClient(config, api.WithRequireApiKey())
}

// Static methods

// Build creates a template and waits for the build to finish.
func Build(ctx context.Context, template *TemplateBase, name string, opts *BuildOptions) (*BuildInfo, error) {
	if opts == nil {
		opts = &BuildOptions{}
	}
	if opts.Alias != "" && name == "" {
		name = opts.Alias
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return nil, err
	}

	cpuCount := opts.CpuCount
	if cpuCount == 0 {
		cpuCount = 2
	}
	memoryMB := opts.MemoryMB
	if memoryMB == 0 {
		memoryMB = 512
	}

	buildInfo, err := requestBuild(ctx, client, name, opts.Tags, cpuCount, memoryMB)
	if err != nil {
		return nil, err
	}

	steps, err := template.instructionsWithHashes()
	if err != nil {
		return nil, err
	}

	if err := template.uploadCopySources(ctx, client, buildInfo.TemplateID, steps); err != nil {
		return nil, err
	}

	err = triggerBuild(ctx, client, buildInfo.TemplateID, buildInfo.BuildID, template.serializeWithSteps(steps))
	if err != nil {
		return nil, err
	}

	logger := opts.OnBuildLogs
	if logger == nil {
		logger = DefaultBuildLogger()
	}

	_, err = waitForBuildFinish(ctx, client, buildInfo.TemplateID, buildInfo.BuildID, logger)
	if err != nil {
		return nil, err
	}

	// Assign tags if specified
	if len(opts.Tags) > 0 {
		if _, err := assignTags(ctx, client, name, opts.Tags); err != nil {
			return nil, fmt.Errorf("failed to assign tags: %w", err)
		}
	}

	return buildInfo, nil
}

// BuildInBackground creates a template and triggers a build without waiting for completion.
func BuildInBackground(ctx context.Context, template *TemplateBase, name string, opts *BuildOptions) (*BuildInfo, error) {
	if opts == nil {
		opts = &BuildOptions{}
	}
	if opts.Alias != "" && name == "" {
		name = opts.Alias
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return nil, err
	}

	cpuCount := opts.CpuCount
	if cpuCount == 0 {
		cpuCount = 2
	}
	memoryMB := opts.MemoryMB
	if memoryMB == 0 {
		memoryMB = 512
	}

	buildInfo, err := requestBuild(ctx, client, name, opts.Tags, cpuCount, memoryMB)
	if err != nil {
		return nil, err
	}

	steps, err := template.instructionsWithHashes()
	if err != nil {
		return nil, err
	}

	if err := template.uploadCopySources(ctx, client, buildInfo.TemplateID, steps); err != nil {
		return nil, err
	}

	err = triggerBuild(ctx, client, buildInfo.TemplateID, buildInfo.BuildID, template.serializeWithSteps(steps))
	if err != nil {
		return nil, err
	}

	return buildInfo, nil
}

// GetBuildStatus retrieves the build status for a given template and build.
func GetBuildStatus(ctx context.Context, templateID, buildID string, opts *GetBuildStatusOptions) (*TemplateBuildStatusResponse, error) {
	if opts == nil {
		opts = &GetBuildStatusOptions{}
	}

	client, err := newApiClientFromStatusOptions(opts)
	if err != nil {
		return nil, err
	}

	return getBuildStatusFromAPI(ctx, client, templateID, buildID, opts.LogsOffset)
}

// Exists checks whether a template with the given name exists.
func Exists(ctx context.Context, name string, opts *BuildOptions) (bool, error) {
	if opts == nil {
		opts = &BuildOptions{}
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return false, err
	}

	return checkAliasExists(ctx, client, name)
}

// AssignTags assigns tags to a template.
func AssignTags(ctx context.Context, targetName string, tags []string, opts *BuildOptions) (*TemplateTagInfo, error) {
	if opts == nil {
		opts = &BuildOptions{}
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return nil, err
	}

	return assignTags(ctx, client, targetName, tags)
}

// RemoveTags removes tags from a template.
func RemoveTags(ctx context.Context, name string, tags []string, opts *BuildOptions) error {
	if opts == nil {
		opts = &BuildOptions{}
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return err
	}

	return removeTags(ctx, client, name, tags)
}

// GetTags retrieves all tags for a template.
func GetTags(ctx context.Context, templateID string, opts *BuildOptions) ([]TemplateTag, error) {
	if opts == nil {
		opts = &BuildOptions{}
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return nil, err
	}

	return getTemplateTags(ctx, client, templateID)
}

func joinPackages(packages []string) string {
	result := ""
	for i, p := range packages {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}
