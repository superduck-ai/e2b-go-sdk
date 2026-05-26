package template

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/api"
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

func TestSetReadyCmdAcceptsStringLikeJs(t *testing.T) {
	template := Template(nil).SetReadyCmd("curl http://localhost:8000/health")

	serialized := template.serialize()
	if serialized.ReadyCmd != "curl http://localhost:8000/health" {
		t.Fatalf("expected string ready command to be serialized, got %#v", serialized)
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
	if len(serialized.Steps) != 1 || serialized.Steps[0].Type != InstructionWorkdir {
		t.Fatalf("expected Dockerfile path to parse steps, got %#v", serialized.Steps)
	}
}

func TestTemplateBaseExposesJsDevcontainerAndMcpHelpers(t *testing.T) {
	template := Template(nil).
		AddMcpServer("exa", "brave").
		BetaDevContainerPrebuild("/workspace").
		BetaSetDevContainerStart("/workspace")

	serialized := template.serialize()
	if len(serialized.Steps) < 2 {
		t.Fatalf("expected helper methods to add steps, got %#v", serialized)
	}
	if serialized.Steps[0].Args[0] != "mcp-gateway pull exa brave" {
		t.Fatalf("unexpected MCP helper command: %#v", serialized.Steps[0])
	}
	if serialized.Steps[1].Args[0] != "devcontainer build --workspace-folder /workspace" {
		t.Fatalf("unexpected devcontainer prebuild command: %#v", serialized.Steps[1])
	}
	if serialized.StartCmd == "" || serialized.ReadyCmd != "[ -f /devcontainer.up ]" {
		t.Fatalf("expected devcontainer start helper to set start and ready commands, got %#v", serialized)
	}
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
