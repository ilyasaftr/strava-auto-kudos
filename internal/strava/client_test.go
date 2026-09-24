package strava_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ilyasaftr/strava-auto-kudos/internal/store"
	"github.com/ilyasaftr/strava-auto-kudos/internal/strava"
)

func TestParseCSRF(t *testing.T) {
	html := `<html><head><meta name="csrf-token" content="token-abc"></head></html>`
	got, err := strava.ParseCSRF(html)
	if err != nil {
		t.Fatal(err)
	}
	if got != "token-abc" {
		t.Fatalf("got %q", got)
	}
}

func TestFollowingFeedParsesFriendActivities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard" {
			w.Write([]byte(`<meta name="csrf-token" content="csrf-1">`))
			return
		}
		if r.URL.Path != "/dashboard/feed" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("feed_type") != "following" {
			t.Fatalf("feed_type %s", r.URL.Query().Get("feed_type"))
		}
		if r.Header.Get("Cookie") != "_strava4_session=sess; _currentH=d3d3LnN0cmF2YS5jb20" {
			t.Fatalf("cookie %q", r.Header.Get("Cookie"))
		}
		if r.Header.Get("x-csrf-token") != "csrf-1" {
			t.Fatalf("csrf %q", r.Header.Get("x-csrf-token"))
		}
		io.WriteString(w, `{
		  "entries": [
		    {
		      "entity": "Activity",
		      "viewingAthlete": {"id": "140248321"},
		      "activity": {
		        "id": "11",
		        "activityName": "Outdoor walk",
		        "ownedByCurrentAthlete": false,
		        "athlete": {"athleteId": "99", "athleteName": "Ada"},
		        "kudosAndComments": {"canKudo": true, "hasKudoed": false}
		      }
		    },
		    {
		      "entity": "Activity",
		      "activity": {
		        "id": "22",
		        "activityName": "Mine",
		        "ownedByCurrentAthlete": true,
		        "athlete": {"athleteId": "140248321", "athleteName": "Me"},
		        "kudosAndComments": {"canKudo": false, "hasKudoed": false}
		      }
		    }
		  ],
		  "pagination": {"hasMore": false}
		}`)
	}))
	t.Cleanup(srv.Close)

	c := strava.New(strava.Options{HTTP: srv.Client(), BaseURL: srv.URL, Cookie: "sess", AthleteID: 140248321, LastRefresh: time.Now()})
	activities, err := c.FollowingActivities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 2 {
		t.Fatalf("len %d", len(activities))
	}
	if activities[0].ID != 11 || !activities[0].CanKudo || activities[0].Owned {
		t.Fatalf("friend %+v", activities[0])
	}
	if activities[1].ID != 22 || !activities[1].Owned {
		t.Fatalf("own %+v", activities[1])
	}
}

func TestCurrentAthleteResolvesID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/frontend/athletes/current" {
			t.Fatalf("path %s", r.URL.Path)
		}
		io.WriteString(w, `{"currentAthlete":{"id":140248321,"id_str":"140248321","firstname":"Ilyasa","lastname":"Fathur Rahman"}}`)
	}))
	t.Cleanup(srv.Close)

	c := strava.New(strava.Options{HTTP: srv.Client(), BaseURL: srv.URL, Cookie: "sess", LastRefresh: time.Now()})
	athlete, err := c.CurrentAthlete(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if athlete.ID != 140248321 || athlete.Name != "Ilyasa Fathur Rahman" {
		t.Fatalf("athlete %+v", athlete)
	}
	if c.AthleteID() != 140248321 {
		t.Fatalf("cached %d", c.AthleteID())
	}
}

func TestFollowingFeedResolvesAthleteWhenMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dashboard":
			w.Write([]byte(`<meta name="csrf-token" content="csrf-1">`))
		case "/frontend/athletes/current":
			io.WriteString(w, `{"currentAthlete":{"id":7,"id_str":"7","firstname":"Ada","lastname":"L"}}`)
		case "/dashboard/feed":
			if r.URL.Query().Get("athlete_id") != "7" {
				t.Fatalf("athlete_id %q", r.URL.Query().Get("athlete_id"))
			}
			io.WriteString(w, `{"entries":[],"pagination":{"hasMore":false}}`)
		default:
			t.Fatalf("path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	c := strava.New(strava.Options{HTTP: srv.Client(), BaseURL: srv.URL, Cookie: "sess", LastRefresh: time.Now()})
	activities, err := c.FollowingActivities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 0 || c.AthleteID() != 7 {
		t.Fatalf("activities %+v athlete %d", activities, c.AthleteID())
	}
}

func TestFollowingFeedStopsAtConfiguredLimit(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard" {
			w.Write([]byte(`<meta name="csrf-token" content="csrf-1">`))
			return
		}
		pages = append(pages, r.URL.RawQuery)
		switch r.URL.Query().Get("cursor") {
		case "":
			io.WriteString(w, `{
			  "entries": [{
			    "entity": "Activity",
			    "cursorData": {"updated_at": 100, "rank": 200},
			    "activity": {"id": "1", "activityName": "A", "athlete": {"athleteId": "9", "athleteName": "Ada"}, "kudosAndComments": {"canKudo": true}}
			  },{
			    "entity": "Activity",
			    "cursorData": {"updated_at": 90, "rank": 190},
			    "activity": {"id": "2", "activityName": "B", "athlete": {"athleteId": "9", "athleteName": "Ada"}, "kudosAndComments": {"canKudo": true}}
			  }],
			  "pagination": {"hasMore": true}
			}`)
		case "190":
			io.WriteString(w, `{
			  "entries": [{
			    "entity": "Activity",
			    "cursorData": {"updated_at": 80, "rank": 180},
			    "activity": {"id": "3", "activityName": "C", "athlete": {"athleteId": "9", "athleteName": "Ada"}, "kudosAndComments": {"canKudo": true}}
			  },{
			    "entity": "Activity",
			    "cursorData": {"updated_at": 70, "rank": 170},
			    "activity": {"id": "4", "activityName": "D", "athlete": {"athleteId": "9", "athleteName": "Ada"}, "kudosAndComments": {"canKudo": true}}
			  }],
			  "pagination": {"hasMore": true}
			}`)
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	t.Cleanup(srv.Close)

	c := strava.New(strava.Options{
		HTTP:        srv.Client(),
		BaseURL:     srv.URL,
		Cookie:      "sess",
		AthleteID:   1,
		FeedLimit:   3,
		LastRefresh: time.Now(),
	})
	activities, err := c.FollowingActivities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 3 {
		t.Fatalf("len %d ids %v", len(activities), activities)
	}
	if activities[0].ID != 1 || activities[1].ID != 2 || activities[2].ID != 3 {
		t.Fatalf("ids %+v", activities)
	}
	if len(pages) != 2 {
		t.Fatalf("pages %v", pages)
	}
	if !strings.Contains(pages[1], "before=90") || !strings.Contains(pages[1], "cursor=190") {
		t.Fatalf("second page query %q", pages[1])
	}
}

func TestGiveKudosPostsToFeedPath(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard" {
			w.Write([]byte(`<meta name="csrf-token" content="csrf-1">`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/feed/activity/11/kudo" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-csrf-token") != "csrf-1" {
			t.Fatalf("csrf %q", r.Header.Get("x-csrf-token"))
		}
		posted = true
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"success":true}`)
	}))
	t.Cleanup(srv.Close)

	c := strava.New(strava.Options{HTTP: srv.Client(), BaseURL: srv.URL, Cookie: "sess", AthleteID: 1, LastRefresh: time.Now()})
	if err := c.GiveKudos(context.Background(), 11); err != nil {
		t.Fatal(err)
	}
	if !posted {
		t.Fatal("expected POST")
	}
}

func TestExpiredSessionOnLoginRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://www.strava.com/login")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	c := strava.New(strava.Options{
		HTTP: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		BaseURL:   srv.URL,
		Cookie:    "stale",
		AthleteID: 1,
	})
	err := c.EnsureSession(context.Background())
	if !errors.Is(err, strava.ErrSessionExpired) {
		t.Fatalf("got %v", err)
	}
}

func TestRefreshCookiesPersistsRotatedSession(t *testing.T) {
	var saved store.Session
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/next/refresh-cookies" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Cookie") != "_strava4_session=old-sess; _currentH=d3d3LnN0cmF2YS5jb20" {
			t.Fatalf("cookie %q", r.Header.Get("Cookie"))
		}
		w.Header().Add("Set-Cookie", "_strava4_session=new-sess; Max-Age=7200; Path=/; HttpOnly")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)

	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	c := strava.New(strava.Options{
		HTTP:    srv.Client(),
		BaseURL: srv.URL,
		Cookie:  "old-sess",
		Now:     func() time.Time { return now },
		OnUpdate: func(sess store.Session) error {
			saved = sess
			return nil
		},
	})
	if err := c.RefreshCookies(context.Background()); err != nil {
		t.Fatal(err)
	}
	if saved.SessionCookie() != "new-sess" {
		t.Fatalf("saved %+v", saved)
	}
	if saved.ExpiresAt != now.Add(2*time.Hour).Unix() {
		t.Fatalf("expires %d", saved.ExpiresAt)
	}
	if saved.RefreshedAt != now.Unix() {
		t.Fatalf("refreshed %d", saved.RefreshedAt)
	}
}

func TestRefreshIfNeededPostsWhenNeverRefreshed(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/next/refresh-cookies" {
			t.Fatalf("path %s", r.URL.Path)
		}
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := strava.New(strava.Options{HTTP: srv.Client(), BaseURL: srv.URL, Cookie: "sess"})
	if err := c.RefreshIfNeeded(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits %d", hits)
	}
	if err := c.RefreshIfNeeded(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("expected throttle, hits %d", hits)
	}
}

func TestParseCSRFMissing(t *testing.T) {
	_, err := strava.ParseCSRF("<html>login</html>")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "csrf-token") {
		t.Fatalf("got %v", err)
	}
}
