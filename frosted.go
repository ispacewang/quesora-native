package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// Only the surface samples and blurs the backdrop; labels paint sharply after it.
var frostedSurface = shader.MustCompile(`package main
var Tint vec4
func Fragment(dst vec4,pos vec2)vec4{
 b:=backdropSample(pos)
 return vec4(mix(b.rgb,Tint.rgb,Tint.a),1)
}
`)

func paintFrostedSurface(p *ui.Painter, r ui.Rect, tint ui.Color) {
	frostedSurface.Draw(p, r, shader.Options{BackdropBlur: 9, Uniforms: map[string]any{"Tint": tint}})
}
