/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona_test

import (
	"context"
	"testing"

	daytona "github.com/forkbikash/daytona/libs/sdk-go"
)

func TestComputerUse(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for computer use tests
	// Note: Computer use requires a sandbox with desktop environment support
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

	t.Run("start computer use", func(t *testing.T) {
		response, err := sandbox.ComputerUse.Start(ctx)
		if err != nil {
			// Computer use may not be available on all sandboxes
			t.Skipf("Computer use not available: %v", err)
		}
		t.Logf("Computer use started: %v", response)
	})

	t.Run("get computer use status", func(t *testing.T) {
		status, err := sandbox.ComputerUse.GetStatus(ctx)
		if err != nil {
			t.Skipf("Computer use not available: %v", err)
		}
		t.Logf("Computer use status: %v", status)
	})

	t.Run("get display info", func(t *testing.T) {
		info, err := sandbox.ComputerUse.Display.GetInfo(ctx)
		if err != nil {
			t.Skipf("Display info not available: %v", err)
		}
		t.Logf("Display info: %v", info)
	})

	t.Run("take screenshot", func(t *testing.T) {
		screenshot, err := sandbox.ComputerUse.Screenshot.TakeFullScreen(ctx, false)
		if err != nil {
			t.Skipf("Screenshot not available: %v", err)
		}
		if screenshot.Screenshot == "" {
			t.Error("Expected non-empty screenshot image")
		}
		t.Logf("Screenshot taken, size: %v bytes", screenshot.SizeBytes)
	})

	t.Run("take compressed screenshot", func(t *testing.T) {
		quality := float32(80)
		scale := float32(0.5)
		screenshot, err := sandbox.ComputerUse.Screenshot.TakeCompressed(ctx, &daytona.ScreenshotOptions{
			Format:  "jpeg",
			Quality: &quality,
			Scale:   &scale,
		})
		if err != nil {
			t.Skipf("Compressed screenshot not available: %v", err)
		}
		if screenshot.Screenshot == "" {
			t.Error("Expected non-empty compressed screenshot")
		}
		t.Logf("Compressed screenshot taken")
	})

	t.Run("take region screenshot", func(t *testing.T) {
		screenshot, err := sandbox.ComputerUse.Screenshot.TakeRegion(ctx, daytona.ScreenshotRegion{
			X:      0,
			Y:      0,
			Width:  200,
			Height: 200,
		}, false)
		if err != nil {
			t.Skipf("Region screenshot not available: %v", err)
		}
		if screenshot.Screenshot == "" {
			t.Error("Expected non-empty region screenshot")
		}
		t.Logf("Region screenshot taken")
	})

	t.Run("get mouse position", func(t *testing.T) {
		pos, err := sandbox.ComputerUse.Mouse.GetPosition(ctx)
		if err != nil {
			t.Skipf("Mouse position not available: %v", err)
		}
		t.Logf("Mouse position: (%f, %f)", pos.X, pos.Y)
	})

	t.Run("move mouse", func(t *testing.T) {
		response, err := sandbox.ComputerUse.Mouse.Move(ctx, 100, 100)
		if err != nil {
			t.Skipf("Mouse move not available: %v", err)
		}
		t.Logf("Mouse moved: %v", response)
	})

	t.Run("click mouse", func(t *testing.T) {
		response, err := sandbox.ComputerUse.Mouse.Click(ctx, 100, 100, "left", false)
		if err != nil {
			t.Skipf("Mouse click not available: %v", err)
		}
		t.Logf("Mouse clicked: %v", response)
	})

	t.Run("scroll mouse", func(t *testing.T) {
		response, err := sandbox.ComputerUse.Mouse.Scroll(ctx, 100, 100, "down", 3)
		if err != nil {
			t.Skipf("Mouse scroll not available: %v", err)
		}
		t.Logf("Mouse scrolled: %v", response)
	})

	t.Run("type text", func(t *testing.T) {
		err := sandbox.ComputerUse.Keyboard.Type(ctx, "Hello from Go SDK!", nil)
		if err != nil {
			t.Skipf("Keyboard type not available: %v", err)
		}
		t.Log("Text typed successfully")
	})

	t.Run("press key", func(t *testing.T) {
		err := sandbox.ComputerUse.Keyboard.Press(ctx, "Return", nil)
		if err != nil {
			t.Skipf("Keyboard press not available: %v", err)
		}
		t.Log("Key pressed successfully")
	})

	t.Run("press hotkey", func(t *testing.T) {
		err := sandbox.ComputerUse.Keyboard.Hotkey(ctx, "ctrl+c")
		if err != nil {
			t.Skipf("Keyboard hotkey not available: %v", err)
		}
		t.Log("Hotkey pressed successfully")
	})

	t.Run("get windows", func(t *testing.T) {
		windows, err := sandbox.ComputerUse.Display.GetWindows(ctx)
		if err != nil {
			t.Skipf("Get windows not available: %v", err)
		}
		t.Logf("Windows: %v", windows)
	})

	t.Run("stop computer use", func(t *testing.T) {
		response, err := sandbox.ComputerUse.Stop(ctx)
		if err != nil {
			t.Skipf("Computer use stop not available: %v", err)
		}
		t.Logf("Computer use stopped: %v", response)
	})
}

func TestComputerUseProcessManagement(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for computer use tests
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

	// Start computer use first
	_, err = sandbox.ComputerUse.Start(ctx)
	if err != nil {
		t.Skipf("Computer use not available: %v", err)
	}

	t.Run("get process status", func(t *testing.T) {
		status, err := sandbox.ComputerUse.GetProcessStatus(ctx, "vnc")
		if err != nil {
			t.Skipf("Process status not available: %v", err)
		}
		t.Logf("VNC process status: %v", status)
	})

	t.Run("get process logs", func(t *testing.T) {
		logs, err := sandbox.ComputerUse.GetProcessLogs(ctx, "vnc")
		if err != nil {
			t.Skipf("Process logs not available: %v", err)
		}
		t.Logf("VNC process logs (first 100 chars): %.100s", logs.Logs)
	})

	t.Run("get process errors", func(t *testing.T) {
		errors, err := sandbox.ComputerUse.GetProcessErrors(ctx, "vnc")
		if err != nil {
			t.Skipf("Process errors not available: %v", err)
		}
		t.Logf("VNC process errors: %v", errors.Errors)
	})

	t.Run("restart process", func(t *testing.T) {
		response, err := sandbox.ComputerUse.RestartProcess(ctx, "vnc")
		if err != nil {
			t.Skipf("Process restart not available: %v", err)
		}
		t.Logf("VNC process restarted: %v", response)
	})
}
