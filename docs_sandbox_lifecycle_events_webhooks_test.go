package e2b_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestDocsSandboxLifecycleEventsWebhooksDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/lifecycle-events-webhooks.mdx"); err != nil {
		t.Fatalf("sandbox lifecycle events webhooks doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/lifecycle-events-webhooks.mdx aligned with the
// Go webhook-management examples. The closures are compile-only examples and
// are intentionally never executed.
func TestDocsSandboxLifecycleEventsWebhooksExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "manage-webhooks",
			fn: func() {
				type webhookRequest struct {
					Name            string   `json:"name,omitempty"`
					URL             string   `json:"url,omitempty"`
					Enabled         *bool    `json:"enabled,omitempty"`
					Events          []string `json:"events,omitempty"`
					SignatureSecret string   `json:"signatureSecret,omitempty"`
				}

				doJSON := func(ctx context.Context, client *http.Client, apiKey, method, endpoint string, body any, out any) error {
					var reader io.Reader
					if body != nil {
						payload, err := json.Marshal(body)
						if err != nil {
							return err
						}
						reader = bytes.NewReader(payload)
					}

					req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
					if err != nil {
						return err
					}
					req.Header.Set("X-API-Key", apiKey)
					if body != nil {
						req.Header.Set("Content-Type", "application/json")
					}

					resp, err := client.Do(req)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode >= 300 {
						return fmt.Errorf("unexpected status: %s", resp.Status)
					}

					if out != nil {
						return json.NewDecoder(resp.Body).Decode(out)
					}

					return nil
				}

				ctx := context.Background()
				client := &http.Client{}
				apiKey := os.Getenv("E2B_API_KEY")
				baseURL := "https://api.e2b.app/events/webhooks"
				webhookID := "wh_123"

				enabled := true
				registerBody := webhookRequest{
					Name:    "My Sandbox Webhook",
					URL:     "https://your-webhook-endpoint.com/webhook",
					Enabled: &enabled,
					Events: []string{
						"sandbox.lifecycle.created",
						"sandbox.lifecycle.updated",
						"sandbox.lifecycle.killed",
					},
					SignatureSecret: "secret-for-event-signature-verification",
				}

				var created map[string]any
				createErr := doJSON(ctx, client, apiKey, http.MethodPost, baseURL, registerBody, &created)

				var webhooks []map[string]any
				listErr := doJSON(ctx, client, apiKey, http.MethodGet, baseURL, nil, &webhooks)

				var current map[string]any
				getErr := doJSON(ctx, client, apiKey, http.MethodGet, baseURL+"/"+webhookID, nil, &current)

				disabled := false
				updateBody := webhookRequest{
					URL:     "https://your-updated-webhook-endpoint.com/webhook",
					Enabled: &disabled,
					Events:  []string{"sandbox.lifecycle.created"},
				}
				updateErr := doJSON(ctx, client, apiKey, http.MethodPatch, baseURL+"/"+webhookID, updateBody, nil)

				deleteErr := doJSON(ctx, client, apiKey, http.MethodDelete, baseURL+"/"+webhookID, nil, nil)

				_ = created
				_ = webhooks
				_ = current
				_ = createErr
				_ = listErr
				_ = getErr
				_ = updateErr
				_ = deleteErr
			},
		},
		{
			name: "verify-signature",
			fn: func() {
				verifyWebhookSignature := func(secret, payload, payloadSignature string) bool {
					sum := sha256.Sum256([]byte(secret + payload))
					expected := base64.StdEncoding.EncodeToString(sum[:])
					expected = strings.TrimRight(expected, "=")

					return expected == payloadSignature
				}

				_ = verifyWebhookSignature("secret", `{"id":"evt_123"}`, "signature")
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 sandbox lifecycle events webhooks doc snippets, got %d", got)
	}
}
