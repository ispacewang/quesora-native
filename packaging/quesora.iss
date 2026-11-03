; Quesora Native: stable identity, per-user upgrades, user data is never packaged.
#ifndef AppVersion
 #error AppVersion is required
#endif
#ifndef NumericVersion
 #error NumericVersion is required
#endif
#ifndef DistDir
 #error DistDir is required
#endif
#ifndef OutputDir
 #error OutputDir is required
#endif
#ifndef IconFile
 #error IconFile is required
#endif

[Setup]
AppId=com.ispace.quesora.native
AppName=Quesora Native
AppVersion={#AppVersion}
AppPublisher=ispacewang
AppPublisherURL=https://github.com/ispacewang/quesora-native
AppSupportURL=https://github.com/ispacewang/quesora-native/issues
AppUpdatesURL=https://github.com/ispacewang/quesora-native/releases
DefaultDirName={localappdata}\Programs\QuesoraNative
DefaultGroupName=Quesora Native
UsePreviousAppDir=yes
UsePreviousTasks=yes
DisableDirPage=auto
DisableWelcomePage=no
AlwaysShowDirOnReadyPage=yes
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
WizardStyle=modern dynamic
WizardSizePercent=110
SetupIconFile={#IconFile}
UninstallDisplayIcon={app}\QuesoraNativePreview.exe
UninstallDisplayName=Quesora Native
VersionInfoVersion={#NumericVersion}
LicenseFile={#DistDir}\LICENSE
CloseApplications=yes
CloseApplicationsFilter=QuesoraNativePreview.exe
RestartApplications=no
Compression=lzma2
SolidCompression=yes
OutputDir={#OutputDir}
OutputBaseFilename=QuesoraNative-{#AppVersion}-setup-windows-x64

[Languages]
Name: "chinesesimp"; MessagesFile: "ChineseSimplified.isl"

[Messages]
WelcomeLabel2=离线导入题库、练习与计时考试。%n%n升级时会沿用上次安装位置并更新程序文件。题库和答题记录独立保存，升级或卸载不会清空学习数据。
FinishedLabel=Quesora Native 已安装完成。%n%n你可以在“设置 → 数据与备份”查看数据位置或导出备份。

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "快捷方式："; Flags: unchecked

[Files]
Source: "{#DistDir}\QuesoraNativePreview.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\NOTICE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\CHANGELOG.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\docs\*"; DestDir: "{app}\docs"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#DistDir}\licenses\*"; DestDir: "{app}\licenses"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "installed.marker"; DestDir: "{app}"; DestName: ".quesora-native-installed"; Attribs: hidden; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\Quesora Native"; Filename: "{app}\QuesoraNativePreview.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\Quesora Native"; Filename: "{app}\QuesoraNativePreview.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\QuesoraNative"; ValueType: string; ValueName: "InstallerVersion"; ValueData: "{#NumericVersion}"; Flags: uninsdeletevalue uninsdeletekeyifempty

[Run]
Filename: "{app}\QuesoraNativePreview.exe"; Description: "启动 Quesora Native"; Flags: nowait postinstall skipifsilent

[Code]
var IsUpgrade: Boolean;

function InitializeSetup: Boolean;
var
  Existing: String;
  OldVersion, NewVersion: Int64;
begin
  Result := True;
  IsUpgrade := RegQueryStringValue(HKCU, 'Software\QuesoraNative', 'InstallerVersion', Existing);
  if RegQueryStringValue(HKCU, 'Software\QuesoraNative', 'InstallerVersion', Existing) and
     StrToVersion(Existing, OldVersion) and
     StrToVersion('{#NumericVersion}', NewVersion) then
  begin
    if ComparePackedVersion(OldVersion, NewVersion) > 0 then
    begin
      SuppressibleMsgBox('已安装更新的版本，本安装包不会降级覆盖。请使用新版安装包。', mbError, MB_OK, IDOK);
      Result := False;
    end;
  end;
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
 Result := IsUpgrade and ((PageID = wpWelcome) or (PageID = wpLicense));
end;
