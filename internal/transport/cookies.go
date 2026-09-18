package transport

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// LoadCookiesFromBrowser returns unencrypted Chrome or Edge cookies for domain.
// Browser managed encrypted cookie values require platform specific decryption
// and are intentionally not handled here.
func LoadCookiesFromBrowser(browser, domain string) (string, error) {
	cookiePath := browserCookiePath(browser)
	if cookiePath == "" {
		return "", fmt.Errorf("unsupported browser or operating system")
	}

	source, err := os.Open(cookiePath)
	if err != nil {
		return "", fmt.Errorf("open browser cookie database: %w", err)
	}
	defer func() { _ = source.Close() }()

	temporary, err := os.CreateTemp("", "opendownload-cookies-*.sqlite")
	if err != nil {
		return "", fmt.Errorf("create temporary cookie database: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if _, err := io.Copy(temporary, source); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("copy browser cookie database: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close temporary cookie database: %w", err)
	}

	database, err := sql.Open("sqlite3", temporaryPath)
	if err != nil {
		return "", fmt.Errorf("open temporary cookie database: %w", err)
	}
	defer func() { _ = database.Close() }()

	rows, err := database.Query("SELECT name, value FROM cookies WHERE host_key LIKE ?", "%"+domain+"%")
	if err != nil {
		return "", fmt.Errorf("query browser cookies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cookies := make([]string, 0)
	for rows.Next() {
		var name string
		var value string
		if err := rows.Scan(&name, &value); err != nil {
			return "", fmt.Errorf("read browser cookie: %w", err)
		}
		cookies = append(cookies, name+"="+value)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate browser cookies: %w", err)
	}
	return strings.Join(cookies, "; "), nil
}

func browserCookiePath(browser string) string {
	if runtime.GOOS != "windows" {
		return ""
	}

	applicationData := os.Getenv("LOCALAPPDATA")
	switch strings.ToLower(browser) {
	case "chrome":
		return filepath.Join(applicationData, "Google", "Chrome", "User Data", "Default", "Cookies")
	case "edge":
		return filepath.Join(applicationData, "Microsoft", "Edge", "User Data", "Default", "Cookies")
	default:
		return ""
	}
}

func loadNetscapeCookieJar(path string) (http.CookieJar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cookie file: %w", err)
	}
	defer func() { _ = file.Close() }()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || (strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#HttpOnly_")) {
			continue
		}
		if err := addNetscapeCookie(jar, line); err != nil {
			return nil, fmt.Errorf("parse cookie file: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read cookie file: %w", err)
	}
	return jar, nil
}

func addNetscapeCookie(jar http.CookieJar, line string) error {
	fields := strings.Split(line, "\t")
	if len(fields) != 7 {
		return fmt.Errorf("expected 7 fields, got %d", len(fields))
	}

	domain := strings.TrimPrefix(fields[0], "#HttpOnly_")
	domain = strings.TrimPrefix(domain, ".")
	if domain == "" {
		return fmt.Errorf("cookie domain is empty")
	}
	secure := strings.EqualFold(fields[3], "TRUE")
	expires, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return fmt.Errorf("parse expiry %q: %w", fields[4], err)
	}
	if expires > 0 && time.Unix(expires, 0).Before(time.Now()) {
		return nil
	}

	scheme := "http"
	if secure {
		scheme = "https"
	}
	cookieURL := &url.URL{Scheme: scheme, Host: domain, Path: "/"}
	cookiePath := fields[2]
	if cookiePath == "" {
		cookiePath = "/"
	}
	cookie := &http.Cookie{
		Name:   fields[5],
		Value:  fields[6],
		Path:   cookiePath,
		Domain: domain,
		Secure: secure,
	}
	if expires > 0 {
		cookie.Expires = time.Unix(expires, 0)
	}
	jar.SetCookies(cookieURL, []*http.Cookie{cookie})
	return nil
}
