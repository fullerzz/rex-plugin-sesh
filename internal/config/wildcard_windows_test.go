//go:build windows

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowsPreviewCommandPassesThePathInTheEnvironment(t *testing.T) {
	got, env := PreviewCommand("dir {}", `C:\work\%USERNAME% & more`)
	assert.Equal(t, `dir "%REX_SESH_PREVIEW_PATH%"`, got)
	assert.Equal(t, []string{`REX_SESH_PREVIEW_PATH=C:\work\%USERNAME% & more`}, env)
	_, env = PreviewCommand("dir {}", `C:\`)
	assert.Equal(t, []string{`REX_SESH_PREVIEW_PATH=C:\\`}, env)
}
