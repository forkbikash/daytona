/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SupportedPythonSeries lists the supported Python version series.
var SupportedPythonSeries = []string{"3.9", "3.10", "3.11", "3.12", "3.13"}

// latestPythonMicroVersions maps Python series to their latest micro versions.
var latestPythonMicroVersions = map[string]string{
	"3.9":  "22",
	"3.10": "17",
	"3.11": "12",
	"3.12": "10",
	"3.13": "3",
}

// Context represents a context file to be added to the image.
type Context struct {
	// SourcePath is the path to the source file or directory.
	SourcePath string
	// ArchivePath is the path inside the archive file in object storage.
	ArchivePath string
}

// PipInstallOptions contains options for the pip install command.
type PipInstallOptions struct {
	// FindLinks are the find-links to use for the pip install command.
	FindLinks []string
	// IndexURL is the index URL to use for the pip install command.
	IndexURL string
	// ExtraIndexURLs are the extra index URLs to use for the pip install command.
	ExtraIndexURLs []string
	// Pre indicates whether to install pre-release versions.
	Pre bool
	// ExtraOptions are extra options to pass to pip install.
	ExtraOptions string
}

// PyprojectOptions contains options for pip install from pyproject.toml.
type PyprojectOptions struct {
	PipInstallOptions
	// OptionalDependencies are the optional dependencies to install.
	OptionalDependencies []string
}

// Image represents an image definition for a Daytona sandbox.
type Image struct {
	dockerfile  string
	contextList []Context
}

// NewImage creates a new empty Image.
func NewImage() *Image {
	return &Image{
		contextList: []Context{},
	}
}

// Dockerfile returns the Dockerfile content.
func (i *Image) Dockerfile() string {
	return i.dockerfile
}

// ContextList returns the list of context files.
func (i *Image) ContextList() []Context {
	return i.contextList
}

// PipInstall adds commands to install packages using pip.
func (i *Image) PipInstall(packages []string, options *PipInstallOptions) *Image {
	if len(packages) == 0 {
		return i
	}

	// Sort packages for reproducibility
	sortedPackages := make([]string, len(packages))
	copy(sortedPackages, packages)
	sort.Strings(sortedPackages)

	extraArgs := i.formatPipInstallArgs(options)
	i.dockerfile += fmt.Sprintf("RUN python -m pip install %s%s\n", strings.Join(sortedPackages, " "), extraArgs)

	return i
}

// RunCommands runs commands in the image.
func (i *Image) RunCommands(commands ...string) *Image {
	for _, command := range commands {
		i.dockerfile += fmt.Sprintf("RUN %s\n", command)
	}
	return i
}

// Env sets environment variables in the image.
func (i *Image) Env(envVars map[string]string) *Image {
	for key, val := range envVars {
		i.dockerfile += fmt.Sprintf("ENV %s=%s\n", key, quoteShell(val))
	}
	return i
}

// Workdir sets the working directory in the image.
func (i *Image) Workdir(dirPath string) *Image {
	i.dockerfile += fmt.Sprintf("WORKDIR %s\n", quoteShell(dirPath))
	return i
}

// Entrypoint sets the entrypoint for the image.
func (i *Image) Entrypoint(commands []string) *Image {
	quotedCommands := make([]string, len(commands))
	for j, cmd := range commands {
		quotedCommands[j] = fmt.Sprintf(`"%s"`, cmd)
	}
	i.dockerfile += fmt.Sprintf("ENTRYPOINT [%s]\n", strings.Join(quotedCommands, ", "))
	return i
}

// Cmd sets the default command for the image.
func (i *Image) Cmd(cmd []string) *Image {
	quotedCmd := make([]string, len(cmd))
	for j, c := range cmd {
		quotedCmd[j] = fmt.Sprintf(`"%s"`, c)
	}
	i.dockerfile += fmt.Sprintf("CMD [%s]\n", strings.Join(quotedCmd, ", "))
	return i
}

// DockerfileCommands extends an image with arbitrary Dockerfile commands.
func (i *Image) DockerfileCommands(commands []string) *Image {
	i.dockerfile += strings.Join(commands, "\n") + "\n"
	return i
}

// AddLocalFile adds a local file to the image.
func (i *Image) AddLocalFile(localPath, remotePath string) *Image {
	// Handle remotePath ending with '/' by appending the filename
	if strings.HasSuffix(remotePath, "/") {
		remotePath = remotePath + filepath.Base(localPath)
	}

	// Expand tilde in path
	expandedPath := expandTilde(localPath)

	i.contextList = append(i.contextList, Context{
		SourcePath:  expandedPath,
		ArchivePath: expandedPath,
	})
	i.dockerfile += fmt.Sprintf("COPY %s %s\n", expandedPath, remotePath)
	return i
}

// AddLocalDir adds a local directory to the image.
func (i *Image) AddLocalDir(localPath, remotePath string) *Image {
	// Expand tilde in path
	expandedPath := expandTilde(localPath)

	i.contextList = append(i.contextList, Context{
		SourcePath:  expandedPath,
		ArchivePath: expandedPath,
	})
	i.dockerfile += fmt.Sprintf("COPY %s %s\n", expandedPath, remotePath)
	return i
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// Base creates an Image from an existing base image.
func Base(image string) *Image {
	img := NewImage()
	img.dockerfile = fmt.Sprintf("FROM %s\n", image)
	return img
}

// DebianSlim creates a Debian slim image based on the official Python Docker image.
func DebianSlim(pythonVersion string) *Image {
	version := processPythonVersion(pythonVersion)
	img := NewImage()

	commands := []string{
		fmt.Sprintf("FROM python:%s-slim-bookworm", version),
		"RUN apt-get update",
		"RUN apt-get install -y gcc gfortran build-essential",
		"RUN pip install --upgrade pip",
		"RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections",
	}

	img.dockerfile = strings.Join(commands, "\n") + "\n"
	return img
}

// formatPipInstallArgs formats pip install arguments.
func (i *Image) formatPipInstallArgs(options *PipInstallOptions) string {
	if options == nil {
		return ""
	}

	var extraArgs strings.Builder

	for _, findLink := range options.FindLinks {
		extraArgs.WriteString(fmt.Sprintf(" --find-links %s", quoteShell(findLink)))
	}

	if options.IndexURL != "" {
		extraArgs.WriteString(fmt.Sprintf(" --index-url %s", quoteShell(options.IndexURL)))
	}

	for _, extraIndexURL := range options.ExtraIndexURLs {
		extraArgs.WriteString(fmt.Sprintf(" --extra-index-url %s", quoteShell(extraIndexURL)))
	}

	if options.Pre {
		extraArgs.WriteString(" --pre")
	}

	if options.ExtraOptions != "" {
		extraArgs.WriteString(" " + strings.TrimSpace(options.ExtraOptions))
	}

	return extraArgs.String()
}

// processPythonVersion processes the Python version and returns the full version string.
func processPythonVersion(pythonVersion string) string {
	if pythonVersion == "" {
		// Default to latest
		pythonVersion = SupportedPythonSeries[len(SupportedPythonSeries)-1]
	}

	// Check if supported
	supported := false
	for _, series := range SupportedPythonSeries {
		if series == pythonVersion {
			supported = true
			break
		}
	}

	if !supported {
		// Default to latest if not supported
		pythonVersion = SupportedPythonSeries[len(SupportedPythonSeries)-1]
	}

	micro := latestPythonMicroVersions[pythonVersion]
	return fmt.Sprintf("%s.%s", pythonVersion, micro)
}

// quoteShell quotes a string for shell use.
func quoteShell(s string) string {
	// Simple quoting - escape single quotes
	if !strings.ContainsAny(s, " \t\n'\"$`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
