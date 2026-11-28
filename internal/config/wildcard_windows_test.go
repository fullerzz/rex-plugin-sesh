//go:build windows

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowsShellQuote(t *testing.T) {
	assert.Equal(t, `dir "C:\Users\Jo Doe"`, SubstitutePath("dir {}", `C:\Users\Jo Doe`))
	assert.Equal(t, `dir "C:\\"`, SubstitutePath("dir {}", `C:\`))
	assert.Equal(t, `dir ""`, SubstitutePath("dir {}", ``))
}
