// Command icons exports optical-size Windows icons and the native titlebar mark.
// Small frames use a geometric Q: questionnaire details cannot survive at desktop sizes.
package main

import (
	"fmt"
	"golang.org/x/image/draw"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func fullIcon(src image.Image, size int) (*image.NRGBA, error) {
	bounds := src.Bounds()
	if bounds.Dx() != bounds.Dy() {
		return nil, fmt.Errorf("icon master must be square")
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
	return dst, nil
}

// smallIcon rasterizes at the requested size, avoiding an intermediate bitmap resize.
// Its thick Q and generous counter stay legible beside the Windows shortcut overlay.
func smallIcon(size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	const samples = 8
	s := float64(size)
	padding := math.Max(1, math.Round(s*.045))
	radius := (s - 2*padding) * .20
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var red, green, blue, alpha float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px, py := float64(x)+(float64(sx)+.5)/samples, float64(y)+(float64(sy)+.5)/samples
					if px < padding || py < padding || px >= s-padding || py >= s-padding {
						continue
					}
					cx := math.Max(padding+radius, math.Min(s-padding-radius, px))
					cy := math.Max(padding+radius, math.Min(s-padding-radius, py))
					if math.Hypot(px-cx, py-cy) > radius {
						continue
					}
					u, v := px/s, py/s
					outer := math.Pow((u-.49)/.255, 2)+math.Pow((v-.465)/.28, 2) <= 1
					inner := math.Pow((u-.49)/.135, 2)+math.Pow((v-.465)/.165, 2) < 1
					// A rounded diagonal tail makes the glyph a Q without fine line details.
					ax, ay, bx, by := .555, .555, .735, .745
					t := math.Max(0, math.Min(1, ((u-ax)*(bx-ax)+(v-ay)*(by-ay))/((bx-ax)*(bx-ax)+(by-ay)*(by-ay))))
					tail := math.Hypot(u-ax-t*(bx-ax), v-ay-t*(by-ay)) <= .065
					r, g, b := 255.0, 151-58*v, 10.0
					if (outer && !inner) || tail {
						r, g, b = 255, 255, 255
					}
					red += r
					green += g
					blue += b
					alpha++
				}
			}
			if alpha > 0 {
				dst.SetNRGBA(x, y, color.NRGBA{uint8(math.Round(red / alpha)), uint8(math.Round(green / alpha)), uint8(math.Round(blue / alpha)), uint8(math.Round(alpha * 255 / (samples * samples)))})
			}
		}
	}
	return dst
}

func writePNG(filename string, img image.Image) error {
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	if err = png.Encode(out, img); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func main() {
	if len(os.Args) != 5 {
		panic("usage: icons master.png windows.png runtime.png frames-directory")
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		panic(err)
	}
	large, err := fullIcon(src, 256)
	if err != nil {
		panic(err)
	}
	if err = writePNG(os.Args[2], large); err != nil {
		panic(err)
	}
	if err = writePNG(os.Args[3], smallIcon(128)); err != nil {
		panic(err)
	}
	if err = os.MkdirAll(os.Args[4], 0755); err != nil {
		panic(err)
	}
	// Include common Windows 100–300% DPI sizes, each with its own direct rasterization.
	for _, size := range []int{16, 20, 24, 32, 40, 48, 60, 64, 72, 96, 128, 256} {
		var img image.Image = smallIcon(size)
		if size >= 128 {
			img, err = fullIcon(src, size)
			if err != nil {
				panic(err)
			}
		}
		if err = writePNG(filepath.Join(os.Args[4], fmt.Sprintf("icon-%d.png", size)), img); err != nil {
			panic(err)
		}
	}
}
