/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	apiclient "github.com/forkbikash/daytona/libs/api-client-go"
	"github.com/forkbikash/daytona/libs/sdk-go/codetoolbox"
	sdkerrors "github.com/forkbikash/daytona/libs/sdk-go/errors"
)

// Sandbox represents a Daytona Sandbox.
type Sandbox struct {
	// FS provides file system operations.
	FS *FileSystem
	// Git provides git operations.
	Git *Git
	// Process provides process execution capabilities.
	Process *Process
	// ComputerUse provides desktop automation capabilities.
	ComputerUse *ComputerUse
	// CodeInterpreter provides code interpretation capabilities.
	CodeInterpreter *CodeInterpreter

	// ID is the unique identifier for the Sandbox.
	ID string
	// Name is the name of the Sandbox.
	Name string
	// OrganizationID is the organization ID of the Sandbox.
	OrganizationID string
	// Snapshot is the snapshot used to create the Sandbox.
	Snapshot *string
	// User is the OS user running in the Sandbox.
	User string
	// Env contains environment variables set in the Sandbox.
	Env map[string]string
	// Labels contains custom labels attached to the Sandbox.
	Labels map[string]string
	// Public indicates whether the Sandbox is publicly accessible.
	Public bool
	// Target is the target location of the runner where the Sandbox runs.
	Target string
	// CPU is the number of CPUs allocated to the Sandbox.
	CPU float32
	// GPU is the number of GPUs allocated to the Sandbox.
	GPU float32
	// Memory is the amount of memory allocated to the Sandbox in GiB.
	Memory float32
	// Disk is the amount of disk space allocated to the Sandbox in GiB.
	Disk float32
	// State is the current state of the Sandbox.
	State *apiclient.SandboxState
	// ErrorReason is the error message if Sandbox is in error state.
	ErrorReason *string
	// BackupState is the current state of Sandbox backup.
	BackupState *string
	// BackupCreatedAt is when the backup was created.
	BackupCreatedAt *string
	// AutoStopInterval is the auto-stop interval in minutes.
	AutoStopInterval *float32
	// AutoArchiveInterval is the auto-archive interval in minutes.
	AutoArchiveInterval *float32
	// AutoDeleteInterval is the auto-delete interval in minutes.
	AutoDeleteInterval *float32
	// Volumes contains volumes attached to the Sandbox.
	Volumes []apiclient.SandboxVolume
	// BuildInfo contains build information for the Sandbox.
	BuildInfo *apiclient.BuildInfo
	// CreatedAt is when the Sandbox was created.
	CreatedAt *string
	// UpdatedAt is when the Sandbox was last updated.
	UpdatedAt *string
	// NetworkBlockAll indicates whether to block all network access.
	NetworkBlockAll bool
	// NetworkAllowList is a comma-separated list of allowed CIDR addresses.
	NetworkAllowList *string

	sandboxAPI      apiclient.SandboxAPI
	toolboxAPI      apiclient.ToolboxAPI
	codeToolbox     codetoolbox.SandboxCodeToolbox
	getToolboxURL   func(ctx context.Context) (string, error)
	toolboxBaseURL  string
	apiClient       *apiclient.APIClient
}

// newSandbox creates a new Sandbox instance.
func newSandbox(
	dto *apiclient.Sandbox,
	apiClient *apiclient.APIClient,
	codeToolbox codetoolbox.SandboxCodeToolbox,
	getToolboxURL func(ctx context.Context) (string, error),
) *Sandbox {
	s := &Sandbox{
		sandboxAPI:    apiClient.SandboxAPI,
		toolboxAPI:    apiClient.ToolboxAPI,
		codeToolbox:   codeToolbox,
		getToolboxURL: getToolboxURL,
		apiClient:     apiClient,
	}
	s.processSandboxDTO(dto)

	// Initialize services
	s.FS = NewFileSystem(s)
	s.Git = NewGit(s)
	s.Process = NewProcess(s, codeToolbox)
	s.ComputerUse = NewComputerUse(s)
	s.CodeInterpreter = NewCodeInterpreter(s)

	return s
}

// processSandboxDTO updates the Sandbox with data from the API response.
func (s *Sandbox) processSandboxDTO(dto *apiclient.Sandbox) {
	s.ID = dto.Id
	s.Name = dto.Name
	s.OrganizationID = dto.OrganizationId
	s.Snapshot = dto.Snapshot
	s.User = dto.User
	s.Env = dto.Env
	s.Labels = dto.Labels
	s.Public = dto.Public
	s.Target = dto.Target
	s.CPU = dto.Cpu
	s.GPU = dto.Gpu
	s.Memory = dto.Memory
	s.Disk = dto.Disk
	s.State = dto.State
	s.ErrorReason = dto.ErrorReason
	s.BackupState = dto.BackupState
	s.BackupCreatedAt = dto.BackupCreatedAt
	s.AutoStopInterval = dto.AutoStopInterval
	s.AutoArchiveInterval = dto.AutoArchiveInterval
	s.AutoDeleteInterval = dto.AutoDeleteInterval
	s.Volumes = dto.Volumes
	s.BuildInfo = dto.BuildInfo
	s.CreatedAt = dto.CreatedAt
	s.UpdatedAt = dto.UpdatedAt
	s.NetworkBlockAll = dto.NetworkBlockAll
	s.NetworkAllowList = dto.NetworkAllowList
}

// getToolboxBaseURL returns the base URL for toolbox API calls.
func (s *Sandbox) getToolboxBaseURL(ctx context.Context) (string, error) {
	if s.toolboxBaseURL != "" {
		return s.toolboxBaseURL, nil
	}

	baseURL, err := s.getToolboxURL(ctx)
	if err != nil {
		return "", err
	}

	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	s.toolboxBaseURL = baseURL + s.ID

	return s.toolboxBaseURL, nil
}

// GetUserHomeDir returns the user's home directory path inside the Sandbox.
func (s *Sandbox) GetUserHomeDir(ctx context.Context) (string, error) {
	response, httpResp, err := s.toolboxAPI.GetUserHomeDirDeprecated(ctx, s.ID).Execute()
	if err != nil {
		return "", handleAPIError(err, httpResp)
	}
	if response.Dir != nil {
		return *response.Dir, nil
	}
	return "", nil
}

// GetWorkDir returns the working directory path inside the Sandbox.
func (s *Sandbox) GetWorkDir(ctx context.Context) (string, error) {
	response, httpResp, err := s.toolboxAPI.GetWorkDirDeprecated(ctx, s.ID).Execute()
	if err != nil {
		return "", handleAPIError(err, httpResp)
	}
	if response.Dir != nil {
		return *response.Dir, nil
	}
	return "", nil
}

// CreateLspServer creates a new LSP server instance.
func (s *Sandbox) CreateLspServer(languageID LspLanguageID, pathToProject string) *LspServer {
	return NewLspServer(languageID, pathToProject, s)
}

// SetLabels sets labels for the Sandbox.
func (s *Sandbox) SetLabels(ctx context.Context, labels map[string]string) (map[string]string, error) {
	req := apiclient.SandboxLabels{Labels: labels}
	response, httpResp, err := s.sandboxAPI.ReplaceLabels(ctx, s.ID).SandboxLabels(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	s.Labels = response.Labels
	return s.Labels, nil
}

// Start starts the Sandbox and waits for it to be ready.
func (s *Sandbox) Start(ctx context.Context, timeout int) error {
	if timeout < 0 {
		return sdkerrors.NewDaytonaError("Timeout must be a non-negative number", 0, nil)
	}

	startTime := time.Now()

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	response, httpResp, err := s.sandboxAPI.StartSandbox(timeoutCtx, s.ID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	s.processSandboxDTO(response)

	timeElapsed := time.Since(startTime)
	remainingTimeout := time.Duration(timeout)*time.Second - timeElapsed
	if remainingTimeout > 0 {
		return s.WaitUntilStarted(ctx, int(remainingTimeout.Seconds()))
	}
	return nil
}

// Stop stops the Sandbox.
func (s *Sandbox) Stop(ctx context.Context, timeout int) error {
	if timeout < 0 {
		return sdkerrors.NewDaytonaError("Timeout must be a non-negative number", 0, nil)
	}

	startTime := time.Now()

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	_, httpResp, err := s.sandboxAPI.StopSandbox(timeoutCtx, s.ID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}

	_ = s.refreshDataSafe(ctx)

	timeElapsed := time.Since(startTime)
	remainingTimeout := time.Duration(timeout)*time.Second - timeElapsed
	if remainingTimeout > 0 {
		return s.WaitUntilStopped(ctx, int(remainingTimeout.Seconds()))
	}
	return nil
}

// Delete deletes the Sandbox.
func (s *Sandbox) Delete(ctx context.Context, timeout int) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	_, httpResp, err := s.sandboxAPI.DeleteSandbox(timeoutCtx, s.ID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}

	_ = s.refreshDataSafe(ctx)
	return nil
}

// WaitUntilStarted waits for the Sandbox to reach the 'started' state.
func (s *Sandbox) WaitUntilStarted(ctx context.Context, timeout int) error {
	if timeout < 0 {
		return sdkerrors.NewDaytonaError("Timeout must be a non-negative number", 0, nil)
	}

	checkInterval := 100 * time.Millisecond
	startTime := time.Now()

	for s.State == nil || *s.State != apiclient.SANDBOXSTATE_STARTED {
		if err := s.RefreshData(ctx); err != nil {
			return err
		}

		if s.State != nil && *s.State == apiclient.SANDBOXSTATE_STARTED {
			return nil
		}

		if s.State != nil && *s.State == apiclient.SANDBOXSTATE_ERROR {
			errMsg := "Sandbox failed to start with status: error"
			if s.ErrorReason != nil {
				errMsg += ", error reason: " + *s.ErrorReason
			}
			return sdkerrors.NewDaytonaError(errMsg, 0, nil)
		}

		if timeout != 0 && time.Since(startTime) > time.Duration(timeout)*time.Second {
			return sdkerrors.NewDaytonaError("Sandbox failed to become ready within the timeout period", 0, nil)
		}

		time.Sleep(checkInterval)
	}

	return nil
}

// WaitUntilStopped waits for the Sandbox to reach the 'stopped' state.
func (s *Sandbox) WaitUntilStopped(ctx context.Context, timeout int) error {
	if timeout < 0 {
		return sdkerrors.NewDaytonaError("Timeout must be a non-negative number", 0, nil)
	}

	checkInterval := 100 * time.Millisecond
	startTime := time.Now()

	for s.State == nil || (*s.State != apiclient.SANDBOXSTATE_STOPPED && *s.State != apiclient.SANDBOXSTATE_DESTROYED) {
		_ = s.refreshDataSafe(ctx)

		if s.State != nil && (*s.State == apiclient.SANDBOXSTATE_STOPPED || *s.State == apiclient.SANDBOXSTATE_DESTROYED) {
			return nil
		}

		if s.State != nil && *s.State == apiclient.SANDBOXSTATE_ERROR {
			errMsg := "Sandbox failed to stop with status: error"
			if s.ErrorReason != nil {
				errMsg += ", error reason: " + *s.ErrorReason
			}
			return sdkerrors.NewDaytonaError(errMsg, 0, nil)
		}

		if timeout != 0 && time.Since(startTime) > time.Duration(timeout)*time.Second {
			return sdkerrors.NewDaytonaError("Sandbox failed to become stopped within the timeout period", 0, nil)
		}

		time.Sleep(checkInterval)
	}

	return nil
}

// RefreshData refreshes the Sandbox data from the API.
func (s *Sandbox) RefreshData(ctx context.Context) error {
	response, httpResp, err := s.sandboxAPI.GetSandbox(ctx, s.ID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	s.processSandboxDTO(response)
	return nil
}

// RefreshActivity refreshes the sandbox activity to reset the timer for automated lifecycle management.
func (s *Sandbox) RefreshActivity(ctx context.Context) error {
	httpResp, err := s.sandboxAPI.UpdateLastActivity(ctx, s.ID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// SetAutostopInterval sets the auto-stop interval for the Sandbox.
func (s *Sandbox) SetAutostopInterval(ctx context.Context, interval float32) error {
	if interval < 0 {
		return sdkerrors.NewDaytonaError("autoStopInterval must be a non-negative integer", 0, nil)
	}

	_, httpResp, err := s.sandboxAPI.SetAutostopInterval(ctx, s.ID, interval).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	s.AutoStopInterval = &interval
	return nil
}

// SetAutoArchiveInterval sets the auto-archive interval for the Sandbox.
func (s *Sandbox) SetAutoArchiveInterval(ctx context.Context, interval float32) error {
	if interval < 0 {
		return sdkerrors.NewDaytonaError("autoArchiveInterval must be a non-negative integer", 0, nil)
	}

	_, httpResp, err := s.sandboxAPI.SetAutoArchiveInterval(ctx, s.ID, interval).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	s.AutoArchiveInterval = &interval
	return nil
}

// SetAutoDeleteInterval sets the auto-delete interval for the Sandbox.
func (s *Sandbox) SetAutoDeleteInterval(ctx context.Context, interval float32) error {
	_, httpResp, err := s.sandboxAPI.SetAutoDeleteInterval(ctx, s.ID, interval).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	s.AutoDeleteInterval = &interval
	return nil
}

// GetPreviewLink retrieves the preview link for the sandbox at the specified port.
func (s *Sandbox) GetPreviewLink(ctx context.Context, port float32) (*apiclient.PortPreviewUrl, error) {
	response, httpResp, err := s.sandboxAPI.GetPortPreviewUrl(ctx, s.ID, port).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Archive archives the sandbox.
func (s *Sandbox) Archive(ctx context.Context) error {
	_, httpResp, err := s.sandboxAPI.ArchiveSandbox(ctx, s.ID).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return s.RefreshData(ctx)
}

// CreateSshAccess creates an SSH access token for the sandbox.
func (s *Sandbox) CreateSshAccess(ctx context.Context, expiresInMinutes *float32) (*apiclient.SshAccessDto, error) {
	req := s.sandboxAPI.CreateSshAccess(ctx, s.ID)
	if expiresInMinutes != nil {
		req = req.ExpiresInMinutes(*expiresInMinutes)
	}
	response, httpResp, err := req.Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// RevokeSshAccess revokes an SSH access token for the sandbox.
func (s *Sandbox) RevokeSshAccess(ctx context.Context, token string) error {
	_, httpResp, err := s.sandboxAPI.RevokeSshAccess(ctx, s.ID).Token(token).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// ValidateSshAccess validates an SSH access token for the sandbox.
func (s *Sandbox) ValidateSshAccess(ctx context.Context, token string) (*apiclient.SshAccessValidationDto, error) {
	response, httpResp, err := s.sandboxAPI.ValidateSshAccess(ctx).Token(token).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// refreshDataSafe refreshes the Sandbox data but doesn't throw an error if the sandbox has been deleted.
func (s *Sandbox) refreshDataSafe(ctx context.Context) error {
	err := s.RefreshData(ctx)
	if err != nil {
		if sdkerrors.IsDaytonaNotFoundError(err) {
			state := apiclient.SANDBOXSTATE_DESTROYED
			s.State = &state
			return nil
		}
		return err
	}
	return nil
}

// getPreviewToken returns the preview token for the sandbox.
func (s *Sandbox) getPreviewToken(ctx context.Context) (string, error) {
	preview, err := s.GetPreviewLink(ctx, 1)
	if err != nil {
		return "", err
	}
	return preview.Token, nil
}

// MarshalJSON implements json.Marshaler.
func (s *Sandbox) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID                  string                  `json:"id"`
		Name                string                  `json:"name"`
		OrganizationID      string                  `json:"organizationId"`
		Snapshot            *string                 `json:"snapshot,omitempty"`
		User                string                  `json:"user"`
		Env                 map[string]string       `json:"env"`
		Labels              map[string]string       `json:"labels"`
		Public              bool                    `json:"public"`
		Target              string                  `json:"target"`
		CPU                 float32                 `json:"cpu"`
		GPU                 float32                 `json:"gpu"`
		Memory              float32                 `json:"memory"`
		Disk                float32                 `json:"disk"`
		State               *apiclient.SandboxState `json:"state,omitempty"`
		ErrorReason         *string                 `json:"errorReason,omitempty"`
		BackupState         *string                 `json:"backupState,omitempty"`
		AutoStopInterval    *float32                `json:"autoStopInterval,omitempty"`
		AutoArchiveInterval *float32                `json:"autoArchiveInterval,omitempty"`
		AutoDeleteInterval  *float32                `json:"autoDeleteInterval,omitempty"`
		NetworkBlockAll     bool                    `json:"networkBlockAll"`
		NetworkAllowList    *string                 `json:"networkAllowList,omitempty"`
	}{
		ID:                  s.ID,
		Name:                s.Name,
		OrganizationID:      s.OrganizationID,
		Snapshot:            s.Snapshot,
		User:                s.User,
		Env:                 s.Env,
		Labels:              s.Labels,
		Public:              s.Public,
		Target:              s.Target,
		CPU:                 s.CPU,
		GPU:                 s.GPU,
		Memory:              s.Memory,
		Disk:                s.Disk,
		State:               s.State,
		ErrorReason:         s.ErrorReason,
		BackupState:         s.BackupState,
		AutoStopInterval:    s.AutoStopInterval,
		AutoArchiveInterval: s.AutoArchiveInterval,
		AutoDeleteInterval:  s.AutoDeleteInterval,
		NetworkBlockAll:     s.NetworkBlockAll,
		NetworkAllowList:    s.NetworkAllowList,
	})
}
