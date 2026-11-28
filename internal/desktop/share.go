// Shares render a dedicated question-only view. The preview and downloaded PNG
// use the same immutable pixels, with no dependency on the window's state.
package desktop

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/ispacewang/quesora-native/internal/domain"
)

const shareDirectoryKey = "share-download-directory"

type shareState struct {
	open, preparing, saving, picking                                 bool
	generation                                                       uint64
	directory, defaultDirectory, message, settingsMessage, savedPath string
	bitmap                                                           *ui.Bitmap
	pixels                                                           image.Image
	pick                                                             func(string) (string, error)
}

func (s *shareState) destination() string {
	if s.directory != "" {
		return s.directory
	}
	return s.defaultDirectory
}

type shareCard struct {
	prompt  string
	options []string
	figure  figureView
}

func captureShare(q domain.Question, order []int) shareCard {
	card := shareCard{prompt: q.Prompt, figure: loadFigure(q)}
	for _, i := range domain.ResponseOrder(q, domain.Response{OptionOrder: order}) {
		card.options = append(card.options, q.Options[i])
	}
	return card
}

func (s shareCard) view(c *ui.Context) {
	ink, muted := ui.Hex("#28221D"), ui.Hex("#6E584B")
	accent, lightAccent := ui.Hex("#D55C24"), ui.Hex("#F7E5D6")
	ui.Column(c.Key("share-canvas")).Label("分享图正文").FillWidth().Shrink(0).Padding(24).
		Background(ui.Hex("#EEE5DA")).Children(func() {
		ui.Column(c.Key("share-paper")).FillWidth().Padding(36, 40, 42, 40).Radius(20).
			Background(ui.Hex("#FFFDFA")).Children(func() {
			ui.Row(c.Key("share-brand")).FillWidth().AlignItems(ui.Center).Gap(8).Margin(0, 0, 30, 0).Children(func() {
				ui.Box(c.Key("accent")).Size(34, 4).Radius(2).Background(accent)
				ui.Spacer(c)
				if appIcon != nil {
					ui.Image(c, appIcon).Size(18, 18).Fit(ui.Contain).Label("Quesora Logo")
				}
				ui.Text(c, "Quesora").FontSize(13).FontWeight(600).TextColor(muted)
			})
			mathTextPrefix(c, "", s.prompt, 26, ink, 500).FillWidth().FontWeight(600).LineHeight(1.58)
			if s.figure.svg != nil {
				w, h := s.figure.svg.Size()
				ui.Box(c.Key("share-figure")).FillWidth().Height(min(280, max(132, 500*h/w))).Margin(28, 0, 0, 0).
					Background(ui.Hex("#F8F7F4")).Radius(14).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
					const inset = 16
					p.Image(s.figure.svg, ui.Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - 2*inset, H: r.H - 2*inset}, ui.Contain)
				})
			}
			if len(s.options) == 0 {
				return
			}
			ui.Column(c.Key("share-options")).FillWidth().Gap(12).Margin(32, 0, 0, 0).Children(func() {
				for i, opt := range s.options {
					ui.Row(c.Key(i)).FillWidth().Padding(14, 16).Gap(16).AlignItems(ui.Start).
						Background(ui.Hex("#F6F2ED")).Radius(12).Children(func() {
						ui.Box(c).Size(29, 29).Shrink(0).Radius(14.5).Background(lightAccent).Center().Children(func() {
							ui.Text(c, string(rune('A'+i))).FontSize(14).FontWeight(600).TextColor(ui.Hex("#8A3F24"))
						})
						mathTextPrefix(c, "", opt, 18, ink, 420).Grow(1).MinWidth(0).LineHeight(1.6).Margin(1, 0, 0, 0)
					})
				}
			})
		})
	})
}

func renderShare(card shareCard) (*image.RGBA, error) {
	const width = 640
	const ceiling = 4096
	tester := ui.NewTester(card.view, width, ceiling)
	box, ok := tester.Find("分享图正文")
	if !ok || box.H <= 0 || box.H > ceiling {
		return nil, fmt.Errorf("题目太长，无法生成单张图片")
	}
	return ui.Render(card.view, width, int(math.Ceil(float64(box.H))), 2), nil
}

func (a *app) openShare(q domain.Question, order []int) {
	if a.share.open || a.share.saving {
		return
	}
	a.share.generation++
	id := a.share.generation
	a.share.open, a.share.preparing = true, true
	a.share.bitmap, a.share.pixels = nil, nil
	a.share.message, a.share.savedPath = "", ""
	card := captureShare(q, order)
	go func() {
		img, err := renderShare(card)
		a.publish(func() {
			if id != a.share.generation || !a.share.open {
				return
			}
			a.share.preparing = false
			if err != nil {
				a.share.message = err.Error()
				return
			}
			a.share.pixels = img
			a.share.bitmap = ui.NewBitmap(img)
		})
	}()
}

// saveShareImage chooses a unique name independent of the question content.
func saveShareImage(dir string, img image.Image, now time.Time) (string, error) {
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("请在设置里选择下载目录")
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "Quesora-题目-"+now.Format("20060102-150405")+"-*.png")
	if err != nil {
		return "", err
	}
	name := file.Name()
	_, err = file.Write(data.Bytes())
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func (a *app) downloadShare() {
	if !a.share.open || a.share.pixels == nil || a.share.saving {
		return
	}
	img, dir, id := a.share.pixels, a.share.destination(), a.share.generation
	a.share.saving = true
	a.share.message, a.share.savedPath = "", ""
	go func() {
		filename, err := saveShareImage(dir, img, time.Now())
		a.publish(func() {
			a.share.saving = false
			if id != a.share.generation || !a.share.open {
				return
			}
			if err != nil {
				a.share.message = "保存失败。请检查设置里的下载目录和写入权限。"
				return
			}
			a.share.savedPath = filename
			a.share.message = "已保存到下载目录"
		})
	}()
}

func (a *app) shareDialog(c *ui.Context) {
	if !a.share.open {
		a.share.pixels, a.share.bitmap = nil, nil
		return
	}
	_, height := c.Size()
	nativeModal(c.Key("share-dialog"), &a.share.open, func() {
		t := c.Theme()
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "分享题目").FontSize(18).FontWeight(600)
			ui.Spacer(c)
			dialogClose(c, "close-share", &a.share.open)
		})
		previewHeight := max(100, height-220)
		ui.Scroll(c.Key("share-preview")).FillWidth().MaxHeight(previewHeight).Children(func() {
			if a.share.bitmap != nil {
				b := a.share.pixels.Bounds()
				ratio := float32(b.Dx()) / float32(b.Dy())
				ui.Image(c, a.share.bitmap).FillWidth().MaxWidth(min(550, max(335, (previewHeight-6)*ratio))).AspectRatio(ratio).Fit(ui.Contain).AlignSelf(ui.Center).Radius(10).Label("分享图片预览")
			} else if a.share.preparing {
				ui.Text(c, "正在生成预览…").FontSize(13).TextColor(t.TextMuted)
			}
		})
		ui.Text(c, "保存到 "+a.share.destination()).FillWidth().FontSize(11).LineHeight(1.45).TextColor(t.TextMuted)
		if a.share.message != "" {
			color := t.TextMuted
			if a.share.savedPath == "" {
				color = t.Danger
			}
			ui.Text(c, a.share.message).FillWidth().FontSize(12).TextColor(color)
		}
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			if filename := a.share.savedPath; filename != "" && a.win != nil {
				control(c, "show-share", "打开文件位置", nil, outlineButton, func() { mygo.Shell.ShowItemInFolder(filename) })
			}
			label := "下载图片"
			if a.share.saving {
				label = "正在保存…"
			}
			control(c, "download-share", label, nil, primaryButton, a.downloadShare).Disabled(a.share.bitmap == nil || a.share.saving)
		})
	}).Width(624)
}
