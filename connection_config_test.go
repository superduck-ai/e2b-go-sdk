package e2b

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNewConnectionConfigAlwaysSetsSdkUserAgent(t *testing.T) {
	config := NewConnectionConfig(&ConnectionOpts{
		Headers: map[string]string{
			"User-Agent": "custom-agent/1.0",
			"X-Test":     "value",
		},
	})

	if config.Headers["User-Agent"] != "e2b-go-sdk/"+sdkVersion {
		t.Fatalf("expected SDK user agent override, got %q", config.Headers["User-Agent"])
	}
	if config.Headers["X-Test"] != "value" {
		t.Fatalf("expected other headers to be preserved, got %#v", config.Headers)
	}
}

func TestConnectionConfigGetHostMatchesJsFormatting(t *testing.T) {
	debugConfig := &ConnectionConfig{Debug: true}
	if got := debugConfig.GetHost("sbx-1", 0, "sandbox.e2b.app"); got != "localhost:0" {
		t.Fatalf("expected debug host localhost:0, got %q", got)
	}

	regularConfig := &ConnectionConfig{}
	if got := regularConfig.GetHost("sbx-1", 0, "sandbox.e2b.app"); got != "0-sbx-1.sandbox.e2b.app" {
		t.Fatalf("expected non-debug host 0-sbx-1.sandbox.e2b.app, got %q", got)
	}
}

func TestConnectionConfigGetHostFallsBackToConfigDomainWhenSandboxDomainMissing(t *testing.T) {
	config := &ConnectionConfig{Domain: "e2b.app"}

	if got := config.GetHost("sbx-1", 49983, ""); got != "49983-sbx-1.e2b.app" {
		t.Fatalf("expected host to fall back to config domain, got %q", got)
	}
}

func TestConnectionConfigGetSandboxUrlUsesStableHostForSupportedDomains(t *testing.T) {
	supportedDomains := []string{"e2b.app", "e2b.dev", "e2b.pro", "e2b-staging.dev"}
	config := &ConnectionConfig{}

	for _, sandboxDomain := range supportedDomains {
		t.Run(sandboxDomain, func(t *testing.T) {
			want := "https://sandbox." + sandboxDomain
			if got := config.GetSandboxUrl("sbx-1", sandboxDomain, 49983); got != want {
				t.Fatalf("expected stable sandbox URL %q, got %q", want, got)
			}
		})
	}
}

func TestConnectionConfigGetSandboxDirectUrlUsesPerSandboxHostForSupportedDomains(t *testing.T) {
	supportedDomains := []string{"e2b.app", "e2b.dev", "e2b.pro", "e2b-staging.dev"}
	config := &ConnectionConfig{}

	for _, sandboxDomain := range supportedDomains {
		t.Run(sandboxDomain, func(t *testing.T) {
			want := "https://49983-sbx-1." + sandboxDomain
			if got := config.GetSandboxDirectUrl("sbx-1", sandboxDomain, 49983); got != want {
				t.Fatalf("expected direct sandbox URL %q, got %q", want, got)
			}
		})
	}
}

func TestConnectionConfigGetSandboxUrlFallsBackToConfigDomainWhenSandboxDomainMissing(t *testing.T) {
	config := &ConnectionConfig{Domain: "e2b.app"}

	if got := config.GetSandboxUrl("sbx-1", "", 49983); got != "https://sandbox.e2b.app" {
		t.Fatalf("expected sandbox URL to fall back to stable config domain host, got %q", got)
	}
}

func TestConnectionConfigGetSandboxDirectUrlFallsBackToConfigDomainWhenSandboxDomainMissing(t *testing.T) {
	config := &ConnectionConfig{Domain: "e2b.app"}

	if got := config.GetSandboxDirectUrl("sbx-1", "", 49983); got != "https://49983-sbx-1.e2b.app" {
		t.Fatalf("expected direct sandbox URL to fall back to config domain, got %q", got)
	}
}

func TestConnectionConfigGetSandboxUrlKeepsPerSandboxHostOutsideStableDomains(t *testing.T) {
	config := &ConnectionConfig{Domain: "e2b.dev"}

	if got := config.GetSandboxUrl("sbx-test", "sandbox.e2b.dev", 49983); got != "https://49983-sbx-test.sandbox.e2b.dev" {
		t.Fatalf("expected sandbox URL to keep per-sandbox host, got %q", got)
	}
}

func TestConnectionConfigGetSandboxUrlsUseExplicitSandboxOverride(t *testing.T) {
	config := &ConnectionConfig{SandboxUrl: "https://sandbox.custom"}

	if got := config.GetSandboxUrl("sbx-1", "e2b.app", 49983); got != "https://sandbox.custom" {
		t.Fatalf("expected sandbox URL override to win, got %q", got)
	}
	if got := config.GetSandboxDirectUrl("sbx-1", "e2b.app", 49983); got != "https://sandbox.custom" {
		t.Fatalf("expected direct sandbox URL override to win, got %q", got)
	}
}

func TestConnectionConfigGetSandboxUrlsStayLocalhostInDebugMode(t *testing.T) {
	config := &ConnectionConfig{Debug: true}

	if got := config.GetSandboxUrl("sbx-1", "e2b.app", 49983); got != "http://localhost:49983" {
		t.Fatalf("expected debug sandbox URL localhost, got %q", got)
	}
	if got := config.GetSandboxDirectUrl("sbx-1", "e2b.app", 49983); got != "http://localhost:49983" {
		t.Fatalf("expected debug direct sandbox URL localhost, got %q", got)
	}
}

func TestNewConnectionConfigAllowsExplicitZeroRequestTimeout(t *testing.T) {
	config := NewConnectionConfig(&ConnectionOpts{
		RequestTimeoutMs: intPtr(0),
	})

	if config.RequestTimeoutMs != 0 {
		t.Fatalf("expected explicit zero request timeout to be preserved, got %d", config.RequestTimeoutMs)
	}
}

func TestNewConnectionConfigDefaultsApiUrlLikeJsConstructor(t *testing.T) {
	t.Setenv("E2B_API_URL", "")
	t.Setenv("E2B_DOMAIN", "")
	t.Setenv("E2B_DEBUG", "")

	defaultConfig := NewConnectionConfig(nil)
	if defaultConfig.ApiUrl != "https://api.e2b.app" {
		t.Fatalf("expected default API URL to match JS constructor, got %q", defaultConfig.ApiUrl)
	}

	config := NewConnectionConfig(&ConnectionOpts{
		Domain: "example.test",
	})

	if config.ApiUrl != "https://api.example.test" {
		t.Fatalf("expected default API URL to match JS constructor, got %q", config.ApiUrl)
	}

	debugConfig := NewConnectionConfig(&ConnectionOpts{
		Domain: "example.test",
		Debug:  boolRef(true),
	})

	if debugConfig.ApiUrl != "http://localhost:3000" {
		t.Fatalf("expected debug API URL to match JS constructor, got %q", debugConfig.ApiUrl)
	}
}

func TestNewConnectionConfigMatchesCurrentJsAndPythonRootDebugTruthiness(t *testing.T) {
	t.Setenv("E2B_API_URL", "")
	t.Setenv("E2B_DOMAIN", "")
	t.Setenv("E2B_DEBUG", "true")

	config := NewConnectionConfig(&ConnectionOpts{
		Debug: boolRef(false),
	})

	if !config.Debug {
		t.Fatalf("expected env debug=true to win over explicit false like current JS/Python root constructors, got %#v", config)
	}
	if config.ApiUrl != "http://localhost:3000" {
		t.Fatalf("expected env debug=true to keep localhost API URL like current JS/Python root constructors, got %q", config.ApiUrl)
	}
}

func TestNewConnectionConfigUsesApiUrlFromArgs(t *testing.T) {
	config := NewConnectionConfig(&ConnectionOpts{
		ApiUrl: "http://localhost:8080",
	})

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected API URL from args, got %q", config.ApiUrl)
	}
}

func TestNewConnectionConfigUsesApiUrlFromEnv(t *testing.T) {
	t.Setenv("E2B_API_URL", "http://localhost:8080")

	config := NewConnectionConfig(nil)

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected API URL from env, got %q", config.ApiUrl)
	}
}

func TestNewConnectionConfigApiUrlArgsHavePriorityOverEnv(t *testing.T) {
	t.Setenv("E2B_API_URL", "http://localhost:1111")

	config := NewConnectionConfig(&ConnectionOpts{
		ApiUrl: "http://localhost:8080",
	})

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected API URL arg to win over env, got %q", config.ApiUrl)
	}
}

func TestConnectionOptsDebugMatchesJsAndPythonOptionalShape(t *testing.T) {
	field, ok := reflect.TypeOf(ConnectionOpts{}).FieldByName("Debug")
	if !ok {
		t.Fatal("expected ConnectionOpts to expose Debug")
	}
	if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected ConnectionOpts.Debug to be *bool, got %v", field.Type)
	}
}

func TestConnectionConfigRootConstantsAreNotExported(t *testing.T) {
	source, err := os.ReadFile("connection_config.go")
	if err != nil {
		t.Fatalf("failed to read connection_config.go: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "connection_config.go", source, 0)
	if err != nil {
		t.Fatalf("failed to parse connection_config.go: %v", err)
	}

	forbidden := map[string]struct{}{
		"RequestTimeoutMs":         {},
		"DefaultSandboxTimeoutMs":  {},
		"KeepalivePingIntervalSec": {},
		"KeepalivePingHeader":      {},
		"EnvdPort":                 {},
		"DefaultUsername":          {},
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range valueSpec.Names {
				if _, forbidden := forbidden[name.Name]; forbidden {
					t.Fatalf("did not expect %s to be exported", name.Name)
				}
			}
		}
	}
}

func TestUsernameTypeIsExportedLikeJsRootSurface(t *testing.T) {
	source, err := os.ReadFile("connection_config.go")
	if err != nil {
		t.Fatalf("failed to read connection_config.go: %v", err)
	}

	if !strings.Contains(string(source), "type Username = string") {
		t.Fatal("expected Username type alias to be exported")
	}
}
