package template

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/api"
	"github.com/e2b-dev/e2b-go-sdk/internal/shared"
)

func TestAssignTagsUsesJsTargetPayloadAndReturnsTagInfo(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"buildID": "bld-123",
			"tags":    []string{"prod", "stable"},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "test-api-key",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	info, err := assignTags(context.Background(), client, "tmpl:latest", []string{"prod", "stable"})
	if err != nil {
		t.Fatalf("expected assign tags to succeed, got %v", err)
	}
	if body["target"] != "tmpl:latest" {
		t.Fatalf("expected JS-style target field, got %#v", body)
	}
	if _, ok := body["templateName"]; ok {
		t.Fatalf("did not expect legacy templateName field, got %#v", body)
	}
	if info == nil || info.BuildID != "bld-123" || !reflect.DeepEqual(info.Tags, []string{"prod", "stable"}) {
		t.Fatalf("unexpected tag info: %#v", info)
	}
}

func TestAssignTagsAcceptsSingleTagLikeJsAndPython(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"buildID": "bld-123",
			"tags":    []string{"prod"},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	info, err := AssignTags(context.Background(), "tmpl:latest", "prod", &BuildOptions{
		ApiKey:           "test-api-key",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("expected AssignTags to succeed, got %v", err)
	}
	if info == nil || !reflect.DeepEqual(info.Tags, []string{"prod"}) {
		t.Fatalf("unexpected tag info: %#v", info)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "prod" {
		t.Fatalf("expected single tag to be normalized to array, got %#v", body)
	}
}

func TestRemoveTagsUsesJsNamePayload(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "test-api-key",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	if err := removeTags(context.Background(), client, "tmpl", []string{"old"}); err != nil {
		t.Fatalf("expected remove tags to succeed, got %v", err)
	}
	if body["name"] != "tmpl" {
		t.Fatalf("expected JS-style name field, got %#v", body)
	}
	if _, ok := body["templateName"]; ok {
		t.Fatalf("did not expect legacy templateName field, got %#v", body)
	}
}

func TestRemoveTagsAcceptsSingleTagLikeJsAndPython(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := RemoveTags(context.Background(), "tmpl", "old", &BuildOptions{
		ApiKey:           "test-api-key",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	}); err != nil {
		t.Fatalf("expected RemoveTags to succeed, got %v", err)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "old" {
		t.Fatalf("expected single tag to be normalized to array, got %#v", body)
	}
}

func TestTemplateTagsRejectUnsupportedTagsShape(t *testing.T) {
	_, err := AssignTags(context.Background(), "tmpl:latest", 123, nil)
	var templateErr *shared.TemplateError
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}

	err = RemoveTags(context.Background(), "tmpl", 123, nil)
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}
}

func TestGetBuildStatusFromAPIMapsJsBuildStatusShape(t *testing.T) {
	timestamp := time.Date(2026, 5, 26, 12, 34, 56, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tmpl-1/builds/bld-1/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("logsOffset"); got != "3" {
			t.Fatalf("expected logsOffset=3, got %q", got)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"buildID":    "bld-1",
			"templateID": "tmpl-1",
			"status":     "error",
			"logEntries": []map[string]any{
				{
					"timestamp": timestamp.Format(time.RFC3339),
					"level":     "info",
					"message":   "building",
				},
			},
			"logs": []string{"building"},
			"reason": map[string]any{
				"message": "step failed",
				"step":    "finalize",
				"logEntries": []map[string]any{
					{
						"timestamp": timestamp.Format(time.RFC3339),
						"level":     "error",
						"message":   "boom",
					},
				},
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "test-api-key",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	status, err := getBuildStatusFromAPI(context.Background(), client, "tmpl-1", "bld-1", 3)
	if err != nil {
		t.Fatalf("expected build status to succeed, got %v", err)
	}
	if status.BuildID != "bld-1" || status.TemplateID != "tmpl-1" || status.Status != BuildStatusError {
		t.Fatalf("unexpected status envelope: %#v", status)
	}
	if len(status.LogEntries) != 1 || status.LogEntries[0].Message != "building" {
		t.Fatalf("expected mapped log entries, got %#v", status.LogEntries)
	}
	if !reflect.DeepEqual(status.Logs, []string{"building"}) {
		t.Fatalf("expected raw logs slice, got %#v", status.Logs)
	}
	if status.Reason == nil || status.Reason.Message != "step failed" || status.Reason.Step != "finalize" {
		t.Fatalf("expected mapped reason, got %#v", status.Reason)
	}
	if len(status.Reason.LogEntries) != 1 || status.Reason.LogEntries[0].Message != "boom" {
		t.Fatalf("expected mapped reason log entries, got %#v", status.Reason.LogEntries)
	}
}

func TestBuildInfoIncludesDeprecatedAliasField(t *testing.T) {
	info := &BuildInfo{
		Alias:      "tmpl:v1",
		Name:       "tmpl:v1",
		Tags:       []string{"stable"},
		TemplateID: "tmpl-1",
		BuildID:    "bld-1",
	}

	if info.Alias != info.Name {
		t.Fatalf("expected deprecated alias to mirror name, got %#v", info)
	}
}

func TestNormalizeBuildNameMatchesJsAndPython(t *testing.T) {
	cases := []struct {
		name string
		opts *BuildOptions
		want string
	}{
		{name: "my-template:v1.0", want: "my-template:v1.0"},
		{name: "my-template", want: "my-template"},
		{opts: &BuildOptions{BasicBuildOptions: BasicBuildOptions{Alias: "legacy-template"}}, want: "legacy-template"},
		{name: "from-name", opts: &BuildOptions{BasicBuildOptions: BasicBuildOptions{Alias: "from-alias"}}, want: "from-name"},
	}

	for _, tc := range cases {
		got, err := normalizeBuildName(tc.name, tc.opts)
		if err != nil {
			t.Fatalf("normalizeBuildName(%q, %#v) returned error: %v", tc.name, tc.opts, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeBuildName(%q, %#v) = %q, want %q", tc.name, tc.opts, got, tc.want)
		}
	}
}

func TestBuildInBackgroundNormalizesLegacyAliasWithoutDroppingOptions(t *testing.T) {
	var createBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("failed to decode create body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-legacy",
				"buildID":    "bld-legacy",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/v2/templates/tmpl-legacy/builds/bld-legacy":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	info, err := BuildInBackground(context.Background(), Template(nil).FromBaseImage(), "", &BuildOptions{
		BasicBuildOptions: BasicBuildOptions{
			Alias:    "legacy-template",
			Tags:     []string{"stable"},
			CpuCount: 4,
			MemoryMB: 1024,
		},
		ApiKey:           "test-api-key",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("BuildInBackground with legacy alias returned error: %v", err)
	}
	if info == nil || info.Name != "legacy-template" || info.Alias != "legacy-template" {
		t.Fatalf("unexpected build info: %#v", info)
	}
	if createBody["name"] != "legacy-template" {
		t.Fatalf("expected legacy alias to be normalized to name, got %#v", createBody)
	}
	if _, ok := createBody["alias"]; ok {
		t.Fatalf("did not expect alias to be sent in build request, got %#v", createBody)
	}
	if got := int(createBody["cpuCount"].(float64)); got != 4 {
		t.Fatalf("expected cpuCount to be preserved, got %#v", createBody)
	}
	if got := int(createBody["memoryMB"].(float64)); got != 1024 {
		t.Fatalf("expected memoryMB to be preserved, got %#v", createBody)
	}
	tags, ok := createBody["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "stable" {
		t.Fatalf("expected tags to be preserved, got %#v", createBody)
	}
}

func TestBuildRejectsMissingNameBeforeApiConfig(t *testing.T) {
	_, err := BuildInBackground(context.Background(), Template(nil), "", nil)
	var templateErr *shared.TemplateError
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}
	if templateErr.Message != "Name must be provided" {
		t.Fatalf("unexpected TemplateError message: %q", templateErr.Message)
	}

	for _, opts := range []*BuildOptions{{}, {BasicBuildOptions: BasicBuildOptions{Alias: ""}}} {
		_, err := normalizeBuildName("", opts)
		var templateErr *shared.TemplateError
		if !errors.As(err, &templateErr) {
			t.Fatalf("expected TemplateError for opts %#v, got %T %v", opts, err, err)
		}
	}
}

func TestInstructionTypeAndArgsMatchJsShape(t *testing.T) {
	if InstructionCopy != "COPY" || InstructionEnv != "ENV" || InstructionRun != "RUN" ||
		InstructionWorkdir != "WORKDIR" || InstructionUser != "USER" {
		t.Fatalf("expected JS-style instruction type values, got %q %q %q %q %q",
			InstructionCopy, InstructionEnv, InstructionRun, InstructionWorkdir, InstructionUser)
	}

	instruction := Template(nil).Copy("src", "dst", nil).SetEnvs(map[string]string{"KEY": "VALUE"}).instructionsList()
	if len(instruction) < 2 {
		t.Fatalf("expected instructions to be recorded, got %#v", instruction)
	}
	if !reflect.DeepEqual(instruction[0].Args, []string{"src", "dst", "", ""}) {
		t.Fatalf("expected COPY args to be tokenized, got %#v", instruction[0].Args)
	}
	if !reflect.DeepEqual(instruction[1].Args, []string{"KEY", "VALUE"}) {
		t.Fatalf("expected ENV args to be tokenized, got %#v", instruction[1].Args)
	}
}

func TestTemplateSerializationMatchesJsStyleHelpers(t *testing.T) {
	template := Template(nil).
		FromPythonImage("3.12").
		Copy("src", "/app/src", nil).
		SetEnvs(map[string]string{"KEY": "VALUE"}).
		SetWorkdir("/app").
		SetUser("user").
		SetStartCmd("python main.py", nil)

	jsonText, err := ToJSON(template)
	if err != nil {
		t.Fatalf("ToJSON returned error: %v", err)
	}
	expectedJSONParts := []string{
		`"fromImage": "python:3.12"`,
		`"startCmd": "python main.py"`,
		`"steps": [`,
		`"type": "COPY"`,
		`"args": [`,
		`"/app/src"`,
	}
	for _, part := range expectedJSONParts {
		if !strings.Contains(jsonText, part) {
			t.Fatalf("expected ToJSON output to contain %q, got %s", part, jsonText)
		}
	}

	dockerfile, err := ToDockerfile(template)
	if err != nil {
		t.Fatalf("ToDockerfile returned error: %v", err)
	}
	expectedDockerfile := "FROM python:3.12\nCOPY src /app/src\nENV KEY=VALUE\nWORKDIR /app\nUSER user\nENTRYPOINT python main.py\n"
	if dockerfile != expectedDockerfile {
		t.Fatalf("unexpected dockerfile output:\n%s", dockerfile)
	}
}

func TestToDockerfileGroupsEnvInstructionsLikeJsAndPython(t *testing.T) {
	template := Template(nil).
		FromUbuntuImage("24.04").
		SetEnvs(map[string]string{"NODE_ENV": "production", "PORT": "8080"}).
		SetEnvs(map[string]string{"DEBUG": "false"})

	dockerfile, err := ToDockerfile(template)
	if err != nil {
		t.Fatalf("ToDockerfile returned error: %v", err)
	}

	expected := "FROM ubuntu:24.04\nENV NODE_ENV=production PORT=8080\nENV DEBUG=false\n"
	if dockerfile != expected {
		t.Fatalf("unexpected dockerfile output:\n%s", dockerfile)
	}
}

func TestUploadFileSetsContentLengthAndAvoidsChunkedEncoding(t *testing.T) {
	var headers http.Header
	var bodyLength int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read upload body: %v", err)
		}
		bodyLength = len(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := uploadFile(context.Background(), server.URL, []byte("archive bytes")); err != nil {
		t.Fatalf("uploadFile returned error: %v", err)
	}

	contentLength := headers.Get("Content-Length")
	if contentLength == "" {
		t.Fatalf("expected Content-Length header, got %#v", headers)
	}
	if contentLength != strconv.Itoa(bodyLength) {
		t.Fatalf("expected Content-Length %d, got %q", bodyLength, contentLength)
	}
	if transferEncoding := headers.Get("Transfer-Encoding"); strings.Contains(strings.ToLower(transferEncoding), "chunked") {
		t.Fatalf("did not expect chunked transfer encoding, got %q", transferEncoding)
	}
}

func TestSetReadyCmdAcceptsStringLikeJs(t *testing.T) {
	template := Template(nil).SetReadyCmd("curl http://localhost:8000/health")

	serialized := template.serialize()
	if serialized.ReadyCmd != "curl http://localhost:8000/health" {
		t.Fatalf("expected string ready command to be serialized, got %#v", serialized)
	}
}

func TestReadyCmdHelpersMatchCurrentTs(t *testing.T) {
	cases := []struct {
		name string
		got  *ReadyCmd
		want string
	}{
		{name: "port", got: WaitForPort(8000), want: "ss -tuln | grep :8000"},
		{name: "url", got: WaitForURL("http://localhost:3000/health", 201), want: `curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/health | grep -q "201"`},
		{name: "process", got: WaitForProcess("nginx"), want: "pgrep nginx > /dev/null"},
		{name: "file", got: WaitForFile("/tmp/ready"), want: "[ -f /tmp/ready ]"},
		{name: "timeout-minimum", got: WaitForTimeout(500), want: "sleep 1"},
		{name: "timeout-floor", got: WaitForTimeout(2500), want: "sleep 2"},
	}

	for _, tc := range cases {
		if tc.got.GetCmd() != tc.want {
			t.Fatalf("%s ready command mismatch: got %q want %q", tc.name, tc.got.GetCmd(), tc.want)
		}
	}
}

func TestFromDockerfileAcceptsFilePathLikeJs(t *testing.T) {
	contextDir := t.TempDir()
	dockerfilePath := filepath.Join(contextDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM python:3.12\nWORKDIR /app\n"), 0o644); err != nil {
		t.Fatalf("failed to write Dockerfile fixture: %v", err)
	}

	template := Template(&TemplateOptions{FileContextPath: contextDir}).FromDockerfile("Dockerfile")
	serialized := template.serialize()
	if serialized.FromImage != "python:3.12" {
		t.Fatalf("expected Dockerfile path to set base image, got %#v", serialized)
	}
	if len(serialized.Steps) != 4 ||
		serialized.Steps[0].Type != InstructionUser ||
		serialized.Steps[0].Args[0] != "root" ||
		serialized.Steps[1].Type != InstructionWorkdir ||
		serialized.Steps[1].Args[0] != "/" ||
		serialized.Steps[2].Type != InstructionWorkdir ||
		serialized.Steps[2].Args[0] != "/app" ||
		serialized.Steps[3].Type != InstructionUser ||
		serialized.Steps[3].Args[0] != "user" {
		t.Fatalf("expected Dockerfile path to parse steps, got %#v", serialized.Steps)
	}
}

func TestFromDockerfileMatchesJsAndPythonInstructionOrder(t *testing.T) {
	dockerfile := `FROM node:24
WORKDIR /app
COPY package.json .
RUN npm install
ENTRYPOINT ["sleep", "20"]`

	template := Template(nil).FromDockerfile(dockerfile)
	instructions := template.instructionsList()

	if template.baseImage != "node:24" {
		t.Fatalf("expected base image node:24, got %q", template.baseImage)
	}
	if len(instructions) != 6 {
		t.Fatalf("expected 6 instructions, got %#v", instructions)
	}

	expected := []Instruction{
		{Type: InstructionUser, Args: []string{"root"}},
		{Type: InstructionWorkdir, Args: []string{"/"}},
		{Type: InstructionWorkdir, Args: []string{"/app"}},
		{Type: InstructionCopy, Args: []string{"package.json", ".", "", ""}},
		{Type: InstructionRun, Args: []string{"npm install"}},
		{Type: InstructionUser, Args: []string{"user"}},
	}
	for i, want := range expected {
		if instructions[i].Type != want.Type || !reflect.DeepEqual(instructions[i].Args, want.Args) {
			t.Fatalf("instruction %d mismatch: got %#v want %#v", i, instructions[i], want)
		}
	}
	if template.startCmd != "sleep 20" {
		t.Fatalf("expected JSON ENTRYPOINT to become shell command, got %q", template.startCmd)
	}
	if template.readyCmd != "sleep 20" {
		t.Fatalf("expected Dockerfile start command to get 20s ready timeout, got %q", template.readyCmd)
	}
}

func TestFromDockerfileQuotesJsonEntrypointArgsWithSpaces(t *testing.T) {
	template := Template(nil).FromDockerfile(`FROM python:3.12
ENTRYPOINT ["python", "-c", "print(\"hi there\")"]`)

	expected := `python -c 'print("hi there")'`
	if template.startCmd != expected {
		t.Fatalf("expected JSON ENTRYPOINT args to be shell-quoted, got %q", template.startCmd)
	}
}

func TestFromDockerfileAppliesE2BDefaultsLikeJsAndPython(t *testing.T) {
	template := Template(nil).FromDockerfile("FROM node:24")
	instructions := template.instructionsList()

	if len(instructions) < 2 {
		t.Fatalf("expected default user/workdir instructions, got %#v", instructions)
	}
	defaultUser := instructions[len(instructions)-2]
	defaultWorkdir := instructions[len(instructions)-1]
	if defaultUser.Type != InstructionUser || defaultUser.Args[0] != "user" {
		t.Fatalf("expected default USER user, got %#v", defaultUser)
	}
	if defaultWorkdir.Type != InstructionWorkdir || defaultWorkdir.Args[0] != "/home/user" {
		t.Fatalf("expected default WORKDIR /home/user, got %#v", defaultWorkdir)
	}
}

func TestFromDockerfileKeepsCustomUserAndWorkdirLikeJsAndPython(t *testing.T) {
	template := Template(nil).FromDockerfile("FROM node:24\nUSER mish\nWORKDIR /home/mish")
	instructions := template.instructionsList()

	if len(instructions) < 2 {
		t.Fatalf("expected custom user/workdir instructions, got %#v", instructions)
	}
	customUser := instructions[len(instructions)-2]
	customWorkdir := instructions[len(instructions)-1]
	if customUser.Type != InstructionUser || customUser.Args[0] != "mish" {
		t.Fatalf("expected custom USER mish, got %#v", customUser)
	}
	if customWorkdir.Type != InstructionWorkdir || customWorkdir.Args[0] != "/home/mish" {
		t.Fatalf("expected custom WORKDIR /home/mish, got %#v", customWorkdir)
	}
}

func TestFromDockerfileParsesCopyChownLikeJsAndPython(t *testing.T) {
	dockerfile := `FROM node:24
COPY --chown=myuser:mygroup app.js /app/
COPY --chown=anotheruser config.json /config/`

	instructions := Template(nil).FromDockerfile(dockerfile).instructionsList()
	if len(instructions) < 4 {
		t.Fatalf("expected COPY instructions after Docker defaults, got %#v", instructions)
	}

	copyInstruction1 := instructions[2]
	if copyInstruction1.Type != InstructionCopy ||
		!reflect.DeepEqual(copyInstruction1.Args, []string{"app.js", "/app/", "myuser:mygroup", ""}) {
		t.Fatalf("unexpected first COPY instruction: %#v", copyInstruction1)
	}

	copyInstruction2 := instructions[3]
	if copyInstruction2.Type != InstructionCopy ||
		!reflect.DeepEqual(copyInstruction2.Args, []string{"config.json", "/config/", "anotheruser", ""}) {
		t.Fatalf("unexpected second COPY instruction: %#v", copyInstruction2)
	}
}

func TestTemplateBaseExposesJsDevcontainerAndMcpHelpers(t *testing.T) {
	mcpTemplate := Template(nil).
		FromTemplate("mcp-gateway").
		AddMcpServer("exa", "brave").
		serialize()
	if len(mcpTemplate.Steps) != 1 || mcpTemplate.Steps[0].Args[0] != "mcp-gateway pull exa brave" {
		t.Fatalf("unexpected MCP helper command: %#v", mcpTemplate.Steps)
	}

	devcontainerTemplate := Template(nil).
		FromTemplate("devcontainer").
		BetaDevContainerPrebuild("/workspace").
		BetaSetDevContainerStart("/workspace")

	serialized := devcontainerTemplate.serialize()
	if len(serialized.Steps) < 1 {
		t.Fatalf("expected helper methods to add steps, got %#v", serialized)
	}
	if serialized.Steps[0].Args[0] != "devcontainer build --workspace-folder /workspace" {
		t.Fatalf("unexpected devcontainer prebuild command: %#v", serialized.Steps[0])
	}
	if serialized.StartCmd == "" || serialized.ReadyCmd != "[ -f /devcontainer.up ]" {
		t.Fatalf("expected devcontainer start helper to set start and ready commands, got %#v", serialized)
	}
}

func TestTemplateBaseRejectsMcpServerWithoutMcpGatewayBase(t *testing.T) {
	assertBuildPanic(t, "MCP servers can only be added to mcp-gateway template", func() {
		Template(nil).FromBaseImage().AddMcpServer("exa")
	})
}

func TestTemplateBaseRejectsDevcontainerHelpersWithoutDevcontainerBase(t *testing.T) {
	assertBuildPanic(t, "Devcontainers can only used in the devcontainer template", func() {
		Template(nil).FromBaseImage().BetaDevContainerPrebuild("/workspace")
	})
	assertBuildPanic(t, "Devcontainers can only used in the devcontainer template", func() {
		Template(nil).FromBaseImage().BetaSetDevContainerStart("/workspace")
	})
}

func TestTemplateBuilderDefaultsAndHelpersMatchCurrentTs(t *testing.T) {
	defaultTemplate := Template(nil).serialize()
	if defaultTemplate.FromImage != "e2bdev/base" {
		t.Fatalf("expected default base image e2bdev/base, got %#v", defaultTemplate.FromImage)
	}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "debian", got: Template(nil).FromDebianImage().serialize().FromImage, want: "debian:stable"},
		{name: "ubuntu", got: Template(nil).FromUbuntuImage().serialize().FromImage, want: "ubuntu:latest"},
		{name: "python", got: Template(nil).FromPythonImage().serialize().FromImage, want: "python:3"},
		{name: "node", got: Template(nil).FromNodeImage().serialize().FromImage, want: "node:lts"},
		{name: "bun", got: Template(nil).FromBunImage().serialize().FromImage, want: "oven/bun:latest"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s default image mismatch: got %q want %q", tc.name, tc.got, tc.want)
		}
	}

	template := Template(nil).
		Copy([]string{"a.txt", "b.txt"}, "/app", &struct {
			ForceUpload     bool
			User            string
			Mode            int
			ResolveSymlinks bool
		}{ForceUpload: true, User: "root", Mode: 0o755, ResolveSymlinks: true}).
		Remove([]string{"/tmp/a", "/tmp/b"}, &struct {
			Recursive bool
			Force     bool
			User      string
		}{Recursive: true, Force: true, User: "root"}).
		Rename("/tmp/old", "/tmp/new", &struct {
			Force bool
			User  string
		}{Force: true, User: "root"}).
		MakeDir([]string{"/app/logs", "/app/cache"}, &struct {
			Mode int
			User string
		}{Mode: 0o755, User: "root"}).
		MakeSymlink("/usr/bin/python3", "/usr/bin/python", &struct {
			Force bool
			User  string
		}{Force: true, User: "root"}).
		RunCmd([]string{"echo one", "echo two"}, &struct{ User string }{User: "root"}).
		PipInstall("numpy", nil).
		PipInstall([]string{"pandas"}, &struct{ G bool }{G: false}).
		NpmInstall("tsx", &struct{ G bool }{G: true}).
		NpmInstall("typescript", &struct{ Dev bool }{Dev: true}).
		BunInstall("tsx", &struct{ G bool }{G: true}).
		BunInstall("typescript", &struct{ Dev bool }{Dev: true}).
		AptInstall([]string{"git", "curl"}, &struct {
			NoInstallRecommends bool
			FixMissing          bool
		}{NoInstallRecommends: true, FixMissing: true}).
		GitClone("https://github.com/e2b-dev/E2B.git", "/src", &struct {
			Branch string
			Depth  int
			User   string
		}{Branch: "main", Depth: 1, User: "root"})

	steps := template.serialize().Steps
	assertStepArgs := func(index int, want []string) {
		t.Helper()
		if !reflect.DeepEqual(steps[index].Args, want) {
			t.Fatalf("step %d args mismatch:\n got %#v\nwant %#v", index, steps[index].Args, want)
		}
	}
	assertStepArgs(0, []string{"a.txt", "/app", "root", "0755"})
	assertStepArgs(1, []string{"b.txt", "/app", "root", "0755"})
	assertStepArgs(2, []string{"rm -r -f /tmp/a /tmp/b", "root"})
	assertStepArgs(3, []string{"mv /tmp/old /tmp/new -f", "root"})
	assertStepArgs(4, []string{"mkdir -p -m 0755 /app/logs /app/cache", "root"})
	assertStepArgs(5, []string{"ln -s -f /usr/bin/python3 /usr/bin/python", "root"})
	assertStepArgs(6, []string{"echo one && echo two", "root"})
	assertStepArgs(7, []string{"pip install numpy", "root"})
	assertStepArgs(8, []string{"pip install --user pandas"})
	assertStepArgs(9, []string{"npm install -g tsx", "root"})
	assertStepArgs(10, []string{"npm install --save-dev typescript"})
	assertStepArgs(11, []string{"bun install -g tsx", "root"})
	assertStepArgs(12, []string{"bun install --dev typescript"})
	assertStepArgs(13, []string{"apt-get update && DEBIAN_FRONTEND=noninteractive DEBCONF_NOWARNINGS=yes apt-get install -y --no-install-recommends --fix-missing git curl", "root"})
	assertStepArgs(14, []string{"git clone https://github.com/e2b-dev/E2B.git --branch main --single-branch --depth 1 /src", "root"})
}

func TestTemplateBuilderOptionalArgsAndCopyItemsMatchCurrentTs(t *testing.T) {
	template := Template(nil).
		RunCmd("echo ok").
		PipInstall().
		NpmInstall().
		BunInstall().
		CopyItems([]CopyItem{{
			Src:  []string{"one.txt", "two.txt"},
			Dest: "/app",
			Mode: 0o644,
		}, {
			Src:  "plain.txt",
			Dest: "/app",
		}})

	steps := template.serialize().Steps
	got := [][]string{
		steps[0].Args,
		steps[1].Args,
		steps[2].Args,
		steps[3].Args,
		steps[4].Args,
		steps[5].Args,
		steps[6].Args,
	}
	want := [][]string{
		{"echo ok"},
		{"pip install .", "root"},
		{"npm install"},
		{"bun install"},
		{"one.txt", "/app", "", "0644"},
		{"two.txt", "/app", "", "0644"},
		{"plain.txt", "/app", "", ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("optional builder args mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func TestFromGCPRegistryReadsServiceAccountLikeCurrentTs(t *testing.T) {
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "service-account.json"), []byte(`{"project_id":"demo"}`), 0o644); err != nil {
		t.Fatalf("failed to write service account fixture: %v", err)
	}

	fromFile := Template(&TemplateOptions{FileContextPath: contextDir}).
		FromGCPRegistry("gcr.io/demo/image:latest", &GCPRegistryCredentials{ServiceAccountJSON: "service-account.json"}).
		serialize()
	if fromFile.FromImageRegistry == nil || fromFile.FromImageRegistry.ServiceAccountJSON != `{"project_id":"demo"}` {
		t.Fatalf("expected GCP credentials to be read from file, got %#v", fromFile.FromImageRegistry)
	}

	fromObject := Template(nil).
		FromGCPRegistry("gcr.io/demo/image:latest", &GCPRegistryCredentials{ServiceAccountJSON: map[string]string{"project_id": "demo"}}).
		serialize()
	if fromObject.FromImageRegistry == nil || fromObject.FromImageRegistry.ServiceAccountJSON != `{"project_id":"demo"}` {
		t.Fatalf("expected GCP object credentials to be stringified, got %#v", fromObject.FromImageRegistry)
	}
}

func assertBuildPanic(t *testing.T, message string, fn func()) {
	t.Helper()
	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("expected panic %q", message)
		}
		if err, ok := got.(error); !ok || err.Error() != message {
			t.Fatalf("expected panic %q, got %#v", message, got)
		}
	}()
	fn()
}

func TestBuildInBackgroundUsesStructuredJsTemplatePayload(t *testing.T) {
	var triggerBody map[string]any
	var uploaded bool
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "src"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatalf("failed to write source fixture: %v", err)
	}
	expectedHash, err := calculateFilesHash("src", "/app/src", contextDir, nil, false)
	if err != nil {
		t.Fatalf("failed to calculate expected hash: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-1",
				"buildID":    "bld-1",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/templates/tmpl-1/files/" + expectedHash:
			if err := json.NewEncoder(w).Encode(map[string]any{
				"present": false,
				"url":     server.URL + "/upload",
			}); err != nil {
				t.Fatalf("failed to encode upload link response: %v", err)
			}
		case "/upload":
			uploaded = true
			w.WriteHeader(http.StatusOK)
		case "/v2/templates/tmpl-1/builds/bld-1":
			if err := json.NewDecoder(r.Body).Decode(&triggerBody); err != nil {
				t.Fatalf("failed to decode trigger body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	template := Template(&TemplateOptions{FileContextPath: contextDir}).
		FromPythonImage("3.12").
		SetStartCmd("python main.py", WaitForFile("/ready")).
		Copy("src", "/app/src", nil)

	buildInfo, err := BuildInBackground(context.Background(), template, "tmpl:v1", &BuildOptions{
		ApiKey:           "test-api-key",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("BuildInBackground returned error: %v", err)
	}
	if buildInfo == nil || buildInfo.TemplateID != "tmpl-1" || buildInfo.BuildID != "bld-1" {
		t.Fatalf("unexpected build info: %#v", buildInfo)
	}
	if _, ok := triggerBody["instructions"]; ok {
		t.Fatalf("did not expect legacy instructions payload, got %#v", triggerBody)
	}
	if triggerBody["fromImage"] != "python:3.12" {
		t.Fatalf("expected JS-style fromImage field, got %#v", triggerBody)
	}
	if triggerBody["startCmd"] != "python main.py" {
		t.Fatalf("expected JS-style startCmd field, got %#v", triggerBody)
	}
	if triggerBody["readyCmd"] != "[ -f /ready ]" {
		t.Fatalf("expected JS-style readyCmd field, got %#v", triggerBody)
	}
	steps, ok := triggerBody["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("expected JS-style steps array, got %#v", triggerBody)
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["filesHash"] != expectedHash {
		t.Fatalf("expected COPY step to include filesHash, got %#v", steps)
	}
	if !uploaded {
		t.Fatal("expected copy source to be uploaded before triggering build")
	}
}

func TestSkipCacheMarksWholeTemplateWhenAppliedBeforeBaseLayer(t *testing.T) {
	template := Template(nil).SkipCache().FromPythonImage("3.12")

	serialized := template.serialize()
	if !serialized.Force {
		t.Fatalf("expected SkipCache before base image to force whole template, got %#v", serialized)
	}
	if serialized.FromImage != "python:3.12" {
		t.Fatalf("expected serialized base image, got %#v", serialized)
	}
}

func TestToDockerfileRejectsTemplateBaseWithoutDockerImage(t *testing.T) {
	_, err := ToDockerfile(Template(nil).FromTemplate("base"))
	if err == nil {
		t.Fatal("expected ToDockerfile to reject templates built from other templates")
	}
	expected := "Cannot convert template built from another template to Dockerfile. Templates based on other templates can only be built using the E2B API."
	if err.Error() != expected {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateBuildOptionShapesIncludeSharedConnectionFields(t *testing.T) {
	buildOpts := reflect.TypeOf(BuildOptions{})
	if _, ok := buildOpts.FieldByName("SandboxUrl"); !ok {
		t.Fatal("expected BuildOptions to expose SandboxUrl like shared JS connection options")
	}

	statusOpts := reflect.TypeOf(GetBuildStatusOptions{})
	if _, ok := statusOpts.FieldByName("SandboxUrl"); !ok {
		t.Fatal("expected GetBuildStatusOptions to expose SandboxUrl like shared JS connection options")
	}
}

func intPtr(value int) *int {
	return &value
}
