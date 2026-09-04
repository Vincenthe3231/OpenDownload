package util

import (
	"net/url"
)

func GetHostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
//... (existing functions)
