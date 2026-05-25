package template

import (
	"context"
	"fmt"

	"github.com/e2b-dev/e2b-go-sdk/api"
)

type TemplateBase struct {
	instructions []Instruction
	options      *TemplateOptions
}

func NewTemplate(options *TemplateOptions) *TemplateBase {
	return &TemplateBase{options: options}
}

// From* methods

func (t *TemplateBase) FromImage(baseImage string, credentials *RegistryCredentials) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: "FROM " + baseImage})
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
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: "FROM template:" + templateName})
	return t
}

func (t *TemplateBase) FromDockerfile(content string) *TemplateBase {
	ParseDockerfile(content, t)
	return t
}

func (t *TemplateBase) FromAWSRegistry(image string, credentials *AWSRegistryCredentials) *TemplateBase {
	return t.FromImage(image, nil)
}

func (t *TemplateBase) FromGCPRegistry(image string, credentials *GCPRegistryCredentials) *TemplateBase {
	return t.FromImage(image, nil)
}

// Builder methods

type CopyOpts struct {
	User            string
	Mode            string
	ForceUpload     bool
	ResolveSymlinks bool
}

func (t *TemplateBase) Copy(src, dest string, opts *CopyOpts) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionCopy, Args: src + " " + dest})
	return t
}

func (t *TemplateBase) CopyItems(items []CopyItem) *TemplateBase {
	for _, item := range items {
		t.Copy(item.Src, item.Dest, nil)
	}
	return t
}

type RemoveOpts struct {
	Force bool
}

func (t *TemplateBase) Remove(path string, opts *RemoveOpts) *TemplateBase {
	force := ""
	if opts != nil && opts.Force {
		force = "-f "
	}
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: "rm -r " + force + path})
	return t
}

func (t *TemplateBase) Rename(src, dest string, opts *CopyOpts) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: "mv " + src + " " + dest})
	return t
}

func (t *TemplateBase) MakeDir(path string, opts *CopyOpts) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: "mkdir -p " + path})
	return t
}

func (t *TemplateBase) MakeSymlink(src, dest string, opts *CopyOpts) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: "ln -s " + src + " " + dest})
	return t
}

type RunCmdOpts struct {
	Force bool
}

func (t *TemplateBase) RunCmd(command string, opts *RunCmdOpts) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: command})
	return t
}

func (t *TemplateBase) SetWorkdir(workdir string) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionWorkdir, Args: workdir})
	return t
}

func (t *TemplateBase) SetUser(user string) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionUser, Args: user})
	return t
}

type InstallOpts struct {
	Force bool
}

func (t *TemplateBase) PipInstall(packages []string, opts *InstallOpts) *TemplateBase {
	if len(packages) == 0 {
		return t.RunCmd("pip install -r requirements.txt", nil)
	}
	return t.RunCmd("pip install "+joinPackages(packages), nil)
}

func (t *TemplateBase) NpmInstall(packages []string, opts *InstallOpts) *TemplateBase {
	if len(packages) == 0 {
		return t.RunCmd("npm install", nil)
	}
	return t.RunCmd("npm install "+joinPackages(packages), nil)
}

func (t *TemplateBase) BunInstall(packages []string, opts *InstallOpts) *TemplateBase {
	if len(packages) == 0 {
		return t.RunCmd("bun install", nil)
	}
	return t.RunCmd("bun add "+joinPackages(packages), nil)
}

func (t *TemplateBase) AptInstall(packages []string, opts *InstallOpts) *TemplateBase {
	return t.RunCmd("apt-get update && apt-get install -y "+joinPackages(packages), nil)
}

func (t *TemplateBase) GitClone(url, path string, opts *CopyOpts) *TemplateBase {
	cmd := "git clone " + url
	if path != "" {
		cmd += " " + path
	}
	return t.RunCmd(cmd, nil)
}

func (t *TemplateBase) SetEnvs(envs map[string]string) *TemplateBase {
	for k, v := range envs {
		t.instructions = append(t.instructions, Instruction{Type: InstructionEnv, Args: k + "=" + v})
	}
	return t
}

func (t *TemplateBase) SkipCache() *TemplateBase {
	if len(t.instructions) > 0 {
		t.instructions[len(t.instructions)-1].Force = true
	}
	return t
}

func (t *TemplateBase) SetStartCmd(startCommand string, readyCommand *ReadyCmd) *TemplateBase {
	t.instructions = append(t.instructions, Instruction{Type: InstructionRun, Args: "CMD " + startCommand})
	return t
}

func (t *TemplateBase) SetReadyCmd(readyCommand *ReadyCmd) *TemplateBase {
	return t
}

func (t *TemplateBase) GetInstructions() []Instruction {
	return t.instructions
}

// newApiClientFromBuildOptions creates an ApiClient from BuildOptions.
func newApiClientFromBuildOptions(opts *BuildOptions) (*api.ApiClient, error) {
	config := &api.ClientConfig{
		ApiKey:      opts.ApiKey,
		AccessToken: opts.AccessToken,
		Domain:      opts.Domain,
		ApiUrl:      opts.ApiUrl,
		Headers:     opts.Headers,
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

	buildInfo, err := RequestBuild(ctx, client, name, opts.Tags, cpuCount, memoryMB)
	if err != nil {
		return nil, err
	}

	instructions := template.GetInstructions()

	err = TriggerBuild(ctx, client, buildInfo.TemplateID, buildInfo.BuildID, instructions)
	if err != nil {
		return nil, err
	}

	logger := opts.OnBuildLogs
	if logger == nil {
		logger = DefaultBuildLogger()
	}

	_, err = WaitForBuildFinish(ctx, client, buildInfo.TemplateID, buildInfo.BuildID, logger)
	if err != nil {
		return nil, err
	}

	// Assign tags if specified
	if len(opts.Tags) > 0 {
		if err := AssignTags(ctx, client, name, opts.Tags); err != nil {
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

	buildInfo, err := RequestBuild(ctx, client, name, opts.Tags, cpuCount, memoryMB)
	if err != nil {
		return nil, err
	}

	instructions := template.GetInstructions()

	err = TriggerBuild(ctx, client, buildInfo.TemplateID, buildInfo.BuildID, instructions)
	if err != nil {
		return nil, err
	}

	return buildInfo, nil
}

// GetBuildStatusByData retrieves the build status for a given template and build.
func GetBuildStatusByData(ctx context.Context, templateID, buildID string, opts *GetBuildStatusOptions) (*TemplateBuildStatusResponse, error) {
	if opts == nil {
		opts = &GetBuildStatusOptions{}
	}

	client, err := newApiClientFromStatusOptions(opts)
	if err != nil {
		return nil, err
	}

	return GetBuildStatusFromAPI(ctx, client, templateID, buildID, opts.LogsOffset)
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

	return CheckAliasExists(ctx, client, name)
}

// AssignTemplateTags assigns tags to a template.
func AssignTemplateTags(ctx context.Context, targetName string, tags []string, opts *BuildOptions) error {
	if opts == nil {
		opts = &BuildOptions{}
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return err
	}

	return AssignTags(ctx, client, targetName, tags)
}

// RemoveTemplateTags removes tags from a template.
func RemoveTemplateTags(ctx context.Context, name string, tags []string, opts *BuildOptions) error {
	if opts == nil {
		opts = &BuildOptions{}
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return err
	}

	return RemoveTags(ctx, client, name, tags)
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

	return GetTemplateTags(ctx, client, templateID)
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
