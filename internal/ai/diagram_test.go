package ai

import (
	"encoding/json"
	"testing"

	"github.com/ispacewang/quesora-native/internal/diagram"
)

func TestAIGeneratedDiagramSurvivesStorageJSON(t *testing.T) {
	qs, err := ParseQuestions(`[{"question":"观察图像","type":"单选题","options":["甲","乙"],"answer":"A","diagram":{"elements":[{"type":"functiongraph","attrs":["x^2"]}]}}]`, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(qs[0].Meta)
	if err != nil {
		t.Fatal(err)
	}
	var read map[string]any
	if err = json.Unmarshal(raw, &read); err != nil {
		t.Fatal(err)
	}
	if svg, err := diagram.FromMeta(read); err != nil || svg == "" {
		t.Fatal("配图丢失", err)
	}
}
