//go:build !windows

package capture

import (
	"context"
	"errors"
)

var ErrNativeCaptureWindowsOnly = errors.New("automatic native capture is supported on Windows only")

func (m *Manager) StartNativeServer(context.Context) (func(), error) {
	return nil, ErrNativeCaptureWindowsOnly
}
