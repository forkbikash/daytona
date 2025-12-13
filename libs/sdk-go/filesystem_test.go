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
	"time"

	daytona "github.com/forkbikash/daytona/libs/sdk-go"
)

func TestFileSystemOperations(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for filesystem tests
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

	testDir := "/tmp/sdk-test-" + time.Now().Format("20060102150405")

	t.Run("create folder", func(t *testing.T) {
		err := sandbox.FS.CreateFolder(ctx, testDir, "755")
		if err != nil {
			t.Fatalf("Failed to create folder: %v", err)
		}
	})

	t.Run("upload file from buffer", func(t *testing.T) {
		content := []byte("Hello, World! This is a test file.")
		err := sandbox.FS.UploadFile(ctx, content, testDir+"/test.txt")
		if err != nil {
			t.Fatalf("Failed to upload file: %v", err)
		}
	})

	t.Run("download file to memory", func(t *testing.T) {
		content, err := sandbox.FS.DownloadFile(ctx, testDir+"/test.txt")
		if err != nil {
			t.Fatalf("Failed to download file: %v", err)
		}
		if string(content) != "Hello, World! This is a test file." {
			t.Errorf("Expected content 'Hello, World! This is a test file.', got: %s", string(content))
		}
	})

	t.Run("upload file from local path", func(t *testing.T) {
		// Create a local temp file
		tmpFile, err := os.CreateTemp("", "daytona-test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		content := "Content from local file"
		if _, err := tmpFile.WriteString(content); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tmpFile.Close()

		err = sandbox.FS.UploadFileFromPath(ctx, tmpFile.Name(), testDir+"/from_local.txt")
		if err != nil {
			t.Fatalf("Failed to upload file from path: %v", err)
		}

		// Verify
		downloaded, err := sandbox.FS.DownloadFile(ctx, testDir+"/from_local.txt")
		if err != nil {
			t.Fatalf("Failed to download file: %v", err)
		}
		if string(downloaded) != content {
			t.Errorf("Expected content '%s', got: %s", content, string(downloaded))
		}
	})

	t.Run("download file to local path", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "daytona-download-test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		localPath := filepath.Join(tmpDir, "downloaded.txt")
		err = sandbox.FS.DownloadFileToPath(ctx, testDir+"/test.txt", localPath)
		if err != nil {
			t.Fatalf("Failed to download file to path: %v", err)
		}

		content, err := os.ReadFile(localPath)
		if err != nil {
			t.Fatalf("Failed to read downloaded file: %v", err)
		}
		if string(content) != "Hello, World! This is a test file." {
			t.Errorf("Expected content 'Hello, World! This is a test file.', got: %s", string(content))
		}
	})

	t.Run("list files", func(t *testing.T) {
		files, err := sandbox.FS.ListFiles(ctx, testDir)
		if err != nil {
			t.Fatalf("Failed to list files: %v", err)
		}
		if len(files) < 2 {
			t.Errorf("Expected at least 2 files, got %d", len(files))
		}

		foundTest := false
		foundFromLocal := false
		for _, f := range files {
			if f.Name == "test.txt" {
				foundTest = true
			}
			if f.Name == "from_local.txt" {
				foundFromLocal = true
			}
		}
		if !foundTest {
			t.Error("Expected to find test.txt in file list")
		}
		if !foundFromLocal {
			t.Error("Expected to find from_local.txt in file list")
		}
	})

	t.Run("get file details", func(t *testing.T) {
		info, err := sandbox.FS.GetFileDetails(ctx, testDir+"/test.txt")
		if err != nil {
			t.Fatalf("Failed to get file details: %v", err)
		}
		if info.Name != "test.txt" {
			t.Errorf("Expected file name 'test.txt', got: %s", info.Name)
		}
		if info.IsDir {
			t.Error("Expected file to not be a directory")
		}
	})

	t.Run("search files by name", func(t *testing.T) {
		result, err := sandbox.FS.SearchFiles(ctx, testDir, "*.txt")
		if err != nil {
			t.Fatalf("Failed to search files: %v", err)
		}
		if len(result.Files) < 2 {
			t.Errorf("Expected at least 2 matching files, got %d", len(result.Files))
		}
	})

	t.Run("find in files", func(t *testing.T) {
		// Upload a file with searchable content
		content := []byte("This file contains the word SEARCHME for testing.")
		err := sandbox.FS.UploadFile(ctx, content, testDir+"/searchable.txt")
		if err != nil {
			t.Fatalf("Failed to upload searchable file: %v", err)
		}

		matches, err := sandbox.FS.FindFiles(ctx, testDir, "SEARCHME")
		if err != nil {
			t.Fatalf("Failed to find in files: %v", err)
		}
		if len(matches) == 0 {
			t.Error("Expected to find matches for 'SEARCHME'")
		}
	})

	t.Run("move file", func(t *testing.T) {
		err := sandbox.FS.MoveFiles(ctx, testDir+"/test.txt", testDir+"/moved.txt")
		if err != nil {
			t.Fatalf("Failed to move file: %v", err)
		}

		// Verify the file was moved
		_, err = sandbox.FS.GetFileDetails(ctx, testDir+"/moved.txt")
		if err != nil {
			t.Error("Expected moved file to exist")
		}

		// Original should not exist
		_, err = sandbox.FS.GetFileDetails(ctx, testDir+"/test.txt")
		if err == nil {
			t.Error("Expected original file to not exist after move")
		}
	})

	t.Run("replace in files", func(t *testing.T) {
		// Upload a file with content to replace
		content := []byte("Replace OLD_VALUE with new value")
		err := sandbox.FS.UploadFile(ctx, content, testDir+"/replace.txt")
		if err != nil {
			t.Fatalf("Failed to upload file: %v", err)
		}

		results, err := sandbox.FS.ReplaceInFiles(ctx, []string{testDir + "/replace.txt"}, "OLD_VALUE", "NEW_VALUE")
		if err != nil {
			t.Fatalf("Failed to replace in files: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected replacement results")
		}

		// Verify replacement
		downloaded, err := sandbox.FS.DownloadFile(ctx, testDir+"/replace.txt")
		if err != nil {
			t.Fatalf("Failed to download file: %v", err)
		}
		if !strings.Contains(string(downloaded), "NEW_VALUE") {
			t.Error("Expected file to contain 'NEW_VALUE' after replacement")
		}
	})

	t.Run("set file permissions", func(t *testing.T) {
		err := sandbox.FS.SetFilePermissions(ctx, testDir+"/moved.txt", daytona.FilePermissionsParams{
			Mode: "644",
		})
		if err != nil {
			t.Fatalf("Failed to set file permissions: %v", err)
		}
	})

	t.Run("delete file", func(t *testing.T) {
		err := sandbox.FS.DeleteFile(ctx, testDir+"/moved.txt", false)
		if err != nil {
			t.Fatalf("Failed to delete file: %v", err)
		}

		// Verify deletion
		_, err = sandbox.FS.GetFileDetails(ctx, testDir+"/moved.txt")
		if err == nil {
			t.Error("Expected file to be deleted")
		}
	})

	t.Run("delete folder recursively", func(t *testing.T) {
		err := sandbox.FS.DeleteFile(ctx, testDir, true)
		if err != nil {
			t.Fatalf("Failed to delete folder: %v", err)
		}

		// Verify deletion
		_, err = sandbox.FS.GetFileDetails(ctx, testDir)
		if err == nil {
			t.Error("Expected folder to be deleted")
		}
	})
}

func TestFileSystemMultipleFiles(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for filesystem tests
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

	testDir := "/tmp/sdk-multi-test-" + time.Now().Format("20060102150405")

	// Create test directory
	err = sandbox.FS.CreateFolder(ctx, testDir, "755")
	if err != nil {
		t.Fatalf("Failed to create folder: %v", err)
	}

	t.Run("upload multiple files", func(t *testing.T) {
		files := []daytona.FileUpload{
			{Source: []byte("File 1 content"), Destination: testDir + "/file1.txt"},
			{Source: []byte("File 2 content"), Destination: testDir + "/file2.txt"},
			{Source: []byte("File 3 content"), Destination: testDir + "/file3.txt"},
		}

		err := sandbox.FS.UploadFiles(ctx, files, 60)
		if err != nil {
			t.Fatalf("Failed to upload multiple files: %v", err)
		}

		// Verify all files exist
		for i := 1; i <= 3; i++ {
			_, err := sandbox.FS.GetFileDetails(ctx, testDir+"/file"+string(rune('0'+i))+".txt")
			if err != nil {
				t.Errorf("Expected file%d.txt to exist", i)
			}
		}
	})

	t.Run("download multiple files", func(t *testing.T) {
		requests := []daytona.FileDownloadRequest{
			{Source: testDir + "/file1.txt"},
			{Source: testDir + "/file2.txt"},
		}

		responses, err := sandbox.FS.DownloadFiles(ctx, requests, 60)
		if err != nil {
			t.Fatalf("Failed to download multiple files: %v", err)
		}

		if len(responses) != 2 {
			t.Errorf("Expected 2 responses, got %d", len(responses))
		}

		for _, resp := range responses {
			if resp.Error != "" {
				t.Errorf("Expected no error, got: %s", resp.Error)
			}
		}
	})

	// Cleanup
	sandbox.FS.DeleteFile(ctx, testDir, true)
}
