package downloader

import (
	"encoding/json"
	"os"
	"time"
)

type DownloadState struct {
	URL        string    `json:"url"`
	OutputPath string    `json:"output_path"`
	TotalSize  int64     `json:"total_size"`
	Downloaded int64     `json:"downloaded"`
	Status     string    `json:"status"` // "pending", "downloading", "completed", "failed"
	UpdatedAt  time.Time `json:"updated_at"`
}

func GetStatePath(outPath string) string {
	return outPath + ".state"
}

func SaveState(outPath string, state *DownloadState) error {
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetStatePath(outPath), data, 0644)
}

func LoadState(outPath string) (*DownloadState, error) {
	data, err := os.ReadFile(GetStatePath(outPath))
	if err != nil {
		return nil, err
	}
	var state DownloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
