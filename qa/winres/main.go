package main

import (
	"fmt"
	"github.com/tc-hib/winres"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// main 接收已按实际尺寸绘制的帧目录、目标 syso 和可选 ICO 路径。
// 不在资源打包阶段缩放；EXE 和安装器必须共用同一组光学尺寸图稿。
func main() {
	if len(os.Args) < 3 || len(os.Args) > 4 {
		panic("usage: winres frames-directory output.syso [output.ico]")
	}
	var frames []image.Image
	for _, size := range []int{16, 20, 24, 32, 40, 48, 60, 64, 72, 96, 128, 256} {
		f, err := os.Open(filepath.Join(os.Args[1], fmt.Sprintf("icon-%d.png", size)))
		must(err)
		img, err := png.Decode(f)
		f.Close()
		must(err)
		if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
			panic("unexpected icon frame dimensions")
		}
		frames = append(frames, img)
	}
	icon, err := winres.NewIconFromImages(frames)
	must(err)
	rs := winres.ResourceSet{}
	must(rs.SetIcon(winres.ID(1), icon))
	rs.SetManifest(winres.AppManifest{Description: "Quesora Native", DPIAwareness: winres.DPIPerMonitorV2, Compatibility: winres.Win10AndAbove, UseCommonControlsV6: true, LongPathAware: true})
	out, err := os.Create(os.Args[2])
	must(err)
	must(rs.WriteObject(out, winres.ArchAMD64))
	must(out.Close())

	if len(os.Args) > 3 {
		ico, err := os.Create(os.Args[3])
		must(err)
		must(icon.SaveICO(ico))
		must(ico.Close())
	}
}
