package main

import (
	"github.com/tc-hib/winres"
	"image/png"
	"os"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// main 接收源 PNG、目标 syso 和可选 ICO 路径；一次生成 EXE 与安装器共用的多尺寸图标和 DPI 清单。
func main() {
	f, err := os.Open(os.Args[1])
	must(err)
	img, err := png.Decode(f)
	f.Close()
	must(err)
	icon, err := winres.NewIconFromResizedImage(img, []int{16, 24, 32, 48, 64, 128, 256})
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
