package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilyasaftr/strava-auto-kudos/internal/config"
	"github.com/ilyasaftr/strava-auto-kudos/internal/kudos"
	"github.com/ilyasaftr/strava-auto-kudos/internal/store"
	"github.com/ilyasaftr/strava-auto-kudos/internal/strava"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || args[0] != "run" {
		return fmt.Errorf("usage: kudos run")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runLoop(cfg)
}

func runLoop(cfg config.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	client, err := newClient(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.EnsureSession(ctx); err != nil {
		return err
	}
	athlete, err := client.CurrentAthlete(ctx)
	if err != nil {
		return err
	}
	slog.Info("session ok", "athlete_id", athlete.ID, "name", athlete.Name, "expires_at", client.Snapshot().ExpiresAt, "feed_limit", cfg.FeedLimit)

	poller := kudos.Poller{
		Feed:      client,
		Giver:     client,
		AthleteID: athlete.ID,
		Delay:     time.Second,
		Logger:    slog.Default(),
	}

	if err := tickOnce(ctx, poller); err != nil {
		return err
	}
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping")
			return nil
		case <-ticker.C:
			if err := tickOnce(ctx, poller); err != nil {
				return err
			}
		}
	}
}

func newClient(cfg config.Config) (*strava.Client, error) {
	sessFile := store.NewSessionFile(cfg.SessionPath())
	saved, err := sessFile.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	opts := strava.Options{
		Cookie:    cfg.SessionCookie,
		FeedLimit: cfg.FeedLimit,
		OnUpdate:  sessFile.Save,
	}
	if saved.SessionCookie() != "" {
		opts.Cookies = saved.Cookies
		opts.Cookie = saved.SessionCookie()
	}
	if saved.ExpiresAt > 0 {
		opts.ExpiresAt = time.Unix(saved.ExpiresAt, 0)
	}
	if saved.RefreshedAt > 0 {
		opts.LastRefresh = time.Unix(saved.RefreshedAt, 0)
	}
	return strava.New(opts), nil
}

func tickOnce(ctx context.Context, poller kudos.Poller) error {
	result, err := poller.Tick(ctx)
	if err != nil {
		if errors.Is(err, strava.ErrSessionExpired) {
			return err
		}
		slog.Error("poll failed", "err", err)
		return nil
	}
	slog.Info("poll complete", "fetched", result.Fetched, "given", result.Given, "skipped", result.Skipped)
	return nil
}
