/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"
	"time"

	apiclient "github.com/forkbikash/daytona/libs/api-client-go"
	sdkerrors "github.com/forkbikash/daytona/libs/sdk-go/errors"
)

// Snapshot represents a Daytona Snapshot which is a pre-configured sandbox.
type Snapshot = apiclient.SnapshotDto

// PaginatedSnapshots represents a paginated list of Daytona Snapshots.
type PaginatedSnapshots struct {
	Items      []Snapshot
	Total      float32
	Page       float32
	TotalPages float32
}

// CreateSnapshotParams contains parameters for creating a new snapshot.
type CreateSnapshotParams struct {
	// Name is the name of the snapshot.
	Name string
	// Image is the image to use. Can be a string (image name) or *Image.
	Image interface{} // string or *Image
	// Resources contains resource allocation for the snapshot.
	Resources *Resources
	// Entrypoint is the entrypoint for the snapshot.
	Entrypoint []string
	// SkipValidation skips validation during creation.
	SkipValidation bool
}

// CreateSnapshotOptions contains options for the create operation.
type CreateSnapshotOptions struct {
	// OnLogs is a callback function to handle snapshot creation logs.
	OnLogs func(chunk string)
	// Timeout is the timeout in seconds (0 means no timeout).
	Timeout int
}

// SnapshotService provides methods for managing Daytona Snapshots.
type SnapshotService struct {
	clientConfig     *apiclient.Configuration
	snapshotsAPI     apiclient.SnapshotsAPI
	objectStorageAPI apiclient.ObjectStorageAPI
}

// NewSnapshotService creates a new SnapshotService instance.
func NewSnapshotService(clientConfig *apiclient.Configuration, snapshotsAPI apiclient.SnapshotsAPI, objectStorageAPI apiclient.ObjectStorageAPI) *SnapshotService {
	return &SnapshotService{
		clientConfig:     clientConfig,
		snapshotsAPI:     snapshotsAPI,
		objectStorageAPI: objectStorageAPI,
	}
}

// List returns a paginated list of Snapshots.
func (s *SnapshotService) List(ctx context.Context, page, limit float32) (*PaginatedSnapshots, error) {
	req := s.snapshotsAPI.GetAllSnapshots(ctx)
	if page > 0 {
		req = req.Page(page)
	}
	if limit > 0 {
		req = req.Limit(limit)
	}

	response, httpResp, err := req.Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	return &PaginatedSnapshots{
		Items:      response.Items,
		Total:      response.Total,
		Page:       response.Page,
		TotalPages: response.TotalPages,
	}, nil
}

// Get retrieves a Snapshot by its name.
func (s *SnapshotService) Get(ctx context.Context, name string) (*Snapshot, error) {
	response, httpResp, err := s.snapshotsAPI.GetSnapshot(ctx, name).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Delete deletes a Snapshot.
func (s *SnapshotService) Delete(ctx context.Context, snapshot *Snapshot) error {
	httpResp, err := s.snapshotsAPI.RemoveSnapshot(ctx, snapshot.Id).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Create creates a new Snapshot.
func (s *SnapshotService) Create(ctx context.Context, params *CreateSnapshotParams, opts *CreateSnapshotOptions) (*Snapshot, error) {
	if opts == nil {
		opts = &CreateSnapshotOptions{}
	}

	createReq := apiclient.CreateSnapshot{
		Name: params.Name,
	}

	switch img := params.Image.(type) {
	case string:
		createReq.ImageName = &img
		if len(params.Entrypoint) > 0 {
			createReq.Entrypoint = params.Entrypoint
		}
	case *Image:
		dockerfile := img.Dockerfile()
		if len(params.Entrypoint) > 0 {
			// Add entrypoint to dockerfile
			img = img.Entrypoint(params.Entrypoint)
			dockerfile = img.Dockerfile()
		}
		buildInfo := apiclient.CreateBuildInfo{
			DockerfileContent: dockerfile,
			ContextHashes:     []string{},
		}
		createReq.BuildInfo = &buildInfo
	}

	if params.Resources != nil {
		createReq.Cpu = params.Resources.CPU
		createReq.Gpu = params.Resources.GPU
		createReq.Memory = params.Resources.Memory
		createReq.Disk = params.Resources.Disk
	}

	createReq.SkipValidation = &params.SkipValidation

	// Create the snapshot
	response, httpResp, err := s.snapshotsAPI.CreateSnapshot(ctx).CreateSnapshot(createReq).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	if response == nil {
		return nil, sdkerrors.NewDaytonaError("Failed to create snapshot. Didn't receive a snapshot from the server API.", 0, nil)
	}

	createdSnapshot := response

	terminalStates := map[apiclient.SnapshotState]bool{
		apiclient.SNAPSHOTSTATE_ACTIVE:       true,
		apiclient.SNAPSHOTSTATE_ERROR:        true,
		apiclient.SNAPSHOTSTATE_BUILD_FAILED: true,
	}

	if opts.OnLogs != nil {
		opts.OnLogs("Creating snapshot " + createdSnapshot.Name + " (" + string(createdSnapshot.State) + ")")
	}

	// Poll for completion
	for !terminalStates[createdSnapshot.State] {
		time.Sleep(1 * time.Second)
		updatedSnapshot, err := s.Get(ctx, createdSnapshot.Name)
		if err != nil {
			return nil, err
		}
		createdSnapshot = updatedSnapshot

		if opts.OnLogs != nil {
			opts.OnLogs("Creating snapshot " + createdSnapshot.Name + " (" + string(createdSnapshot.State) + ")")
		}
	}

	if opts.OnLogs != nil && createdSnapshot.State == apiclient.SNAPSHOTSTATE_ACTIVE {
		opts.OnLogs("Created snapshot " + createdSnapshot.Name + " (" + string(createdSnapshot.State) + ")")
	}

	if createdSnapshot.State == apiclient.SNAPSHOTSTATE_ERROR || createdSnapshot.State == apiclient.SNAPSHOTSTATE_BUILD_FAILED {
		errMsg := "Failed to create snapshot. Name: " + createdSnapshot.Name
		if reason := createdSnapshot.ErrorReason.Get(); reason != nil {
			errMsg += " Reason: " + *reason
		}
		return nil, sdkerrors.NewDaytonaError(errMsg, 0, nil)
	}

	return createdSnapshot, nil
}

// Activate activates a Snapshot.
func (s *SnapshotService) Activate(ctx context.Context, snapshot *Snapshot) (*Snapshot, error) {
	response, httpResp, err := s.snapshotsAPI.ActivateSnapshot(ctx, snapshot.Id).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}
