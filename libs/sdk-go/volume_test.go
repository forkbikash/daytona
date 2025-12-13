/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona_test

import (
	"context"
	"testing"
	"time"

	daytona "github.com/forkbikash/daytona/libs/sdk-go"
)

func TestVolumeOperations(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	volumeName := "go-sdk-test-volume-" + time.Now().Format("20060102150405")
	var createdVolume *daytona.Volume

	t.Run("create volume", func(t *testing.T) {
		var err error
		createdVolume, err = client.Volume.Create(ctx, volumeName)
		if err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}
		if createdVolume == nil {
			t.Fatal("Expected non-nil volume")
		}
		if createdVolume.Name != volumeName {
			t.Errorf("Expected volume name %s, got %s", volumeName, createdVolume.Name)
		}
		t.Logf("Created volume: %s (ID: %s)", createdVolume.Name, createdVolume.Id)
	})

	if createdVolume == nil {
		t.Fatal("Volume was not created, cannot continue tests")
	}

	t.Run("get volume", func(t *testing.T) {
		volume, err := client.Volume.Get(ctx, volumeName, false)
		if err != nil {
			t.Fatalf("Failed to get volume: %v", err)
		}
		if volume.Id != createdVolume.Id {
			t.Errorf("Expected volume ID %s, got %s", createdVolume.Id, volume.Id)
		}
	})

	t.Run("get volume with create flag", func(t *testing.T) {
		newVolumeName := "go-sdk-test-volume-create-" + time.Now().Format("20060102150405")
		volume, err := client.Volume.Get(ctx, newVolumeName, true)
		if err != nil {
			t.Fatalf("Failed to get/create volume: %v", err)
		}
		if volume.Name != newVolumeName {
			t.Errorf("Expected volume name %s, got %s", newVolumeName, volume.Name)
		}

		// Cleanup the extra volume
		client.Volume.Delete(ctx, volume)
	})

	t.Run("list volumes", func(t *testing.T) {
		volumes, err := client.Volume.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list volumes: %v", err)
		}

		found := false
		for _, v := range volumes {
			if v.Id == createdVolume.Id {
				found = true
				break
			}
		}
		if !found {
			t.Error("Created volume not found in list")
		}
	})

	// Cleanup
	t.Run("delete volume", func(t *testing.T) {
		err := client.Volume.Delete(ctx, createdVolume)
		if err != nil {
			t.Fatalf("Failed to delete volume: %v", err)
		}

		// Verify deletion
		_, err = client.Volume.Get(ctx, volumeName, false)
		if err == nil {
			t.Error("Expected error when getting deleted volume")
		}
	})
}

func TestVolumeWithSandbox(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	volumeName := "go-sdk-test-volume-sandbox-" + time.Now().Format("20060102150405")

	// Create a volume
	volume, err := client.Volume.Create(ctx, volumeName)
	if err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}
	defer client.Volume.Delete(ctx, volume)

	t.Run("create sandbox with volume mount", func(t *testing.T) {
		sandbox, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
			CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
				Language: daytona.CodeLanguagePython,
				Volumes: []daytona.VolumeMount{
					{
						VolumeID:  volume.Id,
						MountPath: "/mnt/shared",
					},
				},
			},
		}, &daytona.CreateOptions{
			Timeout: 120,
		})
		if err != nil {
			t.Fatalf("Failed to create sandbox with volume: %v", err)
		}
		defer sandbox.Delete(ctx, 60)

		// Write a file to the volume
		content := []byte("Data stored in volume")
		err = sandbox.FS.UploadFile(ctx, content, "/mnt/shared/test-data.txt")
		if err != nil {
			t.Fatalf("Failed to write to volume: %v", err)
		}

		// Verify the file exists
		downloaded, err := sandbox.FS.DownloadFile(ctx, "/mnt/shared/test-data.txt")
		if err != nil {
			t.Fatalf("Failed to read from volume: %v", err)
		}
		if string(downloaded) != "Data stored in volume" {
			t.Errorf("Expected content 'Data stored in volume', got: %s", string(downloaded))
		}
	})

	t.Run("verify volume persistence across sandboxes", func(t *testing.T) {
		// Create first sandbox and write data
		sandbox1, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
			CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
				Language: daytona.CodeLanguagePython,
				Volumes: []daytona.VolumeMount{
					{
						VolumeID:  volume.Id,
						MountPath: "/mnt/shared",
					},
				},
			},
		}, &daytona.CreateOptions{
			Timeout: 120,
		})
		if err != nil {
			t.Fatalf("Failed to create sandbox1: %v", err)
		}

		// Write data from sandbox1
		content := []byte("Persistent data from sandbox1")
		err = sandbox1.FS.UploadFile(ctx, content, "/mnt/shared/persistent.txt")
		if err != nil {
			t.Fatalf("Failed to write to volume from sandbox1: %v", err)
		}

		// Delete sandbox1
		sandbox1.Delete(ctx, 60)

		// Create second sandbox with the same volume
		sandbox2, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
			CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
				Language: daytona.CodeLanguagePython,
				Volumes: []daytona.VolumeMount{
					{
						VolumeID:  volume.Id,
						MountPath: "/mnt/shared",
					},
				},
			},
		}, &daytona.CreateOptions{
			Timeout: 120,
		})
		if err != nil {
			t.Fatalf("Failed to create sandbox2: %v", err)
		}
		defer sandbox2.Delete(ctx, 60)

		// Read data from sandbox2
		downloaded, err := sandbox2.FS.DownloadFile(ctx, "/mnt/shared/persistent.txt")
		if err != nil {
			t.Fatalf("Failed to read from volume in sandbox2: %v", err)
		}
		if string(downloaded) != "Persistent data from sandbox1" {
			t.Errorf("Expected persistent data, got: %s", string(downloaded))
		}
	})
}
