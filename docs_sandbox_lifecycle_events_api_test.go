package e2b_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestDocsSandboxLifecycleEventsAPIDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/lifecycle-events-api.mdx"); err != nil {
		t.Fatalf("sandbox lifecycle events api doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/lifecycle-events-api.mdx aligned with the Go
// lifecycle events polling examples. The closures are compile-only examples and
// are intentionally never executed.
func TestDocsSandboxLifecycleEventsAPIExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "fetch-events",
			fn: func() {
				fetchEvents := func(ctx context.Context, client *http.Client, apiKey, endpoint string) ([]map[string]any, error) {
					req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
					if err != nil {
						return nil, err
					}
					req.Header.Set("X-API-Key", apiKey)

					resp, err := client.Do(req)
					if err != nil {
						return nil, err
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return nil, fmt.Errorf("unexpected status: %s", resp.Status)
					}

					var events []map[string]any
					if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
						return nil, err
					}

					return events, nil
				}

				ctx := context.Background()
				client := &http.Client{}
				apiKey := os.Getenv("E2B_API_KEY")
				sandboxID := "sbx_123"

				sandboxEvents, sandboxErr := fetchEvents(
					ctx,
					client,
					apiKey,
					"https://api.e2b.app/events/sandboxes/"+sandboxID,
				)

				teamEvents, teamErr := fetchEvents(
					ctx,
					client,
					apiKey,
					"https://api.e2b.app/events/sandboxes?limit=10",
				)

				query := url.Values{}
				query.Add("types", "sandbox.lifecycle.created")
				query.Add("types", "sandbox.lifecycle.killed")

				filteredEvents, filteredErr := fetchEvents(
					ctx,
					client,
					apiKey,
					"https://api.e2b.app/events/sandboxes?"+query.Encode(),
				)

				_ = sandboxEvents
				_ = teamEvents
				_ = filteredEvents
				_ = sandboxErr
				_ = teamErr
				_ = filteredErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 sandbox lifecycle events api doc snippet, got %d", got)
	}
}
