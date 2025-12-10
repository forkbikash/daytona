/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"
	"fmt"

	apiclient "github.com/forkbikash/daytona/libs/api-client-go"
)

// LspLanguageID represents supported language server types.
type LspLanguageID string

const (
	LspLanguagePython     LspLanguageID = "python"
	LspLanguageTypeScript LspLanguageID = "typescript"
	LspLanguageJavaScript LspLanguageID = "javascript"
)

// Position represents a zero-based position within a text document.
type Position struct {
	// Line is the zero-based line number.
	Line int32
	// Character is the zero-based character offset.
	Character int32
}

// LspServer provides Language Server Protocol functionality for code intelligence.
type LspServer struct {
	languageID    LspLanguageID
	pathToProject string
	sandbox       *Sandbox
}

// NewLspServer creates a new LspServer instance.
func NewLspServer(languageID LspLanguageID, pathToProject string, sandbox *Sandbox) *LspServer {
	return &LspServer{
		languageID:    languageID,
		pathToProject: pathToProject,
		sandbox:       sandbox,
	}
}

// Start starts the language server.
func (l *LspServer) Start(ctx context.Context) error {
	req := apiclient.LspServerRequest{
		LanguageId:    string(l.languageID),
		PathToProject: l.pathToProject,
	}
	httpResp, err := l.sandbox.toolboxAPI.LspStartDeprecated(ctx, l.sandbox.ID).LspServerRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Stop stops the language server.
func (l *LspServer) Stop(ctx context.Context) error {
	req := apiclient.LspServerRequest{
		LanguageId:    string(l.languageID),
		PathToProject: l.pathToProject,
	}
	httpResp, err := l.sandbox.toolboxAPI.LspStopDeprecated(ctx, l.sandbox.ID).LspServerRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// DidOpen notifies the language server that a file has been opened.
func (l *LspServer) DidOpen(ctx context.Context, path string) error {
	req := apiclient.LspDocumentRequest{
		LanguageId:    string(l.languageID),
		PathToProject: l.pathToProject,
		Uri:           fmt.Sprintf("file://%s", path),
	}
	httpResp, err := l.sandbox.toolboxAPI.LspDidOpenDeprecated(ctx, l.sandbox.ID).LspDocumentRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// DidClose notifies the language server that a file has been closed.
func (l *LspServer) DidClose(ctx context.Context, path string) error {
	req := apiclient.LspDocumentRequest{
		LanguageId:    string(l.languageID),
		PathToProject: l.pathToProject,
		Uri:           fmt.Sprintf("file://%s", path),
	}
	httpResp, err := l.sandbox.toolboxAPI.LspDidCloseDeprecated(ctx, l.sandbox.ID).LspDocumentRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// DocumentSymbols gets symbol information from a document.
func (l *LspServer) DocumentSymbols(ctx context.Context, path string) ([]apiclient.LspSymbol, error) {
	uri := fmt.Sprintf("file://%s", path)
	response, httpResp, err := l.sandbox.toolboxAPI.LspDocumentSymbolsDeprecated(ctx, l.sandbox.ID).LanguageId(string(l.languageID)).PathToProject(l.pathToProject).Uri(uri).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// SandboxSymbols searches for symbols matching the query string across the entire Sandbox.
func (l *LspServer) SandboxSymbols(ctx context.Context, query string) ([]apiclient.LspSymbol, error) {
	response, httpResp, err := l.sandbox.toolboxAPI.LspWorkspaceSymbolsDeprecated(ctx, l.sandbox.ID).LanguageId(string(l.languageID)).PathToProject(l.pathToProject).Query(query).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// WorkspaceSymbols is deprecated. Use SandboxSymbols instead.
func (l *LspServer) WorkspaceSymbols(ctx context.Context, query string) ([]apiclient.LspSymbol, error) {
	return l.SandboxSymbols(ctx, query)
}

// Completions gets completion suggestions at a position in a file.
func (l *LspServer) Completions(ctx context.Context, path string, position Position) (*apiclient.CompletionList, error) {
	req := apiclient.LspCompletionParams{
		LanguageId:    string(l.languageID),
		PathToProject: l.pathToProject,
		Uri:           fmt.Sprintf("file://%s", path),
		Position: apiclient.Position{
			Line:      float32(position.Line),
			Character: float32(position.Character),
		},
	}
	response, httpResp, err := l.sandbox.toolboxAPI.LspCompletionsDeprecated(ctx, l.sandbox.ID).LspCompletionParams(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}
