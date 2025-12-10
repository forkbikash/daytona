/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	sdkerrors "github.com/daytonaio/sdk-go/errors"
	"github.com/gorilla/websocket"
)

// ExecutionError represents an error during code execution.
type ExecutionError struct {
	// Name is the error type name.
	Name string
	// Value is the error message.
	Value string
	// Traceback is the stack trace.
	Traceback string
}

// ExecutionResult contains the result of code execution.
type ExecutionResult struct {
	// Stdout is the standard output.
	Stdout string
	// Stderr is the standard error.
	Stderr string
	// Error contains error information if execution failed.
	Error *ExecutionError
}

// OutputMessage represents an output message from code execution.
type OutputMessage struct {
	Output string
}

// RunCodeOptions contains options for running code.
type RunCodeOptions struct {
	// Context is the interpreter context to use.
	Context *InterpreterContext
	// Envs are environment variables for the execution.
	Envs map[string]string
	// Timeout is the execution timeout in seconds.
	Timeout *int32
	// OnStdout is called for stdout output.
	OnStdout func(msg OutputMessage)
	// OnStderr is called for stderr output.
	OnStderr func(msg OutputMessage)
	// OnError is called when an error occurs.
	OnError func(err ExecutionError)
}

// InterpreterContext represents an interpreter context.
type InterpreterContext struct {
	// ID is the context identifier.
	ID string
	// Language is the interpreter language.
	Language string
	// Cwd is the working directory.
	Cwd string
}

// CodeInterpreter handles Python code interpretation and execution within a Sandbox.
type CodeInterpreter struct {
	sandbox *Sandbox
}

// NewCodeInterpreter creates a new CodeInterpreter instance.
func NewCodeInterpreter(sandbox *Sandbox) *CodeInterpreter {
	return &CodeInterpreter{sandbox: sandbox}
}

// RunCode runs Python code in the sandbox.
func (ci *CodeInterpreter) RunCode(ctx context.Context, code string, options *RunCodeOptions) (*ExecutionResult, error) {
	if strings.TrimSpace(code) == "" {
		return nil, sdkerrors.NewDaytonaError("Code is required for execution", 0, nil)
	}

	if options == nil {
		options = &RunCodeOptions{}
	}

	// Get the toolbox base URL
	toolboxURL, err := ci.sandbox.getToolboxBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	// Convert HTTP URL to WebSocket URL
	wsURL := strings.Replace(toolboxURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = wsURL + "/process/interpreter/execute"

	// Get preview token for authentication
	token, err := ci.sandbox.getPreviewToken(ctx)
	if err != nil {
		return nil, err
	}

	// Create WebSocket connection
	header := map[string][]string{
		"Authorization": {"Bearer " + token},
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return nil, sdkerrors.NewDaytonaError("Failed to connect to interpreter: "+err.Error(), 0, nil)
	}
	defer conn.Close()

	// Build and send execution request
	payload := map[string]interface{}{
		"code": code,
	}
	if options.Context != nil && options.Context.ID != "" {
		payload["contextId"] = options.Context.ID
	}
	if options.Envs != nil {
		payload["envs"] = options.Envs
	}
	if options.Timeout != nil {
		payload["timeout"] = *options.Timeout
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if err := conn.WriteMessage(websocket.TextMessage, payloadBytes); err != nil {
		return nil, sdkerrors.NewDaytonaError("Failed to send code: "+err.Error(), 0, nil)
	}

	result := &ExecutionResult{}
	var mu sync.Mutex

	// Read messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				break
			}
			return nil, sdkerrors.NewDaytonaError("WebSocket error: "+err.Error(), 0, nil)
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal(message, &chunk); err != nil {
			continue // Skip invalid JSON
		}

		chunkType, _ := chunk["type"].(string)

		mu.Lock()
		switch chunkType {
		case "stdout":
			text, _ := chunk["text"].(string)
			result.Stdout += text
			if options.OnStdout != nil {
				options.OnStdout(OutputMessage{Output: text})
			}
		case "stderr":
			text, _ := chunk["text"].(string)
			result.Stderr += text
			if options.OnStderr != nil {
				options.OnStderr(OutputMessage{Output: text})
			}
		case "error":
			execError := ExecutionError{
				Name:      getString(chunk, "name"),
				Value:     getString(chunk, "value"),
				Traceback: getString(chunk, "traceback"),
			}
			result.Error = &execError
			if options.OnError != nil {
				options.OnError(execError)
			}
		case "control":
			text, _ := chunk["text"].(string)
			if text == "completed" || text == "interrupted" {
				mu.Unlock()
				return result, nil
			}
		}
		mu.Unlock()
	}

	return result, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
