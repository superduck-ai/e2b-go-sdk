package template

import (
	"context"
	"fmt"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

// TemplateInfo represents a template returned by the List Templates API.
type TemplateInfo struct {
	TemplateID    string     `json:"templateID"`
	Aliases       []string   `json:"aliases"`
	Names         []string   `json:"names"`
	ImageRef      string     `json:"imageRef,omitempty"`
	BuildCount    int        `json:"buildCount"`
	BuildID       string     `json:"buildID"`
	BuildStatus   string     `json:"buildStatus"`
	CPUCount      int        `json:"cpuCount"`
	MemoryMB      int        `json:"memoryMB"`
	DiskSizeMB    int        `json:"diskSizeMB"`
	EnvdVersion   string     `json:"envdVersion"`
	Public        bool       `json:"public"`
	SpawnCount    int64      `json:"spawnCount"`
	LastSpawnedAt *time.Time `json:"lastSpawnedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ListTemplatesOpts are options for listing templates.
type ListTemplatesOpts struct {
	TeamID string
	ConnectionOpts
}

// ListTemplates returns all templates accessible by the authenticated user.
// If TeamID is set, only templates belonging to that team are returned.
func ListTemplates(ctx context.Context, opts *ListTemplatesOpts) ([]TemplateInfo, error) {
	if opts == nil {
		opts = &ListTemplatesOpts{}
	}
	ctx, cancel := shared.MergeContexts(ctx, opts.Signal)
	defer cancel()

	client, err := newApiClientFromConnectionOptions(&opts.ConnectionOpts)
	if err != nil {
		return nil, err
	}

	path := "/templates"
	if opts.TeamID != "" {
		path = fmt.Sprintf("/templates?teamID=%s", opts.TeamID)
	}

	var templates []TemplateInfo
	_, err = client.Get(ctx, path, &templates)
	if err != nil {
		return nil, err
	}

	return templates, nil
}
