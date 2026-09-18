package util

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/mattn/go-sqlite3"
)

// Helper to get default cookie path based on OS and Browser
func getCookiePath(browser string) string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("LOCALAPPDATA")
		switch browser {
		case "chrome":
			return filepath.Join(appData, "Google\\Chrome\\User Data\\Default\\Cookies")
		case "edge":
			return filepath.Join(appData, "Microsoft\\Edge\\User Data\\Default\\Cookies")
		}
	}
	return ""
}

func LoadCookiesFromBrowser(browser string, domain string) (string, error) {
	path := getCookiePath(browser)
	if path == "" {
		return "", fmt.Errorf("unsupported browser or OS")
	}

	// Browsers lock the cookie file, copy it first
	tmpFile := filepath.Join(os.TempDir(), "opendownload_cookies.sqlite")
	if _, err := os.Stat(path); err != nil {
		return "", err
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(tmpFile, bytes, 0644)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpFile) }()

	db, err := sql.Open("sqlite3", tmpFile)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()

	// Query cookies (simplified - depends on platform specific encryption handling!)
	// NOTE: Real decryption requires OS-specific APIs (DPAPI on Windows, Keychain on macOS)
	// For MVP, if cookies are unencrypted, this works
	query := "SELECT name, value FROM cookies WHERE host_key LIKE ?"
	rows, err := db.Query(query, "%"+domain+"%")
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()

	var cookieStr string
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return "", err
		}
		cookieStr += fmt.Sprintf("%s=%s; ", name, value)
	}

	return cookieStr, nil
}
