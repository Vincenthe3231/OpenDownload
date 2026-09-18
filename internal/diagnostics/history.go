package diagnostics

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxHistoryFileBytes = 1 << 20
	maxHistoryFiles     = 5
	historyRetention    = 7 * 24 * time.Hour
)

// History persists only redacted local diagnostics. It never sends telemetry.
type History struct {
	mu      sync.Mutex
	root    string
	enabled bool
}

func NewHistory() *History {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		root, _ = os.UserConfigDir()
	}
	root = filepath.Join(root, "OpenDownload")
	history := &History{root: filepath.Join(root, "diagnostics")}
	history.enabled = readSetting(filepath.Join(root, "settings.json"))
	return history
}

func (h *History) Enabled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enabled
}

func (h *History) SetEnabled(enabled bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = enabled
	if err := os.MkdirAll(filepath.Dir(h.root), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]bool{"developerDiagnostics": enabled})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Dir(h.root), "settings.json"), data, 0o600)
}

func (h *History) Record(diagnostic Diagnostic) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.enabled {
		diagnostic.TechnicalDetail = ""
	}
	diagnostic = redactedDiagnostic(diagnostic)
	if err := os.MkdirAll(h.root, 0o700); err != nil {
		return err
	}
	if err := h.rotateLocked(); err != nil {
		return err
	}
	data, err := json.Marshal(diagnostic)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(h.root, "diagnostics.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func (h *History) List() ([]Diagnostic, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entries := make([]Diagnostic, 0)
	for index := 0; index < maxHistoryFiles; index++ {
		name := "diagnostics.log"
		if index > 0 {
			name = "diagnostics." + string(rune('0'+index)) + ".log"
		}
		file, err := os.Open(filepath.Join(h.root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var diagnostic Diagnostic
			if json.Unmarshal(scanner.Bytes(), &diagnostic) == nil && time.Since(diagnostic.OccurredAt) <= historyRetention {
				if !h.enabled {
					diagnostic.TechnicalDetail = ""
				}
				entries = append(entries, redactedDiagnostic(diagnostic))
			}
		}
		closeErr := file.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].OccurredAt.Before(entries[j].OccurredAt) })
	return entries, nil
}

func (h *History) Export() (string, error) {
	entries, err := h.List()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	return string(data), err
}

func (h *History) rotateLocked() error {
	path := filepath.Join(h.root, "diagnostics.log")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || info.Size() < maxHistoryFileBytes {
		return err
	}
	for index := maxHistoryFiles - 1; index >= 1; index-- {
		from := filepath.Join(h.root, "diagnostics."+string(rune('0'+index-1))+".log")
		if index == 1 {
			from = path
		}
		to := filepath.Join(h.root, "diagnostics."+string(rune('0'+index))+".log")
		if _, statErr := os.Stat(from); statErr == nil {
			_ = os.Remove(to)
			if err := os.Rename(from, to); err != nil {
				return err
			}
		}
	}
	return nil
}

func redactedDiagnostic(diagnostic Diagnostic) Diagnostic {
	diagnostic.TechnicalDetail = Redact(diagnostic.TechnicalDetail)
	if len(diagnostic.SafeContext) > 0 {
		copyContext := make(map[string]string, len(diagnostic.SafeContext))
		for key, value := range diagnostic.SafeContext {
			copyContext[key] = Redact(value)
		}
		diagnostic.SafeContext = copyContext
	}
	return diagnostic
}

func readSetting(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), `"developerDiagnostics":true`)
}
