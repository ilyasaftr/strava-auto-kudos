package strava

import (
	"context"
	"net/http"
	"time"
)

const (
	minRefreshGap = 4 * time.Minute
	refreshLead   = time.Hour
	maxRefreshAge = 4 * time.Hour
)

func NeedsRefresh(lastRefresh, expiresAt, now time.Time) bool {
	if lastRefresh.IsZero() {
		return true
	}
	if now.Sub(lastRefresh) < minRefreshGap {
		return false
	}
	if !expiresAt.IsZero() && !now.Before(expiresAt.Add(-refreshLead)) {
		return true
	}
	return now.Sub(lastRefresh) >= maxRefreshAge
}

func (c *Client) RefreshIfNeeded(ctx context.Context) error {
	if !c.jar.NeedsRefresh() {
		return nil
	}
	return c.RefreshCookies(ctx)
}

func (c *Client) RefreshCookies(ctx context.Context) error {
	body, status, loc, err := c.do(ctx, http.MethodPost, "/api/next/refresh-cookies", nil, "application/json")
	if err != nil {
		return err
	}
	if isLogin(status, loc, string(body)) || status >= 300 {
		return ErrSessionExpired
	}
	c.jar.MarkRefreshed()
	return c.persist(c.jar.Snapshot())
}
