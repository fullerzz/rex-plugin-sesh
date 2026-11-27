package sources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUniqueNamesAddParentsOnlyWhereNamesClash(t *testing.T) {
	assert.Equal(t,
		[]string{"a/docs", "b/docs", "api", "x/a/src", "y/a/src", "/"},
		uniqueNames([]string{"/src/a/docs", "/src/b/docs", "/src/api", "/x/a/src", "/y/a/src", "/"}))
	// A path that runs out of elements keeps its full name.
	assert.Equal(t, []string{"docs", "a/docs"}, uniqueNames([]string{"/docs", "/a/docs"}))
}

// Every directory is listed, even where two share a last element.
func TestZoxideListsDirectoriesThatShareAName(t *testing.T) {
	z := Zoxide{Runner: func(context.Context) (string, error) {
		return "9 /src/a/docs\n3 /src/b/docs\n1 /src/api\n", nil
	}}
	got, err := z.List(t.Context())
	require.NoError(t, err)
	ordered := got.Ordered()
	names := make([]string, 0, len(ordered))
	for _, s := range ordered {
		names = append(names, s.Name)
	}
	assert.Equal(t, []string{"a/docs", "b/docs", "api"}, names)
}
