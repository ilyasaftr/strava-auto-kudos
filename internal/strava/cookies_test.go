package strava_test

import (
	"testing"
	"time"

	"github.com/ilyasaftr/strava-auto-kudos/internal/strava"
)

func TestNeedsRefreshOnStartup(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if !strava.NeedsRefresh(time.Time{}, time.Time{}, now) {
		t.Fatal("expected refresh when never refreshed")
	}
}

func TestNeedsRefreshSkipsWhenRecentlyRefreshed(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	last := now.Add(-2 * time.Minute)
	expires := now.Add(10 * time.Minute)
	if strava.NeedsRefresh(last, expires, now) {
		t.Fatal("expected skip inside min refresh gap")
	}
}

func TestNeedsRefreshBeforeExpiry(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	last := now.Add(-10 * time.Minute)
	expires := now.Add(30 * time.Minute)
	if !strava.NeedsRefresh(last, expires, now) {
		t.Fatal("expected refresh when expiry is within one hour")
	}
}

func TestNeedsRefreshAfterMaxAge(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	last := now.Add(-5 * time.Hour)
	expires := now.Add(48 * time.Hour)
	if !strava.NeedsRefresh(last, expires, now) {
		t.Fatal("expected refresh after 4 hours even if expiry is far")
	}
}

func TestNeedsRefreshHoldsWhenFreshAndFarFromExpiry(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	last := now.Add(-30 * time.Minute)
	expires := now.Add(12 * time.Hour)
	if strava.NeedsRefresh(last, expires, now) {
		t.Fatal("expected no refresh")
	}
}
