/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package daytona

import (
	"context"

	apiclient "github.com/forkbikash/daytona/libs/api-client-go"
)

// ScreenshotRegion defines coordinates for a screenshot region.
type ScreenshotRegion struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// ScreenshotOptions contains options for taking screenshots.
type ScreenshotOptions struct {
	ShowCursor *bool
	Format     string
	Quality    *float32
	Scale      *float32
}

// Mouse provides mouse operations for computer use functionality.
type Mouse struct {
	sandbox *Sandbox
}

// NewMouse creates a new Mouse instance.
func NewMouse(sandbox *Sandbox) *Mouse {
	return &Mouse{sandbox: sandbox}
}

// GetPosition gets the current mouse cursor position.
func (m *Mouse) GetPosition(ctx context.Context) (*apiclient.MousePosition, error) {
	response, httpResp, err := m.sandbox.toolboxAPI.GetMousePositionDeprecated(ctx, m.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Move moves the mouse cursor to the specified coordinates.
func (m *Mouse) Move(ctx context.Context, x, y float32) (*apiclient.MouseMoveResponse, error) {
	req := apiclient.MouseMoveRequest{X: x, Y: y}
	response, httpResp, err := m.sandbox.toolboxAPI.MoveMouseDeprecated(ctx, m.sandbox.ID).MouseMoveRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Click clicks the mouse at the specified coordinates.
func (m *Mouse) Click(ctx context.Context, x, y float32, button string, double bool) (*apiclient.MouseClickResponse, error) {
	if button == "" {
		button = "left"
	}
	req := apiclient.MouseClickRequest{
		X:      x,
		Y:      y,
		Button: &button,
		Double: &double,
	}
	response, httpResp, err := m.sandbox.toolboxAPI.ClickMouseDeprecated(ctx, m.sandbox.ID).MouseClickRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Drag drags the mouse from start to end coordinates.
func (m *Mouse) Drag(ctx context.Context, startX, startY, endX, endY float32, button string) (*apiclient.MouseDragResponse, error) {
	if button == "" {
		button = "left"
	}
	req := apiclient.MouseDragRequest{
		StartX: startX,
		StartY: startY,
		EndX:   endX,
		EndY:   endY,
		Button: &button,
	}
	response, httpResp, err := m.sandbox.toolboxAPI.DragMouseDeprecated(ctx, m.sandbox.ID).MouseDragRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Scroll scrolls the mouse wheel at the specified coordinates.
func (m *Mouse) Scroll(ctx context.Context, x, y float32, direction string, amount float32) (*apiclient.MouseScrollResponse, error) {
	if amount == 0 {
		amount = 1
	}
	req := apiclient.MouseScrollRequest{
		X:         x,
		Y:         y,
		Direction: direction,
		Amount:    &amount,
	}
	response, httpResp, err := m.sandbox.toolboxAPI.ScrollMouseDeprecated(ctx, m.sandbox.ID).MouseScrollRequest(req).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Keyboard provides keyboard operations for computer use functionality.
type Keyboard struct {
	sandbox *Sandbox
}

// NewKeyboard creates a new Keyboard instance.
func NewKeyboard(sandbox *Sandbox) *Keyboard {
	return &Keyboard{sandbox: sandbox}
}

// Type types the specified text.
func (k *Keyboard) Type(ctx context.Context, text string, delay *float32) error {
	req := apiclient.KeyboardTypeRequest{
		Text:  text,
		Delay: delay,
	}
	httpResp, err := k.sandbox.toolboxAPI.TypeTextDeprecated(ctx, k.sandbox.ID).KeyboardTypeRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Press presses a key with optional modifiers.
func (k *Keyboard) Press(ctx context.Context, key string, modifiers []string) error {
	req := apiclient.KeyboardPressRequest{
		Key:       key,
		Modifiers: modifiers,
	}
	httpResp, err := k.sandbox.toolboxAPI.PressKeyDeprecated(ctx, k.sandbox.ID).KeyboardPressRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Hotkey presses a hotkey combination.
func (k *Keyboard) Hotkey(ctx context.Context, keys string) error {
	req := apiclient.KeyboardHotkeyRequest{
		Keys: keys,
	}
	httpResp, err := k.sandbox.toolboxAPI.PressHotkeyDeprecated(ctx, k.sandbox.ID).KeyboardHotkeyRequest(req).Execute()
	if err != nil {
		return handleAPIError(err, httpResp)
	}
	return nil
}

// Screenshot provides screenshot operations for computer use functionality.
type Screenshot struct {
	sandbox *Sandbox
}

// NewScreenshot creates a new Screenshot instance.
func NewScreenshot(sandbox *Sandbox) *Screenshot {
	return &Screenshot{sandbox: sandbox}
}

// TakeFullScreen takes a screenshot of the entire screen.
func (s *Screenshot) TakeFullScreen(ctx context.Context, showCursor bool) (*apiclient.ScreenshotResponse, error) {
	response, httpResp, err := s.sandbox.toolboxAPI.TakeScreenshotDeprecated(ctx, s.sandbox.ID).ShowCursor(showCursor).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// TakeRegion takes a screenshot of a specific region.
func (s *Screenshot) TakeRegion(ctx context.Context, region ScreenshotRegion, showCursor bool) (*apiclient.RegionScreenshotResponse, error) {
	response, httpResp, err := s.sandbox.toolboxAPI.TakeRegionScreenshotDeprecated(ctx, s.sandbox.ID).Height(region.Height).Width(region.Width).Y(region.Y).X(region.X).ShowCursor(showCursor).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// TakeCompressed takes a compressed screenshot of the entire screen.
func (s *Screenshot) TakeCompressed(ctx context.Context, options *ScreenshotOptions) (*apiclient.CompressedScreenshotResponse, error) {
	req := s.sandbox.toolboxAPI.TakeCompressedScreenshotDeprecated(ctx, s.sandbox.ID)
	if options != nil {
		if options.ShowCursor != nil {
			req = req.ShowCursor(*options.ShowCursor)
		}
		if options.Format != "" {
			req = req.Format(options.Format)
		}
		if options.Quality != nil {
			req = req.Quality(*options.Quality)
		}
		if options.Scale != nil {
			req = req.Scale(*options.Scale)
		}
	}
	response, httpResp, err := req.Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// TakeCompressedRegion takes a compressed screenshot of a specific region.
func (s *Screenshot) TakeCompressedRegion(ctx context.Context, region ScreenshotRegion, options *ScreenshotOptions) (*apiclient.CompressedScreenshotResponse, error) {
	req := s.sandbox.toolboxAPI.TakeCompressedRegionScreenshotDeprecated(ctx, s.sandbox.ID).Height(region.Height).Width(region.Width).X(region.X).Y(region.Y)
	if options != nil {
		if options.ShowCursor != nil {
			req = req.ShowCursor(*options.ShowCursor)
		}
		if options.Format != "" {
			req = req.Format(options.Format)
		}
		if options.Quality != nil {
			req = req.Quality(*options.Quality)
		}
		if options.Scale != nil {
			req = req.Scale(*options.Scale)
		}
	}
	response, httpResp, err := req.Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Display provides display operations for computer use functionality.
type Display struct {
	sandbox *Sandbox
}

// NewDisplay creates a new Display instance.
func NewDisplay(sandbox *Sandbox) *Display {
	return &Display{sandbox: sandbox}
}

// GetInfo gets information about the displays.
func (d *Display) GetInfo(ctx context.Context) (*apiclient.DisplayInfoResponse, error) {
	response, httpResp, err := d.sandbox.toolboxAPI.GetDisplayInfoDeprecated(ctx, d.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// GetWindows gets the list of open windows.
func (d *Display) GetWindows(ctx context.Context) (*apiclient.WindowsResponse, error) {
	response, httpResp, err := d.sandbox.toolboxAPI.GetWindowsDeprecated(ctx, d.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// ComputerUse provides computer use functionality for interacting with the desktop environment.
type ComputerUse struct {
	// Mouse provides mouse operations.
	Mouse *Mouse
	// Keyboard provides keyboard operations.
	Keyboard *Keyboard
	// Screenshot provides screenshot operations.
	Screenshot *Screenshot
	// Display provides display operations.
	Display *Display

	sandbox *Sandbox
}

// NewComputerUse creates a new ComputerUse instance.
func NewComputerUse(sandbox *Sandbox) *ComputerUse {
	return &ComputerUse{
		Mouse:      NewMouse(sandbox),
		Keyboard:   NewKeyboard(sandbox),
		Screenshot: NewScreenshot(sandbox),
		Display:    NewDisplay(sandbox),
		sandbox:    sandbox,
	}
}

// Start starts all computer use processes.
func (cu *ComputerUse) Start(ctx context.Context) (*apiclient.ComputerUseStartResponse, error) {
	response, httpResp, err := cu.sandbox.toolboxAPI.StartComputerUseDeprecated(ctx, cu.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// Stop stops all computer use processes.
func (cu *ComputerUse) Stop(ctx context.Context) (*apiclient.ComputerUseStopResponse, error) {
	response, httpResp, err := cu.sandbox.toolboxAPI.StopComputerUseDeprecated(ctx, cu.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// GetStatus gets the status of all computer use processes.
func (cu *ComputerUse) GetStatus(ctx context.Context) (*apiclient.ComputerUseStatusResponse, error) {
	response, httpResp, err := cu.sandbox.toolboxAPI.GetComputerUseStatusDeprecated(ctx, cu.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// GetProcessStatus gets the status of a specific VNC process.
func (cu *ComputerUse) GetProcessStatus(ctx context.Context, processName string) (*apiclient.ProcessStatusResponse, error) {
	response, httpResp, err := cu.sandbox.toolboxAPI.GetProcessStatusDeprecated(ctx, processName, cu.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// RestartProcess restarts a specific VNC process.
func (cu *ComputerUse) RestartProcess(ctx context.Context, processName string) (*apiclient.ProcessRestartResponse, error) {
	response, httpResp, err := cu.sandbox.toolboxAPI.RestartProcessDeprecated(ctx, processName, cu.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// GetProcessLogs gets logs for a specific VNC process.
func (cu *ComputerUse) GetProcessLogs(ctx context.Context, processName string) (*apiclient.ProcessLogsResponse, error) {
	response, httpResp, err := cu.sandbox.toolboxAPI.GetProcessLogsDeprecated(ctx, processName, cu.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}

// GetProcessErrors gets error logs for a specific VNC process.
func (cu *ComputerUse) GetProcessErrors(ctx context.Context, processName string) (*apiclient.ProcessErrorsResponse, error) {
	response, httpResp, err := cu.sandbox.toolboxAPI.GetProcessErrorsDeprecated(ctx, processName, cu.sandbox.ID).Execute()
	if err != nil {
		return nil, handleAPIError(err, httpResp)
	}
	return response, nil
}
