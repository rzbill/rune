package cmd

import (
	"github.com/rzbill/rune/pkg/api/client"
	"github.com/spf13/viper"
)

// buildClientOptions builds ClientOptions using current context from viper and an optional address override.
func buildClientOptions(addressOverride string) *client.ClientOptions {
	opts := client.DefaultClientOptions()
	// Address precedence: explicit override > context server (normalized) > default
	if addressOverride != "" {
		opts.Address = ensureDefaultGRPCPort(addressOverride)
	} else {
		if s := viper.GetString("contexts.default.server"); s != "" {
			opts.Address = ensureDefaultGRPCPort(s)
		}
	}
	// Token from context/env
	if t := viper.GetString("contexts.default.token"); t != "" {
		opts.Token = t
	} else if t, ok := getEnv("RUNE_TOKEN"); ok {
		opts.Token = t
	}
	return opts
}

// newAPIClient creates a new client using buildClientOptions with optional address and token overrides.
func newAPIClient(addressOverride string, tokenOverride string) (*client.Client, error) {
	opts := buildClientOptions(addressOverride)
	if tokenOverride != "" {
		opts.Token = tokenOverride
	}
	return client.NewClient(opts)
}
