// MyGo 负责段落换行、基线、主题和绘制。题库里的公式只作为排版数据。
package desktop

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/ispacewang/quesora-native/internal/mathtext"
)

type formulaKey struct {
	tex     string
	size    int
	display bool
}
type formulaIcon struct {
	image                  *ui.SVG
	width, ascent, descent float32
}

// Views can rebuild every frame; retain a small FIFO of immutable glyph images,
// including failed parses. Share previews may render concurrently offscreen.
var formulaCache = struct {
	sync.Mutex
	keys  [64]formulaKey
	next  int
	icons map[formulaKey]formulaIcon
}{icons: make(map[formulaKey]formulaIcon, 64)}

func cachedFormula(key formulaKey) formulaIcon {
	formulaCache.Lock()
	defer formulaCache.Unlock()
	if icon, ok := formulaCache.icons[key]; ok {
		return icon
	}
	var icon formulaIcon
	if vector, err := mathtext.Render(key.tex, key.size, key.display); err == nil {
		if svg, err := ui.ParseSVG([]byte(vector.SVG)); err == nil {
			icon = formulaIcon{svg, vector.Width, vector.Ascent, vector.Descent}
		}
	}
	i := formulaCache.next
	delete(formulaCache.icons, formulaCache.keys[i])
	formulaCache.keys[i] = key
	formulaCache.icons[key] = icon
	formulaCache.next = (i + 1) % len(formulaCache.keys)
	return icon
}

func mathText(c *ui.Context, text string, size int, ink ui.Color) ui.Element {
	return mathTextPrefix(c, "", text, size, ink, 300)
}

// Keep answer/option prefixes out of auto-detection. Otherwise an isolated
// formula stops being recognizable as soon as it has a Chinese label.
func mathTextPrefix(c *ui.Context, prefix, text string, size int, ink ui.Color, maxWidth float32) ui.Element {
	raw := prefix + text
	parts := mathtext.Split(text)
	if len(parts) == 0 {
		return ui.Text(c, raw).FontSize(float32(size)).TextColor(ink)
	}
	var spans []ui.Span
	var icons map[int]*ui.SVG
	offset := 0
	endsLine := true
	addText := func(s string) {
		if s == "" {
			return
		}
		spans = append(spans, ui.Span{Text: s})
		offset += utf8.RuneCountInString(s)
		endsLine = strings.HasSuffix(s, "\n")
	}
	addText(prefix)
	for i, part := range parts {
		if part.Kind == mathtext.Plain {
			addText(part.Source)
			continue
		}
		display := part.Kind == mathtext.Display
		renderSize := size
		if display {
			renderSize += 2
		}
		icon := cachedFormula(formulaKey{part.TeX, renderSize, display})
		if icon.image == nil {
			addText(part.Source)
			continue
		}
		if display && !endsLine {
			addText("\n")
		}
		if icons == nil {
			icons = map[int]*ui.SVG{}
		}
		icons[offset] = icon.image
		scale := min(1, maxWidth/icon.width, 160/(icon.ascent+icon.descent))
		spans = append(spans, ui.Span{Text: "\uFFFC", ObjectWidth: icon.width * scale, ObjectHeight: icon.ascent * scale, ObjectDescent: icon.descent * scale})
		offset++
		endsLine = false
		if display && i+1 < len(parts) && !strings.HasPrefix(parts[i+1].Source, "\n") && !strings.HasPrefix(parts[i+1].Source, "\r\n") {
			addText("\n")
		}
	}
	if len(icons) == 0 {
		return ui.Text(c, raw).FontSize(float32(size)).TextColor(ink)
	}
	return ui.RichText(c, spans...).Label(raw).FontSize(float32(size)).TextColor(ink).
		DrawObjects(func(p *ui.Painter, at int, r ui.Rect) {
			if icon := icons[at]; icon != nil {
				p.Icon(icon, r, ink)
			}
		})
}
