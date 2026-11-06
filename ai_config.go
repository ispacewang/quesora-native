// AI 配置独立于题库备份：含 Key 的整个配置仅以 Windows 当前用户保护的密文落盘。
package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// saveAIConfig 先生成密文并同步临时文件，最后原子替换；失败保留旧配置。
func saveAIConfig(filename string, c aiConfig) error {
	if filename == "" {
		return errors.New("AI 配置目录不可用")
	}
	raw, e := json.Marshal(c)
	if e != nil {
		return e
	}
	encrypted, e := protectAISecret(raw)
	if e != nil {
		return errors.New("无法使用 Windows 加密保存配置")
	}
	if e = os.MkdirAll(filepath.Dir(filename), 0700); e != nil {
		return errors.New("无法创建 AI 配置目录")
	}
	f, e := os.CreateTemp(filepath.Dir(filename), ".ai-config-*")
	if e != nil {
		return errors.New("无法保存 AI 配置")
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, e = f.Write(encrypted); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e == nil {
		e = os.Rename(temp, filename)
	}
	if e != nil {
		return errors.New("无法保存 AI 配置，原配置已保留")
	}
	return nil
}

// loadAIConfig 限制文件大小，无法解密或损坏时明确报错，不尝试明文回退。
func loadAIConfig(filename string) (aiConfig, error) {
	c := defaultAIConfig()
	info, e := os.Stat(filename)
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return c, errors.New("AI 配置文件无法读取或格式异常")
	}
	raw, e := os.ReadFile(filename)
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, errors.New("AI 配置无法读取")
	}
	if len(raw) > 65536 {
		return c, errors.New("AI 配置文件异常")
	}
	raw, e = unprotectAISecret(raw)
	if e != nil {
		return c, errors.New("AI 配置无法解密，请在当前 Windows 用户下重新配置")
	}
	var loaded aiConfig
	if json.Unmarshal(raw, &loaded) != nil {
		return c, errors.New("AI 配置损坏，请重新配置")
	}
	c.Enabled = loaded.Enabled
	c.AutoJudge = loaded.AutoJudge
	if _, ok := providerByID(loaded.Selected); ok {
		c.Selected = loaded.Selected
	}
	for _, p := range aiProviders {
		if v, ok := loaded.Profiles[p.ID]; ok {
			c.Profiles[p.ID] = v
		}
	}
	return c, nil
}
