/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	sdkerrors "github.com/forkbikash/daytona/libs/sdk-go/errors"
)

// ObjectStorageConfig holds configuration for the object storage client.
type ObjectStorageConfig struct {
	EndpointURL     string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	BucketName      string
}

// ObjectStorage provides methods for uploading files to S3-compatible storage.
type ObjectStorage struct {
	bucketName string
	s3Client   *s3.Client
}

// NewObjectStorage creates a new ObjectStorage instance.
func NewObjectStorage(ctx context.Context, cfg ObjectStorageConfig) (*ObjectStorage, error) {
	bucketName := cfg.BucketName
	if bucketName == "" {
		bucketName = "daytona-volume-builds"
	}

	region := extractAwsRegion(cfg.EndpointURL)
	if region == "" {
		region = "us-east-1"
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)),
	)
	if err != nil {
		return nil, sdkerrors.NewDaytonaError("failed to load AWS config: "+err.Error(), 0, nil)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.EndpointURL)
		o.UsePathStyle = true
	})

	return &ObjectStorage{
		bucketName: bucketName,
		s3Client:   s3Client,
	}, nil
}

// Upload uploads a file or directory to object storage and returns the content hash.
func (o *ObjectStorage) Upload(ctx context.Context, path, organizationID, archiveBasePath string) (string, error) {
	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", sdkerrors.NewDaytonaError("path does not exist: "+path, 0, nil)
	}

	// Compute hash for the path
	pathHash, err := o.computeHashForPathMD5(path, archiveBasePath)
	if err != nil {
		return "", err
	}

	// Define the S3 prefix and key
	prefix := organizationID + "/" + pathHash + "/"
	s3Key := prefix + "context.tar"

	// Check if it already exists in S3
	exists, err := o.folderExistsInS3(ctx, prefix)
	if err != nil {
		return "", err
	}
	if exists {
		return pathHash, nil
	}

	// Upload to S3
	if err := o.uploadAsTar(ctx, s3Key, path, archiveBasePath); err != nil {
		return "", err
	}

	return pathHash, nil
}

// computeHashForPathMD5 computes an MD5 hash for a file or directory.
func (o *ObjectStorage) computeHashForPathMD5(pathStr, archiveBasePath string) (string, error) {
	hasher := md5.New()
	absPathStr, err := filepath.Abs(pathStr)
	if err != nil {
		return "", sdkerrors.NewDaytonaError("failed to get absolute path: "+err.Error(), 0, nil)
	}

	// Include archiveBasePath in hash
	hasher.Write([]byte(archiveBasePath))

	info, err := os.Stat(absPathStr)
	if err != nil {
		return "", sdkerrors.NewDaytonaError("failed to stat path: "+err.Error(), 0, nil)
	}

	if info.IsDir() {
		if err := o.hashDirectory(absPathStr, pathStr, hasher); err != nil {
			return "", err
		}
	} else {
		if err := o.hashFile(absPathStr, hasher); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// hashDirectory recursively hashes a directory and its contents.
func (o *ObjectStorage) hashDirectory(dirPath, basePath string, hasher io.Writer) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return sdkerrors.NewDaytonaError("failed to read directory: "+err.Error(), 0, nil)
	}

	hasSubdirs := false
	hasFiles := false
	for _, entry := range entries {
		if entry.IsDir() {
			hasSubdirs = true
		} else if entry.Type().IsRegular() {
			hasFiles = true
		}
	}

	if !hasSubdirs && !hasFiles {
		// Empty directory
		relDir, err := filepath.Rel(basePath, dirPath)
		if err != nil {
			return sdkerrors.NewDaytonaError("failed to get relative path: "+err.Error(), 0, nil)
		}
		hasher.Write([]byte(relDir))
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			if err := o.hashDirectory(fullPath, basePath, hasher); err != nil {
				return err
			}
		} else if entry.Type().IsRegular() {
			relPath, err := filepath.Rel(basePath, fullPath)
			if err != nil {
				return sdkerrors.NewDaytonaError("failed to get relative path: "+err.Error(), 0, nil)
			}
			hasher.Write([]byte(relPath))

			if err := o.hashFile(fullPath, hasher); err != nil {
				return err
			}
		}
	}

	return nil
}

// hashFile hashes a file's contents.
func (o *ObjectStorage) hashFile(filePath string, hasher io.Writer) error {
	f, err := os.Open(filePath)
	if err != nil {
		return sdkerrors.NewDaytonaError("failed to open file: "+err.Error(), 0, nil)
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return sdkerrors.NewDaytonaError("failed to hash file: "+err.Error(), 0, nil)
	}

	return nil
}

// folderExistsInS3 checks if a prefix (folder) exists in S3.
func (o *ObjectStorage) folderExistsInS3(ctx context.Context, prefix string) (bool, error) {
	resp, err := o.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(o.bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, sdkerrors.NewDaytonaError("failed to list S3 objects: "+err.Error(), 0, nil)
	}

	return len(resp.Contents) > 0, nil
}

// uploadAsTar creates a tar archive and uploads it to S3.
func (o *ObjectStorage) uploadAsTar(ctx context.Context, s3Key, sourcePath, archiveBasePath string) error {
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return sdkerrors.NewDaytonaError("failed to get absolute path: "+err.Error(), 0, nil)
	}

	normalizedSourcePath := filepath.Clean(sourcePath)
	normalizedArchiveBasePath := filepath.Clean(archiveBasePath)

	// Determine the base prefix for creating relative paths in the tar
	// The goal is to create a tar where files are at their archiveBasePath location
	var basePrefix string
	if normalizedArchiveBasePath == "." {
		// When archiveBasePath is empty, use root so file keeps its full path
		basePrefix = "/"
	} else if normalizedSourcePath == normalizedArchiveBasePath {
		// When source path equals archive base path (e.g., AddLocalFile with absolute path),
		// use root so the file is at its full absolute path in the tar
		basePrefix = "/"
	} else if strings.HasSuffix(normalizedSourcePath, normalizedArchiveBasePath) {
		// Normal case: extract the base prefix by removing archiveBasePath from the end
		basePrefix = normalizedSourcePath[:len(normalizedSourcePath)-len(normalizedArchiveBasePath)]
		if basePrefix == "" {
			basePrefix = "/"
		}
	} else {
		// Fallback: use root
		basePrefix = "/"
	}

	// Create tar archive in memory
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err = filepath.Walk(normalizedSourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get path relative to basePrefix
		var relPath string
		if basePrefix == "/" {
			// Remove leading slash to make it a valid tar path
			relPath = strings.TrimPrefix(path, "/")
		} else {
			relPath, err = filepath.Rel(basePrefix, path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return sdkerrors.NewDaytonaError("failed to create tar archive: "+err.Error(), 0, nil)
	}

	if err := tw.Close(); err != nil {
		return sdkerrors.NewDaytonaError("failed to close tar writer: "+err.Error(), 0, nil)
	}

	// Upload to S3
	_, err = o.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(o.bucketName),
		Key:    aws.String(s3Key),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		return sdkerrors.NewDaytonaError("failed to upload to S3: "+err.Error(), 0, nil)
	}

	return nil
}

// extractAwsRegion extracts the AWS region from an S3 endpoint URL.
func extractAwsRegion(endpoint string) string {
	// Match patterns like s3.us-east-1.amazonaws.com or s3-us-east-1.amazonaws.com
	re := regexp.MustCompile(`s3[.-]([a-z0-9-]+)\.amazonaws\.com`)
	matches := re.FindStringSubmatch(endpoint)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
