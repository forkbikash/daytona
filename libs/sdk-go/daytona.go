/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

// Package daytona provides a Go SDK for interacting with Daytona sandboxes.
package daytona

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	apiclient "github.com/forkbikash/daytona/libs/api-client-go"
	"github.com/forkbikash/daytona/libs/sdk-go/codetoolbox"
	sdkerrors "github.com/forkbikash/daytona/libs/sdk-go/errors"
	"github.com/joho/godotenv"
)

const (
	defaultAPIURL = "https://app.daytona.io/api"
	sdkVersion    = "0.1.0"
)

// CodeLanguage represents supported programming languages for code execution.
type CodeLanguage string

const (
	CodeLanguagePython     CodeLanguage = "python"
	CodeLanguageTypeScript CodeLanguage = "typescript"
	CodeLanguageJavaScript CodeLanguage = "javascript"
)

// VolumeMount represents a volume mount for a Sandbox.
type VolumeMount struct {
	VolumeID  string `json:"volumeId"`
	MountPath string `json:"mountPath"`
}

// Config holds configuration options for initializing the Daytona client.
type Config struct {
	// APIKey is the API key for authentication with the Daytona API.
	APIKey string
	// JWTToken is the JWT token for authentication with the Daytona API.
	JWTToken string
	// OrganizationID is the organization ID for authentication.
	OrganizationID string
	// APIURL is the URL of the Daytona API.
	APIURL string
	// Target is the target location for Sandboxes.
	Target string
}

// Resources represents resource allocation for a Sandbox.
type Resources struct {
	// CPU allocation for the Sandbox in cores.
	CPU *int32
	// GPU allocation for the Sandbox in units.
	GPU *int32
	// Memory allocation for the Sandbox in GiB.
	Memory *int32
	// Disk space allocation for the Sandbox in GiB.
	Disk *int32
}

// CreateSandboxBaseParams contains base parameters for creating a new Sandbox.
type CreateSandboxBaseParams struct {
	// Name is the optional name of the Sandbox.
	Name string
	// User is the optional os user to use for the Sandbox.
	User string
	// Language is the programming language for direct code execution.
	Language CodeLanguage
	// EnvVars are optional environment variables to set in the Sandbox.
	EnvVars map[string]string
	// Labels are optional Sandbox labels.
	Labels map[string]string
	// Public indicates if the Sandbox port preview is public.
	Public bool
	// AutoStopInterval is the auto-stop interval in minutes (0 means disabled).
	AutoStopInterval *int32
	// AutoArchiveInterval is the auto-archive interval in minutes.
	AutoArchiveInterval *int32
	// AutoDeleteInterval is the auto-delete interval in minutes.
	AutoDeleteInterval *int32
	// Volumes is an optional array of volumes to mount to the Sandbox.
	Volumes []VolumeMount
	// NetworkBlockAll indicates whether to block all network access.
	NetworkBlockAll bool
	// NetworkAllowList is a comma-separated list of allowed CIDR network addresses.
	NetworkAllowList string
	// Ephemeral indicates whether the Sandbox should be ephemeral.
	Ephemeral bool
}

// CreateSandboxFromSnapshotParams contains parameters for creating a Sandbox from a snapshot.
type CreateSandboxFromSnapshotParams struct {
	CreateSandboxBaseParams
	// Snapshot is the name of the snapshot to use for the Sandbox.
	Snapshot string
}

// CreateSandboxFromImageParams contains parameters for creating a Sandbox from an image.
type CreateSandboxFromImageParams struct {
	CreateSandboxBaseParams
	// Image is the Docker image to use for the Sandbox.
	Image string
	// DynamicImage is a dynamically built Image.
	DynamicImage *Image
	// Resources is the resource allocation for the Sandbox.
	Resources *Resources
}

// CreateOptions contains options for the create operation.
type CreateOptions struct {
	// Timeout in seconds (0 means no timeout, default is 60).
	Timeout int
	// OnSnapshotCreateLogs is a callback function to handle snapshot creation logs.
	OnSnapshotCreateLogs func(chunk string)
}

// SandboxFilter is used to filter Sandboxes.
type SandboxFilter struct {
	// IDOrName is the ID or name of the Sandbox to retrieve.
	IDOrName string
	// Labels are labels to filter Sandboxes.
	Labels map[string]string
}

// PaginatedSandboxes represents a paginated list of Sandboxes.
type PaginatedSandboxes struct {
	Items      []*Sandbox
	Total      int32
	Page       int32
	TotalPages int32
}

// Daytona is the main client for interacting with the Daytona API.
type Daytona struct {
	apiClient        *apiclient.APIClient
	sandboxAPI       apiclient.SandboxAPI
	objectStorageAPI apiclient.ObjectStorageAPI
	configAPI        apiclient.ConfigAPI

	apiKey         string
	jwtToken       string
	organizationID string
	apiURL         string
	target         string

	proxyToolboxURL string

	// Volume is the service for managing Daytona Volumes.
	Volume *VolumeService
	// Snapshot is the service for managing Daytona Snapshots.
	Snapshot *SnapshotService
}

// NewDaytona creates a new Daytona client instance.
func NewDaytona(config *Config) (*Daytona, error) {
	d := &Daytona{}

	// Load environment variables from .env files
	_ = godotenv.Load()
	_ = godotenv.Overload(".env.local")

	// Set configuration from provided config or environment variables
	if config != nil {
		d.apiKey = config.APIKey
		d.jwtToken = config.JWTToken
		d.organizationID = config.OrganizationID
		d.apiURL = config.APIURL
		d.target = config.Target
	}

	// Fall back to environment variables
	if d.apiKey == "" && d.jwtToken == "" {
		d.apiKey = os.Getenv("DAYTONA_API_KEY")
	}
	if d.jwtToken == "" {
		d.jwtToken = os.Getenv("DAYTONA_JWT_TOKEN")
	}
	if d.organizationID == "" {
		d.organizationID = os.Getenv("DAYTONA_ORGANIZATION_ID")
	}
	if d.apiURL == "" {
		d.apiURL = os.Getenv("DAYTONA_API_URL")
		if d.apiURL == "" {
			d.apiURL = os.Getenv("DAYTONA_SERVER_URL")
		}
	}
	if d.target == "" {
		d.target = os.Getenv("DAYTONA_TARGET")
	}

	// Set default API URL
	if d.apiURL == "" {
		d.apiURL = defaultAPIURL
	}

	// Validate configuration
	if d.apiKey == "" && d.jwtToken == "" {
		return nil, sdkerrors.NewDaytonaError("API key or JWT token is required", 0, nil)
	}

	if d.apiKey == "" && d.organizationID == "" {
		return nil, sdkerrors.NewDaytonaError("Organization ID is required when using JWT token", 0, nil)
	}

	// Create API client configuration
	apiConfig := apiclient.NewConfiguration()
	apiConfig.Servers = apiclient.ServerConfigurations{
		{URL: d.apiURL},
	}

	// Set authorization header
	token := d.apiKey
	if token == "" {
		token = d.jwtToken
	}

	apiConfig.AddDefaultHeader("Authorization", "Bearer "+token)
	apiConfig.AddDefaultHeader("X-Daytona-Source", "go-sdk")
	apiConfig.AddDefaultHeader("X-Daytona-SDK-Version", sdkVersion)

	if d.apiKey == "" && d.organizationID != "" {
		apiConfig.AddDefaultHeader("X-Daytona-Organization-ID", d.organizationID)
	}

	d.apiClient = apiclient.NewAPIClient(apiConfig)
	d.sandboxAPI = d.apiClient.SandboxAPI
	d.objectStorageAPI = d.apiClient.ObjectStorageAPI
	d.configAPI = d.apiClient.ConfigAPI

	// Initialize services
	d.Volume = NewVolumeService(d.apiClient.VolumesAPI)
	d.Snapshot = NewSnapshotService(apiConfig, d.apiClient.SnapshotsAPI, d.apiClient.ObjectStorageAPI)

	return d, nil
}

// Create creates a new Sandbox from a snapshot.
func (d *Daytona) Create(ctx context.Context, params *CreateSandboxFromSnapshotParams, opts *CreateOptions) (*Sandbox, error) {
	return d.createSandbox(ctx, params, nil, opts)
}

// CreateFromImage creates a new Sandbox from an image.
func (d *Daytona) CreateFromImage(ctx context.Context, params *CreateSandboxFromImageParams, opts *CreateOptions) (*Sandbox, error) {
	return d.createSandbox(ctx, nil, params, opts)
}

func (d *Daytona) createSandbox(ctx context.Context, snapshotParams *CreateSandboxFromSnapshotParams, imageParams *CreateSandboxFromImageParams, opts *CreateOptions) (*Sandbox, error) {
	startTime := time.Now()

	// Set default options
	if opts == nil {
		opts = &CreateOptions{Timeout: 60}
	}
	if opts.Timeout == 0 {
		opts.Timeout = 60
	}

	// Get base params
	var baseParams CreateSandboxBaseParams
	if snapshotParams != nil {
		baseParams = snapshotParams.CreateSandboxBaseParams
	} else if imageParams != nil {
		baseParams = imageParams.CreateSandboxBaseParams
	} else {
		baseParams = CreateSandboxBaseParams{Language: CodeLanguagePython}
	}

	// Set default language
	if baseParams.Language == "" {
		baseParams.Language = CodeLanguagePython
	}

	// Prepare labels
	labels := baseParams.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["code-toolbox-language"] = string(baseParams.Language)

	// Validate parameters
	if opts.Timeout < 0 {
		return nil, sdkerrors.NewDaytonaError("Timeout must be a non-negative number", 0, nil)
	}

	if baseParams.AutoStopInterval != nil && *baseParams.AutoStopInterval < 0 {
		return nil, sdkerrors.NewDaytonaError("autoStopInterval must be a non-negative integer", 0, nil)
	}

	if baseParams.Ephemeral {
		autoDeleteZero := int32(0)
		baseParams.AutoDeleteInterval = &autoDeleteZero
	}

	if baseParams.AutoArchiveInterval != nil && *baseParams.AutoArchiveInterval < 0 {
		return nil, sdkerrors.NewDaytonaError("autoArchiveInterval must be a non-negative integer", 0, nil)
	}

	// Build create request
	createReq := apiclient.CreateSandbox{
		Public:           &baseParams.Public,
		NetworkBlockAll:  &baseParams.NetworkBlockAll,
		NetworkAllowList: &baseParams.NetworkAllowList,
	}
	if len(baseParams.EnvVars) > 0 {
		createReq.Env = &baseParams.EnvVars
	}
	if len(labels) > 0 {
		createReq.Labels = &labels
	}

	if baseParams.Name != "" {
		createReq.Name = &baseParams.Name
	}
	if baseParams.User != "" {
		createReq.User = &baseParams.User
	}
	if d.target != "" {
		createReq.Target = &d.target
	}
	if baseParams.AutoStopInterval != nil {
		createReq.AutoStopInterval = baseParams.AutoStopInterval
	}
	if baseParams.AutoArchiveInterval != nil {
		createReq.AutoArchiveInterval = baseParams.AutoArchiveInterval
	}
	if baseParams.AutoDeleteInterval != nil {
		createReq.AutoDeleteInterval = baseParams.AutoDeleteInterval
	}

	// Convert volume mounts
	if len(baseParams.Volumes) > 0 {
		volumes := make([]apiclient.SandboxVolume, len(baseParams.Volumes))
		for i, v := range baseParams.Volumes {
			volumes[i] = apiclient.SandboxVolume{
				VolumeId:  v.VolumeID,
				MountPath: v.MountPath,
			}
		}
		createReq.Volumes = volumes
	}

	// Handle snapshot params
	if snapshotParams != nil && snapshotParams.Snapshot != "" {
		createReq.Snapshot = &snapshotParams.Snapshot
	}

	// Handle image params
	if imageParams != nil {
		if imageParams.DynamicImage != nil {
			// Use dynamic image
			buildInfo := apiclient.CreateBuildInfo{
				DockerfileContent: imageParams.DynamicImage.dockerfile,
			}
			createReq.BuildInfo = &buildInfo
		} else if imageParams.Image != "" {
			// Use static image
			dockerfile := fmt.Sprintf("FROM %s", imageParams.Image)
			buildInfo := apiclient.CreateBuildInfo{
				DockerfileContent: dockerfile,
			}
			createReq.BuildInfo = &buildInfo
		}

		if imageParams.Resources != nil {
			createReq.Cpu = imageParams.Resources.CPU
			createReq.Gpu = imageParams.Resources.GPU
			createReq.Memory = imageParams.Resources.Memory
			createReq.Disk = imageParams.Resources.Disk
		}
	}

	// Create sandbox with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
	defer cancel()

	response, httpResp, err := d.sandboxAPI.CreateSandbox(timeoutCtx).CreateSandbox(createReq).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	// Get code toolbox
	codeToolbox := d.getCodeToolbox(baseParams.Language)

	// Create sandbox instance
	sandbox := newSandbox(response, d.apiClient, codeToolbox, d.getProxyToolboxURL)

	// Wait for sandbox to be started
	if sandbox.State != nil && *sandbox.State != apiclient.SANDBOXSTATE_STARTED {
		timeElapsed := time.Since(startTime)
		remainingTimeout := time.Duration(opts.Timeout)*time.Second - timeElapsed
		if remainingTimeout > 0 {
			if err := sandbox.WaitUntilStarted(ctx, int(remainingTimeout.Seconds())); err != nil {
				return nil, err
			}
		}
	}

	return sandbox, nil
}

// Get retrieves a Sandbox by its ID or name.
func (d *Daytona) Get(ctx context.Context, sandboxIDOrName string) (*Sandbox, error) {
	response, httpResp, err := d.sandboxAPI.GetSandbox(ctx, sandboxIDOrName).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	var language CodeLanguage
	if response.Labels != nil {
		if lang, ok := response.Labels["code-toolbox-language"]; ok {
			language = CodeLanguage(lang)
		}
	}
	codeToolbox := d.getCodeToolbox(language)

	return newSandbox(response, d.apiClient, codeToolbox, d.getProxyToolboxURL), nil
}

// FindOne finds a Sandbox by its ID, name, or labels.
func (d *Daytona) FindOne(ctx context.Context, filter SandboxFilter) (*Sandbox, error) {
	if filter.IDOrName != "" {
		return d.Get(ctx, filter.IDOrName)
	}

	result, err := d.List(ctx, filter.Labels, 1, 1)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		labelsJSON, _ := json.Marshal(filter.Labels)
		return nil, sdkerrors.NewDaytonaError(fmt.Sprintf("No sandbox found with labels %s", string(labelsJSON)), 0, nil)
	}

	return result.Items[0], nil
}

// List returns a paginated list of Sandboxes filtered by labels.
func (d *Daytona) List(ctx context.Context, labels map[string]string, page, limit int32) (*PaginatedSandboxes, error) {
	req := d.sandboxAPI.ListSandboxesPaginated(ctx)

	if page > 0 {
		req = req.Page(float32(page))
	}
	if limit > 0 {
		req = req.Limit(float32(limit))
	}
	if len(labels) > 0 {
		labelsJSON, _ := json.Marshal(labels)
		req = req.Labels(string(labelsJSON))
	}

	response, httpResp, err := req.Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}

	sandboxes := make([]*Sandbox, len(response.Items))
	for i, item := range response.Items {
		var language CodeLanguage
		if item.Labels != nil {
			if lang, ok := item.Labels["code-toolbox-language"]; ok {
				language = CodeLanguage(lang)
			}
		}
		codeToolbox := d.getCodeToolbox(language)
		sandboxes[i] = newSandbox(&item, d.apiClient, codeToolbox, d.getProxyToolboxURL)
	}

	return &PaginatedSandboxes{
		Items:      sandboxes,
		Total:      int32(response.Total),
		Page:       int32(response.Page),
		TotalPages: int32(response.TotalPages),
	}, nil
}

// Start starts a Sandbox and waits for it to be ready.
func (d *Daytona) Start(ctx context.Context, sandbox *Sandbox, timeout int) error {
	return sandbox.Start(ctx, timeout)
}

// Stop stops a Sandbox.
func (d *Daytona) Stop(ctx context.Context, sandbox *Sandbox, timeout int) error {
	return sandbox.Stop(ctx, timeout)
}

// Delete deletes a Sandbox.
func (d *Daytona) Delete(ctx context.Context, sandbox *Sandbox, timeout int) error {
	return sandbox.Delete(ctx, timeout)
}

// getCodeToolbox returns the appropriate code toolbox based on language.
func (d *Daytona) getCodeToolbox(language CodeLanguage) codetoolbox.SandboxCodeToolbox {
	switch language {
	case CodeLanguageJavaScript:
		return &codetoolbox.JavaScriptCodeToolbox{}
	case CodeLanguageTypeScript:
		return &codetoolbox.TypeScriptCodeToolbox{}
	case CodeLanguagePython, "":
		return &codetoolbox.PythonCodeToolbox{}
	default:
		return &codetoolbox.PythonCodeToolbox{}
	}
}

// getProxyToolboxURL returns the proxy toolbox URL.
func (d *Daytona) getProxyToolboxURL(ctx context.Context) (string, error) {
	if d.proxyToolboxURL != "" {
		return d.proxyToolboxURL, nil
	}

	config, httpResp, err := d.configAPI.ConfigControllerGetConfig(ctx).Execute()
	if err != nil {
		return "", handleAPIError(err, httpResp)
	}

	if config.ProxyToolboxUrl != "" {
		d.proxyToolboxURL = config.ProxyToolboxUrl
	}

	return d.proxyToolboxURL, nil
}

// handleAPIError handles API errors and converts them to SDK errors.
func handleAPIError(err error, resp *http.Response) error {
	if resp == nil {
		return sdkerrors.NewDaytonaError(err.Error(), 0, nil)
	}

	statusCode := resp.StatusCode
	headers := resp.Header

	switch statusCode {
	case http.StatusNotFound:
		return sdkerrors.NewDaytonaNotFoundError(err.Error(), statusCode, headers)
	case http.StatusTooManyRequests:
		return sdkerrors.NewDaytonaRateLimitError(err.Error(), statusCode, headers)
	default:
		return sdkerrors.NewDaytonaError(err.Error(), statusCode, headers)
	}
}
