package commands

import (
	"fmt"
	"net/url"
	"os"
)

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 47831
	DefaultURL  = "http://127.0.0.1:47831"
)

// ResolveURL determines the management API URL from CLI flags and environment.
//
// Precedence:
//  1. flagURL (--url) — wins over everything
//  2. flagHost/flagPort (--host/--port) — either or both; missing half uses default
//  3. PORTIER_URL environment variable
//  4. DefaultURL
func ResolveURL(flagURL, flagHost string, flagPort int) (string, error) {
	if flagURL != "" {
		if err := validateURL(flagURL); err != nil {
			return "", fmt.Errorf("--url: %w", err)
		}
		return flagURL, nil
	}

	if flagHost != "" || flagPort != 0 {
		host := flagHost
		if host == "" {
			host = DefaultHost
		}
		port := flagPort
		if port == 0 {
			port = DefaultPort
		}
		return fmt.Sprintf("http://%s:%d", host, port), nil
	}

	if envURL := os.Getenv("PORTIER_URL"); envURL != "" {
		if err := validateURL(envURL); err != nil {
			return "", fmt.Errorf("PORTIER_URL: %w", err)
		}
		return envURL, nil
	}

	return DefaultURL, nil
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL %q must use http or https scheme", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("URL %q is missing a host", raw)
	}
	return nil
}
