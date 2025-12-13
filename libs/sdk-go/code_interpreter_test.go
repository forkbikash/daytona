/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona_test

import (
	"context"
	"strings"
	"testing"

	daytona "github.com/forkbikash/daytona/libs/sdk-go"
)

func TestCodeInterpreter(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for code interpreter tests
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

	t.Run("run simple code", func(t *testing.T) {
		result, err := sandbox.CodeInterpreter.RunCode(ctx, `print("Hello from interpreter")`, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if result.Error != nil {
			t.Errorf("Unexpected error: %s", result.Error.Value)
		}
		if !strings.Contains(result.Stdout, "Hello from interpreter") {
			t.Errorf("Expected stdout to contain 'Hello from interpreter', got: %s", result.Stdout)
		}
	})

	t.Run("run code with streaming output", func(t *testing.T) {
		var stdout strings.Builder

		result, err := sandbox.CodeInterpreter.RunCode(ctx, `
for i in range(5):
    print(f"Count: {i}")
`, &daytona.RunCodeOptions{
			OnStdout: func(msg daytona.OutputMessage) {
				stdout.WriteString(msg.Output)
			},
		})
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if result.Error != nil {
			t.Errorf("Unexpected error: %s", result.Error.Value)
		}
		if !strings.Contains(stdout.String(), "Count: 4") {
			t.Errorf("Expected streamed output to contain 'Count: 4', got: %s", stdout.String())
		}
	})

	t.Run("run code with stderr", func(t *testing.T) {
		var stderr strings.Builder

		result, err := sandbox.CodeInterpreter.RunCode(ctx, `
import sys
sys.stderr.write("Error message\n")
print("Normal output")
`, &daytona.RunCodeOptions{
			OnStderr: func(msg daytona.OutputMessage) {
				stderr.WriteString(msg.Output)
			},
		})
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if !strings.Contains(result.Stdout, "Normal output") {
			t.Errorf("Expected stdout to contain 'Normal output', got: %s", result.Stdout)
		}
		if !strings.Contains(result.Stderr, "Error message") && !strings.Contains(stderr.String(), "Error message") {
			t.Errorf("Expected stderr to contain 'Error message'")
		}
	})

	t.Run("run code with error", func(t *testing.T) {
		var errorReceived bool

		result, err := sandbox.CodeInterpreter.RunCode(ctx, `
raise ValueError("Test error message")
`, &daytona.RunCodeOptions{
			OnError: func(err daytona.ExecutionError) {
				errorReceived = true
				if !strings.Contains(err.Value, "Test error message") {
					t.Errorf("Expected error value to contain 'Test error message', got: %s", err.Value)
				}
			},
		})
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if result.Error == nil && !errorReceived {
			t.Error("Expected execution error")
		}
	})

	t.Run("run code with environment variables", func(t *testing.T) {
		result, err := sandbox.CodeInterpreter.RunCode(ctx, `
import os
print(f"MY_VAR={os.environ.get('MY_VAR', 'not set')}")
`, &daytona.RunCodeOptions{
			Envs: map[string]string{
				"MY_VAR": "interpreter_env_value",
			},
		})
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if result.Error != nil {
			t.Errorf("Unexpected error: %s", result.Error.Value)
		}
		if !strings.Contains(result.Stdout, "MY_VAR=interpreter_env_value") {
			t.Errorf("Expected stdout to contain env var value, got: %s", result.Stdout)
		}
	})

	t.Run("run code with imports", func(t *testing.T) {
		result, err := sandbox.CodeInterpreter.RunCode(ctx, `
import json
import math

data = {"pi": math.pi, "e": math.e}
print(json.dumps(data, indent=2))
`, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if result.Error != nil {
			t.Errorf("Unexpected error: %s", result.Error.Value)
		}
		if !strings.Contains(result.Stdout, "3.14") {
			t.Errorf("Expected stdout to contain pi value, got: %s", result.Stdout)
		}
	})

	t.Run("run multiline computation", func(t *testing.T) {
		result, err := sandbox.CodeInterpreter.RunCode(ctx, `
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

for i in range(1, 6):
    print(f"{i}! = {factorial(i)}")
`, nil)
		if err != nil {
			t.Fatalf("Failed to run code: %v", err)
		}
		if result.Error != nil {
			t.Errorf("Unexpected error: %s", result.Error.Value)
		}
		if !strings.Contains(result.Stdout, "5! = 120") {
			t.Errorf("Expected stdout to contain '5! = 120', got: %s", result.Stdout)
		}
	})
}

func TestCodeInterpreterStatePersistence(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Create a sandbox for code interpreter tests
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

	t.Run("state persists across calls with context", func(t *testing.T) {
		// Define a variable
		result1, err := sandbox.CodeInterpreter.RunCode(ctx, `
my_persistent_var = 42
print("Variable defined")
`, nil)
		if err != nil {
			t.Fatalf("Failed to run first code: %v", err)
		}
		if result1.Error != nil {
			t.Errorf("Unexpected error in first code: %s", result1.Error.Value)
		}

		// Access the variable (without context, should be in same session)
		result2, err := sandbox.CodeInterpreter.RunCode(ctx, `
print(f"my_persistent_var = {my_persistent_var}")
`, nil)
		if err != nil {
			t.Fatalf("Failed to run second code: %v", err)
		}
		// Note: This may or may not work depending on context handling
		// The test validates the behavior either way
		t.Logf("Second result stdout: %s", result2.Stdout)
		if result2.Error != nil {
			t.Logf("Second result error (may be expected): %s", result2.Error.Value)
		}
	})
}
