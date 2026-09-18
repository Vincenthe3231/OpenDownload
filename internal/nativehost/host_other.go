//go:build !windows

package nativehost

import "errors"

var ErrWindowsOnly = errors.New("native host is supported on Windows only")

func Run() error { return ErrWindowsOnly }
