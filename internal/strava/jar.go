package strava

import (
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ilyasaftr/strava-auto-kudos/internal/store"
)

// defaultCurrentH is the static _currentH host marker Strava sets
// (base64 of "www.strava.com"). It is not session auth, so the client
// sends it by default unless the server rotates it.
const defaultCurrentH = "d3d3LnN0cmF2YS5jb20"

type CookieJar struct {
	mu          sync.Mutex
	cookies     map[string]string
	expiresAt   time.Time
	lastRefresh time.Time
	now         func() time.Time
}

func NewCookieJar(cookies map[string]string, expiresAt, lastRefresh time.Time, now func() time.Time) *CookieJar {
	copied := map[string]string{}
	for name, value := range cookies {
		if value != "" {
			copied[name] = value
		}
	}
	if now == nil {
		now = time.Now
	}
	return &CookieJar{
		cookies:     copied,
		expiresAt:   expiresAt,
		lastRefresh: lastRefresh,
		now:         now,
	}
}

func (j *CookieJar) Header() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	cookies := maps.Clone(j.cookies)
	if cookies == nil {
		cookies = map[string]string{}
	}
	if _, ok := cookies["_currentH"]; !ok {
		cookies["_currentH"] = defaultCurrentH
	}
	return encodeCookies(cookies)
}

func (j *CookieJar) NeedsRefresh() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return NeedsRefresh(j.lastRefresh, j.expiresAt, j.now())
}

func (j *CookieJar) MarkRefreshed() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lastRefresh = j.now()
	if j.expiresAt.IsZero() {
		j.expiresAt = j.now().Add(6 * time.Hour)
	}
}

func (j *CookieJar) Merge(setCookies []*http.Cookie) bool {
	if len(setCookies) == 0 {
		return false
	}
	now := j.now()
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cookies == nil {
		j.cookies = map[string]string{}
	}
	updated := false
	for _, cookie := range setCookies {
		if cookie.Name == "" || cookie.Value == "" || cookie.MaxAge < 0 {
			continue
		}
		if j.cookies[cookie.Name] != cookie.Value {
			j.cookies[cookie.Name] = cookie.Value
			updated = true
		}
		if cookie.Name == "_strava4_session" {
			if exp := cookieExpiry(cookie, now); !exp.IsZero() {
				j.expiresAt = exp
			}
		}
	}
	return updated
}

func (j *CookieJar) Snapshot() store.Session {
	j.mu.Lock()
	defer j.mu.Unlock()
	sess := store.Session{Cookies: maps.Clone(j.cookies)}
	if !j.expiresAt.IsZero() {
		sess.ExpiresAt = j.expiresAt.Unix()
	}
	if !j.lastRefresh.IsZero() {
		sess.RefreshedAt = j.lastRefresh.Unix()
	}
	return sess
}

func encodeCookies(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cookies))
	if v := cookies["_strava4_session"]; v != "" {
		parts = append(parts, "_strava4_session="+v)
	}
	if v := cookies["_currentH"]; v != "" {
		parts = append(parts, "_currentH="+v)
	}
	for name, value := range cookies {
		if name == "_strava4_session" || name == "_currentH" || value == "" {
			continue
		}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func cookieExpiry(cookie *http.Cookie, now time.Time) time.Time {
	if cookie == nil {
		return time.Time{}
	}
	if cookie.MaxAge > 0 {
		return now.Add(time.Duration(cookie.MaxAge) * time.Second)
	}
	if !cookie.Expires.IsZero() {
		return cookie.Expires.UTC()
	}
	return time.Time{}
}
