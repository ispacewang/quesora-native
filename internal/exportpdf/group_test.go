package exportpdf

import (
	"reflect"
	"testing"

	"github.com/ispacewang/quesora-native/internal/domain"
	"golang.org/x/image/font/sfnt"
)

func TestPDFGroupsQuestionsAndKeepsSourceIndices(t *testing.T) {
	input := []domain.Question{{Kind: "多选题"}, {Kind: "单选题"}, {Kind: "多选题"}, {Kind: "简答题"}}
	want := []questionGroup{{"单选题", []int{1}}, {"多选题", []int{0, 2}}, {"简答题", []int{3}}}
	if got := groups(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected order: %#v", got)
	}
}

func TestPaperFontCoverageIdentifiesMissingMath(t *testing.T) {
	raw, err := paperFont(400)
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := sfnt.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	l := pageLayout{coverage: coverage}
	if !l.canPrint('题') || !l.canPrint('A') {
		t.Fatal("ordinary text should stay selectable")
	}
	if l.canPrint('π') || l.canPrint('𝑥') {
		t.Fatal("missing or supplementary glyph should use the fallback")
	}
}
