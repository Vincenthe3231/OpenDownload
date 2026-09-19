package nativehost

import (
	"errors"
	"strings"
	"testing"
)

func TestRegistrationRunnerInspectUsesBundledScript(t *testing.T) {
	var commandName string
	var commandArgs []string
	runner := NewRegistrationRunnerWithOptions(`C:\OpenDownload`, func(name string, args ...string) ([]byte, error) {
		commandName = name
		commandArgs = append([]string(nil), args...)
		return []byte(`{"hostExecutableFound":true,"manifestExists":true,"healthy":true}`), nil
	})

	status, err := runner.Inspect()
	if err != nil {
		t.Fatalf("inspect returned error: %v", err)
	}
	if !status.Healthy || !status.HostExecutableFound {
		t.Fatalf("unexpected status: %+v", status)
	}
	if commandName != "powershell.exe" {
		t.Fatalf("command name = %q, want powershell.exe", commandName)
	}
	joined := strings.Join(commandArgs, "\x00")
	for _, expected := range []string{"-Action", "Inspect", "-InstallRoot", "-ConfigPath", "register-native-host.ps1", "browser-ids.json"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("command arguments do not contain %q: %v", expected, commandArgs)
		}
	}
}

func TestRegistrationRunnerInstallUsesInstallAction(t *testing.T) {
	var commandArgs []string
	runner := NewRegistrationRunnerWithOptions(`C:\OpenDownload`, func(_ string, args ...string) ([]byte, error) {
		commandArgs = append([]string(nil), args...)
		return []byte(`{"healthy":true}`), nil
	})

	if err := runner.Install(); err != nil {
		t.Fatalf("install returned error: %v", err)
	}
	joined := strings.Join(commandArgs, "\x00")
	if !strings.Contains(joined, "-Action") || !strings.Contains(joined, "Install") {
		t.Fatalf("install action missing from command arguments: %v", commandArgs)
	}
}

func TestRegistrationRunnerPropagatesCommandFailure(t *testing.T) {
	runner := NewRegistrationRunnerWithOptions(`C:\OpenDownload`, func(_ string, _ ...string) ([]byte, error) {
		return []byte("registration failed"), errors.New("exit status 1")
	})

	if _, err := runner.Inspect(); err == nil {
		t.Fatal("inspect succeeded despite command failure")
	}
	if err := runner.Install(); err == nil {
		t.Fatal("install succeeded despite command failure")
	}
}
