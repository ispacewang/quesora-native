package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const installedMarker = ".quesora-native-installed"

type dataLocation struct {
	Dir       string
	LegacyDB  string
	Installed bool
}

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
