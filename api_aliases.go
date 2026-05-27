package e2b

import rootapi "github.com/superduck-ai/e2b-go-sdk/api"

type ApiClient = rootapi.ApiClient

func NewApiClient(config *ConnectionConfig, opts *struct {
	RequireAccessToken bool
	RequireApiKey      bool
}) (*ApiClient, error) {
	if config == nil {
		config = NewConnectionConfig(nil)
	}
	rootOpts := make([]rootapi.ApiClientOption, 0, 2)
	if opts != nil && opts.RequireApiKey {
		rootOpts = append(rootOpts, rootapi.WithRequireApiKey())
	}
	if opts != nil && opts.RequireAccessToken {
		rootOpts = append(rootOpts, rootapi.WithRequireAccessToken())
	}
	return rootapi.NewApiClient(toClientConfig(config), rootOpts...)
}
