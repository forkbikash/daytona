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

func TestSnapshotOperations(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	snapshotName := "go-sdk-test-snapshot-" + time.Now().Format("20060102150405")
	var createdSnapshot *daytona.Snapshot

	t.Run("create snapshot from image", func(t *testing.T) {
		var err error
		createdSnapshot, err = client.Snapshot.Create(ctx, &daytona.CreateSnapshotParams{
			Name:  snapshotName,
			Image: "python:3.12-slim",
		}, &daytona.CreateSnapshotOptions{
			OnLogs: func(chunk string) {
				t.Logf("Snapshot log: %s", chunk)
			},
			Timeout: 300,
		})
		if err != nil {
			t.Fatalf("Failed to create snapshot: %v", err)
		}
		if createdSnapshot == nil {
			t.Fatal("Expected non-nil snapshot")
		}
		if createdSnapshot.Name != snapshotName {
			t.Errorf("Expected snapshot name %s, got %s", snapshotName, createdSnapshot.Name)
		}
		t.Logf("Created snapshot: %s (ID: %s)", createdSnapshot.Name, createdSnapshot.Id)
	})

	if createdSnapshot == nil {
		t.Fatal("Snapshot was not created, cannot continue tests")
	}

	t.Run("get snapshot", func(t *testing.T) {
		snapshot, err := client.Snapshot.Get(ctx, snapshotName)
		if err != nil {
			t.Fatalf("Failed to get snapshot: %v", err)
		}
		if snapshot.Id != createdSnapshot.Id {
			t.Errorf("Expected snapshot ID %s, got %s", createdSnapshot.Id, snapshot.Id)
		}
	})

	t.Run("list snapshots", func(t *testing.T) {
		result, err := client.Snapshot.List(ctx, 1, 10)
		if err != nil {
			t.Fatalf("Failed to list snapshots: %v", err)
		}
		if len(result.Items) == 0 {
			t.Error("Expected at least one snapshot")
		}

		found := false
		for _, s := range result.Items {
			if s.Id == createdSnapshot.Id {
				found = true
				break
			}
		}
		if !found {
			t.Error("Created snapshot not found in list")
		}
	})

	t.Run("create sandbox from snapshot", func(t *testing.T) {
		sandbox, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
			CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
				Language: daytona.CodeLanguagePython,
			},
			Snapshot: snapshotName,
		}, &daytona.CreateOptions{
			Timeout: 120,
		})
		if err != nil {
			t.Fatalf("Failed to create sandbox from snapshot: %v", err)
		}
		defer sandbox.Delete(ctx, 60)

		if sandbox.Snapshot == nil || *sandbox.Snapshot != snapshotName {
			t.Error("Expected sandbox to reference the snapshot")
		}

		// Verify Python is available
		response, err := sandbox.Process.CodeRun(ctx, "import sys; print(sys.version)", nil, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		t.Logf("Python version in snapshot: %s", response.Result)
	})

	// Cleanup
	t.Run("delete snapshot", func(t *testing.T) {
		err := client.Snapshot.Delete(ctx, createdSnapshot)
		if err != nil {
			t.Fatalf("Failed to delete snapshot: %v", err)
		}
	})
}

func TestSnapshotWithDynamicImage(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	snapshotName := "go-sdk-dynamic-snapshot-" + time.Now().Format("20060102150405")

	t.Run("create snapshot with dynamic image", func(t *testing.T) {
		// Create a dynamic image
		image := daytona.Base("python:3.12-slim").
			RunCommands("pip install requests").
			Env(map[string]string{"MY_ENV_VAR": "test_value"})

		snapshot, err := client.Snapshot.Create(ctx, &daytona.CreateSnapshotParams{
			Name:  snapshotName,
			Image: image,
		}, &daytona.CreateSnapshotOptions{
			OnLogs: func(chunk string) {
				t.Logf("Snapshot log: %s", chunk)
			},
			Timeout: 600, // Dynamic images take longer to build
		})
		if err != nil {
			t.Fatalf("Failed to create snapshot: %v", err)
		}

		defer client.Snapshot.Delete(ctx, snapshot)

		// Create sandbox and verify the environment
		sandbox, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
			CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
				Language: daytona.CodeLanguagePython,
			},
			Snapshot: snapshotName,
		}, &daytona.CreateOptions{
			Timeout: 120,
		})
		if err != nil {
			t.Fatalf("Failed to create sandbox: %v", err)
		}
		defer sandbox.Delete(ctx, 60)

		// Verify requests is installed
		response, err := sandbox.Process.CodeRun(ctx, "import requests; print('requests installed')", nil, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Output: %s", response.ExitCode, response.Result)
		}
	})
}

func TestSnapshotWithResources(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	snapshotName := "go-sdk-resource-snapshot-" + time.Now().Format("20060102150405")

	t.Run("create snapshot with custom resources", func(t *testing.T) {
		cpu := int32(2)
		memory := int32(4)
		disk := int32(10)

		snapshot, err := client.Snapshot.Create(ctx, &daytona.CreateSnapshotParams{
			Name:  snapshotName,
			Image: "python:3.12-slim",
			Resources: &daytona.Resources{
				CPU:    &cpu,
				Memory: &memory,
				Disk:   &disk,
			},
		}, &daytona.CreateSnapshotOptions{
			OnLogs: func(chunk string) {
				t.Logf("Snapshot log: %s", chunk)
			},
			Timeout: 300,
		})
		if err != nil {
			t.Fatalf("Failed to create snapshot: %v", err)
		}

		defer client.Snapshot.Delete(ctx, snapshot)

		if snapshot.Cpu != float32(cpu) {
			t.Errorf("Expected CPU %d, got %f", cpu, snapshot.Cpu)
		}
		if snapshot.Mem != float32(memory) {
			t.Errorf("Expected Memory %d, got %f", memory, snapshot.Mem)
		}
	})
}
