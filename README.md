# Quesora Native

[![Windows CI](https://github.com/ispacewang/quesora-native/actions/workflows/windows.yml/badge.svg)](https://github.com/ispacewang/quesora-native/actions/workflows/windows.yml)
[![Release](https://img.shields.io/github/v/release/ispacewang/quesora-native?include_prereleases)](https://github.com/ispacewang/quesora-native/releases)
[![Listed on Awesome MyGo](https://awesomemygo.dev/badge.svg)](https://awesomemygo.dev/apps/quesora-native)
![Windows x64](https://img.shields.io/badge/platform-Windows%20x64-4a7dbf)
[![Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-4a7dbf)](LICENSE)

Quesora 的 Go + [MyGo](https://github.com/egoist/mygo) 原生 Windows 刷题应用。

![主界面](docs/images/preview-light.png)

## 下载与使用

从 [Releases](https://github.com/ispacewang/quesora-native/releases) 下载 Windows 10/11 x64 安装包，或解压便携 ZIP 运行。当前为预览版。

安装器支持自定义目录和覆盖升级，保留题库与学习记录。点击「上传题库」导入 XLSX、XLS 或 CSV；外观、备份恢复和检查更新位于「设置」。

AI 助手为 Beta，默认关闭；开启后需自行配置服务商 API Key，调用可能计费。详见 [AI 使用说明](docs/ai.md)。

## 从源码构建

需要 Windows x64、go.mod 指定版本的 Go 和 Python 3.10+。

```powershell
git clone https://github.com/ispacewang/quesora-native.git
cd quesora-native
python build-preview.py
```

产物位于 build/release/。构建安装包需安装 Inno Setup 6.7.1+，然后运行 `python build-preview.py --installer`。

## 文档与许可证

[使用说明](docs/usage.md) · [开发导览](docs/development.md) · [更新记录](CHANGELOG.md)

[Apache-2.0](LICENSE)，第三方许可见 [NOTICE](NOTICE)。
