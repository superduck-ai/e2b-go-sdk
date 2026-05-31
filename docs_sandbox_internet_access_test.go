package e2b_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxInternetAccessDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/internet-access.mdx"); err != nil {
		t.Fatalf("sandbox internet access doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/internet-access.mdx aligned with the exported
// Go SDK network and host access surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsSandboxInternetAccessExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "toggle-internet-access",
			fn: func() {
				ctx := context.Background()

				allowInternet := true
				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					AllowInternetAccess: &allowInternet,
				})
				if sandbox != nil {
					defer sandbox.Kill(context.Background(), nil)
				}

				allowInternet = false
				isolatedSandbox, isolatedErr := e2b.Create(ctx, "", &e2b.SandboxOpts{
					AllowInternetAccess: &allowInternet,
				})
				if isolatedSandbox != nil {
					defer isolatedSandbox.Kill(context.Background(), nil)
				}

				_ = sandbox
				_ = isolatedSandbox
				_ = err
				_ = isolatedErr
			},
		},
		{
			name: "allow-and-deny-lists",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Network: &e2b.SandboxNetworkOpts{
						DenyOut:  []string{e2b.ALL_TRAFFIC},
						AllowOut: []string{"1.1.1.1", "8.8.8.0/24"},
					},
				})
				restrictedSandbox, restrictedErr := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Network: &e2b.SandboxNetworkOpts{
						DenyOut: []string{"8.8.8.8"},
					},
				})

				_ = sandbox
				_ = restrictedSandbox
				_ = err
				_ = restrictedErr
			},
		},
		{
			name: "selector-callbacks-and-rules",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Network: &e2b.SandboxNetworkOpts{
						DenyOut: func(ctx e2b.SandboxNetworkSelectorContext) []string {
							return []string{ctx.AllTraffic}
						},
						AllowOut: func(ctx e2b.SandboxNetworkSelectorContext) []string {
							hosts := make([]string, 0, len(ctx.Rules))
							for host := range ctx.Rules {
								hosts = append(hosts, host)
							}
							return hosts
						},
						Rules: e2b.SandboxNetworkRules{
							"api.example.com": []e2b.SandboxNetworkRule{
								{
									Transform: &e2b.SandboxNetworkTransform{
										Headers: map[string]string{
											"X-Trace": "on",
										},
									},
								},
							},
						},
					},
				})

				_ = sandbox
				_ = err
			},
		},
		{
			name: "update-network",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				updateErr := sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{
					DenyOut: []string{"8.8.8.8"},
				}, nil)
				replaceErr := sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{
					DenyOut:  []string{e2b.ALL_TRAFFIC},
					AllowOut: []string{"api.example.com"},
				}, nil)

				allowInternet := false
				toggleErr := sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{
					AllowInternetAccess: &allowInternet,
				}, nil)
				clearErr := sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{}, nil)

				_ = updateErr
				_ = replaceErr
				_ = toggleErr
				_ = clearErr
			},
		},
		{
			name: "public-url-and-token",
			fn: func() {
				ctx := context.Background()
				allowPublicTraffic := false

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Network: &e2b.SandboxNetworkOpts{
						AllowPublicTraffic: &allowPublicTraffic,
						MaskRequestHost:    "custom-host.example.com:${PORT}",
					},
				})
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "python3 -m http.server 3000", &e2b.CommandStartOpts{
					Background: true,
				})
				handle := execution.(*e2b.CommandHandle)
				req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+sandbox.GetHost(3000), nil)
				if req != nil && sandbox.TrafficAccessToken != "" {
					req.Header.Set("e2b-traffic-access-token", sandbox.TrafficAccessToken)
				}

				_ = handle
				_ = req
				_ = runErr
				_ = reqErr
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 sandbox internet access doc snippets, got %d", got)
	}
}
