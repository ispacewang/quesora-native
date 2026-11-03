package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type updateTestTransport func(*http.Request) (*http.Response, error)

func (f updateTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func updateFixture(version string) updateRelease {
	tag := "v" + version
	prefix := "QuesoraNative-" + version
	return updateRelease{Tag: tag, Prerelease: strings.Contains(version, "-"), Assets: []updateAsset{
		{Name: prefix + "-setup-windows-x64.exe", URL: "https://github.com/ispacewang/quesora-native/releases/download/" + tag + "/" + prefix + "-setup-windows-x64.exe", Size: 12},
		{Name: prefix + "-SHA256SUMS.txt", URL: "https://github.com/ispacewang/quesora-native/releases/download/" + tag + "/" + prefix + "-SHA256SUMS.txt", Size: 200},
	}}
}
func TestUpdateVersionsAndChannels(t *testing.T) {
	for _, pair := range [][2]string{{"0.1.0-preview.10", "0.1.0-preview.9"}, {"0.1.0", "0.1.0-preview.99"}, {"0.2.0-preview.1", "0.1.9"}} {
		n, e := compareUpdateVersions(pair[0], pair[1])
		if e != nil || n <= 0 {
			t.Fatal(pair, n, e)
		}
	}
	for _, bad := range []string{"1.0", "01.2.3", "1.2.3-preview.01", "../2.0.0"} {
		if _, e := compareUpdateVersions(bad, "0.1.0"); e == nil {
			t.Fatal("invalid version", bad)
		}
	}
	releases := []updateRelease{updateFixture("0.1.0-preview.10"), updateFixture("0.1.0-preview.9"), updateFixture("0.1.0")}
	c, e := selectUpdateRelease(releases, "0.1.0-preview.4")
	if e != nil || c == nil || c.Version != "0.1.0" {
		t.Fatal(c, e)
	}
	c, e = selectUpdateRelease([]updateRelease{updateFixture("0.2.0-preview.1")}, "0.1.0")
	if e != nil || c != nil {
		t.Fatal("stable channel accepted preview", c, e)
	}
	c, e = selectUpdateRelease([]updateRelease{updateFixture("0.1.0-preview.3")}, "0.1.0-preview.4")
	if e != nil || c != nil {
		t.Fatal("downgrade offered", c, e)
	}
}
func TestUpdateRejectsUntrustedOrAmbiguousAssets(t *testing.T) {
	for _, bad := range []string{"http://github.com/ispacewang/quesora-native/releases/download/v0.2.0/a.exe", "https://github.com.evil.test/a.exe", "https://github.com/other/repo/releases/download/v0.2.0/a.exe", "https://user@github.com/ispacewang/quesora-native/releases/download/v0.2.0/a.exe"} {
		release := updateFixture("0.2.0")
		release.Assets[0].URL = bad
		if _, e := selectUpdateRelease([]updateRelease{release}, "0.1.0"); e == nil {
			t.Fatal("unsafe URL accepted", bad)
		}
	}
	release := updateFixture("0.2.0")
	release.Assets = append(release.Assets, release.Assets[0])
	if _, e := selectUpdateRelease([]updateRelease{release}, "0.1.0"); e == nil {
		t.Fatal("duplicate installer accepted")
	}
	client := newUpdateHTTPClient()
	req, _ := http.NewRequest("GET", "https://evil.test/a", nil)
	if e := client.CheckRedirect(req, nil); e == nil {
		t.Fatal("foreign redirect allowed")
	}
}
func TestDownloadUpdateVerifiesHashAndRemovesPartialFiles(t *testing.T) {
	release := updateFixture("0.2.0")
	payload := []byte("MZ-test-setup")
	release.Assets[0].Size = int64(len(payload))
	candidate, e := selectUpdateRelease([]updateRelease{release}, "0.1.0")
	if e != nil {
		t.Fatal(e)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	good := true
	client := &http.Client{Transport: updateTestTransport(func(r *http.Request) (*http.Response, error) {
		body := string(payload)
		if strings.HasSuffix(r.URL.Path, ".txt") {
			digest := hash
			if !good {
				digest = strings.Repeat("0", 64)
			}
			body = digest + "  " + candidate.Installer.Name + "\n"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: http.Header{}}, nil
	})}
	dir := t.TempDir()
	download, e := downloadUpdate(context.Background(), client, *candidate, dir, nil)
	if e != nil {
		t.Fatal(e)
	}
	if download.SHA256 != hash {
		t.Fatal("hash not retained")
	}
	if _, e = os.Stat(download.Path); e != nil {
		t.Fatal(e)
	}
	if e = verifyDownloadedUpdate(download); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(download.Path, []byte("changed"), 0600)
	if e = verifyDownloadedUpdate(download); e == nil {
		t.Fatal("changed installer accepted")
	}
	good = false
	badDir := t.TempDir()
	if _, e = downloadUpdate(context.Background(), client, *candidate, badDir, nil); e == nil {
		t.Fatal("bad checksum accepted")
	}
	files, _ := filepath.Glob(filepath.Join(badDir, "*"))
	if len(files) != 0 {
		t.Fatal("partial artifacts left", files)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = downloadUpdate(ctx, client, *candidate, t.TempDir(), nil); e == nil {
		t.Fatal("cancel ignored")
	}
}

func TestUpdateMetadataFailureDoesNotBecomeNoUpdate(t *testing.T) {
	for _, status := range []int{403, 429, 500} {
		client := &http.Client{Transport: updateTestTransport(func(*http.Request) (*http.Response, error) {
			h := http.Header{}
			if status == 403 {
				h.Set("X-RateLimit-Remaining", "0")
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Header: h}, nil
		})}
		if c, e := fetchUpdate(context.Background(), client, "0.1.0-preview.4"); e == nil || c != nil {
			t.Fatal("failed request reported success", status, c, e)
		}
	}
}
