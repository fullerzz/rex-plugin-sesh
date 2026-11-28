package launcher

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodePowerShellIsBase64UTF16LE(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(EncodePowerShell("é 'x' \"y\"\n"))
	require.NoError(t, err)
	require.Zero(t, len(raw)%2)
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[2*i:])
	}
	assert.Equal(t, "é 'x' \"y\"\n", string(utf16.Decode(units)))
}

// The config machine's binary path and client ID can't reach the script.
func TestWindowsPickerArgvCarriesOnlyTheScript(t *testing.T) {
	argv := WindowsPickerArgv()
	require.Len(t, argv, 6)
	assert.Equal(t, []string{"powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand"}, argv[:5])
	assert.Equal(t, EncodePowerShell(WindowsPicker), argv[5])
}
