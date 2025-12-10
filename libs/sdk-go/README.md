# Daytona SDK for Go

A Go SDK for interacting with the Daytona API, providing a simple interface for Daytona Sandbox management, Git operations, file system operations, and language server protocol support.

## Installation

```bash
go get github.com/daytonaio/sdk-go
```

**Note:** This SDK requires the Daytona API client. If you're using this from outside the Daytona monorepo, ensure you have access to `github.com/daytonaio/apiclient`.

## Requirements

- Go 1.21 or later
- Daytona API key (get one from [Daytona](https://www.daytona.io))

## Quick Start

Here's a simple example of using the SDK:

```go
package main

import (
	"context"
	"fmt"
	"log"

	daytona "github.com/daytonaio/sdk-go"
)

func main() {
	ctx := context.Background()

	// Initialize using environment variables
	client, err := daytona.NewDaytona(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create a sandbox
	sandbox, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Run code in the sandbox
	response, err := sandbox.Process.CodeRun(ctx, `print("Hello World!")`, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.Result)

	// Clean up when done
	err = client.Delete(ctx, sandbox, 60)
	if err != nil {
		log.Fatal(err)
	}
}
```

## Configuration

The SDK can be configured using environment variables or by passing a configuration object:

```go
import daytona "github.com/daytonaio/sdk-go"

// Initialize with configuration
client, err := daytona.NewDaytona(&daytona.Config{
	APIKey: "your-api-key",
	APIURL: "your-api-url",
	Target: "us",
})
```

Or using environment variables:

- `DAYTONA_API_KEY`: Your Daytona API key
- `DAYTONA_API_URL`: The Daytona API URL
- `DAYTONA_TARGET`: Your target environment

You can also customize sandbox creation:

```go
autoStopInterval := int32(60)
autoArchiveInterval := int32(60)
autoDeleteInterval := int32(120)

sandbox, err := client.Create(ctx, &daytona.CreateSandboxFromSnapshotParams{
	CreateSandboxBaseParams: daytona.CreateSandboxBaseParams{
		Language:            daytona.CodeLanguagePython,
		EnvVars:             map[string]string{"NODE_ENV": "development"},
		AutoStopInterval:    &autoStopInterval,
		AutoArchiveInterval: &autoArchiveInterval,
		AutoDeleteInterval:  &autoDeleteInterval,
	},
}, nil)
```

## Features

- **Sandbox Management**: Create, manage and remove sandboxes
- **Git Operations**: Clone repositories, manage branches, and more
- **File System Operations**: Upload, download, search and manipulate files
- **Language Server Protocol**: Interact with language servers for code intelligence
- **Process Management**: Execute code and commands in sandboxes
- **Code Interpreter**: Execute Python code with WebSocket streaming
- **Computer Use**: Desktop automation with mouse, keyboard, and screenshot operations

## Examples

### Execute Commands

```go
// Execute a shell command
response, err := sandbox.Process.ExecuteCommand(ctx, `echo "Hello, World!"`, "", nil, nil)
if err != nil {
	log.Fatal(err)
}
fmt.Println(response.Result)

// Run Python code
response, err = sandbox.Process.CodeRun(ctx, `
x = 10
y = 20
print(f"Sum: {x + y}")
`, nil, nil)
if err != nil {
	log.Fatal(err)
}
fmt.Println(response.Result)
```

### File Operations

```go
// Upload a file
err := sandbox.FS.UploadFile(ctx, []byte("Hello, World!"), "path/to/file.txt")

// Download a file
content, err := sandbox.FS.DownloadFile(ctx, "path/to/file.txt")

// Search for files
matches, err := sandbox.FS.FindFiles(ctx, "root_dir", "search_pattern")
```

### Git Operations

```go
// Clone a repository
err := sandbox.Git.Clone(ctx, "https://github.com/example/repo", "path/to/clone", nil)

// List branches
branches, err := sandbox.Git.Branches(ctx, "path/to/repo")

// Add files
err = sandbox.Git.Add(ctx, "path/to/repo", []string{"file1.txt", "file2.txt"})
```

### Language Server Protocol

```go
// Create and start a language server
lsp := sandbox.CreateLspServer(daytona.LspLanguageTypeScript, "path/to/project")
err := lsp.Start(ctx)

// Notify the LSP for the file
err = lsp.DidOpen(ctx, "path/to/file.ts")

// Get document symbols
symbols, err := lsp.DocumentSymbols(ctx, "path/to/file.ts")

// Get completions
completions, err := lsp.Completions(ctx, "path/to/file.ts", daytona.Position{
	Line:      10,
	Character: 15,
})
```

### Computer Use (Desktop Automation)

```go
// Start computer use
_, err := sandbox.ComputerUse.Start(ctx)

// Take a screenshot
screenshot, err := sandbox.ComputerUse.Screenshot.TakeFullScreen(ctx, false)

// Click the mouse
_, err = sandbox.ComputerUse.Mouse.Click(ctx, 100, 200, "left", false)

// Type text
err = sandbox.ComputerUse.Keyboard.Type(ctx, "Hello World!", nil)
```

### Code Interpreter

```go
result, err := sandbox.CodeInterpreter.RunCode(ctx, `
import sys
print("Python version:", sys.version)
`, &daytona.RunCodeOptions{
	OnStdout: func(msg daytona.OutputMessage) {
		fmt.Print(msg.Output)
	},
})
```

### PTY Sessions (Interactive Terminal)

```go
// Create a PTY session with a handle for interactive terminal access
cols := float32(120)
rows := float32(30)

handle, err := sandbox.Process.CreatePtyWithHandle(ctx,
	daytona.PtyCreateOptions{
		ID:   "my-interactive-session",
		Cols: &cols,
		Rows: &rows,
	},
	daytona.PtyConnectOptions{
		OnData: func(data []byte) {
			// Handle terminal output
			fmt.Print(string(data))
		},
	},
)
if err != nil {
	log.Fatal(err)
}
defer handle.Disconnect()

// Send commands to the terminal
handle.SendInput("ls -la\n")
handle.SendInput("echo 'Hello from PTY!'\n")
handle.SendInput("exit\n")

// Wait for the session to complete
result, err := handle.Wait()
if err != nil {
	log.Fatal(err)
}
fmt.Printf("PTY session exited with code: %d\n", *result.ExitCode)
```

### Snapshots

```go
// Create a snapshot from an image
snapshot, err := client.Snapshot.Create(ctx, &daytona.CreateSnapshotParams{
	Name:  "my-snapshot",
	Image: "python:3.12-slim",
}, nil)

// List snapshots
snapshots, err := client.Snapshot.List(ctx, 1, 10)
```

### Volumes

```go
// Create a volume
volume, err := client.Volume.Create(ctx, "my-volume")

// List volumes
volumes, err := client.Volume.List(ctx)

// Delete a volume
err = client.Volume.Delete(ctx, volume)
```

## Contributing

Daytona is Open Source under the [Apache License 2.0](/libs/sdk-go/LICENSE), and is the [copyright of its contributors](/NOTICE). If you would like to contribute to the software, read the Developer Certificate of Origin Version 1.1 (https://developercertificate.org/). Afterwards, navigate to the [contributing guide](/CONTRIBUTING.md) to get started.
