package picker

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisableCloseRefusesCtrlXAndHidesIt(t *testing.T) {
	closed := ""
	m := newTeaModel([]model.Session{{Source: "rex", Name: "api", SessionID: "session:1"}}, Options{
		DisplayOptions: DisplayOptions{DisableClose: true},
		Backend: &fakeBackend{closeWorkspace: func(_ context.Context, id string) error {
			closed = id
			return nil
		}},
	})
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = updated.(teaModel)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(teaModel)
	}
	assert.Empty(t, closed)
	assert.Empty(t, m.closingSessionID)
	assert.Equal(t, []string{"api"}, sessionNames(m.list.All))
	assert.True(t, m.shake.Animating(), "the refused key shakes the picker")
	assert.NotContains(t, ansi.Strip(m.footerText()), "ctrl+x")
}

func TestCurrentSessionRowShowsMarker(t *testing.T) {
	current := ansi.Strip(row(model.Session{Source: "rex", Name: "api", SessionID: "session:1", Attached: true}, false, 80, false, ""))
	other := ansi.Strip(row(model.Session{Source: "rex", Name: "web", SessionID: "session:2"}, false, 80, false, ""))
	assert.True(t, strings.HasPrefix(current, "◆   [rex]"), current)
	assert.NotContains(t, other, "◆")
}

// The marker sits in the rail, so an agent's status glyph still shows.
func TestCurrentMarkerKeepsTheAgentStatus(t *testing.T) {
	got := ansi.Strip(row(model.Session{Source: "rex", Name: "api", SessionID: "session:1", Attached: true, AgentStatus: "done"}, false, 80, false, ""))
	assert.True(t, strings.HasPrefix(got, "◆ ● [rex]"), got)
	selected := ansi.Strip(row(model.Session{Source: "rex", Name: "api", SessionID: "session:1", Attached: true}, true, 80, false, ""))
	assert.True(t, strings.HasPrefix(selected, "┃"), "the selection rail wins on the selected row")
}

// Without history or agent statuses every sort mode keeps rex ls order.
func TestSortModesKeepRexOrderWithoutHistory(t *testing.T) {
	items := []model.Session{
		{Source: "rex", Name: "c", SessionID: "session:3"},
		{Source: "rex", Name: "a", SessionID: "session:1"},
		{Source: "rex", Name: "b", SessionID: "session:2"},
	}
	for _, mode := range []string{workspaceSortWorkspace, workspaceSortRecent, workspaceSortAgent} {
		m := newTeaModel(items, Options{DisplayOptions: DisplayOptions{WorkspaceSort: mode}})
		require.Equal(t, []string{"c", "a", "b"}, sessionNames(m.list.All), mode)
		next, _ := m.toggleWorkspaceSort()
		assert.Equal(t, []string{"c", "a", "b"}, sessionNames(next.list.All), "after toggling from "+mode)
	}
}
