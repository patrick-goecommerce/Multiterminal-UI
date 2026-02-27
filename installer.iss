; Inno Setup Script for Multiterminal UI
; Requires Inno Setup 6.x (https://jrsoftware.org/isinfo.php)

#define MyAppName "Multiterminal UI"
#define MyAppExeName "mtui.exe"
#define MyAppPublisher "go eCommerce GmbH"
#define MyAppURL "https://github.com/patrick-goecommerce/Multiterminal-UI"
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
LicenseFile=LICENSE

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "german"; MessagesFile: "compiler:Languages\German.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "build\bin\mtui-portable.exe"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Check: NeedsAddPath(ExpandConstant('{app}'))

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Uppercase(Param) + ';', ';' + Uppercase(OrigPath) + ';') = 0;
end;

// Returns one dot-separated version segment (0-based index).
function VersionPart(const V: string; Index: integer): integer;
var
  S: string;
  P, Cur: integer;
begin
  S := V;
  Cur := 0;
  Result := 0;
  while True do
  begin
    P := Pos('.', S);
    if P = 0 then
    begin
      if Cur = Index then
        Result := StrToIntDef(S, 0);
      exit;
    end;
    if Cur = Index then
    begin
      Result := StrToIntDef(Copy(S, 1, P - 1), 0);
      exit;
    end;
    S := Copy(S, P + 1, Length(S));
    Inc(Cur);
  end;
end;

// Returns true when A is strictly greater than B (semver: major.minor.patch).
function IsVersionGreater(const A, B: string): boolean;
var
  i, av, bv: integer;
begin
  Result := False;
  for i := 0 to 2 do
  begin
    av := VersionPart(A, i);
    bv := VersionPart(B, i);
    if av > bv then begin Result := True; exit; end;
    if av < bv then exit;
  end;
end;

// Returns the DisplayVersion stored by a previous Inno Setup installation,
// or an empty string when the app is not installed yet.
function GetInstalledVersion(): string;
var
  UninstallKey: string;
  V: string;
begin
  UninstallKey := 'Software\Microsoft\Windows\CurrentVersion\Uninstall\' +
                  '{B7E3F4A1-9C2D-4E5F-A8B6-1D3E5F7A9C2B}_is1';
  Result := '';
  if RegQueryStringValue(HKEY_CURRENT_USER, UninstallKey, 'DisplayVersion', V) then
    Result := V;
end;

// Downgrade protection: abort when the setup version is older than what is
// already installed.  Same version (reinstall) and upgrades are always allowed.
function InitializeSetup(): boolean;
var
  Installed, Setup: string;
begin
  Result := True;
  Installed := GetInstalledVersion();
  if Installed = '' then exit;  // not installed yet — nothing to check

  Setup := '{#MyAppVersion}';

  if IsVersionGreater(Installed, Setup) then
  begin
    MsgBox(
      'Eine neuere Version (' + Installed + ') ist bereits installiert.' + #13#10 +
      'Dieser Installer (' + Setup + ') kann keine ältere Version einspielen.' + #13#10#13#10 +
      'Lade die aktuelle Version von GitHub herunter:' + #13#10 +
      '{#MyAppURL}/releases',
      mbError, MB_OK
    );
    Result := False;
  end;
end;

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
