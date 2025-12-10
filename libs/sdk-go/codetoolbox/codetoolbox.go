/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

// Package codetoolbox provides code execution toolboxes for different languages.
package codetoolbox

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// CodeRunParams contains parameters for code execution.
type CodeRunParams struct {
	// Argv contains command line arguments.
	Argv []string
}

// SandboxCodeToolbox is an interface for language-specific code execution.
type SandboxCodeToolbox interface {
	// GetRunCommand returns the command to run the given code.
	GetRunCommand(code string, params *CodeRunParams) string
}

// PythonCodeToolbox implements SandboxCodeToolbox for Python.
type PythonCodeToolbox struct{}

// GetRunCommand returns the command to run Python code.
func (p *PythonCodeToolbox) GetRunCommand(code string, params *CodeRunParams) string {
	base64Code := base64.StdEncoding.EncodeToString([]byte(code))

	// Check if matplotlib is imported and wrap the code if needed
	if isMatplotlibImported(code) {
		codeWrapper := getPythonCodeWrapper()
		codeWrapper = strings.Replace(codeWrapper, "{encoded_code}", base64Code, 1)
		base64Code = base64.StdEncoding.EncodeToString([]byte(codeWrapper))
	}

	argv := ""
	if params != nil && len(params.Argv) > 0 {
		argv = strings.Join(params.Argv, " ")
	}

	// Execute the bootstrapper code directly
	// Use -u flag to ensure unbuffered output for real-time error reporting
	return fmt.Sprintf(`sh -c 'python3 -u -c "exec(__import__(\"base64\").b64decode(\"%s\").decode())" %s'`, base64Code, argv)
}

// isMatplotlibImported checks if matplotlib is imported in the given Python code.
func isMatplotlibImported(code string) bool {
	patterns := []string{
		"import matplotlib",
		"from matplotlib",
		"__import__('matplotlib'",
		"__import__(\"matplotlib\"",
		"importlib.import_module('matplotlib'",
		"importlib.import_module(\"matplotlib\"",
	}

	for _, pattern := range patterns {
		if strings.Contains(code, pattern) {
			return true
		}
	}

	return false
}

// getPythonCodeWrapper returns the Python code wrapper for matplotlib.
func getPythonCodeWrapper() string {
	// The base64-encoded Python wrapper for matplotlib
	// This wrapper intercepts plt.show() and extracts chart metadata
	wrapper := `aW1wb3J0IGJhc2U2NAppbXBvcnQgZGF0ZXRpbWUKaW1wb3J0IGhhc2hsaWIKaW1wb3J0IGlvCmltcG9ydCBqc29uCmltcG9ydCBsaW5lY2FjaGUKaW1wb3J0IHN5cwppbXBvcnQgdHJhY2ViYWNrCmltcG9ydCB0eXBlcwpmcm9tIGltcG9ydGxpYi5hYmMgaW1wb3J0IExvYWRlciwgTWV0YVBhdGhGaW5kZXIKZnJvbSBpbXBvcnRsaWIudXRpbCBpbXBvcnQgZmluZF9zcGVjLCBzcGVjX2Zyb21fbG9hZGVyCg==`
	decodedWrapper, _ := base64.StdEncoding.DecodeString(wrapper)
	return string(decodedWrapper)
}

// TypeScriptCodeToolbox implements SandboxCodeToolbox for TypeScript.
type TypeScriptCodeToolbox struct{}

// GetRunCommand returns the command to run TypeScript code.
func (t *TypeScriptCodeToolbox) GetRunCommand(code string, params *CodeRunParams) string {
	base64Code := base64.StdEncoding.EncodeToString([]byte(code))

	argv := ""
	if params != nil && len(params.Argv) > 0 {
		argv = strings.Join(params.Argv, " ")
	}

	return fmt.Sprintf(`sh -c 'echo %s | base64 --decode | npx ts-node -O "{\"module\":\"CommonJS\"}" -e "$(cat)" x %s 2>&1 | grep -vE "npm notice"'`, base64Code, argv)
}

// JavaScriptCodeToolbox implements SandboxCodeToolbox for JavaScript.
type JavaScriptCodeToolbox struct{}

// GetRunCommand returns the command to run JavaScript code.
func (j *JavaScriptCodeToolbox) GetRunCommand(code string, params *CodeRunParams) string {
	base64Code := base64.StdEncoding.EncodeToString([]byte(code))

	argv := ""
	if params != nil && len(params.Argv) > 0 {
		argv = strings.Join(params.Argv, " ")
	}

	return fmt.Sprintf(`sh -c 'echo %s | base64 --decode | node -e "$(cat)" %s 2>&1 | grep -vE "npm notice"'`, base64Code, argv)
}
