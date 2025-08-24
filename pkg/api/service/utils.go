package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
)

func ensureNamespaceExists(ctx context.Context, nsRepo *repos.NamespaceRepo, namespace string, ensureNamespace bool) error {
	// Check if it's a reserved namespace
	if namespace == "system" || namespace == "default" {
		return nil
	}

	// Check if the namespace exists in the store
	_, err := nsRepo.Get(ctx, namespace)
	if err != nil {
		if store.IsNotFoundError(err) {
			// Namespace doesn't exist, check if we should create it
			if ensureNamespace {
				// Create the namespace
				newNs := &types.Namespace{
					Name:      namespace,
					Labels:    map[string]string{"rune/auto-created": "true"},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				if err := nsRepo.Create(ctx, newNs); err != nil {
					return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
				}
				return nil
			} else {
				return fmt.Errorf("namespace %s does not exist (use --create-namespace to create it)", namespace)
			}
		}
		// Some other error occurred
		return fmt.Errorf("failed to check namespace %s: %w", namespace, err)
	}

	// Namespace exists, nothing to do
	return nil
}
