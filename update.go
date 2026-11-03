// 官方 GitHub Release 发现与版本选择。没有启动时网络请求，调用方显式触发并持有取消上下文。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const updateRepository = "ispacewang/quesora-native"
const updateAPI = "https://api.github.com/repos/" + updateRepository + "/releases?per_page=100"
const maxUpdateBytes int64 = 200 << 20

// updateRateLimitError carries the server retry boundary; the UI must not immediately repeat the same request.
type updateRateLimitError struct{ Until time.Time }

func (e *updateRateLimitError) Error() string {
	return "GitHub 请求频率受限，请在 " + e.Until.Local().Format("15:04:05") + " 后重试，或使用发布页面。"
}

type updateAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}
type updateRelease struct {
	Tag        string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []updateAsset `json:"assets"`
}

// updateCandidate 只由验证过的仓库标签和固定 Windows 产物名构造，不执行 Release 正文里的命令或链接。
type updateCandidate struct {
	Version, Tag, Page   string
	Installer, Checksums updateAsset
}
type updateVersion struct {
	core [3]string
	pre  []string
}

var updateVersionRE = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func parseUpdateVersion(raw string) (updateVersion, error) {
	var v updateVersion
	if len(raw) > 128 {
		return v, fmt.Errorf("无效版本号")
	}
	m := updateVersionRE.FindStringSubmatch(raw)
	if m == nil {
		return v, fmt.Errorf("无效版本号：%s", raw)
	}
	for i := 0; i < 3; i++ {
		if len(m[i+1]) > 1 && m[i+1][0] == '0' {
			return v, fmt.Errorf("版本号不能有前导零")
		}
		v.core[i] = m[i+1]
	}
	if m[4] != "" {
		v.pre = strings.Split(m[4], ".")
		for _, part := range v.pre {
			if digitsOnly(part) && len(part) > 1 && part[0] == '0' {
				return v, fmt.Errorf("预览版本号不能有前导零")
			}
		}
	}
	return v, nil
}
func compareNumberText(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

// compareUpdateVersions 遵循本项目版本格式，并按 SemVer 的数字和预发布优先级比较，避免 preview.10 被排在 preview.9 前面。
func compareUpdateVersions(a, b string) (int, error) {
	x, e := parseUpdateVersion(a)
	if e != nil {
		return 0, e
	}
	y, e := parseUpdateVersion(b)
	if e != nil {
		return 0, e
	}
	for i := range x.core {
		if n := compareNumberText(x.core[i], y.core[i]); n != 0 {
			return n, nil
		}
	}
	if len(x.pre) == 0 && len(y.pre) == 0 {
		return 0, nil
	}
	if len(x.pre) == 0 {
		return 1, nil
	}
	if len(y.pre) == 0 {
		return -1, nil
	}
	for i := 0; i < min(len(x.pre), len(y.pre)); i++ {
		p, q := x.pre[i], y.pre[i]
		var n int
		switch {
		case digitsOnly(p) && digitsOnly(q):
			n = compareNumberText(p, q)
		case digitsOnly(p):
			n = -1
		case digitsOnly(q):
			n = 1
		default:
			n = strings.Compare(p, q)
		}
		if n != 0 {
			return n, nil
		}
	}
	if len(x.pre) < len(y.pre) {
		return -1, nil
	}
	if len(x.pre) > len(y.pre) {
		return 1, nil
	}
	return 0, nil
}

func trustedUpdateAsset(raw, tag, name string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "/"+updateRepository+"/releases/download/"+tag+"/"+name
}
func candidateFromRelease(r updateRelease) (*updateCandidate, error) {
	if _, e := parseUpdateVersion(r.Tag); e != nil || !strings.HasPrefix(r.Tag, "v") {
		return nil, nil
	}
	version := strings.TrimPrefix(r.Tag, "v")
	prefix := "QuesoraNative-" + version
	c := &updateCandidate{Version: version, Tag: r.Tag, Page: "https://github.com/" + updateRepository + "/releases/tag/" + r.Tag}
	counts := map[string]int{}
	for _, asset := range r.Assets {
		switch asset.Name {
		case prefix + "-setup-windows-x64.exe":
			counts["installer"]++
			c.Installer = asset
		case prefix + "-SHA256SUMS.txt":
			counts["checksums"]++
			c.Checksums = asset
		}
	}
	if counts["installer"] > 1 || counts["checksums"] > 1 {
		return nil, fmt.Errorf("发布产物重复，无法确认安全的更新文件")
	}
	if counts["installer"] == 0 || counts["checksums"] == 0 {
		return nil, nil
	}
	for _, asset := range []updateAsset{c.Installer, c.Checksums} {
		if !trustedUpdateAsset(asset.URL, c.Tag, asset.Name) {
			return nil, fmt.Errorf("更新下载地址不属于官方仓库")
		}
	}
	if c.Installer.Size <= 0 || c.Installer.Size > maxUpdateBytes || c.Checksums.Size <= 0 || c.Checksums.Size > 32<<10 {
		return nil, fmt.Errorf("更新产物大小不符合限制")
	}
	return c, nil
}

// selectUpdateRelease 预览版可接收更高预览/正式版，正式版只接收正式版；永不建议降级或未完成上传的版本。
func selectUpdateRelease(releases []updateRelease, current string) (*updateCandidate, error) {
	running, e := parseUpdateVersion(current)
	if e != nil {
		return nil, e
	}
	var best *updateCandidate
	for _, r := range releases {
		if r.Draft {
			continue
		}
		v, e := parseUpdateVersion(r.Tag)
		if e != nil {
			continue
		}
		if len(running.pre) == 0 && (r.Prerelease || len(v.pre) > 0) {
			continue
		}
		n, e := compareUpdateVersions(r.Tag, current)
		if e != nil || n <= 0 {
			continue
		}
		c, e := candidateFromRelease(r)
		if e != nil {
			return nil, e
		}
		if c == nil {
			continue
		}
		if best == nil {
			best = c
		} else if n, _ := compareUpdateVersions(c.Version, best.Version); n > 0 {
			best = c
		}
	}
	return best, nil
}

// newUpdateHTTPClient 仅允许 HTTPS 的 GitHub/API/发行资产 CDN 重定向；不携带用户 Git 凭据。
func newUpdateHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("更新下载重定向过多")
		}
		u := req.URL
		if u.Scheme != "https" || u.User != nil {
			return fmt.Errorf("拒绝非 HTTPS 更新地址")
		}
		switch u.Host {
		case "github.com", "api.github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com", "github-releases.githubusercontent.com":
			return nil
		}
		return fmt.Errorf("拒绝非官方更新重定向")
	}}
}
func updateGET(ctx context.Context, client *http.Client, raw string) (*http.Response, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "QuesoraNative/"+appVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode == 403 && (resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "") {
			until := time.Now().Add(time.Minute)
			if seconds, e := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); e == nil && time.Unix(seconds, 0).After(until) {
				until = time.Unix(seconds, 0)
			}
			if seconds, e := strconv.ParseInt(resp.Header.Get("Retry-After"), 10, 64); e == nil && seconds > 0 && seconds <= 86400 {
				if next := time.Now().Add(time.Duration(seconds) * time.Second); next.After(until) {
					until = next
				}
			}
			return nil, &updateRateLimitError{Until: until}
		}
		return nil, fmt.Errorf("更新服务返回 HTTP %d", resp.StatusCode)
	}
	return resp, nil
}
func readUpdateText(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
	resp, e := updateGET(ctx, client, raw)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("更新元数据超过大小限制")
	}
	return data, nil
}

// fetchUpdate 使用发布列表而非 latest，以正确处理 GitHub 默认排除预发布版本的行为。
// API 契约：https://docs.github.com/en/rest/releases/releases#list-releases
func fetchUpdate(ctx context.Context, client *http.Client, current string) (*updateCandidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	data, e := readUpdateText(ctx, client, updateAPI, 4<<20)
	if e != nil {
		return nil, e
	}
	var releases []updateRelease
	if e = json.Unmarshal(data, &releases); e != nil {
		return nil, fmt.Errorf("更新信息无法解析：%w", e)
	}
	return selectUpdateRelease(releases, current)
}
