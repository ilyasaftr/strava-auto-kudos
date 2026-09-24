package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SessionCookie string
	PollInterval  time.Duration
	FeedLimit     int
	DataDir       string
}

func Load() (Config, error) {
	_ = loadDotEnv(".env")

	interval := 2 * time.Minute
	if raw := os.Getenv("POLL_INTERVAL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("POLL_INTERVAL: %w", err)
		}
		interval = parsed
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	cookie := strings.TrimSpace(os.Getenv("STRAVA_SESSION_COOKIE"))
	cookie = strings.TrimPrefix(cookie, "_strava4_session=")
	if cookie == "" {
		return Config{}, fmt.Errorf("STRAVA_SESSION_COOKIE is required (_strava4_session from strava.com cookies)")
	}

	feedLimit := 20
	if raw := strings.TrimSpace(os.Getenv("STRAVA_FEED_LIMIT")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("STRAVA_FEED_LIMIT: %w", err)
		}
		if n < 1 {
			return Config{}, fmt.Errorf("STRAVA_FEED_LIMIT must be at least 1")
		}
		if n > 100 {
			n = 100
		}
		feedLimit = n
	}

	return Config{
		SessionCookie: cookie,
		PollInterval:  interval,
		FeedLimit:     feedLimit,
		DataDir:       dataDir,
	}, nil
}

func (c Config) SessionPath() string { return filepath.Join(c.DataDir, "session.json") }

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
