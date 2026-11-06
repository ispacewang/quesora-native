// 原生 AI 助手：沿用现有主题、间距和模态布局，底部操作固定、正文独立滚动。
package main

import (
	"embed"
	"fmt"
	"github.com/egoist/mygo/ui"
	"sync"
)

//go:embed resources/providers/*.svg
var aiIconFiles embed.FS
var aiBrandIcons map[string]*ui.SVG
var aiIconsOnce sync.Once

func aiProviderIcon(id string) *ui.SVG {
	aiIconsOnce.Do(func() {
		aiBrandIcons = map[string]*ui.SVG{}
		for _, p := range aiProviders {
			raw, e := aiIconFiles.ReadFile("resources/providers/" + p.ID + ".svg")
			if e == nil {
				aiBrandIcons[p.ID] = ui.MustParseSVG(raw)
			}
		}
	})
	return aiBrandIcons[id]
}

var aiSparkIcon = svgIcon(`<path d="m12 3 2.8 6.2L21 12l-6.2 2.8L12 21l-2.8-6.2L3 12l6.2-2.8z"/>`)

func aiField(c *ui.Context, label string, value *string, password bool) {
	ui.Column(c.Key(label)).FillWidth().Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500)
		input := ui.TextInput(c.Key("input"), value).Label(label).FillWidth().Height(36).Radius(0).FontSize(13)
		if password {
			input.Password()
		}
	})
}
func (a *app) aiDialogs(c *ui.Context) {
	t := c.Theme()
	_, h := c.Size()
	wasOpen := a.ai.Open
	nativeModal(c.Key("ai-assistant"), &a.ai.Open, func() {
		ui.Row(c).FillWidth().Padding(20, 24, 12).Gap(10).AlignItems(ui.Center).Children(func() {
			icon(c, aiSparkIcon, 20)
			ui.Text(c, "AI 助手 · Beta").FontSize(18).FontWeight(600)
			ui.Spacer(c)
			p, _ := providerByID(a.ai.Config.Selected)
			ui.Icon(c, aiProviderIcon(p.ID)).Size(18, 18)
			ui.Text(c, p.Name).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Row(c).FillWidth().Padding(0, 24, 12).Gap(8).Wrap().Children(func() {
			for _, tab := range []struct{ ID, Name string }{{"generate", "出题"}, {"diagnose", "学习诊断"}, {"settings", "服务商配置"}} {
				style := ghostButton
				if a.ai.Tab == tab.ID {
					style = outlineButton
				}
				control(c, "tab-"+tab.ID, tab.Name, nil, style, func() { a.openAI(tab.ID); a.ai.Message = ""; a.ai.Error = false }).Disabled(a.ai.Busy || a.ai.Saving || !a.ai.Config.Enabled && tab.ID != "settings").Height(28)
			}
			if a.ai.JudgeQuestion.ID != "" {
				control(c, "tab-judge", "单题复核", nil, outlineButton, func() { a.ai.Tab = "judge" }).Disabled(a.ai.Busy || a.ai.Saving).Height(28)
			}
		})
		ui.Scroll(c.Key("ai-body-" + a.ai.Tab)).FillWidth().MaxHeight(max(120, h-260)).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 24, 20).Gap(16).Disabled(a.ai.Busy || a.ai.Saving).Children(func() {
				switch a.ai.Tab {
				case "settings":
					a.aiSettingsBody(c)
				case "diagnose":
					a.aiDiagnosisBody(c)
				case "judge":
					a.aiJudgmentBody(c)
				default:
					a.aiGenerateBody(c)
				}
			})
		})
		color := t.TextMuted
		if a.ai.Error {
			color = t.Danger
		}
		ui.Text(c, a.aiStatusText()).Label("AI任务状态").FillWidth().FontSize(11).LineHeight(1.4).TextColor(color).Padding(8, 24).MaxHeight(64)
		ui.Row(c).FillWidth().Padding(14, 24).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Gap(8).AlignItems(ui.Center).Children(func() {
			if a.ai.Busy && !a.aiConfigWriting() {
				control(c, "cancel-task", "取消任务", nil, outlineButton, a.cancelAITask).Label("取消AI任务")
			}
			if !a.ai.Busy && a.ai.Tab == "settings" {
				control(c, "forget-key", "删除已存密钥", nil, ghostButton, a.forgetAIProvider).Label("删除AI密钥").TextColor(t.Danger).Disabled(a.ai.Saving || a.ai.Config.Profiles[a.ai.EditorProvider].Key == "")
			}
			ui.Spacer(c)
			if !a.ai.Busy {
				switch a.ai.Tab {
				case "settings":
					control(c, "test", "测试连接", nil, outlineButton, a.testAIConnection).Label("测试AI连接").Disabled(a.ai.Saving || !a.ai.Config.Enabled)
					control(c, "save-settings", "保存配置", nil, primaryButton, a.saveAISettings).Label("保存AI配置").Disabled(a.ai.Saving)
				case "diagnose":
					control(c, "diagnose", "开始诊断", nil, primaryButton, a.diagnoseAI).Label("开始AI诊断").Disabled(a.ai.Saving)
				case "judge":
					control(c, "apply-judgment", "应用到练习记录", checkIcon, primaryButton, a.applyAIJudgment).Label("应用AI复核").Disabled(a.ai.Saving || a.ai.Judgment == nil)
				default:
					if len(a.ai.Preview) > 0 {
						control(c, "discard", "放弃预览", nil, outlineButton, func() { a.ai.Preview = nil; a.ai.Message = "" }).Disabled(a.ai.Saving)
						control(c, "save-bank", "保存为题库", bookIcon, primaryButton, a.saveAIPreview).Label("保存AI题库").Disabled(a.ai.Saving || a.store == nil)
					} else {
						control(c, "generate", "生成预览", aiSparkIcon, primaryButton, a.generateAIQuestions).Label("生成AI题库预览").Disabled(a.ai.Saving || a.importBusy)
					}
				}
			}
			control(c, "close-ai", "关闭", nil, outlineButton, a.closeAI).Label("关闭AI助手").Disabled(a.ai.Saving || a.aiConfigWriting())
		})
	}).Width(680).Padding(0).Gap(0).Disabled(a.ai.Saving)
	if wasOpen && !a.ai.Open {
		if a.ai.Saving || a.aiConfigWriting() {
			a.ai.Open = true
		} else {
			a.closeAI()
		}
	}
}

func (a *app) aiSettingsBody(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "选择服务商").FontSize(12).FontWeight(500)
	ui.Grid(c.Key("ai-providers")).FillWidth().Columns(5).Gap(8).Children(func() {
		for _, p := range aiProviders {
			box := ui.ButtonBase(c.Key(p.ID)).Label("AI服务商："+p.Name).MinWidth(0).Height(72).Border(1, t.Border).Background(t.Background).OnClick(func() { a.selectAIProvider(p.ID) })
			if p.ID == a.ai.EditorProvider {
				box.BorderColor(t.Accent).Background(t.Accent.Alpha(.06))
			}
			box.Children(func() {
				ui.Column(c).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, aiProviderIcon(p.ID)).Size(24, 24)
					ui.Text(c, p.Name).FontSize(11).FontWeight(500).SingleLine()
				})
			})
		}
	})
	settingRow(c, "提交时使用 AI 判题", "日常练习提交后交给当前模型判题；失败可重试。考试仍按本地标准答案评分。", a.ai.EditorAuto, func() { a.ai.EditorAuto = !a.ai.EditorAuto })
	aiField(c, "API 地址", &a.ai.Editor.BaseURL, false)
	aiField(c, "模型名称 / 推理接入点 ID", &a.ai.Editor.Model, false)
	aiField(c, "API Key", &a.ai.Editor.Key, true)
	ui.Text(c, "密钥仅使用 Windows 当前用户加密保存在本机，不包含在题库备份中。切换厂商后请分别保存配置。").FillWidth().FontSize(11).LineHeight(1.6).TextColor(t.TextMuted)
	p, _ := providerByID(a.ai.EditorProvider)
	if p.ID == "qwen" {
		ui.Text(c, "千问密钥与地域对应；可填写百炼控制台提供的业务空间专属地址。").FillWidth().FontSize(11).TextColor(t.TextMuted)
	}
	if p.ID == "doubao" {
		ui.Text(c, "豆包填写已开通模型的 Model ID，或控制台创建的 ep- 推理接入点 ID。").FillWidth().FontSize(11).TextColor(t.TextMuted)
	}
	ui.Text(c, "测试连接会发送一条极短请求，可能产生少量费用。").FillWidth().FontSize(11).TextColor(t.TextMuted)
}

// aiGenerationProviderSelect 使用 MyGo 原生选择器，收起时只占一行并保留键盘导航。
func (a *app) aiGenerationProviderSelect(c *ui.Context) {
	t := c.Theme()
	sel := ui.SelectBase(c.Key("generation-provider"), &a.ai.Config.Selected)
	trigger := sel.Trigger.Label("选择出题服务商").FillWidth().MinWidth(0).Height(36).Padding(0, 12).Border(1, t.Border).Background(t.Background).Radius(0).OnChange(func() { a.selectAIGenerationProvider(a.ai.Config.Selected) })
	if trigger.Hovered() {
		trigger.BorderColor(t.TextMuted.Alpha(.5))
	}
	if trigger.Focused() || sel.Open() {
		trigger.BorderColor(t.Accent)
	}
	trigger.Children(func() {
		p, _ := providerByID(a.ai.Config.Selected)
		ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, aiProviderIcon(p.ID)).Size(18, 18).Shrink(0)
			ui.Text(c, p.Name).Grow(1).MinWidth(0).SingleLine().FontSize(13)
			ui.Icon(c, rightIcon).Rotate(90).Size(12, 12).Shrink(0).TextColor(t.TextMuted)
		})
	})
	sel.Popup(func(panel ui.Element) {
		panel.Padding(4).Border(1, t.Border).Radius(0).Background(t.Background).Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, .12))
		ui.Scroll(c).FillWidth().MaxHeight(220).Children(func() {
			for _, p := range aiProviders {
				item := sel.Item(p.ID).Label("出题服务商："+p.Name).FillWidth().Padding(8, 10)
				if item.Highlighted() {
					item.Background(t.Surface)
				}
				item.Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, aiProviderIcon(p.ID)).Size(16, 16).Shrink(0)
						ui.Text(c, p.Name).Grow(1).MinWidth(0).SingleLine().FontSize(13)
						status := "未配置"
						if a.ai.Config.Profiles[p.ID].Key != "" {
							status = "已配置"
						}
						ui.Text(c, status).FontSize(11).TextColor(t.TextMuted)
						if p.ID == a.ai.Config.Selected {
							ui.Icon(c, checkIcon).Size(13, 13).Shrink(0).TextColor(t.Accent)
						}
					})
				})
			}
		})
	})
}

func (a *app) aiGenerateBody(c *ui.Context) {
	t := c.Theme()
	if len(a.ai.Preview) > 0 {
		a.aiPreviewBody(c)
		return
	}
	ui.Column(c.Key("generation-provider-field")).FillWidth().Gap(6).Children(func() {
		ui.Text(c, "出题服务商").FontSize(12).FontWeight(500)
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() { a.aiGenerationProviderSelect(c) })
			control(c, "configure-generation-provider", "配置此服务商", settingsIcon, outlineButton, func() { a.openAI("settings") }).Label("配置出题服务商").Height(36)
		})
		profile := a.ai.Config.Profiles[a.ai.Config.Selected]
		status := "未配置"
		if profile.Key != "" {
			status = "已配置"
		}
		ui.Text(c, "当前模型："+profile.Model+" · "+status).FillWidth().FontSize(11).TextColor(t.TextMuted)
	})
	ui.Text(c, "根据主题或参考资料生成标准题目").FontSize(14).FontWeight(600)
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() { aiField(c, "新题库名称", &a.ai.Bank, false) })
		ui.Column(c).Width(100).Children(func() { aiField(c, "题量（1–500）", &a.ai.Count, false) })
	})
	aiField(c, "出题主题", &a.ai.Topic, false)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "参考资料（可选）").FontSize(12).FontWeight(500)
		ui.Spacer(c)
		control(c, "attach", "添加附件", uploadIcon, outlineButton, a.pickAIMaterial).Label("添加AI参考资料").Height(28)
		if a.ai.Material != "" {
			control(c, "clear-material", "清除", nil, ghostButton, func() { a.ai.Material = ""; a.ai.MaterialName = "" }).Height(28)
		}
	})
	if a.ai.MaterialName != "" {
		ui.Text(c, a.ai.MaterialName).FillWidth().FontSize(11).TextColor(t.Accent)
	}
	ui.TextArea(c.Key("ai-material"), &a.ai.Material).Label("AI参考资料正文").FillWidth().Height(110).FontSize(12).Radius(0).Placeholder("可粘贴资料，或添加 TXT、Markdown、CSV、Excel、DOCX、文本 PDF（16 MiB / 60000 字以内）")
	ui.Text(c, "点击生成后，主题和参考资料将发送至当前服务商，可能产生费用。生成结果先预览，确认保存后才会写入题库。").FillWidth().FontSize(11).LineHeight(1.6).TextColor(t.TextMuted)
}
func (a *app) aiPreviewBody(c *ui.Context) {
	t := c.Theme()
	i := min(max(a.ai.PreviewIndex, 0), len(a.ai.Preview)-1)
	q := a.ai.Preview[i]
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf("预览 %d / %d · %s", i+1, len(a.ai.Preview), q.Kind)).FontSize(12).FontWeight(600)
		ui.Spacer(c)
		control(c, "preview-prev", "上一题", leftIcon, outlineButton, func() { a.ai.PreviewIndex-- }).Disabled(i == 0)
		control(c, "preview-next", "下一题", rightIcon, outlineButton, func() { a.ai.PreviewIndex++ }).Disabled(i+1 == len(a.ai.Preview))
	})
	ui.Text(c, q.Prompt).FillWidth().FontSize(15).FontWeight(600).LineHeight(1.6)
	for i, opt := range q.Options {
		ui.Text(c, fmt.Sprintf("%c. %s", 'A'+i, opt)).FillWidth().FontSize(13).Padding(8, 12).Background(t.Surface)
	}
	ans := q.TextAnswer
	if !isTextQuestion(q) {
		ans = letters(q.Answer)
	}
	ui.Text(c, "答案："+ans).FillWidth().FontSize(13).TextColor(t.Success)
	ui.Text(c, q.Explanation).FillWidth().FontSize(12).LineHeight(1.6).TextColor(t.TextMuted)
}
func (a *app) aiDiagnosisBody(c *ui.Context) {
	good, bad := a.counts()
	ui.Text(c, "当前题库："+a.bank).FillWidth().FontSize(14).FontWeight(600)
	ui.Text(c, fmt.Sprintf("已答 %d 题 · 正确 %d · 错误 %d", good+bad, good, bad)).FillWidth().FontSize(12).TextColor(c.Theme().TextMuted)
	ui.Text(c, "仅发送当前题库名称和各题型答题统计，分析薄弱题型及下一步学习建议。").FillWidth().FontSize(12).LineHeight(1.6).TextColor(c.Theme().TextMuted)
	if a.ai.Result != "" {
		ui.Text(c, a.ai.Result).FillWidth().FontSize(14).LineHeight(1.7).Selectable()
	}
}
func (a *app) aiJudgmentBody(c *ui.Context) {
	ui.Text(c, a.ai.JudgeQuestion.Prompt).FillWidth().FontSize(14).FontWeight(600).LineHeight(1.6)
	ui.Text(c, "复核会发送本题题干、选项、参考答案和本次作答。AI 判断仅供参考，确认应用后才更新练习记录。").FillWidth().FontSize(11).LineHeight(1.6).TextColor(c.Theme().TextMuted)
	if a.ai.Judgment != nil {
		label := "AI 认为：需订正"
		color := c.Theme().Danger
		if a.ai.Judgment.Correct {
			label = "AI 认为：回答正确"
			color = c.Theme().Success
		}
		ui.Text(c, label).FillWidth().FontSize(14).FontWeight(600).TextColor(color)
		ui.Text(c, a.ai.Judgment.Explanation).FillWidth().FontSize(13).LineHeight(1.7).Selectable()
	}
}
