// 官方 Release 订阅源包含预览版，不消耗 REST API 的匿名查询配额。
// 仅使用固定仓库的标签链接；资产地址按已验证版本构造，再确认长度和 SHA-256。
package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const updateFeed = "https://github.com/" + updateRepository + "/releases.atom"

type updateHTTPError struct{ Status int }

func (e *updateHTTPError) Error() string { return fmt.Sprintf("更新服务返回 HTTP %d", e.Status) }

// fetchUpdate 预览通道优先使用公开订阅源；正式通道通过 API 复核 prerelease 标记。
// 订阅源失败时只调用一次 API，不重试限流响应。
func fetchUpdate(ctx context.Context, client *http.Client, current string) (*updateCandidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	running, err := parseUpdateVersion(current)
	if err != nil {
		return nil, err
	}
	// Atom 没有 release.prerelease 字段，不能单凭正式格式标签判断其为正式版。
	if len(running.pre) == 0 {
		return fetchUpdateAPI(ctx, client, current)
	}
	candidate, err := fetchUpdateFeed(ctx, client, current)
	if err == nil {
		return candidate, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return fetchUpdateAPI(ctx, client, current)
}

func fetchUpdateFeed(ctx context.Context, client *http.Client, current string) (*updateCandidate, error) {
	running, err := parseUpdateVersion(current)
	if err != nil {
		return nil, err
	}
	data, err := readUpdateText(ctx, client, updateFeed, 2<<20)
	if err != nil {
		return nil, err
	}
	var feed struct {
		XMLName xml.Name
		Entries []struct {
			Links []struct {
				Href string `xml:"href,attr"`
				Rel  string `xml:"rel,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if xml.Unmarshal(data, &feed) != nil || feed.XMLName.Local != "feed" || feed.XMLName.Space != "http://www.w3.org/2005/Atom" || len(feed.Entries) > 100 {
		return nil, fmt.Errorf("官方发布订阅信息无效")
	}
	tags := []string{}
	seen := map[string]bool{}
	valid := 0
	for _, entry := range feed.Entries {
		for _, link := range entry.Links {
			if link.Rel != "" && link.Rel != "alternate" {
				continue
			}
			u, e := url.Parse(link.Href)
			prefix := "/" + updateRepository + "/releases/tag/"
			if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, prefix) {
				continue
			}
			tag := strings.TrimPrefix(u.Path, prefix)
			version, e := parseUpdateVersion(tag)
			if e != nil || !strings.HasPrefix(tag, "v") {
				continue
			}
			valid++
			if seen[tag] {
				continue
			}
			seen[tag] = true
			if len(running.pre) == 0 && len(version.pre) > 0 {
				continue
			}
			if cmp, _ := compareUpdateVersions(tag, current); cmp > 0 {
				tags = append(tags, tag)
			}
		}
	}
	if len(feed.Entries) > 0 && valid == 0 {
		return nil, fmt.Errorf("订阅信息不包含官方仓库标签")
	}
	sort.Slice(tags, func(i, j int) bool { n, _ := compareUpdateVersions(tags[i], tags[j]); return n > 0 })
	// Bound asset probes even if a feed contains many incomplete publications.
	for _, tag := range tags[:min(len(tags), 5)] {
		version := strings.TrimPrefix(tag, "v")
		prefix := "QuesoraNative-" + version
		release := updateRelease{Tag: tag}
		complete := true
		for _, name := range []string{prefix + "-setup-windows-x64.exe", prefix + "-SHA256SUMS.txt"} {
			raw := "https://github.com/" + updateRepository + "/releases/download/" + tag + "/" + name
			response, e := updateRequest(ctx, client, http.MethodHead, raw)
			if e != nil {
				var status *updateHTTPError
				if errors.As(e, &status) && status.Status == http.StatusNotFound {
					complete = false
					break
				}
				return nil, e
			}
			size := response.ContentLength
			response.Body.Close()
			if size <= 0 {
				return nil, fmt.Errorf("发布文件未提供有效长度")
			}
			release.Assets = append(release.Assets, updateAsset{Name: name, URL: raw, Size: size})
		}
		if !complete {
			continue
		}
		candidate, e := candidateFromRelease(release)
		if e != nil {
			return nil, e
		}
		if candidate != nil {
			return candidate, nil
		}
	}
	if len(tags) > 5 {
		return nil, fmt.Errorf("近期发布的安装文件尚未就绪")
	}
	return nil, nil
}
