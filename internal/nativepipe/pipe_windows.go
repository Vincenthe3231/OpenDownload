//go:build windows

package nativepipe

import (
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func nameAndSID() (string, string, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", "", err
	}
	sid := user.User.Sid.String()
	return `\\.\pipe\OpenDownload-` + sid, sid, nil
}

// Name returns the per-user app pipe name.
func Name() (string, error) {
	name, _, err := nameAndSID()
	return name, err
}

// Listen creates a pipe ACL that grants access only to the current user.
func Listen() (net.Listener, error) {
	name, sid, err := nameAndSID()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(name, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + sid + ")",
		MessageMode:        false,
		InputBufferSize:    1 << 20,
		OutputBufferSize:   1 << 20,
	})
}

// Dial connects to the current user's app pipe.
func Dial() (net.Conn, error) {
	name, err := Name()
	if err != nil {
		return nil, err
	}
	return winio.DialPipe(name, nil)
}
