// AI 服务商与网络边界：仅连接所选厂商官方 HTTPS 地址，禁止认证请求重定向。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// aiProvider 将品牌、默认模型和官方服务地址绑定，防止切换服务商时误发密钥。
type aiProvider struct{ ID, Name, BaseURL, Model, Help string }

var aiProviders = []aiProvider{
	{"deepseek", "DeepSeek", "https://api.deepseek.com", "deepseek-flash", "https://api-docs.deepseek.com/"},
	{"openai", "OpenAI", "https://api.openai.com/v1", "gpt-4.1-mini", "https://developers.openai.com/api/docs/models/gpt-4.1-mini"},
	{"qwen", "通义千问", "https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen-plus", "https://help.aliyun.com/zh/model-studio/compatibility-of-openai-with-dashscope"},
	{"zhipu", "智谱 GLM", "https://open.bigmodel.cn/api/paas/v4", "glm-5.3", "https://docs.bigmodel.cn/cn/api/introduction"},
	{"doubao", "豆包", "https://ark.cn-beijing.volces.com/api/v3", "doubao-seed-2-1-pro-260628", "https://console.volcengine.com/ark"},
}

// aiProfile 保存单个厂商的地址、模型及密钥；落盘前整份配置由 DPAPI 加密。
type aiProfile struct{ BaseURL, Model, Key string }

// aiConfig 将 Beta 总开关与判题开关分开，旧配置升级后仍默认关闭 Beta。
type aiConfig struct {
	Enabled   bool
	AutoJudge bool
	Selected  string
	Profiles  map[string]aiProfile
}

func defaultAIConfig() aiConfig {
	c := aiConfig{Selected: "deepseek", Profiles: map[string]aiProfile{}}
	for _, p := range aiProviders {
		c.Profiles[p.ID] = aiProfile{BaseURL: p.BaseURL, Model: p.Model}
	}
	return c
}
func providerByID(id string) (aiProvider, bool) {
	for _, p := range aiProviders {
		if p.ID == id {
			return p, true
		}
	}
	return aiProvider{}, false
}

// validateAIProfile supports regional/workspace URLs but never an arbitrary third-party proxy.
func validateAIProfile(id string, p aiProfile) error {
	if _, ok := providerByID(id); !ok {
		return errors.New("请选择支持的 AI 服务商")
	}
	if strings.TrimSpace(p.Key) == "" || len(p.Key) > 4096 || strings.ContainsAny(p.Key, "\r\n\t ") {
		return errors.New("请填写有效的 API Key")
	}
	if strings.TrimSpace(p.Model) == "" || len(p.Model) > 160 || strings.ContainsAny(p.Model, "\r\n") {
		return errors.New("请填写模型名称或推理接入点 ID")
	}
	if len(p.BaseURL) > 2048 {
		return errors.New("服务地址过长")
	}
	u, e := url.Parse(p.BaseURL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" && u.Port() != "443" {
		return errors.New("服务地址必须是所选厂商的官方 HTTPS 地址")
	}
	h := strings.ToLower(u.Hostname())
	ok := false
	switch id {
	case "deepseek":
		ok = h == "api.deepseek.com"
	case "openai":
		ok = h == "api.openai.com"
	case "qwen":
		ok = h == "dashscope.aliyuncs.com" || h == "dashscope-intl.aliyuncs.com" || h == "dashscope-us.aliyuncs.com" || strings.HasSuffix(h, ".maas.aliyuncs.com")
	case "zhipu":
		ok = h == "open.bigmodel.cn"
	case "doubao":
		ok = h == "ark.cn-beijing.volces.com" || h == "ark.ap-southeast.bytepluses.com"
	}
	if !ok {
		return errors.New("地址与所选厂商不匹配，请使用其官方 API 地址")
	}
	return nil
}
func newAIHTTPClient() *http.Client {
	return &http.Client{Timeout: 180 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("AI 接口不允许重定向") }}
}

// aiChat never exposes raw service bodies or transport errors: they may echo credentials.
// A failed request is not automatically retried, avoiding duplicate paid generations.
func aiChat(ctx context.Context, client *http.Client, id string, p aiProfile, system, user string, tokens int) (string, error) {
	if e := ctx.Err(); e != nil {
		return "", e
	}
	if e := validateAIProfile(id, p); e != nil {
		return "", e
	}
	if len(user) > 220000 || len(system) > 12000 {
		return "", errors.New("发送内容过长，请减少参考资料")
	}
	body := map[string]any{"model": strings.TrimSpace(p.Model), "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}, "stream": false}
	if id == "openai" {
		body["max_completion_tokens"] = tokens
	} else {
		body["max_tokens"] = tokens
	}
	// Use direct answers for predictable JSON and short connection tests.
	switch id {
	case "deepseek", "zhipu", "doubao":
		body["thinking"] = map[string]string{"type": "disabled"}
	case "qwen":
		body["enable_thinking"] = false
	}
	data, e := json.Marshal(body)
	if e != nil {
		return "", errors.New("无法构造 AI 请求")
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.BaseURL, "/")+"/chat/completions", bytes.NewReader(data))
	if e != nil {
		return "", errors.New("服务地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+p.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("AI 请求未完成，请检查网络、服务地址或稍后重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		switch resp.StatusCode {
		case 401, 403:
			return "", errors.New("认证失败，请检查 API Key、地域和模型权限")
		case 404:
			return "", errors.New("模型或接口不存在，请核对模型名称和地址")
		case 429:
			return "", errors.New("服务限流或额度不足，请检查账户后稍后重试")
		default:
			return "", fmt.Errorf("AI 服务返回 HTTP %d，请稍后重试", resp.StatusCode)
		}
	}
	const limit = 2 << 20
	raw, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil {
		return "", errors.New("读取 AI 响应失败")
	}
	if len(raw) > limit {
		return "", errors.New("AI 响应过大")
	}
	var v struct {
		Choices []struct {
			Message      struct{ Content string }
			FinishReason string `json:"finish_reason"`
		}
	}
	if json.Unmarshal(raw, &v) != nil || len(v.Choices) == 0 || strings.TrimSpace(v.Choices[0].Message.Content) == "" {
		return "", errors.New("AI 未返回可用内容，请调整模型或重试")
	}
	if v.Choices[0].FinishReason != "" && v.Choices[0].FinishReason != "stop" {
		return "", errors.New("AI 输出被截断或未完成，请减少题量或换用合适模型")
	}
	return v.Choices[0].Message.Content, nil
}
