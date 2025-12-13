/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona_test

import (
	"context"
	"strings"
	"testing"
	"time"

	daytona "github.com/forkbikash/daytona/libs/sdk-go"
)

func TestProcessExecuteCommand(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for process tests
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

	t.Run("execute simple command", func(t *testing.T) {
		response, err := sandbox.Process.ExecuteCommand(ctx, `echo "Hello, World!"`, "", nil, nil)
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		if !strings.Contains(response.Result, "Hello, World!") {
			t.Errorf("Expected output to contain 'Hello, World!', got: %s", response.Result)
		}
	})

	t.Run("execute command with cwd", func(t *testing.T) {
		response, err := sandbox.Process.ExecuteCommand(ctx, "pwd", "/tmp", nil, nil)
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		if !strings.Contains(response.Result, "/tmp") {
			t.Errorf("Expected output to contain '/tmp', got: %s", response.Result)
		}
	})

	t.Run("execute command with env vars", func(t *testing.T) {
		response, err := sandbox.Process.ExecuteCommand(ctx, `echo $MY_VAR`, "", map[string]string{
			"MY_VAR": "test_value",
		}, nil)
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		if !strings.Contains(response.Result, "test_value") {
			t.Errorf("Expected output to contain 'test_value', got: %s", response.Result)
		}
	})

	t.Run("execute failing command", func(t *testing.T) {
		response, err := sandbox.Process.ExecuteCommand(ctx, "exit 1", "", nil, nil)
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}
		if response.ExitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", response.ExitCode)
		}
	})

	t.Run("execute command with timeout", func(t *testing.T) {
		timeout := float32(5)
		response, err := sandbox.Process.ExecuteCommand(ctx, "sleep 1 && echo done", "", nil, &timeout)
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
	})

	t.Run("create directory via command and verify with filesystem", func(t *testing.T) {
		testDir := "/tmp/cmd-created-dir-" + time.Now().Format("20060102150405")

		// Create a directory using mkdir command
		response, err := sandbox.Process.ExecuteCommand(ctx, "mkdir -p "+testDir, "", nil, nil)
		if err != nil {
			t.Fatalf("Failed to execute mkdir command: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0 for mkdir, got %d", response.ExitCode)
		}

		// Create a file inside the directory
		response, err = sandbox.Process.ExecuteCommand(ctx, "echo 'created via execute command' > "+testDir+"/test-file.txt", "", nil, nil)
		if err != nil {
			t.Fatalf("Failed to execute echo command: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0 for echo, got %d", response.ExitCode)
		}

		// Verify directory exists using filesystem API
		dirInfo, err := sandbox.FS.GetFileDetails(ctx, testDir)
		if err != nil {
			t.Fatalf("Failed to get directory details - directory was not created via ExecuteCommand: %v", err)
		}
		if !dirInfo.IsDir {
			t.Error("Expected path to be a directory")
		}
		t.Logf("Directory created via ExecuteCommand verified: %s", testDir)

		// Verify file exists and has correct content
		content, err := sandbox.FS.DownloadFile(ctx, testDir+"/test-file.txt")
		if err != nil {
			t.Fatalf("Failed to download file - file was not created via ExecuteCommand: %v", err)
		}
		if !strings.Contains(string(content), "created via execute command") {
			t.Errorf("Expected file content to contain 'created via execute command', got: %s", string(content))
		}
		t.Logf("File content verified: %s", strings.TrimSpace(string(content)))

		// Cleanup
		err = sandbox.FS.DeleteFile(ctx, testDir, true)
		if err != nil {
			t.Logf("Failed to cleanup test directory: %v", err)
		}
	})
}

func TestProcessCodeRun(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for code execution tests
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

	t.Run("run Python code", func(t *testing.T) {
		code := `
x = 10
y = 20
print(f"Sum: {x + y}")
`
		response, err := sandbox.Process.CodeRun(ctx, code, nil, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		if !strings.Contains(response.Result, "Sum: 30") {
			t.Errorf("Expected output to contain 'Sum: 30', got: %s", response.Result)
		}
	})

	t.Run("run Python code with imports", func(t *testing.T) {
		code := `
import sys
print(f"Python version: {sys.version_info.major}.{sys.version_info.minor}")
`
		response, err := sandbox.Process.CodeRun(ctx, code, nil, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		if !strings.Contains(response.Result, "Python version:") {
			t.Errorf("Expected output to contain Python version, got: %s", response.Result)
		}
	})

	t.Run("run Python code with args", func(t *testing.T) {
		code := `
import sys
print(f"Args: {sys.argv[1:]}")
`
		response, err := sandbox.Process.CodeRun(ctx, code, &daytona.CodeRunParams{
			Argv: []string{"arg1", "arg2"},
		}, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		if !strings.Contains(response.Result, "arg1") || !strings.Contains(response.Result, "arg2") {
			t.Errorf("Expected output to contain args, got: %s", response.Result)
		}
	})

	t.Run("run Python code with env vars", func(t *testing.T) {
		code := `
import os
print(f"MY_VAR={os.environ.get('MY_VAR', 'not set')}")
`
		response, err := sandbox.Process.CodeRun(ctx, code, &daytona.CodeRunParams{
			Env: map[string]string{
				"MY_VAR": "env_value",
			},
		}, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", response.ExitCode)
		}
		if !strings.Contains(response.Result, "MY_VAR=env_value") {
			t.Errorf("Expected output to contain env var, got: %s", response.Result)
		}
	})

	t.Run("run failing Python code", func(t *testing.T) {
		code := `
raise Exception("Test error")
`
		response, err := sandbox.Process.CodeRun(ctx, code, nil, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if response.ExitCode == 0 {
			t.Error("Expected non-zero exit code for failing code")
		}
	})
}

func TestProcessSessions(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for session tests
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

	sessionID := "test-session-" + time.Now().Format("20060102150405")

	t.Run("create session", func(t *testing.T) {
		err := sandbox.Process.CreateSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
	})

	t.Run("get session", func(t *testing.T) {
		session, err := sandbox.Process.GetSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		if session.SessionId != sessionID {
			t.Errorf("Expected session ID %s, got %s", sessionID, session.SessionId)
		}
	})

	t.Run("execute session command sync", func(t *testing.T) {
		response, err := sandbox.Process.ExecuteSessionCommand(ctx, sessionID, `echo "Hello from session"`, false, nil)
		if err != nil {
			t.Fatalf("Failed to execute session command: %v", err)
		}
		if response.ExitCode == nil || *response.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %v", response.ExitCode)
		}
		// Check both Stdout and Output as demuxing may vary
		if !strings.Contains(response.Stdout, "Hello from session") && !strings.Contains(response.Output, "Hello from session") {
			t.Errorf("Expected output to contain 'Hello from session', got stdout: %s, output: %s", response.Stdout, response.Output)
		}
	})

	t.Run("execute session command async", func(t *testing.T) {
		response, err := sandbox.Process.ExecuteSessionCommand(ctx, sessionID, `sleep 1 && echo "Async done"`, true, nil)
		if err != nil {
			t.Fatalf("Failed to execute session command: %v", err)
		}
		if response.CmdID == "" {
			t.Error("Expected command ID for async execution")
		}

		// Wait for command to complete
		time.Sleep(2 * time.Second)

		// Get command info
		cmd, err := sandbox.Process.GetSessionCommand(ctx, sessionID, response.CmdID)
		if err != nil {
			t.Fatalf("Failed to get session command: %v", err)
		}
		if cmd.Id != response.CmdID {
			t.Errorf("Expected command ID %s, got %s", response.CmdID, cmd.Id)
		}
	})

	t.Run("get session command logs", func(t *testing.T) {
		// First execute a command
		response, err := sandbox.Process.ExecuteSessionCommand(ctx, sessionID, `echo "Log test"`, false, nil)
		if err != nil {
			t.Fatalf("Failed to execute session command: %v", err)
		}

		logs, err := sandbox.Process.GetSessionCommandLogs(ctx, sessionID, response.CmdID)
		if err != nil {
			t.Fatalf("Failed to get session command logs: %v", err)
		}
		// Check both Stdout and Output as demuxing may vary
		if !strings.Contains(logs.Stdout, "Log test") && !strings.Contains(logs.Output, "Log test") {
			t.Errorf("Expected logs to contain 'Log test', got stdout: %s, output: %s", logs.Stdout, logs.Output)
		}
	})

	t.Run("list sessions", func(t *testing.T) {
		sessions, err := sandbox.Process.ListSessions(ctx)
		if err != nil {
			t.Fatalf("Failed to list sessions: %v", err)
		}
		found := false
		for _, s := range sessions {
			if s.SessionId == sessionID {
				found = true
				break
			}
		}
		if !found {
			t.Error("Created session not found in list")
		}
	})

	t.Run("delete session", func(t *testing.T) {
		err := sandbox.Process.DeleteSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to delete session: %v", err)
		}
	})
}

func TestProcessPty(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for PTY tests
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

	t.Run("create PTY session", func(t *testing.T) {
		cols := float32(80)
		rows := float32(24)

		response, err := sandbox.Process.CreatePty(ctx, daytona.PtyCreateOptions{
			ID:   "test-pty",
			Cols: &cols,
			Rows: &rows,
		})
		if err != nil {
			t.Fatalf("Failed to create PTY: %v", err)
		}
		if response.SessionId == "" {
			t.Error("Expected non-empty session ID")
		}
	})

	t.Run("list PTY sessions", func(t *testing.T) {
		sessions, err := sandbox.Process.ListPtySessions(ctx)
		if err != nil {
			t.Fatalf("Failed to list PTY sessions: %v", err)
		}
		if len(sessions) == 0 {
			t.Error("Expected at least one PTY session")
		}
	})

	t.Run("get PTY session info", func(t *testing.T) {
		info, err := sandbox.Process.GetPtySessionInfo(ctx, "test-pty")
		if err != nil {
			t.Fatalf("Failed to get PTY session info: %v", err)
		}
		if info.Id != "test-pty" {
			t.Errorf("Expected PTY ID 'test-pty', got %s", info.Id)
		}
	})

	t.Run("resize PTY session", func(t *testing.T) {
		// Create a PTY with handle to activate it (lazy start PTYs can't be resized until connected)
		cols := float32(80)
		rows := float32(24)

		handle, err := sandbox.Process.CreatePtyWithHandle(ctx,
			daytona.PtyCreateOptions{
				ID:   "test-pty-resize",
				Cols: &cols,
				Rows: &rows,
			},
			daytona.PtyConnectOptions{
				OnData: func(data []byte) {
					// Ignore output for this test
				},
			},
		)
		if err != nil {
			t.Fatalf("Failed to create PTY with handle for resize test: %v", err)
		}
		defer handle.Disconnect()

		// Now resize the active PTY session
		info, err := sandbox.Process.ResizePtySession(ctx, "test-pty-resize", 120, 40)
		if err != nil {
			t.Fatalf("Failed to resize PTY session: %v", err)
		}
		if info.Cols != 120 {
			t.Errorf("Expected cols 120, got %v", info.Cols)
		}
		if info.Rows != 40 {
			t.Errorf("Expected rows 40, got %v", info.Rows)
		}
	})

	t.Run("kill PTY session", func(t *testing.T) {
		err := sandbox.Process.KillPtySession(ctx, "test-pty")
		if err != nil {
			t.Fatalf("Failed to kill PTY session: %v", err)
		}
	})
}

func TestProcessPtyWithHandle(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for PTY handle tests
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

	t.Run("create PTY with handle and interact", func(t *testing.T) {
		cols := float32(80)
		rows := float32(24)

		var output strings.Builder
		handle, err := sandbox.Process.CreatePtyWithHandle(ctx,
			daytona.PtyCreateOptions{
				ID:   "test-pty-handle",
				Cols: &cols,
				Rows: &rows,
			},
			daytona.PtyConnectOptions{
				OnData: func(data []byte) {
					output.Write(data)
				},
			},
		)
		if err != nil {
			t.Fatalf("Failed to create PTY with handle: %v", err)
		}
		defer handle.Disconnect()

		if !handle.IsConnected() {
			t.Error("Expected PTY handle to be connected")
		}

		// Send a command
		err = handle.SendInput("echo 'Hello from PTY'\n")
		if err != nil {
			t.Fatalf("Failed to send input: %v", err)
		}

		// Wait for output
		time.Sleep(1 * time.Second)

		// Exit the shell
		err = handle.SendInput("exit\n")
		if err != nil {
			t.Fatalf("Failed to send exit: %v", err)
		}

		// Wait for exit
		result, err := handle.Wait()
		if err != nil {
			t.Fatalf("Failed to wait for PTY: %v", err)
		}

		t.Logf("PTY output: %s", output.String())
		t.Logf("PTY exit code: %v", result.ExitCode)

		if !strings.Contains(output.String(), "Hello from PTY") {
			t.Error("Expected output to contain 'Hello from PTY'")
		}
	})

	t.Run("create directory via PTY and verify with filesystem", func(t *testing.T) {
		cols := float32(80)
		rows := float32(24)
		testDir := "/tmp/pty-created-dir-" + time.Now().Format("20060102150405")

		var output strings.Builder
		handle, err := sandbox.Process.CreatePtyWithHandle(ctx,
			daytona.PtyCreateOptions{
				ID:   "test-pty-mkdir",
				Cols: &cols,
				Rows: &rows,
			},
			daytona.PtyConnectOptions{
				OnData: func(data []byte) {
					output.Write(data)
				},
			},
		)
		if err != nil {
			t.Fatalf("Failed to create PTY with handle: %v", err)
		}
		defer handle.Disconnect()

		// Create a directory using mkdir command
		err = handle.SendInput("mkdir -p " + testDir + "\n")
		if err != nil {
			t.Fatalf("Failed to send mkdir command: %v", err)
		}

		// Create a file inside the directory
		err = handle.SendInput("echo 'created via pty' > " + testDir + "/test-file.txt\n")
		if err != nil {
			t.Fatalf("Failed to send echo command: %v", err)
		}

		// Wait for commands to execute
		time.Sleep(2 * time.Second)

		// Exit the shell
		err = handle.SendInput("exit\n")
		if err != nil {
			t.Fatalf("Failed to send exit: %v", err)
		}

		// Wait for PTY to exit
		_, err = handle.Wait()
		if err != nil {
			t.Fatalf("Failed to wait for PTY: %v", err)
		}

		t.Logf("PTY output: %s", output.String())

		// Verify directory exists using filesystem API
		dirInfo, err := sandbox.FS.GetFileDetails(ctx, testDir)
		if err != nil {
			t.Fatalf("Failed to get directory details - directory was not created via PTY: %v", err)
		}
		if !dirInfo.IsDir {
			t.Error("Expected path to be a directory")
		}
		t.Logf("Directory created via PTY verified: %s", testDir)

		// Verify file exists and has correct content
		content, err := sandbox.FS.DownloadFile(ctx, testDir+"/test-file.txt")
		if err != nil {
			t.Fatalf("Failed to download file - file was not created via PTY: %v", err)
		}
		if !strings.Contains(string(content), "created via pty") {
			t.Errorf("Expected file content to contain 'created via pty', got: %s", string(content))
		}
		t.Logf("File content verified: %s", strings.TrimSpace(string(content)))

		// Cleanup
		err = sandbox.FS.DeleteFile(ctx, testDir, true)
		if err != nil {
			t.Logf("Failed to cleanup test directory: %v", err)
		}
	})
}
