package preview

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeScreen struct{ session, block string }

func (f *fakeScreen) Screen(_ context.Context, sessionID, blockID string) (string, error) {
	f.session, f.block = sessionID, blockID
	return "screen\n", nil
}

func TestRenderPaneReadsTheSessionBlock(t *testing.T) {
	fake := &fakeScreen{}
	var hosts []string
	original := newScreenReader
	newScreenReader = func(host string) screenReader {
		hosts = append(hosts, host)
		return fake
	}
	t.Cleanup(func() { newScreenReader = original })
	text, err := RenderPane(context.Background(), model.Session{Source: "rex", SessionID: "session:1", BlockID: "block:9"})
	require.NoError(t, err)
	assert.Equal(t, "screen\n", text)
	assert.Equal(t, &fakeScreen{session: "session:1", block: "block:9"}, fake)
	_, err = RenderPane(context.Background(), model.Session{Machine: "devbox", Source: "rex", SessionID: "session:2", BlockID: "block:3"})
	require.NoError(t, err)
	assert.Equal(t, []string{"", "devbox"}, hosts, "a host's session is read on that host")
}

// A host's directory is not on this machine, so its preview runs nothing.
func TestRenderSummarizesAHostSession(t *testing.T) {
	out, err := Render(context.Background(), model.Session{Machine: "devbox", Source: "rex", Name: "train", SessionID: "session:2", Path: "/home/me/train"}, "ls {}")
	require.NoError(t, err)
	assert.Equal(t, "session: train\nhost: devbox\nid: session:2\npath: /home/me/train\n", out)
}

func TestRenderPaneReportsSessionsWithoutABlock(t *testing.T) {
	for _, s := range []model.Session{
		{Source: "rex", SessionID: "session:1"},
		{Source: "config", Path: "/tmp"},
	} {
		_, err := RenderPane(context.Background(), s)
		require.ErrorIs(t, err, ErrPaneUnavailable)
	}
}

func TestRenderMissingPathWithoutWorkspaceReturnsStableText(t *testing.T) {
	out, err := Render(context.Background(), model.Session{Name: "api"}, "printf %s {}")
	require.NoError(t, err)
	assert.Contains(t, out, "No item path available")
}

func TestRenderMissingPathWithSessionReturnsSessionSummary(t *testing.T) {
	out, err := Render(context.Background(), model.Session{Name: "api", SessionID: "ws1"}, "printf %s {}")
	require.NoError(t, err)
	require.Contains(t, out, "session: api")
	assert.Contains(t, out, "id: ws1")
}

func TestRenderDirectoryFallbackSorted(t *testing.T) {
	d := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(d, "b"), []byte(""), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(d, "a"), []byte(""), 0600))
	out, err := Render(context.Background(), model.Session{Path: d}, "")
	require.NoError(t, err)
	assert.Contains(t, out, "a\nb")
}
