// 内嵌字体、软件标志和重要题资源的单次初始化，成品无需依赖系统中文字体安装情况。

package main

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"github.com/egoist/mygo/ui"
	"io"
	"sync"
)

//go:embed resources/fonts/*.ttf.gz resources/app-icon.png resources/zhongyao-*.png
var nativeAssets embed.FS
var fontOnce sync.Once
var appIcon *ui.Bitmap

// loadNativeAssets 幂等注册内嵌字体并解码固定资源；构建期资源缺失视为程序打包错误。
func loadNativeAssets() {
	fontOnce.Do(func() {
		for _, weight := range []int{400, 500, 600, 700} {
			data, err := nativeAssets.ReadFile(fmt.Sprintf("resources/fonts/noto-%d.ttf.gz", weight))
			if err != nil {
				panic(err)
			}
			reader, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				panic(err)
			}
			font, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				panic(err)
			}
			if err = ui.RegisterFont(font, "Noto Sans SC"); err != nil {
				panic(err)
			}
		}
		data, err := nativeAssets.ReadFile("resources/app-icon.png")
		if err != nil {
			panic(err)
		}
		loadImportantAssets()
		appIcon, err = ui.DecodeBitmap(data)
		if err != nil {
			panic(err)
		}
	})
}
