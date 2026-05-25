package template

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/api"
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
	TemplateID string `json:"templateID"`
	BuildID    string `json:"buildID"`
}

// triggerBuildRequest is the request body for POST /v2/templates/{templateID}/builds/{buildID}.
type triggerBuildRequest struct {
	Instructions []instructionPayload `json:"instructions"`
}

type instructionPayload struct {
	Type            InstructionType `json:"type"`
	Args            string          `json:"args"`
	Force           bool            `json:"force,omitempty"`
	ForceUpload     bool            `json:"forceUpload,omitempty"`
	FilesHash       string          `json:"filesHash,omitempty"`
	ResolveSymlinks bool            `json:"resolveSymlinks,omitempty"`
}

// buildStatusAPIResponse is the raw API response for build status.
type buildStatusAPIResponse struct {
	BuildID    string              `json:"buildID"`
	TemplateID string              `json:"templateID"`
	Status     TemplateBuildStatus `json:"status"`
	Logs       string              `json:"logs"`
	Reason     string              `json:"reason"`
}

// tagsRequest is the request body for POST/DELETE /templates/tags.
type tagsRequest struct {
	TemplateName string   `json:"templateName"`
	Tags         []string `json:"tags"`
}

// aliasResponse represents the response from the alias check endpoint.
type aliasResponse struct {
	TemplateID string `json:"templateID"`
}

// RequestBuild creates a new template and build via POST /v3/templates.
func RequestBuild(ctx context.Context, client *api.ApiClient, name string, tags []string, cpuCount, memoryMB int) (*BuildInfo, error) {
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

	return &BuildInfo{
		Name:       name,
		Tags:       tags,
		TemplateID: resp.TemplateID,
		BuildID:    resp.BuildID,
	}, nil
}

// TriggerBuild triggers an existing build with instructions via POST /v2/templates/{templateID}/builds/{buildID}.
func TriggerBuild(ctx context.Context, client *api.ApiClient, templateID, buildID string, instructions []Instruction) error {
	payloads := make([]instructionPayload, len(instructions))
	for i, inst := range instructions {
		payloads[i] = instructionPayload{
			Type:            inst.Type,
			Args:            inst.Args,
			Force:           inst.Force,
			ForceUpload:     inst.ForceUpload,
			FilesHash:       inst.FilesHash,
			ResolveSymlinks: inst.ResolveSymlinks,
		}
	}

	reqBody := &triggerBuildRequest{Instructions: payloads}
	path := fmt.Sprintf("/v2/templates/%s/builds/%s", templateID, buildID)
	_, err := client.Post(ctx, path, reqBody, nil)
	if err != nil {
		return fmt.Errorf("trigger build failed: %w", err)
	}
	return nil
}

// GetBuildStatusFromAPI retrieves the build status via GET /templates/{templateID}/builds/{buildID}/status.
func GetBuildStatusFromAPI(ctx context.Context, client *api.ApiClient, templateID, buildID string, logsOffset int) (*TemplateBuildStatusResponse, error) {
	path := fmt.Sprintf("/templates/%s/builds/%s/status?logsOffset=%d", templateID, buildID, logsOffset)

	var resp buildStatusAPIResponse
	_, err := client.Get(ctx, path, &resp)
	if err != nil {
		return nil, fmt.Errorf("get build status failed: %w", err)
	}

	return &TemplateBuildStatusResponse{
		BuildID:    resp.BuildID,
		TemplateID: resp.TemplateID,
		Status:     resp.Status,
		Logs:       resp.Logs,
		Reason:     resp.Reason,
	}, nil
}

// WaitForBuildFinish polls build status every 2 seconds until it reaches "ready" or "error".
func WaitForBuildFinish(ctx context.Context, client *api.ApiClient, templateID, buildID string, logger BuildLogger) (*TemplateBuildStatusResponse, error) {
	logsOffset := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		status, err := GetBuildStatusFromAPI(ctx, client, templateID, buildID, logsOffset)
		if err != nil {
			return nil, err
		}

		// Emit logs if logger is provided
		if logger != nil && status.Logs != "" {
			logger(&LogEntry{
				Timestamp: time.Now(),
				Level:     LogLevelInfo,
				Message:   status.Logs,
			})
			logsOffset += len(status.Logs)
		}

		switch status.Status {
		case BuildStatusReady:
			return status, nil
		case BuildStatusError:
			return status, fmt.Errorf("build failed: %s", status.Reason)
		}

		time.Sleep(2 * time.Second)
	}
}

// CheckAliasExists checks if a template alias exists via GET /templates/aliases/{alias}.
// Returns true if found (200), false if not found (404).
func CheckAliasExists(ctx context.Context, client *api.ApiClient, name string) (bool, error) {
	path := fmt.Sprintf("/templates/aliases/%s", name)
	var resp aliasResponse
	_, err := client.Get(ctx, path, &resp)
	if err != nil {
		if _, ok := err.(*api.NotFoundError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// AssignTags assigns tags to a template via POST /templates/tags.
func AssignTags(ctx context.Context, client *api.ApiClient, templateName string, tags []string) error {
	reqBody := &tagsRequest{
		TemplateName: templateName,
		Tags:         tags,
	}
	_, err := client.Post(ctx, "/templates/tags", reqBody, nil)
	if err != nil {
		return fmt.Errorf("assign tags failed: %w", err)
	}
	return nil
}

// RemoveTags removes tags from a template via DELETE /templates/tags with a JSON body.
func RemoveTags(ctx context.Context, client *api.ApiClient, templateName string, tags []string) error {
	reqBody := &tagsRequest{
		TemplateName: templateName,
		Tags:         tags,
	}
	_, err := client.Do(ctx, http.MethodDelete, "/templates/tags", reqBody, nil)
	if err != nil {
		return fmt.Errorf("remove tags failed: %w", err)
	}
	return nil
}

// GetTemplateTags retrieves tags for a template via GET /templates/{templateID}/tags.
func GetTemplateTags(ctx context.Context, client *api.ApiClient, templateID string) ([]TemplateTag, error) {
	path := fmt.Sprintf("/templates/%s/tags", templateID)
	var tags []TemplateTag
	_, err := client.Get(ctx, path, &tags)
	if err != nil {
		return nil, fmt.Errorf("get template tags failed: %w", err)
	}
	return tags, nil
}
