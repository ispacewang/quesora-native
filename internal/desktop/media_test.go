package desktop

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/ispacewang/quesora-native/internal/domain"
	"github.com/ispacewang/quesora-native/internal/exportpdf"
)

func TestShareCardContainsOnlyPromptAndDisplayedOptions(t *testing.T) {
	loadNativeAssets()
	q := mediaDemoQuestion(domain.Question{ID: "sharing", Bank: "PRIVATE BANK", Kind: "单选题"})
	card := captureShare(q, []int{2, 1, 3, 0})
	if card.options[0] != q.Options[2] {
		t.Fatal("option order changed")
	}
	tt := ui.NewTester(card.view, 640, 1000)
	content := strings.Join(tt.Texts(), "\n")
	logo, ok := tt.Find("Quesora Logo")
	if !ok || logo.Y > 76 {
		t.Fatal("brand not visible at top", logo)
	}
	if !strings.Contains(content, "Quesora") || !strings.Contains(content, q.Prompt) || strings.Contains(content, q.Explanation) || strings.Contains(content, q.Bank) || strings.Contains(content, "下载图片") {
		t.Fatal(content)
	}
	pngImage, err := renderShare(card)
	if err != nil || pngImage.Bounds().Dx() != 1280 || pngImage.Bounds().Dy() < 400 {
		t.Fatal(pngImage.Bounds(), err)
	}
	dir := t.TempDir()
	first, err := saveShareImage(dir, pngImage, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := saveShareImage(dir, pngImage, time.Unix(1, 0))
	if err != nil || first == second || filepath.Dir(first) != dir {
		t.Fatal(second, err)
	}
	f, err := os.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width != 1280 || cfg.Height != pngImage.Bounds().Dy() {
		t.Fatal(cfg, err)
	}
}

func TestMediaDiagramAndShareActionsAreReachable(t *testing.T) {
	a := newApp()
	a.questions[0] = mediaDemoQuestion(a.questions[0])
	updates := make(chan func(), 4)
	a.dispatch = func(update func()) { updates <- update }
	a.share.defaultDirectory = t.TempDir()
	tt := ui.NewTester(a.view, 1100, 720)
	if _, ok := tt.Find("题目配图"); !ok {
		t.Fatal("question diagram missing")
	}
	tt.Scroll(550, 490, 0, 300)
	click(t, tt, "放大配图")
	if _, ok := tt.Find("左移配图"); !ok {
		t.Fatal("pan controls missing when zoomed")
	}
	tt.Scroll(570, 480, 0, 1000)
	click(t, tt, "分享题目")
	select {
	case fn := <-updates:
		fn()
	case <-time.After(12 * time.Second):
		t.Fatal("share did not render")
	}
	tt.Frame()
	if a.share.bitmap == nil || !a.share.open {
		t.Fatal(a.share.message)
	}
	click(t, tt, "下载图片")
	select {
	case fn := <-updates:
		fn()
	case <-time.After(12 * time.Second):
		t.Fatal("share did not save")
	}
	if a.share.savedPath == "" || filepath.Dir(a.share.savedPath) != a.share.defaultDirectory {
		t.Fatal(a.share.message)
	}
}

func TestPaperAndAnswersIncludeFormulaAndDiagramImages(t *testing.T) {
	q := mediaDemoQuestion(domain.Question{ID: "media", Bank: "数学", Kind: "单选题"})
	paper := exportpdf.Paper{Bank: "数学", Minutes: 20, Rasterize: rasterizePaperSVG}
	for i := 0; i < 5; i++ {
		paper.Questions = append(paper.Questions, q)
		paper.Responses = append(paper.Responses, domain.Response{OptionOrder: []int{2, 1, 3, 0}})
	}
	for _, answers := range []bool{false, true} {
		filename := filepath.Join(t.TempDir(), "paper.pdf")
		if err := exportpdf.Write(filename, paper, answers); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filename)
		if err != nil || !bytes.HasPrefix(raw, []byte("%PDF-")) {
			t.Fatal(err)
		}
		if bytes.Count(raw, []byte("/Subtype /Image")) < 10 {
			t.Fatal("missing formula or question figures")
		}
		if bytes.Count(raw, []byte("/Type /Page\n")) < 2 {
			t.Fatal("long paper was not paginated")
		}
	}
}

func TestShareDirectorySurvivesReopenAndReset(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "questions.db")
	store, err := openQuestionStore(filename)
	if err != nil {
		t.Fatal(err)
	}
	a := newApp()
	a.store = store
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	updates := make(chan func(), 8)
	a.dispatch = func(update func()) { updates <- update }
	folder := t.TempDir()
	a.share.defaultDirectory = t.TempDir()
	a.share.pick = func(string) (string, error) { return folder, nil }
	awaitChange := func() {
		deadline := time.After(5 * time.Second)
		for a.share.picking {
			select {
			case fn := <-updates:
				fn()
			case <-deadline:
				t.Fatal("settings update timed out")
			}
		}
	}
	a.setShareDirectory(false)
	awaitChange()
	if a.share.destination() != folder {
		t.Fatal(a.share.settingsMessage)
	}
	reread, err := openQuestionStore(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer reread.Close()
	b := newApp()
	b.store = reread
	b.loadShareDirectory()
	if b.share.directory != folder {
		t.Fatal("path was not persisted")
	}
	a.setShareDirectory(true)
	awaitChange()
	if a.share.destination() != a.share.defaultDirectory {
		t.Fatal(a.share.settingsMessage)
	}
	b.loadShareDirectory()
	if b.share.directory != "" {
		t.Fatal("reset was not persisted")
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
}
