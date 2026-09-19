package nativehost

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CommandRunner runs one external command and returns combined output.
// Tests inject a runner so registration checks never touch the real registry.
type CommandRunner func(name string, args ...string) ([]byte, error)

// RegistrationFailure is the safe failure returned by the registration script.
type RegistrationFailure struct {
	Code        string `json:"code"`
	UserMessage string `json:"userMessage"`
	Retryable   bool   `json:"retryable"`
}

// RegistrationStatus describes the installed Gecko native-host wiring.
type RegistrationStatus struct {
	HostExecutableFound             bool                 `json:"hostExecutableFound"`
	ManifestExists                  bool                 `json:"manifestExists"`
	ManifestPathMatchesInstall      bool                 `json:"manifestPathMatchesInstall"`
	ManifestExtensionIDMatches      bool                 `json:"manifestExtensionIdMatches"`
	MozillaRegistryPointsToManifest bool                 `json:"mozillaRegistryPointsToExpectedManifest"`
	Healthy                         bool                 `json:"healthy"`
	Failure                         *RegistrationFailure `json:"failure,omitempty"`
}

// RegistrationRunner invokes the bundled Gecko registration script.
type RegistrationRunner struct {
	installRoot string
	run         CommandRunner
}

// NewRegistrationRunner creates a runner rooted beside the desktop executable.
func NewRegistrationRunner() *RegistrationRunner {
	installRoot := ""
	if executable, err := os.Executable(); err == nil {
		installRoot = filepath.Dir(executable)
	}
	return NewRegistrationRunnerWithOptions(installRoot, runCommand)
}

// NewRegistrationRunnerWithOptions creates a runner with explicit dependencies.
func NewRegistrationRunnerWithOptions(installRoot string, runner CommandRunner) *RegistrationRunner {
	if runner == nil {
		runner = runCommand
	}
	return &RegistrationRunner{installRoot: installRoot, run: runner}
}

// Inspect returns safe registration status from the bundled PowerShell script.
func (r *RegistrationRunner) Inspect() (RegistrationStatus, error) {
	output, err := r.runScript("Inspect")
	if err != nil {
		return RegistrationStatus{}, fmt.Errorf("inspect native-host registration: %w", err)
	}

	var status RegistrationStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return RegistrationStatus{}, fmt.Errorf("parse native-host registration status: %w", err)
	}
	return status, nil
}

// Install repairs registration for the current user.
func (r *RegistrationRunner) Install() error {
	if _, err := r.runScript("Install"); err != nil {
		return fmt.Errorf("install native-host registration: %w", err)
	}
	return nil
}

func (r *RegistrationRunner) runScript(action string) ([]byte, error) {
	if r == nil || r.run == nil {
		return nil, fmt.Errorf("native-host registration runner is unavailable")
	}
	if r.installRoot == "" {
		return nil, fmt.Errorf("native-host installation directory is unavailable")
	}

	installRoot, err := filepath.Abs(r.installRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve native-host installation directory: %w", err)
	}
	scriptPath := filepath.Join(installRoot, "register-native-host.ps1")
	configPath := filepath.Join(installRoot, "browser-ids.json")
	args := []string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-Action", action,
		"-InstallRoot", installRoot,
		"-ConfigPath", configPath,
	}
	return r.run("powershell.exe", args...)
}

func runCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
