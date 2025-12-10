/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

// Package errors provides error types for the Daytona SDK.
package errors

import (
	"fmt"
	"net/http"
)

// DaytonaError is the base error type for Daytona SDK.
type DaytonaError struct {
	Message    string
	StatusCode int
	Headers    http.Header
}

func (e *DaytonaError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s (status code: %d)", e.Message, e.StatusCode)
	}
	return e.Message
}

// NewDaytonaError creates a new DaytonaError.
func NewDaytonaError(message string, statusCode int, headers http.Header) *DaytonaError {
	return &DaytonaError{
		Message:    message,
		StatusCode: statusCode,
		Headers:    headers,
	}
}

// DaytonaNotFoundError is returned when a resource is not found.
type DaytonaNotFoundError struct {
	*DaytonaError
}

// NewDaytonaNotFoundError creates a new DaytonaNotFoundError.
func NewDaytonaNotFoundError(message string, statusCode int, headers http.Header) *DaytonaNotFoundError {
	return &DaytonaNotFoundError{
		DaytonaError: NewDaytonaError(message, statusCode, headers),
	}
}

// DaytonaRateLimitError is returned when the rate limit is exceeded.
type DaytonaRateLimitError struct {
	*DaytonaError
}

// NewDaytonaRateLimitError creates a new DaytonaRateLimitError.
func NewDaytonaRateLimitError(message string, statusCode int, headers http.Header) *DaytonaRateLimitError {
	return &DaytonaRateLimitError{
		DaytonaError: NewDaytonaError(message, statusCode, headers),
	}
}

// DaytonaTimeoutError is returned when an operation times out.
type DaytonaTimeoutError struct {
	*DaytonaError
}

// NewDaytonaTimeoutError creates a new DaytonaTimeoutError.
func NewDaytonaTimeoutError(message string) *DaytonaTimeoutError {
	return &DaytonaTimeoutError{
		DaytonaError: NewDaytonaError(message, 0, nil),
	}
}

// IsDaytonaError checks if an error is a DaytonaError.
func IsDaytonaError(err error) bool {
	_, ok := err.(*DaytonaError)
	return ok
}

// IsDaytonaNotFoundError checks if an error is a DaytonaNotFoundError.
func IsDaytonaNotFoundError(err error) bool {
	_, ok := err.(*DaytonaNotFoundError)
	return ok
}

// IsDaytonaRateLimitError checks if an error is a DaytonaRateLimitError.
func IsDaytonaRateLimitError(err error) bool {
	_, ok := err.(*DaytonaRateLimitError)
	return ok
}

// IsDaytonaTimeoutError checks if an error is a DaytonaTimeoutError.
func IsDaytonaTimeoutError(err error) bool {
	_, ok := err.(*DaytonaTimeoutError)
	return ok
}
