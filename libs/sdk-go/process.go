/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	apiclient "github.com/forkbikash/daytona/libs/api-client-go"
	"github.com/forkbikash/daytona/libs/sdk-go/codetoolbox"
)

var (
	// STDOUT_PREFIX_BYTES are the multiplexing markers for stdout.
	STDOUT_PREFIX_BYTES = []byte{0x01, 0x01, 0x01}
	// STDERR_PREFIX_BYTES are the multiplexing markers for stderr.
	STDERR_PREFIX_BYTES = []byte{0x02, 0x02, 0x02}
)

// CodeRunParams contains parameters for code execution.
type CodeRunParams struct {
	// Argv contains command line arguments.
	Argv []string
	// Env contains environment variables.
	Env map[string]string
}

// ExecuteResponse contains the results of command execution.
type ExecuteResponse struct {
	// ExitCode is the command's exit status.
	ExitCode int32
	// Result is the standard output from the command.
	Result string
}

// SessionExecuteResponse contains the results of session command execution.
type SessionExecuteResponse struct {
	// CmdID is the unique identifier for the executed command.
	CmdID string
	// Output is the combined command output.
	Output string
	// Stdout is the standard output from the command.
	Stdout string
	// Stderr is the standard error from the command.
	Stderr string
	// ExitCode is the command exit status.
	ExitCode *int32
}

// SessionCommandLogsResponse contains the logs for a session command.
type SessionCommandLogsResponse struct {
	// Output is the combined output.
	Output string
	// Stdout is the standard output.
	Stdout string
	// Stderr is the standard error.
	Stderr string
}

// Process handles process and code execution within a Sandbox.
type Process struct {
	sandbox     *Sandbox
	codeToolbox codetoolbox.SandboxCodeToolbox
}

// NewProcess creates a new Process instance.
func NewProcess(sandbox *Sandbox, codeToolbox codetoolbox.SandboxCodeToolbox) *Process {
	return &Process{
		sandbox:     sandbox,
		codeToolbox: codeToolbox,
	}
}

// ExecuteCommand executes a shell command in the Sandbox.
func (p *Process) ExecuteCommand(ctx context.Context, command string, cwd string, env map[string]string, timeout *float32) (*ExecuteResponse, error) {
	// Base64 encode the command for safe execution
	base64Cmd := base64.StdEncoding.EncodeToString([]byte(command))
	safeCommand := fmt.Sprintf("echo '%s' | base64 -d | sh", base64Cmd)

	// Add environment variable exports if provided
	if len(env) > 0 {
		var exports []string
		for key, value := range env {
			encodedValue := base64.StdEncoding.EncodeToString([]byte(value))
			exports = append(exports, fmt.Sprintf("export %s=$(echo '%s' | base64 -d)", key, encodedValue))
		}
		safeCommand = strings.Join(exports, ";") + "; " + safeCommand
	}

	safeCommand = fmt.Sprintf(`sh -c "%s"`, safeCommand)

	req := apiclient.ExecuteRequest{
		Command: safeCommand,
	}
	if cwd != "" {
		req.Cwd = &cwd
	}
	if timeout != nil {
		req.Timeout = timeout
	}

	response, httpResp, err := p.sandbox.toolboxAPI.ExecuteCommandDeprecated(ctx, p.sandbox.ID).ExecuteRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	return &ExecuteResponse{
		ExitCode: int32(response.ExitCode),
		Result:   response.Result,
	}, nil
}

// CodeRun executes code in the Sandbox using the appropriate language runtime.
func (p *Process) CodeRun(ctx context.Context, code string, params *CodeRunParams, timeout *float32) (*ExecuteResponse, error) {
	var toolboxParams *codetoolbox.CodeRunParams
	if params != nil {
		toolboxParams = &codetoolbox.CodeRunParams{
			Argv: params.Argv,
		}
	}

	runCommand := p.codeToolbox.GetRunCommand(code, toolboxParams)

	var env map[string]string
	if params != nil {
		env = params.Env
	}

	return p.ExecuteCommand(ctx, runCommand, "", env, timeout)
}

// CreateSession creates a new long-running background session in the Sandbox.
func (p *Process) CreateSession(ctx context.Context, sessionID string) error {
	req := apiclient.CreateSessionRequest{
		SessionId: sessionID,
	}
	httpResp, err := p.sandbox.toolboxAPI.CreateSessionDeprecated(ctx, p.sandbox.ID).CreateSessionRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// GetSession gets a session in the sandbox.
func (p *Process) GetSession(ctx context.Context, sessionID string) (*apiclient.Session, error) {
	response, httpResp, err := p.sandbox.toolboxAPI.GetSessionDeprecated(ctx, p.sandbox.ID, sessionID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// GetSessionCommand gets information about a specific command executed in a session.
func (p *Process) GetSessionCommand(ctx context.Context, sessionID, commandID string) (*apiclient.Command, error) {
	response, httpResp, err := p.sandbox.toolboxAPI.GetSessionCommandDeprecated(ctx, p.sandbox.ID, sessionID, commandID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// ExecuteSessionCommand executes a command in an existing session.
func (p *Process) ExecuteSessionCommand(ctx context.Context, sessionID string, command string, async bool, timeout *float32) (*SessionExecuteResponse, error) {
	req := apiclient.SessionExecuteRequest{
		Command:  command,
		RunAsync: &async,
	}

	response, httpResp, err := p.sandbox.toolboxAPI.ExecuteSessionCommandDeprecated(ctx, p.sandbox.ID, sessionID).SessionExecuteRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	result := &SessionExecuteResponse{}
	if response.CmdId != nil {
		result.CmdID = *response.CmdId
	}
	if response.ExitCode != nil {
		exitCode := int32(*response.ExitCode)
		result.ExitCode = &exitCode
	}

	if response.Output != nil {
		result.Output = *response.Output
		// Demux the output
		stdout, stderr := demuxLog([]byte(*response.Output))
		result.Stdout = string(stdout)
		result.Stderr = string(stderr)
	}

	return result, nil
}

// GetSessionCommandLogs gets the logs for a command executed in a session.
func (p *Process) GetSessionCommandLogs(ctx context.Context, sessionID, commandID string) (*SessionCommandLogsResponse, error) {
	response, httpResp, err := p.sandbox.toolboxAPI.GetSessionCommandLogsDeprecated(ctx, p.sandbox.ID, sessionID, commandID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	result := &SessionCommandLogsResponse{
		Output: response,
	}
	stdout, stderr := demuxLog([]byte(response))
	result.Stdout = string(stdout)
	result.Stderr = string(stderr)

	return result, nil
}

// ListSessions lists all active sessions in the Sandbox.
func (p *Process) ListSessions(ctx context.Context) ([]apiclient.Session, error) {
	response, httpResp, err := p.sandbox.toolboxAPI.ListSessionsDeprecated(ctx, p.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// DeleteSession deletes a session from the Sandbox.
func (p *Process) DeleteSession(ctx context.Context, sessionID string) error {
	httpResp, err := p.sandbox.toolboxAPI.DeleteSessionDeprecated(ctx, p.sandbox.ID, sessionID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// PtyCreateOptions contains options for creating a PTY session.
type PtyCreateOptions struct {
	// ID is the unique identifier for the PTY session.
	ID string
	// Cwd is the starting directory for the PTY session.
	Cwd string
	// Envs contains environment variables for the PTY session.
	Envs map[string]string
	// Cols is the number of terminal columns.
	Cols *float32
	// Rows is the number of terminal rows.
	Rows *float32
}

// CreatePty creates a new PTY session in the sandbox.
func (p *Process) CreatePty(ctx context.Context, options PtyCreateOptions) (*apiclient.PtyCreateResponse, error) {
	lazyStart := true

	// Convert Envs to map[string]interface{}
	envs := make(map[string]interface{})
	for k, v := range options.Envs {
		envs[k] = v
	}

	req := apiclient.PtyCreateRequest{
		Id:        options.ID,
		Cwd:       &options.Cwd,
		Envs:      envs,
		Cols:      options.Cols,
		Rows:      options.Rows,
		LazyStart: &lazyStart,
	}

	response, httpResp, err := p.sandbox.toolboxAPI.CreatePTYSessionDeprecated(ctx, p.sandbox.ID).PtyCreateRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	return response, nil
}

// ListPtySessions lists all PTY sessions in the sandbox.
func (p *Process) ListPtySessions(ctx context.Context) ([]apiclient.PtySessionInfo, error) {
	response, httpResp, err := p.sandbox.toolboxAPI.ListPTYSessionsDeprecated(ctx, p.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response.Sessions, nil
}

// GetPtySessionInfo gets detailed information about a specific PTY session.
func (p *Process) GetPtySessionInfo(ctx context.Context, sessionID string) (*apiclient.PtySessionInfo, error) {
	response, httpResp, err := p.sandbox.toolboxAPI.GetPTYSessionDeprecated(ctx, p.sandbox.ID, sessionID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// KillPtySession kills a PTY session and terminates its associated process.
func (p *Process) KillPtySession(ctx context.Context, sessionID string) error {
	httpResp, err := p.sandbox.toolboxAPI.DeletePTYSessionDeprecated(ctx, p.sandbox.ID, sessionID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// ResizePtySession resizes a PTY session's terminal dimensions.
func (p *Process) ResizePtySession(ctx context.Context, sessionID string, cols, rows float32) (*apiclient.PtySessionInfo, error) {
	req := apiclient.PtyResizeRequest{
		Cols: cols,
		Rows: rows,
	}
	response, httpResp, err := p.sandbox.toolboxAPI.ResizePTYSessionDeprecated(ctx, p.sandbox.ID, sessionID).PtyResizeRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// CreatePtyWithHandle creates a new PTY session and returns a handle for interacting with it.
// The handle provides methods for sending input, resizing, and waiting for the session to exit.
func (p *Process) CreatePtyWithHandle(ctx context.Context, options PtyCreateOptions, connectOpts PtyConnectOptions) (*PtyHandle, error) {
	// Create the PTY session
	response, err := p.CreatePty(ctx, options)
	if err != nil {
		return nil, err
	}

	// Connect to the PTY session
	return p.ConnectPty(ctx, response.SessionId, connectOpts)
}

// ConnectPty connects to an existing PTY session and returns a handle for interacting with it.
func (p *Process) ConnectPty(ctx context.Context, sessionID string, options PtyConnectOptions) (*PtyHandle, error) {
	// Get the toolbox URL
	toolboxURL, err := p.sandbox.getToolboxURL(ctx)
	if err != nil {
		return nil, err
	}

	// Get the API key from the configuration
	apiKey := ""
	if p.sandbox.apiClient != nil && p.sandbox.apiClient.GetConfig() != nil {
		apiKey = p.sandbox.apiClient.GetConfig().DefaultHeader["Authorization"]
		// Strip "Bearer " prefix if present
		if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
			apiKey = apiKey[7:]
		}
	}

	// Create WebSocket connection
	ws, err := createPtyWebSocket(toolboxURL, p.sandbox.ID, sessionID, apiKey)
	if err != nil {
		return nil, err
	}

	// Create the handle with resize and kill callbacks
	handle := NewPtyHandle(
		ws,
		sessionID,
		options.OnData,
		func(cols, rows float32) error {
			_, err := p.ResizePtySession(ctx, sessionID, cols, rows)
			return err
		},
		func() error {
			return p.KillPtySession(ctx, sessionID)
		},
	)

	// Wait for connection to be established
	if err := handle.WaitForConnection(); err != nil {
		handle.Disconnect()
		return nil, err
	}

	return handle, nil
}

// demuxLog demultiplexes combined stdout/stderr log data.
func demuxLog(data []byte) ([]byte, []byte) {
	var outChunks, errChunks [][]byte
	state := "none"
	i := 0

	for i < len(data) {
		stdoutIndex := bytes.Index(data[i:], STDOUT_PREFIX_BYTES)
		stderrIndex := bytes.Index(data[i:], STDERR_PREFIX_BYTES)

		if stdoutIndex != -1 {
			stdoutIndex += i
		}
		if stderrIndex != -1 {
			stderrIndex += i
		}

		nextIdx := -1
		nextMarker := ""
		nextLen := 0

		if stdoutIndex != -1 && (stderrIndex == -1 || stdoutIndex < stderrIndex) {
			nextIdx = stdoutIndex
			nextMarker = "stdout"
			nextLen = len(STDOUT_PREFIX_BYTES)
		} else if stderrIndex != -1 {
			nextIdx = stderrIndex
			nextMarker = "stderr"
			nextLen = len(STDERR_PREFIX_BYTES)
		}

		if nextIdx == -1 {
			// No more markers, dump remainder into current state
			if state == "stdout" {
				outChunks = append(outChunks, data[i:])
			} else if state == "stderr" {
				errChunks = append(errChunks, data[i:])
			}
			break
		}

		// Write everything before the marker into current state
		if state == "stdout" && nextIdx > i {
			outChunks = append(outChunks, data[i:nextIdx])
		} else if state == "stderr" && nextIdx > i {
			errChunks = append(errChunks, data[i:nextIdx])
		}

		// Advance past marker and switch state
		i = nextIdx + nextLen
		state = nextMarker
	}

	return bytes.Join(outChunks, nil), bytes.Join(errChunks, nil)
}
