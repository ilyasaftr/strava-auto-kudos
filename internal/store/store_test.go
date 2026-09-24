package store_test

import (
	"path/filepath"
	"testing"

	"github.com/ilyasaftr/strava-auto-kudos/internal/store"
)

func TestSessionFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	f := store.NewSessionFile(path)
	want := store.Session{
		Cookies: map[string]string{
			"_strava4_session": "abc",
			"_currentH":        "host",
		},
		ExpiresAt:   111,
		RefreshedAt: 100,
	}
	if err := f.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionCookie() != "abc" || got.ExpiresAt != 111 {
		t.Fatalf("got %+v", got)
	}
}
