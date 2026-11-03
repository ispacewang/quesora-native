// 区分安装版、便携版与源码运行的数据位置。安装版优先使用当前用户目录，不依赖工作目录。

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const installedMarker = ".quesora-native-installed"

// dataLocation 同时描述当前数据目录和允许迁移的同安装目录旧库，不扫描其他用户目录。
type dataLocation struct {
	Dir       string
	LegacyDB  string
	Installed bool
}

// resolveDataLocation 优先检查安装标记；安装版必须获得绝对用户目录，便携/源码版保留原有定位规则。
func resolveDataLocation(exe, cwd, localData string) (dataLocation, error) {
	dir := filepath.Dir(exe)
	if info, err := os.Stat(filepath.Join(dir, installedMarker)); err == nil && info.Mode().IsRegular() {
		if !filepath.IsAbs(localData) {
			return dataLocation{}, fmt.Errorf("无法确定当前用户的数据目录，请使用 -data-dir 指定可写目录")
		}
		return dataLocation{Dir: filepath.Join(localData, "QuesoraNative", "data"), LegacyDB: filepath.Join(dir, "data", "questions.db"), Installed: true}, nil
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "mygo.json")); err == nil {
		dir = filepath.Dir(dir)
	}
	if _, err := os.Stat(filepath.Join(cwd, "mygo.json")); err == nil {
		dir = cwd
	}
	return dataLocation{Dir: filepath.Join(dir, "data")}, nil
}

func defaultDataLocation() (dataLocation, error) {
	exe, err := os.Executable()
	if err != nil {
		return dataLocation{}, err
	}
	cwd, _ := os.Getwd()
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local, _ = os.UserConfigDir()
	}
	return resolveDataLocation(exe, cwd, local)
}

// prepareInstalledData 仅在目标库不存在时创建旧库的一致性快照；源文件保留，已有目标绝不覆盖。
func prepareInstalledData(location dataLocation) error {
	if !location.Installed {
		return nil
	}
	target := filepath.Join(location.Dir, "questions.db")
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(location.LegacyDB); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	db, err := openReadOnlyDatabase(location.LegacyDB)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = snapshotDatabase(db, target, false); err != nil {
		return fmt.Errorf("迁移旧数据失败，原文件已保留：%w", err)
	}
	return nil
}
