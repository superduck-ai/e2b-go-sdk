package applecontainer

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	RuntimeName = "applecontainer"

	defaultContainerNamePrefix  = "e2b-sandbox-"
	defaultEnvdPort             = 49983
	defaultHealthTimeoutSeconds = 30
	defaultCPUs                 = 1
	defaultMemoryMB             = 1024

	envEnvdBinary = "E2B_APPLE_CONTAINER_ENVD_BINARY"
	envBaseImage  = "E2B_APPLE_CONTAINER_BASE_IMAGE"
)

type RuntimeOptions struct {
	ContainerNamePrefix  string
	EnvdBinary           string
	EnvdPort             int
	HealthTimeoutSeconds int
	DefaultCPUs          int
	DefaultMemoryMB      int
	Templates            map[string]TemplateOptions
	Logger               *log.Logger
}

type TemplateOptions struct {
	Image            string
	CPUs             int
	MemoryMB         int
	StartCmd         string
	PrebakedEnvdPath string
}

func DefaultRuntimeOptions() RuntimeOptions {
	baseImage := strings.TrimSpace(os.Getenv(envBaseImage))
	if baseImage == "" {
		baseImage = "docker.io/library/debian:bookworm-slim"
	}

	return RuntimeOptions{
		ContainerNamePrefix:  defaultContainerNamePrefix,
		EnvdBinary:           strings.TrimSpace(os.Getenv(envEnvdBinary)),
		EnvdPort:             defaultEnvdPort,
		HealthTimeoutSeconds: defaultHealthTimeoutSeconds,
		DefaultCPUs:          defaultCPUs,
		DefaultMemoryMB:      defaultMemoryMB,
		Templates: map[string]TemplateOptions{
			"base": {Image: baseImage},
		},
	}
}

func (o RuntimeOptions) withDefaults() RuntimeOptions {
	defaults := DefaultRuntimeOptions()
	if strings.TrimSpace(o.ContainerNamePrefix) == "" {
		o.ContainerNamePrefix = defaults.ContainerNamePrefix
	}
	if strings.TrimSpace(o.EnvdBinary) == "" {
		o.EnvdBinary = defaults.EnvdBinary
	}
	if o.EnvdPort == 0 {
		o.EnvdPort = defaults.EnvdPort
	}
	if o.HealthTimeoutSeconds == 0 {
		o.HealthTimeoutSeconds = defaults.HealthTimeoutSeconds
	}
	if o.DefaultCPUs == 0 {
		o.DefaultCPUs = defaults.DefaultCPUs
	}
	if o.DefaultMemoryMB == 0 {
		o.DefaultMemoryMB = defaults.DefaultMemoryMB
	}
	if len(o.Templates) == 0 {
		o.Templates = defaults.Templates
	}
	return o
}

func (o RuntimeOptions) requiresHostEnvdBinary() bool {
	for _, template := range o.Templates {
		if strings.TrimSpace(template.PrebakedEnvdPath) == "" {
			return true
		}
	}
	return len(o.Templates) == 0
}

func (o RuntimeOptions) Validate() error {
	if strings.TrimSpace(o.ContainerNamePrefix) == "" {
		return fmt.Errorf("applecontainer container name prefix is required")
	}
	if strings.TrimSpace(o.EnvdBinary) != "" && !filepath.IsAbs(o.EnvdBinary) {
		return fmt.Errorf("applecontainer envd binary must be an absolute path: %s", o.EnvdBinary)
	}
	if o.EnvdPort <= 0 || o.EnvdPort > 65535 {
		return fmt.Errorf("applecontainer envd port must be between 1 and 65535")
	}
	if o.HealthTimeoutSeconds <= 0 {
		return fmt.Errorf("applecontainer health timeout must be positive")
	}
	if o.DefaultCPUs <= 0 {
		return fmt.Errorf("applecontainer default CPUs must be positive")
	}
	if o.DefaultMemoryMB <= 0 {
		return fmt.Errorf("applecontainer default memory must be positive")
	}
	if len(o.Templates) == 0 {
		return fmt.Errorf("applecontainer templates are required")
	}
	requiresHostEnvdBinary := false
	for templateID, template := range o.Templates {
		if strings.TrimSpace(templateID) == "" {
			return fmt.Errorf("applecontainer template id must not be empty")
		}
		if strings.TrimSpace(template.Image) == "" {
			return fmt.Errorf("applecontainer template %q image is required", templateID)
		}
		if template.CPUs < 0 {
			return fmt.Errorf("applecontainer template %q CPUs must not be negative", templateID)
		}
		if template.MemoryMB < 0 {
			return fmt.Errorf("applecontainer template %q memory must not be negative", templateID)
		}
		if prebakedEnvdPath := strings.TrimSpace(template.PrebakedEnvdPath); prebakedEnvdPath != "" {
			if !path.IsAbs(prebakedEnvdPath) {
				return fmt.Errorf("applecontainer template %q prebaked envd path must be absolute: %s", templateID, prebakedEnvdPath)
			}
		} else {
			requiresHostEnvdBinary = true
		}
	}
	if requiresHostEnvdBinary && strings.TrimSpace(o.EnvdBinary) == "" {
		return fmt.Errorf("applecontainer envd binary is required unless every template sets PrebakedEnvdPath; set RuntimeOptions.EnvdBinary or %s", envEnvdBinary)
	}
	return nil
}
