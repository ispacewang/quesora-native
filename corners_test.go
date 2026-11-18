package main

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"testing"
)

func TestCornerStyleSwitchPersistsAndRestores(t *testing.T) {
	a := newApp()
	s := populatedStore(t, filepath.Join(t.TempDir(), "questions.db"))
	defer s.close()
	a.store = s
	a.questions, _ = s.load()
	a.jobs = newLibraryJobs()
	defer a.jobs.close()
	updates := make(chan func(), 32)
	a.dispatch = func(f func()) { updates <- f }
	a.openSettings()
	tt := ui.NewTester(a.view, 1450, 800)
	click(t, tt, "边角：圆角")
	drainMistakeJobs(t, a, updates)
	raw, _ := json.Marshal(a.preferences())
	var p map[string]any
	json.Unmarshal(raw, &p)
	if p["Rounded"] != true {
		t.Fatal("rounded selection not saved", string(raw))
	}
	next := newApp()
	next.store = s
	next.questions, _ = s.load()
	if e := next.restorePreferences(); e != nil {
		t.Fatal(e)
	}
	next.openSettings()
	nt := ui.NewTester(next.view, 800, 500)
	// This remains reachable at the supported minimum window size.
	nt.Scroll(400, 300, 0, 180)
	click(t, nt, "边角：直角")
	raw, _ = json.Marshal(next.preferences())
	json.Unmarshal(raw, &p)
	if p["Rounded"] != false {
		t.Fatal("square selection ignored")
	}
}

func TestCornerRadiiScaleAndCap(t *testing.T) {
	previous := float32(-1)
	for _, size := range []float32{0, 16, 24, 28, 50, 80, 160, 500} {
		r := scaledCornerRadius(true, size)
		if scaledCornerRadius(false, size) != 0 || r < previous || r > 16 || r > size/2 {
			t.Fatalf("invalid radius %v for extent %v", r, size)
		}
		if size > 0 && size <= 80 && r <= previous {
			t.Fatal("small and large components share one fixed radius")
		}
		previous = r
	}
}
func TestRoundedQuestionCornersDoNotReflow(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1450, 800)
	before, ok := tt.Find("A. −1")
	if !ok {
		t.Fatal("option missing")
	}
	img := tt.Image()
	saved := make([][4]uint8, 0, 144)
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			p := img.RGBAAt(int(before.X)+x, int(before.Y)+y)
			saved = append(saved, [4]uint8{p.R, p.G, p.B, p.A})
		}
	}
	a.setRounded(true)
	tt.Frame()
	after, _ := tt.Find("A. −1")
	if before != after {
		t.Fatal("appearance change moved question layout")
	}
	changed := false
	i := 0
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			p := tt.Image().RGBAAt(int(after.X)+x, int(after.Y)+y)
			if saved[i] != [4]uint8{p.R, p.G, p.B, p.A} {
				changed = true
			}
			i++
		}
	}
	if !changed {
		t.Fatal("rounded setting did not reach rendered option corners")
	}
	click(t, tt, "切换错题面板")
	click(t, tt, "切换错题面板")
}
func TestOldPreferencesDefaultToSquareCorners(t *testing.T) {
	s := populatedStore(t, filepath.Join(t.TempDir(), "questions.db"))
	defer s.close()
	_, e := s.db.Exec("INSERT INTO settings(key,value) VALUES('learning-preferences',?)", `{"Version":2,"Appearance":"light"}`)
	if e != nil {
		t.Fatal(e)
	}
	a := newApp()
	a.store = s
	a.questions, _ = s.load()
	a.rounded = true
	if e = a.restorePreferences(); e != nil || a.rounded {
		t.Fatal("old preferences should remain square", e)
	}
	next := newApp()
	next.rounded = true
	a.acceptRestoredState(next)
	if !a.rounded {
		t.Fatal("backup restore lost corner style")
	}
}
