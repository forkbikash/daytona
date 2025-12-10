/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// PtyConnectOptions contains options for connecting to a PTY session.
type PtyConnectOptions struct {
	// OnData is a callback to handle PTY output data.
	OnData func(data []byte)
}

// PtyResult represents the result when a PTY session exits.
type PtyResult struct {
	// ExitCode is the exit code when the PTY process ends.
	ExitCode *int
	// Error is the error message if the PTY failed.
	Error string
}

// PtyHandle provides a handle for managing a PTY session.
type PtyHandle struct {
	ws                    *websocket.Conn
	sessionID             string
	exitCode              *int
	error                 string
	connected             bool
	connectionEstablished bool
	onData                func(data []byte)
	handleResize          func(cols, rows float32) error
	handleKill            func() error
	mu                    sync.RWMutex
	done                  chan struct{}
}

// controlMessage represents a WebSocket control message.
type controlMessage struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// exitData represents exit data from WebSocket close.
type exitData struct {
	ExitCode   *int   `json:"exitCode,omitempty"`
	ExitReason string `json:"exitReason,omitempty"`
	Error      string `json:"error,omitempty"`
}

// NewPtyHandle creates a new PtyHandle instance.
func NewPtyHandle(
	ws *websocket.Conn,
	sessionID string,
	onData func(data []byte),
	handleResize func(cols, rows float32) error,
	handleKill func() error,
) *PtyHandle {
	h := &PtyHandle{
		ws:           ws,
		sessionID:    sessionID,
		onData:       onData,
		handleResize: handleResize,
		handleKill:   handleKill,
		done:         make(chan struct{}),
	}
	go h.readMessages()
	return h
}

// SessionID returns the PTY session ID.
func (h *PtyHandle) SessionID() string {
	return h.sessionID
}

// ExitCode returns the exit code of the PTY process (if terminated).
func (h *PtyHandle) ExitCode() *int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.exitCode
}

// Error returns the error message if the PTY failed.
func (h *PtyHandle) Error() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.error
}

// IsConnected checks if the PTY session is connected.
func (h *PtyHandle) IsConnected() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connected
}

// WaitForConnection waits for the WebSocket connection to be established.
func (h *PtyHandle) WaitForConnection() error {
	h.mu.RLock()
	if h.connectionEstablished {
		h.mu.RUnlock()
		return nil
	}
	h.mu.RUnlock()

	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return errors.New("PTY connection timeout")
		case <-ticker.C:
			h.mu.RLock()
			if h.connectionEstablished {
				h.mu.RUnlock()
				return nil
			}
			if h.error != "" {
				err := h.error
				h.mu.RUnlock()
				return errors.New(err)
			}
			h.mu.RUnlock()
		case <-h.done:
			h.mu.RLock()
			if h.error != "" {
				err := h.error
				h.mu.RUnlock()
				return errors.New(err)
			}
			h.mu.RUnlock()
			return errors.New("connection closed")
		}
	}
}

// SendInput sends input data to the PTY session.
func (h *PtyHandle) SendInput(data string) error {
	if !h.IsConnected() {
		return errors.New("PTY is not connected")
	}

	return h.ws.WriteMessage(websocket.TextMessage, []byte(data))
}

// SendInputBytes sends raw byte input to the PTY session.
func (h *PtyHandle) SendInputBytes(data []byte) error {
	if !h.IsConnected() {
		return errors.New("PTY is not connected")
	}

	return h.ws.WriteMessage(websocket.BinaryMessage, data)
}

// Resize resizes the PTY terminal dimensions.
func (h *PtyHandle) Resize(cols, rows float32) error {
	return h.handleResize(cols, rows)
}

// Disconnect disconnects from the PTY session and cleans up resources.
func (h *PtyHandle) Disconnect() error {
	h.mu.Lock()
	h.connected = false
	h.mu.Unlock()

	if h.ws != nil {
		return h.ws.Close()
	}
	return nil
}

// Wait waits for the PTY process to exit and returns the result.
func (h *PtyHandle) Wait() (*PtyResult, error) {
	h.mu.RLock()
	if h.exitCode != nil {
		result := &PtyResult{
			ExitCode: h.exitCode,
			Error:    h.error,
		}
		h.mu.RUnlock()
		return result, nil
	}
	h.mu.RUnlock()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.mu.RLock()
			if h.exitCode != nil {
				result := &PtyResult{
					ExitCode: h.exitCode,
					Error:    h.error,
				}
				h.mu.RUnlock()
				return result, nil
			}
			h.mu.RUnlock()
		case <-h.done:
			h.mu.RLock()
			result := &PtyResult{
				ExitCode: h.exitCode,
				Error:    h.error,
			}
			h.mu.RUnlock()
			return result, nil
		}
	}
}

// Kill kills the PTY process and terminates the session.
func (h *PtyHandle) Kill() error {
	return h.handleKill()
}

// readMessages reads messages from the WebSocket connection.
func (h *PtyHandle) readMessages() {
	defer close(h.done)

	for {
		messageType, data, err := h.ws.ReadMessage()
		if err != nil {
			h.mu.Lock()
			h.connected = false
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				// Try to parse close reason for exit data
				if closeErr, ok := err.(*websocket.CloseError); ok && closeErr.Text != "" {
					var ed exitData
					if jsonErr := json.Unmarshal([]byte(closeErr.Text), &ed); jsonErr == nil {
						h.exitCode = ed.ExitCode
						if ed.ExitReason != "" {
							h.error = ed.ExitReason
						}
						if ed.Error != "" {
							h.error = ed.Error
						}
					}
				}
				if h.exitCode == nil {
					exitCode := 0
					h.exitCode = &exitCode
				}
			} else {
				h.error = err.Error()
			}
			h.mu.Unlock()
			return
		}

		switch messageType {
		case websocket.TextMessage:
			// Try to parse as control message
			var ctrl controlMessage
			if err := json.Unmarshal(data, &ctrl); err == nil && ctrl.Type == "control" {
				h.mu.Lock()
				if ctrl.Status == "connected" {
					h.connectionEstablished = true
					h.connected = true
				} else if ctrl.Status == "error" {
					h.error = ctrl.Error
					if h.error == "" {
						h.error = "Unknown connection error"
					}
					h.connected = false
				}
				h.mu.Unlock()
				continue
			}

			// Regular PTY text output
			if h.onData != nil {
				h.onData(data)
			}

		case websocket.BinaryMessage:
			// Binary PTY output
			if h.onData != nil {
				h.onData(data)
			}
		}
	}
}

// createPtyWebSocket creates a WebSocket connection for PTY.
func createPtyWebSocket(baseURL, sessionID, apiKey string) (*websocket.Conn, error) {
	// Convert HTTP URL to WebSocket URL
	wsURL := strings.Replace(baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = wsURL + "/process/pty/" + sessionID + "/connect"

	header := http.Header{}
	if apiKey != "" {
		header.Set("Authorization", "Bearer "+apiKey)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
