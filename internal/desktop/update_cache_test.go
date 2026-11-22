package desktop

import (
	"context"
	"github.com/ispacewang/quesora-native/internal/buildinfo"
	"github.com/ispacewang/quesora-native/internal/updater"
	"strings"
	"testing"
	"time"
)

func updateFixture(version string) updater.Release {
	tag := "v" + version
	prefix := "QuesoraNative-" + version
	return updater.Release{Tag: tag, Prerelease: strings.Contains(version, "-"), Assets: []updater.Asset{
		{Name: prefix + "-setup-windows-x64.exe", URL: "https://github.com/ispacewang/quesora-native/releases/download/" + tag + "/" + prefix + "-setup-windows-x64.exe", Size: 12},
		{Name: prefix + "-SHA256SUMS.txt", URL: "https://github.com/ispacewang/quesora-native/releases/download/" + tag + "/" + prefix + "-SHA256SUMS.txt", Size: 200},
	}}
}
func TestUpdateRecentCheckPreservesCandidateWithoutMoreTraffic(t *testing.T) {
	a := newApp()
	queue := make(chan func(), 8)
	a.dispatch = func(f func()) { queue <- f }
	calls := 0
	candidate, _ := updater.SelectRelease([]updater.Release{updateFixture("0.99.0-preview.1")}, buildinfo.Version)
	a.updates.fetch = func(context.Context) (*updater.Candidate, error) { calls++; return candidate, nil }
	a.checkForUpdate()
	drainUpdate(t, a, queue)
	downloaded := &updater.Download{Path: "not-executed"}
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
