// 下载到私有临时文件并校验官方 SHA-256，校验成功后才发布可执行安装器路径。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type downloadedUpdate struct {
	Path, SHA256 string
	Size         int64
}

func installerChecksum(data []byte, name string) (string, error) {
	value := ""
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		count++
		decoded, e := hex.DecodeString(fields[0])
		if e != nil || len(decoded) != sha256.Size {
			return "", fmt.Errorf("更新校验值无效")
		}
		value = strings.ToLower(fields[0])
	}
	if count != 1 {
		return "", fmt.Errorf("未找到唯一的安装器校验值")
	}
	return value, nil
}

type updateProgressWriter struct {
	dst            io.Writer
	written, total int64
	last           time.Time
	progress       func(int64, int64)
}

func (w *updateProgressWriter) Write(p []byte) (int, error) {
	n, e := w.dst.Write(p)
	w.written += int64(n)
	now := time.Now()
	if w.progress != nil && (now.Sub(w.last) >= 100*time.Millisecond || w.written == w.total) {
		w.last = now
		w.progress(w.written, w.total)
	}
	return n, e
}

// downloadUpdate 不执行文件。取消、长度不符或哈希失败会移除临时内容，已有完整缓存不受影响。
func downloadUpdate(ctx context.Context, client *http.Client, c updateCandidate, cache string, progress func(int64, int64)) (downloadedUpdate, error) {
	var result downloadedUpdate
	if e := ctx.Err(); e != nil {
		return result, e
	}
	trusted, e := candidateFromRelease(updateRelease{Tag: c.Tag, Assets: []updateAsset{c.Installer, c.Checksums}})
	if e != nil {
		return result, e
	}
	if trusted == nil || trusted.Version != c.Version {
		return result, fmt.Errorf("更新候选无效")
	}
	if !filepath.IsAbs(cache) {
		return result, fmt.Errorf("更新缓存目录必须为绝对路径")
	}
	data, e := readUpdateText(ctx, client, c.Checksums.URL, 32<<10)
	if e != nil {
		return result, e
	}
	expected, e := installerChecksum(data, c.Installer.Name)
	if e != nil {
		return result, e
	}
	resp, e := updateGET(ctx, client, c.Installer.URL)
	if e != nil {
		return result, e
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxUpdateBytes {
		return result, fmt.Errorf("安装包超过大小限制")
	}
	if e = os.MkdirAll(cache, 0700); e != nil {
		return result, e
	}
	tmp, e := os.CreateTemp(cache, ".quesora-update-*.partial")
	if e != nil {
		return result, e
	}
	name := tmp.Name()
	defer os.Remove(name)
	h := sha256.New()
	writer := &updateProgressWriter{dst: io.MultiWriter(tmp, h), total: c.Installer.Size, progress: progress}
	n, copyErr := io.Copy(writer, io.LimitReader(resp.Body, c.Installer.Size+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return result, copyErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	if n != c.Installer.Size {
		return result, fmt.Errorf("安装包长度与发布信息不符")
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return result, fmt.Errorf("安装包 SHA-256 校验失败，未允许安装")
	}
	target := filepath.Join(cache, c.Installer.Name)
	if e = os.Rename(name, target); e != nil {
		return result, e
	}
	return downloadedUpdate{Path: target, SHA256: got, Size: n}, nil
}

// verifyDownloadedUpdate 在启动前再次检查文件，拒绝下载后被替换/修改的缓存内容。
func verifyDownloadedUpdate(d downloadedUpdate) error {
	info, e := os.Lstat(d.Path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() != d.Size || d.Size <= 0 || d.Size > maxUpdateBytes {
		return fmt.Errorf("已下载的安装包被修改，请重新下载")
	}
	file, e := os.Open(d.Path)
	if e != nil {
		return e
	}
	defer file.Close()
	h := sha256.New()
	if _, e = io.Copy(h, file); e != nil {
		return e
	}
	if hex.EncodeToString(h.Sum(nil)) != d.SHA256 {
		return fmt.Errorf("安装前 SHA-256 复核失败，请重新下载")
	}
	return nil
}
