// AI 窗口状态仅在 UI 线程修改；HTTP 任务捕获不可变快照，通过 publish 返回。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
)

// aiState 持有跨帧模型和可取消任务，不保存 UI Element 或 Context。
type aiState struct {
	EditorAuto                                 bool
	Open, Busy, Saving, Error                  bool
	Tab, Task, Message, Path                   string
	Config                                     aiConfig
	EditorProvider                             string
	Editor                                     aiProfile
	Client                                     *http.Client
	cancel                                     context.CancelFunc
	epoch                                      uint64
	Topic, Bank, Count, Material, MaterialName string
	Preview                                    []question
	PreviewIndex                               int
	Result                                     string
	Judgment                                   *aiJudgment
	JudgeQuestion                              question
	JudgeResponse                              response
}
type aiTaskResult struct {
	Answer    *aiAnswerResult
	Material  *aiMaterial
	Text      string
	Questions []question
	Judgment  *aiJudgment
	Config    *aiConfig
}

func newAIState() aiState {
	c := defaultAIConfig()
	return aiState{Config: c, EditorProvider: c.Selected, Editor: c.Profiles[c.Selected], Client: newAIHTTPClient(), Count: "10", Tab: "generate"}
}
func (a *app) configureAI() {
	if a.store == nil {
		return
	}
	a.ai.Path = filepath.Join(filepath.Dir(a.store.path), "ai-settings.bin")
	c, e := loadAIConfig(a.ai.Path)
	if e != nil {
		a.libraryStatus = e.Error()
		return
	}
	a.ai.Config = c
	a.ai.EditorAuto = c.AutoJudge
	a.ai.EditorProvider = c.Selected
	a.ai.Editor = c.Profiles[c.Selected]
}
func (a *app) openAI(tab string) {
	if a.exam != nil {
		return
	}
	if !a.ai.Config.Enabled && tab != "settings" {
		a.openSettings()
		return
	}
	a.ai.Open = true
	a.ai.Tab = tab
	if tab == "settings" && !a.ai.Busy {
		a.ai.EditorAuto = a.ai.Config.AutoJudge
		a.ai.EditorProvider = a.ai.Config.Selected
		a.ai.Editor = a.ai.Config.Profiles[a.ai.Config.Selected]
	}
}
func (a *app) selectAIProvider(id string) {
	if a.ai.Busy || id == a.ai.EditorProvider {
		return
	}
	a.ai.EditorProvider = id
	a.ai.Editor = a.ai.Config.Profiles[id]
	a.ai.Message = ""
	a.ai.Error = false
}

// selectAIGenerationProvider 切换当前会话服务商，不发送请求，也不更改任何厂商的模型/密钥。
// 持久默认项仍由“保存配置”写入；缺少密钥时生成流程会打开对应配置，不回退到其他厂商。
func (a *app) selectAIGenerationProvider(id string) {
	if a.ai.Busy || a.ai.Saving || !a.ai.Config.Enabled {
		return
	}
	if _, ok := providerByID(id); !ok {
		return
	}
	a.ai.Config.Selected = id
	a.ai.Message = ""
	a.ai.Error = false
}

func (a *app) aiReady() (string, aiProfile, error) {
	if !a.ai.Config.Enabled {
		return "", aiProfile{}, errors.New("AI（Beta）已关闭，请先在设置中启用")
	}
	id := a.ai.Config.Selected
	p := a.ai.Config.Profiles[id]
	return id, p, validateAIProfile(id, p)
}

// startAITask invalidates late completions after cancellation/replacement; workers never retain UI elements.
func (a *app) startAITask(kind string, work func(context.Context) (aiTaskResult, error)) {
	if a.ai.Busy || a.ai.Saving || a.closed || a.restoreBusy || a.updates.installing || a.dispatch == nil {
		return
	}
	if !a.ai.Config.Enabled && kind != "save" && kind != "forget" && kind != "toggle" {
		a.aiFailure(errors.New("请先在设置中启用 AI（Beta）"))
		return
	}
	a.ai.epoch++
	epoch := a.ai.epoch
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	a.ai.cancel = cancel
	a.ai.Busy = true
	a.ai.Error = false
	a.ai.Task = kind
	a.ai.Message = "正在连接所选 AI 服务…"
	if kind == "save" || kind == "forget" || kind == "toggle" {
		a.ai.Message = "正在加密保存配置…"
	}
	if kind == "material" {
		a.ai.Message = "请选择参考资料…"
	}
	go func() {
		defer cancel()
		result, e := work(ctx)
		a.publish(func() {
			if a.ai.epoch != epoch {
				return
			}
			a.ai.Busy = false
			a.ai.cancel = nil
			if e != nil {
				a.ai.Error = !errors.Is(e, context.Canceled)
				a.ai.Message = e.Error()
				if kind == "answer" {
					a.notice = "AI 判题失败：" + e.Error()
				}
				if errors.Is(e, context.Canceled) {
					a.ai.Message = "已取消"
				}
				return
			}
			a.ai.Message = result.Text
			if result.Answer != nil {
				a.finishAIAnswer(*result.Answer)
			}
			if result.Material != nil {
				a.ai.Material = result.Material.Text
				a.ai.MaterialName = result.Material.Name
			}
			if result.Questions != nil {
				a.ai.Preview = result.Questions
				a.ai.PreviewIndex = 0
			}
			if result.Judgment != nil {
				a.ai.Judgment = result.Judgment
			}
			if kind == "diagnose" {
				a.ai.Result = result.Text
			}
			if result.Config != nil {
				a.ai.Config = *result.Config
				if kind == "toggle" && !a.ai.Config.Enabled {
					a.ai.Open = false
					a.ai.Preview = nil
					a.ai.Judgment = nil
					a.ai.Result = ""
					a.ai.Material = ""
					a.ai.MaterialName = ""
					a.ai.Editor.Key = ""
				}
			}
		})
	}()
}

// aiConfigWriting identifies a short, atomic local write that cannot be cancelled halfway.
func (a *app) aiConfigWriting() bool {
	return a.ai.Busy && (a.ai.Task == "save" || a.ai.Task == "forget" || a.ai.Task == "toggle")
}
func (a *app) cancelAITask() {
	if a.aiConfigWriting() {
		return
	}
	if a.ai.cancel != nil {
		a.ai.cancel()
	}
	a.ai.epoch++
	a.ai.cancel = nil
	a.ai.Busy = false
	a.ai.Message = "已取消，未保存结果"
	if a.ai.Task == "answer" {
		a.notice = "已取消 AI 判题，答案尚未提交"
	}
	a.ai.Error = false
}
func (a *app) closeAI() {
	if a.ai.Saving || a.aiConfigWriting() {
		return
	}
	if a.ai.Busy {
		a.cancelAITask()
	}
	a.ai.Editor.Key = ""
	a.ai.Open = false
}

// saveAISettings 先验证编辑副本；成功加密写入后才替换活动配置。
func (a *app) saveAISettings() {
	id, p, filename := a.ai.EditorProvider, a.ai.Editor, a.ai.Path
	if e := validateAIProfile(id, p); e != nil {
		a.ai.Error = true
		a.ai.Message = e.Error()
		return
	}
	c := defaultAIConfig()
	c.Selected = id
	c.Enabled = a.ai.Config.Enabled
	c.AutoJudge = a.ai.EditorAuto
	for k, v := range a.ai.Config.Profiles {
		c.Profiles[k] = v
	}
	c.Profiles[id] = p
	a.startAITask("save", func(context.Context) (aiTaskResult, error) {
		if e := saveAIConfig(filename, c); e != nil {
			return aiTaskResult{}, e
		}
		return aiTaskResult{Text: "配置已使用 Windows 当前用户加密保存", Config: &c}, nil
	})
}
func (a *app) testAIConnection() {
	id, p, client := a.ai.EditorProvider, a.ai.Editor, a.ai.Client
	a.startAITask("test", func(ctx context.Context) (aiTaskResult, error) {
		_, e := aiChat(ctx, client, id, p, "你是连接测试助手。", "只回复 OK。", 1024)
		return aiTaskResult{Text: "连接成功，当前密钥和模型可用"}, e
	})
}
func (a *app) forgetAIProvider() {
	if a.ai.Busy || a.ai.Saving {
		return
	}
	c := defaultAIConfig()
	c.Selected = a.ai.Config.Selected
	c.Enabled = a.ai.Config.Enabled
	c.AutoJudge = a.ai.Config.AutoJudge
	for k, v := range a.ai.Config.Profiles {
		c.Profiles[k] = v
	}
	p := c.Profiles[a.ai.EditorProvider]
	p.Key = ""
	c.Profiles[a.ai.EditorProvider] = p
	filename := a.ai.Path
	a.startAITask("forget", func(context.Context) (aiTaskResult, error) {
		if e := saveAIConfig(filename, c); e != nil {
			return aiTaskResult{}, e
		}
		return aiTaskResult{Text: "已删除该厂商的已保存密钥", Config: &c}, nil
	})
	a.ai.Editor.Key = ""
}
func (a *app) aiFailure(e error) { a.ai.Error = true; a.ai.Message = e.Error() }
func (a *app) pickAIMaterial() {
	if a.ai.Busy || a.ai.Saving || a.win == nil {
		return
	}
	pick := a.win
	a.startAITask("material", func(ctx context.Context) (aiTaskResult, error) { return a.readPickedAIMaterial(ctx, pick) })
}
func (a *app) aiStatusText() string {
	if !a.ai.Config.Enabled && !a.ai.Busy && a.ai.Message == "" {
		return "AI（Beta）已关闭；可先保存配置，再到设置中启用。"
	}
	if a.ai.Busy {
		return a.ai.Message
	}
	if a.ai.Message != "" {
		return a.ai.Message
	}
	p, _ := providerByID(a.ai.Config.Selected)
	return fmt.Sprintf("当前服务：%s · %s", p.Name, a.ai.Config.Profiles[p.ID].Model)
}

// toggleAIFeatures is an explicit opt-in. Disabling cancels network work before persisting the switch.
func (a *app) toggleAIFeatures() {
	if a.ai.Saving || a.aiConfigWriting() {
		return
	}
	enabled := !a.ai.Config.Enabled
	if a.ai.Busy {
		a.cancelAITask()
	}
	c := defaultAIConfig()
	c.Selected = a.ai.Config.Selected
	c.AutoJudge = a.ai.Config.AutoJudge
	c.Enabled = enabled
	for k, v := range a.ai.Config.Profiles {
		c.Profiles[k] = v
	}
	filename := a.ai.Path
	a.startAITask("toggle", func(context.Context) (aiTaskResult, error) {
		if e := saveAIConfig(filename, c); e != nil {
			return aiTaskResult{}, e
		}
		text := "AI（Beta）已关闭"
		if enabled {
			text = "AI（Beta）已开启，请配置服务商后使用"
		}
		return aiTaskResult{Text: text, Config: &c}, nil
	})
}
