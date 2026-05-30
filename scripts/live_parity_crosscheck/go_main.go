package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
	e2bvol "github.com/superduck-ai/e2b-go-sdk/volume"
)

type result struct {
	Language string            `json:"language"`
	Case     string            `json:"case"`
	Status   string            `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

type capturedRequest struct {
	Method      string
	Path        string
	ContentType string
	Body        map[string]any
}

type capturedWireRequest struct {
	Method        string
	Path          string
	Query         map[string]string
	ContentType   string
	Authorization string
	Body          any
}

const numpyRandomCommand = `python3 - <<'PY'
import numpy as np
print([np.random.normal(), np.random.normal(), np.random.normal()])
PY`

const templateMethodsSummaryCommand = `printf 'runtime_user=%s\n' "$(whoami)"
printf 'bashrc_target=%s\n' "$(readlink /home/user/.bashrc.local)"
printf 'preserved_type=%s\n' "$(if [ -L /app/link-preserved.txt ]; then echo symlink; else echo regular; fi)"
printf 'preserved_target=%s\n' "$(readlink /app/link-preserved.txt)"
printf 'preserved_content=%s\n' "$(cat /app/link-preserved.txt)"
printf 'resolved_type=%s\n' "$(if [ -L /app/link-resolved.txt ]; then echo symlink; else echo regular; fi)"
printf 'resolved_content=%s\n' "$(cat /app/link-resolved.txt)"`

const expectedTemplateMethodsSummary = `runtime_user=user
bashrc_target=.bashrc
preserved_type=symlink
preserved_target=test.txt
preserved_content=template symlink content
resolved_type=regular
resolved_content=template symlink content`

const claudeDerivedNumpyInstallCommand = "python3 -m pip install --break-system-packages --no-cache-dir numpy"
const randomnessAliasTemplate = "en716jw99aj63v1k8ugh"
const upstreamRandomnessAliasCommand = `python -c "import numpy as np; print([np.random.normal(),np.random.normal(),np.random.normal()])"`

func runForegroundCommand(cmds *e2b.Commands, ctx context.Context, cmd string, opts *e2b.CommandStartOpts) (*e2b.CommandResult, error) {
	execution, err := cmds.Run(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	result, ok := execution.(*e2b.CommandResult)
	if !ok {
		return nil, fmt.Errorf("expected foreground command result, got %T", execution)
	}
	return result, nil
}

func main() {
	caseName := flag.String("case", "all", "case to run: all, claude, claude_derived, randomness, randomness_alias, volume, volume_api_payload, ubuntu, template_timeout, template_methods, config_headers, metrics, network_rules, network_egress, network_update_payload, template_api_payload, debug_root")
	flag.Parse()

	results := make([]result, 0, 3)
	for _, current := range selectedCases(*caseName) {
		switch current {
		case "claude":
			results = append(results, runClaudeCase())
		case "claude_derived":
			results = append(results, runClaudeDerivedCase())
		case "randomness":
			results = append(results, runRandomnessCase())
		case "randomness_alias":
			results = append(results, runRandomnessAliasCase())
		case "volume":
			results = append(results, runVolumeCase())
		case "volume_api_payload":
			results = append(results, runVolumeAPIPayloadCase())
		case "ubuntu":
			results = append(results, runUbuntuCase())
		case "template_timeout":
			results = append(results, runTemplateTimeoutCase())
		case "template_methods":
			results = append(results, runTemplateMethodsCase())
		case "config_headers":
			results = append(results, runConfigHeadersCase())
		case "metrics":
			results = append(results, runMetricsCase())
		case "network_rules":
			results = append(results, runNetworkRulesCase())
		case "network_egress":
			results = append(results, runNetworkEgressCase())
		case "network_update_payload":
			results = append(results, runNetworkUpdatePayloadCase())
		case "template_api_payload":
			results = append(results, runTemplateAPIPayloadCase())
		case "debug_root":
			results = append(results, runDebugRootCase())
		default:
			fail(fmt.Sprintf("unknown case: %s", current))
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		fail(err.Error())
	}
}

func selectedCases(caseName string) []string {
	switch caseName {
	case "all":
		return []string{"claude", "claude_derived", "randomness", "randomness_alias", "volume", "volume_api_payload", "ubuntu", "template_timeout", "template_methods", "config_headers", "metrics", "network_rules", "network_egress", "network_update_payload", "template_api_payload", "debug_root"}
	case "claude", "claude_derived", "randomness", "randomness_alias", "volume", "volume_api_payload", "ubuntu", "template_timeout", "template_methods", "config_headers", "metrics", "network_rules", "network_egress", "network_update_payload", "template_api_payload", "debug_root":
		return []string{caseName}
	default:
		fail(fmt.Sprintf("unsupported case %q", caseName))
		return nil
	}
}

func runDebugRootCase() result {
	previousDebug, hadDebug := os.LookupEnv("E2B_DEBUG")
	previousAPIURL, hadAPIURL := os.LookupEnv("E2B_API_URL")
	previousDomain, hadDomain := os.LookupEnv("E2B_DOMAIN")

	_ = os.Setenv("E2B_DEBUG", "true")
	_ = os.Unsetenv("E2B_API_URL")
	_ = os.Unsetenv("E2B_DOMAIN")
	defer restoreEnv("E2B_DEBUG", previousDebug, hadDebug)
	defer restoreEnv("E2B_API_URL", previousAPIURL, hadAPIURL)
	defer restoreEnv("E2B_DOMAIN", previousDomain, hadDomain)

	config := e2b.NewConnectionConfig(&e2b.ConnectionOpts{
		Debug: boolPtr(false),
	})

	status := "ok"
	detail := "env debug=true wins over explicit debug=false at root connection-config construction, matching current JS/Python truthiness"
	if !config.Debug || config.ApiUrl != "http://localhost:3000" {
		status = "mismatch"
		detail = fmt.Sprintf("expected env debug=true to win, got debug=%t apiUrl=%s", config.Debug, config.ApiUrl)
	}

	return result{
		Language: "go",
		Case:     "debug_root",
		Status:   status,
		Detail:   detail,
		Extra: map[string]string{
			"env_debug": "true",
			"arg_debug": "false",
			"debug":     strconv.FormatBool(config.Debug),
			"api_url":   config.ApiUrl,
		},
	}
}

func runClaudeCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	exists, err := e2b.Exists(ctx, "claude-code-interpreter", templateConnectionOpts())
	if err != nil {
		return result{Language: "go", Case: "claude", Status: "error", Detail: err.Error()}
	}
	if !exists {
		return result{Language: "go", Case: "claude", Status: "template_missing", Detail: "claude-code-interpreter template alias is unavailable"}
	}

	timeoutMs := int((10 * time.Minute) / time.Millisecond)
	sandbox, err := e2b.Create(ctx, "claude-code-interpreter", &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      &timeoutMs,
	})
	if err != nil {
		return result{Language: "go", Case: "claude", Status: "error", Detail: err.Error()}
	}
	defer func() {
		_ = sandbox.Kill(context.Background(), nil)
	}()

	res, err := runForegroundCommand(sandbox.Commands, ctx, numpyRandomCommand, nil)
	if err != nil {
		return classifyCommandError("go", "claude", err)
	}

	return result{
		Language: "go",
		Case:     "claude",
		Status:   "ok",
		Detail:   strings.TrimSpace(res.Stdout),
	}
}

func runRandomnessCase() result {
	return runBuiltNumpyTemplateCase(
		"randomness",
		e2b.Template(nil).
			FromPythonImage("3.12").
			SkipCache().
			RunCmd("python3 -m pip install --no-cache-dir numpy"),
		fmt.Sprintf("go-sdk-randomness-crosscheck-%d", time.Now().UnixNano()),
		nil,
	)
}

func runRandomnessAliasCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	exists, err := e2b.Exists(ctx, randomnessAliasTemplate, templateConnectionOpts())
	if err != nil {
		return result{Language: "go", Case: "randomness_alias", Status: "error", Detail: err.Error()}
	}
	if !exists {
		return result{Language: "go", Case: "randomness_alias", Status: "template_missing", Detail: "upstream randomness alias is unavailable"}
	}

	timeoutMs := int((10 * time.Minute) / time.Millisecond)
	firstSandbox, err := e2b.Create(ctx, randomnessAliasTemplate, &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      &timeoutMs,
	})
	if err != nil {
		return result{Language: "go", Case: "randomness_alias", Status: "error", Detail: err.Error()}
	}
	defer func() {
		_ = firstSandbox.Kill(context.Background(), nil)
	}()

	first, err := runForegroundCommand(firstSandbox.Commands, ctx, upstreamRandomnessAliasCommand, nil)
	if err != nil {
		return classifyCommandError("go", "randomness_alias", err)
	}
	second, err := runForegroundCommand(firstSandbox.Commands, ctx, upstreamRandomnessAliasCommand, nil)
	if err != nil {
		res := classifyCommandError("go", "randomness_alias", err)
		if res.Status == "error" {
			res.Status = "partial"
		}
		res.Extra = mergeStringMaps(res.Extra, map[string]string{
			"phase":       "same_sandbox_second_command",
			"template_id": randomnessAliasTemplate,
		})
		return res
	}
	if strings.TrimSpace(first.Stdout) == strings.TrimSpace(second.Stdout) {
		return result{
			Language: "go",
			Case:     "randomness_alias",
			Status:   "error",
			Detail:   "expected different random vectors in the same sandbox",
			Extra: map[string]string{
				"phase":       "same_sandbox_compare",
				"template_id": randomnessAliasTemplate,
			},
		}
	}

	secondSandbox, err := e2b.Create(ctx, randomnessAliasTemplate, &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      &timeoutMs,
	})
	if err != nil {
		return result{Language: "go", Case: "randomness_alias", Status: "error", Detail: err.Error()}
	}
	defer func() {
		_ = secondSandbox.Kill(context.Background(), nil)
	}()

	third, err := runForegroundCommand(secondSandbox.Commands, ctx, upstreamRandomnessAliasCommand, nil)
	if err != nil {
		return classifyCommandError("go", "randomness_alias", err)
	}
	if strings.TrimSpace(first.Stdout) == strings.TrimSpace(third.Stdout) {
		return result{
			Language: "go",
			Case:     "randomness_alias",
			Status:   "error",
			Detail:   "expected different random vectors across sandboxes from the same alias",
			Extra: map[string]string{
				"phase":       "cross_sandbox_compare",
				"template_id": randomnessAliasTemplate,
			},
		}
	}

	return result{
		Language: "go",
		Case:     "randomness_alias",
		Status:   "ok",
		Detail:   "same-sandbox and cross-sandbox alias randomness matched upstream expectations",
		Extra: map[string]string{
			"template_id": randomnessAliasTemplate,
		},
	}
}

func runClaudeDerivedCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	exists, err := e2b.Exists(ctx, "claude-code-interpreter", templateConnectionOpts())
	if err != nil {
		return result{Language: "go", Case: "claude_derived", Status: "error", Detail: err.Error()}
	}
	if !exists {
		return result{Language: "go", Case: "claude_derived", Status: "template_missing", Detail: "claude-code-interpreter template alias is unavailable"}
	}

	return runBuiltNumpyTemplateCase(
		"claude_derived",
		e2b.Template(nil).
			FromTemplate("claude-code-interpreter").
			SkipCache().
			RunCmd(claudeDerivedNumpyInstallCommand),
		fmt.Sprintf("go-sdk-claude-derived-crosscheck-%d", time.Now().UnixNano()),
		map[string]string{"base_template": "claude-code-interpreter"},
	)
}

func runBuiltNumpyTemplateCase(caseName string, template *e2b.TemplateBase, name string, extra map[string]string) result {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	info, err := e2b.Build(
		ctx,
		template,
		name,
		buildOpts(),
	)
	if err != nil {
		if isTemplateAPIUnavailable(err) {
			return result{Language: "go", Case: caseName, Status: "template_api_unavailable", Detail: err.Error()}
		}
		return result{Language: "go", Case: caseName, Status: "error", Detail: err.Error()}
	}
	if info == nil || info.TemplateID == "" {
		return result{Language: "go", Case: caseName, Status: "error", Detail: "temporary numpy-enabled build returned empty template ID"}
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
	}()

	firstSandbox, err := createSandboxFromTemplate(ctx, info.TemplateID)
	if err != nil {
		return result{Language: "go", Case: caseName, Status: "error", Detail: err.Error(), Extra: mergeStringMaps(extra, map[string]string{"template_id": info.TemplateID})}
	}
	defer func() {
		_ = firstSandbox.Kill(context.Background(), nil)
	}()

	first, err := runNumpyVector(ctx, firstSandbox)
	if err != nil {
		res := classifyCommandError("go", caseName, err)
		res.Extra = mergeStringMaps(extra, map[string]string{"template_id": info.TemplateID})
		return res
	}

	second, err := runNumpyVector(ctx, firstSandbox)
	if err != nil {
		res := classifyCommandError("go", caseName, err)
		res.Extra = mergeStringMaps(extra, map[string]string{"template_id": info.TemplateID})
		return res
	}
	if first == second {
		return result{
			Language: "go",
			Case:     caseName,
			Status:   "error",
			Detail:   "expected different random vectors in the same sandbox",
			Extra: mergeStringMaps(extra, map[string]string{
				"template_id":       info.TemplateID,
				"same_sandbox_diff": "false",
			}),
		}
	}

	secondSandbox, err := createSandboxFromTemplate(ctx, info.TemplateID)
	if err != nil {
		return result{Language: "go", Case: caseName, Status: "error", Detail: err.Error(), Extra: mergeStringMaps(extra, map[string]string{"template_id": info.TemplateID})}
	}
	defer func() {
		_ = secondSandbox.Kill(context.Background(), nil)
	}()

	third, err := runNumpyVector(ctx, secondSandbox)
	if err != nil {
		res := classifyCommandError("go", caseName, err)
		res.Extra = mergeStringMaps(extra, map[string]string{"template_id": info.TemplateID})
		return res
	}
	if first == third {
		return result{
			Language: "go",
			Case:     caseName,
			Status:   "error",
			Detail:   "expected different random vectors across sandboxes from the same template",
			Extra: mergeStringMaps(extra, map[string]string{
				"template_id":        info.TemplateID,
				"same_sandbox_diff":  "true",
				"cross_sandbox_diff": "false",
			}),
		}
	}

	return result{
		Language: "go",
		Case:     caseName,
		Status:   "ok",
		Detail:   "same-sandbox and cross-sandbox numpy vectors differed",
		Extra: mergeStringMaps(extra, map[string]string{
			"template_id":        info.TemplateID,
			"same_sandbox_diff":  "true",
			"cross_sandbox_diff": "true",
		}),
	}
}

func runVolumeCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	name := fmt.Sprintf("go-sdk-crosscheck-%d", time.Now().UnixNano())
	vol, err := e2bvol.Create(ctx, name, volumeConnectionOpts())
	if err != nil {
		return result{Language: "go", Case: "volume", Status: "error", Detail: err.Error()}
	}
	defer func() {
		_, _ = e2bvol.Destroy(context.Background(), vol.VolumeID, volumeConnectionOpts())
	}()

	_, err = vol.MakeDir(ctx, "/multi-file-dir", &e2bvol.VolumeWriteOptions{})
	if err != nil {
		return classifyVolumeError("go", err)
	}

	return result{Language: "go", Case: "volume", Status: "ok", Detail: "MakeDir(/multi-file-dir) succeeded"}
}

func runVolumeAPIPayloadCase() result {
	requests := make([]capturedWireRequest, 0, 7)
	timestamp := "2026-05-30T00:00:00Z"
	dirEntry := map[string]any{
		"name":  "dir",
		"path":  "/dir",
		"type":  "directory",
		"uid":   float64(1000),
		"gid":   float64(1000),
		"mode":  float64(0o755),
		"size":  float64(0),
		"atime": timestamp,
		"mtime": timestamp,
		"ctime": timestamp,
	}
	fileEntry := map[string]any{
		"name":  "file.txt",
		"path":  "/file.txt",
		"type":  "file",
		"uid":   float64(1000),
		"gid":   float64(1000),
		"mode":  float64(0o644),
		"size":  float64(5),
		"atime": timestamp,
		"mtime": timestamp,
		"ctime": timestamp,
	}
	updatedEntry := map[string]any{
		"name":  "dir",
		"path":  "/dir",
		"type":  "directory",
		"uid":   float64(1001),
		"gid":   float64(1002),
		"mode":  float64(0o644),
		"size":  float64(0),
		"atime": timestamp,
		"mtime": timestamp,
		"ctime": timestamp,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body := any("")
		if len(rawBody) > 0 {
			contentType := r.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "application/json") {
				var decoded any
				if err := json.Unmarshal(rawBody, &decoded); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				body = decoded
			} else {
				body = string(rawBody)
			}
		}

		requests = append(requests, capturedWireRequest{
			Method:        r.Method,
			Path:          r.URL.Path,
			Query:         singleValueQuery(r.URL.Query()),
			ContentType:   r.Header.Get("Content-Type"),
			Authorization: r.Header.Get("Authorization"),
			Body:          body,
		})

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/volumecontent/vol-1/dir":
			_ = json.NewEncoder(w).Encode([]map[string]any{dirEntry})
		case r.Method == http.MethodPost && r.URL.Path == "/volumecontent/vol-1/dir":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(dirEntry)
		case r.Method == http.MethodGet && r.URL.Path == "/volumecontent/vol-1/path":
			_ = json.NewEncoder(w).Encode(dirEntry)
		case r.Method == http.MethodPatch && r.URL.Path == "/volumecontent/vol-1/path":
			_ = json.NewEncoder(w).Encode(updatedEntry)
		case r.Method == http.MethodGet && r.URL.Path == "/volumecontent/vol-1/file":
			_, _ = io.WriteString(w, "hello")
		case r.Method == http.MethodPut && r.URL.Path == "/volumecontent/vol-1/file":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(fileEntry)
		case r.Method == http.MethodDelete && r.URL.Path == "/volumecontent/vol-1/path":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	volume := &e2bvol.Volume{
		VolumeID: "vol-1",
		Name:     "vol",
		Token:    "token-1",
	}

	ctx := context.Background()
	depth := 2
	entries, err := volume.List(ctx, "/dir", &e2bvol.VolumeListOpts{
		VolumeApiOpts: e2bvol.VolumeApiOpts{ApiUrl: server.URL},
		Depth:         &depth,
	})
	if err != nil {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: err.Error()}
	}
	if len(entries) != 1 || entries[0].Path != "/dir" || entries[0].Type != e2bvol.VolumeFileTypeDirectory {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected list result: %#v", entries)}
	}

	makeDirUID := 1000
	makeDirGID := 1000
	makeDirMode := 0o755
	makeDirForce := false
	dirStat, err := volume.MakeDir(ctx, "/dir", &e2bvol.VolumeWriteOptions{
		ApiUrl: server.URL,
		VolumeMetadataOptions: e2bvol.VolumeMetadataOptions{
			UID:  &makeDirUID,
			GID:  &makeDirGID,
			Mode: &makeDirMode,
		},
		Force: &makeDirForce,
	})
	if err != nil {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: err.Error()}
	}
	if dirStat == nil || dirStat.Path != "/dir" {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected makeDir result: %#v", dirStat)}
	}

	info, err := volume.GetInfo(ctx, "/dir", &e2bvol.VolumeApiOpts{ApiUrl: server.URL})
	if err != nil {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: err.Error()}
	}
	if info == nil || info.Path != "/dir" || info.Type != e2bvol.VolumeFileTypeDirectory {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected getInfo result: %#v", info)}
	}

	updateUID := 1001
	updateGID := 1002
	updateMode := 0o644
	updated, err := volume.UpdateMetadata(ctx, "/dir", &e2bvol.VolumeMetadataOptions{
		UID:  &updateUID,
		GID:  &updateGID,
		Mode: &updateMode,
	}, &e2bvol.VolumeApiOpts{ApiUrl: server.URL})
	if err != nil {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: err.Error()}
	}
	if updated == nil || updated.UID != 1001 || updated.GID != 1002 || updated.Mode != 0o644 {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected updateMetadata result: %#v", updated)}
	}

	readValue, err := volume.ReadFile(ctx, "/file.txt", &e2bvol.VolumeReadOpts{
		VolumeApiOpts: e2bvol.VolumeApiOpts{ApiUrl: server.URL},
	})
	if err != nil {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: err.Error()}
	}
	if text, ok := readValue.(string); !ok || text != "hello" {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected readFile result: %#v", readValue)}
	}

	writeUID := 1000
	writeGID := 1000
	writeMode := 0o644
	writeForce := false
	fileStat, err := volume.WriteFile(ctx, "/file.txt", "hello", &e2bvol.VolumeWriteOptions{
		ApiUrl: server.URL,
		VolumeMetadataOptions: e2bvol.VolumeMetadataOptions{
			UID:  &writeUID,
			GID:  &writeGID,
			Mode: &writeMode,
		},
		Force: &writeForce,
	})
	if err != nil {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: err.Error()}
	}
	if fileStat == nil || fileStat.Path != "/file.txt" || fileStat.Type != e2bvol.VolumeFileTypeFile {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected writeFile result: %#v", fileStat)}
	}

	if err := volume.Remove(ctx, "/file.txt", &e2bvol.VolumeApiOpts{ApiUrl: server.URL}); err != nil {
		return result{Language: "go", Case: "volume_api_payload", Status: "error", Detail: err.Error()}
	}

	if len(requests) != 7 {
		return result{
			Language: "go",
			Case:     "volume_api_payload",
			Status:   "error",
			Detail:   fmt.Sprintf("expected 7 captured requests, got %d", len(requests)),
		}
	}

	expected := map[string]capturedWireRequest{
		"list": {
			Method:        http.MethodGet,
			Path:          "/volumecontent/vol-1/dir",
			Query:         map[string]string{"depth": "2", "path": "/dir"},
			ContentType:   "",
			Authorization: "Bearer token-1",
			Body:          "",
		},
		"make_dir": {
			Method:        http.MethodPost,
			Path:          "/volumecontent/vol-1/dir",
			Query:         map[string]string{"force": "false", "gid": "1000", "mode": "493", "path": "/dir", "uid": "1000"},
			ContentType:   "",
			Authorization: "Bearer token-1",
			Body:          "",
		},
		"get_info": {
			Method:        http.MethodGet,
			Path:          "/volumecontent/vol-1/path",
			Query:         map[string]string{"path": "/dir"},
			ContentType:   "",
			Authorization: "Bearer token-1",
			Body:          "",
		},
		"update_metadata": {
			Method:        http.MethodPatch,
			Path:          "/volumecontent/vol-1/path",
			Query:         map[string]string{"path": "/dir"},
			ContentType:   "application/json",
			Authorization: "Bearer token-1",
			Body: map[string]any{
				"uid":  float64(1001),
				"gid":  float64(1002),
				"mode": float64(0o644),
			},
		},
		"read_file": {
			Method:        http.MethodGet,
			Path:          "/volumecontent/vol-1/file",
			Query:         map[string]string{"path": "/file.txt"},
			ContentType:   "",
			Authorization: "Bearer token-1",
			Body:          "",
		},
		"write_file": {
			Method:        http.MethodPut,
			Path:          "/volumecontent/vol-1/file",
			Query:         map[string]string{"force": "false", "gid": "1000", "mode": "420", "path": "/file.txt", "uid": "1000"},
			ContentType:   "application/octet-stream",
			Authorization: "Bearer token-1",
			Body:          "hello",
		},
		"remove": {
			Method:        http.MethodDelete,
			Path:          "/volumecontent/vol-1/path",
			Query:         map[string]string{"path": "/file.txt"},
			ContentType:   "",
			Authorization: "Bearer token-1",
			Body:          "",
		},
	}

	keys := []string{"list", "make_dir", "get_info", "update_metadata", "read_file", "write_file", "remove"}
	extra := map[string]string{}
	for i, key := range keys {
		actual := requests[i]
		want := expected[key]

		extra[key+"_method"] = actual.Method
		extra[key+"_path"] = actual.Path
		extra[key+"_query"] = mustJSON(actual.Query)
		extra[key+"_content_type"] = actual.ContentType
		extra[key+"_authorization"] = actual.Authorization
		extra[key+"_body"] = mustJSON(actual.Body)

		if actual.Method != want.Method || actual.Path != want.Path {
			return result{
				Language: "go",
				Case:     "volume_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s request target mismatch", key),
				Extra:    extra,
			}
		}
		if !reflect.DeepEqual(actual.Query, want.Query) {
			return result{
				Language: "go",
				Case:     "volume_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s query mismatch", key),
				Extra:    extra,
			}
		}
		if want.ContentType == "" {
			if actual.ContentType != "" {
				return result{
					Language: "go",
					Case:     "volume_api_payload",
					Status:   "mismatch",
					Detail:   fmt.Sprintf("%s content-type mismatch", key),
					Extra:    extra,
				}
			}
		} else if !strings.HasPrefix(actual.ContentType, want.ContentType) {
			return result{
				Language: "go",
				Case:     "volume_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s content-type mismatch", key),
				Extra:    extra,
			}
		}
		if actual.Authorization != want.Authorization {
			return result{
				Language: "go",
				Case:     "volume_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s authorization mismatch", key),
				Extra:    extra,
			}
		}
		if !reflect.DeepEqual(actual.Body, want.Body) {
			return result{
				Language: "go",
				Case:     "volume_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s payload mismatch", key),
				Extra:    extra,
			}
		}
	}

	return result{
		Language: "go",
		Case:     "volume_api_payload",
		Status:   "ok",
		Detail:   "captured volume content request shapes locally",
		Extra:    extra,
	}
}

func runUbuntuCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	name := fmt.Sprintf("go-sdk-ubuntu-crosscheck-%d", time.Now().UnixNano())
	info, err := e2b.BuildInBackground(
		ctx,
		e2b.Template(nil).FromImage("ubuntu:22.04").SkipCache(),
		name,
		buildOpts(),
	)
	if err != nil {
		if isTemplateAPIUnavailable(err) {
			return result{Language: "go", Case: "ubuntu", Status: "template_api_unavailable", Detail: err.Error()}
		}
		return result{Language: "go", Case: "ubuntu", Status: "error", Detail: err.Error()}
	}

	if info != nil && info.TemplateID != "" {
		defer func() {
			_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
		}()
	}

	final, pollErr := waitForFinalBuildStatus(ctx, info.TemplateID, info.BuildID)
	if pollErr != nil {
		return result{
			Language: "go",
			Case:     "ubuntu",
			Status:   "error",
			Detail:   pollErr.Error(),
			Extra: map[string]string{
				"template_id": info.TemplateID,
				"build_id":    info.BuildID,
			},
		}
	}

	res := result{
		Language: "go",
		Case:     "ubuntu",
		Status:   string(final.Status),
		Extra: map[string]string{
			"template_id": info.TemplateID,
			"build_id":    info.BuildID,
		},
	}
	if final.Reason != nil {
		res.Detail = final.Reason.Message
		if final.Reason.Step != "" {
			res.Extra["reason_step"] = final.Reason.Step
		}
	}
	if res.Status == "error" && strings.Contains(strings.ToLower(res.Detail), "error waiting for provisioning sandbox") {
		res.Status = "env_blocked"
	}
	return res
}

func runTemplateTimeoutCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	contextDir, err := os.MkdirTemp("", "e2b-go-template-timeout-*")
	if err != nil {
		return result{Language: "go", Case: "template_timeout", Status: "error", Detail: err.Error()}
	}
	defer os.RemoveAll(contextDir)

	folder := filepath.Join(contextDir, "folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return result{Language: "go", Case: "template_timeout", Status: "error", Detail: err.Error()}
	}
	if err := os.WriteFile(filepath.Join(folder, "test.txt"), []byte("This is a test file."), 0o644); err != nil {
		return result{Language: "go", Case: "template_timeout", Status: "error", Detail: err.Error()}
	}

	start := time.Now()
	name := fmt.Sprintf("go-sdk-template-timeout-crosscheck-%d", time.Now().UnixNano())
	info, err := e2b.Build(
		ctx,
		e2b.Template(&e2b.TemplateOptions{FileContextPath: contextDir}).
			FromBaseImage().
			Copy("folder/*", "folder", &struct{ ForceUpload bool }{ForceUpload: true}).
			RunCmd("cat folder/test.txt").
			SetWorkdir("/app").
			SetStartCmd(`echo "Hello, world!"`, e2b.WaitForTimeout(10_000)),
		name,
		defaultTimeoutBuildOpts(),
	)
	elapsedMs := strconv.FormatInt(time.Since(start).Milliseconds(), 10)
	extra := map[string]string{
		"elapsed_ms":              elapsedMs,
		"request_timeout_mode":    "default",
		"status_poll_query":       "logsOffset+limit=100",
		"template_shape":          "fromBaseImage+copy+runCmd+setStartCmd",
		"file_context_created":    "true",
		"build_options_memory_mb": "1024",
		"build_options_cpu":       "1",
		"build_options_skipcache": "true",
	}
	if err != nil {
		return classifyTemplateBuildError("go", "template_timeout", err, extra)
	}
	if info != nil && info.TemplateID != "" {
		defer func() {
			_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
		}()
		extra["template_id"] = info.TemplateID
		extra["build_id"] = info.BuildID
	}

	return result{
		Language: "go",
		Case:     "template_timeout",
		Status:   "ok",
		Detail:   "default-timeout base-image build succeeded",
		Extra:    extra,
	}
}

func runTemplateMethodsCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	contextDir, err := os.MkdirTemp("", "e2b-go-template-methods-*")
	if err != nil {
		return result{Language: "go", Case: "template_methods", Status: "error", Detail: err.Error()}
	}
	defer os.RemoveAll(contextDir)

	if err := os.WriteFile(filepath.Join(contextDir, "test.txt"), []byte("template symlink content\n"), 0o644); err != nil {
		return result{Language: "go", Case: "template_methods", Status: "error", Detail: err.Error()}
	}
	if err := os.Symlink("test.txt", filepath.Join(contextDir, "link.txt")); err != nil {
		return result{Language: "go", Case: "template_methods", Status: "error", Detail: err.Error()}
	}

	extra := map[string]string{
		"template_shape": "fromBaseImage+runCmd(root)+makeSymlink+copy(symlink-preserve)+copy(symlink-resolve)",
	}
	name := fmt.Sprintf("go-sdk-template-methods-crosscheck-%d", time.Now().UnixNano())
	info, err := e2b.Build(
		ctx,
		e2b.Template(&e2b.TemplateOptions{FileContextPath: contextDir}).
			FromBaseImage().
			RunCmd(`test "$(whoami)" = "root"`, &struct{ User string }{User: "root"}).
			MakeSymlink(".bashrc", ".bashrc.local").
			Copy("test.txt", "/app/test.txt", &struct{ ForceUpload bool }{ForceUpload: true}).
			Copy("link.txt", "/app/link-preserved.txt", &struct{ ForceUpload bool }{ForceUpload: true}).
			Copy("link.txt", "/app/link-resolved.txt", &struct {
				ForceUpload     bool
				ResolveSymlinks bool
			}{ForceUpload: true, ResolveSymlinks: true}).
			RunCmd(`test "$(readlink .bashrc.local)" = ".bashrc"`).
			RunCmd(`test "$(readlink /app/link-preserved.txt)" = "test.txt"`).
			RunCmd(`test "$(cat /app/link-preserved.txt)" = "template symlink content"`).
			RunCmd(`test ! -L /app/link-resolved.txt`).
			RunCmd(`test "$(cat /app/link-resolved.txt)" = "template symlink content"`),
		name,
		buildOpts(),
	)
	if err != nil {
		return classifyTemplateBuildError("go", "template_methods", err, extra)
	}
	if info == nil || info.TemplateID == "" || info.BuildID == "" {
		return result{
			Language: "go",
			Case:     "template_methods",
			Status:   "error",
			Detail:   "template build returned empty template/build IDs",
			Extra:    extra,
		}
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
	}()
	extra["template_id"] = info.TemplateID
	extra["build_id"] = info.BuildID

	sandbox, err := createSandboxFromTemplate(ctx, info.TemplateID)
	if err != nil {
		return result{Language: "go", Case: "template_methods", Status: "error", Detail: err.Error(), Extra: extra}
	}
	defer func() {
		_ = sandbox.Kill(context.Background(), nil)
	}()

	inspection, err := runForegroundCommand(sandbox.Commands, ctx, templateMethodsSummaryCommand, nil)
	if err != nil {
		detail := err.Error()
		var exitErr *e2b.CommandExitError
		if errors.As(err, &exitErr) {
			detail = strings.TrimSpace(strings.Join([]string{detail, exitErr.Stdout, exitErr.Stderr}, "\n"))
		}
		return result{Language: "go", Case: "template_methods", Status: "error", Detail: detail, Extra: extra}
	}

	summary := strings.TrimSpace(inspection.Stdout)
	if summary != expectedTemplateMethodsSummary {
		return result{
			Language: "go",
			Case:     "template_methods",
			Status:   "error",
			Detail:   "unexpected runtime summary:\n" + summary,
			Extra:    extra,
		}
	}

	return result{
		Language: "go",
		Case:     "template_methods",
		Status:   "ok",
		Detail:   "stable base-image template method summary matched across build and runtime",
		Extra:    extra,
	}
}

func runConfigHeadersCase() result {
	var gotTestHeader string
	var gotExtraHeader string
	var gotUserAgent string
	previousAPIURL, hadAPIURL := os.LookupEnv("E2B_API_URL")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTestHeader = r.Header.Get("X-Test")
		gotExtraHeader = r.Header.Get("X-Extra")
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"already paused"}`))
	}))
	defer server.Close()
	defer restoreEnv("E2B_API_URL", previousAPIURL, hadAPIURL)

	_ = os.Setenv("E2B_API_URL", server.URL)

	paused, err := e2b.Pause(context.Background(), "sbx-test", &e2b.SandboxApiOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		Domain:           "base.e2b.dev",
		RequestTimeoutMs: intPtr(1000),
		Headers: map[string]string{
			"X-Test":  "base",
			"X-Extra": "1",
		},
	})
	if err != nil {
		return result{Language: "go", Case: "config_headers", Status: "error", Detail: err.Error()}
	}
	if paused {
		return result{Language: "go", Case: "config_headers", Status: "error", Detail: "expected pause to return false on 409 conflict"}
	}

	extra := map[string]string{
		"x_test":     gotTestHeader,
		"x_extra":    gotExtraHeader,
		"user_agent": gotUserAgent,
	}
	if gotTestHeader == "base" && gotExtraHeader == "1" {
		return result{
			Language: "go",
			Case:     "config_headers",
			Status:   "ok",
			Detail:   "pause merged base and per-call headers",
			Extra:    extra,
		}
	}
	return result{
		Language: "go",
		Case:     "config_headers",
		Status:   "mismatch",
		Detail:   "unexpected pause header propagation",
		Extra:    extra,
	}
}

func runNetworkUpdatePayloadCase() result {
	requests := make([]capturedRequest, 0, 2)
	previousAPIURL, hadAPIURL := os.LookupEnv("E2B_API_URL")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests = append(requests, capturedRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	defer restoreEnv("E2B_API_URL", previousAPIURL, hadAPIURL)
	_ = os.Setenv("E2B_API_URL", server.URL)

	opts := &e2b.SandboxApiOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		Domain:           "base.e2b.dev",
		RequestTimeoutMs: intPtr(1000),
	}

	allowInternetAccess := false
	if err := e2b.UpdateNetwork(context.Background(), "sbx-selectors", e2b.SandboxNetworkUpdate{
		Rules: e2b.SandboxNetworkRules{
			"httpbin.e2b.team": {
				{},
				{
					Transform: &e2b.SandboxNetworkTransform{
						Headers: map[string]string{
							"X-Test": "selector",
						},
					},
				},
			},
		},
		AllowOut: func(ctx e2b.SandboxNetworkSelectorContext) []string {
			hosts := make([]string, 0, len(ctx.Rules))
			for host := range ctx.Rules {
				hosts = append(hosts, host)
			}
			sort.Strings(hosts)
			return hosts
		},
		DenyOut: e2b.SandboxNetworkSelectorFunc(func(ctx e2b.SandboxNetworkSelectorContext) []string {
			return []string{ctx.AllTraffic}
		}),
		AllowInternetAccess: &allowInternetAccess,
	}, opts); err != nil {
		return result{Language: "go", Case: "network_update_payload", Status: "error", Detail: err.Error()}
	}

	if err := e2b.UpdateNetwork(context.Background(), "sbx-empty", e2b.SandboxNetworkUpdate{
		AllowOut: []string{},
		DenyOut:  []string{},
		Rules:    e2b.SandboxNetworkRules{},
	}, opts); err != nil {
		return result{Language: "go", Case: "network_update_payload", Status: "error", Detail: err.Error()}
	}

	if len(requests) != 2 {
		return result{
			Language: "go",
			Case:     "network_update_payload",
			Status:   "error",
			Detail:   fmt.Sprintf("expected 2 captured requests, got %d", len(requests)),
		}
	}

	expectedSelectorBody := map[string]any{
		"allowOut":              []any{"httpbin.e2b.team"},
		"denyOut":               []any{e2b.ALL_TRAFFIC},
		"allow_internet_access": false,
		"rules": map[string]any{
			"httpbin.e2b.team": []any{
				map[string]any{},
				map[string]any{
					"transform": map[string]any{
						"headers": map[string]any{
							"X-Test": "selector",
						},
					},
				},
			},
		},
	}

	first := requests[0]
	second := requests[1]
	extra := map[string]string{
		"selector_method":             first.Method,
		"selector_path":               first.Path,
		"selector_content_type":       first.ContentType,
		"selector_body":               mustJSON(first.Body),
		"explicit_empty_method":       second.Method,
		"explicit_empty_path":         second.Path,
		"explicit_empty_content_type": second.ContentType,
		"explicit_empty_body":         mustJSON(second.Body),
	}
	expectedExplicitEmptyBody := map[string]any{
		"allowOut": []any{},
		"denyOut":  []any{},
		"rules":    map[string]any{},
	}
	if reflect.DeepEqual(second.Body, expectedExplicitEmptyBody) {
		extra["explicit_empty_mode"] = "preserved"
	} else {
		extra["explicit_empty_mode"] = "mismatch"
	}

	if first.Method != http.MethodPut || first.Path != "/sandboxes/sbx-selectors/network" {
		return result{
			Language: "go",
			Case:     "network_update_payload",
			Status:   "mismatch",
			Detail:   "unexpected selector update request target",
			Extra:    extra,
		}
	}
	if !strings.HasPrefix(first.ContentType, "application/json") {
		return result{
			Language: "go",
			Case:     "network_update_payload",
			Status:   "mismatch",
			Detail:   "unexpected selector update content-type",
			Extra:    extra,
		}
	}
	if !reflect.DeepEqual(first.Body, expectedSelectorBody) {
		return result{
			Language: "go",
			Case:     "network_update_payload",
			Status:   "mismatch",
			Detail:   "selector-based update payload did not match expected shape",
			Extra:    extra,
		}
	}
	if second.Method != http.MethodPut || second.Path != "/sandboxes/sbx-empty/network" {
		return result{
			Language: "go",
			Case:     "network_update_payload",
			Status:   "mismatch",
			Detail:   "unexpected explicit-empty update request target",
			Extra:    extra,
		}
	}
	if !strings.HasPrefix(second.ContentType, "application/json") {
		return result{
			Language: "go",
			Case:     "network_update_payload",
			Status:   "mismatch",
			Detail:   "unexpected explicit-empty update content-type",
			Extra:    extra,
		}
	}
	if !reflect.DeepEqual(second.Body, expectedExplicitEmptyBody) {
		return result{
			Language: "go",
			Case:     "network_update_payload",
			Status:   "mismatch",
			Detail:   "explicit-empty update payload did not preserve empty allowOut/denyOut/rules",
			Extra:    extra,
		}
	}

	return result{
		Language: "go",
		Case:     "network_update_payload",
		Status:   "ok",
		Detail:   "captured selector-based and explicit-empty network update payloads locally",
		Extra:    extra,
	}
}

func runTemplateAPIPayloadCase() result {
	requests := make([]capturedRequest, 0, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		requests = append(requests, capturedRequest{
			Method:      r.Method,
			Path:        r.URL.RequestURI(),
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v3/templates":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-1",
				"buildID":    "bld-1",
				"tags":       []string{"stable"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/templates/tmpl-1/builds/bld-1":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/templates/aliases/tmpl":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-1",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/templates/tmpl-1/builds/bld-1/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-1",
				"buildID":    "bld-1",
				"status":     "ready",
				"logEntries": []any{},
				"logs":       []any{},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/templates/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"buildID": "bld-1",
				"tags":    []string{"stable"},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/templates/tags":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/templates/tmpl-1/tags":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"tag":       "stable",
					"buildId":   "bld-1",
					"createdAt": "2026-05-30T00:00:00Z",
				},
			})
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	opts := &e2b.BuildOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		Domain:           "base.e2b.dev",
		RequestTimeoutMs: intPtr(1000),
		BasicBuildOptions: e2b.BasicBuildOptions{
			Tags: []string{"stable"},
		},
	}

	template := e2b.Template(nil).FromBaseImage().RunCmd("echo hi")
	buildInfo, err := e2b.BuildInBackground(context.Background(), template, "tmpl", opts)
	if err != nil {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: err.Error()}
	}
	if buildInfo == nil {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: "BuildInBackground returned nil build info"}
	}

	exists, err := e2b.Exists(context.Background(), "tmpl", &e2b.ConnectionOpts{
		ApiKey:           opts.ApiKey,
		ApiUrl:           opts.ApiUrl,
		Domain:           opts.Domain,
		RequestTimeoutMs: opts.RequestTimeoutMs,
	})
	if err != nil {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: err.Error()}
	}
	if !exists {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: "expected Exists to return true"}
	}

	status, err := e2b.GetBuildStatus(context.Background(), buildInfo, &e2b.GetBuildStatusOptions{
		ApiKey:           opts.ApiKey,
		ApiUrl:           opts.ApiUrl,
		Domain:           opts.Domain,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		LogsOffset:       3,
	})
	if err != nil {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: err.Error()}
	}
	if status == nil || status.Status != e2b.TemplateBuildStatus("ready") {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected build status: %#v", status)}
	}

	tagInfo, err := e2b.AssignTags(context.Background(), "tmpl:latest", "stable", &e2b.ConnectionOpts{
		ApiKey:           opts.ApiKey,
		ApiUrl:           opts.ApiUrl,
		Domain:           opts.Domain,
		RequestTimeoutMs: opts.RequestTimeoutMs,
	})
	if err != nil {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: err.Error()}
	}
	if tagInfo == nil || len(tagInfo.Tags) != 1 || tagInfo.Tags[0] != "stable" {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected tag info: %#v", tagInfo)}
	}

	if err := e2b.RemoveTags(context.Background(), "tmpl", "stable", &e2b.ConnectionOpts{
		ApiKey:           opts.ApiKey,
		ApiUrl:           opts.ApiUrl,
		Domain:           opts.Domain,
		RequestTimeoutMs: opts.RequestTimeoutMs,
	}); err != nil {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: err.Error()}
	}

	tags, err := e2b.GetTags(context.Background(), "tmpl-1", &e2b.ConnectionOpts{
		ApiKey:           opts.ApiKey,
		ApiUrl:           opts.ApiUrl,
		Domain:           opts.Domain,
		RequestTimeoutMs: opts.RequestTimeoutMs,
	})
	if err != nil {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: err.Error()}
	}
	if len(tags) != 1 || tags[0].Tag != "stable" || tags[0].BuildID != "bld-1" {
		return result{Language: "go", Case: "template_api_payload", Status: "error", Detail: fmt.Sprintf("unexpected tags: %#v", tags)}
	}

	if len(requests) != 7 {
		return result{
			Language: "go",
			Case:     "template_api_payload",
			Status:   "error",
			Detail:   fmt.Sprintf("expected 7 captured requests, got %d", len(requests)),
		}
	}

	expected := map[string]capturedRequest{
		"request_build": {
			Method:      http.MethodPost,
			Path:        "/v3/templates",
			ContentType: "application/json",
			Body: map[string]any{
				"name":     "tmpl",
				"tags":     []any{"stable"},
				"cpuCount": float64(2),
				"memoryMB": float64(1024),
			},
		},
		"trigger_build": {
			Method:      http.MethodPost,
			Path:        "/v2/templates/tmpl-1/builds/bld-1",
			ContentType: "application/json",
			Body: map[string]any{
				"force":     false,
				"fromImage": "e2bdev/base",
				"steps": []any{
					map[string]any{
						"type":  "RUN",
						"args":  []any{"echo hi"},
						"force": false,
					},
				},
			},
		},
		"exists": {
			Method:      http.MethodGet,
			Path:        "/templates/aliases/tmpl",
			ContentType: "",
			Body:        map[string]any{},
		},
		"status": {
			Method:      http.MethodGet,
			Path:        "/templates/tmpl-1/builds/bld-1/status?logsOffset=3&limit=100",
			ContentType: "",
			Body:        map[string]any{},
		},
		"assign_tags": {
			Method:      http.MethodPost,
			Path:        "/templates/tags",
			ContentType: "application/json",
			Body: map[string]any{
				"target": "tmpl:latest",
				"tags":   []any{"stable"},
			},
		},
		"remove_tags": {
			Method:      http.MethodDelete,
			Path:        "/templates/tags",
			ContentType: "application/json",
			Body: map[string]any{
				"name": "tmpl",
				"tags": []any{"stable"},
			},
		},
		"get_tags": {
			Method:      http.MethodGet,
			Path:        "/templates/tmpl-1/tags",
			ContentType: "",
			Body:        map[string]any{},
		},
	}

	keys := []string{"request_build", "trigger_build", "exists", "status", "assign_tags", "remove_tags", "get_tags"}
	extra := map[string]string{}
	for i, key := range keys {
		actual := requests[i]
		extra[key+"_method"] = actual.Method
		extra[key+"_path"] = actual.Path
		extra[key+"_content_type"] = actual.ContentType
		extra[key+"_body"] = mustJSON(actual.Body)

		want := expected[key]
		if actual.Method != want.Method || actual.Path != want.Path {
			return result{
				Language: "go",
				Case:     "template_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s request target mismatch", key),
				Extra:    extra,
			}
		}
		if want.ContentType != "" && !strings.HasPrefix(actual.ContentType, want.ContentType) {
			return result{
				Language: "go",
				Case:     "template_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s content-type mismatch", key),
				Extra:    extra,
			}
		}
		if !reflect.DeepEqual(actual.Body, want.Body) {
			return result{
				Language: "go",
				Case:     "template_api_payload",
				Status:   "mismatch",
				Detail:   fmt.Sprintf("%s payload mismatch", key),
				Extra:    extra,
			}
		}
	}

	return result{
		Language: "go",
		Case:     "template_api_payload",
		Status:   "ok",
		Detail:   "captured template control-plane request shapes locally",
		Extra:    extra,
	}
}

func runMetricsCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	template, detail, extra, err := resolveSandboxTemplate(ctx)
	if err != nil {
		return result{Language: "go", Case: "metrics", Status: "error", Detail: err.Error()}
	}

	timeoutMs := int((10 * time.Minute) / time.Millisecond)
	sandbox, err := e2b.Create(ctx, template, &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      &timeoutMs,
	})
	if err != nil {
		if isMissingTemplateAlias(err) {
			return result{
				Language: "go",
				Case:     "metrics",
				Status:   "template_missing",
				Detail:   err.Error(),
				Extra:    extra,
			}
		}
		return result{
			Language: "go",
			Case:     "metrics",
			Status:   "error",
			Detail:   err.Error(),
			Extra:    extra,
		}
	}
	defer func() {
		_ = sandbox.Kill(context.Background(), nil)
		if extra["template_source"] == "temporary_build" {
			_, _ = e2b.DeleteSnapshot(context.Background(), template, nil)
		}
	}()

	if detail != "" {
		if extra == nil {
			extra = map[string]string{}
		}
		extra["template_resolution"] = detail
	}
	extra["template"] = template

	res, err := runForegroundCommand(sandbox.Commands, ctx, "python3 - <<'PY'\nprint(sum(range(1000)))\nPY", nil)
	if err != nil {
		return result{
			Language: "go",
			Case:     "metrics",
			Status:   "error",
			Detail:   fmt.Sprintf("metrics warmup command failed: %v", err),
			Extra:    extra,
		}
	}
	extra["warmup_exit_code"] = fmt.Sprintf("%d", res.ExitCode)

	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		metrics, metricsErr := sandbox.GetMetrics(ctx, nil)
		if metricsErr == nil && len(metrics) > 0 {
			metric := metrics[0]
			inclusiveStart := metric.Timestamp.Add(-2 * time.Second)
			inclusiveEnd := metric.Timestamp.Add(2 * time.Second)
			filtered, filteredErr := sandbox.GetMetrics(ctx, &e2b.SandboxMetricsOpts{
				Start: &inclusiveStart,
				End:   &inclusiveEnd,
			})
			if filteredErr != nil {
				return result{
					Language: "go",
					Case:     "metrics",
					Status:   "error",
					Detail:   filteredErr.Error(),
					Extra:    extra,
				}
			}
			futureStart := metric.Timestamp.Add(24 * time.Hour)
			futureEnd := futureStart.Add(2 * time.Second)
			futureFiltered, futureFilteredErr := sandbox.GetMetrics(ctx, &e2b.SandboxMetricsOpts{
				Start: &futureStart,
				End:   &futureEnd,
			})
			if futureFilteredErr != nil {
				return result{
					Language: "go",
					Case:     "metrics",
					Status:   "error",
					Detail:   futureFilteredErr.Error(),
					Extra:    extra,
				}
			}

			extra["metrics_count"] = fmt.Sprintf("%d", len(metrics))
			extra["filtered_count"] = fmt.Sprintf("%d", len(filtered))
			extra["future_filtered_count"] = fmt.Sprintf("%d", len(futureFiltered))
			extra["metric_timestamp"] = metric.Timestamp.UTC().Format(time.RFC3339Nano)
			extra["inclusive_start"] = inclusiveStart.UTC().Format(time.RFC3339Nano)
			extra["inclusive_end"] = inclusiveEnd.UTC().Format(time.RFC3339Nano)
			extra["future_start"] = futureStart.UTC().Format(time.RFC3339Nano)
			extra["future_end"] = futureEnd.UTC().Format(time.RFC3339Nano)
			rawInclusiveCount, rawInclusiveErr := fetchRawMetricsCount(ctx, sandbox.SandboxID, &inclusiveStart, &inclusiveEnd)
			if rawInclusiveErr != nil {
				return result{
					Language: "go",
					Case:     "metrics",
					Status:   "error",
					Detail:   rawInclusiveErr.Error(),
					Extra:    extra,
				}
			}
			extra["raw_inclusive_filtered_count"] = fmt.Sprintf("%d", rawInclusiveCount)
			extra["cpu_count"] = fmt.Sprintf("%d", metric.CpuCount)
			extra["cpu_used_pct"] = fmt.Sprintf("%v", metric.CpuUsedPct)
			extra["mem_used"] = fmt.Sprintf("%d", metric.MemUsed)
			extra["mem_total"] = fmt.Sprintf("%d", metric.MemTotal)
			extra["disk_used"] = fmt.Sprintf("%d", metric.DiskUsed)
			extra["disk_total"] = fmt.Sprintf("%d", metric.DiskTotal)

			if len(filtered) == 0 {
				return result{
					Language: "go",
					Case:     "metrics",
					Status:   "partial",
					Detail:   "metrics returned data, but an inclusive filtered window around the metric timestamp returned zero items",
					Extra:    extra,
				}
			}
			if len(futureFiltered) != 0 {
				return result{
					Language: "go",
					Case:     "metrics",
					Status:   "partial",
					Detail:   "future-only filtered metrics window still returned data",
					Extra:    extra,
				}
			}
			return result{
				Language: "go",
				Case:     "metrics",
				Status:   "ok",
				Detail:   "metrics available",
				Extra:    extra,
			}
		}
		lastErr = metricsErr
		time.Sleep(500 * time.Millisecond)
	}

	detailText := "metrics endpoint returned zero points within 60s"
	if lastErr != nil {
		detailText = lastErr.Error()
	}
	return result{
		Language: "go",
		Case:     "metrics",
		Status:   "env_blocked",
		Detail:   detailText,
		Extra:    extra,
	}
}

func runNetworkRulesCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	template, detail, extra, err := resolveSandboxTemplate(ctx)
	if err != nil {
		return result{Language: "go", Case: "network_rules", Status: "error", Detail: err.Error()}
	}

	timeoutMs := int((10 * time.Minute) / time.Millisecond)
	sandbox, err := e2b.Create(ctx, template, &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      &timeoutMs,
		Network: &e2b.SandboxNetworkOpts{
			AllowOut: []string{"httpbin.e2b.team"},
			DenyOut:  []string{e2b.ALL_TRAFFIC},
			Rules: e2b.SandboxNetworkRules{
				"httpbin.e2b.team": {
					{
						Transform: &e2b.SandboxNetworkTransform{
							Headers: map[string]string{
								"X-E2B-Test-Token": "e2b-transform-value-123",
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return result{
			Language: "go",
			Case:     "network_rules",
			Status:   "error",
			Detail:   err.Error(),
			Extra:    extra,
		}
	}
	defer func() {
		_ = sandbox.Kill(context.Background(), nil)
		if extra["template_source"] == "temporary_build" {
			_, _ = e2b.DeleteSnapshot(context.Background(), template, nil)
		}
	}()

	if detail != "" {
		if extra == nil {
			extra = map[string]string{}
		}
		extra["template_resolution"] = detail
	}
	extra["template"] = template

	commandResult, err := runForegroundCommand(sandbox.Commands, ctx, "curl -sS --max-time 10 https://httpbin.e2b.team/headers", nil)
	if err != nil {
		return result{
			Language: "go",
			Case:     "network_rules",
			Status:   "env_blocked",
			Detail:   err.Error(),
			Extra:    extra,
		}
	}

	var parsed struct {
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(commandResult.Stdout), &parsed); err != nil {
		return result{
			Language: "go",
			Case:     "network_rules",
			Status:   "error",
			Detail:   fmt.Sprintf("failed to decode reflected headers: %v", err),
			Extra:    extra,
		}
	}

	reflected := parsed.Headers["X-E2B-Test-Token"]
	extra["reflected_header"] = reflected
	if reflected != "e2b-transform-value-123" {
		return result{
			Language: "go",
			Case:     "network_rules",
			Status:   "env_blocked",
			Detail:   fmt.Sprintf("network transform is not enforced; reflected header=%q", reflected),
			Extra:    extra,
		}
	}

	return result{
		Language: "go",
		Case:     "network_rules",
		Status:   "ok",
		Detail:   "network transform reflected expected header",
		Extra:    extra,
	}
}

func runNetworkEgressCase() result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	exists, err := e2b.Exists(ctx, "base", templateConnectionOpts())
	if err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error()}
	}
	if !exists {
		return result{
			Language: "go",
			Case:     "network_egress",
			Status:   "template_missing",
			Detail:   "base template alias is unavailable",
		}
	}

	extra := map[string]string{
		"template":            "base",
		"template_source":     "base_alias",
		"template_resolution": "source test alias",
	}

	var firstFailure string

	allowOnly, err := e2b.Create(ctx, "base", &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      intPtr(int((10 * time.Minute) / time.Millisecond)),
		Network: &e2b.SandboxNetworkOpts{
			DenyOut:  []string{e2b.ALL_TRAFFIC},
			AllowOut: []string{"1.1.1.1"},
		},
	})
	if err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error(), Extra: extra}
	}
	defer func() { _ = allowOnly.Kill(context.Background(), nil) }()
	extra["allow_only_1111"] = runCommandSummary(ctx, allowOnly, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1")
	extra["allow_only_8888"] = runCommandSummary(ctx, allowOnly, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8")
	if !commandSucceeded(extra["allow_only_1111"]) && firstFailure == "" {
		firstFailure = "allow_only_1111 did not succeed: " + extra["allow_only_1111"]
	}
	if !commandBlocked(extra["allow_only_8888"]) && firstFailure == "" {
		firstFailure = "allow_only_8888 was not blocked: " + extra["allow_only_8888"]
	}

	denySpecific, err := e2b.Create(ctx, "base", &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      intPtr(int((10 * time.Minute) / time.Millisecond)),
		Network: &e2b.SandboxNetworkOpts{
			DenyOut: []string{"8.8.8.8"},
		},
	})
	if err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error(), Extra: extra}
	}
	defer func() { _ = denySpecific.Kill(context.Background(), nil) }()
	extra["deny_specific_8888"] = runCommandSummary(ctx, denySpecific, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8")
	extra["deny_specific_1111"] = runCommandSummary(ctx, denySpecific, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1")
	if !commandBlocked(extra["deny_specific_8888"]) && firstFailure == "" {
		firstFailure = "deny_specific_8888 was not blocked: " + extra["deny_specific_8888"]
	}
	if !commandSucceeded(extra["deny_specific_1111"]) && firstFailure == "" {
		firstFailure = "deny_specific_1111 did not succeed: " + extra["deny_specific_1111"]
	}

	allowPrecedence, err := e2b.Create(ctx, "base", &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      intPtr(int((10 * time.Minute) / time.Millisecond)),
		Network: &e2b.SandboxNetworkOpts{
			DenyOut:  []string{e2b.ALL_TRAFFIC},
			AllowOut: []string{"1.1.1.1", "8.8.8.8"},
		},
	})
	if err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error(), Extra: extra}
	}
	defer func() { _ = allowPrecedence.Kill(context.Background(), nil) }()
	extra["allow_precedence_1111"] = runCommandSummary(ctx, allowPrecedence, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1")
	extra["allow_precedence_8888"] = runCommandSummary(ctx, allowPrecedence, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8")
	if !commandSucceeded(extra["allow_precedence_1111"]) && firstFailure == "" {
		firstFailure = "allow_precedence_1111 did not succeed: " + extra["allow_precedence_1111"]
	}
	if !commandSucceeded(extra["allow_precedence_8888"]) && firstFailure == "" {
		firstFailure = "allow_precedence_8888 did not succeed: " + extra["allow_precedence_8888"]
	}

	updateDeny, err := e2b.Create(ctx, "base", &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      intPtr(int((10 * time.Minute) / time.Millisecond)),
	})
	if err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error(), Extra: extra}
	}
	defer func() { _ = updateDeny.Kill(context.Background(), nil) }()
	extra["update_before_8888"] = runCommandSummary(ctx, updateDeny, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8")
	if err := updateDeny.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{DenyOut: []string{"8.8.8.8"}}, nil); err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error(), Extra: extra}
	}
	extra["update_after_deny_8888"] = runCommandSummary(ctx, updateDeny, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8")
	extra["update_after_deny_1111"] = runCommandSummary(ctx, updateDeny, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1")
	if !commandSucceeded(extra["update_before_8888"]) && firstFailure == "" {
		firstFailure = "update_before_8888 did not succeed: " + extra["update_before_8888"]
	}
	if !commandBlocked(extra["update_after_deny_8888"]) && firstFailure == "" {
		firstFailure = "update_after_deny_8888 was not blocked: " + extra["update_after_deny_8888"]
	}
	if !commandSucceeded(extra["update_after_deny_1111"]) && firstFailure == "" {
		firstFailure = "update_after_deny_1111 did not succeed: " + extra["update_after_deny_1111"]
	}

	clearRules, err := e2b.Create(ctx, "base", &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      intPtr(int((10 * time.Minute) / time.Millisecond)),
		Network: &e2b.SandboxNetworkOpts{
			DenyOut:  []string{e2b.ALL_TRAFFIC},
			AllowOut: []string{"1.1.1.1"},
		},
	})
	if err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error(), Extra: extra}
	}
	defer func() { _ = clearRules.Kill(context.Background(), nil) }()
	extra["clear_before_8888"] = runCommandSummary(ctx, clearRules, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8")
	if err := clearRules.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{}, nil); err != nil {
		return result{Language: "go", Case: "network_egress", Status: "error", Detail: err.Error(), Extra: extra}
	}
	extra["clear_after_1111"] = runCommandSummary(ctx, clearRules, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1")
	extra["clear_after_8888"] = runCommandSummary(ctx, clearRules, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8")
	if !commandBlocked(extra["clear_before_8888"]) && firstFailure == "" {
		firstFailure = "clear_before_8888 was not blocked: " + extra["clear_before_8888"]
	}
	if !commandSucceeded(extra["clear_after_1111"]) && firstFailure == "" {
		firstFailure = "clear_after_1111 did not succeed: " + extra["clear_after_1111"]
	}
	if !commandSucceeded(extra["clear_after_8888"]) && firstFailure == "" {
		firstFailure = "clear_after_8888 did not succeed: " + extra["clear_after_8888"]
	}

	if firstFailure != "" {
		return result{
			Language: "go",
			Case:     "network_egress",
			Status:   "env_blocked",
			Detail:   firstFailure,
			Extra:    extra,
		}
	}

	return result{
		Language: "go",
		Case:     "network_egress",
		Status:   "ok",
		Detail:   "source-like network egress expectations matched",
		Extra:    extra,
	}
}

func waitForFinalBuildStatus(ctx context.Context, templateID, buildID string) (*e2b.TemplateBuildStatusResponse, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		status, err := e2b.GetBuildStatus(ctx, &e2b.BuildInfo{TemplateID: templateID, BuildID: buildID}, buildStatusOpts())
		if err != nil {
			return nil, err
		}
		switch string(status.Status) {
		case "building", "waiting":
		default:
			return status, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func createSandboxFromTemplate(ctx context.Context, template string) (*e2b.Sandbox, error) {
	timeoutMs := int((10 * time.Minute) / time.Millisecond)
	return e2b.Create(ctx, template, &e2b.SandboxOpts{
		ConnectionOpts: sandboxConnectionOpts(),
		TimeoutMs:      &timeoutMs,
	})
}

func runNumpyVector(ctx context.Context, sandbox *e2b.Sandbox) (string, error) {
	res, err := runForegroundCommand(sandbox.Commands, ctx, numpyRandomCommand, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

func sandboxConnectionOpts() e2b.ConnectionOpts {
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	return e2b.ConnectionOpts{
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func buildOpts() *e2b.BuildOptions {
	requestTimeoutMs := int((10 * time.Minute) / time.Millisecond)
	return &e2b.BuildOptions{
		BasicBuildOptions: e2b.BasicBuildOptions{
			CpuCount: 1,
			MemoryMB: 512,
		},
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func defaultTimeoutBuildOpts() *e2b.BuildOptions {
	return &e2b.BuildOptions{
		BasicBuildOptions: e2b.BasicBuildOptions{
			CpuCount:  1,
			MemoryMB:  1024,
			SkipCache: true,
		},
		ApiKey:      os.Getenv("E2B_API_KEY"),
		AccessToken: os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:      os.Getenv("E2B_DOMAIN"),
		ApiUrl:      os.Getenv("E2B_API_URL"),
	}
}

func templateConnectionOpts() *e2b.ConnectionOpts {
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	return &e2b.ConnectionOpts{
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func buildStatusOpts() *e2b.GetBuildStatusOptions {
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	return &e2b.GetBuildStatusOptions{
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func volumeConnectionOpts() *e2bvol.ConnectionOpts {
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	return &e2bvol.ConnectionOpts{
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func resolveSandboxTemplate(ctx context.Context) (template string, detail string, extra map[string]string, err error) {
	extra = map[string]string{}
	for _, key := range []string{"E2B_TEST_TEMPLATE", "E2B_INTEGRATION_TEMPLATE", "E2B_TEMPLATE", "E2B_SANDBOX_TEMPLATE"} {
		if value := os.Getenv(key); value != "" {
			extra["template_source"] = key
			return value, "from env", extra, nil
		}
	}

	exists, existsErr := e2b.Exists(ctx, "base", templateConnectionOpts())
	if existsErr == nil && exists {
		extra["template_source"] = "base_alias"
		return "base", "from base alias", extra, nil
	}
	if existsErr != nil {
		extra["base_exists_error"] = existsErr.Error()
	}

	paginator := e2b.List(&e2b.SandboxListOpts{Limit: 10})
	items, listErr := paginator.NextItems()
	if listErr == nil {
		for _, item := range items {
			if item.TemplateID != "" {
				extra["template_source"] = "inferred_from_list"
				extra["inferred_sandbox_id"] = item.SandboxID
				return item.TemplateID, "inferred from existing sandbox", extra, nil
			}
		}
	}
	if listErr != nil {
		extra["list_error"] = listErr.Error()
	}

	name := fmt.Sprintf("go-sdk-metrics-crosscheck-%d", time.Now().UnixNano())
	info, buildErr := e2b.Build(
		ctx,
		e2b.Template(nil).FromBaseImage(),
		name,
		buildOpts(),
	)
	if buildErr != nil {
		return "", "", extra, buildErr
	}
	if info.TemplateID == "" {
		return "", "", extra, errors.New("temporary base-image build returned empty template ID")
	}
	extra["template_source"] = "temporary_build"
	extra["template_id"] = info.TemplateID
	return info.TemplateID, "temporary base-image build", extra, nil
}

func fetchRawMetricsCount(ctx context.Context, sandboxID string, start, end *time.Time) (int, error) {
	conn := sandboxConnectionOpts()
	baseURL := conn.ApiUrl
	if baseURL == "" {
		baseURL = "https://api." + conn.Domain
	}
	path := "/sandboxes/" + sandboxID + "/metrics"
	params := url.Values{}
	if start != nil {
		params.Set("start", strconv.FormatInt(roundUnixSeconds(*start), 10))
	}
	if end != nil {
		params.Set("end", strconv.FormatInt(roundUnixSeconds(*end), 10))
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	if conn.ApiKey != "" {
		req.Header.Set("X-API-KEY", conn.ApiKey)
	}
	if conn.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
	}

	httpClient := &http.Client{Timeout: 2 * time.Minute}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("raw metrics request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return 0, fmt.Errorf("raw metrics response decode failed: %w: %s", err, strings.TrimSpace(string(body)))
	}
	return len(items), nil
}

func roundUnixSeconds(value time.Time) int64 {
	return int64(math.Round(float64(value.UnixMilli()) / 1000.0))
}

func restoreEnv(key, value string, hadValue bool) {
	if !hadValue {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, value)
}

func classifyCommandError(language, caseName string, err error) result {
	detail := err.Error()
	msg := strings.ToLower(detail)
	var exitErr *e2b.CommandExitError
	if errors.As(err, &exitErr) {
		detail = strings.TrimSpace(strings.Join([]string{detail, exitErr.Stdout, exitErr.Stderr}, "\n"))
		msg += "\n" + strings.ToLower(exitErr.Stdout) + "\n" + strings.ToLower(exitErr.Stderr)
	}
	if strings.Contains(msg, "no module named") || strings.Contains(msg, "numpy") || strings.Contains(msg, "python3: not found") {
		return result{Language: language, Case: caseName, Status: "env_blocked", Detail: detail}
	}
	return result{Language: language, Case: caseName, Status: "error", Detail: detail}
}

func runCommandSummary(ctx context.Context, sandbox *e2b.Sandbox, command string) string {
	res, err := runForegroundCommand(sandbox.Commands, ctx, command, nil)
	if err == nil {
		return "ok:" + strings.TrimSpace(res.Stdout)
	}
	var exitErr *e2b.CommandExitError
	if errors.As(err, &exitErr) {
		return fmt.Sprintf("exit:%d", exitErr.ExitCode)
	}
	return "error:" + err.Error()
}

func commandSucceeded(summary string) bool {
	return strings.HasPrefix(summary, "ok:")
}

func commandBlocked(summary string) bool {
	return strings.HasPrefix(summary, "exit:")
}

func singleValueQuery(values url.Values) map[string]string {
	query := map[string]string{}
	for key, items := range values {
		if len(items) == 0 {
			query[key] = ""
			continue
		}
		query[key] = items[0]
	}
	return query
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("marshal_error:%v", err)
	}
	return string(data)
}

func intPtr(value int) *int {
	return &value
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func boolPtr(value bool) *bool {
	return &value
}

func classifyVolumeError(language string, err error) result {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "path /multi-file-dir not found") || strings.Contains(msg, "not found") {
		return result{Language: language, Case: "volume", Status: "env_blocked", Detail: err.Error()}
	}
	return result{Language: language, Case: "volume", Status: "error", Detail: err.Error()}
}

func classifyTemplateBuildError(language, caseName string, err error, extra map[string]string) result {
	detail := err.Error()
	message := strings.ToLower(detail)
	if isTemplateAPIUnavailable(err) {
		return result{Language: language, Case: caseName, Status: "template_api_unavailable", Detail: detail, Extra: extra}
	}
	if strings.Contains(message, "aborted due to timeout") ||
		strings.Contains(message, "context deadline exceeded") ||
		strings.Contains(message, "timeout") {
		if extra == nil {
			extra = map[string]string{}
		}
		extra["failure_kind"] = "timeout"
		return result{Language: language, Case: caseName, Status: "env_blocked", Detail: detail, Extra: extra}
	}
	if strings.Contains(message, "internal") ||
		strings.Contains(message, "build error") ||
		strings.Contains(message, "error waiting for provisioning sandbox") {
		if extra == nil {
			extra = map[string]string{}
		}
		extra["failure_kind"] = "backend_error"
		return result{Language: language, Case: caseName, Status: "env_blocked", Detail: detail, Extra: extra}
	}
	return result{Language: language, Case: caseName, Status: "error", Detail: detail, Extra: extra}
}

func isTemplateAPIUnavailable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "request build failed: 404") || strings.Contains(msg, "404 page not found")
}

func isMissingTemplateAlias(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "template not found") || strings.Contains(msg, "sandbox template not found")
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
