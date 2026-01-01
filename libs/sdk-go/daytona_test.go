/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	daytona "github.com/forkbikash/daytona/libs/sdk-go"
)

// getTestClient creates a Daytona client for testing.
// Requires DAYTONA_API_KEY environment variable to be set.
func getTestClient(t *testing.T) *daytona.Daytona {
	t.Helper()

	client, err := daytona.NewDaytona(nil)
	if err != nil {
		t.Fatalf("Failed to create Daytona client: %v", err)
	}

	return client
}

func TestNewDaytona(t *testing.T) {
	t.Run("create client with environment variables", func(t *testing.T) {
		client, err := daytona.NewDaytona(nil)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		if client == nil {
			t.Fatal("Expected non-nil client")
		}
	})
}

func TestSandboxLifecycle(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	var sandbox *daytona.Sandbox

	t.Run("create sandbox", func(t *testing.T) {
		var err error
		sandbox, err = client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
			CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
				Language: daytona.CodeLanguagePython,
				Labels: map[string]string{
					"test": "go-sdk-test",
				},
			},
		}, &daytona.CreateOptions{
			Timeout: 120,
		})
		if err != nil {
			t.Fatalf("Failed to create sandbox: %v", err)
		}
		if sandbox == nil {
			t.Fatal("Expected non-nil sandbox")
		}
		if sandbox.ID == "" {
			t.Fatal("Expected sandbox to have an ID")
		}
		t.Logf("Created sandbox: %s", sandbox.ID)
	})

	if sandbox == nil {
		t.Fatal("Sandbox was not created, cannot continue tests")
	}

	t.Run("get sandbox", func(t *testing.T) {
		retrieved, err := client.Get(ctx, sandbox.ID)
		if err != nil {
			t.Fatalf("Failed to get sandbox: %v", err)
		}
		if retrieved.ID != sandbox.ID {
			t.Errorf("Expected sandbox ID %s, got %s", sandbox.ID, retrieved.ID)
		}
	})

	t.Run("list sandboxes", func(t *testing.T) {
		result, err := client.List(ctx, map[string]string{"test": "go-sdk-test"}, 1, 10)
		if err != nil {
			t.Fatalf("Failed to list sandboxes: %v", err)
		}
		if len(result.Items) == 0 {
			t.Fatal("Expected at least one sandbox in the list")
		}
		found := false
		for _, s := range result.Items {
			if s.ID == sandbox.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("Created sandbox not found in list")
		}
	})

	t.Run("find one sandbox", func(t *testing.T) {
		found, err := client.FindOne(ctx, daytona.SandboxFilter{
			IDOrName: sandbox.ID,
		})
		if err != nil {
			t.Fatalf("Failed to find sandbox: %v", err)
		}
		if found.ID != sandbox.ID {
			t.Errorf("Expected sandbox ID %s, got %s", sandbox.ID, found.ID)
		}
	})

	t.Run("set labels", func(t *testing.T) {
		labels, err := sandbox.SetLabels(ctx, map[string]string{
			"test":    "go-sdk-test",
			"updated": "true",
		})
		if err != nil {
			t.Fatalf("Failed to set labels: %v", err)
		}
		if labels["updated"] != "true" {
			t.Error("Expected updated label to be set")
		}
	})

	t.Run("get user home dir", func(t *testing.T) {
		homeDir, err := sandbox.GetUserHomeDir(ctx)
		if err != nil {
			t.Fatalf("Failed to get user home dir: %v", err)
		}
		if homeDir == "" {
			t.Error("Expected non-empty home directory")
		}
		t.Logf("User home dir: %s", homeDir)
	})

	t.Run("get work dir", func(t *testing.T) {
		workDir, err := sandbox.GetWorkDir(ctx)
		if err != nil {
			t.Fatalf("Failed to get work dir: %v", err)
		}
		if workDir == "" {
			t.Error("Expected non-empty work directory")
		}
		t.Logf("Work dir: %s", workDir)
	})

	t.Run("refresh data", func(t *testing.T) {
		err := sandbox.RefreshData(ctx)
		if err != nil {
			t.Fatalf("Failed to refresh data: %v", err)
		}
	})

	t.Run("refresh activity", func(t *testing.T) {
		err := sandbox.RefreshActivity(ctx)
		if err != nil {
			t.Fatalf("Failed to refresh activity: %v", err)
		}
	})

	t.Run("set autostop interval", func(t *testing.T) {
		err := sandbox.SetAutostopInterval(ctx, 30)
		if err != nil {
			t.Fatalf("Failed to set autostop interval: %v", err)
		}
	})

	t.Run("get preview link", func(t *testing.T) {
		preview, err := sandbox.GetPreviewLink(ctx, 8080)
		if err != nil {
			t.Fatalf("Failed to get preview link: %v", err)
		}
		if preview.Url == "" {
			t.Error("Expected non-empty preview URL")
		}
		t.Logf("Preview URL: %s", preview.Url)
	})

	t.Run("stop sandbox", func(t *testing.T) {
		err := sandbox.Stop(ctx, 60)
		if err != nil {
			t.Fatalf("Failed to stop sandbox: %v", err)
		}
	})

	t.Run("start sandbox", func(t *testing.T) {
		err := sandbox.Start(ctx, 120)
		if err != nil {
			t.Fatalf("Failed to start sandbox: %v", err)
		}
	})

	// Cleanup
	t.Run("delete sandbox", func(t *testing.T) {
		err := sandbox.Delete(ctx, 60)
		if err != nil {
			t.Fatalf("Failed to delete sandbox: %v", err)
		}
	})
}

func TestCreateSandboxFromImage(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	sandbox, err := client.CreateFromImage(ctx, &daytona.CreateSandboxFromImageParams{
		CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
			Language: daytona.CodeLanguagePython,
			Labels: map[string]string{
				"test": "go-sdk-image-test",
			},
		},
		Image: "python:3.12-slim",
	}, &daytona.CreateOptions{
		Timeout: 180,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox from image: %v", err)
	}

	t.Logf("Created sandbox from image: %s", sandbox.ID)

	// Cleanup
	defer func() {
		if err := sandbox.Delete(ctx, 60); err != nil {
			t.Logf("Failed to delete sandbox: %v", err)
		}
	}()

	if sandbox.ID == "" {
		t.Fatal("Expected sandbox to have an ID")
	}
}

func TestCreateSandboxFromCustomDockerfile(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a dynamic image with custom dockerfile using Base
	customImage := daytona.Base("python:3.12-slim").
		RunCommands("apt-get update", "apt-get install -y curl").
		Env(map[string]string{"MY_ENV": "test-value"}).
		Workdir("/app")

	sandbox, err := client.CreateFromImage(ctx, &daytona.CreateSandboxFromImageParams{
		CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
			Language: daytona.CodeLanguagePython,
			Labels: map[string]string{
				"test": "go-sdk-custom-dockerfile-test",
			},
		},
		DynamicImage: customImage,
	}, &daytona.CreateOptions{
		Timeout: 180,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox from custom dockerfile: %v", err)
	}

	t.Logf("Created sandbox from custom dockerfile: %s", sandbox.ID)

	// Cleanup
	defer func() {
		if err := sandbox.Delete(ctx, 60); err != nil {
			t.Logf("Failed to delete sandbox: %v", err)
		}
	}()

	if sandbox.ID == "" {
		t.Fatal("Expected sandbox to have an ID")
	}

	// Verify the dockerfile was generated correctly
	dockerfile := customImage.Dockerfile()
	if dockerfile == "" {
		t.Error("Expected non-empty dockerfile")
	}
	if !strings.Contains(dockerfile, "FROM python:3.12-slim") {
		t.Error("Expected dockerfile to contain base image")
	}
	if !strings.Contains(dockerfile, "RUN apt-get update") {
		t.Error("Expected dockerfile to contain apt-get update command")
	}
	if !strings.Contains(dockerfile, "ENV MY_ENV=test-value") {
		t.Error("Expected dockerfile to contain environment variable")
	}
	if !strings.Contains(dockerfile, "WORKDIR /app") {
		t.Error("Expected dockerfile to contain workdir")
	}
}

func TestCreateSandboxWithLocalFile(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a temporary file to add to the image
	tmpDir := t.TempDir()
	localFilePath := filepath.Join(tmpDir, "test_script.py")
	fileContent := []byte(`print("Hello from custom dockerfile!")`)
	if err := os.WriteFile(localFilePath, fileContent, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create a dynamic image with AddLocalFile
	customImage := daytona.Base("python:3.12-slim").
		Workdir("/app").
		AddLocalFile(localFilePath, "/app/test_script.py")

	sandbox, err := client.CreateFromImage(ctx, &daytona.CreateSandboxFromImageParams{
		CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
			Language: daytona.CodeLanguagePython,
			Labels: map[string]string{
				"test": "go-sdk-add-local-file-test",
			},
		},
		DynamicImage: customImage,
	}, &daytona.CreateOptions{
		Timeout: 180,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox with local file: %v", err)
	}

	t.Logf("Created sandbox with local file: %s", sandbox.ID)

	// Cleanup
	defer func() {
		if err := sandbox.Delete(ctx, 60); err != nil {
			t.Logf("Failed to delete sandbox: %v", err)
		}
	}()

	if sandbox.ID == "" {
		t.Fatal("Expected sandbox to have an ID")
	}

	// Verify the dockerfile was generated correctly
	dockerfile := customImage.Dockerfile()
	if !strings.Contains(dockerfile, "COPY") {
		t.Error("Expected dockerfile to contain COPY command")
	}

	// Verify the context list includes the local file
	contextList := customImage.ContextList()
	if len(contextList) == 0 {
		t.Fatal("Expected context list to contain the local file")
	}

	found := false
	for _, ctxFile := range contextList {
		if ctxFile.SourcePath == localFilePath {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected context list to contain the temp file path")
	}
}

func TestCreateSandboxFromDebianSlimImage(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a DebianSlim image with pip packages
	debianImage := daytona.DebianSlim("3.12").
		PipInstall([]string{"requests", "numpy"}, nil)

	sandbox, err := client.CreateFromImage(ctx, &daytona.CreateSandboxFromImageParams{
		CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
			Language: daytona.CodeLanguagePython,
			Labels: map[string]string{
				"test": "go-sdk-debian-slim-test",
			},
		},
		DynamicImage: debianImage,
	}, &daytona.CreateOptions{
		Timeout: 180,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox from DebianSlim image: %v", err)
	}

	t.Logf("Created sandbox from DebianSlim image: %s", sandbox.ID)

	// Cleanup
	defer func() {
		if err := sandbox.Delete(ctx, 60); err != nil {
			t.Logf("Failed to delete sandbox: %v", err)
		}
	}()

	if sandbox.ID == "" {
		t.Fatal("Expected sandbox to have an ID")
	}

	// Verify the dockerfile was generated correctly
	dockerfile := debianImage.Dockerfile()
	if dockerfile == "" {
		t.Error("Expected non-empty dockerfile")
	}
	if !strings.Contains(dockerfile, "FROM python:") {
		t.Error("Expected dockerfile to contain python base image")
	}
	if !strings.Contains(dockerfile, "pip install") {
		t.Error("Expected dockerfile to contain pip install command")
	}
}

func TestSshAccess(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for SSH tests
	sandbox, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
		CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
			Language: daytona.CodeLanguagePython,
		},
	}, &daytona.CreateOptions{
		Timeout: 120,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer func() {
		sandbox.Delete(ctx, 60)
	}()

	var sshToken string

	t.Run("create SSH access", func(t *testing.T) {
		expiresIn := float32(60)
		sshAccess, err := sandbox.CreateSshAccess(ctx, &expiresIn)
		if err != nil {
			t.Fatalf("Failed to create SSH access: %v", err)
		}
		if sshAccess.Token == "" {
			t.Error("Expected non-empty SSH token")
		}
		sshToken = sshAccess.Token
		t.Logf("SSH access ID: %s", sshAccess.Id)
	})

	t.Run("validate SSH access", func(t *testing.T) {
		if sshToken == "" {
			t.Skip("No SSH token available")
		}
		validation, err := sandbox.ValidateSshAccess(ctx, sshToken)
		if err != nil {
			t.Fatalf("Failed to validate SSH access: %v", err)
		}
		if !validation.Valid {
			t.Error("Expected SSH token to be valid")
		}
	})

	t.Run("revoke SSH access", func(t *testing.T) {
		if sshToken == "" {
			t.Skip("No SSH token available")
		}
		err := sandbox.RevokeSshAccess(ctx, sshToken)
		if err != nil {
			t.Fatalf("Failed to revoke SSH access: %v", err)
		}
	})
}
