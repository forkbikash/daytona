/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"

	apiclient "github.com/daytonaio/apiclient"
)

// GitCommitResponse contains the response from a git commit operation.
type GitCommitResponse struct {
	// SHA is the hash of the commit.
	SHA string
}

// Git provides Git operations within a Sandbox.
type Git struct {
	sandbox *Sandbox
}

// NewGit creates a new Git instance.
func NewGit(sandbox *Sandbox) *Git {
	return &Git{sandbox: sandbox}
}

// Add stages the specified files for the next commit.
func (g *Git) Add(ctx context.Context, path string, files []string) error {
	req := apiclient.GitAddRequest{
		Path:  path,
		Files: files,
	}
	httpResp, err := g.sandbox.toolboxAPI.GitAddFilesDeprecated(ctx, g.sandbox.ID).GitAddRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Branches lists branches in the repository.
func (g *Git) Branches(ctx context.Context, path string) (*apiclient.ListBranchResponse, error) {
	response, httpResp, err := g.sandbox.toolboxAPI.GitListBranchesDeprecated(ctx, g.sandbox.ID).Path(path).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// CreateBranch creates a new branch in the repository.
func (g *Git) CreateBranch(ctx context.Context, path, name string) error {
	req := apiclient.GitBranchRequest{
		Path: path,
		Name: name,
	}
	httpResp, err := g.sandbox.toolboxAPI.GitCreateBranchDeprecated(ctx, g.sandbox.ID).GitBranchRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// DeleteBranch deletes a branch in the repository.
func (g *Git) DeleteBranch(ctx context.Context, path, name string) error {
	req := apiclient.GitDeleteBranchRequest{
		Path: path,
		Name: name,
	}
	httpResp, err := g.sandbox.toolboxAPI.GitDeleteBranchDeprecated(ctx, g.sandbox.ID).GitDeleteBranchRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// CheckoutBranch checks out a branch in the repository.
func (g *Git) CheckoutBranch(ctx context.Context, path, branch string) error {
	req := apiclient.GitCheckoutRequest{
		Path:   path,
		Branch: branch,
	}
	httpResp, err := g.sandbox.toolboxAPI.GitCheckoutBranchDeprecated(ctx, g.sandbox.ID).GitCheckoutRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// CloneOptions contains options for cloning a repository.
type CloneOptions struct {
	// Branch is the specific branch to clone.
	Branch string
	// CommitID is the specific commit to clone.
	CommitID string
	// Username is the git username for authentication.
	Username string
	// Password is the git password or token for authentication.
	Password string
}

// Clone clones a Git repository into the specified path.
func (g *Git) Clone(ctx context.Context, url, path string, opts *CloneOptions) error {
	req := apiclient.GitCloneRequest{
		Url:  url,
		Path: path,
	}

	if opts != nil {
		if opts.Branch != "" {
			req.Branch = &opts.Branch
		}
		if opts.CommitID != "" {
			req.CommitId = &opts.CommitID
		}
		if opts.Username != "" {
			req.Username = &opts.Username
		}
		if opts.Password != "" {
			req.Password = &opts.Password
		}
	}

	httpResp, err := g.sandbox.toolboxAPI.GitCloneRepositoryDeprecated(ctx, g.sandbox.ID).GitCloneRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Commit commits staged changes.
func (g *Git) Commit(ctx context.Context, path, message, author, email string, allowEmpty bool) (*GitCommitResponse, error) {
	req := apiclient.GitCommitRequest{
		Path:       path,
		Message:    message,
		Author:     author,
		Email:      email,
		AllowEmpty: &allowEmpty,
	}
	response, httpResp, err := g.sandbox.toolboxAPI.GitCommitChangesDeprecated(ctx, g.sandbox.ID).GitCommitRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return &GitCommitResponse{SHA: response.Hash}, nil
}

// Push pushes local changes to the remote repository.
func (g *Git) Push(ctx context.Context, path string, username, password string) error {
	req := apiclient.GitRepoRequest{
		Path: path,
	}
	if username != "" {
		req.Username = &username
	}
	if password != "" {
		req.Password = &password
	}
	httpResp, err := g.sandbox.toolboxAPI.GitPushChangesDeprecated(ctx, g.sandbox.ID).GitRepoRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Pull pulls changes from the remote repository.
func (g *Git) Pull(ctx context.Context, path string, username, password string) error {
	req := apiclient.GitRepoRequest{
		Path: path,
	}
	if username != "" {
		req.Username = &username
	}
	if password != "" {
		req.Password = &password
	}
	httpResp, err := g.sandbox.toolboxAPI.GitPullChangesDeprecated(ctx, g.sandbox.ID).GitRepoRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Status gets the current status of the Git repository.
func (g *Git) Status(ctx context.Context, path string) (*apiclient.GitStatus, error) {
	response, httpResp, err := g.sandbox.toolboxAPI.GitGetStatusDeprecated(ctx, g.sandbox.ID).Path(path).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}
