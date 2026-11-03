// Command icons adapts the reviewed square master to clean Windows/runtime PNG assets.
// The master includes generator padding; the fixed crop/clip is part of this asset's export specification.
package main

import (
	"fmt"
	"golang.org/x/image/draw"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func exportIcon(src image.Image, size int, filename string) error {
	bounds := src.Bounds()
	if bounds.Dx() != bounds.Dy() {
		return fmt.Errorf("icon master must be square")
	}
	// Crop inside the approved tile to exclude stray pixels in generated transparent padding.
	inset := int(math.Round(float64(bounds.Dx()) * .085))
	crop := image.Rect(bounds.Min.X+inset, bounds.Min.Y+inset, bounds.Max.X-inset, bounds.Max.Y-inset)
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	padding := int(math.Round(float64(size) * .045))
	tile := image.Rect(padding, padding, size-padding, size-padding)
	draw.CatmullRom.Scale(dst, tile, src, crop, draw.Src, nil)
	left, top, right, bottom := float64(tile.Min.X), float64(tile.Min.Y), float64(tile.Max.X), float64(tile.Max.Y)
	radius := (right - left) * .20
	// Supersample only the geometric export mask; preserve the reviewed master colors inside it.
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			covered := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					px, py := float64(x)+(float64(sx)+.5)/4, float64(y)+(float64(sy)+.5)/4
					if px < left || px >= right || py < top || py >= bottom {
						continue
					}
					cx := math.Max(left+radius, math.Min(right-radius, px))
					cy := math.Max(top+radius, math.Min(bottom-radius, py))
					if (px-cx)*(px-cx)+(py-cy)*(py-cy) <= radius*radius {
						covered++
					}
				}
			}
			v := dst.NRGBAAt(x, y)
			v.A = uint8(int(v.A) * covered / 16)
			if v.A == 0 {
				v = color.NRGBA{}
			}
			dst.SetNRGBA(x, y, v)
		}
	}
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	if err = png.Encode(out, dst); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
func main() {
	if len(os.Args) != 4 {
		panic("usage: icons master.png windows.png runtime.png")
	}
	f, e := os.Open(os.Args[1])
	if e != nil {
		panic(e)
	}
	src, e := png.Decode(f)
	f.Close()
	if e != nil {
		panic(e)
	}
	for _, out := range []struct {
		path string
		size int
	}{{os.Args[2], 256}, {os.Args[3], 128}} {
		if e = exportIcon(src, out.size, out.path); e != nil {
			panic(e)
		}
	}
}
