package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func releaseFeed(tags ...string) string {
	body := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">`
	for _, tag := range tags {
		body += `<entry><link rel="alternate" href="https://github.com/ispacewang/quesora-native/releases/tag/` + tag + `"/></entry>`
	}
	return body + "</feed>"
}
func updateHTTPReply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}
func TestUpdateUsesOfficialFeedWithoutAnonymousAPIBudget(t *testing.T) {
	apiCalls, feedCalls, heads := 0, 0, 0
	c := newUpdateHTTPClient()
	c.Transport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.github.com" {
			apiCalls++
			reply := updateHTTPReply(403, "{}")
			reply.Header.Set("X-RateLimit-Remaining", "0")
			return reply, nil
		}
		if strings.HasSuffix(r.URL.Path, "/releases.atom") {
			feedCalls++
			return updateHTTPReply(200, releaseFeed("v0.1.0-preview.6", "v0.1.0-preview.5")), nil
		}
		if r.Method == http.MethodHead {
			heads++
			reply := updateHTTPReply(200, "")
			reply.ContentLength = 100
			return reply, nil
		}
		return nil, fmt.Errorf("unexpected request")
	})
	candidate, e := fetchUpdate(context.Background(), c, "0.1.0-preview.5")
	if e != nil || candidate == nil || candidate.Version != "0.1.0-preview.6" || apiCalls != 0 || feedCalls != 1 || heads != 2 {
		t.Fatal("API quota still blocks discovery", candidate, e, apiCalls, feedCalls, heads)
	}
}
func TestUpdateFeedUnavailableFallsBackToOfficialAPI(t *testing.T) {
	feeds, apis := 0, 0
	data, _ := json.Marshal([]updateRelease{updateFixture("0.1.0-preview.6")})
	c := newUpdateHTTPClient()
	c.Transport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.github.com" {
			apis++
			return updateHTTPReply(200, string(data)), nil
		}
		feeds++
		return updateHTTPReply(503, "temporary failure"), nil
	})
	candidate, e := fetchUpdate(context.Background(), c, "0.1.0-preview.5")
	if e != nil || candidate == nil || feeds != 1 || apis != 1 {
		t.Fatal(candidate, e, feeds, apis)
	}
}
func TestUpdateFeedSkipsIncompleteUploads(t *testing.T) {
	c := newUpdateHTTPClient()
	c.Transport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/releases.atom") {
			return updateHTTPReply(200, releaseFeed("v0.3.0-preview.1", "v0.2.0", "v0.1.1")), nil
		}
		if r.Method != http.MethodHead {
			return nil, fmt.Errorf("unexpected API request")
		}
		if strings.Contains(r.URL.Path, "/v0.2.0/") || strings.Contains(r.URL.Path, "/v0.3.0-preview.1/") {
			return updateHTTPReply(404, "missing installer"), nil
		}
		reply := updateHTTPReply(200, "")
		reply.ContentLength = 100
		return reply, nil
	})
	candidate, e := fetchUpdate(context.Background(), c, "0.1.0-preview.6")
	if e != nil || candidate == nil || candidate.Version != "0.1.1" {
		t.Fatal(candidate, e)
	}
}
func TestUpdateFeedCannotRedirectDiscoveryToForeignRepository(t *testing.T) {
	foreign := false
	c := newUpdateHTTPClient()
	c.Transport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "github.com" && r.URL.Host != "api.github.com" {
			foreign = true
		}
		if strings.HasSuffix(r.URL.Path, "/releases.atom") {
			return updateHTTPReply(200, strings.ReplaceAll(releaseFeed("v9.0.0"), "github.com/ispacewang/quesora-native", "evil.example/other/repo")), nil
		}
		return updateHTTPReply(500, "failure"), nil
	})
	candidate, e := fetchUpdate(context.Background(), c, "0.1.0")
	if foreign || candidate != nil || e == nil {
		t.Fatal("untrusted feed accepted or failure reported as no update", candidate, e, foreign)
	}
}
func TestUpdateRecentCheckPreservesCandidateWithoutMoreTraffic(t *testing.T) {
	a := newApp()
	queue := make(chan func(), 8)
	a.dispatch = func(f func()) { queue <- f }
	calls := 0
	candidate, _ := selectUpdateRelease([]updateRelease{updateFixture("0.99.0-preview.1")}, appVersion)
	a.updates.fetch = func(context.Context) (*updateCandidate, error) { calls++; return candidate, nil }
	a.checkForUpdate()
	drainUpdate(t, a, queue)
	downloaded := &downloadedUpdate{Path: "not-executed"}
	a.updates.downloaded = downloaded
	a.checkForUpdate()
	drainUpdate(t, a, queue)
	if calls != 1 || a.updates.candidate != candidate || a.updates.downloaded != downloaded {
		t.Fatal("repeated click re-requested or discarded the verified installer", calls, a.updates.message)
	}
	a.updates.lastChecked = time.Now().Add(-2 * time.Minute)
	a.checkForUpdate()
	drainUpdate(t, a, queue)
	if calls != 2 {
		t.Fatal("expired cache prevented refresh", calls)
	}
}
func TestUpdateRetryAfterHTTPDateIsRespected(t *testing.T) {
	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	c := newUpdateHTTPClient()
	c.Transport = updateTestTransport(func(*http.Request) (*http.Response, error) {
		r := updateHTTPReply(429, "")
		r.Header.Set("Retry-After", until.UTC().Format(http.TimeFormat))
		return r, nil
	})
	_, e := updateGET(context.Background(), c, updateAPI)
	rate, ok := e.(*updateRateLimitError)
	if !ok || rate.Until.Before(until) {
		t.Fatal("HTTP-date retry boundary ignored", e)
	}
}

func TestUpdateStableChannelRequiresReleasePrereleaseFlag(t *testing.T) {
	releases := []updateRelease{updateFixture("0.3.0"), updateFixture("0.2.0")}
	releases[0].Prerelease = true
	data, _ := json.Marshal(releases)
	apis := 0
	c := newUpdateHTTPClient()
	c.Transport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.github.com" {
			apis++
			return updateHTTPReply(200, string(data)), nil
		}
		if strings.HasSuffix(r.URL.Path, "/releases.atom") {
			return updateHTTPReply(200, releaseFeed("v0.3.0", "v0.2.0")), nil
		}
		reply := updateHTTPReply(200, "")
		reply.ContentLength = 100
		return reply, nil
	})
	candidate, e := fetchUpdate(context.Background(), c, "0.1.0")
	if e != nil || candidate == nil || candidate.Version != "0.2.0" || apis != 1 {
		t.Fatal("stable channel accepted a release marked preview", candidate, e, apis)
	}
}
