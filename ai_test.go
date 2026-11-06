package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type aiTestTransport func(*http.Request) (*http.Response, error)

func (f aiTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func aiReply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestAIProvidersSendOnlyToTheirOwnHTTPSOrigin(t *testing.T) {
	for _, p := range aiProviders {
		t.Run(p.ID, func(t *testing.T) {
			profile := aiProfile{BaseURL: p.BaseURL, Model: p.Model, Key: "test-secret"}
			c := newAIHTTPClient()
			c.Transport = aiTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != strings.TrimRight(p.BaseURL, "/")+"/chat/completions" || r.Header.Get("Authorization") != "Bearer test-secret" {
					t.Fatal("wrong endpoint or authentication")
				}
				var body map[string]any
				if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
					t.Fatal(e)
				}
				if body["model"] != p.Model || len(body["messages"].([]any)) != 2 {
					t.Fatal("wrong payload")
				}
				return aiReply(200, `{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`), nil
			})
			out, e := aiChat(context.Background(), c, p.ID, profile, "system", "question", 256)
			if e != nil || out != "answer" {
				t.Fatal(out, e)
			}
			profile.BaseURL = "https://attacker.example/v1"
			if _, e = aiChat(context.Background(), c, p.ID, profile, "s", "u", 256); e == nil {
				t.Fatal("foreign origin accepted")
			}
		})
	}
}
func TestAIRejectsRedirectsAndDoesNotExposeServiceErrors(t *testing.T) {
	p := aiProviders[0]
	profile := aiProfile{BaseURL: p.BaseURL, Model: p.Model, Key: "private-test-value"}
	c := newAIHTTPClient()
	calls := 0
	c.Transport = aiTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		v := aiReply(302, "private-test-value")
		v.Header.Set("Location", "https://example.org/stolen")
		return v, nil
	})
	if _, e := aiChat(context.Background(), c, p.ID, profile, "s", "u", 128); e == nil || strings.Contains(e.Error(), profile.Key) || calls != 1 {
		t.Fatal("redirect or leak", e, calls)
	}
	c.Transport = aiTestTransport(func(r *http.Request) (*http.Response, error) { return aiReply(401, "private-test-value"), nil })
	if _, e := aiChat(context.Background(), c, p.ID, profile, "s", "u", 128); e == nil || strings.Contains(e.Error(), profile.Key) {
		t.Fatal("error leaked key", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := aiChat(ctx, c, p.ID, profile, "s", "u", 128); e == nil {
		t.Fatal("cancel ignored")
	}
}
func TestAIOutputRejectsBadAnswersAndDoesNotGuessGrades(t *testing.T) {
	valid := `[{"question":"2+2?","type":"单选题","options":["A. 3","B. 4"],"answer":"B","explanation":"相加为4"}]`
	qs, e := parseAIQuestions("\x60\x60\x60json\n"+valid+"\n\x60\x60\x60", "AI题库")
	if e != nil || len(qs) != 1 || qs[0].Answer != 2 || qs[0].Options[0] != "3" {
		t.Fatal(qs, e)
	}
	for _, bad := range []string{strings.Replace(valid, `"B","explanation"`, `"Z","explanation"`, 1), strings.Replace(valid, `"B","explanation"`, `"AB","explanation"`, 1), `[{"question":"x","type":"填空题","answer":""}]`} {
		if _, e := parseAIQuestions(bad, "bank"); e == nil {
			t.Fatal("invalid question accepted", bad)
		}
	}
	for _, bad := range []string{`{"correct":"true","explanation":"x"}`, `{"explanation":"正确"}`, "答案正确", `{"correct":true,"explanation":""}`} {
		if _, e := parseAIJudgment(bad); e == nil {
			t.Fatal("grade guessed", bad)
		}
	}
	if g, e := parseAIJudgment(`{"correct":false,"explanation":"缺少要点"}`); e != nil || g.Correct {
		t.Fatal(g, e)
	}
}
func TestAIKeysEncryptedAndConfigReloadable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ai-settings.bin")
	cfg := defaultAIConfig()
	v := cfg.Profiles["deepseek"]
	v.Key = "a-private-test-key"
	cfg.Profiles["deepseek"] = v
	if e := saveAIConfig(p, cfg); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(p)
	if e != nil || strings.Contains(string(data), v.Key) {
		t.Fatal("plaintext saved", e)
	}
	got, e := loadAIConfig(p)
	if e != nil || got.Profiles["deepseek"].Key != v.Key {
		t.Fatal("reload failed", e)
	}
}
