package main

import (
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo/ui"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type generationTestQuestion struct {
	Question    string   `json:"question"`
	Type        string   `json:"type"`
	Options     []string `json:"options"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
}

func generationQuestion(n int) generationTestQuestion {
	return generationTestQuestion{Question: fmt.Sprintf("知识点 %d 的结果？", n), Type: "单选题", Options: []string{"1", "2"}, Answer: "B", Explanation: "测试解析"}
}
func generationReply(rows []generationTestQuestion) *http.Response {
	content, _ := json.Marshal(rows)
	body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}, "finish_reason": "stop"}}})
	return aiReply(200, string(body))
}
func generationTestApp(t *testing.T, count int) (*app, chan func()) {
	t.Helper()
	a := newApp()
	a.ai.Config.Enabled = true
	p := a.ai.Config.Profiles["deepseek"]
	p.Key = "test-only"
	a.ai.Config.Profiles["deepseek"] = p
	a.ai.Topic = "算术"
	a.ai.Bank = "去重题库"
	a.ai.Count = fmt.Sprint(count)
	updates := make(chan func(), 128)
	a.dispatch = func(f func()) { updates <- f }
	return a, updates
}
func TestAIGenerationKeepsUniqueQuestionsAcrossBatches(t *testing.T) {
	a, updates := generationTestApp(t, 15)
	var e error
	a.store, e = openQuestionStore(filepath.Join(t.TempDir(), "data.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer a.store.close()
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	calls := 0
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		calls++
		rows := []generationTestQuestion{}
		if calls == 1 {
			for n := 1; n <= 10; n++ {
				rows = append(rows, generationQuestion(n))
			}
		} else {
			rows = append(rows, generationQuestion(1))
			for n := 11; n <= 14; n++ {
				rows = append(rows, generationQuestion(n))
			}
		}
		return generationReply(rows), nil
	})
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if a.ai.Error || len(a.ai.Preview) != 14 {
		t.Fatalf("valid questions discarded: count=%d status=%s", len(a.ai.Preview), a.ai.Message)
	}
	if calls != 2 {
		t.Fatalf("unexpected extra paid requests: %d", calls)
	}
	if !strings.Contains(a.ai.Message, "15") || !strings.Contains(a.ai.Message, "14") || !strings.Contains(a.ai.Message, "重复") {
		t.Fatal("partial result not explained", a.ai.Message)
	}
	qs, e := a.store.load()
	if e != nil || len(qs) != 0 {
		t.Fatal("preview saved without confirmation", e)
	}
	a.saveAIPreview()
	pumpAI(t, a, updates)
	qs, e = a.store.load()
	if e != nil || len(qs) != 14 {
		t.Fatal("unique preview not saved", len(qs), e)
	}
}
func TestAIGenerationSendsPreviousQuestionsToLaterBatches(t *testing.T) {
	a, updates := generationTestApp(t, 11)
	calls := 0
	previousFound := false
	nextIndex := 0
	a.ai.Client.Transport = aiTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		var request struct{ Messages []struct{ Content string } }
		if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
			return nil, e
		}
		var input struct {
			PreviousQuestions []struct {
				Question string `json:"question"`
			} `json:"previous_questions"`
			StartIndex int `json:"start_index"`
		}
		if e := json.Unmarshal([]byte(request.Messages[1].Content), &input); e != nil {
			return nil, e
		}
		if calls == 2 {
			nextIndex = input.StartIndex
			for _, q := range input.PreviousQuestions {
				if q.Question == generationQuestion(1).Question {
					previousFound = true
				}
			}
		}
		rows := []generationTestQuestion{}
		start, end := 1, 10
		if calls == 2 {
			start, end = 11, 11
		}
		for n := start; n <= end; n++ {
			rows = append(rows, generationQuestion(n))
		}
		return generationReply(rows), nil
	})
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if a.ai.Error || len(a.ai.Preview) != 11 {
		t.Fatal(a.ai.Message)
	}
	if !previousFound || nextIndex != 11 {
		t.Fatal("later batch cannot avoid earlier questions", previousFound, nextIndex)
	}
}
func TestAIGenerationFiltersDuplicatesWithinBatch(t *testing.T) {
	a, updates := generationTestApp(t, 3)
	calls := 0
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return generationReply([]generationTestQuestion{generationQuestion(1), generationQuestion(1), generationQuestion(2)}), nil
	})
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if a.ai.Error || len(a.ai.Preview) != 2 || calls != 1 {
		t.Fatal("duplicate invalidated usable batch", len(a.ai.Preview), calls, a.ai.Message)
	}
}
func TestAIGenerationGenericStemAllowsDifferentOptions(t *testing.T) {
	a, updates := generationTestApp(t, 2)
	q := generationQuestion(1)
	q.Question = "下列哪个说法正确？"
	other := q
	other.Options = []string{"地球是平的", "地球近似球形"}
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		return generationReply([]generationTestQuestion{q, other}), nil
	})
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if a.ai.Error || len(a.ai.Preview) != 2 {
		t.Fatal("different option sets mistaken for duplicate", a.ai.Message)
	}
}
func TestAIGenerationIgnoresOptionOrderWhitespaceAndChangedAnswerWhenDeduplicating(t *testing.T) {
	a, updates := generationTestApp(t, 3)
	q := generationQuestion(1)
	other := q
	other.Question = "  知识点   1 的结果？ "
	other.Options = []string{"2", "1"}
	other.Answer = "A"
	conflict := q
	conflict.Answer = "A"
	conflict.Explanation = "另一份解析"
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		return generationReply([]generationTestQuestion{q, other, conflict}), nil
	})
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if a.ai.Error || len(a.ai.Preview) != 1 || a.ai.Preview[0].Answer != 2 {
		t.Fatal("duplicate/conflicting answers retained", a.ai.Preview, a.ai.Message)
	}
}
func TestAIGenerationRepeatedBatchesHaveBoundedRequests(t *testing.T) {
	a, updates := generationTestApp(t, 21)
	calls := 0
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		calls++
		size := 10
		if calls == 3 {
			size = 1
		}
		rows := []generationTestQuestion{}
		for n := 0; n < size; n++ {
			rows = append(rows, generationQuestion(1))
		}
		return generationReply(rows), nil
	})
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if a.ai.Error || len(a.ai.Preview) != 1 || calls != 3 {
		t.Fatal("duplicate responses discarded result or caused extra requests", len(a.ai.Preview), calls, a.ai.Message)
	}
}
func TestAIGenerationStillRejectsInvalidLaterBatch(t *testing.T) {
	a, updates := generationTestApp(t, 11)
	calls := 0
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		calls++
		rows := []generationTestQuestion{}
		if calls == 1 {
			for n := 1; n <= 10; n++ {
				rows = append(rows, generationQuestion(n))
			}
		} else {
			bad := generationQuestion(11)
			bad.Answer = "Z"
			rows = append(rows, bad)
		}
		return generationReply(rows), nil
	})
	a.generateAIQuestions()
	pumpAI(t, a, updates)
	if !a.ai.Error || len(a.ai.Preview) != 0 || calls != 2 {
		t.Fatal("invalid question accepted", a.ai.Message)
	}
}

func TestAIGenerationPreviousContextIsBoundedAndDoesNotIncludeAnswers(t *testing.T) {
	qs := []question{}
	for n := 0; n < 500; n++ {
		qs = append(qs, question{Kind: "单选题", Prompt: fmt.Sprint(n) + strings.Repeat("长题干", 1000), Options: []string{strings.Repeat("长选项", 1000), "另一个选项"}, TextAnswer: "不应发送的答案", Explanation: "不应发送的解析"})
	}
	hints := aiPreviousQuestions(qs)
	data, e := json.Marshal(hints)
	if e != nil || len(data) > 48000 || len(hints) == 0 || len(hints) >= 500 {
		t.Fatal("unbounded or empty history", len(data), len(hints), e)
	}
	if !strings.HasPrefix(hints[len(hints)-1].Question, "499") || strings.Contains(string(data), "不应发送") {
		t.Fatal("history does not prefer latest questions or leaks answers")
	}
}

func TestAIGenerationPageSelectsEveryConfiguredProvider(t *testing.T) {
	for _, provider := range aiProviders {
		t.Run(provider.ID, func(t *testing.T) {
			a, updates := generationTestApp(t, 1)
			profile := a.ai.Config.Profiles[provider.ID]
			profile.Key = "test-only-" + provider.ID
			profile.Model = "chosen-" + provider.ID
			a.ai.Config.Profiles[provider.ID] = profile
			a.openAI("generate")
			tt := ui.NewTester(a.view, 800, 600)
			click(t, tt, "出题服务商："+provider.Name)
			if a.ai.Config.Selected != provider.ID {
				t.Fatal("generation selection did not change", a.ai.Config.Selected)
			}
			selectedURL, selectedModel, selectedKey := "", "", ""
			a.ai.Client.Transport = aiTestTransport(func(r *http.Request) (*http.Response, error) {
				selectedURL = r.URL.String()
				selectedKey = r.Header.Get("Authorization")
				var body struct{ Model string }
				if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
					return nil, e
				}
				selectedModel = body.Model
				return generationReply([]generationTestQuestion{generationQuestion(1)}), nil
			})
			a.generateAIQuestions()
			pumpAI(t, a, updates)
			if a.ai.Error || len(a.ai.Preview) != 1 {
				t.Fatal(a.ai.Message)
			}
			if selectedURL != strings.TrimRight(profile.BaseURL, "/")+"/chat/completions" || selectedModel != profile.Model || selectedKey != "Bearer "+profile.Key {
				t.Fatal("request did not use selected provider profile", selectedURL, selectedModel)
			}
			if a.ai.Preview[0].Meta["AI服务商"] != provider.ID {
				t.Fatal("preview attribution incorrect")
			}
		})
	}
}
func TestAIUnconfiguredGenerationProviderOpensMatchingSettingsWithoutTraffic(t *testing.T) {
	a, _ := generationTestApp(t, 1)
	calls := 0
	a.ai.Client.Transport = aiTestTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return generationReply([]generationTestQuestion{generationQuestion(1)}), nil
	})
	a.openAI("generate")
	tt := ui.NewTester(a.view, 800, 600)
	click(t, tt, "出题服务商：通义千问")
	if _, ok := tt.Find("配置出题服务商"); !ok {
		t.Fatal("missing configuration entry")
	}
	a.generateAIQuestions()
	if calls != 0 || a.ai.Busy || a.ai.Tab != "settings" || a.ai.EditorProvider != "qwen" {
		t.Fatal("unconfigured choice used a fallback provider", calls, a.ai.Tab, a.ai.EditorProvider)
	}
	if a.ai.Config.Profiles["deepseek"].Key != "test-only" {
		t.Fatal("selection overwrote another provider profile")
	}
}

func TestAIGenerationProviderPickerKeepsFooterAccessible(t *testing.T) {
	for _, height := range []float32{500, 600} {
		a, _ := generationTestApp(t, 1)
		a.openAI("generate")
		tt := ui.NewTester(a.view, 800, int(height))
		for _, label := range []string{"生成AI题库预览", "关闭AI助手", "配置出题服务商"} {
			bounds, ok := tt.Find(label)
			if !ok || bounds.Y < 0 || bounds.Y+bounds.H > height {
				t.Fatal("generation action clipped", height, label, bounds)
			}
		}
	}
}
