package main

import (
	"context"
	"os"

	"github.com/rzbill/rune/internal/config"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/viper"
)

// bootstrapAndResolveRegistryAuth creates/updates referenced Secrets based on runefile config
// and materializes credentials into viper's docker.registries for runner consumption.
func bootstrapAndResolveRegistryAuth(cfg *config.Config, st store.Store, logger log.Logger) error {
	if cfg == nil {
		return nil
	}
	regs := cfg.Docker.Registries
	if len(regs) == 0 {
		return nil
	}
	// For bootstrap we need encryption; if store opts don't include KEK, proceed but secrets won't be encrypted in memory store.
	secRepo := repos.NewSecretRepo(st)

	// Build a new slice of registry maps to set back into viper
	var outRegs []map[string]any
	for _, r := range regs {
		entry := map[string]any{
			"name":     r.Name,
			"registry": r.Registry,
		}
		auth := map[string]any{}
		// Copy simple fields
		if r.Auth.Type != "" {
			auth["type"] = r.Auth.Type
		}
		if r.Auth.Region != "" {
			auth["region"] = r.Auth.Region
		}

		// Handle fromSecret cases
		var fromSecretName string
		var fromSecretNS string
		switch v := r.Auth.FromSecret.(type) {
		case string:
			if v != "" {
				fromSecretName = v
			}
		case map[string]any:
			if n, ok := v["name"].(string); ok {
				fromSecretName = n
			}
			if ns, ok := v["namespace"].(string); ok {
				fromSecretNS = ns
			}
		}
		if fromSecretName != "" {
			if fromSecretNS == "" {
				fromSecretNS = "system"
			}
			// Bootstrap if requested
			if r.Auth.Bootstrap {
				// prepare data map from env-expanded values
				data := map[string]string{}
				for k, val := range r.Auth.Data {
					data[k] = os.ExpandEnv(val)
				}
				if len(data) > 0 {
					ref := types.FormatRef(types.ResourceTypeSecret, fromSecretNS, fromSecretName)
					existing, err := secRepo.Get(context.Background(), ref)
					if err != nil {
						// create
						s := &types.Secret{Name: fromSecretName, Namespace: fromSecretNS, Data: data, Type: "static"}
						if cerr := secRepo.Create(context.Background(), s); cerr != nil {
							logger.Error("Failed to create registry secret", log.Str("ref", ref), log.Err(cerr))
							return cerr
						}
						logger.Info("Created registry secret", log.Str("ref", ref))
					} else if !r.Auth.Immutable {
						// update if different and manage != ignore
						if r.Auth.Manage != "ignore" {
							// naive diff by JSON stringification via types; here just overwrite
							s := &types.Secret{Name: fromSecretName, Namespace: fromSecretNS, Data: data, Type: existing.Type}
							if uerr := secRepo.Update(context.Background(), ref, s); uerr != nil {
								logger.Error("Failed to update registry secret", log.Str("ref", ref), log.Err(uerr))
								return uerr
							}
							logger.Info("Updated registry secret", log.Str("ref", ref))
						}
					}
				}
			}
			// Resolve secret now
			ref := types.FormatRef(types.ResourceTypeSecret, fromSecretNS, fromSecretName)
			s, err := secRepo.Get(context.Background(), ref)
			if err != nil {
				logger.Error("Failed to fetch registry secret", log.Str("ref", ref), log.Err(err))
				return err
			}
			// infer keys: .dockerconfigjson > token > username/password > ecr keys
			if val, ok := s.Data[".dockerconfigjson"]; ok && val != "" {
				auth["type"] = "dockerconfigjson"
				auth["dockerconfigjson"] = val
			} else if tk, ok := s.Data["token"]; ok && tk != "" {
				auth["type"] = "token"
				auth["token"] = tk
			} else if u, uok := s.Data["username"]; uok {
				if p, pok := s.Data["password"]; pok {
					auth["type"] = "basic"
					auth["username"] = u
					auth["password"] = p
				}
			} else if ak, ok := s.Data["awsAccessKeyId"]; ok {
				// ECR explicit keys; region is in runefile
				auth["type"] = "ecr"
				auth["awsAccessKeyId"] = ak
				if sk, ok := s.Data["awsSecretAccessKey"]; ok {
					auth["awsSecretAccessKey"] = sk
				}
				if st, ok := s.Data["awsSessionToken"]; ok {
					auth["awsSessionToken"] = st
				}
			}
		} else {
			// If direct fields provided in runefile, expand env and set
			if r.Auth.Username != "" {
				auth["username"] = os.ExpandEnv(r.Auth.Username)
			}
			if r.Auth.Password != "" {
				auth["password"] = os.ExpandEnv(r.Auth.Password)
			}
			if r.Auth.Token != "" {
				auth["token"] = os.ExpandEnv(r.Auth.Token)
			}
		}

		entry["auth"] = auth
		outRegs = append(outRegs, entry)
	}

	// Write back into viper so runner manager can read
	viper.Set("docker.registries", outRegs)
	return nil
}
