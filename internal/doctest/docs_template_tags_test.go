package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateTagsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/tags.mdx"); err != nil {
		t.Fatalf("template tags doc is missing: %v", err)
	}
}

// This test keeps docs/template/tags.mdx aligned with the exported Go SDK
// tags surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsTemplateTagsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "default-tag",
			fn: func() {
				ctx := context.Background()

				implicitDefault, implicitErr := e2b.Create(ctx, "my-template", nil)
				explicitDefault, explicitErr := e2b.Create(ctx, "my-template:default", nil)

				_ = implicitDefault
				_ = explicitDefault
				_ = implicitErr
				_ = explicitErr
			},
		},
		{
			name: "tag-or-build-id-reference",
			fn: func() {
				ctx := context.Background()

				production, productionErr := e2b.Create(ctx, "my-template:production", nil)
				exactBuild, exactBuildErr := e2b.Create(ctx, "my-template:f47ac10b-58cc-4372-a567-0e02b2c3d479", nil)
				namespaced, namespacedErr := e2b.Create(ctx, "acme/my-template:f47ac10b-58cc-4372-a567-0e02b2c3d479", nil)

				_ = production
				_ = exactBuild
				_ = namespaced
				_ = productionErr
				_ = exactBuildErr
				_ = namespacedErr
			},
		},
		{
			name: "build-single-tag",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				info, err := e2b.Build(ctx, template, "my-template:v1.0.0", nil)
				if info != nil {
					_ = info.TemplateID
					_ = info.BuildID
					_ = info.Tags
				}
				_ = err
			},
		},
		{
			name: "build-multiple-tags",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				info, err := e2b.Build(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						Tags: []string{"v1.2.0", "latest"},
					},
				})
				if info != nil {
					_ = info.TemplateID
					_ = info.BuildID
					_ = info.Tags
				}
				_ = err
			},
		},
		{
			name: "exists",
			fn: func() {
				ctx := context.Background()

				exists, err := e2b.Exists(ctx, "my-template", nil)
				_ = exists
				_ = err
			},
		},
		{
			name: "assign-tags",
			fn: func() {
				ctx := context.Background()

				single, singleErr := e2b.AssignTags(ctx, "my-template:v1.2.0", "production", nil)
				multiple, multipleErr := e2b.AssignTags(ctx, "my-template:v1.2.0", []string{"stable", "latest"}, nil)

				if single != nil {
					_ = single.BuildID
					_ = single.Tags
				}
				if multiple != nil {
					_ = multiple.BuildID
					_ = multiple.Tags
				}
				_ = singleErr
				_ = multipleErr
			},
		},
		{
			name: "remove-tags",
			fn: func() {
				ctx := context.Background()

				removeOneErr := e2b.RemoveTags(ctx, "my-template", "staging", nil)
				removeManyErr := e2b.RemoveTags(ctx, "my-template", []string{"canary", "old"}, nil)

				_ = removeOneErr
				_ = removeManyErr
			},
		},
		{
			name: "list-tags",
			fn: func() {
				ctx := context.Background()
				info := &e2b.BuildInfo{TemplateID: "tmpl_123"}

				tags, err := e2b.GetTags(ctx, info.TemplateID, nil)
				for _, tag := range tags {
					_ = tag.Tag
					_ = tag.BuildID
					_ = tag.CreatedAt
				}
				_ = err
			},
		},
	}

	if got := len(snippets); got != 8 {
		t.Fatalf("expected 8 template tags doc snippets, got %d", got)
	}
}
