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

func TestGitOperations(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for git tests
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

	repoPath := "/tmp/git-test-" + time.Now().Format("20060102150405")

	t.Run("clone public repository", func(t *testing.T) {
		err := sandbox.Git.Clone(ctx, "https://github.com/octocat/Hello-World.git", repoPath, nil)
		if err != nil {
			t.Fatalf("Failed to clone repository: %v", err)
		}

		// Verify the clone by checking for a file
		_, err = sandbox.FS.GetFileDetails(ctx, repoPath+"/README")
		if err != nil {
			t.Error("Expected README to exist after clone")
		}
	})

	t.Run("get git status", func(t *testing.T) {
		status, err := sandbox.Git.Status(ctx, repoPath)
		if err != nil {
			t.Fatalf("Failed to get git status: %v", err)
		}
		if status.CurrentBranch == "" {
			t.Error("Expected current branch to be set")
		}
		t.Logf("Current branch: %s", status.CurrentBranch)
	})

	t.Run("list branches", func(t *testing.T) {
		branches, err := sandbox.Git.Branches(ctx, repoPath)
		if err != nil {
			t.Fatalf("Failed to list branches: %v", err)
		}
		if len(branches.Branches) == 0 {
			t.Error("Expected at least one branch")
		}
		t.Logf("Branches: %v", branches.Branches)
	})

	t.Run("create branch", func(t *testing.T) {
		branchName := "test-branch-" + time.Now().Format("20060102150405")
		err := sandbox.Git.CreateBranch(ctx, repoPath, branchName)
		if err != nil {
			t.Fatalf("Failed to create branch: %v", err)
		}

		// Verify branch exists
		branches, err := sandbox.Git.Branches(ctx, repoPath)
		if err != nil {
			t.Fatalf("Failed to list branches: %v", err)
		}

		found := false
		for _, b := range branches.Branches {
			if b == branchName {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected new branch to exist")
		}
	})

	t.Run("checkout branch", func(t *testing.T) {
		// First create a branch to checkout
		branchName := "checkout-test-" + time.Now().Format("20060102150405")
		err := sandbox.Git.CreateBranch(ctx, repoPath, branchName)
		if err != nil {
			t.Fatalf("Failed to create branch: %v", err)
		}

		err = sandbox.Git.CheckoutBranch(ctx, repoPath, branchName)
		if err != nil {
			t.Fatalf("Failed to checkout branch: %v", err)
		}

		// Verify checkout
		status, err := sandbox.Git.Status(ctx, repoPath)
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}
		if status.CurrentBranch != branchName {
			t.Errorf("Expected current branch to be %s, got %s", branchName, status.CurrentBranch)
		}
	})

	t.Run("add and commit", func(t *testing.T) {
		// Create a new file
		content := []byte("Test file for git commit")
		err := sandbox.FS.UploadFile(ctx, content, repoPath+"/test-commit.txt")
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Add the file
		err = sandbox.Git.Add(ctx, repoPath, []string{"test-commit.txt"})
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}

		// Commit
		response, err := sandbox.Git.Commit(ctx, repoPath, "Test commit from Go SDK", "Test Author", "test@example.com", false)
		if err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
		if response.SHA == "" {
			t.Error("Expected commit SHA to be non-empty")
		}
		t.Logf("Commit SHA: %s", response.SHA)
	})

	t.Run("delete branch", func(t *testing.T) {
		// Create a branch to delete
		branchName := "delete-test-" + time.Now().Format("20060102150405")
		err := sandbox.Git.CreateBranch(ctx, repoPath, branchName)
		if err != nil {
			t.Fatalf("Failed to create branch: %v", err)
		}

		// Make sure we're not on the branch we're deleting
		status, _ := sandbox.Git.Status(ctx, repoPath)
		if status.CurrentBranch == branchName {
			sandbox.Git.CheckoutBranch(ctx, repoPath, "master")
		}

		err = sandbox.Git.DeleteBranch(ctx, repoPath, branchName)
		if err != nil {
			t.Fatalf("Failed to delete branch: %v", err)
		}

		// Verify branch is deleted
		branches, err := sandbox.Git.Branches(ctx, repoPath)
		if err != nil {
			t.Fatalf("Failed to list branches: %v", err)
		}

		for _, b := range branches.Branches {
			if b == branchName {
				t.Error("Expected branch to be deleted")
			}
		}
	})
}

func TestGitCloneWithOptions(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for git tests
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

	t.Run("clone specific branch", func(t *testing.T) {
		repoPath := "/tmp/git-branch-test-" + time.Now().Format("20060102150405")

		err := sandbox.Git.Clone(ctx, "https://github.com/octocat/Hello-World.git", repoPath, &daytona.CloneOptions{
			Branch: "test",
		})
		if err != nil {
			t.Fatalf("Failed to clone repository with branch: %v", err)
		}

		// Verify we're on the correct branch
		status, err := sandbox.Git.Status(ctx, repoPath)
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}
		if status.CurrentBranch != "test" {
			t.Errorf("Expected to be on 'test' branch, got %s", status.CurrentBranch)
		}
	})
}
