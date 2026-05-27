package template

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
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
	return &TemplateBase{baseImage: "e2bdev/base", options: options}
}

// From* methods

func (t *TemplateBase) FromImage(baseImage string, credentials ...*RegistryCredentials) *TemplateBase {
	t.baseImage = baseImage
	t.baseTemplate = ""
	var creds *RegistryCredentials
	if len(credentials) > 0 {
		creds = credentials[0]
	}
	if creds != nil {
		t.registryConfig = &registryConfigPayload{
			Type:     "registry",
			Username: creds.Username,
			Password: creds.Password,
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
	v := "stable"
	if len(variant) > 0 {
		v = variant[0]
	}
	return t.FromImage("debian:"+v, nil)
}

func (t *TemplateBase) FromUbuntuImage(variant ...string) *TemplateBase {
	v := "latest"
	if len(variant) > 0 {
		v = variant[0]
	}
	return t.FromImage("ubuntu:"+v, nil)
}

func (t *TemplateBase) FromPythonImage(version ...string) *TemplateBase {
	v := "3"
	if len(version) > 0 {
		v = version[0]
	}
	return t.FromImage("python:"+v, nil)
}

func (t *TemplateBase) FromNodeImage(variant ...string) *TemplateBase {
	v := "lts"
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
			ServiceAccountJSON: readGCPServiceAccountJSON(t.fileContextPath(), credentials.ServiceAccountJSON),
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

func (t *TemplateBase) Copy(src any, dest string, opts ...any) *TemplateBase {
	opt := firstOpt(opts)
	forceUpload := optionBool(opt, "ForceUpload", "forceUpload")
	resolveSymlinks := optionBool(opt, "ResolveSymlinks", "resolveSymlinks")
	user := optionString(opt, "User", "user")
	mode := optionMode(opt)
	for _, source := range stringList(src) {
		args := []string{source, dest, user, mode}
		t.instructions = append(t.instructions, Instruction{
			Type:            InstructionCopy,
			Args:            args,
			Force:           forceUpload || t.forceNextLayer,
			ForceUpload:     forceUpload,
			ResolveSymlinks: resolveSymlinks,
		})
	}
	return t
}

func (t *TemplateBase) CopyItems(items []CopyItem) *TemplateBase {
	for _, item := range items {
		t.Copy(item.Src, item.Dest, &struct {
			ForceUpload     bool
			User            string
			Mode            int
			ResolveSymlinks bool
		}{
			ForceUpload:     item.ForceUpload,
			User:            item.User,
			Mode:            item.Mode,
			ResolveSymlinks: item.ResolveSymlinks,
		})
	}
	return t
}

func (t *TemplateBase) Remove(path any, opts ...any) *TemplateBase {
	opt := firstOpt(opts)
	args := []string{"rm"}
	if optionBool(opt, "Recursive", "recursive") {
		args = append(args, "-r")
	}
	if optionBool(opt, "Force", "force") {
		args = append(args, "-f")
	}
	args = append(args, stringList(path)...)
	return t.RunCmd(strings.Join(args, " "), &struct {
		User string
	}{User: optionString(opt, "User", "user")})
}

func (t *TemplateBase) Rename(src, dest string, opts ...any) *TemplateBase {
	opt := firstOpt(opts)
	args := []string{"mv", src, dest}
	if optionBool(opt, "Force", "force") {
		args = append(args, "-f")
	}
	return t.RunCmd(strings.Join(args, " "), &struct {
		User string
	}{User: optionString(opt, "User", "user")})
}

func (t *TemplateBase) MakeDir(path any, opts ...any) *TemplateBase {
	opt := firstOpt(opts)
	args := []string{"mkdir", "-p"}
	if mode := optionMode(opt); mode != "" {
		args = append(args, "-m "+mode)
	}
	args = append(args, stringList(path)...)
	return t.RunCmd(strings.Join(args, " "), &struct {
		User string
	}{User: optionString(opt, "User", "user")})
}

func (t *TemplateBase) MakeSymlink(src, dest string, opts ...any) *TemplateBase {
	opt := firstOpt(opts)
	args := []string{"ln", "-s"}
	if optionBool(opt, "Force", "force") {
		args = append(args, "-f")
	}
	args = append(args, src, dest)
	return t.RunCmd(strings.Join(args, " "), &struct {
		User string
	}{User: optionString(opt, "User", "user")})
}

func (t *TemplateBase) RunCmd(command any, opts ...any) *TemplateBase {
	opt := firstOpt(opts)
	args := []string{strings.Join(stringList(command), " && ")}
	force := t.forceNextLayer || optionBool(opt, "Force", "force")
	if user := optionString(opt, "User", "user"); user != "" {
		args = append(args, user)
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

func (t *TemplateBase) PipInstall(values ...any) *TemplateBase {
	packages, opts := installArgs(values)
	global := optionBoolDefault(opts, true, "G", "g")
	args := []string{"pip", "install"}
	if !global {
		args = append(args, "--user")
	}
	pkgs := stringList(packages)
	if len(pkgs) == 0 {
		pkgs = []string{"."}
	}
	args = append(args, pkgs...)
	runOpts := &struct {
		User string
	}{}
	if global {
		runOpts.User = "root"
	}
	return t.RunCmd(strings.Join(args, " "), runOpts)
}

func (t *TemplateBase) NpmInstall(values ...any) *TemplateBase {
	packages, opts := installArgs(values)
	global := optionBool(opts, "G", "g")
	args := []string{"npm", "install"}
	if global {
		args = append(args, "-g")
	}
	if optionBool(opts, "Dev", "dev") {
		args = append(args, "--save-dev")
	}
	args = append(args, stringList(packages)...)
	runOpts := &struct {
		User string
	}{}
	if global {
		runOpts.User = "root"
	}
	return t.RunCmd(strings.Join(args, " "), runOpts)
}

func (t *TemplateBase) BunInstall(values ...any) *TemplateBase {
	packages, opts := installArgs(values)
	global := optionBool(opts, "G", "g")
	args := []string{"bun", "install"}
	if global {
		args = append(args, "-g")
	}
	if optionBool(opts, "Dev", "dev") {
		args = append(args, "--dev")
	}
	args = append(args, stringList(packages)...)
	runOpts := &struct {
		User string
	}{}
	if global {
		runOpts.User = "root"
	}
	return t.RunCmd(strings.Join(args, " "), runOpts)
}

func (t *TemplateBase) AptInstall(packages any, opts ...any) *TemplateBase {
	opt := firstOpt(opts)
	pkgs := stringList(packages)
	install := "DEBIAN_FRONTEND=noninteractive DEBCONF_NOWARNINGS=yes apt-get install -y "
	if optionBool(opt, "NoInstallRecommends", "noInstallRecommends") {
		install += "--no-install-recommends "
	}
	if optionBool(opt, "FixMissing", "fixMissing") {
		install += "--fix-missing "
	}
	install += strings.Join(pkgs, " ")
	return t.RunCmd([]string{"apt-get update", install}, &struct {
		User string
	}{User: "root"})
}

func (t *TemplateBase) AddMcpServer(servers ...any) *TemplateBase {
	if t.baseTemplate != "mcp-gateway" {
		panic(&shared.BuildError{Message: "MCP servers can only be added to mcp-gateway template"})
	}
	serverList := flattenStringArgs(servers)
	if len(serverList) == 0 {
		return t
	}
	return t.RunCmd("mcp-gateway pull "+strings.Join(serverList, " "), &struct {
		User string
	}{User: "root"})
}

func (t *TemplateBase) GitClone(url string, args ...any) *TemplateBase {
	path := ""
	var opts any
	if len(args) > 0 {
		if p, ok := args[0].(string); ok {
			path = p
			if len(args) > 1 {
				opts = args[1]
			}
		} else {
			opts = args[0]
		}
	}
	parts := []string{"git", "clone", url}
	if branch := optionString(opts, "Branch", "branch"); branch != "" {
		parts = append(parts, "--branch "+branch, "--single-branch")
	}
	if depth := optionInt(opts, "Depth", "depth"); depth > 0 {
		parts = append(parts, "--depth "+strconv.Itoa(depth))
	}
	if path != "" {
		parts = append(parts, path)
	}
	return t.RunCmd(strings.Join(parts, " "), &struct {
		User string
	}{User: optionString(opts, "User", "user")})
}

func (t *TemplateBase) SetEnvs(envs map[string]string) *TemplateBase {
	if len(envs) == 0 {
		return t
	}
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		args = append(args, k, envs[k])
	}
	t.instructions = append(t.instructions, Instruction{Type: InstructionEnv, Args: args, Force: t.forceNextLayer})
	return t
}

func (t *TemplateBase) SkipCache() *TemplateBase {
	t.forceNextLayer = true
	return t
}

func (t *TemplateBase) SetStartCmd(startCommand string, readyCommand ...interface{}) *TemplateBase {
	t.startCmd = startCommand
	if cmd, ok := resolveReadyCommand(firstOpt(readyCommand)); ok {
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
	if t.baseTemplate != "devcontainer" {
		panic(&shared.BuildError{Message: "Devcontainers can only used in the devcontainer template"})
	}
	return t.RunCmd("devcontainer build --workspace-folder "+devcontainerDirectory, &struct {
		User string
	}{User: "root"})
}

func (t *TemplateBase) BetaSetDevContainerStart(devcontainerDirectory string) *TemplateBase {
	if t.baseTemplate != "devcontainer" {
		panic(&shared.BuildError{Message: "Devcontainers can only used in the devcontainer template"})
	}
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

func readGCPServiceAccountJSON(contextPath string, pathOrContent any) string {
	switch value := pathOrContent.(type) {
	case nil:
		return ""
	case string:
		data, err := os.ReadFile(filepathJoinIfRelative(contextPath, value))
		if err != nil {
			panic(&shared.BuildError{Message: err.Error()})
		}
		return string(data)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			panic(&shared.BuildError{Message: err.Error()})
		}
		return string(data)
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

func firstOpt(values []any) any {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func installArgs(values []any) (packages any, opts any) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) == 1 {
		return values[0], nil
	}
	return values[0], values[1]
}

func stringList(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return append([]string{}, v...)
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprint(item))
		}
		return result
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			result := make([]string, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				result = append(result, fmt.Sprint(rv.Index(i).Interface()))
			}
			return result
		}
		return []string{fmt.Sprint(value)}
	}
}

func flattenStringArgs(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, stringList(value)...)
	}
	return result
}

func optionBool(opts any, names ...string) bool {
	value, ok := optionValue(opts, names...)
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case *bool:
		return v != nil && *v
	case string:
		parsed, _ := strconv.ParseBool(v)
		return parsed
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() && rv.Kind() == reflect.Bool {
			return rv.Bool()
		}
		return false
	}
}

func optionBoolDefault(opts any, defaultValue bool, names ...string) bool {
	if _, ok := optionValue(opts, names...); !ok {
		return defaultValue
	}
	return optionBool(opts, names...)
}

func optionString(opts any, names ...string) string {
	value, ok := optionValue(opts, names...)
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	default:
		return fmt.Sprint(value)
	}
}

func optionInt(opts any, names ...string) int {
	value, ok := optionValue(opts, names...)
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int8, int16, int32, int64:
		return int(reflect.ValueOf(value).Int())
	case uint, uint8, uint16, uint32, uint64:
		return int(reflect.ValueOf(value).Uint())
	case string:
		parsed, _ := strconv.Atoi(v)
		return parsed
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() {
			switch rv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return int(rv.Int())
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return int(rv.Uint())
			}
		}
		return 0
	}
}

func optionMode(opts any) string {
	value, ok := optionValue(opts, "Mode", "mode")
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case int:
		if v == 0 {
			return ""
		}
		return fmt.Sprintf("%04o", v)
	case int8, int16, int32, int64:
		n := reflect.ValueOf(value).Int()
		if n == 0 {
			return ""
		}
		return fmt.Sprintf("%04o", n)
	case uint, uint8, uint16, uint32, uint64:
		n := reflect.ValueOf(value).Uint()
		if n == 0 {
			return ""
		}
		return fmt.Sprintf("%04o", n)
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() {
			switch rv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				n := rv.Int()
				if n == 0 {
					return ""
				}
				return fmt.Sprintf("%04o", n)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				n := rv.Uint()
				if n == 0 {
					return ""
				}
				return fmt.Sprintf("%04o", n)
			}
		}
		return fmt.Sprint(value)
	}
}

func optionValue(opts any, names ...string) (any, bool) {
	if opts == nil {
		return nil, false
	}
	if m, ok := opts.(map[string]any); ok {
		for _, name := range names {
			if value, ok := m[name]; ok {
				return value, true
			}
		}
	}

	rv := reflect.ValueOf(opts)
	for rv.IsValid() && rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil, false
	}
	if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
		for _, name := range names {
			value := rv.MapIndex(reflect.ValueOf(name))
			if value.IsValid() && value.CanInterface() {
				return value.Interface(), true
			}
		}
		return nil, false
	}
	if rv.Kind() != reflect.Struct {
		return nil, false
	}
	for _, name := range names {
		field := rv.FieldByName(name)
		if field.IsValid() && field.CanInterface() {
			return field.Interface(), true
		}
	}
	return nil, false
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
		config.Domain = "e2b.app"
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
		config.Domain = "e2b.app"
	}
	return api.NewApiClient(config, api.WithRequireApiKey())
}

// Static methods

// Build creates a template and waits for the build to finish.
func Build(ctx context.Context, template *TemplateBase, name string, opts *BuildOptions) (*BuildInfo, error) {
	if opts == nil {
		opts = &BuildOptions{}
	}
	var err error
	name, err = normalizeBuildName(name, opts)
	if err != nil {
		return nil, err
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
	var err error
	name, err = normalizeBuildName(name, opts)
	if err != nil {
		return nil, err
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

func normalizeBuildName(name string, opts *BuildOptions) (string, error) {
	if name == "" && opts != nil {
		name = opts.Alias
	}
	if name == "" {
		return "", &shared.TemplateError{SandboxError: shared.SandboxError{Message: "Name must be provided"}}
	}
	return name, nil
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

// AliasExists checks whether a template with the given alias exists.
//
// Deprecated: Use Exists instead.
func AliasExists(ctx context.Context, alias string, opts *BuildOptions) (bool, error) {
	return Exists(ctx, alias, opts)
}

// AssignTags assigns tags to a template.
func AssignTags(ctx context.Context, targetName string, tags any, opts *BuildOptions) (*TemplateTagInfo, error) {
	if opts == nil {
		opts = &BuildOptions{}
	}

	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return nil, err
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return nil, err
	}

	return assignTags(ctx, client, targetName, normalizedTags)
}

// RemoveTags removes tags from a template.
func RemoveTags(ctx context.Context, name string, tags any, opts *BuildOptions) error {
	if opts == nil {
		opts = &BuildOptions{}
	}

	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return err
	}

	client, err := newApiClientFromBuildOptions(opts)
	if err != nil {
		return err
	}

	return removeTags(ctx, client, name, normalizedTags)
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

func normalizeTags(tags any) ([]string, error) {
	switch value := tags.(type) {
	case string:
		return []string{value}, nil
	case []string:
		return append([]string{}, value...), nil
	default:
		return nil, &shared.TemplateError{SandboxError: shared.SandboxError{Message: "Tags must be a string or []string"}}
	}
}
