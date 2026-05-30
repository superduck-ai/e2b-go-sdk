package template

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

// requestBuildRequest is the request body for POST /v3/templates.
type requestBuildRequest struct {
	Name     string   `json:"name"`
	Tags     []string `json:"tags,omitempty"`
	CpuCount int      `json:"cpuCount,omitempty"`
	MemoryMB int      `json:"memoryMB,omitempty"`
}

// requestBuildResponse is the response from POST /v3/templates.
type requestBuildResponse struct {
	TemplateID string   `json:"templateID"`
	BuildID    string   `json:"buildID"`
	Tags       []string `json:"tags"`
}

// triggerBuildRequest is the request body for POST /v2/templates/{templateID}/builds/{buildID}.
type triggerBuildRequest = triggerBuildTemplate

type triggerBuildTemplate struct {
	StartCmd          string                 `json:"startCmd,omitempty"`
	ReadyCmd          string                 `json:"readyCmd,omitempty"`
	Steps             []instructionPayload   `json:"steps"`
	Force             bool                   `json:"force"`
	FromImage         string                 `json:"fromImage,omitempty"`
	FromTemplate      string                 `json:"fromTemplate,omitempty"`
	FromImageRegistry *registryConfigPayload `json:"fromImageRegistry,omitempty"`
}

type instructionPayload struct {
	Type            InstructionType `json:"type"`
	Args            []string        `json:"args"`
	Force           bool            `json:"force"`
	ForceUpload     bool            `json:"forceUpload,omitempty"`
	FilesHash       string          `json:"filesHash,omitempty"`
	ResolveSymlinks bool            `json:"resolveSymlinks,omitempty"`
}

type registryConfigPayload struct {
	Type               string `json:"type"`
	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	AwsAccessKeyID     string `json:"awsAccessKeyId,omitempty"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey,omitempty"`
	AwsSessionToken    string `json:"awsSessionToken,omitempty"`
	AwsRegion          string `json:"awsRegion,omitempty"`
	ServiceAccountJSON string `json:"serviceAccountJson,omitempty"`
}

// buildStatusAPIResponse is the raw API response for build status.
type buildStatusAPIResponse struct {
	BuildID    string                        `json:"buildID"`
	TemplateID string                        `json:"templateID"`
	Status     TemplateBuildStatus           `json:"status"`
	LogEntries []buildLogEntryAPIResponse    `json:"logEntries"`
	Logs       []string                      `json:"logs"`
	Reason     *buildStatusReasonAPIResponse `json:"reason"`
}

type buildLogEntryAPIResponse struct {
	Timestamp time.Time     `json:"timestamp"`
	Level     LogEntryLevel `json:"level"`
	Message   string        `json:"message"`
}

type buildStatusReasonAPIResponse struct {
	Message    string                     `json:"message"`
	Step       string                     `json:"step,omitempty"`
	LogEntries []buildLogEntryAPIResponse `json:"logEntries"`
}

// tagsRequest is the request body for POST/DELETE /templates/tags.
type assignTagsRequest struct {
	Target string   `json:"target"`
	Tags   []string `json:"tags"`
}

type removeTagsRequest struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type assignTagsResponse struct {
	BuildID string   `json:"buildID"`
	Tags    []string `json:"tags"`
}

type fileUploadLinkResponse struct {
	Present bool   `json:"present"`
	URL     string `json:"url"`
}

// aliasResponse represents the response from the alias check endpoint.
type aliasResponse struct {
	TemplateID string `json:"templateID"`
}

const buildLogsRefreshInterval = 200 * time.Millisecond
const buildStatusLogsLimit = 100

// requestBuild creates a new template and build via POST /v3/templates.
func requestBuild(ctx context.Context, client *api.ApiClient, name string, tags []string, cpuCount, memoryMB int) (*BuildInfo, error) {
	reqBody := &requestBuildRequest{
		Name:     name,
		Tags:     tags,
		CpuCount: cpuCount,
		MemoryMB: memoryMB,
	}

	var resp requestBuildResponse
	_, err := client.Post(ctx, "/v3/templates", reqBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("request build failed: %w", err)
	}

	responseTags := resp.Tags
	if responseTags == nil {
		responseTags = append([]string{}, tags...)
	}

	return &BuildInfo{
		Alias:      name,
		Name:       name,
		Tags:       responseTags,
		TemplateID: resp.TemplateID,
		BuildID:    resp.BuildID,
	}, nil
}

func getFileUploadLink(ctx context.Context, client *api.ApiClient, templateID, filesHash, callerTrace string) (*fileUploadLinkResponse, error) {
	path := fmt.Sprintf("/templates/%s/files/%s", url.PathEscape(templateID), url.PathEscape(filesHash))
	var resp fileUploadLinkResponse
	_, err := client.Get(ctx, path, &resp)
	if err != nil {
		return nil, appendCallerTrace(fmt.Errorf("get file upload link failed: %w", err), callerTrace)
	}
	return &resp, nil
}

func uploadFile(ctx context.Context, uploadURL string, archive []byte, callerTrace string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(archive))
	if err != nil {
		return appendCallerTrace(fmt.Errorf("failed to create upload request: %w", err), callerTrace)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return appendCallerTrace(fmt.Errorf("failed to upload file: %w", err), callerTrace)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return appendCallerTrace(fmt.Errorf("failed to upload file: %s", strings.TrimSpace(string(body))), callerTrace)
	}

	return nil
}

// triggerBuild triggers an existing build with the serialized template via POST /v2/templates/{templateID}/builds/{buildID}.
func triggerBuild(ctx context.Context, client *api.ApiClient, templateID, buildID string, template triggerBuildTemplate) error {
	path := fmt.Sprintf("/v2/templates/%s/builds/%s", templateID, buildID)
	_, err := client.Post(ctx, path, &template, nil)
	if err != nil {
		return fmt.Errorf("trigger build failed: %w", err)
	}
	return nil
}

// getBuildStatusFromAPI retrieves the build status via GET /templates/{templateID}/builds/{buildID}/status.
func getBuildStatusFromAPI(ctx context.Context, client *api.ApiClient, templateID, buildID string, logsOffset int) (*TemplateBuildStatusResponse, error) {
	path := fmt.Sprintf("/templates/%s/builds/%s/status?logsOffset=%d&limit=%d", templateID, buildID, logsOffset, buildStatusLogsLimit)

	var resp buildStatusAPIResponse
	_, err := client.Get(ctx, path, &resp)
	if err != nil {
		return nil, fmt.Errorf("get build status failed: %w", err)
	}

	return &TemplateBuildStatusResponse{
		BuildID:    resp.BuildID,
		TemplateID: resp.TemplateID,
		Status:     resp.Status,
		LogEntries: mapBuildLogEntries(resp.LogEntries),
		Logs:       resp.Logs,
		Reason:     mapBuildStatusReason(resp.Reason),
	}, nil
}

// waitForBuildFinish polls build status every 200ms until it reaches "ready" or "error".
func waitForBuildFinish(ctx context.Context, client *api.ApiClient, templateID, buildID string, logger BuildLogger, callerTraces []string) (*TemplateBuildStatusResponse, error) {
	logsOffset := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		status, err := getBuildStatusFromAPI(ctx, client, templateID, buildID, logsOffset)
		if err != nil {
			return nil, err
		}

		logsOffset += len(status.LogEntries)

		// Emit logs if logger is provided
		if logger != nil {
			for _, entry := range status.LogEntries {
				logEntry := entry
				logger(&logEntry)
			}
		}

		switch status.Status {
		case BuildStatusReady:
			return status, nil
		case BuildStatusError:
			reason := "unknown error"
			callerTrace := ""
			if status.Reason != nil && status.Reason.Message != "" {
				reason = status.Reason.Message
				index := buildStepIndex(status.Reason.Step, len(callerTraces))
				if index >= 0 && index < len(callerTraces) {
					callerTrace = callerTraces[index]
				}
			}
			return status, &shared.BuildError{Message: fmt.Sprintf("build failed: %s", reason), CallerTrace: callerTrace}
		}

		timer := time.NewTimer(buildLogsRefreshInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// checkAliasExists checks if a template alias exists via GET /templates/aliases/{alias}.
// Returns true if found (200), false if not found (404).
func checkAliasExists(ctx context.Context, client *api.ApiClient, name string) (bool, error) {
	path := fmt.Sprintf("/templates/aliases/%s", name)
	var resp aliasResponse
	_, err := client.Get(ctx, path, &resp)
	if err != nil {
		if _, ok := err.(*api.NotFoundError); ok {
			return false, nil
		}
		if apiErr, ok := err.(*api.ApiError); ok && apiErr.StatusCode == http.StatusForbidden {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

// AssignTags assigns tags to a template via POST /templates/tags.
func assignTags(ctx context.Context, client *api.ApiClient, targetName string, tags []string) (*TemplateTagInfo, error) {
	reqBody := &assignTagsRequest{
		Target: targetName,
		Tags:   tags,
	}
	var resp assignTagsResponse
	_, err := client.Post(ctx, "/templates/tags", reqBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("assign tags failed: %w", err)
	}
	return &TemplateTagInfo{
		BuildID: resp.BuildID,
		Tags:    resp.Tags,
	}, nil
}

// RemoveTags removes tags from a template via DELETE /templates/tags with a JSON body.
func removeTags(ctx context.Context, client *api.ApiClient, templateName string, tags []string) error {
	reqBody := &removeTagsRequest{
		Name: templateName,
		Tags: tags,
	}
	_, err := client.Do(ctx, http.MethodDelete, "/templates/tags", reqBody, nil)
	if err != nil {
		return fmt.Errorf("remove tags failed: %w", err)
	}
	return nil
}

// getTemplateTags retrieves tags for a template via GET /templates/{templateID}/tags.
func getTemplateTags(ctx context.Context, client *api.ApiClient, templateID string) ([]TemplateTag, error) {
	path := fmt.Sprintf("/templates/%s/tags", templateID)
	var tags []TemplateTag
	_, err := client.Get(ctx, path, &tags)
	if err != nil {
		return nil, fmt.Errorf("get template tags failed: %w", err)
	}
	return tags, nil
}

func mapBuildLogEntries(entries []buildLogEntryAPIResponse) []LogEntry {
	if len(entries) == 0 {
		return nil
	}
	logs := make([]LogEntry, len(entries))
	for i, entry := range entries {
		logs[i] = LogEntry{
			Timestamp: entry.Timestamp,
			Level:     entry.Level,
			Message:   stripAnsi(entry.Message),
		}
	}
	return logs
}

func mapBuildStatusReason(reason *buildStatusReasonAPIResponse) *BuildStatusReason {
	if reason == nil {
		return nil
	}
	return &BuildStatusReason{
		Message:    reason.Message,
		Step:       reason.Step,
		LogEntries: mapBuildLogEntries(reason.LogEntries),
	}
}
