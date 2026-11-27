package diagram

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

var allowedTags = wordSet("svg g path rect circle ellipse line polyline polygon text tspan title desc")
var allowedAttrs = wordSet("viewBox width height x y x1 y1 x2 y2 cx cy r rx ry dx dy d points transform fill fill-rule fill-opacity stroke stroke-opacity stroke-width stroke-linecap stroke-linejoin stroke-dasharray opacity font-size font-family font-weight text-anchor dominant-baseline preserveAspectRatio")

func wordSet(input string) map[string]bool {
	m := map[string]bool{}
	for _, word := range strings.Fields(input) {
		m[word] = true
	}
	return m
}

// SanitizeSVG accepts only local drawing primitives. No documents, scripting,
// CSS, external sources, nested SVG canvases or recursive references are used.
func SanitizeSVG(raw string) (string, error) {
	invalid := errors.New("SVG 配图包含不支持的元素或尺寸")
	if len(raw) == 0 || len(raw) > MaxBytes {
		return "", invalid
	}
	var buf bytes.Buffer
	dec := xml.NewDecoder(strings.NewReader(raw))
	enc := xml.NewEncoder(&buf)
	depth, count := 0, 0
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", invalid
		}
		switch t := token.(type) {
		case xml.StartElement:
			if !allowedTags[t.Name.Local] || (depth == 0) != (t.Name.Local == "svg") || depth >= 12 || count >= 500 {
				return "", invalid
			}
			if depth == 0 && count > 0 {
				return "", invalid
			}
			depth++
			count++
			t.Name.Space = ""
			attrs := make([]xml.Attr, 0, len(t.Attr)+1)
			if depth == 1 {
				attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/svg"})
			}
			for _, a := range t.Attr {
				if a.Name.Space != "" || !allowedAttrs[a.Name.Local] || strings.Contains(strings.ToLower(a.Value), "url(") {
					continue
				}
				if (a.Name.Local == "width" || a.Name.Local == "height") && depth == 1 {
					n, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(a.Value), "px"), 64)
					if err != nil || !finite(n) || n <= 0 || n > 4096 {
						continue
					}
				}
				if strings.EqualFold(a.Value, "currentColor") && (a.Name.Local == "fill" || a.Name.Local == "stroke") {
					a.Value = "#27313e"
				}
				attrs = append(attrs, a)
			}
			t.Attr = attrs
			token = t
		case xml.EndElement:
			depth--
			t.Name.Space = ""
			token = t
		case xml.CharData:
			if depth == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return "", invalid
				}
				continue
			}
		case xml.Comment:
			continue
		case xml.ProcInst:
			if depth == 0 && count == 0 && t.Target == "xml" {
				continue
			}
			return "", invalid
		default:
			return "", invalid
		}
		if enc.EncodeToken(token) != nil {
			return "", invalid
		}
	}
	if depth != 0 || count < 2 || enc.Flush() != nil {
		return "", invalid
	}
	return buf.String(), nil
}

// FromMeta handles both original formats. Structured data is parsed against the
// same limits used by AI output and spreadsheet import.
func FromMeta(meta map[string]any) (string, error) {
	var err error
	if raw, ok := meta["diagramSvg"].(string); ok && raw != "" {
		var svg string
		svg, err = SanitizeSVG(raw)
		if err == nil {
			return svg, nil
		}
	}
	if data, ok := meta["diagram"]; ok && data != nil {
		raw, e := json.Marshal(data)
		if e != nil {
			return "", e
		}
		cfg, e := Parse(raw)
		if e != nil {
			return "", e
		}
		return cfg.SVG()
	}
	return "", err
}
