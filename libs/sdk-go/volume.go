/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"

	apiclient "github.com/daytonaio/apiclient"
	sdkerrors "github.com/daytonaio/sdk-go/errors"
)

// Volume represents a Daytona Volume which is a shared storage volume for Sandboxes.
type Volume = apiclient.VolumeDto

// VolumeService provides methods for managing Daytona Volumes.
type VolumeService struct {
	volumesAPI apiclient.VolumesAPI
}

// NewVolumeService creates a new VolumeService instance.
func NewVolumeService(volumesAPI apiclient.VolumesAPI) *VolumeService {
	return &VolumeService{
		volumesAPI: volumesAPI,
	}
}

// List returns all available Volumes.
func (v *VolumeService) List(ctx context.Context) ([]Volume, error) {
	response, httpResp, err := v.volumesAPI.ListVolumes(ctx).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Get retrieves a Volume by its name.
// If create is true and the Volume doesn't exist, it will be created.
func (v *VolumeService) Get(ctx context.Context, name string, create bool) (*Volume, error) {
	response, httpResp, err := v.volumesAPI.GetVolumeByName(ctx, name).Execute()
	if err != nil {
		if sdkerrors.IsDaytonaNotFoundError(handleAPIError(err, httpResp)) && create {
			return v.Create(ctx, name)
		}
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Create creates a new Volume with the specified name.
func (v *VolumeService) Create(ctx context.Context, name string) (*Volume, error) {
	req := apiclient.CreateVolume{
		Name: name,
	}
	response, httpResp, err := v.volumesAPI.CreateVolume(ctx).CreateVolume(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Delete deletes a Volume.
func (v *VolumeService) Delete(ctx context.Context, volume *Volume) error {
	httpResp, err := v.volumesAPI.DeleteVolume(ctx, volume.Id).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}
