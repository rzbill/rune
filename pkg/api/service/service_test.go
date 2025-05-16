package service

import (
	"context"
	"testing"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestScaleService(t *testing.T) {
	// Create in-memory store
	store := store.NewMemoryStore()
	require.NotNil(t, store)

	// Create service service with logger
	logger := log.NewTestLogger()
	svc := NewServiceService(store, logger)

	// Create test service
	testService := &types.Service{
		ID:        "test-service-id",
		Name:      "test-service",
		Namespace: "default",
		Scale:     1,
		Metadata: &types.ServiceMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	// Store test service
	ctx := context.Background()
	err := store.Create(ctx, types.ResourceTypeService, testService.Namespace, testService.Name, testService)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name      string
		request   *generated.ScaleServiceRequest
		wantScale int
		wantErr   codes.Code
	}{
		{
			name: "scale_up",
			request: &generated.ScaleServiceRequest{
				Name:      testService.Name,
				Namespace: testService.Namespace,
				Scale:     3,
			},
			wantScale: 3,
			wantErr:   codes.OK,
		},
		{
			name: "scale_down",
			request: &generated.ScaleServiceRequest{
				Name:      testService.Name,
				Namespace: testService.Namespace,
				Scale:     0,
			},
			wantScale: 0,
			wantErr:   codes.OK,
		},
		{
			name: "invalid_scale",
			request: &generated.ScaleServiceRequest{
				Name:      testService.Name,
				Namespace: testService.Namespace,
				Scale:     -1,
			},
			wantScale: 0, // Previous value preserved
			wantErr:   codes.InvalidArgument,
		},
		{
			name: "service_not_found",
			request: &generated.ScaleServiceRequest{
				Name:      "nonexistent-service",
				Namespace: testService.Namespace,
				Scale:     3,
			},
			wantScale: 0, // Not applicable
			wantErr:   codes.NotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call ScaleService
			resp, err := svc.ScaleService(ctx, tc.request)

			// Check error
			if tc.wantErr != codes.OK {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok, "expected gRPC status error")
				assert.Equal(t, tc.wantErr, st.Code(), "unexpected error code")
				return
			}

			// Check success
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NotNil(t, resp.Service)
			assert.Equal(t, tc.wantScale, int(resp.Service.Scale), "unexpected scale value")

			// Verify service was updated in the store
			var updatedService types.Service
			err = store.Get(ctx, types.ResourceTypeService, tc.request.Namespace, tc.request.Name, &updatedService)
			require.NoError(t, err)
			assert.Equal(t, tc.wantScale, updatedService.Scale, "service scale not updated in store")
		})
	}
}
