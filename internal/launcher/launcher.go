// Package launcher holds the script that starts the picker on a Windows host
// that isn't the machine whose config opened it.
package launcher

import (
	"encoding/base64"
	"encoding/binary"
	"unicode/utf16"
)

// WindowsInstallPath is where a Windows host's own rex-sesh.exe lives, under
// the account's %LOCALAPPDATA%.
const WindowsInstallPath = `Programs\rex-sesh\rex-sesh.exe`

// WindowsPicker is the PowerShell script that runs a Windows host's own
// rex-sesh.exe picker. The config machine's binary path and client ID mean
// nothing there, so it takes no arguments. It looks in WindowsInstallPath,
// then on PATH, and otherwise says what to install and exits 127.
const WindowsPicker = `$candidates = @()
if ($env:LOCALAPPDATA) { $candidates += Join-Path $env:LOCALAPPDATA '` + WindowsInstallPath + `' }
$found = Get-Command rex-sesh.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if ($found) { $candidates += $found.Source }
foreach ($c in $candidates) {
  if (Test-Path -LiteralPath $c -PathType Leaf) { & $c picker; exit $LASTEXITCODE }
}
[Console]::Error.WriteLine('Sesh cannot start: rex-sesh is not installed on this machine.')
[Console]::Error.WriteLine('Install rex-sesh.exe at %LOCALAPPDATA%\` + WindowsInstallPath + `.')
[Console]::Error.WriteLine('See "Install on a Windows host" in the rex-sesh docs.')
exit 127
`

// WindowsPickerArgv is the command a Rex shell block runs, with shell
// "none", to start WindowsPicker. -EncodedCommand carries the script as
// base64, so no Windows command-line quoting can change it.
func WindowsPickerArgv() []string {
	return []string{"powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", EncodePowerShell(WindowsPicker)}
}

// EncodePowerShell encodes script for powershell -EncodedCommand: base64 of
// its UTF-16LE bytes.
func EncodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(b[2*i:], u)
	}
	return base64.StdEncoding.EncodeToString(b)
}
