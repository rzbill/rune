package registryauth

import (
	"context"
)

// BuildProviders constructs providers from normalized registries configuration.
// Each entry is expected to contain keys: name, registry, auth{type, username, password, token, region, dockerconfigjson}
func BuildProviders(ctx context.Context, regs []map[string]any) []Provider {
	var out []Provider
	for _, r := range regs {
		host, _ := r["registry"].(string)
		auth, _ := r["auth"].(map[string]any)
		if auth == nil {
			continue
		}
		t, _ := auth["type"].(string)
		switch t {
		case "basic":
			out = append(out, NewBasicTokenProvider(BasicTokenConfig{
				Registry: host,
				Username: str(auth["username"]),
				Password: str(auth["password"]),
			}))
		case "token":
			out = append(out, NewBasicTokenProvider(BasicTokenConfig{
				Registry: host,
				Token:    str(auth["token"]),
			}))
		case "dockerconfigjson":
			if raw := str(auth["dockerconfigjson"]); raw != "" {
				out = append(out, NewDockerConfigJSONProvider(host, raw))
			}
		case "ecr":
			out = append(out, NewECRProvider(ECRConfig{Registry: host, Region: str(auth["region"])}))
		default:
			// ignore unknown
		}
	}
	return out
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
