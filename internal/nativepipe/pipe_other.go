//go:build !windows

package nativepipe

import (
	"errors"
	"net"
)

var ErrWindowsOnly = errors.New("native capture pipe is supported on Windows only")

func Name() (string, error)         { return "", ErrWindowsOnly }
func Listen() (net.Listener, error) { return nil, ErrWindowsOnly }
func Dial() (net.Conn, error)       { return nil, ErrWindowsOnly }
