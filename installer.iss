; Inno Setup Script for Multiterminal UI
; Requires Inno Setup 6.x (https://jrsoftware.org/isinfo.php)

#define MyAppName "Multiterminal UI"
#define MyAppExeName "mtui.exe"
#define MyAppPublisher "Patrick GoEcommerce"
#define MyAppURL "https://github.com/patrick-goecommerce/multiterminal"
#define MyAppDescription "A terminal multiplexer for Claude Code power users"

; Version is passed via /DMyAppVersion=x.y.z on the command line.
; Fallback to 0.0.0 for local builds.
#ifndef MyAppVersion
  #define MyAppVersion "0.0.0"
#endif

[Setup]
AppId={{B7E3F4A1-9C2D-4E5F-A8B6-1D3E5F7A9C2B}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
OutputBaseFilename=mtui-setup-{#MyAppVersion}
OutputDir=installer-output
Compression=lzma2/ultra64
SolidCompression=yes
SetupIconFile=build\windows\icon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "german"; MessagesFile: "compiler:Languages\German.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "build\bin\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
