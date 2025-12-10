/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"
	"io"
	"os"
	"path/filepath"

	apiclient "github.com/forkbikash/daytona/libs/api-client-go"
)

// FilePermissionsParams contains parameters for setting file permissions.
type FilePermissionsParams struct {
	// Mode is the file mode/permissions in octal format (e.g. "644").
	Mode string
	// Owner is the user owner of the file.
	Owner string
	// Group is the group owner of the file.
	Group string
}

// FileUpload represents a file to be uploaded to the Sandbox.
type FileUpload struct {
	// Source is the file content or local file path.
	Source interface{} // Can be []byte or string (file path)
	// Destination is the absolute destination path in the Sandbox.
	Destination string
}

// FileDownloadRequest represents a request to download a file from the Sandbox.
type FileDownloadRequest struct {
	// Source is the source path in the Sandbox.
	Source string
	// Destination is the optional destination path in the local filesystem.
	Destination string
}

// FileDownloadResponse represents the response to a file download request.
type FileDownloadResponse struct {
	// Source is the original source path requested for download.
	Source string
	// Result is the download result (file path or content).
	Result interface{} // Can be []byte or string
	// Error is the error message if the download failed.
	Error string
}

// FileSystem provides file system operations within a Sandbox.
type FileSystem struct {
	sandbox *Sandbox
}

// NewFileSystem creates a new FileSystem instance.
func NewFileSystem(sandbox *Sandbox) *FileSystem {
	return &FileSystem{sandbox: sandbox}
}

// CreateFolder creates a new directory in the Sandbox with specified permissions.
func (fs *FileSystem) CreateFolder(ctx context.Context, path string, mode string) error {
	httpResp, err := fs.sandbox.toolboxAPI.CreateFolderDeprecated(ctx, fs.sandbox.ID).Path(path).Mode(mode).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// DeleteFile deletes a file or directory from the Sandbox.
func (fs *FileSystem) DeleteFile(ctx context.Context, path string, recursive bool) error {
	req := fs.sandbox.toolboxAPI.DeleteFileDeprecated(ctx, fs.sandbox.ID).Path(path)
	if recursive {
		req = req.Recursive(recursive)
	}
	httpResp, err := req.Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// DownloadFile downloads a file from the Sandbox to memory.
func (fs *FileSystem) DownloadFile(ctx context.Context, remotePath string) ([]byte, error) {
	responses, err := fs.DownloadFiles(ctx, []FileDownloadRequest{{Source: remotePath}}, 30*60)
	if err != nil {
		return nil, err
	}

	if len(responses) > 0 && responses[0].Error != "" {
		return nil, &fileDownloadError{message: responses[0].Error}
	}

	if content, ok := responses[0].Result.([]byte); ok {
		return content, nil
	}

	return nil, nil
}

// DownloadFileToPath downloads a file from the Sandbox to a local file path.
func (fs *FileSystem) DownloadFileToPath(ctx context.Context, remotePath, localPath string) error {
	responses, err := fs.DownloadFiles(ctx, []FileDownloadRequest{{Source: remotePath, Destination: localPath}}, 30*60)
	if err != nil {
		return err
	}

	if len(responses) > 0 && responses[0].Error != "" {
		return &fileDownloadError{message: responses[0].Error}
	}

	return nil
}

// DownloadFiles downloads multiple files from the Sandbox.
func (fs *FileSystem) DownloadFiles(ctx context.Context, files []FileDownloadRequest, timeoutSec int) ([]FileDownloadResponse, error) {
	if len(files) == 0 {
		return []FileDownloadResponse{}, nil
	}

	// Create directories for destinations
	for _, f := range files {
		if f.Destination != "" {
			if err := os.MkdirAll(filepath.Dir(f.Destination), 0755); err != nil {
				return nil, err
			}
		}
	}

	// Prepare paths
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Source
	}

	req := apiclient.DownloadFiles{Paths: paths}
	fileResp, httpResp, err := fs.sandbox.toolboxAPI.DownloadFilesDeprecated(ctx, fs.sandbox.ID).DownloadFiles(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	defer fileResp.Close()

	// Read the response body
	content, err := io.ReadAll(fileResp)
	if err != nil {
		return nil, err
	}

	results := make([]FileDownloadResponse, len(files))
	for i, f := range files {
		if f.Destination != "" {
			// Write to file
			if err := os.WriteFile(f.Destination, content, 0644); err != nil {
				results[i] = FileDownloadResponse{
					Source: f.Source,
					Error:  err.Error(),
				}
			} else {
				results[i] = FileDownloadResponse{
					Source: f.Source,
					Result: f.Destination,
				}
			}
		} else {
			results[i] = FileDownloadResponse{
				Source: f.Source,
				Result: content,
			}
		}
	}

	return results, nil
}

// FindFiles searches for text patterns within files in the Sandbox.
func (fs *FileSystem) FindFiles(ctx context.Context, path, pattern string) ([]apiclient.Match, error) {
	response, httpResp, err := fs.sandbox.toolboxAPI.FindInFilesDeprecated(ctx, fs.sandbox.ID).Path(path).Pattern(pattern).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// GetFileDetails retrieves detailed information about a file or directory.
func (fs *FileSystem) GetFileDetails(ctx context.Context, path string) (*apiclient.FileInfo, error) {
	response, httpResp, err := fs.sandbox.toolboxAPI.GetFileInfoDeprecated(ctx, fs.sandbox.ID).Path(path).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// ListFiles lists contents of a directory in the Sandbox.
func (fs *FileSystem) ListFiles(ctx context.Context, path string) ([]apiclient.FileInfo, error) {
	response, httpResp, err := fs.sandbox.toolboxAPI.ListFilesDeprecated(ctx, fs.sandbox.ID).Path(path).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// MoveFiles moves or renames a file or directory.
func (fs *FileSystem) MoveFiles(ctx context.Context, source, destination string) error {
	httpResp, err := fs.sandbox.toolboxAPI.MoveFileDeprecated(ctx, fs.sandbox.ID).Source(source).Destination(destination).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// ReplaceInFiles replaces text content in multiple files.
func (fs *FileSystem) ReplaceInFiles(ctx context.Context, files []string, pattern, newValue string) ([]apiclient.ReplaceResult, error) {
	req := apiclient.ReplaceRequest{
		Files:    files,
		Pattern:  pattern,
		NewValue: newValue,
	}
	response, httpResp, err := fs.sandbox.toolboxAPI.ReplaceInFilesDeprecated(ctx, fs.sandbox.ID).ReplaceRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// SearchFiles searches for files and directories by name pattern in the Sandbox.
func (fs *FileSystem) SearchFiles(ctx context.Context, path, pattern string) (*apiclient.SearchFilesResponse, error) {
	response, httpResp, err := fs.sandbox.toolboxAPI.SearchFilesDeprecated(ctx, fs.sandbox.ID).Path(path).Pattern(pattern).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// SetFilePermissions sets permissions and ownership for a file or directory.
func (fs *FileSystem) SetFilePermissions(ctx context.Context, path string, permissions FilePermissionsParams) error {
	httpResp, err := fs.sandbox.toolboxAPI.SetFilePermissionsDeprecated(ctx, fs.sandbox.ID).Path(path).Mode(permissions.Mode).Owner(permissions.Owner).Group(permissions.Group).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// UploadFile uploads a file buffer to the Sandbox.
func (fs *FileSystem) UploadFile(ctx context.Context, content []byte, remotePath string) error {
	return fs.UploadFiles(ctx, []FileUpload{{Source: content, Destination: remotePath}}, 30*60)
}

// UploadFileFromPath uploads a local file to the Sandbox.
func (fs *FileSystem) UploadFileFromPath(ctx context.Context, localPath, remotePath string) error {
	return fs.UploadFiles(ctx, []FileUpload{{Source: localPath, Destination: remotePath}}, 30*60)
}

// UploadFiles uploads multiple files to the Sandbox.
func (fs *FileSystem) UploadFiles(ctx context.Context, files []FileUpload, timeoutSec int) error {
	for _, f := range files {
		var content []byte
		var err error

		switch source := f.Source.(type) {
		case []byte:
			content = source
		case string:
			// Read file from path
			content, err = os.ReadFile(source)
			if err != nil {
				return err
			}
		}

		file, err := os.CreateTemp("", "daytona-upload-*")
		if err != nil {
			return err
		}
		defer os.Remove(file.Name())

		if _, err := file.Write(content); err != nil {
			file.Close()
			return err
		}
		file.Close()

		// Reopen for reading
		uploadFile, err := os.Open(file.Name())
		if err != nil {
			return err
		}
		defer uploadFile.Close()

		httpResp, err := fs.sandbox.toolboxAPI.UploadFileDeprecated(ctx, fs.sandbox.ID).Path(f.Destination).File(uploadFile).Execute()
		if err != nil {
			return handleAPIError(err, httpResp)
		}
	}

	return nil
}

// fileDownloadError is a custom error for file download failures.
type fileDownloadError struct {
	message string
}

func (e *fileDownloadError) Error() string {
	return e.message
}
