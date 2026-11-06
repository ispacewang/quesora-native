// Command icons resizes the user-provided artwork for Windows and the native titlebar.
// Every size preserves the full questionnaire and Q bubble from the same master.
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

// fullIcon scales the whole reference once, then clips only the outer tile corners.
// Do not replace small sizes with a different mark or crop the questionnaire details.
func fullIcon(src image.Image, size int) (*image.NRGBA, error) {
	bounds := src.Bounds()
	if bounds.Dx() != bounds.Dy() {
		return nil, fmt.Errorf("icon master must be square")
	}
	// Preserve the original composition, including its internal margins.
	crop := bounds
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
	runtime, err := fullIcon(src, 128)
	if err != nil {
		panic(err)
	}
	if err = writePNG(os.Args[3], runtime); err != nil {
		panic(err)
	}
	if err = os.MkdirAll(os.Args[4], 0755); err != nil {
		panic(err)
	}
	// Resize each DPI frame directly from the high-resolution source, never another PNG.
	for _, size := range []int{16, 20, 24, 32, 40, 48, 60, 64, 72, 96, 128, 256} {
		img, err := fullIcon(src, size)
		if err != nil {
			panic(err)
		}
		if err = writePNG(filepath.Join(os.Args[4], fmt.Sprintf("icon-%d.png", size)), img); err != nil {
			panic(err)
		}
	}
}
