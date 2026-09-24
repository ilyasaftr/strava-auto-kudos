package strava

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ilyasaftr/strava-auto-kudos/internal/kudos"
	"github.com/ilyasaftr/strava-auto-kudos/internal/store"
)

const DefaultBaseURL = "https://www.strava.com"

var (
	ErrSessionExpired = errors.New("strava session cookie expired; copy a fresh _strava4_session from browser cookies")
	csrfPattern       = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)
)

type SessionUpdater func(store.Session) error

type Client struct {
	http      *http.Client
	baseURL   string
	athleteID int64
	feedLimit int
	jar       *CookieJar
	onUpdate  SessionUpdater

	mu   sync.Mutex
	csrf string
}

type Options struct {
	HTTP        *http.Client
	BaseURL     string
	Cookie      string
	Cookies     map[string]string
	ExpiresAt   time.Time
	LastRefresh time.Time
	AthleteID   int64
	FeedLimit   int
	OnUpdate    SessionUpdater
	Now         func() time.Time
}

func New(opts Options) *Client {
	httpClient := opts.HTTP
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 25 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	cookies := map[string]string{}
	for name, value := range opts.Cookies {
		if value != "" {
			cookies[name] = value
		}
	}
	session := strings.TrimPrefix(strings.TrimSpace(opts.Cookie), "_strava4_session=")
	if session != "" {
		cookies["_strava4_session"] = session
	}
	feedLimit := opts.FeedLimit
	if feedLimit <= 0 {
		feedLimit = 20
	}
	return &Client{
		http:      httpClient,
		baseURL:   base,
		athleteID: opts.AthleteID,
		feedLimit: feedLimit,
		jar:       NewCookieJar(cookies, opts.ExpiresAt, opts.LastRefresh, opts.Now),
		onUpdate:  opts.OnUpdate,
	}
}

func (c *Client) Snapshot() store.Session {
	return c.jar.Snapshot()
}

func (c *Client) SetAthleteID(id int64) { c.athleteID = id }

func (c *Client) AthleteID() int64 { return c.athleteID }

func (c *Client) CurrentAthlete(ctx context.Context) (kudos.Athlete, error) {
	body, status, loc, err := c.do(ctx, http.MethodGet, "/frontend/athletes/current", nil, "application/json")
	if err != nil {
		return kudos.Athlete{}, err
	}
	if isLogin(status, loc, string(body)) {
		return kudos.Athlete{}, ErrSessionExpired
	}
	if status >= 300 {
		return kudos.Athlete{}, fmt.Errorf("current athlete status %d: %s", status, truncate(string(body)))
	}
	var payload struct {
		Current struct {
			ID        json.Number `json:"id"`
			IDStr     string      `json:"id_str"`
			FirstName string      `json:"firstname"`
			LastName  string      `json:"lastname"`
		} `json:"currentAthlete"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return kudos.Athlete{}, fmt.Errorf("decode current athlete: %w", err)
	}
	id, err := payload.Current.ID.Int64()
	if err != nil || id == 0 {
		id, err = strconv.ParseInt(payload.Current.IDStr, 10, 64)
	}
	if err != nil || id == 0 {
		return kudos.Athlete{}, fmt.Errorf("current athlete id missing")
	}
	athlete := kudos.Athlete{
		ID:   kudos.AthleteID(id),
		Name: strings.TrimSpace(payload.Current.FirstName + " " + payload.Current.LastName),
	}
	c.athleteID = id
	return athlete, nil
}

func ParseCSRF(html string) (string, error) {
	match := csrfPattern.FindStringSubmatch(html)
	if len(match) != 2 || match[1] == "" {
		return "", fmt.Errorf("csrf-token meta tag not found")
	}
	return match[1], nil
}

func (c *Client) persist(sess store.Session) error {
	if c.onUpdate == nil {
		return nil
	}
	return c.onUpdate(sess)
}

func (c *Client) EnsureSession(ctx context.Context) error {
	if err := c.RefreshIfNeeded(ctx); err != nil {
		return err
	}
	body, status, loc, err := c.do(ctx, http.MethodGet, "/dashboard", nil, "text/html")
	if err != nil {
		return err
	}
	if isLogin(status, loc, string(body)) {
		return ErrSessionExpired
	}
	if status >= 300 {
		return fmt.Errorf("dashboard status %d", status)
	}
	token, err := ParseCSRF(string(body))
	if err != nil {
		return ErrSessionExpired
	}
	c.mu.Lock()
	c.csrf = token
	c.mu.Unlock()
	return nil
}

func (c *Client) FollowingActivities(ctx context.Context) ([]kudos.Activity, error) {
	if err := c.RefreshIfNeeded(ctx); err != nil {
		return nil, err
	}
	if err := c.ensureCSRF(ctx); err != nil {
		return nil, err
	}
	if c.athleteID == 0 {
		if _, err := c.CurrentAthlete(ctx); err != nil {
			return nil, fmt.Errorf("resolve current athlete: %w", err)
		}
	}

	collected := make([]kudos.Activity, 0, c.feedLimit)
	seen := map[kudos.ActivityID]struct{}{}
	var before, cursor string
	for pageNum := 0; pageNum < 10 && len(collected) < c.feedLimit; pageNum++ {
		page, err := c.feedPage(ctx, before, cursor)
		if err != nil {
			return nil, err
		}
		for _, activity := range page.Activities {
			if _, ok := seen[activity.ID]; ok {
				continue
			}
			seen[activity.ID] = struct{}{}
			collected = append(collected, activity)
			if len(collected) >= c.feedLimit {
				break
			}
		}
		if !page.HasMore || page.Cursor == "" || page.Cursor == cursor {
			break
		}
		before, cursor = page.Before, page.Cursor
	}
	return collected, nil
}

func (c *Client) feedPage(ctx context.Context, before, cursor string) (FeedPage, error) {
	query := url.Values{
		"feed_type":  {"following"},
		"athlete_id": {strconv.FormatInt(c.athleteID, 10)},
	}
	if before != "" && cursor != "" {
		query.Set("before", before)
		query.Set("cursor", cursor)
	}
	path := "/dashboard/feed?" + query.Encode()
	body, status, loc, err := c.do(ctx, http.MethodGet, path, nil, "application/json")
	if err != nil {
		return FeedPage{}, err
	}
	if isLogin(status, loc, string(body)) {
		if refreshErr := c.RefreshCookies(ctx); refreshErr != nil {
			return FeedPage{}, ErrSessionExpired
		}
		body, status, loc, err = c.do(ctx, http.MethodGet, path, nil, "application/json")
		if err != nil {
			return FeedPage{}, err
		}
		if isLogin(status, loc, string(body)) {
			return FeedPage{}, ErrSessionExpired
		}
	}
	if status >= 300 {
		return FeedPage{}, fmt.Errorf("feed status %d: %s", status, truncate(string(body)))
	}
	return ParseFeed(body)
}

func (c *Client) GiveKudos(ctx context.Context, activityID kudos.ActivityID) error {
	if err := c.RefreshIfNeeded(ctx); err != nil {
		return err
	}
	if err := c.ensureCSRF(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/feed/activity/%d/kudo", activityID)
	body, status, loc, err := c.do(ctx, http.MethodPost, path, nil, "application/json")
	if err != nil {
		return err
	}
	if isLogin(status, loc, string(body)) {
		return ErrSessionExpired
	}
	if status >= 300 {
		return fmt.Errorf("kudo status %d: %s", status, truncate(string(body)))
	}
	return nil
}

func (c *Client) ensureCSRF(ctx context.Context) error {
	c.mu.Lock()
	ok := c.csrf != ""
	c.mu.Unlock()
	if ok {
		return nil
	}
	return c.EnsureSession(ctx)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, accept string) ([]byte, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Cookie", c.jar.Header())
	req.Header.Set("Accept", accept)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", c.baseURL+"/dashboard")
	c.mu.Lock()
	csrf := c.csrf
	c.mu.Unlock()
	if csrf != "" {
		req.Header.Set("x-csrf-token", csrf)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if updated := c.jar.Merge(res.Cookies()); updated {
		_ = c.persist(c.jar.Snapshot())
	}
	return payload, res.StatusCode, res.Header.Get("Location"), nil
}

func isLogin(status int, location, body string) bool {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return true
	}
	if status >= 300 && status < 400 && strings.Contains(strings.ToLower(location), "login") {
		return true
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, "/login") && strings.Contains(lower, "password")
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
