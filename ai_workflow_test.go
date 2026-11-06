package main

import (
	"context"
	"github.com/egoist/mygo/ui"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func pumpAI(t *testing.T, a *app, updates chan func()) {
	t.Helper()
	deadline := time.After(8 * time.Second)
	for a.ai.Busy || a.ai.Saving {
		select {
		case fn := <-updates:
			fn()
		case <-deadline:
			t.Fatal("AI task timed out")
		}
	}
}
func TestAIGenerationPreviewNeedsExplicitSave(t *testing.T) {
	a := newApp()
	a.ai.Config.Enabled = true
	a.store, _ = openQuestionStore(filepath.Join(t.TempDir(), "data.db"))
	defer a.store.close()
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	updates := make(chan func(), 64)
	a.dispatch = func(fn func()) { updates <- fn }
	p := a.ai.Config.Profiles["deepseek"]
	p.Key = "test-only"
	a.ai.Config.Profiles["deepseek"] = p
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		return aiReply(200, `{"choices":[{"message":{"content":"[{\"question\":\"1+1?\",\"type\":\"单选题\",\"options\":[\"1\",\"2\"],\"answer\":\"B\",\"explanation\":\"相加\"}]"},"finish_reason":"stop"}]}`), nil
	})
	a.ai.Count = "1"
	a.ai.Topic = "算术"
	a.ai.Bank = "预览题库"
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if len(a.ai.Preview) != 1 {
		t.Fatal(a.ai.Message)
	}
	qs, _ := a.store.load()
	if len(qs) != 0 {
		t.Fatal("generated questions saved without confirmation")
	}
	a.saveAIPreview()
	pumpAI(t, a, updates)
	qs, _ = a.store.load()
	if len(qs) != 1 || a.bank != "预览题库" {
		t.Fatal(a.ai.Message, qs)
	}
}
func TestAICancelRejectsLateGeneration(t *testing.T) {
	a := newApp()
	a.ai.Config.Enabled = true
	updates := make(chan func(), 8)
	a.dispatch = func(f func()) { updates <- f }
	started := make(chan struct{})
	done := make(chan struct{})
	a.startAITask("generate", func(ctx context.Context) (aiTaskResult, error) {
		close(started)
		<-ctx.Done()
		close(done)
		return aiTaskResult{Text: "stale"}, nil
	})
	<-started
	a.cancelAITask()
	<-done
	select {
	case fn := <-updates:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("no completion")
	}
	if a.ai.Busy || a.ai.Message == "stale" {
		t.Fatal("cancelled task changed UI")
	}
}
func TestAIUIProvidersAndFooterAccessibleInSmallWindow(t *testing.T) {
	a := newApp()
	a.ai.Config.Enabled = true
	a.openAI("settings")
	tt := ui.NewTester(a.view, 800, 500)
	for _, p := range aiProviders {
		if _, ok := tt.Find("AI服务商：" + p.Name); !ok {
			t.Fatal("missing provider", p.Name)
		}
	}
	for _, label := range []string{"关闭AI助手", "保存AI配置", "测试AI连接"} {
		r, ok := tt.Find(label)
		if !ok || r.Y < 0 || r.Y+r.H > 500 {
			t.Fatal("inaccessible action", label, r)
		}
	}
	a.quick = true
	a.chosen = 0
	tt.Key(0, ui.KeyA)
	if a.chosen != 0 {
		t.Fatal("practice shortcut crossed AI modal")
	}
}

func TestAIAutoGradingDoesNotSilentlyFallBackOnNetworkFailure(t *testing.T) {
	a := newApp()
	a.ai.Config.Enabled = true
	a.ai.Config.AutoJudge = true
	p := a.ai.Config.Profiles["deepseek"]
	p.Key = "test-only"
	a.ai.Config.Profiles["deepseek"] = p
	updates := make(chan func(), 8)
	a.dispatch = func(f func()) { updates <- f }
	a.chosen = 2
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) { return aiReply(401, "private"), nil })
	a.submit()
	pumpAI(t, a, updates)
	if len(a.answers) != 0 {
		t.Fatal("failed AI request saved an offline judgment")
	}
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		return aiReply(200, `{"choices":[{"message":{"content":"{\"correct\":true,\"explanation\":\"符合要点\"}"},"finish_reason":"stop"}]}`), nil
	})
	a.submit()
	pumpAI(t, a, updates)
	if r, ok := a.answers["m1"]; !ok || !r.Correct || r.AIExplanation == "" {
		t.Fatal(a.notice, r)
	}
}

func TestAIConfigWriteCannotBeCancelledOrClosed(t *testing.T) {
	a := newApp()
	a.ai.Open = true
	a.ai.Busy = true
	a.ai.Task = "save"
	a.closeAI()
	if !a.ai.Open || !a.ai.Busy {
		t.Fatal("closing cancelled an atomic settings write")
	}
	a.cancelAITask()
	if !a.ai.Busy {
		t.Fatal("settings write was cancelled")
	}
}
func TestAIReopeningConfigRestoresSavedKey(t *testing.T) {
	a := newApp()
	a.ai.Config.Enabled = true
	p := a.ai.Config.Profiles["deepseek"]
	p.Key = "test-key"
	a.ai.Config.Profiles["deepseek"] = p
	a.closeAI()
	a.openAI("generate")
	tt := ui.NewTester(a.view, 800, 500)
	click(t, tt, "服务商配置")
	if a.ai.Editor.Key != p.Key {
		t.Fatal("saved profile did not populate")
	}
}
func TestAIFeaturesDefaultOffAndNoTrafficWhileDisabled(t *testing.T) {
	a := newApp()
	if a.ai.Config.Enabled {
		t.Fatal("beta must be opt-in")
	}
	calls := 0
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) { calls++; return aiReply(200, "{}"), nil })
	updates := make(chan func(), 8)
	a.dispatch = func(f func()) { updates <- f }
	a.testAIConnection()
	a.generateAIQuestions()
	a.diagnoseAI()
	if calls != 0 || a.ai.Busy {
		t.Fatal("disabled AI started a network job")
	}
	a.ai.Config.AutoJudge = true
	a.chosen = 2
	a.submit()
	if _, ok := a.answers["m1"]; !ok {
		t.Fatal("disabled beta disrupted offline practice")
	}
	tt := ui.NewTester(a.view, 800, 500)
	if _, ok := tt.Find("AI助手"); ok {
		t.Fatal("disabled beta entry visible")
	}
}
func TestAIBetaTogglePersistsAndCancelsPendingRequest(t *testing.T) {
	a := newApp()
	a.ai.Path = filepath.Join(t.TempDir(), "ai-settings.bin")
	updates := make(chan func(), 16)
	a.dispatch = func(f func()) { updates <- f }
	a.toggleAIFeatures()
	pumpAI(t, a, updates)
	if !a.ai.Config.Enabled {
		t.Fatal(a.ai.Message)
	}
	c, e := loadAIConfig(a.ai.Path)
	if e != nil || !c.Enabled {
		t.Fatal("beta enable not persisted", e)
	}
	started := make(chan struct{})
	a.startAITask("generate", func(ctx context.Context) (aiTaskResult, error) {
		close(started)
		<-ctx.Done()
		return aiTaskResult{Text: "late"}, nil
	})
	<-started
	a.toggleAIFeatures()
	pumpAI(t, a, updates)
	if a.ai.Config.Enabled {
		t.Fatal("beta stayed enabled")
	}
	c, e = loadAIConfig(a.ai.Path)
	if e != nil || c.Enabled {
		t.Fatal("beta disable not persisted", e)
	}
}

func TestAINavigatingCancelsPendingAnswer(t *testing.T) {
	a := newApp()
	a.ai.Config.Enabled = true
	updates := make(chan func(), 8)
	a.dispatch = func(f func()) { updates <- f }
	started := make(chan struct{})
	done := make(chan struct{})
	a.startAITask("answer", func(ctx context.Context) (aiTaskResult, error) {
		close(started)
		<-ctx.Done()
		close(done)
		return aiTaskResult{Answer: &aiAnswerResult{Question: a.questions[0], Response: response{Correct: true}}}, nil
	})
	<-started
	a.navigate(1)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("navigation did not cancel")
	}
	select {
	case f := <-updates:
		f()
	case <-time.After(3 * time.Second):
		t.Fatal("missing completion")
	}
	if len(a.answers) != 0 {
		t.Fatal("late answer changed practice")
	}
}
