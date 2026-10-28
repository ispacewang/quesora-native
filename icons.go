package main

import "github.com/egoist/mygo/ui"

func svgIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	downloadIcon = svgIcon(`<path d="M21 15v4H3v-4m9-12v12m-5-5 5 5 5-5"/>`)
	bookIcon     = svgIcon(`<path d="M12 7v14m0-14C9 4 4 4 2 5v15c3-1 7-1 10 1 3-2 7-2 10-1V5c-3-1-7-1-10 2z"/>`)
	closeIcon    = svgIcon(`<path d="M18 6 6 18M6 6l12 12"/>`)
	moonIcon     = svgIcon(`<path d="M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13z"/>`)
	sunIcon      = svgIcon(`<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>`)
	minusIcon    = svgIcon(`<path d="M5 12h14"/>`)
	squareIcon   = svgIcon(`<rect x="5" y="5" width="14" height="14"/>`)
	checkIcon    = svgIcon(`<path d="m20 6-11 11-5-5"/>`)
	helpIcon     = svgIcon(`<circle cx="12" cy="12" r="10"/><path d="M9.1 9a3 3 0 0 1 5.8 1c0 2-3 3-3 3m0 4h.01"/>`)
	infoIcon     = svgIcon(`<circle cx="12" cy="12" r="10"/><path d="M12 16v-4m0-4h.01"/>`)
	chartIcon    = svgIcon(`<path d="M3 3v18h18m-2-12-5 5-4-4-3 3"/>`)
	pulseIcon    = svgIcon(`<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>`)
	uploadIcon   = svgIcon(`<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>`)
	shuffleIcon  = svgIcon(`<path d="m18 14 4 4-4 4m0-20 4 4-4 4M2 18h2c5 0 9-12 14-12h4M2 6h2c2 0 4 2 6 6m4 4c2 2 3 2 8 2"/>`)
	zapIcon      = svgIcon(`<path d="m13 2-10 12h8l-1 8 11-12h-8z"/>`)
	leftIcon     = svgIcon(`<path d="m15 18-6-6 6-6"/>`)
	rightIcon    = svgIcon(`<path d="m9 18 6-6-6-6"/>`)
	historyIcon  = svgIcon(`<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>`)
	plusIcon     = svgIcon(`<path d="M12 5v14M5 12h14"/>`)
	eyeIcon      = svgIcon(`<path d="M2 12s4-8 10-8 10 8 10 8-4 8-10 8S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`)
)

func icon(c *ui.Context, s *ui.SVG, size float32) { ui.Icon(c, s).Size(size, size).Shrink(0) }
